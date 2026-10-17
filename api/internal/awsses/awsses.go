// Package awsses calls the few Amazon SES v2 and STS operations Yuva needs to manage SES tenants,
// signed with minio-go's Signature Version 4 signer.
package awsses

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7/pkg/signer"
)

const (
	maxResponseBytes = 4 << 20
	identityPageSize = 1000
	entityPageSize   = 100
	resourcePageSize = 100
	maxPages         = 1000
)

type Config struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	// SESEndpoint and STSEndpoint replace https://email.<region>.amazonaws.com and
	// https://sts.<region>.amazonaws.com; for tests.
	SESEndpoint string
	STSEndpoint string
	HTTPClient  *http.Client
	// Interval is the least time between two requests; 0 means one a second, negative none.
	Interval time.Duration
}

type Client struct {
	region, key, secret string
	ses, sts            string
	http                *http.Client
	interval            time.Duration

	mu      sync.Mutex
	next    time.Time
	account string
}

func New(c Config) *Client {
	out := &Client{
		region: c.Region, key: c.AccessKeyID, secret: c.SecretAccessKey,
		ses: strings.TrimRight(c.SESEndpoint, "/"), sts: strings.TrimRight(c.STSEndpoint, "/"),
		http: c.HTTPClient, interval: c.Interval,
	}
	if out.ses == "" {
		out.ses = "https://email." + c.Region + ".amazonaws.com"
	}
	if out.sts == "" {
		out.sts = "https://sts." + c.Region + ".amazonaws.com"
	}
	if out.http == nil {
		out.http = &http.Client{Timeout: 30 * time.Second}
	}
	if out.interval == 0 {
		out.interval = time.Second
	}
	return out
}

func (c *Client) Region() string { return c.region }

// Error is an answer of AWS other than success. Code is the exception name without its namespace,
// such as NotFoundException or AccessDenied.
type Error struct {
	Operation string
	Status    int
	Code      string
	Message   string
}

func (e *Error) Error() string {
	msg := e.Code
	if e.Message != "" {
		msg += ": " + e.Message
	}
	return fmt.Sprintf("amazon ses %s: %s (HTTP %d)", e.Operation, msg, e.Status)
}

// Temporary reports an answer worth retrying later: throttling or a server error.
func (e *Error) Temporary() bool {
	return e.Status == http.StatusTooManyRequests || e.Status >= 500 || e.Code == "TooManyRequestsException" || e.Code == "Throttling"
}

// IsCode reports whether err is an AWS answer with the code.
func IsCode(err error, code string) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}

// Temporary reports whether err is worth retrying: an AWS throttling or server error, or a failure
// to reach AWS at all.
func Temporary(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Temporary()
	}
	return err != nil
}

type Tenant struct {
	Name, ID, ARN, SendingStatus string
	Tags                         map[string]string
}

type Resource struct{ Type, ARN string }

type Identity struct {
	Name, Type     string
	Verified       bool
	SendingEnabled bool
}

type Status struct {
	Status, Cause string
	UpdatedAt     *time.Time
}

type Entity struct {
	Reference, Type, Aggregate, Impact string
	Customer, AWS                      Status
}

func (c *Client) wait(ctx context.Context) error {
	if c.interval < 0 {
		return nil
	}
	c.mu.Lock()
	now := time.Now()
	at := c.next
	if at.Before(now) {
		at = now
	}
	c.next = at.Add(c.interval)
	c.mu.Unlock()
	if d := time.Until(at); d > 0 {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
	return nil
}

// call sends a REST-JSON request to SES v2.
func (c *Client) call(ctx context.Context, op, method, path string, query url.Values, in, out any) error {
	var body []byte
	if in != nil {
		var err error
		if body, err = json.Marshal(in); err != nil {
			return err
		}
	}
	u := c.ses + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	sum := sha256.Sum256(body)
	req.Header.Set("X-Amz-Content-Sha256", hex.EncodeToString(sum[:]))
	signed := c.sign(req, "ses")
	return c.do(ctx, op, signed, func(status int, h http.Header, raw []byte) error {
		if status/100 != 2 {
			return restError(op, status, h, raw)
		}
		if out == nil || len(raw) == 0 {
			return nil
		}
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("amazon ses %s: %w", op, err)
		}
		return nil
	})
}

// sign signs req for service. A path with escaped parts (an ARN in the path) is signed as AWS wants
// it for every service but S3: each segment encoded twice. minio's signer encodes the path once, so
// it signs a copy whose path holds the once-encoded form.
func (c *Client) sign(req *http.Request, service string) *http.Request {
	if req.URL.RawPath == "" {
		return signer.SignV4WithServiceType(*req, c.key, c.secret, "", c.region, service)
	}
	cp := req.Clone(req.Context())
	u := *req.URL
	u.Path, u.RawPath = req.URL.RawPath, ""
	cp.URL = &u
	cp.Header = req.Header.Clone()
	signed := signer.SignV4WithServiceType(*cp, c.key, c.secret, "", c.region, service)
	req.Header.Set("X-Amz-Date", signed.Header.Get("X-Amz-Date"))
	req.Header.Set("Authorization", signed.Header.Get("Authorization"))
	return req
}

func (c *Client) do(ctx context.Context, op string, req *http.Request, read func(int, http.Header, []byte) error) error {
	if err := c.wait(ctx); err != nil {
		return err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("amazon ses %s: %w", op, scrubURLError(err))
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("amazon ses %s: %w", op, err)
	}
	return read(res.StatusCode, res.Header, raw)
}

func scrubURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

func restError(op string, status int, h http.Header, raw []byte) error {
	e := &Error{Operation: op, Status: status}
	var body struct {
		Type     string `json:"__type"`
		Message  string `json:"message"`
		Message2 string `json:"Message"`
		Code     string `json:"code"`
	}
	_ = json.Unmarshal(raw, &body)
	e.Code = errorCode(h.Get("X-Amzn-Errortype"))
	if e.Code == "" {
		e.Code = errorCode(body.Type)
	}
	if e.Code == "" {
		e.Code = body.Code
	}
	if e.Code == "" {
		e.Code = http.StatusText(status)
	}
	e.Message = body.Message
	if e.Message == "" {
		e.Message = body.Message2
	}
	e.Message = truncate(e.Message, 1000)
	return e
}

// errorCode is the exception name of an x-amzn-ErrorType or __type value such as
// "NotFoundException:http://internal.amazon.com/coral/com.amazonaws.ses/".
func errorCode(v string) string {
	v, _, _ = strings.Cut(v, ":")
	if i := strings.LastIndex(v, "#"); i >= 0 {
		v = v[i+1:]
	}
	return strings.TrimSpace(v)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (c *Client) stsCall(ctx context.Context, action string, params url.Values, out any) error {
	form := url.Values{"Action": {action}, "Version": {"2011-06-15"}}
	for k, v := range params {
		form[k] = v
	}
	body := []byte(form.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.sts+"/", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	sum := sha256.Sum256(body)
	req.Header.Set("X-Amz-Content-Sha256", hex.EncodeToString(sum[:]))
	signed := signer.SignV4STS(*req, c.key, c.secret, c.region)
	return c.do(ctx, action, signed, func(status int, _ http.Header, raw []byte) error {
		if status/100 != 2 {
			var x struct {
				Error struct {
					Code    string `xml:"Code"`
					Message string `xml:"Message"`
				} `xml:"Error"`
			}
			_ = xml.Unmarshal(raw, &x)
			e := &Error{Operation: action, Status: status, Code: x.Error.Code, Message: truncate(x.Error.Message, 1000)}
			if e.Code == "" {
				e.Code = http.StatusText(status)
			}
			return e
		}
		if err := xml.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("amazon sts %s: %w", action, err)
		}
		return nil
	})
}

// CallerAccount is the AWS account of the server's own key (sts:GetCallerIdentity); it is asked once.
func (c *Client) CallerAccount(ctx context.Context) (string, error) {
	c.mu.Lock()
	acct := c.account
	c.mu.Unlock()
	if acct != "" {
		return acct, nil
	}
	var out struct {
		Result struct {
			Account string `xml:"Account"`
		} `xml:"GetCallerIdentityResult"`
	}
	if err := c.stsCall(ctx, "GetCallerIdentity", nil, &out); err != nil {
		return "", err
	}
	if out.Result.Account == "" {
		return "", errors.New("amazon sts GetCallerIdentity: no account in the answer")
	}
	c.mu.Lock()
	c.account = out.Result.Account
	c.mu.Unlock()
	return out.Result.Account, nil
}

// AccessKeyAccount is the AWS account an access key id belongs to (sts:GetAccessKeyInfo).
func (c *Client) AccessKeyAccount(ctx context.Context, keyID string) (string, error) {
	var out struct {
		Result struct {
			Account string `xml:"Account"`
		} `xml:"GetAccessKeyInfoResult"`
	}
	if err := c.stsCall(ctx, "GetAccessKeyInfo", url.Values{"AccessKeyId": {keyID}}, &out); err != nil {
		return "", err
	}
	if out.Result.Account == "" {
		return "", errors.New("amazon sts GetAccessKeyInfo: no account in the answer")
	}
	return out.Result.Account, nil
}

type tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

type tenantBody struct {
	TenantName    string `json:"TenantName"`
	TenantID      string `json:"TenantId"`
	TenantArn     string `json:"TenantArn"`
	SendingStatus string `json:"SendingStatus"`
	Tags          []tag  `json:"Tags"`
}

func (t tenantBody) tenant() Tenant {
	out := Tenant{Name: t.TenantName, ID: t.TenantID, ARN: t.TenantArn, SendingStatus: t.SendingStatus}
	if len(t.Tags) > 0 {
		out.Tags = map[string]string{}
		for _, g := range t.Tags {
			out.Tags[g.Key] = g.Value
		}
	}
	return out
}

// CreateTenant creates a tenant with its own suppression list for bounces and complaints.
func (c *Client) CreateTenant(ctx context.Context, name string, tags map[string]string) (Tenant, error) {
	in := struct {
		TenantName            string `json:"TenantName"`
		Tags                  []tag  `json:"Tags,omitempty"`
		SuppressionAttributes struct {
			SuppressedReasons []string `json:"SuppressedReasons"`
			SuppressionScope  string   `json:"SuppressionScope"`
		} `json:"SuppressionAttributes"`
	}{TenantName: name}
	in.SuppressionAttributes.SuppressedReasons = []string{"BOUNCE", "COMPLAINT"}
	in.SuppressionAttributes.SuppressionScope = "TENANT"
	for _, k := range slices.Sorted(maps.Keys(tags)) {
		in.Tags = append(in.Tags, tag{Key: k, Value: tags[k]})
	}
	var out tenantBody
	if err := c.call(ctx, "CreateTenant", http.MethodPost, "/v2/email/tenants", nil, in, &out); err != nil {
		return Tenant{}, err
	}
	return out.tenant(), nil
}

func (c *Client) GetTenant(ctx context.Context, name string) (Tenant, error) {
	var out struct {
		Tenant tenantBody `json:"Tenant"`
	}
	if err := c.call(ctx, "GetTenant", http.MethodPost, "/v2/email/tenants/get", nil, map[string]string{"TenantName": name}, &out); err != nil {
		return Tenant{}, err
	}
	return out.Tenant.tenant(), nil
}

func (c *Client) DeleteTenant(ctx context.Context, name string) error {
	return c.call(ctx, "DeleteTenant", http.MethodPost, "/v2/email/tenants/delete", nil, map[string]string{"TenantName": name}, nil)
}

// Associate adds a resource to a tenant; one already associated is fine.
func (c *Client) Associate(ctx context.Context, tenant, resourceARN string) error {
	err := c.call(ctx, "CreateTenantResourceAssociation", http.MethodPost, "/v2/email/tenants/resources", nil,
		map[string]string{"TenantName": tenant, "ResourceArn": resourceARN}, nil)
	if IsCode(err, "AlreadyExistsException") {
		return nil
	}
	return err
}

// Dissociate removes a resource from a tenant; one not associated is fine.
func (c *Client) Dissociate(ctx context.Context, tenant, resourceARN string) error {
	err := c.call(ctx, "DeleteTenantResourceAssociation", http.MethodPost, "/v2/email/tenants/resources/delete", nil,
		map[string]string{"TenantName": tenant, "ResourceArn": resourceARN}, nil)
	if IsCode(err, "NotFoundException") {
		return nil
	}
	return err
}

func (c *Client) TenantResources(ctx context.Context, tenant string) ([]Resource, error) {
	var out []Resource
	next := ""
	for range maxPages {
		in := map[string]any{"TenantName": tenant, "PageSize": resourcePageSize}
		if next != "" {
			in["NextToken"] = next
		}
		var page struct {
			TenantResources []struct {
				ResourceType string `json:"ResourceType"`
				ResourceArn  string `json:"ResourceArn"`
			} `json:"TenantResources"`
			NextToken string `json:"NextToken"`
		}
		if err := c.call(ctx, "ListTenantResources", http.MethodPost, "/v2/email/tenants/resources/list", nil, in, &page); err != nil {
			return nil, err
		}
		for _, r := range page.TenantResources {
			out = append(out, Resource{Type: r.ResourceType, ARN: r.ResourceArn})
		}
		if next = page.NextToken; next == "" {
			return out, nil
		}
	}
	return out, errors.New("amazon ses ListTenantResources: too many pages")
}

// Identities lists every e-mail address and domain identity of the account in the region.
func (c *Client) Identities(ctx context.Context) ([]Identity, error) {
	var out []Identity
	next := ""
	for range maxPages {
		q := url.Values{"PageSize": {strconv.Itoa(identityPageSize)}}
		if next != "" {
			q.Set("NextToken", next)
		}
		var page struct {
			EmailIdentities []struct {
				IdentityType       string `json:"IdentityType"`
				IdentityName       string `json:"IdentityName"`
				SendingEnabled     bool   `json:"SendingEnabled"`
				VerificationStatus string `json:"VerificationStatus"`
			} `json:"EmailIdentities"`
			NextToken string `json:"NextToken"`
		}
		if err := c.call(ctx, "ListEmailIdentities", http.MethodGet, "/v2/email/identities", q, nil, &page); err != nil {
			return nil, err
		}
		for _, i := range page.EmailIdentities {
			out = append(out, Identity{Name: i.IdentityName, Type: i.IdentityType, Verified: i.VerificationStatus == "SUCCESS", SendingEnabled: i.SendingEnabled})
		}
		if next = page.NextToken; next == "" {
			return out, nil
		}
	}
	return out, errors.New("amazon ses ListEmailIdentities: too many pages")
}

type statusBody struct {
	Status               string   `json:"Status"`
	Cause                string   `json:"Cause"`
	LastUpdatedTimestamp *float64 `json:"LastUpdatedTimestamp"`
}

func (s *statusBody) status() Status {
	if s == nil {
		return Status{}
	}
	out := Status{Status: s.Status, Cause: s.Cause}
	if s.LastUpdatedTimestamp != nil {
		sec, frac := math.Modf(*s.LastUpdatedTimestamp)
		t := time.Unix(int64(sec), int64(frac*1e9)).UTC()
		out.UpdatedAt = &t
	}
	return out
}

// ReputationEntities lists every reputation entity whose reference starts with prefix, such as the
// tenant ARNs under a name prefix.
func (c *Client) ReputationEntities(ctx context.Context, prefix string) ([]Entity, error) {
	var out []Entity
	next := ""
	for range maxPages {
		in := map[string]any{"Filter": map[string]string{"ENTITY_REFERENCE_PREFIX": prefix}, "PageSize": entityPageSize}
		if next != "" {
			in["NextToken"] = next
		}
		var page struct {
			ReputationEntities []struct {
				ReputationEntityReference string      `json:"ReputationEntityReference"`
				ReputationEntityType      string      `json:"ReputationEntityType"`
				CustomerManagedStatus     *statusBody `json:"CustomerManagedStatus"`
				AwsSesManagedStatus       *statusBody `json:"AwsSesManagedStatus"`
				SendingStatusAggregate    string      `json:"SendingStatusAggregate"`
				ReputationImpact          string      `json:"ReputationImpact"`
			} `json:"ReputationEntities"`
			NextToken string `json:"NextToken"`
		}
		if err := c.call(ctx, "ListReputationEntities", http.MethodPost, "/v2/email/reputation/entities", nil, in, &page); err != nil {
			return nil, err
		}
		for _, e := range page.ReputationEntities {
			out = append(out, Entity{
				Reference: e.ReputationEntityReference, Type: e.ReputationEntityType, Aggregate: e.SendingStatusAggregate,
				Impact: e.ReputationImpact, Customer: e.CustomerManagedStatus.status(), AWS: e.AwsSesManagedStatus.status(),
			})
		}
		if next = page.NextToken; next == "" {
			return out, nil
		}
	}
	return out, errors.New("amazon ses ListReputationEntities: too many pages")
}

// Reinstate sets a tenant's customer-managed status to REINSTATED, SES's grace state that lets it
// send while findings are still open.
func (c *Client) Reinstate(ctx context.Context, tenantARN string) error {
	escaped := "/v2/email/reputation/entities/RESOURCE/" + url.PathEscape(tenantARN) + "/customer-managed-status"
	escaped = strings.ReplaceAll(escaped, ":", "%3A")
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.ses+escaped, nil)
	if err != nil {
		return err
	}
	return c.callRequest(ctx, "UpdateReputationEntityCustomerManagedStatus", req, map[string]string{"SendingStatus": "REINSTATED"})
}

func (c *Client) callRequest(ctx context.Context, op string, req *http.Request, in any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	req.Header.Set("Content-Type", "application/json")
	sum := sha256.Sum256(body)
	req.Header.Set("X-Amz-Content-Sha256", hex.EncodeToString(sum[:]))
	signed := c.sign(req, "ses")
	return c.do(ctx, op, signed, func(status int, h http.Header, raw []byte) error {
		if status/100 != 2 {
			return restError(op, status, h, raw)
		}
		return nil
	})
}

package awsses_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/productdevbook/yuva/api/internal/awsses"
)

const (
	testKey    = "AKIDEXAMPLE"
	testSecret = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
	testRegion = "eu-west-1"
)

type seen struct {
	method, path, rawQuery string
	header                 http.Header
	host                   string
	body                   []byte
}

type answer struct {
	status int
	header map[string]string
	body   string
}

type fakeAWS struct {
	mu      sync.Mutex
	reqs    []seen
	answers []answer
}

func (f *fakeAWS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, seen{method: r.Method, path: r.URL.EscapedPath(), rawQuery: r.URL.RawQuery, header: r.Header.Clone(), host: r.Host, body: body})
	a := answer{status: 200, body: "{}"}
	if len(f.answers) > 0 {
		a, f.answers = f.answers[0], f.answers[1:]
	}
	if a.status == 0 {
		a = answer{status: 200, body: "{}"}
	}
	f.mu.Unlock()
	for k, v := range a.header {
		w.Header().Set(k, v)
	}
	w.WriteHeader(a.status)
	_, _ = io.WriteString(w, a.body)
}

func newClient(t *testing.T, answers ...answer) (*awsses.Client, *fakeAWS) {
	t.Helper()
	f := &fakeAWS{answers: answers}
	ts := httptest.NewServer(f)
	t.Cleanup(ts.Close)
	c := awsses.New(awsses.Config{Region: testRegion, AccessKeyID: testKey, SecretAccessKey: testSecret, SESEndpoint: ts.URL, STSEndpoint: ts.URL, Interval: -1})
	return c, f
}

// awsEscape is RFC 3986 percent-encoding of everything but unreserved characters, as SigV4 wants it.
func awsEscape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if ('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z') || ('0' <= c && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func hmacSHA(key []byte, v string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(v))
	return m.Sum(nil)
}

// verifySignature recomputes the SigV4 signature of a received request from scratch: the canonical
// URI encodes each segment of the path as sent once more (every service but S3).
func verifySignature(t *testing.T, r seen, service string) {
	t.Helper()
	auth := r.header.Get("Authorization")
	date := r.header.Get("X-Amz-Date")
	if len(date) < 8 {
		t.Fatalf("no X-Amz-Date: %v", r.header)
	}
	scope := date[:8] + "/" + testRegion + "/" + service + "/aws4_request"
	prefix := "AWS4-HMAC-SHA256 Credential=" + testKey + "/" + scope + ", SignedHeaders="
	if !strings.HasPrefix(auth, prefix) {
		t.Fatalf("Authorization %q does not start with %q", auth, prefix)
	}
	signedHeaders, sig, ok := strings.Cut(strings.TrimPrefix(auth, prefix), ", Signature=")
	if !ok {
		t.Fatalf("Authorization %q", auth)
	}
	sum := sha256.Sum256(r.body)
	if service == "ses" {
		if got := r.header.Get("X-Amz-Content-Sha256"); got != hex.EncodeToString(sum[:]) {
			t.Fatalf("x-amz-content-sha256 %s is not the body's hash", got)
		}
		if !slices.Contains(strings.Split(signedHeaders, ";"), "x-amz-content-sha256") {
			t.Fatalf("x-amz-content-sha256 is not signed: %s", signedHeaders)
		}
	}
	segs := strings.Split(r.path, "/")
	for i, s := range segs {
		segs[i] = awsEscape(s)
	}
	q, _ := url.ParseQuery(r.rawQuery)
	var qs []string
	for k, vs := range q {
		for _, v := range vs {
			qs = append(qs, awsEscape(k)+"="+awsEscape(v))
		}
	}
	slices.Sort(qs)
	var ch strings.Builder
	for _, h := range strings.Split(signedHeaders, ";") {
		v := strings.Join(r.header.Values(h), ",")
		if h == "host" {
			v = r.host
		}
		ch.WriteString(h + ":" + strings.TrimSpace(v) + "\n")
	}
	canonical := strings.Join([]string{r.method, strings.Join(segs, "/"), strings.Join(qs, "&"), ch.String(), signedHeaders, hex.EncodeToString(sum[:])}, "\n")
	cr := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + date + "\n" + scope + "\n" + hex.EncodeToString(cr[:])
	key := hmacSHA(hmacSHA(hmacSHA(hmacSHA([]byte("AWS4"+testSecret), date[:8]), testRegion), service), "aws4_request")
	if want := hex.EncodeToString(hmacSHA(key, toSign)); sig != want {
		t.Fatalf("signature %s, computed %s over\n%s", sig, want, canonical)
	}
}

func jsonBody(t *testing.T, r seen) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.body, &m); err != nil {
		t.Fatalf("body %q: %v", r.body, err)
	}
	return m
}

func TestOperations(t *testing.T) {
	ctx := context.Background()
	const arn = "arn:aws:ses:eu-west-1:123456789012:tenant/yuva-abc/tn-1"
	for _, tc := range []struct {
		name   string
		answer answer
		call   func(*awsses.Client) error
		method string
		path   string
		check  func(*testing.T, seen)
	}{
		{
			name:   "create tenant",
			answer: answer{status: 200, body: `{"TenantName":"yuva-abc","TenantId":"tn-1","TenantArn":"` + arn + `","SendingStatus":"ENABLED"}`},
			call: func(c *awsses.Client) error {
				tn, err := c.CreateTenant(ctx, "yuva-abc", map[string]string{"yuva-workspace": "w", "yuva-inbox": "i"})
				if err == nil && (tn.ARN != arn || tn.Name != "yuva-abc") {
					err = fmt.Errorf("tenant %+v", tn)
				}
				return err
			},
			method: "POST", path: "/v2/email/tenants",
			check: func(t *testing.T, r seen) {
				b := jsonBody(t, r)
				sup := b["SuppressionAttributes"].(map[string]any)
				if b["TenantName"] != "yuva-abc" || sup["SuppressionScope"] != "TENANT" || fmt.Sprint(sup["SuppressedReasons"]) != "[BOUNCE COMPLAINT]" {
					t.Fatalf("body %s", r.body)
				}
				if fmt.Sprint(b["Tags"]) != "[map[Key:yuva-inbox Value:i] map[Key:yuva-workspace Value:w]]" {
					t.Fatalf("tags %v", b["Tags"])
				}
			},
		},
		{
			name:   "get tenant",
			answer: answer{status: 200, body: `{"Tenant":{"TenantName":"yuva-abc","TenantArn":"` + arn + `","Tags":[{"Key":"yuva-inbox","Value":"i"}]}}`},
			call: func(c *awsses.Client) error {
				tn, err := c.GetTenant(ctx, "yuva-abc")
				if err == nil && (tn.ARN != arn || tn.Tags["yuva-inbox"] != "i") {
					err = fmt.Errorf("tenant %+v", tn)
				}
				return err
			},
			method: "POST", path: "/v2/email/tenants/get",
			check: func(t *testing.T, r seen) {
				if jsonBody(t, r)["TenantName"] != "yuva-abc" {
					t.Fatalf("body %s", r.body)
				}
			},
		},
		{
			name: "delete tenant", call: func(c *awsses.Client) error { return c.DeleteTenant(ctx, "yuva-abc") },
			method: "POST", path: "/v2/email/tenants/delete",
		},
		{
			name: "associate", call: func(c *awsses.Client) error {
				return c.Associate(ctx, "yuva-abc", "arn:aws:ses:eu-west-1:1:identity/example.com")
			},
			method: "POST", path: "/v2/email/tenants/resources",
			check: func(t *testing.T, r seen) {
				b := jsonBody(t, r)
				if b["TenantName"] != "yuva-abc" || b["ResourceArn"] != "arn:aws:ses:eu-west-1:1:identity/example.com" {
					t.Fatalf("body %s", r.body)
				}
			},
		},
		{
			name:   "associate twice",
			answer: answer{status: 400, header: map[string]string{"X-Amzn-ErrorType": "AlreadyExistsException:http://internal.amazon.com/"}, body: `{"message":"exists"}`},
			call:   func(c *awsses.Client) error { return c.Associate(ctx, "yuva-abc", "arn:x") },
			method: "POST", path: "/v2/email/tenants/resources",
		},
		{
			name:   "dissociate gone",
			answer: answer{status: 404, body: `{"__type":"com.amazonaws.ses#NotFoundException","message":"gone"}`},
			call:   func(c *awsses.Client) error { return c.Dissociate(ctx, "yuva-abc", "arn:x") },
			method: "POST", path: "/v2/email/tenants/resources/delete",
		},
		{
			name: "reinstate", call: func(c *awsses.Client) error { return c.Reinstate(ctx, arn) },
			method: "PUT", path: "/v2/email/reputation/entities/RESOURCE/arn%3Aaws%3Ases%3Aeu-west-1%3A123456789012%3Atenant%2Fyuva-abc%2Ftn-1/customer-managed-status",
			check: func(t *testing.T, r seen) {
				if jsonBody(t, r)["SendingStatus"] != "REINSTATED" {
					t.Fatalf("body %s", r.body)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, f := newClient(t, tc.answer)
			if err := tc.call(c); err != nil {
				t.Fatal(err)
			}
			if len(f.reqs) != 1 {
				t.Fatalf("%d requests", len(f.reqs))
			}
			r := f.reqs[0]
			if r.method != tc.method || r.path != tc.path {
				t.Fatalf("%s %s, want %s %s", r.method, r.path, tc.method, tc.path)
			}
			verifySignature(t, r, "ses")
			if tc.check != nil {
				tc.check(t, r)
			}
		})
	}
}

func TestPaging(t *testing.T) {
	ctx := context.Background()
	c, f := newClient(t,
		answer{status: 200, body: `{"EmailIdentities":[{"IdentityType":"DOMAIN","IdentityName":"example.com","SendingEnabled":true,"VerificationStatus":"SUCCESS"}],"NextToken":"n/2+"}`},
		answer{status: 200, body: `{"EmailIdentities":[{"IdentityType":"EMAIL_ADDRESS","IdentityName":"a@example.org","SendingEnabled":true,"VerificationStatus":"PENDING"}]}`},
	)
	ids, err := c.Identities(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || !ids[0].Verified || ids[1].Verified || ids[1].Type != "EMAIL_ADDRESS" {
		t.Fatalf("identities %+v", ids)
	}
	if len(f.reqs) != 2 || f.reqs[0].method != "GET" || f.reqs[0].path != "/v2/email/identities" {
		t.Fatalf("requests %+v", f.reqs)
	}
	if q, _ := url.ParseQuery(f.reqs[1].rawQuery); q.Get("NextToken") != "n/2+" || q.Get("PageSize") != "1000" {
		t.Fatalf("second page query %q", f.reqs[1].rawQuery)
	}
	for _, r := range f.reqs {
		verifySignature(t, r, "ses")
	}

	c, f = newClient(t,
		answer{status: 200, body: `{"ReputationEntities":[{"ReputationEntityReference":"arn:a","ReputationEntityType":"RESOURCE","SendingStatusAggregate":"DISABLED","ReputationImpact":"HIGH",
			"CustomerManagedStatus":{"Status":"DISABLED","Cause":"policy","LastUpdatedTimestamp":1760000000.5},"AwsSesManagedStatus":{"Status":"ENABLED"}}],"NextToken":"t"}`},
		answer{status: 200, body: `{"ReputationEntities":[{"ReputationEntityReference":"arn:b","SendingStatusAggregate":"ENABLED"}]}`},
	)
	es, err := c.ReputationEntities(ctx, "arn:aws:ses:eu-west-1:1:tenant/yuva-")
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 2 || es[0].Aggregate != "DISABLED" || es[0].Customer.Cause != "policy" || es[0].Customer.UpdatedAt == nil || es[0].Customer.UpdatedAt.Unix() != 1760000000 || es[1].Reference != "arn:b" {
		t.Fatalf("entities %+v", es)
	}
	b := jsonBody(t, f.reqs[0])
	if f.reqs[0].path != "/v2/email/reputation/entities" || b["Filter"].(map[string]any)["ENTITY_REFERENCE_PREFIX"] != "arn:aws:ses:eu-west-1:1:tenant/yuva-" {
		t.Fatalf("first page %s %s", f.reqs[0].path, f.reqs[0].body)
	}
	if jsonBody(t, f.reqs[1])["NextToken"] != "t" {
		t.Fatalf("second page %s", f.reqs[1].body)
	}

	c, f = newClient(t,
		answer{status: 200, body: `{"TenantResources":[{"ResourceType":"EMAIL_IDENTITY","ResourceArn":"arn:i"}],"NextToken":"x"}`},
		answer{status: 200, body: `{"TenantResources":[{"ResourceType":"CONFIGURATION_SET","ResourceArn":"arn:c"}]}`},
	)
	rs, err := c.TenantResources(ctx, "yuva-abc")
	if err != nil || len(rs) != 2 || rs[1].Type != "CONFIGURATION_SET" || f.reqs[0].path != "/v2/email/tenants/resources/list" {
		t.Fatalf("resources %+v %v", rs, err)
	}
}

func TestSTS(t *testing.T) {
	ctx := context.Background()
	c, f := newClient(t,
		answer{status: 200, body: `<GetCallerIdentityResponse><GetCallerIdentityResult><Account>123456789012</Account></GetCallerIdentityResult></GetCallerIdentityResponse>`},
		answer{status: 200, body: `<GetAccessKeyInfoResponse><GetAccessKeyInfoResult><Account>210987654321</Account></GetAccessKeyInfoResult></GetAccessKeyInfoResponse>`},
		answer{status: 400, body: `<ErrorResponse><Error><Code>InvalidParameterValue</Code><Message>bad key</Message></Error></ErrorResponse>`},
	)
	for range 2 {
		acct, err := c.CallerAccount(ctx)
		if err != nil || acct != "123456789012" {
			t.Fatalf("caller %q %v", acct, err)
		}
	}
	acct, err := c.AccessKeyAccount(ctx, "AKIAOTHERACCOUNT0001")
	if err != nil || acct != "210987654321" {
		t.Fatalf("key account %q %v", acct, err)
	}
	_, err = c.AccessKeyAccount(ctx, "nope")
	if !awsses.IsCode(err, "InvalidParameterValue") || awsses.Temporary(err) {
		t.Fatalf("error %v", err)
	}
	if len(f.reqs) != 3 {
		t.Fatalf("%d requests; the caller account must be asked once", len(f.reqs))
	}
	for i, r := range f.reqs {
		verifySignature(t, r, "sts")
		form, _ := url.ParseQuery(string(r.body))
		want := []string{"GetCallerIdentity", "GetAccessKeyInfo", "GetAccessKeyInfo"}[i]
		if r.method != "POST" || r.path != "/" || form.Get("Action") != want || form.Get("Version") != "2011-06-15" {
			t.Fatalf("request %d: %s %s %s", i, r.method, r.path, r.body)
		}
	}
	if form, _ := url.ParseQuery(string(f.reqs[1].body)); form.Get("AccessKeyId") != "AKIAOTHERACCOUNT0001" {
		t.Fatalf("body %s", f.reqs[1].body)
	}
}

func TestErrors(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		answer    answer
		code      string
		temporary bool
	}{
		{answer{status: 404, header: map[string]string{"X-Amzn-ErrorType": "NotFoundException:http://internal.amazon.com/coral/com.amazonaws.ses/"}, body: `{"message":"no tenant"}`}, "NotFoundException", false},
		{answer{status: 400, body: `{"__type":"com.amazonaws.ses#AlreadyExistsException","Message":"taken"}`}, "AlreadyExistsException", false},
		{answer{status: 429, body: `{"__type":"TooManyRequestsException"}`}, "TooManyRequestsException", true},
		{answer{status: 503, body: `oops`}, "Service Unavailable", true},
		{answer{status: 403, body: `{"code":"AccessDeniedException","message":"denied"}`}, "AccessDeniedException", false},
	} {
		c, _ := newClient(t, tc.answer)
		_, err := c.GetTenant(ctx, "yuva-abc")
		var e *awsses.Error
		if !errors.As(err, &e) || e.Code != tc.code || e.Status != tc.answer.status || awsses.Temporary(err) != tc.temporary {
			t.Fatalf("%d: error %#v", tc.answer.status, err)
		}
		if strings.Contains(err.Error(), testSecret) {
			t.Fatal("the error shows the secret")
		}
	}
	c := awsses.New(awsses.Config{Region: testRegion, AccessKeyID: testKey, SecretAccessKey: testSecret, SESEndpoint: "http://127.0.0.1:1", Interval: -1})
	err := c.DeleteTenant(ctx, "yuva-abc")
	if err == nil || !awsses.Temporary(err) || strings.Contains(err.Error(), "http://") {
		t.Fatalf("unreachable: %v", err)
	}
}

// Package microsoft signs a mailbox in with the Microsoft identity platform only to obtain a token
// for IMAP: the authorization-code flow with PKCE asks for IMAP.AccessAsUser.All and
// offline_access and nothing else, and the one client that sends anything to Microsoft may only
// POST to the tenant's token endpoint. The token is used for nothing but AUTHENTICATE XOAUTH2.
package microsoft

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	LoginHost = "login.microsoftonline.com"
	IMAPHost  = "outlook.office365.com"
	IMAPPort  = 993

	IMAPScope    = "https://outlook.office.com/IMAP.AccessAsUser.All"
	OfflineScope = "offline_access"

	AuthorityCommon        = "common"
	AuthorityConsumers     = "consumers"
	AuthorityOrganizations = "organizations"

	ResultAccessDenied         = "access_denied"
	ResultConsentRequired      = "consent_required"
	ResultAdminConsentRequired = "admin_consent_required"
	ResultConditionalAccess    = "conditional_access"
	ResultTokenFailed          = "token_failed"
)

// Scopes is the whole request: IMAP and a refresh token.
var Scopes = []string{IMAPScope, OfflineScope}

var tenantRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$`)

// ValidTenant accepts a tenant id, a verified domain or one of the multi-tenant authorities.
func ValidTenant(t string) bool { return tenantRe.MatchString(t) }

// App is the app registration a sign-in uses: the server's own or the team's.
type App struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// Tokens are what a sign-in or a refresh gives.
type Tokens struct {
	Access  string    `json:"access_token"`
	Refresh string    `json:"refresh_token,omitempty"`
	Expires time.Time `json:"expires_at,omitzero"`
	Scopes  []string  `json:"scopes,omitempty"`
}

// PKCE is a code verifier and its S256 challenge.
func PKCE() (verifier, challenge string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

func endpoint(tenant, path string) string {
	return "https://" + LoginHost + "/" + url.PathEscape(tenant) + path
}

// TokenURL is the tenant's token endpoint, the only URL the token client reaches.
func TokenURL(tenant string) string { return endpoint(tenant, "/oauth2/v2.0/token") }

// AuthorizeURL asks for Scopes only, with PKCE, for the mailbox loginHint.
func AuthorizeURL(tenant, clientID, redirect, state, challenge, loginHint string) string {
	q := url.Values{
		"client_id": {clientID}, "response_type": {"code"}, "redirect_uri": {redirect}, "response_mode": {"query"},
		"scope": {strings.Join(Scopes, " ")}, "state": {state}, "code_challenge": {challenge}, "code_challenge_method": {"S256"},
		"login_hint": {loginHint},
	}
	return endpoint(tenant, "/oauth2/v2.0/authorize") + "?" + q.Encode()
}

// AdminConsentState marks the redirect back from AdminConsentURL.
const AdminConsentState = "admin_consent"

// AdminConsentURL is the page where an admin of the tenant consents to the app's IMAP permission
// for the whole organization.
func AdminConsentURL(tenant, clientID, redirect string) string {
	q := url.Values{"client_id": {clientID}, "scope": {IMAPScope}, "redirect_uri": {redirect}, "state": {AdminConsentState}}
	return endpoint(tenant, "/v2.0/adminconsent") + "?" + q.Encode()
}

// Authority picks the authority for an address whose kind is known: consumers for personal
// Microsoft accounts, organizations for work ones, common otherwise.
func Authority(personal, work bool) string {
	switch {
	case personal:
		return AuthorityConsumers
	case work:
		return AuthorityOrganizations
	}
	return AuthorityCommon
}

// Error is Microsoft refusing a sign-in, a code or a refresh token: on the redirect back (Status
// 0) or from the token endpoint.
type Error struct {
	Status      int
	Code        string
	AADSTS      []int
	Description string
}

var aadstsRe = regexp.MustCompile(`AADSTS(\d{5,6})`)

func (e *Error) Error() string {
	where := "the sign-in"
	if e.Status != 0 {
		where = "the token endpoint (" + strconv.Itoa(e.Status) + ")"
	}
	s := "Microsoft refused " + where + ": " + e.Code
	if e.Description != "" {
		s += ": " + e.Description
	}
	return s
}

func (e *Error) has(codes ...int) bool {
	for _, c := range codes {
		if slices.Contains(e.AADSTS, c) {
			return true
		}
	}
	return false
}

// Result names what went wrong in the words the person is shown.
func (e *Error) Result() string {
	switch {
	case e.has(65001, 90094, 90095, 90008):
		return ResultAdminConsentRequired
	case slices.ContainsFunc(e.AADSTS, func(c int) bool { return c >= 53000 && c <= 53011 }):
		return ResultConditionalAccess
	case e.has(65004), e.Code == "access_denied":
		return ResultAccessDenied
	case e.Code == "consent_required", e.Code == "interaction_required":
		return ResultConsentRequired
	}
	return ResultTokenFailed
}

// NewError reads an error Microsoft sent: its code, description and AADSTS numbers. The
// description keeps its first line, without trace and correlation ids.
func NewError(status int, code, description string, codes []int) *Error {
	e := &Error{Status: status, Code: code, AADSTS: slices.Clone(codes)}
	first, _, _ := strings.Cut(strings.ReplaceAll(description, "\r", ""), "\n")
	e.Description = strings.TrimSpace(first)
	if r := []rune(e.Description); len(r) > 300 {
		e.Description = string(r[:300])
	}
	for _, m := range aadstsRe.FindAllStringSubmatch(description, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && !slices.Contains(e.AADSTS, n) {
			e.AADSTS = append(e.AADSTS, n)
		}
	}
	if e.Code == "" {
		e.Code = "error"
	}
	return e
}

// ScopeError is a token that grants other permissions than the IMAP one.
type ScopeError struct{ Granted []string }

func (e *ScopeError) Error() string {
	return "the token grants " + strings.Join(e.Granted, " ") + ", not only " + IMAPScope
}

// identityScopes come with a token without granting anything on a mailbox.
var identityScopes = []string{OfflineScope, "openid", "profile", "email"}

func scopeName(s string) string {
	s = strings.ToLower(s)
	for _, prefix := range []string{"https://outlook.office.com/", "https://outlook.office365.com/"} {
		s = strings.TrimPrefix(s, prefix)
	}
	return s
}

// CheckScopes refuses a token whose granted permissions on the mailbox are anything but
// IMAP.AccessAsUser.All, such as an app also consented to SMTP.Send or other mail permissions.
// offline_access, which Microsoft may leave out of the answer, shows in the refresh token.
func CheckScopes(granted []string) error {
	var resource []string
	for _, s := range granted {
		if !slices.Contains(identityScopes, strings.ToLower(s)) {
			resource = append(resource, scopeName(s))
		}
	}
	slices.Sort(resource)
	if !slices.Equal(slices.Compact(resource), []string{scopeName(IMAPScope)}) {
		return &ScopeError{Granted: slices.Clone(granted)}
	}
	return nil
}

// Accounts are the addresses an access token names (upn, unique_name, preferred_username,
// email), lower case. A work account's token is a JWT that names them; a personal account's is
// opaque and names none, and then the mailbox's refusal of the token is what tells.
func Accounts(access string) []string {
	parts := strings.Split(access, ".")
	if len(parts) != 3 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		return nil
	}
	var out []string
	for _, k := range []string{"upn", "unique_name", "preferred_username", "email"} {
		v, _ := claims[k].(string)
		v = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(v, "live.com#")))
		if strings.Contains(v, "@") && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// Config is how the token client reaches Microsoft: Dial connects after the caller's address
// checks, RootCAs replaces the system's for tests, Now is the clock expiry is measured on.
type Config struct {
	Dial    func(ctx context.Context, network, addr string) (net.Conn, error)
	RootCAs *x509.CertPool
	Now     func() time.Time
	Timeout time.Duration
}

func (c Config) now() time.Time {
	if c.Now == nil {
		return time.Now()
	}
	return c.Now()
}

// RefusedError is a request the token client kept from leaving the process.
type RefusedError struct {
	Method string
	URL    string
}

func (e *RefusedError) Error() string { return "refused to send " + e.Method + " to " + e.URL }

// tokenOnly lets one POST to one URL through.
type tokenOnly struct {
	url  string
	next http.RoundTripper
}

func (t tokenOnly) RoundTrip(r *http.Request) (*http.Response, error) {
	u := *r.URL
	u.RawQuery, u.Fragment = "", ""
	if r.Method != http.MethodPost || u.String() != t.url {
		if r.Body != nil {
			_ = r.Body.Close()
		}
		return nil, &RefusedError{Method: r.Method, URL: u.Scheme + "://" + u.Host + u.Path}
	}
	return t.next.RoundTrip(r)
}

// TokenClient reaches the tenant's token endpoint and nothing else, and follows no redirect.
func (c Config) TokenClient(tenant string) *http.Client {
	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{RootCAs: c.RootCAs, MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		DisableKeepAlives:     true,
		Proxy:                 nil,
	}
	if c.Dial != nil {
		tr.DialContext = c.Dial
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &http.Client{
		Transport:     tokenOnly{url: TokenURL(tenant), next: tr},
		Timeout:       timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

type tokenAnswer struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    any    `json:"expires_in"`
	Error        string `json:"error"`
	Description  string `json:"error_description"`
	ErrorCodes   []int  `json:"error_codes"`
}

// Exchange trades the code for tokens, proving the sign-in with the PKCE verifier.
func Exchange(ctx context.Context, cfg Config, tenant string, app App, code, redirect, verifier string) (Tokens, error) {
	return token(ctx, cfg, tenant, url.Values{
		"grant_type": {"authorization_code"}, "client_id": {app.ClientID}, "client_secret": {app.ClientSecret}, "code": {code},
		"redirect_uri": {redirect}, "code_verifier": {verifier}, "scope": {strings.Join(Scopes, " ")},
	})
}

// Refresh trades the refresh token for new tokens, asking for the same scopes again.
func Refresh(ctx context.Context, cfg Config, tenant string, app App, refresh string) (Tokens, error) {
	t, err := token(ctx, cfg, tenant, url.Values{
		"grant_type": {"refresh_token"}, "client_id": {app.ClientID}, "client_secret": {app.ClientSecret},
		"refresh_token": {refresh}, "scope": {strings.Join(Scopes, " ")},
	})
	if err == nil && t.Refresh == "" {
		t.Refresh = refresh
	}
	return t, err
}

func token(ctx context.Context, cfg Config, tenant string, form url.Values) (Tokens, error) {
	if !ValidTenant(tenant) {
		return Tokens{}, fmt.Errorf("not a tenant: %q", tenant)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenURL(tenant), bytes.NewReader([]byte(form.Encode())))
	if err != nil {
		return Tokens{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := cfg.TokenClient(tenant).Do(req)
	if err != nil {
		var refused *RefusedError
		if errors.As(err, &refused) {
			return Tokens{}, refused
		}
		return Tokens{}, fmt.Errorf("the token endpoint did not answer: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if err != nil {
		return Tokens{}, err
	}
	var a tokenAnswer
	_ = json.Unmarshal(raw, &a)
	if res.StatusCode < 200 || res.StatusCode > 299 || a.AccessToken == "" {
		if res.StatusCode >= 500 && a.Error == "" {
			return Tokens{}, fmt.Errorf("the token endpoint answered %d", res.StatusCode)
		}
		return Tokens{}, NewError(res.StatusCode, a.Error, a.Description, a.ErrorCodes)
	}
	if !strings.EqualFold(a.TokenType, "bearer") {
		return Tokens{}, NewError(res.StatusCode, "unsupported_token_type", "", nil)
	}
	t := Tokens{Access: a.AccessToken, Refresh: a.RefreshToken, Scopes: strings.Fields(a.Scope)}
	if t.Scopes == nil {
		t.Scopes = []string{}
	}
	if n := seconds(a.ExpiresIn); n > 0 {
		t.Expires = cfg.now().Add(time.Duration(n) * time.Second)
	}
	return t, nil
}

// seconds reads expires_in, which Microsoft sends as a number and older endpoints as a string.
func seconds(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	}
	return 0
}

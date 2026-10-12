package microsoft_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/productdevbook/yuva/api/internal/migrate/microsoft"
	"github.com/productdevbook/yuva/api/internal/migrate/microsoft/microsofttest"
)

const redirect = "https://yuva.example.com/migrations/sign-in/callback"

func TestMicrosoftSignInScopes(t *testing.T) {
	u, err := url.Parse(microsoft.AuthorizeURL("organizations", "app-id", redirect, "state-1", "challenge-1", "support@contoso.example"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Host != "login.microsoftonline.com" || u.Path != "/organizations/oauth2/v2.0/authorize" {
		t.Fatalf("authorize endpoint %s%s", u.Host, u.Path)
	}
	if q.Get("scope") != "https://outlook.office.com/IMAP.AccessAsUser.All offline_access" {
		t.Fatalf("scope %q", q.Get("scope"))
	}
	if q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") != "challenge-1" ||
		q.Get("login_hint") != "support@contoso.example" || q.Get("redirect_uri") != redirect || q.Get("state") != "state-1" {
		t.Fatalf("authorize query %v", q)
	}
	cases := []struct {
		granted []string
		ok      bool
	}{
		{[]string{"https://outlook.office.com/IMAP.AccessAsUser.All"}, true},
		{[]string{"https://outlook.office.com/IMAP.AccessAsUser.All", "offline_access"}, true},
		{[]string{"IMAP.AccessAsUser.All", "openid", "profile", "email"}, true},
		{[]string{"https://outlook.office.com/IMAP.AccessAsUser.All", "https://outlook.office.com/SMTP.Send"}, false},
		{[]string{"https://outlook.office.com/IMAP.AccessAsUser.All", "https://outlook.office.com/Mail.ReadWrite"}, false},
		{[]string{"https://outlook.office.com/POP.AccessAsUser.All"}, false},
		{[]string{"offline_access"}, false},
		{nil, false},
	}
	for _, c := range cases {
		err := microsoft.CheckScopes(c.granted)
		var se *microsoft.ScopeError
		if (err == nil) != c.ok || (err != nil && !errors.As(err, &se)) {
			t.Errorf("CheckScopes(%q) = %v, want ok %v", c.granted, err, c.ok)
		}
	}
}

// dialRecorder fails every connection and remembers it was asked.
type dialRecorder struct {
	mu    sync.Mutex
	addrs []string
}

func (d *dialRecorder) dial(_ context.Context, _, addr string) (net.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.addrs = append(d.addrs, addr)
	return nil, errors.New("no network in this test")
}

func TestTokenClientReachesOnlyTheTokenEndpoint(t *testing.T) {
	rec := &dialRecorder{}
	client := microsoft.Config{Dial: rec.dial}.TokenClient("organizations")
	refused := []struct{ method, url string }{
		{http.MethodGet, "https://login.microsoftonline.com/organizations/oauth2/v2.0/token"},
		{http.MethodPut, "https://login.microsoftonline.com/organizations/oauth2/v2.0/token"},
		{http.MethodDelete, "https://login.microsoftonline.com/organizations/oauth2/v2.0/token"},
		{http.MethodPost, "https://login.microsoftonline.com/common/oauth2/v2.0/token"},
		{http.MethodPost, "https://login.microsoftonline.com/organizations/oauth2/v2.0/authorize"},
		{http.MethodPost, "https://login.microsoftonline.com/organizations/oauth2/v2.0/logout"},
		{http.MethodPost, "https://graph.microsoft.com/v1.0/me/sendMail"},
		{http.MethodPost, "https://outlook.office.com/api/v2.0/me/messages"},
		{http.MethodPost, "http://login.microsoftonline.com/organizations/oauth2/v2.0/token"},
	}
	for _, c := range refused {
		req, _ := http.NewRequest(c.method, c.url, strings.NewReader("x=1"))
		_, err := client.Do(req)
		var re *microsoft.RefusedError
		if !errors.As(err, &re) {
			t.Errorf("%s %s: %v, want refused", c.method, c.url, err)
		}
	}
	if len(rec.addrs) != 0 {
		t.Fatalf("refused requests dialled %v", rec.addrs)
	}
	req, _ := http.NewRequest(http.MethodPost, "https://login.microsoftonline.com/organizations/oauth2/v2.0/token?x=1", strings.NewReader("x=1"))
	if _, err := client.Do(req); err == nil || len(rec.addrs) != 1 || rec.addrs[0] != "login.microsoftonline.com:443" {
		t.Fatalf("the token endpoint was not dialled: %v %v", err, rec.addrs)
	}
}

func TestExchangeRefreshAndErrors(t *testing.T) {
	var mu sync.Mutex
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	ms := microsofttest.New(t, clock, "")
	ms.Register("app-id", "app-secret")
	cfg := microsoft.Config{Dial: ms.Dial, RootCAs: ms.Pool, Now: clock}
	app := microsoft.App{ClientID: "app-id", ClientSecret: "app-secret"}
	ctx := context.Background()

	signIn := func(verifierOK bool) (microsoft.Tokens, error) {
		verifier, challenge := microsoft.PKCE()
		back, err := ms.Authorize(microsoft.AuthorizeURL("organizations", app.ClientID, redirect, "s", challenge, "ece@contoso.example"))
		if err != nil {
			t.Fatal(err)
		}
		if !verifierOK {
			verifier, _ = microsoft.PKCE()
		}
		return microsoft.Exchange(ctx, cfg, "organizations", app, back.Query().Get("code"), redirect, verifier)
	}
	ms.Next(microsofttest.SignIn{Account: "ece@contoso.example"})
	if _, err := signIn(false); err == nil || !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("a wrong PKCE verifier: %v", err)
	}
	tokens, err := signIn(true)
	if err != nil {
		t.Fatal(err)
	}
	if got := microsoft.Accounts(tokens.Access); len(got) != 1 || got[0] != "ece@contoso.example" {
		t.Fatalf("accounts %v", got)
	}
	if !tokens.Expires.Equal(now.Add(time.Hour)) || tokens.Refresh == "" || microsoft.CheckScopes(tokens.Scopes) != nil {
		t.Fatalf("tokens %+v", tokens)
	}
	if _, err := microsoft.Exchange(ctx, cfg, "organizations", microsoft.App{ClientID: "app-id", ClientSecret: "wrong"}, "code", redirect, "v"); err == nil {
		t.Fatal("a wrong secret was accepted")
	}
	mu.Lock()
	now = now.Add(2 * time.Hour)
	mu.Unlock()
	fresh, err := microsoft.Refresh(ctx, cfg, "organizations", app, tokens.Refresh)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Access == tokens.Access || !fresh.Expires.Equal(now.Add(time.Hour)) {
		t.Fatalf("refresh %+v", fresh)
	}
	ms.RevokeRefresh()
	_, err = microsoft.Refresh(ctx, cfg, "organizations", app, fresh.Refresh)
	var me *microsoft.Error
	if !errors.As(err, &me) || me.Result() != microsoft.ResultTokenFailed || me.Status != http.StatusBadRequest {
		t.Fatalf("revoked refresh: %v", err)
	}
	if strings.Contains(err.Error(), fresh.Refresh) || strings.Contains(err.Error(), "Trace ID") {
		t.Fatalf("error text %q", err.Error())
	}
	for _, r := range ms.Requests() {
		if r.Method != http.MethodPost || r.Path != "/organizations/oauth2/v2.0/token" {
			t.Errorf("sent %s %s", r.Method, r.Path)
		}
		if r.Form.Get("scope") != "https://outlook.office.com/IMAP.AccessAsUser.All offline_access" {
			t.Errorf("token request scope %q", r.Form.Get("scope"))
		}
	}
}

// Microsoft's answers, as its error reference and the token endpoint word them, name what the
// person is told.
func TestErrorResults(t *testing.T) {
	cases := []struct {
		status            int
		code, description string
		codes             []int
		want              string
	}{
		{0, "access_denied", "AADSTS65004: User declined to consent to access the app.", nil, microsoft.ResultAccessDenied},
		{0, "access_denied", "The user has denied access to the scope requested by the client application.", nil, microsoft.ResultAccessDenied},
		{0, "consent_required", "AADSTS65001: The user or administrator has not consented to use the application with ID 'app-id' named 'Yuva'. Send an interactive authorization request for this user and resource.", nil, microsoft.ResultAdminConsentRequired},
		{0, "access_denied", "AADSTS90094: The grant requires admin permission.", nil, microsoft.ResultAdminConsentRequired},
		{0, "access_denied", "AADSTS90095: Admin consent is required for the permissions requested by this application. An admin consent request may be sent to the admin.", nil, microsoft.ResultAdminConsentRequired},
		{0, "access_denied", "AADSTS53003: Access has been blocked by Conditional Access policies. The access policy does not allow token issuance.", nil, microsoft.ResultConditionalAccess},
		{0, "access_denied", "AADSTS53000: Device is not in required device state: compliant.", nil, microsoft.ResultConditionalAccess},
		{0, "interaction_required", "AADSTS50079: Due to a configuration change made by your administrator, you must enroll in multi-factor authentication.", nil, microsoft.ResultConsentRequired},
		{400, "invalid_grant", "AADSTS65001: The user or administrator has not consented to use the application.", []int{65001}, microsoft.ResultAdminConsentRequired},
		{400, "invalid_grant", "AADSTS70000: The provided authorization code is invalid or has expired.", []int{70000}, microsoft.ResultTokenFailed},
		{401, "invalid_client", "AADSTS7000215: Invalid client secret provided.", []int{7000215}, microsoft.ResultTokenFailed},
		{0, "invalid_request", "AADSTS50194: Application 'app-id' is not configured as a multi-tenant application.", nil, microsoft.ResultTokenFailed},
	}
	for _, c := range cases {
		e := microsoft.NewError(c.status, c.code, c.description+"\r\nTrace ID: 1\r\nCorrelation ID: 2\r\nTimestamp: 3", c.codes)
		if got := e.Result(); got != c.want {
			t.Errorf("%s %q: %s, want %s", c.code, c.description, got, c.want)
		}
		if strings.Contains(e.Error(), "Trace ID") {
			t.Errorf("trace ids kept in %q", e.Error())
		}
	}
}

func TestAccounts(t *testing.T) {
	body := base64.RawURLEncoding.EncodeToString([]byte(`{"upn":"Ece@Contoso.example","unique_name":"live.com#ece@contoso.example","email":"ece@contoso.example"}`))
	if got := microsoft.Accounts("eyJhbGciOiJSUzI1NiJ9." + body + ".sig"); len(got) != 1 || got[0] != "ece@contoso.example" {
		t.Fatalf("work token names %v", got)
	}
	for _, opaque := range []string{"EwBwA8l6BAAU", "a.b.c.d.e", "a." + base64.RawURLEncoding.EncodeToString([]byte("not json")) + ".c", ""} {
		if got := microsoft.Accounts(opaque); got != nil {
			t.Errorf("%q names %v", opaque, got)
		}
	}
}

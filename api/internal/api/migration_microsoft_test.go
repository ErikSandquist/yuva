package api_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/productdevbook/yuva/api/internal/api"
	"github.com/productdevbook/yuva/api/internal/migrate/microsoft/microsofttest"
)

const imapScope = "https://outlook.office.com/IMAP.AccessAsUser.All"

// mxResolver answers MX records from a table and nothing else.
type mxResolver map[string]string

func (r mxResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return nil, errors.New("no addresses in this test")
}

func (r mxResolver) LookupMX(_ context.Context, name string) ([]*net.MX, error) {
	if h, ok := r[name]; ok {
		return []*net.MX{{Host: h, Pref: 10}}, nil
	}
	return nil, errors.New("no MX")
}

func (r mxResolver) LookupSRV(context.Context, string, string, string) (string, []*net.SRV, error) {
	return "", nil, errors.New("no SRV")
}

var testMX = mxResolver{"contoso.example": "contoso-example.mail.protection.outlook.com."}

type msRun struct {
	h  *harness
	tm team
	ms *microsofttest.Server
}

// newMSRun starts a server whose Microsoft sign-in reaches the fake identity platform; imap, when
// set, is where outlook.office365.com:993 leads.
func newMSRun(t *testing.T, publicURL string, configure func(*api.MigrationSettings), imap func(ctx context.Context) (net.Conn, error)) *msRun {
	t.Helper()
	var ms *microsofttest.Server
	h := newHarnessWith(t, func(d *api.Deps) {
		ms = microsofttest.New(t, d.Now, "")
		d.Migrations = api.MigrationSettings{
			AllowPrivate: true, TestTimeout: 5 * time.Second, RootCAs: ms.Pool, Resolver: testMX,
			MicrosoftDial: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if strings.HasPrefix(addr, "outlook.office365.com:") {
					if imap == nil {
						return nil, errors.New("no IMAP server in this test")
					}
					return imap(ctx)
				}
				return ms.Dial(ctx, network, addr)
			},
		}
		if configure != nil {
			configure(&d.Migrations)
		}
		if publicURL != "" {
			d.Auth.PublicURL = publicURL
		}
	})
	return &msRun{h: h, tm: newTeam(t, h), ms: ms}
}

func (r *msRun) preset(t *testing.T, c *client) map[string]any {
	t.Helper()
	for _, p := range c.expect(http.StatusOK, "GET", "/v1/migrations/presets", nil).body["items"].([]any) {
		if p.(map[string]any)["id"] == "microsoft" {
			return p.(map[string]any)
		}
	}
	t.Fatal("no microsoft preset")
	return nil
}

// signIn starts a sign-in, lets the fake platform answer as next says, and follows Microsoft's
// redirect back in browser; it answers the result the panel gets.
func (r *msRun) signIn(t *testing.T, browser *client, id string, body map[string]any, next microsofttest.SignIn) (string, url.Values) {
	t.Helper()
	res := r.tm.owner.expect(http.StatusOK, "POST", "/v1/migrations/"+id+"/sign-in", body)
	if strings.Contains(string(res.raw), "team-secret") {
		t.Fatalf("the sign-in answer carries the client secret: %s", res.raw)
	}
	authorize, _ := url.Parse(res.str("authorize_url"))
	r.ms.Next(next)
	back, err := r.ms.Authorize(res.str("authorize_url"))
	if err != nil {
		t.Fatal(err)
	}
	return r.follow(t, browser, back), authorize.Query()
}

func (r *msRun) follow(t *testing.T, browser *client, back *url.URL) string {
	t.Helper()
	target := r.h.url + back.Path + "?" + back.RawQuery
	res, err := noRedirect(browser).Get(target)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	loc, err := url.Parse(res.Header.Get("Location"))
	if res.StatusCode != http.StatusSeeOther || err != nil {
		t.Fatalf("callback: %d %v", res.StatusCode, err)
	}
	return loc.Query().Get("result")
}

func (r *msRun) stored(t *testing.T, id string) int {
	t.Helper()
	return r.h.count("SELECT count(*) FROM migration_credentials WHERE workspace_id = $1 AND migration_id = $2", r.tm.ws, id)
}

func TestMicrosoftSignInWithTheTeamsApp(t *testing.T) {
	r := newMSRun(t, "", nil, nil)
	r.ms.Register("team-app", "team-secret")
	owner := r.tm.owner
	if p := r.preset(t, owner); p["available"] != true || p["team_client"] != true || p["redirect_url"] != testOrigin+"/migrations/sign-in/callback" ||
		len(p["scopes"].([]any)) != 2 || p["scopes"].([]any)[0] != imapScope || p["scopes"].([]any)[1] != "offline_access" {
		t.Fatalf("preset %v", p)
	}
	det := owner.expect(http.StatusOK, "POST", "/v1/migrations/detect", map[string]any{"address": "help@contoso.example"})
	if det.str("preset") != "microsoft" || det.str("found_by") != "mx" || det.body["available"] != true || det.body["note"] != nil {
		t.Fatalf("detect: %s", det.raw)
	}
	m := owner.expect(http.StatusCreated, "POST", "/v1/migrations", map[string]any{"preset": "microsoft", "source_address": "Help@Contoso.example"})
	id, path := m.str("id"), "/v1/migrations/"+m.str("id")
	if got := m.body["actions"].([]any); len(got) != 3 || got[0] != "update" || got[1] != "sign_in" || got[2] != "delete" {
		t.Fatalf("actions %v", got)
	}
	owner.expectProblem(http.StatusConflict, "migration_state", "PUT", path+"/credentials", map[string]any{"username": "help@contoso.example", "password": "x"})
	owner.expectProblem(http.StatusConflict, "migration_state", "POST", path+"/test", nil)
	owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", path+"/sign-in", map[string]any{})
	owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", path+"/sign-in", map[string]any{"client_id": "team-app"})
	owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", path+"/sign-in", map[string]any{"tenant": "contoso.example"})
	owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", path+"/sign-in", map[string]any{"client_id": "team-app", "client_secret": "team-secret", "tenant": "bad tenant"})
	r.tm.agent.expectProblem(http.StatusForbidden, "forbidden", "POST", path+"/sign-in", map[string]any{"client_id": "team-app", "client_secret": "team-secret"})
	other := newTeam(t, r.h)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", path+"/sign-in", map[string]any{"client_id": "team-app", "client_secret": "team-secret"})

	app := map[string]any{"client_id": "team-app", "client_secret": "team-secret"}
	result, q := r.signIn(t, owner, id, app, microsofttest.SignIn{Error: "access_denied", Description: "AADSTS65004: User declined to consent to access the app."})
	if result != "access_denied" || q.Get("scope") != imapScope+" offline_access" || q.Get("login_hint") != "help@contoso.example" ||
		q.Get("client_id") != "team-app" || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("declined: %s %v", result, q)
	}
	if tenants := r.ms.Tenants(); tenants[len(tenants)-1] != "organizations" {
		t.Fatalf("a Microsoft 365 domain signed in at %v", tenants)
	}
	if got := owner.expect(http.StatusOK, "GET", path, nil); got.str("sign_in_result") != "access_denied" || got.body["admin_consent_url"] != nil {
		t.Fatalf("after declining: %s", got.raw)
	}

	result, _ = r.signIn(t, owner, id, app, microsofttest.SignIn{Error: "consent_required",
		Description: "AADSTS65001: The user or administrator has not consented to use the application with ID 'team-app' named 'Yuva import'."})
	got := owner.expect(http.StatusOK, "GET", path, nil)
	admin, _ := url.Parse(got.str("admin_consent_url"))
	if result != "admin_consent_required" || got.str("sign_in_result") != "admin_consent_required" || admin == nil ||
		admin.Host != "login.microsoftonline.com" || admin.Path != "/contoso.example/v2.0/adminconsent" ||
		admin.Query().Get("client_id") != "team-app" || admin.Query().Get("scope") != imapScope ||
		admin.Query().Get("redirect_uri") != testOrigin+"/migrations/sign-in/callback" {
		t.Fatalf("admin consent: %s %s", result, got.raw)
	}
	landing := r.follow(t, owner, &url.URL{Path: "/migrations/sign-in/callback", RawQuery: "admin_consent=True&tenant=contoso.example&state=admin_consent&scope=" + url.QueryEscape(imapScope)})
	if landing != "admin_consented" {
		t.Fatalf("admin landing: %s", landing)
	}
	result, _ = r.signIn(t, owner, id, app, microsofttest.SignIn{Error: "access_denied", Description: "AADSTS53003: Access has been blocked by Conditional Access policies."})
	if result != "conditional_access" {
		t.Fatalf("conditional access: %s", result)
	}

	result, _ = r.signIn(t, owner, id, app, microsofttest.SignIn{Account: "help@contoso.example", Scope: imapScope + " https://outlook.office.com/SMTP.Send"})
	if result != "scope_too_broad" || r.stored(t, id) != 0 {
		t.Fatalf("a token that can also send mail: %s, %d stored", result, r.stored(t, id))
	}
	result, _ = r.signIn(t, owner, id, app, microsofttest.SignIn{Account: "ceo@contoso.example"})
	if result != "account_mismatch" || r.stored(t, id) != 0 {
		t.Fatalf("another account: %s, %d stored", result, r.stored(t, id))
	}
	result, _ = r.signIn(t, owner, id, app, microsofttest.SignIn{Account: "help@contoso.example", Scope: imapScope + " offline_access"})
	if result != "source_unreachable" || r.stored(t, id) != 0 {
		t.Fatalf("no IMAP server: %s, %d stored", result, r.stored(t, id))
	}

	res := owner.expect(http.StatusOK, "POST", path+"/sign-in", app)
	r.ms.Next(microsofttest.SignIn{Account: "help@contoso.example"})
	back, err := r.ms.Authorize(res.str("authorize_url"))
	if err != nil {
		t.Fatal(err)
	}
	if result := r.follow(t, other.owner, back); result != "session_mismatch" {
		t.Fatalf("another member's browser: %s", result)
	}
	if result := r.follow(t, owner, back); result != "expired" {
		t.Fatalf("a used sign-in: %s", result)
	}
	if n := r.h.count("SELECT count(*) FROM migration_sign_ins WHERE workspace_id = $1", r.tm.ws); n != 0 {
		t.Fatalf("%d sign-ins left", n)
	}
	audits := r.h.count("SELECT count(*) FROM migration_audit WHERE workspace_id = $1 AND migration_id = $2 AND action = 'sign_in_failed'", r.tm.ws, id)
	if audits != 7 {
		t.Fatalf("%d failed sign-ins audited", audits)
	}
	for _, v := range r.ms.Issued() {
		for _, q := range []string{
			"SELECT count(*) FROM migration_audit WHERE workspace_id = $1 AND detail::text LIKE '%' || $2 || '%'",
			"SELECT count(*) FROM events WHERE workspace_id = $1 AND payload::text LIKE '%' || $2 || '%'",
			"SELECT count(*) FROM migrations WHERE workspace_id = $1 AND source_state::text LIKE '%' || $2 || '%'",
		} {
			if n := r.h.count(q, r.tm.ws, v); n != 0 {
				t.Fatalf("a token is stored in the clear: %s", q)
			}
		}
	}
	for _, q := range []string{
		"SELECT count(*) FROM migration_audit WHERE workspace_id = $1 AND detail::text LIKE '%team-secret%'",
		"SELECT count(*) FROM events WHERE workspace_id = $1 AND payload::text LIKE '%team-secret%'",
	} {
		if n := r.h.count(q, r.tm.ws); n != 0 {
			t.Fatalf("the client secret is stored in the clear: %s", q)
		}
	}
}

func TestMicrosoftSignInWithTheServersApp(t *testing.T) {
	r := newMSRun(t, "", func(s *api.MigrationSettings) {
		s.MicrosoftClientID, s.MicrosoftClientSecret, s.MicrosoftTenant = "server-app", "server-secret", "common"
	}, nil)
	r.ms.Register("server-app", "server-secret")
	r.ms.Register("team-app", "team-secret")
	owner := r.tm.owner
	if p := r.preset(t, owner); p["available"] != true || p["team_client"] != false {
		t.Fatalf("preset %v", p)
	}
	id := owner.expect(http.StatusCreated, "POST", "/v1/migrations", map[string]any{"preset": "microsoft", "source_address": "someone@outlook.com"}).str("id")
	result, q := r.signIn(t, owner, id, map[string]any{}, microsofttest.SignIn{Account: "someone@outlook.com", Personal: true, Scope: imapScope + " https://outlook.office.com/Mail.Send"})
	if result != "scope_too_broad" || q.Get("client_id") != "server-app" {
		t.Fatalf("the server's app: %s %v", result, q)
	}
	if tenants := r.ms.Tenants(); tenants[len(tenants)-1] != "consumers" {
		t.Fatalf("an Outlook.com address signed in at %v", tenants)
	}
	result, q = r.signIn(t, owner, id, map[string]any{"client_id": "team-app", "client_secret": "team-secret", "tenant": "contoso.example"},
		microsofttest.SignIn{Account: "someone@outlook.com", Personal: true})
	if result != "source_unreachable" || q.Get("client_id") != "team-app" {
		t.Fatalf("the team's own app: %s %v", result, q)
	}
	if tenants := r.ms.Tenants(); tenants[len(tenants)-1] != "contoso.example" {
		t.Fatalf("a single-tenant app signed in at %v", tenants)
	}
	r.ms.ExchangeError = "invalid_client AADSTS7000215: Invalid client secret provided."
	result, _ = r.signIn(t, owner, id, map[string]any{}, microsofttest.SignIn{Account: "someone@outlook.com", Personal: true})
	if result != "token_failed" || r.stored(t, id) != 0 {
		t.Fatalf("a refused code: %s", result)
	}
}

func TestMicrosoftNotOfferedWithoutAnyApp(t *testing.T) {
	r := newMSRun(t, "http://yuva.example.net", nil, nil)
	owner := r.tm.owner
	if p := r.preset(t, owner); p["available"] != false || p["team_client"] != true {
		t.Fatalf("preset %v", p)
	}
	owner.expectProblem(http.StatusConflict, "provider_not_configured", "POST", "/v1/migrations", map[string]any{"preset": "microsoft", "source_address": "a@outlook.com"})
	det := owner.expect(http.StatusOK, "POST", "/v1/migrations/detect", map[string]any{"address": "someone@hotmail.com"})
	if det.str("preset") != "microsoft" || det.body["available"] != false || det.str("note") != "microsoft_later" {
		t.Fatalf("detect: %s", det.raw)
	}
}

func TestMicrosoftCreateMigrationOverMCP(t *testing.T) {
	r := newMSRun(t, "", nil, nil)
	cs := r.h.mcpSession(r.h.oauthToken(r.tm.owner, r.tm.ws, "Claude Code", testOrigin+"/mcp", []string{"inboxes:read", "migrations:manage"}))
	created, errText := callTool(t, cs, "create_migration", map[string]any{"address": "help@contoso.example"})
	if errText != "" {
		t.Fatal(errText)
	}
	if created["provider"] != "microsoft" || created["state"] != "draft" || !strings.Contains(created["connect_url"].(string), "/migrations/"+created["id"].(string)) {
		t.Fatalf("create_migration: %v", created)
	}
}

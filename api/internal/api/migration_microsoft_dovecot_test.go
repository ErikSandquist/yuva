package api_test

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/productdevbook/yuva/api/internal/api"
	"github.com/productdevbook/yuva/api/internal/migrate/microsoft/microsofttest"
)

// xoauth2Target reads YUVA_TEST_XOAUTH2_URL (imaps://:password@127.0.0.1:59994, the migrate-test
// Dovecot that signs in XOAUTH2 tokens by asking the fake identity platform), its certificate
// for outlook.office365.com (YUVA_TEST_XOAUTH2_CERT), the address that Dovecot asks about tokens
// (YUVA_TEST_XOAUTH2_INTROSPECTION, 127.0.0.1:59980) and YUVA_TEST_IMAP_FIXTURES, which it serves.
func xoauth2Target(t *testing.T) (addr, pass string, cert []byte, introspection, fixtures string) {
	t.Helper()
	raw, certFile := os.Getenv("YUVA_TEST_XOAUTH2_URL"), os.Getenv("YUVA_TEST_XOAUTH2_CERT")
	introspection, fixtures = os.Getenv("YUVA_TEST_XOAUTH2_INTROSPECTION"), os.Getenv("YUVA_TEST_IMAP_FIXTURES")
	if raw == "" || certFile == "" || introspection == "" || fixtures == "" {
		t.Skip("YUVA_TEST_XOAUTH2_URL, YUVA_TEST_XOAUTH2_CERT, YUVA_TEST_XOAUTH2_INTROSPECTION and YUVA_TEST_IMAP_FIXTURES are not set")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "imaps" {
		t.Fatalf("YUVA_TEST_XOAUTH2_URL must be imaps://: %v", err)
	}
	pass, _ = u.User.Password()
	if cert, err = os.ReadFile(certFile); err != nil {
		t.Fatal(err)
	}
	return u.Host, pass, cert, introspection, fixtures
}

// TestDovecotMicrosoftXOAUTH2 moves a mailbox through Sign in with Microsoft, against the fake
// identity platform and a Dovecot that takes its tokens over AUTHENTICATE XOAUTH2: a sign-in to
// another account is refused, the copy outlives the access token and refreshes it, a refresh
// Microsoft refuses stops the migration until the person signs in again, and the mailbox ends as
// it began.
func TestDovecotMicrosoftXOAUTH2(t *testing.T) {
	addr, pass, cert, introspection, fixtures := xoauth2Target(t)
	var ms *microsofttest.Server
	logs := &logBuffer{}
	h := newHarnessWith(t, func(d *api.Deps) {
		ms = microsofttest.New(t, d.Now, introspection)
		pool := ms.Pool.Clone()
		if !pool.AppendCertsFromPEM(cert) {
			t.Fatal("YUVA_TEST_XOAUTH2_CERT holds no certificate")
		}
		d.Log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
		d.Migrations = api.MigrationSettings{
			AllowPrivate: true, TestTimeout: 10 * time.Second, RootCAs: pool, Resolver: testMX,
			MicrosoftClientID: "server-app", MicrosoftClientSecret: "server-secret", MicrosoftTenant: "common",
			MicrosoftDial: func(ctx context.Context, network, a string) (net.Conn, error) {
				if a == "outlook.office365.com:993" {
					var dialer net.Dialer
					return dialer.DialContext(ctx, "tcp", addr)
				}
				return ms.Dial(ctx, network, a)
			},
		}
		d.Attachments = api.AttachmentSettings{MaxBytes: 25 << 20, Types: []string{
			"image/png", "image/jpeg", "image/gif", "application/pdf", "text/plain", "text/csv", "application/zip", "video/mp4",
		}}
	})
	ms.Register("server-app", "server-secret")
	ms.Lifetime = 20 * time.Minute
	r := &msRun{h: h, tm: newTeam(t, h), ms: ms}
	d := &dovecotRun{h: h, tm: r.tm, fixtures: fixtures}
	owner, ctx := r.tm.owner, context.Background()

	want := expectedFixture(t, fixtures, "acme")
	box := copyMailbox(t, fixtures, "acme")
	address := box + "@example.com"
	before := imapSnapshot(t, addr, address, pass)
	channel := owner.expect(http.StatusCreated, "POST", "/v1/inboxes/"+r.tm.inbox+"/channels", map[string]any{
		"kind": "email", "name": "Support mail", "email": map[string]any{"address": address, "display_name": "Support"},
	}).str("id")
	id := owner.expect(http.StatusCreated, "POST", "/v1/migrations", map[string]any{
		"preset": "microsoft", "source_address": address, "channel_id": channel, "options": map[string]any{"team_addresses": want.TeamAddresses},
	}).str("id")
	path := "/v1/migrations/" + id

	if result, _ := r.signIn(t, owner, id, map[string]any{}, microsofttest.SignIn{Account: "someone-else@example.com", Personal: true}); result != "account_mismatch" || r.stored(t, id) != 0 {
		t.Fatalf("a personal account signed in as someone else: %s, %d stored", result, r.stored(t, id))
	}
	if result, _ := r.signIn(t, owner, id, map[string]any{}, microsofttest.SignIn{Account: address}); result != "connected" {
		t.Fatalf("sign-in: %s %s", result, logs.String())
	}
	m := d.migration(id)
	cred := m.body["credentials"].(map[string]any)
	server := cred["server"].(map[string]any)
	if m.str("state") != "connected" || m.str("sign_in_result") != "connected" || cred["kind"] != "oauth" || server["host"] != "outlook.office365.com" ||
		num(server["port"]) != 993 || cred["expires_at"] == nil || cred["scopes"].([]any)[0] != imapScope {
		t.Fatalf("after the sign-in: %s", m.raw)
	}
	folders := map[string]map[string]any{}
	for _, f := range owner.expect(http.StatusOK, "GET", path+"/folders", nil).body["items"].([]any) {
		folders[f.(map[string]any)["name"].(string)] = f.(map[string]any)
	}
	for _, f := range want.Folders {
		if got := folders[f.Name]; got == nil || got["kind"] != f.Kind || num(got["messages"]) != f.Messages {
			t.Errorf("folder %s after the sign-in: %v, want %+v", f.Name, got, f)
		}
	}
	if res := owner.expect(http.StatusOK, "POST", path+"/test", nil); res.body["ok"] != true {
		t.Fatalf("connection test: %s", res.raw)
	}

	owner.expect(http.StatusOK, "POST", path+"/scan", nil)
	if err := d.work(t, ctx); err != nil {
		t.Fatal(err)
	}
	if m = d.migration(id); m.str("state") != "ready" {
		t.Fatalf("after the scan: %s", m.raw)
	}
	checkDryRun(t, m.body["estimate"].(map[string]any), want)

	h.clock.Advance(30 * time.Minute)
	refreshed := ms.Refreshes()
	owner.expect(http.StatusOK, "POST", path+"/start", nil)
	if err := d.work(t, ctx); err != nil {
		t.Fatal(err)
	}
	m = d.migration(id)
	counts := m.body["counts"].(map[string]any)
	if m.str("state") != "syncing" || num(counts["imported"]) != want.MessagesIn+want.MessagesOut || num(counts["failed"]) != 0 {
		t.Fatalf("after the copy: %s", m.raw)
	}
	if ms.Refreshes() == refreshed {
		t.Fatal("the copy began with an expired token and did not refresh it")
	}
	if at, _ := time.Parse(time.RFC3339Nano, m.body["credentials"].(map[string]any)["expires_at"].(string)); !at.After(h.clock.Now()) {
		t.Fatalf("the refreshed token was not stored: expires %v", at)
	}
	conversations := func() int {
		return h.count("SELECT count(*) FROM conversations WHERE workspace_id = $1 AND migration_id = $2", r.tm.ws, id)
	}
	if n := conversations(); n != want.Conversations {
		t.Fatalf("%d conversations, want %d", n, want.Conversations)
	}

	h.clock.Advance(3 * time.Minute)
	if err := d.work(t, ctx); err != nil {
		t.Fatal(err)
	}
	if m = d.migration(id); m.str("state") != "syncing" || num(m.body["counts"].(map[string]any)["imported"]) != want.MessagesIn+want.MessagesOut || conversations() != want.Conversations {
		t.Fatalf("a sync pass added something: %s", m.raw)
	}

	ms.RevokeRefresh()
	h.clock.Advance(time.Hour)
	if err := d.work(t, ctx); err != nil {
		t.Fatal(err)
	}
	m = d.migration(id)
	if m.str("state") != "needs_attention" || !strings.Contains(m.str("state_reason"), "sign in with Microsoft again") || !slices.Contains(strs(m.body["actions"]), "sign_in") {
		t.Fatalf("after a refused refresh: %s", m.raw)
	}
	errs := owner.expect(http.StatusOK, "GET", path+"/errors", nil).body["items"].([]any)
	if !slices.ContainsFunc(errs, func(e any) bool { return e.(map[string]any)["code"] == "token_refresh_failed" }) {
		t.Fatalf("errors: %v", errs)
	}
	res := owner.expect(http.StatusOK, "POST", path+"/test", nil)
	if steps := res.body["steps"].([]any); res.body["ok"] != false || steps[len(steps)-1].(map[string]any)["code"] != "token_refresh_failed" {
		t.Fatalf("test after a refused refresh: %s", res.raw)
	}
	if result, _ := r.signIn(t, owner, id, map[string]any{}, microsofttest.SignIn{Account: address}); result != "connected" {
		t.Fatalf("signing in again: %s", result)
	}
	if m = d.migration(id); m.str("state") != "needs_attention" {
		t.Fatalf("after signing in again: %s", m.raw)
	}
	owner.expect(http.StatusOK, "POST", path+"/resume", nil)
	if err := d.work(t, ctx); err != nil {
		t.Fatal(err)
	}
	if m = d.migration(id); m.str("state") != "syncing" || conversations() != want.Conversations {
		t.Fatalf("after resuming: %s", m.raw)
	}
	if m = owner.expect(http.StatusOK, "POST", path+"/finish", map[string]any{"force": true}); m.str("state") != "completed" || m.body["credentials_deleted_at"] == nil {
		t.Fatalf("finish: %s", m.raw)
	}

	if after := imapSnapshot(t, addr, address, pass); !reflect.DeepEqual(before, after) {
		t.Fatalf("the mailbox changed:\nbefore %v\nafter  %v", before, after)
	}
	cmds := rawlogCommands(t, fixtures, box)
	allowed := []string{"CAPABILITY", "NOOP", "LOGOUT", "STARTTLS", "LOGIN", "AUTHENTICATE", "LIST", "STATUS", "EXAMINE", "UNSELECT", "SEARCH", "UID SEARCH", "FETCH", "UID FETCH"}
	for _, c := range cmds {
		if !slices.Contains(allowed, c) {
			t.Errorf("the mailbox was sent %s", c)
		}
	}
	if ms.Introspections() == 0 {
		t.Fatal("Dovecot never asked about a token")
	}
	for _, v := range ms.Issued() {
		if strings.Contains(logs.String(), v) {
			t.Fatal("a token is in the logs")
		}
		for _, q := range []string{
			"SELECT count(*) FROM migration_errors WHERE workspace_id = $1 AND detail LIKE '%' || $2 || '%'",
			"SELECT count(*) FROM migration_audit WHERE workspace_id = $1 AND detail::text LIKE '%' || $2 || '%'",
			"SELECT count(*) FROM events WHERE workspace_id = $1 AND payload::text LIKE '%' || $2 || '%'",
			"SELECT count(*) FROM river_job WHERE args->>'workspace_id' = $1::text AND args::text LIKE '%' || $2 || '%'",
		} {
			if n := h.count(q, r.tm.ws, v); n != 0 {
				t.Fatalf("a token is stored in the clear: %s", q)
			}
		}
	}
	if strings.Contains(logs.String(), "server-secret") {
		t.Fatal("the app's secret is in the logs")
	}
}

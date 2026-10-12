package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/productdevbook/yuva/api/internal/api"
	"github.com/productdevbook/yuva/api/internal/migrate/helpdesk/helpdesktest"
)

type deskRun struct {
	h     *harness
	tm    team
	desk  *helpdesktest.Server
	owner string
}

func newDeskRun(t *testing.T) *deskRun {
	t.Helper()
	var desk *helpdesktest.Server
	var hp *harness
	h := newHarnessWith(t, func(d *api.Deps) {
		desk = helpdesktest.New(t, d.Now)
		d.Migrations = api.MigrationSettings{
			AllowPrivate: true, TestTimeout: 5 * time.Second, HelpdeskDial: desk.Dial, RootCAs: desk.Pool,
			HelpdeskSleep: func(_ context.Context, dur time.Duration) error {
				hp.clock.Advance(dur)
				return nil
			},
		}
		d.Attachments = api.AttachmentSettings{MaxBytes: 1 << 20, Types: []string{"image/png", "text/plain"}}
	})
	hp = h
	tm := newTeam(t, h)
	me := tm.owner.expect(http.StatusOK, "GET", "/v1/me", nil)
	owner := me.body["memberships"].([]any)[0].(map[string]any)["member_id"].(string)
	return &deskRun{h: h, tm: tm, desk: desk, owner: owner}
}

// emailChannel adds an e-mail channel with a unique address to inbox.
func (d *deskRun) emailChannel(t *testing.T, inbox, name string) (string, string) {
	t.Helper()
	address := unique(name) + "@acme.example.com"
	channel := d.tm.owner.expect(http.StatusCreated, "POST", "/v1/inboxes/"+inbox+"/channels", map[string]any{
		"kind": "email", "name": name, "email": map[string]any{"address": address, "display_name": name},
	}).str("id")
	return address, channel
}

// noRedirect is a browser that stops at the redirect, with the member's session.
func noRedirect(c *client) *http.Client {
	return &http.Client{Jar: c.http.Jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// callback is the help desk sending the browser back; it answers the result the panel gets.
func (d *deskRun) callback(t *testing.T, browser *client, query url.Values) string {
	t.Helper()
	res, err := noRedirect(browser).Get(d.h.url + "/migrations/sign-in/callback?" + query.Encode())
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("callback: status %d", res.StatusCode)
	}
	loc, err := url.Parse(res.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return loc.Query().Get("result")
}

// signIn starts a sign-in and returns its authorize URL's query.
func (d *deskRun) signIn(t *testing.T, id string, body map[string]any) url.Values {
	t.Helper()
	r := d.tm.owner.expect(http.StatusOK, "POST", "/v1/migrations/"+id+"/sign-in", body)
	u, err := url.Parse(r.str("authorize_url"))
	if err != nil {
		t.Fatal(err)
	}
	if r.str("redirect_url") != testOrigin+"/migrations/sign-in/callback" {
		t.Fatalf("redirect_url %s", r.str("redirect_url"))
	}
	return u.Query()
}

// work runs the workspace's migration jobs as River would, moving the clock to each wait, until
// stop says so.
func (d *deskRun) work(t *testing.T, stop func() bool) {
	t.Helper()
	ctx := context.Background()
	for range 2000 {
		if stop() {
			return
		}
		var jobs []riverJob
		for _, kind := range []string{"migration_scan", "migration_copy", "migration_sync"} {
			jobs = append(jobs, d.h.jobs(kind, d.tm.ws)...)
		}
		if len(jobs) == 0 {
			var state, reason, errs string
			_ = d.h.st.Pool.QueryRow(ctx, "SELECT state, state_reason FROM migrations WHERE workspace_id = $1 ORDER BY created_at DESC LIMIT 1", d.tm.ws).Scan(&state, &reason)
			_ = d.h.st.Pool.QueryRow(ctx, "SELECT coalesce(string_agg(code || ' ' || detail, '; '), '') FROM migration_errors WHERE workspace_id = $1", d.tm.ws).Scan(&errs)
			t.Fatalf("no migration job is left and the migration did not get there: %s %q %s", state, reason, errs)
		}
		j := jobs[0]
		var a api.MigrationJobArgs
		if err := json.Unmarshal(j.args, &a); err != nil {
			t.Fatal(err)
		}
		res, err := d.h.srv.RunMigrationSlice(ctx, a, j.id, 20*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case res.Done:
			d.h.dropJob(j.id)
		case res.Until.After(d.h.clock.Now()):
			d.h.clock.Advance(res.Until.Sub(d.h.clock.Now()))
		}
	}
	t.Fatal("the migration did not get there in 2000 slices")
}

// syncPass moves the clock past the next catch-up sync and runs until it read the help desk.
func (d *deskRun) syncPass(t *testing.T, id string) {
	t.Helper()
	prev := d.str(t, "SELECT coalesce(synced_at::text, '') FROM migrations WHERE workspace_id = $1 AND id = $2", d.tm.ws, id)
	d.h.clock.Advance(3 * time.Minute)
	d.work(t, func() bool {
		return d.str(t, "SELECT coalesce(synced_at::text, '') FROM migrations WHERE workspace_id = $1 AND id = $2", d.tm.ws, id) != prev
	})
}

func (d *deskRun) state(id string) string {
	return d.tm.owner.expect(http.StatusOK, "GET", "/v1/migrations/"+id, nil).str("state")
}

func (d *deskRun) str(t *testing.T, query string, args ...any) string {
	t.Helper()
	var s string
	if err := d.h.st.Pool.QueryRow(context.Background(), query, args...).Scan(&s); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return s
}

// checkRequests holds the recorded requests to the read-only rules: GETs only, besides the token
// exchange, within the budget, with the token only where it belongs.
func (d *deskRun) checkRequests(t *testing.T, apiHost, tokenPath string, limit, incremental int) {
	t.Helper()
	reqs := d.desk.Requests()
	for i, r := range reqs {
		if r.Method != http.MethodGet && !(r.Method == http.MethodPost && r.Host == apiHost && r.Path == tokenPath) {
			t.Errorf("%s %s%s was sent", r.Method, r.Host, r.Path)
		}
		if r.Host == helpdesktest.OutsideHost && r.Auth != "" {
			t.Errorf("an attachment outside the help desk was fetched with the token")
		}
		if r.Host != apiHost || r.Method != http.MethodGet {
			continue
		}
		all, inc := 0, 0
		for _, o := range reqs[:i+1] {
			if o.Host == apiHost && o.Method == http.MethodGet && o.At.After(r.At.Add(-time.Minute)) {
				all++
				if strings.Contains(o.Path, "/incremental/") {
					inc++
				}
			}
		}
		if all > limit || (incremental > 0 && inc > incremental) {
			t.Errorf("the minute up to %s saw %d requests, %d incremental", r.At.Format(time.TimeOnly), all, inc)
		}
	}
}

func TestZendeskMigration(t *testing.T) {
	d := newDeskRun(t)
	address, channel := d.emailChannel(t, d.tm.inbox, "support")
	billingInbox := d.tm.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Billing", "slug": "billing"}).body["inbox"].(map[string]any)["id"].(string)
	_, billingChannel := d.emailChannel(t, billingInbox, "billing")
	id := d.tm.owner.expect(http.StatusCreated, "POST", "/v1/migrations", map[string]any{
		"source": "zendesk", "preset": "zendesk", "source_address": address, "channel_id": channel,
	}).str("id")
	path := "/v1/migrations/" + id
	if got := d.tm.owner.expect(http.StatusOK, "GET", path, nil).body["actions"].([]any); !slices.Contains(got, any("sign_in")) || slices.Contains(got, any("set_credentials")) {
		t.Fatalf("actions: %v", got)
	}
	d.tm.owner.expectProblem(http.StatusConflict, "migration_state", "PUT", path+"/credentials", map[string]any{"username": "a", "password": "b"})
	d.tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", path+"/sign-in", map[string]any{"subdomain": "Not A Subdomain", "client_id": "zd-client", "client_secret": "zd-secret"})
	d.tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", path+"/sign-in", map[string]any{"subdomain": "acme", "client_id": "zd-client"})

	other := newTeam(t, d.h)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", path+"/sign-in", map[string]any{"subdomain": "acme", "client_id": "x", "client_secret": "y"})
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", path, nil)
	d.tm.agent.expectProblem(http.StatusForbidden, "forbidden", "POST", path+"/sign-in", map[string]any{"subdomain": "acme", "client_id": "x", "client_secret": "y"})

	start := map[string]any{"subdomain": helpdesktest.ZendeskSubdomain, "client_id": "zd-client", "client_secret": "zd-secret"}
	q := d.signIn(t, id, start)
	if q.Get("scope") != "read" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("redirect_uri") != testOrigin+"/migrations/sign-in/callback" {
		t.Fatalf("authorize query: %v", q)
	}
	if got := d.callback(t, d.tm.owner, url.Values{"error": {"access_denied"}, "state": {q.Get("state")}}); got != "access_denied" {
		t.Fatalf("declined: %s", got)
	}
	if got := d.callback(t, d.tm.owner, url.Values{"code": {helpdesktest.ZendeskCode}, "state": {q.Get("state")}}); got != "expired" {
		t.Fatalf("a used state: %s", got)
	}
	q = d.signIn(t, id, start)
	if got := d.callback(t, other.owner, url.Values{"code": {helpdesktest.ZendeskCode}, "state": {q.Get("state")}}); got != "session_mismatch" {
		t.Fatalf("another workspace's browser: %s", got)
	}
	d.desk.ZendeskScope = "read write"
	q = d.signIn(t, id, start)
	if got := d.callback(t, d.tm.owner, url.Values{"code": {helpdesktest.ZendeskCode}, "state": {q.Get("state")}}); got != "scope_too_broad" {
		t.Fatalf("a broader token: %s", got)
	}
	if n := d.h.count("SELECT count(*) FROM migration_credentials WHERE workspace_id = $1 AND migration_id = $2", d.tm.ws, id); n != 0 {
		t.Fatal("a token with more than read was stored")
	}
	d.desk.ZendeskScope = "read"
	q = d.signIn(t, id, start)
	if got := d.callback(t, d.tm.owner, url.Values{"code": {helpdesktest.ZendeskCode}, "state": {q.Get("state")}}); got != "connected" {
		t.Fatalf("sign-in: %s", got)
	}
	m := d.tm.owner.expect(http.StatusOK, "GET", path, nil)
	creds := m.body["credentials"].(map[string]any)
	if m.str("state") != "connected" || m.str("sign_in_result") != "connected" || creds["kind"] != "oauth" ||
		creds["server"].(map[string]any)["host"] != "acme.zendesk.com" || fmt.Sprint(creds["scopes"]) != "[read]" || strings.Contains(string(m.raw), "zd-secret") {
		t.Fatalf("connected: %s", m.raw)
	}
	if res := d.tm.owner.expect(http.StatusOK, "POST", path+"/test", nil); res.body["ok"] != true {
		t.Fatalf("test: %s", res.raw)
	}
	folders := d.tm.owner.expect(http.StatusOK, "GET", path+"/folders", nil).body["items"].([]any)
	if len(folders) != 2 || folders[1].(map[string]any)["name"] != "Acme Billing" || folders[1].(map[string]any)["address"] != "billing@acme.example.com" {
		t.Fatalf("folders: %v", folders)
	}
	d.tm.owner.expect(http.StatusOK, "PATCH", path, map[string]any{"options": map[string]any{
		"inboxes": []any{map[string]any{"folder": "Acme Billing", "inbox_id": billingInbox}},
		"people":  []any{map[string]any{"address": "ece@acme.example.com", "member_id": d.owner}},
		"labels":  []any{map[string]any{"folder": "vip", "action": "none"}},
	}})
	d.tm.owner.expect(http.StatusOK, "POST", path+"/scan", nil)
	d.work(t, func() bool { return d.state(id) == "ready" })

	m = d.tm.owner.expect(http.StatusOK, "GET", path, nil)
	est := m.body["estimate"].(map[string]any)
	var labels []string
	for _, l := range est["labels"].([]any) {
		labels = append(labels, l.(map[string]any)["name"].(string))
	}
	if num(est["conversations"]) != 3 || num(est["skipped"].(map[string]any)["trash"]) != 1 || fmt.Sprint(labels) != "[billing login]" || num(est["contacts_new"]) != 3 {
		t.Fatalf("dry run: %v", est)
	}
	var team []string
	for _, s := range m.body["team_suggestions"].([]any) {
		team = append(team, fmt.Sprint(s.(map[string]any)["address"], " ", num(s.(map[string]any)["messages"])))
	}
	if fmt.Sprint(team) != "[deniz@acme.example.com 1 ece@acme.example.com 1]" {
		t.Errorf("team suggestions: %v", team)
	}
	folders = d.tm.owner.expect(http.StatusOK, "GET", path+"/folders", nil).body["items"].([]any)
	if folders[1].(map[string]any)["inbox_id"] != billingInbox || num(folders[0].(map[string]any)["messages"]) != 3 {
		t.Errorf("folders after the scan: %v", folders)
	}
	preview := d.tm.owner.expect(http.StatusOK, "GET", path+"/preview", nil).body["items"].([]any)
	if len(preview) != 3 {
		t.Fatalf("preview: %v", preview)
	}
	top := preview[0].(map[string]any)
	if top["subject"] != "Feature request" || top["status"] != "pending" || len(top["messages"].([]any)) != 2 {
		t.Errorf("preview's newest: %v", top)
	}
	if n := d.h.count("SELECT count(*) FROM conversations WHERE workspace_id = $1", d.tm.ws); n != 0 {
		t.Fatalf("%d conversations before the start", n)
	}

	d.tm.owner.expect(http.StatusOK, "POST", path+"/start", nil)
	d.work(t, func() bool { return d.state(id) == "syncing" })

	if n := d.h.count("SELECT count(*) FROM conversations WHERE workspace_id = $1 AND migration_id = $2", d.tm.ws, id); n != 3 {
		t.Fatalf("%d conversations, want 3", n)
	}
	conv := func(subject string) string {
		return d.str(t, "SELECT id::text FROM conversations WHERE workspace_id = $1 AND subject = $2", d.tm.ws, subject)
	}
	signin, invoice, feature := conv("Cannot sign in"), conv("Invoice copy"), conv("Feature request")
	for c, want := range map[string]string{signin: "open " + d.tm.inbox + " " + channel + " " + d.owner, invoice: "closed " + billingInbox + " " + billingChannel + " -",
		feature: "pending " + d.tm.inbox + " - -"} {
		got := d.str(t, "SELECT status || ' ' || inbox_id || ' ' || coalesce(channel_id::text, '-') || ' ' || coalesce(assignee_id::text, '-') FROM conversations WHERE workspace_id = $1 AND id = $2", d.tm.ws, c)
		if got != want {
			t.Errorf("conversation %s: %s, want %s", c, got, want)
		}
	}
	if got := d.str(t, "SELECT string_agg(l.name, ',' ORDER BY l.name) FROM conversation_labels cl JOIN labels l ON l.workspace_id = cl.workspace_id AND l.id = cl.label_id WHERE cl.workspace_id = $1 AND cl.conversation_id = $2", d.tm.ws, signin); got != "login" {
		t.Errorf("labels: %s", got)
	}
	shape := func(c string) string {
		return d.str(t, `SELECT string_agg(m.kind || ':' || coalesce(m.direction, '-') || ':' || m.author_type || ':' || coalesce(m.author_member_id::text, coalesce(m.imported_author->>'address', ct.name)), ' ' ORDER BY m.created_at)
			FROM messages m LEFT JOIN contacts ct ON ct.workspace_id = m.workspace_id AND ct.id = m.author_contact_id WHERE m.workspace_id = $1 AND m.conversation_id = $2`, d.tm.ws, c)
	}
	if got := shape(signin); got != "message:in:contact:Ayşe Yılmaz note:-:member:"+d.owner+" message:out:member:"+d.owner+" message:in:contact:Carol Reed" {
		t.Errorf("Cannot sign in: %s", got)
	}
	if got := shape(invoice); got != "message:in:contact:Bob Stone message:out:member:deniz@acme.example.com" {
		t.Errorf("Invoice copy: %s", got)
	}
	if got := d.str(t, "SELECT string_agg(filename, ',' ORDER BY filename) FROM attachments WHERE workspace_id = $1", d.tm.ws); got != "log.txt,order.txt,screenshot.png" {
		t.Errorf("attachments: %s", got)
	}
	if got := d.str(t, "SELECT c.attributes::text FROM contacts c JOIN contact_emails e ON e.workspace_id = c.workspace_id AND e.contact_id = c.id WHERE c.workspace_id = $1 AND e.email = 'ayse@example.org'", d.tm.ws); !strings.Contains(got, `"zendesk_id": "101"`) || !strings.Contains(got, `"zendesk_external_id": "crm-101"`) || !strings.Contains(got, `"phone"`) {
		t.Errorf("requester attributes: %s", got)
	}
	if got := d.str(t, "SELECT c.name || ' ' || (c.attributes->>'zendesk_id') FROM conversations v JOIN contacts c ON c.workspace_id = v.workspace_id AND c.id = v.contact_id WHERE v.workspace_id = $1 AND v.id = $2", d.tm.ws, feature); got != "Phone Caller 104" {
		t.Errorf("requester without an address: %s", got)
	}
	if n := d.h.count("SELECT count(*) FROM events WHERE workspace_id = $1 AND (type LIKE 'conversation.%' OR type LIKE 'message.%')", d.tm.ws); n != 0 {
		t.Errorf("imported history wrote %d conversation or message events", n)
	}
	if n := d.h.count("SELECT count(*) FROM messages WHERE workspace_id = $1 AND imported_at IS NULL", d.tm.ws); n != 0 {
		t.Errorf("%d messages are not marked imported", n)
	}
	var withToken, without []string
	for _, r := range d.desk.Requests() {
		if strings.Contains(r.Path, "attachment") || r.Host == helpdesktest.OutsideHost {
			if r.Auth != "" {
				withToken = append(withToken, r.Host)
			} else {
				without = append(without, r.Host)
			}
		}
	}
	slices.Sort(withToken)
	if fmt.Sprint(withToken) != "[acme.zendesk.com p12.zdusercontent.com]" || fmt.Sprint(without) != "["+helpdesktest.OutsideHost+"]" {
		t.Errorf("attachments fetched with the token from %v and without from %v", withToken, without)
	}

	d.h.clock.Advance(31 * time.Minute)
	changed := d.h.clock.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	d.desk.Override("zendesk/tickets_zd-c2.json", []byte(`{"tickets": [{"id": 1001, "subject": "Cannot sign in", "status": "open", "brand_id": 360001,
		"requester_id": 101, "assignee_id": 9002, "tags": ["login", "vip"], "via": {"channel": "email"}, "created_at": "2026-09-01T08:00:00Z",
		"updated_at": "`+changed+`"}], "after_cursor": "zd-c3", "end_of_stream": true}`))
	comments, _ := helpdesktestFixture("zendesk/comments_1001.json")
	comments = strings.Replace(comments, `], "users": [`, `, {"id": 70009, "type": "Comment", "author_id": 101, "public": true, "created_at": "`+changed+`",
		"body": "It happens again.", "plain_body": "It happens again.", "html_body": "<p>It happens again.</p>", "attachments": []}], "users": [`, 1)
	d.desk.Override("zendesk/comments_1001.json", []byte(comments))
	synced := func() bool {
		return d.h.count("SELECT count(*) FROM messages WHERE workspace_id = $1 AND conversation_id = $2", d.tm.ws, signin) == 5
	}
	d.work(t, synced)
	for range 3 {
		d.syncPass(t, id)
	}
	if n := d.h.count("SELECT count(*) FROM messages WHERE workspace_id = $1 AND conversation_id = $2", d.tm.ws, signin); n != 5 {
		t.Errorf("after sync the conversation has %d messages, want 5", n)
	}
	if n := d.h.count("SELECT count(*) FROM messages WHERE workspace_id = $1 AND body = 'It happens again.' AND imported_at IS NULL AND direction = 'in'", d.tm.ws); n != 1 {
		t.Errorf("the comment written after the switch began is there %d times as new mail", n)
	}
	if n := d.h.count("SELECT count(*) FROM events WHERE workspace_id = $1 AND type = 'message.created'", d.tm.ws); n != 1 {
		t.Errorf("%d message.created events, want the new comment's", n)
	}
	refreshed := 0
	for _, r := range d.desk.Requests() {
		if r.Method == http.MethodPost && strings.Contains(r.Body, "refresh_token") {
			refreshed++
		}
	}
	if refreshed == 0 {
		t.Error("the expiring token was not refreshed")
	}
	d.checkRequests(t, "acme.zendesk.com", "/oauth/tokens", helpdesktest.ZendeskLimit/2, 5)

	d.tm.owner.expect(http.StatusOK, "POST", path+"/finish", map[string]any{"force": true})
	if n := d.h.count("SELECT count(*) FROM migration_credentials WHERE workspace_id = $1 AND migration_id = $2", d.tm.ws, id) +
		d.h.count("SELECT count(*) FROM migration_sign_ins WHERE workspace_id = $1 AND migration_id = $2", d.tm.ws, id); n != 0 {
		t.Error("the credentials outlived the migration")
	}
}

func TestHelpScoutMigration(t *testing.T) {
	d := newDeskRun(t)
	address, channel := d.emailChannel(t, d.tm.inbox, "support")
	id := d.tm.owner.expect(http.StatusCreated, "POST", "/v1/migrations", map[string]any{
		"source": "helpscout", "preset": "helpscout", "source_address": address, "channel_id": channel,
		"options": map[string]any{"people": []any{map[string]any{"address": "mert@hs.example.com", "member_id": d.owner}}},
	}).str("id")
	path := "/v1/migrations/" + id
	q := d.signIn(t, id, map[string]any{"client_id": "hs-client", "client_secret": "hs-secret"})
	if q.Get("client_id") != "hs-client" || q.Get("state") == "" || q.Get("scope") != "" {
		t.Fatalf("authorize query: %v", q)
	}
	if got := d.callback(t, d.tm.agent, url.Values{"code": {helpdesktest.HelpScoutCode}, "state": {q.Get("state")}}); got != "session_mismatch" {
		t.Fatalf("an agent's browser: %s", got)
	}
	q = d.signIn(t, id, map[string]any{"client_id": "hs-client", "client_secret": "wrong"})
	if got := d.callback(t, d.tm.owner, url.Values{"code": {helpdesktest.HelpScoutCode}, "state": {q.Get("state")}}); got != "token_failed" {
		t.Fatalf("a wrong secret: %s", got)
	}
	q = d.signIn(t, id, map[string]any{"client_id": "hs-client", "client_secret": "hs-secret"})
	if got := d.callback(t, d.tm.owner, url.Values{"code": {helpdesktest.HelpScoutCode}, "state": {q.Get("state")}}); got != "connected" {
		t.Fatalf("sign-in: %s", got)
	}
	d.tm.owner.expect(http.StatusOK, "POST", path+"/scan", nil)
	d.work(t, func() bool { return d.state(id) == "ready" })
	est := d.tm.owner.expect(http.StatusOK, "GET", path, nil).body["estimate"].(map[string]any)
	if num(est["conversations"]) != 2 || num(est["messages"]) != 6 || num(est["skipped"].(map[string]any)["spam"]) != 1 {
		t.Fatalf("dry run: %v", est)
	}
	d.tm.owner.expect(http.StatusOK, "POST", path+"/start", nil)
	d.work(t, func() bool { return d.state(id) == "syncing" })

	order := d.str(t, "SELECT id::text FROM conversations WHERE workspace_id = $1 AND subject = 'Order missing'", d.tm.ws)
	pricing := d.str(t, "SELECT id::text FROM conversations WHERE workspace_id = $1 AND subject = 'Pricing'", d.tm.ws)
	if got := d.str(t, "SELECT status || ' ' || coalesce(assignee_id::text, '-') FROM conversations WHERE workspace_id = $1 AND id = $2", d.tm.ws, order); got != "open "+d.owner {
		t.Errorf("Order missing: %s", got)
	}
	if got := d.str(t, "SELECT status FROM conversations WHERE workspace_id = $1 AND id = $2", d.tm.ws, pricing); got != "closed" {
		t.Errorf("Pricing: %s", got)
	}
	if got := d.str(t, "SELECT string_agg(kind || ':' || coalesce(direction, '-'), ' ' ORDER BY created_at) FROM messages WHERE workspace_id = $1 AND conversation_id = $2", d.tm.ws, order); got != "message:in note:- message:out" {
		t.Errorf("Order missing threads: %s", got)
	}
	if got := d.str(t, "SELECT header_message_id || ' ' || (raw_key IS NOT NULL)::text FROM message_emails WHERE workspace_id = $1 AND conversation_id = $2", d.tm.ws, order); got != "order-4411@example.org true" {
		t.Errorf("the e-mail thread's original: %s", got)
	}
	if got := d.str(t, "SELECT string_agg(filename, ',' ORDER BY filename) FROM attachments WHERE workspace_id = $1", d.tm.ws); got != "prices.txt,receipt.txt" {
		t.Errorf("attachments: %s", got)
	}
	if got := d.str(t, "SELECT c.attributes->>'helpscout_id' || ' ' || (c.attributes->>'phone') FROM conversations v JOIN contacts c ON c.workspace_id = v.workspace_id AND c.id = v.contact_id WHERE v.workspace_id = $1 AND v.id = $2", d.tm.ws, order); got != "801 +90 212 000 0801" {
		t.Errorf("customer attributes: %s", got)
	}
	if got := d.str(t, "SELECT string_agg(l.name, ',') FROM conversation_labels cl JOIN labels l ON l.workspace_id = cl.workspace_id AND l.id = cl.label_id WHERE cl.workspace_id = $1 AND cl.conversation_id = $2", d.tm.ws, pricing); got != "sales" {
		t.Errorf("labels: %s", got)
	}

	at := d.h.clock.Now().Add(time.Minute).UTC().Format(time.RFC3339)
	d.h.clock.Advance(3 * time.Minute)
	d.desk.Override("helpscout/conversations_since.json", []byte(`{"_embedded": {"conversations": [{"id": 7002, "threads": 3, "status": "active", "state": "published",
		"subject": "Pricing", "mailboxId": 12, "assignee": null, "createdAt": "2026-08-20T10:00:00Z", "userUpdatedAt": "`+at+`",
		"source": {"type": "chat"}, "tags": [{"tag": "sales"}], "primaryCustomer": {"id": 802, "first": "Kerem", "last": "Aslan", "email": "kerem@example.net"}}]},
		"page": {"size": 25, "totalElements": 1, "totalPages": 1, "number": 1}}`))
	threads, _ := helpdesktestFixture("helpscout/threads_7002.json")
	threads = strings.Replace(threads, `]}, "_links"`, `, {"id": 9, "type": "chat", "state": "published", "body": "<p>And for ten people?</p>",
		"source": {"type": "chat", "via": "customer"}, "customer": {"id": 802, "first": "Kerem", "last": "Aslan", "email": "kerem@example.net"},
		"createdBy": {"id": 802, "type": "customer", "email": "kerem@example.net"}, "createdAt": "`+at+`", "_embedded": {"attachments": []}}]}, "_links"`, 1)
	d.desk.Override("helpscout/threads_7002.json", []byte(threads))
	d.work(t, func() bool {
		return d.h.count("SELECT count(*) FROM messages WHERE workspace_id = $1 AND conversation_id = $2 AND kind <> 'event'", d.tm.ws, pricing) == 3
	})
	for range 3 {
		d.syncPass(t, id)
	}
	if n := d.h.count("SELECT count(*) FROM messages WHERE workspace_id = $1 AND conversation_id = $2 AND kind <> 'event'", d.tm.ws, pricing); n != 3 {
		t.Errorf("after sync Pricing has %d messages, want 3", n)
	}
	if got := d.str(t, "SELECT status FROM conversations WHERE workspace_id = $1 AND id = $2", d.tm.ws, pricing); got != "open" {
		t.Errorf("a customer's new message left the conversation %s", got)
	}
	d.checkRequests(t, "api.helpscout.net", "/v2/oauth2/token", helpdesktest.HelpScoutLimit/2, 0)
}

func helpdesktestFixture(name string) (string, error) {
	b, err := helpdesktest.Fixture(name)
	return string(b), err
}

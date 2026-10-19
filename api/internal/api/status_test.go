package api_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/api"
	"github.com/productdevbook/yuva/api/internal/store"
)

type statusTeam struct {
	team
	page, slug string
	api, web   string
}

func newStatusTeam(t *testing.T, h *harness) statusTeam {
	t.Helper()
	tm := newTeam(t, h)
	slug := unique("st")
	pg := tm.owner.expect(http.StatusCreated, "POST", "/v1/status-pages", map[string]any{"inbox_id": tm.inbox, "name": "Acme", "slug": slug})
	st := statusTeam{team: tm, page: pg.str("id"), slug: slug}
	st.api = tm.owner.expect(http.StatusCreated, "POST", "/v1/status-pages/"+st.page+"/components", map[string]any{"name": "API"}).str("id")
	st.web = tm.owner.expect(http.StatusCreated, "POST", "/v1/status-pages/"+st.page+"/components", map[string]any{"name": "Website"}).str("id")
	return st
}

func (st statusTeam) path(p string) string { return "/v1/status-pages/" + st.page + p }

func (st statusTeam) componentState(t *testing.T, id string) string {
	t.Helper()
	for _, c := range st.owner.expect(http.StatusOK, "GET", st.path("/components"), nil).body["items"].([]any) {
		if m := c.(map[string]any); m["id"] == id {
			return m["state"].(string)
		}
	}
	t.Fatalf("no component %s", id)
	return ""
}

func publicStatus(t *testing.T, h *harness, path string) response {
	t.Helper()
	return h.client().do("GET", path, nil)
}

func TestStatusPageAccess(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	st := newStatusTeam(t, h)

	st.owner.expectProblem(http.StatusConflict, "status_page_exists", "POST", "/v1/status-pages", map[string]any{"inbox_id": st.inbox, "name": "Again", "slug": unique("st")})
	other := st.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Other", "slug": "other"}).body["inbox"].(map[string]any)["id"].(string)
	st.owner.expectProblem(http.StatusConflict, "slug_taken", "POST", "/v1/status-pages", map[string]any{"inbox_id": other, "name": "Taken", "slug": st.slug})
	st.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/status-pages", map[string]any{"inbox_id": other, "name": "Reserved", "slug": "status"})
	st.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/status-pages", map[string]any{"inbox_id": other, "name": "Short", "slug": "ab"})

	if r := publicStatus(t, h, "/status/"+st.slug); r.status != http.StatusNotFound {
		t.Fatalf("unpublished page answers %d", r.status)
	}
	if r := publicStatus(t, h, "/client/v1/status/"+st.slug); r.status != http.StatusNotFound {
		t.Fatalf("unpublished JSON answers %d", r.status)
	}
	prev := st.owner.expect(http.StatusOK, "GET", st.path("/preview"), nil)
	if !strings.Contains(string(prev.raw), "Acme") || prev.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("preview: %s %v", prev.raw, prev.header)
	}

	st.agent.expectProblem(http.StatusNotFound, "not_found", "GET", st.path(""), nil)
	st.agent.expectProblem(http.StatusNotFound, "not_found", "POST", st.path("/components/"+st.api+"/state"), map[string]any{"state": "degraded"})
	st.agent.expectProblem(http.StatusNotFound, "not_found", "POST", st.path("/incidents"), map[string]any{"title": "Down", "message": "Looking"})
	if items := st.agent.expect(http.StatusOK, "GET", "/v1/status-pages", nil).body["items"].([]any); len(items) != 0 {
		t.Fatalf("an agent without the inbox lists %v", items)
	}

	st.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+st.inbox+"/members/"+st.agentID, nil)
	st.agent.expect(http.StatusOK, "GET", st.path(""), nil)
	st.agent.expect(http.StatusOK, "POST", st.path("/components/"+st.api+"/state"), map[string]any{"state": "degraded"})
	st.agent.expect(http.StatusCreated, "POST", st.path("/incidents"), map[string]any{"title": "Slow", "message": "Looking"})
	st.agent.expectProblem(http.StatusForbidden, "forbidden", "POST", st.path("/components"), map[string]any{"name": "DB"})
	st.agent.expectProblem(http.StatusForbidden, "forbidden", "POST", st.path("/groups"), map[string]any{"name": "Core"})
	st.agent.expectProblem(http.StatusForbidden, "forbidden", "PATCH", st.path(""), map[string]any{"published": true})
	st.agent.expectProblem(http.StatusForbidden, "forbidden", "DELETE", st.path(""), nil)

	stranger := newTeam(t, h)
	stranger.owner.expectProblem(http.StatusNotFound, "not_found", "GET", st.path(""), nil)
	stranger.owner.expectProblem(http.StatusNotFound, "not_found", "POST", st.path("/components/"+st.api+"/state"), map[string]any{"state": "major_outage"})
	stranger.owner.expectProblem(http.StatusNotFound, "not_found", "GET", st.path("/incidents"), nil)
	strangerKey := h.client()
	strangerKey.bearer = stranger.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "k"}).str("secret")
	strangerKey.expectProblem(http.StatusNotFound, "not_found", "GET", st.path(""), nil)

	limited := h.client()
	limited.bearer = st.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "limited", "inbox_ids": []string{other}, "scopes": []string{"status:read", "status:manage"}}).str("secret")
	limited.expectProblem(http.StatusNotFound, "not_found", "GET", st.path(""), nil)
	mine := h.client()
	mine.bearer = st.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "mine", "inbox_ids": []string{st.inbox}, "scopes": []string{"status:manage"}}).str("secret")
	mine.expect(http.StatusOK, "POST", st.path("/components/"+st.web+"/state"), map[string]any{"state": "partial_outage"})
	mine.expectProblem(http.StatusForbidden, "insufficient_scope", "GET", st.path(""), nil)
	reader := h.client()
	reader.bearer = st.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "reader", "scopes": []string{"status:read"}}).str("secret")
	reader.expect(http.StatusOK, "GET", st.path("/components"), nil)
	reader.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", st.path("/components/"+st.web+"/state"), map[string]any{"state": "operational"})
	full := h.client()
	full.bearer = st.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "full"}).str("secret")
	full.expect(http.StatusOK, "GET", st.path(""), nil)
	if src := st.owner.expect(http.StatusOK, "GET", st.path("/components"), nil).body["items"].([]any)[1].(map[string]any)["state_source"]; src != "api_key" {
		t.Fatalf("state set with a key has source %v", src)
	}
}

func TestStatusIncidentFlow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	st := newStatusTeam(t, h)
	st.owner.expect(http.StatusOK, "PATCH", st.path(""), map[string]any{"published": true})
	st.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+st.inbox+"/members/"+st.agentID, nil)

	page := publicStatus(t, h, "/status/"+st.slug)
	if page.status != http.StatusOK || !strings.Contains(string(page.raw), "All systems operational") {
		t.Fatalf("published page %d %s", page.status, page.raw)
	}
	if cc := page.header.Get("Cache-Control"); cc != "public, max-age=30, stale-if-error=86400" {
		t.Fatalf("Cache-Control %q", cc)
	}
	if csp := page.header.Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors *") || !strings.Contains(csp, "script-src 'sha256-") {
		t.Fatalf("CSP %q", csp)
	}

	draft := st.agent.expect(http.StatusCreated, "POST", st.path("/incidents"), map[string]any{
		"title": "Draft", "message": "Not yet", "components": []map[string]any{{"component_id": st.api}},
	})
	if st.componentState(t, st.api) != "operational" {
		t.Fatal("a draft changed a component")
	}
	if r := publicStatus(t, h, "/status/"+st.slug+"/incidents/"+draft.str("id")); r.status != http.StatusNotFound {
		t.Fatalf("a draft's page answers %d", r.status)
	}
	st.agent.expect(http.StatusNoContent, "DELETE", st.path("/incidents/"+draft.str("id")), nil)

	inc := st.agent.expect(http.StatusCreated, "POST", st.path("/incidents"), map[string]any{
		"title": "API down", "message": "We are looking into errors.", "impact": "major", "publish": true,
		"components": []map[string]any{{"component_id": st.api, "state": "major_outage"}},
	})
	id := inc.str("id")
	if inc.str("state") != "investigating" || st.componentState(t, st.api) != "major_outage" {
		t.Fatalf("incident %s, component %s", inc.raw, st.componentState(t, st.api))
	}
	st.agent.expectProblem(http.StatusConflict, "incident_published", "DELETE", st.path("/incidents/"+id), nil)

	js := publicStatus(t, h, "/client/v1/status/"+st.slug)
	if js.status != http.StatusOK || js.str("state") != "major_outage" || js.header.Get("Access-Control-Allow-Origin") != "*" || len(js.body["open"].([]any)) != 1 {
		t.Fatalf("JSON %d %s %v", js.status, js.raw, js.header)
	}
	if b := publicStatus(t, h, "/status/"+st.slug+"/badge.svg"); !strings.Contains(string(b.raw), "Major outage") || b.header.Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("badge %s", b.raw)
	}
	if f := publicStatus(t, h, "/status/"+st.slug+"/feed.atom"); !strings.Contains(string(f.raw), "API down") {
		t.Fatalf("feed %s", f.raw)
	}

	second := st.owner.expect(http.StatusCreated, "POST", st.path("/incidents"), map[string]any{
		"title": "API slow", "message": "Slow too.", "publish": true, "components": []map[string]any{{"component_id": st.api, "state": "degraded"}},
	})
	st.agent.expect(http.StatusCreated, "POST", st.path("/incidents/"+id+"/updates"), map[string]any{"state": "monitoring", "body": "A fix is out, see https://example.com/x."})
	upd := st.agent.expect(http.StatusCreated, "POST", st.path("/incidents/"+id+"/updates"), map[string]any{"state": "resolved", "body": "Fixed."})
	if upd.str("state") != "resolved" || upd.body["resolved_at"] == nil {
		t.Fatalf("resolve %s", upd.raw)
	}
	if got := st.componentState(t, st.api); got != "degraded" {
		t.Fatalf("another open incident holds the component at degraded, got %s", got)
	}
	st.agent.expectProblem(http.StatusConflict, "incident_closed", "POST", st.path("/incidents/"+id+"/updates"), map[string]any{"body": "again"})
	st.agent.expectProblem(http.StatusConflict, "incident_closed", "PATCH", st.path("/incidents/"+id), map[string]any{"title": "x"})
	st.owner.expect(http.StatusCreated, "POST", st.path("/incidents/"+second.str("id")+"/updates"), map[string]any{"state": "resolved", "body": "Done."})
	if got := st.componentState(t, st.api); got != "operational" {
		t.Fatalf("after both incidents the component is %s", got)
	}

	firstUpdate := upd.body["updates"].([]any)[2].(map[string]any)["id"].(string)
	edited := st.agent.expect(http.StatusOK, "PATCH", st.path("/incidents/"+id+"/updates/"+firstUpdate), map[string]any{"body": "We looked into errors."})
	if edited.body["updates"].([]any)[2].(map[string]any)["edited_at"] == nil {
		t.Fatalf("edit of a published update has no mark: %s", edited.raw)
	}
	html := publicStatus(t, h, "/status/"+st.slug+"/incidents/"+id)
	if html.status != http.StatusOK || !strings.Contains(string(html.raw), "We looked into errors.") || !strings.Contains(string(html.raw), `<a href="https://example.com/x"`) {
		t.Fatalf("incident page %d %s", html.status, html.raw)
	}
	if hist := publicStatus(t, h, "/status/"+st.slug+"/history"); !strings.Contains(string(hist.raw), "API down") {
		t.Fatalf("history %s", hist.raw)
	}

	events := st.owner.expect(http.StatusOK, "GET", "/v1/events?after=0", nil).body["events"].([]any)
	seen := map[string]int{}
	for _, e := range events {
		seen[e.(map[string]any)["type"].(string)]++
	}
	if seen["status.incident.created"] < 3 || seen["status.incident.resolved"] != 2 || seen["status.component.updated"] < 4 {
		t.Fatalf("events %v", seen)
	}
}

func TestStatusIncidentAndMemberStates(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	st := newStatusTeam(t, h)
	st.owner.expect(http.StatusOK, "POST", st.path("/components/"+st.api+"/state"), map[string]any{"state": "degraded"})
	st.owner.expect(http.StatusOK, "POST", st.path("/components/"+st.web+"/state"), map[string]any{"state": "major_outage"})
	inc := st.owner.expect(http.StatusCreated, "POST", st.path("/incidents"), map[string]any{
		"title": "Trouble", "message": "Looking.", "publish": true,
		"components": []map[string]any{{"component_id": st.api, "state": "degraded"}, {"component_id": st.web, "state": "degraded"}},
	})
	if got := st.componentState(t, st.web); got != "major_outage" {
		t.Fatalf("an incident lowered a worse state a member set: %s", got)
	}
	st.owner.expect(http.StatusCreated, "POST", st.path("/incidents/"+inc.str("id")+"/updates"), map[string]any{"state": "resolved", "body": "Fixed."})
	if got := st.componentState(t, st.api); got != "operational" {
		t.Fatalf("a component the incident held at the member's state stays %s after resolve", got)
	}
}

func TestStatusMaintenanceRunsOnTime(t *testing.T) {
	h := newHarness(t)
	st := newStatusTeam(t, h)
	st.owner.expect(http.StatusOK, "PATCH", st.path(""), map[string]any{"published": true})
	start := h.clock.Now().Add(time.Hour)
	st.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", st.path("/incidents"), map[string]any{"kind": "maintenance", "title": "DB", "message": "Upgrade"})
	m := st.owner.expect(http.StatusCreated, "POST", st.path("/incidents"), map[string]any{
		"kind": "maintenance", "title": "Database upgrade", "message": "Planned.", "publish": true,
		"scheduled_start": start.Format(time.RFC3339), "scheduled_end": start.Add(time.Hour).Format(time.RFC3339),
		"components": []map[string]any{{"component_id": st.web}},
	})
	if m.str("state") != "scheduled" || st.componentState(t, st.web) != "operational" {
		t.Fatalf("scheduled maintenance %s", m.raw)
	}
	if n := len(h.jobs("status_maintenance", st.ws)); n != 2 {
		t.Fatalf("%d maintenance jobs, want start and end", n)
	}
	args := api.StatusMaintenanceArgs{WorkspaceID: uuid.MustParse(st.ws), IncidentID: uuid.MustParse(m.str("id"))}
	ctx := context.Background()
	if err := h.srv.RunStatusMaintenance(ctx, args); err != nil {
		t.Fatal(err)
	}
	if got := st.owner.expect(http.StatusOK, "GET", st.path("/incidents/"+m.str("id")), nil).str("state"); got != "scheduled" {
		t.Fatalf("started early: %s", got)
	}
	h.clock.Advance(time.Hour + time.Second)
	if err := h.srv.RunStatusMaintenance(ctx, args); err != nil {
		t.Fatal(err)
	}
	if got := st.owner.expect(http.StatusOK, "GET", st.path("/incidents/"+m.str("id")), nil).str("state"); got != "in_progress" || st.componentState(t, st.web) != "maintenance" {
		t.Fatalf("at the start: %s, component %s", got, st.componentState(t, st.web))
	}
	if js := publicStatus(t, h, "/client/v1/status/"+st.slug); js.str("state") != "maintenance" {
		t.Fatalf("page state %s", js.raw)
	}
	h.clock.Advance(time.Hour)
	if err := h.srv.RunStatusMaintenance(ctx, args); err != nil {
		t.Fatal(err)
	}
	done := st.owner.expect(http.StatusOK, "GET", st.path("/incidents/"+m.str("id")), nil)
	if done.str("state") != "completed" || st.componentState(t, st.web) != "operational" || len(done.body["updates"].([]any)) != 3 {
		t.Fatalf("at the end: %s, component %s", done.raw, st.componentState(t, st.web))
	}
	st.owner.expectProblem(http.StatusConflict, "incident_closed", "POST", st.path("/incidents/"+m.str("id")+"/updates"), map[string]any{"body": "More"})
}

func TestStatusPublicRateLimit(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	st := newStatusTeam(t, h)
	st.owner.expect(http.StatusOK, "PATCH", st.path(""), map[string]any{"published": true})
	c := h.client()
	for i := range 120 {
		if r := c.do("GET", "/status/"+st.slug+"/badge.svg", nil); r.status != http.StatusOK {
			t.Fatalf("request %d: %d", i, r.status)
		}
	}
	if r := c.do("GET", "/client/v1/status/"+st.slug, nil); r.status != http.StatusTooManyRequests {
		t.Fatalf("request 121: %d", r.status)
	}
	if r := h.client().do("GET", "/status/"+st.slug, nil); r.status != http.StatusOK {
		t.Fatalf("another address: %d", r.status)
	}
}

func TestStatusURLRedirect(t *testing.T) {
	t.Parallel()
	h := newHarnessWith(t, func(d *api.Deps) { d.Status = api.StatusSettings{URL: "https://status.example.test"} })
	st := newStatusTeam(t, h)
	st.owner.expect(http.StatusOK, "PATCH", st.path(""), map[string]any{"published": true})
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := noFollow.Get(h.url + "/status/" + st.slug + "/history")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") != "https://status.example.test/"+st.slug+"/history" {
		t.Fatalf("redirect %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	for path, want := range map[string]int{"/" + st.slug: http.StatusOK, "/" + st.slug + "/badge.svg": http.StatusOK, "/v1/me": http.StatusNotFound, "/": http.StatusOK} {
		req, _ := http.NewRequest("GET", h.url+path, nil)
		req.Host = "status.example.test"
		res, err := noFollow.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != want {
			t.Fatalf("%s on the status host: %d, want %d", path, res.StatusCode, want)
		}
	}
	if u := st.owner.expect(http.StatusOK, "GET", st.path(""), nil).str("url"); u != "https://status.example.test/"+st.slug {
		t.Fatalf("page url %s", u)
	}
}

func TestStatusMCPTools(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	st := newStatusTeam(t, h)
	reader := st.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "r", "scopes": []string{"status:read"}}).str("secret")
	names := toolNames(t, h.mcpSession(reader))
	if !slices.Contains(names, "list_status_pages") || !slices.Contains(names, "get_status_page") || slices.Contains(names, "create_incident") {
		t.Fatalf("status:read lists %v", names)
	}
	key := st.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "m", "scopes": []string{"status:read", "status:manage"}}).str("secret")
	cs := h.mcpSession(key)
	pages, errText := callTool(t, cs, "list_status_pages", nil)
	if errText != "" || len(pages["items"].([]any)) != 1 {
		t.Fatalf("list_status_pages %v %s", pages, errText)
	}
	inc, errText := callTool(t, cs, "create_incident", map[string]any{"page_id": st.page, "title": "Errors", "message": "Looking.",
		"components": []map[string]any{{"component_id": st.api}}})
	if errText != "" || inc["draft"] != true || st.componentState(t, st.api) != "operational" {
		t.Fatalf("create_incident %v %s", inc, errText)
	}
	st.owner.expect(http.StatusOK, "POST", st.path("/incidents/"+inc["id"].(string)+"/publish"), nil)
	if st.componentState(t, st.api) != "major_outage" {
		t.Fatal("publishing did not set the component")
	}
	if _, errText := callTool(t, cs, "set_component_state", map[string]any{"page_id": st.page, "component_id": st.web, "state": "degraded"}); errText != "" {
		t.Fatal(errText)
	}
	detail, errText := callTool(t, cs, "get_status_page", map[string]any{"page_id": st.page})
	if errText != "" || detail["page"].(map[string]any)["state"] != "major_outage" || len(detail["open"].([]any)) != 1 {
		t.Fatalf("get_status_page %v %s", detail, errText)
	}
	res, errText := callTool(t, cs, "resolve_incident", map[string]any{"page_id": st.page, "incident_id": inc["id"], "body": "Fixed."})
	if errText != "" || res["state"] != "resolved" || st.componentState(t, st.api) != "operational" {
		t.Fatalf("resolve_incident %v %s", res, errText)
	}
}

func TestStatusWebhooks(t *testing.T) {
	t.Parallel()
	h := privateHarness(t)
	st := newStatusTeam(t, h)
	rcv := newReceiver(t)
	st.owner.expect(http.StatusCreated, "POST", "/v1/webhooks", map[string]any{"url": rcv.srv.URL,
		"events": []string{"status.incident.created", "status.incident.updated", "status.incident.resolved", "status.component.updated"}})
	h.runWebhooks(st.ws)
	rcv.take()
	inc := st.owner.expect(http.StatusCreated, "POST", st.path("/incidents"), map[string]any{"title": "Errors", "message": "Looking.",
		"components": []map[string]any{{"component_id": st.api}}}).str("id")
	h.runWebhooks(st.ws)
	if got := rcv.take(); len(got) != 0 {
		t.Fatalf("a draft sent %v", got[0].typ)
	}
	st.owner.expect(http.StatusOK, "POST", st.path("/incidents/"+inc+"/publish"), nil)
	st.owner.expect(http.StatusCreated, "POST", st.path("/incidents/"+inc+"/updates"), map[string]any{"state": "resolved", "body": "Fixed."})
	h.runWebhooks(st.ws)
	var types []string
	for _, hk := range rcv.take() {
		types = append(types, hk.typ)
	}
	want := []string{"status.component.updated", "status.incident.created", "status.component.updated", "status.incident.resolved"}
	if !slices.Equal(types, want) {
		t.Fatalf("webhooks %v, want %v", types, want)
	}
}

func TestStatusPageChangedAtNeverMovesBack(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	st := newStatusTeam(t, h)
	ws, page := uuid.MustParse(st.ws), uuid.MustParse(st.page)
	changed := func() time.Time {
		pg, err := h.st.GetStatusPage(context.Background(), store.GetStatusPageParams{WorkspaceID: ws, ID: page})
		if err != nil {
			t.Fatal(err)
		}
		return pg.ChangedAt
	}
	st.owner.expect(http.StatusOK, "PATCH", st.path(""), map[string]any{"name": "One"})
	first := changed()
	st.owner.expect(http.StatusOK, "PATCH", st.path(""), map[string]any{"name": "Two"})
	same := changed()
	h.clock.Set(first.Add(-time.Hour))
	st.owner.expect(http.StatusOK, "PATCH", st.path(""), map[string]any{"name": "Three"})
	back := changed()
	if !same.After(first) || !back.After(same) {
		t.Fatalf("changed_at %v, %v, %v must only grow", first, same, back)
	}
}

func TestStatusComponentHolderHandover(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	st := newStatusTeam(t, h)
	open := func(id string) store.StatusComponentState {
		rows, err := h.st.OpenStatusComponentStates(context.Background(), store.OpenStatusComponentStatesParams{
			WorkspaceID: uuid.MustParse(st.ws), ComponentIds: []uuid.UUID{uuid.MustParse(id)},
		})
		if err != nil || len(rows) != 1 {
			t.Fatalf("open spans %v %v", rows, err)
		}
		return rows[0]
	}
	raise := func(title string) string {
		return st.owner.expect(http.StatusCreated, "POST", st.path("/incidents"), map[string]any{
			"title": title, "message": "Looking.", "publish": true,
			"components": []map[string]any{{"component_id": st.api, "state": "major_outage"}},
		}).str("id")
	}
	a, b := raise("First"), raise("Second")
	st.owner.expect(http.StatusCreated, "POST", st.path("/incidents/"+a+"/updates"), map[string]any{"state": "resolved", "body": "Fixed."})
	if got := st.componentState(t, st.api); got != "major_outage" {
		t.Fatalf("the second incident still holds the component: %s", got)
	}
	if o := open(st.api); o.Source != "incident" || o.SourceID == nil || o.SourceID.String() != b {
		t.Fatalf("holder %v %v, want incident %s", o.Source, o.SourceID, b)
	}
}

func TestStatusSlugRenameRedirects(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	st := newStatusTeam(t, h)
	st.owner.expect(http.StatusOK, "PATCH", st.path(""), map[string]any{"published": true})
	stranger := newTeam(t, h)
	old, current := st.slug, unique("st")

	full := h.client()
	full.bearer = st.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "full"}).str("secret")
	full.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", st.path("/components/"+st.web+"/state"), map[string]any{"state": "degraded"})

	st.owner.expect(http.StatusOK, "PATCH", st.path(""), map[string]any{"slug": current})
	visitor := h.client()
	visitor.http.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	for _, suffix := range []string{"", "/history", "/feed.atom", "/badge.svg", "/incidents/" + uuid.New().String()} {
		r := visitor.do("GET", "/status/"+old+suffix, nil)
		if want := "/status/" + current + suffix; r.status != http.StatusMovedPermanently || !strings.HasSuffix(r.header.Get("Location"), want) {
			t.Fatalf("%s: %d %q, want 301 to ...%s", suffix, r.status, r.header.Get("Location"), want)
		}
	}
	if r := visitor.do("GET", "/client/v1/status/"+old, nil); r.status != http.StatusMovedPermanently || !strings.HasSuffix(r.header.Get("Location"), "/status/"+current) {
		t.Fatalf("JSON: %d %q", r.status, r.header.Get("Location"))
	}
	if r := visitor.do("GET", "/status/"+current, nil); r.status != http.StatusOK {
		t.Fatalf("current slug answers %d", r.status)
	}

	stranger.owner.expectProblem(http.StatusConflict, "slug_taken", "POST", "/v1/status-pages", map[string]any{"inbox_id": stranger.inbox, "name": "Mine", "slug": old})
	mine := stranger.owner.expect(http.StatusCreated, "POST", "/v1/status-pages", map[string]any{"inbox_id": stranger.inbox, "name": "Mine", "slug": unique("st")}).str("id")
	stranger.owner.expectProblem(http.StatusConflict, "slug_taken", "PATCH", "/v1/status-pages/"+mine, map[string]any{"slug": old})

	st.owner.expect(http.StatusOK, "PATCH", st.path(""), map[string]any{"slug": old})
	if r := visitor.do("GET", "/status/"+old, nil); r.status != http.StatusOK {
		t.Fatalf("the page took its own former slug back: %d", r.status)
	}
	if r := visitor.do("GET", "/status/"+current, nil); r.status != http.StatusMovedPermanently {
		t.Fatalf("the slug it left answers %d", r.status)
	}
	stranger.owner.expectProblem(http.StatusConflict, "slug_taken", "PATCH", "/v1/status-pages/"+mine, map[string]any{"slug": current})

	st.owner.expect(http.StatusNoContent, "DELETE", st.path(""), nil)
	if r := visitor.do("GET", "/status/"+current, nil); r.status != http.StatusNotFound {
		t.Fatalf("a deleted page's former slug answers %d", r.status)
	}
	stranger.owner.expect(http.StatusOK, "PATCH", "/v1/status-pages/"+mine, map[string]any{"slug": current})
}

package api_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"uuid"
)

type statusListTeam struct {
	listTeam
	page, slug, statusList string
	api, web               string
}

func newStatusListTeam(t *testing.T, h *harness) statusListTeam {
	t.Helper()
	lt := newListTeam(t, h)
	slug := unique("st")
	pg := lt.owner.expect(http.StatusCreated, "POST", "/v1/status-pages", map[string]any{"inbox_id": lt.chatInbox, "name": "Product news", "slug": slug})
	st := statusListTeam{listTeam: lt, page: pg.str("id"), slug: slug}
	if es := pg.body["email_subscribers"].(map[string]any); es["enabled"] != false || es["list_id"] != nil {
		t.Fatalf("new page email_subscribers: %v", es)
	}
	st.api = lt.owner.expect(http.StatusCreated, "POST", "/v1/status-pages/"+st.page+"/components", map[string]any{"name": "API"}).str("id")
	st.web = lt.owner.expect(http.StatusCreated, "POST", "/v1/status-pages/"+st.page+"/components", map[string]any{"name": "Website"}).str("id")
	lt.owner.expectProblem(http.StatusConflict, "sender_not_ready", "PATCH", "/v1/status-pages/"+st.page, map[string]any{"email_subscribers": map[string]any{"enabled": true}})
	r := lt.owner.expect(http.StatusOK, "PATCH", "/v1/status-pages/"+st.page, map[string]any{"email_subscribers": map[string]any{"enabled": true, "channel_id": lt.emailChannel}})
	es := r.body["email_subscribers"].(map[string]any)
	if es["enabled"] != true || es["channel_id"] != lt.emailChannel || es["list_id"] == nil || es["sender_ready"] != true {
		t.Fatalf("email_subscribers: %v", es)
	}
	st.statusList = es["list_id"].(string)
	return st
}

func (st statusListTeam) statusSubscription(t *testing.T, email string) map[string]any {
	t.Helper()
	for _, it := range items(st.owner.expect(http.StatusOK, "GET", "/v1/lists/"+st.statusList+"/subscriptions?q="+url.QueryEscape(email), nil)) {
		if it["email"] == email {
			return st.owner.expect(http.StatusOK, "GET", "/v1/lists/"+st.statusList+"/subscriptions/"+it["id"].(string), nil).body
		}
	}
	return nil
}

func TestStatusListIsSeparate(t *testing.T) {
	h := newHarness(t)
	st := newStatusListTeam(t, h)

	l := st.owner.expect(http.StatusOK, "GET", "/v1/lists/"+st.statusList, nil)
	if l.body["kind"] != "status" || l.body["status_page_id"] != st.page || l.body["name"] != "Product news (status)" ||
		l.body["public"] != false || l.body["double_opt_in"] != true {
		t.Fatalf("status list: %s", l.raw)
	}
	for _, it := range items(st.owner.expect(http.StatusOK, "GET", "/v1/lists", nil)) {
		if it["id"] == st.statusList {
			t.Fatal("GET /v1/lists shows the status list")
		}
	}
	if got := items(st.owner.expect(http.StatusOK, "GET", "/v1/lists?kind=status", nil)); len(got) != 1 || got[0]["id"] != st.statusList {
		t.Fatalf("kind=status: %v", got)
	}
	st.owner.expectProblem(http.StatusBadRequest, "validation_failed", "GET", "/v1/lists?kind=other", nil)
	st.owner.expectProblem(http.StatusConflict, "list_kind", "PATCH", "/v1/lists/"+st.statusList, map[string]any{"public": true})
	st.owner.expectProblem(http.StatusConflict, "list_kind", "DELETE", "/v1/lists/"+st.statusList, nil)
	st.owner.expectProblem(http.StatusConflict, "list_kind", "GET", "/v1/lists/"+st.statusList+"/consent-export?format=iys", nil)
	st.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/broadcasts", map[string]any{
		"inbox_id": st.chatInbox, "name": "x", "audience": map[string]any{"list_ids": []string{st.statusList}}})
	st.form(h)

	addr := unique("watcher") + "@example.org"
	r := st.owner.expect(http.StatusAccepted, "POST", "/v1/lists/"+st.statusList+"/subscriptions", map[string]any{"email": addr, "consent": map[string]any{"confirmed": true}})
	if r.body["state"] != "pending" {
		t.Fatalf("API subscription of a status list: %s", r.raw)
	}
	mails := h.runListMail(st.ws)
	if len(mails) != 1 || !strings.Contains(mails[0].GetHeader("Subject"), "Product news") || !strings.Contains(mails[0].Text, "incidents and maintenance") {
		t.Fatalf("status confirmation: %d", len(mails))
	}
	confirm := linkIn(t, mails[0], "s")
	if status, page := pageAt(t, h, "GET", confirm, nil); status != http.StatusOK || !strings.Contains(page, "incidents and maintenance") {
		t.Fatalf("GET /s/: %d %s", status, page)
	}
	if status, page := pageAt(t, h, "POST", confirm, url.Values{"action": {"confirm"}}); status != http.StatusOK || !strings.Contains(page, "You will get notices") {
		t.Fatalf("POST /s/: %d %s", status, page)
	}
	if welcome := h.runListMail(st.ws); len(welcome) != 0 {
		t.Fatal("a status list sent a welcome mail")
	}

	st.owner.expect(http.StatusCreated, "POST", "/v1/lists/"+st.list+"/subscriptions", map[string]any{"email": addr, "consent": map[string]any{"confirmed": true}})
	welcome := h.runListMail(st.ws)
	if len(welcome) != 1 {
		t.Fatalf("%d welcome mails", len(welcome))
	}
	manage := linkIn(t, welcome[0], "m")
	if _, page := pageAt(t, h, "GET", manage, nil); strings.Contains(page, st.statusList) {
		t.Fatal("a marketing /m/ page offers the status list")
	}
	if status, page := pageAt(t, h, "POST", manage, url.Values{"action": {"all"}}); status != http.StatusOK || !strings.Contains(page, "no more list mail") {
		t.Fatalf("unsubscribe all: %d %s", status, page)
	}
	sub := st.statusSubscription(t, addr)
	if sub["state"] != "subscribed" {
		t.Fatalf("unsubscribe from everything ended the status subscription: %v", sub["state"])
	}

	contact := uuid.MustParse(sub["contact"].(map[string]any)["id"].(string))
	statusManage := strings.TrimPrefix(h.srv.ManageURL(uuid.MustParse(st.ws), uuid.MustParse(st.statusList), contact), testOrigin)
	status, page := pageAt(t, h, "GET", statusManage, nil)
	if status != http.StatusOK || !strings.Contains(page, "Back to the status page") || !strings.Contains(page, `name="components"`) || strings.Contains(page, `value="all"`) {
		t.Fatalf("status /m/: %d %s", status, page)
	}
	if _, page := pageAt(t, h, "POST", statusManage, url.Values{"action": {"components"}}); !strings.Contains(page, "Choose at least one component") {
		t.Fatalf("no components: %s", page)
	}
	if _, page := pageAt(t, h, "POST", statusManage, url.Values{"action": {"components"}, "components": {st.api}}); !strings.Contains(page, "Your choices are saved") {
		t.Fatalf("components saved: %s", page)
	}
	if sub := st.statusSubscription(t, addr); len(sub["components"].([]any)) != 1 || sub["components"].([]any)[0] != st.api {
		t.Fatalf("components: %v", sub["components"])
	}
	if status, body := pageAt(t, h, "POST", statusManage, url.Values{"List-Unsubscribe": {"One-Click"}}); status != http.StatusOK || body != "" {
		t.Fatalf("one-click: %d %q", status, body)
	}
	if sub := st.statusSubscription(t, addr); sub["state"] != "unsubscribed" {
		t.Fatalf("after one-click: %v", sub["state"])
	}
	var optouts int
	if err := h.st.Pool.QueryRow(context.Background(), "SELECT count(*) FROM email_optouts WHERE workspace_id = $1", st.ws).Scan(&optouts); err != nil {
		t.Fatal(err)
	}
	if optouts != 1 {
		t.Fatalf("%d opt-outs, want only the marketing one", optouts)
	}
	st.owner.expectProblem(http.StatusConflict, "unsubscribed", "POST", "/v1/lists/"+st.statusList+"/subscriptions", map[string]any{"email": addr, "consent": map[string]any{"confirmed": true}})
	objector := unique("objector") + "@example.org"
	st.owner.expect(http.StatusCreated, "POST", "/v1/lists/"+st.list+"/subscriptions", map[string]any{"email": objector, "consent": map[string]any{"confirmed": true}})
	pageAt(t, h, "POST", linkIn(t, h.runListMail(st.ws)[0], "m"), url.Values{"action": {"all"}})
	st.owner.expectProblem(http.StatusConflict, "opted_out", "POST", "/v1/lists/"+st.list+"/subscriptions", map[string]any{"email": objector, "consent": map[string]any{"confirmed": true}})
	if r := st.owner.expect(http.StatusAccepted, "POST", "/v1/lists/"+st.statusList+"/subscriptions", map[string]any{"email": objector, "consent": map[string]any{"confirmed": true}}); r.body["state"] != "pending" {
		t.Fatalf("status subscription of an opted-out address: %s", r.raw)
	}

	other := newListTeam(t, h)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/lists/"+st.statusList, nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "PATCH", "/v1/status-pages/"+st.page, map[string]any{"email_subscribers": map[string]any{"enabled": false}})
	other.owner.expectProblem(http.StatusConflict, "sender_not_ready", "PATCH", "/v1/status-pages/"+newStatusListTeamPage(t, other), map[string]any{
		"email_subscribers": map[string]any{"channel_id": st.emailChannel}})

	st.owner.expect(http.StatusNoContent, "DELETE", "/v1/status-pages/"+st.page, nil)
	st.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/lists/"+st.statusList, nil)
}

func newStatusListTeamPage(t *testing.T, lt listTeam) string {
	t.Helper()
	return lt.owner.expect(http.StatusCreated, "POST", "/v1/status-pages", map[string]any{"inbox_id": lt.chatInbox, "name": "Other", "slug": unique("st")}).str("id")
}

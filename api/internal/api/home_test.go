package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/api"
)

func homeOf(c *client) response {
	c.h.t.Helper()
	return c.expect(http.StatusOK, "GET", "/v1/home", nil)
}

func homeBlock(r response, name string) map[string]any {
	b, _ := r.body[name].(map[string]any)
	return b
}

func checklist(t *testing.T, r response) map[string][2]any {
	t.Helper()
	out := map[string][2]any{}
	var order []string
	for _, it := range homeBlock(r, "setup")["items"].([]any) {
		m := it.(map[string]any)
		out[m["id"].(string)] = [2]any{m["done"], m["count"]}
		order = append(order, m["id"].(string))
	}
	out["order"] = [2]any{fmt.Sprint(order), nil}
	return out
}

func messageTime(t *testing.T, c *client, conv, id string) time.Time {
	t.Helper()
	for _, m := range messages(c, conv) {
		if m["id"] == id {
			at, err := time.Parse(time.RFC3339Nano, m["created_at"].(string))
			if err != nil {
				t.Fatal(err)
			}
			return at
		}
	}
	t.Fatalf("message %s not in %s", id, conv)
	return time.Time{}
}

func setBroadcastSent(t *testing.T, h *harness, id string, finished time.Time, counts map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(counts)
	if _, err := h.st.Pool.Exec(context.Background(), `UPDATE broadcasts SET state = 'sent', started_at = $2, finished_at = $2, counts = $3, click_tracking = true WHERE id = $1`,
		id, finished, raw); err != nil {
		t.Fatal(err)
	}
}

func TestHomeEmptyWorkspace(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	r := homeOf(tm.owner)
	inbox := homeBlock(r, "inbox")
	if inbox["open"] != float64(0) || inbox["unassigned"] != float64(0) || inbox["waiting"] != float64(0) || inbox["oldest_waiting"] != nil {
		t.Fatalf("inbox of an empty workspace: %s", r.raw)
	}
	if props := homeBlock(r, "analytics")["properties"].([]any); len(props) != 0 {
		t.Fatalf("analytics of an empty workspace: %s", r.raw)
	}
	if b := homeBlock(r, "broadcasts"); b == nil || b["last_sent"] != nil {
		t.Fatalf("broadcasts of an empty workspace: %s", r.raw)
	}
	got := checklist(t, r)
	if got["order"][0] != "[chat email app docs analytics newsletter]" {
		t.Fatalf("checklist order: %v", got["order"][0])
	}
	for _, id := range []string{"chat", "email", "app", "docs", "analytics", "newsletter"} {
		if got[id] != [2]any{false, float64(0)} {
			t.Fatalf("%s of an empty workspace: %v", id, got[id])
		}
	}

	key := tm.apiKey(h)
	key.expectProblem(http.StatusForbidden, "member_session_required", "GET", "/v1/home", nil)
	h.client().expectProblem(http.StatusUnauthorized, "unauthenticated", "GET", "/v1/home", nil)
}

func TestHomeInboxCounts(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	key := tm.apiKey(h)
	tm.owner.expect(http.StatusOK, "PATCH", "/v1/workspace", map[string]any{"bots_may_send": true})
	tm.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+tm.inbox+"/members/"+tm.agentID, nil)
	second := tm.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Billing", "slug": "billing"}).body["inbox"].(map[string]any)["id"].(string)

	hidden := tm.owner.expect(http.StatusCreated, "POST", "/v1/conversations", map[string]any{"inbox_id": second, "contact_id": tm.contact, "subject": "Invoice"}).str("id")
	post(key, hidden, map[string]any{"kind": "message", "direction": "in", "body": "where is my invoice?"})

	answered := tm.conversation(tm.owner)
	post(key, answered, map[string]any{"kind": "message", "direction": "in", "body": "first"})
	post(tm.owner, answered, map[string]any{"kind": "message", "body": "on it"})
	again := post(key, answered, map[string]any{"kind": "message", "direction": "in", "body": "still broken"})
	post(key, answered, map[string]any{"kind": "message", "direction": "in", "body": "hello?"})

	noted := tm.conversation(tm.owner)
	post(key, noted, map[string]any{"kind": "message", "direction": "in", "body": "help"})
	post(tm.owner, noted, map[string]any{"kind": "note", "body": "a note is no reply"})

	byBot := tm.conversation(tm.owner)
	post(key, byBot, map[string]any{"kind": "message", "direction": "in", "body": "help"})
	post(key, byBot, map[string]any{"kind": "message", "direction": "out", "body": "the bot answered"})
	tm.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+byBot, map[string]any{"assignee_id": tm.agentID})

	closed := tm.conversation(tm.owner)
	post(key, closed, map[string]any{"kind": "message", "direction": "in", "body": "help"})
	tm.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+closed, map[string]any{"status": "closed"})

	r := homeOf(tm.owner)
	inbox := homeBlock(r, "inbox")
	counts := tm.owner.expect(http.StatusOK, "GET", "/v1/conversations/counts", nil)
	if inbox["open"] != float64(4) || inbox["open"] != counts.body["all"] || inbox["unassigned"] != float64(3) || inbox["unassigned"] != counts.body["unassigned"] || inbox["waiting"] != float64(3) {
		t.Fatalf("owner's inbox: %s, counts %s", r.raw, counts.raw)
	}
	if w := inbox["oldest_waiting"].(map[string]any); w["id"] != hidden || w["inbox_id"] != second || w["subject"] != "Invoice" {
		t.Fatalf("owner's oldest waiting: %s", r.raw)
	}

	r = homeOf(tm.agent)
	inbox = homeBlock(r, "inbox")
	if inbox["open"] != float64(3) || inbox["unassigned"] != float64(2) || inbox["waiting"] != float64(2) {
		t.Fatalf("agent's inbox: %s", r.raw)
	}
	w := inbox["oldest_waiting"].(map[string]any)
	contact := w["contact"].(map[string]any)
	since, _ := time.Parse(time.RFC3339Nano, w["since"].(string))
	if w["id"] != answered || contact["id"] != tm.contact || contact["name"] != "Ayşe Yılmaz" || contact["email"] != "ayse@example.com" ||
		!since.Equal(messageTime(t, tm.owner, answered, again)) {
		t.Fatalf("agent's oldest waiting is not the first message after the reply: %s", r.raw)
	}
	if r.body["setup"] != nil {
		t.Fatalf("an agent gets the setup checklist: %s", r.raw)
	}

	if inbox := homeBlock(homeOf(other.owner), "inbox"); inbox["open"] != float64(0) || inbox["oldest_waiting"] != nil {
		t.Fatalf("another workspace sees %v", inbox)
	}
}

func TestHomeAnalytics(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	ctx := context.Background()
	tm.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+tm.inbox+"/members/"+tm.agentID, nil)
	second := tm.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Other", "slug": "other"}).body["inbox"].(map[string]any)["id"].(string)
	h.clock.Set(time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC))
	linked, key := createProperty(tm.owner, map[string]any{"name": "Linked", "domains": []string{"example.com"}, "inbox_id": tm.inbox})
	elsewhere, otherKey := createProperty(tm.owner, map[string]any{"name": "Elsewhere", "domains": []string{"example.com"}, "inbox_id": second})
	unlinked, _ := createProperty(tm.owner, map[string]any{"name": "Unlinked", "domains": []string{"example.com"}})

	visitors := func(at time.Time, n int, prefix string) {
		h.clock.Set(at)
		var steps []visitStep
		for i := range n {
			steps = append(steps, visitStep{ua: uaMac, ip: fmt.Sprintf("2001:db8:%s%d::1", prefix, i+1), page: "https://example.com/"})
		}
		sendVisits(h, key, steps)
	}
	visitors(time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC), 5, "a")
	if err := h.srv.RollupAnalyticsDay(ctx, uuid.MustParse(tm.ws), uuid.MustParse(linked), time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	visitors(time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC), 2, "b")
	visitors(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC), 3, "c")
	visitors(time.Date(2026, 10, 12, 9, 50, 0, 0, time.UTC), 2, "d")
	h.clock.Set(time.Date(2026, 10, 12, 9, 50, 0, 0, time.UTC))
	sendVisits(h, otherKey, []visitStep{{ua: uaPixel, ip: "198.51.100.9", page: "https://example.com/"}})
	h.clock.Set(time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC))

	r := homeOf(tm.owner)
	props := homeBlock(r, "analytics")["properties"].([]any)
	var ids []string
	for _, p := range props {
		ids = append(ids, p.(map[string]any)["id"].(string))
	}
	if !slices.Equal(ids, []string{linked, elsewhere, unlinked}) {
		t.Fatalf("owner's properties: %v", ids)
	}
	p := props[0].(map[string]any)
	var days []string
	for _, d := range p["days"].([]any) {
		m := d.(map[string]any)
		days = append(days, fmt.Sprintf("%s=%v", m["day"], m["visitors"]))
	}
	want := []string{"2026-10-06=0", "2026-10-07=0", "2026-10-08=0", "2026-10-09=0", "2026-10-10=5", "2026-10-11=2", "2026-10-12=5"}
	if !slices.Equal(days, want) || p["visitors_today"] != float64(5) || p["realtime_visitors"] != float64(2) || p["timezone"] != "UTC" || p["inbox_id"] != tm.inbox {
		t.Fatalf("linked property: %v %s", days, r.raw)
	}

	q := query(tm.owner, linked, map[string]any{"metrics": []string{"visitors"}, "range": map[string]any{"from": "2026-10-06", "to": "2026-10-12"}, "dimensions": []string{"time:day"}})
	perDay := map[string]float64{}
	for _, row := range q.body["rows"].([]any) {
		m := row.(map[string]any)
		perDay[m["dimensions"].([]any)[0].(string)] = m["metrics"].(map[string]any)["visitors"].(float64)
	}
	for _, d := range p["days"].([]any) {
		m := d.(map[string]any)
		if perDay[m["day"].(string)] != m["visitors"].(float64) {
			t.Fatalf("home %v and the query %v disagree", days, perDay)
		}
	}
	rt := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+linked+"/realtime", nil)
	listed := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties", nil).body["items"].([]any)[0].(map[string]any)
	if rt.body["visitors"] != p["realtime_visitors"] || listed["visitors_today"] != p["visitors_today"] {
		t.Fatalf("realtime %v, list %v, home %v/%v", rt.body["visitors"], listed["visitors_today"], p["realtime_visitors"], p["visitors_today"])
	}
	if e := props[1].(map[string]any); e["realtime_visitors"] != float64(1) || e["visitors_today"] != float64(1) {
		t.Fatalf("second property: %v", e)
	}

	props = homeBlock(homeOf(tm.agent), "analytics")["properties"].([]any)
	if len(props) != 1 || props[0].(map[string]any)["id"] != linked {
		t.Fatalf("agent's properties: %v", props)
	}
	tm.owner.expect(http.StatusNoContent, "DELETE", "/v1/inboxes/"+tm.inbox+"/members/"+tm.agentID, nil)
	if r := homeOf(tm.agent); r.body["analytics"] != nil {
		t.Fatalf("an agent who sees no property gets analytics: %s", r.raw)
	}
}

func TestHomeWithoutAnalyticsOrBroadcasts(t *testing.T) {
	h := newHarnessWith(t, func(d *api.Deps) { d.Analytics.Off, d.Broadcasts.Mode = true, "off" })
	tm := newTeam(t, h)
	r := homeOf(tm.owner)
	if r.body["analytics"] != nil || r.body["broadcasts"] != nil {
		t.Fatalf("a server without analytics and broadcasts: %s", r.raw)
	}
	if got := checklist(t, r); got["order"][0] != "[chat email app docs newsletter]" {
		t.Fatalf("checklist without analytics: %v", got["order"][0])
	}
}

func sentAt(b map[string]any) time.Time {
	at, _ := time.Parse(time.RFC3339Nano, b["sent_at"].(string))
	return at
}

func TestHomeLastBroadcast(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	tm.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+tm.inbox+"/members/"+tm.agentID, nil)
	second := tm.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Other", "slug": "other"}).body["inbox"].(map[string]any)["id"].(string)
	mine := tm.owner.expect(http.StatusCreated, "POST", "/v1/broadcasts", map[string]any{"inbox_id": tm.inbox, "name": "October news"}).str("id")
	hidden := tm.owner.expect(http.StatusCreated, "POST", "/v1/broadcasts", map[string]any{"inbox_id": second, "name": "Billing news"}).str("id")
	tm.owner.expect(http.StatusCreated, "POST", "/v1/broadcasts", map[string]any{"inbox_id": tm.inbox, "name": "Draft"})
	if b := homeBlock(homeOf(tm.owner), "broadcasts"); b["last_sent"] != nil {
		t.Fatalf("drafts count as sent: %v", b)
	}
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	setBroadcastSent(t, h, mine, at, map[string]any{"recipients": 120, "sent": 118, "clicked": 30, "replied": 4})
	setBroadcastSent(t, h, hidden, at.Add(time.Hour), map[string]any{"recipients": 9, "sent": 9})

	b := homeBlock(homeOf(tm.owner), "broadcasts")["last_sent"].(map[string]any)
	if b["id"] != hidden {
		t.Fatalf("owner's last broadcast: %v", b)
	}
	b = homeBlock(homeOf(tm.agent), "broadcasts")["last_sent"].(map[string]any)
	if b["id"] != mine || b["name"] != "October news" || !sentAt(b).Equal(at) || b["recipients"] != float64(120) ||
		b["sent"] != float64(118) || b["clicked"] != float64(30) || b["replied"] != float64(4) || b["click_tracking"] != true {
		t.Fatalf("agent's last broadcast: %v", b)
	}
}

func TestHomeSetupChecklistAgreesWithMCP(t *testing.T) {
	h := newHarness(t)
	ct := newChatTeam(t, h, "live", true)
	tm := ct.team
	inboxB := tm.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Billing", "slug": "billing"}).body["inbox"].(map[string]any)["id"].(string)
	docs := tm.owner.expect(http.StatusCreated, "POST", "/v1/inboxes/"+inboxB+"/channels", map[string]any{
		"kind": "chat", "name": "Docs", "chat": map[string]any{"allowed_origins": []string{"https://docs.example.net"}, "allow_anonymous": true},
	}).body["chat"].(map[string]any)["public_key"].(string)
	tm.owner.expect(http.StatusCreated, "POST", "/v1/inboxes/"+inboxB+"/channels", map[string]any{
		"kind": "email", "name": "Support mail",
		"email": map[string]any{"address": unique("support") + "@example.com", "display_name": "Acme", "smtp": map[string]any{"host": "smtp.example.com", "port": 2525, "tls": "starttls"}},
	})
	tm.owner.expect(http.StatusCreated, "POST", "/v1/inboxes/"+tm.inbox+"/channels", map[string]any{"kind": "app", "name": "iOS and Android"})
	createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"example.com"}, "inbox_id": ct.chatInbox})
	createProperty(tm.owner, map[string]any{"name": "Docs", "domains": []string{"docs.example.net", "*.example.org"}})
	tm.owner.expect(http.StatusCreated, "POST", "/v1/lists", map[string]any{"inbox_id": ct.chatInbox, "name": "Product news"})
	tm.owner.expect(http.StatusCreated, "POST", "/v1/lists", map[string]any{"inbox_id": inboxB, "name": "Billing news"})

	got := checklist(t, homeOf(tm.owner))
	if got["chat"] != [2]any{true, float64(2)} || got["email"] != [2]any{true, float64(1)} || got["app"] != [2]any{true, float64(1)} ||
		got["docs"] != [2]any{false, float64(0)} || got["analytics"] != [2]any{true, float64(2)} || got["newsletter"] != [2]any{true, float64(2)} {
		t.Fatalf("checklist: %v", got)
	}
	site := h.client()
	site.origin = "https://docs.example.net"
	site.expect(http.StatusNoContent, "POST", "/client/v1/channels/"+docs+"/page-ratings", map[string]any{"page": "https://docs.example.net/start", "rating": "up"})
	if got := checklist(t, homeOf(tm.owner)); got["docs"] != [2]any{true, float64(1)} || got["chat"] != [2]any{true, float64(2)} {
		t.Fatalf("a rated docs site: %v", got)
	}

	cs := h.mcpSession(h.oauthToken(tm.owner, tm.ws, "Claude Code", testOrigin+"/mcp", []string{"inboxes:read", "analytics:read", "lists:read"}))
	state, errText := callTool(t, cs, "get_setup_state", map[string]any{})
	if errText != "" {
		t.Fatal(errText)
	}
	if state["not_visible"] != nil {
		t.Fatalf("not visible: %v", state["not_visible"])
	}

	type kv = map[string]any
	pick := func(m kv, keys ...string) kv {
		out := kv{}
		for _, k := range keys {
			if v, ok := m[k]; ok {
				out[k] = v
			}
		}
		return out
	}
	var wantInboxes []any
	for _, in := range tm.owner.expect(http.StatusOK, "GET", "/v1/inboxes", nil).body["items"].([]any) {
		in := in.(kv)
		si := kv{"id": in["id"], "name": in["name"]}
		var chs []any
		for _, ch := range tm.owner.expect(http.StatusOK, "GET", "/v1/inboxes/"+in["id"].(string)+"/channels", nil).body["items"].([]any) {
			ch := ch.(kv)
			c := pick(ch, "id", "inbox_id", "kind", "name", "created_at")
			if chat, ok := ch["chat"].(kv); ok {
				for k, v := range pick(chat, "public_key", "allowed_origins", "allow_anonymous", "discussions", "greeting") {
					c[k] = v
				}
			}
			if app, ok := ch["app"].(kv); ok {
				for k, v := range pick(app, "public_key", "allow_anonymous", "platforms") {
					c[k] = v
				}
			}
			if e, ok := ch["email"].(kv); ok {
				c["email_address"] = e["address"]
				if f, ok := e["from_address"]; ok {
					c["from_address"] = f
				}
			}
			if g, ok := c["greeting"]; ok && g == "" {
				delete(c, "greeting")
			}
			chs = append(chs, c)
		}
		if chs != nil {
			si["channels"] = chs
		}
		wantInboxes = append(wantInboxes, si)
	}
	var wantProps []any
	for _, p := range tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties", nil).body["items"].([]any) {
		wantProps = append(wantProps, pick(p.(kv), "id", "name", "domains", "platforms", "inbox_id"))
	}
	var wantLists []any
	for _, l := range tm.owner.expect(http.StatusOK, "GET", "/v1/lists?limit=100", nil).body["items"].([]any) {
		wantLists = append(wantLists, pick(l.(kv), "id", "name", "inbox_id"))
	}
	for name, want := range map[string][]any{"inboxes": wantInboxes, "analytics_properties": wantProps, "mailing_lists": wantLists} {
		a, _ := json.Marshal(state[name])
		b, _ := json.Marshal(want)
		if string(a) != string(b) {
			t.Fatalf("get_setup_state %s differs from the list endpoints:\n%s\n%s", name, a, b)
		}
	}
}

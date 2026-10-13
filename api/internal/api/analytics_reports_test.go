package api_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jhillyerd/enmime/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/productdevbook/yuva/api/internal/analytics"
)

// viewsFrom collects n page views of page from n visitors of the site.
func viewsFrom(h *harness, key, origin, page string, n, first int) {
	h.t.Helper()
	for i := range n {
		ip := fmt.Sprintf("81.%d.%d.10", (first+i)/256, (first+i)%256)
		r := collect(h, key, collectReq{origin: origin, ip: ip, header: map[string]string{"User-Agent": uaMac}, body: map[string]any{"n": "pageview", "u": origin + page}})
		if r.status != http.StatusAccepted {
			h.t.Fatalf("collect %d %s", r.status, r.raw)
		}
	}
}

func supportRows(r response) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, row := range r.body["rows"].([]any) {
		m := row.(map[string]any)
		out[m["dimensions"].([]any)[0].(string)] = m["metrics"].(map[string]any)
	}
	return out
}

func supportQuery(r map[string]any) map[string]any {
	q := map[string]any{"dataset": "support", "dimensions": []string{"page"},
		"metrics": []string{"conversations", "pageviews", "support_rate", "feedback", "ratings_up", "ratings_down", "helpful_rate", "discussions"}}
	for k, v := range r {
		q[k] = v
	}
	return q
}

// seedSupport gives a docs site three pages with support and one without: /docs/billing leads to
// six conversations in 120 views, /docs/install to one in 150 with feedback, ratings and a thread,
// /pricing to one in 5, /docs/home to none in 400.
func seedSupport(t *testing.T, h *harness, ct chatTeam) (string, string) {
	t.Helper()
	host := strings.TrimPrefix(ct.origin, "https://")
	id, key := createProperty(ct.owner, map[string]any{"name": "Docs", "domains": []string{host}, "timezone": "UTC", "inbox_id": ct.chatInbox})
	viewsFrom(h, key, ct.origin, "/docs/billing", 120, 0)
	viewsFrom(h, key, ct.origin, "/docs/install", 150, 1000)
	viewsFrom(h, key, ct.origin, "/pricing", 5, 2000)
	viewsFrom(h, key, ct.origin, "/docs/home", 400, 3000)
	h.flushAnalytics()
	for i := range 6 {
		cs := ct.session(h, map[string]any{})
		cs.expect(http.StatusCreated, "POST", "/client/v1/conversations", map[string]any{"body": fmt.Sprintf("billing question %d", i),
			"page_url": ct.origin + "/docs/billing?plan=pro#vat", "page_title": "Billing", "client_id": unique("c")})
	}
	for _, page := range []string{"/docs/install", "/pricing"} {
		cs := ct.session(h, map[string]any{})
		cs.expect(http.StatusCreated, "POST", "/client/v1/conversations", map[string]any{"body": "help", "page_url": ct.origin + page, "client_id": unique("c")})
	}
	cs := ct.session(h, map[string]any{})
	cs.expect(http.StatusCreated, "POST", "/client/v1/feedback", map[string]any{"body": "step 2 fails", "page_url": ct.origin + "/docs/install",
		"rating": "down", "client_id": unique("c")})
	site := h.client()
	site.origin = ct.origin
	for _, rating := range []string{"up", "up", "up", "down"} {
		site.expect(http.StatusNoContent, "POST", "/client/v1/channels/"+ct.key+"/page-ratings", map[string]any{"page": ct.origin + "/docs/install", "rating": rating})
	}
	user := ct.session(h, map[string]any{"identity_token": ct.token(h, map[string]any{"sub": "u-" + unique("d"), "name": "Ada Lovelace"})})
	user.expect(http.StatusCreated, "POST", "/client/v1/discussions", map[string]any{"page_url": ct.origin + "/docs/install", "title": "Does it run on ARM?", "body": "Asking."})
	return id, key
}

func TestAnalyticsSupportReport(t *testing.T) {
	h := newHarness(t)
	h.clock.Set(time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC))
	ct := newChatTeam(t, h, "live", true)
	other := newTeam(t, h)
	id, _ := seedSupport(t, h, ct)

	convs := ct.owner.expect(http.StatusOK, "GET", "/v1/conversations?inbox_id="+ct.chatInbox+"&limit=100", nil).body["items"].([]any)
	pages := 0
	for _, c := range convs {
		if p, ok := c.(map[string]any)["page"].(map[string]any); ok && p["page_url"] == ct.origin+"/docs/billing" {
			pages++
		}
	}
	if pages != 6 {
		t.Fatalf("%d conversations carry the billing page without query and fragment", pages)
	}

	r := query(ct.owner, id, supportQuery(map[string]any{"range": map[string]any{"period": "today"}, "insights": true}))
	rows := r.body["rows"].([]any)
	order := []string{}
	for _, row := range rows {
		order = append(order, row.(map[string]any)["dimensions"].([]any)[0].(string))
	}
	if strings.Join(order, " ") != "/docs/billing /docs/install /pricing" {
		t.Fatalf("support report order %v: %s", order, r.raw)
	}
	byPage := supportRows(r)
	if b := byPage["/docs/billing"]; b["conversations"] != 6.0 || b["pageviews"] != 120.0 || b["support_rate"] != 50.0 {
		t.Fatalf("billing %v", b)
	}
	if in := byPage["/docs/install"]; in["pageviews"] != 150.0 || in["ratings_up"] != 3.0 || in["ratings_down"] != 1.0 || in["helpful_rate"] != 75.0 ||
		in["feedback"] != 1.0 || in["discussions"] != 1.0 || in["conversations"] != 1.0 || in["support_rate"] != 6.67 {
		t.Fatalf("a docs page's views next to its ratings %v", in)
	}
	if tot := totals(r); tot["conversations"] != 8.0 || tot["pageviews"] != 675.0 {
		t.Fatalf("totals %v", tot)
	}
	ins := r.body["insights"].([]any)
	if len(ins) != 1 || ins[0].(map[string]any)["kind"] != "support_hotspot" || ins[0].(map[string]any)["value"] != "/docs/billing" {
		t.Fatalf("insights %v", ins)
	}
	byConversations := query(ct.owner, id, supportQuery(map[string]any{"range": map[string]any{"period": "today"},
		"order_by": []map[string]any{{"metric": "support_rate"}}}))
	if first := byConversations.body["rows"].([]any)[0].(map[string]any)["dimensions"].([]any)[0]; first != "/pricing" {
		t.Fatalf("an explicit order ranks by rate alone, got %v first", first)
	}
	series := query(ct.owner, id, map[string]any{"dataset": "support", "metrics": []string{"conversations", "pageviews"},
		"range": map[string]any{"from": "2026-10-11", "to": "2026-10-12"}, "dimensions": []string{"time:day"}})
	if day := supportRows(series)["2026-10-12"]; len(series.body["rows"].([]any)) != 2 || day["conversations"] != 8.0 || day["pageviews"] != 675.0 {
		t.Fatalf("series %s", series.raw)
	}

	ct.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties/"+id+"/query",
		map[string]any{"metrics": []string{"conversations"}, "range": map[string]any{"period": "today"}})
	ct.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties/"+id+"/query",
		supportQuery(map[string]any{"range": map[string]any{"period": "today"}, "dimensions": []string{"country"}}))
	ct.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties/"+id+"/query",
		map[string]any{"dataset": "support", "metrics": []string{"bounce_rate"}, "range": map[string]any{"period": "today"}})
	unlinked, _ := createProperty(ct.owner, map[string]any{"name": "Unlinked", "domains": []string{"example.com"}})
	ct.owner.expectProblem(http.StatusUnprocessableEntity, "no_linked_inbox", "POST", "/v1/analytics/properties/"+unlinked+"/query",
		supportQuery(map[string]any{"range": map[string]any{"period": "today"}}))
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/analytics/properties/"+id+"/query",
		supportQuery(map[string]any{"range": map[string]any{"period": "today"}}))

	h.clock.Set(time.Date(2026, 10, 13, 3, 0, 0, 0, time.UTC))
	if err := h.srv.RollupAnalyticsDay(context.Background(), uuid.MustParse(ct.ws), uuid.MustParse(id), time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.st.Pool.Exec(context.Background(), `DELETE FROM page_ratings WHERE workspace_id = $1`, ct.ws); err != nil {
		t.Fatal(err)
	}
	rolled := query(ct.owner, id, supportQuery(map[string]any{"range": dayRange("2026-10-12")}))
	if served(rolled) != "rollups" || numbers(rolled) != numbers(query(ct.owner, id, supportQuery(map[string]any{"range": dayRange("2026-10-12")}))) {
		t.Fatalf("served from %s", served(rolled))
	}
	if in := supportRows(rolled)["/docs/install"]; in["ratings_up"] != 3.0 || in["pageviews"] != 150.0 {
		t.Fatalf("a rolled-up day reads the rollup, not the inbox: %v", in)
	}
}

func conversationPageOf(t *testing.T, c *client, id string) any {
	t.Helper()
	return c.expect(http.StatusOK, "GET", "/v1/conversations/"+id, nil).body["page"]
}

func TestConversationPageNeverRefusesAConversation(t *testing.T) {
	h := newHarness(t)
	ct := newChatTeam(t, h, "live", true)
	start := func(cs contactSession, body map[string]any) string {
		body["body"], body["client_id"] = "hello", unique("c")
		return cs.expect(http.StatusCreated, "POST", "/client/v1/conversations", body).body["conversation"].(map[string]any)["id"].(string)
	}
	chat := ct.session(h, map[string]any{})
	kept := start(chat, map[string]any{"page_url": ct.origin + "/docs/billing?plan=pro#vat", "page_title": "Billing"})
	if p, ok := conversationPageOf(t, ct.owner, kept).(map[string]any); !ok || p["page_url"] != ct.origin+"/docs/billing" || p["page_title"] != "Billing" {
		t.Fatalf("an allowed page was not kept: %v", p)
	}
	for _, body := range []map[string]any{
		{"page_url": "https://elsewhere.example/docs", "page_title": "Elsewhere"},
		{"page_title": "No page"},
		{"page_url": "not a url"},
		{"page_url": ct.origin + "/" + strings.Repeat("a", 2100)},
	} {
		if p := conversationPageOf(t, ct.owner, start(ct.session(h, map[string]any{}), body)); p != nil {
			t.Fatalf("%v stored the page %v", body, p)
		}
	}
	at := newAppTeam(t, h, false)
	app := at.session(h, "u-"+unique("a"))
	id := start(app, map[string]any{"page_url": "https://web.example/help", "page_title": "Help"})
	if p := conversationPageOf(t, at.owner, id); p != nil {
		t.Fatalf("an app channel stored the page %v", p)
	}
}

func TestAnalyticsMCPSupportTools(t *testing.T) {
	h := newHarness(t)
	h.clock.Set(time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC))
	ct := newChatTeam(t, h, "live", true)
	id, _ := seedSupport(t, h, ct)

	reader := ct.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "analytics", "scopes": []string{"analytics:read"}}).str("secret")
	cs := h.mcpSession(reader)
	names := toolNames(t, cs)
	for _, want := range []string{"list_properties", "analytics_overview", "analytics_query", "analytics_realtime", "analytics_support_report",
		"list_goals", "funnel_report", "add_annotation", "get_analytics_snippet"} {
		if !slices.Contains(names, want) {
			t.Fatalf("analytics:read lists %v, no %s", names, want)
		}
	}
	for _, manage := range []string{"create_goal", "create_property"} {
		if slices.Contains(names, manage) {
			t.Fatalf("analytics:read lists %s", manage)
		}
	}
	props, errText := callTool(t, cs, "list_properties", nil)
	if errText != "" || len(props["items"].([]any)) != 1 {
		t.Fatalf("list_properties %v %s", props, errText)
	}
	out, errText := callTool(t, cs, "analytics_support_report", map[string]any{"property_id": id, "period": "month", "order_by": "conversations"})
	if errText != "" {
		t.Fatal(errText)
	}
	top := out["rows"].([]any)[0].(map[string]any)
	if page := top["customer_content"].(map[string]any)["dimensions"].([]any)[0]; page != "/docs/billing" ||
		top["metrics"].(map[string]any)["conversations"] != 6.0 || top["metrics"].(map[string]any)["support_rate"] != 50.0 {
		t.Fatalf("which pages lead to the most conversations this month: %v", out)
	}
	for _, needle := range []string{"/docs/billing", "/docs/install"} {
		if at := outsideCustomerContent(out, needle, ""); len(at) > 0 {
			t.Fatalf("a page path outside customer_content at %v", at)
		}
	}
	overview, errText := callTool(t, cs, "analytics_overview", map[string]any{"property_id": id, "period": "today"})
	if errText != "" || overview["totals"].(map[string]any)["metrics"].(map[string]any)["pageviews"] != 675.0 {
		t.Fatalf("overview %v %s", overview, errText)
	}
	if at := outsideCustomerContent(overview, "/docs/home", ""); len(at) > 0 {
		t.Fatalf("a page path outside customer_content at %v", at)
	}
	snippet, errText := callTool(t, cs, "get_analytics_snippet", map[string]any{"property_id": id, "framework": "html"})
	if errText != "" || !strings.Contains(snippet["code"].(string), "/yuva-analytics.js") {
		t.Fatalf("snippet %v %s", snippet, errText)
	}
	res, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "yuva://property/" + id})
	if err != nil || !strings.Contains(res.Contents[0].Text, `"overview"`) {
		t.Fatalf("property resource %v %v", res, err)
	}
	prompts, err := cs.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var promptNames []string
	for _, p := range prompts.Prompts {
		promptNames = append(promptNames, p.Name)
	}
	if !slices.Contains(promptNames, "analytics_review") || slices.Contains(promptNames, "setup_analytics") {
		t.Fatalf("prompts %v", promptNames)
	}

	other := ct.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "support", "scopes": []string{"conversations:read"}}).str("secret")
	ocs := h.mcpSession(other)
	for _, n := range toolNames(t, ocs) {
		if strings.Contains(n, "analytics") || n == "list_properties" || n == "list_goals" || n == "funnel_report" || n == "add_annotation" ||
			n == "create_goal" || n == "create_property" {
			t.Fatalf("a token without analytics:read lists %s", n)
		}
	}
	if _, errText := callTool(t, ocs, "analytics_query", map[string]any{"property_id": id, "metrics": []string{"visitors"}}); errText == "" {
		t.Fatal("a token without analytics:read ran analytics_query")
	}
	if _, err := ocs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "yuva://property/" + id}); err == nil {
		t.Fatal("a token without analytics:read read the property")
	}

	manager := ct.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "manage", "scopes": []string{"analytics:read", "analytics:manage"}}).str("secret")
	mcs := h.mcpSession(manager)
	goal, errText := callTool(t, mcs, "create_goal", map[string]any{"property_id": id, "name": "Billing read", "kind": "pageview", "path_pattern": "/docs/billing"})
	if errText != "" || goal["name"] != "Billing read" {
		t.Fatalf("create_goal %v %s", goal, errText)
	}
	if _, errText := callTool(t, mcs, "add_annotation", map[string]any{"property_id": id, "starts_on": "2026-10-12", "text": "Docs relaunch"}); errText != "" {
		t.Fatal(errText)
	}

	stranger := newTeam(t, h)
	skey := stranger.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "x", "scopes": []string{"analytics:read"}}).str("secret")
	scs := h.mcpSession(skey)
	if _, errText := callTool(t, scs, "analytics_support_report", map[string]any{"property_id": id}); !strings.Contains(errText, "not_found") && !strings.Contains(errText, "no such") {
		t.Fatalf("another workspace's token: %q", errText)
	}
}

func memberEmail(c *client) string {
	return c.expect(http.StatusOK, "GET", "/v1/me", nil).body["person"].(map[string]any)["email"].(string)
}

func TestAnalyticsDropAlertsOnceAndAnnotates(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	ctx := context.Background()
	day := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	if err := (&analytics.PGStore{Pool: h.st.Pool}).EnsurePartitions(ctx, day.AddDate(0, 0, -8), day.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	h.clock.Set(day.Add(-8 * 24 * time.Hour))
	id, key := createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"example.com"}, "timezone": "UTC"})
	for d := 7; d >= 1; d-- {
		for _, hour := range []int{10, 11} {
			h.clock.Set(day.AddDate(0, 0, -d).Add(time.Duration(hour)*time.Hour + 20*time.Minute))
			viewsFrom(h, key, "https://example.com", "/", 15, d*100+hour*1000)
		}
	}
	h.clock.Set(day.Add(10*time.Hour + 30*time.Minute))
	viewsFrom(h, key, "https://example.com", "/", 2, 9000)
	h.flushAnalytics()

	path := "/v1/analytics/properties/" + id + "/subscription"
	if s := tm.owner.expect(http.StatusOK, "GET", path, nil).body; s["report"] != "none" || len(s["alerts"].([]any)) != 0 {
		t.Fatalf("default subscription %v", s)
	}
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "PUT", path, map[string]any{"alerts": []string{"meteor"}})
	if s := tm.owner.expect(http.StatusOK, "PUT", path, map[string]any{"alerts": []string{"drop", "spike", "drop"}}).body; len(s["alerts"].([]any)) != 2 {
		t.Fatalf("subscription %v", s)
	}
	tm.agent.expectProblem(http.StatusNotFound, "not_found", "PUT", path, map[string]any{"alerts": []string{"drop"}})
	other.owner.expectProblem(http.StatusNotFound, "not_found", "PUT", path, map[string]any{"alerts": []string{"drop"}})
	key2 := h.client()
	key2.bearer = tm.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "k", "scopes": []string{"analytics:read"}}).str("secret")
	key2.expectProblem(http.StatusForbidden, "member_session_required", "PUT", path, map[string]any{"alerts": []string{"drop"}})

	owner := memberEmail(tm.owner)
	before := len(h.mail.to(owner))
	h.clock.Set(day.Add(12*time.Hour + 5*time.Minute))
	n, err := h.srv.CheckAnalyticsAlerts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	h.clock.Set(day.Add(12*time.Hour + 50*time.Minute))
	again, err := h.srv.CheckAnalyticsAlerts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mails := h.mail.to(owner)[before:]
	if n != 1 || again != 0 || len(mails) != 1 || !strings.Contains(mails[0].Subject, "fewer page views than usual") ||
		!strings.Contains(mails[0].Text, "2 page views on 2026-10-10 between 10:00 and 12:00, against 30") {
		t.Fatalf("fired %d then %d, mails %+v", n, again, mails)
	}
	if strings.Contains(mails[0].HTML, "<img") {
		t.Fatal("an alert mail carries an image")
	}
	alerts := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/alerts", nil).body["items"].([]any)
	if len(alerts) != 1 {
		t.Fatalf("alerts %v", alerts)
	}
	a := alerts[0].(map[string]any)
	if a["kind"] != "drop" || a["current"] != 2.0 || a["baseline"] != 30.0 || a["annotation_id"] == nil || a["day"] != "2026-10-10" {
		t.Fatalf("alert %v", a)
	}
	notes := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/annotations?from=2026-10-10&to=2026-10-10", nil).body["items"].([]any)
	if len(notes) != 1 || notes[0].(map[string]any)["kind"] != "alert" || notes[0].(map[string]any)["alert_id"] != a["id"] {
		t.Fatalf("annotations %v", notes)
	}
	tm.owner.expectProblem(http.StatusForbidden, "annotation_read_only", "PATCH", "/v1/analytics/properties/"+id+"/annotations/"+a["annotation_id"].(string),
		map[string]any{"text": "x"})
	events := tm.owner.expect(http.StatusOK, "GET", "/v1/events?after=0&limit=500", nil).body["events"].([]any)
	found := 0
	for _, e := range events {
		m := e.(map[string]any)
		if m["type"] == "analytics.alert" && m["data"].(map[string]any)["alert"].(map[string]any)["id"] == a["id"] {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("%d analytics.alert events", found)
	}
	for _, e := range other.owner.expect(http.StatusOK, "GET", "/v1/events?after=0&limit=500", nil).body["events"].([]any) {
		if e.(map[string]any)["type"] == "analytics.alert" {
			t.Fatal("another workspace sees the alert")
		}
	}
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/analytics/properties/"+id+"/alerts", nil)
}

func TestAnalyticsWeeklyReportInMemberLanguage(t *testing.T) {
	h := newHarness(t)
	h.clock.Set(time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC))
	ct := newChatTeam(t, h, "live", true)
	host := strings.TrimPrefix(ct.origin, "https://")
	id, key := createProperty(ct.owner, map[string]any{"name": "Docs", "domains": []string{host}, "timezone": "Europe/Istanbul", "inbox_id": ct.chatInbox})
	viewsFrom(h, key, ct.origin, "/docs/install", 12, 0)
	h.flushAnalytics()
	cs := ct.session(h, map[string]any{})
	cs.expect(http.StatusCreated, "POST", "/client/v1/conversations", map[string]any{"body": "help", "page_url": ct.origin + "/docs/install", "client_id": unique("c")})

	ct.agent.expect(http.StatusOK, "PATCH", "/v1/me", map[string]any{"locale": "tr"})
	path := "/v1/analytics/properties/" + id + "/subscription"
	ct.agent.expect(http.StatusOK, "PUT", path, map[string]any{"report": "weekly"})
	ct.agent.expectProblem(http.StatusBadRequest, "validation_failed", "PUT", path, map[string]any{"report": "daily"})
	agent := memberEmail(ct.agent)
	before := len(h.mail.to(agent))

	ctx := context.Background()
	h.clock.Set(time.Date(2026, 10, 12, 4, 30, 0, 0, time.UTC))
	if n, err := h.srv.SendDueAnalyticsReports(ctx); err != nil || n != 0 {
		t.Fatalf("before Monday 08:00 in the property's zone: %d %v", n, err)
	}
	h.clock.Set(time.Date(2026, 10, 12, 5, 30, 0, 0, time.UTC))
	n, err := h.srv.SendDueAnalyticsReports(ctx)
	if err != nil || n != 1 {
		t.Fatalf("sent %d: %v", n, err)
	}
	if n, _ := h.srv.SendDueAnalyticsReports(ctx); n != 0 {
		t.Fatalf("the week went out %d more times", n)
	}
	mails := h.mail.to(agent)[before:]
	if len(mails) != 1 {
		t.Fatalf("%d reports", len(mails))
	}
	m := mails[0]
	for _, want := range []string{"Docs, 2026-10-05 – 2026-10-11", "Ziyaretçi: 12", "Sayfa görüntüleme: 12", "/docs/install: 12", "Konuşma: 1",
		"En çok konuşma: /docs/install, 1 (1.000 sayfa görüntülemede 83,3)"} {
		if !strings.Contains(m.Text, want) {
			t.Fatalf("report lacks %q:\n%s", want, m.Text)
		}
	}
	if m.Subject != "Docs: haftalık analitik raporunuz" || strings.Contains(m.HTML, "<img") || strings.Contains(m.HTML, "/c/") {
		t.Fatalf("subject %q, or the report tracks", m.Subject)
	}
	sub := ct.agent.expect(http.StatusOK, "GET", path, nil).body
	if sub["report"] != "weekly" || sub["last_report_at"] == nil {
		t.Fatalf("subscription %v", sub)
	}

	ct.agent.expect(http.StatusAccepted, "POST", path+"/send-report", nil)
	if got := h.mail.to(agent)[before:]; len(got) != 2 || !strings.HasPrefix(got[1].Subject, "Docs: haftalık") {
		t.Fatalf("send-report mails %+v", got)
	}
	for range 4 {
		ct.agent.expect(http.StatusAccepted, "POST", path+"/send-report", nil)
	}
	ct.agent.expectProblem(http.StatusTooManyRequests, "rate_limited", "POST", path+"/send-report", nil)
	newTeam(t, h).owner.expectProblem(http.StatusNotFound, "not_found", "POST", path+"/send-report", nil)
}

func TestBroadcastUTMCampaignVisits(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	h.clock.Set(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	bt := newBroadcastTeam(t, h, bs)
	bt.subscribe(t, "ada@example.org")
	id := bt.draft(t, nil)
	b := bt.get(t, id)
	if b["utm"] != true || b["utm_campaign"] != "october-news" {
		t.Fatalf("broadcast utm %v %v", b["utm"], b["utm_campaign"])
	}
	bt.owner.expectProblem(http.StatusBadRequest, "validation_failed", "PATCH", "/v1/broadcasts/"+id, map[string]any{"utm_campaign": "Oct News!"})
	bt.owner.expect(http.StatusOK, "PATCH", "/v1/broadcasts/"+id, map[string]any{"utm_campaign": "oct-2026"})
	off := bt.draft(t, map[string]any{"utm": false, "name": "Plain"})
	if bt.get(t, off)["utm"] != false {
		t.Fatal("utm off was not kept")
	}

	bt.schedule(t, id)
	bt.run(t)
	sent := bs.to("ada@example.org")
	if len(sent) != 1 {
		t.Fatalf("%d mails", len(sent))
	}
	var raw bytes.Buffer
	for _, m := range bs.sent {
		if m.to[0] == "ada@example.org" {
			raw.Write(m.raw)
		}
	}
	env, err := enmime.ReadEnvelope(&raw)
	if err != nil {
		t.Fatal(err)
	}
	link := "https://acme.example/news?utm_source=newsletter&amp;utm_medium=email&amp;utm_campaign=oct-2026"
	if !strings.Contains(env.HTML, link) || !strings.Contains(env.HTML, "https://acme.example/open?utm_source=newsletter") ||
		!strings.Contains(env.Text, "https://acme.example/news?utm_source=newsletter&utm_medium=email&utm_campaign=oct-2026") {
		t.Fatalf("links without the campaign:\n%s\n%s", env.HTML, env.Text)
	}
	for _, u := range []string{"/m/", "/u/"} {
		if strings.Contains(env.HTML, u) && strings.Contains(env.HTML, u+"?utm_") {
			t.Fatal("the unsubscribe link got a campaign")
		}
	}

	pid, key := createProperty(bt.owner, map[string]any{"name": "Acme", "domains": []string{"acme.example"}, "timezone": "UTC"})
	for i, page := range []string{"/news?utm_source=newsletter&utm_medium=email&utm_campaign=oct-2026", "/news?utm_source=newsletter&utm_medium=email&utm_campaign=oct-2026", "/news?utm_campaign=other", "/"} {
		r := collect(h, key, collectReq{origin: "https://acme.example", ip: fmt.Sprintf("81.200.%d.10", i), header: map[string]string{"User-Agent": uaMac},
			body: map[string]any{"n": "pageview", "u": "https://acme.example" + page}})
		if r.status != http.StatusAccepted {
			t.Fatalf("collect %d", r.status)
		}
	}
	h.flushAnalytics()
	report := bt.owner.expect(http.StatusOK, "GET", "/v1/broadcasts/"+id+"/report", nil).body
	visits, ok := report["campaign_visits"].([]any)
	if !ok || len(visits) != 1 {
		t.Fatalf("campaign visits %v", report["campaign_visits"])
	}
	if v := visits[0].(map[string]any); v["property_id"] != pid || v["visits"] != 2.0 || v["visitors"] != 2.0 || v["pageviews"] != 2.0 {
		t.Fatalf("campaign visits %v", v)
	}
	agentReport := bt.agent.expect(http.StatusOK, "GET", "/v1/broadcasts/"+id+"/report", nil).body
	if v, ok := agentReport["campaign_visits"].([]any); !ok || len(v) != 0 {
		t.Fatalf("an agent sees a property without an inbox: %v", agentReport["campaign_visits"])
	}
	k := h.client()
	k.bearer = bt.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "b", "scopes": []string{"broadcasts:read"}}).str("secret")
	if _, ok := k.expect(http.StatusOK, "GET", "/v1/broadcasts/"+id+"/report", nil).body["campaign_visits"]; ok {
		t.Fatal("a key without analytics:read gets campaign visits")
	}
}

func insightsOf(t *testing.T, r response) []map[string]any {
	t.Helper()
	var out []map[string]any
	seen := map[string]bool{}
	for _, x := range r.body["insights"].([]any) {
		m := x.(map[string]any)
		key := fmt.Sprint(m["kind"], m["metric"], m["dimension"], m["value"])
		if seen[key] {
			t.Fatalf("insight %s twice: %s", key, r.raw)
		}
		seen[key] = true
		out = append(out, m)
	}
	return out
}

func TestAnalyticsInsightsWithAnEmptyComparison(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	ctx := context.Background()
	today := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	if err := (&analytics.PGStore{Pool: h.st.Pool}).EnsurePartitions(ctx, today.AddDate(0, 0, -2), today); err != nil {
		t.Fatal(err)
	}
	empty, emptyKey := createProperty(tm.owner, map[string]any{"name": "New site", "domains": []string{"example.com"}, "timezone": "UTC"})
	grown, grownKey := createProperty(tm.owner, map[string]any{"name": "Grown", "domains": []string{"example.com"}, "timezone": "UTC"})
	small, smallKey := createProperty(tm.owner, map[string]any{"name": "Small", "domains": []string{"example.com"}, "timezone": "UTC"})
	h.clock.Set(today.Add(-12 * time.Hour))
	viewsFrom(h, grownKey, "https://example.com", "/", 15, 0)
	viewsFrom(h, smallKey, "https://example.com", "/", 5, 100)
	h.clock.Set(today.Add(12 * time.Hour))
	viewsFrom(h, emptyKey, "https://example.com", "/", 12, 200)
	viewsFrom(h, grownKey, "https://example.com", "/", 30, 300)
	viewsFrom(h, smallKey, "https://example.com", "/", 40, 400)
	h.flushAnalytics()
	ask := func(id string, dims ...string) []map[string]any {
		q := map[string]any{"metrics": []string{"visitors", "visits", "pageviews", "bounce_rate"}, "range": map[string]any{"period": "today"},
			"compare": map[string]any{"with": "previous"}, "insights": true}
		if len(dims) > 0 {
			q["dimensions"] = dims
		}
		return insightsOf(t, query(tm.owner, id, q))
	}

	for _, dims := range [][]string{nil, {"time:day"}, {"page"}} {
		got := ask(empty, dims...)
		kinds := map[string]bool{}
		for _, in := range got {
			if in["kind"] == "metric_change" {
				t.Fatalf("a zero baseline reported as a change: %v", got)
			}
			if in["kind"] == "metric_new" {
				if in["previous"] != 0.0 || in["current"] != 12.0 || in["change"] != nil {
					t.Fatalf("metric_new %v", in)
				}
				kinds[in["metric"].(string)] = true
			}
		}
		if len(kinds) != 3 || !kinds["visitors"] || !kinds["visits"] || !kinds["pageviews"] {
			t.Fatalf("%v: metric_new for %v in %v", dims, kinds, got)
		}
	}
	page := ask(empty, "page")
	if last := page[len(page)-1]; last["kind"] != "new_value" || last["value"] != "/" || last["previous"] != 0.0 || last["change"] != nil {
		t.Fatalf("new page %v", page)
	}

	got := ask(grown)
	if len(got) != 3 {
		t.Fatalf("grown %v", got)
	}
	for _, in := range got {
		if in["kind"] != "metric_change" || in["previous"] != 15.0 || in["current"] != 30.0 || in["change"] != 100.0 {
			t.Fatalf("change %v", in)
		}
	}
	if got := ask(small); len(got) != 0 {
		t.Fatalf("a baseline under 10 gave %v", got)
	}

	key := tm.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "a", "scopes": []string{"analytics:read"}}).str("secret")
	out, errText := callTool(t, h.mcpSession(key), "analytics_overview", map[string]any{"property_id": empty, "period": "today"})
	if errText != "" {
		t.Fatal(errText)
	}
	seen := map[string]bool{}
	for _, x := range out["insights"].([]any) {
		m := x.(map[string]any)
		k := fmt.Sprint(m["kind"], m["metric"], m["dimension"], m["customer_content"])
		if seen[k] || m["kind"] == "metric_change" {
			t.Fatalf("overview insights %v", out["insights"])
		}
		seen[k] = true
	}
}

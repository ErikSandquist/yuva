package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/api"
)

// send collects one event from a visitor and fails the test unless it is taken.
func send(h *harness, key, ip, ua string, ev map[string]any) {
	h.t.Helper()
	r := collect(h, key, collectReq{origin: "https://example.com", ip: ip, header: map[string]string{"User-Agent": ua}, body: ev})
	if r.status != http.StatusAccepted {
		h.t.Fatalf("collect %v: %d %s", ev, r.status, r.raw)
	}
}

func view(page string) map[string]any {
	return map[string]any{"n": "pageview", "u": "https://example.com" + page}
}

func event(name, page string, props map[string]any, revenue map[string]any) map[string]any {
	ev := map[string]any{"n": name, "u": "https://example.com" + page}
	if props != nil {
		ev["p"] = props
	}
	if revenue != nil {
		ev["$"] = revenue
	}
	return ev
}

func totals(r response) map[string]any { return r.body["totals"].(map[string]any) }

// rowsBy maps a breakdown's first dimension to its metrics.
func rowsBy(r response) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, row := range r.body["rows"].([]any) {
		m := row.(map[string]any)
		out[m["dimensions"].([]any)[0].(string)] = m["metrics"].(map[string]any)
	}
	return out
}

// revenueOf is a metrics object's revenue as "CUR:total/events" per currency, largest first.
func revenueOf(m map[string]any) string {
	var out []string
	list, _ := m["revenue"].([]any)
	for _, x := range list {
		r := x.(map[string]any)
		out = append(out, fmt.Sprintf("%s:%v/%v", r["currency"], r["total"], r["events"]))
	}
	return strings.Join(out, ",")
}

// signupDay is three visitors on example.com: two sign up (one on the pro plan for 29.90 EUR, one
// free with 0 USD and an e-mail-shaped prop), one only looks at the pricing page.
func signupDay(h *harness, key string) {
	h.t.Helper()
	send(h, key, "203.0.113.10", uaMac, view("/pricing"))
	send(h, key, "203.0.113.10", uaMac, event("Signup", "/signup", map[string]any{"plan": "pro", "seats": 3}, map[string]any{"a": "29.90", "c": "eur"}))
	send(h, key, "198.51.100.20", uaPixel, view("/pricing"))
	send(h, key, "198.51.100.20", uaPixel, event("Signup", "/signup", map[string]any{"plan": "free", "email": "ayse@example.com"}, map[string]any{"a": 5, "c": "USD"}))
	send(h, key, "2001:db8:1::7", uaWindows, view("/pricing"))
	h.flushAnalytics()
}

func TestAnalyticsSignupEventGoalAndProps(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	id, key := createProperty(tm.owner, map[string]any{"name": "Shop", "domains": []string{"example.com"}, "tracked_props": []string{"plan"}})
	ctx := context.Background()
	h.clock.Set(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	signupDay(h, key)
	day := "2026-10-12"

	var props string
	if err := h.st.Pool.QueryRow(ctx, `SELECT props::text FROM analytics_events WHERE workspace_id = $1 AND property_id = $2 AND name = 'Signup' AND currency = 'USD'`,
		tm.ws, id).Scan(&props); err != nil {
		t.Fatal(err)
	}
	if props != `{"plan": "free"}` {
		t.Fatalf("stored props %s", props)
	}
	health := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/health?days=1", nil)
	if totals(health)["scrubbed"].(float64) != 1 {
		t.Fatalf("the dropped e-mail prop is not counted: %s", health.raw)
	}

	goal := tm.owner.expect(http.StatusCreated, "POST", "/v1/analytics/properties/"+id+"/goals", map[string]any{"name": "Signed up", "kind": "event", "event_name": "Signup"})
	pro := tm.owner.expect(http.StatusCreated, "POST", "/v1/analytics/properties/"+id+"/goals",
		map[string]any{"name": "Pro signup", "kind": "event", "event_name": "Signup", "prop_key": "plan", "prop_value": "pro"})
	tm.owner.expectProblem(http.StatusConflict, "goal_taken", "POST", "/v1/analytics/properties/"+id+"/goals", map[string]any{"name": "Signed up", "kind": "pageview", "path_pattern": "/x"})

	answers := func() []string {
		t.Helper()
		events := query(tm.owner, id, map[string]any{"metrics": []string{"visitors", "events", "revenue"}, "range": dayRange(day), "dimensions": []string{"event"}})
		signup := rowsBy(events)["Signup"]
		if signup["visitors"].(float64) != 2 || signup["events"].(float64) != 2 || revenueOf(signup) != "EUR:2990/1,USD:500/1" {
			t.Fatalf("event breakdown %s", events.raw)
		}
		conv := query(tm.owner, id, map[string]any{"metrics": []string{"visitors", "conversions", "conversion_rate", "completions", "revenue"},
			"range": dayRange(day), "goal_id": goal.str("id")})
		m := totals(conv)
		if m["visitors"].(float64) != 3 || m["conversions"].(float64) != 2 || m["conversion_rate"].(float64) != 66.67 || m["completions"].(float64) != 2 ||
			revenueOf(m) != "EUR:2990/1,USD:500/1" {
			t.Fatalf("goal %s", conv.raw)
		}
		byPlan := query(tm.owner, id, map[string]any{"metrics": []string{"visitors", "events", "revenue"}, "range": dayRange(day), "dimensions": []string{"prop"},
			"prop": "plan", "filters": []map[string]any{{"dimension": "event", "op": "is", "values": []string{"Signup"}}}})
		plans := rowsBy(byPlan)
		if plans["pro"]["events"].(float64) != 1 || revenueOf(plans["pro"]) != "EUR:2990/1" || plans["free"]["visitors"].(float64) != 1 || len(plans) != 2 {
			t.Fatalf("prop breakdown %s", byPlan.raw)
		}
		proConv := query(tm.owner, id, map[string]any{"metrics": []string{"conversions", "revenue"}, "range": dayRange(day), "goal_id": pro.str("id"),
			"dimensions": []string{"channel"}})
		if totals(proConv)["conversions"].(float64) != 1 || revenueOf(totals(proConv)) != "EUR:2990/1" {
			t.Fatalf("pro goal %s", proConv.raw)
		}
		return []string{numbers(events), numbers(conv), numbers(byPlan), numbers(proConv)}
	}
	raw := answers()

	h.clock.Set(time.Date(2026, 10, 13, 3, 0, 0, 0, time.UTC))
	if err := h.srv.RollupAnalyticsDay(ctx, uuid.MustParse(tm.ws), uuid.MustParse(id), time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	rolled := answers()
	for i := range raw {
		if raw[i] != rolled[i] {
			t.Fatalf("rollups differ from raw rows:\nraw    %s\nrollup %s", raw[i], rolled[i])
		}
	}
	if r := query(tm.owner, id, map[string]any{"metrics": []string{"conversions"}, "range": dayRange(day), "goal_id": goal.str("id")}); served(r) != "rollups" {
		t.Fatalf("a rolled-up goal is served from %s", served(r))
	}
	if n := h.count(`SELECT count(*) FROM analytics_daily_props WHERE workspace_id = $1 AND property_id = $2 AND name = 'Signup' AND prop = 'plan'
		AND value IN ('pro', 'free') AND currency = ''`, tm.ws, id); n != 2 {
		t.Fatalf("%d rollup rows of the tracked prop", n)
	}

	for _, path := range []string{"/goals", "/goals/" + goal.str("id"), "/funnels", "/segments", "/annotations"} {
		method := "GET"
		if strings.HasPrefix(path, "/goals/") {
			method = "DELETE"
		}
		other.owner.expectProblem(http.StatusNotFound, "not_found", method, "/v1/analytics/properties/"+id+path, nil)
	}
	otherID, _ := createProperty(other.owner, map[string]any{"name": "Other", "domains": []string{"example.com"}})
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/analytics/properties/"+otherID+"/query",
		map[string]any{"metrics": []string{"conversions"}, "range": dayRange(day), "goal_id": goal.str("id")})
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties/"+id+"/query",
		map[string]any{"metrics": []string{"conversions"}, "range": dayRange(day)})
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties/"+id+"/query",
		map[string]any{"metrics": []string{"bounce_rate"}, "range": dayRange(day), "dimensions": []string{"event"}})
}

func TestAnalyticsGoalCountsTheRawWindow(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	id, key := createProperty(tm.owner, map[string]any{"name": "Docs", "domains": []string{"example.com"}})
	ctx := context.Background()
	ws, prop := uuid.MustParse(tm.ws), uuid.MustParse(id)
	for d := 8; d <= 10; d++ {
		h.clock.Set(time.Date(2026, 10, d, 12, 0, 0, 0, time.UTC))
		send(h, key, fmt.Sprintf("203.0.113.%d", d), uaMac, view("/docs/start"))
		send(h, key, fmt.Sprintf("198.51.100.%d", d), uaPixel, view("/pricing"))
		h.flushAnalytics()
	}
	h.clock.Set(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	for d := 8; d <= 10; d++ {
		if err := h.srv.RollupAnalyticsDay(ctx, ws, prop, time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
	}
	goal := tm.owner.expect(http.StatusCreated, "POST", "/v1/analytics/properties/"+id+"/goals", map[string]any{"name": "Read the docs", "kind": "pageview", "path_pattern": "/docs/**"})
	if goal.str("counts_from") > "2026-10-08" {
		t.Fatalf("counts_from %s", goal.str("counts_from"))
	}
	q := map[string]any{"metrics": []string{"visitors", "conversions", "conversion_rate"}, "range": map[string]any{"from": "2026-10-08", "to": "2026-10-10"},
		"goal_id": goal.str("id"), "dimensions": []string{"time:day"}}
	r := query(tm.owner, id, q)
	if m := totals(r); m["conversions"].(float64) != 3 || m["conversion_rate"].(float64) != 50 || served(r) != "mixed" {
		t.Fatalf("a goal made today does not count the days before it: %s", r.raw)
	}
	before := numbers(r)
	if _, err := h.srv.ScanAnalyticsRollups(ctx); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM river_job WHERE kind = 'analytics_goal_rollup' AND args->>'property_id' = $1", id); n != 3 {
		t.Fatalf("%d goal rollups queued", n)
	}
	for d := 8; d <= 10; d++ {
		if err := h.srv.RollupAnalyticsGoals(ctx, ws, prop, time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
	}
	r = query(tm.owner, id, q)
	if served(r) != "rollups" || numbers(r) != before {
		t.Fatalf("goal rollups: %s\nwant %s", r.raw, before)
	}
	notes := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/annotations", nil).body["items"].([]any)
	if len(notes) != 1 || notes[0].(map[string]any)["kind"] != "goal_created" || notes[0].(map[string]any)["text"] != "Read the docs" ||
		notes[0].(map[string]any)["starts_on"] != "2026-10-12" {
		t.Fatalf("annotations %v", notes)
	}

	tm.owner.expect(http.StatusOK, "PATCH", "/v1/analytics/properties/"+id+"/goals/"+goal.str("id"), map[string]any{"path_pattern": "/pricing"})
	if r := query(tm.owner, id, q); served(r) != "mixed" || totals(r)["conversions"].(float64) != 3 {
		t.Fatalf("a changed goal is not counted anew: %s", r.raw)
	}
}

func TestAnalyticsFunnelFollowsVisits(t *testing.T) {
	h := newHarnessWith(t, func(d *api.Deps) { d.Analytics.RawDays = 120 })
	tm := newTeam(t, h)
	other := newTeam(t, h)
	id, key := createProperty(tm.owner, map[string]any{"name": "Funnel", "domains": []string{"example.com"}})
	h.clock.Set(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	type visitor struct {
		ip    string
		steps []map[string]any
	}
	land, price, signup := view("/"), view("/pricing"), event("Signup", "/signup", nil, nil)
	for i, v := range []visitor{
		{"203.0.113.1", []map[string]any{land, price, signup}},
		{"203.0.113.2", []map[string]any{land, price, signup}},
		{"203.0.113.3", []map[string]any{land, price}},
		{"203.0.113.4", []map[string]any{land, price}},
		{"203.0.113.5", []map[string]any{land}},
		{"203.0.113.6", []map[string]any{land, signup, price}},
		{"203.0.113.7", []map[string]any{price, signup}},
	} {
		for _, s := range v.steps {
			h.clock.Advance(time.Minute)
			send(h, key, v.ip, fmt.Sprintf("%s visitor/%d", uaMac, i), s)
		}
	}
	h.flushAnalytics()
	goal := func(body map[string]any) string {
		return tm.owner.expect(http.StatusCreated, "POST", "/v1/analytics/properties/"+id+"/goals", body).str("id")
	}
	g1 := goal(map[string]any{"name": "Landed", "kind": "pageview", "path_pattern": "/"})
	g2 := goal(map[string]any{"name": "Pricing", "kind": "pageview", "path_pattern": "/pricing"})
	g3 := goal(map[string]any{"name": "Signup", "kind": "event", "event_name": "Signup"})
	f := tm.owner.expect(http.StatusCreated, "POST", "/v1/analytics/properties/"+id+"/funnels", map[string]any{"name": "Sign-up", "steps": []string{g1, g2, g3}})
	report := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/funnels/"+f.str("id")+"/report?period=today", nil)
	var got []string
	for _, s := range report.body["steps"].([]any) {
		m := s.(map[string]any)
		got = append(got, fmt.Sprintf("%s:%v/%v/%v", m["name"], m["visitors"], m["dropoff"], m["conversion_rate"]))
	}
	if strings.Join(got, " ") != "Landed:6/0/100 Pricing:5/1/83.33 Signup:2/3/33.33" {
		t.Fatalf("funnel %s", report.raw)
	}

	tm.owner.expectProblem(http.StatusConflict, "goal_in_use", "DELETE", "/v1/analytics/properties/"+id+"/goals/"+g2, nil)
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties/"+id+"/funnels", map[string]any{"name": "Short", "steps": []string{g1}})
	tm.owner.expectProblem(http.StatusUnprocessableEntity, "range_too_large", "GET", "/v1/analytics/properties/"+id+"/funnels/"+f.str("id")+"/report?from=2026-07-01&to=2026-10-12", nil)
	res := tm.owner.do("GET", "/v1/analytics/properties/"+id+"/funnels/"+f.str("id")+"/report?from=2026-01-01&to=2026-01-31", nil)
	if res.status != http.StatusUnprocessableEntity || res.str("code") != "needs_raw_window" || res.str("raw_window_from") == "" {
		t.Fatalf("needs raw window %d %s", res.status, res.raw)
	}
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/analytics/properties/"+id+"/funnels/"+f.str("id")+"/report", nil)
	tm.agent.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/analytics/properties/"+id+"/funnels/"+f.str("id")+"/report", nil)
	reader := h.client()
	reader.bearer = tm.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "reader", "scopes": []string{"analytics:read"}}).str("secret")
	reader.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/funnels/"+f.str("id")+"/report", nil)
	reader.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", "/v1/analytics/properties/"+id+"/goals", map[string]any{"name": "X", "kind": "pageview", "path_pattern": "/x"})
}

func TestAnalyticsBuiltInEvents(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	id, key := createProperty(tm.owner, map[string]any{"name": "Links", "domains": []string{"example.com"}})
	h.clock.Set(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	refused := func(ev map[string]any) {
		t.Helper()
		r := collect(h, key, collectReq{origin: "https://example.com", ip: "203.0.113.9", body: ev})
		if r.status != http.StatusBadRequest {
			t.Fatalf("%v was taken: %d %s", ev, r.status, r.raw)
		}
	}
	refused(event("outbound_click", "/", nil, nil))
	refused(event("outbound_click", "/", map[string]any{"url": "https://example.com/elsewhere"}, nil))
	refused(event("file_download", "/", map[string]any{"url": "javascript:alert(1)"}, nil))
	refused(event("Signup", "/", map[string]any{"yuva_plan": "pro"}, nil))
	refused(map[string]any{"n": "pageview", "u": "https://example.com/", "$": map[string]any{"a": "1", "c": "EUR"}})
	refused(event("Buy", "/", nil, map[string]any{"a": "1.5", "c": "JPY"}))

	send(h, key, "203.0.113.1", uaMac, view("/"))
	send(h, key, "203.0.113.1", uaMac, event("outbound_click", "/", map[string]any{"url": "https://github.com/productdevbook/yuva?tab=readme#top"}, nil))
	send(h, key, "203.0.113.2", uaPixel, view("/docs"))
	send(h, key, "203.0.113.2", uaPixel, event("file_download", "/docs", map[string]any{"url": "https://example.com/guide.pdf?token=1"}, nil))
	send(h, key, "203.0.113.3", uaWindows, view("/"))
	send(h, key, "203.0.113.3", uaWindows, event("chat_open", "/", nil, nil))
	send(h, key, "198.51.100.4", uaMac, view("/missing"))
	send(h, key, "198.51.100.4", uaMac, event("not_found", "/missing", nil, nil))
	h.flushAnalytics()

	r := query(tm.owner, id, map[string]any{"metrics": []string{"visitors", "events"}, "range": map[string]any{"period": "today"}, "dimensions": []string{"event"}})
	byName := rowsBy(r)
	for _, n := range []string{"outbound_click", "file_download", "chat_open", "not_found"} {
		if byName[n]["events"].(float64) != 1 {
			t.Fatalf("%s: %s", n, r.raw)
		}
	}
	urls := query(tm.owner, id, map[string]any{"metrics": []string{"events"}, "range": map[string]any{"period": "today"}, "dimensions": []string{"prop"}, "prop": "url"})
	got := rowsBy(urls)
	if got["https://github.com/productdevbook/yuva"] == nil || got["https://example.com/guide.pdf"] == nil {
		t.Fatalf("link urls %s", urls.raw)
	}
	bounce := query(tm.owner, id, map[string]any{"metrics": []string{"bounce_rate"}, "range": map[string]any{"period": "today"},
		"filters": []map[string]any{{"dimension": "event", "op": "is", "values": []string{"not_found"}}}})
	if totals(bounce)["bounce_rate"].(float64) != 100 {
		t.Fatalf("a 404 visit is not a bounce: %s", bounce.raw)
	}
	chat := query(tm.owner, id, map[string]any{"metrics": []string{"visitors", "bounce_rate"}, "range": map[string]any{"period": "today"},
		"filters": []map[string]any{{"dimension": "event", "op": "is", "values": []string{"chat_open"}}}})
	if m := totals(chat); m["visitors"].(float64) != 1 || m["bounce_rate"].(float64) != 0 {
		t.Fatalf("opening the chat: %s", chat.raw)
	}
}

func TestAnalyticsSegmentsAndAnnotations(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	tm.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+tm.inbox+"/members/"+tm.agentID, nil)
	id, key := createProperty(tm.owner, map[string]any{"name": "Linked", "domains": []string{"example.com"}, "inbox_id": tm.inbox})
	h.clock.Set(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	signupDay(h, key)

	seg := tm.agent.expect(http.StatusCreated, "POST", "/v1/analytics/properties/"+id+"/segments", map[string]any{"name": "Pro buyers",
		"filters": []map[string]any{{"dimension": "prop", "prop": "plan", "op": "is", "values": []string{"pro"}}}})
	tm.agent.expectProblem(http.StatusConflict, "segment_taken", "POST", "/v1/analytics/properties/"+id+"/segments", map[string]any{"name": "Pro buyers",
		"filters": []map[string]any{{"dimension": "country", "op": "is", "values": []string{"TR"}}}})
	tm.agent.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties/"+id+"/segments", map[string]any{"name": "No key",
		"filters": []map[string]any{{"dimension": "prop", "op": "is", "values": []string{"pro"}}}})
	list := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/segments", nil).body["items"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["created_by"] != tm.agentID {
		t.Fatalf("segments %v", list)
	}
	r := query(tm.owner, id, map[string]any{"metrics": []string{"visitors", "pageviews"}, "range": map[string]any{"period": "today"}, "segment_id": seg.str("id")})
	if m := totals(r); m["visitors"].(float64) != 1 || m["pageviews"].(float64) != 1 {
		t.Fatalf("the agent's segment for the owner: %s", r.raw)
	}
	ownerSeg := tm.owner.expect(http.StatusCreated, "POST", "/v1/analytics/properties/"+id+"/segments", map[string]any{"name": "Phones",
		"filters": []map[string]any{{"dimension": "device", "op": "is", "values": []string{"mobile"}}}})
	tm.agent.expectProblem(http.StatusForbidden, "forbidden", "PATCH", "/v1/analytics/properties/"+id+"/segments/"+ownerSeg.str("id"), map[string]any{"name": "Mine"})
	tm.owner.expect(http.StatusOK, "PATCH", "/v1/analytics/properties/"+id+"/segments/"+seg.str("id"), map[string]any{"name": "Pro plan"})
	tm.agent.expect(http.StatusNoContent, "DELETE", "/v1/analytics/properties/"+id+"/segments/"+seg.str("id"), nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/analytics/properties/"+id+"/query",
		map[string]any{"metrics": []string{"visitors"}, "range": map[string]any{"period": "today"}, "segment_id": ownerSeg.str("id")})

	note := tm.agent.expect(http.StatusCreated, "POST", "/v1/analytics/properties/"+id+"/annotations", map[string]any{"starts_on": "2026-10-10", "ends_on": "2026-10-11", "text": "Launch week"})
	tm.agent.expect(http.StatusOK, "PATCH", "/v1/analytics/properties/"+id+"/annotations/"+note.str("id"), map[string]any{"text": "Launch"})
	tm.agent.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties/"+id+"/annotations", map[string]any{"starts_on": "2026-10-10", "ends_on": "2026-10-01", "text": "Backwards"})
	tm.owner.expect(http.StatusOK, "POST", "/v1/analytics/properties/"+id+"/rotate-key", nil)
	items := tm.agent.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/annotations?from=2026-10-11&to=2026-10-12", nil).body["items"].([]any)
	var kinds []string
	var rotated string
	for _, it := range items {
		m := it.(map[string]any)
		kinds = append(kinds, m["kind"].(string)+":"+m["text"].(string))
		if m["kind"] == "key_rotated" {
			rotated = m["id"].(string)
		}
	}
	if strings.Join(kinds, ",") != "note:Launch,key_rotated:" {
		t.Fatalf("annotations %v", kinds)
	}
	tm.owner.expectProblem(http.StatusForbidden, "annotation_read_only", "PATCH", "/v1/analytics/properties/"+id+"/annotations/"+rotated, map[string]any{"text": "x"})
	tm.agent.expectProblem(http.StatusForbidden, "forbidden", "DELETE", "/v1/analytics/properties/"+id+"/annotations/"+rotated, nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "DELETE", "/v1/analytics/properties/"+id+"/annotations/"+note.str("id"), nil)
	tm.owner.expect(http.StatusNoContent, "DELETE", "/v1/analytics/properties/"+id+"/annotations/"+rotated, nil)

	raw, _ := json.Marshal(tm.agent.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/goals", nil).body)
	if string(raw) != `{"items":[]}` {
		t.Fatalf("goals %s", raw)
	}
	tm.agent.expectProblem(http.StatusForbidden, "forbidden", "POST", "/v1/analytics/properties/"+id+"/goals", map[string]any{"name": "X", "kind": "pageview", "path_pattern": "/x"})
}

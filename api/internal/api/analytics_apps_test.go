package api_test

import (
	"context"
	"net/http"
	"testing"
	"time"
	"uuid"
)

func serverEvents(c *client, want int, id string, events ...map[string]any) response {
	c.h.t.Helper()
	return c.expect(want, "POST", "/v1/analytics/properties/"+id+"/events", map[string]any{"events": events})
}

func appBatch(key, platform string, events ...map[string]any) map[string]any {
	return map[string]any{"k": key, "a": map[string]any{"platform": platform, "version": "2.3.1", "os_version": "18.1", "device": "mobile", "width": 390, "language": "tr-TR"},
		"e": events}
}

func screen(name string, at time.Time) map[string]any {
	return map[string]any{"n": "screen", "u": "app://com.example.shop/" + name, "t": at.UnixMilli()}
}

func TestAnalyticsServerEventJoinsBrowserVisit(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	id, key := createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"example.com"}})
	h.clock.Set(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	send(h, key, "203.0.113.10", testUA, view("/pricing"))
	h.clock.Advance(time.Minute)

	backend := tm.apiKey(h)
	r := serverEvents(backend, http.StatusAccepted, id,
		map[string]any{"name": "Signup", "url": "https://example.com/signup", "ip": "203.0.113.99", "user_agent": testUA,
			"props": map[string]any{"plan": "pro"}, "revenue": map[string]any{"a": "29.90", "c": "EUR"}},
		map[string]any{"name": "Signup", "url": "https://example.com/signup", "visitor_key": "session-42", "ip": "198.51.100.7", "accept_language": "de-DE"},
		map[string]any{"name": "Signup", "url": "https://example.com/signup", "ip": "192.0.2.1", "user_agent": testUA, "gpc": true},
		map[string]any{"name": "Signup", "url": "https://other.example.net/signup", "visitor_key": "x"},
		map[string]any{"name": "Signup", "url": "https://example.com/signup"},
		map[string]any{"name": "engagement", "url": "https://example.com/signup", "visitor_key": "x"},
	)
	errs := r.body["errors"].([]any)
	if r.body["accepted"].(float64) != 3 || len(errs) != 3 || errs[0].(map[string]any)["index"].(float64) != 3 || errs[1].(map[string]any)["index"].(float64) != 4 {
		t.Fatalf("server events answer %s", r.raw)
	}
	h.flushAnalytics()

	if n := h.count("SELECT count(*) FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 2 {
		t.Fatalf("%d visits, want the browser's visit and the visitor key's", n)
	}
	if n := h.count(`SELECT count(*) FROM analytics_events e JOIN analytics_visits v ON v.property_id = e.property_id AND v.day = e.day AND v.visit = e.visit
		WHERE e.workspace_id = $1 AND e.property_id = $2 AND v.platform = 1 AND e.revenue_minor = 2990`, tm.ws, id); n != 1 {
		t.Fatal("the server event with the forwarded address and user agent did not join the browser's visit")
	}
	if n := h.count("SELECT count(*) FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2 AND platform = 4 AND language = 'de'", tm.ws, id); n != 1 {
		t.Fatal("the visitor key's event did not start a server visit")
	}
	byPlatform := rowsBy(query(tm.owner, id, map[string]any{"metrics": []string{"visitors", "events"}, "range": dayRange("2026-10-12"), "dimensions": []string{"platform"}}))
	if byPlatform["web"]["visitors"].(float64) != 1 || byPlatform["web"]["events"].(float64) != 1 || byPlatform["server"]["visitors"].(float64) != 1 {
		t.Fatalf("platforms %v", byPlatform)
	}
}

func TestAnalyticsServerEventsNeedAKeyWithWrite(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	id, _ := createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"example.com"}})
	ev := map[string]any{"name": "Signup", "url": "https://example.com/signup", "visitor_key": "k"}
	body := map[string]any{"events": []any{ev}}
	path := "/v1/analytics/properties/" + id + "/events"

	reader := h.client()
	reader.bearer = tm.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "reader", "scopes": []string{"analytics:read"}}).str("secret")
	reader.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", path, body)
	tm.owner.expectProblem(http.StatusForbidden, "forbidden", "POST", path, body)
	other.apiKey(h).expectProblem(http.StatusNotFound, "not_found", "POST", path, body)
	tm.apiKey(h).expectProblem(http.StatusBadRequest, "validation_failed", "POST", path, map[string]any{"events": []any{}})
	serverEvents(tm.apiKey(h), http.StatusAccepted, id, ev)
}

func TestAnalyticsAppLateEventsAndRelaunch(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	id, key := createProperty(tm.owner, map[string]any{"name": "Shop", "domains": []string{"example.com"}, "platforms": []string{"web", "ios"}})
	webOnly, webKey := createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"example.com"}})
	ctx := context.Background()
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	h.clock.Set(now)
	offline := now.Add(-50 * time.Hour)
	first, relaunch := uuid.New().String(), uuid.New().String()

	ok := func(r response) {
		t.Helper()
		if r.status != http.StatusAccepted {
			t.Fatalf("status %d: %s", r.status, r.raw)
		}
	}
	ok(collect(h, key, collectReq{ip: "203.0.113.5", body: appBatch(first, "ios",
		screen("Home", offline.Add(time.Minute)),
		screen("Home", offline),
		map[string]any{"n": "Purchase", "u": "app://com.example.shop/Cart", "t": offline.Add(2 * time.Minute).UnixMilli(), "$": map[string]any{"a": "9.99", "c": "EUR"}},
		screen("Old", now.Add(-73*time.Hour)),
		map[string]any{"n": "screen", "u": "https://example.com/not-a-screen"},
	)}))
	h.clock.Advance(time.Minute)
	ok(collect(h, key, collectReq{ip: "203.0.113.5", body: appBatch(relaunch, "ios", screen("Home", now.Add(time.Minute)))}))
	h.flushAnalytics()

	rows, err := h.st.Pool.Query(ctx, `SELECT day::text, ts, path FROM analytics_events WHERE workspace_id = $1 AND property_id = $2 ORDER BY ts`, tm.ws, id)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var day, path string
		var ts time.Time
		if err := rows.Scan(&day, &ts, &path); err != nil {
			t.Fatal(err)
		}
		got = append(got, day+" "+ts.UTC().Format("15:04")+" "+path)
	}
	want := []string{"2026-10-10 10:00 /Home", "2026-10-10 10:01 /Home", "2026-10-10 10:02 /Cart", "2026-10-12 12:01 /Home"}
	if len(got) != len(want) {
		t.Fatalf("events %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("events %v, want %v", got, want)
		}
	}
	if n := h.count(`SELECT count(*) FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2 AND day = '2026-10-10' AND views = 2 AND events = 1
		AND entry_path = '/Home' AND hostname = 'com.example.shop' AND platform = 2 AND app_version = '2.3.1' AND os = 'iOS' AND os_major = 18 AND language = 'tr'`, tm.ws, id); n != 1 {
		t.Fatal("the offline visit is not described by its app")
	}
	health := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/health?days=1", nil)
	dropped := totals(health)["dropped"].(map[string]any)
	if dropped["late"].(float64) != 1 || dropped["invalid"].(float64) != 1 {
		t.Fatalf("health %s", health.raw)
	}

	h.clock.Advance(time.Minute)
	ok(collect(h, key, collectReq{ip: "203.0.113.5", body: appBatch(first, "ios", screen("Settings", h.clock.Now()))}))
	h.flushAnalytics()
	today := totals(query(tm.owner, id, map[string]any{"metrics": []string{"visitors", "visits", "pageviews"}, "range": dayRange("2026-10-12")}))
	if today["visitors"].(float64) != 2 || today["visits"].(float64) != 2 || today["pageviews"].(float64) != 2 {
		t.Fatalf("a relaunch must be a new visitor and visit: %v", today)
	}
	screens := rowsBy(query(tm.owner, id, map[string]any{"metrics": []string{"visitors", "pageviews"}, "range": map[string]any{"from": "2026-10-10", "to": "2026-10-12"},
		"dimensions": []string{"page"}, "filters": []map[string]any{{"dimension": "platform", "op": "is", "values": []string{"ios"}}}}))
	if screens["/Home"]["pageviews"].(float64) != 3 || screens["/Settings"]["pageviews"].(float64) != 1 {
		t.Fatalf("screens %v", screens)
	}

	r := collect(h, key, collectReq{ip: "203.0.113.5", body: appBatch(first, "android", screen("Home", now))})
	if r.status != http.StatusForbidden || r.body["code"] != "platform_not_allowed" {
		t.Fatalf("android to an ios property: %d %s", r.status, r.raw)
	}
	r = collect(h, webKey, collectReq{ip: "203.0.113.5", body: appBatch(first, "ios", screen("Home", now))})
	if r.status != http.StatusForbidden || r.body["code"] != "origin_not_allowed" {
		t.Fatalf("app to a web property: %d %s", r.status, r.raw)
	}
	r = collect(h, key, collectReq{ip: "203.0.113.5", body: map[string]any{"a": map[string]any{"platform": "ios"}, "e": []any{screen("Home", now)}}})
	if r.status != http.StatusBadRequest {
		t.Fatalf("no process key: %d %s", r.status, r.raw)
	}
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/analytics/properties/"+id+"/query",
		map[string]any{"metrics": []string{"visitors"}, "range": dayRange("2026-10-12")})
	if n := h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1 AND property_id = $2", tm.ws, webOnly); n != 0 {
		t.Fatalf("%d events on the web property", n)
	}
}

func TestAnalyticsConsentNeverKeepsAnInvalidEvent(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	id, key := createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"example.com"}, "collection": "consent"})
	h.clock.Set(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	consented := map[string]any{"a": 1}
	r := collect(h, key, collectReq{origin: "https://example.com", ip: "203.0.113.10", body: map[string]any{"e": []any{
		map[string]any{"n": "pageview", "u": "https://example.com/pricing", "c": consented},
		map[string]any{"n": "pageview", "u": "not a url", "c": consented},
	}}})
	if r.status != http.StatusAccepted {
		t.Fatalf("status %d: %s", r.status, r.raw)
	}
	h.flushAnalytics()
	if n := h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 1 {
		t.Fatalf("%d events stored, want only the valid one", n)
	}
	health := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/health?days=1", nil)
	if totals(health)["dropped"].(map[string]any)["invalid"].(float64) != 1 {
		t.Fatalf("health %s", health.raw)
	}
}

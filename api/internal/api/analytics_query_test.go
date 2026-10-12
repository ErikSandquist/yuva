package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/api"
)

// writeCityDB writes a small city database: 203.0.113.0/24 is Istanbul, 198.51.100.0/24 Berlin and
// 2001:db8::/32 Los Angeles.
func writeCityDB(t *testing.T) string {
	t.Helper()
	tree, err := mmdbwriter.New(mmdbwriter.Options{DatabaseType: "DBIP-City-Lite", IncludeReservedNetworks: true})
	if err != nil {
		t.Fatal(err)
	}
	for cidr, p := range map[string][3]string{
		"203.0.113.0/24": {"TR", "Istanbul", "Istanbul"}, "198.51.100.0/24": {"DE", "Berlin", "Berlin"}, "2001:db8::/32": {"US", "California", "Los Angeles"},
	} {
		_, n, _ := net.ParseCIDR(cidr)
		if err := tree.Insert(n, mmdbtype.Map{
			"country":      mmdbtype.Map{"iso_code": mmdbtype.String(p[0])},
			"subdivisions": mmdbtype.Slice{mmdbtype.Map{"names": mmdbtype.Map{"en": mmdbtype.String(p[1])}}},
			"city":         mmdbtype.Map{"names": mmdbtype.Map{"en": mmdbtype.String(p[2])}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if _, err := tree.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "city.mmdb")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func geoHarness(t *testing.T, configure func(*api.Deps)) *harness {
	t.Helper()
	path := writeCityDB(t)
	h := newHarnessWith(t, func(d *api.Deps) {
		d.Analytics.GeoIP, d.Analytics.GeoIPFile = analytics.GeoFile, path
		if configure != nil {
			configure(d)
		}
	})
	if _, err := h.srv.UpdateGeoIP(context.Background()); err != nil {
		t.Fatal(err)
	}
	return h
}

func (c *clock) Set(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = at
}

const (
	uaMac     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15"
	uaPixel   = "Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36"
	uaWindows = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:143.0) Gecko/20100101 Firefox/143.0"
)

type visitStep struct {
	ua, ip, page, referrer string
	engaged                int
}

// sendVisits collects each visitor's pages in order, with an engagement after each page when asked.
func sendVisits(h *harness, key string, steps []visitStep) {
	h.t.Helper()
	for _, s := range steps {
		ev := map[string]any{"n": "pageview", "u": s.page, "w": 1280}
		if s.referrer != "" {
			ev["r"] = s.referrer
		}
		r := collect(h, key, collectReq{origin: "https://example.com", ip: s.ip, header: map[string]string{"User-Agent": s.ua}, body: ev})
		if r.status != http.StatusAccepted {
			h.t.Fatalf("collect %d %s", r.status, r.raw)
		}
		if s.engaged > 0 {
			r = collect(h, key, collectReq{origin: "https://example.com", ip: s.ip, header: map[string]string{"User-Agent": s.ua},
				body: map[string]any{"n": "engagement", "u": s.page, "ms": s.engaged, "sd": 60}})
			if r.status != http.StatusAccepted {
				h.t.Fatalf("collect %d %s", r.status, r.raw)
			}
		}
	}
	h.flushAnalytics()
}

func trafficOf(day int) []visitStep {
	return []visitStep{
		{uaMac, "203.0.113.10", "https://example.com/?utm_source=newsletter&utm_medium=email&utm_campaign=oct", "", 5000},
		{uaMac, "203.0.113.10", "https://example.com/pricing", "", 12000},
		{uaPixel, "198.51.100.20", "https://example.com/blog/" + fmt.Sprint(day), "https://www.google.com/search?q=x", 3000},
		{uaPixel, "198.51.100.20", "https://example.com/pricing", "", 0},
		{uaPixel, "198.51.100.20", "https://example.com/signup", "", 8000},
		{uaWindows, "2001:db8:1::7", "https://example.com/pricing", "https://chatgpt.com/", 0},
		{uaWindows, "203.0.113.99", "https://example.com/docs", "https://blog.example.net/post", 1500},
		{uaMac, "198.51.100.77", "https://example.com/", "https://m.facebook.com/", 0},
	}
}

func query(c *client, id string, q map[string]any) response {
	c.h.t.Helper()
	return c.expect(http.StatusOK, "POST", "/v1/analytics/properties/"+id+"/query", q)
}

func dayRange(day string) map[string]any { return map[string]any{"from": day, "to": day} }

var allMetrics = []string{"visitors", "visits", "pageviews", "views_per_visit", "bounce_rate", "visit_duration", "time_on_page", "scroll_depth"}

// rollupQueries are the queries the rollups answer.
func rollupQueries(day string) []map[string]any {
	qs := []map[string]any{
		{"metrics": allMetrics, "range": dayRange(day)},
		{"metrics": allMetrics, "range": dayRange(day), "dimensions": []string{"time:day"}},
		{"metrics": []string{"visitors", "visits", "pageviews"}, "range": dayRange(day), "dimensions": []string{"time:hour"}},
		{"metrics": allMetrics, "range": dayRange(day), "filters": []map[string]any{{"dimension": "channel", "op": "is", "values": []string{"organic_search"}}}},
		{"metrics": allMetrics, "range": dayRange(day), "dimensions": []string{"page"}, "filters": []map[string]any{{"dimension": "country", "op": "is", "values": []string{"DE"}}}},
	}
	for _, d := range []string{"page", "entry_page", "exit_page", "channel", "source", "referrer", "utm_source", "utm_medium", "utm_campaign",
		"country", "region", "city", "device", "browser", "browser_version", "os", "os_version", "screen_size", "language", "platform", "hostname"} {
		qs = append(qs, map[string]any{"metrics": allMetrics, "range": dayRange(day), "dimensions": []string{d}, "limit": 1000})
	}
	return qs
}

func numbers(r response) string {
	b, _ := json.Marshal(map[string]any{"rows": r.body["rows"], "totals": r.body["totals"]})
	return string(b)
}

func served(r response) string {
	return r.body["meta"].(map[string]any)["served_from"].(string)
}

func TestAnalyticsRollupsEqualRawAcrossTimeZones(t *testing.T) {
	h := geoHarness(t, nil)
	tm := newTeam(t, h)
	ist, istKey := createProperty(tm.owner, map[string]any{"name": "Istanbul", "domains": []string{"example.com"}, "timezone": "Europe/Istanbul"})
	la, laKey := createProperty(tm.owner, map[string]any{"name": "Los Angeles", "domains": []string{"example.com"}, "timezone": "America/Los_Angeles"})
	ctx := context.Background()

	h.clock.Set(time.Date(2026, 10, 10, 21, 30, 0, 0, time.UTC))
	sendVisits(h, istKey, trafficOf(1))
	sendVisits(h, laKey, trafficOf(1))
	h.clock.Set(time.Date(2026, 10, 11, 6, 30, 0, 0, time.UTC))
	sendVisits(h, istKey, trafficOf(2))
	sendVisits(h, laKey, trafficOf(2))

	for _, c := range []struct {
		id, day, other string
		hours          []string
	}{
		{ist, "2026-10-11", "2026-10-10", []string{"2026-10-11T00:00", "2026-10-11T09:00"}},
		{la, "2026-10-10", "2026-10-11", []string{"2026-10-10T14:00", "2026-10-10T23:00"}},
	} {
		r := query(tm.owner, c.id, map[string]any{"metrics": []string{"visitors", "pageviews"}, "range": map[string]any{"from": "2026-10-10", "to": "2026-10-11"}, "dimensions": []string{"time:day"}})
		rows := r.body["rows"].([]any)
		byDay := map[string]float64{}
		for _, row := range rows {
			m := row.(map[string]any)
			byDay[m["dimensions"].([]any)[0].(string)] = m["metrics"].(map[string]any)["pageviews"].(float64)
		}
		if len(rows) != 2 || byDay[c.day] != 16 || byDay[c.other] != 0 {
			t.Fatalf("%s: days %s", c.day, r.raw)
		}
		r = query(tm.owner, c.id, map[string]any{"metrics": []string{"pageviews"}, "range": dayRange(c.day), "dimensions": []string{"time:hour"}})
		var hours []string
		for _, row := range r.body["rows"].([]any) {
			m := row.(map[string]any)
			if m["metrics"].(map[string]any)["pageviews"].(float64) > 0 {
				hours = append(hours, m["dimensions"].([]any)[0].(string))
			}
		}
		if len(r.body["rows"].([]any)) != 24 || !reflect.DeepEqual(hours, c.hours) {
			t.Fatalf("%s: hours %v", c.day, hours)
		}
	}

	raw := map[string][]string{}
	for _, c := range []struct{ id, day string }{{ist, "2026-10-11"}, {la, "2026-10-10"}} {
		for _, q := range rollupQueries(c.day) {
			r := query(tm.owner, c.id, q)
			if served(r) != "raw" {
				t.Fatalf("an open day was served from %s", served(r))
			}
			raw[c.id] = append(raw[c.id], numbers(r))
		}
	}
	city := query(tm.owner, la, map[string]any{"metrics": []string{"visitors"}, "range": dayRange("2026-10-10"), "dimensions": []string{"city"}})
	found := false
	for _, row := range city.body["rows"].([]any) {
		m := row.(map[string]any)
		if m["dimensions"].([]any)[0] == "Istanbul" {
			found = m["country"] == "TR" && m["region"] == "Istanbul" && m["metrics"].(map[string]any)["visitors"].(float64) == 2
		}
	}
	if !found {
		t.Fatalf("cities %s", city.raw)
	}
	channels := query(tm.owner, la, map[string]any{"metrics": []string{"visitors"}, "range": dayRange("2026-10-10"), "dimensions": []string{"channel"}})
	for _, want := range []string{"email", "organic_search", "ai_assistants", "referral", "social"} {
		if !strings.Contains(string(channels.raw), `"`+want+`"`) {
			t.Fatalf("no %s in %s", want, channels.raw)
		}
	}

	queued := func(property, day string) int {
		return h.count("SELECT count(*) FROM river_job WHERE kind = 'analytics_rollup' AND args->>'property_id' = $1 AND args->>'day' = $2", property, day)
	}
	h.clock.Set(time.Date(2026, 10, 11, 8, 59, 0, 0, time.UTC))
	if _, err := h.srv.ScanAnalyticsRollups(ctx); err != nil || queued(la, "2026-10-10") != 0 {
		t.Fatalf("a day was queued before its salt's grace ended: %v", err)
	}
	h.clock.Set(time.Date(2026, 10, 11, 9, 1, 0, 0, time.UTC))
	if _, err := h.srv.ScanAnalyticsRollups(ctx); err != nil || queued(la, "2026-10-10") != 1 || queued(ist, "2026-10-11") != 0 {
		t.Fatalf("scan two hours after the Los Angeles midnight: %v", err)
	}
	if err := h.srv.RollupAnalyticsDay(ctx, uuid.MustParse(tm.ws), uuid.MustParse(la), time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	h.clock.Set(time.Date(2026, 10, 11, 23, 1, 0, 0, time.UTC))
	if _, err := h.srv.ScanAnalyticsRollups(ctx); err != nil || queued(ist, "2026-10-11") != 1 {
		t.Fatalf("scan two hours after the Istanbul midnight: %v", err)
	}
	if err := h.srv.RollupAnalyticsDay(ctx, uuid.MustParse(tm.ws), uuid.MustParse(ist), time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ id, day string }{{ist, "2026-10-11"}, {la, "2026-10-10"}} {
		for i, q := range rollupQueries(c.day) {
			r := query(tm.owner, c.id, q)
			if served(r) != "rollups" {
				t.Fatalf("%v was served from %s", q, served(r))
			}
			if got := numbers(r); got != raw[c.id][i] {
				t.Fatalf("%s %v:\nrollups %s\nraw     %s", c.day, q["dimensions"], got, raw[c.id][i])
			}
		}
	}

	h.clock.Set(time.Date(2026, 10, 11, 23, 30, 0, 0, time.UTC))
	sendVisits(h, laKey, trafficOf(3)[:2])
	if r := query(tm.owner, la, map[string]any{"metrics": []string{"pageviews"}, "range": map[string]any{"from": "2026-10-10", "to": "2026-10-11"}}); served(r) != "mixed" ||
		r.body["totals"].(map[string]any)["pageviews"].(float64) != 18 {
		t.Fatalf("rollups and today's raw rows: %s", r.raw)
	}
}

func TestAnalyticsSaltIsGoneTwoHoursAfterMidnight(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	id, key := createProperty(tm.owner, map[string]any{"name": "Salted", "domains": []string{"example.com"}, "timezone": "Europe/Istanbul"})
	ctx := context.Background()
	h.clock.Set(time.Date(2026, 10, 11, 20, 30, 0, 0, time.UTC))
	sendVisits(h, key, trafficOf(1)[:1])
	salts := func() int {
		return h.count("SELECT count(*) FROM analytics_salts WHERE workspace_id = $1 AND property_id = $2 AND day = '2026-10-11'", tm.ws, id)
	}
	var visitor int64
	if err := h.st.Pool.QueryRow(ctx, "SELECT visitor FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2", tm.ws, id).Scan(&visitor); err != nil {
		t.Fatal(err)
	}
	h.clock.Set(time.Date(2026, 10, 11, 22, 59, 0, 0, time.UTC))
	if _, err := h.srv.DeleteExpiredAnalyticsSalts(ctx); err != nil || salts() != 1 {
		t.Fatalf("the salt went before the grace ended: %v", err)
	}
	sendVisits(h, key, trafficOf(1)[:1])
	h.clock.Set(time.Date(2026, 10, 11, 23, 0, 0, 0, time.UTC))
	if _, err := h.srv.DeleteExpiredAnalyticsSalts(ctx); err != nil || salts() != 0 {
		t.Fatalf("the salt outlived midnight plus two hours: %v", err)
	}
	if n := h.count("SELECT count(*) FROM analytics_salts WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 1 {
		t.Fatalf("%d salts left; the next day's should stay", n)
	}
	rows, err := h.st.Pool.Query(ctx, "SELECT salt FROM analytics_salts")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	short, _ := analytics.Shorten("203.0.113.10")
	ua := uaMac + "\x00\x00"
	for rows.Next() {
		var salt []byte
		if err := rows.Scan(&salt); err != nil {
			t.Fatal(err)
		}
		if analytics.VisitorHash(salt, uuid.MustParse(id), short, ua) == visitor {
			t.Fatal("a remaining salt recomputes the deleted day's visitor")
		}
	}
}

func TestAnalyticsBotsAreDropped(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	id, key := createProperty(tm.owner, map[string]any{"name": "Bots", "domains": []string{"example.com"}})
	page := pageview("https://example.com/")
	for _, r := range []collectReq{
		{origin: "https://example.com", ip: "203.0.113.1", header: map[string]string{"User-Agent": "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"}, body: page},
		{origin: "https://example.com", ip: "203.0.113.2", header: map[string]string{"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/141.0.0.0 Safari/537.36"}, body: page},
		{origin: "https://example.com", ip: "203.0.113.3", header: map[string]string{"User-Agent": "curl/8.9.1"}, body: page},
		{origin: "https://example.com", ip: "203.0.113.4", header: map[string]string{"Sec-Fetch-Site": "", "Accept-Language": ""}, body: page},
		{origin: "https://example.com", ip: "203.0.113.5", body: map[string]any{"n": "pageview", "u": "https://example.com/", "r": "https://best-seo-offer.com/"}},
	} {
		if res := collect(h, key, r); res.status != http.StatusAccepted {
			t.Fatalf("collect %d %s", res.status, res.raw)
		}
	}
	h.flushAnalytics()
	if n := h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 0 {
		t.Fatalf("%d bot events stored", n)
	}
	if n := h.count("SELECT coalesce(sum(events), 0) FROM analytics_ingest_stats WHERE workspace_id = $1 AND property_id = $2 AND reason = 'bot'", tm.ws, id); n != 5 {
		t.Fatalf("%d bots counted", n)
	}

	sendVisits(h, key, trafficOf(1)[:2])
	for i := 0; i < analytics.BotBurstViews+5; i++ {
		h.clock.Advance(2 * time.Second)
		if r := collect(h, key, collectReq{origin: "https://example.com", ip: "198.51.100.9", body: pageview(fmt.Sprintf("https://example.com/p/%d", i))}); r.status != http.StatusAccepted {
			t.Fatalf("collect %d %s", r.status, r.raw)
		}
	}
	h.flushAnalytics()
	if n := h.count("SELECT count(*) FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2 AND bot", tm.ws, id); n != 1 {
		t.Fatalf("%d visits marked as bots", n)
	}
	r := query(tm.owner, id, map[string]any{"metrics": []string{"visitors", "pageviews"}, "range": map[string]any{"period": "today"}})
	if m := r.body["totals"].(map[string]any); m["visitors"].(float64) != 1 || m["pageviews"].(float64) != 2 {
		t.Fatalf("a bot visit is counted: %s", r.raw)
	}
	if n := h.count("SELECT coalesce(sum(events), 0) FROM analytics_ingest_stats WHERE workspace_id = $1 AND property_id = $2 AND reason = 'bot'", tm.ws, id); n != 10 {
		t.Fatalf("%d bots counted after the burst", n)
	}
	health := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/health?days=2", nil)
	totals := health.body["totals"].(map[string]any)
	if totals["dropped"].(map[string]any)["bot"].(float64) != 10 || totals["accepted"].(float64) != float64(4+analytics.BotBurstViews) ||
		len(health.body["days"].([]any)) != 2 || health.body["geoip"].(map[string]any)["source"] != "off" {
		t.Fatalf("health %s", health.raw)
	}
}

func TestAnalyticsQueryAccessAndAnswers(t *testing.T) {
	h := geoHarness(t, func(d *api.Deps) { d.Analytics.RawDays = 7 })
	tm := newTeam(t, h)
	other := newTeam(t, h)
	tm.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+tm.inbox+"/members/"+tm.agentID, nil)
	linked, key := createProperty(tm.owner, map[string]any{"name": "Linked", "domains": []string{"example.com"}, "inbox_id": tm.inbox})
	unlinked, _ := createProperty(tm.owner, map[string]any{"name": "Unlinked", "domains": []string{"example.com"}})
	kids, kidsKey := createProperty(tm.owner, map[string]any{"name": "Kids", "domains": []string{"example.com"}, "audience": "children"})
	sendVisits(h, key, trafficOf(1))
	sendVisits(h, kidsKey, trafficOf(1)[:1])

	q := map[string]any{"metrics": []string{"visitors"}, "range": map[string]any{"period": "today"}}
	for _, path := range []string{"/query", "/realtime", "/health"} {
		method, body := "GET", any(nil)
		if path == "/query" {
			method, body = "POST", q
		}
		tm.agent.expect(http.StatusOK, method, "/v1/analytics/properties/"+linked+path, body)
		tm.agent.expectProblem(http.StatusNotFound, "not_found", method, "/v1/analytics/properties/"+unlinked+path, body)
		other.owner.expectProblem(http.StatusNotFound, "not_found", method, "/v1/analytics/properties/"+linked+path, body)
		other.agent.expectProblem(http.StatusNotFound, "not_found", method, "/v1/analytics/properties/"+linked+path, body)
	}
	plain := h.client()
	plain.bearer = tm.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "plain", "scopes": []string{"conversations:read"}}).str("secret")
	plain.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", "/v1/analytics/properties/"+linked+"/query", q)
	limited := h.client()
	limited.bearer = tm.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "limited", "inbox_ids": []string{tm.inbox}, "scopes": []string{"analytics:read"}}).str("secret")
	limited.expect(http.StatusOK, "POST", "/v1/analytics/properties/"+linked+"/query", q)
	limited.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/analytics/properties/"+unlinked+"/query", q)

	r := query(tm.owner, linked, map[string]any{"metrics": []string{"visitors", "pageviews", "bounce_rate"}, "range": map[string]any{"period": "today"},
		"dimensions": []string{"page"}, "limit": 2, "compare": map[string]any{"with": "previous"}})
	if len(r.body["rows"].([]any)) != 2 || r.str("next_cursor") == "" || r.body["comparison"] == nil {
		t.Fatalf("page one %s", r.raw)
	}
	first := r.body["rows"].([]any)[0].(map[string]any)
	if first["dimensions"].([]any)[0] != "/pricing" || first["metrics"].(map[string]any)["visitors"].(float64) != 3 {
		t.Fatalf("top page %v", first)
	}
	r2 := query(tm.owner, linked, map[string]any{"metrics": []string{"visitors"}, "range": map[string]any{"period": "today"}, "dimensions": []string{"page"}, "limit": 2, "cursor": r.str("next_cursor")})
	if len(r2.body["rows"].([]any)) != 2 || r2.body["rows"].([]any)[0].(map[string]any)["dimensions"].([]any)[0] == "/pricing" {
		t.Fatalf("page two %s", r2.raw)
	}

	tooOld := map[string]any{"metrics": []string{"visitors"}, "range": map[string]any{"period": "30d"}, "dimensions": []string{"channel", "country"}}
	res := tm.owner.do("POST", "/v1/analytics/properties/"+linked+"/query", tooOld)
	if res.status != http.StatusUnprocessableEntity || res.str("code") != "needs_raw_window" || res.str("raw_window_from") == "" {
		t.Fatalf("needs raw window: %d %s", res.status, res.raw)
	}
	tm.owner.expectProblem(http.StatusUnprocessableEntity, "range_too_large", "POST", "/v1/analytics/properties/"+linked+"/query",
		map[string]any{"metrics": []string{"visitors"}, "range": map[string]any{"from": "2024-01-01", "to": "2026-01-01"}})
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties/"+linked+"/query",
		map[string]any{"metrics": []string{"visitors", "pageviews"}, "range": map[string]any{"period": "today"}, "dimensions": []string{"time:hour", "page"}})
	both := query(tm.owner, linked, map[string]any{"metrics": []string{"visitors"}, "range": map[string]any{"period": "7d"}, "dimensions": []string{"channel", "country"}})
	if served(both) != "raw" || !strings.Contains(string(both.raw), `"dimensions":["organic_search","DE"]`) {
		t.Fatalf("two dimensions %s", both.raw)
	}
	filtered := query(tm.owner, linked, map[string]any{"metrics": []string{"visitors", "pageviews"}, "range": map[string]any{"period": "today"},
		"filters": []map[string]any{{"dimension": "page", "op": "matches", "values": []string{"/blog/*"}}}})
	if m := filtered.body["totals"].(map[string]any); m["visitors"].(float64) != 1 || m["pageviews"].(float64) != 3 {
		t.Fatalf("page filter %s", filtered.raw)
	}

	csvReq := map[string]any{"metrics": []string{"visitors", "pageviews"}, "range": map[string]any{"period": "today"}, "dimensions": []string{"city"}}
	body, _ := json.Marshal(csvReq)
	req, _ := http.NewRequest("POST", h.url+"/v1/analytics/properties/"+linked+"/query?format=csv", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", tm.owner.ip)
	res2, err := tm.owner.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var csv bytes.Buffer
	_, _ = csv.ReadFrom(res2.Body)
	res2.Body.Close()
	if res2.StatusCode != http.StatusOK || !strings.HasPrefix(res2.Header.Get("Content-Type"), "text/csv") ||
		!strings.HasPrefix(csv.String(), `"city","country","region","visitors","pageviews"`+"\r\n") || !strings.Contains(csv.String(), `"Istanbul","TR","Istanbul","2","3"`) {
		t.Fatalf("csv %d %s", res2.StatusCode, csv.String())
	}

	rt := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+linked+"/realtime", nil)
	if rt.body["visitors"].(float64) != 5 || rt.body["pageviews"].(float64) != 8 || len(rt.body["minutes"].([]any)) != 30 ||
		rt.body["pages"].([]any)[0].(map[string]any)["value"] != "/pricing" {
		t.Fatalf("realtime %s", rt.raw)
	}
	ctx := context.Background()
	var country, region, city string
	if err := h.st.Pool.QueryRow(ctx, "SELECT country, region, city FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2", tm.ws, kids).Scan(&country, &region, &city); err != nil {
		t.Fatal(err)
	}
	if country != "TR" || region != "" || city != "" {
		t.Fatalf("a children's property keeps %q %q %q", country, region, city)
	}
}

func TestAnalyticsMaintenance(t *testing.T) {
	h := newHarnessWith(t, func(d *api.Deps) {
		d.Analytics.RawDays = 7
		d.OperatorEmails = []string{"operator-" + unique("op") + "@example.com"}
	})
	tm := newTeam(t, h)
	id, _ := createProperty(tm.owner, map[string]any{"name": "Kept", "domains": []string{"example.com"}, "retention_months": 1})
	ctx := context.Background()
	pg := &analytics.PGStore{Pool: h.st.Pool}
	today := analytics.LocalDay(h.clock.Now(), time.UTC)
	old := today.AddDate(0, 0, -30)
	if err := pg.EnsurePartitions(ctx, old, old); err != nil {
		t.Fatal(err)
	}
	ws, prop := uuid.MustParse(tm.ws), uuid.MustParse(id)
	for _, day := range []time.Time{today.AddDate(0, -2, 0), today.AddDate(0, 0, -3)} {
		if _, err := h.st.Pool.Exec(ctx, `INSERT INTO analytics_daily (workspace_id, property_id, day, dim, value, visitors, visits, views, bounces,
			duration_s, events, engaged_ms, engaged_views, scroll_sum, scroll_views) VALUES ($1, $2, $3, 0, '', 1, 1, 1, 0, 0, 0, 0, 0, 0, 0)`, ws, prop, day); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.srv.MaintainAnalytics(ctx); err != nil {
		t.Fatal(err)
	}
	exists := func(name string) bool {
		return h.count("SELECT count(*) FROM pg_class WHERE relname = $1", name) == 1
	}
	if exists(analytics.PartitionName(analytics.EventsTable, old)) || exists(analytics.PartitionName(analytics.VisitsTable, old)) {
		t.Fatal("a partition past the raw window was kept")
	}
	for _, d := range []time.Time{today.AddDate(0, 0, -4), today, today.AddDate(0, 0, 7)} {
		if !exists(analytics.PartitionName(analytics.EventsTable, d)) || !exists(analytics.PartitionName(analytics.VisitsTable, d)) {
			t.Fatalf("no partition for %s", d.Format(time.DateOnly))
		}
	}
	if n := h.count("SELECT count(*) FROM analytics_daily WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 1 {
		t.Fatalf("%d rollup rows; the one past the retention should be gone", n)
	}

	day := today.AddDate(0, 0, -2)
	if _, err := h.st.Pool.Exec(ctx, `INSERT INTO analytics_ingest_stats (workspace_id, property_id, day, reason, events) VALUES
		($1, $2, $3, 'accepted', 40), ($1, $2, $3, 'bot', 90), ($1, $2, $3, 'rate', 30), ($1, $2, $3, 'over_limit', 500)`, ws, prop, day); err != nil {
		t.Fatal(err)
	}
	if err := h.srv.RollupAnalyticsDay(ctx, ws, prop, day); err != nil {
		t.Fatal(err)
	}
	if err := h.srv.RollupAnalyticsDay(ctx, ws, prop, day); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM operator_notices n JOIN workspaces w ON w.name = n.workspace_name WHERE w.id = $1 AND n.kind = 'analytics_dropped' AND n.detail = $2",
		tm.ws, "120 of 160 analytics events on "+day.Format(time.DateOnly)+" were dropped"); n != 1 {
		t.Fatalf("%d operator notices about a mostly dropped day", n)
	}
}

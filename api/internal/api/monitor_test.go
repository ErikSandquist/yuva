package api_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/api"
)

func sendRecords(c *client, want int, id string, recs ...map[string]any) response {
	c.h.t.Helper()
	return c.expect(want, "POST", "/v1/analytics/properties/"+id+"/requests", map[string]any{"records": recs})
}

func apiQuery(c *client, id string, q map[string]any) response {
	c.h.t.Helper()
	q["dataset"] = "api"
	return query(c, id, q)
}

func createAPIProperty(c *client, body map[string]any) string {
	c.h.t.Helper()
	body["kind"] = "api"
	if _, ok := body["domains"]; !ok {
		body["domains"] = []string{}
	}
	id, _ := createProperty(c, body)
	return id
}

func (h *harness) flushMonitor() {
	h.t.Helper()
	if err := h.srv.FlushMonitor(context.Background()); err != nil {
		h.t.Fatal(err)
	}
}

// sent is what the test sent of one route, to check the answers against.
type sent struct {
	requests, errors, clientErrors int
	durations                      []int64
}

func (s *sent) percentile(q float64) float64 {
	d := slices.Clone(s.durations)
	slices.Sort(d)
	rank := min(max(int(math.Ceil(q*float64(len(d)))), 1), len(d))
	return float64(d[rank-1]) / 1000
}

func metricOf(m map[string]any, key string) float64 {
	v, _ := m[key].(float64)
	return v
}

func within(got, want, rel float64) bool {
	if want == 0 {
		return got == 0
	}
	return math.Abs(got-want)/want <= rel
}

var apiMetrics = []string{"requests", "errors", "error_rate", "client_errors", "client_error_rate", "p50", "p95", "p99", "avg_duration"}

func TestMonitorCountsRoutesAndPercentiles(t *testing.T) {
	h := geoHarness(t, nil)
	tm := newTeam(t, h)
	id := createAPIProperty(tm.owner, map[string]any{"name": "Shop API", "domains": []string{"api.example.com"}, "timezone": "Asia/Kolkata"})
	ctx := context.Background()
	now := time.Date(2026, 10, 12, 9, 0, 30, 0, time.UTC)
	h.clock.Set(now)
	backend := tm.apiKey(h)

	r := rand.New(rand.NewPCG(11, 12))
	want := map[string]*sent{}
	var batch []map[string]any
	add := func(route string, rec map[string]any) {
		s := want[route]
		if s == nil {
			s = &sent{}
			want[route] = s
		}
		status := rec["status"].(int)
		s.requests++
		if status == 0 || status >= 500 {
			s.errors++
		} else if status >= 400 {
			s.clientErrors++
		}
		s.durations = append(s.durations, rec["duration_us"].(int64))
		batch = append(batch, rec)
	}
	for i := range 2400 {
		dur := int64(math.Exp(r.NormFloat64()*0.9+9.5)) + 1
		at := now.Add(-time.Duration(r.IntN(150)) * time.Second).Format(time.RFC3339Nano)
		switch i % 6 {
		case 0, 1, 2:
			status := []int{200, 200, 200, 201, 404, 500, 503, 0}[r.IntN(8)]
			add("/users/{id}", map[string]any{"method": "GET", "route": "/users/{id}", "status": status, "duration_us": dur, "time": at,
				"consumer": []string{"mobile-app", "partner-api", "ayse@example.com"}[i%3], "ip": []string{"203.0.113.7", "198.51.100.20", "2001:db8:1::7"}[i%3],
				"response_size": int64(200 + r.IntN(5000))})
		case 3:
			add("/orders/{id}/items", map[string]any{"method": "post", "path": fmt.Sprintf("/orders/%d/items?coupon=SPRING", 1000+r.IntN(900)), "status": 201,
				"duration_us": dur * 3, "time": at, "environment": "staging"})
		case 4:
			add("/files/{id}", map[string]any{"method": "GET", "path": "/files/" + uuid.New().String(), "status": []int{200, 304, 403}[r.IntN(3)],
				"duration_us": dur / 2, "time": at})
		case 5:
			add("(unmatched)", map[string]any{"method": "GET", "path": fmt.Sprintf("/wp-admin/%d/setup.php", r.IntN(1e6)), "status": 404, "duration_us": int64(80 + r.IntN(40)),
				"time": at})
		}
	}
	for start := 0; start < len(batch); start += 1000 {
		res := sendRecords(backend, http.StatusAccepted, id, batch[start:min(start+1000, len(batch))]...)
		if len(res.body["errors"].([]any)) != 0 {
			t.Fatalf("records refused: %s", res.raw)
		}
	}
	invalid := sendRecords(backend, http.StatusAccepted, id,
		map[string]any{"method": "GET", "route": "/ok", "status": 200, "duration_us": 10},
		map[string]any{"method": "GET", "status": 200, "duration_us": 10},
		map[string]any{"method": "GET", "route": "/x", "status": 700, "duration_us": 10},
		map[string]any{"method": "GET", "route": "/x", "status": 500, "duration_us": 10, "sample_rate": 10},
		map[string]any{"method": "GET", "route": "/old", "status": 200, "duration_us": 10, "time": now.Add(-73 * time.Hour).Format(time.RFC3339)},
	)
	if invalid.body["accepted"].(float64) != 2 || len(invalid.body["errors"].([]any)) != 3 {
		t.Fatalf("invalid records %s", invalid.raw)
	}
	add("/ok", map[string]any{"method": "GET", "status": 200, "duration_us": int64(10)})
	h.flushMonitor()

	if n := h.count("SELECT count(*) FROM monitor_minute WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n == 0 || n > 300 {
		t.Fatalf("%d minute rows for %d records", n, len(batch))
	}
	for _, table := range []string{"monitor_minute", "monitor_dim_hour", "monitor_names", "monitor_routes", "monitor_rollup_hours", "monitor_ingest_stats"} {
		var dump string
		if err := h.st.Pool.QueryRow(ctx, "SELECT coalesce(string_agg(t::text, '|'), '') FROM "+table+" t WHERE t.workspace_id = $1", tm.ws).Scan(&dump); err != nil {
			t.Fatal(err)
		}
		for _, addr := range []string{"203.0.113", "198.51.100", "2001:db8", "ayse@example.com", "SPRING"} {
			if strings.Contains(dump, addr) {
				t.Fatalf("%s holds %q", table, addr)
			}
		}
	}
	var routes []string
	rows, err := h.st.Pool.Query(ctx, "SELECT pattern FROM monitor_routes WHERE workspace_id = $1 AND property_id = $2 AND pattern <> '(other)' ORDER BY pattern", tm.ws, id)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var p string
		_ = rows.Scan(&p)
		routes = append(routes, p)
	}
	rows.Close()
	if fmt.Sprint(routes) != "[(unmatched) /files/{id} /ok /orders/{id}/items /users/{id}]" {
		t.Fatalf("routes %v", routes)
	}
	var countries []string
	if err := h.st.Pool.QueryRow(ctx, "SELECT array_agg(value ORDER BY value) FROM monitor_names WHERE workspace_id = $1 AND kind = 'country' AND value <> '(other)'", tm.ws).Scan(&countries); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(countries) != "[DE TR US]" {
		t.Fatalf("countries %v", countries)
	}
	if n := h.count("SELECT count(*) FROM monitor_names WHERE workspace_id = $1 AND kind = 'consumer' AND value = '(scrubbed)'", tm.ws); n != 1 {
		t.Fatal("the e-mail-shaped consumer is not scrubbed")
	}

	check := func(label string, res response) {
		t.Helper()
		got := rowsBy(res)
		total := 0
		for route, s := range want {
			m := got[route]
			if m == nil {
				t.Fatalf("%s: no row for %s: %s", label, route, res.raw)
			}
			total += s.requests
			errRate := math.Round(float64(s.errors)*10000/float64(s.requests)) / 100
			if m["requests"].(float64) != float64(s.requests) || m["errors"].(float64) != float64(s.errors) ||
				m["client_errors"].(float64) != float64(s.clientErrors) || m["error_rate"].(float64) != errRate {
				t.Fatalf("%s: %s counts %v, sent %+v", label, route, m, *s)
			}
			for _, p := range []struct {
				name string
				q    float64
			}{{"p50", 0.5}, {"p95", 0.95}, {"p99", 0.99}} {
				if !within(m[p.name].(float64), s.percentile(p.q), 0.011) {
					t.Fatalf("%s: %s %s %v, exact %v", label, route, p.name, m[p.name], s.percentile(p.q))
				}
			}
		}
		if len(got) != len(want) || totals(res)["requests"].(float64) != float64(total) {
			t.Fatalf("%s: rows %v", label, got)
		}
	}
	day := "2026-10-12"
	byRoute := map[string]any{"metrics": apiMetrics, "range": dayRange(day), "dimensions": []string{"route"}}
	live := apiQuery(tm.owner, id, byRoute)
	check("minute rows", live)
	if served(live) != "raw" {
		t.Fatalf("served from %s", served(live))
	}
	minutes := apiQuery(tm.owner, id, map[string]any{"metrics": []string{"requests", "errors"}, "range": map[string]any{"period": "15m"}, "dimensions": []string{"time:minute"}})
	list := minutes.body["rows"].([]any)
	sum := 0.0
	for _, row := range list {
		sum += row.(map[string]any)["metrics"].(map[string]any)["requests"].(float64)
	}
	if len(list) != 15 || sum != totals(live)["requests"].(float64) || !strings.HasSuffix(list[len(list)-1].(map[string]any)["dimensions"].([]any)[0].(string), "T14:30+05:30") {
		t.Fatalf("minutes %s", minutes.raw)
	}
	statuses := rowsBy(apiQuery(tm.owner, id, map[string]any{"metrics": []string{"requests"}, "range": dayRange(day), "dimensions": []string{"status_class"},
		"filters": []map[string]any{{"dimension": "route", "op": "is", "values": []string{"/users/{id}"}}}}))
	u := want["/users/{id}"]
	if metricOf(statuses["5xx"], "requests")+metricOf(statuses["none"], "requests") != float64(u.errors) || metricOf(statuses["4xx"], "requests") != float64(u.clientErrors) {
		t.Fatalf("status classes %v", statuses)
	}
	rate := totals(apiQuery(tm.owner, id, map[string]any{"metrics": []string{"requests", "requests_per_minute"}, "range": map[string]any{"period": "1h"}}))
	if rate["requests_per_minute"].(float64) != math.Round(rate["requests"].(float64)/60*100)/100 {
		t.Fatalf("requests per minute %v", rate)
	}
	staging := totals(apiQuery(tm.owner, id, map[string]any{"metrics": []string{"requests"}, "range": dayRange(day),
		"filters": []map[string]any{{"dimension": "environment", "op": "is", "values": []string{"staging"}}}}))
	if staging["requests"].(float64) != float64(want["/orders/{id}/items"].requests) {
		t.Fatalf("staging %v", staging)
	}

	h.clock.Set(time.Date(2026, 10, 13, 6, 0, 0, 0, time.UTC))
	h.flushMonitor()
	if err := h.srv.RollupMonitor(ctx, uuid.MustParse(tm.ws), uuid.MustParse(id)); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM monitor_rollup_hours WHERE workspace_id = $1 AND property_id = $2 AND (built_at IS NULL OR day_built_at IS NULL)", tm.ws, id); n != 0 {
		t.Fatalf("%d hours or days not folded", n)
	}
	folded := apiQuery(tm.owner, id, byRoute)
	check("folded days", folded)
	if served(folded) != "rollups" || numbers(folded) != numbers(live) {
		t.Fatalf("folded days differ from minute rows:\nminutes %s\ndays    %s", numbers(live), numbers(folded))
	}
	hours := apiQuery(tm.owner, id, map[string]any{"metrics": []string{"requests"}, "range": dayRange(day), "dimensions": []string{"time:hour"}})
	if hl := hours.body["rows"].([]any); len(hl) != 24 || hl[14].(map[string]any)["dimensions"].([]any)[0] != "2026-10-12T14:00+05:30" {
		t.Fatalf("local hours %s", hours.raw)
	}
}

func TestMonitorWritersLateRecordsAndRefolds(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	id := createAPIProperty(tm.owner, map[string]any{"name": "API"})
	ctx := context.Background()
	now := time.Date(2026, 10, 12, 12, 0, 20, 0, time.UTC)
	h.clock.Set(now)
	backend := tm.apiKey(h)
	rec := func(at time.Time, status int) map[string]any {
		return map[string]any{"method": "GET", "route": "/ping", "status": status, "duration_us": 1500, "time": at.Format(time.RFC3339)}
	}
	sendRecords(backend, http.StatusAccepted, id, rec(now, 200), rec(now, 500))
	h.flushMonitor()
	h.flushMonitor()
	sendRecords(backend, http.StatusAccepted, id, rec(now, 200))
	h.flushMonitor()
	sum := func() int {
		return h.count("SELECT coalesce(sum(requests), 0)::int FROM monitor_minute WHERE workspace_id = $1 AND property_id = $2", tm.ws, id)
	}
	if n := h.count("SELECT count(*) FROM monitor_minute WHERE workspace_id = $1 AND property_id = $2 AND status = 200", tm.ws, id); n != 1 || sum() != 3 {
		t.Fatalf("cumulative upserts: %d rows, %d requests", n, sum())
	}

	h.clock.Set(time.Date(2026, 10, 12, 13, 10, 0, 0, time.UTC))
	h.flushMonitor()
	if err := h.srv.RollupMonitor(ctx, uuid.MustParse(tm.ws), uuid.MustParse(id)); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM monitor_hour WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 2 {
		t.Fatalf("%d hour rows", n)
	}
	sendRecords(backend, http.StatusAccepted, id, rec(now.Add(5*time.Minute), 200))
	h.flushMonitor()
	if n := h.count("SELECT count(DISTINCT writer) FROM monitor_minute WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 2 || sum() != 4 {
		t.Fatalf("a late record is written under a fresh writer: %d writers, %d requests", n, sum())
	}
	if n := h.count("SELECT count(*) FROM monitor_rollup_hours WHERE workspace_id = $1 AND property_id = $2 AND dirty", tm.ws, id); n != 1 {
		t.Fatal("the late record did not mark its hour dirty")
	}
	if err := h.srv.RollupMonitor(ctx, uuid.MustParse(tm.ws), uuid.MustParse(id)); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT sum(requests)::int FROM monitor_hour WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 4 {
		t.Fatalf("refolded hour holds %d requests", n)
	}
	q := apiQuery(tm.owner, id, map[string]any{"metrics": []string{"requests", "errors"}, "range": dayRange("2026-10-12")})
	if totals(q)["requests"].(float64) != 4 || totals(q)["errors"].(float64) != 1 || served(q) != "rollups" {
		t.Fatalf("after the refold %s", q.raw)
	}
}

func TestMonitorAccessAndKinds(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	id := createAPIProperty(tm.owner, map[string]any{"name": "API"})
	site, siteKey := createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"example.com"}})
	rec := map[string]any{"method": "GET", "route": "/ping", "status": 200, "duration_us": 900}
	body := map[string]any{"records": []any{rec}}
	path := "/v1/analytics/properties/" + id + "/requests"

	reader := h.client()
	reader.bearer = tm.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "reader", "scopes": []string{"analytics:read"}}).str("secret")
	reader.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", path, body)
	tm.owner.expectProblem(http.StatusForbidden, "forbidden", "POST", path, body)
	other.apiKey(h).expectProblem(http.StatusNotFound, "not_found", "POST", path, body)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/analytics/properties/"+id+"/query", map[string]any{"dataset": "api", "metrics": []string{"requests"}, "range": map[string]any{"period": "1h"}})
	backend := tm.apiKey(h)
	backend.expectProblem(http.StatusBadRequest, "validation_failed", "POST", path, map[string]any{"records": []any{}})
	backend.expectProblem(http.StatusUnprocessableEntity, "property_kind", "POST", "/v1/analytics/properties/"+site+"/requests", body)
	sendRecords(backend, http.StatusAccepted, id, rec)

	tm.owner.expectProblem(http.StatusUnprocessableEntity, "property_kind", "POST", "/v1/analytics/properties/"+site+"/query",
		map[string]any{"dataset": "api", "metrics": []string{"requests"}, "range": map[string]any{"period": "1h"}})
	tm.owner.expectProblem(http.StatusUnprocessableEntity, "property_kind", "POST", "/v1/analytics/properties/"+id+"/query",
		map[string]any{"metrics": []string{"visitors"}, "range": map[string]any{"period": "today"}})
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties/"+site+"/query",
		map[string]any{"metrics": []string{"visitors"}, "range": map[string]any{"period": "1h"}})
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties/"+id+"/query",
		map[string]any{"dataset": "api", "metrics": []string{"visitors"}, "range": map[string]any{"period": "1h"}})
	tm.owner.expectProblem(http.StatusUnprocessableEntity, "range_too_large", "POST", "/v1/analytics/properties/"+id+"/query",
		map[string]any{"dataset": "api", "metrics": []string{"requests"}, "range": map[string]any{"period": "7d"}, "dimensions": []string{"time:minute"}})
	backend.expectProblem(http.StatusUnprocessableEntity, "property_kind", "POST", "/v1/analytics/properties/"+id+"/events",
		map[string]any{"events": []any{map[string]any{"name": "pageview", "url": "https://example.com/", "visitor_key": "k"}}})
	tm.owner.expectProblem(http.StatusUnprocessableEntity, "property_kind", "POST", "/v1/analytics/properties/"+id+"/goals",
		map[string]any{"name": "Signed up", "kind": "event", "event_name": "Signup"})
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties",
		map[string]any{"kind": "api", "name": "Web", "domains": []string{}, "platforms": []string{"web"}})
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties",
		map[string]any{"name": "No domains", "domains": []string{}})
	got := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id, nil)
	if got.str("kind") != "api" || got.body["apdex_ms"].(float64) != 500 || got.body["monitor_hour_days"].(float64) != 90 || len(got.body["platforms"].([]any)) != 0 {
		t.Fatalf("API property %s", got.raw)
	}
	tm.owner.expect(http.StatusOK, "PATCH", "/v1/analytics/properties/"+id, map[string]any{"apdex_ms": 250, "monitor_hour_days": 30, "platforms": []string{"ios"}})
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "PATCH", "/v1/analytics/properties/"+id, map[string]any{"monitor_hour_days": 3})
	var pubKey string
	if err := h.st.Pool.QueryRow(context.Background(), "SELECT public_key FROM analytics_properties WHERE id = $1", id).Scan(&pubKey); err != nil {
		t.Fatal(err)
	}
	if r := collect(h, pubKey, collectReq{origin: "https://example.com", ip: "203.0.113.7", body: pageview("https://example.com/")}); r.status != http.StatusNotFound {
		t.Fatalf("collect with an API property's key: %d", r.status)
	}
	if r := collect(h, siteKey, collectReq{origin: "https://example.com", ip: "203.0.113.7", body: pageview("https://example.com/")}); r.status != http.StatusAccepted {
		t.Fatalf("collect on the site: %d", r.status)
	}

	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	_ = json.NewEncoder(zw).Encode(map[string]any{"records": []any{rec, rec}})
	_ = zw.Close()
	req, _ := http.NewRequest(http.MethodPost, h.url+path, &zipped)
	req.Header.Set("Authorization", "Bearer "+backend.bearer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	res.Body.Close()
	if res.StatusCode != http.StatusAccepted || out["accepted"].(float64) != 2 {
		t.Fatalf("gzip body: %d %v", res.StatusCode, out)
	}
	req, _ = http.NewRequest(http.MethodPost, h.url+path, strings.NewReader("x"))
	req.Header.Set("Authorization", "Bearer "+backend.bearer)
	req.Header.Set("Content-Encoding", "br")
	if res, err = http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("brotli body: %d", res.StatusCode)
	}
}

func TestMonitorUsageLimitAndCaps(t *testing.T) {
	operator := "operator-" + unique("op") + "@example.com"
	h := newHarnessWith(t, func(d *api.Deps) { d.OperatorEmails = []string{operator} })
	tm := newTeam(t, h)
	id := createAPIProperty(tm.owner, map[string]any{"name": "API"})
	h.clock.Set(time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC))
	backend := tm.apiKey(h)
	if err := h.srv.OperatorLimitMonitor(context.Background(), uuid.MustParse(tm.ws), 1500); err != nil {
		t.Fatal(err)
	}
	var recs []map[string]any
	for i := range 1000 {
		recs = append(recs, map[string]any{"method": "GET", "route": fmt.Sprintf("/r%d", i), "status": 200, "duration_us": 100})
	}
	sendRecords(backend, http.StatusAccepted, id, recs...)
	sendRecords(backend, http.StatusAccepted, id, map[string]any{"method": "GET", "route": "/one-too-many", "status": 200, "duration_us": 100})
	h.flushMonitor()
	var owner string
	if err := h.st.Pool.QueryRow(context.Background(), "SELECT p.email FROM members m JOIN people p ON p.id = m.person_id WHERE m.workspace_id = $1 AND m.role = 'owner'", tm.ws).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	limitMails := func() int {
		n := 0
		for _, m := range h.mail.to(owner) {
			if strings.Contains(m.Subject, "reached its API monitoring limit") {
				n++
			}
		}
		return n
	}
	if limitMails() != 0 {
		t.Fatal("told about the cap before reaching it")
	}
	sendRecords(backend, http.StatusAccepted, id, recs[:600]...)
	h.flushMonitor()
	usage := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties", nil).body["usage"].(map[string]any)
	if usage["requests"].(float64) != 1500 || usage["requests_limit"].(float64) != 1500 || usage["requests_limit_reached_at"] == nil {
		t.Fatalf("usage %v", usage)
	}
	if n := h.count("SELECT records::int FROM monitor_ingest_stats WHERE workspace_id = $1 AND property_id = $2 AND reason = 'over_limit'", tm.ws, id); n != 101 {
		t.Fatalf("%d records over the limit", n)
	}
	if limitMails() != 1 {
		t.Fatal("the owner was not told about the cap")
	}
	notices := func() int {
		return h.count(`SELECT count(*) FROM operator_notices n JOIN workspaces w ON w.name = n.workspace_name
			WHERE w.id = $1 AND n.kind = 'monitor_limit' AND n.detail LIKE '1,500 API request records%'`, tm.ws)
	}
	if notices() != 1 {
		t.Fatal("no operator notice")
	}
	sendRecords(backend, http.StatusAccepted, id, recs[:10]...)
	h.flushMonitor()
	if limitMails() != 1 || notices() != 1 {
		t.Fatal("told about the cap twice in a month")
	}
	res := apiQuery(tm.owner, id, map[string]any{"metrics": []string{"requests"}, "range": dayRange("2026-10-12"), "dimensions": []string{"route"}, "limit": 1000})
	byRoute := rowsBy(res)
	caps := res.body["meta"].(map[string]any)["caps_reached"].([]any)
	if byRoute["(other)"]["requests"].(float64) != 1 || byRoute["/one-too-many"] != nil || len(caps) != 1 || caps[0] != "routes" {
		t.Fatalf("route cap: other %v, caps %v", byRoute["(other)"], caps)
	}
	if res.body["next_cursor"] == nil {
		t.Fatal("1,001 routes fit on one page of 1,000")
	}
}

func TestMonitorHiddenRoutes(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	id := createAPIProperty(tm.owner, map[string]any{"name": "API"})
	site, _ := createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"example.com"}})
	h.clock.Set(time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC))
	sendRecords(tm.apiKey(h), http.StatusAccepted, id,
		map[string]any{"method": "GET", "route": "/healthz", "status": 200, "duration_us": 50},
		map[string]any{"method": "GET", "route": "/healthz", "status": 200, "duration_us": 50},
		map[string]any{"method": "GET", "path": "/users/42", "status": 200, "duration_us": 5000},
		map[string]any{"method": "GET", "path": "/users/43", "status": 500, "duration_us": 9000},
	)
	h.flushMonitor()
	routes := map[string]map[string]any{}
	for _, r := range tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/routes", nil).body["items"].([]any) {
		m := r.(map[string]any)
		routes[m["pattern"].(string)] = m
	}
	if routes["/healthz"]["inferred"] != false || routes["/users/{id}"]["inferred"] != true || routes["/healthz"]["hidden"] != false || routes["/users/{id}"]["last_seen_at"] == nil {
		t.Fatalf("routes %v", routes)
	}
	health := fmt.Sprint(routes["/healthz"]["id"])
	path := "/v1/analytics/properties/" + id + "/routes/" + health
	q := map[string]any{"metrics": []string{"requests", "errors"}, "range": dayRange("2026-10-12"), "dimensions": []string{"route"}}
	if totals(apiQuery(tm.owner, id, q))["requests"].(float64) != 4 {
		t.Fatal("before hiding")
	}
	hidden := tm.owner.expect(http.StatusOK, "PATCH", path, map[string]any{"hidden": true})
	if hidden.body["hidden"] != true || hidden.str("pattern") != "/healthz" {
		t.Fatalf("hide %s", hidden.raw)
	}
	res := apiQuery(tm.owner, id, q)
	if rows := rowsBy(res); rows["/healthz"] != nil || totals(res)["requests"].(float64) != 2 || totals(res)["errors"].(float64) != 1 {
		t.Fatalf("a hidden route is answered: %s", res.raw)
	}
	only := apiQuery(tm.owner, id, map[string]any{"metrics": []string{"requests"}, "range": dayRange("2026-10-12"),
		"filters": []map[string]any{{"dimension": "route", "op": "is", "values": []string{"/healthz"}}}})
	if totals(only)["requests"].(float64) != 0 {
		t.Fatalf("a filter reaches a hidden route: %s", only.raw)
	}
	tm.owner.expect(http.StatusOK, "PATCH", path, map[string]any{"hidden": false})
	if totals(apiQuery(tm.owner, id, q))["requests"].(float64) != 4 {
		t.Fatal("the route came not back")
	}

	reader := h.client()
	reader.bearer = tm.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "reader", "scopes": []string{"analytics:read"}}).str("secret")
	reader.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/routes", nil)
	reader.expectProblem(http.StatusForbidden, "insufficient_scope", "PATCH", path, map[string]any{"hidden": true})
	tm.agent.expectProblem(http.StatusNotFound, "not_found", "PATCH", path, map[string]any{"hidden": true})
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/analytics/properties/"+id+"/routes", nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "PATCH", path, map[string]any{"hidden": true})
	tm.owner.expectProblem(http.StatusNotFound, "not_found", "PATCH", "/v1/analytics/properties/"+id+"/routes/999999", map[string]any{"hidden": true})
	tm.owner.expectProblem(http.StatusUnprocessableEntity, "property_kind", "GET", "/v1/analytics/properties/"+site+"/routes", nil)
	tm.owner.expectProblem(http.StatusUnprocessableEntity, "property_kind", "PATCH", "/v1/analytics/properties/"+site+"/routes/1", map[string]any{"hidden": true})
}

func TestMonitorTimeZoneChangeAppliesFromTheNextLocalDay(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	id := createAPIProperty(tm.owner, map[string]any{"name": "API", "timezone": "UTC"})
	ctx := context.Background()
	backend := tm.apiKey(h)
	send := func(at time.Time) {
		t.Helper()
		h.clock.Set(at)
		sendRecords(backend, http.StatusAccepted, id, map[string]any{"method": "GET", "route": "/ping", "status": 200, "duration_us": 100, "time": at.Format(time.RFC3339)})
		h.flushMonitor()
	}
	send(time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC))
	h.clock.Set(time.Date(2026, 10, 12, 10, 5, 0, 0, time.UTC))
	changed := tm.owner.expect(http.StatusOK, "PATCH", "/v1/analytics/properties/"+id, map[string]any{"timezone": "Asia/Kolkata"})
	if changed.str("timezone") != "Asia/Kolkata" || changed.str("timezone_applies_at") != "2026-10-13T00:00:00Z" {
		t.Fatalf("change %s", changed.raw)
	}
	send(time.Date(2026, 10, 12, 20, 0, 0, 0, time.UTC))
	send(time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC))
	send(time.Date(2026, 10, 13, 18, 0, 0, 0, time.UTC))
	if got := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id, nil); got.body["timezone_applies_at"] != nil {
		t.Fatalf("the zone still waits: %s", got.raw)
	}
	var days []string
	rows, err := h.st.Pool.Query(ctx, "SELECT to_char(hour AT TIME ZONE 'UTC', 'MM-DD HH24:MI') || ' ' || to_char(day, 'MM-DD') FROM monitor_rollup_hours WHERE property_id = $1 ORDER BY hour", id)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var d string
		_ = rows.Scan(&d)
		days = append(days, d)
	}
	rows.Close()
	if fmt.Sprint(days) != "[10-12 10:00 10-12 10-12 20:00 10-12 10-13 00:30 10-13 10-13 17:30 10-13]" {
		t.Fatalf("hours and their days %v", days)
	}
	answers := func() string {
		t.Helper()
		byDay := apiQuery(tm.owner, id, map[string]any{"metrics": []string{"requests"}, "range": map[string]any{"from": "2026-10-12", "to": "2026-10-13"},
			"dimensions": []string{"time:day"}})
		hours := apiQuery(tm.owner, id, map[string]any{"metrics": []string{"requests"}, "range": dayRange("2026-10-13"), "dimensions": []string{"time:hour"}})
		var hl []string
		for _, r := range hours.body["rows"].([]any) {
			m := r.(map[string]any)
			if m["metrics"].(map[string]any)["requests"].(float64) > 0 {
				hl = append(hl, m["dimensions"].([]any)[0].(string))
			}
		}
		first := hours.body["rows"].([]any)[0].(map[string]any)["dimensions"].([]any)[0]
		return fmt.Sprintf("%s first %v hours %v", numbers(byDay), first, hl)
	}
	want := `{"rows":[{"dimensions":["2026-10-12"],"metrics":{"requests":2}},{"dimensions":["2026-10-13"],"metrics":{"requests":2}}],"totals":{"requests":4}} first 2026-10-13T05:30+05:30 hours [2026-10-13T06:00+05:30 2026-10-13T23:00+05:30]`
	if got := answers(); got != want {
		t.Fatalf("from minute rows:\n%s\nwant\n%s", got, want)
	}
	h.clock.Set(time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC))
	h.flushMonitor()
	if err := h.srv.RollupMonitor(ctx, uuid.MustParse(tm.ws), uuid.MustParse(id)); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM monitor_rollup_hours WHERE property_id = $1 AND (built_at IS NULL OR day_built_at IS NULL)", id); n != 0 {
		t.Fatalf("%d hours or days not folded", n)
	}
	if got := answers(); got != want {
		t.Fatalf("from folded rows:\n%s\nwant\n%s", got, want)
	}
}

func TestMonitorOff(t *testing.T) {
	h := newHarnessWith(t, func(d *api.Deps) { d.Analytics.Monitor.Off = true })
	tm := newTeam(t, h)
	site, _ := createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"example.com"}})
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties", map[string]any{"kind": "api", "name": "API", "domains": []string{}})
	tm.apiKey(h).expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/analytics/properties/"+site+"/requests",
		map[string]any{"records": []any{map[string]any{"method": "GET", "route": "/", "status": 200, "duration_us": 1}}})
	if u := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties", nil).body["usage"].(map[string]any); u["requests"] != nil {
		t.Fatalf("usage %v", u)
	}
}

func TestMonitorDeleteProperty(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	id := createAPIProperty(tm.owner, map[string]any{"name": "API"})
	h.clock.Set(time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC))
	sendRecords(tm.apiKey(h), http.StatusAccepted, id, map[string]any{"method": "GET", "route": "/", "status": 200, "duration_us": 1, "consumer": "c"})
	h.flushMonitor()
	ws, prop := uuid.MustParse(tm.ws), uuid.MustParse(id)
	tm.owner.expect(http.StatusAccepted, "DELETE", "/v1/analytics/properties/"+id, map[string]any{"name": "API"})
	done, err := h.srv.DeleteAnalytics(context.Background(), api.AnalyticsDeleteArgs{WorkspaceID: ws, PropertyID: prop, Property: true}, time.Minute)
	if err != nil || !done {
		t.Fatalf("delete %v %v", done, err)
	}
	for _, table := range []string{"monitor_minute", "monitor_dim_hour", "monitor_rollup_hours", "monitor_routes", "monitor_names"} {
		if n := h.count("SELECT count(*) FROM "+table+" WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 0 {
			t.Fatalf("%s keeps %d rows", table, n)
		}
	}
}

func TestMonitorTimeSeriesWithALocalClockAndAPendingZone(t *testing.T) {
	istanbul, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		t.Fatal(err)
	}
	local := time.Local
	time.Local = istanbul
	t.Cleanup(func() { time.Local = local })
	h := newHarness(t)
	tm := newTeam(t, h)
	id := createAPIProperty(tm.owner, map[string]any{"name": "API", "timezone": "UTC"})
	now := time.Date(2026, 10, 12, 15, 0, 30, 0, time.Local)
	h.clock.Set(now)
	tm.owner.expect(http.StatusOK, "PATCH", "/v1/analytics/properties/"+id, map[string]any{"timezone": "Asia/Kolkata"})
	var recs []map[string]any
	for i := range 30 {
		recs = append(recs, map[string]any{"method": "GET", "route": "/ping", "status": 200, "duration_us": 100,
			"time": now.Add(-time.Duration(i*2) * time.Minute).Format(time.RFC3339)})
	}
	sendRecords(tm.apiKey(h), http.StatusAccepted, id, recs...)
	h.flushMonitor()
	total := totals(apiQuery(tm.owner, id, map[string]any{"metrics": []string{"requests"}, "range": map[string]any{"period": "1h"}}))["requests"].(float64)
	if total != 30 {
		t.Fatalf("totals %v", total)
	}
	for _, c := range []struct {
		grain, period string
		buckets       int
		first, last   string
	}{
		{"time:minute", "1h", 60, "2026-10-12T11:01+00:00", "2026-10-12T12:00+00:00"},
		{"time:hour", "today", 24, "2026-10-12T00:00+00:00", "2026-10-12T23:00+00:00"},
		{"time:day", "7d", 7, "2026-10-06", "2026-10-12"},
	} {
		res := apiQuery(tm.owner, id, map[string]any{"metrics": []string{"requests"}, "range": map[string]any{"period": c.period}, "dimensions": []string{c.grain}})
		rows := res.body["rows"].([]any)
		seen, sum := map[string]bool{}, 0.0
		for _, r := range rows {
			m := r.(map[string]any)
			label := m["dimensions"].([]any)[0].(string)
			if seen[label] {
				t.Fatalf("%s: %s twice", c.grain, label)
			}
			seen[label] = true
			sum += m["metrics"].(map[string]any)["requests"].(float64)
		}
		first, last := rows[0].(map[string]any)["dimensions"].([]any)[0], rows[len(rows)-1].(map[string]any)["dimensions"].([]any)[0]
		if len(rows) != c.buckets || sum != 30 || totals(res)["requests"].(float64) != 30 || first != c.first || last != c.last {
			t.Fatalf("%s %s: %d rows from %v to %v, %v requests in rows, totals %v", c.grain, c.period, len(rows), first, last, sum, totals(res))
		}
	}

	behind := createAPIProperty(tm.owner, map[string]any{"name": "Behind", "timezone": "Europe/Istanbul"})
	tm.owner.expect(http.StatusOK, "PATCH", "/v1/analytics/properties/"+behind, map[string]any{"timezone": "UTC"})
	for _, dims := range [][]string{{"time:hour"}, {"time:hour", "route"}} {
		res := apiQuery(tm.owner, behind, map[string]any{"metrics": []string{"requests"}, "range": map[string]any{"period": "today"}, "dimensions": dims})
		var prev time.Time
		rows := res.body["rows"].([]any)
		for _, r := range rows {
			label := r.(map[string]any)["dimensions"].([]any)[0].(string)
			at, err := time.Parse("2006-01-02T15:04-07:00", label)
			if err != nil || !at.After(prev) {
				t.Fatalf("%v: %s after %v is out of order: %s", dims, label, prev, res.raw)
			}
			prev = at
		}
		if len(dims) == 1 && (len(rows) != 27 || rows[26].(map[string]any)["dimensions"].([]any)[0] != "2026-10-12T23:00+00:00") {
			t.Fatalf("the Istanbul day that ends in UTC has %d hours: %s", len(rows), res.raw)
		}
	}
}

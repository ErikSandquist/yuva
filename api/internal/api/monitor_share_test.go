package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/api"
	"github.com/productdevbook/yuva/api/internal/monitor"
)

var monitorSampleDay = time.Date(2026, 10, 12, 12, 0, 20, 0, time.UTC)

// monitorSample sends seven requests of 10:01 to 10:07 on monitorSampleDay: acme five of them,
// globex one, and one without a consumer.
func monitorSample(c *client, id string) {
	c.h.t.Helper()
	rec := func(m int, method, route string, status int, consumer string) map[string]any {
		r := map[string]any{"method": method, "route": route, "status": status, "duration_us": 1000 + 300*m,
			"time": time.Date(2026, 10, 12, 10, m, 0, 0, time.UTC).Format(time.RFC3339)}
		if consumer != "" {
			r["consumer"] = consumer
		}
		return r
	}
	sendRecords(c, http.StatusAccepted, id, rec(1, "GET", "/users/{id}", 200, "acme"), rec(2, "GET", "/users/{id}", 200, "acme"),
		rec(3, "GET", "/users/{id}", 200, "acme"), rec(4, "GET", "/users/{id}", 500, "globex"), rec(5, "POST", "/orders", 201, "acme"),
		rec(6, "POST", "/orders", 201, "acme"), rec(7, "POST", "/orders", 404, ""))
}

func sampleQuery(dims []string, filters ...map[string]any) map[string]any {
	q := map[string]any{"dataset": "api", "metrics": []string{"requests", "errors", "p95"}, "range": dayRange("2026-10-12")}
	if len(dims) > 0 {
		q["dimensions"] = dims
	}
	if len(filters) > 0 {
		q["filters"] = filters
	}
	return q
}

func TestMonitorShareShowsOnlyItsViews(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	h.clock.Set(monitorSampleDay)
	id := createAPIProperty(tm.owner, map[string]any{"name": "Shop API", "timezone": "UTC"})
	site, _ := createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"example.com"}})
	foreign := createAPIProperty(other.owner, map[string]any{"name": "Foreign API", "timezone": "UTC"})
	monitorSample(tm.apiKey(h), id)
	monitorSample(other.apiKey(h), foreign)
	monitorSample(other.apiKey(h), foreign)
	h.flushMonitor()

	shares := "/v1/analytics/properties/" + id + "/shares"
	for _, views := range [][]string{{"overview"}, {"api_overview", "pages"}} {
		tm.owner.expectProblem(http.StatusUnprocessableEntity, "property_kind", "POST", shares, map[string]any{"name": "Mixed", "views": views})
	}
	tm.owner.expectProblem(http.StatusUnprocessableEntity, "property_kind", "POST", "/v1/analytics/properties/"+site+"/shares",
		map[string]any{"name": "Site as API", "views": []string{"api_overview"}})
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", shares, map[string]any{"name": "Theirs", "views": []string{"api_overview"}})

	created := tm.owner.expect(http.StatusCreated, "POST", shares, map[string]any{"name": "Status", "views": []string{"api_overview", "api_routes"}, "password": "correct horse"})
	token, shareID := created.str("token"), created.body["share"].(map[string]any)["id"].(string)
	base := "/client/v1/analytics-shares/" + token
	viewer := h.client()
	viewer.expectProblem(http.StatusUnauthorized, "share_password_required", "POST", base+"/query", sampleQuery(nil))
	viewer.bearer = viewer.expect(http.StatusOK, "POST", base+"/access", map[string]any{"password": "correct horse"}).str("access_token")
	page := viewer.expect(http.StatusOK, "GET", base, nil)
	if page.body["property"].(map[string]any)["kind"] != "api" || fmt.Sprint(page.body["views"]) != "[api_overview api_routes]" || page.body["geoip"] == nil {
		t.Fatalf("page %s", page.raw)
	}

	allowed := []map[string]any{sampleQuery(nil), sampleQuery([]string{"time:hour"}), sampleQuery([]string{"status_class"}), sampleQuery([]string{"route"}),
		sampleQuery([]string{"route", "method"}, map[string]any{"dimension": "status_class", "op": "is", "values": []string{"2xx"}})}
	for _, q := range allowed {
		shared := viewer.expect(http.StatusOK, "POST", base+"/query", q)
		if member := apiQuery(tm.owner, id, q); numbers(shared) != numbers(member) {
			t.Fatalf("shared %s\nmember %s", numbers(shared), numbers(member))
		}
	}
	if r := viewer.expect(http.StatusOK, "POST", base+"/query", sampleQuery(nil)); totals(r)["requests"].(float64) != 7 {
		t.Fatalf("a share counts another workspace's requests: %s", r.raw)
	}
	byConsumer := sampleQuery([]string{"consumer"})
	consumerFilter := sampleQuery(nil, map[string]any{"dimension": "consumer", "op": "is", "values": []string{"acme"}})
	refused := []map[string]any{byConsumer, consumerFilter, sampleQuery([]string{"route", "consumer"}),
		{"dataset": "uptime", "metrics": []string{"uptime"}, "range": dayRange("2026-10-12")},
		{"dataset": "api", "metrics": []string{"conversations"}, "range": dayRange("2026-10-12")},
		{"dataset": "support", "metrics": []string{"conversations"}, "range": dayRange("2026-10-12")}}
	for _, q := range refused {
		viewer.expectProblem(http.StatusForbidden, "view_not_shared", "POST", base+"/query", q)
	}
	viewer.expectProblem(http.StatusForbidden, "view_not_shared", "GET", base+"/realtime", nil)

	tm.owner.expect(http.StatusOK, "PATCH", shares+"/"+shareID, map[string]any{"views": []string{"api_overview"}})
	viewer.expectProblem(http.StatusForbidden, "view_not_shared", "POST", base+"/query", sampleQuery([]string{"route"}))
	newRoutes := sampleQuery([]string{"route"})
	newRoutes["routes"] = "new"
	viewer.expectProblem(http.StatusForbidden, "view_not_shared", "POST", base+"/query", newRoutes)
	tm.owner.expectProblem(http.StatusUnprocessableEntity, "property_kind", "PATCH", shares+"/"+shareID, map[string]any{"views": []string{"overview"}})

	tm.owner.expect(http.StatusOK, "PATCH", shares+"/"+shareID, map[string]any{"views": []string{"api_consumers"}})
	shared := viewer.expect(http.StatusOK, "POST", base+"/query", byConsumer)
	if got := rowsBy(shared); len(got) != 2 || got["acme"]["requests"].(float64) != 5 || got["globex"]["requests"].(float64) != 1 {
		t.Fatalf("consumers %s", shared.raw)
	}
	viewer.expect(http.StatusOK, "POST", base+"/query", consumerFilter)
	viewer.expectProblem(http.StatusForbidden, "view_not_shared", "POST", base+"/query", sampleQuery([]string{"status_class"}))

	for _, c := range []struct{ method, path string }{
		{"GET", shares}, {"PATCH", shares + "/" + shareID}, {"POST", shares + "/" + shareID + "/revoke"}, {"DELETE", shares + "/" + shareID},
	} {
		var body any
		if c.method == "PATCH" {
			body = map[string]any{"views": []string{"api_overview", "api_routes", "api_consumers"}}
		}
		other.owner.expectProblem(http.StatusNotFound, "not_found", c.method, c.path, body)
	}

	tm.owner.expect(http.StatusOK, "PATCH", shares+"/"+shareID, map[string]any{"password": "battery staple"})
	viewer.expectProblem(http.StatusUnauthorized, "share_password_required", "POST", base+"/query", sampleQuery(nil))
	viewer.bearer = viewer.expect(http.StatusOK, "POST", base+"/access", map[string]any{"password": "battery staple"}).str("access_token")
	viewer.expect(http.StatusOK, "POST", base+"/query", byConsumer)
	tm.owner.expect(http.StatusOK, "POST", shares+"/"+shareID+"/revoke", nil)
	viewer.expectProblem(http.StatusNotFound, "not_found", "GET", base, nil)
	viewer.expectProblem(http.StatusNotFound, "not_found", "POST", base+"/query", byConsumer)

	open := tm.owner.expect(http.StatusCreated, "POST", shares, map[string]any{"name": "Week", "views": []string{"api_overview"},
		"expires_at": h.clock.Now().Add(time.Hour).Format(time.RFC3339)}).str("token")
	anyone := h.client()
	if r := anyone.expect(http.StatusOK, "POST", "/client/v1/analytics-shares/"+open+"/query", sampleQuery(nil)); totals(r)["requests"].(float64) != 7 {
		t.Fatalf("open link %s", r.raw)
	}
	theirs := other.owner.expect(http.StatusCreated, "POST", "/v1/analytics/properties/"+foreign+"/shares", map[string]any{"name": "Theirs", "views": []string{"api_overview"}}).str("token")
	if r := anyone.expect(http.StatusOK, "POST", "/client/v1/analytics-shares/"+theirs+"/query", sampleQuery(nil)); totals(r)["requests"].(float64) != 14 {
		t.Fatalf("another workspace's link %s", r.raw)
	}
	h.clock.Advance(2 * time.Hour)
	anyone.expectProblem(http.StatusNotFound, "not_found", "POST", "/client/v1/analytics-shares/"+open+"/query", sampleQuery(nil))
}

// csvQuery is queryAnalytics's CSV answer.
func csvQuery(c *client, id string, q map[string]any) []byte {
	c.h.t.Helper()
	return c.expect(http.StatusOK, "POST", "/v1/analytics/properties/"+id+"/query?format=csv", q).raw
}

// ndjsonOf is a query answer's rows as an NDJSON export writes them, each normalised.
func ndjsonOf(t *testing.T, r response, dims []string) []string {
	t.Helper()
	var out []string
	for _, row := range r.body["rows"].([]any) {
		m := row.(map[string]any)
		line := map[string]any{}
		for k, v := range m["metrics"].(map[string]any) {
			line[k] = v
		}
		for i, d := range dims {
			line[d] = m["dimensions"].([]any)[i]
		}
		out = append(out, string(mustJSONTest(line)))
	}
	return out
}

func ndjsonLines(t *testing.T, body []byte) []string {
	t.Helper()
	var out []string
	for line := range bytes.SplitSeq(bytes.TrimSpace(body), []byte("\n")) {
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("%v: %s", err, line)
		}
		out = append(out, string(mustJSONTest(m)))
	}
	return out
}

func TestMonitorExportsEqualTheQuery(t *testing.T) {
	h, _ := monitorHarness(t, true, nil)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	h.clock.Set(monitorSampleDay)
	id := createAPIProperty(tm.owner, map[string]any{"name": "Shop API", "timezone": "Europe/Istanbul"})
	monitorSample(tm.apiKey(h), id)
	h.flushMonitor()
	path := "/v1/analytics/properties/" + id + "/exports"
	export := func(body map[string]any) (map[string]any, []byte) {
		t.Helper()
		ex := tm.owner.expect(http.StatusAccepted, "POST", path, body).body
		runExport(t, h, tm.ws, ex["id"].(string))
		status, _, file := download(tm.owner, path+"/"+ex["id"].(string)+"/download?workspace_id="+tm.ws)
		if status != http.StatusOK {
			t.Fatalf("download %d %s", status, file)
		}
		return ex, file
	}

	tm.owner.expectProblem(http.StatusUnprocessableEntity, "property_kind", "POST", path, map[string]any{"kind": "raw", "range": dayRange("2026-10-12")})
	tm.owner.expectProblem(http.StatusUnprocessableEntity, "property_kind", "POST", path, map[string]any{"kind": "daily", "dataset": "traffic", "range": dayRange("2026-10-12")})
	tm.owner.expectProblem(http.StatusUnprocessableEntity, "property_kind", "POST", path, map[string]any{"kind": "view",
		"query": map[string]any{"metrics": []string{"visitors"}, "range": dayRange("2026-10-12"), "dataset": "traffic"}})
	tm.owner.expectProblem(http.StatusUnprocessableEntity, "dimension_combination", "POST", path, map[string]any{"kind": "view",
		"query": sampleQuery([]string{"consumer", "status"})})
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", path, map[string]any{"kind": "daily", "dimension": "page", "range": dayRange("2026-10-12")})
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", path, map[string]any{"kind": "view", "query": sampleQuery(nil)})

	view := sampleQuery([]string{"route", "method"}, map[string]any{"dimension": "status_class", "op": "is_not", "values": []string{"4xx"}})
	view["metrics"] = []string{"requests", "errors", "error_rate", "p50", "p95", "apdex", "response_bytes"}
	ex, file := export(map[string]any{"kind": "view", "query": view})
	if want := csvQuery(tm.owner, id, view); !bytes.Equal(file, want) || ex["dataset"] != "api" {
		t.Fatalf("view CSV export\n%s\nquery\n%s", file, want)
	}
	_, file = export(map[string]any{"kind": "view", "format": "ndjson", "query": view})
	if got, want := ndjsonLines(t, file), ndjsonOf(t, apiQuery(tm.owner, id, view), []string{"route", "method"}); fmt.Sprint(got) != fmt.Sprint(want) || len(got) != 2 {
		t.Fatalf("view NDJSON export %v, query %v", got, want)
	}

	daily := map[string]any{"dataset": "api", "metrics": []string{"requests", "errors", "error_rate", "client_errors", "client_error_rate", "p50", "p95", "p99",
		"avg_duration", "apdex", "request_bytes", "response_bytes"}, "dimensions": []string{"time:day", "consumer"}, "range": map[string]any{"from": "2026-10-11", "to": "2026-10-12"}}
	ex, file = export(map[string]any{"kind": "daily", "dimension": "consumer", "range": map[string]any{"from": "2026-10-11", "to": "2026-10-12"}})
	if want := csvQuery(tm.owner, id, daily); !bytes.Equal(file, want) || !bytes.Contains(file, []byte(`"2026-10-12","acme","5",`)) || ex["from"] != "2026-10-11" {
		t.Fatalf("daily CSV export\n%s\nquery\n%s", file, want)
	}

	hours := map[string]any{"dataset": "api", "metrics": []string{"requests"}, "dimensions": []string{"time:hour"}, "range": map[string]any{"period": "24h"}}
	ex, file = export(map[string]any{"kind": "view", "query": hours})
	fixed := map[string]any{"dataset": "api", "metrics": []string{"requests"}, "dimensions": []string{"time:hour"},
		"range": map[string]any{"since": "2026-10-11T12:01:00Z", "until": "2026-10-12T12:01:00Z"}}
	if want := csvQuery(tm.owner, id, fixed); !bytes.Equal(file, want) || ex["from"] != "2026-10-11" || ex["to"] != "2026-10-12" {
		t.Fatalf("24h export %v\n%s\nquery\n%s", ex, file, want)
	}

	tg := newTarget(t)
	check := createCheck(tm.owner, map[string]any{"name": "Health", "url": tg.url("a.test"), "property_id": id})["id"].(string)
	t0 := time.Date(2026, 10, 12, 12, 1, 5, 0, time.UTC)
	for m, status := range []int{200, 200, 503, 200} {
		tg.status.Store(int32(status))
		h.runCheck(tm.ws, check, t0.Add(time.Duration(m)*time.Minute))
	}
	h.settleChecks(t0.Add(10 * time.Minute))
	uptime := map[string]any{"dataset": "uptime", "metrics": []string{"runs", "failures", "uptime", "downtime", "response_time_p50", "response_time_p95",
		"response_time_p99", "avg_response_time"}, "dimensions": []string{"time:day", "check"}, "range": dayRange("2026-10-12")}
	ex, file = export(map[string]any{"kind": "daily", "dataset": "uptime", "dimension": "check", "range": dayRange("2026-10-12")})
	if want := csvQuery(tm.owner, id, uptime); !bytes.Equal(file, want) || !bytes.Contains(file, []byte(`"2026-10-12","`+check+`","4","1",`)) || ex["dataset"] != "uptime" {
		t.Fatalf("uptime daily export\n%s\nquery\n%s", file, want)
	}
	minutes := map[string]any{"dataset": "uptime", "metrics": []string{"runs", "failures"}, "dimensions": []string{"time:minute"},
		"range": map[string]any{"since": "2026-10-12T12:00:00Z", "until": "2026-10-12T12:10:00Z"}}
	_, file = export(map[string]any{"kind": "view", "format": "ndjson", "query": minutes})
	if got, want := ndjsonLines(t, file), ndjsonOf(t, query(tm.owner, id, minutes), []string{"time:minute"}); fmt.Sprint(got) != fmt.Sprint(want) || len(got) != 10 {
		t.Fatalf("uptime NDJSON export %v, query %v", got, want)
	}

	list := tm.owner.expect(http.StatusOK, "GET", path, nil).body["items"].([]any)
	if len(list) != 6 {
		t.Fatalf("%d exports", len(list))
	}
	first := list[0].(map[string]any)["id"].(string)
	if status, _, _ := download(other.owner, path+"/"+first+"/download?workspace_id="+other.ws); status != http.StatusNotFound {
		t.Fatalf("another workspace downloads with %d", status)
	}
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", path, nil)
}

func TestMonitorConsumerErasure(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	h.clock.Set(time.Date(2026, 10, 13, 12, 0, 20, 0, time.UTC))
	id := createAPIProperty(tm.owner, map[string]any{"name": "Shop API", "timezone": "UTC"})
	backend := tm.apiKey(h)
	monitorSample(backend, id)
	h.flushMonitor()
	ctx := context.Background()
	ws, prop := uuid.MustParse(tm.ws), uuid.MustParse(id)
	if err := h.srv.RollupMonitor(ctx, ws, prop); err != nil {
		t.Fatal(err)
	}
	rec := func(consumer string) map[string]any {
		return map[string]any{"method": "GET", "route": "/users/{id}", "status": 200, "duration_us": 900, "consumer": consumer,
			"time": h.clock.Now().Format(time.RFC3339)}
	}
	sendRecords(backend, http.StatusAccepted, id, rec("acme"), rec("globex"))
	h.flushMonitor()
	sendRecords(backend, http.StatusAccepted, id, rec("acme"))

	if n := h.count("SELECT count(*) FROM monitor_dim_day WHERE workspace_id = $1 AND property_id = $2 AND dim = 1", tm.ws, id); n == 0 {
		t.Fatal("the sample's day was not folded")
	}
	twoDays := map[string]any{"from": "2026-10-12", "to": "2026-10-13"}
	q := func(dims []string, filters ...map[string]any) map[string]any {
		out := sampleQuery(dims, filters...)
		out["range"] = twoDays
		return out
	}
	before := apiQuery(tm.owner, id, q(nil))
	beforeConsumers := rowsBy(apiQuery(tm.owner, id, q([]string{"consumer"})))
	if totals(before)["requests"].(float64) != 9 || beforeConsumers["acme"]["requests"].(float64) != 6 {
		t.Fatalf("before %s %v", before.raw, beforeConsumers)
	}

	consumers := "/v1/analytics/properties/" + id + "/consumers"
	page := tm.owner.expect(http.StatusOK, "GET", consumers+"?limit=1", nil)
	items := page.body["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["name"] != "acme" || page.str("next_cursor") == "" {
		t.Fatalf("first page %s", page.raw)
	}
	acme := int(items[0].(map[string]any)["id"].(float64))
	rest := tm.owner.expect(http.StatusOK, "GET", consumers+"?limit=1&cursor="+page.str("next_cursor"), nil)
	if items := rest.body["items"].([]any); len(items) != 1 || items[0].(map[string]any)["name"] != "globex" || rest.body["next_cursor"] != nil {
		t.Fatalf("second page %s", rest.raw)
	}
	if items := tm.owner.expect(http.StatusOK, "GET", consumers+"?q=GLOB", nil).body["items"].([]any); len(items) != 1 {
		t.Fatalf("search %v", items)
	}
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", consumers, nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "DELETE", fmt.Sprint(consumers, "/", acme), nil)
	var otherID int
	if err := h.st.Pool.QueryRow(ctx, "SELECT id FROM monitor_names WHERE workspace_id = $1 AND property_id = $2 AND kind = 'consumer' AND value = '(other)'",
		ws, prop).Scan(&otherID); err != nil {
		t.Fatal(err)
	}
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "DELETE", fmt.Sprint(consumers, "/", otherID), nil)
	site, _ := createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"example.com"}})
	tm.owner.expectProblem(http.StatusUnprocessableEntity, "property_kind", "GET", "/v1/analytics/properties/"+site+"/consumers", nil)

	tm.owner.expect(http.StatusNoContent, "DELETE", fmt.Sprint(consumers, "/", acme), nil)
	tm.owner.expectProblem(http.StatusNotFound, "not_found", "DELETE", fmt.Sprint(consumers, "/", acme), nil)
	h.flushMonitor()
	left := func() int {
		return h.count("SELECT count(*) FROM monitor_dim_hour WHERE workspace_id = $1 AND property_id = $2 AND dim = 1 AND value = $3", tm.ws, id, acme) +
			h.count("SELECT count(*) FROM monitor_dim_day WHERE workspace_id = $1 AND property_id = $2 AND dim = 1 AND value = $3", tm.ws, id, acme)
	}
	if n := left(); n != 0 {
		t.Fatalf("%d rows of the erased consumer, the unwritten record included", n)
	}
	if n := h.count("SELECT count(*) FROM monitor_names WHERE workspace_id = $1 AND property_id = $2 AND value = 'acme'", tm.ws, id); n != 0 {
		t.Fatal("the erased name is kept")
	}
	after := apiQuery(tm.owner, id, q(nil))
	afterConsumers := rowsBy(apiQuery(tm.owner, id, q([]string{"consumer"})))
	if totals(after)["requests"].(float64) != 10 || len(afterConsumers) != 1 || fmt.Sprint(afterConsumers["globex"]) != fmt.Sprint(beforeConsumers["globex"]) {
		t.Fatalf("after %s %v", after.raw, afterConsumers)
	}
	if r := apiQuery(tm.owner, id, q(nil, map[string]any{"dimension": "consumer", "op": "is", "values": []string{"acme"}})); totals(r)["requests"].(float64) != 0 {
		t.Fatalf("the erased consumer still filters: %s", r.raw)
	}
	if items := tm.owner.expect(http.StatusOK, "GET", consumers, nil).body["items"].([]any); len(items) != 1 {
		t.Fatalf("listed after erasure %v", items)
	}
	if n := len(h.jobs("monitor_erase", tm.ws)); n != 2 {
		t.Fatalf("%d sweeps queued", n)
	}

	if _, err := h.st.Pool.Exec(ctx, `INSERT INTO monitor_dim_hour (workspace_id, property_id, hour, writer, dim, value, route, method, status_class, requests, errors,
		client_errors, sampled, duration_sum, duration_min, duration_max, durations, request_bytes, response_bytes, response_sizes)
		VALUES ($1, $2, '2026-10-13T12:00:00Z', 99, 1, $3, 1, 0, 2, 1, 0, 0, 0, 1, 1, 1, '\x', 0, 0, '\x')`, ws, prop, acme); err != nil {
		t.Fatal(err)
	}
	if err := h.srv.SweepErasedConsumer(ctx, ws, prop, int32(acme)); err != nil {
		t.Fatal(err)
	}
	if n := left(); n != 0 {
		t.Fatal("the sweep left a row written after the erasure")
	}

	sendRecords(backend, http.StatusAccepted, id, rec("acme"))
	h.flushMonitor()
	var again int
	if err := h.st.Pool.QueryRow(ctx, "SELECT id FROM monitor_names WHERE workspace_id = $1 AND property_id = $2 AND kind = 'consumer' AND value = 'acme'",
		ws, prop).Scan(&again); err != nil || again == acme {
		t.Fatalf("a consumer named again gets id %d (erased %d): %v", again, acme, err)
	}
	if got := rowsBy(apiQuery(tm.owner, id, q([]string{"consumer"}))); got["acme"]["requests"].(float64) != 1 {
		t.Fatalf("the new acme %v", got)
	}
}

func TestMonitorRetentionDropsEachTierPastItsDays(t *testing.T) {
	h, _ := monitorHarness(t, true, nil)
	tm := newTeam(t, h)
	h.clock.Set(time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC))
	id := createAPIProperty(tm.owner, map[string]any{"name": "Kept", "timezone": "UTC", "monitor_hour_days": 7, "retention_months": 1})
	tg := newTarget(t)
	check := createCheck(tm.owner, map[string]any{"name": "Health", "url": tg.url("a.test")})["id"].(string)
	ctx := context.Background()
	ws, prop, chk := uuid.MustParse(tm.ws), uuid.MustParse(id), uuid.MustParse(check)
	st := &monitor.PGStore{Pool: h.st.Pool}
	day := func(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }
	minuteGone, minuteKept := day("2026-10-04"), day("2026-10-05")
	hourGone, hourKept := day("2026-10-04").Add(23*time.Hour), day("2026-10-05")
	dayGone, dayKept := day("2026-09-11"), day("2026-09-12")
	resultGone, resultKept := day("2026-09-12"), day("2026-09-13")
	checkDayGone, checkDayKept := day("2025-09-06"), day("2025-09-07")
	if err := st.EnsureMinutePartitions(ctx, minuteGone, minuteKept); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureHourPartitions(ctx, dayGone, hourKept); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureCheckPartitions(ctx, resultGone, resultKept); err != nil {
		t.Fatal(err)
	}
	const counters = `1, 0, 0, 0, 1, 1, 1, '\x', 0, 0, '\x'`
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := h.st.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, at := range []time.Time{minuteGone.Add(23*time.Hour + 59*time.Minute), minuteKept} {
		exec(`INSERT INTO monitor_minute (workspace_id, property_id, minute, writer, environment, service, route, method, status, api_version, source, platform,
			requests, errors, client_errors, sampled, duration_sum, duration_min, duration_max, durations, request_bytes, response_bytes, response_sizes)
			VALUES ($1, $2, $3, 1, 0, 0, 1, 0, 200, 0, 0, 0, `+counters+`)`, ws, prop, at)
	}
	for _, at := range []time.Time{hourGone, hourKept} {
		exec(`INSERT INTO monitor_hour (workspace_id, property_id, hour, environment, service, route, method, status, api_version, source, platform,
			requests, errors, client_errors, sampled, duration_sum, duration_min, duration_max, durations, request_bytes, response_bytes, response_sizes)
			VALUES ($1, $2, $3, 0, 0, 1, 0, 200, 0, 0, 0, `+counters+`)`, ws, prop, at)
		exec(`INSERT INTO monitor_dim_hour (workspace_id, property_id, hour, writer, dim, value, route, method, status_class,
			requests, errors, client_errors, sampled, duration_sum, duration_min, duration_max, durations, request_bytes, response_bytes, response_sizes)
			VALUES ($1, $2, $3, 0, 1, 1, 1, 0, 2, `+counters+`)`, ws, prop, at)
	}
	for _, d := range []time.Time{dayGone, dayKept} {
		exec(`INSERT INTO monitor_day (workspace_id, property_id, day, environment, service, route, method, status, api_version, source, platform,
			requests, errors, client_errors, sampled, duration_sum, duration_min, duration_max, durations, request_bytes, response_bytes, response_sizes)
			VALUES ($1, $2, $3, 0, 0, 1, 0, 200, 0, 0, 0, `+counters+`)`, ws, prop, d)
		exec(`INSERT INTO monitor_dim_day (workspace_id, property_id, day, dim, value, route, method, status_class,
			requests, errors, client_errors, sampled, duration_sum, duration_min, duration_max, durations, request_bytes, response_bytes, response_sizes)
			VALUES ($1, $2, $3, 1, 1, 1, 0, 2, `+counters+`)`, ws, prop, d)
		exec(`INSERT INTO monitor_rollup_hours (workspace_id, property_id, hour, day, built_at, day_built_at) VALUES ($1, $2, $3, $4, $3, $3)`, ws, prop, d, d)
	}
	for _, at := range []time.Time{resultGone.Add(12 * time.Hour), resultKept.Add(12 * time.Hour)} {
		exec(`INSERT INTO monitor_check_results (workspace_id, check_id, at, ok, state, status, retried, redirects, dns_us, connect_us, tls_us, ttfb_us,
			total_us, answered, seconds) VALUES ($1, $2, $3, true, 'up', 200, false, 0, 1, 1, 1, 1, 1, true, 60)`, ws, chk, at)
	}
	for _, d := range []time.Time{checkDayGone, checkDayKept} {
		exec(`INSERT INTO monitor_check_days (workspace_id, check_id, day, runs, failures, unknown, monitored_seconds, down_seconds, answered, total_sum_us,
			ttfb_sum_us, durations) VALUES ($1, $2, $3, 1, 0, 0, 60, 0, 1, 1, 1, '\x')`, ws, chk, d)
	}
	exec("UPDATE monitor_checks SET folded_until = $3 WHERE workspace_id = $1 AND id = $2", ws, chk, day("2026-10-11"))

	if err := h.srv.MaintainAnalytics(ctx); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		table, col string
		gone, kept time.Time
	}{
		{"monitor_minute", "minute", minuteGone.Add(23*time.Hour + 59*time.Minute), minuteKept},
		{"monitor_hour", "hour", hourGone, hourKept},
		{"monitor_dim_hour", "hour", hourGone, hourKept},
		{"monitor_day", "day", dayGone, dayKept},
		{"monitor_dim_day", "day", dayGone, dayKept},
		{"monitor_rollup_hours", "hour", dayGone, dayKept},
	} {
		sql := "SELECT count(*) FROM " + c.table + " WHERE workspace_id = $1 AND property_id = $2 AND " + c.col + " = $3"
		if h.count(sql, ws, prop, c.gone) != 0 || h.count(sql, ws, prop, c.kept) != 1 {
			t.Fatalf("%s: %d past its retention, %d inside it", c.table, h.count(sql, ws, prop, c.gone), h.count(sql, ws, prop, c.kept))
		}
	}
	results := "SELECT count(*) FROM monitor_check_results WHERE check_id = $1 AND at::date = $2"
	if h.count(results, chk, resultGone) != 0 || h.count(results, chk, resultKept) != 1 {
		t.Fatalf("check results: %d past 30 days, %d inside", h.count(results, chk, resultGone), h.count(results, chk, resultKept))
	}
	checkDays := "SELECT count(*) FROM monitor_check_days WHERE check_id = $1 AND day = $2"
	if h.count(checkDays, chk, checkDayGone) != 0 || h.count(checkDays, chk, checkDayKept) != 1 {
		t.Fatalf("check days: %d past 400 days, %d inside", h.count(checkDays, chk, checkDayGone), h.count(checkDays, chk, checkDayKept))
	}
}

func TestMonitorDeletePropertyTakesEveryTier(t *testing.T) {
	h, _ := monitorHarness(t, true, nil)
	tm := newTeam(t, h)
	h.clock.Set(monitorSampleDay)
	id := createAPIProperty(tm.owner, map[string]any{"name": "API", "timezone": "UTC"})
	keep := createAPIProperty(tm.owner, map[string]any{"name": "Other API", "timezone": "UTC"})
	backend := tm.apiKey(h)
	for _, p := range []string{id, keep} {
		monitorSample(backend, p)
		sendRecords(backend, http.StatusAccepted, p, map[string]any{"method": "GET", "route": "/", "status": 200, "duration_us": 1, "service": "web",
			"release": "v1", "consumer": "acme", "time": time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC).Format(time.RFC3339)})
	}
	h.flushMonitor()
	ctx := context.Background()
	ws := uuid.MustParse(tm.ws)
	for _, p := range []string{id, keep} {
		if err := h.srv.RollupMonitor(ctx, ws, uuid.MustParse(p)); err != nil {
			t.Fatal(err)
		}
	}
	tg := newTarget(t)
	check := createCheck(tm.owner, map[string]any{"name": "Health", "url": tg.url("a.test"), "property_id": id})["id"].(string)
	h.runCheck(tm.ws, check, monitorSampleDay.Add(time.Minute))
	tm.owner.expect(http.StatusCreated, "POST", "/v1/analytics/properties/"+id+"/shares", map[string]any{"name": "Status", "views": []string{"api_overview"}})
	tables := []string{"monitor_minute", "monitor_hour", "monitor_dim_hour", "monitor_day", "monitor_dim_day", "monitor_rollup_hours", "monitor_routes",
		"monitor_names", "monitor_ingest_stats", "monitor_releases", "monitor_alert_rules", "analytics_annotations", "analytics_shares"}
	for _, table := range tables {
		if h.count("SELECT count(*) FROM "+table+" WHERE workspace_id = $1 AND property_id = $2", tm.ws, id) == 0 {
			t.Fatalf("the test wrote no %s rows", table)
		}
	}
	kept := map[string]int{}
	for _, table := range tables {
		kept[table] = h.count("SELECT count(*) FROM "+table+" WHERE workspace_id = $1 AND property_id = $2", tm.ws, keep)
	}

	tm.owner.expect(http.StatusAccepted, "DELETE", "/v1/analytics/properties/"+id, map[string]any{"name": "API"})
	done, err := h.srv.DeleteAnalytics(ctx, api.AnalyticsDeleteArgs{WorkspaceID: ws, PropertyID: uuid.MustParse(id), Property: true}, time.Minute)
	if err != nil || !done {
		t.Fatalf("delete %v %v", done, err)
	}
	for _, table := range tables {
		if n := h.count("SELECT count(*) FROM "+table+" WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 0 {
			t.Fatalf("%s keeps %d rows of the deleted property", table, n)
		}
		if n := h.count("SELECT count(*) FROM "+table+" WHERE workspace_id = $1 AND property_id = $2", tm.ws, keep); n != kept[table] {
			t.Fatalf("%s lost rows of another property: %d of %d", table, n, kept[table])
		}
	}
	if h.count("SELECT count(*) FROM monitor_checks WHERE id = $1 AND property_id IS NULL", check) != 1 ||
		h.count("SELECT count(*) FROM monitor_check_results WHERE check_id = $1", check) == 0 {
		t.Fatal("a check of the workspace went with the property")
	}
	if !strings.Contains(fmt.Sprint(tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/checks/"+check, nil).body), "Health") {
		t.Fatal("the check is gone")
	}
}

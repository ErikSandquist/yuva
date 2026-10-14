package api_test

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"
)

// fx is one request of the analysis fixture, as sent.
type fx struct {
	at                                              time.Time
	method, route                                   string
	status                                          int
	durUS, reqSize, respSize                        int64
	consumer, ip, apiVersion, env, service, release string
	source, platform, appVersion                    string
	weight                                          int64
}

var fxCountries = map[string]string{"203.0.113.7": "TR", "198.51.100.20": "DE", "2001:db8:1::7": "US"}

var fxCoreDims = []string{"route", "method", "status", "status_class", "api_version", "environment", "service", "source", "platform"}
var fxExtraDims = []string{"consumer", "country", "app_version"}

func (r fx) value(dim string) string {
	switch dim {
	case "route":
		return r.route
	case "method":
		return r.method
	case "status":
		return strconv.Itoa(r.status)
	case "status_class":
		if r.status == 0 {
			return "none"
		}
		return strconv.Itoa(r.status/100) + "xx"
	case "api_version":
		return r.apiVersion
	case "environment":
		if r.env == "" {
			return "production"
		}
		return r.env
	case "service":
		return r.service
	case "source":
		return r.source
	case "platform":
		return r.platform
	case "consumer":
		return r.consumer
	case "country":
		if r.source == "app" {
			return "TR"
		}
		return fxCountries[r.ip]
	case "app_version":
		return r.appVersion
	}
	panic(dim)
}

type weighted struct{ v, w int64 }

// tally is what the test counts by hand of the requests it sent.
type tally struct {
	requests, errors, clientErrors, durSum, maxDur, reqBytes, respBytes int64
	durs, sizes                                                         []weighted
	sat, tol                                                            int64
	conversations                                                       int64
}

func fxApdex(route string) int64 {
	if route == "/export" {
		return 2_000_000
	}
	return 100_000
}

func (t *tally) add(r fx) {
	w := r.weight
	t.requests += w
	switch {
	case r.status == 0 || r.status >= 500:
		t.errors += w
	case r.status >= 400:
		t.clientErrors += w
	}
	t.durSum += r.durUS * w
	t.maxDur = max(t.maxDur, r.durUS)
	t.durs = append(t.durs, weighted{r.durUS, w})
	if r.reqSize > 0 {
		t.reqBytes += r.reqSize * w
	}
	if r.respSize >= 0 {
		t.respBytes += r.respSize * w
		t.sizes = append(t.sizes, weighted{r.respSize, w})
	}
	switch th := fxApdex(r.route); {
	case r.durUS <= th:
		t.sat += w
	case r.durUS <= 4*th:
		t.tol += w
	}
}

func nearestRank(vals []weighted, q float64) float64 {
	vals = slices.Clone(vals)
	slices.SortFunc(vals, func(a, b weighted) int { return int(a.v - b.v) })
	var n int64
	for _, v := range vals {
		n += v.w
	}
	if n == 0 {
		return 0
	}
	rank := min(max(int64(math.Ceil(q*float64(n))), 1), n)
	var seen int64
	for _, v := range vals {
		if seen += v.w; seen >= rank {
			return float64(v.v)
		}
	}
	return float64(vals[len(vals)-1].v)
}

var fxMetrics = []string{"requests", "errors", "error_rate", "client_errors", "client_error_rate", "p50", "p75", "p90", "p95", "p99", "avg_duration",
	"max_duration", "apdex", "request_bytes", "response_bytes", "response_size_p50", "response_size_p95"}

func (t *tally) check(label string, got map[string]any) error {
	pct := func(n int64) float64 {
		if t.requests == 0 {
			return 0
		}
		return float64(n) * 100 / float64(t.requests)
	}
	exact := map[string]float64{"requests": float64(t.requests), "errors": float64(t.errors), "client_errors": float64(t.clientErrors),
		"request_bytes": float64(t.reqBytes), "response_bytes": float64(t.respBytes), "max_duration": float64(t.maxDur) / 1000}
	near := map[string]float64{"error_rate": pct(t.errors), "client_error_rate": pct(t.clientErrors)}
	if t.requests > 0 {
		near["avg_duration"] = float64(t.durSum) / float64(t.requests) / 1000
		near["apdex"] = (float64(t.sat) + float64(t.tol)/2) / float64(t.requests)
	}
	for name, want := range exact {
		if v, ok := got[name]; ok && v.(float64) != want {
			return fmt.Errorf("%s: %s is %v, counted %v", label, name, v, want)
		}
	}
	for name, want := range near {
		if v, ok := got[name]; ok && math.Abs(v.(float64)-want) > 0.006 {
			return fmt.Errorf("%s: %s is %v, counted %v", label, name, v, want)
		}
	}
	for _, p := range []struct {
		name string
		q    float64
	}{{"p50", 0.5}, {"p75", 0.75}, {"p90", 0.9}, {"p95", 0.95}, {"p99", 0.99}} {
		if v, ok := got[p.name]; ok && !within(v.(float64), nearestRank(t.durs, p.q)/1000, 0.011) {
			return fmt.Errorf("%s: %s is %v, exact %v", label, p.name, v, nearestRank(t.durs, p.q)/1000)
		}
	}
	for _, p := range []struct {
		name string
		q    float64
	}{{"response_size_p50", 0.5}, {"response_size_p95", 0.95}} {
		if v, ok := got[p.name]; ok && math.Abs(v.(float64)-nearestRank(t.sizes, p.q)) > 0.09*nearestRank(t.sizes, p.q)+0.5 {
			return fmt.Errorf("%s: %s is %v, exact %v", label, p.name, v, nearestRank(t.sizes, p.q))
		}
	}
	return nil
}

type fxFilter struct {
	dim, op string
	values  []string
}

func (f fxFilter) body() map[string]any {
	return map[string]any{"dimension": f.dim, "op": f.op, "values": f.values}
}

func (f fxFilter) keeps(r fx) bool {
	in := slices.Contains(f.values, r.value(f.dim))
	if f.op == "is_not" {
		return !in
	}
	return in
}

// expected counts the fixture by dims, with the filters, leaving out the requests without a value of
// an extra dimension the query reads.
func expected(recs []fx, dims []string, filters []fxFilter) (map[string]*tally, *tally) {
	used := slices.Clone(dims)
	for _, f := range filters {
		used = append(used, f.dim)
	}
	rows, total := map[string]*tally{}, &tally{}
next:
	for _, r := range recs {
		for _, d := range used {
			if slices.Contains(fxExtraDims, d) && r.value(d) == "" {
				continue next
			}
		}
		for _, f := range filters {
			if !f.keeps(r) {
				continue next
			}
		}
		total.add(r)
		var key []string
		for _, d := range dims {
			key = append(key, r.value(d))
		}
		k := strings.Join(key, "\x1f")
		if rows[k] == nil {
			rows[k] = &tally{}
		}
		rows[k].add(r)
	}
	return rows, total
}

// fxToday is the fixture of 2026-10-12 in Istanbul: 360 requests between 12:00 and 15:00 local
// time, from the records endpoint, OTLP and an app.
func fxToday() []fx {
	day := time.Date(2026, 10, 12, 9, 0, 10, 0, time.UTC)
	routes := []struct{ method, route string }{{"GET", "/users/{id}"}, {"POST", "/orders"}, {"GET", "/export"}, {"GET", "/health"}}
	statuses := []int{200, 200, 201, 304, 404, 500, 0}
	durs := []int64{20_000, 80_000, 150_000, 350_000, 600_000, 1_500_000, 3_000_000, 9_000_000}
	var out []fx
	for i := range 360 {
		rt := routes[i%4]
		r := fx{at: day.Add(time.Duration(i*37%179) * time.Minute), method: rt.method, route: rt.route, status: statuses[(i/4)%7],
			durUS: durs[(i*7)%8] + int64(i%5)*1000, reqSize: -1, respSize: -1, weight: 1}
		if i%3 != 0 {
			r.reqSize = int64(100 + i)
		}
		if i%4 != 0 {
			r.respSize = int64(500 + 37*i)
		}
		switch i % 5 {
		case 0:
			r.source = "app"
			r.platform = []string{"ios", "android"}[(i/5)%2]
			r.appVersion = []string{"2.0", "2.1"}[(i/10)%2]
			r.apiVersion = []string{"v1", "v2", ""}[(i/3)%3]
		case 1:
			r.source = "otlp"
			r.consumer = []string{"acme", "globex", ""}[i%3]
			r.ip = []string{"203.0.113.7", "198.51.100.20", "2001:db8:1::7", ""}[i%4]
			r.apiVersion = []string{"v1", "v2", ""}[(i/3)%3]
			r.service, r.env, r.release = "users", "production", "1.1.0"
		default:
			r.source = "server"
			r.consumer = []string{"acme", "globex", ""}[i%3]
			r.ip = []string{"203.0.113.7", "198.51.100.20", "2001:db8:1::7", ""}[i%4]
			r.apiVersion = []string{"v1", "v2", ""}[(i/3)%3]
			r.env = []string{"", "staging"}[(i/5)%2]
			r.service = []string{"users", "billing"}[(i/2)%2]
			if r.service == "users" {
				r.release = "1.0.0"
				if r.at.After(time.Date(2026, 10, 12, 10, 30, 0, 0, time.UTC)) && i%13 != 0 {
					r.release = "1.1.0"
				}
			}
			if (r.status == 200 || r.status == 201) && i%11 == 0 {
				r.weight = 3
			}
		}
		out = append(out, r)
	}
	return out
}

// fxYesterday: /users/{id} and /legacy, which today never answers, at 13:00 local time.
func fxYesterday() []fx {
	at := time.Date(2026, 10, 11, 10, 0, 10, 0, time.UTC)
	var out []fx
	for i := range 12 {
		route := []string{"/users/{id}", "/legacy"}[i%2]
		out = append(out, fx{at: at.Add(time.Duration(i) * time.Minute), method: "GET", route: route, status: []int{200, 500}[i%3/2], durUS: 40_000,
			reqSize: -1, respSize: 100, weight: 1, source: "server", consumer: "acme", service: "users", release: "0.9.0"})
	}
	return out
}

func serverRecord(r fx) map[string]any {
	m := map[string]any{"method": r.method, "route": r.route, "status": r.status, "duration_us": r.durUS, "time": r.at.Format(time.RFC3339Nano)}
	set := func(k, v string) {
		if v != "" {
			m[k] = v
		}
	}
	set("consumer", r.consumer)
	set("ip", r.ip)
	set("api_version", r.apiVersion)
	set("environment", r.env)
	set("service", r.service)
	set("release", r.release)
	if r.reqSize >= 0 {
		m["request_size"] = r.reqSize
	}
	if r.respSize >= 0 {
		m["response_size"] = r.respSize
	}
	if r.weight > 1 {
		m["sample_rate"] = r.weight
	}
	return m
}

func otlpBody(recs []fx) []byte {
	str := func(k, v string) map[string]any {
		return map[string]any{"key": k, "value": map[string]any{"stringValue": v}}
	}
	num := func(k string, v int64) map[string]any {
		return map[string]any{"key": k, "value": map[string]any{"intValue": v}}
	}
	var spans []any
	for i, r := range recs {
		attrs := []any{str("http.request.method", r.method), str("http.route", r.route), num("http.response.status_code", int64(r.status))}
		if r.status == 0 {
			attrs = attrs[:2]
		}
		for _, a := range []struct{ k, v string }{{"client.address", r.ip}, {"yuva.consumer", r.consumer}, {"yuva.api_version", r.apiVersion}} {
			if a.v != "" {
				attrs = append(attrs, str(a.k, a.v))
			}
		}
		if r.reqSize >= 0 {
			attrs = append(attrs, num("http.request.body.size", r.reqSize))
		}
		if r.respSize >= 0 {
			attrs = append(attrs, num("http.response.body.size", r.respSize))
		}
		start := r.at.UnixNano()
		spans = append(spans, map[string]any{"traceId": fmt.Sprintf("%032x", i+1), "spanId": fmt.Sprintf("%016x", i+1), "name": r.route, "kind": 2,
			"startTimeUnixNano": strconv.FormatInt(start, 10), "endTimeUnixNano": strconv.FormatInt(start+r.durUS*1000, 10), "attributes": attrs})
	}
	b, _ := json.Marshal(map[string]any{"resourceSpans": []any{map[string]any{
		"resource":   map[string]any{"attributes": []any{str("service.name", "users"), str("deployment.environment.name", "production"), str("service.version", "1.1.0")}},
		"scopeSpans": []any{map[string]any{"scope": map[string]any{"name": "fixture"}, "spans": spans}},
	}}})
	return b
}

// sendFixture sends records through their own path: the records endpoint, OTLP or the app.
func sendFixture(t *testing.T, h *harness, bearer, appKey, id string, recs []fx) {
	t.Helper()
	backend := h.client()
	backend.bearer = bearer
	var server []map[string]any
	var spans []fx
	apps := map[[2]string][]map[string]any{}
	for _, r := range recs {
		switch r.source {
		case "server":
			server = append(server, serverRecord(r))
		case "otlp":
			spans = append(spans, r)
		case "app":
			m := map[string]any{"method": r.method, "route": r.route, "status": r.status, "duration_us": r.durUS, "time": r.at.Format(time.RFC3339Nano)}
			if r.apiVersion != "" {
				m["api_version"] = r.apiVersion
			}
			if r.reqSize >= 0 {
				m["request_size"] = r.reqSize
			}
			if r.respSize >= 0 {
				m["response_size"] = r.respSize
			}
			k := [2]string{r.platform, r.appVersion}
			apps[k] = append(apps[k], m)
		}
	}
	if len(server) > 0 {
		if res := sendRecords(backend, http.StatusAccepted, id, server...); len(res.body["errors"].([]any)) != 0 {
			t.Fatalf("records refused: %s", res.raw)
		}
	}
	if len(spans) > 0 {
		if a := exportOTLP(h, bearer, id, "application/json", otlpBody(spans), nil); a.status != http.StatusOK || strings.Contains(string(a.body), "partial") {
			t.Fatalf("OTLP export: %d %s", a.status, a.body)
		}
	}
	for k, list := range apps {
		if r := appRequests(h, appKey, map[string]any{"a": map[string]any{"platform": k[0], "version": k[1]}, "records": list}, nil); r.status != http.StatusAccepted {
			t.Fatalf("app records: %d %s", r.status, r.raw)
		}
	}
}

func rowKeyed(res response) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, row := range res.body["rows"].([]any) {
		m := row.(map[string]any)
		var key []string
		for _, d := range m["dimensions"].([]any) {
			key = append(key, d.(string))
		}
		out[strings.Join(key, "\x1f")] = m["metrics"].(map[string]any)
	}
	return out
}

func TestMonitorAnalysisMatchesTheFixture(t *testing.T) {
	h := geoHarness(t, nil)
	ct := newChatTeam(t, h, "live", true)
	other := newTeam(t, h)
	id, appKey := createProperty(ct.owner, map[string]any{"kind": "api", "name": "Shop API", "domains": []string{}, "timezone": "Europe/Istanbul",
		"platforms": []string{"ios", "android"}, "apdex_ms": 100, "inbox_id": ct.chatInbox})
	backend := ct.apiKey(h)

	h.clock.Set(time.Date(2026, 10, 11, 10, 15, 0, 0, time.UTC))
	yesterday := fxYesterday()
	sendFixture(t, h, backend.bearer, appKey, id, yesterday)
	h.flushMonitor()
	ct.session(h, map[string]any{}).expect(http.StatusCreated, "POST", "/client/v1/conversations", map[string]any{"body": "yesterday", "client_id": unique("c")})

	now := time.Date(2026, 10, 12, 11, 59, 0, 0, time.UTC)
	h.clock.Set(now)
	today := fxToday()
	sendFixture(t, h, backend.bearer, appKey, id, today)
	for range 2 {
		ct.session(h, map[string]any{}).expect(http.StatusCreated, "POST", "/client/v1/conversations", map[string]any{"body": "today", "client_id": unique("c")})
	}
	theirs, _ := createProperty(other.owner, map[string]any{"kind": "api", "name": "Theirs", "domains": []string{}})
	sendRecords(other.apiKey(h), http.StatusAccepted, theirs, map[string]any{"method": "POST", "route": "/orders", "status": 500, "duration_us": 5,
		"service": "users", "release": "1.0.0"})
	h.flushMonitor()

	routes := map[string]string{}
	for _, r := range ct.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/routes", nil).body["items"].([]any) {
		m := r.(map[string]any)
		routes[m["pattern"].(string)] = fmt.Sprint(m["id"])
	}
	export := "/v1/analytics/properties/" + id + "/routes/" + routes["/export"]
	if r := ct.owner.expect(http.StatusOK, "PATCH", export, map[string]any{"apdex_ms": 2000}); r.body["apdex_ms"].(float64) != 2000 || r.body["hidden"] != false {
		t.Fatalf("route apdex %s", r.raw)
	}
	ct.owner.expectProblem(http.StatusBadRequest, "validation_failed", "PATCH", export, map[string]any{"apdex_ms": 0})
	other.owner.expectProblem(http.StatusNotFound, "not_found", "PATCH", export, map[string]any{"apdex_ms": 10})

	day := dayRange("2026-10-12")
	breakdown := func(label string, dims []string, filters []fxFilter) {
		t.Helper()
		body := map[string]any{"metrics": fxMetrics, "range": day, "limit": 1000}
		if len(dims) > 0 {
			body["dimensions"] = dims
		}
		if len(filters) > 0 {
			var fs []map[string]any
			for _, f := range filters {
				fs = append(fs, f.body())
			}
			body["filters"] = fs
		}
		res := apiQuery(ct.owner, id, body)
		want, total := expected(today, dims, filters)
		if err := total.check(label+" totals", totals(res)); err != nil {
			t.Fatalf("%v\n%s", err, res.raw)
		}
		got := rowKeyed(res)
		if len(dims) == 0 {
			return
		}
		if len(got) != len(want) {
			t.Fatalf("%s: %d rows, counted %d: %s", label, len(got), len(want), res.raw)
		}
		for k, w := range want {
			if got[k] == nil {
				t.Fatalf("%s: no row %q: %s", label, k, res.raw)
			}
			if err := w.check(label+" "+k, got[k]); err != nil {
				t.Fatal(err)
			}
		}
	}
	every := func(tier string) {
		t.Helper()
		breakdown(tier, nil, nil)
		for _, d := range slices.Concat(fxCoreDims, fxExtraDims) {
			breakdown(tier+" by "+d, []string{d}, nil)
		}
		for i, a := range fxCoreDims {
			for _, b := range fxCoreDims[i+1:] {
				breakdown(tier+" by "+a+" and "+b, []string{a, b}, nil)
			}
		}
		for _, e := range fxExtraDims {
			for _, c := range []string{"route", "method", "status_class"} {
				breakdown(tier+" by "+e+" and "+c, []string{e, c}, nil)
				breakdown(tier+" by "+c+" and "+e, []string{c, e}, nil)
			}
		}
		for _, d := range slices.Concat(fxCoreDims, fxExtraDims) {
			v := today[7].value(d)
			if v == "" {
				v = today[2].value(d)
			}
			breakdown(tier+" "+d+" is "+v, nil, []fxFilter{{d, "is", []string{v}}})
			breakdown(tier+" "+d+" is not "+v, []string{"route"}, []fxFilter{{d, "is_not", []string{v}}})
		}
		breakdown(tier+" consumers of a route", []string{"consumer"}, []fxFilter{{"route", "is", []string{"/users/{id}"}}})
		breakdown(tier+" staging v2", []string{"service", "source"}, []fxFilter{{"environment", "is", []string{"staging"}}, {"api_version", "is", []string{"v2"}}})
		breakdown(tier+" country of 5xx", []string{"country"}, []fxFilter{{"status_class", "is", []string{"5xx"}}, {"method", "is", []string{"GET"}}})
		res := apiQuery(ct.owner, id, map[string]any{"metrics": []string{"requests", "p95"}, "range": day, "dimensions": []string{"time:hour", "consumer"}})
		byHour := map[string]*tally{}
		for _, r := range today {
			if r.consumer != "" {
				k := r.at.In(time.FixedZone("", 3*3600)).Format("2006-01-02T15:00-07:00") + "\x1f" + r.consumer
				if byHour[k] == nil {
					byHour[k] = &tally{}
				}
				byHour[k].add(r)
			}
		}
		got := rowKeyed(res)
		if len(got) != len(byHour) {
			t.Fatalf("%s: %d hour and consumer rows, counted %d: %s", tier, len(got), len(byHour), res.raw)
		}
		for k, w := range byHour {
			if err := w.check(tier+" "+k, got[k]); err != nil {
				t.Fatal(err)
			}
		}
	}
	every("minute rows")

	for _, c := range []struct {
		body map[string]any
		code string
	}{
		{map[string]any{"dimensions": []string{"consumer", "country"}}, "dimension_combination"},
		{map[string]any{"dimensions": []string{"consumer"}, "filters": []map[string]any{{"dimension": "app_version", "op": "is", "values": []string{"2.0"}}}}, "dimension_combination"},
		{map[string]any{"dimensions": []string{"consumer", "environment"}}, "dimension_combination"},
		{map[string]any{"dimensions": []string{"status"}, "filters": []map[string]any{{"dimension": "country", "op": "is", "values": []string{"TR"}}}}, "dimension_combination"},
		{map[string]any{"dimensions": []string{"consumer", "time:minute"}, "range": map[string]any{"period": "1h"}}, "dimension_combination"},
		{map[string]any{"dimensions": []string{"consumer"}, "range": map[string]any{"period": "15m"}}, "dimension_combination"},
	} {
		body := map[string]any{"dataset": "api", "metrics": []string{"requests"}, "range": day}
		for k, v := range c.body {
			body[k] = v
		}
		ct.owner.expectProblem(http.StatusUnprocessableEntity, c.code, "POST", "/v1/analytics/properties/"+id+"/query", body)
	}

	hour := func(utc int) time.Time { return time.Date(2026, 10, 12, utc, 0, 0, 0, time.UTC) }
	window := apiQuery(ct.owner, id, map[string]any{"metrics": fxMetrics, "dimensions": []string{"consumer"},
		"range": map[string]any{"since": hour(10).Format(time.RFC3339), "until": hour(11).Format(time.RFC3339)}})
	var inWindow []fx
	for _, r := range today {
		if !r.at.Before(hour(10)) && r.at.Before(hour(11)) {
			inWindow = append(inWindow, r)
		}
	}
	want, total := expected(inWindow, []string{"consumer"}, nil)
	if err := total.check("since and until", totals(window)); err != nil {
		t.Fatal(err)
	}
	for k, w := range want {
		if err := w.check("since and until "+k, rowKeyed(window)[k]); err != nil {
			t.Fatal(err)
		}
	}
	if meta := window.body["meta"].(map[string]any); meta["since"] != "2026-10-12T10:00:00Z" || meta["until"] != "2026-10-12T11:00:00Z" {
		t.Fatalf("meta %v", meta)
	}
	for _, r := range []map[string]any{
		{"since": "2026-10-12T10:00:30Z", "until": "2026-10-12T11:00:00Z"},
		{"since": "2026-10-12T10:00:00Z"},
		{"since": "2026-10-12T10:00:00Z", "until": "2026-10-12T11:00:00Z", "period": "1h"},
		{"since": "2026-10-12T11:00:00Z", "until": "2026-10-12T10:00:00Z"},
	} {
		ct.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties/"+id+"/query",
			map[string]any{"dataset": "api", "metrics": []string{"requests"}, "range": r})
	}
	ct.owner.expectProblem(http.StatusUnprocessableEntity, "dimension_combination", "POST", "/v1/analytics/properties/"+id+"/query",
		map[string]any{"dataset": "api", "metrics": []string{"requests"}, "dimensions": []string{"consumer"},
			"range": map[string]any{"since": "2026-10-12T10:05:00Z", "until": "2026-10-12T11:00:00Z"}})

	cmp := apiQuery(ct.owner, id, map[string]any{"metrics": []string{"requests", "errors"}, "range": day, "dimensions": []string{"route"},
		"compare": map[string]any{"with": "previous"}})
	comparison := cmp.body["comparison"].(map[string]any)
	prevRows, prevTotal := expected(yesterday, []string{"route"}, nil)
	if comparison["from"] != "2026-10-11" || comparison["since"] != "2026-10-10T21:00:00Z" || comparison["until"] != "2026-10-11T21:00:00Z" {
		t.Fatalf("comparison range %v", comparison)
	}
	if err := prevTotal.check("previous day", comparison["totals"].(map[string]any)); err != nil {
		t.Fatal(err)
	}
	for i, row := range cmp.body["rows"].([]any) {
		route := row.(map[string]any)["dimensions"].([]any)[0].(string)
		was := comparison["rows"].([]any)[i].(map[string]any)
		w := prevRows[route]
		if w == nil {
			w = &tally{}
		}
		if was["dimensions"].([]any)[0] != route || was["metrics"].(map[string]any)["requests"].(float64) != float64(w.requests) {
			t.Fatalf("comparison row of %s: %v, counted %d", route, was, w.requests)
		}
	}
	hourly := apiQuery(ct.owner, id, map[string]any{"metrics": []string{"requests"}, "range": day, "dimensions": []string{"time:hour"},
		"compare": map[string]any{"with": "previous"}})
	hrows := hourly.body["comparison"].(map[string]any)["rows"].([]any)
	if len(hrows) != 24 || hrows[13].(map[string]any)["dimensions"].([]any)[0] != "2026-10-11T13:00+03:00" ||
		hrows[13].(map[string]any)["metrics"].(map[string]any)["requests"].(float64) != float64(len(yesterday)) {
		t.Fatalf("hourly comparison %s", hourly.raw)
	}
	recent := apiQuery(ct.owner, id, map[string]any{"metrics": []string{"requests"}, "range": map[string]any{"period": "1h"}, "compare": map[string]any{"with": "previous"}})
	var lastHour, hourBefore int64
	end := now.Add(time.Minute)
	for _, r := range today {
		switch {
		case !r.at.Before(end.Add(-time.Hour)) && r.at.Before(end):
			lastHour += r.weight
		case !r.at.Before(end.Add(-2*time.Hour)) && r.at.Before(end.Add(-time.Hour)):
			hourBefore += r.weight
		}
	}
	rc := recent.body["comparison"].(map[string]any)
	if totals(recent)["requests"].(float64) != float64(lastHour) || rc["totals"].(map[string]any)["requests"].(float64) != float64(hourBefore) ||
		rc["since"] != end.Add(-2*time.Hour).Format(time.RFC3339) {
		t.Fatalf("last hour %d before %d: %s", lastHour, hourBefore, recent.raw)
	}
	year := apiQuery(ct.owner, id, map[string]any{"metrics": []string{"requests"}, "range": day, "compare": map[string]any{"with": "year"}})
	if yc := year.body["comparison"].(map[string]any); yc["from"] != "2025-10-12" || yc["totals"].(map[string]any)["requests"].(float64) != 0 {
		t.Fatalf("year comparison %s", year.raw)
	}

	routeList := func(state string, filters []map[string]any, compare bool) response {
		body := map[string]any{"metrics": []string{"requests"}, "range": day, "dimensions": []string{"route"}, "routes": state}
		if filters != nil {
			body["filters"] = filters
		}
		if compare {
			body["compare"] = map[string]any{"with": "previous"}
		}
		return apiQuery(ct.owner, id, body)
	}
	names := func(res response) []string {
		var out []string
		for k := range rowKeyed(res) {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	if got := names(routeList("new", nil, false)); fmt.Sprint(got) != "[/export /health /orders]" {
		t.Fatalf("new routes %v", got)
	}
	silent := routeList("silent", nil, true)
	legacy := 0
	for _, r := range yesterday {
		if r.route == "/legacy" {
			legacy++
		}
	}
	if got := names(silent); fmt.Sprint(got) != "[/legacy]" || totals(silent)["requests"].(float64) != 0 ||
		silent.body["comparison"].(map[string]any)["rows"].([]any)[0].(map[string]any)["metrics"].(map[string]any)["requests"].(float64) != float64(legacy) {
		t.Fatalf("silent routes %s", silent.raw)
	}
	if got := names(routeList("silent", []map[string]any{{"dimension": "environment", "op": "is", "values": []string{"staging"}}}, false)); len(got) != 0 {
		t.Fatalf("silent routes in staging %v", got)
	}
	ct.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties/"+id+"/query",
		map[string]any{"dataset": "api", "metrics": []string{"requests"}, "range": day, "routes": "new"})

	support := apiQuery(ct.owner, id, map[string]any{"metrics": []string{"requests", "conversations"}, "range": map[string]any{"from": "2026-10-11", "to": "2026-10-12"},
		"dimensions": []string{"time:day"}})
	if rows := rowKeyed(support); rows["2026-10-11"]["conversations"].(float64) != 1 || rows["2026-10-12"]["conversations"].(float64) != 2 ||
		rows["2026-10-12"]["requests"].(float64) != float64(totalOf(today)) || totals(support)["conversations"].(float64) != 3 {
		t.Fatalf("conversations %s", support.raw)
	}
	weekly := apiQuery(ct.owner, id, map[string]any{"metrics": []string{"requests", "conversations"}, "range": map[string]any{"period": "month"},
		"dimensions": []string{"time:week"}})
	if rows := rowKeyed(weekly); len(rows) != 3 || rows["2026-10-05"]["conversations"].(float64) != 1 || rows["2026-10-12"]["conversations"].(float64) != 2 ||
		rows["2026-10-12"]["requests"].(float64) != float64(totalOf(today)) || rows["2026-10-05"]["requests"].(float64) != float64(totalOf(yesterday)) {
		t.Fatalf("weeks %s", weekly.raw)
	}
	monthly := apiQuery(ct.owner, id, map[string]any{"metrics": []string{"requests"}, "range": map[string]any{"period": "month"}, "dimensions": []string{"time:month"}})
	if rows := rowKeyed(monthly); len(rows) != 1 || rows["2026-10-01"]["requests"].(float64) != float64(totalOf(today)+totalOf(yesterday)) {
		t.Fatalf("months %s", monthly.raw)
	}
	ct.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties/"+id+"/query",
		map[string]any{"dataset": "api", "metrics": []string{"conversations"}, "range": day, "dimensions": []string{"route"}})
	unlinked := createAPIProperty(ct.owner, map[string]any{"name": "Unlinked"})
	ct.owner.expectProblem(http.StatusUnprocessableEntity, "no_linked_inbox", "POST", "/v1/analytics/properties/"+unlinked+"/query",
		map[string]any{"dataset": "api", "metrics": []string{"conversations"}, "range": day})

	deploys := func(prop string, c *client) []string {
		var out []string
		for _, a := range c.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+prop+"/annotations", nil).body["items"].([]any) {
			m := a.(map[string]any)
			if m["kind"] == "deploy" {
				at, _ := time.Parse(time.RFC3339Nano, m["at"].(string))
				out = append(out, fmt.Sprintf("%s %s/%s %s %s %s", m["starts_on"], m["service"], m["environment"], m["release"], m["text"], at.UTC().Format(time.RFC3339Nano)))
			}
		}
		slices.Sort(out)
		return out
	}
	firstOf := map[string]time.Time{}
	for _, r := range slices.Concat(yesterday, today) {
		if r.release == "" || r.source != "server" {
			continue
		}
		k := r.service + "/" + r.value("environment") + " " + r.release
		if at, ok := firstOf[k]; !ok || r.at.Before(at) {
			firstOf[k] = r.at
		}
	}
	var wantDeploys []string
	for k, at := range firstOf {
		rel := strings.Fields(k)[1]
		wantDeploys = append(wantDeploys, fmt.Sprintf("%s %s %s %s", at.In(time.FixedZone("", 3*3600)).Format(time.DateOnly), k, rel, at.Format(time.RFC3339Nano)))
	}
	slices.Sort(wantDeploys)
	if got := deploys(id, ct.owner); fmt.Sprint(got) != fmt.Sprint(wantDeploys) || len(got) < 4 {
		t.Fatalf("deploy annotations\n%v\nwant\n%v", strings.Join(got, "\n"), strings.Join(wantDeploys, "\n"))
	}
	if got := deploys(theirs, other.owner); len(got) != 1 || !strings.Contains(got[0], "users/production 1.0.0") {
		t.Fatalf("the other workspace's deploys %v", got)
	}
	sendFixture(t, h, backend.bearer, appKey, id, today[:40])
	h.flushMonitor()
	if got := deploys(id, ct.owner); len(got) != len(wantDeploys) {
		t.Fatalf("a release seen before wrote a marker again: %v", got)
	}
	var deployID string
	for _, a := range ct.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/annotations", nil).body["items"].([]any) {
		if m := a.(map[string]any); m["kind"] == "deploy" {
			deployID = m["id"].(string)
		}
	}
	ct.owner.expectProblem(http.StatusForbidden, "annotation_read_only", "PATCH", "/v1/analytics/properties/"+id+"/annotations/"+deployID, map[string]any{"text": "x"})
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/analytics/properties/"+id+"/annotations", nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/analytics/properties/"+id+"/query",
		map[string]any{"dataset": "api", "metrics": []string{"requests"}, "range": day, "routes": "silent", "dimensions": []string{"route"}})
	other.apiKey(h).expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/analytics/properties/"+id+"/query",
		map[string]any{"dataset": "api", "metrics": []string{"requests"}, "range": day, "dimensions": []string{"consumer"}})

	h.clock.Set(time.Date(2026, 10, 13, 6, 0, 0, 0, time.UTC))
	h.flushMonitor()
	if err := h.srv.RollupMonitor(t.Context(), uuid.MustParse(ct.ws), uuid.MustParse(id)); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM monitor_rollup_hours WHERE workspace_id = $1 AND property_id = $2 AND (built_at IS NULL OR day_built_at IS NULL)", ct.ws, id); n != 0 {
		t.Fatalf("%d hours or days not folded", n)
	}
	today = slices.Concat(today, today[:40])
	every("folded days")
	if r := apiQuery(ct.owner, id, map[string]any{"metrics": []string{"requests"}, "range": day, "dimensions": []string{"consumer"}}); served(r) != "rollups" {
		t.Fatalf("an extra dimension of folded days is served from %s", served(r))
	}
	ct.owner.expect(http.StatusOK, "PATCH", export, map[string]any{"apdex_ms": nil})
	reset := totals(apiQuery(ct.owner, id, map[string]any{"metrics": []string{"apdex"}, "range": day,
		"filters": []map[string]any{{"dimension": "route", "op": "is", "values": []string{"/export"}}}}))
	var sat, tol, n int64
	for _, r := range today {
		if r.route == "/export" {
			n += r.weight
			if r.durUS <= 100_000 {
				sat += r.weight
			} else if r.durUS <= 400_000 {
				tol += r.weight
			}
		}
	}
	if want := (float64(sat) + float64(tol)/2) / float64(n); math.Abs(reset["apdex"].(float64)-want) > 0.0006 {
		t.Fatalf("apdex with the property's threshold %v, counted %v", reset["apdex"], want)
	}
}

func totalOf(recs []fx) int64 {
	var n int64
	for _, r := range recs {
		n += r.weight
	}
	return n
}

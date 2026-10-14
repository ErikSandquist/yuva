package api_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/productdevbook/yuva/api/internal/api"
)

func otlpFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../otlp/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func gzipped(b []byte) []byte {
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	_, _ = zw.Write(b)
	_ = zw.Close()
	return out.Bytes()
}

type otlpAnswer struct {
	status int
	header http.Header
	body   []byte
}

func exportOTLP(h *harness, bearer, id, contentType string, body []byte, header map[string]string) otlpAnswer {
	h.t.Helper()
	req, err := http.NewRequest(http.MethodPost, h.url+"/otlp/"+id+"/v1/traces", bytes.NewReader(body))
	if err != nil {
		h.t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return otlpAnswer{status: res.StatusCode, header: res.Header, body: b}
}

// protoFields reads a message's varint and bytes fields by number.
func protoFields(t *testing.T, b []byte) map[protowire.Number]any {
	t.Helper()
	out := map[protowire.Number]any{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			t.Fatalf("malformed answer %x", b)
		}
		b = b[n:]
		switch typ {
		case protowire.VarintType:
			v, m := protowire.ConsumeVarint(b)
			out[num], n = v, m
		case protowire.BytesType:
			v, m := protowire.ConsumeBytes(b)
			out[num], n = v, m
		default:
			t.Fatalf("unexpected wire type %v", typ)
		}
		b = b[n:]
	}
	return out
}

// fixtureRecords are the fixture's spans as records of the records endpoint.
func fixtureRecords() []map[string]any {
	at := func(s int) string { return time.Date(2026, 10, 12, 12, 0, s, 0, time.UTC).Format(time.RFC3339) }
	common := func(m map[string]any) map[string]any {
		m["environment"], m["service"], m["release"] = "staging", "shop-api", "1.4.2"
		return m
	}
	return []map[string]any{
		common(map[string]any{"method": "GET", "route": "/users/{id}", "status": 200, "duration_us": 12500, "time": at(1), "ip": "203.0.113.7",
			"consumer": "mobile-app", "api_version": "2026-10-01", "response_size": 512}),
		common(map[string]any{"method": "GET", "route": "/users/{id}", "status": 500, "duration_us": 80000, "time": at(2), "ip": "2001:db8:1::7",
			"consumer": "partner-api", "api_version": "2026-10-01"}),
		common(map[string]any{"method": "POST", "route": "/orders", "status": 201, "duration_us": 30000, "time": at(3), "ip": "198.51.100.20",
			"consumer": "ayse@example.com", "request_size": 128, "response_size": 64}),
		common(map[string]any{"method": "GET", "path": "/files/0f8fad5b-d9cb-469f-a165-70867728950e", "status": 200, "duration_us": 4000, "time": at(4)}),
		common(map[string]any{"method": "GET", "path": "/wp-admin/setup.php", "status": 404, "duration_us": 200, "time": at(5)}),
		common(map[string]any{"method": "GET", "route": "/health", "status": 200, "duration_us": 1000, "time": at(6), "sample_rate": 2}),
		common(map[string]any{"method": "GET", "route": "/health", "status": 200, "duration_us": 1500, "time": at(7), "sample_rate": 4}),
		common(map[string]any{"method": "GET", "route": "/broken", "status": 700, "duration_us": 2000, "time": at(11)}),
	}
}

// rollupPrint is a property's minute and dimension rows with their names, without the source.
func rollupPrint(h *harness, ws, id string) string {
	h.t.Helper()
	var core, dims string
	name := func(kind, col string) string {
		return fmt.Sprintf("coalesce((SELECT value FROM monitor_names n WHERE n.property_id = m.property_id AND n.kind = '%s' AND n.id = m.%s), '')", kind, col)
	}
	err := h.st.Pool.QueryRow(h.t.Context(), `SELECT coalesce(string_agg(x, E'\n' ORDER BY x), '') FROM (
		SELECT concat_ws(' ', to_char(m.minute AT TIME ZONE 'UTC', 'HH24:MI'), `+name("environment", "environment")+`, `+name("service", "service")+`, r.pattern,
			m.method, m.status, `+name("api_version", "api_version")+`, m.platform, sum(m.requests), sum(m.errors), sum(m.client_errors), sum(m.sampled),
			sum(m.duration_sum), min(m.duration_min), max(m.duration_max), sum(m.request_bytes), sum(m.response_bytes),
			string_agg(encode(m.durations, 'hex'), '+'), string_agg(encode(m.response_sizes, 'hex'), '+')) x
		FROM monitor_minute m JOIN monitor_routes r ON r.property_id = m.property_id AND r.id = m.route
		WHERE m.workspace_id = $1 AND m.property_id = $2
		GROUP BY m.property_id, m.minute, m.environment, m.service, r.pattern, m.method, m.status, m.api_version, m.platform) t`, ws, id).Scan(&core)
	if err != nil {
		h.t.Fatal(err)
	}
	err = h.st.Pool.QueryRow(h.t.Context(), `SELECT coalesce(string_agg(x, E'\n' ORDER BY x), '') FROM (
		SELECT concat_ws(' ', m.dim, coalesce(n.value, ''), r.pattern, m.method, m.status_class, sum(m.requests), sum(m.errors), sum(m.sampled),
			sum(m.duration_sum), string_agg(encode(m.durations, 'hex'), '+')) x
		FROM monitor_dim_hour m JOIN monitor_routes r ON r.property_id = m.property_id AND r.id = m.route
		LEFT JOIN monitor_names n ON n.property_id = m.property_id AND n.id = m.value
			AND n.kind = CASE m.dim WHEN 1 THEN 'consumer' WHEN 2 THEN 'country' ELSE 'app_version' END
		WHERE m.workspace_id = $1 AND m.property_id = $2
		GROUP BY m.dim, n.value, r.pattern, m.method, m.status_class) t`, ws, id).Scan(&dims)
	if err != nil {
		h.t.Fatal(err)
	}
	return core + "\n--\n" + dims
}

func TestOTLPExportsCountLikeRecords(t *testing.T) {
	h := geoHarness(t, nil)
	tm := newTeam(t, h)
	h.clock.Set(time.Date(2026, 10, 12, 12, 1, 0, 0, time.UTC))
	viaProto := createAPIProperty(tm.owner, map[string]any{"name": "Protobuf"})
	viaJSON := createAPIProperty(tm.owner, map[string]any{"name": "JSON"})
	viaRecords := createAPIProperty(tm.owner, map[string]any{"name": "Records"})
	viaOld := createAPIProperty(tm.owner, map[string]any{"name": "Semantic conventions 1.20"})
	backend := tm.apiKey(h)

	pb := exportOTLP(h, backend.bearer, viaProto, "application/x-protobuf", gzipped(otlpFixture(t, "traces.pb")), map[string]string{"Content-Encoding": "gzip"})
	if pb.status != http.StatusOK || pb.header.Get("Content-Type") != "application/x-protobuf" {
		t.Fatalf("protobuf export: %d %s %q", pb.status, pb.header.Get("Content-Type"), pb.body)
	}
	partial := protoFields(t, protoFields(t, pb.body)[1].([]byte))
	if partial[1] != uint64(1) || !strings.Contains(string(partial[2].([]byte)), "status must be 100 to 599") {
		t.Fatalf("protobuf partial success %v", partial)
	}
	js := exportOTLP(h, backend.bearer, viaJSON, "application/json", otlpFixture(t, "traces.json"), nil)
	var answer struct {
		PartialSuccess struct {
			RejectedSpans string `json:"rejectedSpans"`
			ErrorMessage  string `json:"errorMessage"`
		} `json:"partialSuccess"`
	}
	if js.status != http.StatusOK || js.header.Get("Content-Type") != "application/json" || json.Unmarshal(js.body, &answer) != nil ||
		answer.PartialSuccess.RejectedSpans != "1" || !strings.Contains(answer.PartialSuccess.ErrorMessage, "status must be 100 to 599") {
		t.Fatalf("JSON export: %d %s", js.status, js.body)
	}
	if old := exportOTLP(h, backend.bearer, viaOld, "application/x-protobuf", otlpFixture(t, "traces-semconv-1.20.pb"), nil); old.status != http.StatusOK ||
		protoFields(t, protoFields(t, old.body)[1].([]byte))[1] != uint64(1) {
		t.Fatalf("1.20 conventions export: %d %q", old.status, old.body)
	}
	recs := sendRecords(backend, http.StatusAccepted, viaRecords, fixtureRecords()...)
	if recs.body["accepted"].(float64) != 7 || len(recs.body["errors"].([]any)) != 1 {
		t.Fatalf("records %s", recs.raw)
	}
	h.flushMonitor()

	want := rollupPrint(h, tm.ws, viaRecords)
	if n := h.count("SELECT sum(m.sampled)::int FROM monitor_minute m JOIN monitor_routes r ON r.property_id = m.property_id AND r.id = m.route WHERE m.workspace_id = $1 AND m.property_id = $2 AND r.pattern = '/health'", tm.ws, viaProto); n != 6 {
		t.Fatalf("sampled spans weigh %d, want 2 + 4", n)
	}
	for name, id := range map[string]string{"protobuf": viaProto, "JSON": viaJSON, "1.20 conventions": viaOld} {
		if got := rollupPrint(h, tm.ws, id); got != want {
			t.Fatalf("%s rollups differ from the records endpoint's:\n%s\nwant\n%s", name, got, want)
		}
		if n := h.count("SELECT count(*) FROM monitor_minute WHERE workspace_id = $1 AND property_id = $2 AND source <> 2", tm.ws, id); n != 0 {
			t.Fatalf("%s: %d rows not of source otlp", name, n)
		}
		stats := h.count("SELECT coalesce(sum(records) FILTER (WHERE reason = 'accepted') * 10 + sum(records) FILTER (WHERE reason = 'invalid'), 0)::int FROM monitor_ingest_stats WHERE workspace_id = $1 AND property_id = $2", tm.ws, id)
		if stats != 71 {
			t.Fatalf("%s: ingest stats %d, want 7 accepted and 1 invalid", name, stats)
		}
	}
	day := map[string]any{"metrics": apiMetrics, "range": dayRange("2026-10-12"), "dimensions": []string{"route"}}
	byRoute := apiQuery(tm.owner, viaProto, day)
	if numbers(byRoute) != numbers(apiQuery(tm.owner, viaRecords, day)) {
		t.Fatalf("answers differ:\n%s\n%s", numbers(byRoute), numbers(apiQuery(tm.owner, viaRecords, day)))
	}
	rows := rowsBy(byRoute)
	if len(rows) != 5 || rows["/users/{id}"]["requests"].(float64) != 2 || rows["/users/{id}"]["errors"].(float64) != 1 || rows["/health"]["requests"].(float64) != 6 ||
		rows["(unmatched)"]["requests"].(float64) != 1 || rows["/files/{id}"] == nil || rows["/orders"] == nil || byRoute.body["meta"].(map[string]any)["estimated"] != true {
		t.Fatalf("by route %s", byRoute.raw)
	}
	if n := h.count("SELECT count(*) FROM monitor_routes WHERE workspace_id = $1 AND property_id = $2 AND inferred AND pattern = '/files/{id}'", tm.ws, viaProto); n != 1 {
		t.Fatal("the route without http.route was not inferred")
	}
	if n := h.count("SELECT count(*) FROM monitor_names WHERE workspace_id = $1 AND property_id = $2 AND value IN ('payments.example', 'SELECT orders', 'List', '/broken')", tm.ws, viaProto); n != 0 {
		t.Fatal("a skipped span left a name")
	}
}

func TestOTLPAccessAndErrors(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	h.clock.Set(time.Date(2026, 10, 12, 12, 1, 0, 0, time.UTC))
	id := createAPIProperty(tm.owner, map[string]any{"name": "API"})
	site, _ := createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"example.com"}})
	backend := tm.apiKey(h).bearer
	reader := tm.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "reader", "scopes": []string{"analytics:read"}}).str("secret")
	pb, js := otlpFixture(t, "traces.pb"), otlpFixture(t, "traces.json")

	statusOf := func(a otlpAnswer, asJSON bool) (int64, string) {
		t.Helper()
		if asJSON {
			var st struct {
				Code    int64  `json:"code"`
				Message string `json:"message"`
			}
			if a.header.Get("Content-Type") != "application/json" || json.Unmarshal(a.body, &st) != nil {
				t.Fatalf("JSON status %d %s %q", a.status, a.header.Get("Content-Type"), a.body)
			}
			return st.Code, st.Message
		}
		if a.header.Get("Content-Type") != "application/x-protobuf" {
			t.Fatalf("protobuf status %d %s %q", a.status, a.header.Get("Content-Type"), a.body)
		}
		f := protoFields(t, a.body)
		msg, _ := f[2].([]byte)
		code, _ := f[1].(uint64)
		return int64(code), string(msg)
	}
	for _, c := range []struct {
		name, bearer, id, ctype string
		body                    []byte
		header                  map[string]string
		status                  int
		code                    int64
		prefix                  string
	}{
		{"no key", "", id, "application/x-protobuf", pb, nil, http.StatusUnauthorized, 16, "unauthenticated:"},
		{"wrong key", "yuva_nope_" + strings.Repeat("x", 32), id, "application/json", js, nil, http.StatusUnauthorized, 16, "unauthenticated:"},
		{"read-only key", reader, id, "application/x-protobuf", pb, nil, http.StatusForbidden, 7, "insufficient_scope:"},
		{"another workspace", other.apiKey(h).bearer, id, "application/json", js, nil, http.StatusNotFound, 5, "not_found:"},
		{"unknown property", backend, "0199e7c0-0000-7000-8000-000000000000", "application/x-protobuf", pb, nil, http.StatusNotFound, 5, "not_found:"},
		{"site", backend, site, "application/json", js, nil, http.StatusUnprocessableEntity, 9, "property_kind:"},
		{"type", backend, id, "text/plain", js, nil, http.StatusUnsupportedMediaType, 3, "unsupported_media_type:"},
		{"not protobuf", backend, id, "application/x-protobuf", []byte{0x0a, 0x05, 0x01}, nil, http.StatusBadRequest, 3, "validation_failed:"},
		{"not JSON", backend, id, "application/json", []byte("{"), nil, http.StatusBadRequest, 3, "validation_failed:"},
		{"not gzip", backend, id, "application/x-protobuf", pb, map[string]string{"Content-Encoding": "gzip"}, http.StatusBadRequest, 3, "validation_failed:"},
	} {
		a := exportOTLP(h, c.bearer, c.id, c.ctype, c.body, c.header)
		code, msg := statusOf(a, c.ctype != "application/x-protobuf")
		if a.status != c.status || code != c.code || !strings.HasPrefix(msg, c.prefix) {
			t.Fatalf("%s: %d code %d %q", c.name, a.status, code, msg)
		}
	}
	if n := h.count("SELECT count(*) FROM monitor_ingest_stats WHERE workspace_id = $1", tm.ws); n != 0 {
		t.Fatalf("refused exports were counted: %d", n)
	}
	empty := exportOTLP(h, backend, id, "application/json", []byte(`{"resourceSpans":[]}`), nil)
	if empty.status != http.StatusOK || string(empty.body) != "{}" {
		t.Fatalf("empty export %d %s", empty.status, empty.body)
	}
	tm.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/otlp/"+id+"/v1/traces", nil)
}

func TestOTLPLimitsAskToRetry(t *testing.T) {
	h := newHarnessWith(t, func(d *api.Deps) { d.Analytics.Monitor.RecordsPerMinute = 9 })
	tm := newTeam(t, h)
	h.clock.Set(time.Date(2026, 10, 12, 12, 1, 0, 0, time.UTC))
	id := createAPIProperty(tm.owner, map[string]any{"name": "API"})
	backend := tm.apiKey(h).bearer
	pb := otlpFixture(t, "traces.pb")
	if a := exportOTLP(h, backend, id, "application/x-protobuf", pb, nil); a.status != http.StatusOK {
		t.Fatalf("first export %d", a.status)
	}
	second := exportOTLP(h, backend, id, "application/x-protobuf", pb, nil)
	if second.status != http.StatusOK {
		t.Fatalf("second export %d", second.status)
	}
	third := exportOTLP(h, backend, id, "application/x-protobuf", pb, nil)
	f := protoFields(t, third.body)
	if third.status != http.StatusTooManyRequests || third.header.Get("Retry-After") != "60" || f[1] != uint64(8) {
		t.Fatalf("over the limit: %d %q %v", third.status, third.header.Get("Retry-After"), f)
	}
	h.flushMonitor()
	if n := h.count("SELECT records::int FROM monitor_ingest_stats WHERE workspace_id = $1 AND property_id = $2 AND reason = 'rate'", tm.ws, id); n != 12 {
		t.Fatalf("%d spans counted as rate", n)
	}

	full := newHarnessWith(t, func(d *api.Deps) { d.Analytics.Monitor.BufferKeys = 1 })
	ft := newTeam(t, full)
	full.clock.Set(time.Date(2026, 10, 12, 12, 1, 0, 0, time.UTC))
	fid := createAPIProperty(ft.owner, map[string]any{"name": "API"})
	busy := exportOTLP(full, ft.apiKey(full).bearer, fid, "application/json", otlpFixture(t, "traces.json"), nil)
	var st map[string]any
	_ = json.Unmarshal(busy.body, &st)
	if busy.status != http.StatusServiceUnavailable || busy.header.Get("Retry-After") != "10" || st["code"] != float64(14) {
		t.Fatalf("buffer full: %d %q %s", busy.status, busy.header.Get("Retry-After"), busy.body)
	}
}

func appRequests(h *harness, key string, body any, header map[string]string) response {
	h.t.Helper()
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, h.url+"/client/v1/analytics/"+key+"/requests", bytes.NewReader(raw))
	if err != nil {
		h.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	out := response{status: res.StatusCode, raw: b, header: res.Header}
	_ = json.Unmarshal(b, &out.body)
	return out
}

func TestAppRequestRecords(t *testing.T) {
	h := geoHarness(t, nil)
	tm := newTeam(t, h)
	h.clock.Set(time.Date(2026, 10, 12, 12, 1, 0, 0, time.UTC))
	open, openKey := createProperty(tm.owner, map[string]any{"kind": "api", "name": "Open", "domains": []string{}, "platforms": []string{"ios", "android"}})
	gated, gatedKey := createProperty(tm.owner, map[string]any{"kind": "api", "name": "Gated", "domains": []string{}, "platforms": []string{"ios"}, "collection": "consent"})
	_, noAppsKey := createProperty(tm.owner, map[string]any{"kind": "api", "name": "No apps", "domains": []string{}})
	_, siteKey := createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"example.com"}, "platforms": []string{"web", "ios"}})
	app := map[string]any{"platform": "ios", "version": "2.3.1"}
	rec := func(status int, consent bool) map[string]any {
		r := map[string]any{"method": "GET", "route": "/users/{id}", "status": status, "duration_us": 42000, "time": "2026-10-12T12:00:30Z",
			"consumer": "someone", "response_size": 900}
		if consent {
			r["c"] = map[string]any{"a": 1}
		}
		return r
	}
	batch := func(recs ...map[string]any) map[string]any { return map[string]any{"a": app, "records": recs} }

	if r := appRequests(h, openKey, batch(rec(200, false), rec(500, false), map[string]any{"method": "GET", "status": 200, "duration_us": 1}), nil); r.status != http.StatusAccepted || len(r.raw) != 0 {
		t.Fatalf("open property: %d %s", r.status, r.raw)
	}
	if r := appRequests(h, gatedKey, batch(rec(200, false), rec(200, false)), nil); r.status != http.StatusAccepted {
		t.Fatalf("before consent: %d %s", r.status, r.raw)
	}
	h.flushMonitor()
	if n := h.count("SELECT coalesce(sum(requests), 0)::int FROM monitor_minute WHERE workspace_id = $1 AND property_id = $2", tm.ws, gated); n != 0 {
		t.Fatalf("%d records counted before consent", n)
	}
	if r := appRequests(h, gatedKey, batch(rec(200, true), rec(200, false)), map[string]string{"Sec-GPC": "1"}); r.status != http.StatusAccepted {
		t.Fatalf("opted out: %d", r.status)
	}
	if r := appRequests(h, gatedKey, batch(rec(200, true), rec(200, false)), nil); r.status != http.StatusAccepted {
		t.Fatalf("after consent: %d %s", r.status, r.raw)
	}
	h.flushMonitor()
	if n := h.count("SELECT coalesce(sum(requests), 0)::int FROM monitor_minute WHERE workspace_id = $1 AND property_id = $2 AND source = 3 AND platform = 1", tm.ws, gated); n != 1 {
		t.Fatalf("%d consented records with source app", n)
	}
	if n := h.count("SELECT coalesce(sum(requests), 0)::int FROM monitor_minute WHERE workspace_id = $1 AND property_id = $2 AND source = 3 AND platform = 1", tm.ws, open); n != 2 {
		t.Fatalf("%d records with source app on the open property", n)
	}
	var names string
	if err := h.st.Pool.QueryRow(h.t.Context(), "SELECT string_agg(kind || '=' || value, ' ' ORDER BY kind) FROM monitor_names WHERE workspace_id = $1 AND property_id = $2 AND value <> '(other)'", tm.ws, open).Scan(&names); err != nil {
		t.Fatal(err)
	}
	if names != "app_version=2.3.1 country=TR environment=production" {
		t.Fatalf("names %q", names)
	}
	if n := h.count("SELECT records::int FROM monitor_ingest_stats WHERE workspace_id = $1 AND property_id = $2 AND reason = 'invalid'", tm.ws, open); n != 1 {
		t.Fatalf("%d invalid records", n)
	}
	rows := rowsBy(apiQuery(tm.owner, open, map[string]any{"metrics": []string{"requests", "errors"}, "range": dayRange("2026-10-12"), "dimensions": []string{"route"}}))
	if rows["/users/{id}"]["requests"].(float64) != 2 || rows["/users/{id}"]["errors"].(float64) != 1 {
		t.Fatalf("rows %v", rows)
	}

	for _, c := range []struct {
		name, key string
		body      any
		header    map[string]string
		status    int
		code      string
	}{
		{"site key", siteKey, batch(rec(200, false)), nil, http.StatusNotFound, "not_found"},
		{"unknown key", "yuva_ak_unknown", batch(rec(200, false)), nil, http.StatusNotFound, "not_found"},
		{"no apps", noAppsKey, batch(rec(200, false)), nil, http.StatusForbidden, "origin_not_allowed"},
		{"browser", openKey, batch(rec(200, false)), map[string]string{"Origin": "https://example.com"}, http.StatusForbidden, "origin_not_allowed"},
		{"platform", gatedKey, map[string]any{"a": map[string]any{"platform": "android"}, "records": []any{rec(200, true)}}, nil, http.StatusForbidden, "platform_not_allowed"},
		{"no app", openKey, map[string]any{"records": []any{rec(200, false)}}, nil, http.StatusBadRequest, "validation_failed"},
		{"no records", openKey, batch(), nil, http.StatusBadRequest, "validation_failed"},
		{"too many", openKey, batch(func() []map[string]any {
			var out []map[string]any
			for range 101 {
				out = append(out, map[string]any{"method": "GET", "route": "/", "status": 200, "duration_us": 1})
			}
			return out
		}()...), nil, http.StatusBadRequest, "validation_failed"},
	} {
		r := appRequests(h, c.key, c.body, c.header)
		if r.status != c.status || r.str("code") != c.code {
			t.Fatalf("%s: %d %s", c.name, r.status, r.raw)
		}
	}
	var many []map[string]any
	for range 100 {
		many = append(many, map[string]any{"method": "GET", "route": "/ping", "status": 200, "duration_us": 1})
	}
	for i := range 6 {
		n := 100
		if i == 5 {
			n = 98
		}
		if r := appRequests(h, openKey, batch(many[:n]...), nil); r.status != http.StatusAccepted {
			t.Fatalf("batch %d: %d", i, r.status)
		}
	}
	if r := appRequests(h, openKey, batch(many[:1]...), nil); r.status != http.StatusTooManyRequests || r.header.Get("Retry-After") == "" {
		t.Fatalf("past 600 a minute from one address: %d %s", r.status, r.raw)
	}
}

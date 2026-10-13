package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/api"
)

const testUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15"

type collectReq struct {
	origin, ip string
	header     map[string]string
	body       any
}

func collect(h *harness, key string, r collectReq) response {
	h.t.Helper()
	raw, ok := r.body.([]byte)
	if !ok {
		raw, _ = json.Marshal(r.body)
	}
	req, err := http.NewRequest(http.MethodPost, h.url+"/client/v1/analytics/"+key, bytes.NewReader(raw))
	if err != nil {
		h.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	req.Header.Set("User-Agent", testUA)
	req.Header.Set("Accept-Language", "tr-TR,tr;q=0.9,en;q=0.8")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	if r.origin != "" {
		req.Header.Set("Origin", r.origin)
	}
	req.Header.Set("X-Forwarded-For", r.ip)
	for k, v := range r.header {
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

func pageview(u string) map[string]any {
	return map[string]any{"n": "pageview", "u": u, "r": "https://www.google.com/search?q=secret", "w": 1280}
}

func (h *harness) flushAnalytics() {
	h.t.Helper()
	if err := h.srv.FlushAnalytics(context.Background()); err != nil {
		h.t.Fatal(err)
	}
}

func createProperty(c *client, body map[string]any) (id, key string) {
	c.h.t.Helper()
	r := c.expect(http.StatusCreated, "POST", "/v1/analytics/properties", body)
	return r.str("id"), r.str("public_key")
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

var testAddresses = regexp.MustCompile(`203\.0\.113\.|198\.51\.100\.|192\.0\.2\.|2001:db8`)

func TestAnalyticsCollectionPrivacy(t *testing.T) {
	logs := &lockedBuffer{}
	h := newHarnessWith(t, func(d *api.Deps) { d.Log = slog.New(slog.NewJSONHandler(logs, nil)) })
	tm := newTeam(t, h)
	id, key := createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"Example.com", "*.example.org"}, "timezone": "Europe/Istanbul"})
	ctx := context.Background()
	ok := func(r response) {
		t.Helper()
		if r.status != http.StatusAccepted {
			t.Fatalf("status %d: %s", r.status, r.raw)
		}
	}

	r := collect(h, key, collectReq{origin: "https://example.com", ip: "203.0.113.77", body: pageview(
		"https://example.com/users/ayse@example.com/orders?utm_source=newsletter&utm_medium=email&utm_campaign=x@y.co&secret=1")})
	ok(r)
	if r.header.Get("Access-Control-Allow-Origin") != "https://example.com" || r.header.Get("Set-Cookie") != "" || len(r.raw) != 0 {
		t.Fatalf("collect answer %v %q", r.header, r.raw)
	}
	ok(collect(h, key, collectReq{origin: "https://example.com", ip: "203.0.113.200", body: map[string]any{"n": "pageview", "u": "https://example.com/pricing"}}))
	ok(collect(h, key, collectReq{origin: "https://example.com", ip: "203.0.113.9", body: map[string]any{"n": "engagement", "u": "https://example.com/pricing", "ms": 4200, "sd": 80}}))
	h.flushAnalytics()

	var visits, views int
	var entry, exit, src, medium, campaign, refHost, refPath, lang string
	var channel, screen int16
	var engaged int64
	if err := h.st.Pool.QueryRow(ctx, `SELECT count(*) OVER (), views, entry_path, exit_path, utm_source, utm_medium, utm_campaign, referrer_host,
		referrer_path, language, channel, screen, engaged_ms FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2`, tm.ws, id).
		Scan(&visits, &views, &entry, &exit, &src, &medium, &campaign, &refHost, &refPath, &lang, &channel, &screen, &engaged); err != nil {
		t.Fatal(err)
	}
	if visits != 1 || views != 2 || entry != "/users/:email/orders" || exit != "/pricing" || src != "newsletter" || medium != "email" ||
		campaign != "" || refHost != "www.google.com" || refPath != "/search" || lang != "tr" || channel != 6 || screen != 3 || engaged != 4200 {
		t.Fatalf("visit: %d %d %q %q %q %q %q %q %q %q %d %d %d", visits, views, entry, exit, src, medium, campaign, refHost, refPath, lang, channel, screen, engaged)
	}
	if n := h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 3 {
		t.Fatalf("%d events", n)
	}
	if n := h.count("SELECT coalesce(sum(events), 0) FROM analytics_ingest_stats WHERE workspace_id = $1 AND property_id = $2 AND reason = 'scrubbed'", tm.ws, id); n != 1 {
		t.Fatalf("%d scrubbed", n)
	}
	usage := tm.owner.expect(http.StatusOK, "GET", "/v1/usage", nil).body["items"].([]any)[0].(map[string]any)
	if usage["analytics_events"].(float64) != 3 {
		t.Fatalf("usage %v", usage)
	}
	prop := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id, nil)
	if prop.body["visitors_today"].(float64) != 1 || prop.str("first_event_at") == "" {
		t.Fatalf("property %s", prop.raw)
	}

	before := h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1", tm.ws)
	for _, hdr := range []map[string]string{{"Sec-GPC": "1"}, {"DNT": "1"}} {
		ok(collect(h, key, collectReq{origin: "https://example.com", ip: "198.51.100.1", header: hdr, body: pageview("https://example.com/gpc")}))
	}
	h.flushAnalytics()
	if n := h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1", tm.ws); n != before {
		t.Fatal("an event with GPC or DNT was stored")
	}
	if n := h.count("SELECT count(*) FROM analytics_ingest_stats WHERE workspace_id = $1 AND reason = 'accepted'", tm.ws); n == 0 {
		t.Fatal("no ingest counts")
	}

	for _, c := range []struct {
		origin, page string
		status       int
		code         string
	}{
		{"https://evil.example.net", "https://example.com/", http.StatusForbidden, "origin_not_allowed"},
		{"https://www.example.com", "https://www.example.com/", http.StatusForbidden, "origin_not_allowed"},
		{"https://example.org", "https://example.org/", http.StatusForbidden, "origin_not_allowed"},
		{"", "https://example.com/", http.StatusForbidden, "origin_not_allowed"},
		{"https://example.com", "https://evil.example.net/", http.StatusForbidden, "page_not_allowed"},
	} {
		r := collect(h, key, collectReq{origin: c.origin, ip: "198.51.100.2", body: pageview(c.page)})
		if r.status != c.status || r.str("code") != c.code {
			t.Fatalf("%s %s: %d %s", c.origin, c.page, r.status, r.raw)
		}
		if c.origin != "" && r.header.Get("Access-Control-Allow-Origin") != c.origin {
			t.Fatalf("refusal without CORS headers: %v", r.header)
		}
	}
	ok(collect(h, key, collectReq{origin: "https://docs.example.org", ip: "198.51.100.3", body: pageview("https://docs.example.org/start")}))
	if r := collect(h, "yuva_ak_unknown", collectReq{origin: "https://example.com", ip: "198.51.100.3", body: pageview("https://example.com/")}); r.status != http.StatusNotFound {
		t.Fatalf("unknown key: %d", r.status)
	}
	if r := collect(h, key, collectReq{origin: "https://example.com", ip: "198.51.100.3", body: map[string]any{"n": "yuva_signup", "u": "https://example.com/"}}); r.status != http.StatusBadRequest {
		t.Fatalf("reserved event name: %d", r.status)
	}
	if r := collect(h, key, collectReq{origin: "https://example.com", ip: "198.51.100.3", body: bytes.Repeat([]byte("a"), 70<<10)}); r.status != http.StatusRequestEntityTooLarge {
		t.Fatalf("large body: %d", r.status)
	}
	pre, err := http.NewRequest(http.MethodOptions, h.url+"/client/v1/analytics/"+key, nil)
	if err != nil {
		t.Fatal(err)
	}
	pre.Header.Set("Origin", "https://example.com")
	pre.Header.Set("Access-Control-Request-Method", "POST")
	res, err := http.DefaultClient.Do(pre)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent || res.Header.Get("Access-Control-Allow-Methods") != "POST" {
		t.Fatalf("preflight %d %v", res.StatusCode, res.Header)
	}

	var flooded response
	for i := 0; i < 125; i++ {
		flooded = collect(h, key, collectReq{origin: "https://example.com", ip: "192.0.2." + string(rune('0'+i%10)), body: pageview("https://example.com/flood")})
		if flooded.status == http.StatusTooManyRequests {
			break
		}
	}
	if flooded.status != http.StatusTooManyRequests || flooded.header.Get("Retry-After") == "" {
		t.Fatalf("flood: %d %s", flooded.status, flooded.raw)
	}
	h.flushAnalytics()
	if n := h.count("SELECT coalesce(sum(events), 0) FROM analytics_ingest_stats WHERE property_id = $1 AND workspace_id = $2 AND reason = 'rate'", id, tm.ws); n == 0 {
		t.Fatal("the flood is not counted")
	}

	tm.owner.expect(http.StatusOK, "PATCH", "/v1/analytics/properties/"+id, map[string]any{"excluded_ips": []string{"198.51.100.0/24"}, "excluded_paths": []string{"/admin/**"}})
	before = h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1", tm.ws)
	ok(collect(h, key, collectReq{origin: "https://example.com", ip: "198.51.100.40", body: pageview("https://example.com/")}))
	ok(collect(h, key, collectReq{origin: "https://example.com", ip: "203.0.113.5", body: pageview("https://example.com/admin/users")}))
	h.flushAnalytics()
	if n := h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1", tm.ws); n != before {
		t.Fatal("an excluded address or path was stored")
	}

	var columns []string
	rows, err := h.st.Pool.Query(ctx, `SELECT table_name || '.' || column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name LIKE 'analytics\_%' AND data_type IN ('inet', 'cidr', 'ARRAY') AND udt_name IN ('inet', 'cidr', '_inet', '_cidr')`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, c)
	}
	rows.Close()
	if len(columns) != 1 || columns[0] != "analytics_properties.excluded_ips" {
		t.Fatalf("address columns: %v", columns)
	}
	rows, err = h.st.Pool.Query(ctx, `SELECT c.relname, a.attname FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relname LIKE 'analytics\_%' AND c.relkind IN ('r', 'p') AND a.attnum > 0 AND NOT a.attisdropped
		  AND a.atttypid IN ('text'::regtype, 'jsonb'::regtype, 'bpchar'::regtype)`)
	if err != nil {
		t.Fatal(err)
	}
	type col struct{ table, column string }
	var text []col
	for rows.Next() {
		var c col
		if err := rows.Scan(&c.table, &c.column); err != nil {
			t.Fatal(err)
		}
		text = append(text, c)
	}
	rows.Close()
	for _, c := range text {
		var hits int
		if err := h.st.Pool.QueryRow(ctx, `SELECT count(*) FROM `+c.table+` WHERE workspace_id = $1 AND `+c.column+`::text ~ '(\d{1,3}\.){3}\d{1,3}|[0-9a-fA-F]{1,4}(:[0-9a-fA-F]{0,4}){2,7}'`, tm.ws).Scan(&hits); err != nil {
			t.Fatal(err)
		}
		if hits > 0 {
			t.Fatalf("%s.%s holds an address-shaped value", c.table, c.column)
		}
	}
	if strings.Contains(logs.String(), `"/client/v1/analytics/`) == false {
		t.Fatal("the request log was not captured")
	}
	if m := testAddresses.FindString(logs.String()); m != "" {
		t.Fatalf("the log holds an address: %s", m)
	}
}

func TestAnalyticsSaltsArePerPropertyAndDay(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	a, keyA := createProperty(tm.owner, map[string]any{"name": "A", "domains": []string{"example.com"}})
	b, keyB := createProperty(tm.owner, map[string]any{"name": "B", "domains": []string{"example.com"}})
	send := func(key string) {
		if r := collect(h, key, collectReq{origin: "https://example.com", ip: "203.0.113.7", body: pageview("https://example.com/")}); r.status != http.StatusAccepted {
			t.Fatalf("collect %d %s", r.status, r.raw)
		}
	}
	send(keyA)
	send(keyB)
	h.clock.Advance(24 * time.Hour)
	send(keyA)
	h.flushAnalytics()
	ctx := context.Background()
	salts := map[string][]byte{}
	rows, err := h.st.Pool.Query(ctx, "SELECT property_id::text || ' ' || day::text, salt, expires_at, day FROM analytics_salts WHERE workspace_id = $1", tm.ws)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var k string
		var s []byte
		var exp, day time.Time
		if err := rows.Scan(&k, &s, &exp, &day); err != nil {
			t.Fatal(err)
		}
		if want := day.AddDate(0, 0, 1).Add(2 * time.Hour); !exp.Equal(want) {
			t.Fatalf("salt of %s expires %v, want %v", k, exp, want)
		}
		salts[k] = s
	}
	rows.Close()
	if len(salts) != 3 {
		t.Fatalf("%d salts", len(salts))
	}
	seen := map[string]bool{}
	for _, s := range salts {
		if seen[string(s)] {
			t.Fatal("two salts are equal")
		}
		seen[string(s)] = true
	}
	var visitors []int64
	rows, err = h.st.Pool.Query(ctx, "SELECT visitor FROM analytics_visits WHERE workspace_id = $1 AND property_id = ANY($2::uuid[]) ORDER BY day, property_id", tm.ws, []string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		visitors = append(visitors, v)
	}
	rows.Close()
	if len(visitors) != 3 || visitors[0] == visitors[1] || visitors[0] == visitors[2] || visitors[1] == visitors[2] {
		t.Fatalf("the same person is the same visitor across properties or days: %v", visitors)
	}
}

func TestAnalyticsConsentMode(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	id, key := createProperty(tm.owner, map[string]any{"name": "Consent", "domains": []string{"example.com"}, "collection": "consent"})
	ev := pageview("https://example.com/")
	if r := collect(h, key, collectReq{origin: "https://example.com", ip: "203.0.113.7", body: ev}); r.status != http.StatusAccepted {
		t.Fatalf("collect %d", r.status)
	}
	h.flushAnalytics()
	if n := h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 0 {
		t.Fatal("an event without consent was stored")
	}
	if n := h.count("SELECT count(*) FROM analytics_salts WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 0 {
		t.Fatal("a salt was made for an event without consent")
	}
	ev["c"] = map[string]any{"a": 1}
	if r := collect(h, key, collectReq{origin: "https://example.com", ip: "203.0.113.7", body: ev}); r.status != http.StatusAccepted {
		t.Fatalf("collect %d", r.status)
	}
	h.flushAnalytics()
	if n := h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 1 {
		t.Fatalf("%d events after consent", n)
	}
}

func TestAnalyticsPropertyAccess(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	second := tm.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Other", "slug": "other"}).body["inbox"].(map[string]any)["id"].(string)
	tm.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+tm.inbox+"/members/"+tm.agentID, nil)
	linked, _ := createProperty(tm.owner, map[string]any{"name": "Linked", "domains": []string{"example.com"}, "inbox_id": tm.inbox})
	elsewhere, _ := createProperty(tm.owner, map[string]any{"name": "Elsewhere", "domains": []string{"example.com"}, "inbox_id": second})
	unlinked, _ := createProperty(tm.owner, map[string]any{"name": "Unlinked", "domains": []string{"example.com"}})

	items := tm.agent.expect(http.StatusOK, "GET", "/v1/analytics/properties", nil).body["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != linked {
		t.Fatalf("agent sees %v", items)
	}
	tm.agent.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+linked, nil)
	for _, id := range []string{elsewhere, unlinked} {
		tm.agent.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/analytics/properties/"+id, nil)
	}
	tm.agent.expectProblem(http.StatusForbidden, "forbidden", "PATCH", "/v1/analytics/properties/"+linked, map[string]any{"name": "Mine"})
	tm.agent.expectProblem(http.StatusForbidden, "forbidden", "POST", "/v1/analytics/properties", map[string]any{"name": "New", "domains": []string{"example.com"}})
	if n := len(tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties", nil).body["items"].([]any)); n != 3 {
		t.Fatalf("owner sees %d", n)
	}

	for _, c := range []struct{ method, path string }{
		{"GET", "/v1/analytics/properties/" + linked}, {"PATCH", "/v1/analytics/properties/" + linked},
		{"POST", "/v1/analytics/properties/" + linked + "/rotate-key"}, {"POST", "/v1/analytics/properties/" + linked + "/delete-data"},
		{"DELETE", "/v1/analytics/properties/" + linked},
	} {
		var body any
		switch c.method {
		case "PATCH":
			body = map[string]any{"name": "Taken"}
		case "DELETE":
			body = map[string]any{"name": "Linked"}
		case "POST":
			body = map[string]any{"all": true}
		}
		other.owner.expectProblem(http.StatusNotFound, "not_found", c.method, c.path, body)
	}
	if n := len(other.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties", nil).body["items"].([]any)); n != 0 {
		t.Fatalf("another workspace sees %d properties", n)
	}

	plain := h.client()
	plain.bearer = tm.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "old style", "scopes": []string{"conversations:read"}}).str("secret")
	plain.expectProblem(http.StatusForbidden, "insufficient_scope", "GET", "/v1/analytics/properties", nil)
	full := tm.apiKey(h)
	full.expect(http.StatusOK, "GET", "/v1/analytics/properties", nil)
	full.expect(http.StatusCreated, "POST", "/v1/analytics/properties", map[string]any{"name": "By key", "domains": []string{"*.example.com"}})
	if r := tm.owner.do("POST", "/v1/api-keys", map[string]any{"name": "limited", "inbox_ids": []string{tm.inbox}, "scopes": []string{"analytics:manage"}}); r.status == http.StatusCreated {
		t.Fatal("a key limited to inboxes holds analytics:manage")
	}
	limited := h.client()
	limited.bearer = tm.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "limited", "inbox_ids": []string{tm.inbox}, "scopes": []string{"analytics:read"}}).str("secret")
	if items := limited.expect(http.StatusOK, "GET", "/v1/analytics/properties", nil).body["items"].([]any); len(items) != 1 || items[0].(map[string]any)["id"] != linked {
		t.Fatalf("limited key sees %v", items)
	}

	for _, bad := range []map[string]any{
		{"name": "Bad", "domains": []string{"https://example.com/path"}},
		{"name": "Bad", "domains": []string{}},
		{"name": "Kids", "domains": []string{"example.com"}, "audience": "children", "consent_features": []string{"persistent"}},
		{"name": "Bad", "domains": []string{"example.com"}, "timezone": "Mars/Base"},
		{"name": "Bad", "domains": []string{"example.com"}, "excluded_ips": []string{"not an address"}},
	} {
		tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties", bad)
	}
	kids := tm.owner.expect(http.StatusCreated, "POST", "/v1/analytics/properties", map[string]any{"name": "Kids", "domains": []string{"kids.example.com"}, "audience": "children"})
	if kids.str("audience") != "children" || kids.body["retention_months"].(float64) != 25 {
		t.Fatalf("children's property %s", kids.raw)
	}
}

func TestAnalyticsKeyRotationAndDeletion(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	id, key := createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"example.com"}})
	send := func(k string) int {
		return collect(h, k, collectReq{origin: "https://example.com", ip: "203.0.113.7", body: pageview("https://example.com/")}).status
	}
	rotated := tm.owner.expect(http.StatusOK, "POST", "/v1/analytics/properties/"+id+"/rotate-key", nil)
	newKey := rotated.str("public_key")
	if newKey == key || !strings.HasPrefix(newKey, "yuva_ak_") || rotated.str("previous_key_expires_at") == "" {
		t.Fatalf("rotation %s", rotated.raw)
	}
	if send(key) != http.StatusAccepted || send(newKey) != http.StatusAccepted {
		t.Fatal("a key in its overlap was refused")
	}
	h.flushAnalytics()

	tm.owner.expectProblem(http.StatusBadRequest, "confirmation_mismatch", "DELETE", "/v1/analytics/properties/"+id, map[string]any{"name": "site"})
	tm.owner.expect(http.StatusAccepted, "DELETE", "/v1/analytics/properties/"+id, map[string]any{"name": "Site"})
	if send(newKey) != http.StatusNotFound {
		t.Fatal("a deleted property's key works")
	}
	tm.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/analytics/properties/"+id, nil)
	done, err := h.srv.DeleteAnalytics(context.Background(), api.AnalyticsDeleteArgs{WorkspaceID: uuid.MustParse(tm.ws), PropertyID: uuid.MustParse(id), Property: true}, time.Minute)
	if err != nil || !done {
		t.Fatalf("deletion %v %v", done, err)
	}
	for _, table := range []string{"analytics_events", "analytics_visit_pages", "analytics_visits", "analytics_salts", "analytics_properties", "analytics_ingest_stats", "analytics_rollup_days"} {
		col := "property_id"
		if table == "analytics_properties" {
			col = "id"
		}
		if n := h.count("SELECT count(*) FROM "+table+" WHERE workspace_id = $1 AND "+col+" = $2", tm.ws, id); n != 0 {
			t.Fatalf("%s keeps %d rows", table, n)
		}
	}

	id, key = createProperty(tm.owner, map[string]any{"name": "Range", "domains": []string{"example.com"}})
	if send(key) != http.StatusAccepted {
		t.Fatal("collect")
	}
	h.flushAnalytics()
	today := h.clock.Now().UTC().Format(time.DateOnly)
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties/"+id+"/delete-data", map[string]any{"from": today})
	tm.owner.expect(http.StatusAccepted, "POST", "/v1/analytics/properties/"+id+"/delete-data", map[string]any{"from": today, "to": today})
	done, err = h.srv.DeleteAnalytics(context.Background(), api.AnalyticsDeleteArgs{WorkspaceID: uuid.MustParse(tm.ws), PropertyID: uuid.MustParse(id), From: today, To: today}, time.Minute)
	if err != nil || !done {
		t.Fatalf("range deletion %v %v", done, err)
	}
	if n := h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1 AND property_id = $2", tm.ws, id) +
		h.count("SELECT count(*) FROM analytics_visit_pages WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 0 {
		t.Fatalf("%d events left", n)
	}
	tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id, nil)

	if send(key) != http.StatusAccepted {
		t.Fatal("collect")
	}
	h.flushAnalytics()
	ws := uuid.MustParse(tm.ws)
	if err := h.srv.OperatorDeleteWorkspace(context.Background(), ws); err != nil {
		t.Fatal(err)
	}
	if send(key) != http.StatusNotFound {
		t.Fatal("a deleted workspace's key works")
	}
	if p, err := h.srv.PurgeWorkspace(context.Background(), ws, time.Minute); err != nil || !p.Done {
		t.Fatalf("purge %v %v", p, err)
	}
	if n := h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1", tm.ws) + h.count("SELECT count(*) FROM analytics_visits WHERE workspace_id = $1", tm.ws) +
		h.count("SELECT count(*) FROM analytics_visit_pages WHERE workspace_id = $1", tm.ws); n != 0 {
		t.Fatalf("%d raw rows outlive their workspace", n)
	}
}

func TestAnalyticsMonthlyLimit(t *testing.T) {
	operator := "operator-" + unique("op") + "@example.com"
	h := newHarnessWith(t, func(d *api.Deps) {
		d.Analytics.MonthlyEvents = 3
		d.OperatorEmails = []string{operator}
	})
	tm := newTeam(t, h)
	id, key := createProperty(tm.owner, map[string]any{"name": "Busy", "domains": []string{"example.com"}})
	for i := 0; i < 3; i++ {
		if r := collect(h, key, collectReq{origin: "https://example.com", ip: "203.0.113." + string(rune('1'+i)), body: pageview("https://example.com/")}); r.status != http.StatusAccepted {
			t.Fatalf("collect %d", r.status)
		}
	}
	h.flushAnalytics()
	if r := collect(h, key, collectReq{origin: "https://example.com", ip: "203.0.113.9", body: pageview("https://example.com/")}); r.status != http.StatusAccepted {
		t.Fatalf("over the cap: %d", r.status)
	}
	h.flushAnalytics()
	if n := h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 3 {
		t.Fatalf("%d events stored under a cap of 3", n)
	}
	if n := h.count("SELECT coalesce(sum(events), 0) FROM analytics_ingest_stats WHERE workspace_id = $1 AND reason = 'over_limit'", tm.ws); n != 1 {
		t.Fatalf("%d over the cap", n)
	}
	usage := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties", nil).body["usage"].(map[string]any)
	if usage["limit"].(float64) != 3 || usage["events"].(float64) != 3 || usage["limit_reached_at"] == nil {
		t.Fatalf("usage %v", usage)
	}
	var owner string
	if err := h.st.Pool.QueryRow(context.Background(), "SELECT p.email FROM members m JOIN people p ON p.id = m.person_id WHERE m.workspace_id = $1 AND m.role = 'owner'", tm.ws).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	limitMails := func() int {
		n := 0
		for _, m := range h.mail.to(owner) {
			if strings.Contains(m.Subject, "reached its analytics limit") {
				n++
			}
		}
		return n
	}
	if limitMails() != 1 {
		t.Fatal("the owner was not told about the cap")
	}
	if n := h.count("SELECT count(*) FROM operator_notices WHERE kind = 'analytics_limit' AND detail LIKE '3 analytics events%'"); n == 0 {
		t.Fatal("no operator notice")
	}
	h.flushAnalytics()
	if limitMails() != 1 {
		t.Fatal("the owner was told twice")
	}
	if err := h.srv.OperatorLimitAnalytics(context.Background(), uuid.MustParse(tm.ws), 0); err != nil {
		t.Fatal(err)
	}
	if _, ok := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties", nil).body["usage"].(map[string]any)["limit"]; ok {
		t.Fatal("the operator's override is not used")
	}
}

func TestAnalyticsOff(t *testing.T) {
	h := newHarnessWith(t, func(d *api.Deps) { d.Analytics.Off = true })
	tm := newTeam(t, h)
	tm.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/analytics/properties", nil)
	if r := collect(h, "yuva_ak_x", collectReq{origin: "https://example.com", ip: "203.0.113.7", body: pageview("https://example.com/")}); r.status != http.StatusNotFound {
		t.Fatalf("collect %d", r.status)
	}
	for _, name := range []string{"/yuva-analytics.js", "/yuva-analytics-consent.js"} {
		res, err := http.Get(h.url + name)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("%s %d", name, res.StatusCode)
		}
	}
}

func TestAnalyticsScriptIsServed(t *testing.T) {
	h := newHarness(t)
	for _, name := range []string{"/yuva-analytics.js", "/yuva-analytics-consent.js"} {
		res, err := http.Get(h.url + name)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "text/javascript; charset=utf-8" ||
			res.Header.Get("Access-Control-Allow-Origin") != "*" || res.Header.Get("Set-Cookie") != "" {
			t.Fatalf("%s %d %v", name, res.StatusCode, res.Header)
		}
	}
}

func TestAnalyticsDeletedKeyStopsOnEveryProcess(t *testing.T) {
	h := newHarness(t)
	other := newHarness(t)
	tm := newTeam(t, h)
	send := func(on *harness, key string) int {
		return collect(on, key, collectReq{origin: "https://example.com", ip: "203.0.113.7", body: pageview("https://example.com/")}).status
	}
	refused := func(on *harness, key string) bool {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if send(on, key) == http.StatusNotFound {
				return true
			}
			time.Sleep(20 * time.Millisecond)
		}
		return false
	}
	id, key := createProperty(tm.owner, map[string]any{"name": "Site", "domains": []string{"example.com"}})
	if send(other, key) != http.StatusAccepted {
		t.Fatal("the second process refused a live key")
	}
	tm.owner.expect(http.StatusAccepted, "DELETE", "/v1/analytics/properties/"+id, map[string]any{"name": "Site"})
	if !refused(other, key) {
		t.Fatal("the second process still takes a deleted property's key")
	}

	_, key = createProperty(tm.owner, map[string]any{"name": "Second", "domains": []string{"example.com"}})
	if send(other, key) != http.StatusAccepted {
		t.Fatal("the second process refused a live key")
	}
	if err := h.srv.OperatorDeleteWorkspace(context.Background(), uuid.MustParse(tm.ws)); err != nil {
		t.Fatal(err)
	}
	if !refused(other, key) {
		t.Fatal("the second process still takes a deleted workspace's key")
	}
}

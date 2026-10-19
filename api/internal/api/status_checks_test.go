package api_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/productdevbook/yuva/api/internal/api"
)

func roundsDropped(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	prefix := "yuva_status_rounds_dropped_total "
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if v, ok := strings.CutPrefix(line, prefix); ok {
			return v
		}
	}
	return "0"
}

type checkEnv struct {
	h        *harness
	st       statusTeam
	resolver staticResolver
}

func newCheckEnv(t *testing.T, allowPrivate bool, canaries []string, configure func(*api.Deps)) checkEnv {
	t.Helper()
	res := staticResolver{}
	h := newHarnessWith(t, func(d *api.Deps) {
		d.Webhooks.Resolver = res
		d.Status.AllowPrivate = allowPrivate
		d.Status.CanaryURLs = canaries
		if configure != nil {
			configure(d)
		}
	})
	st := newStatusTeam(t, h)
	t.Cleanup(func() {
		_, _ = h.st.Pool.Exec(context.Background(), "DELETE FROM status_monitors WHERE workspace_id = $1", st.ws)
	})
	return checkEnv{h: h, st: st, resolver: res}
}

func canary(t *testing.T) []string {
	return []string{newCanary(t).url}
}

type testCanary struct {
	url  string
	hits atomic.Int32
	down atomic.Bool
}

// newCanary answers 204 until down is set, and then closes the connection without an answer.
func newCanary(t *testing.T) *testCanary {
	c := &testCanary{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		c.hits.Add(1)
		if c.down.Load() {
			if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
				_ = conn.Close()
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	c.url = srv.URL
	return c
}

func failingApp(t *testing.T, failing *atomic.Bool) string {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if failing.Load() {
			w.WriteHeader(http.StatusBadGateway)
		}
	}))
	t.Cleanup(app.Close)
	return "http://app.example.com:" + hostPort(t, app.URL) + "/"
}

func hostPort(t *testing.T, raw string) string {
	t.Helper()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(strings.TrimPrefix(raw, "http://"), "https://"))
	if err != nil {
		t.Fatal(err)
	}
	return port
}

// runChecks starts a round and runs this workspace's checks of it, as the queue would, and returns
// the rounds they ran in.
func (e checkEnv) runChecks(t *testing.T) []string {
	t.Helper()
	ctx := context.Background()
	if err := e.h.srv.ScheduleStatusChecks(ctx); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var rounds []string
	for _, j := range e.h.jobs("status_check", e.st.ws) {
		var a api.StatusCheckArgs
		if err := json.Unmarshal(j.args, &a); err != nil {
			t.Fatal(err)
		}
		if err := e.h.srv.RunStatusCheck(ctx, a); err != nil {
			t.Fatal(err)
		}
		e.h.dropJob(j.id)
		if id := a.RoundID.String(); !seen[id] {
			seen[id] = true
			rounds = append(rounds, id)
		}
	}
	return rounds
}

func (e checkEnv) settle(t *testing.T, round string) {
	t.Helper()
	ctx := context.Background()
	var raw []byte
	if err := e.h.st.Pool.QueryRow(ctx, "SELECT args FROM river_job WHERE kind = 'status_round' AND args->>'round_id' = $1", round).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var a api.StatusRoundArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatal(err)
	}
	if err := e.h.srv.SettleStatusRound(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, err := e.h.st.Pool.Exec(ctx, "DELETE FROM river_job WHERE args->>'round_id' = $1", round); err != nil {
		t.Fatal(err)
	}
}

// runRound runs one sweep's checks of this workspace and judges their rounds without moving the clock.
func (e checkEnv) runRound(t *testing.T) int {
	t.Helper()
	rounds := e.runChecks(t)
	for _, id := range rounds {
		e.settle(t, id)
	}
	return len(rounds)
}

// round runs one sweep's checks and judges them, then moves the clock past every monitor's next slot.
func (e checkEnv) round(t *testing.T) {
	t.Helper()
	e.runRound(t)
	e.h.clock.Advance(61 * time.Second)
}

func (e checkEnv) failures(t *testing.T, id string) float64 {
	t.Helper()
	return e.st.monitorBody(t, id).body["consecutive_failures"].(float64)
}

func (e checkEnv) checks(t *testing.T, id string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, c := range e.st.owner.expect(http.StatusOK, "GET", e.st.path("/monitors/"+id+"/checks"), nil).body["items"].([]any) {
		out = append(out, c.(map[string]any))
	}
	return out
}

func (e checkEnv) create(t *testing.T, body map[string]any) response {
	t.Helper()
	body["component_id"] = e.st.api
	if body["name"] == nil {
		body["name"] = "Check"
	}
	return e.st.owner.expect(http.StatusCreated, "POST", e.st.path("/monitors"), body)
}

func TestStatusHTTPMonitorFailsAndRecovers(t *testing.T) {
	var failing atomic.Bool
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if failing.Load() {
			w.WriteHeader(http.StatusInternalServerError)
		}
		_, _ = w.Write([]byte("status: ok"))
	}))
	defer app.Close()
	e := newCheckEnv(t, true, canary(t), nil)
	e.resolver["app.example.com"] = "127.0.0.1"
	m := e.create(t, map[string]any{"kind": "http", "http": map[string]any{"url": "http://app.example.com:" + hostPort(t, app.URL) + "/health", "keyword": "ok"}})
	if m.body["token"] != nil || m.body["url"] != nil || m.body["failure_threshold"] != 3.0 || m.body["recovery_threshold"] != 2.0 ||
		m.body["interval"] != 60.0 || m.body["timeout"] != 10.0 || m.body["next_check_at"] == nil {
		t.Fatalf("created %s", m.raw)
	}
	if hs, _ := m.body["http"].(map[string]any); hs["method"] != "GET" || hs["expected_status_min"] != 200.0 || hs["cert_warn_days"] != 14.0 {
		t.Fatalf("http settings %s", m.raw)
	}
	id := m.str("id")
	e.st.owner.expectProblem(http.StatusConflict, "monitor_kind", "POST", e.st.path("/monitors/"+id+"/token"), nil)

	e.round(t)
	if got := e.st.monitorBody(t, id); got.str("state") != "up" {
		t.Fatalf("first round %s", got.raw)
	}
	failing.Store(true)
	for i := range 2 {
		e.round(t)
		if got := e.st.monitorBody(t, id); got.str("state") != "up" || got.body["consecutive_failures"] != float64(i+1) {
			t.Fatalf("failure %d below the threshold %s", i+1, got.raw)
		}
	}
	e.round(t)
	down := e.st.monitorBody(t, id)
	if down.str("state") != "down" || down.str("incident_id") == "" || !strings.Contains(down.str("last_message"), "answered 500") {
		t.Fatalf("after the threshold %s", down.raw)
	}
	if got := e.st.componentState(t, e.st.api); got != "major_outage" {
		t.Fatalf("component %s", got)
	}
	failing.Store(false)
	e.round(t)
	if got := e.st.monitorBody(t, id); got.str("state") != "down" {
		t.Fatalf("recovered after one success %s", got.raw)
	}
	e.round(t)
	if got := e.st.monitorBody(t, id); got.str("state") != "up" {
		t.Fatalf("after the recovery threshold %s", got.raw)
	}
	if got := e.st.componentState(t, e.st.api); got != "operational" {
		t.Fatalf("component after recovery %s", got)
	}
	checks := e.checks(t, id)
	if len(checks) != 6 || checks[0]["source"] != "http" || checks[0]["status_code"] != 200.0 || checks[0]["duration_ms"] == nil ||
		checks[2]["ok"] != false || checks[2]["status_code"] != 500.0 {
		t.Fatalf("checks %v", checks)
	}
	sum := e.st.owner.expect(http.StatusOK, "GET", e.st.path("/monitors/"+id+"/checks/summary"), nil)
	if sum.body["checks"] != 6.0 || sum.body["failures"] != 3.0 || sum.body["uptime"] != 50.0 || sum.body["p50_ms"] == nil ||
		len(sum.body["buckets"].([]any)) != 96 || sum.body["bucket_seconds"] != 900.0 {
		t.Fatalf("summary %s", sum.raw)
	}
	week := e.st.owner.expect(http.StatusOK, "GET", e.st.path("/monitors/"+id+"/checks/summary?range=7d"), nil)
	if len(week.body["buckets"].([]any)) != 168 || week.body["checks"] != 6.0 {
		t.Fatalf("week %s", week.raw)
	}

	other := newStatusTeam(t, e.h)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", other.path("/monitors/"+id), nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", e.st.path("/monitors/"+id+"/checks/summary"), nil)
}

func TestStatusHTTPMonitorKeywordAndHeaders(t *testing.T) {
	var (
		mu   sync.Mutex
		seen http.Header
	)
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = r.Header.Clone()
		mu.Unlock()
		_, _ = w.Write([]byte("all good"))
	}))
	defer app.Close()
	e := newCheckEnv(t, true, canary(t), nil)
	e.resolver["app.example.com"] = "127.0.0.1"
	target := "http://app.example.com:" + hostPort(t, app.URL) + "/"
	e.st.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", e.st.path("/monitors"), map[string]any{"kind": "http", "name": "x", "component_id": e.st.api,
		"http": map[string]any{"url": target, "headers": []any{map[string]any{"name": "Host", "value": "x"}}}})
	e.st.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", e.st.path("/monitors"), map[string]any{"kind": "http", "name": "x", "component_id": e.st.api,
		"http": map[string]any{"url": target, "headers": []any{map[string]any{"name": "X-Key"}}}})
	e.st.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", e.st.path("/monitors"), map[string]any{"kind": "http", "name": "x", "component_id": e.st.api,
		"interval": 30, "http": map[string]any{"url": target}})
	m := e.create(t, map[string]any{"kind": "http", "failure_threshold": 1, "http": map[string]any{"url": target, "keyword": "missing",
		"headers": []any{map[string]any{"name": "authorization", "value": "Bearer s3cret-value"}}}})
	id := m.str("id")
	if strings.Contains(string(m.raw), "s3cret") || !strings.Contains(string(m.raw), `"header_names":["Authorization"]`) {
		t.Fatalf("created %s", m.raw)
	}
	var sealed []byte
	if err := e.h.st.Pool.QueryRow(context.Background(), "SELECT http_headers FROM status_monitors WHERE id = $1", id).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	if len(sealed) == 0 || strings.Contains(string(sealed), "s3cret") {
		t.Fatal("header values are not sealed")
	}
	e.round(t)
	got := e.st.monitorBody(t, id)
	if got.str("state") != "down" || !strings.Contains(got.str("last_message"), "keyword") || strings.Contains(string(got.raw), "s3cret") {
		t.Fatalf("keyword missing %s", got.raw)
	}
	mu.Lock()
	if seen.Get("Authorization") != "Bearer s3cret-value" {
		t.Fatalf("sent headers %v", seen)
	}
	mu.Unlock()

	up := e.st.owner.expect(http.StatusOK, "PATCH", e.st.path("/monitors/"+id), map[string]any{"recovery_threshold": 1, "http": map[string]any{"keyword": "good",
		"headers": []any{map[string]any{"name": "Authorization"}, map[string]any{"name": "X-Extra", "value": "1"}}}})
	if strings.Contains(string(up.raw), "s3cret") || !strings.Contains(string(up.raw), `"header_names":["Authorization","X-Extra"]`) {
		t.Fatalf("updated %s", up.raw)
	}
	e.round(t)
	if got := e.st.monitorBody(t, id); got.str("state") != "up" {
		t.Fatalf("keyword found %s", got.raw)
	}
	mu.Lock()
	defer mu.Unlock()
	if seen.Get("Authorization") != "Bearer s3cret-value" || seen.Get("X-Extra") != "1" {
		t.Fatalf("kept header value %v", seen)
	}
}

func TestStatusOutsideCheckAddresses(t *testing.T) {
	e := newCheckEnv(t, false, canary(t), nil)
	e.resolver["private.example.com"] = "10.0.0.5"
	e.resolver["meta.example.com"] = "169.254.169.254"
	e.resolver["flip.example.com"] = "93.184.215.14"
	for _, body := range []map[string]any{
		{"kind": "http", "http": map[string]any{"url": "http://private.example.com/"}},
		{"kind": "http", "http": map[string]any{"url": "http://meta.example.com/"}},
		{"kind": "http", "http": map[string]any{"url": "http://127.0.0.1:8080/"}},
		{"kind": "http", "http": map[string]any{"url": "http://localhost/"}},
		{"kind": "http", "http": map[string]any{"url": "http://[fd00:ec2::254]/"}},
		{"kind": "tcp", "tcp": map[string]any{"host": "10.1.2.3", "port": 22}},
		{"kind": "tcp", "tcp": map[string]any{"host": "private.example.com", "port": 5432}},
	} {
		body["name"], body["component_id"] = "Refused", e.st.api
		e.st.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", e.st.path("/monitors"), body)
	}
	for _, host := range []string{"flip.example.com:443", "[2606:4700::1111]", "[2606:4700::1111]:443", "2606:4700::1111]", "fe80::1%eth0", "flip.example.com:"} {
		r := e.st.owner.expect(http.StatusBadRequest, "POST", e.st.path("/monitors"), map[string]any{"kind": "tcp", "name": "Port", "component_id": e.st.api,
			"tcp": map[string]any{"host": host, "port": 443}})
		if r.str("code") != "validation_failed" || !strings.Contains(r.str("detail"), "tcp.host") {
			t.Fatalf("tcp host %q: %s", host, r.raw)
		}
	}
	v6 := e.create(t, map[string]any{"kind": "tcp", "paused": true, "tcp": map[string]any{"host": "2606:4700::1111", "port": 443}})
	if tcp, _ := v6.body["tcp"].(map[string]any); tcp["host"] != "2606:4700::1111" || tcp["port"] != 443.0 {
		t.Fatalf("bare IPv6 host %s", v6.raw)
	}
	var stored string
	if err := e.h.st.Pool.QueryRow(context.Background(), "SELECT target FROM status_monitors WHERE id = $1", v6.str("id")).Scan(&stored); err != nil || stored != "[2606:4700::1111]:443" {
		t.Fatalf("stored IPv6 target %q %v", stored, err)
	}
	m := e.create(t, map[string]any{"kind": "tcp", "failure_threshold": 1, "tcp": map[string]any{"host": "flip.example.com", "port": 443}})
	r := e.st.owner.expect(http.StatusBadRequest, "PATCH", e.st.path("/monitors/"+m.str("id")), map[string]any{"tcp": map[string]any{"host": "flip.example.com:22", "port": 22}})
	if !strings.Contains(r.str("detail"), "tcp.host") {
		t.Fatalf("tcp host with a port on update %s", r.raw)
	}
	e.st.owner.expectProblem(http.StatusBadRequest, "validation_failed", "PATCH", e.st.path("/monitors/"+m.str("id")), map[string]any{"tcp": map[string]any{"host": "private.example.com", "port": 1}})
	e.resolver["flip.example.com"] = "10.0.0.5"
	e.round(t)
	got := e.st.monitorBody(t, m.str("id"))
	if got.str("state") != "down" || !strings.Contains(got.str("last_message"), "refused address 10.0.0.5") {
		t.Fatalf("resolved to a private address at check time %s", got.raw)
	}

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://meta.example.com/latest/meta-data/", http.StatusFound)
	}))
	defer redirect.Close()
	p := newCheckEnv(t, true, canary(t), nil)
	p.resolver["meta.example.com"] = "169.254.169.254"
	p.resolver["app.example.com"] = "127.0.0.1"
	p.st.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", p.st.path("/monitors"), map[string]any{"kind": "http", "name": "x",
		"component_id": p.st.api, "http": map[string]any{"url": "http://169.254.169.254/"}})
	follow := p.create(t, map[string]any{"kind": "http", "failure_threshold": 1, "http": map[string]any{"url": "http://app.example.com:" + hostPort(t, redirect.URL) + "/", "follow_redirects": true}})
	stay := p.create(t, map[string]any{"kind": "http", "failure_threshold": 1, "http": map[string]any{"url": "http://app.example.com:" + hostPort(t, redirect.URL) + "/"}})
	p.round(t)
	if got := p.st.monitorBody(t, follow.str("id")); got.str("state") != "down" || !strings.Contains(got.str("last_message"), "refused") {
		t.Fatalf("redirect into metadata %s", got.raw)
	}
	if got := p.st.monitorBody(t, stay.str("id")); got.str("state") != "down" || !strings.Contains(got.str("last_message"), "answered 302") {
		t.Fatalf("redirect not followed %s", got.raw)
	}
}

func TestStatusRoundCanaries(t *testing.T) {
	var failing atomic.Bool
	c := newCanary(t)
	logs := &logBuffer{}
	e := newCheckEnv(t, true, []string{c.url}, func(d *api.Deps) { d.Log = slog.New(slog.NewTextHandler(logs, nil)) })
	e.resolver["app.example.com"] = "127.0.0.1"
	target := failingApp(t, &failing)
	id := e.create(t, map[string]any{"kind": "http", "failure_threshold": 10, "http": map[string]any{"url": target}}).str("id")

	e.round(t)
	if n := c.hits.Load(); n != 0 {
		t.Fatalf("canaries probed %d times for a round without failures", n)
	}
	failing.Store(true)
	e.runRound(t)
	if n := c.hits.Load(); n != 1 || e.failures(t, id) != 1 {
		t.Fatalf("canaries probed %d times, failures %v", n, e.failures(t, id))
	}
	other := e.create(t, map[string]any{"kind": "http", "failure_threshold": 10, "http": map[string]any{"url": target}}).str("id")
	e.runRound(t)
	if n := c.hits.Load(); n != 1 || e.failures(t, other) != 1 {
		t.Fatalf("canary verdict not kept for a minute: probed %d times, failures %v", n, e.failures(t, other))
	}
	e.st.owner.expect(http.StatusNoContent, "DELETE", e.st.path("/monitors/"+other), nil)
	e.h.clock.Advance(61 * time.Second)

	c.down.Store(true)
	before := roundsDropped(t)
	e.round(t)
	if e.failures(t, id) != 1 || len(e.checks(t, id)) != 2 || roundsDropped(t) == before {
		t.Fatalf("round counted while every canary failed: failures %v, drops %v -> %v", e.failures(t, id), before, roundsDropped(t))
	}

	e.h.clock.Advance(time.Hour)
	e.round(t)
	if e.failures(t, id) != 2 || strings.Count(logs.String(), "status canaries are unreachable") != 1 {
		t.Fatalf("an hour after the canaries last answered: failures %v, warnings %d", e.failures(t, id), strings.Count(logs.String(), "status canaries are unreachable"))
	}
	e.round(t)
	if e.failures(t, id) != 3 || strings.Count(logs.String(), "status canaries are unreachable") != 1 {
		t.Fatalf("within the hour of the warning: failures %v, warnings %d", e.failures(t, id), strings.Count(logs.String(), "status canaries are unreachable"))
	}
	c.down.Store(false)
	e.round(t)
	c.down.Store(true)
	e.round(t)
	if e.failures(t, id) != 4 {
		t.Fatalf("guard not armed again after the canaries answered: failures %v", e.failures(t, id))
	}
}

func TestStatusCanariesUnreachableOrOff(t *testing.T) {
	for _, off := range []bool{false, true} {
		var failing atomic.Bool
		failing.Store(true)
		c := newCanary(t)
		c.down.Store(true)
		logs := &logBuffer{}
		e := newCheckEnv(t, true, []string{c.url}, func(d *api.Deps) {
			d.Log = slog.New(slog.NewTextHandler(logs, nil))
			d.Status.CanariesOff = off
		})
		e.resolver["app.example.com"] = "127.0.0.1"
		id := e.create(t, map[string]any{"kind": "http", "failure_threshold": 1, "http": map[string]any{"url": failingApp(t, &failing)}}).str("id")
		e.round(t)
		if got := e.st.monitorBody(t, id); got.str("state") != "down" {
			t.Fatalf("off=%v: failure not counted while the canaries never answered %s", off, got.raw)
		}
		wantHits, wantWarnings := int32(1), 1
		if off {
			wantHits, wantWarnings = 0, 0
		}
		if c.hits.Load() != wantHits || strings.Count(logs.String(), "status canaries are unreachable") != wantWarnings {
			t.Fatalf("off=%v: canaries probed %d times, warnings %d", off, c.hits.Load(), strings.Count(logs.String(), "status canaries are unreachable"))
		}
	}
}

func TestStatusRoundDecisionKept(t *testing.T) {
	var failing atomic.Bool
	failing.Store(true)
	c := newCanary(t)
	e := newCheckEnv(t, true, []string{c.url}, nil)
	e.resolver["app.example.com"] = "127.0.0.1"
	id := e.create(t, map[string]any{"kind": "http", "failure_threshold": 1, "http": map[string]any{"url": failingApp(t, &failing)}}).str("id")
	ctx := context.Background()

	rounds := e.runChecks(t)
	if len(rounds) != 1 {
		t.Fatalf("rounds %v", rounds)
	}
	if _, err := e.h.st.Pool.Exec(ctx, "UPDATE status_checks SET round_dropped = true WHERE round_id = $1", rounds[0]); err != nil {
		t.Fatal(err)
	}
	e.settle(t, rounds[0])
	if got := e.st.monitorBody(t, id); got.str("state") != "pending" || len(e.checks(t, id)) != 0 || c.hits.Load() != 0 {
		t.Fatalf("a retried round judged again: canaries probed %d times %s", c.hits.Load(), got.raw)
	}

	e.h.clock.Advance(61 * time.Second)
	rounds = e.runChecks(t)
	e.settle(t, rounds[0])
	var stored *bool
	if err := e.h.st.Pool.QueryRow(ctx, "SELECT round_dropped FROM status_checks WHERE round_id = $1", rounds[0]).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if got := e.st.monitorBody(t, id); got.str("state") != "down" || stored == nil || *stored {
		t.Fatalf("judgement %v %s", stored, got.raw)
	}
}

func TestStatusLateCheckJudgedAlone(t *testing.T) {
	var failing atomic.Bool
	e := newCheckEnv(t, true, canary(t), nil)
	e.resolver["app.example.com"] = "127.0.0.1"
	id := e.create(t, map[string]any{"kind": "http", "recovery_threshold": 1, "http": map[string]any{"url": failingApp(t, &failing)}}).str("id")
	ctx := context.Background()
	if err := e.h.srv.ScheduleStatusChecks(ctx); err != nil {
		t.Fatal(err)
	}
	jobs := e.h.jobs("status_check", e.st.ws)
	if len(jobs) != 1 {
		t.Fatalf("jobs %d", len(jobs))
	}
	var a api.StatusCheckArgs
	if err := json.Unmarshal(jobs[0].args, &a); err != nil {
		t.Fatal(err)
	}
	a.Deadline = e.h.clock.Now().Add(5 * time.Second)
	if err := e.h.srv.RunStatusCheck(ctx, a); err != nil {
		t.Fatal(err)
	}
	e.h.dropJob(jobs[0].id)
	var own string
	if err := e.h.st.Pool.QueryRow(ctx, "SELECT round_id::text FROM status_checks WHERE monitor_id = $1 AND pending", id).Scan(&own); err != nil {
		t.Fatalf("late check left no pending result: %v", err)
	}
	if own == a.RoundID.String() {
		t.Fatal("late check kept its round")
	}
	e.settle(t, a.RoundID.String())
	e.settle(t, own)
	if got := e.st.monitorBody(t, id); got.str("state") != "up" || len(e.checks(t, id)) != 1 {
		t.Fatalf("late check not counted %s", got.raw)
	}
}

func TestStatusClaimFitsWorkers(t *testing.T) {
	defer api.SetStatusClaimBudget(25)()
	var failing atomic.Bool
	e := newCheckEnv(t, true, canary(t), nil)
	e.resolver["app.example.com"] = "127.0.0.1"
	target := failingApp(t, &failing)
	first := e.create(t, map[string]any{"kind": "http", "timeout": 20, "http": map[string]any{"url": target}}).str("id")
	second := e.create(t, map[string]any{"kind": "http", "timeout": 20, "http": map[string]any{"url": target}}).str("id")
	e.runRound(t)
	if len(e.checks(t, first))+len(e.checks(t, second)) != 1 {
		t.Fatalf("claimed more timeout than the budget: %d and %d checks", len(e.checks(t, first)), len(e.checks(t, second)))
	}
	e.runRound(t)
	if len(e.checks(t, first)) != 1 || len(e.checks(t, second)) != 1 {
		t.Fatalf("the check left over is not claimed next: %d and %d checks", len(e.checks(t, first)), len(e.checks(t, second)))
	}
}

func selfSigned(t *testing.T, name string, notAfter time.Time) (tls.Certificate, *x509.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: notAfter,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true, IsCA: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, cert
}

func TestStatusHTTPMonitorCertificate(t *testing.T) {
	pair, cert := selfSigned(t, "tls.example.com", time.Now().Add(5*24*time.Hour))
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{pair}}
	srv.StartTLS()
	defer srv.Close()

	e := newCheckEnv(t, true, canary(t), func(d *api.Deps) { d.Status.RootCAs = pool })
	e.resolver["tls.example.com"] = "127.0.0.1"
	id := e.create(t, map[string]any{"kind": "http", "failure_threshold": 1, "recovery_threshold": 1,
		"http": map[string]any{"url": "https://tls.example.com:" + hostPort(t, srv.URL) + "/"}}).str("id")
	e.round(t)
	got := e.st.monitorBody(t, id)
	if got.str("state") != "down" || !strings.Contains(got.str("last_message"), "certificate expires on") || got.str("cert_expires_at") == "" {
		t.Fatalf("expiring certificate %s", got.raw)
	}
	e.st.owner.expect(http.StatusOK, "PATCH", e.st.path("/monitors/"+id), map[string]any{"http": map[string]any{"cert_warn_days": 3}})
	e.round(t)
	if got := e.st.monitorBody(t, id); got.str("state") != "up" {
		t.Fatalf("certificate outside the warning %s", got.raw)
	}
}

func TestStatusHTTPMonitorCertificateOfFirstHop(t *testing.T) {
	pool := x509.NewCertPool()
	urls := map[string]string{}
	start := func(name string, notAfter time.Time, next string) *httptest.Server {
		pair, cert := selfSigned(t, name, notAfter)
		pool.AddCert(cert)
		srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/go" {
				http.Redirect(w, r, urls[next]+"/", http.StatusFound)
				return
			}
			_, _ = w.Write([]byte("ok"))
		}))
		srv.TLS = &tls.Config{Certificates: []tls.Certificate{pair}}
		srv.StartTLS()
		t.Cleanup(srv.Close)
		urls[name] = "https://" + name + ":" + hostPort(t, srv.URL)
		return srv
	}
	start("long.example.com", time.Now().Add(90*24*time.Hour), "short.example.com")
	start("short.example.com", time.Now().Add(5*24*time.Hour), "long.example.com")

	e := newCheckEnv(t, true, canary(t), func(d *api.Deps) { d.Status.RootCAs = pool })
	e.resolver["long.example.com"] = "127.0.0.1"
	e.resolver["short.example.com"] = "127.0.0.1"
	long := e.create(t, map[string]any{"kind": "http", "failure_threshold": 1, "recovery_threshold": 1,
		"http": map[string]any{"url": urls["long.example.com"] + "/go", "follow_redirects": true}}).str("id")
	short := e.create(t, map[string]any{"kind": "http", "failure_threshold": 1, "recovery_threshold": 1,
		"http": map[string]any{"url": urls["short.example.com"] + "/go", "follow_redirects": true}}).str("id")
	e.round(t)
	expires := func(r response) time.Time {
		at, err := time.Parse(time.RFC3339Nano, r.str("cert_expires_at"))
		if err != nil {
			t.Fatalf("cert_expires_at %s", r.raw)
		}
		return at
	}
	if got := e.st.monitorBody(t, long); got.str("state") != "up" || expires(got).Before(time.Now().Add(60*24*time.Hour)) {
		t.Fatalf("redirected to a host whose certificate expires soon %s", got.raw)
	}
	if got := e.st.monitorBody(t, short); got.str("state") != "down" || !strings.Contains(got.str("last_message"), "certificate expires on") ||
		expires(got).After(time.Now().Add(10*24*time.Hour)) {
		t.Fatalf("expiring certificate before a redirect %s", got.raw)
	}
}

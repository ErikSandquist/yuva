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

func roundsDropped(t *testing.T, reason string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	prefix := `yuva_status_rounds_dropped_total{reason="` + reason + `"} `
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

// canary answers 204, as a reachable canary does.
func canary(t *testing.T) []string {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	t.Cleanup(srv.Close)
	return []string{srv.URL}
}

func closedURL(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return "http://" + addr + "/"
}

func hostPort(t *testing.T, raw string) string {
	t.Helper()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(strings.TrimPrefix(raw, "http://"), "https://"))
	if err != nil {
		t.Fatal(err)
	}
	return port
}

// round runs one sweep's checks of this workspace and judges the round, as the queue would, then
// moves the clock past every monitor's next slot.
func (e checkEnv) round(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if err := e.h.srv.ScheduleStatusChecks(ctx); err != nil {
		t.Fatal(err)
	}
	rounds := map[string]bool{}
	for _, j := range e.h.jobs("status_check", e.st.ws) {
		var a api.StatusCheckArgs
		if err := json.Unmarshal(j.args, &a); err != nil {
			t.Fatal(err)
		}
		if err := e.h.srv.RunStatusCheck(ctx, a); err != nil {
			t.Fatal(err)
		}
		e.h.dropJob(j.id)
		rounds[a.RoundID.String()] = true
	}
	for id := range rounds {
		var raw []byte
		if err := e.h.st.Pool.QueryRow(ctx, "SELECT args FROM river_job WHERE kind = 'status_round' AND args->>'round_id' = $1", id).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var a api.StatusRoundArgs
		if err := json.Unmarshal(raw, &a); err != nil {
			t.Fatal(err)
		}
		if err := e.h.srv.SettleStatusRound(ctx, a); err != nil {
			t.Fatal(err)
		}
		if _, err := e.h.st.Pool.Exec(ctx, "DELETE FROM river_job WHERE args->>'round_id' = $1", id); err != nil {
			t.Fatal(err)
		}
	}
	e.h.clock.Advance(61 * time.Second)
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
	m := e.create(t, map[string]any{"kind": "tcp", "failure_threshold": 1, "tcp": map[string]any{"host": "flip.example.com", "port": 443}})
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

func TestStatusRoundDropped(t *testing.T) {
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer broken.Close()
	target := "http://app.example.com:" + hostPort(t, broken.URL) + "/"

	e := newCheckEnv(t, true, []string{closedURL(t)}, nil)
	e.resolver["app.example.com"] = "127.0.0.1"
	id := e.create(t, map[string]any{"kind": "http", "failure_threshold": 1, "http": map[string]any{"url": target}}).str("id")
	before := roundsDropped(t, "canaries")
	e.round(t)
	if got := e.st.monitorBody(t, id); got.str("state") != "pending" || len(e.checks(t, id)) != 0 {
		t.Fatalf("failure counted while every canary failed %s", got.raw)
	}
	if after := roundsDropped(t, "canaries"); after == before {
		t.Fatalf("canary drops %v -> %v", before, after)
	}

	m := newCheckEnv(t, true, canary(t), nil)
	m.resolver["app.example.com"] = "127.0.0.1"
	var ids []string
	for range 10 {
		ids = append(ids, m.create(t, map[string]any{"kind": "http", "failure_threshold": 1, "http": map[string]any{"url": target}}).str("id"))
	}
	before = roundsDropped(t, "majority")
	m.round(t)
	for _, id := range ids {
		if got := m.st.monitorBody(t, id); got.str("state") != "pending" {
			t.Fatalf("failure counted in a round where most checks failed %s", got.raw)
		}
	}
	if after := roundsDropped(t, "majority"); after == before {
		t.Fatalf("majority drops %v -> %v", before, after)
	}
	m.st.owner.expect(http.StatusOK, "PATCH", m.st.path("/monitors/"+ids[0]), map[string]any{"paused": true})
	for _, id := range ids[2:] {
		m.st.owner.expect(http.StatusNoContent, "DELETE", m.st.path("/monitors/"+id), nil)
	}
	m.round(t)
	if got := m.st.monitorBody(t, ids[1]); got.str("state") != "down" {
		t.Fatalf("a lone failure is counted %s", got.raw)
	}
}

func TestStatusHTTPMonitorCertificate(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "tls.example.com"}, DNSNames: []string{"tls.example.com"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(5 * 24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true, IsCA: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
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

package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func ok(context.Context) error { return nil }

func failing(err error) func(context.Context) error {
	return func(context.Context) error { return err }
}

func TestStatusFromChecks(t *testing.T) {
	for _, c := range []struct {
		name   string
		checks []Check
		want   Status
	}{
		{"no checks", nil, Healthy},
		{"all healthy", []Check{{Name: "db", Run: ok}, {Name: "cache", Run: ok}}, Healthy},
		{"critical down", []Check{{Name: "db", Run: failing(errors.New("refused"))}, {Name: "cache", Run: ok}}, Down},
		{"optional down", []Check{{Name: "db", Run: ok}, {Name: "cache", Optional: true, Run: failing(errors.New("refused"))}}, Degraded},
		{"degraded", []Check{{Name: "db", Run: failing(fmt.Errorf("slow replica: %w", ErrDegraded))}}, Degraded},
	} {
		h := New(Config{Service: "api"})
		for _, ch := range c.checks {
			if err := h.Register(ch); err != nil {
				t.Fatal(err)
			}
		}
		if got := h.Run(context.Background()).Status; got != c.want {
			t.Fatalf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

func TestChecksRunAtOnceWithinTheirTimeouts(t *testing.T) {
	h := New(Config{Service: "api"})
	hang := func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	ignores := func(context.Context) error { time.Sleep(2 * time.Second); return nil }
	_ = h.Register(Check{Name: "db", Timeout: 50 * time.Millisecond, Run: hang})
	_ = h.Register(Check{Name: "stuck", Timeout: 50 * time.Millisecond, Optional: true, Run: ignores})
	_ = h.Register(Check{Name: "panics", Optional: true, Run: func(context.Context) error { panic("boom") }})
	start := time.Now()
	r := h.Run(context.Background())
	if time.Since(start) > time.Second {
		t.Fatalf("checks took %s", time.Since(start))
	}
	if r.Status != Down || r.Checks[1].Status != Down || r.Checks[1].Message != "no answer within 50ms" || r.Checks[2].Status != Down ||
		!strings.Contains(r.Checks[2].Message, "panicked") {
		t.Fatalf("report %+v", r)
	}
}

func TestRegisterRefuses(t *testing.T) {
	h := New(Config{Service: "api"})
	if h.Register(Check{Name: " ", Run: ok}) == nil || h.Register(Check{Name: "db"}) == nil {
		t.Fatal("a check without a name or Run was taken")
	}
	_ = h.Register(Check{Name: "db", Run: ok})
	if !errors.Is(h.Register(Check{Name: "db", Run: ok}), ErrDuplicate) {
		t.Fatal("a second check of one name was taken")
	}
	for i := 1; i < MaxChecks; i++ {
		if err := h.Register(Check{Name: fmt.Sprint("c", i), Run: ok}); err != nil {
			t.Fatal(err)
		}
	}
	if !errors.Is(h.Register(Check{Name: "extra", Run: ok}), ErrTooManyChecks) {
		t.Fatal("a 51st check was taken")
	}
}

func TestHandlerServesHealthJSON(t *testing.T) {
	var runs atomic.Int32
	down := atomic.Bool{}
	h := New(Config{Service: "billing", Version: "1.4.2", CacheFor: -1})
	_ = h.Register(Check{Name: "postgres", Kind: Database, Run: func(context.Context) error {
		runs.Add(1)
		if down.Load() {
			return errors.New("dial tcp: connection refused " + strings.Repeat("x", 300))
		}
		return nil
	}})
	_ = h.Register(Check{Name: "redis", Kind: Cache, Optional: true, Run: ok})
	srv := httptest.NewServer(h.Handler())
	defer srv.Close()
	get := func() (int, map[string]any, string) {
		resp, err := http.Get(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var doc map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, doc, resp.Header.Get("Content-Type")
	}
	code, doc, ctype := get()
	if code != 200 || ctype != "application/health+json" || doc["status"] != "pass" || doc["releaseId"] != "1.4.2" || doc["version"] != "1" ||
		doc["serviceId"] != "billing" {
		t.Fatalf("%d %s %v", code, ctype, doc)
	}
	pg := doc["checks"].(map[string]any)["postgres:responseTime"].([]any)[0].(map[string]any)
	if pg["componentType"] != "datastore" || pg["observedUnit"] != "ms" || pg["status"] != "pass" || pg["critical"] != true || pg["kind"] != "database" {
		t.Fatalf("postgres %v", pg)
	}
	redis := doc["checks"].(map[string]any)["redis:responseTime"].([]any)[0].(map[string]any)
	if redis["componentType"] != "component" || redis["critical"] != false || redis["kind"] != "cache" {
		t.Fatalf("redis %v", redis)
	}
	down.Store(true)
	code, doc, _ = get()
	pg = doc["checks"].(map[string]any)["postgres:responseTime"].([]any)[0].(map[string]any)
	if code != 503 || doc["status"] != "fail" || pg["status"] != "fail" || len([]rune(pg["output"].(string))) != MaxMessage {
		t.Fatalf("down: %d %v", code, doc)
	}
	resp, err := http.Post(srv.URL, "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST answered %d", resp.StatusCode)
	}

	cached := New(Config{Service: "billing", CacheFor: time.Minute})
	n := atomic.Int32{}
	_ = cached.Register(Check{Name: "db", Run: func(context.Context) error { n.Add(1); return nil }})
	c := httptest.NewServer(cached.Handler())
	defer c.Close()
	for range 3 {
		r, _ := http.Get(c.URL)
		r.Body.Close()
	}
	if n.Load() != 1 {
		t.Fatalf("a cached report ran the checks %d times", n.Load())
	}
}

func TestPushSendsTheReport(t *testing.T) {
	var got []wireReport
	var auth string
	var status atomic.Int32
	status.Store(200)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/monitoring/health" || r.Method != http.MethodPost {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		auth = r.Header.Get("Authorization")
		var rep wireReport
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &rep); err != nil {
			t.Error(err)
		}
		got = append(got, rep)
		if s := status.Load(); s != 200 {
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(int(s))
			_, _ = w.Write([]byte(`{"code":"rate_limited"}`))
			return
		}
		_, _ = w.Write([]byte(`{"service_id":"x","state":"healthy","ignored_checks":[]}`))
	}))
	defer srv.Close()
	h := New(Config{Service: "billing", Environment: "staging", Version: "1.4.2", Instance: "pod-a", BaseURL: srv.URL + "/", APIKey: "yuva_sk_test"})
	_ = h.Register(Check{Name: "postgres", Kind: Database, Run: ok})
	_ = h.Register(Check{Name: "redis", Kind: Cache, Optional: true, Run: failing(errors.New("refused"))})
	if err := h.Push(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := got[0]
	if auth != "Bearer yuva_sk_test" || r.Service != "billing" || r.Environment != "staging" || r.Version != "1.4.2" || r.Instance != "pod-a" ||
		r.Status != Degraded || len(r.Checks) != 2 || r.Checks[0].Critical != true || r.Checks[1].Critical || r.Checks[1].Status != Down ||
		r.Checks[1].Message != "refused" || r.Checks[0].LatencyMS == nil {
		t.Fatalf("report %+v", r)
	}

	status.Store(429)
	var pushErr error
	h.cfg.OnError = func(err error) { pushErr = err }
	h.pushOnce(context.Background())
	var pe *PushError
	if !errors.As(pushErr, &pe) || pe.Status != 429 || pe.RetryAfter != 2*time.Minute {
		t.Fatalf("429 %v", pushErr)
	}
	status.Store(200)
	h.pushOnce(context.Background())
	if len(got) != 2 {
		t.Fatalf("pushed during Retry-After: %d reports", len(got))
	}
	h.now = func() time.Time { return time.Now().Add(3 * time.Minute) }
	h.pushOnce(context.Background())
	if len(got) != 3 {
		t.Fatalf("after Retry-After: %d reports", len(got))
	}
	if err := New(Config{Service: "billing"}).Push(context.Background()); err == nil {
		t.Fatal("a push without BaseURL and APIKey")
	}
}

func TestStartPushesUntilClose(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { n.Add(1) }))
	defer srv.Close()
	h := New(Config{Service: "billing", BaseURL: srv.URL, APIKey: "k", Interval: 20 * time.Millisecond})
	h.Start()
	h.Start()
	time.Sleep(110 * time.Millisecond)
	h.Close()
	after := n.Load()
	time.Sleep(60 * time.Millisecond)
	if after < 3 || n.Load() != after {
		t.Fatalf("pushes %d, then %d after Close", after, n.Load())
	}
	h.Close()
}

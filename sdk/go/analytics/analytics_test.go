package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type recorder struct {
	mu       sync.Mutex
	requests []map[string]any
	headers  []http.Header
	answers  []func(w http.ResponseWriter)
}

func (rec *recorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	rec.mu.Lock()
	rec.requests = append(rec.requests, body)
	rec.headers = append(rec.headers, r.Header.Clone())
	var answer func(http.ResponseWriter)
	if len(rec.answers) > 0 {
		answer, rec.answers = rec.answers[0], rec.answers[1:]
	}
	rec.mu.Unlock()
	if r.URL.Path != "/v1/analytics/properties/prop-1/events" || r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if answer != nil {
		answer(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"accepted": 1, "errors": []}`))
}

func (rec *recorder) events(i int) []any {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return rec.requests[i]["events"].([]any)
}

func (rec *recorder) count() int {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return len(rec.requests)
}

func newTestClient(t *testing.T, rec *recorder, cfg Config) (*Client, *[]error) {
	t.Helper()
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)
	var mu sync.Mutex
	errs := &[]error{}
	cfg.BaseURL, cfg.APIKey, cfg.PropertyID = srv.URL+"/", "yuva_key", "prop-1"
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = time.Hour
	}
	cfg.OnError = func(err error) {
		mu.Lock()
		defer mu.Unlock()
		*errs = append(*errs, err)
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	c.sleep = func(ctx context.Context, d time.Duration) error { return ctx.Err() }
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c, errs
}

func visitorRequest() *http.Request {
	r := httptest.NewRequest(http.MethodPost, "https://example.com/signup?utm_source=news&secret=1", nil)
	r.RemoteAddr = "203.0.113.7:51234"
	r.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605.1.15")
	r.Header.Set("Sec-CH-UA-Platform", `"macOS"`)
	r.Header.Set("Sec-CH-UA-Mobile", "?0")
	r.Header.Set("Accept-Language", "tr-TR,tr;q=0.9")
	r.Header.Set("Referer", "https://www.google.com/")
	return r
}

func TestTrackForwardsTheVisitorAndBatches(t *testing.T) {
	rec := &recorder{}
	c, errs := newTestClient(t, rec, Config{})
	at := time.Date(2026, 10, 12, 9, 0, 0, 0, time.FixedZone("TRT", 3*3600))
	if err := c.Track(visitorRequest(), Event{Name: "Signup", Time: at, Props: map[string]any{"plan": "pro"}, Revenue: &Revenue{Amount: "29.90", Currency: "EUR"}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Send(Event{Name: "Renewal", URL: "https://example.com/billing", VisitorKey: "session-42", NonInteractive: true}); err != nil {
		t.Fatal(err)
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rec.count() != 1 {
		t.Fatalf("%d requests, want one batch", rec.count())
	}
	if got := rec.headers[0].Get("Authorization"); got != "Bearer yuva_key" {
		t.Fatalf("authorization %q", got)
	}
	evs := rec.events(0)
	first := evs[0].(map[string]any)
	want := map[string]any{
		"name": "Signup", "url": "https://example.com/signup?utm_source=news&secret=1", "referrer": "https://www.google.com/",
		"timestamp": "2026-10-12T06:00:00Z", "ip": "203.0.113.7", "user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605.1.15",
		"sec_ch_ua_platform": `"macOS"`, "sec_ch_ua_mobile": "?0", "accept_language": "tr-TR,tr;q=0.9",
	}
	for k, v := range want {
		if first[k] != v {
			t.Fatalf("%s = %v, want %v (%v)", k, first[k], v, first)
		}
	}
	if first["revenue"].(map[string]any)["a"] != "29.90" || first["props"].(map[string]any)["plan"] != "pro" {
		t.Fatalf("revenue or props %v", first)
	}
	second := evs[1].(map[string]any)
	if second["visitor_key"] != "session-42" || second["interactive"] != false || second["ip"] != nil || second["user_agent"] != nil {
		t.Fatalf("visitor key event %v", second)
	}
	if len(*errs) != 0 {
		t.Fatalf("errors %v", *errs)
	}
}

func TestTrackSendsNothingForDNTOrGPC(t *testing.T) {
	rec := &recorder{}
	c, _ := newTestClient(t, rec, Config{})
	for _, h := range []string{"DNT", "Sec-GPC"} {
		r := visitorRequest()
		r.Header.Set(h, "1")
		if err := c.Track(r, Event{Name: "Signup"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rec.count() != 0 {
		t.Fatalf("%d requests", rec.count())
	}
}

func TestFullBatchesGoOutAtOnce(t *testing.T) {
	rec := &recorder{}
	c, _ := newTestClient(t, rec, Config{})
	for range MaxBatch {
		if err := c.Send(Event{Name: "Tick", URL: "https://example.com/", VisitorKey: "k"}); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for rec.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if rec.count() != 1 || len(rec.events(0)) != MaxBatch {
		t.Fatalf("%d requests before the interval", rec.count())
	}
	if err := c.Send(Event{Name: "Tick", URL: "https://example.com/", VisitorKey: "k"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rec.count() != 2 || len(rec.events(1)) != 1 {
		t.Fatalf("close sent %d requests", rec.count())
	}
	if err := c.Send(Event{Name: "Tick", URL: "https://example.com/", VisitorKey: "k"}); !errors.Is(err, ErrClosed) {
		t.Fatalf("send after close: %v", err)
	}
}

func TestRetriesAndRefusals(t *testing.T) {
	busy := func(w http.ResponseWriter) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	rec := &recorder{answers: []func(http.ResponseWriter){busy, busy}}
	c, errs := newTestClient(t, rec, Config{})
	var waits []time.Duration
	c.sleep = func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}
	if err := c.Send(Event{Name: "Signup", URL: "https://example.com/", VisitorKey: "k"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rec.count() != 3 || len(waits) != 2 || waits[0] != 2*time.Second || waits[1] != 2*time.Second {
		t.Fatalf("%d requests, waits %v", rec.count(), waits)
	}

	rec.answers = []func(http.ResponseWriter){func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"insufficient_scope"}`))
	}}
	if err := c.Send(Event{Name: "Signup", URL: "https://example.com/", VisitorKey: "k"}); err != nil {
		t.Fatal(err)
	}
	err := c.Flush(context.Background())
	var batch *BatchError
	if !errors.As(err, &batch) || batch.Status != http.StatusForbidden || rec.count() != 4 || len(*errs) != 1 {
		t.Fatalf("a refused batch is retried or not reported: %v, %d requests", err, rec.count())
	}

	rec.answers = []func(http.ResponseWriter){func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"accepted": 1, "errors": [{"index": 1, "detail": "engagement is sent by browsers only"}]}`))
	}}
	_ = c.Send(Event{Name: "Signup", URL: "https://example.com/", VisitorKey: "k"})
	_ = c.Send(Event{Name: "engagement", URL: "https://example.com/", VisitorKey: "k"})
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	var ev *EventError
	if len(*errs) != 2 || !errors.As((*errs)[1], &ev) || ev.Event.Name != "engagement" {
		t.Fatalf("event errors %v", *errs)
	}
}

func TestQueueLimit(t *testing.T) {
	rec := &recorder{}
	c, errs := newTestClient(t, rec, Config{QueueSize: 1})
	_ = c.Send(Event{Name: "A", URL: "https://example.com/", VisitorKey: "k"})
	if err := c.Send(Event{Name: "B", URL: "https://example.com/", VisitorKey: "k"}); !errors.Is(err, ErrQueueFull) || len(*errs) != 1 {
		t.Fatalf("queue limit: %v", err)
	}
	if err := c.Send(Event{Name: "C"}); err == nil {
		t.Fatal("an event without a URL was queued")
	}
}

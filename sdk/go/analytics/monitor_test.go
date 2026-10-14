package analytics

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordSink struct {
	mu      sync.Mutex
	batches [][]map[string]any
	answer  func(w http.ResponseWriter)
}

func (s *recordSink) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/analytics/properties/prop-1/requests" || r.Header.Get("Content-Encoding") != "gzip" || r.Header.Get("Authorization") != "Bearer yuva_key" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	zr, err := gzip.NewReader(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var body struct {
		Records []map[string]any `json:"records"`
	}
	_ = json.NewDecoder(zr).Decode(&body)
	s.mu.Lock()
	s.batches = append(s.batches, body.Records)
	answer := s.answer
	s.mu.Unlock()
	if answer != nil {
		answer(w)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"accepted": 1, "errors": []}`))
}

func (s *recordSink) all() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []map[string]any
	for _, b := range s.batches {
		out = append(out, b...)
	}
	return out
}

func newMonitorClient(t *testing.T, sink *recordSink) (*Client, *[]error) {
	t.Helper()
	srv := httptest.NewServer(sink)
	t.Cleanup(srv.Close)
	var mu sync.Mutex
	errs := &[]error{}
	c, err := New(Config{BaseURL: srv.URL, APIKey: "yuva_key", PropertyID: "prop-1", FlushInterval: time.Hour, OnError: func(err error) {
		mu.Lock()
		defer mu.Unlock()
		*errs = append(*errs, err)
	}})
	if err != nil {
		t.Fatal(err)
	}
	c.sleep = func(ctx context.Context, d time.Duration) error { return ctx.Err() }
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c, errs
}

func serve(h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, target, rd)
	r.RemoteAddr = "203.0.113.7:4000"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestMiddlewareRecordsTheMuxPattern(t *testing.T) {
	sink := &recordSink{}
	c, errs := newMonitorClient(t, sink)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"` + r.PathValue("id") + `"}`))
	})
	mux.HandleFunc("POST api.example.com/orders", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/boom", func(http.ResponseWriter, *http.Request) { panic("boom") })
	h := c.Middleware(Monitor{Service: "shop", Environment: "staging", Country: true,
		Consumer: func(r *http.Request) string { return r.Header.Get("X-Client") }})(mux)

	serve(h, http.MethodGet, "http://api.example.com/users/42?token=secret", "")
	serve(h, http.MethodPost, "http://api.example.com/orders", `{"sku":"A-1"}`)
	serve(h, http.MethodGet, "http://api.example.com/wp-login.php?x=1", "")
	serve(h, http.MethodDelete, "http://api.example.com/users/42", "")
	func() {
		defer func() {
			if recover() == nil {
				t.Error("the panic did not go on")
			}
		}()
		serve(h, http.MethodGet, "http://api.example.com/boom", "")
	}()
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	recs := sink.all()
	if len(recs) != 5 || len(*errs) != 0 {
		t.Fatalf("%d records, errors %v", len(recs), *errs)
	}
	want := []map[string]any{
		{"method": "GET", "route": "/users/{id}", "status": 200.0, "response_size": 11.0, "service": "shop", "environment": "staging", "ip": "203.0.113.7"},
		{"method": "POST", "route": "/orders", "status": 201.0, "request_size": 13.0, "response_size": 0.0},
		{"method": "GET", "path": "/wp-login.php", "status": 404.0},
		{"method": "DELETE", "path": "/users/42", "status": 405.0},
		{"method": "GET", "route": "/boom", "status": 0.0},
	}
	for i, w := range want {
		for k, v := range w {
			if recs[i][k] != v {
				t.Fatalf("record %d: %s = %v, want %v (%v)", i, k, recs[i][k], v, recs[i])
			}
		}
		if _, ok := recs[i]["duration_us"].(float64); !ok || recs[i]["time"] == nil {
			t.Fatalf("record %d has no duration or time: %v", i, recs[i])
		}
		for _, k := range []string{"query", "headers", "user_agent", "body"} {
			if _, ok := recs[i][k]; ok {
				t.Fatalf("record %d carries %s", i, k)
			}
		}
	}
	if strings.Contains(recs[0]["route"].(string)+recs[2]["path"].(string), "?") {
		t.Fatal("a query string was sent")
	}
}

func TestMiddlewareRouteBatchesAndSampling(t *testing.T) {
	sink := &recordSink{}
	c, _ := newMonitorClient(t, sink)
	n := 0
	c.sample = func(rate int) bool {
		n++
		return n%rate == 0
	}
	h := c.Middleware(Monitor{SampleRate: 4, Route: func(*http.Request) string { return "/things/{id}" }})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fail") != "" {
			w.WriteHeader(http.StatusBadGateway)
		}
	}))
	for range 2000 {
		serve(h, http.MethodGet, "http://api.example.com/things/1", "")
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(sink.all()) < MaxRecordBatch && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := len(sink.all()); got != MaxRecordBatch {
		t.Fatalf("%d records before the interval, want a full batch of %d", got, MaxRecordBatch)
	}
	for range 3 {
		serve(h, http.MethodGet, "http://api.example.com/things/1?fail=1", "")
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	recs := sink.all()
	failed := 0
	for _, r := range recs {
		if r["route"] != "/things/{id}" {
			t.Fatalf("route %v", r)
		}
		switch r["status"] {
		case 200.0:
			if r["sample_rate"] != 4.0 {
				t.Fatalf("a sampled record %v", r)
			}
		case 502.0:
			failed++
			if r["sample_rate"] != nil {
				t.Fatalf("an error was sampled: %v", r)
			}
		}
	}
	if len(recs) != 503 || failed != 3 {
		t.Fatalf("%d records, %d errors", len(recs), failed)
	}
}

func TestRecordErrorsAndQueue(t *testing.T) {
	sink := &recordSink{answer: func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"accepted": 1, "errors": [{"index": 1, "detail": "status must be 100 to 599, or 0 when no response was sent"}]}`))
	}}
	c, errs := newMonitorClient(t, sink)
	_ = c.SendRecord(Record{Method: "GET", Route: "/a", Status: 200, Duration: time.Millisecond})
	_ = c.SendRecord(Record{Method: "GET", Route: "/b", Status: 900, Duration: time.Millisecond})
	if err := c.SendRecord(Record{Method: "GET", Status: 200}); err == nil {
		t.Fatal("a record without a route or path was queued")
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	var re *RecordError
	if len(*errs) != 1 || !errors.As((*errs)[0], &re) || re.Record.Route != "/b" {
		t.Fatalf("record errors %v", *errs)
	}
	if got := sink.all()[0]["duration_us"]; got != 1000.0 {
		t.Fatalf("duration %v", got)
	}
}

func TestPatternRoute(t *testing.T) {
	for in, want := range map[string]string{
		"GET /users/{id}":                  "/users/{id}",
		"POST api.example.com/orders/{id}": "/orders/{id}",
		"example.com/":                     "/",
		"/static/":                         "/static/",
		"":                                 "",
	} {
		if got := patternRoute(in); got != want {
			t.Errorf("patternRoute(%q) = %q, want %q", in, got, want)
		}
	}
}

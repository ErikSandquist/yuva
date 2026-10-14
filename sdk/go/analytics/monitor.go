package analytics

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Record is one request an API served, for a property of kind api. Method, Status and Duration
// are required, and Route or Path.
type Record struct {
	Method string
	// Route is the pattern that matched, such as /users/{id}. Without one, Path is sent and Yuva
	// infers a pattern, counting 404 and 405 answers as (unmatched).
	Route string
	Path  string
	// Status is the response status, or 0 when no response was sent.
	Status   int
	Duration time.Duration
	// RequestSize and ResponseSize are body bytes; negative when unknown.
	RequestSize  int64
	ResponseSize int64
	// Time is when the request started; now when zero.
	Time time.Time
	// Consumer names who called, such as an API key's name or a client id, never a person's
	// address.
	Consumer    string
	APIVersion  string
	Environment string
	Service     string
	Release     string
	// IP is the caller's address, sent only to find its country; Yuva shortens it on arrival and
	// stores no address.
	IP string
	// SampleRate says the record stands for this many requests; only successful ones (status 1 to
	// 399) may be sampled.
	SampleRate int
}

type wireRecord struct {
	Method       string    `json:"method"`
	Route        string    `json:"route,omitempty"`
	Path         string    `json:"path,omitempty"`
	Status       int       `json:"status"`
	DurationUS   int64     `json:"duration_us"`
	RequestSize  *int64    `json:"request_size,omitempty"`
	ResponseSize *int64    `json:"response_size,omitempty"`
	Time         time.Time `json:"time"`
	Consumer     string    `json:"consumer,omitempty"`
	APIVersion   string    `json:"api_version,omitempty"`
	Environment  string    `json:"environment,omitempty"`
	Service      string    `json:"service,omitempty"`
	Release      string    `json:"release,omitempty"`
	IP           string    `json:"ip,omitempty"`
	SampleRate   int       `json:"sample_rate,omitempty"`
}

func (r Record) wire() wireRecord {
	w := wireRecord{
		Method: r.Method, Route: r.Route, Status: r.Status, DurationUS: max(r.Duration.Microseconds(), 0), Time: r.Time.UTC(),
		Consumer: r.Consumer, APIVersion: r.APIVersion, Environment: r.Environment, Service: r.Service, Release: r.Release, IP: r.IP,
	}
	if w.Route == "" {
		w.Path = r.Path
	}
	if r.RequestSize >= 0 {
		w.RequestSize = &r.RequestSize
	}
	if r.ResponseSize >= 0 {
		w.ResponseSize = &r.ResponseSize
	}
	if r.SampleRate > 1 {
		w.SampleRate = r.SampleRate
	}
	return w
}

// RecordError is a request record Yuva left out of an accepted batch as invalid.
type RecordError struct {
	Record Record
	Detail string
}

func (e *RecordError) Error() string {
	route := e.Record.Route
	if route == "" {
		route = e.Record.Path
	}
	return fmt.Sprintf("analytics: request record %s %s refused: %s", e.Record.Method, route, e.Detail)
}

// ErrRecordQueueFull is reported when a request record is dropped because the queue holds
// Config.RecordQueueSize records.
var ErrRecordQueueFull = errors.New("analytics: the request record queue is full; record dropped")

// SendRecord queues a request record as it is.
func (c *Client) SendRecord(r Record) error {
	if r.Method == "" || (r.Route == "" && r.Path == "") {
		return errors.New("analytics: a request record needs a Method and a Route or Path")
	}
	if r.Time.IsZero() {
		r.Time = c.now()
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrClosed
	}
	if len(c.records) >= c.cfg.RecordQueueSize {
		c.mu.Unlock()
		c.report(ErrRecordQueueFull)
		return ErrRecordQueueFull
	}
	c.records = append(c.records, r)
	full := len(c.records) >= MaxRecordBatch
	c.mu.Unlock()
	if full {
		select {
		case c.wake <- struct{}{}:
		default:
		}
	}
	return nil
}

func (c *Client) sendRecords(ctx context.Context, batch []Record) error {
	wire := make([]wireRecord, len(batch))
	for i, r := range batch {
		wire[i] = r.wire()
	}
	raw, err := json.Marshal(map[string]any{"records": wire})
	if err != nil {
		return &BatchError{Events: len(batch), Err: err}
	}
	var body bytes.Buffer
	zw := gzip.NewWriter(&body)
	if _, err := zw.Write(raw); err != nil {
		return &BatchError{Events: len(batch), Err: err}
	}
	if err := zw.Close(); err != nil {
		return &BatchError{Events: len(batch), Err: err}
	}
	return c.deliver(ctx, c.requests, body.Bytes(), true, len(batch), func(i int, detail string) {
		c.report(&RecordError{Record: batch[i], Detail: detail})
	})
}

// Monitor configures Middleware. Every field is optional.
type Monitor struct {
	// Route names a request's route pattern. When nil, Middleware reads r.Pattern after the
	// handler ran (http.ServeMux sets it), without its method and host; a request no pattern
	// matched is sent with its path.
	Route func(r *http.Request) string
	// Consumer names who called, such as the API key's name or a client id; never a person's
	// address.
	Consumer   func(r *http.Request) string
	APIVersion func(r *http.Request) string
	// Environment is production when empty.
	Environment string
	Service     string
	Release     string
	// SampleRate sends one in SampleRate successful requests (status 1 to 399), each standing for
	// SampleRate requests; errors and client errors are always sent. Every request when 0 or 1.
	SampleRate int
	// Country sends the caller's address (Config.ClientIP) so Yuva can count the request's
	// country; the address is shortened on arrival and never stored.
	Country bool
}

// Middleware records every request the handler answers. Wrap the http.ServeMux itself, so the
// pattern it matched is set on the request Middleware sees:
//
//	http.ListenAndServe(":8080", client.Middleware(analytics.Monitor{Service: "shop"})(mux))
//
// A handler that panics is recorded with the status it wrote, or 0 when it wrote none, and the
// panic goes on.
func (c *Client) Middleware(m Monitor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := c.now()
			rw := &responseWriter{ResponseWriter: w}
			var body *countingBody
			if r.Body != nil && r.Body != http.NoBody {
				body = &countingBody{ReadCloser: r.Body}
				r.Body = body
			}
			defer func() {
				v := recover()
				status := rw.status
				if status == 0 && v == nil {
					status = http.StatusOK
				}
				c.recordRequest(m, r, status, c.now().Sub(start), body, rw.written, start)
				if v != nil {
					panic(v)
				}
			}()
			next.ServeHTTP(rw, r)
		})
	}
}

func (c *Client) recordRequest(m Monitor, r *http.Request, status int, took time.Duration, body *countingBody, written int64, start time.Time) {
	weight := 1
	if m.SampleRate > 1 && status > 0 && status < 400 {
		if !c.sample(m.SampleRate) {
			return
		}
		weight = m.SampleRate
	}
	rec := Record{Method: r.Method, Status: status, Duration: took, RequestSize: r.ContentLength, ResponseSize: written, Time: start,
		Environment: m.Environment, Service: m.Service, Release: m.Release, SampleRate: weight}
	if body != nil && body.n > 0 {
		rec.RequestSize = body.n
	}
	if r.ContentLength < 0 && (body == nil || body.n == 0) {
		rec.RequestSize = -1
	}
	if m.Route != nil {
		rec.Route = m.Route(r)
	} else {
		rec.Route = patternRoute(r.Pattern)
	}
	if rec.Route == "" {
		rec.Path = r.URL.Path
	}
	if m.Consumer != nil {
		rec.Consumer = m.Consumer(r)
	}
	if m.APIVersion != nil {
		rec.APIVersion = m.APIVersion(r)
	}
	if m.Country {
		rec.IP = c.cfg.ClientIP(r)
	}
	_ = c.SendRecord(rec)
}

// patternRoute is a ServeMux pattern without its method and host: "GET example.com/users/{id}"
// is /users/{id}.
func patternRoute(p string) string {
	if _, rest, ok := strings.Cut(p, " "); ok {
		p = strings.TrimSpace(rest)
	}
	if i := strings.IndexByte(p, '/'); i > 0 {
		p = p[i:]
	}
	if !strings.HasPrefix(p, "/") {
		return ""
	}
	return p
}

type countingBody struct {
	io.ReadCloser
	n int64
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.n += int64(n)
	return n, err
}

// responseWriter notes the status and the body bytes; Unwrap lets http.ResponseController reach
// the connection's own writer.
type responseWriter struct {
	http.ResponseWriter
	status  int
	written int64
}

func (w *responseWriter) WriteHeader(status int) {
	if w.status == 0 && status >= 200 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.written += int64(n)
	return n, err
}

func (w *responseWriter) Flush() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil && w.status == 0 {
		w.status = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

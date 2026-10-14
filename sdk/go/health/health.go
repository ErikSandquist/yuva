// Package health reports a service's health and its dependencies' to Yuva, for service health in
// API monitoring.
//
// Register a check per dependency (a database, a cache, a queue, another API), each with a
// timeout. Handler serves the checks as application/health+json (the IETF health check draft),
// which Yuva reads when a member adds the service as a pulled one; Start pushes the same report
// to `POST /v1/monitoring/health` on an interval with an API key that has the `analytics:write`
// scope. Pushed and pulled reports give Yuva the same states.
//
// A check that returns nil is healthy, one that returns an error wrapping ErrDegraded is
// degraded, and any other error, or running past its timeout, is down. A critical dependency down
// makes the service down, any other degraded. Check errors become the dependency's message (at
// most 200 characters): Yuva shows it to members only, but Handler serves it to whoever can call
// the endpoint, so keep secrets out of errors or put the handler behind your own access rules.
package health

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Status is how a service or dependency is doing.
type Status string

const (
	Healthy  Status = "healthy"
	Degraded Status = "degraded"
	Down     Status = "down"
)

// Kind is what a dependency is.
type Kind string

const (
	Database Kind = "database"
	Cache    Kind = "cache"
	Queue    Kind = "queue"
	HTTP     Kind = "http"
	Other    Kind = "other"
)

const (
	// MaxChecks is how many dependencies a service reports.
	MaxChecks = 50
	// MaxMessage is how many characters of a check's error Yuva keeps.
	MaxMessage = 200

	defaultTimeout  = 5 * time.Second
	defaultInterval = 30 * time.Second
	defaultCacheFor = time.Second
)

var (
	// ErrDegraded marks a check's error as degraded rather than down.
	ErrDegraded = errors.New("degraded")
	// ErrTooManyChecks is returned by Register past MaxChecks checks.
	ErrTooManyChecks = errors.New("health: at most 50 checks")
	// ErrDuplicate is returned by Register for a name already registered.
	ErrDuplicate = errors.New("health: a check of this name is registered")
)

// Check is one dependency's check.
type Check struct {
	// Name names the dependency, such as "postgres"; 1 to 100 characters.
	Name string
	// Kind is what it is; Other when empty.
	Kind Kind
	// Optional makes the dependency non-critical: down, it makes the service degraded, not down.
	Optional bool
	// Timeout bounds Run; 5 seconds when zero.
	Timeout time.Duration
	// Run checks the dependency, such as db.PingContext. It must return when ctx ends.
	Run func(ctx context.Context) error
}

// Config configures a Health. Service is required; BaseURL and APIKey only for pushing.
type Config struct {
	// Service is the service's name, as its request records and spans give it.
	Service string
	// Environment is "production" when empty.
	Environment string
	// Version is the running release.
	Version string
	// Instance names this process, such as the pod name; the host name is a good choice.
	Instance string
	// BaseURL is the Yuva server, such as https://yuva.example.com.
	BaseURL string
	// APIKey is an API key with the analytics:write scope.
	APIKey string
	// Interval is how often Start pushes; 30 seconds when zero. Tell Yuva the same interval in the
	// service's settings, since it marks a service down after three intervals without a report.
	Interval time.Duration
	// CacheFor is how long Handler reuses a report, so a busy endpoint does not run the checks on
	// every request; one second when zero, none when negative.
	CacheFor time.Duration
	// HTTPClient sends the reports; http.DefaultClient when nil.
	HTTPClient *http.Client
	// OnError learns about reports that could not be pushed. It is called from Start's goroutine.
	OnError func(error)
}

// Result is one check's outcome.
type Result struct {
	Name     string
	Kind     Kind
	Status   Status
	Critical bool
	Latency  time.Duration
	Message  string
	At       time.Time
}

// Report is the service's state from one run of its checks.
type Report struct {
	Status Status
	Checks []Result
	At     time.Time
}

// Health runs the registered checks. It is safe for concurrent use.
type Health struct {
	cfg  Config
	now  func() time.Time
	push string

	mu     sync.Mutex
	checks []Check
	cached *Report

	stop     context.CancelFunc
	done     chan struct{}
	waitTill time.Time
}

// New returns a Health for cfg.
func New(cfg Config) *Health {
	if cfg.Environment == "" {
		cfg.Environment = "production"
	}
	if cfg.Interval <= 0 {
		cfg.Interval = defaultInterval
	}
	if cfg.CacheFor == 0 {
		cfg.CacheFor = defaultCacheFor
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	return &Health{cfg: cfg, now: time.Now, push: strings.TrimRight(cfg.BaseURL, "/") + "/v1/monitoring/health"}
}

// Register adds a check.
func (h *Health) Register(c Check) error {
	if n := utf8.RuneCountInString(strings.TrimSpace(c.Name)); n < 1 || n > 100 {
		return errors.New("health: a check's name has 1 to 100 characters")
	}
	if c.Run == nil {
		return errors.New("health: a check needs Run")
	}
	if c.Kind == "" {
		c.Kind = Other
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	c.Name = strings.TrimSpace(c.Name)
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.checks) >= MaxChecks {
		return ErrTooManyChecks
	}
	for _, o := range h.checks {
		if o.Name == c.Name {
			return ErrDuplicate
		}
	}
	h.checks = append(h.checks, c)
	h.cached = nil
	return nil
}

func clip(s string) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= MaxMessage {
		return s
	}
	return string([]rune(s)[:MaxMessage])
}

func runCheck(ctx context.Context, c Check, now func() time.Time) Result {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	start := now()
	r := Result{Name: c.Name, Kind: c.Kind, Critical: !c.Optional, Status: Healthy, At: start}
	errc := make(chan error, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				errc <- fmt.Errorf("check panicked: %v", p)
			}
		}()
		errc <- c.Run(ctx)
	}()
	var err error
	select {
	case err = <-errc:
	case <-ctx.Done():
		err = fmt.Errorf("no answer within %s", c.Timeout)
	}
	r.Latency = now().Sub(start)
	switch {
	case err == nil:
	case errors.Is(err, ErrDegraded):
		r.Status, r.Message = Degraded, clip(err.Error())
	default:
		r.Status, r.Message = Down, clip(err.Error())
	}
	return r
}

// overall is the service's status from its checks: a critical one down makes it down, any other
// trouble degraded.
func overall(results []Result) Status {
	s := Healthy
	for _, r := range results {
		switch {
		case r.Status == Down && r.Critical:
			return Down
		case r.Status != Healthy:
			s = Degraded
		}
	}
	return s
}

// Run runs every check at once, each within its timeout.
func (h *Health) Run(ctx context.Context) Report {
	h.mu.Lock()
	checks := append([]Check(nil), h.checks...)
	h.mu.Unlock()
	results := make([]Result, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = runCheck(ctx, c, h.now)
		}()
	}
	wg.Wait()
	return Report{Status: overall(results), Checks: results, At: h.now()}
}

func (h *Health) report(ctx context.Context) Report {
	if h.cfg.CacheFor > 0 {
		h.mu.Lock()
		if c := h.cached; c != nil && h.now().Sub(c.At) < h.cfg.CacheFor {
			h.mu.Unlock()
			return *c
		}
		h.mu.Unlock()
	}
	r := h.Run(ctx)
	h.mu.Lock()
	h.cached = &r
	h.mu.Unlock()
	return r
}

var draftStatus = map[Status]string{Healthy: "pass", Degraded: "warn", Down: "fail"}

type draftCheck struct {
	ComponentType string  `json:"componentType"`
	ObservedValue float64 `json:"observedValue"`
	ObservedUnit  string  `json:"observedUnit"`
	Status        string  `json:"status"`
	Time          string  `json:"time"`
	Output        string  `json:"output,omitempty"`
	Critical      bool    `json:"critical"`
	Kind          Kind    `json:"kind"`
}

type draftDoc struct {
	Status    string                  `json:"status"`
	Version   string                  `json:"version,omitempty"`
	ReleaseID string                  `json:"releaseId,omitempty"`
	ServiceID string                  `json:"serviceId"`
	Checks    map[string][]draftCheck `json:"checks,omitempty"`
}

// HealthJSON is a report as application/health+json: each check under "<name>:responseTime"
// with its latency in milliseconds, plus the "critical" and "kind" keys Yuva reads.
func (h *Health) HealthJSON(r Report) []byte {
	doc := draftDoc{Status: draftStatus[r.Status], ReleaseID: h.cfg.Version, ServiceID: h.cfg.Service, Checks: map[string][]draftCheck{}}
	if h.cfg.Version != "" {
		doc.Version = strings.SplitN(h.cfg.Version, ".", 2)[0]
	}
	for _, c := range r.Checks {
		typ := "component"
		if c.Kind == Database {
			typ = "datastore"
		}
		doc.Checks[c.Name+":responseTime"] = []draftCheck{{ComponentType: typ, ObservedValue: float64(c.Latency.Microseconds()) / 1000, ObservedUnit: "ms",
			Status: draftStatus[c.Status], Time: c.At.UTC().Format(time.RFC3339), Output: c.Message, Critical: c.Critical, Kind: c.Kind}}
	}
	b, _ := json.Marshal(doc)
	return b
}

// Handler serves the checks as application/health+json: 200 while healthy or degraded, 503
// when down, never cached by clients.
func (h *Health) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet && req.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		r := h.report(req.Context())
		w.Header().Set("Content-Type", "application/health+json")
		w.Header().Set("Cache-Control", "no-store")
		if r.Status == Down {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		if req.Method == http.MethodGet {
			_, _ = w.Write(h.HealthJSON(r))
		}
	})
}

type wireCheck struct {
	Name      string   `json:"name"`
	Kind      Kind     `json:"kind"`
	Status    Status   `json:"status"`
	LatencyMS *float64 `json:"latency_ms,omitempty"`
	Critical  bool     `json:"critical"`
	Message   string   `json:"message,omitempty"`
}

type wireReport struct {
	Service     string      `json:"service"`
	Environment string      `json:"environment,omitempty"`
	Version     string      `json:"version,omitempty"`
	Instance    string      `json:"instance,omitempty"`
	Status      Status      `json:"status"`
	Checks      []wireCheck `json:"checks"`
}

// PushError is a report Yuva did not take.
type PushError struct {
	Status     int
	Body       string
	RetryAfter time.Duration
	Err        error
}

func (e *PushError) Error() string {
	if e.Err != nil {
		return "health: report not sent: " + e.Err.Error()
	}
	return fmt.Sprintf("health: report refused: %d %s", e.Status, e.Body)
}

func (e *PushError) Unwrap() error { return e.Err }

// Push runs the checks and sends the report to Yuva once.
func (h *Health) Push(ctx context.Context) error {
	return h.send(ctx, h.Run(ctx))
}

func (h *Health) send(ctx context.Context, r Report) error {
	if h.cfg.BaseURL == "" || h.cfg.APIKey == "" || h.cfg.Service == "" {
		return errors.New("health: pushing needs BaseURL, APIKey and Service")
	}
	body := wireReport{Service: h.cfg.Service, Environment: h.cfg.Environment, Version: h.cfg.Version, Instance: h.cfg.Instance, Status: r.Status,
		Checks: []wireCheck{}}
	for _, c := range r.Checks {
		ms := float64(c.Latency.Microseconds()) / 1000
		body.Checks = append(body.Checks, wireCheck{Name: c.Name, Kind: c.Kind, Status: c.Status, LatencyMS: &ms, Critical: c.Critical, Message: c.Message})
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.push, bytes.NewReader(raw))
	if err != nil {
		return &PushError{Err: err}
	}
	req.Header.Set("Authorization", "Bearer "+h.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.cfg.HTTPClient.Do(req)
	if err != nil {
		return &PushError{Err: err}
	}
	defer resp.Body.Close()
	text, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode/100 == 2 {
		return nil
	}
	e := &PushError{Status: resp.StatusCode, Body: strings.TrimSpace(string(text))}
	if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs > 0 {
		e.RetryAfter = time.Duration(secs) * time.Second
	}
	return e
}

// Start pushes a report now and then every Interval until Close; a 429 or 503 with Retry-After
// skips the pushes due before it. Start after Start does nothing.
func (h *Health) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stop != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.stop, h.done = cancel, make(chan struct{})
	go h.loop(ctx, h.done)
}

func (h *Health) loop(ctx context.Context, done chan struct{}) {
	defer close(done)
	t := time.NewTicker(h.cfg.Interval)
	defer t.Stop()
	for {
		h.pushOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (h *Health) pushOnce(ctx context.Context) {
	h.mu.Lock()
	wait := h.waitTill
	h.mu.Unlock()
	if h.now().Before(wait) {
		return
	}
	err := h.Push(ctx)
	if err == nil || ctx.Err() != nil {
		return
	}
	var pe *PushError
	if errors.As(err, &pe) && pe.RetryAfter > 0 {
		h.mu.Lock()
		h.waitTill = h.now().Add(pe.RetryAfter)
		h.mu.Unlock()
	}
	if h.cfg.OnError != nil {
		h.cfg.OnError(err)
	}
}

// Close stops Start's pushes and waits for one that is under way.
func (h *Health) Close() {
	h.mu.Lock()
	stop, done := h.stop, h.done
	h.stop = nil
	h.mu.Unlock()
	if stop == nil {
		return
	}
	stop()
	<-done
}

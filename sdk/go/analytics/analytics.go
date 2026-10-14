// Package analytics sends server-side analytics events to a Yuva analytics property, for events
// only the backend sees, such as a signup or a payment it handled.
//
// Events are queued and sent in batches of up to 100 to
// `POST /v1/analytics/properties/{id}/events` with an API key that has the `analytics:write`
// scope. Track forwards the visitor's address, user agent and client hints from their request, so
// Yuva hashes them exactly as it hashes the browser's own requests and the event joins the
// visitor's browser visit of that day; none of it is stored. Requests with `DNT: 1` or
// `Sec-GPC: 1` are not sent at all.
//
// For an API property, Middleware records every request a net/http server answers (its route,
// method, status, duration and sizes, never a body, header or query) and sends the records in
// batches of up to 500 to `POST /v1/analytics/properties/{id}/requests` with the same key.
//
// Failed batches are retried with backoff (honouring `Retry-After`); a batch Yuva refuses as a
// whole (400, 401, 403, 404) is dropped and reported to Config.OnError. Call Close before the
// process exits so queued events and records are sent.
package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// MaxBatch is the most events one request carries; MaxRecordBatch the most request records.
	MaxBatch       = 100
	MaxRecordBatch = 500

	defaultInterval   = 5 * time.Second
	defaultQueue      = 10_000
	defaultRecords    = 50_000
	defaultRetries    = 5
	defaultMinBackoff = time.Second
	maxBackoff        = time.Minute
)

var (
	// ErrQueueFull is reported when an event is dropped because the queue holds Config.QueueSize
	// events.
	ErrQueueFull = errors.New("analytics: the queue is full; event dropped")
	// ErrClosed is returned by Track and Send after Close.
	ErrClosed = errors.New("analytics: client closed")
)

// Config configures a Client. BaseURL, APIKey and PropertyID are required.
type Config struct {
	// BaseURL is the Yuva server, such as https://yuva.example.com.
	BaseURL string
	// APIKey is an API key with the analytics:write scope.
	APIKey string
	// PropertyID is the analytics property's id (not its public yuva_ak_ key).
	PropertyID string
	// HTTPClient sends the requests; http.DefaultClient when nil.
	HTTPClient *http.Client
	// FlushInterval is how often queued events are sent; 5 seconds when zero. A full batch is
	// sent at once.
	FlushInterval time.Duration
	// QueueSize is how many events wait at most; 10,000 when zero. Events beyond it are dropped.
	QueueSize int
	// RecordQueueSize is how many request records wait at most; 50,000 when zero.
	RecordQueueSize int
	// MaxRetries is how often a batch is retried after a network error, 429 or 5xx; 5 when zero,
	// none when negative.
	MaxRetries int
	// MinBackoff is the first retry's delay, doubled up to a minute; one second when zero.
	MinBackoff time.Duration
	// ClientIP returns the visitor's address for Track. The default is the host of
	// r.RemoteAddr; behind a proxy, return the address your proxy reports.
	ClientIP func(r *http.Request) string
	// OnError learns about dropped events and batches. It is called from the sending goroutine.
	OnError func(error)
}

// Revenue is an amount in a currency, such as "29.90" EUR. Yuva converts nothing.
type Revenue struct {
	Amount   string `json:"a"`
	Currency string `json:"c"`
}

// Event is one server-side event. Name and URL are required; a visitor is named by IP and
// UserAgent or by VisitorKey.
type Event struct {
	// Name is "pageview", a built-in event or a custom event's name.
	Name string
	// URL is the page (on one of the property's domains) or an app screen, app://<app id>/<screen>.
	URL string
	// Referrer is the page's referrer.
	Referrer string
	// Time is when it happened; now when zero. Yuva drops events older than 72 hours.
	Time time.Time
	// Props are up to 30 string, number or boolean values.
	Props map[string]any
	// Revenue the event carries.
	Revenue *Revenue
	// NonInteractive keeps a visit with only this event a bounce.
	NonInteractive bool
	// Consent says the visitor agreed to analytics; a property with consent collection stores only
	// such events.
	Consent bool

	// IP, UserAgent, SecCHUAPlatform, SecCHUAMobile and AcceptLanguage are forwarded from the
	// visitor's request; Track fills them in.
	IP              string
	UserAgent       string
	SecCHUAPlatform string
	SecCHUAMobile   string
	AcceptLanguage  string
	// VisitorKey is your own key for the visitor, such as a session id (never an e-mail address),
	// used instead of IP and UserAgent.
	VisitorKey string
	// PersistentID is the id the visitor's browser or app keeps after they agreed to the
	// persistent consent feature; with Consent it names them on every day. IdentityToken, an
	// identity token of the property's inbox sent with it, links them to their contact.
	PersistentID  string
	IdentityToken string
}

type wireEvent struct {
	Name            string         `json:"name"`
	URL             string         `json:"url"`
	Referrer        string         `json:"referrer,omitempty"`
	Timestamp       time.Time      `json:"timestamp"`
	Props           map[string]any `json:"props,omitempty"`
	Revenue         *Revenue       `json:"revenue,omitempty"`
	Interactive     *bool          `json:"interactive,omitempty"`
	Consent         bool           `json:"consent,omitempty"`
	IP              string         `json:"ip,omitempty"`
	UserAgent       string         `json:"user_agent,omitempty"`
	SecCHUAPlatform string         `json:"sec_ch_ua_platform,omitempty"`
	SecCHUAMobile   string         `json:"sec_ch_ua_mobile,omitempty"`
	AcceptLanguage  string         `json:"accept_language,omitempty"`
	VisitorKey      string         `json:"visitor_key,omitempty"`
	PersistentID    string         `json:"persistent_id,omitempty"`
	IdentityToken   string         `json:"identity_token,omitempty"`
}

func (e Event) wire() wireEvent {
	w := wireEvent{
		Name: e.Name, URL: e.URL, Referrer: e.Referrer, Timestamp: e.Time.UTC(), Props: e.Props, Revenue: e.Revenue, Consent: e.Consent,
		IP: e.IP, UserAgent: e.UserAgent, SecCHUAPlatform: e.SecCHUAPlatform, SecCHUAMobile: e.SecCHUAMobile,
		AcceptLanguage: e.AcceptLanguage, VisitorKey: e.VisitorKey, PersistentID: e.PersistentID, IdentityToken: e.IdentityToken,
	}
	if e.NonInteractive {
		w.Interactive = new(bool)
	}
	return w
}

// EventError is an event Yuva left out of an accepted batch as invalid.
type EventError struct {
	Event  Event
	Detail string
}

func (e *EventError) Error() string {
	return fmt.Sprintf("analytics: event %q refused: %s", e.Event.Name, e.Detail)
}

// BatchError is a batch Yuva refused as a whole, or that failed after every retry; Events counts
// its events or request records.
type BatchError struct {
	Status int
	Body   string
	Events int
	Err    error
}

func (e *BatchError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("analytics: %d events not sent: %v", e.Events, e.Err)
	}
	return fmt.Sprintf("analytics: %d events not sent: %d %s", e.Events, e.Status, e.Body)
}

func (e *BatchError) Unwrap() error { return e.Err }

// Client queues events and sends them in the background. It is safe for concurrent use.
type Client struct {
	cfg      Config
	endpoint string
	requests string
	now      func() time.Time
	sleep    func(context.Context, time.Duration) error
	sample   func(n int) bool

	mu      sync.Mutex
	queue   []Event
	records []Record
	closed  bool
	wake    chan struct{}
	flushes chan chan error
	stop    chan struct{}
	done    chan struct{}
}

// New starts a client. Close it to send what is queued and stop its goroutine.
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.PropertyID == "" {
		return nil, errors.New("analytics: BaseURL, APIKey and PropertyID are required")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = defaultInterval
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = defaultQueue
	}
	if cfg.RecordQueueSize <= 0 {
		cfg.RecordQueueSize = defaultRecords
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = defaultRetries
	}
	if cfg.MinBackoff <= 0 {
		cfg.MinBackoff = defaultMinBackoff
	}
	if cfg.ClientIP == nil {
		cfg.ClientIP = remoteIP
	}
	property := strings.TrimSuffix(cfg.BaseURL, "/") + "/v1/analytics/properties/" + cfg.PropertyID
	c := &Client{
		cfg:      cfg,
		endpoint: property + "/events",
		requests: property + "/requests",
		now:      time.Now,
		sleep:    sleepContext,
		sample:   func(n int) bool { return rand.IntN(n) == 0 },
		wake:     make(chan struct{}, 1),
		flushes:  make(chan chan error),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go c.run()
	return c, nil
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Track queues an event for the visitor of r. It forwards the visitor's address (Config.ClientIP),
// User-Agent, Sec-CH-UA-Platform, Sec-CH-UA-Mobile and Accept-Language, and takes the URL and
// referrer from r when e leaves them empty. Nothing is sent for a request with DNT: 1 or
// Sec-GPC: 1.
func (c *Client) Track(r *http.Request, e Event) error {
	if r.Header.Get("DNT") == "1" || r.Header.Get("Sec-GPC") == "1" {
		return nil
	}
	if e.VisitorKey == "" {
		if e.IP == "" {
			e.IP = c.cfg.ClientIP(r)
		}
		if e.UserAgent == "" {
			e.UserAgent = r.UserAgent()
			e.SecCHUAPlatform, e.SecCHUAMobile = r.Header.Get("Sec-CH-UA-Platform"), r.Header.Get("Sec-CH-UA-Mobile")
		}
	}
	if e.AcceptLanguage == "" {
		e.AcceptLanguage = r.Header.Get("Accept-Language")
	}
	if e.URL == "" {
		e.URL = requestURL(r)
	}
	if e.Referrer == "" {
		e.Referrer = r.Referer()
	}
	return c.Send(e)
}

func requestURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	u := scheme + "://" + r.Host + r.URL.EscapedPath()
	if r.URL.RawQuery != "" {
		u += "?" + r.URL.RawQuery
	}
	return u
}

// Send queues an event as it is.
func (c *Client) Send(e Event) error {
	if e.Name == "" || e.URL == "" {
		return errors.New("analytics: an event needs a Name and a URL")
	}
	if e.Time.IsZero() {
		e.Time = c.now()
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrClosed
	}
	if len(c.queue) >= c.cfg.QueueSize {
		c.mu.Unlock()
		c.report(ErrQueueFull)
		return ErrQueueFull
	}
	c.queue = append(c.queue, e)
	full := len(c.queue) >= MaxBatch
	c.mu.Unlock()
	if full {
		select {
		case c.wake <- struct{}{}:
		default:
		}
	}
	return nil
}

// Flush sends everything queued now and returns the last error of a batch that was not sent.
func (c *Client) Flush(ctx context.Context) error {
	reply := make(chan error, 1)
	select {
	case c.flushes <- reply:
	case <-c.done:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close sends what is queued, within ctx, and stops the client.
func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()
	err := c.Flush(ctx)
	close(c.stop)
	<-c.done
	return err
}

func (c *Client) run() {
	defer close(c.done)
	tick := time.NewTicker(c.cfg.FlushInterval)
	defer tick.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-c.stop
		cancel()
	}()
	for {
		select {
		case <-c.stop:
			return
		case reply := <-c.flushes:
			reply <- c.sendAll(ctx)
		case <-tick.C:
			_ = c.sendAll(ctx)
		case <-c.wake:
			_ = c.sendAll(ctx)
		}
	}
}

func (c *Client) take() []Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := min(len(c.queue), MaxBatch)
	batch := c.queue[:n:n]
	c.queue = c.queue[n:]
	return batch
}

func (c *Client) takeRecords() []Record {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := min(len(c.records), MaxRecordBatch)
	batch := c.records[:n:n]
	c.records = c.records[n:]
	return batch
}

func (c *Client) sendAll(ctx context.Context) error {
	var last error
	for {
		batch := c.take()
		if len(batch) == 0 {
			break
		}
		if err := c.send(ctx, batch); err != nil {
			last = err
			c.report(err)
		}
	}
	for {
		batch := c.takeRecords()
		if len(batch) == 0 {
			return last
		}
		if err := c.sendRecords(ctx, batch); err != nil {
			last = err
			c.report(err)
		}
	}
}

func (c *Client) report(err error) {
	if c.cfg.OnError != nil {
		c.cfg.OnError(err)
	}
}

type result struct {
	Accepted int `json:"accepted"`
	Errors   []struct {
		Index  int    `json:"index"`
		Detail string `json:"detail"`
	} `json:"errors"`
}

// send posts one batch, retrying network errors, 429 and 5xx.
func (c *Client) send(ctx context.Context, batch []Event) error {
	wire := make([]wireEvent, len(batch))
	for i, e := range batch {
		wire[i] = e.wire()
	}
	body, err := json.Marshal(map[string]any{"events": wire})
	if err != nil {
		return &BatchError{Events: len(batch), Err: err}
	}
	return c.deliver(ctx, c.endpoint, body, false, len(batch), func(e int, detail string) {
		c.report(&EventError{Event: batch[e], Detail: detail})
	})
}

// deliver posts one batch of n items, retrying network errors, 429 and 5xx, and reports the items
// an accepted batch left out.
func (c *Client) deliver(ctx context.Context, endpoint string, body []byte, gzipped bool, n int, refused func(i int, detail string)) error {
	backoff := c.cfg.MinBackoff
	for attempt := 0; ; attempt++ {
		status, raw, retryAfter, err := c.post(ctx, endpoint, body, gzipped)
		switch {
		case err == nil && status == http.StatusAccepted:
			var res result
			if json.Unmarshal(raw, &res) == nil {
				for _, e := range res.Errors {
					if e.Index >= 0 && e.Index < n {
						refused(e.Index, e.Detail)
					}
				}
			}
			return nil
		case err == nil && status != http.StatusTooManyRequests && status < 500:
			return &BatchError{Status: status, Body: strings.TrimSpace(string(raw)), Events: n}
		}
		if attempt >= c.cfg.MaxRetries || ctx.Err() != nil {
			if err == nil {
				return &BatchError{Status: status, Body: strings.TrimSpace(string(raw)), Events: n}
			}
			return &BatchError{Events: n, Err: err}
		}
		wait := backoff
		if retryAfter > wait {
			wait = retryAfter
		}
		if c.sleep(ctx, wait) != nil {
			return &BatchError{Events: n, Err: ctx.Err()}
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

func (c *Client) post(ctx context.Context, endpoint string, body []byte, gzipped bool) (int, []byte, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	if gzipped {
		req.Header.Set("Content-Encoding", "gzip")
	}
	res, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return 0, nil, 0, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if err != nil {
		return 0, nil, 0, err
	}
	var retryAfter time.Duration
	if s, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && s > 0 {
		retryAfter = time.Duration(s) * time.Second
	}
	return res.StatusCode, raw, retryAfter, nil
}

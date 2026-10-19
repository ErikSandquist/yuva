package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"golang.org/x/net/http/httpguts"

	"github.com/productdevbook/yuva/api/internal/jobs"
	"github.com/productdevbook/yuva/api/internal/metrics"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
	"github.com/productdevbook/yuva/api/internal/webhook"
)

const (
	monitorHTTP = string(oas.StatusMonitorHttp)
	monitorTCP  = string(oas.StatusMonitorTcp)

	minOutsideInterval     = 60
	maxOutsideInterval     = 86400
	defaultCheckTimeout    = 10
	maxCheckTimeout        = 30
	defaultCertWarnDays    = 14
	defaultOutsideFailures = 3
	defaultOutsideRecovery = 2
	maxMonitorHeaders      = 20
	maxMonitorHeaderValue  = 4096
	maxMonitorBody         = 16384
	maxMonitorKeyword      = 200
	keywordReadBytes       = 64 << 10
	maxMonitorRedirects    = 5

	canaryTimeout = 5 * time.Second
	canaryCache   = time.Minute
	canaryArmed   = time.Hour
	canaryWarn    = time.Hour
	// A round's checks are scheduled within one sweep interval and may take maxCheckTimeout each; the
	// round is judged after both and this slack.
	roundSlack    = 10 * time.Second
	staleCheckAge = time.Hour
)

// claimBudget is the worker-seconds the check queue has before a round's checks would start too
// late, if every check took its whole timeout; a sweep claims no more timeout than that.
var claimBudget = int32(jobs.StatusCheckWorkers * int((jobs.StatusMonitorSweepInterval+roundSlack)/time.Second))

var defaultCanaryURLs = []string{"https://www.google.com/generate_204", "https://www.cloudflare.com/cdn-cgi/trace"}

var (
	errNoMonitorToken = problem(http.StatusConflict, "monitor_kind", "http and tcp monitors have no token")
	refusedHeaders    = map[string]bool{"Host": true, "Content-Length": true, "Transfer-Encoding": true, "Connection": true}
)

func outsideKind(kind string) bool { return kind == monitorHTTP || kind == monitorTCP }

type monitorHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type outsideSpec struct {
	target    string
	timeout   int32
	interval  int32
	method    *string
	headers   []monitorHeader
	body      *string
	statusMin *int32
	statusMax *int32
	keyword   *string
	follow    bool
	certWarn  *int32
}

func (o outsideSpec) headerNames() []string {
	names := make([]string, len(o.headers))
	for i, h := range o.headers {
		names[i] = h.Name
	}
	return names
}

func headerSecretContext(workspaceID, monitorID uuid.UUID) []byte {
	return append(append([]byte("status-monitor-headers:"), workspaceID[:]...), monitorID[:]...)
}

func (s *Server) sealHeaders(ws, id uuid.UUID, h []monitorHeader) []byte {
	if len(h) == 0 {
		return nil
	}
	return s.secrets.Seal(mustJSON(h), headerSecretContext(ws, id))
}

func (s *Server) monitorHeaders(m store.StatusMonitor) ([]monitorHeader, error) {
	if len(m.HttpHeaders) == 0 {
		return nil, nil
	}
	plain, err := s.secrets.Open(m.HttpHeaders, headerSecretContext(m.WorkspaceID, m.ID))
	if err != nil {
		return nil, err
	}
	var out []monitorHeader
	return out, json.Unmarshal(plain, &out)
}

func (s *Server) minOutsideInterval() int32 {
	return int32(max(minOutsideInterval, s.status.MinInterval))
}

func (s *Server) checkOutsideInterval(v int32) error {
	if lo := s.minOutsideInterval(); v < lo || v > maxOutsideInterval {
		return errValidation(fmt.Sprintf("interval must be %d to %d seconds", lo, maxOutsideInterval))
	}
	return nil
}

func checkTimeout(v int32) error {
	if v < 1 || v > maxCheckTimeout {
		return errValidation("timeout must be 1 to 30 seconds")
	}
	return nil
}

var errRefusedTarget = errValidation("the target points to a private, loopback or link-local address")

// checkTargetHost refuses a host that is, or now resolves to, an address checks may not reach. A
// name that does not resolve yet is let through; every check resolves it again.
func (s *Server) checkTargetHost(ctx context.Context, host string) error {
	if err := webhook.CheckHost(host, s.status.AllowPrivate); err != nil {
		return errRefusedTarget
	}
	rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := webhook.Resolve(rctx, s.webhooks.Resolver, strings.TrimSuffix(host, "."), s.status.AllowPrivate); errors.Is(err, webhook.ErrRefusedAddress) {
		return errRefusedTarget
	}
	return nil
}

func (s *Server) checkHTTPURL(ctx context.Context, raw string) (string, error) {
	if len(raw) > 2000 {
		return "", errValidation("http.url must be at most 2000 characters")
	}
	u, err := webhook.CheckURL(raw, s.status.AllowPrivate)
	if errors.Is(err, webhook.ErrRefusedAddress) {
		return "", errRefusedTarget
	}
	if err != nil {
		return "", errValidation("http.url must be an http or https URL with a host and no credentials")
	}
	if err := s.checkTargetHost(ctx, u.Hostname()); err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *Server) checkTCPTarget(ctx context.Context, t oas.StatusMonitorTCP) (string, error) {
	host := strings.TrimSpace(t.Host)
	if host == "" || len(host) > 253 || strings.ContainsAny(host, "/ @?#[]%") {
		return "", errValidation("tcp.host must be a host name or an IP address")
	}
	if strings.Contains(host, ":") {
		if _, err := netip.ParseAddr(host); err != nil {
			return "", errValidation("tcp.host must be a host name or an IP address without a port; the port goes in tcp.port")
		}
	}
	if t.Port < 1 || t.Port > 65535 {
		return "", errValidation("tcp.port must be 1 to 65535")
	}
	if err := s.checkTargetHost(ctx, host); err != nil {
		return "", err
	}
	return net.JoinHostPort(host, strconv.Itoa(int(t.Port))), nil
}

// mergeHeaders checks the given headers; one without a value keeps the value stored for its name.
func mergeHeaders(given []oas.StatusMonitorHeader, stored []monitorHeader) ([]monitorHeader, error) {
	if len(given) > maxMonitorHeaders {
		return nil, errValidation("http.headers takes at most 20 headers")
	}
	old := map[string]string{}
	for _, h := range stored {
		old[h.Name] = h.Value
	}
	seen := map[string]bool{}
	out := make([]monitorHeader, 0, len(given))
	for _, h := range given {
		name := http.CanonicalHeaderKey(strings.TrimSpace(h.Name))
		if name == "" || len(name) > 100 || !httpguts.ValidHeaderFieldName(name) {
			return nil, errValidation("http.headers: " + strconv.Quote(h.Name) + " is not a header name")
		}
		if refusedHeaders[name] {
			return nil, errValidation("http.headers may not set " + name)
		}
		if seen[name] {
			return nil, errValidation("http.headers names " + name + " twice")
		}
		seen[name] = true
		var value string
		if h.Value != nil {
			value = *h.Value
		} else if v, ok := old[name]; ok {
			value = v
		} else {
			return nil, errValidation("http.headers: " + name + " needs a value")
		}
		if len(value) > maxMonitorHeaderValue || !httpguts.ValidHeaderFieldValue(value) {
			return nil, errValidation("http.headers: the value of " + name + " is not a valid header value")
		}
		out = append(out, monitorHeader{Name: name, Value: value})
	}
	return out, nil
}

func monitorText(v string, max int, field string) (*string, error) {
	if v == "" {
		return nil, nil
	}
	if len(v) > max {
		return nil, errValidation(field + " must be at most " + strconv.Itoa(max) + " bytes")
	}
	return &v, nil
}

type httpFields struct {
	url       *string
	method    *oas.StatusMonitorHTTPMethod
	headers   *[]oas.StatusMonitorHeader
	body      *string
	statusMin *int32
	statusMax *int32
	keyword   *string
	follow    *bool
	certWarn  *int32
}

func (s *Server) applyHTTP(ctx context.Context, o *outsideSpec, f httpFields, stored []monitorHeader) error {
	var err error
	if f.url != nil {
		if o.target, err = s.checkHTTPURL(ctx, *f.url); err != nil {
			return err
		}
	}
	if f.method != nil {
		switch m := string(*f.method); m {
		case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS":
			o.method = &m
		default:
			return errValidation("http.method must be GET, HEAD, POST, PUT, PATCH, DELETE or OPTIONS")
		}
	}
	if f.headers != nil {
		if o.headers, err = mergeHeaders(*f.headers, stored); err != nil {
			return err
		}
	} else {
		o.headers = stored
	}
	if f.body != nil {
		if o.body, err = monitorText(*f.body, maxMonitorBody, "http.body"); err != nil {
			return err
		}
	}
	if f.keyword != nil {
		if o.keyword, err = monitorText(*f.keyword, maxMonitorKeyword, "http.keyword"); err != nil {
			return err
		}
	}
	for _, v := range []*int32{f.statusMin, f.statusMax} {
		if v != nil && (*v < 100 || *v > 599) {
			return errValidation("http.expected_status_min and http.expected_status_max must be 100 to 599")
		}
	}
	if f.statusMin != nil {
		o.statusMin = f.statusMin
	}
	if f.statusMax != nil {
		o.statusMax = f.statusMax
	}
	if *o.statusMin > *o.statusMax {
		return errValidation("http.expected_status_min must not be above http.expected_status_max")
	}
	if f.follow != nil {
		o.follow = *f.follow
	}
	if f.certWarn != nil {
		if *f.certWarn < 0 || *f.certWarn > 365 {
			return errValidation("http.cert_warn_days must be 0 to 365")
		}
		o.certWarn = f.certWarn
	}
	if o.keyword != nil && *o.method == http.MethodHead {
		return errValidation("http.keyword needs a method whose answer has a body, not HEAD")
	}
	return nil
}

func (s *Server) newOutsideSpec(ctx context.Context, b oas.CreateStatusMonitorJSONRequestBody) (outsideSpec, error) {
	kind := string(b.Kind)
	o := outsideSpec{timeout: defaultCheckTimeout, interval: s.minOutsideInterval()}
	if b.Grace != nil {
		return o, errValidation("only a heartbeat monitor has grace")
	}
	if b.Interval != nil {
		if err := s.checkOutsideInterval(*b.Interval); err != nil {
			return o, err
		}
		o.interval = *b.Interval
	}
	if b.Timeout != nil {
		if err := checkTimeout(*b.Timeout); err != nil {
			return o, err
		}
		o.timeout = *b.Timeout
	}
	if kind == monitorTCP {
		if b.Tcp == nil || b.Http != nil {
			return o, errValidation("a tcp monitor takes tcp and not http")
		}
		var err error
		o.target, err = s.checkTCPTarget(ctx, *b.Tcp)
		return o, err
	}
	if b.Http == nil || b.Tcp != nil {
		return o, errValidation("an http monitor takes http and not tcp")
	}
	h := b.Http
	get, lo, hi, warn := "GET", int32(200), int32(299), int32(defaultCertWarnDays)
	o.method, o.statusMin, o.statusMax, o.certWarn = &get, &lo, &hi, &warn
	err := s.applyHTTP(ctx, &o, httpFields{url: &h.Url, method: h.Method, headers: h.Headers, body: h.Body, statusMin: h.ExpectedStatusMin,
		statusMax: h.ExpectedStatusMax, keyword: h.Keyword, follow: h.FollowRedirects, certWarn: h.CertWarnDays}, nil)
	return o, err
}

func storedSpec(m store.StatusMonitor) outsideSpec {
	o := outsideSpec{method: m.HttpMethod, body: m.HttpBody, statusMin: m.StatusMin, statusMax: m.StatusMax, keyword: m.Keyword,
		follow: m.FollowRedirects, certWarn: m.CertWarnDays}
	if m.Target != nil {
		o.target = *m.Target
	}
	if m.TimeoutSeconds != nil {
		o.timeout = *m.TimeoutSeconds
	}
	if m.IntervalSeconds != nil {
		o.interval = *m.IntervalSeconds
	}
	return o
}

func (s *Server) updateOutsideSpec(ctx context.Context, m store.StatusMonitor, b oas.UpdateStatusMonitorJSONRequestBody) (outsideSpec, bool, error) {
	o := storedSpec(m)
	if b.Grace != nil {
		return o, false, errValidation("only a heartbeat monitor has grace")
	}
	if b.Interval != nil {
		if err := s.checkOutsideInterval(*b.Interval); err != nil {
			return o, false, err
		}
		o.interval = *b.Interval
	}
	if b.Timeout != nil {
		if err := checkTimeout(*b.Timeout); err != nil {
			return o, false, err
		}
		o.timeout = *b.Timeout
	}
	before := o.target
	if m.Kind == monitorTCP {
		if b.Http != nil {
			return o, false, errValidation("a tcp monitor takes tcp and not http")
		}
		if b.Tcp != nil {
			var err error
			if o.target, err = s.checkTCPTarget(ctx, *b.Tcp); err != nil {
				return o, false, err
			}
		}
		return o, o.target != before, nil
	}
	if b.Tcp != nil {
		return o, false, errValidation("an http monitor takes http and not tcp")
	}
	stored, err := s.monitorHeaders(m)
	if err != nil {
		return o, false, err
	}
	if b.Http == nil {
		o.headers = stored
		return o, false, nil
	}
	h := b.Http
	err = s.applyHTTP(ctx, &o, httpFields{url: h.Url, method: h.Method, headers: h.Headers, body: h.Body, statusMin: h.ExpectedStatusMin,
		statusMax: h.ExpectedStatusMax, keyword: h.Keyword, follow: h.FollowRedirects, certWarn: h.CertWarnDays}, stored)
	return o, o.target != before, err
}

func outsideBody(m store.StatusMonitor, out *oas.StatusMonitor) {
	if !outsideKind(m.Kind) || m.Target == nil {
		return
	}
	out.Timeout, out.NextCheckAt, out.CertExpiresAt = m.TimeoutSeconds, m.CheckAt, m.CertExpiresAt
	if m.Kind == monitorTCP {
		host, port, _ := net.SplitHostPort(*m.Target)
		p, _ := strconv.Atoi(port)
		out.Tcp = &oas.StatusMonitorTCP{Host: host, Port: int32(p)}
		return
	}
	names := m.HttpHeaderNames
	if names == nil {
		names = []string{}
	}
	h := &oas.StatusMonitorHTTP{Url: *m.Target, HeaderNames: names, Body: m.HttpBody, Keyword: m.Keyword, FollowRedirects: m.FollowRedirects}
	if m.HttpMethod != nil {
		h.Method = oas.StatusMonitorHTTPMethod(*m.HttpMethod)
	}
	if m.StatusMin != nil && m.StatusMax != nil {
		h.ExpectedStatusMin, h.ExpectedStatusMax = *m.StatusMin, *m.StatusMax
	}
	if m.CertWarnDays != nil {
		h.CertWarnDays = *m.CertWarnDays
	}
	out.Http = h
}

type checkOutcome struct {
	ok          bool
	errText     string
	durationMs  *int32
	statusCode  *int32
	certExpires *time.Time
}

func (o *checkOutcome) fail(format string, args ...any) checkOutcome {
	o.ok, o.errText = false, fmt.Sprintf(format, args...)
	return *o
}

func millis(d time.Duration) *int32 {
	v := int32(min(d.Milliseconds(), 1<<30))
	return &v
}

func describeCheckError(err error, timeout time.Duration) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, http.ErrHandlerTimeout) {
		return fmt.Sprintf("no answer within %d seconds", int(timeout/time.Second))
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return fmt.Sprintf("no answer within %d seconds", int(timeout/time.Second))
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	return err.Error()
}

func (s *Server) dialCheck(allowPrivate bool, timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return webhook.Dial(ctx, s.webhooks.Resolver, allowPrivate, timeout, network, addr)
	}
}

// firstHop keeps the TLS state of the monitored URL's own answer when a redirect is followed, since
// the certificate checked is its host's and not a redirect target's.
type firstHop struct {
	redirected bool
	tls        *tls.ConnectionState
}

func (s *Server) checkClient(follow bool, timeout time.Duration, own []monitorHeader, first *firstHop) *http.Client {
	allow := s.status.AllowPrivate
	return &http.Client{
		Transport: &http.Transport{
			Proxy: nil, DialContext: s.dialCheck(allow, timeout), DisableKeepAlives: true, TLSClientConfig: &tls.Config{RootCAs: s.status.RootCAs},
			TLSHandshakeTimeout: timeout, ResponseHeaderTimeout: timeout, MaxResponseHeaderBytes: 64 << 10,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !follow {
				return http.ErrUseLastResponse
			}
			if len(via) == 1 && req.Response != nil {
				first.redirected, first.tls = true, req.Response.TLS
			}
			if len(via) > maxMonitorRedirects {
				return fmt.Errorf("more than %d redirects", maxMonitorRedirects)
			}
			if _, err := webhook.CheckURL(req.URL.String(), allow); err != nil {
				return fmt.Errorf("redirect to %s refused: %w", req.URL.Redacted(), err)
			}
			if req.URL.Host != via[0].URL.Host {
				for _, h := range own {
					req.Header.Del(h.Name)
				}
			}
			return nil
		},
	}
}

func (s *Server) runOutsideCheck(ctx context.Context, m store.StatusMonitor) checkOutcome {
	timeout := time.Duration(*m.TimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var out checkOutcome
	if m.Kind == monitorTCP {
		start := time.Now()
		conn, err := webhook.Dial(ctx, s.webhooks.Resolver, s.status.AllowPrivate, timeout, "tcp", *m.Target)
		if err != nil {
			return out.fail("%s", describeCheckError(err, timeout))
		}
		out.durationMs = millis(time.Since(start))
		_ = conn.Close()
		out.ok = true
		return out
	}
	headers, err := s.monitorHeaders(m)
	if err != nil {
		return out.fail("the monitor's headers cannot be read")
	}
	var body io.Reader
	if m.HttpBody != nil {
		body = strings.NewReader(*m.HttpBody)
	}
	req, err := http.NewRequestWithContext(ctx, *m.HttpMethod, *m.Target, body)
	if err != nil {
		return out.fail("%s", err.Error())
	}
	req.Header.Set("User-Agent", "Yuva-Status/"+s.version)
	for _, h := range headers {
		req.Header.Set(h.Name, h.Value)
	}
	start := time.Now()
	var first firstHop
	res, err := s.checkClient(m.FollowRedirects, timeout, headers, &first).Do(req)
	if err != nil {
		return out.fail("%s", describeCheckError(err, timeout))
	}
	defer res.Body.Close()
	out.durationMs, out.statusCode = millis(time.Since(start)), new(int32(res.StatusCode))
	state := res.TLS
	if first.redirected {
		state = first.tls
	}
	if state != nil && len(state.PeerCertificates) > 0 {
		exp := state.PeerCertificates[0].NotAfter
		out.certExpires = &exp
	}
	if res.StatusCode < int(*m.StatusMin) || res.StatusCode > int(*m.StatusMax) {
		msg := fmt.Sprintf("answered %d, expected %d to %d", res.StatusCode, *m.StatusMin, *m.StatusMax)
		if res.StatusCode >= 300 && res.StatusCode < 400 && !m.FollowRedirects {
			msg += "; redirects are not followed"
		}
		return out.fail("%s", msg)
	}
	if m.Keyword != nil {
		b, err := io.ReadAll(io.LimitReader(res.Body, keywordReadBytes))
		if err != nil {
			return out.fail("%s", describeCheckError(err, timeout))
		}
		if !bytes.Contains(b, []byte(*m.Keyword)) {
			return out.fail("the first 64 KiB of the answer do not contain the keyword")
		}
	}
	if out.certExpires != nil && m.CertWarnDays != nil && *m.CertWarnDays > 0 {
		if left := out.certExpires.Sub(s.now()); left < time.Duration(*m.CertWarnDays)*24*time.Hour {
			return out.fail("the certificate expires on %s, within %d days", out.certExpires.UTC().Format(time.DateOnly), *m.CertWarnDays)
		}
	}
	out.ok = true
	return out
}

// canariesAnswer reports whether any canary answers at all, and otherwise the last error.
func (s *Server) canariesAnswer(ctx context.Context) (bool, error) {
	urls := s.status.CanaryURLs
	if len(urls) == 0 {
		urls = defaultCanaryURLs
	}
	client := &http.Client{
		Timeout:       canaryTimeout,
		Transport:     &http.Transport{Proxy: nil, DialContext: s.dialCheck(true, canaryTimeout), DisableKeepAlives: true, TLSClientConfig: &tls.Config{RootCAs: s.status.RootCAs}},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		up   bool
		last error
	)
	for _, u := range urls {
		wg.Go(func() {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
			if err == nil {
				req.Header.Set("User-Agent", "Yuva-Status/"+s.version)
				var res *http.Response
				if res, err = client.Do(req); err == nil {
					_ = res.Body.Close()
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				last = err
				return
			}
			up = true
		})
	}
	wg.Wait()
	return up, last
}

type canaryGuard struct {
	mu       sync.Mutex
	probedAt time.Time
	up       bool
	lastUp   time.Time
	lastErr  error
	warnedAt time.Time
}

// serverTrouble reports whether failing checks are the server's own trouble: every canary fails while
// they answered within the last hour. Canaries that have not answered for an hour leave the guard
// off, so failures count, and the server says so once an hour.
func (s *Server) serverTrouble(ctx context.Context) bool {
	if s.status.CanariesOff {
		return false
	}
	g := &s.canaries
	g.mu.Lock()
	defer g.mu.Unlock()
	now := s.now()
	if g.probedAt.IsZero() || now.Sub(g.probedAt) >= canaryCache {
		up, err := s.canariesAnswer(ctx)
		g.probedAt, g.up = now, up
		if up {
			g.lastUp = now
		} else {
			g.lastErr = err
		}
	}
	if g.up {
		return false
	}
	if !g.lastUp.IsZero() && now.Sub(g.lastUp) < canaryArmed {
		return true
	}
	if g.warnedAt.IsZero() || now.Sub(g.warnedAt) >= canaryWarn {
		g.warnedAt = now
		s.log.WarnContext(ctx, "status canaries are unreachable, so failed checks count even when this server's own network is down; "+
			"set YUVA_STATUS_CANARY_URLS to URLs this server can reach, or to off", slog.Any("error", g.lastErr))
	}
	return false
}

// StatusCheckArgs is one http or tcp check of a round; a check that starts too late to finish by the
// round's deadline still runs and is judged in a round of its own.
type StatusCheckArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	MonitorID   uuid.UUID `json:"monitor_id"`
	RoundID     uuid.UUID `json:"round_id"`
	Deadline    time.Time `json:"deadline"`
}

func (StatusCheckArgs) Kind() string { return "status_check" }

func (StatusCheckArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.StatusCheckQueue, MaxAttempts: 1}
}

type StatusRoundArgs struct {
	RoundID uuid.UUID `json:"round_id"`
}

func (StatusRoundArgs) Kind() string { return "status_round" }

func (StatusRoundArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.StatusCheckQueue, MaxAttempts: 5}
}

type statusCheckWorker struct {
	river.WorkerDefaults[StatusCheckArgs]
	s *Server
}

func (w *statusCheckWorker) Work(ctx context.Context, job *river.Job[StatusCheckArgs]) error {
	return w.s.RunStatusCheck(ctx, job.Args)
}

func (w *statusCheckWorker) Timeout(*river.Job[StatusCheckArgs]) time.Duration {
	return maxCheckTimeout*time.Second + roundSlack
}

type statusRoundWorker struct {
	river.WorkerDefaults[StatusRoundArgs]
	s *Server
}

func (w *statusRoundWorker) Work(ctx context.Context, job *river.Job[StatusRoundArgs]) error {
	return w.s.SettleStatusRound(ctx, job.Args)
}

// ScheduleStatusChecks starts a round: it claims the http and tcp checks due within the next sweep and
// queues each at its time and the round's judging after them.
func (s *Server) ScheduleStatusChecks(ctx context.Context) error {
	now := s.now()
	until := now.Add(jobs.StatusMonitorSweepInterval)
	due, err := s.st.AnyDueStatusChecks(ctx, until)
	if err != nil || !due {
		return err
	}
	round := newID()
	deadline := until.Add(maxCheckTimeout*time.Second + roundSlack)
	return s.st.InTxRaw(ctx, func(tx pgx.Tx, q *store.Queries) error {
		claimed, err := q.ClaimDueStatusChecks(ctx, store.ClaimDueStatusChecksParams{Until: until, Budget: claimBudget})
		if err != nil || len(claimed) == 0 {
			return err
		}
		params := make([]river.InsertManyParams, 0, len(claimed)+1)
		for _, c := range claimed {
			params = append(params, river.InsertManyParams{
				Args:       StatusCheckArgs{WorkspaceID: c.WorkspaceID, MonitorID: c.ID, RoundID: round, Deadline: deadline},
				InsertOpts: &river.InsertOpts{Queue: jobs.StatusCheckQueue, MaxAttempts: 1, ScheduledAt: maxTime(c.Due, now)},
			})
		}
		params = append(params, river.InsertManyParams{
			Args:       StatusRoundArgs{RoundID: round},
			InsertOpts: &river.InsertOpts{Queue: jobs.StatusCheckQueue, MaxAttempts: 5, ScheduledAt: deadline.Add(roundSlack)},
		})
		_, err = s.jobs.InsertManyTx(ctx, tx, params)
		return err
	})
}

// RunStatusCheck runs one check and keeps its result pending until its round is judged.
func (s *Server) RunStatusCheck(ctx context.Context, a StatusCheckArgs) error {
	m, err := s.st.GetStatusMonitorByID(ctx, store.GetStatusMonitorByIDParams{WorkspaceID: a.WorkspaceID, ID: a.MonitorID})
	if store.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if m.Paused || !outsideKind(m.Kind) || m.Target == nil || m.TimeoutSeconds == nil {
		return nil
	}
	if live, err := s.workspaceLive(ctx, a.WorkspaceID); err != nil || !live {
		return err
	}
	at := s.now()
	round, late := a.RoundID, at.Add(time.Duration(*m.TimeoutSeconds)*time.Second).After(a.Deadline)
	if late {
		round = newID()
		s.log.WarnContext(ctx, "status check started too late for its round and is judged on its own", slog.String("workspace_id", a.WorkspaceID.String()),
			slog.String("monitor_id", a.MonitorID.String()))
	}
	out := s.runOutsideCheck(ctx, m)
	return s.st.InTxRaw(ctx, func(tx pgx.Tx, q *store.Queries) error {
		if err := q.CreatePendingStatusCheck(ctx, store.CreatePendingStatusCheckParams{
			ID: newID(), WorkspaceID: m.WorkspaceID, MonitorID: m.ID, At: at, Ok: out.ok, Source: m.Kind, Error: messagePtr(out.errText),
			DurationMs: out.durationMs, StatusCode: out.statusCode, RoundID: &round,
		}); err != nil {
			return err
		}
		if out.certExpires != nil {
			if err := q.SetStatusMonitorCert(ctx, store.SetStatusMonitorCertParams{WorkspaceID: m.WorkspaceID, ID: m.ID, CertExpiresAt: out.certExpires}); err != nil {
				return err
			}
		}
		if late {
			_, err := s.jobs.InsertTx(ctx, tx, StatusRoundArgs{RoundID: round}, nil)
			return err
		}
		return nil
	})
}

// SettleStatusRound applies a round's results, or drops its failures when they are the server's own
// trouble. The judgement is stored on the round's checks before any is applied, so a retry applies
// the same one.
func (s *Server) SettleStatusRound(ctx context.Context, a StatusRoundArgs) error {
	rows, err := s.st.ListRoundStatusChecks(ctx, &a.RoundID)
	if err != nil || len(rows) == 0 {
		return err
	}
	var decided *bool
	failures := 0
	for _, r := range rows {
		if !r.Ok {
			failures++
		}
		if r.RoundDropped != nil {
			decided = r.RoundDropped
		}
	}
	drop := decided == nil && failures > 0 && s.serverTrouble(ctx)
	if decided != nil {
		drop = *decided
	}
	if err := s.st.DecideStatusRound(ctx, store.DecideStatusRoundParams{Dropped: &drop, RoundID: &a.RoundID}); err != nil {
		return err
	}
	if drop && decided == nil {
		metrics.StatusRoundsDropped.Inc()
		s.log.WarnContext(ctx, "status round dropped because every canary failed", slog.Int("failures", failures), slog.Int("checks", len(rows)))
	}
	var errs []error
	for _, r := range rows {
		if drop && !r.Ok {
			if err := s.st.DeleteStatusCheck(ctx, store.DeleteStatusCheckParams{WorkspaceID: r.WorkspaceID, ID: r.ID}); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		res := monitorResult{ok: r.Ok, source: r.Source, check: &r.ID}
		if r.Error != nil {
			res.errText = *r.Error
		}
		if _, err := s.takeMonitorResult(ctx, r.WorkspaceID, r.MonitorID, res); err != nil && err != errStatusMonitorGone {
			s.log.ErrorContext(ctx, "status round", slog.String("workspace_id", r.WorkspaceID.String()),
				slog.String("monitor_id", r.MonitorID.String()), slog.Any("error", err))
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *Server) GetStatusMonitorCheckSummary(ctx context.Context, req oas.GetStatusMonitorCheckSummaryRequestObject) (oas.GetStatusMonitorCheckSummaryResponseObject, error) {
	p := principalFrom(ctx)
	pg, err := visibleStatusPage(ctx, s.st.Queries, p, req.PageId, false)
	if err != nil {
		return nil, err
	}
	m, err := visibleStatusMonitor(ctx, s.st.Queries, pg, req.MonitorId)
	if err != nil {
		return nil, err
	}
	rng, bucket, span := oas.StatusCheckRange24h, 15*time.Minute, 24*time.Hour
	if req.Params.Range != nil {
		switch *req.Params.Range {
		case oas.StatusCheckRange24h:
		case oas.StatusCheckRange7d:
			rng, bucket, span = oas.StatusCheckRange7d, time.Hour, 7*24*time.Hour
		default:
			return nil, errValidation("range must be 24h or 7d")
		}
	}
	until := s.now().Truncate(bucket).Add(bucket)
	since := until.Add(-span)
	rows, err := s.st.StatusCheckBuckets(ctx, store.StatusCheckBucketsParams{
		BucketSeconds: int32(bucket / time.Second), WorkspaceID: p.workspaceID, MonitorID: m.ID, Since: since, Until: until,
	})
	if err != nil {
		return nil, err
	}
	tot, err := s.st.StatusCheckTotals(ctx, store.StatusCheckTotalsParams{WorkspaceID: p.workspaceID, MonitorID: m.ID, Since: since, Until: until})
	if err != nil {
		return nil, err
	}
	byStart := make(map[int64]store.StatusCheckBucketsRow, len(rows))
	for _, r := range rows {
		byStart[r.Start.Unix()] = r
	}
	ms := func(v int32) *int32 {
		if v < 0 {
			return nil
		}
		return &v
	}
	out := oas.GetStatusMonitorCheckSummary200JSONResponse{
		Range: rng, From: since, To: until, BucketSeconds: int32(bucket / time.Second), Checks: tot.Checks, Failures: tot.Failures,
		P50Ms: ms(tot.P50), P95Ms: ms(tot.P95), Buckets: make([]oas.StatusCheckBucket, 0, int(span/bucket)),
	}
	if tot.Checks > 0 {
		up := float64(tot.Checks-tot.Failures) * 100 / float64(tot.Checks)
		out.Uptime = &up
	}
	for t := since; t.Before(until); t = t.Add(bucket) {
		b := oas.StatusCheckBucket{Start: t}
		if r, ok := byStart[t.Unix()]; ok {
			b.Checks, b.Failures, b.P50Ms, b.P95Ms = r.Checks, r.Failures, ms(r.P50), ms(r.P95)
		}
		out.Buckets = append(out.Buckets, b)
	}
	return out, nil
}

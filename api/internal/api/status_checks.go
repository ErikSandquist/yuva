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
	// A round's checks are scheduled within one sweep interval and may take maxCheckTimeout each; the
	// round is judged after both and this slack.
	roundSlack = 10 * time.Second
	// roundDropFloor is the least number of failures that, as more than half a round, drop it.
	roundDropFloor = 10
	staleCheckAge  = time.Hour
)

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

// outsideSpec is what an http or tcp monitor checks.
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
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if host == "" || len(host) > 253 || strings.ContainsAny(host, "/ @?#") {
		return "", errValidation("tcp.host must be a host name or an IP address")
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

// httpFields applies the given http settings over o, as given on create (in) or update (up).
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

// newOutsideSpec reads an http or tcp monitor's settings from a create request.
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

// updateOutsideSpec applies an update request to an http or tcp monitor's stored settings.
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

// describeCheckError names why a request or connection failed, without Go's wrapping.
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

// checkClient follows redirects only when follow is set, at most 5, each hop checked again; the
// monitor's own headers never go to another host.
func (s *Server) checkClient(follow bool, timeout time.Duration, own []monitorHeader) *http.Client {
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
	res, err := s.checkClient(m.FollowRedirects, timeout, headers).Do(req)
	if err != nil {
		return out.fail("%s", describeCheckError(err, timeout))
	}
	defer res.Body.Close()
	out.durationMs, out.statusCode = millis(time.Since(start)), new(int32(res.StatusCode))
	if res.TLS != nil && len(res.TLS.PeerCertificates) > 0 {
		exp := res.TLS.PeerCertificates[0].NotAfter
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

// canariesAnswer reports whether any canary answers at all; the server's own network is then up.
func (s *Server) canariesAnswer(ctx context.Context) bool {
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
		wg sync.WaitGroup
		mu sync.Mutex
		up bool
	)
	for _, u := range urls {
		wg.Go(func() {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", "Yuva-Status/"+s.version)
			res, err := client.Do(req)
			if err != nil {
				s.log.WarnContext(ctx, "status canary failed", slog.String("url", u), slog.Any("error", err))
				return
			}
			_ = res.Body.Close()
			mu.Lock()
			up = true
			mu.Unlock()
		})
	}
	wg.Wait()
	return up
}

// StatusCheckArgs is one http or tcp check of a round; a check that cannot finish before the
// round's deadline is skipped.
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

// StatusRoundArgs judges a round once its checks are done.
type StatusRoundArgs struct {
	RoundID      uuid.UUID `json:"round_id"`
	CanariesDown bool      `json:"canaries_down"`
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

// ScheduleStatusChecks starts a round: it claims the http and tcp checks due within the next sweep,
// checks the canaries, and queues each check at its time and the round's judging after them.
func (s *Server) ScheduleStatusChecks(ctx context.Context) error {
	now := s.now()
	until := now.Add(jobs.StatusMonitorSweepInterval)
	due, err := s.st.AnyDueStatusChecks(ctx, until)
	if err != nil || !due {
		return err
	}
	canariesDown := !s.canariesAnswer(ctx)
	round := newID()
	deadline := until.Add(maxCheckTimeout*time.Second + roundSlack)
	return s.st.InTxRaw(ctx, func(tx pgx.Tx, q *store.Queries) error {
		claimed, err := q.ClaimDueStatusChecks(ctx, until)
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
			Args:       StatusRoundArgs{RoundID: round, CanariesDown: canariesDown},
			InsertOpts: &river.InsertOpts{Queue: jobs.StatusCheckQueue, MaxAttempts: 5, ScheduledAt: deadline.Add(roundSlack)},
		})
		_, err = s.jobs.InsertManyTx(ctx, tx, params)
		return err
	})
}

// RunStatusCheck runs one check and keeps its result pending until the round is judged.
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
	at := s.now()
	if at.Add(time.Duration(*m.TimeoutSeconds) * time.Second).After(a.Deadline) {
		s.log.WarnContext(ctx, "status check started too late for its round", slog.String("workspace_id", a.WorkspaceID.String()),
			slog.String("monitor_id", a.MonitorID.String()))
		return nil
	}
	if live, err := s.workspaceLive(ctx, a.WorkspaceID); err != nil || !live {
		return err
	}
	out := s.runOutsideCheck(ctx, m)
	round := a.RoundID
	if err := s.st.CreatePendingStatusCheck(ctx, store.CreatePendingStatusCheckParams{
		ID: newID(), WorkspaceID: m.WorkspaceID, MonitorID: m.ID, At: at, Ok: out.ok, Source: m.Kind, Error: messagePtr(out.errText),
		DurationMs: out.durationMs, StatusCode: out.statusCode, RoundID: &round,
	}); err != nil {
		return err
	}
	if out.certExpires != nil {
		return s.st.SetStatusMonitorCert(ctx, store.SetStatusMonitorCertParams{WorkspaceID: m.WorkspaceID, ID: m.ID, CertExpiresAt: out.certExpires})
	}
	return nil
}

// SettleStatusRound counts a round's results, or, when every canary failed or more than half of
// its checks (at least 10) failed, drops its failures as the server's own trouble.
func (s *Server) SettleStatusRound(ctx context.Context, a StatusRoundArgs) error {
	rows, err := s.st.ListRoundStatusChecks(ctx, &a.RoundID)
	if err != nil {
		return err
	}
	failures := 0
	for _, r := range rows {
		if !r.Ok {
			failures++
		}
	}
	reason := ""
	switch {
	case failures == 0:
	case a.CanariesDown:
		reason = "canaries"
	case failures >= roundDropFloor && failures*2 > len(rows):
		reason = "majority"
	}
	if reason != "" {
		metrics.StatusRoundsDropped.WithLabelValues(reason).Inc()
		s.log.WarnContext(ctx, "status round dropped as the server's own trouble", slog.String("reason", reason),
			slog.Int("failures", failures), slog.Int("checks", len(rows)))
	}
	var errs []error
	for _, r := range rows {
		if reason != "" && !r.Ok {
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

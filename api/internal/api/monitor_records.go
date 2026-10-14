package api

import (
	"compress/gzip"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/monitor"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
	"github.com/productdevbook/yuva/api/internal/webhook"
)

const (
	maxRequestRecords              = 1000
	maxRecordDurationUS            = int64(24 * time.Hour / time.Microsecond)
	maxSampleRate                  = 10_000
	defaultMonitorRecordsPerMinute = 600_000
	defaultMonitorBufferKeys       = 200_000
	defaultMonitorMinuteDays       = 8
	monitorBusyRetry               = 10
	defaultMonitorChecks           = 500
	defaultMonitorMinInterval      = 30 * time.Second
	defaultMonitorCheckDays        = 30
)

var (
	errMonitorNeedsKey = problem(http.StatusForbidden, "forbidden", "only an API key can send request records")
	errMonitorBusy     = &apiError{Status: http.StatusServiceUnavailable, Code: "unavailable", Detail: "the server holds as many open rows as it may; try again shortly",
		RetryAfter: monitorBusyRetry}
	errEncoding = problem(http.StatusUnsupportedMediaType, "unsupported_media_type", "send the body as it is or with Content-Encoding: gzip")
)

// MonitorSettings: zero values mean the defaults; Off removes API monitoring from the server.
type MonitorSettings struct {
	Off bool
	// MonthlyRequests caps each workspace's request records a month; 0 is no limit.
	MonthlyRequests  int64
	RecordsPerMinute int
	BufferKeys       int
	MinuteDays       int
	// Checks caps a workspace's uptime checks, MinInterval is their shortest interval and CheckDays
	// how long their runs are kept; AllowPrivate lets them call private and loopback addresses.
	Checks       int
	MinInterval  time.Duration
	CheckDays    int
	AllowPrivate bool
	// Resolver and Connect replace the uptime runner's name lookups and connections; for tests.
	Resolver webhook.Resolver
	Connect  func(ctx context.Context, network, address string) (net.Conn, error)
}

func monitorSettings(m MonitorSettings) MonitorSettings {
	if m.RecordsPerMinute <= 0 {
		m.RecordsPerMinute = defaultMonitorRecordsPerMinute
	}
	if m.BufferKeys <= 0 {
		m.BufferKeys = defaultMonitorBufferKeys
	}
	if m.MinuteDays <= 0 {
		m.MinuteDays = defaultMonitorMinuteDays
	}
	if m.Checks <= 0 {
		m.Checks = defaultMonitorChecks
	}
	if m.MinInterval <= 0 {
		m.MinInterval = defaultMonitorMinInterval
	}
	if m.CheckDays <= 0 {
		m.CheckDays = defaultMonitorCheckDays
	}
	return m
}

type monitorState struct {
	settings MonitorSettings
	store    *monitor.PGStore
	agg      *monitor.Aggregator
	usage    *usageCache
	releases releaseCache
	prober   *monitor.Prober
	resolver webhook.Resolver
	parts    sync.Map
}

func (s *Server) newMonitor(d Deps) *monitorState {
	set := monitorSettings(d.Analytics.Monitor)
	st := &monitor.PGStore{Pool: d.Store.Pool}
	resolver := set.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return &monitorState{
		settings: set, store: st, usage: &usageCache{m: map[uuid.UUID]*usageEntry{}}, resolver: resolver,
		prober: monitor.NewProber(set.AllowPrivate, resolver, set.Connect),
		agg:    &monitor.Aggregator{Store: st, Log: s.log, Now: s.now, MaxKeys: set.BufferKeys, AfterFlush: s.afterMonitorFlush},
	}
}

func (s *Server) monitorOn() (*monitorState, error) {
	if s.analytics == nil || s.analytics.monitor == nil {
		return nil, errNotFound
	}
	return s.analytics.monitor, nil
}

// FlushMonitor writes the open request rows now.
func (s *Server) FlushMonitor(ctx context.Context) error {
	m, err := s.monitorOn()
	if err != nil {
		return nil
	}
	return m.agg.Flush(ctx)
}

func (s *Server) workspaceMonitorLimit(override *int64) int64 {
	if override != nil {
		return *override
	}
	return s.analytics.monitor.settings.MonthlyRequests
}

func (s *Server) loadMonitorUsage(ctx context.Context, ws uuid.UUID, now time.Time) (*usageEntry, error) {
	month := monthOf(now)
	c := s.analytics.monitor.usage
	c.mu.Lock()
	e, ok := c.m[ws]
	c.mu.Unlock()
	if ok && e.month.Equal(month) && now.Sub(e.loaded) < usageCacheTTL && !now.Before(e.loaded) {
		return e, nil
	}
	u, err := s.st.GetMonitorUsage(ctx, store.GetMonitorUsageParams{WorkspaceID: ws, Month: month})
	if err != nil {
		return nil, err
	}
	e = &usageEntry{month: month, limit: s.workspaceMonitorLimit(u.MonthlyLimit), events: u.Requests, reached: u.LimitReachedAt != nil, loaded: now}
	c.mu.Lock()
	c.m[ws] = e
	c.mu.Unlock()
	return e, nil
}

// takeMonitorUsage counts up to n records against the workspace's monthly cap and says how many
// fit, so a workspace's records reach the cap exactly.
func (s *Server) takeMonitorUsage(ctx context.Context, ws uuid.UUID, n int, now time.Time) (int, error) {
	e, err := s.loadMonitorUsage(ctx, ws, now)
	if err != nil {
		return 0, err
	}
	c := s.analytics.monitor.usage
	c.mu.Lock()
	defer c.mu.Unlock()
	if e.limit > 0 {
		n = int(max(min(int64(n), e.limit-e.events), 0))
	}
	e.events += int64(n)
	return n, nil
}

func (s *Server) monitorUsage(ctx context.Context, ws uuid.UUID, out *oas.AnalyticsUsage) error {
	if s.analytics.monitor == nil {
		return nil
	}
	u, err := s.st.GetMonitorUsage(ctx, store.GetMonitorUsageParams{WorkspaceID: ws, Month: monthOf(s.now())})
	if err != nil {
		return err
	}
	out.Requests = &u.Requests
	if l := s.workspaceMonitorLimit(u.MonthlyLimit); l > 0 {
		out.RequestsLimit = &l
		if u.Requests >= l {
			out.RequestsLimitReachedAt = u.LimitReachedAt
		}
	}
	return nil
}

// OperatorLimitMonitor sets a workspace's monthly request records in place of
// YUVA_MONITOR_MONTHLY_REQUESTS; 0 is no limit.
func (s *Server) OperatorLimitMonitor(ctx context.Context, workspaceID uuid.UUID, requests int64) error {
	if requests < 0 {
		return errors.New("the limit must be a number of requests, 0 for no limit")
	}
	n, err := s.st.SetWorkspaceMonitorLimit(ctx, store.SetWorkspaceMonitorLimitParams{ID: workspaceID, MonthlyRequests: &requests})
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("no such workspace")
	}
	return nil
}

const requestsSuffix = "/requests"

// gunzipRecords decompresses a request records body sent with Content-Encoding: gzip, up to the
// same size as any other body.
func (s *Server) gunzipRecords(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/v1/analytics/properties/") || !strings.HasSuffix(r.URL.Path, requestsSuffix) {
			next.ServeHTTP(w, r)
			return
		}
		switch strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Encoding"))) {
		case "", "identity":
		case "gzip":
			zr, err := gzip.NewReader(r.Body)
			if err != nil {
				writeProblem(w, errValidation("the body is not gzip"))
				return
			}
			defer zr.Close()
			r.Body, r.ContentLength = http.MaxBytesReader(w, zr, maxBodyBytes), -1
		default:
			writeProblem(w, errEncoding)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) SendAnalyticsRequests(ctx context.Context, req oas.SendAnalyticsRequestsRequestObject) (oas.SendAnalyticsRequestsResponseObject, error) {
	m, err := s.monitorOn()
	if err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if !p.isKey() {
		return nil, errMonitorNeedsKey
	}
	prop, err := visibleProperty(ctx, s.st.Queries, p, req.PropertyId, false)
	if err != nil {
		return nil, err
	}
	if !isAPIProperty(prop) {
		return nil, errAPIOnly
	}
	if req.Body == nil || len(req.Body.Records) == 0 || len(req.Body.Records) > maxRequestRecords {
		return nil, errValidation("records must hold 1 to 1,000 records")
	}
	now, zone := s.now(), monitorZone(prop.Timezone, prop.TimezoneHistory)
	day := zone.LocalDay(now)
	count := func(reason string, n int) {
		if n > 0 {
			m.agg.Count(prop.WorkspaceID, prop.ID, day, reason, int64(n))
		}
	}
	rules := pathRules(prop.PathRules)
	out := oas.AnalyticsRequestsResult{Errors: []oas.AnalyticsEventError{}}
	recs := make([]monitor.Record, 0, len(req.Body.Records))
	late := 0
	for i := range req.Body.Records {
		rec, isLate, err := s.requestRecord(&req.Body.Records[i], zone, rules, now)
		if err != nil {
			out.Errors = append(out.Errors, oas.AnalyticsEventError{Index: i, Detail: err.Error()})
			continue
		}
		out.Accepted++
		if isLate {
			late++
			continue
		}
		recs = append(recs, rec)
	}
	count(monitor.ReasonInvalid, len(out.Errors))
	count(monitor.ReasonLate, late)
	if _, err := s.admitRecords(ctx, m, prop, keyRecordsLimit(p), recs, now, count); err != nil {
		return nil, err
	}
	return oas.SendAnalyticsRequests202JSONResponse(out), nil
}

func keyRecordsLimit(p principal) string { return "mk:" + p.keyID.String() }

// admitRecords takes the checked records that fit the property's per-minute limit and the one
// under limitKey (none when empty), then the workspace's monthly cap, and adds them up; the others
// are counted by their reason. wait is set when the limits refused the first record, so none was
// taken.
func (s *Server) admitRecords(ctx context.Context, m *monitorState, prop store.AnalyticsProperty, limitKey string, recs []monitor.Record, now time.Time,
	count func(reason string, n int)) (wait time.Duration, err error) {
	if len(recs) == 0 {
		return 0, nil
	}
	perMinute := limit{m.settings.RecordsPerMinute, time.Minute}
	taken := 0
	for range recs {
		ok, w := s.analytics.limits.allowWait("m:"+prop.ID.String(), perMinute, now)
		if ok && limitKey != "" {
			ok, w = s.analytics.limits.allowWait(limitKey, perMinute, now)
		}
		if !ok {
			if taken == 0 {
				wait = w
			}
			break
		}
		taken++
	}
	count(monitor.ReasonRate, len(recs)-taken)
	if recs = recs[:taken]; len(recs) == 0 {
		return wait, nil
	}
	fit, err := s.takeMonitorUsage(ctx, prop.WorkspaceID, len(recs), now)
	if err != nil {
		return 0, err
	}
	count(monitor.ReasonOverLimit, len(recs)-fit)
	if recs = recs[:fit]; len(recs) == 0 {
		return 0, nil
	}
	if err := m.agg.Add(ctx, monitor.Property{WorkspaceID: prop.WorkspaceID, ID: prop.ID}, recs); err != nil {
		if errors.Is(err, monitor.ErrFull) {
			count(monitor.ReasonBuffer, len(recs))
			return 0, errMonitorBusy
		}
		s.log.ErrorContext(ctx, "monitor records", slog.Any("error", err))
		return 0, err
	}
	count(monitor.ReasonAccepted, len(recs))
	s.noteReleases(ctx, m, prop, recs)
	return 0, nil
}

const maxCachedReleases = 100_000

type releaseKey struct {
	prop                          uuid.UUID
	service, environment, release string
}

// releaseCache remembers the releases this process already wrote, so only a release it has not
// seen asks the database.
type releaseCache struct {
	mu   sync.Mutex
	seen map[releaseKey]bool
}

// noteReleases writes a deploy annotation for each release the records bring that their service
// and environment never had; a failure only costs the marker.
func (s *Server) noteReleases(ctx context.Context, m *monitorState, prop store.AnalyticsProperty, recs []monitor.Record) {
	first := map[releaseKey]time.Time{}
	c := &m.releases
	c.mu.Lock()
	for i := range recs {
		r := &recs[i]
		if r.Release == "" {
			continue
		}
		k := releaseKey{prop.ID, r.Service, r.Environment, r.Release}
		if c.seen[k] {
			continue
		}
		if at, ok := first[k]; !ok || r.Time.Before(at) {
			first[k] = r.Time
		}
	}
	c.mu.Unlock()
	if len(first) == 0 {
		return
	}
	rels := make([]monitor.Release, 0, len(first))
	for k, at := range first {
		rels = append(rels, monitor.Release{Service: k.service, Environment: k.environment, Release: k.release, At: at})
	}
	if _, err := m.store.NoteReleases(ctx, monitor.Property{WorkspaceID: prop.WorkspaceID, ID: prop.ID}, rels,
		monitorZone(prop.Timezone, prop.TimezoneHistory), s.now()); err != nil {
		s.log.ErrorContext(ctx, "monitor releases", slog.Any("error", err))
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seen == nil || len(c.seen) > maxCachedReleases {
		c.seen = map[releaseKey]bool{}
	}
	for k := range first {
		c.seen[k] = true
	}
}

// recordCountry is the first and only use of a record's address: it is shortened, looked up for
// its country and dropped from the record.
func (s *Server) recordCountry(ip **string) string {
	raw := derefString(*ip)
	*ip = nil
	if raw == "" {
		return ""
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return s.analytics.geo.Lookup(analytics.ShortenAddr(addr)).Country
}

func recordText(v *string, max int, name string) (string, error) {
	if v == nil {
		return "", nil
	}
	if len(*v) > max {
		return "", errors.New(name + " has at most " + formatCount(int64(max)) + " characters")
	}
	return analytics.Clean(*v, max), nil
}

// requestRecord checks one request record; late reports a valid record older than 72 hours.
func (s *Server) requestRecord(r *oas.AnalyticsRequestRecord, zone monitor.Zone, rules []analytics.PathRule, now time.Time) (monitor.Record, bool, error) {
	country := s.recordCountry(&r.Ip)
	out := monitor.Record{Country: country, Source: monitor.SourceServer, Platform: monitor.PlatformNone, RequestBytes: -1, ResponseBytes: -1, Weight: 1}
	if m := strings.TrimSpace(r.Method); m == "" || len(m) > 20 {
		return out, false, errors.New("method must be 1 to 20 characters")
	}
	out.Method = monitor.Method(r.Method)
	if r.Status != 0 && (r.Status < 100 || r.Status > 599) {
		return out, false, errors.New("status must be 100 to 599, or 0 when no response was sent")
	}
	out.Status = int16(r.Status)
	if r.DurationUs < 0 || r.DurationUs > maxRecordDurationUS {
		return out, false, errors.New("duration_us must be 0 to 86,400,000,000 microseconds")
	}
	out.DurationUS = r.DurationUs
	for _, size := range []struct {
		v    *int64
		dst  *int64
		name string
	}{{r.RequestSize, &out.RequestBytes, "request_size"}, {r.ResponseSize, &out.ResponseBytes, "response_size"}} {
		if size.v == nil {
			continue
		}
		if *size.v < 0 {
			return out, false, errors.New(size.name + " must not be negative")
		}
		*size.dst = *size.v
	}
	route, inferred, err := recordRoute(r, out.Status, rules)
	if err != nil {
		return out, false, err
	}
	out.Route, out.Inferred = route, inferred
	if r.SampleRate != nil {
		if *r.SampleRate < 1 || *r.SampleRate > maxSampleRate {
			return out, false, errors.New("sample_rate must be 1 to 10,000")
		}
		if *r.SampleRate > 1 && (out.Status == 0 || out.Status >= 400) {
			return out, false, errors.New("only records with a status of 1 to 399 may be sampled")
		}
		out.Weight = int64(*r.SampleRate)
	}
	if r.Consumer != nil {
		c := analytics.Clean(*r.Consumer, 100)
		if c == "" || len(*r.Consumer) > 100 {
			return out, false, errors.New("consumer must be 1 to 100 characters")
		}
		out.Consumer = monitor.ScrubConsumer(c)
	}
	if out.APIVersion, err = recordText(r.ApiVersion, 40, "api_version"); err != nil {
		return out, false, err
	}
	if out.Environment, err = recordText(r.Environment, 40, "environment"); err != nil {
		return out, false, err
	}
	if out.Environment == "" {
		out.Environment = monitor.DefaultEnvironment
	}
	if out.Service, err = recordText(r.Service, 100, "service"); err != nil {
		return out, false, err
	}
	if out.Release, err = recordText(r.Release, 80, "release"); err != nil {
		return out, false, err
	}
	var at time.Time
	if r.Time != nil {
		at = *r.Time
	}
	at, late := eventTime(at, now)
	out.Time, out.Hour, out.Day = at, zone.LocalHour(at), zone.LocalDay(at)
	return out, late, nil
}

// recordRoute is the route a record is counted under: the property's path rules first, then the
// record's pattern with any id-shaped segment taken out, or a pattern inferred from its path. A
// path that answered 404 or 405 is (unmatched).
func recordRoute(r *oas.AnalyticsRequestRecord, status int16, rules []analytics.PathRule) (string, bool, error) {
	var raw string
	inferred := false
	switch {
	case r.Route != nil:
		raw = strings.TrimSpace(*r.Route)
		if !strings.HasPrefix(raw, "/") || len(raw) > monitor.MaxRoute {
			return "", false, errors.New("route must be a pattern of at most 300 characters starting with /")
		}
	case r.Path != nil:
		raw = strings.TrimSpace(*r.Path)
		if len(raw) > analytics.MaxURLLength {
			return "", false, errors.New("path has at most 2,000 characters")
		}
		if i := strings.IndexAny(raw, "?#"); i >= 0 {
			raw = raw[:i]
		}
		if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
			return monitor.Unmatched, false, nil
		}
		inferred = true
	default:
		return "", false, errors.New("give route, or path when the router knows no pattern")
	}
	if mapped := analytics.ApplyRules(rules, raw); mapped != raw {
		return mapped, false, nil
	}
	route := monitor.InferRoute(raw)
	if len(route) > monitor.MaxRoute {
		return "", false, errors.New("the route has more than 300 characters")
	}
	return route, inferred, nil
}

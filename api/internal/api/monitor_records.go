package api

import (
	"compress/gzip"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/monitor"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	maxRequestRecords              = 1000
	maxRecordDurationUS            = int64(24 * time.Hour / time.Microsecond)
	maxSampleRate                  = 10_000
	defaultMonitorRecordsPerMinute = 600_000
	defaultMonitorBufferKeys       = 200_000
	defaultMonitorMinuteDays       = 8
	monitorBusyRetry               = 10
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
	return m
}

type monitorState struct {
	settings MonitorSettings
	store    *monitor.PGStore
	agg      *monitor.Aggregator
	usage    *usageCache
}

func (s *Server) newMonitor(d Deps) *monitorState {
	set := monitorSettings(d.Analytics.Monitor)
	st := &monitor.PGStore{Pool: d.Store.Pool}
	return &monitorState{
		settings: set, store: st, usage: &usageCache{m: map[uuid.UUID]*usageEntry{}},
		agg: &monitor.Aggregator{Store: st, Log: s.log, Now: s.now, MaxKeys: set.BufferKeys},
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
	e = &usageEntry{month: month, limit: s.workspaceMonitorLimit(u.MonthlyLimit), events: u.Requests, loaded: now}
	c.mu.Lock()
	c.m[ws] = e
	c.mu.Unlock()
	return e, nil
}

// takeMonitorUsage counts n records against the workspace's monthly cap; false means the cap is
// reached.
func (s *Server) takeMonitorUsage(ctx context.Context, ws uuid.UUID, n int, now time.Time) (bool, error) {
	e, err := s.loadMonitorUsage(ctx, ws, now)
	if err != nil {
		return false, err
	}
	c := s.analytics.monitor.usage
	c.mu.Lock()
	defer c.mu.Unlock()
	if e.limit > 0 && e.events+int64(n) > e.limit {
		return false, nil
	}
	e.events += int64(n)
	return true, nil
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
	now, loc := s.now(), propertyLocation(prop.Timezone)
	day := monitor.LocalDay(now, loc)
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
		rec, isLate, err := s.requestRecord(&req.Body.Records[i], loc, rules, now)
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
	if len(recs) == 0 {
		return oas.SendAnalyticsRequests202JSONResponse(out), nil
	}
	taken := 0
	for range recs {
		if !s.analytics.limits.allow("m:"+prop.ID.String(), limit{m.settings.RecordsPerMinute, time.Minute}, now) ||
			!s.analytics.limits.allow("mk:"+p.keyID.String(), limit{m.settings.RecordsPerMinute, time.Minute}, now) {
			break
		}
		taken++
	}
	count(monitor.ReasonRate, len(recs)-taken)
	recs = recs[:taken]
	if len(recs) == 0 {
		return oas.SendAnalyticsRequests202JSONResponse(out), nil
	}
	ok, err := s.takeMonitorUsage(ctx, prop.WorkspaceID, len(recs), now)
	if err != nil {
		return nil, err
	}
	if !ok {
		count(monitor.ReasonOverLimit, len(recs))
		return oas.SendAnalyticsRequests202JSONResponse(out), nil
	}
	if err := m.agg.Add(ctx, monitor.Property{WorkspaceID: prop.WorkspaceID, ID: prop.ID, Location: loc}, recs); err != nil {
		if errors.Is(err, monitor.ErrFull) {
			count(monitor.ReasonBuffer, len(recs))
			return nil, errMonitorBusy
		}
		s.log.ErrorContext(ctx, "monitor records", slog.Any("error", err))
		return nil, err
	}
	count(monitor.ReasonAccepted, len(recs))
	return oas.SendAnalyticsRequests202JSONResponse(out), nil
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
func (s *Server) requestRecord(r *oas.AnalyticsRequestRecord, loc *time.Location, rules []analytics.PathRule, now time.Time) (monitor.Record, bool, error) {
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
	if _, err = recordText(r.Release, 80, "release"); err != nil {
		return out, false, err
	}
	var at time.Time
	if r.Time != nil {
		at = *r.Time
	}
	at, late := eventTime(at, now)
	out.Time, out.Hour, out.Day = at, monitor.LocalHour(at, loc), monitor.LocalDay(at, loc)
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

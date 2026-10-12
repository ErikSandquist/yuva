package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/mail"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	analyticsPrefix          = "/client/v1/analytics/"
	maxAnalyticsBody         = 64 << 10
	maxAnalyticsBatch        = 100
	maxEngagedMS             = 30 * 60 * 1000
	propertyCacheTTL         = time.Minute
	maxCachedKeys            = 10_000
	usageCacheTTL            = 30 * time.Second
	analyticsBufferRetry     = 5
	defaultAnalyticsBuffer   = 50_000
	defaultVisitCache        = 200_000
	defaultSaltGrace         = 2 * time.Hour
	defaultPropertyPerMinute = 60_000
	defaultPropertiesPerWS   = 50
	noticeAnalyticsLimit     = "analytics_limit"
	analyticsPropertyKey     = ctxKey(200)
)

var (
	limitAnalyticsPerAddress = limit{120, time.Minute}
	errPageNotAllowed        = problem(http.StatusForbidden, "page_not_allowed", "the page's host is not one of the property's domains")
	errPropertyOrigin        = problem(http.StatusForbidden, "origin_not_allowed", "this origin is not one of the property's domains")
	errAnalyticsBodyTooLarge = problem(http.StatusRequestEntityTooLarge, "payload_too_large", "send at most 64 KiB")
	errAnalyticsBusy         = &apiError{Status: http.StatusServiceUnavailable, Code: "unavailable", Detail: "the server takes no more events right now; try again shortly", RetryAfter: analyticsBufferRetry}
)

// AnalyticsSettings: zero values mean the defaults; Off removes analytics from the server.
type AnalyticsSettings struct {
	Off                     bool
	Buffer                  int
	VisitCache              int
	SaltGrace               time.Duration
	MonthlyEvents           int64
	PropertyEventsPerMinute int
	PropertiesPerWorkspace  int
}

func analyticsSettings(a AnalyticsSettings) AnalyticsSettings {
	if a.Buffer <= 0 {
		a.Buffer = defaultAnalyticsBuffer
	}
	if a.VisitCache <= 0 {
		a.VisitCache = defaultVisitCache
	}
	if a.SaltGrace <= 0 {
		a.SaltGrace = defaultSaltGrace
	}
	if a.PropertyEventsPerMinute <= 0 {
		a.PropertyEventsPerMinute = defaultPropertyPerMinute
	}
	if a.PropertiesPerWorkspace <= 0 {
		a.PropertiesPerWorkspace = defaultPropertiesPerWS
	}
	return a
}

type analyticsState struct {
	settings AnalyticsSettings
	store    analytics.Store
	buffer   *analytics.Buffer
	flusher  *analytics.Flusher
	resolver *analytics.Resolver
	props    *propertyCache
	usage    *usageCache
	limits   *rateLimiter
}

func (s *Server) newAnalytics(d Deps) *analyticsState {
	set := analyticsSettings(d.Analytics)
	st := &analytics.PGStore{Pool: d.Store.Pool}
	a := &analyticsState{
		settings: set, store: st, buffer: analytics.NewBuffer(set.Buffer), props: &propertyCache{m: map[string]*cachedProperty{}},
		usage: &usageCache{m: map[uuid.UUID]*usageEntry{}}, limits: newRateLimiter(),
		resolver: &analytics.Resolver{Store: st, Salts: analytics.NewSalts(st), Visits: analytics.NewVisits(set.VisitCache), Grace: set.SaltGrace},
	}
	a.flusher = &analytics.Flusher{Store: st, Buffer: a.buffer, Log: s.log, Now: s.now, AfterFlush: s.afterAnalyticsFlush}
	return a
}

func (a *analyticsState) forget(propertyID uuid.UUID) {
	a.props.forget(propertyID)
	a.resolver.Salts.Forget(propertyID)
	a.resolver.Visits.Forget(propertyID)
}

// RunAnalytics writes collected events until ctx ends and then drains what is left; it returns at
// once when analytics is off.
func (s *Server) RunAnalytics(ctx context.Context) {
	if s.analytics == nil {
		return
	}
	if err := s.analytics.flusher.Prepare(ctx); err != nil {
		s.log.ErrorContext(ctx, "analytics partitions", slog.Any("error", err))
	}
	s.analytics.flusher.Run(ctx)
}

// FlushAnalytics writes the collected events now.
func (s *Server) FlushAnalytics(ctx context.Context) error {
	if s.analytics == nil {
		return nil
	}
	return s.analytics.flusher.Flush(ctx)
}

type cachedProperty struct {
	prop  store.AnalyticsProperty
	loc   *time.Location
	rules []analytics.PathRule
	at    time.Time
}

// propertyCache finds properties by public key for a minute; an unknown key is remembered too, so
// a flood of made-up keys does not reach the database on every request.
type propertyCache struct {
	mu sync.Mutex
	m  map[string]*cachedProperty
}

func (c *propertyCache) get(ctx context.Context, st *store.Store, key string, now time.Time) (*cachedProperty, error) {
	c.mu.Lock()
	e, ok := c.m[key]
	c.mu.Unlock()
	if ok && now.Sub(e.at) < propertyCacheTTL && now.After(e.at.Add(-time.Second)) {
		if e.prop.ID == uuid.Nil() {
			return nil, nil
		}
		return e, nil
	}
	e = &cachedProperty{at: now}
	prop, err := st.FindAnalyticsPropertyByKey(ctx, store.FindAnalyticsPropertyByKeyParams{Key: key, Now: new(now)})
	switch {
	case store.IsNotFound(err):
	case err != nil:
		return nil, err
	default:
		e.prop, e.loc, e.rules = prop, propertyLocation(prop.Timezone), pathRules(prop.PathRules)
	}
	c.mu.Lock()
	if len(c.m) >= maxCachedKeys {
		c.m = map[string]*cachedProperty{}
	}
	c.m[key] = e
	c.mu.Unlock()
	if e.prop.ID == uuid.Nil() {
		return nil, nil
	}
	return e, nil
}

func (c *propertyCache) forgetWorkspace(ws uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.m {
		if e.prop.WorkspaceID == ws {
			delete(c.m, k)
		}
	}
}

func (c *propertyCache) forget(propertyID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.m {
		if e.prop.ID == propertyID {
			delete(c.m, k)
		}
	}
}

type usageEntry struct {
	month   time.Time
	limit   int64
	events  int64
	reached bool
	loaded  time.Time
}

// usageCache knows each workspace's events this month and its cap, read again every 30 seconds
// and moved by this process's own events in between.
type usageCache struct {
	mu sync.Mutex
	m  map[uuid.UUID]*usageEntry
}

func (s *Server) workspaceAnalyticsLimit(override *int64) int64 {
	if override != nil {
		return *override
	}
	return s.analytics.settings.MonthlyEvents
}

func (s *Server) loadUsage(ctx context.Context, ws uuid.UUID, now time.Time) (*usageEntry, error) {
	month := monthOf(now)
	c := s.analytics.usage
	c.mu.Lock()
	e, ok := c.m[ws]
	c.mu.Unlock()
	if ok && e.month.Equal(month) && now.Sub(e.loaded) < usageCacheTTL && !now.Before(e.loaded) {
		return e, nil
	}
	u, err := s.st.GetAnalyticsUsage(ctx, store.GetAnalyticsUsageParams{WorkspaceID: ws, Month: month})
	if err != nil {
		return nil, err
	}
	e = &usageEntry{month: month, limit: s.workspaceAnalyticsLimit(u.MonthlyLimit), events: u.Events, reached: u.LimitReachedAt != nil, loaded: now}
	c.mu.Lock()
	c.m[ws] = e
	c.mu.Unlock()
	return e, nil
}

// takeUsage counts n events against the workspace's monthly cap; false means the cap is reached.
func (s *Server) takeUsage(ctx context.Context, ws uuid.UUID, n int, now time.Time) (bool, error) {
	e, err := s.loadUsage(ctx, ws, now)
	if err != nil {
		return false, err
	}
	c := s.analytics.usage
	c.mu.Lock()
	defer c.mu.Unlock()
	if e.limit > 0 && e.events+int64(n) > e.limit {
		return false, nil
	}
	e.events += int64(n)
	return true, nil
}

func (s *Server) analyticsUsage(ctx context.Context, ws uuid.UUID) (oas.AnalyticsUsage, error) {
	month := monthOf(s.now())
	u, err := s.st.GetAnalyticsUsage(ctx, store.GetAnalyticsUsageParams{WorkspaceID: ws, Month: month})
	if err != nil {
		return oas.AnalyticsUsage{}, err
	}
	out := oas.AnalyticsUsage{Month: month.Format("2006-01"), Events: u.Events}
	if l := s.workspaceAnalyticsLimit(u.MonthlyLimit); l > 0 {
		out.Limit = &l
		if u.Events >= l {
			out.LimitReachedAt = u.LimitReachedAt
		}
	}
	return out, nil
}

// afterAnalyticsFlush tells owners and the operator, once a month, that a workspace reached its cap.
func (s *Server) afterAnalyticsFlush(ctx context.Context, res analytics.IngestResult) {
	now := s.now()
	for ws, total := range res.Usage {
		e, err := s.loadUsage(ctx, ws, now)
		if err != nil {
			s.log.ErrorContext(ctx, "analytics usage", slog.Any("error", err))
			continue
		}
		c := s.analytics.usage
		c.mu.Lock()
		if e.month.Equal(res.Month) && total > e.events {
			e.events = total
		}
		limit, reached := e.limit, e.reached
		c.mu.Unlock()
		if limit <= 0 || total < limit || reached {
			continue
		}
		if err := s.noteAnalyticsLimit(ctx, ws, res.Month, total, limit); err != nil {
			s.log.ErrorContext(ctx, "analytics limit notice", slog.Any("error", err))
		}
	}
}

func (s *Server) noteAnalyticsLimit(ctx context.Context, ws uuid.UUID, month time.Time, total, limit int64) error {
	var w store.Workspace
	var owners []store.ListOwnerAddressesRow
	first := false
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		n, err := q.MarkAnalyticsLimitReached(ctx, store.MarkAnalyticsLimitReachedParams{WorkspaceID: ws, Month: month, Now: new(s.now())})
		if err != nil || n == 0 {
			return err
		}
		first = true
		if w, err = q.GetWorkspace(ctx, ws); err != nil {
			return err
		}
		if owners, err = q.ListOwnerAddresses(ctx, ws); err != nil {
			return err
		}
		if len(s.operator) == 0 {
			return nil
		}
		return q.CreateOperatorNotice(ctx, store.CreateOperatorNoticeParams{
			ID: uuid.New(), Kind: noticeAnalyticsLimit, WorkspaceName: &w.Name, CreatedAt: s.now(),
			Detail: new(formatCount(total) + " analytics events in " + month.Format("January 2006") + "; the limit is " + formatCount(limit)),
		})
	})
	c := s.analytics.usage
	c.mu.Lock()
	if e, ok := c.m[ws]; ok {
		e.reached = true
	}
	c.mu.Unlock()
	if err != nil || !first {
		return err
	}
	for _, o := range owners {
		msg, err := mail.Render("analytics_limit", o.Locale, map[string]any{
			"Workspace": w.Name, "Limit": formatCount(limit), "Month": month.Format("2006-01"), "URL": s.auth.PublicURL + "/analytics",
		})
		if err != nil {
			return err
		}
		msg.To = o.Email
		s.sendMail(ctx, msg)
	}
	return nil
}

func formatCount(n int64) string {
	raw := []byte(strings.TrimPrefix(strconv.FormatInt(n, 10), "-"))
	var out []byte
	for i, c := range raw {
		if i > 0 && (len(raw)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if n < 0 {
		return "-" + string(out)
	}
	return string(out)
}

// analyticsCORS is clientCORS for the collection endpoint: the key in the path names the property,
// and a browser's Origin host must be one of its domains.
func (s *Server) analyticsCORS(w http.ResponseWriter, r *http.Request, next http.Handler) {
	a := s.analytics
	if a == nil {
		writeProblem(w, errNotFound)
		return
	}
	raw := r.Header.Get("Origin")
	if raw != "" {
		w.Header().Add("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Origin", raw)
		w.Header().Set("Access-Control-Expose-Headers", "Retry-After")
	}
	key := strings.TrimPrefix(r.URL.Path, analyticsPrefix)
	if key == "" || strings.Contains(key, "/") || len(key) > 200 {
		writeProblem(w, errNotFound)
		return
	}
	cp, err := a.props.get(r.Context(), s.st, key, s.now())
	if err != nil {
		s.log.ErrorContext(r.Context(), "analytics property", slog.Any("error", err))
		writeProblem(w, errInternal)
		return
	}
	if cp == nil {
		writeProblem(w, errNotFound)
		return
	}
	host := ""
	if origin, ok := normalizeOrigin(raw); ok {
		if u, err := url.Parse(origin); err == nil {
			host = matchHost(u.Hostname())
		}
	}
	if host == "" || !slices.Contains(cp.prop.Platforms, string(oas.AnalyticsPlatformWeb)) || !analytics.DomainAllowed(cp.prop.Domains, host) {
		if r.Method == http.MethodPost {
			a.buffer.Drop(cp.prop.WorkspaceID, cp.prop.ID, analytics.LocalDay(s.now(), cp.loc), analytics.ReasonOrigin, 1)
		}
		writeProblem(w, errPropertyOrigin)
		return
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Max-Age", "600")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), analyticsPropertyKey, cp)))
}

func matchHost(h string) string {
	h = strings.ToLower(h)
	if strings.Contains(h, ":") {
		return "[" + h + "]"
	}
	return h
}

type collectEvent struct {
	N  string `json:"n"`
	U  string `json:"u"`
	R  string `json:"r"`
	W  int    `json:"w"`
	H  bool   `json:"h"`
	MS int64  `json:"ms"`
	SD int    `json:"sd"`
	C  *struct {
		A int `json:"a"`
	} `json:"c"`
}

func parseAnalyticsBatch(body []byte) ([]collectEvent, error) {
	var probe struct {
		E json.RawMessage `json:"e"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, errors.New("the body is not a JSON object")
	}
	if probe.E == nil {
		var e collectEvent
		if err := json.Unmarshal(body, &e); err != nil {
			return nil, errors.New("the event does not match AnalyticsEvent")
		}
		return []collectEvent{e}, nil
	}
	var evs []collectEvent
	if err := json.Unmarshal(probe.E, &evs); err != nil {
		return nil, errors.New("e must be a list of events")
	}
	if len(evs) < 1 || len(evs) > maxAnalyticsBatch {
		return nil, errors.New("e must hold 1 to 100 events")
	}
	return evs, nil
}

// serveAnalyticsCollect takes events for the property analyticsCORS found. The client's address is
// shortened right after it is checked against the property's excluded addresses; only the short
// form reaches rate limits and hashing, and nothing of it is stored or logged.
func (s *Server) serveAnalyticsCollect(w http.ResponseWriter, r *http.Request) {
	a := s.analytics
	cp, _ := r.Context().Value(analyticsPropertyKey).(*cachedProperty)
	if a == nil || cp == nil {
		writeProblem(w, errNotFound)
		return
	}
	if r.Header.Get("Sec-GPC") == "1" || r.Header.Get("DNT") == "1" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	ctx, now, prop := r.Context(), s.now(), cp.prop
	day := analytics.LocalDay(now, cp.loc)
	drop := func(reason string, n int) { a.buffer.Drop(prop.WorkspaceID, prop.ID, day, reason, int64(n)) }
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxAnalyticsBody))
	if err != nil {
		drop(analytics.ReasonInvalid, 1)
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeProblem(w, errAnalyticsBodyTooLarge)
			return
		}
		writeProblem(w, errValidation("the body could not be read"))
		return
	}
	evs, err := parseAnalyticsBatch(body)
	if err != nil {
		drop(analytics.ReasonInvalid, 1)
		writeProblem(w, errValidation(err.Error()))
		return
	}
	pages := make([]analytics.Page, len(evs))
	for i, e := range evs {
		if e.N != "pageview" && e.N != "engagement" {
			drop(analytics.ReasonInvalid, len(evs))
			writeProblem(w, errValidation("n must be pageview or engagement"))
			return
		}
		pg, ok := analytics.ParsePage(e.U, e.H)
		if !ok {
			drop(analytics.ReasonInvalid, len(evs))
			writeProblem(w, errValidation("u must be an http(s) URL of at most 2,000 characters"))
			return
		}
		if !analytics.DomainAllowed(prop.Domains, matchHost(pg.Host)) {
			drop(analytics.ReasonOrigin, len(evs))
			writeProblem(w, errPageNotAllowed)
			return
		}
		pages[i] = pg
	}
	var keep []int
	for i, e := range evs {
		if prop.Collection != string(oas.AnalyticsCollectionConsent) || (e.C != nil && e.C.A == 1) {
			keep = append(keep, i)
		}
	}
	if len(keep) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	addr, err := netip.ParseAddr(s.clientIP(r))
	if err != nil {
		drop(analytics.ReasonInvalid, len(keep))
		w.WriteHeader(http.StatusAccepted)
		return
	}
	addr = addr.Unmap()
	excluded := slices.ContainsFunc(prop.ExcludedIps, func(p netip.Prefix) bool { return p.Contains(addr) })
	short := analytics.ShortenAddr(addr)
	addr = netip.Addr{}
	if excluded {
		drop(analytics.ReasonExcluded, len(keep))
		w.WriteHeader(http.StatusAccepted)
		return
	}
	for i := range keep {
		if ok, wait := a.limits.allowWait("a:"+prop.ID.String()+":"+short.String(), limitAnalyticsPerAddress, now); !ok {
			drop(analytics.ReasonRate, len(keep)-i)
			writeProblem(w, rateLimited(wait))
			return
		}
	}
	for i := range keep {
		if !a.limits.allow("p:"+prop.ID.String(), limit{a.settings.PropertyEventsPerMinute, time.Minute}, now) {
			drop(analytics.ReasonRate, len(keep)-i)
			w.WriteHeader(http.StatusAccepted)
			return
		}
	}
	ok, err := s.takeUsage(ctx, prop.WorkspaceID, len(keep), now)
	if err != nil {
		s.log.ErrorContext(ctx, "analytics usage", slog.Any("error", err))
		writeProblem(w, errInternal)
		return
	}
	if !ok {
		drop(analytics.ReasonOverLimit, len(keep))
		w.WriteHeader(http.StatusAccepted)
		return
	}
	ua := r.UserAgent() + "\x00" + r.Header.Get("Sec-CH-UA-Platform") + "\x00" + r.Header.Get("Sec-CH-UA-Mobile")
	children := prop.Audience == string(oas.AnalyticsAudienceChildren)
	language := ""
	if !children {
		language = analytics.Language(r.Header.Get("Accept-Language"))
	}
	var own string
	if u, err := url.Parse(s.auth.PublicURL); err == nil {
		own = u.Hostname()
	}
	out := make([]analytics.Event, 0, len(keep))
	scrubbed := 0
	for _, i := range keep {
		e, pg := evs[i], pages[i]
		if slices.ContainsFunc(prop.ExcludedPaths, func(p string) bool { return analytics.MatchPath(p, pg.Path) }) {
			drop(analytics.ReasonExcluded, 1)
			continue
		}
		path := analytics.ApplyRules(cp.rules, pg.Path)
		engagement := e.N == "engagement"
		v, ok, err := a.resolver.Resolve(ctx, prop.WorkspaceID, prop.ID, cp.loc, now, short, ua, engagement)
		if err != nil {
			s.log.ErrorContext(ctx, "analytics visit", slog.Any("error", err))
			writeProblem(w, errInternal)
			return
		}
		if !ok {
			continue
		}
		if pg.Scrubbed {
			scrubbed++
		}
		ev := analytics.Event{
			WorkspaceID: prop.WorkspaceID, PropertyID: prop.ID, Day: v.Day, TS: now, Visit: v.Visit, Visitor: v.Visitor,
			Kind: analytics.KindPageview, Name: e.N, Hostname: pg.Host, Path: path, Interactive: true,
		}
		if engagement {
			ev.Kind, ev.Interactive = analytics.KindEngagement, false
			ev.EngagedMS = new(int32(min(max(e.MS, 0), maxEngagedMS)))
			ev.Scroll = new(int16(min(max(e.SD, 0), 100)))
		} else {
			refHost, refPath := analytics.ParseReferrer(e.R, pg.Host, own)
			source := pg.Acquisition.Source
			if source == "" {
				source = refHost
			}
			info := &analytics.VisitInfo{
				Hostname: pg.Host, ReferrerHost: refHost, ReferrerPath: refPath, Acquisition: pg.Acquisition, Source: source,
				Channel:  analytics.ClassifyChannel(refHost, pg.Acquisition, refHost != "" && analytics.DomainAllowed(prop.Domains, matchHost(refHost))),
				Language: language, Platform: analytics.PlatformWeb,
			}
			if !children {
				info.Screen = analytics.ScreenClass(e.W)
			}
			ev.Info = info
		}
		out = append(out, ev)
	}
	if len(out) > 0 && !a.buffer.Add(out) {
		drop(analytics.ReasonBuffer, len(out))
		writeProblem(w, errAnalyticsBusy)
		return
	}
	if scrubbed > 0 {
		a.buffer.Count(prop.WorkspaceID, prop.ID, day, analytics.ReasonScrubbed, int64(scrubbed))
	}
	w.WriteHeader(http.StatusAccepted)
}

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
	"github.com/productdevbook/yuva/api/internal/webhook"
)

const (
	maxChecksPerHost        = 10
	maxCheckHeaders         = 5
	defaultCheckInterval    = 60
	defaultCheckTimeout     = 10
	defaultFailuresDown     = 2
	defaultCertWarningDays  = 14
	defaultCheckRedirects   = 5
	checkResolveTimeout     = 5 * time.Second
	defaultCheckResultLimit = 100
)

var (
	errCheckGone       = problem(http.StatusNotFound, "not_found", "no such uptime check")
	errCheckLimit      = problem(http.StatusForbidden, "check_limit", "this workspace has as many uptime checks as the server allows (YUVA_MONITOR_CHECKS)")
	errHostLimit       = problem(http.StatusForbidden, "host_limit", "a workspace has at most 10 uptime checks on one host")
	errCheckRefused    = problem(http.StatusUnprocessableEntity, "refused_address", "the URL names an address the server does not call: loopback, private, link-local, CGNAT, multicast, reserved or cloud metadata")
	forbiddenCheckHead = []string{"host", "cookie", "content-length", "transfer-encoding", "connection", "upgrade", "te", "trailer", "keep-alive", "expect"}
)

type sealedHeader struct {
	Name  string `json:"name"`
	Value []byte `json:"value"`
}

func checkSecretContext(ws, checkID uuid.UUID) []byte {
	return append(append([]byte("monitor-check-header:"), ws[:]...), checkID[:]...)
}

func headersOf(raw []byte) []sealedHeader {
	var out []sealedHeader
	_ = json.Unmarshal(raw, &out)
	return out
}

// checkFields are a check's settings while they are checked.
type checkFields struct {
	name, url, host, method string
	headers                 []oas.MonitorCheckHeader
	sealed                  []sealedHeader
	keepHeaders             bool
	expected                []int32
	keyword                 *string
	keywordAbsent           bool
	timeout, interval       int32
	failures, certDays      int32
	redirects               int32
	service                 *string
	propertyID              *uuid.UUID
	paused                  bool
}

func checkFieldsOf(c store.MonitorCheck) checkFields {
	return checkFields{name: c.Name, url: c.Url, host: c.Host, method: c.Method, sealed: headersOf(c.Headers), keepHeaders: true,
		expected: c.ExpectedStatuses, keyword: c.Keyword, keywordAbsent: c.KeywordAbsent, timeout: c.TimeoutSeconds, interval: c.IntervalSeconds,
		failures: c.FailuresBeforeDown, certDays: c.CertWarningDays, redirects: c.MaxRedirects, service: c.Service, propertyID: c.PropertyID, paused: c.Paused}
}

func (f *checkFields) apply(b oas.MonitorCheckFields) {
	if b.Name != nil {
		f.name = *b.Name
	}
	if b.Url != nil {
		f.url = *b.Url
	}
	if b.Method != nil {
		f.method = string(*b.Method)
	}
	if b.Headers != nil {
		f.headers, f.keepHeaders = *b.Headers, false
	}
	if b.ExpectedStatuses != nil {
		f.expected = []int32{}
		for _, v := range *b.ExpectedStatuses {
			f.expected = append(f.expected, int32(v))
		}
	}
	if b.Keyword.IsSpecified() {
		f.keyword = nil
		if !b.Keyword.IsNull() {
			f.keyword = new(b.Keyword.MustGet())
		}
	}
	if b.KeywordAbsent != nil {
		f.keywordAbsent = *b.KeywordAbsent
	}
	if b.TimeoutSeconds != nil {
		f.timeout = int32(*b.TimeoutSeconds)
	}
	if b.IntervalSeconds != nil {
		f.interval = int32(*b.IntervalSeconds)
	}
	if b.FailuresBeforeDown != nil {
		f.failures = int32(*b.FailuresBeforeDown)
	}
	if b.CertWarningDays != nil {
		f.certDays = int32(*b.CertWarningDays)
	}
	if b.MaxRedirects != nil {
		f.redirects = int32(*b.MaxRedirects)
	}
	if b.Service.IsSpecified() {
		f.service = nil
		if !b.Service.IsNull() {
			f.service = new(b.Service.MustGet())
		}
	}
	if b.PropertyId.IsSpecified() {
		f.propertyID = nil
		if !b.PropertyId.IsNull() {
			f.propertyID = new(b.PropertyId.MustGet())
		}
	}
	if b.Paused != nil {
		f.paused = *b.Paused
	}
}

func validHeaderValue(v string) bool {
	return !strings.ContainsAny(v, "\r\n\x00")
}

// checkedURL is a URL the uptime runner may call, with its host in lower case: http or https,
// without credentials or fragment, its host refused when it names or resolves to an address the
// server does not call; ctx bounds the lookup.
func (s *Server) checkedURL(ctx context.Context, m *monitorState, raw, field string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) > 2000 {
		return "", "", errValidation(field + " must be at most 2000 characters")
	}
	u, err := m.prober.CheckTarget(raw)
	if errors.Is(err, webhook.ErrRefusedAddress) {
		return "", "", errCheckRefused
	}
	if err != nil {
		return "", "", errValidation(field + " must be an http or https URL with a host and no credentials or fragment")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if _, err := netip.ParseAddr(host); err != nil {
		lookup, cancel := context.WithTimeout(ctx, checkResolveTimeout)
		ips, err := m.resolver.LookupNetIP(lookup, "ip", host)
		cancel()
		if err == nil && slices.ContainsFunc(ips, m.prober.Refused) {
			return "", "", errCheckRefused
		}
	}
	return u.String(), host, nil
}

// validate checks the fields and the URL's address; ctx bounds the lookup of its host.
func (s *Server) validateCheck(ctx context.Context, m *monitorState, f *checkFields) error {
	var err error
	if f.name, err = trimmed(f.name, 1, 100, "name"); err != nil {
		return err
	}
	if f.url, f.host, err = s.checkedURL(ctx, m, f.url, "url"); err != nil {
		return err
	}
	if f.method != string(oas.MonitorCheckGet) && f.method != string(oas.MonitorCheckHead) {
		return errValidation("method must be GET or HEAD")
	}
	if !f.keepHeaders {
		if len(f.headers) > maxCheckHeaders {
			return errValidation("a check sends at most 5 headers")
		}
		seen := []string{}
		for _, h := range f.headers {
			name := strings.TrimSpace(h.Name)
			lower := strings.ToLower(name)
			if name == "" || len(name) > 100 || strings.ContainsFunc(name, func(r rune) bool { return r <= ' ' || r > '~' || strings.ContainsRune("()<>@,;:\\\"/[]?={}", r) }) {
				return errValidation("header names are tokens of at most 100 characters")
			}
			if slices.Contains(forbiddenCheckHead, lower) || strings.HasPrefix(lower, "proxy-") {
				return errValidation("the header " + name + " cannot be set")
			}
			if slices.Contains(seen, lower) {
				return errValidation("the header " + name + " is listed twice")
			}
			if len(h.Value) > 2000 || !validHeaderValue(h.Value) {
				return errValidation("header values have at most 2000 characters and no line breaks")
			}
			seen = append(seen, lower)
		}
	}
	if len(f.expected) > 20 {
		return errValidation("expected_statuses lists at most 20 statuses")
	}
	for _, v := range f.expected {
		if v < 200 || v > 399 {
			return errValidation("expected_statuses are statuses from 200 to 399")
		}
	}
	slices.Sort(f.expected)
	f.expected = slices.Compact(f.expected)
	if f.keyword != nil {
		k := *f.keyword
		if k == "" || len([]rune(k)) > 200 {
			return errValidation("keyword has 1 to 200 characters")
		}
		if f.method != string(oas.MonitorCheckGet) {
			return errValidation("a keyword needs method GET, since HEAD reads no body")
		}
	} else if f.keywordAbsent {
		return errValidation("keyword_absent needs a keyword")
	}
	if f.timeout < 1 || f.timeout > 30 {
		return errValidation("timeout_seconds must be 1 to 30")
	}
	if f.interval < 30 || f.interval > 86400 {
		return errValidation("interval_seconds must be 30 to 86400")
	}
	if least := int32(m.settings.MinInterval / time.Second); f.interval < least {
		return problem(http.StatusUnprocessableEntity, "interval_too_short", "this server runs checks at most every "+formatCount(int64(least))+" seconds (YUVA_MONITOR_MIN_INTERVAL)")
	}
	if f.failures < 1 || f.failures > 10 {
		return errValidation("failures_before_down must be 1 to 10")
	}
	if f.certDays < 1 || f.certDays > 90 {
		return errValidation("cert_warning_days must be 1 to 90")
	}
	if f.redirects < 0 || f.redirects > 5 {
		return errValidation("max_redirects must be 0 to 5")
	}
	if f.service != nil {
		v, err := trimmed(*f.service, 1, 100, "service")
		if err != nil {
			return err
		}
		f.service = &v
	}
	return nil
}

// sealHeaders encrypts the header values under the master key, bound to the check.
func (s *Server) sealHeaders(f *checkFields, ws, id uuid.UUID) []byte {
	if f.keepHeaders {
		return mustJSON(nonNil(f.sealed))
	}
	out := []sealedHeader{}
	for _, h := range f.headers {
		out = append(out, sealedHeader{Name: strings.TrimSpace(h.Name), Value: s.secrets.Seal([]byte(h.Value), checkSecretContext(ws, id))})
	}
	return mustJSON(out)
}

func checkResultBody(r store.MonitorCheckResult) oas.MonitorCheckResult {
	ms := func(us int64) float64 { return float64(us) / 1000 }
	out := oas.MonitorCheckResult{At: r.At, Ok: r.Ok, Status: int(r.Status), Retried: r.Retried, Redirects: int(r.Redirects), Detail: r.Detail,
		State: oas.MonitorResultPending, Timings: oas.MonitorCheckTimings{Dns: ms(r.DnsUs), Connect: ms(r.ConnectUs), Tls: ms(r.TlsUs), Ttfb: ms(r.TtfbUs),
			Total: ms(r.TotalUs)}}
	if r.State != nil {
		out.State = oas.MonitorCheckResultState(*r.State)
	}
	if r.Error != nil {
		out.Error = new(oas.MonitorCheckError(*r.Error))
	}
	if r.CertDaysLeft != nil {
		out.CertDaysLeft = new(int(*r.CertDaysLeft))
	}
	return out
}

func checkBody(c store.MonitorCheck, last *store.MonitorCheckResult) oas.MonitorCheck {
	out := oas.MonitorCheck{
		Id: c.ID, Name: c.Name, Url: c.Url, Method: oas.MonitorCheckMethod(c.Method), HeaderNames: []string{}, ExpectedStatuses: []int{},
		Keyword: c.Keyword, KeywordAbsent: c.KeywordAbsent, TimeoutSeconds: int(c.TimeoutSeconds), IntervalSeconds: int(c.IntervalSeconds),
		FailuresBeforeDown: int(c.FailuresBeforeDown), CertWarningDays: int(c.CertWarningDays), MaxRedirects: int(c.MaxRedirects), Service: c.Service,
		PropertyId: c.PropertyID, Paused: c.Paused, State: oas.MonitorCheckState(c.State), StateSince: c.StateSince, CertExpiresAt: c.CertExpiresAt,
		CreatedBy: c.CreatedBy, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
	for _, h := range headersOf(c.Headers) {
		out.HeaderNames = append(out.HeaderNames, h.Name)
	}
	for _, v := range c.ExpectedStatuses {
		out.ExpectedStatuses = append(out.ExpectedStatuses, int(v))
	}
	if !c.Paused {
		out.NextRunAt = &c.NextRunAt
	}
	if last != nil {
		out.LastResult = new(checkResultBody(*last))
	}
	return out
}

func (s *Server) checkBodies(ctx context.Context, ws uuid.UUID, checks []store.MonitorCheck) ([]oas.MonitorCheck, error) {
	ids := make([]uuid.UUID, len(checks))
	for i, c := range checks {
		ids[i] = c.ID
	}
	last := map[uuid.UUID]store.MonitorCheckResult{}
	if len(ids) > 0 {
		rows, err := s.st.ListLastMonitorCheckResults(ctx, store.ListLastMonitorCheckResultsParams{WorkspaceID: ws, CheckIds: ids,
			Since: s.now().Add(-3 * 24 * time.Hour)})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			last[r.CheckID] = r
		}
	}
	out := make([]oas.MonitorCheck, len(checks))
	for i, c := range checks {
		var l *store.MonitorCheckResult
		if r, ok := last[c.ID]; ok {
			l = &r
		}
		out[i] = checkBody(c, l)
	}
	return out, nil
}

func (s *Server) checkProperty(ctx context.Context, q *store.Queries, p principal, id *uuid.UUID) error {
	if id == nil {
		return nil
	}
	_, err := visibleProperty(ctx, q, p, *id, false)
	return err
}

func (s *Server) ListMonitorChecks(ctx context.Context, req oas.ListMonitorChecksRequestObject) (oas.ListMonitorChecksResponseObject, error) {
	m, err := s.monitorOn()
	if err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	rows, err := s.st.ListMonitorChecks(ctx, store.ListMonitorChecksParams{WorkspaceID: p.workspaceID, PropertyID: req.Params.PropertyId})
	if err != nil {
		return nil, err
	}
	items, err := s.checkBodies(ctx, p.workspaceID, rows)
	if err != nil {
		return nil, err
	}
	return oas.ListMonitorChecks200JSONResponse{Items: items, Limit: m.settings.Checks, HostLimit: maxChecksPerHost,
		MinIntervalSeconds: new(int(m.settings.MinInterval / time.Second))}, nil
}

func (s *Server) getCheck(ctx context.Context, q *store.Queries, ws, id uuid.UUID, lock bool) (store.MonitorCheck, error) {
	get := q.GetMonitorCheck
	if lock {
		get = func(ctx context.Context, arg store.GetMonitorCheckParams) (store.MonitorCheck, error) {
			return q.LockMonitorCheck(ctx, store.LockMonitorCheckParams(arg))
		}
	}
	c, err := get(ctx, store.GetMonitorCheckParams{WorkspaceID: ws, ID: id})
	if store.IsNotFound(err) {
		return c, errCheckGone
	}
	return c, err
}

func (s *Server) GetMonitorCheck(ctx context.Context, req oas.GetMonitorCheckRequestObject) (oas.GetMonitorCheckResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	c, err := s.getCheck(ctx, s.st.Queries, p.workspaceID, req.CheckId, false)
	if err != nil {
		return nil, err
	}
	body, err := s.checkBodies(ctx, p.workspaceID, []store.MonitorCheck{c})
	if err != nil {
		return nil, err
	}
	return oas.GetMonitorCheck200JSONResponse(body[0]), nil
}

func (s *Server) CreateMonitorCheck(ctx context.Context, req oas.CreateMonitorCheckRequestObject) (oas.CreateMonitorCheckResponseObject, error) {
	m, err := s.monitorOn()
	if err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if err := requireAnalyticsManager(p); err != nil {
		return nil, err
	}
	b := req.Body
	f := checkFields{method: string(oas.MonitorCheckGet), headers: []oas.MonitorCheckHeader{}, expected: []int32{}, timeout: defaultCheckTimeout,
		interval: max(defaultCheckInterval, int32(m.settings.MinInterval/time.Second)), failures: defaultFailuresDown, certDays: defaultCertWarningDays,
		redirects: defaultCheckRedirects}
	f.apply(oas.MonitorCheckFields{Name: &b.Name, Url: &b.Url, Method: b.Method, Headers: b.Headers, ExpectedStatuses: b.ExpectedStatuses,
		Keyword: b.Keyword, KeywordAbsent: b.KeywordAbsent, TimeoutSeconds: b.TimeoutSeconds, IntervalSeconds: b.IntervalSeconds,
		FailuresBeforeDown: b.FailuresBeforeDown, CertWarningDays: b.CertWarningDays, MaxRedirects: b.MaxRedirects, Service: b.Service,
		PropertyId: b.PropertyId, Paused: b.Paused})
	if err := s.validateCheck(ctx, m, &f); err != nil {
		return nil, err
	}
	var out store.MonitorCheck
	err = s.st.InTx(ctx, func(q *store.Queries) error {
		if err := s.checkProperty(ctx, q, p, f.propertyID); err != nil {
			return err
		}
		if _, err := q.LockWorkspace(ctx, p.workspaceID); err != nil {
			return err
		}
		n, err := q.CountMonitorChecks(ctx, p.workspaceID)
		if err != nil {
			return err
		}
		if n >= int64(m.settings.Checks) {
			return errCheckLimit
		}
		id := newID()
		if on, err := q.CountMonitorChecksOnHost(ctx, store.CountMonitorChecksOnHostParams{WorkspaceID: p.workspaceID, Host: f.host, ID: id}); err != nil {
			return err
		} else if on >= maxChecksPerHost {
			return errHostLimit
		}
		now := s.now()
		out, err = q.CreateMonitorCheck(ctx, store.CreateMonitorCheckParams{
			ID: id, WorkspaceID: p.workspaceID, PropertyID: f.propertyID, Name: f.name, Url: f.url, Host: f.host, Method: f.method,
			Headers: s.sealHeaders(&f, p.workspaceID, id), ExpectedStatuses: f.expected, Keyword: f.keyword, KeywordAbsent: f.keywordAbsent,
			TimeoutSeconds: f.timeout, IntervalSeconds: f.interval, FailuresBeforeDown: f.failures, CertWarningDays: f.certDays, MaxRedirects: f.redirects,
			Service: f.service, Paused: f.paused, CreatedBy: p.memberRef(), Now: now,
		})
		if err != nil {
			return err
		}
		member := p.memberRef()
		if member == nil {
			return nil
		}
		_, err = q.CreateMonitorAlertRule(ctx, store.CreateMonitorAlertRuleParams{
			ID: newID(), WorkspaceID: p.workspaceID, Name: truncateRunes(out.Name+" is down", 100), Kind: string(oas.MonitorRuleCheckState), CheckID: &out.ID,
			WindowMinutes: defaultRuleWindow, MinRequests: defaultRuleMinRequests, Recipients: []uuid.UUID{*member}, Enabled: true, CreatedBy: member, Now: now,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return oas.CreateMonitorCheck201JSONResponse(checkBody(out, nil)), nil
}

func (s *Server) UpdateMonitorCheck(ctx context.Context, req oas.UpdateMonitorCheckRequestObject) (oas.UpdateMonitorCheckResponseObject, error) {
	m, err := s.monitorOn()
	if err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if err := requireAnalyticsManager(p); err != nil {
		return nil, err
	}
	cur, err := s.getCheck(ctx, s.st.Queries, p.workspaceID, req.CheckId, false)
	if err != nil {
		return nil, err
	}
	f := checkFieldsOf(cur)
	f.apply(oas.MonitorCheckFields(*req.Body))
	if err := s.validateCheck(ctx, m, &f); err != nil {
		return nil, err
	}
	var out store.MonitorCheck
	err = s.st.InTx(ctx, func(q *store.Queries) error {
		cur, err := s.getCheck(ctx, q, p.workspaceID, req.CheckId, true)
		if err != nil {
			return err
		}
		if err := s.checkProperty(ctx, q, p, f.propertyID); err != nil {
			return err
		}
		if f.host != cur.Host {
			if _, err := q.LockWorkspace(ctx, p.workspaceID); err != nil {
				return err
			}
			on, err := q.CountMonitorChecksOnHost(ctx, store.CountMonitorChecksOnHostParams{WorkspaceID: p.workspaceID, Host: f.host, ID: cur.ID})
			if err != nil {
				return err
			}
			if on >= maxChecksPerHost {
				return errHostLimit
			}
		}
		now := s.now()
		next := cur.NextRunAt
		if f.url != cur.Url || f.method != cur.Method || f.interval != cur.IntervalSeconds || (cur.Paused && !f.paused) {
			next = now
		}
		out, err = q.UpdateMonitorCheck(ctx, store.UpdateMonitorCheckParams{
			WorkspaceID: p.workspaceID, ID: cur.ID, PropertyID: f.propertyID, Name: f.name, Url: f.url, Host: f.host, Method: f.method,
			Headers: s.sealHeaders(&f, p.workspaceID, cur.ID), ExpectedStatuses: f.expected, Keyword: f.keyword, KeywordAbsent: f.keywordAbsent,
			TimeoutSeconds: f.timeout, IntervalSeconds: f.interval, FailuresBeforeDown: f.failures, CertWarningDays: f.certDays, MaxRedirects: f.redirects,
			Service: f.service, Paused: f.paused, NextRunAt: next, Now: now,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	body, err := s.checkBodies(ctx, p.workspaceID, []store.MonitorCheck{out})
	if err != nil {
		return nil, err
	}
	return oas.UpdateMonitorCheck200JSONResponse(body[0]), nil
}

func (s *Server) DeleteMonitorCheck(ctx context.Context, req oas.DeleteMonitorCheckRequestObject) (oas.DeleteMonitorCheckResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if err := requireAnalyticsManager(p); err != nil {
		return nil, err
	}
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		c, err := s.getCheck(ctx, q, p.workspaceID, req.CheckId, true)
		if err != nil {
			return err
		}
		if _, err := q.DeleteMonitorCheckResults(ctx, store.DeleteMonitorCheckResultsParams{WorkspaceID: p.workspaceID, CheckID: c.ID}); err != nil {
			return err
		}
		_, err = q.DeleteMonitorCheck(ctx, store.DeleteMonitorCheckParams{WorkspaceID: p.workspaceID, ID: c.ID})
		return err
	})
	if err != nil {
		return nil, err
	}
	return oas.DeleteMonitorCheck204Response{}, nil
}

func (s *Server) ListMonitorCheckResults(ctx context.Context, req oas.ListMonitorCheckResultsRequestObject) (oas.ListMonitorCheckResultsResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	limit := defaultCheckResultLimit
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	if limit < 1 || limit > 500 {
		return nil, errValidation("limit must be 1 to 500")
	}
	c, err := s.getCheck(ctx, s.st.Queries, p.workspaceID, req.CheckId, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.st.ListMonitorCheckResults(ctx, store.ListMonitorCheckResultsParams{WorkspaceID: p.workspaceID, CheckID: c.ID, Before: req.Params.Before,
		Lim: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := oas.ListMonitorCheckResults200JSONResponse{Items: make([]oas.MonitorCheckResult, len(rows))}
	for i, r := range rows {
		out.Items[i] = checkResultBody(r)
	}
	return out, nil
}

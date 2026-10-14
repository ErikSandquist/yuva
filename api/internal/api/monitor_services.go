package api

import (
	"context"
	"net/http"
	"slices"
	"time"
	"uuid"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/productdevbook/yuva/api/internal/monitor"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	defaultHistoryDays  = 90
	defaultHealthHours  = 24
	defaultChangesLimit = 100
	maxHealthHours      = 2160
	maxHistoryDays      = 400
	secondsPerDay       = 86400
)

func healthStatusPtr(v *string) *oas.MonitorHealthStatus {
	if v == nil {
		return nil
	}
	return new(oas.MonitorHealthStatus(*v))
}

func dependencyBody(d store.MonitorDependency, mt maintenance) oas.MonitorDependency {
	return oas.MonitorDependency{Id: d.ID, Name: d.Name, Kind: oas.MonitorDependencyKind(d.Kind), Critical: d.Critical, State: oas.MonitorServiceState(d.State),
		StateSince: utcPtr(d.StateSince), Status: healthStatusPtr(d.Status), LatencyMs: d.LatencyMs, Message: d.Message, LastReportAt: utcPtr(d.ReportedAt),
		Maintenance: mt.dependency(d)}
}

func serviceBody(svc store.MonitorService) oas.MonitorService {
	out := oas.MonitorService{Id: svc.ID, Name: svc.Name, Environment: svc.Environment, Origin: oas.MonitorServiceOrigin(svc.Origin), PropertyId: svc.PropertyID,
		Mode: oas.MonitorServiceMode(svc.Mode), HealthUrl: svc.HealthUrl, IntervalSeconds: int(svc.IntervalSeconds), TimeoutSeconds: int(svc.TimeoutSeconds),
		SilenceAfterSeconds: int(silenceAfter(svc) / time.Second), ErrorRateThreshold: svc.ErrorRateThreshold, P95ThresholdMs: svc.P95ThresholdMs,
		Version: svc.Version, State: oas.MonitorServiceState(svc.State), StateSince: utcPtr(svc.StateSince), Reason: svc.Reason,
		LastReportAt: utcPtr(svc.LastReportAt), LastStatus: healthStatusPtr(svc.LastStatus), CheckIds: []uuid.UUID{}, Dependencies: []oas.MonitorDependency{},
		CreatedBy: svc.CreatedBy, CreatedAt: svc.CreatedAt.UTC(), UpdatedAt: svc.UpdatedAt.UTC()}
	if svc.SilenceSeconds != nil {
		out.SilenceSeconds = new(int(*svc.SilenceSeconds))
	}
	if svc.Mode == modePull {
		out.NextPullAt = utcPtr(svc.NextPullAt)
	}
	return out
}

// serviceBodies are services with their dependencies, linked checks, instances (counted, or
// listed with withInstances), open incident and maintenance.
func (s *Server) serviceBodies(ctx context.Context, q *store.Queries, ws uuid.UUID, svcs []store.MonitorService, withInstances bool) ([]oas.MonitorService, error) {
	out := make([]oas.MonitorService, len(svcs))
	if len(svcs) == 0 {
		return out, nil
	}
	ids, names := make([]uuid.UUID, len(svcs)), []string{}
	for i, svc := range svcs {
		ids[i] = svc.ID
		if !slices.Contains(names, svc.Name) {
			names = append(names, svc.Name)
		}
	}
	mt, err := s.activeMaintenance(ctx, q, ws, s.now())
	if err != nil {
		return nil, err
	}
	deps, err := q.ListMonitorDependencies(ctx, store.ListMonitorDependenciesParams{WorkspaceID: ws, ServiceIds: ids})
	if err != nil {
		return nil, err
	}
	checks, err := q.ListMonitorChecksForServices(ctx, store.ListMonitorChecksForServicesParams{WorkspaceID: ws, Names: names})
	if err != nil {
		return nil, err
	}
	counts, err := q.CountMonitorInstances(ctx, ws)
	if err != nil {
		return nil, err
	}
	open, err := q.ListOpenMonitorIncidents(ctx, ws)
	if err != nil {
		return nil, err
	}
	for i, svc := range svcs {
		b := serviceBody(svc)
		b.Maintenance = mt.service(svc.ID)
		for _, d := range deps {
			if d.ServiceID == svc.ID {
				b.Dependencies = append(b.Dependencies, dependencyBody(d, mt))
			}
		}
		for _, c := range checks {
			if c.Service != nil && *c.Service == svc.Name {
				b.CheckIds = append(b.CheckIds, c.ID)
			}
		}
		for _, c := range counts {
			if c.ServiceID == svc.ID {
				b.InstanceCount = int(c.N)
			}
		}
		for _, inc := range open {
			if inc.Kind != kindIncident || !slices.Contains(inc.ServiceIds, svc.ID) {
				continue
			}
			if b.OpenIncidentId == nil || (inc.AutoServiceID != nil && *inc.AutoServiceID == svc.ID) {
				b.OpenIncidentId = new(inc.ID)
			}
		}
		if withInstances {
			rows, err := q.ListMonitorInstances(ctx, store.ListMonitorInstancesParams{WorkspaceID: ws, ServiceID: svc.ID})
			if err != nil {
				return nil, err
			}
			list := make([]oas.MonitorInstance, len(rows))
			for j, r := range rows {
				list[j] = oas.MonitorInstance{Instance: r.Instance, Version: r.Version, Status: healthStatusPtr(r.Status), FirstSeenAt: r.FirstSeenAt.UTC(),
					LastSeenAt: r.LastSeenAt.UTC()}
			}
			b.Instances = &list
		}
		out[i] = b
	}
	return out, nil
}

func (s *Server) getService(ctx context.Context, q *store.Queries, ws, id uuid.UUID, lock bool) (store.MonitorService, error) {
	var svc store.MonitorService
	var err error
	if lock {
		svc, err = q.LockMonitorService(ctx, store.LockMonitorServiceParams{WorkspaceID: ws, ID: id})
	} else {
		svc, err = q.GetMonitorService(ctx, store.GetMonitorServiceParams{WorkspaceID: ws, ID: id})
	}
	if store.IsNotFound(err) {
		return svc, errServiceGone
	}
	return svc, err
}

// serviceFields are a service's settings while they are checked.
type serviceFields struct {
	propertyID        *uuid.UUID
	mode              string
	healthURL         *string
	interval, timeout int32
	silence           *int32
	errorRate         float64
	p95               *float64
}

func serviceFieldsOf(svc store.MonitorService) serviceFields {
	return serviceFields{propertyID: svc.PropertyID, mode: svc.Mode, healthURL: svc.HealthUrl, interval: svc.IntervalSeconds, timeout: svc.TimeoutSeconds,
		silence: svc.SilenceSeconds, errorRate: svc.ErrorRateThreshold, p95: svc.P95ThresholdMs}
}

func (f *serviceFields) apply(b oas.MonitorServiceFields) {
	if b.PropertyId.IsSpecified() {
		f.propertyID = nil
		if !b.PropertyId.IsNull() {
			f.propertyID = new(b.PropertyId.MustGet())
		}
	}
	if b.Mode != nil {
		f.mode = string(*b.Mode)
	}
	if b.HealthUrl.IsSpecified() {
		f.healthURL = nil
		if !b.HealthUrl.IsNull() {
			f.healthURL = new(b.HealthUrl.MustGet())
		}
	}
	if b.IntervalSeconds != nil {
		f.interval = int32(*b.IntervalSeconds)
	}
	if b.TimeoutSeconds != nil {
		f.timeout = int32(*b.TimeoutSeconds)
	}
	if b.SilenceSeconds.IsSpecified() {
		f.silence = nil
		if !b.SilenceSeconds.IsNull() {
			f.silence = new(int32(b.SilenceSeconds.MustGet()))
		}
	}
	if b.ErrorRateThreshold != nil {
		f.errorRate = *b.ErrorRateThreshold
	}
	if b.P95ThresholdMs.IsSpecified() {
		f.p95 = nil
		if !b.P95ThresholdMs.IsNull() {
			f.p95 = new(b.P95ThresholdMs.MustGet())
		}
	}
}

func (s *Server) validateService(ctx context.Context, q *store.Queries, m *monitorState, p principal, f *serviceFields) error {
	if !oas.MonitorServiceMode(f.mode).Valid() {
		return errValidation("mode must be push or pull")
	}
	if f.interval < 30 || f.interval > 600 {
		return errValidation("interval_seconds must be 30 to 600")
	}
	if f.timeout < 1 || f.timeout > 30 {
		return errValidation("timeout_seconds must be 1 to 30")
	}
	if f.silence != nil && (*f.silence < 60 || *f.silence > 86400) {
		return errValidation("silence_seconds must be 60 to 86400")
	}
	if f.errorRate <= 0 || f.errorRate > 100 {
		return errValidation("error_rate_threshold is a percent above 0, at most 100")
	}
	if f.p95 != nil && (*f.p95 <= 0 || *f.p95 > 3_600_000) {
		return errValidation("p95_threshold_ms is a number of milliseconds above 0, at most 3,600,000")
	}
	if f.mode == modePull {
		if f.healthURL == nil {
			return errValidation("a pull service needs health_url")
		}
		if least := int32(m.settings.MinInterval / time.Second); f.interval < least {
			return problem(http.StatusUnprocessableEntity, "interval_too_short", "this server pulls at most every "+formatCount(int64(least))+" seconds (YUVA_MONITOR_MIN_INTERVAL)")
		}
	}
	if f.healthURL != nil {
		u, _, err := s.checkedURL(ctx, m, *f.healthURL, "health_url")
		if err != nil {
			return err
		}
		f.healthURL = &u
	}
	if f.propertyID != nil {
		if _, err := visibleProperty(ctx, q, p, *f.propertyID, false); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) ListMonitorServices(ctx context.Context, req oas.ListMonitorServicesRequestObject) (oas.ListMonitorServicesResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	rows, err := s.st.ListMonitorServices(ctx, store.ListMonitorServicesParams{WorkspaceID: p.workspaceID, PropertyID: req.Params.PropertyId})
	if err != nil {
		return nil, err
	}
	items, err := s.serviceBodies(ctx, s.st.Queries, p.workspaceID, rows, false)
	if err != nil {
		return nil, err
	}
	return oas.ListMonitorServices200JSONResponse{Items: items, Limit: maxMonitorServices}, nil
}

func (s *Server) GetMonitorService(ctx context.Context, req oas.GetMonitorServiceRequestObject) (oas.GetMonitorServiceResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	svc, err := s.getService(ctx, s.st.Queries, p.workspaceID, req.ServiceId, false)
	if err != nil {
		return nil, err
	}
	body, err := s.serviceBodies(ctx, s.st.Queries, p.workspaceID, []store.MonitorService{svc}, true)
	if err != nil {
		return nil, err
	}
	return oas.GetMonitorService200JSONResponse(body[0]), nil
}

func (s *Server) CreateMonitorService(ctx context.Context, req oas.CreateMonitorServiceRequestObject) (oas.CreateMonitorServiceResponseObject, error) {
	m, err := s.monitorOn()
	if err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if err := requireAnalyticsManager(p); err != nil {
		return nil, err
	}
	b := req.Body
	name, err := trimmed(b.Name, 1, 100, "name")
	if err != nil {
		return nil, err
	}
	env := monitor.DefaultEnvironment
	if b.Environment != nil {
		if env, err = trimmed(*b.Environment, 1, 40, "environment"); err != nil {
			return nil, err
		}
	}
	f := serviceFields{mode: modePush, interval: defaultServiceInterval, timeout: defaultServiceTimeout, errorRate: defaultServiceErrorRate}
	f.apply(oas.MonitorServiceFields{PropertyId: b.PropertyId, Mode: b.Mode, HealthUrl: b.HealthUrl, IntervalSeconds: b.IntervalSeconds,
		TimeoutSeconds: b.TimeoutSeconds, SilenceSeconds: b.SilenceSeconds, ErrorRateThreshold: b.ErrorRateThreshold, P95ThresholdMs: b.P95ThresholdMs})
	if err := s.validateService(ctx, s.st.Queries, m, p, &f); err != nil {
		return nil, err
	}
	var out store.MonitorService
	err = s.st.InTx(ctx, func(q *store.Queries) error {
		if _, err := q.LockWorkspace(ctx, p.workspaceID); err != nil {
			return err
		}
		n, err := q.CountMonitorServices(ctx, p.workspaceID)
		if err != nil {
			return err
		}
		if n >= maxMonitorServices {
			return errServiceLimit
		}
		now := s.now()
		var next *time.Time
		if f.mode == modePull {
			next = &now
		}
		out, err = q.CreateMonitorService(ctx, store.CreateMonitorServiceParams{ID: newID(), WorkspaceID: p.workspaceID, Name: name, Environment: env,
			Origin: originMember, PropertyID: f.propertyID, Mode: f.mode, HealthUrl: f.healthURL, IntervalSeconds: f.interval, TimeoutSeconds: f.timeout,
			SilenceSeconds: f.silence, ErrorRateThreshold: f.errorRate, P95ThresholdMs: f.p95, NextPullAt: next, CreatedBy: p.memberRef(), Now: now})
		if store.IsNotFound(err) {
			return errServiceExists
		}
		if err != nil {
			return err
		}
		member := p.memberRef()
		if member == nil {
			return nil
		}
		_, err = q.CreateMonitorAlertRule(ctx, store.CreateMonitorAlertRuleParams{ID: newID(), WorkspaceID: p.workspaceID,
			Name: truncateRunes(serviceLabel(out)+" is degraded or down", 100), Kind: string(oas.MonitorRuleServiceState), ServiceID: &out.ID,
			WindowMinutes: defaultRuleWindow, MinRequests: defaultRuleMinRequests, Recipients: []uuid.UUID{*member}, Enabled: true, CreatedBy: member, Now: now})
		return err
	})
	if err != nil {
		return nil, err
	}
	body, err := s.serviceBodies(ctx, s.st.Queries, p.workspaceID, []store.MonitorService{out}, false)
	if err != nil {
		return nil, err
	}
	return oas.CreateMonitorService201JSONResponse(body[0]), nil
}

func (s *Server) UpdateMonitorService(ctx context.Context, req oas.UpdateMonitorServiceRequestObject) (oas.UpdateMonitorServiceResponseObject, error) {
	m, err := s.monitorOn()
	if err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if err := requireAnalyticsManager(p); err != nil {
		return nil, err
	}
	cur, err := s.getService(ctx, s.st.Queries, p.workspaceID, req.ServiceId, false)
	if err != nil {
		return nil, err
	}
	f := serviceFieldsOf(cur)
	f.apply(oas.MonitorServiceFields(*req.Body))
	if err := s.validateService(ctx, s.st.Queries, m, p, &f); err != nil {
		return nil, err
	}
	var out store.MonitorService
	err = s.st.InTx(ctx, func(q *store.Queries) error {
		cur, err := s.getService(ctx, q, p.workspaceID, req.ServiceId, true)
		if err != nil {
			return err
		}
		now := s.now()
		next := cur.NextPullAt
		switch {
		case f.mode != modePull:
			next = nil
		case cur.Mode != modePull || deref(cur.HealthUrl) != deref(f.healthURL) || cur.IntervalSeconds != f.interval || next == nil:
			next = &now
		}
		out, err = q.UpdateMonitorService(ctx, store.UpdateMonitorServiceParams{WorkspaceID: p.workspaceID, ID: cur.ID, PropertyID: f.propertyID, Mode: f.mode,
			HealthUrl: f.healthURL, IntervalSeconds: f.interval, TimeoutSeconds: f.timeout, SilenceSeconds: f.silence, ErrorRateThreshold: f.errorRate,
			P95ThresholdMs: f.p95, NextPullAt: next, Now: now})
		return err
	})
	if err != nil {
		return nil, err
	}
	body, err := s.serviceBodies(ctx, s.st.Queries, p.workspaceID, []store.MonitorService{out}, false)
	if err != nil {
		return nil, err
	}
	return oas.UpdateMonitorService200JSONResponse(body[0]), nil
}

func (s *Server) DeleteMonitorService(ctx context.Context, req oas.DeleteMonitorServiceRequestObject) (oas.DeleteMonitorServiceResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if err := requireAnalyticsManager(p); err != nil {
		return nil, err
	}
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		svc, err := s.getService(ctx, q, p.workspaceID, req.ServiceId, true)
		if err != nil {
			return err
		}
		deps, err := q.ListMonitorDependencies(ctx, store.ListMonitorDependenciesParams{WorkspaceID: p.workspaceID, ServiceIds: []uuid.UUID{svc.ID}})
		if err != nil {
			return err
		}
		depIDs := []uuid.UUID{}
		for _, d := range deps {
			depIDs = append(depIDs, d.ID)
		}
		if err := q.RemoveServiceFromIncidents(ctx, store.RemoveServiceFromIncidentsParams{WorkspaceID: p.workspaceID, ServiceID: svc.ID, DependencyIds: depIDs}); err != nil {
			return err
		}
		_, err = q.DeleteMonitorService(ctx, store.DeleteMonitorServiceParams{WorkspaceID: p.workspaceID, ID: svc.ID})
		return err
	})
	if err != nil {
		return nil, err
	}
	return oas.DeleteMonitorService204Response{}, nil
}

func (s *Server) serviceDependency(ctx context.Context, ws uuid.UUID, svc store.MonitorService, id *uuid.UUID) error {
	if id == nil {
		return nil
	}
	d, err := s.st.GetMonitorDependency(ctx, store.GetMonitorDependencyParams{WorkspaceID: ws, ID: *id})
	if store.IsNotFound(err) || (err == nil && d.ServiceID != svc.ID) {
		return problem(http.StatusNotFound, "not_found", "no such dependency of the service")
	}
	return err
}

func (s *Server) ListMonitorStateChanges(ctx context.Context, req oas.ListMonitorStateChangesRequestObject) (oas.ListMonitorStateChangesResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	limit := defaultChangesLimit
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	if limit < 1 || limit > 500 {
		return nil, errValidation("limit must be 1 to 500")
	}
	svc, err := s.getService(ctx, s.st.Queries, p.workspaceID, req.ServiceId, false)
	if err != nil {
		return nil, err
	}
	if err := s.serviceDependency(ctx, p.workspaceID, svc, req.Params.DependencyId); err != nil {
		return nil, err
	}
	rows, err := s.st.ListMonitorStateChanges(ctx, store.ListMonitorStateChangesParams{WorkspaceID: p.workspaceID, ServiceID: svc.ID,
		DependencyID: req.Params.DependencyId, Before: req.Params.Before, Lim: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := oas.ListMonitorStateChanges200JSONResponse{Items: make([]oas.MonitorStateChange, len(rows))}
	for i, r := range rows {
		out.Items[i] = oas.MonitorStateChange{Id: r.ID, ServiceId: r.ServiceID, DependencyId: r.DependencyID, Dependency: r.DependencyName,
			State: oas.MonitorServiceState(r.State), Previous: oas.MonitorServiceState(r.Previous), Reason: r.Reason, At: r.At.UTC(), Maintenance: r.Maintenance}
	}
	return out, nil
}

// window is a span of time inside a maintenance window.
type window struct{ from, to time.Time }

// serviceHistory spreads a subject's state changes over UTC days: the state of each instant
// outside maintenance, maintenance time apart, and the time not known yet as unknown.
func serviceHistory(changes []store.MonitorStateChange, windows []window, created, from, now time.Time, days int) oas.MonitorServiceHistory {
	out := oas.MonitorServiceHistory{Days: []oas.MonitorServiceHistoryDay{}}
	marks := []time.Time{}
	end := from.AddDate(0, 0, days)
	for d := from; d.Before(end); d = d.AddDate(0, 0, 1) {
		marks = append(marks, d)
	}
	marks = append(marks, end)
	for _, c := range changes {
		if c.At.After(from) && c.At.Before(end) {
			marks = append(marks, c.At)
		}
	}
	for _, w := range windows {
		for _, t := range []time.Time{w.from, w.to} {
			if t.After(from) && t.Before(end) {
				marks = append(marks, t)
			}
		}
	}
	for _, t := range []time.Time{created, now} {
		if t.After(from) && t.Before(end) {
			marks = append(marks, t)
		}
	}
	slices.SortFunc(marks, func(a, b time.Time) int { return a.Compare(b) })
	marks = slices.CompactFunc(marks, func(a, b time.Time) bool { return a.Equal(b) })
	stateAt := func(t time.Time) string {
		st := stateUnknown
		for _, c := range changes {
			if c.At.After(t) {
				break
			}
			st = c.State
		}
		return st
	}
	inWindow := func(t time.Time) bool {
		for _, w := range windows {
			if !t.Before(w.from) && t.Before(w.to) {
				return true
			}
		}
		return false
	}
	byDay := map[time.Time]*oas.MonitorServiceHistoryDay{}
	var order []time.Time
	var known, down int64
	for i := 0; i+1 < len(marks); i++ {
		a, b := marks[i], marks[i+1]
		day := utcDay(a)
		if !day.AddDate(0, 0, 1).After(created) {
			continue
		}
		d, ok := byDay[day]
		if !ok {
			d = &oas.MonitorServiceHistoryDay{Day: openapi_types.Date{Time: day}, Worst: oas.MonitorServiceUnknown}
			byDay[day] = d
			order = append(order, day)
		}
		secs := int(b.Sub(a) / time.Second)
		switch st := stateAt(a); {
		case a.Before(created) || !a.Before(now):
			d.UnknownSeconds += secs
		case inWindow(a):
			d.MaintenanceSeconds += secs
		default:
			switch st {
			case stateHealthy:
				d.HealthySeconds += secs
			case stateDegraded:
				d.DegradedSeconds += secs
			case stateDown:
				d.DownSeconds += secs
			default:
				d.UnknownSeconds += secs
			}
			if st != stateUnknown && (d.Worst == oas.MonitorServiceUnknown || rankOf(st) > rankOf(string(d.Worst))) {
				d.Worst = oas.MonitorServiceState(st)
			}
		}
	}
	for _, day := range order {
		d := byDay[day]
		k := int64(d.HealthySeconds + d.DegradedSeconds + d.DownSeconds)
		if k > 0 {
			d.Uptime = new(roundTo(100*float64(k-int64(d.DownSeconds))/float64(k), 3))
		}
		known, down = known+k, down+int64(d.DownSeconds)
		out.Days = append(out.Days, *d)
	}
	if known > 0 {
		out.Uptime = new(roundTo(100*float64(known-down)/float64(known), 3))
	}
	return out
}

func roundTo(v float64, digits int) float64 {
	p := 1.0
	for range digits {
		p *= 10
	}
	return float64(int64(v*p+0.5)) / p
}

func (s *Server) GetMonitorServiceHistory(ctx context.Context, req oas.GetMonitorServiceHistoryRequestObject) (oas.GetMonitorServiceHistoryResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	days := defaultHistoryDays
	if req.Params.Days != nil {
		days = *req.Params.Days
	}
	if days < 1 || days > maxHistoryDays {
		return nil, errValidation("days must be 1 to 400")
	}
	svc, err := s.getService(ctx, s.st.Queries, p.workspaceID, req.ServiceId, false)
	if err != nil {
		return nil, err
	}
	dep := req.Params.DependencyId
	if err := s.serviceDependency(ctx, p.workspaceID, svc, dep); err != nil {
		return nil, err
	}
	created := svc.CreatedAt
	if dep != nil {
		d, err := s.st.GetMonitorDependency(ctx, store.GetMonitorDependencyParams{WorkspaceID: p.workspaceID, ID: *dep})
		if err != nil {
			return nil, err
		}
		created = d.CreatedAt
	}
	now := s.now()
	from := utcDay(now).AddDate(0, 0, 1-days)
	changes, err := s.st.ListSubjectStateChanges(ctx, store.ListSubjectStateChangesParams{WorkspaceID: p.workspaceID, ServiceID: svc.ID, DependencyID: dep, FromAt: from})
	if err != nil {
		return nil, err
	}
	rows, err := s.st.ListMonitorMaintenanceBetween(ctx, store.ListMonitorMaintenanceBetweenParams{WorkspaceID: p.workspaceID, FromAt: &from, ToAt: &now})
	if err != nil {
		return nil, err
	}
	var windows []window
	for _, w := range rows {
		covers := slices.Contains(w.ServiceIds, svc.ID) || (dep != nil && slices.Contains(w.DependencyIds, *dep))
		if !covers || w.ScheduledStart == nil || w.ScheduledEnd == nil {
			continue
		}
		to := *w.ScheduledEnd
		if w.ResolvedAt != nil && w.ResolvedAt.Before(to) {
			to = *w.ResolvedAt
		}
		windows = append(windows, window{*w.ScheduledStart, to})
	}
	return oas.GetMonitorServiceHistory200JSONResponse(serviceHistory(changes, windows, created, from, now, days)), nil
}

func (s *Server) ListMonitorHealthHours(ctx context.Context, req oas.ListMonitorHealthHoursRequestObject) (oas.ListMonitorHealthHoursResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	hours := defaultHealthHours
	if req.Params.Hours != nil {
		hours = *req.Params.Hours
	}
	if hours < 1 || hours > maxHealthHours {
		return nil, errValidation("hours must be 1 to 2160")
	}
	svc, err := s.getService(ctx, s.st.Queries, p.workspaceID, req.ServiceId, false)
	if err != nil {
		return nil, err
	}
	from := s.now().UTC().Truncate(time.Hour).Add(-time.Duration(hours-1) * time.Hour)
	rows, err := s.st.ListMonitorHealthHours(ctx, store.ListMonitorHealthHoursParams{WorkspaceID: p.workspaceID, ServiceID: svc.ID, FromAt: from})
	if err != nil {
		return nil, err
	}
	out := oas.ListMonitorHealthHours200JSONResponse{Items: make([]oas.MonitorHealthHour, len(rows))}
	for i, r := range rows {
		h := oas.MonitorHealthHour{Hour: r.Hour.UTC(), Reports: r.Reports, Healthy: r.Healthy, Degraded: r.Degraded, Down: r.Down, Dependency: nilIfEmpty(r.Dependency)}
		if r.LatencyN > 0 {
			h.LatencyAvg = new(roundTo(r.LatencySum/float64(r.LatencyN), 3))
			if sk, err := monitor.DecodeSketch(r.Latencies); err == nil {
				ms := func(q float64) *float64 { return new(roundTo(sk.Quantile(q)/1000, 3)) }
				h.LatencyP50, h.LatencyP95, h.LatencyP99 = ms(0.5), ms(0.95), ms(0.99)
			}
		}
		out.Items[i] = h
	}
	return out, nil
}

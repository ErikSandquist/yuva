package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/productdevbook/yuva/api/internal/jobs"
	"github.com/productdevbook/yuva/api/internal/monitor"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	maxMonitorServices        = 100
	maxServiceDependencies    = 50
	defaultServiceInterval    = 60
	defaultServiceTimeout     = 10
	defaultServiceErrorRate   = 5
	serviceReportsPerMinute   = 600
	silenceIntervals          = 3
	recoveryEvaluations       = 2
	serviceWindow             = 5 * time.Minute
	serviceMinRequests        = 20
	autoIncidentHealthy       = 5 * time.Minute
	instanceSilence           = 24 * time.Hour
	stateChangesKept          = 400 * 24 * time.Hour
	healthHoursKept           = 90 * 24 * time.Hour
	pullsDueBatch             = 500
	healthAccept              = "application/health+json, application/vnd.spring-boot.actuator.v3+json, application/json;q=0.9, */*;q=0.5"
	stateUnknown              = string(oas.MonitorServiceUnknown)
	stateHealthy              = string(oas.MonitorServiceHealthy)
	stateDegraded             = string(oas.MonitorServiceDegraded)
	stateDown                 = string(oas.MonitorServiceDown)
	originReport              = string(oas.MonitorServiceFromReport)
	originMember              = string(oas.MonitorServiceFromMember)
	originRecords             = string(oas.MonitorServiceFromRecords)
	modePull                  = string(oas.MonitorServicePull)
	modePush                  = string(oas.MonitorServicePush)
	maxCachedServiceSightings = 100_000
)

var (
	errServiceGone     = problem(http.StatusNotFound, "not_found", "no such service")
	errServiceLimit    = problem(http.StatusForbidden, "service_limit", "a workspace has at most 100 services")
	errServiceExists   = problem(http.StatusConflict, "service_exists", "the workspace has a service of this name and environment")
	errHealthNeedsKey  = problem(http.StatusForbidden, "forbidden", "only an API key can report health")
	limitServiceReport = limit{serviceReportsPerMinute, time.Minute}
)

// healthReport is one report about a service: pushed, or what a pull found.
type healthReport struct {
	at       time.Time
	status   string
	message  string
	version  string
	instance string
	checks   []monitor.HealthCheck
}

// nextHealthState applies a raw evaluation to a subject's state: a worse state, and unknown when
// nothing is known any more, at once; a better one once two evaluations in a row found that same
// state; a subject that was unknown takes any state at once.
func nextHealthState(cur, pending string, inRow int32, raw string) (string, string, int32) {
	switch {
	case raw == cur:
		return cur, "", 0
	case cur == stateUnknown, raw == stateUnknown:
		return raw, "", 0
	case monitor.HealthRank(raw) > monitor.HealthRank(cur):
		return raw, "", 0
	}
	if pending == raw {
		inRow++
	} else {
		pending, inRow = raw, 1
	}
	if inRow >= recoveryEvaluations {
		return raw, "", 0
	}
	return cur, pending, inRow
}

func silenceAfter(svc store.MonitorService) time.Duration {
	if svc.SilenceSeconds != nil {
		return time.Duration(*svc.SilenceSeconds) * time.Second
	}
	return time.Duration(silenceIntervals*svc.IntervalSeconds) * time.Second
}

// maintenance is what the active maintenance windows of a workspace cover.
type maintenance struct {
	services, dependencies, checks map[uuid.UUID]bool
}

func (m maintenance) service(id uuid.UUID) bool { return m.services[id] }

func (m maintenance) dependency(d store.MonitorDependency) bool {
	return m.services[d.ServiceID] || m.dependencies[d.ID]
}

func (m maintenance) check(id uuid.UUID) bool { return m.checks[id] }

func maintenanceActive(i store.MonitorIncident, now time.Time) bool {
	return i.Kind == string(oas.MonitorIncidentMaintenance) && i.ResolvedAt == nil && i.ScheduledStart != nil && !now.Before(*i.ScheduledStart) &&
		i.ScheduledEnd != nil && now.Before(*i.ScheduledEnd)
}

func (s *Server) activeMaintenance(ctx context.Context, q *store.Queries, ws uuid.UUID, now time.Time) (maintenance, error) {
	m := maintenance{services: map[uuid.UUID]bool{}, dependencies: map[uuid.UUID]bool{}, checks: map[uuid.UUID]bool{}}
	open, err := q.ListOpenMonitorIncidents(ctx, ws)
	if err != nil {
		return m, err
	}
	for _, i := range open {
		if !maintenanceActive(i, now) {
			continue
		}
		for _, id := range i.ServiceIds {
			m.services[id] = true
		}
		for _, id := range i.DependencyIds {
			m.dependencies[id] = true
		}
		for _, id := range i.CheckIds {
			m.checks[id] = true
		}
	}
	return m, nil
}

// serviceTraffic is the linked property's last 5 closed minutes of the service's requests.
type serviceTraffic struct {
	requests       int64
	errorRate, p95 float64
}

func (s *Server) serviceTraffic(ctx context.Context, svc store.MonitorService) (*serviceTraffic, error) {
	m := s.analytics.monitor
	if svc.PropertyID == nil || m == nil {
		return nil, nil
	}
	prop, err := s.st.GetAnalyticsProperty(ctx, store.GetAnalyticsPropertyParams{WorkspaceID: svc.WorkspaceID, ID: *svc.PropertyID})
	if store.IsNotFound(err) || (err == nil && !isAPIProperty(prop)) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	now := s.now()
	end := now.Add(-alertLag).Truncate(time.Minute)
	q := monitor.Query{WorkspaceID: prop.WorkspaceID, PropertyID: prop.ID, Zone: monitorZone(prop.Timezone, prop.TimezoneHistory), Start: end.Add(-serviceWindow),
		End: end, Metrics: []string{"requests", "error_rate", "p95"}, Now: now, ApdexMS: int64(prop.ApdexMs),
		MinuteFrom: monitor.LocalDay(now, time.UTC).AddDate(0, 0, 1-m.settings.MinuteDays),
		Filters: []monitor.Filter{{Dimension: "service", Op: "is", Values: []string{svc.Name, ""}},
			{Dimension: "environment", Op: "is", Values: []string{svc.Environment, ""}}}}
	res, err := m.store.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return &serviceTraffic{requests: res.Totals.Requests, errorRate: res.Totals.Metric("error_rate", 0), p95: res.Totals.Metric("p95", 0)}, nil
}

// cause is one reason a service is not healthy.
type cause struct {
	state, text string
}

// rawServiceState is a service's state from what is known now, before the hysteresis, with the
// reason of the worst cause.
func rawServiceState(svc store.MonitorService, deps []store.MonitorDependency, checks []store.MonitorCheck, traffic *serviceTraffic, mt maintenance,
	now time.Time) (string, string) {
	var causes []cause
	reported := svc.LastReportAt != nil
	silent := reported && now.Sub(*svc.LastReportAt) > silenceAfter(svc)
	if silent {
		causes = append(causes, cause{stateDown, "No report for " + durationText(now.Sub(*svc.LastReportAt))})
	}
	if reported && !silent {
		switch deref(svc.LastStatus) {
		case stateDown:
			causes = append(causes, cause{stateDown, firstText(deref(svc.LastMessage), "The service reports itself down")})
		case stateDegraded:
			causes = append(causes, cause{stateDegraded, firstText(deref(svc.LastMessage), "The service reports itself degraded")})
		}
		for _, d := range deps {
			if mt.dependencies[d.ID] || !dependencyReported(svc, d) {
				continue
			}
			switch deref(d.Status) {
			case stateDown:
				if d.Critical {
					causes = append(causes, cause{stateDown, d.Name + " is down"})
				} else {
					causes = append(causes, cause{stateDegraded, d.Name + " is down"})
				}
			case stateDegraded:
				causes = append(causes, cause{stateDegraded, d.Name + " is degraded"})
			}
		}
	}
	for _, c := range checks {
		if !c.Paused && c.State == resultDown && !mt.check(c.ID) {
			causes = append(causes, cause{stateDown, "The uptime check " + c.Name + " is down"})
		}
	}
	if traffic != nil && traffic.requests >= serviceMinRequests {
		if traffic.errorRate >= svc.ErrorRateThreshold {
			causes = append(causes, cause{stateDegraded, "5xx rate " + strconv.FormatFloat(traffic.errorRate, 'f', 1, 64) + "% over 5 minutes"})
		}
		if svc.P95ThresholdMs != nil && traffic.p95 > *svc.P95ThresholdMs {
			causes = append(causes, cause{stateDegraded, "p95 " + strconv.FormatFloat(traffic.p95, 'f', 0, 64) + " ms over 5 minutes"})
		}
	}
	state, reason := stateUnknown, ""
	if reported && !silent {
		state = stateHealthy
	}
	for _, c := range causes {
		if state == stateUnknown || state == stateHealthy || monitor.HealthRank(c.state) > monitor.HealthRank(state) {
			state, reason = c.state, c.text
		}
	}
	return state, reason
}

func firstText(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

func durationText(d time.Duration) string {
	switch {
	case d >= 2*time.Hour:
		return strconv.Itoa(int(d/time.Hour)) + " hours"
	case d >= 2*time.Minute:
		return strconv.Itoa(int(d/time.Minute)) + " minutes"
	}
	return strconv.Itoa(int(d/time.Second)) + " seconds"
}

func dependencyReported(svc store.MonitorService, d store.MonitorDependency) bool {
	return d.ReportedAt != nil && svc.LastReportAt != nil && d.ReportedAt.Equal(*svc.LastReportAt)
}

// rawDependencyState is the dependency's last reported status while its service's last report
// listed it and the service is not silent.
func rawDependencyState(svc store.MonitorService, d store.MonitorDependency, now time.Time) string {
	if !dependencyReported(svc, d) || now.Sub(*svc.LastReportAt) > silenceAfter(svc) || d.Status == nil {
		return stateUnknown
	}
	return *d.Status
}

// serviceChange is what one evaluation changed.
type serviceChange struct {
	svc          store.MonitorService
	previous     string
	changed      bool
	depsChanged  bool
	maintenance  bool
	ignored      []string
	dependencies []store.MonitorDependency
}

// evaluateService applies report (if any) to the service and evaluates its state and its
// dependencies' in one transaction that holds the service, then opens incidents and applies alert
// rules for what changed.
func (s *Server) evaluateService(ctx context.Context, ws, id uuid.UUID, report *healthReport) (serviceChange, error) {
	var ch serviceChange
	pre, err := s.st.GetMonitorService(ctx, store.GetMonitorServiceParams{WorkspaceID: ws, ID: id})
	if err != nil {
		if store.IsNotFound(err) {
			return ch, errServiceGone
		}
		return ch, err
	}
	traffic, err := s.serviceTraffic(ctx, pre)
	if err != nil {
		s.log.WarnContext(ctx, "monitor service traffic", slog.String("workspace_id", ws.String()), slog.Any("error", err))
		traffic = nil
	}
	err = s.st.InTx(ctx, func(q *store.Queries) error {
		svc, err := q.LockMonitorService(ctx, store.LockMonitorServiceParams{WorkspaceID: ws, ID: id})
		if store.IsNotFound(err) {
			return errServiceGone
		}
		if err != nil {
			return err
		}
		now := s.now()
		if report != nil {
			if ch.ignored, err = s.applyReport(ctx, q, &svc, *report); err != nil {
				return err
			}
		}
		mt, err := s.activeMaintenance(ctx, q, ws, now)
		if err != nil {
			return err
		}
		deps, err := q.ListMonitorDependencies(ctx, store.ListMonitorDependenciesParams{WorkspaceID: ws, ServiceIds: []uuid.UUID{svc.ID}})
		if err != nil {
			return err
		}
		checks, err := q.ListMonitorChecksForServices(ctx, store.ListMonitorChecksForServicesParams{WorkspaceID: ws, Names: []string{svc.Name}})
		if err != nil {
			return err
		}
		ch.maintenance = mt.service(svc.ID)
		for i, d := range deps {
			raw := rawDependencyState(svc, d, now)
			next, pending, inRow := nextHealthState(d.State, deref(d.BetterState), d.BetterInRow, raw)
			if next == d.State && pending == deref(d.BetterState) && inRow == d.BetterInRow {
				continue
			}
			since := d.StateSince
			if next != d.State {
				since = &now
				ch.depsChanged = true
				reason := d.Name + " is " + next
				if d.Message != nil && next != stateHealthy && next != stateUnknown {
					reason += ": " + *d.Message
				}
				if err := q.InsertMonitorStateChange(ctx, store.InsertMonitorStateChangeParams{ID: newID(), WorkspaceID: ws, ServiceID: svc.ID, DependencyID: &d.ID,
					State: next, Previous: d.State, Reason: new(truncateRunes(reason, 300)), Maintenance: mt.dependency(d), At: now}); err != nil {
					return err
				}
			}
			if err := q.SetMonitorDependencyState(ctx, store.SetMonitorDependencyStateParams{WorkspaceID: ws, ID: d.ID, State: next, StateSince: since,
				BetterState: nilIfEmpty(pending), BetterInRow: inRow}); err != nil {
				return err
			}
			deps[i].State, deps[i].StateSince = next, since
		}
		ch.dependencies = deps
		raw, reason := rawServiceState(svc, deps, checks, traffic, mt, now)
		ch.previous = svc.State
		next, pending, inRow := nextHealthState(svc.State, deref(svc.BetterState), svc.BetterInRow, raw)
		since := svc.StateSince
		keep := svc.Reason
		if next != svc.State {
			since, ch.changed = &now, true
			keep = nilIfEmpty(truncateRunes(reason, 300))
			if err := q.InsertMonitorStateChange(ctx, store.InsertMonitorStateChangeParams{ID: newID(), WorkspaceID: ws, ServiceID: svc.ID, State: next,
				Previous: svc.State, Reason: keep, Maintenance: ch.maintenance, At: now}); err != nil {
				return err
			}
		} else if next == raw {
			keep = nilIfEmpty(truncateRunes(reason, 300))
		}
		svc, err = q.SetMonitorServiceState(ctx, store.SetMonitorServiceStateParams{WorkspaceID: ws, ID: svc.ID, State: next, StateSince: since, Reason: keep,
			BetterState: nilIfEmpty(pending), BetterInRow: inRow})
		ch.svc = svc
		return err
	})
	if err != nil {
		return ch, err
	}
	if ch.changed && !ch.maintenance {
		if err := s.autoIncident(ctx, ch.svc, ch.previous); err != nil {
			s.log.WarnContext(ctx, "monitor incident", slog.String("workspace_id", ws.String()), slog.Any("error", err))
		}
	}
	if ch.changed || ch.depsChanged {
		if err := s.serviceRulesOn(ctx, ch.svc); err != nil {
			s.log.WarnContext(ctx, "monitor service alerts", slog.String("workspace_id", ws.String()), slog.Any("error", err))
		}
	}
	return ch, nil
}

func nilIfEmpty(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// applyReport records a report on the locked service: its own status, version and instance, its
// dependencies (new names past 50 are ignored) and the hour's counts and latencies.
func (s *Server) applyReport(ctx context.Context, q *store.Queries, svc *store.MonitorService, r healthReport) ([]string, error) {
	ws := svc.WorkspaceID
	ignored := []string{}
	deps, err := q.ListMonitorDependencies(ctx, store.ListMonitorDependenciesParams{WorkspaceID: ws, ServiceIds: []uuid.UUID{svc.ID}})
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, d := range deps {
		known[d.Name] = true
	}
	n := len(deps)
	var taken []monitor.HealthCheck
	for _, c := range r.checks {
		if !known[c.Name] {
			if n >= maxServiceDependencies {
				ignored = append(ignored, c.Name)
				continue
			}
			n++
			known[c.Name] = true
		}
		taken = append(taken, c)
	}
	at := r.at
	for _, c := range taken {
		arg := store.UpsertMonitorDependencyParams{ID: newID(), WorkspaceID: ws, ServiceID: svc.ID, Name: c.Name, Kind: c.Kind, Critical: c.Critical, Status: &c.Status,
			LatencyMs: c.LatencyMS, Message: nilIfEmpty(c.Message), At: &at}
		if _, err := q.UpsertMonitorDependency(ctx, arg); err != nil {
			return nil, err
		}
	}
	if err := q.SetMonitorServiceReport(ctx, store.SetMonitorServiceReportParams{WorkspaceID: ws, ID: svc.ID, At: &at, Status: &r.status,
		LastMessage: nilIfEmpty(truncateRunes(r.message, monitor.HealthMessageRunes)), Version: nilIfEmpty(r.version)}); err != nil {
		return nil, err
	}
	svc.LastReportAt, svc.LastStatus, svc.LastMessage = &at, &r.status, nilIfEmpty(truncateRunes(r.message, monitor.HealthMessageRunes))
	if r.version != "" {
		svc.Version = &r.version
	}
	if r.instance != "" {
		if err := q.UpsertMonitorInstance(ctx, store.UpsertMonitorInstanceParams{WorkspaceID: ws, ServiceID: svc.ID, Instance: r.instance,
			Version: nilIfEmpty(r.version), Status: &r.status, At: at}); err != nil {
			return nil, err
		}
	}
	return ignored, s.recordHealthHour(ctx, q, *svc, r.status, taken, at)
}

func statusCounts(status string) (int64, int64, int64) {
	switch status {
	case stateDown:
		return 0, 0, 1
	case stateDegraded:
		return 0, 1, 0
	}
	return 1, 0, 0
}

// recordHealthHour adds a report to its UTC hour: the service's own row (dependency ”) and one per
// dependency, whose latency goes into the hour's sketch. The service row is locked, so reports of
// one service never race on a sketch.
func (s *Server) recordHealthHour(ctx context.Context, q *store.Queries, svc store.MonitorService, status string, checks []monitor.HealthCheck, at time.Time) error {
	hour := at.UTC().Truncate(time.Hour)
	rows, err := q.ListMonitorHealthHoursAt(ctx, store.ListMonitorHealthHoursAtParams{WorkspaceID: svc.WorkspaceID, ServiceID: svc.ID, Hour: hour})
	if err != nil {
		return err
	}
	sketches := map[string][]byte{}
	for _, r := range rows {
		sketches[r.Dependency] = r.Latencies
	}
	h, d, dn := statusCounts(status)
	if err := q.UpsertMonitorHealthHour(ctx, store.UpsertMonitorHealthHourParams{WorkspaceID: svc.WorkspaceID, ServiceID: svc.ID, Dependency: "", Hour: hour,
		Healthy: h, Degraded: d, Down: dn}); err != nil {
		return err
	}
	for _, c := range checks {
		h, d, dn := statusCounts(c.Status)
		arg := store.UpsertMonitorHealthHourParams{WorkspaceID: svc.WorkspaceID, ServiceID: svc.ID, Dependency: c.Name, Hour: hour, Healthy: h, Degraded: d, Down: dn}
		if c.LatencyMS != nil {
			sk := monitor.NewSketch(monitor.DurationScale)
			if raw := sketches[c.Name]; raw != nil {
				if old, err := monitor.DecodeSketch(raw); err == nil {
					sk = old
				}
			}
			sk.Add(int64(*c.LatencyMS*1000), 1)
			arg.Latencies, arg.LatencySum, arg.LatencyN = sk.Encode(), *c.LatencyMS, 1
		}
		if err := q.UpsertMonitorHealthHour(ctx, arg); err != nil {
			return err
		}
	}
	return nil
}

// validReport checks a pushed report; checks get their defaults.
func validReport(b oas.MonitorHealthReport, at time.Time) (healthReport, string, string, error) {
	r := healthReport{at: at, status: stateHealthy}
	name, err := trimmed(b.Service, 1, 100, "service")
	if err != nil {
		return r, "", "", err
	}
	env := monitor.DefaultEnvironment
	if b.Environment != nil {
		if env, err = trimmed(*b.Environment, 1, 40, "environment"); err != nil {
			return r, "", "", err
		}
	}
	if b.Version != nil {
		if r.version, err = trimmed(*b.Version, 1, 80, "version"); err != nil {
			return r, "", "", err
		}
	}
	if b.Instance != nil {
		if r.instance, err = trimmed(*b.Instance, 1, 200, "instance"); err != nil {
			return r, "", "", err
		}
	}
	if b.Status != nil {
		if !b.Status.Valid() {
			return r, "", "", errValidation("status must be healthy, degraded or down")
		}
		r.status = string(*b.Status)
	}
	if b.Checks != nil {
		if len(*b.Checks) > maxServiceDependencies {
			return r, "", "", errValidation("a report has at most 50 checks")
		}
		seen := map[string]bool{}
		for i, c := range *b.Checks {
			at := "checks[" + strconv.Itoa(i) + "]."
			hc := monitor.HealthCheck{Kind: "other", Critical: true}
			if hc.Name, err = trimmed(c.Name, 1, 100, at+"name"); err != nil {
				return r, "", "", err
			}
			if seen[hc.Name] {
				return r, "", "", errValidation("the check " + hc.Name + " is listed twice")
			}
			seen[hc.Name] = true
			if !c.Status.Valid() {
				return r, "", "", errValidation(at + "status must be healthy, degraded or down")
			}
			hc.Status = string(c.Status)
			if c.Kind != nil {
				if !c.Kind.Valid() {
					return r, "", "", errValidation(at + "kind must be database, cache, queue, http or other")
				}
				hc.Kind = string(*c.Kind)
			}
			if c.Critical != nil {
				hc.Critical = *c.Critical
			}
			if c.LatencyMs != nil {
				if *c.LatencyMs < 0 || *c.LatencyMs > 3_600_000 {
					return r, "", "", errValidation(at + "latency_ms must be 0 to 3,600,000")
				}
				hc.LatencyMS = new(*c.LatencyMs)
			}
			if c.Message != nil {
				m := strings.TrimSpace(*c.Message)
				if len([]rune(m)) > monitor.HealthMessageRunes {
					return r, "", "", errValidation(at + "message must be at most 200 characters")
				}
				hc.Message = m
			}
			r.checks = append(r.checks, hc)
		}
	}
	return r, name, env, nil
}

// ensureService finds a service by name and environment or makes it, within the workspace's 100.
// It returns nil without an error when the cap is reached and capped is false.
func (s *Server) ensureService(ctx context.Context, ws uuid.UUID, name, env, origin string, prop *uuid.UUID) (*store.MonitorService, bool, error) {
	svc, err := s.st.GetMonitorServiceByName(ctx, store.GetMonitorServiceByNameParams{WorkspaceID: ws, Name: name, Environment: env})
	if err == nil {
		return &svc, false, nil
	}
	if !store.IsNotFound(err) {
		return nil, false, err
	}
	var out *store.MonitorService
	err = s.st.InTx(ctx, func(q *store.Queries) error {
		if _, err := q.LockWorkspace(ctx, ws); err != nil {
			return err
		}
		got, err := q.GetMonitorServiceByName(ctx, store.GetMonitorServiceByNameParams{WorkspaceID: ws, Name: name, Environment: env})
		if err == nil {
			out = &got
			return nil
		}
		if !store.IsNotFound(err) {
			return err
		}
		n, err := q.CountMonitorServices(ctx, ws)
		if err != nil {
			return err
		}
		if n >= maxMonitorServices {
			return errServiceLimit
		}
		made, err := q.CreateMonitorService(ctx, store.CreateMonitorServiceParams{ID: newID(), WorkspaceID: ws, Name: name, Environment: env, Origin: origin,
			PropertyID: prop, Mode: modePush, IntervalSeconds: defaultServiceInterval, TimeoutSeconds: defaultServiceTimeout,
			ErrorRateThreshold: defaultServiceErrorRate, Now: s.now()})
		if err != nil {
			return err
		}
		out = &made
		return nil
	})
	return out, out != nil, err
}

func (s *Server) SendMonitorHealth(ctx context.Context, req oas.SendMonitorHealthRequestObject) (oas.SendMonitorHealthResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if !p.isKey() {
		return nil, errHealthNeedsKey
	}
	if req.Body == nil {
		return nil, errValidation("the body is a health report")
	}
	now := s.now()
	r, name, env, err := validReport(*req.Body, now)
	if err != nil {
		return nil, err
	}
	if ok, wait := s.analytics.limits.allowWait("mh:"+p.workspaceID.String()+":"+env+":"+name, limitServiceReport, now); !ok {
		return nil, rateLimited(wait)
	}
	svc, _, err := s.ensureService(ctx, p.workspaceID, name, env, originReport, nil)
	if err != nil {
		return nil, err
	}
	ch, err := s.evaluateService(ctx, p.workspaceID, svc.ID, &r)
	if err != nil {
		return nil, err
	}
	return oas.SendMonitorHealth200JSONResponse{ServiceId: ch.svc.ID, State: oas.MonitorServiceState(ch.svc.State), IgnoredChecks: ch.ignored}, nil
}

// MonitorPullArgs pulls one service's health URL for one slot; attempt 2 is the try 20 seconds
// after one that got no answer.
type MonitorPullArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	ServiceID   uuid.UUID `json:"service_id"`
	Slot        int64     `json:"slot"`
	Attempt     int       `json:"attempt"`
}

func (MonitorPullArgs) Kind() string { return "monitor_pull" }

func (MonitorPullArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.MonitoringQueue, MaxAttempts: 1, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

type monitorPullWorker struct {
	river.WorkerDefaults[MonitorPullArgs]
	s *Server
}

func (w *monitorPullWorker) Timeout(*river.Job[MonitorPullArgs]) time.Duration {
	return 30*time.Second + checkJobSlack
}

func (w *monitorPullWorker) Work(ctx context.Context, job *river.Job[MonitorPullArgs]) error {
	return w.s.RunMonitorPull(ctx, job.Args)
}

type monitorStatesWorker struct {
	river.WorkerDefaults[jobs.MonitorStatesArgs]
	s *Server
}

func (w *monitorStatesWorker) Work(ctx context.Context, _ *river.Job[jobs.MonitorStatesArgs]) error {
	return w.s.EvaluateMonitorStates(ctx)
}

// QueueDueMonitorPulls takes the pull services that are due, moves each one's next pull on by its
// interval and queues one pull per service and slot.
func (s *Server) QueueDueMonitorPulls(ctx context.Context) (int, error) {
	if _, err := s.monitorOn(); err != nil {
		return 0, nil
	}
	now := s.now()
	queued := 0
	err := s.st.InTxRaw(ctx, func(tx pgx.Tx, q *store.Queries) error {
		due, err := q.ListDueMonitorPulls(ctx, store.ListDueMonitorPullsParams{Now: &now, Lim: pullsDueBatch})
		if err != nil || len(due) == 0 {
			return err
		}
		batch := make([]river.InsertManyParams, 0, len(due))
		for _, d := range due {
			every := time.Duration(d.IntervalSeconds) * time.Second
			next := d.NextPullAt.Add(every)
			if !next.After(now) {
				next = now.Add(every)
			}
			if err := q.SetMonitorServiceNextPull(ctx, store.SetMonitorServiceNextPullParams{WorkspaceID: d.WorkspaceID, ID: d.ID, NextPullAt: &next}); err != nil {
				return err
			}
			batch = append(batch, river.InsertManyParams{Args: MonitorPullArgs{WorkspaceID: d.WorkspaceID, ServiceID: d.ID, Slot: d.NextPullAt.Unix(), Attempt: 1}})
		}
		queued = len(batch)
		_, err = s.jobs.InsertManyTx(ctx, tx, batch)
		return err
	})
	return queued, err
}

// pullReport is what one pull of a health URL says: the endpoint's own reading, or down when it
// gave no answer.
func pullReport(res monitor.ProbeResult, at time.Time) healthReport {
	r := healthReport{at: at}
	if !res.Answered {
		r.status, r.message = stateDown, "The health endpoint gave no answer: "+res.Detail
		return r
	}
	reading := monitor.ParseHealth(res.Status, res.ContentType, res.Body)
	r.status, r.version, r.checks = reading.Status, reading.Version, reading.Checks
	if reading.Format == monitor.FormatStatus && r.status == stateDown {
		r.message = "The health endpoint answered " + strconv.Itoa(res.Status)
	}
	if len(r.checks) > maxServiceDependencies {
		r.checks = r.checks[:maxServiceDependencies]
	}
	return r
}

// RunMonitorPull calls a pull service's health URL through the uptime runner and evaluates what it
// said; a pull with no answer is tried again 20 seconds later before it counts.
func (s *Server) RunMonitorPull(ctx context.Context, a MonitorPullArgs) error {
	m, err := s.monitorOn()
	if err != nil {
		return nil
	}
	svc, err := s.st.GetMonitorService(ctx, store.GetMonitorServiceParams{WorkspaceID: a.WorkspaceID, ID: a.ServiceID})
	if store.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if svc.Mode != modePull || svc.HealthUrl == nil {
		return nil
	}
	at := s.now()
	res := m.prober.Probe(ctx, monitor.ProbeSpec{URL: *svc.HealthUrl, Method: http.MethodGet, Timeout: time.Duration(svc.TimeoutSeconds) * time.Second,
		MaxRedirects: defaultCheckRedirects, UserAgent: "Yuva-Uptime/" + s.version, AnyStatus: true, KeepBody: true, Accept: healthAccept})
	if !res.Answered && a.Attempt <= 1 {
		retry := a
		retry.Attempt = 2
		_, err := s.jobs.Insert(ctx, retry, &river.InsertOpts{ScheduledAt: at.Add(checkRetryDelay)})
		return err
	}
	r := pullReport(res, at)
	_, err = s.evaluateService(ctx, a.WorkspaceID, a.ServiceID, &r)
	if err == errServiceGone {
		return nil
	}
	return err
}

// EvaluateMonitorStates is the monitor_states job: in every workspace with services or open
// incidents it ends the maintenance windows that passed their end, evaluates every service and
// resolves the automatic incidents of services healthy for 5 minutes.
func (s *Server) EvaluateMonitorStates(ctx context.Context) error {
	if _, err := s.monitorOn(); err != nil {
		return nil
	}
	ids, err := s.st.ListMonitorHealthWorkspaces(ctx)
	if err != nil {
		return err
	}
	for _, ws := range ids {
		if err := s.evaluateWorkspaceHealth(ctx, ws); err != nil {
			s.log.WarnContext(ctx, "monitor states", slog.String("workspace_id", ws.String()), slog.Any("error", err))
		}
	}
	return nil
}

func (s *Server) evaluateWorkspaceHealth(ctx context.Context, ws uuid.UUID) error {
	if err := s.endMaintenance(ctx, ws); err != nil {
		return err
	}
	svcs, err := s.st.ListMonitorServices(ctx, store.ListMonitorServicesParams{WorkspaceID: ws})
	if err != nil {
		return err
	}
	byID := map[uuid.UUID]store.MonitorService{}
	for _, svc := range svcs {
		ch, err := s.evaluateService(ctx, ws, svc.ID, nil)
		if err == errServiceGone {
			continue
		}
		if err != nil {
			return err
		}
		byID[svc.ID] = ch.svc
	}
	return s.resolveHealedIncidents(ctx, ws, byID)
}

// serviceSighting is a service a record or span named, as this process last noted it.
type serviceSighting struct {
	ws                  uuid.UUID
	name, env, instance string
}

// noteServices makes the services that records name, links them to the property when they have
// none, and notes their version and instance, at most once a minute per service and instance in
// each process; a failure only costs the note.
func (s *Server) noteServices(ctx context.Context, m *monitorState, prop store.AnalyticsProperty, recs []monitor.Record) {
	now := s.now()
	type seen struct {
		version string
		at      time.Time
	}
	fresh := map[serviceSighting]seen{}
	m.sightings.mu.Lock()
	for i := range recs {
		r := &recs[i]
		if r.Service == "" {
			continue
		}
		k := serviceSighting{prop.WorkspaceID, r.Service, r.Environment, r.Instance}
		if last, ok := m.sightings.at[k]; ok && now.Sub(last) < time.Minute {
			continue
		}
		f := fresh[k]
		if r.Release != "" {
			f.version = r.Release
		}
		if r.Time.After(f.at) {
			f.at = r.Time
		}
		fresh[k] = f
	}
	if m.sightings.at == nil || len(m.sightings.at) > maxCachedServiceSightings {
		m.sightings.at = map[serviceSighting]time.Time{}
	}
	for k := range fresh {
		m.sightings.at[k] = now
	}
	m.sightings.mu.Unlock()
	for k, f := range fresh {
		svc, _, err := s.ensureService(ctx, k.ws, k.name, k.env, originRecords, &prop.ID)
		if err == errServiceLimit {
			continue
		}
		if err != nil {
			s.log.WarnContext(ctx, "monitor services from records", slog.Any("error", err))
			continue
		}
		if err := s.st.NoteMonitorServiceRecords(ctx, store.NoteMonitorServiceRecordsParams{WorkspaceID: k.ws, ID: svc.ID, Version: nilIfEmpty(f.version),
			PropertyID: &prop.ID}); err != nil {
			s.log.WarnContext(ctx, "monitor services from records", slog.Any("error", err))
			continue
		}
		if k.instance != "" {
			at := f.at
			if at.After(now) || at.IsZero() {
				at = now
			}
			if err := s.st.UpsertMonitorInstance(ctx, store.UpsertMonitorInstanceParams{WorkspaceID: k.ws, ServiceID: svc.ID, Instance: k.instance,
				Version: nilIfEmpty(f.version), At: at}); err != nil {
				s.log.WarnContext(ctx, "monitor services from records", slog.Any("error", err))
			}
		}
	}
}

// trimMonitorHealth deletes instances silent for 24 hours, state changes past 400 days and health
// hours past 90.
func (s *Server) trimMonitorHealth(ctx context.Context, ws uuid.UUID) error {
	now := s.now()
	if _, err := s.st.DeleteSilentMonitorInstances(ctx, store.DeleteSilentMonitorInstancesParams{WorkspaceID: ws, Before: now.Add(-instanceSilence)}); err != nil {
		return err
	}
	if _, err := s.st.DeleteOldMonitorStateChanges(ctx, store.DeleteOldMonitorStateChangesParams{WorkspaceID: ws, Before: now.Add(-stateChangesKept)}); err != nil {
		return err
	}
	_, err := s.st.DeleteOldMonitorHealthHours(ctx, store.DeleteOldMonitorHealthHoursParams{WorkspaceID: ws, Before: now.Add(-healthHoursKept)})
	return err
}

func stateValue(state string) float64 {
	return float64(monitor.HealthRank(state))
}

func serviceLabel(svc store.MonitorService) string {
	if svc.Environment == monitor.DefaultEnvironment {
		return svc.Name
	}
	return fmt.Sprintf("%s (%s)", svc.Name, svc.Environment)
}

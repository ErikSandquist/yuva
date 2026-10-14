package api

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/mail"
	"github.com/productdevbook/yuva/api/internal/monitor"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	maxMonitorRules        = 200
	maxRuleRecipients      = 20
	defaultRuleWindow      = 5
	defaultRuleMinRequests = 20
	default5xxThreshold    = 5
	// Alerts judge the minutes that closed: a minute's rows are written within 10 seconds of a
	// record, so the window ends at the minute that ended 30 seconds ago.
	alertLag             = 30 * time.Second
	trafficWindow        = 15 * time.Minute
	trafficBaselineDays  = 7
	trafficMinBaseline   = 50
	spikeFactor          = 3
	dropShare            = 0.5
	alertMargin          = 0.9
	silentPerMinute      = 1
	monitorAlertPushTTL  = time.Hour
	eventMonitorAlert    = string(oas.MonitorAlertEventTypeMonitorAlert)
	monitorAlertsDefault = 50
)

var (
	errRuleGone  = problem(http.StatusNotFound, "not_found", "no such alert rule")
	errRuleLimit = problem(http.StatusForbidden, "rule_limit", "a workspace has at most 200 alert rules")

	metricRuleKinds  = []string{"error_rate", "client_error_rate", "p95", "p99", "apdex"}
	trafficRuleKinds = []string{"spike", "drop", "silent"}
	checkRuleKinds   = []string{"check_state", "certificate"}
	ownerRuleKinds   = []string{"check_state", "certificate", "service_state"}
)

func ruleBody(r store.MonitorAlertRule, firing int64) oas.MonitorAlertRule {
	out := oas.MonitorAlertRule{Id: r.ID, Name: r.Name, Kind: oas.MonitorAlertRuleKind(r.Kind), PropertyId: r.PropertyID, Route: r.Route, Service: r.Service,
		CheckId: r.CheckID, ServiceId: r.ServiceID, DependencyId: r.DependencyID, Threshold: r.Threshold, WindowMinutes: int(r.WindowMinutes), MinRequests: int(r.MinRequests), Recipients: nonNil(r.Recipients),
		Enabled: r.Enabled, Firing: int(firing), CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	return out
}

func monitorAlertBody(a store.MonitorAlert, r store.MonitorAlertRule) oas.MonitorAlert {
	out := oas.MonitorAlert{Id: a.ID, RuleId: a.RuleID, RuleName: r.Name, Kind: oas.MonitorAlertRuleKind(a.Kind), Key: a.Key, State: oas.MonitorAlertFiring,
		PropertyId: a.PropertyID, Route: r.Route, Service: r.Service, CheckId: a.CheckID, ServiceId: a.ServiceID, DependencyId: a.DependencyID, Value: a.Value, Threshold: a.Threshold, Baseline: a.Baseline,
		ResolvedValue: a.ResolvedValue, WindowMinutes: new(int(a.WindowMinutes)), Text: a.Text, FiredAt: a.FiredAt.UTC(),
		AnnotationId: a.AnnotationID}
	if a.ResolvedAt != nil {
		out.State, out.ResolvedAt = oas.MonitorAlertResolved, new(a.ResolvedAt.UTC())
	}
	if a.Kind == string(oas.MonitorRuleSilent) {
		out.Route = &a.Key
	}
	return out
}

// ruleFields are a rule's settings while they are checked.
type ruleFields struct {
	name                *string
	kind                string
	propertyID, checkID *uuid.UUID
	serviceID, depID    *uuid.UUID
	route, service      *string
	threshold           *float64
	window, minRequests int32
	recipients          []uuid.UUID
	enabled             bool
	prop                *store.AnalyticsProperty
	check               *store.MonitorCheck
	svc                 *store.MonitorService
	dep                 *store.MonitorDependency
}

func ruleFieldsOf(r store.MonitorAlertRule) ruleFields {
	return ruleFields{name: &r.Name, kind: r.Kind, propertyID: r.PropertyID, checkID: r.CheckID, serviceID: r.ServiceID, depID: r.DependencyID, route: r.Route, service: r.Service, threshold: r.Threshold,
		window: r.WindowMinutes, minRequests: r.MinRequests, recipients: r.Recipients, enabled: r.Enabled}
}

func (f *ruleFields) apply(b oas.MonitorAlertRuleUpdate) {
	if b.Name != nil {
		f.name = b.Name
	}
	if b.Threshold.IsSpecified() {
		f.threshold = nil
		if !b.Threshold.IsNull() {
			f.threshold = new(b.Threshold.MustGet())
		}
	}
	if b.WindowMinutes != nil {
		f.window = int32(*b.WindowMinutes)
	}
	if b.MinRequests != nil {
		f.minRequests = int32(*b.MinRequests)
	}
	if b.Recipients != nil {
		f.recipients = *b.Recipients
	}
	if b.Enabled != nil {
		f.enabled = *b.Enabled
	}
}

func thresholdText(kind string, t float64) string {
	v := strconv.FormatFloat(t, 'f', -1, 64)
	switch kind {
	case "error_rate", "client_error_rate":
		return v + "%"
	case "p95", "p99":
		return v + " ms"
	case "certificate":
		return v + " days"
	}
	return v
}

func defaultRuleName(f *ruleFields) string {
	subject := "all checks"
	if f.kind == "service_state" {
		subject = "all services"
	}
	switch {
	case f.dep != nil && f.svc != nil:
		subject = f.dep.Name + " of " + serviceLabel(*f.svc)
	case f.svc != nil:
		subject = serviceLabel(*f.svc)
	case f.prop != nil && f.route != nil:
		subject = f.prop.Name + " " + *f.route
	case f.prop != nil && f.service != nil:
		subject = f.prop.Name + " (" + *f.service + ")"
	case f.check != nil:
		subject = f.check.Name
	case f.prop != nil:
		subject = f.prop.Name
	}
	t := ""
	if f.threshold != nil {
		t = thresholdText(f.kind, *f.threshold)
	}
	var name string
	switch f.kind {
	case "error_rate":
		name = "Error rate above " + t + " on " + subject
	case "client_error_rate":
		name = "Client error rate above " + t + " on " + subject
	case "p95", "p99":
		name = f.kind + " above " + t + " on " + subject
	case "apdex":
		name = "Apdex below " + t + " on " + subject
	case "spike":
		name = "Traffic spike on " + subject
	case "drop":
		name = "Traffic drop on " + subject
	case "silent":
		name = "Silent routes on " + subject
	case "check_state":
		name = subject + " is down"
	case "certificate":
		name = "Certificate of " + subject + " expiring"
	case "service_state":
		name = subject + " is degraded or down"
	}
	return truncateRunes(name, 100)
}

// validateRule checks a rule whole; the property must be visible to p and the check must exist.
func (s *Server) validateRule(ctx context.Context, q *store.Queries, p principal, f *ruleFields) error {
	if !oas.MonitorAlertRuleKind(f.kind).Valid() {
		return errValidation("kind must be one of error_rate, client_error_rate, p95, p99, apdex, spike, drop, silent, check_state, certificate, service_state")
	}
	onChecks := slices.Contains(ownerRuleKinds, f.kind)
	if f.propertyID != nil {
		prop, err := visibleProperty(ctx, q, p, *f.propertyID, false)
		if err != nil {
			return err
		}
		if !onChecks && !isAPIProperty(prop) {
			return errAPIOnly
		}
		f.prop = &prop
	} else if !onChecks {
		return errValidation("a rule of kind " + f.kind + " needs property_id, an API property")
	}
	if f.kind == "service_state" {
		if f.checkID != nil {
			return errValidation("check_id belongs to check_state and certificate rules")
		}
		if f.serviceID != nil {
			svc, err := s.getService(ctx, q, p.workspaceID, *f.serviceID, false)
			if err != nil {
				return err
			}
			f.svc = &svc
		}
		if f.depID != nil {
			if f.svc == nil {
				return errValidation("dependency_id needs the service_id it belongs to")
			}
			d, err := q.GetMonitorDependency(ctx, store.GetMonitorDependencyParams{WorkspaceID: p.workspaceID, ID: *f.depID})
			if store.IsNotFound(err) || (err == nil && d.ServiceID != f.svc.ID) {
				return problem(http.StatusNotFound, "not_found", "no such dependency of the service")
			}
			if err != nil {
				return err
			}
			f.dep = &d
		}
	} else if f.serviceID != nil || f.depID != nil {
		return errValidation("service_id and dependency_id belong to service_state rules")
	}
	if onChecks {
		if f.route != nil || f.service != nil {
			return errValidation("check and service rules take no route or service")
		}
		if f.checkID != nil && f.kind != "service_state" {
			c, err := s.getCheck(ctx, q, p.workspaceID, *f.checkID, false)
			if err != nil {
				return err
			}
			f.check = &c
		}
	} else if f.checkID != nil {
		return errValidation("check_id belongs to check_state and certificate rules")
	}
	if f.route != nil {
		routes, err := q.ListMonitorRoutes(ctx, store.ListMonitorRoutesParams{WorkspaceID: p.workspaceID, PropertyID: f.prop.ID})
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(routes, func(r store.ListMonitorRoutesRow) bool { return r.Pattern == *f.route }) {
			return errValidation("route must be a route of the property, as listAnalyticsRoutes lists it")
		}
	}
	if f.service != nil {
		v, err := trimmed(*f.service, 1, 100, "service")
		if err != nil {
			return err
		}
		f.service = &v
	}
	t := f.threshold
	switch {
	case slices.Contains(metricRuleKinds, f.kind):
		if t == nil {
			return errValidation("a rule of kind " + f.kind + " needs a threshold")
		}
		switch f.kind {
		case "error_rate", "client_error_rate":
			if *t <= 0 || *t > 100 {
				return errValidation("threshold is a percent above 0, at most 100")
			}
		case "p95", "p99":
			if *t <= 0 || *t > 3_600_000 {
				return errValidation("threshold is a number of milliseconds above 0, at most 3,600,000")
			}
		case "apdex":
			if *t <= 0 || *t >= 1 {
				return errValidation("threshold is an Apdex score above 0 and below 1")
			}
		}
	case f.kind == "certificate":
		if t != nil && (*t < 1 || *t > 365 || *t != math.Trunc(*t)) {
			return errValidation("threshold is a number of days from 1 to 365")
		}
	default:
		if t != nil {
			return errValidation("a rule of kind " + f.kind + " takes no threshold")
		}
	}
	if f.window < 5 || f.window > 60 {
		return errValidation("window_minutes must be 5 to 60")
	}
	if f.minRequests < 1 || f.minRequests > 1_000_000 {
		return errValidation("min_requests must be 1 to 1,000,000")
	}
	if len(f.recipients) > maxRuleRecipients {
		return errValidation("a rule has at most 20 recipients")
	}
	slices.SortFunc(f.recipients, func(a, b uuid.UUID) int { return a.Compare(b) })
	f.recipients = slices.Compact(f.recipients)
	if len(f.recipients) > 0 {
		arg := store.ListMonitorRecipientsParams{WorkspaceID: p.workspaceID, MemberIds: f.recipients, Scoped: f.prop != nil}
		if f.prop != nil {
			arg.InboxID = f.prop.InboxID
		}
		found, err := q.ListMonitorRecipients(ctx, arg)
		if err != nil {
			return err
		}
		if len(found) != len(f.recipients) {
			return errValidation("recipients must be members of the workspace who see the rule's property")
		}
	}
	if f.name == nil {
		f.name = new(defaultRuleName(f))
	}
	name, err := trimmed(*f.name, 1, 100, "name")
	if err != nil {
		return err
	}
	f.name = &name
	return nil
}

func (s *Server) ruleVisible(ctx context.Context, q *store.Queries, p principal, r store.MonitorAlertRule) (bool, error) {
	if r.PropertyID == nil {
		return true, nil
	}
	prop, err := q.GetAnalyticsProperty(ctx, store.GetAnalyticsPropertyParams{WorkspaceID: p.workspaceID, ID: *r.PropertyID})
	if store.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return propertyVisible(ctx, q, p, prop)
}

func (s *Server) visibleRule(ctx context.Context, q *store.Queries, p principal, id uuid.UUID, lock bool) (store.MonitorAlertRule, error) {
	get := q.GetMonitorAlertRule
	if lock {
		get = func(ctx context.Context, arg store.GetMonitorAlertRuleParams) (store.MonitorAlertRule, error) {
			return q.LockMonitorAlertRule(ctx, store.LockMonitorAlertRuleParams(arg))
		}
	}
	r, err := get(ctx, store.GetMonitorAlertRuleParams{WorkspaceID: p.workspaceID, ID: id})
	if store.IsNotFound(err) {
		return r, errRuleGone
	}
	if err != nil {
		return r, err
	}
	ok, err := s.ruleVisible(ctx, q, p, r)
	if err != nil {
		return r, err
	}
	if !ok {
		return r, errRuleGone
	}
	return r, nil
}

func (s *Server) firingCounts(ctx context.Context, ws uuid.UUID) (map[uuid.UUID]int64, error) {
	rows, err := s.st.CountOpenMonitorAlerts(ctx, ws)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]int64{}
	for _, r := range rows {
		out[r.RuleID] = r.Firing
	}
	return out, nil
}

func (s *Server) ListMonitorAlertRules(ctx context.Context, req oas.ListMonitorAlertRulesRequestObject) (oas.ListMonitorAlertRulesResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	rows, err := s.st.ListMonitorAlertRules(ctx, store.ListMonitorAlertRulesParams{WorkspaceID: p.workspaceID, PropertyID: req.Params.PropertyId,
		CheckID: req.Params.CheckId, ServiceID: req.Params.ServiceId})
	if err != nil {
		return nil, err
	}
	firing, err := s.firingCounts(ctx, p.workspaceID)
	if err != nil {
		return nil, err
	}
	out := oas.ListMonitorAlertRules200JSONResponse{Items: []oas.MonitorAlertRule{}}
	for _, r := range rows {
		ok, err := s.ruleVisible(ctx, s.st.Queries, p, r)
		if err != nil {
			return nil, err
		}
		if ok {
			out.Items = append(out.Items, ruleBody(r, firing[r.ID]))
		}
	}
	return out, nil
}

func (s *Server) GetMonitorAlertRule(ctx context.Context, req oas.GetMonitorAlertRuleRequestObject) (oas.GetMonitorAlertRuleResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	r, err := s.visibleRule(ctx, s.st.Queries, p, req.RuleId, false)
	if err != nil {
		return nil, err
	}
	firing, err := s.firingCounts(ctx, p.workspaceID)
	if err != nil {
		return nil, err
	}
	return oas.GetMonitorAlertRule200JSONResponse(ruleBody(r, firing[r.ID])), nil
}

func (s *Server) CreateMonitorAlertRule(ctx context.Context, req oas.CreateMonitorAlertRuleRequestObject) (oas.CreateMonitorAlertRuleResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if err := requireAnalyticsManager(p); err != nil {
		return nil, err
	}
	b := req.Body
	f := ruleFields{name: b.Name, kind: string(b.Kind), propertyID: b.PropertyId, checkID: b.CheckId, serviceID: b.ServiceId, depID: b.DependencyId, route: b.Route,
		service: b.Service,
		window:  defaultRuleWindow, minRequests: defaultRuleMinRequests, recipients: []uuid.UUID{}, enabled: true}
	f.apply(oas.MonitorAlertRuleUpdate{Threshold: b.Threshold, WindowMinutes: b.WindowMinutes, MinRequests: b.MinRequests, Recipients: b.Recipients,
		Enabled: b.Enabled})
	var out store.MonitorAlertRule
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		if err := s.validateRule(ctx, q, p, &f); err != nil {
			return err
		}
		if _, err := q.LockWorkspace(ctx, p.workspaceID); err != nil {
			return err
		}
		n, err := q.CountMonitorAlertRules(ctx, p.workspaceID)
		if err != nil {
			return err
		}
		if n >= maxMonitorRules {
			return errRuleLimit
		}
		out, err = q.CreateMonitorAlertRule(ctx, store.CreateMonitorAlertRuleParams{
			ID: newID(), WorkspaceID: p.workspaceID, Name: *f.name, Kind: f.kind, PropertyID: f.propertyID, Route: f.route, Service: f.service,
			CheckID: f.checkID, ServiceID: f.serviceID, DependencyID: f.depID, Threshold: f.threshold, WindowMinutes: f.window, MinRequests: f.minRequests,
			Recipients: f.recipients, Enabled: f.enabled, CreatedBy: p.memberRef(), Now: s.now(),
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return oas.CreateMonitorAlertRule201JSONResponse(ruleBody(out, 0)), nil
}

func (s *Server) UpdateMonitorAlertRule(ctx context.Context, req oas.UpdateMonitorAlertRuleRequestObject) (oas.UpdateMonitorAlertRuleResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if err := requireAnalyticsManager(p); err != nil {
		return nil, err
	}
	var out store.MonitorAlertRule
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		cur, err := s.visibleRule(ctx, q, p, req.RuleId, true)
		if err != nil {
			return err
		}
		f := ruleFieldsOf(cur)
		f.apply(*req.Body)
		if err := s.validateRule(ctx, q, p, &f); err != nil {
			return err
		}
		now := s.now()
		out, err = q.UpdateMonitorAlertRule(ctx, store.UpdateMonitorAlertRuleParams{WorkspaceID: p.workspaceID, ID: cur.ID, Name: *f.name, Threshold: f.threshold,
			WindowMinutes: f.window, MinRequests: f.minRequests, Recipients: f.recipients, Enabled: f.enabled, Now: now})
		if err != nil {
			return err
		}
		if !out.Enabled {
			_, err = q.ResolveRuleAlertsQuietly(ctx, store.ResolveRuleAlertsQuietlyParams{WorkspaceID: p.workspaceID, RuleID: out.ID, ResolvedAt: &now})
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	firing, err := s.firingCounts(ctx, p.workspaceID)
	if err != nil {
		return nil, err
	}
	return oas.UpdateMonitorAlertRule200JSONResponse(ruleBody(out, firing[out.ID])), nil
}

func (s *Server) DeleteMonitorAlertRule(ctx context.Context, req oas.DeleteMonitorAlertRuleRequestObject) (oas.DeleteMonitorAlertRuleResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if err := requireAnalyticsManager(p); err != nil {
		return nil, err
	}
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		r, err := s.visibleRule(ctx, q, p, req.RuleId, true)
		if err != nil {
			return err
		}
		_, err = q.DeleteMonitorAlertRule(ctx, store.DeleteMonitorAlertRuleParams{WorkspaceID: p.workspaceID, ID: r.ID})
		return err
	})
	if err != nil {
		return nil, err
	}
	return oas.DeleteMonitorAlertRule204Response{}, nil
}

func (s *Server) ListMonitorAlerts(ctx context.Context, req oas.ListMonitorAlertsRequestObject) (oas.ListMonitorAlertsResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	limit := monitorAlertsDefault
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	if limit < 1 || limit > 100 {
		return nil, errValidation("limit must be 1 to 100")
	}
	arg := store.ListMonitorAlertsParams{WorkspaceID: p.workspaceID, AllProperties: p.seesAllInboxes(), PropertyIds: []uuid.UUID{},
		PropertyID: req.Params.PropertyId, CheckID: req.Params.CheckId, RuleID: req.Params.RuleId, ServiceID: req.Params.ServiceId, Before: req.Params.Before,
		Lim: int32(limit)}
	if req.Params.State != nil {
		if !req.Params.State.Valid() {
			return nil, errValidation("state must be firing or resolved")
		}
		arg.State = new(string(*req.Params.State))
	}
	if !arg.AllProperties {
		props, err := s.st.ListAnalyticsProperties(ctx, store.ListAnalyticsPropertiesParams{WorkspaceID: p.workspaceID, ViewerID: p.viewerID()})
		if err != nil {
			return nil, err
		}
		for _, pr := range props {
			arg.PropertyIds = append(arg.PropertyIds, pr.ID)
		}
	}
	rows, err := s.st.ListMonitorAlerts(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := oas.ListMonitorAlerts200JSONResponse{Items: make([]oas.MonitorAlert, len(rows))}
	for i, r := range rows {
		a := store.MonitorAlert{ID: r.ID, WorkspaceID: r.WorkspaceID, RuleID: r.RuleID, Key: r.Key, Kind: r.Kind, PropertyID: r.PropertyID, CheckID: r.CheckID,
			ServiceID: r.ServiceID, DependencyID: r.DependencyID,
			Value: r.Value, Threshold: r.Threshold, Baseline: r.Baseline, ResolvedValue: r.ResolvedValue, WindowMinutes: r.WindowMinutes, Text: r.Text,
			FiredAt: r.FiredAt, ResolvedAt: r.ResolvedAt, AnnotationID: r.AnnotationID, CreatedAt: r.CreatedAt}
		out.Items[i] = monitorAlertBody(a, store.MonitorAlertRule{Name: r.RuleName, Route: r.RuleRoute, Service: r.RuleService})
	}
	return out, nil
}

// ruleFinding is what one evaluation of a rule found for one key: holds when the alert should be
// open, clears when an open one may resolve (from Since on, a whole window after it fired).
type ruleFinding struct {
	key           string
	holds, clears bool
	value         float64
	baseline      *float64
	text          string
	checkID       *uuid.UUID
	serviceID     *uuid.UUID
	depID         *uuid.UUID
	since         time.Time
}

type ruleScope struct {
	prop     *store.AnalyticsProperty
	checks   map[uuid.UUID]store.MonitorCheck
	services map[uuid.UUID]store.MonitorService
	deps     map[uuid.UUID]store.MonitorDependency
}

func (s *Server) monitorQuery(prop store.AnalyticsProperty, start, end time.Time, metrics, dims []string, r store.MonitorAlertRule) monitor.Query {
	m := s.analytics.monitor
	q := monitor.Query{WorkspaceID: prop.WorkspaceID, PropertyID: prop.ID, Zone: monitorZone(prop.Timezone, prop.TimezoneHistory), Start: start, End: end,
		Metrics: metrics, Dimensions: dims, Now: s.now(), ApdexMS: int64(prop.ApdexMs),
		MinuteFrom: monitor.LocalDay(s.now(), time.UTC).AddDate(0, 0, 1-m.settings.MinuteDays)}
	if r.Route != nil {
		q.Filters = append(q.Filters, monitor.Filter{Dimension: "route", Op: "is", Values: []string{*r.Route}})
	}
	if r.Service != nil {
		q.Filters = append(q.Filters, monitor.Filter{Dimension: "service", Op: "is", Values: []string{*r.Service}})
	}
	return q
}

func ruleBad(kind string, v, t float64) bool {
	if kind == "apdex" {
		return v < t
	}
	return v > t
}

func ruleGood(kind string, v, t float64) bool {
	if kind == "apdex" {
		return v >= t+(1-t)*(1-alertMargin)
	}
	return v <= t*alertMargin
}

func metricText(kind string, v float64, window int32) string {
	switch kind {
	case "error_rate":
		return fmt.Sprintf("Error rate %s%% over %d minutes", strconv.FormatFloat(v, 'f', 2, 64), window)
	case "client_error_rate":
		return fmt.Sprintf("Client error rate %s%% over %d minutes", strconv.FormatFloat(v, 'f', 2, 64), window)
	case "p95", "p99":
		return fmt.Sprintf("%s %s ms over %d minutes", kind, strconv.FormatFloat(v, 'f', 0, 64), window)
	}
	return fmt.Sprintf("Apdex %s over %d minutes", strconv.FormatFloat(v, 'f', 2, 64), window)
}

// evaluateRule applies a rule to the minutes that closed and to its checks' states.
func (s *Server) evaluateRule(ctx context.Context, r store.MonitorAlertRule, open []store.MonitorAlert) ([]ruleFinding, ruleScope, error) {
	var scope ruleScope
	now := s.now()
	end := now.Add(-alertLag).Truncate(time.Minute)
	if r.PropertyID != nil {
		prop, err := s.st.GetAnalyticsProperty(ctx, store.GetAnalyticsPropertyParams{WorkspaceID: r.WorkspaceID, ID: *r.PropertyID})
		if store.IsNotFound(err) {
			return nil, scope, nil
		}
		if err != nil {
			return nil, scope, err
		}
		scope.prop = &prop
	}
	st := s.analytics.monitor.store
	switch {
	case slices.Contains(metricRuleKinds, r.Kind):
		if scope.prop == nil || !isAPIProperty(*scope.prop) || r.Threshold == nil {
			return nil, scope, nil
		}
		window := time.Duration(r.WindowMinutes) * time.Minute
		res, err := st.Query(ctx, s.monitorQuery(*scope.prop, end.Add(-window), end, []string{"requests", r.Kind}, nil, r))
		if err != nil {
			return nil, scope, err
		}
		v, n, t := res.Totals.Metric(r.Kind, 0), res.Totals.Requests, *r.Threshold
		enough := n >= int64(r.MinRequests)
		return []ruleFinding{{key: "", holds: enough && ruleBad(r.Kind, v, t), clears: !enough || ruleGood(r.Kind, v, t), value: v,
			text: metricText(r.Kind, v, r.WindowMinutes), since: end.Add(-window)}}, scope, nil
	case r.Kind == "spike" || r.Kind == "drop":
		if scope.prop == nil || !isAPIProperty(*scope.prop) {
			return nil, scope, nil
		}
		count := func(from, to time.Time) (float64, error) {
			res, err := st.Query(ctx, s.monitorQuery(*scope.prop, from, to, []string{"requests"}, nil, r))
			return float64(res.Totals.Requests), err
		}
		cur, err := count(end.Add(-trafficWindow), end)
		if err != nil {
			return nil, scope, err
		}
		base := 0.0
		for d := 1; d <= trafficBaselineDays; d++ {
			back := time.Duration(d) * 24 * time.Hour
			n, err := count(end.Add(-trafficWindow-back), end.Add(-back))
			if err != nil {
				return nil, scope, err
			}
			base += n
		}
		base /= trafficBaselineDays
		f := ruleFinding{key: "", value: cur, baseline: &base, since: end.Add(-trafficWindow), clears: base < trafficMinBaseline}
		if r.Kind == "spike" {
			f.holds = base >= trafficMinBaseline && cur >= spikeFactor*base
			f.clears = f.clears || cur < spikeFactor*base*alertMargin
			f.text = fmt.Sprintf("%s requests in 15 minutes against %s on average in the same minutes of the 7 days before", num(cur), num(base))
		} else {
			f.holds = base >= trafficMinBaseline && cur <= dropShare*base
			f.clears = f.clears || cur > dropShare*base*(2-alertMargin)
			f.text = fmt.Sprintf("%s requests in 15 minutes, down from %s on average in the same minutes of the 7 days before", num(cur), num(base))
		}
		return []ruleFinding{f}, scope, nil
	case r.Kind == "silent":
		if scope.prop == nil || !isAPIProperty(*scope.prop) {
			return nil, scope, nil
		}
		byRoute := func(from, to time.Time) (map[string]float64, error) {
			res, err := st.Query(ctx, s.monitorQuery(*scope.prop, from, to, []string{"requests"}, []string{"route"}, r))
			out := map[string]float64{}
			for _, row := range res.Rows {
				out[row.Dims[0]] = float64(row.Agg.Requests)
			}
			return out, err
		}
		before, err := byRoute(end.Add(-trafficWindow-24*time.Hour), end.Add(-trafficWindow))
		if err != nil {
			return nil, scope, err
		}
		recent, err := byRoute(end.Add(-trafficWindow), end)
		if err != nil {
			return nil, scope, err
		}
		keys := []string{}
		for route, n := range before {
			if n >= silentPerMinute*24*60 && route != monitor.Unmatched && route != monitor.OtherValue {
				keys = append(keys, route)
			}
		}
		for _, o := range open {
			if !slices.Contains(keys, o.Key) {
				keys = append(keys, o.Key)
			}
		}
		slices.Sort(keys)
		var out []ruleFinding
		for _, route := range keys {
			perMinute := before[route] / (24 * 60)
			qualifies := before[route] >= silentPerMinute*24*60
			out = append(out, ruleFinding{key: route, holds: qualifies && recent[route] == 0, clears: recent[route] > 0, value: recent[route], baseline: &perMinute,
				text: fmt.Sprintf("Route %s had no requests for 15 minutes after %s a minute over the day before", route, strconv.FormatFloat(perMinute, 'f', 1, 64))})
		}
		return out, scope, nil
	case r.Kind == "service_state":
		return s.evaluateServiceRule(ctx, r, scope)
	}
	mt, err := s.activeMaintenance(ctx, s.st.Queries, r.WorkspaceID, now)
	if err != nil {
		return nil, scope, err
	}
	var checks []store.MonitorCheck
	if r.CheckID != nil {
		c, err := s.st.GetMonitorCheck(ctx, store.GetMonitorCheckParams{WorkspaceID: r.WorkspaceID, ID: *r.CheckID})
		if err != nil && !store.IsNotFound(err) {
			return nil, scope, err
		}
		if err == nil {
			checks = append(checks, c)
		}
	} else {
		var err error
		if checks, err = s.st.ListMonitorChecks(ctx, store.ListMonitorChecksParams{WorkspaceID: r.WorkspaceID, PropertyID: r.PropertyID}); err != nil {
			return nil, scope, err
		}
	}
	scope.checks = map[uuid.UUID]store.MonitorCheck{}
	var out []ruleFinding
	for _, c := range checks {
		scope.checks[c.ID] = c
		f := ruleFinding{key: c.ID.String(), checkID: &c.ID}
		if r.Kind == "check_state" {
			down := c.State == resultDown && !c.Paused
			f.holds, f.clears = down && !mt.check(c.ID), !down && (c.State == resultUp || c.Paused)
			if down {
				f.value = 1
			}
			f.text = "Check " + c.Name + " is down"
		} else {
			if c.CertExpiresAt == nil || !strings.HasPrefix(c.Url, "https://") {
				f.clears = true
			} else {
				days := math.Floor(c.CertExpiresAt.Sub(now).Hours() / 24)
				t := float64(c.CertWarningDays)
				if r.Threshold != nil {
					t = *r.Threshold
				}
				f.value, f.holds, f.clears = days, days <= t, days > t
				f.text = fmt.Sprintf("The certificate of %s expires in %s days", c.Name, num(days))
			}
		}
		out = append(out, f)
	}
	return out, scope, nil
}

// CheckMonitorAlerts applies every enabled rule of every workspace; it returns how many alerts
// fired or resolved.
func (s *Server) CheckMonitorAlerts(ctx context.Context) (int, error) {
	if _, err := s.monitorOn(); err != nil {
		return 0, nil
	}
	ids, err := s.st.ListMonitorAlertWorkspaces(ctx)
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, ws := range ids {
		rules, err := s.st.ListEnabledMonitorAlertRules(ctx, ws)
		if err != nil {
			return changed, err
		}
		for _, r := range rules {
			n, err := s.applyRule(ctx, r)
			if err != nil {
				s.log.WarnContext(ctx, "monitor alerts", slog.String("workspace_id", ws.String()), slog.String("rule_id", r.ID.String()), slog.Any("error", err))
				continue
			}
			changed += n
		}
	}
	return changed, nil
}

// checkRulesOn applies the check rules that cover c right after its state changed.
func (s *Server) checkRulesOn(ctx context.Context, c store.MonitorCheck) error {
	rules, err := s.st.ListEnabledMonitorAlertRules(ctx, c.WorkspaceID)
	if err != nil {
		return err
	}
	for _, r := range rules {
		if r.Kind != "check_state" {
			continue
		}
		if (r.CheckID != nil && *r.CheckID != c.ID) || (r.CheckID == nil && r.PropertyID != nil && (c.PropertyID == nil || *c.PropertyID != *r.PropertyID)) {
			continue
		}
		if _, err := s.applyRule(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

// applyRule fires the alerts whose condition holds and resolves the open ones that cleared, or
// whose key the rule no longer covers; each happens once, whichever process gets there first.
func (s *Server) applyRule(ctx context.Context, r store.MonitorAlertRule) (int, error) {
	open, err := s.st.ListOpenMonitorAlerts(ctx, store.ListOpenMonitorAlertsParams{WorkspaceID: r.WorkspaceID, RuleID: r.ID})
	if err != nil {
		return 0, err
	}
	findings, scope, err := s.evaluateRule(ctx, r, open)
	if err != nil {
		return 0, err
	}
	byKey := map[string]store.MonitorAlert{}
	for _, o := range open {
		byKey[o.Key] = o
	}
	now, changed := s.now(), 0
	seen := map[string]bool{}
	for _, f := range findings {
		seen[f.key] = true
		o, isOpen := byKey[f.key]
		switch {
		case isOpen && f.clears && (f.since.IsZero() || !f.since.Before(o.FiredAt)):
			ok, err := s.resolveMonitorAlert(ctx, r, o, &f.value, f, scope, now)
			if err != nil {
				return changed, err
			}
			if ok {
				changed++
			}
		case !isOpen && f.holds:
			ok, err := s.fireMonitorAlert(ctx, r, f, scope, now)
			if err != nil {
				return changed, err
			}
			if ok {
				changed++
			}
		}
	}
	for _, o := range open {
		if seen[o.Key] {
			continue
		}
		ok, err := s.resolveMonitorAlert(ctx, r, o, nil, ruleFinding{key: o.Key, text: "No longer covered by the rule"}, scope, now)
		if err != nil {
			return changed, err
		}
		if ok {
			changed++
		}
	}
	return changed, nil
}

func (s *Server) alertEventData(a store.MonitorAlert, r store.MonitorAlertRule, scope ruleScope) oas.MonitorAlertEventData {
	d := oas.MonitorAlertEventData{Alert: monitorAlertBody(a, r)}
	if scope.prop != nil {
		d.Property = &oas.AnalyticsPropertyRef{Id: scope.prop.ID, Name: scope.prop.Name}
	}
	if a.CheckID != nil {
		if c, ok := scope.checks[*a.CheckID]; ok {
			d.Check = &oas.MonitorCheckRef{Id: c.ID, Name: c.Name, Url: c.Url}
		}
	}
	if a.ServiceID != nil {
		if svc, ok := scope.services[*a.ServiceID]; ok {
			d.Service = &oas.MonitorServiceRef{Id: svc.ID, Name: svc.Name, Environment: svc.Environment}
		}
	}
	return d
}

func (s *Server) alertAnnotation(ctx context.Context, q *store.Queries, a store.MonitorAlert, scope ruleScope, text string, at time.Time, service *string) (*uuid.UUID, error) {
	prop := scope.prop
	if c, ok := scope.checks[derefID(a.CheckID)]; prop == nil && ok && c.PropertyID != nil {
		p, err := q.GetAnalyticsProperty(ctx, store.GetAnalyticsPropertyParams{WorkspaceID: a.WorkspaceID, ID: *c.PropertyID})
		if err != nil && !store.IsNotFound(err) {
			return nil, err
		}
		if err == nil {
			prop = &p
		}
	}
	if svc, ok := scope.services[derefID(a.ServiceID)]; prop == nil && ok && svc.PropertyID != nil {
		p, err := q.GetAnalyticsProperty(ctx, store.GetAnalyticsPropertyParams{WorkspaceID: a.WorkspaceID, ID: *svc.PropertyID})
		if err != nil && !store.IsNotFound(err) {
			return nil, err
		}
		if err == nil {
			prop = &p
		}
	}
	if prop == nil {
		return nil, nil
	}
	day := monitorZone(prop.Timezone, prop.TimezoneHistory).LocalDay(at)
	ann, err := q.CreateMonitorAlertAnnotation(ctx, store.CreateMonitorAlertAnnotationParams{ID: newID(), WorkspaceID: a.WorkspaceID, PropertyID: prop.ID,
		Day: day, Text: analytics.Clean(text, 300), AlertID: &a.ID, At: &at, Service: service, Now: s.now()})
	if err != nil {
		return nil, err
	}
	return &ann.ID, nil
}

func (s *Server) fireMonitorAlert(ctx context.Context, r store.MonitorAlertRule, f ruleFinding, scope ruleScope, now time.Time) (bool, error) {
	end := now.Add(-alertLag).Truncate(time.Minute)
	text := f.text
	if r.Threshold != nil && slices.Contains(metricRuleKinds, r.Kind) {
		text += ", past " + thresholdText(r.Kind, *r.Threshold)
	}
	var a store.MonitorAlert
	fired := false
	err := s.inTx(ctx, r.WorkspaceID, func(q *store.Queries, events *eventBatch) error {
		var err error
		a, err = q.OpenMonitorAlert(ctx, store.OpenMonitorAlertParams{ID: newID(), WorkspaceID: r.WorkspaceID, RuleID: r.ID, Key: f.key, Kind: r.Kind,
			PropertyID: r.PropertyID, CheckID: f.checkID, ServiceID: f.serviceID, DependencyID: f.depID, Value: f.value, Threshold: r.Threshold, Baseline: f.baseline, WindowMinutes: r.WindowMinutes,
			Text: analytics.Clean(text, 300), FiredAt: end, Now: now})
		if store.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		ann, err := s.alertAnnotation(ctx, q, a, scope, a.Text, now, r.Service)
		if err != nil {
			return err
		}
		if ann != nil {
			if a, err = q.SetMonitorAlertAnnotation(ctx, store.SetMonitorAlertAnnotationParams{WorkspaceID: r.WorkspaceID, ID: a.ID, AnnotationID: ann}); err != nil {
				return err
			}
		}
		events.add(eventMonitorAlert, alertInbox(scope), nil, s.alertEventData(a, r, scope))
		fired = true
		return nil
	})
	if err != nil || !fired {
		return false, err
	}
	s.deliverMonitorAlert(ctx, r, a, scope)
	return true, nil
}

func (s *Server) resolveMonitorAlert(ctx context.Context, r store.MonitorAlertRule, o store.MonitorAlert, value *float64, f ruleFinding, scope ruleScope, now time.Time) (bool, error) {
	var a store.MonitorAlert
	resolved := false
	err := s.inTx(ctx, r.WorkspaceID, func(q *store.Queries, events *eventBatch) error {
		var err error
		a, err = q.ResolveMonitorAlert(ctx, store.ResolveMonitorAlertParams{WorkspaceID: r.WorkspaceID, ID: o.ID, ResolvedAt: &now, ResolvedValue: value})
		if store.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		text := "Resolved: " + o.Text
		if value != nil && f.text != "" && slices.Contains(metricRuleKinds, r.Kind) {
			text = "Resolved: " + f.text
		}
		if _, err := s.alertAnnotation(ctx, q, a, scope, text, now, r.Service); err != nil {
			return err
		}
		events.add(eventMonitorAlert, alertInbox(scope), nil, s.alertEventData(a, r, scope))
		resolved = true
		return nil
	})
	if err != nil || !resolved {
		return false, err
	}
	s.deliverMonitorAlert(ctx, r, a, scope)
	return true, nil
}

func alertInbox(scope ruleScope) *uuid.UUID {
	if scope.prop == nil {
		return nil
	}
	return scope.prop.InboxID
}

// alertPaths are where a member opens an alert: the property's alerts, or the check.
func (s *Server) alertPaths(a store.MonitorAlert, scope ruleScope) (string, string) {
	if scope.prop != nil {
		path := "/analytics/" + scope.prop.ID.String() + "/alerts"
		return path + "?workspace_id=" + a.WorkspaceID.String(), s.panelPropertyURL(a.WorkspaceID, scope.prop.ID, "alerts")
	}
	path := "/monitoring/checks"
	if a.ServiceID != nil {
		path = "/monitoring/services/" + a.ServiceID.String()
	} else if a.CheckID != nil {
		path += "/" + a.CheckID.String()
	}
	return path + "?workspace_id=" + a.WorkspaceID.String(), s.auth.PublicURL + path + "?workspace=" + a.WorkspaceID.String()
}

func (s *Server) monitorAlertMail(r store.MonitorAlertRule, a store.MonitorAlert, scope ruleScope, url, locale string) map[string]any {
	d := map[string]any{
		"Kind": a.Kind, "Resolved": a.ResolvedAt != nil, "Rule": r.Name, "Window": strconv.Itoa(int(a.WindowMinutes)), "URL": url, "SettingsURL": url,
		"Value": reportDecimal(a.Value, locale), "Route": deref(r.Route), "Service": deref(r.Service), "Days": reportNumber(a.Value, locale),
		"Subject": "", "Check": "", "ServiceName": "", "Dependency": "", "State": "",
	}
	switch a.Kind {
	case "error_rate", "client_error_rate":
		d["Value"] = reportPercentDecimal(a.Value, locale)
	case "p95", "p99":
		d["Value"] = reportNumber(a.Value, locale)
	case "apdex":
		d["Value"] = reportFixed(a.Value, 2, locale)
	case "spike", "drop":
		d["Value"] = reportNumber(a.Value, locale)
	case "silent":
		d["Route"] = a.Key
	}
	if a.ResolvedValue != nil && slices.Contains(metricRuleKinds, a.Kind) {
		switch a.Kind {
		case "error_rate", "client_error_rate":
			d["Now"] = reportPercentDecimal(*a.ResolvedValue, locale)
		case "apdex":
			d["Now"] = reportFixed(*a.ResolvedValue, 2, locale)
		default:
			d["Now"] = reportNumber(*a.ResolvedValue, locale)
		}
	}
	if a.Threshold != nil {
		switch a.Kind {
		case "error_rate", "client_error_rate":
			d["Threshold"] = reportPercentDecimal(*a.Threshold, locale)
		case "apdex":
			d["Threshold"] = reportFixed(*a.Threshold, 2, locale)
		default:
			d["Threshold"] = reportNumber(*a.Threshold, locale)
		}
	}
	if a.Baseline != nil {
		d["Baseline"] = reportNumber(*a.Baseline, locale)
		if a.Kind == "silent" {
			d["Baseline"] = reportFixed(*a.Baseline, 1, locale)
		}
	}
	if scope.prop != nil {
		d["Subject"] = scope.prop.Name
	}
	if a.CheckID != nil {
		if c, ok := scope.checks[*a.CheckID]; ok {
			d["Check"], d["Subject"] = c.Name, c.Name
		}
	}
	if a.ServiceID != nil {
		if svc, ok := scope.services[*a.ServiceID]; ok {
			d["ServiceName"], d["Subject"] = serviceLabel(svc), serviceLabel(svc)
		}
		d["State"] = stateDegraded
		if a.Value >= 2 {
			d["State"] = stateDown
		}
	}
	if a.DependencyID != nil {
		if dep, ok := scope.deps[*a.DependencyID]; ok {
			d["Dependency"] = dep.Name
		}
	}
	return d
}

// deliverMonitorAlert mails and pushes an alert that fired or resolved to the rule's recipients who
// still see what it is about; push is urgent, replaces the alert's earlier push and lives an hour.
func (s *Server) deliverMonitorAlert(ctx context.Context, r store.MonitorAlertRule, a store.MonitorAlert, scope ruleScope) {
	if len(r.Recipients) == 0 {
		return
	}
	arg := store.ListMonitorRecipientsParams{WorkspaceID: r.WorkspaceID, MemberIds: r.Recipients, Scoped: scope.prop != nil}
	if scope.prop != nil {
		arg.InboxID = scope.prop.InboxID
	}
	recips, err := s.st.ListMonitorRecipients(ctx, arg)
	if err != nil {
		s.log.WarnContext(ctx, "monitor alert recipients", slog.String("workspace_id", r.WorkspaceID.String()), slog.Any("error", err))
		return
	}
	pushPath, mailURL := s.alertPaths(a, scope)
	var batch []river.InsertManyParams
	for _, rc := range recips {
		data := s.monitorAlertMail(r, a, scope, mailURL, rc.Locale)
		msg, err := mail.Render("monitor_alert", rc.Locale, data)
		if err != nil {
			s.log.WarnContext(ctx, "monitor alert mail", slog.Any("error", err))
			continue
		}
		msg.To = rc.Email
		if err := s.mailer.Send(ctx, msg); err != nil {
			s.log.WarnContext(ctx, "monitor alert mail", slog.String("workspace_id", r.WorkspaceID.String()), slog.Any("error", err))
		}
		if s.push == nil {
			continue
		}
		subs, err := s.st.ListLivePushSubscriptionIDs(ctx, store.ListLivePushSubscriptionIDsParams{PersonID: rc.PersonID, Now: s.now()})
		if err != nil {
			s.log.WarnContext(ctx, "monitor alert push", slog.Any("error", err))
			continue
		}
		pm, err := mail.Render("push_monitor_alert", rc.Locale, data)
		if err != nil {
			s.log.WarnContext(ctx, "monitor alert push", slog.Any("error", err))
			continue
		}
		ws := r.WorkspaceID
		payload := oas.PushNotification{Event: oas.PushMonitorAlert, Title: pm.Subject, Body: strings.TrimSpace(pm.Text), Url: pushPath,
			Tag: "monitor-alert-" + a.ID.String(), WorkspaceId: &ws, InboxId: alertInbox(scope)}
		for _, id := range subs {
			batch = append(batch, river.InsertManyParams{Args: PushArgs{WorkspaceID: &ws, SubscriptionID: id, Payload: mustJSON(payload),
				TTL: int(monitorAlertPushTTL / time.Second), Urgency: "high", Topic: strings.ReplaceAll(a.ID.String(), "-", "")}, InsertOpts: pushOpts()})
		}
	}
	if len(batch) > 0 {
		if _, err := s.jobs.InsertMany(ctx, batch); err != nil {
			s.log.WarnContext(ctx, "monitor alert push", slog.Any("error", err))
		}
	}
}

func reportFixed(v float64, digits int, locale string) string {
	s := strconv.FormatFloat(v, 'f', digits, 64)
	if l := mailLocale(locale); l == "tr" || l == "de" {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s
}

func reportPercentDecimal(v float64, locale string) string {
	n := reportFixed(v, 1, locale)
	switch mailLocale(locale) {
	case "tr":
		return "%" + n
	case "de":
		return n + " %"
	}
	return n + "%"
}

// defaultAPIRule is the rule a new API property gets: 5xx rate above 5% over 5 minutes.
func defaultAPIRule(ws, prop uuid.UUID, createdBy *uuid.UUID, now time.Time) store.CreateMonitorAlertRuleParams {
	recipients := []uuid.UUID{}
	if createdBy != nil {
		recipients = append(recipients, *createdBy)
	}
	return store.CreateMonitorAlertRuleParams{ID: newID(), WorkspaceID: ws, Name: "5xx rate above 5%", Kind: string(oas.MonitorRuleErrorRate), PropertyID: &prop,
		Threshold: new(float64(default5xxThreshold)), WindowMinutes: defaultRuleWindow, MinRequests: defaultRuleMinRequests, Recipients: recipients,
		Enabled: true, CreatedBy: createdBy, Now: now}
}

// evaluateServiceRule finds which services (or the rule's one dependency) are degraded or down;
// none fires inside an active maintenance window.
func (s *Server) evaluateServiceRule(ctx context.Context, r store.MonitorAlertRule, scope ruleScope) ([]ruleFinding, ruleScope, error) {
	var svcs []store.MonitorService
	if r.ServiceID != nil {
		svc, err := s.st.GetMonitorService(ctx, store.GetMonitorServiceParams{WorkspaceID: r.WorkspaceID, ID: *r.ServiceID})
		if err != nil && !store.IsNotFound(err) {
			return nil, scope, err
		}
		if err == nil {
			svcs = append(svcs, svc)
		}
	} else {
		var err error
		if svcs, err = s.st.ListMonitorServices(ctx, store.ListMonitorServicesParams{WorkspaceID: r.WorkspaceID, PropertyID: r.PropertyID}); err != nil {
			return nil, scope, err
		}
	}
	mt, err := s.activeMaintenance(ctx, s.st.Queries, r.WorkspaceID, s.now())
	if err != nil {
		return nil, scope, err
	}
	scope.services, scope.deps = map[uuid.UUID]store.MonitorService{}, map[uuid.UUID]store.MonitorDependency{}
	var out []ruleFinding
	for _, svc := range svcs {
		scope.services[svc.ID] = svc
		f := ruleFinding{key: svc.ID.String(), serviceID: &svc.ID}
		state, held := svc.State, mt.service(svc.ID)
		text := "Service " + serviceLabel(svc) + " is " + state
		if svc.Reason != nil && *svc.Reason != "" {
			text += ": " + *svc.Reason
		}
		if r.DependencyID != nil {
			d, err := s.st.GetMonitorDependency(ctx, store.GetMonitorDependencyParams{WorkspaceID: r.WorkspaceID, ID: *r.DependencyID})
			if store.IsNotFound(err) {
				continue
			}
			if err != nil {
				return nil, scope, err
			}
			scope.deps[d.ID] = d
			f.key, f.depID = d.ID.String(), &d.ID
			state, held = d.State, mt.dependency(d)
			text = d.Name + " of " + serviceLabel(svc) + " is " + state
			if d.Message != nil && *d.Message != "" {
				text += ": " + *d.Message
			}
		}
		bad := state == stateDegraded || state == stateDown
		f.holds, f.clears, f.value, f.text = bad && !held, !bad, stateValue(state), text
		out = append(out, f)
	}
	return out, scope, nil
}

// serviceRulesOn applies the service_state rules that cover svc right after it or one of its
// dependencies changed state.
func (s *Server) serviceRulesOn(ctx context.Context, svc store.MonitorService) error {
	rules, err := s.st.ListEnabledMonitorAlertRules(ctx, svc.WorkspaceID)
	if err != nil {
		return err
	}
	for _, r := range rules {
		if r.Kind != "service_state" {
			continue
		}
		if (r.ServiceID != nil && *r.ServiceID != svc.ID) || (r.ServiceID == nil && r.PropertyID != nil && (svc.PropertyID == nil || *svc.PropertyID != *r.PropertyID)) {
			continue
		}
		if _, err := s.applyRule(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

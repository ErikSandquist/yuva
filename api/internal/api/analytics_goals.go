package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	maxGoalsPerProperty       = 100
	maxFunnelsPerProperty     = 20
	maxSegmentsPerProperty    = 100
	maxAnnotationsPerProperty = 1000
	maxFunnelDays             = 90
	maxBusyFunnelDays         = 31
	busyPropertyEvents        = 10_000_000
	analyticsFunnelTTL        = 10 * time.Minute

	annotationNote        = "note"
	annotationKeyRotated  = "key_rotated"
	annotationGoalCreated = "goal_created"
)

var (
	errGoalGone          = problem(http.StatusNotFound, "not_found", "no such goal")
	errFunnelGone        = problem(http.StatusNotFound, "not_found", "no such funnel")
	errSegmentGone       = problem(http.StatusNotFound, "not_found", "no such segment")
	errAnnotationGone    = problem(http.StatusNotFound, "not_found", "no such annotation")
	errGoalTaken         = problem(http.StatusConflict, "goal_taken", "another goal of the property has this name")
	errFunnelTaken       = problem(http.StatusConflict, "funnel_taken", "another funnel of the property has this name")
	errSegmentTaken      = problem(http.StatusConflict, "segment_taken", "another segment of the property has this name")
	errGoalLimit         = problem(http.StatusForbidden, "goal_limit", "a property has at most 100 goals")
	errFunnelLimit       = problem(http.StatusForbidden, "funnel_limit", "a property has at most 20 funnels")
	errSegmentLimit      = problem(http.StatusForbidden, "segment_limit", "a property has at most 100 segments")
	errAnnotationLimit   = problem(http.StatusForbidden, "annotation_limit", "a property has at most 1,000 annotations")
	errAnnotationSystem  = problem(http.StatusForbidden, "annotation_read_only", "Yuva's own annotations cannot be changed")
	errFunnelStepsBroken = problem(http.StatusConflict, "funnel_steps_missing", "a step of this funnel is no longer a goal of the property")
)

// mayEditShared: a segment or annotation is changed by its author, owners, admins and API keys
// without an inbox limit.
func mayEditShared(p principal, createdBy *uuid.UUID) bool {
	if p.isKey() {
		return !p.limited
	}
	if p.role == roleOwner || p.role == roleAdmin {
		return true
	}
	return createdBy != nil && *createdBy == p.memberID
}

func goalDef(g store.AnalyticsGoal) *analytics.Goal {
	out := &analytics.Goal{ID: g.ID, Kind: g.Kind, Path: orZero(g.PathPattern), Event: orZero(g.EventName), PropKey: orZero(g.PropKey),
		PropValue: orZero(g.PropValue), Version: g.Version}
	if g.ScrollPct != nil {
		out.Scroll = *g.ScrollPct
	}
	return out
}

func orZero[T any](v *T) T {
	var zero T
	if v == nil {
		return zero
	}
	return *v
}

func goalBody(g store.AnalyticsGoal) oas.AnalyticsGoal {
	out := oas.AnalyticsGoal{Id: g.ID, Name: g.Name, Kind: oas.AnalyticsGoalKind(g.Kind), PathPattern: g.PathPattern, EventName: g.EventName,
		PropKey: g.PropKey, PropValue: g.PropValue, CountsFrom: openapi_types.Date{Time: g.CountsFrom}, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt}
	if g.ScrollPct != nil {
		out.ScrollPct = new(int(*g.ScrollPct))
	}
	return out
}

// goalFields are a goal's definition while it is checked; fields its kind does not use are dropped.
type goalFields struct {
	name, kind                      string
	path, event, propKey, propValue *string
	scroll                          *int16
}

func (f *goalFields) definition() string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%d", f.kind, orZero(f.path), orZero(f.event), orZero(f.propKey), orZero(f.propValue), orZero(f.scroll))
}

func (f *goalFields) validate() error {
	var err error
	if f.name, err = trimmed(f.name, 1, 120, "name"); err != nil {
		return err
	}
	switch f.kind {
	case analytics.GoalPageview, analytics.GoalScroll:
		if f.path == nil {
			return errValidation("a " + f.kind + " goal needs path_pattern")
		}
		p, err := validPathPattern(*f.path)
		if err != nil {
			return err
		}
		f.path, f.event, f.propKey, f.propValue = &p, nil, nil, nil
		if f.kind == analytics.GoalPageview {
			f.scroll = nil
		} else if f.scroll == nil || *f.scroll < 1 || *f.scroll > 100 {
			return errValidation("a scroll goal needs scroll_pct from 1 to 100")
		}
	case analytics.GoalEvent:
		if f.event == nil {
			return errValidation("an event goal needs event_name")
		}
		name, err := analytics.EventName(*f.event)
		if err != nil {
			return errValidation(strings.Replace(err.Error(), "n ", "event_name ", 1))
		}
		f.event, f.path, f.scroll = &name, nil, nil
		if (f.propKey == nil) != (f.propValue == nil) {
			return errValidation("prop_key and prop_value go together")
		}
		if f.propKey != nil {
			k, err := trimmed(*f.propKey, 1, 300, "prop_key")
			if err != nil {
				return err
			}
			if analytics.ReservedKey(k) {
				return errValidation("prop keys starting with $ or yuva_ are reserved")
			}
			v := analytics.Clean(*f.propValue, analytics.MaxPropValue)
			f.propKey, f.propValue = &k, &v
		}
	default:
		return errValidation("kind must be pageview, event or scroll")
	}
	return nil
}

func scrollOf(v *int) *int16 {
	if v == nil {
		return nil
	}
	if *v < 0 || *v > 100 {
		return new(int16(-1))
	}
	return new(int16(*v))
}

func (s *Server) ListAnalyticsGoals(ctx context.Context, req oas.ListAnalyticsGoalsRequestObject) (oas.ListAnalyticsGoalsResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	prop, err := visibleProperty(ctx, s.st.Queries, p, req.PropertyId, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.st.ListAnalyticsGoals(ctx, store.ListAnalyticsGoalsParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID})
	if err != nil {
		return nil, err
	}
	out := oas.ListAnalyticsGoals200JSONResponse{Items: make([]oas.AnalyticsGoal, len(rows))}
	for i, g := range rows {
		out.Items[i] = goalBody(g)
	}
	return out, nil
}

func (s *Server) CreateAnalyticsGoal(ctx context.Context, req oas.CreateAnalyticsGoalRequestObject) (oas.CreateAnalyticsGoalResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	b := req.Body
	f := goalFields{name: b.Name, kind: string(b.Kind), path: b.PathPattern, event: b.EventName, propKey: b.PropKey, propValue: b.PropValue, scroll: scrollOf(b.ScrollPct)}
	var out store.AnalyticsGoal
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		prop, err := visibleProperty(ctx, q, p, req.PropertyId, true)
		if err != nil {
			return err
		}
		if err := requireAnalyticsManager(p); err != nil {
			return err
		}
		if err := f.validate(); err != nil {
			return err
		}
		n, err := q.CountAnalyticsGoals(ctx, store.CountAnalyticsGoalsParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID})
		if err != nil {
			return err
		}
		if n >= maxGoalsPerProperty {
			return errGoalLimit
		}
		version, err := q.BumpAnalyticsGoalsVersion(ctx, store.BumpAnalyticsGoalsVersionParams{WorkspaceID: p.workspaceID, ID: prop.ID})
		if err != nil {
			return err
		}
		now := s.now()
		out, err = q.CreateAnalyticsGoal(ctx, store.CreateAnalyticsGoalParams{
			ID: newID(), WorkspaceID: p.workspaceID, PropertyID: prop.ID, Name: f.name, Kind: f.kind, PathPattern: f.path, EventName: f.event,
			PropKey: f.propKey, PropValue: f.propValue, ScrollPct: f.scroll, Version: version, CountsFrom: s.rawFrom(), CreatedBy: p.memberRef(), Now: now,
		})
		if err != nil {
			return err
		}
		today := analytics.LocalDay(now, propertyLocation(prop.Timezone))
		_, err = q.CreateAnalyticsAnnotation(ctx, store.CreateAnalyticsAnnotationParams{
			ID: newID(), WorkspaceID: p.workspaceID, PropertyID: prop.ID, Kind: annotationGoalCreated, StartsOn: today, EndsOn: today,
			Text: out.Name, GoalID: &out.ID, Now: now,
		})
		return err
	})
	if store.IsUniqueViolation(err) {
		return nil, errGoalTaken
	}
	if err != nil {
		return nil, err
	}
	s.analytics.answers.forget(out.PropertyID)
	return oas.CreateAnalyticsGoal201JSONResponse(goalBody(out)), nil
}

func (s *Server) UpdateAnalyticsGoal(ctx context.Context, req oas.UpdateAnalyticsGoalRequestObject) (oas.UpdateAnalyticsGoalResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	b := req.Body
	var out store.AnalyticsGoal
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		prop, err := visibleProperty(ctx, q, p, req.PropertyId, true)
		if err != nil {
			return err
		}
		if err := requireAnalyticsManager(p); err != nil {
			return err
		}
		cur, err := q.GetAnalyticsGoal(ctx, store.GetAnalyticsGoalParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: req.GoalId})
		if store.IsNotFound(err) {
			return errGoalGone
		}
		if err != nil {
			return err
		}
		f := goalFields{name: cur.Name, kind: cur.Kind, path: cur.PathPattern, event: cur.EventName, propKey: cur.PropKey, propValue: cur.PropValue, scroll: cur.ScrollPct}
		before := f.definition()
		if b.Name != nil {
			f.name = *b.Name
		}
		if b.Kind != nil {
			f.kind = string(*b.Kind)
		}
		if b.PathPattern != nil {
			f.path = b.PathPattern
		}
		if b.EventName != nil {
			f.event = b.EventName
		}
		if b.PropKey.IsSpecified() {
			f.propKey, f.propValue = nil, nil
			if !b.PropKey.IsNull() {
				f.propKey = new(b.PropKey.MustGet())
			}
		}
		if b.PropValue != nil {
			f.propValue = b.PropValue
		}
		if b.ScrollPct != nil {
			f.scroll = scrollOf(b.ScrollPct)
		}
		if err := f.validate(); err != nil {
			return err
		}
		version := cur.Version
		if f.definition() != before {
			if version, err = q.BumpAnalyticsGoalsVersion(ctx, store.BumpAnalyticsGoalsVersionParams{WorkspaceID: p.workspaceID, ID: prop.ID}); err != nil {
				return err
			}
		}
		out, err = q.UpdateAnalyticsGoal(ctx, store.UpdateAnalyticsGoalParams{
			WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: cur.ID, Name: f.name, Kind: f.kind, PathPattern: f.path, EventName: f.event,
			PropKey: f.propKey, PropValue: f.propValue, ScrollPct: f.scroll, Version: version, Now: s.now(),
		})
		return err
	})
	if store.IsUniqueViolation(err) {
		return nil, errGoalTaken
	}
	if err != nil {
		return nil, err
	}
	s.analytics.answers.forget(out.PropertyID)
	return oas.UpdateAnalyticsGoal200JSONResponse(goalBody(out)), nil
}

func (s *Server) DeleteAnalyticsGoal(ctx context.Context, req oas.DeleteAnalyticsGoalRequestObject) (oas.DeleteAnalyticsGoalResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		prop, err := visibleProperty(ctx, q, p, req.PropertyId, true)
		if err != nil {
			return err
		}
		if err := requireAnalyticsManager(p); err != nil {
			return err
		}
		funnels, err := q.ListFunnelsWithGoal(ctx, store.ListFunnelsWithGoalParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, GoalID: req.GoalId})
		if err != nil {
			return err
		}
		if len(funnels) > 0 {
			e := problem(http.StatusConflict, "goal_in_use", "the funnels "+strings.Join(funnels, ", ")+" have this goal as a step; change them first")
			return e
		}
		n, err := q.DeleteAnalyticsGoal(ctx, store.DeleteAnalyticsGoalParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: req.GoalId})
		if err != nil {
			return err
		}
		if n == 0 {
			return errGoalGone
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.analytics.answers.forget(req.PropertyId)
	return oas.DeleteAnalyticsGoal204Response{}, nil
}

func funnelBody(f store.AnalyticsFunnel) oas.AnalyticsFunnel {
	return oas.AnalyticsFunnel{Id: f.ID, Name: f.Name, Steps: f.Steps, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt}
}

// funnelSteps checks that every step is a goal of the property.
func funnelSteps(ctx context.Context, q *store.Queries, ws, property uuid.UUID, steps []uuid.UUID) error {
	if len(steps) < 2 || len(steps) > 8 {
		return errValidation("steps must list 2 to 8 goals")
	}
	goals, err := q.ListAnalyticsGoalsByID(ctx, store.ListAnalyticsGoalsByIDParams{WorkspaceID: ws, PropertyID: property, Ids: steps})
	if err != nil {
		return err
	}
	for _, id := range steps {
		if !slices.ContainsFunc(goals, func(g store.AnalyticsGoal) bool { return g.ID == id }) {
			return errValidation("step " + id.String() + " is not a goal of this property")
		}
	}
	return nil
}

func (s *Server) ListAnalyticsFunnels(ctx context.Context, req oas.ListAnalyticsFunnelsRequestObject) (oas.ListAnalyticsFunnelsResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	prop, err := visibleProperty(ctx, s.st.Queries, p, req.PropertyId, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.st.ListAnalyticsFunnels(ctx, store.ListAnalyticsFunnelsParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID})
	if err != nil {
		return nil, err
	}
	out := oas.ListAnalyticsFunnels200JSONResponse{Items: make([]oas.AnalyticsFunnel, len(rows))}
	for i, f := range rows {
		out.Items[i] = funnelBody(f)
	}
	return out, nil
}

func (s *Server) CreateAnalyticsFunnel(ctx context.Context, req oas.CreateAnalyticsFunnelRequestObject) (oas.CreateAnalyticsFunnelResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	var out store.AnalyticsFunnel
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		prop, err := visibleProperty(ctx, q, p, req.PropertyId, true)
		if err != nil {
			return err
		}
		if err := requireAnalyticsManager(p); err != nil {
			return err
		}
		name, err := trimmed(req.Body.Name, 1, 120, "name")
		if err != nil {
			return err
		}
		if err := funnelSteps(ctx, q, p.workspaceID, prop.ID, req.Body.Steps); err != nil {
			return err
		}
		n, err := q.CountAnalyticsFunnels(ctx, store.CountAnalyticsFunnelsParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID})
		if err != nil {
			return err
		}
		if n >= maxFunnelsPerProperty {
			return errFunnelLimit
		}
		out, err = q.CreateAnalyticsFunnel(ctx, store.CreateAnalyticsFunnelParams{ID: newID(), WorkspaceID: p.workspaceID, PropertyID: prop.ID,
			Name: name, Steps: req.Body.Steps, CreatedBy: p.memberRef(), Now: s.now()})
		return err
	})
	if store.IsUniqueViolation(err) {
		return nil, errFunnelTaken
	}
	if err != nil {
		return nil, err
	}
	return oas.CreateAnalyticsFunnel201JSONResponse(funnelBody(out)), nil
}

func (s *Server) UpdateAnalyticsFunnel(ctx context.Context, req oas.UpdateAnalyticsFunnelRequestObject) (oas.UpdateAnalyticsFunnelResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	var out store.AnalyticsFunnel
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		prop, err := visibleProperty(ctx, q, p, req.PropertyId, true)
		if err != nil {
			return err
		}
		if err := requireAnalyticsManager(p); err != nil {
			return err
		}
		cur, err := q.GetAnalyticsFunnel(ctx, store.GetAnalyticsFunnelParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: req.FunnelId})
		if store.IsNotFound(err) {
			return errFunnelGone
		}
		if err != nil {
			return err
		}
		if req.Body.Name != nil {
			if cur.Name, err = trimmed(*req.Body.Name, 1, 120, "name"); err != nil {
				return err
			}
		}
		if req.Body.Steps != nil {
			if err := funnelSteps(ctx, q, p.workspaceID, prop.ID, *req.Body.Steps); err != nil {
				return err
			}
			cur.Steps = *req.Body.Steps
		}
		out, err = q.UpdateAnalyticsFunnel(ctx, store.UpdateAnalyticsFunnelParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: cur.ID,
			Name: cur.Name, Steps: cur.Steps, Now: s.now()})
		return err
	})
	if store.IsUniqueViolation(err) {
		return nil, errFunnelTaken
	}
	if err != nil {
		return nil, err
	}
	return oas.UpdateAnalyticsFunnel200JSONResponse(funnelBody(out)), nil
}

func (s *Server) DeleteAnalyticsFunnel(ctx context.Context, req oas.DeleteAnalyticsFunnelRequestObject) (oas.DeleteAnalyticsFunnelResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	prop, err := visibleProperty(ctx, s.st.Queries, p, req.PropertyId, false)
	if err != nil {
		return nil, err
	}
	if err := requireAnalyticsManager(p); err != nil {
		return nil, err
	}
	n, err := s.st.DeleteAnalyticsFunnel(ctx, store.DeleteAnalyticsFunnelParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: req.FunnelId})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, errFunnelGone
	}
	return oas.DeleteAnalyticsFunnel204Response{}, nil
}

// funnelRange is the report's days: a period, from and to, or the last 30 days.
func funnelRange(params oas.GetAnalyticsFunnelReportParams, today time.Time) (time.Time, time.Time, error) {
	r := oas.AnalyticsRange{From: params.From, To: params.To}
	if params.Period != nil {
		r.Period = new(oas.AnalyticsRangePeriod(*params.Period))
	} else if params.From == nil && params.To == nil {
		r.Period = new(oas.AnalyticsLast30Days)
	}
	return analyticsRange(r, today)
}

func (s *Server) GetAnalyticsFunnelReport(ctx context.Context, req oas.GetAnalyticsFunnelReportRequestObject) (oas.GetAnalyticsFunnelReportResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	prop, err := visibleProperty(ctx, s.st.Queries, p, req.PropertyId, false)
	if err != nil {
		return nil, err
	}
	f, err := s.st.GetAnalyticsFunnel(ctx, store.GetAnalyticsFunnelParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: req.FunnelId})
	if store.IsNotFound(err) {
		return nil, errFunnelGone
	}
	if err != nil {
		return nil, err
	}
	now := s.now()
	today := analytics.LocalDay(now, propertyLocation(prop.Timezone))
	from, to, err := funnelRange(req.Params, today)
	if err != nil {
		return nil, err
	}
	rawFrom := s.rawFrom()
	if from.Before(rawFrom) {
		return nil, analyticsQueryError(&analytics.NeedsRawError{From: rawFrom})
	}
	limit := maxFunnelDays
	busy, err := s.st.CountAnalyticsEventsSince(ctx, store.CountAnalyticsEventsSinceParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, FromDay: today.AddDate(0, 0, -29)})
	if err != nil {
		return nil, err
	}
	if busy > busyPropertyEvents {
		limit = maxBusyFunnelDays
	}
	if int(to.Sub(from).Hours()/24)+1 > limit {
		e := problem(http.StatusUnprocessableEntity, "range_too_large", "a funnel report spans at most "+strconv.Itoa(limit)+" days")
		if limit == maxBusyFunnelDays {
			e.Detail += ", as the property counted more than 10 million events over the last 30 days"
		}
		return nil, e
	}
	goals, err := s.st.ListAnalyticsGoalsByID(ctx, store.ListAnalyticsGoalsByIDParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, Ids: f.Steps})
	if err != nil {
		return nil, err
	}
	steps := make([]analytics.Goal, len(f.Steps))
	names := make([]string, len(f.Steps))
	versions := make([]string, len(f.Steps))
	for i, id := range f.Steps {
		j := slices.IndexFunc(goals, func(g store.AnalyticsGoal) bool { return g.ID == id })
		if j < 0 {
			return nil, errFunnelStepsBroken
		}
		steps[i], names[i], versions[i] = *goalDef(goals[j]), goals[j].Name, strconv.FormatInt(goals[j].Version, 10)
	}
	a := s.analytics
	key := prop.ID.String() + "|funnel|" + f.ID.String() + "|" + f.UpdatedAt.String() + "|" + strings.Join(versions, ",") + "|" +
		from.Format(time.DateOnly) + "|" + to.Format(time.DateOnly)
	var counts []analytics.FunnelStep
	if v, ok := a.answers.get(key, now); ok {
		counts = v.([]analytics.FunnelStep)
	} else {
		release, err := a.slots.acquire(ctx, prop.WorkspaceID)
		if err != nil {
			return nil, err
		}
		counts, err = a.store.Funnel(ctx, p.workspaceID, prop.ID, from, to, steps)
		release()
		if err != nil {
			return nil, err
		}
		a.answers.put(key, counts, now, analyticsFunnelTTL)
	}
	out := oas.AnalyticsFunnelReport{FunnelId: f.ID, From: openapi_types.Date{Time: from}, To: openapi_types.Date{Time: to},
		RawWindowFrom: openapi_types.Date{Time: rawFrom}, Steps: make([]oas.AnalyticsFunnelStep, len(steps))}
	for i, c := range counts {
		st := oas.AnalyticsFunnelStep{GoalId: f.Steps[i], Name: names[i], Visitors: c.Visitors, Visits: c.Visits, StepRate: 100}
		if first := counts[0].Visitors; first > 0 {
			st.ConversionRate = percent(c.Visitors, first)
		}
		if i > 0 {
			prev := counts[i-1].Visitors
			st.Dropoff, st.StepRate = prev-c.Visitors, 0
			if prev > 0 {
				st.StepRate = percent(c.Visitors, prev)
			}
		}
		out.Steps[i] = st
	}
	return oas.GetAnalyticsFunnelReport200JSONResponse(out), nil
}

func percent(a, b int64) float64 {
	return float64(int64(float64(a)/float64(b)*10000+0.5)) / 100
}

// segmentFilters checks a segment's or query's filters and turns them into the query's.
func segmentFilters(in []oas.AnalyticsFilter) ([]analytics.Filter, error) {
	out := make([]analytics.Filter, len(in))
	for i, f := range in {
		out[i] = analytics.Filter{Dimension: string(f.Dimension), Op: string(f.Op), Values: f.Values, Prop: deref(f.Prop)}
		if len(f.Values) == 0 || len(f.Values) > 20 {
			return nil, errValidation("a filter has 1 to 20 values")
		}
		for _, v := range f.Values {
			if len(v) > 2000 {
				return nil, errValidation("filter values have at most 2,000 characters")
			}
		}
		if out[i].Prop != "" {
			p, err := trimmed(out[i].Prop, 1, 300, "filters.prop")
			if err != nil {
				return nil, err
			}
			out[i].Prop = p
		}
	}
	if err := analytics.CheckFilters(out); err != nil {
		return nil, analyticsQueryError(err)
	}
	return out, nil
}

func segmentBody(sg store.AnalyticsSegment) oas.AnalyticsSegment {
	out := oas.AnalyticsSegment{Id: sg.ID, Name: sg.Name, CreatedBy: sg.CreatedBy, CreatedAt: sg.CreatedAt, UpdatedAt: sg.UpdatedAt, Filters: []oas.AnalyticsFilter{}}
	_ = json.Unmarshal(sg.Filters, &out.Filters)
	return out
}

func (s *Server) ListAnalyticsSegments(ctx context.Context, req oas.ListAnalyticsSegmentsRequestObject) (oas.ListAnalyticsSegmentsResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	prop, err := visibleProperty(ctx, s.st.Queries, p, req.PropertyId, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.st.ListAnalyticsSegments(ctx, store.ListAnalyticsSegmentsParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID})
	if err != nil {
		return nil, err
	}
	out := oas.ListAnalyticsSegments200JSONResponse{Items: make([]oas.AnalyticsSegment, len(rows))}
	for i, sg := range rows {
		out.Items[i] = segmentBody(sg)
	}
	return out, nil
}

func (s *Server) CreateAnalyticsSegment(ctx context.Context, req oas.CreateAnalyticsSegmentRequestObject) (oas.CreateAnalyticsSegmentResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	name, err := trimmed(req.Body.Name, 1, 100, "name")
	if err != nil {
		return nil, err
	}
	if len(req.Body.Filters) < 1 || len(req.Body.Filters) > 10 {
		return nil, errValidation("a segment has 1 to 10 filters")
	}
	if _, err := segmentFilters(req.Body.Filters); err != nil {
		return nil, err
	}
	var out store.AnalyticsSegment
	err = s.st.InTx(ctx, func(q *store.Queries) error {
		prop, err := visibleProperty(ctx, q, p, req.PropertyId, true)
		if err != nil {
			return err
		}
		n, err := q.CountAnalyticsSegments(ctx, store.CountAnalyticsSegmentsParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID})
		if err != nil {
			return err
		}
		if n >= maxSegmentsPerProperty {
			return errSegmentLimit
		}
		out, err = q.CreateAnalyticsSegment(ctx, store.CreateAnalyticsSegmentParams{ID: newID(), WorkspaceID: p.workspaceID, PropertyID: prop.ID,
			Name: name, Filters: mustJSON(req.Body.Filters), CreatedBy: p.memberRef(), Now: s.now()})
		return err
	})
	if store.IsUniqueViolation(err) {
		return nil, errSegmentTaken
	}
	if err != nil {
		return nil, err
	}
	return oas.CreateAnalyticsSegment201JSONResponse(segmentBody(out)), nil
}

func (s *Server) UpdateAnalyticsSegment(ctx context.Context, req oas.UpdateAnalyticsSegmentRequestObject) (oas.UpdateAnalyticsSegmentResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	var out store.AnalyticsSegment
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		prop, err := visibleProperty(ctx, q, p, req.PropertyId, false)
		if err != nil {
			return err
		}
		cur, err := q.GetAnalyticsSegment(ctx, store.GetAnalyticsSegmentParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: req.SegmentId})
		if store.IsNotFound(err) {
			return errSegmentGone
		}
		if err != nil {
			return err
		}
		if !mayEditShared(p, cur.CreatedBy) {
			return errForbidden
		}
		if req.Body.Name != nil {
			if cur.Name, err = trimmed(*req.Body.Name, 1, 100, "name"); err != nil {
				return err
			}
		}
		if req.Body.Filters != nil {
			if len(*req.Body.Filters) < 1 || len(*req.Body.Filters) > 10 {
				return errValidation("a segment has 1 to 10 filters")
			}
			if _, err := segmentFilters(*req.Body.Filters); err != nil {
				return err
			}
			cur.Filters = mustJSON(*req.Body.Filters)
		}
		out, err = q.UpdateAnalyticsSegment(ctx, store.UpdateAnalyticsSegmentParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: cur.ID,
			Name: cur.Name, Filters: cur.Filters, Now: s.now()})
		return err
	})
	if store.IsUniqueViolation(err) {
		return nil, errSegmentTaken
	}
	if err != nil {
		return nil, err
	}
	return oas.UpdateAnalyticsSegment200JSONResponse(segmentBody(out)), nil
}

func (s *Server) DeleteAnalyticsSegment(ctx context.Context, req oas.DeleteAnalyticsSegmentRequestObject) (oas.DeleteAnalyticsSegmentResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	prop, err := visibleProperty(ctx, s.st.Queries, p, req.PropertyId, false)
	if err != nil {
		return nil, err
	}
	cur, err := s.st.GetAnalyticsSegment(ctx, store.GetAnalyticsSegmentParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: req.SegmentId})
	if store.IsNotFound(err) {
		return nil, errSegmentGone
	}
	if err != nil {
		return nil, err
	}
	if !mayEditShared(p, cur.CreatedBy) {
		return nil, errForbidden
	}
	if _, err := s.st.DeleteAnalyticsSegment(ctx, store.DeleteAnalyticsSegmentParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: cur.ID}); err != nil {
		return nil, err
	}
	return oas.DeleteAnalyticsSegment204Response{}, nil
}

func annotationBody(a store.AnalyticsAnnotation) oas.AnalyticsAnnotation {
	return oas.AnalyticsAnnotation{Id: a.ID, Kind: oas.AnalyticsAnnotationKind(a.Kind), StartsOn: openapi_types.Date{Time: a.StartsOn},
		EndsOn: openapi_types.Date{Time: a.EndsOn}, Text: a.Text, GoalId: a.GoalID, AlertId: a.AlertID, CreatedBy: a.CreatedBy, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt}
}

func dateOnly(d openapi_types.Date) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

func annotationDays(starts time.Time, ends *openapi_types.Date) (time.Time, time.Time, error) {
	end := starts
	if ends != nil {
		end = dateOnly(*ends)
	}
	if end.Before(starts) {
		return starts, end, errValidation("ends_on must not be before starts_on")
	}
	if end.Sub(starts) > maxAnalyticsRange*24*time.Hour {
		return starts, end, errValidation("an annotation spans at most 400 days")
	}
	return starts, end, nil
}

func (s *Server) ListAnalyticsAnnotations(ctx context.Context, req oas.ListAnalyticsAnnotationsRequestObject) (oas.ListAnalyticsAnnotationsResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	prop, err := visibleProperty(ctx, s.st.Queries, p, req.PropertyId, false)
	if err != nil {
		return nil, err
	}
	arg := store.ListAnalyticsAnnotationsParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID}
	if req.Params.From != nil {
		arg.FromDay = new(dateOnly(*req.Params.From))
	}
	if req.Params.To != nil {
		arg.ToDay = new(dateOnly(*req.Params.To))
	}
	if arg.FromDay != nil && arg.ToDay != nil && arg.ToDay.Before(*arg.FromDay) {
		return nil, errValidation("to must not be before from")
	}
	rows, err := s.st.ListAnalyticsAnnotations(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := oas.ListAnalyticsAnnotations200JSONResponse{Items: make([]oas.AnalyticsAnnotation, len(rows))}
	for i, a := range rows {
		out.Items[i] = annotationBody(a)
	}
	return out, nil
}

func (s *Server) CreateAnalyticsAnnotation(ctx context.Context, req oas.CreateAnalyticsAnnotationRequestObject) (oas.CreateAnalyticsAnnotationResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	b := req.Body
	text, err := trimmed(b.Text, 1, 300, "text")
	if err != nil {
		return nil, err
	}
	starts, ends, err := annotationDays(dateOnly(b.StartsOn), b.EndsOn)
	if err != nil {
		return nil, err
	}
	var out store.AnalyticsAnnotation
	err = s.st.InTx(ctx, func(q *store.Queries) error {
		prop, err := visibleProperty(ctx, q, p, req.PropertyId, true)
		if err != nil {
			return err
		}
		n, err := q.CountAnalyticsAnnotations(ctx, store.CountAnalyticsAnnotationsParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID})
		if err != nil {
			return err
		}
		if n >= maxAnnotationsPerProperty {
			return errAnnotationLimit
		}
		out, err = q.CreateAnalyticsAnnotation(ctx, store.CreateAnalyticsAnnotationParams{ID: newID(), WorkspaceID: p.workspaceID, PropertyID: prop.ID,
			Kind: annotationNote, StartsOn: starts, EndsOn: ends, Text: text, CreatedBy: p.memberRef(), Now: s.now()})
		return err
	})
	if err != nil {
		return nil, err
	}
	return oas.CreateAnalyticsAnnotation201JSONResponse(annotationBody(out)), nil
}

func (s *Server) UpdateAnalyticsAnnotation(ctx context.Context, req oas.UpdateAnalyticsAnnotationRequestObject) (oas.UpdateAnalyticsAnnotationResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	prop, err := visibleProperty(ctx, s.st.Queries, p, req.PropertyId, false)
	if err != nil {
		return nil, err
	}
	cur, err := s.st.GetAnalyticsAnnotation(ctx, store.GetAnalyticsAnnotationParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: req.AnnotationId})
	if store.IsNotFound(err) {
		return nil, errAnnotationGone
	}
	if err != nil {
		return nil, err
	}
	if cur.Kind != annotationNote {
		return nil, errAnnotationSystem
	}
	if !mayEditShared(p, cur.CreatedBy) {
		return nil, errForbidden
	}
	b := req.Body
	if b.Text != nil {
		if cur.Text, err = trimmed(*b.Text, 1, 300, "text"); err != nil {
			return nil, err
		}
	}
	starts := cur.StartsOn
	if b.StartsOn != nil {
		starts = dateOnly(*b.StartsOn)
	}
	ends := b.EndsOn
	if ends == nil {
		ends = &openapi_types.Date{Time: cur.EndsOn}
		if b.StartsOn != nil && cur.EndsOn.Before(starts) {
			ends = b.StartsOn
		}
	}
	if cur.StartsOn, cur.EndsOn, err = annotationDays(starts, ends); err != nil {
		return nil, err
	}
	out, err := s.st.UpdateAnalyticsAnnotation(ctx, store.UpdateAnalyticsAnnotationParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: cur.ID,
		StartsOn: cur.StartsOn, EndsOn: cur.EndsOn, Text: cur.Text, Now: s.now()})
	if store.IsNotFound(err) {
		return nil, errAnnotationGone
	}
	if err != nil {
		return nil, err
	}
	return oas.UpdateAnalyticsAnnotation200JSONResponse(annotationBody(out)), nil
}

func (s *Server) DeleteAnalyticsAnnotation(ctx context.Context, req oas.DeleteAnalyticsAnnotationRequestObject) (oas.DeleteAnalyticsAnnotationResponseObject, error) {
	if err := s.analyticsOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	prop, err := visibleProperty(ctx, s.st.Queries, p, req.PropertyId, false)
	if err != nil {
		return nil, err
	}
	cur, err := s.st.GetAnalyticsAnnotation(ctx, store.GetAnalyticsAnnotationParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: req.AnnotationId})
	if store.IsNotFound(err) {
		return nil, errAnnotationGone
	}
	if err != nil {
		return nil, err
	}
	owner := cur.CreatedBy
	if cur.Kind != annotationNote {
		owner = nil
	}
	if !mayEditShared(p, owner) {
		return nil, errForbidden
	}
	if _, err := s.st.DeleteAnalyticsAnnotation(ctx, store.DeleteAnalyticsAnnotationParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: cur.ID}); err != nil {
		return nil, err
	}
	return oas.DeleteAnalyticsAnnotation204Response{}, nil
}

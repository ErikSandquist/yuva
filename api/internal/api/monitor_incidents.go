package api

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	maxIncidentComponents  = 100
	maxMaintenanceWindow   = 31 * 24 * time.Hour
	defaultIncidentsLimit  = 50
	eventMonitorIncident   = string(oas.MonitorIncidentEventTypeMonitorIncident)
	incidentResolved       = string(oas.MonitorIncidentResolved)
	kindMaintenance        = string(oas.MonitorIncidentMaintenance)
	kindIncident           = string(oas.MonitorIncidentIncident)
	maintenanceEndedText   = "The maintenance window ended."
	incidentHealthyText    = " has been healthy for 5 minutes."
	incidentServiceGoneTxt = "The service was deleted."
)

var (
	errIncidentGone = problem(http.StatusNotFound, "not_found", "no such incident")
	errIncidentOpen = problem(http.StatusConflict, "incident_open", "the service has another open automatic incident")
)

func incidentUpdateBody(u store.MonitorIncidentUpdate) oas.MonitorIncidentUpdate {
	return oas.MonitorIncidentUpdate{Id: u.ID, State: oas.MonitorIncidentState(u.State), Text: u.Text, AuthorId: u.AuthorID, Notify: u.Notify,
		CreatedAt: u.CreatedAt.UTC()}
}

func incidentBody(i store.MonitorIncident, updates []store.MonitorIncidentUpdate, now time.Time) oas.MonitorIncident {
	out := oas.MonitorIncident{Id: i.ID, Origin: oas.MonitorIncidentOrigin(i.Origin), Kind: oas.MonitorIncidentKind(i.Kind), Title: i.Title,
		State: oas.MonitorIncidentState(i.State), Impact: oas.MonitorIncidentImpact(i.Impact), ServiceIds: nonNil(i.ServiceIds), DependencyIds: nonNil(i.DependencyIds),
		CheckIds: nonNil(i.CheckIds), StartedAt: i.StartedAt.UTC(), ResolvedAt: utcPtr(i.ResolvedAt), ScheduledStart: utcPtr(i.ScheduledStart),
		ScheduledEnd: utcPtr(i.ScheduledEnd), Published: i.Published, Updates: []oas.MonitorIncidentUpdate{}, CreatedBy: i.CreatedBy, CreatedAt: i.CreatedAt.UTC(),
		UpdatedAt: i.UpdatedAt.UTC()}
	out.Active = i.ResolvedAt == nil && (i.Kind == kindIncident || maintenanceActive(i, now))
	for _, u := range updates {
		out.Updates = append(out.Updates, incidentUpdateBody(u))
	}
	return out
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	return new(t.UTC())
}

func (s *Server) incidentBodies(ctx context.Context, q *store.Queries, ws uuid.UUID, incidents []store.MonitorIncident) ([]oas.MonitorIncident, error) {
	ids := make([]uuid.UUID, len(incidents))
	for i, inc := range incidents {
		ids[i] = inc.ID
	}
	byIncident := map[uuid.UUID][]store.MonitorIncidentUpdate{}
	if len(ids) > 0 {
		ups, err := q.ListMonitorIncidentUpdates(ctx, store.ListMonitorIncidentUpdatesParams{WorkspaceID: ws, IncidentIds: ids})
		if err != nil {
			return nil, err
		}
		for _, u := range ups {
			byIncident[u.IncidentID] = append(byIncident[u.IncidentID], u)
		}
	}
	now := s.now()
	out := make([]oas.MonitorIncident, len(incidents))
	for i, inc := range incidents {
		out[i] = incidentBody(inc, byIncident[inc.ID], now)
	}
	return out, nil
}

// incidentEvent queues the monitor.incident event of an incident as it is after the change.
func (s *Server) incidentEvent(ctx context.Context, q *store.Queries, events *eventBatch, i store.MonitorIncident, u *store.MonitorIncidentUpdate) error {
	body, err := s.incidentBodies(ctx, q, i.WorkspaceID, []store.MonitorIncident{i})
	if err != nil {
		return err
	}
	d := oas.MonitorIncidentEventData{Incident: body[0]}
	if u != nil {
		d.Update = new(incidentUpdateBody(*u))
	}
	events.add(eventMonitorIncident, nil, nil, d)
	return nil
}

func (s *Server) addIncidentUpdate(ctx context.Context, q *store.Queries, events *eventBatch, i store.MonitorIncident, state, text string, author *uuid.UUID,
	notify bool) error {
	u, err := q.InsertMonitorIncidentUpdate(ctx, store.InsertMonitorIncidentUpdateParams{ID: newID(), WorkspaceID: i.WorkspaceID, IncidentID: i.ID, State: state,
		Text: truncateRunes(text, 5000), AuthorID: author, Notify: notify, Now: s.now()})
	if err != nil {
		return err
	}
	return s.incidentEvent(ctx, q, events, i, &u)
}

func impactOf(state string) string {
	if state == stateDown {
		return string(oas.MonitorImpactMajor)
	}
	return string(oas.MonitorImpactMinor)
}

func impactRank(impact string) int {
	return slices.Index([]string{"none", "minor", "major", "critical"}, impact)
}

func serviceStateText(svc store.MonitorService) string {
	text := serviceLabel(svc) + " is " + svc.State
	if svc.Reason != nil && *svc.Reason != "" {
		text += ": " + *svc.Reason
	}
	return text
}

// autoIncident opens a service's automatic incident when it turned degraded or down, or tells the
// open one that it got worse.
func (s *Server) autoIncident(ctx context.Context, svc store.MonitorService, previous string) error {
	if svc.State != stateDegraded && svc.State != stateDown {
		return nil
	}
	now := s.now()
	return s.inTx(ctx, svc.WorkspaceID, func(q *store.Queries, events *eventBatch) error {
		made, err := q.CreateMonitorIncident(ctx, store.CreateMonitorIncidentParams{ID: newID(), WorkspaceID: svc.WorkspaceID, Origin: string(oas.MonitorIncidentAuto),
			Kind: kindIncident, Title: truncateRunes(serviceLabel(svc)+" is "+svc.State, 200), State: string(oas.MonitorIncidentInvestigating),
			Impact: impactOf(svc.State), ServiceIds: []uuid.UUID{svc.ID}, DependencyIds: []uuid.UUID{}, CheckIds: []uuid.UUID{}, AutoServiceID: &svc.ID,
			StartedAt: now, Now: now})
		if err == nil {
			return s.addIncidentUpdate(ctx, q, events, made, made.State, serviceStateText(svc), nil, true)
		}
		if !store.IsNotFound(err) {
			return err
		}
		open, err := q.LockOpenAutoIncident(ctx, store.LockOpenAutoIncidentParams{WorkspaceID: svc.WorkspaceID, ServiceID: &svc.ID})
		if store.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if previous == stateUnknown || previous == stateHealthy || monitorRankWorse(svc.State, previous) {
			impact := open.Impact
			if impactRank(impactOf(svc.State)) > impactRank(impact) {
				impact = impactOf(svc.State)
			}
			open, err = q.UpdateMonitorIncident(ctx, incidentUpdateArg(open, func(a *store.UpdateMonitorIncidentParams) { a.Impact, a.Now = impact, now }))
			if err != nil {
				return err
			}
			return s.addIncidentUpdate(ctx, q, events, open, open.State, serviceStateText(svc), nil, false)
		}
		return nil
	})
}

func monitorRankWorse(a, b string) bool {
	return rankOf(a) > rankOf(b)
}

func rankOf(state string) int {
	switch state {
	case stateDown:
		return 2
	case stateDegraded:
		return 1
	}
	return 0
}

func incidentUpdateArg(i store.MonitorIncident, change func(*store.UpdateMonitorIncidentParams)) store.UpdateMonitorIncidentParams {
	a := store.UpdateMonitorIncidentParams{WorkspaceID: i.WorkspaceID, ID: i.ID, Title: i.Title, State: i.State, Impact: i.Impact, ServiceIds: nonNil(i.ServiceIds),
		DependencyIds: nonNil(i.DependencyIds), CheckIds: nonNil(i.CheckIds), StartedAt: i.StartedAt, ResolvedAt: i.ResolvedAt, ScheduledStart: i.ScheduledStart,
		ScheduledEnd: i.ScheduledEnd, Published: i.Published, Now: i.UpdatedAt}
	change(&a)
	return a
}

// resolveHealedIncidents resolves the automatic incidents whose service has been healthy for 5
// minutes, or is gone.
func (s *Server) resolveHealedIncidents(ctx context.Context, ws uuid.UUID, services map[uuid.UUID]store.MonitorService) error {
	open, err := s.st.ListOpenMonitorIncidents(ctx, ws)
	if err != nil {
		return err
	}
	now := s.now()
	for _, i := range open {
		if i.AutoServiceID == nil {
			continue
		}
		svc, ok := services[*i.AutoServiceID]
		text := incidentServiceGoneTxt
		if ok {
			if svc.State != stateHealthy || svc.StateSince == nil || now.Sub(*svc.StateSince) < autoIncidentHealthy {
				continue
			}
			text = serviceLabel(svc) + incidentHealthyText
		} else if _, err := s.st.GetMonitorService(ctx, store.GetMonitorServiceParams{WorkspaceID: ws, ID: *i.AutoServiceID}); !store.IsNotFound(err) {
			continue
		}
		if err := s.resolveIncident(ctx, ws, i.ID, text, func(cur store.MonitorIncident) bool { return cur.ResolvedAt == nil }); err != nil {
			return err
		}
	}
	return nil
}

// resolveIncident resolves an open incident with a server update that notifies, when still is
// true of it under the lock.
func (s *Server) resolveIncident(ctx context.Context, ws, id uuid.UUID, text string, still func(store.MonitorIncident) bool) error {
	now := s.now()
	return s.inTx(ctx, ws, func(q *store.Queries, events *eventBatch) error {
		cur, err := q.LockMonitorIncident(ctx, store.LockMonitorIncidentParams{WorkspaceID: ws, ID: id})
		if store.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !still(cur) {
			return nil
		}
		cur, err = q.UpdateMonitorIncident(ctx, incidentUpdateArg(cur, func(a *store.UpdateMonitorIncidentParams) {
			a.State, a.ResolvedAt, a.Now = incidentResolved, &now, now
		}))
		if err != nil {
			return err
		}
		return s.addIncidentUpdate(ctx, q, events, cur, incidentResolved, text, nil, true)
	})
}

// endMaintenance resolves the maintenance windows that passed their end, then opens incidents for
// the services they covered that are still degraded or down.
func (s *Server) endMaintenance(ctx context.Context, ws uuid.UUID) error {
	open, err := s.st.ListOpenMonitorIncidents(ctx, ws)
	if err != nil {
		return err
	}
	now := s.now()
	for _, i := range open {
		if i.Kind != kindMaintenance || i.ScheduledEnd == nil || now.Before(*i.ScheduledEnd) {
			continue
		}
		if err := s.resolveIncident(ctx, ws, i.ID, maintenanceEndedText, func(cur store.MonitorIncident) bool {
			return cur.ResolvedAt == nil && cur.ScheduledEnd != nil && !now.Before(*cur.ScheduledEnd)
		}); err != nil {
			return err
		}
		s.afterMaintenance(ctx, i)
	}
	return nil
}

// afterMaintenance re-evaluates the services a finished window covered, which opens the incidents
// and alerts their state held back.
func (s *Server) afterMaintenance(ctx context.Context, i store.MonitorIncident) {
	ids := slices.Clone(i.ServiceIds)
	for _, depID := range i.DependencyIds {
		if d, err := s.st.GetMonitorDependency(ctx, store.GetMonitorDependencyParams{WorkspaceID: i.WorkspaceID, ID: depID}); err == nil && !slices.Contains(ids, d.ServiceID) {
			ids = append(ids, d.ServiceID)
		}
	}
	for _, id := range ids {
		ch, err := s.evaluateService(ctx, i.WorkspaceID, id, nil)
		if err != nil {
			continue
		}
		if !ch.changed {
			if err := s.autoIncident(ctx, ch.svc, ch.svc.State); err != nil {
				s.log.WarnContext(ctx, "monitor incident", slog.String("workspace_id", i.WorkspaceID.String()), slog.Any("error", err))
			}
		}
		if err := s.serviceRulesOn(ctx, ch.svc); err != nil {
			s.log.WarnContext(ctx, "monitor service alerts", slog.String("workspace_id", i.WorkspaceID.String()), slog.Any("error", err))
		}
	}
}

func (s *Server) getIncident(ctx context.Context, q *store.Queries, ws, id uuid.UUID, lock bool) (store.MonitorIncident, error) {
	var i store.MonitorIncident
	var err error
	if lock {
		i, err = q.LockMonitorIncident(ctx, store.LockMonitorIncidentParams{WorkspaceID: ws, ID: id})
	} else {
		i, err = q.GetMonitorIncident(ctx, store.GetMonitorIncidentParams{WorkspaceID: ws, ID: id})
	}
	if store.IsNotFound(err) {
		return i, errIncidentGone
	}
	return i, err
}

// incidentFields are an incident's settings while they are checked.
type incidentFields struct {
	kind, title, state, impact     string
	services, dependencies, checks []uuid.UUID
	scheduledStart, scheduledEnd   *time.Time
	published                      bool
}

func (f *incidentFields) apply(c oas.MonitorIncidentComponents) {
	if c.ServiceIds != nil {
		f.services = *c.ServiceIds
	}
	if c.DependencyIds != nil {
		f.dependencies = *c.DependencyIds
	}
	if c.CheckIds != nil {
		f.checks = *c.CheckIds
	}
}

func compactIDs(ids []uuid.UUID) []uuid.UUID {
	out := slices.Clone(nonNil(ids))
	slices.SortFunc(out, func(a, b uuid.UUID) int { return a.Compare(b) })
	return slices.Compact(out)
}

func (s *Server) validateIncident(ctx context.Context, q *store.Queries, ws uuid.UUID, f *incidentFields, now time.Time, created bool) error {
	var err error
	if f.title, err = trimmed(f.title, 1, 200, "title"); err != nil {
		return err
	}
	if !oas.MonitorIncidentKind(f.kind).Valid() {
		return errValidation("kind must be incident or maintenance")
	}
	if !oas.MonitorIncidentState(f.state).Valid() {
		return errValidation("state must be investigating, identified, monitoring or resolved")
	}
	if !oas.MonitorIncidentImpact(f.impact).Valid() {
		return errValidation("impact must be none, minor, major or critical")
	}
	f.services, f.dependencies, f.checks = compactIDs(f.services), compactIDs(f.dependencies), compactIDs(f.checks)
	for _, set := range []struct {
		ids   []uuid.UUID
		name  string
		count func(context.Context, uuid.UUID, []uuid.UUID) (int64, error)
	}{
		{f.services, "service_ids", func(ctx context.Context, ws uuid.UUID, ids []uuid.UUID) (int64, error) {
			return q.CountMonitorServicesIn(ctx, store.CountMonitorServicesInParams{WorkspaceID: ws, Ids: ids})
		}},
		{f.dependencies, "dependency_ids", func(ctx context.Context, ws uuid.UUID, ids []uuid.UUID) (int64, error) {
			return q.CountMonitorDependenciesIn(ctx, store.CountMonitorDependenciesInParams{WorkspaceID: ws, Ids: ids})
		}},
		{f.checks, "check_ids", func(ctx context.Context, ws uuid.UUID, ids []uuid.UUID) (int64, error) {
			return q.CountMonitorChecksIn(ctx, store.CountMonitorChecksInParams{WorkspaceID: ws, Ids: ids})
		}},
	} {
		if len(set.ids) > maxIncidentComponents {
			return errValidation(set.name + " lists at most 100")
		}
		if len(set.ids) == 0 {
			continue
		}
		n, err := set.count(ctx, ws, set.ids)
		if err != nil {
			return err
		}
		if n != int64(len(set.ids)) {
			return errValidation(set.name + " must name components of the workspace")
		}
	}
	if f.kind == kindMaintenance {
		if f.scheduledStart == nil || f.scheduledEnd == nil {
			return errValidation("a maintenance window needs scheduled_start and scheduled_end")
		}
		if !f.scheduledEnd.After(*f.scheduledStart) || f.scheduledEnd.Sub(*f.scheduledStart) > maxMaintenanceWindow {
			return errValidation("scheduled_end comes after scheduled_start, at most 31 days later")
		}
		if created && !f.scheduledEnd.After(now) {
			return errValidation("scheduled_end must be in the future")
		}
	} else if f.scheduledStart != nil || f.scheduledEnd != nil {
		return errValidation("only maintenance windows are scheduled")
	}
	return nil
}

func (s *Server) ListMonitorIncidents(ctx context.Context, req oas.ListMonitorIncidentsRequestObject) (oas.ListMonitorIncidentsResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	limit := defaultIncidentsLimit
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	if limit < 1 || limit > 100 {
		return nil, errValidation("limit must be 1 to 100")
	}
	arg := store.ListMonitorIncidentsParams{WorkspaceID: p.workspaceID, ServiceID: req.Params.ServiceId, Before: req.Params.Before, Lim: int32(limit)}
	if st := req.Params.State; st != nil {
		switch *st {
		case oas.MonitorIncidentsOpen:
			arg.Open = new(true)
		case oas.MonitorIncidentsResolved:
			arg.Open = new(false)
		default:
			return nil, errValidation("state must be open or resolved")
		}
	}
	if k := req.Params.Kind; k != nil {
		if !k.Valid() {
			return nil, errValidation("kind must be incident or maintenance")
		}
		arg.Kind = new(string(*k))
	}
	rows, err := s.st.ListMonitorIncidents(ctx, arg)
	if err != nil {
		return nil, err
	}
	items, err := s.incidentBodies(ctx, s.st.Queries, p.workspaceID, rows)
	if err != nil {
		return nil, err
	}
	return oas.ListMonitorIncidents200JSONResponse{Items: items}, nil
}

func (s *Server) GetMonitorIncident(ctx context.Context, req oas.GetMonitorIncidentRequestObject) (oas.GetMonitorIncidentResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	i, err := s.getIncident(ctx, s.st.Queries, p.workspaceID, req.IncidentId, false)
	if err != nil {
		return nil, err
	}
	body, err := s.incidentBodies(ctx, s.st.Queries, p.workspaceID, []store.MonitorIncident{i})
	if err != nil {
		return nil, err
	}
	return oas.GetMonitorIncident200JSONResponse(body[0]), nil
}

func (s *Server) CreateMonitorIncident(ctx context.Context, req oas.CreateMonitorIncidentRequestObject) (oas.CreateMonitorIncidentResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if err := requireAnalyticsManager(p); err != nil {
		return nil, err
	}
	b := req.Body
	f := incidentFields{kind: kindIncident, title: b.Title, state: string(oas.MonitorIncidentInvestigating), impact: string(oas.MonitorImpactMinor),
		scheduledStart: b.ScheduledStart, scheduledEnd: b.ScheduledEnd}
	if b.Kind != nil {
		f.kind = string(*b.Kind)
	}
	if f.kind == kindMaintenance {
		f.state, f.impact = string(oas.MonitorIncidentIdentified), string(oas.MonitorImpactNone)
	}
	if b.State != nil {
		f.state = string(*b.State)
	}
	if b.Impact != nil {
		f.impact = string(*b.Impact)
	}
	if b.Published != nil {
		f.published = *b.Published
	}
	f.apply(oas.MonitorIncidentComponents{ServiceIds: b.ServiceIds, DependencyIds: b.DependencyIds, CheckIds: b.CheckIds})
	text := strings.TrimSpace(deref(b.Text))
	if text == "" {
		text = strings.TrimSpace(b.Title)
	}
	if len([]rune(text)) > 5000 {
		return nil, errValidation("text must be at most 5000 characters")
	}
	notify := true
	if b.Notify != nil {
		notify = *b.Notify
	}
	now := s.now()
	var out store.MonitorIncident
	err := s.inTx(ctx, p.workspaceID, func(q *store.Queries, events *eventBatch) error {
		if err := s.validateIncident(ctx, q, p.workspaceID, &f, now, true); err != nil {
			return err
		}
		started := now
		if f.kind == kindMaintenance {
			started = *f.scheduledStart
		}
		var resolved *time.Time
		if f.state == incidentResolved {
			resolved = &now
		}
		var err error
		out, err = q.CreateMonitorIncident(ctx, store.CreateMonitorIncidentParams{ID: newID(), WorkspaceID: p.workspaceID, Origin: string(oas.MonitorIncidentManual),
			Kind: f.kind, Title: f.title, State: f.state, Impact: f.impact, ServiceIds: f.services, DependencyIds: f.dependencies, CheckIds: f.checks,
			StartedAt: started, ResolvedAt: resolved, ScheduledStart: f.scheduledStart, ScheduledEnd: f.scheduledEnd, Published: f.published,
			CreatedBy: p.memberRef(), Now: now})
		if err != nil {
			return err
		}
		return s.addIncidentUpdate(ctx, q, events, out, f.state, text, p.memberRef(), notify)
	})
	if err != nil {
		return nil, err
	}
	body, err := s.incidentBodies(ctx, s.st.Queries, p.workspaceID, []store.MonitorIncident{out})
	if err != nil {
		return nil, err
	}
	return oas.CreateMonitorIncident201JSONResponse(body[0]), nil
}

func (s *Server) UpdateMonitorIncident(ctx context.Context, req oas.UpdateMonitorIncidentRequestObject) (oas.UpdateMonitorIncidentResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if err := requireAnalyticsManager(p); err != nil {
		return nil, err
	}
	b := req.Body
	now := s.now()
	var out store.MonitorIncident
	err := s.inTx(ctx, p.workspaceID, func(q *store.Queries, events *eventBatch) error {
		cur, err := s.getIncident(ctx, q, p.workspaceID, req.IncidentId, true)
		if err != nil {
			return err
		}
		f := incidentFields{kind: cur.Kind, title: cur.Title, state: cur.State, impact: cur.Impact, services: cur.ServiceIds, dependencies: cur.DependencyIds,
			checks: cur.CheckIds, scheduledStart: cur.ScheduledStart, scheduledEnd: cur.ScheduledEnd, published: cur.Published}
		if b.Title != nil {
			f.title = *b.Title
		}
		if b.Impact != nil {
			f.impact = string(*b.Impact)
		}
		if b.ScheduledStart != nil {
			f.scheduledStart = b.ScheduledStart
		}
		if b.ScheduledEnd != nil {
			f.scheduledEnd = b.ScheduledEnd
		}
		if b.Published != nil {
			f.published = *b.Published
		}
		f.apply(oas.MonitorIncidentComponents{ServiceIds: b.ServiceIds, DependencyIds: b.DependencyIds, CheckIds: b.CheckIds})
		if err := s.validateIncident(ctx, q, p.workspaceID, &f, now, false); err != nil {
			return err
		}
		out, err = q.UpdateMonitorIncident(ctx, incidentUpdateArg(cur, func(a *store.UpdateMonitorIncidentParams) {
			a.Title, a.Impact, a.ServiceIds, a.DependencyIds, a.CheckIds, a.Published, a.Now = f.title, f.impact, f.services, f.dependencies, f.checks, f.published, now
			a.ScheduledStart, a.ScheduledEnd = f.scheduledStart, f.scheduledEnd
			if f.kind == kindMaintenance {
				a.StartedAt = *f.scheduledStart
			}
		}))
		if err != nil {
			return err
		}
		return s.incidentEvent(ctx, q, events, out, nil)
	})
	if err != nil {
		return nil, err
	}
	body, err := s.incidentBodies(ctx, s.st.Queries, p.workspaceID, []store.MonitorIncident{out})
	if err != nil {
		return nil, err
	}
	return oas.UpdateMonitorIncident200JSONResponse(body[0]), nil
}

func (s *Server) DeleteMonitorIncident(ctx context.Context, req oas.DeleteMonitorIncidentRequestObject) (oas.DeleteMonitorIncidentResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if err := requireAnalyticsManager(p); err != nil {
		return nil, err
	}
	n, err := s.st.DeleteMonitorIncident(ctx, store.DeleteMonitorIncidentParams{WorkspaceID: p.workspaceID, ID: req.IncidentId})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, errIncidentGone
	}
	return oas.DeleteMonitorIncident204Response{}, nil
}

func (s *Server) CreateMonitorIncidentUpdate(ctx context.Context, req oas.CreateMonitorIncidentUpdateRequestObject) (oas.CreateMonitorIncidentUpdateResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if err := requireAnalyticsManager(p); err != nil {
		return nil, err
	}
	b := req.Body
	if !b.State.Valid() {
		return nil, errValidation("state must be investigating, identified, monitoring or resolved")
	}
	text, err := trimmed(b.Text, 1, 5000, "text")
	if err != nil {
		return nil, err
	}
	state := string(b.State)
	notify := state == incidentResolved
	if b.Notify != nil {
		notify = *b.Notify
	}
	now := s.now()
	var out store.MonitorIncident
	endedWindow := false
	err = s.inTx(ctx, p.workspaceID, func(q *store.Queries, events *eventBatch) error {
		cur, err := s.getIncident(ctx, q, p.workspaceID, req.IncidentId, true)
		if err != nil {
			return err
		}
		endedWindow = cur.Kind == kindMaintenance && cur.ResolvedAt == nil && state == incidentResolved
		out, err = q.UpdateMonitorIncident(ctx, incidentUpdateArg(cur, func(a *store.UpdateMonitorIncidentParams) {
			a.State, a.Now = state, now
			switch {
			case state == incidentResolved && cur.ResolvedAt == nil:
				a.ResolvedAt = &now
			case state != incidentResolved:
				a.ResolvedAt = nil
			}
		}))
		if store.IsUniqueViolation(err) {
			return errIncidentOpen
		}
		if err != nil {
			return err
		}
		return s.addIncidentUpdate(ctx, q, events, out, state, text, p.memberRef(), notify)
	})
	if err != nil {
		return nil, err
	}
	if endedWindow {
		s.afterMaintenance(ctx, out)
	}
	body, err := s.incidentBodies(ctx, s.st.Queries, p.workspaceID, []store.MonitorIncident{out})
	if err != nil {
		return nil, err
	}
	return oas.CreateMonitorIncidentUpdate201JSONResponse(body[0]), nil
}

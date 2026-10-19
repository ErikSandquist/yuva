package api

import (
	"context"
	"slices"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

// StatusMaintenanceArgs starts or completes one maintenance when its window begins or ends; it is
// queued for both times whenever the window or the publication changes and does nothing when not due.
type StatusMaintenanceArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	IncidentID  uuid.UUID `json:"incident_id"`
}

func (StatusMaintenanceArgs) Kind() string { return "status_maintenance" }

type statusMaintenanceWorker struct {
	river.WorkerDefaults[StatusMaintenanceArgs]
	s *Server
}

func (w *statusMaintenanceWorker) Work(ctx context.Context, job *river.Job[StatusMaintenanceArgs]) error {
	return w.s.RunStatusMaintenance(ctx, job.Args)
}

func queueMaintenance(ev *eventBatch, inc store.StatusIncident, now time.Time) {
	if inc.Kind != kindMaintenance || inc.PublishedAt == nil || incidentClosed(inc.State) {
		return
	}
	args := StatusMaintenanceArgs{WorkspaceID: inc.WorkspaceID, IncidentID: inc.ID}
	if inc.State == incScheduled {
		ev.job(args, &river.InsertOpts{ScheduledAt: maxTime(*inc.ScheduledStart, now)})
	}
	ev.job(args, &river.InsertOpts{ScheduledAt: maxTime(*inc.ScheduledEnd, now)})
}

func lockedStatusIncident(ctx context.Context, q *store.Queries, pg store.StatusPage, id uuid.UUID) (store.StatusIncident, error) {
	inc, err := q.LockStatusIncident(ctx, store.LockStatusIncidentParams{WorkspaceID: pg.WorkspaceID, PageID: pg.ID, ID: id})
	if store.IsNotFound(err) {
		return inc, errStatusIncidentGone
	}
	return inc, err
}

func updateBody(u store.StatusIncidentUpdate) oas.StatusIncidentUpdate {
	return oas.StatusIncidentUpdate{Id: u.ID, State: oas.StatusIncidentState(u.State), Body: u.Body, AuthorMemberId: u.AuthorMemberID,
		AuthorApiKeyId: u.AuthorKeyID, CreatedAt: u.CreatedAt, EditedAt: u.EditedAt}
}

type incidentParts struct {
	components map[uuid.UUID][]store.StatusIncidentComponent
	updates    map[uuid.UUID][]store.StatusIncidentUpdate
}

func loadIncidentParts(ctx context.Context, q *store.Queries, ws uuid.UUID, incs []store.StatusIncident) (incidentParts, error) {
	ids := make([]uuid.UUID, len(incs))
	for i, inc := range incs {
		ids[i] = inc.ID
	}
	out := incidentParts{components: map[uuid.UUID][]store.StatusIncidentComponent{}, updates: map[uuid.UUID][]store.StatusIncidentUpdate{}}
	comps, err := q.ListStatusIncidentComponents(ctx, store.ListStatusIncidentComponentsParams{WorkspaceID: ws, IncidentIds: ids})
	if err != nil {
		return out, err
	}
	for _, c := range comps {
		out.components[c.IncidentID] = append(out.components[c.IncidentID], c)
	}
	ups, err := q.ListStatusIncidentUpdates(ctx, store.ListStatusIncidentUpdatesParams{WorkspaceID: ws, IncidentIds: ids})
	if err != nil {
		return out, err
	}
	for _, u := range ups {
		out.updates[u.IncidentID] = append(out.updates[u.IncidentID], u)
	}
	return out, nil
}

func (s *Server) incidentBody(pg store.StatusPage, inc store.StatusIncident, parts incidentParts) oas.StatusIncident {
	out := oas.StatusIncident{
		Id: inc.ID, PageId: inc.PageID, Kind: oas.StatusIncidentKind(inc.Kind), Title: inc.Title, Impact: oas.StatusIncidentImpact(inc.Impact),
		State: oas.StatusIncidentState(inc.State), Components: []oas.StatusIncidentComponent{}, Updates: []oas.StatusIncidentUpdate{},
		Url: s.pageURL(pg, "/incidents/"+inc.ID.String()), PublishedAt: inc.PublishedAt, ScheduledStart: inc.ScheduledStart,
		ScheduledEnd: inc.ScheduledEnd, StartedAt: inc.StartedAt, ResolvedAt: inc.ResolvedAt, CreatedAt: inc.CreatedAt, UpdatedAt: inc.UpdatedAt,
	}
	for _, c := range parts.components[inc.ID] {
		out.Components = append(out.Components, oas.StatusIncidentComponent{ComponentId: c.ComponentID, State: oas.StatusComponentStateValue(c.State)})
	}
	for _, u := range parts.updates[inc.ID] {
		out.Updates = append(out.Updates, updateBody(u))
	}
	if inc.PublishedAt == nil {
		out.Notify = &inc.Notify
	}
	return out
}

func (s *Server) incidentOut(ctx context.Context, q *store.Queries, pg store.StatusPage, inc store.StatusIncident) (oas.StatusIncident, error) {
	parts, err := loadIncidentParts(ctx, q, pg.WorkspaceID, []store.StatusIncident{inc})
	if err != nil {
		return oas.StatusIncident{}, err
	}
	return s.incidentBody(pg, inc, parts), nil
}

func incidentComponentIDs(ctx context.Context, q *store.Queries, inc store.StatusIncident) ([]uuid.UUID, error) {
	rows, err := q.ListStatusIncidentComponents(ctx, store.ListStatusIncidentComponentsParams{WorkspaceID: inc.WorkspaceID, IncidentIds: []uuid.UUID{inc.ID}})
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ComponentID
	}
	return ids, nil
}

// statusActor is who a component's state change is recorded as: an incident or a monitor.
type statusActor struct {
	source string
	id     *uuid.UUID
}

type statusHolder struct {
	component     uuid.UUID
	state, source string
	id            uuid.UUID
}

func automaticSource(source string) bool {
	return source == statusSourceIncident || source == statusSourceMonitor
}

// statusHolders lists the failing monitors and the open published incidents that hold the components,
// monitors first so that a monitor keeps a state it shares with an incident. A monitor does not hold a
// component whose state a member or key set later than the monitor's last change.
func statusHolders(ctx context.Context, q *store.Queries, ws uuid.UUID, ids []uuid.UUID) ([]statusHolder, error) {
	mons, err := q.FailingStatusMonitors(ctx, store.FailingStatusMonitorsParams{WorkspaceID: ws, ComponentIds: ids})
	if err != nil {
		return nil, err
	}
	var out []statusHolder
	for _, m := range mons {
		out = append(out, statusHolder{component: m.ComponentID, state: m.State, source: statusSourceMonitor, id: m.ID})
	}
	incs, err := q.ActiveStatusHolders(ctx, store.ActiveStatusHoldersParams{WorkspaceID: ws, ComponentIds: ids, ExceptID: uuid.Nil()})
	if err != nil {
		return nil, err
	}
	for _, h := range incs {
		out = append(out, statusHolder{component: h.ComponentID, state: h.State, source: statusSourceIncident, id: h.IncidentID})
	}
	return out, nil
}

// settleComponents sets each component to the worst state the incidents and failing monitors holding it
// set, unless a member set a worse one. A component nothing holds goes back to operational when restore
// names it or an incident or monitor set its current state; otherwise a state a member set stays.
func (s *Server) settleComponents(ctx context.Context, q *store.Queries, ev *eventBatch, pg store.StatusPage, ids []uuid.UUID,
	restore map[uuid.UUID]bool, closer statusActor, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	if closer.source == "" {
		closer.source = statusSourceIncident
	}
	ws := pg.WorkspaceID
	rows, err := q.OpenStatusComponentStates(ctx, store.OpenStatusComponentStatesParams{WorkspaceID: ws, ComponentIds: ids})
	if err != nil {
		return err
	}
	open := map[uuid.UUID]store.StatusComponentState{}
	for _, o := range rows {
		open[o.ComponentID] = o
	}
	holders, err := statusHolders(ctx, q, ws, ids)
	if err != nil {
		return err
	}
	sorted := slices.Clone(ids)
	slices.SortFunc(sorted, func(a, b uuid.UUID) int { return a.Compare(b) })
	for _, id := range slices.Compact(sorted) {
		var top *statusHolder
		for i := range holders {
			if h := &holders[i]; h.component == id && (top == nil || stateRank(h.state) > stateRank(top.state)) {
				top = h
			}
		}
		auto := automaticSource(open[id].Source)
		target, source, by := stOperational, closer.source, closer.id
		if top == nil {
			if !restore[id] && !auto {
				continue
			}
		} else {
			hid := top.id
			target, source, by = top.state, top.source, &hid
		}
		c, err := lockedStatusComponent(ctx, q, pg, id)
		if err == errStatusComponentGone {
			continue
		}
		if err != nil {
			return err
		}
		if top != nil && !auto && stateRank(c.State) > stateRank(target) {
			continue
		}
		if top != nil && c.State == target && !auto {
			if err := q.EndStatusComponentState(ctx, store.EndStatusComponentStateParams{WorkspaceID: ws, ComponentID: id, Now: now}); err != nil {
				return err
			}
			if _, err := q.StartStatusComponentState(ctx, store.StartStatusComponentStateParams{
				ID: newID(), WorkspaceID: ws, ComponentID: id, State: target, Source: source, SourceID: by, Now: now,
			}); err != nil {
				return err
			}
			continue
		}
		if _, err := s.setComponentState(ctx, q, ev, pg, c, target, source, by, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) incidentEvent(ctx context.Context, q *store.Queries, ev *eventBatch, pg store.StatusPage, inc store.StatusIncident,
	change oas.StatusIncidentChange, update *store.StatusIncidentUpdate) (oas.StatusIncident, error) {
	body, err := s.incidentOut(ctx, q, pg, inc)
	if err != nil {
		return body, err
	}
	data := oas.StatusIncidentEventData{Incident: body, Page: statusPageRef(pg), Change: change}
	if update != nil {
		u := updateBody(*update)
		data.Update = &u
	}
	typ := statusEventUpdated
	switch change {
	case oas.StatusChangeCreated:
		typ = statusEventCreated
	case oas.StatusChangeResolved:
		typ = statusEventResolved
	}
	inbox := pg.InboxID
	ev.add(typ, &inbox, nil, data)
	if err := q.TouchStatusPage(ctx, store.TouchStatusPageParams{WorkspaceID: pg.WorkspaceID, ID: pg.ID, Now: s.now()}); err != nil {
		return body, err
	}
	return body, nil
}

func defaultIncidentState(kind string) string {
	if kind == kindMaintenance {
		return stMaintenance
	}
	return stMajorOutage
}

func incidentComponents(ctx context.Context, q *store.Queries, pg store.StatusPage, kind string, in []oas.StatusIncidentComponentInput) ([]store.StatusIncidentComponent, error) {
	var out []store.StatusIncidentComponent
	var ids []uuid.UUID
	for _, c := range in {
		if slices.Contains(ids, c.ComponentId) {
			return nil, errValidation("components names a component twice")
		}
		st := defaultIncidentState(kind)
		if c.State != nil {
			if st = string(*c.State); !validComponentState(st) {
				return nil, errValidation("components state must be a component state")
			}
		}
		ids = append(ids, c.ComponentId)
		out = append(out, store.StatusIncidentComponent{WorkspaceID: pg.WorkspaceID, ComponentID: c.ComponentId, State: st})
	}
	if len(ids) > maxStatusComponents {
		return nil, errValidation(statusComponentsLimit)
	}
	found, err := q.ListStatusComponentsByIDs(ctx, store.ListStatusComponentsByIDsParams{WorkspaceID: pg.WorkspaceID, PageID: pg.ID, Ids: ids})
	if err != nil {
		return nil, err
	}
	if len(found) != len(ids) {
		return nil, errValidation("components names a component that is not on this page")
	}
	return out, nil
}

func writeIncidentComponents(ctx context.Context, q *store.Queries, inc store.StatusIncident, comps []store.StatusIncidentComponent) error {
	if err := q.ClearStatusIncidentComponents(ctx, store.ClearStatusIncidentComponentsParams{WorkspaceID: inc.WorkspaceID, IncidentID: inc.ID}); err != nil {
		return err
	}
	for _, c := range comps {
		if err := q.AddStatusIncidentComponent(ctx, store.AddStatusIncidentComponentParams{
			WorkspaceID: inc.WorkspaceID, IncidentID: inc.ID, ComponentID: c.ComponentID, State: c.State,
		}); err != nil {
			return err
		}
	}
	return nil
}

func statusAuthor(p principal) (member, key *uuid.UUID) {
	if p.isKey() {
		id := p.keyID
		return nil, &id
	}
	if p.memberID == uuid.Nil() {
		return nil, nil
	}
	id := p.memberID
	return &id, nil
}

func checkWindow(start, end *time.Time) error {
	if start == nil || end == nil {
		return errValidation("maintenance needs scheduled_start and scheduled_end")
	}
	if !end.After(*start) {
		return errValidation("scheduled_end must be after scheduled_start")
	}
	return nil
}

func (s *Server) ListStatusIncidents(ctx context.Context, req oas.ListStatusIncidentsRequestObject) (oas.ListStatusIncidentsResponseObject, error) {
	p := principalFrom(ctx)
	pg, err := visibleStatusPage(ctx, s.st.Queries, p, req.PageId, false)
	if err != nil {
		return nil, err
	}
	limit, err := pageSize(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	at, id, err := decodeCursor(req.Params.Cursor)
	if err != nil {
		return nil, err
	}
	var kind *string
	if req.Params.Kind != nil {
		k := string(*req.Params.Kind)
		if k != kindIncident && k != kindMaintenance {
			return nil, errValidation("kind must be incident or maintenance")
		}
		kind = &k
	}
	rows, err := s.st.ListStatusIncidents(ctx, store.ListStatusIncidentsParams{
		WorkspaceID: p.workspaceID, PageID: pg.ID, Kind: kind, OpenOnly: req.Params.Open != nil && *req.Params.Open,
		CursorAt: at, CursorID: id, Lim: limit + 1,
	})
	if err != nil {
		return nil, err
	}
	out := oas.ListStatusIncidents200JSONResponse{Items: []oas.StatusIncident{}}
	if len(rows) > int(limit) {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		c := encodeCursor(last.CreatedAt, last.ID)
		out.NextCursor = &c
	}
	parts, err := loadIncidentParts(ctx, s.st.Queries, p.workspaceID, rows)
	if err != nil {
		return nil, err
	}
	for _, inc := range rows {
		out.Items = append(out.Items, s.incidentBody(pg, inc, parts))
	}
	return out, nil
}

func (s *Server) CreateStatusIncident(ctx context.Context, req oas.CreateStatusIncidentRequestObject) (oas.CreateStatusIncidentResponseObject, error) {
	p := principalFrom(ctx)
	b := req.Body
	kind := kindIncident
	if b.Kind != nil {
		kind = string(*b.Kind)
	}
	if kind != kindIncident && kind != kindMaintenance {
		return nil, errValidation("kind must be incident or maintenance")
	}
	title, err := trimmed(b.Title, 1, 200, "title")
	if err != nil {
		return nil, err
	}
	message, err := statusText(b.Message)
	if err != nil {
		return nil, err
	}
	impact := string(oas.StatusImpactMinor)
	if kind == kindMaintenance {
		impact = string(oas.StatusImpactNone)
	}
	if b.Impact != nil {
		if impact = string(*b.Impact); !b.Impact.Valid() {
			return nil, errValidation("impact must be none, minor, major or critical")
		}
	}
	now := s.now()
	state := incInvestigating
	if kind == kindMaintenance {
		if err := checkWindow(b.ScheduledStart, b.ScheduledEnd); err != nil {
			return nil, err
		}
		state = incScheduled
		if !b.ScheduledStart.After(now) {
			state = incInProgress
		}
	} else if b.ScheduledStart != nil || b.ScheduledEnd != nil {
		return nil, errValidation("only maintenance has a scheduled window")
	}
	if b.State != nil {
		want := string(*b.State)
		if err := checkIncidentMove(kind, state, want); err != nil {
			return nil, err
		}
		if incidentClosed(want) {
			return nil, errValidation("a new incident cannot be " + want)
		}
		state = want
	}
	publish := b.Publish != nil && *b.Publish
	notify := b.Notify == nil || *b.Notify
	var out oas.StatusIncident
	err = s.inTx(ctx, p.workspaceID, func(q *store.Queries, ev *eventBatch) error {
		pg, err := visibleStatusPage(ctx, q, p, req.PageId, true)
		if err != nil {
			return err
		}
		var in []oas.StatusIncidentComponentInput
		if b.Components != nil {
			in = *b.Components
		}
		comps, err := incidentComponents(ctx, q, pg, kind, in)
		if err != nil {
			return err
		}
		arg := store.CreateStatusIncidentParams{
			ID: newID(), WorkspaceID: p.workspaceID, PageID: pg.ID, Kind: kind, Title: title, Impact: impact, State: state,
			ScheduledStart: b.ScheduledStart, ScheduledEnd: b.ScheduledEnd, CreatedBy: p.creator(), Notify: notify, Now: now,
		}
		if publish {
			arg.PublishedAt = &now
		}
		if state == incInProgress {
			arg.StartedAt = &now
		}
		inc, err := q.CreateStatusIncident(ctx, arg)
		if err != nil {
			return err
		}
		if err := writeIncidentComponents(ctx, q, inc, comps); err != nil {
			return err
		}
		member, key := statusAuthor(p)
		up, err := q.CreateStatusIncidentUpdate(ctx, store.CreateStatusIncidentUpdateParams{
			ID: newID(), WorkspaceID: p.workspaceID, IncidentID: inc.ID, State: state, Body: message, AuthorMemberID: member, AuthorKeyID: key, Notify: true, Now: now,
		})
		if err != nil {
			return err
		}
		if incidentHolds(publish, state) {
			ids, err := incidentComponentIDs(ctx, q, inc)
			if err != nil {
				return err
			}
			if err := s.settleComponents(ctx, q, ev, pg, ids, nil, statusActor{}, now); err != nil {
				return err
			}
		}
		queueMaintenance(ev, inc, now)
		if notify {
			if err := s.queueNotice(ctx, q, ev, pg, inc, &up.ID); err != nil {
				return err
			}
		}
		out, err = s.incidentEvent(ctx, q, ev, pg, inc, oas.StatusChangeCreated, &up)
		return err
	})
	if err != nil {
		return nil, err
	}
	return oas.CreateStatusIncident201JSONResponse(out), nil
}

func (s *Server) GetStatusIncident(ctx context.Context, req oas.GetStatusIncidentRequestObject) (oas.GetStatusIncidentResponseObject, error) {
	p := principalFrom(ctx)
	pg, err := visibleStatusPage(ctx, s.st.Queries, p, req.PageId, false)
	if err != nil {
		return nil, err
	}
	inc, err := s.st.GetStatusIncident(ctx, store.GetStatusIncidentParams{WorkspaceID: p.workspaceID, PageID: pg.ID, ID: req.IncidentId})
	if store.IsNotFound(err) {
		return nil, errStatusIncidentGone
	}
	if err != nil {
		return nil, err
	}
	out, err := s.incidentOut(ctx, s.st.Queries, pg, inc)
	if err != nil {
		return nil, err
	}
	return oas.GetStatusIncident200JSONResponse(out), nil
}

func incidentUpdateArg(inc store.StatusIncident, now time.Time) store.UpdateStatusIncidentParams {
	return store.UpdateStatusIncidentParams{
		WorkspaceID: inc.WorkspaceID, ID: inc.ID, Title: inc.Title, Impact: inc.Impact, State: inc.State, ScheduledStart: inc.ScheduledStart,
		ScheduledEnd: inc.ScheduledEnd, PublishedAt: inc.PublishedAt, StartedAt: inc.StartedAt, ResolvedAt: inc.ResolvedAt, Now: now,
	}
}

func (s *Server) UpdateStatusIncident(ctx context.Context, req oas.UpdateStatusIncidentRequestObject) (oas.UpdateStatusIncidentResponseObject, error) {
	p := principalFrom(ctx)
	b := req.Body
	var out oas.StatusIncident
	err := s.inTx(ctx, p.workspaceID, func(q *store.Queries, ev *eventBatch) error {
		pg, err := visibleStatusPage(ctx, q, p, req.PageId, true)
		if err != nil {
			return err
		}
		inc, err := lockedStatusIncident(ctx, q, pg, req.IncidentId)
		if err != nil {
			return err
		}
		if incidentClosed(inc.State) {
			return errIncidentClosed
		}
		now := s.now()
		arg := incidentUpdateArg(inc, now)
		if b.Title != nil {
			if arg.Title, err = trimmed(*b.Title, 1, 200, "title"); err != nil {
				return err
			}
		}
		if b.Impact != nil {
			if !b.Impact.Valid() {
				return errValidation("impact must be none, minor, major or critical")
			}
			arg.Impact = string(*b.Impact)
		}
		windowChanged := b.ScheduledStart != nil || b.ScheduledEnd != nil
		if windowChanged {
			if inc.Kind != kindMaintenance {
				return errValidation("only maintenance has a scheduled window")
			}
			if b.ScheduledStart != nil {
				if inc.State != incScheduled && !b.ScheduledStart.Equal(*inc.ScheduledStart) {
					return errValidation("scheduled_start cannot change once maintenance has started")
				}
				arg.ScheduledStart = b.ScheduledStart
			}
			if b.ScheduledEnd != nil {
				arg.ScheduledEnd = b.ScheduledEnd
			}
			if err := checkWindow(arg.ScheduledStart, arg.ScheduledEnd); err != nil {
				return err
			}
		}
		touched, err := incidentComponentIDs(ctx, q, inc)
		if err != nil {
			return err
		}
		removed := map[uuid.UUID]bool{}
		if b.Components != nil {
			comps, err := incidentComponents(ctx, q, pg, inc.Kind, *b.Components)
			if err != nil {
				return err
			}
			for _, id := range touched {
				if !slices.ContainsFunc(comps, func(c store.StatusIncidentComponent) bool { return c.ComponentID == id }) {
					removed[id] = true
				}
			}
			if err := writeIncidentComponents(ctx, q, inc, comps); err != nil {
				return err
			}
			for _, c := range comps {
				touched = append(touched, c.ComponentID)
			}
		}
		if inc, err = q.UpdateStatusIncident(ctx, arg); err != nil {
			return err
		}
		if b.Components != nil && incidentHolds(inc.PublishedAt != nil, inc.State) {
			if err := s.settleComponents(ctx, q, ev, pg, touched, removed, statusActor{statusSourceIncident, &inc.ID}, now); err != nil {
				return err
			}
		}
		if windowChanged {
			queueMaintenance(ev, inc, now)
			if b.Notify == nil || *b.Notify {
				if err := s.queueNotice(ctx, q, ev, pg, inc, nil); err != nil {
					return err
				}
			}
		}
		out, err = s.incidentEvent(ctx, q, ev, pg, inc, oas.StatusChangeChanged, nil)
		return err
	})
	if err != nil {
		return nil, err
	}
	return oas.UpdateStatusIncident200JSONResponse(out), nil
}

func (s *Server) DeleteStatusIncident(ctx context.Context, req oas.DeleteStatusIncidentRequestObject) (oas.DeleteStatusIncidentResponseObject, error) {
	p := principalFrom(ctx)
	err := s.inTx(ctx, p.workspaceID, func(q *store.Queries, _ *eventBatch) error {
		pg, err := visibleStatusPage(ctx, q, p, req.PageId, true)
		if err != nil {
			return err
		}
		inc, err := lockedStatusIncident(ctx, q, pg, req.IncidentId)
		if err != nil {
			return err
		}
		if inc.PublishedAt != nil {
			return errIncidentPublished
		}
		_, err = q.DeleteStatusIncident(ctx, store.DeleteStatusIncidentParams{WorkspaceID: p.workspaceID, PageID: pg.ID, ID: inc.ID})
		return err
	})
	if err != nil {
		return nil, err
	}
	return oas.DeleteStatusIncident204Response{}, nil
}

func (s *Server) PublishStatusIncident(ctx context.Context, req oas.PublishStatusIncidentRequestObject) (oas.PublishStatusIncidentResponseObject, error) {
	p := principalFrom(ctx)
	var out oas.StatusIncident
	err := s.inTx(ctx, p.workspaceID, func(q *store.Queries, ev *eventBatch) error {
		pg, err := visibleStatusPage(ctx, q, p, req.PageId, true)
		if err != nil {
			return err
		}
		inc, err := lockedStatusIncident(ctx, q, pg, req.IncidentId)
		if err != nil {
			return err
		}
		if inc.PublishedAt != nil {
			out, err = s.incidentOut(ctx, q, pg, inc)
			return err
		}
		now := s.now()
		arg := incidentUpdateArg(inc, now)
		arg.PublishedAt = &now
		if inc, err = q.UpdateStatusIncident(ctx, arg); err != nil {
			return err
		}
		if incidentHolds(true, inc.State) {
			ids, err := incidentComponentIDs(ctx, q, inc)
			if err != nil {
				return err
			}
			if err := s.settleComponents(ctx, q, ev, pg, ids, nil, statusActor{}, now); err != nil {
				return err
			}
		}
		queueMaintenance(ev, inc, now)
		notify := inc.Notify
		if req.Body != nil && req.Body.Notify != nil {
			notify = *req.Body.Notify
		}
		if notify {
			ups, err := q.ListStatusIncidentUpdates(ctx, store.ListStatusIncidentUpdatesParams{WorkspaceID: inc.WorkspaceID, IncidentIds: []uuid.UUID{inc.ID}})
			if err != nil {
				return err
			}
			var latest *uuid.UUID
			if i := slices.IndexFunc(ups, func(u store.StatusIncidentUpdate) bool { return u.Notify }); i >= 0 {
				latest = &ups[i].ID
			}
			if err := s.queueNotice(ctx, q, ev, pg, inc, latest); err != nil {
				return err
			}
		}
		out, err = s.incidentEvent(ctx, q, ev, pg, inc, oas.StatusChangePublished, nil)
		return err
	})
	if err != nil {
		return nil, err
	}
	return oas.PublishStatusIncident200JSONResponse(out), nil
}

func (s *Server) moveIncident(ctx context.Context, q *store.Queries, ev *eventBatch, pg store.StatusPage, inc store.StatusIncident,
	state, body string, member, key *uuid.UUID, change oas.StatusIncidentChange, notify bool, now time.Time) (oas.StatusIncident, error) {
	return s.moveIncidentWith(ctx, q, ev, pg, inc, state, body, member, key, change, true, notify, now)
}

// Closing the incident puts its components back to operational when restoreAll is set; without it only those an incident or monitor set go back.
func (s *Server) moveIncidentWith(ctx context.Context, q *store.Queries, ev *eventBatch, pg store.StatusPage, inc store.StatusIncident,
	state, body string, member, key *uuid.UUID, change oas.StatusIncidentChange, restoreAll, notify bool, now time.Time) (oas.StatusIncident, error) {
	held := incidentHolds(inc.PublishedAt != nil, inc.State)
	arg := incidentUpdateArg(inc, now)
	arg.State = state
	if state == incInProgress && inc.StartedAt == nil {
		arg.StartedAt = &now
	}
	if incidentClosed(state) {
		arg.ResolvedAt = &now
		change = oas.StatusChangeResolved
	}
	inc, err := q.UpdateStatusIncident(ctx, arg)
	if err != nil {
		return oas.StatusIncident{}, err
	}
	up, err := q.CreateStatusIncidentUpdate(ctx, store.CreateStatusIncidentUpdateParams{
		ID: newID(), WorkspaceID: inc.WorkspaceID, IncidentID: inc.ID, State: state, Body: body, AuthorMemberID: member, AuthorKeyID: key, Notify: notify, Now: now,
	})
	if err != nil {
		return oas.StatusIncident{}, err
	}
	if holds := incidentHolds(inc.PublishedAt != nil, inc.State); holds != held {
		ids, err := incidentComponentIDs(ctx, q, inc)
		if err != nil {
			return oas.StatusIncident{}, err
		}
		var restore map[uuid.UUID]bool
		if !holds && restoreAll {
			restore = map[uuid.UUID]bool{}
			for _, id := range ids {
				restore[id] = true
			}
		}
		if err := s.settleComponents(ctx, q, ev, pg, ids, restore, statusActor{statusSourceIncident, &inc.ID}, now); err != nil {
			return oas.StatusIncident{}, err
		}
	}
	if notify {
		if err := s.queueNotice(ctx, q, ev, pg, inc, &up.ID); err != nil {
			return oas.StatusIncident{}, err
		}
	}
	return s.incidentEvent(ctx, q, ev, pg, inc, change, &up)
}

func (s *Server) CreateStatusIncidentUpdate(ctx context.Context, req oas.CreateStatusIncidentUpdateRequestObject) (oas.CreateStatusIncidentUpdateResponseObject, error) {
	p := principalFrom(ctx)
	body, err := statusText(req.Body.Body)
	if err != nil {
		return nil, err
	}
	var out oas.StatusIncident
	err = s.inTx(ctx, p.workspaceID, func(q *store.Queries, ev *eventBatch) error {
		pg, err := visibleStatusPage(ctx, q, p, req.PageId, true)
		if err != nil {
			return err
		}
		inc, err := lockedStatusIncident(ctx, q, pg, req.IncidentId)
		if err != nil {
			return err
		}
		state := inc.State
		if req.Body.State != nil {
			state = string(*req.Body.State)
		}
		if err := checkIncidentMove(inc.Kind, inc.State, state); err != nil {
			return err
		}
		notify := state != inc.State
		if req.Body.Notify != nil {
			notify = *req.Body.Notify
		}
		member, key := statusAuthor(p)
		out, err = s.moveIncident(ctx, q, ev, pg, inc, state, body, member, key, oas.StatusChangeUpdatePosted, notify, s.now())
		return err
	})
	if err != nil {
		return nil, err
	}
	return oas.CreateStatusIncidentUpdate201JSONResponse(out), nil
}

func (s *Server) EditStatusIncidentUpdate(ctx context.Context, req oas.EditStatusIncidentUpdateRequestObject) (oas.EditStatusIncidentUpdateResponseObject, error) {
	p := principalFrom(ctx)
	body, err := statusText(req.Body.Body)
	if err != nil {
		return nil, err
	}
	var out oas.StatusIncident
	err = s.inTx(ctx, p.workspaceID, func(q *store.Queries, ev *eventBatch) error {
		pg, err := visibleStatusPage(ctx, q, p, req.PageId, true)
		if err != nil {
			return err
		}
		inc, err := lockedStatusIncident(ctx, q, pg, req.IncidentId)
		if err != nil {
			return err
		}
		now := s.now()
		var edited *time.Time
		if inc.PublishedAt != nil {
			edited = &now
		}
		up, err := q.EditStatusIncidentUpdate(ctx, store.EditStatusIncidentUpdateParams{WorkspaceID: p.workspaceID, IncidentID: inc.ID, ID: req.UpdateId, Body: body, EditedAt: edited})
		if store.IsNotFound(err) {
			return errStatusUpdateGone
		}
		if err != nil {
			return err
		}
		out, err = s.incidentEvent(ctx, q, ev, pg, inc, oas.StatusChangeUpdateEdited, &up)
		return err
	})
	if err != nil {
		return nil, err
	}
	return oas.EditStatusIncidentUpdate200JSONResponse(out), nil
}

// RunStatusMaintenance starts published maintenance whose window has begun and completes it once
// the window has ended, unless a member did it first.
func (s *Server) RunStatusMaintenance(ctx context.Context, a StatusMaintenanceArgs) error {
	found, err := s.st.ListStatusIncidentsByIDs(ctx, store.ListStatusIncidentsByIDsParams{WorkspaceID: a.WorkspaceID, Ids: []uuid.UUID{a.IncidentID}})
	if err != nil || len(found) == 0 {
		return err
	}
	return s.inTx(ctx, a.WorkspaceID, func(q *store.Queries, ev *eventBatch) error {
		pg, err := q.LockStatusPage(ctx, store.LockStatusPageParams{WorkspaceID: a.WorkspaceID, ID: found[0].PageID})
		if store.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		inc, err := lockedStatusIncident(ctx, q, pg, a.IncidentID)
		if err == errStatusIncidentGone {
			return nil
		}
		if err != nil {
			return err
		}
		if inc.Kind != kindMaintenance || inc.PublishedAt == nil {
			return nil
		}
		now := s.now()
		words := statusWordsFor(pg.Locale)
		if inc.State == incScheduled && !inc.ScheduledStart.After(now) {
			if _, err := s.moveIncident(ctx, q, ev, pg, inc, incInProgress, words.MaintenanceStarted, nil, nil, oas.StatusChangeStarted, inc.Notify, now); err != nil {
				return err
			}
			if inc, err = lockedStatusIncident(ctx, q, pg, a.IncidentID); err != nil {
				return err
			}
		}
		if inc.State == incInProgress && !inc.ScheduledEnd.After(now) {
			_, err := s.moveIncident(ctx, q, ev, pg, inc, incCompleted, words.MaintenanceDone, nil, nil, oas.StatusChangeResolved, inc.Notify, now)
			return err
		}
		return nil
	})
}

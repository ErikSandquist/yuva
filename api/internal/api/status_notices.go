package api

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/productdevbook/yuva/api/internal/email"
	"github.com/productdevbook/yuva/api/internal/jobs"
	"github.com/productdevbook/yuva/api/internal/mail"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/realtime"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	noticeHold           = 60 * time.Second
	noticeGap            = 15 * time.Minute
	noticeRateWindow     = time.Hour
	noticeRateMax        = 10
	noticeStale          = 6 * time.Hour
	noticeLease          = 5 * time.Minute
	noticeBudget         = 4 * time.Minute
	noticeSnapshotPage   = 1000
	noticeBatch          = 50
	noticeDeliveriesKept = 90 * 24 * time.Hour
	noticeFeedbackID     = "yuvastatus"
	eventStatusNotice    = string(oas.StatusNoticeUpdated)

	noticePending   = string(oas.StatusNoticePending)
	noticeSending   = string(oas.StatusNoticeSending)
	noticeSent      = string(oas.StatusNoticeSent)
	noticeSkipped   = string(oas.StatusNoticeSkipped)
	noticeCancelled = string(oas.StatusNoticeCancelled)
)

var (
	noticeRetry         = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute}
	errNoticeNotPending = problem(http.StatusConflict, "not_pending", "the notice is no longer waiting: it is sending or done")
	errNoticeGone       = problem(http.StatusNotFound, "not_found", "no such notice")
	errNoticeNoSend     = errors.New("the channel cannot send")
)

type StatusNoticeArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	NoticeID    uuid.UUID `json:"notice_id"`
}

func (StatusNoticeArgs) Kind() string { return "status_notice" }

func noticeJobOpts(at time.Time) *river.InsertOpts {
	return &river.InsertOpts{Queue: jobs.StatusNoticeQueue, ScheduledAt: at, MaxAttempts: 5}
}

type statusNoticeWorker struct {
	river.WorkerDefaults[StatusNoticeArgs]
	s *Server
}

func (w *statusNoticeWorker) Work(ctx context.Context, job *river.Job[StatusNoticeArgs]) error {
	return w.s.RunStatusNotice(ctx, job.Args.WorkspaceID, job.Args.NoticeID, noticeBudget)
}

func (w *statusNoticeWorker) Timeout(*river.Job[StatusNoticeArgs]) time.Duration {
	return noticeBudget + 2*time.Minute
}

type statusNoticeSweepWorker struct {
	river.WorkerDefaults[jobs.StatusNoticeSweepArgs]
	s *Server
}

func (w *statusNoticeSweepWorker) Work(ctx context.Context, _ *river.Job[jobs.StatusNoticeSweepArgs]) error {
	return w.s.SweepStatusNotices(ctx)
}

func noticeCounts(raw []byte) oas.StatusNoticeCounts {
	var c oas.StatusNoticeCounts
	_ = json.Unmarshal(raw, &c)
	return c
}

func noticeBody(n store.StatusNotice, updates []uuid.UUID) oas.StatusNotice {
	if updates == nil {
		updates = []uuid.UUID{}
	}
	out := oas.StatusNotice{
		Id: n.ID, PageId: n.PageID, IncidentId: n.IncidentID, State: oas.StatusNoticeState(n.State), Final: n.Final, DueAt: n.DueAt,
		Counts: noticeCounts(n.Counts), UpdateIds: updates, StartedAt: n.StartedAt, FinishedAt: n.FinishedAt, CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
	if n.SkipReason != nil {
		r := oas.StatusNoticeSkipReason(*n.SkipReason)
		out.SkipReason = &r
	}
	return out
}

func noticeBodies(ctx context.Context, q *store.Queries, ws uuid.UUID, rows []store.StatusNotice) ([]oas.StatusNotice, error) {
	ids := make([]uuid.UUID, len(rows))
	for i, n := range rows {
		ids[i] = n.ID
	}
	ups, err := q.ListStatusNoticeUpdateIDs(ctx, store.ListStatusNoticeUpdateIDsParams{WorkspaceID: ws, NoticeIds: ids})
	if err != nil {
		return nil, err
	}
	byNotice := map[uuid.UUID][]uuid.UUID{}
	for _, u := range ups {
		byNotice[u.NoticeID] = append(byNotice[u.NoticeID], u.ID)
	}
	out := make([]oas.StatusNotice, len(rows))
	for i, n := range rows {
		out[i] = noticeBody(n, byNotice[n.ID])
	}
	return out, nil
}

// noticeSignal tells /v1/realtime about a notice. It goes through the transaction's connection, so
// Postgres delivers it only when the change commits.
func (s *Server) noticeSignal(ctx context.Context, q *store.Queries, inboxID uuid.UUID, n store.StatusNotice) {
	bodies, err := noticeBodies(ctx, q, n.WorkspaceID, []store.StatusNotice{n})
	if err == nil {
		inbox := inboxID
		var payload string
		payload, err = realtime.SignalPayload(realtime.Event{Type: eventStatusNotice, WorkspaceID: n.WorkspaceID, InboxID: &inbox, CreatedAt: s.now(),
			Data: mustJSON(bodies[0])})
		if err == nil {
			err = q.NotifySignal(ctx, store.NotifySignalParams{Channel: realtime.SignalChannel, Payload: payload})
		}
	}
	if err != nil {
		s.log.WarnContext(ctx, "status notice signal", slog.Any("error", err))
	}
}

func finalNotice(inc store.StatusIncident) bool {
	return incidentClosed(inc.State) || (inc.Kind == kindMaintenance && inc.State == incInProgress)
}

func (s *Server) queueNotice(ctx context.Context, q *store.Queries, ev *eventBatch, pg store.StatusPage, inc store.StatusIncident, update *uuid.UUID) error {
	if s.status.NoticesOff || !pg.EmailEnabled || pg.MailingListID == nil || !pg.Published || inc.PublishedAt == nil {
		return nil
	}
	now := s.now()
	final := finalNotice(inc)
	n, err := q.LockPendingStatusNotice(ctx, store.LockPendingStatusNoticeParams{WorkspaceID: inc.WorkspaceID, IncidentID: inc.ID})
	switch {
	case err == nil:
		if final && !n.Final {
			due := n.DueAt
			if hold := now.Add(noticeHold); hold.Before(due) {
				due = hold
			}
			if n, err = q.MoveStatusNotice(ctx, store.MoveStatusNoticeParams{WorkspaceID: n.WorkspaceID, ID: n.ID, Final: true, DueAt: due, Now: now}); err != nil {
				return err
			}
			ev.job(StatusNoticeArgs{WorkspaceID: n.WorkspaceID, NoticeID: n.ID}, noticeJobOpts(due))
		}
	case store.IsNotFound(err):
		due := now.Add(noticeHold)
		if !final {
			last, err := q.LastStartedStatusNotice(ctx, store.LastStartedStatusNoticeParams{WorkspaceID: inc.WorkspaceID, IncidentID: inc.ID})
			if err != nil && !store.IsNotFound(err) {
				return err
			}
			if err == nil && last.Add(noticeGap).After(due) {
				due = last.Add(noticeGap)
			}
		}
		if n, err = q.CreateStatusNotice(ctx, store.CreateStatusNoticeParams{
			ID: newID(), WorkspaceID: inc.WorkspaceID, PageID: pg.ID, IncidentID: inc.ID, Final: final, DueAt: due, Now: now,
		}); err != nil {
			return err
		}
		ev.job(StatusNoticeArgs{WorkspaceID: n.WorkspaceID, NoticeID: n.ID}, noticeJobOpts(due))
	default:
		return err
	}
	if update != nil {
		if err := q.AttachStatusNoticeUpdate(ctx, store.AttachStatusNoticeUpdateParams{WorkspaceID: inc.WorkspaceID, ID: *update, NoticeID: &n.ID}); err != nil {
			return err
		}
	}
	s.noticeSignal(ctx, q, pg.InboxID, n)
	return nil
}

func (s *Server) ListStatusNotices(ctx context.Context, req oas.ListStatusNoticesRequestObject) (oas.ListStatusNoticesResponseObject, error) {
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
	var state *string
	if req.Params.State != nil {
		if !req.Params.State.Valid() {
			return nil, errValidation("state must be pending, sending, sent, skipped or cancelled")
		}
		state = new(string(*req.Params.State))
	}
	rows, err := s.st.ListStatusNotices(ctx, store.ListStatusNoticesParams{
		WorkspaceID: p.workspaceID, PageID: pg.ID, IncidentID: req.Params.IncidentId, State: state, BeforeAt: at, BeforeID: id, Lim: limit + 1,
	})
	if err != nil {
		return nil, err
	}
	out := oas.ListStatusNotices200JSONResponse{}
	if len(rows) > int(limit) {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		c := encodeCursor(last.CreatedAt, last.ID)
		out.NextCursor = &c
	}
	if out.Items, err = noticeBodies(ctx, s.st.Queries, p.workspaceID, rows); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Server) GetStatusNotice(ctx context.Context, req oas.GetStatusNoticeRequestObject) (oas.GetStatusNoticeResponseObject, error) {
	p := principalFrom(ctx)
	pg, err := visibleStatusPage(ctx, s.st.Queries, p, req.PageId, false)
	if err != nil {
		return nil, err
	}
	n, err := s.st.GetStatusNotice(ctx, store.GetStatusNoticeParams{WorkspaceID: p.workspaceID, PageID: pg.ID, ID: req.NoticeId})
	if store.IsNotFound(err) {
		return nil, errNoticeGone
	}
	if err != nil {
		return nil, err
	}
	bodies, err := noticeBodies(ctx, s.st.Queries, p.workspaceID, []store.StatusNotice{n})
	if err != nil {
		return nil, err
	}
	return oas.GetStatusNotice200JSONResponse(bodies[0]), nil
}

func (s *Server) CancelStatusNotice(ctx context.Context, req oas.CancelStatusNoticeRequestObject) (oas.CancelStatusNoticeResponseObject, error) {
	p := principalFrom(ctx)
	var out oas.StatusNotice
	err := s.inTx(ctx, p.workspaceID, func(q *store.Queries, _ *eventBatch) error {
		pg, err := visibleStatusPage(ctx, q, p, req.PageId, true)
		if err != nil {
			return err
		}
		n, err := q.CancelStatusNotice(ctx, store.CancelStatusNoticeParams{WorkspaceID: p.workspaceID, PageID: pg.ID, ID: req.NoticeId, Now: s.now()})
		if store.IsNotFound(err) {
			if _, err := q.GetStatusNotice(ctx, store.GetStatusNoticeParams{WorkspaceID: p.workspaceID, PageID: pg.ID, ID: req.NoticeId}); err != nil {
				if store.IsNotFound(err) {
					return errNoticeGone
				}
				return err
			}
			return errNoticeNotPending
		}
		if err != nil {
			return err
		}
		bodies, err := noticeBodies(ctx, q, p.workspaceID, []store.StatusNotice{n})
		if err != nil {
			return err
		}
		out = bodies[0]
		s.noticeSignal(ctx, q, pg.InboxID, n)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return oas.CancelStatusNotice200JSONResponse(out), nil
}

func (s *Server) GetStatusNoticeReach(ctx context.Context, req oas.GetStatusNoticeReachRequestObject) (oas.GetStatusNoticeReachResponseObject, error) {
	p := principalFrom(ctx)
	pg, err := visibleStatusPage(ctx, s.st.Queries, p, req.PageId, false)
	if err != nil {
		return nil, err
	}
	out := oas.GetStatusNoticeReach200JSONResponse{}
	if pg.MailingListID == nil {
		return out, nil
	}
	var comps []uuid.UUID
	if req.Params.ComponentId != nil {
		comps = *req.Params.ComponentId
	}
	if len(comps) > 100 {
		return nil, errValidation("component_id names at most 100 components")
	}
	n, err := s.st.CountStatusNoticeReach(ctx, store.CountStatusNoticeReachParams{WorkspaceID: p.workspaceID, ListID: *pg.MailingListID, ComponentIds: comps})
	if err != nil {
		return nil, err
	}
	out.Subscribers = n
	return out, nil
}

// It reads across workspaces; each job then works in the notice's own workspace.
func (s *Server) SweepStatusNotices(ctx context.Context) error {
	rows, err := s.st.DueStatusNotices(ctx, s.now())
	if err != nil || len(rows) == 0 {
		return err
	}
	batch := make([]river.InsertManyParams, len(rows))
	for i, r := range rows {
		batch[i] = river.InsertManyParams{Args: StatusNoticeArgs{WorkspaceID: r.WorkspaceID, NoticeID: r.ID}, InsertOpts: noticeJobOpts(time.Time{})}
	}
	_, err = s.jobs.InsertMany(ctx, batch)
	return err
}

// noticeSend is what sending one notice needs, read when the job runs so a corrected update is
// mailed as corrected.
type noticeSend struct {
	n       store.StatusNotice
	pg      store.StatusPage
	inc     store.StatusIncident
	inbox   store.Inbox
	list    store.MailingList
	ch      store.EmailChannel
	loc     *time.Location
	comps   []store.StatusIncidentComponent
	names   map[uuid.UUID]string
	current map[uuid.UUID]string
	updates []store.StatusIncidentUpdate

	cfg                             email.SMTPConfig
	from, fromName, replyTo, domain string
	dkim                            *email.DKIMKey
	ses                             []email.Header
	data                            map[string]map[string]any
}

func (s *Server) noticeChannel(ctx context.Context, q *store.Queries, pg store.StatusPage) (store.MailingList, store.EmailChannel, error) {
	l, err := q.GetMailingList(ctx, store.GetMailingListParams{WorkspaceID: pg.WorkspaceID, ID: *pg.MailingListID})
	if err != nil {
		return l, store.EmailChannel{}, err
	}
	if l.ChannelID == nil {
		return l, store.EmailChannel{}, errNoticeNoSend
	}
	ch, err := q.GetEmailChannel(ctx, store.GetEmailChannelParams{WorkspaceID: pg.WorkspaceID, ChannelID: *l.ChannelID})
	if store.IsNotFound(err) {
		return l, ch, errNoticeNoSend
	}
	if err != nil {
		return l, ch, err
	}
	if _, _, ok, err := s.broadcastAccount(ch); err != nil || !ok {
		return l, ch, errNoticeNoSend
	}
	return l, ch, nil
}

func (s *Server) claimNotice(ctx context.Context, ws, id uuid.UUID) (store.StatusNotice, bool, error) {
	var (
		n  store.StatusNotice
		ok bool
	)
	err := s.inTx(ctx, ws, func(q *store.Queries, _ *eventBatch) error {
		var err error
		n, err = q.LockStatusNotice(ctx, store.LockStatusNoticeParams{WorkspaceID: ws, ID: id})
		if store.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		now := s.now()
		switch {
		case n.State == noticePending && !n.DueAt.After(now):
		case n.State == noticeSending && (n.LeaseUntil == nil || !n.LeaseUntil.After(now)):
			n, err = q.SetStatusNoticeState(ctx, store.SetStatusNoticeStateParams{WorkspaceID: ws, ID: id, State: noticeSending,
				LeaseUntil: new(now.Add(noticeLease)), Now: now})
			ok = err == nil
			return err
		default:
			return nil
		}
		pg, err := q.GetStatusPage(ctx, store.GetStatusPageParams{WorkspaceID: ws, ID: n.PageID})
		if err != nil {
			return err
		}
		inc, err := q.GetStatusIncident(ctx, store.GetStatusIncidentParams{WorkspaceID: ws, PageID: pg.ID, ID: n.IncidentID})
		if err != nil {
			return err
		}
		reason, err := s.noticeSkip(ctx, q, n, pg, inc, now)
		if err != nil {
			return err
		}
		if reason != "" {
			n, err = q.SetStatusNoticeState(ctx, store.SetStatusNoticeStateParams{WorkspaceID: ws, ID: id, State: noticeSkipped,
				SkipReason: &reason, FinishedAt: &now, Now: now})
			if err == nil {
				s.noticeSignal(ctx, q, pg.InboxID, n)
			}
			return err
		}
		n, err = q.SetStatusNoticeState(ctx, store.SetStatusNoticeStateParams{WorkspaceID: ws, ID: id, State: noticeSending,
			LeaseUntil: new(now.Add(noticeLease)), StartedAt: &now, Now: now})
		if err == nil {
			ok = true
			s.noticeSignal(ctx, q, pg.InboxID, n)
		}
		return err
	})
	return n, ok, err
}

func (s *Server) noticeSkip(ctx context.Context, q *store.Queries, n store.StatusNotice, pg store.StatusPage, inc store.StatusIncident, now time.Time) (string, error) {
	if s.status.NoticesOff || !pg.EmailEnabled || pg.MailingListID == nil {
		return string(oas.NoticeSkipEmailOff), nil
	}
	if !pg.Published || inc.PublishedAt == nil {
		return string(oas.NoticeSkipUnpublished), nil
	}
	live, err := s.workspaceLive(ctx, pg.WorkspaceID)
	if err != nil {
		return "", err
	}
	w, err := q.GetWorkspace(ctx, pg.WorkspaceID)
	if err != nil {
		return "", err
	}
	if !live || w.BroadcastsStatus == wsSuspended {
		return string(oas.NoticeSkipSuspended), nil
	}
	if _, _, err := s.noticeChannel(ctx, q, pg); errors.Is(err, errNoticeNoSend) || store.IsNotFound(err) {
		return string(oas.NoticeSkipSenderNotReady), nil
	} else if err != nil {
		return "", err
	}
	if !n.Final {
		started, err := q.CountStartedStatusNotices(ctx, store.CountStartedStatusNoticesParams{WorkspaceID: pg.WorkspaceID, PageID: pg.ID, Since: now.Add(-noticeRateWindow)})
		if err != nil {
			return "", err
		}
		if started >= noticeRateMax {
			return string(oas.NoticeSkipRate), nil
		}
		return "", nil
	}
	earlier, err := q.HasEarlierSentStatusNotice(ctx, store.HasEarlierSentStatusNoticeParams{WorkspaceID: pg.WorkspaceID, IncidentID: inc.ID, NoticeID: n.ID})
	if err != nil {
		return "", err
	}
	if !earlier {
		return string(oas.NoticeSkipNothingEarlier), nil
	}
	return "", nil
}

func (s *Server) RunStatusNotice(ctx context.Context, ws, id uuid.UUID, budget time.Duration) error {
	n, ok, err := s.claimNotice(ctx, ws, id)
	if err != nil || !ok {
		return err
	}
	ns, err := s.loadNoticeSend(ctx, n)
	if errors.Is(err, errNoticeNoSend) || store.IsNotFound(err) {
		return s.finishNotice(ctx, n, noticeSkipped, string(oas.NoticeSkipSenderNotReady))
	}
	if err != nil {
		return err
	}
	if !n.SnapshotDone {
		recipients, err := s.snapshotNotice(ctx, ns)
		if err != nil {
			return err
		}
		if recipients == 0 {
			return s.finishNotice(ctx, n, noticeSkipped, string(oas.NoticeSkipNoRecipients))
		}
	}
	return s.sendNotice(ctx, ns, budget)
}

func (s *Server) finishNotice(ctx context.Context, n store.StatusNotice, state, reason string) error {
	return s.inTx(ctx, n.WorkspaceID, func(q *store.Queries, _ *eventBatch) error {
		now := s.now()
		var skip *string
		if reason != "" {
			skip = &reason
		}
		if _, err := q.SetStatusNoticeState(ctx, store.SetStatusNoticeStateParams{WorkspaceID: n.WorkspaceID, ID: n.ID, State: state,
			SkipReason: skip, FinishedAt: &now, Now: now}); err != nil {
			return err
		}
		cur, err := q.RecountStatusNotice(ctx, store.RecountStatusNoticeParams{WorkspaceID: n.WorkspaceID, ID: n.ID, Now: now})
		if err != nil {
			return err
		}
		pg, err := q.GetStatusPage(ctx, store.GetStatusPageParams{WorkspaceID: n.WorkspaceID, ID: n.PageID})
		if err == nil {
			s.noticeSignal(ctx, q, pg.InboxID, cur)
		}
		return nil
	})
}

func (s *Server) loadNoticeSend(ctx context.Context, n store.StatusNotice) (*noticeSend, error) {
	q := s.st.Queries
	ws := n.WorkspaceID
	ns := &noticeSend{n: n, names: map[uuid.UUID]string{}, current: map[uuid.UUID]string{}, data: map[string]map[string]any{}}
	var err error
	if ns.pg, err = q.GetStatusPage(ctx, store.GetStatusPageParams{WorkspaceID: ws, ID: n.PageID}); err != nil {
		return nil, err
	}
	if ns.pg.MailingListID == nil {
		return nil, errNoticeNoSend
	}
	if ns.inc, err = q.GetStatusIncident(ctx, store.GetStatusIncidentParams{WorkspaceID: ws, PageID: n.PageID, ID: n.IncidentID}); err != nil {
		return nil, err
	}
	if ns.inbox, err = q.GetInbox(ctx, store.GetInboxParams{WorkspaceID: ws, ID: ns.pg.InboxID}); err != nil {
		return nil, err
	}
	if ns.list, ns.ch, err = s.noticeChannel(ctx, q, ns.pg); err != nil {
		return nil, err
	}
	if ns.loc, err = time.LoadLocation(ns.pg.Timezone); err != nil {
		ns.loc = time.UTC
	}
	if ns.comps, err = q.ListStatusIncidentComponents(ctx, store.ListStatusIncidentComponentsParams{WorkspaceID: ws, IncidentIds: []uuid.UUID{ns.inc.ID}}); err != nil {
		return nil, err
	}
	all, err := q.ListStatusComponents(ctx, store.ListStatusComponentsParams{WorkspaceID: ws, PageID: ns.pg.ID})
	if err != nil {
		return nil, err
	}
	for _, c := range all {
		if !c.Hidden {
			ns.names[c.ID] = c.Name
			ns.current[c.ID] = c.State
		}
	}
	if ns.updates, err = q.ListStatusNoticeUpdates(ctx, store.ListStatusNoticeUpdatesParams{WorkspaceID: ws, NoticeID: &n.ID}); err != nil {
		return nil, err
	}
	if len(ns.updates) == 0 {
		all, err := q.ListStatusIncidentUpdates(ctx, store.ListStatusIncidentUpdatesParams{WorkspaceID: ws, IncidentIds: []uuid.UUID{ns.inc.ID}})
		if err != nil {
			return nil, err
		}
		if i := slices.IndexFunc(all, func(u store.StatusIncidentUpdate) bool { return u.Notify }); i >= 0 {
			ns.updates = all[i : i+1]
		}
	}
	cfg, from, ok, err := s.broadcastAccount(ns.ch)
	if err != nil || !ok {
		return nil, errNoticeNoSend
	}
	ns.cfg, ns.from, ns.domain = cfg, from, email.Domain(from)
	if ns.dkim, err = s.dkimKey(ns.ch, from, cfg.Host); err != nil {
		return nil, errNoticeNoSend
	}
	if ns.ses, err = s.sesHeaders(ctx, ns.ch, cfg); err != nil {
		return nil, err
	}
	ns.fromName = ns.ch.DisplayName
	if ns.fromName == "" {
		ns.fromName = ns.inbox.Name
	}
	ns.replyTo = ns.ch.Address
	if isCatchAll(ns.replyTo) {
		ns.replyTo = from
	}
	return ns, nil
}

func (s *Server) snapshotNotice(ctx context.Context, ns *noticeSend) (int64, error) {
	ws := ns.n.WorkspaceID
	after := uuid.Nil()
	for {
		var (
			done bool
			last uuid.UUID
		)
		err := s.inTx(ctx, ws, func(q *store.Queries, _ *eventBatch) error {
			rows, err := q.StatusNoticeRecipients(ctx, store.StatusNoticeRecipientsParams{
				WorkspaceID: ws, ListID: ns.list.ID, IncidentID: ns.inc.ID, NoticeID: ns.n.ID, EarlierOnly: ns.n.Final, After: after, Lim: noticeSnapshotPage,
			})
			if err != nil {
				return err
			}
			if len(rows) < noticeSnapshotPage {
				done = true
			}
			if len(rows) == 0 {
				return nil
			}
			subs := make([]uuid.UUID, len(rows))
			for i, r := range rows {
				subs[i] = r.ID
			}
			prev, err := q.PreviousStatusNoticeMessages(ctx, store.PreviousStatusNoticeMessagesParams{
				WorkspaceID: ws, IncidentID: ns.inc.ID, NoticeID: ns.n.ID, SubscriptionIds: subs,
			})
			if err != nil {
				return err
			}
			latest := map[uuid.UUID]string{}
			for _, p := range prev {
				if _, ok := latest[p.SubscriptionID]; !ok {
					latest[p.SubscriptionID] = p.HeaderMessageID
				}
			}
			var ins store.InsertStatusNoticeDeliveriesParams
			ins.WorkspaceID, ins.NoticeID, ins.Now = ws, ns.n.ID, s.now()
			for _, r := range rows {
				token := email.NewConversationToken()
				ins.Ids = append(ins.Ids, newID())
				ins.SubscriptionIds = append(ins.SubscriptionIds, r.ID)
				ins.ContactIds = append(ins.ContactIds, r.ContactID)
				ins.Emails = append(ins.Emails, r.Email)
				ins.Tokens = append(ins.Tokens, token)
				ins.Headers = append(ins.Headers, email.NewMessageID(token, ns.domain))
				ins.Previous = append(ins.Previous, latest[r.ID])
			}
			last = rows[len(rows)-1].ID
			_, err = q.InsertStatusNoticeDeliveries(ctx, ins)
			return err
		})
		if err != nil {
			return 0, err
		}
		if done {
			break
		}
		after = last
	}
	var cur store.StatusNotice
	err := s.inTx(ctx, ws, func(q *store.Queries, _ *eventBatch) error {
		now := s.now()
		if err := q.FinishStatusNoticeSnapshot(ctx, store.FinishStatusNoticeSnapshotParams{WorkspaceID: ws, ID: ns.n.ID, Now: now}); err != nil {
			return err
		}
		var err error
		cur, err = q.RecountStatusNotice(ctx, store.RecountStatusNoticeParams{WorkspaceID: ws, ID: ns.n.ID, Now: now})
		if err == nil {
			s.noticeSignal(ctx, q, ns.pg.InboxID, cur)
		}
		return err
	})
	if err != nil {
		return 0, err
	}
	ns.n = cur
	return noticeCounts(cur.Counts).Recipients, nil
}

type noticeResult struct {
	d        store.StatusNoticeDelivery
	state    string
	skip     *string
	attempts int32
	next     *time.Time
	errText  *string
	sentAt   *time.Time
	bounced  *time.Time
	relayID  *string
}

// With retries left for later the notice is handed to the sweep through its lease.
func (s *Server) sendNotice(ctx context.Context, ns *noticeSend, budget time.Duration) error {
	ws := ns.n.WorkspaceID
	deadline := time.Now().Add(budget)
	var sess email.Session
	defer func() {
		if sess != nil {
			_ = sess.Close()
		}
	}()
	pace := time.Duration(0)
	if ns.ch.BroadcastRate > 0 {
		pace = time.Second / time.Duration(ns.ch.BroadcastRate)
	}
	sentToday, err := s.st.CountStatusNoticeMailsSince(ctx, store.CountStatusNoticeMailsSinceParams{WorkspaceID: ws, Since: utcDay(s.now())})
	if err != nil {
		return err
	}
	for {
		now := s.now()
		if time.Now().After(deadline) {
			if err := s.st.RenewStatusNoticeLease(ctx, store.RenewStatusNoticeLeaseParams{WorkspaceID: ws, ID: ns.n.ID, LeaseUntil: &now, Now: now}); err != nil {
				return err
			}
			_, err := s.jobs.Insert(ctx, StatusNoticeArgs{WorkspaceID: ws, NoticeID: ns.n.ID}, noticeJobOpts(time.Time{}))
			return err
		}
		due, err := s.st.DueStatusNoticeDeliveries(ctx, store.DueStatusNoticeDeliveriesParams{WorkspaceID: ws, NoticeID: ns.n.ID, Now: now, Lim: noticeBatch})
		if err != nil {
			return err
		}
		if len(due) == 0 {
			qd, err := s.st.QueuedStatusNoticeDeliveries(ctx, store.QueuedStatusNoticeDeliveriesParams{WorkspaceID: ws, NoticeID: ns.n.ID})
			if err != nil {
				return err
			}
			if qd.Queued == 0 {
				return s.finishNotice(ctx, ns.n, noticeSent, "")
			}
			next := qd.NextAttempt
			return s.st.RenewStatusNoticeLease(ctx, store.RenewStatusNoticeLeaseParams{WorkspaceID: ws, ID: ns.n.ID, LeaseUntil: &next, Now: now})
		}
		results, broken, batchErr := s.sendNoticeBatch(ctx, ns, &sess, due, pace, &sentToday)
		if broken && sess != nil {
			_ = sess.Close()
			sess = nil
		}
		if err := s.commitNoticeBatch(ctx, ns, results); err != nil {
			return err
		}
		if batchErr != nil {
			now := s.now()
			if err := s.st.RenewStatusNoticeLease(ctx, store.RenewStatusNoticeLeaseParams{WorkspaceID: ws, ID: ns.n.ID, LeaseUntil: &now, Now: now}); err != nil {
				return err
			}
			return batchErr
		}
	}
}

func (s *Server) sendNoticeBatch(ctx context.Context, ns *noticeSend, sess *email.Session, due []store.StatusNoticeDelivery, pace time.Duration, sentToday *int64) ([]noticeResult, bool, error) {
	ws := ns.n.WorkspaceID
	until := time.Now().Add(batchTime)
	var out []noticeResult
	refs, err := s.noticeReferences(ctx, ns, due)
	if err != nil {
		s.log.ErrorContext(ctx, "status notice references", slog.Any("error", err))
	}
	attempted := false
	for i, d := range due {
		if i > 0 && time.Now().After(until) {
			break
		}
		res := noticeResult{d: d, state: deliveryQueued, attempts: d.Attempts}
		skip := func(reason string) {
			res.state, res.skip = noticeSkipped, &reason
		}
		active, err := s.st.StatusSubscriptionActive(ctx, store.StatusSubscriptionActiveParams{WorkspaceID: ws, ID: d.SubscriptionID})
		if err != nil {
			s.log.ErrorContext(ctx, "status notice recipient check", slog.Any("error", err))
			return out, false, err
		}
		suppressed, err := s.st.IsEmailSuppressedAnyScope(ctx, store.IsEmailSuppressedAnyScopeParams{WorkspaceID: ws, Email: d.Email})
		if err != nil {
			s.log.ErrorContext(ctx, "status notice recipient check", slog.Any("error", err))
			return out, false, err
		}
		switch {
		case !active:
			skip("unsubscribed")
		case suppressed:
			skip("suppressed")
		case s.now().Sub(d.CreatedAt) > noticeStale:
			skip("stale")
		case s.status.NoticesDaily > 0 && *sentToday >= s.status.NoticesDaily:
			res.next = new(utcDay(s.now()).AddDate(0, 0, 1))
		}
		if res.skip != nil || res.next != nil {
			out = append(out, res)
			continue
		}
		raw, err := s.noticeMessage(ctx, ns, d, refs[d.SubscriptionID])
		if err != nil {
			res.state, res.errText = deliveryFailed, new(truncateRunes("the mail could not be built: "+err.Error(), maxDeliveryError))
			out = append(out, res)
			continue
		}
		if *sess == nil {
			openCtx, cancel := context.WithTimeout(ctx, time.Minute)
			*sess, err = email.OpenSession(openCtx, s.sender, ns.cfg)
			cancel()
			if err != nil {
				*sess = nil
				res.attempts++
				res.errText = new(truncateRunes(err.Error(), maxDeliveryError))
				res.next = new(s.now().Add(noticeRetry[0]))
				out = append(out, res)
				return out, true, nil
			}
		}
		if attempted && pace > 0 {
			select {
			case <-ctx.Done():
				return out, false, nil
			case <-time.After(pace):
			}
		}
		attempted = true
		sendCtx, cancel := context.WithTimeout(ctx, time.Minute)
		relayID, err := (*sess).Send(sendCtx, ns.from, []string{d.Email}, raw)
		cancel()
		now := s.now()
		res.attempts = d.Attempts + 1
		switch {
		case err == nil:
			res.state, res.sentAt = deliverySent, &now
			if relayID != "" {
				res.relayID = &relayID
			}
			*sentToday++
		case email.BadMailbox(err):
			res.state, res.bounced, res.errText = string(oas.BroadcastDeliveryStateBounced), &now, new(truncateRunes(err.Error(), maxDeliveryError))
		case email.IsPermanent(err):
			res.state, res.errText = deliveryFailed, new(truncateRunes(err.Error(), maxDeliveryError))
		default:
			res.errText = new(truncateRunes(err.Error(), maxDeliveryError))
			if int(res.attempts) > len(noticeRetry) {
				res.state = deliveryFailed
			} else {
				res.next = new(now.Add(noticeRetry[res.attempts-1]))
			}
		}
		out = append(out, res)
		var brokenErr *email.BrokenError
		if errors.As(err, &brokenErr) {
			return out, true, nil
		}
	}
	return out, false, nil
}

// noticeReferences are the Message-IDs of each recipient's earlier notices of the incident, oldest
// first, so mail clients keep them in one thread.
func (s *Server) noticeReferences(ctx context.Context, ns *noticeSend, due []store.StatusNoticeDelivery) (map[uuid.UUID][]string, error) {
	subs := make([]uuid.UUID, len(due))
	for i, d := range due {
		subs[i] = d.SubscriptionID
	}
	rows, err := s.st.PreviousStatusNoticeMessages(ctx, store.PreviousStatusNoticeMessagesParams{
		WorkspaceID: ns.n.WorkspaceID, IncidentID: ns.inc.ID, NoticeID: ns.n.ID, SubscriptionIds: subs,
	})
	out := map[uuid.UUID][]string{}
	if err != nil {
		return out, err
	}
	for _, r := range rows {
		out[r.SubscriptionID] = append([]string{r.HeaderMessageID}, out[r.SubscriptionID]...)
	}
	return out, nil
}

func (s *Server) noticeData(ns *noticeSend, locale string) map[string]any {
	w := statusWordsFor(locale)
	if d, ok := ns.data[w.Lang]; ok {
		return d
	}
	inc := ns.inc
	d := map[string]any{
		"Inbox": ns.inbox.Name, "Page": oneLine(ns.pg.Name), "Title": oneLine(inc.Title), "Maintenance": inc.Kind == kindMaintenance,
		"State": w.IncidentStates[inc.State], "Zone": ns.pg.Timezone, "Sender": senderLine(ns.inbox),
		"URL": s.pageURL(ns.pg, "/incidents/"+inc.ID.String()),
	}
	if inc.Impact != string(oas.StatusImpactNone) {
		d["Impact"] = w.Impacts[inc.Impact]
	}
	if inc.ScheduledStart != nil && inc.ScheduledEnd != nil {
		d["Window"] = w.moment(inc.ScheduledStart.In(ns.loc)) + " – " + w.moment(inc.ScheduledEnd.In(ns.loc))
	}
	var comps []map[string]string
	for _, c := range ns.comps {
		if name, ok := ns.names[c.ComponentID]; ok {
			state := c.State
			if incidentClosed(inc.State) {
				state = ns.current[c.ComponentID]
			}
			comps = append(comps, map[string]string{"Name": name, "State": w.States[state]})
		}
	}
	d["Components"] = comps
	var ups []map[string]any
	for _, u := range ns.updates {
		ups = append(ups, map[string]any{"State": w.IncidentStates[u.State], "At": w.moment(u.CreatedAt.In(ns.loc)), "Text": u.Body,
			"HTML": template.HTML(richStatusText(u.Body)), "Edited": u.EditedAt != nil})
	}
	d["Updates"] = ups
	ns.data[w.Lang] = d
	return d
}

func (s *Server) noticeMessage(ctx context.Context, ns *noticeSend, d store.StatusNoticeDelivery, refs []string) ([]byte, error) {
	ws := ns.n.WorkspaceID
	locale := ns.pg.Locale
	if c, err := s.st.GetContact(ctx, store.GetContactParams{WorkspaceID: ws, ID: d.ContactID}); err == nil && c.Locale != nil && *c.Locale != "" {
		locale = *c.Locale
	}
	manage := s.manageURL(ws, ns.list.ID, d.ContactID, uuid.Nil())
	data := map[string]any{"ManageURL": manage}
	for k, v := range s.noticeData(ns, locale) {
		data[k] = v
	}
	brand := mail.Brand{Name: ns.inbox.Name, Color: inboxMailBrand(ns.inbox).Color}
	if ns.pg.AccentColor != nil {
		brand.Color = *ns.pg.AccentColor
	}
	msg, err := mail.RenderAs(brand, "status_notice", locale, data)
	if err != nil {
		return nil, err
	}
	listID := "<s" + strings.ReplaceAll(ns.list.ID.String(), "-", "")[20:] + "." + ns.domain + ">"
	if p := headerPhrase(ns.pg.Name); p != "" {
		listID = p + " " + listID
	}
	headers := []email.Header{
		{Name: "List-Unsubscribe", Value: "<" + manage + ">"},
		{Name: "List-Unsubscribe-Post", Value: "List-Unsubscribe=One-Click"},
		{Name: "List-Id", Value: listID},
		{Name: "Feedback-ID", Value: hex8(ns.inc.ID) + ":" + hex8(ns.list.ID) + ":" + hex8(ws) + ":" + noticeFeedbackID},
		{Name: "X-Auto-Response-Suppress", Value: "OOF, AutoReply"},
	}
	out := email.Outgoing{
		From: email.Address{Name: ns.fromName, Email: ns.from}, ReplyTo: email.Address{Name: ns.fromName, Email: ns.replyTo},
		To: email.Address{Email: d.Email}, Subject: msg.Subject, MessageID: d.HeaderMessageID, Text: msg.Text, HTML: msg.HTML,
		Date: s.now(), Headers: append(headers, ns.ses...),
	}
	if len(refs) > 0 {
		out.InReplyTo, out.References = refs[len(refs)-1], refs
	} else if d.PreviousMessageID != nil {
		out.InReplyTo, out.References = *d.PreviousMessageID, []string{*d.PreviousMessageID}
	}
	raw, err := email.Build(out)
	if err != nil {
		return nil, err
	}
	return signWith(ns.dkim, raw)
}

func (s *Server) commitNoticeBatch(ctx context.Context, ns *noticeSend, results []noticeResult) error {
	if len(results) == 0 {
		return nil
	}
	ws := ns.n.WorkspaceID
	var bounced int32
	err := s.inTx(ctx, ws, func(q *store.Queries, events *eventBatch) error {
		var sent int32
		for _, r := range results {
			if err := q.SaveStatusNoticeDelivery(ctx, store.SaveStatusNoticeDeliveryParams{
				WorkspaceID: ws, ID: r.d.ID, State: r.state, SkipReason: r.skip, Attempts: r.attempts, NextAttemptAt: r.next,
				Error: r.errText, SentAt: r.sentAt, BouncedAt: r.bounced, RelayMessageID: r.relayID,
			}); err != nil {
				return err
			}
			switch r.state {
			case deliverySent:
				sent++
			case string(oas.BroadcastDeliveryStateBounced):
				bounced++
				if err := s.suppressBounced(ctx, q, events, ws, r.d.Email, deref(r.errText)); err != nil {
					return err
				}
			}
		}
		now := s.now()
		if sent > 0 || bounced > 0 {
			if err := q.AddSendingDay(ctx, store.AddSendingDayParams{WorkspaceID: ws, Day: utcDay(now), Sent: sent, Bounced: bounced}); err != nil {
				return err
			}
		}
		if sent > 0 {
			if err := q.AddStatusNoticeUsage(ctx, store.AddStatusNoticeUsageParams{WorkspaceID: ws, Month: monthOf(now), Notices: int64(sent)}); err != nil {
				return err
			}
		}
		if err := q.RenewStatusNoticeLease(ctx, store.RenewStatusNoticeLeaseParams{WorkspaceID: ws, ID: ns.n.ID, LeaseUntil: new(now.Add(noticeLease)), Now: now}); err != nil {
			return err
		}
		cur, err := q.RecountStatusNotice(ctx, store.RecountStatusNoticeParams{WorkspaceID: ws, ID: ns.n.ID, Now: now})
		if err != nil {
			return err
		}
		ns.n = cur
		s.noticeSignal(ctx, q, ns.pg.InboxID, cur)
		return nil
	})
	if err != nil || bounced == 0 {
		return err
	}
	return s.checkWorkspaceRates(ctx, ws)
}

func noticeDeliveryOf(ctx context.Context, q *store.Queries, ws uuid.UUID, ids []string) (*store.StatusNoticeDelivery, error) {
	for _, id := range ids {
		if id == "" {
			continue
		}
		d, err := q.FindStatusNoticeDeliveryByHeader(ctx, store.FindStatusNoticeDeliveryByHeaderParams{WorkspaceID: ws, HeaderMessageID: id})
		if store.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return &d, nil
	}
	return nil, nil
}

func (s *Server) noticeFeedback(ctx context.Context, d store.StatusNoticeDelivery, complaint bool, detail string) error {
	ws := d.WorkspaceID
	err := s.inTx(ctx, ws, func(q *store.Queries, events *eventBatch) error {
		now := s.now()
		if complaint {
			n, err := q.MarkStatusDeliveryComplained(ctx, store.MarkStatusDeliveryComplainedParams{WorkspaceID: ws, ID: d.ID, Now: now})
			if err != nil {
				return err
			}
			if n == 1 {
				if err := q.AddSendingDay(ctx, store.AddSendingDayParams{WorkspaceID: ws, Day: utcDay(now), Complained: 1}); err != nil {
					return err
				}
			}
			if err := q.SuppressEmailScoped(ctx, store.SuppressEmailScopedParams{
				WorkspaceID: ws, Email: d.Email, Reason: suppressComplaint, Detail: truncateRunes(detail, 2000), Scope: "broadcasts", Now: now,
			}); err != nil {
				return err
			}
			if err := s.complainedAbout(ctx, q, events, ws, d.Email); err != nil {
				return err
			}
		} else {
			n, err := q.MarkStatusDeliveryBounced(ctx, store.MarkStatusDeliveryBouncedParams{WorkspaceID: ws, ID: d.ID, Now: now, Error: optionalText(detail, maxDeliveryError)})
			if err != nil {
				return err
			}
			if n == 1 {
				if err := q.AddSendingDay(ctx, store.AddSendingDayParams{WorkspaceID: ws, Day: utcDay(now), Bounced: 1}); err != nil {
					return err
				}
			}
			if err := s.suppressBounced(ctx, q, events, ws, d.Email, detail); err != nil {
				return err
			}
		}
		cur, err := q.RecountStatusNotice(ctx, store.RecountStatusNoticeParams{WorkspaceID: ws, ID: d.NoticeID, Now: now})
		if err != nil {
			return err
		}
		if pg, err := q.GetStatusPage(ctx, store.GetStatusPageParams{WorkspaceID: ws, ID: cur.PageID}); err == nil {
			s.noticeSignal(ctx, q, pg.InboxID, cur)
		}
		return s.contactChanged(ctx, q, events, ws, d.Email)
	})
	if err != nil {
		return err
	}
	return s.checkWorkspaceRates(ctx, ws)
}

func noticeDeliveryOfTokens(ctx context.Context, q *store.Queries, ch store.FindEmailChannelByAddressRow, m *email.Message, contactID uuid.UUID) (*store.StatusNoticeDelivery, error) {
	ids := slices.Concat(m.InReplyTo, m.References)
	if len(ids) == 0 {
		return nil, nil
	}
	var tokens []string
	for _, id := range ids {
		if t, _, ok := email.TokenFromID(id); ok {
			tokens = append(tokens, t)
		}
	}
	d, err := q.FindStatusNoticeDeliveryByTokens(ctx, store.FindStatusNoticeDeliveryByTokensParams{
		WorkspaceID: ch.WorkspaceID, InboxID: ch.InboxID, Tokens: tokens, Ids: ids, ContactID: contactID,
	})
	if store.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Server) noticeAutoReply(ctx context.Context, ch store.FindEmailChannelByAddressRow, m *email.Message) (bool, error) {
	d, err := noticeDeliveryOfTokens(ctx, s.st.Queries, ch, m, uuid.Nil())
	if err != nil || d == nil {
		return false, err
	}
	err = s.st.InTx(ctx, func(q *store.Queries) error {
		n, err := q.MarkStatusDeliveryAutoReplied(ctx, store.MarkStatusDeliveryAutoRepliedParams{WorkspaceID: d.WorkspaceID, ID: d.ID, Now: s.now()})
		if err != nil || n == 0 {
			return err
		}
		_, err = q.RecountStatusNotice(ctx, store.RecountStatusNoticeParams{WorkspaceID: d.WorkspaceID, ID: d.NoticeID, Now: s.now()})
		return err
	})
	return err == nil, err
}

func (s *Server) noticeReplyStart(ctx context.Context, q *store.Queries, c store.Conversation, d store.StatusNoticeDelivery, contactID uuid.UUID, now time.Time) error {
	inc, err := q.StatusNoticeIncident(ctx, store.StatusNoticeIncidentParams{WorkspaceID: c.WorkspaceID, NoticeID: d.NoticeID})
	if store.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	linked, err := q.StatusIncidentConversationLinked(ctx, store.StatusIncidentConversationLinkedParams{WorkspaceID: c.WorkspaceID, IncidentID: inc.ID, ConversationID: c.ID})
	if err != nil {
		return err
	}
	if !linked {
		n, err := q.CountStatusIncidentConversations(ctx, store.CountStatusIncidentConversationsParams{WorkspaceID: c.WorkspaceID, IncidentID: inc.ID})
		if err != nil {
			return err
		}
		if int(n) < maxIncidentLinks {
			if err := q.LinkStatusIncidentConversation(ctx, store.LinkStatusIncidentConversationParams{
				WorkspaceID: c.WorkspaceID, IncidentID: inc.ID, ConversationID: c.ID, Now: now,
			}); err != nil {
				return err
			}
		}
	}
	if d.ContactID != contactID {
		return nil
	}
	n, err := q.MarkStatusDeliveryReplied(ctx, store.MarkStatusDeliveryRepliedParams{WorkspaceID: c.WorkspaceID, ID: d.ID, Now: now, ConversationID: &c.ID})
	if err != nil || n == 0 {
		return err
	}
	_, err = q.RecountStatusNotice(ctx, store.RecountStatusNoticeParams{WorkspaceID: c.WorkspaceID, ID: d.NoticeID, Now: now})
	return err
}

func (s *Server) CleanStatusNotices(ctx context.Context) error {
	ids, err := s.st.ListWorkspaceIDs(ctx)
	if err != nil {
		return err
	}
	before := s.now().Add(-noticeDeliveriesKept)
	for _, ws := range ids {
		if _, err := s.st.DeleteOldStatusNoticeDeliveries(ctx, store.DeleteOldStatusNoticeDeliveriesParams{WorkspaceID: ws, Before: before, Lim: 5000}); err != nil {
			return err
		}
	}
	return nil
}

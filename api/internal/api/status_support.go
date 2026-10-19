package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/realtime"
	"github.com/productdevbook/yuva/api/internal/store"
)

const statusReplyAttempts = 8

func (s *Server) clientStatus(ctx context.Context, q *store.Queries, inboxID, ws uuid.UUID) (*oas.ClientInboxStatus, error) {
	pg, err := q.GetInboxStatusPage(ctx, store.GetInboxStatusPageParams{WorkspaceID: ws, InboxID: inboxID})
	if store.IsNotFound(err) || (err == nil && !pg.Published) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	now := s.now()
	key := pg.ID.String() + "|client"
	if e, ok := s.statusCache.get(key, pg.ChangedAt, now); ok {
		var out oas.ClientInboxStatus
		if err := json.Unmarshal(e.body, &out); err == nil {
			return &out, nil
		}
	}
	comps, err := q.ListStatusComponents(ctx, store.ListStatusComponentsParams{WorkspaceID: ws, PageID: pg.ID})
	if err != nil {
		return nil, err
	}
	incs, err := q.ClientStatusIncidents(ctx, store.ClientStatusIncidentsParams{WorkspaceID: ws, PageID: pg.ID})
	if err != nil {
		return nil, err
	}
	out := oas.ClientInboxStatus{Url: s.pageURL(pg, ""), State: oas.StatusComponentStateValue(pageState(comps)), Open: []oas.ClientStatusIncident{}}
	for _, i := range incs {
		out.Open = append(out.Open, oas.ClientStatusIncident{
			Id: i.ID, Kind: oas.StatusIncidentKind(i.Kind), Title: i.Title, Impact: oas.StatusIncidentImpact(i.Impact),
			State: oas.StatusIncidentState(i.State), UpdatedAt: i.LastUpdateAt, Url: s.pageURL(pg, "/incidents/"+i.ID.String()),
		})
	}
	s.statusCache.put(key, statusCacheEntry{changed: pg.ChangedAt, at: now, ctype: "application/json", body: mustJSON(out)})
	return &out, nil
}

func clientStatusEvent(typ string) bool {
	switch typ {
	case statusEventComponent, statusEventCreated, statusEventUpdated, statusEventResolved, realtime.StatusHint:
		return true
	}
	return false
}

// statusHint lets the inbox's contact connections compare their status line after a change that
// stores no event, such as publishing or deleting the page.
func (s *Server) statusHint(ctx context.Context, ws, inboxID uuid.UUID) {
	s.signal(ctx, realtime.Event{Type: realtime.StatusHint, WorkspaceID: ws, InboxID: &inboxID, Data: json.RawMessage("{}")})
}

func (s *Server) statusPageSignal(ctx context.Context, ws, inboxID, pageID uuid.UUID) {
	s.signal(ctx, realtime.Event{Type: eventStatusPage, WorkspaceID: ws, InboxID: &inboxID, Data: mustJSON(map[string]uuid.UUID{"page_id": pageID})})
}

func statusLinkTargets(ctx context.Context, q *store.Queries, p principal, pageID, incidentID, conversationID uuid.UUID) (store.StatusIncident, store.Conversation, error) {
	pg, err := visibleStatusPage(ctx, q, p, pageID, true)
	if err != nil {
		return store.StatusIncident{}, store.Conversation{}, err
	}
	inc, err := lockedStatusIncident(ctx, q, pg, incidentID)
	if err != nil {
		return inc, store.Conversation{}, err
	}
	c, err := visibleConversation(ctx, q, p, conversationID, true)
	return inc, c, err
}

func (s *Server) LinkStatusIncidentConversation(ctx context.Context, req oas.LinkStatusIncidentConversationRequestObject) (oas.LinkStatusIncidentConversationResponseObject, error) {
	p := principalFrom(ctx)
	err := s.inTx(ctx, p.workspaceID, func(q *store.Queries, _ *eventBatch) error {
		inc, c, err := statusLinkTargets(ctx, q, p, req.PageId, req.IncidentId, req.ConversationId)
		if err != nil {
			return err
		}
		linked, err := q.StatusIncidentConversationLinked(ctx, store.StatusIncidentConversationLinkedParams{
			WorkspaceID: p.workspaceID, IncidentID: inc.ID, ConversationID: c.ID,
		})
		if err != nil || linked {
			return err
		}
		n, err := q.CountStatusIncidentConversations(ctx, store.CountStatusIncidentConversationsParams{WorkspaceID: p.workspaceID, IncidentID: inc.ID})
		if err != nil {
			return err
		}
		if int(n) >= maxIncidentLinks {
			return errTooManyLinks()
		}
		member, key := statusAuthor(p)
		return q.LinkStatusIncidentConversation(ctx, store.LinkStatusIncidentConversationParams{
			WorkspaceID: p.workspaceID, IncidentID: inc.ID, ConversationID: c.ID, LinkedByMemberID: member, LinkedByApiKeyID: key, Now: s.now(),
		})
	})
	if err != nil {
		return nil, err
	}
	return oas.LinkStatusIncidentConversation204Response{}, nil
}

func (s *Server) UnlinkStatusIncidentConversation(ctx context.Context, req oas.UnlinkStatusIncidentConversationRequestObject) (oas.UnlinkStatusIncidentConversationResponseObject, error) {
	p := principalFrom(ctx)
	err := s.inTx(ctx, p.workspaceID, func(q *store.Queries, _ *eventBatch) error {
		inc, c, err := statusLinkTargets(ctx, q, p, req.PageId, req.IncidentId, req.ConversationId)
		if err != nil {
			return err
		}
		return q.UnlinkStatusIncidentConversation(ctx, store.UnlinkStatusIncidentConversationParams{
			WorkspaceID: p.workspaceID, IncidentID: inc.ID, ConversationID: c.ID,
		})
	})
	if err != nil {
		return nil, err
	}
	return oas.UnlinkStatusIncidentConversation204Response{}, nil
}

func visibleStatusIncident(ctx context.Context, q *store.Queries, p principal, pageID, incidentID uuid.UUID) (store.StatusIncident, error) {
	pg, err := visibleStatusPage(ctx, q, p, pageID, false)
	if err != nil {
		return store.StatusIncident{}, err
	}
	inc, err := q.GetStatusIncident(ctx, store.GetStatusIncidentParams{WorkspaceID: p.workspaceID, PageID: pg.ID, ID: incidentID})
	if store.IsNotFound(err) {
		return inc, errStatusIncidentGone
	}
	return inc, err
}

func (s *Server) ListStatusIncidentConversations(ctx context.Context, req oas.ListStatusIncidentConversationsRequestObject) (oas.ListStatusIncidentConversationsResponseObject, error) {
	p := principalFrom(ctx)
	limit, err := pageSize(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	at, id, err := decodeCursor(req.Params.Cursor)
	if err != nil {
		return nil, err
	}
	inc, err := visibleStatusIncident(ctx, s.st.Queries, p, req.PageId, req.IncidentId)
	if err != nil {
		return nil, err
	}
	rows, err := s.st.ListStatusIncidentConversations(ctx, store.ListStatusIncidentConversationsParams{
		WorkspaceID: p.workspaceID, IncidentID: inc.ID, AllInboxes: p.seesAllInboxes(), ViewerID: p.viewerID(),
		CursorAt: at, CursorID: id, Lim: limit + 1,
	})
	if err != nil {
		return nil, err
	}
	out := oas.ListStatusIncidentConversations200JSONResponse{Items: []oas.StatusIncidentConversation{}}
	if len(rows) > int(limit) {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur := encodeCursor(last.LinkedAt, last.Conversation.ID)
		out.NextCursor = &cur
	}
	convs := make([]store.Conversation, len(rows))
	for i, r := range rows {
		convs[i] = r.Conversation
	}
	items, err := s.listItems(ctx, p, convs)
	if err != nil {
		return nil, err
	}
	for i, c := range items {
		out.Items = append(out.Items, oas.StatusIncidentConversation{
			Id: c.Id, InboxId: c.InboxId, ContactId: c.ContactId, ChannelId: c.ChannelId, ChannelKind: c.ChannelKind, Kind: c.Kind, Feedback: c.Feedback, Page: c.Page, Subject: c.Subject,
			Status: c.Status, SnoozeUntil: c.SnoozeUntil, Priority: c.Priority, Spam: c.Spam, AssigneeId: c.AssigneeId, Labels: c.Labels,
			LastMessageAt: c.LastMessageAt, LastActivityAt: c.LastActivityAt, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
			RelatedConversationId: c.RelatedConversationId, ClosedAt: c.ClosedAt, Rating: c.Rating, LastReadByContactAt: c.LastReadByContactAt, BroadcastId: c.BroadcastId,
			Imported: c.Imported, Contact: c.Contact, LastMessage: c.LastMessage, Unread: c.Unread, PinnedAt: c.PinnedAt, LinkedAt: rows[i].LinkedAt,
		})
	}
	return out, nil
}

func (s *Server) ListConversationIncidents(ctx context.Context, req oas.ListConversationIncidentsRequestObject) (oas.ListConversationIncidentsResponseObject, error) {
	p := principalFrom(ctx)
	c, err := visibleConversation(ctx, s.st.Queries, p, req.ConversationId, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.st.ListConversationIncidents(ctx, store.ListConversationIncidentsParams{
		WorkspaceID: p.workspaceID, ConversationID: c.ID, AllInboxes: p.seesAllInboxes(), ViewerID: p.viewerID(),
	})
	if err != nil {
		return nil, err
	}
	out := oas.ListConversationIncidents200JSONResponse{Items: []oas.ConversationIncident{}}
	for _, r := range rows {
		i := r.StatusIncident
		out.Items = append(out.Items, oas.ConversationIncident{
			Id: i.ID, Page: oas.StatusPageRef{Id: i.PageID, Name: r.PageName, Slug: r.PageSlug}, Kind: oas.StatusIncidentKind(i.Kind),
			Title: i.Title, Impact: oas.StatusIncidentImpact(i.Impact), State: oas.StatusIncidentState(i.State),
			Url: s.statusURLFor(r.PageSlug, r.PageDomain, r.PageDomainState, r.PageCfStatus, r.PageCfSslStatus, "/incidents/"+i.ID.String()), PublishedAt: i.PublishedAt, ResolvedAt: i.ResolvedAt, LinkedAt: r.LinkedAt,
		})
	}
	return out, nil
}

type StatusReplyArgs struct {
	WorkspaceID    uuid.UUID `json:"workspace_id"`
	ReplyID        uuid.UUID `json:"reply_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
}

func (StatusReplyArgs) Kind() string { return "status_reply" }

func (StatusReplyArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: statusReplyAttempts, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

type statusReplyWorker struct {
	river.WorkerDefaults[StatusReplyArgs]
	s *Server
}

func (w *statusReplyWorker) Work(ctx context.Context, job *river.Job[StatusReplyArgs]) error {
	err := w.s.RunStatusReply(ctx, job.Args)
	if err != nil && job.Attempt >= job.MaxAttempts {
		w.s.finishStatusReply(context.WithoutCancel(ctx), job.Args, outcomeFailed, reasonGaveUp)
	}
	return err
}

// A retried job finds the message it already posted through this client id instead of sending it again.
func statusReplyClientID(reply uuid.UUID) string { return "status-reply:" + reply.String() }

const (
	outcomePending = "pending"
	outcomeSent    = "sent"
	outcomeSkipped = "skipped"
	outcomeFailed  = "failed"

	reasonClosed      = "closed"
	reasonUnlinked    = "unlinked"
	reasonConvDeleted = "conversation_deleted"
	reasonRevoked     = "revoked"
	reasonGaveUp      = "gave_up"
)

var (
	maxIncidentLinks     = 5000
	statusReplyBatchSize = 500
)

func errTooManyLinks() *apiError {
	return problem(http.StatusUnprocessableEntity, "too_many_links", "an incident takes at most "+strconv.Itoa(maxIncidentLinks)+" linked conversations")
}

var errStatusReplyGone = problem(http.StatusNotFound, "not_found", "no such reply")

func (s *Server) ReplyToStatusIncidentConversations(ctx context.Context, req oas.ReplyToStatusIncidentConversationsRequestObject) (oas.ReplyToStatusIncidentConversationsResponseObject, error) {
	p := principalFrom(ctx)
	b := req.Body
	if err := requireScope(p, oas.MessagesWrite); err != nil {
		return nil, err
	}
	closeAfter := b.Close != nil && *b.Close
	if closeAfter {
		if err := requireScope(p, oas.ConversationsWrite); err != nil {
			return nil, err
		}
	}
	if p.deliversAsBot() && !p.botsMaySend {
		return nil, errBotSendingDisabled
	}
	if len([]rune(b.Body)) > maxMessageBodyRunes {
		return nil, errValidation("body must be at most 65536 characters")
	}
	if strings.TrimSpace(b.Body) == "" {
		return nil, errValidation("body is required")
	}
	var reply store.StatusIncidentReply
	err := s.st.InTxRaw(ctx, func(tx pgx.Tx, q *store.Queries) error {
		inc, err := visibleStatusIncident(ctx, q, p, req.PageId, req.IncidentId)
		if err != nil {
			return err
		}
		ids, err := q.ListStatusReplyTargets(ctx, store.ListStatusReplyTargetsParams{
			WorkspaceID: p.workspaceID, IncidentID: inc.ID, AllInboxes: p.seesAllInboxes(), ViewerID: p.viewerID(),
		})
		if err != nil {
			return err
		}
		member, key := statusAuthor(p)
		var grant *uuid.UUID
		if p.isGrant() {
			g := p.grantID
			grant = &g
		}
		reply, err = q.CreateStatusIncidentReply(ctx, store.CreateStatusIncidentReplyParams{
			WorkspaceID: p.workspaceID, ID: newID(), IncidentID: inc.ID, Body: b.Body, Close: closeAfter, MemberID: member, ApiKeyID: key,
			GrantID: grant, Via: p.via, McpOnly: p.mcpOnly, Queued: int32(len(ids)), CreatedAt: s.now(),
		})
		if err != nil {
			return err
		}
		for len(ids) > 0 {
			batch := ids[:min(statusReplyBatchSize, len(ids))]
			ids = ids[len(batch):]
			if err := q.AddStatusReplyTargets(ctx, store.AddStatusReplyTargetsParams{WorkspaceID: p.workspaceID, ReplyID: reply.ID, ConversationIds: batch}); err != nil {
				return err
			}
			jobs := make([]river.InsertManyParams, len(batch))
			for i, id := range batch {
				jobs[i] = river.InsertManyParams{Args: StatusReplyArgs{WorkspaceID: p.workspaceID, ReplyID: reply.ID, ConversationID: id}}
			}
			if _, err := s.jobs.InsertManyTx(ctx, tx, jobs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return oas.ReplyToStatusIncidentConversations202JSONResponse{Id: reply.ID, Queued: reply.Queued}, nil
}

func (s *Server) GetStatusIncidentReply(ctx context.Context, req oas.GetStatusIncidentReplyRequestObject) (oas.GetStatusIncidentReplyResponseObject, error) {
	p := principalFrom(ctx)
	limit, err := pageSize(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	_, after, err := decodeCursor(req.Params.Cursor)
	if err != nil {
		return nil, err
	}
	inc, err := visibleStatusIncident(ctx, s.st.Queries, p, req.PageId, req.IncidentId)
	if err != nil {
		return nil, err
	}
	reply, err := s.st.GetStatusIncidentReply(ctx, store.GetStatusIncidentReplyParams{WorkspaceID: p.workspaceID, IncidentID: inc.ID, ID: req.ReplyId})
	if store.IsNotFound(err) {
		return nil, errStatusReplyGone
	}
	if err != nil {
		return nil, err
	}
	counts, err := s.st.CountStatusReplyTargets(ctx, store.CountStatusReplyTargetsParams{
		WorkspaceID: p.workspaceID, ReplyID: reply.ID, AllInboxes: p.seesAllInboxes(), ViewerID: p.viewerID(),
	})
	if err != nil {
		return nil, err
	}
	rows, err := s.st.ListStatusReplyTargetOutcomes(ctx, store.ListStatusReplyTargetOutcomesParams{
		WorkspaceID: p.workspaceID, ReplyID: reply.ID, AllInboxes: p.seesAllInboxes(), ViewerID: p.viewerID(), CursorID: after, Lim: limit + 1,
	})
	if err != nil {
		return nil, err
	}
	out := oas.GetStatusIncidentReply200JSONResponse{
		Id: reply.ID, IncidentId: reply.IncidentID, Close: reply.Close, Queued: reply.Queued, CreatedAt: reply.CreatedAt, Items: []oas.StatusIncidentReplyTarget{},
	}
	for _, c := range counts {
		switch c.Outcome {
		case outcomePending:
			out.Counts.Pending = c.N
		case outcomeSent:
			out.Counts.Sent = c.N
		case outcomeSkipped:
			out.Counts.Skipped = c.N
		case outcomeFailed:
			out.Counts.Failed = c.N
		}
	}
	if len(rows) > int(limit) {
		rows = rows[:limit]
		cur := encodeCursor(reply.CreatedAt, rows[len(rows)-1].ConversationID)
		out.NextCursor = &cur
	}
	for _, r := range rows {
		out.Items = append(out.Items, oas.StatusIncidentReplyTarget{
			ConversationId: r.ConversationID, Outcome: oas.StatusIncidentReplyOutcome(r.Outcome), Reason: r.Reason, FinishedAt: r.FinishedAt,
		})
	}
	return out, nil
}

func (s *Server) statusReplyPrincipal(ctx context.Context, reply store.StatusIncidentReply) (principal, bool, error) {
	ws, err := s.st.GetWorkspace(ctx, reply.WorkspaceID)
	if store.IsNotFound(err) || (err == nil && ws.DeletedAt != nil) {
		return principal{}, false, nil
	}
	if err != nil {
		return principal{}, false, err
	}
	p := principal{workspaceID: reply.WorkspaceID, via: reply.Via, mcpOnly: reply.McpOnly, botsMaySend: ws.BotsMaySend, broadcastsByKeys: ws.BroadcastsByKeys}
	switch {
	case reply.ApiKeyID != nil:
		k, err := s.st.GetAPIKey(ctx, store.GetAPIKeyParams{WorkspaceID: reply.WorkspaceID, ID: *reply.ApiKeyID})
		if store.IsNotFound(err) {
			return p, false, nil
		}
		if err != nil {
			return p, false, err
		}
		if k.RevokedAt != nil || (k.ExpiresAt != nil && !s.now().Before(*k.ExpiresAt)) {
			return p, false, nil
		}
		p.keyID, p.scopes, p.limited = k.ID, k.Scopes, k.InboxLimited
	case reply.MemberID != nil:
		m, err := s.st.GetMember(ctx, store.GetMemberParams{WorkspaceID: reply.WorkspaceID, ID: *reply.MemberID})
		if store.IsNotFound(err) {
			return p, false, nil
		}
		if err != nil {
			return p, false, err
		}
		p.memberID, p.role = m.ID, m.Role
		if reply.GrantID != nil {
			g, err := s.st.GetOAuthGrant(ctx, store.GetOAuthGrantParams{WorkspaceID: reply.WorkspaceID, ID: *reply.GrantID})
			if store.IsNotFound(err) {
				return p, false, nil
			}
			if err != nil {
				return p, false, err
			}
			p.grantID, p.scopes = g.ID, g.Scopes
		}
	default:
		return p, false, nil
	}
	return p, true, nil
}

func (s *Server) finishStatusReply(ctx context.Context, a StatusReplyArgs, outcome, reason string) {
	var r *string
	if reason != "" {
		r = &reason
	}
	now := s.now()
	err := s.st.FinishStatusReplyTarget(ctx, store.FinishStatusReplyTargetParams{
		WorkspaceID: a.WorkspaceID, ReplyID: a.ReplyID, ConversationID: a.ConversationID, Outcome: outcome, Reason: r, Now: &now,
	})
	if err != nil {
		s.log.WarnContext(ctx, "status reply outcome not recorded", slog.String("conversation_id", a.ConversationID.String()), slog.Any("error", err))
	}
}

func (s *Server) RunStatusReply(ctx context.Context, a StatusReplyArgs) error {
	pending, err := s.st.StatusReplyTargetOutcome(ctx, store.StatusReplyTargetOutcomeParams{WorkspaceID: a.WorkspaceID, ReplyID: a.ReplyID, ConversationID: a.ConversationID})
	if store.IsNotFound(err) {
		return nil
	}
	if err != nil || pending != outcomePending {
		return err
	}
	outcome, reason, err := s.sendStatusReply(ctx, a)
	if err != nil {
		return err
	}
	s.finishStatusReply(ctx, a, outcome, reason)
	return nil
}

func (s *Server) sendStatusReply(ctx context.Context, a StatusReplyArgs) (outcome, reason string, err error) {
	reply, err := s.st.GetStatusReply(ctx, store.GetStatusReplyParams{WorkspaceID: a.WorkspaceID, ID: a.ReplyID})
	if store.IsNotFound(err) {
		return outcomeSkipped, reasonConvDeleted, nil
	}
	if err != nil {
		return "", "", err
	}
	p, ok, err := s.statusReplyPrincipal(ctx, reply)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return outcomeSkipped, reasonRevoked, nil
	}
	needs := []oas.ApiKeyScope{oas.StatusManage, oas.MessagesWrite}
	if reply.Close {
		needs = append(needs, oas.ConversationsWrite)
	}
	for _, sc := range needs {
		if err := requireScope(p, sc); err != nil {
			return outcomeFailed, err.(*apiError).Code, nil
		}
	}
	c, err := s.st.GetConversation(ctx, store.GetConversationParams{WorkspaceID: a.WorkspaceID, ID: a.ConversationID})
	if store.IsNotFound(err) {
		return outcomeSkipped, reasonConvDeleted, nil
	}
	if err != nil {
		return "", "", err
	}
	if c.Status == string(oas.ConversationStatusClosed) {
		return outcomeSkipped, reasonClosed, nil
	}
	linked, err := s.st.StatusIncidentConversationLinked(ctx, store.StatusIncidentConversationLinkedParams{
		WorkspaceID: a.WorkspaceID, IncidentID: reply.IncidentID, ConversationID: a.ConversationID,
	})
	if err != nil {
		return "", "", err
	}
	if !linked {
		return outcomeSkipped, reasonUnlinked, nil
	}
	clientID := statusReplyClientID(a.ReplyID)
	actx := context.WithValue(ctx, principalKey, p)
	_, err = s.CreateMessage(actx, oas.CreateMessageRequestObject{ConversationId: a.ConversationID, JSONBody: &oas.CreateMessageJSONRequestBody{
		Kind: oas.MessageCreateKindMessage, Body: &reply.Body, ClientId: &clientID,
	}})
	if err == nil && reply.Close {
		closed := oas.ConversationStatusClosed
		_, err = s.UpdateConversation(actx, oas.UpdateConversationRequestObject{ConversationId: a.ConversationID, Body: &oas.ConversationUpdate{Status: &closed}})
	}
	var e *apiError
	if errors.As(err, &e) && e.Status < 500 && e.Status != http.StatusTooManyRequests && e.Status != http.StatusConflict {
		s.log.WarnContext(ctx, "status reply not sent", slog.String("conversation_id", a.ConversationID.String()), slog.String("code", e.Code))
		return outcomeFailed, e.Code, nil
	}
	if err != nil {
		return "", "", err
	}
	return outcomeSent, "", nil
}

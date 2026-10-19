package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/realtime"
	"github.com/productdevbook/yuva/api/internal/store"
)

const statusReplyAttempts = 8

// clientStatus is the status line of an inbox's published page, nil when it has none.
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
	out := oas.ClientInboxStatus{Url: s.statusURL(pg.Slug, ""), State: oas.StatusComponentStateValue(pageState(comps)), Open: []oas.ClientStatusIncident{}}
	for _, i := range incs {
		out.Open = append(out.Open, oas.ClientStatusIncident{
			Id: i.ID, Kind: oas.StatusIncidentKind(i.Kind), Title: i.Title, Impact: oas.StatusIncidentImpact(i.Impact),
			State: oas.StatusIncidentState(i.State), UpdatedAt: i.LastUpdateAt, Url: s.statusURL(pg.Slug, "/incidents/"+i.ID.String()),
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

// statusLinkTargets finds an incident of a page and a conversation, both of which the caller must see.
func statusLinkTargets(ctx context.Context, q *store.Queries, p principal, pageID, incidentID, conversationID uuid.UUID) (store.StatusIncident, store.Conversation, error) {
	pg, err := visibleStatusPage(ctx, q, p, pageID, false)
	if err != nil {
		return store.StatusIncident{}, store.Conversation{}, err
	}
	inc, err := q.GetStatusIncident(ctx, store.GetStatusIncidentParams{WorkspaceID: p.workspaceID, PageID: pg.ID, ID: incidentID})
	if store.IsNotFound(err) {
		return inc, store.Conversation{}, errStatusIncidentGone
	}
	if err != nil {
		return inc, store.Conversation{}, err
	}
	c, err := visibleConversation(ctx, q, p, conversationID, false)
	return inc, c, err
}

// linkIncidentConversation links a conversation to an incident; linking twice keeps the first link.
func linkIncidentConversation(ctx context.Context, q *store.Queries, ws, incidentID, conversationID uuid.UUID, member, key *uuid.UUID, now time.Time) error {
	return q.LinkStatusIncidentConversation(ctx, store.LinkStatusIncidentConversationParams{
		WorkspaceID: ws, IncidentID: incidentID, ConversationID: conversationID, LinkedByMemberID: member, LinkedByApiKeyID: key, Now: now,
	})
}

func (s *Server) LinkStatusIncidentConversation(ctx context.Context, req oas.LinkStatusIncidentConversationRequestObject) (oas.LinkStatusIncidentConversationResponseObject, error) {
	p := principalFrom(ctx)
	inc, c, err := statusLinkTargets(ctx, s.st.Queries, p, req.PageId, req.IncidentId, req.ConversationId)
	if err != nil {
		return nil, err
	}
	member, key := statusAuthor(p)
	if err := linkIncidentConversation(ctx, s.st.Queries, p.workspaceID, inc.ID, c.ID, member, key, s.now()); err != nil {
		return nil, err
	}
	return oas.LinkStatusIncidentConversation204Response{}, nil
}

func (s *Server) UnlinkStatusIncidentConversation(ctx context.Context, req oas.UnlinkStatusIncidentConversationRequestObject) (oas.UnlinkStatusIncidentConversationResponseObject, error) {
	p := principalFrom(ctx)
	inc, c, err := statusLinkTargets(ctx, s.st.Queries, p, req.PageId, req.IncidentId, req.ConversationId)
	if err != nil {
		return nil, err
	}
	if err := s.st.UnlinkStatusIncidentConversation(ctx, store.UnlinkStatusIncidentConversationParams{
		WorkspaceID: p.workspaceID, IncidentID: inc.ID, ConversationID: c.ID,
	}); err != nil {
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
			Url: s.statusURL(r.PageSlug, "/incidents/"+i.ID.String()), PublishedAt: i.PublishedAt, ResolvedAt: i.ResolvedAt, LinkedAt: r.LinkedAt,
		})
	}
	return out, nil
}

// StatusReplyArgs posts one "Reply to linked" reply into one conversation as the member or API key
// that asked for it. The message carries a client id made from ReplyID, so a retried job finds the
// reply it already posted instead of sending it again.
type StatusReplyArgs struct {
	WorkspaceID    uuid.UUID  `json:"workspace_id"`
	IncidentID     uuid.UUID  `json:"incident_id"`
	ConversationID uuid.UUID  `json:"conversation_id"`
	ReplyID        uuid.UUID  `json:"reply_id"`
	Body           string     `json:"body"`
	Close          bool       `json:"close,omitempty"`
	MemberID       *uuid.UUID `json:"member_id,omitempty"`
	APIKeyID       *uuid.UUID `json:"api_key_id,omitempty"`
	Via            string     `json:"via,omitempty"`
	MCPOnly        bool       `json:"mcp_only,omitempty"`
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
	return w.s.RunStatusReply(ctx, job.Args)
}

func statusReplyClientID(reply uuid.UUID) string { return "status-reply:" + reply.String() }

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
	var queued int32
	err := s.inTx(ctx, p.workspaceID, func(q *store.Queries, ev *eventBatch) error {
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
		reply := newID()
		for _, id := range ids {
			ev.job(StatusReplyArgs{
				WorkspaceID: p.workspaceID, IncidentID: inc.ID, ConversationID: id, ReplyID: reply, Body: b.Body, Close: closeAfter,
				MemberID: member, APIKeyID: key, Via: p.via, MCPOnly: p.mcpOnly,
			}, nil)
		}
		queued = int32(len(ids))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return oas.ReplyToStatusIncidentConversations202JSONResponse{Queued: queued}, nil
}

// statusReplyPrincipal is the member or API key a reply job acts as, as it is now: a member who left
// or a key that was revoked or expired sends nothing.
func (s *Server) statusReplyPrincipal(ctx context.Context, a StatusReplyArgs) (principal, bool, error) {
	ws, err := s.st.GetWorkspace(ctx, a.WorkspaceID)
	if store.IsNotFound(err) || (err == nil && ws.DeletedAt != nil) {
		return principal{}, false, nil
	}
	if err != nil {
		return principal{}, false, err
	}
	p := principal{workspaceID: a.WorkspaceID, via: a.Via, mcpOnly: a.MCPOnly, botsMaySend: ws.BotsMaySend, broadcastsByKeys: ws.BroadcastsByKeys}
	switch {
	case a.APIKeyID != nil:
		k, err := s.st.GetAPIKey(ctx, store.GetAPIKeyParams{WorkspaceID: a.WorkspaceID, ID: *a.APIKeyID})
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
	case a.MemberID != nil:
		m, err := s.st.GetMember(ctx, store.GetMemberParams{WorkspaceID: a.WorkspaceID, ID: *a.MemberID})
		if store.IsNotFound(err) {
			return p, false, nil
		}
		if err != nil {
			return p, false, err
		}
		p.memberID, p.role = m.ID, m.Role
	default:
		return p, false, nil
	}
	return p, true, nil
}

// RunStatusReply posts the reply through the normal reply path, so it goes out on the conversation's
// own channel, then closes the conversation when asked. A conversation closed before its turn gets nothing.
func (s *Server) RunStatusReply(ctx context.Context, a StatusReplyArgs) error {
	p, ok, err := s.statusReplyPrincipal(ctx, a)
	if err != nil || !ok {
		return err
	}
	c, err := s.st.GetConversation(ctx, store.GetConversationParams{WorkspaceID: a.WorkspaceID, ID: a.ConversationID})
	if store.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if c.Status == string(oas.ConversationStatusClosed) {
		return nil
	}
	linked, err := s.st.StatusIncidentConversationLinked(ctx, store.StatusIncidentConversationLinkedParams{
		WorkspaceID: a.WorkspaceID, IncidentID: a.IncidentID, ConversationID: a.ConversationID,
	})
	if err != nil || !linked {
		return err
	}
	clientID := statusReplyClientID(a.ReplyID)
	actx := context.WithValue(ctx, principalKey, p)
	_, err = s.CreateMessage(actx, oas.CreateMessageRequestObject{ConversationId: a.ConversationID, JSONBody: &oas.CreateMessageJSONRequestBody{
		Kind: oas.MessageCreateKindMessage, Body: &a.Body, ClientId: &clientID,
	}})
	if err == nil && a.Close {
		closed := oas.ConversationStatusClosed
		_, err = s.UpdateConversation(actx, oas.UpdateConversationRequestObject{ConversationId: a.ConversationID, Body: &oas.ConversationUpdate{Status: &closed}})
	}
	var e *apiError
	if errors.As(err, &e) && e.Status < 500 && e.Status != http.StatusTooManyRequests && e.Status != http.StatusConflict {
		s.log.WarnContext(ctx, "status reply not sent", slog.String("conversation_id", a.ConversationID.String()), slog.String("code", e.Code))
		return nil
	}
	return err
}

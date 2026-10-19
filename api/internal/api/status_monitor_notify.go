package api

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/productdevbook/yuva/api/internal/mail"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

// StatusMonitorNotifyArgs tells the members of a monitor's inbox that it went down or came back.
type StatusMonitorNotifyArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	MonitorID   uuid.UUID `json:"monitor_id"`
	Down        bool      `json:"down"`
}

func (StatusMonitorNotifyArgs) Kind() string { return "status_monitor_notify" }

type statusMonitorNotifyWorker struct {
	river.WorkerDefaults[StatusMonitorNotifyArgs]
	s *Server
}

func (w *statusMonitorNotifyWorker) Work(ctx context.Context, job *river.Job[StatusMonitorNotifyArgs]) error {
	return w.s.NotifyStatusMonitor(ctx, job.Args)
}

type statusMonitorMail struct {
	Inbox, Page, Monitor, Component, State, Message, URL, SettingsURL string
	Down                                                              bool
}

func statusMonitorPath(ws, page uuid.UUID) string {
	return "/status-pages/" + page.String() + "/monitors?workspace_id=" + ws.String()
}

// NotifyStatusMonitor notifies the inbox's members as a new conversation of the inbox does: by push
// and e-mail as each member chose for new conversations, members who are away left out.
func (s *Server) NotifyStatusMonitor(ctx context.Context, a StatusMonitorNotifyArgs) error {
	ws := a.WorkspaceID
	if live, err := s.workspaceLive(ctx, ws); err != nil || !live {
		return err
	}
	m, err := s.st.GetStatusMonitorByID(ctx, store.GetStatusMonitorByIDParams{WorkspaceID: ws, ID: a.MonitorID})
	if store.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	pg, err := s.st.GetStatusPage(ctx, store.GetStatusPageParams{WorkspaceID: ws, ID: m.PageID})
	if err != nil {
		return err
	}
	in, err := s.st.GetInbox(ctx, store.GetInboxParams{WorkspaceID: ws, ID: pg.InboxID})
	if err != nil {
		return err
	}
	comp, err := s.st.GetStatusComponent(ctx, store.GetStatusComponentParams{WorkspaceID: ws, PageID: pg.ID, ID: m.ComponentID})
	if err != nil {
		return err
	}
	ev := oas.NewAsyncConversation
	if in.Mode == string(oas.Live) {
		ev = oas.NewLiveConversation
	}
	candidates, err := s.st.ListNotificationCandidates(ctx, store.ListNotificationCandidatesParams{WorkspaceID: ws, InboxID: in.ID})
	if err != nil {
		return err
	}
	overrides, err := s.st.ListInboxNotificationsOfInbox(ctx, store.ListInboxNotificationsOfInboxParams{WorkspaceID: ws, InboxID: in.ID})
	if err != nil {
		return err
	}
	perMember := make(map[uuid.UUID]oas.NotificationEventsUpdate, len(overrides))
	for _, o := range overrides {
		perMember[o.MemberID] = parseNotificationUpdate(o.Events)
	}
	data := statusMonitorMail{
		Inbox: in.Name, Page: pg.Name, Monitor: m.Name, Component: comp.Name, Down: a.Down,
		URL:         s.auth.PublicURL + statusMonitorPath(ws, pg.ID),
		SettingsURL: s.auth.PublicURL + "/settings/notifications?workspace_id=" + ws.String(),
	}
	if m.LastMessage != nil {
		data.Message = *m.LastMessage
	}
	now := s.now()
	var jobs []river.InsertManyParams
	for _, r := range candidates {
		if r.Availability == "away" {
			continue
		}
		events := applyNotificationEvents(roleNotificationDefaults(r.Role), parseNotificationUpdate(r.NotificationEvents))
		ch := notificationChannelsFor(applyNotificationEvents(events, perMember[r.ID]), ev)
		mailData := data
		if a.Down && m.DownState != nil {
			mailData.State = statusWordsFor(r.Locale).States[*m.DownState]
		}
		if ch.Push && s.push != nil {
			subs, err := s.st.ListLivePushSubscriptionIDs(ctx, store.ListLivePushSubscriptionIDsParams{PersonID: r.PersonID, Now: now})
			if err != nil {
				return err
			}
			if len(subs) > 0 {
				msg, err := mail.Render("push_status_monitor", r.Locale, mailData)
				if err != nil {
					return err
				}
				inbox := in.ID
				payload := oas.PushNotification{
					Event: ev, Title: msg.Subject, Body: strings.TrimSpace(msg.Text), Url: statusMonitorPath(ws, pg.ID),
					Tag: "status-monitor-" + m.ID.String(), WorkspaceId: &ws, InboxId: &inbox,
				}
				for _, id := range subs {
					jobs = append(jobs, river.InsertManyParams{Args: PushArgs{
						WorkspaceID: &ws, SubscriptionID: id, Payload: mustJSON(payload), TTL: int(liveNotificationTTL / time.Second), Urgency: "high",
						Topic: strings.ReplaceAll(m.ID.String(), "-", ""),
					}, InsertOpts: pushOpts()})
				}
			}
		}
		if ch.Email {
			msg, err := mail.Render("status_monitor", r.Locale, mailData)
			if err != nil {
				return err
			}
			msg.To = r.Email
			if err := s.mailer.Send(ctx, msg); err != nil {
				s.log.WarnContext(ctx, "status monitor e-mail", slog.Any("error", err))
			}
		}
	}
	if len(jobs) == 0 {
		return nil
	}
	_, err = s.jobs.InsertMany(ctx, jobs)
	return err
}

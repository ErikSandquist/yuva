package api

import (
	"context"
	"slices"
	"strings"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const statusListSuffix = " (status)"

func noStatusEmail() oas.StatusEmailSubscribers {
	return oas.StatusEmailSubscribers{Warnings: []oas.StatusEmailSubscribersWarnings{}}
}

func (s *Server) statusEmailBody(ctx context.Context, q *store.Queries, pg store.StatusPage) (oas.StatusEmailSubscribers, error) {
	out := noStatusEmail()
	out.Enabled = pg.EmailEnabled
	if pg.MailingListID == nil {
		return out, nil
	}
	l, err := q.GetMailingList(ctx, store.GetMailingListParams{WorkspaceID: pg.WorkspaceID, ID: *pg.MailingListID})
	if store.IsNotFound(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.ListId, out.ChannelId = &l.ID, l.ChannelID
	counts, err := q.CountSubscriptions(ctx, store.CountSubscriptionsParams{WorkspaceID: pg.WorkspaceID, ListIds: []uuid.UUID{l.ID}})
	if err != nil {
		return out, err
	}
	for _, c := range counts {
		out.Counts = oas.MailingListCounts{Subscribed: c.Subscribed, Pending: c.Pending, Unsubscribed: c.Unsubscribed}
	}
	in, err := q.GetInbox(ctx, store.GetInboxParams{WorkspaceID: pg.WorkspaceID, ID: pg.InboxID})
	if err != nil {
		return out, err
	}
	if in.SenderPostalAddress == nil || *in.SenderPostalAddress == "" {
		out.Warnings = append(out.Warnings, oas.StatusWarningNoPostalAddress)
	}
	if l.ChannelID == nil {
		return out, nil
	}
	ch, err := q.GetEmailChannel(ctx, store.GetEmailChannelParams{WorkspaceID: pg.WorkspaceID, ChannelID: *l.ChannelID})
	if store.IsNotFound(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	_, ok := listSender(ch)
	out.SenderReady = &ok
	row, err := q.GetDeliverabilityCheck(ctx, store.GetDeliverabilityCheckParams{WorkspaceID: pg.WorkspaceID, ChannelID: ch.ChannelID})
	if err != nil && !store.IsNotFound(err) {
		return out, err
	}
	checked := false
	if err == nil {
		c := s.deliverabilityBody(row)
		if c.State == oas.DeliverabilityReceived || c.State == oas.DeliverabilityUnverifiable {
			checked = c.Spf.Status != oas.AuthFail && c.Dkim.Status != oas.AuthFail && c.Dmarc.Status != oas.AuthFail
		}
		if c.State == oas.DeliverabilityReceived && c.Dkim.Status == oas.AuthPass && !c.Dkim.Aligned {
			out.Warnings = append(out.Warnings, oas.StatusWarningDkimUnaligned)
		}
	}
	if !checked {
		out.Warnings = append(out.Warnings, oas.StatusWarningNoDeliverabilityCheck)
	}
	return out, nil
}

func statusListName(ctx context.Context, q *store.Queries, pg store.StatusPage) (string, error) {
	taken, err := q.MailingListNameTaken(ctx, store.MailingListNameTakenParams{WorkspaceID: pg.WorkspaceID, InboxID: pg.InboxID, Name: pg.Name})
	if err != nil || !taken {
		return pg.Name, err
	}
	name := []rune(pg.Name)
	if max := 100 - len([]rune(statusListSuffix)); len(name) > max {
		name = name[:max]
	}
	return string(name) + statusListSuffix, nil
}

func (s *Server) setStatusEmail(ctx context.Context, q *store.Queries, pg store.StatusPage, in oas.StatusEmailSubscribersUpdate) (store.StatusPage, error) {
	var list *store.MailingList
	if pg.MailingListID != nil {
		l, err := q.LockMailingList(ctx, store.LockMailingListParams{WorkspaceID: pg.WorkspaceID, ID: *pg.MailingListID})
		if err != nil && !store.IsNotFound(err) {
			return pg, err
		}
		if err == nil {
			list = &l
		}
	}
	channel := (*uuid.UUID)(nil)
	if list != nil {
		channel = list.ChannelID
	}
	if in.ChannelId != nil {
		if err := listChannel(ctx, q, pg.WorkspaceID, pg.InboxID, *in.ChannelId); err != nil {
			return pg, errSenderNotReady
		}
		channel = in.ChannelId
	}
	enabled := boolOr(in.Enabled, pg.EmailEnabled)
	if enabled {
		if channel == nil {
			return pg, errSenderNotReady
		}
		if err := listChannel(ctx, q, pg.WorkspaceID, pg.InboxID, *channel); err != nil {
			return pg, errSenderNotReady
		}
	}
	now := s.now()
	switch {
	case list == nil && (enabled || channel != nil):
		name, err := statusListName(ctx, q, pg)
		if err != nil {
			return pg, err
		}
		l, err := q.CreateMailingList(ctx, store.CreateMailingListParams{
			ID: newID(), WorkspaceID: pg.WorkspaceID, InboxID: pg.InboxID, ChannelID: channel, Kind: listKindStatus, Name: name,
			DoubleOptIn: true, Now: now,
		})
		if store.IsUniqueViolation(err) {
			return pg, errListNameTaken
		}
		if err != nil {
			return pg, err
		}
		list = &l
	case list != nil && !uuidPtrEqual(list.ChannelID, channel):
		if _, err := q.UpdateMailingList(ctx, store.UpdateMailingListParams{
			WorkspaceID: pg.WorkspaceID, ID: list.ID, Name: list.Name, Description: list.Description, DoubleOptIn: true,
			ChannelID: channel, ArchivedAt: list.ArchivedAt, Now: now,
		}); err != nil {
			return pg, err
		}
	}
	var listID *uuid.UUID
	if list != nil {
		listID = &list.ID
	}
	return q.SetStatusPageEmail(ctx, store.SetStatusPageEmailParams{WorkspaceID: pg.WorkspaceID, ID: pg.ID, MailingListID: listID, EmailEnabled: enabled, Now: now})
}

func uuidPtrEqual(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func statusListPage(ctx context.Context, q *store.Queries, l store.MailingList) (store.StatusPage, bool, error) {
	if l.Kind != listKindStatus {
		return store.StatusPage{}, false, nil
	}
	pg, err := q.GetStatusPageByList(ctx, store.GetStatusPageByListParams{WorkspaceID: l.WorkspaceID, MailingListID: &l.ID})
	if store.IsNotFound(err) {
		return pg, false, nil
	}
	return pg, err == nil, err
}

// A real choice sets chose_components, which outlives the rows so that deleting every chosen
// component never widens it to all.
func setSubscriptionComponents(ctx context.Context, q *store.Queries, sub store.ListSubscription, visible, chosen []uuid.UUID) error {
	if err := q.ClearSubscriptionComponents(ctx, store.ClearSubscriptionComponentsParams{WorkspaceID: sub.WorkspaceID, SubscriptionID: sub.ID}); err != nil {
		return err
	}
	var keep []uuid.UUID
	for _, id := range chosen {
		if slices.Contains(visible, id) && !slices.Contains(keep, id) {
			keep = append(keep, id)
		}
	}
	if len(keep) == 0 || len(keep) == len(visible) {
		return q.SetSubscriptionChoseComponents(ctx, store.SetSubscriptionChoseComponentsParams{WorkspaceID: sub.WorkspaceID, ID: sub.ID, Chose: false})
	}
	if err := q.SetSubscriptionChoseComponents(ctx, store.SetSubscriptionChoseComponentsParams{WorkspaceID: sub.WorkspaceID, ID: sub.ID, Chose: true}); err != nil {
		return err
	}
	for _, id := range keep {
		if err := q.AddSubscriptionComponent(ctx, store.AddSubscriptionComponentParams{WorkspaceID: sub.WorkspaceID, SubscriptionID: sub.ID, ComponentID: id}); err != nil {
			return err
		}
	}
	return nil
}

func subscriptionComponentIDs(ctx context.Context, q *store.Queries, sub store.ListSubscription) ([]uuid.UUID, error) {
	rows, err := q.ListSubscriptionComponents(ctx, store.ListSubscriptionComponentsParams{WorkspaceID: sub.WorkspaceID, SubscriptionIds: []uuid.UUID{sub.ID}})
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		out[i] = r.ComponentID
	}
	return out, nil
}

func visibleComponents(ctx context.Context, q *store.Queries, pg store.StatusPage) ([]store.StatusComponent, error) {
	all, err := q.ListStatusComponents(ctx, store.ListStatusComponentsParams{WorkspaceID: pg.WorkspaceID, PageID: pg.ID})
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(c store.StatusComponent) bool { return c.Hidden }), nil
}

func (s *Server) statusConfirmData(ctx context.Context, pg store.StatusPage, sub store.ListSubscription) (map[string]string, error) {
	name := []rune(pg.Name)
	if len(name) > 100 {
		name = name[:100]
	}
	out := map[string]string{"Page": string(name), "URL": s.confirmURL(sub)}
	chosen, err := subscriptionComponentIDs(ctx, s.st.Queries, sub)
	if err != nil || len(chosen) == 0 {
		return out, err
	}
	visible, err := visibleComponents(ctx, s.st.Queries, pg)
	if err != nil {
		return out, err
	}
	var names []string
	for _, c := range visible {
		if slices.Contains(chosen, c.ID) {
			names = append(names, c.Name)
		}
	}
	out["Components"] = strings.Join(names, ", ")
	return out, nil
}

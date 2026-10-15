package api

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

func (s *Server) GetHome(ctx context.Context, _ oas.GetHomeRequestObject) (oas.GetHomeResponseObject, error) {
	p := principalFrom(ctx)
	inbox, err := s.homeInbox(ctx, p)
	if err != nil {
		return nil, err
	}
	out := oas.GetHome200JSONResponse{Inbox: inbox}
	if s.analytics != nil {
		if out.Analytics, err = s.homeAnalytics(ctx, p); err != nil {
			return nil, err
		}
	}
	if s.broadcastMode() != broadcastsOff {
		if out.Broadcasts, err = s.homeBroadcasts(ctx, p); err != nil {
			return nil, err
		}
	}
	if p.seesAllInboxes() {
		parts, err := s.setupParts(ctx, p, setupWant{inboxes: true, channels: true, properties: true, lists: true})
		if err != nil {
			return nil, err
		}
		out.Setup = &oas.HomeSetup{Items: parts.checklist()}
	}
	return out, nil
}

func (s *Server) homeInbox(ctx context.Context, p principal) (oas.HomeInbox, error) {
	r, err := s.st.HomeInbox(ctx, store.HomeInboxParams{WorkspaceID: p.workspaceID, AllInboxes: p.seesAllInboxes(), ViewerID: p.viewerID()})
	if err != nil {
		return oas.HomeInbox{}, err
	}
	out := oas.HomeInbox{Open: r.Open, Unassigned: r.Unassigned, Waiting: r.Waiting}
	if r.OldestID != nil && r.OldestContactID != nil {
		w := oas.HomeWaitingConversation{
			Id: *r.OldestID, InboxId: *r.OldestInboxID, Subject: derefString(r.OldestSubject), Since: *r.OldestSince,
			Contact: oas.HomeContact{Id: *r.OldestContactID, Name: derefString(r.OldestContactName)},
		}
		if r.OldestContactEmail != nil {
			w.Contact.Email = new(openapi_types.Email(*r.OldestContactEmail))
		}
		out.OldestWaiting = &w
	}
	return out, nil
}

func (s *Server) homeAnalytics(ctx context.Context, p principal) (*oas.HomeAnalytics, error) {
	props, err := s.st.ListAnalyticsProperties(ctx, store.ListAnalyticsPropertiesParams{WorkspaceID: p.workspaceID, AllInboxes: p.seesAllInboxes(), ViewerID: p.viewerID()})
	if err != nil {
		return nil, err
	}
	if len(props) == 0 && !p.seesAllInboxes() {
		return nil, nil
	}
	want := make([]analytics.HomeProperty, len(props))
	for i, prop := range props {
		want[i] = analytics.HomeProperty{ID: prop.ID, Loc: propertyLocation(prop.Timezone)}
	}
	visitors, err := s.analytics.store.Home(ctx, p.workspaceID, want, s.rawFrom(), s.now())
	if err != nil {
		return nil, err
	}
	out := &oas.HomeAnalytics{Properties: make([]oas.HomeProperty, len(props))}
	for i, prop := range props {
		v := visitors[prop.ID]
		hp := oas.HomeProperty{
			Id: prop.ID, InboxId: prop.InboxID, Name: prop.Name, Timezone: prop.Timezone, Domains: make([]oas.AnalyticsDomain, len(prop.Domains)),
			Platforms: make([]oas.AnalyticsPlatform, len(prop.Platforms)), RealtimeVisitors: v.Realtime, Days: make([]oas.HomeDay, len(v.Days)),
		}
		for j, d := range prop.Domains {
			hp.Domains[j] = oas.AnalyticsDomain(d)
		}
		for j, pl := range prop.Platforms {
			hp.Platforms[j] = oas.AnalyticsPlatform(pl)
		}
		for j, d := range v.Days {
			hp.Days[j] = oas.HomeDay{Day: openapi_types.Date{Time: d}, Visitors: v.Visitors[j]}
		}
		if n := len(v.Visitors); n > 0 {
			hp.VisitorsToday = v.Visitors[n-1]
		}
		out.Properties[i] = hp
	}
	return out, nil
}

func (s *Server) homeBroadcasts(ctx context.Context, p principal) (*oas.HomeBroadcasts, error) {
	b, err := s.st.HomeLastBroadcast(ctx, store.HomeLastBroadcastParams{WorkspaceID: p.workspaceID, AllInboxes: p.seesAllInboxes(), ViewerID: p.viewerID()})
	if store.IsNotFound(err) {
		return &oas.HomeBroadcasts{}, nil
	}
	if err != nil {
		return nil, err
	}
	c := parseCounts(b.Counts)
	sentAt := b.UpdatedAt
	if b.FinishedAt != nil {
		sentAt = *b.FinishedAt
	}
	return &oas.HomeBroadcasts{LastSent: &oas.HomeBroadcast{
		Id: b.ID, InboxId: b.InboxID, Name: b.Name, Subject: b.Subject, SentAt: sentAt, ClickTracking: b.ClickTracking,
		Recipients: c.Recipients, Sent: c.Sent, Clicked: c.Clicked, Replied: c.Replied,
	}}, nil
}

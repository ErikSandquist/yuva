package api

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

// operatorContext lets the command line act as an owner of one workspace through the same
// handlers, validation and events as the HTTP API.
func operatorContext(ctx context.Context, workspaceID uuid.UUID) context.Context {
	return context.WithValue(ctx, principalKey, principal{workspaceID: workspaceID, role: roleOwner})
}

// OperatorCreateInbox creates an inbox for the command line and returns its identity secret once.
func (s *Server) OperatorCreateInbox(ctx context.Context, workspaceID uuid.UUID, body oas.InboxCreate) (oas.Inbox, string, error) {
	res, err := s.CreateInbox(operatorContext(ctx, workspaceID), oas.CreateInboxRequestObject{Body: &body})
	if err != nil {
		return oas.Inbox{}, "", err
	}
	created, ok := res.(oas.CreateInbox201JSONResponse)
	if !ok {
		return oas.Inbox{}, "", fmt.Errorf("unexpected response %T", res)
	}
	return created.Inbox, created.IdentitySecret, nil
}

func (s *Server) OperatorListInboxes(ctx context.Context, workspaceID uuid.UUID) ([]oas.Inbox, error) {
	res, err := s.ListInboxes(operatorContext(ctx, workspaceID), oas.ListInboxesRequestObject{})
	if err != nil {
		return nil, err
	}
	list, ok := res.(oas.ListInboxes200JSONResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response %T", res)
	}
	return list.Items, nil
}

func (s *Server) OperatorCreateChannel(ctx context.Context, workspaceID, inboxID uuid.UUID, body oas.ChannelCreate) (oas.Channel, error) {
	res, err := s.CreateChannel(operatorContext(ctx, workspaceID), oas.CreateChannelRequestObject{InboxId: inboxID, Body: &body})
	if err != nil {
		return oas.Channel{}, err
	}
	created, ok := res.(oas.CreateChannel201JSONResponse)
	if !ok {
		return oas.Channel{}, fmt.Errorf("unexpected response %T", res)
	}
	return oas.Channel(created), nil
}

func (s *Server) OperatorListChannels(ctx context.Context, workspaceID, inboxID uuid.UUID) ([]oas.Channel, error) {
	res, err := s.ListChannels(operatorContext(ctx, workspaceID), oas.ListChannelsRequestObject{InboxId: inboxID})
	if err != nil {
		return nil, err
	}
	list, ok := res.(oas.ListChannels200JSONResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response %T", res)
	}
	return list.Items, nil
}

func (s *Server) OperatorCreateAnalyticsProperty(ctx context.Context, workspaceID uuid.UUID, body oas.AnalyticsPropertyCreate) (oas.AnalyticsProperty, error) {
	res, err := s.CreateAnalyticsProperty(operatorContext(ctx, workspaceID), oas.CreateAnalyticsPropertyRequestObject{Body: &body})
	if err != nil {
		return oas.AnalyticsProperty{}, err
	}
	created, ok := res.(oas.CreateAnalyticsProperty201JSONResponse)
	if !ok {
		return oas.AnalyticsProperty{}, fmt.Errorf("unexpected response %T", res)
	}
	return oas.AnalyticsProperty(created), nil
}

func (s *Server) OperatorListAnalyticsProperties(ctx context.Context, workspaceID uuid.UUID) (oas.AnalyticsPropertyList, error) {
	res, err := s.ListAnalyticsProperties(operatorContext(ctx, workspaceID), oas.ListAnalyticsPropertiesRequestObject{})
	if err != nil {
		return oas.AnalyticsPropertyList{}, err
	}
	list, ok := res.(oas.ListAnalyticsProperties200JSONResponse)
	if !ok {
		return oas.AnalyticsPropertyList{}, fmt.Errorf("unexpected response %T", res)
	}
	return oas.AnalyticsPropertyList(list), nil
}

// OperatorLimitAnalytics sets a workspace's monthly analytics events in place of
// YUVA_ANALYTICS_MONTHLY_EVENTS; 0 is no limit.
func (s *Server) OperatorLimitAnalytics(ctx context.Context, workspaceID uuid.UUID, events int64) error {
	if events < 0 {
		return errors.New("the limit must be a number of events, 0 for no limit")
	}
	n, err := s.st.SetWorkspaceAnalyticsLimit(ctx, store.SetWorkspaceAnalyticsLimitParams{ID: workspaceID, MonthlyEvents: &events})
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("no such workspace")
	}
	return nil
}

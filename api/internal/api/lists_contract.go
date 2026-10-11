package api

import (
	"context"
	"net/http"

	"github.com/productdevbook/yuva/api/internal/oas"
)

var errListsNotYet = problem(http.StatusNotImplemented, "not_implemented", "mailing lists are not served yet")

func (s *Server) ListClientMailingLists(context.Context, oas.ListClientMailingListsRequestObject) (oas.ListClientMailingListsResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) CreateClientSubscription(context.Context, oas.CreateClientSubscriptionRequestObject) (oas.CreateClientSubscriptionResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) ListClientSubscriptions(context.Context, oas.ListClientSubscriptionsRequestObject) (oas.ListClientSubscriptionsResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) UnsubscribeClient(context.Context, oas.UnsubscribeClientRequestObject) (oas.UnsubscribeClientResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) SubscribeClient(context.Context, oas.SubscribeClientRequestObject) (oas.SubscribeClientResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) ExportContactConsent(context.Context, oas.ExportContactConsentRequestObject) (oas.ExportContactConsentResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) ListContactSubscriptions(context.Context, oas.ListContactSubscriptionsRequestObject) (oas.ListContactSubscriptionsResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) ListMailingLists(context.Context, oas.ListMailingListsRequestObject) (oas.ListMailingListsResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) CreateMailingList(context.Context, oas.CreateMailingListRequestObject) (oas.CreateMailingListResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) DeleteMailingList(context.Context, oas.DeleteMailingListRequestObject) (oas.DeleteMailingListResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) GetMailingList(context.Context, oas.GetMailingListRequestObject) (oas.GetMailingListResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) UpdateMailingList(context.Context, oas.UpdateMailingListRequestObject) (oas.UpdateMailingListResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) ExportListConsent(context.Context, oas.ExportListConsentRequestObject) (oas.ExportListConsentResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) GetMailingListGrowth(context.Context, oas.GetMailingListGrowthRequestObject) (oas.GetMailingListGrowthResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) ListSubscriptions(context.Context, oas.ListSubscriptionsRequestObject) (oas.ListSubscriptionsResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) CreateSubscription(context.Context, oas.CreateSubscriptionRequestObject) (oas.CreateSubscriptionResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) DeleteSubscription(context.Context, oas.DeleteSubscriptionRequestObject) (oas.DeleteSubscriptionResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) GetSubscription(context.Context, oas.GetSubscriptionRequestObject) (oas.GetSubscriptionResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) UpdateSubscription(context.Context, oas.UpdateSubscriptionRequestObject) (oas.UpdateSubscriptionResponseObject, error) {
	return nil, errListsNotYet
}

func (s *Server) ResendSubscriptionConfirmation(context.Context, oas.ResendSubscriptionConfirmationRequestObject) (oas.ResendSubscriptionConfirmationResponseObject, error) {
	return nil, errListsNotYet
}

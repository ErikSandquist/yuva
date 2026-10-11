package api

import (
	"context"
	"net/http"

	"github.com/productdevbook/yuva/api/internal/oas"
)

var errBroadcastsNotYet = problem(http.StatusNotImplemented, "not_implemented", "broadcasts are not served yet")

func (s *Server) ListBroadcasts(context.Context, oas.ListBroadcastsRequestObject) (oas.ListBroadcastsResponseObject, error) {
	return nil, errBroadcastsNotYet
}

func (s *Server) CreateBroadcast(context.Context, oas.CreateBroadcastRequestObject) (oas.CreateBroadcastResponseObject, error) {
	return nil, errBroadcastsNotYet
}

func (s *Server) GetBroadcast(context.Context, oas.GetBroadcastRequestObject) (oas.GetBroadcastResponseObject, error) {
	return nil, errBroadcastsNotYet
}

func (s *Server) UpdateBroadcast(context.Context, oas.UpdateBroadcastRequestObject) (oas.UpdateBroadcastResponseObject, error) {
	return nil, errBroadcastsNotYet
}

func (s *Server) DeleteBroadcast(context.Context, oas.DeleteBroadcastRequestObject) (oas.DeleteBroadcastResponseObject, error) {
	return nil, errBroadcastsNotYet
}

func (s *Server) CountBroadcastAudience(context.Context, oas.CountBroadcastAudienceRequestObject) (oas.CountBroadcastAudienceResponseObject, error) {
	return nil, errBroadcastsNotYet
}

func (s *Server) ReviewBroadcast(context.Context, oas.ReviewBroadcastRequestObject) (oas.ReviewBroadcastResponseObject, error) {
	return nil, errBroadcastsNotYet
}

func (s *Server) SendBroadcastTest(context.Context, oas.SendBroadcastTestRequestObject) (oas.SendBroadcastTestResponseObject, error) {
	return nil, errBroadcastsNotYet
}

func (s *Server) ScheduleBroadcast(context.Context, oas.ScheduleBroadcastRequestObject) (oas.ScheduleBroadcastResponseObject, error) {
	return nil, errBroadcastsNotYet
}

func (s *Server) UnscheduleBroadcast(context.Context, oas.UnscheduleBroadcastRequestObject) (oas.UnscheduleBroadcastResponseObject, error) {
	return nil, errBroadcastsNotYet
}

func (s *Server) PauseBroadcast(context.Context, oas.PauseBroadcastRequestObject) (oas.PauseBroadcastResponseObject, error) {
	return nil, errBroadcastsNotYet
}

func (s *Server) ResumeBroadcast(context.Context, oas.ResumeBroadcastRequestObject) (oas.ResumeBroadcastResponseObject, error) {
	return nil, errBroadcastsNotYet
}

func (s *Server) CancelBroadcast(context.Context, oas.CancelBroadcastRequestObject) (oas.CancelBroadcastResponseObject, error) {
	return nil, errBroadcastsNotYet
}

func (s *Server) ListBroadcastDeliveries(context.Context, oas.ListBroadcastDeliveriesRequestObject) (oas.ListBroadcastDeliveriesResponseObject, error) {
	return nil, errBroadcastsNotYet
}

func (s *Server) GetBroadcastReport(context.Context, oas.GetBroadcastReportRequestObject) (oas.GetBroadcastReportResponseObject, error) {
	return nil, errBroadcastsNotYet
}

func (s *Server) RequestBroadcastApproval(context.Context, oas.RequestBroadcastApprovalRequestObject) (oas.RequestBroadcastApprovalResponseObject, error) {
	return nil, errBroadcastsNotYet
}

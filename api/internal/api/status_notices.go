package api

import (
	"context"

	"github.com/productdevbook/yuva/api/internal/oas"
)

func (s *Server) ListStatusNotices(ctx context.Context, req oas.ListStatusNoticesRequestObject) (oas.ListStatusNoticesResponseObject, error) {
	return nil, errNotFound
}

func (s *Server) GetStatusNotice(ctx context.Context, req oas.GetStatusNoticeRequestObject) (oas.GetStatusNoticeResponseObject, error) {
	return nil, errNotFound
}

func (s *Server) CancelStatusNotice(ctx context.Context, req oas.CancelStatusNoticeRequestObject) (oas.CancelStatusNoticeResponseObject, error) {
	return nil, errNotFound
}

func (s *Server) GetStatusNoticeReach(ctx context.Context, req oas.GetStatusNoticeReachRequestObject) (oas.GetStatusNoticeReachResponseObject, error) {
	return nil, errNotFound
}

package api

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/store"
)

func (s *Server) FillRateLimiter(now time.Time) {
	for i := range maxBuckets {
		s.limits.allow(fmt.Sprint("fill", i), limit{1, time.Hour}, now)
	}
}

func SetMaxIncidentLinks(n int) (restore func()) {
	old := maxIncidentLinks
	maxIncidentLinks = n
	return func() { maxIncidentLinks = old }
}

func SetStatusReplyBatchSize(n int) (restore func()) {
	old := statusReplyBatchSize
	statusReplyBatchSize = n
	return func() { statusReplyBatchSize = old }
}

func (s *Server) ManageURL(ws, list, contact uuid.UUID) string {
	return s.manageURL(ws, list, contact, uuid.Nil())
}

func SetStatusClaimBudget(n int32) (restore func()) {
	old := claimBudget
	claimBudget = n
	return func() { claimBudget = old }
}

func (s *Server) CleanWorkspaceImages(ctx context.Context, ws uuid.UUID) error {
	return s.cleanUnusedImages(ctx, ws)
}

func (s *Server) CountHelpView(ws, center, article uuid.UUID, locale string) {
	s.countHelpArticle(store.HelpCenter{WorkspaceID: ws, ID: center}, article, locale, helpArticleCount{views: 1})
}

func (s *Server) CountHelpSearch(ws, center uuid.UUID, locale, query string, found bool) {
	s.countHelpSearch(store.HelpCenter{WorkspaceID: ws, ID: center}, locale, query, found)
}

func SetMaxIdeaLinks(n int) (restore func()) {
	old := maxIdeaLinks
	maxIdeaLinks = n
	return func() { maxIdeaLinks = old }
}

func SetRoadmapBatchSize(n int) (restore func()) {
	old := roadmapBatchSize
	roadmapBatchSize = n
	return func() { roadmapBatchSize = old }
}

func SetMaxPendingIdeas(n int) (restore func()) {
	old := maxPendingIdeas
	maxPendingIdeas = n
	return func() { maxPendingIdeas = old }
}

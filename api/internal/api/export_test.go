package api

import (
	"fmt"
	"time"
	"uuid"
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

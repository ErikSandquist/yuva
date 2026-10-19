package api

import (
	"fmt"
	"time"
)

func (s *Server) FillRateLimiter(now time.Time) {
	for i := range maxBuckets {
		s.limits.allow(fmt.Sprint("fill", i), limit{1, time.Hour}, now)
	}
}

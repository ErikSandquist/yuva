package api

import (
	"fmt"
	"math"
	"net/http"
	"net/netip"
	"sync"
	"time"
)

// rateLimited is the 429 for a refused window: the detail names the wait and Retry-After carries
// it in seconds.
func rateLimited(wait time.Duration) *apiError {
	secs := max(int(math.Ceil(wait.Seconds())), 1)
	var in string
	switch mins := (secs + 59) / 60; {
	case secs < 60:
		in = fmt.Sprintf("%d seconds", secs)
	case mins == 1:
		in = "a minute"
	case mins < 60:
		in = fmt.Sprintf("%d minutes", mins)
	case mins == 60:
		in = "an hour"
	default:
		in = fmt.Sprintf("%d hours", (mins+59)/60)
	}
	e := problem(http.StatusTooManyRequests, "rate_limited", "too many requests; try again in "+in)
	e.RetryAfter = secs
	return e
}

type limit struct {
	n      int
	window time.Duration
}

var (
	limitSessionPerIP      = limit{30, time.Minute}
	limitChannelPerIP      = limit{120, time.Minute}
	limitSessionPerChannel = limit{600, time.Minute}
	limitWritePerIP        = limit{60, time.Minute}
	limitWritePerChannel   = limit{1200, time.Minute}
	limitTypingPerSession  = limit{60, time.Minute}
	limitBearer            = limit{600, time.Minute}
	limitRegisterPerIP     = limit{20, time.Hour}
	limitAuthorizePerIP    = limit{60, time.Minute}
)

const (
	maxBuckets                      = 100_000
	defaultAnonymousContactsPerHour = 20
)

type bucket struct {
	start  time.Time
	window time.Duration
	n      int
}

// rateLimiter counts requests per key in fixed windows, in this process only. Finished windows
// are swept every minute; when maxBuckets windows are open, new keys are refused until a sweep
// frees room, so rotating source addresses cannot grow it without bound.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	swept   time.Time
}

func newRateLimiter() *rateLimiter { return &rateLimiter{buckets: map[string]*bucket{}} }

func (l *rateLimiter) sweep(now time.Time) {
	for k, b := range l.buckets {
		if now.Sub(b.start) >= b.window || now.Before(b.start) {
			delete(l.buckets, k)
		}
	}
	l.swept = now
}

func (l *rateLimiter) allow(key string, lim limit, now time.Time) bool {
	ok, _ := l.allowWait(key, lim, now)
	return ok
}

// allowWait is allow that also says, when refused, how long until the window ends.
func (l *rateLimiter) allowWait(key string, lim limit, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.swept) >= time.Minute || now.Before(l.swept) {
		l.sweep(now)
	}
	b := l.buckets[key]
	if b == nil || now.Sub(b.start) >= lim.window || now.Before(b.start) {
		if b == nil && len(l.buckets) >= maxBuckets {
			return false, time.Minute
		}
		b = &bucket{start: now, window: lim.window}
		l.buckets[key] = b
	}
	if b.n >= lim.n {
		return false, b.start.Add(b.window).Sub(now)
	}
	b.n++
	return true, 0
}

// rateIP is the client address as a rate limit key: IPv6 clients usually hold a whole /64.
func rateIP(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	if a = a.Unmap(); a.Is6() {
		return netip.PrefixFrom(a, 64).Masked().String()
	}
	return a.String()
}

func (s *Server) rateLimit(checks ...rateCheck) error {
	now := s.now()
	for _, c := range checks {
		if ok, wait := s.limits.allowWait(c.key, c.lim, now); !ok {
			return rateLimited(wait)
		}
	}
	return nil
}

type rateCheck struct {
	key string
	lim limit
}

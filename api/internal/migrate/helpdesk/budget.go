package helpdesk

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Class is the limit a request counts against: Zendesk's incremental exports have their own.
type Class int

const (
	ClassAPI Class = iota
	ClassIncremental
)

const minute = time.Minute

// Budget is a migration's use of the help desk's rate limit, kept on the migration row so a
// restart does not reset it. Requests come from a bucket that holds a tenth of the per-minute
// budget and refills with the rest over the minute, so no minute ever sees more than half of the
// limit the help desk last reported (Default before it reports one); incremental requests are
// also at most IncrementalMax in any minute.
type Budget struct {
	Limit       int         `json:"limit,omitempty"`
	Tokens      float64     `json:"tokens"`
	At          time.Time   `json:"at,omitzero"`
	Incremental []time.Time `json:"incremental,omitempty"`

	Default        int      `json:"-"`
	IncrementalMax int      `json:"-"`
	LimitHeaders   []string `json:"-"`
}

// PerMinute is the most requests any minute may see.
func (b *Budget) PerMinute() int {
	limit := b.Limit
	if limit <= 0 {
		limit = b.Default * 2
	}
	return max(limit/2, 2)
}

func (b *Budget) capacity() float64 { return max(1, float64(b.PerMinute())/10) }

func (b *Budget) perSecond() float64 {
	return (float64(b.PerMinute()) - b.capacity()) / minute.Seconds()
}

// Take reserves a request at now, or tells when one may be sent.
func (b *Budget) Take(class Class, now time.Time) error {
	if b.At.IsZero() {
		b.Tokens, b.At = b.capacity(), now
	}
	if now.After(b.At) {
		b.Tokens += now.Sub(b.At).Seconds() * b.perSecond()
		b.At = now
	}
	b.Tokens = min(b.Tokens, b.capacity())
	if class == ClassIncremental && b.IncrementalMax > 0 {
		kept := b.Incremental[:0]
		for _, t := range b.Incremental {
			if now.Before(t.Add(minute)) {
				kept = append(kept, t)
			}
		}
		b.Incremental = kept
		if len(kept) >= b.IncrementalMax {
			return &WaitError{Until: kept[0].Add(minute), Incremental: true}
		}
	}
	if b.Tokens < 1 {
		wait := time.Duration((1 - b.Tokens) / b.perSecond() * float64(time.Second))
		return &WaitError{Until: now.Add(max(wait, time.Millisecond))}
	}
	b.Tokens--
	if class == ClassIncremental && b.IncrementalMax > 0 {
		b.Incremental = append(b.Incremental, now)
	}
	return nil
}

// Saw learns the per-minute limit from an answer's headers.
func (b *Budget) Saw(h http.Header) {
	for _, name := range b.LimitHeaders {
		if v, err := strconv.Atoi(strings.TrimSpace(h.Get(name))); err == nil && v > 0 {
			b.Limit = v
			return
		}
	}
}

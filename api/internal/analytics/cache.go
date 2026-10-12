package analytics

import (
	"container/list"
	"context"
	"sync"
	"time"
	"uuid"
)

type saltKey struct {
	property uuid.UUID
	day      time.Time
}

type saltEntry struct {
	salt    []byte
	expires time.Time
}

// Salts keeps each property's salt of the day in memory until it expires; every instance reads the
// one the first of them stored.
type Salts struct {
	store Store
	mu    sync.Mutex
	m     map[saltKey]saltEntry
	swept time.Time
}

func NewSalts(st Store) *Salts { return &Salts{store: st, m: map[saltKey]saltEntry{}} }

// Get returns the salt of a property's local day, which is accepted until expires.
func (s *Salts) Get(ctx context.Context, workspaceID, propertyID uuid.UUID, day, expires, now time.Time) ([]byte, error) {
	k := saltKey{propertyID, day}
	s.mu.Lock()
	if now.Sub(s.swept) > time.Minute {
		for key, e := range s.m {
			if !now.Before(e.expires) {
				delete(s.m, key)
			}
		}
		s.swept = now
	}
	e, ok := s.m[k]
	s.mu.Unlock()
	if ok && now.Before(e.expires) {
		return e.salt, nil
	}
	salt, exp, err := s.store.Salt(ctx, workspaceID, propertyID, day, NewSalt(), expires)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.m[k] = saltEntry{salt, exp}
	s.mu.Unlock()
	return salt, nil
}

// Forget drops a property's salts, after the property or its data was deleted.
func (s *Salts) Forget(propertyID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.m {
		if k.property == propertyID {
			delete(s.m, k)
		}
	}
}

const (
	VisitGap = 30 * time.Minute
	// A visitor with more events in a day, or more page views in a burst, is a bot for the rest of
	// the day.
	BotDailyEvents = 1000
	BotBurstViews  = 200
	BotBurstWindow = 10 * time.Minute
)

type visitKey struct {
	property uuid.UUID
	visitor  int64
}

type visitEntry struct {
	key   visitKey
	day   time.Time
	visit int64
	last  time.Time

	events     int32
	burstStart time.Time
	burstViews int32
	bot        bool
}

// Visits remembers each visitor's current visit, least recently used first out, so most events
// find their visit without a query.
type Visits struct {
	mu    sync.Mutex
	max   int
	order *list.List
	m     map[visitKey]*list.Element
}

func NewVisits(max int) *Visits {
	return &Visits{max: max, order: list.New(), m: map[visitKey]*list.Element{}}
}

// Get returns the visitor's visit of the day and when it was last seen.
func (v *Visits) Get(property uuid.UUID, visitor int64, day time.Time) (int64, time.Time, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	el, ok := v.m[visitKey{property, visitor}]
	if !ok {
		return 0, time.Time{}, false
	}
	e := el.Value.(*visitEntry)
	if !e.day.Equal(day) {
		return 0, time.Time{}, false
	}
	v.order.MoveToFront(el)
	return e.visit, e.last, true
}

// Put records that the visitor's visit was seen at last.
func (v *Visits) Put(property uuid.UUID, visitor int64, day time.Time, visit int64, last time.Time) {
	v.mu.Lock()
	defer v.mu.Unlock()
	k := visitKey{property, visitor}
	if el, ok := v.m[k]; ok {
		e := el.Value.(*visitEntry)
		if e.visit == visit && last.Before(e.last) {
			last = e.last
		}
		if !e.day.Equal(day) {
			e.events, e.burstViews, e.bot = 0, 0, false
		}
		e.day, e.visit, e.last = day, visit, last
		v.order.MoveToFront(el)
		return
	}
	v.m[k] = v.order.PushFront(&visitEntry{key: k, day: day, visit: visit, last: last})
	for v.order.Len() > v.max {
		old := v.order.Back()
		v.order.Remove(old)
		delete(v.m, old.Value.(*visitEntry).key)
	}
}

// Tally counts an event of the visitor's day and reports whether the visitor is a bot from now on;
// first is true for the event that made it one. A visitor not in the cache is not counted.
func (v *Visits) Tally(property uuid.UUID, visitor int64, day, now time.Time, view bool) (bot, first bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	el, ok := v.m[visitKey{property, visitor}]
	if !ok {
		return false, false
	}
	e := el.Value.(*visitEntry)
	if !e.day.Equal(day) {
		return false, false
	}
	if e.bot {
		return true, false
	}
	e.events++
	if view {
		if now.Sub(e.burstStart) >= BotBurstWindow || now.Before(e.burstStart) {
			e.burstStart, e.burstViews = now, 0
		}
		e.burstViews++
	}
	e.bot = e.events > BotDailyEvents || e.burstViews > BotBurstViews
	return e.bot, e.bot
}

// MarkBot remembers that the visitor is a bot for the rest of the day.
func (v *Visits) MarkBot(property uuid.UUID, visitor int64, day time.Time) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if el, ok := v.m[visitKey{property, visitor}]; ok {
		if e := el.Value.(*visitEntry); e.day.Equal(day) {
			e.bot = true
		}
	}
}

func (v *Visits) Forget(property uuid.UUID) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for k, el := range v.m {
		if k.property == property {
			v.order.Remove(el)
			delete(v.m, k)
		}
	}
}

func (v *Visits) Len() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.order.Len()
}

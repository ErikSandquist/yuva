package analytics

import (
	"context"
	"log/slog"
	"sync"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/metrics"
)

const (
	FlushInterval = time.Second
	flushAt       = 2000
	drainTimeout  = 10 * time.Second
	maxBackoff    = 30 * time.Second
)

type countKey struct {
	workspace, property uuid.UUID
	day                 time.Time
	reason              string
}

// Buffer holds events between the handler, which answers before anything is written, and the
// flusher. It never holds more than max events.
type Buffer struct {
	mu     sync.Mutex
	max    int
	events []Event
	drops  map[countKey]int64
	full   chan struct{}
}

func NewBuffer(max int) *Buffer {
	return &Buffer{max: max, drops: map[countKey]int64{}, full: make(chan struct{}, 1)}
}

// Add takes all of evs or, when they do not fit, none.
func (b *Buffer) Add(evs []Event) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.events)+len(evs) > b.max {
		return false
	}
	b.events = append(b.events, evs...)
	metrics.AnalyticsBufferEvents.Set(float64(len(b.events)))
	metrics.AnalyticsEvents.WithLabelValues("accepted").Add(float64(len(evs)))
	if len(b.events) >= flushAt {
		select {
		case b.full <- struct{}{}:
		default:
		}
	}
	return true
}

// Drop counts events of a property's day that were not stored.
func (b *Buffer) Drop(workspaceID, propertyID uuid.UUID, day time.Time, reason string, n int64) {
	metrics.AnalyticsDropped.WithLabelValues(reason).Add(float64(n))
	b.mu.Lock()
	defer b.mu.Unlock()
	b.drops[countKey{workspaceID, propertyID, day, reason}] += n
}

// Count records events of a property's day for the ingest counts without dropping them, such as
// events that had something scrubbed.
func (b *Buffer) Count(workspaceID, propertyID uuid.UUID, day time.Time, reason string, n int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.drops[countKey{workspaceID, propertyID, day, reason}] += n
}

func (b *Buffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}

func (b *Buffer) take() Batch {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := Batch{Events: b.events}
	for k, n := range b.drops {
		out.Drops = append(out.Drops, Count{WorkspaceID: k.workspace, PropertyID: k.property, Day: k.day, Reason: k.reason, N: n})
	}
	b.events, b.drops = nil, map[countKey]int64{}
	metrics.AnalyticsBufferEvents.Set(0)
	return out
}

// putBack returns a batch that could not be written, in front of newer events, as far as it fits.
func (b *Buffer) putBack(batch Batch) {
	b.mu.Lock()
	defer b.mu.Unlock()
	room := max(b.max-len(b.events), 0)
	evs := batch.Events
	if len(evs) > room {
		metrics.AnalyticsDropped.WithLabelValues(ReasonBuffer).Add(float64(len(evs) - room))
		evs = evs[:room]
	}
	b.events = append(append(make([]Event, 0, len(evs)+len(b.events)), evs...), b.events...)
	for _, c := range batch.Drops {
		b.drops[countKey{c.WorkspaceID, c.PropertyID, c.Day, c.Reason}] += c.N
	}
	metrics.AnalyticsBufferEvents.Set(float64(len(b.events)))
}

// Flusher writes the buffer to the store every second, or sooner when it fills up.
type Flusher struct {
	Store  Store
	Buffer *Buffer
	Log    *slog.Logger
	Now    func() time.Time
	// AfterFlush learns each workspace's events of the month after a batch was written.
	AfterFlush func(context.Context, IngestResult)

	mu         sync.Mutex
	partitions map[time.Time]bool
}

// Prepare makes the partitions of yesterday to tomorrow (UTC) exist, which cover today in every
// time zone.
func (f *Flusher) Prepare(ctx context.Context) error {
	today := LocalDay(f.Now(), time.UTC)
	return f.ensure(ctx, []time.Time{today.AddDate(0, 0, -1), today, today.AddDate(0, 0, 1), today.AddDate(0, 0, 2)})
}

func (f *Flusher) ensure(ctx context.Context, days []time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.partitions == nil {
		f.partitions = map[time.Time]bool{}
	}
	for _, d := range days {
		if f.partitions[d] {
			continue
		}
		if err := f.Store.EnsurePartitions(ctx, d, d); err != nil {
			return err
		}
		f.partitions[d] = true
	}
	return nil
}

// Flush writes what the buffer holds; on failure the events go back into the buffer.
func (f *Flusher) Flush(ctx context.Context) error {
	batch := f.Buffer.take()
	if len(batch.Events) == 0 && len(batch.Drops) == 0 {
		return nil
	}
	start := time.Now()
	res, err := f.write(ctx, batch)
	if err != nil {
		f.Buffer.putBack(batch)
		return err
	}
	metrics.AnalyticsFlushSeconds.Observe(time.Since(start).Seconds())
	metrics.AnalyticsEvents.WithLabelValues("stored").Add(float64(len(batch.Events)))
	metrics.AnalyticsLateEvents.Add(float64(res.Late))
	if f.AfterFlush != nil && len(res.Usage) > 0 {
		f.AfterFlush(ctx, res)
	}
	return nil
}

func (f *Flusher) write(ctx context.Context, batch Batch) (IngestResult, error) {
	seen := map[time.Time]bool{}
	var days []time.Time
	for _, e := range batch.Events {
		if !seen[e.Day] {
			seen[e.Day] = true
			days = append(days, e.Day)
		}
	}
	if err := f.ensure(ctx, days); err != nil {
		return IngestResult{}, err
	}
	return f.Store.Ingest(ctx, batch, f.Now())
}

// Run flushes until ctx ends, then drains the buffer for up to ten seconds.
func (f *Flusher) Run(ctx context.Context) {
	tick := time.NewTicker(FlushInterval)
	defer tick.Stop()
	backoff := time.Duration(0)
	var retryAt time.Time
	for {
		select {
		case <-ctx.Done():
			f.drain()
			return
		case <-tick.C:
		case <-f.Buffer.full:
		}
		if time.Now().Before(retryAt) {
			continue
		}
		if err := f.Flush(ctx); err != nil {
			backoff = min(max(backoff*2, FlushInterval), maxBackoff)
			retryAt = time.Now().Add(backoff)
			f.Log.ErrorContext(ctx, "analytics flush", slog.Any("error", err), slog.Duration("retry_in", backoff))
			continue
		}
		backoff = 0
	}
}

func (f *Flusher) drain() {
	ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
	defer cancel()
	for first := true; (first || f.Buffer.Len() > 0) && ctx.Err() == nil; first = false {
		if err := f.Flush(ctx); err != nil {
			f.Log.Error("analytics drain", slog.Any("error", err), slog.Int("events_left", f.Buffer.Len()))
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
			}
		}
	}
}

package analytics

import (
	"cmp"
	"context"
	"encoding/binary"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
)

// FunnelStep is how many visitors (added up per day) and visits reached a step after every step
// before it within one visit.
type FunnelStep struct {
	Visitors, Visits int64
}

// FunnelWorkers spreads the scan of a busy month over more cores; funnels are few and their answers
// cached. FunnelChunkEvents bounds the events of the days read at once, so the steps' events held
// in memory stay in the tens of megabytes even for the busiest properties.
const (
	FunnelWorkers     = "4"
	FunnelChunkEvents = 10_000_000
)

// funnelEvent is an event of a funnel step: at is its time in microseconds shifted left by 8 with
// its step mask below, so it orders by time.
type funnelEvent struct {
	visitor, visit int64
	at             uint64
}

// Funnel follows the visits of a property's days from to through the steps in order: an event
// completes at most one step, the next one the visit has not completed. The database only finds the
// steps' events, unsorted, as bare integers; a visit and a visitor live in one day, so each day's
// events are put in order and followed here, several days at once.
func (s *PGStore) Funnel(ctx context.Context, ws, property uuid.UUID, from, to time.Time, steps []Goal) ([]FunnelStep, error) {
	if len(steps) > 8 {
		return nil, errors.New("analytics: a funnel has at most 8 steps")
	}
	out := make([]FunnelStep, len(steps))
	err := pgx.BeginTxFunc(ctx, s.Pool, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead}, func(tx pgx.Tx) error {
		if err := queryLimits(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT set_config('max_parallel_workers_per_gather', $1, true)`, FunnelWorkers); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT day, events FROM analytics_ingest_stats
			WHERE workspace_id = $1 AND property_id = $2 AND day BETWEEN $3 AND $4 AND reason = 'accepted'`, ws, property, from, to)
		if err != nil {
			return err
		}
		perDay := map[time.Time]int64{}
		for rows.Next() {
			var d time.Time
			var n int64
			if err := rows.Scan(&d, &n); err != nil {
				rows.Close()
				return err
			}
			perDay[d.UTC()] = n
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		start, sum := from, int64(0)
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			if sum += perDay[d]; d.Equal(to) || sum+perDay[d.AddDate(0, 0, 1)] > FunnelChunkEvents {
				if err := funnelChunk(ctx, tx, ws, property, start, d, steps, out); err != nil {
					return err
				}
				start, sum = d.AddDate(0, 0, 1), 0
			}
		}
		return nil
	})
	return out, err
}

// funnelChunk reads the steps' events of the days from to and adds how far their visits got to out.
func funnelChunk(ctx context.Context, tx pgx.Tx, ws, property uuid.UUID, from, to time.Time, steps []Goal, out []FunnelStep) error {
	bots := map[[2]int64]bool{}
	rows, err := tx.Query(ctx, `SELECT (day - DATE '2000-01-01')::int8, visit FROM analytics_visits
		WHERE workspace_id = $1 AND property_id = $2 AND day BETWEEN $3 AND $4 AND bot`, ws, property, from, to)
	if err != nil {
		return err
	}
	for rows.Next() {
		var k [2]int64
		if err := rows.Scan(&k[0], &k[1]); err != nil {
			rows.Close()
			return err
		}
		bots[k] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	sc := newScope(ws, property, from, to)
	masks := make([]string, len(steps))
	conds := make([]string, len(steps))
	for i, g := range steps {
		conds[i] = "(" + g.cond(sc, "e") + ")"
		masks[i] = "CASE WHEN " + conds[i] + " THEN " + strconv.Itoa(1<<i) + " ELSE 0 END"
	}
	rows, err = tx.Query(ctx, `
SELECT (e.day - DATE '2000-01-01')::int8, e.visitor, e.visit, ((extract(epoch FROM e.ts) * 1000000)::int8 << 8) | (`+strings.Join(masks, " | ")+`)
FROM analytics_events e
WHERE e.workspace_id = $1 AND e.property_id = $2 AND e.day BETWEEN $3 AND $4 AND (`+strings.Join(conds, " OR ")+`)`, sc.args...)
	if err != nil {
		return err
	}
	byDay := map[int64][]funnelEvent{}
	for rows.Next() {
		raw := rows.RawValues()
		if len(raw) != 4 || len(raw[0]) != 8 || len(raw[1]) != 8 || len(raw[2]) != 8 || len(raw[3]) != 8 {
			rows.Close()
			return errors.New("analytics: a funnel row is not four integers")
		}
		day, visit := int64(binary.BigEndian.Uint64(raw[0])), int64(binary.BigEndian.Uint64(raw[2]))
		if !bots[[2]int64{day, visit}] {
			byDay[day] = append(byDay[day], funnelEvent{int64(binary.BigEndian.Uint64(raw[1])), visit, binary.BigEndian.Uint64(raw[3])})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	work := make(chan []funnelEvent)
	for range 4 {
		wg.Go(func() {
			counts := make([]FunnelStep, len(steps))
			for evs := range work {
				followDay(evs, len(steps), counts)
			}
			mu.Lock()
			for i, c := range counts {
				out[i].Visitors += c.Visitors
				out[i].Visits += c.Visits
			}
			mu.Unlock()
		})
	}
	for _, evs := range byDay {
		work <- evs
	}
	close(work)
	wg.Wait()
	return nil
}

// followDay counts how far each visit and visitor of one day got.
func followDay(evs []funnelEvent, steps int, out []FunnelStep) {
	slices.SortFunc(evs, func(a, b funnelEvent) int {
		return cmp.Or(cmp.Compare(a.visitor, b.visitor), cmp.Compare(a.visit, b.visit), cmp.Compare(a.at, b.at))
	})
	best, reached := 0, 0
	for i, e := range evs {
		if reached < steps && e.at&(1<<reached) != 0 {
			reached++
		}
		last := i == len(evs)-1
		if last || evs[i+1].visit != e.visit || evs[i+1].visitor != e.visitor {
			for j := 0; j < reached; j++ {
				out[j].Visits++
			}
			best, reached = max(best, reached), 0
		}
		if last || evs[i+1].visitor != e.visitor {
			for j := 0; j < best; j++ {
				out[j].Visitors++
			}
			best = 0
		}
	}
}

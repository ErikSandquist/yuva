package analytics

import (
	"context"
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
)

// FunnelStep is how many visitors (added up per day) and visits reached a step after every step
// before it within one visit.
type FunnelStep struct {
	Visitors, Visits int64
}

// FunnelWorkMem and FunnelWorkers let the sort of a busy month's events stay in memory and spread
// over more cores; funnels are few and their answers cached.
const (
	FunnelWorkMem = "96MB"
	FunnelWorkers = "4"
)

type dayVisit struct {
	day, visit int64
}

// Funnel follows the visits of a property's days from to through the steps in order: an event
// completes at most one step, the next one the visit has not completed. Rows come ordered by day and
// visitor, so a visitor's furthest step is known once their rows are read; they are many, so they
// come as bare integers. Bots' visits are few and left out here rather than joined, which would hash
// the events.
func (s *PGStore) Funnel(ctx context.Context, ws, property uuid.UUID, from, to time.Time, steps []Goal) ([]FunnelStep, error) {
	out := make([]FunnelStep, len(steps))
	err := pgx.BeginTxFunc(ctx, s.Pool, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead}, func(tx pgx.Tx) error {
		if err := queryLimits(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT set_config('work_mem', $1, true), set_config('max_parallel_workers_per_gather', $2, true)`,
			FunnelWorkMem, FunnelWorkers); err != nil {
			return err
		}
		bots := map[dayVisit]bool{}
		rows, err := tx.Query(ctx, `SELECT (day - DATE '2000-01-01')::int8, visit FROM analytics_visits
			WHERE workspace_id = $1 AND property_id = $2 AND day BETWEEN $3 AND $4 AND bot`, ws, property, from, to)
		if err != nil {
			return err
		}
		for rows.Next() {
			var k dayVisit
			if err := rows.Scan(&k.day, &k.visit); err != nil {
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
SELECT (e.day - DATE '2000-01-01')::int8, e.visitor, e.visit, (`+strings.Join(masks, " | ")+`)::int8
FROM analytics_events e
WHERE e.workspace_id = $1 AND e.property_id = $2 AND e.day BETWEEN $3 AND $4 AND (`+strings.Join(conds, " OR ")+`)
ORDER BY e.day, e.visitor, e.visit, e.ts`, sc.args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		var cur dayVisit
		var visitor int64
		started, bot := false, false
		best, reached := 0, 0
		endVisit := func() {
			if !bot {
				for i := 0; i < reached; i++ {
					out[i].Visits++
				}
				best = max(best, reached)
			}
			reached = 0
		}
		endVisitor := func() {
			for i := 0; i < best; i++ {
				out[i].Visitors++
			}
			best = 0
		}
		for rows.Next() {
			raw := rows.RawValues()
			if len(raw) != 4 || len(raw[0]) != 8 || len(raw[1]) != 8 || len(raw[2]) != 8 || len(raw[3]) != 8 {
				return errors.New("analytics: a funnel row is not four integers")
			}
			k := dayVisit{int64(binary.BigEndian.Uint64(raw[0])), int64(binary.BigEndian.Uint64(raw[2]))}
			vr, mask := int64(binary.BigEndian.Uint64(raw[1])), binary.BigEndian.Uint64(raw[3])
			if started && (k != cur || vr != visitor) {
				endVisit()
				if k.day != cur.day || vr != visitor {
					endVisitor()
				}
			}
			if !started || k != cur {
				bot = bots[k]
			}
			started, cur, visitor = true, k, vr
			if reached < len(steps) && mask&(1<<reached) != 0 {
				reached++
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if started {
			endVisit()
			endVisitor()
		}
		return nil
	})
	return out, err
}

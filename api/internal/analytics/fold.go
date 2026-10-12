package analytics

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
)

// FoldMargin is how far behind now a fold of an open day ends. An ingest that writes an event
// older than half of it, or changes a visit that ended that long before a visit gap, moves the
// day's sequences, so a fold never misses a change to the visits it holds.
const FoldMargin = 2 * time.Minute

// A fold of an open day holds the numbers of the visitors whose every visit begun by until had
// ended a visit gap before it (at the cut). Such a visit can no longer change: a later event starts a
// new visit, and anything else moves the day's sequences. A query adds the raw rows of the visitors
// with a visit that ended after the cut and takes away those of their visits the fold holds, so the
// answer is the raw one while each visitor is still counted once a day.
type fold struct {
	day, until time.Time
}

func cutOf(until time.Time) time.Time { return until.Add(-VisitGap) }

// foldedVisits narrows the scope to the visits a fold up to until holds.
func (s *scope) foldedVisits(until time.Time) {
	u, c := s.arg(until), s.arg(cutOf(until))
	s.visitWhere = append(s.visitWhere, "v.started_at <= "+u, `v.visitor NOT IN (SELECT o.visitor FROM analytics_visits o
    WHERE o.workspace_id = $1 AND o.property_id = $2 AND o.day BETWEEN $3 AND $4 AND o.started_at <= `+u+` AND o.ended_at > `+c+`)`)
}

// openVisitors are the visitors of a folded day with a visit that ended after the fold's cut, whom
// the fold does not hold in full, and among them those with such a visit begun by until, of whom it
// holds nothing.
func openVisitors(ctx context.Context, tx pgx.Tx, ws, property uuid.UUID, f fold) (open, mixed []int64, err error) {
	rows, err := tx.Query(ctx, `SELECT visitor, bool_or(started_at <= $4) FROM analytics_visits
		WHERE workspace_id = $1 AND property_id = $2 AND day = $3 AND ended_at > $5 GROUP BY visitor`, ws, property, f.day, f.until, cutOf(f.until))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	open, mixed = []int64{}, []int64{}
	for rows.Next() {
		var v int64
		var m bool
		if err := rows.Scan(&v, &m); err != nil {
			return nil, nil, err
		}
		open = append(open, v)
		if m {
			mixed = append(mixed, v)
		}
	}
	return open, mixed, rows.Err()
}

// ofVisitors narrows the scope to the visits of some visitors; with until, to those a fold up to
// until holds, where mixed are the visitors of whom it holds nothing.
func (s *scope) ofVisitors(visitors []int64, until *time.Time, mixed []int64) {
	s.visitWhere = append(s.visitWhere, "v.visitor = ANY("+s.arg(visitors)+"::bigint[])")
	if until != nil {
		s.visitWhere = append(s.visitWhere, "v.started_at <= "+s.arg(*until), "NOT (v.visitor = ANY("+s.arg(mixed)+"::bigint[]))")
	}
}

func aggregationLimits(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT set_config('jit', 'off', true), set_config('work_mem', $1, true)`, QueryWorkMem)
	return err
}

// FoldDay replaces a property's fold of an open local day with one up to until. It does nothing for
// a day without raw rows or one already rolled up.
func (s *PGStore) FoldDay(ctx context.Context, ws, property uuid.UUID, day time.Time, tz string, until time.Time) error {
	return pgx.BeginTxFunc(ctx, s.Pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead}, func(tx pgx.Tx) error {
		if err := aggregationLimits(ctx, tx); err != nil {
			return err
		}
		var built bool
		var late, eng int64
		err := tx.QueryRow(ctx, `SELECT built_at IS NOT NULL, late_seq, engagement_seq FROM analytics_rollup_days
			WHERE workspace_id = $1 AND property_id = $2 AND day = $3`, ws, property, day).Scan(&built, &late, &eng)
		if errors.Is(err, pgx.ErrNoRows) || built {
			return nil
		}
		if err != nil {
			return err
		}
		if err := deleteFold(ctx, tx, ws, property, day); err != nil {
			return err
		}
		sc := newScope(ws, property, day, day)
		sc.engagement = true
		sc.foldedVisits(until)
		for _, q := range []string{sc.visitSetsSQL(visitDims(), true), sc.pageSetsSQL(false)} {
			if _, err := tx.Exec(ctx, `INSERT INTO analytics_partial_daily (workspace_id, property_id, day, dim, value, visitors, visits, views, bounces,
				duration_s, events, engaged_ms, engaged_views, scroll_sum, scroll_views) SELECT $1, $2, r.* FROM (`+q+`) AS r`, sc.args...); err != nil {
				return err
			}
		}
		hs := newScope(ws, property, day, day)
		hs.foldedVisits(until)
		q := hs.hourlySQL(tz)
		if _, err := tx.Exec(ctx, `INSERT INTO analytics_partial_hourly (workspace_id, property_id, day, hour, visitors, visits, views, events)
			SELECT $1, $2, h.day, h.hour, h.visitors, h.visits, h.views, h.events FROM (`+q+`) AS h(day, hour, visitors, visits, views, events)`,
			hs.args...); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO analytics_partial_days (workspace_id, property_id, day, until_at, late_seq, engagement_seq, built_at)
			VALUES ($1, $2, $3, $4, $5, $6, now())`, ws, property, day, until, late, eng)
		return err
	})
}

func deleteFold(ctx context.Context, tx pgx.Tx, ws, property uuid.UUID, day time.Time) error {
	for _, t := range []string{"analytics_partial_daily", "analytics_partial_hourly", "analytics_partial_days"} {
		if _, err := tx.Exec(ctx, "DELETE FROM "+t+" WHERE workspace_id = $1 AND property_id = $2 AND day = $3", ws, property, day); err != nil {
			return err
		}
	}
	return nil
}

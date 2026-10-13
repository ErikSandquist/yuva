package analytics

import (
	"cmp"
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
)

// visitDims are the dimensions that come from visits alone.
func visitDims() []Dim {
	var out []Dim
	for _, d := range Dims {
		if !d.Page && !d.Event {
			out = append(out, d)
		}
	}
	return out
}

type dailyRow struct {
	day   time.Time
	dim   int16
	value string
	sums  Sums
}

func scanDaily(rows pgx.Rows) ([]dailyRow, error) {
	defer rows.Close()
	var out []dailyRow
	for rows.Next() {
		var r dailyRow
		if err := rows.Scan(append([]any{&r.day, &r.dim, &r.value}, r.sums.scan()...)...); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// foldTop keeps each dimension's top values of a day by visitors and adds the rest up as (other).
func foldTop(rows []dailyRow, top int) []dailyRow {
	byDim := map[int16][]dailyRow{}
	for _, r := range rows {
		byDim[r.dim] = append(byDim[r.dim], r)
	}
	var out []dailyRow
	for _, dim := range slices.Sorted(func(yield func(int16) bool) {
		for k := range byDim {
			if !yield(k) {
				return
			}
		}
	}) {
		list := byDim[dim]
		if dim == DimTotal || len(list) <= top {
			out = append(out, list...)
			continue
		}
		slices.SortFunc(list, func(a, b dailyRow) int {
			return cmp.Or(cmp.Compare(b.sums.Visitors, a.sums.Visitors), cmp.Compare(b.sums.Visits, a.sums.Visits), strings.Compare(a.value, b.value))
		})
		other := dailyRow{day: list[0].day, dim: dim, value: OtherValue}
		for _, r := range list[top:] {
			other.sums.Add(r.sums)
		}
		out = append(append(out, list[:top]...), other)
	}
	return out
}

// RollupResult says whether the day was rolled up for the first time, and its ingest counts.
type RollupResult struct {
	First    bool
	Accepted int64
	// Dropped counts the events refused for any reason but the monthly cap.
	Dropped int64
}

// RollupDay replaces a property's rollups of one local day with fresh ones from its raw rows.
// The day's row in analytics_rollup_days is locked first, so events that arrive meanwhile mark
// the day dirty again rather than being lost.
func (s *PGStore) RollupDay(ctx context.Context, ws, property uuid.UUID, day time.Time, tz string, top int) (RollupResult, error) {
	var res RollupResult
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO analytics_rollup_days (workspace_id, property_id, day) VALUES ($1, $2, $3)
			ON CONFLICT (property_id, day) DO NOTHING`, ws, property, day); err != nil {
			return err
		}
		var built *time.Time
		if err := tx.QueryRow(ctx, `SELECT built_at FROM analytics_rollup_days WHERE workspace_id = $1 AND property_id = $2 AND day = $3 FOR UPDATE`,
			ws, property, day).Scan(&built); err != nil {
			return err
		}
		res.First = built == nil
		for _, t := range []string{"analytics_daily", "analytics_hourly"} {
			if _, err := tx.Exec(ctx, "DELETE FROM "+t+" WHERE workspace_id = $1 AND property_id = $2 AND day = $3", ws, property, day); err != nil {
				return err
			}
		}
		if err := deleteFold(ctx, tx, ws, property, day); err != nil {
			return err
		}
		sc := newScope(ws, property, day, day)
		sc.engagement = true
		var all []dailyRow
		for _, q := range []string{sc.visitSetsSQL(visitDims(), true), sc.pageSetsSQL(true)} {
			rows, err := tx.Query(ctx, q, sc.args...)
			if err != nil {
				return fmt.Errorf("rollup: %w", err)
			}
			got, err := scanDaily(rows)
			if err != nil {
				return fmt.Errorf("rollup: %w", err)
			}
			all = append(all, got...)
		}
		all = foldTop(all, top)
		copyRows := make([][]any, len(all))
		for i, r := range all {
			m := r.sums
			copyRows[i] = []any{ws, property, day, r.dim, r.value, m.Visitors, m.Visits, m.Views, m.Bounces, m.DurationS, m.Events,
				m.EngagedMS, m.EngagedViews, m.ScrollSum, m.ScrollViews}
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"analytics_daily"}, []string{"workspace_id", "property_id", "day", "dim", "value", "visitors",
			"visits", "views", "bounces", "duration_s", "events", "engaged_ms", "engaged_views", "scroll_sum", "scroll_views"},
			pgx.CopyFromRows(copyRows)); err != nil {
			return fmt.Errorf("rollup: %w", err)
		}
		hs := newScope(ws, property, day, day)
		q := hs.hourlySQL(tz)
		if _, err := tx.Exec(ctx, `INSERT INTO analytics_hourly (workspace_id, property_id, day, hour, visitors, visits, views, events)
			SELECT $1, $2, h.day, h.hour, h.visitors, h.visits, h.views, h.events FROM (`+q+`) AS h(day, hour, visitors, visits, views, events)`,
			hs.args...); err != nil {
			return fmt.Errorf("rollup hourly: %w", err)
		}
		if err := buildExtras(ctx, tx, ws, property, day, top); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE analytics_rollup_days SET built_at = now(), dirty = false WHERE workspace_id = $1 AND property_id = $2 AND day = $3`,
			ws, property, day); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE analytics_properties SET rollup_version = rollup_version + 1 WHERE workspace_id = $1 AND id = $2`, ws, property); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT coalesce(sum(events) FILTER (WHERE reason = 'accepted'), 0)::bigint,
			coalesce(sum(events) FILTER (WHERE reason NOT IN ('accepted', 'scrubbed', 'over_limit')), 0)::bigint
			FROM analytics_ingest_stats WHERE workspace_id = $1 AND property_id = $2 AND day = $3`, ws, property, day).Scan(&res.Accepted, &res.Dropped)
	})
	return res, err
}

var partitionDay = regexp.MustCompile(`^analytics_(?:events|visits|visit_pages)_(\d{8})$`)

// DropPartitionsBefore drops the raw partitions of days before day. It is server maintenance over
// every workspace's rows of those days (an exception to the workspace rule, see architecture.md).
func (s *PGStore) DropPartitionsBefore(ctx context.Context, day time.Time) ([]string, error) {
	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", int64(PartitionLock)); err != nil {
		return nil, err
	}
	defer conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", int64(PartitionLock))
	rows, err := conn.Query(ctx, `SELECT c.relname, p.relname, i.inhdetachpending FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid JOIN pg_class p ON p.oid = i.inhparent
		WHERE p.relname IN ('analytics_events', 'analytics_visits', 'analytics_visit_pages') ORDER BY c.relname`)
	if err != nil {
		return nil, err
	}
	type part struct {
		name, parent string
		pending      bool
	}
	parts, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (part, error) {
		var p part
		err := r.Scan(&p.name, &p.parent, &p.pending)
		return p, err
	})
	if err != nil {
		return nil, err
	}
	var dropped []string
	for _, p := range parts {
		m := partitionDay.FindStringSubmatch(p.name)
		if m == nil {
			continue
		}
		d, err := time.Parse("20060102", m[1])
		if err != nil || !d.Before(day) {
			continue
		}
		detach := "CONCURRENTLY"
		if p.pending {
			detach = "FINALIZE"
		}
		if _, err := conn.Exec(ctx, fmt.Sprintf("ALTER TABLE %s DETACH PARTITION %s %s", pgx.Identifier{p.parent}.Sanitize(), pgx.Identifier{p.name}.Sanitize(), detach)); err != nil {
			return dropped, fmt.Errorf("detach %s: %w", p.name, err)
		}
		if _, err := conn.Exec(ctx, "DROP TABLE "+pgx.Identifier{p.name}.Sanitize()); err != nil {
			return dropped, fmt.Errorf("drop %s: %w", p.name, err)
		}
		dropped = append(dropped, p.name)
	}
	return dropped, nil
}

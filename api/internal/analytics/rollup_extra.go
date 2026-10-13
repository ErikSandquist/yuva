package analytics

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
)

// propRow is a row of the goal or prop rollups of a day: for goals, name is the dimension code.
type propRow struct {
	name, key string
	extraRow
}

func (r propRow) mergeKey() string {
	cur := "\x00"
	if r.currency != nil {
		cur = *r.currency
	}
	return r.name + "\x1f" + r.key + "\x1f" + r.value + "\x1f" + cur
}

// mergeRows adds up rows that ended up with the same name, key, value and currency.
func mergeRows(rows []propRow) []propRow {
	byKey := map[string]int{}
	var out []propRow
	for _, r := range rows {
		k := r.mergeKey()
		if i, ok := byKey[k]; ok {
			o := &out[i]
			o.visitors, o.visits, o.n = o.visitors+r.visitors, o.visits+r.visits, o.n+r.n
			o.revenue, o.revenueCount = o.revenue+r.revenue, o.revenueCount+r.revenueCount
			continue
		}
		byKey[k] = len(out)
		out = append(out, r)
	}
	return out
}

// topKeys are the top keys of a ranking by visitors, or all of them when they are few.
func topKeys(rank map[string]int64, top int) map[string]bool {
	keys := slices.Collect(maps.Keys(rank))
	if len(keys) > top {
		slices.SortFunc(keys, func(a, b string) int { return cmp.Or(cmp.Compare(rank[b], rank[a]), cmp.Compare(a, b)) })
		keys = keys[:top]
	}
	out := map[string]bool{}
	for _, k := range keys {
		out[k] = true
	}
	return out
}

// foldValues keeps each name and key's top values by visitors and adds the rest up as (other).
func foldValues(rows []propRow, top int) []propRow {
	rank := map[string]map[string]int64{}
	for _, r := range rows {
		if r.currency != nil {
			continue
		}
		part := r.name + "\x1f" + r.key
		if rank[part] == nil {
			rank[part] = map[string]int64{}
		}
		rank[part][r.value] = r.visitors
	}
	keep := map[string]map[string]bool{}
	for part, values := range rank {
		keep[part] = topKeys(values, top)
	}
	for i := range rows {
		if k := keep[rows[i].name+"\x1f"+rows[i].key]; k != nil && !k[rows[i].value] {
			rows[i].value = OtherValue
		}
	}
	return mergeRows(rows)
}

// foldNames keeps the top event names by visitors; the rest add up to the event (other), whose
// props are not kept.
func foldNames(rows []propRow, top int) []propRow {
	rank := map[string]int64{}
	for _, r := range rows {
		if r.name != "" && r.key == "" && r.currency == nil {
			rank[r.name] = r.visitors
		}
	}
	keep := topKeys(rank, top)
	out := rows[:0]
	for _, r := range rows {
		if r.name != "" && !keep[r.name] {
			if r.key != "" {
				continue
			}
			r.name = OtherValue
		}
		out = append(out, r)
	}
	return mergeRows(out)
}

func loadGoals(ctx context.Context, tx pgx.Tx, ws, property uuid.UUID) ([]Goal, error) {
	rows, err := tx.Query(ctx, `SELECT id, kind, coalesce(path_pattern, ''), coalesce(event_name, ''), coalesce(prop_key, ''), coalesce(prop_value, ''),
		coalesce(scroll_pct, 0), version FROM analytics_goals WHERE workspace_id = $1 AND property_id = $2 ORDER BY id`, ws, property)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Goal, error) {
		var g Goal
		err := r.Scan(&g.ID, &g.Kind, &g.Path, &g.Event, &g.PropKey, &g.PropValue, &g.Scroll, &g.Version)
		return g, err
	})
}

func currencyOf(r propRow) string {
	if r.currency == nil {
		return ""
	}
	return *r.currency
}

// buildExtras replaces a day's goal and prop rollups with fresh ones from its raw rows and records
// the goals version they were built with. The version is read before the goals, so rows built
// from newer goals than the version says are only ever rebuilt again, never trusted too early.
func buildExtras(ctx context.Context, tx pgx.Tx, ws, property uuid.UUID, day time.Time, top int) error {
	var version int64
	var tracked []string
	if err := tx.QueryRow(ctx, `SELECT goals_version, tracked_props FROM analytics_properties WHERE workspace_id = $1 AND id = $2`,
		ws, property).Scan(&version, &tracked); err != nil {
		return err
	}
	goals, err := loadGoals(ctx, tx, ws, property)
	if err != nil {
		return err
	}
	for _, t := range []string{"analytics_daily_goals", "analytics_daily_props"} {
		if _, err := tx.Exec(ctx, "DELETE FROM "+t+" WHERE workspace_id = $1 AND property_id = $2 AND day = $3", ws, property, day); err != nil {
			return err
		}
	}
	var goalRows [][]any
	if len(goals) > 0 {
		groups := make([]group, len(goalRollupDims))
		for i, code := range goalRollupDims {
			groups[i] = group{code, dimByCode(code).Group}
		}
		sc := newScope(ws, property, day, day)
		conds := make([]string, len(goals))
		for i, g := range goals {
			conds[i] = g.cond(sc, "e")
		}
		r, err := tx.Query(ctx, sc.goalSetsSQL(conds, groups, true, false), sc.args...)
		if err != nil {
			return fmt.Errorf("goal rollup: %w", err)
		}
		idx, got, err := scanGoals(r)
		if err != nil {
			return fmt.Errorf("goal rollup: %w", err)
		}
		byGoal := make([][]propRow, len(goals))
		for i, x := range got {
			if x.currency != nil && *x.currency == "" {
				continue
			}
			byGoal[idx[i]] = append(byGoal[idx[i]], propRow{name: strconv.Itoa(int(x.group)), extraRow: x})
		}
		for i, rows := range byGoal {
			for _, r := range foldValues(rows, top) {
				goalRows = append(goalRows, []any{ws, property, day, goals[i].ID, r.group, r.value, currencyOf(r), r.visitors, r.visits, r.n, r.revenue, r.revenueCount})
			}
		}
	}
	if len(goalRows) > 0 {
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"analytics_daily_goals"}, []string{"workspace_id", "property_id", "day", "goal_id", "dim", "value",
			"currency", "visitors", "visits", "completions", "revenue_minor", "revenue_events"}, pgx.CopyFromRows(goalRows)); err != nil {
			return fmt.Errorf("goal rollup: %w", err)
		}
	}
	sc := newScope(ws, property, day, day)
	keys := sc.arg(nonNilStrings(tracked))
	r, err := tx.Query(ctx, "WITH "+sc.visitsCTE()+`
SELECT CASE WHEN GROUPING(e.name) = 0 THEN e.name ELSE '' END, p.key, p.val, CASE WHEN GROUPING(e.currency) = 0 THEN coalesce(e.currency, '') END,
    count(DISTINCT e.visitor), count(DISTINCT e.visit), count(*), coalesce(sum(e.revenue_minor), 0)::bigint, count(e.revenue_minor)
FROM analytics_events e JOIN vv v ON v.day = e.day AND v.visit = e.visit
CROSS JOIN LATERAL (SELECT '' AS key, '' AS val UNION ALL SELECT k, coalesce(e.props->>k, '') FROM unnest(`+keys+`::text[]) AS k) AS p
WHERE e.workspace_id = $1 AND e.property_id = $2 AND e.day BETWEEN $3 AND $4 AND e.kind = 3
GROUP BY GROUPING SETS ((p.key, p.val), (p.key, p.val, e.currency), (e.name, p.key, p.val), (e.name, p.key, p.val, e.currency))`, sc.args...)
	if err != nil {
		return fmt.Errorf("prop rollup: %w", err)
	}
	var props []propRow
	for r.Next() {
		var x propRow
		if err := r.Scan(&x.name, &x.key, &x.value, &x.currency, &x.visitors, &x.visits, &x.n, &x.revenue, &x.revenueCount); err != nil {
			r.Close()
			return fmt.Errorf("prop rollup: %w", err)
		}
		if x.currency != nil && *x.currency == "" {
			continue
		}
		props = append(props, x)
	}
	r.Close()
	if err := r.Err(); err != nil {
		return fmt.Errorf("prop rollup: %w", err)
	}
	if len(props) > 0 {
		props = foldValues(foldNames(props, top), top)
		rows := make([][]any, len(props))
		for i, x := range props {
			rows[i] = []any{ws, property, day, x.name, x.key, x.value, currencyOf(x), x.visitors, x.visits, x.n, x.revenue, x.revenueCount}
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"analytics_daily_props"}, []string{"workspace_id", "property_id", "day", "name", "prop", "value",
			"currency", "visitors", "visits", "events", "revenue_minor", "revenue_events"}, pgx.CopyFromRows(rows)); err != nil {
			return fmt.Errorf("prop rollup: %w", err)
		}
	}
	_, err = tx.Exec(ctx, `UPDATE analytics_rollup_days SET goals_version = $4 WHERE workspace_id = $1 AND property_id = $2 AND day = $3`,
		ws, property, day, version)
	return err
}

func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// RollupGoalsDay rebuilds the goal and prop rollups of a rolled-up day after the property's goals
// or tracked props changed; a day that is not rolled up, or dirty, gets a whole rollup instead.
func (s *PGStore) RollupGoalsDay(ctx context.Context, ws, property uuid.UUID, day time.Time, top int) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var built *time.Time
		var dirty bool
		err := tx.QueryRow(ctx, `SELECT built_at, dirty FROM analytics_rollup_days WHERE workspace_id = $1 AND property_id = $2 AND day = $3 FOR UPDATE`,
			ws, property, day).Scan(&built, &dirty)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (built == nil || dirty)) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := aggregationLimits(ctx, tx); err != nil {
			return err
		}
		if err := buildExtras(ctx, tx, ws, property, day, top); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE analytics_properties SET rollup_version = rollup_version + 1 WHERE workspace_id = $1 AND id = $2`, ws, property)
		return err
	})
}

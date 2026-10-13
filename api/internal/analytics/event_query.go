package analytics

import (
	"context"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
)

// EventMetrics are what an event or prop breakdown counts, over the events of each row.
var EventMetrics = []string{"visitors", "visits", "events", "revenue"}

// eventSetsSQL: per day, the visitors, visits and events among the custom events that pass
// eventWhere, and per currency their revenue, in total (group 0) and per value of value (group 1).
func (s *scope) eventSetsSQL(eventWhere []string, value string) string {
	sets := []string{"(e.day)", "(e.day, e.currency)"}
	code, val := "0", "''"
	if value != "" {
		v := "(" + value + ")"
		sets = append(sets, "(e.day, "+v+")", "(e.day, "+v+", e.currency)")
		code = "CASE WHEN GROUPING(" + v + ") = 0 THEN 1 ELSE 0 END"
		val = "CASE WHEN GROUPING(" + v + ") = 0 THEN " + v + " ELSE '' END"
	}
	where := append([]string{"e.workspace_id = $1", "e.property_id = $2", "e.day BETWEEN $3 AND $4", "e.kind = 3"}, eventWhere...)
	return "WITH " + s.visitsCTE() + `
SELECT e.day, (` + code + `)::smallint, ` + val + `, CASE WHEN GROUPING(e.currency) = 0 THEN coalesce(e.currency, '') END,
    count(DISTINCT e.visitor), count(DISTINCT e.visit), count(*), coalesce(sum(e.revenue_minor), 0)::bigint, count(e.revenue_minor)
FROM analytics_events e JOIN vv v ON v.day = e.day AND v.visit = e.visit
WHERE ` + strings.Join(where, " AND ") + `
GROUP BY GROUPING SETS (` + strings.Join(sets, ", ") + `)`
}

// eventQuery answers a breakdown of custom events by name or by a prop's value, with at most one
// other dimension: a time grain or a visit dimension. Event and prop filters narrow the events,
// the others the visits.
func eventQuery(ctx context.Context, tx pgx.Tx, q Query) (Result, error) {
	if err := CheckFilters(q.Filters); err != nil {
		return Result{}, err
	}
	for _, m := range q.Metrics {
		if !slices.Contains(EventMetrics, m) {
			return Result{}, badQuery("an event or prop breakdown counts visitors, visits, events and revenue, not %s", m)
		}
	}
	grain, dims, err := splitDims(q)
	if err != nil {
		return Result{}, err
	}
	var ev *Dim
	for i, d := range dims {
		switch {
		case d.Event && ev != nil:
			return Result{}, badQuery("break down by event or by prop, not both")
		case d.Event:
			ev = &dims[i]
		case d.Page:
			return Result{}, badQuery("an event or prop breakdown cannot be combined with pages")
		}
	}
	if ev.Code == DimProp && q.Prop == "" {
		return Result{}, badQuery("a prop breakdown needs prop, the key to break down by")
	}
	var evFilters, visitFilters []Filter
	for _, f := range q.Filters {
		if d, _ := DimByName(f.Dimension); d.Event {
			evFilters = append(evFilters, f)
		} else {
			visitFilters = append(visitFilters, f)
		}
	}
	name, byName := "", false
	if len(evFilters) == 1 && evFilters[0].Dimension == "event" && evFilters[0].Op == "is" && len(evFilters[0].Values) == 1 {
		name, byName = evFilters[0].Values[0], true
	}
	shaped := len(dims) == 1 && len(visitFilters) == 0 && (len(evFilters) == 0 || (ev.Code == DimProp && byName))
	p := &extraPlan{plan: plan{q: q, grain: grain, dims: dims}}
	built, err := rolledDays(ctx, tx, q)
	if err != nil {
		return Result{}, err
	}
	if !shaped && q.From.Before(q.RawFrom) {
		return Result{}, &NeedsRawError{From: q.RawFrom}
	}
	for _, d := range days(q.From, q.To) {
		_, ok := built[d]
		switch {
		case ev.Code == DimEvent && shaped && ok, ev.Code == DimProp && d.Before(q.RawFrom) && ok:
			p.rolled = append(p.rolled, d)
		case !d.Before(q.RawFrom):
			p.raw = append(p.raw, d)
		}
	}
	var rows, totals acc
	if len(p.rolled) > 0 {
		sql := `SELECT day, CASE WHEN name = '' THEN 0 ELSE 1 END::smallint, name, CASE WHEN currency <> '' THEN currency END,
			visitors, visits, events, revenue_minor, revenue_events FROM analytics_daily_props
			WHERE workspace_id = $1 AND property_id = $2 AND day = ANY($3::date[]) AND prop = ''`
		args := []any{q.WorkspaceID, q.PropertyID, p.rolled}
		if ev.Code == DimProp {
			sql = `SELECT day, CASE WHEN prop = '' THEN 0 ELSE 1 END::smallint, value, CASE WHEN currency <> '' THEN currency END,
				visitors, visits, events, revenue_minor, revenue_events FROM analytics_daily_props
				WHERE workspace_id = $1 AND property_id = $2 AND day = ANY($3::date[]) AND name = $4 AND prop IN ('', $5)`
			args = append(args, name, q.Prop)
		}
		r, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return Result{}, err
		}
		got, err := scanExtra(r)
		if err != nil {
			return Result{}, err
		}
		p.add(got, false, &rows, &totals)
	}
	if len(p.raw) > 0 {
		sc := newScope(q.WorkspaceID, q.PropertyID, p.raw[0], p.raw[len(p.raw)-1])
		for _, f := range visitFilters {
			if err := sc.addFilter(f); err != nil {
				return Result{}, badQuery("%s", err.Error())
			}
		}
		if len(p.raw) < len(days(p.raw[0], p.raw[len(p.raw)-1])) {
			sc.onlyDays(p.raw)
		}
		var where []string
		for _, f := range evFilters {
			d, _ := DimByName(f.Dimension)
			where = append(where, sc.cond(sc.eventExpr(d, "e", f.Prop), f, false))
		}
		value := valueGroup(dims, func(d Dim) string {
			if d.Event {
				return sc.eventExpr(d, "e", q.Prop)
			}
			return d.Group
		})
		r, err := tx.Query(ctx, sc.eventSetsSQL(where, value), sc.args...)
		if err != nil {
			return Result{}, err
		}
		got, err := scanExtra(r)
		if err != nil {
			return Result{}, err
		}
		p.add(got, false, &rows, &totals)
	}
	return p.finish(Result{ServedFrom: served(len(p.rolled) > 0, len(p.raw) > 0)}, &rows, &totals), nil
}

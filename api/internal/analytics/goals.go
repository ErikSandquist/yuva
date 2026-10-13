package analytics

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
)

const (
	GoalPageview = "pageview"
	GoalEvent    = "event"
	GoalScroll   = "scroll"
)

// Goal is what completes a goal: a page view whose path matches Path, a custom event named Event
// (whose prop PropKey equals PropValue, when set), or a page matching Path scrolled to Scroll
// percent. Version is the property's goals version when its definition last changed.
type Goal struct {
	ID        uuid.UUID
	Kind      string
	Path      string
	Event     string
	PropKey   string
	PropValue string
	Scroll    int16
	Version   int64
}

// cond is the condition an event of the alias meets when it completes the goal.
func (g Goal) cond(s *scope, alias string) string {
	path := func() string {
		return s.cond(alias+".path", Filter{Op: "matches", Values: []string{g.Path}}, true)
	}
	switch g.Kind {
	case GoalPageview:
		return alias + ".kind = 1 AND " + path()
	case GoalScroll:
		return alias + ".kind = 4 AND " + alias + ".scroll >= " + s.arg(g.Scroll) + " AND " + path()
	}
	c := alias + ".kind = 3 AND " + alias + ".name = " + s.arg(g.Event)
	if g.PropKey != "" {
		c += " AND " + alias + ".props->>" + s.arg(g.PropKey) + " = " + s.arg(g.PropValue)
	}
	return c
}

// goalRollupDims are the breakdowns a day's goal rollups keep next to the total.
var goalRollupDims = []int16{1, 4, 5, 9, 12, 15}

type mode int

const (
	modeTraffic mode = iota
	modeGoal
	modeEvents
)

func queryMode(q Query) (mode, error) {
	goalMetric := false
	for _, m := range q.Metrics {
		if slices.Contains(GoalMetrics, m) && m != "revenue" {
			goalMetric = true
		}
	}
	events := slices.ContainsFunc(q.Dimensions, func(d string) bool { return d == "event" || d == "prop" })
	switch {
	case q.Goal != nil && events:
		return 0, badQuery("a goal's numbers cannot be broken down by event or prop")
	case q.Goal != nil:
		return modeGoal, nil
	case events:
		return modeEvents, nil
	case goalMetric || slices.Contains(q.Metrics, "revenue"):
		return 0, badQuery("conversions, conversion_rate, completions and revenue need a goal_id; revenue also comes with an event or prop dimension")
	}
	return modeTraffic, nil
}

// extraRow is a row of goal or event numbers: Currency nil holds the counts, a currency its
// revenue. Group tells the sets of a statement apart (a dimension code, or 0 for totals).
type extraRow struct {
	day                   time.Time
	group                 int16
	value                 string
	currency              *string
	visitors, visits, n   int64
	revenue, revenueCount int64
}

func (r extraRow) sums(goal bool) Sums {
	var s Sums
	switch {
	case r.currency == nil && goal:
		s.Conversions, s.Completions = r.visitors, r.n
	case r.currency == nil:
		s.Visitors, s.Visits, s.Events = r.visitors, r.visits, r.n
	case *r.currency != "":
		s.addRevenue(*r.currency, Money{r.revenue, r.revenueCount})
	}
	return s
}

// scanGoals reads goalSetsSQL's rows, each with the index of its goal.
func scanGoals(rows pgx.Rows) ([]int32, []extraRow, error) {
	defer rows.Close()
	var goals []int32
	var out []extraRow
	for rows.Next() {
		var g int32
		var r extraRow
		if err := rows.Scan(&r.day, &g, &r.group, &r.value, &r.currency, &r.visitors, &r.visits, &r.n, &r.revenue, &r.revenueCount); err != nil {
			return nil, nil, err
		}
		r.day = r.day.UTC()
		goals, out = append(goals, g), append(out, r)
	}
	return goals, out, rows.Err()
}

func scanExtra(rows pgx.Rows) ([]extraRow, error) {
	defer rows.Close()
	var out []extraRow
	for rows.Next() {
		var r extraRow
		if err := rows.Scan(&r.day, &r.group, &r.value, &r.currency, &r.visitors, &r.visits, &r.n, &r.revenue, &r.revenueCount); err != nil {
			return nil, err
		}
		r.day = r.day.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// group is a breakdown of a statement: its code and its value over the visit alias v (and the
// visit page alias p).
type group struct {
	code int16
	expr string
}

// goalSetsSQL: per day and goal (the index of its condition), the visitors and visits that
// completed it, the completions, and per currency their revenue, per value of each group and, with
// totals, in total (group 0). The day's events are read once for all the goals.
func (s *scope) goalSetsSQL(conds []string, groups []group, totals, page bool) string {
	var sets []string
	if totals {
		sets = []string{"(v.day, g.goal)", "(v.day, g.goal, g.cur)"}
	}
	hits := make([]string, len(conds))
	for i, c := range conds {
		hits[i] = "(" + strconv.Itoa(i) + ", " + c + ")"
	}
	var codes, values []string
	for _, gr := range groups {
		e := "(" + gr.expr + ")"
		sets = append(sets, "(v.day, g.goal, "+e+")", "(v.day, g.goal, "+e+", g.cur)")
		codes = append(codes, "WHEN GROUPING("+e+") = 0 THEN "+strconv.Itoa(int(gr.code)))
		values = append(values, "WHEN GROUPING("+e+") = 0 THEN "+e)
	}
	join := ""
	if page {
		join = " JOIN pv p ON p.day = v.day AND p.visit = v.visit AND p.views > 0"
	}
	ctes := "WITH " + s.visitsCTE()
	if page {
		ctes += ", " + s.pagesCTE()
	}
	return ctes + `, g AS (
SELECT e.day, e.visit, gl.goal, e.currency AS cur, count(*) AS n, coalesce(sum(e.revenue_minor), 0) AS rev, count(e.revenue_minor) AS rn
FROM analytics_events e CROSS JOIN LATERAL (VALUES ` + strings.Join(hits, ", ") + `) AS gl(goal, hit)
WHERE e.workspace_id = $1 AND e.property_id = $2 AND e.day BETWEEN $3 AND $4 AND (` + strings.Join(conds, " OR ") + `) AND gl.hit
GROUP BY e.day, e.visit, gl.goal, e.currency)
SELECT v.day, g.goal, (CASE ` + strings.Join(append(codes, "WHEN true THEN 0"), " ") + ` END)::smallint,
    CASE ` + strings.Join(append(values, "WHEN true THEN ''"), " ") + ` END,
    CASE WHEN GROUPING(g.cur) = 0 THEN coalesce(g.cur, '') END,
    count(DISTINCT v.visitor), count(DISTINCT v.visit), sum(g.n)::bigint, sum(g.rev)::bigint, sum(g.rn)::bigint
FROM g JOIN vv v ON v.day = g.day AND v.visit = g.visit` + join + `
GROUP BY GROUPING SETS (` + strings.Join(sets, ", ") + `)`
}

// extraPlan answers the goal or event part of a query: from rollups on the days listed in rolled,
// from raw rows on those in raw.
type extraPlan struct {
	plan
	rolled, raw []time.Time
}

func splitDims(q Query) (string, []Dim, error) {
	if len(q.Dimensions) > 2 {
		return "", nil, badQuery("at most two dimensions")
	}
	grain := ""
	var dims []Dim
	for _, name := range q.Dimensions {
		if g, ok := strings.CutPrefix(name, "time:"); ok {
			if grain != "" || !slices.Contains([]string{"day", "week", "month"}, g) {
				return "", nil, badQuery("%q: goals and events take one time grain of time:day, time:week or time:month", name)
			}
			grain = g
			continue
		}
		d, ok := DimByName(name)
		if !ok {
			return "", nil, badQuery("unknown dimension %q", name)
		}
		if slices.ContainsFunc(dims, func(x Dim) bool { return x.Name == name }) {
			return "", nil, badQuery("dimension %q is listed twice", name)
		}
		dims = append(dims, d)
	}
	return grain, dims, nil
}

// rolledDays reads which days of the range are rolled up (a dirty day counts only before the raw
// window) and with which goals version.
func rolledDays(ctx context.Context, tx pgx.Tx, q Query) (map[time.Time]int64, error) {
	rows, err := tx.Query(ctx, `SELECT day, goals_version FROM analytics_rollup_days
		WHERE workspace_id = $1 AND property_id = $2 AND day BETWEEN $3 AND $4 AND built_at IS NOT NULL AND (NOT dirty OR day < $5)`,
		q.WorkspaceID, q.PropertyID, q.From, q.To, q.RawFrom)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[time.Time]int64{}
	for rows.Next() {
		var d time.Time
		var v int64
		if err := rows.Scan(&d, &v); err != nil {
			return nil, err
		}
		out[d.UTC()] = v
	}
	return out, rows.Err()
}

func served(rolled, raw bool) string {
	switch {
	case rolled && raw:
		return ServedMixed
	case rolled:
		return ServedRollups
	}
	return ServedRaw
}

func mergeServed(a, b string) string {
	if a == "" || a == b {
		return b
	}
	if b == "" {
		return a
	}
	return ServedMixed
}

// add puts the rows of a statement into the query's rows and totals: group 0 is the totals, which
// are rows too when the query breaks down by time alone.
func (p *extraPlan) add(got []extraRow, goal bool, rows, totals *acc) {
	for _, r := range got {
		s := r.sums(goal)
		if r.group == DimTotal {
			totals.add(nil, s)
			if len(p.dims) == 0 {
				rows.add(p.dimsOf(r.day, 0, nil), s)
			}
			continue
		}
		var values []string
		if len(p.dims) > 0 {
			values = strings.SplitN(r.value, valueSep, len(p.dims))
		}
		rows.add(p.dimsOf(r.day, 0, values), s)
	}
}

// valueGroup is the breakdown of dims as one value, the dimensions' values joined in query order.
func valueGroup(dims []Dim, page func(Dim) string) string {
	parts := make([]string, len(dims))
	for i, d := range dims {
		parts[i] = "(" + page(d) + ")"
	}
	return strings.Join(parts, " || E'\\x1f' || ")
}

func goalQuery(ctx context.Context, tx pgx.Tx, q Query) (Result, error) {
	if err := CheckFilters(q.Filters); err != nil {
		return Result{}, err
	}
	grain, dims, err := splitDims(q)
	if err != nil {
		return Result{}, err
	}
	for _, d := range dims {
		if d.Event {
			return Result{}, badQuery("a goal's numbers cannot be broken down by event or prop")
		}
	}
	var traffic []string
	rate := false
	for _, m := range q.Metrics {
		if !slices.Contains(Metrics, m) && !slices.Contains(GoalMetrics, m) {
			return Result{}, badQuery("unknown metric %q", m)
		}
		if slices.Contains(GoalMetrics, m) {
			rate = rate || m == "conversion_rate"
			continue
		}
		traffic = append(traffic, m)
	}
	if rate && !slices.Contains(traffic, "visitors") {
		traffic = append(traffic, "visitors")
	}
	p := &extraPlan{plan: plan{q: q, grain: grain, dims: dims}}
	var rows, totals acc
	var res Result
	if len(traffic) > 0 {
		tq := q
		tq.Goal, tq.Metrics = nil, traffic
		tq.SortBy = slices.DeleteFunc(slices.Clone(q.SortBy), func(m string) bool { return slices.Contains(GoalMetrics, m) })
		tr, err := trafficQuery(ctx, tx, tq)
		if err != nil {
			return res, err
		}
		res.ServedFrom = tr.ServedFrom
		for _, r := range tr.Rows {
			rows.add(r.Dims, r.Sums)
		}
		totals.add(nil, tr.Totals)
	}
	code, shaped := DimTotal, len(q.Filters) == 0
	if len(dims) > 1 || (len(dims) == 1 && !slices.Contains(goalRollupDims, dims[0].Code)) {
		shaped = false
	} else if len(dims) == 1 {
		code = dims[0].Code
	}
	built, err := rolledDays(ctx, tx, q)
	if err != nil {
		return res, err
	}
	if !shaped && q.From.Before(q.RawFrom) {
		return res, &NeedsRawError{From: q.RawFrom}
	}
	for _, d := range days(q.From, q.To) {
		v, ok := built[d]
		switch {
		case shaped && ok && (v >= q.Goal.Version || d.Before(q.RawFrom)):
			p.rolled = append(p.rolled, d)
		case !d.Before(q.RawFrom):
			p.raw = append(p.raw, d)
		}
	}
	if len(p.rolled) > 0 {
		r, err := tx.Query(ctx, `SELECT day, dim, value, CASE WHEN currency <> '' THEN currency END, visitors, visits, completions, revenue_minor, revenue_events
			FROM analytics_daily_goals WHERE workspace_id = $1 AND property_id = $2 AND goal_id = $3 AND day = ANY($4::date[]) AND dim = ANY($5::smallint[])`,
			q.WorkspaceID, q.PropertyID, q.Goal.ID, p.rolled, []int16{DimTotal, code})
		if err != nil {
			return res, err
		}
		got, err := scanExtra(r)
		if err != nil {
			return res, err
		}
		p.add(got, true, &rows, &totals)
	}
	if len(p.raw) > 0 {
		sc := newScope(q.WorkspaceID, q.PropertyID, p.raw[0], p.raw[len(p.raw)-1])
		for _, f := range q.Filters {
			if err := sc.addFilter(f); err != nil {
				return res, badQuery("%s", err.Error())
			}
		}
		if len(p.raw) < len(days(p.raw[0], p.raw[len(p.raw)-1])) {
			sc.onlyDays(p.raw)
		}
		page := slices.ContainsFunc(dims, func(d Dim) bool { return d.Page })
		cond := []string{q.Goal.cond(sc, "e")}
		statements := []string{sc.goalSetsSQL(cond, nil, true, false)}
		if len(dims) > 0 {
			statements = append(statements, sc.goalSetsSQL(cond, []group{{1, valueGroup(dims, func(d Dim) string {
				if d.Page {
					return "p.path"
				}
				return d.Group
			})}}, false, page))
		}
		for _, st := range statements {
			r, err := tx.Query(ctx, st, sc.args...)
			if err != nil {
				return res, err
			}
			_, got, err := scanGoals(r)
			if err != nil {
				return res, err
			}
			p.add(got, true, &rows, &totals)
		}
	}
	res.ServedFrom = mergeServed(res.ServedFrom, served(len(p.rolled) > 0, len(p.raw) > 0))
	return p.finish(res, &rows, &totals), nil
}

// finish fills a time series's empty buckets and adds the totals up.
func (p *extraPlan) finish(res Result, rows, totals *acc) Result {
	res.Totals = Sums{}
	for _, r := range totals.list {
		res.Totals.Add(r.Sums)
	}
	if len(p.dims) == 0 && p.grain != "" {
		p.fillTime(rows)
	}
	res.Rows = nil
	for _, r := range rows.list {
		res.Rows = append(res.Rows, *r)
	}
	if p.grain != "" && len(p.dims) == 0 {
		slices.SortFunc(res.Rows, func(a, b Row) int { return strings.Compare(a.Dims[0], b.Dims[0]) })
	}
	return res
}

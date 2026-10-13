package analytics

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
)

const (
	QueryTimeout = 15 * time.Second
	QueryWorkMem = "64MB"

	ServedRollups = "rollups"
	ServedRaw     = "raw"
	ServedMixed   = "mixed"

	valueSep = "\x1f"
)

// Query asks for metrics of a property's local days From to To, broken down by up to two
// dimensions (one of them may be time:hour, time:day, time:week or time:month) and narrowed by
// filters. RawFrom is the oldest day whose raw rows are kept.
type Query struct {
	WorkspaceID uuid.UUID
	PropertyID  uuid.UUID
	Timezone    string
	From, To    time.Time
	Metrics     []string
	Dimensions  []string
	Filters     []Filter
	RawFrom     time.Time
	// SortBy are further metrics the rows are ordered by.
	SortBy []string
	// Goal asks for its conversions next to the metrics; Prop is the key a prop dimension reads.
	Goal *Goal
	Prop string
	// Dataset is empty for traffic, or DatasetSupport.
	Dataset string
}

type Row struct {
	Dims []string
	Sums Sums
}

type Result struct {
	Rows       []Row
	Totals     Sums
	ServedFrom string
}

// NeedsRawError: the query can only be answered from raw rows, which are kept from From on.
type NeedsRawError struct {
	From time.Time
}

func (e *NeedsRawError) Error() string {
	return "this query needs raw rows, which are kept from " + e.From.Format(time.DateOnly)
}

var ErrBadQuery = errors.New("bad query")

func badQuery(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrBadQuery, fmt.Sprintf(format, args...))
}

// plan is a query taken apart: its time grain, its other dimensions and where its days come from.
type plan struct {
	q          Query
	grain      string
	dims       []Dim
	engagement bool
	built      []time.Time
	open       []time.Time
	folds      []fold
	rollups    bool
	pair       int16
	pairValue  string
	filterDim  *Dim
}

func (p *plan) parse() error {
	if p.q.To.Before(p.q.From) {
		return badQuery("the range ends before it starts")
	}
	for _, name := range p.q.Dimensions {
		if g, ok := strings.CutPrefix(name, "time:"); ok {
			if p.grain != "" || !slices.Contains([]string{"hour", "day", "week", "month"}, g) {
				return badQuery("%q: a query has at most one time grain, time:hour, time:day, time:week or time:month", name)
			}
			p.grain = g
			continue
		}
		d, ok := DimByName(name)
		if !ok {
			return badQuery("unknown dimension %q", name)
		}
		if d.Event {
			return badQuery("%q cannot be combined with these metrics", name)
		}
		if slices.ContainsFunc(p.dims, func(x Dim) bool { return x.Name == name }) {
			return badQuery("dimension %q is listed twice", name)
		}
		p.dims = append(p.dims, d)
	}
	if len(p.q.Dimensions) > 2 {
		return badQuery("at most two dimensions")
	}
	if p.grain == "hour" && len(p.dims) > 0 {
		return badQuery("time:hour cannot be combined with another dimension")
	}
	for _, m := range p.q.Metrics {
		if !slices.Contains(Metrics, m) {
			return badQuery("unknown metric %q", m)
		}
		if p.grain == "hour" && !slices.Contains([]string{"visitors", "visits", "pageviews", "events"}, m) {
			return badQuery("time:hour serves visitors, visits, pageviews and events")
		}
		p.engagement = p.engagement || engagementMetrics[m]
	}
	return CheckFilters(p.q.Filters)
}

func CheckFilters(fs []Filter) error {
	for _, f := range fs {
		if _, ok := DimByName(f.Dimension); !ok {
			return badQuery("unknown filter dimension %q", f.Dimension)
		}
		if !slices.Contains([]string{"is", "is_not", "contains", "not_contains", "matches"}, f.Op) || len(f.Values) == 0 {
			return badQuery("the filter on %q needs an op of is, is_not, contains, not_contains or matches and at least one value", f.Dimension)
		}
		if (f.Dimension == "prop") != (f.Prop != "") {
			return badQuery("a filter names a prop key exactly when its dimension is prop")
		}
	}
	return nil
}

// fromRollups decides whether built days can be read from the rollups: totals and any single
// dimension without filters; totals with one exact filter on a visit dimension; pages with one
// exact filter on channel, source or country.
func (p *plan) fromRollups() bool {
	fs := p.q.Filters
	single := len(fs) == 1 && fs[0].Op == "is" && len(fs[0].Values) == 1
	for _, f := range fs {
		if d, _ := DimByName(f.Dimension); d.Event {
			return false
		}
	}
	switch {
	case p.grain == "hour":
		return len(fs) == 0
	case len(p.dims) == 0 && len(fs) == 0:
		return true
	case len(p.dims) == 0 && single:
		d, _ := DimByName(fs[0].Dimension)
		if d.Page || d.Filter != "" {
			return false
		}
		p.filterDim = &d
		return true
	case len(p.dims) == 1 && len(fs) == 0:
		return true
	case len(p.dims) == 1 && p.dims[0].Page && single:
		code, ok := pagePairs[fs[0].Dimension]
		p.pair, p.pairValue = code, fs[0].Values[0]
		return ok
	}
	return false
}

// foldable: the rollup shapes a fold holds, which are all but the page pairs.
func (p *plan) foldable() bool {
	return p.pair == 0
}

func days(from, to time.Time) []time.Time {
	var out []time.Time
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		out = append(out, d)
	}
	return out
}

// Query answers q from the rollups of finished days and the raw rows of the others, under a
// statement timeout: traffic, a goal's conversions next to it, or custom events.
func (s *PGStore) Query(ctx context.Context, q Query) (Result, error) {
	mode, err := queryMode(q)
	if err != nil {
		return Result{}, err
	}
	var res Result
	err = pgx.BeginTxFunc(ctx, s.Pool, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead}, func(tx pgx.Tx) error {
		if err := queryLimits(ctx, tx); err != nil {
			return err
		}
		switch mode {
		case modeGoal:
			res, err = goalQuery(ctx, tx, q)
		case modeEvents:
			res, err = eventQuery(ctx, tx, q)
		default:
			res, err = trafficQuery(ctx, tx, q)
		}
		return err
	})
	return res, err
}

// trafficQuery answers a query of visits and page views.
func trafficQuery(ctx context.Context, tx pgx.Tx, q Query) (Result, error) {
	p := &plan{q: q}
	if err := p.parse(); err != nil {
		return Result{}, err
	}
	var res Result
	err := func() error {
		rows, err := tx.Query(ctx, `SELECT d.day, d.built_at IS NOT NULL AND (NOT d.dirty OR d.day < $5), f.until_at,
			coalesce(f.late_seq = d.late_seq AND (NOT $6 OR f.engagement_seq = d.engagement_seq), false)
			FROM analytics_rollup_days d LEFT JOIN analytics_partial_days f ON f.workspace_id = d.workspace_id AND f.property_id = d.property_id AND f.day = d.day
			WHERE d.workspace_id = $1 AND d.property_id = $2 AND d.day BETWEEN $3 AND $4`, q.WorkspaceID, q.PropertyID, q.From, q.To, q.RawFrom, p.engagement)
		if err != nil {
			return err
		}
		built := map[time.Time]bool{}
		folded := map[time.Time]time.Time{}
		for rows.Next() {
			var d time.Time
			var b, ok bool
			var until *time.Time
			if err := rows.Scan(&d, &b, &until, &ok); err != nil {
				rows.Close()
				return err
			}
			built[d.UTC()] = b
			if ok {
				folded[d.UTC()] = *until
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if p.fromRollups() {
			for _, d := range days(q.From, q.To) {
				until, ok := folded[d]
				switch {
				case built[d]:
					p.built = append(p.built, d)
				case d.Before(q.RawFrom):
				case ok && p.foldable():
					p.folds = append(p.folds, fold{day: d, until: until})
				default:
					p.open = append(p.open, d)
				}
			}
		} else {
			if q.From.Before(q.RawFrom) {
				return &NeedsRawError{From: q.RawFrom}
			}
			p.open = days(q.From, q.To)
		}
		res, err = p.run(ctx, tx)
		return err
	}()
	return res, err
}

// queryLimits sets a query transaction's timeout and memory. JIT compiling costs more than it saves
// on these short aggregations, sorts and hashes of a busy day fit the work memory instead of
// spilling to disk, and a raw scan hands its narrow rows to workers cheaply enough that the
// planner's default tuple cost keeps it from splitting a month of visits across them.
func queryLimits(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT set_config('statement_timeout', $1, true), set_config('jit', 'off', true), set_config('work_mem', $2, true),
		set_config('parallel_tuple_cost', '0.01', true)`, fmt.Sprint(QueryTimeout.Milliseconds()), QueryWorkMem)
	return err
}

type acc struct {
	keys map[string]*Row
	list []*Row
}

func (a *acc) add(dims []string, s Sums) {
	k := strings.Join(dims, valueSep)
	if a.keys == nil {
		a.keys = map[string]*Row{}
	}
	r, ok := a.keys[k]
	if !ok {
		r = &Row{Dims: dims}
		a.keys[k] = r
		a.list = append(a.list, r)
	}
	r.Sums.Add(s)
}

// bucket is the start of the time bucket a day or hour falls in, as the API shows it.
func (p *plan) bucket(day time.Time, hour int) string {
	switch p.grain {
	case "hour":
		return day.Format(time.DateOnly) + fmt.Sprintf("T%02d:00", hour)
	case "week":
		wd := (int(day.Weekday()) + 6) % 7
		return day.AddDate(0, 0, -wd).Format(time.DateOnly)
	case "month":
		return time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC).Format(time.DateOnly)
	}
	return day.Format(time.DateOnly)
}

func (p *plan) dimsOf(day time.Time, hour int, values []string) []string {
	var out []string
	vi := 0
	for _, name := range p.q.Dimensions {
		if strings.HasPrefix(name, "time:") {
			out = append(out, p.bucket(day, hour))
			continue
		}
		v := ""
		if vi < len(values) {
			v = values[vi]
		}
		out = append(out, v)
		vi++
	}
	return out
}

func (p *plan) run(ctx context.Context, tx pgx.Tx) (Result, error) {
	var rows, totals acc
	res := Result{}
	if len(p.built)+len(p.folds) > 0 {
		list := slices.Clone(p.built)
		for _, f := range p.folds {
			list = append(list, f.day)
		}
		daily, hourly := "analytics_daily", "analytics_hourly"
		if len(p.folds) > 0 {
			daily, hourly = rollupsAndFolds("daily", dailyColumns), rollupsAndFolds("hourly", hourlyColumns)
		}
		if err := p.readRollups(ctx, tx, list, daily, hourly, &rows, &totals); err != nil {
			return res, err
		}
	}
	for _, f := range p.folds {
		open, mixed, err := openVisitors(ctx, tx, p.q.WorkspaceID, p.q.PropertyID, f)
		if err != nil {
			return res, err
		}
		if len(open) == 0 {
			continue
		}
		if err := p.readRaw(ctx, tx, []time.Time{f.day}, []rawPart{
			{func(sc *scope) { sc.ofVisitors(open, nil, nil) }, 1},
			{func(sc *scope) { sc.ofVisitors(open, &f.until, mixed) }, -1},
		}, &rows, &totals); err != nil {
			return res, err
		}
	}
	if len(p.open) > 0 {
		if err := p.readRaw(ctx, tx, p.open, []rawPart{{nil, 1}}, &rows, &totals); err != nil {
			return res, err
		}
	}
	rolled, raw := len(p.built)+len(p.folds) > 0, len(p.open)+len(p.folds) > 0
	switch {
	case rolled && raw:
		res.ServedFrom = ServedMixed
	case rolled:
		res.ServedFrom = ServedRollups
	default:
		res.ServedFrom = ServedRaw
	}
	for _, r := range totals.list {
		res.Totals.Add(r.Sums)
	}
	if len(p.dims) == 0 && p.grain != "" {
		p.fillTime(&rows)
	}
	for _, r := range rows.list {
		res.Rows = append(res.Rows, *r)
	}
	if p.grain != "" && len(p.dims) == 0 {
		slices.SortFunc(res.Rows, func(a, b Row) int { return strings.Compare(a.Dims[0], b.Dims[0]) })
	}
	return res, nil
}

// fillTime adds the time buckets without data, so a series has every point of the range.
func (p *plan) fillTime(rows *acc) {
	for _, d := range days(p.q.From, p.q.To) {
		hours := []int{0}
		if p.grain == "hour" {
			hours = make([]int, 24)
			for i := range hours {
				hours[i] = i
			}
		}
		for _, h := range hours {
			rows.add(p.dimsOf(d, h, nil), Sums{})
		}
	}
}

const (
	dailyColumns  = "workspace_id, property_id, day, dim, value, visitors, visits, views, bounces, duration_s, events, engaged_ms, engaged_views, scroll_sum, scroll_views"
	hourlyColumns = "workspace_id, property_id, day, hour, visitors, visits, views, events"
)

// rollupsAndFolds reads the built days' rollups and the folded days' folds as one table; a day has
// rows in only one of them.
func rollupsAndFolds(kind, cols string) string {
	return "(SELECT " + cols + " FROM analytics_" + kind + " UNION ALL SELECT " + cols + " FROM analytics_partial_" + kind + ") AS t"
}

// readRollups reads the days' rows of the daily and hourly tables.
func (p *plan) readRollups(ctx context.Context, tx pgx.Tx, days []time.Time, daily, hourly string, rows, totals *acc) error {
	if p.grain == "hour" {
		r, err := tx.Query(ctx, `SELECT day, hour, visitors, visits, views, events FROM `+hourly+`
			WHERE workspace_id = $1 AND property_id = $2 AND day = ANY($3::date[])`, p.q.WorkspaceID, p.q.PropertyID, days)
		if err != nil {
			return err
		}
		defer r.Close()
		for r.Next() {
			var day time.Time
			var hour int16
			var s Sums
			if err := r.Scan(&day, &hour, &s.Visitors, &s.Visits, &s.Views, &s.Events); err != nil {
				return err
			}
			rows.add(p.dimsOf(day.UTC(), int(hour), nil), s)
		}
		if err := r.Err(); err != nil {
			return err
		}
		return p.rollupTotals(ctx, tx, days, daily, totals)
	}
	dim, value := DimTotal, ""
	switch {
	case p.filterDim != nil:
		dim, value = p.filterDim.Code, p.q.Filters[0].Values[0]
	case p.pair != 0:
		dim = p.pair
	case len(p.dims) == 1:
		dim = p.dims[0].Code
	}
	cond := ""
	args := []any{p.q.WorkspaceID, p.q.PropertyID, days, dim}
	switch {
	case p.filterDim != nil:
		cond, args = " AND value = $5", append(args, value)
	case p.pair != 0:
		cond, args = " AND split_part(value, E'\\t', 2) = $5", append(args, p.pairValue)
	}
	cols := "day, dim, value, visitors, visits, views, bounces, duration_s, events, engaged_ms, engaged_views, scroll_sum, scroll_views"
	if p.grain == "" {
		cols = "min(day), dim, value, " + p.sumOf("visitors", "visits", "views", "bounces", "duration_s", "events", "engaged_ms", "engaged_views", "scroll_sum", "scroll_views")
		cond += " GROUP BY dim, value"
	}
	r, err := tx.Query(ctx, `SELECT `+cols+`
		FROM `+daily+` WHERE workspace_id = $1 AND property_id = $2 AND day = ANY($3::date[]) AND dim = $4`+cond, args...)
	if err != nil {
		return err
	}
	got, err := scanDaily(r)
	if err != nil {
		return err
	}
	for _, d := range got {
		var values []string
		if len(p.dims) == 1 {
			v := d.value
			if p.pair != 0 {
				v, _, _ = strings.Cut(v, "\t")
			}
			values = []string{v}
		}
		rows.add(p.dimsOf(d.day.UTC(), 0, values), d.sums)
		if len(p.dims) == 0 {
			totals.add(nil, d.sums)
		}
	}
	if len(p.dims) > 0 {
		return p.rollupTotals(ctx, tx, days, daily, totals)
	}
	return nil
}

// rollupTotals adds the days' totals under the query's filter.
func (p *plan) rollupTotals(ctx context.Context, tx pgx.Tx, days []time.Time, daily string, totals *acc) error {
	args := []any{p.q.WorkspaceID, p.q.PropertyID, days, DimTotal}
	cond := ""
	if p.pair != 0 {
		f, _ := DimByName(p.q.Filters[0].Dimension)
		args[3] = f.Code
		cond, args = " AND value = $5", append(args, p.pairValue)
	}
	r, err := tx.Query(ctx, `SELECT min(day), dim, '', sum(visitors)::bigint, sum(visits)::bigint, sum(views)::bigint, sum(bounces)::bigint,
		sum(duration_s)::bigint, sum(events)::bigint, sum(engaged_ms)::bigint, sum(engaged_views)::bigint, sum(scroll_sum)::bigint, sum(scroll_views)::bigint
		FROM `+daily+` WHERE workspace_id = $1 AND property_id = $2 AND day = ANY($3::date[]) AND dim = $4`+cond+` GROUP BY dim`, args...)
	if err != nil {
		return err
	}
	got, err := scanDaily(r)
	if err != nil {
		return err
	}
	for _, d := range got {
		totals.add(nil, d.sums)
	}
	return nil
}

// collapse adds a query's per-day rows up per value when the query has no time grain, so the
// database returns one row per value; visitors stay the sum of each day's.
func (p *plan) collapse(sql string) string {
	if p.grain != "" {
		return sql
	}
	return `SELECT min(d), dim, value, ` + p.sumOf("visitors", "visits", "views", "bounces", "dur", "events", "eng_ms", "eng_views", "scroll_sum", "scroll_views") + `
FROM (` + sql + `) AS r(d, dim, value, visitors, visits, views, bounces, dur, events, eng_ms, eng_views, scroll_sum, scroll_views)
GROUP BY dim, value`
}

// need are the sums the query's metrics and order need.
func (p *plan) need() int {
	return needsOf(p.q.Metrics, p.q.SortBy)
}

// sumOf adds up the needed ones of the ten sum columns.
func (p *plan) sumOf(cols ...string) string {
	var exprs [10]string
	for i, c := range cols {
		exprs[i] = "sum(" + c + ")::bigint"
	}
	return sumsSQL(p.need(), exprs)
}

// rawPart is a share of the raw rows of some days: narrow, when set, narrows them further, and
// sign adds them (1) or takes them away (-1).
type rawPart struct {
	narrow func(*scope)
	sign   int64
}

// rawSQL is the statement adding up the parts, one build of each over the days; each part's
// arguments follow the previous part's, whose first four they share.
func (p *plan) rawSQL(list []time.Time, parts []rawPart, build func(*scope) string, keys, width int) (string, []any, error) {
	var args []any
	var out []string
	for _, part := range parts {
		sc := newScope(p.q.WorkspaceID, p.q.PropertyID, list[0], list[len(list)-1])
		if args != nil {
			sc.args = args
		}
		sc.engagement, sc.need = p.engagement, p.need()
		for _, f := range p.q.Filters {
			if err := sc.addFilter(f); err != nil {
				return "", nil, badQuery("%s", err.Error())
			}
		}
		if len(list) < len(days(list[0], list[len(list)-1])) {
			sc.onlyDays(list)
		}
		if part.narrow != nil {
			part.narrow(sc)
		}
		q := build(sc)
		if part.sign < 0 {
			cols, neg := make([]string, width), make([]string, width)
			for i := range cols {
				cols[i] = fmt.Sprintf("c%d", i)
				neg[i] = cols[i]
				if i >= keys {
					neg[i] = "-" + cols[i]
				}
			}
			q = "SELECT " + strings.Join(neg, ", ") + " FROM (" + q + ") AS n(" + strings.Join(cols, ", ") + ")"
		}
		out = append(out, q)
		args = sc.args
	}
	return strings.Join(out, "\nUNION ALL\n"), args, nil
}

// readRaw adds up the parts of the days' raw numbers.
func (p *plan) readRaw(ctx context.Context, tx pgx.Tx, list []time.Time, parts []rawPart, rows, totals *acc) error {
	if p.grain == "hour" {
		sql, args, err := p.rawSQL(list, parts, func(sc *scope) string { return sc.hourlySQL(p.q.Timezone) }, 2, 6)
		if err != nil {
			return err
		}
		r, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer r.Close()
		for r.Next() {
			var day time.Time
			var hour int
			var s Sums
			if err := r.Scan(&day, &hour, &s.Visitors, &s.Visits, &s.Views, &s.Events); err != nil {
				return err
			}
			rows.add(p.dimsOf(day.UTC(), hour, nil), s)
		}
		if err := r.Err(); err != nil {
			return err
		}
		r.Close()
		return p.rawTotals(ctx, tx, list, parts, totals)
	}
	page := slices.IndexFunc(p.dims, func(d Dim) bool { return d.Page })
	build := func(sc *scope) string {
		switch {
		case len(p.dims) == 0:
			return p.collapse(sc.visitRowsSQL(nil))
		case len(p.dims) == 1 && page == 0:
			return p.collapse(sc.pageSetsSQL(false))
		case len(p.dims) == 1:
			return p.collapse(sc.visitSetsSQL(p.dims, true))
		case page >= 0:
			return p.collapse(sc.pagePairSQL(p.dims[1-page].Group))
		}
		return p.collapse(sc.visitRowsSQL([]string{p.dims[0].Group, p.dims[1].Group}))
	}
	sql, args, err := p.rawSQL(list, parts, build, 3, 13)
	if err != nil {
		return err
	}
	r, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	got, err := scanDaily(r)
	if err != nil {
		return err
	}
	for _, d := range got {
		day := d.day.UTC()
		if d.dim == DimTotal && (len(p.dims) == 0 || page < 0) {
			totals.add(nil, d.sums)
			if len(p.dims) > 0 {
				continue
			}
		}
		var values []string
		switch {
		case len(p.dims) == 1:
			values = []string{d.value}
		case len(p.dims) == 2:
			values = strings.SplitN(d.value, valueSep, 2)
			if page == 1 {
				values[0], values[1] = values[1], values[0]
			}
		}
		rows.add(p.dimsOf(day, 0, values), d.sums)
	}
	if page >= 0 {
		return p.rawTotals(ctx, tx, list, parts, totals)
	}
	return nil
}

func (p *plan) rawTotals(ctx context.Context, tx pgx.Tx, list []time.Time, parts []rawPart, totals *acc) error {
	sql, args, err := p.rawSQL(list, parts, func(sc *scope) string { return p.collapse(sc.visitRowsSQL(nil)) }, 3, 13)
	if err != nil {
		return err
	}
	r, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	got, err := scanDaily(r)
	if err != nil {
		return err
	}
	for _, d := range got {
		totals.add(nil, d.sums)
	}
	return nil
}

// SortRows orders rows by the metrics asked for (each ascending or descending), then by their
// dimension values.
func SortRows(rows []Row, order []Order) {
	slices.SortStableFunc(rows, func(a, b Row) int {
		for _, o := range order {
			c := cmp.Compare(a.Sums.Metric(o.Metric), b.Sums.Metric(o.Metric))
			if o.Desc {
				c = -c
			}
			if c != 0 {
				return c
			}
		}
		return slices.Compare(a.Dims, b.Dims)
	})
}

type Order struct {
	Metric string
	Desc   bool
}

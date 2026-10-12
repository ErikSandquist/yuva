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
	for _, f := range p.q.Filters {
		if _, ok := DimByName(f.Dimension); !ok {
			return badQuery("unknown filter dimension %q", f.Dimension)
		}
		if !slices.Contains([]string{"is", "is_not", "contains", "not_contains", "matches"}, f.Op) || len(f.Values) == 0 {
			return badQuery("the filter on %q needs an op of is, is_not, contains, not_contains or matches and at least one value", f.Dimension)
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

func days(from, to time.Time) []time.Time {
	var out []time.Time
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		out = append(out, d)
	}
	return out
}

// Query answers q from the rollups of finished days and the raw rows of the others, under a
// statement timeout.
func (s *PGStore) Query(ctx context.Context, q Query) (Result, error) {
	p := &plan{q: q}
	if err := p.parse(); err != nil {
		return Result{}, err
	}
	var res Result
	err := pgx.BeginTxFunc(ctx, s.Pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = %d", QueryTimeout.Milliseconds())); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT day, built_at IS NOT NULL AND (NOT dirty OR day < $5) FROM analytics_rollup_days
			WHERE workspace_id = $1 AND property_id = $2 AND day BETWEEN $3 AND $4`, q.WorkspaceID, q.PropertyID, q.From, q.To, q.RawFrom)
		if err != nil {
			return err
		}
		built := map[time.Time]bool{}
		for rows.Next() {
			var d time.Time
			var b bool
			if err := rows.Scan(&d, &b); err != nil {
				rows.Close()
				return err
			}
			built[d.UTC()] = b
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if p.fromRollups() {
			for _, d := range days(q.From, q.To) {
				if built[d] {
					p.built = append(p.built, d)
				} else if !d.Before(q.RawFrom) {
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
	})
	return res, err
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

func (p *plan) scope(from, to time.Time) (*scope, error) {
	sc := newScope(p.q.WorkspaceID, p.q.PropertyID, from, to)
	sc.engagement, sc.need = p.engagement, p.need()
	for _, f := range p.q.Filters {
		if err := sc.addFilter(f); err != nil {
			return nil, badQuery("%s", err.Error())
		}
	}
	return sc, nil
}

func (p *plan) run(ctx context.Context, tx pgx.Tx) (Result, error) {
	var rows, totals acc
	res := Result{}
	if len(p.built) > 0 {
		if err := p.readRollups(ctx, tx, &rows, &totals); err != nil {
			return res, err
		}
	}
	if len(p.open) > 0 {
		if err := p.readRaw(ctx, tx, &rows, &totals); err != nil {
			return res, err
		}
	}
	switch {
	case len(p.built) > 0 && len(p.open) > 0:
		res.ServedFrom = ServedMixed
	case len(p.built) > 0:
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

func (p *plan) readRollups(ctx context.Context, tx pgx.Tx, rows, totals *acc) error {
	if p.grain == "hour" {
		r, err := tx.Query(ctx, `SELECT day, hour, visitors, visits, views, events FROM analytics_hourly
			WHERE workspace_id = $1 AND property_id = $2 AND day = ANY($3::date[])`, p.q.WorkspaceID, p.q.PropertyID, p.built)
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
		return p.rollupTotals(ctx, tx, totals)
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
	args := []any{p.q.WorkspaceID, p.q.PropertyID, p.built, dim}
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
		FROM analytics_daily WHERE workspace_id = $1 AND property_id = $2 AND day = ANY($3::date[]) AND dim = $4`+cond, args...)
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
		return p.rollupTotals(ctx, tx, totals)
	}
	return nil
}

// rollupTotals adds the built days' totals under the query's filter.
func (p *plan) rollupTotals(ctx context.Context, tx pgx.Tx, totals *acc) error {
	args := []any{p.q.WorkspaceID, p.q.PropertyID, p.built, DimTotal}
	cond := ""
	if p.pair != 0 {
		f, _ := DimByName(p.q.Filters[0].Dimension)
		args[3] = f.Code
		cond, args = " AND value = $5", append(args, p.pairValue)
	}
	r, err := tx.Query(ctx, `SELECT min(day), dim, '', sum(visitors)::bigint, sum(visits)::bigint, sum(views)::bigint, sum(bounces)::bigint,
		sum(duration_s)::bigint, sum(events)::bigint, sum(engaged_ms)::bigint, sum(engaged_views)::bigint, sum(scroll_sum)::bigint, sum(scroll_views)::bigint
		FROM analytics_daily WHERE workspace_id = $1 AND property_id = $2 AND day = ANY($3::date[]) AND dim = $4`+cond+` GROUP BY dim`, args...)
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

// rawScope reads the open days only.
func (p *plan) rawScope() (*scope, error) {
	sc, err := p.scope(p.open[0], p.open[len(p.open)-1])
	if err != nil {
		return nil, err
	}
	if len(p.open) < len(days(p.open[0], p.open[len(p.open)-1])) {
		sc.onlyDays(p.open)
	}
	return sc, nil
}

func (p *plan) readRaw(ctx context.Context, tx pgx.Tx, rows, totals *acc) error {
	sc, err := p.rawScope()
	if err != nil {
		return err
	}
	if p.grain == "hour" {
		r, err := tx.Query(ctx, sc.hourlySQL(p.q.Timezone), sc.args...)
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
		return p.rawTotals(ctx, tx, totals)
	}
	var sql string
	page := slices.IndexFunc(p.dims, func(d Dim) bool { return d.Page })
	switch {
	case len(p.dims) == 0:
		sql = sc.visitRowsSQL(nil)
	case len(p.dims) == 1 && page == 0:
		sql = sc.pageSetsSQL(false)
	case len(p.dims) == 1:
		sql = sc.visitSetsSQL(p.dims, true)
	case page >= 0:
		sql = sc.pagePairSQL(p.dims[1-page].Group)
	default:
		sql = sc.visitRowsSQL([]string{p.dims[0].Group, p.dims[1].Group})
	}
	r, err := tx.Query(ctx, p.collapse(sql), sc.args...)
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
		return p.rawTotals(ctx, tx, totals)
	}
	return nil
}

func (p *plan) rawTotals(ctx context.Context, tx pgx.Tx, totals *acc) error {
	sc, err := p.rawScope()
	if err != nil {
		return err
	}
	r, err := tx.Query(ctx, p.collapse(sc.visitRowsSQL(nil)), sc.args...)
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

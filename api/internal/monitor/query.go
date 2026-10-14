package monitor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/productdevbook/yuva/api/internal/analytics"
)

const (
	ServedRollups = analytics.ServedRollups
	ServedRaw     = analytics.ServedRaw
	ServedMixed   = analytics.ServedMixed

	MaxMinuteBuckets = 1440
	minuteLabel      = "2006-01-02T15:04"
)

// Dims are the dimensions an API query breaks down and filters by; Metrics its metrics.
var (
	Dims    = []string{"route", "method", "status", "status_class", "environment"}
	Metrics = []string{"requests", "requests_per_minute", "errors", "error_rate", "client_errors", "client_error_rate", "p50", "p95", "p99", "avg_duration"}
	Grains  = []string{"time:minute", "time:hour", "time:day"}
)

var ErrBadQuery = analytics.ErrBadQuery

// ErrMinuteRange: a time:minute query reaches past the minute rows or has too many buckets.
var ErrMinuteRange = errors.New("time:minute serves at most 1,440 minutes within the minute rows")

func badQuery(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrBadQuery, fmt.Sprintf(format, args...))
}

type Filter = analytics.Filter

// Query asks for an API property's requests of [Start, End), broken down by up to two dimensions
// (one may be a time grain) and narrowed by filters. MinuteFrom is the first minute minute rows are
// kept for; Days says the range is whole local days, which daily rows may answer.
type Query struct {
	WorkspaceID uuid.UUID
	PropertyID  uuid.UUID
	Zone        Zone
	Start, End  time.Time
	Days        bool
	Metrics     []string
	Dimensions  []string
	Filters     []Filter
	MinuteFrom  time.Time
	Now         time.Time
}

type Row struct {
	Dims []string
	Agg  Agg
	// Minutes is how many minutes of the row's range have passed, for requests_per_minute.
	Minutes float64
}

type Result struct {
	Rows       []Row
	Totals     Agg
	Minutes    float64
	ServedFrom string
	Caps       []string
}

// Metric is a metric of a's requests over minutes.
func (a *Agg) Metric(name string, minutes float64) float64 {
	pct := func(n int64) float64 {
		if a.Requests == 0 {
			return 0
		}
		return round(float64(n)*100/float64(a.Requests), 2)
	}
	switch name {
	case "requests":
		return float64(a.Requests)
	case "requests_per_minute":
		if minutes <= 0 {
			return 0
		}
		return round(float64(a.Requests)/minutes, 2)
	case "errors":
		return float64(a.Errors)
	case "error_rate":
		return pct(a.Errors)
	case "client_errors":
		return float64(a.ClientErrors)
	case "client_error_rate":
		return pct(a.ClientErrors)
	case "p50":
		return round(a.DurationQuantile(0.5)/1000, 6)
	case "p95":
		return round(a.DurationQuantile(0.95)/1000, 6)
	case "p99":
		return round(a.DurationQuantile(0.99)/1000, 6)
	case "avg_duration":
		if a.Requests == 0 {
			return 0
		}
		return round(float64(a.DurationSum)/float64(a.Requests)/1000, 6)
	}
	return 0
}

func round(v float64, digits int) float64 {
	p, err := strconv.ParseFloat(strconv.FormatFloat(v, 'f', digits, 64), 64)
	if err != nil {
		return v
	}
	return p
}

type plan struct {
	q        Query
	grain    string
	dims     []string
	days     []time.Time
	hours    []time.Time
	minutes  [][2]time.Time
	sketches bool
}

func (p *plan) parse() error {
	q := p.q
	if !q.End.After(q.Start) {
		return badQuery("the range ends before it starts")
	}
	if len(q.Dimensions) > 2 {
		return badQuery("at most two dimensions")
	}
	for _, d := range q.Dimensions {
		switch {
		case slices.Contains(Grains, d):
			if p.grain != "" {
				return badQuery("a query has at most one time grain")
			}
			p.grain = strings.TrimPrefix(d, "time:")
		case slices.Contains(Dims, d):
			if slices.Contains(p.dims, d) {
				return badQuery("dimension %q is listed twice", d)
			}
			p.dims = append(p.dims, d)
		default:
			return badQuery("the api dataset breaks down by %s and time:minute, time:hour or time:day, not %q", strings.Join(Dims, ", "), d)
		}
	}
	for _, m := range q.Metrics {
		if !slices.Contains(Metrics, m) {
			return badQuery("the api dataset offers %s, not %s", strings.Join(Metrics, ", "), m)
		}
		if m == "p50" || m == "p95" || m == "p99" {
			p.sketches = true
		}
	}
	for _, f := range q.Filters {
		if !slices.Contains(Dims, f.Dimension) {
			return badQuery("the api dataset filters on %s, not %q", strings.Join(Dims, ", "), f.Dimension)
		}
		if !slices.Contains([]string{"is", "is_not", "contains", "not_contains", "matches"}, f.Op) || len(f.Values) == 0 {
			return badQuery("the filter on %q needs an op of is, is_not, contains, not_contains or matches and at least one value", f.Dimension)
		}
	}
	if p.grain == "minute" && (q.Start.Before(q.MinuteFrom) || q.End.Sub(q.Start) > MaxMinuteBuckets*time.Minute) {
		return ErrMinuteRange
	}
	return nil
}

type hourState struct {
	hour, day       time.Time
	built, dayBuilt bool
}

// tiers decides where each part of the range is read from: closed days from daily rows, closed
// hours from hourly rows, everything else from minute rows. Hours without a row had no requests.
func (p *plan) tiers(ctx context.Context, tx pgx.Tx) error {
	q := p.q
	if p.grain == "minute" {
		p.minutes = [][2]time.Time{{q.Start, q.End}}
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT hour, day, built_at IS NOT NULL AND NOT dirty, day_built_at IS NOT NULL FROM monitor_rollup_hours
		WHERE workspace_id = $1 AND property_id = $2 AND hour > $3 AND hour < $4 ORDER BY hour`, q.WorkspaceID, q.PropertyID, q.Start.Add(-time.Hour), q.End)
	if err != nil {
		return err
	}
	var states []hourState
	for rows.Next() {
		var h hourState
		if err := rows.Scan(&h.hour, &h.day, &h.built, &h.dayBuilt); err != nil {
			rows.Close()
			return err
		}
		h.hour, h.day = h.hour.UTC(), h.day.UTC()
		states = append(states, h)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	byDay := map[time.Time]bool{}
	if q.Days && p.grain != "hour" {
		ready := map[time.Time]bool{}
		for _, h := range states {
			if _, ok := ready[h.day]; !ok {
				ready[h.day] = true
			}
			ready[h.day] = ready[h.day] && h.built && h.dayBuilt
		}
		for d, ok := range ready {
			if ok && !q.Zone.DayStart(d).Before(q.Start) && !q.Zone.DayStart(d.AddDate(0, 0, 1)).After(q.End) {
				byDay[d] = true
				p.days = append(p.days, d)
			}
		}
		slices.SortFunc(p.days, func(a, b time.Time) int { return a.Compare(b) })
	}
	for _, h := range states {
		if byDay[h.day] {
			continue
		}
		end := q.Zone.HourEnd(h.hour)
		if h.built && !h.hour.Before(q.Start) && !end.After(q.End) {
			p.hours = append(p.hours, h.hour)
			continue
		}
		from, to := maxTime(h.hour, q.Start), minTime(end, q.End)
		if n := len(p.minutes); n > 0 && p.minutes[n-1][1].Equal(from) {
			p.minutes[n-1][1] = to
		} else {
			p.minutes = append(p.minutes, [2]time.Time{from, to})
		}
	}
	return nil
}

// names maps a property's dictionary ids to their values.
type nameMaps struct {
	routes, envs map[int32]string
	hidden       []int32
}

func loadNames(ctx context.Context, tx pgx.Tx, ws, prop uuid.UUID) (nameMaps, error) {
	out := nameMaps{routes: map[int32]string{}, envs: map[int32]string{}}
	rows, err := tx.Query(ctx, `SELECT id, pattern FROM monitor_routes WHERE workspace_id = $1 AND property_id = $2`, ws, prop)
	if err != nil {
		return out, err
	}
	if err := collectNames(rows, out.routes); err != nil {
		return out, err
	}
	rows, err = tx.Query(ctx, `SELECT id FROM monitor_routes WHERE workspace_id = $1 AND property_id = $2 AND hidden ORDER BY id`, ws, prop)
	if err != nil {
		return out, err
	}
	if out.hidden, err = pgx.CollectRows(rows, pgx.RowTo[int32]); err != nil {
		return out, err
	}
	rows, err = tx.Query(ctx, `SELECT id, value FROM monitor_names WHERE workspace_id = $1 AND property_id = $2 AND kind = 'environment'`, ws, prop)
	if err != nil {
		return out, err
	}
	return out, collectNames(rows, out.envs)
}

func collectNames(rows pgx.Rows, into map[int32]string) error {
	defer rows.Close()
	for rows.Next() {
		var id int32
		var v string
		if err := rows.Scan(&id, &v); err != nil {
			return err
		}
		into[id] = v
	}
	return rows.Err()
}

// matches is a filter's test of one value, as the analytics filters compare: whole values, parts
// without regard to case, or patterns (on routes * is a segment and ** any number of them).
func matches(f Filter, v string) bool {
	for _, want := range f.Values {
		switch f.Op {
		case "is", "is_not":
			if v == want {
				return true
			}
		case "contains", "not_contains":
			if strings.Contains(strings.ToLower(v), strings.ToLower(want)) {
				return true
			}
		case "matches":
			if f.Dimension == "route" {
				if analytics.MatchPath(want, v) {
					return true
				}
			} else if wildcard(want, v) {
				return true
			}
		}
	}
	return false
}

func wildcard(pattern, v string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == v
	}
	if !strings.HasPrefix(v, parts[0]) {
		return false
	}
	v = v[len(parts[0]):]
	for _, part := range parts[1 : len(parts)-1] {
		i := strings.Index(v, part)
		if i < 0 {
			return false
		}
		v = v[i+len(part):]
	}
	return strings.HasSuffix(v, parts[len(parts)-1])
}

func negative(op string) bool { return op == "is_not" || op == "not_contains" }

// filterSQL turns the filters into conditions on ids and codes, numbering parameters from next.
func filterSQL(fs []Filter, names nameMaps, next int) (string, []any) {
	var conds []string
	var args []any
	for _, f := range fs {
		var col string
		set := []int32{}
		switch f.Dimension {
		case "route":
			col = "route"
			for id, v := range names.routes {
				if matches(f, v) {
					set = append(set, id)
				}
			}
		case "environment":
			col = "environment"
			for id, v := range names.envs {
				if matches(f, v) {
					set = append(set, id)
				}
			}
		case "method":
			col = "method"
			for code := range int16(len(methods)) {
				if matches(f, MethodName(code)) {
					set = append(set, int32(code))
				}
			}
		case "status":
			col = "status"
			for code := range int32(600) {
				if matches(f, strconv.Itoa(int(code))) {
					set = append(set, code)
				}
			}
		case "status_class":
			col = "(status / 100)"
			for class := range int16(6) {
				if matches(f, StatusClassName(class)) {
					set = append(set, int32(class))
				}
			}
		}
		slices.Sort(set)
		cond := fmt.Sprintf("%s = ANY($%d::int[])", col, next+len(args))
		if negative(f.Op) {
			cond = "NOT " + cond
		}
		conds, args = append(conds, cond), append(args, set)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(conds, " AND "), args
}

type tier struct {
	table, timeCol, where string
	args                  []any
}

// bucketSQL is a tier's expression for the query's grain: instants for minutes and hours, local
// dates for days, each in the zone that applied at the row's time. Parameters go to args.
func (p *plan) bucketSQL(t tier, args *[]any) string {
	z := p.q.Zone
	switch p.grain {
	case "minute":
		return t.timeCol
	case "hour":
		if t.timeCol == "minute" {
			return z.sql("minute", func(tz string) string {
				return "(minute - make_interval(mins => extract(minute FROM minute AT TIME ZONE " + tz + ")::int))"
			}, true, args)
		}
		return t.timeCol
	case "day":
		if t.timeCol == "day" {
			return "day::timestamp AT TIME ZONE 'UTC'"
		}
		return z.sql(t.timeCol, func(tz string) string {
			return "((" + t.timeCol + " AT TIME ZONE " + tz + ")::date)::timestamp AT TIME ZONE 'UTC'"
		}, false, args)
	}
	return ""
}

func dimSQL(d string) string {
	switch d {
	case "status_class":
		return "(status / 100)::int"
	case "method", "status":
		return d + "::int"
	}
	return d
}

type group struct {
	bucket time.Time
	vals   []int32
	agg    Agg
}

func (s *PGStore) Query(ctx context.Context, q Query) (Result, error) {
	p := &plan{q: q}
	if err := p.parse(); err != nil {
		return Result{}, err
	}
	var res Result
	err := pgx.BeginTxFunc(ctx, s.Pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = "+strconv.Itoa(int(analytics.QueryTimeout.Milliseconds()))); err != nil {
			return err
		}
		if err := p.tiers(ctx, tx); err != nil {
			return err
		}
		names, err := loadNames(ctx, tx, q.WorkspaceID, q.PropertyID)
		if err != nil {
			return err
		}
		var tiers []tier
		if len(p.days) > 0 {
			tiers = append(tiers, tier{table: "monitor_day", timeCol: "day", where: "day = ANY($3::date[])", args: []any{p.days}})
		}
		if len(p.hours) > 0 {
			tiers = append(tiers, tier{table: "monitor_hour", timeCol: "hour", where: "hour = ANY($3::timestamptz[])", args: []any{p.hours}})
		}
		if len(p.minutes) > 0 {
			var or []string
			var args []any
			for _, r := range p.minutes {
				or = append(or, fmt.Sprintf("(minute >= $%d AND minute < $%d)", 3+len(args), 4+len(args)))
				args = append(args, r[0], r[1])
			}
			tiers = append(tiers, tier{table: MinuteTable, timeCol: "minute", where: "(" + strings.Join(or, " OR ") + ")", args: args})
		}
		groups := map[string]*group{}
		var order []string
		for _, t := range tiers {
			args := append([]any{q.WorkspaceID, q.PropertyID}, t.args...)
			var cols []string
			if p.grain != "" {
				cols = append(cols, p.bucketSQL(t, &args))
			}
			for _, d := range p.dims {
				cols = append(cols, dimSQL(d))
			}
			fsql, fargs := filterSQL(q.Filters, names, len(args)+1)
			args = append(args, fargs...)
			if len(names.hidden) > 0 {
				args = append(args, names.hidden)
				fsql += fmt.Sprintf(" AND NOT route = ANY($%d::int[])", len(args))
			}
			sel := `sum(requests)::bigint, sum(errors)::bigint, sum(client_errors)::bigint, sum(sampled)::bigint, sum(duration_sum)::bigint,
				min(duration_min)::bigint, max(duration_max)::bigint, sum(request_bytes)::bigint, sum(response_bytes)::bigint`
			if p.sketches {
				sel += ", array_agg(durations)"
			}
			groupBy := " HAVING count(*) > 0"
			if len(cols) > 0 {
				pos := make([]string, len(cols))
				for i := range cols {
					pos[i] = strconv.Itoa(i + 1)
				}
				groupBy = " GROUP BY " + strings.Join(pos, ", ")
				sel = strings.Join(cols, ", ") + ", " + sel
			}
			rows, err := tx.Query(ctx, "SELECT "+sel+" FROM "+t.table+" WHERE workspace_id = $1 AND property_id = $2 AND "+t.where+fsql+groupBy, args...)
			if err != nil {
				return fmt.Errorf("monitor query: %w", err)
			}
			for rows.Next() {
				var g group
				g.vals = make([]int32, len(p.dims))
				var dest []any
				if p.grain != "" {
					dest = append(dest, &g.bucket)
				}
				for i := range g.vals {
					dest = append(dest, &g.vals[i])
				}
				a := &g.agg
				dest = append(dest, &a.Requests, &a.Errors, &a.ClientErrors, &a.Sampled, &a.DurationSum, &a.DurationMin, &a.DurationMax,
					&a.RequestBytes, &a.ResponseBytes)
				var sketches [][]byte
				if p.sketches {
					dest = append(dest, &sketches)
				}
				if err := rows.Scan(dest...); err != nil {
					rows.Close()
					return err
				}
				a.Durations, a.Sizes = NewSketch(DurationScale), NewSketch(SizeScale)
				for _, raw := range sketches {
					sk, err := DecodeSketch(raw)
					if err != nil {
						rows.Close()
						return err
					}
					a.Durations.Merge(sk)
				}
				key := g.bucket.UTC().Format(time.RFC3339) + "|" + fmt.Sprint(g.vals)
				if cur := groups[key]; cur != nil {
					cur.agg.Merge(&g.agg)
				} else {
					g.bucket = g.bucket.UTC()
					groups[key] = &g
					order = append(order, key)
				}
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return fmt.Errorf("monitor query: %w", err)
			}
		}
		res = p.result(groups, order, names)
		res.Caps, err = caps(ctx, tx, q)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	switch {
	case len(p.minutes) == 0:
		res.ServedFrom = ServedRollups
	case len(p.days) == 0 && len(p.hours) == 0:
		res.ServedFrom = ServedRaw
	default:
		res.ServedFrom = ServedMixed
	}
	return res, nil
}

// passed is how many minutes of [from, to) are not in the future.
func passed(from, to, now time.Time) float64 {
	end := minTime(to, now.Truncate(time.Minute).Add(time.Minute))
	if !end.After(from) {
		return 0
	}
	return end.Sub(from).Minutes()
}

func (p *plan) step(t time.Time) time.Time {
	switch p.grain {
	case "minute":
		return t.Add(time.Minute)
	case "hour":
		return p.q.Zone.HourEnd(t)
	}
	return p.q.Zone.DayStart(p.q.Zone.LocalDay(t).AddDate(0, 0, 1))
}

// bucketStart is a bucket's first instant: day buckets are labelled with their local date.
func (p *plan) bucketStart(b time.Time) time.Time {
	if p.grain == "day" {
		return p.q.Zone.DayStart(b)
	}
	return b
}

func (p *plan) label(b time.Time) string {
	if p.grain == "day" {
		return b.Format(time.DateOnly)
	}
	return b.In(p.q.Zone.At(b)).Format(minuteLabel)
}

func (p *plan) result(groups map[string]*group, order []string, names nameMaps) Result {
	q := p.q
	res := Result{Totals: NewAgg(), Minutes: passed(q.Start, q.End, q.Now)}
	if p.grain != "" && len(p.dims) == 0 {
		have := map[time.Time]bool{}
		for _, g := range groups {
			have[p.bucketStart(g.bucket)] = true
		}
		for t := p.firstBucket(); t.Before(q.End); t = p.step(t) {
			if !have[t] {
				b := t
				if p.grain == "day" {
					b = q.Zone.LocalDay(t)
				}
				key := b.Format(time.RFC3339) + "|[]"
				groups[key] = &group{bucket: b, agg: NewAgg()}
				order = append(order, key)
			}
		}
	}
	for _, key := range order {
		g := groups[key]
		res.Totals.Merge(&g.agg)
		var dims []string
		minutes := res.Minutes
		if p.grain != "" {
			start := p.bucketStart(g.bucket)
			dims = append(dims, p.label(g.bucket))
			minutes = passed(maxTime(start, q.Start), minTime(p.step(start), q.End), q.Now)
		}
		for i, d := range p.dims {
			v := g.vals[i]
			switch d {
			case "route":
				dims = append(dims, names.routes[v])
			case "environment":
				dims = append(dims, names.envs[v])
			case "method":
				dims = append(dims, MethodName(int16(v)))
			case "status":
				dims = append(dims, strconv.Itoa(int(v)))
			case "status_class":
				dims = append(dims, StatusClassName(int16(v)))
			}
		}
		if p.grain != "" || len(p.dims) > 0 {
			res.Rows = append(res.Rows, Row{Dims: dims, Agg: g.agg, Minutes: minutes})
		}
	}
	if p.grain != "" {
		slices.SortStableFunc(res.Rows, func(a, b Row) int { return strings.Compare(a.Dims[0], b.Dims[0]) })
	}
	return res
}

func (p *plan) firstBucket() time.Time {
	q := p.q
	switch p.grain {
	case "minute":
		return q.Start.Truncate(time.Minute)
	case "hour":
		return q.Zone.LocalHour(q.Start)
	}
	return q.Zone.DayStart(q.Zone.LocalDay(q.Start))
}

// caps lists the caps the property has reached, and dimension_rows when an hour of the range did.
func caps(ctx context.Context, tx pgx.Tx, q Query) ([]string, error) {
	out := []string{}
	var routes int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM monitor_routes WHERE workspace_id = $1 AND property_id = $2 AND pattern NOT IN ('(other)', '(unmatched)')`,
		q.WorkspaceID, q.PropertyID).Scan(&routes); err != nil {
		return nil, err
	}
	if routes >= Caps[KindRoute] {
		out = append(out, "routes")
	}
	rows, err := tx.Query(ctx, `SELECT kind, count(*) FROM monitor_names WHERE workspace_id = $1 AND property_id = $2 AND value <> '(other)'
		AND kind IN ('consumer', 'api_version', 'app_version', 'environment') GROUP BY kind`, q.WorkspaceID, q.PropertyID)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for rows.Next() {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			rows.Close()
			return nil, err
		}
		counts[kind] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var services int
	if err := tx.QueryRow(ctx, `SELECT count(DISTINCT value) FROM monitor_names WHERE workspace_id = $1 AND kind = 'service' AND value <> '(other)'`,
		q.WorkspaceID).Scan(&services); err != nil {
		return nil, err
	}
	counts[KindService] = services
	for _, k := range []struct{ kind, name string }{{KindConsumer, "consumers"}, {KindAPIVersion, "api_versions"}, {KindAppVersion, "app_versions"},
		{KindEnvironment, "environments"}, {KindService, "services"}} {
		if counts[k.kind] >= Caps[k.kind] {
			out = append(out, k.name)
		}
	}
	var capped bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM monitor_rollup_hours WHERE workspace_id = $1 AND property_id = $2 AND capped
		AND hour > $3 AND hour < $4)`, q.WorkspaceID, q.PropertyID, q.Start.Add(-time.Hour), q.End).Scan(&capped); err != nil {
		return nil, err
	}
	if capped {
		out = append(out, "dimension_rows")
	}
	return out, nil
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

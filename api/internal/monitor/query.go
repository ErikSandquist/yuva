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
	minuteLabel      = "2006-01-02T15:04-07:00"

	RoutesNew    = "new"
	RoutesSilent = "silent"
)

// CoreDims break down and filter from every tier; one of ExtraDims joins only the dimensions its
// hourly rows keep (extraWith).
var (
	CoreDims  = []string{"route", "method", "status", "status_class", "api_version", "environment", "service", "source", "platform"}
	ExtraDims = []string{"consumer", "country", "app_version"}
	Dims      = slices.Concat(CoreDims, ExtraDims)
	Metrics   = []string{"requests", "requests_per_minute", "errors", "error_rate", "client_errors", "client_error_rate", "p50", "p75", "p90", "p95", "p99",
		"avg_duration", "max_duration", "apdex", "request_bytes", "response_bytes", "response_size_p50", "response_size_p95", "conversations", "latency_bands"}
	Grains = []string{"time:minute", "time:hour", "time:day", "time:week", "time:month"}

	extraWith       = []string{"route", "method", "status_class"}
	durationMetrics = []string{"p50", "p75", "p90", "p95", "p99", "apdex", "latency_bands"}
	sizeMetrics     = []string{"response_size_p50", "response_size_p95"}
	extraCodes      = map[string]int16{"consumer": DimConsumer, "country": DimCountry, "app_version": DimAppVersion}
	nameKinds       = map[string]string{"api_version": KindAPIVersion, "environment": KindEnvironment, "service": KindService, "consumer": KindConsumer,
		"country": KindCountry, "app_version": KindAppVersion}
	sourceNames   = map[int32]string{int32(SourceServer): "server", int32(SourceOTLP): "otlp", int32(SourceApp): "app"}
	platformNames = map[int32]string{int32(PlatformNone): "", int32(PlatformIOS): "ios", int32(PlatformAndroid): "android"}
)

var ErrBadQuery = analytics.ErrBadQuery

// ErrMinuteRange: a time:minute query reaches past the minute rows or has too many buckets.
var ErrMinuteRange = errors.New("time:minute serves at most 1,440 minutes within the minute rows")

// ErrDimensionCombination: the query asks for dimensions that are not kept together.
var ErrDimensionCombination = errors.New("dimension combination")

func badQuery(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrBadQuery, fmt.Sprintf(format, args...))
}

func combination(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrDimensionCombination, fmt.Sprintf(format, args...))
}

type Filter = analytics.Filter

// Query asks for an API property's requests of [Start, End), broken down by up to two dimensions
// (one may be a time grain) and narrowed by filters. MinuteFrom is the first minute minute rows are
// kept for; Days says the range was given as local days, which its previous period follows.
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
	// ApdexMS is the property's Apdex threshold, for routes without their own.
	ApdexMS int64
	// InboxID is the linked inbox whose conversations the conversations metric counts.
	InboxID *uuid.UUID
	// RouteState new or silent narrows the answer to Routes, and each of them gets a row.
	RouteState string
	Routes     []int32
}

// Previous is the period of the range's length just before it: as many local days for a range of
// days, as many minutes otherwise.
func (q Query) Previous() (time.Time, time.Time) {
	if q.Days {
		from, to := q.Zone.LocalDay(q.Start), q.Zone.LocalDay(q.End.Add(-time.Nanosecond))
		n := int(to.Sub(from).Hours()/24) + 1
		return q.Zone.DayStart(from.AddDate(0, 0, -n)), q.Start
	}
	return q.Start.Add(-q.End.Sub(q.Start)), q.Start
}

type Row struct {
	// Start is the first instant of a row's time bucket, which orders rows: labels of two zones do
	// not sort by time.
	Start time.Time
	Dims  []string
	Agg   Agg
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
	ms := func(q float64) float64 { return round(a.DurationQuantile(q)/1000, 6) }
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
		return ms(0.5)
	case "p75":
		return ms(0.75)
	case "p90":
		return ms(0.9)
	case "p95":
		return ms(0.95)
	case "p99":
		return ms(0.99)
	case "avg_duration":
		if a.Requests == 0 {
			return 0
		}
		return round(float64(a.DurationSum)/float64(a.Requests)/1000, 6)
	case "max_duration":
		if a.Requests == 0 {
			return 0
		}
		return round(float64(a.DurationMax)/1000, 6)
	case "apdex":
		if a.Requests == 0 {
			return 0
		}
		return round((float64(a.Satisfied)+float64(a.Tolerating)/2)/float64(a.Requests), 3)
	case "request_bytes":
		return float64(a.RequestBytes)
	case "response_bytes":
		return float64(a.ResponseBytes)
	case "response_size_p50":
		return round(a.Sizes.Quantile(0.5), 0)
	case "response_size_p95":
		return round(a.Sizes.Quantile(0.95), 0)
	case "conversations":
		return float64(a.Conversations)
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
	q     Query
	grain string
	dims  []string
	// extra is the extra dimension the query reads, which the dimension tables answer.
	extra   string
	days    []time.Time
	hours   []time.Time
	minutes [][2]time.Time
	// open counts the hours of an extra dimension query that are not folded yet.
	open                    int
	sketches, sizes, apdex  bool
	conversations, onlyTime bool
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
			return badQuery("the api dataset breaks down by %s and %s, not %q", strings.Join(Dims, ", "), strings.Join(Grains, ", "), d)
		}
	}
	for _, m := range q.Metrics {
		if !slices.Contains(Metrics, m) {
			return badQuery("the api dataset offers %s, not %s", strings.Join(Metrics, ", "), m)
		}
		p.sketches = p.sketches || slices.Contains(durationMetrics, m)
		p.sizes = p.sizes || slices.Contains(sizeMetrics, m)
		p.apdex = p.apdex || m == "apdex"
		p.conversations = p.conversations || m == "conversations"
	}
	for _, f := range q.Filters {
		if !slices.Contains(Dims, f.Dimension) {
			return badQuery("the api dataset filters on %s, not %q", strings.Join(Dims, ", "), f.Dimension)
		}
		if !slices.Contains([]string{"is", "is_not", "contains", "not_contains", "matches"}, f.Op) || len(f.Values) == 0 {
			return badQuery("the filter on %q needs an op of is, is_not, contains, not_contains or matches and at least one value", f.Dimension)
		}
	}
	used := slices.Clone(p.dims)
	for _, f := range q.Filters {
		if !slices.Contains(used, f.Dimension) {
			used = append(used, f.Dimension)
		}
	}
	for _, d := range used {
		if !slices.Contains(ExtraDims, d) {
			continue
		}
		if p.extra != "" && p.extra != d {
			return combination("consumer, country and app_version are kept apart: break down or filter by one of them, not %s and %s", p.extra, d)
		}
		p.extra = d
	}
	if p.extra != "" {
		for _, d := range used {
			if d != p.extra && !slices.Contains(extraWith, d) {
				return combination("%s is kept by local hour with route, method and status_class only, not with %s", p.extra, d)
			}
		}
		if p.grain == "minute" {
			return combination("%s is kept by local hour: break it down by time:hour, time:day, time:week or time:month, not time:minute", p.extra)
		}
		if !q.Zone.LocalHour(q.Start).Equal(q.Start) || !q.Zone.LocalHour(q.End).Equal(q.End) {
			return combination("%s is kept by local hour: give a range of whole local hours", p.extra)
		}
	}
	if p.conversations && (len(p.dims) > 0 || (p.grain != "" && !slices.Contains([]string{"day", "week", "month"}, p.grain)) || len(q.Filters) > 0 || q.RouteState != "") {
		return badQuery("conversations are counted per local day of the linked inbox: ask for them with no dimension or time:day, time:week or time:month alone, and no filter or routes")
	}
	if q.RouteState != "" && (len(q.Dimensions) != 1 || q.Dimensions[0] != "route") {
		return badQuery("routes takes the dimension route alone")
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
// hours from hourly rows, everything else from minute rows. An extra dimension has hourly rows of
// every writer for open hours too, and no minute rows. Hours without a row had no requests.
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
	byDay := map[int64]bool{}
	if p.grain != "hour" {
		ready := map[int64]bool{}
		days := map[int64]time.Time{}
		for _, h := range states {
			k := h.day.UnixNano()
			if _, ok := ready[k]; !ok {
				ready[k], days[k] = true, h.day
			}
			ready[k] = ready[k] && h.built && h.dayBuilt
		}
		for k, ok := range ready {
			d := days[k]
			if ok && !q.Zone.DayStart(d).Before(q.Start) && !q.Zone.DayStart(d.AddDate(0, 0, 1)).After(q.End) {
				byDay[k] = true
				p.days = append(p.days, d)
			}
		}
		slices.SortFunc(p.days, func(a, b time.Time) int { return a.Compare(b) })
	}
	for _, h := range states {
		if byDay[h.day.UnixNano()] {
			continue
		}
		end := q.Zone.HourEnd(h.hour)
		if p.extra != "" {
			if !h.hour.Before(q.Start) && h.hour.Before(q.End) {
				p.hours = append(p.hours, h.hour)
				if !h.built {
					p.open++
				}
			}
			continue
		}
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

// nameMaps maps a property's dictionary ids to their values.
type nameMaps struct {
	routes map[int32]string
	hidden []int32
	apdex  map[int32]int64
	kinds  map[string]map[int32]string
}

func loadNames(ctx context.Context, tx pgx.Tx, ws, prop uuid.UUID) (nameMaps, error) {
	out := nameMaps{routes: map[int32]string{}, apdex: map[int32]int64{}, kinds: map[string]map[int32]string{}}
	for _, kind := range nameKinds {
		out.kinds[kind] = map[int32]string{}
	}
	for _, kind := range []string{KindAPIVersion, KindEnvironment, KindService} {
		out.kinds[kind][0] = ""
	}
	rows, err := tx.Query(ctx, `SELECT id, pattern, hidden, apdex_ms FROM monitor_routes WHERE workspace_id = $1 AND property_id = $2 ORDER BY id`, ws, prop)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var id int32
		var pattern string
		var hidden bool
		var apdex *int32
		if err := rows.Scan(&id, &pattern, &hidden, &apdex); err != nil {
			rows.Close()
			return out, err
		}
		out.routes[id] = pattern
		if hidden {
			out.hidden = append(out.hidden, id)
		}
		if apdex != nil {
			out.apdex[id] = int64(*apdex)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	rows, err = tx.Query(ctx, `SELECT kind, id, value FROM monitor_names WHERE workspace_id = $1 AND property_id = $2`, ws, prop)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, v string
		var id int32
		if err := rows.Scan(&kind, &id, &v); err != nil {
			return out, err
		}
		if m := out.kinds[kind]; m != nil {
			m[id] = v
		}
	}
	return out, rows.Err()
}

// domain is every value a dimension can take, by its id or code.
func (n nameMaps) domain(d string) map[int32]string {
	out := map[int32]string{}
	switch d {
	case "route":
		return n.routes
	case "method":
		for code := range int16(len(methods)) {
			out[int32(code)] = MethodName(code)
		}
	case "status":
		for code := range int32(600) {
			out[code] = strconv.Itoa(int(code))
		}
	case "status_class":
		for class := range int16(6) {
			out[int32(class)] = StatusClassName(class)
		}
	case "source":
		return sourceNames
	case "platform":
		return platformNames
	default:
		return n.kinds[nameKinds[d]]
	}
	return out
}

func (n nameMaps) label(d string, v int32) string {
	switch d {
	case "status":
		return strconv.Itoa(int(v))
	case "method":
		return MethodName(int16(v))
	case "status_class":
		return StatusClassName(int16(v))
	}
	return n.domain(d)[v]
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

// column is a dimension's expression in a core table or, with dims, a dimension table.
func column(d string, dims bool) string {
	switch d {
	case "status_class":
		if dims {
			return "status_class::int"
		}
		return "(status / 100)::int"
	case "method", "status", "source", "platform":
		return d + "::int"
	case "consumer", "country", "app_version":
		return "value"
	}
	return d
}

// filterSQL turns the filters into conditions on ids and codes, numbering parameters from next.
func filterSQL(fs []Filter, names nameMaps, dims bool, next int) (string, []any) {
	var conds []string
	var args []any
	for _, f := range fs {
		set := []int32{}
		for id, v := range names.domain(f.Dimension) {
			if matches(f, v) {
				set = append(set, id)
			}
		}
		slices.Sort(set)
		cond := fmt.Sprintf("%s = ANY($%d::int[])", column(f.Dimension, dims), next+len(args))
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
// dates for days, weeks and months, each in the zone that applied at the row's time. Parameters
// go to args.
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
	}
	if t.timeCol == "day" {
		return "day::timestamp AT TIME ZONE 'UTC'"
	}
	return z.sql(t.timeCol, func(tz string) string {
		return "((" + t.timeCol + " AT TIME ZONE " + tz + ")::date)::timestamp AT TIME ZONE 'UTC'"
	}, false, args)
}

// coarse is the week (from Monday) or month a local date is in.
func (p *plan) coarse(day time.Time) time.Time {
	switch p.grain {
	case "week":
		return day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
	case "month":
		return time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	return day
}

func (p *plan) daily() bool { return p.grain == "day" || p.grain == "week" || p.grain == "month" }

type group struct {
	bucket time.Time
	vals   []int32
	agg    Agg
}

type groups struct {
	m     map[string]*group
	order []string
}

func (gs *groups) add(g *group) {
	key := bucketKey(g.bucket, g.vals)
	if cur := gs.m[key]; cur != nil {
		cur.agg.Merge(&g.agg)
		return
	}
	g.bucket = g.bucket.UTC()
	gs.m[key] = g
	gs.order = append(gs.order, key)
}

func (p *plan) tierList() []tier {
	var tiers []tier
	if p.extra != "" {
		code := extraCodes[p.extra]
		if len(p.days) > 0 {
			tiers = append(tiers, tier{table: "monitor_dim_day", timeCol: "day", where: "day = ANY($3::date[]) AND dim = $4", args: []any{p.days, code}})
		}
		if len(p.hours) > 0 {
			tiers = append(tiers, tier{table: DimHourTable, timeCol: "hour", where: "hour = ANY($3::timestamptz[]) AND dim = $4", args: []any{p.hours, code}})
		}
		return tiers
	}
	if len(p.days) > 0 {
		tiers = append(tiers, tier{table: "monitor_day", timeCol: "day", where: "day = ANY($3::date[])", args: []any{p.days}})
	}
	if len(p.hours) > 0 {
		tiers = append(tiers, tier{table: HourTable, timeCol: "hour", where: "hour = ANY($3::timestamptz[])", args: []any{p.hours}})
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
	return tiers
}

// collect adds up the rows of every tier by the query's bucket and dimensions. With apdex, each
// route's requests are counted under that route's threshold before they are added up.
func (p *plan) collect(ctx context.Context, tx pgx.Tx, names nameMaps) (*groups, error) {
	q := p.q
	out := &groups{m: map[string]*group{}}
	routeCol := slices.Index(p.dims, "route")
	hiddenRoute := p.apdex && routeCol < 0
	dims := p.extra != ""
	for _, t := range p.tierList() {
		args := append([]any{q.WorkspaceID, q.PropertyID}, t.args...)
		var cols []string
		if p.grain != "" {
			cols = append(cols, p.bucketSQL(t, &args))
		}
		for _, d := range p.dims {
			cols = append(cols, column(d, dims))
		}
		if hiddenRoute {
			cols = append(cols, "route")
		}
		fsql, fargs := filterSQL(q.Filters, names, dims, len(args)+1)
		args = append(args, fargs...)
		if len(names.hidden) > 0 {
			args = append(args, names.hidden)
			fsql += fmt.Sprintf(" AND NOT route = ANY($%d::int[])", len(args))
		}
		if q.RouteState != "" {
			args = append(args, append([]int32{}, q.Routes...))
			fsql += fmt.Sprintf(" AND route = ANY($%d::int[])", len(args))
		}
		sel := `sum(requests)::bigint, sum(errors)::bigint, sum(client_errors)::bigint, sum(sampled)::bigint, sum(duration_sum)::bigint,
			min(duration_min)::bigint, max(duration_max)::bigint, sum(request_bytes)::bigint, sum(response_bytes)::bigint`
		if p.sketches {
			sel += ", array_agg(durations)"
		}
		if p.sizes {
			sel += ", array_agg(response_sizes)"
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
			return nil, fmt.Errorf("monitor query: %w", err)
		}
		for rows.Next() {
			g := &group{vals: make([]int32, len(p.dims))}
			var dest []any
			if p.grain != "" {
				dest = append(dest, &g.bucket)
			}
			for i := range g.vals {
				dest = append(dest, &g.vals[i])
			}
			var route int32
			if hiddenRoute {
				dest = append(dest, &route)
			}
			a := &g.agg
			dest = append(dest, &a.Requests, &a.Errors, &a.ClientErrors, &a.Sampled, &a.DurationSum, &a.DurationMin, &a.DurationMax,
				&a.RequestBytes, &a.ResponseBytes)
			var durations, sizes [][]byte
			if p.sketches {
				dest = append(dest, &durations)
			}
			if p.sizes {
				dest = append(dest, &sizes)
			}
			if err := rows.Scan(dest...); err != nil {
				rows.Close()
				return nil, err
			}
			a.Durations, a.Sizes = NewSketch(DurationScale), NewSketch(SizeScale)
			for _, set := range []struct {
				raw [][]byte
				sk  *Sketch
			}{{durations, &a.Durations}, {sizes, &a.Sizes}} {
				for _, raw := range set.raw {
					sk, err := DecodeSketch(raw)
					if err != nil {
						rows.Close()
						return nil, err
					}
					set.sk.Merge(sk)
				}
			}
			if p.apdex {
				if routeCol >= 0 {
					route = g.vals[routeCol]
				}
				t := q.ApdexMS
				if ms, ok := names.apdex[route]; ok {
					t = ms
				}
				sat, tol := a.Durations.ApdexCounts(t * 1000)
				a.Satisfied, a.Tolerating = int64(sat), int64(tol)
			}
			if p.grain == "week" || p.grain == "month" {
				g.bucket = p.coarse(g.bucket.UTC())
			}
			out.add(g)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("monitor query: %w", err)
		}
	}
	return out, nil
}

// addConversations counts the conversations the linked inbox received in the range, by the local
// day they started on.
func (p *plan) addConversations(ctx context.Context, tx pgx.Tx, gs *groups) error {
	q := p.q
	if !p.conversations || q.InboxID == nil {
		return nil
	}
	args := []any{q.WorkspaceID, *q.InboxID, q.Start, q.End}
	sel, groupBy := "count(*)", ""
	if p.grain != "" {
		sel, groupBy = p.bucketSQL(tier{timeCol: "created_at"}, &args)+", count(*)", " GROUP BY 1"
	}
	rows, err := tx.Query(ctx, `SELECT `+sel+` FROM conversations WHERE workspace_id = $1 AND inbox_id = $2 AND kind <> 'feedback' AND NOT spam
		AND created_at >= $3 AND created_at < $4`+groupBy, args...)
	if err != nil {
		return fmt.Errorf("monitor conversations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		g := &group{agg: NewAgg()}
		dest := []any{&g.agg.Conversations}
		if p.grain != "" {
			dest = []any{&g.bucket, &g.agg.Conversations}
		}
		if err := rows.Scan(dest...); err != nil {
			return err
		}
		if g.agg.Conversations == 0 {
			continue
		}
		g.bucket = p.coarse(g.bucket.UTC())
		gs.add(g)
	}
	return rows.Err()
}

func readOnly(ctx context.Context, s *PGStore, fn func(tx pgx.Tx) error) error {
	return pgx.BeginTxFunc(ctx, s.Pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = "+strconv.Itoa(int(analytics.QueryTimeout.Milliseconds()))); err != nil {
			return err
		}
		return fn(tx)
	})
}

func (s *PGStore) Query(ctx context.Context, q Query) (Result, error) {
	p := &plan{q: q}
	if err := p.parse(); err != nil {
		return Result{}, err
	}
	var res Result
	err := readOnly(ctx, s, func(tx pgx.Tx) error {
		if err := p.tiers(ctx, tx); err != nil {
			return err
		}
		names, err := loadNames(ctx, tx, q.WorkspaceID, q.PropertyID)
		if err != nil {
			return err
		}
		gs, err := p.collect(ctx, tx, names)
		if err != nil {
			return err
		}
		if err := p.addConversations(ctx, tx, gs); err != nil {
			return err
		}
		res = p.result(gs, names)
		res.Caps, err = caps(ctx, tx, q)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	live, folded := len(p.minutes)+p.open, len(p.days)+len(p.hours)-p.open
	switch {
	case live == 0:
		res.ServedFrom = ServedRollups
	case folded == 0:
		res.ServedFrom = ServedRaw
	default:
		res.ServedFrom = ServedMixed
	}
	return res, nil
}

// RouteSet lists the routes a query with RouteState state narrows to: new ones, first seen in the
// range, or silent ones, called in the previous period (with the query's filters) and not since
// the range began. (unmatched), (other) and hidden routes are left out.
func (s *PGStore) RouteSet(ctx context.Context, q Query, state string) ([]int32, error) {
	out := []int32{}
	err := readOnly(ctx, s, func(tx pgx.Tx) error {
		names, err := loadNames(ctx, tx, q.WorkspaceID, q.PropertyID)
		if err != nil {
			return err
		}
		keep := func(id int32) bool { return !special(names.routes[id]) && !slices.Contains(names.hidden, id) }
		if state != RoutesNew && state != RoutesSilent {
			return badQuery("routes is new or silent, not %q", state)
		}
		base := q
		base.Dimensions, base.Metrics, base.RouteState, base.Routes = []string{"route"}, []string{"requests"}, "", nil
		called := func(start, end time.Time) (map[int32]bool, error) {
			sub := base
			sub.Start, sub.End = start, end
			p := &plan{q: sub}
			if err := p.parse(); err != nil {
				return nil, err
			}
			if err := p.tiers(ctx, tx); err != nil {
				return nil, err
			}
			gs, err := p.collect(ctx, tx, names)
			if err != nil {
				return nil, err
			}
			ids := map[int32]bool{}
			for _, g := range gs.m {
				if g.agg.Requests > 0 {
					ids[g.vals[0]] = true
				}
			}
			return ids, nil
		}
		if state == RoutesNew {
			during, err := called(q.Start, q.End)
			if err != nil {
				return err
			}
			var ids []int32
			for id := range during {
				if keep(id) {
					ids = append(ids, id)
				}
			}
			times, err := routeTimes(ctx, tx, q, ids)
			if err != nil {
				return err
			}
			for _, id := range ids {
				if t, ok := times[id]; ok && !t.First.Before(q.Start) {
					out = append(out, id)
				}
			}
			slices.Sort(out)
			return nil
		}
		from, to := q.Previous()
		before, err := called(from, to)
		if err != nil {
			return err
		}
		since, err := called(q.Start, maxTime(q.End, q.Zone.HourEnd(q.Zone.LocalHour(q.Now))))
		if err != nil {
			return err
		}
		for id := range before {
			if !since[id] && keep(id) {
				out = append(out, id)
			}
		}
		slices.Sort(out)
		return nil
	})
	return out, err
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
	z := p.q.Zone
	switch p.grain {
	case "minute":
		return t.Add(time.Minute)
	case "hour":
		return z.HourEnd(t)
	case "week":
		return z.DayStart(z.LocalDay(t).AddDate(0, 0, 7))
	case "month":
		return z.DayStart(z.LocalDay(t).AddDate(0, 1, 0))
	}
	return z.DayStart(z.LocalDay(t).AddDate(0, 0, 1))
}

// bucketStart is a bucket's first instant: day, week and month buckets are labelled with their
// local date.
func (p *plan) bucketStart(b time.Time) time.Time {
	if p.daily() {
		return p.q.Zone.DayStart(b)
	}
	return b
}

func (p *plan) label(b time.Time) string {
	if p.daily() {
		return b.Format(time.DateOnly)
	}
	return b.In(p.q.Zone.At(b)).Format(minuteLabel)
}

func (p *plan) result(gs *groups, names nameMaps) Result {
	q := p.q
	res := Result{Totals: NewAgg(), Minutes: passed(q.Start, q.End, q.Now)}
	if p.grain != "" && len(p.dims) == 0 {
		have := map[int64]bool{}
		for _, g := range gs.m {
			have[p.bucketStart(g.bucket).UnixNano()] = true
		}
		for t := p.firstBucket(); t.Before(q.End); t = p.step(t) {
			if have[t.UnixNano()] {
				continue
			}
			b := t.UTC()
			if p.daily() {
				b = q.Zone.LocalDay(t)
			}
			gs.add(&group{bucket: b, agg: NewAgg()})
		}
	}
	if q.RouteState != "" {
		seen := map[int32]bool{}
		for _, g := range gs.m {
			seen[g.vals[0]] = true
		}
		for _, id := range q.Routes {
			if !seen[id] {
				gs.add(&group{vals: []int32{id}, agg: NewAgg()})
			}
		}
	}
	for _, key := range gs.order {
		g := gs.m[key]
		res.Totals.Merge(&g.agg)
		var dims []string
		var start time.Time
		minutes := res.Minutes
		if p.grain != "" {
			start = p.bucketStart(g.bucket)
			dims = append(dims, p.label(g.bucket))
			minutes = passed(maxTime(start, q.Start), minTime(p.step(start), q.End), q.Now)
		}
		for i, d := range p.dims {
			dims = append(dims, names.label(d, g.vals[i]))
		}
		if p.grain != "" || len(p.dims) > 0 {
			res.Rows = append(res.Rows, Row{Start: start, Dims: dims, Agg: g.agg, Minutes: minutes})
		}
	}
	if p.grain != "" {
		slices.SortStableFunc(res.Rows, func(a, b Row) int { return a.Start.Compare(b.Start) })
	}
	return res
}

// bucketKey names a group by its bucket's instant and its dimension values, so the same instant
// in two locations is one group.
func bucketKey(bucket time.Time, vals []int32) string {
	return strconv.FormatInt(bucket.UnixNano(), 10) + "|" + fmt.Sprint(vals)
}

func (p *plan) firstBucket() time.Time {
	q := p.q
	switch p.grain {
	case "minute":
		return q.Start.Truncate(time.Minute)
	case "hour":
		return q.Zone.LocalHour(q.Start)
	}
	return q.Zone.DayStart(p.coarse(q.Zone.LocalDay(q.Start)))
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

// RouteTimes is when a route's first and latest requests were, to the finest rows that keep them.
type RouteTimes struct {
	First, Last time.Time
}

// routeTimes reads each route's first and latest request from the rollups: minute rows where they
// are kept, hour rows before them and day rows before the hours.
func routeTimes(ctx context.Context, tx pgx.Tx, q Query, ids []int32) (map[int32]RouteTimes, error) {
	out := map[int32]RouteTimes{}
	if len(ids) == 0 {
		return out, nil
	}
	var hoursFrom *time.Time
	if err := tx.QueryRow(ctx, `SELECT min(hour) FROM monitor_hour WHERE workspace_id = $1 AND property_id = $2`, q.WorkspaceID, q.PropertyID).Scan(&hoursFrom); err != nil {
		return nil, err
	}
	dayBefore := q.MinuteFrom
	if hoursFrom != nil && hoursFrom.Before(dayBefore) {
		dayBefore = *hoursFrom
	}
	type era struct {
		sql, col string
		before   time.Time
		day      bool
	}
	for _, e := range []era{
		{"monitor_minute", "minute", time.Time{}, false},
		{"monitor_hour", "hour", q.MinuteFrom, false},
		{"monitor_day", "day", q.Zone.LocalDay(dayBefore), true},
	} {
		first := "min(" + e.col + ")"
		args := []any{q.WorkspaceID, q.PropertyID, ids}
		if !e.before.IsZero() {
			first = "min(" + e.col + ") FILTER (WHERE " + e.col + " < $4)"
			args = append(args, e.before)
		}
		rows, err := tx.Query(ctx, "SELECT route, "+first+", max("+e.col+") FROM "+e.sql+
			" WHERE workspace_id = $1 AND property_id = $2 AND route = ANY($3::int[]) AND requests > 0 GROUP BY route", args...)
		if err != nil {
			return nil, fmt.Errorf("route times: %w", err)
		}
		for rows.Next() {
			var id int32
			var f *time.Time
			var l time.Time
			if err := rows.Scan(&id, &f, &l); err != nil {
				rows.Close()
				return nil, err
			}
			if e.day {
				l = q.Zone.DayStart(l)
				if f != nil {
					f = new(q.Zone.DayStart(*f))
				}
			}
			t, seen := out[id]
			if f != nil && (!seen || t.First.IsZero() || f.Before(t.First)) {
				t.First = f.UTC()
			}
			if l.After(t.Last) {
				t.Last = l.UTC()
			}
			out[id] = t
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	for id, t := range out {
		if t.First.IsZero() {
			delete(out, id)
		}
	}
	return out, nil
}

// RouteTimes says when each of a query's routes had its first and latest request, by pattern.
func (s *PGStore) RouteTimes(ctx context.Context, q Query, ids []int32) (map[string]RouteTimes, error) {
	out := map[string]RouteTimes{}
	err := readOnly(ctx, s, func(tx pgx.Tx) error {
		names, err := loadNames(ctx, tx, q.WorkspaceID, q.PropertyID)
		if err != nil {
			return err
		}
		times, err := routeTimes(ctx, tx, q, ids)
		if err != nil {
			return err
		}
		for id, t := range times {
			out[names.routes[id]] = t
		}
		return nil
	})
	return out, err
}

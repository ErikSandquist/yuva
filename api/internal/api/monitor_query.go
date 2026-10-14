package api

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/monitor"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const monitorOpenTTL = 5 * time.Second

var errMonitorNoInbox = problem(http.StatusUnprocessableEntity, "no_linked_inbox",
	"conversations need a property linked to an inbox; link one in the property's settings")

var monitorPeriods = map[oas.AnalyticsRangePeriod]time.Duration{
	oas.AnalyticsLast15Minutes: 15 * time.Minute, oas.AnalyticsLastHour: time.Hour, oas.AnalyticsLast6Hours: 6 * time.Hour,
	oas.AnalyticsLast24Hours: 24 * time.Hour,
}

// monitorRequest is an api query checked and turned into the property's query, its comparison and
// its page.
type monitorRequest struct {
	q      monitor.Query
	cmp    *monitor.Query
	order  []analytics.Order
	limit  int
	offset int
}

// monitorRange is the instants a range of an api query covers and whether it was given as days.
func monitorRange(r oas.AnalyticsRange, zone monitor.Zone, now time.Time) (time.Time, time.Time, bool, error) {
	if r.Since != nil || r.Until != nil {
		if r.Period != nil || r.From != nil || r.To != nil {
			return time.Time{}, time.Time{}, false, errValidation("send range.since and range.until without period, from or to")
		}
		if r.Since == nil || r.Until == nil {
			return time.Time{}, time.Time{}, false, errValidation("range.since needs range.until")
		}
		since, until := r.Since.UTC(), r.Until.UTC()
		if !since.Truncate(time.Minute).Equal(since) || !until.Truncate(time.Minute).Equal(until) {
			return time.Time{}, time.Time{}, false, errValidation("range.since and range.until fall on a whole minute")
		}
		if !until.After(since) {
			return time.Time{}, time.Time{}, false, errValidation("the range ends before it starts")
		}
		if until.Sub(since) > maxAnalyticsRange*24*time.Hour {
			return time.Time{}, time.Time{}, false, problem(http.StatusUnprocessableEntity, "range_too_large", "a range may span at most 400 days")
		}
		return since, until, false, nil
	}
	if r.Period != nil {
		if d, ok := monitorPeriods[*r.Period]; ok {
			if r.From != nil || r.To != nil {
				return time.Time{}, time.Time{}, false, errValidation("send either range.period, or range.from and range.to")
			}
			end := now.Truncate(time.Minute).Add(time.Minute)
			return end.Add(-d), end, false, nil
		}
	}
	from, to, err := analyticsRange(r, zone.LocalDay(now))
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	return zone.DayStart(from), zone.DayStart(to.AddDate(0, 0, 1)), true, nil
}

// monitorComparison is the range an api query is compared with.
func monitorComparison(c oas.AnalyticsCompare, q monitor.Query) (monitor.Query, error) {
	out := q
	z := q.Zone
	switch c.With {
	case oas.AnalyticsComparePrevious:
		out.Start, out.End = q.Previous()
	case oas.AnalyticsCompareYear:
		if q.Days {
			from, to := z.LocalDay(q.Start), z.LocalDay(q.End.Add(-time.Nanosecond))
			out.Start, out.End = z.DayStart(from.AddDate(-1, 0, 0)), z.DayStart(to.AddDate(-1, 0, 1))
		} else {
			out.Start, out.End = q.Start.AddDate(-1, 0, 0), q.End.AddDate(-1, 0, 0)
		}
	case oas.AnalyticsCompareRange:
		if c.From == nil || c.To == nil {
			return out, errValidation("compare.with range needs compare.from and compare.to")
		}
		from, to, err := checkedRange(c.From.Time, c.To.Time)
		if err != nil {
			return out, err
		}
		out.Start, out.End, out.Days = z.DayStart(from), z.DayStart(to.AddDate(0, 0, 1)), true
	default:
		return out, errValidation("compare.with must be previous, year or range")
	}
	return out, nil
}

func (s *Server) monitorRequest(prop store.AnalyticsProperty, b oas.AnalyticsQuery) (monitorRequest, error) {
	var mr monitorRequest
	m := s.analytics.monitor
	if b.GoalId != nil || b.Prop != nil || b.SegmentId != nil || (b.Insights != nil && *b.Insights) {
		return mr, errValidation("the api dataset takes no goal_id, prop, segment_id or insights")
	}
	zone, now := monitorZone(prop.Timezone, prop.TimezoneHistory), s.now()
	q := monitor.Query{WorkspaceID: prop.WorkspaceID, PropertyID: prop.ID, Zone: zone, Now: now, ApdexMS: int64(prop.ApdexMs), InboxID: prop.InboxID,
		MinuteFrom: monitor.LocalDay(now, time.UTC).AddDate(0, 0, 1-m.settings.MinuteDays)}
	var err error
	if q.Start, q.End, q.Days, err = monitorRange(b.Range, zone, now); err != nil {
		return mr, err
	}
	for _, metric := range b.Metrics {
		if slices.Contains(q.Metrics, string(metric)) {
			return mr, errValidation("metric " + string(metric) + " is listed twice")
		}
		q.Metrics = append(q.Metrics, string(metric))
	}
	if slices.Contains(q.Metrics, "conversations") && prop.InboxID == nil {
		return mr, errMonitorNoInbox
	}
	if b.Dimensions != nil {
		for _, d := range *b.Dimensions {
			q.Dimensions = append(q.Dimensions, string(d))
		}
	}
	if b.Filters != nil {
		for _, f := range *b.Filters {
			if len(f.Values) == 0 || len(f.Values) > 20 {
				return mr, errValidation("a filter has 1 to 20 values")
			}
			if f.Prop != nil {
				return mr, errValidation("the api dataset has no props")
			}
			q.Filters = append(q.Filters, monitor.Filter{Dimension: string(f.Dimension), Op: string(f.Op), Values: f.Values})
		}
	}
	if b.Routes != nil {
		q.RouteState = string(*b.Routes)
	}
	first := "requests"
	if !slices.Contains(q.Metrics, first) && len(q.Metrics) > 0 {
		first = q.Metrics[0]
	}
	mr.order = []analytics.Order{{Metric: first, Desc: true}}
	if b.OrderBy != nil && len(*b.OrderBy) > 0 {
		mr.order = nil
		for _, o := range *b.OrderBy {
			if !slices.Contains(monitor.Metrics, string(o.Metric)) {
				return mr, errValidation("the api dataset orders by its own metrics, not " + string(o.Metric))
			}
			mr.order = append(mr.order, analytics.Order{Metric: string(o.Metric), Desc: o.Dir == nil || *o.Dir == oas.AnalyticsDesc})
		}
	}
	mr.limit = defaultAnalyticsLimit
	if b.Limit != nil {
		mr.limit = *b.Limit
	}
	if mr.limit < 1 || mr.limit > 1000 {
		return mr, errValidation("limit must be 1 to 1000")
	}
	if b.Cursor != nil && *b.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(*b.Cursor)
		if err == nil {
			mr.offset, err = strconv.Atoi(string(raw))
		}
		if err != nil || mr.offset < 0 {
			return mr, errValidation("cursor is not one this query returned")
		}
	}
	if b.Compare != nil {
		c, err := monitorComparison(*b.Compare, q)
		if err != nil {
			return mr, err
		}
		mr.cmp = &c
	}
	mr.q = q
	return mr, nil
}

func monitorQueryError(err error) error {
	switch {
	case errors.Is(err, monitor.ErrMinuteRange):
		return problem(http.StatusUnprocessableEntity, "range_too_large", err.Error())
	case errors.Is(err, monitor.ErrDimensionCombination):
		return problem(http.StatusUnprocessableEntity, "dimension_combination", strings.TrimPrefix(err.Error(), monitor.ErrDimensionCombination.Error()+": "))
	}
	return analyticsQueryError(err)
}

func apiMetricsBody(a *monitor.Agg, minutes float64, names []string) oas.AnalyticsMetrics {
	var m oas.AnalyticsMetrics
	fields := map[string]**float64{
		"requests": &m.Requests, "requests_per_minute": &m.RequestsPerMinute, "errors": &m.Errors, "error_rate": &m.ErrorRate,
		"client_errors": &m.ClientErrors, "client_error_rate": &m.ClientErrorRate, "p50": &m.P50, "p75": &m.P75, "p90": &m.P90, "p95": &m.P95,
		"p99": &m.P99, "avg_duration": &m.AvgDuration, "max_duration": &m.MaxDuration, "apdex": &m.Apdex, "request_bytes": &m.RequestBytes,
		"response_bytes": &m.ResponseBytes, "response_size_p50": &m.ResponseSizeP50, "response_size_p95": &m.ResponseSizeP95,
		"conversations": &m.Conversations,
	}
	for _, n := range names {
		if f, ok := fields[n]; ok {
			*f = new(a.Metric(n, minutes))
		}
	}
	return m
}

// sortMonitorRows orders rows by the order's metrics; with a time grain, in time order first.
func sortMonitorRows(rows []monitor.Row, dims []string, order []analytics.Order) {
	if timeOnly(dims) {
		return
	}
	slices.SortStableFunc(rows, func(a, b monitor.Row) int {
		for _, o := range order {
			x, y := a.Agg.Metric(o.Metric, a.Minutes), b.Agg.Metric(o.Metric, b.Minutes)
			if c := cmp.Compare(x, y); c != 0 {
				if o.Desc {
					return -c
				}
				return c
			}
		}
		return strings.Compare(strings.Join(a.Dims, "\x1f"), strings.Join(b.Dims, "\x1f"))
	})
	if timeIndex(dims) >= 0 {
		slices.SortStableFunc(rows, func(a, b monitor.Row) int { return a.Start.Compare(b.Start) })
	}
}

// runMonitorQuery answers from the cache or the store, at most four queries at a time per
// workspace like every analytics query. Answers that read minute rows or reach into the current
// minute live five seconds, others until a fold moves the property's rollup version.
func (s *Server) runMonitorQuery(ctx context.Context, prop store.AnalyticsProperty, q monitor.Query) (monitor.Result, error) {
	a, m := s.analytics, s.analytics.monitor
	now := q.Now
	keyQ := q
	keyQ.Now, keyQ.Zone = time.Time{}, monitor.Zone{}
	key, _ := json.Marshal(keyQ)
	cacheKey := prop.ID.String() + "|" + strconv.Itoa(int(prop.RollupVersion)) + "|api|" + prop.Timezone + string(prop.TimezoneHistory) + "|" + string(key)
	if v, ok := a.answers.get(cacheKey, now); ok {
		return v.(monitor.Result), nil
	}
	release, err := a.slots.acquire(ctx, prop.WorkspaceID)
	if err != nil {
		return monitor.Result{}, err
	}
	defer release()
	res, err := m.store.Query(ctx, q)
	if err != nil {
		return res, monitorQueryError(err)
	}
	ttl := analyticsFinishedTTL
	if res.ServedFrom != monitor.ServedRollups || q.End.After(now) {
		ttl = monitorOpenTTL
	}
	a.answers.put(cacheKey, res, now, ttl)
	return res, nil
}

// monitorRoutes narrows a query and its comparison to the new or silent routes of its range.
func (s *Server) monitorRoutes(ctx context.Context, prop store.AnalyticsProperty, mr *monitorRequest) error {
	if mr.q.RouteState == "" {
		return nil
	}
	if len(mr.q.Dimensions) != 1 || mr.q.Dimensions[0] != "route" {
		return errValidation("routes takes the dimension route alone")
	}
	release, err := s.analytics.slots.acquire(ctx, prop.WorkspaceID)
	if err != nil {
		return err
	}
	defer release()
	ids, err := s.analytics.monitor.store.RouteSet(ctx, mr.q, mr.q.RouteState)
	if err != nil {
		return monitorQueryError(err)
	}
	mr.q.Routes = ids
	if mr.cmp != nil {
		mr.cmp.RouteState, mr.cmp.Routes = mr.q.RouteState, ids
	}
	return nil
}

// bucketOrder numbers a result's time buckets in time order, so a comparison's rows line up with
// the range's by position.
func bucketOrder(rows []monitor.Row, ti int) map[string]int {
	starts := map[string]time.Time{}
	for _, r := range rows {
		starts[r.Dims[ti]] = r.Start
	}
	labels := make([]string, 0, len(starts))
	for l := range starts {
		labels = append(labels, l)
	}
	slices.SortFunc(labels, func(a, b string) int { return starts[a].Compare(starts[b]) })
	out := map[string]int{}
	for i, l := range labels {
		out[l] = i
	}
	return out
}

func monitorRowKey(dims []string, ti int, order map[string]int) string {
	parts := slices.Clone(dims)
	if ti >= 0 {
		parts[ti] = strconv.Itoa(order[parts[ti]])
	}
	return strings.Join(parts, "\x1f")
}

func (s *Server) queryMonitor(ctx context.Context, prop store.AnalyticsProperty, b oas.AnalyticsQuery, csv bool) (oas.QueryAnalyticsResponseObject, error) {
	if s.analytics.monitor == nil {
		return nil, errNotFound
	}
	if !isAPIProperty(prop) {
		return nil, errAPIOnly
	}
	if b.Dataset == nil || *b.Dataset != oas.AnalyticsDatasetApi {
		return nil, errSiteOnly
	}
	mr, err := s.monitorRequest(prop, b)
	if err != nil {
		return nil, err
	}
	if err := s.monitorRoutes(ctx, prop, &mr); err != nil {
		return nil, err
	}
	res, err := s.runMonitorQuery(ctx, prop, mr.q)
	if err != nil {
		return nil, err
	}
	var prev *monitor.Result
	if mr.cmp != nil {
		c, err := s.runMonitorQuery(ctx, prop, *mr.cmp)
		if err != nil {
			return nil, err
		}
		prev = &c
	}
	q := mr.q
	rows := slices.Clone(res.Rows)
	sortMonitorRows(rows, q.Dimensions, mr.order)
	next := ""
	if !timeOnly(q.Dimensions) {
		offset := min(mr.offset, len(rows))
		end := min(offset+mr.limit, len(rows))
		if end < len(rows) {
			next = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(end)))
		}
		rows = rows[offset:end]
	}
	out := oas.AnalyticsResult{
		Rows: make([]oas.AnalyticsRow, len(rows)), Totals: apiMetricsBody(&res.Totals, res.Minutes, q.Metrics),
		Meta: oas.AnalyticsResultMeta{
			From: openapi_types.Date{Time: q.Zone.LocalDay(q.Start)}, To: openapi_types.Date{Time: q.Zone.LocalDay(q.End.Add(-time.Nanosecond))},
			Since: new(q.Start), Until: new(q.End), Timezone: prop.Timezone, ServedFrom: oas.AnalyticsResultMetaServedFrom(res.ServedFrom),
			RawWindowFrom: openapi_types.Date{Time: q.MinuteFrom},
		},
	}
	if res.Totals.Sampled > 0 || (prev != nil && prev.Totals.Sampled > 0) {
		out.Meta.Estimated = new(true)
	}
	if len(res.Caps) > 0 {
		caps := make([]oas.AnalyticsResultMetaCapsReached, len(res.Caps))
		for i, c := range res.Caps {
			caps[i] = oas.AnalyticsResultMetaCapsReached(c)
		}
		out.Meta.CapsReached = &caps
	}
	if next != "" {
		out.NextCursor = &next
	}
	for i, r := range rows {
		out.Rows[i] = oas.AnalyticsRow{Dimensions: slices.Clone(r.Dims), Metrics: apiMetricsBody(&r.Agg, r.Minutes, q.Metrics)}
	}
	if prev != nil {
		c := mr.cmp
		ti := timeIndex(q.Dimensions)
		var cur, before map[string]int
		if ti >= 0 {
			cur, before = bucketOrder(res.Rows, ti), bucketOrder(prev.Rows, ti)
		}
		byKey := map[string]monitor.Row{}
		for _, r := range prev.Rows {
			byKey[monitorRowKey(r.Dims, ti, before)] = r
		}
		comp := oas.AnalyticsComparison{From: openapi_types.Date{Time: c.Zone.LocalDay(c.Start)}, To: openapi_types.Date{Time: c.Zone.LocalDay(c.End.Add(-time.Nanosecond))},
			Since: new(c.Start), Until: new(c.End), Totals: apiMetricsBody(&prev.Totals, prev.Minutes, q.Metrics), Rows: make([]oas.AnalyticsRow, len(rows))}
		for i, r := range rows {
			m, ok := byKey[monitorRowKey(r.Dims, ti, cur)]
			if !ok {
				m = monitor.Row{Dims: r.Dims, Agg: monitor.NewAgg()}
			}
			comp.Rows[i] = oas.AnalyticsRow{Dimensions: slices.Clone(m.Dims), Metrics: apiMetricsBody(&m.Agg, m.Minutes, q.Metrics)}
		}
		out.Comparison = &comp
	}
	if csv {
		body := analyticsCSV(q.Dimensions, q.Metrics, out.Rows)
		name := `attachment; filename="api-` + out.Meta.From.Format(time.DateOnly) + `-` + out.Meta.To.Format(time.DateOnly) + `.csv"`
		return oas.QueryAnalytics200TextcsvResponse{Body: bytes.NewReader(body), ContentLength: int64(len(body)),
			Headers: oas.QueryAnalytics200ResponseHeaders{ContentDisposition: &name}}, nil
	}
	return oas.QueryAnalytics200JSONResponse{Body: out}, nil
}

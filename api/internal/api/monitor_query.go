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

var monitorPeriods = map[oas.AnalyticsRangePeriod]time.Duration{
	oas.AnalyticsLast15Minutes: 15 * time.Minute, oas.AnalyticsLastHour: time.Hour, oas.AnalyticsLast6Hours: 6 * time.Hour,
	oas.AnalyticsLast24Hours: 24 * time.Hour,
}

// monitorRequest is an api query checked and turned into the property's query and its page.
type monitorRequest struct {
	q      monitor.Query
	order  []analytics.Order
	limit  int
	offset int
}

func (s *Server) monitorRequest(prop store.AnalyticsProperty, b oas.AnalyticsQuery) (monitorRequest, error) {
	var mr monitorRequest
	m := s.analytics.monitor
	if b.GoalId != nil || b.Prop != nil || b.SegmentId != nil || b.Compare != nil || (b.Insights != nil && *b.Insights) {
		return mr, errValidation("the api dataset takes no goal_id, prop, segment_id, compare or insights")
	}
	zone, now := monitorZone(prop.Timezone, prop.TimezoneHistory), s.now()
	q := monitor.Query{WorkspaceID: prop.WorkspaceID, PropertyID: prop.ID, Zone: zone, Now: now,
		MinuteFrom: monitor.LocalDay(now, time.UTC).AddDate(0, 0, 1-m.settings.MinuteDays)}
	if d, ok := monitorPeriods[derefPeriod(b.Range.Period)]; ok && b.Range.Period != nil {
		if b.Range.From != nil || b.Range.To != nil {
			return mr, errValidation("send either range.period, or range.from and range.to")
		}
		q.End = now.Truncate(time.Minute).Add(time.Minute)
		q.Start = q.End.Add(-d)
	} else {
		from, to, err := analyticsRange(b.Range, zone.LocalDay(now))
		if err != nil {
			return mr, err
		}
		q.Start, q.End, q.Days = zone.DayStart(from), zone.DayStart(to.AddDate(0, 0, 1)), true
	}
	for _, metric := range b.Metrics {
		if slices.Contains(q.Metrics, string(metric)) {
			return mr, errValidation("metric " + string(metric) + " is listed twice")
		}
		q.Metrics = append(q.Metrics, string(metric))
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
	mr.q = q
	return mr, nil
}

func derefPeriod(p *oas.AnalyticsRangePeriod) oas.AnalyticsRangePeriod {
	if p == nil {
		return ""
	}
	return *p
}

func monitorQueryError(err error) error {
	if errors.Is(err, monitor.ErrMinuteRange) {
		return problem(http.StatusUnprocessableEntity, "range_too_large", err.Error())
	}
	return analyticsQueryError(err)
}

func apiMetricsBody(a *monitor.Agg, minutes float64, names []string) oas.AnalyticsMetrics {
	var m oas.AnalyticsMetrics
	for _, n := range names {
		v := a.Metric(n, minutes)
		switch n {
		case "requests":
			m.Requests = &v
		case "requests_per_minute":
			m.RequestsPerMinute = &v
		case "errors":
			m.Errors = &v
		case "error_rate":
			m.ErrorRate = &v
		case "client_errors":
			m.ClientErrors = &v
		case "client_error_rate":
			m.ClientErrorRate = &v
		case "p50":
			m.P50 = &v
		case "p95":
			m.P95 = &v
		case "p99":
			m.P99 = &v
		case "avg_duration":
			m.AvgDuration = &v
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
	res, err := s.runMonitorQuery(ctx, prop, mr.q)
	if err != nil {
		return nil, err
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
			Timezone: prop.Timezone, ServedFrom: oas.AnalyticsResultMetaServedFrom(res.ServedFrom), RawWindowFrom: openapi_types.Date{Time: q.MinuteFrom},
		},
	}
	if res.Totals.Sampled > 0 {
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
	if csv {
		body := analyticsCSV(q.Dimensions, q.Metrics, out.Rows)
		name := `attachment; filename="api-` + out.Meta.From.Format(time.DateOnly) + `-` + out.Meta.To.Format(time.DateOnly) + `.csv"`
		return oas.QueryAnalytics200TextcsvResponse{Body: bytes.NewReader(body), ContentLength: int64(len(body)),
			Headers: oas.QueryAnalytics200ResponseHeaders{ContentDisposition: &name}}, nil
	}
	return oas.QueryAnalytics200JSONResponse{Body: out}, nil
}

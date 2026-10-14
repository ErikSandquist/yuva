package api

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/monitor"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

var errUptimeOnly = problem(http.StatusUnprocessableEntity, "property_kind", "queryMonitoring answers dataset uptime; query a property for its other datasets")

func uptimeMetricsBody(a *monitor.UptimeAgg, names []string) oas.AnalyticsMetrics {
	var m oas.AnalyticsMetrics
	fields := map[string]**float64{
		"runs": &m.Runs, "failures": &m.Failures, "up": &m.Up, "uptime": &m.Uptime, "downtime": &m.Downtime, "response_time_p50": &m.ResponseTimeP50,
		"response_time_p95": &m.ResponseTimeP95, "response_time_p99": &m.ResponseTimeP99, "avg_response_time": &m.AvgResponseTime, "ttfb": &m.Ttfb,
		"cert_days_left": &m.CertDaysLeft,
	}
	for _, n := range names {
		if f, ok := fields[n]; ok {
			*f = new(a.Metric(n))
		}
	}
	return m
}

type uptimeRequest struct {
	q      monitor.UptimeQuery
	cmp    *monitor.UptimeQuery
	order  []analytics.Order
	limit  int
	offset int
}

// uptimeRequest checks an uptime query body; the checks are the property's, or the workspace's
// without one, narrowed by the check filters.
func (s *Server) uptimeRequest(ctx context.Context, ws uuid.UUID, prop *store.AnalyticsProperty, b oas.AnalyticsQuery) (uptimeRequest, error) {
	var ur uptimeRequest
	m := s.analytics.monitor
	if b.GoalId != nil || b.Prop != nil || b.SegmentId != nil || b.Routes != nil || (b.Insights != nil && *b.Insights) {
		return ur, errValidation("the uptime dataset takes no goal_id, prop, segment_id, routes or insights")
	}
	now := s.now()
	utc := monitor.FixedZone(time.UTC)
	q := monitor.UptimeQuery{WorkspaceID: ws, Now: now, ResultsFrom: monitor.LocalDay(now, time.UTC).AddDate(0, 0, 1-m.settings.CheckDays)}
	start, end, days, err := monitorRange(b.Range, utc, now)
	if err != nil {
		return ur, err
	}
	q.Start, q.End = start, end
	for _, metric := range b.Metrics {
		if slices.Contains(q.Metrics, string(metric)) {
			return ur, errValidation("metric " + string(metric) + " is listed twice")
		}
		q.Metrics = append(q.Metrics, string(metric))
	}
	if b.Dimensions != nil {
		for _, d := range *b.Dimensions {
			q.Dimensions = append(q.Dimensions, string(d))
		}
	}
	var propID *uuid.UUID
	if prop != nil {
		propID = &prop.ID
	}
	checks, err := s.st.ListMonitorChecks(ctx, store.ListMonitorChecksParams{WorkspaceID: ws, PropertyID: propID})
	if err != nil {
		return ur, err
	}
	keep := func(id uuid.UUID) bool { return true }
	if b.Filters != nil {
		var tests []func(uuid.UUID) bool
		for _, f := range *b.Filters {
			if f.Dimension != oas.AnalyticsDimCheck || (f.Op != oas.AnalyticsFilterIs && f.Op != oas.AnalyticsFilterIsNot) || len(f.Values) == 0 || len(f.Values) > 20 {
				return ur, errValidation("the uptime dataset filters on check with is or is_not and 1 to 20 check ids")
			}
			values, negate := f.Values, f.Op == oas.AnalyticsFilterIsNot
			tests = append(tests, func(id uuid.UUID) bool { return slices.Contains(values, id.String()) != negate })
		}
		keep = func(id uuid.UUID) bool {
			for _, t := range tests {
				if !t(id) {
					return false
				}
			}
			return true
		}
	}
	for _, c := range checks {
		if keep(c.ID) {
			q.Checks = append(q.Checks, monitor.UptimeCheck{ID: c.ID, FoldedUntil: c.FoldedUntil})
		}
	}
	first := q.Metrics[0]
	ur.order = []analytics.Order{{Metric: first, Desc: true}}
	if b.OrderBy != nil && len(*b.OrderBy) > 0 {
		ur.order = nil
		for _, o := range *b.OrderBy {
			if !slices.Contains(monitor.UptimeMetrics, string(o.Metric)) {
				return ur, errValidation("the uptime dataset orders by its own metrics, not " + string(o.Metric))
			}
			ur.order = append(ur.order, analytics.Order{Metric: string(o.Metric), Desc: o.Dir == nil || *o.Dir == oas.AnalyticsDesc})
		}
	}
	ur.limit = defaultAnalyticsLimit
	if b.Limit != nil {
		ur.limit = *b.Limit
	}
	if ur.limit < 1 || ur.limit > 1000 {
		return ur, errValidation("limit must be 1 to 1000")
	}
	if b.Cursor != nil && *b.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(*b.Cursor)
		if err == nil {
			ur.offset, err = strconv.Atoi(string(raw))
		}
		if err != nil || ur.offset < 0 {
			return ur, errValidation("cursor is not one this query returned")
		}
	}
	if b.Compare != nil {
		c, err := monitorComparison(*b.Compare, monitor.Query{Zone: utc, Start: q.Start, End: q.End, Days: days})
		if err != nil {
			return ur, err
		}
		cq := q
		cq.Start, cq.End = c.Start, c.End
		ur.cmp = &cq
	}
	ur.q = q
	return ur, nil
}

func (s *Server) runUptimeQuery(ctx context.Context, q monitor.UptimeQuery) (monitor.UptimeResult, error) {
	release, err := s.analytics.slots.acquire(ctx, q.WorkspaceID)
	if err != nil {
		return monitor.UptimeResult{}, err
	}
	defer release()
	res, err := s.analytics.monitor.store.QueryUptime(ctx, q)
	if err != nil {
		return res, monitorQueryError(err)
	}
	return res, nil
}

func sortUptimeRows(rows []monitor.UptimeRow, dims []string, order []analytics.Order) {
	if timeOnly(dims) {
		return
	}
	slices.SortStableFunc(rows, func(a, b monitor.UptimeRow) int {
		for _, o := range order {
			if c := cmp.Compare(a.Agg.Metric(o.Metric), b.Agg.Metric(o.Metric)); c != 0 {
				if o.Desc {
					return -c
				}
				return c
			}
		}
		return strings.Compare(strings.Join(a.Dims, "\x1f"), strings.Join(b.Dims, "\x1f"))
	})
	if timeIndex(dims) >= 0 {
		slices.SortStableFunc(rows, func(a, b monitor.UptimeRow) int { return a.Start.Compare(b.Start) })
	}
}

// queryUptime answers dataset uptime over a property's checks, or the workspace's with prop nil.
func (s *Server) queryUptime(ctx context.Context, ws uuid.UUID, prop *store.AnalyticsProperty, b oas.AnalyticsQuery) (oas.AnalyticsResult, error) {
	ur, err := s.uptimeRequest(ctx, ws, prop, b)
	if err != nil {
		return oas.AnalyticsResult{}, err
	}
	res, err := s.runUptimeQuery(ctx, ur.q)
	if err != nil {
		return oas.AnalyticsResult{}, err
	}
	var prev *monitor.UptimeResult
	if ur.cmp != nil {
		c, err := s.runUptimeQuery(ctx, *ur.cmp)
		if err != nil {
			return oas.AnalyticsResult{}, err
		}
		prev = &c
	}
	q := ur.q
	rows := slices.Clone(res.Rows)
	sortUptimeRows(rows, q.Dimensions, ur.order)
	next := ""
	if !timeOnly(q.Dimensions) {
		offset := min(ur.offset, len(rows))
		end := min(offset+ur.limit, len(rows))
		if end < len(rows) {
			next = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(end)))
		}
		rows = rows[offset:end]
	}
	utc := monitor.FixedZone(time.UTC)
	out := oas.AnalyticsResult{
		Rows: make([]oas.AnalyticsRow, len(rows)), Totals: uptimeMetricsBody(&res.Totals, q.Metrics),
		Meta: oas.AnalyticsResultMeta{
			From: openapi_types.Date{Time: utc.LocalDay(q.Start)}, To: openapi_types.Date{Time: utc.LocalDay(q.End.Add(-time.Nanosecond))},
			Since: new(q.Start), Until: new(q.End), Timezone: "UTC", ServedFrom: oas.AnalyticsResultMetaServedFrom(res.ServedFrom),
			RawWindowFrom: openapi_types.Date{Time: q.ResultsFrom},
		},
	}
	if next != "" {
		out.NextCursor = &next
	}
	for i, r := range rows {
		out.Rows[i] = oas.AnalyticsRow{Dimensions: slices.Clone(r.Dims), Metrics: uptimeMetricsBody(&r.Agg, q.Metrics)}
	}
	if prev != nil {
		c := ur.cmp
		ti := timeIndex(q.Dimensions)
		position := func(rs []monitor.UptimeRow) map[string]int {
			starts := map[string]time.Time{}
			for _, r := range rs {
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
		var cur, before map[string]int
		if ti >= 0 {
			cur, before = position(res.Rows), position(prev.Rows)
		}
		byKey := map[string]monitor.UptimeRow{}
		for _, r := range prev.Rows {
			byKey[monitorRowKey(r.Dims, ti, before)] = r
		}
		comp := oas.AnalyticsComparison{From: openapi_types.Date{Time: utc.LocalDay(c.Start)}, To: openapi_types.Date{Time: utc.LocalDay(c.End.Add(-time.Nanosecond))},
			Since: new(c.Start), Until: new(c.End), Totals: uptimeMetricsBody(&prev.Totals, q.Metrics), Rows: make([]oas.AnalyticsRow, len(rows))}
		for i, r := range rows {
			m, ok := byKey[monitorRowKey(r.Dims, ti, cur)]
			if !ok {
				m = monitor.UptimeRow{Dims: r.Dims, Agg: monitor.NewUptimeAgg()}
			}
			comp.Rows[i] = oas.AnalyticsRow{Dimensions: slices.Clone(m.Dims), Metrics: uptimeMetricsBody(&m.Agg, q.Metrics)}
		}
		out.Comparison = &comp
	}
	return out, nil
}

func (s *Server) QueryMonitoring(ctx context.Context, req oas.QueryMonitoringRequestObject) (oas.QueryMonitoringResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	if req.Body.Dataset != nil && *req.Body.Dataset != oas.AnalyticsDatasetUptime {
		return nil, errUptimeOnly
	}
	out, err := s.queryUptime(ctx, p.workspaceID, nil, *req.Body)
	if err != nil {
		return nil, err
	}
	if req.Params.Format != nil && *req.Params.Format == oas.MonitoringFormatCSV {
		dims := []string{}
		if req.Body.Dimensions != nil {
			for _, d := range *req.Body.Dimensions {
				dims = append(dims, string(d))
			}
		}
		metrics := []string{}
		for _, m := range req.Body.Metrics {
			metrics = append(metrics, string(m))
		}
		body := analyticsCSV(dims, metrics, out.Rows)
		name := `attachment; filename="uptime-` + out.Meta.From.Format(time.DateOnly) + `-` + out.Meta.To.Format(time.DateOnly) + `.csv"`
		return oas.QueryMonitoring200TextcsvResponse{Body: bytes.NewReader(body), ContentLength: int64(len(body)),
			Headers: oas.QueryMonitoring200ResponseHeaders{ContentDisposition: &name}}, nil
	}
	return oas.QueryMonitoring200JSONResponse{Body: out}, nil
}

// queryPropertyUptime answers queryAnalytics with dataset uptime over the property's checks.
func (s *Server) queryPropertyUptime(ctx context.Context, prop store.AnalyticsProperty, b oas.AnalyticsQuery, csv bool) (oas.QueryAnalyticsResponseObject, error) {
	if s.analytics.monitor == nil {
		return nil, errNotFound
	}
	out, err := s.queryUptime(ctx, prop.WorkspaceID, &prop, b)
	if err != nil {
		return nil, err
	}
	if csv {
		var dims, metrics []string
		if b.Dimensions != nil {
			for _, d := range *b.Dimensions {
				dims = append(dims, string(d))
			}
		}
		for _, m := range b.Metrics {
			metrics = append(metrics, string(m))
		}
		body := analyticsCSV(dims, metrics, out.Rows)
		name := `attachment; filename="uptime-` + out.Meta.From.Format(time.DateOnly) + `-` + out.Meta.To.Format(time.DateOnly) + `.csv"`
		return oas.QueryAnalytics200TextcsvResponse{Body: bytes.NewReader(body), ContentLength: int64(len(body)),
			Headers: oas.QueryAnalytics200ResponseHeaders{ContentDisposition: &name}}, nil
	}
	return oas.QueryAnalytics200JSONResponse{Body: out}, nil
}

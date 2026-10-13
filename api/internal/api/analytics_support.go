package api

import (
	"cmp"
	"context"
	"math"
	"net/http"
	"slices"
	"strings"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	supportMinViews     = 100
	insightMinCount     = 10
	insightTotalChange  = 0.2
	insightRowChange    = 0.5
	insightRows         = 3
	hotspotMinCalls     = 3
	hotspotRateMultiple = 2
)

var errNoLinkedInbox = problem(http.StatusUnprocessableEntity, "no_linked_inbox",
	"the support dataset needs a property linked to an inbox; link one in the property's settings")

// countMetrics are the metrics that are counts, which insights compare.
var countMetrics = []string{"visitors", "visits", "pageviews", "events", "conversions", "completions", "conversations", "feedback",
	"ratings_up", "ratings_down", "discussions"}

func checkSupportQuery(prop store.AnalyticsProperty, b oas.AnalyticsQuery) error {
	if prop.InboxID == nil {
		return errNoLinkedInbox
	}
	if b.GoalId != nil || b.Prop != nil || b.SegmentId != nil {
		return errValidation("the support dataset takes no goal_id, prop or segment_id")
	}
	places := 0
	if b.Dimensions != nil {
		for _, d := range *b.Dimensions {
			switch string(d) {
			case "page", "hostname":
				places++
			case "time:day", "time:week", "time:month":
			default:
				return errValidation("the support dataset breaks down by page or hostname and time:day, time:week or time:month")
			}
		}
	}
	if places > 1 {
		return errValidation("the support dataset breaks down by page or by hostname, not both")
	}
	if b.Filters != nil {
		for _, f := range *b.Filters {
			if !slices.Contains(analytics.SupportDims, string(f.Dimension)) {
				return errValidation("the support dataset filters on page and hostname only")
			}
		}
	}
	return nil
}

func checkDatasetMetric(support bool, m string) error {
	own := slices.Contains(analytics.SupportMetrics, m)
	switch {
	case own && !support:
		return errValidation("metric " + m + " needs dataset support")
	case support && !own && m != "visitors" && m != "pageviews":
		return errValidation("the support dataset offers visitors, pageviews and its own metrics, not " + m)
	}
	return nil
}

// supportQuery answers a support query: the inbox's support per page next to the page's traffic.
func (s *Server) supportQuery(ctx context.Context, prop store.AnalyticsProperty, q analytics.Query) (analytics.Result, error) {
	if prop.InboxID == nil {
		return analytics.Result{}, errNoLinkedInbox
	}
	src := analytics.SupportSource{WorkspaceID: prop.WorkspaceID, PropertyID: prop.ID, InboxID: *prop.InboxID, Timezone: prop.Timezone,
		Rules: pathRules(prop.PathRules)}
	got, served, err := (&analytics.PGStore{Pool: s.st.Pool}).Support(ctx, src, q.From, q.To)
	if err != nil {
		return analytics.Result{}, err
	}
	grain, place := "", ""
	for _, d := range q.Dimensions {
		if g, ok := strings.CutPrefix(d, "time:"); ok {
			grain = g
		} else {
			place = d
		}
	}
	var res analytics.Result
	byKey := map[string]int{}
	add := func(dims []string, sums analytics.Sums, create bool) {
		k := strings.Join(dims, "\x1f")
		i, ok := byKey[k]
		if !ok {
			if !create {
				return
			}
			i = len(res.Rows)
			byKey[k] = i
			res.Rows = append(res.Rows, analytics.Row{Dims: dims})
		}
		res.Rows[i].Sums.Add(sums)
	}
rows:
	for _, r := range got {
		for _, f := range q.Filters {
			v, path := r.Host, false
			if f.Dimension == "page" {
				v, path = r.Path, true
			}
			if !analytics.MatchValue(f, v, path) {
				continue rows
			}
		}
		res.Totals.Add(r.Sums)
		dims := make([]string, len(q.Dimensions))
		for i, d := range q.Dimensions {
			switch d {
			case "page":
				dims[i] = r.Path
			case "hostname":
				dims[i] = r.Host
			default:
				dims[i] = analytics.Bucket(grain, r.Day)
			}
		}
		if len(dims) > 0 {
			add(dims, r.Sums, true)
		}
	}
	res.ServedFrom = served
	if !needsViews(q) {
		return res, nil
	}
	tq := q
	tq.Dataset, tq.Metrics, tq.SortBy = "", []string{"visitors", "pageviews"}, nil
	traffic, err := s.analytics.store.Query(ctx, tq)
	if err != nil {
		return res, err
	}
	views := func(t analytics.Sums) analytics.Sums { return analytics.Sums{Visitors: t.Visitors, Views: t.Views} }
	res.Totals.Add(views(traffic.Totals))
	for _, r := range traffic.Rows {
		add(r.Dims, views(r.Sums), place == "")
	}
	if traffic.ServedFrom != served {
		res.ServedFrom = analytics.ServedMixed
	}
	return res, nil
}

func needsViews(q analytics.Query) bool {
	for _, m := range slices.Concat(q.Metrics, q.SortBy) {
		if m == "visitors" || m == "pageviews" || m == "support_rate" {
			return true
		}
	}
	return false
}

// sortSupportRows puts pages and hosts with fewer than supportMinViews page views after the others,
// keeping the order within each part and the time buckets together.
func sortSupportRows(rows []analytics.Row, dims []string) {
	if len(dims) == 0 || timeOnly(dims) {
		return
	}
	ti := timeIndex(dims)
	slices.SortStableFunc(rows, func(a, b analytics.Row) int {
		if ti >= 0 {
			if c := strings.Compare(a.Dims[ti], b.Dims[ti]); c != 0 {
				return c
			}
		}
		low := func(r analytics.Row) int {
			if r.Sums.Views < supportMinViews {
				return 1
			}
			return 0
		}
		return cmp.Compare(low(a), low(b))
	})
}

// analyticsInsights applies the fixed insight rules to an answer.
func analyticsInsights(q analytics.Query, ans analyticsAnswer, order []analytics.Order) []oas.AnalyticsInsight {
	out := []oas.AnalyticsInsight{}
	res := ans.result
	if c := ans.comparison; c != nil {
		for _, m := range q.Metrics {
			if !slices.Contains(countMetrics, m) {
				continue
			}
			cur, prev := res.Totals.Metric(m), c.Totals.Metric(m)
			if max(cur, prev) < insightMinCount || (prev > 0 && math.Abs(cur-prev)/prev < insightTotalChange) || (prev == 0 && cur == 0) {
				continue
			}
			out = append(out, insight(oas.AnalyticsInsightMetricChange, m, "", "", cur, prev))
		}
	}
	place := -1
	for i, d := range q.Dimensions {
		if strings.HasPrefix(d, "time:") {
			return out
		}
		place = i
	}
	if place < 0 || len(q.Dimensions) != 1 {
		return out
	}
	dim := q.Dimensions[0]
	if c := ans.comparison; c != nil {
		metric := ""
		for _, o := range append(slices.Clone(order), analytics.Order{Metric: q.Metrics[0]}) {
			if slices.Contains(countMetrics, o.Metric) {
				metric = o.Metric
				break
			}
		}
		if metric != "" {
			before := map[string]float64{}
			for _, r := range c.Rows {
				before[r.Dims[0]] = r.Sums.Metric(metric)
			}
			var moved, fresh []oas.AnalyticsInsight
			for _, r := range res.Rows {
				v := r.Dims[0]
				if v == analytics.OtherValue {
					continue
				}
				cur, prev := r.Sums.Metric(metric), before[v]
				switch {
				case prev == 0 && cur >= insightMinCount:
					fresh = append(fresh, insight(oas.AnalyticsInsightNewValue, metric, dim, v, cur, 0))
				case prev > 0 && max(cur, prev) >= insightMinCount && math.Abs(cur-prev)/prev >= insightRowChange:
					moved = append(moved, insight(oas.AnalyticsInsightRowChange, metric, dim, v, cur, prev))
				}
			}
			slices.SortStableFunc(moved, func(a, b oas.AnalyticsInsight) int {
				return cmp.Compare(math.Abs(b.Current-*b.Previous), math.Abs(a.Current-*a.Previous))
			})
			slices.SortStableFunc(fresh, func(a, b oas.AnalyticsInsight) int { return cmp.Compare(b.Current, a.Current) })
			out = append(append(out, moved[:min(len(moved), insightRows)]...), fresh[:min(len(fresh), insightRows)]...)
		}
	}
	if q.Dataset == analytics.DatasetSupport && dim == "page" {
		overall := res.Totals.Metric("support_rate")
		var hot []oas.AnalyticsInsight
		for _, r := range res.Rows {
			rate := r.Sums.Metric("support_rate")
			if overall > 0 && r.Sums.Conversations >= hotspotMinCalls && rate >= hotspotRateMultiple*overall {
				hot = append(hot, insight(oas.AnalyticsInsightSupportHotspot, "support_rate", dim, r.Dims[0], rate, overall))
			}
		}
		slices.SortStableFunc(hot, func(a, b oas.AnalyticsInsight) int { return cmp.Compare(b.Current, a.Current) })
		out = append(out, hot[:min(len(hot), insightRows)]...)
	}
	return out
}

func insight(kind oas.AnalyticsInsightKind, metric, dim, value string, cur, prev float64) oas.AnalyticsInsight {
	out := oas.AnalyticsInsight{Kind: kind, Metric: oas.AnalyticsMetric(metric), Current: cur, Previous: &prev}
	if dim != "" {
		out.Dimension, out.Value = &dim, &value
	}
	if prev > 0 {
		out.Change = new(math.Round((cur-prev)/prev*1000) / 10)
	}
	return out
}

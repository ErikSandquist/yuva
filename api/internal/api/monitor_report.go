package api

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/productdevbook/yuva/api/internal/mail"
	"github.com/productdevbook/yuva/api/internal/monitor"
	"github.com/productdevbook/yuva/api/internal/store"
)

// reportMinRouteRequests keeps rarely called routes out of the slowest routes.
const reportMinRouteRequests = 20

type apiReportData struct {
	Property, From, To                   string
	Monthly                              bool
	Requests, ErrorRate, ClientErrorRate reportMetric
	P95                                  reportMetric
	Slowest, Failing, Checks, Alerts     []reportRow
	AlertCount                           string
	URL, SettingsURL                     string
}

// renderMonitorReport builds an API property's report of the local days from to: requests, error
// rates and p95 against the period before, the slowest and most failing routes, the uptime of its
// checks and the alerts that fired.
func (s *Server) renderMonitorReport(ctx context.Context, prop store.AnalyticsProperty, freq string, from, to time.Time, locale string) (mail.Message, error) {
	m := s.analytics.monitor
	zone := monitorZone(prop.Timezone, prop.TimezoneHistory)
	start, end := zone.DayStart(from), zone.DayStart(to.AddDate(0, 0, 1))
	prevFrom := from.AddDate(0, 0, -(int(to.Sub(from).Hours()/24) + 1))
	if freq == reportMonthly {
		prevFrom = from.AddDate(0, -1, 0)
	}
	base := monitor.Query{WorkspaceID: prop.WorkspaceID, PropertyID: prop.ID, Zone: zone, Start: start, End: end, Days: true, Now: s.now(),
		ApdexMS: int64(prop.ApdexMs), MinuteFrom: monitor.LocalDay(s.now(), time.UTC).AddDate(0, 0, 1-m.settings.MinuteDays)}
	totals := base
	totals.Metrics = []string{"requests", "error_rate", "client_error_rate", "p95"}
	cur, err := s.runMonitorQuery(ctx, prop, totals)
	if err != nil {
		return mail.Message{}, err
	}
	prevQ := totals
	prevQ.Start, prevQ.End = zone.DayStart(prevFrom), start
	prev, err := s.runMonitorQuery(ctx, prop, prevQ)
	if err != nil {
		return mail.Message{}, err
	}
	metric := func(name string, format func(float64) string) reportMetric {
		c, p := cur.Totals.Metric(name, 0), prev.Totals.Metric(name, 0)
		return reportMetric{Value: format(c), Change: reportChange(c, p, locale)}
	}
	pct := func(v float64) string { return reportPercentDecimal(v, locale) }
	msText := func(v float64) string { return reportNumber(v, locale) + " ms" }
	d := apiReportData{Property: prop.Name, From: from.Format(time.DateOnly), To: to.Format(time.DateOnly), Monthly: freq == reportMonthly,
		Requests: metric("requests", func(v float64) string { return reportNumber(v, locale) }), ErrorRate: metric("error_rate", pct),
		ClientErrorRate: metric("client_error_rate", pct), P95: metric("p95", msText), Slowest: []reportRow{}, Failing: []reportRow{}, Checks: []reportRow{},
		Alerts: []reportRow{}, URL: s.panelPropertyURL(prop.WorkspaceID, prop.ID, ""), SettingsURL: s.panelPropertyURL(prop.WorkspaceID, prop.ID, "settings")}
	byRoute := base
	byRoute.Metrics, byRoute.Dimensions = []string{"requests", "errors", "error_rate", "p95"}, []string{"route"}
	routes, err := s.runMonitorQuery(ctx, prop, byRoute)
	if err != nil {
		return mail.Message{}, err
	}
	rows := slices.DeleteFunc(slices.Clone(routes.Rows), func(r monitor.Row) bool {
		return r.Dims[0] == monitor.Unmatched || r.Dims[0] == monitor.OtherValue
	})
	slices.SortStableFunc(rows, func(a, b monitor.Row) int { return cmp.Compare(b.Agg.Metric("p95", 0), a.Agg.Metric("p95", 0)) })
	for _, r := range rows {
		if len(d.Slowest) == reportTopRows {
			break
		}
		if r.Agg.Requests >= reportMinRouteRequests {
			d.Slowest = append(d.Slowest, reportRow{Name: r.Dims[0], Value: msText(r.Agg.Metric("p95", 0))})
		}
	}
	slices.SortStableFunc(rows, func(a, b monitor.Row) int { return cmp.Compare(float64(b.Agg.Errors), float64(a.Agg.Errors)) })
	for _, r := range rows {
		if len(d.Failing) == reportTopRows || r.Agg.Errors == 0 {
			break
		}
		d.Failing = append(d.Failing, reportRow{Name: r.Dims[0], Value: reportNumber(float64(r.Agg.Errors), locale) + " · " + pct(r.Agg.Metric("error_rate", 0))})
	}
	checks, err := s.st.ListMonitorChecks(ctx, store.ListMonitorChecksParams{WorkspaceID: prop.WorkspaceID, PropertyID: &prop.ID})
	if err != nil {
		return mail.Message{}, err
	}
	for _, c := range checks {
		res, err := s.runUptimeQuery(ctx, monitor.UptimeQuery{WorkspaceID: prop.WorkspaceID, Checks: []monitor.UptimeCheck{{ID: c.ID, FoldedUntil: c.FoldedUntil}},
			Start: start, End: end, Metrics: []string{"uptime"}, Now: s.now(), ResultsFrom: monitor.LocalDay(s.now(), time.UTC).AddDate(0, 0, 1-m.settings.CheckDays)})
		if err != nil {
			return mail.Message{}, err
		}
		if res.Totals.Runs > 0 {
			d.Checks = append(d.Checks, reportRow{Name: c.Name, Value: reportPercentDecimal(res.Totals.Metric("uptime"), locale)})
		}
	}
	alerts, err := s.st.ListMonitorAlertsFired(ctx, store.ListMonitorAlertsFiredParams{WorkspaceID: prop.WorkspaceID, PropertyID: &prop.ID, FromAt: start, ToAt: end})
	if err != nil {
		return mail.Message{}, err
	}
	d.AlertCount = reportNumber(float64(len(alerts)), locale)
	loc := zone.Current()
	for _, a := range alerts[max(0, len(alerts)-reportTopRows):] {
		d.Alerts = append(d.Alerts, reportRow{Name: a.RuleName, Value: a.FiredAt.In(loc).Format("2006-01-02 15:04")})
	}
	return mail.Render("monitor_report", locale, d)
}

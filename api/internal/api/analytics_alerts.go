package api

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/mail"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	alertWindow        = 2 * time.Hour
	alertBaselineDays  = 7
	alertLongDays      = 28
	alertMinViews      = 20
	alertSpikeFactor   = 3
	alertDropShare     = 0.5
	alertMinGoal       = 10
	alertMinCalls      = 5
	alertRateFactor    = 2
	alertMinReferrals  = 10
	alertDailyRuleHour = 6
)

type alertFinding struct {
	kind, key, metric, text string
	day                     time.Time
	hoursFrom, hoursTo      *time.Time
	current, baseline       float64
	goalID                  *uuid.UUID
	goalName, referrer      *string
}

// CheckAnalyticsAlerts applies the alert rules to every property with alert subscribers and fires
// what they find; it returns how many alerts fired.
func (s *Server) CheckAnalyticsAlerts(ctx context.Context) (int, error) {
	if s.analytics == nil {
		return 0, nil
	}
	ids, err := s.st.ListAnalyticsReportWorkspaceIDs(ctx)
	if err != nil {
		return 0, err
	}
	fired := 0
	for _, ws := range ids {
		props, err := s.st.ListAnalyticsAlertProperties(ctx, ws)
		if err != nil {
			return fired, err
		}
		for _, row := range props {
			prop, err := s.st.GetAnalyticsProperty(ctx, store.GetAnalyticsPropertyParams{WorkspaceID: ws, ID: row.ID})
			if store.IsNotFound(err) {
				continue
			}
			if err != nil {
				return fired, err
			}
			found, err := s.alertFindings(ctx, prop, s.now())
			if err != nil {
				s.log.WarnContext(ctx, "analytics alerts", slog.String("workspace_id", ws.String()), slog.String("property_id", prop.ID.String()),
					slog.Any("error", err))
				continue
			}
			for _, f := range found {
				ok, err := s.fireAlert(ctx, prop, f)
				if err != nil {
					return fired, err
				}
				if ok {
					fired++
				}
			}
		}
	}
	return fired, nil
}

func (s *Server) alertQuery(ctx context.Context, prop store.AnalyticsProperty, from, to time.Time, metrics, dims []string, goal *analytics.Goal, dataset string) (analytics.Result, error) {
	q := analytics.Query{WorkspaceID: prop.WorkspaceID, PropertyID: prop.ID, Timezone: prop.Timezone, From: from, To: to, RawFrom: s.rawFrom(),
		Metrics: metrics, Dimensions: dims, Goal: goal, Dataset: dataset}
	ans, err := s.runAnalyticsQuery(ctx, prop, q, nil)
	return ans.result, err
}

// alertFindings applies the hourly rules (spike, drop) to the last two finished hours and, in the
// property's sixth hour of the day, the daily rules to yesterday.
func (s *Server) alertFindings(ctx context.Context, prop store.AnalyticsProperty, now time.Time) ([]alertFinding, error) {
	loc := propertyLocation(prop.Timezone)
	lt := now.In(loc)
	end := time.Date(lt.Year(), lt.Month(), lt.Day(), lt.Hour(), 0, 0, 0, loc)
	start := end.Add(-alertWindow)
	today := analytics.LocalDay(now, loc)
	first := analytics.LocalDay(start, loc).AddDate(0, 0, -alertBaselineDays)
	if first.Before(s.rawFrom()) {
		first = s.rawFrom()
	}
	var out []alertFinding
	hourly, err := s.alertQuery(ctx, prop, first, today, []string{"pageviews"}, []string{"time:hour"}, nil, "")
	if err != nil {
		return nil, err
	}
	views := map[string]float64{}
	for _, r := range hourly.Rows {
		views[r.Dims[0]] = r.Sums.Metric("pageviews")
	}
	window := func(back int) float64 {
		sum := 0.0
		for h := start; h.Before(end); h = h.Add(time.Hour) {
			t := h.In(loc).AddDate(0, 0, -back)
			sum += views[t.Format("2006-01-02T15:00")]
		}
		return sum
	}
	current, baseline := window(0), 0.0
	for d := 1; d <= alertBaselineDays; d++ {
		baseline += window(d)
	}
	baseline /= alertBaselineDays
	day := analytics.LocalDay(end.Add(-time.Minute), loc)
	hours := start.In(loc).Format("15:04") + "–" + end.In(loc).Format("15:04")
	if baseline >= alertMinViews {
		from, to := start, end
		switch {
		case current >= alertSpikeFactor*baseline:
			out = append(out, alertFinding{kind: string(oas.AnalyticsAlertSpike), key: day.Format(time.DateOnly), metric: "pageviews", day: day,
				hoursFrom: &from, hoursTo: &to, current: current, baseline: baseline,
				text: fmt.Sprintf("Page views %s were %s against %s on the days before", hours, num(current), num(baseline))})
		case current <= alertDropShare*baseline:
			out = append(out, alertFinding{kind: string(oas.AnalyticsAlertDrop), key: day.Format(time.DateOnly), metric: "pageviews", day: day,
				hoursFrom: &from, hoursTo: &to, current: current, baseline: baseline,
				text: fmt.Sprintf("Page views %s fell to %s against %s on the days before", hours, num(current), num(baseline))})
		}
	}
	if lt.Hour() != alertDailyRuleHour {
		return out, nil
	}
	yesterday := today.AddDate(0, 0, -1)
	ykey := yesterday.Format(time.DateOnly)
	goals, err := s.st.ListAnalyticsGoals(ctx, store.ListAnalyticsGoalsParams{WorkspaceID: prop.WorkspaceID, PropertyID: prop.ID})
	if err != nil {
		return nil, err
	}
	for _, g := range goals {
		res, err := s.alertQuery(ctx, prop, yesterday.AddDate(0, 0, -alertBaselineDays), yesterday, []string{"conversions"}, []string{"time:day"}, goalDef(g), "")
		if err != nil {
			return nil, err
		}
		cur, base := 0.0, 0.0
		for _, r := range res.Rows {
			if r.Dims[0] == ykey {
				cur = r.Sums.Metric("conversions")
			} else {
				base += r.Sums.Metric("conversions")
			}
		}
		base /= alertBaselineDays
		if base >= alertMinGoal && cur <= alertDropShare*base {
			id, name := g.ID, g.Name
			out = append(out, alertFinding{kind: string(oas.AnalyticsAlertGoalDrop), key: ykey + "|" + g.ID.String(), metric: "conversions", day: yesterday,
				current: cur, baseline: base, goalID: &id, goalName: &name,
				text: fmt.Sprintf("Goal %s: %s conversions yesterday against %s a day before", name, num(cur), num(base))})
		}
	}
	if prop.InboxID != nil {
		y, err := s.alertQuery(ctx, prop, yesterday, yesterday, []string{"conversations", "pageviews"}, nil, nil, analytics.DatasetSupport)
		if err != nil {
			return nil, err
		}
		b, err := s.alertQuery(ctx, prop, yesterday.AddDate(0, 0, -alertLongDays), yesterday.AddDate(0, 0, -1), []string{"conversations", "pageviews"}, nil, nil,
			analytics.DatasetSupport)
		if err != nil {
			return nil, err
		}
		rate, base := y.Totals.Metric("support_rate"), b.Totals.Metric("support_rate")
		if y.Totals.Conversations >= alertMinCalls && y.Totals.Views > 0 && rate >= alertRateFactor*base {
			out = append(out, alertFinding{kind: string(oas.AnalyticsAlertSupportLoad), key: ykey, metric: "support_rate", day: yesterday,
				current: rate, baseline: base,
				text: fmt.Sprintf("%s conversations per 1,000 page views yesterday against %s on the 28 days before", rateText(rate), rateText(base))})
		}
	}
	y, err := s.alertQuery(ctx, prop, yesterday, yesterday, []string{"visits"}, []string{"referrer"}, nil, "")
	if err != nil {
		return nil, err
	}
	var fresh []analytics.Row
	for _, r := range y.Rows {
		if r.Dims[0] != "" && r.Dims[0] != analytics.OtherValue && r.Sums.Visits >= alertMinReferrals {
			fresh = append(fresh, r)
		}
	}
	if len(fresh) > 0 {
		b, err := s.alertQuery(ctx, prop, yesterday.AddDate(0, 0, -alertLongDays), yesterday.AddDate(0, 0, -1), []string{"visits"}, []string{"referrer"}, nil, "")
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for _, r := range b.Rows {
			seen[r.Dims[0]] = r.Sums.Visits > 0
		}
		for _, r := range fresh {
			if seen[r.Dims[0]] {
				continue
			}
			host := r.Dims[0]
			out = append(out, alertFinding{kind: string(oas.AnalyticsAlertNewReferrer), key: "referrer:" + host, metric: "visits", day: yesterday,
				current: float64(r.Sums.Visits), referrer: &host,
				text: fmt.Sprintf("New referrer %s: %s visits yesterday", host, num(float64(r.Sums.Visits)))})
		}
	}
	return out, nil
}

func num(v float64) string { return formatCount(int64(v + 0.5)) }

func rateText(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

// fireAlert stores an alert unless its key fired before, annotates the day, writes the
// analytics.alert event (and so the webhooks) and mails the subscribers of its kind.
func (s *Server) fireAlert(ctx context.Context, prop store.AnalyticsProperty, f alertFinding) (bool, error) {
	ws, now := prop.WorkspaceID, s.now()
	var a store.AnalyticsAlert
	fired := false
	err := s.inTx(ctx, ws, func(q *store.Queries, events *eventBatch) error {
		var err error
		a, err = q.CreateAnalyticsAlert(ctx, store.CreateAnalyticsAlertParams{
			ID: newID(), WorkspaceID: ws, PropertyID: prop.ID, Kind: f.kind, Key: f.key, Day: f.day, HoursFrom: f.hoursFrom, HoursTo: f.hoursTo,
			Metric: f.metric, Current: f.current, Baseline: f.baseline, GoalID: f.goalID, GoalName: f.goalName, Referrer: f.referrer,
			Text: analytics.Clean(f.text, 300), Now: now,
		})
		if store.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		ann, err := q.CreateAnalyticsAlertAnnotation(ctx, store.CreateAnalyticsAlertAnnotationParams{ID: newID(), WorkspaceID: ws, PropertyID: prop.ID,
			Day: f.day, Text: a.Text, AlertID: &a.ID, Now: now})
		if err != nil {
			return err
		}
		if a, err = q.SetAnalyticsAlertAnnotation(ctx, store.SetAnalyticsAlertAnnotationParams{WorkspaceID: ws, ID: a.ID, AnnotationID: &ann.ID}); err != nil {
			return err
		}
		events.add(eventAnalyticsAlert, prop.InboxID, nil, oas.AnalyticsAlertEventData{Alert: alertBody(a),
			Property: oas.AnalyticsPropertyRef{Id: prop.ID, Name: prop.Name}})
		fired = true
		return nil
	})
	if err != nil || !fired {
		return false, err
	}
	subs, err := s.st.ListAnalyticsSubscriberAddresses(ctx, store.ListAnalyticsSubscriberAddressesParams{WorkspaceID: ws, PropertyID: prop.ID, Kind: f.kind})
	if err != nil {
		return true, err
	}
	for _, sub := range subs {
		msg, err := mail.Render("analytics_alert", sub.Locale, alertMailData(s, prop, a, sub.Locale))
		if err != nil {
			return true, err
		}
		msg.To = sub.Email
		if err := s.mailer.Send(ctx, msg); err != nil {
			s.log.WarnContext(ctx, "analytics alert mail", slog.String("workspace_id", ws.String()), slog.Any("error", err))
		}
	}
	return true, nil
}

func alertMailData(s *Server, prop store.AnalyticsProperty, a store.AnalyticsAlert, locale string) map[string]any {
	loc := propertyLocation(prop.Timezone)
	d := map[string]any{
		"Kind": a.Kind, "Property": prop.Name, "Day": a.Day.Format(time.DateOnly),
		"Current": reportNumber(a.Current, locale), "Baseline": reportNumber(a.Baseline, locale),
		"URL": s.panelPropertyURL(prop.WorkspaceID, prop.ID, ""), "SettingsURL": s.panelPropertyURL(prop.WorkspaceID, prop.ID, "settings"),
		"Goal": deref(a.GoalName), "Referrer": deref(a.Referrer),
	}
	if a.HoursFrom != nil && a.HoursTo != nil {
		d["HourFrom"], d["HourTo"] = a.HoursFrom.In(loc).Format("15:04"), a.HoursTo.In(loc).Format("15:04")
	}
	if a.Kind == string(oas.AnalyticsAlertSupportLoad) {
		d["Current"], d["Baseline"] = reportDecimal(a.Current, locale), reportDecimal(a.Baseline, locale)
	}
	return d
}

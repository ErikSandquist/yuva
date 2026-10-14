package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// sendBatch sends n records of route at at, the first errors of them answering 500.
func sendBatch(c *client, id, route string, at time.Time, n, errors int) {
	c.h.t.Helper()
	recs := make([]map[string]any, 0, n)
	for i := range n {
		status := 200
		if i < errors {
			status = 500
		}
		recs = append(recs, map[string]any{"method": "GET", "route": route, "status": status, "duration_us": 1000 + i,
			"time": at.Add(time.Duration(i%50) * time.Second).Format(time.RFC3339Nano)})
		if len(recs) == 1000 {
			sendRecords(c, http.StatusAccepted, id, recs...)
			recs = recs[:0]
		}
	}
	if len(recs) > 0 {
		sendRecords(c, http.StatusAccepted, id, recs...)
	}
}

func alertsOf(c *client, query string) []map[string]any {
	c.h.t.Helper()
	var out []map[string]any
	for _, a := range c.expect(http.StatusOK, "GET", "/v1/monitoring/alerts"+query, nil).body["items"].([]any) {
		out = append(out, a.(map[string]any))
	}
	return out
}

func TestMonitorErrorRateAlertFiresAndResolvesOnce(t *testing.T) {
	h, _ := monitorHarness(t, false, nil)
	tm := newTeam(t, h)
	owner := personEmail(tm.owner)
	t0 := time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC)
	h.clock.Set(t0)
	id := createAPIProperty(tm.owner, map[string]any{"name": "Orders API"})
	rules := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/alert-rules?property_id="+id, nil).body["items"].([]any)
	if len(rules) != 1 {
		t.Fatalf("default rules %v", rules)
	}
	def := rules[0].(map[string]any)
	if def["kind"] != "error_rate" || def["threshold"].(float64) != 5 || def["window_minutes"].(float64) != 5 || def["min_requests"].(float64) != 20 ||
		len(def["recipients"].([]any)) != 1 {
		t.Fatalf("default rule %v", def)
	}
	key := tm.apiKey(h)

	h.clock.Set(t0.Add(2 * time.Minute))
	sendBatch(key, id, "/orders/{id}", t0.Add(time.Minute), 15, 15)
	h.flushMonitor()
	if h.monitorAlerts(tm.ws, t0.Add(5*time.Minute+40*time.Second)) != 0 {
		t.Fatal("fewer than min_requests fired")
	}

	h.clock.Set(t0.Add(7 * time.Minute))
	sendBatch(key, id, "/orders/{id}", t0.Add(6*time.Minute), 100, 10)
	h.flushMonitor()
	at := t0.Add(10*time.Minute + 40*time.Second)
	if n := h.monitorAlerts(tm.ws, at); n != 1 {
		t.Fatalf("10%% errors: %d changes", n)
	}
	if n := h.monitorAlerts(tm.ws, at.Add(5*time.Second)); n != 0 {
		t.Fatalf("fired again: %d", n)
	}
	open := alertsOf(tm.owner, "?state=firing&property_id="+id)
	if len(open) != 1 || open[0]["value"].(float64) < 9 || open[0]["fired_at"] != t0.Add(10*time.Minute).Format(time.RFC3339) {
		t.Fatalf("open alerts %v", open)
	}
	if m := h.alertMails(owner); len(m) != 1 || !strings.HasPrefix(m[0], "Alert: 5xx rate above 5%") {
		t.Fatalf("mails %v", m)
	}

	h.clock.Set(t0.Add(12 * time.Minute))
	sendBatch(key, id, "/orders/{id}", t0.Add(11*time.Minute), 1000, 48)
	h.flushMonitor()
	if n := h.monitorAlerts(tm.ws, t0.Add(16*time.Minute+40*time.Second)); n != 0 {
		t.Fatalf("4.8%% is past the margin and resolved: %d", n)
	}

	h.clock.Set(t0.Add(17 * time.Minute))
	sendBatch(key, id, "/orders/{id}", t0.Add(16*time.Minute), 100, 2)
	h.flushMonitor()
	if n := h.monitorAlerts(tm.ws, t0.Add(20*time.Minute+40*time.Second)); n != 1 {
		t.Fatalf("2%% errors over a whole window: %d changes", n)
	}
	if n := h.monitorAlerts(tm.ws, t0.Add(26*time.Minute+40*time.Second)); n != 0 {
		t.Fatalf("after resolving: %d", n)
	}
	all := alertsOf(tm.owner, "?property_id="+id)
	if len(all) != 1 || all[0]["state"] != "resolved" || all[0]["resolved_value"].(float64) != 2 {
		t.Fatalf("alerts %v", all)
	}
	if m := h.alertMails(owner); len(m) != 2 || !strings.HasPrefix(m[1], "Resolved: ") {
		t.Fatalf("mails %v", m)
	}
	if n := h.count("SELECT count(*) FROM events WHERE workspace_id = $1 AND type = 'monitor.alert'", tm.ws); n != 2 {
		t.Fatalf("events %d", n)
	}
	ann := query(tm.owner, id, map[string]any{"dataset": "api", "metrics": []string{"requests"}, "range": map[string]any{"from": "2026-04-07", "to": "2026-04-07"}})
	if ann.status != http.StatusOK {
		t.Fatal(ann.raw)
	}
	notes := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+id+"/annotations", nil).body["items"].([]any)
	alertsNoted := 0
	for _, n := range notes {
		if n.(map[string]any)["kind"] == "alert" && n.(map[string]any)["at"] != nil {
			alertsNoted++
		}
	}
	if alertsNoted != 2 {
		t.Fatalf("annotations %v", notes)
	}

	webhookAlert := tm.owner.expect(http.StatusOK, "GET", "/v1/events?after=0", nil)
	if !strings.Contains(string(webhookAlert.raw), `"type":"monitor.alert"`) || !strings.Contains(string(webhookAlert.raw), `"state":"resolved"`) {
		t.Fatalf("event feed %s", webhookAlert.raw)
	}
}

func TestMonitorTrafficRulesAgainstBaselines(t *testing.T) {
	h, _ := monitorHarness(t, false, nil)
	tm := newTeam(t, h)
	tEnd := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	h.clock.Set(tEnd.Add(-8 * 24 * time.Hour))
	id := createAPIProperty(tm.owner, map[string]any{"name": "Traffic API"})
	key := tm.apiKey(h)
	for _, r := range []string{"/a", "/b", "/c", "/d"} {
		h.clock.Set(tEnd.Add(-8 * 24 * time.Hour))
		sendBatch(key, id, r, h.clock.Now().Add(-time.Minute), 1, 0)
	}
	h.flushMonitor()
	rule := func(body map[string]any) string {
		body["property_id"] = id
		return tm.owner.expect(http.StatusCreated, "POST", "/v1/monitoring/alert-rules", body).str("id")
	}
	spike := rule(map[string]any{"kind": "spike", "route": "/a"})
	drop := rule(map[string]any{"kind": "drop", "route": "/b"})
	quiet := rule(map[string]any{"kind": "spike", "route": "/d"})
	silent := rule(map[string]any{"kind": "silent"})
	tm.owner.expect(http.StatusBadRequest, "POST", "/v1/monitoring/alert-rules", map[string]any{"kind": "spike", "property_id": id, "threshold": 3})
	tm.owner.expect(http.StatusBadRequest, "POST", "/v1/monitoring/alert-rules", map[string]any{"kind": "silent", "property_id": id, "route": "/nope"})
	tm.owner.expect(http.StatusBadRequest, "POST", "/v1/monitoring/alert-rules", map[string]any{"kind": "p95", "property_id": id})

	for d := 7; d >= 1; d-- {
		day := tEnd.Add(-time.Duration(d) * 24 * time.Hour)
		h.clock.Set(day)
		sendBatch(key, id, "/a", day.Add(-10*time.Minute), 60, 0)
		sendBatch(key, id, "/b", day.Add(-10*time.Minute), 60, 0)
		sendBatch(key, id, "/d", day.Add(-10*time.Minute), 40, 0)
		h.flushMonitor()
	}
	h.clock.Set(tEnd)
	var steady []map[string]any
	for m := range 24 * 60 {
		steady = append(steady, map[string]any{"method": "GET", "route": "/c", "status": 200, "duration_us": 900,
			"time": tEnd.Add(-15*time.Minute - 24*time.Hour + time.Duration(m)*time.Minute).Format(time.RFC3339)})
	}
	sendRecords(key, http.StatusAccepted, id, steady[:1000]...)
	sendRecords(key, http.StatusAccepted, id, steady[1000:]...)
	sendBatch(key, id, "/c", tEnd.Add(-16*time.Minute), 30, 0)
	sendBatch(key, id, "/a", tEnd.Add(-10*time.Minute), 200, 0)
	sendBatch(key, id, "/b", tEnd.Add(-10*time.Minute), 20, 0)
	sendBatch(key, id, "/d", tEnd.Add(-10*time.Minute), 400, 0)
	h.flushMonitor()
	at := tEnd.Add(40 * time.Second)
	if n := h.monitorAlerts(tm.ws, at); n != 3 {
		t.Fatalf("changes %d, want spike, drop and one silent route: %v", n, alertsOf(tm.owner, "?state=firing"))
	}
	byRule := map[string]map[string]any{}
	for _, a := range alertsOf(tm.owner, "?state=firing&property_id="+id) {
		byRule[a["rule_id"].(string)] = a
	}
	if a := byRule[spike]; a == nil || a["value"].(float64) != 200 || a["baseline"].(float64) != 60 {
		t.Fatalf("spike %v", a)
	}
	if a := byRule[drop]; a == nil || a["value"].(float64) != 20 || a["baseline"].(float64) != 60 {
		t.Fatalf("drop %v", a)
	}
	if byRule[quiet] != nil {
		t.Fatalf("a baseline below 50 fired: %v", byRule[quiet])
	}
	if a := byRule[silent]; a == nil || a["key"] != "/c" || a["route"] != "/c" {
		t.Fatalf("silent %v", byRule)
	}
	if n := h.monitorAlerts(tm.ws, at.Add(10*time.Second)); n != 0 {
		t.Fatalf("fired again: %d", n)
	}
	h.clock.Set(tEnd.Add(time.Minute))
	sendBatch(key, id, "/c", tEnd.Add(time.Minute), 1, 0)
	h.flushMonitor()
	if n := h.monitorAlerts(tm.ws, tEnd.Add(2*time.Minute+40*time.Second)); n != 1 {
		t.Fatalf("the silent route was called again: %d changes", n)
	}
	for _, a := range alertsOf(tm.owner, "?rule_id="+silent) {
		if a["state"] != "resolved" {
			t.Fatalf("silent alert %v", a)
		}
	}
	tm.owner.expect(http.StatusOK, "PATCH", "/v1/monitoring/alert-rules/"+spike, map[string]any{"enabled": false})
	if a := alertsOf(tm.owner, "?rule_id="+spike); len(a) != 1 || a[0]["state"] != "resolved" {
		t.Fatalf("a disabled rule's alerts: %v", a)
	}
}

func TestMonitorMCPToolsAnswerLikeTheQuery(t *testing.T) {
	h, _ := monitorHarness(t, false, nil)
	tm := newTeam(t, h)
	now := time.Date(2026, 4, 25, 15, 30, 0, 0, time.UTC)
	h.clock.Set(now)
	id := createAPIProperty(tm.owner, map[string]any{"name": "Tools API"})
	key := tm.apiKey(h)
	for i, r := range []string{"/slow", "/fast", "/broken"} {
		recs := []map[string]any{}
		for j := range 40 {
			status := 200
			if r == "/broken" && j%4 == 0 {
				status = 503
			}
			recs = append(recs, map[string]any{"method": "GET", "route": r, "status": status, "duration_us": (i + 1) * 1000 * (3 - i) * (j + 1),
				"consumer": "partner", "time": now.Add(-time.Duration(j) * time.Minute).Format(time.RFC3339)})
		}
		sendRecords(key, http.StatusAccepted, id, recs...)
	}
	h.flushMonitor()
	cs := h.mcpSession(key.bearer)
	for tool, order := range map[string]string{"api_slow_routes": "p95", "api_failing_routes": "errors"} {
		out, errText := callTool(t, cs, tool, map[string]any{"property_id": id, "period": "24h"})
		if errText != "" {
			t.Fatal(errText)
		}
		ob := []map[string]any{{"metric": order}}
		if order == "errors" {
			ob = append(ob, map[string]any{"metric": "error_rate"})
		}
		want := query(tm.owner, id, map[string]any{"dataset": "api", "metrics": []string{"requests", "errors", "error_rate", "client_error_rate", "p50", "p95", "p99"},
			"dimensions": []string{"route"}, "range": map[string]any{"period": "24h"}, "limit": 10, "order_by": ob}).body["rows"].([]any)
		got := out["rows"].([]any)
		if len(got) != len(want) || len(got) != 3 {
			t.Fatalf("%s: %d rows, query %d", tool, len(got), len(want))
		}
		for i := range got {
			g, w := got[i].(map[string]any), want[i].(map[string]any)
			if fmt.Sprint(g["customer_content"].(map[string]any)["dimensions"]) != fmt.Sprint(w["dimensions"]) || fmt.Sprint(g["metrics"]) != fmt.Sprint(w["metrics"]) {
				t.Fatalf("%s row %d: %v, query %v", tool, i, g, w)
			}
		}
		if tool == "api_failing_routes" && got[0].(map[string]any)["customer_content"].(map[string]any)["dimensions"].([]any)[0] != "/broken" {
			t.Fatalf("most failing %v", got[0])
		}
	}
	hist, errText := callTool(t, cs, "api_route_history", map[string]any{"property_id": id, "route": "/broken", "period": "1h"})
	if errText != "" {
		t.Fatal(errText)
	}
	want := query(tm.owner, id, map[string]any{"dataset": "api", "metrics": []string{"requests", "errors", "error_rate", "client_error_rate", "p95", "p99"},
		"dimensions": []string{"time:minute"}, "range": map[string]any{"period": "1h"}, "compare": map[string]any{"with": "previous"},
		"filters": []map[string]any{{"dimension": "route", "op": "is", "values": []string{"/broken"}}}}).body
	if fmt.Sprint(hist["totals"].(map[string]any)["metrics"]) != fmt.Sprint(want["totals"]) || len(hist["rows"].([]any)) != 60 {
		t.Fatalf("history %v, query %v", hist["totals"], want["totals"])
	}
	cons, errText := callTool(t, cs, "api_consumer_errors", map[string]any{"property_id": id, "consumer": "partner", "period": "today"})
	if errText != "" {
		t.Fatal(errText)
	}
	if tot := cons["totals"].(map[string]any)["metrics"].(map[string]any); tot["requests"].(float64) != 120 || tot["errors"].(float64) != 10 {
		t.Fatalf("consumer %v", tot)
	}
	alerts, errText := callTool(t, cs, "monitoring_alerts", map[string]any{"property_id": id})
	if errText != "" {
		t.Fatal(errText)
	}
	if len(alerts["rules"].([]any)) != 1 || alerts["firing"].(float64) != 0 {
		t.Fatalf("alert state %v", alerts)
	}
	state, errText := callTool(t, cs, "get_setup_state", map[string]any{})
	if errText != "" || !strings.Contains(fmt.Sprint(state["integrations"]), "api_monitoring") || !strings.Contains(fmt.Sprint(state["analytics_properties"]), "kind:api") {
		t.Fatalf("setup state %v %s", state["analytics_properties"], errText)
	}
	if text := promptText(t, cs, "setup_api_monitoring", map[string]string{"api_url": "https://api.example.com"}); !strings.Contains(text, "kind api") {
		t.Fatalf("prompt %s", text)
	}
}

func TestMonitorWeeklyReportForAnAPIProperty(t *testing.T) {
	h, _ := monitorHarness(t, false, nil)
	tm := newTeam(t, h)
	owner := personEmail(tm.owner)
	now := time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)
	h.clock.Set(now)
	id := createAPIProperty(tm.owner, map[string]any{"name": "Report API"})
	sendBatch(tm.apiKey(h), id, "/reports/{id}", now.Add(-26*time.Hour), 200, 10)
	h.flushMonitor()
	tm.owner.expect(http.StatusAccepted, "POST", "/v1/analytics/properties/"+id+"/subscription/send-report", nil)
	m := h.mail.wait(t, owner, 2)
	last := m[len(m)-1]
	if !strings.Contains(last.Subject, "weekly API report") || !strings.Contains(last.Text, "Requests: 200") || !strings.Contains(last.Text, "/reports/{id}: 10") {
		t.Fatalf("report %s\n%s", last.Subject, last.Text)
	}
}

func TestMonitorAlertPushIsUrgent(t *testing.T) {
	h, fake := pushHarness(t)
	tm := newTeam(t, h)
	b := fake.subscribe(tm.owner)
	t0 := time.Date(2026, 4, 9, 10, 0, 0, 0, time.UTC)
	h.clock.Set(t0)
	id := createAPIProperty(tm.owner, map[string]any{"name": "Push API"})
	h.clock.Set(t0.Add(2 * time.Minute))
	sendBatch(tm.apiKey(h), id, "/pay", t0.Add(time.Minute), 50, 50)
	h.flushMonitor()
	if n := h.monitorAlerts(tm.ws, t0.Add(5*time.Minute+40*time.Second)); n != 1 {
		t.Fatalf("changes %d", n)
	}
	alert := alertsOf(tm.owner, "?state=firing")[0]
	h.runPushes(tm.ws)
	got := fake.to(fake.take(), b)
	if len(got) != 1 {
		t.Fatalf("pushes %d", len(got))
	}
	d := got[0]
	if d.header.Get("Urgency") != "high" || d.header.Get("TTL") != "3600" || d.header.Get("Topic") != strings.ReplaceAll(alert["id"].(string), "-", "") {
		t.Fatalf("push headers %v", d.header)
	}
	if d.payload["event"] != "monitor_alert" || !strings.HasPrefix(d.payload["url"].(string), "/analytics/"+id+"/alerts") ||
		!strings.HasPrefix(d.payload["title"].(string), "Alert: ") {
		t.Fatalf("push payload %v", d.payload)
	}
}

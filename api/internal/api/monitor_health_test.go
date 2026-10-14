package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/api"
)

func healthChecks(pg, redis string) []map[string]any {
	return []map[string]any{
		{"name": "postgres", "kind": "database", "status": pg, "latency_ms": 4.2},
		{"name": "redis", "kind": "cache", "status": redis, "critical": false, "latency_ms": 0.8, "message": "redis " + redis},
	}
}

// push sends a report at at and returns the answer.
func (h *harness) push(key *client, at time.Time, body map[string]any) map[string]any {
	h.t.Helper()
	h.clock.Set(at)
	return key.expect(http.StatusOK, "POST", "/v1/monitoring/health", body).body
}

func (h *harness) evaluateStates(at time.Time) {
	h.t.Helper()
	h.clock.Set(at)
	if err := h.srv.EvaluateMonitorStates(context.Background()); err != nil {
		h.t.Fatal(err)
	}
}

func incidentsOf(c *client, query string) []map[string]any {
	c.h.t.Helper()
	return items(c.expect(http.StatusOK, "GET", "/v1/monitoring/incidents"+query, nil))
}

func TestMonitorServiceStatesFromPushedReports(t *testing.T) {
	h, _ := monitorHarness(t, true, nil)
	tm := newTeam(t, h)
	key := tm.apiKey(h)
	owner := personEmail(tm.owner)
	t0 := time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC)
	h.clock.Set(t0)
	tm.owner.expect(http.StatusCreated, "POST", "/v1/monitoring/alert-rules", map[string]any{"kind": "service_state", "recipients": []string{memberID(tm.owner)}})
	incidentEvents := "SELECT count(*) FROM events WHERE workspace_id = $1 AND type = 'monitor.incident'"
	report := func(at time.Time, pg, redis string) map[string]any {
		return h.push(key, at, map[string]any{"service": "billing", "version": "1.4.2", "instance": "pod-a", "checks": healthChecks(pg, redis)})
	}
	first := report(t0, "healthy", "healthy")
	id := first["service_id"].(string)
	if first["state"] != "healthy" || len(first["ignored_checks"].([]any)) != 0 {
		t.Fatalf("first report %v", first)
	}
	svc := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+id, nil).body
	if svc["origin"] != "report" || svc["environment"] != "production" || svc["version"] != "1.4.2" || svc["instance_count"].(float64) != 1 ||
		len(svc["instances"].([]any)) != 1 || len(svc["dependencies"].([]any)) != 2 || svc["silence_after_seconds"].(float64) != 180 {
		t.Fatalf("service %v", svc)
	}

	if s := report(t0.Add(30*time.Second), "down", "healthy")["state"]; s != "down" {
		t.Fatalf("a critical dependency down: %v", s)
	}
	open := incidentsOf(tm.owner, "?state=open")
	if len(open) != 1 || open[0]["origin"] != "auto" || open[0]["impact"] != "major" || open[0]["title"] != "billing is down" || open[0]["published"] != false ||
		len(open[0]["updates"].([]any)) != 1 || !strings.Contains(open[0]["updates"].([]any)[0].(map[string]any)["text"].(string), "postgres is down") {
		t.Fatalf("incidents after the outage %v", open)
	}
	if mails := h.alertMails(owner); len(mails) != 1 {
		t.Fatalf("alert mails %v", mails)
	}
	report(t0.Add(60*time.Second), "down", "healthy")
	if n := len(incidentsOf(tm.owner, "")); n != 1 || len(h.alertMails(owner)) != 1 || h.monitorAlerts(tm.ws, t0.Add(70*time.Second)) != 0 {
		t.Fatalf("a second report of the outage: %d incidents, %d mails", n, len(h.alertMails(owner)))
	}
	if s := report(t0.Add(90*time.Second), "healthy", "healthy")["state"]; s != "down" {
		t.Fatalf("one healthy evaluation recovers: %v", s)
	}
	if s := report(t0.Add(120*time.Second), "healthy", "healthy")["state"]; s != "healthy" {
		t.Fatalf("two healthy evaluations: %v", s)
	}
	if mails := h.alertMails(owner); len(mails) != 2 || !strings.HasPrefix(mails[1], "Resolved: ") {
		t.Fatalf("alert mails %v", mails)
	}
	for at := t0.Add(150 * time.Second); at.Before(t0.Add(8 * time.Minute)); at = at.Add(30 * time.Second) {
		report(at, "healthy", "healthy")
		h.evaluateStates(at.Add(time.Second))
		resolved := at.Add(time.Second).Sub(t0.Add(120 * time.Second))
		if open := incidentsOf(tm.owner, "?state=open"); (len(open) == 0) != (resolved >= 5*time.Minute) {
			t.Fatalf("%v after recovery: open incidents %v", resolved, open)
		}
	}
	all := incidentsOf(tm.owner, "")
	if len(all) != 1 || all[0]["state"] != "resolved" || len(all[0]["updates"].([]any)) != 2 || h.count(incidentEvents, tm.ws) != 2 {
		t.Fatalf("one incident opened and closed once: %v, %d events", all, h.count(incidentEvents, tm.ws))
	}

	at := t0.Add(8 * time.Minute)
	if s := report(at, "healthy", "down")["state"]; s != "degraded" {
		t.Fatalf("a non-critical dependency down: %v", s)
	}
	if open := incidentsOf(tm.owner, "?state=open"); len(open) != 1 || open[0]["impact"] != "minor" || open[0]["title"] != "billing is degraded" {
		t.Fatalf("a new outage %v", open)
	}
	svc = tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+id, nil).body
	for _, d := range svc["dependencies"].([]any) {
		dep := d.(map[string]any)
		if dep["name"] == "redis" && (dep["state"] != "down" || dep["message"] != "redis down" || dep["critical"] != false) {
			t.Fatalf("redis %v", dep)
		}
	}
	h.evaluateStates(at.Add(180 * time.Second))
	if s := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+id, nil).body; s["state"] != "degraded" {
		t.Fatalf("within the silence timeout: %v", s["state"])
	}
	h.evaluateStates(at.Add(181 * time.Second))
	silent := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+id, nil).body
	if silent["state"] != "down" || silent["reason"] != "No report for 3 minutes" {
		t.Fatalf("silence: %v %v", silent["state"], silent["reason"])
	}
	for _, d := range silent["dependencies"].([]any) {
		if d.(map[string]any)["state"] != "unknown" {
			t.Fatalf("a silent service's dependency %v", d)
		}
	}

	changes := items(tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+id+"/state-changes", nil))
	var serviceChanges []string
	for i := len(changes) - 1; i >= 0; i-- {
		if c := changes[i]; c["dependency_id"] == nil {
			serviceChanges = append(serviceChanges, fmt.Sprint(c["previous"], ">", c["state"]))
		}
	}
	if fmt.Sprint(serviceChanges) != "[unknown>healthy healthy>down down>healthy healthy>degraded degraded>down]" {
		t.Fatalf("service changes %v", serviceChanges)
	}
	day := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+id+"/history?days=1", nil).body["days"].([]any)[0].(map[string]any)
	if day["day"] != "2026-04-02" || day["down_seconds"].(float64) != 90 || day["degraded_seconds"].(float64) != 181 || day["worst"] != "down" ||
		day["healthy_seconds"].(float64) != 390 {
		t.Fatalf("history %v", day)
	}
	hours := items(tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+id+"/hours?hours=2", nil))
	var pg map[string]any
	for _, r := range hours {
		if r["dependency"] == "postgres" {
			pg = r
		}
	}
	if len(hours) != 3 || pg == nil || pg["reports"].(float64) != 17 || pg["down"].(float64) != 2 || math.Abs(pg["latency_p95"].(float64)-4.2) > 4.2*0.011 {
		t.Fatalf("hours %v", hours)
	}

	mcpKey := tm.apiKey(h)
	out, errText := callTool(t, h.mcpSession(mcpKey.bearer), "service_health", nil)
	if errText != "" {
		t.Fatal(errText)
	}
	listed := items(tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services", nil))
	if len(out["services"].([]any)) != len(listed) || out["down"].(float64) != 1 {
		t.Fatalf("mcp %v, endpoint %v", out, listed)
	}
	for i, s := range out["services"].([]any) {
		m := s.(map[string]any)
		if m["id"] != listed[i]["id"] || m["state"] != listed[i]["state"] || m["reason"] != listed[i]["reason"] ||
			len(m["dependencies"].([]any)) != len(listed[i]["dependencies"].([]any)) {
			t.Fatalf("mcp service %v, endpoint %v", m, listed[i])
		}
	}
	inc, errText := callTool(t, h.mcpSession(mcpKey.bearer), "monitoring_incidents", map[string]any{"state": "open"})
	if errText != "" {
		t.Fatal(errText)
	}
	openNow := incidentsOf(tm.owner, "?state=open")
	if got := inc["items"].([]any); len(got) != len(openNow) || got[0].(map[string]any)["id"] != openNow[0]["id"] ||
		len(got[0].(map[string]any)["updates"].([]any)) != len(openNow[0]["updates"].([]any)) {
		t.Fatalf("mcp incidents %v, endpoint %v", got, openNow)
	}

	h.clock.Set(at.Add(25 * time.Hour))
	if err := h.srv.MaintainAnalytics(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+id, nil).body["instance_count"].(float64); n != 0 {
		t.Fatalf("instances after 24 hours of silence: %v", n)
	}
	tm.owner.expect(http.StatusNoContent, "DELETE", "/v1/monitoring/services/"+id, nil)
	for _, table := range []string{"monitor_dependencies", "monitor_instances", "monitor_state_changes", "monitor_health_hours"} {
		if n := h.count("SELECT count(*) FROM "+table+" WHERE service_id = $1", id); n != 0 {
			t.Fatalf("%s rows of a deleted service: %d", table, n)
		}
	}
	if n := h.count("SELECT count(*) FROM monitor_incidents WHERE workspace_id = $1 AND $2::uuid = ANY(service_ids)", tm.ws, id); n != 0 {
		t.Fatalf("incidents still listing a deleted service: %d", n)
	}
}

func TestMonitorHealthReportsAreChecked(t *testing.T) {
	h, _ := monitorHarness(t, true, nil)
	tm := newTeam(t, h)
	key := tm.apiKey(h)
	at := time.Date(2026, 4, 3, 9, 0, 0, 0, time.UTC)
	h.clock.Set(at)
	reader := h.client()
	reader.bearer = tm.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "reader", "scopes": []string{"analytics:read"}}).str("secret")
	ok := map[string]any{"service": "api"}
	tm.owner.expect(http.StatusForbidden, "POST", "/v1/monitoring/health", ok)
	reader.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", "/v1/monitoring/health", ok)
	many := []map[string]any{}
	for i := range 51 {
		many = append(many, map[string]any{"name": fmt.Sprint("d", i), "status": "healthy"})
	}
	for _, bad := range []map[string]any{
		{}, {"service": ""}, {"service": strings.Repeat("s", 101)}, {"service": "api", "status": "fine"}, {"service": "api", "environment": strings.Repeat("e", 41)},
		{"service": "api", "checks": many},
		{"service": "api", "checks": []map[string]any{{"name": "db", "status": "healthy"}, {"name": "db", "status": "down"}}},
		{"service": "api", "checks": []map[string]any{{"name": "db", "status": "healthy", "message": strings.Repeat("ş", 201)}}},
		{"service": "api", "checks": []map[string]any{{"name": "db", "status": "healthy", "latency_ms": -1}}},
		{"service": "api", "checks": []map[string]any{{"name": "db", "status": "healthy", "kind": "disk"}}},
		{"service": "api", "checks": []map[string]any{{"status": "healthy"}}},
	} {
		key.expect(http.StatusBadRequest, "POST", "/v1/monitoring/health", bad)
	}
	if n := len(items(tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services", nil))); n != 0 {
		t.Fatalf("refused reports made %d services", n)
	}
	edge := key.expect(http.StatusOK, "POST", "/v1/monitoring/health", map[string]any{"service": "api",
		"checks": []map[string]any{{"name": "db", "status": "healthy", "message": strings.Repeat("ş", 200)}}}).body
	if edge["state"] != "healthy" {
		t.Fatalf("a 200 character message %v", edge)
	}

	wide := many[:50]
	if r := h.push(key, at, map[string]any{"service": "wide", "checks": wide}); len(r["ignored_checks"].([]any)) != 0 {
		t.Fatalf("50 dependencies %v", r)
	}
	r := h.push(key, at.Add(time.Second), map[string]any{"service": "wide", "checks": append([]map[string]any{{"name": "extra", "status": "down"}}, wide[:3]...)})
	if fmt.Sprint(r["ignored_checks"]) != "[extra]" || r["state"] != "healthy" {
		t.Fatalf("a 51st dependency %v", r)
	}

	for i := range 600 {
		if i%100 == 0 {
			h.clock.Set(at.Add(2 * time.Second))
		}
		key.expect(http.StatusOK, "POST", "/v1/monitoring/health", map[string]any{"service": "busy"})
	}
	key.expectProblem(http.StatusTooManyRequests, "rate_limited", "POST", "/v1/monitoring/health", map[string]any{"service": "busy"})
	key.expect(http.StatusOK, "POST", "/v1/monitoring/health", map[string]any{"service": "busy", "environment": "staging"})

	n := len(items(tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services", nil)))
	for i := n; i < 100; i++ {
		key.expect(http.StatusOK, "POST", "/v1/monitoring/health", map[string]any{"service": fmt.Sprint("svc-", i)})
	}
	key.expectProblem(http.StatusForbidden, "service_limit", "POST", "/v1/monitoring/health", map[string]any{"service": "one-more"})
	tm.owner.expectProblem(http.StatusForbidden, "service_limit", "POST", "/v1/monitoring/services", map[string]any{"name": "one-more"})
	key.expect(http.StatusOK, "POST", "/v1/monitoring/health", map[string]any{"service": "api"})
}

func TestMonitorMaintenanceHoldsAlertsAndIncidents(t *testing.T) {
	h, _ := monitorHarness(t, true, nil)
	tm := newTeam(t, h)
	key := tm.apiKey(h)
	owner := personEmail(tm.owner)
	t0 := time.Date(2026, 4, 4, 22, 0, 0, 0, time.UTC)
	h.clock.Set(t0)
	report := func(at time.Time, pg string) map[string]any {
		return h.push(key, at, map[string]any{"service": "orders", "checks": healthChecks(pg, "healthy")})
	}
	id := report(t0, "healthy")["service_id"].(string)
	tm.owner.expect(http.StatusCreated, "POST", "/v1/monitoring/alert-rules", map[string]any{"kind": "service_state", "service_id": id,
		"recipients": []string{memberID(tm.owner)}})
	tm.owner.expect(http.StatusBadRequest, "POST", "/v1/monitoring/incidents", map[string]any{"kind": "maintenance", "title": "No window"})
	tm.owner.expect(http.StatusBadRequest, "POST", "/v1/monitoring/incidents", map[string]any{"kind": "maintenance", "title": "Backwards",
		"scheduled_start": t0.Add(time.Hour), "scheduled_end": t0})
	tm.owner.expect(http.StatusBadRequest, "POST", "/v1/monitoring/incidents", map[string]any{"title": "Scheduled incident", "scheduled_start": t0})
	tm.agent.expect(http.StatusForbidden, "POST", "/v1/monitoring/incidents", map[string]any{"title": "Agent"})
	win := tm.owner.expect(http.StatusCreated, "POST", "/v1/monitoring/incidents", map[string]any{"kind": "maintenance", "title": "Database upgrade",
		"service_ids": []string{id}, "scheduled_start": t0.Add(time.Minute), "scheduled_end": t0.Add(11 * time.Minute), "text": "Upgrading Postgres to 18."}).body
	if win["state"] != "identified" || win["impact"] != "none" || win["active"] != false || win["started_at"] != "2026-04-04T22:01:00Z" {
		t.Fatalf("window %v", win)
	}
	if s := report(t0.Add(time.Minute), "down")["state"]; s != "down" {
		t.Fatalf("down inside the window: %v", s)
	}
	for at := t0.Add(90 * time.Second); at.Before(t0.Add(11 * time.Minute)); at = at.Add(30 * time.Second) {
		report(at, "down")
		h.evaluateStates(at.Add(time.Second))
		h.monitorAlerts(tm.ws, at.Add(2*time.Second))
	}
	svc := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+id, nil).body
	if svc["maintenance"] != true || svc["state"] != "down" {
		t.Fatalf("service in the window %v", svc)
	}
	var own []map[string]any
	for _, c := range items(tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+id+"/state-changes", nil)) {
		if c["dependency_id"] == nil {
			own = append(own, c)
		}
	}
	if len(own) != 2 || own[0]["state"] != "down" || own[0]["maintenance"] != true || own[1]["maintenance"] != false {
		t.Fatalf("changes %v", own)
	}
	if auto := h.count("SELECT count(*) FROM monitor_incidents WHERE workspace_id = $1 AND origin = 'auto'", tm.ws); auto != 0 || len(h.alertMails(owner)) != 0 {
		t.Fatalf("inside a window: %d automatic incidents, %v mails", auto, h.alertMails(owner))
	}
	report(t0.Add(11*time.Minute), "down")
	h.evaluateStates(t0.Add(11*time.Minute + time.Second))
	got := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/incidents/"+win["id"].(string), nil).body
	if got["state"] != "resolved" || got["active"] != false || got["updates"].([]any)[0].(map[string]any)["text"] != "The maintenance window ended." {
		t.Fatalf("window after its end %v", got)
	}
	if open := incidentsOf(tm.owner, "?state=open&kind=incident"); len(open) != 1 || open[0]["origin"] != "auto" || len(h.alertMails(owner)) != 1 {
		t.Fatalf("after the window: incidents %v, mails %v", open, h.alertMails(owner))
	}
	day := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+id+"/history?days=1", nil).body["days"].([]any)[0].(map[string]any)
	if day["maintenance_seconds"].(float64) != 600 || day["healthy_seconds"].(float64) != 60 {
		t.Fatalf("history %v", day)
	}

	manual := tm.owner.expect(http.StatusCreated, "POST", "/v1/monitoring/incidents", map[string]any{"title": "Payments slow", "impact": "major",
		"service_ids": []string{id}}).body
	mid := manual["id"].(string)
	tm.owner.expect(http.StatusOK, "PATCH", "/v1/monitoring/incidents/"+mid, map[string]any{"published": true, "title": "Payments are slow"})
	up := tm.owner.expect(http.StatusCreated, "POST", "/v1/monitoring/incidents/"+mid+"/updates", map[string]any{"state": "resolved", "text": "Fixed."}).body
	if up["state"] != "resolved" || up["resolved_at"] == nil || up["published"] != true || len(up["updates"].([]any)) != 2 ||
		up["updates"].([]any)[0].(map[string]any)["notify"] != true {
		t.Fatalf("resolved manual incident %v", up)
	}
	tm.owner.expect(http.StatusBadRequest, "POST", "/v1/monitoring/incidents/"+mid+"/updates", map[string]any{"state": "resolved", "text": strings.Repeat("x", 5001)})
	tm.owner.expect(http.StatusNoContent, "DELETE", "/v1/monitoring/incidents/"+mid, nil)
	tm.owner.expect(http.StatusNotFound, "GET", "/v1/monitoring/incidents/"+mid, nil)
}

type healthTarget struct {
	srv   *httptest.Server
	ctype atomic.Value
	body  atomic.Value
	code  atomic.Int32
	hits  atomic.Int32
}

func newHealthTarget(t *testing.T, code int, ctype, body string) *healthTarget {
	tg := &healthTarget{}
	tg.set(code, ctype, body)
	tg.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tg.hits.Add(1)
		if ct := tg.ctype.Load().(string); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.WriteHeader(int(tg.code.Load()))
		_, _ = w.Write([]byte(tg.body.Load().(string)))
	}))
	t.Cleanup(tg.srv.Close)
	return tg
}

func (tg *healthTarget) set(code int, ctype, body string) {
	tg.code.Store(int32(code))
	tg.ctype.Store(ctype)
	tg.body.Store(body)
}

func (tg *healthTarget) url(host string) string {
	return "http://" + host + ":" + tg.srv.URL[strings.LastIndex(tg.srv.URL, ":")+1:] + "/health"
}

// runPull makes a pull at at as the queue would: one with no answer is tried again 20 seconds
// later.
func (h *harness) runPull(ws, id string, at time.Time) {
	h.t.Helper()
	ctx := context.Background()
	h.clock.Set(at)
	a := api.MonitorPullArgs{WorkspaceID: uuid.MustParse(ws), ServiceID: uuid.MustParse(id), Slot: at.Unix(), Attempt: 1}
	if err := h.srv.RunMonitorPull(ctx, a); err != nil {
		h.t.Fatal(err)
	}
	for _, j := range h.jobs("monitor_pull", ws) {
		var next api.MonitorPullArgs
		if err := json.Unmarshal(j.args, &next); err != nil {
			h.t.Fatal(err)
		}
		h.dropJob(j.id)
		if next.Attempt == 2 && next.ServiceID == a.ServiceID {
			h.clock.Set(at.Add(20 * time.Second))
			if err := h.srv.RunMonitorPull(ctx, next); err != nil {
				h.t.Fatal(err)
			}
		}
	}
}

func depStates(svc map[string]any) string {
	var out []string
	for _, d := range svc["dependencies"].([]any) {
		m := d.(map[string]any)
		out = append(out, fmt.Sprint(m["name"], ":", m["kind"], ":", m["state"], ":", m["critical"]))
	}
	return strings.Join(out, " ")
}

func TestMonitorPulledHealthFormats(t *testing.T) {
	h, _ := monitorHarness(t, true, nil)
	tm := newTeam(t, h)
	key := tm.apiKey(h)
	t0 := time.Date(2026, 4, 5, 6, 0, 0, 0, time.UTC)
	h.clock.Set(t0)
	actuatorDown := `{"status":"DOWN","components":{"db":{"status":"UP"},"redis":{"status":"DOWN","details":{"error":"Connection refused"}}}}`
	draft := newHealthTarget(t, 200, "application/health+json",
		`{"status":"pass","releaseId":"2.0.1","checks":{"postgres:responseTime":[{"componentType":"datastore","observedValue":3,"observedUnit":"ms","status":"pass"}]}}`)
	actuator := newHealthTarget(t, 503, "application/vnd.spring-boot.actuator.v3+json", actuatorDown)
	plain := newHealthTarget(t, 503, "text/plain", "unavailable")
	create := func(name string, tg *healthTarget, host string) string {
		return tm.owner.expect(http.StatusCreated, "POST", "/v1/monitoring/services", map[string]any{"name": name, "mode": "pull", "health_url": tg.url(host),
			"interval_seconds": 30}).str("id")
	}
	ids := map[string]string{"draft": create("draft", draft, "draft.test"), "actuator": create("actuator", actuator, "actuator.test"),
		"plain": create("plain", plain, "plain.test")}
	if n, err := h.srv.QueueDueMonitorPulls(context.Background()); err != nil || n != 3 {
		t.Fatalf("due pulls %d %v", n, err)
	}
	for _, j := range h.jobs("monitor_pull", tm.ws) {
		h.dropJob(j.id)
	}
	get := func(name string) map[string]any {
		return tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+ids[name], nil).body
	}
	for _, name := range []string{"draft", "actuator", "plain"} {
		h.runPull(tm.ws, ids[name], t0)
	}
	if s := get("draft"); s["state"] != "healthy" || s["version"] != "2.0.1" || depStates(s) != "postgres:database:healthy:true" ||
		s["dependencies"].([]any)[0].(map[string]any)["latency_ms"].(float64) != 3 {
		t.Fatalf("health+json %v", s)
	}
	act := get("actuator")
	if act["state"] != "down" || depStates(act) != "db:database:healthy:true redis:cache:down:true" ||
		act["dependencies"].([]any)[1].(map[string]any)["message"] != "Connection refused" {
		t.Fatalf("actuator %v", act)
	}
	pushed := h.push(key, t0, map[string]any{"service": "pushed-like-actuator", "status": "down", "checks": []map[string]any{
		{"name": "db", "kind": "database", "status": "healthy"}, {"name": "redis", "kind": "cache", "status": "down", "message": "Connection refused"}}})
	same := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+pushed["service_id"].(string), nil).body
	if same["state"] != act["state"] || depStates(same) != depStates(act) || same["reason"] != act["reason"] {
		t.Fatalf("pushed %v %v %v, pulled %v %v %v", same["state"], depStates(same), same["reason"], act["state"], depStates(act), act["reason"])
	}
	if s := get("plain"); s["state"] != "down" || s["reason"] != "The health endpoint answered 503" || len(s["dependencies"].([]any)) != 0 {
		t.Fatalf("status only %v", s)
	}
	plain.set(200, "text/plain", "ok")
	h.runPull(tm.ws, ids["plain"], t0.Add(30*time.Second))
	if s := get("plain")["state"]; s != "down" {
		t.Fatalf("one healthy pull: %v", s)
	}
	h.runPull(tm.ws, ids["plain"], t0.Add(60*time.Second))
	if s := get("plain")["state"]; s != "healthy" {
		t.Fatalf("two healthy pulls: %v", s)
	}
	actuator.set(200, "application/json", `{"status":"UP","components":{"db":{"status":"UP"},"redis":{"status":"UP"}}}`)
	h.runPull(tm.ws, ids["actuator"], t0.Add(30*time.Second))
	h.runPull(tm.ws, ids["actuator"], t0.Add(60*time.Second))
	if s := get("actuator"); s["state"] != "healthy" || depStates(s) != "db:database:healthy:true redis:cache:healthy:true" {
		t.Fatalf("actuator back up %v", s)
	}

	hits := draft.hits.Load()
	draft.srv.Close()
	h.runPull(tm.ws, ids["draft"], t0.Add(90*time.Second))
	gone := get("draft")
	if gone["state"] != "down" || !strings.HasPrefix(gone["reason"].(string), "The health endpoint gave no answer: ") || draft.hits.Load() != hits {
		t.Fatalf("no answer %v", gone)
	}
	if n := h.count("SELECT count(*) FROM monitor_health_hours WHERE service_id = $1 AND dependency = '' AND down = 1", ids["draft"]); n != 1 {
		t.Fatalf("a pull without an answer counted %d times", n)
	}
	tm.owner.expect(http.StatusOK, "PATCH", "/v1/monitoring/services/"+ids["plain"], map[string]any{"mode": "push", "health_url": nil})
	h.runPull(tm.ws, ids["plain"], t0.Add(120*time.Second))
	if s := get("plain"); s["mode"] != "push" || s["next_pull_at"] != nil || s["last_report_at"] != "2026-04-05T06:01:00Z" {
		t.Fatalf("a push service is not pulled: %v", s)
	}
}

func TestMonitorHealthURLsRefuseAddresses(t *testing.T) {
	h, dns := monitorHarness(t, false, nil)
	tm := newTeam(t, h)
	tg := newHealthTarget(t, 200, "application/health+json", `{"status":"pass"}`)
	dns.set("pub.example", "93.184.216.34")
	dns.set("inside.example", "10.0.0.7")
	for _, u := range []string{tg.srv.URL + "/health", "http://localhost/health", "http://10.1.2.3/", "http://169.254.169.254/latest/meta-data/",
		"http://[::1]/", "http://inside.example/health", "http://100.64.0.9/"} {
		tm.owner.expectProblem(http.StatusUnprocessableEntity, "refused_address", "POST", "/v1/monitoring/services", map[string]any{"name": "x", "mode": "pull",
			"health_url": u})
	}
	for _, body := range []map[string]any{
		{"name": "x", "mode": "pull"}, {"name": "x", "mode": "pull", "health_url": "ftp://pub.example/"},
		{"name": "x", "mode": "pull", "health_url": "http://u:p@pub.example/"}, {"name": "x", "interval_seconds": 20}, {"name": "x", "interval_seconds": 601},
		{"name": "x", "silence_seconds": 30}, {"name": "x", "timeout_seconds": 31}, {"name": "x", "error_rate_threshold": 0}, {"name": ""},
		{"name": "x", "mode": "both"},
	} {
		tm.owner.expect(http.StatusBadRequest, "POST", "/v1/monitoring/services", body)
	}
	id := tm.owner.expect(http.StatusCreated, "POST", "/v1/monitoring/services", map[string]any{"name": "public", "mode": "pull",
		"health_url": tg.url("pub.example")}).str("id")
	tm.owner.expectProblem(http.StatusConflict, "service_exists", "POST", "/v1/monitoring/services", map[string]any{"name": "public"})
	tm.owner.expectProblem(http.StatusUnprocessableEntity, "refused_address", "PATCH", "/v1/monitoring/services/"+id, map[string]any{"health_url": "http://127.0.0.1/"})
	dns.set("pub.example", "127.0.0.1")
	h.runPull(tm.ws, id, time.Date(2026, 4, 6, 8, 0, 0, 0, time.UTC))
	s := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+id, nil).body
	if s["state"] != "down" || !strings.Contains(s["reason"].(string), "an address the server does not call") || tg.hits.Load() != 0 {
		t.Fatalf("a health URL rebound to loopback: %v, %d hits", s["reason"], tg.hits.Load())
	}
	rules := items(tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/alert-rules?service_id="+id, nil))
	if len(rules) != 1 || rules[0]["kind"] != "service_state" || rules[0]["name"] != "public is degraded or down" {
		t.Fatalf("the member's default rule %v", rules)
	}
}

func TestMonitorServicesStayInTheirWorkspace(t *testing.T) {
	h, _ := monitorHarness(t, true, nil)
	tm, other := newTeam(t, h), newTeam(t, h)
	at := time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC)
	mine := h.push(tm.apiKey(h), at, map[string]any{"service": "api", "checks": healthChecks("down", "healthy")})
	id := mine["service_id"].(string)
	theirs := h.push(other.apiKey(h), at, map[string]any{"service": "api"})
	if theirs["service_id"] == id || theirs["state"] != "healthy" {
		t.Fatalf("the same name in another workspace %v", theirs)
	}
	svc := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+id, nil).body
	dep := svc["dependencies"].([]any)[0].(map[string]any)["id"].(string)
	inc := incidentsOf(tm.owner, "")[0]["id"].(string)
	for _, r := range []struct{ method, path string }{
		{"GET", "/v1/monitoring/services/" + id}, {"PATCH", "/v1/monitoring/services/" + id}, {"DELETE", "/v1/monitoring/services/" + id},
		{"GET", "/v1/monitoring/services/" + id + "/history"}, {"GET", "/v1/monitoring/services/" + id + "/state-changes"},
		{"GET", "/v1/monitoring/services/" + id + "/hours"}, {"GET", "/v1/monitoring/incidents/" + inc}, {"PATCH", "/v1/monitoring/incidents/" + inc},
		{"DELETE", "/v1/monitoring/incidents/" + inc},
	} {
		other.owner.expect(http.StatusNotFound, r.method, r.path, map[string]any{"interval_seconds": 120, "title": "taken"})
	}
	other.owner.expect(http.StatusNotFound, "POST", "/v1/monitoring/incidents/"+inc+"/updates", map[string]any{"state": "resolved", "text": "taken"})
	other.owner.expect(http.StatusNotFound, "POST", "/v1/monitoring/alert-rules", map[string]any{"kind": "service_state", "service_id": id})
	other.owner.expect(http.StatusBadRequest, "POST", "/v1/monitoring/incidents", map[string]any{"title": "x", "service_ids": []string{id}})
	other.owner.expect(http.StatusBadRequest, "POST", "/v1/monitoring/incidents", map[string]any{"title": "x", "dependency_ids": []string{dep}})
	if n := len(items(other.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services", nil))); n != 1 {
		t.Fatalf("services of another workspace: %d", n)
	}
	if n := len(incidentsOf(other.owner, "")); n != 0 {
		t.Fatalf("incidents of another workspace: %d", n)
	}
	tm.owner.expect(http.StatusNotFound, "GET", "/v1/monitoring/services/"+id+"/history?dependency_id="+uuid.NewV7().String(), nil)
	if n := len(items(tm.agent.expect(http.StatusOK, "GET", "/v1/monitoring/services", nil))); n != 1 || len(incidentsOf(tm.agent, "")) != 1 {
		t.Fatal("an agent reads services and incidents")
	}
	tm.agent.expect(http.StatusForbidden, "POST", "/v1/monitoring/services", map[string]any{"name": "agent"})
	tm.agent.expect(http.StatusForbidden, "DELETE", "/v1/monitoring/services/"+id, nil)
}

func TestMonitorServicesFromRecordsAndSpans(t *testing.T) {
	h := geoHarness(t, nil)
	tm := newTeam(t, h)
	h.clock.Set(time.Date(2026, 10, 12, 12, 1, 0, 0, time.UTC))
	prop := createAPIProperty(tm.owner, map[string]any{"name": "Shop"})
	backend := tm.apiKey(h)
	sendRecords(backend, http.StatusAccepted, prop, map[string]any{"method": "GET", "route": "/orders", "status": 200, "duration_us": 1200, "service": "checkout",
		"release": "9.1.0"})
	if js := exportOTLP(h, backend.bearer, prop, "application/json", otlpFixture(t, "traces.json"), nil); js.status != http.StatusOK {
		t.Fatalf("export %d", js.status)
	}
	byName := map[string]map[string]any{}
	for _, s := range items(tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services", nil)) {
		byName[s["name"].(string)+"/"+s["environment"].(string)] = s
	}
	co, shop := byName["checkout/production"], byName["shop-api/staging"]
	if co == nil || co["origin"] != "records" || co["property_id"] != prop || co["version"] != "9.1.0" || co["state"] != "unknown" {
		t.Fatalf("a service from records %v", byName)
	}
	if shop == nil || shop["instance_count"].(float64) != 1 || shop["version"] != "1.4.2" {
		t.Fatalf("a service from spans %v", shop)
	}
	got := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/services/"+shop["id"].(string), nil).body
	if got["instances"].([]any)[0].(map[string]any)["instance"] != "shop-api-7f9c" {
		t.Fatalf("instances %v", got["instances"])
	}
}

package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/api"
)

type fakeDNS struct {
	mu sync.Mutex
	m  map[string][]netip.Addr
}

func (f *fakeDNS) set(host string, ips ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[host] = nil
	for _, ip := range ips {
		f.m[host] = append(f.m[host], netip.MustParseAddr(ip))
	}
}

func (f *fakeDNS) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ips, ok := f.m[host]; ok {
		return ips, nil
	}
	if strings.HasSuffix(host, ".test") {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

func monitorHarness(t *testing.T, allowPrivate bool, configure func(*api.MonitorSettings)) (*harness, *fakeDNS) {
	dns := &fakeDNS{m: map[string][]netip.Addr{}}
	h := newHarnessWith(t, func(d *api.Deps) {
		d.Analytics.Monitor.AllowPrivate, d.Analytics.Monitor.Resolver = allowPrivate, dns
		if configure != nil {
			configure(&d.Analytics.Monitor)
		}
	})
	return h, dns
}

// target is a test endpoint whose answer the test switches.
type target struct {
	srv    *httptest.Server
	status atomic.Int32
	hits   atomic.Int32
	header atomic.Value
}

func newTarget(t *testing.T) *target {
	tg := &target{}
	tg.status.Store(200)
	tg.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tg.hits.Add(1)
		tg.header.Store(r.Header.Get("X-Api-Key"))
		w.WriteHeader(int(tg.status.Load()))
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(tg.srv.Close)
	return tg
}

func (tg *target) url(host string) string {
	return "http://" + host + ":" + tg.srv.URL[strings.LastIndex(tg.srv.URL, ":")+1:] + "/health"
}

// runCheck makes the check's run at at as the queue would: a failed first try is tried again 20
// seconds later.
func (h *harness) runCheck(ws, id string, at time.Time) {
	h.t.Helper()
	ctx := context.Background()
	h.clock.Set(at)
	a := api.MonitorCheckArgs{WorkspaceID: uuid.MustParse(ws), CheckID: uuid.MustParse(id), Slot: at.Unix(), Attempt: 1}
	if err := h.srv.RunMonitorCheck(ctx, a); err != nil {
		h.t.Fatal(err)
	}
	for _, j := range h.jobs("monitor_check", ws) {
		var next api.MonitorCheckArgs
		if err := json.Unmarshal(j.args, &next); err != nil {
			h.t.Fatal(err)
		}
		h.dropJob(j.id)
		if next.Attempt == 2 && next.CheckID == a.CheckID {
			h.clock.Set(at.Add(20 * time.Second))
			if err := h.srv.RunMonitorCheck(ctx, next); err != nil {
				h.t.Fatal(err)
			}
		}
	}
}

func (h *harness) settleChecks(at time.Time) {
	h.t.Helper()
	h.clock.Set(at)
	if err := h.srv.SettleMonitorResults(context.Background()); err != nil {
		h.t.Fatal(err)
	}
}

// monitorAlerts runs the alerts job at at and returns how many alerts of ws fired or resolved.
func (h *harness) monitorAlerts(ws string, at time.Time) int {
	h.t.Helper()
	h.clock.Set(at)
	events := "SELECT count(*) FROM events WHERE workspace_id = $1 AND type = 'monitor.alert'"
	before := h.count(events, ws)
	if _, err := h.srv.CheckMonitorAlerts(context.Background()); err != nil {
		h.t.Fatal(err)
	}
	return h.count(events, ws) - before
}

// alertMails are the monitoring alert mails addr got.
func (h *harness) alertMails(addr string) []string {
	var out []string
	for _, m := range h.mail.to(addr) {
		if strings.HasPrefix(m.Subject, "Alert: ") || strings.HasPrefix(m.Subject, "Resolved: ") {
			out = append(out, m.Subject)
		}
	}
	return out
}

func createCheck(c *client, body map[string]any) map[string]any {
	c.h.t.Helper()
	return c.expect(http.StatusCreated, "POST", "/v1/monitoring/checks", body).body
}

func TestMonitorChecksRefuseAddressesWhenSavedAndRun(t *testing.T) {
	h, dns := monitorHarness(t, false, nil)
	tm := newTeam(t, h)
	tg := newTarget(t)
	dns.set("internal.example", "10.0.0.8")
	dns.set("mixed.example", "93.184.216.34", "192.168.0.4")
	dns.set("meta.example", "169.254.169.254")
	dns.set("mapped.example", "::ffff:127.0.0.1")
	dns.set("pub.example", "93.184.216.34")
	for _, u := range []string{
		tg.srv.URL, "http://localhost/", "http://api.localhost/", "http://[::1]/", "http://10.1.1.1/", "http://192.168.1.1/", "http://100.64.0.1/",
		"http://169.254.169.254/latest/meta-data/", "http://[fd00:ec2::254]/", "http://[fe80::1]/", "http://[::ffff:127.0.0.1]/", "http://[::ffff:a9fe:a9fe]/",
		"http://224.0.0.1/", "http://0.0.0.0/", "http://internal.example/", "http://mixed.example/", "http://meta.example/", "http://mapped.example/",
	} {
		tm.owner.expectProblem(http.StatusUnprocessableEntity, "refused_address", "POST", "/v1/monitoring/checks", map[string]any{"name": "x", "url": u})
	}
	for _, u := range []string{"http://user:pw@pub.example/", "ftp://pub.example/", "https://pub.example/#x", "pub.example/health"} {
		tm.owner.expect(http.StatusBadRequest, "POST", "/v1/monitoring/checks", map[string]any{"name": "x", "url": u})
	}
	c := createCheck(tm.owner, map[string]any{"name": "Public", "url": tg.url("pub.example")})
	dns.set("pub.example", "127.0.0.1")
	at := time.Date(2026, 3, 2, 9, 0, 5, 0, time.UTC)
	h.runCheck(tm.ws, c["id"].(string), at)
	res := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/checks/"+c["id"].(string)+"/results", nil).body["items"].([]any)
	if len(res) != 1 || res[0].(map[string]any)["error"] != "refused_address" || res[0].(map[string]any)["retried"] != true {
		t.Fatalf("a check rebound to loopback: %v", res)
	}
	if tg.hits.Load() != 0 {
		t.Fatal("the server called a refused address")
	}

	hp, _ := monitorHarness(t, true, nil)
	tp := newTeam(t, hp)
	createCheck(tp.owner, map[string]any{"name": "Internal", "url": tg.url("127.0.0.1")})
	for _, u := range []string{"http://169.254.169.254/", "http://[fe80::1]/", "http://[::ffff:169.254.169.254]/", "http://[fd00:ec2::254]/"} {
		tp.owner.expectProblem(http.StatusUnprocessableEntity, "refused_address", "POST", "/v1/monitoring/checks", map[string]any{"name": "x", "url": u})
	}
}

func TestMonitorCheckStatesUptimeAndAlerts(t *testing.T) {
	h, _ := monitorHarness(t, true, func(m *api.MonitorSettings) { m.Checks, m.MinInterval = 12, 60*time.Second })
	tm := newTeam(t, h)
	tg := newTarget(t)
	owner := personEmail(tm.owner)
	t0 := time.Date(2026, 3, 3, 10, 0, 5, 0, time.UTC)
	h.clock.Set(t0.Add(-time.Hour))
	prop := createAPIProperty(tm.owner, map[string]any{"name": "Shop API"})

	tm.owner.expectProblem(http.StatusUnprocessableEntity, "interval_too_short", "POST", "/v1/monitoring/checks",
		map[string]any{"name": "Fast", "url": tg.url("a.test"), "interval_seconds": 45})
	tm.owner.expect(http.StatusBadRequest, "POST", "/v1/monitoring/checks", map[string]any{"name": "Head", "url": tg.url("a.test"), "method": "HEAD", "keyword": "ok"})
	tm.agent.expect(http.StatusForbidden, "POST", "/v1/monitoring/checks", map[string]any{"name": "Agent", "url": tg.url("a.test")})

	c := createCheck(tm.owner, map[string]any{"name": "Health", "url": tg.url("a.test"), "property_id": prop,
		"headers": []map[string]any{{"name": "X-Api-Key", "value": "sk_live_secret"}}})
	id := c["id"].(string)
	if c["state"] != "unknown" || fmt.Sprint(c["header_names"]) != "[X-Api-Key]" || c["interval_seconds"].(float64) != 60 || strings.Contains(fmt.Sprint(c), "sk_live") {
		t.Fatalf("check %v", c)
	}
	var raw string
	if err := h.st.Pool.QueryRow(context.Background(), "SELECT headers::text FROM monitor_checks WHERE id = $1", id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "sk_live_secret") || !strings.Contains(raw, "X-Api-Key") {
		t.Fatalf("headers at rest %s", raw)
	}
	rules := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/alert-rules?check_id="+id, nil).body["items"].([]any)
	if len(rules) != 1 || rules[0].(map[string]any)["kind"] != "check_state" {
		t.Fatalf("default check rule %v", rules)
	}

	for i := 1; i < 10; i++ {
		createCheck(tm.owner, map[string]any{"name": fmt.Sprint("Same host ", i), "url": tg.url("a.test") + fmt.Sprint("?n=", i)})
	}
	tm.owner.expectProblem(http.StatusForbidden, "host_limit", "POST", "/v1/monitoring/checks", map[string]any{"name": "Eleventh", "url": tg.url("a.test")})
	createCheck(tm.owner, map[string]any{"name": "B", "url": tg.url("b.test")})
	createCheck(tm.owner, map[string]any{"name": "C", "url": tg.url("c.test")})
	tm.owner.expectProblem(http.StatusForbidden, "check_limit", "POST", "/v1/monitoring/checks", map[string]any{"name": "D", "url": tg.url("d.test")})
	for _, item := range tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/checks", nil).body["items"].([]any) {
		if other := item.(map[string]any)["id"].(string); other != id {
			tm.owner.expect(http.StatusOK, "PATCH", "/v1/monitoring/checks/"+other, map[string]any{"paused": true})
		}
	}

	run := func(minute int, status int) {
		tg.status.Store(int32(status))
		h.runCheck(tm.ws, id, t0.Add(time.Duration(minute)*time.Minute))
	}
	state := func() string {
		return tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/checks/"+id, nil).str("state")
	}
	run(0, 200)
	if tg.header.Load() != "sk_live_secret" {
		t.Fatalf("header sent %v", tg.header.Load())
	}
	h.settleChecks(t0.Add(2 * time.Minute))
	if state() != "up" {
		t.Fatalf("after a good run: %s", state())
	}
	run(1, 500)
	run(2, 500)
	h.settleChecks(t0.Add(2*time.Minute + 50*time.Second))
	if state() != "up" {
		t.Fatalf("one failed run makes no outage: %s", state())
	}
	h.settleChecks(t0.Add(4 * time.Minute))
	if state() != "down" {
		t.Fatalf("two failed runs: %s", state())
	}
	if n := len(h.alertMails(owner)); n != 1 {
		t.Fatalf("down mails %d", n)
	}
	if h.monitorAlerts(tm.ws, t0.Add(4*time.Minute)) != 0 {
		t.Fatal("the alert fired twice")
	}
	run(3, 200)
	h.settleChecks(t0.Add(5 * time.Minute))
	if state() != "up" || len(h.alertMails(owner)) != 2 || h.monitorAlerts(tm.ws, t0.Add(6*time.Minute)) != 0 {
		t.Fatalf("recovery: state %s, mails %d", state(), len(h.alertMails(owner)))
	}
	alerts := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/alerts?check_id="+id, nil).body["items"].([]any)
	if len(alerts) != 1 || alerts[0].(map[string]any)["state"] != "resolved" {
		t.Fatalf("alerts %v", alerts)
	}
	if n := h.count("SELECT count(*) FROM events WHERE workspace_id = $1 AND type = 'monitor.alert'", tm.ws); n != 2 {
		t.Fatalf("monitor.alert events %d", n)
	}
	if n := h.count("SELECT count(*) FROM analytics_annotations WHERE workspace_id = $1 AND property_id = $2 AND kind = 'alert'", tm.ws, prop); n != 2 {
		t.Fatalf("alert annotations %d", n)
	}
	tg.status.Store(500)
	h.runCheck(tm.ws, id, t0.Add(4*time.Minute))
	tg.status.Store(200)
	results := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/checks/"+id+"/results", nil).body["items"].([]any)
	if len(results) != 5 || results[0].(map[string]any)["state"] != "pending" || results[0].(map[string]any)["ok"] != false {
		t.Fatalf("results %v", results)
	}
	retried := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/checks/"+id+"/results?limit=1&before="+t0.Add(3*time.Minute+30*time.Second).Format(time.RFC3339), nil).body["items"].([]any)
	if len(retried) != 1 || retried[0].(map[string]any)["state"] != "up" || retried[0].(map[string]any)["retried"] != false {
		t.Fatalf("results before %v", retried)
	}
	h.settleChecks(t0.Add(6 * time.Minute))

	q := map[string]any{"dataset": "uptime", "metrics": []string{"runs", "failures", "uptime", "downtime", "up", "response_time_p95", "avg_response_time"},
		"range": map[string]any{"since": t0.Truncate(time.Minute).Format(time.RFC3339), "until": t0.Add(10 * time.Minute).Truncate(time.Minute).Format(time.RFC3339)}}
	ws := tm.owner.expect(http.StatusOK, "POST", "/v1/monitoring/query", q).body
	tot := ws["totals"].(map[string]any)
	if tot["runs"].(float64) != 5 || tot["failures"].(float64) != 3 || tot["downtime"].(float64) != 60 || tot["uptime"].(float64) != 80 || tot["up"].(float64) != 0 ||
		tot["response_time_p95"].(float64) <= 0 {
		t.Fatalf("uptime totals %v", tot)
	}
	byProp := query(tm.owner, prop, q).body["totals"].(map[string]any)
	if fmt.Sprint(byProp) != fmt.Sprint(tot) {
		t.Fatalf("property %v, workspace %v", byProp, tot)
	}
	q["dimensions"] = []string{"time:minute"}
	series := tm.owner.expect(http.StatusOK, "POST", "/v1/monitoring/query", q).body["rows"].([]any)
	if len(series) != 10 || series[0].(map[string]any)["dimensions"].([]any)[0] != "2026-03-03T10:00+00:00" ||
		series[2].(map[string]any)["metrics"].(map[string]any)["failures"].(float64) != 1 {
		t.Fatalf("minutes %v", series)
	}
	q["dimensions"] = []string{"check"}
	q["filters"] = []map[string]any{{"dimension": "check", "op": "is", "values": []string{id}}}
	rows := tm.owner.expect(http.StatusOK, "POST", "/v1/monitoring/query", q).body["rows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["dimensions"].([]any)[0] != id {
		t.Fatalf("by check %v", rows)
	}

	day := map[string]any{"dataset": "uptime", "metrics": []string{"runs", "failures", "uptime", "downtime"}, "range": map[string]any{"from": "2026-03-03", "to": "2026-03-03"}}
	before := tm.owner.expect(http.StatusOK, "POST", "/v1/monitoring/query", day).body
	h.clock.Set(time.Date(2026, 3, 4, 1, 0, 0, 0, time.UTC))
	if err := h.srv.MaintainAnalytics(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM monitor_check_days WHERE check_id = $1", id); n != 1 {
		t.Fatalf("check days %d", n)
	}
	after := tm.owner.expect(http.StatusOK, "POST", "/v1/monitoring/query", day).body
	if fmt.Sprint(before["totals"]) != fmt.Sprint(after["totals"]) || after["meta"].(map[string]any)["served_from"] != "rollups" {
		t.Fatalf("before the fold %v, after %v", before, after)
	}

	key := tm.apiKey(h)
	out, errText := callTool(t, h.mcpSession(key.bearer), "uptime_checks", map[string]any{"period": "7d"})
	if errText != "" {
		t.Fatal(errText)
	}
	week := tm.owner.expect(http.StatusOK, "POST", "/v1/monitoring/query", map[string]any{"dataset": "uptime", "metrics": []string{"uptime", "runs"},
		"dimensions": []string{"check"}, "range": map[string]any{"period": "7d"}}).body["rows"].([]any)
	for _, ch := range out["checks"].([]any) {
		m := ch.(map[string]any)
		if m["id"] != id {
			continue
		}
		if len(week) != 1 || m["metrics"].(map[string]any)["uptime"] != week[0].(map[string]any)["metrics"].(map[string]any)["uptime"] ||
			m["metrics"].(map[string]any)["runs"] != week[0].(map[string]any)["metrics"].(map[string]any)["runs"] {
			t.Fatalf("mcp %v, query %v", m, week)
		}
	}

	tm.owner.expect(http.StatusNoContent, "DELETE", "/v1/monitoring/checks/"+id, nil)
	if n := h.count("SELECT count(*) FROM monitor_check_results WHERE check_id = $1", id) + h.count("SELECT count(*) FROM monitor_check_days WHERE check_id = $1", id) +
		h.count("SELECT count(*) FROM monitor_alert_rules WHERE check_id = $1", id); n != 0 {
		t.Fatalf("rows left of a deleted check: %d", n)
	}
}

func TestMonitorUnknownWhenMostChecksFail(t *testing.T) {
	h, _ := monitorHarness(t, true, nil)
	tm := newTeam(t, h)
	good, bad := newTarget(t), newTarget(t)
	bad.status.Store(503)
	at := time.Date(2026, 3, 5, 8, 0, 3, 0, time.UTC)
	var ids []string
	for i := range 20 {
		tg := good
		if i < 11 {
			tg = bad
		}
		ids = append(ids, createCheck(tm.owner, map[string]any{"name": fmt.Sprint("Check ", i), "url": tg.url(fmt.Sprintf("u%d.test", i)),
			"failures_before_down": 1})["id"].(string))
	}
	for _, id := range ids {
		h.runCheck(tm.ws, id, at)
	}
	h.settleChecks(at.Add(2 * time.Minute))
	if n := h.count("SELECT count(*) FROM monitor_check_results WHERE workspace_id = $1 AND state = 'unknown'", tm.ws); n != 20 {
		t.Fatalf("unknown runs %d", n)
	}
	for _, id := range ids {
		if s := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/checks/"+id, nil).str("state"); s != "unknown" {
			t.Fatalf("a check moved on an unknown minute: %s", s)
		}
	}
	q := map[string]any{"dataset": "uptime", "metrics": []string{"runs", "failures"}, "range": map[string]any{"since": "2026-03-05T08:00:00Z", "until": "2026-03-05T08:02:00Z"}}
	if tot := tm.owner.expect(http.StatusOK, "POST", "/v1/monitoring/query", q).body["totals"].(map[string]any); tot["runs"].(float64) != 0 {
		t.Fatalf("unknown runs counted: %v", tot)
	}
	for _, id := range ids[11:] {
		h.runCheck(tm.ws, id, at.Add(time.Minute))
	}
	h.settleChecks(at.Add(3 * time.Minute))
	if s := tm.owner.expect(http.StatusOK, "GET", "/v1/monitoring/checks/"+ids[19], nil).str("state"); s != "up" {
		t.Fatalf("a quiet minute is judged: %s", s)
	}
	if m := len(h.alertMails(personEmail(tm.owner))); m != 0 {
		t.Fatalf("mails about unknown minutes: %d", m)
	}
}

func TestMonitorChecksAndRulesStayInTheirWorkspace(t *testing.T) {
	h, _ := monitorHarness(t, true, nil)
	tm, other := newTeam(t, h), newTeam(t, h)
	tg := newTarget(t)
	prop := createAPIProperty(tm.owner, map[string]any{"name": "Private API"})
	id := createCheck(tm.owner, map[string]any{"name": "Mine", "url": tg.url("mine.test"), "property_id": prop})["id"].(string)
	rule := tm.owner.expect(http.StatusCreated, "POST", "/v1/monitoring/alert-rules", map[string]any{"kind": "p95", "property_id": prop, "threshold": 300}).str("id")
	for _, r := range []struct{ method, path string }{
		{"GET", "/v1/monitoring/checks/" + id}, {"PATCH", "/v1/monitoring/checks/" + id}, {"DELETE", "/v1/monitoring/checks/" + id},
		{"GET", "/v1/monitoring/checks/" + id + "/results"}, {"GET", "/v1/monitoring/alert-rules/" + rule}, {"PATCH", "/v1/monitoring/alert-rules/" + rule},
		{"DELETE", "/v1/monitoring/alert-rules/" + rule},
	} {
		other.owner.expect(http.StatusNotFound, r.method, r.path, map[string]any{"name": "taken"})
	}
	other.owner.expect(http.StatusNotFound, "POST", "/v1/monitoring/checks", map[string]any{"name": "x", "url": tg.url("x.test"), "property_id": prop})
	other.owner.expect(http.StatusNotFound, "POST", "/v1/monitoring/alert-rules", map[string]any{"kind": "check_state", "check_id": id})
	if items := other.owner.expect(http.StatusOK, "GET", "/v1/monitoring/checks", nil).body["items"].([]any); len(items) != 0 {
		t.Fatalf("checks of another workspace: %v", items)
	}
	if items := other.owner.expect(http.StatusOK, "GET", "/v1/monitoring/alert-rules", nil).body["items"].([]any); len(items) != 0 {
		t.Fatalf("rules of another workspace: %v", items)
	}
	h.runCheck(tm.ws, id, time.Date(2026, 3, 6, 7, 0, 1, 0, time.UTC))
	h.settleChecks(time.Date(2026, 3, 6, 7, 3, 0, 0, time.UTC))
	q := map[string]any{"dataset": "uptime", "metrics": []string{"runs"}, "dimensions": []string{"check"}, "range": map[string]any{"period": "7d"},
		"filters": []map[string]any{{"dimension": "check", "op": "is", "values": []string{id}}}}
	if rows := other.owner.expect(http.StatusOK, "POST", "/v1/monitoring/query", q).body["rows"].([]any); len(rows) != 0 {
		t.Fatalf("runs of another workspace: %v", rows)
	}
	if rows := tm.owner.expect(http.StatusOK, "POST", "/v1/monitoring/query", q).body["rows"].([]any); len(rows) != 1 {
		t.Fatalf("own runs: %v", rows)
	}
	if items := tm.agent.expect(http.StatusOK, "GET", "/v1/monitoring/checks", nil).body["items"].([]any); len(items) != 1 {
		t.Fatalf("an agent reads checks: %v", items)
	}
	tm.agent.expect(http.StatusNotFound, "GET", "/v1/monitoring/alert-rules/"+rule, nil)
	tm.owner.expect(http.StatusBadRequest, "POST", "/v1/monitoring/alert-rules", map[string]any{"kind": "p95", "property_id": prop, "threshold": 300,
		"recipients": []string{tm.agentID}})
}

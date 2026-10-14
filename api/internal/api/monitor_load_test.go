package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/productdevbook/yuva/api/internal/api"
	"github.com/productdevbook/yuva/api/internal/monitor"
	"github.com/productdevbook/yuva/api/internal/store"
)

// The API monitoring load test runs only when YUVA_TEST_MONITOR_LOAD is set; YUVA_TEST_MONITOR_REPORT,
// when set, is a file the report is written to. It writes 30 days of folded rows for 300M requests a
// month on one property (500 routes) and then drives the records endpoint in bursts of
// YUVA_TEST_MONITOR_RATE records a second (2000) for YUVA_TEST_MONITOR_SECONDS (30), from 5,000
// consumers, with the aggregator flushing every 10 seconds.

const (
	loadRoutes      = 500
	loadConsumers   = 5000
	loadMonthly     = 300_000_000
	loadDays        = 30
	loadStatusesPer = 4
)

func TestMonitorLoad(t *testing.T) {
	if os.Getenv("YUVA_TEST_MONITOR_LOAD") == "" {
		t.Skip("YUVA_TEST_MONITOR_LOAD is not set")
	}
	h := newHarnessWith(t, func(d *api.Deps) {
		d.Analytics.Monitor.RecordsPerMinute = 1 << 30
	})
	ctx := context.Background()
	today := monitor.LocalDay(time.Now(), time.UTC)
	now := today.Add(16*time.Hour + 30*time.Second)
	h.clock.Set(now)
	rep := &loadReport{}
	rep.add(t, "# API monitoring load test, %s, Go %s, %d CPUs", time.Now().UTC().Format(time.DateOnly), runtime.Version(), runtime.NumCPU())
	tm := newTeam(t, h)
	id := createAPIProperty(tm.owner, map[string]any{"name": "Load API"})
	ws, prop := uuid.MustParse(tm.ws), uuid.MustParse(id)
	st := &monitor.PGStore{Pool: h.st.Pool}

	start := time.Now()
	rows := writeMonitorHistory(t, h, st, ws, prop, today, now)
	rep.add(t, "## History: %d requests a month, %d days and today's hours, %d rows, written in %v", loadMonthly, loadDays-1, rows, time.Since(start).Round(time.Second))

	backend := tm.apiKey(h)
	loadMonitorIngest(t, h, rep, backend.bearer, id)

	end := now.Truncate(time.Minute).Add(time.Minute)
	views := []struct {
		name   string
		q      monitor.Query
		target time.Duration
	}{
		{"Overview, 30 days by day: requests, error rates, p95", monitor.Query{Start: monitor.DayStart(today.AddDate(0, 0, 1-loadDays), time.UTC), End: monitor.DayStart(today.AddDate(0, 0, 1), time.UTC),
			Days: true, Dimensions: []string{"time:day"}, Metrics: []string{"requests", "error_rate", "client_error_rate", "p95"}}, time.Second},
		{"Routes, 30 days: requests, error rate, p95", monitor.Query{Start: monitor.DayStart(today.AddDate(0, 0, 1-loadDays), time.UTC), End: monitor.DayStart(today.AddDate(0, 0, 1), time.UTC),
			Days: true, Dimensions: []string{"route"}, Metrics: []string{"requests", "error_rate", "p50", "p95", "p99"}}, time.Second},
		{"Last 60 minutes by minute", monitor.Query{Start: end.Add(-time.Hour), End: end, Dimensions: []string{"time:minute"},
			Metrics: []string{"requests", "error_rate", "p95"}}, time.Second},
		{"A route's 30 days by status", monitor.Query{Start: monitor.DayStart(today.AddDate(0, 0, 1-loadDays), time.UTC), End: monitor.DayStart(today.AddDate(0, 0, 1), time.UTC),
			Days: true, Dimensions: []string{"status", "time:day"}, Metrics: []string{"requests", "p95"},
			Filters: []monitor.Filter{{Dimension: "route", Op: "is", Values: []string{"/v1/things7/{id}"}}}}, time.Second},
	}
	rep.add(t, "| View | p50 | p95 | target (p95) |")
	rep.add(t, "|---|---|---|---|")
	for _, v := range views {
		q := v.q
		q.WorkspaceID, q.PropertyID, q.Zone, q.Now, q.MinuteFrom = ws, prop, monitor.FixedZone(time.UTC), now, today.AddDate(0, 0, -7)
		var times []time.Duration
		var res monitor.Result
		for range 20 {
			s := time.Now()
			var err error
			if res, err = st.Query(ctx, q); err != nil {
				t.Fatal(err)
			}
			times = append(times, time.Since(s))
		}
		p50, p95 := quantile(times, 0.5), quantile(times, 0.95)
		rep.add(t, "| %s (%d rows, %s) | %v | %v | < %v |", v.name, len(res.Rows), res.ServedFrom, p50.Round(time.Millisecond), p95.Round(time.Millisecond), v.target)
		if p95 > v.target {
			t.Errorf("%s: p95 %v over the target %v", v.name, p95, v.target)
		}
	}
	var size int64
	if err := h.st.Pool.QueryRow(ctx, `SELECT coalesce(sum(pg_total_relation_size(c.oid)), 0)::bigint FROM pg_class c
		WHERE c.relkind = 'r' AND c.relname LIKE 'monitor\_%'`).Scan(&size); err != nil {
		t.Fatal(err)
	}
	rep.add(t, "- monitor tables %.0f MiB with indexes", float64(size)/(1<<20))
	if path := os.Getenv("YUVA_TEST_MONITOR_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(strings.Join(rep.lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.st.Pool.Exec(ctx, "DELETE FROM monitor_minute WHERE workspace_id = $1", ws); err != nil {
		t.Error(err)
	}
	if err := h.st.DeleteMonitorRollups(ctx, store.DeleteMonitorRollupsParams{WorkspaceID: ws, PropertyID: prop}); err != nil {
		t.Error(err)
	}
}

func loadRoute(i int) string { return fmt.Sprintf("/v1/things%d/{id}", i) }

func loadDuration(r *rand.Rand, route int) int64 {
	return int64(math.Exp(r.NormFloat64()*0.8+8+float64(route%7)*0.3)) + 1
}

// writeMonitorHistory writes folded days before today and folded hours of today up to the last
// full hour, as the folds would have, and returns how many rows it wrote.
func writeMonitorHistory(t *testing.T, h *harness, st *monitor.PGStore, ws, prop uuid.UUID, today, now time.Time) int {
	t.Helper()
	ctx := context.Background()
	if err := st.EnsureHourPartitions(ctx, today.AddDate(0, 0, -loadDays), today); err != nil {
		t.Fatal(err)
	}
	var routes []monitor.NameRequest
	for i := range loadRoutes {
		routes = append(routes, monitor.NameRequest{Kind: monitor.KindRoute, Value: loadRoute(i)})
	}
	routes = append(routes, monitor.NameRequest{Kind: monitor.KindEnvironment, Value: monitor.DefaultEnvironment})
	ids, err := st.Names(ctx, ws, prop, routes, now)
	if err != nil {
		t.Fatal(err)
	}
	env := ids[len(ids)-1]
	statuses := []int16{200, 201, 404, 500}
	shares := []float64{0.9, 0.06, 0.03, 0.01}
	perKeyDay := float64(loadMonthly) / 30 / float64(loadRoutes)
	r := rand.New(rand.NewPCG(3, 4))
	agg := func(route int, n int64, status int16) monitor.Agg {
		a := monitor.NewAgg()
		samples := min(n, 400)
		for range samples {
			rec := monitor.Record{DurationUS: loadDuration(r, route), Status: status, RequestBytes: 120, ResponseBytes: int64(500 + r.IntN(2000)), Weight: n / samples}
			a.Add(&rec)
		}
		return a
	}
	cols := []string{"workspace_id", "property_id", "day", "environment", "service", "route", "method", "status", "api_version", "source", "platform",
		"requests", "errors", "client_errors", "sampled", "duration_sum", "duration_min", "duration_max", "durations", "request_bytes", "response_bytes", "response_sizes"}
	vals := func(a monitor.Agg) []any {
		return []any{a.Requests, a.Errors, a.ClientErrors, int64(0), a.DurationSum, a.DurationMin, a.DurationMax, a.Durations.Encode(), a.RequestBytes, a.ResponseBytes, a.Sizes.Encode()}
	}
	written := 0
	var days, hours [][]any
	var marks [][]any
	for d := today.AddDate(0, 0, 1-loadDays); d.Before(today); d = d.AddDate(0, 0, 1) {
		for route := range loadRoutes {
			for si, status := range statuses {
				a := agg(route, int64(perKeyDay*shares[si]), status)
				days = append(days, append([]any{ws, prop, d, env, int32(0), ids[route], monitor.Method("GET"), status, int32(0), int16(monitor.SourceServer), int16(0)}, vals(a)...))
			}
		}
		for hr := range 24 {
			marks = append(marks, []any{ws, prop, d.Add(time.Duration(hr) * time.Hour), d, now, false, false, now})
		}
	}
	for hr := today; hr.Add(time.Hour).Before(now.Truncate(time.Hour).Add(time.Second)); hr = hr.Add(time.Hour) {
		for route := range loadRoutes {
			for si, status := range statuses {
				a := agg(route, int64(perKeyDay*shares[si]/24), status)
				hours = append(hours, append([]any{ws, prop, hr, env, int32(0), ids[route], monitor.Method("GET"), status, int32(0), int16(monitor.SourceServer), int16(0)}, vals(a)...))
			}
		}
		marks = append(marks, []any{ws, prop, hr, today, now, false, false, nil})
	}
	for _, c := range []struct {
		table string
		cols  []string
		rows  [][]any
	}{
		{"monitor_day", cols, days},
		{"monitor_hour", append([]string{"workspace_id", "property_id", "hour"}, cols[3:]...), hours},
		{"monitor_rollup_hours", []string{"workspace_id", "property_id", "hour", "day", "built_at", "dirty", "capped", "day_built_at"}, marks},
	} {
		n, err := h.st.Pool.CopyFrom(ctx, pgx.Identifier{c.table}, c.cols, pgx.CopyFromRows(c.rows))
		if err != nil {
			t.Fatalf("%s: %v", c.table, err)
		}
		written += int(n)
	}
	if _, err := h.st.Pool.Exec(ctx, "ANALYZE monitor_day, monitor_hour, monitor_rollup_hours"); err != nil {
		t.Fatal(err)
	}
	return written
}

// loadMonitorIngest sends bursts of records for the configured time while a goroutine flushes every
// ten seconds, then waits until the last records are visible to a query.
func loadMonitorIngest(t *testing.T, h *harness, rep *loadReport, key, id string) {
	rate := envInt("YUVA_TEST_MONITOR_RATE", 2000)
	seconds := envInt("YUVA_TEST_MONITOR_SECONDS", 30)
	batch := 100
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var flushes []time.Duration
	var flushMu sync.Mutex
	flushDone := make(chan struct{})
	go func() {
		defer close(flushDone)
		tick := time.NewTicker(monitor.FlushInterval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
			s := time.Now()
			if err := h.srv.FlushMonitor(context.Background()); err != nil {
				t.Errorf("flush: %v", err)
				return
			}
			flushMu.Lock()
			flushes = append(flushes, time.Since(s))
			flushMu.Unlock()
		}
	}()
	client := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: 64, MaxConnsPerHost: 64}}
	url := h.url + "/v1/analytics/properties/" + id + "/requests"
	var latMu sync.Mutex
	var latencies []time.Duration
	var accepted, refused atomic.Int64
	mem := sampleMemory()
	cpu0, began := cpuSeconds(), time.Now()
	jobs := make(chan int, rate)
	var wg sync.WaitGroup
	at := h.clock.Now().Format(time.RFC3339Nano)
	for w := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewPCG(uint64(w), 9))
			for range jobs {
				recs := make([]map[string]any, batch)
				for i := range recs {
					route := r.IntN(loadRoutes)
					status := 200
					switch x := r.IntN(100); {
					case x < 3:
						status = 404
					case x < 4:
						status = 500
					}
					recs[i] = map[string]any{"method": "GET", "route": loadRoute(route), "status": status, "duration_us": loadDuration(r, route), "time": at,
						"consumer": fmt.Sprintf("client-%d", r.IntN(loadConsumers)), "response_size": 500 + r.IntN(2000)}
				}
				body, _ := json.Marshal(map[string]any{"records": recs})
				req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
				req.Header.Set("Authorization", "Bearer "+key)
				req.Header.Set("Content-Type", "application/json")
				s := time.Now()
				res, err := client.Do(req)
				took := time.Since(s)
				if err != nil {
					refused.Add(int64(batch))
					continue
				}
				res.Body.Close()
				latMu.Lock()
				latencies = append(latencies, took)
				latMu.Unlock()
				if res.StatusCode == http.StatusAccepted {
					accepted.Add(int64(batch))
				} else {
					refused.Add(int64(batch))
				}
			}
		}()
	}
	per := rate / batch / 10
	tick := time.NewTicker(100 * time.Millisecond)
	for time.Since(began) < time.Duration(seconds)*time.Second {
		<-tick.C
		for range max(per, 1) {
			jobs <- 1
		}
	}
	tick.Stop()
	close(jobs)
	wg.Wait()
	lastSent := time.Now()
	sent := lastSent.Sub(began)
	want := accepted.Load()
	q := map[string]any{"dataset": "api", "metrics": []string{"requests"}, "range": map[string]any{"period": "15m"}}
	var visible time.Duration
	owner := h.client()
	owner.bearer = key
	frozen := h.clock.Now()
	for {
		h.clock.Set(frozen.Add(time.Since(lastSent)))
		res := owner.do(http.MethodPost, "/v1/analytics/properties/"+id+"/query", q)
		if res.status == http.StatusOK && int64(totals(res)["requests"].(float64)) >= want {
			visible = time.Since(lastSent)
			break
		}
		if time.Since(lastSent) > time.Minute {
			t.Fatalf("records sent were not visible within a minute: %s", res.raw)
		}
		time.Sleep(250 * time.Millisecond)
	}
	cancel()
	<-flushDone
	cpu := cpuSeconds() - cpu0
	total, heap := mem.end()
	flushMu.Lock()
	defer flushMu.Unlock()
	rep.add(t, "## Ingest: %d records/s in batches of %d for %ds, 500 routes, 5,000 consumers", rate, batch, seconds)
	rep.add(t, "- accepted %d (%.0f/s), refused %d; CPU %.1f cores (server and load generator in one process)", want, float64(want)/sent.Seconds(), refused.Load(), cpu/sent.Seconds())
	rep.add(t, "- request p50 %v, p99 %v (target p99 < 100ms); visible to a query %v after the last record (target < 15s)",
		quantile(latencies, 0.5).Round(time.Microsecond), quantile(latencies, 0.99).Round(time.Microsecond), visible.Round(time.Millisecond))
	rep.add(t, "- flush p50 %v, max %v over %d flushes; peak Go memory %d MiB (heap objects %d MiB)", quantile(flushes, 0.5).Round(time.Millisecond),
		slices.Max(append(flushes, 0)).Round(time.Millisecond), len(flushes), total>>20, heap>>20)
	if p99 := quantile(latencies, 0.99); p99 > 100*time.Millisecond {
		t.Errorf("ingest p99 %v over 100ms", p99)
	}
	if visible > 15*time.Second {
		t.Errorf("records visible after %v, over 15s", visible)
	}
	if refused.Load() > 0 {
		t.Errorf("%d records refused", refused.Load())
	}
}

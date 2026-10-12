package api_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"runtime"
	"runtime/metrics"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/api"
)

// The analytics load test runs only when YUVA_TEST_ANALYTICS_LOAD names the monthly volumes to
// measure (such as "1M,10M"); YUVA_TEST_ANALYTICS_REPORT, when set, is a file the report is
// written to. YUVA_TEST_ANALYTICS_INGEST is the ingest rate in events per second (5000) and
// YUVA_TEST_ANALYTICS_INGEST_SECONDS how long it is held (60).

const loadRawDays = 90

type loadReport struct {
	mu    sync.Mutex
	lines []string
}

func (r *loadReport) add(t *testing.T, format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	t.Log(line)
	r.mu.Lock()
	r.lines = append(r.lines, line)
	r.mu.Unlock()
}

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}

func TestAnalyticsLoad(t *testing.T) {
	sizes := os.Getenv("YUVA_TEST_ANALYTICS_LOAD")
	if sizes == "" {
		t.Skip("YUVA_TEST_ANALYTICS_LOAD is not set")
	}
	h := newHarnessWith(t, func(d *api.Deps) {
		d.Analytics.PropertyEventsPerMinute = 1 << 30
		d.Analytics.Buffer = 200_000
		d.Analytics.RawDays = loadRawDays
	})
	h.clock.Set(time.Now().UTC())
	rep := &loadReport{}
	rep.add(t, "# Analytics load test, %s, Go %s, %d CPUs", time.Now().UTC().Format(time.DateOnly), runtime.Version(), runtime.NumCPU())
	loadIngest(t, h, rep)
	for _, size := range strings.Split(sizes, ",") {
		size = strings.TrimSpace(size)
		perMonth, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(size, "M"), "m"), 64)
		if err != nil {
			t.Fatalf("YUVA_TEST_ANALYTICS_LOAD: %q", size)
		}
		loadQueries(t, h, rep, size, int(perMonth*1_000_000))
	}
	if path := os.Getenv("YUVA_TEST_ANALYTICS_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(strings.Join(rep.lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

type memSampler struct {
	peakTotal, peakHeap atomic.Uint64
	stop                chan struct{}
	done                chan struct{}
}

func sampleMemory() *memSampler {
	m := &memSampler{stop: make(chan struct{}), done: make(chan struct{})}
	samples := []metrics.Sample{{Name: "/memory/classes/total:bytes"}, {Name: "/memory/classes/heap/objects:bytes"}}
	go func() {
		defer close(m.done)
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			metrics.Read(samples)
			if v := samples[0].Value.Uint64(); v > m.peakTotal.Load() {
				m.peakTotal.Store(v)
			}
			if v := samples[1].Value.Uint64(); v > m.peakHeap.Load() {
				m.peakHeap.Store(v)
			}
			select {
			case <-m.stop:
				return
			case <-tick.C:
			}
		}
	}()
	return m
}

func (m *memSampler) end() (total, heap uint64) {
	close(m.stop)
	<-m.done
	return m.peakTotal.Load(), m.peakHeap.Load()
}

func cpuSeconds() float64 {
	var ru syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return float64(ru.Utime.Sec+ru.Stime.Sec) + float64(ru.Utime.Usec+ru.Stime.Usec)/1e6
}

func quantile(d []time.Duration, q float64) time.Duration {
	if len(d) == 0 {
		return 0
	}
	s := slices.Clone(d)
	slices.Sort(s)
	return s[min(len(s)-1, int(float64(len(s))*q))]
}

var loadAgents = []string{uaMac, uaPixel, uaWindows,
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.6 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
}

// loadIngest drives the collection endpoint at a steady rate from many addresses while a flusher
// writes every second, and reports what it took. It runs first, so the memory it reports is its
// own, and deletes its rows afterwards and compacts the day they were in, so the query views read
// one property's days as planned.
func loadIngest(t *testing.T, h *harness, rep *loadReport) {
	tm := newTeam(t, h)
	id, key := createProperty(tm.owner, map[string]any{"name": "Load ingest", "domains": []string{"example.com"}})
	day := analytics.LocalDay(h.clock.Now(), time.UTC)
	defer func() {
		for _, table := range []string{analytics.EventsTable, analytics.VisitPagesTable, analytics.VisitsTable} {
			if _, err := h.st.Pool.Exec(context.Background(), "DELETE FROM "+table+" WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); err != nil {
				t.Error(err)
			}
			if _, err := h.st.Pool.Exec(context.Background(), "VACUUM FULL "+analytics.PartitionName(table, day)); err != nil {
				t.Error(err)
			}
		}
	}()
	rate := envInt("YUVA_TEST_ANALYTICS_INGEST", 5000)
	seconds := envInt("YUVA_TEST_ANALYTICS_INGEST_SECONDS", 60)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var flushes []time.Duration
	var flushMu sync.Mutex
	flushDone := make(chan struct{})
	go func() {
		defer close(flushDone)
		tick := time.NewTicker(analytics.FlushInterval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
			start := time.Now()
			if err := h.srv.FlushAnalytics(context.Background()); err != nil {
				t.Errorf("flush: %v", err)
				return
			}
			flushMu.Lock()
			flushes = append(flushes, time.Since(start))
			flushMu.Unlock()
		}
	}()
	client := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: 256, MaxConnsPerHost: 256}}
	url := h.url + "/client/v1/analytics/" + key
	var accepted, refused, busy atomic.Int64
	mem := sampleMemory()
	cpu0, start := cpuSeconds(), time.Now()
	jobs := make(chan int, rate)
	var wg sync.WaitGroup
	for w := 0; w < 64; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewPCG(uint64(w), 7))
			for i := range jobs {
				ip := fmt.Sprintf("100.%d.%d.%d", 64+r.IntN(64), r.IntN(256), r.IntN(256))
				body := fmt.Sprintf(`{"n":"pageview","u":"https://example.com/p/%d?utm_source=s%d","r":"https://www.google.com/","w":%d}`, r.IntN(500), i%40, 360+r.IntN(1600))
				req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
				req.Header.Set("Content-Type", "text/plain")
				req.Header.Set("Origin", "https://example.com")
				req.Header.Set("Sec-Fetch-Site", "cross-site")
				req.Header.Set("Accept-Language", "en-US,en;q=0.9")
				req.Header.Set("User-Agent", loadAgents[i%len(loadAgents)])
				req.Header.Set("X-Forwarded-For", ip)
				res, err := client.Do(req)
				if err != nil {
					refused.Add(1)
					continue
				}
				res.Body.Close()
				switch res.StatusCode {
				case http.StatusAccepted:
					accepted.Add(1)
				case http.StatusServiceUnavailable:
					busy.Add(1)
				default:
					refused.Add(1)
				}
			}
		}()
	}
	tick := time.NewTicker(10 * time.Millisecond)
	per := rate / 100
	n := 0
	for time.Since(start) < time.Duration(seconds)*time.Second {
		<-tick.C
		for i := 0; i < per; i++ {
			jobs <- n
			n++
		}
	}
	tick.Stop()
	close(jobs)
	wg.Wait()
	sent := time.Since(start)
	time.Sleep(2 * analytics.FlushInterval)
	cancel()
	<-flushDone
	if err := h.srv.FlushAnalytics(context.Background()); err != nil {
		t.Fatal(err)
	}
	cpu := cpuSeconds() - cpu0
	total, heap := mem.end()
	flushMu.Lock()
	defer flushMu.Unlock()
	rep.add(t, "## Ingest: %d events/s asked for %ds", rate, seconds)
	rep.add(t, "- accepted %d (%.0f/s), 503 %d, other %d; CPU %.1f cores (server and load generator in one process)",
		accepted.Load(), float64(accepted.Load())/sent.Seconds(), busy.Load(), refused.Load(), cpu/sent.Seconds())
	rep.add(t, "- flush p50 %v, p95 %v, max %v over %d flushes", quantile(flushes, 0.5), quantile(flushes, 0.95), slices.Max(append(flushes, 0)), len(flushes))
	rep.add(t, "- peak Go memory %d MiB (heap objects %d MiB); target under 384 MiB", total>>20, heap>>20)
	if busy.Load() > 0 {
		t.Errorf("the buffer refused %d events", busy.Load())
	}
	if total > 384<<20 {
		t.Errorf("peak memory %d MiB is over 384 MiB", total>>20)
	}
}

// loadQueries writes 90 days of synthetic traffic at perMonth events a month for one property
// straight into the raw tables, rolls the finished days up, and times the views of the plan.
func loadQueries(t *testing.T, h *harness, rep *loadReport, size string, perMonth int) {
	ctx := context.Background()
	tm := newTeam(t, h)
	id, _ := createProperty(tm.owner, map[string]any{"name": "Load " + size, "domains": []string{"example.com"}})
	ws, prop := uuid.MustParse(tm.ws), uuid.MustParse(id)
	today := analytics.LocalDay(h.clock.Now(), time.UTC)
	now := today.Add(16 * time.Hour)
	h.clock.Set(now)
	first := today.AddDate(0, 0, 1-loadRawDays)
	pg := &analytics.PGStore{Pool: h.st.Pool}
	if err := pg.EnsurePartitions(ctx, first, today.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	events, visits := 0, 0
	perDay := perMonth / 30
	var wg sync.WaitGroup
	var mu sync.Mutex
	errs := make(chan error, loadRawDays)
	sem := make(chan struct{}, 4)
	for d := first; !d.After(today); d = d.AddDate(0, 0, 1) {
		wg.Add(1)
		sem <- struct{}{}
		go func(day time.Time) {
			defer wg.Done()
			defer func() { <-sem }()
			var until time.Time
			if day.Equal(today) {
				until = now
			}
			e, v, err := writeLoadDay(ctx, h, ws, prop, day, perDay, until)
			if err != nil {
				errs <- err
				return
			}
			mu.Lock()
			events, visits = events+e, visits+v
			mu.Unlock()
		}(d)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	loaded := time.Since(start)
	if _, err := h.st.Pool.Exec(ctx, "ANALYZE analytics_events, analytics_visit_pages, analytics_visits"); err != nil {
		t.Fatal(err)
	}
	var size64 int64
	if err := h.st.Pool.QueryRow(ctx, `SELECT coalesce(sum(pg_total_relation_size(c.oid)), 0)::bigint FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
		JOIN pg_class p ON p.oid = i.inhparent WHERE p.relname IN ('analytics_events', 'analytics_visit_pages', 'analytics_visits')`).Scan(&size64); err != nil {
		t.Fatal(err)
	}
	start = time.Now()
	var rollups []time.Duration
	var rmu sync.Mutex
	sem = make(chan struct{}, 2)
	for d := first; d.Before(today); d = d.AddDate(0, 0, 1) {
		wg.Add(1)
		sem <- struct{}{}
		go func(day time.Time) {
			defer wg.Done()
			defer func() { <-sem }()
			s := time.Now()
			if err := h.srv.RollupAnalyticsDay(ctx, ws, prop, day); err != nil {
				t.Error(err)
			}
			rmu.Lock()
			rollups = append(rollups, time.Since(s))
			rmu.Unlock()
		}(d)
	}
	wg.Wait()
	rolled := time.Since(start)
	start = time.Now()
	if err := h.srv.FoldAnalyticsDay(ctx, ws, prop, today); err != nil {
		t.Fatal(err)
	}
	folded := time.Since(start)
	if _, err := h.st.Pool.Exec(ctx, "VACUUM ANALYZE analytics_daily, analytics_hourly, analytics_rollup_days, analytics_partial_daily, analytics_partial_hourly, analytics_partial_days"); err != nil {
		t.Fatal(err)
	}
	rep.add(t, "## %s events a month: 90 days, %d events and %d visits (%.1f GB raw with indexes, all partitions)", size, events, visits, float64(size64)/1e9)
	rep.add(t, "- written in %v; %d days rolled up in %v (2 workers), one day p50 %v, max %v; today folded up to %s in %v", loaded.Round(time.Second), len(rollups),
		rolled.Round(time.Second), quantile(rollups, 0.5).Round(time.Millisecond), slices.Max(append(rollups, 0)).Round(time.Millisecond),
		now.Add(-analytics.FoldMargin).Format("15:04"), folded.Round(time.Millisecond))

	base := analytics.Query{WorkspaceID: ws, PropertyID: prop, Timezone: "UTC", To: today, RawFrom: first,
		Metrics: []string{"visitors", "visits", "pageviews", "bounce_rate", "visit_duration"}, SortBy: []string{"visitors"}}
	with := func(from time.Time, dims []string, filters ...analytics.Filter) analytics.Query {
		q := base
		q.From, q.Dimensions, q.Filters = from, dims, filters
		return q
	}
	overview := func(from time.Time) []analytics.Query {
		return []analytics.Query{with(from, []string{"time:day"}), with(from, []string{"page"}), with(from, []string{"source"}), with(from, []string{"channel"}),
			with(from, []string{"country"}), with(from, []string{"device"}), with(from, []string{"browser"})}
	}
	views := []struct {
		name    string
		qs      []analytics.Query
		target1 time.Duration
		target2 time.Duration
	}{
		{"Overview, 30 days, no filter (rollups + today folded)", overview(today.AddDate(0, 0, -29)), 150 * time.Millisecond, 300 * time.Millisecond},
		{"Overview, 12 months, no filter (rollups)", overview(today.AddDate(-1, 0, 1)), 150 * time.Millisecond, 250 * time.Millisecond},
		{"Pages, 30 days, country is TR (rollup pairs)", []analytics.Query{with(today.AddDate(0, 0, -29), []string{"page"}, analytics.Filter{Dimension: "country", Op: "is", Values: []string{"TR"}})}, 200 * time.Millisecond, time.Second},
		{"Sources, 30 days, device is mobile (raw)", []analytics.Query{with(today.AddDate(0, 0, -29), []string{"source"}, analytics.Filter{Dimension: "device", Op: "is", Values: []string{"mobile"}})}, 200 * time.Millisecond, time.Second},
		{"Channel by country, 7 days, two filters (raw)", []analytics.Query{with(today.AddDate(0, 0, -6), []string{"channel", "country"},
			analytics.Filter{Dimension: "page", Op: "matches", Values: []string{"/docs/**"}}, analytics.Filter{Dimension: "browser", Op: "is", Values: []string{"Chrome"}})}, 200 * time.Millisecond, 600 * time.Millisecond},
	}
	rep.add(t, "| View | p50 | p95 | target |")
	rep.add(t, "|---|---|---|---|")
	for _, v := range views {
		target := v.target1
		if perMonth > 1_000_000 {
			target = v.target2
		}
		var times []time.Duration
		for i := 0; i < 20; i++ {
			s := time.Now()
			var g sync.WaitGroup
			slots := make(chan struct{}, 4)
			for _, q := range v.qs {
				g.Add(1)
				slots <- struct{}{}
				go func(q analytics.Query) {
					defer g.Done()
					defer func() { <-slots }()
					if _, err := pg.Query(ctx, q); err != nil {
						t.Error(err)
					}
				}(q)
			}
			g.Wait()
			times = append(times, time.Since(s))
		}
		p50, p95 := quantile(times, 0.5), quantile(times, 0.95)
		rep.add(t, "| %s | %v | %v | < %v |", v.name, p50.Round(time.Millisecond), p95.Round(time.Millisecond), target)
		if p50 > target {
			t.Errorf("%s at %s: p50 %v over the target %v", v.name, size, p50, target)
		}
	}
	var times []time.Duration
	for i := 0; i < 20; i++ {
		s := time.Now()
		if _, err := pg.Realtime(ctx, ws, prop, time.UTC, h.clock.Now()); err != nil {
			t.Fatal(err)
		}
		times = append(times, time.Since(s))
	}
	rt := 50 * time.Millisecond
	if perMonth > 1_000_000 {
		rt = 100 * time.Millisecond
	}
	rep.add(t, "| Realtime, last 30 minutes | %v | %v | < %v |", quantile(times, 0.5).Round(time.Millisecond), quantile(times, 0.95).Round(time.Millisecond), rt)
	if quantile(times, 0.5) > rt {
		t.Errorf("realtime at %s: p50 %v over the target %v", size, quantile(times, 0.5), rt)
	}
}

var loadCountries = func() []string {
	var out []string
	for a := 'A'; a <= 'Z' && len(out) < 150; a++ {
		for b := 'A'; b <= 'Z' && len(out) < 150; b += 4 {
			out = append(out, string([]rune{a, b}))
		}
	}
	out[0] = "TR"
	return out
}()

// writeLoadDay copies one day of synthetic visits and their events: Zipf-distributed pages over
// 10,000 paths, 40 sources, 150 countries, visits of 1 to 15 page views with an engagement after
// most of them, 3% bots, busier in the afternoon. A day in progress (until set) keeps only what
// happened by until.
func writeLoadDay(ctx context.Context, h *harness, ws, prop uuid.UUID, day time.Time, perDay int, until time.Time) (int, int, error) {
	r := rand.New(rand.NewPCG(uint64(day.Unix()), uint64(perDay)))
	zipf := rand.NewZipf(rand.New(rand.NewPCG(uint64(day.Unix()), 1)), 1.1, 1, 9999)
	sections := []string{"/blog/", "/docs/", "/p/", "/help/"}
	path := func() string {
		n := zipf.Uint64()
		if n == 0 {
			return "/"
		}
		return sections[n%4] + strconv.FormatUint(n, 10)
	}
	browsers := []string{"Chrome", "Safari", "Firefox", "Edge", "Samsung Browser"}
	oses := []string{"Windows", "MacOS", "iOS", "Android", "Linux"}
	var evRows, visitRows, pageRows [][]any
	events, written := 0, 0
	happened := func(t time.Time) bool { return until.IsZero() || !t.After(until) }
	for events < perDay {
		views := 1 + r.IntN(15)
		if r.IntN(2) == 0 {
			views = 1 + r.IntN(3)
		}
		hour := min(23, max(0, int(r.NormFloat64()*4+15)))
		start := day.Add(time.Duration(hour)*time.Hour + time.Duration(r.IntN(3600))*time.Second)
		visitor, visit := r.Int64(), r.Int64()
		bot := r.IntN(100) < 3
		ts := start
		entry, exit := "", ""
		var engaged int64
		type pageSums struct{ views, engViews, scrollViews, engMS, scrollSum int64 }
		pages := map[string]*pageSums{}
		var order []string
		kept := 0
		for v := 0; v < views; v++ {
			p := path()
			view := happened(ts)
			if view {
				if v == 0 {
					entry = p
				}
				exit = p
				kept++
				if pages[p] == nil {
					pages[p] = &pageSums{}
					order = append(order, p)
				}
				pages[p].views++
				evRows = append(evRows, []any{ws, prop, day, ts, visit, visitor, int16(1), "pageview", "example.com", p, nil, nil, nil, nil, nil, true})
				written++
			}
			events++
			if r.IntN(10) < 7 {
				ms := int32(1000 + r.IntN(120_000))
				scroll := int16(r.IntN(101))
				if at := ts.Add(time.Duration(ms) * time.Millisecond); view && happened(at) {
					ps := pages[p]
					engaged += int64(ms)
					ps.engViews, ps.scrollViews, ps.engMS, ps.scrollSum = ps.engViews+1, ps.scrollViews+1, ps.engMS+int64(ms), ps.scrollSum+int64(scroll)
					evRows = append(evRows, []any{ws, prop, day, at, visit, visitor, int16(4), "engagement", "example.com", p, nil, nil, nil, ms, scroll, false})
					written++
				}
				events++
			}
			ts = ts.Add(time.Duration(10+r.IntN(300)) * time.Second)
		}
		if !until.IsZero() && ts.After(until) {
			ts = until
		}
		for _, p := range order {
			ps := pages[p]
			pageRows = append(pageRows, []any{ws, prop, day, visit, p, int32(ps.views), int32(0), ps.engMS, int32(ps.engViews), ps.scrollSum, int32(ps.scrollViews)})
		}
		src := r.IntN(40)
		channel := int16(1 + src%9)
		source := ""
		if src > 0 {
			source = "source" + strconv.Itoa(src)
		}
		country := loadCountries[min(len(loadCountries)-1, int(r.ExpFloat64()*20))]
		b := r.IntN(len(browsers))
		row := []any{ws, prop, day, visit, visitor, start, ts, "example.com", entry, exit, int32(kept), int32(0), false, engaged,
			channel, source, source, "", source, "", "", "", "", country, "", "", int16(1 + r.IntN(3)), browsers[b], int16(120 + r.IntN(20)),
			oses[r.IntN(len(oses))], int16(10 + r.IntN(8)), int16(1 + r.IntN(4)), "en", int16(1), "", false, nil, bot}
		if kept > 0 {
			visitRows = append(visitRows, row)
		}
	}
	err := pgx.BeginFunc(ctx, h.st.Pool, func(tx pgx.Tx) error {
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{analytics.PartitionName(analytics.EventsTable, day)}, []string{"workspace_id", "property_id", "day", "ts",
			"visit", "visitor", "kind", "name", "hostname", "path", "props", "revenue_minor", "currency", "engaged_ms", "scroll", "interactive"},
			pgx.CopyFromRows(evRows)); err != nil {
			return err
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{analytics.PartitionName(analytics.VisitsTable, day)}, []string{"workspace_id", "property_id", "day", "visit",
			"visitor", "started_at", "ended_at", "hostname", "entry_path", "exit_path", "views", "events", "interactive", "engaged_ms", "channel", "source",
			"referrer_host", "referrer_path", "utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content", "country", "region", "city", "device",
			"browser", "browser_major", "os", "os_major", "screen", "language", "platform", "app_version", "persistent", "contact_id", "bot"},
			pgx.CopyFromRows(visitRows)); err != nil {
			return err
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{analytics.PartitionName(analytics.VisitPagesTable, day)}, []string{"workspace_id", "property_id", "day", "visit",
			"path", "views", "events", "eng_ms", "eng_views", "scroll_sum", "scroll_views"}, pgx.CopyFromRows(pageRows)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO analytics_rollup_days (workspace_id, property_id, day) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, ws, prop, day)
		return err
	})
	return written, len(visitRows), err
}

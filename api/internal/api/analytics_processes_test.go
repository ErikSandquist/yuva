package api_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/analytics"
)

type processes struct {
	t      *testing.T
	a, b   *harness
	ws, id string
	key    string
}

func newProcesses(t *testing.T, at time.Time) *processes {
	a, b := newHarness(t), newHarness(t)
	a.clock.Set(at)
	b.clock.Set(at)
	tm := newTeam(t, a)
	id, key := createProperty(tm.owner, map[string]any{"name": "Two processes", "domains": []string{"example.com"}})
	return &processes{t: t, a: a, b: b, ws: tm.ws, id: id, key: key}
}

func (p *processes) send(on *harness, body map[string]any) {
	p.t.Helper()
	if r := collect(on, p.key, collectReq{origin: "https://example.com", ip: "203.0.113.7", body: body}); r.status != http.StatusAccepted {
		p.t.Fatalf("collect %d %s", r.status, r.raw)
	}
}

type storedVisit struct {
	visit                   int64
	views                   int
	engaged                 int64
	entry, refHost, browser string
	channel                 int16
}

func (p *processes) visits() []storedVisit {
	p.t.Helper()
	rows, err := p.a.st.Pool.Query(context.Background(), `SELECT visit, views, engaged_ms, entry_path, referrer_host, browser, channel
		FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2 ORDER BY started_at`, p.ws, p.id)
	if err != nil {
		p.t.Fatal(err)
	}
	defer rows.Close()
	var out []storedVisit
	for rows.Next() {
		var v storedVisit
		if err := rows.Scan(&v.visit, &v.views, &v.engaged, &v.entry, &v.refHost, &v.browser, &v.channel); err != nil {
			p.t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

func TestAnalyticsVisitIsNotSplitBetweenProcesses(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Second)
	p := newProcesses(t, at)
	p.send(p.a, map[string]any{"n": "pageview", "u": "https://example.com/", "r": "https://www.google.com/search?q=x", "w": 1280})
	p.b.clock.Advance(100 * time.Millisecond)
	p.send(p.b, map[string]any{"n": "pageview", "u": "https://example.com/pricing", "w": 1280})
	p.b.clock.Advance(100 * time.Millisecond)
	p.send(p.b, map[string]any{"n": "engagement", "u": "https://example.com/pricing", "ms": 4200, "sd": 50})
	p.b.flushAnalytics()
	p.a.flushAnalytics()

	v := p.visits()
	if len(v) != 1 || v[0].views != 2 || v[0].entry != "/" || v[0].engaged != 4200 {
		t.Fatalf("one visit across two processes was stored as %+v", v)
	}
	if v[0].refHost != "www.google.com" || v[0].channel != int16(analytics.ChannelOrganic) || v[0].browser != "Safari" {
		t.Fatalf("the visit lost its first event's description when the joining process flushed first: %+v", v[0])
	}
	if n := p.a.count("SELECT count(DISTINCT visit) FROM analytics_events WHERE workspace_id = $1 AND property_id = $2 AND visit = $3", p.ws, p.id, v[0].visit); n != 1 {
		t.Fatalf("events of %d visits", n)
	}
	if n := p.a.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1 AND property_id = $2 AND kind = 4 AND visit = $3", p.ws, p.id, v[0].visit); n != 1 {
		t.Fatal("the engagement on the other process did not join the visit")
	}

	p.a.clock.Set(at.Add(10 * time.Minute))
	p.b.clock.Set(at.Add(10 * time.Minute))
	p.send(p.a, map[string]any{"n": "pageview", "u": "https://example.com/signup", "w": 1280})
	p.a.flushAnalytics()
	if v := p.visits(); len(v) != 1 || v[0].views != 3 {
		t.Fatalf("a visit continued after the claim window did not keep its id: %+v", v)
	}

	p.a.clock.Set(at.Add(10*time.Minute + analytics.VisitGap + time.Minute))
	p.b.clock.Set(at.Add(10*time.Minute + analytics.VisitGap + time.Minute + 50*time.Millisecond))
	p.send(p.b, map[string]any{"n": "pageview", "u": "https://example.com/blog", "r": "https://news.ycombinator.com/", "w": 1280})
	p.send(p.a, map[string]any{"n": "pageview", "u": "https://example.com/blog/2", "w": 1280})
	p.a.flushAnalytics()
	p.b.flushAnalytics()
	v = p.visits()
	if len(v) != 2 || v[0].views != 3 || v[1].views != 2 || v[1].refHost != "news.ycombinator.com" {
		t.Fatalf("a return after the gap, seen by both processes: %+v", v)
	}
}

func TestAnalyticsConcurrentFirstViewsStartOneVisit(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Second)
	p := newProcesses(t, at)
	ws, prop := uuid.MustParse(p.ws), uuid.MustParse(p.id)
	pg := &analytics.PGStore{Pool: p.a.st.Pool}
	resolver := func() *analytics.Resolver {
		return &analytics.Resolver{Store: pg, Salts: analytics.NewSalts(pg), Visits: analytics.NewVisits(1000), Grace: 2 * time.Hour}
	}
	r1, r2 := resolver(), resolver()
	const visitors = 40
	type result struct {
		v   analytics.Visitor
		err error
	}
	got := make([][2]result, visitors)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range visitors {
		short, _ := analytics.Shorten(fmt.Sprintf("198.51.%d.7", i))
		for j, r := range []*analytics.Resolver{r1, r2} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				now := at.Add(time.Duration(j) * time.Millisecond)
				v, ok, err := r.Resolve(context.Background(), ws, prop, time.UTC, now, short, uaMac, false)
				if err == nil && !ok {
					err = fmt.Errorf("a pageview was not resolved")
				}
				got[i][j] = result{v, err}
			}()
		}
	}
	close(start)
	wg.Wait()
	for i, pair := range got {
		a, b := pair[0], pair[1]
		if a.err != nil || b.err != nil {
			t.Fatal(a.err, b.err)
		}
		if a.v.Visitor != b.v.Visitor {
			t.Fatalf("visitor %d: two processes made the day's salt and got different ones", i)
		}
		if a.v.Visit != b.v.Visit || a.v.NewVisit == b.v.NewVisit {
			t.Fatalf("visitor %d: visits %d and %d, new %v and %v", i, a.v.Visit, b.v.Visit, a.v.NewVisit, b.v.NewVisit)
		}
	}
	if n := p.a.count("SELECT count(*) FROM analytics_salts WHERE workspace_id = $1 AND property_id = $2", p.ws, p.id); n != 1 {
		t.Fatalf("%d salts", n)
	}

	short, _ := analytics.Shorten("192.0.2.1")
	first, _, err := r1.Resolve(context.Background(), ws, prop, time.UTC, at, short, uaMac, false)
	if err != nil || !first.NewVisit {
		t.Fatalf("first view %+v %v", first, err)
	}
	eng, ok, err := r2.Resolve(context.Background(), ws, prop, time.UTC, at.Add(analytics.ClaimWindow-time.Second), short, uaMac, true)
	if err != nil || !ok || eng.Visit != first.Visit {
		t.Fatalf("an engagement on the other process inside the window: %+v %v %v", eng, ok, err)
	}
	r3 := resolver()
	if _, ok, err := r3.Resolve(context.Background(), ws, prop, time.UTC, at.Add(analytics.ClaimWindow+time.Second), short, uaMac, true); err != nil || ok {
		t.Fatalf("an engagement after the window joined a visit that was never stored: %v %v", ok, err)
	}

	p.a.clock.Set(at.Add(analytics.ClaimWindow + time.Second))
	if _, err := p.a.srv.DeleteExpiredAnalyticsSalts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := p.a.count("SELECT count(*) FROM analytics_visit_claims WHERE workspace_id = $1 AND property_id = $2", p.ws, p.id); n != 0 {
		t.Fatalf("%d claims outlived the window", n)
	}
}

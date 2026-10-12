package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"net/http"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/api"
)

// peakRSS is the process's resident memory high-water mark in bytes.
func peakRSS(t *testing.T) int64 {
	t.Helper()
	f, err := os.Open("/proc/self/status")
	if err != nil {
		t.Skip("no /proc/self/status here")
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "VmHWM:"); ok {
			kb, _ := strconv.ParseInt(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "kB")), 10, 64)
			return kb << 10
		}
	}
	return 0
}

// TestDovecotLargeMailbox moves the bulk fixture mailbox (gen -large -bulk 200000 -bulk-body
// 10000) and reports how it went; the nightly workflow runs it, pull requests do not.
func TestDovecotLargeMailbox(t *testing.T) {
	if os.Getenv("YUVA_TEST_LARGE_MAILBOX") == "" {
		t.Skip("YUVA_TEST_LARGE_MAILBOX is not set")
	}
	d := newDovecotRun(t)
	key := d.h.client()
	key.bearer = d.tm.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "large"}).str("secret")
	d.tm.owner = key
	h, tm := d.h, d.tm
	ctx := context.Background()
	want := expectedFixture(t, d.fixtures, "bulk")
	sub := h.hub.Subscribe(uuid.MustParse(tm.ws))
	defer h.hub.Unsubscribe(sub)
	var mu sync.Mutex
	var progress []time.Time
	stop := make(chan struct{})
	collected := make(chan struct{})
	go func() {
		defer close(collected)
		for {
			select {
			case <-stop:
				return
			case e := <-sub.Events():
				if e.Type == "migration.progress" {
					mu.Lock()
					progress = append(progress, time.Now())
					mu.Unlock()
				}
			}
		}
	}()
	dbSize := func() int64 {
		var n int64
		if err := h.st.Pool.QueryRow(ctx, "SELECT pg_database_size(current_database())").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	id, _ := d.connect(t, "bulk", want.TeamAddresses, "")
	var slowest time.Duration
	slices := 0
	run := func() time.Duration {
		started := time.Now()
		for range 100000 {
			var jobs []riverJob
			for _, kind := range []string{"migration_scan", "migration_copy"} {
				jobs = append(jobs, h.jobs(kind, tm.ws)...)
			}
			if len(jobs) == 0 {
				break
			}
			var a api.MigrationJobArgs
			if err := json.Unmarshal(jobs[0].args, &a); err != nil {
				t.Fatal(err)
			}
			at := time.Now()
			res, err := h.srv.RunMigrationSlice(ctx, a, jobs[0].id, 30*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			slowest = max(slowest, time.Since(at))
			slices++
			if res.Done || res.Until.After(h.clock.Now()) {
				h.dropJob(jobs[0].id)
			}
		}
		return time.Since(started)
	}
	tm.owner.expect(http.StatusOK, "POST", "/v1/migrations/"+id+"/scan", nil)
	scan := run()
	m := d.migration(id)
	if m.str("state") != "ready" {
		t.Fatalf("after the scan: %s", m.raw)
	}
	checkDryRun(t, m.body["estimate"].(map[string]any), want)
	sizeBefore := dbSize()
	tm.owner.expect(http.StatusOK, "POST", "/v1/migrations/"+id+"/start", nil)
	mu.Lock()
	progress = progress[:0]
	mu.Unlock()
	slices = 0
	copied := run()
	close(stop)
	<-collected
	m = d.migration(id)
	counts := m.body["counts"].(map[string]any)
	imported := num(counts["imported"])
	if m.str("state") != "syncing" || imported != want.MessagesIn+want.MessagesOut || num(counts["failed"]) != 0 {
		t.Fatalf("after the copy: %s", m.raw)
	}
	growth := dbSize() - sizeBefore
	rss := peakRSS(t)
	closest := time.Duration(0)
	for i := 1; i < len(progress); i++ {
		if gap := progress[i].Sub(progress[i-1]); closest == 0 || gap < closest {
			closest = gap
		}
	}
	report := fmt.Sprintf("## Large mailbox migration\n\n| | |\n|---|---|\n| messages | %d |\n| scan | %s |\n| copy | %s |\n| copy rate | %.0f messages/s |\n"+
		"| slowest slice | %s |\n| peak RSS | %d MiB |\n| database growth | %d bytes/message |\n| progress signals | %d, closest %s apart |\n",
		imported, scan.Round(time.Second), copied.Round(time.Second), float64(imported)/copied.Seconds(), slowest.Round(time.Millisecond),
		rss>>20, growth/int64(max(imported, 1)), len(progress), closest.Round(time.Millisecond))
	t.Log("\n" + report)
	if path := os.Getenv("GITHUB_STEP_SUMMARY"); path != "" {
		if f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644); err == nil {
			_, _ = f.WriteString(report)
			_ = f.Close()
		}
	}
	if slowest > time.Minute {
		t.Errorf("a slice took %s, over a minute", slowest)
	}
	if rss > 300<<20 {
		t.Errorf("peak RSS %d MiB, over 300 MiB", rss>>20)
	}
	if limit := int(copied/(2*time.Second)) + slices + 1; len(progress) > limit {
		t.Errorf("%d progress signals in %s, more than one every two seconds", len(progress), copied)
	}
}

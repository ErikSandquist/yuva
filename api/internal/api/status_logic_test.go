package api

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestUptimeDays(t *testing.T) {
	day := func(d, h int) time.Time { return time.Date(2026, 3, d, h, 0, 0, 0, time.UTC) }
	spans := []stateSpan{
		{state: stOperational, start: day(2, 0), end: day(3, 6)},
		{state: stMajorOutage, start: day(3, 6), end: day(3, 12)},
		{state: stPartialOutage, start: day(3, 12), end: day(3, 18)},
		{state: stMaintenance, start: day(3, 18), end: day(4, 0)},
		{state: stOperational, start: day(4, 0)},
	}
	days := uptimeDays(spans, 4, time.UTC, day(4, 12))
	if len(days) != 4 || !days[0].date.Equal(day(1, 0)) || !days[3].date.Equal(day(4, 0)) {
		t.Fatalf("days %+v", days)
	}
	if days[0].known || days[0].uptime() != nil {
		t.Fatalf("a day before the component existed has data: %+v", days[0])
	}
	if u := days[1].uptime(); !days[1].known || u == nil || *u != 100 || days[1].state != stOperational {
		t.Fatalf("day 2 %+v", days[1])
	}
	if u := days[2].uptime(); u == nil || *u != 50 || days[2].state != stMajorOutage {
		t.Fatalf("day 3: major 6h and partial 6h over 18h counted (maintenance left out) is 50%%, worst major: %+v %v", days[2], *u)
	}
	if u := days[3].uptime(); u == nil || *u != 100 || days[3].counts != 12*time.Hour {
		t.Fatalf("today counts until now: %+v", days[3])
	}
	if u := uptimeOver(days); u == nil || *u != 83.33 {
		t.Fatalf("over the window: 9h down of 54h counted, got %v", *u)
	}
	onlyMaintenance := uptimeDays([]stateSpan{{state: stMaintenance, start: day(1, 0)}}, 1, time.UTC, day(1, 12))
	if onlyMaintenance[0].uptime() != nil || onlyMaintenance[0].state != stMaintenance {
		t.Fatalf("a day of maintenance only has no uptime: %+v", onlyMaintenance[0])
	}
}

func TestUptimeDaysInPageZone(t *testing.T) {
	ist, _ := time.LoadLocation("Europe/Istanbul")
	now := time.Date(2026, 3, 4, 1, 0, 0, 0, ist)
	outage := []stateSpan{
		{state: stOperational, start: now.Add(-72 * time.Hour), end: now.Add(-90 * time.Minute)},
		{state: stMajorOutage, start: now.Add(-90 * time.Minute), end: now.Add(-30 * time.Minute)},
		{state: stOperational, start: now.Add(-30 * time.Minute)},
	}
	days := uptimeDays(outage, 2, ist, now)
	if days[0].state != stMajorOutage || days[1].state != stMajorOutage {
		t.Fatalf("the outage from 23:30 to 00:30 local touches both local days: %+v", days)
	}
	if u := days[1].uptime(); u == nil || *u != 50 {
		t.Fatalf("today local is 30 of 60 minutes down: %+v", days[1])
	}
}

func TestWorstState(t *testing.T) {
	for _, c := range []struct {
		in   []string
		want string
	}{
		{nil, stOperational},
		{[]string{stOperational, stMaintenance}, stMaintenance},
		{[]string{stMaintenance, stDegraded}, stDegraded},
		{[]string{stPartialOutage, stMajorOutage, stDegraded}, stMajorOutage},
	} {
		if got := worstState(c.in); got != c.want {
			t.Errorf("worstState(%v) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestCheckIncidentMove(t *testing.T) {
	for _, c := range []struct {
		kind, from, to string
		ok             bool
	}{
		{kindIncident, incInvestigating, incIdentified, true},
		{kindIncident, incMonitoring, incInvestigating, true},
		{kindIncident, incMonitoring, incResolved, true},
		{kindIncident, incResolved, incInvestigating, false},
		{kindIncident, incInvestigating, incInProgress, false},
		{kindMaintenance, incScheduled, incInProgress, true},
		{kindMaintenance, incScheduled, incCompleted, true},
		{kindMaintenance, incInProgress, incScheduled, false},
		{kindMaintenance, incCompleted, incInProgress, false},
		{kindMaintenance, incInProgress, incResolved, false},
	} {
		err := checkIncidentMove(c.kind, c.from, c.to)
		if (err == nil) != c.ok {
			t.Errorf("%s %s -> %s: %v", c.kind, c.from, c.to, err)
		}
	}
	if err := checkIncidentMove(kindIncident, incResolved, incResolved); err != errIncidentClosed {
		t.Errorf("a resolved incident takes no update: %v", err)
	}
}

func TestIncidentHolds(t *testing.T) {
	if incidentHolds(false, incInvestigating) || incidentHolds(true, incScheduled) || incidentHolds(true, incResolved) || !incidentHolds(true, incInProgress) {
		t.Fatal("only published open incidents and maintenance in progress hold components")
	}
}

func TestRichStatusText(t *testing.T) {
	got := string(richStatusText("Fixed <b>it</b>.\nSee https://example.com/a?b=1&c=2."))
	want := `Fixed &lt;b&gt;it&lt;/b&gt;.<br>See <a href="https://example.com/a?b=1&amp;c=2" rel="nofollow noopener">https://example.com/a?b=1&amp;c=2</a>.`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if _, err := statusText(strings.Repeat("a", 5001)); err == nil {
		t.Fatal("more than 5000 characters accepted")
	}
	if s, err := statusText(" a\r\nb "); err != nil || s != "a\nb" {
		t.Fatalf("statusText %q %v", s, err)
	}
}

func TestRichStatusTextLeavesHostlessURLsAlone(t *testing.T) {
	got := string(richStatusText("Try http://. and https:///x then https://example.com/ok."))
	want := `Try http://. and https:///x then <a href="https://example.com/ok" rel="nofollow noopener">https://example.com/ok</a>.`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestStatusUniqueConflict(t *testing.T) {
	violation := func(constraint string) error {
		return fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505", ConstraintName: constraint})
	}
	if err := statusUniqueConflict(violation("status_pages_workspace_id_inbox_id_key")); err != errStatusPageExists {
		t.Fatalf("one page per inbox maps to %v", err)
	}
	if err := statusUniqueConflict(violation("status_pages_slug_key")); err != errStatusSlugTaken {
		t.Fatalf("slug maps to %v", err)
	}
	other := violation("status_components_pkey")
	if err := statusUniqueConflict(other); err != other {
		t.Fatalf("an unrelated violation became %v", err)
	}
	plain := errors.New("boom")
	if err := statusUniqueConflict(plain); err != plain {
		t.Fatalf("a plain error became %v", err)
	}
}

func cellColumn(grid *cellGrid, class string, col int) int {
	n := 0
	for _, c := range grid.Cells() {
		if c.Class != class {
			continue
		}
		for _, m := range strings.Split(c.D, "Z") {
			var x, y int
			if _, err := fmt.Sscanf(m, "M%d %d", &x, &y); err == nil && x == col*cellPitch {
				n++
			}
		}
	}
	return n
}

func TestStepCellsBridgesOnlyGapsWithinTheIntervalAndWarnsOnSpikes(t *testing.T) {
	points := make([]int32, 20)
	for i := range points {
		points[i] = 100
	}
	points[5] = 900
	points[10], points[15], points[16], points[17] = -1, -1, -1, -1
	cells, empty := stepCells(points, 1, 0, 0)
	if empty {
		t.Fatal("empty")
	}
	if cellColumn(cells, "hot", 5) == 0 {
		t.Fatal("spike has no warn cell")
	}
	if cellColumn(cells, "hot", 4)+cellColumn(cells, "hot", 3) != 0 {
		t.Fatal("a flat value drew warn cells")
	}
	for _, class := range []string{"on", "hot", "on w", "hot w"} {
		if n := cellColumn(cells, class, 16); n != 0 {
			t.Fatalf("null column has %d %q cells", n, class)
		}
	}
	if cellColumn(cells, "off", 16) != chartRows {
		t.Fatal("null column is not all off")
	}
	if cellColumn(cells, "on", 10) == 0 {
		t.Fatal("a gap within the interval was not bridged")
	}
	if hold := stepHold(60*time.Second, 67*time.Second); hold != 2 {
		t.Fatalf("hold %d", hold)
	}
}

package api_test

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/api"
)

func (nt noticeTeam) noticeID(t *testing.T, incident string) uuid.UUID {
	t.Helper()
	ns := nt.notices(t, incident)
	if len(ns) == 0 {
		t.Fatalf("no notice of incident %s", incident)
	}
	return uuid.MustParse(ns[0]["id"].(string))
}

func (nt noticeTeam) subscriptionID(t *testing.T, addr string) (id, contact string) {
	t.Helper()
	for _, it := range items(nt.owner.expect(http.StatusOK, "GET", "/v1/lists/"+nt.list+"/subscriptions", nil)) {
		if it["email"] == addr {
			sub := nt.owner.expect(http.StatusOK, "GET", "/v1/lists/"+nt.list+"/subscriptions/"+it["id"].(string), nil).body
			return it["id"].(string), sub["contact"].(map[string]any)["id"].(string)
		}
	}
	t.Fatalf("no subscription of %s", addr)
	return "", ""
}

func TestStatusNoticeDatabaseErrorFailsTheJob(t *testing.T) {
	dsn := os.Getenv("YUVA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("YUVA_TEST_DATABASE_URL is not set")
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	t.Setenv("YUVA_TEST_DATABASE_URL", dsn+sep+"lock_timeout=300")
	h, bs := newBroadcastHarness(t, nil)
	nt := newNoticeTeam(t, h, bs)
	nt.subscriber(t, "ada@example.org")
	nt.subscriber(t, "ben@example.org")
	inc := nt.incident(t, map[string]any{"title": "Slow", "message": "Looking.", "publish": true})
	h.clock.Advance(61 * time.Second)
	ctx := context.Background()
	ws, id := uuid.MustParse(nt.ws), nt.noticeID(t, inc)
	if err := h.srv.RunStatusNotice(ctx, ws, id, 0); err != nil {
		t.Fatal(err)
	}

	tx, err := h.st.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL lock_timeout = '10s'"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "LOCK TABLE email_suppressions IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if err := h.srv.RunStatusNotice(ctx, ws, id, time.Minute); err == nil {
		t.Fatal("a database error while sending ended the job without an error")
	}
	if time.Since(started) > 30*time.Second {
		t.Fatal("the job kept spinning on a database error")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	h.clock.Advance(time.Second)
	nt.runNotices(t)
	if len(bs.to("ada@example.org")) != 1 || len(bs.to("ben@example.org")) != 1 {
		t.Fatalf("after the retry: ada %d ben %d", len(bs.to("ada@example.org")), len(bs.to("ben@example.org")))
	}
	if n := nt.notices(t, inc)[0]; n["state"] != "sent" {
		t.Fatalf("after the retry: %v", n)
	}
}

func TestStatusMaintenanceHonoursNotifyAndSpeaks(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	nt := newNoticeTeam(t, h, bs)
	nt.subscriber(t, "ada@example.org")
	ctx := context.Background()
	start := h.clock.Now().Add(time.Hour)
	maintenance := func(title string, notify bool) (string, api.StatusMaintenanceArgs) {
		id := nt.incident(t, map[string]any{
			"kind": "maintenance", "title": title, "message": "Planned.", "publish": true, "notify": notify,
			"scheduled_start": start.Format(time.RFC3339), "scheduled_end": start.Add(time.Hour).Format(time.RFC3339),
		})
		return id, api.StatusMaintenanceArgs{WorkspaceID: uuid.MustParse(nt.ws), IncidentID: uuid.MustParse(id)}
	}
	loud, loudArgs := maintenance("Loud", true)
	quiet, quietArgs := maintenance("Quiet", false)
	if n := len(nt.notices(t, quiet)); n != 0 {
		t.Fatalf("a quiet maintenance queued %d notices at publish", n)
	}
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)

	bodies := func(id string) []string {
		var out []string
		for _, u := range nt.owner.expect(http.StatusOK, "GET", nt.path("/incidents/"+id), nil).body["updates"].([]any) {
			out = append(out, u.(map[string]any)["body"].(string))
		}
		return out
	}
	nt.owner.expect(http.StatusOK, "PATCH", nt.path(""), map[string]any{"locale": "tr"})
	h.clock.Advance(start.Sub(h.clock.Now()) + time.Second)
	for _, a := range []api.StatusMaintenanceArgs{loudArgs, quietArgs} {
		if err := h.srv.RunStatusMaintenance(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	h.clock.Advance(time.Hour)
	for _, a := range []api.StatusMaintenanceArgs{loudArgs, quietArgs} {
		if err := h.srv.RunStatusMaintenance(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)

	if n := len(nt.notices(t, loud)); n != 3 {
		t.Fatalf("a loud maintenance has %d notices, want publish, start and completion", n)
	}
	if n := len(nt.notices(t, quiet)); n != 0 {
		t.Fatalf("a quiet maintenance has %d notices", n)
	}
	for _, id := range []string{loud, quiet} {
		got := strings.Join(bodies(id), "|")
		if !strings.Contains(got, "Bakım başladı.") || !strings.Contains(got, "Bakım tamamlandı.") {
			t.Fatalf("updates of %s: %s", id, got)
		}
	}
	if len(bs.to("ada@example.org")) != 3 {
		t.Fatalf("%d mails to ada", len(bs.to("ada@example.org")))
	}
}

func TestStatusFinalNoticeNeedsAnEarlierOne(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	nt := newNoticeTeam(t, h, bs)
	nt.subscriber(t, "ada@example.org")
	first := nt.incident(t, map[string]any{"title": "First", "message": "Looking.", "publish": true})
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	nt.subscriber(t, "ben@example.org")
	for i := range 9 {
		nt.incident(t, map[string]any{"title": "Flap " + string(rune('a'+i)), "message": "x", "publish": true})
		h.clock.Advance(61 * time.Second)
		nt.runNotices(t)
	}
	unheard := nt.incident(t, map[string]any{"title": "Unheard", "message": "x", "publish": true})
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	if n := nt.notices(t, unheard)[0]; n["state"] != "skipped" || n["skip_reason"] != "rate" {
		t.Fatalf("the eleventh notice: %v", n)
	}

	ada, ben := len(bs.to("ada@example.org")), len(bs.to("ben@example.org"))
	nt.update(t, first, map[string]any{"body": "Fixed.", "state": "resolved"})
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	ns := nt.notices(t, first)
	if ns[0]["final"] != true || ns[0]["state"] != "sent" || ns[0]["counts"].(map[string]any)["recipients"] != float64(1) {
		t.Fatalf("final notice of a heard incident: %v", ns[0])
	}
	if len(bs.to("ada@example.org")) != ada+1 || len(bs.to("ben@example.org")) != ben {
		t.Fatalf("recipients of the final notice: ada %d->%d ben %d->%d", ada, len(bs.to("ada@example.org")), ben, len(bs.to("ben@example.org")))
	}

	before := bs.count()
	nt.update(t, unheard, map[string]any{"body": "Fixed.", "state": "resolved"})
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	ns = nt.notices(t, unheard)
	if ns[0]["final"] != true || ns[0]["state"] != "skipped" || ns[0]["skip_reason"] != "nothing_earlier" || bs.count() != before {
		t.Fatalf("final notice of an unheard incident: %v", ns[0])
	}
}

func TestStatusNoticeDailyLimitWaitsForTomorrow(t *testing.T) {
	h, bs := newBroadcastHarness(t, func(d *api.Deps) { d.Status = api.StatusSettings{NoticesDaily: 2} })
	nt := newNoticeTeam(t, h, bs)
	for _, a := range []string{"ada@example.org", "ben@example.org", "cy@example.org"} {
		nt.subscriber(t, a)
	}
	toHour := func(days, hour int) {
		now := h.clock.Now().UTC()
		h.clock.Advance(time.Date(now.Year(), now.Month(), now.Day()+days, hour, 0, 0, 0, time.UTC).Sub(now))
	}
	toHour(1, 23)
	midnight := utcMidnight(h.clock.Now().Add(2 * time.Hour))

	inc := nt.incident(t, map[string]any{"title": "Slow", "message": "Looking.", "publish": true})
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	n := nt.notices(t, inc)[0]
	if c := n["counts"].(map[string]any); n["state"] != "sending" || bs.count() != 2 || c["queued"] != float64(1) || c["skipped"] != float64(0) {
		t.Fatalf("at the daily limit: %d mails, %v", bs.count(), n)
	}
	var next time.Time
	if err := h.st.Pool.QueryRow(context.Background(),
		"SELECT next_attempt_at FROM status_notice_deliveries WHERE workspace_id = $1 AND state = 'queued'", nt.ws).Scan(&next); err != nil {
		t.Fatal(err)
	}
	if !next.Equal(midnight) {
		t.Fatalf("next attempt %s, want %s", next, midnight)
	}

	h.clock.Advance(2 * time.Hour)
	nt.runNotices(t)
	n = nt.notices(t, inc)[0]
	if c := n["counts"].(map[string]any); n["state"] != "sent" || bs.count() != 3 || c["sent"] != float64(3) || c["skipped"] != float64(0) {
		t.Fatalf("the next day: %d mails, %v", bs.count(), n)
	}

	toHour(2, 23)
	late := nt.incident(t, map[string]any{"title": "Slow again", "message": "Looking.", "publish": true})
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	before := bs.count()
	if c := nt.notices(t, late)[0]["counts"].(map[string]any); c["queued"] != float64(1) {
		t.Fatalf("at the limit again: %v", c)
	}
	h.clock.Advance(7 * time.Hour)
	nt.runNotices(t)
	n = nt.notices(t, late)[0]
	if c := n["counts"].(map[string]any); n["state"] != "sent" || bs.count() != before || c["skipped"] != float64(1) || c["queued"] != float64(0) {
		t.Fatalf("after the staleness window: %d mails, %v", bs.count()-before, n)
	}
	var reason string
	if err := h.st.Pool.QueryRow(context.Background(),
		"SELECT skip_reason FROM status_notice_deliveries WHERE workspace_id = $1 AND state = 'skipped'", nt.ws).Scan(&reason); err != nil || reason != "stale" {
		t.Fatalf("skip reason %q: %v", reason, err)
	}
}

func utcMidnight(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func TestStatusSubscriberWhoseComponentsWereDeleted(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	nt := newNoticeTeam(t, h, bs)
	nt.subscriber(t, "ada@example.org")
	nt.subscriber(t, "ben@example.org")
	subID, contact := nt.subscriptionID(t, "ben@example.org")
	manage := strings.TrimPrefix(h.srv.ManageURL(uuid.MustParse(nt.ws), uuid.MustParse(nt.list), uuid.MustParse(contact)), testOrigin)
	chose := func() bool {
		var v bool
		if err := h.st.Pool.QueryRow(context.Background(), "SELECT chose_components FROM list_subscriptions WHERE workspace_id = $1 AND id = $2", nt.ws, subID).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	if _, page := pageAt(t, h, "POST", manage, map[string][]string{"action": {"components"}, "components": {nt.api, nt.web}}); !strings.Contains(page, "Your choices are saved") || chose() {
		t.Fatalf("choosing every component: %s", page)
	}
	if _, page := pageAt(t, h, "POST", manage, map[string][]string{"action": {"components"}, "components": {nt.api}}); !strings.Contains(page, "Your choices are saved") || !chose() {
		t.Fatalf("choosing one component: %s", page)
	}

	nt.owner.expect(http.StatusNoContent, "DELETE", nt.path("/components/"+nt.api), nil)
	sub := nt.owner.expect(http.StatusOK, "GET", "/v1/lists/"+nt.list+"/subscriptions/"+subID, nil).body
	if c, ok := sub["components"].([]any); !ok || len(c) != 0 {
		t.Fatalf("components of a subscriber whose choice is gone: %v", sub["components"])
	}
	if r := nt.owner.expect(http.StatusOK, "GET", nt.path("/notice-reach?component_id="+nt.web), nil); r.body["subscribers"] != float64(1) {
		t.Fatalf("reach of a component: %v", r.body)
	}
	if r := nt.owner.expect(http.StatusOK, "GET", nt.path("/notice-reach"), nil); r.body["subscribers"] != float64(2) {
		t.Fatalf("reach of no component: %v", r.body)
	}

	touching := nt.incident(t, map[string]any{"title": "Web down", "message": "x", "publish": true, "components": []map[string]any{{"component_id": nt.web}}})
	general := nt.incident(t, map[string]any{"title": "Everywhere", "message": "x", "publish": true})
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	if n := nt.notices(t, touching)[0]["counts"].(map[string]any); n["recipients"] != float64(1) {
		t.Fatalf("an incident touching a component: %v", n)
	}
	if n := nt.notices(t, general)[0]["counts"].(map[string]any); n["recipients"] != float64(2) {
		t.Fatalf("an incident touching none: %v", n)
	}
	if len(bs.to("ada@example.org")) != 2 || len(bs.to("ben@example.org")) != 1 {
		t.Fatalf("mails: ada %d ben %d", len(bs.to("ada@example.org")), len(bs.to("ben@example.org")))
	}
}

func TestHomeLeavesOutStatusLists(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	nt := newNoticeTeam(t, h, bs)
	got := checklist(t, homeOf(nt.owner))
	if got["newsletter"] != [2]any{true, float64(1)} {
		t.Fatalf("the status list counted as a newsletter list: %v", got["newsletter"])
	}
}

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/api"
	"github.com/productdevbook/yuva/api/internal/email"
	"github.com/productdevbook/yuva/api/internal/store"
)

type noticeTeam struct {
	broadcastTeam
	page, slug, list, api, web string
}

func newNoticeTeam(t *testing.T, h *harness, bs *broadcastSender) noticeTeam {
	t.Helper()
	bt := newBroadcastTeam(t, h, bs)
	nt := noticeTeam{broadcastTeam: bt, slug: unique("st")}
	nt.page = bt.owner.expect(http.StatusCreated, "POST", "/v1/status-pages", map[string]any{
		"inbox_id": bt.chatInbox, "name": "Acme", "slug": nt.slug, "published": true, "timezone": "Europe/Berlin",
	}).str("id")
	nt.api = bt.owner.expect(http.StatusCreated, "POST", nt.path("/components"), map[string]any{"name": "API"}).str("id")
	nt.web = bt.owner.expect(http.StatusCreated, "POST", nt.path("/components"), map[string]any{"name": "Website"}).str("id")
	r := bt.owner.expect(http.StatusOK, "PATCH", nt.path(""), map[string]any{"email_subscribers": map[string]any{"enabled": true, "channel_id": bt.emailChannel}})
	nt.list = r.body["email_subscribers"].(map[string]any)["list_id"].(string)
	return nt
}

func (nt noticeTeam) path(p string) string { return "/v1/status-pages/" + nt.page + p }

func (nt noticeTeam) subscriber(t *testing.T, addr string, components ...string) {
	t.Helper()
	id := nt.owner.expect(http.StatusAccepted, "POST", "/v1/lists/"+nt.list+"/subscriptions", map[string]any{
		"email": addr, "consent": map[string]any{"confirmed": true},
	}).str("id")
	ctx := context.Background()
	if _, err := nt.h.st.Pool.Exec(ctx, "UPDATE list_subscriptions SET state = 'subscribed', consent_at = now() WHERE workspace_id = $1 AND id = $2", nt.ws, id); err != nil {
		t.Fatal(err)
	}
	if len(components) > 0 {
		if _, err := nt.h.st.Pool.Exec(ctx, "UPDATE list_subscriptions SET chose_components = true WHERE workspace_id = $1 AND id = $2", nt.ws, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range components {
		if _, err := nt.h.st.Pool.Exec(ctx, "INSERT INTO status_subscriber_components (workspace_id, subscription_id, component_id) VALUES ($1, $2, $3)", nt.ws, id, c); err != nil {
			t.Fatal(err)
		}
	}
}

// runNotices does what the sweep and the notice queue do.
func (nt noticeTeam) runNotices(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if err := nt.h.srv.SweepStatusNotices(ctx); err != nil {
		t.Fatal(err)
	}
	for _, j := range nt.h.jobs("status_notice", nt.ws) {
		var a api.StatusNoticeArgs
		if err := json.Unmarshal(j.args, &a); err != nil {
			t.Fatal(err)
		}
		if err := nt.h.srv.RunStatusNotice(ctx, a.WorkspaceID, a.NoticeID, time.Minute); err != nil {
			t.Fatal(err)
		}
		nt.h.dropJob(j.id)
	}
}

func (nt noticeTeam) notices(t *testing.T, incident string) []map[string]any {
	t.Helper()
	return items(nt.owner.expect(http.StatusOK, "GET", nt.path("/notices?incident_id="+incident), nil))
}

func (nt noticeTeam) incident(t *testing.T, body map[string]any) string {
	t.Helper()
	return nt.owner.expect(http.StatusCreated, "POST", nt.path("/incidents"), body).str("id")
}

func (nt noticeTeam) update(t *testing.T, incident string, body map[string]any) {
	t.Helper()
	nt.owner.expect(http.StatusCreated, "POST", nt.path("/incidents/"+incident+"/updates"), body)
}

func TestStatusNoticeThrottleAndContent(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	nt := newNoticeTeam(t, h, bs)
	nt.subscriber(t, "ada@example.org")
	nt.subscriber(t, "ben@example.org", nt.api)
	nt.subscriber(t, "cy@example.org", nt.web)

	if r := nt.owner.expect(http.StatusOK, "GET", nt.path("/notice-reach?component_id="+nt.api), nil); r.body["subscribers"] != float64(2) {
		t.Fatalf("reach: %v", r.body)
	}
	inc := nt.incident(t, map[string]any{"title": "API down", "message": "Requests <fail>.", "components": []map[string]any{{"component_id": nt.api}}})
	if got := nt.owner.expect(http.StatusOK, "GET", nt.path("/incidents/"+inc), nil); got.body["notify"] != true {
		t.Fatalf("draft notify: %v", got.body["notify"])
	}
	if len(nt.notices(t, inc)) != 0 {
		t.Fatal("a draft queued a notice")
	}
	nt.owner.expect(http.StatusOK, "POST", nt.path("/incidents/"+inc+"/publish"), nil)
	ns := nt.notices(t, inc)
	if len(ns) != 1 || ns[0]["state"] != "pending" || ns[0]["final"] != false || len(ns[0]["update_ids"].([]any)) != 1 {
		t.Fatalf("after publish: %v", ns)
	}
	nt.update(t, inc, map[string]any{"body": "Only text."})
	nt.update(t, inc, map[string]any{"body": "Found it at https://acme.example/log.", "state": "identified"})
	ns = nt.notices(t, inc)
	if len(ns) != 1 || len(ns[0]["update_ids"].([]any)) != 2 {
		t.Fatalf("updates within the hold: %v", ns)
	}
	nt.runNotices(t)
	if bs.count() != 0 {
		t.Fatal("a notice went out during its hold")
	}
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	if len(bs.to("ada@example.org")) != 1 || len(bs.to("ben@example.org")) != 1 || len(bs.to("cy@example.org")) != 0 {
		t.Fatalf("recipients: ada %d ben %d cy %d", len(bs.to("ada@example.org")), len(bs.to("ben@example.org")), len(bs.to("cy@example.org")))
	}
	first := bs.to("ada@example.org")[0]
	hd := first.Header
	if hd.Get("Subject") != "[Acme] API down" || hd.Get("List-Unsubscribe-Post") != "List-Unsubscribe=One-Click" ||
		!strings.Contains(hd.Get("List-Unsubscribe"), "/m/") || !strings.Contains(hd.Get("List-Id"), "<s") ||
		!strings.HasSuffix(hd.Get("Feedback-Id"), ":yuvastatus") || hd.Get("X-Auto-Response-Suppress") != "OOF, AutoReply" ||
		hd.Get("Auto-Submitted") != "" || hd.Get("In-Reply-To") != "" {
		t.Fatalf("headers: %v", hd)
	}
	env, err := email.Parse(rawOf(t, bs, "ada@example.org", 0))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(env.Text, "Identified") || !strings.Contains(env.Text, "Found it at") || strings.Contains(env.Text, "Only text.") ||
		!strings.Contains(env.HTML, "&lt;fail&gt;") || !strings.Contains(env.HTML, `href="https://acme.example/log"`) || !strings.Contains(env.Text, "Europe/Berlin") {
		t.Fatalf("content: %s", env.Text)
	}
	ns = nt.notices(t, inc)
	if c := ns[0]["counts"].(map[string]any); ns[0]["state"] != "sent" || c["sent"] != float64(2) || c["recipients"] != float64(2) {
		t.Fatalf("sent notice: %v", ns[0])
	}

	nt.update(t, inc, map[string]any{"body": "Watching.", "state": "monitoring"})
	ns = nt.notices(t, inc)
	due, _ := time.Parse(time.RFC3339Nano, ns[0]["due_at"].(string))
	if ns[0]["state"] != "pending" || due.Before(h.clock.Now().Add(10*time.Minute)) {
		t.Fatalf("the gap: %v", ns[0])
	}
	nt.update(t, inc, map[string]any{"body": "Fixed.", "state": "resolved"})
	ns = nt.notices(t, inc)
	due, _ = time.Parse(time.RFC3339Nano, ns[0]["due_at"].(string))
	if len(ns) != 2 || ns[0]["final"] != true || due.After(h.clock.Now().Add(time.Minute)) {
		t.Fatalf("the resolution: %v", ns[0])
	}
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	again := bs.to("ada@example.org")
	if len(again) != 2 || again[1].Header.Get("Subject") != "[Acme] API down" || again[1].Header.Get("In-Reply-To") != first.Header.Get("Message-Id") {
		t.Fatalf("second notice: %d %v", len(again), again[len(again)-1].Header)
	}

	quiet := nt.incident(t, map[string]any{"title": "Quiet", "message": "x", "publish": true, "notify": false})
	if len(nt.notices(t, quiet)) != 0 {
		t.Fatal("notify false queued a notice")
	}
	undo := nt.incident(t, map[string]any{"title": "Undo me", "message": "x", "publish": true})
	id := nt.notices(t, undo)[0]["id"].(string)
	other := newListTeam(t, h)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", nt.path("/notices/"+id), nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", nt.path("/notices/"+id+"/cancel"), nil)
	if c := nt.owner.expect(http.StatusOK, "POST", nt.path("/notices/"+id+"/cancel"), nil); c.body["state"] != "cancelled" {
		t.Fatalf("cancel: %v", c.body)
	}
	nt.owner.expectProblem(http.StatusConflict, "not_pending", "POST", nt.path("/notices/"+id+"/cancel"), nil)
	nt.owner.expectProblem(http.StatusNotFound, "not_found", "GET", nt.path("/notices/"+uuid.New().String()), nil)
	nt.owner.expectProblem(http.StatusBadRequest, "validation_failed", "GET", nt.path("/notices?state=lost"), nil)

	manage, _ := url.Parse(strings.Trim(first.Header.Get("List-Unsubscribe"), "<>"))
	if status, body := pageAt(t, h, "POST", manage.Path, url.Values{"List-Unsubscribe": {"One-Click"}}); status != http.StatusOK || body != "" {
		t.Fatalf("one-click: %d", status)
	}
	before := bs.count()
	gone := nt.incident(t, map[string]any{"title": "Nobody left", "message": "x", "publish": true})
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	if ns := nt.notices(t, gone); ns[0]["state"] != "skipped" || ns[0]["skip_reason"] != "no_recipients" || bs.count() != before {
		t.Fatalf("after one-click: %v", ns[0])
	}
}

func rawOf(t *testing.T, bs *broadcastSender, addr string, i int) []byte {
	t.Helper()
	bs.mu.Lock()
	defer bs.mu.Unlock()
	n := 0
	for _, m := range bs.sent {
		if m.to[0] == addr {
			if n == i {
				return m.raw
			}
			n++
		}
	}
	t.Fatalf("no mail %d to %s", i, addr)
	return nil
}

func TestStatusNoticeRateAndFeedback(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	nt := newNoticeTeam(t, h, bs)
	for _, a := range []string{"ses@example.org", "arf@example.org", "dsn@example.org", "reply@example.org"} {
		nt.subscriber(t, a)
	}
	inc := nt.incident(t, map[string]any{"title": "Slow", "message": "Looking.", "publish": true})
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	if bs.count() != 4 {
		t.Fatalf("%d notices sent", bs.count())
	}

	signer := newSNSSigner(t, h.web)
	sesID := strings.Trim(bs.to("ses@example.org")[0].Header.Get("Message-Id"), "<>")
	payload := `{"notificationType":"Bounce","bounce":{"bounceType":"Permanent","bouncedRecipients":[{"emailAddress":"ses@example.org","status":"5.1.1"}]},"mail":{"commonHeaders":{"messageId":"<` + sesID + `>"}}}`
	note := &email.SNSMessage{Type: "Notification", MessageId: unique("m"), TopicArn: testSESTopic, Message: payload, Timestamp: time.Now().UTC().Format(time.RFC3339)}
	if st := h.postSNS(signer.sign(t, note)); st != http.StatusOK {
		t.Fatalf("ses: %d", st)
	}
	address := nt.owner.expect(http.StatusOK, "GET", "/v1/channels/"+nt.emailChannel, nil).body["email"].(map[string]any)["address"].(string)
	arfID := bs.to("arf@example.org")[0].Header.Get("Message-Id")
	if r := h.ingest(address, arfMail("fbl@mailbox.example", address, arfID, "arf@example.org"), nil); r.str("status") != "bounce" {
		t.Fatalf("arf: %v", r.body)
	}
	dsnID := strings.Trim(bs.to("dsn@example.org")[0].Header.Get("Message-Id"), "<>")
	if r := h.ingestFrom(address, "", dsnMail(address, dsnID, "dsn@example.org", "5.1.1"), nil); r.str("status") != "bounce" {
		t.Fatalf("dsn: %v", r.body)
	}
	suppressed := func(addr string) string {
		t.Helper()
		var reason string
		_ = h.st.Pool.QueryRow(context.Background(), "SELECT reason FROM email_suppressions WHERE workspace_id = $1 AND email = $2", nt.ws, addr).Scan(&reason)
		return reason
	}
	if suppressed("ses@example.org") != "bounce" || suppressed("arf@example.org") != "complaint" || suppressed("dsn@example.org") != "bounce" {
		t.Fatalf("suppressions: %q %q %q", suppressed("ses@example.org"), suppressed("arf@example.org"), suppressed("dsn@example.org"))
	}
	ns := nt.notices(t, inc)
	if c := ns[0]["counts"].(map[string]any); c["bounced"] != float64(2) || c["complained"] != float64(1) {
		t.Fatalf("counts after feedback: %v", c)
	}

	replyTo := bs.to("reply@example.org")[0].Header.Get("Message-Id")
	r := h.ingest(address, buildMail(mailOpts{from: "reply@example.org", to: address, subject: "Re: [Acme] Slow", messageID: newMessageID(), body: "Still slow here",
		headers: map[string]string{"In-Reply-To": replyTo, "References": replyTo}}), nil)
	conv := r.str("conversation_id")
	if r.str("status") != "stored" || conv == "" {
		t.Fatalf("reply: %v", r.body)
	}
	linked := items(nt.owner.expect(http.StatusOK, "GET", nt.path("/incidents/"+inc+"/conversations"), nil))
	if len(linked) != 1 || linked[0]["id"] != conv {
		t.Fatalf("linked conversations: %v", linked)
	}
	if c := nt.notices(t, inc)[0]["counts"].(map[string]any); c["replied"] != float64(1) {
		t.Fatalf("replied: %v", c)
	}

	sent := 0
	for i := range 12 {
		id := nt.incident(t, map[string]any{"title": "Flap " + string(rune('a'+i)), "message": "x", "publish": true})
		h.clock.Advance(61 * time.Second)
		nt.runNotices(t)
		n := nt.notices(t, id)[0]
		switch n["state"] {
		case "sent":
			sent++
		case "skipped":
			if n["skip_reason"] != "rate" || sent != 9 {
				t.Fatalf("skipped after %d more notices: %v", sent, n)
			}
			return
		default:
			t.Fatalf("notice %d: %v", i, n)
		}
	}
	t.Fatal("the eleventh notice in an hour went out")
}

func noticeMail(t *testing.T, bs *broadcastSender, addr string, i int) *email.Message {
	t.Helper()
	env, err := email.Parse(rawOf(t, bs, addr, i))
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func (nt noticeTeam) updateNotify(t *testing.T, incident string) []bool {
	t.Helper()
	rows, err := nt.h.st.Pool.Query(context.Background(),
		"SELECT notify FROM status_incident_updates WHERE workspace_id = $1 AND incident_id = $2 ORDER BY created_at, id", nt.ws, incident)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []bool
	for rows.Next() {
		var b bool
		if err := rows.Scan(&b); err != nil {
			t.Fatal(err)
		}
		out = append(out, b)
	}
	return out
}

func lastNoticeMail(t *testing.T, bs *broadcastSender, addr string) *email.Message {
	t.Helper()
	return noticeMail(t, bs, addr, len(bs.to(addr))-1)
}

func TestStatusFinalNoticeNamesComponentsOnly(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	nt := newNoticeTeam(t, h, bs)
	nt.subscriber(t, "ada@example.org")
	inc := nt.incident(t, map[string]any{"title": "API down", "message": "Requests fail.", "publish": true,
		"components": []map[string]any{{"component_id": nt.api, "state": "degraded"}}})
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	nt.update(t, inc, map[string]any{"body": "Hidden remark.", "notify": false})
	nt.update(t, inc, map[string]any{"body": "Fixed.", "state": "resolved"})
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)

	opening := noticeMail(t, bs, "ada@example.org", 0)
	if !strings.Contains(opening.Text, "API: Degraded performance") || !strings.Contains(opening.HTML, "Degraded performance") {
		t.Fatalf("opening notice: %s", opening.Text)
	}
	final := noticeMail(t, bs, "ada@example.org", 1)
	for name, body := range map[string]string{"text": final.Text, "html": final.HTML} {
		if !strings.Contains(body, "Resolved") || !strings.Contains(body, "Fixed.") || !strings.Contains(body, "API") ||
			strings.Contains(body, "Degraded") || strings.Contains(body, "Operational") || strings.Contains(body, "API:") ||
			strings.Contains(body, "Hidden remark.") {
			t.Fatalf("resolution %s part: %s", name, body)
		}
	}
}

func TestStatusNoticeCoversExactlyItsUpdates(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	nt := newNoticeTeam(t, h, bs)
	nt.subscriber(t, "ada@example.org")

	quiet := nt.incident(t, map[string]any{"title": "Quiet", "message": "Quiet text.", "notify": false})
	loud := nt.incident(t, map[string]any{"title": "Loud", "message": "Public text."})
	if got := nt.updateNotify(t, quiet); len(got) != 1 || got[0] {
		t.Fatalf("opening update of a quiet incident: %v", got)
	}
	if got := nt.updateNotify(t, loud); len(got) != 1 || !got[0] {
		t.Fatalf("opening update of a notifying incident: %v", got)
	}

	nt.update(t, loud, map[string]any{"body": "Draft remark.", "notify": false})
	nt.update(t, loud, map[string]any{"body": "Second text.", "state": "identified"})
	nt.owner.expect(http.StatusOK, "POST", nt.path("/incidents/"+loud+"/publish"), nil)
	ns := nt.notices(t, loud)
	if len(ns) != 1 || len(ns[0]["update_ids"].([]any)) != 3 {
		t.Fatalf("publish attaches every draft update: %v", ns)
	}
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	env := lastNoticeMail(t, bs, "ada@example.org")
	for name, body := range map[string]string{"text": env.Text, "html": env.HTML} {
		first, second := strings.Index(body, "Second text."), strings.Index(body, "Public text.")
		if first < 0 || second < 0 || first > second || !strings.Contains(body, "Draft remark.") {
			t.Fatalf("publish %s part: %s", name, body)
		}
	}

	nt.update(t, loud, map[string]any{"body": "Hidden remark.", "notify": false})
	if ns = nt.notices(t, loud); len(ns[0]["update_ids"].([]any)) != 3 {
		t.Fatalf("a published update with notify false was attached: %v", ns)
	}
}

func TestStatusPublishDraftNoticeCarriesOpeningText(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	nt := newNoticeTeam(t, h, bs)
	nt.subscriber(t, "ada@example.org")

	quiet := nt.incident(t, map[string]any{"title": "Quiet", "message": "Quiet text.", "notify": false})
	nt.update(t, quiet, map[string]any{"body": "Draft remark.", "notify": false})
	nt.owner.expect(http.StatusOK, "POST", nt.path("/incidents/"+quiet+"/publish"), map[string]any{"notify": true})
	ns := nt.notices(t, quiet)
	if len(ns) != 1 || len(ns[0]["update_ids"].([]any)) != 2 {
		t.Fatalf("publish attaches every draft update: %v", ns)
	}
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	env := lastNoticeMail(t, bs, "ada@example.org")
	for name, body := range map[string]string{"text": env.Text, "html": env.HTML} {
		if !strings.Contains(body, "Quiet text.") || !strings.Contains(body, "Draft remark.") {
			t.Fatalf("publish %s part: %s", name, body)
		}
	}
}

func TestStatusMaintenanceWindowChangeNotice(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	nt := newNoticeTeam(t, h, bs)
	nt.subscriber(t, "ada@example.org")
	start := h.clock.Now().Add(48 * time.Hour)
	window := func(shift time.Duration) map[string]any {
		return map[string]any{"scheduled_start": start.Add(shift).Format(time.RFC3339), "scheduled_end": start.Add(shift + time.Hour).Format(time.RFC3339)}
	}
	body := window(0)
	body["kind"], body["title"], body["message"], body["publish"] = "maintenance", "Upgrade", "Planned.", true
	maint := nt.incident(t, body)
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	nt.update(t, maint, map[string]any{"body": "Maintenance remark.", "notify": false})
	nt.owner.expect(http.StatusOK, "PATCH", nt.path("/incidents/"+maint), window(time.Hour))
	ns := nt.notices(t, maint)
	if len(ns) != 2 || ns[0]["state"] != "pending" || len(ns[0]["update_ids"].([]any)) != 0 {
		t.Fatalf("window change notice: %v", ns)
	}
	h.clock.Advance(16 * time.Minute)
	nt.runNotices(t)
	if ns = nt.notices(t, maint); ns[0]["state"] != "sent" {
		t.Fatalf("window change notice: %v", ns)
	}
	moved := lastNoticeMail(t, bs, "ada@example.org")
	for name, body := range map[string]string{"text": moved.Text, "html": moved.HTML} {
		if !strings.Contains(body, "The maintenance is now scheduled for") || strings.Contains(body, "Planned.") ||
			strings.Contains(body, "Maintenance remark.") || strings.Contains(body, "Scheduled:") {
			t.Fatalf("window change %s part: %s", name, body)
		}
	}
}

func TestStatusQuietCloseCancelsPendingNotices(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	nt := newNoticeTeam(t, h, bs)
	nt.subscriber(t, "ada@example.org")

	inc := nt.incident(t, map[string]any{"title": "API down", "message": "Requests fail.", "publish": true})
	ns := nt.notices(t, inc)
	if len(ns) != 1 || ns[0]["state"] != "pending" {
		t.Fatalf("opening notice: %v", ns)
	}
	nt.update(t, inc, map[string]any{"body": "Fixed.", "state": "resolved", "notify": false})
	ns = nt.notices(t, inc)
	if len(ns) != 1 || ns[0]["state"] != "cancelled" || ns[0]["skip_reason"] != "closed_quietly" {
		t.Fatalf("after a quiet close: %v", ns)
	}
	h.clock.Advance(16 * time.Minute)
	nt.runNotices(t)
	if bs.count() != 0 {
		t.Fatal("a notice went out after a quiet close")
	}

	loud := nt.incident(t, map[string]any{"title": "Slow", "message": "Slow.", "publish": true})
	nt.update(t, loud, map[string]any{"body": "Done.", "state": "resolved"})
	ns = nt.notices(t, loud)
	if len(ns) != 1 || ns[0]["state"] != "pending" || ns[0]["final"] != true {
		t.Fatalf("after a notifying close: %v", ns)
	}
}

func TestStatusWindowWordingOnlyForAWindowChange(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	nt := newNoticeTeam(t, h, bs)
	nt.subscriber(t, "ada@example.org")
	start := h.clock.Now().Add(48 * time.Hour)
	window := func(shift time.Duration) map[string]any {
		return map[string]any{"scheduled_start": start.Add(shift).Format(time.RFC3339), "scheduled_end": start.Add(shift + time.Hour).Format(time.RFC3339)}
	}
	body := window(0)
	body["kind"], body["title"], body["message"], body["publish"] = "maintenance", "Upgrade", "Planned.", true
	maint := nt.incident(t, body)
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	first := lastNoticeMail(t, bs, "ada@example.org")
	for name, text := range map[string]string{"text": first.Text, "html": first.HTML} {
		if strings.Contains(text, "now scheduled for") || !strings.Contains(text, "Planned.") || !strings.Contains(text, "Scheduled") {
			t.Fatalf("first announcement %s part: %s", name, text)
		}
	}

	nt.owner.expect(http.StatusOK, "PATCH", nt.path("/incidents/"+maint), window(0))
	if ns := nt.notices(t, maint); len(ns) != 1 {
		t.Fatalf("an unchanged window queued a notice: %v", ns)
	}

	nt.owner.expect(http.StatusOK, "PATCH", nt.path("/incidents/"+maint), window(time.Hour))
	h.clock.Advance(16 * time.Minute)
	nt.runNotices(t)
	moved := lastNoticeMail(t, bs, "ada@example.org")
	if !strings.Contains(moved.Text, "The maintenance is now scheduled for") {
		t.Fatalf("window change: %s", moved.Text)
	}

	nt.update(t, maint, map[string]any{"body": "Still on.", "notify": true})
	h.clock.Advance(16 * time.Minute)
	nt.runNotices(t)
	again := lastNoticeMail(t, bs, "ada@example.org")
	if strings.Contains(again.Text, "now scheduled for") || !strings.Contains(again.Text, "Still on.") {
		t.Fatalf("update after a window change: %s", again.Text)
	}
}

func TestStatusWindowChangeFollowsIncidentNotify(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	nt := newNoticeTeam(t, h, bs)
	nt.subscriber(t, "ada@example.org")
	start := h.clock.Now().Add(48 * time.Hour)
	window := func(shift time.Duration) map[string]any {
		return map[string]any{"scheduled_start": start.Add(shift).Format(time.RFC3339), "scheduled_end": start.Add(shift + time.Hour).Format(time.RFC3339)}
	}
	body := window(0)
	body["kind"], body["title"], body["message"], body["publish"], body["notify"] = "maintenance", "Quiet upgrade", "Planned.", true, false
	maint := nt.incident(t, body)
	nt.owner.expect(http.StatusOK, "PATCH", nt.path("/incidents/"+maint), window(time.Hour))
	if ns := nt.notices(t, maint); len(ns) != 0 {
		t.Fatalf("a not-notify maintenance mailed a window change: %v", ns)
	}
	explicit := window(2 * time.Hour)
	explicit["notify"] = true
	nt.owner.expect(http.StatusOK, "PATCH", nt.path("/incidents/"+maint), explicit)
	if ns := nt.notices(t, maint); len(ns) != 1 {
		t.Fatalf("an explicit notify did not mail the window change: %v", ns)
	}

	loud := window(0)
	loud["kind"], loud["title"], loud["message"], loud["publish"] = "maintenance", "Loud upgrade", "Planned.", true
	other := nt.incident(t, loud)
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	off := window(time.Hour)
	off["notify"] = false
	nt.owner.expect(http.StatusOK, "PATCH", nt.path("/incidents/"+other), off)
	if ns := nt.notices(t, other); len(ns) != 1 {
		t.Fatalf("notify false on the change still queued a notice: %v", ns)
	}
	nt.owner.expect(http.StatusOK, "PATCH", nt.path("/incidents/"+other), window(2*time.Hour))
	if ns := nt.notices(t, other); len(ns) != 2 {
		t.Fatalf("a window change without notify did not follow the incident: %v", ns)
	}
}

func TestStatusAttachTakesOnlyFreeUpdatesOfTheSameIncident(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	nt := newNoticeTeam(t, h, bs)
	nt.subscriber(t, "ada@example.org")
	one := nt.incident(t, map[string]any{"title": "One", "message": "One.", "publish": true})
	two := nt.incident(t, map[string]any{"title": "Two", "message": "Two.", "publish": true})
	noticeOf := func(incident string) (uuid.UUID, uuid.UUID) {
		ns := nt.notices(t, incident)
		return uuid.MustParse(ns[0]["id"].(string)), uuid.MustParse(ns[0]["update_ids"].([]any)[0].(string))
	}
	noticeOne, updateOne := noticeOf(one)
	noticeTwo, updateTwo := noticeOf(two)

	ctx := context.Background()
	ws := uuid.MustParse(nt.ws)
	if err := h.st.AttachStatusNoticeUpdates(ctx, store.AttachStatusNoticeUpdatesParams{
		WorkspaceID: ws, IncidentID: uuid.MustParse(two), NoticeID: &noticeTwo, Ids: []uuid.UUID{updateOne, updateTwo},
	}); err != nil {
		t.Fatal(err)
	}
	owner := func(update uuid.UUID) uuid.UUID {
		var n *uuid.UUID
		if err := h.st.Pool.QueryRow(ctx, "SELECT notice_id FROM status_incident_updates WHERE workspace_id = $1 AND id = $2", ws, update).Scan(&n); err != nil || n == nil {
			t.Fatalf("notice of %s: %v", update, err)
		}
		return *n
	}
	if owner(updateOne) != noticeOne || owner(updateTwo) != noticeTwo {
		t.Fatalf("an update of another incident was moved: %v %v", owner(updateOne), owner(updateTwo))
	}

	if err := h.st.AttachStatusNoticeUpdates(ctx, store.AttachStatusNoticeUpdatesParams{
		WorkspaceID: ws, IncidentID: uuid.MustParse(one), NoticeID: &noticeTwo, Ids: []uuid.UUID{updateOne},
	}); err != nil {
		t.Fatal(err)
	}
	if owner(updateOne) != noticeOne {
		t.Fatal("an update already covered by a notice was re-pointed")
	}
	h.clock.Advance(61 * time.Second)
	nt.runNotices(t)
	if got := len(bs.to("ada@example.org")); got != 2 {
		t.Fatalf("%d notices went out", got)
	}
}

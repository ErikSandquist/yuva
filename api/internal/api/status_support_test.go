package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/productdevbook/yuva/api/internal/api"
)

type supportTeam struct {
	statusTeam
	chatKey, appKey, origin, email string
}

func newSupportTeam(t *testing.T, h *harness) supportTeam {
	t.Helper()
	st := newStatusTeam(t, h)
	origin := "https://" + unique("site") + ".example"
	chat := st.owner.expect(http.StatusCreated, "POST", "/v1/inboxes/"+st.inbox+"/channels", map[string]any{
		"kind": "chat", "name": "Website", "chat": map[string]any{"allowed_origins": []string{origin + "/"}, "allow_anonymous": true},
	})
	app := st.owner.expect(http.StatusCreated, "POST", "/v1/inboxes/"+st.inbox+"/channels", map[string]any{
		"kind": "app", "name": "Apps", "app": map[string]any{"allow_anonymous": true},
	})
	addr := unique("support") + "@example.com"
	st.owner.expect(http.StatusCreated, "POST", "/v1/inboxes/"+st.inbox+"/channels", map[string]any{
		"kind": "email", "name": "Mail",
		"email": map[string]any{"address": addr, "display_name": "Acme", "smtp": map[string]any{"host": "smtp.example.com", "port": 2525, "tls": "starttls"}},
	})
	return supportTeam{statusTeam: st, chatKey: chat.body["chat"].(map[string]any)["public_key"].(string),
		appKey: app.body["app"].(map[string]any)["public_key"].(string), origin: origin, email: addr}
}

func (sp supportTeam) clientStatus(t *testing.T, h *harness) map[string]any {
	t.Helper()
	c := h.client()
	c.origin = sp.origin
	r := c.expect(http.StatusOK, "GET", "/client/v1/channels/"+sp.chatKey, nil)
	s, _ := r.body["status"].(map[string]any)
	return s
}

func (sp supportTeam) visitor(h *harness, key, origin string) contactSession {
	h.t.Helper()
	c := h.client()
	c.origin = origin
	r := c.expect(http.StatusCreated, "POST", "/client/v1/session", map[string]any{"channel_key": key})
	c.bearer = r.str("token")
	return contactSession{client: c, token: r.str("token"), contactID: r.body["contact"].(map[string]any)["id"].(string), body: r.body}
}

func openTitles(s map[string]any) []string {
	var out []string
	if s == nil {
		return nil
	}
	for _, o := range s["open"].([]any) {
		out = append(out, o.(map[string]any)["title"].(string))
	}
	return out
}

func TestClientInboxStatus(t *testing.T) {
	h := newHarness(t)
	sp := newSupportTeam(t, h)

	if s := sp.clientStatus(t, h); s != nil {
		t.Fatalf("an unpublished page shows %v", s)
	}
	sp.owner.expect(http.StatusOK, "PATCH", sp.path(""), map[string]any{"published": true})
	s := sp.clientStatus(t, h)
	if s == nil || s["slug"] != sp.slug || s["state"] != "operational" || s["url"] != testOrigin+"/status/"+sp.slug || len(openTitles(s)) != 0 {
		t.Fatalf("published page status %v", s)
	}

	draft := sp.owner.expect(http.StatusCreated, "POST", sp.path("/incidents"), map[string]any{
		"title": "Draft", "message": "Not yet", "components": []map[string]any{{"component_id": sp.api}},
	})
	now := h.clock.Now()
	sp.owner.expect(http.StatusCreated, "POST", sp.path("/incidents"), map[string]any{
		"kind": "maintenance", "title": "Later", "message": "Upgrade", "publish": true,
		"scheduled_start": now.Add(time.Hour), "scheduled_end": now.Add(2 * time.Hour),
	})
	if got := openTitles(sp.clientStatus(t, h)); len(got) != 0 {
		t.Fatalf("drafts or scheduled maintenance shown: %v", got)
	}
	running := sp.owner.expect(http.StatusCreated, "POST", sp.path("/incidents"), map[string]any{
		"kind": "maintenance", "title": "Now", "message": "Upgrading", "publish": true,
		"scheduled_start": now.Add(-time.Minute), "scheduled_end": now.Add(time.Hour),
	})
	sp.owner.expect(http.StatusOK, "POST", sp.path("/incidents/"+draft.str("id")+"/publish"), nil)
	s = sp.clientStatus(t, h)
	open := s["open"].([]any)
	if s["state"] != "major_outage" || len(open) != 2 {
		t.Fatalf("status with an incident and running maintenance: %v", s)
	}
	inc := open[0].(map[string]any)
	if open[1].(map[string]any)["id"] != running.str("id") && inc["id"] != running.str("id") {
		t.Fatalf("running maintenance missing: %v", open)
	}
	for _, o := range open {
		m := o.(map[string]any)
		if m["url"] != testOrigin+"/status/"+sp.slug+"/incidents/"+m["id"].(string) || m["updated_at"] == nil || m["impact"] == nil || m["state"] == nil {
			t.Fatalf("open item %v", m)
		}
	}

	app := h.client()
	r := app.expect(http.StatusCreated, "POST", "/client/v1/session", map[string]any{"channel_key": sp.appKey})
	if st, _ := r.body["inbox"].(map[string]any)["status"].(map[string]any); len(openTitles(st)) != 2 {
		t.Fatalf("app session status: %s", r.raw)
	}
	app.bearer = r.str("token")
	if st, _ := app.expect(http.StatusOK, "GET", "/client/v1/session", nil).body["inbox"].(map[string]any)["status"].(map[string]any); len(openTitles(st)) != 2 {
		t.Fatal("GET /client/v1/session has no status")
	}

	sp.owner.expect(http.StatusCreated, "POST", sp.path("/incidents/"+draft.str("id")+"/updates"), map[string]any{"body": "Fixed", "state": "resolved"})
	if got := openTitles(sp.clientStatus(t, h)); len(got) != 1 || got[0] != "Now" {
		t.Fatalf("after resolving: %v", got)
	}
	sp.owner.expect(http.StatusOK, "PATCH", sp.path(""), map[string]any{"published": false})
	if s := sp.clientStatus(t, h); s != nil {
		t.Fatalf("unpublished again shows %v", s)
	}

	other := newSupportTeam(t, h)
	if s := other.clientStatus(t, h); s != nil {
		t.Fatalf("another workspace's channel shows %v", s)
	}
}

func TestClientStatusFrameOnChangeOnly(t *testing.T) {
	h := newHarness(t)
	sp := newSupportTeam(t, h)
	sp.owner.expect(http.StatusOK, "PATCH", sp.path(""), map[string]any{"published": true})
	cs := sp.visitor(h, sp.chatKey, sp.origin)
	ws, _, err := dialContact(h, cs.token, sp.origin, "")
	if err != nil {
		t.Fatal(err)
	}
	defer ws.close()
	if m := ws.nextNot("presence"); m.Type != "ready" {
		t.Fatalf("first frame %s %s", m.Type, m.Data)
	}
	frameStatus := func() map[string]any {
		t.Helper()
		m := ws.nextNot("presence")
		if m.Type != "inbox.updated" {
			t.Fatalf("frame %s %s", m.Type, m.Data)
		}
		var in map[string]any
		_ = json.Unmarshal(m.Data, &in)
		s, _ := in["status"].(map[string]any)
		return s
	}

	inc := sp.owner.expect(http.StatusCreated, "POST", sp.path("/incidents"), map[string]any{
		"title": "API down", "message": "Looking", "publish": true, "components": []map[string]any{{"component_id": sp.api}},
	})
	if s := frameStatus(); s["state"] != "major_outage" || strings.Join(openTitles(s), ",") != "API down" {
		t.Fatalf("frame after publishing: %v", s)
	}

	sp.owner.expect(http.StatusOK, "PATCH", sp.path("/components/"+sp.web), map[string]any{"description": "The site"})
	sp.owner.expect(http.StatusCreated, "POST", sp.path("/groups"), map[string]any{"name": "Core"})
	other := newSupportTeam(t, h)
	other.owner.expect(http.StatusOK, "PATCH", other.path(""), map[string]any{"published": true})
	other.owner.expect(http.StatusCreated, "POST", other.path("/incidents"), map[string]any{"title": "Elsewhere", "message": "x", "publish": true})
	ws.quiet(300*time.Millisecond, "presence")

	sp.owner.expect(http.StatusOK, "POST", sp.path("/components/"+sp.web+"/state"), map[string]any{"state": "degraded"})
	ws.quiet(300*time.Millisecond, "presence")
	sp.owner.expect(http.StatusOK, "PATCH", sp.path("/components/"+sp.api), map[string]any{"hidden": true})
	if s := frameStatus(); s["state"] != "degraded" {
		t.Fatalf("frame after hiding the failing component: %v", s)
	}

	sp.owner.expect(http.StatusCreated, "POST", sp.path("/incidents/"+inc.str("id")+"/updates"), map[string]any{"body": "Fixed", "state": "resolved"})
	if s := frameStatus(); len(openTitles(s)) != 0 {
		t.Fatalf("frame after resolving: %v", s)
	}
	sp.owner.expect(http.StatusOK, "PATCH", sp.path(""), map[string]any{"published": false})
	if s := frameStatus(); s != nil {
		t.Fatalf("frame after unpublishing: %v", s)
	}
}

func TestStatusIncidentLinks(t *testing.T) {
	h := newHarness(t)
	sp := newSupportTeam(t, h)
	inc := sp.owner.expect(http.StatusCreated, "POST", sp.path("/incidents"), map[string]any{"title": "Down", "message": "Looking", "publish": true}).str("id")
	other := sp.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Other", "slug": "other"}).body["inbox"].(map[string]any)["id"].(string)
	mine := sp.owner.expect(http.StatusCreated, "POST", "/v1/conversations", map[string]any{"inbox_id": sp.inbox, "contact_id": sp.contact, "subject": "Mine"}).str("id")
	elsewhere := sp.owner.expect(http.StatusCreated, "POST", "/v1/conversations", map[string]any{"inbox_id": other, "contact_id": sp.contact, "subject": "Else"}).str("id")
	link := func(conv string) string { return sp.path("/incidents/" + inc + "/conversations/" + conv) }

	sp.agent.expectProblem(http.StatusNotFound, "not_found", "PUT", link(mine), nil)
	sp.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+sp.inbox+"/members/"+sp.agentID, nil)
	sp.agent.expect(http.StatusNoContent, "PUT", link(mine), nil)
	sp.agent.expect(http.StatusNoContent, "PUT", link(mine), nil)
	sp.agent.expectProblem(http.StatusNotFound, "not_found", "PUT", link(elsewhere), nil)
	sp.owner.expect(http.StatusNoContent, "PUT", link(elsewhere), nil)
	sp.agent.expectProblem(http.StatusNotFound, "not_found", "DELETE", link(elsewhere), nil)
	sp.agent.expectProblem(http.StatusNotFound, "not_found", "PUT", sp.path("/incidents/"+sp.api+"/conversations/"+mine), nil)

	items := sp.agent.expect(http.StatusOK, "GET", sp.path("/incidents/"+inc+"/conversations"), nil).body["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != mine || items[0].(map[string]any)["status"] != "open" || items[0].(map[string]any)["linked_at"] == nil {
		t.Fatalf("agent sees linked %v", items)
	}
	if items := sp.owner.expect(http.StatusOK, "GET", sp.path("/incidents/"+inc+"/conversations?limit=1"), nil); len(items.body["items"].([]any)) != 1 || items.str("next_cursor") == "" {
		t.Fatalf("owner first page %s", items.raw)
	}
	links := sp.agent.expect(http.StatusOK, "GET", "/v1/conversations/"+mine+"/incidents", nil).body["items"].([]any)
	if len(links) != 1 || links[0].(map[string]any)["id"] != inc || links[0].(map[string]any)["page"].(map[string]any)["slug"] != sp.slug {
		t.Fatalf("conversation incidents %v", links)
	}
	sp.owner.expect(http.StatusNoContent, "DELETE", "/v1/inboxes/"+sp.inbox+"/members/"+sp.agentID, nil)
	sp.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+other+"/members/"+sp.agentID, nil)
	if links := sp.agent.expect(http.StatusOK, "GET", "/v1/conversations/"+elsewhere+"/incidents", nil).body["items"].([]any); len(links) != 0 {
		t.Fatalf("an agent without the page's inbox sees %v", links)
	}
	sp.agent.expectProblem(http.StatusNotFound, "not_found", "PUT", link(elsewhere), nil)

	stranger := newTeam(t, h)
	stranger.owner.expectProblem(http.StatusNotFound, "not_found", "PUT", link(mine), nil)
	stranger.owner.expectProblem(http.StatusNotFound, "not_found", "GET", sp.path("/incidents/"+inc+"/conversations"), nil)
	stranger.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/conversations/"+mine+"/incidents", nil)
	strangerConv := stranger.owner.expect(http.StatusCreated, "POST", "/v1/conversations", map[string]any{"inbox_id": stranger.inbox, "contact_id": stranger.contact}).str("id")
	sp.owner.expectProblem(http.StatusNotFound, "not_found", "PUT", link(strangerConv), nil)

	key := h.client()
	key.bearer = sp.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "k", "scopes": []string{"status:manage"}}).str("secret")
	key.expectProblem(http.StatusForbidden, "insufficient_scope", "PUT", link(mine), nil)
	key.bearer = sp.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "k2", "scopes": []string{"status:manage", "conversations:read"}}).str("secret")
	key.expect(http.StatusNoContent, "DELETE", link(mine), nil)
	if items := sp.owner.expect(http.StatusOK, "GET", sp.path("/incidents/"+inc+"/conversations"), nil).body["items"].([]any); len(items) != 1 {
		t.Fatalf("after unlinking %v", items)
	}
}

func TestReplyToLinked(t *testing.T) {
	h := newHarness(t)
	sp := newSupportTeam(t, h)
	inc := sp.owner.expect(http.StatusCreated, "POST", sp.path("/incidents"), map[string]any{"title": "Down", "message": "Looking", "publish": true}).str("id")

	mailConv := h.ingest(sp.email, buildMail(mailOpts{from: "cust@example.net", to: sp.email, subject: "Broken", messageID: newMessageID(), body: "It is down"}), nil).str("conversation_id")
	chat := sp.visitor(h, sp.chatKey, sp.origin)
	chatConv := chat.start("chat says down")
	app := sp.visitor(h, sp.appKey, "")
	appConv := app.start("app says down")
	closed := sp.owner.expect(http.StatusCreated, "POST", "/v1/conversations", map[string]any{"inbox_id": sp.inbox, "contact_id": sp.contact}).str("id")
	sp.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+closed, map[string]any{"status": "closed"})
	spam := sp.owner.expect(http.StatusCreated, "POST", "/v1/conversations", map[string]any{"inbox_id": sp.inbox, "contact_id": sp.contact}).str("id")
	sp.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+spam, map[string]any{"spam": true})
	for _, c := range []string{mailConv, chatConv, appConv, closed, spam} {
		sp.owner.expect(http.StatusNoContent, "PUT", sp.path("/incidents/"+inc+"/conversations/"+c), nil)
	}

	sp.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", sp.path("/incidents/"+inc+"/reply"), map[string]any{"body": "  "})
	bot := h.client()
	bot.bearer = sp.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "bot", "scopes": []string{"status:manage", "messages:write"}}).str("secret")
	bot.expectProblem(http.StatusForbidden, "bot_sending_disabled", "POST", sp.path("/incidents/"+inc+"/reply"), map[string]any{"body": "Hi"})
	stranger := newTeam(t, h)
	stranger.owner.expectProblem(http.StatusNotFound, "not_found", "POST", sp.path("/incidents/"+inc+"/reply"), map[string]any{"body": "Hi"})

	r := sp.owner.expect(http.StatusAccepted, "POST", sp.path("/incidents/"+inc+"/reply"), map[string]any{"body": "We are on it.", "close": true})
	if r.body["queued"] != float64(3) {
		t.Fatalf("queued %s", r.raw)
	}
	jobs := h.jobs("status_reply", sp.ws)
	if len(jobs) != 3 {
		t.Fatalf("%d jobs", len(jobs))
	}
	run := func() {
		t.Helper()
		for _, j := range jobs {
			var a api.StatusReplyArgs
			if err := json.Unmarshal(j.args, &a); err != nil {
				t.Fatal(err)
			}
			if err := h.srv.RunStatusReply(context.Background(), a); err != nil {
				t.Fatal(err)
			}
		}
	}
	replies := func(conv string) []map[string]any {
		var out []map[string]any
		for _, m := range messages(sp.owner, conv) {
			if m["kind"] == "message" && m["direction"] == "out" && m["body"] == "We are on it." {
				out = append(out, m)
			}
		}
		return out
	}
	sp.owner.expect(http.StatusNoContent, "DELETE", sp.path("/incidents/"+inc+"/conversations/"+appConv), nil)
	run()
	sp.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+chatConv, map[string]any{"status": "open"})
	run()
	if len(replies(appConv)) != 0 {
		t.Fatal("a conversation unlinked before its job got the reply")
	}

	for _, c := range []string{mailConv, chatConv} {
		got := replies(c)
		if len(got) != 1 {
			t.Fatalf("conversation %s got %d replies", c, len(got))
		}
		if a := got[0]["author"].(map[string]any); a["type"] != "member" {
			t.Fatalf("reply author %v", a)
		}
	}
	if len(replies(closed)) != 0 || len(replies(spam)) != 0 {
		t.Fatal("a closed or spam conversation got the reply")
	}
	for _, c := range []string{mailConv} {
		if s := sp.owner.expect(http.StatusOK, "GET", "/v1/conversations/"+c, nil).str("status"); s != "closed" {
			t.Fatalf("conversation %s is %s", c, s)
		}
	}
	if s := sp.owner.expect(http.StatusOK, "GET", "/v1/conversations/"+chatConv, nil).str("status"); s != "open" {
		t.Fatalf("reopened chat conversation is %s after the finished job ran again", s)
	}
	rep := sp.owner.expect(http.StatusOK, "GET", sp.path("/incidents/"+inc+"/replies/"+r.str("id")), nil)
	if c := rep.body["counts"].(map[string]any); c["sent"] != float64(2) || c["skipped"] != float64(1) || c["pending"] != float64(0) || c["failed"] != float64(0) || rep.body["queued"] != float64(3) || rep.body["close"] != true {
		t.Fatalf("report %s", rep.raw)
	}
	byConv := outcomes(rep)
	if byConv[mailConv] != "sent" || byConv[chatConv] != "sent" || byConv[appConv] != "skipped:unlinked" || len(byConv) != 3 {
		t.Fatalf("outcomes %v", byConv)
	}
	if em, _ := replies(mailConv)[0]["email"].(map[string]any); em == nil {
		t.Fatalf("the e-mail conversation's reply was not mailed: %v", replies(mailConv)[0])
	}
	if n := h.count("SELECT count(*) FROM river_job WHERE kind = 'email_send' AND args->>'workspace_id' = $1", sp.ws); n != 1 {
		t.Fatalf("%d email_send jobs, want the one of the e-mail conversation", n)
	}
	for _, c := range []struct {
		s    contactSession
		conv string
	}{{chat, chatConv}} {
		msgs := c.s.expect(http.StatusOK, "GET", "/client/v1/conversations/"+c.conv+"/messages", nil).body["items"].([]any)
		if !strings.Contains(fmt.Sprint(msgs), "We are on it.") {
			t.Fatalf("contact does not see the reply in %s: %v", c.conv, msgs)
		}
	}
}

func outcomes(r response) map[string]string {
	out := map[string]string{}
	for _, it := range r.body["items"].([]any) {
		m := it.(map[string]any)
		v := m["outcome"].(string)
		if reason, ok := m["reason"].(string); ok {
			v += ":" + reason
		}
		out[m["conversation_id"].(string)] = v
	}
	return out
}

func (h *harness) runStatusReplies(t *testing.T, ws string) {
	t.Helper()
	for _, j := range h.jobs("status_reply", ws) {
		var a api.StatusReplyArgs
		if err := json.Unmarshal(j.args, &a); err != nil {
			t.Fatal(err)
		}
		if err := h.srv.RunStatusReply(context.Background(), a); err != nil {
			t.Fatal(err)
		}
	}
}

func TestClientStatusResumeFrame(t *testing.T) {
	h := newHarness(t)
	sp := newSupportTeam(t, h)
	sp.owner.expect(http.StatusOK, "PATCH", sp.path(""), map[string]any{"published": true})
	cs := sp.visitor(h, sp.chatKey, sp.origin)
	ws, _, err := dialContact(h, cs.token, sp.origin, "")
	if err != nil {
		t.Fatal(err)
	}
	ready := ws.nextNot("presence")
	if ready.Type != "ready" {
		t.Fatalf("first frame %s", ready.Type)
	}
	ws.close()

	sp.owner.expect(http.StatusCreated, "POST", sp.path("/incidents"), map[string]any{
		"title": "API down", "message": "Looking", "publish": true, "components": []map[string]any{{"component_id": sp.api}},
	})
	resume := func(from int64) []wsMessage {
		t.Helper()
		c, _, err := dialContact(h, cs.token, sp.origin, fmt.Sprintf("?last_event_id=%d", from))
		if err != nil {
			t.Fatal(err)
		}
		defer c.close()
		msgs := c.until(func(m wsMessage) bool { return m.Type == "inbox.updated" })
		var types []string
		for _, m := range msgs {
			types = append(types, m.Type)
		}
		if strings.Join(types, " ") != "ready inbox.updated" {
			t.Fatalf("frames on resume: %v", types)
		}
		return msgs
	}
	last := resume(ready.LastEventID)[1]
	var in map[string]any
	_ = json.Unmarshal(last.Data, &in)
	st, _ := in["status"].(map[string]any)
	if st["state"] != "major_outage" || strings.Join(openTitles(st), ",") != "API down" {
		t.Fatalf("status missed while away: %s", last.Data)
	}
	if again := resume(ready.LastEventID)[1]; again.Type != "inbox.updated" {
		t.Fatalf("second resume: %s", again.Type)
	}
}

func TestStatusLinkConcurrentDelete(t *testing.T) {
	h := newHarness(t)
	sp := newSupportTeam(t, h)
	inc := sp.owner.expect(http.StatusCreated, "POST", sp.path("/incidents"), map[string]any{"title": "Down", "message": "Looking", "publish": true}).str("id")
	for i := 0; i < 12; i++ {
		contact := sp.owner.expect(http.StatusCreated, "POST", "/v1/contacts", map[string]any{"name": fmt.Sprint("C", i)}).str("id")
		conv := sp.owner.expect(http.StatusCreated, "POST", "/v1/conversations", map[string]any{"inbox_id": sp.inbox, "contact_id": contact}).str("id")
		var got [2]int
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			got[0] = sp.owner.do("PUT", sp.path("/incidents/"+inc+"/conversations/"+conv), nil).status
		}()
		go func() {
			defer wg.Done()
			got[1] = sp.owner.do("DELETE", "/v1/contacts/"+contact, nil).status
		}()
		wg.Wait()
		if got[0] != http.StatusNoContent && got[0] != http.StatusNotFound {
			t.Fatalf("link during a delete: status %d", got[0])
		}
	}
	sp.owner.expectProblem(http.StatusNotFound, "not_found", "PUT", sp.path("/incidents/"+inc+"/conversations/"+sp.api), nil)

	conv := sp.owner.expect(http.StatusCreated, "POST", "/v1/conversations", map[string]any{"inbox_id": sp.inbox, "contact_id": sp.contact}).str("id")
	for i := 0; i < 12; i++ {
		draft := sp.owner.expect(http.StatusCreated, "POST", sp.path("/incidents"), map[string]any{"title": "Draft", "message": "x"}).str("id")
		var got int
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			got = sp.owner.do("PUT", sp.path("/incidents/"+draft+"/conversations/"+conv), nil).status
		}()
		go func() {
			defer wg.Done()
			sp.owner.do("DELETE", sp.path("/incidents/"+draft), nil)
		}()
		wg.Wait()
		if got != http.StatusNoContent && got != http.StatusNotFound {
			t.Fatalf("link during an incident delete: status %d", got)
		}
	}
}

func TestStatusLinkLimit(t *testing.T) {
	h := newHarness(t)
	sp := newSupportTeam(t, h)
	defer api.SetMaxIncidentLinks(2)()
	inc := sp.owner.expect(http.StatusCreated, "POST", sp.path("/incidents"), map[string]any{"title": "Down", "message": "Looking", "publish": true}).str("id")
	conv := func() string {
		return sp.owner.expect(http.StatusCreated, "POST", "/v1/conversations", map[string]any{"inbox_id": sp.inbox, "contact_id": sp.contact}).str("id")
	}
	a, b, c := conv(), conv(), conv()
	link := func(conv string) string { return sp.path("/incidents/" + inc + "/conversations/" + conv) }
	sp.owner.expect(http.StatusNoContent, "PUT", link(a), nil)
	sp.owner.expect(http.StatusNoContent, "PUT", link(b), nil)
	sp.owner.expect(http.StatusNoContent, "PUT", link(a), nil)
	r := sp.owner.expect(http.StatusUnprocessableEntity, "PUT", link(c), nil)
	if r.str("code") != "too_many_links" || !strings.Contains(r.str("detail"), "at most 2") {
		t.Fatalf("limit problem %s", r.raw)
	}
	sp.owner.expect(http.StatusNoContent, "DELETE", link(a), nil)
	sp.owner.expect(http.StatusNoContent, "PUT", link(c), nil)
}

func TestReplyOutcomes(t *testing.T) {
	h := newHarness(t)
	sp := newSupportTeam(t, h)
	defer api.SetStatusReplyBatchSize(2)()
	inc := sp.owner.expect(http.StatusCreated, "POST", sp.path("/incidents"), map[string]any{"title": "Down", "message": "Looking", "publish": true}).str("id")
	var convs []string
	for i := 0; i < 5; i++ {
		c := sp.owner.expect(http.StatusCreated, "POST", "/v1/conversations", map[string]any{"inbox_id": sp.inbox, "contact_id": sp.contact, "subject": fmt.Sprint("C", i)}).str("id")
		sp.owner.expect(http.StatusNoContent, "PUT", sp.path("/incidents/"+inc+"/conversations/"+c), nil)
		convs = append(convs, c)
	}
	sp.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+sp.inbox+"/members/"+sp.agentID, nil)

	reply := sp.agent.expect(http.StatusAccepted, "POST", sp.path("/incidents/"+inc+"/reply"), map[string]any{"body": "Working on it"})
	if reply.body["queued"] != float64(5) || reply.str("id") == "" {
		t.Fatalf("reply %s", reply.raw)
	}
	id := reply.str("id")
	if n := h.count("SELECT count(*) FROM status_incident_replies WHERE workspace_id = $1", sp.ws); n != 1 {
		t.Fatalf("%d reply rows", n)
	}
	jobs := h.jobs("status_reply", sp.ws)
	if len(jobs) != 5 || strings.Contains(string(jobs[0].args), "Working on it") {
		t.Fatalf("%d jobs, args %s", len(jobs), jobs[0].args)
	}
	path := sp.path("/incidents/" + inc + "/replies/" + id)
	pending := sp.agent.expect(http.StatusOK, "GET", path, nil)
	if c := pending.body["counts"].(map[string]any); c["pending"] != float64(5) || len(pending.body["items"].([]any)) != 5 {
		t.Fatalf("before the jobs ran: %s", pending.raw)
	}

	sp.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+convs[0], map[string]any{"status": "closed"})
	sp.owner.expect(http.StatusNoContent, "DELETE", sp.path("/incidents/"+inc+"/conversations/"+convs[1]), nil)
	sp.owner.expect(http.StatusNoContent, "DELETE", "/v1/inboxes/"+sp.inbox+"/members/"+sp.agentID, nil)
	h.runStatusReplies(t, sp.ws)

	done := sp.owner.expect(http.StatusOK, "GET", path, nil)
	got := outcomes(done)
	want := map[string]string{convs[0]: "skipped:closed", convs[1]: "skipped:unlinked", convs[2]: "failed:not_found", convs[3]: "failed:not_found", convs[4]: "failed:not_found"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("outcomes %v, want %v", got, want)
	}
	if c := done.body["counts"].(map[string]any); c["skipped"] != float64(2) || c["failed"] != float64(3) || c["pending"] != float64(0) {
		t.Fatalf("counts %v", c)
	}
	if done.body["items"].([]any)[0].(map[string]any)["finished_at"] == nil {
		t.Fatalf("no finished_at: %s", done.raw)
	}

	page := sp.owner.expect(http.StatusOK, "GET", path+"?limit=2", nil)
	if len(page.body["items"].([]any)) != 2 || page.str("next_cursor") == "" {
		t.Fatalf("first page %s", page.raw)
	}
	seen := 2
	for cur := page.str("next_cursor"); cur != ""; {
		next := sp.owner.expect(http.StatusOK, "GET", path+"?limit=2&cursor="+cur, nil)
		seen += len(next.body["items"].([]any))
		cur = next.str("next_cursor")
	}
	if seen != 5 {
		t.Fatalf("pages cover %d targets", seen)
	}

	sp.owner.expectProblem(http.StatusNotFound, "not_found", "GET", sp.path("/incidents/"+inc+"/replies/"+inc), nil)
	sp.agent.expectProblem(http.StatusNotFound, "not_found", "GET", path, nil)
	stranger := newTeam(t, h)
	stranger.owner.expectProblem(http.StatusNotFound, "not_found", "GET", path, nil)
	other := newSupportTeam(t, h)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", other.path("/incidents/"+inc+"/replies/"+id), nil)
}

func TestReplyRechecksCaller(t *testing.T) {
	h := newHarness(t)
	sp := newSupportTeam(t, h)
	sp.owner.expect(http.StatusOK, "PATCH", "/v1/workspace", map[string]any{"bots_may_send": true})
	inc := sp.owner.expect(http.StatusCreated, "POST", sp.path("/incidents"), map[string]any{"title": "Down", "message": "Looking", "publish": true}).str("id")
	conv := sp.owner.expect(http.StatusCreated, "POST", "/v1/conversations", map[string]any{"inbox_id": sp.inbox, "contact_id": sp.contact}).str("id")
	sp.owner.expect(http.StatusNoContent, "PUT", sp.path("/incidents/"+inc+"/conversations/"+conv), nil)
	replyPath := sp.path("/incidents/" + inc + "/reply")

	granted := h.client()
	granted.bearer = h.oauthToken(sp.owner, sp.ws, "Claude Code", testOrigin, []string{"status:manage", "messages:write"})
	live := granted.expect(http.StatusAccepted, "POST", replyPath, map[string]any{"body": "From the grant"}).str("id")
	h.runStatusReplies(t, sp.ws)
	if got := outcomes(sp.owner.expect(http.StatusOK, "GET", sp.path("/incidents/"+inc+"/replies/"+live), nil)); got[conv] != "sent" {
		t.Fatalf("grant reply outcome %v", got)
	}

	revoked := granted.expect(http.StatusAccepted, "POST", replyPath, map[string]any{"body": "Revoked grant"}).str("id")
	for _, g := range sp.owner.expect(http.StatusOK, "GET", "/v1/oauth/grants", nil).body["items"].([]any) {
		sp.owner.expect(http.StatusNoContent, "DELETE", "/v1/oauth/grants/"+g.(map[string]any)["id"].(string), nil)
	}
	keyed := h.client()
	key := sp.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "bot", "scopes": []string{"status:manage", "messages:write"}})
	keyed.bearer = key.str("secret")
	byKey := keyed.expect(http.StatusAccepted, "POST", replyPath, map[string]any{"body": "Revoked key"}).str("id")
	sp.owner.expect(http.StatusNoContent, "DELETE", "/v1/api-keys/"+key.body["api_key"].(map[string]any)["id"].(string), nil)
	h.runStatusReplies(t, sp.ws)

	for id, body := range map[string]string{revoked: "Revoked grant", byKey: "Revoked key"} {
		if got := outcomes(sp.owner.expect(http.StatusOK, "GET", sp.path("/incidents/"+inc+"/replies/"+id), nil)); got[conv] != "skipped:revoked" {
			t.Fatalf("%s outcome %v", body, got)
		}
		for _, m := range messages(sp.owner, conv) {
			if m["body"] == body {
				t.Fatalf("%q was sent after its caller was revoked", body)
			}
		}
	}
	sent := 0
	for _, m := range messages(sp.owner, conv) {
		if m["body"] == "From the grant" {
			sent++
		}
	}
	if sent != 1 {
		t.Fatalf("the live grant's reply was sent %d times", sent)
	}
}

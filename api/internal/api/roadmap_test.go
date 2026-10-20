package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/productdevbook/yuva/api/internal/api"
)

type roadmapTeam struct {
	chatTeam
	board, slug string
}

func newRoadmapTeam(t *testing.T, h *harness, public bool) roadmapTeam {
	t.Helper()
	ct := newChatTeam(t, h, "live", true)
	slug := unique("rm")
	b := ct.owner.expect(http.StatusCreated, "POST", "/v1/roadmap-boards", map[string]any{
		"inbox_id": ct.chatInbox, "channel_id": ct.channel, "name": "Acme roadmap", "slug": slug, "public": public,
	})
	return roadmapTeam{chatTeam: ct, board: b.str("id"), slug: slug}
}

func (rt roadmapTeam) feedback(h *harness) string {
	h.t.Helper()
	cs := rt.session(h, map[string]any{})
	return cs.expect(http.StatusCreated, "POST", "/client/v1/feedback", map[string]any{"category": "idea", "body": "please add " + unique("thing")}).body["conversation"].(map[string]any)["id"].(string)
}

func (rt roadmapTeam) idea(c *client, title string) string {
	rt.h().t.Helper()
	return c.expect(http.StatusCreated, "POST", "/v1/discussions", map[string]any{"kind": "idea", "board_id": rt.board, "title": title, "body": "Details of " + title}).str("id")
}

func (rt roadmapTeam) h() *harness { return rt.owner.h }

func (rt roadmapTeam) scopedKey(scopes ...string) *client {
	rt.h().t.Helper()
	body := map[string]any{"name": unique("key")}
	if scopes != nil {
		body["scopes"] = scopes
	}
	k := rt.h().client()
	k.bearer = rt.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", body).str("secret")
	return k
}

func (rt roadmapTeam) progress(conv string) any {
	rt.h().t.Helper()
	f, _ := rt.owner.expect(http.StatusOK, "GET", "/v1/conversations/"+conv, nil).body["feedback"].(map[string]any)
	return f["progress"]
}

func (rt roadmapTeam) runProgress(t *testing.T) {
	t.Helper()
	for _, j := range rt.h().jobs("roadmap_progress", rt.ws) {
		var a api.RoadmapProgressArgs
		if err := json.Unmarshal(j.args, &a); err != nil {
			t.Fatal(err)
		}
		if err := rt.h().srv.RunRoadmapProgress(context.Background(), a); err != nil {
			t.Fatal(err)
		}
	}
}

func (rt roadmapTeam) changes(idea string) []map[string]any {
	rt.h().t.Helper()
	return items(rt.owner.expect(http.StatusOK, "GET", "/v1/discussions/"+idea+"/stage-changes", nil))
}

func TestRoadmapBoardsAndRoles(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, false)
	other := newTeam(t, h)

	b := rt.owner.expect(http.StatusOK, "GET", "/v1/roadmap-boards/"+rt.board, nil)
	if b.body["public"] != false || b.str("suggestions") != "moderated" || !strings.HasSuffix(b.str("url"), "/roadmap/"+rt.slug) || b.body["counts"].(map[string]any)["ideas"] != float64(0) {
		t.Fatalf("new board %s", b.raw)
	}
	rt.agent.expectProblem(http.StatusForbidden, "forbidden", "POST", "/v1/roadmap-boards", map[string]any{"inbox_id": rt.chatInbox, "channel_id": rt.channel, "name": "x", "slug": unique("rm")})
	rt.agent.expectProblem(http.StatusForbidden, "forbidden", "PATCH", "/v1/roadmap-boards/"+rt.board, map[string]any{"public": true})
	rt.agent.expectProblem(http.StatusForbidden, "forbidden", "DELETE", "/v1/roadmap-boards/"+rt.board, nil)
	if got := items(rt.agent.expect(http.StatusOK, "GET", "/v1/roadmap-boards", nil)); len(got) != 1 {
		t.Fatalf("an agent with the inbox lists %v", got)
	}
	rt.owner.expectProblem(http.StatusConflict, "roadmap_board_exists", "POST", "/v1/roadmap-boards", map[string]any{"inbox_id": rt.chatInbox, "channel_id": rt.channel, "name": "again", "slug": unique("rm")})

	second := rt.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Second", "slug": "second"}).body["inbox"].(map[string]any)["id"].(string)
	secondChat := rt.owner.expect(http.StatusCreated, "POST", "/v1/inboxes/"+second+"/channels", map[string]any{"kind": "chat", "name": "Site", "chat": map[string]any{"allowed_origins": []string{rt.origin}}}).str("id")
	rt.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/roadmap-boards", map[string]any{"inbox_id": second, "channel_id": rt.channel, "name": "x", "slug": unique("rm")})
	rt.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/roadmap-boards", map[string]any{"inbox_id": second, "channel_id": secondChat, "name": "x", "slug": "status"})
	rt.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/roadmap-boards", map[string]any{"inbox_id": second, "channel_id": secondChat, "name": "x", "slug": "Bad_Slug"})
	rt.owner.expectProblem(http.StatusConflict, "slug_taken", "POST", "/v1/roadmap-boards", map[string]any{"inbox_id": second, "channel_id": secondChat, "name": "x", "slug": rt.slug})

	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/roadmap-boards/"+rt.board, nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "PATCH", "/v1/roadmap-boards/"+rt.board, map[string]any{"public": true})
	other.owner.expectProblem(http.StatusNotFound, "not_found", "DELETE", "/v1/roadmap-boards/"+rt.board, nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/discussions", map[string]any{"kind": "idea", "board_id": rt.board, "title": "Hello there", "body": "x"})
	if got := items(other.owner.expect(http.StatusOK, "GET", "/v1/roadmap-boards", nil)); len(got) != 0 {
		t.Fatalf("another workspace lists %v", got)
	}

	renamed := unique("rm")
	u := rt.owner.expect(http.StatusOK, "PATCH", "/v1/roadmap-boards/"+rt.board, map[string]any{"slug": renamed, "public": true, "suggestions": "open"})
	if u.str("slug") != renamed || u.body["public"] != true || u.str("suggestions") != "open" {
		t.Fatalf("updated board %s", u.raw)
	}
	if h.count("SELECT count(*) FROM roadmap_board_slugs WHERE slug = $1", rt.slug) != 1 {
		t.Fatal("the former slug is not kept")
	}
	rt.owner.expectProblem(http.StatusConflict, "slug_taken", "POST", "/v1/roadmap-boards", map[string]any{"inbox_id": second, "channel_id": secondChat, "name": "x", "slug": rt.slug})

	idea := rt.idea(rt.owner, "Dark mode")
	rt.owner.expect(http.StatusNoContent, "DELETE", "/v1/roadmap-boards/"+rt.board, nil)
	rt.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/discussions/"+idea, nil)
	if n := h.count("SELECT count(*) FROM discussions WHERE workspace_id = $1", rt.ws); n != 0 {
		t.Fatalf("%d discussions survive the board", n)
	}
}

func TestRoadmapScopesAreNeverImplied(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, false)
	idea := rt.idea(rt.owner, "Dark mode")

	scopes := rt.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "plain"}).body["api_key"].(map[string]any)["scopes"].([]any)
	has := func(s string) bool {
		for _, v := range scopes {
			if v == s {
				return true
			}
		}
		return false
	}
	if !has("roadmap:read") || !has("roadmap:write") || has("roadmap:stage") || has("status:manage") {
		t.Fatalf("scopes of a key made without naming them: %v", scopes)
	}
	plain := rt.scopedKey()
	if r := plain.expect(http.StatusForbidden, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "planned"}); r.str("code") != "insufficient_scope" || r.str("scope") != "roadmap:stage" {
		t.Fatalf("unnamed roadmap:stage %s", r.raw)
	}
	reader := rt.scopedKey("roadmap:read", "conversations:read")
	reader.expect(http.StatusOK, "GET", "/v1/roadmap-boards/"+rt.board, nil)
	reader.expect(http.StatusOK, "GET", "/v1/discussions/"+idea+"/stage-changes", nil)
	if r := reader.expect(http.StatusForbidden, "PATCH", "/v1/roadmap-boards/"+rt.board, map[string]any{"public": true}); r.str("scope") != "roadmap:write" {
		t.Fatalf("board settings need roadmap:write: %s", r.raw)
	}
	conv := rt.feedback(h)
	if r := reader.expect(http.StatusForbidden, "PUT", "/v1/discussions/"+idea+"/conversations/"+conv, nil); r.str("scope") != "roadmap:write" {
		t.Fatalf("linking needs roadmap:write: %s", r.raw)
	}
	if r := rt.scopedKey("roadmap:write").expect(http.StatusForbidden, "PUT", "/v1/discussions/"+idea+"/conversations/"+conv, nil); r.str("scope") != "conversations:read" {
		t.Fatalf("linking needs conversations:read: %s", r.raw)
	}
	if r := rt.scopedKey("roadmap:read", "roadmap:write", "conversations:read").expect(http.StatusForbidden, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "planned"}); r.str("scope") != "roadmap:stage" {
		t.Fatalf("roadmap:write does not move stages: %s", r.raw)
	}

	named := rt.scopedKey("roadmap:stage", "roadmap:read")
	if r := named.expect(http.StatusOK, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "planned"}); r.body["change"].(map[string]any)["api_key_id"] == nil {
		t.Fatalf("a key's move %s", r.raw)
	}
	named.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "shipped", "comment": "Done"})
	if rt.owner.expect(http.StatusOK, "GET", "/v1/discussions/"+idea, nil).body["discussion"].(map[string]any)["stage"] != "planned" {
		t.Fatal("a refused move changed the stage")
	}
}

func TestRoadmapStageMovesSetLinkedProgress(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, true)
	idea := rt.idea(rt.agent, "Dark mode")
	convs := []string{rt.feedback(h), rt.feedback(h), rt.feedback(h)}
	for _, c := range convs {
		rt.agent.expect(http.StatusNoContent, "PUT", "/v1/discussions/"+idea+"/conversations/"+c, nil)
		rt.agent.expect(http.StatusNoContent, "PUT", "/v1/discussions/"+idea+"/conversations/"+c, nil)
	}
	d := rt.agent.expect(http.StatusOK, "GET", "/v1/discussions/"+idea, nil).body["discussion"].(map[string]any)
	if d["linked_count"] != float64(3) || d["stage"] != "ideas" || d["kind"] != "idea" || d["page"] != "" {
		t.Fatalf("idea %v", d)
	}
	linked := items(rt.agent.expect(http.StatusOK, "GET", "/v1/conversations?idea_id="+idea, nil))
	if len(linked) != 3 || linked[0]["idea"].(map[string]any)["id"] != idea {
		t.Fatalf("linked feedback %v", linked)
	}
	free := rt.feedback(h)
	if got := items(rt.agent.expect(http.StatusOK, "GET", "/v1/conversations?idea=none", nil)); len(got) != 1 || got[0]["id"] != free {
		t.Fatalf("feedback without an idea %v", got)
	}
	rt.agent.expectProblem(http.StatusBadRequest, "validation_failed", "GET", "/v1/conversations?idea=some", nil)

	r := rt.agent.expect(http.StatusOK, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "planned", "comment": "We started on it."})
	change := r.body["change"].(map[string]any)
	if change["linked"] != float64(3) || change["progress"] != "planned" || change["comment_post_id"] == nil || r.body["discussion"].(map[string]any)["stage"] != "planned" {
		t.Fatalf("move %s", r.raw)
	}
	if r := rt.agent.expect(http.StatusOK, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "planned"}); r.body["change"] != nil {
		t.Fatalf("moving to the stage it is in recorded a change: %s", r.raw)
	}
	for _, c := range convs {
		if rt.progress(c) != nil {
			t.Fatal("progress is set before the job ran")
		}
	}
	rt.runProgress(t)
	for _, c := range convs {
		if rt.progress(c) != "planned" {
			t.Fatalf("progress after planned: %v", rt.progress(c))
		}
		var seen bool
		for _, m := range messages(rt.owner, c) {
			if ev, _ := m["event"].(map[string]any); ev["type"] == "feedback_progress" && ev["idea_id"] == idea && ev["feedback_progress"] == "planned" {
				seen = true
			}
		}
		if !seen {
			t.Fatalf("no feedback_progress event naming the idea on %s", c)
		}
	}
	if ch := rt.changes(idea); ch[0]["finished_at"] == nil || ch[0]["progress_set"] != float64(3) || ch[0]["progress_skipped"] != float64(0) {
		t.Fatalf("change after the job %v", ch[0])
	}
	if free := rt.progress(free); free != nil {
		t.Fatalf("unlinked feedback got progress %v", free)
	}

	rt.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+convs[0], map[string]any{"feedback_progress": "wont_do"})
	rt.runProgress(t)
	if rt.progress(convs[0]) != "wont_do" {
		t.Fatal("a rerun of an earlier job changed a hand-set progress")
	}

	rt.agent.expect(http.StatusOK, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "in_progress"})
	rt.agent.expect(http.StatusOK, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "shipped"})
	rt.runProgress(t)
	for _, c := range convs {
		if rt.progress(c) != "done" {
			t.Fatalf("progress after shipped: %v", rt.progress(c))
		}
	}
	ch := rt.changes(idea)
	if len(ch) != 3 || ch[0]["to_stage"] != "shipped" || ch[1]["to_stage"] != "in_progress" || ch[1]["progress_set"] != float64(0) || ch[1]["progress_skipped"] != float64(3) || ch[1]["finished_at"] == nil {
		t.Fatalf("a superseded move %v", ch)
	}
	if ch[0]["progress_set"] != float64(3) {
		t.Fatalf("the newest move %v", ch[0])
	}

	late := rt.feedback(h)
	rt.agent.expect(http.StatusNoContent, "PUT", "/v1/discussions/"+idea+"/conversations/"+late, nil)
	if rt.progress(late) != "done" {
		t.Fatal("linking to a shipped idea did not set the progress at once")
	}
	rt.agent.expect(http.StatusNoContent, "DELETE", "/v1/discussions/"+idea+"/conversations/"+late, nil)
	rt.agent.expect(http.StatusNoContent, "DELETE", "/v1/discussions/"+idea+"/conversations/"+late, nil)
	if rt.progress(late) != "done" {
		t.Fatal("unlinking changed the progress")
	}

	rt.agent.expect(http.StatusOK, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "ideas"})
	if n := len(h.jobs("roadmap_progress", rt.ws)); n != 3 {
		t.Fatalf("%d progress jobs, a move back to ideas queues none", n)
	}
	rt.agent.expect(http.StatusOK, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "closed"})
	rt.runProgress(t)
	if rt.progress(convs[1]) != "wont_do" {
		t.Fatalf("progress after closed: %v", rt.progress(convs[1]))
	}
	if rt.progress(late) != "done" {
		t.Fatal("an unlinked feedback changed with the idea")
	}

	var stored int
	for _, e := range rt.owner.expect(http.StatusOK, "GET", "/v1/events?after=0&limit=500", nil).body["events"].([]any) {
		if m := e.(map[string]any); m["type"] == "discussion.stage_changed" {
			stored++
			data := m["data"].(map[string]any)
			if data["from"] == nil || data["to"] == nil || data["linked"] == nil || data["discussion"].(map[string]any)["id"] != idea {
				t.Fatalf("stage event %v", m)
			}
		}
	}
	if stored != 5 {
		t.Fatalf("%d stored discussion.stage_changed events, want 5", stored)
	}
}

func TestRoadmapProgressInBatchesAndRevokedCallers(t *testing.T) {
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, true)
	defer api.SetRoadmapBatchSize(2)()
	idea := rt.idea(rt.owner, "Dark mode")
	var convs []string
	for range 5 {
		c := rt.feedback(h)
		rt.owner.expect(http.StatusNoContent, "PUT", "/v1/discussions/"+idea+"/conversations/"+c, nil)
		convs = append(convs, c)
	}
	rt.owner.expect(http.StatusOK, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "in_progress"})
	rt.runProgress(t)
	for _, c := range convs {
		if rt.progress(c) != "in_progress" {
			t.Fatalf("progress %v", rt.progress(c))
		}
	}
	if ch := rt.changes(idea)[0]; ch["progress_set"] != float64(5) || ch["progress_skipped"] != float64(0) {
		t.Fatalf("batched change %v", ch)
	}

	keyRes := rt.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "mover", "scopes": []string{"roadmap:stage", "conversations:read"}})
	mover := h.client()
	mover.bearer = keyRes.str("secret")
	mover.expect(http.StatusOK, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "shipped"})
	rt.owner.expect(http.StatusNoContent, "DELETE", "/v1/api-keys/"+keyRes.body["api_key"].(map[string]any)["id"].(string), nil)
	rt.runProgress(t)
	for _, c := range convs {
		if rt.progress(c) != "in_progress" {
			t.Fatalf("a revoked key's move set %v", rt.progress(c))
		}
	}
	if ch := rt.changes(idea)[0]; ch["finished_at"] == nil || ch["progress_set"] != float64(0) || ch["progress_skipped"] != float64(5) {
		t.Fatalf("revoked move %v", ch)
	}
}

func TestRoadmapLinks(t *testing.T) {
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, true)
	other := newTeam(t, h)
	idea, second := rt.idea(rt.owner, "Dark mode"), rt.idea(rt.owner, "Export to CSV")
	conv := rt.feedback(h)
	page := rt.owner.expect(http.StatusCreated, "POST", "/v1/conversations", map[string]any{"inbox_id": rt.chatInbox, "contact_id": rt.contact, "subject": "Help"}).str("id")

	rt.owner.expectProblem(http.StatusConflict, "not_feedback", "PUT", "/v1/discussions/"+idea+"/conversations/"+page, nil)
	plain := rt.owner.expect(http.StatusCreated, "POST", "/v1/discussions", map[string]any{
		"channel_id": rt.channel, "page_url": rt.origin + "/guide", "title": "A page thread", "body": "x",
	}).str("id")
	rt.owner.expectProblem(http.StatusConflict, "not_an_idea", "PUT", "/v1/discussions/"+plain+"/conversations/"+conv, nil)
	rt.owner.expectProblem(http.StatusConflict, "not_an_idea", "POST", "/v1/discussions/"+plain+"/stage", map[string]any{"stage": "planned"})
	rt.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/discussions/"+plain+"/stage-changes", nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "PUT", "/v1/discussions/"+idea+"/conversations/"+conv, nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "planned"})
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/discussions/"+idea+"/stage-changes", nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/conversations?idea_id="+idea, nil)

	rt.owner.expect(http.StatusNoContent, "PUT", "/v1/discussions/"+idea+"/conversations/"+conv, nil)
	rt.owner.expect(http.StatusNoContent, "PUT", "/v1/discussions/"+second+"/conversations/"+conv, nil)
	if got := items(rt.owner.expect(http.StatusOK, "GET", "/v1/conversations?idea_id="+idea, nil)); len(got) != 0 {
		t.Fatalf("a feedback keeps its old link: %v", got)
	}
	c := rt.owner.expect(http.StatusOK, "GET", "/v1/conversations/"+conv, nil)
	if c.body["idea"].(map[string]any)["id"] != second || c.body["idea"].(map[string]any)["title"] != "Export to CSV" || c.body["idea"].(map[string]any)["stage"] != "ideas" {
		t.Fatalf("idea on the conversation %s", c.raw)
	}
	if n := h.count("SELECT count(*) FROM roadmap_idea_conversations WHERE workspace_id = $1 AND conversation_id = $2", rt.ws, conv); n != 1 {
		t.Fatalf("%d links for one feedback", n)
	}

	other2 := rt.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Other inbox", "slug": "other-inbox"}).body["inbox"].(map[string]any)["id"].(string)
	contact := rt.owner.expect(http.StatusCreated, "POST", "/v1/contacts", map[string]any{"name": "Bo", "emails": []string{"bo@example.com"}}).str("id")
	foreign := rt.owner.expect(http.StatusCreated, "POST", "/v1/conversations", map[string]any{"inbox_id": other2, "contact_id": contact, "subject": "Idea elsewhere", "kind": "feedback"})
	rt.owner.expectProblem(http.StatusNotFound, "not_found", "PUT", "/v1/discussions/"+idea+"/conversations/"+foreign.str("id"), nil)

	rt.owner.expect(http.StatusNoContent, "DELETE", "/v1/inboxes/"+rt.chatInbox+"/members/"+rt.agentID, nil)
	rt.agent.expectProblem(http.StatusNotFound, "not_found", "PUT", "/v1/discussions/"+idea+"/conversations/"+conv, nil)
	rt.agent.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "planned"})
	rt.agent.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/conversations?idea_id="+idea, nil)

	defer api.SetMaxIdeaLinks(2)()
	a, b := rt.feedback(h), rt.feedback(h)
	rt.owner.expect(http.StatusNoContent, "PUT", "/v1/discussions/"+idea+"/conversations/"+a, nil)
	rt.owner.expect(http.StatusNoContent, "PUT", "/v1/discussions/"+idea+"/conversations/"+a, nil)
	rt.owner.expect(http.StatusNoContent, "PUT", "/v1/discussions/"+idea+"/conversations/"+conv, nil)
	rt.owner.expectProblem(http.StatusUnprocessableEntity, "too_many_links", "PUT", "/v1/discussions/"+idea+"/conversations/"+b, nil)
}

func TestIdeaFromFeedbackAndReplyToLinked(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, true)
	cs := rt.session(h, map[string]any{"identity_token": rt.token(h, map[string]any{"sub": "user-1", "name": "Ada"})})
	conv := cs.expect(http.StatusCreated, "POST", "/client/v1/feedback", map[string]any{"category": "idea", "body": "dark mode please"}).body["conversation"].(map[string]any)["id"].(string)
	chat := cs.start("hello")

	rt.owner.expectProblem(http.StatusConflict, "not_feedback", "POST", "/v1/conversations/"+chat+"/discussion", map[string]any{"kind": "idea", "title": "Dark mode", "body": "x"})
	rt.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/conversations/"+conv+"/discussion", map[string]any{"kind": "idea", "title": "Dark mode", "body": "x", "answer": "y"})
	d := rt.owner.expect(http.StatusCreated, "POST", "/v1/conversations/"+conv+"/discussion", map[string]any{"kind": "idea", "title": "Dark mode", "body": "People ask for a dark theme."}).body["discussion"].(map[string]any)
	if d["kind"] != "idea" || d["stage"] != "ideas" || d["state"] != "published" || d["board_id"] != rt.board || d["author"].(map[string]any)["kind"] != "reader" || d["linked_count"] != float64(1) {
		t.Fatalf("idea from feedback %v", d)
	}
	if rt.owner.expect(http.StatusOK, "GET", "/v1/conversations/"+conv, nil).body["idea"].(map[string]any)["id"] != d["id"] {
		t.Fatal("the feedback is not linked to the idea it started")
	}
	idea := d["id"].(string)

	second := rt.feedback(h)
	rt.owner.expect(http.StatusNoContent, "PUT", "/v1/discussions/"+idea+"/conversations/"+second, nil)
	closed := rt.feedback(h)
	rt.owner.expect(http.StatusNoContent, "PUT", "/v1/discussions/"+idea+"/conversations/"+closed, nil)
	rt.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+closed, map[string]any{"status": "closed"})

	keyNoScope := rt.scopedKey("conversations:read", "messages:write")
	keyNoScope.expect(http.StatusForbidden, "POST", "/v1/discussions/"+idea+"/linked-replies", map[string]any{"body": "Shipped!"})
	rt.owner.expect(http.StatusOK, "PATCH", "/v1/workspace", map[string]any{"bots_may_send": true})
	bot := rt.scopedKey("roadmap:write", "roadmap:read", "messages:write", "conversations:read")
	other := newTeam(t, h)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/discussions/"+idea+"/linked-replies", map[string]any{"body": "Shipped!"})

	r := bot.expect(http.StatusAccepted, "POST", "/v1/discussions/"+idea+"/linked-replies", map[string]any{"body": "Dark mode shipped!"})
	if r.body["queued"] != float64(2) {
		t.Fatalf("queued %s", r.raw)
	}
	jobs := h.jobs("linked_reply", rt.ws)
	if len(jobs) != 2 {
		t.Fatalf("%d linked_reply jobs", len(jobs))
	}
	for _, j := range jobs {
		var a api.LinkedReplyArgs
		if err := json.Unmarshal(j.args, &a); err != nil {
			t.Fatal(err)
		}
		if err := h.srv.RunLinkedReply(context.Background(), a); err != nil {
			t.Fatal(err)
		}
	}
	rep := bot.expect(http.StatusOK, "GET", "/v1/discussions/"+idea+"/linked-replies/"+r.str("id"), nil)
	counts := rep.body["counts"].(map[string]any)
	if rep.str("idea_id") != idea || rep.body["incident_id"] != nil || counts["sent"] != float64(2) {
		t.Fatalf("report %s", rep.raw)
	}
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/discussions/"+idea+"/linked-replies/"+r.str("id"), nil)
	var sent int
	for _, c := range []string{conv, second} {
		for _, m := range messages(rt.owner, c) {
			if m["kind"] == "message" && m["body"] == "Dark mode shipped!" {
				sent++
			}
		}
	}
	if sent != 2 {
		t.Fatalf("%d replies reached the open conversations", sent)
	}
	for _, m := range messages(rt.owner, closed) {
		if m["body"] == "Dark mode shipped!" {
			t.Fatal("the reply reached a closed conversation")
		}
	}
	if st := rt.owner.expect(http.StatusOK, "GET", "/v1/conversations/"+conv, nil); st.str("status") == "closed" {
		t.Fatal("close was not asked")
	}
}

func TestLegacyStatusReplyJobsStillRun(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	sp := newSupportTeam(t, h)
	conv := sp.visitor(h, sp.chatKey, sp.origin).start("is the API down?")
	inc := sp.owner.expect(http.StatusCreated, "POST", sp.path("/incidents"), map[string]any{"title": "Down", "message": "Looking", "publish": true}).str("id")
	sp.owner.expect(http.StatusNoContent, "PUT", sp.path("/incidents/"+inc+"/conversations/"+conv), nil)
	sp.owner.expect(http.StatusAccepted, "POST", sp.path("/incidents/"+inc+"/reply"), map[string]any{"body": "We are on it."})

	jobs := h.jobs("linked_reply", sp.ws)
	if len(jobs) != 1 || len(h.jobs("status_reply", sp.ws)) != 0 {
		t.Fatalf("%d linked_reply jobs", len(jobs))
	}
	var a api.LinkedReplyArgs
	if err := json.Unmarshal(jobs[0].args, &a); err != nil {
		t.Fatal(err)
	}
	legacy := api.StatusReplyArgs(a)
	if legacy.Kind() != "status_reply" || a.Kind() != "linked_reply" {
		t.Fatalf("job kinds %s %s", legacy.Kind(), a.Kind())
	}
	if err := h.srv.RunStatusReply(context.Background(), legacy); err != nil {
		t.Fatal(err)
	}
	rep := sp.owner.expect(http.StatusOK, "GET", sp.path("/incidents/"+inc+"/replies/"+a.ReplyID.String()), nil)
	if rep.body["counts"].(map[string]any)["sent"] != float64(1) || rep.str("incident_id") != inc || rep.body["idea_id"] != nil {
		t.Fatalf("incident reply report %s", rep.raw)
	}
}

func TestRoadmapRealtime(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, true)
	other := newTeam(t, h)
	owner, agent, outsider := rt.owner.dial(""), rt.agent.dial(""), other.owner.dial("")
	owner.ready()
	agent.ready()
	outsider.ready()
	scoped := h.client()
	scoped.bearer = rt.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "noroadmap", "scopes": []string{"conversations:read"}}).body["secret"].(string)
	blind := scoped.dial("")
	blind.ready()

	idea := rt.idea(rt.owner, "Dark mode")
	rt.owner.expect(http.StatusOK, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "planned"})
	rt.owner.expect(http.StatusOK, "PATCH", "/v1/roadmap-boards/"+rt.board, map[string]any{"name": "Renamed"})

	for _, w := range []*wsClient{owner, agent} {
		got := map[string]bool{}
		w.until(func(m wsMessage) bool {
			got[m.Type] = true
			return got["roadmap.board.updated"] && got["discussion.stage_changed"] && got["discussion.created"]
		})
	}
	other.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Sentinel", "slug": "sentinel"})
	outsider.until(func(m wsMessage) bool {
		if strings.HasPrefix(m.Type, "roadmap.") || m.Type == "discussion.stage_changed" {
			t.Fatalf("another workspace heard %s", m.Type)
		}
		return m.Type == "inbox.created"
	})
	rt.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Sentinel", "slug": "sentinel"})
	blind.until(func(m wsMessage) bool {
		if m.Type == "roadmap.board.updated" {
			t.Fatal("a key without roadmap:read heard the board signal")
		}
		return m.Type == "inbox.created"
	})
}

func TestStageChangeIsAWebhook(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, true)
	hook := rt.owner.expect(http.StatusCreated, "POST", "/v1/webhooks", map[string]any{"url": "https://hooks.example/stage", "events": []string{"discussion.stage_changed"}})
	id := hook.body["endpoint"].(map[string]any)["id"].(string)
	idea := rt.idea(rt.owner, "Dark mode")
	rt.owner.expect(http.StatusOK, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "shipped"})
	h.runWebhooks(rt.ws)
	got := items(rt.owner.expect(http.StatusOK, "GET", "/v1/webhooks/"+id+"/deliveries", nil))
	if len(got) != 1 || got[0]["event_type"] != "discussion.stage_changed" {
		t.Fatalf("deliveries %v", got)
	}
}

func TestBoardsPinChannelsAndInboxes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, false)
	rt.owner.expectProblem(http.StatusConflict, "channel_has_roadmap_board", "DELETE", "/v1/channels/"+rt.channel, nil)
	rt.idea(rt.owner, "Dark mode")

	conv := rt.feedback(h)
	d := rt.owner.expect(http.StatusCreated, "POST", "/v1/conversations/"+conv+"/discussion", map[string]any{"kind": "idea", "title": "Export", "body": "x"}).body["discussion"].(map[string]any)
	if n := h.count("SELECT count(*) FROM discussion_follows WHERE discussion_id = $1", d["id"]); n != 0 {
		t.Fatalf("%d followers of an idea on a private board", n)
	}
	rt.owner.expect(http.StatusOK, "POST", "/v1/discussions/"+d["id"].(string)+"/stage", map[string]any{"stage": "planned"})
	if n := h.count("SELECT count(*) FROM reader_notifications WHERE workspace_id = $1", rt.ws); n != 0 {
		t.Fatalf("%d reader notifications from a private board", n)
	}

	second := rt.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Second", "slug": "second"}).body["inbox"].(map[string]any)["id"].(string)
	rt.owner.expect(http.StatusOK, "POST", "/v1/conversations/"+conv+"/move", map[string]any{"inbox_id": second})
	if c := rt.owner.expect(http.StatusOK, "GET", "/v1/conversations/"+conv, nil); c.body["idea"] != nil {
		t.Fatalf("a moved feedback keeps its idea: %s", c.raw)
	}
	rt.owner.expect(http.StatusNoContent, "DELETE", "/v1/roadmap-boards/"+rt.board, nil)
	rt.owner.expect(http.StatusNoContent, "DELETE", "/v1/channels/"+rt.channel, nil)
}

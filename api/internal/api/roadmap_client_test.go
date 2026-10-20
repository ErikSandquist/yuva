package api_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/productdevbook/yuva/api/internal/api"
)

func (rt roadmapTeam) public() *client {
	c := rt.h().client()
	c.origin = rt.origin
	return c
}

func (rt roadmapTeam) ideasPath(query string) string {
	return "/client/v1/channels/" + rt.key + "/discussions?kind=idea" + query
}

func (rt roadmapTeam) member(h *harness, sub string) contactSession {
	return rt.session(h, map[string]any{"identity_token": rt.token(h, map[string]any{"sub": sub, "name": "User " + sub})})
}

func TestClientRoadmapSuggestionsAndVotes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, true)
	public := rt.public()

	inbox := public.expect(http.StatusOK, "GET", "/client/v1/channels/"+rt.key, nil)
	rm, _ := inbox.body["roadmap"].(map[string]any)
	if rm == nil || !strings.HasSuffix(rm["url"].(string), "/roadmap/"+rt.slug) || rm["suggestions"] != "moderated" {
		t.Fatalf("client inbox %s", inbox.raw)
	}
	board := public.expect(http.StatusOK, "GET", "/client/v1/channels/"+rt.key+"/roadmap", nil)
	if board.str("name") != "Acme roadmap" || board.header.Get("Cache-Control") != "public, max-age=60" || board.body["stages"].(map[string]any)["shipped"] != float64(0) {
		t.Fatalf("client board %s %v", board.raw, board.header)
	}
	stranger := h.client()
	stranger.origin = "https://elsewhere.example"
	stranger.expectProblem(http.StatusForbidden, "origin_not_allowed", "GET", "/client/v1/channels/"+rt.key+"/roadmap", nil)

	visitor := rt.session(h, map[string]any{})
	visitor.expectProblem(http.StatusForbidden, "sign_in_required", "POST", "/client/v1/discussions", map[string]any{"kind": "idea", "title": "Dark mode", "body": "Please."})

	ada := newReader(h, rt.chatTeam, "Ada Lovelace")
	ada.site.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/client/v1/discussions", map[string]any{"kind": "idea", "page_url": rt.origin + "/x", "title": "Dark mode", "body": "Please."})
	th := ada.site.expect(http.StatusCreated, "POST", "/client/v1/discussions", map[string]any{"kind": "idea", "title": "Dark\nmode", "body": "<script>x</script>Please, **really**."})
	if th.str("state") != "pending" || th.str("kind") != "idea" || th.str("stage") != "ideas" || th.str("title") != "Dark mode" || th.body["mine"] != true || strings.Contains(th.str("body_html"), "<script") || th.str("page") != "" {
		t.Fatalf("a reader's suggestion %s", th.raw)
	}
	idea, opening := th.str("id"), th.str("opening_post_id")
	if got := items(public.expect(http.StatusOK, "GET", rt.ideasPath(""), nil)); len(got) != 0 {
		t.Fatalf("a pending suggestion is public: %v", got)
	}
	if got := items(ada.site.expect(http.StatusOK, "GET", rt.ideasPath(""), nil)); len(got) != 1 {
		t.Fatalf("its author sees it: %v", got)
	}
	public.expectProblem(http.StatusNotFound, "not_found", "GET", "/client/v1/channels/"+rt.key+"/discussions/"+idea, nil)
	queue := items(rt.agent.expect(http.StatusOK, "GET", "/v1/discussion-posts?state=pending", nil))
	if len(queue) != 1 || queue[0]["discussion"].(map[string]any)["kind"] != "idea" {
		t.Fatalf("moderation queue %v", queue)
	}
	if b := rt.owner.expect(http.StatusOK, "GET", "/v1/roadmap-boards/"+rt.board, nil); b.body["counts"].(map[string]any)["pending"] != float64(1) {
		t.Fatalf("board counts %s", b.raw)
	}
	rt.owner.expect(http.StatusOK, "PATCH", "/v1/discussion-posts/"+opening, map[string]any{"state": "published"})
	if got := items(public.expect(http.StatusOK, "GET", rt.ideasPath(""), nil)); len(got) != 1 || got[0]["stage"] != "ideas" {
		t.Fatalf("the approved idea %v", got)
	}

	bo := rt.member(h, "user-2")
	vote := bo.expect(http.StatusOK, "PUT", "/client/v1/discussion-posts/"+opening+"/vote", nil)
	if vote.body["votes"] != float64(1) || vote.body["voted"] != true {
		t.Fatalf("vote %s", vote.raw)
	}
	bo.expect(http.StatusOK, "PUT", "/client/v1/discussion-posts/"+opening+"/vote", nil)
	if h.count("SELECT vote_count FROM discussions WHERE id = $1", idea) != 1 {
		t.Fatal("the vote count on the idea is not kept")
	}
	ada.site.expectProblem(http.StatusForbidden, "forbidden", "PUT", "/client/v1/discussion-posts/"+opening+"/vote", nil)
	if n := h.count("SELECT count(*) FROM discussion_follows WHERE discussion_id = $1", idea); n != 2 {
		t.Fatalf("%d followers, voting follows the idea", n)
	}
	rt.owner.expect(http.StatusOK, "PUT", "/v1/discussion-posts/"+opening+"/vote", nil)
	if h.count("SELECT vote_count FROM discussions WHERE id = $1", idea) != 2 {
		t.Fatal("a member's vote is not counted")
	}
	bo.expect(http.StatusOK, "DELETE", "/client/v1/discussion-posts/"+opening+"/vote", nil)
	if h.count("SELECT vote_count FROM discussions WHERE id = $1", idea) != 1 {
		t.Fatal("taking a vote back is not counted")
	}
	bo.expect(http.StatusOK, "PUT", "/client/v1/discussion-posts/"+opening+"/vote", nil)

	second := bo.expect(http.StatusCreated, "POST", "/client/v1/discussions", map[string]any{"kind": "idea", "title": "Export to CSV", "body": "x"})
	if second.str("state") != "published" {
		t.Fatalf("an identity-token user publishes at once: %s", second.raw)
	}
	top := items(public.expect(http.StatusOK, "GET", rt.ideasPath("&sort=top"), nil))
	if len(top) != 2 || top[0]["id"] != idea || top[0]["votes"] != float64(2) {
		t.Fatalf("sorted by votes %v", top)
	}
	if found := items(public.expect(http.StatusOK, "GET", rt.ideasPath("&q=dar"), nil)); len(found) != 1 || found[0]["id"] != idea {
		t.Fatalf("title search %v", found)
	}
	if found := items(public.expect(http.StatusOK, "GET", rt.ideasPath("&q="+url.QueryEscape("export csv")), nil)); len(found) != 1 {
		t.Fatalf("title search with two words %v", found)
	}
	public.expect(http.StatusOK, "GET", rt.ideasPath("&q="+url.QueryEscape("dark & ( | !")), nil)
	public.expectProblem(http.StatusBadRequest, "query_empty", "GET", rt.ideasPath("&q="+url.QueryEscape("&|!")), nil)
	public.expectProblem(http.StatusBadRequest, "validation_failed", "GET", rt.ideasPath("&sort=unanswered"), nil)
	if only := items(public.expect(http.StatusOK, "GET", rt.ideasPath("&stage=planned"), nil)); len(only) != 0 {
		t.Fatalf("stage filter %v", only)
	}
	teamIdeas := items(rt.agent.expect(http.StatusOK, "GET", "/v1/discussions?kind=idea&sort=votes&board_id="+rt.board, nil))
	if len(teamIdeas) != 2 || teamIdeas[0]["id"] != idea || teamIdeas[0]["linked_count"] != float64(0) {
		t.Fatalf("team idea list %v", teamIdeas)
	}

	h.clock.Advance(time.Minute)
	rt.owner.expect(http.StatusOK, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "planned", "comment": "Scheduled."})
	h.clock.Advance(time.Minute)
	rt.owner.expect(http.StatusOK, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "shipped"})
	if only := items(public.expect(http.StatusOK, "GET", rt.ideasPath("&stage=shipped"), nil)); len(only) != 1 || only[0]["stage"] != "shipped" {
		t.Fatalf("shipped ideas %v", only)
	}
	cb := public.expect(http.StatusOK, "GET", "/client/v1/channels/"+rt.key+"/roadmap", nil)
	if st := cb.body["stages"].(map[string]any); st["shipped"] != float64(1) || st["ideas"] != float64(1) {
		t.Fatalf("stage counts %s", cb.raw)
	}
	charlie := rt.member(h, "user-3")
	charlie.expectProblem(http.StatusConflict, "idea_closed", "PUT", "/client/v1/discussion-posts/"+opening+"/vote", nil)
	rt.owner.expectProblem(http.StatusConflict, "idea_closed", "PUT", "/v1/discussion-posts/"+opening+"/vote", map[string]any(nil))
	charlie.expect(http.StatusCreated, "POST", "/client/v1/discussions/"+idea+"/posts", map[string]any{"body": "Great, thanks!"})
	charlie.expectProblem(http.StatusConflict, "not_supported", "PUT", "/client/v1/discussions/"+idea+"/answer", map[string]any{"post_id": opening})
	rt.owner.expectProblem(http.StatusConflict, "not_supported", "PUT", "/v1/discussions/"+idea+"/answer", map[string]any{"post_id": opening})

	notes := ada.expect(http.StatusOK, "GET", "/v1/account/notifications", nil)
	var staged int
	for _, n := range items(notes) {
		if n["kind"] == "stage" {
			staged++
		}
	}
	if staged != 2 {
		t.Fatalf("%d stage notifications: %s", staged, notes.raw)
	}
	contact := queue[0]["post"].(map[string]any)["author"].(map[string]any)["contact_id"].(string)
	if _, err := h.srv.SendDiscussionDigest(context.Background(), digestArgs(t, rt.chatTeam, idea, contact)); err != nil {
		t.Fatal(err)
	}
	sent := h.mail.to(ada.email)
	digest := sent[len(sent)-1]
	if !strings.Contains(digest.Text, "Shipped") || !strings.Contains(digest.Text, "/roadmap/"+rt.slug+"/ideas/"+idea) || !strings.Contains(digest.Text, "Great, thanks!") || !strings.Contains(digest.Text, "roadmap") {
		t.Fatalf("digest %+v", digest)
	}

	rt.owner.expect(http.StatusOK, "POST", "/v1/discussions/"+idea+"/stage", map[string]any{"stage": "closed"})
	if got := items(public.expect(http.StatusOK, "GET", rt.ideasPath(""), nil)); len(got) != 1 || got[0]["id"] == idea {
		t.Fatalf("a closed idea is in the public list: %v", got)
	}
	if d := public.expect(http.StatusOK, "GET", "/client/v1/channels/"+rt.key+"/discussions/"+idea, nil); d.body["discussion"].(map[string]any)["stage"] != "closed" {
		t.Fatalf("a closed idea answers with its stage: %s", d.raw)
	}
}

func TestClientRoadmapModesAndLimits(t *testing.T) {
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, true)
	defer api.SetMaxPendingIdeas(2)()

	rt.owner.expect(http.StatusOK, "PATCH", "/v1/roadmap-boards/"+rt.board, map[string]any{"suggestions": "off"})
	ada := newReader(h, rt.chatTeam, "Ada")
	ada.site.expectProblem(http.StatusForbidden, "suggestions_off", "POST", "/client/v1/discussions", map[string]any{"kind": "idea", "title": "Dark mode", "body": "x"})
	if n := h.count("SELECT count(*) FROM discussions WHERE workspace_id = $1", rt.ws); n != 0 {
		t.Fatalf("%d discussions after a refused suggestion", n)
	}

	rt.owner.expect(http.StatusOK, "PATCH", "/v1/roadmap-boards/"+rt.board, map[string]any{"suggestions": "moderated"})
	mine := rt.idea(rt.owner, "Dark mode")
	rt.owner.expect(http.StatusOK, "PATCH", "/v1/roadmap-boards/"+rt.board, map[string]any{"suggestions": "off"})
	if r := ada.site.expect(http.StatusCreated, "POST", "/client/v1/discussions/"+mine+"/posts", map[string]any{"body": "I agree"}); r.str("state") != "pending" {
		t.Fatalf("a reply on a board without suggestions %s", r.raw)
	}
	rt.owner.expect(http.StatusOK, "PATCH", "/v1/roadmap-boards/"+rt.board, map[string]any{"suggestions": "moderated"})

	for _, title := range []string{"Idea number one", "Idea number two"} {
		newReader(h, rt.chatTeam, "").site.expect(http.StatusCreated, "POST", "/client/v1/discussions", map[string]any{"kind": "idea", "title": title, "body": "x"})
	}
	full := newReader(h, rt.chatTeam, "")
	r := full.site.expect(http.StatusTooManyRequests, "POST", "/client/v1/discussions", map[string]any{"kind": "idea", "title": "Idea number three", "body": "x"})
	if r.str("code") != "suggestions_full" || r.header.Get("Retry-After") == "" {
		t.Fatalf("pending cap %s %v", r.raw, r.header)
	}
	rt.owner.expect(http.StatusOK, "PATCH", "/v1/roadmap-boards/"+rt.board, map[string]any{"suggestions": "open"})
	if r := full.site.expect(http.StatusCreated, "POST", "/client/v1/discussions", map[string]any{"kind": "idea", "title": "Idea number three", "body": "x"}); r.str("state") != "published" {
		t.Fatalf("open board %s", r.raw)
	}
}

func TestClientRoadmapPrivateAndOff(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, false)
	public := rt.public()
	idea := rt.idea(rt.owner, "Dark mode")

	if got := public.expect(http.StatusOK, "GET", "/client/v1/channels/"+rt.key, nil); got.body["roadmap"] != nil {
		t.Fatalf("a private board is in the client inbox: %s", got.raw)
	}
	public.expectProblem(http.StatusNotFound, "roadmap_off", "GET", "/client/v1/channels/"+rt.key+"/roadmap", nil)
	public.expectProblem(http.StatusNotFound, "roadmap_off", "GET", rt.ideasPath(""), nil)
	public.expectProblem(http.StatusNotFound, "roadmap_off", "GET", "/client/v1/channels/"+rt.key+"/discussions/"+idea, nil)

	rt.owner.expect(http.StatusOK, "PATCH", "/v1/channels/"+rt.channel, map[string]any{"chat": map[string]any{"allowed_origins": []string{rt.origin}, "discussions": "off"}})
	ada := h.client()
	email := unique("reader") + "@example.com"
	ada.expect(http.StatusAccepted, "POST", "/v1/auth/code", map[string]any{"email": email, "reader": true})
	ada.expect(http.StatusOK, "POST", "/v1/auth/code/verify", map[string]any{"email": email, "code": h.mail.code(t, email)})
	ada.expectProblem(http.StatusForbidden, "discussions_off", "POST", "/v1/account/reader-tokens", map[string]any{"channel_key": rt.key, "origin": rt.origin})
	rt.owner.expect(http.StatusOK, "PATCH", "/v1/roadmap-boards/"+rt.board, map[string]any{"public": true})
	tok := ada.expect(http.StatusCreated, "POST", "/v1/account/reader-tokens", map[string]any{"channel_key": rt.key, "origin": rt.origin})
	site := h.client()
	site.origin, site.bearer = rt.origin, tok.str("token")
	site.expect(http.StatusCreated, "POST", "/client/v1/discussions", map[string]any{"kind": "idea", "title": "Export to CSV", "body": "x"})
	site.expectProblem(http.StatusForbidden, "discussions_off", "POST", "/client/v1/discussions", map[string]any{"page_url": rt.origin + "/guide", "title": "A page thread", "body": "x"})
	ada.expect(http.StatusCreated, "POST", "/v1/account/reader-tokens", map[string]any{"channel_key": rt.key, "origin": testOrigin})

	rt.owner.expect(http.StatusOK, "PATCH", "/v1/roadmap-boards/"+rt.board, map[string]any{"public": false})
	site.expectProblem(http.StatusNotFound, "roadmap_off", "POST", "/client/v1/discussions", map[string]any{"kind": "idea", "title": "Export to CSV", "body": "x"})
}

func TestRoadmapOffAndIdeaLimit(t *testing.T) {
	t.Parallel()
	off := newHarnessWith(t, func(d *api.Deps) { d.Roadmap = api.RoadmapSettings{Off: true} })
	ct := newChatTeam(t, off, "live", true)
	ct.owner.expectProblem(http.StatusNotFound, "roadmap_off", "GET", "/v1/roadmap-boards", nil)
	ct.owner.expectProblem(http.StatusNotFound, "roadmap_off", "POST", "/v1/roadmap-boards", map[string]any{"inbox_id": ct.chatInbox, "channel_id": ct.channel, "name": "x", "slug": unique("rm")})
	ct.owner.expectProblem(http.StatusNotFound, "roadmap_off", "GET", "/v1/discussions?kind=idea", nil)
	pub := off.client()
	pub.origin = ct.origin
	pub.expectProblem(http.StatusNotFound, "roadmap_off", "GET", "/client/v1/channels/"+ct.key+"/roadmap", nil)
	if got := pub.expect(http.StatusOK, "GET", "/client/v1/channels/"+ct.key, nil); got.body["roadmap"] != nil {
		t.Fatalf("roadmap in the inbox of a server with the roadmap off: %s", got.raw)
	}

	limited := newHarnessWith(t, func(d *api.Deps) { d.Roadmap = api.RoadmapSettings{MaxIdeas: 1} })
	rt := newRoadmapTeam(t, limited, true)
	rt.idea(rt.owner, "Dark mode")
	r := rt.owner.expect(http.StatusUnprocessableEntity, "POST", "/v1/discussions", map[string]any{"kind": "idea", "board_id": rt.board, "title": "Export to CSV", "body": "x"})
	if r.str("code") != "hosted_limit" || r.str("limit") != "YUVA_ROADMAP_MAX_IDEAS" {
		t.Fatalf("idea limit %s", r.raw)
	}
}

func TestIdeasAreNotPageThreads(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, true)
	page := rt.origin + "/guide/install"
	thread := rt.owner.expect(http.StatusCreated, "POST", "/v1/discussions", map[string]any{"channel_id": rt.channel, "page_url": page, "title": "A page thread", "body": "x"})
	rt.idea(rt.owner, "Dark mode")
	ada := newReader(h, rt.chatTeam, "Ada")
	ada.site.expect(http.StatusCreated, "POST", "/client/v1/discussions", map[string]any{"kind": "idea", "title": "Export to CSV", "body": "x"})

	if got := items(rt.owner.expect(http.StatusOK, "GET", "/v1/discussions", nil)); len(got) != 1 || got[0]["id"] != thread.str("id") || got[0]["kind"] != "page" {
		t.Fatalf("the default list holds ideas: %v", got)
	}
	if got := items(rt.owner.expect(http.StatusOK, "GET", "/v1/discussions?kind=page", nil)); len(got) != 1 {
		t.Fatalf("kind=page %v", got)
	}
	if got := items(rt.owner.expect(http.StatusOK, "GET", "/v1/discussions?kind=idea", nil)); len(got) != 2 {
		t.Fatalf("kind=idea %v", got)
	}
	if got := items(rt.owner.expect(http.StatusOK, "GET", "/v1/discussions?kind=idea&state=pending", nil)); len(got) != 1 {
		t.Fatalf("pending ideas %v", got)
	}
	if got := items(rt.owner.expect(http.StatusOK, "GET", "/v1/discussions?pending=true", nil)); len(got) != 0 {
		t.Fatalf("pending=true lists an idea among page threads: %v", got)
	}
	rt.owner.expectProblem(http.StatusBadRequest, "validation_failed", "GET", "/v1/discussions?stage=planned", nil)
	rt.owner.expectProblem(http.StatusBadRequest, "validation_failed", "GET", "/v1/discussions?kind=idea&page="+url.QueryEscape(page), nil)
	rt.owner.expectProblem(http.StatusBadRequest, "validation_failed", "GET", "/v1/discussions?kind=poll", nil)

	list := "/client/v1/channels/" + rt.key + "/discussions?page=" + url.QueryEscape(page)
	if got := items(rt.public().expect(http.StatusOK, "GET", list, nil)); len(got) != 1 || got[0]["kind"] != "page" {
		t.Fatalf("client page list %v", got)
	}
	rt.public().expectProblem(http.StatusBadRequest, "validation_failed", "GET", "/client/v1/channels/"+rt.key+"/discussions", nil)

	pages := items(rt.owner.expect(http.StatusOK, "GET", "/v1/docs/pages", nil))
	if len(pages) != 1 || pages[0]["page"] != page || pages[0]["open_threads"] != float64(1) {
		t.Fatalf("docs pages count an idea: %v", pages)
	}
	sum := rt.owner.expect(http.StatusOK, "GET", "/v1/docs/summary", nil)
	if tot := sum.body["totals"].(map[string]any); tot["pending_posts"] != float64(0) || tot["threads"] != float64(1) {
		t.Fatalf("docs summary counts a pending idea: %s", sum.raw)
	}
	if sites := items(rt.owner.expect(http.StatusOK, "GET", "/v1/docs/sites", nil)); len(sites) > 1 {
		t.Fatalf("docs sites %v", sites)
	}
	if got := items(ada.expect(http.StatusOK, "GET", "/v1/account/discussions", nil)); len(got) != 0 {
		t.Fatalf("the reader account lists ideas as page threads: %v", got)
	}
}

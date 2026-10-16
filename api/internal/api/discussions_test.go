package api_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"uuid"

	"github.com/productdevbook/yuva/api/internal/api"
)

type reader struct {
	*client
	email string
	token string
	site  *client
}

// newReader signs a person up on the account page and connects them to a chat channel's site.
func newReader(h *harness, ct chatTeam, name string) reader {
	h.t.Helper()
	email := unique("reader") + "@example.com"
	c := h.client()
	c.expect(http.StatusAccepted, "POST", "/v1/auth/code", map[string]any{"email": email, "reader": true})
	c.expect(http.StatusOK, "POST", "/v1/auth/code/verify", map[string]any{"email": email, "code": h.mail.code(h.t, email)})
	if name != "" {
		c.expect(http.StatusOK, "PATCH", "/v1/account", map[string]any{"name": name})
	}
	tok := c.expect(http.StatusCreated, "POST", "/v1/account/reader-tokens", map[string]any{"channel_key": ct.key, "origin": ct.origin})
	site := h.client()
	site.origin, site.bearer = ct.origin, tok.str("token")
	return reader{client: c, email: email, token: tok.str("token"), site: site}
}

func items(r response) []map[string]any {
	var out []map[string]any
	for _, v := range r.body["items"].([]any) {
		out = append(out, v.(map[string]any))
	}
	return out
}

func TestDiscussionAccessAndModeration(t *testing.T) {
	h := newHarness(t)
	ct := newChatTeam(t, h, "live", true)
	other := newChatTeam(t, h, "live", true)
	page := ct.origin + "/guide/install"
	list := "/client/v1/channels/" + ct.key + "/discussions?page=" + url.QueryEscape(page)
	public := h.client()
	public.origin = ct.origin

	stranger := h.client()
	email := unique("reader") + "@example.com"
	stranger.expect(http.StatusAccepted, "POST", "/v1/auth/code", map[string]any{"email": email})
	if len(h.mail.to(email)) != 0 {
		t.Fatal("a sign-in code went to an unknown address without reader: true")
	}

	ada := newReader(h, ct, "Ada Lovelace")
	ada.expectProblem(http.StatusForbidden, "origin_not_allowed", "POST", "/v1/account/reader-tokens", map[string]any{"channel_key": ct.key, "origin": other.origin})
	ada.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/account/reader-tokens", map[string]any{"channel_key": "yuva_pk_nope", "origin": ct.origin})
	if acc := ada.expect(http.StatusOK, "GET", "/v1/account", nil); acc.str("initials") != "AL" || acc.body["member"] != false || acc.body["email_notifications"] != true {
		t.Fatalf("account %s", acc.raw)
	}
	foreign := h.client()
	foreign.origin, foreign.bearer = other.origin, ada.token
	foreign.expectProblem(http.StatusForbidden, "origin_not_allowed", "POST", "/client/v1/discussions", map[string]any{"page_url": page, "title": "Hello there", "body": "x"})
	wrong := h.client()
	wrong.origin, wrong.bearer = other.origin, ada.token
	wrong.expectProblem(http.StatusForbidden, "origin_not_allowed", "GET", "/client/v1/channels/"+other.key+"/discussions?page="+url.QueryEscape(other.origin+"/x"), nil)
	docsKey := ct.owner.expect(http.StatusCreated, "POST", "/v1/inboxes/"+ct.chatInbox+"/channels", map[string]any{
		"kind": "chat", "name": "Docs", "chat": map[string]any{"allowed_origins": []string{ct.origin}},
	}).body["chat"].(map[string]any)["public_key"].(string)
	ada.site.expectProblem(http.StatusUnauthorized, "unauthenticated", "GET", "/client/v1/channels/"+docsKey+"/discussions?page="+url.QueryEscape(page), nil)

	visitor := ct.session(h, map[string]any{})
	visitor.expectProblem(http.StatusForbidden, "sign_in_required", "POST", "/client/v1/discussions", map[string]any{"page_url": page, "title": "Hello there", "body": "x"})

	th := ada.site.expect(http.StatusCreated, "POST", "/client/v1/discussions", map[string]any{"page_url": page, "title": "Does it\nrun on ARM?", "body": "<script>x</script> **Pi**"})
	if th.str("state") != "pending" || th.body["mine"] != true || th.str("title") != "Does it run on ARM?" || strings.Contains(th.str("body_html"), "<script") {
		t.Fatalf("reader's first thread %s", th.raw)
	}
	id, opening := th.str("id"), th.str("opening_post_id")
	if got := items(public.expect(http.StatusOK, "GET", list, nil)); len(got) != 0 {
		t.Fatalf("a pending thread is public: %v", got)
	}
	public.expectProblem(http.StatusNotFound, "not_found", "GET", "/client/v1/channels/"+ct.key+"/discussions/"+id, nil)
	if r := ada.site.expect(http.StatusOK, "GET", list, nil); len(items(r)) != 1 || r.header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("author's own list %s %v", r.raw, r.header)
	}
	bob := newReader(h, ct, "Bob")
	if got := items(bob.site.expect(http.StatusOK, "GET", list, nil)); len(got) != 0 {
		t.Fatalf("another reader sees a pending thread: %v", got)
	}
	bob.site.expectProblem(http.StatusNotFound, "not_found", "POST", "/client/v1/discussions/"+id+"/posts", map[string]any{"body": "me too"})

	queue := items(ct.agent.expect(http.StatusOK, "GET", "/v1/discussion-posts?state=pending", nil))
	if len(queue) != 1 || queue[0]["post"].(map[string]any)["opening"] != true {
		t.Fatalf("moderation queue %v", queue)
	}
	contact := queue[0]["post"].(map[string]any)["author"].(map[string]any)["contact_id"].(string)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/discussions/"+id, nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "PATCH", "/v1/discussion-posts/"+opening, map[string]any{"state": "published"})
	if got := items(other.owner.expect(http.StatusOK, "GET", "/v1/discussion-posts", nil)); len(got) != 0 {
		t.Fatalf("another workspace's queue %v", got)
	}
	if c := ct.owner.expect(http.StatusOK, "GET", "/v1/contacts/"+contact, nil); c.body["emails"].([]any)[0] != ada.email {
		t.Fatalf("reader contact %s", c.raw)
	}

	ct.agent.expectProblem(http.StatusForbidden, "forbidden", "PATCH", "/v1/discussion-posts/"+opening, map[string]any{"state": "published"})
	ct.agent.expectProblem(http.StatusForbidden, "forbidden", "PATCH", "/v1/discussion-posts/"+opening, map[string]any{"state": "published", "body": "x"})
	if got := items(ct.agent.expect(http.StatusOK, "GET", "/v1/discussion-posts?state=pending", nil)); len(got) != 1 {
		t.Fatalf("an agent's refused approval changed the queue: %v", got)
	}
	ct.owner.expect(http.StatusOK, "PATCH", "/v1/discussion-posts/"+opening, map[string]any{"state": "published"})
	r := public.expect(http.StatusOK, "GET", list, nil)
	if got := items(r); len(got) != 1 || r.header.Get("Cache-Control") != "public, max-age=60" || got[0]["author"].(map[string]any)["name"] != "Ada Lovelace" {
		t.Fatalf("approved thread %s %v", r.raw, r.header)
	}
	for _, k := range []string{"contact_id", "member_id"} {
		if strings.Contains(string(r.raw), k) {
			t.Fatalf("public list exposes %s: %s", k, r.raw)
		}
	}
	second := ada.site.expect(http.StatusCreated, "POST", "/client/v1/discussions/"+id+"/posts", map[string]any{"body": "More detail."})
	if second.str("state") != "published" {
		t.Fatalf("a trusted reader's post waits: %s", second.raw)
	}
	bobPost := bob.site.expect(http.StatusCreated, "POST", "/client/v1/discussions/"+id+"/posts", map[string]any{"body": "Spam spam"})
	if bobPost.str("state") != "pending" {
		t.Fatalf("a new reader's reply %s", bobPost.raw)
	}
	if got := public.expect(http.StatusOK, "GET", "/client/v1/channels/"+ct.key+"/discussions/"+id, nil).body["posts"].([]any); len(got) != 1 {
		t.Fatalf("public sees a pending reply: %v", got)
	}
	if got := ada.site.expect(http.StatusOK, "GET", "/client/v1/channels/"+ct.key+"/discussions/"+id, nil).body["posts"].([]any); len(got) != 1 {
		t.Fatalf("another reader sees a pending reply: %v", got)
	}
	if got := bob.site.expect(http.StatusOK, "GET", "/client/v1/channels/"+ct.key+"/discussions/"+id, nil).body["posts"].([]any); len(got) != 2 {
		t.Fatalf("the author does not see their pending reply: %v", got)
	}
	ct.agent.expectProblem(http.StatusForbidden, "forbidden", "POST", "/v1/discussion-posts/"+bobPost.str("id")+"/spam", nil)
	ct.agent.expectProblem(http.StatusForbidden, "forbidden", "PATCH", "/v1/discussion-posts/"+bobPost.str("id"), map[string]any{"state": "hidden"})
	if got := items(ct.owner.expect(http.StatusOK, "GET", "/v1/discussion-posts?state=pending", nil)); len(got) != 1 || got[0]["post"].(map[string]any)["id"] != bobPost.str("id") {
		t.Fatalf("an agent's refused moderation changed the post: %v", got)
	}
	ct.owner.expect(http.StatusOK, "POST", "/v1/discussion-posts/"+bobPost.str("id")+"/spam", nil)
	again := bob.site.expect(http.StatusCreated, "POST", "/client/v1/discussions/"+id+"/posts", map[string]any{"body": "Spam again"})
	ct.owner.expect(http.StatusOK, "PATCH", "/v1/channels/"+ct.channel, map[string]any{"chat": map[string]any{"allowed_origins": []string{ct.origin}, "allow_anonymous": true, "discussions": "open"}})
	if again.str("state") != "pending" || bob.site.expect(http.StatusCreated, "POST", "/client/v1/discussions/"+id+"/posts", map[string]any{"body": "x"}).str("state") != "pending" {
		t.Fatalf("a spammer's post is published: %s", again.raw)
	}

	h.clock.Advance(time.Second)
	user := ct.session(h, map[string]any{"identity_token": ct.token(h, map[string]any{"sub": "u-" + unique("d"), "name": "Cem"})})
	ans := user.expect(http.StatusCreated, "POST", "/client/v1/discussions/"+id+"/posts", map[string]any{"body": "Yes, arm64 builds exist."})
	if ans.str("state") != "published" {
		t.Fatalf("an identity-token user's reply %s", ans.raw)
	}
	user.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/client/v1/discussions/"+id+"/posts", map[string]any{"body": "x", "parent_post_id": opening})
	nested := user.expect(http.StatusCreated, "POST", "/client/v1/discussions/"+id+"/posts", map[string]any{"body": "thanks", "parent_post_id": ans.str("id")})
	user.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/client/v1/discussions/"+id+"/posts", map[string]any{"body": "x", "parent_post_id": nested.str("id")})

	user.expectProblem(http.StatusForbidden, "forbidden", "PUT", "/client/v1/discussion-posts/"+ans.str("id")+"/vote", nil)
	if v := ada.site.expect(http.StatusOK, "PUT", "/client/v1/discussion-posts/"+ans.str("id")+"/vote", nil); v.body["votes"] != 1.0 || v.body["voted"] != true {
		t.Fatalf("vote %s", v.raw)
	}
	ada.site.expect(http.StatusOK, "PUT", "/client/v1/discussion-posts/"+ans.str("id")+"/vote", nil)
	user.expectProblem(http.StatusForbidden, "forbidden", "PUT", "/client/v1/discussions/"+id+"/answer", map[string]any{"post_id": ans.str("id")})
	ada.site.expectProblem(http.StatusBadRequest, "validation_failed", "PUT", "/client/v1/discussions/"+id+"/answer", map[string]any{"post_id": nested.str("id")})
	if a := ada.site.expect(http.StatusOK, "PUT", "/client/v1/discussions/"+id+"/answer", map[string]any{"post_id": ans.str("id")}); a.str("answer_post_id") != ans.str("id") {
		t.Fatalf("accept %s", a.raw)
	}
	ct.agent.expectProblem(http.StatusForbidden, "forbidden", "PATCH", "/v1/discussion-posts/"+ans.str("id"), map[string]any{"state": "hidden"})
	detail := public.expect(http.StatusOK, "GET", "/client/v1/channels/"+ct.key+"/discussions/"+id, nil)
	if first := detail.body["posts"].([]any)[0].(map[string]any); first["id"] != ans.str("id") || first["accepted"] != true || first["votes"] != 1.0 {
		t.Fatalf("accepted answer first %s", detail.raw)
	}
	ct.owner.expect(http.StatusOK, "PATCH", "/v1/discussion-posts/"+nested.str("id"), map[string]any{"state": "hidden"})
	ct.owner.expect(http.StatusOK, "PATCH", "/v1/discussion-posts/"+nested.str("id"), map[string]any{"state": "published"})

	ct.agent.expect(http.StatusOK, "PATCH", "/v1/discussions/"+id, map[string]any{"locked": true})
	ada.site.expectProblem(http.StatusForbidden, "discussion_locked", "POST", "/client/v1/discussions/"+id+"/posts", map[string]any{"body": "x"})
	ada.site.expectProblem(http.StatusForbidden, "discussion_locked", "PATCH", "/client/v1/discussion-posts/"+second.str("id"), map[string]any{"body": "y"})
	ct.agent.expect(http.StatusCreated, "POST", "/v1/discussions/"+id+"/posts", map[string]any{"body": "Locked; see the release notes."})
	ct.agent.expect(http.StatusOK, "PATCH", "/v1/discussions/"+id, map[string]any{"locked": false})

	h.clock.Advance(31 * time.Minute)
	ada.site.expectProblem(http.StatusForbidden, "edit_window_closed", "PATCH", "/client/v1/discussion-posts/"+second.str("id"), map[string]any{"body": "y"})
	bob.site.expectProblem(http.StatusForbidden, "forbidden", "DELETE", "/client/v1/discussion-posts/"+second.str("id"), nil)

	ct.owner.expect(http.StatusNoContent, "DELETE", "/v1/inboxes/"+ct.chatInbox+"/members/"+ct.agentID, nil)
	ct.agent.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/discussions/"+id, nil)
	ct.agent.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/discussions/"+id+"/posts", map[string]any{"body": "x"})
	if got := items(ct.agent.expect(http.StatusOK, "GET", "/v1/discussion-posts", nil)); len(got) != 0 {
		t.Fatalf("an agent without the inbox sees the queue %v", got)
	}

	notes := ada.expect(http.StatusOK, "GET", "/v1/account/notifications?unread=true", nil)
	if notes.body["unread"].(float64) < 2 {
		t.Fatalf("reader notifications %s", notes.raw)
	}
	if mine := items(ada.expect(http.StatusOK, "GET", "/v1/account/discussions", nil)); len(mine) != 1 || mine[0]["started"] != true || mine[0]["answered"] != true {
		t.Fatalf("my discussions %v", mine)
	}
	if _, err := h.srv.SendDiscussionDigest(context.Background(), digestArgs(t, ct, id, contact)); err != nil {
		t.Fatal(err)
	}
	sent := h.mail.to(ada.email)
	digest := sent[len(sent)-1]
	if !strings.Contains(digest.Text, "Yes, arm64 builds exist.") || digest.Unsubscribe == "" || !strings.Contains(digest.Subject, "answered") {
		t.Fatalf("digest %+v", digest)
	}
	link := strings.TrimPrefix(digest.Unsubscribe, testOrigin)
	public.expect(http.StatusOK, "GET", link, nil)
	if got := items(ada.expect(http.StatusOK, "GET", "/v1/account/following", nil)); len(got) != 1 {
		t.Fatalf("opening the unsubscribe page unfollowed: %v", got)
	}
	public.expect(http.StatusOK, "POST", link, nil)
	if got := items(ada.expect(http.StatusOK, "GET", "/v1/account/following", nil)); len(got) != 0 {
		t.Fatalf("still following after unsubscribe: %v", got)
	}

	ada.expect(http.StatusNoContent, "POST", "/v1/auth/sign-out", nil)
	ada.site.expectProblem(http.StatusUnauthorized, "unauthenticated", "PUT", "/client/v1/discussions/"+id+"/follow", nil)

	ct.owner.expect(http.StatusOK, "PATCH", "/v1/channels/"+ct.channel, map[string]any{"chat": map[string]any{"allowed_origins": []string{ct.origin}, "discussions": "off"}})
	public.expectProblem(http.StatusForbidden, "discussions_off", "GET", list, nil)
	user.expectProblem(http.StatusForbidden, "discussions_off", "POST", "/client/v1/discussions", map[string]any{"page_url": page, "title": "Hello there", "body": "x"})
}

func digestArgs(t *testing.T, ct chatTeam, discussion, contact string) api.DiscussionDigestArgs {
	t.Helper()
	var ids [3]uuid.UUID
	for i, v := range []string{ct.ws, discussion, contact} {
		id, err := uuid.Parse(v)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = id
	}
	return api.DiscussionDigestArgs{WorkspaceID: ids[0], DiscussionID: ids[1], ContactID: ids[2]}
}

func TestReaderAccountDeletion(t *testing.T) {
	h := newHarness(t)
	ct := newChatTeam(t, h, "live", true)
	page := ct.origin + "/faq"
	ada := newReader(h, ct, "Ada")
	th := ada.site.expect(http.StatusCreated, "POST", "/client/v1/discussions", map[string]any{"page_url": page, "title": "A question", "body": "Why?"})
	ct.owner.expect(http.StatusOK, "PATCH", "/v1/discussion-posts/"+th.str("opening_post_id"), map[string]any{"state": "published"})
	ct.owner.expect(http.StatusCreated, "POST", "/v1/discussions/"+th.str("id")+"/posts", map[string]any{"body": "Because."})

	ada.expectProblem(http.StatusBadRequest, "confirmation_mismatch", "DELETE", "/v1/me", map[string]any{"email": "x@example.com"})
	ada.expect(http.StatusNoContent, "DELETE", "/v1/me", map[string]any{"email": ada.email})
	d := ct.owner.expect(http.StatusOK, "GET", "/v1/discussions/"+th.str("id"), nil).body["discussion"].(map[string]any)
	if a := d["author"].(map[string]any); a["deleted"] != true || a["name"] != nil || a["kind"] != "reader" {
		t.Fatalf("deleted reader's thread %v", d)
	}
	if got := items(ct.owner.expect(http.StatusOK, "GET", "/v1/contacts?q="+url.QueryEscape(ada.email), nil)); len(got) != 0 {
		t.Fatalf("the reader's contact stayed: %v", got)
	}
}

func TestDiscussionFromConversation(t *testing.T) {
	h := newHarness(t)
	ct := newChatTeam(t, h, "live", true)
	cs := ct.session(h, map[string]any{"identity_token": ct.token(h, map[string]any{"sub": "u-" + unique("d"), "name": "Cem"})})
	fb := cs.expect(http.StatusCreated, "POST", "/client/v1/feedback", map[string]any{
		"body": "secret order 4711 fails", "page_url": ct.origin + "/install", "page_title": "Install", "client_id": unique("c"),
	}).body["conversation"].(map[string]any)
	conv := "/v1/conversations/" + fb["id"].(string) + "/discussion"
	r := ct.agent.expect(http.StatusCreated, "POST", conv, map[string]any{"title": "Install fails on step 2", "body": "Step 2 fails.", "answer": "Run it as root."})
	d := r.body["discussion"].(map[string]any)
	if d["author"].(map[string]any)["kind"] != "reader" || d["answer_post_id"] == nil || d["conversation_id"] != fb["id"] || d["page"] != ct.origin+"/install" {
		t.Fatalf("converted %s", r.raw)
	}
	if strings.Contains(string(r.raw), "4711") {
		t.Fatalf("private text became public: %s", r.raw)
	}
	ct.agent.expectProblem(http.StatusConflict, "already_discussed", "POST", conv, map[string]any{"title": "Again here", "body": "x"})
	other := newChatTeam(t, h, "live", true)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", conv, map[string]any{"title": "Mine now", "body": "x"})

	visitor := ct.session(h, map[string]any{})
	vc := visitor.expect(http.StatusCreated, "POST", "/client/v1/feedback", map[string]any{
		"body": "it breaks", "page_url": ct.origin + "/install", "client_id": unique("c"),
	}).body["conversation"].(map[string]any)
	vd := ct.agent.expect(http.StatusCreated, "POST", "/v1/conversations/"+vc["id"].(string)+"/discussion", map[string]any{"title": "It breaks", "body": "It breaks."})
	if vd.body["discussion"].(map[string]any)["author"].(map[string]any)["kind"] != "member" {
		t.Fatalf("an anonymous visitor became the author: %s", vd.raw)
	}
}

func TestRefusedReaderWriteLeavesNoContact(t *testing.T) {
	h := newHarness(t)
	ct := newChatTeam(t, h, "live", true)
	page := ct.origin + "/faq"
	user := ct.session(h, map[string]any{"identity_token": ct.token(h, map[string]any{"sub": "u-" + unique("d"), "name": "Cem"})})
	th := user.expect(http.StatusCreated, "POST", "/client/v1/discussions", map[string]any{"page_url": page, "title": "A question", "body": "Why?"})
	ct.owner.expect(http.StatusOK, "PATCH", "/v1/discussions/"+th.str("id"), map[string]any{"locked": true})

	ada := newReader(h, ct, "Ada")
	missing := "00000000-0000-7000-8000-000000000000"
	ada.site.expectProblem(http.StatusNotFound, "not_found", "PUT", "/client/v1/discussion-posts/"+missing+"/vote", nil)
	ada.site.expectProblem(http.StatusNotFound, "not_found", "POST", "/client/v1/discussions/"+missing+"/posts", map[string]any{"body": "x"})
	ada.site.expectProblem(http.StatusNotFound, "not_found", "PUT", "/client/v1/discussions/"+missing+"/follow", nil)
	ada.site.expectProblem(http.StatusForbidden, "discussion_locked", "POST", "/client/v1/discussions/"+th.str("id")+"/posts", map[string]any{"body": "x"})
	ada.site.expectProblem(http.StatusForbidden, "discussion_locked", "PUT", "/client/v1/discussion-posts/"+th.str("opening_post_id")+"/vote", nil)
	ada.site.expectProblem(http.StatusBadRequest, "page_origin", "POST", "/client/v1/discussions", map[string]any{"page_url": "https://elsewhere.example/x", "title": "Hello there", "body": "x"})
	contacts := func() int {
		return len(items(ct.owner.expect(http.StatusOK, "GET", "/v1/contacts?q="+url.QueryEscape(ada.email), nil)))
	}
	if n := contacts(); n != 0 {
		t.Fatalf("refused writes created %d contacts for the reader", n)
	}
	ct.owner.expect(http.StatusOK, "PATCH", "/v1/discussions/"+th.str("id"), map[string]any{"locked": false})
	ada.site.expectProblem(http.StatusForbidden, "forbidden", "PUT", "/client/v1/discussions/"+th.str("id")+"/answer", map[string]any{"post_id": th.str("opening_post_id")})
	if n := contacts(); n != 0 {
		t.Fatalf("a refused answer created %d contacts", n)
	}
	ada.site.expect(http.StatusOK, "PUT", "/client/v1/discussion-posts/"+th.str("opening_post_id")+"/vote", nil)
	if n := contacts(); n != 1 {
		t.Fatalf("an accepted write made %d contacts", n)
	}
}

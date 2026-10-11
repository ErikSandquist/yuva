package api_test

import (
	"net/http"
	"net/url"
	"testing"
)

func TestDocumentationPages(t *testing.T) {
	h := newHarness(t)
	ct := newChatTeam(t, h, "live", true)
	other := newChatTeam(t, h, "live", true)
	page := ct.origin + "/guide/install"
	ratings := "/client/v1/channels/" + ct.key + "/page-ratings"

	site := h.client()
	site.origin = ct.origin
	site.expect(http.StatusNoContent, "POST", ratings, map[string]any{"page": page + "?ref=nav#step-2", "title": "Install", "rating": "up"})
	site.expect(http.StatusNoContent, "POST", ratings, map[string]any{"page": page, "rating": "down"})
	site.expect(http.StatusNoContent, "POST", ratings, map[string]any{"page": page, "rating": "down", "previous": "up"})
	site.expectProblem(http.StatusBadRequest, "page_origin", "POST", ratings, map[string]any{"page": other.origin + "/guide", "rating": "up"})
	site.expectProblem(http.StatusBadRequest, "validation_failed", "POST", ratings, map[string]any{"page": "/guide", "rating": "up"})
	site.expectProblem(http.StatusNotFound, "not_found", "POST", "/client/v1/channels/nope/page-ratings", map[string]any{"page": page, "rating": "up"})
	stranger := h.client()
	stranger.origin = other.origin
	stranger.expectProblem(http.StatusForbidden, "origin_not_allowed", "POST", ratings, map[string]any{"page": page, "rating": "up"})

	cs := ct.session(h, map[string]any{})
	fb := cs.expect(http.StatusCreated, "POST", "/client/v1/feedback", map[string]any{
		"body": "step 2 fails", "page_url": page + "#step-2", "page_title": "Install", "rating": "down",
		"allow_email": true, "email": "success@simulator.amazonses.com", "client_id": unique("c"),
	}).body["conversation"].(map[string]any)
	if f := fb["feedback"].(map[string]any); fb["kind"] != "feedback" || f["page_url"] != page || f["rating"] != "down" || f["category"] != "other" {
		t.Fatalf("page feedback %v", fb)
	}
	cs.expectProblem(http.StatusBadRequest, "page_origin", "POST", "/client/v1/feedback", map[string]any{"body": "x", "page_url": other.origin + "/guide"})

	user := ct.session(h, map[string]any{"identity_token": ct.token(h, map[string]any{"sub": "u-" + unique("d"), "name": "Ada Lovelace"})})
	th := user.expect(http.StatusCreated, "POST", "/client/v1/discussions", map[string]any{
		"page_url": page + "?a=1", "page_title": "Install", "title": "Does it run on ARM?", "body": "Asking for a *Pi*.",
	})
	if th.str("state") != "published" || th.str("page") != page || th.str("body_html") != "<p>Asking for a <em>Pi</em>.</p>\n" {
		t.Fatalf("thread %s", th.raw)
	}
	threads := "/client/v1/channels/" + ct.key + "/discussions?page=" + url.QueryEscape(page+"?utm=x#top")
	stranger.expectProblem(http.StatusForbidden, "origin_not_allowed", "GET", threads, nil)
	site.expectProblem(http.StatusBadRequest, "page_origin", "GET", "/client/v1/channels/"+ct.key+"/discussions?page="+url.QueryEscape(other.origin+"/guide"), nil)

	pages := ct.owner.expect(http.StatusOK, "GET", "/v1/docs/pages?days=7", nil).body["items"].([]any)
	if len(pages) != 1 {
		t.Fatalf("docs pages %v", pages)
	}
	if p := pages[0].(map[string]any); p["page"] != page || p["title"] != "Install" || p["up"] != 0.0 || p["down"] != 2.0 ||
		p["open_feedback"] != 1.0 || p["open_threads"] != 1.0 || p["unanswered_threads"] != 1.0 || p["pending_posts"] != 0.0 {
		t.Fatalf("docs page %v", p)
	}
	ct.owner.expectProblem(http.StatusBadRequest, "validation_failed", "GET", "/v1/docs/pages?days=8", nil)
	if got := ct.agent.expect(http.StatusOK, "GET", "/v1/discussions?page="+url.QueryEscape(page+"#x"), nil).body["items"].([]any); len(got) != 1 {
		t.Fatalf("threads of the page %v", got)
	}
	if got := other.owner.expect(http.StatusOK, "GET", "/v1/docs/pages", nil).body["items"].([]any); len(got) != 0 {
		t.Fatalf("another workspace sees %v", got)
	}
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/discussions/"+th.str("id"), nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/docs/page?inbox_id="+ct.chatInbox+"&page="+url.QueryEscape(page), nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/docs/summary?inbox_id="+ct.chatInbox, nil)
	sum := ct.agent.expect(http.StatusOK, "GET", "/v1/docs/summary?days=7", nil).body
	if tot := sum["totals"].(map[string]any); len(sum["days"].([]any)) != 7 || tot["down"] != 2.0 || tot["feedback"] != 1.0 || tot["threads"] != 1.0 || tot["pending_posts"] != 0.0 || tot["rated_pages"] != 1.0 {
		t.Fatalf("summary %v", sum)
	}
	if tot := other.owner.expect(http.StatusOK, "GET", "/v1/docs/summary", nil).body["totals"].(map[string]any); tot["down"] != 0.0 || tot["feedback"] != 0.0 {
		t.Fatalf("another workspace's summary %v", tot)
	}

	ct.owner.expect(http.StatusNoContent, "DELETE", "/v1/inboxes/"+ct.chatInbox+"/members/"+ct.agentID, nil)
	ct.agent.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/discussions/"+th.str("id"), nil)
	if got := ct.agent.expect(http.StatusOK, "GET", "/v1/docs/pages", nil).body["items"].([]any); len(got) != 0 {
		t.Fatalf("an agent without the inbox sees %v", got)
	}
	if tot := ct.agent.expect(http.StatusOK, "GET", "/v1/docs/summary", nil).body["totals"].(map[string]any); tot["down"] != 0.0 || tot["threads"] != 0.0 {
		t.Fatalf("an agent without the inbox counts %v", tot)
	}
	if got := ct.agent.expect(http.StatusOK, "GET", "/v1/discussions", nil).body["items"].([]any); len(got) != 0 {
		t.Fatalf("an agent without the inbox lists %v", got)
	}
}

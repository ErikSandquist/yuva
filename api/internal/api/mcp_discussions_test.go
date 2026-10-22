package api_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/productdevbook/yuva/api/internal/api"
)

func TestMCPRequestBroadcastApprovalOwnerGrantOnly(t *testing.T) {
	t.Parallel()
	h, bs := newBroadcastHarness(t, func(d *api.Deps) { d.Broadcasts.Mode = "approval" })
	bt := newBroadcastTeam(t, h, bs)
	if _, err := h.st.Pool.Exec(context.Background(), `UPDATE workspaces SET broadcasts_status = 'approval_required' WHERE id = $1`, bt.ws); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"website": "https://acme.example", "sender_description": "Product news", "consent_description": "A form on the site"}
	const tool = "request_broadcast_approval"

	key := h.client()
	key.bearer = apiKey(bt.owner, "approval", "workspace:manage")
	key.expectProblem(http.StatusForbidden, "member_session_required", "POST", "/v1/workspace/broadcast-approval", body)
	keyMCP := h.mcpSession(key.bearer)
	if slices.Contains(toolNames(t, keyMCP), tool) {
		t.Fatalf("an API key lists %s", tool)
	}
	if _, errText := callTool(t, keyMCP, tool, body); errText == "" {
		t.Fatal("an API key asked for approval")
	}

	agentREST := h.client()
	agentREST.bearer = h.oauthToken(bt.agent, bt.ws, "Claude Code", testOrigin, []string{"inboxes:read"})
	agentREST.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", "/v1/workspace/broadcast-approval", body)
	agent := h.mcpSession(h.oauthToken(bt.agent, bt.ws, "Claude Code", testOrigin+"/mcp", []string{"inboxes:read"}))
	if slices.Contains(toolNames(t, agent), tool) {
		t.Fatalf("an agent lists %s", tool)
	}
	if _, errText := callTool(t, agent, tool, body); errText == "" {
		t.Fatal("an agent asked for approval")
	}

	narrow := h.mcpSession(h.oauthToken(bt.owner, bt.ws, "Claude Code", testOrigin+"/mcp", []string{"inboxes:read"}))
	if slices.Contains(toolNames(t, narrow), tool) {
		t.Fatalf("an owner's connection without workspace:manage lists %s", tool)
	}
	if ws := bt.owner.expect(http.StatusOK, "GET", "/v1/workspace", nil).body["broadcasts"].(map[string]any); ws["status"] != "approval_required" {
		t.Fatalf("a refused request changed the status: %v", ws)
	}

	owner := h.mcpSession(h.oauthToken(bt.owner, bt.ws, "Claude Code", testOrigin+"/mcp", []string{"workspace:manage"}))
	if !slices.Contains(toolNames(t, owner), tool) {
		t.Fatalf("the owner's connection lacks %s", tool)
	}
	out, errText := callTool(t, owner, tool, body)
	if errText != "" || out["status"] != "approval_requested" {
		t.Fatalf("%s: %v %s", tool, out, errText)
	}
	if ws := bt.owner.expect(http.StatusOK, "GET", "/v1/workspace", nil).body["broadcasts"].(map[string]any); ws["status"] != "approval_requested" {
		t.Fatalf("status after the request: %v", ws)
	}
	if _, errText := callTool(t, owner, tool, body); !strings.Contains(errText, "invalid_state") {
		t.Fatalf("a second request: %q", errText)
	}
}

func TestMCPDiscussionReplyAndAnswer(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChatTeam(t, h, "live", true)
	other := newChatTeam(t, h, "live", true)
	ada := newReader(h, ct, "Ada Lovelace")
	th := ada.site.expect(http.StatusCreated, "POST", "/client/v1/discussions", map[string]any{"page_url": ct.origin + "/guide/install", "title": "Does it run on ARM?", "body": "On a Pi?"})
	id := th.str("id")
	ct.owner.expect(http.StatusOK, "PATCH", "/v1/discussion-posts/"+th.str("opening_post_id"), map[string]any{"state": "published"})
	scopes := []string{"conversations:read", "messages:write"}
	writeTools := []string{"reply_to_discussion", "mark_discussion_answer"}

	off := h.mcpSession(h.oauthToken(ct.owner, ct.ws, "Claude Code", testOrigin+"/mcp", scopes))
	for _, name := range writeTools {
		if slices.Contains(toolNames(t, off), name) {
			t.Fatalf("%s is listed while bots_may_send is off", name)
		}
	}
	if _, errText := callTool(t, off, "reply_to_discussion", map[string]any{"discussion_id": id, "body": "x"}); !strings.Contains(errText, "bot_sending_disabled") {
		t.Fatalf("reply with bots_may_send off: %q", errText)
	}

	key := h.client()
	key.bearer = apiKey(ct.owner, "posts", "conversations:read", "messages:write")
	key.expectProblem(http.StatusForbidden, "member_session_required", "POST", "/v1/discussions/"+id+"/posts", map[string]any{"body": "x"})
	key.expectProblem(http.StatusForbidden, "member_session_required", "PUT", "/v1/discussions/"+id+"/answer", map[string]any{"post_id": th.str("opening_post_id")})
	key.expectProblem(http.StatusForbidden, "member_session_required", "DELETE", "/v1/discussions/"+id+"/answer", nil)

	ct.owner.expect(http.StatusOK, "PATCH", "/v1/workspace", map[string]any{"bots_may_send": true})
	for _, name := range writeTools {
		if slices.Contains(toolNames(t, h.mcpSession(key.bearer)), name) {
			t.Fatalf("an API key lists %s", name)
		}
	}
	readOnly := h.mcpSession(h.oauthToken(ct.owner, ct.ws, "Claude Code", testOrigin+"/mcp", []string{"conversations:read"}))
	if _, errText := callTool(t, readOnly, "reply_to_discussion", map[string]any{"discussion_id": id, "body": "x"}); !strings.Contains(errText, "messages:write") {
		t.Fatalf("reply without messages:write: %q", errText)
	}

	cs := h.mcpSession(h.oauthToken(ct.owner, ct.ws, "Claude Code", testOrigin+"/mcp", scopes))
	for _, name := range writeTools {
		if !slices.Contains(toolNames(t, cs), name) {
			t.Fatalf("%s is not listed while bots_may_send is on", name)
		}
	}
	reply, errText := callTool(t, cs, "reply_to_discussion", map[string]any{"discussion_id": id, "body": "Yes, arm64 builds exist."})
	if errText != "" || reply["via"] != "Claude Code" || reply["author_kind"] != "member" || reply["state"] != "published" {
		t.Fatalf("reply_to_discussion: %v %s", reply, errText)
	}
	answerID := reply["id"].(string)
	if _, errText := callTool(t, cs, "reply_to_discussion", map[string]any{"discussion_id": id, "body": "x", "parent_post_id": th.str("opening_post_id")}); !strings.Contains(errText, "validation_failed") {
		t.Fatalf("a reply to the opening post: %q", errText)
	}

	agent := h.client()
	agent.bearer = h.oauthToken(ct.agent, ct.ws, "Raycast", testOrigin, scopes)
	nested := agent.expect(http.StatusCreated, "POST", "/v1/discussions/"+id+"/posts", map[string]any{"body": "Thanks!", "parent_post_id": answerID})
	if a := nested.body["author"].(map[string]any); a["via"] != "Raycast" || a["member_id"] != ct.agentID {
		t.Fatalf("a member app's post author: %s", nested.raw)
	}
	posts := ct.owner.expect(http.StatusOK, "GET", "/v1/discussions/"+id, nil).body["posts"].([]any)
	if a := posts[0].(map[string]any)["author"].(map[string]any); posts[0].(map[string]any)["id"] != answerID || a["via"] != "Claude Code" {
		t.Fatalf("via in the thread: %v", posts)
	}
	public := h.client()
	public.origin = ct.origin
	if r := public.expect(http.StatusOK, "GET", "/client/v1/channels/"+ct.key+"/discussions/"+id, nil); strings.Contains(string(r.raw), `"via"`) {
		t.Fatalf("readers see via: %s", r.raw)
	}

	mark := map[string]any{"discussion_id": id, "post_id": answerID}
	for range 2 {
		if d, errText := callTool(t, cs, "mark_discussion_answer", mark); errText != "" || d["answered"] != true {
			t.Fatalf("mark_discussion_answer: %v %s", d, errText)
		}
	}
	if d := ct.owner.expect(http.StatusOK, "GET", "/v1/discussions/"+id, nil).body["discussion"].(map[string]any); d["answer_post_id"] != answerID {
		t.Fatalf("answer after marking: %v", d)
	}
	if _, errText := callTool(t, cs, "mark_discussion_answer", map[string]any{"discussion_id": id}); !strings.Contains(errText, "validation_failed") {
		t.Fatalf("neither post_id nor clear: %q", errText)
	}
	if _, errText := callTool(t, cs, "mark_discussion_answer", map[string]any{"discussion_id": id, "post_id": nested.str("id")}); !strings.Contains(errText, "validation_failed") {
		t.Fatalf("a reply to a reply as the answer: %q", errText)
	}
	for range 2 {
		if d, errText := callTool(t, cs, "mark_discussion_answer", map[string]any{"discussion_id": id, "clear": true}); errText != "" || d["answered"] != false {
			t.Fatalf("clear: %v %s", d, errText)
		}
	}
	if d := agent.expect(http.StatusOK, "PUT", "/v1/discussions/"+id+"/answer", map[string]any{"post_id": answerID}); d.str("answer_post_id") != answerID {
		t.Fatalf("a member app marks the answer: %s", d.raw)
	}
	agent.expect(http.StatusOK, "DELETE", "/v1/discussions/"+id+"/answer", nil)

	stranger := h.client()
	stranger.bearer = h.oauthToken(other.owner, other.ws, "Claude Code", testOrigin, scopes)
	stranger.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/discussions/"+id+"/posts", map[string]any{"body": "x"})
	stranger.expectProblem(http.StatusNotFound, "not_found", "PUT", "/v1/discussions/"+id+"/answer", map[string]any{"post_id": answerID})
}

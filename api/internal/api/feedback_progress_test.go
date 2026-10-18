package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestFeedbackProgress(t *testing.T) {
	h := newHarness(t)
	at := newAppTeam(t, h, false)
	cs := at.session(h, "user-1")
	fb := cs.expect(http.StatusCreated, "POST", "/client/v1/feedback", map[string]any{"category": "idea", "body": "dark mode"}).body["conversation"].(map[string]any)["id"].(string)
	chat := cs.start("where is my order")

	progress := func(body map[string]any) any {
		f, _ := body["feedback"].(map[string]any)
		return f["progress"]
	}
	if r := at.owner.expect(http.StatusOK, "GET", "/v1/conversations/"+fb, nil); progress(r.body) != nil {
		t.Fatalf("new feedback has progress: %s", r.raw)
	}

	ws, _, err := dialContact(h, cs.token, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if m := ws.nextNot("presence", "inbox.updated"); m.Type != "ready" {
		t.Fatalf("first frame %s %s", m.Type, m.Data)
	}

	r := at.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+fb, map[string]any{"feedback_progress": "in_progress"})
	if progress(r.body) != "in_progress" || r.str("status") != "open" {
		t.Fatalf("set in_progress: %s", r.raw)
	}
	m := ws.nextNot("presence", "inbox.updated")
	if m.Type != "conversation.updated" || !strings.Contains(string(m.Data), `"feedback_progress":"in_progress"`) || !strings.Contains(string(m.Data), `"status":"open"`) {
		t.Fatalf("contact frame after in_progress: %s %s", m.Type, m.Data)
	}
	at.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+fb, map[string]any{"feedback_progress": "in_progress"})
	r = at.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+fb, map[string]any{"feedback_progress": "done", "status": "closed"})
	if progress(r.body) != "done" || r.str("status") != "closed" {
		t.Fatalf("set done and close: %s", r.raw)
	}
	m = ws.nextNot("presence", "inbox.updated")
	if m.Type != "conversation.updated" || !strings.Contains(string(m.Data), `"feedback_progress":"done"`) || !strings.Contains(string(m.Data), `"status":"closed"`) {
		t.Fatalf("contact frame after done: %s %s", m.Type, m.Data)
	}

	c := cs.expect(http.StatusOK, "GET", "/client/v1/conversations/"+fb, nil)
	if progress(c.body) != "done" {
		t.Fatalf("client conversation: %s", c.raw)
	}
	for _, it := range cs.expect(http.StatusOK, "GET", "/client/v1/conversations?kind=feedback", nil).body["items"].([]any) {
		if progress(it.(map[string]any)) != "done" {
			t.Fatalf("client list: %v", it)
		}
	}
	cs.expect(http.StatusCreated, "POST", "/client/v1/conversations/"+fb+"/messages", map[string]any{"body": "still broken"})
	if r := at.owner.expect(http.StatusOK, "GET", "/v1/conversations/"+fb, nil); r.str("status") != "open" || progress(r.body) != "done" {
		t.Fatalf("a contact's reply reopens and keeps the progress: %s", r.raw)
	}

	r = at.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+fb, map[string]any{"feedback_progress": nil})
	if _, ok := r.body["feedback"].(map[string]any)["progress"]; ok {
		t.Fatalf("cleared progress still set: %s", r.raw)
	}
	m = ws.nextNot("presence", "inbox.updated", "message.created", "read")
	for m.Type == "conversation.updated" && strings.Contains(string(m.Data), `"feedback_progress"`) {
		m = ws.nextNot("presence", "inbox.updated", "message.created", "read")
	}
	if m.Type != "conversation.updated" || strings.Contains(string(m.Data), "feedback_progress") {
		t.Fatalf("contact frame after clearing: %s %s", m.Type, m.Data)
	}

	var got []string
	for _, msg := range messages(at.owner, fb) {
		ev, _ := msg["event"].(map[string]any)
		if ev["type"] != "feedback_progress" {
			continue
		}
		got = append(got, describeProgressEvent(ev))
		if msg["kind"] != "event" || msg["author"].(map[string]any)["type"] != "member" {
			t.Fatalf("progress event message: %v", msg)
		}
	}
	if strings.Join(got, " ") != "-:in_progress in_progress:done done:null" {
		t.Fatalf("progress events %v", got)
	}
	for _, msg := range cs.expect(http.StatusOK, "GET", "/client/v1/conversations/"+fb+"/messages", nil).body["items"].([]any) {
		if strings.Contains(msg.(map[string]any)["body"].(string), "progress") {
			t.Fatalf("contact sees an event message: %v", msg)
		}
	}

	at.owner.expectProblem(http.StatusConflict, "not_feedback", "PATCH", "/v1/conversations/"+chat, map[string]any{"feedback_progress": "planned"})
	if r := at.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+chat, map[string]any{"feedback_progress": nil}); r.body["feedback"] != nil {
		t.Fatalf("null on a chat: %s", r.raw)
	}
	for _, msg := range messages(at.owner, chat) {
		if msg["kind"] == "event" {
			t.Fatalf("null on a chat recorded %v", msg)
		}
	}
	at.owner.expectProblem(http.StatusBadRequest, "validation_failed", "PATCH", "/v1/conversations/"+fb, map[string]any{"feedback_progress": "shipped"})

	agentID := at.agentID
	at.agent.expectProblem(http.StatusNotFound, "not_found", "PATCH", "/v1/conversations/"+fb, map[string]any{"feedback_progress": "planned"})
	at.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+at.appInbox+"/members/"+agentID, nil)
	if r := at.agent.expect(http.StatusOK, "PATCH", "/v1/conversations/"+fb, map[string]any{"feedback_progress": "planned"}); progress(r.body) != "planned" {
		t.Fatalf("agent with access: %s", r.raw)
	}

	key := at.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{
		"name": "assistant", "scopes": []string{"conversations:read", "conversations:write"},
	}).str("secret")
	mcs := h.mcpSession(key)
	if out, errText := callTool(t, mcs, "set_feedback_progress", map[string]any{"conversation_id": fb, "progress": "wont_do"}); errText != "" || out["feedback_progress"] != "wont_do" {
		t.Fatalf("set_feedback_progress: %v %s", out, errText)
	}
	if out, errText := callTool(t, mcs, "list_feedback", map[string]any{}); errText != "" || !strings.Contains(fmtJSON(out), `"feedback_progress":"wont_do"`) {
		t.Fatalf("list_feedback: %v %s", out, errText)
	}
	if _, errText := callTool(t, mcs, "set_feedback_progress", map[string]any{"conversation_id": chat, "progress": "done"}); !strings.Contains(errText, "not_feedback") {
		t.Fatalf("set_feedback_progress on a chat: %s", errText)
	}
	if _, errText := callTool(t, mcs, "set_feedback_progress", map[string]any{"conversation_id": chat, "progress": "none"}); errText != "" {
		t.Fatalf("none on a chat: %s", errText)
	}
	if out, errText := callTool(t, mcs, "set_feedback_progress", map[string]any{"conversation_id": fb, "progress": "none"}); errText != "" || out["feedback_progress"] != nil {
		t.Fatalf("clearing with none: %v %s", out, errText)
	}
	at.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+fb, map[string]any{"feedback_progress": "planned"})

	other := newAppTeam(t, h, false)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "PATCH", "/v1/conversations/"+fb, map[string]any{"feedback_progress": "wont_do"})
	if r := at.owner.expect(http.StatusOK, "GET", "/v1/conversations/"+fb, nil); progress(r.body) != "planned" {
		t.Fatalf("another workspace changed it: %s", r.raw)
	}
}

func describeProgressEvent(ev map[string]any) string {
	prev, next := "-", "null"
	if p, ok := ev["previous_feedback_progress"].(string); ok {
		prev = p
	}
	if n, ok := ev["feedback_progress"].(string); ok {
		next = n
	} else if _, ok := ev["feedback_progress"]; !ok {
		next = "absent"
	}
	return prev + ":" + next
}

func fmtJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

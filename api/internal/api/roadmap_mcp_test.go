package api_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRoadmapMCP(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, true)
	idea := rt.idea(rt.owner, "Dark mode for the dashboard")
	other := rt.idea(rt.owner, "Export to CSV")
	convs := []string{rt.feedback(h), rt.feedback(h)}
	free := rt.feedback(h)

	plain := rt.scopedKey()
	names := toolNames(t, h.mcpSession(plain.bearer))
	for _, want := range []string{"list_roadmap_ideas", "get_roadmap_idea", "link_feedback_to_idea", "unlink_feedback_from_idea"} {
		if !slices.Contains(names, want) {
			t.Fatalf("a key made without naming scopes lacks %s: %v", want, names)
		}
	}
	if slices.Contains(names, "move_idea_stage") {
		t.Fatal("move_idea_stage is listed to a key that does not name roadmap:stage")
	}
	if blind := toolNames(t, h.mcpSession(rt.scopedKey("conversations:read").bearer)); slices.ContainsFunc(blind, func(n string) bool { return strings.Contains(n, "idea") }) {
		t.Fatalf("roadmap tools without roadmap scopes: %v", blind)
	}
	stager := rt.scopedKey("roadmap:stage", "roadmap:read", "roadmap:write", "conversations:read")
	cs := h.mcpSession(stager.bearer)
	if !slices.Contains(toolNames(t, cs), "move_idea_stage") {
		t.Fatal("move_idea_stage is not listed to a key that names roadmap:stage")
	}
	if _, msg := callTool(t, h.mcpSession(plain.bearer), "move_idea_stage", map[string]any{"idea_id": idea, "stage": "planned", "confirm_linked": 0}); msg == "" {
		t.Fatal("move_idea_stage ran without roadmap:stage")
	}

	found, msg := callTool(t, cs, "list_roadmap_ideas", map[string]any{"query": "dar"})
	if msg != "" || len(found["items"].([]any)) != 1 || found["items"].([]any)[0].(map[string]any)["customer_content"].(map[string]any)["title"] != "Dark mode for the dashboard" {
		t.Fatalf("list_roadmap_ideas %v %s", found, msg)
	}
	if leaks := outsideCustomerContent(found, "Dark mode", "$"); len(leaks) != 0 {
		t.Fatalf("idea text outside customer_content: %v", leaks)
	}
	for _, c := range convs {
		if _, msg := callTool(t, cs, "link_feedback_to_idea", map[string]any{"idea_id": idea, "conversation_id": c}); msg != "" {
			t.Fatal(msg)
		}
	}
	if _, msg := callTool(t, cs, "link_feedback_to_idea", map[string]any{"idea_id": idea, "conversation_id": convs[0]}); msg != "" {
		t.Fatal(msg)
	}
	got, msg := callTool(t, cs, "list_feedback", map[string]any{"without_idea": true})
	if msg != "" || len(got["items"].([]any)) != 1 || got["items"].([]any)[0].(map[string]any)["id"] != free {
		t.Fatalf("feedback without an idea %v %s", got, msg)
	}

	detail, msg := callTool(t, cs, "get_roadmap_idea", map[string]any{"idea_id": idea})
	wc, _ := detail["would_change"].(map[string]any)
	if msg != "" || detail["linked_feedback"] != float64(2) || wc["planned"] != float64(2) || wc["ideas"] != float64(0) {
		t.Fatalf("get_roadmap_idea %v %s", detail, msg)
	}
	if leaks := outsideCustomerContent(detail, "Details of", "$"); len(leaks) != 0 {
		t.Fatalf("idea body outside customer_content: %v", leaks)
	}

	_, msg = callTool(t, cs, "move_idea_stage", map[string]any{"idea_id": idea, "stage": "planned", "confirm_linked": 1})
	if !strings.Contains(msg, "confirm_mismatch") {
		t.Fatalf("a wrong count: %q", msg)
	}
	if rt.owner.expect(http.StatusOK, "GET", "/v1/discussions/"+idea, nil).body["discussion"].(map[string]any)["stage"] != "ideas" {
		t.Fatal("a refused move changed the stage")
	}
	moved, msg := callTool(t, cs, "move_idea_stage", map[string]any{"idea_id": idea, "stage": "planned", "confirm_linked": 2})
	if msg != "" || moved["moved"] != true || moved["progress"] != "planned" || moved["linked"] != float64(2) {
		t.Fatalf("move_idea_stage %v %s", moved, msg)
	}
	rt.runProgress(t)
	if rt.progress(convs[0]) != "planned" {
		t.Fatal("the move did not set the progress")
	}
	if _, msg := callTool(t, cs, "unlink_feedback_from_idea", map[string]any{"idea_id": idea, "conversation_id": convs[1]}); msg != "" {
		t.Fatal(msg)
	}
	if _, msg := callTool(t, cs, "get_roadmap_idea", map[string]any{"idea_id": other}); msg != "" {
		t.Fatal(msg)
	}
	newTeam(t, h).owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/discussions/"+idea, nil)

	res, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "yuva://roadmap/" + rt.board})
	if err != nil || len(res.Contents) == 0 || !strings.Contains(res.Contents[0].Text, "Export to CSV") {
		t.Fatalf("roadmap resource: %v %v", res, err)
	}
	prompts, err := cs.ListPrompts(context.Background(), nil)
	if err != nil || !slices.ContainsFunc(prompts.Prompts, func(p *mcp.Prompt) bool { return p.Name == "link_feedback_to_ideas" }) {
		t.Fatalf("prompts %v %v", prompts, err)
	}
	pr, err := cs.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "link_feedback_to_ideas"})
	if err != nil || !strings.Contains(pr.Messages[0].Content.(*mcp.TextContent).Text, "Never call move_idea_stage") {
		t.Fatalf("prompt %v %v", pr, err)
	}

	preview := rt.owner.expectRaw(t, "GET", "/v1/roadmap-boards/"+rt.board+"/preview")
	if !strings.Contains(preview, "Export to CSV") || strings.Contains(preview, "<yuva-chat") {
		t.Fatalf("preview:\n%s", preview)
	}
}

func (c *client) expectRaw(t *testing.T, method, path string) string {
	t.Helper()
	r := c.expect(http.StatusOK, method, path, nil)
	return string(r.raw)
}

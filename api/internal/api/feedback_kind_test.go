package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCountsAndClientListByKind(t *testing.T) {
	h := newHarness(t)
	at := newAppTeam(t, h, false)
	other := newAppTeam(t, h, false)
	other.session(h, "user-1").expect(http.StatusCreated, "POST", "/client/v1/feedback", map[string]any{"category": "bug", "body": "elsewhere"})
	label := at.owner.expect(http.StatusCreated, "POST", "/v1/labels", map[string]any{"name": "vip"}).str("id")
	ownerMe := at.owner.expect(http.StatusOK, "GET", "/v1/me", nil).body["memberships"].([]any)[0].(map[string]any)["member_id"].(string)

	cs := at.session(h, "user-1")
	bug := cs.expect(http.StatusCreated, "POST", "/client/v1/feedback", map[string]any{"category": "bug", "body": "it crashes"}).body["conversation"].(map[string]any)["id"].(string)
	cs.expect(http.StatusCreated, "POST", "/client/v1/feedback", map[string]any{"category": "idea", "body": "dark mode"})
	chat := cs.start("where is my order")
	at.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+bug, map[string]any{"assignee_id": ownerMe, "labels": []string{label}})
	at.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+chat, map[string]any{"labels": []string{label}})

	type countByID struct {
		ID    string `json:"id"`
		Count int64  `json:"count"`
	}
	type counts struct {
		All, Mine, Unassigned, Spam, Feedback int64
		Inboxes, Labels, Assignees            []countByID
		FeedbackCategories                    []struct {
			Category string `json:"category"`
			Count    int64  `json:"count"`
		} `json:"feedback_categories"`
	}
	get := func(query string) counts {
		t.Helper()
		r := at.owner.expect(http.StatusOK, "GET", "/v1/conversations/counts"+query, nil)
		var out counts
		if err := json.Unmarshal(r.raw, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	one := func(l []countByID) int64 {
		if len(l) != 1 {
			return -1
		}
		return l[0].Count
	}

	all := get("")
	if all.All != 3 || all.Unassigned != 2 || all.Mine != 1 || all.Feedback != 2 || len(all.FeedbackCategories) != 2 || one(all.Labels) != 2 || one(all.Inboxes) != 3 {
		t.Fatalf("without kind %+v", all)
	}
	conv := get("?kind=conversation")
	if conv.All != 1 || conv.Unassigned != 1 || conv.Mine != 0 || conv.Feedback != 0 || len(conv.FeedbackCategories) != 0 ||
		one(conv.Labels) != 1 || one(conv.Inboxes) != 1 || len(conv.Assignees) != 0 {
		t.Fatalf("kind=conversation %+v", conv)
	}
	fb := get("?kind=feedback")
	if fb.All != 2 || fb.Unassigned != 1 || fb.Mine != 1 || fb.Feedback != 2 || len(fb.FeedbackCategories) != 2 ||
		one(fb.Labels) != 1 || one(fb.Inboxes) != 2 || one(fb.Assignees) != 1 {
		t.Fatalf("kind=feedback %+v", fb)
	}
	for _, q := range []string{"?kind=feedback,conversation", "?kind=feedback&kind=conversation"} {
		if both := get(q); both.All != all.All || both.Feedback != all.Feedback || one(both.Labels) != 2 {
			t.Fatalf("%s %+v", q, both)
		}
	}
	at.owner.expectProblem(http.StatusBadRequest, "validation_failed", "GET", "/v1/conversations/counts?kind=chat", nil)

	listKinds := func(query string) []string {
		t.Helper()
		var kinds []string
		for _, it := range cs.expect(http.StatusOK, "GET", "/client/v1/conversations"+query, nil).body["items"].([]any) {
			kinds = append(kinds, it.(map[string]any)["kind"].(string))
		}
		return kinds
	}
	if k := listKinds(""); len(k) != 3 {
		t.Fatalf("client list without kind %v", k)
	}
	if k := listKinds("?kind=feedback"); len(k) != 2 || k[0] != "feedback" || k[1] != "feedback" {
		t.Fatalf("client list kind=feedback %v", k)
	}
	if k := listKinds("?kind=conversation"); len(k) != 1 || k[0] != "conversation" {
		t.Fatalf("client list kind=conversation %v", k)
	}
	if k := listKinds("?kind=conversation,feedback"); len(k) != 3 {
		t.Fatalf("client list both kinds %v", k)
	}
	cs.expectProblem(http.StatusBadRequest, "validation_failed", "GET", "/client/v1/conversations?kind=chat", nil)
	if k := other.session(h, "user-1").expect(http.StatusOK, "GET", "/client/v1/conversations?kind=feedback", nil).body["items"].([]any); len(k) != 1 {
		t.Fatalf("another workspace's contact sees %v", k)
	}
}

func TestFeedbackIsNeverRated(t *testing.T) {
	h := newHarness(t)
	at := newAppTeam(t, h, false)
	at.owner.expect(http.StatusOK, "PATCH", "/v1/inboxes/"+at.appInbox, map[string]any{"ask_for_rating": true})
	h.clock.Advance(time.Second)
	cs := at.session(h, "user-1")
	fb := cs.expect(http.StatusCreated, "POST", "/client/v1/feedback", map[string]any{"category": "bug", "body": "it crashes"}).body["conversation"].(map[string]any)["id"].(string)
	chat := cs.start("where is my order")
	for _, id := range []string{fb, chat} {
		post(at.owner, id, map[string]any{"kind": "message", "body": "thanks"})
		at.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+id, map[string]any{"status": "closed"})
	}
	if r := cs.expect(http.StatusOK, "GET", "/client/v1/conversations/"+chat, nil); r.body["can_rate"] != true {
		t.Fatalf("closed conversation %s", r.raw)
	}
	if r := cs.expect(http.StatusOK, "GET", "/client/v1/conversations/"+fb, nil); r.body["can_rate"] != false {
		t.Fatalf("closed feedback %s", r.raw)
	}
	for _, it := range cs.expect(http.StatusOK, "GET", "/client/v1/conversations?kind=feedback", nil).body["items"].([]any) {
		if it.(map[string]any)["can_rate"] != false {
			t.Fatalf("listed feedback %v", it)
		}
	}
	cs.expectProblem(http.StatusConflict, "rating_unavailable", "POST", "/client/v1/conversations/"+fb+"/rating", map[string]any{"rating": "good"})
	cs.expect(http.StatusCreated, "POST", "/client/v1/conversations/"+chat+"/rating", map[string]any{"rating": "good"})

	if _, err := h.st.Pool.Exec(context.Background(), "UPDATE conversations SET rating = 'bad', rated_at = closed_at + interval '1 second' WHERE id = $1", fb); err != nil {
		t.Fatal(err)
	}
	if r := cs.expect(http.StatusOK, "GET", "/client/v1/conversations/"+fb, nil); r.body["can_rate"] != false || r.body["rating"] != nil {
		t.Fatalf("feedback rated before ratings left feedback %s", r.raw)
	}
	cs.expectProblem(http.StatusConflict, "rating_unavailable", "POST", "/client/v1/conversations/"+fb+"/rating", map[string]any{"rating": "good"})
}

func TestFeedbackTextAndAttachments(t *testing.T) {
	h := newHarness(t)
	at := newAppTeam(t, h, false)
	cs := at.session(h, "user-1")
	long := strings.Repeat("é", 250)
	plain := cs.expect(http.StatusCreated, "POST", "/client/v1/feedback", map[string]any{"category": "bug", "body": "\n  Settings crash  \nsteps: open settings"}).body["conversation"].(map[string]any)["id"].(string)
	cut := cs.expect(http.StatusCreated, "POST", "/client/v1/feedback", map[string]any{"category": "idea", "body": long}).body["conversation"].(map[string]any)["id"].(string)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("category", "bug")
	for _, name := range []string{"a.png", "b.png"} {
		fw, _ := mw.CreateFormFile("files", name)
		_, _ = fw.Write([]byte("\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 40)))
	}
	_ = mw.Close()
	hr, _ := http.NewRequest("POST", h.url+"/client/v1/feedback", &buf)
	hr.Header.Set("Content-Type", mw.FormDataContentType())
	hr.Header.Set("Authorization", "Bearer "+cs.token)
	res, err := http.DefaultClient.Do(hr)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("multipart feedback: %d %s", res.StatusCode, raw)
	}
	var mp map[string]any
	_ = json.Unmarshal(raw, &mp)
	shots := mp["conversation"].(map[string]any)["id"].(string)

	want := map[string][2]any{plain: {"Settings crash", float64(0)}, cut: {strings.Repeat("é", 200), float64(0)}, shots: {"", float64(2)}}
	check := func(where string, items []any) {
		t.Helper()
		seen := 0
		for _, it := range items {
			c := it.(map[string]any)
			w, ok := want[c["id"].(string)]
			if !ok {
				continue
			}
			seen++
			fb := c["feedback"].(map[string]any)
			if fb["text"] != w[0] || fb["attachment_count"] != w[1] {
				t.Fatalf("%s: %s feedback %v, want %v", where, c["id"], fb, w)
			}
		}
		if seen != len(want) {
			t.Fatalf("%s: saw %d of %d", where, seen, len(want))
		}
	}
	check("/v1 list", at.owner.expect(http.StatusOK, "GET", "/v1/conversations?kind=feedback", nil).body["items"].([]any))
	check("/client/v1 list", cs.expect(http.StatusOK, "GET", "/client/v1/conversations?kind=feedback", nil).body["items"].([]any))
	var one []any
	for id := range want {
		one = append(one, at.owner.expect(http.StatusOK, "GET", "/v1/conversations/"+id, nil).body)
	}
	check("/v1 get", one)

	key := at.apiKey(h)
	r := key.expect(http.StatusCreated, "POST", "/v1/feedback", map[string]any{
		"inbox_id": at.appInbox, "category": "praise", "body": "Love the new release\nthanks", "contact": map[string]any{"external_id": "acct-7"},
	})
	if fb := r.body["conversation"].(map[string]any)["feedback"].(map[string]any); fb["text"] != "Love the new release" || fb["attachment_count"] != float64(0) {
		t.Fatalf("api feedback %s", r.raw)
	}
	if _, err := h.st.Pool.Exec(context.Background(), "UPDATE conversations SET feedback = feedback - 'text' - 'attachment_count' WHERE id = $1", plain); err != nil {
		t.Fatal(err)
	}
	if fb := at.owner.expect(http.StatusOK, "GET", "/v1/conversations/"+plain, nil).body["feedback"].(map[string]any); fb["text"] != "" || fb["attachment_count"] != float64(0) || fb["category"] != "bug" {
		t.Fatalf("stored feedback without text %v", fb)
	}
}

func TestListByStatuses(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	other.conversation(other.owner)
	byStatus := map[string]string{}
	for _, st := range []string{"open", "pending", "snoozed", "closed"} {
		id := tm.owner.expect(http.StatusCreated, "POST", "/v1/conversations", map[string]any{"inbox_id": tm.inbox, "contact_id": tm.contact}).str("id")
		patch := map[string]any{"status": st}
		if st == "snoozed" {
			patch["snooze_until"] = h.clock.Now().Add(time.Hour).UTC().Format(time.RFC3339)
		}
		if st != "open" {
			tm.owner.expect(http.StatusOK, "PATCH", "/v1/conversations/"+id, patch)
		}
		byStatus[id] = st
	}
	statuses := func(query string) map[string]int {
		t.Helper()
		out := map[string]int{}
		for _, it := range listItems(tm.owner, query) {
			st, ok := byStatus[it["id"].(string)]
			if !ok {
				t.Fatalf("%s listed a conversation of another workspace or test: %v", query, it)
			}
			out[st]++
		}
		return out
	}
	if got := statuses(""); len(got) != 4 || got["snoozed"] != 1 {
		t.Fatalf("without status %v", got)
	}
	if got := statuses("?status=open"); len(got) != 1 || got["open"] != 1 {
		t.Fatalf("status=open %v", got)
	}
	for _, q := range []string{"?status=open,pending", "?status=open&status=pending", "?status=open,%20pending"} {
		if got := statuses(q); len(got) != 2 || got["open"] != 1 || got["pending"] != 1 {
			t.Fatalf("%s %v", q, got)
		}
	}
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "GET", "/v1/conversations?status=open,done", nil)
}

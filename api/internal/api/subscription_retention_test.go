package api_test

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestSubscriptionRetention(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	lt := newListTeam(t, h)
	add := func(lt listTeam, addr string) string {
		t.Helper()
		return lt.owner.expect(http.StatusCreated, "POST", "/v1/lists/"+lt.list+"/subscriptions", map[string]any{"email": addr, "consent": map[string]any{"confirmed": true}}).str("id")
	}
	unsubscribe := func(lt listTeam, id string) {
		t.Helper()
		lt.owner.expect(http.StatusOK, "PATCH", "/v1/lists/"+lt.list+"/subscriptions/"+id, map[string]any{"state": "unsubscribed"})
	}
	ask := func(lt listTeam, addrs ...string) {
		t.Helper()
		c, token := lt.form(h)
		h.clock.Advance(3 * time.Second)
		for _, a := range addrs {
			c.expect(http.StatusAccepted, "POST", "/client/v1/channels/"+lt.key+"/subscriptions", lt.subscribeBody(a, token))
		}
	}
	type row struct {
		state  string
		events int
	}
	look := func(id string) row {
		t.Helper()
		var r row
		if err := h.st.Pool.QueryRow(ctx, "SELECT coalesce((SELECT state FROM list_subscriptions WHERE id = $1), ''), (SELECT count(*) FROM consent_events WHERE subscription_id = $1)", id).Scan(&r.state, &r.events); err != nil {
			t.Fatal(err)
		}
		return r
	}
	optouts := func(ws string) int {
		t.Helper()
		var n int
		if err := h.st.Pool.QueryRow(ctx, "SELECT count(*) FROM email_optouts WHERE workspace_id = $1 AND reason = 'erasure_after_optout'", ws).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	run := func() {
		t.Helper()
		if err := h.srv.ExpireSubscriptions(ctx); err != nil {
			t.Fatal(err)
		}
	}

	fresh, again, gone, kept := unique("fresh")+"@example.org", unique("again")+"@example.org", unique("gone")+"@example.org", unique("kept")+"@example.org"
	keptID := add(lt, kept)
	goneID := add(lt, gone)
	unsubscribe(lt, goneID)
	againID := add(lt, again)
	unsubscribe(lt, againID)
	ask(lt, fresh, again)
	freshID := lt.subscription(t, fresh)["id"].(string)
	if r := look(againID); r.state != "pending" {
		t.Fatalf("asked again: %+v", r)
	}
	if r := look(freshID); r.state != "pending" || r.events == 0 {
		t.Fatalf("fresh: %+v", r)
	}
	before := map[string]row{kept: look(keptID), gone: look(goneID), again: look(againID)}

	h.clock.Advance(29 * 24 * time.Hour)
	run()
	if r := look(freshID); r.state != "pending" {
		t.Fatalf("pending before 30 days: %+v", r)
	}

	h.clock.Advance(2 * 24 * time.Hour)
	run()
	if r := look(freshID); r != (row{}) {
		t.Fatalf("pending after 30 days: %+v", r)
	}
	if r := look(againID); r.state != "unsubscribed" || r.events != before[again].events {
		t.Fatalf("pending that had objected: %+v, before %+v", r, before[again])
	}
	if r := look(goneID); r != before[gone] {
		t.Fatalf("unsubscribed after 30 days: %+v, before %+v", r, before[gone])
	}
	if r := look(keptID); r != before[kept] {
		t.Fatalf("subscribed after 30 days: %+v, before %+v", r, before[kept])
	}
	if optouts(lt.ws) != 0 {
		t.Fatal("opt-out written before 3 years")
	}

	h.clock.Advance(3*365*24*time.Hour - 32*24*time.Hour)
	run()
	if r := look(goneID); r != before[gone] {
		t.Fatalf("unsubscribed before 3 years: %+v", r)
	}

	other := newListTeam(t, h)
	otherKept, otherGone := add(other, kept), add(other, gone)
	unsubscribe(other, otherGone)
	ask(other, fresh)
	otherFresh := other.subscription(t, fresh)["id"].(string)
	otherBefore := map[string]row{otherKept: look(otherKept), otherGone: look(otherGone), otherFresh: look(otherFresh)}

	h.clock.Advance(2 * 24 * time.Hour)
	run()
	for _, id := range []string{goneID, againID} {
		if r := look(id); r != (row{}) {
			t.Fatalf("unsubscribed after 3 years: %+v", r)
		}
	}
	if r := look(keptID); r != before[kept] {
		t.Fatalf("subscribed after 3 years: %+v, before %+v", r, before[kept])
	}
	if n := optouts(lt.ws); n != 2 {
		t.Fatalf("opt-outs after 3 years: %d", n)
	}
	for id, r := range otherBefore {
		if got := look(id); got != r {
			t.Fatalf("other workspace %s: %+v, before %+v", id, got, r)
		}
	}
	if optouts(other.ws) != 0 {
		t.Fatal("opt-out written in the other workspace")
	}
}

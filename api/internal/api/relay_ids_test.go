package api_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
)

const sesID = "010201a12ec87f14-7c03a3b2-526f-4620-bf54-987d64efa3b3-000000@eu-west-1.amazonses.com"

func TestReplyToRelayMessageIDThreads(t *testing.T) {
	h := newHarness(t)
	et := newEmailTeam(t, h, false)
	first := newMessageID()
	conv := h.ingest(et.address, buildMail(mailOpts{from: "relay@example.net", to: et.address, subject: "Invoice", messageID: first, body: "Where is my invoice?"}), nil).str("conversation_id")
	reply := et.owner.expect(http.StatusCreated, "POST", "/v1/conversations/"+conv+"/messages", map[string]any{"kind": "message", "body": "Here it is."})
	h.smtp.mu.Lock()
	h.smtp.relay = sesID
	h.smtp.mu.Unlock()
	h.sendQueued(t, et.ws, reply.str("id"))
	if sent := lastOf(messages(et.owner, conv), "message"); sent["delivery"].(map[string]any)["state"] != "sent" {
		t.Fatalf("after send: %v", sent)
	}

	r := h.ingest(et.address, buildMail(mailOpts{
		from: "relay@example.net", to: et.address, subject: "Re: Invoice", messageID: newMessageID(),
		headers: map[string]string{"In-Reply-To": "<" + sesID + ">", "References": "<" + sesID + ">"}, body: "Thanks!",
	}), nil)
	if r.str("conversation_id") != conv {
		t.Fatalf("a reply to the relay's Message-ID did not thread: %v", r.body)
	}
	other := newEmailTeam(t, h, false)
	r = h.ingest(other.address, buildMail(mailOpts{
		from: "relay@example.net", to: other.address, subject: "Re: Invoice", messageID: newMessageID(),
		headers: map[string]string{"In-Reply-To": "<" + sesID + ">"}, body: "Wrong workspace.",
	}), nil)
	if r.str("conversation_id") == conv || r.str("conversation_id") == "" {
		t.Fatalf("another workspace joined the conversation: %v", r.body)
	}
}

func TestBroadcastReplyToRelayMessageID(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	bt := newBroadcastTeam(t, h, bs)
	bt.subscribe(t, "ada@example.org", "grace@example.org")
	var mu sync.Mutex
	relayed := map[string]string{}
	bs.mu.Lock()
	bs.relay = func(to string) string {
		mu.Lock()
		defer mu.Unlock()
		id := fmt.Sprintf("010201a12ec87f14-7c03a3b2-526f-4620-bf54-987d64efa3b3-%06d@eu-west-1.amazonses.com", len(relayed))
		relayed[to] = id
		return id
	}
	bs.mu.Unlock()
	id := bt.draft(t, nil)
	bt.schedule(t, id)
	bt.run(t)
	address := bt.owner.expect(http.StatusOK, "GET", "/v1/channels/"+bt.emailChannel, nil).body["email"].(map[string]any)["address"].(string)
	adaRelay := "<" + relayed["ada@example.org"] + ">"
	if len(relayed) != 2 || relayed["ada@example.org"] == relayed["grace@example.org"] {
		t.Fatalf("relay ids: %v", relayed)
	}

	r := h.ingest(address, buildMail(mailOpts{from: "ada@example.org", to: address, subject: "Re: News", messageID: newMessageID(), body: "Thanks!",
		headers: map[string]string{"In-Reply-To": adaRelay, "References": adaRelay}}), nil)
	conv := r.str("conversation_id")
	if r.str("status") != "stored" || conv == "" {
		t.Fatalf("reply: %v", r.body)
	}
	if c := bt.owner.expect(http.StatusOK, "GET", "/v1/conversations/"+conv, nil).body; c["broadcast_id"] != id {
		t.Fatalf("conversation: %v", c)
	}
	if ev := messages(bt.owner, conv)[0]; ev["kind"] != "event" || ev["event"].(map[string]any)["type"] != "broadcast_reply" {
		t.Fatalf("first entry: %v", ev)
	}
	if d := deliveryOf(t, bt.owner, id, "ada@example.org"); d["replied_at"] == nil {
		t.Fatalf("ada: %v", d)
	}
	again := h.ingest(address, buildMail(mailOpts{from: "ada@example.org", to: address, subject: "Re: News", messageID: newMessageID(), body: "One more thing",
		headers: map[string]string{"In-Reply-To": adaRelay}}), nil)
	if again.str("conversation_id") != conv {
		t.Fatalf("second reply opened another conversation: %v", again.body)
	}
	if cc := counts(bt.get(t, id)); cc["replied"] != float64(1) {
		t.Fatalf("counts: %v", cc)
	}
}

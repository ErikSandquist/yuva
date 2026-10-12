package api_test

import (
	"net/http"
	"testing"
	"time"
)

func TestYuvaDKIMNotThroughSES(t *testing.T) {
	h := newHarness(t)
	lt := newListTeam(t, h)
	ch := lt.owner.expect(http.StatusOK, "GET", "/v1/channels/"+lt.emailChannel, nil)
	address := ch.body["email"].(map[string]any)["address"]
	lt.owner.expect(http.StatusCreated, "POST", "/v1/channels/"+lt.emailChannel+"/dkim-key", map[string]any{"selector": "yuva"})

	lt.owner.expectProblem(http.StatusConflict, "dkim_relay_unsupported", "PATCH", "/v1/channels/"+lt.emailChannel, map[string]any{"email": map[string]any{
		"address": address, "dkim": map[string]any{"enabled": true},
		"broadcast_smtp": map[string]any{"host": "email-smtp.eu-west-1.amazonaws.com", "port": 587, "username": "AKIA", "password": "x", "tls": "starttls"},
	}})
	if h.count("SELECT count(*) FROM email_channels WHERE channel_id = $1 AND broadcast_smtp_host IS NULL AND NOT dkim_enabled", lt.emailChannel) != 1 {
		t.Fatal("the refused update was saved")
	}
	lt.owner.expectProblem(http.StatusConflict, "dkim_relay_unsupported", "PATCH", "/v1/channels/"+lt.emailChannel, map[string]any{"email": map[string]any{
		"address": address, "dkim": map[string]any{"enabled": true},
		"smtp": map[string]any{"host": "email-smtp-fips.us-east-1.amazonaws.com", "tls": "starttls"},
	}})

	if _, err := h.st.Pool.Exec(t.Context(), "UPDATE email_channels SET dkim_enabled = true WHERE channel_id = $1", lt.emailChannel); err != nil {
		t.Fatal(err)
	}
	c, token := lt.form(h)
	h.clock.Advance(3 * time.Second)
	c.expect(http.StatusAccepted, "POST", "/client/v1/channels/"+lt.key+"/subscriptions", lt.subscribeBody(unique("reader")+"@example.org", token))
	mails := h.runListMail(lt.ws)
	if len(mails) != 1 || mails[0].GetHeader("DKIM-Signature") == "" {
		t.Fatalf("a non-SES relay mail is not signed: %d", len(mails))
	}
	if mails[0].GetHeader("Date") == "" {
		t.Fatal("no Date")
	}

	if _, err := h.st.Pool.Exec(t.Context(), "UPDATE email_channels SET smtp_host = 'email-smtp.eu-west-1.amazonaws.com' WHERE channel_id = $1", lt.emailChannel); err != nil {
		t.Fatal(err)
	}
	dkim := lt.owner.expect(http.StatusOK, "GET", "/v1/channels/"+lt.emailChannel, nil).body["email"].(map[string]any)["dkim"].(map[string]any)
	if dkim["enabled"] != true || dkim["unsupported_reason"] == nil {
		t.Fatalf("dkim read model: %v", dkim)
	}
	c, token = lt.form(h)
	h.clock.Advance(3 * time.Second)
	c.expect(http.StatusAccepted, "POST", "/client/v1/channels/"+lt.key+"/subscriptions", lt.subscribeBody(unique("reader")+"@example.org", token))
	mails = h.runListMail(lt.ws)
	if len(mails) != 1 || mails[0].GetHeader("DKIM-Signature") != "" {
		t.Fatalf("mail through SES signed by Yuva: %d", len(mails))
	}

	if _, err := h.st.Pool.Exec(t.Context(), "UPDATE email_channels SET broadcast_smtp_host = 'smtp.example.com' WHERE channel_id = $1", lt.emailChannel); err != nil {
		t.Fatal(err)
	}
	dkim = lt.owner.expect(http.StatusOK, "GET", "/v1/channels/"+lt.emailChannel, nil).body["email"].(map[string]any)["dkim"].(map[string]any)
	if dkim["unsupported_reason"] != nil {
		t.Fatalf("a non-SES broadcast account is refused: %v", dkim)
	}
}

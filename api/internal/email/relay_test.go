package email_test

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/productdevbook/yuva/api/internal/email"
)

const sesProviderID = "010201a12ec87f14-7c03a3b2-526f-4620-bf54-987d64efa3b3-000000"

func TestDataReplyID(t *testing.T) {
	for reply, want := range map[string]string{
		"Ok " + sesProviderID:                  sesProviderID,
		"2.0.0 Ok: queued as 4Ab1Cd2XyZ":       "4Ab1Cd2XyZ",
		"Ok: queued as 4Ab1Cd2XyZ":             "4Ab1Cd2XyZ",
		"2.0.0 OK  1697040000 a1-20020a gsmtp": "",
		"2.0.0 queued":                         "",
		"":                                     "",
	} {
		if got := email.DataReplyID(reply); got != want {
			t.Errorf("%q: %q, want %q", reply, got, want)
		}
	}
}

func TestRelayMessageID(t *testing.T) {
	for _, tc := range []struct{ host, id, want string }{
		{"email-smtp.eu-west-1.amazonaws.com", sesProviderID, sesProviderID + "@eu-west-1.amazonses.com"},
		{"EMAIL-SMTP.eu-west-1.amazonaws.com.", sesProviderID, sesProviderID + "@eu-west-1.amazonses.com"},
		{"email-smtp.us-east-1.amazonaws.com", sesProviderID, sesProviderID + "@email.amazonses.com"},
		{"email-smtp-fips.us-gov-west-1.amazonaws.com", sesProviderID, sesProviderID + "@us-gov-west-1.amazonses.com"},
		{"smtp.example.com", sesProviderID, ""},
		{"email-smtp.eu-west-1.amazonaws.com.example.com", sesProviderID, ""},
		{"email-smtp.eu-west-1.amazonaws.com", "4Ab1Cd2XyZ", ""},
		{"email-smtp.eu-west-1.amazonaws.com", "", ""},
	} {
		if got := email.RelayMessageID(tc.host, tc.id); got != tc.want {
			t.Errorf("%s %q: %q, want %q", tc.host, tc.id, got, tc.want)
		}
	}
}

func TestSESReplyGivesRelayMessageID(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	f := &fakeSMTP{dataReply: "Ok " + sesProviderID}
	go f.serve(t, l)
	ctx := context.Background()
	sender := email.SMTPSender{AllowPrivate: true, Resolver: fixedResolver{netip.MustParseAddr("127.0.0.1")}}
	msg := []byte("Subject: hi\r\n\r\nhello\r\n")
	want := sesProviderID + "@eu-west-1.amazonses.com"
	cfg := email.SMTPConfig{Host: "email-smtp.eu-west-1.amazonaws.com", Port: l.Addr().(*net.TCPAddr).Port, TLS: "none"}

	got, err := email.SendRelayed(ctx, sender, cfg, "news@example.com", []string{"a@example.org"}, msg)
	if err != nil || got != want {
		t.Fatalf("one message: %q %v", got, err)
	}
	s, err := email.OpenSession(ctx, sender, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got, err := s.Send(ctx, "news@example.com", []string{"b@example.org"}, msg); err != nil || got != want {
		t.Fatalf("session: %q %v", got, err)
	}

	cfg.Host = "127.0.0.1"
	if got, err := email.SendRelayed(ctx, sender, cfg, "news@example.com", []string{"a@example.org"}, msg); err != nil || got != "" {
		t.Fatalf("another relay: %q %v", got, err)
	}
	if f.messages != 3 {
		t.Fatalf("messages=%d", f.messages)
	}
}

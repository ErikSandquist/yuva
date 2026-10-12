package email_test

import (
	"encoding/base64"
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-msgauth/dkim"

	"github.com/productdevbook/yuva/api/internal/email"
)

func TestTrustedAuthPrefersMailFromSPF(t *testing.T) {
	const helo = "spf=none (mx.example.net: no SPF records found for postmaster@a1.smtp.example.org) smtp.helo=a1.smtp.example.org"
	const mailFrom = "spf=pass (mx.example.net: domain of b-1@mail.example.com designates 192.0.2.1 as permitted sender) smtp.mailfrom=b-1@mail.example.com"
	for _, tc := range []struct {
		name, header, spf, domain string
	}{
		{"helo first", "mx.example.net; dkim=pass header.d=example.com header.s=s1 header.b=abc; dmarc=pass header.from=example.com policy.dmarc=none; " + helo + "; " + mailFrom + "; arc=none smtp.remote-ip=192.0.2.1", "pass", "mail.example.com"},
		{"mail from first", "mx.example.net; " + mailFrom + "; " + helo, "pass", "mail.example.com"},
		{"helo alone", "mx.example.net; " + helo, "none", "a1.smtp.example.org"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, ok := email.TrustedAuth(tc.header, "mx.example.net")
			if !ok {
				t.Fatal("not trusted")
			}
			if v.SPF != tc.spf || v.SPFDomain != tc.domain {
				t.Fatalf("spf %q %q, want %q %q", v.SPF, v.SPFDomain, tc.spf, tc.domain)
			}
		})
	}
}

func TestDKIMSurvivesRelayRewrites(t *testing.T) {
	want := []string{"From", "To", "Subject", "Reply-To", "MIME-Version", "Content-Type", "List-Unsubscribe", "List-Unsubscribe-Post", "List-Id"}
	if !slices.Equal(email.DKIMHeaders, want) {
		t.Fatalf("DKIMHeaders %v", email.DKIMHeaders)
	}
	private, public, err := email.NewDKIMKey()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := email.ParseDKIMKey(private)
	if err != nil {
		t.Fatal(err)
	}
	raw := "From: News <news@example.com>\r\nTo: a@example.org\r\nSubject: Hello\r\nDate: Mon, 12 Oct 2026 10:00:00 +0000\r\n" +
		"Message-ID: <abcdefghijklmnopqrstuvwx.abcdefghijklmnop@example.com>\r\nFeedback-ID: 1:2:3:yuva\r\n" +
		"MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nhello\r\n"
	signed, err := email.SignDKIM([]byte(raw), email.DKIMKey{Domain: "example.com", Selector: "yuva", Signer: signer})
	if err != nil {
		t.Fatal(err)
	}
	relayed := strings.NewReplacer(
		"Message-ID: <abcdefghijklmnopqrstuvwx.abcdefghijklmnop@example.com>", "Message-ID: <010201a12ec87f14-7c03a3b2-526f-4620-bf54-987d64efa3b3-000000@eu-west-1.amazonses.com>",
		"Feedback-ID: 1:2:3:yuva", "Feedback-ID: ::1.eu-west-1.abc:AmazonSES",
	).Replace(string(signed))
	verifications, err := dkim.VerifyWithOptions(strings.NewReader(relayed), &dkim.VerifyOptions{
		LookupTXT: func(domain string) ([]string, error) {
			if domain != "yuva._domainkey.example.com" {
				return nil, errors.New("no record")
			}
			return []string{"v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(public)}, nil
		},
	})
	if err != nil || len(verifications) != 1 || verifications[0].Err != nil {
		t.Fatalf("verify: %v %+v", err, verifications)
	}
	for _, h := range verifications[0].HeaderKeys {
		if strings.EqualFold(h, "Message-ID") || strings.EqualFold(h, "Feedback-ID") {
			t.Fatalf("signed %s", h)
		}
	}
}

func TestDKIMSurvivesAddressNormalizing(t *testing.T) {
	private, public, err := email.NewDKIMKey()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := email.ParseDKIMKey(private)
	if err != nil {
		t.Fatal(err)
	}
	from := email.Address{Name: "Yuva header check", Email: "yuva-ar-check@example.com"}
	raw, err := email.Build(email.Outgoing{
		From: from, ReplyTo: from, To: email.Address{Email: "deliv-e29c2d18@example.com"},
		Subject: "Header check", Text: "hello", MessageID: "abc.def@example.com",
		Date: time.Date(2026, 10, 12, 13, 0, 0, 0, time.FixedZone("", 3*3600)),
		Headers: []email.Header{
			{Name: "List-Unsubscribe", Value: "<https://example.com/m>"},
			{Name: "List-Unsubscribe-Post", Value: "List-Unsubscribe=One-Click"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	signed, err := email.SignDKIM(raw, email.DKIMKey{Domain: "example.com", Selector: "yuva", Signer: signer})
	if err != nil {
		t.Fatal(err)
	}
	msg := string(signed)
	for _, h := range []string{
		"\r\nFrom: Yuva header check <yuva-ar-check@example.com>\r\n",
		"\r\nReply-To: Yuva header check <yuva-ar-check@example.com>\r\n",
		"\r\nTo: deliv-e29c2d18@example.com\r\n",
		"\r\nDate: Mon, 12 Oct 2026 13:00:00 +0300\r\n",
	} {
		if !strings.Contains(msg, h) {
			t.Fatalf("missing %q in\n%s", h, msg)
		}
	}
	relay := regexp.MustCompile(`(?m)^(From|Reply-To|To|Date): .*$`)
	relayed := relay.ReplaceAllStringFunc(msg, func(line string) string {
		name, v, _ := strings.Cut(line, ": ")
		switch name {
		case "Date":
			return "Date: Mon, 12 Oct 2026 10:00:07 +0000\r"
		case "To":
			return "To: " + strings.Trim(strings.TrimSuffix(v, "\r"), "<>") + "\r"
		}
		return name + ": " + strings.Replace(v, `"`, "", 2)
	})
	if relayed == msg {
		t.Fatal("relay changed nothing")
	}
	verifications, err := dkim.VerifyWithOptions(strings.NewReader(relayed), &dkim.VerifyOptions{
		LookupTXT: func(domain string) ([]string, error) {
			if domain != "yuva._domainkey.example.com" {
				return nil, errors.New("no record")
			}
			return []string{"v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(public)}, nil
		},
	})
	if err != nil || len(verifications) != 1 || verifications[0].Err != nil {
		t.Fatalf("verify: %v %+v", err, verifications)
	}
}

func TestFormatAddress(t *testing.T) {
	for _, tc := range []struct {
		in   email.Address
		want string
	}{
		{email.Address{Email: "a@example.com"}, "a@example.com"},
		{email.Address{Name: "Yuva header check", Email: "a@example.com"}, "Yuva header check <a@example.com>"},
		{email.Address{Name: "O'Brien & Co", Email: "a@example.com"}, "O'Brien & Co <a@example.com>"},
		{email.Address{Name: "Acme, Inc.", Email: "a@example.com"}, `"Acme, Inc." <a@example.com>`},
		{email.Address{Name: "Dr. Who", Email: "a@example.com"}, `"Dr. Who" <a@example.com>`},
		{email.Address{Name: `Say "hi"`, Email: "a@example.com"}, `"Say \"hi\"" <a@example.com>`},
		{email.Address{Name: "two  spaces", Email: "a@example.com"}, `"two  spaces" <a@example.com>`},
		{email.Address{Name: "=?utf-8?q?x?=", Email: "a@example.com"}, `"=?utf-8?q?x?=" <a@example.com>`},
		{email.Address{Name: "Çağrı Destek", Email: "a@example.com"}, "=?utf-8?q?=C3=87a=C4=9Fr=C4=B1_Destek?= <a@example.com>"},
		{email.Address{Name: "Müller, GmbH", Email: "a@example.com"}, "=?utf-8?b?TcO8bGxlciwgR21iSA==?= <a@example.com>"},
	} {
		if got := email.FormatAddress(tc.in); got != tc.want {
			t.Errorf("FormatAddress(%+v) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

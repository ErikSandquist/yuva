package email_test

import (
	"testing"

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

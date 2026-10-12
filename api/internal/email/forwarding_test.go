package email_test

import (
	"os"
	"strings"
	"testing"

	"github.com/productdevbook/yuva/api/internal/email"
)

func TestGoogleForwardingConfirmation(t *testing.T) {
	raw, err := os.ReadFile("testdata/forwarding/gmail_confirmation.eml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := email.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := email.GoogleForwardingConfirmation(m)
	if !ok || c.Code != "462781234" || c.From != "support@example.com" ||
		c.Link != "https://mail-settings.google.com/mail/vf-%5BANGjdJ8exampleexampleexample%5D-Qx7exampleToken0000" {
		t.Fatalf("%+v %v", c, ok)
	}
	for name, change := range map[string][2]string{
		"another sender":      {"From: Gmail Team <forwarding-noreply@google.com>", "From: Gmail Team <forwarding-noreply@google.com.example.net>"},
		"codes that disagree": {"Confirmation code: 462781234", "Confirmation code: 999999999"},
		"no code":             {"(#462781234) ", ""},
		"a link to elsewhere": {"https://mail-settings.google.com/mail/vf-", "https://mail-settings.google.com.example.net/mail/vf-"},
	} {
		m, err := email.Parse([]byte(strings.Replace(string(raw), change[0], change[1], 1)))
		if err != nil {
			t.Fatal(err)
		}
		c, ok := email.GoogleForwardingConfirmation(m)
		switch name {
		case "no code":
			if !ok || c.Code != "462781234" {
				t.Errorf("%s: %+v %v", name, c, ok)
			}
		case "a link to elsewhere":
			if !ok || c.Link != "" {
				t.Errorf("%s: %+v %v", name, c, ok)
			}
		default:
			if ok {
				t.Errorf("%s: taken %+v", name, c)
			}
		}
	}
}

func TestMigrationProbeHeader(t *testing.T) {
	m, err := email.Parse([]byte("From: a@example.com\r\nTo: b@example.com\r\nSubject: t\r\nYuva-Probe:  0199-x.abc \r\n\r\nhi\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m.MigrationProbe != "0199-x.abc" {
		t.Fatalf("%q", m.MigrationProbe)
	}
}

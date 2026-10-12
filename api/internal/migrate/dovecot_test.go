package migrate

import (
	"context"
	"net"
	"net/url"
	"os"
	"strconv"
	"testing"
)

func TestDovecotConnectionTest(t *testing.T) {
	raw := os.Getenv("YUVA_TEST_IMAP_URL")
	if raw == "" {
		t.Skip("YUVA_TEST_IMAP_URL is not set")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	pass, _ := u.User.Password()
	host, portStr, _ := net.SplitHostPort(u.Host)
	port, _ := strconv.Atoi(portStr)
	mode := "tls"
	if u.Scheme == "imap" {
		mode = "starttls"
	}
	preset, _ := LookupPreset(PresetIMAP)
	tester := Tester{Resolver: net.DefaultResolver, AllowPrivate: true}
	ctx := context.Background()
	target := Target{Preset: preset, Server: Server{Host: host, Port: port, TLS: mode}, Username: "acme", Password: pass}

	res := tester.Run(ctx, target)
	if f := res.Failed(); f.Code != CodeTLSFailed || len(res.Certificate) != 64 {
		t.Fatalf("self-signed certificate: %+v", res)
	}
	target.CertificateSHA256 = res.Certificate

	target.Password = "wrong-" + pass
	res = tester.Run(ctx, target)
	if f := res.Failed(); f.Step != StepSignIn || f.Code != CodeAuthFailed || f.Detail != "NO [AUTHENTICATIONFAILED] Authentication failed." {
		t.Fatalf("wrong password: %+v", res)
	}

	for user, want := range map[string]int64{"acme": 19, "yerel": 5} {
		target.Username, target.Password = user, pass
		res = tester.Run(ctx, target)
		if !res.OK || res.Folders != 7 || res.Messages != want {
			t.Fatalf("%s: %+v", user, res)
		}
	}

	tester.AllowPrivate = false
	if res := tester.Run(ctx, target); res.Failed().Code != CodeAddressNotAllowed {
		t.Fatalf("private address without YUVA_MIGRATION_ALLOW_PRIVATE: %+v", res)
	}
}

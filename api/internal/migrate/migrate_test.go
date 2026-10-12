package migrate

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/productdevbook/yuva/api/internal/migrate/imapread"
)

func TestMXTable(t *testing.T) {
	cases := []struct {
		host, preset, note, server string
	}{
		{"smtp.google.com", PresetGoogleWorkspace, "", ""},
		{"aspmx.l.google.com.", PresetGoogleWorkspace, "", ""},
		{"alt1.aspmx.l.google.com", PresetGoogleWorkspace, "", ""},
		{"aspmx2.googlemail.com", PresetGoogleWorkspace, "", ""},
		{"example-com.mail.protection.outlook.com", PresetMicrosoft, "", ""},
		{"example-com.k-v1.mx.microsoft", PresetMicrosoft, "", ""},
		{"mx.yandex.net", PresetYandex, "", ""},
		{"mx.yandex.ru", PresetYandex, "", ""},
		{"mx.zoho.com", PresetZoho, "", "imappro.zoho.com"},
		{"mx2.zoho.eu", PresetZoho, "", "imappro.zoho.eu"},
		{"mx3.zoho.com.au", PresetZoho, "", "imappro.zoho.com.au"},
		{"mx01.mail.icloud.com", PresetICloud, "", ""},
		{"mx02.mail.icloud.com", PresetICloud, "", ""},
		{"mta5.am0.yahoodns.net", PresetYahoo, "", ""},
		{"in1-smtp.messagingengine.com", PresetFastmail, "", ""},
		{"in2-smtp.messagingengine.com", PresetFastmail, "", ""},
		{"mx00.gmx.net", PresetGMX, "", ""},
		{"mx01.gmx.net", PresetGMX, "", ""},
		{"mx00.mail.com", PresetGMX, "", ""},
		{"route1.mx.cloudflare.net", PresetIMAP, NoteCloudflare, ""},
		{"mail.protonmail.ch", PresetIMAP, NoteProton, ""},
		{"mailsec.protonmail.ch", PresetIMAP, NoteProton, ""},
	}
	for _, c := range cases {
		preset, note, server, ok := MatchMX(c.host)
		if !ok || preset != c.preset || note != c.note || server != c.server {
			t.Errorf("%s: got %q %q %q %v, want %q %q %q", c.host, preset, note, server, ok, c.preset, c.note, c.server)
		}
	}
	covered := map[int]bool{}
	for i, r := range mxRules {
		for _, c := range cases {
			if p, n, _, ok := MatchMX(c.host); ok && p == r.preset && n == r.note {
				covered[i] = true
			}
		}
	}
	if len(covered) != len(mxRules) {
		t.Errorf("%d of %d MX rules have a case", len(covered), len(mxRules))
	}
	for _, h := range []string{"mx.example.com", "google.com.evil.example", "aspmx.l.google.com.evil.example", "mx.zoho.com.evil.example", "mail.protection.outlook.com.example"} {
		if p, _, _, ok := MatchMX(h); ok {
			t.Errorf("%s matched %s", h, p)
		}
	}
}

func TestConsumerDomains(t *testing.T) {
	cases := map[string][2]string{
		"gmail.com": {PresetGmail, ""}, "googlemail.com": {PresetGmail, ""},
		"outlook.com": {PresetMicrosoft, ""}, "hotmail.co.uk": {PresetMicrosoft, ""}, "live.com.tr": {PresetMicrosoft, ""}, "msn.com": {PresetMicrosoft, ""},
		"yandex.ru": {PresetYandex, ""}, "yandex.com.tr": {PresetYandex, ""}, "ya.ru": {PresetYandex, ""},
		"icloud.com": {PresetICloud, ""}, "me.com": {PresetICloud, ""}, "mac.com": {PresetICloud, ""},
		"zohomail.com": {PresetZoho, "imap.zoho.com"}, "zohomail.eu": {PresetZoho, "imap.zoho.eu"}, "zoho.com": {PresetZoho, "imap.zoho.com"},
		"yahoo.com": {PresetYahoo, ""}, "yahoo.co.jp": {PresetYahoo, ""}, "ymail.com": {PresetYahoo, ""}, "rocketmail.com": {PresetYahoo, ""},
		"fastmail.com": {PresetFastmail, ""}, "fastmail.fm": {PresetFastmail, ""},
		"gmx.de": {PresetGMX, "imap.gmx.net"}, "gmx.net": {PresetGMX, "imap.gmx.net"}, "gmx.com": {PresetGMX, "imap.gmx.com"}, "mail.com": {PresetGMX, "imap.mail.com"},
	}
	for domain, want := range cases {
		preset, host, ok := consumer(domain)
		if !ok || preset != want[0] || host != want[1] {
			t.Errorf("%s: got %q %q %v, want %v", domain, preset, host, ok, want)
		}
	}
	for _, d := range []string{"example.com", "outlook.example.com", "gmail.example.com", "mygmail.com", "yahoo.example.org"} {
		if p, _, ok := consumer(d); ok {
			t.Errorf("%s matched %s", d, p)
		}
	}
}

type fakeResolver struct {
	mx  map[string][]*net.MX
	srv map[string][]*net.SRV
	ips map[string][]netip.Addr
}

func (r fakeResolver) LookupMX(_ context.Context, name string) ([]*net.MX, error) {
	if v, ok := r.mx[name]; ok {
		return v, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
}

func (r fakeResolver) LookupSRV(_ context.Context, service, proto, name string) (string, []*net.SRV, error) {
	key := "_" + service + "._" + proto + "." + name
	if v, ok := r.srv[key]; ok {
		return key, v, nil
	}
	return "", nil, &net.DNSError{Err: "no such host", Name: key, IsNotFound: true}
}

func (r fakeResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	if v, ok := r.ips[host]; ok {
		return v, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

func TestDetect(t *testing.T) {
	r := fakeResolver{
		mx: map[string][]*net.MX{
			"acme.example":   {{Host: "alt1.aspmx.l.google.com.", Pref: 5}, {Host: "aspmx.l.google.com.", Pref: 1}},
			"corp.example":   {{Host: "corp-example.mail.protection.outlook.com.", Pref: 0}},
			"zoho.example":   {{Host: "mx.zoho.eu.", Pref: 10}},
			"proton.example": {{Host: "mail.protonmail.ch.", Pref: 10}},
			"own.example":    {{Host: "mx.own.example.", Pref: 10}},
		},
		srv: map[string][]*net.SRV{"_imaps._tcp.own.example": {{Target: "imap.own.example.", Port: 993}}},
	}
	d := Detector{Resolver: r}
	ctx := context.Background()
	got := d.Detect(ctx, "Support@Acme.Example")
	if got.Preset != PresetGoogleWorkspace || got.FoundBy != FoundByMX || got.Server == nil || got.Server.Host != "imap.gmail.com" ||
		strings.Join(got.MX, ",") != "aspmx.l.google.com,alt1.aspmx.l.google.com" || got.Address != "support@acme.example" {
		t.Errorf("acme: %+v", got)
	}
	if got := d.Detect(ctx, "a@corp.example"); got.Preset != PresetMicrosoft || got.Available || got.Note != NoteMicrosoftLater {
		t.Errorf("corp: %+v", got)
	}
	if got := d.Detect(ctx, "a@zoho.example"); got.Preset != PresetZoho || got.Server == nil || got.Server.Host != "imappro.zoho.eu" {
		t.Errorf("zoho: %+v", got)
	}
	if got := d.Detect(ctx, "a@proton.example"); got.Note != NoteProton || got.Available {
		t.Errorf("proton: %+v", got)
	}
	if got := d.Detect(ctx, "a@own.example"); got.Preset != PresetIMAP || got.FoundBy != FoundBySRV || got.Server.Host != "imap.own.example" {
		t.Errorf("own: %+v", got)
	}
	if got := d.Detect(ctx, "a@outlook.com"); got.Preset != PresetMicrosoft || got.FoundBy != FoundByDomain || got.Available {
		t.Errorf("outlook.com: %+v", got)
	}
	if got := d.Detect(ctx, "a@nothing.example"); got.Preset != PresetIMAP || got.FoundBy != FoundByNone || got.Server != nil {
		t.Errorf("nothing: %+v", got)
	}
}

func TestParseAutoconfig(t *testing.T) {
	doc := `<?xml version="1.0"?>
<clientConfig version="1.1"><emailProvider id="own.example">
<incomingServer type="pop3"><hostname>pop.own.example</hostname><port>995</port><socketType>SSL</socketType></incomingServer>
<incomingServer type="imap"><hostname>plain.%EMAILDOMAIN%</hostname><port>143</port><socketType>plain</socketType></incomingServer>
<incomingServer type="imap"><hostname>imap.%EMAILDOMAIN%</hostname><port>993</port><socketType>SSL</socketType></incomingServer>
</emailProvider></clientConfig>`
	s, ok := parseAutoconfig([]byte(doc), "a@own.example", "own.example")
	if !ok || s != (Server{Host: "imap.own.example", Port: 993, TLS: "tls"}) {
		t.Fatalf("got %+v %v", s, ok)
	}
	if _, ok := parseAutoconfig([]byte(`<clientConfig><emailProvider><incomingServer type="imap"><hostname>bad host</hostname><port>993</port><socketType>SSL</socketType></incomingServer></emailProvider></clientConfig>`), "a@x.example", "x.example"); ok {
		t.Fatal("accepted an invalid host")
	}
}

func TestAddressesRefusePrivate(t *testing.T) {
	r := fakeResolver{ips: map[string][]netip.Addr{
		"public.example":   {netip.MustParseAddr("93.184.215.14")},
		"private.example":  {netip.MustParseAddr("10.0.0.5")},
		"mixed.example":    {netip.MustParseAddr("93.184.215.14"), netip.MustParseAddr("127.0.0.1")},
		"metadata.example": {netip.MustParseAddr("169.254.169.254")},
	}}
	ctx := context.Background()
	if _, err := Addresses(ctx, r, "public.example", false); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"private.example", "mixed.example", "metadata.example", "127.0.0.1", "localhost", "[::1]"} {
		if _, err := Addresses(ctx, r, h, false); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%s: %v", h, err)
		}
	}
	if _, err := Addresses(ctx, r, "private.example", true); err != nil {
		t.Errorf("private allowed: %v", err)
	}
	if _, err := Addresses(ctx, r, "metadata.example", true); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("metadata with private allowed: %v", err)
	}
	if _, err := Addresses(ctx, r, "missing.example", false); err == nil || errors.Is(err, ErrNotAllowed) {
		t.Errorf("missing: %v", err)
	}
}

func TestErrorCorpus(t *testing.T) {
	files, err := filepath.Glob("testdata/errors/*.txt")
	if err != nil || len(files) == 0 {
		t.Fatal("no corpus", err)
	}
	for _, f := range files {
		fields := map[string]string{}
		fh, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(fh)
		for sc.Scan() {
			k, v, _ := strings.Cut(sc.Text(), ": ")
			fields[k] = v
		}
		fh.Close()
		if fields["source"] == "" {
			t.Errorf("%s: no source", f)
		}
		typ, rest, _ := strings.Cut(fields["answer"], " ")
		se := &imapread.ServerError{Type: typ, Text: rest}
		if strings.HasPrefix(rest, "[") {
			code, text, _ := strings.Cut(rest[1:], "] ")
			se.Code, se.Text = code, text
		}
		code, detail := ClassifyAnswer(se, fields["host"])
		if code != fields["code"] {
			t.Errorf("%s: %s, want %s", f, code, fields["code"])
		}
		if clean := Scrub(detail); strings.Contains(clean, "dXNlcj1zb21ldXNlckBleGFtcGxlLmNvbQFhdXRoPUJlYXJlcg") {
			t.Errorf("%s: base64 kept in %q", f, clean)
		}
	}
}

func TestScrub(t *testing.T) {
	cases := map[string]string{
		"NO [AUTHENTICATIONFAILED] Invalid credentials (Failure)":                    "NO [AUTHENTICATIONFAILED] Invalid credentials (Failure)",
		"NO [ALERT] see https://support.google.com/accounts/answer/185833 (Failure)": "NO [ALERT] see https://support.google.com/accounts/answer/185833 (Failure)",
		"BAD bad password hunter2pass for x":                                         "BAD bad password [removed] for x",
		"BAD echo AHVzZXJAZXhhbXBsZS5jb20AaHVudGVyMnBhc3M=":                          "BAD echo [removed]",
		"NO line\r\nbreak": "NO line  break",
	}
	for in, want := range cases {
		if got := Scrub(in, "hunter2pass"); got != want {
			t.Errorf("Scrub(%q) = %q, want %q", in, got, want)
		}
	}
	if got := Scrub(strings.Repeat("a ", 400)); len([]rune(got)) > 301 {
		t.Errorf("not cut: %d", len(got))
	}
}

func TestFolderKinds(t *testing.T) {
	gmail, _ := LookupPreset(PresetGmail)
	yandex, _ := LookupPreset(PresetYandex)
	cases := []struct {
		f        imapread.Folder
		gmail    bool
		kind     string
		selected bool
		preset   Preset
	}{
		{imapread.Folder{Name: "INBOX", Delim: "/"}, false, KindInbox, true, yandex},
		{imapread.Folder{Name: "[Gmail]/All Mail", Delim: "/", Attrs: []string{`\All`, `\HasNoChildren`}}, true, KindAll, true, gmail},
		{imapread.Folder{Name: "[Gmail]/Sent Mail", Delim: "/", Attrs: []string{`\Sent`}}, true, KindSent, false, gmail},
		{imapread.Folder{Name: "[Gmail]/Spam", Delim: "/", Attrs: []string{`\Junk`}}, true, KindSpam, false, gmail},
		{imapread.Folder{Name: "Customers", Delim: "/"}, true, KindLabel, false, gmail},
		{imapread.Folder{Name: "Sent", Delim: "|"}, false, KindSent, true, yandex},
		{imapread.Folder{Name: "Отправленные", Delim: "|"}, false, KindSent, true, yandex},
		{imapread.Folder{Name: "Спам", Delim: "|"}, false, KindSpam, false, yandex},
		{imapread.Folder{Name: "Gönderilmiş Öğeler", Delim: "/"}, false, KindSent, true, yandex},
		{imapread.Folder{Name: "INBOX.Drafts", Delim: "."}, false, KindDrafts, false, yandex},
		{imapread.Folder{Name: "Projects/Archive", Delim: "/"}, false, KindUser, true, yandex},
		{imapread.Folder{Name: "Customers", Delim: "/"}, false, KindUser, true, yandex},
	}
	for _, c := range cases {
		kind := FolderKind(c.f, c.gmail)
		if kind != c.kind || DefaultSelected(c.preset, kind) != c.selected {
			t.Errorf("%s: %s %v, want %s %v", c.f.Name, kind, DefaultSelected(c.preset, kind), c.kind, c.selected)
		}
	}
}

package analytics

import (
	"bytes"
	"testing"
	"time"
	"uuid"
)

func TestShortenKeepsOnlyTheNetwork(t *testing.T) {
	for in, want := range map[string]string{
		"203.0.113.77":                "203.0.113.0/24",
		"::ffff:203.0.113.77":         "203.0.113.0/24",
		"2001:db8:abcd:12:1:2:3:4":    "2001:db8:abcd::/48",
		"2001:db8:abcd:ffff::1%eth0":  "2001:db8:abcd::/48",
		"2001:db8:abcd:0012:ffff::ff": "2001:db8:abcd::/48",
	} {
		s, ok := Shorten(in)
		if !ok || s.String() != want {
			t.Errorf("Shorten(%q) = %q, %v; want %q", in, s, ok, want)
		}
	}
	if _, ok := Shorten("not an address"); ok {
		t.Fatal("a non-address was shortened")
	}
}

func TestVisitorHashSeesOnlyTheShortenedAddress(t *testing.T) {
	salt, other := NewSalt(), NewSalt()
	if bytes.Equal(salt, other) || len(salt) != SaltBytes {
		t.Fatal("salts are not random")
	}
	p, q := uuid.New(), uuid.New()
	a, _ := Shorten("203.0.113.7")
	b, _ := Shorten("203.0.113.250")
	c, _ := Shorten("203.0.114.7")
	ua := "Mozilla/5.0"
	if VisitorHash(salt, p, a, ua) != VisitorHash(salt, p, b, ua) {
		t.Fatal("two addresses of one /24 are different visitors")
	}
	for name, h := range map[string]int64{
		"another network":  VisitorHash(salt, p, c, ua),
		"another agent":    VisitorHash(salt, p, a, ua+" x"),
		"another property": VisitorHash(salt, q, a, ua),
		"another salt":     VisitorHash(other, p, a, ua),
	} {
		if h == VisitorHash(salt, p, a, ua) {
			t.Errorf("%s gives the same visitor", name)
		}
	}
	now := time.Now()
	v := VisitorHash(salt, p, a, ua)
	if VisitHash(salt, v, now) == VisitHash(salt, v, now.Add(time.Second)) {
		t.Fatal("visits starting at different times share an id")
	}
}

func TestParsePageScrubs(t *testing.T) {
	p, ok := ParsePage("https://Example.com/users/ayse@example.com/orders/?utm_source=News&utm_medium=email&utm_campaign=jan@x.io&token=secret&email=a@b.co#top", false)
	if !ok {
		t.Fatal("not parsed")
	}
	if p.Host != "example.com" || p.Path != "/users/:email/orders/" || !p.Scrubbed {
		t.Fatalf("page %+v", p)
	}
	if p.Acquisition != (Acquisition{Source: "News", Medium: "email"}) {
		t.Fatalf("acquisition %+v", p.Acquisition)
	}
	p, _ = ParsePage("https://app.example.com/?ref=producthunt#/settings/billing", true)
	if p.Path != "/#/settings/billing" || p.Acquisition.Source != "producthunt" || p.Scrubbed {
		t.Fatalf("hash route %+v", p)
	}
	p, _ = ParsePage("https://example.com", false)
	if p.Path != "/" {
		t.Fatalf("empty path %q", p.Path)
	}
	for _, bad := range []string{"javascript:alert(1)", "file:///etc/passwd", "https://", "/relative"} {
		if _, ok := ParsePage(bad, false); ok {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestParseReferrer(t *testing.T) {
	if h, p := ParseReferrer("https://www.Google.com/search?q=my+name", "example.com"); h != "www.google.com" || p != "/search" {
		t.Fatalf("referrer %q %q", h, p)
	}
	if h, _ := ParseReferrer("https://example.com/pricing", "example.com"); h != "" {
		t.Fatal("the page's own host is a referrer")
	}
	if h, p := ParseReferrer("https://mail.example.net/u/me@example.net/inbox", "example.com"); h != "mail.example.net" || p != "/u/:email/inbox" {
		t.Fatalf("referrer %q %q", h, p)
	}
}

func TestDomainsAndPatterns(t *testing.T) {
	domains := []string{"example.com", "*.example.org"}
	for host, want := range map[string]bool{
		"example.com": true, "www.example.com": false, "docs.example.org": true, "a.b.example.org": true,
		"example.org": false, "evilexample.org": false, "example.com.evil.net": false,
	} {
		if DomainAllowed(domains, host) != want {
			t.Errorf("DomainAllowed(%q) != %v", host, want)
		}
	}
	for _, c := range []struct {
		pattern, path string
		want          bool
	}{
		{"/admin/**", "/admin", true}, {"/admin/**", "/admin/users/1", true}, {"/admin/*", "/admin/users/1", false},
		{"/orders/*", "/orders/42", true}, {"/orders/*", "/orders", false}, {"/", "/", true}, {"/a/*/c", "/a/b/c", true},
		{"/preview/*", "/preview/x#y", true},
	} {
		if MatchPath(c.pattern, c.path) != c.want {
			t.Errorf("MatchPath(%q, %q) != %v", c.pattern, c.path, c.want)
		}
	}
	rules := []PathRule{{"/orders/*", "/orders/:id"}, {"/**", "/all"}}
	if ApplyRules(rules, "/orders/42") != "/orders/:id" || ApplyRules(rules[:1], "/pricing") != "/pricing" {
		t.Fatal("path rules")
	}
}

func TestVisitsEvictTheLeastRecentlyUsed(t *testing.T) {
	v := NewVisits(2)
	p := uuid.New()
	day := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	now := time.Now()
	v.Put(p, 1, day, 11, now)
	v.Put(p, 2, day, 22, now)
	v.Get(p, 1, day)
	v.Put(p, 3, day, 33, now)
	if _, _, ok := v.Get(p, 2, day); ok || v.Len() != 2 {
		t.Fatal("the least recently used visit was kept")
	}
	if visit, _, ok := v.Get(p, 1, day); !ok || visit != 11 {
		t.Fatal("a recently used visit was evicted")
	}
	if _, _, ok := v.Get(p, 1, day.AddDate(0, 0, 1)); ok {
		t.Fatal("a visit crossed midnight")
	}
}

func TestLocalDay(t *testing.T) {
	ist, _ := time.LoadLocation("Europe/Istanbul")
	at := time.Date(2026, 10, 12, 22, 30, 0, 0, time.UTC)
	if d := LocalDay(at, ist); !d.Equal(time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("local day %v", d)
	}
	if e := DayEnd(time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC), ist); !e.Equal(time.Date(2026, 10, 13, 21, 0, 0, 0, time.UTC)) {
		t.Fatalf("day end %v", e)
	}
}

package api_test

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/productdevbook/yuva/api/internal/api"
)

const homeHost = "status.home.test"

func homeHarness(t *testing.T, tune func(*api.StatusSettings)) *harness {
	return newHarnessWith(t, func(d *api.Deps) {
		d.Status = api.StatusSettings{URL: "https://" + homeHost}
		if tune != nil {
			tune(&d.Status)
		}
	})
}

func homeGet(t *testing.T, h *harness, path string) (int, string, http.Header) {
	t.Helper()
	return onHost(t, h, homeHost, path)
}

func ldBlocks(t *testing.T, page string) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	for _, m := range regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`).FindAllStringSubmatch(page, -1) {
		var b map[string]any
		if err := json.Unmarshal([]byte(m[1]), &b); err != nil {
			t.Fatalf("JSON-LD does not parse: %v in %s", err, m[1])
		}
		out[b["@type"].(string)] = b
	}
	return out
}

func TestStatusHomeServesEveryLanguage(t *testing.T) {
	h := homeHarness(t, nil)
	code, page, hdr := homeGet(t, h, "/")
	if code != http.StatusOK || !strings.HasPrefix(hdr.Get("Content-Type"), "text/html") {
		t.Fatalf("/: %d %s", code, hdr.Get("Content-Type"))
	}
	for _, want := range []string{`<html lang="en" dir="ltr">`, `<link rel="canonical" href="https://` + homeHost + `/">`, "Tell your customers before they ask.",
		`<h1 id="hero-h">`, `<meta property="og:image" content="https://` + homeHost + `/og.png">`, `hreflang="x-default" href="https://` + homeHost + `/"`,
		`hreflang="tr" href="https://` + homeHost + `/tr/"`, `hreflang="zh-Hans" href="https://` + homeHost + `/zh-hans/"`, "/yuva-status.js"} {
		if !strings.Contains(page, want) {
			t.Fatalf("/ lacks %q", want)
		}
	}
	if n := strings.Count(page, `<h1`); n != 1 {
		t.Fatalf("%d h1", n)
	}
	if n := strings.Count(page, `rel="alternate" hreflang=`); n != 48 {
		t.Fatalf("%d hreflang alternates, want 47 and x-default", n)
	}
	if hdr.Get("Cache-Control") != "public, max-age=30, stale-if-error=86400" || hdr.Get("ETag") == "" || strings.Contains(hdr.Get("Content-Security-Policy"), "script-src") {
		t.Fatalf("headers %v", hdr)
	}
	if strings.Contains(page, "<script>") || strings.Contains(page, "https://app.useyuva.com") {
		t.Fatal("a script or a foreign host")
	}

	ld := ldBlocks(t, page)
	for _, kind := range []string{"Organization", "WebSite", "SoftwareApplication", "FAQPage"} {
		if ld[kind] == nil {
			t.Fatalf("no %s JSON-LD", kind)
		}
	}
	if q := ld["FAQPage"]["mainEntity"].([]any); len(q) != 5 || q[0].(map[string]any)["name"] != "Is it open source?" {
		t.Fatalf("FAQ %v", q)
	}
	if app := ld["SoftwareApplication"]; app["applicationCategory"] != "BusinessApplication" || app["operatingSystem"] != "Web" || app["offers"] != nil {
		t.Fatalf("SoftwareApplication %v", app)
	}

	req, _ := http.NewRequest("GET", h.url+"/", nil)
	req.Host = homeHost
	req.Header.Set("If-None-Match", hdr.Get("ETag"))
	res, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotModified {
		t.Fatalf("If-None-Match: %d", res.StatusCode)
	}

	code, tr, _ := homeGet(t, h, "/tr/")
	if code != http.StatusOK || !strings.Contains(tr, `<html lang="tr" dir="ltr">`) || !strings.Contains(tr, "Müşterileriniz sormadan önce haber verin.") ||
		!strings.Contains(tr, `<link rel="canonical" href="https://`+homeHost+`/tr/">`) || !strings.Contains(tr, `property="og:locale" content="tr_TR"`) {
		t.Fatalf("/tr/: %d", code)
	}
	if tld := ldBlocks(t, tr); tld["FAQPage"]["mainEntity"].([]any)[0].(map[string]any)["name"] != "Açık kaynak mı?" || tld["WebSite"]["inLanguage"] != "tr" {
		t.Fatal("the Turkish page carries English JSON-LD")
	}
	if _, de, _ := homeGet(t, h, "/de/"); !strings.Contains(de, "Sagen Sie es Ihren Kunden, bevor sie fragen.") || !strings.Contains(de, `lang="de"`) {
		t.Fatal("German words")
	}
	code, ar, _ := homeGet(t, h, "/ar/")
	if code != http.StatusOK || !strings.Contains(ar, `<html lang="ar" dir="rtl">`) || !strings.Contains(ar, "Tell your customers before they ask.") {
		t.Fatalf("/ar/ answers %d without right-to-left or the English fallback", code)
	}
	if code, _, _ := homeGet(t, h, "/zh-hans/"); code != http.StatusOK {
		t.Fatalf("/zh-hans/: %d", code)
	}
	if code, _, _ := homeGet(t, h, "/xx/"); code != http.StatusNotFound {
		t.Fatalf("/xx/: %d", code)
	}
}

func TestStatusHomeLangRedirect(t *testing.T) {
	h := homeHarness(t, nil)
	for path, want := range map[string]string{"/?lang=tr": "/tr/", "/?lang=de": "/de/", "/tr/?lang=en": "/", "/?lang=zh-Hans": "/zh-hans/", "/?lang=pt_br": "/pt-br/", "/tr": "/tr/"} {
		code, _, hdr := homeGet(t, h, path)
		if code != http.StatusMovedPermanently || hdr.Get("Location") != want {
			t.Fatalf("%s: %d %s, want 301 %s", path, code, hdr.Get("Location"), want)
		}
	}
	if code, _, _ := homeGet(t, h, "/?lang=nope"); code != http.StatusOK {
		t.Fatalf("unknown ?lang=: %d", code)
	}
}

func TestStatusHomeCrawling(t *testing.T) {
	h := homeHarness(t, nil)
	code, robots, _ := homeGet(t, h, "/robots.txt")
	if code != http.StatusOK || !strings.Contains(robots, "Allow: /") || !strings.Contains(robots, "Sitemap: https://"+homeHost+"/sitemap.xml") {
		t.Fatalf("robots.txt %d %s", code, robots)
	}
	code, sm, hdr := homeGet(t, h, "/sitemap.xml")
	if code != http.StatusOK || !strings.HasPrefix(hdr.Get("Content-Type"), "application/xml") {
		t.Fatalf("sitemap %d %s", code, hdr.Get("Content-Type"))
	}
	var set struct {
		URLs []struct {
			Loc     string `xml:"loc"`
			Lastmod string `xml:"lastmod"`
			Links   []struct {
				Hreflang string `xml:"hreflang,attr"`
				Href     string `xml:"href,attr"`
			} `xml:"http://www.w3.org/1999/xhtml link"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal([]byte(sm), &set); err != nil {
		t.Fatalf("sitemap does not parse: %v", err)
	}
	if len(set.URLs) != 47 || set.URLs[0].Loc != "https://"+homeHost+"/" || set.URLs[1].Loc != "https://"+homeHost+"/tr/" || set.URLs[0].Lastmod == "" || len(set.URLs[0].Links) != 48 {
		t.Fatalf("sitemap has %d urls, first %+v", len(set.URLs), set.URLs[0])
	}
	if code, _, hdr := homeGet(t, h, "/og.png"); code != http.StatusOK || hdr.Get("Content-Type") != "image/png" {
		t.Fatalf("og.png %d %s", code, hdr.Get("Content-Type"))
	}
	if code, body, _ := homeGet(t, h, "/favicon.svg"); code != http.StatusOK || !strings.HasPrefix(body, "<svg xmlns=") {
		t.Fatalf("favicon %d", code)
	}
	_, page, _ := homeGet(t, h, "/")
	font := regexp.MustCompile(`href="(/fonts/Geist-Variable[^"]+)"`).FindStringSubmatch(page)
	if font == nil {
		t.Fatal("no font preload")
	}
	if code, _, hdr := homeGet(t, h, font[1]); code != http.StatusOK || hdr.Get("Content-Type") != "font/woff2" {
		t.Fatalf("font %d %s", code, hdr.Get("Content-Type"))
	}
}

func TestStatusHomeOff(t *testing.T) {
	h := homeHarness(t, func(s *api.StatusSettings) { s.HomeOff = true })
	for _, path := range []string{"/", "/tr/", "/robots.txt", "/sitemap.xml"} {
		code, body, hdr := homeGet(t, h, path)
		if code != http.StatusNotFound || strings.Contains(body, "Tell your customers") {
			t.Fatalf("%s with the home page off: %d", path, code)
		}
		if path == "/" && (!strings.HasPrefix(hdr.Get("Content-Type"), "text/html") || !strings.Contains(body, "<!doctype html>")) {
			t.Fatalf("/ answers %s %q", hdr.Get("Content-Type"), body)
		}
	}
}

func TestStatusHomeLiveCard(t *testing.T) {
	slug := unique("home")
	h := homeHarness(t, func(s *api.StatusSettings) { s.HomePage = slug })
	tm := newTeam(t, h)
	pg := tm.owner.expect(http.StatusCreated, "POST", "/v1/status-pages", map[string]any{"inbox_id": tm.inbox, "name": "Acme Cloud", "slug": slug}).str("id")
	for _, name := range []string{"API", "Website", "Panel", "E-mail", "Chat", "Sixth"} {
		tm.owner.expect(http.StatusCreated, "POST", "/v1/status-pages/"+pg+"/components", map[string]any{"name": name})
	}
	card := func() string {
		h.clock.Advance(time.Minute)
		_, page, _ := homeGet(t, h, "/")
		return page
	}
	if page := card(); strings.Contains(page, `class="h-aside"`) || strings.Contains(page, "Acme Cloud") || !strings.Contains(page, `class="h-hero solo"`) {
		t.Fatal("an unpublished page is shown")
	}
	tm.owner.expect(http.StatusOK, "PATCH", "/v1/status-pages/"+pg, map[string]any{"published": true})
	page := card()
	if !strings.Contains(page, `class="h-aside"`) || !strings.Contains(page, "Acme Cloud") || strings.Contains(page, `h-hero solo`) ||
		!strings.Contains(page, "All systems operational") || !strings.Contains(page, "100.00%") || !strings.Contains(page, `href="https://`+homeHost+`/`+slug+`"`) {
		t.Fatalf("no live card for a published page")
	}
	if n := strings.Count(page, `class="hc-row`); n != 5 {
		t.Fatalf("%d card rows, want 5", n)
	}
	shown := 0
	for _, name := range []string{"API", "Website", "Panel", "E-mail", "Chat", "Sixth"} {
		if strings.Contains(page, "<b>"+name+"</b>") {
			shown++
		}
	}
	if shown != 5 {
		t.Fatalf("%d of 6 components shown", shown)
	}
	tm.owner.expect(http.StatusOK, "PATCH", "/v1/status-pages/"+pg, map[string]any{"published": false})
	if page := card(); strings.Contains(page, "Acme Cloud") {
		t.Fatal("a page that was unpublished again is shown")
	}
}

func TestStatusHomeLeavesDomainsAndPages(t *testing.T) {
	dns := newFakeDNS()
	h := domainHarness(t, dns, func(s *api.StatusSettings) { s.URL = "https://" + homeHost })
	st := newStatusTeam(t, h)
	st.owner.expect(http.StatusOK, "PATCH", st.path(""), map[string]any{"published": true})
	name := unique("d") + ".example.org"
	token := customDomain(t, st.owner.expect(http.StatusOK, "PATCH", st.path(""), map[string]any{"custom_domain": name}))["txt_value"].(string)
	dns.set(name, testStatusCNAME, token)
	if d := st.check(t, h); d["state"] != "verified" {
		t.Fatalf("domain %v", d)
	}
	code, body, _ := onHost(t, h, name, "/")
	if code != http.StatusOK || !strings.Contains(body, "Acme") || strings.Contains(body, "Tell your customers before they ask.") {
		t.Fatalf("/ on the own domain: %d", code)
	}
	if code, body, _ := onHost(t, h, homeHost, "/"+st.slug); code != http.StatusOK || !strings.Contains(body, "Acme") {
		t.Fatalf("/{slug} on the status host: %d", code)
	}
	if code, body, _ := onHost(t, h, name, "/robots.txt"); code == http.StatusOK && strings.Contains(body, "Sitemap") {
		t.Fatal("the own domain serves the home page's robots.txt")
	}
}

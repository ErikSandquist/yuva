package analytics

import (
	"bytes"
	"compress/gzip"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
)

var browsers = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36 Edg/141.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:143.0) Gecko/20100101 Firefox/143.0",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:143.0) Gecko/20100101 Firefox/143.0",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.6 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPad; CPU OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.6 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/141.0.7390.41 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/28.0 Chrome/130.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; SM-X710) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Android 15; Mobile; rv:143.0) Gecko/143.0 Firefox/143.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 OPR/124.0.0.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 YaBrowser/25.8.0.0 Safari/537.36",
	"Mozilla/5.0 (Linux; Android 13; M2101K6G) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36",
}

var crawlerAgents = []string{
	"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
	"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm) Chrome/116.0.1938.76 Safari/537.36",
	"Mozilla/5.0 (compatible; AhrefsBot/7.0; +http://ahrefs.com/robot/)",
	"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; GPTBot/1.2; +https://openai.com/gptbot)",
	"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; ClaudeBot/1.0; +claudebot@anthropic.com)",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/141.0.0.0 Safari/537.36",
	"curl/8.9.1",
	"Wget/1.21.4",
	"python-requests/2.32.3",
	"Go-http-client/2.0",
	"facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)",
	"Mozilla/5.0 (compatible; SemrushBot/7~bl; +http://www.semrush.com/bot.html)",
	"",
	"Mozilla",
}

func TestAgentsAndBots(t *testing.T) {
	for _, ua := range browsers {
		if a := ParseAgent(ua, "", ""); a.Bot || a.Browser == "" || a.OS == "" || a.Device == DeviceUnknown {
			t.Errorf("%q: %+v", ua, a)
		}
	}
	for _, ua := range crawlerAgents {
		if !ParseAgent(ua, "", "").Bot {
			t.Errorf("%q is not a bot", ua)
		}
	}
	a := ParseAgent(browsers[6], "", "")
	if a.Browser != "Safari" || a.BrowserMajor != 18 || a.OS != "iOS" || a.OSMajor != 18 || a.Device != DeviceMobile {
		t.Fatalf("iPhone %+v", a)
	}
	a = ParseAgent(browsers[9], `"Android"`, "?1")
	if a.Browser != "Chrome" || a.BrowserMajor != 141 || a.OS != "Android" || a.OSMajor != 15 || a.Device != DeviceMobile {
		t.Fatalf("Pixel %+v", a)
	}
	if a := ParseAgent(browsers[1], `"Windows"`, "?0"); a.OS != "Windows" || a.OSMajor != 10 || a.Device != DeviceDesktop {
		t.Fatalf("Windows %+v", a)
	}
	m := crawlers()
	start := time.Now()
	for i := 0; i < 1000; i++ {
		m.match(browsers[i%len(browsers)])
	}
	if per := time.Since(start) / 1000; per > 200*time.Microsecond {
		t.Fatalf("matching a browser against the crawler list takes %v", per)
	}
}

func TestSourcesAndChannels(t *testing.T) {
	for _, c := range []struct {
		host, path string
		a          Acquisition
		internal   bool
		source     string
		channel    Channel
	}{
		{"www.google.com", "/search", Acquisition{}, false, "Google", ChannelOrganic},
		{"www.google.com.tr", "/", Acquisition{}, false, "Google", ChannelOrganic},
		{"duckduckgo.com", "/", Acquisition{}, false, "DuckDuckGo", ChannelOrganic},
		{"chatgpt.com", "/", Acquisition{}, false, "ChatGPT", ChannelAIAssistants},
		{"gemini.google.com", "/app", Acquisition{}, false, "Gemini", ChannelAIAssistants},
		{"www.perplexity.ai", "/search", Acquisition{}, false, "Perplexity", ChannelAIAssistants},
		{"mail.google.com", "/mail/u/0", Acquisition{}, false, "Gmail", ChannelEmail},
		{"m.facebook.com", "/", Acquisition{}, false, "Facebook", ChannelSocial},
		{"t.co", "/abc", Acquisition{}, false, "Twitter", ChannelSocial},
		{"", "", Acquisition{Source: "chatgpt.com"}, false, "chatgpt.com", ChannelAIAssistants},
		{"", "", Acquisition{Source: "newsletter", Medium: "email"}, false, "newsletter", ChannelEmail},
		{"", "", Acquisition{Source: "facebook", Medium: "cpc"}, false, "facebook", ChannelPaidSocial},
		{"www.google.com", "/", Acquisition{Source: "google", Medium: "cpc"}, false, "google", ChannelPaidSearch},
		{"blog.example.net", "/post", Acquisition{}, false, "blog.example.net", ChannelReferral},
		{"docs.example.com", "/", Acquisition{}, true, "docs.example.com", ChannelInternal},
		{"", "", Acquisition{}, false, "", ChannelDirect},
	} {
		src, ch := Classify(c.host, c.path, c.a, c.internal)
		if src != c.source || ch != c.channel {
			t.Errorf("%s%s %+v: %q %s, want %q %s", c.host, c.path, c.a, src, ChannelName(ch), c.source, ChannelName(c.channel))
		}
	}
	if !SpamReferrer("best-seo-offer.com") && !SpamReferrer("www.0n-line.tv") {
		t.Fatal("no spam domain matched")
	}
	if SpamReferrer("www.google.com") || SpamReferrer("example.com") {
		t.Fatal("a real referrer is spam")
	}
}

func testMMDB(t *testing.T, path string, built time.Time) {
	t.Helper()
	tree, err := mmdbwriter.New(mmdbwriter.Options{DatabaseType: "DBIP-City-Lite", IncludeReservedNetworks: true, BuildEpoch: built.Unix()})
	if err != nil {
		t.Fatal(err)
	}
	place := func(country, region, city string) mmdbtype.Map {
		return mmdbtype.Map{
			"country":      mmdbtype.Map{"iso_code": mmdbtype.String(country)},
			"subdivisions": mmdbtype.Slice{mmdbtype.Map{"names": mmdbtype.Map{"en": mmdbtype.String(region)}}},
			"city":         mmdbtype.Map{"names": mmdbtype.Map{"en": mmdbtype.String(city)}},
		}
	}
	for cidr, p := range map[string]mmdbtype.Map{
		"203.0.113.0/24":  place("TR", "Istanbul", "Istanbul"),
		"198.51.100.0/24": place("DE", "Berlin", "Berlin"),
		"2001:db8::/32":   place("US", "California", "Los Angeles"),
	} {
		_, n, _ := net.ParseCIDR(cidr)
		if err := tree.Insert(n, p); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if _, err := tree.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGeoIPLookupsAndDownloads(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "city.mmdb")
	testMMDB(t, file, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	g := &GeoIP{Mode: GeoFile, File: file}
	if err := g.Load(); err != nil {
		t.Fatal(err)
	}
	ip, _ := Shorten("203.0.113.77")
	if l := g.Lookup(ip); l != (Location{"TR", "Istanbul", "Istanbul"}) {
		t.Fatalf("lookup %+v", l)
	}
	ip6, _ := Shorten("2001:db8:abcd:1::5")
	if l := g.Lookup(ip6); l.Country != "US" || l.City != "Los Angeles" {
		t.Fatalf("lookup %+v", l)
	}
	unknown, _ := Shorten("192.0.2.1")
	if l := g.Lookup(unknown); l != (Location{}) {
		t.Fatalf("unknown %+v", l)
	}
	if info := g.Info(); info.Source != GeoFile || !info.BuiltAt.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("info %+v", info)
	}
	_ = g.Close()

	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write(raw)
	_ = zw.Close()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if strings.Contains(r.URL.Path, "2026-11") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(gz.Bytes())
	}))
	defer srv.Close()
	now := time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC)
	downloads := t.TempDir()
	if err := os.WriteFile(filepath.Join(downloads, "dbip-city-lite-2026-08.mmdb"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	g = &GeoIP{Mode: GeoDBIP, Dir: downloads, Now: func() time.Time { return now }, URL: func(m time.Time) string { return srv.URL + "/" + m.Format("2006-01") }}
	defer g.Close()
	path, err := g.Update(context.Background())
	if err != nil || filepath.Base(path) != "dbip-city-lite-2026-10.mmdb" || hits.Load() != 2 {
		t.Fatalf("update %q %v after %d requests", path, err, hits.Load())
	}
	if _, err := os.Stat(filepath.Join(downloads, "dbip-city-lite-2026-08.mmdb")); !os.IsNotExist(err) {
		t.Fatal("the older release was kept")
	}
	if l := g.Lookup(ip); l.Country != "TR" {
		t.Fatalf("lookup after download %+v", l)
	}
	if _, err := g.Update(context.Background()); err != nil || hits.Load() != 3 {
		t.Fatalf("second update: %v, %d requests", err, hits.Load())
	}
	now = time.Date(2026, 12, 2, 3, 0, 0, 0, time.UTC)
	hits.Store(0)
	srvFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "down", http.StatusBadGateway) }))
	defer srvFail.Close()
	g.URL = func(m time.Time) string { return srvFail.URL + "/" + m.Format("2006-01") }
	if _, err := g.Update(context.Background()); err == nil || g.Info().Error == "" {
		t.Fatal("a failed download is not reported")
	}
	if l := g.Lookup(ip); l.Country != "TR" {
		t.Fatal("a failed download dropped the database in use")
	}
}

package api_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/productdevbook/yuva/api/internal/api"
)

func (c *client) page(path string) response {
	c.h.t.Helper()
	req, err := http.NewRequest("GET", c.h.url+path, nil)
	if err != nil {
		c.h.t.Fatal(err)
	}
	req.Header.Set("X-Forwarded-For", c.ip)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.h.t.Fatal(err)
	}
	defer res.Body.Close()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := res.Body.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return response{status: res.StatusCode, raw: []byte(sb.String()), header: res.Header}
}

func TestPublicRoadmapPages(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, false)
	visitor := h.client()

	private := visitor.page("/roadmap/" + rt.slug)
	unknown := visitor.page("/roadmap/" + unique("nope"))
	if private.status != 404 || unknown.status != 404 || string(private.raw) != string(unknown.raw) || strings.Contains(string(private.raw), "<html") {
		t.Fatalf("a private board is not the plain 404: %d %q / %d %q", private.status, private.raw, unknown.status, unknown.raw)
	}
	secret := rt.idea(rt.owner, "Secret plan")
	if r := visitor.page("/roadmap/" + rt.slug + "/ideas/" + secret); r.status != 404 {
		t.Fatalf("an idea of a private board renders: %d", r.status)
	}

	prev := rt.owner.expect(http.StatusOK, "GET", "/v1/roadmap-boards/"+rt.board+"/preview", nil)
	ideaPrev := rt.owner.expect(http.StatusOK, "GET", "/v1/roadmap-boards/"+rt.board+"/preview?view=idea&id="+secret+"&locale=tr", nil)
	if !strings.Contains(string(prev.raw), "Secret plan") || !strings.Contains(string(ideaPrev.raw), "Details of Secret plan") || !strings.Contains(string(ideaPrev.raw), `lang="tr"`) || prev.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("preview of a private board:\n%s", prev.raw)
	}
	rt.owner.expectProblem(http.StatusBadRequest, "validation_failed", "GET", "/v1/roadmap-boards/"+rt.board+"/preview?view=idea", nil)
	newTeam(t, h).owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/roadmap-boards/"+rt.board+"/preview", nil)
	rt.owner.expect(http.StatusOK, "PATCH", "/v1/roadmap-boards/"+rt.board, map[string]any{"public": true, "description": "What we build."})
	dark := rt.idea(rt.owner, "Dark <b>mode</b>")
	csv := rt.idea(rt.owner, "Export to CSV")
	planned := rt.idea(rt.owner, "Planned thing")
	shipped := rt.idea(rt.owner, "Shipped thing")
	rt.owner.expect(http.StatusOK, "POST", "/v1/discussions/"+planned+"/stage", map[string]any{"stage": "planned"})
	rt.owner.expect(http.StatusOK, "POST", "/v1/discussions/"+shipped+"/stage", map[string]any{"stage": "shipped"})
	ada := newReader(h, rt.chatTeam, "Ada Lovelace")
	pendingIdea := ada.site.expect(http.StatusCreated, "POST", "/client/v1/discussions", map[string]any{"kind": "idea", "title": "Pending suggestion", "body": "x"})
	rt.owner.expect(http.StatusCreated, "POST", "/v1/discussions/"+dark+"/posts", map[string]any{"body": "We **like** it."})
	rt.owner.expect(http.StatusCreated, "POST", "/v1/discussions/"+csv+"/posts", map[string]any{"body": "x"})

	home := visitor.page("/roadmap/" + rt.slug)
	body := string(home.raw)
	for _, want := range []string{"What we build.", "Planned thing", "Shipped thing", "Export to CSV", "Dark &lt;b&gt;mode&lt;/b&gt;", "<h2><span>Ideas</span>", `data-roadmap hidden`, `rel="canonical"`, "<yuva-chat"} {
		if !strings.Contains(body, want) {
			t.Fatalf("home lacks %q:\n%s", want, body)
		}
	}
	if home.status != 200 || strings.Contains(body, "Pending suggestion") {
		t.Fatalf("home %d", home.status)
	}
	if cc := home.header.Get("Cache-Control"); cc != "public, max-age=60, stale-while-revalidate=300, stale-if-error=86400" || home.header.Get("ETag") == "" {
		t.Fatalf("cache headers %v", home.header)
	}
	if csp := home.header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "'sha256-") {
		t.Fatalf("CSP %q", csp)
	}
	req, _ := http.NewRequest("GET", h.url+"/roadmap/"+rt.slug, nil)
	req.Header.Set("If-None-Match", home.header.Get("ETag"))
	req.Header.Set("X-Forwarded-For", visitor.ip)
	if res, err := http.DefaultClient.Do(req); err != nil || res.StatusCode != http.StatusNotModified {
		t.Fatalf("conditional request: %v %v", res, err)
	}

	page := visitor.page("/roadmap/" + rt.slug + "/ideas/" + dark)
	pb := string(page.raw)
	for _, want := range []string{"<strong>like</strong>", "Dark &lt;b&gt;mode&lt;/b&gt;", "BreadcrumbList", "Comments", "Suggested on", "1 comment"} {
		if !strings.Contains(pb, want) {
			t.Fatalf("idea page lacks %q:\n%s", want, pb)
		}
	}
	if page.status != 200 || strings.Contains(pb, "@") && strings.Contains(pb, "example.com") {
		t.Fatalf("idea page %d", page.status)
	}
	if r := visitor.page("/roadmap/" + rt.slug + "/ideas/" + pendingIdea.str("id")); r.status != 404 {
		t.Fatalf("a pending idea renders: %d", r.status)
	}
	if r := visitor.page("/roadmap/" + rt.slug + "/ideas/not-an-id"); r.status != 404 {
		t.Fatalf("bad id: %d", r.status)
	}
	if r := visitor.page("/roadmap/" + rt.slug + "/other"); r.status != 404 {
		t.Fatalf("unknown path: %d", r.status)
	}
	other := newRoadmapTeam(t, h, true)
	if r := visitor.page("/roadmap/" + other.slug + "/ideas/" + dark); r.status != 404 {
		t.Fatalf("an idea of another board renders: %d", r.status)
	}
	if tr := string(visitor.page("/roadmap/" + rt.slug + "?lang=tr").raw); !strings.Contains(tr, "Yol haritası") || !strings.Contains(tr, `lang="tr"`) {
		t.Fatalf("turkish page:\n%s", tr)
	}
	if de := string(visitor.page("/roadmap/" + rt.slug + "?lang=de").raw); !strings.Contains(de, "Geplant") {
		t.Fatalf("german page")
	}

	rt.owner.expect(http.StatusOK, "POST", "/v1/discussions/"+dark+"/stage", map[string]any{"stage": "closed"})
	closed := string(visitor.page("/roadmap/" + rt.slug + "/ideas/" + dark).raw)
	if !strings.Contains(closed, "Closed") {
		t.Fatal("a closed idea's own page lacks its stage")
	}
	if strings.Contains(string(visitor.page("/roadmap/"+rt.slug).raw), "Dark &lt;b&gt;") {
		t.Fatal("a closed idea is on the board")
	}

	renamed := unique("rm")
	rt.owner.expect(http.StatusOK, "PATCH", "/v1/roadmap-boards/"+rt.board, map[string]any{"slug": renamed})
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := noFollow.Get(h.url + "/roadmap/" + rt.slug + "/ideas/" + dark + "?lang=tr")
	if err != nil || res.StatusCode != http.StatusMovedPermanently || !strings.HasSuffix(res.Header.Get("Location"), "/roadmap/"+renamed+"/ideas/"+dark+"?lang=tr") {
		t.Fatalf("former slug: %v %v", res, err)
	}
	rt.owner.expect(http.StatusOK, "PATCH", "/v1/roadmap-boards/"+rt.board, map[string]any{"public": false})
	res, err = noFollow.Get(h.url + "/roadmap/" + rt.slug)
	if err != nil || res.StatusCode != http.StatusNotFound {
		t.Fatalf("former slug of a private board: %v %v", res, err)
	}
	if r := visitor.page("/roadmap/" + renamed); r.status != 404 {
		t.Fatalf("a board made private still renders from the cache: %d", r.status)
	}
}

func TestPublicRoadmapShowsRecentlyShipped(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, true)
	old, fresh := rt.idea(rt.owner, "Old release"), rt.idea(rt.owner, "Fresh release")
	rt.owner.expect(http.StatusOK, "POST", "/v1/discussions/"+old+"/stage", map[string]any{"stage": "shipped"})
	key := rt.scopedKey("roadmap:stage", "conversations:read")
	h.clock.Advance(100 * 24 * time.Hour)
	key.expect(http.StatusOK, "POST", "/v1/discussions/"+fresh+"/stage", map[string]any{"stage": "shipped"})
	v := h.client()
	home := string(v.page("/roadmap/" + rt.slug).raw)
	if strings.Contains(home, "Old release") || !strings.Contains(home, "Fresh release") {
		t.Fatalf("recent column:\n%s", home)
	}
	if all := string(v.page("/roadmap/" + rt.slug + "?stage=shipped").raw); !strings.Contains(all, "Old release") || !strings.Contains(all, "All shipped") {
		t.Fatalf("all shipped:\n%s", all)
	}
}

func TestPublicRoadmapLimitsAndOff(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, true)
	v := h.client()
	var last int
	for range 125 {
		last = v.page("/roadmap/" + rt.slug).status
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("the 125th request in a minute answered %d", last)
	}
	off := newHarnessWith(t, func(d *api.Deps) { d.Roadmap = api.RoadmapSettings{Off: true} })
	if r := off.client().page("/roadmap/anything"); r.status != http.StatusNotFound {
		t.Fatalf("roadmap off: %d", r.status)
	}
}

func TestErasedReaderLeavesThePublicPageAtOnce(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rt := newRoadmapTeam(t, h, true)
	idea := rt.idea(rt.owner, "Dark mode")
	ada := newReader(h, rt.chatTeam, "Ada Lovelace")
	reply := ada.site.expect(http.StatusCreated, "POST", "/client/v1/discussions/"+idea+"/posts", map[string]any{"body": "I need this"})
	rt.owner.expect(http.StatusOK, "PATCH", "/v1/discussion-posts/"+reply.str("id"), map[string]any{"state": "published"})
	v := h.client()
	path := "/roadmap/" + rt.slug + "/ideas/" + idea
	if !strings.Contains(string(v.page(path).raw), "Ada Lovelace") {
		t.Fatal("the reply is not on the page")
	}
	ada.expect(http.StatusNoContent, "DELETE", "/v1/me", map[string]any{"email": ada.email})
	if p := string(v.page(path).raw); strings.Contains(p, "Ada Lovelace") {
		t.Fatalf("an erased reader's name is still served:\n%s", p)
	}
}

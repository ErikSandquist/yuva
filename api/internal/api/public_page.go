package api

import (
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
	"uuid"
)

//go:embed fonts/*.woff2
var publicFontFiles embed.FS

type publicFont struct {
	name string
	body []byte
}

var publicFonts = func() map[string]publicFont {
	out := map[string]publicFont{}
	for _, f := range []string{"Geist-Variable.woff2", "GeistMono-Variable.woff2"} {
		b, err := publicFontFiles.ReadFile("fonts/" + f)
		if err != nil {
			panic(err)
		}
		h := sha256.Sum256(b)
		name := strings.TrimSuffix(f, ".woff2") + "." + hex.EncodeToString(h[:4]) + ".woff2"
		out[strings.TrimSuffix(f, "-Variable.woff2")] = publicFont{name: name, body: b}
	}
	return out
}()

func publicFontPath(family string) string { return "/fonts/" + publicFonts[family].name }

// servePublicFont answers without looking up the page: the files are the same for every page and
// carry their content hash in the name.
func servePublicFont(w http.ResponseWriter, r *http.Request) {
	name := path.Base(r.PathValue("name"))
	for _, f := range publicFonts {
		if f.name != name {
			continue
		}
		h := w.Header()
		h.Set("Content-Type", "font/woff2")
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write(f.body)
		}
		return
	}
	writeProblem(w, errNotFound)
}

var limitPublicPerIP = limit{120, time.Minute}

type publicEntry struct {
	changed, at time.Time
	ctype       string
	body        []byte
}

// publicCache keeps rendered public answers per page, view and language; an entry is used only
// while the page's changed_at is the one it was made from and for at most ttl.
type publicCache struct {
	mu      sync.Mutex
	entries map[string]publicEntry
	ttl     time.Duration
	max     int
}

func newPublicCache(ttl time.Duration, max int) *publicCache {
	return &publicCache{entries: map[string]publicEntry{}, ttl: ttl, max: max}
}

func (c *publicCache) get(key string, changed, now time.Time) (publicEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || !e.changed.Equal(changed) || now.Sub(e.at) >= c.ttl || now.Before(e.at) {
		return e, false
	}
	return e, true
}

func (c *publicCache) put(key string, e publicEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.max {
		c.entries = map[string]publicEntry{}
	}
	c.entries[key] = e
}

func (c *publicCache) forget(page uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prefix := page.String() + "|"
	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
}

// redirectPublic sends a visitor to target, keeping the query; permanent moves keep the method of a POST.
func redirectPublic(w http.ResponseWriter, r *http.Request, target string, permanent bool) {
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	post := r.Method == http.MethodPost
	code := http.StatusFound
	switch {
	case permanent && post:
		code = http.StatusPermanentRedirect
	case permanent:
		code = http.StatusMovedPermanently
	case post:
		code = http.StatusTemporaryRedirect
	}
	http.Redirect(w, r, target, code)
}

func publicScriptHash(script string) string {
	h := sha256.Sum256([]byte(script))
	return base64.StdEncoding.EncodeToString(h[:])
}

type publicCSP struct {
	img, script, connect string
	scriptHash           string
	frameAncestors       string
	formAction           string
}

func (c publicCSP) String() string {
	var b strings.Builder
	b.WriteString("default-src 'none'; style-src 'unsafe-inline'; img-src " + c.img)
	if c.connect != "" {
		b.WriteString("; connect-src " + c.connect)
	}
	b.WriteString("; font-src 'self'; script-src ")
	if c.script != "" {
		b.WriteString(c.script + " ")
	}
	b.WriteString("'sha256-" + c.scriptHash + "'")
	b.WriteString("; frame-ancestors " + c.frameAncestors + "; base-uri 'none'; form-action " + c.formAction)
	return b.String()
}

func writePublicAnswer(w http.ResponseWriter, r *http.Request, e publicEntry, cacheControl, csp string) {
	sum := sha256.Sum256(e.body)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	h := w.Header()
	h.Set("Content-Type", e.ctype)
	h.Set("Cache-Control", cacheControl)
	h.Set("ETag", etag)
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("X-Content-Type-Options", "nosniff")
	if csp != "" {
		h.Set("Content-Security-Policy", csp)
	}
	if strings.Contains(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Length", strconv.Itoa(len(e.body)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(e.body)
	}
}

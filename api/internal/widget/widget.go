package widget

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed dist
var embedded embed.FS

// AnalyticsScript and AnalyticsConsentScript are served only while analytics is on.
const (
	AnalyticsScript        = "yuva-analytics.js"
	AnalyticsConsentScript = "yuva-analytics-consent.js"
)

var Files = []string{"yuva.js", "yuva-chat.js", "yuva-docs.js", "yuva-subscribe.js", "yuva-status.js", AnalyticsScript, AnalyticsConsentScript}

func Handler(name string) http.Handler {
	body, err := embedded.ReadFile("dist/" + name)
	if err != nil {
		panic("widget: dist/" + name + " is missing: " + err.Error())
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Type", "text/javascript; charset=utf-8")
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Cache-Control", "public, max-age=300")
		h.Set("ETag", etag)
		h.Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
	})
}

func Chunks() http.Handler {
	sub, err := fs.Sub(embedded, "dist/widget")
	if err != nil {
		panic("widget: " + err.Error())
	}
	return chunkHandler(sub)
}

func ChunkNames() []string {
	entries, _ := fs.ReadDir(embedded, "dist/widget")
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".js") {
			names = append(names, e.Name())
		}
	}
	return names
}

func chunkHandler(fsys fs.FS) http.Handler {
	files := map[string][]byte{}
	current := map[string]string{}
	entries, _ := fs.ReadDir(fsys, ".")
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".js") {
			continue
		}
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			panic("widget: " + err.Error())
		}
		files[name] = body
		current[chunkStem(name)] = name
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("file")
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("X-Content-Type-Options", "nosniff")
		if body, ok := files[name]; ok {
			h.Set("Content-Type", "text/javascript; charset=utf-8")
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
			http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
			return
		}
		// A page loaded before a deploy still asks for the previous build's chunk.
		if latest, ok := current[chunkStem(name)]; ok && strings.HasSuffix(name, ".js") {
			h.Set("Cache-Control", "no-store")
			http.Redirect(w, r, latest, http.StatusFound)
			return
		}
		h.Set("Cache-Control", "no-store")
		http.NotFound(w, r)
	})
}

func chunkStem(name string) string {
	base := strings.TrimSuffix(path.Base(name), ".js")
	if i := strings.LastIndexByte(base, '-'); i > 0 {
		return base[:i]
	}
	return base
}

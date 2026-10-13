package ui

import (
	"bytes"
	"embed"
	"html"
	"io/fs"
	"net/http"
	"path"
	"slices"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

var dist, _ = fs.Sub(embedded, "dist")

// csp lets the panel load only its own code. Styles allow inline because the e-mail frame
// (srcdoc, which inherits this policy) carries its own <style>, and images allow any http(s)
// source because a member may choose to show an e-mail's remote images there.
const cspBase = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob: https: http:; font-src 'self' data:; connect-src 'self'; " +
	"frame-src 'self'; worker-src 'self'; manifest-src 'self'; object-src 'none'; base-uri 'self'; " +
	"form-action 'self'; frame-ancestors "

const csp = cspBase + "'none'"

func Handler(analyticsKey string) http.Handler {
	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		panic("ui: dist/index.html is missing: " + err.Error())
	}
	if analyticsKey != "" {
		index = withAnalytics(index, analyticsKey)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if info, err := fs.Stat(dist, name); err == nil && !info.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					w.Header().Set("Cache-Control", "no-cache")
				}
				http.ServeFileFS(w, r, dist, name)
				return
			}
			if path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		if r.Method == http.MethodGet {
			_, _ = w.Write(index)
		}
	})
}

// Ahead of the panel's scripts: deferred scripts run in document order and the panel reports its first screen at start.
func withAnalytics(index []byte, key string) []byte {
	at := bytes.Index(index, []byte("<script"))
	if at < 0 {
		at = bytes.Index(index, []byte("</head>"))
	}
	if at < 0 {
		return index
	}
	tag := `<script defer data-manual data-key="` + html.EscapeString(key) + `" src="/yuva-analytics.js"></script>` + "\n    "
	return slices.Concat(index[:at], []byte(tag), index[at:])
}

// SharePage serves the panel's page for an analytics share link: framable only by ancestors
// (origins the server has already checked), and without the panel's own analytics script, which
// would count the link's token.
func SharePage(w http.ResponseWriter, r *http.Request, ancestors []string) {
	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	if len(ancestors) == 0 {
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Frame-Options", "DENY")
	} else {
		h.Set("Content-Security-Policy", cspBase+strings.Join(ancestors, " "))
	}
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Robots-Tag", "noindex")
	if r.Method == http.MethodGet {
		_, _ = w.Write(index)
	}
}

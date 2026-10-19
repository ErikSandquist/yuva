package api

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"net/http"
	"path"
	"strings"
)

//go:embed fonts/*.woff2
var statusFontFiles embed.FS

type statusFont struct {
	name string
	body []byte
}

var statusFonts = func() map[string]statusFont {
	out := map[string]statusFont{}
	for _, f := range []string{"Geist-Variable.woff2", "GeistMono-Variable.woff2"} {
		b, err := statusFontFiles.ReadFile("fonts/" + f)
		if err != nil {
			panic(err)
		}
		h := sha256.Sum256(b)
		name := strings.TrimSuffix(f, ".woff2") + "." + hex.EncodeToString(h[:4]) + ".woff2"
		out[strings.TrimSuffix(f, "-Variable.woff2")] = statusFont{name: name, body: b}
	}
	return out
}()

func statusFontPath(family string) string { return "/fonts/" + statusFonts[family].name }

// serveStatusFont answers without looking up the page: the files are the same for every page and
// carry their content hash in the name.
func serveStatusFont(w http.ResponseWriter, r *http.Request) {
	name := path.Base(r.PathValue("name"))
	for _, f := range statusFonts {
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

package widget

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestChunkHandler(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /widget/{file}", chunkHandler(fstest.MapFS{
		"yuva-docs-thread-abc123.js": {Data: []byte("export const a = 1")},
	}))
	get := func(path string) *http.Response {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec.Result()
	}

	res := get("/widget/yuva-docs-thread-abc123.js")
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "text/javascript; charset=utf-8" ||
		res.Header.Get("Access-Control-Allow-Origin") != "*" || res.Header.Get("Cache-Control") != "public, max-age=31536000, immutable" ||
		res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("chunk: %d %v", res.StatusCode, res.Header)
	}

	res = get("/widget/yuva-docs-thread-old999.js")
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") != "/widget/yuva-docs-thread-abc123.js" ||
		res.Header.Get("Access-Control-Allow-Origin") != "*" || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("previous build's chunk: %d %v", res.StatusCode, res.Header)
	}

	for _, path := range []string{"/widget/other-abc123.js", "/widget/yuva-docs-thread-abc123.css"} {
		if res = get(path); res.StatusCode != http.StatusNotFound {
			t.Fatalf("%s: %d", path, res.StatusCode)
		}
	}
}

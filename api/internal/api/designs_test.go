package api_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
)

func (c *client) uploadImage(name, contentType string, data []byte) response {
	c.h.t.Helper()
	return c.uploadImageTo("/v1/email-images", name, contentType, data)
}

func (c *client) uploadImageTo(path, name, contentType string, data []byte) response {
	c.h.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, name))
	hdr.Set("Content-Type", contentType)
	pw, _ := w.CreatePart(hdr)
	_, _ = pw.Write(data)
	_ = w.Close()
	req, _ := http.NewRequest("POST", c.h.url+path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-Forwarded-For", c.ip)
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	} else {
		req.Header.Set("Origin", testOrigin)
	}
	if c.workspace != "" {
		req.Header.Set("Yuva-Workspace", c.workspace)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.h.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := response{status: res.StatusCode, raw: raw, header: res.Header}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

func testPNG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// gpsJPEG carries an EXIF block with a GPS position marker after the SOI.
func gpsJPEG(t *testing.T) []byte {
	t.Helper()
	var j bytes.Buffer
	if err := jpeg.Encode(&j, image.NewRGBA(image.Rect(0, 0, 40, 30)), nil); err != nil {
		t.Fatal(err)
	}
	app := append([]byte("Exif\x00\x00II*\x00\x08\x00\x00\x00\x00\x00"), []byte("GPSLatitude=41.0082;GPSLongitude=28.9784")...)
	seg := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(app)+2))
	b := j.Bytes()
	return append(append(append([]byte{}, b[:2]...), append(seg, app...)...), b[2:]...)
}

func designDoc(blocks ...map[string]any) map[string]any {
	if blocks == nil {
		blocks = []map[string]any{}
	}
	return map[string]any{
		"schema": 1,
		"theme": map[string]any{"width": 600, "font": "system", "colors": map[string]any{
			"page": "#f4f4f5", "card": "#ffffff", "text": "#18181b", "muted": "#52525b", "accent": "#2563eb", "link": "#1d4ed8", "border": "#e4e4e7"}},
		"blocks": blocks,
	}
}

func TestEmailDesigns(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	tm := newTeam(t, h)
	o := tm.owner

	tm.agent.expectProblem(http.StatusForbidden, "forbidden", "GET", "/v1/email-designs", nil)
	blank := o.expect(http.StatusCreated, "POST", "/v1/email-designs", map[string]any{"name": "Blank"})
	if blank.body["version"].(float64) != 1 || blank.body["document"].(map[string]any)["schema"].(float64) != 1 {
		t.Fatalf("blank design: %s", blank.raw)
	}

	doc := designDoc(map[string]any{"id": "h", "type": "heading", "level": 1, "text": "Hi {{ contact.first_name | \"there\" }}"},
		map[string]any{"id": "t", "type": "text", "html": "<p>It's <a href=\"https://example.com/?e={{ contact.email }}\">here</a></p>"})
	d := o.expect(http.StatusCreated, "POST", "/v1/email-designs", map[string]any{"name": "October", "inbox_id": tm.inbox, "document": doc})
	id := d.str("id")
	if !strings.Contains(string(d.raw), "merge_field_without_fallback") || !strings.Contains(string(d.raw), "sender_identity_missing") {
		t.Errorf("warnings: %s", d.raw)
	}

	bad := designDoc(map[string]any{"id": "x", "type": "text", "html": "<p onclick=\"steal()\">hi</p>"})
	r := o.expect(http.StatusUnprocessableEntity, "PUT", "/v1/email-designs/"+id, map[string]any{"name": "October", "document": bad, "base_version": 1})
	if r.str("code") != "invalid_rich_text" || !strings.Contains(string(r.raw), `"block_id":"x"`) {
		t.Errorf("rich text: %s", r.raw)
	}
	unknown := designDoc(map[string]any{"id": "x", "type": "text", "html": "<p>{{ contact.password }}</p>"})
	o.expectProblem(http.StatusUnprocessableEntity, "unknown_merge_field", "PUT", "/v1/email-designs/"+id, map[string]any{"name": "October", "document": unknown, "base_version": 1})
	newer := designDoc()
	newer["schema"] = 2
	o.expectProblem(http.StatusUnprocessableEntity, "unsupported_schema", "POST", "/v1/email-designs/render", map[string]any{"document": newer})

	saved := o.expect(http.StatusOK, "PUT", "/v1/email-designs/"+id, map[string]any{"name": "October news", "document": doc, "base_version": 1, "inbox_id": tm.inbox})
	if saved.body["version"].(float64) != 2 {
		t.Fatalf("save: %s", saved.raw)
	}
	stale := o.expect(http.StatusConflict, "PUT", "/v1/email-designs/"+id, map[string]any{"name": "Other tab", "document": doc, "base_version": 1})
	if stale.str("code") != "version_conflict" || stale.body["design"].(map[string]any)["name"] != "October news" {
		t.Fatalf("conflict: %s", stale.raw)
	}
	o.expect(http.StatusOK, "PUT", "/v1/email-designs/"+id, map[string]any{"name": "October news", "document": designDoc(), "base_version": 2, "snapshot": true, "inbox_id": tm.inbox})
	versions := o.expect(http.StatusOK, "GET", "/v1/email-designs/"+id+"/versions", nil).body["items"].([]any)
	if len(versions) != 3 || versions[0].(map[string]any)["reason"] != "manual" || versions[1].(map[string]any)["reason"] != "conflict" || versions[2].(map[string]any)["version"].(float64) != 1 {
		t.Fatalf("versions: %v", versions)
	}
	restored := o.expect(http.StatusOK, "POST", "/v1/email-designs/"+id+"/versions/1/restore", map[string]any{"base_version": 3})
	if restored.body["version"].(float64) != 4 || len(restored.body["document"].(map[string]any)["blocks"].([]any)) != 2 {
		t.Fatalf("restore: %s", restored.raw)
	}

	rendered := o.expect(http.StatusOK, "POST", "/v1/email-designs/render", map[string]any{"document": doc, "contact_id": tm.contact, "inbox_id": tm.inbox, "dark": true})
	if html := rendered.str("html"); !strings.Contains(html, "Hi Ayşe") || !strings.Contains(html, "ayse%40example.com") || rendered.body["size_bytes"].(float64) < 1000 {
		t.Errorf("render: %s", rendered.raw)
	}

	copied := o.expect(http.StatusCreated, "POST", "/v1/email-designs/"+id+"/duplicate", nil)
	if copied.str("name") != "October news (copy)" {
		t.Errorf("duplicate: %s", copied.raw)
	}
	o.expect(http.StatusNoContent, "DELETE", "/v1/email-designs/"+copied.str("id"), nil)
	list := o.expect(http.StatusOK, "GET", "/v1/email-designs?q=october", nil).body["items"].([]any)
	if len(list) != 1 {
		t.Errorf("list: %v", list)
	}
	if n := len(o.expect(http.StatusOK, "GET", "/v1/email-designs?archived=true", nil).body["items"].([]any)); n != 1 {
		t.Errorf("archived: %d", n)
	}

	other := newTeam(t, h)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/email-designs/"+id, nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "PUT", "/v1/email-designs/"+id, map[string]any{"name": "x", "document": doc, "base_version": 4})
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/email-designs/"+id+"/versions", nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/email-designs/render", map[string]any{"document": doc, "contact_id": tm.contact})

	reader := h.client()
	reader.bearer = o.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "reader", "scopes": []string{"broadcasts:read"}}).str("secret")
	reader.expect(http.StatusOK, "GET", "/v1/email-designs/"+id, nil)
	r = reader.expect(http.StatusForbidden, "POST", "/v1/email-designs", map[string]any{"name": "x"})
	if r.str("code") != "insufficient_scope" || r.str("scope") != "broadcasts:write" {
		t.Errorf("scope: %s", r.raw)
	}
	none := h.client()
	none.bearer = o.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "lists", "scopes": []string{"lists:read"}}).str("secret")
	none.expectProblem(http.StatusForbidden, "insufficient_scope", "GET", "/v1/email-designs", nil)
	o.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "send", "scopes": []string{"broadcasts:send"}})
}

func TestEmailDesignConflictKeepsTheirs(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	o := newTeam(t, h).owner

	id := o.expect(http.StatusCreated, "POST", "/v1/email-designs", map[string]any{"name": "News"}).str("id")
	theirs := designDoc(map[string]any{"id": "a", "type": "heading", "level": 1, "text": "Tab A"})
	mine := designDoc(map[string]any{"id": "b", "type": "heading", "level": 1, "text": "Tab B"})
	o.expect(http.StatusOK, "PUT", "/v1/email-designs/"+id, map[string]any{"name": "News", "document": theirs, "base_version": 1})
	o.expectProblem(http.StatusConflict, "version_conflict", "PUT", "/v1/email-designs/"+id, map[string]any{"name": "News", "document": mine, "base_version": 1})
	o.expectProblem(http.StatusConflict, "version_conflict", "PUT", "/v1/email-designs/"+id, map[string]any{"name": "News", "document": mine, "base_version": 1})
	kept := o.expect(http.StatusOK, "PUT", "/v1/email-designs/"+id, map[string]any{"name": "News", "document": mine, "base_version": 2})
	if kept.body["version"].(float64) != 3 {
		t.Fatalf("keep mine: %s", kept.raw)
	}

	heading := func(v int) string {
		r := o.expect(http.StatusOK, "GET", "/v1/email-designs/"+id+"/versions/"+strconv.Itoa(v), nil)
		return r.body["document"].(map[string]any)["blocks"].([]any)[0].(map[string]any)["text"].(string)
	}
	if got := heading(2); got != "Tab A" {
		t.Fatalf("their version: %q", got)
	}
	versions := o.expect(http.StatusOK, "GET", "/v1/email-designs/"+id+"/versions", nil).body["items"].([]any)
	if len(versions) != 2 || versions[0].(map[string]any)["version"].(float64) != 2 || versions[0].(map[string]any)["reason"] != "conflict" {
		t.Fatalf("versions: %v", versions)
	}

	o.expectProblem(http.StatusConflict, "version_conflict", "POST", "/v1/email-designs/"+id+"/versions/2/restore", map[string]any{"base_version": 2})
	if got := heading(3); got != "Tab B" {
		t.Fatalf("restore conflict version: %q", got)
	}

	for v := 3; v < 60; v++ {
		o.expect(http.StatusOK, "PUT", "/v1/email-designs/"+id, map[string]any{"name": "News", "document": mine, "base_version": v, "snapshot": true})
	}
	versions = o.expect(http.StatusOK, "GET", "/v1/email-designs/"+id+"/versions", nil).body["items"].([]any)
	last := versions[len(versions)-1].(map[string]any)
	if len(versions) != 50 || versions[0].(map[string]any)["version"].(float64) != 60 || last["version"].(float64) != 3 || last["reason"] != "conflict" {
		t.Fatalf("pruned versions: %d, newest %v, oldest %v", len(versions), versions[0], last)
	}
	if versions[len(versions)-2].(map[string]any)["version"].(float64) != 12 {
		t.Fatalf("kept besides the conflict snapshot: %v", versions[len(versions)-2])
	}
}

func TestEmailImages(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	tm := newTeam(t, h)
	o := tm.owner

	pic := testPNG(t, 2400, 600, color.RGBA{30, 64, 175, 255})
	up := o.uploadImage("banner.png", "image/png", pic)
	if up.status != http.StatusCreated || up.body["width"].(float64) != 1200 || up.body["height"].(float64) != 300 || up.body["content_type"] != "image/png" {
		t.Fatalf("upload: %d %s", up.status, up.raw)
	}
	if again := o.uploadImage("same.png", "image/png", pic); again.status != http.StatusOK || again.str("id") != up.str("id") {
		t.Fatalf("dedupe: %d %s", again.status, again.raw)
	}
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`)
	if r := o.uploadImage("logo.png", "image/png", svg); r.status != http.StatusUnsupportedMediaType || r.str("code") != "image_type" {
		t.Fatalf("svg: %d %s", r.status, r.raw)
	}
	photo := o.uploadImage("photo.jpg", "image/jpeg", gpsJPEG(t))
	if photo.status != http.StatusCreated {
		t.Fatalf("jpeg: %d %s", photo.status, photo.raw)
	}

	pub := h.client()
	imageURL := h.url + strings.TrimPrefix(photo.str("url"), testOrigin)
	res, err := pub.http.Get(imageURL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || bytes.Contains(body, []byte("GPS")) || bytes.Contains(body, []byte("Exif")) {
		t.Fatalf("served jpeg: %d, metadata kept: %v", res.StatusCode, bytes.Contains(body, []byte("GPS")))
	}
	for k, want := range map[string]string{"Cache-Control": "public, max-age=31536000, immutable", "Content-Type": "image/jpeg",
		"X-Content-Type-Options": "nosniff", "Cross-Origin-Resource-Policy": "cross-origin"} {
		if got := res.Header.Get(k); got != want {
			t.Errorf("%s: %q, want %q", k, got, want)
		}
	}
	if res.Header.Get("Set-Cookie") != "" || res.Header.Get("ETag") == "" {
		t.Errorf("cookies or no etag: %v", res.Header)
	}
	req, _ := http.NewRequest("GET", imageURL, nil)
	req.Header.Set("If-None-Match", res.Header.Get("ETag"))
	cached, err := pub.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	cached.Body.Close()
	if cached.StatusCode != http.StatusNotModified {
		t.Errorf("If-None-Match: %d", cached.StatusCode)
	}

	use := designDoc(map[string]any{"id": "hero", "type": "image", "image_id": up.str("id"), "alt": "Banner"})
	d := o.expect(http.StatusCreated, "POST", "/v1/email-designs", map[string]any{"name": "With image", "document": use})
	r := o.expect(http.StatusConflict, "DELETE", "/v1/email-images/"+up.str("id"), nil)
	if r.str("code") != "image_in_use" || !strings.Contains(string(r.raw), d.str("id")) {
		t.Fatalf("in use: %s", r.raw)
	}
	other := newTeam(t, h)
	other.owner.expectProblem(http.StatusUnprocessableEntity, "foreign_asset", "POST", "/v1/email-designs", map[string]any{"name": "Theft", "document": use})
	other.owner.expectProblem(http.StatusNotFound, "not_found", "DELETE", "/v1/email-images/"+up.str("id"), nil)
	if n := len(other.owner.expect(http.StatusOK, "GET", "/v1/email-images", nil).body["items"].([]any)); n != 0 {
		t.Errorf("other workspace sees %d images", n)
	}
	o.expect(http.StatusNoContent, "DELETE", "/v1/email-designs/"+d.str("id"), nil)
	o.expect(http.StatusNoContent, "DELETE", "/v1/email-images/"+photo.str("id"), nil)
	if gone, _ := pub.http.Get(imageURL); gone.StatusCode != http.StatusNotFound {
		t.Errorf("deleted image still served: %d", gone.StatusCode)
	}
	if icon, _ := pub.http.Get(h.url + "/i/s/github-dark.png"); icon.StatusCode != http.StatusOK || icon.Header.Get("Content-Type") != "image/png" {
		t.Errorf("icon: %d", icon.StatusCode)
	}
}

func TestEmailTemplates(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	tm := newTeam(t, h)
	o := tm.owner
	items := o.expect(http.StatusOK, "GET", "/v1/email-templates?locale=tr", nil).body["items"].([]any)
	if len(items) != 6 || items[0].(map[string]any)["slug"] != "newsletter" || items[0].(map[string]any)["name"] != "Bülten" {
		t.Fatalf("templates: %v", items)
	}
	tpl := o.expect(http.StatusOK, "GET", "/v1/email-templates/newsletter?locale=de", nil)
	if tpl.body["locale"] != "de" || len(tpl.body["images"].([]any)) != 5 {
		t.Fatalf("template: %s", tpl.raw)
	}
	o.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/email-templates/nope", nil)

	d := o.expect(http.StatusCreated, "POST", "/v1/email-designs", map[string]any{"name": "Bülten", "template": map[string]any{"slug": "newsletter", "locale": "tr"}})
	if d.str("template_slug") != "newsletter" || strings.Contains(string(d.raw), `"asset"`) || !strings.Contains(string(d.raw), "Devamını okuyun") {
		t.Fatalf("from template: %s", d.raw)
	}
	imgs := o.expect(http.StatusOK, "GET", "/v1/email-images", nil).body["items"].([]any)
	if len(imgs) != 5 {
		t.Fatalf("template images: %d", len(imgs))
	}
	o.expect(http.StatusCreated, "POST", "/v1/email-designs", map[string]any{"name": "Again", "template": map[string]any{"slug": "newsletter"}})
	if n := len(o.expect(http.StatusOK, "GET", "/v1/email-images", nil).body["items"].([]any)); n != 5 {
		t.Errorf("template images not reused: %d", n)
	}

	pub := h.client()
	for _, path := range []string{"/email-templates/newsletter/tr.html", "/email-templates/newsletter/en.webp", "/email-templates/newsletter/assets/hero.jpg"} {
		res, err := pub.http.Get(h.url + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK || res.Header.Get("Cache-Control") != "public, max-age=31536000, immutable" {
			t.Errorf("%s: %d %v", path, res.StatusCode, res.Header)
		}
		if strings.HasSuffix(path, ".html") && !strings.Contains(res.Header.Get("Content-Security-Policy"), "style-src 'unsafe-inline'") {
			t.Errorf("%s: CSP %q blocks the preview's styles", path, res.Header.Get("Content-Security-Policy"))
		}
	}
	if res, _ := pub.http.Get(h.url + "/email-templates/newsletter/../../v1/me"); res.StatusCode == http.StatusOK {
		t.Error("path escaped the gallery")
	}
}

package api_test

import (
	"context"
	"image/color"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/api"
)

type helpTeam struct {
	team
	h      *harness
	center string
}

func newHelpTeam(t *testing.T, h *harness, locales ...string) helpTeam {
	t.Helper()
	tm := newTeam(t, h)
	if len(locales) == 0 {
		locales = []string{"en"}
	}
	c := tm.owner.expect(http.StatusCreated, "POST", "/v1/help-centers", map[string]any{
		"inbox_id": tm.inbox, "name": "Acme help", "slug": unique("acme"), "default_locale": locales[0], "locales": locales,
	})
	return helpTeam{team: tm, h: h, center: c.str("id")}
}

func (ht helpTeam) path(p string) string { return "/v1/help-centers/" + ht.center + p }

func (ht helpTeam) article(c *client, locale, title, body string) (string, map[string]any) {
	ht.h.t.Helper()
	r := c.expect(http.StatusCreated, "POST", ht.path("/articles"), map[string]any{"locale": locale, "title": title, "body": body})
	return r.str("id"), r.body["translations"].([]any)[0].(map[string]any)
}

func (ht helpTeam) publish(c *client, article, locale string) response {
	ht.h.t.Helper()
	return c.expect(http.StatusOK, "POST", ht.path("/articles/"+article+"/translations/"+locale+"/publish"), nil)
}

func (ht helpTeam) key(name string, body map[string]any) *client {
	ht.h.t.Helper()
	body["name"] = name
	k := ht.h.client()
	k.bearer = ht.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", body).str("secret")
	return k
}

func (ht helpTeam) events(typ string) []map[string]any {
	ht.h.t.Helper()
	var out []map[string]any
	for _, e := range ht.owner.expect(http.StatusOK, "GET", "/v1/events?after=0&limit=500", nil).body["events"].([]any) {
		if m := e.(map[string]any); m["type"] == typ {
			out = append(out, m)
		}
	}
	return out
}

func TestHelpRolesAndScopes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ht := newHelpTeam(t, h, "en", "tr")
	ht.owner.expectProblem(http.StatusConflict, "help_center_exists", "POST", "/v1/help-centers", map[string]any{
		"inbox_id": ht.inbox, "name": "Again", "slug": unique("again"),
	})
	ht.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/help-centers", map[string]any{
		"inbox_id": ht.inbox, "name": "Reserved", "slug": "help",
	})

	ht.agent.expectProblem(http.StatusNotFound, "not_found", "GET", ht.path(""), nil)
	ht.agent.expectProblem(http.StatusNotFound, "not_found", "POST", ht.path("/articles"), map[string]any{"locale": "en", "title": "Hidden"})
	ht.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+ht.inbox+"/members/"+ht.agentID, nil)
	if c := ht.agent.expect(http.StatusOK, "GET", ht.path(""), nil); c.body["can_publish"] != false {
		t.Fatalf("agent can_publish: %v", c.body["can_publish"])
	}
	ht.agent.expectProblem(http.StatusForbidden, "forbidden", "PATCH", ht.path(""), map[string]any{"name": "Mine"})
	ht.agent.expectProblem(http.StatusForbidden, "forbidden", "DELETE", ht.path(""), nil)
	col := ht.agent.expect(http.StatusCreated, "POST", ht.path("/collections"), map[string]any{"translations": map[string]any{"en": map[string]any{"name": "Billing"}}})
	art, _ := ht.article(ht.agent, "en", "Agent wrote this", "Body")
	ht.agent.expect(http.StatusOK, "PATCH", ht.path("/articles/"+art), map[string]any{"collection_id": col.str("id")})
	ht.agent.expectProblem(http.StatusForbidden, "forbidden", "POST", ht.path("/articles/"+art+"/translations/en/publish"), nil)

	ht.owner.expect(http.StatusOK, "PATCH", ht.path(""), map[string]any{"agents_may_publish": true})
	if c := ht.agent.expect(http.StatusOK, "GET", ht.path(""), nil); c.body["can_publish"] != true {
		t.Fatalf("agent can_publish with agents_may_publish: %v", c.body["can_publish"])
	}
	ht.publish(ht.agent, art, "en")
	ht.agent.expect(http.StatusOK, "POST", ht.path("/articles/"+art+"/translations/en/unpublish"), nil)
	ht.publish(ht.agent, art, "en")
	ht.owner.expect(http.StatusOK, "PATCH", ht.path(""), map[string]any{"agents_may_publish": false})
	ht.agent.expectProblem(http.StatusForbidden, "forbidden", "DELETE", ht.path("/articles/"+art), nil)
	ht.agent.expect(http.StatusCreated, "PUT", ht.path("/articles/"+art+"/translations/tr"), map[string]any{"base_version": 0, "title": "Ajan yazdı"})
	ht.agent.expect(http.StatusNoContent, "DELETE", ht.path("/articles/"+art+"/translations/tr"), nil)
	ht.agent.expectProblem(http.StatusForbidden, "forbidden", "DELETE", ht.path("/articles/"+art+"/translations/en"), nil)

	other := ht.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Other", "slug": "other"}).body["inbox"].(map[string]any)["id"].(string)
	otherCenter := ht.owner.expect(http.StatusCreated, "POST", "/v1/help-centers", map[string]any{"inbox_id": other, "name": "Other", "slug": unique("other")}).str("id")
	ht.agent.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/help-centers/"+otherCenter, nil)
	if items := ht.agent.expect(http.StatusOK, "GET", "/v1/help-centers", nil).body["items"].([]any); len(items) != 1 {
		t.Fatalf("agent sees %d centers", len(items))
	}

	stranger := newTeam(t, h)
	stranger.owner.expectProblem(http.StatusNotFound, "not_found", "GET", ht.path(""), nil)
	stranger.owner.expectProblem(http.StatusNotFound, "not_found", "GET", ht.path("/articles/"+art), nil)
	strangerKey := h.client()
	strangerKey.bearer = stranger.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "k"}).str("secret")
	strangerKey.expectProblem(http.StatusNotFound, "not_found", "GET", ht.path(""), nil)
	strangerKey.expectProblem(http.StatusNotFound, "not_found", "PUT", ht.path("/articles/"+art+"/translations/en"), map[string]any{"base_version": 1, "title": "Taken over"})

	full := ht.key("default", map[string]any{})
	full.expect(http.StatusOK, "GET", ht.path("/articles/"+art), nil)
	draft, _ := ht.article(full, "en", "Key draft", "")
	full.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", ht.path("/articles/"+draft+"/translations/en/publish"), nil)
	full.expectProblem(http.StatusForbidden, "insufficient_scope", "DELETE", ht.path("/articles/"+art), nil)
	full.expectProblem(http.StatusForbidden, "insufficient_scope", "PATCH", ht.path(""), map[string]any{"name": "Key"})
	publisher := ht.key("publisher", map[string]any{"scopes": []string{"help:read", "help:write", "help:publish"}})
	ht.publish(publisher, draft, "en")
	reader := ht.key("reader", map[string]any{"scopes": []string{"help:read"}})
	reader.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", ht.path("/articles"), map[string]any{"locale": "en", "title": "Nope"})
	limited := ht.key("limited", map[string]any{"inbox_ids": []string{other}, "scopes": []string{"help:read", "help:write", "help:publish"}})
	limited.expectProblem(http.StatusNotFound, "not_found", "GET", ht.path(""), nil)
	limited.expect(http.StatusOK, "GET", "/v1/help-centers/"+otherCenter, nil)

	grant := h.client()
	grant.bearer = h.oauthToken(ht.owner, ht.ws, "Claude Code", testOrigin, []string{"help:read", "help:write"})
	g, _ := ht.article(grant, "en", "Written by an assistant", "Draft")
	grant.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", ht.path("/articles/"+g+"/translations/en/publish"), nil)
	if c := grant.expect(http.StatusOK, "GET", ht.path(""), nil); c.body["can_publish"] != false {
		t.Fatalf("OAuth token can_publish: %v", c.body["can_publish"])
	}
	grant.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", ht.path("/articles/"+art+"/translations/en/unpublish"), nil)
	grant.expectProblem(http.StatusForbidden, "member_session_required", "DELETE", ht.path("/articles/"+art), nil)
	grant.expect(http.StatusNoContent, "DELETE", ht.path("/articles/"+g), nil)
	for _, body := range []map[string]any{{"published": true}, {"name": "Renamed by an assistant"}, {"slug": unique("grant")}} {
		grant.expectProblem(http.StatusForbidden, "insufficient_scope", "PATCH", ht.path(""), body)
	}
	grant.expectProblem(http.StatusForbidden, "insufficient_scope", "DELETE", "/v1/help-centers/"+otherCenter, nil)
	third := ht.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Third", "slug": "third"}).body["inbox"].(map[string]any)["id"].(string)
	grant.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", "/v1/help-centers", map[string]any{"inbox_id": third, "name": "Third", "slug": unique("third")})
	if c := ht.owner.expect(http.StatusOK, "GET", ht.path(""), nil); c.str("name") != "Acme help" {
		t.Fatalf("an OAuth token renamed the center: %s", c.str("name"))
	}

	if n := len(ht.events("help.article.published")); n != 3 {
		t.Fatalf("published events: %d", n)
	}
	if n := len(ht.events("help.article.deleted")); n != 2 {
		t.Fatalf("deleted events: %d", n)
	}
	for _, e := range stranger.owner.expect(http.StatusOK, "GET", "/v1/events?after=0&limit=500", nil).body["events"].([]any) {
		if strings.HasPrefix(e.(map[string]any)["type"].(string), "help.") {
			t.Fatalf("another workspace sees %v", e)
		}
	}
	convKey := ht.key("conversations", map[string]any{"scopes": []string{"conversations:read"}})
	for _, e := range convKey.expect(http.StatusOK, "GET", "/v1/events?after=0&limit=500", nil).body["events"].([]any) {
		if strings.HasPrefix(e.(map[string]any)["type"].(string), "help.") {
			t.Fatalf("a key without help:read sees %v", e)
		}
	}
}

func TestHelpOff(t *testing.T) {
	t.Parallel()
	h := newHarnessWith(t, func(d *api.Deps) { d.Help.Off = true })
	tm := newTeam(t, h)
	tm.owner.expectProblem(http.StatusNotFound, "help_off", "GET", "/v1/help-centers", nil)
	tm.owner.expectProblem(http.StatusNotFound, "help_off", "POST", "/v1/help-centers", map[string]any{"inbox_id": tm.inbox, "name": "Off", "slug": unique("off")})
	h.client().expectProblem(http.StatusNotFound, "help_off", "GET", "/client/v1/channels/nokey/help", nil)
}

func TestHelpCenterSettings(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ht := newHelpTeam(t, h, "en", "tr")
	c := ht.owner.expect(http.StatusOK, "GET", ht.path(""), nil)
	slug := c.str("slug")
	if !strings.HasSuffix(c.str("url"), "/help/"+slug+"/") || c.body["published"] != false {
		t.Fatalf("center: %s", c.raw)
	}
	ht.owner.expectProblem(http.StatusBadRequest, "validation_failed", "PATCH", ht.path(""), map[string]any{"locales": []string{"tr"}})
	ht.owner.expect(http.StatusCreated, "POST", ht.path("/collections"), map[string]any{"translations": map[string]any{"tr": map[string]any{"name": "Fatura"}}})
	ht.owner.expectProblem(http.StatusConflict, "locale_in_use", "PATCH", ht.path(""), map[string]any{"locales": []string{"en"}})
	renamed := unique("renamed")
	ht.owner.expect(http.StatusOK, "PATCH", ht.path(""), map[string]any{"slug": renamed, "accent_color": "#112233", "site_url": "https://example.com"})
	other := newTeam(t, h)
	other.owner.expectProblem(http.StatusConflict, "slug_taken", "POST", "/v1/help-centers", map[string]any{"inbox_id": other.inbox, "name": "X", "slug": slug})
	other.owner.expectProblem(http.StatusConflict, "slug_taken", "POST", "/v1/help-centers", map[string]any{"inbox_id": other.inbox, "name": "X", "slug": renamed})
	ht.owner.expect(http.StatusOK, "PATCH", ht.path(""), map[string]any{"slug": slug})
	ht.owner.expectProblem(http.StatusBadRequest, "validation_failed", "PATCH", ht.path(""), map[string]any{"chat_channel_id": uuid.NewV7().String()})
	ht.owner.expectProblem(http.StatusBadRequest, "validation_failed", "PATCH", ht.path(""), map[string]any{"logo_url": "http://example.com/logo.png"})
	cleared := ht.owner.expect(http.StatusOK, "PATCH", ht.path(""), map[string]any{"accent_color": nil, "published": true})
	if _, ok := cleared.body["accent_color"]; ok || cleared.body["published"] != true {
		t.Fatalf("cleared: %s", cleared.raw)
	}

	top := ht.owner.expect(http.StatusCreated, "POST", ht.path("/collections"), map[string]any{
		"icon": "billing", "position": 1, "translations": map[string]any{"en": map[string]any{"name": "Payments & Invoices"}},
	})
	if tr := top.body["translations"].(map[string]any)["en"].(map[string]any); tr["slug"] != "payments-invoices" {
		t.Fatalf("collection slug: %v", tr["slug"])
	}
	sub := ht.owner.expect(http.StatusCreated, "POST", ht.path("/collections"), map[string]any{
		"parent_id": top.str("id"), "translations": map[string]any{"en": map[string]any{"name": "Refunds"}},
	})
	ht.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", ht.path("/collections"), map[string]any{
		"parent_id": sub.str("id"), "translations": map[string]any{"en": map[string]any{"name": "Too deep"}},
	})
	ht.owner.expectProblem(http.StatusBadRequest, "locale_not_offered", "POST", ht.path("/collections"), map[string]any{
		"translations": map[string]any{"de": map[string]any{"name": "Rechnungen"}},
	})
	ht.owner.expectProblem(http.StatusBadRequest, "validation_failed", "PATCH", ht.path("/collections/"+top.str("id")), map[string]any{"parent_id": sub.str("id")})
	ht.owner.expectProblem(http.StatusBadRequest, "validation_failed", "PATCH", ht.path("/collections/"+top.str("id")), map[string]any{"remove_locales": []string{"en"}})
	art := ht.owner.expect(http.StatusCreated, "POST", ht.path("/articles"), map[string]any{"locale": "en", "title": "Refund rules", "collection_id": sub.str("id"), "paths": []string{"/billing/*"}})
	ht.owner.expectProblem(http.StatusBadRequest, "validation_failed", "PATCH", ht.path("/articles/"+art.str("id")), map[string]any{"paths": []string{"/a*b"}})
	list := ht.owner.expect(http.StatusOK, "GET", ht.path("/collections"), nil).body["items"].([]any)
	if len(list) != 3 || list[1].(map[string]any)["id"] != top.str("id") || list[2].(map[string]any)["id"] != sub.str("id") || list[2].(map[string]any)["article_count"] != float64(1) {
		t.Fatalf("collections: %v", list)
	}
	ht.owner.expect(http.StatusNoContent, "DELETE", ht.path("/collections/"+sub.str("id")), nil)
	if a := ht.owner.expect(http.StatusOK, "GET", ht.path("/articles/"+art.str("id")), nil); a.body["collection_id"] != nil {
		t.Fatalf("article kept a deleted collection: %v", a.body["collection_id"])
	}
	ht.owner.expect(http.StatusNoContent, "DELETE", ht.path(""), nil)
	ht.owner.expectProblem(http.StatusNotFound, "not_found", "GET", ht.path(""), nil)
	other.owner.expect(http.StatusCreated, "POST", "/v1/help-centers", map[string]any{"inbox_id": other.inbox, "name": "X", "slug": renamed})
}

func TestHelpArticleVersionsAndRevisions(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ht := newHelpTeam(t, h, "en", "tr")
	o := ht.owner
	art, first := ht.article(o, "en", "Reset your password", "## Steps\n\nOpen **settings**.")
	if first["version"] != float64(1) || first["state"] != "draft" || first["slug"] != "reset-your-password" {
		t.Fatalf("first translation: %v", first)
	}
	tr := func(locale string) string { return ht.path("/articles/" + art + "/translations/" + locale) }
	saved := o.expect(http.StatusOK, "PUT", tr("en"), map[string]any{"base_version": 1, "title": "Reset your password", "body": "## Steps\n\nOpen **settings** and choose a new password."})
	if saved.body["version"] != float64(2) {
		t.Fatalf("saved version: %v", saved.body["version"])
	}
	stale := o.do("PUT", tr("en"), map[string]any{"base_version": 1, "title": "Lost update"})
	if stale.status != http.StatusConflict || stale.str("code") != "version_conflict" || stale.body["translation"].(map[string]any)["version"] != float64(2) {
		t.Fatalf("stale save: %d %s", stale.status, stale.raw)
	}
	o.expectProblem(http.StatusConflict, "version_conflict", "PUT", tr("en"), map[string]any{"base_version": 0, "title": "Again"})
	o.expectProblem(http.StatusBadRequest, "locale_not_offered", "PUT", tr("de"), map[string]any{"base_version": 0, "title": "Passwort"})
	o.expect(http.StatusCreated, "PUT", tr("tr"), map[string]any{"base_version": 0, "title": "Şifreni sıfırla"})

	revs := func(locale string) []any {
		return o.expect(http.StatusOK, "GET", tr(locale)+"/revisions", nil).body["items"].([]any)
	}
	if n := len(revs("en")); n != 1 {
		t.Fatalf("revisions after a quick second save: %d", n)
	}
	h.clock.Advance(11 * time.Minute)
	o.expect(http.StatusOK, "PUT", tr("en"), map[string]any{"base_version": 2, "title": "Reset your password", "body": "## Steps\n\nOpen settings."})
	if n := len(revs("en")); n != 2 {
		t.Fatalf("revisions after a later save: %d", n)
	}

	pub := ht.publish(o, art, "en")
	published := pub.body["published"].(map[string]any)
	if pub.body["state"] != "published" || !strings.Contains(published["html"].(string), `<h2 id="steps">Steps</h2>`) || published["excerpt"] != "Steps Open settings." {
		t.Fatalf("published: %s", pub.raw)
	}
	if !strings.HasSuffix(pub.str("url"), "/en/articles/1-reset-your-password") {
		t.Fatalf("url: %s", pub.str("url"))
	}
	ht.publish(o, art, "en")
	if n := len(revs("en")); n != 3 {
		t.Fatalf("revisions after publishing twice: %d", n)
	}
	o.expectProblem(http.StatusConflict, "version_conflict", "POST", tr("en")+"/publish", map[string]any{"base_version": 1})
	if n := len(ht.events("help.article.published")); n != 1 {
		t.Fatalf("an unchanged publish sent an event: %d", n)
	}
	a := o.expect(http.StatusOK, "GET", ht.path("/articles/"+art), nil).body["translations"].([]any)
	if a[0].(map[string]any)["locale"] != "en" || a[1].(map[string]any)["outdated"] != true || a[0].(map[string]any)["outdated"] != false {
		t.Fatalf("translations: %v", a)
	}

	changed := o.expect(http.StatusOK, "PUT", tr("en"), map[string]any{"base_version": 3, "title": "Reset a password", "body": "Changed"})
	if changed.body["state"] != "changed" || changed.body["published"].(map[string]any)["title"] != "Reset your password" {
		t.Fatalf("changed: %s", changed.raw)
	}
	byState := func(state string) int {
		return len(o.expect(http.StatusOK, "GET", ht.path("/articles?locale=en&state="+state), nil).body["items"].([]any))
	}
	if byState("changed") != 1 || byState("published") != 0 || byState("draft") != 0 || byState("missing") != 0 {
		t.Fatal("state filters")
	}
	oldest := revs("en")
	firstRev := oldest[len(oldest)-1].(map[string]any)
	o.expectProblem(http.StatusConflict, "version_conflict", "POST", tr("en")+"/revisions/"+firstRev["id"].(string)+"/restore", map[string]any{"base_version": 1})
	restored := o.expect(http.StatusOK, "POST", tr("en")+"/revisions/"+firstRev["id"].(string)+"/restore", map[string]any{"base_version": 4})
	if restored.str("title") != "Reset your password" || restored.str("body") != "## Steps\n\nOpen **settings**." || restored.body["version"] != float64(5) {
		t.Fatalf("restored: %s", restored.raw)
	}
	if revs("en")[0].(map[string]any)["kind"] != "restore" {
		t.Fatal("restore kept no revision")
	}

	version := 5.0
	for i := range 55 {
		h.clock.Advance(11 * time.Minute)
		r := o.expect(http.StatusOK, "PUT", tr("en"), map[string]any{"base_version": version, "title": "Reset your password", "body": "Edit " + string(rune('a'+i%26))})
		version = r.body["version"].(float64)
	}
	all := revs("en")
	kinds := map[string]int{}
	for _, r := range all {
		kinds[r.(map[string]any)["kind"].(string)]++
	}
	if len(all) != 51 || kinds["publish"] != 1 {
		t.Fatalf("revisions kept: %d %v", len(all), kinds)
	}

	un := o.expect(http.StatusOK, "POST", tr("en")+"/unpublish", nil)
	if un.body["state"] != "draft" || un.body["published"] != nil || un.body["url"] != nil {
		t.Fatalf("unpublished: %s", un.raw)
	}
	o.expect(http.StatusOK, "POST", tr("en")+"/unpublish", nil)
	if n := len(ht.events("help.article.unpublished")); n != 1 {
		t.Fatalf("unpublished events: %d", n)
	}
	o.expect(http.StatusNoContent, "DELETE", tr("tr"), nil)
	o.expectProblem(http.StatusConflict, "last_translation", "DELETE", tr("en"), nil)
	o.expect(http.StatusNoContent, "DELETE", ht.path("/articles/"+art), nil)
	if n := h.count("SELECT count(*) FROM help_article_revisions WHERE article_id = $1", art); n != 0 {
		t.Fatalf("revisions left: %d", n)
	}
}

func TestHelpSearch(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ht := newHelpTeam(t, h, "en", "tr", "pl")
	o := ht.owner
	add := func(locale, title, body string) string {
		id, _ := ht.article(o, locale, title, body)
		ht.publish(o, id, locale)
		return id
	}
	reset := add("en", "Reset your password", "Open the settings and choose a new password.")
	billing := add("en", "Billing and invoices", "Download every invoice as a PDF.")
	add("en", "Change the e-mail address", "Settings, then Account.")
	refund := add("tr", "İade nasıl yapılır", "Siparişler sayfasından iade isteyin.")
	sifre := add("tr", "Şifre sıfırlama", "Ayarlar sayfasını açın.")
	haslo := add("pl", "Zmiana hasła", "Otwórz ustawienia konta.")
	draft, _ := ht.article(o, "en", "Unreleased password manager", "Not yet")

	search := func(q string, extra string) response {
		return o.expect(http.StatusOK, "GET", ht.path("/search?q="+url.QueryEscape(q)+extra), nil)
	}
	ids := func(r response) []string {
		var out []string
		for _, it := range r.body["items"].([]any) {
			out = append(out, it.(map[string]any)["article_id"].(string))
		}
		return out
	}
	expect := func(q, extra string, want ...string) response {
		t.Helper()
		r := search(q, extra)
		got := ids(r)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("search %q%s: got %v want %v (%s)", q, extra, got, want, r.raw)
		}
		return r
	}
	r := expect("password", "", reset)
	hl := r.body["items"].([]any)[0].(map[string]any)["highlights"].([]any)[0].(map[string]any)
	if hl["field"] != "title" || hl["start"] != float64(11) || hl["end"] != float64(19) || r.body["partial"] != false {
		t.Fatalf("highlight: %s", r.raw)
	}
	expect("passwords", "", reset)
	expect("reset passw", "", reset)
	expect("invoice", "", billing)
	pr := expect("password invoices", "", reset, billing)
	if pr.body["partial"] != true {
		t.Fatal("an OR fallback is not marked partial")
	}
	expect("password", "&drafts=true", reset, draft)
	expect("İADE", "&locale=tr", refund)
	expect("iade", "&locale=tr", refund)
	expect("SIFIR", "&locale=tr", sifre)
	expect("sıfırlama", "&locale=tr", sifre)
	expect("invoices", "&locale=tr", billing)
	expect("hasła", "&locale=pl", haslo)
	expect("has", "&locale=pl", haslo)
	expect("nothing matches this", "")
	o.expectProblem(http.StatusBadRequest, "query_empty", "GET", ht.path("/search?q=%21%21%21"), nil)
	o.expectProblem(http.StatusBadRequest, "locale_not_offered", "GET", ht.path("/search?q=x&locale=de"), nil)
	if n := h.count("SELECT count(*) FROM help_search_daily WHERE center_id = $1", ht.center); n != 0 {
		t.Fatalf("a member's search was counted: %d", n)
	}
	if titles := o.expect(http.StatusOK, "GET", ht.path("/articles?q=passw"), nil).body["items"].([]any); len(titles) != 0 {
		t.Fatalf("title filter matched a partial word: %v", titles)
	}
	if titles := o.expect(http.StatusOK, "GET", ht.path("/articles?q=password"), nil).body["items"].([]any); len(titles) != 2 {
		t.Fatalf("title filter: %v", titles)
	}
}

func TestHelpImagesAndRender(t *testing.T) {
	h := newHarness(t)
	ht := newHelpTeam(t, h)
	o := ht.owner
	ht.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+ht.inbox+"/members/"+ht.agentID, nil)
	up := ht.agent.uploadImageTo(ht.path("/images"), "screen shot.png", "image/png", testPNG(t, 40, 20, color.RGBA{R: 200, A: 255}))
	if up.status != http.StatusCreated || !strings.HasPrefix(up.str("markdown"), "![screen shot](") || up.body["width"] != float64(40) {
		t.Fatalf("upload: %d %s", up.status, up.raw)
	}
	again := ht.agent.uploadImageTo(ht.path("/images"), "again.png", "image/png", testPNG(t, 40, 20, color.RGBA{R: 200, A: 255}))
	if again.status != http.StatusOK || again.str("id") != up.str("id") {
		t.Fatalf("same image: %d %s", again.status, again.raw)
	}
	unused := o.uploadImageTo(ht.path("/images"), "unused.png", "image/png", testPNG(t, 10, 10, color.RGBA{B: 200, A: 255}))
	other := newTeam(t, h)
	foreign := other.owner.uploadImage("foreign.png", "image/png", testPNG(t, 12, 12, color.RGBA{G: 200, A: 255}))

	body := "# Title\n\n" + up.str("markdown") + "\n\n![outside](https://evil.example/x.png)\n\n![theirs](" + foreign.str("url") + ")\n\n<script>alert(1)</script>\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n~~old~~ https://example.com"
	html := o.expect(http.StatusOK, "POST", ht.path("/render"), map[string]any{"body": body}).str("html")
	for _, want := range []string{`<h1 id="title">`, `<img src="` + up.str("url") + `" alt="screen shot" loading="lazy"`, `<a href="https://evil.example/x.png"`, `<a href="` + foreign.str("url") + `"`, "<table>", "<del>old</del>", `<a href="https://example.com" target="_blank" rel="noopener">`} {
		if !strings.Contains(html, want) {
			t.Fatalf("render lacks %s:\n%s", want, html)
		}
	}
	if strings.Contains(html, "<script") || strings.Contains(html, "alert(1)</") || strings.Contains(html, `<img src="https://evil`) {
		t.Fatalf("render kept unsafe markup:\n%s", html)
	}
	art, _ := ht.article(ht.agent, "en", "With a picture", body)
	for _, force := range []string{"", "?force=true"} {
		r := o.do("DELETE", "/v1/email-images/"+up.str("id")+force, nil)
		used, _ := r.body["used_by"].(map[string]any)
		if r.status != http.StatusConflict || r.str("code") != "image_in_use" || used == nil || len(used["help_articles"].([]any)) != 1 {
			t.Fatalf("deleting an image a draft uses: %d %s", r.status, r.raw)
		}
	}
	ht.agent.expect(http.StatusOK, "PUT", ht.path("/articles/"+art+"/translations/en"), map[string]any{"base_version": 1, "title": "With a picture", "body": "No picture now"})
	if n := h.count("SELECT count(*) FROM help_article_translations WHERE article_id = $1 AND image_ids @> ARRAY[$2::uuid]", art, up.str("id")); n != 0 {
		t.Fatal("the draft still lists the image it dropped")
	}
	o.expectProblem(http.StatusConflict, "image_in_use", "DELETE", "/v1/email-images/"+up.str("id"), nil)

	h.clock.Advance(8 * 24 * time.Hour)
	if err := h.srv.CleanWorkspaceImages(context.Background(), uuid.MustParse(ht.ws)); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM email_images WHERE id = $1", up.str("id")); n != 1 {
		t.Fatal("the sweep deleted an image a kept revision shows")
	}
	if n := h.count("SELECT count(*) FROM email_images WHERE id = $1", unused.str("id")); n != 0 {
		t.Fatal("the sweep kept an unused image")
	}
	o.expect(http.StatusNoContent, "DELETE", ht.path("/articles/"+art), nil)
	if err := h.srv.CleanWorkspaceImages(context.Background(), uuid.MustParse(ht.ws)); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM email_images WHERE id = $1", up.str("id")); n != 0 {
		t.Fatal("the sweep kept the image of a deleted article")
	}

	pv := o.do("GET", ht.path("/preview"), nil)
	if pv.status != http.StatusOK || pv.header.Get("Cache-Control") != "no-store" || !strings.Contains(string(pv.raw), "Acme help") {
		t.Fatalf("preview: %d %s", pv.status, pv.raw)
	}
}

func TestHelpCountersAndReport(t *testing.T) {
	h := newHarness(t)
	ht := newHelpTeam(t, h, "en", "tr")
	o := ht.owner
	art, _ := ht.article(o, "en", "Reset your password", "Body")
	ht.publish(o, art, "en")
	gone, _ := ht.article(o, "en", "Soon deleted", "Body")
	ws, center := uuid.MustParse(ht.ws), uuid.MustParse(ht.center)
	h.srv.CountHelpView(ws, center, uuid.MustParse(art), "en")
	h.srv.CountHelpView(ws, center, uuid.MustParse(art), "en")
	h.srv.CountHelpView(ws, center, uuid.MustParse(gone), "en")
	h.srv.CountHelpSearch(ws, center, "tr", "  Fatura  İPTAL ", false)
	h.srv.CountHelpSearch(ws, center, "tr", "fatura iptal", false)
	h.srv.CountHelpSearch(ws, center, "en", "me@example.com", false)
	h.srv.CountHelpSearch(ws, center, "en", "12345 678", false)
	h.srv.CountHelpSearch(ws, center, "en", "password", true)
	o.expect(http.StatusNoContent, "DELETE", ht.path("/articles/"+gone), nil)
	if err := h.srv.FlushHelpCounters(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := h.srv.FlushHelpCounters(context.Background()); err != nil {
		t.Fatal(err)
	}
	rep := o.expect(http.StatusOK, "GET", ht.path("/report?days=7"), nil)
	top := rep.body["top_articles"].([]any)
	if rep.body["views"] != float64(2) || rep.body["searches"] != float64(5) || rep.body["no_result_searches"] != float64(4) || len(top) != 1 || top[0].(map[string]any)["title"] != "Reset your password" {
		t.Fatalf("report: %s", rep.raw)
	}
	q := rep.body["no_result_queries"].([]any)
	if len(q) != 2 || q[0].(map[string]any)["query"] != "(scrubbed)" || q[0].(map[string]any)["searches"] != float64(2) || q[1].(map[string]any)["query"] != "fatura iptal" || q[1].(map[string]any)["searches"] != float64(2) {
		t.Fatalf("no-result queries: %v", q)
	}
	if n := h.count("SELECT coalesce(sum(help_searches), 0)::int FROM usage_counters WHERE workspace_id = $1", ht.ws); n != 5 {
		t.Fatalf("usage help_searches: %d", n)
	}
	o.expectProblem(http.StatusBadRequest, "validation_failed", "GET", ht.path("/report?days=5"), nil)

	h.clock.Advance(91 * 24 * time.Hour)
	if err := h.srv.CleanHelp(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM help_search_daily WHERE workspace_id = $1", ht.ws); n != 0 {
		t.Fatalf("search counters after 91 days: %d", n)
	}
	if n := h.count("SELECT count(*) FROM help_article_daily WHERE workspace_id = $1", ht.ws); n != 1 {
		t.Fatalf("article counters after 91 days: %d", n)
	}
}

func TestHelpSearchQueryCap(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ht := newHelpTeam(t, h)
	ws, center := uuid.MustParse(ht.ws), uuid.MustParse(ht.center)
	for i := range 1999 {
		h.srv.CountHelpSearch(ws, center, "en", "query "+strconv.Itoa(i), false)
	}
	if err := h.srv.FlushHelpCounters(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"query 1", "fresh one", "fresh two", "fresh three"} {
		h.srv.CountHelpSearch(ws, center, "en", q, false)
	}
	if err := h.srv.FlushHelpCounters(context.Background()); err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		h.srv.CountHelpSearch(ws, center, "en", "buffered "+strconv.Itoa(i), false)
	}
	if err := h.srv.FlushHelpCounters(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM help_search_daily WHERE center_id = $1", ht.center); n != 2001 {
		t.Fatalf("distinct queries kept: %d", n)
	}
	if n := h.count("SELECT searches FROM help_search_daily WHERE center_id = $1 AND query = '(other)'", ht.center); n != 5 {
		t.Fatalf("searches under (other): %d", n)
	}
	if n := h.count("SELECT searches FROM help_search_daily WHERE center_id = $1 AND query = 'query 1'", ht.center); n != 2 {
		t.Fatalf("a known query past the cap: %d", n)
	}
}

func TestHelpFormerSlugs(t *testing.T) {
	h := newHarness(t)
	ht := newHelpTeam(t, h)
	first := ht.owner.expect(http.StatusOK, "GET", ht.path(""), nil).str("slug")
	slugs := []string{first}
	for range 10 {
		h.clock.Advance(time.Minute)
		next := unique("slug")
		ht.owner.expect(http.StatusOK, "PATCH", ht.path(""), map[string]any{"slug": next})
		slugs = append(slugs, next)
	}
	ht.owner.expectProblem(http.StatusTooManyRequests, "rate_limited", "PATCH", ht.path(""), map[string]any{"slug": unique("slug")})
	if n := h.count("SELECT count(*) FROM help_center_slugs WHERE center_id = $1", ht.center); n != 10 {
		t.Fatalf("former slugs: %d", n)
	}
	h.clock.Advance(25 * time.Hour)
	ht.owner.expect(http.StatusOK, "PATCH", ht.path(""), map[string]any{"slug": unique("slug")})
	if n := h.count("SELECT count(*) FROM help_center_slugs WHERE center_id = $1", ht.center); n != 10 {
		t.Fatalf("former slugs after the eleventh change: %d", n)
	}
	other := newTeam(t, h)
	other.owner.expectProblem(http.StatusConflict, "slug_taken", "POST", "/v1/help-centers", map[string]any{"inbox_id": other.inbox, "name": "X", "slug": slugs[1]})
	took := other.owner.expect(http.StatusCreated, "POST", "/v1/help-centers", map[string]any{"inbox_id": other.inbox, "name": "X", "slug": slugs[0]})
	key := h.client()
	key.bearer = other.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "slugs", "scopes": []string{"help:read", "help:write", "help:publish"}}).str("secret")

	h.clock.Advance(91 * 24 * time.Hour)
	if err := h.srv.CleanHelp(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM help_center_slugs WHERE center_id = $1", ht.center); n != 0 {
		t.Fatalf("former slugs after 91 days: %d", n)
	}
	key.expect(http.StatusOK, "PATCH", "/v1/help-centers/"+took.str("id"), map[string]any{"slug": slugs[1]})
}

func TestHelpConcurrentWrites(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ht := newHelpTeam(t, h)
	o := ht.owner
	var arts []string
	cols := make([]string, 4)
	for i := range cols {
		cols[i] = o.expect(http.StatusCreated, "POST", ht.path("/collections"), map[string]any{"translations": map[string]any{"en": map[string]any{"name": "Group " + strconv.Itoa(i)}}}).str("id")
		for j := range 3 {
			id := o.expect(http.StatusCreated, "POST", ht.path("/articles"), map[string]any{"locale": "en", "title": "Article " + strconv.Itoa(i*3+j), "collection_id": cols[i]}).str("id")
			ht.publish(o, id, "en")
			arts = append(arts, id)
		}
	}
	var wg sync.WaitGroup
	statuses := make(chan int, 256)
	run := func(method, path string, body any) {
		defer wg.Done()
		statuses <- o.do(method, path, body).status
	}
	for i, a := range arts {
		tr := ht.path("/articles/" + a + "/translations/en")
		wg.Add(3)
		go run("POST", tr+"/unpublish", nil)
		go run("PATCH", ht.path("/articles/"+a), map[string]any{"position": i})
		if i%3 == 0 {
			go run("DELETE", ht.path("/articles/"+a), nil)
		} else {
			go run("POST", tr+"/publish", nil)
		}
	}
	for _, c := range cols {
		wg.Add(1)
		go run("DELETE", ht.path("/collections/"+c), nil)
	}
	wg.Add(1)
	go run("PATCH", ht.path(""), map[string]any{"description": "Busy"})
	wg.Wait()
	close(statuses)
	for st := range statuses {
		if st >= 500 {
			t.Fatalf("a concurrent write answered %d", st)
		}
	}
	o.expect(http.StatusNoContent, "DELETE", ht.path(""), nil)
}

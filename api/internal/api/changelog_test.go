package api_test

import (
	"context"
	"encoding/json"
	"image/color"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/productdevbook/yuva/api/internal/api"
	"github.com/productdevbook/yuva/api/internal/store"
)

type changelogTeam struct {
	listTeam
	changelog string
	slug      string
}

func newChangelogTeam(t *testing.T, h *harness) changelogTeam {
	t.Helper()
	lt := newListTeam(t, h)
	slug := unique("news")
	c := lt.owner.expect(http.StatusCreated, "POST", "/v1/changelogs", map[string]any{
		"inbox_id": lt.chatInbox, "name": "Acme news", "slug": slug, "default_locale": "en", "locales": []string{"en", "tr"}, "published": true,
	})
	return changelogTeam{listTeam: lt, changelog: c.str("id"), slug: slug}
}

func (ct changelogTeam) path(p string) string { return "/v1/changelogs/" + ct.changelog + p }

func (ct changelogTeam) entry(c *client, locale, title, body string, labels ...string) string {
	ct.h().t.Helper()
	if len(labels) == 0 {
		labels = []string{"new"}
	}
	return c.expect(http.StatusCreated, "POST", ct.path("/entries"), map[string]any{"locale": locale, "title": title, "body": body, "labels": labels}).str("id")
}

func (ct changelogTeam) h() *harness { return ct.owner.h }

func (ct changelogTeam) publish(c *client, entry string) response {
	ct.h().t.Helper()
	return c.expect(http.StatusOK, "POST", ct.path("/entries/"+entry+"/publish"), nil)
}

func (ct changelogTeam) get(c *client, entry string) response {
	ct.h().t.Helper()
	return c.expect(http.StatusOK, "GET", ct.path("/entries/"+entry), nil)
}

func (ct changelogTeam) apiKey(name string, body map[string]any) *client {
	ct.h().t.Helper()
	body["name"] = name
	k := ct.h().client()
	k.bearer = ct.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", body).str("secret")
	return k
}

func (ct changelogTeam) events(typ string) []map[string]any {
	ct.h().t.Helper()
	var out []map[string]any
	for _, e := range ct.owner.expect(http.StatusOK, "GET", "/v1/events?after=0&limit=500", nil).body["events"].([]any) {
		if m := e.(map[string]any); m["type"] == typ {
			out = append(out, m)
		}
	}
	return out
}

func (ct changelogTeam) public() *client {
	c := ct.h().client()
	c.origin = ct.origin
	return c
}

func (ct changelogTeam) feedPath(q string) string {
	return "/client/v1/channels/" + ct.key + "/whats-new" + q
}

func TestChangelogRolesAndScopes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChangelogTeam(t, h)
	o, agent := ct.owner, ct.agent

	o.expectProblem(http.StatusConflict, "changelog_exists", "POST", "/v1/changelogs", map[string]any{"inbox_id": ct.chatInbox, "name": "Again", "slug": unique("again")})
	o.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/changelogs", map[string]any{"inbox_id": ct.inbox, "name": "Reserved", "slug": "help"})

	if c := agent.expect(http.StatusOK, "GET", ct.path(""), nil); c.body["can_publish"] != false || !strings.HasSuffix(c.str("url"), "/changelog/"+ct.slug) {
		t.Fatalf("agent's view: %s", c.raw)
	}
	agent.expectProblem(http.StatusForbidden, "forbidden", "PATCH", ct.path(""), map[string]any{"name": "Mine"})
	agent.expectProblem(http.StatusForbidden, "forbidden", "DELETE", ct.path(""), nil)
	draft := ct.entry(agent, "en", "Agent wrote this", "Draft text")
	agent.expectProblem(http.StatusForbidden, "forbidden", "POST", ct.path("/entries/"+draft+"/publish"), nil)
	agent.expectProblem(http.StatusForbidden, "forbidden", "POST", ct.path("/entries/"+draft+"/schedule"), map[string]any{"publish_at": h.clock.Now().Add(time.Hour).Format(time.RFC3339)})

	o.expect(http.StatusOK, "PATCH", ct.path(""), map[string]any{"agents_may_publish": true})
	if c := agent.expect(http.StatusOK, "GET", ct.path(""), nil); c.body["can_publish"] != true {
		t.Fatalf("agent can_publish: %s", c.raw)
	}
	if e := ct.publish(agent, draft); e.str("state") != "published" || e.str("published_at") == "" {
		t.Fatalf("published by an agent: %s", e.raw)
	}
	agent.expect(http.StatusOK, "POST", ct.path("/entries/"+draft+"/unpublish"), nil)
	ct.publish(agent, draft)
	o.expect(http.StatusOK, "PATCH", ct.path(""), map[string]any{"agents_may_publish": false})
	agent.expectProblem(http.StatusForbidden, "forbidden", "DELETE", ct.path("/entries/"+draft), nil)
	agent.expectProblem(http.StatusForbidden, "forbidden", "PATCH", ct.path("/entries/"+draft), map[string]any{"labels": []string{"fixed"}})
	agent.expectProblem(http.StatusForbidden, "forbidden", "DELETE", ct.path("/entries/"+draft+"/translations/en"), nil)

	other := o.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Other", "slug": "other"}).body["inbox"].(map[string]any)["id"].(string)
	otherLog := o.expect(http.StatusCreated, "POST", "/v1/changelogs", map[string]any{"inbox_id": other, "name": "Other", "slug": unique("other")}).str("id")
	agent.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/changelogs/"+otherLog, nil)
	agent.expectProblem(http.StatusNotFound, "not_found", "POST", "/v1/changelogs/"+otherLog+"/entries", map[string]any{"locale": "en", "title": "Hidden", "labels": []string{"new"}})
	if items := agent.expect(http.StatusOK, "GET", "/v1/changelogs", nil).body["items"].([]any); len(items) != 1 {
		t.Fatalf("agent sees %d changelogs", len(items))
	}

	stranger := newTeam(t, h)
	stranger.owner.expectProblem(http.StatusNotFound, "not_found", "GET", ct.path(""), nil)
	stranger.owner.expectProblem(http.StatusNotFound, "not_found", "GET", ct.path("/entries/"+draft), nil)
	stranger.owner.expectProblem(http.StatusNotFound, "not_found", "POST", ct.path("/entries/"+draft+"/publish"), nil)
	stranger.owner.expectProblem(http.StatusNotFound, "not_found", "POST", ct.path("/entries/"+draft+"/broadcast"), map[string]any{"locale": "en", "list_ids": []string{ct.list}})
	strangerKey := h.client()
	strangerKey.bearer = stranger.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "k", "scopes": []string{"changelog:read", "changelog:write", "changelog:publish"}}).str("secret")
	strangerKey.expectProblem(http.StatusNotFound, "not_found", "GET", ct.path(""), nil)
	strangerKey.expectProblem(http.StatusNotFound, "not_found", "POST", ct.path("/entries/"+draft+"/unpublish"), nil)
	strangerKey.expectProblem(http.StatusNotFound, "not_found", "PUT", ct.path("/entries/"+draft+"/translations/en"), map[string]any{"base_version": 1, "title": "Taken over"})

	full := ct.apiKey("default", map[string]any{})
	full.expect(http.StatusOK, "GET", ct.path("/entries/"+draft), nil)
	fromKey := ct.entry(full, "en", "Key draft", "")
	full.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", ct.path("/entries/"+fromKey+"/publish"), nil)
	full.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", ct.path("/entries/"+fromKey+"/schedule"), map[string]any{"publish_at": h.clock.Now().Add(time.Hour).Format(time.RFC3339)})
	full.expectProblem(http.StatusForbidden, "insufficient_scope", "PATCH", ct.path(""), map[string]any{"name": "Key"})
	publisher := ct.apiKey("publisher", map[string]any{"scopes": []string{"changelog:read", "changelog:write", "changelog:publish"}})
	if e := ct.publish(publisher, fromKey); e.str("state") != "published" {
		t.Fatalf("key published: %s", e.raw)
	}
	reader := ct.apiKey("reader", map[string]any{"scopes": []string{"changelog:read"}})
	reader.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", ct.path("/entries"), map[string]any{"locale": "en", "title": "Nope", "labels": []string{"new"}})
	limited := ct.apiKey("limited", map[string]any{"inbox_ids": []string{other}, "scopes": []string{"changelog:read", "changelog:write", "changelog:publish"}})
	limited.expectProblem(http.StatusNotFound, "not_found", "GET", ct.path(""), nil)
	limited.expect(http.StatusOK, "GET", "/v1/changelogs/"+otherLog, nil)

	grant := h.client()
	grant.bearer = h.oauthToken(o, ct.ws, "Claude Code", testOrigin, []string{"changelog:read", "changelog:write"})
	g := ct.entry(grant, "en", "Written by an assistant", "Draft")
	if c := grant.expect(http.StatusOK, "GET", ct.path(""), nil); c.body["can_publish"] != false {
		t.Fatalf("OAuth token can_publish: %s", c.raw)
	}
	o.expect(http.StatusOK, "PATCH", ct.path(""), map[string]any{"agents_may_publish": true})
	grant.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", ct.path("/entries/"+g+"/publish"), nil)
	grant.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", ct.path("/entries/"+g+"/schedule"), map[string]any{"publish_at": h.clock.Now().Add(time.Hour).Format(time.RFC3339)})
	grant.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", ct.path("/entries/"+draft+"/unpublish"), nil)
	grant.expectProblem(http.StatusForbidden, "member_session_required", "DELETE", ct.path("/entries/"+draft), nil)
	for _, body := range []map[string]any{{"published": false}, {"name": "Renamed"}, {"slug": unique("grant")}} {
		grant.expectProblem(http.StatusForbidden, "insufficient_scope", "PATCH", ct.path(""), body)
	}
	grant.expectProblem(http.StatusForbidden, "insufficient_scope", "DELETE", "/v1/changelogs/"+otherLog, nil)
	if ent := ct.get(o, draft); ent.str("state") != "published" {
		t.Fatalf("an OAuth token changed a published entry: %s", ent.raw)
	}
	if c := o.expect(http.StatusOK, "GET", ct.path(""), nil); c.str("name") != "Acme news" || c.body["published"] != true {
		t.Fatalf("an OAuth token changed the changelog: %s", c.raw)
	}
	grant.expect(http.StatusNoContent, "DELETE", ct.path("/entries/"+g), nil)

	if n := len(ct.events("changelog.entry.published")); n != 3 {
		t.Fatalf("published events: %d", n)
	}
	if n := len(ct.events("changelog.entry.unpublished")); n != 1 {
		t.Fatalf("unpublished events: %d", n)
	}
	for _, e := range stranger.owner.expect(http.StatusOK, "GET", "/v1/events?after=0&limit=500", nil).body["events"].([]any) {
		if strings.HasPrefix(e.(map[string]any)["type"].(string), "changelog.") {
			t.Fatalf("another workspace sees %v", e)
		}
	}
	convKey := ct.apiKey("conversations", map[string]any{"scopes": []string{"conversations:read"}})
	for _, e := range convKey.expect(http.StatusOK, "GET", "/v1/events?after=0&limit=500", nil).body["events"].([]any) {
		if strings.HasPrefix(e.(map[string]any)["type"].(string), "changelog.") {
			t.Fatalf("a key without changelog:read sees %v", e)
		}
	}
	data := ct.events("changelog.entry.published")[0]["data"].(map[string]any)
	if data["changelog"].(map[string]any)["id"] != ct.changelog || data["entry"].(map[string]any)["translations"].([]any)[0].(map[string]any)["title"] != "Agent wrote this" {
		t.Fatalf("event data: %v", data)
	}
}

func TestChangelogOff(t *testing.T) {
	t.Parallel()
	h := newHarnessWith(t, func(d *api.Deps) { d.Changelog.Off = true })
	tm := newTeam(t, h)
	tm.owner.expectProblem(http.StatusNotFound, "whats_new_off", "GET", "/v1/changelogs", nil)
	tm.owner.expectProblem(http.StatusNotFound, "whats_new_off", "POST", "/v1/changelogs", map[string]any{"inbox_id": tm.inbox, "name": "Off", "slug": unique("off")})
	h.client().expectProblem(http.StatusNotFound, "whats_new_off", "GET", "/client/v1/channels/nokey/whats-new", nil)
}

func TestChangelogEntryLimit(t *testing.T) {
	t.Parallel()
	h := newHarnessWith(t, func(d *api.Deps) { d.Changelog.MaxEntries = 2 })
	ct := newChangelogTeam(t, h)
	ct.entry(ct.owner, "en", "First entry", "")
	second := ct.entry(ct.owner, "en", "Second entry", "")
	r := ct.owner.expect(http.StatusUnprocessableEntity, "POST", ct.path("/entries"), map[string]any{"locale": "en", "title": "Third entry", "labels": []string{"new"}})
	if r.str("code") != "hosted_limit" || r.str("limit") != "YUVA_CHANGELOG_MAX_ENTRIES" {
		t.Fatalf("limit: %s", r.raw)
	}
	ct.owner.expect(http.StatusNoContent, "DELETE", ct.path("/entries/"+second), nil)
	ct.entry(ct.owner, "en", "Third entry", "")
}

func TestChangelogTranslationsAndPublishing(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChangelogTeam(t, h)
	o := ct.owner
	id := ct.entry(o, "en", "Faster search", "Results appear **as you type**.", "improved", "fixed")
	tr := func(l string) string { return ct.path("/entries/" + id + "/translations/" + l) }

	e := ct.get(o, id)
	first := e.body["translations"].([]any)[0].(map[string]any)
	if e.str("state") != "draft" || e.body["number"] != float64(1) || first["version"] != float64(1) || first["slug"] != "faster-search" || first["state"] != "draft" {
		t.Fatalf("new entry: %s", e.raw)
	}
	o.expectProblem(http.StatusBadRequest, "locale_not_offered", "PUT", tr("de"), map[string]any{"base_version": 0, "title": "Schnellere Suche"})
	o.expectProblem(http.StatusConflict, "not_published", "POST", ct.path("/entries/"+id+"/publish"), map[string]any{"locale": "en"})
	stale := o.do("PUT", tr("en"), map[string]any{"base_version": 7, "title": "Lost update"})
	if stale.status != http.StatusConflict || stale.str("code") != "version_conflict" || stale.body["translation"].(map[string]any)["version"] != float64(1) {
		t.Fatalf("stale save: %d %s", stale.status, stale.raw)
	}
	saved := o.expect(http.StatusOK, "PUT", tr("en"), map[string]any{"base_version": 1, "title": "Faster search", "body": "Results appear **as you type**, even offline."})
	if saved.body["version"] != float64(2) {
		t.Fatalf("saved: %s", saved.raw)
	}
	o.expect(http.StatusCreated, "PUT", tr("tr"), map[string]any{"base_version": 0, "title": "Daha hızlı arama", "body": "Sonuçlar **yazarken** çıkar."})

	only := ct.entry(o, "tr", "Yalnız Türkçe", "")
	o.expectProblem(http.StatusConflict, "no_default_translation", "POST", ct.path("/entries/"+only+"/publish"), nil)
	o.expectProblem(http.StatusConflict, "no_default_translation", "POST", ct.path("/entries/"+only+"/schedule"), map[string]any{"publish_at": h.clock.Now().Add(time.Hour).Format(time.RFC3339)})
	o.expectProblem(http.StatusConflict, "last_translation", "DELETE", ct.path("/entries/"+only+"/translations/tr"), nil)

	pub := ct.publish(o, id)
	date := pub.str("published_at")
	trs := pub.body["translations"].([]any)
	en, trk := trs[0].(map[string]any), trs[1].(map[string]any)
	if pub.str("state") != "published" || date == "" || en["state"] != "published" || trk["state"] != "published" {
		t.Fatalf("published: %s", pub.raw)
	}
	html := en["published"].(map[string]any)["html"].(string)
	if !strings.Contains(html, "<strong>as you type</strong>") || en["published"].(map[string]any)["excerpt"] != "Results appear as you type, even offline." {
		t.Fatalf("published copy: %v", en["published"])
	}
	if !strings.HasSuffix(en["url"].(string), "/changelog/"+ct.slug+"/en/entries/1-faster-search") {
		t.Fatalf("url: %v", en["url"])
	}
	if n := len(ct.events("changelog.entry.published")); n != 1 {
		t.Fatalf("published events: %d", n)
	}

	h.clock.Advance(time.Hour)
	o.expect(http.StatusOK, "PUT", tr("en"), map[string]any{"base_version": 2, "title": "Faster search", "body": "New text."})
	changed := ct.get(o, id).body["translations"].([]any)[0].(map[string]any)
	if changed["state"] != "changed" || changed["published"].(map[string]any)["body"] != "Results appear **as you type**, even offline." {
		t.Fatalf("changed: %v", changed)
	}
	if pubTr := o.expect(http.StatusOK, "GET", ct.path("/entries?state=changed"), nil).body["items"].([]any); len(pubTr) != 1 {
		t.Fatalf("changed filter: %v", pubTr)
	}
	if got := o.expect(http.StatusOK, "GET", ct.path("/entries?state=published"), nil).body["items"].([]any); len(got) != 1 {
		t.Fatalf("published filter: %v", got)
	}
	if got := o.expect(http.StatusOK, "GET", ct.path("/entries?label=fixed&locale=tr"), nil).body["items"].([]any); len(got) != 1 {
		t.Fatalf("label filter: %v", got)
	}
	if got := o.expect(http.StatusOK, "GET", ct.path("/entries?label=new"), nil).body["items"].([]any); len(got) != 1 {
		t.Fatalf("label new matches only the other entry: %v", got)
	}
	o.expectProblem(http.StatusConflict, "version_conflict", "POST", ct.path("/entries/"+id+"/publish"), map[string]any{"locale": "en", "base_version": 1})
	byLocale := o.expect(http.StatusOK, "POST", ct.path("/entries/"+id+"/publish"), map[string]any{"locale": "en", "base_version": 3})
	if byLocale.body["translations"].([]any)[0].(map[string]any)["state"] != "published" || byLocale.str("published_at") != date {
		t.Fatalf("one translation: %s", byLocale.raw)
	}
	again := ct.publish(o, id)
	if again.str("published_at") != date {
		t.Fatalf("republishing moved the date: %s != %s", again.str("published_at"), date)
	}
	o.expectProblem(http.StatusConflict, "default_translation", "DELETE", tr("en"), nil)
	o.expect(http.StatusNoContent, "DELETE", tr("tr"), nil)

	un := o.expect(http.StatusOK, "POST", ct.path("/entries/"+id+"/unpublish"), nil)
	if un.str("state") != "draft" || un.str("published_at") != date || un.body["translations"].([]any)[0].(map[string]any)["published"] != nil {
		t.Fatalf("unpublished: %s", un.raw)
	}
	if n := len(ct.events("changelog.entry.unpublished")); n != 1 {
		t.Fatalf("unpublished events: %d", n)
	}
	if got := ct.public().do("GET", ct.feedPath(""), nil).body["items"]; len(got.([]any)) != 0 {
		t.Fatalf("an unpublished entry is in the feed: %v", got)
	}
	repub := ct.publish(o, id)
	if repub.str("published_at") != date {
		t.Fatalf("the date after unpublishing: %s", repub.str("published_at"))
	}
	o.expect(http.StatusNoContent, "DELETE", ct.path("/entries/"+id), nil)
	if n := len(ct.events("changelog.entry.deleted")); n != 1 {
		t.Fatalf("deleted events: %d", n)
	}
	o.expectProblem(http.StatusNotFound, "not_found", "GET", ct.path("/entries/"+id), nil)
}

func TestChangelogScheduledPublishIsIdempotentAndChecksTheScheduler(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChangelogTeam(t, h)
	o := ct.owner
	ws := uuid.MustParse(ct.ws)
	ctx := context.Background()
	run := func(entry string) {
		t.Helper()
		if err := h.srv.PublishScheduledEntry(ctx, ws, uuid.MustParse(entry)); err != nil {
			t.Fatal(err)
		}
	}
	at := func(d time.Duration) map[string]any {
		return map[string]any{"publish_at": h.clock.Now().Add(d).UTC().Format(time.RFC3339Nano)}
	}

	id := ct.entry(o, "en", "Scheduled for later", "Text.")
	o.expectProblem(http.StatusBadRequest, "validation_failed", "POST", ct.path("/entries/"+id+"/schedule"), at(-time.Minute))
	o.expectProblem(http.StatusBadRequest, "validation_failed", "POST", ct.path("/entries/"+id+"/schedule"), at(366*24*time.Hour))
	s := o.expect(http.StatusOK, "POST", ct.path("/entries/"+id+"/schedule"), at(5*time.Minute))
	if s.str("state") != "scheduled" || s.str("publish_at") == "" || s.str("scheduled_by_member_id") == "" {
		t.Fatalf("scheduled: %s", s.raw)
	}
	if n := len(h.jobs("changelog_publish", ct.ws)); n != 1 {
		t.Fatalf("jobs: %d", n)
	}
	run(id)
	if ct.get(o, id).str("state") != "scheduled" {
		t.Fatal("published before its time")
	}
	h.clock.Advance(6 * time.Minute)
	run(id)
	run(id)
	if err := h.srv.SweepChangelog(ctx); err != nil {
		t.Fatal(err)
	}
	pub := ct.get(o, id)
	if pub.str("state") != "published" || pub.str("published_at") == "" || pub.body["publish_at"] != nil {
		t.Fatalf("published by the schedule: %s", pub.raw)
	}
	if n := len(ct.events("changelog.entry.published")); n != 1 {
		t.Fatalf("a repeated job published %d times", n)
	}
	o.expectProblem(http.StatusConflict, "already_published", "POST", ct.path("/entries/"+id+"/schedule"), at(time.Hour))

	lost := ct.entry(o, "en", "Scheduled by an agent", "Text.")
	o.expect(http.StatusOK, "PATCH", ct.path(""), map[string]any{"agents_may_publish": true})
	ct.agent.expect(http.StatusOK, "POST", ct.path("/entries/"+lost+"/schedule"), at(5*time.Minute))
	o.expect(http.StatusOK, "PATCH", ct.path(""), map[string]any{"agents_may_publish": false})
	h.clock.Advance(6 * time.Minute)
	run(lost)
	failed := ct.get(o, lost)
	if failed.str("state") != "draft" || failed.body["schedule_failed"].(map[string]any)["reason"] != "lost_right" {
		t.Fatalf("an agent who lost the right: %s", failed.raw)
	}
	if n := len(ct.events("changelog.entry.published")); n != 1 {
		t.Fatalf("a refused schedule published: %d", n)
	}

	revoked := ct.entry(o, "en", "Scheduled by a key", "Text.")
	keyRes := o.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "pipeline", "scopes": []string{"changelog:read", "changelog:write", "changelog:publish"}})
	pipeline := h.client()
	pipeline.bearer = keyRes.str("secret")
	ks := pipeline.expect(http.StatusOK, "POST", ct.path("/entries/"+revoked+"/schedule"), at(5*time.Minute))
	if ks.str("scheduled_by_api_key_id") != keyRes.body["api_key"].(map[string]any)["id"] {
		t.Fatalf("scheduled by: %s", ks.raw)
	}
	o.expect(http.StatusNoContent, "DELETE", "/v1/api-keys/"+keyRes.body["api_key"].(map[string]any)["id"].(string), nil)
	h.clock.Advance(6 * time.Minute)
	run(revoked)
	if f := ct.get(o, revoked); f.str("state") != "draft" || f.body["schedule_failed"].(map[string]any)["reason"] != "key_revoked" {
		t.Fatalf("a revoked key: %s", f.raw)
	}
	again := o.expect(http.StatusOK, "POST", ct.path("/entries/"+revoked+"/schedule"), at(time.Minute))
	if again.body["schedule_failed"] != nil || again.str("state") != "scheduled" {
		t.Fatalf("a new schedule keeps the failure: %s", again.raw)
	}

	swept := ct.entry(o, "en", "Found by the sweep", "Text.")
	o.expect(http.StatusOK, "POST", ct.path("/entries/"+swept+"/schedule"), at(time.Minute))
	for _, j := range h.jobs("changelog_publish", ct.ws) {
		h.dropJob(j.id)
	}
	h.clock.Advance(2 * time.Minute)
	if err := h.srv.SweepChangelog(ctx); err != nil {
		t.Fatal(err)
	}
	var due int
	for _, j := range h.jobs("changelog_publish", ct.ws) {
		var a api.ChangelogPublishArgs
		if err := json.Unmarshal(j.args, &a); err != nil {
			t.Fatal(err)
		}
		due++
		run(a.EntryID.String())
		h.dropJob(j.id)
	}
	if due != 2 || ct.get(o, swept).str("state") != "published" || ct.get(o, revoked).str("state") != "published" {
		t.Fatalf("sweep queued %d jobs; swept %s revoked-then-rescheduled %s", due, ct.get(o, swept).str("state"), ct.get(o, revoked).str("state"))
	}

	un := ct.entry(o, "en", "Scheduled then unscheduled", "Text.")
	o.expect(http.StatusOK, "POST", ct.path("/entries/"+un+"/schedule"), at(time.Hour))
	if e := o.expect(http.StatusOK, "POST", ct.path("/entries/"+un+"/unschedule"), nil); e.str("state") != "draft" || e.body["publish_at"] != nil {
		t.Fatalf("unscheduled: %s", e.raw)
	}
	h.clock.Advance(2 * time.Hour)
	run(un)
	if ct.get(o, un).str("state") != "draft" {
		t.Fatal("an unscheduled entry was published")
	}

	for i := range 50 {
		e := ct.entry(o, "en", "Queued number "+strings.Repeat("x", i%5+1), "")
		o.expect(http.StatusOK, "POST", ct.path("/entries/"+e+"/schedule"), at(24*time.Hour))
	}
	extra := ct.entry(o, "en", "One too many", "")
	o.expectProblem(http.StatusConflict, "too_many_scheduled", "POST", ct.path("/entries/"+extra+"/schedule"), at(24*time.Hour))
}

func TestChangelogImagesAndCleanup(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChangelogTeam(t, h)
	o := ct.owner
	upload := func(c color.Color) (string, string) {
		r := o.uploadImageTo(ct.path("/images"), "shot.png", "image/png", testPNG(t, 40, 30, c))
		if r.status != http.StatusCreated {
			t.Fatalf("upload: %d %s", r.status, r.raw)
		}
		return r.str("id"), r.str("markdown")
	}
	cover, _ := upload(color.RGBA{R: 200, A: 255})
	inText, markdown := upload(color.RGBA{G: 200, A: 255})
	unused, _ := upload(color.RGBA{B: 200, A: 255})

	o.expectProblem(http.StatusBadRequest, "validation_failed", "POST", ct.path("/entries"), map[string]any{
		"locale": "en", "title": "Bad image", "labels": []string{"new"}, "image_id": uuid.NewV7().String(),
	})
	id := o.expect(http.StatusCreated, "POST", ct.path("/entries"), map[string]any{
		"locale": "en", "title": "With pictures", "labels": []string{"new"}, "image_id": cover, "body": markdown,
	}).str("id")
	if e := ct.get(o, id); e.body["image"].(map[string]any)["id"] != cover {
		t.Fatalf("entry image: %s", e.raw)
	}

	for _, img := range []string{cover, inText} {
		r := o.expect(http.StatusConflict, "DELETE", "/v1/email-images/"+img, nil)
		used := r.body["used_by"].(map[string]any)["changelog_entries"].([]any)
		if r.str("code") != "image_in_use" || len(used) != 1 || used[0].(map[string]any)["id"] != id {
			t.Fatalf("deleting an image of an entry: %s", r.raw)
		}
	}

	ws := uuid.MustParse(ct.ws)
	exists := func(id string) bool { return h.count("SELECT count(*) FROM email_images WHERE id = $1", id) == 1 }
	sweep := func() int {
		t.Helper()
		keys, err := h.st.DeleteUnusedEmailImages(context.Background(), store.DeleteUnusedEmailImagesParams{WorkspaceID: ws, Before: time.Now().Add(time.Hour), Lim: 100})
		if err != nil {
			t.Fatal(err)
		}
		return len(keys)
	}
	if n := sweep(); n != 1 {
		t.Fatalf("the sweep deleted %d images; only the unused one should go", n)
	}
	if exists(unused) || !exists(cover) || !exists(inText) {
		t.Fatalf("after the first sweep: unused %v cover %v in text %v", exists(unused), exists(cover), exists(inText))
	}

	ct.publish(o, id)
	if html := ct.get(o, id).body["translations"].([]any)[0].(map[string]any)["published"].(map[string]any)["html"].(string); !strings.Contains(html, "<img") {
		t.Fatalf("the published body lost its image: %s", html)
	}
	o.expect(http.StatusOK, "PATCH", ct.path("/entries/"+id), map[string]any{"image_id": nil})
	if n := sweep(); n != 1 {
		t.Fatalf("after the cover left the entry the sweep deleted %d", n)
	}
	if exists(cover) || !exists(inText) {
		t.Fatalf("after the cover left: cover %v in text %v", exists(cover), exists(inText))
	}
	o.expect(http.StatusNoContent, "DELETE", ct.path("/entries/"+id), nil)
	if n := sweep(); n != 1 || exists(inText) {
		t.Fatalf("after the entry went the sweep deleted %d", n)
	}
}

func TestChangelogClientFeedAndInbox(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChangelogTeam(t, h)
	o, public := ct.owner, ct.public()
	inbox := "/client/v1/channels/" + ct.key

	if got := public.expect(http.StatusOK, "GET", inbox, nil); got.body["whats_new"] != nil || got.body["chat"].(map[string]any)["announce_updates"] != false {
		t.Fatalf("before any entry: %s", got.raw)
	}
	public.expect(http.StatusOK, "GET", ct.feedPath(""), nil)
	older := ct.entry(o, "en", "Older entry", "First text.", "new")
	ct.publish(o, older)
	h.clock.Advance(time.Hour)
	newer := ct.entry(o, "en", "Newer entry", "Second **text**.", "fixed", "improved")
	draft := ct.entry(o, "en", "Still a draft", "Hidden.")
	cover := o.uploadImageTo(ct.path("/images"), "cover.png", "image/png", testPNG(t, 64, 32, color.RGBA{R: 9, A: 255}))
	o.expect(http.StatusOK, "PATCH", ct.path("/entries/"+newer), map[string]any{"image_id": cover.str("id")})
	o.expect(http.StatusCreated, "PUT", ct.path("/entries/"+newer+"/translations/tr"), map[string]any{"base_version": 0, "title": "Daha yeni giriş", "body": "İkinci metin."})
	ct.publish(o, newer)

	got := public.expect(http.StatusOK, "GET", inbox, nil)
	wn, _ := got.body["whats_new"].(map[string]any)
	if wn == nil || !strings.HasSuffix(wn["url"].(string), "/changelog/"+ct.slug) || wn["default_locale"] != "en" || wn["latest_published_at"] == nil {
		t.Fatalf("client inbox: %s", got.raw)
	}
	feed := public.expect(http.StatusOK, "GET", ct.feedPath(""), nil)
	items := feed.body["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["title"] != "Newer entry" || items[1].(map[string]any)["title"] != "Older entry" {
		t.Fatalf("feed order: %s", feed.raw)
	}
	top := items[0].(map[string]any)
	if feed.header.Get("Cache-Control") != "public, max-age=60" || top["fallback"] != false || top["locale"] != "en" ||
		!strings.Contains(top["html"].(string), "<strong>text</strong>") || top["excerpt"] != "Second text." ||
		top["image"].(map[string]any)["width"] != float64(64) || top["labels"].([]any)[0] != "fixed" || !strings.Contains(top["url"].(string), "/en/entries/2-newer-entry") {
		t.Fatalf("entry shape: %v %v", top, feed.header)
	}
	if feed.body["changelog"].(map[string]any)["latest_published_at"] != wn["latest_published_at"] {
		t.Fatalf("latest: %v vs %v", feed.body["changelog"], wn)
	}
	trFeed := public.expect(http.StatusOK, "GET", ct.feedPath("?locale=tr-TR"), nil).body["items"].([]any)
	if trFeed[0].(map[string]any)["title"] != "Daha yeni giriş" || trFeed[0].(map[string]any)["fallback"] != false || trFeed[1].(map[string]any)["fallback"] != true || trFeed[1].(map[string]any)["locale"] != "en" {
		t.Fatalf("tr feed: %v", trFeed)
	}
	page := public.expect(http.StatusOK, "GET", ct.feedPath("?limit=1"), nil)
	if len(page.body["items"].([]any)) != 1 || page.str("next_cursor") == "" {
		t.Fatalf("page one: %s", page.raw)
	}
	next := public.expect(http.StatusOK, "GET", ct.feedPath("?limit=1&cursor="+page.str("next_cursor")), nil)
	if len(next.body["items"].([]any)) != 1 || next.body["items"].([]any)[0].(map[string]any)["title"] != "Older entry" || next.str("next_cursor") != "" {
		t.Fatalf("page two: %s", next.raw)
	}
	public.expectProblem(http.StatusBadRequest, "validation_failed", "GET", ct.feedPath("?limit=0"), nil)
	one := public.expect(http.StatusOK, "GET", ct.feedPath("/"+newer+"?locale=tr"), nil)
	if one.str("title") != "Daha yeni giriş" {
		t.Fatalf("one entry: %s", one.raw)
	}
	public.expectProblem(http.StatusNotFound, "not_found", "GET", ct.feedPath("/"+draft), nil)
	stranger := h.client()
	stranger.origin = "https://elsewhere.example"
	stranger.expectProblem(http.StatusForbidden, "origin_not_allowed", "GET", ct.feedPath(""), nil)
	other := newChangelogTeam(t, h)
	other.public().expectProblem(http.StatusNotFound, "not_found", "GET", other.feedPath("/"+newer), nil)

	o.expect(http.StatusOK, "PATCH", ct.path(""), map[string]any{"published": false})
	public.expectProblem(http.StatusNotFound, "whats_new_off", "GET", ct.feedPath(""), nil)
	if got := public.expect(http.StatusOK, "GET", inbox, nil); got.body["whats_new"] != nil {
		t.Fatalf("an unpublished changelog is in the inbox: %s", got.raw)
	}
	o.expect(http.StatusOK, "PATCH", ct.path(""), map[string]any{"published": true})
	o.expect(http.StatusNoContent, "DELETE", ct.path("/entries/"+newer), nil)
	o.expect(http.StatusNoContent, "DELETE", ct.path("/entries/"+older), nil)
	if got := public.expect(http.StatusOK, "GET", inbox, nil); got.body["whats_new"] != nil {
		t.Fatalf("a changelog without entries is in the inbox: %s", got.raw)
	}
}

func TestChatChannelAnnounceUpdates(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChatTeam(t, h, "live", true)
	read := func() (member, client any) {
		m := ct.owner.expect(http.StatusOK, "GET", "/v1/channels/"+ct.channel, nil).body["chat"].(map[string]any)["announce_updates"]
		c := h.client()
		c.origin = ct.origin
		return m, c.expect(http.StatusOK, "GET", "/client/v1/channels/"+ct.key, nil).body["chat"].(map[string]any)["announce_updates"]
	}
	if m, c := read(); m != false || c != false {
		t.Fatalf("default: %v %v", m, c)
	}
	chat := map[string]any{"allowed_origins": []string{ct.origin}, "announce_updates": true}
	ct.owner.expect(http.StatusOK, "PATCH", "/v1/channels/"+ct.channel, map[string]any{"chat": chat})
	if m, c := read(); m != true || c != true {
		t.Fatalf("on: %v %v", m, c)
	}
	ct.owner.expect(http.StatusOK, "PATCH", "/v1/channels/"+ct.channel, map[string]any{"chat": map[string]any{"allowed_origins": []string{ct.origin}, "greeting": "Hello"}})
	if m, c := read(); m != true || c != true {
		t.Fatalf("an update without the field keeps it: %v %v", m, c)
	}
	chat["announce_updates"] = false
	ct.owner.expect(http.StatusOK, "PATCH", "/v1/channels/"+ct.channel, map[string]any{"chat": chat})
	if m, c := read(); m != false || c != false {
		t.Fatalf("off: %v %v", m, c)
	}
}

func TestChangelogBroadcastFromEntry(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChangelogTeam(t, h)
	o := ct.owner
	cover := o.uploadImageTo(ct.path("/images"), "cover.png", "image/png", testPNG(t, 120, 60, color.RGBA{R: 90, B: 200, A: 255}))
	inline := o.uploadImageTo(ct.path("/images"), "inline.png", "image/png", testPNG(t, 50, 50, color.RGBA{G: 150, A: 255}))
	body := "Searching is **faster** now, see [the docs](https://example.com/docs). {{ contact.email }}\n\n## Details\n\n- one\n- two\n\n" +
		inline.str("markdown") + "\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"
	id := o.expect(http.StatusCreated, "POST", ct.path("/entries"), map[string]any{
		"locale": "en", "title": "Faster search", "body": body, "labels": []string{"improved"}, "image_id": cover.str("id"),
	}).str("id")
	post := func(c *client, want int, req map[string]any) response {
		return c.expect(want, "POST", ct.path("/entries/"+id+"/broadcast"), req)
	}
	req := map[string]any{"locale": "en", "list_ids": []string{ct.list}, "channel_id": ct.emailChannel}
	o.expectProblem(http.StatusConflict, "not_published", "POST", ct.path("/entries/"+id+"/broadcast"), req)
	ct.publish(o, id)

	ct.agent.expectProblem(http.StatusForbidden, "forbidden", "POST", ct.path("/entries/"+id+"/broadcast"), req)
	o.expectProblem(http.StatusNotFound, "not_found", "POST", ct.path("/entries/"+id+"/broadcast"), map[string]any{"locale": "tr", "list_ids": []string{ct.list}})
	o.expectProblem(http.StatusBadRequest, "validation_failed", "POST", ct.path("/entries/"+id+"/broadcast"), map[string]any{"locale": "en", "list_ids": []string{uuid.NewV7().String()}})
	other := o.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Other", "slug": "other"}).body["inbox"].(map[string]any)["id"].(string)
	foreign := o.expect(http.StatusCreated, "POST", "/v1/lists", map[string]any{"inbox_id": other, "name": "Elsewhere"}).str("id")
	o.expectProblem(http.StatusBadRequest, "validation_failed", "POST", ct.path("/entries/"+id+"/broadcast"), map[string]any{"locale": "en", "list_ids": []string{foreign}})
	noLists := ct.apiKey("no-lists", map[string]any{"scopes": []string{"changelog:read", "broadcasts:write"}})
	noLists.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", ct.path("/entries/"+id+"/broadcast"), req)
	if n := h.count("SELECT count(*) FROM broadcasts WHERE workspace_id = $1", ct.ws); n != 0 {
		t.Fatalf("a refused request left %d broadcasts", n)
	}
	if n := h.count("SELECT count(*) FROM email_designs WHERE workspace_id = $1", ct.ws); n != 0 {
		t.Fatalf("a refused request left %d designs", n)
	}

	b := post(o, http.StatusCreated, req)
	if b.str("state") != "draft" || b.str("changelog_entry_id") != id || b.str("name") != "What's new: Faster search" || b.str("subject") != "Faster search" ||
		!strings.HasPrefix(b.str("preheader"), "Searching is faster now") || strings.Contains(b.str("preheader"), "{{") || b.str("design_id") == "" {
		t.Fatalf("broadcast: %s", b.raw)
	}
	if lists := b.body["audience"].(map[string]any)["list_ids"].([]any); len(lists) != 1 || lists[0] != ct.list {
		t.Fatalf("audience: %s", b.raw)
	}
	doc := o.expect(http.StatusOK, "GET", "/v1/email-designs/"+b.str("design_id"), nil).body["document"].(map[string]any)
	var ids, types []string
	var raw strings.Builder
	var imageIDs []any
	for _, bl := range doc["blocks"].([]any) {
		m := bl.(map[string]any)
		ids = append(ids, m["id"].(string))
		types = append(types, m["type"].(string))
		j, _ := json.Marshal(m)
		raw.Write(j)
		if m["image_id"] != nil {
			imageIDs = append(imageIDs, m["image_id"])
		}
	}
	text := raw.String()
	for _, gone := range []string{"logo", "f1", "f2", "f3", "f4", "hero"} {
		if slices.Contains(ids, gone) && gone != "hero" {
			t.Fatalf("block %q is left in the design: %v", gone, ids)
		}
	}
	for _, gone := range []string{"Northwind", "<table", "{{ contact"} {
		if strings.Contains(text, gone) {
			t.Fatalf("%q is left in the design: %s", gone, text)
		}
	}
	if len(imageIDs) != 2 || imageIDs[0] != cover.str("id") || imageIDs[1] != inline.str("id") {
		t.Fatalf("images in the design: %v", imageIDs)
	}
	if !strings.Contains(text, "IMPROVED") || !strings.Contains(text, "Faster search") || !strings.Contains(text, "/changelog/"+ct.slug+"/en/entries/1-faster-search") || !strings.Contains(text, "{{ inbox.name }}") {
		t.Fatalf("design text: %s", text)
	}
	if h.smtp.count() != 0 {
		t.Fatal("making the draft sent mail")
	}
	ctKey := ct.apiKey("mailer", map[string]any{"scopes": []string{"changelog:read", "broadcasts:write", "lists:read"}})
	post(ctKey, http.StatusCreated, req)

	o.expect(http.StatusNoContent, "DELETE", ct.path("/entries/"+id), nil)
	if after := o.expect(http.StatusOK, "GET", "/v1/broadcasts/"+b.str("id"), nil); after.body["changelog_entry_id"] != nil {
		t.Fatalf("the broadcast kept a deleted entry: %s", after.raw)
	}
}

func TestChangelogClientReadsAreLimited(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChangelogTeam(t, h)
	public := ct.public()
	for range 120 {
		public.expect(http.StatusOK, "GET", ct.feedPath("?limit=1"), nil)
	}
	r := public.expect(http.StatusTooManyRequests, "GET", ct.feedPath("?limit=1"), nil)
	if r.str("code") != "rate_limited" || r.header.Get("Retry-After") == "" {
		t.Fatalf("limit: %s %v", r.raw, r.header)
	}
	other := ct.public()
	other.ip = "10.9.9.9"
	other.expect(http.StatusOK, "GET", ct.feedPath("?limit=1"), nil)
}

func TestChangelogScheduledEntryIsLockedForThoseWhoCannotPublish(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChangelogTeam(t, h)
	o := ct.owner
	id := ct.entry(ct.agent, "en", "Agent draft", "Text.")
	o.expect(http.StatusOK, "POST", ct.path("/entries/"+id+"/schedule"), map[string]any{"publish_at": h.clock.Now().Add(time.Hour).Format(time.RFC3339)})
	tr := ct.path("/entries/" + id + "/translations/en")
	ct.agent.expectProblem(http.StatusForbidden, "entry_scheduled", "PUT", tr, map[string]any{"base_version": 1, "title": "Sneaky change"})
	ct.agent.expectProblem(http.StatusForbidden, "entry_scheduled", "PATCH", ct.path("/entries/"+id), map[string]any{"labels": []string{"fixed"}})
	ct.agent.expectProblem(http.StatusForbidden, "entry_scheduled", "DELETE", ct.path("/entries/"+id+"/translations/tr"), nil)
	grant := h.client()
	grant.bearer = h.oauthToken(o, ct.ws, "Claude Code", testOrigin, []string{"changelog:read", "changelog:write"})
	grant.expectProblem(http.StatusForbidden, "member_session_required", "PUT", tr, map[string]any{"base_version": 1, "title": "Sneaky change"})
	o.expect(http.StatusOK, "PUT", tr, map[string]any{"base_version": 1, "title": "Owner edit"})
	o.expect(http.StatusOK, "POST", ct.path("/entries/"+id+"/unschedule"), nil)
	ct.agent.expect(http.StatusOK, "PUT", tr, map[string]any{"base_version": 2, "title": "Agent again"})
}

func TestChangelogBroadcastNameFitsLongTitles(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChangelogTeam(t, h)
	id := ct.entry(ct.owner, "en", strings.Repeat("{{ ", 66)+"x", "Body.")
	ct.publish(ct.owner, id)
	b := ct.owner.expect(http.StatusCreated, "POST", ct.path("/entries/"+id+"/broadcast"), map[string]any{"locale": "en", "list_ids": []string{ct.list}})
	if n := len([]rune(b.str("name"))); n > 200 || strings.Contains(b.str("name"), "{{") {
		t.Fatalf("name: %d %q", n, b.str("name"))
	}
}

func (c *client) fetch(path string, header map[string]string) response {
	c.h.t.Helper()
	req, err := http.NewRequest("GET", c.h.url+path, nil)
	if err != nil {
		c.h.t.Fatal(err)
	}
	req.Header.Set("X-Forwarded-For", c.ip)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	hc := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := hc.Do(req)
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

func TestChangelogPublicPagesAndFeed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChangelogTeam(t, h)
	o := ct.owner
	web := h.client()
	base := "/changelog/" + ct.slug

	pub := ct.entry(o, "en", "Faster search", "Results appear **as you type**.\n\n| a | b |\n|---|---|\n| 1 | 2 |\n", "improved")
	o.expect(http.StatusCreated, "PUT", ct.path("/entries/"+pub+"/translations/tr"), map[string]any{"base_version": 0, "title": "Daha hızlı arama", "body": "Sonuçlar **yazarken** çıkar."})
	ct.publish(o, pub)
	draft := ct.entry(o, "en", "Secret draft", "Hidden words.")
	sched := ct.entry(o, "en", "Scheduled secret", "Later words.")
	o.expect(http.StatusOK, "POST", ct.path("/entries/"+sched+"/schedule"), map[string]any{"publish_at": h.clock.Now().Add(time.Hour).Format(time.RFC3339)})
	_ = draft

	root := web.fetch(base, nil)
	if root.status != http.StatusFound || !strings.HasSuffix(root.header.Get("Location"), base+"/en/") {
		t.Fatalf("root: %d %v", root.status, root.header)
	}
	home := web.fetch(base+"/en/", nil)
	body := string(home.raw)
	if home.status != 200 || !strings.Contains(body, "Faster search") || !strings.Contains(body, "<strong>as you type</strong>") ||
		!strings.Contains(body, `hreflang="tr"`) || !strings.Contains(body, `rel="canonical" href="http://localhost:8080`+base+`/en/"`) ||
		!strings.Contains(body, "feed.atom") || strings.Contains(body, "Secret draft") || strings.Contains(body, "Scheduled secret") || strings.Contains(body, "Hidden words") {
		t.Fatalf("home: %d %s", home.status, body)
	}
	if home.header.Get("Cache-Control") != "public, max-age=60, stale-while-revalidate=300, stale-if-error=86400" || home.header.Get("ETag") == "" ||
		!strings.Contains(home.header.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatalf("headers: %v", home.header)
	}
	if again := web.fetch(base+"/en/", map[string]string{"If-None-Match": home.header.Get("ETag")}); again.status != http.StatusNotModified {
		t.Fatalf("etag: %d", again.status)
	}
	if tr := web.fetch(base+"/tr/", nil); !strings.Contains(string(tr.raw), "Daha hızlı arama") || !strings.Contains(string(tr.raw), "Yenilikler") {
		t.Fatalf("tr: %s", tr.raw)
	}
	if lab := web.fetch(base+"/en/?label=fixed", nil); strings.Contains(string(lab.raw), "Faster search") || lab.status != 200 {
		t.Fatalf("label filter: %d", lab.status)
	}

	entry := web.fetch(base+"/en/entries/1-faster-search", nil)
	eb := string(entry.raw)
	if entry.status != 200 || !strings.Contains(eb, `"@type":"BlogPosting"`) || !strings.Contains(eb, `"@type":"BreadcrumbList"`) || !strings.Contains(eb, `hreflang="tr" href=`) {
		t.Fatalf("entry: %d %s", entry.status, eb)
	}
	wrong := web.fetch(base+"/en/entries/1-old-slug", nil)
	if wrong.status != http.StatusMovedPermanently || !strings.HasSuffix(wrong.header.Get("Location"), base+"/en/entries/1-faster-search") {
		t.Fatalf("slug redirect: %d %v", wrong.status, wrong.header)
	}
	for _, p := range []string{"/en/entries/2-secret-draft", "/en/entries/3-scheduled-secret", "/en/entries/9-nothing", "/de/", "/en/nothing", "/tr/entries/2-secret-draft"} {
		if r := web.fetch(base+p, nil); r.status != http.StatusNotFound || !strings.HasPrefix(r.header.Get("Content-Type"), "text/plain") {
			t.Fatalf("%s: %d %s", p, r.status, r.raw)
		}
	}
	if r := web.fetch("/changelog/no-such-slug/en/", nil); r.status != http.StatusNotFound {
		t.Fatalf("unknown slug: %d", r.status)
	}

	feed := web.fetch(base+"/en/feed.atom", nil)
	fb := string(feed.raw)
	if feed.status != 200 || !strings.HasPrefix(feed.header.Get("Content-Type"), "application/atom+xml") || !strings.Contains(fb, `<category term="improved">`) ||
		!strings.Contains(fb, `type="html"`) || !strings.Contains(fb, "&lt;strong&gt;as you type&lt;/strong&gt;") || strings.Contains(fb, "Secret") || strings.Contains(fb, "Scheduled") {
		t.Fatalf("feed: %d %s", feed.status, fb)
	}
	if trFeed := web.fetch(base+"/tr/feed.atom", nil); !strings.Contains(string(trFeed.raw), "Daha hızlı arama") || strings.Contains(string(trFeed.raw), "Faster search") {
		t.Fatalf("tr feed: %s", trFeed.raw)
	}
	site := web.fetch(base+"/sitemap.xml", nil)
	if sb := string(site.raw); site.status != 200 || !strings.Contains(sb, "/en/entries/1-faster-search") || !strings.Contains(sb, `hreflang="tr"`) || strings.Contains(sb, "secret") {
		t.Fatalf("sitemap: %d %s", site.status, sb)
	}
	if font := web.fetch(base+publicFontPathForTest(t, string(home.raw)), nil); font.status != 200 {
		t.Fatalf("font: %d", font.status)
	}

	renamed := unique("renamed")
	o.expect(http.StatusOK, "PATCH", ct.path(""), map[string]any{"slug": renamed})
	moved := web.fetch(base+"/en/entries/1-faster-search?x=1", nil)
	if moved.status != http.StatusMovedPermanently || !strings.Contains(moved.header.Get("Location"), "/changelog/"+renamed+"/en/entries/1-faster-search") {
		t.Fatalf("former slug: %d %v", moved.status, moved.header)
	}
	o.expect(http.StatusOK, "PATCH", ct.path(""), map[string]any{"published": false})
	for _, p := range []string{"/changelog/" + renamed + "/en/", "/changelog/" + renamed + "/en/feed.atom", "/changelog/" + renamed + "/en/entries/1-faster-search", base + "/en/"} {
		if r := web.fetch(p, nil); r.status != http.StatusNotFound {
			t.Fatalf("unpublished %s: %d", p, r.status)
		}
	}
	o.expect(http.StatusOK, "PATCH", ct.path(""), map[string]any{"published": true})
	ct.owner.expect(http.StatusOK, "POST", ct.path("/entries/"+pub+"/unpublish"), nil)
	if r := web.fetch("/changelog/"+renamed+"/en/", nil); strings.Contains(string(r.raw), "Faster search") {
		t.Fatalf("an unpublished entry is on the page")
	}
	if r := web.fetch("/changelog/"+renamed+"/en/feed.atom", nil); strings.Contains(string(r.raw), "Faster search") {
		t.Fatalf("an unpublished entry is in the feed")
	}
}

func publicFontPathForTest(t *testing.T, page string) string {
	t.Helper()
	i := strings.Index(page, "/fonts/Geist-Variable.")
	if i < 0 {
		t.Fatal("no font in page")
	}
	j := strings.Index(page[i:], ".woff2")
	return page[i : i+j+len(".woff2")]
}

func TestChangelogPublicPagesAreLimitedAndOffByServer(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChangelogTeam(t, h)
	web := h.client()
	for range 120 {
		web.fetch("/changelog/"+ct.slug+"/en/", nil)
	}
	if r := web.fetch("/changelog/"+ct.slug+"/en/", nil); r.status != http.StatusTooManyRequests || r.header.Get("Retry-After") == "" {
		t.Fatalf("limit: %d %v", r.status, r.header)
	}
	off := newHarnessWith(t, func(d *api.Deps) { d.Changelog.Off = true })
	if r := off.client().fetch("/changelog/x/en/", nil); r.status != http.StatusNotFound {
		t.Fatalf("off: %d", r.status)
	}
}

func TestChangelogPreview(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChangelogTeam(t, h)
	o := ct.owner
	o.expect(http.StatusOK, "PATCH", ct.path(""), map[string]any{"published": false})
	id := ct.entry(o, "en", "Draft only", "Working **draft** text.")
	prev := func(c *client, q string) response { return c.do("GET", ct.path("/preview"+q), nil) }
	r := prev(o, "?draft=true")
	if r.status != 200 || !strings.Contains(string(r.raw), "Working <strong>draft</strong> text.") || !strings.Contains(string(r.raw), `name="robots"`) ||
		r.header.Get("Cache-Control") != "no-store" || !strings.Contains(r.header.Get("Content-Security-Policy"), "frame-ancestors 'self'") {
		t.Fatalf("draft home: %d %s %v", r.status, r.raw, r.header)
	}
	if r := prev(o, ""); strings.Contains(string(r.raw), "Draft only") {
		t.Fatalf("a draft shows without draft=true")
	}
	if r := prev(o, "?view=entry&draft=true&id="+id); r.status != 200 || !strings.Contains(string(r.raw), "Draft only") {
		t.Fatalf("draft entry: %d %s", r.status, r.raw)
	}
	o.expectProblem(http.StatusNotFound, "not_found", "GET", ct.path("/preview?view=entry&id="+id), nil)
	o.expectProblem(http.StatusBadRequest, "validation_failed", "GET", ct.path("/preview?view=entry"), nil)
	o.expectProblem(http.StatusBadRequest, "locale_not_offered", "GET", ct.path("/preview?locale=de"), nil)
	ct.publish(o, id)
	if r := prev(o, "?view=entry&id="+id); r.status != 200 {
		t.Fatalf("published entry of an unpublished changelog: %d", r.status)
	}
	stranger := newTeam(t, h)
	stranger.owner.expectProblem(http.StatusNotFound, "not_found", "GET", ct.path("/preview"), nil)
	reader := ct.apiKey("reader", map[string]any{"scopes": []string{"conversations:read"}})
	reader.expectProblem(http.StatusForbidden, "insufficient_scope", "GET", ct.path("/preview"), nil)
}

func TestChangelogMCPDraftsAndNeverPublishes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChangelogTeam(t, h)
	o := ct.owner
	tok := h.client()
	tok.bearer = h.oauthTokenVia(o, ct.ws, "Claude Code", testOrigin+"/mcp", []string{"changelog:read", "changelog:write"}, "http://127.0.0.1:33418/callback", "http://127.0.0.1:33418/callback")
	cs := h.mcpSession(tok.bearer)
	names := toolNames(t, cs)
	for _, want := range []string{"list_whats_new_entries", "get_whats_new_entry", "draft_whats_new_entry", "update_whats_new_draft"} {
		if !slices.Contains(names, want) {
			t.Fatalf("missing %s: %v", want, names)
		}
	}
	for _, n := range names {
		if strings.Contains(n, "publish") || strings.Contains(n, "schedule_whats") {
			t.Fatalf("a tool could publish: %s", n)
		}
	}
	if blind := toolNames(t, h.mcpSession(ct.apiKey("blind", map[string]any{"scopes": []string{"conversations:read"}}).bearer)); slices.ContainsFunc(blind, func(n string) bool { return strings.Contains(n, "whats_new") }) {
		t.Fatalf("What's new tools without changelog scopes: %v", blind)
	}

	draft, msg := callTool(t, cs, "draft_whats_new_entry", map[string]any{"title": "Faster search", "body": "Results appear **as you type**.", "labels": []string{"improved"}})
	if msg != "" || draft["state"] != "draft" || draft["locale"] != "en" || !strings.Contains(draft["note"].(string), "cannot publish") {
		t.Fatalf("draft %v %s", draft, msg)
	}
	entry := draft["entry_id"].(string)
	if _, msg := callTool(t, cs, "draft_whats_new_entry", map[string]any{"entry_id": entry, "locale": "tr", "title": "Daha hızlı arama", "body": "Sonuçlar yazarken çıkar."}); msg != "" {
		t.Fatal(msg)
	}
	if _, msg := callTool(t, cs, "draft_whats_new_entry", map[string]any{"title": "No labels", "body": "x"}); msg == "" {
		t.Fatal("an entry without labels was drafted")
	}
	upd, msg := callTool(t, cs, "update_whats_new_draft", map[string]any{"entry_id": entry, "locale": "en", "base_version": 1, "body": "New text.", "labels": []string{"fixed"}})
	if msg != "" || upd["version"] != float64(2) {
		t.Fatalf("update %v %s", upd, msg)
	}
	if _, msg := callTool(t, cs, "update_whats_new_draft", map[string]any{"entry_id": entry, "locale": "en", "base_version": 1, "body": "Stale."}); !strings.Contains(msg, "version_conflict") {
		t.Fatalf("stale update: %q", msg)
	}
	got, msg := callTool(t, cs, "get_whats_new_entry", map[string]any{"entry_id": entry})
	if msg != "" || got["labels"].([]any)[0] != "fixed" || len(got["translations"].([]any)) != 2 {
		t.Fatalf("get %v %s", got, msg)
	}
	list, msg := callTool(t, cs, "list_whats_new_entries", map[string]any{"state": "draft"})
	if msg != "" || len(list["items"].([]any)) != 1 {
		t.Fatalf("list %v %s", list, msg)
	}
	res, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "yuva://changelog/" + ct.changelog})
	if err != nil || !strings.Contains(res.Contents[0].Text, "Daha hızlı arama") {
		t.Fatalf("resource: %v %v", res, err)
	}
	pr, err := cs.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "draft_release_notes", Arguments: map[string]string{"notes": "fix: search lag"}})
	if err != nil || !strings.Contains(pr.Messages[0].Content.(*mcp.TextContent).Text, "Do not try to publish") {
		t.Fatalf("prompt: %v %v", pr, err)
	}

	rest := h.client()
	rest.bearer = h.oauthToken(o, ct.ws, "Claude Code", testOrigin, []string{"changelog:read", "changelog:write"})
	rest.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", ct.path("/entries/"+entry+"/publish"), nil)
	if e := ct.get(o, entry); e.str("state") != "draft" {
		t.Fatalf("the token published: %s", e.raw)
	}
	o.expect(http.StatusOK, "PATCH", ct.path(""), map[string]any{"agents_may_publish": true})
	rest.expectProblem(http.StatusForbidden, "insufficient_scope", "POST", ct.path("/entries/"+entry+"/schedule"), map[string]any{"publish_at": h.clock.Now().Add(time.Hour).Format(time.RFC3339)})

	stranger := newTeam(t, h)
	str := h.client()
	str.bearer = h.oauthTokenVia(stranger.owner, stranger.ws, "Claude Code", testOrigin+"/mcp", []string{"changelog:read", "changelog:write"}, "http://127.0.0.1:33418/callback", "http://127.0.0.1:33418/callback")
	scs := h.mcpSession(str.bearer)
	if _, msg := callTool(t, scs, "get_whats_new_entry", map[string]any{"changelog_id": ct.changelog, "entry_id": entry}); msg == "" {
		t.Fatal("another workspace read the entry")
	}
	if _, err := scs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "yuva://changelog/" + ct.changelog}); err == nil {
		t.Fatal("another workspace read the resource")
	}
}

func TestChangelogMCPKeyCannotEditScheduledOrRelabelPublished(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ct := newChangelogTeam(t, h)
	o := ct.owner
	key := ct.apiKey("pipeline", map[string]any{"scopes": []string{"changelog:read", "changelog:write", "changelog:publish"}})
	cs := h.mcpSession(key.bearer)
	sched := ct.entry(o, "en", "Scheduled one", "Text.")
	o.expect(http.StatusOK, "POST", ct.path("/entries/"+sched+"/schedule"), map[string]any{"publish_at": h.clock.Now().Add(time.Hour).Format(time.RFC3339)})
	for _, args := range []map[string]any{
		{"entry_id": sched, "locale": "en", "base_version": 1, "body": "Sneaky."},
		{"entry_id": sched, "locale": "tr", "title": "Gizli", "body": "x"},
	} {
		tool := "update_whats_new_draft"
		if args["base_version"] == nil {
			tool = "draft_whats_new_entry"
		}
		if _, msg := callTool(t, cs, tool, args); !strings.Contains(msg, "entry_scheduled") {
			t.Fatalf("%s on a scheduled entry: %q", tool, msg)
		}
	}
	if e := ct.get(o, sched); e.body["translations"].([]any)[0].(map[string]any)["body"] != "Text." || len(e.body["translations"].([]any)) != 1 {
		t.Fatalf("a scheduled entry changed: %s", e.raw)
	}
	pub := ct.entry(o, "en", "Published one", "Text.", "new")
	ct.publish(o, pub)
	if _, msg := callTool(t, cs, "update_whats_new_draft", map[string]any{"entry_id": pub, "locale": "en", "base_version": 1, "labels": []string{"fixed"}}); !strings.Contains(msg, "forbidden") {
		t.Fatalf("relabelling a published entry: %q", msg)
	}
	if ct.get(o, pub).body["labels"].([]any)[0] != "new" {
		t.Fatal("the labels changed")
	}
	if _, msg := callTool(t, cs, "update_whats_new_draft", map[string]any{"entry_id": pub, "locale": "en", "base_version": 1, "body": "Draft only."}); msg != "" {
		t.Fatal(msg)
	}
}

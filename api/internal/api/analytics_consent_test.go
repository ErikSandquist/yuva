package api_test

import (
	"context"
	"net/http"
	"testing"
	"time"
	"uuid"
)

const (
	pidAyse  = "0f8fad5b-d9cb-469f-a165-70867728950e"
	pidOther = "7c9e6679-7425-40de-944b-e07fc1f90ae7"
	pidThird = "Q2hhbmdlTWVQbGVhc2UtMTIzNDU2"
)

type consentSite struct {
	id, key, inbox, secret string
}

// consentProperty is a property with both consent features, linked to an inbox of its own.
func consentProperty(t *testing.T, tm team) consentSite {
	t.Helper()
	in := tm.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Shop", "slug": unique("shop")})
	inbox := in.body["inbox"].(map[string]any)["id"].(string)
	id, key := createProperty(tm.owner, map[string]any{"name": "Shop", "domains": []string{"example.com"}, "timezone": "UTC", "inbox_id": inbox,
		"consent_features": []string{"persistent", "identify"}})
	return consentSite{id: id, key: key, inbox: inbox, secret: in.str("identity_secret")}
}

func consented(page, pid string) map[string]any {
	return map[string]any{"n": "pageview", "u": "https://example.com" + page, "c": map[string]any{"a": 1, "p": pid}}
}

func identity(h *harness, secret, sub string) string {
	return signToken(secret, map[string]any{"alg": "HS256", "typ": "JWT"}, map[string]any{
		"sub": sub, "exp": h.clock.Now().Add(5 * time.Minute).Unix(), "email": sub + "@shop.example", "email_verified": true})
}

func identify(h *harness, key, pid, token string) {
	h.t.Helper()
	send(h, key, "203.0.113.10", uaMac, map[string]any{"n": "yuva_consent", "u": "https://example.com/account", "c": map[string]any{"a": 1, "p": pid, "j": token}})
}

func contactVisits(c *client, contact string) []any {
	c.h.t.Helper()
	return c.expect(http.StatusOK, "GET", "/v1/contacts/"+contact+"/visits", nil).body["items"].([]any)
}

func contactBySub(h *harness, ws, inbox, sub string) string {
	h.t.Helper()
	var id string
	if err := h.st.Pool.QueryRow(context.Background(), `SELECT contact_id::text FROM contact_external_ids
		WHERE workspace_id = $1 AND inbox_id = $2 AND external_id = $3`, ws, inbox, sub).Scan(&id); err != nil {
		h.t.Fatalf("no contact for %s: %v", sub, err)
	}
	return id
}

func TestAnalyticsConsentDefaultModeStoresNothingPersistent(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	site := consentProperty(t, tm)
	plainID, plainKey := createProperty(tm.owner, map[string]any{"name": "Plain", "domains": []string{"example.com"}, "timezone": "UTC"})
	h.clock.Set(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))

	send(h, site.key, "203.0.113.10", uaMac, view("/pricing"))
	send(h, site.key, "198.51.100.11", uaMac, map[string]any{"n": "pageview", "u": "https://example.com/", "c": map[string]any{"a": 0, "p": pidAyse}})
	send(h, site.key, "192.0.2.12", uaMac, map[string]any{"n": "pageview", "u": "https://example.com/", "c": map[string]any{"a": 1, "p": "short"}})
	send(h, site.key, "2001:db8:1::13", uaMac, map[string]any{"n": "pageview", "u": "https://example.com/",
		"c": map[string]any{"a": 0, "p": pidOther, "j": identity(h, site.secret, "u-1")}})
	send(h, plainKey, "203.0.113.14", uaMac, consented("/", pidAyse))
	send(h, plainKey, "203.0.113.14", uaMac, map[string]any{"n": "yuva_consent", "u": "https://example.com/", "c": map[string]any{"a": 1, "p": pidAyse,
		"j": identity(h, site.secret, "u-2")}})
	h.flushAnalytics()

	for _, id := range []string{site.id, plainID} {
		if n := h.count("SELECT count(*) FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2 AND (persistent OR visitor_type <> 0 OR contact_id IS NOT NULL)", tm.ws, id); n != 0 {
			t.Fatalf("%d visits without consent were made persistent", n)
		}
	}
	if n := h.count("SELECT count(*) FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2", tm.ws, site.id); n != 4 {
		t.Fatalf("%d visits, want every visit counted in default mode", n)
	}
	if n := h.count("SELECT count(*) FROM analytics_persistent_visitors WHERE workspace_id = $1", tm.ws); n != 0 {
		t.Fatalf("%d persistent visitors without consent", n)
	}
	if n := h.count("SELECT count(*) FROM contact_external_ids WHERE workspace_id = $1 AND external_id IN ('u-1', 'u-2')", tm.ws); n != 0 {
		t.Fatal("an identity token without consent or on a property without identify made a contact")
	}
	if n := h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1 AND name = 'yuva_consent'", tm.ws); n != 0 {
		t.Fatal("yuva_consent was stored as an event")
	}
	if items := contactVisits(tm.owner, tm.contact); len(items) != 0 {
		t.Fatalf("contact visits %v", items)
	}
}

func TestAnalyticsConsentReturningVisitorRetentionAndContactVisits(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	site := consentProperty(t, tm)
	pricing := tm.owner.expect(http.StatusCreated, "POST", "/v1/analytics/properties/"+site.id+"/goals",
		map[string]any{"name": "Pricing", "kind": "pageview", "path_pattern": "/pricing"}).str("id")
	signup := tm.owner.expect(http.StatusCreated, "POST", "/v1/analytics/properties/"+site.id+"/goals",
		map[string]any{"name": "Signup", "kind": "pageview", "path_pattern": "/signup"}).str("id")
	funnel := tm.owner.expect(http.StatusCreated, "POST", "/v1/analytics/properties/"+site.id+"/funnels",
		map[string]any{"name": "Buy", "steps": []string{pricing, signup}}).str("id")

	h.clock.Set(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	send(h, site.key, "203.0.113.10", uaMac, consented("/pricing", pidAyse))
	send(h, site.key, "198.51.100.20", uaPixel, view("/pricing"))
	identify(h, site.key, pidAyse, identity(h, site.secret, "u-1"))
	forged := signToken("not-the-secret", map[string]any{"alg": "HS256", "typ": "JWT"}, map[string]any{"sub": "u-9", "exp": h.clock.Now().Add(time.Minute).Unix()})
	send(h, site.key, "203.0.113.30", uaWindows, consented("/", pidOther))
	identify(h, site.key, pidOther, forged)
	expired := signToken(site.secret, map[string]any{"alg": "HS256", "typ": "JWT"}, map[string]any{"sub": "u-8", "exp": h.clock.Now().Add(-time.Hour).Unix()})
	identify(h, site.key, pidOther, expired)
	h.flushAnalytics()

	h.clock.Set(time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC))
	send(h, site.key, "198.51.100.77", uaWindows, consented("/signup", pidAyse))
	send(h, site.key, "198.51.100.20", uaPixel, view("/pricing"))
	h.flushAnalytics()

	if n := h.count("SELECT count(*) FROM contact_external_ids WHERE workspace_id = $1 AND external_id IN ('u-8', 'u-9')", tm.ws); n != 0 {
		t.Fatal("a forged or expired token made a contact")
	}
	if n := h.count("SELECT count(*) FROM analytics_persistent_visitors WHERE workspace_id = $1 AND contact_id IS NOT NULL", tm.ws); n != 1 {
		t.Fatalf("%d linked persistent ids, want only the one with a valid token", n)
	}
	ayse := contactBySub(h, tm.ws, site.inbox, "u-1")

	two := map[string]any{"from": "2026-10-12", "to": "2026-10-13"}
	totals := query(tm.owner, site.id, map[string]any{"metrics": []string{"visitors", "visits"}, "range": two}).body["totals"].(map[string]any)
	if totals["visitors"].(float64) != 4 || totals["visits"].(float64) != 5 {
		t.Fatalf("totals %v: want the consented visitor once over two days and the anonymous one once per day", totals)
	}
	weekly := query(tm.owner, site.id, map[string]any{"metrics": []string{"visitors"}, "range": two, "dimensions": []string{"time:week"}})
	if v := weekly.body["rows"].([]any)[0].(map[string]any)["metrics"].(map[string]any)["visitors"].(float64); v != 4 {
		t.Fatalf("week visitors %v", v)
	}
	filtered := query(tm.owner, site.id, map[string]any{"metrics": []string{"visitors"}, "range": two,
		"filters": []map[string]any{{"dimension": "browser", "op": "is_not", "values": []string{"Chrome"}}}}).body["totals"].(map[string]any)
	if filtered["visitors"].(float64) != 2 {
		t.Fatalf("filtered visitors %v, want the consented visitor once and the other consented one", filtered)
	}
	daily := rowsBy(query(tm.owner, site.id, map[string]any{"metrics": []string{"visitors"}, "range": two, "dimensions": []string{"time:day"}}))
	if daily["2026-10-12"]["visitors"].(float64) != 3 || daily["2026-10-13"]["visitors"].(float64) != 2 {
		t.Fatalf("daily visitors %v", daily)
	}
	types := rowsBy(query(tm.owner, site.id, map[string]any{"metrics": []string{"visitors"}, "range": dayRange("2026-10-13"), "dimensions": []string{"visitor_type"}}))
	if types["returning"]["visitors"].(float64) != 1 || types[""]["visitors"].(float64) != 1 || types["new"] != nil {
		t.Fatalf("visitor types %v", types)
	}

	report := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+site.id+"/funnels/"+funnel+"/report?from=2026-10-12&to=2026-10-13", nil)
	steps := report.body["steps"].([]any)
	first, second := steps[0].(map[string]any), steps[1].(map[string]any)
	if first["consented_visitors"].(float64) != 1 || second["consented_visitors"].(float64) != 1 || second["visitors"].(float64) != 0 {
		t.Fatalf("funnel %s: want the consented visitor through both steps across days, nobody within a day", report.raw)
	}

	if err := h.srv.BuildAnalyticsCohorts(context.Background()); err != nil {
		t.Fatal(err)
	}
	ret := tm.owner.expect(http.StatusOK, "GET", "/v1/analytics/properties/"+site.id+"/retention?grain=day&from=2026-10-12&to=2026-10-13", nil)
	cohorts := ret.body["cohorts"].([]any)
	if len(cohorts) != 1 || ret.body["built_at"] == nil {
		t.Fatalf("retention %s", ret.raw)
	}
	c := cohorts[0].(map[string]any)
	periods := c["periods"].([]any)
	if c["cohort"] != "2026-10-12" || c["visitors"].(float64) != 2 || len(periods) != 2 || periods[0].(float64) != 2 || periods[1].(float64) != 1 {
		t.Fatalf("cohort %v", c)
	}

	visits := contactVisits(tm.owner, ayse)
	if len(visits) != 2 {
		t.Fatalf("contact visits %v", visits)
	}
	latest := visits[0].(map[string]any)
	if latest["entry_page"] != "/signup" || latest["visitor_type"] != "returning" || latest["property_id"] != site.id ||
		latest["pages"].([]any)[0].(map[string]any)["path"] != "/signup" || visits[1].(map[string]any)["visitor_type"] != "new" {
		t.Fatalf("latest visit %v", latest)
	}
	page := tm.owner.expect(http.StatusOK, "GET", "/v1/contacts/"+ayse+"/visits?limit=1", nil)
	next := page.str("next_cursor")
	rest := tm.owner.expect(http.StatusOK, "GET", "/v1/contacts/"+ayse+"/visits?limit=1&cursor="+next, nil).body["items"].([]any)
	if len(page.body["items"].([]any)) != 1 || len(rest) != 1 || rest[0].(map[string]any)["entry_page"] != "/pricing" {
		t.Fatalf("paging %s %v", page.raw, rest)
	}

	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/contacts/"+ayse+"/visits", nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/analytics/properties/"+site.id+"/retention", nil)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "DELETE", "/v1/analytics/properties/"+site.id+"/visitors/"+pidAyse, nil)
	reader := h.client()
	reader.bearer = tm.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "reader", "scopes": []string{"contacts:read"}}).str("secret")
	reader.expectProblem(http.StatusForbidden, "insufficient_scope", "GET", "/v1/contacts/"+ayse+"/visits", nil)

	r := collect(h, site.key, collectReq{origin: "https://example.com", ip: "203.0.113.10", header: map[string]string{"Sec-GPC": "1"},
		body: map[string]any{"n": "yuva_consent", "u": "https://example.com/", "c": map[string]any{"a": 0, "p": pidAyse}}})
	if r.status != http.StatusAccepted {
		t.Fatalf("withdraw %d %s", r.status, r.raw)
	}
	if n := h.count("SELECT count(*) FROM analytics_persistent_visitors WHERE workspace_id = $1 AND property_id = $2 AND contact_id IS NOT NULL", tm.ws, site.id); n != 0 {
		t.Fatal("consent(false) left the persistent id linked")
	}
	if items := contactVisits(tm.owner, ayse); len(items) != 0 {
		t.Fatalf("visits after withdrawal %v", items)
	}
	if n := h.count("SELECT count(*) FROM analytics_persistent_days WHERE workspace_id = $1 AND property_id = $2", tm.ws, site.id); n != 1 {
		t.Fatalf("%d cohort days, want only the other consented visitor's", n)
	}
}

func TestAnalyticsConsentContactDeletionMergeAndVisitorDeletion(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	site := consentProperty(t, tm)
	h.clock.Set(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	send(h, site.key, "203.0.113.10", uaMac, consented("/pricing", pidAyse))
	send(h, site.key, "203.0.113.10", uaMac, map[string]any{"n": "Signup", "u": "https://example.com/signup", "c": map[string]any{"a": 1, "p": pidAyse}})
	send(h, site.key, "198.51.100.20", uaPixel, consented("/docs", pidOther))
	send(h, site.key, "198.51.100.30", uaWindows, view("/"))
	h.flushAnalytics()
	identify(h, site.key, pidAyse, identity(h, site.secret, "u-1"))
	ayse := contactBySub(h, tm.ws, site.inbox, "u-1")

	tm.owner.expect(http.StatusOK, "POST", "/v1/contacts/"+tm.contact+"/merge", map[string]any{"source_id": ayse})
	if items := contactVisits(tm.owner, tm.contact); len(items) != 1 {
		t.Fatalf("merged contact visits %v", items)
	}
	if n := h.count("SELECT count(*) FROM analytics_persistent_visitors WHERE workspace_id = $1 AND contact_id = $2", tm.ws, tm.contact); n != 1 {
		t.Fatal("merge did not move the persistent id's link")
	}

	h.clock.Advance(time.Minute)
	send(h, site.key, "203.0.113.10", uaMac, consented("/account", pidAyse))
	h.flushAnalytics()
	if items := contactVisits(tm.owner, tm.contact); len(items) != 1 || items[0].(map[string]any)["pages"].([]any)[1] == nil {
		t.Fatalf("a later event of the linked id did not join its visit: %v", items)
	}

	tm.owner.expect(http.StatusNoContent, "DELETE", "/v1/contacts/"+tm.contact, nil)
	if n := h.count("SELECT count(*) FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2", tm.ws, site.id); n != 2 {
		t.Fatalf("%d visits after deleting the contact, want the two others", n)
	}
	if n := h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1 AND property_id = $2", tm.ws, site.id); n != 2 {
		t.Fatalf("%d events after deleting the contact", n)
	}
	if n := h.count("SELECT count(*) FROM analytics_persistent_visitors WHERE workspace_id = $1 AND property_id = $2", tm.ws, site.id); n != 1 {
		t.Fatalf("%d persistent ids after deleting the contact", n)
	}

	tm.agent.expectProblem(http.StatusNotFound, "not_found", "DELETE", "/v1/analytics/properties/"+site.id+"/visitors/"+pidOther, nil)
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "DELETE", "/v1/analytics/properties/"+site.id+"/visitors/short", nil)
	tm.owner.expect(http.StatusNoContent, "DELETE", "/v1/analytics/properties/"+site.id+"/visitors/"+pidThird, nil)
	tm.owner.expect(http.StatusNoContent, "DELETE", "/v1/analytics/properties/"+site.id+"/visitors/"+pidOther, nil)
	if n := h.count("SELECT count(*) FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2", tm.ws, site.id); n != 1 {
		t.Fatalf("%d visits after deleting the visitor, want the anonymous one", n)
	}
	if n := h.count("SELECT count(*) FROM analytics_persistent_visitors WHERE workspace_id = $1", tm.ws); n != 0 {
		t.Fatalf("%d persistent ids left", n)
	}
}

func TestAnalyticsConsentAppsAndServerEvents(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	in := tm.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "App", "slug": unique("app")})
	inbox := in.body["inbox"].(map[string]any)["id"].(string)
	id, key := createProperty(tm.owner, map[string]any{"name": "App", "domains": []string{"example.com"}, "platforms": []string{"web", "ios"},
		"timezone": "UTC", "inbox_id": inbox, "consent_features": []string{"persistent", "identify"}})
	h.clock.Set(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	at := h.clock.Now()
	withConsent := func(ev map[string]any, c map[string]any) map[string]any { ev["c"] = c; return ev }
	r := collect(h, key, collectReq{ip: "203.0.113.10", body: appBatch("0f8fad5b-d9cb-469f-a165-000000000001", "ios",
		withConsent(screen("Home", at), map[string]any{"a": 1, "p": pidAyse}),
		withConsent(screen("Cart", at.Add(time.Second)), map[string]any{"a": 1, "p": pidAyse}),
		screen("Home", at))})
	if r.status != http.StatusAccepted {
		t.Fatalf("app batch %d %s", r.status, r.raw)
	}
	backend := tm.apiKey(h)
	serverEvents(backend, http.StatusAccepted, id,
		map[string]any{"name": "Purchase", "url": "app://com.example.shop/Cart", "visitor_key": "s-1", "consent": true, "persistent_id": pidAyse,
			"identity_token": identity(h, in.str("identity_secret"), "u-7")},
		map[string]any{"name": "Purchase", "url": "app://com.example.shop/Cart", "visitor_key": "s-2", "persistent_id": pidOther})
	h.flushAnalytics()

	if n := h.count("SELECT count(*) FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2 AND persistent AND platform = 2 AND views = 2 AND events = 1", tm.ws, id); n != 1 {
		t.Fatal("the app's consented screens and the server event did not share one persistent visit")
	}
	if n := h.count("SELECT count(*) FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2 AND NOT persistent", tm.ws, id); n != 2 {
		t.Fatalf("%d default-mode visits, want the screen without consent and the server event without consent", n)
	}
	app := contactBySub(h, tm.ws, inbox, "u-7")
	if items := contactVisits(tm.owner, app); len(items) != 1 || items[0].(map[string]any)["platform"] != "ios" {
		t.Fatalf("app contact visits %v", items)
	}
}

func TestAnalyticsConsentFeatureRules(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	base := func(extra map[string]any) map[string]any {
		b := map[string]any{"name": "Kids", "domains": []string{"example.com"}}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties",
		base(map[string]any{"audience": "children", "consent_features": []string{"persistent"}}))
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties",
		base(map[string]any{"consent_features": []string{"identify"}, "inbox_id": tm.inbox}))
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/analytics/properties",
		base(map[string]any{"consent_features": []string{"persistent", "identify"}}))
	id, _ := createProperty(tm.owner, base(map[string]any{"consent_features": []string{"persistent"}}))
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "PATCH", "/v1/analytics/properties/"+id, map[string]any{"audience": "children"})
	kids, _ := createProperty(tm.owner, base(map[string]any{"audience": "children"}))
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "PATCH", "/v1/analytics/properties/"+kids, map[string]any{"consent_features": []string{"persistent"}})
}

func TestAnalyticsConsentVisitorTypeKeepsVisitNumbers(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	site := consentProperty(t, tm)
	h.clock.Set(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	for _, page := range []string{"/", "/pricing", "/signup"} {
		send(h, site.key, "203.0.113.10", uaMac, consented(page, pidAyse))
		h.flushAnalytics()
		h.clock.Advance(time.Minute)
	}
	send(h, site.key, "198.51.100.20", uaPixel, view("/"))
	h.flushAnalytics()
	check := func(when string) {
		t.Helper()
		rows := rowsBy(query(tm.owner, site.id, map[string]any{"metrics": []string{"visitors", "visits", "pageviews"}, "range": dayRange("2026-10-12"),
			"dimensions": []string{"visitor_type"}}))
		if rows["new"] == nil || rows["new"]["pageviews"].(float64) != 3 || rows["new"]["visits"].(float64) != 1 || rows[""]["pageviews"].(float64) != 1 {
			t.Fatalf("%s: visitor types %v", when, rows)
		}
	}
	check("raw")
	ws, prop, day := uuid.MustParse(tm.ws), uuid.MustParse(site.id), time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	h.clock.Set(time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC))
	if err := h.srv.FoldAnalyticsDay(context.Background(), ws, prop, day); err != nil {
		t.Fatal(err)
	}
	check("folded")
	h.clock.Set(time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))
	if err := h.srv.RollupAnalyticsDay(context.Background(), ws, prop, day); err != nil {
		t.Fatal(err)
	}
	check("rollups")
}

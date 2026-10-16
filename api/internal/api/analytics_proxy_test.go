package api_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	"github.com/productdevbook/yuva/api/internal/api"
)

func TestAnalyticsCollectBehindSiteProxy(t *testing.T) {
	logs := &lockedBuffer{}
	const proxy = "198.51.100.50"
	h := newHarnessWith(t, func(d *api.Deps) {
		d.Log = slog.New(slog.NewJSONHandler(logs, nil))
		d.Auth.TrustedProxies = append(d.Auth.TrustedProxies, netip.MustParsePrefix(proxy+"/32"))
	})
	tm := newTeam(t, h)
	ctx := context.Background()
	send := func(key, ip, page string) {
		t.Helper()
		if r := collect(h, key, collectReq{origin: "https://example.com", ip: ip, body: pageview("https://example.com" + page)}); r.status != http.StatusAccepted {
			t.Fatalf("%s: status %d: %s", ip, r.status, r.raw)
		}
	}
	visits := func(id string) int {
		t.Helper()
		return h.count("SELECT count(*) FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2", tm.ws, id)
	}
	property := func(name string) (string, string) {
		return createProperty(tm.owner, map[string]any{"name": name, "domains": []string{"example.com"}, "timezone": "UTC"})
	}

	proxied, proxiedKey := property("Proxied")
	for i := range 65 {
		send(proxiedKey, "203.0.113.10, "+proxy, "/a")
		if i%5 == 0 {
			send(proxiedKey, "192.0.2.20, "+proxy, "/b")
		}
	}
	for range 52 {
		send(proxiedKey, "192.0.2.20,"+proxy, "/b")
	}
	direct, directKey := property("Direct")
	send(directKey, "198.51.100.60", "/")
	send(directKey, "198.51.100.60", "/again")
	send(directKey, "", "/")
	ignored, ignoredKey := property("Ignored")
	send(ignoredKey, proxy, "/")
	for _, chain := range []string{"10.1.2.3, ", "192.168.0.9, ", "127.0.0.1, ", "100.64.0.1, ", "fe80::1, ", "not-an-address, ", "203.0.113.10:443, "} {
		send(ignoredKey, chain+proxy, "/x")
	}
	h.flushAnalytics()

	if n := visits(proxied); n != 2 {
		t.Fatalf("two visitors behind one proxy are %d visits", n)
	}
	if n := visits(direct); n != 2 {
		t.Fatalf("direct requests: %d visits", n)
	}
	if n := visits(ignored); n != 2 {
		t.Fatalf("private or malformed forwarded addresses: %d visits", n)
	}
	if n := h.count("SELECT count(*) FROM analytics_events WHERE workspace_id = $1 AND property_id = $2", tm.ws, ignored); n != 8 {
		t.Fatalf("%d events with ignored forwarded addresses", n)
	}

	rows, err := h.st.Pool.Query(ctx, `SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'workspace_id'
		WHERE n.nspname = 'public' AND c.relname LIKE 'analytics\_%' AND c.relkind IN ('r', 'p') AND NOT c.relispartition`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	rows.Close()
	if len(tables) == 0 {
		t.Fatal("no analytics tables")
	}
	for _, table := range tables {
		if n := h.count(`SELECT count(*) FROM `+table+` t WHERE workspace_id = $1 AND t::text ~ '203\.0\.113\.|192\.0\.2\.|198\.51\.100\.'`, tm.ws); n > 0 {
			t.Fatalf("%s holds a forwarded or proxy address", table)
		}
	}
	if !strings.Contains(logs.String(), `"/client/v1/analytics/`) {
		t.Fatal("the request log was not captured")
	}
	if m := testAddresses.FindString(logs.String()); m != "" {
		t.Fatalf("the log holds an address: %s", m)
	}
}

func TestAnalyticsCollectIgnoresForgedForwardedFor(t *testing.T) {
	h := newHarnessWith(t, func(d *api.Deps) { d.Auth.TrustedProxies = nil })
	tm := newTeam(t, h)
	id, key := createProperty(tm.owner, map[string]any{"name": "Forged", "domains": []string{"example.com"}, "timezone": "UTC"})
	for i := range 20 {
		ip := fmt.Sprintf("203.0.113.%d", i+1)
		if r := collect(h, key, collectReq{origin: "https://example.com", ip: ip, body: pageview("https://example.com/")}); r.status != http.StatusAccepted {
			t.Fatalf("%s: status %d: %s", ip, r.status, r.raw)
		}
	}
	h.flushAnalytics()
	if n := h.count("SELECT count(*) FROM analytics_visits WHERE workspace_id = $1 AND property_id = $2", tm.ws, id); n != 1 {
		t.Fatalf("rotating forged addresses made %d visitors, want the one socket address", n)
	}
}

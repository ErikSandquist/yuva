package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseStatusPushBodies(t *testing.T) {
	for _, c := range []struct {
		file, source, message string
		ok, take              bool
	}{
		{"alertmanager_firing.json", "alertmanager", "The API answers 5xx on every request", false, true},
		{"alertmanager_resolved.json", "alertmanager", "The API answers 5xx on every request", true, true},
		{"grafana_firing.json", "grafana", "Checkout p95 latency is above 3 seconds", false, true},
		{"grafana_resolved.json", "grafana", "Checkout p95 latency is above 3 seconds", true, true},
		{"grafana_legacy_alerting.json", "grafana", "[Alerting] Queue depth alert", false, true},
		{"grafana_legacy_no_data.json", "", "", false, false},
	} {
		body, err := os.ReadFile(filepath.Join("testdata", "status", c.file))
		if err != nil {
			t.Fatal(err)
		}
		r, take, err := parseStatusPush(body)
		if err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		if take != c.take || r.ok != c.ok || r.source != c.source || r.message != c.message || r.state != "" {
			t.Errorf("%s: take %v, %+v", c.file, take, r)
		}
	}
}

func TestParseStatusPushOwnJSON(t *testing.T) {
	r, take, err := parseStatusPush([]byte(`{"state":"partial_outage","message":"EU workers lag"}`))
	if err != nil || !take || r.ok || r.state != "partial_outage" || r.message != "EU workers lag" || r.source != "push" {
		t.Fatalf("partial outage: %+v %v %v", r, take, err)
	}
	if r, take, err = parseStatusPush([]byte(`{"state":"operational"}`)); err != nil || !take || !r.ok {
		t.Fatalf("operational: %+v %v %v", r, take, err)
	}
	for _, bad := range []string{`{"state":"maintenance"}`, `{"state":"broken"}`, `{"status":"firing"}`, `[]`, `not json`, `{}`} {
		if _, _, err := parseStatusPush([]byte(bad)); err != errPushUnreadable {
			t.Errorf("%s: %v", bad, err)
		}
	}
}

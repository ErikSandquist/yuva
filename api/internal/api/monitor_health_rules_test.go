package api

import (
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/store"
)

func TestHealthStateHysteresis(t *testing.T) {
	type step struct{ raw, want string }
	for _, c := range []struct {
		name  string
		start string
		steps []step
	}{
		{"unknown takes the first state at once", "unknown", []step{{"healthy", "healthy"}}},
		{"worse at once", "healthy", []step{{"degraded", "degraded"}, {"down", "down"}}},
		{"two healthy evaluations recover", "down", []step{{"healthy", "down"}, {"healthy", "healthy"}}},
		{"a relapse resets the count", "down", []step{{"healthy", "down"}, {"down", "down"}, {"healthy", "down"}, {"healthy", "healthy"}}},
		{"a better state must repeat itself", "down", []step{{"degraded", "down"}, {"healthy", "down"}, {"healthy", "healthy"}}},
		{"down to degraded after two", "down", []step{{"degraded", "down"}, {"degraded", "degraded"}}},
		{"unknown at once", "down", []step{{"unknown", "unknown"}}},
	} {
		state, pending, n := c.start, "", int32(0)
		for i, s := range c.steps {
			state, pending, n = nextHealthState(state, pending, n, s.raw)
			if state != s.want {
				t.Fatalf("%s: step %d (%s) gave %s, want %s", c.name, i, s.raw, state, s.want)
			}
		}
	}
}

func TestRawServiceState(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	at := now.Add(-30 * time.Second)
	status := func(s string) *string { return &s }
	svc := store.MonitorService{ID: uuid.NewV7(), Name: "billing", Environment: "production", IntervalSeconds: 60, ErrorRateThreshold: 5, LastReportAt: &at,
		LastStatus: status("healthy")}
	dep := func(name, st string, critical bool) store.MonitorDependency {
		return store.MonitorDependency{ID: uuid.NewV7(), ServiceID: svc.ID, Name: name, Critical: critical, Status: status(st), ReportedAt: &at}
	}
	none := maintenance{services: map[uuid.UUID]bool{}, dependencies: map[uuid.UUID]bool{}, checks: map[uuid.UUID]bool{}}
	pg, cache := dep("postgres", "down", true), dep("redis", "down", false)
	stale := dep("old", "down", true)
	stale.ReportedAt = new(at.Add(-time.Minute))
	check := store.MonitorCheck{ID: uuid.NewV7(), Name: "Billing health", State: "down"}
	p95 := 300.0
	for _, c := range []struct {
		name    string
		svc     func(store.MonitorService) store.MonitorService
		deps    []store.MonitorDependency
		checks  []store.MonitorCheck
		traffic *serviceTraffic
		mt      maintenance
		want    string
		reason  string
	}{
		{"healthy report", nil, []store.MonitorDependency{dep("postgres", "healthy", true)}, nil, nil, none, "healthy", ""},
		{"critical dependency down", nil, []store.MonitorDependency{pg, cache}, nil, nil, none, "down", "postgres is down"},
		{"non-critical dependency down", nil, []store.MonitorDependency{cache}, nil, nil, none, "degraded", "redis is down"},
		{"dependency degraded", nil, []store.MonitorDependency{dep("queue", "degraded", true)}, nil, nil, none, "degraded", "queue is degraded"},
		{"a dependency the last report left out", nil, []store.MonitorDependency{stale}, nil, nil, none, "healthy", ""},
		{"own status", func(s store.MonitorService) store.MonitorService { s.LastStatus = status("down"); return s }, nil, nil, nil, none, "down",
			"The service reports itself down"},
		{"silence", func(s store.MonitorService) store.MonitorService { s.LastReportAt = new(now.Add(-181 * time.Second)); return s },
			[]store.MonitorDependency{pg}, nil, nil, none, "down", "No report for 3 minutes"},
		{"silence set by the member", func(s store.MonitorService) store.MonitorService {
			s.LastReportAt, s.SilenceSeconds = new(now.Add(-181*time.Second)), new(int32(600))
			return s
		}, nil, nil, nil, none, "healthy", ""},
		{"linked check down", nil, nil, []store.MonitorCheck{check}, nil, none, "down", "The uptime check Billing health is down"},
		{"5xx rate", nil, nil, nil, &serviceTraffic{requests: 40, errorRate: 7.5}, none, "degraded", "5xx rate 7.5% over 5 minutes"},
		{"too few requests", nil, nil, nil, &serviceTraffic{requests: 19, errorRate: 50}, none, "healthy", ""},
		{"p95", func(s store.MonitorService) store.MonitorService { s.P95ThresholdMs = &p95; return s }, nil, nil, &serviceTraffic{requests: 40, p95: 812},
			none, "degraded", "p95 812 ms over 5 minutes"},
		{"never reported", func(s store.MonitorService) store.MonitorService { s.LastReportAt, s.LastStatus = nil, nil; return s }, nil, nil, nil, none,
			"unknown", ""},
		{"never reported, check down", func(s store.MonitorService) store.MonitorService { s.LastReportAt, s.LastStatus = nil, nil; return s }, nil,
			[]store.MonitorCheck{check}, nil, none, "down", "The uptime check Billing health is down"},
		{"dependency under maintenance", nil, []store.MonitorDependency{pg}, nil, nil,
			maintenance{services: map[uuid.UUID]bool{}, dependencies: map[uuid.UUID]bool{pg.ID: true}, checks: map[uuid.UUID]bool{}}, "healthy", ""},
		{"service under maintenance", nil, []store.MonitorDependency{pg}, nil, nil,
			maintenance{services: map[uuid.UUID]bool{svc.ID: true}, dependencies: map[uuid.UUID]bool{}, checks: map[uuid.UUID]bool{}}, "down", "postgres is down"},
		{"check under maintenance", nil, nil, []store.MonitorCheck{check},
			nil, maintenance{services: map[uuid.UUID]bool{}, dependencies: map[uuid.UUID]bool{}, checks: map[uuid.UUID]bool{check.ID: true}}, "healthy", ""},
	} {
		s := svc
		if c.svc != nil {
			s = c.svc(s)
		}
		state, reason := rawServiceState(s, c.deps, c.checks, c.traffic, c.mt, now)
		if state != c.want || reason != c.reason {
			t.Fatalf("%s: %s %q, want %s %q", c.name, state, reason, c.want, c.reason)
		}
	}
}

package monitor

import (
	"strings"
	"testing"
)

func check(t *testing.T, r HealthReading, name string) HealthCheck {
	t.Helper()
	for _, c := range r.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no check %s in %+v", name, r.Checks)
	return HealthCheck{}
}

func TestParseHealthJSON(t *testing.T) {
	body := `{"status":"warn","version":"1","releaseId":"1.4.2","checks":{
		"postgres:responseTime":[{"componentType":"datastore","observedValue":12.5,"observedUnit":"ms","status":"pass"}],
		"postgres:connections":[{"componentType":"datastore","observedValue":40,"status":"pass"}],
		"search:responseTime":[{"componentType":"component","observedValue":0.25,"observedUnit":"s","status":"fail","output":"timeout","critical":false,"kind":"http"}],
		"cache":[{"status":"warn","kind":"cache"}]}}`
	r := ParseHealth(503, "application/health+json; charset=utf-8", []byte(body))
	if r.Format != FormatHealthJSON || r.Status != HealthDegraded || r.Version != "1.4.2" || len(r.Checks) != 3 {
		t.Fatalf("reading %+v", r)
	}
	pg := check(t, r, "postgres")
	if pg.Kind != "database" || pg.Status != HealthHealthy || !pg.Critical || pg.LatencyMS == nil || *pg.LatencyMS != 12.5 {
		t.Fatalf("postgres %+v", pg)
	}
	search := check(t, r, "search")
	if search.Kind != "http" || search.Status != HealthDown || search.Critical || search.Message != "timeout" || *search.LatencyMS != 250 {
		t.Fatalf("search %+v", search)
	}
	if c := check(t, r, "cache"); c.Kind != "cache" || c.Status != HealthDegraded || c.LatencyMS != nil {
		t.Fatalf("cache %+v", c)
	}
	if r := ParseHealth(200, "application/json", []byte(`{"status":"pass"}`)); r.Format != FormatHealthJSON || r.Status != HealthHealthy {
		t.Fatalf("health+json shape under application/json: %+v", r)
	}
	long := `{"status":"fail","checks":{"db":[{"status":"fail","output":"` + strings.Repeat("é", 300) + `"}]}}`
	if c := check(t, ParseHealth(503, "application/health+json", []byte(long)), "db"); len([]rune(c.Message)) != HealthMessageRunes {
		t.Fatalf("message of %d runes", len([]rune(c.Message)))
	}
}

func TestParseActuator(t *testing.T) {
	v3 := `{"status":"DOWN","components":{"db":{"status":"UP","details":{"database":"PostgreSQL"}},"diskSpace":{"status":"UP"},
		"redis":{"status":"DOWN","details":{"error":"org.springframework.data.redis.RedisConnectionFailureException: Unable to connect"}},
		"rabbit":{"status":"OUT_OF_SERVICE"},"custom":{"status":"FLAKY"}}}`
	r := ParseHealth(503, "application/vnd.spring-boot.actuator.v3+json", []byte(v3))
	if r.Format != FormatActuator || r.Status != HealthDown || len(r.Checks) != 5 {
		t.Fatalf("reading %+v", r)
	}
	for name, want := range map[string][2]string{"db": {"database", HealthHealthy}, "diskSpace": {"other", HealthHealthy}, "redis": {"cache", HealthDown},
		"rabbit": {"queue", HealthDown}, "custom": {"other", HealthDegraded}} {
		if c := check(t, r, name); c.Kind != want[0] || c.Status != want[1] || !c.Critical {
			t.Fatalf("%s %+v", name, c)
		}
	}
	if !strings.HasPrefix(check(t, r, "redis").Message, "org.springframework") {
		t.Fatal("the indicator's error is the message")
	}
	v2 := `{"status":"UP","details":{"mongo":{"status":"UP"}}}`
	if r := ParseHealth(200, "application/json", []byte(v2)); r.Format != FormatActuator || r.Status != HealthHealthy || check(t, r, "mongo").Kind != "database" {
		t.Fatalf("actuator 2.0 %+v", r)
	}
}

func TestParseHealthByStatus(t *testing.T) {
	for _, c := range []struct {
		status int
		ctype  string
		body   string
		want   string
	}{
		{200, "text/plain", "OK", HealthHealthy},
		{204, "", "", HealthHealthy},
		{302, "text/html", "<a>", HealthHealthy},
		{503, "text/plain", "unavailable", HealthDown},
		{500, "application/json", `{"error":"x"}`, HealthDown},
		{200, "application/json", `{"status":"maybe"}`, HealthHealthy},
		{404, "application/json", `[1,2]`, HealthDown},
	} {
		r := ParseHealth(c.status, c.ctype, []byte(c.body))
		if r.Format != FormatStatus || r.Status != c.want || len(r.Checks) != 0 {
			t.Fatalf("%d %s %q: %+v", c.status, c.ctype, c.body, r)
		}
	}
}

// The Go SDK's health.Handler serves this shape; read back, it must give what its pushed report
// says.
func TestParseGoSDKHealthJSON(t *testing.T) {
	body := `{"status":"warn","version":"1","releaseId":"1.4.2","serviceId":"billing","checks":{
		"postgres:responseTime":[{"componentType":"datastore","observedValue":3.2,"observedUnit":"ms","status":"pass","time":"2026-04-02T09:00:00Z","critical":true,"kind":"database"}],
		"redis:responseTime":[{"componentType":"component","observedValue":0.4,"observedUnit":"ms","status":"fail","time":"2026-04-02T09:00:00Z","output":"refused","critical":false,"kind":"cache"}]}}`
	r := ParseHealth(200, "application/health+json", []byte(body))
	pg, redis := check(t, r, "postgres"), check(t, r, "redis")
	if r.Status != HealthDegraded || pg.Kind != "database" || !pg.Critical || pg.Status != HealthHealthy || *pg.LatencyMS != 3.2 ||
		redis.Kind != "cache" || redis.Critical || redis.Status != HealthDown || redis.Message != "refused" {
		t.Fatalf("reading %+v", r)
	}
}

package monitor

import (
	"encoding/json"
	"mime"
	"slices"
	"strings"
	"unicode/utf8"
)

// Health statuses, from the best.
const (
	HealthHealthy  = "healthy"
	HealthDegraded = "degraded"
	HealthDown     = "down"
)

// Formats a pulled health endpoint answered in.
const (
	FormatHealthJSON = "health+json"
	FormatActuator   = "actuator"
	FormatStatus     = "status"
)

// HealthMessageRunes is how long a dependency's message may be.
const HealthMessageRunes = 200

// HealthCheck is one dependency as a health report names it.
type HealthCheck struct {
	Name      string
	Kind      string
	Status    string
	LatencyMS *float64
	Critical  bool
	Message   string
}

// HealthReading is what a pulled health endpoint said.
type HealthReading struct {
	Format  string
	Status  string
	Version string
	Checks  []HealthCheck
}

// Worse is the worse of two health statuses.
func Worse(a, b string) string {
	if HealthRank(b) > HealthRank(a) {
		return b
	}
	return a
}

// HealthRank orders statuses: healthy 0, degraded 1, down 2.
func HealthRank(s string) int {
	switch s {
	case HealthDown:
		return 2
	case HealthDegraded:
		return 1
	}
	return 0
}

func clip(s string) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= HealthMessageRunes {
		return s
	}
	r := []rune(s)
	return string(r[:HealthMessageRunes])
}

// ParseHealth reads a pulled answer: application/health+json (the IETF health check draft),
// Spring Boot Actuator's status and components, or else only its status code, 200 to 399 being
// healthy.
func ParseHealth(status int, contentType string, body []byte) HealthReading {
	media, _, _ := mime.ParseMediaType(contentType)
	var doc map[string]json.RawMessage
	if json.Unmarshal(body, &doc) == nil {
		var top string
		_ = json.Unmarshal(doc["status"], &top)
		switch {
		case media == "application/health+json":
			return healthJSON(doc, top)
		case strings.Contains(media, "spring-boot.actuator"):
			return actuator(doc, top)
		case top != "" && actuatorStatus(top) != "":
			return actuator(doc, top)
		case top != "" && draftStatus(top) != "":
			return healthJSON(doc, top)
		}
	}
	r := HealthReading{Format: FormatStatus, Status: HealthHealthy}
	if status < 200 || status > 399 {
		r.Status = HealthDown
	}
	return r
}

func draftStatus(s string) string {
	switch strings.ToLower(s) {
	case "pass", "ok", "up":
		return HealthHealthy
	case "warn":
		return HealthDegraded
	case "fail", "error", "down":
		return HealthDown
	}
	return ""
}

func healthJSON(doc map[string]json.RawMessage, top string) HealthReading {
	r := HealthReading{Format: FormatHealthJSON, Status: draftStatus(top)}
	if r.Status == "" {
		r.Status = HealthDegraded
	}
	var release, version string
	_ = json.Unmarshal(doc["releaseId"], &release)
	_ = json.Unmarshal(doc["version"], &version)
	r.Version = clipVersion(release, version)
	var checks map[string][]map[string]json.RawMessage
	_ = json.Unmarshal(doc["checks"], &checks)
	byName := map[string]*HealthCheck{}
	var names []string
	for key, entries := range checks {
		name, measurement, _ := strings.Cut(key, ":")
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		c, ok := byName[name]
		if !ok {
			c = &HealthCheck{Name: name, Kind: "other", Status: HealthHealthy, Critical: true}
			byName[name] = c
			names = append(names, name)
		}
		for _, e := range entries {
			var st, typ, kind, output, unit string
			_ = json.Unmarshal(e["status"], &st)
			_ = json.Unmarshal(e["componentType"], &typ)
			_ = json.Unmarshal(e["kind"], &kind)
			_ = json.Unmarshal(e["output"], &output)
			_ = json.Unmarshal(e["observedUnit"], &unit)
			s := draftStatus(st)
			if s == "" {
				s = HealthDegraded
			}
			c.Status = Worse(c.Status, s)
			if typ == "datastore" {
				c.Kind = "database"
			}
			if slices.Contains(DependencyKinds, kind) {
				c.Kind = kind
			}
			var critical bool
			if json.Unmarshal(e["critical"], &critical) == nil && e["critical"] != nil {
				c.Critical = critical
			}
			if output != "" && c.Message == "" {
				c.Message = clip(output)
			}
			var v float64
			if measurement == "responseTime" && json.Unmarshal(e["observedValue"], &v) == nil && v >= 0 {
				switch unit {
				case "s":
					v *= 1000
				case "us", "µs":
					v /= 1000
				case "ns":
					v /= 1e6
				}
				c.LatencyMS = &v
			}
		}
	}
	slices.Sort(names)
	for _, n := range names {
		r.Checks = append(r.Checks, *byName[n])
	}
	return r
}

func actuatorStatus(s string) string {
	switch s {
	case "UP":
		return HealthHealthy
	case "DOWN", "OUT_OF_SERVICE":
		return HealthDown
	case "UNKNOWN":
		return HealthDegraded
	}
	return ""
}

// actuatorKinds tells a dependency's kind from the names of Spring Boot's health indicators.
var actuatorKinds = map[string]string{
	"db": "database", "datasource": "database", "mongo": "database", "cassandra": "database", "couchbase": "database", "neo4j": "database",
	"influxdb": "database", "elasticsearch": "database", "r2dbc": "database", "redis": "cache", "hazelcast": "cache", "rabbit": "queue",
	"jms": "queue", "kafka": "queue", "mail": "http",
}

func actuator(doc map[string]json.RawMessage, top string) HealthReading {
	r := HealthReading{Format: FormatActuator, Status: actuatorStatus(top)}
	if r.Status == "" {
		r.Status = HealthDegraded
	}
	raw := doc["components"]
	if raw == nil {
		raw = doc["details"]
	}
	var comps map[string]struct {
		Status  string                     `json:"status"`
		Details map[string]json.RawMessage `json:"details"`
	}
	_ = json.Unmarshal(raw, &comps)
	names := make([]string, 0, len(comps))
	for n := range comps {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		c := comps[n]
		if c.Status == "" {
			continue
		}
		s := actuatorStatus(c.Status)
		if s == "" {
			s = HealthDegraded
		}
		kind, longest := "other", 0
		for prefix, k := range actuatorKinds {
			if strings.HasPrefix(strings.ToLower(n), prefix) && len(prefix) > longest {
				kind, longest = k, len(prefix)
			}
		}
		var msg string
		_ = json.Unmarshal(c.Details["error"], &msg)
		r.Checks = append(r.Checks, HealthCheck{Name: clipName(n), Kind: kind, Status: s, Critical: true, Message: clip(msg)})
	}
	return r
}

func clipName(n string) string {
	if utf8.RuneCountInString(n) <= 100 {
		return n
	}
	return string([]rune(n)[:100])
}

func clipVersion(vs ...string) string {
	for _, v := range vs {
		if v = strings.TrimSpace(v); v != "" {
			if utf8.RuneCountInString(v) > 80 {
				v = string([]rune(v)[:80])
			}
			return v
		}
	}
	return ""
}

// DependencyKinds are the kinds a dependency may have.
var DependencyKinds = []string{"database", "cache", "queue", "http", "other"}

package monitor

import (
	"fmt"
	"strings"
	"time"
)

// ZoneChange is a zone a property counted in until a local midnight of that zone.
type ZoneChange struct {
	Timezone string    `json:"timezone"`
	Until    time.Time `json:"until"`
}

type zoneSpan struct {
	loc         *time.Location
	from, until time.Time
}

// Zone is the time zones a property's hours and days were counted in: each earlier zone until its
// Until, then the current one. A change takes effect at a local midnight of the zone before it, so
// local dates never go back and every local day is one run of instants.
type Zone struct {
	spans []zoneSpan
}

// FixedZone is a zone that never changed.
func FixedZone(loc *time.Location) Zone { return Zone{spans: []zoneSpan{{loc: loc}}} }

// NewZone is the current zone after the earlier ones; a zone that cannot be loaded counts as UTC.
func NewZone(current *time.Location, earlier []ZoneChange) Zone {
	var z Zone
	var from time.Time
	for _, c := range earlier {
		if !c.Until.After(from) {
			continue
		}
		loc, err := time.LoadLocation(c.Timezone)
		if err != nil {
			loc = time.UTC
		}
		z.spans = append(z.spans, zoneSpan{loc: loc, from: from, until: c.Until.UTC()})
		from = c.Until.UTC()
	}
	z.spans = append(z.spans, zoneSpan{loc: current, from: from})
	return z
}

func (z Zone) span(t time.Time) zoneSpan {
	for _, s := range z.spans {
		if s.until.IsZero() || t.Before(s.until) {
			return s
		}
	}
	return zoneSpan{loc: time.UTC}
}

// At is the zone that applies at t.
func (z Zone) At(t time.Time) *time.Location { return z.span(t).loc }

// Current is the zone that applies from the last change on.
func (z Zone) Current() *time.Location { return z.spans[len(z.spans)-1].loc }

// LocalHour is the first instant of the local hour t is in, never before the zone took effect.
func (z Zone) LocalHour(t time.Time) time.Time { return maxTime(LocalHour(t, z.At(t)), z.span(t).from) }

// HourEnd is when the local hour that starts at h ends: an hour later, or when the zone after it
// takes effect.
func (z Zone) HourEnd(h time.Time) time.Time {
	s := z.span(h)
	end := LocalHour(h, s.loc).Add(time.Hour)
	if !s.until.IsZero() && end.After(s.until) {
		return s.until
	}
	return end
}

// LocalDay is the local date of t, as midnight UTC.
func (z Zone) LocalDay(t time.Time) time.Time { return LocalDay(t, z.At(t)) }

// DayStart is the first instant of the local day day.
func (z Zone) DayStart(day time.Time) time.Time {
	for _, s := range z.spans {
		start := DayStart(day, s.loc)
		if s.until.IsZero() || start.Before(s.until) {
			return maxTime(start, s.from)
		}
	}
	return DayStart(day, time.UTC)
}

// sql is expr in the zone of col's value: expr takes the parameter that holds a zone's name, and
// with clip the result is never before that zone took effect. Parameters are appended to args.
func (z Zone) sql(col string, expr func(tz string) string, clip bool, args *[]any) string {
	param := func(v any) string {
		*args = append(*args, v)
		return fmt.Sprintf("$%d", len(*args))
	}
	part := func(s zoneSpan) string {
		e := expr(param(s.loc.String()) + "::text")
		if clip && !s.from.IsZero() {
			e = "greatest(" + e + ", " + param(s.from) + "::timestamptz)"
		}
		return e
	}
	if len(z.spans) == 1 {
		return part(z.spans[0])
	}
	var b strings.Builder
	b.WriteString("CASE")
	for _, s := range z.spans[:len(z.spans)-1] {
		fmt.Fprintf(&b, " WHEN %s < %s::timestamptz THEN %s", col, param(s.until), part(s))
	}
	fmt.Fprintf(&b, " ELSE %s END", part(z.spans[len(z.spans)-1]))
	return b.String()
}

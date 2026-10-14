package monitor

import (
	"testing"
	"time"
)

func TestZoneChangesAtTheOldZonesMidnight(t *testing.T) {
	ist, _ := time.LoadLocation("Europe/Istanbul")
	kol, _ := time.LoadLocation("Asia/Kolkata")
	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	day := func(s string) time.Time { return at(s + "T00:00:00Z") }

	ahead := NewZone(kol, []ZoneChange{{Timezone: "UTC", Until: at("2026-10-13T00:00:00Z")}})
	for _, c := range []struct {
		t          string
		hour, end  string
		local, ymd string
	}{
		{"2026-10-12T23:59:00Z", "2026-10-12T23:00:00Z", "2026-10-13T00:00:00Z", "UTC", "2026-10-12"},
		{"2026-10-13T00:10:00Z", "2026-10-13T00:00:00Z", "2026-10-13T00:30:00Z", "Asia/Kolkata", "2026-10-13"},
		{"2026-10-13T00:40:00Z", "2026-10-13T00:30:00Z", "2026-10-13T01:30:00Z", "Asia/Kolkata", "2026-10-13"},
	} {
		h := ahead.LocalHour(at(c.t))
		if h != at(c.hour) || ahead.HourEnd(h) != at(c.end) || ahead.At(at(c.t)).String() != c.local || ahead.LocalDay(at(c.t)) != day(c.ymd) {
			t.Errorf("%s: hour %v to %v in %v on %v", c.t, h, ahead.HourEnd(h), ahead.At(at(c.t)), ahead.LocalDay(at(c.t)))
		}
	}
	if ahead.DayStart(day("2026-10-13")) != at("2026-10-13T00:00:00Z") || ahead.DayStart(day("2026-10-14")) != at("2026-10-13T18:30:00Z") ||
		ahead.DayStart(day("2026-10-12")) != at("2026-10-12T00:00:00Z") {
		t.Error("days around a change to a zone ahead")
	}

	behind := NewZone(time.UTC, []ZoneChange{{Timezone: "Europe/Istanbul", Until: at("2026-10-13T21:00:00Z")}})
	if behind.DayStart(day("2026-10-13")) != at("2026-10-12T21:00:00Z") || behind.DayStart(day("2026-10-14")) != at("2026-10-14T00:00:00Z") {
		t.Error("the day a change to a zone behind lengthens runs from the old zone's midnight to the new one's")
	}
	if behind.LocalDay(at("2026-10-13T22:00:00Z")) != day("2026-10-13") || behind.At(at("2026-10-13T20:59:00Z")).String() != ist.String() {
		t.Error("dates after a change to a zone behind")
	}
	if FixedZone(kol).DayStart(day("2026-10-13")) != at("2026-10-12T18:30:00Z") {
		t.Error("a fixed zone")
	}
}

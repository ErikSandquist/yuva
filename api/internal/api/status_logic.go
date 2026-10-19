package api

import (
	"html/template"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/productdevbook/yuva/api/internal/oas"
)

const (
	stOperational   = string(oas.StatusOperational)
	stDegraded      = string(oas.StatusDegraded)
	stPartialOutage = string(oas.StatusPartialOutage)
	stMajorOutage   = string(oas.StatusMajorOutage)
	stMaintenance   = string(oas.StatusMaintenance)

	kindIncident    = string(oas.StatusKindIncident)
	kindMaintenance = string(oas.StatusKindMaintenance)

	incInvestigating = string(oas.StatusInvestigating)
	incIdentified    = string(oas.StatusIdentified)
	incMonitoring    = string(oas.StatusMonitoring)
	incResolved      = string(oas.StatusResolved)
	incScheduled     = string(oas.StatusScheduled)
	incInProgress    = string(oas.StatusInProgress)
	incCompleted     = string(oas.StatusCompleted)

	maxStatusUpdateRunes = 5000
)

var (
	componentStates   = []string{stOperational, stDegraded, stPartialOutage, stMajorOutage, stMaintenance}
	incidentStateList = []string{incInvestigating, incIdentified, incMonitoring, incResolved}
	maintenanceStates = []string{incScheduled, incInProgress, incCompleted}
)

// stateRank orders component states from best to worst; maintenance counts only when nothing is worse.
func stateRank(s string) int {
	switch s {
	case stMaintenance:
		return 1
	case stDegraded:
		return 2
	case stPartialOutage:
		return 3
	case stMajorOutage:
		return 4
	}
	return 0
}

func worseState(a, b string) string {
	if stateRank(b) > stateRank(a) {
		return b
	}
	return a
}

func worstState(states []string) string {
	out := stOperational
	for _, s := range states {
		out = worseState(out, s)
	}
	return out
}

func validComponentState(s string) bool { return slices.Contains(componentStates, s) }

func incidentClosed(state string) bool { return state == incResolved || state == incCompleted }

// incidentHolds reports an incident that sets its components' states: published and open, or
// maintenance in progress.
func incidentHolds(published bool, state string) bool {
	return published && state != incScheduled && !incidentClosed(state)
}

// checkIncidentMove allows any move between an incident's states until it is resolved, and moves
// of maintenance forward only.
func checkIncidentMove(kind, from, to string) error {
	if incidentClosed(from) {
		return errIncidentClosed
	}
	if kind == kindMaintenance {
		i, j := slices.Index(maintenanceStates, from), slices.Index(maintenanceStates, to)
		if j < 0 {
			return errValidation("maintenance states are scheduled, in_progress and completed")
		}
		if j < i {
			return errValidation("maintenance cannot go back to " + to)
		}
		return nil
	}
	if !slices.Contains(incidentStateList, to) {
		return errValidation("incident states are investigating, identified, monitoring and resolved")
	}
	return nil
}

type stateSpan struct {
	state      string
	start, end time.Time
}

type uptimeDay struct {
	date   time.Time
	known  bool
	state  string
	counts time.Duration
	down   time.Duration
}

func (d uptimeDay) uptime() *float64 {
	if d.counts <= 0 {
		return nil
	}
	v := uptimePercent(d.counts, d.down)
	return &v
}

func uptimePercent(counts, down time.Duration) float64 {
	return math.Round((1-float64(down)/float64(counts))*10000) / 100
}

// uptimeDays splits the spans into the last n days of loc ending with now's day. A day's uptime is
// the share not in major_outage, partial_outage counting half, maintenance left out; its state is
// the worst state seen in it.
func uptimeDays(spans []stateSpan, n int, loc *time.Location, now time.Time) []uptimeDay {
	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	out := make([]uptimeDay, n)
	for i := range n {
		from := today.AddDate(0, 0, i-n+1)
		to := from.AddDate(0, 0, 1)
		if to.After(now) {
			to = now
		}
		d := uptimeDay{date: from, state: stOperational}
		for _, sp := range spans {
			end := sp.end
			if end.IsZero() || end.After(now) {
				end = now
			}
			a, b := maxTime(sp.start, from), minTime(end, to)
			if !b.After(a) {
				continue
			}
			d.known = true
			d.state = worseState(d.state, sp.state)
			dur := b.Sub(a)
			switch sp.state {
			case stMaintenance:
				continue
			case stMajorOutage:
				d.down += dur
			case stPartialOutage:
				d.down += dur / 2
			}
			d.counts += dur
		}
		out[i] = d
	}
	return out
}

func uptimeOver(days []uptimeDay) *float64 {
	var counts, down time.Duration
	for _, d := range days {
		counts += d.counts
		down += d.down
	}
	if counts <= 0 {
		return nil
	}
	v := uptimePercent(counts, down)
	return &v
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func statusText(raw string) (string, error) {
	s := strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	if n := len([]rune(s)); n < 1 || n > maxStatusUpdateRunes {
		return "", errValidation("body must be 1 to 5000 characters")
	}
	return s, nil
}

var statusLinkPattern = regexp.MustCompile(`https?://[^\s<>"']+`)

func richStatusText(s string) template.HTML {
	var b strings.Builder
	rest := s
	for {
		loc := statusLinkPattern.FindStringIndex(rest)
		if loc == nil {
			b.WriteString(template.HTMLEscapeString(rest))
			break
		}
		link := strings.TrimRight(rest[loc[0]:loc[1]], ".,;:!?)]")
		b.WriteString(template.HTMLEscapeString(rest[:loc[0]]))
		esc := template.HTMLEscapeString(link)
		b.WriteString(`<a href="` + esc + `" rel="nofollow noopener">` + esc + `</a>`)
		rest = rest[loc[0]+len(link):]
	}
	return template.HTML(strings.ReplaceAll(b.String(), "\n", "<br>"))
}

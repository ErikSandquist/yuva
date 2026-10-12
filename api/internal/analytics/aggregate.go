package analytics

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

// Dim is a dimension: its code in analytics_daily (fixed; stored), its name in the API, and how
// raw rows give its value. Region and city values carry the country (and region) before them,
// joined by tabs, so places of the same name stay apart.
type Dim struct {
	Code int16
	Name string
	// Group is the value over the visit alias v; empty for page dimensions.
	Group string
	// Filter is what a filter compares; Group when empty.
	Filter string
	// Page dimensions come from the events of a visit: Path is the event column.
	Page bool
	// Path values are paths, so `matches` takes a path pattern.
	Path bool
	// Codes are the stored numbers of a dimension whose values are names, so exact filters compare
	// the column itself; Column is that column.
	Codes  map[string]int16
	Column string
}

const (
	DimTotal         int16 = 0
	DimPage          int16 = 30
	DimPageByChannel int16 = 40
	DimPageBySource  int16 = 41
	DimPageByCountry int16 = 42

	OtherValue = "(other)"
)

var channelNames = map[Channel]string{
	ChannelDirect: "direct", ChannelOrganic: "organic_search", ChannelPaidSearch: "paid_search", ChannelSocial: "social",
	ChannelPaidSocial: "paid_social", ChannelEmail: "email", ChannelReferral: "referral", ChannelAIAssistants: "ai_assistants",
	ChannelInternal: "internal",
}

func ChannelName(c Channel) string { return channelNames[c] }

func codesOf(names map[int]string) map[string]int16 {
	out := map[string]int16{"": 0}
	for k, n := range names {
		out[n] = int16(k)
	}
	return out
}

var (
	deviceNames   = map[int]string{1: "desktop", 2: "mobile", 3: "tablet"}
	screenNames   = map[int]string{1: "phone", 2: "tablet", 3: "laptop", 4: "desktop"}
	platformNames = map[int]string{1: "web", 2: "ios", 3: "android", 4: "server"}
)

func channelCodes() map[string]int16 {
	m := map[int]string{}
	for c, n := range channelNames {
		m[int(c)] = n
	}
	return codesOf(m)
}

func caseOf(col string, names map[int]string) string {
	var b strings.Builder
	b.WriteString("CASE " + col)
	for k := 0; k < 16; k++ {
		if n, ok := names[k]; ok {
			fmt.Fprintf(&b, " WHEN %d THEN '%s'", k, n)
		}
	}
	b.WriteString(" ELSE '' END")
	return b.String()
}

func channelCase() string {
	m := map[int]string{}
	for c, n := range channelNames {
		m[int(c)] = n
	}
	return caseOf("v.channel", m)
}

func withVersion(name, major string) string {
	return "CASE WHEN " + name + " = '' THEN '' WHEN " + major + " = 0 THEN " + name + " ELSE " + name + " || ' ' || " + major + " END"
}

var Dims = []Dim{
	{Code: 1, Name: "entry_page", Group: "v.entry_path", Path: true},
	{Code: 2, Name: "exit_page", Group: "v.exit_path", Path: true},
	{Code: 3, Name: "hostname", Group: "v.hostname"},
	{Code: 4, Name: "channel", Group: channelCase(), Codes: channelCodes(), Column: "v.channel"},
	{Code: 5, Name: "source", Group: "v.source"},
	{Code: 6, Name: "referrer", Group: "v.referrer_host"},
	{Code: 7, Name: "utm_source", Group: "v.utm_source"},
	{Code: 8, Name: "utm_medium", Group: "v.utm_medium"},
	{Code: 9, Name: "utm_campaign", Group: "v.utm_campaign"},
	{Code: 10, Name: "utm_term", Group: "v.utm_term"},
	{Code: 11, Name: "utm_content", Group: "v.utm_content"},
	{Code: 12, Name: "country", Group: "v.country"},
	{Code: 13, Name: "region", Group: "CASE WHEN v.region = '' THEN '' ELSE v.country || E'\\t' || v.region END", Filter: "v.region"},
	{Code: 14, Name: "city", Group: "CASE WHEN v.city = '' THEN '' ELSE v.country || E'\\t' || v.region || E'\\t' || v.city END", Filter: "v.city"},
	{Code: 15, Name: "device", Group: caseOf("v.device", deviceNames), Codes: codesOf(deviceNames), Column: "v.device"},
	{Code: 16, Name: "browser", Group: "v.browser"},
	{Code: 17, Name: "browser_version", Group: withVersion("v.browser", "v.browser_major")},
	{Code: 18, Name: "os", Group: "v.os"},
	{Code: 19, Name: "os_version", Group: withVersion("v.os", "v.os_major")},
	{Code: 20, Name: "screen_size", Group: caseOf("v.screen", screenNames), Codes: codesOf(screenNames), Column: "v.screen"},
	{Code: 21, Name: "language", Group: "v.language"},
	{Code: 22, Name: "platform", Group: caseOf("v.platform", platformNames), Codes: codesOf(platformNames), Column: "v.platform"},
	{Code: 23, Name: "app_version", Group: "v.app_version"},
	{Code: DimPage, Name: "page", Page: true, Path: true},
}

// pagePairs are the page breakdowns the rollups keep per channel, source and country, so a page
// list filtered by one of them needs no raw rows.
var pagePairs = map[string]int16{"channel": DimPageByChannel, "source": DimPageBySource, "country": DimPageByCountry}

func DimByName(name string) (Dim, bool) {
	for _, d := range Dims {
		if d.Name == name {
			return d, true
		}
	}
	return Dim{}, false
}

func dimByCode(code int16) Dim {
	for _, d := range Dims {
		if d.Code == code {
			return d
		}
	}
	return Dim{}
}

func (d Dim) filterExpr() string {
	if d.Filter != "" {
		return d.Filter
	}
	return d.Group
}

// Sums are the additive numbers of a set of visits, from which every metric follows.
type Sums struct {
	Visitors, Visits, Views, Bounces, DurationS, Events, EngagedMS, EngagedViews, ScrollSum, ScrollViews int64
}

func (s *Sums) Add(o Sums) {
	s.Visitors += o.Visitors
	s.Visits += o.Visits
	s.Views += o.Views
	s.Bounces += o.Bounces
	s.DurationS += o.DurationS
	s.Events += o.Events
	s.EngagedMS += o.EngagedMS
	s.EngagedViews += o.EngagedViews
	s.ScrollSum += o.ScrollSum
	s.ScrollViews += o.ScrollViews
}

func (s *Sums) scan() []any {
	return []any{&s.Visitors, &s.Visits, &s.Views, &s.Bounces, &s.DurationS, &s.Events, &s.EngagedMS, &s.EngagedViews, &s.ScrollSum, &s.ScrollViews}
}

var Metrics = []string{"visitors", "visits", "pageviews", "views_per_visit", "bounce_rate", "visit_duration", "time_on_page", "scroll_depth", "events"}

// engagementMetrics need the engagement events of each visit.
var engagementMetrics = map[string]bool{"time_on_page": true, "scroll_depth": true}

func ratio(a, b int64, scale float64) float64 {
	if b == 0 {
		return 0
	}
	return math.Round(float64(a)/float64(b)*scale*100) / 100
}

// Metric is one metric of s: counts, or averages rounded to two decimals (bounce rate and scroll
// depth in percent, durations in seconds).
func (s Sums) Metric(name string) float64 {
	switch name {
	case "visitors":
		return float64(s.Visitors)
	case "visits":
		return float64(s.Visits)
	case "pageviews":
		return float64(s.Views)
	case "views_per_visit":
		return ratio(s.Views, s.Visits, 1)
	case "bounce_rate":
		return ratio(s.Bounces, s.Visits, 100)
	case "visit_duration":
		return ratio(s.DurationS, s.Visits, 1)
	case "time_on_page":
		return ratio(s.EngagedMS, s.EngagedViews, 0.001)
	case "scroll_depth":
		return ratio(s.ScrollSum, s.ScrollViews, 1)
	case "events":
		return float64(s.Events)
	}
	return 0
}

// The sums a query can need, in the order Sums scans them.
const (
	needVisitors = 1 << iota
	needVisits
	needViews
	needBounces
	needDuration
	needEvents
	needEngagedMS
	needEngagedViews
	needScrollSum
	needScrollViews
	needAll = 1<<iota - 1
)

var metricNeeds = map[string]int{
	"visitors": needVisitors, "visits": needVisits, "pageviews": needViews, "views_per_visit": needViews | needVisits,
	"bounce_rate": needBounces | needVisits, "visit_duration": needDuration | needVisits, "events": needEvents,
	"time_on_page": needEngagedMS | needEngagedViews, "scroll_depth": needScrollSum | needScrollViews,
}

// needsOf are the sums that metrics are computed from.
func needsOf(metrics ...[]string) int {
	n := 0
	for _, list := range metrics {
		for _, m := range list {
			n |= metricNeeds[m]
		}
	}
	return n
}

// sumsSQL lists a group's sums, with zero for those the query does not need, so a query without
// visitors counts no distinct values.
func sumsSQL(need int, exprs [10]string) string {
	out := make([]string, len(exprs))
	for i, e := range exprs {
		out[i] = "0::bigint"
		if need&(1<<i) != 0 {
			out[i] = e
		}
	}
	return strings.Join(out, ", ")
}

func (s *scope) sums(alias string) string {
	return sumsSQL(s.need, [10]string{"count(DISTINCT v.visitor)", "count(*)", "coalesce(sum(" + alias + ".views), 0)::bigint",
		"count(*) FILTER (WHERE v.bounce)", "coalesce(sum(v.dur), 0)::bigint", "coalesce(sum(" + alias + ".events), 0)::bigint",
		"coalesce(sum(" + alias + ".eng_ms), 0)::bigint", "coalesce(sum(" + alias + ".eng_views), 0)::bigint",
		"coalesce(sum(" + alias + ".scroll_sum), 0)::bigint", "coalesce(sum(" + alias + ".scroll_views), 0)::bigint"})
}

// scope is the rows an aggregation reads: one property's days from to, with filters as SQL over the
// visit alias v and, for page filters, over a page's path. args starts with workspace, property,
// from, to.
type scope struct {
	args       []any
	visitWhere []string
	pathWhere  []string
	engagement bool
	need       int
}

func newScope(ws, property any, from, to time.Time) *scope {
	return &scope{args: []any{ws, property, from, to}, need: needAll}
}

// onlyDays narrows the scope to some of its days.
func (s *scope) onlyDays(days []time.Time) {
	a := s.arg(days)
	s.visitWhere = append(s.visitWhere, "v.day = ANY("+a+"::date[])")
	s.pathWhere = append(s.pathWhere, "day = ANY("+a+"::date[])")
}

func (s *scope) arg(v any) string {
	s.args = append(s.args, v)
	return fmt.Sprintf("$%d", len(s.args))
}

// visitsCTE is the property's visits without bots, with each visit's bounce, duration and (when
// asked) engagement.
func (s *scope) visitsCTE() string {
	eng := "0::bigint AS eng_ms, 0::bigint AS eng_views, 0::bigint AS scroll_sum, 0::bigint AS scroll_views"
	from := "analytics_visits v"
	if s.engagement {
		eng = "coalesce(ev.eng_ms, 0) AS eng_ms, coalesce(ev.eng_views, 0) AS eng_views, coalesce(ev.scroll_sum, 0) AS scroll_sum, coalesce(ev.scroll_views, 0) AS scroll_views"
		from = `analytics_visits v LEFT JOIN (
    SELECT day, visit, sum(eng_ms) AS eng_ms, sum(eng_views) AS eng_views, sum(scroll_sum) AS scroll_sum, sum(scroll_views) AS scroll_views
    FROM analytics_visit_pages WHERE workspace_id = $1 AND property_id = $2 AND day BETWEEN $3 AND $4 AND eng_views > 0
    GROUP BY day, visit) ev ON ev.day = v.day AND ev.visit = v.visit`
	}
	where := append([]string{"v.workspace_id = $1", "v.property_id = $2", "v.day BETWEEN $3 AND $4", "NOT v.bot"}, s.visitWhere...)
	return `vv AS (
SELECT v.day, v.visit, v.visitor, v.started_at, v.views, v.events, v.views <= 1 AND NOT v.interactive AS bounce,
    extract(epoch FROM v.ended_at - v.started_at)::bigint AS dur, ` + eng + `,
    v.entry_path, v.exit_path, v.hostname, v.channel, v.source, v.referrer_host, v.utm_source, v.utm_medium, v.utm_campaign,
    v.utm_term, v.utm_content, v.country, v.region, v.city, v.device, v.browser, v.browser_major, v.os, v.os_major, v.screen,
    v.language, v.platform, v.app_version
FROM ` + from + `
WHERE ` + strings.Join(where, " AND ") + `)`
}

// pagesCTE is each visit's numbers per path.
func (s *scope) pagesCTE() string {
	where := append([]string{"workspace_id = $1", "property_id = $2", "day BETWEEN $3 AND $4"}, s.pathWhere...)
	return `pv AS (
SELECT day, visit, path, views, events, eng_ms, eng_views, scroll_sum, scroll_views
FROM analytics_visit_pages WHERE ` + strings.Join(where, " AND ") + `)`
}

// visitRowsSQL: per day, the visits' sums (dimension 0) and, with groups, per combined value of
// the groups (dimension 1, values joined by a unit separator), in one pass.
func (s *scope) visitRowsSQL(groups []string) string {
	if len(groups) == 0 {
		return "WITH " + s.visitsCTE() + `
SELECT v.day, 0::smallint, '', ` + s.sums("v") + `
FROM vv v GROUP BY v.day`
	}
	sel := make([]string, len(groups))
	for i, g := range groups {
		sel[i] = "(" + g + ")"
	}
	return "WITH " + s.visitsCTE() + `
SELECT v.day, (CASE WHEN GROUPING(` + sel[0] + `) = 0 THEN 1 ELSE 0 END)::smallint,
    CASE WHEN GROUPING(` + sel[0] + `) = 0 THEN ` + strings.Join(sel, " || E'\\x1f' || ") + ` ELSE '' END, ` + s.sums("v") + `
FROM vv v GROUP BY GROUPING SETS ((v.day), (v.day, ` + strings.Join(sel, ", ") + `))`
}

// visitSetsSQL: per day, dimension and value, the visits' sums, for each of dims in one pass;
// with totals, dimension 0 holds the day's totals.
func (s *scope) visitSetsSQL(dims []Dim, totals bool) string {
	var sets, codes, values []string
	if totals {
		sets = append(sets, "(v.day)")
	}
	for _, d := range dims {
		e := "(" + d.Group + ")"
		sets = append(sets, "(v.day, "+e+")")
		codes = append(codes, fmt.Sprintf("WHEN GROUPING(%s) = 0 THEN %d", e, d.Code))
		values = append(values, fmt.Sprintf("WHEN GROUPING(%s) = 0 THEN %s", e, e))
	}
	return "WITH " + s.visitsCTE() + `
SELECT v.day, (CASE ` + strings.Join(codes, " ") + ` ELSE 0 END)::smallint, CASE ` + strings.Join(values, " ") + ` ELSE '' END,
    ` + s.sums("v") + `
FROM vv v GROUP BY GROUPING SETS (` + strings.Join(sets, ", ") + `)`
}

// pageSetsSQL: per day and path, the sums of the visits that have the path with the path's own
// views, events and engagement; with pairs, also per path and channel, source and country (the
// value then is the path, a tab and the other value).
func (s *scope) pageSetsSQL(pairs bool) string {
	if !pairs {
		return "WITH " + s.visitsCTE() + ", " + s.pagesCTE() + `
SELECT p.day, ` + fmt.Sprint(DimPage) + `::smallint, p.path, ` + s.sums("p") + `
FROM pv p JOIN vv v ON v.day = p.day AND v.visit = p.visit
GROUP BY p.day, p.path`
	}
	channel, _ := DimByName("channel")
	ch := "(" + channel.Group + ")"
	return "WITH " + s.visitsCTE() + ", " + s.pagesCTE() + fmt.Sprintf(`
SELECT p.day, (CASE WHEN GROUPING(%[1]s) = 0 THEN %[2]d WHEN GROUPING(v.source) = 0 THEN %[3]d WHEN GROUPING(v.country) = 0 THEN %[4]d ELSE %[5]d END)::smallint,
    p.path || CASE WHEN GROUPING(%[1]s) = 0 THEN E'\t' || %[1]s WHEN GROUPING(v.source) = 0 THEN E'\t' || v.source
        WHEN GROUPING(v.country) = 0 THEN E'\t' || v.country ELSE '' END,
    `, ch, DimPageByChannel, DimPageBySource, DimPageByCountry, DimPage) + s.sums("p") + `
FROM pv p JOIN vv v ON v.day = p.day AND v.visit = p.visit
GROUP BY GROUPING SETS ((p.day, p.path), (p.day, p.path, ` + ch + `), (p.day, p.path, v.source), (p.day, p.path, v.country))`
}

// pagePairSQL: per day and path, the sums of the visits that have the path, with a visit grouping
// after a unit separator.
func (s *scope) pagePairSQL(other string) string {
	return "WITH " + s.visitsCTE() + ", " + s.pagesCTE() + `
SELECT p.day, 0::smallint, p.path || E'\x1f' || (` + other + `), ` + s.sums("p") + `
FROM pv p JOIN vv v ON v.day = p.day AND v.visit = p.visit
GROUP BY p.day, p.path, (` + other + `)`
}

// hourlySQL: per day and local hour, the distinct visitors and visits with an event in the hour,
// its page views and events.
func (s *scope) hourlySQL(tz string) string {
	return `SELECT day, hour, count(*), sum(visits)::bigint, sum(views)::bigint, sum(events)::bigint FROM (
SELECT day, hour, visitor, count(*) AS visits, sum(views) AS views, sum(events) AS events FROM (
WITH ` + s.visitsCTE() + `
SELECT e.day, extract(hour FROM e.ts AT TIME ZONE ` + s.arg(tz) + `)::int AS hour, e.visitor, e.visit,
    count(*) FILTER (WHERE e.kind = 1) AS views, count(*) FILTER (WHERE e.kind = 3) AS events
FROM analytics_events e JOIN vv v ON v.day = e.day AND v.visit = e.visit
WHERE e.workspace_id = $1 AND e.property_id = $2 AND e.day BETWEEN $3 AND $4
GROUP BY e.day, hour, e.visitor, e.visit) AS per_visit
GROUP BY day, hour, visitor) AS per_visitor
GROUP BY day, hour`
}

// Filter narrows a query: the dimension's value is (is) or is not (is_not) one of Values,
// contains or does not contain one of them, or matches one of their patterns.
type Filter struct {
	Dimension string
	Op        string
	Values    []string
}

var regexMeta = regexp.MustCompile(`[\\.+?()|\[\]{}^$]`)

// patternRegex turns a pattern into an anchored regular expression: on paths `*` is one segment
// and `**` any number of them; elsewhere `*` is any text.
func patternRegex(pattern string, path bool) string {
	var b strings.Builder
	b.WriteString("^")
	rest := pattern
	for rest != "" {
		switch {
		case path && strings.HasPrefix(rest, "/**"):
			b.WriteString("(/.*)?")
			rest = rest[3:]
		case path && strings.HasPrefix(rest, "**"):
			b.WriteString(".*")
			rest = rest[2:]
		case strings.HasPrefix(rest, "*"):
			if path {
				b.WriteString("[^/]*")
			} else {
				b.WriteString(".*")
			}
			rest = rest[1:]
		default:
			b.WriteString(regexMeta.ReplaceAllString(rest[:1], `\$0`))
			rest = rest[1:]
		}
	}
	if path {
		b.WriteString("/?")
	}
	b.WriteString("$")
	return b.String()
}

// patternPrefix is the text every value a pattern matches starts with, checked before the regular
// expression because it is much cheaper; on paths a `/**` may match nothing, so its slash is left out.
func patternPrefix(pattern string, path bool) string {
	prefix, _, wild := strings.Cut(pattern, "*")
	if wild && path {
		prefix = strings.TrimSuffix(prefix, "/")
	}
	return prefix
}

func (s *scope) cond(expr string, f Filter, path bool) string {
	switch f.Op {
	case "is":
		return "(" + expr + ") = ANY(" + s.arg(f.Values) + "::text[])"
	case "is_not":
		return "NOT ((" + expr + ") = ANY(" + s.arg(f.Values) + "::text[]))"
	case "contains", "not_contains":
		parts := make([]string, len(f.Values))
		for i, v := range f.Values {
			parts[i] = "strpos(lower(" + expr + "), " + s.arg(strings.ToLower(v)) + ") > 0"
		}
		c := "(" + strings.Join(parts, " OR ") + ")"
		if f.Op == "not_contains" {
			return "NOT " + c
		}
		return c
	case "matches":
		parts := make([]string, len(f.Values))
		for i, v := range f.Values {
			parts[i] = "(" + expr + ") ~ " + s.arg(patternRegex(v, path))
			if prefix := patternPrefix(v, path); prefix != "" {
				parts[i] = "starts_with(" + expr + ", " + s.arg(prefix) + ") AND " + parts[i]
			}
		}
		return "(" + strings.Join(parts, " OR ") + ")"
	}
	return "false"
}

// addFilter narrows the visits; a page filter keeps the visits with a matching page view and, when
// pages are broken down, only the matching pages.
func (s *scope) addFilter(f Filter) error {
	d, ok := DimByName(f.Dimension)
	if !ok {
		return fmt.Errorf("unknown dimension %q", f.Dimension)
	}
	if d.Page {
		c := s.cond("\x00path", f, true)
		s.visitWhere = append(s.visitWhere, `(v.day, v.visit) IN (SELECT pe.day, pe.visit FROM analytics_visit_pages pe
    WHERE pe.workspace_id = $1 AND pe.property_id = $2 AND pe.day BETWEEN $3 AND $4 AND pe.views > 0 AND `+strings.ReplaceAll(c, "\x00path", "pe.path")+")")
		s.pathWhere = append(s.pathWhere, strings.ReplaceAll(c, "\x00path", "path"))
		return nil
	}
	if d.Codes != nil && (f.Op == "is" || f.Op == "is_not") {
		codes := []int16{}
		for _, v := range f.Values {
			if c, ok := d.Codes[v]; ok {
				codes = append(codes, c)
			}
		}
		c := d.Column + " = ANY(" + s.arg(codes) + "::smallint[])"
		if f.Op == "is_not" {
			c = "NOT " + c
		}
		s.visitWhere = append(s.visitWhere, c)
		return nil
	}
	s.visitWhere = append(s.visitWhere, s.cond(d.filterExpr(), f, d.Path))
	return nil
}

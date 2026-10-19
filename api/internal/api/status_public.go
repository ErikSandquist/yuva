package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"uuid"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	statusPathPrefix   = "/status/"
	clientStatusPrefix = "/client/v1/status/"
	statusCacheTTL     = 30 * time.Second
	statusMaxCached    = 5000
	statusRecentDays   = 14
	statusPublicMax    = 500
	statusFeedEntries  = 50
	statusCacheControl = "public, max-age=30, stale-if-error=86400"
)

var limitStatusPerIP = limit{120, time.Minute}

type statusCacheEntry struct {
	changed, at time.Time
	ctype       string
	body        []byte
}

// statusCache keeps rendered public answers per page, view and language; an entry is used only while
// the page's changed_at is the one it was made from and for at most statusCacheTTL, since day bars and
// relative times move on their own.
type statusCache struct {
	mu      sync.Mutex
	entries map[string]statusCacheEntry
}

func newStatusCache() *statusCache { return &statusCache{entries: map[string]statusCacheEntry{}} }

func (c *statusCache) get(key string, changed, now time.Time) (statusCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || !e.changed.Equal(changed) || now.Sub(e.at) >= statusCacheTTL || now.Before(e.at) {
		return e, false
	}
	return e, true
}

func (c *statusCache) put(key string, e statusCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= statusMaxCached {
		c.entries = map[string]statusCacheEntry{}
	}
	c.entries[key] = e
}

func (c *statusCache) forget(page uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prefix := page.String() + "|"
	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
}

type statusHostKey struct{}

// statusOn is the context value of a request that came to the status host or to an own domain.
type statusOn struct{ domain bool }

// statusHost serves the status pages at /{slug} on YUVA_STATUS_URL's host and nothing else there,
// and a page at / on its verified own domain and nothing else there.
func (s *Server) statusHost(next http.Handler) http.Handler {
	var statusHost string
	if u, err := url.Parse(s.status.URL); s.status.URL != "" && err == nil {
		statusHost = u.Host
	}
	var publicHost string
	if u, err := url.Parse(s.auth.PublicURL); err == nil {
		publicHost = u.Host
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case statusHost != "" && strings.EqualFold(r.Host, statusHost):
			if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, statusPathPrefix) || strings.Count(r.URL.Path, "/") > 3 {
				writeProblem(w, errNotFound)
				return
			}
			r2 := r.Clone(context.WithValue(r.Context(), statusHostKey{}, statusOn{}))
			r2.URL.Path = strings.TrimSuffix(statusPathPrefix, "/") + r.URL.Path
			r2.URL.RawPath = ""
			next.ServeHTTP(w, r2)
		case s.statusDomainsOn() && !strings.EqualFold(r.Host, publicHost):
			host := strings.ToLower(r.Host)
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			slug, ok := s.domainPage(r.Context(), strings.TrimSuffix(host, "."))
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			path, ok := statusDomainPath(slug, r.URL.Path)
			if !ok {
				writeProblem(w, errNotFound)
				return
			}
			r2 := r.Clone(context.WithValue(r.Context(), statusHostKey{}, statusOn{domain: true}))
			r2.URL.Path = path
			r2.URL.RawPath = ""
			next.ServeHTTP(w, r2)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func (s *Server) routeStatus(mux *http.ServeMux) {
	mux.HandleFunc("GET "+statusPathPrefix+"{slug}", s.serveStatus("page"))
	mux.HandleFunc("GET "+statusPathPrefix+"{slug}/history", s.serveStatus("history"))
	mux.HandleFunc("GET "+statusPathPrefix+"{slug}/incidents/{id}", s.serveStatus("incident"))
	mux.HandleFunc("GET "+statusPathPrefix+"{slug}/feed.atom", s.serveStatus("feed"))
	mux.HandleFunc("GET "+statusPathPrefix+"{slug}/badge.svg", s.serveStatus("badge"))
	mux.HandleFunc("GET "+statusPathPrefix+"{slug}/subscribe", s.serveStatusSubscribe)
	mux.HandleFunc("POST "+statusPathPrefix+"{slug}/subscribe", s.serveStatusSubscribe)
	mux.HandleFunc("GET "+statusPathPrefix+"{slug}/subscribe/sent", s.serveStatusSubscribe)
}

func (s *Server) publicStatusPage(w http.ResponseWriter, r *http.Request) (store.StatusPage, bool) {
	ctx := r.Context()
	slug := r.PathValue("slug")
	redirect := http.StatusFound
	if r.Method != http.MethodGet {
		redirect = http.StatusTemporaryRedirect
	}
	if s.status.URL != "" && ctx.Value(statusHostKey{}) == nil {
		target := s.statusURL(slug, strings.TrimPrefix(r.URL.Path, statusPathPrefix+slug))
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, redirect)
		return store.StatusPage{}, false
	}
	if err := s.rateLimit(rateCheck{"status:ip:" + rateIP(s.clientIP(r)), limitStatusPerIP}); err != nil {
		writeProblem(w, err.(*apiError))
		return store.StatusPage{}, false
	}
	pg, err := s.st.FindStatusPageBySlug(ctx, slug)
	if store.IsNotFound(err) {
		pg, err = s.st.FindStatusPageByFormerSlug(ctx, slug)
		if err == nil && pg.Published {
			target := s.pageURL(pg, strings.TrimPrefix(r.URL.Path, statusPathPrefix+slug))
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			if redirect == http.StatusFound {
				redirect = http.StatusMovedPermanently
			} else {
				redirect = http.StatusPermanentRedirect
			}
			http.Redirect(w, r, target, redirect)
			return pg, false
		}
	}
	if store.IsNotFound(err) || (err == nil && !pg.Published) {
		s.statusGone(w)
		return pg, false
	}
	if err != nil {
		s.log.ErrorContext(ctx, "status page", slog.Any("error", err))
		writeProblem(w, errInternal)
		return pg, false
	}
	return pg, true
}

func (s *Server) serveStatus(view string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		pg, ok := s.publicStatusPage(w, r)
		if !ok {
			return
		}
		var err error
		var incident uuid.UUID
		if view == "incident" {
			if incident, err = uuid.Parse(r.PathValue("id")); err != nil {
				s.statusGone(w)
				return
			}
		}
		e, err := s.statusAnswer(ctx, pg, view, incident)
		if err == errStatusIncidentGone {
			s.statusGone(w)
			return
		}
		if err != nil {
			s.log.ErrorContext(ctx, "status page", slog.Any("error", err))
			writeProblem(w, errInternal)
			return
		}
		writeStatusAnswer(w, e)
	}
}

func (s *Server) statusGone(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("Cache-Control", "public, max-age=30")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(statusWordsFor("en").Gone + "\n"))
}

var (
	statusScript     = `document.querySelectorAll("time[data-local]").forEach(function(t){var d=new Date(t.getAttribute("datetime"));if(!isNaN(d))t.textContent=d.toLocaleString(document.documentElement.lang,{dateStyle:"medium",timeStyle:"short"})});setTimeout(function(){location.reload()},60000)`
	statusScriptHash = func() string {
		h := sha256.Sum256([]byte(statusScript))
		return base64.StdEncoding.EncodeToString(h[:])
	}()
	statusPageCSP = "default-src 'none'; style-src 'unsafe-inline'; img-src https: data:; script-src 'sha256-" + statusScriptHash + "'; frame-ancestors *; base-uri 'none'; form-action 'none'"
)

func writeStatusAnswer(w http.ResponseWriter, e statusCacheEntry) {
	h := w.Header()
	h.Set("Content-Type", e.ctype)
	h.Set("Cache-Control", statusCacheControl)
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	switch {
	case strings.HasPrefix(e.ctype, "text/html"):
		h.Set("Content-Security-Policy", statusPageCSP)
	case strings.HasPrefix(e.ctype, "application/json"):
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	default:
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(e.body)
}

func (s *Server) statusAnswer(ctx context.Context, pg store.StatusPage, view string, incident uuid.UUID) (statusCacheEntry, error) {
	now := s.now()
	key := pg.ID.String() + "|" + view + "|" + incident.String() + "|" + statusWordsFor(pg.Locale).Lang
	if e, ok := s.statusCache.get(key, pg.ChangedAt, now); ok {
		return e, nil
	}
	var inc store.StatusIncident
	if view == "incident" {
		var err error
		inc, err = s.st.GetStatusIncident(ctx, store.GetStatusIncidentParams{WorkspaceID: pg.WorkspaceID, PageID: pg.ID, ID: incident})
		if store.IsNotFound(err) || (err == nil && inc.PublishedAt == nil) {
			return statusCacheEntry{}, errStatusIncidentGone
		}
		if err != nil {
			return statusCacheEntry{}, err
		}
	}
	snap, err := s.statusSnapshot(ctx, s.st.Queries, pg, now)
	if err != nil {
		return statusCacheEntry{}, err
	}
	e := statusCacheEntry{changed: pg.ChangedAt, at: now}
	switch view {
	case "json":
		e.ctype = "application/json"
		e.body, err = json.Marshal(snap.out)
	case "feed":
		e.ctype = "application/atom+xml; charset=utf-8"
		e.body, err = s.statusFeed(snap)
	case "badge":
		e.ctype = "image/svg+xml"
		e.body = statusBadge(snap)
	case "incident":
		var parts incidentParts
		if parts, err = loadIncidentParts(ctx, s.st.Queries, pg.WorkspaceID, []store.StatusIncident{inc}); err != nil {
			return e, err
		}
		v := s.statusView(snap, "incident")
		iv := snap.incidentView(inc, parts)
		v.Incident = &iv
		e.ctype = "text/html; charset=utf-8"
		e.body, err = renderStatus(v)
	default:
		e.ctype = "text/html; charset=utf-8"
		e.body, err = renderStatus(s.statusView(snap, view))
	}
	if err != nil {
		return e, err
	}
	s.statusCache.put(key, e)
	return e, nil
}

type statusSnap struct {
	page      store.StatusPage
	words     statusWords
	loc       *time.Location
	now       time.Time
	out       oas.PublicStatusPage
	groups    []store.StatusComponentGroup
	comps     []store.StatusComponent
	days      map[uuid.UUID][]uptimeDay
	uptime    map[uuid.UUID]*float64
	incidents []store.StatusIncident
	parts     incidentParts
	visible   map[uuid.UUID]bool
	url       func(path string) string
}

func (s *Server) statusSnapshot(ctx context.Context, q *store.Queries, pg store.StatusPage, now time.Time) (*statusSnap, error) {
	ws := pg.WorkspaceID
	snap := &statusSnap{page: pg, words: statusWordsFor(pg.Locale), loc: propertyLocation(pg.Timezone), now: now,
		days: map[uuid.UUID][]uptimeDay{}, uptime: map[uuid.UUID]*float64{}, visible: map[uuid.UUID]bool{},
		url: func(path string) string { return s.pageURL(pg, path) }}
	groups, err := q.ListStatusGroups(ctx, store.ListStatusGroupsParams{WorkspaceID: ws, PageID: pg.ID})
	if err != nil {
		return nil, err
	}
	all, err := q.ListStatusComponents(ctx, store.ListStatusComponentsParams{WorkspaceID: ws, PageID: pg.ID})
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	for _, c := range all {
		if !c.Hidden {
			snap.comps = append(snap.comps, c)
			snap.visible[c.ID] = true
			ids = append(ids, c.ID)
		}
	}
	for _, g := range groups {
		if slices.ContainsFunc(snap.comps, func(c store.StatusComponent) bool { return c.GroupID != nil && *c.GroupID == g.ID }) {
			snap.groups = append(snap.groups, g)
		}
	}
	window := now.AddDate(0, 0, -int(pg.HistoryDays)-1)
	if pg.ShowUptime && len(ids) > 0 {
		spans, err := q.StatusComponentSpans(ctx, store.StatusComponentSpansParams{WorkspaceID: ws, ComponentIds: ids, Since: window})
		if err != nil {
			return nil, err
		}
		by := map[uuid.UUID][]stateSpan{}
		for _, sp := range spans {
			var end time.Time
			if sp.EndedAt != nil {
				end = *sp.EndedAt
			}
			by[sp.ComponentID] = append(by[sp.ComponentID], stateSpan{state: sp.State, start: sp.StartedAt, end: end})
		}
		for _, id := range ids {
			snap.days[id] = uptimeDays(by[id], int(pg.HistoryDays), snap.loc, now)
			snap.uptime[id] = uptimeOver(snap.days[id])
		}
	}
	if snap.incidents, err = q.ListPublicStatusIncidents(ctx, store.ListPublicStatusIncidentsParams{WorkspaceID: ws, PageID: pg.ID, Since: window, Lim: statusPublicMax}); err != nil {
		return nil, err
	}
	if snap.parts, err = loadIncidentParts(ctx, q, ws, snap.incidents); err != nil {
		return nil, err
	}
	snap.out = snap.json()
	return snap, nil
}

func (snap *statusSnap) publicIncident(inc store.StatusIncident, parts incidentParts) oas.PublicStatusIncident {
	out := oas.PublicStatusIncident{Id: inc.ID, Kind: oas.StatusIncidentKind(inc.Kind), Title: inc.Title, Impact: oas.StatusIncidentImpact(inc.Impact),
		State: oas.StatusIncidentState(inc.State), Components: []uuid.UUID{}, Updates: []oas.PublicStatusUpdate{},
		Url: snap.url("/incidents/" + inc.ID.String()), PublishedAt: *inc.PublishedAt, ScheduledStart: inc.ScheduledStart,
		ScheduledEnd: inc.ScheduledEnd, ResolvedAt: inc.ResolvedAt, UpdatedAt: inc.UpdatedAt}
	for _, c := range parts.components[inc.ID] {
		if snap.visible[c.ComponentID] {
			out.Components = append(out.Components, c.ComponentID)
		}
	}
	for _, u := range parts.updates[inc.ID] {
		out.Updates = append(out.Updates, oas.PublicStatusUpdate{State: oas.StatusIncidentState(u.State), Body: u.Body, CreatedAt: u.CreatedAt, EditedAt: u.EditedAt})
	}
	return out
}

func (snap *statusSnap) json() oas.PublicStatusPage {
	pg := snap.page
	out := oas.PublicStatusPage{Name: pg.Name, Description: pg.Description, Url: snap.url(""), LogoUrl: pg.LogoUrl, AccentColor: pg.AccentColor,
		SiteUrl: pg.SiteUrl, SupportUrl: pg.SupportUrl, Locale: pg.Locale, Timezone: pg.Timezone, State: oas.StatusComponentStateValue(pageState(snap.comps)),
		Groups: []oas.PublicStatusGroup{}, Components: []oas.PublicStatusComponent{}, Open: []oas.PublicStatusIncident{}, Recent: []oas.PublicStatusIncident{},
		UpdatedAt: pg.ChangedAt}
	for _, g := range snap.groups {
		out.Groups = append(out.Groups, oas.PublicStatusGroup{Id: g.ID, Name: g.Name})
	}
	for _, c := range snap.comps {
		pc := oas.PublicStatusComponent{Id: c.ID, Name: c.Name, Description: c.Description, GroupId: c.GroupID, State: oas.StatusComponentStateValue(c.State)}
		if pg.ShowUptime {
			pc.Uptime = snap.uptime[c.ID]
			days := []oas.PublicStatusDay{}
			for _, d := range snap.days[c.ID] {
				st := d.state
				if !d.known {
					continue
				}
				days = append(days, oas.PublicStatusDay{Date: openapi_types.Date{Time: d.date}, Uptime: d.uptime(), State: oas.StatusComponentStateValue(st)})
			}
			pc.Days = &days
		}
		out.Components = append(out.Components, pc)
	}
	recentFrom := snap.now.AddDate(0, 0, -statusRecentDays)
	for _, inc := range snap.incidents {
		switch {
		case !incidentClosed(inc.State):
			out.Open = append(out.Open, snap.publicIncident(inc, snap.parts))
		case inc.ResolvedAt != nil && inc.ResolvedAt.After(recentFrom):
			out.Recent = append(out.Recent, snap.publicIncident(inc, snap.parts))
		}
	}
	slices.SortStableFunc(out.Open, func(a, b oas.PublicStatusIncident) int {
		if (a.State == oas.StatusScheduled) != (b.State == oas.StatusScheduled) {
			if a.State == oas.StatusScheduled {
				return 1
			}
			return -1
		}
		return 0
	})
	return out
}

func (s *Server) GetClientStatusPage(ctx context.Context, req oas.GetClientStatusPageRequestObject) (oas.GetClientStatusPageResponseObject, error) {
	r := requestFrom(ctx)
	if err := s.rateLimit(rateCheck{"status:ip:" + rateIP(s.clientIP(r)), limitStatusPerIP}); err != nil {
		return nil, err
	}
	pg, err := s.st.FindStatusPageBySlug(ctx, req.Slug)
	if store.IsNotFound(err) {
		pg, err = s.st.FindStatusPageByFormerSlug(ctx, req.Slug)
		if err == nil && pg.Published {
			target := s.pageURL(pg, "")
			return oas.GetClientStatusPage301Response{Headers: oas.GetClientStatusPage301ResponseHeaders{Location: &target}}, nil
		}
	}
	if store.IsNotFound(err) || (err == nil && !pg.Published) {
		return nil, errStatusPageGone
	}
	if err != nil {
		return nil, err
	}
	e, err := s.statusAnswer(ctx, pg, "json", uuid.Nil())
	if err != nil {
		return nil, err
	}
	return statusAnswerResponse(e), nil
}

type statusAnswerResponse statusCacheEntry

func (a statusAnswerResponse) VisitGetClientStatusPageResponse(w http.ResponseWriter) error {
	writeStatusAnswer(w, statusCacheEntry(a))
	return nil
}

type statusPreviewResponse []byte

func (b statusPreviewResponse) VisitPreviewStatusPageResponse(w http.ResponseWriter) error {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Security-Policy", strings.Replace(statusPageCSP, "frame-ancestors *", "frame-ancestors 'self'", 1))
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(b)
	return err
}

func (s *Server) PreviewStatusPage(ctx context.Context, req oas.PreviewStatusPageRequestObject) (oas.PreviewStatusPageResponseObject, error) {
	p := principalFrom(ctx)
	pg, err := visibleStatusPage(ctx, s.st.Queries, p, req.PageId, false)
	if err != nil {
		return nil, err
	}
	snap, err := s.statusSnapshot(ctx, s.st.Queries, pg, s.now())
	if err != nil {
		return nil, err
	}
	body, err := renderStatus(s.statusView(snap, "page"))
	if err != nil {
		return nil, err
	}
	return statusPreviewResponse(body), nil
}

var statusColors = map[string]string{stOperational: "#16a34a", stDegraded: "#ca8a04", stPartialOutage: "#ea580c", stMajorOutage: "#dc2626", stMaintenance: "#2563eb", "": "#9ca3af"}

type timeView struct{ ISO, Text string }

type updateView struct {
	State  string
	Body   template.HTML
	At     timeView
	Edited bool
}

type incidentView struct {
	URL, Title, State, StateClass, Impact, Affects string
	Maintenance, Closed                            bool
	Start, End                                     *timeView
	Updates                                        []updateView
	When                                           timeView
}

type dayView struct{ Color, Title string }

type componentView struct {
	Name, Description, State, Color, Uptime string
	Days                                    []dayView
}

type groupView struct {
	Name       string
	Components []componentView
}

type monthView struct {
	Name      string
	Incidents []incidentView
}

type statusPageView struct {
	W                                            statusWords
	View, Name, Description, Logo, Site, Support string
	Color                                        template.CSS
	Home, HistoryURL, FeedURL, SubscribeURL      string
	State, StateColor, Headline                  string
	Open, Recent                                 []incidentView
	Groups                                       []groupView
	Months                                       []monthView
	NoHistory, OldestDay                         string
	Incident                                     *incidentView
	Updated                                      timeView
	Script                                       template.JS
}

func (snap *statusSnap) at(t time.Time) timeView {
	return timeView{ISO: t.UTC().Format(time.RFC3339), Text: snap.words.moment(t.In(snap.loc))}
}

func (snap *statusSnap) incidentView(inc store.StatusIncident, parts incidentParts) incidentView {
	w := snap.words
	v := incidentView{URL: snap.url("/incidents/" + inc.ID.String()), Title: inc.Title, State: w.IncidentStates[inc.State], StateClass: inc.State,
		Impact: w.Impacts[inc.Impact], Maintenance: inc.Kind == kindMaintenance, Closed: incidentClosed(inc.State), When: snap.at(*inc.PublishedAt)}
	if inc.ScheduledStart != nil {
		a, b := snap.at(*inc.ScheduledStart), snap.at(*inc.ScheduledEnd)
		v.Start, v.End = &a, &b
		v.When = a
	}
	var names []string
	for _, c := range parts.components[inc.ID] {
		if i := slices.IndexFunc(snap.comps, func(x store.StatusComponent) bool { return x.ID == c.ComponentID }); i >= 0 {
			names = append(names, snap.comps[i].Name)
		}
	}
	v.Affects = strings.Join(names, ", ")
	for _, u := range parts.updates[inc.ID] {
		v.Updates = append(v.Updates, updateView{State: w.IncidentStates[u.State], Body: richStatusText(u.Body), At: snap.at(u.CreatedAt), Edited: u.EditedAt != nil})
	}
	return v
}

func (s *Server) statusView(snap *statusSnap, view string) statusPageView {
	pg, w := snap.page, snap.words
	state := string(snap.out.State)
	v := statusPageView{W: w, View: view, Name: pg.Name, Description: pg.Description, Color: defaultBrandColor, Home: snap.url(""),
		HistoryURL: snap.url("/history"), FeedURL: snap.url("/feed.atom"), State: w.States[state], StateColor: statusColors[state],
		Headline: w.Headlines[state], Updated: snap.at(pg.ChangedAt), Script: template.JS(statusScript)}
	if pg.AccentColor != nil {
		v.Color = template.CSS(*pg.AccentColor)
	}
	if s.statusEmailOpen(pg) {
		v.SubscribeURL = snap.url("/subscribe")
	}
	if pg.LogoUrl != nil {
		v.Logo = *pg.LogoUrl
	}
	if pg.SiteUrl != nil {
		v.Site = *pg.SiteUrl
	}
	if pg.SupportUrl != nil {
		v.Support = *pg.SupportUrl
	}
	recentFrom := snap.now.AddDate(0, 0, -statusRecentDays)
	for _, inc := range snap.incidents {
		switch {
		case !incidentClosed(inc.State):
			v.Open = append(v.Open, snap.incidentView(inc, snap.parts))
		case inc.ResolvedAt != nil && inc.ResolvedAt.After(recentFrom):
			v.Recent = append(v.Recent, snap.incidentView(inc, snap.parts))
		}
	}
	group := func(id *uuid.UUID) []componentView {
		var out []componentView
		for _, c := range snap.comps {
			if (id == nil) != (c.GroupID == nil) || (id != nil && *id != *c.GroupID) {
				continue
			}
			cv := componentView{Name: c.Name, Description: c.Description, State: w.States[c.State], Color: statusColors[c.State]}
			if pg.ShowUptime {
				if u := snap.uptime[c.ID]; u != nil {
					cv.Uptime = strings.Replace(w.Uptime, "%s", fmt.Sprintf("%.2f%%", *u), 1)
				}
				for _, d := range snap.days[c.ID] {
					title := w.day(d.date) + ": " + w.NoData
					color := statusColors[""]
					if d.known {
						color = statusColors[d.state]
						title = w.day(d.date) + ": " + w.States[d.state]
						if u := d.uptime(); u != nil {
							title += fmt.Sprintf(" (%.2f%%)", *u)
						}
					}
					cv.Days = append(cv.Days, dayView{Color: color, Title: title})
				}
			}
			out = append(out, cv)
		}
		return out
	}
	if ungrouped := group(nil); len(ungrouped) > 0 {
		v.Groups = append(v.Groups, groupView{Components: ungrouped})
	}
	for _, g := range snap.groups {
		id := g.ID
		v.Groups = append(v.Groups, groupView{Name: g.Name, Components: group(&id)})
	}
	if view == "history" {
		for _, inc := range snap.incidents {
			iv := snap.incidentView(inc, snap.parts)
			t := *inc.PublishedAt
			if inc.ScheduledStart != nil {
				t = *inc.ScheduledStart
			}
			name := w.month(t.In(snap.loc))
			if n := len(v.Months); n == 0 || v.Months[n-1].Name != name {
				v.Months = append(v.Months, monthView{Name: name})
			}
			v.Months[len(v.Months)-1].Incidents = append(v.Months[len(v.Months)-1].Incidents, iv)
		}
		v.NoHistory = strings.Replace(w.NoHistory, "%d", fmt.Sprint(pg.HistoryDays), 1)
	}
	if pg.ShowUptime {
		v.OldestDay = strings.Replace(w.DaysAgo, "%d", fmt.Sprint(pg.HistoryDays-1), 1)
	}
	return v
}

var statusTemplate = template.Must(template.New("status").Parse(`<!doctype html>
<html lang="{{.W.Lang}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Name}} – {{if .Incident}}{{.Incident.Title}}{{else}}{{.Headline}}{{end}}</title>
<link rel="alternate" type="application/atom+xml" href="{{.FeedURL}}" title="{{.W.Feed}}">
<style>
:root{--brand:{{.Color}};color-scheme:light dark}
body{font:16px/1.5 system-ui,sans-serif;max-width:52rem;margin:2rem auto;padding:0 1rem;color:#1f2328;background:#fff}
@media (prefers-color-scheme:dark){body{color:#e6e6e6;background:#16181d}.card{background:#1f2229}}
a{color:var(--brand)}
header{display:flex;align-items:center;gap:.75rem;flex-wrap:wrap;margin-bottom:1.5rem}
header img{height:2rem;width:auto}
header h1{font-size:1.4rem;margin:0;flex:1}
header nav a{margin-left:1rem;font-size:.95rem}
.banner{border-radius:.75rem;padding:1rem 1.25rem;color:#fff;font-weight:600;font-size:1.15rem;margin:0 0 1.5rem}
h2{font-size:1.1rem;margin:2rem 0 .75rem}
h3{font-size:1rem;margin:1.25rem 0 .5rem;opacity:.85}
.card{border:1px solid #8884;border-radius:.75rem;padding:1rem 1.25rem;margin:0 0 1rem}
.card h3{margin:0 0 .25rem;opacity:1}
.meta{font-size:.9rem;opacity:.75}
.update{margin:.75rem 0 0}
.update b{margin-right:.4rem}
.comp{border-bottom:1px solid #8883;padding:.6rem 0}
.comp:last-child{border-bottom:0}
.row{display:flex;justify-content:space-between;gap:1rem}
.state{font-size:.9rem;font-weight:600}
.bars{display:flex;gap:1px;margin:.4rem 0 .2rem;height:1.6rem}
.bars span{flex:1;border-radius:2px;min-width:1px}
.scale{display:flex;justify-content:space-between;font-size:.8rem;opacity:.7}
footer{margin:3rem 0 1rem;font-size:.85rem;opacity:.75;display:flex;gap:1rem;flex-wrap:wrap}
</style></head><body>
<header>{{if .Logo}}<img src="{{.Logo}}" alt="">{{end}}<h1><a href="{{.Home}}" style="color:inherit;text-decoration:none">{{.Name}}</a></h1>
<nav>{{if .Site}}<a href="{{.Site}}" rel="noopener">{{.W.Site}}</a>{{end}}{{if .Support}}<a href="{{.Support}}" rel="noopener">{{.W.Support}}</a>{{end}}</nav></header>
{{if .Incident}}{{with .Incident}}
<p><a href="{{$.Home}}">← {{$.W.Back}}</a></p>
<h2>{{.Title}}</h2>
<p class="meta">{{.State}}{{if not .Maintenance}} · {{.Impact}}{{end}}{{if .Affects}} · {{$.W.Affects}}: {{.Affects}}{{end}}</p>
{{if .Start}}<p class="meta">{{$.W.Window}}: <time datetime="{{.Start.ISO}}" data-local>{{.Start.Text}}</time> – <time datetime="{{.End.ISO}}" data-local>{{.End.Text}}</time></p>{{end}}
{{range .Updates}}<div class="card"><p class="update"><b>{{.State}}</b><span class="meta"><time datetime="{{.At.ISO}}" data-local>{{.At.Text}}</time>{{if .Edited}} · {{$.W.Edited}}{{end}}</span></p>{{if .Body}}<p>{{.Body}}</p>{{end}}</div>{{end}}
{{end}}{{else if eq .View "history"}}
<p><a href="{{.Home}}">← {{.W.Back}}</a></p>
<h2>{{.W.History}}</h2>
{{range .Months}}<h3>{{.Name}}</h3>{{range .Incidents}}<div class="card"><h3><a href="{{.URL}}">{{.Title}}</a></h3><p class="meta">{{.State}} · <time datetime="{{.When.ISO}}" data-local>{{.When.Text}}</time></p></div>{{end}}{{else}}<p>{{.NoHistory}}</p>{{end}}
{{else}}
<p class="banner" style="background:{{.StateColor}}">{{.Headline}}</p>
{{if .Description}}<p>{{.Description}}</p>{{end}}
{{range .Open}}<div class="card"><h3><a href="{{.URL}}">{{.Title}}</a></h3>
<p class="meta">{{.State}}{{if not .Maintenance}} · {{.Impact}}{{end}}{{if .Affects}} · {{$.W.Affects}}: {{.Affects}}{{end}}</p>
{{if .Start}}<p class="meta">{{$.W.Window}}: <time datetime="{{.Start.ISO}}" data-local>{{.Start.Text}}</time> – <time datetime="{{.End.ISO}}" data-local>{{.End.Text}}</time></p>{{end}}
{{range .Updates}}<p class="update"><b>{{.State}}</b><span class="meta"><time datetime="{{.At.ISO}}" data-local>{{.At.Text}}</time>{{if .Edited}} · {{$.W.Edited}}{{end}}</span>{{if .Body}}<br>{{.Body}}{{end}}</p>{{end}}</div>{{end}}
<h2>{{.W.Components}}</h2>
{{range .Groups}}{{if .Name}}<h3>{{.Name}}</h3>{{end}}<div class="card">{{range .Components}}<div class="comp">
<div class="row"><span>{{.Name}}{{if .Description}}<br><span class="meta">{{.Description}}</span>{{end}}</span><span class="state" style="color:{{.Color}}">{{.State}}</span></div>
{{if .Days}}<div class="bars">{{range .Days}}<span style="background:{{.Color}}" title="{{.Title}}"></span>{{end}}</div>
<div class="scale"><span>{{$.OldestDay}}</span><span>{{.Uptime}}</span><span>{{$.W.Today}}</span></div>{{end}}
</div>{{end}}</div>{{end}}
<h2>{{.W.Recent}}</h2>
{{range .Recent}}<div class="card"><h3><a href="{{.URL}}">{{.Title}}</a></h3><p class="meta">{{.State}} · <time datetime="{{.When.ISO}}" data-local>{{.When.Text}}</time></p></div>{{else}}<p>{{.W.NoRecent}}</p>{{end}}
{{end}}
<footer><a href="{{.HistoryURL}}">{{.W.History}}</a><a href="{{.FeedURL}}">{{.W.Feed}}</a>{{if .SubscribeURL}}<a href="{{.SubscribeURL}}" target="_top">{{.W.Subscribe}}</a>{{end}}<span>{{.W.Updated}}: <time datetime="{{.Updated.ISO}}" data-local>{{.Updated.Text}}</time></span></footer>
<script>{{.Script}}</script>
</body></html>`))

func renderStatus(v statusPageView) ([]byte, error) {
	var b bytes.Buffer
	if err := statusTemplate.Execute(&b, v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
}

type atomText struct {
	Type string `xml:"type,attr,omitempty"`
	Body string `xml:",chardata"`
}

type atomEntry struct {
	ID        string   `xml:"id"`
	Title     string   `xml:"title"`
	Updated   string   `xml:"updated"`
	Published string   `xml:"published"`
	Link      atomLink `xml:"link"`
	Content   atomText `xml:"content"`
}

type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	ID      string      `xml:"id"`
	Title   string      `xml:"title"`
	Updated string      `xml:"updated"`
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

func (s *Server) statusFeed(snap *statusSnap) ([]byte, error) {
	w := snap.words
	feed := atomFeed{ID: snap.url(""), Title: snap.page.Name, Updated: snap.page.ChangedAt.UTC().Format(time.RFC3339),
		Links: []atomLink{{Href: snap.url(""), Rel: "alternate"}, {Href: snap.url("/feed.atom"), Rel: "self"}}}
	incs := slices.Clone(snap.incidents)
	slices.SortStableFunc(incs, func(a, b store.StatusIncident) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	for i, inc := range incs {
		if i == statusFeedEntries {
			break
		}
		var content strings.Builder
		for _, u := range snap.parts.updates[inc.ID] {
			content.WriteString("<p><b>" + template.HTMLEscapeString(w.IncidentStates[u.State]) + "</b> " +
				template.HTMLEscapeString(w.moment(u.CreatedAt.In(snap.loc))))
			if u.Body != "" {
				content.WriteString("<br>" + string(richStatusText(u.Body)))
			}
			content.WriteString("</p>")
		}
		url := snap.url("/incidents/" + inc.ID.String())
		feed.Entries = append(feed.Entries, atomEntry{ID: "urn:uuid:" + inc.ID.String(), Title: inc.Title + " – " + w.IncidentStates[inc.State],
			Updated: inc.UpdatedAt.UTC().Format(time.RFC3339), Published: inc.PublishedAt.UTC().Format(time.RFC3339),
			Link: atomLink{Href: url}, Content: atomText{Type: "html", Body: content.String()}})
	}
	out, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}

func statusBadge(snap *statusSnap) []byte {
	state := string(snap.out.State)
	label, value := "status", snap.words.States[state]
	lw, vw := 6*len([]rune(label))+12, 7*len([]rune(value))+12
	var b bytes.Buffer
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img" aria-label="%s: %s">`, lw+vw,
		template.HTMLEscapeString(label), template.HTMLEscapeString(value))
	fmt.Fprintf(&b, `<rect width="%d" height="20" rx="3" fill="#555"/><rect x="%d" width="%d" height="20" rx="3" fill="%s"/><rect x="%d" width="4" height="20" fill="%s"/>`,
		lw+vw, lw, vw, statusColors[state], lw, statusColors[state])
	fmt.Fprintf(&b, `<g fill="#fff" font-family="Verdana,DejaVu Sans,sans-serif" font-size="11"><text x="%d" y="14">%s</text><text x="%d" y="14">%s</text></g></svg>`,
		6, template.HTMLEscapeString(label), lw+6, template.HTMLEscapeString(value))
	return b.Bytes()
}

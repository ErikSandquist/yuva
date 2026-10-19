package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/productdevbook/yuva/api/internal/cfsaas"
	"github.com/productdevbook/yuva/api/internal/jobs"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	domainPending  = string(oas.StatusDomainPending)
	domainVerified = string(oas.StatusDomainVerified)
	domainFailed   = string(oas.StatusDomainFailed)
	domainOff      = string(oas.StatusDomainOff)

	statusDomainsAsk        = "ask"
	statusDomainsCloudflare = "cloudflare"
	statusDomainsOff        = "off"

	statusDomainRetry    = 10 * time.Minute
	statusDomainRecheck  = 24 * time.Hour
	statusDomainGrace    = 72 * time.Hour
	statusDomainTXT      = "_yuva-status."
	statusDomainCacheTTL = 30 * time.Second
	statusDomainSweepMax = 500
	statusDomainLookup   = 10 * time.Second
)

var (
	errStatusDomainTaken = problem(http.StatusConflict, "domain_taken", "another status page has this domain")
	errStatusDomainLimit = problem(http.StatusForbidden, "status_domain_limit", "this workspace has as many own domains as the server allows (YUVA_STATUS_DOMAIN_LIMIT)")

	domainLabelPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	statusDomainPaths  = []string{"", "/history", "/feed.atom", "/badge.svg", "/subscribe", "/subscribe/sent"}
)

// StatusResolver is the part of net.Resolver the domain checks use; tests give a fake one.
type StatusResolver interface {
	LookupCNAME(ctx context.Context, host string) (string, error)
	LookupTXT(ctx context.Context, name string) ([]string, error)
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// StatusDomainArgs checks one page's own domain.
type StatusDomainArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	PageID      uuid.UUID `json:"page_id"`
}

func (StatusDomainArgs) Kind() string { return "status_domain_check" }

func (StatusDomainArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateScheduled, rivertype.JobStateRetryable,
	}}}
}

// StatusDomainReleaseArgs deletes a Cloudflare custom hostname whose domain was removed, turned off
// or whose page is gone.
type StatusDomainReleaseArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	HostnameID  string    `json:"hostname_id"`
	Domain      string    `json:"domain"`
}

func (StatusDomainReleaseArgs) Kind() string { return "status_domain_release" }

func (s *Server) statusDomainsMode() string {
	if s.status.Domains == "" {
		return statusDomainsAsk
	}
	return s.status.Domains
}

func (s *Server) statusDomainsOn() bool { return s.statusDomainsMode() != statusDomainsOff }

func hostnameOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func (s *Server) statusCNAME() string {
	if s.status.CNAME != "" {
		return strings.TrimSuffix(strings.ToLower(s.status.CNAME), ".")
	}
	return hostnameOf(s.auth.PublicURL)
}

func (s *Server) ownHosts() []string {
	var out []string
	for _, h := range []string{hostnameOf(s.auth.PublicURL), hostnameOf(s.status.URL), s.statusCNAME()} {
		if h != "" && !slices.Contains(out, h) {
			out = append(out, h)
		}
	}
	return out
}

func (s *Server) statusURLFor(slug string, domain, state *string, path string) string {
	if s.statusDomainsOn() && domain != nil && state != nil && *state == domainVerified {
		scheme := "https"
		if u, err := url.Parse(s.auth.PublicURL); err == nil && u.Scheme == "http" {
			scheme = "http"
		}
		return scheme + "://" + *domain + path
	}
	return s.statusURL(slug, path)
}

// pageURL names a page's public path on its verified own domain, else at the status URL.
func (s *Server) pageURL(pg store.StatusPage, path string) string {
	return s.statusURLFor(pg.Slug, pg.CustomDomain, pg.DomainState, path)
}

func (s *Server) normalStatusDomain(raw string) (string, error) {
	d := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")
	bad := errValidation("custom_domain must be a hostname such as status.example.com")
	if len(d) < 4 || len(d) > 253 || net.ParseIP(d) != nil {
		return "", bad
	}
	labels := strings.Split(d, ".")
	if len(labels) < 2 {
		return "", bad
	}
	for _, l := range labels {
		if !domainLabelPattern.MatchString(l) {
			return "", bad
		}
	}
	if strings.Trim(labels[len(labels)-1], "0123456789") == "" {
		return "", bad
	}
	for _, h := range s.ownHosts() {
		if d == h || strings.HasSuffix(d, "."+h) || strings.HasSuffix(h, "."+d) {
			return "", errValidation("custom_domain must not be, contain or be inside one of this server's own hosts")
		}
	}
	return d, nil
}

func newDomainToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "yuva-verify-" + hex.EncodeToString(b)
}

func (s *Server) customDomainBody(pg store.StatusPage) *oas.StatusCustomDomain {
	if !s.statusDomainsOn() || pg.CustomDomain == nil || pg.DomainState == nil || pg.DomainToken == nil {
		return nil
	}
	out := &oas.StatusCustomDomain{
		Domain: *pg.CustomDomain, State: oas.StatusDomainState(*pg.DomainState), CnameTarget: s.statusCNAME(),
		TxtName: statusDomainTXT + *pg.CustomDomain, TxtValue: *pg.DomainToken, Certificates: oas.StatusCertificatesAsk,
		Problem: pg.DomainProblem, CheckedAt: pg.DomainCheckedAt, NextCheckAt: pg.DomainCheckAt, VerifiedAt: pg.DomainVerifiedAt,
		MissingSince: pg.DomainMissingSince,
	}
	if s.statusDomainsMode() == statusDomainsCloudflare {
		out.Certificates = oas.StatusCertificatesCloudflare
		cf := &oas.StatusCloudflareHostname{Id: pg.CfHostnameID, Status: pg.CfStatus, SslStatus: pg.CfSslStatus, CheckedAt: pg.CfCheckedAt}
		if len(pg.CfErrors) > 0 {
			errs := slices.Clone(pg.CfErrors)
			cf.Errors = &errs
		}
		out.Cloudflare = cf
	}
	return out
}

func releaseCloudflare(ev *eventBatch, pg store.StatusPage) {
	if pg.CfHostnameID != nil && pg.CustomDomain != nil {
		ev.job(StatusDomainReleaseArgs{WorkspaceID: pg.WorkspaceID, HostnameID: *pg.CfHostnameID, Domain: *pg.CustomDomain}, nil)
	}
}

// setStatusDomain applies custom_domain of a PATCH to the locked page.
func (s *Server) setStatusDomain(ctx context.Context, q *store.Queries, ev *eventBatch, cur store.StatusPage, raw *string) (store.StatusPage, error) {
	if !s.statusDomainsOn() {
		return cur, errValidation("own domains are off on this server (YUVA_STATUS_DOMAINS=off)")
	}
	now := s.now()
	if raw == nil || strings.TrimSpace(*raw) == "" {
		if cur.CustomDomain == nil {
			return cur, nil
		}
		releaseCloudflare(ev, cur)
		s.statusDomainCache.forget(*cur.CustomDomain)
		return q.SetStatusPageDomain(ctx, store.SetStatusPageDomainParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID, Now: now})
	}
	d, err := s.normalStatusDomain(*raw)
	if err != nil {
		return cur, err
	}
	check := func(pg store.StatusPage) (store.StatusPage, error) {
		ev.job(StatusDomainArgs{WorkspaceID: pg.WorkspaceID, PageID: pg.ID}, nil)
		return pg, nil
	}
	if cur.CustomDomain != nil && *cur.CustomDomain == d && *cur.DomainState != domainOff {
		pg, err := q.CheckStatusDomainSoon(ctx, store.CheckStatusDomainSoonParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID, Now: now})
		if err != nil {
			return cur, err
		}
		return check(pg)
	}
	if taken, err := q.StatusDomainTaken(ctx, store.StatusDomainTakenParams{Domain: &d, ID: cur.ID}); err != nil || taken {
		if err == nil {
			err = errStatusDomainTaken
		}
		return cur, err
	}
	if lim := s.status.DomainLimit; lim > 0 {
		n, err := q.CountStatusDomains(ctx, store.CountStatusDomainsParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID})
		if err != nil {
			return cur, err
		}
		if n >= int64(lim) {
			return cur, errStatusDomainLimit
		}
	}
	token := newDomainToken()
	if cur.CustomDomain != nil && *cur.CustomDomain == d {
		token = *cur.DomainToken
	}
	if cur.CustomDomain != nil {
		releaseCloudflare(ev, cur)
		s.statusDomainCache.forget(*cur.CustomDomain)
	}
	state := domainPending
	pg, err := q.SetStatusPageDomain(ctx, store.SetStatusPageDomainParams{
		WorkspaceID: cur.WorkspaceID, ID: cur.ID, Domain: &d, Token: &token, State: &state, CheckAt: &now, Now: now,
	})
	if err != nil {
		return cur, err
	}
	return check(pg)
}

type domainLookup struct {
	seen, wrong, unsure bool
	problem             string
}

func dnsMissing(err error) bool {
	var de *net.DNSError
	return errors.As(err, &de) && de.IsNotFound
}

func (s *Server) lookupStatusDomain(ctx context.Context, domain, token string) domainLookup {
	r := s.status.Resolver
	if r == nil {
		r = net.DefaultResolver
	}
	ctx, cancel := context.WithTimeout(ctx, statusDomainLookup)
	defer cancel()
	target := s.statusCNAME()
	var out domainLookup
	var problems []string
	cnameOK := false
	cname, err := r.LookupCNAME(ctx, domain)
	switch {
	case err != nil && !dnsMissing(err):
		out.unsure = true
		problems = append(problems, "DNS lookup of "+domain+" failed: "+err.Error())
	case err == nil && strings.TrimSuffix(strings.ToLower(cname), ".") == target:
		cnameOK = true
	case err == nil:
		mine, err1 := r.LookupHost(ctx, domain)
		theirs, err2 := r.LookupHost(ctx, target)
		if err1 == nil && err2 == nil && len(mine) > 0 && !slices.ContainsFunc(mine, func(a string) bool { return !slices.Contains(theirs, a) }) {
			cnameOK = true
		} else if len(mine) > 0 {
			out.wrong = true
			problems = append(problems, domain+" points at "+strings.TrimSuffix(cname, ".")+", not at "+target)
		} else {
			problems = append(problems, "no CNAME from "+domain+" to "+target)
		}
	default:
		problems = append(problems, "no CNAME from "+domain+" to "+target)
	}
	txtOK := false
	txts, err := r.LookupTXT(ctx, statusDomainTXT+domain)
	switch {
	case err != nil && !dnsMissing(err):
		out.unsure = true
		problems = append(problems, "DNS lookup of "+statusDomainTXT+domain+" failed: "+err.Error())
	case slices.ContainsFunc(txts, func(t string) bool { return strings.TrimSpace(t) == token }):
		txtOK = true
	case len(txts) > 0:
		out.wrong = true
		problems = append(problems, "the TXT record "+statusDomainTXT+domain+" does not hold the verification value")
	default:
		problems = append(problems, "no TXT record "+statusDomainTXT+domain)
	}
	out.seen = cnameOK && txtOK
	out.problem = strings.Join(problems, "; ")
	return out
}

// RunStatusDomainCheck looks the page's own domain up and moves its state: pending or failed to verified
// when both records are there, verified to off when they have been gone for 3 days. A lookup that
// fails for another reason than a missing record changes nothing and is tried again in 10 minutes.
func (s *Server) RunStatusDomainCheck(ctx context.Context, ws, id uuid.UUID) error {
	if !s.statusDomainsOn() {
		return nil
	}
	pg, err := s.st.GetStatusPage(ctx, store.GetStatusPageParams{WorkspaceID: ws, ID: id})
	if store.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if pg.CustomDomain == nil || *pg.DomainState == domainOff {
		return nil
	}
	domain, token := *pg.CustomDomain, *pg.DomainToken
	look := s.lookupStatusDomain(ctx, domain, token)
	var moved bool
	err = s.inTx(ctx, ws, func(q *store.Queries, ev *eventBatch) error {
		cur, err := q.LockStatusPage(ctx, store.LockStatusPageParams{WorkspaceID: ws, ID: id})
		if store.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if cur.CustomDomain == nil || *cur.CustomDomain != domain || *cur.DomainToken != token || *cur.DomainState == domainOff {
			return nil
		}
		now := s.now()
		state := *cur.DomainState
		arg := store.RecordStatusDomainCheckParams{WorkspaceID: ws, ID: id, State: state, Now: now, VerifiedAt: cur.DomainVerifiedAt,
			MissingSince: cur.DomainMissingSince}
		if look.problem != "" {
			arg.Problem = &look.problem
		}
		next := now.Add(statusDomainRetry)
		switch {
		case look.unsure:
		case look.seen && state == domainVerified:
			arg.MissingSince = nil
			next = now.Add(statusDomainRecheck)
		case look.seen:
			arg.State, arg.VerifiedAt, arg.MissingSince = domainVerified, &now, nil
			next = now.Add(statusDomainRecheck)
		case state == domainVerified:
			if arg.MissingSince == nil {
				arg.MissingSince = &now
			}
			next = now.Add(statusDomainRecheck)
			if !now.Before(arg.MissingSince.Add(statusDomainGrace)) {
				arg.State = domainOff
			} else if until := arg.MissingSince.Add(statusDomainGrace); until.Before(next) {
				next = until
			}
		case look.wrong:
			arg.State = domainFailed
		default:
			arg.State = domainPending
		}
		if arg.State != domainOff {
			arg.CheckAt = &next
		}
		moved = (arg.State == domainVerified) != (state == domainVerified)
		arg.Moved = moved
		if _, err := q.RecordStatusDomainCheck(ctx, arg); err != nil {
			return err
		}
		if arg.State == domainOff && cur.CfHostnameID != nil {
			releaseCloudflare(ev, cur)
			_, err = q.SetStatusPageCloudflare(ctx, store.SetStatusPageCloudflareParams{WorkspaceID: ws, ID: id, Now: now})
			return err
		}
		pg = cur
		pg.DomainState = &arg.State
		return nil
	})
	if err != nil {
		return err
	}
	if moved {
		s.statusDomainCache.forget(domain)
		s.statusHint(ctx, ws, pg.InboxID)
	}
	if s.statusDomainsMode() == statusDomainsCloudflare && pg.DomainState != nil && *pg.DomainState == domainVerified {
		return s.syncCloudflareHostname(ctx, pg)
	}
	return nil
}

// syncCloudflareHostname creates the verified domain's custom hostname, or reads its state, and keeps
// checking every 10 minutes until Cloudflare reports both the hostname and its certificate active.
func (s *Server) syncCloudflareHostname(ctx context.Context, pg store.StatusPage) error {
	cf := s.status.Cloudflare
	if cf == nil {
		return nil
	}
	domain := *pg.CustomDomain
	var h cfsaas.Hostname
	var err error
	created := false
	if pg.CfHostnameID != nil {
		h, err = cf.Get(ctx, *pg.CfHostnameID)
	}
	if pg.CfHostnameID == nil || errors.Is(err, cfsaas.ErrNotFound) {
		h, err = cf.Create(ctx, domain)
		created = err == nil
	}
	if err != nil {
		s.log.ErrorContext(ctx, "cloudflare custom hostname", slog.String("workspace_id", pg.WorkspaceID.String()),
			slog.String("domain", domain), slog.Any("error", err))
	}
	return s.inTx(ctx, pg.WorkspaceID, func(q *store.Queries, ev *eventBatch) error {
		cur, lerr := q.LockStatusPage(ctx, store.LockStatusPageParams{WorkspaceID: pg.WorkspaceID, ID: pg.ID})
		gone := store.IsNotFound(lerr) || (lerr == nil && (cur.CustomDomain == nil || *cur.CustomDomain != domain || *cur.DomainState != domainVerified))
		if gone {
			if created {
				ev.job(StatusDomainReleaseArgs{WorkspaceID: pg.WorkspaceID, HostnameID: h.ID, Domain: domain}, nil)
			}
			return nil
		}
		if lerr != nil {
			return lerr
		}
		now := s.now()
		retry := now.Add(statusDomainRetry)
		arg := store.SetStatusPageCloudflareParams{WorkspaceID: pg.WorkspaceID, ID: pg.ID, Now: now, CheckAt: &retry,
			HostnameID: cur.CfHostnameID, Status: cur.CfStatus, SslStatus: cur.CfSslStatus}
		if err != nil {
			arg.Errors = []string{err.Error()}
		} else {
			arg.HostnameID, arg.Status, arg.SslStatus, arg.Errors = &h.ID, &h.Status, &h.SSL, h.Errors
			if h.Status == "active" && h.SSL == "active" {
				arg.CheckAt = nil
			}
		}
		_, err := q.SetStatusPageCloudflare(ctx, arg)
		if err == nil && (arg.Status != cur.CfStatus || arg.SslStatus != cur.CfSslStatus) {
			s.statusHint(ctx, pg.WorkspaceID, pg.InboxID)
		}
		return err
	})
}

// ReleaseStatusDomain deletes a custom hostname at Cloudflare; an error asks River to try again.
func (s *Server) ReleaseStatusDomain(ctx context.Context, a StatusDomainReleaseArgs) error {
	if s.status.Cloudflare == nil {
		s.log.ErrorContext(ctx, "cloudflare custom hostname not deleted: YUVA_STATUS_DOMAINS is not cloudflare",
			slog.String("domain", a.Domain), slog.String("hostname_id", a.HostnameID))
		return nil
	}
	return s.status.Cloudflare.Delete(ctx, a.HostnameID)
}

// QueueStatusDomainChecks queues a check of every own domain that is due.
func (s *Server) QueueStatusDomainChecks(ctx context.Context) error {
	if !s.statusDomainsOn() {
		return nil
	}
	due, err := s.st.ListDueStatusDomains(ctx, store.ListDueStatusDomainsParams{Now: s.now(), Lim: statusDomainSweepMax})
	if err != nil || len(due) == 0 {
		return err
	}
	params := make([]river.InsertManyParams, len(due))
	for i, d := range due {
		params[i] = river.InsertManyParams{Args: StatusDomainArgs{WorkspaceID: d.WorkspaceID, PageID: d.ID}}
	}
	_, err = s.jobs.InsertMany(ctx, params)
	return err
}

type statusDomainCheckWorker struct {
	river.WorkerDefaults[StatusDomainArgs]
	s *Server
}

func (w *statusDomainCheckWorker) Work(ctx context.Context, job *river.Job[StatusDomainArgs]) error {
	return w.s.RunStatusDomainCheck(ctx, job.Args.WorkspaceID, job.Args.PageID)
}

type statusDomainReleaseWorker struct {
	river.WorkerDefaults[StatusDomainReleaseArgs]
	s *Server
}

func (w *statusDomainReleaseWorker) Work(ctx context.Context, job *river.Job[StatusDomainReleaseArgs]) error {
	return w.s.ReleaseStatusDomain(ctx, job.Args)
}

type statusDomainSweepWorker struct {
	river.WorkerDefaults[jobs.StatusDomainSweepArgs]
	s *Server
}

func (w *statusDomainSweepWorker) Work(ctx context.Context, _ *river.Job[jobs.StatusDomainSweepArgs]) error {
	return w.s.QueueStatusDomainChecks(ctx)
}

func (s *Server) addStatusDomainWorkers(workers *river.Workers) {
	river.AddWorker(workers, &statusDomainCheckWorker{s: s})
	river.AddWorker(workers, &statusDomainReleaseWorker{s: s})
	river.AddWorker(workers, &statusDomainSweepWorker{s: s})
}

func (s *Server) CheckStatusDomain(ctx context.Context, req oas.CheckStatusDomainRequestObject) (oas.CheckStatusDomainResponseObject, error) {
	if s.statusDomainsMode() != statusDomainsAsk {
		return nil, errNotFound
	}
	d := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(req.Params.Domain)), ".")
	if _, err := s.st.FindStatusPageByDomain(ctx, &d); store.IsNotFound(err) {
		return nil, errNotFound
	} else if err != nil {
		return nil, err
	}
	return oas.CheckStatusDomain200JSONResponse{Domain: d}, nil
}

type statusDomainEntry struct {
	slug string
	ok   bool
	at   time.Time
}

// statusDomainCache keeps which hosts are verified own domains for a short while, also the hosts
// that are not, so requests on unknown hosts do not each ask the database.
type statusDomainCache struct {
	mu      sync.Mutex
	entries map[string]statusDomainEntry
}

func (c *statusDomainCache) forget(domain string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, domain)
}

func (s *Server) domainPage(ctx context.Context, host string) (string, bool) {
	if host == "" || !strings.Contains(host, ".") || net.ParseIP(host) != nil {
		return "", false
	}
	c := s.statusDomainCache
	now := time.Now()
	c.mu.Lock()
	e, hit := c.entries[host]
	c.mu.Unlock()
	if hit && now.Sub(e.at) < statusDomainCacheTTL {
		return e.slug, e.ok
	}
	pg, err := s.st.FindStatusPageByDomain(ctx, &host)
	if err != nil && !store.IsNotFound(err) {
		s.log.ErrorContext(ctx, "status domain", slog.Any("error", err))
		return "", false
	}
	e = statusDomainEntry{ok: err == nil, at: now}
	if e.ok {
		e.slug = pg.Slug
	}
	c.mu.Lock()
	if len(c.entries) >= statusMaxCached {
		c.entries = map[string]statusDomainEntry{}
	}
	c.entries[host] = e
	c.mu.Unlock()
	return e.slug, e.ok
}

// statusDomainPath maps a path on an own domain to the page's path under /status/{slug}; false for
// anything else, which the domain does not serve.
func statusDomainPath(slug, path string) (string, bool) {
	if path == "/" {
		path = ""
	}
	if slices.Contains(statusDomainPaths, path) {
		return statusPathPrefix + slug + path, true
	}
	if id, ok := strings.CutPrefix(path, "/incidents/"); ok && id != "" && !strings.Contains(id, "/") {
		return statusPathPrefix + slug + path, true
	}
	return "", false
}

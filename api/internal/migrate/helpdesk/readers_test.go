package helpdesk_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/productdevbook/yuva/api/internal/migrate/helpdesk"
	"github.com/productdevbook/yuva/api/internal/migrate/helpdesk/helpdesktest"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(_ context.Context, d time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	return nil
}

func fixtureConfig(t *testing.T) (*helpdesktest.Server, helpdesk.Config, *fakeClock) {
	clock := &fakeClock{now: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)}
	srv := helpdesktest.New(t, clock.Now)
	return srv, helpdesk.Config{Dial: srv.Dial, RootCAs: srv.Pool, Now: clock.Now, Sleep: clock.Sleep}, clock
}

// onlyGETs checks that the recorded requests are GETs, besides POSTs to the token endpoint.
func onlyGETs(t *testing.T, reqs []helpdesktest.Request, tokenPath string) {
	t.Helper()
	for _, r := range reqs {
		if r.Method != http.MethodGet && !(r.Method == http.MethodPost && r.Path == tokenPath) {
			t.Errorf("%s %s%s was sent", r.Method, r.Host, r.Path)
		}
	}
}

// withinBudget checks that no minute saw more than limit requests to host, and no more than
// incremental requests to the incremental export.
func withinBudget(t *testing.T, reqs []helpdesktest.Request, host string, limit, incremental int) {
	t.Helper()
	for i, r := range reqs {
		if r.Host != host {
			continue
		}
		all, inc := 0, 0
		for _, o := range reqs[:i+1] {
			if o.Host != host || !o.At.After(r.At.Add(-time.Minute)) || o.Method != http.MethodGet {
				continue
			}
			all++
			if strings.Contains(o.Path, "/incremental/") {
				inc++
			}
		}
		if all > limit || (incremental > 0 && inc > incremental) {
			t.Errorf("the minute up to %s saw %d requests (%d incremental) to %s", r.At.Format(time.TimeOnly), all, inc, host)
		}
	}
}

func TestZendeskReaderAgainstFixtures(t *testing.T) {
	srv, cfg, _ := fixtureConfig(t)
	ctx := context.Background()
	app := helpdesk.App{ClientID: "zd-client", ClientSecret: "zd-secret"}
	verifier, _ := helpdesk.PKCE()
	tokens, err := helpdesk.ZendeskExchange(ctx, cfg, helpdesktest.ZendeskSubdomain, app, helpdesktest.ZendeskCode, "https://yuva.example.com/migrations/sign-in/callback", verifier)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tokens.Scopes, []string{"read"}) || tokens.Expires.IsZero() {
		t.Fatalf("tokens: %+v", tokens)
	}
	if _, err := helpdesk.ZendeskExchange(ctx, cfg, helpdesktest.ZendeskSubdomain, app, "wrong-code", "x", verifier); err == nil {
		t.Error("a wrong code was taken")
	}
	budget := &helpdesk.Budget{}
	z := helpdesk.NewZendesk(cfg, helpdesktest.ZendeskSubdomain, func() string { return tokens.Access }, budget)
	me, err := z.Me(ctx)
	if err != nil || !me.Admin || me.Email != "deniz@acme.example.com" {
		t.Fatalf("me: %+v %v", me, err)
	}
	folders, err := z.Folders(ctx)
	if err != nil || len(folders) != 2 || folders[1].Address != "billing@acme.example.com" {
		t.Fatalf("folders: %+v %v", folders, err)
	}
	var tickets []helpdesk.Ticket
	c := helpdesk.Cursor{}
	for {
		page, err := z.Tickets(ctx, c)
		var wait *helpdesk.WaitError
		if errors.As(err, &wait) {
			_ = cfg.Sleep(ctx, wait.Until.Sub(cfg.Now()))
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		tickets = append(tickets, page.Tickets...)
		c = page.Next
		if page.End {
			break
		}
	}
	if c.After != "zd-c2" || len(tickets) != 4 {
		t.Fatalf("cursor %+v, %d tickets", c, len(tickets))
	}
	first := tickets[0]
	if first.Requester.Email != "ayse@example.org" || first.Requester.ExternalID != "crm-101" || first.Assignee == nil ||
		first.Assignee.Email != "ece@acme.example.com" || first.Status != helpdesk.StatusOpen || !slices.Equal(first.Tags, []string{"login", "vip"}) {
		t.Errorf("ticket 1001: %+v", first)
	}
	if tickets[1].Status != helpdesk.StatusClosed || tickets[2].Status != helpdesk.StatusPending || tickets[3].Status != helpdesk.StatusDeleted {
		t.Errorf("statuses: %s %s %s", tickets[1].Status, tickets[2].Status, tickets[3].Status)
	}
	msgs, err := z.Messages(ctx, first)
	if err != nil || len(msgs) != 4 {
		t.Fatalf("messages: %d %v", len(msgs), err)
	}
	if msgs[0].Direction != "in" || msgs[0].Note || !msgs[1].Note || msgs[2].Direction != "out" || msgs[3].Author.Email != "carol@example.net" {
		t.Errorf("comments: %+v", msgs)
	}
	paged, err := z.Messages(ctx, tickets[2])
	if err != nil || len(paged) != 2 {
		t.Errorf("paged comments: %d %v", len(paged), err)
	}
	for _, a := range msgs[0].Attachments {
		data, err := z.Attachment(ctx, first, a)
		if err != nil || len(data) == 0 {
			t.Errorf("attachment %s: %v", a.Name, err)
		}
	}
	second, err := z.Messages(ctx, tickets[1])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := z.Attachment(ctx, tickets[1], second[0].Attachments[0]); err != nil {
		t.Error(err)
	}
	reqs := srv.Requests()
	onlyGETs(t, reqs, "/oauth/tokens")
	withinBudget(t, reqs, "acme.zendesk.com", helpdesktest.ZendeskLimit/2, 5)
	for _, r := range reqs {
		hasToken := r.Auth == "Bearer "+tokens.Access
		switch {
		case r.Host == helpdesktest.OutsideHost && r.Auth != "":
			t.Errorf("the attachment on %s was fetched with %q", r.Host, r.Auth)
		case r.Host != helpdesktest.OutsideHost && r.Method == http.MethodGet && !hasToken:
			t.Errorf("%s%s was fetched without the token", r.Host, r.Path)
		}
	}
	if !slices.ContainsFunc(reqs, func(r helpdesktest.Request) bool { return strings.HasSuffix(r.Host, ".zdusercontent.com") }) ||
		!slices.ContainsFunc(reqs, func(r helpdesktest.Request) bool { return r.Host == helpdesktest.OutsideHost }) {
		t.Error("the attachments were not fetched from their hosts")
	}
	if budget.Limit != helpdesktest.ZendeskLimit {
		t.Errorf("budget limit %d, want the X-Rate-Limit %d", budget.Limit, helpdesktest.ZendeskLimit)
	}
}

func TestHelpScoutReaderAgainstFixtures(t *testing.T) {
	srv, cfg, _ := fixtureConfig(t)
	ctx := context.Background()
	app := helpdesk.App{ClientID: "hs-client", ClientSecret: "hs-secret"}
	tokens, err := helpdesk.HelpScoutExchange(ctx, cfg, app, helpdesktest.HelpScoutCode)
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := helpdesk.HelpScoutRefresh(ctx, cfg, app, tokens.Refresh)
	if err != nil || refreshed.Access == tokens.Access {
		t.Fatalf("refresh: %+v %v", refreshed, err)
	}
	h := helpdesk.NewHelpScout(cfg, func() string { return refreshed.Access }, &helpdesk.Budget{})
	folders, err := h.Folders(ctx)
	if err != nil || len(folders) != 2 || folders[0].Address != "support@hs.example.com" {
		t.Fatalf("mailboxes: %+v %v", folders, err)
	}
	agents, err := h.Agents(ctx)
	if err != nil || len(agents) != 2 {
		t.Fatalf("users: %+v %v", agents, err)
	}
	var tickets []helpdesk.Ticket
	c := helpdesk.Cursor{}
	for {
		page, err := h.Tickets(ctx, c)
		if err != nil {
			t.Fatal(err)
		}
		tickets = append(tickets, page.Tickets...)
		c = page.Next
		if page.End {
			break
		}
	}
	if len(tickets) != 3 || tickets[2].Status != helpdesk.StatusSpam || tickets[1].Status != helpdesk.StatusClosed || tickets[0].Messages != 4 {
		t.Fatalf("conversations: %+v", tickets)
	}
	msgs, err := h.Messages(ctx, tickets[0])
	if err != nil || len(msgs) != 3 {
		t.Fatalf("threads: %+v %v", msgs, err)
	}
	if msgs[0].Raw == nil || !bytes.Contains(msgs[0].Raw, []byte("order-4411@example.org")) || !msgs[1].Note || msgs[2].Direction != "out" || msgs[2].Raw != nil {
		t.Errorf("threads 7001: %+v", msgs)
	}
	chat, err := h.Messages(ctx, tickets[1])
	if err != nil || len(chat) != 2 || chat[0].Direction != "in" || chat[1].Direction != "out" || len(chat[1].Attachments) != 1 {
		t.Fatalf("threads 7002: %+v %v", chat, err)
	}
	data, err := h.Attachment(ctx, tickets[1], chat[1].Attachments[0])
	if err != nil || len(data) == 0 {
		t.Errorf("attachment: %v", err)
	}
	customer, err := h.Customer(ctx, tickets[0].Requester)
	if err != nil || customer.Phone == "" {
		t.Errorf("customer: %+v %v", customer, err)
	}
	reqs := srv.Requests()
	onlyGETs(t, reqs, "/v2/oauth2/token")
	withinBudget(t, reqs, "api.helpscout.net", helpdesktest.HelpScoutLimit/2, 0)
}

// A minute never sees more than half the reported limit, nor more than five incremental requests.
func TestBudgetKeepsHalfTheLimit(t *testing.T) {
	b := helpdesk.ZendeskBudget(&helpdesk.Budget{})
	h := http.Header{}
	h.Set("X-Rate-Limit", "40")
	b.Saw(h)
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	var sent, incremental []time.Time
	for len(sent) < 200 {
		class := helpdesk.ClassAPI
		if len(sent)%3 == 0 {
			class = helpdesk.ClassIncremental
		}
		err := b.Take(class, now)
		var wait *helpdesk.WaitError
		if errors.As(err, &wait) {
			now = wait.Until
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		sent = append(sent, now)
		if class == helpdesk.ClassIncremental {
			incremental = append(incremental, now)
		}
	}
	for _, list := range []struct {
		at  []time.Time
		max int
	}{{sent, 20}, {incremental, 5}} {
		for i, at := range list.at {
			n := 0
			for _, o := range list.at[:i+1] {
				if o.After(at.Add(-time.Minute)) {
					n++
				}
			}
			if n > list.max {
				t.Fatalf("%d requests in the minute up to %s, want at most %d", n, at, list.max)
			}
		}
	}
}

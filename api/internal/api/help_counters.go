package api

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	helpFlushEvery        = time.Minute
	helpQueryStoredLen    = 100
	helpScrubbed          = "(scrubbed)"
	helpOtherQuery        = "(other)"
	helpQueriesPerDay     = 2000
	helpArticleMonthsKept = 25
	helpSearchDaysKept    = 90
)

var helpURLShape = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://|\bwww\.)`)

type helpArticleKey struct {
	ws, center, article uuid.UUID
	locale              string
	day                 time.Time
}

type helpArticleCount struct {
	views, conversations, sent int32
	up, down                   int32
}

type helpSearchKey struct {
	ws, center uuid.UUID
	locale     string
	day        time.Time
	query      string
}

type helpSearchCount struct {
	searches, noResults, clicks int32
}

type helpDayKey struct {
	ws, center uuid.UUID
	day        time.Time
}

type helpUsageKey struct {
	ws    uuid.UUID
	month time.Time
}

// Buffered in process so a view or a search never writes in its request.
type helpCounters struct {
	mu       sync.Mutex
	articles map[helpArticleKey]helpArticleCount
	searches map[helpSearchKey]helpSearchCount
	distinct map[helpDayKey]int
	usage    map[helpUsageKey]int64
}

func newHelpCounters() *helpCounters {
	return &helpCounters{
		articles: map[helpArticleKey]helpArticleCount{},
		searches: map[helpSearchKey]helpSearchCount{},
		distinct: map[helpDayKey]int{},
		usage:    map[helpUsageKey]int64{},
	}
}

func (h *helpCounters) addArticle(k helpArticleKey, n helpArticleCount) {
	h.mu.Lock()
	defer h.mu.Unlock()
	cur := h.articles[k]
	cur.views += n.views
	cur.conversations += n.conversations
	cur.sent += n.sent
	cur.up += n.up
	cur.down += n.down
	h.articles[k] = cur
}

func (h *helpCounters) addSearch(k helpSearchKey, n helpSearchCount) {
	h.mu.Lock()
	defer h.mu.Unlock()
	day := helpDayKey{ws: k.ws, center: k.center, day: k.day}
	if _, ok := h.searches[k]; !ok {
		if h.distinct[day] >= helpQueriesPerDay {
			k.query = helpOtherQuery
		}
		if _, ok := h.searches[k]; !ok {
			h.distinct[day]++
		}
	}
	cur := h.searches[k]
	cur.searches += n.searches
	cur.noResults += n.noResults
	cur.clicks += n.clicks
	h.searches[k] = cur
	if n.searches > 0 {
		h.usage[helpUsageKey{ws: k.ws, month: monthOf(k.day)}] += int64(n.searches)
	}
}

func (h *helpCounters) drain() (map[helpArticleKey]helpArticleCount, map[helpSearchKey]helpSearchCount, map[helpUsageKey]int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	a, s, u := h.articles, h.searches, h.usage
	h.articles, h.searches, h.usage = map[helpArticleKey]helpArticleCount{}, map[helpSearchKey]helpSearchCount{}, map[helpUsageKey]int64{}
	h.distinct = map[helpDayKey]int{}
	return a, s, u
}

func (h *helpCounters) restore(a map[helpArticleKey]helpArticleCount, s map[helpSearchKey]helpSearchCount, u map[helpUsageKey]int64) {
	for k, n := range a {
		h.addArticle(k, n)
	}
	h.mu.Lock()
	for k, n := range s {
		if _, ok := h.searches[k]; !ok {
			h.distinct[helpDayKey{ws: k.ws, center: k.center, day: k.day}]++
		}
		cur := h.searches[k]
		cur.searches += n.searches
		cur.noResults += n.noResults
		cur.clicks += n.clicks
		h.searches[k] = cur
	}
	for k, n := range u {
		h.usage[k] += n
	}
	h.mu.Unlock()
}

func helpStoredQuery(q string) string {
	words := helpQueryWords(q)
	if len(words) == 0 {
		return ""
	}
	if strings.Contains(q, "@") || helpURLShape.MatchString(q) {
		return helpScrubbed
	}
	digits := true
	for _, w := range words {
		for _, r := range w {
			if r < '0' || r > '9' {
				digits = false
			}
		}
	}
	if digits {
		return helpScrubbed
	}
	out := strings.Join(words, " ")
	if r := []rune(out); len(r) > helpQueryStoredLen {
		out = strings.TrimSpace(string(r[:helpQueryStoredLen]))
	}
	return out
}

func (s *Server) countHelpArticle(c store.HelpCenter, articleID uuid.UUID, locale string, n helpArticleCount) {
	s.helpCounts.addArticle(helpArticleKey{ws: c.WorkspaceID, center: c.ID, article: articleID, locale: locale, day: utcDay(s.now())}, n)
}

// A changed answer moves the count from the earlier answer to the new one.
func (s *Server) countHelpReaction(c store.HelpCenter, article uuid.UUID, locale string, rating, previous oas.PageRating) {
	if rating == previous {
		return
	}
	var n helpArticleCount
	for r, d := range map[oas.PageRating]int32{rating: 1, previous: -1} {
		if r == "" {
			continue
		}
		if r == oas.PageRatingUp {
			n.up += d
		} else {
			n.down += d
		}
	}
	s.countHelpArticle(c, article, locale, n)
}

func (s *Server) countHelpSearch(c store.HelpCenter, locale, query string, found bool) {
	q := helpStoredQuery(query)
	if q == "" {
		return
	}
	n := helpSearchCount{searches: 1}
	if !found {
		n.noResults = 1
	}
	s.helpCounts.addSearch(helpSearchKey{ws: c.WorkspaceID, center: c.ID, locale: locale, day: utcDay(s.now()), query: q}, n)
}

func (s *Server) countHelpSearchClick(c store.HelpCenter, locale, query string) {
	q := helpStoredQuery(query)
	if q == "" {
		return
	}
	s.helpCounts.addSearch(helpSearchKey{ws: c.WorkspaceID, center: c.ID, locale: locale, day: utcDay(s.now()), query: q}, helpSearchCount{clicks: 1})
}

// Rows of articles or centers deleted since are dropped; on failure the counts go back.
func (s *Server) FlushHelpCounters(ctx context.Context) error {
	a, sr, u := s.helpCounts.drain()
	if len(a) == 0 && len(sr) == 0 && len(u) == 0 {
		return nil
	}
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		for k, n := range a {
			if err := q.AddHelpArticleDaily(ctx, store.AddHelpArticleDailyParams{
				WorkspaceID: k.ws, ArticleID: k.article, Locale: k.locale, Day: k.day, Views: n.views, HelpfulUp: n.up, HelpfulDown: n.down, Conversations: n.conversations, Sent: n.sent,
			}); err != nil {
				return err
			}
		}
		for k, n := range sr {
			if err := q.AddHelpSearchDaily(ctx, store.AddHelpSearchDailyParams{
				WorkspaceID: k.ws, CenterID: k.center, Locale: k.locale, Day: k.day, Query: k.query,
				MaxQueries: helpQueriesPerDay, Overflow: helpOtherQuery, Searches: n.searches, NoResults: n.noResults, Clicks: n.clicks,
			}); err != nil {
				return err
			}
		}
		for k, n := range u {
			if err := q.AddHelpSearchUsage(ctx, store.AddHelpSearchUsageParams{WorkspaceID: k.ws, Month: k.month, Searches: n}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		s.helpCounts.restore(a, sr, u)
	}
	return err
}

func (s *Server) RunHelpCounters(ctx context.Context) {
	if s.help.Off {
		return
	}
	t := time.NewTicker(helpFlushEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			drain, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			if err := s.FlushHelpCounters(drain); err != nil {
				s.log.ErrorContext(drain, "help counters", slog.Any("error", err))
			}
			return
		case <-t.C:
			if err := s.FlushHelpCounters(ctx); err != nil {
				s.log.ErrorContext(ctx, "help counters", slog.Any("error", err))
			}
		}
	}
}

func (s *Server) CleanHelp(ctx context.Context) error {
	ids, err := s.st.ListWorkspaceIDs(ctx)
	if err != nil {
		return err
	}
	today := utcDay(s.now())
	for _, ws := range ids {
		if _, err := s.st.DeleteOldHelpArticleDaily(ctx, store.DeleteOldHelpArticleDailyParams{WorkspaceID: ws, Before: today.AddDate(0, -helpArticleMonthsKept, 0)}); err != nil {
			return err
		}
		if _, err := s.st.DeleteOldHelpSearchDaily(ctx, store.DeleteOldHelpSearchDailyParams{WorkspaceID: ws, Before: today.AddDate(0, 0, -helpSearchDaysKept)}); err != nil {
			return err
		}
		if _, err := s.st.DeleteOldHelpFormerSlugs(ctx, store.DeleteOldHelpFormerSlugsParams{WorkspaceID: ws, Before: s.now().AddDate(0, 0, -helpFormerSlugDays)}); err != nil {
			return err
		}
	}
	return nil
}

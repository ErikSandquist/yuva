package api

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	maxHelpQueryWords    = 8
	maxHelpQueryWordLen  = 50
	maxHelpQueryLen      = 200
	helpFallbackMinCount = 3
	helpViewsWindow      = 30
)

// İ and ı are i whatever the language, as in the search vectors.
func foldHelpText(s string) string {
	return strings.ToLower(strings.NewReplacer("İ", "i", "ı", "i").Replace(s))
}

func helpWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r)
}

// Words hold only letters, digits and marks, so each is safe as a tsquery lexeme.
func helpQueryWords(q string) []string {
	words := strings.FieldsFunc(foldHelpText(q), func(r rune) bool { return !helpWordRune(r) })
	if len(words) > maxHelpQueryWords {
		words = words[:maxHelpQueryWords]
	}
	for i, w := range words {
		if r := []rune(w); len(r) > maxHelpQueryWordLen {
			words[i] = string(r[:maxHelpQueryWordLen])
		}
	}
	return words
}

type helpHit struct {
	result oas.HelpSearchResult
	rank   float64
	views  int64
}

func (s *Server) searchHelp(ctx context.Context, c store.HelpCenter, locale string, words []string, drafts bool, lim int32) (oas.HelpSearchResults, error) {
	out := oas.HelpSearchResults{Items: []oas.HelpSearchResult{}}
	locales := []string{locale}
	err := s.st.InTxRaw(ctx, func(tx pgx.Tx, q *store.Queries) error {
		if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '500ms'"); err != nil {
			return err
		}
		if !drafts && locale != c.DefaultLocale {
			n, err := q.CountHelpPublishedInLocale(ctx, store.CountHelpPublishedInLocaleParams{WorkspaceID: c.WorkspaceID, CenterID: c.ID, Locale: locale})
			if err != nil {
				return err
			}
			if n < helpFallbackMinCount {
				locales = append(locales, c.DefaultLocale)
			}
		}
		for _, anyWord := range []bool{false, true} {
			if anyWord && len(words) < 2 {
				break
			}
			var hits []helpHit
			for _, l := range locales {
				h, err := s.searchHelpLocale(ctx, q, c, l, words, drafts, anyWord, lim)
				if err != nil {
					return err
				}
				hits = append(hits, h...)
			}
			if len(hits) > 0 {
				out.Items, out.Partial = mergeHelpHits(hits, lim), anyWord
				return nil
			}
		}
		return nil
	})
	if store.IsQueryCanceled(err) {
		return out, errHelpSearchTimeout
	}
	if err != nil {
		return out, err
	}
	for i := range out.Items {
		out.Items[i].Highlights = helpHighlights(words, out.Items[i].Title, out.Items[i].Excerpt)
	}
	return out, nil
}

func (s *Server) searchHelpLocale(ctx context.Context, q *store.Queries, c store.HelpCenter, locale string, words []string, drafts, anyWord bool, lim int32) ([]helpHit, error) {
	cfg := helpSearchConfig(locale)
	heads, last := words[:len(words)-1], words[len(words)-1]
	prefix := strings.Join(words, " ")
	var hits []helpHit
	if drafts {
		rows, err := q.SearchHelpDrafts(ctx, store.SearchHelpDraftsParams{
			WorkspaceID: c.WorkspaceID, CenterID: c.ID, Locale: locale, Lim: lim, AnyWord: anyWord, Config: cfg, Heads: heads, Last: last, Prefix: prefix,
		})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			t := store.HelpArticleTranslation{Locale: r.Locale, PublishedAt: r.PublishedAt, PublishedSlug: r.PublishedSlug, HasUnpublishedChanges: r.HasUnpublishedChanges}
			hits = append(hits, helpHit{rank: r.Rank, result: oas.HelpSearchResult{
				ArticleId: r.ArticleID, CollectionId: r.CollectionID, Number: r.Number, Locale: r.Locale, Title: r.Title,
				Excerpt: helpPlainExcerpt(r.BodyStart), State: helpState(t), Url: s.helpPublicURL(c, r.Number, t),
			}})
		}
		return hits, nil
	}
	rows, err := q.SearchHelpPublished(ctx, store.SearchHelpPublishedParams{
		WorkspaceID: c.WorkspaceID, CenterID: c.ID, Locale: locale, Lim: lim, AnyWord: anyWord, Config: cfg, Heads: heads, Last: last,
		Prefix: prefix, Since: utcDay(s.now()).AddDate(0, 0, -helpViewsWindow+1),
	})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		t := store.HelpArticleTranslation{Locale: r.Locale, PublishedAt: new(time.Time{}), PublishedSlug: &r.Slug, HasUnpublishedChanges: r.HasUnpublishedChanges}
		hits = append(hits, helpHit{rank: r.Rank, views: r.Views, result: oas.HelpSearchResult{
			ArticleId: r.ArticleID, CollectionId: r.CollectionID, Number: r.Number, Locale: r.Locale, Title: r.Title,
			Excerpt: r.Excerpt, State: helpState(t), Url: s.helpPublicURL(c, r.Number, t),
		}})
	}
	return hits, nil
}

func mergeHelpHits(hits []helpHit, lim int32) []oas.HelpSearchResult {
	first := map[string]int{}
	for i, h := range hits {
		if _, ok := first[h.result.ArticleId.String()]; !ok {
			first[h.result.ArticleId.String()] = i
		}
	}
	kept := hits[:0:0]
	for i, h := range hits {
		if first[h.result.ArticleId.String()] == i {
			kept = append(kept, h)
		}
	}
	slices.SortStableFunc(kept, func(a, b helpHit) int {
		switch {
		case a.rank != b.rank:
			if a.rank > b.rank {
				return -1
			}
			return 1
		case a.views != b.views:
			if a.views > b.views {
				return -1
			}
			return 1
		}
		return int(a.result.Number - b.result.Number)
	})
	if len(kept) > int(lim) {
		kept = kept[:lim]
	}
	out := make([]oas.HelpSearchResult, len(kept))
	for i, h := range kept {
		out[i] = h.result
	}
	return out
}

func helpWordMatches(query, word string) bool {
	if query == word {
		return true
	}
	short, long := query, word
	if utf8.RuneCountInString(short) > utf8.RuneCountInString(long) {
		short, long = long, short
	}
	return utf8.RuneCountInString(short) >= 3 && strings.HasPrefix(long, short)
}

// Offsets count UTF-16 code units, as JavaScript strings do.
func helpHighlights(words []string, title, excerpt string) []oas.HelpHighlight {
	out := []oas.HelpHighlight{}
	for _, f := range []struct {
		field oas.HelpHighlightField
		text  string
	}{{oas.HelpHighlightTitle, title}, {oas.HelpHighlightExcerpt, excerpt}} {
		var start, pos int32
		var word []rune
		flush := func() {
			if len(word) > 0 && slices.ContainsFunc(words, func(q string) bool { return helpWordMatches(q, foldHelpText(string(word))) }) {
				out = append(out, oas.HelpHighlight{Field: f.field, Start: start, End: pos})
			}
			word = word[:0]
		}
		for _, r := range f.text {
			if helpWordRune(r) {
				if len(word) == 0 {
					start = pos
				}
				word = append(word, r)
			} else {
				flush()
			}
			pos += int32(utf16.RuneLen(r))
		}
		flush()
	}
	return out
}

func (s *Server) SearchHelpArticles(ctx context.Context, req oas.SearchHelpArticlesRequestObject) (oas.SearchHelpArticlesResponseObject, error) {
	if err := s.helpOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	prm := req.Params
	lim := int32(10)
	if prm.Limit != nil {
		if *prm.Limit < 1 || *prm.Limit > 20 {
			return nil, errValidation("limit must be 1 to 20")
		}
		lim = *prm.Limit
	}
	if utf8.RuneCountInString(prm.Q) > maxHelpQueryLen {
		return nil, errValidation("q must be at most 200 characters")
	}
	words := helpQueryWords(prm.Q)
	if len(words) == 0 {
		return nil, errHelpQueryEmpty
	}
	c, err := visibleHelpCenter(ctx, s.st.Queries, p, req.CenterId, false)
	if err != nil {
		return nil, err
	}
	locale := c.DefaultLocale
	if prm.Locale != nil {
		if err := offersLocale(c, *prm.Locale); err != nil {
			return nil, err
		}
		locale = *prm.Locale
	}
	out, err := s.searchHelp(ctx, c, locale, words, boolOr(prm.Drafts, false), lim)
	if err != nil {
		return nil, err
	}
	return oas.SearchHelpArticles200JSONResponse(out), nil
}

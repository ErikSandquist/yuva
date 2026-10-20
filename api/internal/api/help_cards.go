package api

import (
	"context"
	"encoding/json"
	"html"
	"slices"
	"strings"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const maxHelpCards = 3

func pickHelpLocale(published []string, prefs ...string) string {
	for _, pref := range prefs {
		want := strings.ToLower(strings.ReplaceAll(pref, "_", "-"))
		if want == "" {
			continue
		}
		if i := slices.IndexFunc(published, func(l string) bool { return strings.ToLower(l) == want }); i >= 0 {
			return published[i]
		}
		base, _, _ := strings.Cut(want, "-")
		if i := slices.IndexFunc(published, func(l string) bool { b, _, _ := strings.Cut(strings.ToLower(l), "-"); return b == base }); i >= 0 {
			return published[i]
		}
	}
	sorted := slices.Clone(published)
	slices.Sort(sorted)
	return sorted[0]
}

func (s *Server) helpCards(ctx context.Context, q *store.Queries, conv store.Conversation, ids []uuid.UUID) (store.HelpCenter, []oas.MessageCard, error) {
	c, err := q.GetInboxHelpCenter(ctx, store.GetInboxHelpCenterParams{WorkspaceID: conv.WorkspaceID, InboxID: conv.InboxID})
	if store.IsNotFound(err) || (err == nil && (!c.Published || s.help.Off)) {
		return c, nil, errValidation("article_ids: this inbox has no help center")
	}
	if err != nil {
		return c, nil, err
	}
	rows, err := q.ListHelpCardArticles(ctx, store.ListHelpCardArticlesParams{WorkspaceID: c.WorkspaceID, CenterID: c.ID, ArticleIds: ids})
	if err != nil {
		return c, nil, err
	}
	contact, err := q.GetContact(ctx, store.GetContactParams{WorkspaceID: conv.WorkspaceID, ID: conv.ContactID})
	if err != nil {
		return c, nil, err
	}
	inbox, err := q.GetInbox(ctx, store.GetInboxParams{WorkspaceID: conv.WorkspaceID, ID: conv.InboxID})
	if err != nil {
		return c, nil, err
	}
	cards := make([]oas.MessageCard, 0, len(ids))
	for _, id := range ids {
		var mine []store.ListHelpCardArticlesRow
		var locales []string
		for _, r := range rows {
			if r.ID == id && slices.Contains(c.Locales, r.Locale) {
				mine = append(mine, r)
				locales = append(locales, r.Locale)
			}
		}
		if len(mine) == 0 {
			return c, nil, errValidation("article_ids: " + id.String() + " is not a published article of this inbox's help center")
		}
		locale := pickHelpLocale(locales, deref(contact.Locale), inbox.DefaultLocale, c.DefaultLocale)
		for _, r := range mine {
			if r.Locale == locale {
				cards = append(cards, oas.MessageCard{
					Kind: oas.MessageCardArticle, ArticleId: id, Locale: locale, Title: r.Title, Excerpt: r.Excerpt,
					Url: s.helpArticleURL(c, r.Number, locale, r.Slug),
				})
			}
		}
	}
	return c, cards, nil
}

func helpCardLines(cards []oas.MessageCard) string {
	lines := make([]string, len(cards))
	for i, card := range cards {
		lines[i] = card.Title + " - " + card.Url
	}
	return strings.Join(lines, "\n")
}

// Clients that show no cards still get each article from the last lines of the body.
func withHelpCards(body string, cards []oas.MessageCard) string {
	if len(cards) == 0 {
		return body
	}
	if strings.TrimSpace(body) == "" {
		return helpCardLines(cards)
	}
	return strings.TrimRight(body, " \t\r\n") + "\n\n" + helpCardLines(cards)
}

func helpCardsOf(raw []byte) []oas.MessageCard {
	if len(raw) == 0 {
		return nil
	}
	var cards []oas.MessageCard
	if json.Unmarshal(raw, &cards) != nil {
		return nil
	}
	return cards
}

func withoutHelpCards(body string, cards []oas.MessageCard) string {
	if len(cards) == 0 {
		return body
	}
	lines := helpCardLines(cards)
	if body == lines {
		return ""
	}
	return strings.TrimRight(strings.TrimSuffix(body, "\n\n"+lines), " \t\r\n")
}

func helpCardsHTML(cards []oas.MessageCard) string {
	var b strings.Builder
	for _, card := range cards {
		b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:16px 0;border:1px solid #d4d4d8;border-radius:8px;border-collapse:separate"><tr><td style="padding:14px 16px;font:14px/1.5 -apple-system,Segoe UI,Helvetica,Arial,sans-serif;color:#52525b">`)
		b.WriteString(`<a href="` + html.EscapeString(card.Url) + `" style="font-size:16px;font-weight:600;color:#0b0b0f;text-decoration:none">` + html.EscapeString(card.Title) + `</a>`)
		if card.Excerpt != "" {
			b.WriteString(`<br>` + html.EscapeString(card.Excerpt))
		}
		b.WriteString(`</td></tr></table>`)
	}
	return b.String()
}

func (s *Server) countHelpSent(ctx context.Context, ws, inbox uuid.UUID, cards []oas.MessageCard) {
	if len(cards) == 0 {
		return
	}
	c, err := s.st.GetInboxHelpCenter(ctx, store.GetInboxHelpCenterParams{WorkspaceID: ws, InboxID: inbox})
	if err != nil {
		return
	}
	for _, card := range cards {
		s.countHelpArticle(c, card.ArticleId, card.Locale, helpArticleCount{sent: 1})
	}
}

package api

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/mail/design"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	utmSource         = "newsletter"
	utmMedium         = "email"
	maxCampaignLength = 100
)

var (
	utmCampaignPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]*$`)
	campaignLetters    = strings.NewReplacer("ç", "c", "ğ", "g", "ı", "i", "ö", "o", "ş", "s", "ü", "u", "ä", "a", "ß", "ss", "é", "e", "è", "e", "á", "a", "à", "a", "ñ", "n")
	campaignSeparators = regexp.MustCompile(`[^a-z0-9_.]+`)
)

// campaignOf is a broadcast's default utm_campaign: its name in lowercase ASCII with dashes.
func campaignOf(name string, id uuid.UUID) string {
	c := campaignSeparators.ReplaceAllString(campaignLetters.Replace(strings.ToLower(name)), "-")
	c = strings.Trim(c, "-")
	if len(c) > maxCampaignLength {
		c = strings.TrimRight(c[:maxCampaignLength], "-")
	}
	if c == "" || !utmCampaignPattern.MatchString(c) {
		c = "broadcast-" + id.String()[:8]
	}
	return c
}

func checkCampaign(v string) (string, error) {
	v = strings.TrimSpace(v)
	if len(v) > maxCampaignLength || !utmCampaignPattern.MatchString(v) {
		return "", errValidation("utm_campaign is 1 to 100 lowercase letters, digits, -, _ and ., starting with a letter or digit")
	}
	return v, nil
}

// withUTM adds the campaign to a web link that has no utm_ parameter of its own.
func withUTM(raw, campaign string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return raw
	}
	for k := range u.Query() {
		if strings.HasPrefix(strings.ToLower(k), "utm_") {
			return raw
		}
	}
	add := "utm_source=" + utmSource + "&utm_medium=" + utmMedium + "&utm_campaign=" + url.QueryEscape(campaign)
	if u.RawQuery != "" {
		u.RawQuery += "&" + add
	} else {
		u.RawQuery = add
	}
	return u.String()
}

// addUTM puts the campaign on the rendered mail's web links, leaving the server's own links
// (unsubscribe, preferences, clicks) alone; the text part gets the same links.
func (r *bcRenderer) addUTM(out design.Output) design.Output {
	if !r.b.Utm || r.b.UtmCampaign == "" {
		return out
	}
	own := []string{r.s.linkBase(), r.s.auth.PublicURL}
	changed := map[string]string{}
	out.HTML = hrefAttr.ReplaceAllStringFunc(out.HTML, func(m string) string {
		raw := html.UnescapeString(m[len(`href="`) : len(m)-1])
		for _, o := range own {
			if o != "" && strings.HasPrefix(raw, o) {
				return m
			}
		}
		next := withUTM(raw, r.b.UtmCampaign)
		if next == raw {
			return m
		}
		changed[raw] = next
		return `href="` + html.EscapeString(next) + `"`
	})
	for from, to := range changed {
		out.Text = replaceURL(out.Text, from, to)
	}
	out.Size = len(out.HTML)
	return out
}

// replaceURL replaces whole occurrences of a URL in text, not the start of a longer one.
func replaceURL(text, from, to string) string {
	var b strings.Builder
	for {
		i := strings.Index(text, from)
		if i < 0 {
			b.WriteString(text)
			return b.String()
		}
		end := i + len(from)
		b.WriteString(text[:i])
		if end < len(text) && !strings.ContainsRune(" \t\r\n)>]\"'", rune(text[end])) {
			b.WriteString(from)
		} else {
			b.WriteString(to)
		}
		text = text[end:]
	}
}

// campaignVisits counts, per Analytics property the caller sees, the visits of the broadcast's
// campaign from the day sending started.
func (s *Server) campaignVisits(ctx context.Context, p principal, b store.Broadcast) ([]oas.BroadcastCampaignVisits, error) {
	if s.analytics == nil || !b.Utm || b.UtmCampaign == "" || b.StartedAt == nil || requireScope(p, oas.AnalyticsRead) != nil {
		return nil, nil
	}
	props, err := s.st.ListAnalyticsProperties(ctx, store.ListAnalyticsPropertiesParams{WorkspaceID: p.workspaceID, AllInboxes: p.seesAllInboxes(),
		ViewerID: p.viewerID()})
	if err != nil {
		return nil, err
	}
	out := []oas.BroadcastCampaignVisits{}
	for _, prop := range props {
		loc := propertyLocation(prop.Timezone)
		from, to := analytics.LocalDay(*b.StartedAt, loc), analytics.LocalDay(s.now(), loc)
		if to.Sub(from) >= maxAnalyticsRange*24*time.Hour {
			from = to.AddDate(0, 0, 1-maxAnalyticsRange)
		}
		q := analytics.Query{WorkspaceID: prop.WorkspaceID, PropertyID: prop.ID, Timezone: prop.Timezone, From: from, To: to, RawFrom: s.rawFrom(),
			Metrics: []string{"visitors", "visits", "pageviews"},
			Filters: []analytics.Filter{{Dimension: "utm_campaign", Op: "is", Values: []string{b.UtmCampaign}}}}
		ans, err := s.runAnalyticsQuery(ctx, prop, q, nil)
		if err != nil {
			return nil, err
		}
		t := ans.result.Totals
		if t.Visits == 0 {
			continue
		}
		out = append(out, oas.BroadcastCampaignVisits{PropertyId: prop.ID, PropertyName: prop.Name, Visitors: t.Visitors, Visits: t.Visits, Pageviews: t.Views})
	}
	return out, nil
}

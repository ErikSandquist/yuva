package analytics

import (
	"strings"
	"sync"

	"github.com/productdevbook/yuva/api/internal/analytics/lists"
)

type SourceKind uint8

const (
	SourceUnknown SourceKind = iota
	SourceSearch
	SourceSocial
	SourceAI
	SourceEmail
)

type source struct {
	name string
	kind SourceKind
	path string
}

type sourceTable struct {
	hosts    map[string][]source
	prefixes map[string]source
	suffixes map[string]source
	names    map[string]SourceKind
	spam     map[string]bool
}

// Yuva's own additions to the CC0 list: assistants it does not name yet and webmail hosts, whose
// visitors came from an e-mail. Where a site is listed under several kinds, the first added wins:
// these, then assistants, social networks and search engines.
var ownSources = []struct {
	kind       SourceKind
	name, host string
}{
	{SourceAI, "ChatGPT", "chatgpt.com"}, {SourceAI, "Perplexity", "perplexity.ai"}, {SourceAI, "Perplexity", "www.perplexity.ai"},
	{SourceAI, "Claude", "claude.ai"}, {SourceAI, "Gemini", "gemini.google.com"}, {SourceAI, "Copilot", "copilot.microsoft.com"},
	{SourceAI, "Copilot", "copilot.cloud.microsoft"}, {SourceAI, "Deepseek", "chat.deepseek.com"}, {SourceAI, "Meta AI", "meta.ai"},
	{SourceAI, "Grok", "grok.com"}, {SourceAI, "Mistral", "chat.mistral.ai"}, {SourceAI, "Poe", "poe.com"}, {SourceAI, "Phind", "phind.com"},
	{SourceAI, "You.com", "you.com"}, {SourceAI, "Kagi Assistant", "kagi.com/assistant"}, {SourceAI, "Duck.ai", "duck.ai"},
	{SourceEmail, "Gmail", "mail.google.com"}, {SourceEmail, "Outlook", "outlook.live.com"}, {SourceEmail, "Outlook", "outlook.office.com"},
	{SourceEmail, "Outlook", "outlook.office365.com"}, {SourceEmail, "Yahoo Mail", "mail.yahoo.com"}, {SourceEmail, "Proton Mail", "mail.proton.me"},
	{SourceEmail, "iCloud Mail", "www.icloud.com/mail"}, {SourceEmail, "Yandex Mail", "mail.yandex.ru"}, {SourceEmail, "Yandex Mail", "mail.yandex.com"},
	{SourceEmail, "Mail.ru", "e.mail.ru"}, {SourceEmail, "GMX", "navigator.gmx.net"}, {SourceEmail, "Web.de", "web.de"},
	{SourceEmail, "Fastmail", "app.fastmail.com"}, {SourceEmail, "Zoho Mail", "mail.zoho.com"}, {SourceEmail, "Tuta", "app.tuta.com"},
	{SourceEmail, "AOL Mail", "mail.aol.com"},
}

var sources = sync.OnceValue(func() *sourceTable {
	t := &sourceTable{hosts: map[string][]source{}, prefixes: map[string]source{}, suffixes: map[string]source{}, names: map[string]SourceKind{}, spam: map[string]bool{}}
	kinds := map[string]SourceKind{"search": SourceSearch, "social": SourceSocial, "ai": SourceAI}
	for _, o := range ownSources {
		t.add(o.kind, o.name, o.host)
	}
	for _, pass := range []string{"ai", "social", "search"} {
		for _, line := range strings.Split(lists.Sources, "\n") {
			if f := strings.Split(line, "\t"); len(f) == 3 && f[0] == pass {
				t.add(kinds[f[0]], f[1], f[2])
			}
		}
	}
	for _, line := range strings.Split(lists.Spammers, "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			t.spam[line] = true
		}
	}
	return t
})

func (t *sourceTable) add(kind SourceKind, name, host string) {
	host, path, _ := strings.Cut(strings.ToLower(host), "/")
	if path != "" {
		path = "/" + path
	}
	host = strings.TrimPrefix(host, "www.")
	s := source{name: name, kind: kind, path: path}
	if _, ok := t.names[strings.ToLower(name)]; !ok {
		t.names[strings.ToLower(name)] = kind
	}
	switch {
	case strings.HasSuffix(host, ".{}") && !strings.Contains(strings.TrimSuffix(host, ".{}"), "{}"):
		k := strings.TrimSuffix(host, ".{}")
		if _, ok := t.prefixes[k]; !ok && path == "" {
			t.prefixes[k] = s
		}
	case strings.HasPrefix(host, "{}.") && !strings.Contains(strings.TrimPrefix(host, "{}."), "{}"):
		k := strings.TrimPrefix(host, "{}.")
		if _, ok := t.suffixes[k]; !ok && path == "" {
			t.suffixes[k] = s
		}
	case !strings.Contains(host, "{}"):
		t.hosts[host] = append(t.hosts[host], s)
	}
}

func (t *sourceTable) lookup(host, path string) (source, bool) {
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	for h := host; h != ""; {
		var found *source
		for i, s := range t.hosts[h] {
			if s.path != "" && strings.HasPrefix(path, s.path) {
				return s, true
			}
			if s.path == "" && found == nil {
				found = &t.hosts[h][i]
			}
		}
		if found != nil {
			return *found, true
		}
		if first, _, ok := strings.Cut(h, "."); ok {
			if s, ok := t.prefixes[first]; ok {
				return s, true
			}
		}
		_, rest, ok := strings.Cut(h, ".")
		if s, found := t.suffixes[rest]; ok && found {
			return s, true
		}
		if !ok || !strings.Contains(rest, ".") {
			break
		}
		h = rest
	}
	return source{}, false
}

// LookupSource names the site a referrer is on (Google, Facebook, ChatGPT, Gmail) and says what
// kind it is.
func LookupSource(host, path string) (string, SourceKind) {
	s, ok := sources().lookup(host, path)
	if !ok {
		return "", SourceUnknown
	}
	return s.name, s.kind
}

// SpamReferrer reports a referrer host on the referrer spam list, or under a domain on it.
func SpamReferrer(host string) bool {
	t := sources()
	for h := strings.TrimPrefix(strings.ToLower(host), "www."); h != ""; {
		if t.spam[h] {
			return true
		}
		_, rest, ok := strings.Cut(h, ".")
		if !ok || !strings.Contains(rest, ".") {
			return false
		}
		h = rest
	}
	return false
}

func sourceKindOf(utmSource string) SourceKind {
	v := strings.ToLower(strings.TrimSpace(utmSource))
	if v == "" {
		return SourceUnknown
	}
	if k, ok := sources().names[v]; ok {
		return k
	}
	if strings.Contains(v, ".") {
		_, k := LookupSource(v, "")
		return k
	}
	return SourceUnknown
}

var emailMediums = []string{"email", "e-mail", "newsletter"}

// Classify names a visit's source and picks its channel from its referrer and acquisition: a UTM
// source is kept as sent, a known referrer by its site's name, any other referrer by its host.
// internal reports a referrer on another of the property's domains.
func Classify(referrerHost, referrerPath string, a Acquisition, internal bool) (string, Channel) {
	refName, refKind := "", SourceUnknown
	if referrerHost != "" {
		refName, refKind = LookupSource(referrerHost, referrerPath)
	}
	src := a.Source
	kind := sourceKindOf(a.Source)
	switch {
	case src == "" && refName != "":
		src, kind = refName, refKind
	case src == "":
		src = referrerHost
	case kind == SourceUnknown:
		kind = refKind
	}
	medium := strings.ToLower(a.Medium)
	switch {
	case medium == "cpc" || medium == "ppc" || strings.HasPrefix(medium, "paid"):
		if kind == SourceSocial {
			return src, ChannelPaidSocial
		}
		return src, ChannelPaidSearch
	case containsFold(emailMediums, medium) || kind == SourceEmail:
		return src, ChannelEmail
	case kind == SourceSearch:
		return src, ChannelOrganic
	case kind == SourceSocial:
		return src, ChannelSocial
	case kind == SourceAI:
		return src, ChannelAIAssistants
	case internal:
		return src, ChannelInternal
	case src != "":
		return src, ChannelReferral
	}
	return src, ChannelDirect
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

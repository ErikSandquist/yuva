package analytics

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxURLLength   = 2000
	maxFieldLength = 300
	EmailSegment   = ":email"
)

var emailShape = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s.]+$`)

func looksLikeEmail(s string) bool {
	return strings.Contains(s, "@") && emailShape.MatchString(strings.TrimSpace(s))
}

// Acquisition is what a page's query says about how the visitor came: only these keys are read, the
// rest of the query is dropped.
type Acquisition struct {
	Source, Medium, Campaign, Term, Content string
}

// Page is a page URL reduced to what is kept: host and path, and the acquisition from its query.
// Scrubbed reports that an e-mail-shaped part was replaced or dropped.
type Page struct {
	Host, Path  string
	Acquisition Acquisition
	Scrubbed    bool
}

// ParsePage keeps the host and path of an http(s) URL; with hash, the fragment is part of the path
// (hash routers).
func ParsePage(raw string, hash bool) (Page, bool) {
	if len(raw) > MaxURLLength {
		return Page{}, false
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return Page{}, false
	}
	p := Page{Host: strings.ToLower(u.Hostname())}
	path := u.Path
	if hash && u.Fragment != "" {
		path += "#" + u.Fragment
	}
	p.Path, p.Scrubbed = scrubPath(path)
	q := u.Query()
	pick := func(keys ...string) string {
		for _, k := range keys {
			v := Clean(q.Get(k), maxFieldLength)
			if v == "" {
				continue
			}
			if looksLikeEmail(v) {
				p.Scrubbed = true
				continue
			}
			return v
		}
		return ""
	}
	p.Acquisition = Acquisition{
		Source: pick("utm_source", "source", "ref"), Medium: pick("utm_medium"), Campaign: pick("utm_campaign"),
		Term: pick("utm_term"), Content: pick("utm_content"),
	}
	return p, true
}

// scrubPath replaces e-mail-shaped segments with :email.
func scrubPath(path string) (string, bool) {
	path = Clean(path, MaxURLLength)
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	scrubbed := false
	segs := strings.Split(path, "/")
	for i, s := range segs {
		if looksLikeEmail(s) {
			segs[i], scrubbed = EmailSegment, true
		}
	}
	return strings.Join(segs, "/"), scrubbed
}

// ParseReferrer keeps a referrer's host and path; none when it is not an http(s) URL or its host
// is one of own (the page's own host, the server's).
func ParseReferrer(raw string, own ...string) (host, path string) {
	if raw == "" || len(raw) > MaxURLLength {
		return "", ""
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", ""
	}
	host = strings.ToLower(u.Hostname())
	for _, o := range own {
		if strings.EqualFold(host, o) {
			return "", ""
		}
	}
	path, _ = scrubPath(u.Path)
	return Clean(host, 253), path
}

// Clean trims s, removes control characters and cuts it to max bytes on a rune boundary.
func Clean(s string, max int) string {
	s = strings.TrimSpace(s)
	if strings.IndexFunc(s, unicode.IsControl) >= 0 {
		s = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, s)
	}
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// DomainAllowed reports whether host is one of domains; `*.example.com` allows every subdomain of
// example.com but not example.com itself.
func DomainAllowed(domains []string, host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, d := range domains {
		if rest, ok := strings.CutPrefix(d, "*."); ok {
			if strings.HasSuffix(host, "."+rest) {
				return true
			}
			continue
		}
		if host == d {
			return true
		}
	}
	return false
}

// MatchPath matches a path against a pattern where `*` is one segment and `**` any number of them.
func MatchPath(pattern, path string) bool {
	if i := strings.IndexByte(path, '#'); i >= 0 && !strings.Contains(pattern, "#") {
		path = path[:i]
	}
	return matchSegments(strings.Split(strings.Trim(pattern, "/"), "/"), strings.Split(strings.Trim(path, "/"), "/"))
}

func matchSegments(pat, segs []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for i := 0; i <= len(segs); i++ {
				if matchSegments(pat[1:], segs[i:]) {
					return true
				}
			}
			return false
		}
		if len(segs) == 0 || (pat[0] != "*" && pat[0] != segs[0]) {
			return false
		}
		pat, segs = pat[1:], segs[1:]
	}
	return len(segs) == 0 || (len(segs) == 1 && segs[0] == "")
}

type PathRule struct {
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
}

// ApplyRules returns the replacement of the first rule whose pattern matches path, else path.
func ApplyRules(rules []PathRule, path string) string {
	for _, r := range rules {
		if MatchPath(r.Pattern, path) {
			return r.Replacement
		}
	}
	return path
}

// ScreenClass buckets a viewport width: 1 phone, 2 tablet, 3 laptop, 4 desktop; 0 unknown.
func ScreenClass(width int) int16 {
	switch {
	case width <= 0:
		return 0
	case width < 576:
		return 1
	case width < 992:
		return 2
	case width < 1440:
		return 3
	default:
		return 4
	}
}

var languageTag = regexp.MustCompile(`^[a-z]{2,3}$`)

// Language is the primary language of an Accept-Language header's first entry, such as "tr".
func Language(acceptLanguage string) string {
	first, _, _ := strings.Cut(acceptLanguage, ",")
	first, _, _ = strings.Cut(first, ";")
	base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(first)), "-")
	if !languageTag.MatchString(base) {
		return ""
	}
	return base
}

package design

import (
	"io"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

var (
	blockTags  = []string{"p", "ul", "ol", "li"}
	inlineTags = []string{"br", "strong", "em", "u", "s", "code", "a"}
	richPolicy = newPolicy(append(slices.Clone(blockTags), inlineTags...))
	inlinePol  = newPolicy(inlineTags)
	urlSchemes = []string{"http", "https", "mailto", "tel"}
)

func newPolicy(tags []string) *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	for _, t := range tags {
		if t != "a" {
			p.AllowElements(t)
		}
	}
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes(urlSchemes...)
	p.RequireParseableURLs(true)
	return p
}

// mergeSpan is replaced before the policy sees the text, so a merge field in a link's query (or a
// link that is only {{ unsubscribe_url }}) parses as a URL; checkURL judges where it stands.
var mergeSpan = regexp.MustCompile(`\{\{[^{}]*\}\}`)

const mergeMask = "https://merge.invalid/"

type node struct {
	kind  html.TokenType
	tag   string
	href  string
	text  string
	attrs int
}

// tokens reads rich text into a canonical token list: tag names, the href, and unescaped text, so
// that two spellings of the same markup (`<br>`, `<br/>`, `&#39;`, `'`) compare equal.
func tokens(s string) []node {
	z := html.NewTokenizer(strings.NewReader(s))
	var out []node
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() != io.EOF {
				out = append(out, node{kind: html.ErrorToken})
			}
			return out
		case html.TextToken:
			t := string(z.Text())
			if n := len(out); n > 0 && out[n-1].kind == html.TextToken {
				out[n-1].text += t
			} else {
				out = append(out, node{kind: tt, text: t})
			}
		case html.StartTagToken, html.SelfClosingTagToken, html.EndTagToken:
			name, more := z.TagName()
			n := node{kind: tt, tag: string(name)}
			if tt == html.SelfClosingTagToken {
				n.kind = html.StartTagToken
			}
			for more {
				var k, v []byte
				k, v, more = z.TagAttr()
				n.attrs++
				if string(k) == "href" {
					n.href = string(v)
				}
			}
			out = append(out, n)
		default:
			out = append(out, node{kind: tt, text: string(z.Raw())})
		}
	}
}

func sameTokens(a, b []node) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// checkRich reports whether rich text survives the policy unchanged, and every link is a URL a
// design may hold.
func checkRich(s string, inline bool) (ok bool, badURL string) {
	p := richPolicy
	if inline {
		p = inlinePol
	}
	masked := mergeSpan.ReplaceAllString(s, mergeMask)
	if !sameTokens(tokens(masked), tokens(p.Sanitize(masked))) {
		return false, ""
	}
	in := tokens(s)
	open := map[string]int{}
	for _, n := range in {
		switch n.kind {
		case html.StartTagToken:
			if n.tag != "br" {
				open[n.tag]++
			}
			if n.tag == "a" {
				if code := checkURL(n.href); code != "" {
					return true, n.href
				}
			}
		case html.EndTagToken:
			open[n.tag]--
		case html.CommentToken, html.DoctypeToken, html.ErrorToken:
			return false, ""
		}
	}
	for _, c := range open {
		if c != 0 {
			return false, ""
		}
	}
	return true, ""
}

// checkURL answers "" for a link a design may hold, else the issue code.
func checkURL(raw string) string {
	if raw == "" || len(raw) > 2048 {
		return "invalid_url"
	}
	segs, bad := parseMerge(raw)
	if len(bad) > 0 {
		return "unknown_merge_field"
	}
	if len(segs) == 1 && segs[0].field != nil && isLinkField(segs[0].field.Name) {
		return ""
	}
	q := strings.IndexByte(raw, '?')
	var plain strings.Builder
	at := 0
	for _, sg := range segs {
		if sg.field != nil {
			if q < 0 || at < q {
				return "merge_field_in_url"
			}
			plain.WriteString("x")
			at += 4
			continue
		}
		plain.WriteString(sg.text)
		at += len(sg.text)
	}
	p := plain.String()
	if strings.ContainsFunc(p, func(r rune) bool { return r <= ' ' || r == 0x7f }) {
		return "invalid_url"
	}
	u, err := url.Parse(p)
	if err != nil || !slices.Contains(urlSchemes, strings.ToLower(u.Scheme)) {
		return "invalid_url"
	}
	if (u.Scheme == "http" || u.Scheme == "https") && u.Host == "" {
		return "invalid_url"
	}
	if (u.Scheme == "mailto" || u.Scheme == "tel") && u.Opaque == "" {
		return "invalid_url"
	}
	return ""
}

// richFields lists the merge fields in rich text's text and in its links.
func richFields(s string) (fields []Field, bad []string) {
	for _, n := range tokens(s) {
		var src string
		switch {
		case n.kind == html.TextToken:
			src = n.text
		case n.kind == html.StartTagToken && n.tag == "a":
			src = n.href
		default:
			continue
		}
		segs, b := parseMerge(src)
		bad = append(bad, b...)
		for _, sg := range segs {
			if sg.field != nil {
				fields = append(fields, *sg.field)
			}
		}
	}
	return fields, bad
}

// SanitizeRich cuts s down to the markup a design's text holds (inline: a heading's).
func SanitizeRich(s string, inline bool) string {
	if inline {
		return inlinePol.Sanitize(s)
	}
	return richPolicy.Sanitize(s)
}

// ValidRich reports whether Validate accepts s as the text of a text block (inline: a heading).
func ValidRich(s string, inline bool) bool {
	ok, bad := checkRich(s, inline)
	if !ok || bad != "" {
		return false
	}
	_, unknown := richFields(s)
	return len(unknown) == 0
}

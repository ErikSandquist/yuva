package api

import (
	"bytes"
	"context"
	"html"
	"regexp"
	"strconv"
	"strings"
	"uuid"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/productdevbook/yuva/api/internal/store"
)

var (
	helpMarkdown = goldmark.New(
		goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.Linkify),
		goldmark.WithParserOptions(parser.WithAutoHeadingID(), parser.WithASTTransformers(util.Prioritized(helpImageFilter{}, 100))),
	)
	helpImagesKey   = parser.NewContextKey()
	helpImageTokens = regexp.MustCompile(`/i/([a-z2-7]{26})\b`)

	helpHTML = func() *bluemonday.Policy {
		p := bluemonday.UGCPolicy()
		p.RequireNoFollowOnLinks(false)
		p.AddTargetBlankToFullyQualifiedLinks(true)
		p.AllowAttrs("loading").Matching(regexp.MustCompile(`^lazy$`)).OnElements("img")
		return p
	}()
)

type helpImageSet struct {
	urls map[string]uuid.UUID
}

func (s helpImageSet) ids() []uuid.UUID {
	out := make([]uuid.UUID, 0, len(s.urls))
	seen := map[uuid.UUID]bool{}
	for _, id := range s.urls {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func (s *Server) helpImages(ctx context.Context, q *store.Queries, ws uuid.UUID, bodies ...string) (helpImageSet, error) {
	set := helpImageSet{urls: map[string]uuid.UUID{}}
	var tokens []string
	for _, b := range bodies {
		for _, m := range helpImageTokens.FindAllStringSubmatch(b, -1) {
			tokens = append(tokens, m[1])
		}
	}
	if len(tokens) == 0 {
		return set, nil
	}
	rows, err := q.ListEmailImagesByTokens(ctx, store.ListEmailImagesByTokensParams{WorkspaceID: ws, Tokens: tokens})
	if err != nil {
		return set, err
	}
	for _, r := range rows {
		set.urls[s.imageURL(r.Token)] = r.ID
		set.urls[s.auth.PublicURL+"/i/"+r.Token] = r.ID
	}
	return set, nil
}

type helpImageFilter struct{}

func (helpImageFilter) Transform(doc *ast.Document, _ text.Reader, pc parser.Context) {
	set, _ := pc.Get(helpImagesKey).(helpImageSet)
	var imgs []*ast.Image
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if img, ok := n.(*ast.Image); ok && entering {
			imgs = append(imgs, img)
		}
		return ast.WalkContinue, nil
	})
	for _, img := range imgs {
		if _, ok := set.urls[string(img.Destination)]; ok {
			img.SetAttributeString("loading", []byte("lazy"))
			continue
		}
		imageAsLink(img)
	}
}

type helpIDs struct {
	seen map[string]bool
}

func (h *helpIDs) Generate(value []byte, _ ast.NodeKind) []byte {
	base := helpSlug(string(value), "section")
	id := base
	for i := 1; h.seen[id]; i++ {
		id = base + "-" + strconv.Itoa(i)
	}
	h.seen[id] = true
	return []byte(id)
}

func (h *helpIDs) Put(value []byte) { h.seen[string(value)] = true }

func renderHelp(body string, images helpImageSet) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	pc := parser.NewContext(parser.WithIDs(&helpIDs{seen: map[string]bool{}}))
	pc.Set(helpImagesKey, images)
	var buf bytes.Buffer
	if err := helpMarkdown.Convert([]byte(body), &buf, parser.WithContext(pc)); err != nil {
		return "<p>" + html.EscapeString(body) + "</p>"
	}
	return helpHTML.Sanitize(buf.String())
}

// A title in a script without Latin letters leaves nothing, so it takes the fallback.
func helpSlug(title, fallback string) string {
	if s := slugify(title); s != "" {
		return s
	}
	return fallback
}

var (
	helpMdImage = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	helpMdLink  = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	helpMdMarks = strings.NewReplacer("**", "", "__", "", "~~", "", "`", "", "#", "", ">", "", "|", " ", "*", "")
)

// Markdown read as text without rendering: a draft search result needs only its start.
func helpPlainExcerpt(md string) string {
	return htmlExcerpt(helpMdMarks.Replace(helpMdLink.ReplaceAllString(helpMdImage.ReplaceAllString(md, "$1"), "$1")))
}

var helpSearchConfigs = map[string]string{
	"ar": "arabic", "hy": "armenian", "eu": "basque", "ca": "catalan", "da": "danish", "nl": "dutch", "en": "english",
	"fi": "finnish", "fr": "french", "de": "german", "el": "greek", "hi": "hindi", "hu": "hungarian", "id": "indonesian",
	"ga": "irish", "it": "italian", "lt": "lithuanian", "ne": "nepali", "nb": "norwegian", "nn": "norwegian", "no": "norwegian",
	"pt": "portuguese", "ro": "romanian", "ru": "russian", "sr": "serbian", "es": "spanish", "sv": "swedish", "ta": "tamil",
	"tr": "turkish", "yi": "yiddish",
}

func helpSearchConfig(locale string) string {
	base, _, _ := strings.Cut(locale, "-")
	if c, ok := helpSearchConfigs[base]; ok {
		return c
	}
	return "simple"
}

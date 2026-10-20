package api

import (
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/mail/design"
)

const mailImageToken = "abcdefghijklmnopqrstuvwxyz"

func mailDocument(blocks []design.Block) design.Document {
	return design.Document{Schema: design.Schema, Blocks: blocks, Theme: design.Theme{
		Width: 600, Font: "system", Radius: 8,
		Colors: design.Colors{Page: "#f4f4f5", Card: "#ffffff", Text: "#18181b", Muted: "#5f5f68", Accent: "#2563eb", Link: "#1d4ed8", Border: "#e4e4e7"},
	}}
}

func mapBody(t *testing.T, body string) []design.Block {
	t.Helper()
	id := uuid.NewV7()
	set := helpImageSet{urls: map[string]uuid.UUID{"https://yuva.example/i/" + mailImageToken: id}}
	blocks, err := mapChangelogHTML(renderHelp(body, set), "https://yuva.example/changelog/acme/en/entries/1-hello", "See the full entry", map[string]uuid.UUID{mailImageToken: id})
	if err != nil {
		t.Fatal(err)
	}
	if issues := design.Validate(mailDocument(blocks), design.Options{}); len(issues) > 0 {
		t.Fatalf("the mapped design is refused: %v", issues)
	}
	return blocks
}

func TestChangelogMappingKeepsOnlyWhatADesignHolds(t *testing.T) {
	t.Parallel()
	body := "Intro with **bold**, a [link](https://example.com/a?x=1) and {{ contact.email }} typed by the author.\n\n" +
		"## A heading\n\n- one\n- two\n\n> quoted words\n\n" +
		"![the shot](https://yuva.example/i/" + mailImageToken + ")\n\n" +
		"| a | b |\n|---|---|\n| 1 | 2 |\n\n```go\nfmt.Println(1)\n```\n\n### Small heading\n\nLast [bad](javascript:alert(1)) words <script>alert(1)</script>.\n"
	blocks := mapBody(t, body)
	var kinds []string
	for _, b := range blocks {
		kinds = append(kinds, b.Type)
	}
	if got := strings.Join(kinds, ","); got != "text,heading,text,image,text,heading,text" {
		t.Fatalf("blocks: %s", got)
	}
	if blocks[1].Level != 2 || blocks[5].Level != 3 || blocks[1].Text != "A heading" {
		t.Fatalf("headings: %+v %+v", blocks[1], blocks[5])
	}
	if blocks[3].ImageID == "" || blocks[3].Alt != "the shot" {
		t.Fatalf("image: %+v", blocks[3])
	}
	if !strings.Contains(blocks[2].HTML, "<ul>") || !strings.Contains(blocks[2].HTML, "<em>quoted words</em>") {
		t.Fatalf("list and quote: %s", blocks[2].HTML)
	}
	if n := strings.Count(blocks[4].HTML, "See the full entry"); n != 1 || strings.Contains(blocks[4].HTML, "<table") || strings.Contains(blocks[4].HTML, "fmt.Println") {
		t.Fatalf("table and code: %s", blocks[4].HTML)
	}
	for _, b := range blocks {
		all := b.HTML + b.Text
		if strings.Contains(all, "{{") || strings.Contains(all, "<script") || strings.Contains(all, "javascript:") {
			t.Fatalf("unsafe text left in %+v", b)
		}
	}
	if !strings.Contains(blocks[0].HTML, `<a href="https://example.com/a?x=1">link</a>`) || !strings.Contains(blocks[0].HTML, "<strong>bold</strong>") {
		t.Fatalf("inline markup: %s", blocks[0].HTML)
	}
}

func TestChangelogMappingDropsForeignImagesAndMergeLinks(t *testing.T) {
	t.Parallel()
	blocks := mapBody(t, "![x](https://elsewhere.example/a.png)\n\n[merge]({{ unsubscribe_url }}) and [more](https://example.com/{{contact.email}})\n")
	if len(blocks) != 1 || blocks[0].Type != "text" {
		t.Fatalf("blocks: %+v", blocks)
	}
	if strings.Contains(blocks[0].HTML, "{{") || strings.Contains(blocks[0].HTML, "<img") {
		t.Fatalf("text: %s", blocks[0].HTML)
	}
}

func TestChangelogMappingStaysInsideTheBlockLimit(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	for i := range 400 {
		b.WriteString("## Heading ")
		b.WriteString(strings.Repeat("x", i%7+1))
		b.WriteString("\n\nParagraph.\n\n")
	}
	blocks := mapBody(t, b.String())
	if len(blocks) > changelogMailBlocks+2 || len(blocks) < changelogMailBlocks {
		t.Fatalf("blocks: %d", len(blocks))
	}
	last := blocks[len(blocks)-1]
	if last.Type != "text" || !strings.Contains(last.HTML, "See the full entry") {
		t.Fatalf("the cut text points nowhere: %+v", last)
	}
}

func TestChangelogReleaseLine(t *testing.T) {
	t.Parallel()
	at := mustTime(t, "2026-10-20T09:00:00Z")
	if got := changelogReleaseLine("tr", []string{"new", "improved"}, at); got != "YENİ · İYİLEŞTİRİLDİ · 20 EKİM 2026" {
		t.Fatalf("tr: %s", got)
	}
	if got := changelogReleaseLine("de", []string{"fixed"}, at); got != "BEHOBEN · 20. OKTOBER 2026" {
		t.Fatalf("de: %s", got)
	}
	if got := changelogReleaseLine("en", []string{"new"}, at); got != "NEW · 20 OCTOBER 2026" {
		t.Fatalf("en: %s", got)
	}
}

func mustTime(t *testing.T, v string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, v)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

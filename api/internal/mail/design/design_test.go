package design

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

var update = flag.Bool("update", false, "rewrite the golden files")

const theme = `{"width": 600, "font": "system", "radius": 8,
  "colors": {"page": "#f4f4f5", "card": "#ffffff", "text": "#18181b", "muted": "#5f5f68", "accent": "#2563eb", "link": "#1d4ed8", "border": "#e4e4e7"}}`

func doc(t *testing.T, blocks string, extra ...string) Document {
	t.Helper()
	th := theme
	if len(extra) > 0 {
		th = extra[0]
	}
	raw := `{"schema": 1, "theme": ` + th + `, "blocks": [` + blocks + `]}`
	d, issues, err := Parse([]byte(raw))
	if err != nil || len(issues) > 0 {
		t.Fatalf("parse: %v %v", err, issues)
	}
	if is := Validate(d, Options{}); len(is) > 0 {
		t.Fatalf("validate: %v", is)
	}
	return d
}

func input(d Document) Input {
	return Input{
		Doc: d, Subject: "Product news", Lang: "en",
		Sender:       Sender{LegalName: "Northwind Ltd", PostalAddress: "12 Harbour Street, Springfield"},
		SubscribedAt: time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC),
		Values: Values{ContactName: "Ada Lovelace", ContactEmail: "ada@example.com", InboxName: "Northwind", ListName: "Product news",
			UnsubscribeURL: "https://yuva.example.com/m/tok", PreferencesURL: "https://yuva.example.com/m/tok", Attributes: map[string]string{"plan": "Pro"}},
		Image: func(b Block) (Image, bool) {
			return Image{URL: "https://yuva.example.com/i/" + strings.ReplaceAll(b.ImageID, "-", "")[:26], Width: 1200, Height: 600}, b.ImageID != ""
		},
		Icon: func(n, v string) string { return "https://yuva.example.com/i/s/" + n + "-" + v + ".png" },
	}
}

func TestRenderGolden(t *testing.T) {
	img := `"image_id": "0190a3c4-7b2e-7d1f-9a3b-1c2d3e4f5a6b"`
	cases := map[string]struct {
		blocks string
		theme  string
		mod    func(*Input)
	}{
		"heading":   {blocks: `{"id": "a", "type": "heading", "level": 1, "text": "Hello <em>{{ contact.first_name | \"there\" }}</em>"}, {"id": "b", "type": "heading", "level": 2, "text": "Second"}, {"id": "c", "type": "heading", "level": 3, "text": "Third", "align": "center"}`},
		"text":      {blocks: `{"id": "t", "type": "text", "html": "<p>One <strong>two</strong> <a href=\"https://example.com/?a=1&amp;b={{ contact.email }}\">three</a></p><ol><li>first</li><li>second</li></ol><p>Last<br>line with <code>code</code></p>"}`},
		"image":     {blocks: `{"id": "i", "type": "image", ` + img + `, "alt": "A harbour at dawn", "link": "https://example.com/"}, {"id": "j", "type": "image", ` + img + `, "alt": "", "decorative": true, "width": 200, "align": "right", "plate": true}, {"id": "k", "type": "image", "alt": "Missing"}`},
		"button":    {blocks: `{"id": "b", "type": "button", "label": "Upgrade to {{ contact.attributes.plan | \"Pro\" }}", "url": "https://example.com/up"}, {"id": "c", "type": "button", "label": "Wide", "url": "https://example.com/", "full_width": true, "color": "#fde68a"}`},
		"divider":   {blocks: `{"id": "d", "type": "divider"}, {"id": "e", "type": "divider", "width": 40, "thickness": 3, "color": "#2563eb", "align": "left"}, {"id": "s", "type": "spacer", "height": 32}`},
		"social":    {blocks: `{"id": "s", "type": "social", "items": [{"network": "github", "url": "https://github.com/example"}, {"network": "mastodon", "url": "https://mastodon.example/@n"}]}, {"id": "t", "type": "social", "style": "text", "items": [{"network": "website", "url": "https://example.com"}]}`},
		"columns":   {blocks: `{"id": "c", "type": "columns", "gap": 24, "columns": [{"width": 33, "blocks": [{"id": "i", "type": "image", ` + img + `, "alt": "Icon"}]}, {"width": 67, "blocks": [{"id": "h", "type": "heading", "level": 3, "text": "Faster search"}, {"id": "t", "type": "text", "html": "<p>Results in half the time.</p>"}]}]}, {"id": "n", "type": "columns", "stack": false, "columns": [{"width": 50, "blocks": [{"id": "x", "type": "text", "html": "<p>Left</p>"}]}, {"width": 50, "blocks": [{"id": "y", "type": "text", "html": "<p>Right</p>"}]}]}`},
		"section":   {blocks: `{"id": "s", "type": "section", "background": "#1e3a8a", "padding": [24, 0, 24, 0], "border_radius": 12, "blocks": [{"id": "h", "type": "heading", "level": 2, "text": "On blue", "color": "#ffffff"}, {"id": "b", "type": "button", "label": "Go", "url": "https://example.com/", "color": "#ffffff"}]}`},
		"footer":    {blocks: `{"id": "f", "type": "footer", "html": "<p>Northwind, the harbour people.</p>"}`},
		"hide":      {blocks: `{"id": "m", "type": "text", "html": "<p>Wide screens only</p>", "hide": "mobile"}, {"id": "d", "type": "text", "html": "<p>Phones only</p>", "hide": "desktop"}`},
		"preheader": {blocks: `{"id": "t", "type": "text", "html": "<p>Body</p>"}`, mod: func(in *Input) { in.Preheader = "Hi {{ contact.first_name }}, three things changed" }},
		"rtl": {blocks: `{"id": "h", "type": "heading", "level": 1, "text": "مرحبا"}, {"id": "t", "type": "text", "html": "<ul><li>واحد</li></ul>"}, {"id": "c", "type": "columns", "columns": [{"width": 50, "blocks": [{"id": "x", "type": "text", "html": "<p>يمين</p>"}]}, {"width": 50, "blocks": [{"id": "y", "type": "text", "html": "<p>يسار</p>"}]}]}`,
			theme: strings.Replace(theme, `"width": 600`, `"width": 600, "direction": "rtl"`, 1)},
		"dark":     {blocks: `{"id": "t", "type": "text", "html": "<p>Dark <a href=\"https://example.com\">link</a></p>"}, {"id": "b", "type": "button", "label": "Go", "url": "https://example.com/"}`, mod: func(in *Input) { in.Dark = true }},
		"darkset":  {blocks: `{"id": "t", "type": "text", "html": "<p>Own dark colours</p>"}`, theme: strings.Replace(theme, `"radius": 8,`, `"radius": 8, "dark": {"page": "#000000", "card": "#111827", "text": "#f9fafb"},`, 1)},
		"tr":       {blocks: `{"id": "t", "type": "text", "html": "<p>Merhaba</p>"}`, mod: func(in *Input) { in.Lang = "tr" }},
		"de":       {blocks: `{"id": "t", "type": "text", "html": "<p>Hallo</p>"}`, mod: func(in *Input) { in.Lang = "de"; in.SubscribedAt = time.Time{} }},
		"nosender": {blocks: `{"id": "t", "type": "text", "html": "<p>Hi</p>"}`, mod: func(in *Input) { in.Sender = Sender{} }},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var d Document
			if c.theme != "" {
				d = doc(t, c.blocks, c.theme)
			} else {
				d = doc(t, c.blocks)
			}
			in := input(d)
			if c.mod != nil {
				c.mod(&in)
			}
			out, err := Render(in)
			if err != nil {
				t.Fatal(err)
			}
			checkWellFormed(t, out.HTML)
			warn, _ := json.MarshalIndent(out.Warnings, "", "  ")
			for ext, body := range map[string]string{"html": out.HTML, "txt": out.Text, "warnings.json": string(warn) + "\n"} {
				path := filepath.Join("testdata", "golden", name+"."+ext)
				if *update {
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
						t.Fatal(err)
					}
					continue
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(want) != body {
					t.Errorf("%s differs from its golden file (go test -update to accept)", path)
				}
			}
		})
	}
}

// checkWellFormed tokenizes the HTML outside conditional comments and checks that every opened
// element closes in order.
func checkWellFormed(t *testing.T, doc string) {
	t.Helper()
	z := html.NewTokenizer(strings.NewReader(doc))
	var open []string
	void := map[string]bool{"meta": true, "br": true, "img": true, "hr": true}
	for {
		switch z.Next() {
		case html.ErrorToken:
			if z.Err() != io.EOF {
				t.Fatal(z.Err())
			}
			if len(open) != 0 {
				t.Fatalf("unclosed %v", open)
			}
			return
		case html.StartTagToken:
			n, _ := z.TagName()
			if !void[string(n)] {
				open = append(open, string(n))
			}
		case html.EndTagToken:
			n, _ := z.TagName()
			if len(open) == 0 || open[len(open)-1] != string(n) {
				t.Fatalf("</%s> closes %v", n, open)
			}
			open = open[:len(open)-1]
		}
	}
}

func TestDarkModeHasOutlookSelectors(t *testing.T) {
	out, err := Render(input(doc(t, `{"id": "t", "type": "text", "html": "<p>x</p>"}`)))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"@media (prefers-color-scheme:dark)", "[data-ogsc] .yd", "[data-ogsb] .yd", `<meta name="color-scheme" content="light dark">`} {
		if !strings.Contains(out.HTML, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestDerivedDarkTextReads(t *testing.T) {
	for _, light := range []string{"#18181b", "#1d4ed8", "#5f5f68", "#7c3aed", "#0f766e"} {
		out, err := Render(input(doc(t, `{"id": "t", "type": "text", "color": "`+light+`", "html": "<p>x</p>"}`)))
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range out.Warnings {
			if w.BlockID == "t" && w.Code == "low_contrast" {
				t.Errorf("%s: %v", light, w)
			}
		}
	}
	out, _ := Render(input(doc(t, `{"id": "t", "type": "text", "color": "#d4d4d8", "html": "<p>x</p>"}`)))
	if !slices.ContainsFunc(out.Warnings, func(i Issue) bool { return i.BlockID == "t" && i.Code == "low_contrast" }) {
		t.Error("pale grey on white is not reported")
	}
}

func TestPreheaderIsPadded(t *testing.T) {
	in := input(doc(t, `{"id": "t", "type": "text", "html": "<p>x</p>"}`))
	in.Preheader = "Short"
	out, _ := Render(in)
	if strings.Count(out.HTML, "&#847;&zwnj;&nbsp;") != 145 || !strings.Contains(out.HTML, "mso-hide:all") {
		t.Error("the preheader is not padded to 150 characters")
	}
}

func TestSizeWarnings(t *testing.T) {
	para := "<p>" + strings.Repeat("Lorem ipsum dolor sit amet. ", 640) + "</p>"
	blocks := []string{}
	for i := range 6 {
		blocks = append(blocks, `{"id": "t`+string(rune('a'+i))+`", "type": "text", "html": "`+para+`"}`)
	}
	out, err := Render(input(doc(t, strings.Join(blocks, ","))))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(out.Warnings, func(i Issue) bool { return i.Code == "html_clipped" }) {
		t.Errorf("%d bytes without html_clipped", out.Size)
	}
	d := doc(t, strings.Join(blocks, ","))
	for len(d.Blocks) < 40 {
		d.Blocks = append(d.Blocks, d.Blocks[0])
	}
	if _, err := Render(input(d)); err != ErrHTMLTooLarge {
		t.Errorf("err %v, want ErrHTMLTooLarge", err)
	}
}

func TestTextPartWrapsOutsideURLs(t *testing.T) {
	long := "https://example.com/" + strings.Repeat("a", 120)
	out, _ := Render(input(doc(t, `{"id": "t", "type": "text", "html": "<p>`+strings.Repeat("word ", 40)+`<a href=\"`+long+`\">link</a></p>"}`)))
	for _, line := range strings.Split(out.Text, "\n") {
		if len([]rune(line)) > 78 && !strings.Contains(line, "https://") {
			t.Errorf("line over 78 columns: %q", line)
		}
	}
	if !strings.Contains(out.Text, "("+long+")") {
		t.Error("the URL was broken")
	}
}

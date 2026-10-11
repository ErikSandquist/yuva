package gallery

import (
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/productdevbook/yuva/api/internal/mail/design"
	"github.com/productdevbook/yuva/api/internal/mail/style"
)

const (
	maxAssetBytes    = 300 << 10
	maxTemplateBytes = 1 << 20
	maxRenderedBytes = 90 << 10
)

func TestGalleryLoads(t *testing.T) {
	ts, err := List()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"newsletter", "product-update", "announcement", "welcome", "event", "digest"}
	var got []string
	for _, tpl := range ts {
		got = append(got, tpl.Slug)
	}
	if !slices.Equal(got, want) {
		t.Errorf("templates %v, want %v", got, want)
	}
	if _, err := fs.Stat(Embedded(), "LICENSE"); err != nil {
		t.Error("the MIT license is not copied with the templates")
	}
}

func TestEveryTemplate(t *testing.T) {
	ts, err := List()
	if err != nil {
		t.Fatal(err)
	}
	fsys := Embedded()
	for _, tpl := range ts {
		t.Run(tpl.Slug, func(t *testing.T) {
			for _, l := range Locales {
				if Text(tpl.Name, l) == "" || tpl.Name[l] == "" || tpl.Description[l] == "" {
					t.Errorf("meta has no %s name or description", l)
				}
				if !slices.Contains(tpl.Locales, l) {
					t.Errorf("no design.%s.json", l)
				}
			}
			checkAssets(t, fsys, tpl)
			var english string
			for _, l := range tpl.Locales {
				d, _, err := tpl.Document(fsys, l)
				if err != nil {
					t.Fatal(err)
				}
				checkContent(t, l, d)
				out, err := Render(fsys, tpl, l, PreviewAssetURL, PreviewIconURL)
				if err != nil {
					t.Fatal(err)
				}
				if out.Size > maxRenderedBytes {
					t.Errorf("%s: %d bytes of HTML, at most %d", l, out.Size, maxRenderedBytes)
				}
				for _, w := range out.Warnings {
					t.Errorf("%s: warning %s on %q (%s)", l, w.Code, w.BlockID, w.Field)
				}
				preview, err := fs.ReadFile(fsys, path.Join(tpl.Slug, "preview."+l+".html"))
				if err != nil || string(preview) != out.HTML {
					t.Errorf("%s: preview.%s.html is not current; run yuva templates render and go generate ./internal/gallery", l, l)
				}
				if l == "en" {
					english = out.Text
				} else if out.Text == english {
					t.Errorf("%s: the text is English", l)
				}
			}
		})
	}
}

func checkAssets(t *testing.T, fsys fs.FS, tpl Template) {
	t.Helper()
	files, err := fs.ReadDir(fsys, path.Join(tpl.Slug, "assets"))
	if err != nil {
		return
	}
	total := 0
	for _, f := range files {
		b, err := Asset(fsys, tpl.Slug, f.Name())
		if err != nil {
			t.Fatal(err)
		}
		total += len(b)
		if len(b) > maxAssetBytes {
			t.Errorf("%s is %d bytes, at most %d", f.Name(), len(b), maxAssetBytes)
		}
		w, _, err := AssetSize(b)
		if err != nil || w > 1200 {
			t.Errorf("%s: %d pixels wide, %v", f.Name(), w, err)
		}
	}
	if total > maxTemplateBytes {
		t.Errorf("assets are %d bytes, at most %d", total, maxTemplateBytes)
	}
}

// checkContent holds the rules of the contribution guide the renderer does not warn about: alt
// text on every image, merge fields with fallbacks, example.com links, and text contrast in the
// theme's light and derived dark palette.
func checkContent(t *testing.T, locale string, d design.Document) {
	t.Helper()
	c := d.Theme.Colors
	if r := style.Contrast(c.Text, c.Card); r < 4.5 {
		t.Errorf("%s: text on card %.2f:1", locale, r)
	}
	if r := style.Contrast(c.Muted, c.Page); r < 4.5 {
		t.Errorf("%s: muted on page %.2f:1", locale, r)
	}
	d.Walk(func(b design.Block) {
		if b.Type == "image" {
			if b.Asset == "" {
				t.Errorf("%s: image %s names no asset", locale, b.ID)
			}
			if !b.Decorative && strings.TrimSpace(b.Alt) == "" {
				t.Errorf("%s: image %s has no alt text", locale, b.ID)
			}
		}
		for _, s := range []string{b.Text, b.HTML, b.Label} {
			for _, f := range design.Fields(s) {
				if !f.HasFallback && strings.HasPrefix(f.Name, "contact.") {
					t.Errorf("%s: %s uses %s without a fallback", locale, b.ID, f.Name)
				}
			}
		}
		for _, u := range append([]string{b.URL, b.Link}, links(b.HTML+b.Text)...) {
			if u != "" && !strings.HasPrefix(u, "https://example.com") && !strings.HasPrefix(u, "https://mastodon.example") {
				t.Errorf("%s: %s links outside example.com: %s", locale, b.ID, u)
			}
		}
		for _, it := range b.Items {
			if !strings.HasPrefix(it.URL, "https://example.com") && !strings.HasPrefix(it.URL, "https://mastodon.example") {
				t.Errorf("%s: %s links outside example.com: %s", locale, b.ID, it.URL)
			}
		}
	})
}

func links(html string) []string {
	var out []string
	for _, part := range strings.Split(html, `href="`)[1:] {
		u, _, _ := strings.Cut(part, `"`)
		out = append(out, u)
	}
	return out
}

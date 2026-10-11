// Package gallery serves the starter e-mail designs of templates/email (MIT), copied into data by
// go generate.
package gallery

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/productdevbook/yuva/api/internal/mail/design"
)

//go:generate go run ./copy ../../../templates/email data

//go:embed all:data
var data embed.FS

// Locales are the languages a template may have its own text in; English is required.
var Locales = []string{"en", "tr", "de"}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type Meta struct {
	Slug        string            `json:"slug"`
	Order       int               `json:"order"`
	Category    string            `json:"category"`
	Name        map[string]string `json:"name"`
	Description map[string]string `json:"description"`
	Author      string            `json:"author"`
	Added       string            `json:"added"`
}

type Template struct {
	Meta
	Locales []string
}

// Text is a meta text in locale, else English.
func Text(m map[string]string, locale string) string {
	if v, ok := m[locale]; ok && v != "" {
		return v
	}
	return m["en"]
}

// Embedded is the gallery built into the server.
func Embedded() fs.FS {
	sub, err := fs.Sub(data, "data")
	if err != nil {
		panic(err)
	}
	return sub
}

// Load reads every template folder of fsys.
func Load(fsys fs.FS) ([]Template, error) {
	dirs, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var out []Template
	for _, d := range dirs {
		if !d.IsDir() || !slugRe.MatchString(d.Name()) {
			continue
		}
		b, err := fs.ReadFile(fsys, path.Join(d.Name(), "meta.json"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", d.Name(), err)
		}
		var t Template
		if err := json.Unmarshal(b, &t.Meta); err != nil {
			return nil, fmt.Errorf("%s/meta.json: %w", d.Name(), err)
		}
		if t.Slug != d.Name() {
			return nil, fmt.Errorf("%s/meta.json: slug %q is not the folder's name", d.Name(), t.Slug)
		}
		for _, l := range Locales {
			if _, err := fs.Stat(fsys, path.Join(d.Name(), "design."+l+".json")); err == nil {
				t.Locales = append(t.Locales, l)
			}
		}
		if !slices.Contains(t.Locales, "en") {
			return nil, fmt.Errorf("%s: design.en.json is missing", d.Name())
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Slug < out[j].Slug
	})
	return out, nil
}

var (
	once      sync.Once
	templates []Template
	loadErr   error
)

// List is the embedded gallery in order.
func List() ([]Template, error) {
	once.Do(func() { templates, loadErr = Load(Embedded()) })
	return templates, loadErr
}

// Find is an embedded template by slug.
func Find(slug string) (Template, bool) {
	ts, err := List()
	if err != nil {
		return Template{}, false
	}
	for _, t := range ts {
		if t.Slug == slug {
			return t, true
		}
	}
	return Template{}, false
}

// DocumentLocale is the locale whose design is used for locale: its own, else English.
func (t Template) DocumentLocale(locale string) string {
	if slices.Contains(t.Locales, locale) {
		return locale
	}
	return "en"
}

// Document reads and checks the template's design in locale (else English).
func (t Template) Document(fsys fs.FS, locale string) (design.Document, string, error) {
	locale = t.DocumentLocale(locale)
	raw, err := fs.ReadFile(fsys, path.Join(t.Slug, "design."+locale+".json"))
	if err != nil {
		return design.Document{}, locale, err
	}
	d, issues, err := design.Parse(raw)
	if err == nil && len(issues) == 0 {
		issues = design.Validate(d, design.Options{Assets: true})
	}
	if err == nil && len(issues) > 0 {
		err = fmt.Errorf("%s/design.%s.json: %v", t.Slug, locale, issues)
	}
	return d, locale, err
}

// Asset is one of the template's files.
func Asset(fsys fs.FS, slug, name string) ([]byte, error) {
	if !slugRe.MatchString(slug) || path.Base(name) != name || name == "" || name[0] == '.' {
		return nil, fs.ErrNotExist
	}
	return fs.ReadFile(fsys, path.Join(slug, "assets", name))
}

// AssetSize reads an asset's pixel size.
func AssetSize(b []byte) (int, int, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	return cfg.Width, cfg.Height, err
}

// Sample is the made-up recipient and sender of previews.
var Sample = struct {
	Values       design.Values
	Sender       design.Sender
	SubscribedAt time.Time
}{
	Values: design.Values{ContactName: "Ada Lovelace", ContactEmail: "ada@example.com", InboxName: "Northwind", ListName: "Northwind News",
		UnsubscribeURL: "https://example.com/unsubscribe", PreferencesURL: "https://example.com/preferences"},
	Sender:       design.Sender{LegalName: "Northwind Ltd", PostalAddress: "12 Harbour Street, Springfield", Contact: "hello@example.com"},
	SubscribedAt: time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC),
}

// Render draws a template in locale for a preview: assets at assetURL(name), icons at iconURL.
func Render(fsys fs.FS, t Template, locale string, assetURL func(name string) string, iconURL func(network, variant string) string) (design.Output, error) {
	d, used, err := t.Document(fsys, locale)
	if err != nil {
		return design.Output{}, err
	}
	sizes := map[string][2]int{}
	in := design.Input{
		Doc: d, Subject: Text(t.Name, used), Lang: used, Sender: Sample.Sender, SubscribedAt: Sample.SubscribedAt,
		Values: Sample.Values, Icon: iconURL,
		Image: func(b design.Block) (design.Image, bool) {
			if b.Asset == "" {
				return design.Image{}, false
			}
			wh, ok := sizes[b.Asset]
			if !ok {
				raw, err := Asset(fsys, t.Slug, b.Asset)
				if err != nil {
					return design.Image{}, false
				}
				w, h, err := AssetSize(raw)
				if err != nil {
					return design.Image{}, false
				}
				wh = [2]int{w, h}
				sizes[b.Asset] = wh
			}
			return design.Image{URL: assetURL(b.Asset), Width: wh[0], Height: wh[1]}, true
		},
	}
	return design.Render(in)
}

// PreviewAssetURL and PreviewIconURL are the links preview.<locale>.html files use: relative to
// the template folder, and the server's built-in icons.
func PreviewAssetURL(name string) string { return "assets/" + name }

func PreviewIconURL(network, variant string) string {
	return "/i/s/" + network + "-" + variant + ".png"
}

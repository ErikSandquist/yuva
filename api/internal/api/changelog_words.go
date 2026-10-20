package api

import (
	"reflect"
	"strings"
)

type changelogWords struct {
	Title, New, Improved, Fixed, All, Older, Empty, Gone, Skip   string
	Back, Feed, PublishedOn, Filter, ReadMore, Labels, Permalink string
}

var changelogLanguages = map[string]changelogWords{
	"en": {
		Title: "What's new", New: "New", Improved: "Improved", Fixed: "Fixed", All: "All", Older: "Older entries", Empty: "Nothing here yet.",
		Gone: "There is nothing here.", Skip: "Skip to content", Back: "Back to What's new", Feed: "Atom feed", PublishedOn: "Published on",
		Filter: "Filter by label", ReadMore: "Open entry", Labels: "Labels", Permalink: "Link to this entry",
	},
	"tr": {
		Title: "Yenilikler", New: "Yeni", Improved: "İyileştirildi", Fixed: "Düzeltildi", All: "Tümü", Older: "Eski girdiler", Empty: "Henüz bir şey yok.",
		Gone: "Burada bir şey yok.", Skip: "İçeriğe geç", Back: "Yeniliklere dön", Feed: "Atom beslemesi", PublishedOn: "Yayın tarihi",
		Filter: "Etikete göre süz", ReadMore: "Girdiyi aç", Labels: "Etiketler", Permalink: "Bu girdinin bağlantısı",
	},
	"de": {
		Title: "Neuigkeiten", New: "Neu", Improved: "Verbessert", Fixed: "Behoben", All: "Alle", Older: "Ältere Einträge", Empty: "Noch nichts hier.",
		Gone: "Hier gibt es nichts.", Skip: "Zum Inhalt springen", Back: "Zurück zu den Neuigkeiten", Feed: "Atom-Feed", PublishedOn: "Veröffentlicht am",
		Filter: "Nach Label filtern", ReadMore: "Eintrag öffnen", Labels: "Labels", Permalink: "Link zu diesem Eintrag",
	},
}

// A missing word falls back to English, so a language is added by filling the words it has.
func changelogWordsFor(locale string) changelogWords {
	tag := strings.ToLower(strings.ReplaceAll(locale, "_", "-"))
	base, _, _ := strings.Cut(tag, "-")
	w := changelogLanguages["en"]
	l, ok := changelogLanguages[base]
	if !ok {
		return w
	}
	dst, src := reflect.ValueOf(&w).Elem(), reflect.ValueOf(l)
	for i := range src.NumField() {
		if v := src.Field(i).String(); v != "" {
			dst.Field(i).SetString(v)
		}
	}
	return w
}

func (w changelogWords) label(l string) string {
	switch l {
	case "new":
		return w.New
	case "improved":
		return w.Improved
	case "fixed":
		return w.Fixed
	}
	return l
}

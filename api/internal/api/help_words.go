package api

import (
	"reflect"
	"strconv"
	"strings"
)

type helpWords struct {
	Help, SearchLabel, SearchPlaceholder, SearchGo, Collections    string
	ArticleOne, ArticleMany, Popular, Other, Related, Contents     string
	WasHelpful, Yes, No, Thanks, Change                            string
	StillNeed, StillNeedText, Results, NoResults, Partial, Back    string
	SearchHint, Gone, Home, Skip, ArticlesIn, UpdatedOn, SubTopics string
}

var helpLanguages = map[string]helpWords{
	"en": {
		Help: "Help center", SearchLabel: "Search the help center", SearchPlaceholder: "Search for answers", SearchGo: "Search", Collections: "Collections",
		ArticleOne: "1 article", ArticleMany: "%d articles", Popular: "Popular articles", Other: "More articles", Related: "Related articles",
		Contents: "On this page", WasHelpful: "Was this article helpful?", Yes: "Yes", No: "No", Thanks: "Thanks for your feedback.",
		Change: "Change your answer", StillNeed: "Still need help?", StillNeedText: "Write to us and we will get back to you.",
		Results: "Results for “%s”", NoResults: "No article matches your search.", Partial: "No article has every word. These have some of them.",
		Back: "Help center", SearchHint: "Type a word or two to search.", Gone: "There is no help center here.", Home: "Home",
		Skip: "Skip to content", ArticlesIn: "Articles", UpdatedOn: "Updated", SubTopics: "In this collection",
	},
	"tr": {
		Help: "Yardım merkezi", SearchLabel: "Yardım merkezinde ara", SearchPlaceholder: "Yanıt arayın", SearchGo: "Ara", Collections: "Koleksiyonlar",
		ArticleOne: "1 makale", ArticleMany: "%d makale", Popular: "Popüler makaleler", Other: "Diğer makaleler", Related: "İlgili makaleler",
		Contents: "Bu sayfada", WasHelpful: "Bu makale yardımcı oldu mu?", Yes: "Evet", No: "Hayır", Thanks: "Geri bildiriminiz için teşekkürler.",
		Change: "Yanıtınızı değiştirin", StillNeed: "Hâlâ yardıma mı ihtiyacınız var?", StillNeedText: "Bize yazın, size dönelim.",
		Results: "“%s” için sonuçlar", NoResults: "Aramanızla eşleşen makale yok.", Partial: "Hiçbir makalede tüm kelimeler yok. Bunlarda bir kısmı var.",
		Back: "Yardım merkezi", SearchHint: "Aramak için bir iki kelime yazın.", Gone: "Burada bir yardım merkezi yok.", Home: "Ana sayfa",
		Skip: "İçeriğe geç", ArticlesIn: "Makaleler", UpdatedOn: "Güncellendi", SubTopics: "Bu koleksiyonda",
	},
	"de": {
		Help: "Hilfecenter", SearchLabel: "Im Hilfecenter suchen", SearchPlaceholder: "Nach Antworten suchen", SearchGo: "Suchen", Collections: "Sammlungen",
		ArticleOne: "1 Artikel", ArticleMany: "%d Artikel", Popular: "Beliebte Artikel", Other: "Weitere Artikel", Related: "Verwandte Artikel",
		Contents: "Auf dieser Seite", WasHelpful: "War dieser Artikel hilfreich?", Yes: "Ja", No: "Nein", Thanks: "Danke für Ihre Rückmeldung.",
		Change: "Antwort ändern", StillNeed: "Brauchen Sie noch Hilfe?", StillNeedText: "Schreiben Sie uns, wir melden uns.",
		Results: "Ergebnisse für „%s“", NoResults: "Kein Artikel passt zu Ihrer Suche.", Partial: "Kein Artikel enthält alle Wörter. Diese enthalten einige davon.",
		Back: "Hilfecenter", SearchHint: "Geben Sie ein oder zwei Wörter ein.", Gone: "Hier gibt es kein Hilfecenter.", Home: "Startseite",
		Skip: "Zum Inhalt springen", ArticlesIn: "Artikel", UpdatedOn: "Aktualisiert", SubTopics: "In dieser Sammlung",
	},
}

// A missing word falls back to English, so a language is added by filling the words it has.
func helpWordsFor(locale string) helpWords {
	tag := strings.ToLower(strings.ReplaceAll(locale, "_", "-"))
	base, _, _ := strings.Cut(tag, "-")
	w := helpLanguages["en"]
	l, ok := helpLanguages[base]
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

func (w helpWords) articles(n int32) string {
	if n == 1 {
		return w.ArticleOne
	}
	return strings.Replace(w.ArticleMany, "%d", strconv.Itoa(int(n)), 1)
}

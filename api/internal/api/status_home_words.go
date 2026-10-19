package api

import (
	"reflect"
	"strings"
)

type homeLocale struct {
	Code, Name, OG string
	RTL            bool
}

// homeLocales are the languages of the panel and the widget, in the order of the language select. A
// locale gets its page at /{lowercase code}/ whether or not homeCatalog has words for it; missing
// words are English.
var homeLocales = []homeLocale{
	{"en", "English", "en_US", false}, {"tr", "Türkçe", "tr_TR", false}, {"de", "Deutsch", "de_DE", false},
	{"ar", "العربية", "ar_AR", true}, {"bn", "বাংলা", "bn_IN", false}, {"ca", "Català", "ca_ES", false},
	{"cs", "Čeština", "cs_CZ", false}, {"da", "Dansk", "da_DK", false}, {"el", "Ελληνικά", "el_GR", false},
	{"es", "Español", "es_ES", false}, {"es-MX", "Español (México)", "es_MX", false}, {"fi", "Suomi", "fi_FI", false},
	{"fr", "Français", "fr_FR", false}, {"fr-CA", "Français (Canada)", "fr_CA", false}, {"gu", "ગુજરાતી", "gu_IN", false},
	{"he", "עברית", "he_IL", true}, {"hi", "हिन्दी", "hi_IN", false}, {"hr", "Hrvatski", "hr_HR", false},
	{"hu", "Magyar", "hu_HU", false}, {"id", "Indonesia", "id_ID", false}, {"it", "Italiano", "it_IT", false},
	{"ja", "日本語", "ja_JP", false}, {"kn", "ಕನ್ನಡ", "kn_IN", false}, {"ko", "한국어", "ko_KR", false},
	{"ml", "മലയാളം", "ml_IN", false}, {"mr", "मराठी", "mr_IN", false}, {"ms", "Melayu", "ms_MY", false},
	{"nb", "Norsk bokmål", "nb_NO", false}, {"nl", "Nederlands", "nl_NL", false}, {"or", "ଓଡ଼ିଆ", "or_IN", false},
	{"pa", "ਪੰਜਾਬੀ", "pa_IN", false}, {"pl", "Polski", "pl_PL", false}, {"pt-BR", "Português (Brasil)", "pt_BR", false},
	{"pt-PT", "Português (Portugal)", "pt_PT", false}, {"ro", "Română", "ro_RO", false}, {"ru", "Русский", "ru_RU", false},
	{"sk", "Slovenčina", "sk_SK", false}, {"sl", "Slovenščina", "sl_SI", false}, {"sv", "Svenska", "sv_SE", false},
	{"ta", "தமிழ்", "ta_IN", false}, {"te", "తెలుగు", "te_IN", false}, {"th", "ไทย", "th_TH", false},
	{"uk", "Українська", "uk_UA", false}, {"ur", "اردو", "ur_PK", true}, {"vi", "Tiếng Việt", "vi_VN", false},
	{"zh-Hans", "简体中文", "zh_CN", false}, {"zh-Hant", "繁體中文", "zh_TW", false},
}

func (l homeLocale) path() string { return "/" + strings.ToLower(l.Code) + "/" }

func (l homeLocale) dir() string {
	if l.RTL {
		return "rtl"
	}
	return "ltr"
}

// homeLocaleFor finds a locale by code, case-insensitively and with "_" for "-", exactly or by its
// base language.
func homeLocaleFor(code string) (homeLocale, bool) {
	tag := strings.ToLower(strings.ReplaceAll(code, "_", "-"))
	for _, l := range homeLocales {
		if strings.ToLower(l.Code) == tag {
			return l, true
		}
	}
	base, _, _ := strings.Cut(tag, "-")
	for _, l := range homeLocales {
		if strings.ToLower(l.Code) == base {
			return l, true
		}
	}
	return homeLocale{}, false
}

type homeFact struct{ N, Label string }

type homeText struct{ Title, Text string }

type homeQA struct{ Q, A string }

// homeWords is everything the home page says. A translation fills the same fields as English; an empty
// string or an empty list falls back to English (see homeWordsFor).
type homeWords struct {
	Title, Description, SiteName, Status       string
	NavLabel, NavFeatures, NavEmbed, NavFAQ    string
	NavDocs, SignIn, Create                    string
	Kicker, H1, Lead, CtaCreate, CtaLive, Note string
	CardLive, CardDaysAgo, CardToday           string
	Facts                                      []homeFact
	FeaturesHead                               string
	Features                                   []homeText
	BreaksHead, SampleData                     string
	Steps                                      []homeText
	ResponseTitle, ResponsePeak, ResponseAlt   string
	Opened, Resolved                           string
	Reach                                      []string
	BandHead, BandH2                           string
	BandNavA, BandNavB, BandNavC               string
	BandOperational, BandSub, BandPowered      string
	BandAlt, SnippetLabel                      string
	ConvHead, ConvH2, ConvText                 string
	Convs                                      []string
	ViaChat, ViaMail, ViaApp                   string
	ConvMore, ConvMoreText, ConvReply          string
	FAQHead                                    string
	FAQ                                        []homeQA
	CtaH2, CtaGuide                            string
	FooterTag, LinkSite, LinkPages, LinkGitHub string
	Language, SkipToContent                    string
}

var homeWordsEN = homeWords{
	Title:       "Status pages by Yuva: tell customers before they ask",
	Description: "A status page that checks your services, drafts the incident and tells everyone at once: on the page, by e-mail, in your chat widget and your apps.",
	SiteName:    "Yuva Status", Status: "Status",
	NavLabel: "Main", NavFeatures: "Features", NavEmbed: "Embed", NavFAQ: "FAQ", NavDocs: "Docs", SignIn: "Sign in", Create: "Create a status page",
	Kicker: "Status pages · open source", H1: "Tell your customers before they ask.",
	Lead:      "A status page that checks your services, drafts the incident and tells everyone at once: on the page, by e-mail, inside your chat widget and your apps.",
	CtaCreate: "Create your page", CtaLive: "See a live page", Note: "Hosted on our status domain or your own. Self-host it under AGPL.",
	CardLive: "Live", CardDaysAgo: "60 days ago", CardToday: "today",
	Facts: []homeFact{{"1 page", "for every product you run"}, {"6 kinds", "of checks: HTTP, TCP, heartbeat, push, Alertmanager, Grafana"},
		{"1 reply", "answers everyone who wrote"}, {"AGPL", "open source, host it yourself if you like"}},
	FeaturesHead: "Everything a status page needs",
	Features: []homeText{
		{"Components and history", "Group what you run, show 90 days in one square per day, uptime per component and every past incident."},
		{"Checks that open incidents", "HTTP and TCP checks, heartbeats for jobs, states pushed from Alertmanager or Grafana. A failure drafts the incident for you."},
		{"Updates people can follow", "Investigating, identified, monitoring, resolved. Planned maintenance with its window. Atom feed, badge and JSON."},
		{"E-mail subscribers", "Double opt-in, per-component choice, one calm e-mail per update, and a minute to undo before it goes."},
		{"In your widget and apps", "The chat widget and the iOS and Android SDKs show the status line before a customer starts to write."},
		{"Your own domain", "Point status.yourcompany.com at Yuva with one CNAME. The certificate is issued and renewed for you."},
	},
	BreaksHead: "When something breaks", SampleData: "Sample data",
	Steps: []homeText{
		{"A check fails", "API response times cross the line you set."},
		{"An incident is drafted", "Affected component, impact and a first update, ready to publish."},
		{"Everyone hears it", "Page, subscribers, the widget, your apps."},
		{"Resolved", "The page goes green and one reply closes every linked conversation."},
	},
	ResponseTitle: "API response time", ResponsePeak: "peak 1.4 s",
	ResponseAlt: "Response time rises from 290 ms to 1.4 s, the incident opens, then falls back",
	Opened:      "opened", Resolved: "resolved",
	Reach:    []string{"saw it on the page", "subscribers e-mailed", "chats answered at once"},
	BandHead: "On your own site", BandH2: "One line at the bottom of your footer.",
	BandNavA: "Pricing", BandNavB: "Docs", BandNavC: "Privacy",
	BandOperational: "All systems operational", BandSub: "Everything Northwind runs on is working.", BandPowered: "Status by Yuva",
	BandAlt: "The status line in a site footer, with 90 days of uptime", SnippetLabel: "Embed code",
	ConvHead: "Part of your support inbox", ConvH2: "Answer everyone who wrote, once.",
	ConvText: "Conversations about an incident link to it. When it is fixed, one reply reaches every customer who asked, by e-mail, chat or in your app.",
	Convs:    []string{"Is the API down? Our sync is failing.", "Requests time out since 8 pm.", "Dashboard feels very slow today."},
	ViaChat:  "Chat", ViaMail: "E-mail", ViaApp: "iOS",
	ConvMore: "+ 11 more", ConvMoreText: "linked to “Elevated API response times”", ConvReply: "Reply to 14 linked conversations",
	FAQHead: "Questions",
	FAQ: []homeQA{
		{"Is it open source?", "Yes. A status page is part of every Yuva workspace, and Yuva is open source under AGPL, so you can also run it on your own server."},
		{"Can I use my own domain?", "Yes. Add a CNAME for status.yourcompany.com; Yuva checks it and issues the certificate."},
		{"What can it check?", "HTTP and TCP endpoints from outside, heartbeats from your jobs, and states pushed by Alertmanager, Grafana or your own scripts."},
		{"Do I need Yuva for support to use it?", "No, the status page works on its own. With the inbox you also get linked conversations and the status line in your widget and apps."},
		{"Which languages does the page speak?", "The page follows the visitor’s language, and visitors can pick one themselves."},
	},
	CtaH2: "Your status page, live in minutes.", CtaGuide: "Read the guide",
	FooterTag: "Yuva · every customer conversation in one place you own", LinkSite: "useyuva.com", LinkPages: "Status pages", LinkGitHub: "GitHub",
	Language: "Language", SkipToContent: "Skip to content",
}

var homeCatalog = map[string]homeWords{
	"en": homeWordsEN,
	"tr": {
		Status:      "Durum",
		Title:       "Yuva durum sayfaları: müşteriniz sormadan haber verin",
		Description: "Servislerinizi denetler, olayın taslağını hazırlar ve herkese aynı anda haber verir: sayfada, e-postayla, sohbet widget'ında ve uygulamalarınızda.",
		NavLabel:    "Ana", NavFeatures: "Özellikler", NavEmbed: "Yerleştir", NavFAQ: "SSS", NavDocs: "Belgeler", SignIn: "Giriş yap", Create: "Durum sayfası oluştur",
		Kicker: "Durum sayfaları · açık kaynak", H1: "Müşterileriniz sormadan önce haber verin.",
		Lead:      "Servislerinizi denetleyen, olayın taslağını sizin için hazırlayan ve herkese aynı anda haber veren bir durum sayfası: sayfada, e-postayla, sohbet widget'ınızda ve uygulamalarınızda.",
		CtaCreate: "Sayfanızı oluşturun", CtaLive: "Canlı bir sayfa görün", Note: "Durum alan adımızda ya da kendi alan adınızda. AGPL ile kendi sunucunuzda da çalıştırabilirsiniz.",
		CardLive: "Canlı", CardDaysAgo: "60 gün önce", CardToday: "bugün",
		Facts: []homeFact{{"1 sayfa", "işlettiğiniz tüm ürünler için"}, {"6 tür", "kontrol: HTTP, TCP, heartbeat, push, Alertmanager, Grafana"},
			{"1 yanıt", "yazan herkesi yanıtlar"}, {"AGPL", "açık kaynak, isterseniz kendiniz barındırın"}},
		FeaturesHead: "Bir durum sayfasının ihtiyacı olan her şey",
		Features: []homeText{
			{"Bileşenler ve geçmiş", "İşlettiklerinizi gruplayın; 90 günü günlük birer kare olarak, bileşen başına erişilebilirliği ve geçmiş tüm olayları gösterin."},
			{"Olay açan kontroller", "HTTP ve TCP kontrolleri, zamanlanmış işler için heartbeat, Alertmanager ya da Grafana'dan gelen durumlar. Bir arıza olayın taslağını sizin için hazırlar."},
			{"Takip edilebilir güncellemeler", "İnceleniyor, sorun bulundu, izleniyor, çözüldü. Zaman aralığıyla planlı bakım. Atom akışı, rozet ve JSON."},
			{"E-posta aboneleri", "Çift onaylı abonelik, bileşen bazında seçim, her güncelleme için tek ve sakin bir e-posta; gönderilmeden önce geri almak için bir dakika."},
			{"Widget'ınızda ve uygulamalarınızda", "Sohbet widget'ı ile iOS ve Android SDK'ları, müşteri yazmaya başlamadan önce durum satırını gösterir."},
			{"Kendi alan adınız", "status.sirketiniz.com adresini tek bir CNAME ile Yuva'ya yönlendirin. Sertifikayı sizin için alır ve yeniler."},
		},
		BreaksHead: "Bir şey bozulduğunda", SampleData: "Örnek veri",
		Steps: []homeText{
			{"Bir kontrol başarısız olur", "API yanıt süreleri belirlediğiniz sınırı aşar."},
			{"Olay taslağı hazırlanır", "Etkilenen bileşen, etki ve ilk güncelleme; yayına hazır."},
			{"Herkes haberdar olur", "Sayfa, aboneler, widget, uygulamalarınız."},
			{"Çözüldü", "Sayfa yeşile döner ve tek bir yanıt bağlı tüm konuşmaları kapatır."},
		},
		ResponseTitle: "API yanıt süresi", ResponsePeak: "en yüksek 1,4 sn",
		ResponseAlt: "Yanıt süresi 290 ms'den 1,4 sn'ye çıkar, olay açılır, sonra eski düzeyine iner",
		Opened:      "açıldı", Resolved: "çözüldü",
		Reach:    []string{"sayfada gördü", "aboneye e-posta gitti", "sohbet tek seferde yanıtlandı"},
		BandHead: "Kendi sitenizde", BandH2: "Alt bilgi alanınızın en altında tek bir satır.",
		BandNavA: "Fiyatlar", BandNavB: "Belgeler", BandNavC: "Gizlilik",
		BandOperational: "Tüm sistemler çalışıyor", BandSub: "Northwind'in dayandığı her şey çalışıyor.", BandPowered: "Durum: Yuva",
		BandAlt: "Sitenin alt bilgisinde durum satırı ve 90 günlük erişilebilirlik", SnippetLabel: "Yerleştirme kodu",
		ConvHead: "Destek gelen kutunuzun parçası", ConvH2: "Yazan herkesi tek seferde yanıtlayın.",
		ConvText: "Bir olayla ilgili konuşmalar olaya bağlanır. Sorun giderilince tek bir yanıt, soran her müşteriye e-posta, sohbet ya da uygulamanızın içinden ulaşır.",
		Convs:    []string{"API çöktü mü? Eşitlememiz hata veriyor.", "İstekler akşam 8'den beri zaman aşımına uğruyor.", "Panel bugün çok yavaş."},
		ViaChat:  "Sohbet", ViaMail: "E-posta", ViaApp: "iOS",
		ConvMore: "+ 11 tane daha", ConvMoreText: "“API yanıt süreleri yükseldi” olayına bağlı", ConvReply: "Bağlı 14 konuşmayı yanıtla",
		FAQHead: "Sorular",
		FAQ: []homeQA{
			{"Açık kaynak mı?", "Evet. Durum sayfası her Yuva çalışma alanının parçasıdır ve Yuva AGPL ile açık kaynaktır; isterseniz kendi sunucunuzda da çalıştırabilirsiniz."},
			{"Kendi alan adımı kullanabilir miyim?", "Evet. status.sirketiniz.com için bir CNAME ekleyin; Yuva kaydı denetler ve sertifikayı verir."},
			{"Neleri kontrol edebilir?", "Dışarıdan HTTP ve TCP uç noktalarını, işlerinizden gelen heartbeat'leri ve Alertmanager, Grafana ya da kendi betiklerinizin ilettiği durumları."},
			{"Kullanmak için destek amaçlı Yuva'ya ihtiyacım var mı?", "Hayır, durum sayfası tek başına çalışır. Gelen kutusuyla birlikte bağlı konuşmaları ve widget'ınızda, uygulamalarınızda durum satırını da alırsınız."},
			{"Sayfa hangi dilleri konuşuyor?", "Sayfa ziyaretçinin diline uyar; ziyaretçiler isterse kendileri de bir dil seçebilir."},
		},
		CtaH2: "Durum sayfanız dakikalar içinde yayında.", CtaGuide: "Kılavuzu okuyun",
		FooterTag: "Yuva · tüm müşteri konuşmaları, sahibi olduğunuz tek bir yerde", LinkSite: "useyuva.com", LinkPages: "Durum sayfaları", LinkGitHub: "GitHub",
		Language: "Dil", SkipToContent: "İçeriğe geç",
	},
	"de": {
		Title:       "Statusseiten von Yuva: Informieren, bevor Kunden fragen",
		Description: "Eine Statusseite, die Ihre Dienste prüft, die Störung vorbereitet und alle auf einmal informiert: auf der Seite, per E-Mail, im Chat-Widget und in Ihren Apps.",
		NavLabel:    "Hauptmenü", NavFeatures: "Funktionen", NavEmbed: "Einbetten", NavFAQ: "FAQ", NavDocs: "Doku", SignIn: "Anmelden", Create: "Statusseite erstellen",
		Kicker: "Statusseiten · Open Source", H1: "Sagen Sie es Ihren Kunden, bevor sie fragen.",
		Lead:      "Eine Statusseite, die Ihre Dienste prüft, die Störung für Sie vorbereitet und alle auf einmal informiert: auf der Seite, per E-Mail, im Chat-Widget und in Ihren Apps.",
		CtaCreate: "Seite erstellen", CtaLive: "Eine Live-Seite ansehen", Note: "Gehostet auf unserer Status-Domain oder Ihrer eigenen. Unter AGPL auch selbst betreibbar.",
		CardLive: "Live", CardDaysAgo: "vor 60 Tagen", CardToday: "heute",
		Facts: []homeFact{{"1 Seite", "für jedes Produkt, das Sie betreiben"}, {"6 Arten", "von Prüfungen: HTTP, TCP, Heartbeat, Push, Alertmanager, Grafana"},
			{"1 Antwort", "erreicht alle, die geschrieben haben"}, {"AGPL", "Open Source, auf Wunsch selbst gehostet"}},
		FeaturesHead: "Alles, was eine Statusseite braucht",
		Features: []homeText{
			{"Komponenten und Verlauf", "Gruppieren Sie, was Sie betreiben, und zeigen Sie 90 Tage als ein Kästchen pro Tag, die Verfügbarkeit je Komponente und jede frühere Störung."},
			{"Prüfungen, die Störungen eröffnen", "HTTP- und TCP-Prüfungen, Heartbeats für Jobs, Zustände aus Alertmanager oder Grafana. Ein Ausfall legt die Störungsmeldung als Entwurf an."},
			{"Meldungen zum Mitverfolgen", "Wird untersucht, Ursache erkannt, wird beobachtet, behoben. Geplante Wartung mit Zeitfenster. Atom-Feed, Badge und JSON."},
			{"E-Mail-Abonnenten", "Double-Opt-in, Auswahl je Komponente, eine ruhige E-Mail pro Meldung und eine Minute Zeit zum Zurückziehen, bevor sie rausgeht."},
			{"In Widget und Apps", "Das Chat-Widget und die iOS- und Android-SDKs zeigen die Statuszeile, noch bevor ein Kunde zu schreiben beginnt."},
			{"Ihre eigene Domain", "Richten Sie status.ihrefirma.de mit einem einzigen CNAME auf Yuva. Das Zertifikat wird für Sie ausgestellt und erneuert."},
		},
		BreaksHead: "Wenn etwas ausfällt", SampleData: "Beispieldaten",
		Steps: []homeText{
			{"Eine Prüfung schlägt fehl", "Die Antwortzeiten der API überschreiten Ihren Grenzwert."},
			{"Eine Störung wird vorbereitet", "Betroffene Komponente, Auswirkung und eine erste Meldung, bereit zur Veröffentlichung."},
			{"Alle erfahren es", "Seite, Abonnenten, Widget, Ihre Apps."},
			{"Behoben", "Die Seite wird grün, und eine einzige Antwort schließt alle verknüpften Gespräche."},
		},
		ResponseTitle: "Antwortzeit der API", ResponsePeak: "Spitze 1,4 s",
		ResponseAlt: "Die Antwortzeit steigt von 290 ms auf 1,4 s, die Störung wird eröffnet, danach sinkt sie wieder",
		Opened:      "eröffnet", Resolved: "behoben",
		Reach:    []string{"sahen es auf der Seite", "Abonnenten per E-Mail informiert", "Chats auf einmal beantwortet"},
		BandHead: "Auf Ihrer eigenen Website", BandH2: "Eine Zeile ganz unten in Ihrer Fußzeile.",
		BandNavA: "Preise", BandNavB: "Doku", BandNavC: "Datenschutz",
		BandOperational: "Alle Systeme betriebsbereit", BandSub: "Alles, worauf Northwind läuft, funktioniert.", BandPowered: "Status von Yuva",
		BandAlt: "Die Statuszeile in der Fußzeile einer Website, mit 90 Tagen Verfügbarkeit", SnippetLabel: "Einbettungscode",
		ConvHead: "Teil Ihres Support-Postfachs", ConvH2: "Alle beantworten, die geschrieben haben, auf einmal.",
		ConvText: "Gespräche zu einer Störung werden mit ihr verknüpft. Ist sie behoben, erreicht eine einzige Antwort jeden Kunden, der gefragt hat, per E-Mail, Chat oder in Ihrer App.",
		Convs:    []string{"Ist die API down? Unsere Synchronisation schlägt fehl.", "Anfragen laufen seit 20 Uhr in einen Timeout.", "Das Dashboard ist heute sehr langsam."},
		ViaChat:  "Chat", ViaMail: "E-Mail", ViaApp: "iOS",
		ConvMore: "+ 11 weitere", ConvMoreText: "verknüpft mit „Erhöhte Antwortzeiten der API“", ConvReply: "14 verknüpfte Gespräche beantworten",
		FAQHead: "Fragen",
		FAQ: []homeQA{
			{"Ist es Open Source?", "Ja. Eine Statusseite gehört zu jedem Yuva-Workspace, und Yuva ist Open Source unter AGPL, Sie können es also auch auf Ihrem eigenen Server betreiben."},
			{"Kann ich meine eigene Domain verwenden?", "Ja. Legen Sie einen CNAME für status.ihrefirma.de an; Yuva prüft ihn und stellt das Zertifikat aus."},
			{"Was kann geprüft werden?", "HTTP- und TCP-Endpunkte von außen, Heartbeats Ihrer Jobs und Zustände, die Alertmanager, Grafana oder Ihre eigenen Skripte melden."},
			{"Brauche ich Yuva für den Support, um es zu nutzen?", "Nein, die Statusseite funktioniert allein. Mit dem Postfach erhalten Sie zusätzlich verknüpfte Gespräche und die Statuszeile in Widget und Apps."},
			{"Welche Sprachen spricht die Seite?", "Die Seite richtet sich nach der Sprache des Besuchers, und Besucher können selbst eine wählen."},
		},
		CtaH2: "Ihre Statusseite, in Minuten live.", CtaGuide: "Anleitung lesen",
		FooterTag: "Yuva · jedes Kundengespräch an einem Ort, der Ihnen gehört", LinkSite: "useyuva.com", LinkPages: "Statusseiten", LinkGitHub: "GitHub",
		Language: "Sprache", SkipToContent: "Zum Inhalt springen",
	},
}

// homeWordsFor returns a locale's words with every empty field and empty list taken from English.
func homeWordsFor(code string) homeWords {
	w := homeCatalog[code]
	fillEnglish(reflect.ValueOf(&w).Elem(), reflect.ValueOf(homeWordsEN))
	return w
}

func fillEnglish(dst, en reflect.Value) {
	for i := range dst.NumField() {
		f := dst.Field(i)
		switch f.Kind() {
		case reflect.String:
			if f.String() == "" {
				f.Set(en.Field(i))
			}
		case reflect.Slice:
			if f.Len() == 0 || f.Len() != en.Field(i).Len() {
				f.Set(en.Field(i))
			}
		}
	}
}

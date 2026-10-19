package api

import (
	"strconv"
	"strings"
	"time"
)

type statusWords struct {
	Lang                                         string
	States, Headlines, IncidentStates, Impacts   map[string]string
	Active, Maintenance, Components, Uptime      string
	DaysAgo, Today, NoData, Recent, NoRecent     string
	History, NoHistory, Back, Feed, Updated      string
	Subscribe                                    string
	Edited, Support, Site, Window, Affects, Gone string
	MonitorTitle, MonitorBody, MonitorResolved   string
	MonitorPaused, MonitorRemoved                string
	MaintenanceStarted, MaintenanceDone          string
	WindowNow                                    string
	Months                                       [12]string

	Name, Dir, Status, Language, Apply, Follow               string
	UptimeDays, AffectedNow, AffectedNone, AffectedOne       string
	AffectedMany, JustNow, MinutesAgo, HoursAgo              string
	IncidentKind, Since, Until, ImpactOf                     string
	ResponseTime, ResponseDuring, Median, P95, Peak, Usually string
	Monitored, Now, HoursBack, StartMark, EndMark, NoChecks  string
	FullHistory, SubscribeHead, SubscribeText, SubscribeGo   string
	FollowEmbed, FeedText, Badge, BadgeText, JSONText        string
	Embed, EmbedText, TimesIn, Powered                       string
	Started, Ended, Ongoing, Duration, Updates, Affected     string
	StayInformed, StayText, HistoryHead, Filter, FilterAll   string
	FilterIncidents, NoMonth, DurMin, DurHours, DurDays      string
	AffectedThen, AffectedSoFar                              string
	LongMonths                                               [12]string
}

var statusLanguages = map[string]statusWords{
	"en": {
		Lang: "en",
		States: map[string]string{stOperational: "Operational", stDegraded: "Degraded performance", stPartialOutage: "Partial outage",
			stMajorOutage: "Major outage", stMaintenance: "Under maintenance"},
		Headlines: map[string]string{stOperational: "All systems operational", stDegraded: "Some systems are slow", stPartialOutage: "Some systems are down",
			stMajorOutage: "Major outage", stMaintenance: "Maintenance in progress"},
		IncidentStates: map[string]string{incInvestigating: "Investigating", incIdentified: "Identified", incMonitoring: "Monitoring",
			incResolved: "Resolved", incScheduled: "Scheduled", incInProgress: "In progress", incCompleted: "Completed"},
		Impacts: map[string]string{"none": "No impact", "minor": "Minor", "major": "Major", "critical": "Critical"},
		Active:  "Current incidents", Maintenance: "Maintenance", Components: "Components", Uptime: "%s uptime",
		DaysAgo: "%d days ago", Today: "Today", NoData: "No data", Recent: "Past incidents", NoRecent: "No incidents in the last 14 days.",
		History: "Incident history", NoHistory: "No incidents in the last %d days.", Back: "Current status", Feed: "Atom feed", Subscribe: "Get notices by e-mail",
		Updated: "Last updated", Edited: "edited", Support: "Get help", Site: "Website", Window: "Scheduled", Affects: "Affects",
		Gone:         "There is no status page here.",
		MonitorTitle: "%s is not working as expected", MonitorBody: "Our monitoring found a problem with %s. We are looking into it.",
		MonitorResolved: "%s is working again.",
		MonitorPaused:   "Monitoring of %s was paused.", MonitorRemoved: "Monitoring of %s was removed.",
		MaintenanceStarted: "The maintenance has started.", MaintenanceDone: "The maintenance is complete.",
		WindowNow: "The maintenance is now scheduled for %s.",
		Months:    [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
		Name:      "English", Status: "Status", Language: "Language", Apply: "Apply", Follow: "Follow",
		UptimeDays: "Uptime · %d days", AffectedNow: "Affected now", AffectedNone: "None", AffectedOne: "1 component",
		AffectedMany: "%d components", JustNow: "Just now", MinutesAgo: "%d min ago", HoursAgo: "%d h ago",
		IncidentKind: "Incident", Since: "since %s", Until: "until %s", ImpactOf: "%s impact",
		ResponseTime: "Response time · 24 h", ResponseDuring: "Response time during the incident", Median: "median", P95: "p95 %s",
		Peak: "Peak", Usually: "usually %s", Monitored: "Monitored component", Now: "now", HoursBack: "−%d h",
		StartMark: "start", EndMark: "end", NoChecks: "No checks in this time.",
		FullHistory: "Full history", SubscribeHead: "Subscribe",
		SubscribeText: "Pick the components you care about; nothing picked means every component. We send a confirmation first.",
		SubscribeGo:   "Subscribe by e-mail", FollowEmbed: "Follow and embed", FeedText: "For feed readers and chat tools.",
		Badge: "Status badge", BadgeText: "For a README or a footer.", JSONText: "For your own app. No key needed.",
		Embed: "Embed", EmbedText: "Inside your own site.", TimesIn: "Times in your time zone (%s).", Powered: "Powered by Yuva",
		Started: "Started", Ended: "Ended", Ongoing: "Ongoing", Duration: "Duration", Updates: "Updates", Affected: "Affected",
		StayInformed: "Stay informed", StayText: "Get the next one by e-mail as it happens, or follow the feed.",
		HistoryHead: "Every incident and maintenance.", Filter: "Filter", FilterAll: "All", FilterIncidents: "Incidents",
		NoMonth: "No incidents this month.", DurMin: "%d min", DurHours: "%d h", DurDays: "%d d",
		AffectedThen: "%s for %s, then %s", AffectedSoFar: "%s for %s so far",
		LongMonths: [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
	},
	"tr": {
		Lang: "tr",
		States: map[string]string{stOperational: "Çalışıyor", stDegraded: "Yavaşlama", stPartialOutage: "Kısmi kesinti",
			stMajorOutage: "Büyük kesinti", stMaintenance: "Bakımda"},
		Headlines: map[string]string{stOperational: "Tüm sistemler çalışıyor", stDegraded: "Bazı sistemler yavaş", stPartialOutage: "Bazı sistemler çalışmıyor",
			stMajorOutage: "Büyük kesinti", stMaintenance: "Bakım sürüyor"},
		IncidentStates: map[string]string{incInvestigating: "İnceleniyor", incIdentified: "Sorun bulundu", incMonitoring: "İzleniyor",
			incResolved: "Çözüldü", incScheduled: "Planlandı", incInProgress: "Sürüyor", incCompleted: "Tamamlandı"},
		Impacts: map[string]string{"none": "Etkisiz", "minor": "Küçük", "major": "Büyük", "critical": "Kritik"},
		Active:  "Süren olaylar", Maintenance: "Bakım", Components: "Bileşenler", Uptime: "%s erişilebilirlik",
		DaysAgo: "%d gün önce", Today: "Bugün", NoData: "Veri yok", Recent: "Geçmiş olaylar", NoRecent: "Son 14 günde olay yok.",
		History: "Olay geçmişi", NoHistory: "Son %d günde olay yok.", Back: "Güncel durum", Feed: "Atom akışı", Subscribe: "E-posta ile bildirim al",
		Updated: "Son güncelleme", Edited: "düzenlendi", Support: "Yardım alın", Site: "Web sitesi", Window: "Planlanan", Affects: "Etkilenen",
		Gone:         "Burada bir durum sayfası yok.",
		MonitorTitle: "%s beklendiği gibi çalışmıyor", MonitorBody: "İzleme sistemimiz %s ile ilgili bir sorun buldu. İnceliyoruz.",
		MonitorResolved: "%s yeniden çalışıyor.",
		MonitorPaused:   "%s izlemesi duraklatıldı.", MonitorRemoved: "%s izlemesi kaldırıldı.",
		MaintenanceStarted: "Bakım başladı.", MaintenanceDone: "Bakım tamamlandı.",
		WindowNow: "Bakım artık şu zamana planlandı: %s.",
		Months:    [12]string{"Oca", "Şub", "Mar", "Nis", "May", "Haz", "Tem", "Ağu", "Eyl", "Eki", "Kas", "Ara"},
		Name:      "Türkçe", Status: "Durum", Language: "Dil", Apply: "Uygula", Follow: "Takip et",
		UptimeDays: "Erişilebilirlik · %d gün", AffectedNow: "Şu an etkilenen", AffectedNone: "Yok", AffectedOne: "1 bileşen",
		AffectedMany: "%d bileşen", JustNow: "Az önce", MinutesAgo: "%d dk önce", HoursAgo: "%d sa önce",
		IncidentKind: "Olay", Since: "başlangıç %s", Until: "bitiş %s", ImpactOf: "%s etki",
		ResponseTime: "Yanıt süresi · 24 sa", ResponseDuring: "Olay sırasında yanıt süresi", Median: "medyan", P95: "p95 %s",
		Peak: "En yüksek", Usually: "genelde %s", Monitored: "İzlenen bileşen", Now: "şimdi", HoursBack: "−%d sa",
		StartMark: "başlangıç", EndMark: "bitiş", NoChecks: "Bu sürede kontrol yok.",
		FullHistory: "Tüm geçmiş", SubscribeHead: "Abone ol",
		SubscribeText: "İlgilendiğiniz bileşenleri seçin; seçim yapmazsanız tüm bileşenler için bildirim alırsınız. Önce bir onay e-postası göndeririz.",
		SubscribeGo:   "E-posta ile abone ol", FollowEmbed: "Takip et ve yerleştir", FeedText: "Akış okuyucular ve sohbet araçları için.",
		Badge: "Durum rozeti", BadgeText: "README ya da sayfa altı için.", JSONText: "Kendi uygulamanız için. Anahtar gerekmez.",
		Embed: "Yerleştir", EmbedText: "Kendi sitenizin içinde.", TimesIn: "Saatler sizin saat diliminizde (%s).", Powered: "Yuva ile çalışır",
		Started: "Başladı", Ended: "Bitti", Ongoing: "Sürüyor", Duration: "Süre", Updates: "Güncellemeler", Affected: "Etkilenen",
		StayInformed: "Haberdar olun", StayText: "Bir sonrakini olduğu anda e-postayla alın ya da akışı takip edin.",
		HistoryHead: "Tüm olaylar ve bakımlar.", Filter: "Süz", FilterAll: "Tümü", FilterIncidents: "Olaylar",
		NoMonth: "Bu ay olay yok.", DurMin: "%d dk", DurHours: "%d sa", DurDays: "%d gün",
		AffectedThen: "%s %s sürdü, sonra %s", AffectedSoFar: "%s %s sürüyor",
		LongMonths: [12]string{"Ocak", "Şubat", "Mart", "Nisan", "Mayıs", "Haziran", "Temmuz", "Ağustos", "Eylül", "Ekim", "Kasım", "Aralık"},
	},
	"de": {
		Lang: "de",
		States: map[string]string{stOperational: "Betriebsbereit", stDegraded: "Eingeschränkte Leistung", stPartialOutage: "Teilausfall",
			stMajorOutage: "Schwerer Ausfall", stMaintenance: "In Wartung"},
		Headlines: map[string]string{stOperational: "Alle Systeme betriebsbereit", stDegraded: "Einige Systeme sind langsam", stPartialOutage: "Einige Systeme sind ausgefallen",
			stMajorOutage: "Schwerer Ausfall", stMaintenance: "Wartung läuft"},
		IncidentStates: map[string]string{incInvestigating: "Wird untersucht", incIdentified: "Ursache erkannt", incMonitoring: "Wird beobachtet",
			incResolved: "Behoben", incScheduled: "Geplant", incInProgress: "Läuft", incCompleted: "Abgeschlossen"},
		Impacts: map[string]string{"none": "Keine Auswirkung", "minor": "Gering", "major": "Erheblich", "critical": "Kritisch"},
		Active:  "Aktuelle Störungen", Maintenance: "Wartung", Components: "Komponenten", Uptime: "%s Verfügbarkeit",
		DaysAgo: "vor %d Tagen", Today: "Heute", NoData: "Keine Daten", Recent: "Vergangene Störungen", NoRecent: "Keine Störungen in den letzten 14 Tagen.",
		History: "Störungsverlauf", NoHistory: "Keine Störungen in den letzten %d Tagen.", Back: "Aktueller Status", Feed: "Atom-Feed", Subscribe: "Meldungen per E-Mail",
		Updated: "Zuletzt aktualisiert", Edited: "bearbeitet", Support: "Hilfe erhalten", Site: "Website", Window: "Geplant", Affects: "Betrifft",
		Gone:         "Hier gibt es keine Statusseite.",
		MonitorTitle: "%s funktioniert nicht wie erwartet", MonitorBody: "Unsere Überwachung hat ein Problem mit %s festgestellt. Wir sehen es uns an.",
		MonitorResolved: "%s funktioniert wieder.",
		MonitorPaused:   "Die Überwachung von %s wurde pausiert.", MonitorRemoved: "Die Überwachung von %s wurde entfernt.",
		MaintenanceStarted: "Die Wartung hat begonnen.", MaintenanceDone: "Die Wartung ist abgeschlossen.",
		WindowNow: "Die Wartung ist jetzt geplant für %s.",
		Months:    [12]string{"Jan.", "Feb.", "März", "Apr.", "Mai", "Juni", "Juli", "Aug.", "Sep.", "Okt.", "Nov.", "Dez."},
		Name:      "Deutsch", Status: "Status", Language: "Sprache", Apply: "Anwenden", Follow: "Folgen",
		UptimeDays: "Verfügbarkeit · %d Tage", AffectedNow: "Jetzt betroffen", AffectedNone: "Keine", AffectedOne: "1 Komponente",
		AffectedMany: "%d Komponenten", JustNow: "Gerade eben", MinutesAgo: "vor %d Min.", HoursAgo: "vor %d Std.",
		IncidentKind: "Störung", Since: "seit %s", Until: "bis %s", ImpactOf: "Auswirkung: %s",
		ResponseTime: "Antwortzeit · 24 Std.", ResponseDuring: "Antwortzeit während der Störung", Median: "Median", P95: "p95 %s",
		Peak: "Spitze", Usually: "sonst %s", Monitored: "Überwachte Komponente", Now: "jetzt", HoursBack: "−%d Std.",
		StartMark: "Beginn", EndMark: "Ende", NoChecks: "Keine Prüfungen in dieser Zeit.",
		FullHistory: "Gesamter Verlauf", SubscribeHead: "Abonnieren",
		SubscribeText: "Wählen Sie die Komponenten, die Sie betreffen; ohne Auswahl gelten alle Komponenten. Wir senden zuerst eine Bestätigung.",
		SubscribeGo:   "Per E-Mail abonnieren", FollowEmbed: "Folgen und einbetten", FeedText: "Für Feedreader und Chat-Werkzeuge.",
		Badge: "Status-Badge", BadgeText: "Für ein README oder eine Fußzeile.", JSONText: "Für Ihre eigene App. Ohne Schlüssel.",
		Embed: "Einbetten", EmbedText: "In Ihrer eigenen Website.", TimesIn: "Zeiten in Ihrer Zeitzone (%s).", Powered: "Betrieben mit Yuva",
		Started: "Begonnen", Ended: "Beendet", Ongoing: "Läuft", Duration: "Dauer", Updates: "Meldungen", Affected: "Betroffen",
		StayInformed: "Auf dem Laufenden bleiben", StayText: "Erhalten Sie die nächste Meldung sofort per E-Mail oder folgen Sie dem Feed.",
		HistoryHead: "Alle Störungen und Wartungen.", Filter: "Filter", FilterAll: "Alle", FilterIncidents: "Störungen",
		NoMonth: "Keine Störungen in diesem Monat.", DurMin: "%d Min.", DurHours: "%d Std.", DurDays: "%d T.",
		AffectedThen: "%s für %s, danach %s", AffectedSoFar: "%s seit %s",
		LongMonths: [12]string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"},
	},
}

func statusWordsFor(locale string) statusWords {
	w, _ := statusLanguage(locale)
	return w
}

// statusLanguage finds the catalogue for a language tag, exactly or by its base language; false
// when there is none and English is used.
func statusLanguage(locale string) (statusWords, bool) {
	tag := strings.ToLower(strings.ReplaceAll(locale, "_", "-"))
	for code, w := range statusLanguages {
		if strings.ToLower(code) == tag {
			return w, true
		}
	}
	base, _, _ := strings.Cut(tag, "-")
	if w, ok := statusLanguages[base]; ok {
		return w, true
	}
	return statusLanguages["en"], false
}

func (w statusWords) direction() string {
	if w.Dir == "" {
		return "ltr"
	}
	return w.Dir
}

func (w statusWords) day(t time.Time) string {
	return strconv.Itoa(t.Day()) + " " + w.Months[t.Month()-1] + " " + strconv.Itoa(t.Year())
}

func (w statusWords) moment(t time.Time) string {
	return w.day(t) + ", " + t.Format("15:04 MST")
}

func (w statusWords) month(t time.Time) string {
	return w.Months[t.Month()-1] + " " + strconv.Itoa(t.Year())
}

func (w statusWords) longMonth(t time.Time) string {
	return w.LongMonths[t.Month()-1] + " " + strconv.Itoa(t.Year())
}

func (w statusWords) shortDay(t time.Time) string {
	return strconv.Itoa(t.Day()) + " " + w.Months[t.Month()-1]
}

func (w statusWords) duration(d time.Duration) string {
	m := int(d.Round(time.Minute) / time.Minute)
	switch {
	case m < 60:
		return fill1(w.DurMin, max(m, 1))
	case m < 48*60:
		if m%60 == 0 {
			return fill1(w.DurHours, m/60)
		}
		return fill1(w.DurHours, m/60) + " " + fill1(w.DurMin, m%60)
	}
	return fill1(w.DurDays, m/(24*60))
}

func (w statusWords) ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return w.JustNow
	case d < time.Hour:
		return fill1(w.MinutesAgo, int(d/time.Minute))
	case d < 48*time.Hour:
		return fill1(w.HoursAgo, int(d/time.Hour))
	}
	return fill1(w.DaysAgo, int(d/(24*time.Hour)))
}

func fill1(format string, n int) string { return strings.Replace(format, "%d", strconv.Itoa(n), 1) }

func fillS(format, s string) string { return strings.Replace(format, "%s", s, 1) }

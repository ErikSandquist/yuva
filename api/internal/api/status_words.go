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
	Edited, Support, Site, Window, Affects, Gone string
	MonitorTitle, MonitorBody, MonitorResolved   string
	MonitorPaused, MonitorRemoved                string
	Months                                       [12]string
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
		History: "Incident history", NoHistory: "No incidents in the last %d days.", Back: "Current status", Feed: "Atom feed",
		Updated: "Last updated", Edited: "edited", Support: "Get help", Site: "Website", Window: "Scheduled", Affects: "Affects",
		Gone:         "There is no status page here.",
		MonitorTitle: "%s is not working as expected", MonitorBody: "Our monitoring found a problem with %s. We are looking into it.",
		MonitorResolved: "%s is working again.",
		MonitorPaused:   "Monitoring of %s was paused.", MonitorRemoved: "Monitoring of %s was removed.",
		Months: [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
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
		History: "Olay geçmişi", NoHistory: "Son %d günde olay yok.", Back: "Güncel durum", Feed: "Atom akışı",
		Updated: "Son güncelleme", Edited: "düzenlendi", Support: "Yardım alın", Site: "Web sitesi", Window: "Planlanan", Affects: "Etkilenen",
		Gone:         "Burada bir durum sayfası yok.",
		MonitorTitle: "%s beklendiği gibi çalışmıyor", MonitorBody: "İzleme sistemimiz %s ile ilgili bir sorun buldu. İnceliyoruz.",
		MonitorResolved: "%s yeniden çalışıyor.",
		MonitorPaused:   "%s izlemesi duraklatıldı.", MonitorRemoved: "%s izlemesi kaldırıldı.",
		Months: [12]string{"Oca", "Şub", "Mar", "Nis", "May", "Haz", "Tem", "Ağu", "Eyl", "Eki", "Kas", "Ara"},
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
		History: "Störungsverlauf", NoHistory: "Keine Störungen in den letzten %d Tagen.", Back: "Aktueller Status", Feed: "Atom-Feed",
		Updated: "Zuletzt aktualisiert", Edited: "bearbeitet", Support: "Hilfe erhalten", Site: "Website", Window: "Geplant", Affects: "Betrifft",
		Gone:         "Hier gibt es keine Statusseite.",
		MonitorTitle: "%s funktioniert nicht wie erwartet", MonitorBody: "Unsere Überwachung hat ein Problem mit %s festgestellt. Wir sehen es uns an.",
		MonitorResolved: "%s funktioniert wieder.",
		MonitorPaused:   "Die Überwachung von %s wurde pausiert.", MonitorRemoved: "Die Überwachung von %s wurde entfernt.",
		Months: [12]string{"Jan.", "Feb.", "März", "Apr.", "Mai", "Juni", "Juli", "Aug.", "Sep.", "Okt.", "Nov.", "Dez."},
	},
}

func statusWordsFor(locale string) statusWords {
	base, _, _ := strings.Cut(strings.ToLower(strings.ReplaceAll(locale, "_", "-")), "-")
	if w, ok := statusLanguages[base]; ok {
		return w
	}
	return statusLanguages["en"]
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

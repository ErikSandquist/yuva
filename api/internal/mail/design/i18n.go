package design

import (
	"fmt"
	"time"
)

type footerText struct {
	since, because     string
	unsubscribe, prefs string
	identityMissing    string
	months             [12]string
	date               func(t time.Time, months [12]string) string
	website, image     string
}

var footerTexts = map[string]footerText{
	"en": {
		since:           "You get this e-mail because you subscribed to %s on %s.",
		because:         "You get this e-mail because you subscribed to %s.",
		unsubscribe:     "Unsubscribe",
		prefs:           "Manage preferences",
		identityMissing: "Add the sender's legal name and postal address in the inbox settings.",
		months:          [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
		date: func(t time.Time, m [12]string) string {
			return fmt.Sprintf("%d %s %d", t.Day(), m[t.Month()-1], t.Year())
		},
		website: "Website",
		image:   "Image",
	},
	"tr": {
		since:           "Bu e-postayı %[2]s tarihinde %[1]s listesine abone olduğunuz için alıyorsunuz.",
		because:         "Bu e-postayı %s listesine abone olduğunuz için alıyorsunuz.",
		unsubscribe:     "Abonelikten çık",
		prefs:           "Tercihleri yönet",
		identityMissing: "Gönderenin yasal adını ve posta adresini gelen kutusu ayarlarına ekleyin.",
		months:          [12]string{"Ocak", "Şubat", "Mart", "Nisan", "Mayıs", "Haziran", "Temmuz", "Ağustos", "Eylül", "Ekim", "Kasım", "Aralık"},
		date: func(t time.Time, m [12]string) string {
			return fmt.Sprintf("%d %s %d", t.Day(), m[t.Month()-1], t.Year())
		},
		website: "Web sitesi",
		image:   "Görsel",
	},
	"de": {
		since:           "Sie erhalten diese E-Mail, weil Sie %s am %s abonniert haben.",
		because:         "Sie erhalten diese E-Mail, weil Sie %s abonniert haben.",
		unsubscribe:     "Abmelden",
		prefs:           "Einstellungen verwalten",
		identityMissing: "Tragen Sie den rechtlichen Namen und die Postanschrift des Absenders in den Einstellungen des Posteingangs ein.",
		months:          [12]string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"},
		date: func(t time.Time, m [12]string) string {
			return fmt.Sprintf("%d. %s %d", t.Day(), m[t.Month()-1], t.Year())
		},
		website: "Website",
		image:   "Bild",
	},
}

func textsFor(lang string) footerText {
	if t, ok := footerTexts[lang]; ok {
		return t
	}
	return footerTexts["en"]
}

var networkNames = map[string]string{
	"x": "X", "bluesky": "Bluesky", "mastodon": "Mastodon", "linkedin": "LinkedIn", "github": "GitHub",
	"youtube": "YouTube", "instagram": "Instagram", "facebook": "Facebook", "tiktok": "TikTok", "discord": "Discord", "threads": "Threads",
}

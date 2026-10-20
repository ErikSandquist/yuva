package api

import (
	"reflect"
	"strconv"
	"strings"
)

type roadmapWords struct {
	Roadmap, Ideas, Planned, InProgress, Shipped, Closed            string
	VoteOne, VoteMany, CommentOne, CommentMany                      string
	PlannedOn, InProgressOn, ShippedOn, ClosedOn, SuggestedOn       string
	AllShipped, RecentShipped, Vote, Suggest, Empty, Gone, Skip     string
	Home, History, Comments, NoComments, Stage, Votes, Reader, Back string
}

var roadmapLanguages = map[string]roadmapWords{
	"en": {
		Roadmap: "Roadmap", Ideas: "Ideas", Planned: "Planned", InProgress: "In progress", Shipped: "Shipped", Closed: "Closed",
		VoteOne: "1 vote", VoteMany: "%d votes", CommentOne: "1 comment", CommentMany: "%d comments",
		PlannedOn: "Planned on", InProgressOn: "In progress since", ShippedOn: "Shipped on", ClosedOn: "Closed on", SuggestedOn: "Suggested on",
		AllShipped: "All shipped", RecentShipped: "Shipped in the last 90 days", Vote: "Vote", Suggest: "Suggest an idea",
		Empty: "Nothing here yet.", Gone: "There is no roadmap here.", Skip: "Skip to content", Home: "Roadmap", History: "History",
		Comments: "Comments", NoComments: "No comments yet.", Stage: "Stage", Votes: "Votes", Reader: "A reader", Back: "Back to the roadmap",
	},
	"tr": {
		Roadmap: "Yol haritası", Ideas: "Fikirler", Planned: "Planlandı", InProgress: "Geliştiriliyor", Shipped: "Yayınlandı", Closed: "Kapatıldı",
		VoteOne: "1 oy", VoteMany: "%d oy", CommentOne: "1 yorum", CommentMany: "%d yorum",
		PlannedOn: "Planlanma tarihi", InProgressOn: "Geliştirme başlangıcı", ShippedOn: "Yayın tarihi", ClosedOn: "Kapanış tarihi", SuggestedOn: "Önerilme tarihi",
		AllShipped: "Tüm yayınlananlar", RecentShipped: "Son 90 günde yayınlananlar", Vote: "Oy ver", Suggest: "Fikir öner",
		Empty: "Henüz bir şey yok.", Gone: "Burada bir yol haritası yok.", Skip: "İçeriğe geç", Home: "Yol haritası", History: "Geçmiş",
		Comments: "Yorumlar", NoComments: "Henüz yorum yok.", Stage: "Aşama", Votes: "Oylar", Reader: "Bir okuyucu", Back: "Yol haritasına dön",
	},
	"de": {
		Roadmap: "Roadmap", Ideas: "Ideen", Planned: "Geplant", InProgress: "In Arbeit", Shipped: "Ausgeliefert", Closed: "Geschlossen",
		VoteOne: "1 Stimme", VoteMany: "%d Stimmen", CommentOne: "1 Kommentar", CommentMany: "%d Kommentare",
		PlannedOn: "Geplant am", InProgressOn: "In Arbeit seit", ShippedOn: "Ausgeliefert am", ClosedOn: "Geschlossen am", SuggestedOn: "Vorgeschlagen am",
		AllShipped: "Alles Ausgelieferte", RecentShipped: "In den letzten 90 Tagen ausgeliefert", Vote: "Abstimmen", Suggest: "Idee vorschlagen",
		Empty: "Noch nichts hier.", Gone: "Hier gibt es keine Roadmap.", Skip: "Zum Inhalt springen", Home: "Roadmap", History: "Verlauf",
		Comments: "Kommentare", NoComments: "Noch keine Kommentare.", Stage: "Stand", Votes: "Stimmen", Reader: "Jemand aus der Leserschaft", Back: "Zurück zur Roadmap",
	},
}

// A missing word falls back to English, so a language is added by filling the words it has.
func roadmapWordsFor(locale string) roadmapWords {
	tag := strings.ToLower(strings.ReplaceAll(locale, "_", "-"))
	base, _, _ := strings.Cut(tag, "-")
	w := roadmapLanguages["en"]
	l, ok := roadmapLanguages[base]
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

func (w roadmapWords) votes(n int32) string {
	if n == 1 {
		return w.VoteOne
	}
	return strings.Replace(w.VoteMany, "%d", strconv.Itoa(int(n)), 1)
}

func (w roadmapWords) comments(n int32) string {
	if n == 1 {
		return w.CommentOne
	}
	return strings.Replace(w.CommentMany, "%d", strconv.Itoa(int(n)), 1)
}

func (w roadmapWords) stage(stage string) string {
	switch stage {
	case stagePlanned:
		return w.Planned
	case stageInProgress:
		return w.InProgress
	case stageShipped:
		return w.Shipped
	case stageClosed:
		return w.Closed
	}
	return w.Ideas
}

func (w roadmapWords) stageOn(stage string) string {
	switch stage {
	case stagePlanned:
		return w.PlannedOn
	case stageInProgress:
		return w.InProgressOn
	case stageShipped:
		return w.ShippedOn
	case stageClosed:
		return w.ClosedOn
	}
	return w.SuggestedOn
}

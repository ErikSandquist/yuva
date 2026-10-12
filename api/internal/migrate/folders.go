package migrate

import (
	"slices"
	"strings"

	"github.com/productdevbook/yuva/api/internal/migrate/imapread"
)

// Folder kinds, as stored in migration_folders.kind.
const (
	KindInbox   = "inbox"
	KindSent    = "sent"
	KindArchive = "archive"
	KindAll     = "all"
	KindUser    = "user"
	KindLabel   = "label"
	KindSpam    = "spam"
	KindTrash   = "trash"
	KindDrafts  = "drafts"
	KindOther   = "other"
)

var specialUse = map[string]string{
	`\all`: KindAll, `\sent`: KindSent, `\archive`: KindArchive, `\junk`: KindSpam, `\trash`: KindTrash, `\drafts`: KindDrafts,
	`\flagged`: KindOther, `\important`: KindOther,
}

var folderNames = map[string]string{
	"sent": KindSent, "sent items": KindSent, "sent messages": KindSent, "sent mail": KindSent, "gesendet": KindSent,
	"gesendete objekte": KindSent, "gesendete elemente": KindSent, "gönderilmiş": KindSent, "gönderilenler": KindSent,
	"gönderilmiş öğeler": KindSent, "отправленные": KindSent, "envoyés": KindSent, "éléments envoyés": KindSent,
	"enviados": KindSent, "inviati": KindSent, "posta inviata": KindSent, "verzonden": KindSent, "verzonden items": KindSent,

	"archive": KindArchive, "archives": KindArchive, "archiv": KindArchive, "arşiv": KindArchive, "архив": KindArchive,
	"all mail": KindAll,

	"spam": KindSpam, "junk": KindSpam, "junk e-mail": KindSpam, "junk email": KindSpam, "bulk mail": KindSpam,
	"spamverdacht": KindSpam, "önemsiz": KindSpam, "önemsiz e-posta": KindSpam, "спам": KindSpam, "courrier indésirable": KindSpam,

	"trash": KindTrash, "deleted items": KindTrash, "deleted messages": KindTrash, "bin": KindTrash, "papierkorb": KindTrash,
	"gelöschte elemente": KindTrash, "gelöschte objekte": KindTrash, "çöp kutusu": KindTrash, "silinmiş öğeler": KindTrash,
	"удаленные": KindTrash, "корзина": KindTrash, "corbeille": KindTrash, "papelera": KindTrash, "cestino": KindTrash,

	"drafts": KindDrafts, "entwürfe": KindDrafts, "taslaklar": KindDrafts, "черновики": KindDrafts, "brouillons": KindDrafts,
	"borradores": KindDrafts, "bozze": KindDrafts,

	"outbox": KindOther, "postausgang": KindOther, "giden kutusu": KindOther, "исходящие": KindOther,
	"conversation history": KindOther, "sync issues": KindOther, "rss feeds": KindOther, "notes": KindOther,
	"chats": KindOther, "starred": KindOther, "important": KindOther,
}

// FolderKind tells what a folder holds, by its special-use attribute first and its name second.
// gmail marks user folders as Gmail labels.
func FolderKind(f imapread.Folder, gmail bool) string {
	for _, a := range f.Attrs {
		if k, ok := specialUse[strings.ToLower(a)]; ok {
			return k
		}
	}
	if strings.EqualFold(f.Name, "INBOX") {
		return KindInbox
	}
	leaf := f.Name
	if f.Delim != "" {
		if i := strings.LastIndex(leaf, f.Delim); i >= 0 {
			leaf = leaf[i+len(f.Delim):]
		}
	}
	if gmail && strings.HasPrefix(f.Name, "[Gmail]") {
		if k, ok := folderNames[strings.ToLower(leaf)]; ok {
			return k
		}
		return KindOther
	}
	if k, ok := folderNames[strings.ToLower(leaf)]; ok && (f.Delim == "" || !strings.Contains(f.Name, f.Delim) || strings.HasPrefix(strings.ToUpper(f.Name), "INBOX"+f.Delim)) {
		return k
	}
	if gmail {
		return KindLabel
	}
	return KindUser
}

// Readable kinds are imported; spam, trash, drafts and the rest never are.
func Readable(kind string) bool {
	switch kind {
	case KindInbox, KindSent, KindArchive, KindAll, KindUser, KindLabel:
		return true
	}
	return false
}

// DefaultSelected tells whether a preset reads a folder unless the team chooses otherwise.
func DefaultSelected(p Preset, kind string) bool {
	for _, f := range p.Folders {
		switch {
		case f == "INBOX" && kind == KindInbox,
			f == `\Sent` && kind == KindSent,
			f == `\Archive` && kind == KindArchive,
			f == `\All` && kind == KindAll,
			f == FolderUser && (kind == KindUser || kind == KindLabel) && !slices.Contains(p.Folders, `\All`):
			return true
		}
	}
	return false
}

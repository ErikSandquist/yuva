package migrate

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/email"
	"github.com/productdevbook/yuva/api/internal/migrate/imapread"
)

// HeaderFields are the header fields the scan reads: enough to thread, to tell the team's mail
// from its customers', and to recognise automatic mail as live ingest does.
var HeaderFields = []string{
	"Message-ID", "In-Reply-To", "References", "From", "To", "Cc", "Date", "Subject", "Return-Path",
	"Auto-Submitted", "Precedence", "List-Id", "X-Autoreply", "X-Autorespond", "X-Auto-Response-Suppress",
}

const (
	maxIDBytes   = 998
	dateEarliest = 7 * 24 * time.Hour
	dateLatest   = 24 * time.Hour
)

type Address struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

type Part struct {
	Type string `json:"type"`
	Size int64  `json:"size"`
}

// Meta is what the scan keeps of a message's headers.
type Meta struct {
	From    Address   `json:"from"`
	To      []Address `json:"to,omitempty"`
	Cc      []Address `json:"cc,omitempty"`
	Subject string    `json:"subject,omitempty"`
	Auto    bool      `json:"auto,omitempty"`
	At      time.Time `json:"at"`
	ID      string    `json:"id,omitempty"`
	Refs    []string  `json:"refs,omitempty"`
	Parts   []Part    `json:"parts,omitempty"`
	// Thread is the thread a provider put the message in (Google Takeout's X-GM-THRID); it
	// threads the message instead of its references.
	Thread string `json:"thread,omitempty"`
}

// ThreadNode is the node a provider's thread id joins its messages by; no message id can equal it.
func ThreadNode(thread string) string { return "x-gm-thrid:" + thread }

// Item is one message of a scanned folder.
type Item struct {
	SourceID  string
	FolderID  int32
	DedupeKey string
	Received  time.Time
	Size      int64
	Meta      Meta
}

// SourceID names a message of an IMAP folder for as long as its UIDVALIDITY holds.
func SourceID(folderID int32, uidValidity, uid uint32) string {
	return fmt.Sprintf("imap:%d:%d:%d", folderID, uidValidity, uid)
}

// ParseSourceID reads a SourceID back.
func ParseSourceID(id string) (folderID int32, uidValidity, uid uint32, ok bool) {
	var f int32
	var v, u uint32
	if n, err := fmt.Sscanf(id, "imap:%d:%d:%d", &f, &v, &u); err != nil || n != 3 || SourceID(f, v, u) != id {
		return 0, 0, 0, false
	}
	return f, v, u, true
}

// MessageTime is the Date header when it lies within a week before to a day after the server
// received the message, else the server's time, and never later than now.
func MessageTime(date, received, now time.Time) time.Time {
	at := received
	if received.IsZero() {
		at = date
	} else if !date.IsZero() && !date.Before(received.Add(-dateEarliest)) && !date.After(received.Add(dateLatest)) {
		at = date
	}
	if at.IsZero() || at.After(now) {
		at = now
	}
	return at.UTC()
}

// ScanHeader reads one fetched header block. A message without a usable Message-ID gets a key
// from a hash of its headers, the same for every copy of it.
func ScanHeader(h imapread.Header, folderID int32, uidValidity uint32, now time.Time) Item {
	it := ScanRaw(h.Header, h.Parts, h.InternalDate, h.Size, now)
	it.SourceID, it.FolderID = SourceID(folderID, uidValidity, h.UID), folderID
	return it
}

// ScanRaw reads a message's header block and the leaves of its structure, as a server or a
// file gives them.
func ScanRaw(header []byte, parts []imapread.Part, received time.Time, size int64, now time.Time) Item {
	it := Item{Received: received.UTC(), Size: size}
	m, err := email.Parse(append(append([]byte{}, header...), "\r\n"...))
	if err != nil {
		m = &email.Message{}
	}
	it.Meta = Meta{
		From: address(m.From), To: addresses(m.To), Cc: addresses(m.Cc), Subject: truncate(strings.Join(strings.Fields(m.Subject), " "), 500),
		Auto: m.Auto, At: MessageTime(m.Date, received, now),
	}
	if id := m.MessageID; id != "" && len(id) <= maxIDBytes {
		it.Meta.ID = id
		it.DedupeKey = id
	} else {
		sum := sha256.Sum256(header)
		it.DedupeKey = "hdr-" + hex.EncodeToString(sum[:20]) + "@yuva.invalid"
	}
	for _, r := range append(append([]string{}, m.InReplyTo...), m.References...) {
		if r != "" && r != it.Meta.ID {
			it.Meta.Refs = append(it.Meta.Refs, r)
		}
	}
	for _, p := range parts {
		if p.Filename == "" && (p.Type == "text/plain" || p.Type == "text/html" || strings.HasPrefix(p.Type, "message/")) {
			continue
		}
		size := p.Size
		if p.Encoding == "base64" {
			size = size * 3 / 4
		}
		it.Meta.Parts = append(it.Meta.Parts, Part{Type: p.Type, Size: size})
	}
	if it.Received.IsZero() {
		it.Received = it.Meta.At
	}
	return it
}

func address(a email.Address) Address {
	return Address{Name: truncate(strings.TrimSpace(a.Name), 200), Email: strings.ToLower(strings.TrimSpace(a.Email))}
}

func addresses(in []email.Address) []Address {
	var out []Address
	for _, a := range in {
		if a.Email != "" {
			out = append(out, address(a))
		}
	}
	return out
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

var subjectPrefix = regexp.MustCompile(`(?i)^\s*((re|aw|sv|ynt|cvp|antw|rv|res|odp|fwd?|wg|ileti|enc|tr)\s*(\[\d+\])?\s*:\s*)+`)

// BaseSubject is a subject without its reply and forward prefixes.
func BaseSubject(s string) string {
	return strings.TrimSpace(subjectPrefix.ReplaceAllString(s, ""))
}

// NameID is the name-based UUID (version 5) of name in the namespace ns, so a retried import
// writes the same rows and objects.
func NameID(ns uuid.UUID, name string) uuid.UUID {
	h := sha1.New()
	h.Write(ns[:])
	h.Write([]byte(name))
	var u uuid.UUID
	copy(u[:], h.Sum(nil))
	u[6] = u[6]&0x0f | 0x50
	u[8] = u[8]&0x3f | 0x80
	return u
}

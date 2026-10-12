// Command gen writes the mailboxes the IMAP migration is tested against, as Maildir trees for
// the Dovecot server of the migrate-test compose profile, and expected.json with what a migration
// of each mailbox must produce. The output is deterministic; nothing is copied from real mail.
// Everything is world-writable because Dovecot runs as its own user in the container.
//
//	go run ./internal/migrate/testdata/gen -out /path/to/fixtures [-large]
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/text/encoding/charmap"
)

type addr struct{ name, email string }

func (a addr) String() string {
	if a.name == "" {
		return a.email
	}
	return mime.QEncoding.Encode("utf-8", a.name) + " <" + a.email + ">"
}

type attachment struct {
	name, contentType string
	size              int
	inline            bool
}

type message struct {
	folders    []string
	from       addr
	fromRaw    string
	to, cc     []addr
	subject    string
	subjectRaw string
	id         string
	inReplyTo  string
	references []string
	date       time.Time
	received   time.Time
	extra      [][2]string
	body       string
	atts       []attachment
	seen       bool
}

type thread struct {
	Subject            string    `json:"subject"`
	Contact            string    `json:"contact"`
	In                 int       `json:"in"`
	Out                int       `json:"out"`
	Authors            []string  `json:"authors"`
	Labels             []string  `json:"labels"`
	LastAt             time.Time `json:"last_at"`
	AttachmentsKept    int       `json:"attachments_kept"`
	AttachmentsSkipped int       `json:"attachments_skipped"`
	SynthesizedID      bool      `json:"synthesized_message_id,omitempty"`
	DateFromServer     bool      `json:"date_from_server,omitempty"`
}

type folder struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Messages int    `json:"messages"`
	Read     bool   `json:"read"`
}

type mailbox struct {
	User          string         `json:"user"`
	Address       string         `json:"address"`
	TeamAddresses []string       `json:"team_addresses"`
	Folders       []folder       `json:"folders"`
	Conversations int            `json:"conversations"`
	MessagesIn    int            `json:"messages_in"`
	MessagesOut   int            `json:"messages_out"`
	Contacts      []string       `json:"contacts"`
	Labels        []string       `json:"labels"`
	Skipped       map[string]int `json:"skipped"`
	Threads       []thread       `json:"threads"`

	messages []message
	kinds    map[string]string
}

var t0 = time.Date(2025, 3, 3, 9, 0, 0, 0, time.UTC)

func at(hours int) time.Time { return t0.Add(time.Duration(hours) * time.Hour) }

var (
	support  = addr{"Acme Support", "support@example.com"}
	sales    = addr{"Acme Sales", "sales@example.com"}
	agent    = addr{"Deniz Kaya", "deniz@example.com"}
	ayse     = addr{"Ayşe Yılmaz", "ayse@example.org"}
	bob      = addr{"Bob Stone", "bob@example.net"}
	carol    = addr{"Carol White", "carol@example.org"}
	dave     = addr{"Dave Green", "dave@example.org"}
	erin     = addr{"Erin Black", "erin@example.org"}
	frank    = addr{"", "frank@example.net"}
	news     = addr{"Weekly News", "news@lists.example.net"}
	mailerd  = addr{"Mail Delivery Subsystem", "mailer-daemon@example.net"}
	olga     = addr{"Ольга Петрова", "olga@example.org"}
	selin    = addr{"Selin Aydın", "selin@example.org"}
	destek   = addr{"Destek", "destek@example.com"}
	musteri  = addr{"Mert Demir", "mert@example.org"}
	bulkAddr = addr{"Bulk", "bulk@example.com"}
)

func acme() *mailbox {
	m := &mailbox{User: "acme", Address: support.email, TeamAddresses: []string{support.email, sales.email, agent.email},
		kinds: map[string]string{"INBOX": "inbox", "Sent": "sent", "Archive": "archive", "Spam": "spam", "Trash": "trash", "Drafts": "drafts", "Customers": "user"}}
	add := func(msgs ...message) { m.messages = append(m.messages, msgs...) }

	add(
		message{folders: []string{"INBOX"}, from: ayse, to: []addr{support}, subject: "Order 1001 never arrived", id: "<order-1001-1@example.org>", date: at(0), body: "Hello, my order has not arrived."},
		message{folders: []string{"Sent"}, from: support, to: []addr{ayse}, subject: "Re: Order 1001 never arrived", id: "<order-1001-2@example.com>", inReplyTo: "<order-1001-1@example.org>",
			references: []string{"<order-1001-1@example.org>"}, date: at(2), body: "Sorry, we are sending it again.", seen: true},
		message{folders: []string{"INBOX", "Customers"}, from: ayse, to: []addr{support}, subject: "Re: Order 1001 never arrived", id: "<order-1001-3@example.org>", inReplyTo: "<order-1001-2@example.com>",
			references: []string{"<order-1001-1@example.org>", "<order-1001-2@example.com>"}, date: at(5), body: "Thank you!"},
	)
	m.Threads = append(m.Threads, thread{Subject: "Order 1001 never arrived", Contact: ayse.email, In: 2, Out: 1, Authors: []string{ayse.email, support.email}, Labels: []string{"Customers"}, LastAt: at(5)})

	add(
		message{folders: []string{"Sent"}, from: sales, to: []addr{bob}, subject: "Your quote", id: "<quote-1@example.com>", date: at(24), body: "Here is the quote you asked for.", seen: true},
		message{folders: []string{"INBOX"}, from: bob, to: []addr{sales}, subject: "Re: Your quote", id: "<quote-2@example.net>", inReplyTo: "<quote-1@example.com>", references: []string{"<quote-1@example.com>"}, date: at(30), body: "Looks good."},
	)
	m.Threads = append(m.Threads, thread{Subject: "Your quote", Contact: bob.email, In: 1, Out: 1, Authors: []string{sales.email, bob.email}, Labels: []string{}, LastAt: at(30)})

	add(
		message{folders: []string{"INBOX"}, from: carol, to: []addr{support}, cc: []addr{dave}, subject: "Invoice question", id: "<invoice-1@example.org>", date: at(48), body: "Which VAT rate applies?"},
		message{folders: []string{"INBOX"}, from: dave, to: []addr{support}, cc: []addr{carol}, subject: "Re: Invoice question", id: "<invoice-2@example.org>", inReplyTo: "<invoice-1@example.org>",
			references: []string{"<invoice-1@example.org>"}, date: at(49), body: "Adding: we are in Germany."},
		message{folders: []string{"Sent"}, from: agent, to: []addr{carol}, cc: []addr{dave}, subject: "Re: Invoice question", id: "<invoice-3@example.com>", inReplyTo: "<invoice-2@example.org>",
			references: []string{"<invoice-1@example.org>", "<invoice-2@example.org>"}, date: at(50), body: "19% then.", seen: true},
	)
	m.Threads = append(m.Threads, thread{Subject: "Invoice question", Contact: carol.email, In: 2, Out: 1, Authors: []string{carol.email, dave.email, agent.email}, Labels: []string{}, LastAt: at(50)})

	add(message{folders: []string{"INBOX"}, from: agent, to: []addr{support}, subject: "Lunch on Friday?", id: "<internal-1@example.com>", date: at(60), body: "Team only."})
	add(
		message{folders: []string{"INBOX"}, from: news, to: []addr{support}, subject: "This week in widgets", id: "<news-1@lists.example.net>", date: at(70), body: "News.",
			extra: [][2]string{{"List-Id", "Weekly News <weekly.lists.example.net>"}, {"Precedence", "list"}}},
		message{folders: []string{"INBOX"}, from: mailerd, fromRaw: "<>", to: []addr{support}, subject: "Undelivered Mail Returned to Sender", id: "<dsn-1@example.net>", date: at(71), body: "Delivery failed.",
			extra: [][2]string{{"Auto-Submitted", "auto-replied"}}},
	)
	add(
		message{folders: []string{"Spam"}, from: addr{"Prize", "prize@example.net"}, to: []addr{support}, subject: "You won", id: "<spam-1@example.net>", date: at(72), body: "Click."},
		message{folders: []string{"Trash"}, from: erin, to: []addr{support}, subject: "Deleted question", id: "<trash-1@example.org>", date: at(73), body: "Ignore me."},
		message{folders: []string{"Drafts"}, from: support, to: []addr{erin}, subject: "Unsent reply", id: "<draft-1@example.com>", date: at(74), body: "Draft.", seen: true},
	)

	add(message{folders: []string{"INBOX"}, from: frank, to: []addr{support}, subject: "No message id here", date: at(80), body: "My client sends no Message-ID."})
	m.Threads = append(m.Threads, thread{Subject: "No message id here", Contact: frank.email, In: 1, Authors: []string{frank.email}, Labels: []string{}, LastAt: at(80), SynthesizedID: true})

	add(
		message{folders: []string{"INBOX"}, from: erin, to: []addr{support}, subject: "Loop A", id: "<loop-a@example.org>", references: []string{"<loop-b@example.org>"}, date: at(90), body: "A."},
		message{folders: []string{"INBOX"}, from: erin, to: []addr{support}, subject: "Re: Loop A", id: "<loop-b@example.org>", references: []string{"<loop-a@example.org>"}, inReplyTo: "<loop-a@example.org>", date: at(91), body: "B."},
	)
	m.Threads = append(m.Threads, thread{Subject: "Loop A", Contact: erin.email, In: 2, Authors: []string{erin.email}, Labels: []string{}, LastAt: at(91)})

	add(message{folders: []string{"INBOX"}, fromRaw: "=?ISO-8859-9?Q?Ay=FEe_Y=FDlmaz?= <ayse.tr@example.org>", to: []addr{support},
		subjectRaw: "=?ISO-8859-9?Q?Sipari=FE_iptali?=", id: "<tr-1@example.org>", date: at(100), body: "Siparisimi iptal etmek istiyorum."})
	m.Threads = append(m.Threads, thread{Subject: "Sipariş iptali", Contact: "ayse.tr@example.org", In: 1, Authors: []string{"ayse.tr@example.org"}, Labels: []string{}, LastAt: at(100)})
	ru, _ := charmap.Windows1251.NewEncoder().String("Возврат денег")
	ruName, _ := charmap.Windows1251.NewEncoder().String(olga.name)
	add(message{folders: []string{"INBOX"}, fromRaw: "=?windows-1251?B?" + base64.StdEncoding.EncodeToString([]byte(ruName)) + "?= <" + olga.email + ">", to: []addr{support},
		subjectRaw: "=?windows-1251?B?" + base64.StdEncoding.EncodeToString([]byte(ru)) + "?=", id: "<ru-1@example.org>", date: at(101), body: "Prosba vernut dengi."})
	m.Threads = append(m.Threads, thread{Subject: "Возврат денег", Contact: olga.email, In: 1, Authors: []string{olga.email}, Labels: []string{}, LastAt: at(101)})

	add(message{folders: []string{"Archive"}, from: dave, to: []addr{support}, subject: "Clock is wrong", id: "<clock-1@example.org>", date: time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC), received: at(110), body: "My clock says 2019."})
	m.Threads = append(m.Threads, thread{Subject: "Clock is wrong", Contact: dave.email, In: 1, Authors: []string{dave.email}, Labels: []string{}, LastAt: at(110), DateFromServer: true})

	add(message{folders: []string{"INBOX"}, from: bob, to: []addr{support}, subject: "Screenshots", id: "<files-1@example.net>", date: at(120), body: "See attached.",
		atts: []attachment{{name: "screen.png", contentType: "image/png", size: 2048}, {name: "logo.png", contentType: "image/png", size: 512, inline: true}, {name: "setup.exe", contentType: "application/x-msdownload", size: 4096}}})
	m.Threads = append(m.Threads, thread{Subject: "Screenshots", Contact: bob.email, In: 1, Authors: []string{bob.email}, Labels: []string{}, LastAt: at(120), AttachmentsKept: 2, AttachmentsSkipped: 1})

	m.Skipped = map[string]int{"internal": 1, "automatic": 2, "spam": 1, "trash": 1, "drafts": 1, "duplicate_copies": 1}
	return m
}

func yerel() *mailbox {
	m := &mailbox{User: "yerel", Address: destek.email, TeamAddresses: []string{destek.email},
		kinds: map[string]string{"INBOX": "inbox", "Gönderilmiş Öğeler": "sent", "Önemsiz": "spam", "Çöp Kutusu": "trash", "Taslaklar": "drafts", "Müşteriler": "user", "Отправленные": "sent"}}
	m.messages = []message{
		{folders: []string{"INBOX", "Müşteriler"}, from: musteri, to: []addr{destek}, subject: "Fatura nerede?", id: "<fatura-1@example.org>", date: at(200), body: "Faturamı bulamıyorum."},
		{folders: []string{"Gönderilmiş Öğeler"}, from: destek, to: []addr{musteri}, subject: "Ynt: Fatura nerede?", id: "<fatura-2@example.com>", inReplyTo: "<fatura-1@example.org>", references: []string{"<fatura-1@example.org>"}, date: at(201), body: "Ekte.", seen: true},
		{folders: []string{"INBOX"}, from: selin, to: []addr{destek}, subject: "Teşekkürler", id: "<tesekkur-1@example.org>", date: at(210), body: "Her şey yolunda."},
		{folders: []string{"Отправленные"}, from: destek, to: []addr{selin}, subject: "Re: Teşekkürler", id: "<tesekkur-2@example.com>", inReplyTo: "<tesekkur-1@example.org>", references: []string{"<tesekkur-1@example.org>"}, date: at(211), body: "Rica ederiz.", seen: true},
		{folders: []string{"Önemsiz"}, from: addr{"", "kampanya@example.net"}, to: []addr{destek}, subject: "Kampanya", id: "<kampanya-1@example.net>", date: at(220), body: "İndirim."},
		{folders: []string{"Çöp Kutusu"}, from: selin, to: []addr{destek}, subject: "Silindi", id: "<silindi-1@example.org>", date: at(221), body: "Sil."},
		{folders: []string{"Taslaklar"}, from: destek, to: []addr{selin}, subject: "Taslak", id: "<taslak-1@example.com>", date: at(222), body: "Taslak.", seen: true},
	}
	m.Threads = []thread{
		{Subject: "Fatura nerede?", Contact: musteri.email, In: 1, Out: 1, Authors: []string{musteri.email, destek.email}, Labels: []string{"Müşteriler"}, LastAt: at(201)},
		{Subject: "Teşekkürler", Contact: selin.email, In: 1, Out: 1, Authors: []string{selin.email, destek.email}, Labels: []string{}, LastAt: at(211)},
	}
	m.Skipped = map[string]int{"spam": 1, "trash": 1, "drafts": 1, "duplicate_copies": 1}
	return m
}

func bulk() *mailbox {
	m := &mailbox{User: "bulk", Address: bulkAddr.email, TeamAddresses: []string{bulkAddr.email}, kinds: map[string]string{"INBOX": "inbox"}}
	for i := range 10000 {
		from := addr{"", fmt.Sprintf("customer%05d@example.org", i)}
		m.messages = append(m.messages, message{folders: []string{"INBOX"}, from: from, to: []addr{bulkAddr}, subject: fmt.Sprintf("Question %d", i),
			id: fmt.Sprintf("<bulk-%05d@example.org>", i), date: at(300 + i/10), body: fmt.Sprintf("Question number %d.", i)})
		m.Threads = append(m.Threads, thread{Subject: fmt.Sprintf("Question %d", i), Contact: from.email, In: 1, Authors: []string{from.email}, Labels: []string{}, LastAt: at(300 + i/10)})
	}
	m.messages = append(m.messages,
		message{folders: []string{"INBOX"}, from: bob, to: []addr{bulkAddr}, subject: "Huge video", id: "<huge-1@example.net>", date: at(2000), body: "60 MiB.",
			atts: []attachment{{name: "video.mp4", contentType: "video/mp4", size: 60 << 20}}},
		message{folders: []string{"INBOX"}, from: carol, to: []addr{bulkAddr}, subject: "Large scan", id: "<large-1@example.org>", date: at(2001), body: "Over the attachment limit.",
			atts: []attachment{{name: "scan.pdf", contentType: "application/pdf", size: 26 << 20}}},
	)
	m.Threads = append(m.Threads, thread{Subject: "Large scan", Contact: carol.email, In: 1, Authors: []string{carol.email}, Labels: []string{}, LastAt: at(2001), AttachmentsSkipped: 1})
	m.Skipped = map[string]int{"too_large": 1}
	return m
}

func (m message) raw() []byte {
	var b strings.Builder
	h := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	if m.fromRaw == "<>" {
		h("Return-Path", "<>")
		h("From", m.from.String())
	} else if m.fromRaw != "" {
		h("From", m.fromRaw)
	} else {
		h("From", m.from.String())
	}
	list := func(as []addr) string {
		var s []string
		for _, a := range as {
			s = append(s, a.String())
		}
		return strings.Join(s, ", ")
	}
	h("To", list(m.to))
	if len(m.cc) > 0 {
		h("Cc", list(m.cc))
	}
	if m.subjectRaw != "" {
		h("Subject", m.subjectRaw)
	} else {
		h("Subject", mime.QEncoding.Encode("utf-8", m.subject))
	}
	h("Date", m.date.Format(time.RFC1123Z))
	if m.id != "" {
		h("Message-ID", m.id)
	}
	if m.inReplyTo != "" {
		h("In-Reply-To", m.inReplyTo)
	}
	if len(m.references) > 0 {
		h("References", strings.Join(m.references, " "))
	}
	for _, e := range m.extra {
		h(e[0], e[1])
	}
	h("MIME-Version", "1.0")
	if len(m.atts) == 0 {
		h("Content-Type", "text/plain; charset=utf-8")
		h("Content-Transfer-Encoding", "8bit")
		b.WriteString("\r\n" + m.body + "\r\n")
		return []byte(b.String())
	}
	const boundary = "yuva-fixture-boundary"
	h("Content-Type", `multipart/mixed; boundary="`+boundary+`"`)
	b.WriteString("\r\n--" + boundary + "\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + m.body + "\r\n")
	for i, a := range m.atts {
		disp := "attachment"
		b.WriteString("--" + boundary + "\r\nContent-Type: " + a.contentType + "; name=\"" + a.name + "\"\r\n")
		if a.inline {
			disp = "inline"
			b.WriteString(fmt.Sprintf("Content-ID: <part%d@fixture>\r\n", i))
		}
		b.WriteString("Content-Disposition: " + disp + "; filename=\"" + a.name + "\"\r\nContent-Transfer-Encoding: base64\r\n\r\n")
		data := make([]byte, a.size)
		for j := range data {
			data[j] = byte(j*7 + i)
		}
		enc := base64.StdEncoding.EncodeToString(data)
		for len(enc) > 76 {
			b.WriteString(enc[:76] + "\r\n")
			enc = enc[76:]
		}
		b.WriteString(enc + "\r\n")
	}
	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String())
}

func folderDir(root, name string) string {
	if name == "INBOX" {
		return filepath.Join(root, "Maildir")
	}
	return filepath.Join(root, "Maildir", "."+strings.ReplaceAll(name, "/", "."))
}

func write(out string, m *mailbox) error {
	home := filepath.Join(out, m.User)
	if err := os.MkdirAll(filepath.Join(home, "rawlog"), 0o777); err != nil {
		return err
	}
	counts := map[string]int{}
	for name := range m.kinds {
		for _, sub := range []string{"cur", "new", "tmp"} {
			if err := os.MkdirAll(filepath.Join(folderDir(home, name), sub), 0o777); err != nil {
				return err
			}
		}
	}
	for i, msg := range m.messages {
		raw := msg.raw()
		received := msg.received
		if received.IsZero() {
			received = msg.date.Add(30 * time.Second)
		}
		flags := ""
		if msg.seen {
			flags = "S"
		}
		for _, f := range msg.folders {
			if _, ok := m.kinds[f]; !ok {
				return fmt.Errorf("%s: unknown folder %s", m.User, f)
			}
			counts[f]++
			name := fmt.Sprintf("%d.M%dP1.fixture,S=%d:2,%s", received.Unix(), i, len(raw), flags)
			p := filepath.Join(folderDir(home, f), "cur", name)
			if err := os.WriteFile(p, raw, 0o666); err != nil {
				return err
			}
			if err := os.Chtimes(p, received, received); err != nil {
				return err
			}
		}
	}
	m.Folders = nil
	for name, kind := range m.kinds {
		read := kind == "inbox" || kind == "sent" || kind == "archive" || kind == "user"
		m.Folders = append(m.Folders, folder{Name: name, Kind: kind, Messages: counts[name], Read: read})
	}
	sort.Slice(m.Folders, func(a, b int) bool { return m.Folders[a].Name < m.Folders[b].Name })
	contacts := map[string]bool{}
	labels := map[string]bool{}
	m.MessagesIn, m.MessagesOut = 0, 0
	for _, t := range m.Threads {
		m.MessagesIn += t.In
		m.MessagesOut += t.Out
		for _, a := range t.Authors {
			if !contains(m.TeamAddresses, a) {
				contacts[a] = true
			}
		}
		for _, l := range t.Labels {
			labels[l] = true
		}
	}
	m.Conversations = len(m.Threads)
	m.Contacts, m.Labels = keys(contacts), keys(labels)
	return filepath.WalkDir(home, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.Chmod(p, 0o777)
		}
		return os.Chmod(p, 0o666)
	})
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func keys(m map[string]bool) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func main() {
	out := flag.String("out", "", "directory to write the mailboxes and expected.json into")
	large := flag.Bool("large", false, "also write the bulk mailbox: 10,000 messages, a 60 MiB message and a 26 MiB attachment")
	flag.Parse()
	if *out == "" {
		log.Fatal("-out is required")
	}
	boxes := []*mailbox{acme(), yerel()}
	if *large {
		boxes = append(boxes, bulk())
	}
	for _, m := range boxes {
		if err := write(*out, m); err != nil {
			log.Fatal(err)
		}
	}
	b, err := json.MarshalIndent(map[string]any{"generated_by": "api/internal/migrate/testdata/gen", "base_time": t0, "mailboxes": boxes}, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "expected.json"), append(b, '\n'), 0o666); err != nil {
		log.Fatal(err)
	}
}

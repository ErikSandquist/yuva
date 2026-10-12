package fileread

import (
	"archive/zip"
	"bytes"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"time"
)

var limits = Limits{MaxEntries: 1000000, MaxRatio: 100, MaxEntry: 1 << 30}

func msg(id, subject, body string, extra ...string) string {
	h := "From: Ayse <ayse@example.org>\r\nTo: help@example.com\r\nSubject: " + subject + "\r\nMessage-ID: " + id + "\r\n"
	for _, e := range extra {
		h += e + "\r\n"
	}
	return h + "\r\n" + body
}

func scanAll(t *testing.T, data []byte, l Layout, variant string) []Message {
	t.Helper()
	var out []Message
	err := Scan(bytes.NewReader(data), int64(len(data)), l, variant, Cursor{}, 1<<20, func(m Message) error {
		if m.SourceID != "" {
			out = append(out, m)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestMboxVariantsAndOffsets(t *testing.T) {
	file := "From MAILER-DAEMON Thu Mar 06 09:00:30 +0000 2025\n" +
		strings.ReplaceAll(msg("<a@x>", "One", "Hi\n>From here\n>>From there\nbye\n"), "\r\n", "\n") + "\n" +
		"From someone Thu Mar  6 10:00:00 2025\n" +
		strings.ReplaceAll(msg("<b@x>", "Two", "Second\n"), "\r\n", "\n") + "\n"
	data := []byte(file)
	l, folders, err := Inspect(bytes.NewReader(data), int64(len(data)), "Clients.mbox", limits)
	if err != nil {
		t.Fatal(err)
	}
	if l.Format != FormatMbox || len(folders) != 1 || folders[0].Name != "Clients" || folders[0].Kind != "archive" {
		t.Fatalf("layout %+v folders %+v", l, folders)
	}
	for variant, body := range map[string]string{MboxRD: "Hi\r\nFrom here\r\n>From there\r\nbye\r\n", MboxO: "Hi\r\nFrom here\r\n>>From there\r\nbye\r\n"} {
		got := scanAll(t, data, l, variant)
		if len(got) != 2 {
			t.Fatalf("%s: %d messages", variant, len(got))
		}
		if !strings.HasSuffix(string(got[0].Raw), "\r\n\r\n"+body) {
			t.Errorf("%s: raw %q", variant, got[0].Raw)
		}
		if want := time.Date(2025, 3, 6, 9, 0, 30, 0, time.UTC); !got[0].Received.Equal(want) {
			t.Errorf("received %s", got[0].Received)
		}
		if want := time.Date(2025, 3, 6, 10, 0, 0, 0, time.UTC); !got[1].Received.Equal(want) {
			t.Errorf("second received %s", got[1].Received)
		}
		for _, m := range got {
			again, tooLarge, err := Read(bytes.NewReader(data), int64(len(data)), m.SourceID, variant, 1<<20)
			if err != nil || tooLarge || !bytes.Equal(again, m.Raw) {
				t.Errorf("%s: read %s again: %q %v %v", variant, m.SourceID, again, tooLarge, err)
			}
		}
	}
	got := scanAll(t, data, l, MboxRD)
	var resumed []Message
	err = Scan(bytes.NewReader(data), int64(len(data)), l, MboxRD, got[0].Next, 1<<20, func(m Message) error {
		resumed = append(resumed, m)
		return nil
	})
	if err != nil || len(resumed) != 1 || resumed[0].SourceID != got[1].SourceID {
		t.Fatalf("resumed from %+v: %+v %v", got[0].Next, resumed, err)
	}
}

func TestTakeoutLabelsAndThreads(t *testing.T) {
	data := []byte("From 1@xxx Mon Mar 03 09:00:30 +0000 2025\r\n" +
		msg("<a@x>", "One", "Hi\r\n", "X-GM-THRID: 1826", `X-Gmail-Labels: Inbox,Opened,Customers,"Big, Co",Category Updates`) + "\r\n" +
		"From 2@xxx Mon Mar 03 10:00:30 +0000 2025\r\n" +
		msg("<b@x>", "Spam", "Buy\r\n", "X-GM-THRID: 1900", "X-Gmail-Labels: Spam,Unread") + "\r\n")
	l, folders, err := Inspect(bytes.NewReader(data), int64(len(data)), "All mail Including Spam and Trash.mbox", limits)
	if err != nil || l.Format != FormatTakeout || !l.Takeout || len(folders) != 1 || folders[0].Name != GmailAll || folders[0].Kind != "all" {
		t.Fatalf("%+v %+v %v", l, folders, err)
	}
	got := scanAll(t, data, l, MboxRD)
	if len(got) != 2 {
		t.Fatalf("%d messages", len(got))
	}
	if got[0].Folder != GmailAll || got[0].Thread != "1826" || !reflect.DeepEqual(got[0].Labels, []string{"INBOX", "Customers", "Big, Co", "[Gmail]/Category Updates"}) {
		t.Errorf("first: %q %q %q", got[0].Folder, got[0].Thread, got[0].Labels)
	}
	if !bytes.HasPrefix(got[0].Raw, []byte("From: Ayse")) || !bytes.HasPrefix(got[0].Header, []byte("From: Ayse")) {
		t.Errorf("Takeout's own fields are kept: %q", got[0].Raw)
	}
	again, _, err := Read(bytes.NewReader(data), int64(len(data)), got[0].SourceID, MboxRD, 1<<20)
	if err != nil || !bytes.Equal(again, got[0].Raw) {
		t.Errorf("read again: %q %v", again, err)
	}
	if got[1].Folder != "" || !reflect.DeepEqual(got[1].Labels, []string{"[Gmail]/Spam"}) {
		t.Errorf("spam: %q %q", got[1].Folder, got[1].Labels)
	}
}

type zipFile struct {
	name    string
	body    string
	method  uint16
	mode    fs.FileMode
	modTime time.Time
}

func makeZip(t *testing.T, files ...zipFile) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range files {
		h := &zip.FileHeader{Name: f.name, Method: f.method, Modified: f.modTime}
		if h.Method == 0 && f.method == 0 && !strings.HasSuffix(f.name, ".mbox") && !strings.HasSuffix(f.name, "/mbox") {
			h.Method = zip.Deflate
		}
		if f.mode != 0 {
			h.SetMode(f.mode)
		}
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(f.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestZipFoldersBecomeFolders(t *testing.T) {
	at := time.Date(2025, 3, 3, 9, 0, 0, 0, time.UTC)
	eml := makeZip(t,
		zipFile{name: "export/INBOX/1.eml", body: msg("<1@x>", "One", "a\n"), modTime: at},
		zipFile{name: "export/Sent/2.eml", body: msg("<2@x>", "Two", "b\n"), modTime: at},
		zipFile{name: "export/Customers/Big/3.eml", body: msg("<3@x>", "Three", "c\n"), modTime: at},
		zipFile{name: "export/Spam/4.eml", body: msg("<4@x>", "Four", "d\n"), modTime: at},
		zipFile{name: "__MACOSX/export/._1.eml", body: "junk"},
		zipFile{name: "export/.DS_Store", body: "junk"},
	)
	l, folders, err := Inspect(bytes.NewReader(eml), int64(len(eml)), "x.zip", limits)
	if err != nil || l.Format != FormatEmlZip {
		t.Fatalf("%+v %v", l, err)
	}
	want := []Folder{{"INBOX", "/", "inbox", 1}, {"Sent", "/", "sent", 1}, {"Customers/Big", "/", "user", 1}, {"Spam", "/", "spam", 1}}
	if !reflect.DeepEqual(folders, want) {
		t.Fatalf("eml folders %+v", folders)
	}
	got := scanAll(t, eml, l, "")
	if len(got) != 4 || got[2].Folder != "Customers/Big" || !got[0].Received.Equal(at) || !strings.Contains(string(got[0].Raw), "a\r\n") {
		t.Fatalf("eml messages %+v", got)
	}
	again, _, err := Read(bytes.NewReader(eml), int64(len(eml)), got[1].SourceID, "", 1<<20)
	if err != nil || !bytes.Equal(again, got[1].Raw) {
		t.Fatalf("read again: %q %v", again, err)
	}

	maildir := makeZip(t,
		zipFile{name: "Maildir/cur/1741000000.M1P1.host,S=10:2,S", body: msg("<1@x>", "One", "a\n")},
		zipFile{name: "Maildir/new/1741000100.M2P1.host", body: msg("<2@x>", "Two", "b\n")},
		zipFile{name: "Maildir/.Sent/cur/1741000200.M3P1.host:2,S", body: msg("<3@x>", "Three", "c\n")},
		zipFile{name: "Maildir/.Customers.Big/cur/1741000300.M4P1.host:2,", body: msg("<4@x>", "Four", "d\n")},
		zipFile{name: "Maildir/.Trash/cur/1741000400.M5P1.host:2,S", body: msg("<5@x>", "Five", "e\n")},
		zipFile{name: "Maildir/cur/1741000500.M6P1.host:2,ST", body: msg("<6@x>", "Six", "f\n")},
		zipFile{name: "Maildir/dovecot-uidlist", body: "1 V1"},
	)
	l, folders, err = Inspect(bytes.NewReader(maildir), int64(len(maildir)), "x.zip", limits)
	if err != nil || l.Format != FormatMaildirZip {
		t.Fatalf("%+v %v", l, err)
	}
	want = []Folder{{"INBOX", "/", "inbox", 2}, {"Sent", "/", "sent", 1}, {"Customers/Big", "/", "user", 1}, {"Trash", "/", "trash", 1}}
	if !reflect.DeepEqual(folders, want) {
		t.Fatalf("maildir folders %+v", folders)
	}
	got = scanAll(t, maildir, l, "")
	if len(got) != 5 || !got[0].Received.Equal(time.Unix(1741000000, 0).UTC()) || got[3].Folder != "Customers/Big" {
		t.Fatalf("maildir messages %+v", got)
	}
}

func TestZipRefusals(t *testing.T) {
	big := strings.Repeat("\x00", 4<<20)
	cases := map[string]struct {
		data   []byte
		lim    Limits
		reason string
		entry  string
	}{
		"traversal": {makeZip(t, zipFile{name: "../evil.eml", body: msg("<1@x>", "x", "y")}), limits, RefusedPathTraversal, "../evil.eml"},
		"absolute":  {makeZip(t, zipFile{name: "/etc/passwd.eml", body: "x"}), limits, RefusedPathTraversal, "/etc/passwd.eml"},
		"backslash": {makeZip(t, zipFile{name: `a\..\..\b.eml`, body: "x"}), limits, RefusedPathTraversal, `a\..\..\b.eml`},
		"symlink":   {makeZip(t, zipFile{name: "INBOX/link.eml", body: "/etc/passwd", mode: fs.ModeSymlink | 0o777}), limits, RefusedSymlink, "INBOX/link.eml"},
		"bomb":      {makeZip(t, zipFile{name: "INBOX/bomb.eml", body: big}), limits, RefusedZipBomb, "INBOX/bomb.eml"},
		"entries": {makeZip(t, zipFile{name: "a.eml", body: "x"}, zipFile{name: "b.eml", body: "x"}, zipFile{name: "c.eml", body: "x"}),
			Limits{MaxEntries: 2, MaxRatio: 100, MaxEntry: 1 << 30}, RefusedTooManyEntries, ""},
		"entry size":      {makeZip(t, zipFile{name: "a.eml", body: strings.Repeat("x", 2000)}), Limits{MaxEntries: 10, MaxRatio: 100, MaxEntry: 1000}, RefusedEntryTooLarge, "a.eml"},
		"compressed mbox": {makeZip(t, zipFile{name: "Inbox.mbox", body: "From x\r\n" + msg("<1@x>", "x", "y"), method: zip.Deflate}), limits, RefusedCompressedMbox, "Inbox.mbox"},
		"empty":           {makeZip(t, zipFile{name: "readme.txt", body: "hello"}), limits, RefusedEmpty, ""},
		"not mail":        {[]byte("%PDF-1.4 hello"), limits, RefusedUnknownFormat, ""},
		"corrupt":         {[]byte("PK\x03\x04 broken"), limits, RefusedCorrupt, ""},
	}
	for name, c := range cases {
		_, _, err := Inspect(bytes.NewReader(c.data), int64(len(c.data)), "x.zip", c.lim)
		var ref *Refusal
		if !errors.As(err, &ref) || ref.Reason != c.reason || ref.Entry != c.entry {
			t.Errorf("%s: %v, want %s on %q", name, err, c.reason, c.entry)
		}
	}
}

func TestStoredMboxInZip(t *testing.T) {
	box := "From a Thu Mar 06 09:00:30 +0000 2025\r\n" + msg("<1@x>", "One", "a\r\n") + "\r\nFrom b Thu Mar 06 09:01:30 +0000 2025\r\n" + msg("<2@x>", "Two", "b\r\n") + "\r\n"
	data := makeZip(t, zipFile{name: "Mail/Inbox.mbox/mbox", body: box}, zipFile{name: "Mail/Clients.mbox", body: box})
	l, folders, err := Inspect(bytes.NewReader(data), int64(len(data)), "x.zip", limits)
	if err != nil || l.Format != FormatMboxZip {
		t.Fatalf("%+v %v", l, err)
	}
	if want := []Folder{{"Inbox", "/", "inbox", 0}, {"Clients", "/", "user", 0}}; !reflect.DeepEqual(folders, want) {
		t.Fatalf("folders %+v", folders)
	}
	var got []Message
	var cursors []Cursor
	err = Scan(bytes.NewReader(data), int64(len(data)), l, MboxRD, Cursor{}, 1<<20, func(m Message) error {
		cursors = append(cursors, m.Next)
		if m.SourceID != "" {
			got = append(got, m)
		}
		return nil
	})
	if err != nil || len(got) != 4 || got[0].Folder != "Inbox" || got[3].Folder != "Clients" {
		t.Fatalf("%+v %v", got, err)
	}
	var resumed []string
	err = Scan(bytes.NewReader(data), int64(len(data)), l, MboxRD, cursors[0], 1<<20, func(m Message) error {
		if m.SourceID != "" {
			resumed = append(resumed, m.SourceID)
		}
		return nil
	})
	if err != nil || !reflect.DeepEqual(resumed, []string{got[1].SourceID, got[2].SourceID, got[3].SourceID}) {
		t.Fatalf("resumed %v %v", resumed, err)
	}
	for _, m := range got {
		again, _, err := Read(bytes.NewReader(data), int64(len(data)), m.SourceID, MboxRD, 1<<20)
		if err != nil || !bytes.Equal(again, m.Raw) {
			t.Fatalf("read %s: %q %v", m.SourceID, again, err)
		}
	}
}

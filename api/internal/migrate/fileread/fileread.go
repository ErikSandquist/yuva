// Package fileread reads the export files a team uploads to move its mail to Yuva: a Google
// Takeout mbox, any mbox, and zips of .eml files, of a Maildir or of mbox files. It reads a file
// through io.ReaderAt a piece at a time and never holds more than one message of it, checks a
// zip before anything is unpacked, and names every message by where it is in the file, so the
// copy reads it again without reading the file from the start.
package fileread

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/productdevbook/yuva/api/internal/migrate"
	"github.com/productdevbook/yuva/api/internal/migrate/imapread"
)

// Formats, as MigrationFileFormat names them.
const (
	FormatTakeout    = "takeout"
	FormatMbox       = "mbox"
	FormatMboxZip    = "mbox_zip"
	FormatEmlZip     = "eml_zip"
	FormatMaildirZip = "maildir_zip"
)

// Refusal reasons, as MigrationFileRefusal names them.
const (
	RefusedUnknownFormat          = "unknown_format"
	RefusedEmpty                  = "empty"
	RefusedCorrupt                = "corrupt"
	RefusedPathTraversal          = "path_traversal"
	RefusedSymlink                = "symlink"
	RefusedTooManyEntries         = "too_many_entries"
	RefusedZipBomb                = "zip_bomb"
	RefusedEntryTooLarge          = "entry_too_large"
	RefusedEncrypted              = "encrypted"
	RefusedUnsupportedCompression = "unsupported_compression"
	RefusedCompressedMbox         = "compressed_mbox"
)

// Refusal is why a file is not read, with the zip entry it is about.
type Refusal struct {
	Reason string
	Entry  string
	Detail string
}

func (r *Refusal) Error() string {
	if r.Entry != "" {
		return r.Reason + ": " + r.Entry + ": " + r.Detail
	}
	return r.Reason + ": " + r.Detail
}

// Limits: MaxEntries and MaxRatio are the zip rules (1,000,000 entries, 100:1); MaxEntry is the
// largest entry unpacked, the server's limit for one file.
type Limits struct {
	MaxEntries int64
	MaxRatio   int64
	MaxEntry   int64
}

// ratioFloor exempts small entries from the ratio rule: a few hundred kilobytes of one repeated
// byte compress far beyond 100:1 and harm nothing.
const ratioFloor = 1 << 20

// Folder is a folder of the file: its name as a mail server would list it ("/" between levels),
// its kind, and for zip entries how many messages it holds.
type Folder struct {
	Name     string
	Delim    string
	Kind     string
	Messages int64
}

// Layout is what Inspect learned of a file that its scan needs again.
type Layout struct {
	Format      string `json:"format"`
	Takeout     bool   `json:"takeout,omitempty"`
	Name        string `json:"name,omitempty"`
	EmlRoot     string `json:"eml_root,omitempty"`
	MaildirRoot string `json:"maildir_root,omitempty"`
	MboxRoot    string `json:"mbox_root,omitempty"`
}

const (
	classNone = iota
	classMaildir
	classEml
	classMbox
)

type classified struct {
	class int
	dir   string
	flags string
}

// classify tells what a zip entry holds by its path: a Maildir message (in cur or new), an .eml
// file, an mbox (X.mbox, or Apple Mail's X.mbox/mbox), or something to leave alone.
func classify(name string) classified {
	name = strings.ReplaceAll(name, `\`, "/")
	if strings.HasSuffix(name, "/") {
		return classified{}
	}
	segs := strings.Split(name, "/")
	base := segs[len(segs)-1]
	if strings.HasPrefix(base, ".") || segs[0] == "__MACOSX" || base == "" {
		return classified{}
	}
	if len(segs) >= 2 && (segs[len(segs)-2] == "cur" || segs[len(segs)-2] == "new") {
		flags := ""
		if i := strings.LastIndex(base, ":2,"); i >= 0 {
			flags = base[i+3:]
		}
		return classified{class: classMaildir, dir: strings.Join(segs[:len(segs)-2], "/"), flags: flags}
	}
	lower := strings.ToLower(base)
	switch {
	case strings.HasSuffix(lower, ".eml"):
		return classified{class: classEml, dir: strings.Join(segs[:len(segs)-1], "/")}
	case strings.HasSuffix(lower, ".mbox"):
		return classified{class: classMbox, dir: strings.Join(segs, "/")[:len(name)-len(".mbox")]}
	case base == "mbox" && len(segs) >= 2 && strings.HasSuffix(strings.ToLower(segs[len(segs)-2]), ".mbox"):
		p := strings.Join(segs[:len(segs)-1], "/")
		return classified{class: classMbox, dir: p[:len(p)-len(".mbox")]}
	}
	return classified{}
}

func unsafePath(name string) bool {
	n := strings.ReplaceAll(name, `\`, "/")
	if strings.HasPrefix(n, "/") || (len(n) >= 2 && n[1] == ':') || strings.ContainsRune(n, 0) {
		return true
	}
	for _, s := range strings.Split(n, "/") {
		if s == ".." {
			return true
		}
	}
	return false
}

func parentDir(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return ""
}

// maildirBase is the Maildir a folder directory belongs to: a Maildir++ folder (.Name) belongs
// to its parent.
func maildirBase(dir string) string {
	leaf := dir[strings.LastIndex(dir, "/")+1:]
	if strings.HasPrefix(leaf, ".") && leaf != "." {
		return parentDir(dir)
	}
	return dir
}

func commonPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	prefix := strings.Split(paths[0], "/")
	if paths[0] == "" {
		return ""
	}
	for _, p := range paths[1:] {
		if p == "" {
			return ""
		}
		segs := strings.Split(p, "/")
		n := 0
		for n < len(prefix) && n < len(segs) && prefix[n] == segs[n] {
			n++
		}
		prefix = prefix[:n]
	}
	return strings.Join(prefix, "/")
}

func rel(p, root string) string {
	switch {
	case root == "":
		return p
	case p == root:
		return ""
	}
	return strings.TrimPrefix(p, root+"/")
}

func orInbox(name string) string {
	if name == "" {
		return "INBOX"
	}
	return name
}

func (l Layout) folderOf(c classified) string {
	switch c.class {
	case classMaildir:
		base := maildirBase(c.dir)
		name := rel(base, l.MaildirRoot)
		if base != c.dir {
			leaf := strings.ReplaceAll(c.dir[strings.LastIndex(c.dir, "/")+1:][1:], ".", "/")
			if name == "" {
				return leaf
			}
			return name + "/" + leaf
		}
		return orInbox(name)
	case classEml:
		return orInbox(rel(c.dir, l.EmlRoot))
	case classMbox:
		return orInbox(rel(c.dir, l.MboxRoot))
	}
	return ""
}

func folderKind(name string, gmail bool) string {
	return migrate.FolderKind(imapread.Folder{Name: name, Delim: "/"}, gmail)
}

// plainName is the folder of a plain mbox file: named for the file, read without a label
// unless the name is a special folder's.
func plainFolder(filename string) Folder {
	name := strings.TrimSpace(strings.TrimSuffix(path.Base(strings.ReplaceAll(filename, `\`, "/")), path.Ext(filename)))
	if name == "" || name == "." || name == "/" {
		name = "Archive"
	}
	kind := folderKind(name, false)
	if kind == migrate.KindUser {
		kind = migrate.KindArchive
	}
	return Folder{Name: name, Delim: "/", Kind: kind}
}

// Inspect reads what kind of file it is and, for a zip, checks every entry before anything is
// unpacked; a file it refuses comes back as a *Refusal.
func Inspect(f io.ReaderAt, size int64, filename string, lim Limits) (Layout, []Folder, error) {
	head := make([]byte, min(size, 64<<10))
	if _, err := f.ReadAt(head, 0); err != nil && !errors.Is(err, io.EOF) {
		return Layout{}, nil, err
	}
	switch {
	case bytes.HasPrefix(head, []byte("PK\x03\x04")), bytes.HasPrefix(head, []byte("PK\x05\x06")):
		return inspectZip(f, size, lim)
	case startsMbox(head):
		if isTakeout(firstHead(head)) {
			return Layout{Format: FormatTakeout, Takeout: true}, []Folder{{Name: GmailAll, Delim: "/", Kind: migrate.KindAll}}, nil
		}
		folder := plainFolder(filename)
		return Layout{Format: FormatMbox, Name: folder.Name}, []Folder{folder}, nil
	}
	return Layout{}, nil, &Refusal{Reason: RefusedUnknownFormat, Detail: "the file is neither an mbox nor a zip"}
}

// firstHead is the header of an mbox's first message.
func firstHead(b []byte) []byte {
	b = bytes.TrimLeft(b, "\r\n")
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		return b[i+1:]
	}
	return nil
}

func inspectZip(f io.ReaderAt, size int64, lim Limits) (Layout, []Folder, error) {
	d, err := readDirectory(f, size)
	if err != nil {
		return Layout{}, nil, &Refusal{Reason: RefusedCorrupt, Detail: "the zip's central directory cannot be read"}
	}
	if d.count > lim.MaxEntries {
		return Layout{}, nil, &Refusal{Reason: RefusedTooManyEntries, Detail: fmt.Sprintf("the zip has %d entries; at most %d are read", d.count, lim.MaxEntries)}
	}
	type dirCount struct {
		c       classified
		n       int64
		takeout bool
	}
	dirs := map[string]*dirCount{}
	var order []string
	var entries, unpacked int64
	counts := [4]int64{}
	takeouts := 0
	err = walkDirectory(f, d, d.start, func(e entry) error {
		entries++
		if entries > lim.MaxEntries {
			return &Refusal{Reason: RefusedTooManyEntries, Detail: fmt.Sprintf("the zip has more than %d entries", lim.MaxEntries)}
		}
		switch {
		case unsafePath(e.Name):
			return &Refusal{Reason: RefusedPathTraversal, Entry: e.Name, Detail: "an entry's path leaves the archive"}
		case e.Symlink:
			return &Refusal{Reason: RefusedSymlink, Entry: e.Name, Detail: "an entry is a symbolic link"}
		case e.Flags&1 == 1:
			return &Refusal{Reason: RefusedEncrypted, Entry: e.Name, Detail: "an entry is encrypted"}
		case e.Size > ratioFloor && e.Size > lim.MaxRatio*max(e.Packed, 1):
			return &Refusal{Reason: RefusedZipBomb, Entry: e.Name, Detail: fmt.Sprintf("an entry is compressed more than %d:1", lim.MaxRatio)}
		case e.Size > lim.MaxEntry:
			return &Refusal{Reason: RefusedEntryTooLarge, Entry: e.Name, Detail: fmt.Sprintf("an entry unpacks to %d bytes, over the server's limit of %d", e.Size, lim.MaxEntry)}
		}
		unpacked += e.Size
		if unpacked > lim.MaxRatio*max(size, 1) && unpacked > ratioFloor {
			return &Refusal{Reason: RefusedZipBomb, Entry: e.Name, Detail: fmt.Sprintf("the entries unpack to more than %d times the file", lim.MaxRatio)}
		}
		c := classify(e.Name)
		if c.class == classNone {
			return nil
		}
		if e.Method != methodStore && e.Method != methodDeflate {
			return &Refusal{Reason: RefusedUnsupportedCompression, Entry: e.Name, Detail: "an entry is neither stored nor deflated"}
		}
		if c.class == classMaildir && strings.Contains(c.flags, "T") {
			return nil
		}
		takeout := false
		if c.class == classMbox {
			if e.Method != methodStore {
				return &Refusal{Reason: RefusedCompressedMbox, Entry: e.Name, Detail: "an mbox in a zip must be stored uncompressed; unzip the archive and upload the mbox file itself"}
			}
			start, err := dataStart(f, e.Offset)
			if err != nil {
				return &Refusal{Reason: RefusedCorrupt, Entry: e.Name, Detail: "an entry cannot be read"}
			}
			h := make([]byte, min(e.Size, 64<<10))
			if _, err := f.ReadAt(h, start); err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			if !startsMbox(h) {
				return nil
			}
			takeout = isTakeout(firstHead(h))
			if takeout {
				takeouts++
			}
		}
		counts[c.class]++
		key := strconv.Itoa(c.class) + "\x00" + c.dir
		if dirs[key] == nil {
			dirs[key] = &dirCount{c: c, takeout: takeout}
			order = append(order, key)
		}
		if c.class != classMbox {
			dirs[key].n++
		}
		return nil
	})
	var ref *Refusal
	if errors.As(err, &ref) {
		return Layout{}, nil, ref
	}
	if errors.Is(err, errCorrupt) {
		return Layout{}, nil, &Refusal{Reason: RefusedCorrupt, Detail: "the zip's central directory cannot be read"}
	}
	if err != nil {
		return Layout{}, nil, err
	}
	l := Layout{}
	var eml, maildir, mbox []string
	for _, k := range order {
		dc := dirs[k]
		switch dc.c.class {
		case classEml:
			eml = append(eml, dc.c.dir)
		case classMaildir:
			maildir = append(maildir, maildirBase(dc.c.dir))
		case classMbox:
			mbox = append(mbox, parentDir(dc.c.dir))
		}
	}
	l.EmlRoot, l.MaildirRoot, l.MboxRoot = commonPrefix(eml), commonPrefix(maildir), commonPrefix(mbox)
	switch {
	case counts[classMaildir] > 0:
		l.Format = FormatMaildirZip
	case counts[classEml] > 0:
		l.Format = FormatEmlZip
	case counts[classMbox] > 0 && int64(takeouts) == counts[classMbox]:
		l.Format = FormatTakeout
	case counts[classMbox] > 0:
		l.Format = FormatMboxZip
	default:
		return Layout{}, nil, &Refusal{Reason: RefusedEmpty, Detail: "the zip holds no .eml file, Maildir or mbox"}
	}
	l.Takeout = takeouts > 0
	byName := map[string]int{}
	var folders []Folder
	for _, k := range order {
		dc := dirs[k]
		name := l.folderOf(dc.c)
		kind := folderKind(name, false)
		if dc.takeout {
			name, kind = GmailAll, migrate.KindAll
		}
		if i, ok := byName[name]; ok {
			folders[i].Messages += dc.n
			continue
		}
		byName[name] = len(folders)
		folders = append(folders, Folder{Name: name, Delim: "/", Kind: kind, Messages: dc.n})
	}
	return l, folders, nil
}

// Cursor is where a scan continues: the central directory record of the zip entry to read next,
// and in an mbox the From_ line of the next message.
type Cursor struct {
	Entry int64 `json:"entry,omitempty"`
	At    int64 `json:"at,omitempty"`
	Done  bool  `json:"done,omitempty"`
}

// Message is one message the scan read.
type Message struct {
	SourceID string
	// Folder is the folder it is read from; empty when nothing reads it (Takeout's spam, trash,
	// drafts and chats).
	Folder string
	// Labels are the other folders it is in (Takeout).
	Labels []string
	// Counted is set when the scan counts it in its folders; Inspect counted zip entries.
	Counted bool
	Thread  string
	// Raw is the message as an IMAP server would send it; nil when larger than the limit.
	Raw      []byte
	Header   []byte
	Parts    []imapread.Part
	Size     int64
	Received time.Time
	// Err tells why a zip entry could not be read; nothing else is set then.
	Err  string
	Next Cursor
}

// Scan reads the file's messages from the cursor on, each at most limit bytes. A message
// without SourceID only moves the cursor past the end of an mbox in a zip.
func Scan(f io.ReaderAt, size int64, l Layout, variant string, from Cursor, limit int64, each func(Message) error) error {
	if !isZip(f) {
		folder := l.Name
		return scanMboxMessages(f, 0, size, from.At, l.Takeout, folder, variant, limit, func(m Message, next int64) error {
			m.Next = Cursor{At: next}
			return each(m)
		})
	}
	d, err := readDirectory(f, size)
	if err != nil {
		return err
	}
	return walkDirectory(f, d, from.Entry, func(e entry) error {
		c := classify(e.Name)
		next := Cursor{Entry: e.Next}
		switch c.class {
		case classNone:
			return nil
		case classMbox:
			start, err := dataStart(f, e.Offset)
			if err != nil {
				return err
			}
			h := make([]byte, min(e.Size, 64<<10))
			if _, err := f.ReadAt(h, start); err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			if !startsMbox(h) {
				return nil
			}
			takeout := isTakeout(firstHead(h))
			resume := start
			if from.Entry == e.At && from.At > start {
				resume = from.At
			}
			err = scanMboxMessages(f, start, start+e.Size, resume, takeout, l.folderOf(c), variant, limit, func(m Message, n int64) error {
				m.Next = Cursor{Entry: e.At, At: n}
				return each(m)
			})
			if err != nil {
				return err
			}
			return each(Message{Next: next})
		}
		if c.class == classMaildir && strings.Contains(c.flags, "T") {
			return nil
		}
		m := Message{SourceID: fmt.Sprintf("zip:%d+%d", e.Offset, e.Size), Folder: l.folderOf(c), Size: e.Size, Received: e.Modified, Next: next}
		if c.class == classMaildir {
			base := e.Name[strings.LastIndexAny(e.Name, `/\`)+1:]
			if i := strings.IndexByte(base, '.'); i > 0 {
				if secs, err := strconv.ParseInt(base[:i], 10, 64); err == nil && secs > 0 {
					m.Received = time.Unix(secs, 0).UTC()
				}
			}
		}
		data, tooLarge, err := openEntry(f, size, e.Offset, e.Size, limit)
		switch {
		case err != nil:
			m.Err = "the entry cannot be unpacked"
		case tooLarge:
			if head, err := entryHead(f, size, e.Offset, e.Size, headBytes); err == nil {
				m.Header = header(unescape(head, ""))
			}
		default:
			m.Raw = unescape(data, "")
			m.Header, m.Parts = header(m.Raw), parts(m.Raw)
		}
		return each(m)
	})
}

func isZip(f io.ReaderAt) bool {
	b := make([]byte, 4)
	_, err := f.ReadAt(b, 0)
	return err == nil && (bytes.Equal(b, []byte("PK\x03\x04")) || bytes.Equal(b, []byte("PK\x05\x06")))
}

func scanMboxMessages(f io.ReaderAt, start, end, from int64, takeout bool, folder, variant string, limit int64, each func(Message, int64) error) error {
	return scanMbox(f, start, end, from, limit, func(mm mboxMessage) error {
		m := Message{SourceID: fmt.Sprintf("file:%d+%d", mm.Offset, mm.Length), Folder: folder, Counted: true, Size: mm.Length, Received: mm.Received}
		if mm.Data != nil {
			m.Raw = unescape(mm.Data, variant)
			m.Header, m.Parts = header(m.Raw), parts(m.Raw)
		} else {
			m.Header = header(unescape(mm.Head, variant))
		}
		if takeout {
			h := readHeader(m.Header)
			labels, inAll := takeoutLabels(h)
			m.Thread = strings.TrimSpace(h.Get("X-GM-THRID"))
			m.Folder, m.Labels = "", labels
			if inAll {
				m.Folder = GmailAll
			}
		}
		m.Header = stripTakeout(m.Header)
		if m.Raw != nil {
			m.Raw = stripTakeout(m.Raw)
		}
		return each(m, mm.Next)
	})
}

// header is a message's header block with the blank line that ends it.
func header(raw []byte) []byte {
	if i := bytes.Index(raw, []byte("\r\n\r\n")); i >= 0 {
		return raw[:i+2]
	}
	return raw
}

// Source ids name where a message is: "file:<offset>+<length>" for an mbox message's bytes,
// "zip:<offset>+<size>" for a zip entry by its local header; a "#<folder>" suffix marks a copy of
// it in another folder.
func parseSource(id string) (kind string, off, n int64, ok bool) {
	if i := strings.IndexByte(id, '#'); i >= 0 {
		id = id[:i]
	}
	kind, rest, found := strings.Cut(id, ":")
	if !found || (kind != "file" && kind != "zip") {
		return "", 0, 0, false
	}
	a, b, found := strings.Cut(rest, "+")
	if !found {
		return "", 0, 0, false
	}
	off, err1 := strconv.ParseInt(a, 10, 64)
	n, err2 := strconv.ParseInt(b, 10, 64)
	if err1 != nil || err2 != nil || off < 0 || n < 0 {
		return "", 0, 0, false
	}
	return kind, off, n, true
}

// IsSource tells a file migration's source id.
func IsSource(id string) bool {
	_, _, _, ok := parseSource(id)
	return ok
}

// ErrUnreadable is a message that cannot be read back from the file.
var ErrUnreadable = errors.New("the message cannot be read from the file")

// Read reads one message again for the copy, as the scan read it; nil and true when it is
// larger than limit.
func Read(f io.ReaderAt, size int64, id, variant string, limit int64) ([]byte, bool, error) {
	kind, off, n, ok := parseSource(id)
	if !ok {
		return nil, false, ErrUnreadable
	}
	if n > limit {
		return nil, true, nil
	}
	if kind == "zip" {
		data, tooLarge, err := openEntry(f, size, off, n, limit)
		if errors.Is(err, errCorrupt) {
			return nil, false, ErrUnreadable
		}
		if err != nil || tooLarge {
			return nil, tooLarge, err
		}
		return unescape(data, ""), false, nil
	}
	if off+n > size {
		return nil, false, ErrUnreadable
	}
	buf := make([]byte, n)
	if _, err := f.ReadAt(buf, off); err != nil && !(errors.Is(err, io.EOF) && int64(len(buf)) == n) {
		return nil, false, err
	}
	return stripTakeout(unescape(buf, variant)), false, nil
}

// stripTakeout drops the X-GM-THRID and X-Gmail-Labels fields Google Takeout puts at the top
// of every message: without them the message is the one Gmail's IMAP server sends, so it gets
// the same key when it has no Message-ID and is stored as it was received.
func stripTakeout(raw []byte) []byte {
	for {
		i := bytes.IndexByte(raw, ':')
		if i < 0 {
			return raw
		}
		name := strings.ToLower(string(raw[:i]))
		if name != "x-gm-thrid" && name != "x-gmail-labels" {
			return raw
		}
		end := 0
		for {
			j := bytes.Index(raw[end:], []byte("\r\n"))
			if j < 0 {
				return raw
			}
			end += j + 2
			if end >= len(raw) || (raw[end] != ' ' && raw[end] != '\t') {
				break
			}
		}
		raw = raw[end:]
	}
}

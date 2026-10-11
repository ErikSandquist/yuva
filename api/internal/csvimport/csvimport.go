// Package csvimport reads subscriber files as spreadsheets export them: it finds the text
// encoding and the column separator and splits the rows per RFC 4180.
package csvimport

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

const (
	UTF8        = "utf-8"
	UTF16LE     = "utf-16le"
	UTF16BE     = "utf-16be"
	Windows1254 = "windows-1254"
	Windows1252 = "windows-1252"

	MaxColumns = 100
)

var (
	ErrEmpty    = errors.New("the file has no header row")
	ErrColumns  = fmt.Errorf("the file has more than %d columns", MaxColumns)
	ErrTooLarge = errors.New("the file has too many rows")
)

// turkishBytes are the Windows-1254 positions of Ğ İ Ş ğ ı ş; Windows-1252 has Icelandic letters
// there and is otherwise the same in the printable range.
var turkishBytes = []byte{0xD0, 0xDD, 0xDE, 0xF0, 0xFD, 0xFE}

// LooksLikeText refuses what is plainly not a CSV file: zip containers (xlsx, ods), old Office
// files and anything with NUL bytes that is not UTF-16 with a byte order mark.
func LooksLikeText(data []byte) bool {
	if bytes.HasPrefix(data, []byte{0xFF, 0xFE}) || bytes.HasPrefix(data, []byte{0xFE, 0xFF}) {
		return true
	}
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) || bytes.HasPrefix(data, []byte{0xD0, 0xCF, 0x11, 0xE0}) {
		return false
	}
	head := data[:min(len(data), 64<<10)]
	return bytes.IndexByte(head, 0) < 0
}

// Decode returns the file as UTF-8 and the encoding it was read as: UTF-8 (a BOM is dropped),
// UTF-16 with a BOM, else Windows-1254 when bytes that are not UTF-8 include Turkish letters,
// else Windows-1252.
func Decode(data []byte) (string, string, error) {
	var enc encoding.Encoding
	name := ""
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return string(data[3:]), UTF8, nil
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		enc, name = unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM), UTF16LE
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		enc, name = unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM), UTF16BE
	case utf8.Valid(data):
		return string(data), UTF8, nil
	case containsAnyByte(data, turkishBytes):
		enc, name = charmap.Windows1254, Windows1254
	default:
		enc, name = charmap.Windows1252, Windows1252
	}
	out, err := enc.NewDecoder().Bytes(data)
	if err != nil {
		return "", name, err
	}
	return strings.TrimPrefix(string(out), "\uFEFF"), name, nil
}

func containsAnyByte(data, set []byte) bool {
	for _, b := range data {
		if bytes.IndexByte(set, b) >= 0 {
			return true
		}
	}
	return false
}

// Delimiter picks `,`, `;` or tab: whichever occurs most often outside quotes in the header row,
// `,` when none does.
func Delimiter(text string) rune {
	counts := map[rune]int{}
	quoted := false
	for _, r := range text {
		if r == '"' {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		if r == '\n' || r == '\r' {
			break
		}
		if r == ',' || r == ';' || r == '\t' {
			counts[r]++
		}
	}
	best := ','
	for _, r := range []rune{';', '\t'} {
		if counts[r] > counts[best] {
			best = r
		}
	}
	return best
}

// Row is one data row and the file line it starts on (the header is line 1).
type Row struct {
	Line   int
	Fields []string
}

// File is a parsed file: its header and data rows, rows whose cells are all empty left out.
type File struct {
	Encoding  string
	Delimiter rune
	Header    []string
	Rows      []Row
}

// Parse decodes and splits data. More than maxRows data rows is ErrTooLarge; a quote that breaks
// RFC 4180 is an error naming its line.
func Parse(data []byte, maxRows int) (File, error) {
	text, enc, err := Decode(data)
	if err != nil {
		return File{}, fmt.Errorf("the file cannot be read as %s", enc)
	}
	f := File{Encoding: enc, Delimiter: Delimiter(text)}
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = f.Delimiter
	r.FieldsPerRecord = -1
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var pe *csv.ParseError
			if errors.As(err, &pe) {
				return f, fmt.Errorf("line %d: %v", pe.StartLine, pe.Err)
			}
			return f, err
		}
		if f.Header == nil {
			if len(rec) > MaxColumns {
				return f, ErrColumns
			}
			f.Header = make([]string, len(rec))
			for i, h := range rec {
				f.Header[i] = strings.TrimSpace(h)
			}
			continue
		}
		if blank(rec) {
			continue
		}
		if len(f.Rows) >= maxRows {
			return f, ErrTooLarge
		}
		line, _ := r.FieldPos(0)
		f.Rows = append(f.Rows, Row{Line: line, Fields: rec})
	}
	if f.Header == nil || blank(f.Header) {
		return f, ErrEmpty
	}
	return f, nil
}

func blank(rec []string) bool {
	for _, v := range rec {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

// Field names a column may hold.
const (
	FieldEmail      = "email"
	FieldName       = "name"
	FieldFirstName  = "first_name"
	FieldLastName   = "last_name"
	FieldLocale     = "locale"
	FieldExternalID = "external_id"
	FieldAttribute  = "attribute"
	FieldSkip       = "skip"
)

var headerFields = map[string]string{
	"email": FieldEmail, "e-mail": FieldEmail, "e_mail": FieldEmail, "mail": FieldEmail, "email address": FieldEmail,
	"e-mail address": FieldEmail, "e-posta": FieldEmail, "eposta": FieldEmail, "e-posta adresi": FieldEmail,
	"name": FieldName, "full name": FieldName, "full_name": FieldName, "ad": FieldName, "ad soyad": FieldName,
	"adı soyadı": FieldName, "isim": FieldName,
	"first_name": FieldFirstName, "first name": FieldFirstName, "firstname": FieldFirstName, "given name": FieldFirstName,
	"last_name": FieldLastName, "last name": FieldLastName, "lastname": FieldLastName, "surname": FieldLastName,
	"family name": FieldLastName, "soyad": FieldLastName, "soyadı": FieldLastName,
	"locale": FieldLocale, "language": FieldLocale, "lang": FieldLocale, "dil": FieldLocale,
	"external_id": FieldExternalID, "external id": FieldExternalID, "user_id": FieldExternalID, "user id": FieldExternalID,
}

// GuessField maps a header to a field; ok is false for headers it does not know.
func GuessField(header string) (string, bool) {
	f, ok := headerFields[strings.ToLower(strings.TrimSpace(header))]
	return f, ok
}

var asciiLetters = map[rune]rune{'ç': 'c', 'ğ': 'g', 'ı': 'i', 'ö': 'o', 'ş': 's', 'ü': 'u', 'ä': 'a', 'ß': 's', 'é': 'e', 'è': 'e', 'á': 'a', 'à': 'a', 'â': 'a', 'î': 'i', 'û': 'u', 'ñ': 'n'}

// AttributeKey turns a header into an attribute key (letters, digits and `_`, at most 64
// characters); empty when nothing is left.
func AttributeKey(header string) string {
	var b strings.Builder
	under := false
	for _, r := range strings.ToLower(strings.TrimSpace(header)) {
		if a, ok := asciiLetters[r]; ok {
			r = a
		}
		switch {
		case r < utf8.RuneSelf && (r >= 'a' && r <= 'z' || r >= '0' && r <= '9'):
			b.WriteRune(r)
			under = false
		case b.Len() > 0 && !under:
			b.WriteByte('_')
			under = true
		}
	}
	return strings.TrimRight(b.String()[:min(b.Len(), 64)], "_")
}

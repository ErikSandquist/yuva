package fileread

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"time"
)

// Mbox variants: how lines that start with "From " were escaped.
const (
	MboxRD = "mboxrd"
	MboxO  = "mboxo"
)

var fromLine = []byte("From ")

// fromLineLayouts are the dates mbox writers put after the sender on a From_ line: Google
// Takeout's with a zone, asctime's without, and an RFC 5322 date.
var fromLineLayouts = []string{
	"Mon Jan 2 15:04:05 -0700 2006", "Mon Jan 2 15:04:05 2006", "Mon Jan 2 15:04:05 MST 2006",
	"Mon, 2 Jan 2006 15:04:05 -0700", "Mon Jan 2 15:04 2006",
}

// fromLineTime reads the date of a From_ line, the time the message was received; zero when
// it has none.
func fromLineTime(line []byte) time.Time {
	fields := strings.Fields(string(line))
	if len(fields) < 3 {
		return time.Time{}
	}
	v := strings.Join(fields[2:], " ")
	for _, l := range fromLineLayouts {
		if t, err := time.Parse(l, v); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// mboxMessage is one message of an mbox: its bytes on disk from Offset for Length, after its
// From_ line and without the blank line that separates it from the next.
type mboxMessage struct {
	Offset   int64
	Length   int64
	Received time.Time
	Next     int64
	// Head is the start of the message on disk, at most headBytes, for its header.
	Head []byte
	// Data is the whole message on disk, nil when it is longer than the limit.
	Data []byte
}

const headBytes = 256 << 10

// startsMbox reports whether the bytes begin, after blank lines, with a From_ line.
func startsMbox(b []byte) bool {
	b = bytes.TrimLeft(b, "\r\n")
	return bytes.HasPrefix(b, fromLine)
}

// scanMbox reads the messages of the mbox in [start, end) from the From_ line at from on; each
// holds at most limit bytes of data.
func scanMbox(f io.ReaderAt, start, end, from int64, limit int64, each func(mboxMessage) error) error {
	if from < start {
		from = start
	}
	br := bufio.NewReaderSize(io.NewSectionReader(f, from, end-from), 64<<10)
	pos := from
	lineStart := true
	var cur *mboxMessage
	var data []byte
	tooLarge := false
	finish := func(next int64) error {
		if cur == nil {
			return nil
		}
		length := next - cur.Offset
		if !tooLarge {
			trimmed := trimSeparator(data)
			length = int64(len(trimmed))
			cur.Data = trimmed
		} else if length > 0 {
			length -= separatorAt(f, next)
		}
		cur.Length, cur.Next = length, next
		if cur.Data != nil {
			cur.Head = cur.Data[:min(len(cur.Data), headBytes)]
		}
		m := *cur
		cur, data, tooLarge = nil, nil, false
		return each(m)
	}
	for {
		chunk, err := br.ReadSlice('\n')
		if len(chunk) > 0 {
			isFrom := lineStart && bytes.HasPrefix(chunk, fromLine)
			if isFrom {
				line := append([]byte{}, chunk...)
				for errors.Is(err, bufio.ErrBufferFull) {
					var more []byte
					more, err = br.ReadSlice('\n')
					line = append(line, more...)
				}
				if err := finish(pos); err != nil {
					return err
				}
				pos += int64(len(line))
				cur = &mboxMessage{Offset: pos, Received: fromLineTime(line)}
				lineStart = true
				if err != nil && !errors.Is(err, io.EOF) {
					return err
				}
				if errors.Is(err, io.EOF) {
					break
				}
				continue
			}
			if cur == nil {
				if len(bytes.TrimRight(chunk, "\r\n")) > 0 {
					return errNotMbox
				}
			} else if !tooLarge {
				if int64(len(data)+len(chunk)) > limit+2 {
					tooLarge = true
					head := data[:min(len(data), headBytes)]
					if len(head) < headBytes {
						head = append(head, chunk[:min(len(chunk), headBytes-len(head))]...)
					}
					cur.Head = append([]byte{}, head...)
					data = nil
				} else {
					data = append(data, chunk...)
				}
			}
			pos += int64(len(chunk))
			lineStart = chunk[len(chunk)-1] == '\n'
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) {
			return err
		}
	}
	return finish(pos)
}

var errNotMbox = errors.New("not an mbox")

// trimSeparator drops the blank line an mbox writer puts after each message.
func trimSeparator(b []byte) []byte {
	switch {
	case bytes.HasSuffix(b, []byte("\r\n\r\n")):
		return b[:len(b)-2]
	case bytes.HasSuffix(b, []byte("\n\n")):
		return b[:len(b)-1]
	}
	return b
}

// separatorAt is the length of the blank line just before offset next, read from the file.
func separatorAt(f io.ReaderAt, next int64) int64 {
	b := make([]byte, 4)
	if next < 4 {
		return 0
	}
	if _, err := f.ReadAt(b, next-4); err != nil {
		return 0
	}
	switch {
	case bytes.Equal(b, []byte("\r\n\r\n")):
		return 2
	case bytes.HasSuffix(b, []byte("\n\n")):
		return 1
	}
	return 0
}

// unescape takes one ">" off the lines the mbox variant escaped, and ends every line with CRLF
// as an IMAP server sends it, so a message read from a file is byte for byte the one read over
// IMAP and gets the same key when it has no Message-ID.
func unescape(b []byte, variant string) []byte {
	out := make([]byte, 0, len(b)+len(b)/40)
	for len(b) > 0 {
		i := bytes.IndexByte(b, '\n')
		var line []byte
		if i < 0 {
			line, b = b, nil
		} else {
			line, b = b[:i], b[i+1:]
		}
		if variant != "" && len(line) > 5 && line[0] == '>' {
			n := 0
			for n < len(line) && line[n] == '>' {
				n++
			}
			if bytes.HasPrefix(line[n:], fromLine) && (variant == MboxRD || n == 1) {
				line = line[1:]
			}
		}
		line = bytes.TrimSuffix(line, []byte("\r"))
		out = append(out, line...)
		if i >= 0 {
			out = append(out, '\r', '\n')
		}
	}
	return out
}

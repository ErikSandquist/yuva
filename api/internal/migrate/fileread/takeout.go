package fileread

import (
	"bufio"
	"bytes"
	"mime"
	"net/textproto"
	"strings"
)

// Gmail's folders as its IMAP server names them, so a Takeout file and an IMAP migration of the
// same mailbox see the same folders and labels.
const (
	GmailAll    = "[Gmail]/All Mail"
	gmailPrefix = "[Gmail]/"
)

var takeoutSystem = map[string]string{
	"Inbox": "INBOX", "Sent": "[Gmail]/Sent Mail", "Spam": "[Gmail]/Spam", "Trash": "[Gmail]/Trash", "Drafts": "[Gmail]/Drafts",
	"Important": "[Gmail]/Important", "Starred": "[Gmail]/Starred", "Chat": "[Gmail]/Chats", "Snoozed": "[Gmail]/Snoozed",
	"Scheduled": "[Gmail]/Scheduled",
}

// takeoutState labels say something about a message, not where it is; they are no folder.
var takeoutState = map[string]bool{"Opened": true, "Unread": true, "Archived": true}

// takeoutUnread are the labels whose messages Gmail's All Mail does not hold, or that are
// never imported.
var takeoutUnread = map[string]bool{"Spam": true, "Trash": true, "Chat": true, "Drafts": true}

func readHeader(head []byte) textproto.MIMEHeader {
	end := bytes.Index(head, []byte("\r\n\r\n"))
	if end < 0 {
		end = bytes.Index(head, []byte("\n\n"))
	}
	if end >= 0 {
		head = head[:end]
	}
	h, _ := textproto.NewReader(bufio.NewReader(bytes.NewReader(append(append([]byte{}, head...), "\r\n\r\n"...)))).ReadMIMEHeader()
	return h
}

// isTakeout tells a Google Takeout mbox by the headers Takeout adds to its first message.
func isTakeout(head []byte) bool {
	h := readHeader(head)
	return h.Get("X-Gmail-Labels") != "" || h.Get("X-GM-THRID") != ""
}

// takeoutLabels are the Gmail labels of a message: the folders it is in, and whether All Mail
// holds it.
func takeoutLabels(h textproto.MIMEHeader) (folders []string, inAll bool) {
	inAll = true
	var dec mime.WordDecoder
	raw := h.Get("X-Gmail-Labels")
	if d, err := dec.DecodeHeader(raw); err == nil {
		raw = d
	}
	for _, l := range splitLabels(raw) {
		switch {
		case l == "" || takeoutState[l]:
			continue
		case takeoutUnread[l]:
			inAll = false
		}
		name := l
		if n, ok := takeoutSystem[l]; ok {
			name = n
		} else if strings.HasPrefix(l, "Category ") {
			name = gmailPrefix + l
		}
		folders = append(folders, name)
	}
	return folders, inAll
}

// splitLabels splits Takeout's comma-separated labels; a label with a comma is quoted.
func splitLabels(v string) []string {
	var out []string
	var cur strings.Builder
	quoted := false
	for _, r := range v {
		switch {
		case r == '"':
			quoted = !quoted
		case r == ',' && !quoted:
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	return append(out, strings.TrimSpace(cur.String()))
}

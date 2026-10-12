package fileread

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"strings"

	"github.com/productdevbook/yuva/api/internal/migrate/imapread"
)

const maxPartDepth = 8

// parts lists a message's leaf parts as an IMAP server's BODYSTRUCTURE would: type, encoded
// size, transfer encoding and file name.
func parts(raw []byte) []imapread.Part {
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	var out []imapread.Part
	walk(textproto.MIMEHeader(m.Header), m.Body, 0, &out)
	return out
}

func walk(h textproto.MIMEHeader, body io.Reader, depth int, out *[]imapread.Part) {
	ct, params, err := mime.ParseMediaType(h.Get("Content-Type"))
	if err != nil {
		ct, params = "text/plain", nil
	}
	if strings.HasPrefix(ct, "multipart/") && params["boundary"] != "" && depth < maxPartDepth {
		mr := multipart.NewReader(body, params["boundary"])
		for {
			p, err := mr.NextRawPart()
			if errors.Is(err, io.EOF) || err != nil {
				return
			}
			walk(p.Header, p, depth+1, out)
		}
	}
	n, _ := io.Copy(io.Discard, body)
	part := imapread.Part{Type: ct, Size: n, Encoding: strings.ToLower(strings.TrimSpace(h.Get("Content-Transfer-Encoding")))}
	if _, dp, err := mime.ParseMediaType(h.Get("Content-Disposition")); err == nil {
		part.Filename = dp["filename"]
		part.Inline = strings.HasPrefix(strings.ToLower(h.Get("Content-Disposition")), "inline")
	}
	if part.Filename == "" {
		part.Filename = params["name"]
	}
	*out = append(*out, part)
}

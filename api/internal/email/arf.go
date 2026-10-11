package email

import (
	"bufio"
	"bytes"
	"mime"
	"net/textproto"
	"strings"

	"github.com/jhillyerd/enmime/v2"
)

// Feedback is an abuse report in the Abuse Reporting Format (RFC 5965): a mailbox provider telling
// the sender that a recipient marked a message as spam.
type Feedback struct {
	Type              string
	OriginalRcptTo    string
	OriginalMessageID string
}

func parseFeedback(root *enmime.Part) *Feedback {
	report := root.DepthMatchFirst(func(p *enmime.Part) bool {
		if p.ContentType != "multipart/report" {
			return false
		}
		_, params, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		return err == nil && strings.EqualFold(params["report-type"], "feedback-report")
	})
	if report == nil {
		return nil
	}
	f := &Feedback{}
	for p := report.FirstChild; p != nil; p = p.NextSibling {
		switch p.ContentType {
		case "message/feedback-report":
			r := textproto.NewReader(bufio.NewReader(bytes.NewReader(append(bytes.TrimLeft(p.Content, "\r\n"), "\r\n\r\n"...))))
			h, _ := r.ReadMIMEHeader()
			f.Type = strings.ToLower(strings.TrimSpace(h.Get("Feedback-Type")))
			f.OriginalRcptTo = strings.ToLower(strings.Trim(strings.TrimSpace(h.Get("Original-Rcpt-To")), "<>"))
		case "message/rfc822", "text/rfc822-headers", "message/rfc822-headers", "message/global", "message/global-headers":
			f.OriginalMessageID = originalMessageID(p)
		}
	}
	if f.Type == "" {
		return nil
	}
	return f
}

// Complaint reports a feedback report that means the recipient called the message spam.
func (f *Feedback) Complaint() bool {
	switch f.Type {
	case "abuse", "fraud", "virus", "other":
		return true
	}
	return false
}

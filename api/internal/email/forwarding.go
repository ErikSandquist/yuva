package email

import (
	"regexp"
	"strings"
)

// MigrationProbeHeader carries a migration's test message: the migration id and an HMAC of it.
const MigrationProbeHeader = "Yuva-Probe"

// GoogleForwardingSender sends Gmail's forwarding confirmations.
const GoogleForwardingSender = "forwarding-noreply@google.com"

// ForwardingConfirmation is what a provider's forwarding confirmation asks the mailbox owner to
// do: enter Code, or open Link, for the mailbox From.
type ForwardingConfirmation struct {
	Code string
	Link string
	From string
}

var (
	googleSubjectCode = regexp.MustCompile(`^\(#([0-9]{6,12})\)\s`)
	googleBodyCode    = regexp.MustCompile(`(?m)^\s*Confirmation code:\s*([0-9]{6,12})\s*$`)
	googleLink        = regexp.MustCompile(`https://mail(?:-settings)?\.google\.com/mail/(?:u/[0-9]+/)?vf-[A-Za-z0-9%_.~\-\[\]]{8,500}`)
	googleRequester   = regexp.MustCompile(`(?i)\bReceive Mail from ([^\s<>()]+@[^\s<>()]+)\s*$`)
)

// GoogleForwardingConfirmation reads Gmail's forwarding confirmation; ok is false for anything
// else, including a message that only looks like one.
func GoogleForwardingConfirmation(m *Message) (ForwardingConfirmation, bool) {
	if m.From.Email != GoogleForwardingSender {
		return ForwardingConfirmation{}, false
	}
	var out ForwardingConfirmation
	if c := googleSubjectCode.FindStringSubmatch(m.Subject); c != nil {
		out.Code = c[1]
	}
	if c := googleBodyCode.FindStringSubmatch(m.Text); c != nil {
		if out.Code != "" && out.Code != c[1] {
			return ForwardingConfirmation{}, false
		}
		out.Code = c[1]
	}
	if out.Code == "" {
		return ForwardingConfirmation{}, false
	}
	out.Link = googleLink.FindString(m.Text)
	if r := googleRequester.FindStringSubmatch(m.Subject); r != nil {
		out.From = strings.ToLower(r[1])
	}
	return out, true
}

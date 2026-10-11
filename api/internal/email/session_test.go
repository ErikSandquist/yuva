package email_test

import (
	"context"
	"errors"
	"net"
	"net/textproto"
	"os"
	"strings"
	"testing"

	"github.com/productdevbook/yuva/api/internal/email"
)

func TestParseFeedbackReport(t *testing.T) {
	raw, err := os.ReadFile("testdata/arf_abuse.eml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := email.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	f := m.Feedback
	if f == nil || !f.Complaint() || f.Type != "abuse" || f.OriginalRcptTo != "reader@example.org" ||
		f.OriginalMessageID != "abcdefghijklmnopqrstuvwx.abcdefghijklmnop@example.com" || !m.Auto || m.Bounce != nil {
		t.Fatalf("feedback: %+v auto=%v", f, m.Auto)
	}
}

func TestSMTPReplies(t *testing.T) {
	refusal := func(code int, msg string) error {
		return &email.PermanentError{Err: &textproto.Error{Code: code, Msg: msg}}
	}
	cases := []struct {
		err      error
		bad      bool
		enhanced string
	}{
		{refusal(550, "5.1.1 <a@example.org>: Recipient address rejected: User unknown"), true, "5.1.1"},
		{refusal(550, "5.2.1 The email account that you tried to reach is disabled"), true, "5.2.1"},
		{refusal(552, "5.2.2 Mailbox full"), false, "5.2.2"},
		{refusal(554, "5.7.1 Message rejected as spam"), false, "5.7.1"},
		{refusal(550, "Relaying denied"), false, ""},
		{&textproto.Error{Code: 451, Msg: "4.1.1 try later"}, false, "4.1.1"},
		{errors.New("connection reset"), false, ""},
	}
	for _, c := range cases {
		_, enhanced, _ := email.Reply(c.err)
		if email.BadMailbox(c.err) != c.bad || enhanced != c.enhanced {
			t.Errorf("%v: bad=%v enhanced=%q", c.err, email.BadMailbox(c.err), enhanced)
		}
	}
}

// fakeSMTP is a minimal SMTP server: it counts connections and messages and refuses RCPT to
// addresses starting with "bad".
type fakeSMTP struct {
	conns, messages, resets int
}

func (f *fakeSMTP) serve(t *testing.T, l net.Listener) {
	for {
		c, err := l.Accept()
		if err != nil {
			return
		}
		f.conns++
		go func(c net.Conn) {
			defer c.Close()
			tp := textproto.NewConn(c)
			_ = tp.PrintfLine("220 fake ESMTP")
			for {
				line, err := tp.ReadLine()
				if err != nil {
					return
				}
				cmd := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
				switch {
				case cmd == "EHLO" || cmd == "HELO":
					_ = tp.PrintfLine("250 fake")
				case cmd == "MAIL":
					_ = tp.PrintfLine("250 2.1.0 ok")
				case cmd == "RCPT" && strings.Contains(strings.ToLower(line), "<bad"):
					_ = tp.PrintfLine("550 5.1.1 user unknown")
				case cmd == "RCPT":
					_ = tp.PrintfLine("250 2.1.5 ok")
				case cmd == "DATA":
					_ = tp.PrintfLine("354 go on")
					if _, err := tp.ReadDotBytes(); err != nil {
						return
					}
					f.messages++
					_ = tp.PrintfLine("250 2.0.0 queued")
				case cmd == "RSET":
					f.resets++
					_ = tp.PrintfLine("250 2.0.0 reset")
				case cmd == "QUIT":
					_ = tp.PrintfLine("221 bye")
					return
				default:
					_ = tp.PrintfLine("502 5.5.2 what")
				}
			}
		}(c)
	}
}

func TestSessionReusesOneConnection(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	f := &fakeSMTP{}
	go f.serve(t, l)
	port := l.Addr().(*net.TCPAddr).Port
	ctx := context.Background()
	cfg := email.SMTPConfig{Host: "127.0.0.1", Port: port, TLS: "none"}
	if _, err := email.OpenSession(ctx, email.SMTPSender{}, cfg); err == nil {
		t.Fatal("a loopback server was allowed without AllowPrivate")
	}
	s, err := email.OpenSession(ctx, email.SMTPSender{AllowPrivate: true}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("Subject: hi\r\n\r\nhello\r\n")
	if err := s.Send(ctx, "news@example.com", []string{"a@example.org"}, msg); err != nil {
		t.Fatal(err)
	}
	err = s.Send(ctx, "news@example.com", []string{"bad@example.org"}, msg)
	if !email.BadMailbox(err) || !email.IsPermanent(err) {
		t.Fatalf("refused recipient: %v", err)
	}
	if err := s.Send(ctx, "news@example.com", []string{"b@example.org"}, msg); err != nil {
		t.Fatalf("after a refusal: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if f.conns != 1 || f.messages != 2 || f.resets != 1 {
		t.Fatalf("conns=%d messages=%d resets=%d", f.conns, f.messages, f.resets)
	}
}

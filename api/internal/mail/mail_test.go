package mail_test

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/fs"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	netmail "net/mail"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/productdevbook/yuva/api/internal/config"
	"github.com/productdevbook/yuva/api/internal/mail"
)

func TestContinuitySubjectByLocale(t *testing.T) {
	for locale, want := range map[string]string{
		"en": "Acme: new reply", "tr": "Acme: yeni yanıt", "tr-TR": "Acme: yeni yanıt", "TR_tr": "Acme: yeni yanıt",
		"de": "Acme: new reply", "": "Acme: new reply",
	} {
		m, err := mail.Render("continuity", locale, map[string]any{"Inbox": "Acme"})
		if err != nil {
			t.Fatal(err)
		}
		if m.Subject != want {
			t.Errorf("%q: subject %q, want %q", locale, m.Subject, want)
		}
	}
}

func samples(inbox, text string) map[string]map[string]any {
	return map[string]map[string]any{
		"sign_in_code":   {"Code": "482913", "Minutes": 10},
		"sign_in_locked": {"Failures": 10, "Hours": 24},
		"invite": {"Workspace": inbox, "Inviter": "Ayşe Yılmaz", "Role": "admin",
			"URL": "https://yuva.example.com", "Days": 7},
		"confirm_email": {"Inbox": inbox, "URL": "https://yuva.example.com/email/confirm?token=abc", "Hours": 24},
		"notification": {
			"Inbox": inbox, "Contact": "Jane Doe", "Subject": "Invoice question",
			"Messages": []string{text, "Can you also send a copy to accounting?"}, "More": 2,
			"Mentions": []map[string]string{{"By": "Mehmet", "Text": "@you can you take this one?"}},
			"Assigned": true, "AssignedBy": "Mehmet", "URL": "https://yuva.example.com/c/1",
			"SettingsURL": "https://yuva.example.com/settings/notifications", "Minutes": 15,
		},
		"discussion_digest": {
			"Inbox": inbox, "Title": "How do I export my data?",
			"Answer": map[string]string{"Author": "Support team", "Text": "Open Settings, then Export.\nThe file arrives by e-mail."},
			"Posts":  []map[string]string{{"Author": "Jane Doe", "Text": text}, {"Author": "A reader", "Text": "Same question here."}},
			"More":   3, "URL": "https://docs.example.com/export#yuva-discussion-1",
			"UnsubscribeURL": "https://yuva.example.com/discussions/unsubscribe/tok",
			"SettingsURL":    "https://yuva.example.com/account",
		},
		"rating_request": {"Inbox": inbox, "GoodURL": "https://yuva.example.com/rate/tok?rating=good",
			"BadURL": "https://yuva.example.com/rate/tok?rating=bad"},
	}
}

var htmlMails = []string{"sign_in_code", "sign_in_locked", "invite", "confirm_email", "notification", "discussion_digest", "rating_request"}

func TestEveryNonPushTemplateHasHTML(t *testing.T) {
	names, err := fs.Glob(os.DirFS("templates/en"), "*.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		name := strings.TrimSuffix(n, ".txt")
		if strings.HasPrefix(name, "push_") || name == "continuity" {
			continue
		}
		if !slices.Contains(htmlMails, name) {
			t.Errorf("%s has no HTML part in this test", name)
		}
	}
}

func TestRenderEveryLocale(t *testing.T) {
	data := samples("Acme Support", "Hello, my invoice is wrong.")
	for _, name := range htmlMails {
		for _, locale := range []string{"en", "tr", "de"} {
			for _, b := range []mail.Brand{{}, {Name: "Acme Support", Color: "#0e7490"}} {
				m, err := mail.RenderAs(b, name, locale, data[name])
				if err != nil {
					t.Fatalf("%s/%s: %v", locale, name, err)
				}
				for part, s := range map[string]string{"subject": m.Subject, "text": m.Text, "html": m.HTML} {
					if strings.TrimSpace(s) == "" || strings.Contains(s, "<no value>") || strings.Contains(s, "ZgotmplZ") {
						t.Errorf("%s/%s %s: %q", locale, name, part, s)
					}
				}
				if !strings.Contains(m.HTML, `<html lang="`+locale+`">`) {
					t.Errorf("%s/%s: html not in its locale", locale, name)
				}
				if locale != "en" {
					en, _ := mail.RenderAs(b, name, "en", data[name])
					if en.Subject == m.Subject || en.Text == m.Text {
						t.Errorf("%s/%s: text falls back to English", locale, name)
					}
				}
				checkHTML(t, locale+"/"+name, m.HTML)
			}
		}
	}
}

func checkHTML(t *testing.T, label, doc string) {
	t.Helper()
	z := html.NewTokenizer(strings.NewReader(doc))
	var open []string
	void := map[string]bool{"meta": true, "br": true, "img": true, "hr": true, "link": true}
	for {
		switch z.Next() {
		case html.ErrorToken:
			if z.Err() != io.EOF {
				t.Errorf("%s: %v", label, z.Err())
			}
			if len(open) != 0 {
				t.Errorf("%s: unclosed %v", label, open)
			}
			return
		case html.StartTagToken:
			name, _ := z.TagName()
			if !void[string(name)] {
				open = append(open, string(name))
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			if len(open) == 0 || open[len(open)-1] != string(name) {
				t.Errorf("%s: </%s> closes %v", label, name, open)
				return
			}
			open = open[:len(open)-1]
		}
	}
}

func TestHTMLEscapesInboxNameAndMessages(t *testing.T) {
	evil := `<script>alert(1)</script>"><img src=x onerror=alert(1)>`
	data := samples(evil, evil)
	b := mail.Brand{Name: evil, Color: `red;background:url(javascript:alert(1))`}
	for _, name := range htmlMails {
		for _, locale := range []string{"en", "tr", "de"} {
			m, err := mail.RenderAs(b, name, locale, data[name])
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(m.HTML, "<script") || strings.Contains(m.HTML, "<img") || strings.Contains(m.HTML, "javascript:") {
				t.Errorf("%s/%s: unescaped input in html", locale, name)
			}
			if name != "sign_in_code" && name != "sign_in_locked" && !strings.Contains(m.HTML, "&lt;script&gt;") {
				t.Errorf("%s/%s: escaped input missing", locale, name)
			}
			checkHTML(t, locale+"/"+name, m.HTML)
		}
	}
}

func TestInboxBrandReplacesYuva(t *testing.T) {
	m, err := mail.RenderAs(mail.Brand{Name: "Acme Support", Color: "#FFEE00"}, "rating_request", "en", samples("Acme Support", "")["rating_request"])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(m.HTML, "Yuva") {
		t.Error("an inbox's mail names Yuva")
	}
	if !strings.Contains(m.HTML, "#ffee00") || !strings.Contains(m.HTML, "color:#111111") {
		t.Error("the inbox colour or its readable text colour is missing")
	}
	y, _ := mail.Render("sign_in_code", "en", samples("", "")["sign_in_code"])
	if !strings.Contains(y.HTML, ">Yuva</span>") {
		t.Error("a team mail does not name Yuva")
	}
}

func TestSMTPSendsMultipartAlternative(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan []byte, 1)
	go fakeSMTP(ln, got)
	m, err := mail.Render("invite", "tr", samples("Acme <Destek>", "")["invite"])
	if err != nil {
		t.Fatal(err)
	}
	m.To, m.Unsubscribe = "success@simulator.amazonses.com", "https://yuva.example.com/u/tok"
	s := mail.SMTP{Config: config.SMTP{Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, TLS: "none", From: "Yuva <yuva@example.com>"}}
	if err := s.Send(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	raw := <-got
	msg, err := netmail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if subj, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject")); subj != m.Subject {
		t.Errorf("subject %q, want %q", subj, m.Subject)
	}
	if msg.Header.Get("List-Unsubscribe") != "<"+m.Unsubscribe+">" {
		t.Errorf("List-Unsubscribe %q", msg.Header.Get("List-Unsubscribe"))
	}
	mt, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/alternative" {
		t.Fatalf("content type %q: %v", msg.Header.Get("Content-Type"), err)
	}
	r := multipart.NewReader(msg.Body, params["boundary"])
	var kinds []string
	for {
		p, err := r.NextRawPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if p.Header.Get("Content-Transfer-Encoding") != "quoted-printable" {
			t.Errorf("encoding %q", p.Header.Get("Content-Transfer-Encoding"))
		}
		body, err := io.ReadAll(quotedprintable.NewReader(p))
		if err != nil {
			t.Fatal(err)
		}
		ct := p.Header.Get("Content-Type")
		kinds = append(kinds, ct)
		want := m.Text
		if strings.HasPrefix(ct, "text/html") {
			want = m.HTML
		}
		if strings.ReplaceAll(string(body), "\r\n", "\n") != strings.ReplaceAll(want, "\r\n", "\n") {
			t.Errorf("%s part differs from the rendered one", ct)
		}
	}
	if !slices.Equal(kinds, []string{"text/plain; charset=utf-8", "text/html; charset=utf-8"}) {
		t.Errorf("parts %v", kinds)
	}
}

func fakeSMTP(ln net.Listener, got chan<- []byte) {
	conn, err := ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	r := bufio.NewReader(conn)
	say := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
	say("220 test")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		switch cmd := strings.ToUpper(strings.TrimSpace(line)); {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			say("250 test")
		case cmd == "DATA":
			say("354 go on")
			var data bytes.Buffer
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				data.WriteString(strings.TrimPrefix(l, "."))
			}
			got <- data.Bytes()
			say("250 queued")
		case cmd == "QUIT":
			say("221 bye")
			return
		default:
			say("250 ok")
		}
	}
}

func TestPreview(t *testing.T) {
	dir := os.Getenv("YUVA_MAIL_PREVIEW_DIR")
	if dir == "" {
		t.Skip("YUVA_MAIL_PREVIEW_DIR is not set")
	}
	data := samples("Acme Support", "Hello, my invoice for October shows the wrong amount.\nCould you check it?")
	for _, name := range htmlMails {
		for _, locale := range []string{"en", "tr", "de"} {
			b := mail.Brand{}
			if name == "confirm_email" || name == "discussion_digest" || name == "rating_request" {
				b = mail.Brand{Name: "Acme Support", Color: "#0e7490"}
			}
			m, err := mail.RenderAs(b, name, locale, data[name])
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(dir, locale), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, locale, name+".html"), []byte(m.HTML), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

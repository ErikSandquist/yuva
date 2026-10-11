package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"embed"
	"encoding/hex"
	"fmt"
	htmltemplate "html/template"
	"log/slog"
	"math"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/productdevbook/yuva/api/internal/config"
)

type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
	// Unsubscribe is a one-click unsubscribe URL (RFC 8058), sent as List-Unsubscribe.
	Unsubscribe string
}

type Mailer interface {
	Send(ctx context.Context, m Message) error
}

//go:embed templates
var templatesFS embed.FS

var locales = []string{"en", "tr", "de"}

var templates = map[string]*template.Template{}

var htmlTemplates = map[string]*htmltemplate.Template{}

func init() {
	for _, l := range locales {
		templates[l] = template.Must(template.ParseFS(templatesFS, "templates/"+l+"/*.txt"))
		htmlTemplates[l] = htmltemplate.Must(htmltemplate.New("layout.html").Funcs(layoutFuncs(resolveBrand(Brand{}))).
			ParseFS(templatesFS, "templates/layout.html", "templates/"+l+"/*.html"))
	}
}

// Brand is who a mail speaks for: the zero value is Yuva itself, for mails to members and people
// signing in; a contact-facing mail sent on behalf of an inbox names the inbox and its colour.
type Brand struct {
	Name  string
	Color string
}

const (
	yuvaColor      = "#c2410c"
	yuvaDarkColor  = "#ff8a5c"
	inboxColor     = "#2563eb"
	pageColor      = "#f4f4f5"
	darkCardColor  = "#131316"
	maxBrandLength = 80
	maxLinkLabel   = 40
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type brand struct {
	Name, Color, Ink, DarkColor, OnColor string
	Yuva                                 bool
}

func resolveBrand(b Brand) brand {
	if strings.TrimSpace(b.Name) == "" {
		return brand{Name: "Yuva", Color: yuvaColor, Ink: yuvaColor, DarkColor: yuvaDarkColor, OnColor: "#ffffff", Yuva: true}
	}
	name := strings.TrimSpace(b.Name)
	if r := []rune(name); len(r) > maxBrandLength {
		name = string(r[:maxBrandLength-1]) + "…"
	}
	c := strings.ToLower(b.Color)
	if !hexColor.MatchString(c) {
		c = inboxColor
	}
	return brand{Name: name, Color: c, Ink: readableOn(c, pageColor, 0), DarkColor: readableOn(c, darkCardColor, 255), OnColor: textOn(c)}
}

func layoutFuncs(b brand) htmltemplate.FuncMap {
	return htmltemplate.FuncMap{
		"brand": func() brand { return b },
		"button": func(url, label string) map[string]string {
			return map[string]string{"URL": url, "Label": label}
		},
		"rating": func(goodURL, good, badURL, bad string) map[string]string {
			return map[string]string{"GoodURL": goodURL, "Good": good, "BadURL": badURL, "Bad": bad}
		},
		"shortURL": shortURL,
		"lines": func(s string) htmltemplate.HTML {
			parts := strings.Split(strings.ReplaceAll(strings.TrimSpace(s), "\r\n", "\n"), "\n")
			for i, p := range parts {
				parts[i] = htmltemplate.HTMLEscapeString(p)
			}
			return htmltemplate.HTML(strings.Join(parts, "<br>"))
		},
		"msoOpen": func() htmltemplate.HTML {
			return `<!--[if mso]><table role="presentation" width="560" align="center" cellpadding="0" cellspacing="0" border="0"><tr><td><![endif]-->`
		},
		"msoClose": func() htmltemplate.HTML { return `<!--[if mso]></td></tr></table><![endif]-->` },
	}
}

// shortURL is the link text for a URL whose query holds a long token: the host and path, shortened
// so it never has to break mid-token on a phone; the href keeps the full URL.
func shortURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	label := u.Host + strings.TrimSuffix(u.Path, "/")
	cut := u.RawQuery != "" || u.Fragment != ""
	if r := []rune(label); len(r) > maxLinkLabel {
		label, cut = string(r[:maxLinkLabel]), true
	}
	if cut {
		label += "…"
	}
	return label
}

func luminance(hex string) float64 {
	v, _ := strconv.ParseUint(hex[1:], 16, 32)
	ch := func(c uint64) float64 {
		f := float64(c) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(v>>16&0xff) + 0.7152*ch(v>>8&0xff) + 0.0722*ch(v&0xff)
}

func contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func textOn(bg string) string {
	if contrast(bg, "#ffffff") >= contrast(bg, "#111111") {
		return "#ffffff"
	}
	return "#111111"
}

// readableOn mixes c towards black (to 0) or white (to 255) until it reads as text on bg.
func readableOn(c, bg string, to float64) string {
	v, _ := strconv.ParseUint(c[1:], 16, 32)
	r, g, b := float64(v>>16&0xff), float64(v>>8&0xff), float64(v&0xff)
	for i := 0; i <= 10; i++ {
		t := float64(i) / 10
		out := fmt.Sprintf("#%02x%02x%02x", int(r+(to-r)*t), int(g+(to-g)*t), int(b+(to-b)*t))
		if contrast(out, bg) >= 4.5 {
			return out
		}
	}
	return fmt.Sprintf("#%02x%02x%02x", int(to), int(to), int(to))
}

type layoutData struct {
	Lang, Subject   string
	Brand           brand
	Content, Footer htmltemplate.HTML
}

func Render(name, locale string, data any) (Message, error) {
	return RenderAs(Brand{}, name, locale, data)
}

// RenderAs renders a mail's subject and text, and its HTML part when the template has one, in the
// locale's language (else English) under brand b.
func RenderAs(b Brand, name, locale string, data any) (Message, error) {
	lang := locale
	t, ok := templates[lang]
	if !ok {
		base, _, _ := strings.Cut(strings.ReplaceAll(locale, "_", "-"), "-")
		lang = strings.ToLower(base)
		if t, ok = templates[lang]; !ok {
			lang, t = "en", templates["en"]
		}
	}
	if t.Lookup(name+".txt") == nil {
		lang, t = "en", templates["en"]
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name+".txt", data); err != nil {
		return Message{}, err
	}
	subject, body, _ := strings.Cut(buf.String(), "\n\n")
	m := Message{Subject: strings.TrimSpace(subject), Text: strings.TrimLeft(body, "\n")}
	h := htmlTemplates[lang]
	if h.Lookup(name+".html") == nil {
		return m, nil
	}
	h, err := h.Clone()
	if err != nil {
		return Message{}, err
	}
	br := resolveBrand(b)
	h.Funcs(layoutFuncs(br))
	ld := layoutData{Lang: lang, Subject: m.Subject, Brand: br}
	buf.Reset()
	if err := h.ExecuteTemplate(&buf, name+".html", data); err != nil {
		return Message{}, err
	}
	ld.Content = htmltemplate.HTML(buf.String())
	if h.Lookup(name+".footer") != nil {
		buf.Reset()
		if err := h.ExecuteTemplate(&buf, name+".footer", data); err != nil {
			return Message{}, err
		}
		ld.Footer = htmltemplate.HTML(strings.TrimSpace(buf.String()))
	}
	buf.Reset()
	if err := h.ExecuteTemplate(&buf, "layout", ld); err != nil {
		return Message{}, err
	}
	m.HTML = buf.String()
	return m, nil
}

type Log struct{ Log *slog.Logger }

func (l Log) Send(ctx context.Context, m Message) error {
	l.Log.InfoContext(ctx, "mail (log mailer, not sent)",
		slog.String("to", m.To), slog.String("subject", m.Subject), slog.String("text", m.Text), slog.Int("html_bytes", len(m.HTML)))
	return nil
}

type Async struct {
	Mailer  Mailer
	Log     *slog.Logger
	Timeout time.Duration
}

func (a Async) Send(ctx context.Context, m Message) error {
	ctx = context.WithoutCancel(ctx)
	go func() {
		ctx, cancel := context.WithTimeout(ctx, a.Timeout)
		defer cancel()
		if err := a.Mailer.Send(ctx, m); err != nil {
			a.Log.ErrorContext(ctx, "mail not sent", slog.String("to", m.To), slog.Any("error", err))
		}
	}()
	return nil
}

type SMTP struct{ Config config.SMTP }

func (s SMTP) Send(ctx context.Context, m Message) error {
	from, err := mail.ParseAddress(s.Config.From)
	if err != nil {
		return fmt.Errorf("smtp from: %w", err)
	}
	addr := net.JoinHostPort(s.Config.Host, strconv.Itoa(s.Config.Port))
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	tlsConfig := &tls.Config{ServerName: s.Config.Host, MinVersion: tls.VersionTLS12}
	if s.Config.TLS == "tls" {
		conn = tls.Client(conn, tlsConfig)
	}
	c, err := smtp.NewClient(conn, s.Config.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if s.Config.TLS == "starttls" {
		if err := c.StartTLS(tlsConfig); err != nil {
			return err
		}
	}
	if s.Config.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", s.Config.Username, s.Config.Password, s.Config.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return err
	}
	if err := c.Rcpt(m.To); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(compose(from, m)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func compose(from *mail.Address, m Message) []byte {
	var b bytes.Buffer
	domain := "yuva.invalid"
	if _, d, ok := strings.Cut(from.Address, "@"); ok {
		domain = d
	}
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	header := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	header("From", from.String())
	header("To", m.To)
	header("Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	header("Date", time.Now().UTC().Format(time.RFC1123Z))
	header("Message-ID", "<"+hex.EncodeToString(id)+"@"+domain+">")
	header("Auto-Submitted", "auto-generated")
	if m.Unsubscribe != "" {
		header("List-Unsubscribe", "<"+m.Unsubscribe+">")
		header("List-Unsubscribe-Post", "List-Unsubscribe=One-Click")
	}
	header("MIME-Version", "1.0")
	if m.HTML == "" {
		header("Content-Type", "text/plain; charset=utf-8")
		header("Content-Transfer-Encoding", "quoted-printable")
		b.WriteString("\r\n")
		writeQP(&b, m.Text)
		return b.Bytes()
	}
	mw := multipart.NewWriter(&b)
	header("Content-Type", `multipart/alternative; boundary="`+mw.Boundary()+`"`)
	b.WriteString("\r\n")
	for _, part := range []struct{ kind, body string }{{"text/plain", m.Text}, {"text/html", m.HTML}} {
		w, _ := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.kind + "; charset=utf-8"},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		var pb bytes.Buffer
		writeQP(&pb, part.body)
		_, _ = w.Write(pb.Bytes())
	}
	_ = mw.Close()
	return b.Bytes()
}

func writeQP(b *bytes.Buffer, s string) {
	qp := quotedprintable.NewWriter(b)
	_, _ = qp.Write([]byte(strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")))
	_ = qp.Close()
}

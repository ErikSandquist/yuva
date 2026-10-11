// Package style holds what the server's own mails (mail's layout.html) and e-mail designs
// (mail/design) draw alike: colour contrast, font stacks, the button and the Outlook wrappers.
package style

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	FontSystem    = "-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif"
	FontHumanist  = "'Segoe UI','Helvetica Neue',Helvetica,Arial,sans-serif"
	FontGeometric = "Avenir,'Avenir Next',Montserrat,Futura,'Century Gothic',Arial,sans-serif"
	FontSerif     = "Georgia,'Times New Roman',Times,serif"
	FontMono      = "ui-monospace,SFMono-Regular,Menlo,Consolas,'Liberation Mono','Courier New',monospace"
)

// Fonts are the stacks a design may name.
var Fonts = map[string]string{
	"system": FontSystem, "humanist": FontHumanist, "geometric": FontGeometric, "serif": FontSerif, "mono": FontMono,
}

// Luminance is the WCAG relative luminance of a #rrggbb colour.
func Luminance(hex string) float64 {
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

// Contrast is the WCAG contrast ratio of two #rrggbb colours, 1 to 21.
func Contrast(a, b string) float64 {
	la, lb := Luminance(a), Luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// TextOn is white or near-black, whichever reads better on bg.
func TextOn(bg string) string {
	if Contrast(bg, "#ffffff") >= Contrast(bg, "#111111") {
		return "#ffffff"
	}
	return "#111111"
}

// ReadableOn mixes c towards black (to 0) or white (to 255) until it reads as text on bg.
func ReadableOn(c, bg string, to float64) string {
	v, _ := strconv.ParseUint(c[1:], 16, 32)
	r, g, b := float64(v>>16&0xff), float64(v>>8&0xff), float64(v&0xff)
	for i := 0; i <= 10; i++ {
		t := float64(i) / 10
		out := fmt.Sprintf("#%02x%02x%02x", int(r+(to-r)*t), int(g+(to-g)*t), int(b+(to-b)*t))
		if Contrast(out, bg) >= 4.5 {
			return out
		}
	}
	return fmt.Sprintf("#%02x%02x%02x", int(to), int(to), int(to))
}

// MSOOpen and MSOClose wrap content in a fixed-width table for Outlook for Windows, which ignores max-width.
func MSOOpen(width int) string {
	return `<!--[if mso]><table role="presentation" width="` + strconv.Itoa(width) + `" align="center" cellpadding="0" cellspacing="0" border="0"><tr><td><![endif]-->`
}

func MSOClose() string { return `<!--[if mso]></td></tr></table><![endif]-->` }

// Button is a link drawn as a button that holds in Outlook for Windows: a table cell with the
// colour and mso-padding-alt around an inline-block link with the same padding.
type Button struct {
	URL, Label           string
	Color, TextColor     string
	Font                 string
	Radius               int
	FontSize, LineHeight int
	PadY, PadX           int
	Margin               string
	FullWidth            bool
	Class, CellClass     string
}

// LayoutButton is the button of the server's own mails.
func LayoutButton(url, label, color, textColor string) Button {
	return Button{URL: url, Label: label, Color: color, TextColor: textColor, Font: FontSystem, Radius: 8,
		FontSize: 15, LineHeight: 20, PadY: 12, PadX: 24, Margin: "24px 0 8px 0"}
}

func px(n int) string { return strconv.Itoa(n) + "px" }

func (b Button) HTML() string {
	var s strings.Builder
	s.WriteString(`<table role="presentation" cellpadding="0" cellspacing="0" border="0"`)
	if b.FullWidth {
		s.WriteString(` width="100%"`)
	}
	s.WriteString(` style="`)
	if b.Margin != "" {
		s.WriteString("margin:" + b.Margin + ";")
	}
	if b.FullWidth {
		s.WriteString("width:100%;")
	}
	s.WriteString(`"><tr><td align="center" bgcolor="` + b.Color + `"`)
	if b.CellClass != "" {
		s.WriteString(` class="` + b.CellClass + `"`)
	}
	pad := px(b.PadY) + " " + px(b.PadX)
	s.WriteString(` style="background-color:` + b.Color + `;border-radius:` + px(b.Radius) + `;mso-padding-alt:` + pad + `;">`)
	s.WriteString(`<a href="` + URLAttr(b.URL) + `" target="_blank"`)
	if b.Class != "" {
		s.WriteString(` class="` + b.Class + `"`)
	}
	display := "inline-block"
	if b.FullWidth {
		display = "block"
	}
	s.WriteString(` style="display:` + display + `;padding:` + pad + `;font-family:` + b.Font + `;font-size:` + px(b.FontSize) +
		`;line-height:` + px(b.LineHeight) + `;font-weight:600;color:` + b.TextColor + `;text-decoration:none;border-radius:` + px(b.Radius) + `;">`)
	s.WriteString(Escape(b.Label))
	s.WriteString(`</a></td></tr></table>`)
	return s.String()
}

// URLAttr makes a URL safe inside a double-quoted attribute the way html/template does: schemes
// other than http, https, mailto and tel become "#ZgotmplZ", bytes that are not allowed in URLs are
// percent-encoded, and the result is HTML-escaped.
func URLAttr(u string) string {
	if i := strings.IndexAny(u, ":/?#"); i > 0 && u[i] == ':' {
		switch strings.ToLower(u[:i]) {
		case "http", "https", "mailto", "tel":
		default:
			return "#ZgotmplZ"
		}
	}
	var b strings.Builder
	for i := 0; i < len(u); i++ {
		c := u[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		case strings.IndexByte("-._~!#$&*+,/:;=?@[]", c) >= 0:
		case c == '%' && i+2 < len(u) && isHex(u[i+1]) && isHex(u[i+2]):
		default:
			fmt.Fprintf(&b, "%%%02x", c)
			continue
		}
		b.WriteByte(c)
	}
	return Escape(b.String())
}

var escaper = strings.NewReplacer("\x00", "\uFFFD", `"`, "&#34;", "&", "&amp;", "'", "&#39;", "+", "&#43;", "<", "&lt;", ">", "&gt;")

// Escape escapes text and attribute values as html/template does.
func Escape(s string) string { return escaper.Replace(s) }

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

package style_test

import (
	"bytes"
	htmltemplate "html/template"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/productdevbook/yuva/api/internal/mail/style"
)

func TestEscapingMatchesHTMLTemplate(t *testing.T) {
	tpl := htmltemplate.Must(htmltemplate.New("").Parse(`<a href="{{.}}">{{.}}</a>`))
	for _, in := range []string{
		"https://yuva.example.com/r/tok?rating=good&x=1",
		"https://example.com/a b/ç?q=\"x\"&y='z'+1",
		"mailto:ada@example.com?subject=Hi there",
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		"data:text/html,<b>",
		"/relative/path?a=<b>",
		"C++ & <Friends>",
		"100%",
	} {
		var b bytes.Buffer
		if err := tpl.Execute(&b, in); err != nil {
			t.Fatal(err)
		}
		want := b.String()
		got := `<a href="` + style.URLAttr(in) + `">` + style.Escape(in) + `</a>`
		if got != want {
			t.Errorf("%q:\n got %s\nwant %s", in, got, want)
		}
	}
}

func TestContrast(t *testing.T) {
	if c := style.Contrast("#000000", "#ffffff"); math.Abs(c-21) > 0.01 {
		t.Errorf("black on white %.2f", c)
	}
	if c := style.Contrast("#777777", "#ffffff"); math.Abs(c-4.48) > 0.01 {
		t.Errorf("#777 on white %.2f", c)
	}
	if style.TextOn("#fde68a") != "#111111" || style.TextOn("#1e3a8a") != "#ffffff" {
		t.Error("TextOn picks the wrong colour")
	}
	for _, c := range []string{"#fde68a", "#c2410c", "#0e7490", "#ffffff"} {
		if got := style.ReadableOn(c, "#f4f4f5", 0); style.Contrast(got, "#f4f4f5") < 4.5 {
			t.Errorf("ReadableOn(%s) = %s does not read", c, got)
		}
	}
}

func TestLayoutUsesTheSystemStack(t *testing.T) {
	b, err := os.ReadFile("../templates/layout.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range strings.Split(string(b), "font-family:")[1:] {
		stack, _, _ := strings.Cut(part, ";")
		if stack != style.FontSystem && stack != style.FontMono {
			t.Errorf("layout.html uses a font stack style does not know: %s", stack)
		}
	}
}

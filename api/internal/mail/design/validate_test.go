package design

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func codes(is []Issue) []string {
	var out []string
	for _, i := range is {
		out = append(out, i.BlockID+":"+i.Field+":"+i.Code)
	}
	return out
}

func validateBlocks(t *testing.T, blocks string, opt Options) []Issue {
	t.Helper()
	d, issues, err := Parse([]byte(`{"schema": 1, "theme": ` + theme + `, "blocks": [` + blocks + `]}`))
	if err != nil {
		t.Fatal(err)
	}
	return append(issues, Validate(d, opt)...)
}

func TestRichTextPolicy(t *testing.T) {
	for _, c := range []struct {
		html string
		ok   bool
	}{
		{`<p>Plain</p>`, true},
		{`<p>It's "quoted" &amp; <strong>bold</strong><br/>next</p>`, true},
		{`<p>It&#39;s &#34;quoted&#34;</p>`, true},
		{`<ul><li>a</li><li><em>b</em> <u>c</u> <s>d</s> <code>e</code></li></ul><ol><li>f</li></ol>`, true},
		{`<p><a href="https://example.com/a?b=1&amp;c=2">x</a> <a href="mailto:hi@example.com">m</a> <a href="tel:+905551112233">t</a></p>`, true},
		{`<p><a href="https://example.com/?e={{ contact.email }}">x</a></p>`, true},
		{`<p><a href="{{ unsubscribe_url }}">leave</a></p>`, true},
		{`<p>Hi {{ contact.first_name | "there" }}</p>`, true},
		{`<script>alert(1)</script>`, false},
		{`<p onclick="x()">a</p>`, false},
		{`<p style="color:red">a</p>`, false},
		{`<p class="x">a</p>`, false},
		{`<style>p{}</style><p>a</p>`, false},
		{`<img src="https://tracker.example/p.gif">`, false},
		{`<p><a href="javascript:alert(1)">x</a></p>`, false},
		{`<p><a href="data:text/html,x">x</a></p>`, false},
		{`<p><a href="/relative">x</a></p>`, false},
		{`<p><a href="https://{{ contact.email }}.example.com/">x</a></p>`, false},
		{`<p><a>no href</a></p>`, false},
		{`<p><a href="https://example.com" target="_blank">x</a></p>`, false},
		{`<h1>a</h1>`, false},
		{`<div>a</div>`, false},
		{`<p>unclosed`, false},
		{`<!-- c --><p>a</p>`, false},
		{`<form><input></form>`, false},
	} {
		is := validateBlocks(t, `{"id": "t", "type": "text", "html": `+jsonString(c.html)+`}`, Options{})
		if (len(is) == 0) != c.ok {
			t.Errorf("%s: issues %v, want ok=%v", c.html, codes(is), c.ok)
		}
	}
}

func TestHeadingIsInline(t *testing.T) {
	if is := validateBlocks(t, `{"id": "h", "type": "heading", "level": 2, "text": "<p>x</p>"}`, Options{}); len(is) != 1 || is[0].Code != "invalid_rich_text" {
		t.Errorf("issues %v", codes(is))
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestValidationNamesBlocks(t *testing.T) {
	is := validateBlocks(t, `
		{"id": "a", "type": "heading", "level": 4, "text": "x"},
		{"id": "a", "type": "text", "html": "<p>{{ contact.password }}</p>"},
		{"id": "b", "type": "button", "label": "", "url": "javascript:x"},
		{"id": "c", "type": "columns", "columns": [{"width": 50, "blocks": [{"id": "s", "type": "section", "blocks": []}]}, {"width": 40, "blocks": []}]},
		{"id": "d", "type": "image", "asset": "hero.png", "alt": "x"},
		{"id": "e", "type": "spacer", "height": 200},
		{"id": "f", "type": "social", "items": [{"network": "myspace", "url": "https://example.com"}]},
		{"id": "g", "type": "text", "html": "<p>x</p>", "label": "not here"},
		{"id": "h", "type": "divider", "color": "red"}`, Options{})
	want := []string{
		"a:level:invalid", "a:id:duplicate_id", "a:html:unknown_merge_field", "b:label:required", "b:url:invalid_url",
		"s:type:nesting", "c:columns:column_widths", "d:asset:asset_not_allowed", "e:height:invalid",
		"f:items.0.network:invalid", "g:label:invalid", "h:color:invalid",
	}
	got := codes(is)
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
			}
		}
		if !found {
			t.Errorf("missing %s in %v", w, got)
		}
	}
}

func TestSectionsDoNotNest(t *testing.T) {
	is := validateBlocks(t, `{"id": "a", "type": "section", "blocks": [{"id": "b", "type": "section", "blocks": []}]}`, Options{})
	if len(is) != 1 || is[0].Code != "nesting" {
		t.Errorf("issues %v", codes(is))
	}
	is = validateBlocks(t, `{"id": "a", "type": "section", "blocks": [{"id": "b", "type": "columns", "columns": [{"width": 50, "blocks": [{"id": "c", "type": "columns", "columns": []}]}, {"width": 50, "blocks": []}]}]}`, Options{})
	if len(is) == 0 || is[0].Code != "nesting" {
		t.Errorf("issues %v", codes(is))
	}
}

func TestTemplatesMayUseAssets(t *testing.T) {
	if is := validateBlocks(t, `{"id": "d", "type": "image", "asset": "hero.png", "alt": "x"}`, Options{Assets: true}); len(is) != 0 {
		t.Errorf("issues %v", codes(is))
	}
}

func TestSchemaAndSize(t *testing.T) {
	if _, _, err := Parse([]byte(`{"schema": 2, "theme": {}, "blocks": []}`)); err != ErrUnsupportedSchema {
		t.Errorf("schema 2: %v", err)
	}
	if _, is, _ := Parse([]byte(`{"theme": {}, "blocks": []}`)); len(is) != 1 || is[0].Field != "schema" {
		t.Errorf("no schema: %v", is)
	}
	var blocks []string
	for range 201 {
		blocks = append(blocks, `{"id": "s", "type": "spacer", "height": 8}`)
	}
	if _, _, err := Parse([]byte(`{"schema": 1, "theme": ` + theme + `, "blocks": [` + strings.Join(blocks, ",") + `]}`)); err != ErrTooLarge {
		t.Errorf("201 blocks: %v", err)
	}
	big := `{"schema": 1, "theme": ` + theme + `, "blocks": [{"id": "t", "type": "text", "html": "` + strings.Repeat("x", MaxJSONBytes) + `"}]}`
	if _, _, err := Parse([]byte(big)); err != ErrTooLarge {
		t.Errorf("big: %v", err)
	}
}

func TestStoredFormRoundTrips(t *testing.T) {
	raw := []byte(`{"schema": 1, "theme": ` + theme + `, "blocks": [{"id": "c", "type": "columns", "gap": 0, "stack": false, "columns": [{"width": 50, "blocks": [{"id": "t", "type": "text", "html": "<p>a</p>"}]}, {"width": 50, "blocks": []}]}]}`)
	d, _, _ := Parse(raw)
	again, is, err := Parse(Marshal(d))
	if err != nil || len(is) > 0 || string(Marshal(again)) != string(Marshal(d)) {
		t.Errorf("%v %v\n%s\n%s", err, is, Marshal(d), Marshal(again))
	}
	if !strings.Contains(string(Marshal(d)), `"gap":0`) || !strings.Contains(string(Marshal(d)), `"stack":false`) {
		t.Errorf("zero values lost: %s", Marshal(d))
	}
}

func TestMergeFields(t *testing.T) {
	v := Values{ContactName: "Ada Lovelace", ContactEmail: "a+b@example.com", Attributes: map[string]string{"plan": "<Pro>"}}
	for in, want := range map[string]string{
		`Hi {{ contact.first_name | "there" }}`:  "Hi Ada",
		`Hi {{contact.name}}`:                    "Hi Ada Lovelace",
		`{{ contact.attributes.plan }}`:          "<Pro>",
		`{{ contact.attributes.missing | "-" }}`: "-",
		`{{ list.name | "our list" }}`:           "our list",
		`a {{ not closed`:                        "a {{ not closed",
		`{{ unknown.field }}`:                    "{{ unknown.field }}",
	} {
		if got := v.Text(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	if got := v.URL("https://example.com/?e={{ contact.email }}"); got != "https://example.com/?e=a%2Bb%40example.com" {
		t.Errorf("url %q", got)
	}
	out, _ := Render(func() Input {
		in := input(doc(t, `{"id": "t", "type": "text", "html": "<p>{{ contact.attributes.plan }}</p>"}`))
		in.Values.Attributes = map[string]string{"plan": `<script>alert(1)</script>`}
		return in
	}())
	if strings.Contains(out.HTML, "<script>") || !strings.Contains(out.HTML, "&lt;script&gt;") {
		t.Error("a merge value became markup")
	}
}

func FuzzMergeFields(f *testing.F) {
	for _, s := range []string{`Hi {{ contact.first_name | "there" }}`, `{{`, `}}{{`, `{{ a | "b" }}`, `{{contact.email}}{{ list.name|"x"}}`, `{{{{}}}}`} {
		f.Add(s, `<b>"&'`)
	}
	f.Fuzz(func(t *testing.T, tpl, val string) {
		if !utf8.ValidString(tpl) || !utf8.ValidString(val) {
			return
		}
		v := Values{ContactName: val, ContactEmail: val, InboxName: val, ListName: val, Attributes: map[string]string{"k": val}}
		for _, f := range Fields(tpl) {
			if !fieldName.MatchString(f.Name) || utf8.RuneCountInString(f.Fallback) > maxFallback {
				t.Fatalf("parsed a bad field %+v", f)
			}
		}
		_ = v.Text(tpl)
		u := v.URL("https://example.com/?q=" + tpl)
		if !strings.ContainsAny(tpl, "<>\" ") && strings.ContainsAny(u, "<>\" ") {
			t.Fatalf("a value broke out of the query: %q", u)
		}
		in := input(Document{Schema: 1, Theme: mustTheme(), Blocks: []Block{{ID: "t", Type: "text", HTML: "<p>" + escapeText(tpl) + "</p>"}}})
		in.Values = v
		out, err := Render(in)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.HTML, "<b>") || strings.Contains(out.HTML, `"&'`) {
			t.Fatalf("a value became markup: %q in %q", val, tpl)
		}
	})
}

func escapeText(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

func mustTheme() Theme {
	var th Theme
	if err := json.Unmarshal([]byte(theme), &th); err != nil {
		panic(err)
	}
	return th
}

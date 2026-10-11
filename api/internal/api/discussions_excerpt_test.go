package api

import (
	"strings"
	"testing"
)

func TestPostExcerpt(t *testing.T) {
	got := postExcerpt("Open **Settings → Keys** and copy `yuva_pk_` ~~old~~ &amp; new\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n<script>x</script>")
	if want := "Open Settings → Keys and copy yuva_pk_ old & new a b 1 2"; got != want {
		t.Fatalf("excerpt %q, want %q", got, want)
	}
	long := postExcerpt(strings.Repeat("word ", 60))
	if r := []rune(long); len(r) > excerptRunes+1 || !strings.HasSuffix(long, "word…") {
		t.Fatalf("long excerpt %q", long)
	}
}

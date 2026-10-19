package api

import (
	"reflect"
	"strings"
	"testing"
)

func verbs(s string) string {
	var out []string
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '%' && (s[i+1] == 's' || s[i+1] == 'd') {
			out = append(out, s[i:i+2])
		}
	}
	return strings.Join(out, ",")
}

func TestStatusLanguagesCoverEveryLocale(t *testing.T) {
	en := statusLanguages["en"]
	for _, l := range homeLocales {
		w, ok := statusLanguages[l.Code]
		if !ok {
			t.Errorf("%s: no statusLanguages entry", l.Code)
			continue
		}
		if w.Lang != l.Code {
			t.Errorf("%s: Lang is %q", l.Code, w.Lang)
		}
		if (w.Dir == "rtl") != l.RTL {
			t.Errorf("%s: Dir %q does not match RTL=%v", l.Code, w.Dir, l.RTL)
		}
		wv, ev := reflect.ValueOf(w), reflect.ValueOf(en)
		for i := range wv.NumField() {
			name := wv.Type().Field(i).Name
			if name == "Dir" {
				continue
			}
			switch f := wv.Field(i); f.Kind() {
			case reflect.String:
				if f.String() == "" {
					t.Errorf("%s: %s is empty", l.Code, name)
				} else if verbs(f.String()) != verbs(ev.Field(i).String()) {
					t.Errorf("%s: %s has verbs %q, English %q", l.Code, name, verbs(f.String()), verbs(ev.Field(i).String()))
				}
			case reflect.Map:
				for _, k := range ev.Field(i).MapKeys() {
					if f.MapIndex(k).String() == "" {
						t.Errorf("%s: %s[%s] is empty", l.Code, name, k)
					}
				}
			case reflect.Array:
				for j := range f.Len() {
					if f.Index(j).String() == "" {
						t.Errorf("%s: %s[%d] is empty", l.Code, name, j)
					}
				}
			}
		}
	}
}

func TestHomeCatalogCoversEveryLocale(t *testing.T) {
	en := reflect.ValueOf(homeWordsEN)
	optional := map[string]bool{"SiteName": true, "LinkSite": true, "LinkGitHub": true, "ViaApp": true}
	for _, l := range homeLocales {
		w, ok := homeCatalog[l.Code]
		if !ok {
			t.Errorf("%s: no homeCatalog entry", l.Code)
			continue
		}
		wv := reflect.ValueOf(w)
		for i := range wv.NumField() {
			name := wv.Type().Field(i).Name
			switch f := wv.Field(i); f.Kind() {
			case reflect.String:
				if f.String() == "" && !optional[name] && l.Code != "en" {
					t.Errorf("%s: %s is empty", l.Code, name)
				}
			case reflect.Slice:
				if f.Len() != en.Field(i).Len() {
					t.Errorf("%s: %s has %d entries, English %d", l.Code, name, f.Len(), en.Field(i).Len())
				}
			}
		}
	}
}

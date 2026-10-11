package mail_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/productdevbook/yuva/api/internal/mail"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files")

func TestServerMailsMatchGolden(t *testing.T) {
	data := samples("Acme Support", "Hello, my invoice is wrong.")
	brands := map[string]mail.Brand{"yuva": {}, "inbox": {Name: "Acme Support", Color: "#0e7490"}, "pale": {Name: "Pale", Color: "#fde68a"}}
	for _, name := range append(append([]string{}, htmlMails...), englishMails...) {
		d := any(data[name])
		if name == "operator_digest" {
			d = map[string]any{"Server": "yuva.example.com", "URL": "https://yuva.example.com", "Signups": 1, "Workspaces": 1,
				"Items": []map[string]any{{"Time": "2026-10-11 09:00 UTC", "Email": "a@example.com", "Workspace": "Acme", "From": "TR"}}}
		}
		for _, locale := range []string{"en", "tr", "de"} {
			for bn, b := range brands {
				m, err := mail.RenderAs(b, name, locale, d)
				if err != nil {
					t.Fatal(err)
				}
				for ext, body := range map[string]string{"html": m.HTML, "txt": m.Subject + "\n\n" + m.Text} {
					path := filepath.Join("testdata", "golden", name+"."+locale+"."+bn+"."+ext)
					if *updateGolden {
						if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
							t.Fatal(err)
						}
						continue
					}
					want, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if string(want) != body {
						t.Errorf("%s differs from its golden file", path)
					}
				}
			}
		}
	}
}

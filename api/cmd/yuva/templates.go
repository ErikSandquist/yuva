package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/productdevbook/yuva/api/internal/gallery"
)

// templatesCLI renders every gallery template's preview.<locale>.html from a templates/email
// folder; it needs no database.
func templatesCLI(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "render" {
		return errors.New("usage: yuva templates render [--dir templates/email]")
	}
	fl := flag.NewFlagSet("templates render", flag.ContinueOnError)
	dir := fl.String("dir", "templates/email", "the gallery folder")
	if err := fl.Parse(args[1:]); err != nil {
		return err
	}
	fsys := os.DirFS(*dir)
	ts, err := gallery.Load(fsys)
	if err != nil {
		return err
	}
	for _, t := range ts {
		for _, l := range t.Locales {
			out, err := gallery.Render(fsys, t, l, gallery.PreviewAssetURL, gallery.PreviewIconURL)
			if err != nil {
				return fmt.Errorf("%s/%s: %w", t.Slug, l, err)
			}
			if err := os.WriteFile(filepath.Join(*dir, t.Slug, "preview."+l+".html"), []byte(out.HTML), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(stdout, "%s\t%s\t%d bytes", t.Slug, l, out.Size)
			for _, w := range out.Warnings {
				fmt.Fprintf(stdout, "\t%s:%s", w.BlockID, w.Code)
			}
			fmt.Fprintln(stdout)
		}
	}
	return nil
}

package design

import (
	"embed"
	"io/fs"
)

//go:embed icons/*.png
var icons embed.FS

// IconPNG is a built-in social icon by file name, "<network>-<dark|light|brand>.png".
func IconPNG(file string) ([]byte, bool) {
	b, err := fs.ReadFile(icons, "icons/"+file)
	return b, err == nil
}

package api

import (
	"fmt"
	"html"
	"html/template"
	"strings"
)

const (
	cellPitch = 4
	cellSize  = cellPitch - 1
)

type cellPath struct{ Class, D string }

// cellGrid collects the squares of one chart or strip: cells of one class merge into a single path,
// cells with a tooltip stay rects.
type cellGrid struct {
	cols, rows int
	order      []string
	paths      map[string]*strings.Builder
	rects      strings.Builder
}

func newCellGrid(cols, rows int) *cellGrid {
	return &cellGrid{cols: cols, rows: rows, paths: map[string]*strings.Builder{}}
}

func (g *cellGrid) add(class string, col, row int) {
	b := g.paths[class]
	if b == nil {
		b = &strings.Builder{}
		g.paths[class] = b
		g.order = append(g.order, class)
	}
	fmt.Fprintf(b, "M%d %dh%dv%dh-%dZ", col*cellPitch, row*cellPitch, cellSize, cellSize, cellSize)
}

func (g *cellGrid) addTitled(class string, col, row int, title string) {
	fmt.Fprintf(&g.rects, `<rect x="%d" y="%d" width="%d" height="%d" class="%s"><title>%s</title></rect>`,
		col*cellPitch, row*cellPitch, cellSize, cellSize, class, html.EscapeString(title))
}

func (g *cellGrid) Cells() []cellPath {
	out := make([]cellPath, 0, len(g.order))
	for _, k := range g.order {
		out = append(out, cellPath{Class: k, D: g.paths[k].String()})
	}
	return out
}

func (g *cellGrid) ViewBox() string {
	return fmt.Sprintf("0 0 %d %d", g.cols*cellPitch-1, g.rows*cellPitch-1)
}

func (g *cellGrid) svg(class, label string) template.HTML {
	var b strings.Builder
	a11y := `aria-hidden="true"`
	if label != "" {
		a11y = fmt.Sprintf(`role="img" aria-label="%s"`, html.EscapeString(label))
	}
	fmt.Fprintf(&b, `<svg class="%s" viewBox="%s" shape-rendering="crispEdges" %s>`, class, g.ViewBox(), a11y)
	for _, p := range g.Cells() {
		fmt.Fprintf(&b, `<path class="%s" d="%s"/>`, p.Class, p.D)
	}
	b.WriteString(g.rects.String())
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

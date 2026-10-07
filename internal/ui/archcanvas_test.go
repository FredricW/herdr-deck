package ui

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/source/arch"
)

// goFiles is a made-up Go module, example.com/shop: each package's file
// imports the packages given, using name through each.
func goFiles(pkgs map[string]map[string]string) map[string]string {
	files := map[string]string{"go.mod": "module example.com/shop\n\ngo 1.24\n"}
	for p, imps := range pkgs {
		name := p[strings.LastIndex(p, "/")+1:]
		var b strings.Builder
		fmt.Fprintf(&b, "package %s\n\n", name)
		deps := slices.Sorted(maps.Keys(imps))
		for i, d := range deps {
			fmt.Fprintf(&b, "import p%d \"example.com/shop/%s\"\n", i, d)
		}
		b.WriteString("\nfunc Run() {\n")
		for i, d := range deps {
			fmt.Fprintf(&b, "\t_ = p%d.%s\n", i, imps[d])
		}
		b.WriteString("}\n")
		files[p+"/"+name+".go"] = b.String()
	}
	return files
}

// sampleShop is a change with every edge status, a layer skip and an
// upward edge, in nested packages with unchanged siblings.
func sampleShop() *arch.Result {
	base := goFiles(map[string]map[string]string{
		"cmd/shop":               {"internal/ui": "Run", "internal/api": "Run"},
		"internal/ui":            {"internal/api": "Run", "internal/source/orders": "Run"},
		"internal/api":           {"internal/source/store": "Run"},
		"internal/source/store":  {},
		"internal/source/orders": {"internal/source/store": "Run"},
		"internal/source/money":  {},
		"internal/source/mail":   {},
		"internal/config":        {},
		"internal/deck":          {},
	})
	head := goFiles(map[string]map[string]string{
		"cmd/shop":               {"internal/ui": "Run", "internal/api": "Run"},
		"internal/ui":            {"internal/api": "Other", "internal/source/store": "Run"},
		"internal/api":           {"internal/source/store": "Run", "internal/config": "Run"},
		"internal/source/store":  {"internal/ui": "Run"},
		"internal/source/orders": {"internal/source/store": "Run"},
		"internal/source/money":  {},
		"internal/source/mail":   {},
		"internal/config":        {},
		"internal/deck":          {},
	})
	head[".config/dev.json"] = `{"x-herdr-deck": {"architecture": {"layers": [
		{"name": "entry", "paths": ["cmd/**"]},
		{"name": "ui", "paths": ["internal/ui/**"]},
		{"name": "api", "paths": ["internal/api/**"], "closed": true},
		{"name": "core", "paths": ["internal/source/**", "internal/config/**", "internal/deck/**"]}]}}}`
	return arch.FromFiles("shop", "origin/main", base, head, arch.Config{})
}

func TestCanvasSample(t *testing.T) {
	r := sampleShop()
	if r.Note != "" {
		t.Fatal(r.Note)
	}
	byStatus := map[arch.Status]int{}
	for _, e := range r.Edges {
		if e.Internal() {
			byStatus[e.Status]++
		}
	}
	if byStatus[arch.Added] == 0 || byStatus[arch.Removed] == 0 || byStatus[arch.Changed] == 0 || byStatus[arch.Unchanged] == 0 {
		t.Fatalf("the sample lacks a status: %v", byStatus)
	}
}

// The same result at the same width is the same canvas, whatever order
// Go's maps hand things out in.
func TestCanvasDeterministic(t *testing.T) {
	for _, w := range []int{60, 80, 120} {
		first := strings.Join(newCanvas(sampleShop(), w).rows(), "\n")
		for range 20 {
			if again := strings.Join(newCanvas(sampleShop(), w).rows(), "\n"); again != first {
				t.Fatalf("width %d: the canvas changed:\n%s\n---\n%s", w, ansi.Strip(first), ansi.Strip(again))
			}
		}
	}
}

// Boxes never overlap: each child lies inside its parent's border, and
// siblings share no cell, at any width; rows are exactly the canvas wide.
func TestCanvasNoOverlap(t *testing.T) {
	for w := 14; w <= 140; w += 3 {
		c := newCanvas(sampleShop(), w)
		if c.root.w > max(w, minInner+4) {
			t.Errorf("width %d: the root is %d wide", w, c.root.w)
		}
		for i, row := range c.rows() {
			if got := ansi.StringWidth(row); got != c.g.w {
				t.Errorf("width %d: row %d is %d wide, want %d", w, i, got, c.g.w)
			}
		}
		c.root.walk(func(b *cbox) {
			for i, k := range b.children {
				if k.x <= b.x || k.y <= b.y || k.x+k.w >= b.x+b.w || k.y+k.h >= b.y+b.h {
					t.Errorf("width %d: %s (%d,%d %dx%d) is not inside %s (%d,%d %dx%d)",
						w, k.path, k.x, k.y, k.w, k.h, b.path, b.x, b.y, b.w, b.h)
				}
				for _, o := range b.children[i+1:] {
					if k.x < o.x+o.w && o.x < k.x+k.w && k.y < o.y+o.h && o.y < k.y+k.h {
						t.Errorf("width %d: %s and %s overlap", w, k.path, o.path)
					}
				}
			}
		})
	}
}

// A marker takes its edge's status colour, gets `!` when the edge goes
// upward or skips a layer, and sits on the source's bottom border and the
// target's top border.
func TestCanvasMarkers(t *testing.T) {
	c := newCanvas(sampleShop(), 120)
	if c.counts || len(c.edges) == 0 {
		t.Fatalf("counts %v, %d edges", c.counts, len(c.edges))
	}
	for i, e := range c.edges {
		label := portMark(i+1, e)
		// A removed edge that skipped a layer is good news: no `!`.
		if e.Risky() != strings.HasSuffix(label, "!") || e.Status == arch.Removed && strings.HasSuffix(label, "!") {
			t.Errorf("edge %d %s → %s: label %q, verdict %v", i+1, e.From, e.To, label, e.Verdict)
		}
		for _, side := range []struct {
			b *cbox
			y int
		}{{c.boxes[e.From], c.boxes[e.From].y + c.boxes[e.From].h - 1}, {c.boxes[e.To], c.boxes[e.To].y}} {
			x, ok := findOn(c.g, side.y, side.b, label)
			if !ok {
				t.Errorf("edge %d: %q not on %s's border at row %d", i+1, label, side.b.path, side.y)
				continue
			}
			if got, want := c.g.cells[side.y][x].st, edgeStyle(e.Status); got != want {
				t.Errorf("edge %d (%s): marker style %v, want %v", i+1, edgeWord(e.Status), got, want)
			}
		}
	}
	// The styles themselves: green, yellow, red, faint.
	for st, want := range map[arch.Status]cstyle{arch.Added: csAdded, arch.Changed: csEdgeMod, arch.Removed: csRemoved, arch.Unchanged: csSame} {
		if got := edgeStyle(st); got != want {
			t.Errorf("%s: %v, want %v", edgeWord(st), got, want)
		}
	}
	if okStyle.GetForeground() != csAdded.style().GetForeground() || failStyle.GetForeground() != csRemoved.style().GetForeground() ||
		warnStyle.GetForeground() != csEdgeMod.style().GetForeground() || !csSame.style().GetFaint() {
		t.Error("edge colours are not green, red, yellow and faint")
	}
}

// findOn finds label on row y within box b.
func findOn(g *grid, y int, b *cbox, label string) (int, bool) {
	rs := []rune(label)
	for x := b.x; x+len(rs) <= b.x+b.w; x++ {
		ok := true
		for i, r := range rs {
			if g.cells[y][x+i].r != r {
				ok = false
				break
			}
		}
		// A whole marker: border on both sides, so 1 is not found in 12.
		if ok && strings.ContainsRune("─┌└", g.cells[y][x-1].r) && strings.ContainsRune("─┐┘", g.cells[y][x+len(rs)].r) {
			return x, true
		}
	}
	return 0, false
}

// Removed edges are ┄ in the legend; added, changed and unchanged are ─,
// told apart by colour (and, in plain text, a word).
func TestCanvasLegend(t *testing.T) {
	r := sampleShop()
	c := newCanvas(r, 80)
	text := ansi.Strip(strings.Join(c.legend(r, 78), "\n"))
	for i, e := range c.edges {
		want := fmt.Sprintf("%d %s %s▸ %s", i+1, strings.TrimPrefix(e.From, "internal/"), edgeLine(e.Status), strings.TrimPrefix(e.To, "internal/"))
		if !strings.Contains(text, want) {
			t.Errorf("legend lacks %q:\n%s", want, text)
		}
		if (edgeLine(e.Status) == "┄") != (e.Status == arch.Removed) {
			t.Errorf("%s edge drawn %q", edgeWord(e.Status), edgeLine(e.Status))
		}
	}
	for _, want := range []string{"✕ upward", "⚠ skips api", "removed", "changed", "─ added", "┄ removed", "unchanged edges between shown boxes"} {
		if !strings.Contains(text, want) {
			t.Errorf("legend lacks %q:\n%s", want, text)
		}
	}
	for _, l := range c.legend(r, 40) {
		if w := ansi.StringWidth(l); w > 40 {
			t.Errorf("legend line %d wide: %q", w, ansi.Strip(l))
		}
	}
}

// Past maxPorts changed edges, each box counts its edges instead.
func TestCanvasCounts(t *testing.T) {
	base, head := map[string]map[string]string{}, map[string]map[string]string{}
	for i := range 8 {
		base[fmt.Sprintf("internal/a%d", i)] = map[string]string{}
		head[fmt.Sprintf("internal/a%d", i)] = map[string]string{}
		base[fmt.Sprintf("internal/b%d", i)] = map[string]string{}
		hb := map[string]string{}
		for j := range 3 {
			hb[fmt.Sprintf("internal/a%d", (i+j)%8)] = "Run"
		}
		head[fmt.Sprintf("internal/b%d", i)] = hb
	}
	r := arch.FromFiles("dense", "origin/main", goFiles(base), goFiles(head), arch.Config{})
	c := newCanvas(r, 80)
	if !c.counts || len(c.edges) != 24 {
		t.Fatalf("counts %v with %d edges", c.counts, len(c.edges))
	}
	plain := ansi.Strip(strings.Join(c.rows(), "\n"))
	if !strings.Contains(plain, "▾3") || !strings.Contains(plain, "▴3") {
		t.Errorf("no counts:\n%s", plain)
	}
	if text := ansi.Strip(strings.Join(c.legend(r, 78), "\n")); !strings.Contains(text, "24 changed edges: ▾ out ▴ in per box") || !strings.Contains(text, "b0 ▸ a0 a1 a2") {
		t.Errorf("legend:\n%s", text)
	}
}

// Unchanged packages under a box fold into its count, and a chain of
// plain folders collapses into one box.
func TestCanvasTree(t *testing.T) {
	c := newCanvas(sampleShop(), 120)
	if b := c.boxes["internal/source"]; b == nil || b.folded != 2 {
		t.Errorf("source = %+v; want money and mail folded", b)
	}
	if c.boxes["internal/deck"] != nil {
		t.Error("an unchanged package is drawn")
	}
	if b := c.boxes["internal/source/store"]; b == nil || b.title() != "~ store" || b.titleStyle() != csChanged {
		t.Errorf("store = %+v", b)
	}
	chain := arch.FromFiles("chain", "origin/main",
		goFiles(map[string]map[string]string{"app/web/ui": {}}),
		goFiles(map[string]map[string]string{"app/web/ui": {"lib": "Run"}, "lib": {}}), arch.Config{})
	c = newCanvas(chain, 80)
	if len(c.root.children) != 2 || c.root.children[0].name != "app/web/ui" {
		var names []string
		for _, k := range c.root.children {
			names = append(names, k.name)
		}
		t.Errorf("root's boxes = %q; want app/web/ui collapsed into one", names)
	}
}

// Stepping goes to a box whose edge lies that way, and from a box with
// something straight ahead, to that one.
func TestCanvasStep(t *testing.T) {
	c := newCanvas(sampleShop(), 120)
	boxes := c.selectable()
	if len(boxes) < 4 {
		t.Fatalf("%d selectable boxes", len(boxes))
	}
	dirs := []struct {
		name   string
		dx, dy int
		ahead  func(from, to *cbox) bool
	}{
		{"down", 0, 1, func(f, b *cbox) bool { return b.y > f.y }},
		{"up", 0, -1, func(f, b *cbox) bool { return b.y < f.y }},
		{"right", 1, 0, func(f, b *cbox) bool { return b.x > f.x }},
		{"left", -1, 0, func(f, b *cbox) bool { return b.x < f.x }},
	}
	for _, from := range boxes {
		for _, d := range dirs {
			to := c.step(from, d.dx, d.dy)
			if to == nil {
				for _, b := range boxes {
					if b != from && d.ahead(from, b) {
						t.Errorf("%s from %s: none, but %s lies that way", d.name, from.path, b.path)
					}
				}
				continue
			}
			if !d.ahead(from, to) || !to.pkg {
				t.Errorf("%s from %s: %s", d.name, from.path, to.path)
			}
		}
	}
	// Siblings in one row: right goes to the next one, left comes back.
	for i, a := range boxes {
		for _, b := range boxes[i+1:] {
			if a.y == b.y && a.h == b.h && b.x == a.x+a.w+1 {
				if got := c.step(a, 1, 0); got != b {
					t.Errorf("right from %s = %v, want %s", a.path, got, b.path)
				}
				if got := c.step(b, -1, 0); got != a {
					t.Errorf("left from %s = %v, want %s", b.path, got, a.path)
				}
			}
		}
	}
}

// A selection draws its box's edges as lines in their status colours, ┄
// for a removed one, each ending in an arrow point on the target's
// border; borders stay whole and get no junction glyphs, and boxes at
// neither end of a line dim.
func TestCanvasFocus(t *testing.T) {
	r := sampleShop()
	c := newCanvas(r, 120)
	sel := "internal/ui"
	if unrouted := c.focus(r, sel); unrouted != 0 {
		t.Errorf("%d edges found no route", unrouted)
	}
	edges := c.focusEdges(r, sel)
	statuses := map[arch.Status]bool{}
	for _, e := range edges {
		statuses[e.Status] = true
	}
	if !statuses[arch.Added] || !statuses[arch.Removed] || !statuses[arch.Changed] {
		t.Fatalf("the selection lacks a status: %v", edges)
	}
	lineCells := map[cstyle]map[rune]bool{}
	arrows := map[cstyle]int{}
	for y, row := range c.g.cells {
		for x, cl := range row {
			if strings.ContainsRune("┼├┤┬┴╋┣┫┳┻", cl.r) {
				t.Errorf("a junction glyph %q at %d,%d", cl.r, x, y)
			}
			switch {
			case cl.kind == cellLine:
				if lineCells[cl.st] == nil {
					lineCells[cl.st] = map[rune]bool{}
				}
				lineCells[cl.st][cl.r] = true
			case isArrow(cl.r):
				arrows[cl.st]++
				if cl.kind != cellHoriz && cl.kind != cellVert {
					t.Errorf("an arrow off a border at %d,%d", x, y)
				}
			case cl.kind == cellHoriz && !strings.ContainsRune("─━", cl.r) && !unicode.IsDigit(cl.r) && cl.r != '!' && cl.r != ' ' && !strings.ContainsRune(" ⋯▾▴", cl.r):
				t.Errorf("a broken horizontal border %q at %d,%d", cl.r, x, y)
			case cl.kind == cellVert && !strings.ContainsRune("│┃", cl.r):
				t.Errorf("a broken vertical border %q at %d,%d", cl.r, x, y)
			}
		}
	}
	for _, st := range []cstyle{csAdded, csEdgeMod, csRemoved} {
		if arrows[st] == 0 {
			t.Errorf("no %v arrow", st)
		}
	}
	if !lineCells[csRemoved]['┄'] && !lineCells[csRemoved]['┆'] {
		t.Errorf("the removed edge is not dotted: %v", lineCells[csRemoved])
	}
	for _, st := range []cstyle{csAdded, csEdgeMod} {
		if lineCells[st]['┄'] || lineCells[st]['┆'] {
			t.Errorf("a %v line is dotted", st)
		}
	}
	// The selected box is heavy; a box at no end of its lines is dim.
	b := c.boxes[sel]
	if c.g.cells[b.y][b.x].r != '┏' {
		t.Errorf("the selection's corner is %q", c.g.cells[b.y][b.x].r)
	}
	keep := map[string]bool{sel: true}
	for _, e := range edges {
		keep[e.From], keep[e.To] = true, true
	}
	for p, bx := range c.boxes {
		if !keep[p] && bx.pkg {
			if st := c.g.cells[bx.y][bx.x].st; st != csFaint {
				t.Errorf("%s is not dimmed: %v", p, st)
			}
		}
	}
}

// A click lands on the innermost package box under it.
func TestCanvasPkgAt(t *testing.T) {
	c := newCanvas(sampleShop(), 120)
	for _, b := range c.selectable() {
		if got := c.pkgAt(b.x+1, b.y+1); got != b {
			t.Errorf("inside %s: %v", b.path, got)
		}
	}
	if got := c.pkgAt(0, 0); got != nil && got.path != "." {
		t.Errorf("the root's corner: %v", got.path)
	}
}

// threeInARow is a root with boxes a, b and c side by side; b sits
// between a and c. gaps leaves free rows above and below them.
func threeInARow(gaps bool) (*grid, map[string]*cbox, *canvas) {
	box := func(p string, x, y, w, h int) *cbox {
		return &cbox{path: p, name: p, pkg: true, impacted: true, x: x, y: y, w: w, h: h}
	}
	top, h, rootH := 2, 5, 9
	if !gaps {
		top, h, rootH = 1, 7, 9
	}
	a, b, c := box("a", 2, top, 8, h), box("b", 11, top, 8, h), box("c", 20, top, 8, h)
	root := &cbox{path: ".", name: "repo", impacted: true, x: 0, y: 0, w: 30, h: rootH, children: []*cbox{a, b, c}}
	g := newGrid(30, rootH)
	g.drawBox(root, nil)
	cv := &canvas{root: root, g: g}
	return g, map[string]*cbox{"a": a, "b": b, "c": c}, cv
}

// lineCellsIn counts the line cells inside box b, borders included.
func lineCellsIn(g *grid, b *cbox) int {
	n := 0
	for y := b.y; y < b.y+b.h; y++ {
		for x := b.x; x < b.x+b.w; x++ {
			if g.cells[y][x].kind == cellLine {
				n++
			}
		}
	}
	return n
}

// A line from a to c goes around b along the gap rows, though through b
// is shorter.
func TestRouteAvoidsSiblings(t *testing.T) {
	g, bx, cv := threeInARow(true)
	if !g.route(bx["a"], bx["c"], arch.Added, cv.lineOf(bx["a"], bx["c"])) {
		t.Fatal("no route")
	}
	if n := lineCellsIn(g, bx["b"]); n > 0 {
		t.Errorf("the line runs through b (%d cells)", n)
	}
	arrows := 0
	for y := range g.h {
		for x := range g.w {
			if isArrow(g.cells[y][x].r) {
				arrows++
				if g.cells[y][x].box != bx["c"] {
					t.Errorf("an arrow on %s's border", g.cells[y][x].box.path)
				}
			}
		}
	}
	if arrows != 1 {
		t.Errorf("%d arrows", arrows)
	}
	// b's borders stay whole.
	for y := bx["b"].y; y < bx["b"].y+bx["b"].h; y++ {
		if r := g.cells[y][bx["b"].x].r; !strings.ContainsRune("┌│└", r) {
			t.Errorf("b's left border has %q", r)
		}
	}
}

// When b walls the way from top to bottom, the line still goes through it,
// leaving its borders whole.
func TestRouteThroughASiblingWhenItMust(t *testing.T) {
	g, bx, cv := threeInARow(false)
	if !g.route(bx["a"], bx["c"], arch.Added, cv.lineOf(bx["a"], bx["c"])) {
		t.Fatal("no route")
	}
	if n := lineCellsIn(g, bx["b"]); n == 0 {
		t.Error("the line found another way, so this fixture tests nothing")
	}
	for y := bx["b"].y + 1; y < bx["b"].y+bx["b"].h-1; y++ {
		for _, x := range []int{bx["b"].x, bx["b"].x + bx["b"].w - 1} {
			if r := g.cells[y][x].r; r != '│' {
				t.Errorf("b's border at %d,%d is %q", x, y, r)
			}
		}
	}
}

// The boxes holding an end are the cheap ones to cross: the endpoints and
// their ancestors, not their siblings.
func TestLineOf(t *testing.T) {
	c := newCanvas(sampleShop(), 120)
	ui, store := c.boxes["internal/ui"], c.boxes["internal/source/store"]
	allowed := c.lineOf(ui, store)
	for _, p := range []string{"internal/ui", "internal/source/store", "internal/source", ".", "internal"} {
		if b := c.boxes[p]; b != nil && !allowed[b] {
			t.Errorf("%s is not allowed", p)
		}
	}
	if b := c.boxes["internal/source/orders"]; b != nil && allowed[b] {
		t.Error("a sibling is allowed")
	}
}

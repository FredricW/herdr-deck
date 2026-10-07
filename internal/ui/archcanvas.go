package ui

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/source/arch"
)

// The Impact tab's canvas (docs/research/architecture-tab.md §12): every
// impacted package is a box, nested in boxes for its parent folders, packed
// in rows like a treemap that fills the width. Unchanged packages under a
// box are not drawn; a count on its top border says how many (⋯7). The
// geometry depends only on the union of the base's and the head's packages
// and on the change, so it is the same for both sides, and the same for
// the same result at the same width.
//
// Borders are neutral (faint for boxes that only give context) and the
// title carries the change's colour. Edges carry status colours: green
// added, yellow changed, faint unchanged, all drawn ─, and red removed,
// drawn ┄. At rest each changed edge is a number on its source box's
// bottom border and its target's top border, with `!` when it goes upward
// or skips a layer, and the legend lists them; past maxPorts each box gets
// counts instead (▾ out, ▴ in).

// cstyle is a canvas cell's style.
type cstyle uint8

const (
	csPlain cstyle = iota
	csFaint
	csBold
	csNew     // a new package's title: green
	csChanged // a changed package's title: yellow
	csGone    // a removed package's title: red
	csMoved   // a package code only moved into: cyan
	csAdded   // an added edge: green
	csEdgeMod // a changed edge: yellow
	csRemoved // a removed edge: red
	csSame    // an unchanged edge: faint
)

func (s cstyle) style() lipgloss.Style {
	switch s {
	case csFaint, csSame:
		return dim
	case csBold:
		return bold
	case csNew:
		return okStyle.Bold(true)
	case csChanged:
		return warnStyle.Bold(true)
	case csGone:
		return failStyle.Bold(true)
	case csMoved:
		return workStyle.Bold(true)
	case csAdded:
		return okStyle
	case csEdgeMod:
		return warnStyle
	case csRemoved:
		return failStyle
	}
	return plain
}

// edgeStyle is an edge's colour by its status.
func edgeStyle(s arch.Status) cstyle {
	switch s {
	case arch.Added:
		return csAdded
	case arch.Changed:
		return csEdgeMod
	case arch.Removed:
		return csRemoved
	}
	return csSame
}

// edgeLine is an edge's straight run: ┄ for a removed edge, ─ for every
// other, which only colour tells apart.
func edgeLine(s arch.Status) string {
	if s == arch.Removed {
		return "┄"
	}
	return "─"
}

// edgeWord names an edge's status in a legend, where colour may not show.
func edgeWord(s arch.Status) string {
	switch s {
	case arch.Added:
		return "added"
	case arch.Removed:
		return "removed"
	case arch.Changed:
		return "changed"
	}
	return "unchanged"
}

// touchIcon is a touchpoint kind's one-column icon.
var touchIcon = map[string]string{
	"env": "$", "http": "⇄", "sql": "≣", "exec": "»", "fs": "▤", "flag": "⊢",
}

// cbox is one box: a folder in the package tree.
type cbox struct {
	path     string
	name     string // label; a collapsed chain shows "internal/source"
	pkg      bool   // has source files on either side
	impacted bool
	status   arch.Status
	children []*cbox
	folded   int // packages under it that are not drawn
	lines    []string
	lane     int

	x, y, w, h int    // after layout, in grid cells
	portW      [2]int // columns the markers need on the top and bottom border
}

// title is what the top border holds: the change marker and the name.
func (b *cbox) title() string {
	switch {
	case !b.pkg:
		return b.name
	case b.status == arch.Added:
		return "+ " + b.name
	case b.status == arch.Changed:
		return "~ " + b.name
	case b.status == arch.Removed:
		return "− " + b.name
	case b.status == arch.Moved:
		return "≡ " + b.name
	}
	return b.name
}

func (b *cbox) titleStyle() cstyle {
	if !b.impacted {
		return csFaint
	}
	if b.pkg {
		switch b.status {
		case arch.Added:
			return csNew
		case arch.Changed:
			return csChanged
		case arch.Removed:
			return csGone
		case arch.Moved:
			return csMoved
		}
	}
	return csBold
}

func cwidth(s string) int { return ansi.StringWidth(s) }

// abbrev cuts s to w columns with an ellipsis.
func abbrev(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// canvasTree builds the boxes for a result: the impacted packages and the
// folders above them, with the repository as the outermost box.
func canvasTree(r *arch.Result) *cbox {
	all := map[string]arch.Package{}
	for _, p := range r.Packages {
		all[p.Path] = p
	}
	nodes := map[string]*cbox{}
	var get func(p string) *cbox
	get = func(p string) *cbox {
		if n, ok := nodes[p]; ok {
			return n
		}
		_, isPkg := all[p]
		n := &cbox{path: p, name: path.Base(p), pkg: isPkg, lane: 1 << 20}
		if p == "." {
			n.name = r.Name
			if n.name == "" {
				n.name = "repository"
			}
		} else {
			parent := get(path.Dir(p))
			parent.children = append(parent.children, n)
		}
		nodes[p] = n
		return n
	}
	get(".")
	for _, p := range r.Packages {
		if p.Impacted {
			get(p.Path).impacted = true
		}
	}
	// Each package not drawn counts on its nearest drawn folder.
	for _, p := range r.Packages {
		if _, ok := nodes[p.Path]; ok {
			continue
		}
		for a := path.Dir(p.Path); ; a = path.Dir(a) {
			if n, ok := nodes[a]; ok {
				n.folded++
				break
			}
		}
	}
	for p, n := range nodes {
		pk, ok := all[p]
		if !ok {
			continue
		}
		n.lane, n.status = pk.Lane, pk.Status
		if !n.impacted {
			continue
		}
		if pk.Files > 0 {
			n.lines = append(n.lines, fmt.Sprintf("+%d −%d", pk.Added, pk.Deleted))
		}
		if !pk.API.Empty() {
			n.lines = append(n.lines, fmt.Sprintf("api +%d −%d ~%d", len(pk.API.Added), len(pk.API.Removed), len(pk.API.Changed)))
		}
		var sig []string
		if pk.MovedIn > 0 {
			var from []string
			for _, f := range pk.MovedFrom {
				from = append(from, path.Base(f))
			}
			sig = append(sig, fmt.Sprintf("≡%d←%s", pk.MovedIn, strings.Join(slices.Compact(from), ",")))
		}
		if len(pk.Touches) > 0 {
			var icons strings.Builder
			for _, k := range pk.Touches {
				icons.WriteString(touchIcon[k])
			}
			sig = append(sig, icons.String())
		}
		if pk.NewDeps > 0 {
			sig = append(sig, fmt.Sprintf("◆%d", pk.NewDeps))
		}
		if len(sig) > 0 {
			n.lines = append(n.lines, strings.Join(sig, " "))
		}
	}
	root := nodes["."]
	var fix func(n *cbox) int
	fix = func(n *cbox) int {
		for _, c := range n.children {
			n.lane = min(n.lane, fix(c))
		}
		slices.SortFunc(n.children, func(a, b *cbox) int {
			if a.lane != b.lane {
				return a.lane - b.lane
			}
			return strings.Compare(a.name, b.name)
		})
		// Collapse a chain of plain folders: internal → source.
		for i, c := range n.children {
			for !c.pkg && c.folded == 0 && len(c.children) == 1 && len(c.lines) == 0 {
				g := c.children[0]
				g.name = c.name + "/" + g.name
				c = g
			}
			n.children[i] = c
		}
		return n.lane
	}
	fix(root)
	return root
}

// minWidth is the narrowest the box can be: its title, markers and folded
// count on the top border, its lines, or a child's minimum.
func (b *cbox) minWidth() int {
	w := cwidth(b.title()) + 5 + b.portW[0] + 2
	if b.folded > 0 {
		w += cwidth(fmt.Sprintf(" ⋯%d", b.folded))
	}
	w = max(w, b.portW[1]+4)
	for _, l := range b.lines {
		w = max(w, cwidth(l)+4)
	}
	for _, c := range b.children {
		w = max(w, c.minWidth()+4)
	}
	return w
}

// pref is the width a box asks for when its parent has inner columns: its
// minimum, grown to fit its children side by side when there is room.
func (b *cbox) pref(inner int) int {
	w := b.minWidth()
	if len(b.children) > 0 {
		sum := -1
		for _, c := range b.children {
			sum += c.pref(inner-4) + 1
		}
		w = max(w, sum+4)
	}
	return min(w, inner)
}

// minInner is the narrowest a box's inside may be to hold children; a box
// narrower than that folds them into its count.
const minInner = 8

// gapY is the blank rows above, between and below a box's rows of
// children: room for the selected box's lines to run.
const gapY = 1

// layout places b at (x, y) with exactly width w, its children flowing in
// rows inside it, each row stretched to fill the width and each box to
// its row's height.
func (b *cbox) layout(x, y, w int) {
	b.x, b.y, b.w = x, y, w
	inner := w - 4 // the border and one column of padding on each side
	if inner < minInner && len(b.children) > 0 {
		for _, c := range b.children {
			b.folded += c.count()
		}
		b.children = nil
	}
	cy := y + 1 + len(b.lines)
	if len(b.children) > 0 {
		cy += gapY
	}
	var row []*cbox
	rowW := 0
	flush := func() {
		if len(row) == 0 {
			return
		}
		extra := inner - rowW
		cx := x + 2
		rowH := 0
		for i, c := range row {
			cw := c.pref(inner)
			add := extra / len(row)
			if i < extra%len(row) {
				add++
			}
			c.layout(cx, cy, cw+add)
			cx += cw + add + 1
			rowH = max(rowH, c.h)
		}
		for _, c := range row {
			c.h = max(c.h, rowH)
		}
		cy += rowH + gapY
		row, rowW = nil, 0
	}
	for _, c := range b.children {
		cw := c.pref(inner)
		need := cw
		if len(row) > 0 {
			need++
		}
		if len(row) > 0 && rowW+need > inner {
			flush()
			need = cw
		}
		row = append(row, c)
		rowW += need
	}
	flush()
	b.h = cy - y + 1
}

// count is how many packages the box and the boxes in it stand for.
func (b *cbox) count() int {
	n := b.folded
	if b.pkg {
		n++
	}
	for _, c := range b.children {
		n += c.count()
	}
	return n
}

// walk visits b and every box inside it, outer boxes first.
func (b *cbox) walk(f func(*cbox)) {
	f(b)
	for _, c := range b.children {
		c.walk(f)
	}
}

// ---- the grid

// Cell kinds: what a cell holds, for the lines a selection routes.
const (
	cellFree   = ' '
	cellText   = 't'
	cellHoriz  = 'h' // a horizontal border
	cellVert   = 'v' // a vertical border
	cellCorner = 'c'
	cellLine   = 'L' // a selected box's edge
)

type ccell struct {
	r    rune
	st   cstyle
	kind byte
	box  *cbox
	// lineH says a line cell runs horizontally (else vertically).
	lineH bool
}

type grid struct {
	w, h  int
	cells [][]ccell
}

func newGrid(w, h int) *grid {
	g := &grid{w: w, h: h, cells: make([][]ccell, h)}
	for y := range g.cells {
		g.cells[y] = make([]ccell, w)
		for x := range g.cells[y] {
			g.cells[y][x] = ccell{r: ' ', kind: cellFree}
		}
	}
	return g
}

func (g *grid) set(x, y int, r rune, st cstyle, kind byte, b *cbox) {
	if x < 0 || y < 0 || x >= g.w || y >= g.h {
		return
	}
	if ansi.StringWidth(string(r)) != 1 {
		r = '?' // one rune, one column: a wide one would shift the row
	}
	g.cells[y][x] = ccell{r: r, st: st, kind: kind, box: b}
}

func (g *grid) text(x, y int, s string, st cstyle, b *cbox) int {
	for _, r := range s {
		g.set(x, y, r, st, cellText, b)
		x++
	}
	return x
}

// drawBox draws b and the boxes inside it; the selected box (sel) gets a
// heavy border, in the same neutral colour.
func (g *grid) drawBox(b, sel *cbox) {
	st := csPlain
	if !b.impacted {
		st = csFaint
	}
	h, v, tl, tr, bl, br := '─', '│', '┌', '┐', '└', '┘'
	if b == sel {
		h, v, tl, tr, bl, br = '━', '┃', '┏', '┓', '┗', '┛'
		st = csBold
	}
	x0, y0, x1, y1 := b.x, b.y, b.x+b.w-1, b.y+b.h-1
	for x := x0 + 1; x < x1; x++ {
		g.set(x, y0, h, st, cellHoriz, b)
		g.set(x, y1, h, st, cellHoriz, b)
	}
	for y := y0 + 1; y < y1; y++ {
		g.set(x0, y, v, st, cellVert, b)
		g.set(x1, y, v, st, cellVert, b)
	}
	g.set(x0, y0, tl, st, cellCorner, b)
	g.set(x1, y0, tr, st, cellCorner, b)
	g.set(x0, y1, bl, st, cellCorner, b)
	g.set(x1, y1, br, st, cellCorner, b)
	end := g.text(x0+2, y0, " "+abbrev(b.title(), b.w-6)+" ", b.titleStyle(), b)
	if b.folded > 0 {
		f := fmt.Sprintf(" ⋯%d ", b.folded)
		if fx := x1 - cwidth(f); fx > end {
			g.text(fx, y0, f, csFaint, b)
		}
	}
	for i, l := range b.lines {
		g.text(x0+2, y0+1+i, abbrev(l, b.w-4), csPlain, b)
	}
	for _, c := range b.children {
		g.drawBox(c, sel)
	}
}

// mark writes a label on a box's top (in) or bottom (out) border, left to
// right after what is there; a label that does not fit is dropped (the
// layout reserves room, so only a box squeezed below its minimum drops one).
func (g *grid) mark(next map[*cbox][2]int, b *cbox, top bool, label string, st cstyle) {
	i, y := 1, b.y+b.h-1
	if top {
		i, y = 0, b.y
	}
	x := next[b][i]
	if x == 0 {
		x = b.x + 2
		if top {
			x = b.x + 2 + cwidth(abbrev(b.title(), b.w-6)) + 3
		}
	}
	if x+cwidth(label) > b.x+b.w-2 {
		return
	}
	for _, r := range label {
		g.set(x, y, r, st, cellHoriz, b)
		x++
	}
	p := next[b]
	p[i] = x + 1 // a border cell between markers: 1─3, not 13
	next[b] = p
}

// line renders row y, grouping runs of one style.
func (g *grid) line(y int) string {
	var b, run strings.Builder
	cur := cstyle(255)
	flush := func() {
		if run.Len() > 0 {
			b.WriteString(cur.style().Render(run.String()))
			run.Reset()
		}
	}
	for _, c := range g.cells[y] {
		if c.st != cur {
			flush()
			cur = c.st
		}
		run.WriteRune(c.r)
	}
	flush()
	return b.String()
}

// ---- the canvas

// maxPorts is the most changed edges that get a marker each; beyond it
// each box gets counts instead, and the legend groups edges by source.
const maxPorts = 20

// canvas is a laid-out, drawn result at one width.
type canvas struct {
	root  *cbox
	boxes map[string]*cbox
	// edges are the changed internal edges between drawn boxes, in the
	// result's (risk) order: marker n is edges[n-1].
	edges  []arch.Edge
	hidden int // changed internal edges with an end not drawn
	same   int // unchanged internal edges between drawn, impacted boxes
	counts bool
	g      *grid
}

// portMark is an edge's marker: its number, with `!` when the edge goes
// upward or skips a layer, so plain text shows the risk too. Plain digits:
// circled ones are drawn two columns wide by some terminals.
func portMark(n int, e arch.Edge) string {
	if e.Risky() {
		return fmt.Sprintf("%d!", n)
	}
	return fmt.Sprint(n)
}

// newCanvas lays the result out at width w and draws it at rest.
func newCanvas(r *arch.Result, w int) *canvas {
	c := &canvas{root: canvasTree(r), boxes: map[string]*cbox{}}
	c.root.walk(func(b *cbox) { c.boxes[b.path] = b })
	for _, e := range r.Edges {
		if !e.Internal() {
			continue
		}
		from, to := c.boxes[e.From], c.boxes[e.To]
		switch {
		case e.Status == arch.Unchanged:
			if from != nil && to != nil && from.impacted && to.impacted {
				c.same++
			}
		case from == nil || to == nil:
			c.hidden++
		default:
			c.edges = append(c.edges, e)
		}
	}
	c.counts = len(c.edges) > maxPorts
	// Reserve border room for every marker, so none is dropped.
	if c.counts {
		out, in := c.tally()
		for b, n := range out {
			b.portW[1] += cwidth(fmt.Sprintf("▾%d!", n)) + 1
		}
		for b, n := range in {
			b.portW[0] += cwidth(fmt.Sprintf("▴%d!", n)) + 1
		}
	} else {
		for i, e := range c.edges {
			l := cwidth(portMark(i+1, e)) + 1
			c.boxes[e.From].portW[1] += l
			c.boxes[e.To].portW[0] += l
		}
	}
	c.root.layout(0, 0, max(w, minInner+4))
	// A narrow width may have folded boxes away: their edges are hidden.
	drawn := map[string]*cbox{}
	c.root.walk(func(b *cbox) { drawn[b.path] = b })
	if len(drawn) < len(c.boxes) {
		c.boxes = drawn
		kept := c.edges[:0]
		for _, e := range c.edges {
			if drawn[e.From] != nil && drawn[e.To] != nil {
				kept = append(kept, e)
			} else {
				c.hidden++
			}
		}
		c.edges = kept
	}
	c.g = newGrid(c.root.w, c.root.h)
	c.g.drawBox(c.root, nil)
	c.markers()
	return c
}

// tally counts each box's changed edges out and in.
func (c *canvas) tally() (out, in map[*cbox]int) {
	out, in = map[*cbox]int{}, map[*cbox]int{}
	for _, e := range c.edges {
		out[c.boxes[e.From]]++
		in[c.boxes[e.To]]++
	}
	return out, in
}

// markers puts the edges' numbers on the borders, or, when dense, each
// box's counts: ▾N out, ▴N in, in an edge colour when all its edges share
// one, with `!` when one of them is risky.
func (c *canvas) markers() {
	next := map[*cbox][2]int{}
	if !c.counts {
		for i, e := range c.edges {
			l := portMark(i+1, e)
			st := edgeStyle(e.Status)
			c.g.mark(next, c.boxes[e.From], false, l, st)
			c.g.mark(next, c.boxes[e.To], true, l, st)
		}
		return
	}
	type tallyOf struct {
		n     int
		st    cstyle
		mixed bool
		risky bool
	}
	out, in := map[*cbox]*tallyOf{}, map[*cbox]*tallyOf{}
	add := func(m map[*cbox]*tallyOf, b *cbox, e arch.Edge) {
		t := m[b]
		if t == nil {
			t = &tallyOf{st: edgeStyle(e.Status)}
			m[b] = t
		}
		t.n++
		t.mixed = t.mixed || t.st != edgeStyle(e.Status)
		t.risky = t.risky || e.Risky()
	}
	var order []*cbox
	seen := map[*cbox]bool{}
	for _, e := range c.edges {
		add(out, c.boxes[e.From], e)
		add(in, c.boxes[e.To], e)
		for _, b := range []*cbox{c.boxes[e.From], c.boxes[e.To]} {
			if !seen[b] {
				seen[b] = true
				order = append(order, b)
			}
		}
	}
	label := func(arrow string, t *tallyOf) (string, cstyle) {
		l := fmt.Sprintf("%s%d", arrow, t.n)
		if t.risky {
			l += "!"
		}
		if t.mixed {
			return l, csBold
		}
		return l, t.st
	}
	for _, b := range order {
		if t := in[b]; t != nil {
			l, st := label("▴", t)
			c.g.mark(next, b, true, l, st)
		}
		if t := out[b]; t != nil {
			l, st := label("▾", t)
			c.g.mark(next, b, false, l, st)
		}
	}
}

// rows are the canvas's lines, styled.
func (c *canvas) rows() []string {
	out := make([]string, c.g.h)
	for y := range out {
		out[y] = c.g.line(y)
	}
	return out
}

// legendItem is one line of the legend.
type legendItem struct {
	text string
	st   cstyle
}

// labelPrefix is what legend names leave out: the folder every drawn
// package shares, or internal/ in Go.
func (c *canvas) labelPrefix(r *arch.Result) string {
	var paths []string
	for p, b := range c.boxes {
		if b.pkg && b.impacted && p != "." {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return ""
	}
	slices.Sort(paths)
	pre := path.Dir(paths[0])
	for _, p := range paths {
		for pre != "." && p != pre && !strings.HasPrefix(p, pre+"/") {
			pre = path.Dir(pre)
		}
	}
	if pre != "." {
		return pre + "/"
	}
	if r.Language == arch.LangGo {
		return "internal/"
	}
	return ""
}

// legend lists the canvas's edges and what its marks mean, w columns wide.
func (c *canvas) legend(r *arch.Result, w int) []string {
	pre := c.labelPrefix(r)
	name := func(p string) string {
		if s := strings.TrimPrefix(p, pre); s != "" {
			return s
		}
		return p
	}
	var items []legendItem
	plainf := func(st cstyle, f string, a ...any) { items = append(items, legendItem{fmt.Sprintf(f, a...), st}) }
	if !c.counts {
		for i, e := range c.edges {
			tag := ""
			switch {
			case e.Risky() && e.Verdict == arch.VerdictUp:
				tag = "  ✕ upward"
			case e.Risky():
				tag = "  ⚠ " + e.Why
			case e.Verdict == arch.VerdictUp:
				tag = "  was upward"
			case e.Verdict == arch.VerdictSkip:
				tag = "  was a skip"
			}
			if e.Status != arch.Added {
				tag += "  " + edgeWord(e.Status)
			}
			plainf(edgeStyle(e.Status), "%d %s %s▸ %s%s", i+1, name(e.From), edgeLine(e.Status), name(e.To), tag)
		}
	} else {
		plainf(csPlain, "%d changed edges: ▾ out ▴ in per box", len(c.edges))
		var from []string
		to := map[string][]string{}
		for _, e := range c.edges {
			if to[e.From] == nil {
				from = append(from, e.From)
			}
			mark := ""
			if e.Risky() {
				mark = "!"
			}
			switch e.Status {
			case arch.Removed:
				mark += "−"
			case arch.Changed:
				mark += "~"
			}
			to[e.From] = append(to[e.From], mark+name(e.To))
		}
		for _, f := range from {
			plainf(csPlain, "%s ▸ %s", name(f), strings.Join(to[f], " "))
		}
	}
	if c.same > 0 {
		plainf(csFaint, "%d unchanged edges between shown boxes", c.same)
	}
	if c.hidden > 0 {
		plainf(csFaint, "%d changed edges into folded packages", c.hidden)
	}
	var out []string
	for _, it := range items {
		out = append(out, it.st.style().Render(abbrev(it.text, w)))
	}
	// What the colours and marks mean, wrapped at the width.
	key := []string{
		csAdded.style().Render("─ added"), csEdgeMod.style().Render("─ changed"),
		csSame.style().Render("─ unchanged"), csRemoved.style().Render("┄ removed"),
	}
	marks := []string{"+ new", "~ changed", "− gone", "≡ moved in", "! risky", "$ env", "⇄ http", "≣ sql", "» exec", "▤ fs", "⊢ flag", "◆ dep"}
	for i := range marks {
		marks[i] = dim.Render(marks[i])
	}
	out = append(out, wrapWords(key, w)...)
	return append(out, wrapWords(marks, w)...)
}

// wrapWords joins styled words with two spaces, wrapped at w columns.
func wrapWords(words []string, w int) []string {
	var out []string
	cur, cw := "", 0
	for _, s := range words {
		sw := ansi.StringWidth(s)
		if cw > 0 && cw+2+sw > w {
			out = append(out, cur)
			cur, cw = "", 0
		}
		if cw > 0 {
			cur += "  "
			cw += 2
		}
		cur += s
		cw += sw
	}
	if cw > 0 {
		out = append(out, cur)
	}
	return out
}

// ---- the selection

// maxLines is how many lines a selection draws at most when its box has
// few changed edges: unchanged edges fill up to it, so a box's place in the
// graph shows without a hub drowning in lines.
const maxLines = 6

// lineGlyphs are a status's straight runs and corners, as horizontal,
// vertical, ┌ ┐ └ ┘: ─ for every status but removed, which is ┄.
func lineGlyphs(s arch.Status) [6]rune {
	if s == arch.Removed {
		return [6]rune{'┄', '┆', '┌', '┐', '└', '┘'}
	}
	return [6]rune{'─', '│', '┌', '┐', '└', '┘'}
}

// selectable are the boxes the selection can rest on: drawn packages, in
// reading order (top to bottom, then left to right).
func (c *canvas) selectable() []*cbox {
	var out []*cbox
	c.root.walk(func(b *cbox) {
		if b.pkg && c.boxes[b.path] == b {
			out = append(out, b)
		}
	})
	slices.SortStableFunc(out, func(a, b *cbox) int {
		if a.y != b.y {
			return a.y - b.y
		}
		return a.x - b.x
	})
	return out
}

// pkgAt is the innermost drawn package box holding cell (x, y), or nil.
func (c *canvas) pkgAt(x, y int) *cbox {
	var hit *cbox
	c.root.walk(func(b *cbox) {
		if b.pkg && c.boxes[b.path] == b && x >= b.x && x < b.x+b.w && y >= b.y && y < b.y+b.h {
			hit = b // walk visits outer boxes first
		}
	})
	return hit
}

// step is the selectable box nearest to from in direction (dx, dy): one
// whose edge lies that way, scored by the distance that way plus twice
// the gap across it, so a box straight ahead wins over a nearer one off
// to the side. Ties go to the one first in reading order.
func (c *canvas) step(from *cbox, dx, dy int) *cbox {
	var best *cbox
	bestScore := 0
	gap := func(a0, a1, b0, b1 int) int { // between [a0,a1) and [b0,b1)
		switch {
		case b1 <= a0:
			return a0 - b1 + 1
		case a1 <= b0:
			return b0 - a1 + 1
		}
		return 0
	}
	for _, b := range c.selectable() {
		if b == from {
			continue
		}
		var along, across int
		switch {
		case dy > 0:
			along, across = b.y-from.y, gap(from.x, from.x+from.w, b.x, b.x+b.w)
		case dy < 0:
			along, across = from.y-b.y, gap(from.x, from.x+from.w, b.x, b.x+b.w)
		case dx > 0:
			along, across = b.x-from.x, gap(from.y, from.y+from.h, b.y, b.y+b.h)
		default:
			along, across = from.x-b.x, gap(from.y, from.y+from.h, b.y, b.y+b.h)
		}
		if along <= 0 {
			continue
		}
		if score := along + 2*across; best == nil || score < bestScore {
			best, bestScore = b, score
		}
	}
	return best
}

// focusEdges are the edges a selection of box sel draws: its changed
// internal edges between drawn boxes, then unchanged ones up to maxLines.
func (c *canvas) focusEdges(r *arch.Result, sel string) []arch.Edge {
	var out []arch.Edge
	for _, e := range c.edges {
		if e.From == sel || e.To == sel {
			out = append(out, e)
		}
	}
	for _, e := range r.Edges {
		if len(out) >= maxLines {
			break
		}
		if e.Status == arch.Unchanged && (e.From == sel || e.To == sel) && c.boxes[e.From] != nil && c.boxes[e.To] != nil {
			out = append(out, e)
		}
	}
	return out
}

// focus redraws the canvas for a selected box: a heavy border on it, its
// edges as lines from its border to the other box's, everything not at
// either end of one dimmed, and no markers. It returns how many of the
// edges found no route.
func (c *canvas) focus(r *arch.Result, sel string) (unrouted int) {
	b := c.boxes[sel]
	if b == nil {
		return 0
	}
	c.g = newGrid(c.root.w, c.root.h)
	c.g.drawBox(c.root, b)
	keep := map[*cbox]bool{b: true}
	for _, e := range c.focusEdges(r, sel) {
		from, to := c.boxes[e.From], c.boxes[e.To]
		keep[from], keep[to] = true, true
		if !c.g.route(from, to, e.Status) {
			unrouted++
		}
	}
	for y := range c.g.cells {
		for x := range c.g.cells[y] {
			cl := &c.g.cells[y][x]
			if cl.kind != cellLine && cl.box != nil && !keep[cl.box] && !isArrow(cl.r) {
				cl.st = csFaint
			}
		}
	}
	return unrouted
}

func isArrow(r rune) bool { return strings.ContainsRune("▸▾◂▴", r) }

// route draws one edge as an orthogonal line from the source box's border
// to the target's, through free cells: crossing a border (at a right
// angle) or another line costs extra and turns cost a little, so routes
// stay straight. Borders and earlier lines stay whole: the line stops on
// one side and goes on at the other. The only glyph it puts on a border
// is its arrow point, on the target's. It reports whether a route exists.
func (g *grid) route(from, to *cbox, status arch.Status) bool {
	type state struct{ x, y, d int }
	dx := []int{1, 0, -1, 0}
	dy := []int{0, 1, 0, -1}
	onBorder := func(b *cbox, x, y int) bool {
		c := g.cells[y][x]
		return c.box == b && (c.kind == cellHoriz || c.kind == cellVert)
	}
	const inf = 1 << 30
	n := g.w * g.h * 4
	dist := make([]int, n)
	prev := make([]int32, n)
	for i := range dist {
		dist[i] = inf
		prev[i] = -1
	}
	idx := func(s state) int { return (s.y*g.w+s.x)*4 + s.d }
	// A bucketed queue: costs are small integers.
	buckets := map[int][]state{}
	push := func(s state, c, p int) {
		if c < dist[idx(s)] {
			dist[idx(s)] = c
			prev[idx(s)] = int32(p)
			buckets[c] = append(buckets[c], s)
		}
	}
	for y := from.y; y < from.y+from.h; y++ {
		for x := from.x; x < from.x+from.w; x++ {
			if !onBorder(from, x, y) {
				continue
			}
			for d := range 4 {
				// Leave a border at a right angle.
				if (g.cells[y][x].kind == cellHoriz) == (d%2 == 1) {
					push(state{x, y, d}, 0, -1)
				}
			}
		}
	}
	end := -1
	for c := 0; c < 8*(g.w+g.h)+200 && end < 0; c++ {
		for len(buckets[c]) > 0 && end < 0 {
			s := buckets[c][0]
			buckets[c] = buckets[c][1:]
			if dist[idx(s)] != c {
				continue
			}
			if onBorder(to, s.x, s.y) && c > 0 {
				end = idx(s)
				break
			}
			for nd := range 4 {
				if nd == (s.d+2)%4 {
					continue
				}
				nx, ny := s.x+dx[nd], s.y+dy[nd]
				if nx < 0 || ny < 0 || nx >= g.w || ny >= g.h {
					continue
				}
				cc := g.cells[ny][nx]
				step := 1
				if nd != s.d {
					step += 2
				}
				switch cc.kind {
				case cellText, cellCorner:
					continue
				case cellHoriz, cellVert:
					if (cc.kind == cellHoriz) != (nd%2 == 1) {
						continue // never run along a border
					}
					if cc.box != to {
						step += 6
					}
				case cellLine:
					if cc.lineH == (nd%2 == 0) {
						continue // never share a line's run
					}
					step += 8
				}
				push(state{nx, ny, nd}, c+step, idx(s))
			}
		}
	}
	if end < 0 {
		return false
	}
	var cells []state
	for i := end; i >= 0; i = int(prev[i]) {
		cells = append(cells, state{(i / 4) % g.w, (i / 4) / g.w, i % 4})
	}
	slices.Reverse(cells)
	st := edgeStyle(status)
	gl := lineGlyphs(status)
	for i, s := range cells {
		c := &g.cells[s.y][s.x]
		in, out := s.d, s.d // 0 right, 1 down, 2 left, 3 up
		if i+1 < len(cells) {
			out = cells[i+1].d
		}
		switch {
		case i == len(cells)-1:
			*c = ccell{r: []rune("▸▾◂▴")[in], st: st, kind: c.kind, box: c.box}
		case c.kind == cellHoriz || c.kind == cellVert || c.kind == cellCorner || c.kind == cellLine:
			// Borders and earlier lines stay whole.
		default:
			*c = ccell{r: bend(in, out, gl), st: st, kind: cellLine, lineH: out%2 == 0, box: c.box}
		}
	}
	return true
}

// bend is the glyph for a line cell entered going in and left going out.
func bend(in, out int, g [6]rune) rune {
	if in%2 == out%2 {
		if in%2 == 0 {
			return g[0]
		}
		return g[1]
	}
	switch [2]int{in, out} {
	case [2]int{2, 1}, [2]int{3, 0}:
		return g[2] // ┌
	case [2]int{0, 1}, [2]int{3, 2}:
		return g[3] // ┐
	case [2]int{2, 3}, [2]int{1, 0}:
		return g[4] // └
	}
	return g[5] // ┘
}

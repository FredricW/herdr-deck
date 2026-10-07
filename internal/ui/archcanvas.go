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
)

type ccell struct {
	r    rune
	st   cstyle
	kind byte
	box  *cbox
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

func (g *grid) drawBox(b *cbox) {
	st := csPlain
	if !b.impacted {
		st = csFaint
	}
	x0, y0, x1, y1 := b.x, b.y, b.x+b.w-1, b.y+b.h-1
	for x := x0 + 1; x < x1; x++ {
		g.set(x, y0, '─', st, cellHoriz, b)
		g.set(x, y1, '─', st, cellHoriz, b)
	}
	for y := y0 + 1; y < y1; y++ {
		g.set(x0, y, '│', st, cellVert, b)
		g.set(x1, y, '│', st, cellVert, b)
	}
	g.set(x0, y0, '┌', st, cellCorner, b)
	g.set(x1, y0, '┐', st, cellCorner, b)
	g.set(x0, y1, '└', st, cellCorner, b)
	g.set(x1, y1, '┘', st, cellCorner, b)
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
		g.drawBox(c)
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
	if e.Verdict.Risky() {
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
	c.g.drawBox(c.root)
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
		t.risky = t.risky || e.Verdict.Risky()
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
			switch e.Verdict {
			case arch.VerdictUp:
				tag = "  ✕ upward"
			case arch.VerdictSkip:
				tag = "  ⚠ " + e.Why
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
			if e.Verdict.Risky() {
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

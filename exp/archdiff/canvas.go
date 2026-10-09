package main

import (
	"fmt"
	"path"
	"slices"
	"strings"
	"unicode/utf8"
)

// The canvas view: every impacted package is a box, nested inside boxes
// for its parent directories, packed in rows like a treemap that fills
// the width. The geometry depends only on the union of the base's and the
// head's packages and on the diff's signals, never on which side is
// shown, so toggling before/after recolours boxes without moving them.
//
// Edges are drawn by one of three modes (edgeMode), to compare them:
// ports (numbered markers on borders plus a legend), lines (orthogonal
// routes through the gaps between boxes) and focus (lines only for the
// selected box's edges).

// cnode is one box: a directory in the package tree.
type cnode struct {
	path     string
	name     string // label; a collapsed chain shows "internal/source"
	pkg      bool   // has Go files on either side
	impacted bool
	kind     rune // '+' new, '~' changed, '−' removed, '≡' only moved into, ' ' context
	children []*cnode
	folded   int // unchanged packages under it that are not drawn
	lines    []string
	lane     int

	x, y, w, h int    // absolute, after layout
	portW      [2]int // columns the port markers need on the top and bottom borders
}

// Touchpoint icons: one column each.
var touchIcon = map[string]string{
	"env": "$", "http": "⇄", "sql": "≣", "exec": "»", "fs": "▤", "flag": "⊢",
}

func (r *result) canvasTree(repoName string) *cnode {
	shown := r.shownNodes()
	all := map[string]bool{}
	for d := range r.base.pkgs {
		all[d] = true
	}
	for d := range r.head.pkgs {
		all[d] = true
	}
	nodes := map[string]*cnode{}
	var get func(p string) *cnode
	get = func(p string) *cnode {
		if n, ok := nodes[p]; ok {
			return n
		}
		n := &cnode{path: p, name: path.Base(p), pkg: all[p], kind: ' ', lane: 1 << 20}
		if p == "." {
			n.name = repoName
		} else {
			parent := get(path.Dir(p))
			parent.children = append(parent.children, n)
		}
		nodes[p] = n
		return n
	}
	get(".")
	for p := range shown {
		get(p).impacted = true
	}
	// Folded: each undrawn package counts on its nearest drawn ancestor.
	for p := range all {
		if _, ok := nodes[p]; ok {
			continue
		}
		a := p
		for {
			a = path.Dir(a)
			if n, ok := nodes[a]; ok {
				n.folded++
				break
			}
		}
	}
	added, removed := setOf(r.pkgsAdded), setOf(r.pkgsRemoved)
	movedIn := map[string]int{}
	movedFrom := map[string]map[string]bool{}
	for _, m := range r.moves.movedDecls {
		if m.from != m.to {
			movedIn[m.to]++
			if movedFrom[m.to] == nil {
				movedFrom[m.to] = map[string]bool{}
			}
			movedFrom[m.to][path.Base(m.from)] = true
		}
	}
	icons := map[string][]string{}
	for _, t := range r.touches {
		if t.added {
			icons[t.pkg] = append(icons[t.pkg], touchIcon[t.touch.kind])
		}
	}
	extNew := map[string]int{}
	for _, e := range r.edges {
		if e.added && strings.HasPrefix(e.to, "ext:") {
			extNew[e.from]++
		}
	}
	for p, n := range nodes {
		if n.pkg {
			n.lane = r.lanes.of[p]
		}
		switch {
		case added[p]:
			n.kind = '+'
		case removed[p]:
			n.kind = '−'
		case r.changedPkgs[p]:
			n.kind = '~'
		case movedIn[p] > 0:
			n.kind = '≡'
		}
		if !n.pkg || !n.impacted {
			continue
		}
		if w, ok := r.weights[p]; ok {
			n.lines = append(n.lines, fmt.Sprintf("+%d −%d", w.add, w.del))
		}
		if s, ok := r.surfaces[p]; ok && !s.empty() {
			n.lines = append(n.lines, fmt.Sprintf("api +%d −%d ~%d", len(s.added), len(s.removed), len(s.changed)))
		}
		var sig []string
		if k := movedIn[p]; k > 0 {
			sig = append(sig, fmt.Sprintf("≡%d←%s", k, strings.Join(slices.Sorted(keys(movedFrom[p])), ",")))
		}
		if is := icons[p]; len(is) > 0 {
			slices.Sort(is)
			sig = append(sig, strings.Join(slices.Compact(is), ""))
		}
		if k := extNew[p]; k > 0 {
			sig = append(sig, fmt.Sprintf("◆%d", k))
		}
		if len(sig) > 0 {
			n.lines = append(n.lines, strings.Join(sig, " "))
		}
	}
	root := nodes["."]
	var fix func(n *cnode) int
	fix = func(n *cnode) int {
		for _, c := range n.children {
			n.lane = min(n.lane, fix(c))
		}
		slices.SortFunc(n.children, func(a, b *cnode) int {
			if a.lane != b.lane {
				return a.lane - b.lane
			}
			return strings.Compare(a.name, b.name)
		})
		// Collapse a chain of plain directories: internal → source.
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

func setOf(ss []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func width(s string) int { return utf8.RuneCountInString(s) }

// title is what goes in the top border: the marker, the name.
func (n *cnode) title() string {
	t := n.name
	if n.kind != ' ' {
		t = string(n.kind) + " " + t
	}
	return t
}

// minWidth is the narrowest the box can be on its own: its title (plus
// the folded count) and its lines, or a child's minimum.
func (n *cnode) minWidth() int {
	// The top border holds the title, the in-port markers and the folded count.
	w := width(n.title()) + 5 + n.portW[0] + 2
	if n.folded > 0 {
		w += width(fmt.Sprintf(" ⋯%d", n.folded))
	}
	w = max(w, n.portW[1]+4)
	for _, l := range n.lines {
		w = max(w, width(l)+4)
	}
	for _, c := range n.children {
		w = max(w, c.minWidth()+4)
	}
	return w
}

// layout places n at (x, y) with exactly width w, children flowing in rows
// inside it and stretched to fill each row. gapY is the blank rows
// between child rows and above the first one, which routed lines need.
func (n *cnode) layout(x, y, w, gapY int) {
	n.x, n.y, n.w = x, y, w
	inner := w - 4 // border and one column of padding each side
	cy := y + 1 + len(n.lines)
	if len(n.children) > 0 {
		cy += gapY
	}
	var row []*cnode
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
			c.layout(cx, cy, cw+add, gapY)
			cx += cw + add + 1
			rowH = max(rowH, c.h)
		}
		for _, c := range row {
			c.stretchTo(rowH)
		}
		cy += rowH + gapY
		row, rowW = nil, 0
	}
	for _, c := range n.children {
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
	flush() // with gapY > 0 the last row keeps its gap below: routes run there
	n.h = cy - y + 1
}

// pref is the width a box asks for when its parent has inner columns:
// its minimum, grown to fit its children side by side when there is room.
func (n *cnode) pref(inner int) int {
	w := n.minWidth()
	if len(n.children) > 0 {
		sum := -1
		for _, c := range n.children {
			sum += c.pref(inner-4) + 1
		}
		w = max(w, sum+4)
	}
	return min(w, inner)
}

// stretchTo makes a box as tall as its row; its own content stays at the top.
func (n *cnode) stretchTo(h int) {
	if n.h < h {
		n.h = h
	}
}

// ---- the grid

type style struct {
	fg   int // 0 default, else 31..37
	bold bool
	dim  bool
}

type cell struct {
	r     rune
	st    style
	kind  byte // ' ' free, 't' text, 'h' horizontal border, 'v' vertical border, 'c' corner, 'L' line
	lineH bool // a line cell running horizontally (else vertically)
	box   *cnode
}

type grid struct {
	w, h  int
	cells [][]cell
}

func newGrid(w, h int) *grid {
	g := &grid{w: w, h: h, cells: make([][]cell, h)}
	for y := range g.cells {
		g.cells[y] = make([]cell, w)
		for x := range g.cells[y] {
			g.cells[y][x] = cell{r: ' ', kind: ' '}
		}
	}
	return g
}

func (g *grid) set(x, y int, r rune, st style, kind byte, b *cnode) {
	if x < 0 || y < 0 || x >= g.w || y >= g.h {
		return
	}
	g.cells[y][x] = cell{r: r, st: st, kind: kind, box: b}
}

func (g *grid) text(x, y int, s string, st style, b *cnode) int {
	for _, r := range s {
		g.set(x, y, r, st, 't', b)
		x++
	}
	return x
}

func kindStyle(k rune) style {
	switch k {
	case '+':
		return style{fg: 32}
	case '~':
		return style{fg: 33}
	case '−':
		return style{fg: 31}
	case '≡':
		return style{fg: 36}
	}
	return style{dim: true}
}

func (g *grid) drawBox(n *cnode, heavy bool) {
	// Borders stay neutral, so colour belongs to the edges; the change
	// kind colours only the title. (Yellow borders for changed packages
	// read as yellow "changed" edges.)
	st := style{}
	if !n.impacted {
		st = style{dim: true}
	}
	h, v, tl, tr, bl, br := '─', '│', '┌', '┐', '└', '┘'
	if heavy {
		h, v, tl, tr, bl, br = '━', '┃', '┏', '┓', '┗', '┛'
		st.bold = true
	}
	x0, y0, x1, y1 := n.x, n.y, n.x+n.w-1, n.y+n.h-1
	for x := x0 + 1; x < x1; x++ {
		g.set(x, y0, h, st, 'h', n)
		g.set(x, y1, h, st, 'h', n)
	}
	for y := y0 + 1; y < y1; y++ {
		g.set(x0, y, v, st, 'v', n)
		g.set(x1, y, v, st, 'v', n)
	}
	g.set(x0, y0, tl, st, 'c', n)
	g.set(x1, y0, tr, st, 'c', n)
	g.set(x0, y1, bl, st, 'c', n)
	g.set(x1, y1, br, st, 'c', n)
	tst := kindStyle(n.kind)
	tst.bold = n.impacted && n.pkg
	tst.dim = !n.impacted
	end := g.text(x0+2, y0, " "+abbrev(n.title(), n.w-6)+" ", tst, n)
	if n.folded > 0 {
		f := fmt.Sprintf(" ⋯%d ", n.folded)
		if fx := x1 - width(f); fx > end {
			g.text(fx, y0, f, style{dim: true}, n)
		}
	}
	for i, l := range n.lines {
		g.text(x0+2, y0+1+i, abbrev(l, n.w-4), style{}, n)
	}
	for _, c := range n.children {
		g.drawBox(c, false)
	}
}

func (g *grid) String(color bool) string {
	var b strings.Builder
	for _, row := range g.cells {
		var line strings.Builder
		cur := style{}
		for _, c := range row {
			if color && c.st != cur {
				line.WriteString(sgr(c.st))
				cur = c.st
			}
			line.WriteRune(c.r)
		}
		if color && cur != (style{}) {
			line.WriteString("\x1b[0m")
		}
		b.WriteString(strings.TrimRight(line.String(), " "))
		b.WriteByte('\n')
	}
	return b.String()
}

func sgr(st style) string {
	s := "\x1b[0"
	if st.bold {
		s += ";1"
	}
	if st.dim {
		s += ";2"
	}
	if st.fg != 0 {
		s += fmt.Sprintf(";%d", st.fg)
	}
	return s + "m"
}

// ---- edges

type edgeMode string

const (
	edgePorts edgeMode = "ports"
	edgeLines edgeMode = "lines"
	edgeFocus edgeMode = "focus"
)

// canvasEdges are the internal edges the change adds or removes, in risk
// order, numbered from 1.
func (r *result) canvasEdges() []edgeChange {
	var es []edgeChange
	for _, e := range r.edges {
		if internal(e.to) {
			es = append(es, e)
		}
	}
	return append(es, r.changedEdges...)
}

// portLabel is an edge's marker: a plain number. Circled digits (①) were
// tried first: they are East Asian "ambiguous" width, and Ghostty and VHS
// draw them two columns wide, which breaks the borders they sit on.
func portLabel(n int) string { return fmt.Sprint(n) }

// portMark is the marker on the border: the number, with "!" when the
// edge goes upward or skips a closed lane, so plain text shows it too.
func portMark(n int, e edgeChange) string {
	if e.verdict == "up" || e.verdict == "skip" {
		return portLabel(n) + "!"
	}
	return portLabel(n)
}

// maxPorts is the most edges that get a marker each; beyond it each box
// gets counts instead (▾ out, ▴ in), and the legend groups by source.
const maxPorts = 20

func edgeStyle(e edgeChange) style {
	switch edgeStatus(e) {
	case "added":
		return style{fg: 32, bold: true}
	case "removed":
		return style{fg: 31}
	case "changed":
		return style{fg: 33, bold: true}
	}
	return style{dim: true}
}

// edgeStatus is what the change did to an edge. It decides the colour
// (the diff preview's: green, red, yellow, faint) and, as a second cue
// without colour, the line: ━ added, ┄ removed, ═ changed, ─ unchanged.
func edgeStatus(e edgeChange) string {
	switch {
	case e.verdict == "same":
		return "unchanged"
	case e.changed:
		return "changed"
	case e.added:
		return "added"
	}
	return "removed"
}

// lineGlyphs are a status's straight runs and corners, as
// horizontal, vertical, ┌ ┐ └ ┘.
var lineGlyphs = map[string][6]rune{
	"added":     {'━', '┃', '┏', '┓', '┗', '┛'},
	"removed":   {'┄', '┆', '┌', '┐', '└', '┘'},
	"changed":   {'═', '║', '╔', '╗', '╚', '╝'},
	"unchanged": {'─', '│', '┌', '┐', '└', '┘'},
}

// mark writes a label on a box's top (in) or bottom (out) border, after
// the title on top, left to right; it drops what does not fit.
func (g *grid) mark(next map[*cnode][2]int, b *cnode, top bool, label string, st style) {
	i, y := 1, b.y+b.h-1
	if top {
		i, y = 0, b.y
	}
	x := next[b][i]
	if x == 0 {
		x = b.x + 2
		if top {
			x = b.x + 2 + width(abbrev(b.title(), b.w-6)) + 3
		}
	}
	if x+width(label) > b.x+b.w-2 {
		return
	}
	for _, r := range label {
		g.set(x, y, r, st, 'h', b)
		x++
	}
	p := next[b]
	p[i] = x + 1 // a border cell between markers: 1─3, not 13
	next[b] = p
}

// placePorts writes each edge's number on the source box's bottom border
// and the target box's top border, after the title.
func (g *grid) placePorts(boxes map[string]*cnode, es []edgeChange) {
	next := map[*cnode][2]int{}
	for i, e := range es {
		st := edgeStyle(e)
		l := portMark(i+1, e)
		g.mark(next, boxes[e.from], false, l, st)
		g.mark(next, boxes[e.to], true, l, st)
	}
}

// placeCounts is the dense form: per box, how many changed edges leave
// (▾N) and arrive (▴N), red when one of them goes upward.
func (g *grid) placeCounts(boxes map[string]*cnode, es []edgeChange) {
	out, in := map[*cnode]int{}, map[*cnode]int{}
	worst := map[*cnode]style{}
	var order []*cnode
	for _, e := range es {
		for _, b := range []*cnode{boxes[e.from], boxes[e.to]} {
			if _, ok := worst[b]; !ok {
				order = append(order, b)
				worst[b] = style{fg: 32, bold: true}
			}
			if e.verdict == "up" || e.verdict == "skip" {
				worst[b] = edgeStyle(e)
			}
		}
		out[boxes[e.from]]++
		in[boxes[e.to]]++
	}
	next := map[*cnode][2]int{}
	for _, b := range order {
		if in[b] > 0 {
			g.mark(next, b, true, fmt.Sprintf("▴%d", in[b]), worst[b])
		}
		if out[b] > 0 {
			g.mark(next, b, false, fmt.Sprintf("▾%d", out[b]), worst[b])
		}
	}
}

// route draws one edge as an orthogonal line from a border of the source
// box to a border of the target, through free cells; crossing a border
// (at a right angle) or another line costs extra, and turns cost a little,
// so the routes stay straight. It returns false when no route exists.
func (g *grid) route(from, to *cnode, e edgeChange) bool {
	type state struct{ x, y, d int }
	dx := []int{1, 0, -1, 0}
	dy := []int{0, 1, 0, -1}
	onBorder := func(b *cnode, x, y int) bool {
		c := g.cells[y][x]
		return c.box == b && (c.kind == 'h' || c.kind == 'v')
	}
	const inf = 1 << 30
	dist := make([]int, g.w*g.h*4)
	prev := make([]int32, g.w*g.h*4)
	for i := range dist {
		dist[i] = inf
		prev[i] = -1
	}
	idx := func(s state) int { return (s.y*g.w+s.x)*4 + s.d }
	// A bucketed queue: costs are small integers.
	buckets := map[int][]state{}
	push := func(s state, c int, p int) {
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
				// leave a border at a right angle
				if (g.cells[y][x].kind == 'h') == (d%2 == 1) {
					push(state{x, y, d}, 0, -1)
				}
			}
		}
	}
	end := -1
	for c := 0; c < 4000 && end < 0; c++ {
		for len(buckets[c]) > 0 && end < 0 {
			s := buckets[c][0]
			buckets[c] = buckets[c][1:]
			if dist[idx(s)] != c {
				continue
			}
			if onBorder(to, s.x, s.y) && dist[idx(s)] > 0 {
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
				case 't', 'c':
					continue
				case 'h', 'v':
					if (cc.kind == 'h') != (nd%2 == 1) {
						continue // never run along a border
					}
					if cc.box != to {
						step += 6
					}
				case 'L':
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
	st := edgeStyle(e)
	status := edgeStatus(e)
	for i, s := range cells {
		c := &g.cells[s.y][s.x]
		in := s.d // directions: 0 right 1 down 2 left 3 up
		out := in
		if i+1 < len(cells) {
			out = cells[i+1].d
		}
		switch {
		case i == len(cells)-1:
			// The arrow point, on the target's border.
			*c = cell{r: []rune("▸▾◂▴")[in], st: st, kind: c.kind, box: c.box}
		case c.kind == 'h' || c.kind == 'v' || c.kind == 'c' || c.kind == 'L':
			// Borders stay whole and an earlier line stays unbroken: this
			// line stops on one side and goes on at the other.
		default:
			*c = cell{r: bend(in, out, status), st: st, kind: 'L', lineH: out%2 == 0, box: c.box}
		}
	}
	return true
}

// bend is the glyph for a line cell entered going in and left going out.
func bend(in, out int, status string) rune {
	g := lineGlyphs[status]
	if in%2 == out%2 {
		if in%2 == 0 {
			return g[0]
		}
		return g[1]
	}
	// corners by (in, out): 0 right 1 down 2 left 3 up
	switch [2]int{in, out} {
	case [2]int{2, 1}, [2]int{3, 0}:
		return g[2]
	case [2]int{0, 1}, [2]int{3, 2}:
		return g[3]
	case [2]int{2, 3}, [2]int{1, 0}:
		return g[4]
	}
	return g[5]
}

// ---- the view

// dimExcept fades every box and its text except the kept ones; lines stay.
func (g *grid) dimExcept(keep map[*cnode]bool) {
	for y := range g.cells {
		for x := range g.cells[y] {
			c := &g.cells[y][x]
			if c.kind != 'L' && c.box != nil && !keep[c.box] && !strings.ContainsRune("━┃┏┓┗┛┣┫┳┻╋─│┌┐└┘├┤┬┴┼┄┆▸▾◂▴", c.r) {
				c.st = style{dim: true}
			}
		}
	}
}

func boolKeys(m map[edgeKey][]site) map[edgeKey]bool {
	b := map[edgeKey]bool{}
	for k := range m {
		b[k] = true
	}
	return b
}

func boxesOf(n *cnode, m map[string]*cnode) map[string]*cnode {
	m[n.path] = n
	for _, c := range n.children {
		boxesOf(c, m)
	}
	return m
}

// canvasView renders the canvas at width w with the given edge mode;
// selected names the focus box (default: the riskiest edge's source).
func (r *result) canvasView(o *out, repoName string, mode edgeMode, selected string, color bool) {
	root := r.canvasTree(repoName)
	boxes := boxesOf(root, map[string]*cnode{})
	// A changed edge may point at a folded package: it is counted, not drawn.
	var es []edgeChange
	hidden := 0
	for _, e := range r.canvasEdges() {
		if boxes[e.from] == nil || boxes[e.to] == nil {
			hidden++
			continue
		}
		es = append(es, e)
	}
	if selected == "" && len(es) > 0 {
		selected = es[0].from
	}
	legendW := 0
	if o.w >= 110 && mode != edgeLines {
		legendW = 40
	}
	cw := o.w - legendW
	gapY := 0
	if mode != edgePorts {
		gapY = 1
	}
	if mode == edgePorts && len(es) <= maxPorts {
		// Reserve border room for every marker ("1─3─12"), so none is dropped.
		for i, e := range es {
			l := width(portMark(i+1, e)) + 1
			boxes[e.from].portW[1] += l
			boxes[e.to].portW[0] += l
		}
	}
	root.layout(0, 0, cw, gapY)
	g := newGrid(cw, root.h)
	g.drawBox(root, false)
	var legend []legendItem
	plain := func(f string, a ...any) { legend = append(legend, legendItem{text: fmt.Sprintf(f, a...)}) }
	edge := func(label string, e edgeChange) {
		legend = append(legend, legendItem{edgeLegend(label, e), edgeStyle(e)})
	}
	var unrouted []string
	switch mode {
	case edgePorts:
		if len(es) <= maxPorts {
			g.placePorts(boxes, es)
			for i, e := range es {
				edge(portLabel(i+1), e)
			}
			break
		}
		g.placeCounts(boxes, es)
		plain("%d changed edges: ▾ out ▴ in per box", len(es))
		var from []string
		to := map[string][]string{}
		for _, e := range es {
			if to[e.from] == nil {
				from = append(from, e.from)
			}
			mark := ""
			if e.verdict == "up" || e.verdict == "skip" {
				mark = "⚠"
			}
			switch edgeStatus(e) {
			case "removed":
				mark += "−"
			case "changed":
				mark += "~"
			}
			to[e.from] = append(to[e.from], mark+pkgLabel(lastSeg(e.to)))
		}
		for _, f := range from {
			plain("%s ▸ %s", pkgLabel(f), strings.Join(to[f], " "))
		}
	case edgeLines, edgeFocus:
		if mode == edgeFocus {
			if b := boxes[selected]; b != nil {
				g.drawBox(b, true)
			}
		}
		for i, e := range es {
			if mode == edgeFocus && e.from != selected && e.to != selected {
				continue
			}
			if !g.route(boxes[e.from], boxes[e.to], e) {
				unrouted = append(unrouted, edgeLegend(fmt.Sprint(i+1), e))
			}
		}
		var context []edgeChange
		if mode == edgeFocus {
			// Unchanged edges fill up to six lines; a hub with many changed
			// edges gets none, or its box drowns in lines.
			room := 6
			for _, e := range es {
				if e.from == selected || e.to == selected {
					room--
				}
			}
			// The selected box's unchanged edges too, light, so its place
			// in the graph shows; everything not touching it is dimmed.
			for _, k := range sortedKeys(boolKeys(r.head.edges)) {
				b, old := r.base.edges[k]
				if !old || fingerprint(b) != fingerprint(r.head.edges[k]) || !internal(k.to) || (k.from != selected && k.to != selected) {
					continue
				}
				if boxes[k.from] == nil || boxes[k.to] == nil || len(context) >= room {
					continue
				}
				e := edgeChange{edgeKey: k, added: true, verdict: "same"}
				if g.route(boxes[k.from], boxes[k.to], e) {
					context = append(context, e)
				}
			}
			keep := map[*cnode]bool{boxes[selected]: true}
			for _, e := range append(slices.Clone(es), context...) {
				if e.from == selected || e.to == selected {
					keep[boxes[e.from]], keep[boxes[e.to]] = true, true
				}
			}
			g.dimExcept(keep)
		}
		if mode == edgeLines {
			for _, e := range es {
				edge("", e)
			}
		}
		if mode == edgeFocus {
			plain("selected: %s", selected)
			n := 0
			for _, e := range es {
				if e.from == selected || e.to == selected {
					edge("", e)
				} else {
					n++
				}
			}
			for _, e := range context {
				edge("", e)
			}
			if n > 0 {
				plain("%d more changed edges: select their box", n)
			}
		}
	}
	unchanged := 0
	for k := range r.head.edges {
		if b, ok := r.base.edges[k]; ok && fingerprint(b) == fingerprint(r.head.edges[k]) && internal(k.to) && boxes[k.from] != nil && boxes[k.to] != nil && boxes[k.from].impacted && boxes[k.to].impacted {
			unchanged++
		}
	}
	plain("%d unchanged edges between shown boxes", unchanged)
	if hidden > 0 {
		plain("%d changed edges into folded packages", hidden)
	}
	for _, u := range unrouted {
		plain("not routed: %s", u)
	}
	plain("edges ━ added ┄ removed ═ changed ─ unchanged")
	plain("+ new ~ changed − gone ≡ moved in · $ env ⇄ http ≣ sql » exec ▤ fs ⊢ flag ◆ dep")

	rows := strings.Split(strings.TrimRight(g.String(color), "\n"), "\n")
	if legendW > 0 {
		for i := range max(len(rows), len(legend)) {
			left := ""
			if i < len(rows) {
				left = rows[i]
			}
			pad := cw - visibleWidth(left)
			right := ""
			if i < len(legend) {
				right = " " + legend[i].render(legendW-1, color)
			}
			o.lines = append(o.lines, left+strings.Repeat(" ", max(pad, 0))+right)
		}
		return
	}
	o.lines = append(o.lines, rows...)
	for _, l := range legend {
		o.lines = append(o.lines, " "+l.render(o.w-1, color))
	}
}

func edgeLegend(label string, e edgeChange) string {
	status := edgeStatus(e)
	arrow := string(lineGlyphs[status][0]) + "▸"
	tag := ""
	switch e.verdict {
	case "up":
		tag = "  ✕ upward"
	case "skip":
		tag = "  ⚠ " + e.why
	}
	if status != "added" {
		tag += "  " + status
	}
	s := fmt.Sprintf("%s %s %s%s", pkgLabel(e.from), arrow, pkgLabel(e.to), tag)
	if label != "" {
		s = label + " " + s
	}
	return s
}

// legendItem is one legend line and its colour.
type legendItem struct {
	text string
	st   style
}

func (l legendItem) render(w int, color bool) string {
	t := abbrev(l.text, w)
	if !color || l.st == (style{}) {
		return t
	}
	return sgr(l.st) + t + "\x1b[0m"
}

// pkgLabel names a package in a legend: its path without "internal/".
func pkgLabel(p string) string { return strings.TrimPrefix(p, labelPrefix) }

// labelPrefix is cut from package names in legends: "internal/" for Go,
// the -src folder for TypeScript.
var labelPrefix = "internal/"

// visibleWidth counts runes outside ANSI escapes.
func visibleWidth(s string) int {
	n, esc := 0, false
	for _, r := range s {
		switch {
		case r == '\x1b':
			esc = true
		case esc && r == 'm':
			esc = false
		case !esc:
			n++
		}
	}
	return n
}

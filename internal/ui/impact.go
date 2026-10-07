package ui

import (
	"context"
	"fmt"
	"path"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/arch"
)

// The Impact tab: what the thread's branch did to its repository's shape
// (internal/source/arch). It opens on Look here first, the changes that
// alter the shape, riskiest first, and under it the canvas of the
// impacted packages. The tab is named for what it answers rather than for
// architecture alone: the call-stack view (flows) is meant to join it.

type archMsg struct {
	key string
	res *arch.Result
}

// archResult is the selected thread's last architecture read, when one
// was read; the tab needs a worktree, as Files does.
func (m Model) archResult() (deck.Thread, *arch.Result, bool) {
	t, ok := m.diffThread()
	if !ok || m.opt.Arch == nil {
		return t, nil, false
	}
	r, ok := m.archs[diffKey(t)]
	return t, r, ok
}

// readArch reads the selected thread's architecture, again when force is
// set (the reader answers from its cache until HEAD moves), else only when
// the selection moved to another thread. One read runs at a time; a move
// meanwhile reads again when it ends.
func (m *Model) readArch(force bool) tea.Cmd {
	t, ok := m.diffThread()
	if m.opt.Arch == nil || !ok {
		return nil
	}
	k := diffKey(t)
	if !force && k == m.archSel {
		return nil
	}
	if m.archReading {
		m.archAgain = true
		return nil
	}
	m.archReading, m.archSel = true, k
	read := m.opt.Arch
	return func() tea.Msg { return archMsg{key: k, res: read(context.Background(), t)} }
}

func (m *Model) setArch(msg archMsg) tea.Cmd {
	m.archReading = false
	m.archs[msg.key] = msg.res
	if m.archAgain {
		m.archAgain = false
		return m.readArch(true)
	}
	return nil
}

// impactLabel is the tab's label: Impact and the number of things to look
// at, ≡ for a change that only moved code, … until the first read.
func (m Model) impactLabel() string {
	_, r, ok := m.archResult()
	switch {
	case !ok:
		return "Impact …"
	case r.Note != "":
		return "Impact"
	case r.PureMove():
		return "Impact ≡"
	}
	return fmt.Sprintf("Impact %d", len(r.Findings))
}

// Look here first shows this many findings at the drawer's normal height,
// and this many at full height; a line counts the rest.
const (
	findingsNormal = 5
	findingsFull   = 12
)

// impactTab fills the Impact tab. Its selection shows only while the
// drawer has the focus: at rest the canvas shows its markers.
func (m Model) impactTab(d *drawer) {
	t, r, ok := m.archResult()
	if !ok {
		d.line(" " + dim.Render("reading the change's shape…"))
		return
	}
	o := impactOpts{limit: findingsNormal, cache: m.icache}
	if m.effectiveSize() == sizeFull {
		o.limit = findingsFull
	}
	if m.iall {
		o.limit = -1
	}
	if m.dfocus && m.iselKey == diffKey(t) {
		o.sel = m.isel
	}
	d.impact = impactLines(d, r, o)
}

// impactOpts are how impactLines draws: how many findings (-1 all), the
// selected box ("" none), and the canvases already drawn.
type impactOpts struct {
	limit int
	sel   string
	cache *canvasCache
}

// impactSite is where ↵ on a row goes: a file, at a line when known, in
// the base commit for something the change removed.
type impactSite struct {
	file string
	line int
	base bool
}

// impactLayout is what the tab drew, for keys and clicks. Its drawer's
// stops are Look here first's rows (and the line that unfolds the rest),
// then the canvas's boxes in reading order, then the selected box's
// detail rows.
type impactLayout struct {
	res *arch.Result
	c   *canvas
	// top and left are the canvas's first drawer line and column.
	top, left int
	findings  int     // stops before the boxes
	boxes     []*cbox // the box stops, in order
	sel       string  // the selected box, if drawn
	sites     []impactSite
}

// boxStop is the stop of the box at path p, or -1.
func (l *impactLayout) boxStop(p string) int {
	for i, b := range l.boxes {
		if b.path == p {
			return l.findings + i
		}
	}
	return -1
}

// canvasCache keeps the last canvases drawn, by result, width and
// selection: a layout is computed many times per key, and routing a
// selection's lines is the costly part.
type canvasCache struct {
	keys []canvasKey
	vals []canvasEntry
}

type canvasKey struct {
	res *arch.Result
	w   int
	sel string
}

type canvasEntry struct {
	c        *canvas
	unrouted int
}

const canvasCacheSize = 8

func (cc *canvasCache) get(r *arch.Result, w int, sel string) (*canvas, int) {
	k := canvasKey{r, w, sel}
	if cc != nil {
		for i, kk := range cc.keys {
			if kk == k {
				return cc.vals[i].c, cc.vals[i].unrouted
			}
		}
	}
	c := newCanvas(r, w)
	unrouted := 0
	if sel != "" && c.boxes[sel] != nil {
		unrouted = c.focus(r, sel)
	}
	if cc != nil {
		cc.keys = append([]canvasKey{k}, cc.keys...)
		cc.vals = append([]canvasEntry{{c, unrouted}}, cc.vals...)
		if len(cc.keys) > canvasCacheSize {
			cc.keys, cc.vals = cc.keys[:canvasCacheSize], cc.vals[:canvasCacheSize]
		}
	}
	return c, unrouted
}

// impactLines writes a result into d: the summary, Look here first, the
// canvas, and beside or below it the legend, or the selected box's detail.
func impactLines(d *drawer, r *arch.Result, o impactOpts) *impactLayout {
	w := d.width
	if r.Note != "" {
		d.line(" " + dim.Render(r.Note))
		return nil
	}
	lay := &impactLayout{res: r}
	cw := canvasWidth(w)
	c, _ := o.cache.get(r, cw, "")
	if o.sel != "" && c.boxes[o.sel] != nil && c.boxes[o.sel].pkg {
		lay.sel = o.sel
	}
	unrouted := 0
	if lay.sel != "" {
		c, unrouted = o.cache.get(r, cw, lay.sel)
	}
	lay.c = c
	pre := c.labelPrefix(r)
	d.line(impactSummary(r, w))
	if r.PureMove() {
		d.line(" " + dim.Render(fmt.Sprintf("%d files and %d declarations moved, %d renamed; no edge, API or touchpoint changed",
			r.Moves.Files, r.Moves.Decls, r.Moves.Renamed)))
	}
	if len(r.Findings) == 0 && !r.PureMove() {
		d.line(" " + dim.Render("nothing changed the shape"))
	}
	for i, f := range r.Findings {
		if i == o.limit {
			act := action{kind: actFindMore}
			d.stopLine(" "+dim.Render(fmt.Sprintf("⋯ %d more · ↵ shows all", len(r.Findings)-o.limit)), act, []zone{{0, w, act}})
			break
		}
		act := action{kind: actFinding, n: i}
		d.stopLine(findingLine(f, i, pre, w), act, []zone{{0, w, act}})
	}
	if o.limit < 0 && len(r.Findings) > findingsNormal {
		act := action{kind: actFindMore}
		d.stopLine(" "+dim.Render("⋯ fewer"), act, []zone{{0, w, act}})
	}
	lay.findings = len(d.stops)
	d.line("")

	// The boxes are stops in reading order; the drawer cursor on one is
	// the selection, drawn on the canvas rather than as a highlight.
	lay.top, lay.left = len(d.lines), 1
	lay.boxes = c.selectable()
	for _, b := range lay.boxes {
		d.nextStop()
		d.stops = append(d.stops, stop{line: lay.top + b.y, act: action{kind: actBox, n: len(d.stops) - lay.findings}})
	}

	// The side is the legend, or the selection's detail: below the
	// canvas, or beside it when wide enough. Its rows start with a
	// blank column, where the cursor's ▸ goes.
	rows := c.rows()
	beside := w >= legendBesideMin
	sideW := w - 1
	if beside {
		sideW = legendWidth
	}
	var side []detailRow
	if lay.sel != "" {
		side = impactDetail(r, c, lay.sel, unrouted, pre, sideW-1)
	} else {
		for _, l := range c.legend(r, sideW-1) {
			side = append(side, detailRow{text: l})
		}
	}
	for i := range side {
		side[i].text = " " + side[i].text
	}
	// side writes detail row i after prefix (w columns in), as a stop when
	// it goes somewhere.
	sideLine := func(prefix string, x int, row detailRow) {
		if row.site == nil {
			d.line(prefix + row.text)
			return
		}
		lay.sites = append(lay.sites, *row.site)
		act := action{kind: actSite, n: len(lay.sites) - 1}
		text := fit(row.text, sideW)
		if d.nextStop() {
			text = highlight("▸"+ansi.Cut(text, 1, sideW), d.light)
		}
		d.stops = append(d.stops, stop{line: len(d.lines), act: act})
		d.lines = append(d.lines, dline{text: ansi.Truncate(prefix+text, w, "…"), zones: []zone{{x, x + sideW, act}}})
	}
	if beside {
		gap := strings.Repeat(" ", max(w-1-c.g.w-legendWidth, 0))
		for i := range max(len(rows), len(side)) {
			left := strings.Repeat(" ", c.g.w)
			if i < len(rows) {
				left = rows[i]
			}
			prefix := " " + left + gap
			if i < len(side) {
				sideLine(prefix, 1+c.g.w+len(gap), side[i])
			} else {
				d.line(prefix)
			}
		}
	} else {
		for _, row := range rows {
			d.line(" " + row)
		}
		for _, row := range side {
			sideLine("", 0, row)
		}
	}
	if r.Inferred {
		d.line(" " + dim.Render(abbrev("layers inferred: declare them in .config/dev.json, x-herdr-deck.architecture", w-2)))
	}
	return lay
}

// detailRow is one line of the selection's detail; site, when set, is
// where ↵ on it goes.
type detailRow struct {
	text string
	site *impactSite
}

// impactDetail describes the selected box: its change, its edges in and
// out (changed first, each with its status and where it is written), its
// API changes, new touchpoints and changed files.
func impactDetail(r *arch.Result, c *canvas, sel string, unrouted int, pre string, w int) []detailRow {
	pk, _ := r.Package(sel)
	b := c.boxes[sel]
	name := func(p string) string {
		if s := arch.Short(p); s != p {
			return s // a third-party or standard-library package
		}
		if s := strings.TrimPrefix(p, pre); s != "" {
			return s
		}
		return p
	}
	var out []detailRow
	add := func(text string, site *impactSite) { out = append(out, detailRow{text: abbrev(text, w), site: site}) }
	head := b.titleStyle().style().Render(b.title())
	if b.name != pk.Path {
		head += dim.Render(" · " + pk.Path)
	}
	if len(r.Lanes) > pk.Lane && !r.Inferred {
		head += dim.Render(" · " + r.Lanes[pk.Lane].Name)
	}
	add(head, nil)
	switch {
	case pk.Files == 1:
		add(dim.Render(fmt.Sprintf("+%d −%d in 1 file", pk.Added, pk.Deleted)), nil)
	case pk.Files > 1:
		add(dim.Render(fmt.Sprintf("+%d −%d in %d files", pk.Added, pk.Deleted, pk.Files)), nil)
	}
	site := func(e arch.Edge) (*impactSite, string) {
		if len(e.Sites) == 0 {
			return nil, ""
		}
		s := e.Sites[0]
		return &impactSite{file: s.File, line: s.Line, base: s.Base}, fmt.Sprintf("%s:%d", path.Base(s.File), s.Line)
	}
	edgeRow := func(e arch.Edge, out bool) {
		st := edgeStyle(e.Status)
		line := edgeLine(e.Status)
		text := line + "▸ " + name(e.To)
		if !out {
			text = "◂" + line + " " + name(e.From)
		}
		if c.boxes[e.From] == nil || c.boxes[e.To] == nil {
			if e.Internal() {
				text += " (folded)"
			}
		}
		// Added is green and says no more, as in the legend.
		var tags []string
		if e.Status != arch.Added {
			tags = append(tags, edgeWord(e.Status))
		}
		switch {
		case e.Verdict == arch.VerdictUp:
			tags = append(tags, "✕ upward")
		case e.Verdict == arch.VerdictSkip:
			tags = append(tags, "⚠ "+e.Why)
		case e.Why != "":
			tags = append(tags, e.Why)
		}
		if e.MovedWithCode {
			tags = append(tags, "≡ moved with its code")
		}
		tag := strings.Join(tags, " ")
		s, where := site(e)
		left := st.style().Render(text) + "  " + dim.Render(tag)
		add(spread(left, dim.Render(where), w), s)
	}
	var outs, ins []arch.Edge
	for _, e := range r.Edges {
		switch sel {
		case e.From:
			outs = append(outs, e)
		case e.To:
			ins = append(ins, e)
		}
	}
	if len(outs)+len(ins) > 0 {
		add(dim.Render(fmt.Sprintf("── imports %d · imported by %d ──", len(outs), len(ins))), nil)
	}
	for _, e := range outs {
		edgeRow(e, true)
	}
	for _, e := range ins {
		edgeRow(e, false)
	}
	if unrouted > 0 {
		add(dim.Render(fmt.Sprintf("%d of its lines found no room", unrouted)), nil)
	}
	if !pk.API.Empty() {
		add(dim.Render("── api ──"), nil)
		for _, k := range pk.API.Added {
			add(okStyle.Render("+ ")+k, nil)
		}
		for _, k := range pk.API.Removed {
			add(failStyle.Render("− ")+k, nil)
		}
		for _, k := range pk.API.Changed {
			add(warnStyle.Render("~ ")+k, nil)
		}
	}
	var touches []arch.Touch
	for _, t := range r.Touches {
		if t.Package == sel {
			touches = append(touches, t)
		}
	}
	if len(touches) > 0 {
		add(dim.Render("── touchpoints ──"), nil)
		for _, t := range touches {
			target := t.Target
			if target == "" {
				target = "(dynamic)"
			}
			sign := okStyle.Render("+")
			if t.Removed {
				sign = failStyle.Render("−")
			}
			s := &impactSite{file: t.Site.File, line: t.Site.Line, base: t.Site.Base}
			add(spread(sign+" "+touchIcon[t.Kind]+" "+t.Kind+" "+target, dim.Render(fmt.Sprintf("%s:%d", path.Base(t.Site.File), t.Site.Line)), w), s)
		}
	}
	if len(pk.Changed) > 0 {
		add(dim.Render("── files ──"), nil)
		for _, f := range pk.Changed {
			add(path.Base(f), &impactSite{file: f, base: pk.Status == arch.Removed})
		}
	}
	return out
}

// The legend sits beside the canvas from legendBesideMin columns on,
// legendWidth wide; below it otherwise.
const (
	legendBesideMin = 110
	legendWidth     = 40
)

// canvasWidth is the canvas's width in a view w columns wide: one column
// of margin on the left, and the legend's beside it when wide enough.
func canvasWidth(w int) int {
	if w >= legendBesideMin {
		return w - 1 - legendWidth - 1
	}
	return w - 2
}

// impactSummary is the tab's first line: how many things to look at and
// their worst kinds, the edge counts, and at the right what was compared.
func impactSummary(r *arch.Result, w int) string {
	var up, skip int
	for _, e := range r.Edges {
		if e.Status == arch.Added {
			switch e.Verdict {
			case arch.VerdictUp:
				up++
			case arch.VerdictSkip:
				skip++
			}
		}
	}
	parts := []string{bold.Render(fmt.Sprintf("%d to look at", len(r.Findings)))}
	if r.PureMove() {
		parts = []string{workStyle.Bold(true).Render("≡ pure move")}
	}
	if up > 0 {
		parts = append(parts, failStyle.Bold(true).Render(fmt.Sprintf("✕ %d upward", up)))
	}
	if n := len(r.Cycles); n > 0 {
		parts = append(parts, failStyle.Render(fmt.Sprintf("↻ %d cycle", n)))
	}
	if skip > 0 {
		parts = append(parts, warnStyle.Render(fmt.Sprintf("⚠ %d skip", skip)))
	}
	a, rm, ch := r.ChangedEdges()
	parts = append(parts, fmt.Sprintf("edges %s %s %s", okStyle.Render(fmt.Sprintf("+%d", a)),
		failStyle.Render(fmt.Sprintf("−%d", rm)), warnStyle.Render(fmt.Sprintf("~%d", ch))))
	right := ""
	switch {
	case w >= 100:
		right = dim.Render(fmt.Sprintf("vs %s · %s", r.Base, langName(r.Language)))
	case w >= wideMin:
		right = dim.Render("vs " + r.Base)
	}
	return " " + spread(strings.Join(parts, "  "), right, w-2)
}

func langName(l string) string {
	if l == arch.LangTS {
		return "TypeScript"
	}
	return "Go"
}

// findingLine is one Look here first row: its number (· after 9), its
// glyph, what it is and why, and at the right where it is written.
func findingLine(f arch.Finding, i int, pre string, w int) string {
	num := dim.Render(" ·")
	if i < 9 {
		num = bold.Render(fmt.Sprintf("%2d", i+1))
	}
	glyph := findingGlyph(f)
	text := f.Text
	if pre != "" && f.Kind == arch.FindEdge {
		from, to, _ := strings.Cut(text, " → ")
		text = strings.TrimPrefix(from, pre) + " → " + strings.TrimPrefix(to, pre)
	}
	if f.Kind == arch.FindTouch || f.Kind == arch.FindAPI {
		text = strings.Replace(text, pre, "", 1)
	}
	why := f.Why
	switch {
	case f.Kind == arch.FindTouch:
		why = "in " + strings.TrimPrefix(strings.TrimPrefix(f.Why, "in "), pre)
	case f.Verdict == arch.VerdictUp:
		why = "upward"
	}
	left := num + " " + glyph + " " + text
	if why != "" {
		left += "  " + dim.Render(why)
	}
	right := ""
	if f.Site.File != "" && w >= wideMin {
		right = dim.Render(fmt.Sprintf("%s:%d", path.Base(f.Site.File), f.Site.Line))
	}
	return spread(left, right, w-1)
}

// findingGlyph marks a finding: ✕ upward, ⚠ a layer skip, ↻ a cycle, else
// its sign in the colour of what it did.
func findingGlyph(f arch.Finding) string {
	switch f.Verdict {
	case arch.VerdictUp:
		return failStyle.Bold(true).Render("✕")
	case arch.VerdictSkip:
		return warnStyle.Bold(true).Render("⚠")
	}
	switch f.Sign {
	case "↻", "−":
		return failStyle.Render(f.Sign)
	case "~":
		return warnStyle.Render(f.Sign)
	case "≡":
		return workStyle.Render(f.Sign)
	}
	return okStyle.Render(f.Sign)
}

package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/arch"
)

// ImpactPane is the Impact view on its own: `herdr-deck arch`, which the
// deck's O opens in a herdr pane zoomed to the whole tab. It shows what
// the drawer's Impact tab shows, with every finding and the room of the
// whole pane, and moves the same way: j k through the findings, the
// arrows or hjkl from box to box, n N through a box's edges, a click
// selects and a second one opens. With no main view above it, a selected
// edge's code shows below the canvas, or beside it when the pane is wide.
// With no Files tab beside it, ↵ opens files in the diff tool. q closes it.

// ImpactOptions wire the pane to the outside world.
type ImpactOptions struct {
	// Thread is whose branch it shows: its worktree, base, id and title.
	Thread deck.Thread
	// Read reads the branch's shape; it runs off the UI goroutine at
	// start and every Tick (zero is DefaultTick), answering from the
	// reader's cache until HEAD moves.
	Read func(context.Context, deck.Thread) *arch.Result
	Tick time.Duration
	// Diff, Patch and FileAt read the branch's changed files, one file's
	// diff and a file at a commit, for a selected edge's code; any may be
	// nil. Split draws that code in the split layout when wide enough.
	Diff   func(context.Context, deck.Thread) deck.Diff
	Patch  func(ctx context.Context, t deck.Thread, mergeBase string, f deck.DiffFile) deck.Patch
	FileAt func(ctx context.Context, t deck.Thread, rev, path string) ([]byte, error)
	Split  bool
	// OpenDiff opens the diff tool for the worktree at path against base,
	// limited to files. Tests replace it so no diff tool is ever run.
	OpenDiff func(path, base string, files []string) error
}

// ImpactPane is the Bubble Tea model of the Impact view's own pane.
type ImpactPane struct {
	opt           ImpactOptions
	res           *arch.Result
	diff          deck.Diff
	diffRead      bool
	reading       bool
	width, height int
	light         bool
	off, cur      int
	sel, edge     string
	esite         int
	all           bool
	status        string
	cache         *canvasCache
	ecode         edgeCodes
	escroll       edgeScroll
}

type paneReadMsg struct {
	res  *arch.Result
	diff *deck.Diff // nil when it was not read again
}
type paneTickMsg struct{}

// paneBeside is the narrowest pane that shows an edge's code beside the
// canvas rather than below it.
const paneBeside = 180

// NewImpactPane returns the pane, reading on Init.
func NewImpactPane(opt ImpactOptions) ImpactPane {
	if opt.Tick == 0 {
		opt.Tick = DefaultTick
	}
	// Init's read is running from the start: Init cannot keep state.
	return ImpactPane{opt: opt, width: defaultWidth, height: defaultHeight, cache: &canvasCache{}, reading: opt.Read != nil}
}

func (p ImpactPane) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.RequestBackgroundColor, p.tick()}
	if p.opt.Read != nil {
		cmds = append(cmds, p.readCmd())
	}
	return tea.Batch(cmds...)
}

// readCmd reads the branch's shape and, the first time and while an edge
// is selected (only its code needs them), its changed files, side by side.
func (p ImpactPane) readCmd() tea.Cmd {
	read, t := p.opt.Read, p.opt.Thread
	diff := p.opt.Diff
	if p.diffRead && p.edge == "" {
		diff = nil
	}
	return func() tea.Msg {
		var msg paneReadMsg
		done := make(chan struct{})
		if diff != nil {
			go func() {
				d := diff(context.Background(), t)
				msg.diff = &d
				close(done)
			}()
		} else {
			close(done)
		}
		msg.res = read(context.Background(), t)
		<-done
		return msg
	}
}

func (p *ImpactPane) read() tea.Cmd {
	if p.opt.Read == nil || p.reading {
		return nil
	}
	p.reading = true
	return p.readCmd()
}

func (p ImpactPane) tick() tea.Cmd {
	return tea.Tick(p.opt.Tick, func(time.Time) tea.Msg { return paneTickMsg{} })
}

// selectedEdge is the selected edge, when one is and it has a site.
func (p ImpactPane) selectedEdge() (arch.Edge, bool) {
	if p.res == nil || p.edge == "" {
		return arch.Edge{}, false
	}
	for _, e := range p.res.Edges {
		if edgeKey(e) == p.edge {
			return e, len(e.Sites) > 0
		}
	}
	return arch.Edge{}, false
}

// paneFrame is where the pane's parts go: the canvas's lines in dw
// columns and dh rows, and with an edge selected its code at column ex in
// ew columns and eh rows below its header.
type paneFrame struct {
	dw, dh     int
	ex, ew, eh int
	edge       bool
}

func (p ImpactPane) frame() paneFrame {
	h := max(p.height-4, 1)
	f := paneFrame{dw: p.width, dh: h}
	if _, ok := p.selectedEdge(); !ok {
		return f
	}
	f.edge = true
	if p.width >= paneBeside {
		f.dw = p.width * 55 / 100
		f.ex, f.ew, f.eh = f.dw+1, p.width-f.dw-1, h-1
		return f
	}
	// The canvas keeps six rows, short of the code's rule, header and a
	// row when the pane is small.
	f.dh = clamp(max(h*45/100, 6), 1, max(h-3, 1))
	f.ew, f.eh = p.width, max(h-f.dh-2, 1)
	return f
}

// body is the canvas's lines, two columns fewer for the scrollbar when
// they overflow h rows.
func (p ImpactPane) body() (d *drawer, h int, barred bool) {
	f := p.frame()
	h = f.dh
	build := func(w int) *drawer {
		d := newDrawer(w, "")
		d.light, d.cur = p.light, p.cur
		if p.res == nil {
			d.line(" " + dim.Render("reading the change's shape…"))
			return d
		}
		o := impactOpts{limit: findingsFull, sel: p.sel, edge: p.edge, cache: p.cache}
		if p.all {
			o.limit = -1
		}
		d.impact = impactLines(d, p.res, o)
		return d
	}
	d = build(f.dw - 2)
	if (scrollbar{total: len(d.lines), h: h}).shown() && f.dw >= barMinWidth {
		return d, h, true
	}
	return build(f.dw), h, false
}

func (p ImpactPane) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
		p.scroll(0)
	case tea.BackgroundColorMsg:
		p.light = !msg.IsDark()
	case paneTickMsg:
		cmd := p.read()
		return p, tea.Batch(cmd, p.tick())
	case paneReadMsg:
		p.reading = false
		if msg.diff != nil {
			p.diff, p.diffRead = *msg.diff, true
		}
		if p.res != msg.res {
			first := p.res == nil
			p.res = msg.res
			p.scroll(0)
			if d, _, _ := p.body(); first && d.impact != nil && len(d.stops) > 0 {
				// As the deck's tab on focus: the cursor on the first
				// finding selects its box.
				p.move(d, 0)
			}
		}
		cmd := p.readEdge()
		return p, cmd
	case edgeViewMsg:
		if p.ecode.keep(msg.key, msg.v) {
			cmd := p.readEdge()
			return p, cmd
		}
	case openedMsg:
		if msg.err != nil {
			p.status = fmt.Sprintf("could not open %s: %v", msg.what, msg.err)
		} else {
			p.status = "opened " + msg.what
		}
	case tea.KeyPressMsg:
		p.status = ""
		cmd := p.key(msg)
		read := p.readEdge()
		return p, tea.Batch(cmd, read)
	case tea.MouseWheelMsg:
		delta := 0
		switch msg.Mouse().Button {
		case tea.MouseWheelDown:
			delta = 3
		case tea.MouseWheelUp:
			delta = -3
		}
		// The wheel scrolls what is under it: the edge's code, or the
		// canvas; never a selection.
		if delta != 0 && p.overCode(msg.Mouse()) {
			p.scrollCode(delta)
		} else if delta != 0 {
			p.scroll(delta)
		}
	case tea.MouseClickMsg:
		p.status = ""
		cmd := p.click(msg.Mouse())
		read := p.readEdge()
		return p, tea.Batch(cmd, read)
	}
	return p, nil
}

// readEdge reads the selected edge's shown site, unless it was read.
func (p *ImpactPane) readEdge() tea.Cmd {
	e, ok := p.selectedEdge()
	if !ok {
		return nil
	}
	if !p.diffRead && p.opt.Diff != nil {
		return nil // the next read brings the diff, and reads it
	}
	rd := edgeReaders{patch: p.opt.Patch, fileAt: p.opt.FileAt}
	return p.ecode.read(rd, p.opt.Thread, p.diff, p.res, e.Sites[clamp(p.esite, 0, len(e.Sites)-1)])
}

// move puts the cursor on stop at and selects what it selects.
func (p *ImpactPane) move(d *drawer, at int) {
	was := p.edge
	p.cur = at
	p.sel = impactSel(d.impact, d.stops, p.cur, p.sel)
	p.edge = impactEdgeAt(d.impact, d.stops, p.cur)
	if p.edge != was {
		p.esite = 0
	}
	p.show()
}

func (p *ImpactPane) key(msg tea.KeyPressMsg) tea.Cmd {
	d, h, _ := p.body()
	e, onEdge := p.selectedEdge()
	boxSel := d.impact != nil && d.impact.sel != ""
	switch s := msg.String(); {
	case s == "q", s == "ctrl+c", s == "esc" && !onEdge:
		return tea.Quit
	case s == "esc":
		// From the edge back to its box.
		if b := d.impact.boxStop(p.sel); b >= 0 {
			p.move(d, b)
		}
	case s == "j", s == "down", s == "k", s == "up", s == "h", s == "left", s == "l", s == "right":
		dir := map[string]string{"j": "down", "down": "down", "k": "up", "up": "up",
			"h": "left", "left": "left", "l": "right", "right": "right"}[s]
		if d.impact == nil || len(d.stops) == 0 {
			switch dir {
			case "down":
				p.scroll(1)
			case "up":
				p.scroll(-1)
			}
			return nil
		}
		p.move(d, impactStep(d.impact, d.stops, p.cur, dir))
	case boxSel && (s == "n" || s == "]" && onEdge):
		p.move(d, edgeCycle(d.stops, p.cur, 1))
	case boxSel && (s == "N" || s == "[" && onEdge):
		p.move(d, edgeCycle(d.stops, p.cur, -1))
	case onEdge && (s == "tab" || s == "."):
		p.esite = (p.esite + 1) % len(e.Sites)
	case onEdge && (s == "shift+tab" || s == ","):
		p.esite = (p.esite - 1 + len(e.Sites)) % len(e.Sites)
	case s == "enter":
		if d.impact != nil && p.cur < len(d.stops) {
			return p.act(d, d.stops[p.cur].act)
		}
	case s == "pgdown", s == "ctrl+d", s == " ":
		p.scroll(max(h-2, 1))
	case s == "pgup", s == "ctrl+u":
		p.scroll(-max(h-2, 1))
	case s == "home", s == "g":
		p.off = 0
	case s == "end", s == "G":
		p.scroll(1 << 30)
	case s == "r":
		return p.read()
	}
	return nil
}

// edgeCycle is the stop of the next (dir 1) or previous edge row after
// cur, wrapping; from a stop that is not an edge row, the first or last.
func edgeCycle(stops []stop, cur, dir int) int {
	var edges []int
	at := -1
	for i, s := range stops {
		if s.act.kind == actEdge {
			if i == cur {
				at = len(edges)
			}
			edges = append(edges, i)
		}
	}
	switch {
	case len(edges) == 0:
		return cur
	case at < 0 && dir > 0:
		return edges[0]
	case at < 0:
		return edges[len(edges)-1]
	}
	return edges[(at+dir+len(edges))%len(edges)]
}

// scroll moves the view by delta lines, within the content.
func (p *ImpactPane) scroll(delta int) {
	d, h, _ := p.body()
	p.off = clamp(p.off+delta, 0, max(len(d.lines)-h, 0))
}

// show scrolls the cursor's stop into view: a box whole when it fits.
func (p *ImpactPane) show() {
	d, h, _ := p.body()
	if p.cur >= len(d.stops) {
		return
	}
	top := d.stops[p.cur].line
	bottom := top
	if d.impact != nil {
		if t, b, ok := impactSpan(d.impact, d.stops[p.cur].act); ok {
			top, bottom = t, b
		}
	}
	if bottom >= p.off+h {
		p.off = min(bottom-h+1, top)
	}
	if top < p.off {
		p.off = top
	}
	p.scroll(0)
}

// act is ↵ on a stop: a finding, an import or a touchpoint opens its file
// in the diff tool, a box its changed files, the ⋯ line shows every
// finding or fewer.
func (p *ImpactPane) act(d *drawer, a action) tea.Cmd {
	lay := d.impact
	var files []string
	switch a.kind {
	case actFindMore:
		p.all = !p.all
		d, _, _ := p.body()
		at := max(d.impact.findings-1, 0)
		for i, s := range d.stops {
			if s.act.kind == actFindMore {
				at = i
			}
		}
		p.move(d, at)
		return nil
	case actFinding:
		if f := lay.res.Findings[a.n]; f.Site.File != "" {
			files = []string{f.Site.File}
		}
	case actBox:
		pk, _ := lay.res.Package(lay.boxes[a.n].path)
		files = pk.Changed
	case actSite:
		files = []string{lay.sites[a.n].file}
	case actEdge:
		if e, ok := p.selectedEdge(); ok {
			files = []string{e.Sites[clamp(p.esite, 0, len(e.Sites)-1)].File}
		}
	}
	if len(files) == 0 {
		p.status = "no file to open"
		return nil
	}
	open := p.opt.OpenDiff
	if open == nil {
		p.status = "opening diffs is off"
		return nil
	}
	path, base := p.opt.Thread.Worktree, lay.res.MergeBase
	what := strings.Join(files, ", ")
	if len(files) > 2 {
		what = fmt.Sprintf("%d files", len(files))
	}
	return func() tea.Msg { return openedMsg{what: what + " in the diff tool", err: open(path, base, files)} }
}

// click selects the box, finding or line under the pointer, or acts on
// it when it is selected already; a detail row acts at once.
func (p *ImpactPane) click(mouse tea.Mouse) tea.Cmd {
	if mouse.Button != tea.MouseLeft {
		return nil
	}
	d, h, _ := p.body()
	i := p.off + mouse.Y - 2
	if mouse.Y < 2 || mouse.Y >= 2+h || i >= len(d.lines) || mouse.X >= p.frame().dw {
		return nil
	}
	at, first, line := impactClickStop(d, i, mouse.X)
	if line != nil {
		if p.edge == edgeKey(*line) {
			return p.act(d, action{kind: actEdge})
		}
		// The edge with its source box; failing its row, the box.
		p.sel = line.From
		d, _, _ = p.body()
		s := d.impact.edgeStop(d.stops, edgeKey(*line))
		if s < 0 {
			s = d.impact.boxStop(line.From)
		}
		if s >= 0 {
			p.move(d, s)
		}
		return nil
	}
	if at < 0 {
		return nil
	}
	if first && p.cur != at {
		p.move(d, at)
		return nil
	}
	p.move(d, at)
	return p.act(d, d.stops[at].act)
}

func (p ImpactPane) View() tea.View {
	v := tea.NewView(p.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "herdr-deck · Impact · " + p.opt.Thread.ID
	return v
}

func (p ImpactPane) render() string {
	w := p.width
	rule := dim.Render(strings.Repeat("─", w))
	t := p.opt.Thread
	title := " " + bold.Render("Impact") + dim.Render(" · ") + t.ID
	if t.Title != "" {
		title += " " + t.Title
	}
	right := ""
	if p.res != nil && p.res.Note == "" {
		right = dim.Render(fmt.Sprintf("%s · %.7s → %.7s ", p.res.Name, p.res.MergeBase, p.res.Head))
	}
	lines := []string{spread(title, right, w), rule}
	f := p.frame()
	d, h, barred := p.body()
	off := clamp(p.off, 0, max(len(d.lines)-h, 0))
	// The canvas's lines are padded to their width, which leaves the
	// scrollbar its two columns when they overflow.
	lw := f.dw
	if barred {
		lw -= 2
	}
	body := make([]string, h)
	for i := range body {
		if j := off + i; j < len(d.lines) {
			body[i] = fit(d.lines[j].text, lw)
		} else {
			body[i] = strings.Repeat(" ", lw)
		}
	}
	if barred {
		body = withBar(body, scrollbar{total: len(d.lines), h: h, off: off}, f.dw, false, p.light)
	}
	if e, ok := p.selectedEdge(); ok && f.edge {
		// The rows draw as the deck's diff preview draws them.
		r := Model{light: p.light, split: p.opt.Split, width: f.ew}
		site := clamp(p.esite, 0, len(e.Sites)-1)
		key := edgeSiteKey(t, p.res, e.Sites[site])
		v, read := p.ecode.views[key]
		code := r.edgeLines(e, v, read, site, f.ew, f.eh, p.escroll.at(key))
		if f.ex > 0 {
			// Beside the canvas, past a divider.
			for i := range body {
				right := ""
				if i < len(code) {
					right = code[i]
				}
				body[i] += dim.Render("│") + right
			}
		} else {
			body = append(body, rule)
			body = append(body, code...)
		}
	}
	lines = append(lines, body...)
	foot := p.status
	if foot == "" {
		foot = dim.Render("j k h l move  n N edges  ↵ diff tool  esc back  r read again  q close")
		if w < wideMin {
			foot = dim.Render("hjkl move  n edge  ↵ diff  q close")
		}
	}
	for len(lines) < p.height-2 {
		lines = append(lines, "")
	}
	lines = append(lines[:max(p.height-2, 2)], rule, fit(" "+foot, w))
	return strings.Join(lines, "\n")
}

// impactClickStop is the stop a click on drawer line i, column x, lands
// on: a box on the canvas, a finding, or a detail row (or the ⋯ line);
// first says the stop selects on a first click and acts on a second one
// (a box, a finding). -1 is none. On a selection's line, or a cell next
// to one, it is the line's edge instead (line), whatever is under it.
func impactClickStop(d *drawer, i, x int) (at int, first bool, line *arch.Edge) {
	lay := d.impact
	if lay == nil || i < 0 || i >= len(d.lines) {
		return -1, false, nil
	}
	if i >= lay.top && i < lay.top+lay.c.g.h && x >= lay.left && x < lay.left+lay.c.g.w {
		if e := lay.c.lineAt(x-lay.left, i-lay.top); e >= 0 && lay.sel != "" {
			return -1, false, &lay.c.routed[e]
		}
		if b := lay.c.pkgAt(x-lay.left, i-lay.top); b != nil {
			return lay.boxStop(b.path), true, nil
		}
		return -1, false, nil
	}
	for _, z := range d.lines[i].zones {
		if x < z.x0 || x >= z.x1 {
			continue
		}
		for k, s := range d.stops {
			if s.act == z.act {
				return k, z.act.kind == actFinding, nil
			}
		}
	}
	return -1, false, nil
}

// overCode says the pointer is over the selected edge's code.
func (p ImpactPane) overCode(mouse tea.Mouse) bool {
	f := p.frame()
	if !f.edge {
		return false
	}
	if f.ex > 0 {
		return mouse.X >= f.ex && mouse.Y >= 2 && mouse.Y < 2+f.dh
	}
	return mouse.Y > 2+f.dh && mouse.Y < 2+f.dh+1+f.eh+1
}

// scrollCode scrolls the selected edge's code by delta rows.
func (p *ImpactPane) scrollCode(delta int) {
	e, ok := p.selectedEdge()
	if !ok {
		return
	}
	key := edgeSiteKey(p.opt.Thread, p.res, e.Sites[clamp(p.esite, 0, len(e.Sites)-1)])
	if v, read := p.ecode.views[key]; read {
		f := p.frame()
		r := Model{light: p.light, split: p.opt.Split, width: f.ew}
		p.escroll = r.scrolled(p.escroll, key, v, f.eh, delta)
	}
}

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
// arrows or hjkl from box to box, a click selects and a second one opens.
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
	// OpenDiff opens the diff tool for the worktree at path against base,
	// limited to files. Tests replace it so no diff tool is ever run.
	OpenDiff func(path, base string, files []string) error
}

// ImpactPane is the Bubble Tea model of the Impact view's own pane.
type ImpactPane struct {
	opt           ImpactOptions
	res           *arch.Result
	reading       bool
	width, height int
	light         bool
	off, cur      int
	sel           string
	all           bool
	status        string
	cache         *canvasCache
}

type paneReadMsg struct{ res *arch.Result }
type paneTickMsg struct{}

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
	if read := p.opt.Read; read != nil {
		t := p.opt.Thread
		cmds = append(cmds, func() tea.Msg { return paneReadMsg{read(context.Background(), t)} })
	}
	return tea.Batch(cmds...)
}

func (p *ImpactPane) read() tea.Cmd {
	if p.opt.Read == nil || p.reading {
		return nil
	}
	p.reading = true
	read, t := p.opt.Read, p.opt.Thread
	return func() tea.Msg { return paneReadMsg{read(context.Background(), t)} }
}

func (p ImpactPane) tick() tea.Cmd {
	return tea.Tick(p.opt.Tick, func(time.Time) tea.Msg { return paneTickMsg{} })
}

// body is the pane's content: h lines under the header, w columns wide,
// two fewer for the scrollbar when it overflows.
func (p ImpactPane) body() (d *drawer, h int, barred bool) {
	h = max(p.height-4, 1)
	build := func(w int) *drawer {
		d := newDrawer(w, "")
		d.light, d.cur = p.light, p.cur
		if p.res == nil {
			d.line(" " + dim.Render("reading the change's shape…"))
			return d
		}
		o := impactOpts{limit: findingsFull, sel: p.sel, cache: p.cache}
		if p.all {
			o.limit = -1
		}
		d.impact = impactLines(d, p.res, o)
		return d
	}
	d = build(p.width - 2)
	if (scrollbar{total: len(d.lines), h: h}).shown() && p.width >= barMinWidth {
		return d, h, true
	}
	return build(p.width), h, false
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
		if p.res != msg.res {
			p.res = msg.res
			p.scroll(0)
		}
	case openedMsg:
		if msg.err != nil {
			p.status = fmt.Sprintf("could not open %s: %v", msg.what, msg.err)
		} else {
			p.status = "opened " + msg.what
		}
	case tea.KeyPressMsg:
		p.status = ""
		return p.key(msg)
	case tea.MouseWheelMsg:
		switch msg.Mouse().Button {
		case tea.MouseWheelDown:
			p.scroll(3)
		case tea.MouseWheelUp:
			p.scroll(-3)
		}
	case tea.MouseClickMsg:
		p.status = ""
		cmd := p.click(msg.Mouse())
		return p, cmd
	}
	return p, nil
}

func (p ImpactPane) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	d, h, _ := p.body()
	switch s := msg.String(); s {
	case "q", "esc", "ctrl+c":
		return p, tea.Quit
	case "j", "down", "k", "up", "h", "left", "l", "right":
		dir := map[string]string{"j": "down", "down": "down", "k": "up", "up": "up",
			"h": "left", "left": "left", "l": "right", "right": "right"}[s]
		if d.impact == nil || len(d.stops) == 0 {
			switch dir {
			case "down":
				p.scroll(1)
			case "up":
				p.scroll(-1)
			}
			return p, nil
		}
		p.cur = impactStep(d.impact, d.stops, p.cur, dir)
		p.sel = impactSel(d.impact, d.stops, p.cur, p.sel)
		p.show()
	case "enter":
		if d.impact != nil && p.cur < len(d.stops) {
			cmd := p.act(d, d.stops[p.cur].act)
			return p, cmd
		}
	case "pgdown", "ctrl+d", " ":
		p.scroll(max(h-2, 1))
	case "pgup", "ctrl+u":
		p.scroll(-max(h-2, 1))
	case "home", "g":
		p.off = 0
	case "end", "G":
		p.scroll(1 << 30)
	case "r":
		cmd := p.read()
		return p, cmd
	}
	return p, nil
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
	if a := d.stops[p.cur].act; a.kind == actBox && d.impact != nil {
		b := d.impact.boxes[a.n]
		bottom = top + b.h - 1
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
		for i, s := range d.stops {
			if s.act.kind == actFindMore {
				p.cur = i
			}
		}
		p.show()
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

// click selects the box or finding under the pointer, or acts on it when
// it is selected already; a detail row acts at once.
func (p *ImpactPane) click(mouse tea.Mouse) tea.Cmd {
	if mouse.Button != tea.MouseLeft {
		return nil
	}
	d, h, _ := p.body()
	i := p.off + mouse.Y - 2
	if mouse.Y < 2 || mouse.Y >= 2+h || i >= len(d.lines) {
		return nil
	}
	at, first := impactClickStop(d, i, mouse.X)
	if at < 0 {
		return nil
	}
	if first && p.cur != at {
		p.cur = at
		p.sel = impactSel(d.impact, d.stops, p.cur, p.sel)
		return nil
	}
	p.cur = at
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
	d, h, barred := p.body()
	off := clamp(p.off, 0, max(len(d.lines)-h, 0))
	body := make([]string, h)
	for i := range body {
		if j := off + i; j < len(d.lines) {
			body[i] = d.lines[j].text
		}
	}
	if barred {
		body = withBar(body, scrollbar{total: len(d.lines), h: h, off: off}, w, false, p.light)
	}
	lines = append(lines, body...)
	foot := p.status
	if foot == "" {
		foot = dim.Render("j k h l move  ↵ diff tool  pgdn scroll  r read again  q close")
		if w < wideMin {
			foot = dim.Render("hjkl move  ↵ diff  q close")
		}
	}
	lines = append(lines, rule, fit(" "+foot, w))
	return strings.Join(lines, "\n")
}

// impactClickStop is the stop a click on drawer line i, column x, lands
// on: a box on the canvas, a finding, or a detail row (or the ⋯ line);
// first says the stop selects on a first click and acts on a second one
// (a box, a finding). -1 is none.
func impactClickStop(d *drawer, i, x int) (at int, first bool) {
	lay := d.impact
	if lay == nil || i < 0 || i >= len(d.lines) {
		return -1, false
	}
	if i >= lay.top && i < lay.top+lay.c.g.h && x >= lay.left && x < lay.left+lay.c.g.w {
		if b := lay.c.pkgAt(x-lay.left, i-lay.top); b != nil {
			return lay.boxStop(b.path), true
		}
		return -1, false
	}
	for _, z := range d.lines[i].zones {
		if x < z.x0 || x >= z.x1 {
			continue
		}
		for k, s := range d.stops {
			if s.act == z.act {
				return k, z.act.kind == actFinding
			}
		}
	}
	return -1, false
}

// Package ui renders a deck.Snapshot as a Bubble Tea program: a compact list
// of the project's work on top and a drawer about the selected row below
// (docs/design/d-list-detail.md). It holds no data code: snapshots come from
// internal/source through Options.Load.
package ui

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// DefaultTick is how often the deck reloads when no file change says to.
const DefaultTick = 5 * time.Second

// Options wires the model to the outside world. Every field may be nil.
type Options struct {
	// Load reads a fresh snapshot. It runs off the UI goroutine, on start,
	// on every Tick and on RefreshMsg. Nil shows the initial snapshot only.
	Load func(context.Context) deck.Snapshot
	// Tick is the reload interval; zero means DefaultTick.
	Tick time.Duration
	// OpenURL opens a link; OpenEditor opens a worktree folder. Tests
	// replace both so nothing is ever opened.
	OpenURL    func(url string) error
	OpenEditor func(path string) error
	// FigmaDesktop opens Figma links in the desktop app (their figma://
	// rewrite) wherever a key or click opens a link, not only after d.
	FigmaDesktop bool
	// FocusPane focuses a herdr pane by id. Tests replace it so no real
	// pane is ever focused.
	FocusPane func(paneID string) error
	// StartDev runs the dev manifest's `up` command for a thread's
	// worktree and returns the line the footer shows. Tests replace it so
	// no dev server is ever started.
	StartDev func(t deck.Thread) (string, error)
	// Now is the clock for ages; Location the zone clock times show in.
	Now      func() time.Time
	Location *time.Location
	// Version is shown dim at the right of the footer when the key help
	// leaves room for it; empty shows nothing.
	Version string
}

// RefreshMsg asks the model to reload its snapshot, e.g. after the project
// folder changed. Reloads are coalesced: one runs at a time.
type RefreshMsg struct{}

type (
	tickMsg     struct{}
	snapshotMsg deck.Snapshot
	openedMsg   struct {
		what string
		err  error
		verb string // "open" when empty
	}
	devUpMsg struct {
		status string
		err    error
	}
)

// drawerSize cycles with z.
type drawerSize int

const (
	sizeNormal drawerSize = iota // about 40 % of the pane
	sizeFull
	sizeHidden
)

const (
	defaultWidth  = 80
	defaultHeight = 28
	// wideMin is the narrowest pane that gets the THREAD and PR columns.
	wideMin = 72
	// noKind marks the chooser as closed.
	noKind deck.LinkKind = -1
)

// Model is the deck's Bubble Tea model.
type Model struct {
	opt   Options
	keys  keyMap
	snap  deck.Snapshot
	rows  []row
	folds map[string]bool // the user's fold toggles, by list name

	cursor    int
	moved     bool // the user moved the cursor, so needs-you no longer pulls it
	listOff   int
	drawerOff int
	mode      drawerMode
	size      drawerSize
	choosing  deck.LinkKind // the link chooser's kind, or noKind
	desktop   bool          // the Figma chooser opens the desktop app
	status    string        // a one-off message in the footer

	width, height int
	light         bool // the terminal's background is light

	loaded      bool // a snapshot came from Load (or there is no Load)
	loading     bool
	pending     bool // a reload was asked for while one ran
	sourcesSeen bool // the Sources view came up by itself once
}

// New returns a model showing snap until Options.Load delivers a fresh one.
func New(snap deck.Snapshot, opt Options) Model {
	if opt.Now == nil {
		opt.Now = time.Now
	}
	if opt.Tick == 0 {
		opt.Tick = DefaultTick
	}
	m := Model{
		opt:      opt,
		keys:     defaultKeys(),
		folds:    map[string]bool{},
		choosing: noKind,
		width:    defaultWidth,
		height:   defaultHeight,
		loaded:   opt.Load == nil,
		loading:  opt.Load != nil, // Init starts the first load
	}
	m.SetSnapshot(snap)
	return m
}

// SetSnapshot replaces the data. The selection stays on the same row when it
// still exists; until the user moves the cursor it follows what needs them.
func (m *Model) SetSnapshot(snap deck.Snapshot) {
	var selKey string
	if r, ok := m.selected(); ok {
		selKey = r.key
	}
	m.snap = snap
	m.rows = buildRows(snap, m.folds)

	switch {
	case !m.moved:
		m.cursor = m.homeRow()
	default:
		found := false
		for i, r := range m.rows {
			if r.key == selKey {
				m.cursor, found = i, true
				break
			}
		}
		if !found {
			m.cursor = m.offGap(clamp(m.cursor, 0, len(m.rows)-1), 1)
		}
	}
	// On another row, the drawer starts over: a report or a scroll
	// position belongs to the row it was opened on.
	if r, _ := m.selected(); r.key != selKey {
		m.drawerOff = 0
		if m.mode == modeReport {
			m.mode = modeRow
		}
	}
	// The Sources view comes up by itself once, when something is missing.
	if m.loaded && !m.sourcesSeen && len(snap.Missing) > 0 && m.mode == modeRow {
		m.mode = modeSources
		m.sourcesSeen = true
	}
	m.ensureVisible()
}

// homeRow is where the cursor rests on its own: the first row that needs the
// user, else the first row of work, else the top.
func (m Model) homeRow() int {
	for i, r := range m.rows {
		if r.kind != rowHeading && r.needsYou() {
			return i
		}
	}
	for i, r := range m.rows {
		if r.kind == rowWork || r.kind == rowInbox {
			return i
		}
	}
	return 0
}

// offGap moves i off a gap in direction dir (1 or -1). A gap always sits
// between two groups, so the next row that way is a heading or a row.
func (m Model) offGap(i, dir int) int {
	for i >= 0 && i < len(m.rows) && m.rows[i].kind == rowGap {
		i += dir
	}
	return i
}

// Snapshot returns the data the model shows.
func (m Model) Snapshot() deck.Snapshot { return m.snap }

// Status returns the one-off footer message, if any.
func (m Model) Status() string { return m.status }

func (m Model) selected() (row, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return row{}, false
	}
	return m.rows[m.cursor], true
}

func (m Model) Init() tea.Cmd {
	if m.opt.Load == nil {
		return tea.RequestBackgroundColor
	}
	return tea.Batch(m.load(), m.tick(), tea.RequestBackgroundColor)
}

func (m Model) load() tea.Cmd {
	load := m.opt.Load
	return func() tea.Msg { return snapshotMsg(load(context.Background())) }
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(m.opt.Tick, func(time.Time) tea.Msg { return tickMsg{} })
}

// refresh starts a reload, or notes that another is wanted after the
// running one.
func (m *Model) refresh() tea.Cmd {
	if m.opt.Load == nil {
		return nil
	}
	if m.loading {
		m.pending = true
		return nil
	}
	m.loading = true
	return m.load()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.scrollDrawer(0)
		m.ensureVisible()
	case tea.BackgroundColorMsg:
		m.light = !msg.IsDark()
	case tickMsg:
		return m, tea.Batch(m.refresh(), m.tick())
	case RefreshMsg:
		return m, m.refresh()
	case snapshotMsg:
		m.loading = false
		m.loaded = true
		m.SetSnapshot(deck.Snapshot(msg))
		if m.pending {
			m.pending = false
			return m, m.refresh()
		}
	case openedMsg:
		verb, past := "open", "opened"
		if msg.verb != "" {
			verb, past = msg.verb, msg.verb+"ed"
		}
		if msg.err != nil {
			m.status = fmt.Sprintf("could not %s %s: %v", verb, msg.what, msg.err)
		} else {
			m.status = past + " " + msg.what
		}
	case devUpMsg:
		if msg.err != nil {
			m.status = "could not start dev servers: " + msg.err.Error()
		} else {
			m.status = msg.status
		}
		// The drawer shows the log and that the servers are starting.
		return m, m.refresh()
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.MouseClickMsg:
		return m.handleClick(msg.Mouse())
	case tea.MouseWheelMsg:
		m.handleWheel(msg.Mouse())
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	if m.choosing != noKind {
		if cmd, done := m.chooserKey(msg); done {
			return m, cmd
		}
	}
	if s := msg.String(); len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		return m, m.openNumbered(int(s[0] - '1'))
	}
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Down):
		m.move(1)
	case key.Matches(msg, m.keys.Up):
		m.move(-1)
	case key.Matches(msg, m.keys.Fold):
		m.toggleFold()
	case key.Matches(msg, m.keys.Linear):
		return m, m.linkKey(deck.LinkLinear)
	case key.Matches(msg, m.keys.Figma):
		return m, m.linkKey(deck.LinkFigma)
	case key.Matches(msg, m.keys.Notion):
		return m, m.linkKey(deck.LinkNotion)
	case key.Matches(msg, m.keys.PR):
		return m, m.linkKey(deck.LinkGitHub)
	case key.Matches(msg, m.keys.Localhost):
		return m, m.linkKey(deck.LinkLocalhost)
	case key.Matches(msg, m.keys.Pane):
		return m, m.focusPane()
	case key.Matches(msg, m.keys.Editor):
		return m, m.openEditor()
	case key.Matches(msg, m.keys.DevUp):
		return m, m.startDev()
	case key.Matches(msg, m.keys.Report):
		m.toggleReport()
	case key.Matches(msg, m.keys.Drawer):
		m.size = (m.size + 1) % 3
		m.drawerOff = 0
		m.ensureVisible()
	case key.Matches(msg, m.keys.Sources):
		m.toggleMode(modeSources)
	case key.Matches(msg, m.keys.Help):
		m.toggleMode(modeHelp)
	case key.Matches(msg, m.keys.PageDown):
		m.scrollDrawer(max(m.layout().drawerH/2, 1))
	case key.Matches(msg, m.keys.PageUp):
		m.scrollDrawer(-max(m.layout().drawerH/2, 1))
	case key.Matches(msg, m.keys.Back):
		m.setMode(modeRow)
	}
	return m, nil
}

func (m *Model) move(delta int) {
	if len(m.rows) == 0 {
		return
	}
	dir := 1
	if delta < 0 {
		dir = -1
	}
	m.cursor = m.offGap(clamp(m.cursor+delta, 0, len(m.rows)-1), dir)
	m.moved = true
	m.setMode(modeRow)
	m.ensureVisible()
}

func (m *Model) setMode(mode drawerMode) {
	m.mode = mode
	m.drawerOff = 0
	m.ensureVisible()
}

func (m *Model) toggleMode(mode drawerMode) {
	if m.mode == mode {
		m.setMode(modeRow)
	} else {
		m.setMode(mode)
	}
}

func (m *Model) toggleFold() {
	r, ok := m.selected()
	if !ok || r.kind != rowHeading || !r.foldable {
		return
	}
	m.folds[r.list] = !r.folded
	m.rows = buildRows(m.snap, m.folds)
	m.ensureVisible()
}

func (m *Model) toggleReport() {
	if m.mode == modeReport {
		m.setMode(modeRow)
		return
	}
	r, _ := m.selected()
	t, ok := r.thread()
	switch {
	case r.kind != rowWork || !ok:
		m.status = "no thread on this row"
	case t.Report == "":
		m.status = t.ID + " has no report yet"
	default:
		m.setMode(modeReport)
	}
}

// links returns the selected row's links in drawer order.
func (m Model) links() []deck.Link {
	r, ok := m.selected()
	if !ok {
		return nil
	}
	return rowLinks(r, m.snap)
}

// linkKey opens the row's only link of kind, or opens the chooser when there
// are several. Pressed again in the chooser, it opens the first.
func (m *Model) linkKey(kind deck.LinkKind) tea.Cmd {
	if kind == deck.LinkLocalhost {
		return m.openLocalhost()
	}
	var of []int
	for i, l := range m.links() {
		if l.Kind == kind {
			of = append(of, i)
		}
	}
	switch len(of) {
	case 0:
		m.status = "no " + kind.String() + " link on this row"
		return nil
	case 1:
		return m.openNumbered(of[0])
	}
	m.setMode(modeRow)
	m.choosing = kind
	m.desktop = false
	return nil
}

// openLocalhost opens the row's first localhost link whose dev server is
// running. Links to servers that are down open only by their digit.
func (m *Model) openLocalhost() tea.Cmd {
	some := false
	for i, l := range m.links() {
		if l.Kind != deck.LinkLocalhost {
			continue
		}
		if !l.Down {
			return m.openNumbered(i)
		}
		some = true
	}
	if some {
		m.status = "no dev server on this row is running; a digit opens a link anyway"
	} else {
		m.status = "no localhost link on this row"
	}
	return nil
}

// chooserKey handles a key while the chooser waits. done is false when the
// key closes the chooser and should then be handled as usual.
func (m *Model) chooserKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	kind := m.choosing
	desktop := m.desktop
	m.choosing, m.desktop = noKind, false
	links := m.links()
	s := msg.String()
	switch {
	case len(s) == 1 && s[0] >= '1' && s[0] <= '9':
		i := int(s[0] - '1')
		if desktop {
			return m.openDesktop(links, i), true
		}
		return m.openNumbered(i), true
	case s == "a":
		var cmds []tea.Cmd
		for _, l := range links {
			if l.Kind == kind {
				cmds = append(cmds, m.openLink(l))
			}
		}
		return tea.Batch(cmds...), true
	case s == "d" && kind == deck.LinkFigma:
		m.choosing, m.desktop = kind, true
		return nil, true
	case key.Matches(msg, m.keys.Back):
		return nil, true
	case s == kindKey(kind):
		for i, l := range links {
			if l.Kind == kind {
				if desktop {
					return m.openDesktop(links, i), true
				}
				return m.openNumbered(i), true
			}
		}
		return nil, true
	}
	return nil, false
}

func kindKey(k deck.LinkKind) string {
	switch k {
	case deck.LinkLinear:
		return "l"
	case deck.LinkFigma:
		return "f"
	case deck.LinkNotion:
		return "n"
	case deck.LinkGitHub:
		return "g"
	case deck.LinkLocalhost:
		return "o"
	}
	return ""
}

func (m *Model) openNumbered(i int) tea.Cmd {
	links := m.links()
	if i < 0 || i >= len(links) {
		m.status = fmt.Sprintf("no link %d on this row", i+1)
		return nil
	}
	return m.openLink(links[i])
}

// openLink opens l, a Figma link in the desktop app when FigmaDesktop is set.
func (m *Model) openLink(l deck.Link) tea.Cmd {
	if d := l.DesktopURL(); d != "" && m.opt.FigmaDesktop {
		return m.openURL(d, l.Label+" in Figma")
	}
	return m.openURL(l.URL, l.Label)
}

func (m *Model) openDesktop(links []deck.Link, i int) tea.Cmd {
	if i < 0 || i >= len(links) || links[i].DesktopURL() == "" {
		m.status = fmt.Sprintf("link %d is not a Figma link", i+1)
		return nil
	}
	return m.openURL(links[i].DesktopURL(), links[i].Label+" in Figma")
}

func (m *Model) openURL(url, what string) tea.Cmd {
	if url == "" {
		m.status = what + ": " + noWorkspace
		return nil
	}
	open := m.opt.OpenURL
	if open == nil {
		m.status = "opening links is off: " + url
		return nil
	}
	return func() tea.Msg { return openedMsg{what: what, err: open(url)} }
}

func (m *Model) openEditor() tea.Cmd {
	r, _ := m.selected()
	t, ok := r.thread()
	if !ok || t.Worktree == "" {
		m.status = "no worktree on this row"
		return nil
	}
	open := m.opt.OpenEditor
	if open == nil {
		m.status = "opening the editor is off: " + t.Worktree
		return nil
	}
	path := t.Worktree
	return func() tea.Msg { return openedMsg{what: tilde(path), err: open(path)} }
}

// startDev runs the dev manifest's `up` command for the selected thread's
// worktree.
func (m *Model) startDev() tea.Cmd {
	r, _ := m.selected()
	t, ok := r.thread()
	switch {
	case r.kind != rowWork || !ok:
		m.status = "no thread on this row"
		return nil
	case t.Worktree == "":
		m.status = "no worktree on this row"
		return nil
	case t.Status == deck.StatusDone:
		m.status = t.ID + " is done"
		return nil
	}
	start := m.opt.StartDev
	if start == nil {
		m.status = "starting dev servers is off"
		return nil
	}
	m.status = "starting " + t.ID + "'s dev servers…"
	return func() tea.Msg {
		status, err := start(t)
		return devUpMsg{status: status, err: err}
	}
}

// focusPane focuses the herdr pane of the selected row's thread; on an inbox
// row, the subject thread's pane, else the coordinator's.
func (m *Model) focusPane() tea.Cmd {
	r, _ := m.selected()
	var t deck.Thread
	var ok bool
	switch r.kind {
	case rowWork:
		t, ok = r.thread()
		if !ok {
			m.status = "no thread on this row"
			return nil
		}
	case rowInbox:
		t, ok = findThread(m.snap, r.inbox.Thread)
	default:
		m.status = "no pane on this row"
		return nil
	}
	id, what := "", ""
	switch {
	case ok && t.Pane != nil:
		id, what = t.Pane.ID, t.ID
	case ok && !m.snap.Herdr && t.PaneID != "":
		// herdr's live state is unknown: try the recorded pane.
		id, what = t.PaneID, t.ID
	case r.kind == rowInbox && m.snap.Project.PaneID != "":
		id, what = m.snap.Project.PaneID, "the coordinator"
	case ok:
		m.status = t.ID + " has no open pane"
		return nil
	default:
		m.status = "no pane for this item"
		return nil
	}
	focus := m.opt.FocusPane
	if focus == nil {
		m.status = "focusing panes is off: " + id
		return nil
	}
	label := what + "'s pane " + id
	if what == "the coordinator" {
		label = "the coordinator's pane " + id
	}
	return func() tea.Msg { return openedMsg{what: label, err: focus(id), verb: "focus"} }
}

func (m *Model) handleClick(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	if mouse.Button != tea.MouseLeft {
		return *m, nil
	}
	m.status = ""
	l := m.layout()
	switch {
	case mouse.Y == 0 && mouse.X >= l.bang.x0 && mouse.X < l.bang.x1:
		m.toggleMode(modeSources)
	case mouse.Y >= l.listTop && mouse.Y < l.listTop+l.listH:
		i := m.listOff + mouse.Y - l.listTop
		if i >= len(m.rows) || m.rows[i].kind == rowGap {
			break
		}
		m.cursor, m.moved = i, true
		m.choosing = noKind
		m.setMode(modeRow)
		if m.rows[i].kind == rowHeading {
			m.toggleFold()
		}
	case mouse.Y >= l.drawerTop && mouse.Y < l.drawerTop+l.drawerH:
		m.scrollDrawer(0) // match the offset the screen was drawn with
		dl := l.drawer.lines
		i := m.drawerOff + mouse.Y - l.drawerTop
		if i >= len(dl) {
			break
		}
		for _, z := range dl[i].zones {
			if mouse.X >= z.x0 && mouse.X < z.x1 {
				m.choosing = noKind
				return *m, m.openNumbered(z.link)
			}
		}
	}
	return *m, nil
}

func (m *Model) handleWheel(mouse tea.Mouse) {
	delta := 0
	switch mouse.Button {
	case tea.MouseWheelDown:
		delta = 1
	case tea.MouseWheelUp:
		delta = -1
	default:
		return
	}
	l := m.layout()
	switch {
	case mouse.Y >= l.listTop && mouse.Y < l.listTop+l.listH:
		m.move(delta)
	case mouse.Y >= l.drawerTop-1 && mouse.Y < l.drawerTop+l.drawerH:
		m.scrollDrawer(delta * 3)
	}
}

// scrollDrawer moves the drawer's view by delta lines, within its content.
func (m *Model) scrollDrawer(delta int) {
	l := m.layout()
	most := 0
	if l.drawer != nil {
		most = max(len(l.drawer.lines)-l.drawerH, 0)
	}
	m.drawerOff = clamp(m.drawerOff+delta, 0, most)
}

// ensureVisible scrolls the list so the cursor shows.
func (m *Model) ensureVisible() {
	h := m.layout().listH
	if h <= 0 {
		return
	}
	if m.cursor < m.listOff {
		m.listOff = m.cursor
	}
	if m.cursor >= m.listOff+h {
		m.listOff = m.cursor - h + 1
	}
	m.listOff = clamp(m.listOff, 0, max(len(m.rows)-h, 0))
}

func (m Model) loc() *time.Location {
	if m.opt.Location != nil {
		return m.opt.Location
	}
	return time.Local
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	return max(lo, min(v, hi))
}

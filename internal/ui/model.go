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

	"github.com/FredricW/herdr-deck/internal/changelog"
	"github.com/FredricW/herdr-deck/internal/config"
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
	// FocusPane focuses a herdr pane by id, switching herdr to its
	// workspace and tab. Tests replace it so no real pane is ever focused.
	FocusPane func(paneID string) error
	// OpenProject starts the coordinator of the project slug and focuses it
	// (`herdr-projects open`), for the project picker when none runs. Tests
	// replace it so no coordinator is ever started.
	OpenProject func(slug string) error
	// Diff reads what a thread's worktree changed against its base. It
	// runs off the UI goroutine when the selection moves to another
	// thread and on every reload. Nil shows no Files section.
	Diff func(context.Context, deck.Thread) deck.Diff
	// OpenDiff opens the diff tool for the worktree at path against base
	// (a commit), for one file (a rename's old and new path) or, with no
	// files, the whole diff. Tests replace it so no diff tool is ever run.
	OpenDiff func(path, base string, files []string) error
	// Patch reads one file's diff against the merge-base for the preview
	// (v on the Files tab). It runs off the UI goroutine when the preview
	// moves to another file and on every reload. Nil turns the preview off.
	Patch func(ctx context.Context, t deck.Thread, mergeBase string, f deck.DiffFile) deck.Patch
	// Commits reads the commits of a thread's branch since its base, for
	// the Commits tab. It runs off the UI goroutine when the selection
	// moves to another thread and on every reload. Nil turns the tab off.
	Commits func(context.Context, deck.Thread) deck.Commits
	// CommitPatch reads one commit for the preview (v or ↵ on the Commits
	// tab). It runs off the UI goroutine. Nil turns that preview off.
	CommitPatch func(ctx context.Context, t deck.Thread, sha string) deck.CommitPatch
	// CommitFiles reads the files a commit changed, for an expanded
	// commit on the Commits tab. It runs off the UI goroutine. Nil leaves
	// commits unexpandable.
	CommitFiles func(ctx context.Context, t deck.Thread, sha string) ([]deck.DiffFile, error)
	// CommitFilePatch reads one file's change in a commit for the preview.
	// It runs off the UI goroutine.
	CommitFilePatch func(ctx context.Context, t deck.Thread, sha string, f deck.DiffFile) deck.Patch
	// OpenCommit opens the diff tool for commit sha of the worktree at
	// path, for some files (a rename's old and new path) or, with none,
	// the whole commit. Tests replace it so no diff tool is ever run.
	OpenCommit func(path, sha string, files []string) error
	// FoldedLists are the headings of the lists that start folded, ignoring
	// case (ui.folded_lists). Nil is config.DefaultFoldedLists; an empty,
	// non-nil list folds none.
	FoldedLists []string
	// DiffSplit starts the diff preview in the split layout (diff.layout =
	// "split"); S switches layouts for the session.
	DiffSplit bool
	// DiffTree starts the Files section in the tree view (diff.view =
	// "tree"); d t switches views for the session.
	DiffTree bool
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
	// BinaryChanged says whether the deck's binary was replaced and a new
	// one is ready. It runs off the UI goroutine on every Tick; when it
	// says yes the program quits and Restart reports true. Nil never
	// restarts.
	BinaryChanged func(context.Context) bool
	// CheckUpdate looks for a newer deck. It runs off the UI goroutine at
	// start and every UpdateEvery (zero is DefaultUpdateEvery). Nil shows
	// no hint.
	CheckUpdate func(context.Context) Update
	UpdateEvery time.Duration
	// RestartFailed says why an earlier restart did not work; the header
	// and the Sources view show it.
	RestartFailed string
	// Settings backs the settings page (s); nil turns it off.
	Settings *SettingsHooks
	// Changelog is this deck's own changelog, which What's new shows.
	Changelog changelog.Log
	// Updated is the X.Y.Z this deck was just updated to: the footer says
	// so until the first key. "" says nothing.
	Updated string
}

// DefaultUpdateEvery is how often CheckUpdate runs.
const DefaultUpdateEvery = time.Hour

// Update is the outcome of an update check.
type Update struct {
	// Available names the newer version (a tag or a short commit); ""
	// when there is none.
	Available string
	// Problem says why the check failed. It shows only in Sources.
	Problem string
	// News is what the newer version's changelog lists beyond this deck,
	// for What's new; empty when it could not be read.
	News []changelog.Release
}

// RefreshMsg asks the model to reload its snapshot, e.g. after the project
// folder changed. Reloads are coalesced: one runs at a time.
type RefreshMsg struct{}

type (
	tickMsg       struct{}
	updateTickMsg struct{}
	binaryMsg     bool
	updateMsg     Update
	updateOnceMsg Update // a check outside the hourly round
	snapshotMsg   deck.Snapshot
	openedMsg     struct {
		what string
		err  error
		verb string // "open" when empty
	}
	devUpMsg struct {
		status string
		err    error
	}
	diffMsg struct {
		key  string
		diff deck.Diff
	}
)

// drawerSize cycles with z.
type drawerSize int

const (
	sizeNormal drawerSize = iota // half of the pane
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
	tab       tabKind       // the drawer tab chosen; curTab is the one shown
	dfocus    bool          // the drawer has the focus (tab), not the list
	dcur      int           // the drawer cursor: a stop of the tab's content
	tree      bool          // the Files tab shows a folder tree
	desktop   bool          // the Figma chooser opens the desktop app
	status    string        // a one-off message in the footer
	notice    string        // "Updated to …", until the first key or click

	width, height int
	light         bool // the terminal's background is light

	loaded      bool // a snapshot came from Load (or there is no Load)
	loading     bool
	pending     bool // a reload was asked for while one ran
	sourcesSeen bool // the Sources view came up by itself once

	checkingBinary bool   // a BinaryChanged call is running
	restart        bool   // quit to run the new binary
	update         Update // the last update check

	diffs     map[string]deck.Diff // the last diff read, by diffKey
	diffSel   string               // the diffKey last asked for
	diffing   bool                 // a Diff call is running
	diffAgain bool                 // the selection moved while it ran

	commitLists  map[string]deck.Commits // the last commits read, by diffKey
	commitsSel   string                  // the diffKey last asked for
	committing   bool                    // a Commits call is running
	commitsAgain bool                    // the selection moved while it ran
	// Expanded commits (by commitKey), their files once read, and the
	// reads running.
	expand         map[string]bool
	commitFileSets map[string]commitFiles
	filesReading   map[string]bool

	// The diff preview (preview.go): on, the thread (diffKey) and file
	// (patchKey) or commit (commitKey) it shows, and its scroll.
	preview    bool
	prevTab    tabKind
	prevThread string
	prevKey    string
	prevOff    int
	prevSplit  bool               // prevOff counts rows of the split layout
	split      bool               // the split layout is chosen (it shows when wide enough)
	patches    map[string]preview // the last patch read, by patchKey (one)
	patchSel   string             // the patchKey last asked for
	patching   bool               // a Patch call is running
	patchAgain bool               // the preview moved while it ran

	set  settingsPage
	pick picker
}

// New returns a model showing snap until Options.Load delivers a fresh one.
func New(snap deck.Snapshot, opt Options) Model {
	if opt.Now == nil {
		opt.Now = time.Now
	}
	if opt.Tick == 0 {
		opt.Tick = DefaultTick
	}
	if opt.UpdateEvery == 0 {
		opt.UpdateEvery = DefaultUpdateEvery
	}
	if opt.FoldedLists == nil {
		opt.FoldedLists = config.DefaultFoldedLists
	}
	m := Model{
		opt:            opt,
		keys:           defaultKeys(),
		folds:          map[string]bool{},
		diffs:          map[string]deck.Diff{},
		commitLists:    map[string]deck.Commits{},
		expand:         map[string]bool{},
		commitFileSets: map[string]commitFiles{},
		filesReading:   map[string]bool{},
		patches:        map[string]preview{},
		choosing:       noKind,
		tree:           opt.DiffTree,
		split:          opt.DiffSplit,
		width:          defaultWidth,
		height:         defaultHeight,
		loaded:         opt.Load == nil,
		loading:        opt.Load != nil, // Init starts the first load
	}
	if opt.Updated != "" {
		m.notice = "Updated to v" + opt.Updated + " · " + m.keys.News.Keys()[0] + " what's new"
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
	m.rows = buildRows(snap, m.folds, m.opt.FoldedLists)

	switch {
	case !m.moved:
		m.cursor = m.homeRow()
		// The cursor jumped to what needs the user: Next says why.
		if r, ok := m.selected(); ok && r.key != selKey && r.needsYou() {
			m.tab = tabOverview
		}
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
		m.drawerOff, m.dcur = 0, 0
		if m.mode == modeReport {
			m.mode = modeRow
		}
	}
	if m.pick.open {
		m.pickVisible()
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
		if r.kind == rowWork {
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

// Restart says the model quit because a new binary is ready to run.
func (m Model) Restart() bool { return m.restart }

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
	cmds := []tea.Cmd{tea.RequestBackgroundColor}
	if m.opt.Load != nil {
		cmds = append(cmds, m.load(), m.tick())
	} else if m.opt.BinaryChanged != nil {
		cmds = append(cmds, m.tick())
	}
	if m.opt.CheckUpdate != nil {
		cmds = append(cmds, m.checkUpdate())
	}
	return tea.Batch(cmds...)
}

func (m Model) checkUpdate() tea.Cmd {
	check := m.opt.CheckUpdate
	return func() tea.Msg { return updateMsg(check(context.Background())) }
}

func (m Model) updateTick() tea.Cmd {
	return tea.Tick(m.opt.UpdateEvery, func(time.Time) tea.Msg { return updateTickMsg{} })
}

// checkBinary asks whether a new binary is ready, one call at a time.
func (m *Model) checkBinary() tea.Cmd {
	if m.opt.BinaryChanged == nil || m.checkingBinary || m.restart {
		return nil
	}
	m.checkingBinary = true
	changed := m.opt.BinaryChanged
	return func() tea.Msg { return binaryMsg(changed(context.Background())) }
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
		m.relayoutPreview()
		m.scrollPreview(0)
		if m.pick.open {
			m.pickVisible()
		}
	case tea.BackgroundColorMsg:
		m.light = !msg.IsDark()
	case tickMsg:
		return m, tea.Batch(m.refresh(), m.checkBinary(), m.tick())
	case binaryMsg:
		m.checkingBinary = false
		if msg {
			m.restart = true
			return m, tea.Quit
		}
	case updateTickMsg:
		return m, m.checkUpdate()
	case updateMsg:
		m.update = Update(msg)
		return m, m.updateTick()
	case updateOnceMsg:
		m.update = Update(msg)
	case settingsMsg:
		if msg.err != nil {
			m.set.err = msg.err.Error()
		} else {
			m.set.cfg, m.set.loaded, m.set.err = msg.cfg, true, ""
		}
		m.followSetting()
	case settingsSavedMsg:
		cmd := m.settingsSaved(msg)
		m.followSetting()
		return m, cmd
	case RefreshMsg:
		return m, m.refresh()
	case snapshotMsg:
		m.loading = false
		m.loaded = true
		m.SetSnapshot(deck.Snapshot(msg))
		m.syncPreview()
		diff := tea.Batch(m.readDiff(true), m.readCommits(true), m.readPatch(true))
		if m.pending {
			m.pending = false
			return m, tea.Batch(m.refresh(), diff)
		}
		return m, diff
	case diffMsg:
		m.diffing = false
		m.diffs[msg.key] = msg.diff
		m.syncPreview()
		if m.diffAgain {
			m.diffAgain = false
			return m, tea.Batch(m.readDiff(true), m.readPatch(false))
		}
		return m, m.readPatch(false)
	case commitsMsg:
		m.committing = false
		m.setCommits(msg.key, msg.commits)
		m.syncPreview()
		if m.commitsAgain {
			m.commitsAgain = false
			return m, tea.Batch(m.readCommits(true), m.readPatch(false))
		}
		return m, m.readPatch(false)
	case commitFilesMsg:
		m.setCommitFiles(msg)
		m.syncPreview()
		return m, m.readPatch(false)
	case patchMsg:
		m.patching = false
		// Only the shown file's patch is drawn: keep that one alone.
		clear(m.patches)
		m.patches[msg.key] = msg.data
		m.scrollPreview(0)
		if m.patchAgain {
			m.patchAgain = false
			return m, m.readPatch(false)
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
		next, cmd := m.handleKey(msg)
		return next.(Model).followDiff(cmd)
	case tea.MouseClickMsg:
		next, cmd := m.handleClick(msg.Mouse())
		return next.(Model).followDiff(cmd)
	case tea.MouseWheelMsg:
		m.handleWheel(msg.Mouse())
		return m.followDiff(nil)
	}
	return m, nil
}

// followDiff adds a diff read to cmd when the selection moved to another
// thread, and a patch read when the preview moved to another file.
func (m Model) followDiff(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.syncPreview()
	return m, tea.Batch(cmd, m.readDiff(false), m.readCommits(false), m.readPatch(false))
}

// diffThread is the selected row's thread whose worktree the Files section
// shows: not for a resolved thread, whose worktree is likely gone.
func (m Model) diffThread() (deck.Thread, bool) {
	r, _ := m.selected()
	t, ok := r.thread()
	if r.kind != rowWork || !ok || t.Worktree == "" || t.Status == deck.StatusDone {
		return deck.Thread{}, false
	}
	return t, true
}

func diffKey(t deck.Thread) string { return t.ID + "\x00" + t.Worktree + "\x00" + t.Base }

// diff is the selected thread's last diff, when one was read.
func (m Model) diff() (deck.Thread, deck.Diff, bool) {
	t, ok := m.diffThread()
	if !ok || m.opt.Diff == nil {
		return t, deck.Diff{}, false
	}
	d, ok := m.diffs[diffKey(t)]
	return t, d, ok
}

// readDiff reads the selected thread's diff, again when force is set, else
// only when the selection moved to another thread. One read runs at a
// time; a move meanwhile reads again when it ends.
func (m *Model) readDiff(force bool) tea.Cmd {
	t, ok := m.diffThread()
	if m.opt.Diff == nil || !ok {
		return nil
	}
	k := diffKey(t)
	if !force && k == m.diffSel {
		return nil
	}
	if m.diffing {
		m.diffAgain = true
		return nil
	}
	m.diffing, m.diffSel = true, k
	read := m.opt.Diff
	return func() tea.Msg { return diffMsg{key: k, diff: read(context.Background(), t)} }
}

// diffKeyPressed focuses the drawer's Files tab, so a digit then previews
// that file and d (filesOnlyKey) opens the file under the cursor in the
// diff tool.
func (m *Model) diffKeyPressed() tea.Cmd {
	if m.opt.Diff == nil {
		m.status = "the Files tab is off"
		return nil
	}
	if _, ok := m.diffThread(); !ok {
		r, _ := m.selected()
		if t, has := r.thread(); r.kind == rowWork && has && t.Status == deck.StatusDone && t.Worktree != "" {
			m.status = t.ID + " is resolved"
		} else {
			m.status = "no worktree on this row"
		}
		return nil
	}
	if m.mode != modeRow || m.curTab() != tabFiles {
		m.switchTab(tabFiles)
	}
	if m.size == sizeHidden {
		m.size = sizeNormal
		m.ensureVisible()
	}
	m.dfocus = true
	return nil
}

// previewFileAt shows the file at display index i in the preview, with
// the Files tab focused and its cursor on that file. Without a preview
// (Options.Patch nil) it opens the file in the diff tool instead.
func (m *Model) previewFileAt(i int) tea.Cmd {
	t, d, ok := m.diff()
	if !ok || d.Note != "" || i < 0 || i >= len(d.Files) {
		m.status = fmt.Sprintf("no file %d on this row", i+1)
		return nil
	}
	m.dfocus = true
	m.dcur = i // every file is a stop, in display order
	m.moveDrawerCursor(0)
	if m.opt.Patch == nil {
		return m.openFile(t, d, i)
	}
	if !m.preview {
		m.togglePreview()
	}
	return nil
}

// openFile opens the file at display index i in the diff tool.
func (m *Model) openFile(t deck.Thread, d deck.Diff, i int) tea.Cmd {
	if i < 0 || i >= len(d.Files) {
		m.status = fmt.Sprintf("no file %d on this row", i+1)
		return nil
	}
	f := fileOrder(d.Files, m.tree)[i]
	if f.Untracked {
		m.status = f.Path + " is untracked: git diff shows it once it is added"
		return nil
	}
	return m.openDiff(t, d, f)
}

// filesOnlyKey handles the Files tab's own keys: t switches list and tree,
// a opens the whole diff in the diff tool, v turns the preview on and
// off, and in the focused drawer d opens the file under the cursor in the
// diff tool. done is false for any other key, so d from the list focuses
// Files.
func (m *Model) filesOnlyKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if m.mode != modeRow || m.curTab() != tabFiles {
		return nil, false
	}
	switch msg.String() {
	case "t":
		m.tree = !m.tree
		m.dcur = 0
		return nil, true
	case "a":
		if t, d, ok := m.diff(); ok && len(d.Files) > 0 {
			return m.openDiff(t, d, deck.DiffFile{}), true
		}
		return nil, true
	case "v":
		m.togglePreview()
		return nil, true
	case "d":
		if !m.dfocus {
			return nil, false
		}
		if t, d, ok := m.diff(); ok && d.Note == "" && len(d.Files) > 0 {
			return m.openFile(t, d, m.dcur), true
		}
		return nil, true
	}
	return nil, false
}

// switchTab shows tab in the drawer, from its top.
func (m *Model) switchTab(tab tabKind) {
	m.setMode(modeRow)
	m.choosing = noKind
	m.tab = tab
	m.dcur = 0
}

// cycleTab moves to the next (dir 1) or previous (-1) tab that has
// something behind it.
func (m *Model) cycleTab(dir int) {
	r, ok := m.selected()
	if !ok || r.kind != rowWork {
		return
	}
	tab := m.curTab()
	for range numTabs {
		tab = (tab + tabKind(dir) + numTabs) % numTabs
		if m.tabEnabled(r, tab) {
			break
		}
	}
	m.switchTab(tab)
}

// moveDrawerCursor moves the drawer cursor by delta stops and scrolls the
// drawer to keep it in view.
func (m *Model) moveDrawerCursor(delta int) {
	l := m.layout()
	if l.drawer == nil || len(l.drawer.stops) == 0 {
		m.scrollDrawer(delta)
		return
	}
	lo := 0
	if _, cs, ok := m.commits(); ok && m.preview && m.curTab() == tabCommits && len(cs.List) > 0 {
		lo = m.commitStop(cs, 0) // the preview stays on the commits
	}
	m.dcur = clamp(m.dcur+delta, lo, len(l.drawer.stops)-1)
	line := l.drawer.stops[m.dcur].line
	if line < m.drawerOff {
		m.drawerOff = line
	}
	if line >= m.drawerOff+l.drawerH {
		m.drawerOff = line - l.drawerH + 1
	}
	m.scrollDrawer(0)
}

// act does what a click or enter on a drawer stop does.
func (m *Model) act(a action) tea.Cmd {
	switch a.kind {
	case actLink:
		return m.openNumbered(a.n)
	case actFile, actDiff:
		t, d, ok := m.diff()
		switch {
		case !ok:
			return nil
		case a.kind == actDiff:
			return m.openDiff(t, d, deck.DiffFile{})
		}
		return m.previewFileAt(a.n - 1)
	case actEvent:
		switch kind, n := m.eventAction(a.n); kind {
		case cmdReport:
			m.setMode(modeReport)
		case cmdLink:
			return m.openNumbered(n)
		case cmdPane:
			return m.focusPane()
		}
	case actTab:
		if r, ok := m.selected(); ok && m.tabEnabled(r, tabKind(a.n)) {
			m.switchTab(tabKind(a.n))
		}
	case actCommit:
		if a.n == 0 {
			// The uncommitted row: those changes are the Files tab's.
			if r, ok := m.selected(); ok && m.tabEnabled(r, tabFiles) {
				m.switchTab(tabFiles)
				m.dfocus = true
			}
			return nil
		}
		m.previewCommit(a.n - 1)
	case actExpand:
		return m.toggleExpand(a.n-1, -1)
	case actCommitFile:
		m.previewCommitFile(a.n-1, a.f)
	}
	return nil
}

// openDiff opens the diff tool on the thread's worktree against the merge
// base, so it shows what the Files section counts: one file, or with a
// zero file the whole diff. A rename passes both paths so git pairs them.
// Untracked files are not in git's diff.
func (m *Model) openDiff(t deck.Thread, d deck.Diff, f deck.DiffFile) tea.Cmd {
	open := m.opt.OpenDiff
	what := "the diff of " + t.ID
	var files []string
	if f.Path != "" {
		what = "the diff of " + f.Path
		if f.OldPath != "" {
			files = append(files, f.OldPath)
		}
		files = append(files, f.Path)
	}
	if open == nil {
		m.status = "opening diffs is off"
		return nil
	}
	base := d.MergeBase
	if base == "" {
		base = d.Base
	}
	path := t.Worktree
	return func() tea.Msg { return openedMsg{what: what, err: open(path, base, files)} }
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.status, m.notice = "", ""
	if m.pick.open {
		return m, m.pickerKey(msg)
	}
	if m.mode == modeSettings {
		if cmd, done := m.settingsKey(msg); done {
			return m, cmd
		}
	}
	if m.choosing != noKind {
		if cmd, done := m.chooserKey(msg); done {
			return m, cmd
		}
	}
	if s := msg.String(); len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		if m.mode == modeRow && m.curTab() == tabCommits {
			m.previewCommit(int(s[0] - '1'))
			return m, nil
		}
		if m.mode == modeRow && m.curTab() == tabFiles {
			return m, m.previewFileAt(int(s[0] - '1'))
		}
		return m, m.openNumbered(int(s[0] - '1'))
	}
	if cmd, done := m.filesOnlyKey(msg); done {
		return m, cmd
	}
	if cmd, done := m.commitsOnlyKey(msg); done {
		return m, cmd
	}
	if m.previewKey(msg) {
		return m, nil
	}
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.NextTab):
		m.cycleTab(1)
	case key.Matches(msg, m.keys.PrevTab):
		m.cycleTab(-1)
	case key.Matches(msg, m.keys.Focus):
		if m.dfocus {
			m.cycleTab(1)
		} else if r, ok := m.selected(); ok && r.kind == rowWork && m.size != sizeHidden {
			m.setMode(modeRow)
			m.dfocus, m.dcur = true, 0
		}
	case key.Matches(msg, m.keys.Unfocus):
		if m.dfocus {
			m.cycleTab(-1)
		}
	case m.dfocus && key.Matches(msg, m.keys.Down):
		m.moveDrawerCursor(1)
	case m.dfocus && key.Matches(msg, m.keys.Up):
		m.moveDrawerCursor(-1)
	case m.dfocus && key.Matches(msg, m.keys.Pane):
		if l := m.layout(); l.drawer != nil && m.dcur < len(l.drawer.stops) {
			return m, m.act(l.drawer.stops[m.dcur].act)
		}
		return m, m.focusPane()
	case m.dfocus && key.Matches(msg, m.keys.Back):
		m.dfocus = false
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
	case key.Matches(msg, m.keys.Diff):
		return m, m.diffKeyPressed()
	case key.Matches(msg, m.keys.Report):
		m.toggleReport()
	case key.Matches(msg, m.keys.Drawer):
		m.size = (m.size + 1) % 3
		if m.size == sizeHidden {
			m.dfocus = false
		}
		m.drawerOff = 0
		m.ensureVisible()
	case key.Matches(msg, m.keys.Sources):
		m.toggleMode(modeSources)
	case key.Matches(msg, m.keys.Help):
		m.toggleMode(modeHelp)
	case key.Matches(msg, m.keys.Settings):
		return m, m.openSettings()
	case key.Matches(msg, m.keys.News):
		m.toggleMode(modeNews)
	case key.Matches(msg, m.keys.Projects):
		m.openPicker()
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
	m.dcur = 0
	m.setMode(modeRow)
	m.ensureVisible()
}

func (m *Model) setMode(mode drawerMode) {
	m.mode = mode
	if mode != modeRow {
		m.dfocus = false // a full view has no drawer cursor
	}
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
	m.rows = buildRows(m.snap, m.folds, m.opt.FoldedLists)
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
	return rowLinks(r)
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
	m.tab = tabOverview // the chooser highlights Overview's chips
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
		if m.snap.LinearKey {
			m.status = what + ": no URL from Linear yet"
		}
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

// focusPane focuses the herdr pane of the selected row's thread.
func (m *Model) focusPane() tea.Cmd {
	r, _ := m.selected()
	t, ok := r.thread()
	switch {
	case r.kind != rowWork:
		m.status = "no pane on this row"
		return nil
	case !ok:
		m.status = "no thread on this row"
		return nil
	}
	id := ""
	switch {
	case t.Pane != nil:
		id = t.Pane.ID
	case !m.snap.Herdr && t.PaneID != "":
		// herdr's live state is unknown: try the recorded pane.
		id = t.PaneID
	default:
		m.status = t.ID + " has no open pane"
		return nil
	}
	focus := m.opt.FocusPane
	if focus == nil {
		m.status = "focusing panes is off: " + id
		return nil
	}
	label := t.ID + "'s pane " + id
	return func() tea.Msg { return openedMsg{what: label, err: focus(id), verb: "focus"} }
}

func (m *Model) handleClick(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	if mouse.Button != tea.MouseLeft {
		return *m, nil
	}
	m.status, m.notice = "", ""
	if m.pick.open {
		return *m, m.pickClick(mouse.Y)
	}
	l := m.layout()
	switch {
	case l.attn > 0 && mouse.Y == l.attn:
		m.openPicker()
	case mouse.Y == 0 && mouse.X >= l.bang.x0 && mouse.X < l.bang.x1:
		m.toggleMode(modeSources)
	case m.preview && mouse.Y >= l.listTop-1 && mouse.Y < l.listTop+l.listH:
		// The preview has the list's place: a click there does nothing.
	case mouse.Y >= l.listTop && mouse.Y < l.listTop+l.listH:
		i := m.listOff + mouse.Y - l.listTop
		if i >= len(m.rows) || m.rows[i].kind == rowGap {
			break
		}
		if i != m.cursor {
			m.dcur = 0
		}
		m.cursor, m.moved = i, true
		m.choosing, m.dfocus = noKind, false
		m.setMode(modeRow)
		if m.rows[i].kind == rowHeading {
			m.toggleFold()
		}
	case l.drawer != nil && l.tabY >= 0 && mouse.Y == l.tabY:
		m.focusDrawer()
		for _, z := range l.drawer.tabZones {
			if mouse.X >= z.x0 && mouse.X < z.x1 {
				return *m, m.act(z.act)
			}
		}
	case mouse.Y >= l.drawerTop && mouse.Y < l.drawerTop+l.drawerH:
		m.scrollDrawer(0) // match the offset the screen was drawn with
		dl := l.drawer.lines
		i := m.drawerOff + mouse.Y - l.drawerTop
		if i >= len(dl) {
			break
		}
		if s := dl[i].setting; s > 0 && !m.set.editing {
			m.set.cursor = s - 1
			m.followSetting()
			break
		}
		m.focusDrawer()
		for _, z := range dl[i].zones {
			if mouse.X < z.x0 || mouse.X >= z.x1 {
				continue
			}
			m.choosing = noKind
			for k, st := range l.drawer.stops {
				if st.line == i && st.act == z.act {
					m.dcur = k
				}
			}
			return *m, m.act(z.act)
		}
	}
	return *m, nil
}

// focusDrawer gives a row's drawer the focus, as a click in it does.
func (m *Model) focusDrawer() {
	if r, ok := m.selected(); ok && r.kind == rowWork && m.mode == modeRow && m.size != sizeHidden {
		m.dfocus = true
	}
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
	if m.pick.open {
		m.pickMove(delta)
		return
	}
	l := m.layout()
	switch {
	case m.preview && mouse.Y >= l.listTop-1 && mouse.Y < l.listTop+l.listH:
		m.scrollPreview(delta * 3)
	case mouse.Y >= l.listTop && mouse.Y < l.listTop+l.listH:
		m.move(delta)
	case l.drawerH > 0 && mouse.Y >= l.listTop+l.listH && mouse.Y < l.drawerTop+l.drawerH:
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

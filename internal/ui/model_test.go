package ui

import (
	"context"
	"errors"
	"flag"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/fake"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden from the current rendering")

// testVersion stands in for the build's version so the goldens do not
// depend on it.
const testVersion = "v9.9.9"

var now = time.Date(2026, 10, 2, 14, 41, 0, 0, time.UTC)

// opened records what the model asked to open; nothing is ever opened.
type opened struct{ urls, dirs, panes []string }

func newModel(t *testing.T, snap deck.Snapshot, w, h int) (Model, *opened) {
	t.Helper()
	o := &opened{}
	m := New(snap, Options{
		Now:        func() time.Time { return now },
		Location:   time.UTC,
		OpenURL:    func(u string) error { o.urls = append(o.urls, u); return nil },
		OpenEditor: func(p string) error { o.dirs = append(o.dirs, p); return nil },
		FocusPane:  func(id string) error { o.panes = append(o.panes, id); return nil },
		Version:    testVersion,
	})
	m, _ = press(m, tea.WindowSizeMsg{Width: w, Height: h})
	return m, o
}

// press feeds messages to the model, running every command it returns until
// none is left, so an open link lands in opened.
func press(m Model, msgs ...tea.Msg) (Model, tea.Cmd) {
	var last tea.Cmd
	for _, msg := range msgs {
		next, cmd := m.Update(msg)
		m = next.(Model)
		last = cmd
		m = run(m, cmd)
	}
	return m, last
}

func run(m Model, cmd tea.Cmd) Model {
	if cmd == nil {
		return m
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			m = run(m, c)
		}
	case openedMsg:
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func keys(s string) []tea.Msg {
	var out []tea.Msg
	for _, r := range s {
		switch r {
		case ' ':
			out = append(out, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
		default:
			out = append(out, tea.KeyPressMsg{Code: r, Text: string(r)})
		}
	}
	return out
}

var esc = tea.KeyPressMsg{Code: tea.KeyEscape}

// screen is the pane as plain text, without trailing spaces.
func screen(m Model) string {
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}

// golden compares the plain-text screen with testdata/<name>.golden.
func golden(t *testing.T, name string, m Model) {
	t.Helper()
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	if len(lines) != m.height {
		t.Errorf("%s: %d lines, want the pane height %d", name, len(lines), m.height)
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > m.width {
			t.Errorf("%s: line %d is %d wide, more than %d: %q", name, i, w, m.width, l)
		}
	}
	got := screen(m)
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(path, []byte(got+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test ./internal/ui -update)", err)
	}
	if got+"\n" != string(want) {
		t.Errorf("%s differs from %s:\n%s", name, path, got)
	}
}

func fakeSnap() deck.Snapshot { return fake.Snapshot("admin-rebuild", now) }

// calm is the mockups' normal use: nothing needs the user.
func calm() deck.Snapshot {
	s := fakeSnap()
	s.Inbox = nil
	s.Threads[1].Status = deck.StatusWorking
	s.Threads[1].StateLine = "working · ~60%"
	s.Threads[1].Activity = "Building overview"
	s.Threads[1].Next = nil
	return s
}

func TestGolden(t *testing.T) {
	missing := calm()
	missing.Missing = []string{
		".state/ticker.json: no such file or directory",
		"thread list: herdr-projects: no session; showing threads/*.toml as last recorded",
	}
	missing.ThreadsAsOf = now.Add(-18 * time.Minute)

	cases := []struct {
		name string
		snap deck.Snapshot
		keys []tea.Msg
	}{
		{name: "normal", snap: calm()},
		{name: "needs-you", snap: fakeSnap()},
		{name: "inbox", snap: fakeSnap(), keys: keys("j")},
		{name: "chooser-linear", snap: calm(), keys: keys("l")},
		{name: "chooser-figma", snap: calm(), keys: keys("jjf")},
		{name: "empty", snap: deck.Snapshot{Project: fakeSnap().Project}},
		{name: "missing", snap: missing},
		{name: "backlog-open", snap: calm(), keys: keys("jjjjjj ")},
		{name: "report", snap: fakeSnap(), keys: keys("r")},
		{name: "help", snap: calm(), keys: keys("?")},
		{name: "drawer-hidden", snap: calm(), keys: keys("zz")},
		{name: "dev", snap: calm(), keys: keys("jj")},
		{name: "dev-fallback", snap: calm(), keys: keys("jjjj")},
		{name: "sources-notes", snap: calm(), keys: keys("!")},
	}
	for _, c := range cases {
		for _, w := range []int{80, 60} {
			name := c.name + "-" + map[int]string{80: "80", 60: "60"}[w]
			t.Run(name, func(t *testing.T) {
				m, _ := newModel(t, deck.Snapshot{}, w, 28)
				m, _ = press(m, snapshotMsg(c.snap))
				m, _ = press(m, c.keys...)
				golden(t, name, m)
			})
		}
	}
}

// find returns the screen column and line where text starts.
func find(t *testing.T, m Model, text string) (int, int) {
	t.Helper()
	for y, l := range strings.Split(ansi.Strip(m.View().Content), "\n") {
		if i := strings.Index(l, text); i >= 0 {
			return ansi.StringWidth(l[:i]), y
		}
	}
	t.Fatalf("%q not on screen:\n%s", text, screen(m))
	return 0, 0
}

func click(x, y int) tea.MouseClickMsg {
	return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}
}

func selectedTitle(m Model) string {
	r, _ := m.selected()
	return r.title()
}

func TestMoveAndClamp(t *testing.T) {
	m, _ := newModel(t, calm(), 80, 28)
	if got := selectedTitle(m); got != "Users page" {
		t.Fatalf("start on %q, want the first row of work", got)
	}
	m, _ = press(m, keys("kkk")...)
	if m.cursor != 0 {
		t.Fatalf("k past the top: cursor %d", m.cursor)
	}
	m, _ = press(m, keys("jjjjjjjjjjjj")...)
	if got := selectedTitle(m); got != "Backlog" {
		t.Fatalf("j past the bottom: on %q, want the folded Backlog heading", got)
	}
}

// A Linear ID read without a workspace has no URL: the drawer says so and
// opening it names the flag and variable instead of opening a broken URL.
func TestLinearWithoutWorkspace(t *testing.T) {
	snap := calm()
	links := snap.TaskLists[0].Tasks[0].Links
	for i := range links {
		links[i].URL = ""
	}
	m, o := newModel(t, snap, 100, 28)
	if !strings.Contains(screen(m), "no Linear workspace") {
		t.Errorf("drawer has no workspace hint:\n%s", screen(m))
	}
	m, _ = press(m, keys("1")...)
	if len(o.urls) != 0 {
		t.Errorf("opened %q, want nothing", o.urls)
	}
	if !strings.Contains(m.Status(), "ABC-1246: no Linear workspace") || !strings.Contains(m.Status(), deck.EnvLinearWorkspace) {
		t.Errorf("status = %q", m.Status())
	}
}

func TestDigitsAndLetterKeysOpenLinks(t *testing.T) {
	m, o := newModel(t, calm(), 80, 28)
	m, _ = press(m, keys("2")...)
	if want := "https://linear.app/acme/issue/ABC-1256"; len(o.urls) != 1 || o.urls[0] != want {
		t.Fatalf("2 opened %q, want %q", o.urls, want)
	}
	if !strings.Contains(m.Status(), "opened ABC-1256") {
		t.Errorf("status = %q", m.Status())
	}
	m, _ = press(m, keys("9")...)
	if len(o.urls) != 1 || !strings.Contains(m.Status(), "no link 9") {
		t.Errorf("9 on a row with 4 links: opened %q, status %q", o.urls, m.Status())
	}
	m, _ = press(m, keys("f")...)
	if !strings.HasPrefix(m.Status(), "no Figma link") {
		t.Errorf("f without Figma links: status %q", m.Status())
	}

	// Document select: one PR, so g opens it at once.
	m, _ = press(m, keys("jjg")...)
	if got := o.urls[len(o.urls)-1]; got != "https://github.com/acme/webshop/pull/2320" {
		t.Errorf("g opened %q", got)
	}
	// Templates page: one localhost link, from its thread.
	press(m, keys("ko")...)
	if got := o.urls[len(o.urls)-1]; got != "http://localhost:5181" {
		t.Errorf("o opened %q", got)
	}
}

func TestLocalhostKeyOpensFirstRunning(t *testing.T) {
	snap := calm()
	// Document select: API docs is down; make Frontend down too, and add a
	// running link after them.
	th := &snap.Threads[3]
	th.Links[1].Down = true
	th.Links = append(th.Links, deck.Link{Kind: deck.LinkLocalhost, Label: "Storybook", URL: "http://localhost:6006"})
	m, o := newModel(t, snap, 80, 28)
	m, _ = press(m, keys("jjo")...)
	if len(o.urls) != 1 || o.urls[0] != "http://localhost:6006" || m.choosing != noKind {
		t.Fatalf("o opened %q (choosing %v), want the running Storybook link", o.urls, m.choosing)
	}

	// The fallback port does not listen: o says so and opens nothing, but
	// its digit still opens it.
	m, _ = press(m, keys("jjo")...)
	if len(o.urls) != 1 || !strings.Contains(m.Status(), "no dev server on this row is running") {
		t.Errorf("o with every server down: opened %q, status %q", o.urls, m.Status())
	}
	m, _ = press(m, keys("3")...)
	if got := o.urls[len(o.urls)-1]; got != "http://localhost:14437" {
		t.Errorf("3 opened %q", got)
	}
	m, _ = press(m, keys("ko")...)
	if !strings.HasPrefix(m.Status(), "no localhost link") {
		t.Errorf("o without localhost links: status %q", m.Status())
	}
}

func TestChooser(t *testing.T) {
	m, o := newModel(t, calm(), 80, 28)

	m, _ = press(m, keys("l")...)
	if m.choosing != deck.LinkLinear || len(o.urls) != 0 {
		t.Fatalf("l with four Linear links: choosing %v, opened %q", m.choosing, o.urls)
	}
	if !strings.Contains(screen(m), "Linear: 1-4 open  a all  l first  esc cancel") {
		t.Errorf("footer does not ask for a digit:\n%s", screen(m))
	}
	m, _ = press(m, keys("3")...)
	if m.choosing != noKind || len(o.urls) != 1 || !strings.HasSuffix(o.urls[0], "ABC-1257") {
		t.Fatalf("3 in the chooser: choosing %v, opened %q", m.choosing, o.urls)
	}

	m, _ = press(m, keys("ll")...)
	if !strings.HasSuffix(o.urls[1], "ABC-1246") {
		t.Errorf("l l opened %q, want the first", o.urls[1])
	}
	m, _ = press(m, keys("la")...)
	if len(o.urls) != 6 {
		t.Errorf("l a opened %d links, want 2 + 4", len(o.urls))
	}
	m, _ = press(m, append(keys("l"), esc)...)
	if m.choosing != noKind || len(o.urls) != 6 {
		t.Errorf("esc: choosing %v, opened %d", m.choosing, len(o.urls))
	}
	// Another key closes the chooser and still does its own job.
	m, _ = press(m, keys("lj")...)
	if m.choosing != noKind || selectedTitle(m) != "Templates page" {
		t.Errorf("j in the chooser: choosing %v, on %q", m.choosing, selectedTitle(m))
	}

	// Figma: d then a digit opens the desktop app.
	m, _ = press(m, keys("jfd")...)
	if !strings.Contains(screen(m), "Figma desktop: 2-4 open") {
		t.Errorf("after f d the footer is:\n%s", screen(m))
	}
	m, _ = press(m, keys("3")...)
	if got := o.urls[len(o.urls)-1]; got != "figma://design/abc123/Document-select?node-id=1138-88367" {
		t.Errorf("f d 3 opened %q", got)
	}
	m, _ = press(m, keys("fd1")...)
	if !strings.Contains(m.Status(), "not a Figma link") {
		t.Errorf("f d 1 (a Linear link): status %q", m.Status())
	}
}

func TestNeedsYouPullsCursorUntilMoved(t *testing.T) {
	m, _ := newModel(t, calm(), 80, 28)
	m, _ = press(m, snapshotMsg(fakeSnap()))
	if got := selectedTitle(m); got != "Users page" || !m.rows[m.cursor].needsYou() {
		t.Fatalf("cursor on %q, want the needs-you row", got)
	}
	if m.rows[0].list != listNeedsYou {
		t.Fatalf("first row is %q, want the Needs you heading", m.rows[0].list)
	}

	// Moved by hand, the cursor stays on its row across refreshes, even
	// when a needs-you row appears above it.
	m, _ = newModel(t, calm(), 80, 28)
	m, _ = press(m, keys("jj")...)
	m, _ = press(m, snapshotMsg(fakeSnap()))
	if got := selectedTitle(m); got != "Document select for summary" {
		t.Fatalf("after refresh the cursor is on %q", got)
	}
}

func TestFolding(t *testing.T) {
	m, _ := newModel(t, calm(), 80, 28)
	if !strings.Contains(screen(m), "+ Backlog (3)") || strings.Contains(screen(m), "Settings page") {
		t.Fatalf("Backlog does not start folded:\n%s", screen(m))
	}
	m, _ = press(m, keys("jjjjjj ")...)
	if !strings.Contains(screen(m), "Settings page") {
		t.Fatalf("space did not unfold Backlog:\n%s", screen(m))
	}
	// The fold sticks across refreshes.
	m, _ = press(m, snapshotMsg(calm()))
	if !strings.Contains(screen(m), "Settings page") {
		t.Fatalf("refresh folded Backlog again")
	}
	// space on a heading folds it, with its threads' glyphs shown.
	m, _ = press(m, keys("kkkkkk ")...)
	if !strings.Contains(screen(m), "+ In progress (3) ◐ ◐ ◇") {
		t.Fatalf("folded In progress lacks its glyphs:\n%s", screen(m))
	}
	// space on a work row does nothing.
	before := len(m.rows)
	m, _ = press(m, keys("jj ")...)
	if len(m.rows) != before {
		t.Fatalf("space on a work row changed the list")
	}
}

// A blank line separates two groups, never ends the list, and the cursor,
// the wheel and clicks all step over it.
func TestGapsBetweenGroups(t *testing.T) {
	m, _ := newModel(t, calm(), 80, 28)
	lines := strings.Split(screen(m), "\n")
	for _, heading := range []string{"On hold until Monday", "+ Backlog (3)"} {
		_, y := find(t, m, heading)
		if strings.TrimSpace(lines[y-1]) != "" {
			t.Errorf("no blank line above %q:\n%s", heading, screen(m))
		}
	}
	if m.rows[0].kind == rowGap || m.rows[len(m.rows)-1].kind == rowGap {
		t.Errorf("a gap starts or ends the list")
	}

	m, _ = press(m, keys("jj")...)
	if got := selectedTitle(m); got != "Document select for summary" {
		t.Fatalf("on %q, want the last row of In progress", got)
	}
	m, _ = press(m, keys("j")...)
	if got := selectedTitle(m); got != "On hold until Monday 2026-10-05" {
		t.Fatalf("j over the gap: on %q", got)
	}
	m, _ = press(m, keys("k")...)
	if got := selectedTitle(m); got != "Document select for summary" {
		t.Fatalf("k over the gap: on %q", got)
	}
	m, _ = press(m, tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelDown})
	if got := selectedTitle(m); got != "On hold until Monday 2026-10-05" {
		t.Fatalf("wheel over the gap: on %q", got)
	}

	// A click on the gap does nothing; the rows below it still hit.
	_, y := find(t, m, "+ Backlog")
	m, _ = press(m, click(5, y-1))
	if got := selectedTitle(m); got != "On hold until Monday 2026-10-05" || strings.Contains(screen(m), "Settings page") {
		t.Fatalf("click on the gap: on %q", got)
	}
	x, y := find(t, m, "Subscriptions list")
	m, _ = press(m, click(x, y))
	if got := selectedTitle(m); got != "Subscriptions list" {
		t.Fatalf("click below a gap selected %q", got)
	}
}

// Scrolled, a click still maps to the row drawn under it, gaps included.
func TestClickInScrolledListWithGaps(t *testing.T) {
	m, _ := newModel(t, calm(), 60, 14)
	m, _ = press(m, keys("jjjjjj")...)
	if m.listOff == 0 {
		t.Fatalf("list did not scroll:\n%s", screen(m))
	}
	x, y := find(t, m, "Subscriptions list")
	m, _ = press(m, click(x, y))
	if got := selectedTitle(m); got != "Subscriptions list" {
		t.Fatalf("click in a scrolled list selected %q:\n%s", got, screen(m))
	}
}

func TestThreadsNoTaskNames(t *testing.T) {
	s := calm()
	s.Threads = append(s.Threads,
		deck.Thread{ID: "t-0009", Title: "Spike", Status: deck.StatusWorking},
		deck.Thread{ID: "t-0010", Title: "Old work", Status: deck.StatusDone},
	)
	m, _ := newModel(t, s, 80, 40)
	out := screen(m)
	for _, want := range []string{listThreads, "◐ Spike", "+ Resolved (1)"} {
		if !strings.Contains(out, want) {
			t.Errorf("screen lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Old work") {
		t.Errorf("resolved threads should start folded")
	}
}

func TestMouse(t *testing.T) {
	m, o := newModel(t, calm(), 80, 28)

	x, y := find(t, m, "Templates page")
	m, _ = press(m, click(x, y))
	if got := selectedTitle(m); got != "Templates page" {
		t.Fatalf("click selected %q", got)
	}

	m, _ = press(m, keys("k")...)
	x, y = find(t, m, "3 ABC-1257")
	m, _ = press(m, click(x+2, y))
	if len(o.urls) != 1 || !strings.HasSuffix(o.urls[0], "ABC-1257") {
		t.Fatalf("click on drawer link 3 opened %q", o.urls)
	}

	x, y = find(t, m, "+ Backlog")
	m, _ = press(m, click(x, y))
	if !strings.Contains(screen(m), "Settings page") {
		t.Fatalf("click on the Backlog heading did not unfold it")
	}

	m, _ = press(m, tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelUp})
	if got := selectedTitle(m); got != "On hold until Monday 2026-10-05" && got != "Subscriptions list" {
		t.Errorf("wheel up over the list: on %q", got)
	}
}

func TestClickOnMissingCount(t *testing.T) {
	s := calm()
	s.Missing = []string{"inbox: permission denied"}
	m, _ := newModel(t, s, 60, 28)
	if m.mode != modeSources {
		t.Fatalf("the Sources view did not come up by itself")
	}
	m, _ = press(m, esc)
	if m.mode != modeRow {
		t.Fatalf("esc did not return")
	}
	x, y := find(t, m, "! 1")
	m, _ = press(m, click(x, y))
	if m.mode != modeSources {
		t.Fatalf("click on ! 1 did not show Sources")
	}
	m, _ = press(m, keys("!")...)
	if m.mode != modeRow {
		t.Fatalf("! did not toggle Sources off")
	}
	// It comes up by itself only once.
	m, _ = press(m, snapshotMsg(s))
	if m.mode != modeRow {
		t.Fatalf("Sources came up a second time")
	}
}

func TestEditorPaneAndReport(t *testing.T) {
	m, o := newModel(t, fakeSnap(), 80, 28)
	m, _ = press(m, keys("e")...)
	if len(o.dirs) != 1 || o.dirs[0] != "/src/worktrees/t-0002" {
		t.Fatalf("e opened %q", o.dirs)
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(o.panes) != 1 || o.panes[0] != "w1Z:p1" || !strings.Contains(m.Status(), "focused t-0002") {
		t.Errorf("enter: focused %q, status %q", o.panes, m.Status())
	}
	m, _ = press(m, keys("r")...)
	if m.mode != modeReport || !strings.Contains(screen(m), "Phase 1 is done") {
		t.Fatalf("r did not show the report:\n%s", screen(m))
	}
	m, _ = press(m, esc)
	if m.mode != modeRow {
		t.Fatalf("esc did not leave the report")
	}
	m, _ = press(m, keys("jjj")...) // Templates page: no report, no worktree
	m, _ = press(m, keys("r")...)
	if m.mode != modeRow || !strings.Contains(m.Status(), "no report") {
		t.Errorf("r without a report: mode %v, status %q", m.mode, m.Status())
	}
	m, _ = press(m, keys("e")...)
	if len(o.dirs) != 1 || !strings.Contains(m.Status(), "no worktree") {
		t.Errorf("e without a worktree: opened %q, status %q", o.dirs, m.Status())
	}
}

func TestDrawerSizes(t *testing.T) {
	m, _ := newModel(t, calm(), 80, 28)
	if l := m.layout(); l.drawerH == 0 || l.listH == 0 {
		t.Fatalf("normal: %+v", l)
	}
	m, _ = press(m, keys("z")...)
	if l := m.layout(); l.listH != 0 || l.drawerH != 23 {
		t.Fatalf("full: list %d, drawer %d", l.listH, l.drawerH)
	}
	m, _ = press(m, keys("z")...)
	if l := m.layout(); l.drawerH != 0 || l.listH != 23 {
		t.Fatalf("hidden: list %d, drawer %d", l.listH, l.drawerH)
	}
	// ? still shows help in the hidden drawer's place.
	m, _ = press(m, keys("?")...)
	if l := m.layout(); l.drawerH == 0 {
		t.Fatalf("help with the drawer hidden: no drawer")
	}
	m, _ = press(m, keys("z")...)
	if m.size != sizeNormal {
		t.Fatalf("z does not cycle back to normal")
	}
}

func TestLongListScrolls(t *testing.T) {
	s := calm()
	for i := range 30 {
		s.TaskLists[0].Tasks = append(s.TaskLists[0].Tasks, deck.Task{Title: "Task " + string(rune('A'+i%26)) + string(rune('a'+i/26))})
	}
	m, _ := newModel(t, s, 60, 20)
	m, _ = press(m, keys(strings.Repeat("j", 25))...)
	if !strings.Contains(screen(m), "▸· "+selectedTitle(m)) {
		t.Fatalf("selected row %q scrolled out of view:\n%s", selectedTitle(m), screen(m))
	}
	golden(t, "scrolled-60", m)
}

func TestNarrowerThanDesign(t *testing.T) {
	for _, w := range []int{40, 50} {
		m, _ := newModel(t, fakeSnap(), w, 20)
		for i, l := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(l) > w {
				t.Errorf("width %d: line %d is %d wide", w, i, ansi.StringWidth(l))
			}
		}
	}
}

func TestRefreshCoalesces(t *testing.T) {
	loads := 0
	m := New(deck.Snapshot{Project: deck.Project{Slug: "p"}}, Options{
		Load: func(context.Context) deck.Snapshot { loads++; return calm() },
		Now:  func() time.Time { return now },
	})
	if !strings.Contains(screen(m), "Reading p…") {
		t.Errorf("before the first load:\n%s", screen(m))
	}
	if m.Init() == nil {
		t.Fatal("Init with Load returns no command")
	}
	// Init's load counts as running, so a refresh before it ends waits;
	// cmd stands for that first load.
	cmd := m.load()
	next, again := m.Update(RefreshMsg{})
	m = next.(Model)
	if again != nil || !m.pending {
		t.Fatal("a second RefreshMsg during a load started another")
	}
	next, follow := m.Update(cmd())
	m = next.(Model)
	if follow == nil || m.pending || !m.loading {
		t.Fatal("the pending reload did not start after the first finished")
	}
	next, _ = m.Update(follow())
	m = next.(Model)
	if loads != 2 || m.loading || m.Snapshot().Project.Name != "Admin rebuild" {
		t.Fatalf("loads %d, loading %v, project %+v", loads, m.loading, m.Snapshot().Project)
	}
	if _, cmd := m.Update(tickMsg{}); cmd == nil {
		t.Fatal("tick did not reload and re-arm")
	}
}

func TestQuit(t *testing.T) {
	for _, k := range []tea.KeyPressMsg{{Code: 'q', Text: "q"}, {Code: 'c', Mod: tea.ModCtrl}} {
		m, _ := newModel(t, calm(), 80, 28)
		_, cmd := m.Update(k)
		if cmd == nil {
			t.Fatalf("%v: no command", k)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%v: command did not quit", k)
		}
	}
}

func TestDrawerScrollStaysInContent(t *testing.T) {
	s := fakeSnap()
	s.Threads[1].Report = strings.Repeat("line\n", 60)
	m, _ := newModel(t, s, 80, 28)
	m, _ = press(m, keys("r")...)
	for range 10 {
		m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	l := m.layout()
	if most := len(l.drawer.lines) - l.drawerH; m.drawerOff != most {
		t.Fatalf("drawerOff = %d after paging past the end, want %d", m.drawerOff, most)
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyPgUp})
	if m.drawerOff >= len(l.drawer.lines)-l.drawerH {
		t.Fatalf("pgup at the end did not scroll back")
	}
}

func TestReportClosesWhenRefreshMovesCursor(t *testing.T) {
	m, _ := newModel(t, fakeSnap(), 80, 28)
	m, _ = press(m, keys("r")...)
	if m.mode != modeReport {
		t.Fatal("r did not open the report")
	}
	// t-0002 is answered and t-0004 needs the user now; the untouched
	// cursor moves there.
	s := calm()
	s.Threads[3].Status = deck.StatusNeedsYou
	m, _ = press(m, snapshotMsg(s))
	if got := selectedTitle(m); got != "Document select for summary" {
		t.Fatalf("cursor on %q", got)
	}
	if m.mode != modeRow {
		t.Fatalf("the report stayed open on %q", selectedTitle(m))
	}
}

func TestClickAfterGrowingUsesDrawnOffset(t *testing.T) {
	m, o := newModel(t, calm(), 80, 28)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	m, _ = press(m, tea.WindowSizeMsg{Width: 80, Height: 60})
	x, y := find(t, m, "1 ABC-1246")
	press(m, click(x, y))
	if len(o.urls) != 1 || !strings.HasSuffix(o.urls[0], "ABC-1246") {
		t.Fatalf("click on link 1 after growing opened %q", o.urls)
	}
}

func TestFocusPane(t *testing.T) {
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}

	// With herdr's live state, the live pane wins over the recorded one.
	s := fakeSnap()
	s.Herdr = true
	s.Project.PaneID = "w1X:p1"
	s.Threads[1].Pane = &deck.Pane{ID: "w30:p2", AgentStatus: "blocked"}
	m, o := newModel(t, s, 80, 28)
	m, _ = press(m, keys("j")...) // the inbox item about t-0002
	if r, _ := m.selected(); r.kind != rowInbox {
		t.Fatalf("j did not reach the inbox row: %+v", r)
	}
	press(m, enter)
	if len(o.panes) != 1 || o.panes[0] != "w30:p2" {
		t.Fatalf("enter on the inbox row focused %q", o.panes)
	}

	// The subject thread has no pane open: the coordinator's pane.
	s.Threads[1].Pane = nil
	m, o = newModel(t, s, 80, 28)
	m, _ = press(m, keys("j")...)
	m, _ = press(m, enter)
	if len(o.panes) != 1 || o.panes[0] != "w1X:p1" || !strings.Contains(m.Status(), "coordinator") {
		t.Fatalf("enter without the thread's pane: focused %q, status %q", o.panes, m.Status())
	}

	// On the thread's own row, a closed pane is said, not focused.
	m, o = newModel(t, s, 80, 28)
	m, _ = press(m, enter)
	if len(o.panes) != 0 || !strings.Contains(m.Status(), "t-0002 has no open pane") {
		t.Fatalf("enter on a thread without a pane: focused %q, status %q", o.panes, m.Status())
	}

	// A failed focus says why.
	m = New(fakeSnap(), Options{FocusPane: func(string) error { return errors.New("pane gone") }})
	m, _ = press(m, enter)
	if !strings.Contains(m.Status(), "could not focus t-0002's pane w1Z:p1: pane gone") {
		t.Fatalf("failed focus: status %q", m.Status())
	}
}

func TestDrawerLivePane(t *testing.T) {
	s := fakeSnap()
	s.Herdr = true
	s.Threads[1].Pane = &deck.Pane{ID: "w1Z:p1", Agent: "claude", AgentStatus: "blocked"}
	m, _ := newModel(t, s, 80, 28)
	out := screen(m)
	for _, want := range []string{"pane w1Z:p1 · claude blocked", "needs you · ~95%"} {
		if !strings.Contains(out, want) {
			t.Errorf("drawer lacks %q:\n%s", want, out)
		}
	}

	s.Threads[1].Pane = nil
	m, _ = newModel(t, s, 80, 28)
	if out := screen(m); !strings.Contains(out, "pane w1Z:p1 closed") {
		t.Errorf("drawer does not say the pane is closed:\n%s", out)
	}
}

func TestSelectionHighlight(t *testing.T) {
	selLine := func(m Model) string {
		for _, l := range strings.Split(m.View().Content, "\n") {
			if strings.Contains(ansi.Strip(l), "▸") {
				return l
			}
		}
		t.Fatalf("no selected row:\n%s", screen(m))
		return ""
	}
	m, _ := newModel(t, fakeSnap(), 80, 28) // the cursor rests on t-0002, which needs the user
	l := selLine(m)
	if strings.Contains(l, "\x1b[7m") || strings.Contains(l, ";7m") {
		t.Errorf("the selected row is still reverse video: %q", l)
	}
	if !strings.Contains(l, "48;5;237m") {
		t.Errorf("the selected row has no dark grey background: %q", l)
	}
	if !strings.Contains(l, "31m") {
		t.Errorf("the selected row lost its red: %q", l)
	}
	if !strings.HasSuffix(l, "\x1b[m") || ansi.StringWidth(l) != 80 {
		t.Errorf("the highlight does not span the row: width %d, %q", ansi.StringWidth(l), l)
	}

	m, _ = press(m, tea.BackgroundColorMsg{Color: color.White})
	if l := selLine(m); !strings.Contains(l, "48;5;254m") || strings.Contains(l, "48;5;237m") {
		t.Errorf("on a light terminal the highlight is not light grey: %q", l)
	}

	// The link chooser's line uses the same highlight.
	m, _ = press(m, keys("l")...)
	if !strings.Contains(m.View().Content, "▶\x1b[m\x1b[48;5;254m") {
		t.Errorf("the chooser's line is not highlighted:\n%q", m.View().Content)
	}
}

func TestFooterVersionYieldsToHelp(t *testing.T) {
	const help = "1-9 link  l f n g o first  ↵ pane  z drawer  ? help"
	for _, tt := range []struct {
		name, version string
		shown         bool
	}{
		{"fits", testVersion, true},
		{"too long", "v1.2.3-45-gabcdef0", false},
		{"empty", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := newModel(t, calm(), 60, 20)
			m.opt.Version = tt.version
			lines := strings.Split(screen(m), "\n")
			foot := lines[len(lines)-1]
			if !strings.Contains(foot, help) {
				t.Errorf("footer %q lost key help %q", foot, help)
			}
			if got := tt.version != "" && strings.HasSuffix(foot, tt.version); got != tt.shown {
				t.Errorf("footer %q: version shown = %v, want %v", foot, got, tt.shown)
			}
			if w := ansi.StringWidth(foot); w > 60 {
				t.Errorf("footer is %d wide", w)
			}
		})
	}
}

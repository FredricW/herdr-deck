package ui

import (
	"image/color"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
)

var (
	tabKey   = tea.KeyPressMsg{Code: tea.KeyTab}
	enterKey = tea.KeyPressMsg{Code: tea.KeyEnter}
)

// Inbox items are news for the coordinator: they tint their thread's row
// yellow and never take a row of their own under Needs you.
func TestInboxTintsRowsNotNeedsYou(t *testing.T) {
	s := calm()
	s.Inbox = []deck.InboxItem{
		{ID: "a", Thread: "t-0004", Subject: "t-0004", Summary: "PR updated"},
		{ID: "b", Kind: "routine", Subject: "pr-followup", Summary: "ran"},
	}
	m, _ := newModel(t, s, 80, 28)
	for _, r := range m.rows {
		if r.list == listNeedsYou {
			t.Errorf("an inbox item made a Needs you row: %+v", r)
		}
	}
	var line string
	for _, l := range strings.Split(m.View().Content, "\n") {
		if strings.Contains(ansi.Strip(l), "Document select") {
			line = l
		}
	}
	if !strings.Contains(line, inboxStyle.Render("Document select for summary")) {
		t.Errorf("t-0004's row is not tinted yellow: %q", line)
	}
	if !strings.Contains(ansi.Strip(m.header(80)), "✉ 2") {
		t.Errorf("the header does not count both items: %q", ansi.Strip(m.header(80)))
	}
	// A folded list says it holds news.
	s.TaskLists[1].Name = "Later"
	s.Inbox[0].Thread = "t-0001"
	m, _ = newModelWith(t, s, 80, 28, func(o *Options) { o.FoldedLists = []string{"later"} })
	if !strings.Contains(screen(m), "+ Later (1) ○ ✉") {
		t.Errorf("the folded list does not show its update:\n%s", screen(m))
	}
}

func TestTabKeysAndMemory(t *testing.T) {
	diffs := map[string]deck.Diff{"/src/worktrees/t-0002": changes()}
	m, _ := newModelWith(t, deck.Snapshot{}, 80, 28, withDiffs(diffs, nil))
	m, _ = press(m, snapshotMsg(calm()))
	if m.curTab() != tabOverview {
		t.Fatalf("starts on %v", m.curTab())
	}
	m, _ = press(m, keys("]")...)
	if m.curTab() != tabFiles || !strings.Contains(screen(m), "11 files") {
		t.Fatalf("] did not show Files:\n%s", screen(m))
	}
	m, _ = press(m, keys("]]")...)
	if m.curTab() != tabOverview {
		t.Fatalf("] ] did not wrap to Overview: %v", m.curTab())
	}
	m, _ = press(m, keys("[")...)
	if m.curTab() != tabLog {
		t.Fatalf("[ from Overview: %v", m.curTab())
	}
	// The tab stays as the cursor moves; t-0003 has no worktree, so its
	// Files tab is skipped and shown dim.
	m, _ = press(m, keys("j")...)
	if m.curTab() != tabLog || !strings.Contains(screen(m), "launched in pane w20:p1") {
		t.Fatalf("Log did not stay on t-0003:\n%s", screen(m))
	}
	m, _ = press(m, keys("]")...)
	if m.curTab() != tabOverview {
		t.Errorf("] on t-0003 went to %v, want Overview (Files has nothing)", m.curTab())
	}
	m, _ = press(m, keys("[")...)
	// A row without a thread shows Overview, and the chosen tab comes back
	// on the next thread.
	m, _ = press(m, keys("jjjjj ")...) // the Backlog heading, unfolded
	m, _ = press(m, keys("j")...)
	if r, _ := m.selected(); r.task == nil || len(r.threads) > 0 || m.curTab() != tabOverview {
		t.Fatalf("on %q: tab %v", r.title(), m.curTab())
	}
	m, _ = press(m, keys("kkkkk")...)
	if m.curTab() != tabLog {
		t.Errorf("back on a thread: tab %v, want Log", m.curTab())
	}
}

func TestNeedsYouJumpShowsOverview(t *testing.T) {
	s := calm()
	m, _ := newModel(t, s, 80, 28)
	m, _ = press(m, keys("]")...) // Log (no Diff: Files is skipped)
	if m.curTab() != tabLog {
		t.Fatalf("tab %v", m.curTab())
	}
	// t-0004 starts to need the user while the cursor was never moved by
	// hand: the cursor jumps there, on Overview.
	s.Threads[3].Status = deck.StatusNeedsYou
	m, _ = press(m, snapshotMsg(s))
	if r, _ := m.selected(); r.title() != "Document select for summary" || m.curTab() != tabOverview {
		t.Errorf("jump: on %q, tab %v", r.title(), m.curTab())
	}
}

func TestDrawerFocusAndCursor(t *testing.T) {
	m, o := newModel(t, calm(), 80, 28)
	m, _ = press(m, tabKey)
	if !m.dfocus {
		t.Fatal("tab did not focus the drawer")
	}
	// j moves the drawer cursor, not the list; enter opens the chip.
	m, _ = press(m, keys("j")...)
	if r, _ := m.selected(); r.title() != "Users page" {
		t.Fatalf("j moved the list to %q", r.title())
	}
	m, _ = press(m, enterKey)
	if len(o.urls) != 1 || !strings.HasSuffix(o.urls[0], "ABC-1256") {
		t.Fatalf("enter on the second chip opened %q", o.urls)
	}
	// tab switches tabs inside the drawer; esc gives the list back.
	m, _ = press(m, tabKey)
	if m.curTab() != tabLog {
		t.Errorf("tab in the drawer: %v", m.curTab())
	}
	m, _ = press(m, esc)
	if m.dfocus {
		t.Error("esc did not return the focus to the list")
	}
	m, _ = press(m, keys("j")...)
	if r, _ := m.selected(); r.title() != "Templates page" {
		t.Errorf("j after esc: %q", r.title())
	}
}

func TestLogEventsAct(t *testing.T) {
	m, o := newModel(t, calm(), 80, 28)
	m, _ = press(m, keys("jj]")...) // t-0004's Log
	if m.curTab() != tabLog {
		t.Fatalf("tab %v", m.curTab())
	}
	// A click on a PR event opens the PR; on another event, the pane.
	x, y := find(t, m, "updated · open · 2 comments")
	m, _ = press(m, click(x, y))
	if !slices.Equal(o.urls, []string{"https://github.com/acme/webshop/pull/2320"}) {
		t.Fatalf("click on the PR event opened %q", o.urls)
	}
	if !m.dfocus {
		t.Error("a click in the drawer did not focus it")
	}
	x, y = find(t, m, "waiting on you")
	press(m, click(x, y))
	if !slices.Equal(o.panes, []string{"w21:p1"}) {
		t.Errorf("click on a waiting event focused %q", o.panes)
	}
	// On t-0002's Log, enter on the newest report shows it.
	m, _ = newModel(t, calm(), 80, 28)
	m, _ = press(m, keys("]")...)
	m, _ = press(m, tabKey, keys("j")[0], enterKey)
	if m.mode != modeReport {
		t.Errorf("enter on a report event: mode %v", m.mode)
	}
}

func TestClickTabsAndUpdates(t *testing.T) {
	m, _ := newModel(t, fakeSnap(), 80, 28)
	x, y := find(t, m, "Log 7")
	m, _ = press(m, click(x+1, y))
	if m.curTab() != tabLog {
		t.Fatalf("click on Log: %v", m.curTab())
	}
	// Files has nothing behind it here (no Diff): a click does nothing.
	x, y = find(t, m, "Files")
	m, _ = press(m, click(x+1, y))
	if m.curTab() != tabLog {
		t.Errorf("click on a dim Files: %v", m.curTab())
	}
	x, y = find(t, m, "Overview")
	m, _ = press(m, click(x+1, y))
	x, y = find(t, m, "1 update · see Log")
	m, _ = press(m, click(x+2, y))
	if m.curTab() != tabLog {
		t.Errorf("click on the updates line: %v", m.curTab())
	}
	if !strings.Contains(screen(m), "waiting on you · pane w1Z:p1") || !strings.Contains(screen(m), "14:38 ✉") {
		t.Errorf("the unhandled item is not marked in Log:\n%s", screen(m))
	}
}

// The drawer's focus ends when the drawer is hidden or a full view takes
// its place, so j/k move the list again and esc closes the view at once.
func TestDrawerFocusEndsWithTheTabs(t *testing.T) {
	m, _ := newModel(t, calm(), 80, 28)
	m, _ = press(m, tabKey)
	m, _ = press(m, keys("zz")...) // hidden
	if m.dfocus {
		t.Fatal("hiding the drawer kept its focus")
	}
	m, _ = press(m, keys("j")...)
	if r, _ := m.selected(); r.title() != "Templates page" {
		t.Errorf("j with the drawer hidden: %q", r.title())
	}
	m, _ = press(m, tabKey)
	if m.dfocus {
		t.Error("tab focused a hidden drawer")
	}

	// From a focused Log, enter on a report event opens the report; one
	// esc closes it.
	m, _ = newModel(t, calm(), 80, 28)
	m, _ = press(m, keys("]")...)
	m, _ = press(m, tabKey, keys("j")[0], enterKey)
	if m.mode != modeReport || m.dfocus {
		t.Fatalf("report: mode %v, focus %v", m.mode, m.dfocus)
	}
	m, _ = press(m, esc)
	if m.mode != modeRow {
		t.Errorf("one esc left mode %v", m.mode)
	}
	m, _ = press(m, tabKey, keys("?")[0])
	if m.dfocus {
		t.Error("help kept the drawer's focus")
	}
}

// The active tab is bold near-black on the named blue, black on a light
// terminal; inactive tabs sit on the selection's grey.
func TestTabColours(t *testing.T) {
	m, _ := newModel(t, calm(), 80, 28)
	bar := func(m Model) string {
		for _, l := range strings.Split(m.View().Content, "\n") {
			if strings.Contains(ansi.Strip(l), " Overview ") {
				return l
			}
		}
		t.Fatal("no tab bar")
		return ""
	}
	l := bar(m)
	if !strings.Contains(l, "38;5;234") || !strings.Contains(l, "44") || !strings.Contains(l, "48;5;237") {
		t.Errorf("dark terminal tab bar %q: want 234 text on blue (44) and 237 grey", l)
	}
	if strings.Contains(l, "97m") {
		t.Errorf("the active tab is still bright white: %q", l)
	}
	m, _ = press(m, tea.BackgroundColorMsg{Color: color.White})
	if l := bar(m); !strings.Contains(l, "38;5;16") || !strings.Contains(l, "48;5;254") {
		t.Errorf("light terminal tab bar %q: want 16 text and 254 grey", l)
	}
}

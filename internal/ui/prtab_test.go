package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/fake"
)

const prURL2320 = "https://github.com/acme/webshop/pull/2320"

// prTabKeys select t-0004 and show its PR tab: Log is the last tab, and
// PR the one before it.
const prTabKeys = "jj[["

func TestPRTabGolden(t *testing.T) {
	noDetail := livePR(func(pr *deck.PullRequest) { pr.Detail = nil })
	failed := livePR(func(pr *deck.PullRequest) {
		pr.Detail, pr.DetailNote = nil, "could not read it: rate limited until 15:00"
	})
	empty := livePR(func(pr *deck.PullRequest) {
		pr.Detail.Body, pr.Detail.Comments, pr.Detail.Labels, pr.Detail.Reviewers = "", nil, nil, nil
		pr.Threads, pr.ReviewRequests = nil, nil
	})
	merged := livePR(func(pr *deck.PullRequest) {
		pr.State, pr.MergeState, pr.Checks, pr.FailingChecks = "MERGED", "", nil, nil
	})
	cases := []struct {
		name string
		snap deck.Snapshot
		keys []tea.Msg
	}{
		{name: "pr-tab", snap: live(), keys: keys(prTabKeys)},
		{name: "pr-tab-full", snap: live(), keys: keys(prTabKeys + "z")},
		{name: "pr-tab-comments", snap: live(), keys: append(keys(prTabKeys+"z"), pgdn, pgdn)},
		{name: "pr-tab-focus", snap: live(), keys: append(keys(prTabKeys+"z"), tabKey, down, down, down)},
		{name: "pr-tab-unfolded", snap: live(), keys: append(keys(prTabKeys+"z"), tabKey, down, down, keySpace, pgdn)},
		{name: "pr-tab-reading", snap: noDetail, keys: keys(prTabKeys)},
		{name: "pr-tab-failed", snap: failed, keys: keys(prTabKeys)},
		{name: "pr-tab-empty", snap: empty, keys: keys(prTabKeys + "z")},
		{name: "pr-tab-merged", snap: merged, keys: keys(prTabKeys)},
		{name: "pr-tab-ticker", snap: calm(), keys: keys(prTabKeys)},
	}
	for _, c := range cases {
		for _, w := range []int{80, 60} {
			name := c.name + "-" + map[int]string{80: "80", 60: "60"}[w]
			t.Run(name, func(t *testing.T) {
				m, _ := newModelWith(t, deck.Snapshot{}, w, 28, func(o *Options) {
					o.CheckLog = fake.CheckLog
					o.DetailPR = func(string) {}
				})
				m, _ = press(m, snapshotMsg(c.snap))
				m, _ = press(m, c.keys...)
				if m.curTab() != tabPR {
					t.Fatalf("tab %v, want PR", m.curTab())
				}
				golden(t, name, m)
			})
		}
	}
}

var (
	pgdn     = tea.KeyPressMsg{Code: tea.KeyPgDown}
	down     = tea.KeyPressMsg{Code: 'j', Text: "j"}
	keySpace = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
)

// The PR tab shows only for a row whose thread has a PR.
func TestPRTabOnlyWithPR(t *testing.T) {
	m, _ := newModel(t, live(), 80, 28)
	if sc := screen(m); strings.Contains(sc, " PR ") && !strings.Contains(sc, "Document select") {
		t.Errorf("a row without a PR shows the PR tab:\n%s", sc)
	}
	m, _ = press(m, keys("jj")...)
	if !strings.Contains(screen(m), "  PR  ") {
		t.Errorf("t-0004's drawer has no PR tab:\n%s", screen(m))
	}
	// ] from Commits lands on PR, and from PR on Log; elsewhere PR is
	// skipped.
	m, _ = press(m, keys("[")...)
	if m.curTab() != tabLog {
		t.Fatalf("[ from Overview: %v", m.curTab())
	}
	m, _ = press(m, keys("[")...)
	if m.curTab() != tabPR {
		t.Fatalf("[ from Log: %v", m.curTab())
	}
	m, _ = press(m, keys("k")...) // t-0003 has no PR: Overview shows
	if m.curTab() != tabOverview {
		t.Errorf("a row without a PR: tab %v", m.curTab())
	}
	m, _ = press(m, keys("j")...) // back on t-0004: the PR tab again
	if m.curTab() != tabPR {
		t.Errorf("back on the PR's row: tab %v", m.curTab())
	}
}

// The PR tab asks the reader for the detail while it shows, and stops
// asking when it does not.
func TestDetailPR(t *testing.T) {
	var asked []string
	m, _ := newModelWith(t, deck.Snapshot{}, 80, 28, func(o *Options) {
		o.DetailPR = func(url string) { asked = append(asked, url) }
	})
	m, _ = press(m, snapshotMsg(live()))
	m, _ = press(m, keys("jj")...)
	if len(asked) != 0 {
		t.Fatalf("asked %q before the PR tab showed", asked)
	}
	m, _ = press(m, keys(prTabKeys[2:])...)
	m, _ = press(m, keys("z")...) // full height: still the tab
	m, _ = press(m, keys("]")...) // Log
	m, _ = press(m, keys("[")...) // PR again
	m, _ = press(m, keys("z")...) // hidden
	_ = m
	want := []string{prURL2320, "", prURL2320, ""}
	if strings.Join(asked, " ") != strings.Join(want, " ") {
		t.Errorf("DetailPR got %q, want %q", asked, want)
	}
}

func TestPRTabComments(t *testing.T) {
	m, o := newModelWith(t, live(), 80, 28, func(opt *Options) { opt.CheckLog = fake.CheckLog })
	m, _ = press(m, keys(prTabKeys+"z")...)
	// A click on a review thread opens it on GitHub.
	m = scrollTo(t, m, "src/pages/users/table.tsx:12")
	x, y := find(t, m, "src/pages/users/table.tsx:12")
	m, _ = press(m, click(x, y))
	if len(o.urls) != 1 || o.urls[0] != prURL2320+"#discussion_r102" {
		t.Fatalf("a click on a review thread opened %q", o.urls)
	}
	// The bot's comment is one dim line; a click on ▸ unfolds it.
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyPgUp})
	m = scrollTo(t, m, "deploy-preview")
	sc := screen(m)
	if strings.Contains(sc, "   Preview deployed") || !strings.Contains(sc, "▸ deploy-preview · bot · 2d ago: Preview deployed") {
		t.Fatalf("the bot's comment is not folded:\n%s", sc)
	}
	x, y = find(t, m, "▸ deploy-preview")
	m, _ = press(m, click(x, y))
	if sc := screen(m); !strings.Contains(sc, "▾ deploy-preview") || !strings.Contains(sc, "   Preview deployed") {
		t.Fatalf("a click on ▸ did not unfold it:\n%s", sc)
	}
	if len(o.urls) != 1 {
		t.Errorf("a click on ▸ opened %q", o.urls[1:])
	}
	// ↵ on the unfolded comment opens it; ← folds it again.
	m, _ = press(m, enterKey)
	if len(o.urls) != 2 || o.urls[1] != prURL2320+"#issuecomment-201" {
		t.Errorf("↵ on the bot's comment opened %q", o.urls)
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyLeft})
	if sc := screen(m); !strings.Contains(sc, "▸ deploy-preview") {
		t.Errorf("← did not fold it:\n%s", sc)
	}
}

func TestPRTabCheckChip(t *testing.T) {
	m, _ := newModelWith(t, live(), 80, 28, func(opt *Options) { opt.CheckLog = fake.CheckLog })
	m, _ = press(m, keys(prTabKeys)...)
	x, y := find(t, m, "[lint]")
	m, _ = press(m, click(x+1, y))
	if m.mode != modeCheck || m.logCheck.Name != "lint" {
		t.Errorf("a click on the check chip: mode %v, check %q", m.mode, m.logCheck.Name)
	}
	m, _ = press(m, esc)
	if m.curTab() != tabPR {
		t.Errorf("esc from the log: tab %v, want PR", m.curTab())
	}
}

// The Overview's PR section is a summary that points to the PR tab.
func TestOverviewPointsToPRTab(t *testing.T) {
	m, _ := newModel(t, live(), 80, 28)
	m, _ = press(m, keys("jj")...)
	sc := screen(m)
	if strings.Contains(sc, "── Review") {
		t.Errorf("Overview still has the Review section:\n%s", sc)
	}
	x, y := find(t, m, "→ PR tab: 3 unresolved review threads")
	m, _ = press(m, click(x+2, y))
	if m.curTab() != tabPR {
		t.Errorf("a click on the pointer: tab %v", m.curTab())
	}
}

// A click on the list's PR number selects the row and shows its PR tab; a
// second click opens the PR on GitHub. g opens it from anywhere.
func TestPRColumnClick(t *testing.T) {
	for _, w := range []int{80, 60} {
		m, o := newModel(t, live(), w, 28)
		x, y := find(t, m, "#2320")
		// Just left of the number is the STATUS column (80) or the WORK
		// column (60): a plain row click, no PR tab.
		m, _ = press(m, click(x-3, y))
		if selectedTitle(m) != "Document select for summary" || m.curTab() != tabOverview {
			t.Fatalf("%d: a click left of the PR: %q, tab %v", w, selectedTitle(m), m.curTab())
		}
		m, _ = press(m, keys("k")...)
		m, _ = press(m, click(x+1, y))
		if selectedTitle(m) != "Document select for summary" || m.curTab() != tabPR || len(o.urls) != 0 {
			t.Fatalf("%d: a click on #2320: %q, tab %v, opened %q", w, selectedTitle(m), m.curTab(), o.urls)
		}
		m, _ = press(m, click(x+1, y))
		if len(o.urls) != 1 || o.urls[0] != prURL2320 {
			t.Fatalf("%d: a second click opened %q", w, o.urls)
		}
		m, _ = press(m, keys("g")...)
		if len(o.urls) != 2 {
			t.Errorf("%d: g opened %q", w, o.urls)
		}
		// With the drawer hidden, a click shows it again on the PR tab.
		m, _ = press(m, keys("kzz")...)
		x, y = find(t, m, "#2320")
		m, _ = press(m, click(x, y))
		if m.size != sizeNormal || m.curTab() != tabPR || len(o.urls) != 2 {
			t.Errorf("%d: a click with the drawer hidden: size %v, tab %v, opened %q", w, m.size, m.curTab(), o.urls)
		}
	}
}

// Hit-testing follows the rows on screen: folded lists, gaps and a
// scrolled list.
func TestPRColumnClickScrolled(t *testing.T) {
	s := live()
	for i := range 30 {
		s.TaskLists[0].Tasks = append(s.TaskLists[0].Tasks, deck.Task{Title: "Task " + string(rune('A'+i%26)) + string(rune('a'+i/26))})
	}
	for _, w := range []int{80, 60} {
		m, _ := newModel(t, s, w, 20)
		m, _ = press(m, keys(strings.Repeat("j", 40))...)
		if m.listOff == 0 {
			t.Fatalf("%d: the list did not scroll", w)
		}
		// Scroll back so t-0004 is on screen, but not at the top.
		for range 40 {
			m, _ = press(m, keys("k")...)
		}
		m, _ = press(m, tea.MouseWheelMsg{X: 1, Y: 4, Button: tea.MouseWheelDown})
		x, y := find(t, m, "#2320")
		m, _ = press(m, click(x, y))
		if selectedTitle(m) != "Document select for summary" || m.curTab() != tabPR {
			t.Errorf("%d: listOff %d: a click on #2320 selected %q, tab %v", w, m.listOff, selectedTitle(m), m.curTab())
		}
		// The PR column of a row without a PR is a plain row click.
		_, y2 := find(t, m, "Templates page")
		m, _ = press(m, click(x, y2))
		if selectedTitle(m) != "Templates page" || m.curTab() == tabPR {
			t.Errorf("%d: a click on an empty PR cell: %q, tab %v", w, selectedTitle(m), m.curTab())
		}
	}
}

// scrollTo pages the drawer down until text shows.
func scrollTo(t *testing.T, m Model, text string) Model {
	t.Helper()
	for range 20 {
		if strings.Contains(screen(m), text) {
			return m
		}
		m, _ = press(m, pgdn)
	}
	t.Fatalf("%q never shows:\n%s", text, screen(m))
	return m
}

// The PR tab, long like a report, has the full views' scrollbar beside its
// content; the card and tab bar keep the full width.
func TestPRTabScrollbar(t *testing.T) {
	m, _ := newModel(t, live(), 80, 28)
	m, _ = press(m, keys(prTabKeys)...)
	l := m.layout()
	if !l.barred || l.drawer.width != 78 {
		t.Fatalf("barred %v, content width %d; want a scrollbar beside 78 columns", l.barred, l.drawer.width)
	}
	if _, y := find(t, m, "◇ review"); !strings.HasSuffix(strings.Split(screen(m), "\n")[y], "◇ review") {
		t.Error("the card lost its full width")
	}
	// A press at the track's end scrolls to the comments.
	m, _ = press(m, click(79, l.drawerTop+l.drawerH-1))
	if m.drawerOff == 0 || !strings.Contains(screen(m), "kim ✓ approved") {
		t.Errorf("a press on the track: offset %d\n%s", m.drawerOff, screen(m))
	}
	// Overview has no scrollbar.
	m, _ = press(m, keys("]]")...)
	if l := m.layout(); l.barred {
		t.Error("Overview got a scrollbar")
	}
}

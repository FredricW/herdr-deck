package ui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/FredricW/herdr-deck/internal/config"
	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/fake"
)

// withProjects is calm in the sample projects root: a Billing export
// thread waits on the user, Docs site has only an inbox update, Search
// spike is paused, Mobile onboarding archived.
func withProjects() deck.Snapshot {
	s := calm()
	s.Herdr = true
	s.Projects = fake.Projects(s, now)
	return s
}

var tab = tea.KeyPressMsg{Code: tea.KeyTab}

func TestPickerGolden(t *testing.T) {
	cases := []struct {
		name string
		keys []tea.Msg
	}{
		{name: "attention"},
		{name: "picker", keys: keys("p")},
		{name: "picker-archived", keys: append(keys("p"), tab)},
		{name: "picker-filter", keys: keys("pdoc")},
	}
	for _, c := range cases {
		for _, w := range []int{80, 60} {
			name := c.name + "-" + map[int]string{80: "80", 60: "60"}[w]
			t.Run(name, func(t *testing.T) {
				m, _ := newModel(t, deck.Snapshot{}, w, 28)
				m, _ = press(m, snapshotMsg(withProjects()))
				m, _ = press(m, c.keys...)
				golden(t, name, m)
			})
		}
	}
}

// pickModel is a deck on withProjects with the picker open; starts records
// the coordinators it asked to start.
func pickModel(t *testing.T, snap deck.Snapshot) (Model, *opened, *[]string) {
	t.Helper()
	var starts []string
	m, o := newModelWith(t, deck.Snapshot{}, 80, 28, func(opt *Options) {
		opt.OpenProject = func(slug string) error { starts = append(starts, slug); return nil }
	})
	m, _ = press(m, snapshotMsg(snap))
	m, _ = press(m, keys("p")...)
	if !m.pick.open {
		t.Fatal("p did not open the picker")
	}
	return m, o, &starts
}

func pickedKey(m Model) string {
	rows := m.pickRows()
	if i := pickCursor(rows, m.pick.sel); i >= 0 {
		return rows[i].key
	}
	return ""
}

func TestPickerOrder(t *testing.T) {
	m, _, _ := pickModel(t, withProjects())
	var got []string
	for _, r := range m.pickRows() {
		if r.kind == pickProject {
			got = append(got, r.project.Slug)
		}
	}
	// Those that need the user first, then by name; an inbox update does
	// not make Docs site need the user.
	want := []string{"billing-export", "admin-rebuild", "docs-site", "search-spike"}
	if !slices.Equal(got, want) {
		t.Errorf("order %v, want %v", got, want)
	}
	m, _ = press(m, tab)
	if rows := m.pickRows(); rows[len(rows)-1].project.Slug != "mobile-onboarding" {
		t.Errorf("archived not last after tab:\n%s", screen(m))
	}
}

func TestPickerGoesToCoordinatorOrPane(t *testing.T) {
	down := tea.KeyPressMsg{Code: tea.KeyDown}
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}

	// Billing export runs a coordinator: focus it.
	m, o, starts := pickModel(t, withProjects())
	m, _ = press(m, enter)
	if !slices.Equal(o.panes, []string{"w4J:p1"}) || len(*starts) > 0 || m.pick.open {
		t.Errorf("panes %v, starts %v, open %v", o.panes, *starts, m.pick.open)
	}
	if m.Status() != "focused Billing export's coordinator" {
		t.Errorf("status %q", m.Status())
	}

	// A thread waiting on the user: its pane.
	m, o, _ = pickModel(t, withProjects())
	m, _ = press(m, down, enter)
	if !slices.Equal(o.panes, []string{"w4K:p3"}) {
		t.Errorf("thread: panes %v", o.panes)
	}

	// Inbox items are updates, not rows: next comes Admin rebuild, then
	// Docs site, which runs no coordinator: start one.
	m, o, starts = pickModel(t, withProjects())
	m, _ = press(m, down, down, down, enter)
	if !slices.Equal(*starts, []string{"docs-site"}) || len(o.panes) > 0 || m.pick.open {
		t.Errorf("starts %v, panes %v, open %v", *starts, o.panes, m.pick.open)
	}
	if m.Status() != "started Docs site's coordinator" {
		t.Errorf("status %q", m.Status())
	}

	// An archived project starts nothing.
	m, o, starts = pickModel(t, withProjects())
	m, _ = press(m, tab, tea.KeyPressMsg{Code: tea.KeyEnd})
	for range 9 {
		m, _ = press(m, down)
	}
	if pickedKey(m) != "project:mobile-onboarding" {
		t.Fatalf("selected %q", pickedKey(m))
	}
	m, _ = press(m, enter)
	if len(*starts)+len(o.panes) > 0 || !strings.Contains(m.Status(), "archived") {
		t.Errorf("archived: starts %v, panes %v, status %q", *starts, o.panes, m.Status())
	}
}

func TestPickerFilterAndEsc(t *testing.T) {
	m, _, _ := pickModel(t, withProjects())
	// j and k type: moving takes the arrows.
	m, _ = press(m, keys("sjk")...)
	if m.pick.filter != "sjk" || !strings.Contains(screen(m), "No project matches “sjk”.") {
		t.Errorf("filter %q:\n%s", m.pick.filter, screen(m))
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.pick.filter != "s" {
		t.Errorf("filter after backspace %q", m.pick.filter)
	}
	m, _ = press(m, keys("earch")...)
	if pickedKey(m) != "project:search-spike" {
		t.Errorf("selected %q after filtering", pickedKey(m))
	}
	m, _ = press(m, esc)
	if !m.pick.open || m.pick.filter != "" {
		t.Errorf("esc did not clear the filter first: open %v, filter %q", m.pick.open, m.pick.filter)
	}
	m, _ = press(m, esc)
	if m.pick.open {
		t.Error("esc did not close the picker")
	}
	if !strings.Contains(screen(m), "WORK") {
		t.Errorf("the deck is not back:\n%s", screen(m))
	}
}

func TestPickerMouse(t *testing.T) {
	m, o, _ := pickModel(t, withProjects())
	_, y := find(t, m, "CSV export job")
	m, _ = press(m, click(10, y))
	if !slices.Equal(o.panes, []string{"w4K:p3"}) || m.pick.open {
		t.Errorf("click on a thread: panes %v, open %v", o.panes, m.pick.open)
	}

	m, _, _ = pickModel(t, withProjects())
	m, _ = press(m, tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelDown})
	if pickedKey(m) != "thread:billing-export/t-0003" {
		t.Errorf("wheel: selected %q", pickedKey(m))
	}

	// A click on the attention line opens the picker.
	m, _ = newModel(t, deck.Snapshot{}, 80, 28)
	m, _ = press(m, snapshotMsg(withProjects()))
	x, y := find(t, m, "1 other project needs you")
	m, _ = press(m, click(x, y))
	if !m.pick.open {
		t.Error("a click on the attention line did not open the picker")
	}
}

func TestAttentionLine(t *testing.T) {
	count := func(snap deck.Snapshot) (int, bool) {
		m, _ := newModel(t, deck.Snapshot{}, 80, 28)
		m, _ = press(m, snapshotMsg(snap))
		return m.otherNeeds(), strings.Contains(screen(m), "other project")
	}
	waiting := deck.Thread{ID: "t-0009", Status: deck.StatusNeedsYou}
	// Billing export's waiting thread counts; Docs site's inbox update
	// does not.
	s := withProjects()
	if n, shown := count(s); n != 1 || !shown {
		t.Errorf("count %d, shown %v; want 1", n, shown)
	}
	// A second project with a waiting thread counts too.
	s.Projects[2].Threads = append(s.Projects[2].Threads, waiting)
	if n, _ := count(s); n != 2 {
		t.Errorf("count %d, want 2", n)
	}
	// This project's own needs never count, nor do archived ones'.
	s = withProjects()
	s.Projects[0].Threads = append(s.Projects[0].Threads, waiting)
	s.Projects[4].Threads = append(s.Projects[4].Threads, waiting)
	s.Projects[1].Threads = nil
	if n, shown := count(s); n != 0 || shown {
		t.Errorf("count %d, shown %v; want none", n, shown)
	}
	// Other projects' needs never join this project's Needs you group.
	m, _ := newModel(t, deck.Snapshot{}, 80, 28)
	m, _ = press(m, snapshotMsg(withProjects()))
	if strings.Contains(screen(m), listNeedsYou) {
		t.Errorf("a Needs you group:\n%s", screen(m))
	}
	// A full-height drawer leaves no list, and no line.
	m, _ = press(m, keys("z")...)
	if strings.Contains(screen(m), "other projects") {
		t.Errorf("attention line with a full drawer:\n%s", screen(m))
	}
}

func TestPickerKeyFromEveryView(t *testing.T) {
	for _, k := range []string{"?", "!", "w", "l", "r"} {
		m, _ := newModel(t, deck.Snapshot{}, 80, 28)
		m, _ = press(m, snapshotMsg(withProjects()))
		m, _ = press(m, keys(k+"p")...)
		if !m.pick.open {
			t.Errorf("%s then p: no picker", k)
		}
	}
	// The settings page takes its keys first.
	e := newSettingsEnv(t, "", nil)
	m := settingsModel(t, e, config.Flags{}, 80, 28)
	m, _ = press(m, snapshotMsg(withProjects()))
	m, _ = press(m, keys("p")...)
	if m.pick.open || m.mode != modeSettings {
		t.Error("p opened the picker over the settings page")
	}
}

func TestPickerWithoutProjects(t *testing.T) {
	m, _ := newModel(t, deck.Snapshot{}, 60, 28)
	m, _ = press(m, snapshotMsg(calm()))
	m, _ = press(m, keys("p")...)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(screen(m), "Projects are not read yet.") || !m.pick.open {
		t.Errorf("open %v:\n%s", m.pick.open, screen(m))
	}
}

func TestPickerClickBelowRowsDoesNothing(t *testing.T) {
	s := withProjects()
	// More projects than fit, so rows lie past the picker's last line.
	for i := range 30 {
		s.Projects = append(s.Projects, deck.ProjectInfo{Project: deck.Project{Slug: fmt.Sprintf("p-%02d", i)}, Status: "active"})
	}
	m, o, starts := pickModel(t, s)
	for _, y := range []int{m.height - 2, m.height - 1} {
		m, _ = press(m, click(10, y))
	}
	if len(o.panes)+len(*starts) > 0 || !m.pick.open {
		t.Errorf("a click on the footer went somewhere: panes %v, starts %v, open %v", o.panes, *starts, m.pick.open)
	}
}

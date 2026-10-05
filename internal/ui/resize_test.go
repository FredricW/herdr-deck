package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/config"
)

func motion(x, y int) tea.MouseMotionMsg {
	return tea.MouseMotionMsg{X: x, Y: y, Button: tea.MouseLeft}
}

func release(x, y int) tea.MouseReleaseMsg {
	return tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft}
}

// keptModel is a model whose KeepDrawer records what it keeps.
func keptModel(t *testing.T, w, h int, set func(*Options)) (Model, *[]float64) {
	t.Helper()
	var kept []float64
	m, _ := newModelWith(t, calm(), w, h, func(o *Options) {
		o.KeepDrawer = func(v float64) error { kept = append(kept, v); return nil }
		set(o)
	})
	return m, &kept
}

func screenLine(m Model, y int) string {
	return strings.Split(ansi.Strip(m.View().Content), "\n")[y]
}

// Dragging the rule above the drawer resizes the list and the drawer live,
// highlights the rule while it moves, and keeps the height on release.
func TestDragDrawerRule(t *testing.T) {
	m, kept := keptModel(t, 80, 28, func(*Options) {})
	l := m.layout()
	if l.sepY != 14 || l.listH != 11 || l.drawerH != 6 {
		t.Fatalf("start: rule %d, list %d, drawer %d", l.sepY, l.listH, l.drawerH)
	}
	cursor, tab := m.cursor, m.curTab()

	m, _ = press(m, click(30, l.sepY))
	if !m.dragging || m.cursor != cursor || m.curTab() != tab || m.dfocus || m.mode != modeRow {
		t.Fatalf("press on the rule: dragging %v, cursor %d→%d, focus %v, mode %v", m.dragging, cursor, m.cursor, m.dfocus, m.mode)
	}
	m, _ = press(m, motion(30, 10))
	if l := m.layout(); l.sepY != 10 || l.listH != 7 || l.drawerH != 10 {
		t.Fatalf("dragged up: rule %d, list %d, drawer %d", l.sepY, l.listH, l.drawerH)
	}
	if got := screenLine(m, 10); !strings.HasPrefix(got, "━━━") {
		t.Errorf("the rule is not highlighted while dragged: %q", got)
	}
	if len(*kept) != 0 {
		t.Errorf("kept %v before the release", *kept)
	}
	m, _ = press(m, release(30, 10))
	if m.dragging || len(*kept) != 1 || (*kept)[0] != 16.0/24 {
		t.Fatalf("release: dragging %v, kept %v", m.dragging, *kept)
	}
	if got := screenLine(m, 10); !strings.HasPrefix(got, "───") {
		t.Errorf("the rule stays highlighted after the release: %q", got)
	}

	// Motion without a drag does nothing.
	m, _ = press(m, motion(30, 20))
	if l := m.layout(); l.sepY != 10 {
		t.Errorf("motion without a press moved the rule to %d", l.sepY)
	}
	// A press and release without moving keeps nothing.
	m, _ = press(m, click(5, 10), release(5, 10))
	if len(*kept) != 1 {
		t.Errorf("a click on the rule kept %v", *kept)
	}
}

// The list keeps three rows and the drawer its card, tab bar and two lines
// of content, however far the rule is dragged.
func TestDragDrawerLimits(t *testing.T) {
	m, _ := keptModel(t, 80, 28, func(*Options) {})
	m, _ = press(m, click(0, 14), motion(0, 0))
	if l := m.layout(); l.listH != minListRows || l.sepY != 6 {
		t.Errorf("dragged to the top: list %d, rule %d", l.listH, l.sepY)
	}
	m, _ = press(m, motion(0, 40))
	l := m.layout()
	if l.headN != 5 || l.drawerH != 2 || l.tabY < 0 {
		t.Errorf("dragged to the bottom: head %d, drawer %d, tab bar %d", l.headN, l.drawerH, l.tabY)
	}
	m, _ = press(m, release(0, 40))
	if !strings.Contains(screen(m), "Overview") {
		t.Errorf("no tab bar at the smallest drawer:\n%s", screen(m))
	}
}

// The height is a share of the pane, so it follows a resize, and z cycles
// back to it.
func TestDraggedHeightFollowsResizeAndZ(t *testing.T) {
	m, _ := keptModel(t, 80, 28, func(*Options) {})
	m, _ = press(m, click(0, 14), motion(0, 10), release(0, 10)) // 16 of 24 lines
	m, _ = press(m, tea.WindowSizeMsg{Width: 80, Height: 52})
	if l := m.layout(); l.listH != 47-32 {
		t.Errorf("52 rows: list %d, want %d", l.listH, 47-32)
	}
	m, _ = press(m, keys("zzz")...)
	if l := m.layout(); m.size != sizeNormal || l.listH != 15 {
		t.Errorf("z back to normal: size %v, list %d", m.size, l.listH)
	}
	// A full drawer has no rule to drag.
	m, _ = press(m, keys("z")...)
	m, _ = press(m, click(0, 2))
	if m.dragging {
		t.Error("a press in a full drawer started a drag")
	}
}

// A pane too small for both limits splits in half as before, and its rule
// does not drag.
func TestSmallPaneDoesNotDrag(t *testing.T) {
	m, kept := keptModel(t, 80, 12, func(*Options) {})
	before := m.layout()
	m, _ = press(m, click(0, before.sepY), motion(0, before.sepY-2), release(0, before.sepY-2))
	m, _ = press(m, keys("+")...)
	if l := m.layout(); l.listH != before.listH || l.drawerH != before.drawerH || len(*kept) != 0 {
		t.Errorf("small pane changed: %+v → %+v, kept %v", before, l, *kept)
	}
}

// + and - grow and shrink the drawer a line at a time, within the limits;
// + shows a hidden drawer and - shrinks a full one to its normal size.
func TestResizeDrawerKeys(t *testing.T) {
	m, kept := keptModel(t, 80, 28, func(*Options) {})
	m, _ = press(m, keys("+")...)
	if l := m.layout(); l.listH != 10 || l.drawerH != 7 {
		t.Errorf("+: list %d, drawer %d", l.listH, l.drawerH)
	}
	m, _ = press(m, keys("--")...)
	if l := m.layout(); l.listH != 12 || l.drawerH != 5 {
		t.Errorf("--: list %d, drawer %d", l.listH, l.drawerH)
	}
	if len(*kept) != 3 || (*kept)[2] != 11.0/24 {
		t.Errorf("kept %v", *kept)
	}
	for range 10 {
		m, _ = press(m, keys("-")...)
	}
	if l := m.layout(); l.drawerH != 2 || len(*kept) != 6 {
		t.Errorf("at the limit: drawer %d, kept %d times", l.drawerH, len(*kept))
	}
	m, _ = press(m, keys("z")...) // full
	m, _ = press(m, keys("-")...)
	if m.size != sizeNormal || m.layout().drawerH != 2 {
		t.Errorf("- on a full drawer: size %v", m.size)
	}
	m, _ = press(m, keys("zz")...) // hidden
	m, _ = press(m, keys("=")...)
	if m.size != sizeNormal {
		t.Errorf("= on a hidden drawer: size %v", m.size)
	}
}

// The setting is the default; a kept height wins over it.
func TestDrawerHeightOptions(t *testing.T) {
	m, _ := keptModel(t, 80, 28, func(o *Options) { o.DrawerHeight = 0.4 })
	if l := m.layout(); l.listH != 23-10 {
		t.Errorf("0.4: list %d", l.listH)
	}
	m, _ = keptModel(t, 80, 28, func(o *Options) { o.DrawerHeight, o.DrawerKept = 0.4, 0.75 })
	if l := m.layout(); l.listH != 23-18 {
		t.Errorf("kept 0.75: list %d", l.listH)
	}
}

// A dragged height survives a restart through the state file.
func TestDraggedHeightPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", config.DrawerFile)
	keep := func(o *Options) {
		o.DrawerKept, _ = config.ReadDrawerHeight(path)
		o.KeepDrawer = func(v float64) error { return config.WriteDrawerHeight(path, v) }
	}
	m, _ := newModelWith(t, calm(), 80, 28, keep)
	m, _ = press(m, click(0, 14), motion(0, 9), release(0, 9))
	want := m.layout()

	again, _ := newModelWith(t, calm(), 80, 28, keep)
	if got := again.layout(); got.listH != want.listH || got.drawerH != want.drawerH {
		t.Errorf("after a restart: list %d, drawer %d; want %d, %d", got.listH, got.drawerH, want.listH, want.drawerH)
	}
}

// Saving ui.drawer_height on the settings page applies it now and keeps
// it in place of the dragged height.
func TestSettingsDrawerHeight(t *testing.T) {
	e := newSettingsEnv(t, settingsFile, nil)
	var kept []float64
	m, _ := newModelWith(t, calm(), 80, 28, func(o *Options) {
		o.Settings = e.hooks(config.Flags{})
		o.DrawerKept = 0.7
		o.KeepDrawer = func(v float64) error { kept = append(kept, v); return nil }
	})
	m, _ = press(m, keys("s")...)
	m = moveTo(t, m, config.KeyDrawerHeight)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.set.input.Value() != "0.5" {
		t.Fatalf("editing %q", m.set.input.Value())
	}
	m.set.input.SetValue("0.4")
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(e.file(t), "drawer_height = 0.4\n") || m.Status() != "saved ui.drawer_height = 0.4" {
		t.Fatalf("status %q, file:\n%s", m.Status(), e.file(t))
	}
	if m.frac != 0.4 || len(kept) != 1 || kept[0] != 0.4 {
		t.Errorf("frac %v, kept %v", m.frac, kept)
	}
	m, _ = press(m, esc)
	if l := m.layout(); l.listH != 23-10 {
		t.Errorf("after esc: list %d", l.listH)
	}
}

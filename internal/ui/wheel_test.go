package ui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/FredricW/herdr-deck/internal/source/fake"
)

func wheel(x, y int, down bool) tea.MouseWheelMsg {
	b := tea.MouseWheelUp
	if down {
		b = tea.MouseWheelDown
	}
	return tea.MouseWheelMsg{Button: b, X: x, Y: y}
}

// selection is everything a wheel must leave alone.
type selection struct {
	cursor, dcur int
	tab          tabKind
	isel, iedge  string
	focus        bool
}

func selOf(m Model) selection {
	return selection{m.cursor, m.dcur, m.curTab(), m.isel, m.iedge, m.dfocus}
}

// Over an edge's code the wheel scrolls the code; the list's row, the
// box and the edge stay.
func TestWheelOverEdgeCode(t *testing.T) {
	m := edgeModel(t, 80, 40)
	m, _ = press(m, keys("n")...)
	if !m.edgeShown() {
		t.Fatal("no edge code")
	}
	l := m.layout()
	before, shot := selOf(m), screen(m)
	_, _, e, _ := m.impactEdge()
	key := edgeSiteKey(fakeSnap().Threads[1], m.archs[diffKey(fakeSnap().Threads[1])], e.Sites[0])
	_, auto := m.edgeRows(m.ecode.views[key], l.listH)
	for range 3 {
		m, _ = press(m, wheel(10, l.listTop+2, true))
	}
	if got := selOf(m); got != before {
		t.Fatalf("the wheel moved a selection: %+v, was %+v", got, before)
	}
	if off := m.escroll.at(key); off <= auto || screen(m) == shot {
		t.Errorf("the code did not scroll: offset %d, from %d", off, auto)
	}
	m, _ = press(m, wheel(10, l.listTop-1, false)) // its header line too
	if got := selOf(m); got != before {
		t.Errorf("up over the header moved a selection: %+v", got)
	}
	// Another site or edge starts at its import again.
	m, _ = press(m, keys("n")...)
	_, _, e2, _ := m.impactEdge()
	if m.escroll.at(edgeSiteKey(fakeSnap().Threads[1], m.archs[diffKey(fakeSnap().Threads[1])], e2.Sites[0])) != -1 {
		t.Error("the next edge kept the scroll")
	}
}

// Over the Impact tab (findings, canvas, detail) the wheel scrolls the
// drawer; the finding, box or edge under the cursor stays.
func TestWheelOverImpact(t *testing.T) {
	for _, keysThen := range []string{"", "jjjjjjjjjj", "n"} {
		m := edgeModel(t, 80, 40)
		m, _ = press(m, keys(keysThen)...)
		l := m.layout()
		before, off := selOf(m), m.drawerOff
		for range 2 {
			m, _ = press(m, wheel(10, l.drawerTop+3, true))
		}
		if got := selOf(m); got != before {
			t.Errorf("after %q: the wheel moved a selection: %+v, was %+v", keysThen, got, before)
		}
		if m.drawerOff <= off {
			t.Errorf("after %q: the drawer did not scroll (%d)", keysThen, m.drawerOff)
		}
	}
}

// Over the diff preview the wheel scrolls it, not the list behind it.
func TestWheelOverPreview(t *testing.T) {
	m, _ := newModelWith(t, fakeSnap(), 80, 40, func(o *Options) { o.Diff, o.Patch = fake.Diff, fake.Patch })
	m, _ = press(m, keys("d")...)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = press(m, keys("jjj")...)
	if !m.preview {
		t.Fatal("no preview")
	}
	l := m.layout()
	before := selOf(m)
	m, _ = press(m, wheel(10, l.listTop+2, true))
	if got := selOf(m); got != before || m.prevOff == 0 {
		t.Errorf("selection %+v (was %+v), offset %d", got, before, m.prevOff)
	}
}

// Over the plain task list the wheel still moves the selection.
func TestWheelOverTheList(t *testing.T) {
	m, _ := newModel(t, calm(), 80, 28)
	l := m.layout()
	before := m.cursor
	m, _ = press(m, wheel(10, l.listTop+1, true))
	if m.cursor == before {
		t.Error("the wheel over the list did not move the selection")
	}
}

// The pane: the wheel over the code scrolls the code, over the canvas the
// canvas; the box and the edge stay.
func TestPaneWheel(t *testing.T) {
	for _, w := range []int{100, 200} {
		th := fakeSnap().Threads[1]
		p := NewImpactPane(ImpactOptions{Thread: th, Read: fake.Arch, Diff: fake.Diff, Patch: fake.Patch, FileAt: fake.FileAt})
		var step func(msg tea.Msg)
		step = func(msg tea.Msg) {
			next, cmd := p.Update(msg)
			p = next.(ImpactPane)
			for _, c := range flatten(cmd) {
				if v, ok := c().(edgeViewMsg); ok {
					step(v)
				}
			}
		}
		step(tea.WindowSizeMsg{Width: w, Height: 30})
		step(paneReadMsg{res: fake.Arch(context.Background(), th), diff: ptr(fake.Diff(context.Background(), th))})
		step(tea.KeyPressMsg{Code: 'n', Text: "n"})
		f := p.frame()
		sel, edge, cur, off := p.sel, p.edge, p.cur, p.off
		// Over the code.
		x, y := f.ex+5, 4
		if f.ex == 0 {
			x, y = 5, 2+f.dh+3
		}
		step(wheel(x, y, true))
		if p.sel != sel || p.edge != edge || p.cur != cur || p.off != off {
			t.Errorf("%d: the wheel over the code moved the canvas or a selection", w)
		}
		if p.escroll.off <= 0 && p.escroll.key == "" {
			t.Errorf("%d: the code did not scroll: %+v", w, p.escroll)
		}
		// Over the canvas.
		step(wheel(5, 4, true))
		if p.sel != sel || p.edge != edge || p.cur != cur || p.off <= off {
			t.Errorf("%d: over the canvas: off %d (was %d), selection %q %q", w, p.off, off, p.sel, p.edge)
		}
	}
}

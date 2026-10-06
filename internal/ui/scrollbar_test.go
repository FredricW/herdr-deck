package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// barString is the scrollbar as plain text, top to bottom.
func (s scrollbar) barString() string {
	return ansi.Strip(strings.Join(s.cells(false, false), ""))
}

func TestScrollbarThumb(t *testing.T) {
	cases := []struct {
		name string
		s    scrollbar
		want string
	}{
		{"fits", scrollbar{total: 10, h: 10}, ""},
		{"empty", scrollbar{total: 0, h: 5}, ""},
		{"no height", scrollbar{total: 50, h: 0}, ""},
		{"top", scrollbar{total: 20, h: 10}, "┃┃┃┃┃│││││"},
		{"bottom", scrollbar{total: 20, h: 10, off: 10}, "│││││┃┃┃┃┃"},
		{"past the end", scrollbar{total: 20, h: 10, off: 99}, "│││││┃┃┃┃┃"},
		{"before the start", scrollbar{total: 20, h: 10, off: -4}, "┃┃┃┃┃│││││"},
		{"middle", scrollbar{total: 40, h: 10, off: 15}, "││││┃┃┃│││"},
		// One line more than shows: the thumb is never the whole track.
		{"one more", scrollbar{total: 11, h: 10}, "┃┃┃┃┃┃┃┃┃│"},
		{"one more, scrolled", scrollbar{total: 11, h: 10, off: 1}, "│┃┃┃┃┃┃┃┃┃"},
		// Huge content: at least one row of thumb.
		{"huge", scrollbar{total: 100000, h: 10, off: 50000}, "│││││┃││││"},
		// A line off either end moves the thumb off that end.
		{"huge, a line down", scrollbar{total: 100000, h: 10, off: 1}, "│┃││││││││"},
		{"huge, a line up", scrollbar{total: 100000, h: 10, off: 99989}, "││││││││┃│"},
		{"tiny track", scrollbar{total: 3, h: 2, off: 1}, "│┃"},
	}
	for _, c := range cases {
		if got := c.s.barString(); !c.s.shown() && c.want != "" || c.s.shown() && got != c.want {
			t.Errorf("%s: %q (shown %v), want %q", c.name, got, c.s.shown(), c.want)
		}
		if c.want == "" && c.s.shown() {
			t.Errorf("%s: shown, want no scrollbar", c.name)
		}
	}
}

// Every offset gives a thumb within the track that moves with it, and
// the thumb's place maps back to an offset that shows it there.
func TestScrollbarRoundTrip(t *testing.T) {
	for _, total := range []int{11, 13, 20, 37, 100, 2001, 100000} {
		for _, h := range []int{2, 3, 10, 24} {
			s := scrollbar{total: total, h: h}
			if !s.shown() {
				continue
			}
			lastTop := -1
			for off := 0; off <= s.most(); off += max(s.most()/500, 1) {
				s.off = off
				top, size := s.thumb()
				if size < 1 || size >= h || top < 0 || top+size > h || top < lastTop {
					t.Fatalf("total %d h %d off %d: thumb %d+%d after %d", total, h, off, top, size, lastTop)
				}
				lastTop = top
			}
			_, size := s.thumb()
			if s.most() < h-size {
				continue // fewer offsets than places
			}
			for top := 0; top <= h-size; top++ {
				s.off = s.offsetAt(top)
				if got, _ := s.thumb(); got != top {
					t.Errorf("total %d h %d: offsetAt(%d) = %d, which shows the thumb at %d", total, h, top, s.off, got)
				}
			}
			if s.offsetAt(-3) != 0 || s.offsetAt(h+3) != s.most() {
				t.Errorf("total %d h %d: offsetAt off the track: %d, %d", total, h, s.offsetAt(-3), s.offsetAt(h+3))
			}
		}
	}
}

func TestScrollbarGrab(t *testing.T) {
	s := scrollbar{total: 40, h: 10, off: 15} // thumb rows 4-6
	if at, off := s.grab(5); at != 1 || off != 15 {
		t.Errorf("on the thumb: grab at %d, offset %d; want 1, 15", at, off)
	}
	// Below it, the thumb's middle jumps under the press.
	if at, off := s.grab(8); at != 1 || off != s.offsetAt(7) {
		t.Errorf("on the track: grab at %d, offset %d; want 1, %d", at, off, s.offsetAt(7))
	}
	if _, off := s.grab(0); off != 0 {
		t.Errorf("track top: offset %d, want 0", off)
	}
	if _, off := s.grab(9); off != s.most() {
		t.Errorf("track bottom: offset %d, want %d", off, s.most())
	}
}

// barColumn is column x of screen lines y0 to y1 of m.
func barColumn(m Model, x, y0, y1 int) string {
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	var b strings.Builder
	for _, l := range lines[y0:y1] {
		b.WriteString(ansi.Cut(l, x, x+1))
	}
	return b.String()
}

// The preview's scrollbar follows J K and the wheel, and a press and drag
// on it scrolls the preview.
func TestPreviewScrollbar(t *testing.T) {
	for _, w := range []int{80, 60} {
		m, _ := previewModel(t, w, 28, nil)
		m, _ = press(m, append(keys("dv"), keys("jjjjjjjj")...)...) // pnpm-lock.yaml: 31 lines
		l := m.layout()
		top, bottom := l.listTop, l.listTop+l.listH
		bar := func() string { return barColumn(m, w-1, top, bottom) }
		s := scrollbar{total: m.previewTotal(), h: l.listH}
		if s.total != 31 || bar() != s.barString() {
			t.Fatalf("%d cols: total %d, bar %q, want %q:\n%s", w, s.total, bar(), s.barString(), screen(m))
		}
		// Content keeps clear of the bar's column.
		if line := screenLine(m, top); ansi.StringWidth(line) != w || !strings.HasSuffix(line, "│") && !strings.HasSuffix(line, "┃") {
			t.Errorf("%d cols: first line %q", w, line)
		}
		m, _ = press(m, keys("JJJ")...)
		s.off = 3
		if bar() != s.barString() {
			t.Errorf("%d cols: J: bar %q, want %q", w, bar(), s.barString())
		}
		m, _ = press(m, tea.MouseWheelMsg{X: 5, Y: top + 2, Button: tea.MouseWheelDown})
		s.off = 6
		if m.prevOff != 6 || bar() != s.barString() {
			t.Errorf("%d cols: wheel: offset %d, bar %q, want %q", w, m.prevOff, bar(), s.barString())
		}
		// A press at the track's bottom scrolls to the end; the thumb is
		// held and turns blue while it moves.
		m, _ = press(m, click(w-1, bottom-1))
		if m.prevOff != s.most() || m.bar != barPreview {
			t.Errorf("%d cols: track bottom: offset %d (want %d), held %v", w, m.prevOff, s.most(), m.bar)
		}
		// Dragging it to the top scrolls back, and the release lets go.
		m, _ = press(m, motion(w-1, top-5))
		if m.prevOff != 0 {
			t.Errorf("%d cols: dragged to the top: offset %d", w, m.prevOff)
		}
		m, _ = press(m, motion(w-1, top+3), release(w-1, top+3))
		if m.prevOff == 0 || m.bar != barNone {
			t.Errorf("%d cols: dragged down: offset %d, held %v", w, m.prevOff, m.bar)
		}
		off := m.prevOff
		m, _ = press(m, motion(w-1, bottom))
		if m.prevOff != off {
			t.Errorf("%d cols: moved after the release: %d, want %d", w, m.prevOff, off)
		}
		// A click left of the bar does nothing, as before.
		m, _ = press(m, click(w-2, top+3))
		if m.prevOff != off || m.bar != barNone || !m.preview {
			t.Errorf("%d cols: click beside the bar: offset %d, held %v", w, m.prevOff, m.bar)
		}
	}
}

// A preview that fits has no scrollbar, and its lines keep the column.
func TestPreviewNoScrollbar(t *testing.T) {
	m, _ := previewModel(t, 80, 40, nil)
	m, _ = press(m, append(keys("dv"), keys("jj")...)...) // columns.ts: one line
	l := m.layout()
	if col := barColumn(m, 79, l.listTop, l.listTop+l.listH); strings.Trim(col, " ") != "" {
		t.Errorf("a scrollbar on a short diff: %q\n%s", col, screen(m))
	}
	m, _ = press(m, click(79, l.listTop))
	if m.bar != barNone {
		t.Error("a click held a scrollbar that is not there")
	}
}

// A full view that overflows (help) gets the scrollbar after a blank
// column, and a drag on it scrolls the view.
func TestFullViewScrollbar(t *testing.T) {
	m, _ := newModelWith(t, calm(), 80, 24, func(*Options) {})
	m, _ = press(m, keys("?")...)
	l := m.layout()
	if !l.barred || l.drawer.width != 78 {
		t.Fatalf("help: barred %v, width %d", l.barred, l.drawer.width)
	}
	top, bottom := l.drawerTop, l.drawerTop+l.drawerH
	s := l.drawerBar(0)
	if got := barColumn(m, 79, top, bottom); got != s.barString() {
		t.Errorf("bar %q, want %q", got, s.barString())
	}
	if gap := barColumn(m, 78, top, bottom); strings.Trim(gap, " ") != "" {
		t.Errorf("no blank column before the bar: %q", gap)
	}
	m, _ = press(m, click(79, bottom-1), release(79, bottom-1))
	if m.drawerOff != s.most() || m.bar != barNone {
		t.Errorf("track bottom: offset %d, want %d", m.drawerOff, s.most())
	}
	m, _ = press(m, click(79, bottom-1), motion(79, top), release(79, top))
	if m.drawerOff != 0 {
		t.Errorf("dragged to the top: offset %d", m.drawerOff)
	}
	// A row's drawer keeps its width and its "↓ N more".
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if l := m.layout(); l.barred {
		t.Error("a row's drawer got a scrollbar")
	}
}

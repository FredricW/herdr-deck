package ui

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/arch"
	"github.com/FredricW/herdr-deck/internal/source/diff"
	"github.com/FredricW/herdr-deck/internal/source/fake"
)

// The selected edge's line is drawn heavy, its whole length and its
// corners, in its colour, bold; ┅ ┇ for a removed one. The other lines
// stay light.
func TestHeavyEdge(t *testing.T) {
	r := sampleShop()
	c := newCanvas(r, 120)
	c.focus(r, "internal/ui")
	removed := -1
	for i, e := range c.routed {
		if e.Status == arch.Removed {
			removed = i
		}
	}
	if removed < 0 || len(c.routed) < 2 {
		t.Fatalf("routed %v", c.routed)
	}
	for sel := range c.routed {
		rows := c.rows(sel)
		for y, row := range rows {
			plain := []rune(ansi.Strip(row))
			for x, cl := range c.g.cells[y] {
				if cl.kind != cellLine {
					continue
				}
				got := plain[x]
				switch {
				case cl.edge == sel+1 && !strings.ContainsRune("━┃┏┓┗┛┅┇", got):
					t.Errorf("edge %d: a light %q at %d,%d on the selected line", sel, got, x, y)
				case cl.edge != sel+1 && !strings.ContainsRune("─│┌┐└┘┄┆", got):
					t.Errorf("edge %d: a heavy %q at %d,%d on another line", sel, got, x, y)
				case cl.edge == sel+1 && c.routed[sel].Status == arch.Removed && strings.ContainsRune("━┃", got):
					t.Errorf("the removed edge is solid at %d,%d", x, y)
				}
			}
		}
	}
	// Bold in its colour.
	if r, st := heavy('┄', csRemoved); r != '┅' || st != csRemovedHeavy || !st.style().GetBold() {
		t.Errorf("heavy ┄ = %q %v", r, st)
	}
	if r, st := heavy('┆', csRemoved); r != '┇' || st.style().GetForeground() != failStyle.GetForeground() {
		t.Errorf("heavy ┆ = %q %v", r, st)
	}
}

// A click finds the line under it, or one a step away; where lines
// cross, the one drawn first, which runs on top.
func TestLineAt(t *testing.T) {
	r := sampleShop()
	c := newCanvas(r, 120)
	c.focus(r, "internal/ui")
	seen := 0
	for y, row := range c.g.cells {
		for x, cl := range row {
			if cl.edge == 0 {
				continue
			}
			if got := c.lineAt(x, y); got != cl.edge-1 {
				t.Errorf("on %d,%d: %d, want %d", x, y, got, cl.edge-1)
			}
			seen++
		}
	}
	if seen == 0 {
		t.Fatal("no line cells")
	}
	// A near miss: a free cell next to a line finds it; one two cells
	// from every line finds none.
	near, far := false, false
	for y := 1; y < c.g.h-1; y++ {
		for x := 1; x < c.g.w-1; x++ {
			if c.g.cells[y][x].edge != 0 {
				continue
			}
			next := 0
			for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
				next = max(next, c.g.cells[y+d[1]][x+d[0]].edge)
			}
			got := c.lineAt(x, y)
			switch {
			case next > 0 && got < 0:
				t.Errorf("next to a line at %d,%d: none", x, y)
			case next > 0:
				near = true
			case got >= 0:
				t.Errorf("no line next to %d,%d, but %d", x, y, got)
			default:
				far = true
			}
		}
	}
	if !near || !far {
		t.Errorf("near %v far %v", near, far)
	}
	// At a crossing the cell keeps the first line's glyph and owner.
	g, bx, cv := threeInARow(true)
	g.route(bx["a"], bx["c"], arch.Added, cv.lineOf(bx["a"], bx["c"]), 1)
	g.route(bx["b"], bx["b"], arch.Removed, cv.lineOf(bx["b"], bx["b"]), 2)
	for y := range g.h {
		for x := range g.w {
			if cl := g.cells[y][x]; cl.kind == cellLine && cl.edge == 1 && !strings.ContainsRune("─│┌┐└┘", cl.r) {
				t.Errorf("the first line's cell %d,%d became %q", x, y, cl.r)
			}
		}
	}
}

func TestHunkAt(t *testing.T) {
	p := diff.ParsePatch([]byte("@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n@@ -10,2 +10,3 @@\n x\n+y\n z\n"), diff.MaxPatchLines)
	for _, c := range []struct {
		line int
		base bool
		want string // the marked line's text
		hunk int    // lines in the hunk
	}{
		{2, false, "B", 5}, {2, true, "b", 5}, {11, false, "y", 4}, {12, false, "z", 4}, {10, true, "x", 4},
	} {
		h, mark, ok := hunkAt(p, c.line, c.base)
		if !ok || h.Lines[mark].Text != c.want || len(h.Lines) != c.hunk || h.Lines[0].Kind != deck.LineHunk {
			t.Errorf("line %d base %v: %v %d %+v", c.line, c.base, ok, mark, h.Lines)
		}
	}
	if _, _, ok := hunkAt(p, 6, false); ok {
		t.Error("line 6 is in no hunk")
	}
	cp, mark := contextPatch([]byte("1\n2\n3\n4\n5\n6\n7\n8\n"), 2, 3)
	if cp.Lines[0].Text != "@@ -1,5 +1,5 @@" || cp.Lines[mark].Text != "2" || len(cp.Lines) != 6 {
		t.Errorf("context %+v mark %d", cp.Lines, mark)
	}
}

// The code view by status: an added import's hunk, a removed one on the
// base side, an unchanged one in its file with a note, a changed edge's
// names in the header, and the site out of several.
func TestEdgeViewByStatus(t *testing.T) {
	th := deck.Thread{ID: "t-1", Worktree: "/wt"}
	res := &arch.Result{MergeBase: "base1", Head: "head1"}
	d := deck.Diff{MergeBase: "base1", Files: []deck.DiffFile{{Path: "a/a.go"}, {Path: "gone/g.go", Change: deck.ChangeDeleted}}}
	rd := edgeReaders{
		patch: func(_ context.Context, _ deck.Thread, _ string, f deck.DiffFile) deck.Patch {
			src := map[string]string{
				"a/a.go":    "@@ -1,3 +1,4 @@\n package a\n import (\n+\t\"x/b\"\n )\n",
				"gone/g.go": "@@ -1,2 +0,0 @@\n-package gone\n-import \"x/b\"\n",
			}[f.Path]
			return diff.ParsePatch([]byte(src), diff.MaxPatchLines)
		},
		fileAt: func(_ context.Context, _ deck.Thread, rev, path string) ([]byte, error) {
			if rev != "head1" || path != "c/c.go" {
				return nil, errors.New("no such file")
			}
			return []byte("package c\n\nimport \"x/b\"\n\nfunc C() {}\n"), nil
		},
	}
	added := readEdgeView(context.Background(), rd, th, d, res, arch.Site{File: "a/a.go", Line: 3})
	if added.note != "" || added.p.patch.Lines[added.mark].Kind != deck.LineAdded || !strings.Contains(added.p.patch.Lines[added.mark].Text, "x/b") {
		t.Errorf("added: %+v", added)
	}
	removed := readEdgeView(context.Background(), rd, th, d, res, arch.Site{File: "gone/g.go", Line: 2, Base: true})
	if removed.p.patch.Lines[removed.mark].Kind != deck.LineDeleted {
		t.Errorf("removed: %+v", removed)
	}
	same := readEdgeView(context.Background(), rd, th, d, res, arch.Site{File: "c/c.go", Line: 3})
	if same.note != "import unchanged" || same.p.patch.Lines[same.mark].Text != "import \"x/b\"" || same.p.patch.Lines[same.mark].Kind != deck.LineContext {
		t.Errorf("unchanged: %+v", same)
	}
	missing := readEdgeView(context.Background(), rd, th, d, res, arch.Site{File: "z/z.go", Line: 1})
	if missing.mark != -1 || !strings.Contains(missing.note, "no such file") {
		t.Errorf("missing: %+v", missing)
	}

	m := Model{width: 80}
	e := arch.Edge{From: "a", To: "b", Status: arch.Changed, Gained: []string{"Render"}, Lost: []string{"Draw"},
		Sites: []arch.Site{{File: "a/a.go", Line: 3}, {File: "a/b.go", Line: 4}}}
	lines := m.edgeLines(e, added, true, 1, 80, 10)
	head := ansi.Strip(lines[0])
	for _, want := range []string{"a ─▸ b", "changed", "2 sites", "+Render", "−Draw", "site 2/2", "b.go:4"} {
		if !strings.Contains(head, want) {
			t.Errorf("header %q lacks %q", head, want)
		}
	}
	if len(lines) != 11 {
		t.Errorf("%d lines", len(lines))
	}
	marked := 0
	for _, l := range lines[1:] {
		if strings.HasPrefix(ansi.Strip(l), "▌") {
			marked++
			if !strings.Contains(l, "x/b") {
				t.Errorf("the mark is on %q", ansi.Strip(l))
			}
		}
	}
	if marked != 1 {
		t.Errorf("%d marked rows", marked)
	}
	if s := ansi.Strip(strings.Join(m.edgeLines(e, same, true, 0, 80, 10), "\n")); !strings.Contains(s, "import unchanged") {
		t.Errorf("unchanged view:\n%s", s)
	}
}

// edgeModel is the sample deck focused on the Impact tab with the api
// box selected (finding 1), at a normal drawer height.
func edgeModel(t *testing.T, w, h int) Model {
	t.Helper()
	m, _ := newModelWith(t, fakeSnap(), w, h, func(o *Options) {
		o.Diff, o.Patch, o.Arch, o.FileAt = fake.Diff, fake.Patch, fake.Arch, fake.FileAt
	})
	m.switchTab(tabImpact)
	m, _ = press(m, tabKey)
	if m.isel != "src/api" {
		t.Fatalf("selection %q", m.isel)
	}
	return m
}

// n and N step through the box's edges, wrapping; ] and [ too once an
// edge is selected; esc goes back to the box, then to the list.
func TestEdgeKeys(t *testing.T) {
	m := edgeModel(t, 80, 40)
	d, lay := impactLay(t, m)
	var rows []string
	for _, s := range d.stops {
		if s.act.kind == actEdge {
			rows = append(rows, edgeKey(lay.edges[s.act.n]))
		}
	}
	if len(rows) < 3 {
		t.Fatalf("edge rows %q", rows)
	}
	for i := range len(rows) + 1 {
		m, _ = press(m, keys("n")...)
		if want := rows[i%len(rows)]; m.iedge != want {
			t.Fatalf("n %d: %q, want %q", i+1, m.iedge, want)
		}
	}
	m, _ = press(m, keys("NN")...) // back past the first, wrapping
	if want := rows[len(rows)-2]; m.iedge != want {
		t.Errorf("N N: %q, want %q", m.iedge, want)
	}
	m, _ = press(m, keys("]")...)
	if m.iedge != rows[len(rows)-1] || m.curTab() != tabImpact {
		t.Errorf("]: %q on %v", m.iedge, m.curTab())
	}
	m, _ = press(m, keys("[")...)
	if m.iedge != rows[len(rows)-2] {
		t.Errorf("[: %q", m.iedge)
	}
	if !m.edgeShown() || !strings.Contains(screen(m), "─▸") {
		t.Error("the main view does not show the edge")
	}
	m, _ = press(m, esc)
	if m.iedge != "" || m.isel != "src/api" || !m.dfocus || m.edgeShown() {
		t.Errorf("esc: edge %q box %q focus %v", m.iedge, m.isel, m.dfocus)
	}
	if strings.Contains(screen(m), "· added ·") {
		t.Error("the main view still shows an edge")
	}
	m, _ = press(m, esc)
	if m.dfocus {
		t.Error("a second esc kept the focus")
	}
	// Without a selected edge, [ and ] switch tabs as ever.
	m = edgeModel(t, 80, 40)
	m, _ = press(m, keys("]")...)
	if m.curTab() == tabImpact {
		t.Error("] did not switch tabs without an edge")
	}
}

// Tab and shift+tab (or . and ,) step through an edge's sites.
func TestEdgeSites(t *testing.T) {
	m := edgeModel(t, 80, 40)
	e := arch.Edge{From: "x", To: "y", Sites: []arch.Site{{File: "a"}, {File: "b"}, {File: "c"}}}
	m.cycleSite(e, 1)
	m.cycleSite(e, 1)
	if m.edgeSite(e) != 2 {
		t.Errorf("site %d", m.edgeSite(e))
	}
	m.cycleSite(e, 1)
	if m.edgeSite(e) != 0 {
		t.Errorf("site %d after wrapping", m.edgeSite(e))
	}
	m.cycleSite(e, -1)
	if m.edgeSite(e) != 2 {
		t.Errorf("site %d back", m.edgeSite(e))
	}
	other := arch.Edge{From: "x", To: "z", Sites: e.Sites}
	if m.edgeSite(other) != 0 {
		t.Error("another edge starts at its first site")
	}
	// The keys, on the sample's edge with one site, say so.
	m, _ = press(m, keys("n")...)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.curTab() != tabImpact || !strings.Contains(m.status, "one site") {
		t.Errorf("tab on an edge: %v %q", m.curTab(), m.status)
	}
}

// A click on a line selects its edge with its source box; a second one
// opens the site in the diff preview.
func TestEdgeClick(t *testing.T) {
	m := edgeModel(t, 80, 60)
	m, _ = press(m, keys("z")...)
	l := m.layout()
	lay := l.drawer.impact
	var x, y, owner int
	found := false
	for gy, row := range lay.c.g.cells {
		for gx, cl := range row {
			if cl.kind == cellLine && !found {
				x, y, owner, found = gx, gy, cl.edge-1, true
			}
		}
	}
	if !found {
		t.Fatal("no line to click")
	}
	e := lay.c.routed[owner]
	sy := l.drawerTop + lay.top + y - m.drawerOff
	if sy < l.drawerTop || sy >= l.drawerTop+l.drawerH {
		t.Fatalf("the line is off screen: %d", sy)
	}
	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: lay.left + x, Y: sy}
	m, _ = press(m, click)
	if m.iedge != edgeKey(e) || m.isel != e.From {
		t.Fatalf("first click: edge %q box %q, want %q with %s", m.iedge, m.isel, edgeKey(e), e.From)
	}
	// The click dropped the drawer to its normal height for the code.
	if m.effectiveSize() != sizeNormal || !m.edgeShown() {
		t.Errorf("size %v, shown %v", m.effectiveSize(), m.edgeShown())
	}
	// Click it again where it is drawn now.
	l = m.layout()
	lay = l.drawer.impact
	idx := lay.c.routedIndex(e.From, e.To)
	for gy, row := range lay.c.g.cells {
		for gx, cl := range row {
			if cl.edge == idx+1 && cl.kind == cellLine {
				x, y = gx, gy
			}
		}
	}
	if sy := l.drawerTop + lay.top + y - m.drawerOff; sy >= l.drawerTop && sy < l.drawerTop+l.drawerH {
		m, _ = press(m, tea.MouseClickMsg{Button: tea.MouseLeft, X: lay.left + x, Y: sy})
		if m.curTab() != tabFiles {
			t.Errorf("second click: tab %v, status %q", m.curTab(), m.status)
		}
	}
}

func TestEdgeGolden(t *testing.T) {
	for _, w := range []int{60, 80, 120} {
		m := edgeModel(t, w, 40)
		m, _ = press(m, keys("n")...)
		golden(t, "impact-edge-"+strconv.Itoa(w), m)
	}
}

// The pane shows the edge's code below the canvas, or beside it when
// wide.
func TestPaneEdge(t *testing.T) {
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
		step(tea.WindowSizeMsg{Width: w, Height: 40})
		step(paneReadMsg{res: fake.Arch(context.Background(), th), diff: fake.Diff(context.Background(), th)})
		step(tea.KeyPressMsg{Code: 'n', Text: "n"})
		if p.edge == "" {
			t.Fatalf("%d: no edge selected", w)
		}
		lines := strings.Split(ansi.Strip(p.render()), "\n")
		if len(lines) != 40 {
			t.Errorf("%d: %d lines", w, len(lines))
		}
		header, marked := -1, -1
		for i, l := range lines {
			if ansi.StringWidth(l) > w {
				t.Errorf("%d: line %d is %d wide", w, i, ansi.StringWidth(l))
			}
			if header < 0 && strings.Contains(l, "─▸ src/admin/users · added") {
				header = i
			}
			if strings.Contains(l, "▌") {
				marked = i
			}
		}
		if header < 0 || marked < 0 {
			t.Fatalf("%d: no code view:\n%s", w, strings.Join(lines, "\n"))
		}
		beside := strings.Index(lines[header], "─▸ src/admin/users") > w/2
		if beside != (w >= paneBeside) {
			t.Errorf("%d: beside %v", w, beside)
		}
		step(tea.KeyPressMsg{Code: tea.KeyEscape})
		if p.edge != "" || p.sel != "src/api" {
			t.Errorf("%d: esc: edge %q box %q", w, p.edge, p.sel)
		}
	}
}

// flatten lists a command's commands, batches opened.
func flatten(cmd tea.Cmd) []tea.Cmd {
	if cmd == nil {
		return nil
	}
	return []tea.Cmd{func() tea.Msg {
		msg := cmd()
		if b, ok := msg.(tea.BatchMsg); ok {
			for _, c := range b {
				for _, f := range flatten(c) {
					if v := f(); v != nil {
						if _, ok := v.(edgeViewMsg); ok {
							return v
						}
					}
				}
			}
			return nil
		}
		return msg
	}}
}

// Selecting an edge at full height drops the drawer to its normal height,
// where fewer findings show; the cursor stays on the edge's row.
func TestEdgeFromFullHeight(t *testing.T) {
	m := edgeModel(t, 80, 40)
	m, _ = press(m, keys("z")...)
	if m.effectiveSize() != sizeFull {
		t.Fatal("not at full height")
	}
	m, _ = press(m, keys("n")...)
	d, lay := impactLay(t, m)
	if m.effectiveSize() != sizeNormal || d.stops[m.dcur].act.kind != actEdge || edgeKey(lay.edges[d.stops[m.dcur].act.n]) != m.iedge {
		t.Errorf("size %v, stop %v, edge %q", m.effectiveSize(), d.stops[m.dcur].act, m.iedge)
	}
	m, _ = press(m, keys("n")...)
	if d, lay := impactLay(t, m); edgeKey(lay.edges[d.stops[m.dcur].act.n]) != m.iedge || m.iedge == "" {
		t.Errorf("the second n: %q", m.iedge)
	}
}

package ui

import (
	"context"
	"fmt"
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

// impactModel is the sample deck on t-0002 (it needs the user, so the
// cursor rests on it) with the Impact tab shown and its read done.
func impactModel(t *testing.T, w, h int, read func(context.Context, deck.Thread) *arch.Result) Model {
	t.Helper()
	m, _ := newModelWith(t, fakeSnap(), w, h, func(o *Options) {
		o.Diff = fake.Diff
		o.Arch = read
	})
	m.switchTab(tabImpact)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyF13}) // any key: reads follow
	return m
}

func TestImpactGolden(t *testing.T) {
	for _, w := range []int{60, 80, 120} {
		h := 28
		if w == 120 {
			h = 36
		}
		m := impactModel(t, w, h, fake.Arch)
		if r, ok := m.selected(); !ok || r.key == "" || m.curTab() != tabImpact {
			t.Fatalf("%d: not on the Impact tab", w)
		}
		golden(t, "impact-"+strconv.Itoa(w), m)
		full, _ := press(m, keys("z")...)
		golden(t, "impact-full-"+strconv.Itoa(w), full)
	}
}

func TestImpactStates(t *testing.T) {
	// Before the first read: the label and the tab say so.
	m, _ := newModelWith(t, fakeSnap(), 80, 28, func(o *Options) {
		o.Diff = fake.Diff
		o.Arch = fake.Arch
	})
	m.switchTab(tabImpact)
	golden(t, "impact-reading-80", m)
	if s := screen(m); !strings.Contains(s, "Impact …") || !strings.Contains(s, "reading the change's shape…") {
		t.Errorf("reading:\n%s", s)
	}

	note := impactModel(t, 80, 28, func(context.Context, deck.Thread) *arch.Result {
		return &arch.Result{Base: "origin/main", Note: "cannot compare with origin/main: unknown revision"}
	})
	if s := screen(note); !strings.Contains(s, "cannot compare with origin/main") {
		t.Errorf("note:\n%s", s)
	}

	// Off: no tab at all.
	off, _ := newModelWith(t, fakeSnap(), 80, 28, func(o *Options) { o.Diff = fake.Diff })
	if strings.Contains(screen(off), "Impact") {
		t.Error("the tab shows while off")
	}
}

// The tab reads on every reload, from the reader's cache, and only for a
// thread with a worktree.
func TestImpactReads(t *testing.T) {
	var reads []string
	m := impactModel(t, 80, 28, func(_ context.Context, th deck.Thread) *arch.Result {
		reads = append(reads, th.ID)
		return fake.Arch(context.Background(), th)
	})
	if len(reads) != 1 || reads[0] != "t-0002" {
		t.Fatalf("reads = %v", reads)
	}
	// Moving within the same thread reads nothing new.
	m, _ = press(m, keys("[]")...)
	if len(reads) != 1 {
		t.Errorf("reads after switching tabs = %v", reads)
	}
	// A reload reads again (the reader answers from its cache).
	m, _ = press(m, snapshotMsg(fakeSnap()))
	if len(reads) != 2 {
		t.Errorf("reads after a reload = %v", reads)
	}
	if !strings.Contains(m.impactLabel(), "Impact ") {
		t.Errorf("label = %q", m.impactLabel())
	}
}

// focusedImpact is impactModel at full height with the drawer focused on
// Look here first's first row.
func focusedImpact(t *testing.T, w, h int, set func(*Options)) Model {
	t.Helper()
	m, _ := newModelWith(t, fakeSnap(), w, h, func(o *Options) {
		o.Diff = fake.Diff
		o.Patch = fake.Patch
		o.Arch = fake.Arch
		set(o)
	})
	m.switchTab(tabImpact)
	m, _ = press(m, keys("z")...)
	m, _ = press(m, tabKey)
	return m
}

func impactLay(t *testing.T, m Model) (*drawer, *impactLayout) {
	t.Helper()
	d := m.layout().drawer
	if d == nil || d.impact == nil {
		t.Fatal("no Impact layout")
	}
	return d, d.impact
}

// The cursor goes through the findings (each selecting its box), into the
// canvas, from box to box, and on into the detail; k comes back.
func TestImpactNavigation(t *testing.T) {
	m := focusedImpact(t, 80, 50, func(*Options) {})
	_, lay := impactLay(t, m)
	if m.dcur != 0 || m.isel != lay.res.Findings[0].Package {
		t.Fatalf("on focus: cursor %d, selection %q", m.dcur, m.isel)
	}
	for range lay.findings - 1 {
		m, _ = press(m, keys("j")...)
	}
	last := lay.res.Findings[lay.findings-1].Package
	m, _ = press(m, keys("j")...)
	if d, lay := impactLay(t, m); d.stops[m.dcur].act.kind != actBox || m.isel != last {
		t.Fatalf("j from the last finding: stop %v, selection %q; want the box of %q", d.stops[m.dcur].act, m.isel, last)
	} else if lay.sel != last {
		t.Errorf("drawn selection %q", lay.sel)
	}
	// Spatially, as the canvas steps.
	_, lay = impactLay(t, m)
	from := lay.c.boxes[m.isel]
	for _, k := range []struct {
		key    string
		dx, dy int
	}{{"l", 1, 0}, {"h", -1, 0}, {"j", 0, 1}, {"k", 0, -1}} {
		want := lay.c.step(from, k.dx, k.dy)
		if want == nil {
			continue
		}
		next, _ := press(m, keys(k.key)...)
		if next.isel != want.path {
			t.Errorf("%s from %s: %q, want %q", k.key, from.path, next.isel, want.path)
		}
	}
	// The selection's lines show only while the drawer has the focus.
	if s := screen(m); !strings.Contains(s, "┏") {
		t.Errorf("no heavy border on the selection:\n%s", s)
	}
	unfocused, _ := press(m, esc)
	if s := screen(unfocused); strings.Contains(s, "┏") || !strings.Contains(s, "1!") {
		t.Errorf("at rest, the canvas should show markers, not a selection:\n%s", s)
	}
	// From the bottom box, j goes into the detail and k comes back.
	for range 10 {
		m, _ = press(m, keys("j")...)
	}
	d, lay := impactLay(t, m)
	if d.stops[m.dcur].act.kind != actSite {
		t.Fatalf("j past the canvas: stop %v", d.stops[m.dcur].act)
	}
	box := m.isel
	for d.stops[m.dcur].act.kind == actSite {
		m, _ = press(m, keys("k")...)
		d, lay = impactLay(t, m)
	}
	if d.stops[m.dcur].act.kind != actBox || lay.boxes[d.stops[m.dcur].act.n].path != box {
		t.Errorf("k out of the detail: stop %v, want the box %s", d.stops[m.dcur].act, box)
	}
}

// ↵ on a finding shows its file in the diff preview, scrolled to its line.
func TestImpactEnterOpensTheLine(t *testing.T) {
	const file = "src/api/users.ts"
	res := &arch.Result{Base: "origin/main", MergeBase: "4f1c2a9", Head: "c3a91f0", Name: "webshop", Language: arch.LangTS, Inferred: true,
		Packages: []arch.Package{{Path: "src/api", Status: arch.Changed, Impacted: true, Files: 1, Changed: []string{file}}},
		Findings: []arch.Finding{{Kind: arch.FindTouch, Sign: "+", Text: "env ABC_TOKEN", Why: "in src/api", Package: "src/api", Edge: -1,
			Site: arch.Site{File: file, Line: 40}}},
	}
	var body strings.Builder
	body.WriteString("@@ -1,60 +1,60 @@\n")
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&body, " line %d\n", i)
	}
	m := focusedImpact(t, 80, 28, func(o *Options) {
		o.Arch = func(context.Context, deck.Thread) *arch.Result { return res }
		o.Patch = func(context.Context, deck.Thread, string, deck.DiffFile) deck.Patch {
			return diff.ParsePatch([]byte(body.String()), diff.MaxPatchLines)
		}
	})
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.curTab() != tabFiles || !m.preview {
		t.Fatalf("tab %v, preview %v; status %q", m.curTab(), m.preview, m.status)
	}
	s := screen(m)
	if !strings.Contains(s, "line 40") || strings.Contains(s, "line 30\n") {
		t.Errorf("the preview is not at line 40:\n%s", s)
	}
	// A file the branch did not change has no diff to show.
	res.Findings[0].Site.File = "src/api/client.ts"
	m = focusedImpact(t, 80, 28, func(o *Options) {
		o.Arch = func(context.Context, deck.Thread) *arch.Result { return res }
	})
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.curTab() != tabImpact || !strings.Contains(m.status, "did not change") {
		t.Errorf("tab %v, status %q", m.curTab(), m.status)
	}
}

// A click on a box selects it, a second click opens its first changed
// file; a click on a finding selects it, a second one opens it.
func TestImpactClick(t *testing.T) {
	m := focusedImpact(t, 80, 60, func(*Options) {})
	m, _ = press(m, esc) // the click gives the focus back
	l := m.layout()
	lay := l.drawer.impact
	b := lay.c.boxes["src/admin/users"]
	if b == nil {
		t.Fatal("no users box")
	}
	line := lay.top + b.y + 1
	if line-m.drawerOff >= l.drawerH {
		t.Fatalf("the box is below the drawer: line %d, height %d", line, l.drawerH)
	}
	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: lay.left + b.x + 2, Y: l.drawerTop + line - m.drawerOff}
	m, _ = press(m, click)
	if !m.dfocus || m.isel != "src/admin/users" || m.curTab() != tabImpact {
		t.Fatalf("first click: focus %v, selection %q, tab %v", m.dfocus, m.isel, m.curTab())
	}
	m, _ = press(m, click)
	if m.curTab() != tabFiles || !m.preview {
		t.Fatalf("second click: tab %v, preview %v, status %q", m.curTab(), m.preview, m.status)
	}
	if _, _, f, _ := m.previewFile(); f.Path != "src/admin/users/UsersList.tsx" {
		t.Errorf("previews %s", f.Path)
	}

	// A finding: the third row.
	m = focusedImpact(t, 80, 60, func(*Options) {})
	l = m.layout()
	row := tea.MouseClickMsg{Button: tea.MouseLeft, X: 10, Y: l.drawerTop + l.drawer.stops[2].line - m.drawerOff}
	m, _ = press(m, row)
	if m.dcur != 2 || m.curTab() != tabImpact || m.isel != "src/admin/users" {
		t.Fatalf("first click on a finding: cursor %d, tab %v, selection %q", m.dcur, m.curTab(), m.isel)
	}
	m, _ = press(m, row)
	if m.curTab() != tabFiles {
		t.Errorf("second click on a finding: tab %v, status %q", m.curTab(), m.status)
	}
}

// O opens the row's Impact view in a pane of its own; without herdr it
// says so.
func TestImpactPaneKey(t *testing.T) {
	var opened []string
	m := impactModel(t, 80, 28, fake.Arch)
	m.opt.OpenImpact = func(th deck.Thread) error { opened = append(opened, th.ID+" "+th.Worktree); return nil }
	m, _ = press(m, keys("O")...)
	if len(opened) != 1 || opened[0] != "t-0002 /src/worktrees/t-0002" || !strings.Contains(m.status, "Impact view") {
		t.Errorf("opened %v, status %q", opened, m.status)
	}
	m.opt.OpenImpact = nil
	m, _ = press(m, keys("O")...)
	if !strings.Contains(m.status, "needs herdr") {
		t.Errorf("status %q", m.status)
	}
}

func TestImpactSelectGolden(t *testing.T) {
	for _, w := range []int{60, 80, 120} {
		m := focusedImpact(t, w, 46, func(*Options) {})
		_, lay := impactLay(t, m)
		for range lay.findings {
			m, _ = press(m, keys("j")...)
		}
		m, _ = press(m, keys("j")...) // one box down: a selection with lines
		golden(t, "impact-select-"+strconv.Itoa(w), m)
	}
}

// The Impact view on its own: the same stops, ↵ to the diff tool, q.
func TestImpactPane(t *testing.T) {
	var opened []string
	th := fakeSnap().Threads[1]
	p := NewImpactPane(ImpactOptions{Thread: th, Read: fake.Arch, OpenDiff: func(path, base string, files []string) error {
		opened = append(opened, path+" "+base+" "+strings.Join(files, " "))
		return nil
	}})
	feed := func(msgs ...tea.Msg) {
		for _, msg := range msgs {
			next, cmd := p.Update(msg)
			p = next.(ImpactPane)
			if cmd != nil {
				if res, ok := cmd().(openedMsg); ok {
					next, _ := p.Update(res)
					p = next.(ImpactPane)
				}
			}
		}
	}
	feed(tea.WindowSizeMsg{Width: 100, Height: 40})
	if s := ansi.Strip(p.render()); !strings.Contains(s, "reading the change's shape") {
		t.Errorf("before the read:\n%s", s)
	}
	feed(paneReadMsg{fake.Arch(context.Background(), th)})
	pg := p
	pg.width, pg.height = 100, 40
	lines := strings.Split(ansi.Strip(p.render()), "\n")
	if len(lines) != 40 {
		t.Errorf("%d lines", len(lines))
	}
	for i, l := range lines {
		if ansi.StringWidth(l) > 100 {
			t.Errorf("line %d is %d wide", i, ansi.StringWidth(l))
		}
	}
	if !strings.Contains(lines[0], "Impact · t-0002") {
		t.Errorf("header %q", lines[0])
	}
	// ↵ on the first finding opens its file in the diff tool.
	feed(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(opened) != 1 || opened[0] != "/src/worktrees/t-0002 0000000 src/api/users.ts" || !strings.Contains(p.status, "diff tool") {
		t.Errorf("opened %v, status %q", opened, p.status)
	}
	// Down to the canvas: the selection moves box by box.
	d, _, _ := p.body()
	for range d.impact.findings {
		feed(tea.KeyPressMsg{Code: 'j', Text: "j"})
	}
	if d, _, _ := p.body(); d.stops[p.cur].act.kind != actBox || p.sel == "" {
		t.Errorf("stop %v, selection %q", d.stops[p.cur].act, p.sel)
	}
	if _, cmd := p.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); cmd == nil {
		t.Error("q does not quit")
	} else if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q does not quit")
	}
}

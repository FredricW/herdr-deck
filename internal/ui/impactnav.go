package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/FredricW/herdr-deck/internal/source/arch"
)

// Moving around the Impact tab. With the drawer focused, the cursor goes
// through Look here first's rows, then the canvas's boxes, then the
// selected box's detail. On a box, the arrows and hjkl move to the nearest
// box that way; off the canvas's edge, j and k go on into the detail or
// back to the findings. The cursor on a box, or on a finding, selects its
// box: its edges are drawn as lines and the detail replaces the legend.

// impactStep is the stop the cursor moves to from stop cur in direction
// dir (up, down, left or right).
func impactStep(lay *impactLayout, stops []stop, cur int, dir string) int {
	if lay == nil || len(stops) == 0 {
		return cur
	}
	cur = clamp(cur, 0, len(stops)-1)
	f, nb := lay.findings, len(lay.boxes)
	if a := stops[cur].act; a.kind == actBox && a.n < nb {
		var dx, dy int
		switch dir {
		case "down":
			dy = 1
		case "up":
			dy = -1
		case "right":
			dx = 1
		default:
			dx = -1
		}
		if to := lay.c.step(lay.boxes[a.n], dx, dy); to != nil {
			return lay.boxStop(to.path)
		}
		switch {
		case dir == "down" && f+nb < len(stops):
			return f + nb // the detail
		case dir == "up" && f > 0:
			return f - 1 // the findings
		}
		return cur
	}
	switch dir {
	case "down":
		if cur == f-1 && nb > 0 {
			// Into the canvas, at the box the finding selected.
			if s := lay.boxStop(lay.sel); s >= 0 {
				return s
			}
			return f
		}
		return min(cur+1, len(stops)-1)
	case "up":
		if cur == f+nb && nb > 0 {
			// Out of the detail, back to its box.
			if s := lay.boxStop(lay.sel); s >= 0 {
				return s
			}
			return f + nb - 1
		}
		return max(cur-1, 0)
	}
	return cur
}

// impactSel is the box the stop at cur selects: a box's own, a finding's
// package when it is drawn, else keep (the detail's rows keep their box).
func impactSel(lay *impactLayout, stops []stop, cur int, keep string) string {
	if lay == nil || cur < 0 || cur >= len(stops) {
		return keep
	}
	switch a := stops[cur].act; a.kind {
	case actBox:
		return lay.boxes[a.n].path
	case actFinding:
		if p := lay.res.Findings[a.n].Package; lay.boxStop(p) >= 0 {
			return p
		}
		return ""
	case actFindMore:
		return ""
	}
	return keep
}

// impactKey moves the focused Impact tab's cursor. done is false for any
// other key, or when the tab is not focused.
func (m *Model) impactKey(msg tea.KeyPressMsg) bool {
	if m.mode != modeRow || m.curTab() != tabImpact || !m.dfocus {
		return false
	}
	dir := ""
	switch msg.String() {
	case "j", "down":
		dir = "down"
	case "k", "up":
		dir = "up"
	case "h", "left":
		dir = "left"
	case "l", "right":
		dir = "right"
	default:
		return false
	}
	l := m.layout()
	onBox := l.drawer != nil && m.dcur < len(l.drawer.stops) && l.drawer.stops[m.dcur].act.kind == actBox
	if (dir == "left" || dir == "right") && !onBox {
		return false // h and l mean something else off the canvas
	}
	if l.drawer == nil || l.drawer.impact == nil || len(l.drawer.stops) == 0 {
		switch dir {
		case "down":
			m.scrollDrawer(1)
		case "up":
			m.scrollDrawer(-1)
		}
		return true
	}
	m.dcur = impactStep(l.drawer.impact, l.drawer.stops, m.dcur, dir)
	m.syncImpactSel()
	m.showImpactStop()
	return true
}

// syncImpactSel selects the box under the Impact tab's cursor.
func (m *Model) syncImpactSel() {
	t, _, ok := m.archResult()
	if !ok || m.mode != modeRow || m.curTab() != tabImpact || !m.dfocus {
		return
	}
	l := m.layout()
	if l.drawer == nil {
		return
	}
	if m.iselKey != diffKey(t) {
		m.isel, m.iselKey = "", diffKey(t)
	}
	m.isel = impactSel(l.drawer.impact, l.drawer.stops, m.dcur, m.isel)
}

// showImpactStop scrolls the drawer to the cursor's stop: a box whole when
// it fits, else its top border.
func (m *Model) showImpactStop() {
	m.moveDrawerCursor(0)
	l := m.layout()
	if l.drawer == nil || l.drawer.impact == nil || m.dcur >= len(l.drawer.stops) {
		return
	}
	lay := l.drawer.impact
	if a := l.drawer.stops[m.dcur].act; a.kind == actBox {
		b := lay.boxes[a.n]
		top, bottom := lay.top+b.y, lay.top+b.y+b.h-1
		if bottom >= m.drawerOff+l.drawerH {
			m.drawerOff = min(bottom-l.drawerH+1, top)
		}
		m.scrollDrawer(0)
	}
}

// impactAct does what ↵ or a second click does on an Impact stop: a
// finding or a detail row shows where it is written, a box its first
// changed file, and the ⋯ line shows every finding, or fewer again.
func (m *Model) impactAct(a action) tea.Cmd {
	l := m.layout()
	if l.drawer == nil || l.drawer.impact == nil {
		return nil
	}
	lay := l.drawer.impact
	switch a.kind {
	case actFinding:
		f := lay.res.Findings[a.n]
		if f.Site.File == "" {
			m.status = f.Text + ": no file to show"
			return nil
		}
		return m.openImpactSite(impactSite{file: f.Site.File, line: f.Site.Line, base: f.Site.Base})
	case actFindMore:
		m.iall = !m.iall
		// The cursor stays on the ⋯ line, which moves to the list's end,
		// or, when every finding fits and there is none, on the last one.
		l := m.layout()
		m.dcur = max(l.drawer.impact.findings-1, 0)
		for i, s := range l.drawer.stops {
			if s.act.kind == actFindMore {
				m.dcur = i
			}
		}
		m.syncImpactSel()
		m.moveDrawerCursor(0)
	case actBox:
		b := lay.boxes[a.n]
		pk, _ := lay.res.Package(b.path)
		if len(pk.Changed) == 0 {
			m.status = "no file of " + b.path + " changed"
			return nil
		}
		return m.openImpactSite(impactSite{file: pk.Changed[0], base: pk.Status == arch.Removed})
	case actSite:
		return m.openImpactSite(lay.sites[a.n])
	}
	return nil
}

// openImpactSite shows a file in the diff preview, at the site's line,
// on the Files tab. A file the branch did not change has no diff to show.
func (m *Model) openImpactSite(s impactSite) tea.Cmd {
	t, d, ok := m.diff()
	if !ok || d.Note != "" {
		m.status = "no diff to show " + s.file + " in"
		return nil
	}
	files := fileOrder(d.Files, m.tree)
	at := -1
	for i, f := range files {
		if f.Path == s.file || f.OldPath == s.file {
			at = i
			break
		}
	}
	if at < 0 {
		m.status = s.file + " did not change on this branch"
		return nil
	}
	if m.opt.Patch == nil {
		return m.openFile(t, d, at)
	}
	m.switchTab(tabFiles)
	m.prevGoto = previewGoto{key: patchKey(t, files[at]), line: s.line, base: s.base}
	return m.previewFileAt(at)
}

// previewGoto is a line the preview scrolls to once its file's patch is
// read: a line number in the new file, or in the old one (base).
type previewGoto struct {
	key  string
	line int
	base bool
}

// applyGoto scrolls the preview to the line asked for, once the patch of
// that file is there, with a few lines of context above it.
func (m *Model) applyGoto() {
	g := m.prevGoto
	if g.key == "" || g.key != m.prevKey {
		return
	}
	p, ok := m.patches[g.key]
	if !ok {
		return
	}
	m.prevGoto = previewGoto{}
	if g.line <= 0 || p.patch.Binary {
		return
	}
	nos := p.split.newNo
	if g.base {
		nos = p.split.oldNo
	}
	k := -1
	for i, n := range nos {
		if n >= g.line {
			k = i
			break
		}
	}
	if k < 0 {
		return
	}
	m.prevOff = len(m.intro(p)) + rowOf(m.layoutRows(p), k) - 3
	m.scrollPreview(0)
}

// impactClick handles a click on the Impact tab's drawer line i: on a box
// or a finding, the first click selects it and a second one acts, as ↵
// does. done is false for clicks it leaves to the drawer's zones (the
// detail's rows act at once).
func (m *Model) impactClick(l frame, i, x int) (tea.Cmd, bool) {
	lay := l.drawer.impact
	if lay == nil || m.mode != modeRow || m.curTab() != tabImpact {
		return nil, false
	}
	at, first := impactClickStop(l.drawer, i, x)
	onCanvas := i >= lay.top && i < lay.top+lay.c.g.h && x >= lay.left && x < lay.left+lay.c.g.w
	switch {
	case at < 0:
		return nil, onCanvas // the canvas's background does nothing
	case !first:
		return nil, false
	case m.dfocus && m.dcur == at:
		return m.act(l.drawer.stops[at].act), true
	}
	m.focusDrawer()
	m.choosing = noKind
	m.dcur = at
	m.syncImpactSel()
	return nil, true
}

// openImpactPane opens the selected thread's Impact view in a herdr pane
// of its own, beside the deck and zoomed to the whole tab.
func (m *Model) openImpactPane() tea.Cmd {
	if m.opt.Arch == nil {
		m.status = "the Impact tab is off (arch.enabled)"
		return nil
	}
	t, ok := m.diffThread()
	if !ok {
		m.status = "no worktree on this row"
		return nil
	}
	open := m.opt.OpenImpact
	if open == nil {
		m.status = "the Impact pane needs herdr: run the deck inside herdr"
		return nil
	}
	return func() tea.Msg { return openedMsg{what: t.ID + "'s Impact view in a pane", err: open(t)} }
}

// impactStopAct is what the Impact tab's cursor is on, when it shows.
func (m Model) impactStopAct() (action, bool) {
	if m.mode != modeRow || m.curTab() != tabImpact || !m.dfocus {
		return action{}, false
	}
	l := m.layout()
	if l.drawer == nil || l.drawer.impact == nil || m.dcur >= len(l.drawer.stops) {
		return action{}, false
	}
	return l.drawer.stops[m.dcur].act, true
}

// relocateImpact puts the Impact tab's cursor back where it was after the
// stops changed (a new height shows more findings, a new read has
// others): on the same finding or ⋯ line (prev) when it still shows, else
// on the selected box, else on the first finding.
func (m *Model) relocateImpact(prev action) {
	if m.mode != modeRow || m.curTab() != tabImpact || !m.dfocus {
		return
	}
	l := m.layout()
	if l.drawer == nil || l.drawer.impact == nil {
		return
	}
	m.dcur = 0
	if s := l.drawer.impact.boxStop(m.isel); s >= 0 {
		m.dcur = s
	}
	if prev.kind == actFinding || prev.kind == actFindMore {
		for i, s := range l.drawer.stops {
			if s.act == prev {
				m.dcur = i
			}
		}
	}
	m.syncImpactSel()
	m.moveDrawerCursor(0)
}

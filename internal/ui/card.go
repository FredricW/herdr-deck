package ui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// The drawer's header card and tab bar (docs/design/drawer/README.md).

// rowDrawer is the selected row's drawer: the header card, the tab bar and
// the tab's content, h lines of which show from the drawer's offset.
func (m Model) rowDrawer(r row, links []deck.Link, width, h int) *drawer {
	if r.kind == rowHeading {
		return m.projectDrawer(r, width)
	}
	d := newDrawer(width, "")
	d.light = m.light
	d.card = m.card(r, width)
	tab := m.curTab()
	if m.dfocus {
		d.cur = m.dcur
	}
	t, hasThread := r.thread()
	switch tab {
	case tabFiles:
		m.filesTab(d)
	case tabLog:
		m.logTab(d, t, links)
	default:
		m.overview(d, r, links, h)
	}
	off := clamp(m.drawerOff, 0, max(len(d.lines)-h, 0))
	d.tabs, d.tabZones = m.tabBar(r, hasThread, tab, d.moreHint(tab, off, h), width)
	return d
}

// moreHint says what is below the drawer's end: Overview's section names
// at 80 columns, else how many lines.
func (d *drawer) moreHint(tab tabKind, off, h int) string {
	end := off + h
	below := len(d.lines) - end
	if below <= 0 {
		return ""
	}
	if tab == tabOverview && d.width >= wideMin {
		var names []string
		for _, s := range d.sections {
			if s.at >= end {
				names = append(names, s.name)
			}
		}
		if len(names) > 0 {
			if hint := "↓ " + strings.Join(names, " · "); ansi.StringWidth(hint) <= d.width/3 {
				return hint
			}
		}
	}
	return fmt.Sprintf("↓ %d more", below)
}

// tabStyle is a tab's label style: bold bright white on blue when active,
// plain on the selection's grey otherwise, dim on it without data.
func tabStyle(active, enabled, light bool) lipgloss.Style {
	if active {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.BrightWhite).Background(colBlue)
	}
	bg := selDark
	if light {
		bg = selLight
	}
	st := lipgloss.NewStyle().Background(lipgloss.ANSIColor(bg))
	if !enabled {
		st = st.Faint(true)
	}
	return st
}

// tabBar is the tab bar's line and its clickable tabs: each label with one
// space of padding on its background, one space apart, and the hint at
// the right.
func (m Model) tabBar(r row, hasThread bool, active tabKind, hint string, width int) (string, []zone) {
	var b strings.Builder
	var zones []zone
	x := 0
	for tab := range numTabs {
		enabled := m.tabEnabled(r, tab)
		label := tab.String()
		if enabled {
			switch tab {
			case tabFiles:
				if _, d, ok := m.diff(); !ok {
					label += " …"
				} else if d.Note == "" {
					label += " " + strconv.Itoa(len(d.Files))
				}
			case tabLog:
				if t, ok := r.thread(); ok && hasThread {
					label += " " + strconv.Itoa(len(t.Log))
				}
			}
		}
		text := " " + label + " "
		if tab > 0 {
			b.WriteString(" ")
			x++
		}
		b.WriteString(tabStyle(tab == active, enabled, m.light).Render(text))
		w := ansi.StringWidth(text)
		if enabled {
			zones = append(zones, zone{x0: x, x1: x + w, act: action{kind: actTab, n: int(tab)}})
		}
		x += w
	}
	right := ""
	if hint != "" {
		right = dim.Render(hint) + " "
	}
	return spread(b.String(), right, width), zones
}

// tabEnabled says whether the row has something behind tab: Files needs a
// worktree that is not resolved, Log a thread.
func (m Model) tabEnabled(r row, tab tabKind) bool {
	switch tab {
	case tabFiles:
		_, ok := m.diffThread()
		return ok && m.opt.Diff != nil
	case tabLog:
		_, ok := r.thread()
		return r.kind == rowWork && ok
	}
	return true
}

// curTab is the tab the drawer shows: the chosen one, or Overview while
// the row has nothing behind it.
func (m Model) curTab() tabKind {
	r, ok := m.selected()
	if !ok || r.kind != rowWork || !m.tabEnabled(r, m.tab) {
		return tabOverview
	}
	return m.tab
}

// card is the header card: the title, the status pill and the percent on
// line 1; on line 2, dim, what the row is, with the progress bar or the PR
// at the right.
func (m Model) card(r row, w int) []string {
	narrow := w < wideMin
	t, hasThread := r.thread()
	pill, pst := "○ no thread", dim
	switch {
	case hasThread:
		pill, pst = statusPill(t)
	case r.task != nil && r.task.Done:
		pill = "✓ done"
	}
	right := pst.Render(pill)
	pct, hasPct := percent(t)
	if hasThread && hasPct && t.Status != deck.StatusDone {
		right += "  " + pst.Render(fmt.Sprintf("~%d%%", pct))
	} else {
		hasPct = false
	}
	line1 := spread(" "+bold.Render(r.title()), right+" ", w)

	var parts []string
	var bar string
	switch {
	case hasThread:
		parts = append(parts, t.ID)
		if r.task != nil && t.Title != "" && !narrow {
			parts = append(parts, t.Title)
		}
		if t.Status == deck.StatusWorking && t.Activity != "" {
			parts = append(parts, t.Activity)
		}
		if p := paneID(t); p != "" && !narrow {
			parts = append(parts, "pane "+p)
		}
		switch {
		case hasPct:
			bar = progressBar(pct, pst)
		case t.PR != nil:
			bar = prBrief(*t.PR, narrow)
		}
	case r.task != nil:
		parts = append(parts, r.list, "TASKS.md")
	}
	if r.task != nil && r.task.Owner != "" {
		parts = append(parts, "owner "+r.task.Owner)
	}
	if bar != "" {
		bar += " "
	}
	line2 := spread(" "+dim.Render(strings.Join(parts, " · ")), bar, w)
	return []string{line1, line2}
}

// statusPill is the card's status: the list's glyph and a word, in the
// status colour.
func statusPill(t deck.Thread) (string, lipgloss.Style) {
	switch t.Status {
	case deck.StatusNeedsYou:
		return "● needs you", needsStyle
	case deck.StatusWorking:
		return "◐ working", workStyle
	case deck.StatusReview:
		if t.Group == "landing" {
			return "↻ landing", okStyle
		}
		return "◇ review", reviewStyle
	case deck.StatusDone:
		return "✓ done", dim
	}
	return "○ idle", dim
}

var percentRe = regexp.MustCompile(`~?(\d{1,3})%`)

// percent is the thread's own estimate: its percent field, else the one
// its state line carries ("needs you · ~95%").
func percent(t deck.Thread) (int, bool) {
	if t.Percent != nil {
		return clamp(*t.Percent, 0, 100), true
	}
	if m := percentRe.FindStringSubmatch(t.StateLine); m != nil {
		n, _ := strconv.Atoi(m[1])
		return clamp(n, 0, 100), true
	}
	return 0, false
}

// progressBar is ten cells, ▰ filled in the status colour, ▱ dim.
func progressBar(pct int, st lipgloss.Style) string {
	n := pct / 10
	return st.Render(strings.Repeat("▰", n)) + dim.Render(strings.Repeat("▱", 10-n))
}

// prBrief is the card's PR when there is no percent: `#2320` and what its
// checks say, once herdr-projects knows the PR's state.
func prBrief(pr deck.PullRequest, narrow bool) string {
	s := reviewStyle.Render(fmt.Sprintf("#%d", pr.Number))
	if pr.Number == 0 {
		s = reviewStyle.Render("PR")
	}
	switch n := len(pr.FailingChecks); {
	case pr.State == "":
	case n > 0 && narrow:
		s += " " + failStyle.Render(fmt.Sprintf("✕ %d", n))
	case n > 0:
		s += " " + failStyle.Render(fmt.Sprintf("✕ %d failing", n))
	default:
		s += " " + okStyle.Render("✓")
	}
	return s
}

// paneID is the thread's herdr pane: the live one, else the recorded one.
func paneID(t deck.Thread) string {
	if t.Pane != nil {
		return t.Pane.ID
	}
	return t.PaneID
}

// projectDrawer shows a list heading, or the project when there is no
// row: a card with its name and no tabs, then the list's note, the goal,
// repos and folder.
func (m Model) projectDrawer(r row, width int) *drawer {
	d := newDrawer(width, "")
	name, what := m.snap.Project.Title(), "project"
	if m.snap.Project.Slug != "" {
		what += " " + m.snap.Project.Slug
	}
	if r.kind == rowHeading && r.list != "" {
		name = r.list
		what = fmt.Sprintf("%d rows", r.count)
		if r.count == 1 {
			what = "1 row"
		}
		if r.list == listNeedsYou {
			what += " · threads waiting on you"
		} else {
			what += " · TASKS.md"
		}
	}
	d.card = []string{fit(" "+bold.Render(name), width), fit(" "+dim.Render(what), width)}
	if r.kind == rowHeading {
		for _, tl := range m.snap.TaskLists {
			if tl.Name == r.list && tl.Note != "" {
				var gs []group
				for _, p := range strings.Split(tl.Note, "\n") {
					gs = append(gs, words(p, plain))
				}
				d.field("Note", false, gs...)
			}
		}
		if r.folded {
			d.field("Folded", false, words(fmt.Sprintf("%d rows · space unfolds", r.count), dim))
		}
	}
	p := m.snap.Project
	if p.Goal != "" {
		d.field("Goal", false, words(p.Goal, plain))
	}
	var repos []group
	for _, repo := range p.Repos {
		repos = append(repos, words(tilde(repo), plain))
	}
	d.field("Repos", false, repos...)
	if p.Dir != "" {
		d.field("Folder", false, words(tilde(p.Dir), plain))
	}
	return d
}

// reportDrawer is the thread's report under its card, with a line saying
// how to return where the tab bar was.
func (m Model) reportDrawer(r row, width int) *drawer {
	t, _ := r.thread()
	d := newDrawer(width, "")
	d.card = m.card(r, width)
	d.tabs = fit(" "+bold.Render("Report")+dim.Render(" · threads/"+t.ID+".md · esc returns"), width)
	d.text(strings.TrimSpace(t.Report), plain)
	return d
}

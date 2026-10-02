package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// frame is the pane's layout: where the list and drawer sit, so clicks and
// the wheel can find what is under the pointer.
type frame struct {
	listTop, listH     int // the list's first screen line and height
	drawerTop, drawerH int // the drawer's first content line and height
	drawer             *drawer
	bang               zone // the header's `! N`, if shown
}

func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "herdr-deck · " + m.snap.Project.Title()
	return v
}

// effectiveSize is the drawer's size after the mode's needs: a report takes
// the full height, and a view asked for by key is never hidden.
func (m Model) effectiveSize() drawerSize {
	switch {
	case m.mode == modeReport:
		return sizeFull
	case m.mode != modeRow && m.size == sizeHidden:
		return sizeNormal
	}
	return m.size
}

// layout works out the frame for the current size and state.
func (m Model) layout() frame {
	h := max(m.height, 8)
	f := frame{listTop: 3}
	body := h - 4 // header, rule, footer rule, footer
	switch m.effectiveSize() {
	case sizeHidden:
		f.listH = body - 1 // column titles
	case sizeFull:
		f.listTop, f.listH = 2, 0
		f.drawerTop, f.drawerH = 3, body-1 // drawer title rule
	default:
		total := max(body*2/5, 6)
		f.listH = max(body-1-total, 1)
		f.drawerTop = f.listTop + f.listH + 1
		f.drawerH = body - 1 - f.listH - 1
	}
	if f.drawerH > 0 {
		f.drawer = m.drawerFor(m.width)
	}
	f.bang = m.bangZone()
	return f
}

func (m Model) drawerFor(width int) *drawer {
	r, ok := m.selected()
	switch m.mode {
	case modeSources:
		return m.sourcesDrawer(width)
	case modeHelp:
		return m.helpDrawer(width)
	case modeReport:
		if ok {
			return m.reportDrawer(r, width)
		}
	}
	if !ok {
		return m.projectDrawer(row{}, width)
	}
	return m.rowDrawer(r, rowLinks(r, m.snap), width, m.choosing)
}

func (m Model) render() string {
	w := m.width
	f := m.layout()
	rule := dim.Render(strings.Repeat("─", w))

	lines := []string{m.header(w), rule}
	if f.listH > 0 || m.effectiveSize() != sizeFull {
		lines = append(lines, dim.Render(fit(m.columnTitles(w), w)))
		lines = append(lines, m.listLines(w, f.listH)...)
	}
	if f.drawer != nil {
		lines = append(lines, drawerRule(f.drawer.title, w))
		dl := f.drawer.lines
		off := clamp(m.drawerOff, 0, max(len(dl)-f.drawerH, 0))
		for i := range f.drawerH {
			if j := off + i; j < len(dl) {
				lines = append(lines, dl[j].text)
			} else {
				lines = append(lines, "")
			}
		}
	}
	lines = append(lines, rule, m.footer(w))
	return strings.Join(lines, "\n")
}

func drawerRule(title string, w int) string {
	t := "─ " + ansi.Truncate(title, max(w-4, 1), "…") + " "
	return bold.Render(t) + dim.Render(strings.Repeat("─", max(w-ansi.StringWidth(t), 0)))
}

// header is the project's name and its counts: needs you, working, review,
// idle, inbox and missing sources.
func (m Model) header(w int) string {
	narrow := w < wideMin
	var n [5]int
	for _, t := range m.snap.Threads {
		n[t.Status]++
	}
	var parts []string
	if c := n[deck.StatusNeedsYou]; c > 0 {
		s := fmt.Sprintf("● %d", c)
		if !narrow {
			s += " needs you"
		}
		parts = append(parts, needsStyle.Render(s))
	}
	for _, st := range []deck.ThreadStatus{deck.StatusWorking, deck.StatusReview, deck.StatusUnknown} {
		if c := n[st]; c > 0 {
			parts = append(parts, statusStyle(st).Render(fmt.Sprintf("%s %d", statusGlyph(st), c)))
		}
	}
	if len(parts) == 0 {
		parts = append(parts, dim.Render("nothing running"))
	}
	if c := len(m.snap.Inbox); c > 0 || len(parts) > 1 || n[deck.StatusNeedsYou]+n[deck.StatusWorking]+n[deck.StatusReview]+n[deck.StatusUnknown] > 0 {
		st := dim
		if c > 0 {
			st = inboxStyle
		}
		parts = append(parts, st.Render(fmt.Sprintf("✉ %d", c)))
	}
	if c := len(m.snap.Missing); c > 0 {
		parts = append(parts, warnStyle.Bold(true).Render(fmt.Sprintf("! %d", c)))
	}
	return spread(" "+bold.Render(m.snap.Project.Title()), strings.Join(parts, "  ")+" ", w)
}

// bangZone is where the header's `! N` sits, for clicks.
func (m Model) bangZone() zone {
	c := len(m.snap.Missing)
	if c == 0 {
		return zone{x0: -1, x1: -1}
	}
	wd := ansi.StringWidth(fmt.Sprintf("! %d", c))
	x1 := m.width - 1
	return zone{x0: x1 - wd, x1: x1}
}

// cols are the list's column widths.
type cols struct {
	wide                                 bool
	work, thread, status, pr, links, dev int
}

func columns(w int) cols {
	if w >= wideMin {
		c := cols{wide: true, thread: 6, status: 18, pr: 6, links: 6, dev: 4}
		c.work = w - 35 - c.status
		return c
	}
	c := cols{status: 15, links: 6, dev: 4}
	c.work = w - 19 - c.status
	if c.work < 12 {
		c.status = max(c.status-(12-c.work), 6)
		c.work = max(w-19-c.status, 6)
	}
	return c
}

func (m Model) columnTitles(w int) string {
	c := columns(w)
	s := "    " + pad("WORK", c.work) + "  "
	if c.wide {
		s += pad("THREAD", c.thread) + "  "
	}
	s += pad("STATUS", c.status) + "  "
	if c.wide {
		s += pad("PR", c.pr) + "  "
	}
	return s + pad("LINKS", c.links) + " " + "DEV"
}

func (m Model) listLines(w, h int) []string {
	var lines []string
	switch {
	case !m.loaded:
		lines = append(lines, dim.Render("  Reading "+m.snap.Project.Title()+"…"))
	case len(m.rows) == 0:
		msg := ansi.Wrap("Nothing yet. Rows appear when the coordinator writes TASKS.md or starts a thread.", max(w-4, 10), "")
		lines = append(lines, "")
		for _, l := range strings.Split(msg, "\n") {
			lines = append(lines, dim.Render("  "+l))
		}
	default:
		end := min(m.listOff+h, len(m.rows))
		firstList := true
		for i := m.listOff; i < end; i++ {
			r := m.rows[i]
			asOf := ""
			if r.kind == rowHeading && r.list != listNeedsYou && firstList {
				firstList = false
				if !m.snap.ThreadsAsOf.IsZero() {
					asOf = "as of " + m.snap.ThreadsAsOf.In(m.loc()).Format("15:04")
				}
			}
			lines = append(lines, m.rowLine(r, i == m.cursor, w, asOf))
		}
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return lines[:h]
}

// cell is one column's text and style.
type cell struct {
	text  string
	style lipgloss.Style
	width int
}

func (m Model) rowLine(r row, sel bool, w int, asOf string) string {
	mark := " "
	if sel {
		mark = "▸"
	}
	var cells []cell
	switch r.kind {
	case rowHeading:
		text := r.list
		st := dim
		if r.list == listNeedsYou {
			st = needsStyle
		}
		if r.folded {
			text = fmt.Sprintf("+ %s (%d)", r.list, r.count)
		}
		cells = append(cells, cell{text: " " + mark + text, style: st})
		for _, g := range r.glyphs {
			cells = append(cells, cell{text: " " + statusGlyph(g), style: statusStyle(g)})
		}
		left := renderCells(cells)
		if asOf != "" {
			return m.finish(spread(left, dim.Render(asOf)+" ", w), sel, w)
		}
		return m.finish(left, sel, w)
	case rowInbox:
		age := ""
		if !r.inbox.Created.IsZero() {
			age = ageText(m.opt.Now().Sub(r.inbox.Created))
		}
		prefix := renderCells([]cell{{text: " " + mark}, {text: "✉", style: inboxStyle}, {text: " "}})
		text := renderCells([]cell{{text: ansi.Truncate(r.title(), max(w-4-8, 4), "…"), style: inboxStyle}})
		right := renderCells([]cell{{text: age + "    ", style: dim}})
		return m.finish(spread(prefix+text, right, w), sel, w)
	}

	c := columns(w)
	t, hasThread := r.thread()
	glyph, gst := "·", dim
	switch {
	case r.task != nil && r.task.Done:
		glyph = "✓"
	case hasThread:
		glyph, gst = statusGlyph(t.Status), statusStyle(t.Status)
	}
	stale := !m.snap.ThreadsAsOf.IsZero() && hasThread
	workSt := plain
	if r.needsYou() {
		workSt = needsStyle
	} else if (r.task != nil && r.task.Done) || (hasThread && t.Status == deck.StatusDone) || stale {
		workSt = dim
	}
	cells = append(cells,
		cell{text: " " + mark},
		cell{text: glyph, style: gst},
		cell{text: " "},
		cell{text: r.title(), style: workSt, width: c.work},
		cell{text: "  "},
	)
	if c.wide {
		id := ""
		if hasThread {
			id = t.ID
		}
		cells = append(cells, cell{text: id, style: dim, width: c.thread}, cell{text: "  "})
	}
	status, sst := "", dim
	if hasThread {
		status, sst = rowStatus(t, !c.wide), statusStyle(t.Status)
		if stale {
			sst = dim
		}
	} else if r.task != nil && r.task.Done {
		status = "done"
	}
	cells = append(cells, cell{text: status, style: sst, width: c.status}, cell{text: "  "})
	if c.wide {
		pr, pst := "", reviewStyle
		if hasThread && t.PR != nil && t.PR.Number > 0 {
			pr = fmt.Sprintf("#%d", t.PR.Number)
			if len(t.PR.FailingChecks) > 0 {
				pr += "✕"
				pst = failStyle
			}
		}
		cells = append(cells, cell{text: pr, style: pst, width: c.pr}, cell{text: "  "})
	}
	cells = append(cells, badgeCells(rowLinks(r, m.snap), c.links)...)
	cells = append(cells, cell{text: " "})
	cells = append(cells, devCells(r, c.dev)...)
	return m.finish(renderCells(cells), sel, w)
}

// rowStatus is the STATUS column. At 60 columns it also carries the PR.
func rowStatus(t deck.Thread, narrow bool) string {
	var s string
	switch t.Status {
	case deck.StatusNeedsYou:
		s = "needs you"
	case deck.StatusWorking:
		s = t.Activity
		if s == "" {
			s = t.StateLine
		}
	case deck.StatusReview:
		if t.PR != nil {
			s = reviewText(*t.PR)
		}
		if s == "" {
			s = t.StateLine
		}
	case deck.StatusDone:
		s = "resolved"
	default:
		s = t.StateLine
	}
	if s == "" {
		s = statusWord(t.Status)
	}
	if narrow && t.PR != nil && t.PR.Number > 0 {
		short := s
		if t.Status == deck.StatusReview {
			short = shortReview(*t.PR)
		}
		s = fmt.Sprintf("#%d %s", t.PR.Number, short)
	}
	return s
}

// reviewText is a PR's state in words: "review required", "approved", ….
func reviewText(pr deck.PullRequest) string {
	switch pr.State {
	case "MERGED":
		return "merged"
	case "CLOSED":
		return "closed"
	}
	if pr.Review != "" {
		return strings.ReplaceAll(strings.ToLower(pr.Review), "_", " ")
	}
	return strings.ToLower(pr.State)
}

func shortReview(pr deck.PullRequest) string {
	switch {
	case pr.State == "MERGED":
		return "merged"
	case pr.Review == "APPROVED":
		return "approved"
	case pr.Review == "CHANGES_REQUESTED":
		return "changes"
	}
	return "review"
}

// badgeCells is the LINKS column: a letter per link kind, with a count when
// there are several (`L4 F3`). The PR has its own column, and localhost
// links show as DEV dots.
func badgeCells(links []deck.Link, width int) []cell {
	var n [5]int
	for _, l := range links {
		if l.Kind >= 0 && int(l.Kind) < len(n) {
			n[l.Kind]++
		}
	}
	var cells []cell
	used := 0
	for _, k := range []deck.LinkKind{deck.LinkLinear, deck.LinkFigma, deck.LinkNotion} {
		if n[k] == 0 {
			continue
		}
		text := k.Badge()
		if n[k] > 1 {
			text += fmt.Sprint(n[k])
		}
		if used > 0 {
			text = " " + text
		}
		if used+ansi.StringWidth(text) > width {
			break
		}
		cells = append(cells, cell{text: text, style: linkStyle(k)})
		used += ansi.StringWidth(text)
	}
	cells = append(cells, cell{text: strings.Repeat(" ", width-used)})
	return cells
}

// devCells is the DEV column: one dot per dev server port, green when it
// listens. Milestone 6 fills in the servers.
func devCells(r row, width int) []cell {
	var cells []cell
	used := 0
	for _, t := range r.threads {
		for _, s := range t.DevServers {
			if used >= width {
				break
			}
			if s.Running {
				cells = append(cells, cell{text: "●", style: okStyle})
			} else {
				cells = append(cells, cell{text: "○", style: dim})
			}
			used++
		}
	}
	return cells
}

func renderCells(cells []cell) string {
	var b strings.Builder
	for _, c := range cells {
		text := c.text
		if c.width > 0 {
			text = pad(text, c.width)
		}
		b.WriteString(c.style.Render(text))
	}
	return b.String()
}

// finish fits a row to the width; a selected row is highlighted across the
// whole width.
func (m Model) finish(s string, sel bool, w int) string {
	s = fit(s, w)
	if sel {
		return highlight(s, m.light)
	}
	return s
}

func (m Model) footer(w int) string {
	if m.status != "" {
		return warnStyle.Render(fit(" "+m.status, w))
	}
	narrow := w < wideMin
	var hint string
	r, ok := m.selected()
	switch {
	case m.choosing != noKind:
		var nums []int
		for i, l := range m.links() {
			if l.Kind == m.choosing && i < 9 {
				nums = append(nums, i+1)
			}
		}
		span := ""
		if len(nums) > 0 {
			span = fmt.Sprint(nums[0])
			if len(nums) > 1 {
				span += fmt.Sprintf("-%d", nums[len(nums)-1])
			}
		}
		if m.desktop {
			hint = fmt.Sprintf("Figma desktop: %s open  f first  esc cancel", span)
		} else {
			hint = fmt.Sprintf("%s: %s open", m.choosing, span)
			if m.choosing == deck.LinkFigma {
				hint += "  d desktop"
			}
			hint += "  a all  " + kindKey(m.choosing) + " first  esc cancel"
		}
	case m.mode != modeRow:
		hint = "esc back  pgup pgdn scroll  z drawer  ? help  q quit"
	case !ok:
		hint = "? help  q quit"
	case r.kind == rowHeading:
		hint = "space fold  j k move  z drawer  ? help"
	case r.kind == rowInbox:
		hint = "↵ go to " + r.inbox.Thread + "  1-9 link  z drawer  ? help"
	case r.needsYou():
		hint = "↵ go to pane  1-9 link  e edit  r report  z drawer  ? help"
		if narrow {
			hint = "↵ go to pane  1-9 link  r report  z drawer  ? help"
		}
	default:
		hint = "1-9 link  l f n g o first  ↵ pane  e edit  z drawer  ? help"
		if narrow {
			hint = "1-9 link  l f n g o first  ↵ pane  z drawer  ? help"
		}
	}
	return fit(" "+hint, w)
}

// spread puts left and right on one line of width w, right-aligning right and
// truncating left when they do not fit.
func spread(left, right string, w int) string {
	rw := ansi.StringWidth(right)
	room := w - rw - 1
	if right == "" {
		room = w
	}
	if room < 1 {
		return fit(left, w)
	}
	left = ansi.Truncate(left, room, "…")
	gap := w - ansi.StringWidth(left) - rw
	return left + strings.Repeat(" ", max(gap, 0)) + right
}

// fit truncates or pads s to exactly w columns.
func fit(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(w-ansi.StringWidth(s), 0))
}

// pad truncates or pads plain text to n columns.
func pad(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return fit(s, n)
}

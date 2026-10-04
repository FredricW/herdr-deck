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
	// sepY is the rule above the drawer (its title rule in a full view),
	// or -1; headTop the card's first line and headN how many of the
	// card's and tab bar's lines show; tabY the tab bar's line, or -1.
	sepY, headTop, headN, tabY int
	bang                       zone // the header's `! N`, if shown
	// attn is the screen line saying other projects need the user, or 0.
	attn int
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
	case m.mode == modeReport || m.mode == modeSettings:
		return sizeFull
	case m.mode != modeRow && m.size == sizeHidden:
		return sizeNormal
	}
	return m.size
}

// headSize is how many lines the drawer draws above its scrolling
// content: a row's card and tab bar (or the report's line), a list
// heading's card, or none for a full view under its title rule.
func (m Model) headSize() int {
	r, ok := m.selected()
	switch {
	case m.mode == modeReport && ok:
		return 3
	case m.mode != modeRow && m.mode != modeReport:
		return 0
	case !ok || r.kind == rowHeading:
		return 2
	}
	return 3
}

// layout works out the frame for the current size and state. The drawer
// takes half of the pane below the header at its normal size: the rule
// above it, the card, the tab bar and the tab's content.
func (m Model) layout() frame {
	h := max(m.height, 8)
	f := frame{listTop: 3, sepY: -1, tabY: -1}
	body := h - 4 // header, rule, footer rule, footer
	head := m.headSize()
	switch m.effectiveSize() {
	case sizeHidden:
		f.listH = body - 1 // column titles
	case sizeFull:
		f.listTop, f.listH = 2, 0
		if head == 0 {
			f.sepY, f.headTop = 2, 3
		} else {
			f.headTop = 2
		}
		f.headN = head
		f.drawerTop = f.headTop + head
		f.drawerH = h - 2 - f.drawerTop
	default:
		block := max(body/2, 6)
		f.listH = max(body-1-block, 1)
		block = body - 1 - f.listH
		f.sepY = f.listTop + f.listH
		f.headTop = f.sepY + 1
		f.headN = min(head, max(block-2, 0))
		f.drawerTop = f.headTop + f.headN
		f.drawerH = block - 1 - f.headN
	}
	if head == 3 && f.headN == 3 && m.mode == modeRow {
		f.tabY = f.headTop + 2
	}
	// Other projects' needs take the list's last line while it has two.
	if f.listH >= 2 && m.otherNeeds() > 0 {
		f.listH--
		f.attn = f.listTop + f.listH
	}
	if f.drawerH > 0 {
		f.drawer = m.drawerFor(m.width, f.drawerH)
	}
	f.bang = m.bangZone()
	return f
}

func (m Model) drawerFor(width, h int) *drawer {
	r, ok := m.selected()
	switch m.mode {
	case modeSources:
		return m.sourcesDrawer(width)
	case modeHelp:
		return m.helpDrawer(width)
	case modeSettings:
		return m.settingsDrawer(width)
	case modeNews:
		return m.newsDrawer(width)
	case modeReport:
		if ok {
			return m.reportDrawer(r, width)
		}
	}
	if !ok {
		return m.projectDrawer(row{}, width)
	}
	return m.rowDrawer(r, rowLinks(r), width, h)
}

func (m Model) render() string {
	w := m.width
	f := m.layout()
	rule := dim.Render(strings.Repeat("─", w))

	lines := []string{m.header(w), rule}
	if m.pick.open {
		lines = append(lines, m.pickerLines(w, m.pickH())...)
		lines = append(lines, rule, m.footer(w))
		return strings.Join(lines, "\n")
	}
	switch {
	case m.preview:
		lines = append(lines, m.previewLines(w, f.listH)...)
	case f.listH > 0 || m.effectiveSize() != sizeFull:
		lines = append(lines, dim.Render(fit(m.columnTitles(w), w)))
		lines = append(lines, m.listLines(w, f.listH)...)
	}
	if f.attn > 0 {
		lines = append(lines, attentionLine(m.otherNeeds(), w))
	}
	if f.drawer != nil {
		switch {
		case f.drawer.title != "":
			lines = append(lines, drawerRule(f.drawer.title, w))
		case f.sepY >= 0:
			lines = append(lines, rule)
		}
		lines = append(lines, f.drawer.head()[:f.headN]...)
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
	if v := m.update.Available; v != "" {
		parts = append(parts, okStyle.Render("↑ "+v))
	}
	if m.opt.RestartFailed != "" {
		s := "↻"
		if !narrow {
			s += " restart failed"
		}
		parts = append(parts, warnStyle.Render(s))
	}
	// `! N` stays last: bangZone finds it at the right edge.
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
	if r.kind == rowGap {
		return ""
	}
	mark := " "
	markSt := plain
	if sel {
		mark = "▸"
		if m.dfocus {
			markSt = dim // the drawer has the focus
		}
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
		cells = append(cells, cell{text: " "}, cell{text: mark, style: markSt}, cell{text: text, style: st})
		for _, g := range r.glyphs {
			cells = append(cells, cell{text: " " + statusGlyph(g), style: statusStyle(g)})
		}
		if r.folded && r.updates > 0 {
			cells = append(cells, cell{text: " ✉", style: inboxStyle})
		}
		left := renderCells(cells)
		if asOf != "" {
			return m.finish(spread(left, dim.Render(asOf)+" ", w), sel, w)
		}
		return m.finish(left, sel, w)
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
	} else if r.updates > 0 {
		// Unhandled inbox items about the row's threads: news to read in
		// its Log, not a need.
		workSt = inboxStyle
	} else if (r.task != nil && r.task.Done) || (hasThread && t.Status == deck.StatusDone) || stale {
		workSt = dim
	}
	cells = append(cells,
		cell{text: " "},
		cell{text: mark, style: markSt},
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
	cells = append(cells, badgeCells(rowLinks(r), c.links)...)
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
// listens; a fallback port (herdr's port token) is marked ~.
func devCells(r row, width int) []cell {
	var cells []cell
	used := 0
	for _, t := range r.threads {
		for _, s := range t.DevServers {
			if s.Fallback && used+2 <= width {
				cells = append(cells, cell{text: "~", style: dim})
				used++
			}
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
	if m.notice != "" {
		return footerLine(okStyle.Render(" "+m.notice), m.opt.Version, w)
	}
	narrow := w < wideMin
	var hint string
	r, ok := m.selected()
	switch {
	case m.pick.open:
		hint = m.pickerFooter()
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
	case m.mode == modeSettings && m.set.editing:
		hint = "↵ save  esc cancel  empty removes it"
	case m.mode == modeSettings:
		hint = "↵ edit  x remove  j k move  esc back  q quit"
	case m.mode == modeReport:
		hint = "esc back  pgup pgdn scroll  ? help  q quit"
	case m.mode != modeRow:
		hint = "esc back  pgup pgdn scroll  z drawer  ? help  q quit"
	case !ok:
		hint = "? help  q quit"
	case r.kind == rowHeading:
		hint = "space fold  j k move  z drawer  ? help"
	default:
		hint = m.rowHint(r, narrow)
	}
	return footerLine(" "+hint, m.opt.Version, w)
}

// footerLine right-aligns the dim version after the key help. The help
// takes priority: the version is dropped when both do not fit with a
// two-column gap.
func footerLine(help, version string, w int) string {
	if version == "" || ansi.StringWidth(help)+2+ansi.StringWidth(version) > w {
		return fit(help, w)
	}
	return spread(help, dim.Render(version), w)
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

// rowHint is the footer's key help for a work row, by the drawer's tab and
// focus.
func (m Model) rowHint(r row, narrow bool) string {
	t, hasThread := r.thread()
	tab := m.curTab()
	layout := "S split"
	if m.split {
		layout = "S unified"
	}
	if m.dfocus {
		switch tab {
		case tabFiles:
			view := "t tree"
			if m.tree {
				view = "t list"
			}
			switch {
			case m.preview && narrow:
				return "j k file  J K scroll  v off  d tool  esc list"
			case m.preview:
				return "j k file  J K scroll  " + layout + "  v off  d diff tool  a all  esc list"
			case narrow:
				return "j k file  ↵ preview  d tool  a all  " + view + "  esc list"
			}
			return "j k file  ↵ preview  d diff tool  a whole diff  " + view + "  esc list  ? help"
		case tabCommits:
			gh := ""
			if hasThread && t.PR != nil {
				gh = "  g GitHub"
			}
			switch {
			case m.preview && narrow:
				return "j k move  J K scroll  v off  esc list"
			case m.preview && gh == "":
				return "j k move  J K scroll  " + layout + "  v off  space files  d diff tool  esc list"
			case m.preview:
				return "j k move  J K scroll  " + layout + "  v off  d diff tool" + gh + "  esc list"
			case narrow:
				return "j k move  ↵ preview  space files  d tool  esc list"
			}
			return "j k move  ↵ preview  space files  d diff tool" + gh + "  esc list  ? help"
		case tabLog:
			return "j k event  ↵ act  tab next tab  esc list  ? help"
		}
		return "j k link  ↵ open  tab next tab  esc list  ? help"
	}
	switch {
	case tab == tabFiles:
		view := "t tree"
		if m.tree {
			view = "t list"
		}
		if narrow {
			return "1-9 preview  a all  " + view + "  tab focus  [ ] tab"
		}
		return "1-9 preview file  a whole diff  " + view + "  tab focus  [ ] tab  ? help"
	case tab == tabCommits && narrow:
		return "1-9 commit  v preview  tab focus  [ ] tab"
	case tab == tabCommits:
		return "1-9 preview commit  v preview  tab focus  [ ] tab  ? help"
	case tab == tabLog && hasThread && t.PR != nil:
		return "1-9 link  g PR  r report  [ ] tab  z drawer  ? help"
	case tab == tabLog:
		return "r report  ↵ go to pane  [ ] tab  z drawer  ? help"
	case !hasThread:
		return "1-9 link  l f n g first  [ ] tab  z drawer  ? help"
	case r.needsYou() && narrow:
		return "↵ go to pane  1-9 link  [ ] tab  z drawer  ? help"
	case r.needsYou():
		return "↵ go to pane  1-9 link  [ ] tab  r report  z drawer  ? help"
	case narrow:
		return "1-9 link  o open  ↵ pane  [ ] tab  z drawer  ? help"
	}
	return "1-9 link  l f n g o first  ↵ pane  [ ] tab  z drawer  ? help"
}

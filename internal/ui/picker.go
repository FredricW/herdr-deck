package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// picker is the project picker (p): every project under the projects root
// with what needs the user in it, drawn over the deck until esc.
type picker struct {
	open     bool
	filter   string
	sel      string // the selected row's key
	off      int    // the first row shown
	archived bool   // archived projects are listed
}

// pickKind says what a picker row shows.
type pickKind int

const (
	pickProject pickKind = iota
	pickThread           // a thread waiting on the user
	pickInbox            // an unhandled inbox item
	pickNote             // a dim line that cannot be selected
)

type pickRow struct {
	kind    pickKind
	key     string
	project deck.ProjectInfo
	thread  deck.Thread
	inbox   deck.InboxItem
	note    string
}

func (r pickRow) selectable() bool { return r.kind != pickNote }

// pickRows lists the projects in the picker's order: those that need the
// user first, then the rest, each by name; archived ones last, and only
// after tab. Under each project come the threads waiting on the user and
// its unhandled inbox items.
func (m Model) pickRows() []pickRow {
	if m.snap.Projects == nil {
		return []pickRow{{kind: pickNote, note: "Projects are not read yet."}}
	}
	filter := strings.ToLower(strings.TrimSpace(m.pick.filter))
	var shown []deck.ProjectInfo
	hidden := 0
	for _, p := range m.snap.Projects {
		if filter != "" && !strings.Contains(strings.ToLower(p.Title()), filter) && !strings.Contains(p.Slug, filter) {
			continue
		}
		if p.Archived() && !m.pick.archived {
			hidden++
			continue
		}
		shown = append(shown, p)
	}
	sort.SliceStable(shown, func(i, j int) bool {
		a, b := shown[i], shown[j]
		if a.Archived() != b.Archived() {
			return b.Archived()
		}
		if na, nb := a.NeedsYou() && !a.Archived(), b.NeedsYou() && !b.Archived(); na != nb {
			return na
		}
		return strings.ToLower(a.Title()) < strings.ToLower(b.Title())
	})
	var rows []pickRow
	for _, p := range shown {
		rows = append(rows, pickRow{kind: pickProject, key: "project:" + p.Slug, project: p})
		if p.Archived() {
			continue
		}
		threads, inbox := p.Needs()
		for _, t := range threads {
			rows = append(rows, pickRow{kind: pickThread, key: "thread:" + p.Slug + "/" + t.ID, project: p, thread: t})
		}
		for _, it := range inbox {
			rows = append(rows, pickRow{kind: pickInbox, key: "inbox:" + p.Slug + "/" + it.ID, project: p, inbox: it})
		}
	}
	switch {
	case hidden > 0:
		word := "projects"
		if hidden == 1 {
			word = "project"
		}
		rows = append(rows, pickRow{kind: pickNote, note: fmt.Sprintf("+ %d archived %s · tab shows", hidden, word)})
	case len(rows) == 0 && filter != "":
		rows = append(rows, pickRow{kind: pickNote, note: "No project matches “" + m.pick.filter + "”."})
	case len(rows) == 0:
		rows = append(rows, pickRow{kind: pickNote, note: "No projects under the projects root."})
	}
	return rows
}

// pickCursor is the selected row's index: the row with the remembered key,
// else the first that can be selected; -1 when none can.
func pickCursor(rows []pickRow, sel string) int {
	first := -1
	for i, r := range rows {
		if !r.selectable() {
			continue
		}
		if r.key == sel {
			return i
		}
		if first < 0 {
			first = i
		}
	}
	return first
}

// otherNeeds counts the projects other than this one, archived ones aside,
// where something waits on the user.
func (m Model) otherNeeds() int {
	n := 0
	for _, p := range m.snap.Projects {
		if p.Slug != m.snap.Project.Slug && !p.Archived() && p.NeedsYou() {
			n++
		}
	}
	return n
}

// attentionLine is the line under the list that says other projects need
// the user.
func attentionLine(n, w int) string {
	text := fmt.Sprintf("● %d other projects need you", n)
	if n == 1 {
		text = "● 1 other project needs you"
	}
	return fit(" "+needsStyle.Render(text)+dim.Render("  p to see"), w)
}

func (m *Model) openPicker() {
	m.pick = picker{open: true}
	m.choosing, m.files = noKind, false
	m.pickVisible()
}

// pickH is how many picker rows fit: the pane less the header, its rule,
// the prompt line, the footer rule and the footer.
func (m Model) pickH() int { return max(max(m.height, 8)-5, 1) }

// pickTop is the screen line of the picker's first row.
const pickTop = 3

// pickVisible scrolls the picker so the selected row shows.
func (m *Model) pickVisible() {
	rows := m.pickRows()
	i := pickCursor(rows, m.pick.sel)
	if i >= 0 {
		m.pick.sel = rows[i].key
	}
	h := m.pickH()
	if i >= 0 && i < m.pick.off {
		m.pick.off = i
	}
	if i >= m.pick.off+h {
		m.pick.off = i - h + 1
	}
	m.pick.off = clamp(m.pick.off, 0, max(len(rows)-h, 0))
}

// pickMove moves the picker's selection by delta selectable rows.
func (m *Model) pickMove(delta int) {
	rows := m.pickRows()
	i := pickCursor(rows, m.pick.sel)
	if i < 0 {
		return
	}
	dir := 1
	if delta < 0 {
		dir, delta = -1, -delta
	}
	for ; delta > 0; delta-- {
		j := i + dir
		for j >= 0 && j < len(rows) && !rows[j].selectable() {
			j += dir
		}
		if j < 0 || j >= len(rows) {
			break
		}
		i = j
	}
	m.pick.sel = rows[i].key
	m.pickVisible()
}

// pickerKey handles every key while the picker is open: printable keys
// type into the filter, so moving takes the arrows.
func (m *Model) pickerKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		if m.pick.filter != "" {
			m.pick.filter = ""
			m.pickVisible()
		} else {
			m.pick.open = false
		}
		return nil
	case "ctrl+c":
		return tea.Quit
	case "enter":
		return m.pickActivate()
	case "up", "ctrl+p", "ctrl+k":
		m.pickMove(-1)
	case "down", "ctrl+n", "ctrl+j":
		m.pickMove(1)
	case "pgup", "ctrl+u":
		m.pickMove(-max(m.pickH()/2, 1))
	case "pgdown", "ctrl+d":
		m.pickMove(max(m.pickH()/2, 1))
	case "tab":
		m.pick.archived = !m.pick.archived
		m.pickVisible()
	case "backspace":
		if r := []rune(m.pick.filter); len(r) > 0 {
			m.pick.filter = string(r[:len(r)-1])
			m.pick.sel = ""
			m.pickVisible()
		}
	default:
		if t := msg.Text; t != "" && !strings.ContainsFunc(t, unicode.IsControl) {
			m.pick.filter += t
			m.pick.sel, m.pick.off = "", 0
			m.pickVisible()
		}
	}
	return nil
}

// pickActivate goes where the selected row points: a project's coordinator
// (started with herdr-projects open when none runs), or the pane of a
// thread or inbox item. The picker closes when something happens.
func (m *Model) pickActivate() tea.Cmd {
	rows := m.pickRows()
	i := pickCursor(rows, m.pick.sel)
	if i < 0 {
		return nil
	}
	r := rows[i]
	p := r.project
	switch r.kind {
	case pickProject:
		if p.Archived() {
			m.status = p.Title() + " is archived: unarchive it with herdr-projects first"
			return nil
		}
		if p.PaneID != "" {
			return m.pickFocus(p.PaneID, p.Title()+"'s coordinator")
		}
		start := m.opt.OpenProject
		if start == nil {
			m.status = "starting coordinators is off"
			return nil
		}
		m.pick.open = false
		m.status = "starting " + p.Title() + "'s coordinator…"
		slug, what := p.Slug, p.Title()+"'s coordinator"
		return func() tea.Msg { return openedMsg{what: what, err: start(slug), verb: "start"} }
	case pickThread:
		return m.pickThreadPane(p, r.thread)
	case pickInbox:
		for _, t := range p.Threads {
			if t.ID == r.inbox.Thread {
				return m.pickThreadPane(p, t)
			}
		}
		if p.PaneID != "" {
			return m.pickFocus(p.PaneID, p.Title()+"'s coordinator")
		}
		m.status = "no pane for this item: ↵ on " + p.Title() + " opens its coordinator"
	}
	return nil
}

func (m *Model) pickThreadPane(p deck.ProjectInfo, t deck.Thread) tea.Cmd {
	switch {
	case t.Pane != nil:
		return m.pickFocus(t.Pane.ID, p.Title()+" "+t.ID+"'s pane")
	case !m.snap.Herdr && t.PaneID != "":
		// herdr's live state is unknown: try the recorded pane.
		return m.pickFocus(t.PaneID, p.Title()+" "+t.ID+"'s pane")
	}
	m.status = p.Title() + " " + t.ID + " has no open pane"
	return nil
}

// pickFocus focuses a pane and closes the picker; herdr shows the pane's
// workspace and tab with it.
func (m *Model) pickFocus(id, what string) tea.Cmd {
	focus := m.opt.FocusPane
	if focus == nil {
		m.status = "focusing panes is off: " + id
		return nil
	}
	m.pick.open = false
	return func() tea.Msg { return openedMsg{what: what, err: focus(id), verb: "focus"} }
}

// pickClick selects the picker row under the pointer and goes there.
func (m *Model) pickClick(y int) tea.Cmd {
	rows := m.pickRows()
	i := m.pick.off + y - pickTop
	if y < pickTop || i < 0 || i >= len(rows) || !rows[i].selectable() {
		return nil
	}
	m.pick.sel = rows[i].key
	return m.pickActivate()
}

// pickerLines are the prompt line and the picker's rows, h of them.
func (m Model) pickerLines(w, h int) []string {
	prompt := " " + bold.Render("Projects") + dim.Render(" › ")
	if m.pick.filter == "" {
		prompt += dim.Render("type to filter")
	} else {
		prompt += m.pick.filter + "_"
	}
	lines := []string{fit(prompt, w)}
	rows := m.pickRows()
	sel := pickCursor(rows, m.pick.sel)
	off := clamp(m.pick.off, 0, max(len(rows)-h, 0))
	for i := off; i < min(off+h, len(rows)); i++ {
		lines = append(lines, m.pickLine(rows[i], i == sel, w))
	}
	for len(lines) < h+1 {
		lines = append(lines, "")
	}
	return lines
}

func (m Model) pickLine(r pickRow, sel bool, w int) string {
	mark := " "
	if sel {
		mark = "▸"
	}
	switch r.kind {
	case pickNote:
		return fit("   "+dim.Render(r.note), w)
	case pickThread, pickInbox:
		var at time.Time
		var glyph, text string
		st := needsStyle
		if r.kind == pickThread {
			glyph, text, at = "●", r.thread.Title, needSince(r.thread)
			if text == "" {
				text = r.thread.ID
			}
			text += dim.Render(" · " + r.thread.ID)
		} else {
			glyph, text, at, st = "✉", "inbox: "+inboxText(r.inbox), r.inbox.Created, inboxStyle
		}
		age := ""
		if !at.IsZero() {
			age = ageText(m.opt.Now().Sub(at))
		}
		left := " " + mark + "    " + st.Render(glyph) + " " + st.Render(text)
		return m.finish(spread(left, dim.Render(age)+"  ", w), sel, w)
	}
	p := r.project
	name := bold
	var tags []string
	if p.Slug == m.snap.Project.Slug {
		tags = append(tags, "here")
	}
	if p.Status != "" && p.Status != "active" {
		name = dim
		tags = append(tags, p.Status)
	}
	left := " " + mark + " " + name.Render(p.Title())
	if len(tags) > 0 {
		left += "  " + dim.Render(strings.Join(tags, " · "))
	}
	return m.finish(spread(left, m.pickCounts(p)+"  ", w), sel, w)
}

// pickCounts is a project's short summary: its open threads by status and
// its inbox, as the header shows them.
func (m Model) pickCounts(p deck.ProjectInfo) string {
	var n [5]int
	for _, t := range p.Threads {
		n[t.Status]++
	}
	var parts []string
	for _, st := range []deck.ThreadStatus{deck.StatusNeedsYou, deck.StatusWorking, deck.StatusReview, deck.StatusUnknown} {
		if c := n[st]; c > 0 {
			style := statusStyle(st)
			if p.Archived() {
				style = dim
			}
			parts = append(parts, style.Render(fmt.Sprintf("%s %d", statusGlyph(st), c)))
		}
	}
	if c := len(p.Inbox); c > 0 && !p.Archived() {
		parts = append(parts, inboxStyle.Render(fmt.Sprintf("✉ %d", c)))
	}
	if len(parts) == 0 {
		if done := n[deck.StatusDone]; done > 0 {
			parts = append(parts, dim.Render(fmt.Sprintf("✓ %d", done)))
		} else {
			parts = append(parts, dim.Render("no threads"))
		}
	}
	if p.Problem != "" {
		parts = append(parts, warnStyle.Bold(true).Render("!"))
	}
	return strings.Join(parts, "  ")
}

// needSince is since when a thread has waited on the user: its agent's
// block when herdr shows one, else herdr-projects' last state change.
func needSince(t deck.Thread) time.Time {
	if t.Pane != nil && t.Pane.AgentStatus == "blocked" && !t.Pane.Since.IsZero() {
		return t.Pane.Since
	}
	return t.Changed
}

// pickerFooter is the key help while the picker is open.
func (m Model) pickerFooter() string {
	rows := m.pickRows()
	i := pickCursor(rows, m.pick.sel)
	act := ""
	if i >= 0 {
		switch r := rows[i]; {
		case r.kind != pickProject:
			act = "↵ go to pane  "
		case r.project.Archived():
			act = ""
		case r.project.PaneID == "" && m.snap.Herdr:
			act = "↵ start coordinator  "
		default:
			act = "↵ go to coordinator  "
		}
	}
	esc := "esc close"
	if m.pick.filter != "" {
		esc = "esc clear"
	}
	return act + "↑↓ move  tab archived  " + esc
}

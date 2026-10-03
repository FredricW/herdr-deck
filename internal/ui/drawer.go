package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/changelog"
	"github.com/FredricW/herdr-deck/internal/deck"
)

// drawerMode says what the drawer shows.
type drawerMode int

const (
	modeRow      drawerMode = iota // the selected row
	modeSources                    // which sources are missing
	modeHelp                       // every key
	modeReport                     // the selected thread's report
	modeSettings                   // the settings page
	modeNews                       // What's new: the changelog
)

// item is one piece of a drawer value: a word of text or a whole link.
type item struct {
	text  string
	style lipgloss.Style
	link  int // index into the row's links, or -1
}

// group is a run of items that starts on a new line and wraps as a block.
type group struct {
	items []item
	sep   string
	hang  int // extra indent of the group's wrapped lines
}

// zone is a clickable link or changed file on a drawer line.
type zone struct {
	x0, x1 int // columns [x0, x1)
	link   int
	// file is the Files section's file number (1-based), -1 for its
	// total line, which opens the whole diff, and 0 for a link.
	file int
}

// dline is one rendered drawer line.
type dline struct {
	text  string
	zones []zone
	// setting is the settings page's row on this line, as an index into
	// config.Specs plus one; 0 is none.
	setting int
}

// drawer builds the drawer's lines for one width.
type drawer struct {
	width  int
	labelW int
	title  string
	lines  []dline
	light  bool // the terminal's background is light
	// filesAt is the line the Files section starts on, or -1.
	filesAt int
	// focus and focusEnd are the first and last lines of the settings
	// page's selected row; -1 elsewhere.
	focus, focusEnd int
}

func newDrawer(width int, title string) *drawer {
	lw := 9
	if width < wideMin {
		lw = 8
	}
	return &drawer{width: width, labelW: lw, title: title, filesAt: -1, focus: -1, focusEnd: -1}
}

func words(text string, st lipgloss.Style) group {
	var g group
	g.sep = " "
	for _, w := range strings.Fields(text) {
		g.items = append(g.items, item{text: w, style: st, link: -1})
	}
	return g
}

func span(text string, st lipgloss.Style) item { return item{text: text, style: st, link: -1} }

// field adds a labelled value. Each group starts on a new line; items wrap
// at the drawer's width. A highlighted field is the link chooser's line.
func (d *drawer) field(label string, highlight bool, groups ...group) {
	d.styledField(label, dim, highlight, groups...)
}

func (d *drawer) styledField(label string, labelStyle lipgloss.Style, chosen bool, groups ...group) {
	avail := max(d.width-1-d.labelW, 4)
	first := true
	prefix := func() (string, int) {
		w := 1 + d.labelW
		if !first {
			return strings.Repeat(" ", w), w
		}
		first = false
		lab := label + strings.Repeat(" ", max(d.labelW-ansi.StringWidth(label), 0))
		if chosen {
			return warnStyle.Render("▶") + highlight(bold.Render(lab), d.light), w
		}
		return " " + labelStyle.Render(lab), w
	}
	for _, g := range groups {
		if len(g.items) == 0 {
			continue
		}
		var b strings.Builder
		var zones []zone
		pre, x := prefix()
		b.WriteString(pre)
		used := 0
		flush := func() {
			d.lines = append(d.lines, dline{text: b.String(), zones: zones})
			b.Reset()
			zones = nil
			pre, x = prefix()
			b.WriteString(pre + strings.Repeat(" ", g.hang))
			x += g.hang
			used = 0
		}
		for _, it := range g.items {
			text := it.text
			w := ansi.StringWidth(text)
			sepW := 0
			if used > 0 {
				sepW = ansi.StringWidth(g.sep)
			}
			if used > 0 && used+sepW+w > avail-(x-1-d.labelW) {
				flush()
				sepW = 0
			}
			if room := avail - (x - 1 - d.labelW) - used - sepW; w > room {
				text = ansi.Truncate(text, max(room, 1), "…")
				w = ansi.StringWidth(text)
			}
			if sepW > 0 {
				b.WriteString(g.sep)
				used += sepW
			}
			if it.link >= 0 {
				zones = append(zones, zone{x0: x + used, x1: x + used + w, link: it.link})
			}
			b.WriteString(it.style.Render(text))
			used += w
		}
		d.lines = append(d.lines, dline{text: b.String(), zones: zones})
	}
}

// line adds a line of its own, truncated to the width.
func (d *drawer) line(s string) {
	d.lines = append(d.lines, dline{text: ansi.Truncate(s, d.width, "…")})
}

// text adds wrapped paragraphs without a label, for reports and help.
func (d *drawer) text(s string, st lipgloss.Style) {
	for _, para := range strings.Split(s, "\n") {
		if strings.TrimSpace(para) == "" {
			d.lines = append(d.lines, dline{})
			continue
		}
		indent := len(para) - len(strings.TrimLeft(para, " "))
		wrapped := ansi.Wrap(strings.TrimRight(para, " "), max(d.width-1, 10), "")
		for _, l := range strings.Split(wrapped, "\n") {
			if indent > 0 && !strings.HasPrefix(l, " ") {
				l = strings.Repeat(" ", indent) + l
			}
			d.lines = append(d.lines, dline{text: " " + st.Render(ansi.Truncate(l, d.width-1, "…"))})
		}
	}
}

// rowDrawer describes the selected row. choosing is the link kind the chooser
// waits on, or -1.
func (m Model) rowDrawer(r row, links []deck.Link, width int, choosing deck.LinkKind) *drawer {
	narrow := width < wideMin
	switch r.kind {
	case rowInbox:
		return m.inboxDrawer(*r.inbox, links, width, choosing)
	case rowHeading:
		return m.projectDrawer(r, width)
	}

	t, hasThread := r.thread()
	title := r.title()
	if narrow && hasThread && r.task != nil {
		title += " · " + t.ID
	}
	d := newDrawer(width, title)

	if r.task != nil && !narrow || r.task == nil {
		for _, th := range r.threads {
			g := group{sep: " ", items: []item{span(th.ID, bold)}}
			if th.Title != "" && (r.task != nil || len(r.threads) > 1) {
				g.items = append(g.items, words(th.Title, plain).items...)
			}
			g.items = append(g.items, m.paneItems(th)...)
			d.field("Thread", false, g)
		}
	}
	switch {
	case hasThread:
		d.field("Status", false, words(threadStatusLine(t), statusStyle(t.Status)))
	case r.task != nil && r.task.Done:
		d.field("Status", false, words("done", dim))
	default:
		d.field("Status", false, words("no thread yet", dim))
	}
	if r.task != nil && r.task.Owner != "" {
		d.field("Owner", false, words(r.task.Owner, plain))
	}
	if hasThread && len(t.Next) > 0 {
		var gs []group
		for _, n := range t.Next {
			g := words(n, plain)
			g.items = append([]item{span("→", bold)}, g.items...)
			g.hang = 2
			gs = append(gs, g)
		}
		d.field("Next", false, gs...)
	}
	if r.task != nil && r.task.Notes != "" {
		var gs []group
		for _, n := range strings.Split(r.task.Notes, "\n") {
			gs = append(gs, words(n, plain))
		}
		d.field("Note", false, gs...)
	}
	m.linkFields(d, links, choosing, t)
	if hasThread && t.Report != "" {
		d.field("Report", false, group{sep: " ", items: []item{span("threads/"+t.ID+".md", plain), span("· r shows it", dim)}})
	}
	if hasThread && t.Branch != "" && !narrow {
		d.field("Branch", false, words(t.Branch, plain))
	}
	if hasThread && (len(t.DevServers) > 0 || t.DevNote != "" || t.DevUp != nil) {
		d.field("Dev", false, devGroup(t))
	}
	if hasThread && t.DevUp != nil {
		d.field("Log", false, words(tilde(t.DevUp.Log), dim))
	}
	if hasThread {
		m.filesField(d)
	}
	return d
}

// maxFiles is how many changed files the Files section lists: one per
// digit.
const maxFiles = 9

// filesField is the Files section: the selected thread's changed files
// with their change marks and line counts, as a list or a folder tree,
// under a total line. It shows nothing until the first diff is read, and
// a dim note when there is none to show.
func (m Model) filesField(d *drawer) {
	_, df, ok := m.diff()
	switch {
	case !ok:
		return
	case df.Note != "":
		d.field("Files", false, words(df.Note, dim))
		return
	case len(df.Files) == 0:
		d.field("Files", false, words("no changes against "+df.Base, dim))
		return
	}
	added, deleted := df.Totals()
	noun := "files"
	if len(df.Files) == 1 {
		noun = "file"
	}
	groups := []group{{sep: " ", items: []item{
		span(fmt.Sprintf("%d %s", len(df.Files), noun), plain),
		span(fmt.Sprintf("+%d", added), addedStyle),
		span(fmt.Sprintf("-%d", deleted), deletedStyle),
		span("vs "+df.Base, dim),
	}}}
	d.light = m.light
	first := len(d.lines)
	d.field("Files", m.files, groups...)
	d.lines[first].zones = []zone{{x0: 1 + d.labelW, x1: d.width, link: -1, file: -1}}
	d.filesAt = first
	if m.tree {
		n := 0
		for _, l := range fileTree(df.Files) {
			if n == maxFiles {
				break
			}
			if l.dir != nil {
				d.dirLine(l.dir, l.depth)
				continue
			}
			n++
			d.fileLine(n, df.Files[l.file], l.depth, treeName(df.Files[l.file]))
		}
	} else {
		for i, f := range df.Files[:min(len(df.Files), maxFiles)] {
			name := f.Path
			if f.OldPath != "" {
				name = f.OldPath + " → " + f.Path
			}
			d.fileLine(i+1, f, 0, name)
		}
	}
	if more := len(df.Files) - maxFiles; more > 0 {
		d.line(strings.Repeat(" ", 1+d.labelW) + dim.Render(fmt.Sprintf("+%d more · d d opens the whole diff", more)))
	}
}

// truncateLeft shortens plain text s to w columns by dropping its start.
func truncateLeft(s string, w int) string {
	if ansi.StringWidth(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && ansi.StringWidth(string(r))+1 > w {
		r = r[1:]
	}
	return "…" + string(r)
}

// paneItems describe a thread's herdr pane: its live agent when herdr shows
// the pane, "closed" when herdr was read and does not.
func (m Model) paneItems(t deck.Thread) []item {
	switch {
	case t.Pane != nil:
		items := []item{span("·", dim), span("pane "+t.Pane.ID, dim)}
		if st := t.Pane.AgentStatus; st != "" && st != "unknown" {
			style := dim
			switch st {
			case "blocked":
				style = needsStyle
			case "working":
				style = workStyle
			}
			items = append(items, span("·", dim), span(agentName(t.Pane.Agent)+" "+st, style))
		}
		return items
	case t.PaneID == "":
		return nil
	case m.snap.Herdr && t.Status != deck.StatusDone:
		return []item{span("·", dim), span("pane "+t.PaneID+" closed", dim)}
	}
	return []item{span("·", dim), span("pane "+t.PaneID, dim)}
}

func agentName(a string) string {
	if a == "" {
		return "agent"
	}
	return a
}

// linkFields adds one line per link kind, numbering every link 1-9 in order.
// pr is the thread whose PR details go next to its GitHub link.
func (m Model) linkFields(d *drawer, links []deck.Link, choosing deck.LinkKind, pr deck.Thread) {
	d.light = m.light
	for _, kind := range []deck.LinkKind{deck.LinkLinear, deck.LinkFigma, deck.LinkNotion, deck.LinkGitHub, deck.LinkLocalhost} {
		var g group
		g.sep = "   "
		if d.width < wideMin {
			g.sep = "  "
		}
		n, unlinked := 0, false
		for i, l := range links {
			if l.Kind != kind {
				continue
			}
			n++
			label := l.Label
			if i < 9 {
				label = fmt.Sprintf("%d %s", i+1, label)
			}
			if pr.PR != nil && l.URL == pr.PR.URL {
				label += prDetails(*pr.PR, d.width < wideMin)
			}
			if kind == deck.LinkLocalhost {
				label += " " + devDot(!l.Down)
			}
			style := plain
			if l.Issue != nil {
				label += " " + issueStyle(*l.Issue).Render(shortState(l.Issue.State))
				if issueClosed(*l.Issue) {
					style = dim
				}
			}
			g.items = append(g.items, item{text: label, style: style, link: i})
			unlinked = unlinked || l.URL == ""
		}
		if n == 0 {
			continue
		}
		highlight := choosing == kind
		groups := []group{g}
		if highlight {
			var extra []item
			if kind == deck.LinkFigma {
				extra = append(extra, span("d desktop app", dim))
			}
			extra = append(extra, span("a all", dim))
			groups = append(groups, group{sep: "  ", items: extra})
		}
		if kind == deck.LinkLinear && unlinked {
			groups = append(groups, group{sep: " ", items: []item{span(noWorkspace, dim)}})
		}
		label := kind.String()
		if kind == deck.LinkLocalhost {
			label = "Open"
		}
		d.field(label, highlight, groups...)
	}
}

// noWorkspace says why a bare Linear ID has no link.
const noWorkspace = "no Linear workspace: set linear_workspace in the config file, --linear-workspace or $" + deck.EnvLinearWorkspace

func prDetails(pr deck.PullRequest, narrow bool) string {
	var parts []string
	if r := reviewText(pr); r != "" {
		parts = append(parts, r)
	}
	if pr.Comments > 0 {
		if narrow {
			parts = append(parts, fmt.Sprintf("%dc", pr.Comments))
		} else {
			parts = append(parts, fmt.Sprintf("%d comments", pr.Comments))
		}
	}
	if n := len(pr.FailingChecks); n > 0 {
		parts = append(parts, fmt.Sprintf("✕ %d failing", n))
	}
	if len(parts) == 0 {
		return ""
	}
	return " · " + strings.Join(parts, " · ")
}

// devGroup is the drawer's Dev line: each server's port, name and dot; a
// fallback port is marked ~. Without servers it says why. While an `up`
// command the deck started runs and a port does not answer yet, it says
// "starting…"; when the command ended with no port answering, "up exited".
func devGroup(t deck.Thread) group {
	var state item
	if u := t.DevUp; u != nil {
		all, some := true, false
		for _, s := range t.DevServers {
			all = all && s.Running
			some = some || s.Running
		}
		switch {
		case u.Alive && (!all || len(t.DevServers) == 0):
			state = span("starting…", warnStyle)
		case !u.Alive && !some:
			state = span("up exited · see the log", dim)
		}
	}
	if len(t.DevServers) == 0 {
		// While the command starts, "not started" is old news; a
		// problem (invalid manifest, no worktree) still shows.
		if state.text == "starting…" && (t.DevNote == "" || strings.HasPrefix(t.DevNote, "not started") || strings.HasPrefix(t.DevNote, "the state file")) {
			return group{items: []item{state}}
		}
		g := words(t.DevNote, dim)
		if state.text != "" {
			g.items = append(g.items, span("·", dim), state)
		}
		return g
	}
	g := group{sep: "   "}
	for _, s := range t.DevServers {
		text := fmt.Sprintf(":%d %s %s", s.Port, s.Name, devDot(s.Running))
		if s.Fallback {
			text = "~" + text
		}
		g.items = append(g.items, span(text, plain))
	}
	if t.DevServers[0].Fallback {
		g.items = append(g.items, span("herdr port token", dim))
	}
	if state.text != "" {
		g.items = append(g.items, state)
	}
	return g
}

// devDot is green ● for a listening port, dim ○ otherwise.
func devDot(up bool) string {
	if up {
		return okStyle.Render("●")
	}
	return dim.Render("○")
}

// threadStatusLine is the drawer's Status: the state line, plus the activity
// when the state line does not already say it.
func threadStatusLine(t deck.Thread) string {
	s := t.StateLine
	if s == "" {
		s = statusWord(t.Status)
	}
	if t.Activity != "" && !strings.Contains(s, t.Activity) {
		s += " · " + t.Activity
	}
	return s
}

func statusWord(s deck.ThreadStatus) string {
	switch s {
	case deck.StatusNeedsYou:
		return "needs you"
	case deck.StatusWorking:
		return "working"
	case deck.StatusReview:
		return "ready for review"
	case deck.StatusDone:
		return "resolved"
	}
	return "idle"
}

func (m Model) inboxDrawer(it deck.InboxItem, links []deck.Link, width int, choosing deck.LinkKind) *drawer {
	title := "Inbox item"
	if !it.Created.IsZero() {
		title += " · " + it.Created.In(m.loc()).Format("15:04")
	}
	d := newDrawer(width, title)
	if it.Kind != "" {
		d.field("Kind", false, words(it.Kind, plain))
	}
	t, ok := findThread(m.snap, it.Thread)
	if ok {
		d.field("Subject", false, group{sep: " ", items: append([]item{span(t.ID, bold)}, words(t.Title, plain).items...)})
	} else if it.Subject != "" {
		d.field("Subject", false, words(it.Subject, plain))
	}
	if it.Summary != "" {
		d.field("Summary", false, words(it.Summary, plain))
	}
	m.linkFields(d, links, choosing, t)
	d.field("File", false, words("inbox/"+it.ID+".md", dim))
	return d
}

// projectDrawer shows the project, and the list's note when on a heading.
func (m Model) projectDrawer(r row, width int) *drawer {
	title := "Project"
	if r.kind == rowHeading && r.list != "" {
		title = r.list
	}
	d := newDrawer(width, title)
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

func (m Model) sourcesDrawer(width int) *drawer {
	d := newDrawer(width, "Sources")
	d.labelW = 2
	read := ""
	if !m.snap.ReadAt.IsZero() {
		read = "read " + m.snap.ReadAt.In(m.loc()).Format("15:04")
	}
	for _, miss := range m.snap.Missing {
		d.styledField("!", warnStyle.Bold(true), false, words(miss, plain))
	}
	if !m.snap.ThreadsAsOf.IsZero() {
		d.styledField("!", warnStyle.Bold(true), false, words("threads as of "+m.snap.ThreadsAsOf.In(m.loc()).Format("15:04")+", when herdr-projects last wrote them", plain))
	}
	if m.opt.RestartFailed != "" {
		d.styledField("!", warnStyle.Bold(true), false, words("a new herdr-deck is installed, but restarting into it failed ("+m.opt.RestartFailed+"); this deck keeps running the old one", plain))
	}
	if v := m.update.Available; v != "" {
		d.styledField("↑", okStyle, false, words("herdr-deck "+v+" is available: run `herdr-deck update`", plain))
	}
	for _, n := range m.snap.Notes {
		d.styledField("·", dim, false, words(n, dim))
	}
	if p := m.update.Problem; p != "" {
		d.styledField("·", dim, false, words("update check: "+p, dim))
	}
	ok := "every source"
	if len(m.snap.Missing) > 0 {
		ok = "everything else"
	}
	d.styledField("✓", okStyle, false, group{sep: " ", items: []item{span(ok, plain), span(read, dim)}})
	d.line("")
	d.line(dim.Render(" ! or esc returns"))
	return d
}

func (m Model) reportDrawer(r row, width int) *drawer {
	t, _ := r.thread()
	d := newDrawer(width, "Report · "+t.ID)
	d.text(strings.TrimSpace(t.Report), plain)
	return d
}

func (m Model) helpDrawer(width int) *drawer {
	d := newDrawer(width, "Keys")
	d.labelW = 11
	for _, k := range helpLines {
		d.field(k[0], false, words(k[1], plain))
	}
	return d
}

var helpLines = [][2]string{
	{"j k", "move; the drawer follows"},
	{"space", "fold or unfold the list under the cursor"},
	{"1-9", "open the drawer's numbered link"},
	{"l f n g", "open the first Linear, Figma, Notion or PR link; with several, pick one: a digit, a all, d Figma desktop app, the letter again the first, esc cancels"},
	{"o", "open the first localhost link whose dev server is running"},
	{"↵", "focus the thread's herdr pane"},
	{"e", "open the thread's worktree in the editor"},
	{"u", "start the thread's dev servers: the dev manifest's up command, detached"},
	{"d", "the thread's changed files: a digit opens that file in the diff tool, d again the whole diff, t switches between list and folder tree; a click on a file opens it"},
	{"r", "the thread's report, full height"},
	{"z", "drawer: normal, full height, hidden"},
	{"pgup pgdn", "scroll the drawer"},
	{"!", "sources: what could not be read"},
	{"s", "settings: every setting with its value and source; ↵ edits, toggles or cycles one in the config file"},
	{"p", "projects and what needs you in each; type to filter, ↵ goes there"},
	{"w", "what's new: the changelog, newest first; with ↑ in the header, also what the newer version brings"},
	{"?", "this help; esc returns"},
	{"q", "quit"},
	{"mouse", "click a row, link, list heading, ! N or the other-projects line; the wheel moves the list or scrolls the drawer"},
}

// tilde shortens a path under the home folder to ~/….
func tilde(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		return filepath.Join("~", rel)
	}
	return p
}

func ageText(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// newsDrawer is What's new: what a newer version brings when the header
// shows ↑, then this deck's changelog, newest first, with the release it
// runs marked.
func (m Model) newsDrawer(width int) *drawer {
	d := newDrawer(width, "What's new")
	d.labelW = 9
	if v := m.update.Available; v != "" {
		d.text(okStyle.Render("↑ "+v+" is available: run `herdr-deck update`"), plain)
		for _, r := range m.update.News {
			d.line("")
			d.release(r, okStyle.Render("new"))
		}
		d.line("")
	}
	running := m.opt.Changelog.Running(m.opt.Version)
	shown := false
	for i, r := range m.opt.Changelog.Releases {
		// A release build's Unreleased section is not in it.
		if r.Empty() || r.Unreleased() && running >= 0 && i != running {
			continue
		}
		if shown {
			d.line("")
		}
		shown = true
		tag := ""
		if i == running {
			tag = okStyle.Render("● running")
			if r.Unreleased() {
				tag = okStyle.Render("● this build")
			}
		}
		d.release(r, tag)
	}
	if !shown {
		d.text("This build has no changelog.", dim)
	}
	return d
}

// release adds one changelog release: its title, date and tag, its summary,
// then each section's items under the section's name.
func (d *drawer) release(r changelog.Release, tag string) {
	head := " " + bold.Render(r.Title())
	if r.Date != "" {
		head += dim.Render(" · " + r.Date)
	}
	if tag != "" {
		head += dim.Render(" · ") + tag
	}
	d.line(head)
	if r.Summary != "" {
		d.text(r.Summary, plain)
	}
	for _, s := range r.Sections {
		var gs []group
		for _, it := range s.Items {
			g := words(it, plain)
			g.items = append([]item{span("·", dim)}, g.items...)
			g.hang = 2
			gs = append(gs, g)
		}
		d.field(s.Name, false, gs...)
	}
}

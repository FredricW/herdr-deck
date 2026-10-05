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
	"github.com/FredricW/herdr-deck/internal/markdown"
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
	modeCheck                      // a failed check's log tail
)

// tabKind is one of the drawer's tabs.
type tabKind int

const (
	tabOverview tabKind = iota
	tabFiles
	tabCommits
	tabLog
	numTabs
)

func (t tabKind) String() string {
	switch t {
	case tabFiles:
		return "Files"
	case tabCommits:
		return "Commits"
	case tabLog:
		return "Log"
	}
	return "Overview"
}

// actKind says what a click, or enter on the drawer cursor, does.
type actKind int

const (
	actNone       actKind = iota
	actLink               // open link n (an index into the row's links)
	actFile               // preview file n (1-based, in display order)
	actDiff               // open the whole diff in the diff tool
	actEvent              // act on Log event n
	actTab                // switch to tab n
	actCommit             // preview commit n (1-based); 0 is the uncommitted row
	actExpand             // expand or collapse commit n (1-based)
	actCommitFile         // preview file f (0-based, in display order) of commit n
	actCheck              // show the log of the PR's check n
	actThread             // open the PR's review thread n on GitHub
)

type action struct {
	kind actKind
	n    int
	f    int // actCommitFile's file
}

// item is one piece of a drawer value: a word of text or a whole link.
type item struct {
	text  string
	style lipgloss.Style
	act   action
	sel   bool // drawn on the selection background
	stop  bool // a stop of the drawer cursor
}

// group is a run of items that starts on a new line and wraps as a block.
type group struct {
	items []item
	sep   string
	hang  int // extra indent of the group's wrapped lines
}

// zone is a clickable part of a drawer line.
type zone struct {
	x0, x1 int // columns [x0, x1)
	act    action
}

// dline is one rendered drawer line.
type dline struct {
	text  string
	zones []zone
	// setting is the settings page's row on this line, as an index into
	// config.Specs plus one; 0 is none.
	setting int
}

// stop is where the drawer cursor can rest: a chip, a file or an event.
type stop struct {
	line int
	act  action
}

// section is where one of Overview's titled sections starts.
type section struct {
	name string
	at   int
}

// drawer builds the drawer's lines for one width.
type drawer struct {
	width  int
	labelW int
	// title is the rule a full view (help, sources, …) starts with; a
	// row's drawer has a card instead and leaves it empty.
	title string
	// card is the header card's two lines; tabs and tabZones the tab bar
	// under it (report says what stands there instead), all drawn above
	// the lines that scroll.
	card     []string
	tabs     string
	tabZones []zone
	lines    []dline
	light    bool // the terminal's background is light
	// focus and focusEnd are the first and last lines of the settings
	// page's selected row; -1 elsewhere.
	focus, focusEnd int

	sections []section
	stops    []stop
	// cur is the stop the drawer cursor is on, or -1; given is how many
	// stops the builder handed out so far.
	cur, given int
}

func newDrawer(width int, title string) *drawer {
	lw := 9
	if width < wideMin {
		lw = 8
	}
	return &drawer{width: width, labelW: lw, title: title, focus: -1, focusEnd: -1, cur: -1}
}

// head is the lines drawn above the scrolling ones: the card and the tab
// bar, or the report's line, with a blank line above and below it.
func (d *drawer) head() []string {
	if d.tabs == "" {
		return d.card
	}
	return append(append([]string(nil), d.card...), "", d.tabs, "")
}

// nextStop hands out the builder's next cursor stop and says whether the
// cursor is on it.
func (d *drawer) nextStop() bool {
	on := d.given == d.cur
	d.given++
	return on
}

// stopLine adds a whole line that is a cursor stop: on the cursor it gets
// `▸` in its first column and the selection background.
func (d *drawer) stopLine(text string, act action, zones []zone) {
	text = fit(text, d.width)
	if d.nextStop() {
		text = highlight("▸"+ansi.Cut(text, 1, d.width), d.light)
	}
	d.stops = append(d.stops, stop{line: len(d.lines), act: act})
	d.lines = append(d.lines, dline{text: text, zones: zones})
}

func words(text string, st lipgloss.Style) group {
	var g group
	g.sep = " "
	for _, w := range strings.Fields(text) {
		g.items = append(g.items, item{text: w, style: st})
	}
	return g
}

func span(text string, st lipgloss.Style) item { return item{text: text, style: st} }

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
		var stops []action
		add := func() {
			for _, a := range stops {
				d.stops = append(d.stops, stop{line: len(d.lines), act: a})
			}
			d.lines = append(d.lines, dline{text: b.String(), zones: zones})
			stops = nil
		}
		pre, x := prefix()
		b.WriteString(pre)
		used := 0
		flush := func() {
			add()
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
			if it.act.kind != actNone {
				zones = append(zones, zone{x0: x + used, x1: x + used + w, act: it.act})
			}
			if it.stop {
				stops = append(stops, it.act)
			}
			if it.sel {
				b.WriteString(highlight(it.style.Render(text), d.light))
			} else {
				b.WriteString(it.style.Render(text))
			}
			used += w
		}
		add()
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

// markdown adds Markdown rendered the way glow does, wrapped to the
// drawer's width in the style that suits the terminal's background.
func (d *drawer) markdown(src string) {
	for _, l := range markdown.Render(src, d.width-1, !d.light) {
		if l == "" {
			d.lines = append(d.lines, dline{})
			continue
		}
		d.lines = append(d.lines, dline{text: " " + ansi.Truncate(l, d.width-1, "…")})
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

func agentName(a string) string {
	if a == "" {
		return "agent"
	}
	return a
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
	{"[ ]", "previous / next tab: Overview, Files, Commits, Log"},
	{"tab", "focus the drawer: j k move in it, ↵ opens, tab switches tabs, esc returns"},
	{"1-9", "open the numbered link; on Files and Commits, preview that file or commit"},
	{"l f n g", "open the first Linear, Figma, Notion or PR link; with several, pick one: a digit, a all, d Figma desktop app, the letter again the first, esc cancels"},
	{"o", "open the first localhost link whose dev server is running"},
	{"↵", "focus the thread's herdr pane"},
	{"e", "open the thread's worktree in the editor"},
	{"u", "start the thread's dev servers: the dev manifest's up command, detached"},
	{"d", "focus the Files tab: a digit previews that file; d again opens the file under the cursor in the diff tool; t list or tree"},
	{"v", "on Files and Commits: preview the file or commit under the cursor in the list's place, coloured by its language; j k pick another, v or esc returns to the list"},
	{"Files", "↵, a digit or a click previews a file; in the focused drawer d opens it in the diff tool; a opens the whole diff there"},
	{"Commits", "the branch's commits since its base, newest first: ↵, a digit or a click previews one as git show does; in the focused drawer d opens the commit in the diff tool and g in the PR on GitHub; a opens the whole diff"},
	{"space → l", "in the focused Commits tab: expand the commit to the files it changed (space again, ← or h collapses it; a click on ▸ / ▾ too); ↵ or a click on a file previews its change in that commit, d opens it in the diff tool"},
	{"J K", "scroll the preview a line; pgup pgdn a page, the wheel over it three lines"},
	{"S", "switch the preview between unified and split (old on the left, new on the right); diff.layout picks the start; a pane under 100 columns shows unified"},
	{"r", "the thread's report, full height"},
	{"c", "a failed check's log, full height (or click its chip); c again the next"},
	{"z", "drawer: normal, full height, hidden"},
	{"pgup pgdn", "scroll the drawer, or the preview while it shows"},
	{"!", "sources: what could not be read"},
	{"s", "settings: every setting with its value and source; ↵ edits, toggles or cycles one in the config file"},
	{"p", "projects and what needs you in each; type to filter, ↵ goes there"},
	{"w", "what's new: the changelog, newest first; with ↑ in the header, also what the newer version brings"},
	{"?", "this help; esc returns"},
	{"q", "quit"},
	{"mouse", "click a row, tab, chip, file, Log event, list heading, ! N or the other-projects line; the wheel moves the list or scrolls the drawer"},
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
	d.light = m.light
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

// release adds one changelog release: its title, date and tag, then its
// summary and each section's items as Markdown under the section's name.
func (d *drawer) release(r changelog.Release, tag string) {
	head := " " + bold.Render(r.Title())
	if r.Date != "" {
		head += dim.Render(" · " + r.Date)
	}
	if tag != "" {
		head += dim.Render(" · ") + tag
	}
	d.line(head)
	var b strings.Builder
	if r.Summary != "" {
		// The parser ends each summary paragraph with one newline.
		b.WriteString(strings.ReplaceAll(strings.TrimSpace(r.Summary), "\n", "\n\n") + "\n\n")
	}
	for _, s := range r.Sections {
		b.WriteString("### " + s.Name + "\n\n")
		for _, it := range s.Items {
			b.WriteString("- " + it + "\n")
		}
		b.WriteString("\n")
	}
	if b.Len() > 0 {
		d.line("")
		d.markdown(b.String())
	}
}

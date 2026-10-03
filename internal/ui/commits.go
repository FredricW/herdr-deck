package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/syntax"
)

// The Commits tab: the thread branch's own commits since its base, newest
// first, each one previewed with v or ↵, opened in the diff tool with d
// and on GitHub with g.

type commitsMsg struct {
	key     string
	commits deck.Commits
}

// commitThread is the selected row's thread whose commits the tab reads:
// not a resolved thread, whose worktree is likely gone.
func (m Model) commitThread() (deck.Thread, bool) {
	r, _ := m.selected()
	t, ok := r.thread()
	if r.kind != rowWork || !ok || t.Worktree == "" || t.Status == deck.StatusDone {
		return deck.Thread{}, false
	}
	return t, true
}

// commits is the selected thread's last commit list, when one was read.
func (m Model) commits() (deck.Thread, deck.Commits, bool) {
	t, ok := m.commitThread()
	if !ok || m.opt.Commits == nil {
		return t, deck.Commits{}, false
	}
	cs, ok := m.commitLists[diffKey(t)]
	return t, cs, ok
}

// readCommits reads the selected thread's commits, again when force is
// set, else only when the selection moved to another thread. One read
// runs at a time; a move meanwhile reads again when it ends.
func (m *Model) readCommits(force bool) tea.Cmd {
	t, ok := m.commitThread()
	if m.opt.Commits == nil || !ok {
		return nil
	}
	k := diffKey(t)
	if !force && k == m.commitsSel {
		return nil
	}
	if m.committing {
		m.commitsAgain = true
		return nil
	}
	m.committing, m.commitsSel = true, k
	read := m.opt.Commits
	return func() tea.Msg { return commitsMsg{key: k, commits: read(context.Background(), t)} }
}

// pseudoRow says whether the Commits tab starts with the uncommitted
// changes' row, which takes the first cursor stop.
func pseudoRow(cs deck.Commits) bool { return cs.Note == "" && cs.Uncommitted > 0 }

// cursorCommit is the index of the commit under the drawer cursor, or -1
// on the uncommitted row or with no commits.
func (m Model) cursorCommit(cs deck.Commits) int {
	i := m.dcur
	if pseudoRow(cs) {
		i--
	}
	if i < 0 || i >= len(cs.List) {
		return -1
	}
	return i
}

// commitStop is the drawer cursor stop of commit i.
func commitStop(cs deck.Commits, i int) int {
	if pseudoRow(cs) {
		return i + 1
	}
	return i
}

// commitsTab is the Commits tab: the uncommitted changes' row when there
// are any, then a line per commit, newest first. Commits 1-9 take the
// digits; every row is a stop of the drawer cursor.
func (m Model) commitsTab(d *drawer, r row) {
	t, _ := r.thread()
	switch {
	case t.Status == deck.StatusDone:
		d.line(" " + dim.Render(t.ID+" is resolved: its commits are on its branch and PR"))
		return
	case t.Worktree == "":
		d.line(" " + dim.Render("no worktree"))
		return
	}
	_, cs, ok := m.commits()
	switch {
	case !ok:
		d.line(" " + dim.Render("reading the commits…"))
		return
	case cs.Note != "":
		d.line(" " + dim.Render(cs.Note))
		return
	}
	if pseudoRow(cs) {
		noun := "files"
		if cs.Uncommitted == 1 {
			noun = "file"
		}
		left := " " + warnStyle.Render("●") + " " + plain.Render(fmt.Sprintf("uncommitted · %d %s", cs.Uncommitted, noun))
		act := action{kind: actCommit, n: 0}
		d.stopLine(spread(left, dim.Render("→ Files")+" ", d.width), act, []zone{{x0: 0, x1: d.width, act: act}})
	}
	if len(cs.List) == 0 {
		d.line(" " + dim.Render("no commits since "+cs.Base))
		return
	}
	// The counts line up in a column as wide as the widest.
	countsW := 0
	for _, c := range cs.List {
		if !c.Merge {
			countsW = max(countsW, ansi.StringWidth(commitCounts(c)))
		}
	}
	for i, c := range cs.List {
		d.commitLine(i+1, c, countsW, m.opt.Now())
	}
	if cs.More > 0 {
		d.line("   " + dim.Render(fmt.Sprintf("+%d more", cs.More)))
	}
}

func commitCounts(c deck.Commit) string { return fmt.Sprintf("+%d −%d", c.Added, c.Deleted) }

// commitLine is one commit: its digit (a dim · after 9), short sha dim,
// subject, and at the right its age and +N −M, the counts right-aligned in
// countsW columns. A merge commit is dim with a ⋔ and has no counts.
func (d *drawer) commitLine(n int, c deck.Commit, countsW int, now time.Time) {
	num := dim.Render("·")
	if n <= 9 {
		num = bold.Render(fmt.Sprint(n))
	}
	lead := " " + num + " " + dim.Render(c.Short) + " "
	leadW := 3 + ansi.StringWidth(c.Short) + 1
	age := ""
	if !c.Time.IsZero() {
		age = ageText(now.Sub(c.Time))
	}
	right, rightW := dim.Render(fmt.Sprintf("%3s", age)), max(3, len(age))
	if countsW > 0 {
		counts := ""
		if !c.Merge {
			counts = addedStyle.Render(fmt.Sprintf("+%d", c.Added)) + " " + deletedStyle.Render(fmt.Sprintf("−%d", c.Deleted))
		}
		right += "  " + strings.Repeat(" ", countsW-ansi.StringWidth(counts)) + counts
		rightW += 2 + countsW
	}
	right += " "
	rightW++
	subject, st := syntax.Clean(c.Subject), plain
	if c.Merge {
		subject, st = "⋔ "+subject, dim
	}
	room := max(d.width-leadW-rightW-2, 4)
	subject = ansi.Truncate(subject, room, "…")
	gap := max(d.width-leadW-ansi.StringWidth(subject)-rightW, 1)
	text := lead + st.Render(subject) + strings.Repeat(" ", gap) + right
	act := action{kind: actCommit, n: n}
	d.stopLine(text, act, []zone{{x0: 0, x1: d.width, act: act}})
}

// commitURL is commit sha on GitHub, inside the thread's pull request:
// <repo>/pull/N/commits/<sha>. "" when the PR's URL is not GitHub's.
func commitURL(pr deck.PullRequest, sha string) string {
	u := strings.TrimRight(pr.URL, "/")
	i := strings.LastIndex(u, "/pull/")
	if i < 0 || !strings.HasPrefix(u, "https://github.com/") {
		return ""
	}
	num, _, _ := strings.Cut(u[i+len("/pull/"):], "/")
	if num == "" {
		return ""
	}
	return u[:i] + "/pull/" + num + "/commits/" + sha
}

// previewCommit shows commit i (0-based, as listed) in the preview, with
// the drawer focused and its cursor on that commit.
func (m *Model) previewCommit(i int) {
	_, cs, ok := m.commits()
	if !ok || cs.Note != "" || i < 0 || i >= len(cs.List) {
		m.status = fmt.Sprintf("no commit %d on this row", i+1)
		return
	}
	m.dfocus = true
	m.dcur = commitStop(cs, i)
	m.moveDrawerCursor(0)
	if !m.preview {
		m.togglePreview()
	}
}

// commitsOnlyKey handles the Commits tab's own keys: v turns the preview
// on and off; in the focused drawer d opens the commit under the cursor
// in the diff tool and g on GitHub. done is false for any other key, so
// d and g act as usual from the list.
func (m *Model) commitsOnlyKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if m.mode != modeRow || m.curTab() != tabCommits {
		return nil, false
	}
	switch msg.String() {
	case "v":
		m.togglePreview()
		return nil, true
	case "d":
		if m.dfocus {
			return m.openCommit(), true
		}
	case "g":
		if m.dfocus {
			return m.openCommitOnGitHub(), true
		}
	}
	return nil, false
}

// commitUnderCursor is the commit under the drawer cursor, or says why there
// is none.
func (m *Model) commitUnderCursor() (deck.Thread, deck.Commit, bool) {
	t, cs, ok := m.commits()
	if !ok || cs.Note != "" {
		m.status = "no commits on this row"
		return t, deck.Commit{}, false
	}
	i := m.cursorCommit(cs)
	if i < 0 {
		m.status = "the uncommitted changes are on the Files tab"
		return t, deck.Commit{}, false
	}
	return t, cs.List[i], true
}

// openCommit opens the commit under the drawer cursor in the diff tool,
// against its first parent.
func (m *Model) openCommit() tea.Cmd {
	t, c, ok := m.commitUnderCursor()
	if !ok {
		return nil
	}
	open := m.opt.OpenCommit
	if open == nil {
		m.status = "opening diffs is off"
		return nil
	}
	path, sha, what := t.Worktree, c.SHA, "commit "+c.Short
	return func() tea.Msg { return openedMsg{what: what, err: open(path, sha)} }
}

// openCommitOnGitHub opens the commit under the drawer cursor in the
// thread's pull request on GitHub.
func (m *Model) openCommitOnGitHub() tea.Cmd {
	t, c, ok := m.commitUnderCursor()
	if !ok {
		return nil
	}
	if t.PR == nil {
		m.status = t.ID + " has no pull request on GitHub"
		return nil
	}
	_, cs, _ := m.commits()
	if cs.Upstream && !c.Pushed {
		m.status = "commit " + c.Short + " is not pushed yet"
		return nil
	}
	u := commitURL(*t.PR, c.SHA)
	if u == "" {
		m.status = "the pull request is not on GitHub"
		return nil
	}
	return m.openURL(u, "commit "+c.Short+" on GitHub")
}

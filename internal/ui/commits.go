package ui

import (
	"context"
	"fmt"
	"path"
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

type commitFilesMsg struct {
	key   string // commitKey
	files []deck.DiffFile
	err   error
}

// commitFiles is one commit's changed files, once read.
type commitFiles struct {
	files []deck.DiffFile
	err   string
}

// centry is a row of the Commits tab the drawer cursor can rest on: the
// uncommitted row (commit -1), a commit (file -1), or file of commit,
// in display order, under an expanded commit.
type centry struct{ commit, file int }

// centryID names a row so the cursor can find it again after rows came
// or went above it: the commit's sha and the file's path.
type centryID struct {
	pseudo    bool
	sha, path string
}

// commitFileList is commit c's files in display order (list or tree, as
// the Files tab shows them), and whether they were read.
func (m Model) commitFileList(t deck.Thread, c deck.Commit) ([]deck.DiffFile, commitFiles, bool) {
	cf, ok := m.commitFileSets[commitKey(t, c.SHA)]
	if !ok || cf.err != "" {
		return nil, cf, ok
	}
	return fileOrder(cf.files, m.tree), cf, true
}

// expanded says whether commit c shows its files.
func (m Model) expanded(t deck.Thread, c deck.Commit) bool {
	return m.expand[commitKey(t, c.SHA)]
}

// commitEntries is the Commits tab's cursor stops in order: the
// uncommitted row, then each commit followed, when it is expanded and
// its files were read, by its files.
func (m Model) commitEntries(t deck.Thread, cs deck.Commits) []centry {
	var es []centry
	if pseudoRow(cs) {
		es = append(es, centry{commit: -1, file: -1})
	}
	if cs.Note != "" {
		return es
	}
	for i, c := range cs.List {
		es = append(es, centry{commit: i, file: -1})
		if !m.expanded(t, c) {
			continue
		}
		files, _, _ := m.commitFileList(t, c)
		for j := range files {
			es = append(es, centry{commit: i, file: j})
		}
	}
	return es
}

// cursorEntry is the row under the drawer cursor.
func (m Model) cursorEntry(t deck.Thread, cs deck.Commits) (centry, bool) {
	es := m.commitEntries(t, cs)
	if m.dcur < 0 || m.dcur >= len(es) {
		return centry{}, false
	}
	return es[m.dcur], true
}

// entryID names entry e of cs.
func (m Model) entryID(t deck.Thread, cs deck.Commits, e centry) centryID {
	if e.commit < 0 {
		return centryID{pseudo: true}
	}
	c := cs.List[e.commit]
	id := centryID{sha: c.SHA}
	if e.file >= 0 {
		files, _, _ := m.commitFileList(t, c)
		id.path = files[e.file].Path
	}
	return id
}

// keepCursor runs change, which may add or remove rows of the Commits tab,
// and puts the drawer cursor back on the row it was on: the same file of
// the same commit, else that commit, else the nearest row.
func (m *Model) keepCursor(change func()) {
	t, cs, ok := m.commits()
	on := m.curTab() == tabCommits
	var id centryID
	had := false
	if ok && on {
		if e, ok := m.cursorEntry(t, cs); ok {
			id, had = m.entryID(t, cs, e), true
		}
	}
	change()
	if !had {
		return
	}
	t, cs, ok = m.commits()
	if !ok {
		return
	}
	es := m.commitEntries(t, cs)
	commitAt := -1
	for k, e := range es {
		eid := m.entryID(t, cs, e)
		if eid == id {
			m.dcur = k
			return
		}
		if e.file < 0 && eid.sha == id.sha && id.sha != "" {
			commitAt = k
		}
	}
	if commitAt >= 0 {
		m.dcur = commitAt
		return
	}
	m.dcur = clamp(m.dcur, 0, max(len(es)-1, 0))
}

// setCommits stores a thread's commit list, the drawer cursor staying on
// the same row though rows came or went above it, so the preview does
// not jump to another commit.
func (m *Model) setCommits(key string, cs deck.Commits) {
	m.keepCursor(func() { m.commitLists[key] = cs })
}

// readCommitFiles reads commit c's files, unless they were read or are
// being read.
func (m *Model) readCommitFiles(t deck.Thread, c deck.Commit) tea.Cmd {
	k := commitKey(t, c.SHA)
	if _, ok := m.commitFileSets[k]; ok || m.filesReading[k] || m.opt.CommitFiles == nil {
		return nil
	}
	m.filesReading[k] = true
	read, sha := m.opt.CommitFiles, c.SHA
	return func() tea.Msg {
		files, err := read(context.Background(), t, sha)
		return commitFilesMsg{key: k, files: files, err: err}
	}
}

// setCommitFiles stores a commit's files, keeping the cursor on its row.
func (m *Model) setCommitFiles(msg commitFilesMsg) {
	delete(m.filesReading, msg.key)
	cf := commitFiles{files: msg.files}
	if msg.err != nil {
		cf.err = msg.err.Error()
	}
	m.keepCursor(func() { m.commitFileSets[msg.key] = cf })
}

// toggleExpand expands commit i, or collapses it, as open says (-1
// toggles). Collapsing from one of its files puts the cursor on it.
func (m *Model) toggleExpand(i, open int) tea.Cmd {
	t, cs, ok := m.commits()
	if !ok || cs.Note != "" || i < 0 || i >= len(cs.List) {
		return nil
	}
	c := cs.List[i]
	k := commitKey(t, c.SHA)
	want := !m.expand[k]
	if open >= 0 {
		want = open == 1
	}
	if want == m.expand[k] {
		return nil
	}
	m.keepCursor(func() { m.expand[k] = want })
	m.moveDrawerCursor(0)
	if want {
		return m.readCommitFiles(t, c)
	}
	return nil
}

// pseudoRow says whether the Commits tab starts with the uncommitted
// changes' row, which takes the first cursor stop.
func pseudoRow(cs deck.Commits) bool { return cs.Note == "" && cs.Uncommitted > 0 }

// cursorCommit is the index of the commit under the drawer cursor (the
// commit of a file row too), or -1 on the uncommitted row or with no
// commits.
func (m Model) cursorCommit(cs deck.Commits) int {
	t, _ := m.commitThread()
	e, ok := m.cursorEntry(t, cs)
	if !ok {
		return -1
	}
	return e.commit
}

// commitStop is the drawer cursor stop of commit i's row.
func (m Model) commitStop(cs deck.Commits, i int) int {
	t, _ := m.commitThread()
	for k, e := range m.commitEntries(t, cs) {
		if e.commit == i && e.file < 0 {
			return k
		}
	}
	return 0
}

// commitsTab is the Commits tab: the uncommitted changes' row when there
// are any, then a line per commit, newest first, each expanded one with
// its files under it. Commits 1-9 take the digits; every commit and file
// row is a stop of the drawer cursor.
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
		open := m.expanded(t, c)
		d.commitLine(i+1, c, open, countsW, m.opt.Now())
		if open {
			m.commitFileLines(d, t, i, c)
		}
	}
	if cs.More > 0 {
		d.line("     " + dim.Render(fmt.Sprintf("+%d more", cs.More)))
	}
}

func commitCounts(c deck.Commit) string { return fmt.Sprintf("+%d −%d", c.Added, c.Deleted) }

// commitLine is one commit: its digit (a dim · after 9), the disclosure
// marker (▸ collapsed, ▾ expanded), short sha dim, subject, and at the
// right its age and +N −M, the counts right-aligned in countsW columns. A
// merge commit is dim with a ⋔ and has no counts. A click on the marker
// expands or collapses the commit; on the rest of the line it previews it.
func (d *drawer) commitLine(n int, c deck.Commit, open bool, countsW int, now time.Time) {
	num := dim.Render("·")
	if n <= 9 {
		num = bold.Render(fmt.Sprint(n))
	}
	marker := "▸"
	if open {
		marker = "▾"
	}
	lead := " " + num + " " + dim.Render(marker) + " " + dim.Render(c.Short) + " "
	leadW := 5 + ansi.StringWidth(c.Short) + 1
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
	d.stopLine(text, act, []zone{{x0: 2, x1: 5, act: action{kind: actExpand, n: n}}, {x0: 0, x1: d.width, act: act}})
}

// commitFileLines lists the files of commit i (c) under its row: a note
// while they are read, else each file's status letter, path and counts,
// in a list or, with the tree view on, under their folders.
func (m Model) commitFileLines(d *drawer, t deck.Thread, i int, c deck.Commit) {
	files, cf, ok := m.commitFileList(t, c)
	switch {
	case !ok:
		d.line("       " + dim.Render("reading the files…"))
		return
	case cf.err != "":
		d.line("       " + dim.Render(cf.err))
		return
	case len(files) == 0:
		d.line("       " + dim.Render("no files changed"))
		return
	}
	if !m.tree {
		for j, f := range files {
			d.commitFileLine(i, j, f, -1, listName(f))
		}
		return
	}
	j := 0
	for _, l := range fileTree(cf.files) {
		if l.dir != nil {
			name := truncateLeft(l.dir.name+"/", max(d.width-10-2*l.depth, 4))
			d.lines = append(d.lines, dline{text: fit(strings.Repeat(" ", 10+2*l.depth)+dim.Render(name), d.width)})
			continue
		}
		d.commitFileLine(i, j, files[j], l.depth, treeName(files[j]))
		j++
	}
}

// commitFileLine is file j of commit i under the commit's row, indented
// past its marker: the status letter, the name (losing its start when
// long) and its counts at the right edge, as the Files tab shows them.
// depth is the tree view's depth, or -1 in the list view.
func (d *drawer) commitFileLine(i, j int, f deck.DiffFile, depth int, name string) {
	lead := "       " + changeMark(f) + "  "
	leadW := 10
	if depth >= 0 {
		lead += strings.Repeat("  ", depth)
		leadW += 2 * depth
	}
	ctext, counts := fileCounts(f)
	switch {
	case f.Change == deck.ChangeRenamed && f.Added == 0 && f.Deleted == 0 && !f.Binary:
		ctext, counts = "", "" // moved, content unchanged
	case !f.Binary:
		// The commit rows' minus sign, −, in the same colour.
		ctext = strings.ReplaceAll(ctext, "-", "−")
		counts = strings.ReplaceAll(counts, "-", "−")
	}
	rightW := ansi.StringWidth(ctext) + 1
	name = truncateLeft(name, max(d.width-leadW-rightW-2, 4))
	shown := plain.Render(name)
	if dir, base := path.Split(name); depth < 0 && dir != "" {
		shown = dim.Render(dir) + plain.Render(base)
	}
	gap := max(d.width-leadW-ansi.StringWidth(name)-rightW, 1)
	act := action{kind: actCommitFile, n: i + 1, f: j}
	d.stopLine(lead+shown+strings.Repeat(" ", gap)+counts+" ", act, []zone{{x0: 0, x1: d.width, act: act}})
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
	m.dcur = m.commitStop(cs, i)
	m.moveDrawerCursor(0)
	if !m.preview {
		m.togglePreview()
	}
}

// previewCommitFile shows file j (in display order) of commit i in the
// preview, with the drawer focused and its cursor on that file.
func (m *Model) previewCommitFile(i, j int) {
	t, cs, ok := m.commits()
	if !ok || i < 0 || i >= len(cs.List) {
		return
	}
	for k, e := range m.commitEntries(t, cs) {
		if e.commit == i && e.file == j {
			m.dfocus = true
			m.dcur = k
			m.moveDrawerCursor(0)
			if !m.preview {
				m.togglePreview()
			}
			return
		}
	}
}

// commitsOnlyKey handles the Commits tab's own keys: v turns the preview
// on and off, a opens the branch's whole diff in the diff tool, as on
// Files. In the focused drawer, d opens the commit or file under the
// cursor in the diff tool and g the commit on GitHub; space, → and l
// expand a commit to its files (space collapses it again), ← and h
// collapse it, also from one of its files. done is false for any other
// key, so these keys act as usual from the list (l is Linear there).
func (m *Model) commitsOnlyKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if m.mode != modeRow || m.curTab() != tabCommits {
		return nil, false
	}
	switch msg.String() {
	case "v":
		m.togglePreview()
		return nil, true
	case "a":
		if t, cs, ok := m.commits(); ok && cs.Note == "" && cs.MergeBase != "" {
			return m.openDiff(t, deck.Diff{Base: cs.Base, MergeBase: cs.MergeBase}, deck.DiffFile{}), true
		}
		return nil, true
	case "d":
		if m.dfocus {
			return m.openCommit(), true
		}
	case "g":
		if m.dfocus {
			return m.openCommitOnGitHub(), true
		}
	case "space", "right", "l", "left", "h":
		if !m.dfocus {
			return nil, false
		}
		t, cs, ok := m.commits()
		if !ok {
			return nil, true
		}
		e, ok := m.cursorEntry(t, cs)
		if !ok || e.commit < 0 {
			return nil, true
		}
		switch msg.String() {
		case "space":
			if e.file >= 0 {
				return m.toggleExpand(e.commit, 0), true
			}
			return m.toggleExpand(e.commit, -1), true
		case "right", "l":
			return m.toggleExpand(e.commit, 1), true
		}
		return m.toggleExpand(e.commit, 0), true
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
// against its first parent; on one of its files, only that file (a
// rename with both paths).
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
	var files []string
	if f, ok := m.cursorCommitFile(); ok {
		what = f.Path + " at " + c.Short
		if f.OldPath != "" {
			files = append(files, f.OldPath)
		}
		files = append(files, f.Path)
	}
	return func() tea.Msg { return openedMsg{what: what, err: open(path, sha, files)} }
}

// cursorCommitFile is the commit's file under the drawer cursor, if the
// cursor is on one.
func (m Model) cursorCommitFile() (deck.DiffFile, bool) {
	t, cs, ok := m.commits()
	if !ok {
		return deck.DiffFile{}, false
	}
	e, ok := m.cursorEntry(t, cs)
	if !ok || e.commit < 0 || e.file < 0 {
		return deck.DiffFile{}, false
	}
	files, _, _ := m.commitFileList(t, cs.List[e.commit])
	return files[e.file], true
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

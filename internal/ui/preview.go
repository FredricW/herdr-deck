package ui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/syntax"
)

// The diff preview: while the Files tab has the focus, v shows the
// highlighted file's diff where the list is, and the drawer below keeps
// moving between files; on the Commits tab it shows the highlighted
// commit, as `git show` does. The list's cursor and scroll stay as they
// were, so turning the preview off shows the list unchanged.

// preview is one file's diff or one commit ready to draw: the patch and
// each line's code, coloured by its file's language (empty for hunk
// headers, notes and file lines), and its lines laid out for the split
// layout (split.go). A commit's preview starts with its author, date and
// body.
type preview struct {
	patch  deck.Patch
	code   []string
	split  splitView
	commit *deck.Commit
	body   []string
}

type patchMsg struct {
	key  string
	data preview
}

// The tint under added and removed lines: subtle on a dark terminal,
// pale on a light one. True colour; the terminal's profile downsamples it.
const (
	addBgDark  = "48;2;22;54;33"
	delBgDark  = "48;2;64;26;31"
	addBgLight = "48;2;222;250;228"
	delBgLight = "48;2;255;228;226"
)

// patchKey names a file of a thread's diff.
func patchKey(t deck.Thread, f deck.DiffFile) string {
	return diffKey(t) + "\x00" + f.OldPath + "\x00" + f.Path
}

// commitKey names a commit of a thread's branch.
func commitKey(t deck.Thread, sha string) string {
	return "c\x00" + diffKey(t) + "\x00" + sha
}

// previewCommitAt is the commit the preview shows on the Commits tab: the
// one under the drawer cursor.
func (m Model) previewCommitAt() (deck.Thread, deck.Commit, bool) {
	t, cs, ok := m.commits()
	if !ok || cs.Note != "" {
		return t, deck.Commit{}, false
	}
	i := m.cursorCommit(cs)
	if i < 0 {
		return t, deck.Commit{}, false
	}
	return t, cs.List[i], true
}

// previewing is what the preview shows on the current tab: the thread
// and the patchKey or commitKey of the file or commit under the cursor.
func (m Model) previewing() (deck.Thread, string, bool) {
	switch m.curTab() {
	case tabFiles:
		if t, _, f, ok := m.previewFile(); ok {
			return t, patchKey(t, f), true
		}
	case tabCommits:
		if t, c, ok := m.previewCommitAt(); ok {
			if f, ok := m.cursorCommitFile(); ok {
				return t, commitKey(t, c.SHA) + "\x00" + f.OldPath + "\x00" + f.Path, true
			}
			return t, commitKey(t, c.SHA), true
		}
	}
	return deck.Thread{}, "", false
}

// previewFile is the file the preview shows: the one under the Files
// tab's cursor.
func (m Model) previewFile() (deck.Thread, deck.Diff, deck.DiffFile, bool) {
	t, d, ok := m.diff()
	if !ok || d.Note != "" || len(d.Files) == 0 {
		return t, d, deck.DiffFile{}, false
	}
	files := fileOrder(d.Files, m.tree)
	return t, d, files[clamp(m.dcur, 0, len(files)-1)], true
}

// togglePreview turns the preview on or off. Turning it on focuses the
// Files or Commits tab and gives the drawer its normal height, so the
// preview has the list's place.
func (m *Model) togglePreview() {
	if m.preview {
		m.preview = false
		return
	}
	commits := m.curTab() == tabCommits
	if !commits && m.opt.Patch == nil || commits && m.opt.CommitPatch == nil {
		m.status = "the diff preview is off"
		return
	}
	if commits {
		// From the uncommitted row, the preview starts at the newest
		// commit.
		if _, cs, ok := m.commits(); ok && m.cursorCommit(cs) < 0 && len(cs.List) > 0 {
			m.dcur = m.commitStop(cs, 0)
		}
	}
	if _, _, ok := m.previewing(); !ok {
		if commits {
			m.status = "no commit to preview"
		} else {
			m.status = "no file to preview"
		}
		return
	}
	if m.size != sizeNormal {
		m.size = sizeNormal
		m.drawerOff = 0
	}
	m.dfocus = true
	m.preview = true
	m.prevTab = m.curTab()
	m.prevThread, m.prevKey, m.prevOff = "", "", 0
	m.prevSplit = m.splitShown()
	m.syncPreview()
}

// syncPreview turns the preview off once its place is gone: the Files or
// Commits tab lost the focus, another view or row came up, or the drawer
// changed size. A file or commit other than the last one shown starts
// from its top.
func (m *Model) syncPreview() {
	if !m.preview {
		return
	}
	t, k, ok := m.previewing()
	switch {
	case !ok, m.mode != modeRow, !m.dfocus, m.curTab() != m.prevTab, m.effectiveSize() != sizeNormal,
		m.prevThread != "" && m.prevThread != diffKey(t):
		m.preview = false
		return
	}
	m.prevThread = diffKey(t)
	if k != m.prevKey {
		m.prevKey, m.prevOff = k, 0
		m.prevSplit = m.splitShown()
	}
	m.relayoutPreview()
	m.scrollPreview(0)
}

// readPatch reads the previewed file's diff, again when force is set, else
// only when the preview moved to another file. One read runs at a time;
// a move meanwhile reads again when it ends.
func (m *Model) readPatch(force bool) tea.Cmd {
	if !m.preview {
		return nil
	}
	t, k, ok := m.previewing()
	if !ok || !force && k == m.patchSel {
		return nil
	}
	if m.patching {
		m.patchAgain = true
		return nil
	}
	if f, ok := m.cursorCommitFile(); ok && m.curTab() == tabCommits {
		_, c, _ := m.previewCommitAt()
		read := m.opt.CommitFilePatch
		if read == nil {
			return nil
		}
		m.patching, m.patchSel = true, k
		if old, had := m.patches[k]; had {
			return func() tea.Msg { return patchMsg{key: k, data: old} }
		}
		return func() tea.Msg {
			return patchMsg{key: k, data: newPreview(f.Path, read(context.Background(), t, c.SHA, f))}
		}
	}
	if m.curTab() == tabCommits {
		_, c, _ := m.previewCommitAt()
		read := m.opt.CommitPatch
		if read == nil {
			return nil
		}
		m.patching, m.patchSel = true, k
		// A commit never changes: one read is enough.
		if old, had := m.patches[k]; had {
			return func() tea.Msg { return patchMsg{key: k, data: old} }
		}
		return func() tea.Msg {
			return patchMsg{key: k, data: newCommitPreview(c, read(context.Background(), t, c.SHA))}
		}
	}
	_, d, f, _ := m.previewFile()
	read := m.opt.Patch
	if read == nil {
		return nil
	}
	m.patching, m.patchSel = true, k
	old, had := m.patches[k]
	return func() tea.Msg {
		p := read(context.Background(), t, d.MergeBase, f)
		if had && samePatch(old.patch, p) {
			return patchMsg{key: k, data: old} // no need to colour it again
		}
		return patchMsg{key: k, data: newPreview(f.Path, p)}
	}
}

func samePatch(a, b deck.Patch) bool {
	return a.More == b.More && a.Cut == b.Cut && a.Binary == b.Binary && a.Note == b.Note && slices.Equal(a.Lines, b.Lines)
}

// newPreview colours a patch's code lines by the file's language. It
// runs with the read, off the UI goroutine.
func newPreview(path string, p deck.Patch) preview {
	return preview{patch: p, code: colourCode(path, p.Lines), split: pairLines(p.Lines)}
}

// newCommitPreview colours a commit's patch file by file, each by its own
// language, and keeps the body's lines. It runs with the read, off the UI
// goroutine.
func newCommitPreview(c deck.Commit, cp deck.CommitPatch) preview {
	code := make([]string, len(cp.Patch.Lines))
	start, path := 0, ""
	flush := func(end int) {
		copy(code[start:end], colourCode(path, cp.Patch.Lines[start:end]))
	}
	for i, l := range cp.Patch.Lines {
		if l.Kind == deck.LineFile {
			flush(i)
			start, path = i, l.Text
			if _, to, ok := strings.Cut(path, " → "); ok {
				path = to
			}
		}
	}
	flush(len(cp.Patch.Lines))
	var body []string
	if cp.Body != "" {
		for _, l := range strings.Split(cp.Body, "\n") {
			body = append(body, syntax.Clean(strings.TrimRight(l, " \r")))
		}
	}
	return preview{patch: cp.Patch, code: code, split: pairLines(cp.Patch.Lines), commit: &c, body: body}
}

// colourCode is lines' code coloured by the language of the file at
// path: one entry per line, empty for lines that are not code.
func colourCode(path string, lines []deck.PatchLine) []string {
	var idx []int
	var text []string
	for i, l := range lines {
		switch l.Kind {
		case deck.LineContext, deck.LineAdded, deck.LineDeleted:
			idx = append(idx, i)
			text = append(text, l.Text)
		}
	}
	code := make([]string, len(lines))
	for j, s := range syntax.Lines(path, text) {
		code[idx[j]] = s
	}
	return code
}

// previewKey handles the keys the preview takes over: J K scroll a line,
// pgup pgdn (ctrl+u ctrl+d) a page, S switches the layout (for the
// session; diff.layout is the one it starts with). done is false for any
// other key.
func (m *Model) previewKey(msg tea.KeyPressMsg) bool {
	if !m.preview {
		return false
	}
	page := max(m.layout().listH-1, 1)
	switch msg.String() {
	case "J", "shift+j", "shift+down":
		m.scrollPreview(1)
	case "K", "shift+k", "shift+up":
		m.scrollPreview(-1)
	case "pgdown", "ctrl+d":
		m.scrollPreview(page)
	case "pgup", "ctrl+u":
		m.scrollPreview(-page)
	case "home":
		m.prevOff = 0
	case "end":
		m.scrollPreview(1 << 30)
	case "S", "shift+s", "|":
		m.split = !m.split
		m.relayoutPreview()
	default:
		return false
	}
	return true
}

// previewTotal is how many lines the preview scrolls through: a commit's
// introduction, then the patch's, the cap's line, or the one line saying
// why there are none.
func (m Model) previewTotal() int {
	p, ok := m.patches[m.prevKey]
	if !ok {
		return 1
	}
	n := len(m.intro(p)) + m.patchRows(p)
	switch {
	case len(p.patch.Lines) == 0 || p.patch.Binary:
		n = len(m.intro(p)) + 1
	case p.patch.More > 0 || p.patch.Cut:
		n++
	}
	return n
}

// intro is the lines a commit's preview starts with: who wrote it and
// when, then its body, each followed by a blank line. A file's preview
// has none.
func (m Model) intro(p preview) []string {
	if p.commit == nil {
		return nil
	}
	c := p.commit
	who := c.Author
	if !c.Time.IsZero() {
		who += " · " + c.Time.In(m.loc()).Format("2006-01-02 15:04") + " · " + ageText(m.opt.Now().Sub(c.Time)) + " ago"
	}
	if c.Merge {
		who += " · merge, against its first parent"
	}
	lines := []string{" " + dim.Render(who), ""}
	if len(p.body) > 0 {
		for _, l := range p.body {
			lines = append(lines, " "+l)
		}
		lines = append(lines, "")
	}
	return lines
}

// scrollPreview moves the preview by delta lines, within its lines.
func (m *Model) scrollPreview(delta int) {
	h := m.layout().listH
	m.prevOff = clamp(m.prevOff+delta, 0, max(m.previewTotal()-h, 0))
}

// previewLines draws the preview in the list's place: a header line where
// the column titles were, then h lines of the diff.
func (m Model) previewLines(w, h int) []string {
	p, ok := m.patches[m.prevKey]
	var head string
	switch f, onFile := m.cursorCommitFile(); {
	case m.curTab() == tabCommits && onFile:
		_, c, _ := m.previewCommitAt()
		head = m.previewHeader(f, c.Short, ok, w, h)
	case m.curTab() == tabCommits:
		_, c, _ := m.previewCommitAt()
		head = m.commitHeader(c, ok, w, h)
	default:
		_, _, f, _ := m.previewFile()
		head = m.previewHeader(f, "", ok, w, h)
	}
	lines := []string{head}
	if !ok {
		lines = append(lines, dim.Render("  reading the diff…"))
	} else {
		// The lines scrolled through: a commit's introduction, then the
		// patch's lines (only the shown ones are drawn) or a note.
		intro := m.intro(p)
		var tail []string
		switch {
		case p.patch.Binary:
			tail = []string{dim.Render("  binary file: no lines to show")}
		case len(p.patch.Lines) == 0:
			note := p.patch.Note
			if note == "" {
				note = "no changes"
			}
			tail = []string{dim.Render("  " + note)}
		case p.patch.More > 0 || p.patch.Cut:
			more := fmt.Sprintf("  … %d more lines", p.patch.More)
			switch {
			case p.patch.Cut && p.patch.More == 0:
				more = "  … more lines not read"
			case p.patch.Cut:
				more = fmt.Sprintf("  … %d+ more lines", p.patch.More)
			}
			tail = []string{dim.Render(more)}
		}
		patch := m.patchRows(p)
		for i := m.prevOff; i < m.prevOff+h; i++ {
			switch {
			case i < len(intro):
				lines = append(lines, fit(intro[i], w))
			case i-len(intro) < patch:
				k := i - len(intro)
				lines = append(lines, fit(m.patchRow(p, k, w), w))
			case i-len(intro)-patch < len(tail):
				lines = append(lines, fit(tail[i-len(intro)-patch], w))
			}
		}
	}
	for len(lines) < h+1 {
		lines = append(lines, "")
	}
	return lines[:h+1]
}

// previewHeader names the file (after the short sha of the commit it is
// shown at, if any) with its +N −M, and at the right which
// lines show. A long path loses its start, so the counts stay.
func (m Model) previewHeader(f deck.DiffFile, sha string, ok bool, w, h int) string {
	counts := addedStyle.Render(fmt.Sprintf("+%d", f.Added)) + " " + deletedStyle.Render(fmt.Sprintf("−%d", f.Deleted))
	if f.Binary {
		counts = dim.Render("binary")
	}
	if f.Untracked {
		counts += dim.Render("  untracked")
	}
	right := m.layoutNote()
	if total := m.previewTotal(); ok && total > h {
		right += dim.Render(fmt.Sprintf("%d–%d/%d", m.prevOff+1, min(m.prevOff+h, total), total)) + " "
	}
	lead := " "
	if sha != "" {
		lead += dim.Render(sha) + " "
	}
	room := w - ansi.StringWidth(lead) - 2 - ansi.StringWidth(counts) - ansi.StringWidth(right) - 2
	name := truncateLeft(listName(f), max(room, 8))
	return spread(lead+bold.Render(name)+"  "+counts, right, w)
}

// commitHeader names the commit with its short sha, subject and +N −M,
// and at the right which lines show.
func (m Model) commitHeader(c deck.Commit, ok bool, w, h int) string {
	counts := addedStyle.Render(fmt.Sprintf("+%d", c.Added)) + " " + deletedStyle.Render(fmt.Sprintf("−%d", c.Deleted))
	if c.Merge {
		counts = dim.Render("merge")
	}
	right := m.layoutNote()
	if total := m.previewTotal(); ok && total > h {
		right += dim.Render(fmt.Sprintf("%d–%d/%d", m.prevOff+1, min(m.prevOff+h, total), total)) + " "
	}
	room := w - 1 - ansi.StringWidth(c.Short) - 1 - 2 - ansi.StringWidth(counts) - ansi.StringWidth(right) - 2
	subject := ansi.Truncate(syntax.Clean(c.Subject), max(room, 8), "…")
	return spread(" "+dim.Render(c.Short)+" "+bold.Render(subject)+"  "+counts, right, w)
}

// diffLine is one line of the diff: a green + or red − in the gutter and
// a tinted background under added and removed lines, the code coloured
// by its language; hunk headers and git's notes dim. Long lines are cut
// with …, never wrapped.
func (m Model) diffLine(l deck.PatchLine, code string, w int) string {
	switch l.Kind {
	case deck.LineFile:
		name := truncateLeft(syntax.Clean(l.Text), max(w-6, 8))
		return dim.Render(" ── ") + bold.Render(name) + " " + dim.Render(strings.Repeat("─", max(w-5-ansi.StringWidth(name), 0)))
	case deck.LineHunk:
		return dim.Render(ansi.Truncate(" "+syntax.Clean(l.Text), w, "…"))
	case deck.LineNote:
		return dim.Render(ansi.Truncate("   \\ "+syntax.Clean(l.Text), w, "…"))
	}
	gutter := "   "
	switch l.Kind {
	case deck.LineAdded:
		gutter = " " + addedStyle.Bold(true).Render("+") + " "
	case deck.LineDeleted:
		gutter = " " + deletedStyle.Bold(true).Render("−") + " "
	}
	s := fit(gutter+ansi.Truncate(code, max(w-3, 1), "…"), w)
	if bg := m.lineTint(l.Kind); bg != "" {
		return tint(s, bg)
	}
	return s
}

// lineTint is the background (SGR parameters) under an added or removed
// line on this terminal, "" for other lines.
func (m Model) lineTint(k deck.LineKind) string {
	switch {
	case k == deck.LineAdded && m.light:
		return addBgLight
	case k == deck.LineAdded:
		return addBgDark
	case k == deck.LineDeleted && m.light:
		return delBgLight
	case k == deck.LineDeleted:
		return delBgDark
	}
	return ""
}

// tint puts the background bg (SGR parameters) under s, set again after
// each of its resets, as highlight does for the selection.
func tint(s, bg string) string {
	on := "\x1b[" + bg + "m"
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+on)
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+on)
	return on + s + "\x1b[m"
}

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
// moving between files. The list's cursor and scroll stay as they were,
// so turning the preview off shows the list unchanged.

// preview is one file's diff ready to draw: the patch and each line's
// code, coloured by the file's language (empty for hunk headers and
// notes).
type preview struct {
	patch deck.Patch
	code  []string
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
// Files tab and gives the drawer its normal height, so the preview has
// the list's place.
func (m *Model) togglePreview() {
	if m.preview {
		m.preview = false
		return
	}
	if m.opt.Patch == nil {
		m.status = "the diff preview is off"
		return
	}
	if _, _, _, ok := m.previewFile(); !ok {
		m.status = "no file to preview"
		return
	}
	if m.size != sizeNormal {
		m.size = sizeNormal
		m.drawerOff = 0
	}
	m.dfocus = true
	m.preview = true
	m.prevThread, m.prevKey, m.prevOff = "", "", 0
	m.syncPreview()
}

// syncPreview turns the preview off once its place is gone: the Files tab
// lost the focus, another view or row came up, or the drawer changed
// size. A file other than the last one shown starts from its top.
func (m *Model) syncPreview() {
	if !m.preview {
		return
	}
	t, _, f, ok := m.previewFile()
	switch {
	case !ok, m.mode != modeRow, !m.dfocus, m.curTab() != tabFiles, m.effectiveSize() != sizeNormal,
		m.prevThread != "" && m.prevThread != diffKey(t):
		m.preview = false
		return
	}
	m.prevThread = diffKey(t)
	if k := patchKey(t, f); k != m.prevKey {
		m.prevKey, m.prevOff = k, 0
	}
	m.scrollPreview(0)
}

// readPatch reads the previewed file's diff, again when force is set, else
// only when the preview moved to another file. One read runs at a time;
// a move meanwhile reads again when it ends.
func (m *Model) readPatch(force bool) tea.Cmd {
	if !m.preview || m.opt.Patch == nil {
		return nil
	}
	t, d, f, ok := m.previewFile()
	if !ok {
		return nil
	}
	k := patchKey(t, f)
	if !force && k == m.patchSel {
		return nil
	}
	if m.patching {
		m.patchAgain = true
		return nil
	}
	m.patching, m.patchSel = true, k
	read := m.opt.Patch
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
	var idx []int
	var text []string
	for i, l := range p.Lines {
		switch l.Kind {
		case deck.LineContext, deck.LineAdded, deck.LineDeleted:
			idx = append(idx, i)
			text = append(text, l.Text)
		}
	}
	code := make([]string, len(p.Lines))
	for j, s := range syntax.Lines(path, text) {
		code[idx[j]] = s
	}
	return preview{patch: p, code: code}
}

// previewKey handles the keys the preview takes over: J K scroll a line,
// pgup pgdn (ctrl+u ctrl+d) a page. done is false for any other key.
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
	default:
		return false
	}
	return true
}

// previewTotal is how many lines the preview scrolls through: the
// patch's, the cap's line, or the one line saying why there are none.
func (m Model) previewTotal() int {
	p, ok := m.patches[m.prevKey]
	if !ok || len(p.patch.Lines) == 0 {
		return 1
	}
	n := len(p.patch.Lines)
	if p.patch.More > 0 || p.patch.Cut {
		n++
	}
	return n
}

// scrollPreview moves the preview by delta lines, within its lines.
func (m *Model) scrollPreview(delta int) {
	h := m.layout().listH
	m.prevOff = clamp(m.prevOff+delta, 0, max(m.previewTotal()-h, 0))
}

// previewLines draws the preview in the list's place: a header line where
// the column titles were, then h lines of the diff.
func (m Model) previewLines(w, h int) []string {
	_, _, f, _ := m.previewFile()
	p, ok := m.patches[m.prevKey]
	lines := []string{m.previewHeader(f, ok, w, h)}
	var body []string
	switch {
	case !ok:
		body = []string{dim.Render("  reading the diff…")}
	case p.patch.Binary:
		body = []string{dim.Render("  binary file: no lines to show")}
	case len(p.patch.Lines) == 0:
		note := p.patch.Note
		if note == "" {
			note = "no changes"
		}
		body = []string{dim.Render("  " + note)}
	default:
		end := min(m.prevOff+h, len(p.patch.Lines))
		for i := m.prevOff; i < end; i++ {
			body = append(body, m.diffLine(p.patch.Lines[i], p.code[i], w))
		}
		if len(body) < h && (p.patch.More > 0 || p.patch.Cut) {
			more := fmt.Sprintf("  … %d more lines", p.patch.More)
			switch {
			case p.patch.Cut && p.patch.More == 0:
				more = "  … more lines not read"
			case p.patch.Cut:
				more = fmt.Sprintf("  … %d+ more lines", p.patch.More)
			}
			body = append(body, dim.Render(more))
		}
	}
	for _, l := range body {
		lines = append(lines, fit(l, w))
	}
	for len(lines) < h+1 {
		lines = append(lines, "")
	}
	return lines[:h+1]
}

// previewHeader names the file with its +N −M, and at the right which
// lines show. A long path loses its start, so the counts stay.
func (m Model) previewHeader(f deck.DiffFile, ok bool, w, h int) string {
	counts := addedStyle.Render(fmt.Sprintf("+%d", f.Added)) + " " + deletedStyle.Render(fmt.Sprintf("−%d", f.Deleted))
	if f.Binary {
		counts = dim.Render("binary")
	}
	if f.Untracked {
		counts += dim.Render("  untracked")
	}
	right := ""
	if total := m.previewTotal(); ok && total > h {
		right = dim.Render(fmt.Sprintf("%d–%d/%d", m.prevOff+1, min(m.prevOff+h, total), total)) + " "
	}
	room := w - 1 - 2 - ansi.StringWidth(counts) - ansi.StringWidth(right) - 2
	name := truncateLeft(listName(f), max(room, 8))
	return spread(" "+bold.Render(name)+"  "+counts, right, w)
}

// diffLine is one line of the diff: a green + or red − in the gutter and
// a tinted background under added and removed lines, the code coloured
// by its language; hunk headers and git's notes dim. Long lines are cut
// with …, never wrapped.
func (m Model) diffLine(l deck.PatchLine, code string, w int) string {
	switch l.Kind {
	case deck.LineHunk:
		return dim.Render(ansi.Truncate(" "+syntax.Clean(l.Text), w, "…"))
	case deck.LineNote:
		return dim.Render(ansi.Truncate("   \\ "+syntax.Clean(l.Text), w, "…"))
	}
	gutter, bg := "   ", ""
	switch l.Kind {
	case deck.LineAdded:
		gutter, bg = " "+addedStyle.Bold(true).Render("+")+" ", addBgDark
		if m.light {
			bg = addBgLight
		}
	case deck.LineDeleted:
		gutter, bg = " "+deletedStyle.Bold(true).Render("−")+" ", delBgDark
		if m.light {
			bg = delBgLight
		}
	}
	s := fit(gutter+ansi.Truncate(code, max(w-3, 1), "…"), w)
	if bg == "" {
		return s
	}
	return tint(s, bg)
}

// tint puts the background bg (SGR parameters) under s, set again after
// each of its resets, as highlight does for the selection.
func tint(s, bg string) string {
	on := "\x1b[" + bg + "m"
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+on)
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+on)
	return on + s + "\x1b[m"
}

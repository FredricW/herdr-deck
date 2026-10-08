package ui

import (
	"context"
	"fmt"
	"path"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/arch"
)

// The selected edge's code: where the import that makes the edge is
// written, drawn the way the diff preview draws a hunk (its colours,
// gutter and layout), in the deck's main view while the edge is selected
// and below or beside the canvas in the Impact pane. An import the branch
// added or removed shows the hunk that holds it; one whose line did not
// change (an unchanged edge, or a changed one whose importers only use
// other names) shows the file around it, untinted. The import's row is
// marked ▌ in the edge's status colour (▶ is drawn as an emoji by some
// terminals).

// edgeContext is how many lines above and below an unchanged import show.
const edgeContext = 5

// edgeView is the code at one site of an edge, ready to draw.
type edgeView struct {
	p    preview
	mark int    // the patch line of the import, -1 when it was not found
	note string // "import unchanged", or why there is no code to show
}

type edgeViewMsg struct {
	key string
	v   edgeView
}

// edgeCodes are the edge views read, the one wanted and the read running.
type edgeCodes struct {
	views   map[string]edgeView
	want    string
	reading bool
	again   bool
}

// keep stores a view, starting over once there are many.
func (ec *edgeCodes) keep(key string, v edgeView) {
	if ec.views == nil || len(ec.views) > 32 {
		ec.views = map[string]edgeView{}
	}
	ec.views[key] = v
	ec.reading = false
}

// edgeSiteKey names a site of a result's edge for the cache.
func edgeSiteKey(t deck.Thread, r *arch.Result, s arch.Site) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s:%d:%t", diffKey(t), r.MergeBase, r.Head, s.File, s.Line, s.Base)
}

// edgeReaders are what reading an edge view needs; any may be nil.
type edgeReaders struct {
	patch  func(ctx context.Context, t deck.Thread, mergeBase string, f deck.DiffFile) deck.Patch
	fileAt func(ctx context.Context, t deck.Thread, rev, path string) ([]byte, error)
}

// readEdgeView reads site s of an edge of r in t's worktree: the hunk of
// the file's diff (d) that holds its line, else the file around the line
// as the commit (the merge-base for a removed import, else the head) has
// it. It runs off the UI goroutine.
func readEdgeView(ctx context.Context, rd edgeReaders, t deck.Thread, d deck.Diff, r *arch.Result, s arch.Site) edgeView {
	for _, f := range d.Files {
		if (f.Path != s.File && f.OldPath != s.File) || rd.patch == nil {
			continue
		}
		p := rd.patch(ctx, t, d.MergeBase, f)
		if hunk, mark, ok := hunkAt(p, s.Line, s.Base); ok {
			return edgeView{p: newPreview(s.File, hunk), mark: mark}
		}
	}
	rev := r.Head
	if s.Base {
		rev = r.MergeBase
	}
	if rd.fileAt == nil {
		return edgeView{mark: -1, note: "no code to show for " + s.File}
	}
	src, err := rd.fileAt(ctx, t, rev, s.File)
	if err != nil {
		return edgeView{mark: -1, note: fmt.Sprintf("%s: %v", s.File, err)}
	}
	p, mark := contextPatch(src, s.Line, edgeContext)
	return edgeView{p: newPreview(s.File, p), mark: mark, note: "import unchanged"}
}

// hunkAt is the hunk of p holding line (of the old file with base, else
// the new one), with the index of that line in it.
func hunkAt(p deck.Patch, line int, base bool) (deck.Patch, int, bool) {
	start, at := -1, -1
	var o, n int
	for i, l := range p.Lines {
		switch l.Kind {
		case deck.LineFile:
			start = -1
		case deck.LineHunk:
			if at >= 0 {
				return deck.Patch{Lines: p.Lines[start:i]}, at - start, true
			}
			h, ok := parseHunk(l.Text)
			if !ok {
				start = -1
				continue
			}
			start, o, n = i, h.oldStart, h.newStart
		case deck.LineContext:
			if start >= 0 && at < 0 && (base && o == line || !base && n == line) {
				at = i
			}
			o++
			n++
		case deck.LineAdded:
			if start >= 0 && at < 0 && !base && n == line {
				at = i
			}
			n++
		case deck.LineDeleted:
			if start >= 0 && at < 0 && base && o == line {
				at = i
			}
			o++
		}
	}
	if at < 0 {
		return deck.Patch{}, -1, false
	}
	return deck.Patch{Lines: p.Lines[start:]}, at - start, true
}

// contextPatch is the file src around line, around lines each way, as a
// patch of context lines under one hunk header, and the line's index.
func contextPatch(src []byte, line, around int) (deck.Patch, int) {
	lines := strings.Split(strings.TrimSuffix(string(src), "\n"), "\n")
	line = clamp(line, 1, len(lines))
	from, to := max(line-around, 1), min(line+around, len(lines))
	p := deck.Patch{Lines: []deck.PatchLine{{Kind: deck.LineHunk, Text: fmt.Sprintf("@@ -%d,%d +%d,%d @@", from, to-from+1, from, to-from+1)}}}
	for i := from; i <= to; i++ {
		p.Lines = append(p.Lines, deck.PatchLine{Kind: deck.LineContext, Text: lines[i-1]})
	}
	return p, 1 + line - from
}

// edgeHeader is the view's first line: the edge, its status and sites,
// for a changed edge the names gained and lost, and at the right which
// site shows and where.
func edgeHeader(e arch.Edge, v edgeView, read bool, site, w int) string {
	st := edgeStyle(e.Status).style()
	left := " " + st.Bold(true).Render(arch.Short(e.From)+" "+edgeLine(e.Status)+"▸ "+arch.Short(e.To)) +
		dim.Render(" · "+edgeWord(e.Status))
	if n := len(e.Sites); n == 1 {
		left += dim.Render(" · 1 site")
	} else {
		left += dim.Render(fmt.Sprintf(" · %d sites", n))
	}
	for _, g := range e.Gained {
		left += " " + okStyle.Render("+"+g)
	}
	for _, l := range e.Lost {
		left += " " + failStyle.Render("−"+l)
	}
	right := ""
	if site < len(e.Sites) {
		s := e.Sites[site]
		right = fmt.Sprintf("%s:%d", path.Base(s.File), s.Line)
		if len(e.Sites) > 1 {
			right = fmt.Sprintf("site %d/%d · %s", site+1, len(e.Sites), right)
		}
		if read && v.note != "" {
			right = v.note + " · " + right
		}
		right = dim.Render(right) + " "
	}
	return spread(left, right, w)
}

// edgeLines draws an edge view in h+1 lines w wide: the header, then the
// code with the import's row marked and in view, and a scrollbar when it
// overflows. m draws the rows, as the diff preview does: its layout and
// colours.
func (m Model) edgeLines(e arch.Edge, v edgeView, read bool, site, w, h int) []string {
	lines := []string{edgeHeader(e, v, read, site, w)}
	switch {
	case !read:
		lines = append(lines, dim.Render("  reading the code…"))
	case len(v.p.patch.Lines) == 0:
		lines = append(lines, dim.Render("  "+v.note))
	default:
		rows := m.layoutRows(v.p)
		at := 0
		for k, r := range rows {
			if v.mark >= 0 && (r.full == v.mark || r.l == v.mark || r.r == v.mark) {
				at = k
				break
			}
		}
		total := len(rows)
		off := clamp(at-h/3, 0, max(total-h, 0))
		bar := scrollbar{total: total, h: h, off: off}
		cw := w
		if bar.shown() {
			cw--
		}
		mark := edgeStyle(e.Status).style().Bold(true).Render("▌")
		var body []string
		for k := off; k < min(off+h, total); k++ {
			row := fit(m.patchRow(v.p, k, cw), cw)
			if r := rows[k]; v.mark >= 0 && (r.full == v.mark || r.l == v.mark || r.r == v.mark) {
				row = mark + ansi.Cut(row, 1, cw)
			}
			body = append(body, row)
		}
		for len(body) < h {
			body = append(body, "")
		}
		if bar.shown() {
			body = withBar(body, bar, w, false, m.light)
		}
		lines = append(lines, body...)
	}
	for len(lines) < h+1 {
		lines = append(lines, "")
	}
	return lines[:h+1]
}

// ---- the deck's side

// impactEdge is the edge selected on the Impact tab (the drawer cursor on
// one of the detail's edge rows), with its result and thread.
func (m Model) impactEdge() (deck.Thread, *arch.Result, arch.Edge, bool) {
	if m.mode != modeRow || m.curTab() != tabImpact || !m.dfocus {
		return deck.Thread{}, nil, arch.Edge{}, false
	}
	t, r, ok := m.archResult()
	if !ok || m.iedge == "" || m.iselKey != diffKey(t) {
		return t, nil, arch.Edge{}, false
	}
	for _, e := range r.Edges {
		if edgeKey(e) == m.iedge {
			return t, r, e, true
		}
	}
	return t, r, arch.Edge{}, false
}

// edgeSite is the shown site of the selected edge: its index, kept per
// edge (a new edge starts at its first).
func (m Model) edgeSite(e arch.Edge) int {
	if m.esiteFor != edgeKey(e) || len(e.Sites) == 0 {
		return 0
	}
	return clamp(m.esite, 0, len(e.Sites)-1)
}

// edgeShown says the main view shows the selected edge's code.
func (m Model) edgeShown() bool {
	_, _, e, ok := m.impactEdge()
	return ok && len(e.Sites) > 0
}

// readEdge reads the selected edge's shown site, unless it was read; one
// read runs at a time, and a move meanwhile reads again when it ends.
func (m *Model) readEdge() tea.Cmd {
	t, r, e, ok := m.impactEdge()
	if !ok || len(e.Sites) == 0 {
		return nil
	}
	s := e.Sites[m.edgeSite(e)]
	key := edgeSiteKey(t, r, s)
	if _, done := m.ecode.views[key]; done {
		return nil
	}
	if m.ecode.reading {
		m.ecode.again = true
		return nil
	}
	m.ecode.reading, m.ecode.want = true, key
	_, d, _ := m.diff()
	rd := edgeReaders{patch: m.opt.Patch, fileAt: m.opt.FileAt}
	return func() tea.Msg { return edgeViewMsg{key: key, v: readEdgeView(context.Background(), rd, t, d, r, s)} }
}

// shownEdgeLines draws the selected edge's code in the list's place.
func (m Model) shownEdgeLines(w, h int) []string {
	t, r, e, _ := m.impactEdge()
	site := m.edgeSite(e)
	v, read := m.ecode.views[edgeSiteKey(t, r, e.Sites[site])]
	return m.edgeLines(e, v, read, site, w, h)
}

// cycleSite shows the selected edge's next (dir 1) or previous site.
func (m *Model) cycleSite(e arch.Edge, dir int) {
	n := len(e.Sites)
	if n < 2 {
		m.status = "this import has one site"
		return
	}
	m.esite = (m.edgeSite(e) + dir + n) % n
	m.esiteFor = edgeKey(e)
}

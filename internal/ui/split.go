package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/syntax"
)

// The diff preview's split layout: the old file on the left and the new
// one on the right, each with its line numbers, the way side-by-side diff
// tools show a change. Within a hunk, a run of removed lines sits next to
// the added lines that follow it, and the shorter side gets blank filler;
// context shows on both sides. Hunk headers and a commit's file rules
// span both sides. An added file shows only its new side, a deleted one
// only its old side, across the whole width. S switches between this and
// the unified layout; diff.layout picks the one the deck starts with.

// splitMinWidth is the narrowest preview that shows the split layout: two
// sides of 49 columns, about 40 of them code after the line numbers and
// the sign. A narrower pane shows the unified layout and says so in the
// preview's header, and goes back to split once it is wide enough.
const splitMinWidth = 100

// sideRow is one row of the split layout: a line across both sides, or a
// patch line on the left and one on the right (-1 is filler).
type sideRow struct {
	full int // a hunk header or a commit's file line, drawn across; else -1
	l, r int // the patch lines on the left and right, -1 for none
	// only is -1 when the row's file has only an old side (it was
	// deleted), 1 when it has only a new one (it was added), else 0.
	only int
}

// anchor is the first patch line the row shows, by which a scroll
// position carries over between the layouts.
func (r sideRow) anchor() int {
	switch {
	case r.full >= 0:
		return r.full
	case r.l >= 0:
		return r.l
	}
	return r.r
}

// splitView is a patch laid out for the split layout.
type splitView struct {
	rows []sideRow
	// oldNo and newNo are each patch line's number in the old and the new
	// file, 0 for none.
	oldNo, newNo []int
	numW         int // the widest line number's digits
}

// pairLines lays a patch's lines out side by side: within a hunk each run
// of removed lines pairs up with the added lines after it, line by line;
// git's "\ No newline" note goes on the side of the line before it.
func pairLines(lines []deck.PatchLine) splitView {
	sv := splitView{oldNo: make([]int, len(lines)), newNo: make([]int, len(lines))}
	only := onlySides(lines)
	var dels, adds []int
	flush := func() {
		for k := range max(len(dels), len(adds)) {
			r := sideRow{full: -1, l: -1, r: -1}
			if k < len(dels) {
				r.l = dels[k]
			}
			if k < len(adds) {
				r.r = adds[k]
			}
			r.only = only[r.anchor()]
			sv.rows = append(sv.rows, r)
		}
		dels, adds = dels[:0], adds[:0]
	}
	both := func(i int) {
		flush()
		sv.rows = append(sv.rows, sideRow{full: -1, l: i, r: i, only: only[i]})
	}
	oldN, newN, maxN := 0, 0, 0
	last := deck.LineContext
	for i, l := range lines {
		switch l.Kind {
		case deck.LineHunk, deck.LineFile:
			flush()
			oldN, newN = 0, 0
			if h, ok := parseHunk(l.Text); ok {
				oldN, newN = h.oldStart, h.newStart
			}
			sv.rows = append(sv.rows, sideRow{full: i, l: -1, r: -1})
		case deck.LineContext:
			sv.oldNo[i], sv.newNo[i] = oldN, newN
			oldN, newN = oldN+1, newN+1
			both(i)
		case deck.LineDeleted:
			if len(adds) > 0 {
				flush() // a removal after additions starts a new run
			}
			sv.oldNo[i] = oldN
			oldN++
			dels = append(dels, i)
		case deck.LineAdded:
			sv.newNo[i] = newN
			newN++
			adds = append(adds, i)
		case deck.LineNote:
			switch last {
			case deck.LineDeleted:
				dels = append(dels, i)
			case deck.LineAdded:
				adds = append(adds, i)
			default:
				both(i)
			}
			continue // the note belongs to the line before it
		}
		maxN = max(maxN, oldN-1, newN-1)
		last = l.Kind
	}
	flush()
	sv.numW = len(strconv.Itoa(max(maxN, 1)))
	return sv
}

// rowOf is the row that shows patch line k, or the one after.
func (sv splitView) rowOf(k int) int {
	return max(sort.Search(len(sv.rows), func(i int) bool { return sv.rows[i].anchor() > k })-1, 0)
}

// hunk is a hunk header's ranges.
type hunk struct{ oldStart, oldCount, newStart, newCount int }

// parseHunk reads `@@ -a,b +c,d @@`; a missing count is 1.
func parseHunk(s string) (hunk, bool) {
	f := strings.Fields(s)
	if len(f) < 3 || f[0] != "@@" {
		return hunk{}, false
	}
	rng := func(t string, sign byte) (int, int, bool) {
		if len(t) < 2 || t[0] != sign {
			return 0, 0, false
		}
		a, b, cut := strings.Cut(t[1:], ",")
		start, err := strconv.Atoi(a)
		if err != nil {
			return 0, 0, false
		}
		n := 1
		if cut {
			if n, err = strconv.Atoi(b); err != nil {
				return 0, 0, false
			}
		}
		return start, n, true
	}
	var h hunk
	var ok1, ok2 bool
	h.oldStart, h.oldCount, ok1 = rng(f[1], '-')
	h.newStart, h.newCount, ok2 = rng(f[2], '+')
	return h, ok1 && ok2
}

// onlySides says for each line whether its file has only a new side (1:
// every hunk starts at -0,0, the file was added), only an old one (-1:
// every hunk ends at +0,0, it was deleted) or both (0). A commit's patch
// has several files, each from its LineFile line.
func onlySides(lines []deck.PatchLine) []int {
	out := make([]int, len(lines))
	start := 0
	mark := func(end int) {
		added, deleted, hunks := true, true, 0
		for _, l := range lines[start:end] {
			if l.Kind != deck.LineHunk {
				continue
			}
			hunks++
			h, ok := parseHunk(l.Text)
			added = added && ok && h.oldStart == 0 && h.oldCount == 0
			deleted = deleted && ok && h.newStart == 0 && h.newCount == 0
		}
		side := 0
		switch {
		case hunks == 0:
		case added:
			side = 1
		case deleted:
			side = -1
		}
		for i := start; i < end; i++ {
			out[i] = side
		}
	}
	for i, l := range lines {
		if l.Kind == deck.LineFile && i > start {
			mark(i)
			start = i
		}
	}
	mark(len(lines))
	return out
}

// splitShown says whether the preview draws the split layout: it is
// chosen and the pane is wide enough.
func (m Model) splitShown() bool { return m.split && m.width >= splitMinWidth }

// patchRows is how many rows the patch's lines take in the layout shown.
func (m Model) patchRows(p preview) int {
	if p.patch.Binary {
		return 0
	}
	if m.splitShown() {
		return len(p.split.rows)
	}
	return len(p.patch.Lines)
}

// patchRow draws row k of the patch in the layout shown.
func (m Model) patchRow(p preview, k, w int) string {
	if !m.splitShown() {
		return m.diffLine(p.patch.Lines[k], p.code[k], w)
	}
	r := p.split.rows[k]
	switch {
	case r.full >= 0:
		return m.diffLine(p.patch.Lines[r.full], "", w)
	case r.only < 0:
		return m.sideCell(p, r.l, true, w)
	case r.only > 0:
		return m.sideCell(p, r.r, false, w)
	}
	lw := (w - 1) / 2
	return m.sideCell(p, r.l, true, lw) + dim.Render("│") + m.sideCell(p, r.r, false, w-1-lw)
}

// sideCell draws patch line i on the old (left) or new side in w
// columns: its line number dim, then like a unified line, a red − or
// green + and the tint under removed and added code. Long lines are cut
// with …; i < 0 is blank filler.
func (m Model) sideCell(p preview, i int, old bool, w int) string {
	if i < 0 {
		return strings.Repeat(" ", max(w, 0))
	}
	sv := p.split
	l := p.patch.Lines[i]
	if l.Kind == deck.LineNote {
		return fit(strings.Repeat(" ", sv.numW+2)+dim.Render("\\ "+syntax.Clean(l.Text)), w)
	}
	no := sv.newNo[i]
	if old {
		no = sv.oldNo[i]
	}
	num := " " + dim.Render(fmt.Sprintf("%*d", sv.numW, no)) + " "
	rest := max(w-sv.numW-2, 1)
	sign := "  "
	switch l.Kind {
	case deck.LineAdded:
		sign = addedStyle.Bold(true).Render("+") + " "
	case deck.LineDeleted:
		sign = deletedStyle.Bold(true).Render("−") + " "
	}
	// The code keeps a blank column at its end, before the divider.
	s := fit(sign+ansi.Truncate(p.code[i], max(rest-3, 1), "…"), rest)
	if bg := m.lineTint(l.Kind); bg != "" {
		s = tint(s, bg)
	}
	return fit(num+s, w)
}

// relayoutPreview keeps the preview near the same hunk when the layout
// shown changes (S, a saved diff.layout, or the pane crossing
// splitMinWidth): prevOff counts rows of the layout prevSplit names.
func (m *Model) relayoutPreview() {
	split := m.splitShown()
	if split == m.prevSplit {
		return
	}
	m.prevSplit = split
	p, ok := m.patches[m.prevKey]
	if !ok || p.patch.Binary {
		m.scrollPreview(0)
		return
	}
	intro := len(m.intro(p))
	if k := m.prevOff - intro; k > 0 {
		lines, rows := len(p.patch.Lines), len(p.split.rows)
		switch {
		case split && k < lines:
			m.prevOff = intro + p.split.rowOf(k)
		case split:
			m.prevOff = intro + rows + k - lines // in the tail
		case k < rows:
			m.prevOff = intro + p.split.rows[k].anchor()
		default:
			m.prevOff = intro + lines + k - rows
		}
	}
	m.scrollPreview(0)
}

// layoutNote is what the preview's header says about its layout: why a
// chosen split layout shows unified.
func (m Model) layoutNote() string {
	if m.split && !m.splitShown() {
		return dim.Render(fmt.Sprintf("split needs ≥ %d cols", splitMinWidth)) + "  "
	}
	return ""
}

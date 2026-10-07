package ui

import (
	"context"
	"fmt"
	"path"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/arch"
)

// The Impact tab: what the thread's branch did to its repository's shape
// (internal/source/arch). It opens on Look here first, the changes that
// alter the shape, riskiest first, and under it the canvas of the
// impacted packages. The tab is named for what it answers rather than for
// architecture alone: the call-stack view (flows) is meant to join it.

type archMsg struct {
	key string
	res *arch.Result
}

// archResult is the selected thread's last architecture read, when one
// was read; the tab needs a worktree, as Files does.
func (m Model) archResult() (deck.Thread, *arch.Result, bool) {
	t, ok := m.diffThread()
	if !ok || m.opt.Arch == nil {
		return t, nil, false
	}
	r, ok := m.archs[diffKey(t)]
	return t, r, ok
}

// readArch reads the selected thread's architecture, again when force is
// set (the reader answers from its cache until HEAD moves), else only when
// the selection moved to another thread. One read runs at a time; a move
// meanwhile reads again when it ends.
func (m *Model) readArch(force bool) tea.Cmd {
	t, ok := m.diffThread()
	if m.opt.Arch == nil || !ok {
		return nil
	}
	k := diffKey(t)
	if !force && k == m.archSel {
		return nil
	}
	if m.archReading {
		m.archAgain = true
		return nil
	}
	m.archReading, m.archSel = true, k
	read := m.opt.Arch
	return func() tea.Msg { return archMsg{key: k, res: read(context.Background(), t)} }
}

func (m *Model) setArch(msg archMsg) tea.Cmd {
	m.archReading = false
	m.archs[msg.key] = msg.res
	if m.archAgain {
		m.archAgain = false
		return m.readArch(true)
	}
	return nil
}

// impactLabel is the tab's label: Impact and the number of things to look
// at, ≡ for a change that only moved code, … until the first read.
func (m Model) impactLabel() string {
	_, r, ok := m.archResult()
	switch {
	case !ok:
		return "Impact …"
	case r.Note != "":
		return "Impact"
	case r.PureMove():
		return "Impact ≡"
	}
	return fmt.Sprintf("Impact %d", len(r.Findings))
}

// Look here first shows this many findings at the drawer's normal height,
// and this many at full height; a line counts the rest.
const (
	findingsNormal = 5
	findingsFull   = 12
)

// impactTab fills the Impact tab.
func (m Model) impactTab(d *drawer) {
	_, r, ok := m.archResult()
	if !ok {
		d.line(" " + dim.Render("reading the change's shape…"))
		return
	}
	limit := findingsNormal
	if m.effectiveSize() == sizeFull {
		limit = findingsFull
	}
	impactLines(d, r, limit)
}

// impactLines writes a result's Look here first, at most limit findings,
// and its canvas into d.
func impactLines(d *drawer, r *arch.Result, limit int) {
	w := d.width
	if r.Note != "" {
		d.line(" " + dim.Render(r.Note))
		return
	}
	c := newCanvas(r, canvasWidth(w))
	pre := c.labelPrefix(r)
	d.line(impactSummary(r, w))
	if r.PureMove() {
		d.line(" " + dim.Render(fmt.Sprintf("%d files and %d declarations moved, %d renamed; no edge, API or touchpoint changed",
			r.Moves.Files, r.Moves.Decls, r.Moves.Renamed)))
	}
	if len(r.Findings) == 0 && !r.PureMove() {
		d.line(" " + dim.Render("nothing changed the shape"))
	}
	for i, f := range r.Findings {
		if i == limit {
			d.line(" " + dim.Render(fmt.Sprintf("⋯ %d more", len(r.Findings)-limit)))
			break
		}
		d.line(findingLine(f, i, pre, w))
	}
	d.line("")
	rows := c.rows()
	if w >= legendBesideMin {
		legend := c.legend(r, legendWidth-1)
		for i := range max(len(rows), len(legend)) {
			left := strings.Repeat(" ", c.g.w)
			if i < len(rows) {
				left = rows[i]
			}
			right := ""
			if i < len(legend) {
				right = legend[i]
			}
			d.line(" " + left + strings.Repeat(" ", max(w-1-c.g.w-legendWidth, 0)+1) + right)
		}
	} else {
		for _, row := range rows {
			d.line(" " + row)
		}
		for _, l := range c.legend(r, w-2) {
			d.line(" " + l)
		}
	}
	if r.Inferred {
		d.line(" " + dim.Render(abbrev("layers inferred: declare them in .config/dev.json, x-herdr-deck.architecture", w-2)))
	}
}

// The legend sits beside the canvas from legendBesideMin columns on,
// legendWidth wide; below it otherwise.
const (
	legendBesideMin = 110
	legendWidth     = 40
)

// canvasWidth is the canvas's width in a view w columns wide: one column
// of margin on the left, and the legend's beside it when wide enough.
func canvasWidth(w int) int {
	if w >= legendBesideMin {
		return w - 1 - legendWidth - 1
	}
	return w - 2
}

// impactSummary is the tab's first line: how many things to look at and
// their worst kinds, the edge counts, and at the right what was compared.
func impactSummary(r *arch.Result, w int) string {
	var up, skip int
	for _, e := range r.Edges {
		if e.Status == arch.Added {
			switch e.Verdict {
			case arch.VerdictUp:
				up++
			case arch.VerdictSkip:
				skip++
			}
		}
	}
	parts := []string{bold.Render(fmt.Sprintf("%d to look at", len(r.Findings)))}
	if r.PureMove() {
		parts = []string{workStyle.Bold(true).Render("≡ pure move")}
	}
	if up > 0 {
		parts = append(parts, failStyle.Bold(true).Render(fmt.Sprintf("✕ %d upward", up)))
	}
	if n := len(r.Cycles); n > 0 {
		parts = append(parts, failStyle.Render(fmt.Sprintf("↻ %d cycle", n)))
	}
	if skip > 0 {
		parts = append(parts, warnStyle.Render(fmt.Sprintf("⚠ %d skip", skip)))
	}
	a, rm, ch := r.ChangedEdges()
	parts = append(parts, fmt.Sprintf("edges %s %s %s", okStyle.Render(fmt.Sprintf("+%d", a)),
		failStyle.Render(fmt.Sprintf("−%d", rm)), warnStyle.Render(fmt.Sprintf("~%d", ch))))
	right := ""
	switch {
	case w >= 100:
		right = dim.Render(fmt.Sprintf("vs %s · %s", r.Base, langName(r.Language)))
	case w >= wideMin:
		right = dim.Render("vs " + r.Base)
	}
	return " " + spread(strings.Join(parts, "  "), right, w-2)
}

func langName(l string) string {
	if l == arch.LangTS {
		return "TypeScript"
	}
	return "Go"
}

// findingLine is one Look here first row: its number (· after 9), its
// glyph, what it is and why, and at the right where it is written.
func findingLine(f arch.Finding, i int, pre string, w int) string {
	num := dim.Render(" ·")
	if i < 9 {
		num = bold.Render(fmt.Sprintf("%2d", i+1))
	}
	glyph := findingGlyph(f)
	text := f.Text
	if pre != "" && f.Kind == arch.FindEdge {
		from, to, _ := strings.Cut(text, " → ")
		text = strings.TrimPrefix(from, pre) + " → " + strings.TrimPrefix(to, pre)
	}
	if f.Kind == arch.FindTouch || f.Kind == arch.FindAPI {
		text = strings.Replace(text, pre, "", 1)
	}
	why := f.Why
	switch {
	case f.Kind == arch.FindTouch:
		why = "in " + strings.TrimPrefix(strings.TrimPrefix(f.Why, "in "), pre)
	case f.Verdict == arch.VerdictUp:
		why = "upward"
	}
	left := num + " " + glyph + " " + text
	if why != "" {
		left += "  " + dim.Render(why)
	}
	right := ""
	if f.Site.File != "" && w >= wideMin {
		right = dim.Render(fmt.Sprintf("%s:%d", path.Base(f.Site.File), f.Site.Line))
	}
	return spread(left, right, w-1)
}

// findingGlyph marks a finding: ✕ upward, ⚠ a layer skip, ↻ a cycle, else
// its sign in the colour of what it did.
func findingGlyph(f arch.Finding) string {
	switch f.Verdict {
	case arch.VerdictUp:
		return failStyle.Bold(true).Render("✕")
	case arch.VerdictSkip:
		return warnStyle.Bold(true).Render("⚠")
	}
	switch f.Sign {
	case "↻", "−":
		return failStyle.Render(f.Sign)
	case "~":
		return warnStyle.Render(f.Sign)
	case "≡":
		return workStyle.Render(f.Sign)
	}
	return okStyle.Render(f.Sign)
}

package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// noWorkspace says why a bare Linear ID has no link.
const noWorkspace = "no Linear workspace: set linear_workspace in the config file, --linear-workspace or $" + deck.EnvLinearWorkspace

// overview is the Overview tab: titled sections in a fixed order, Next,
// PR, Note, Links, Dev and Thread, the empty ones left out. A blank line
// goes between them only when they all fit in h lines.
func (m Model) overview(d *drawer, r row, links []deck.Link, h int) {
	t, hasThread := r.thread()
	narrow := d.width < wideMin
	prLink := -1
	if hasThread && t.PR != nil {
		for i, l := range links {
			if l.Kind == deck.LinkGitHub && l.URL == t.PR.URL {
				prLink = i
			}
		}
	}

	type sec struct {
		name  string
		style lipgloss.Style
		body  func()
	}
	var secs []sec
	if hasThread && len(t.Next) > 0 {
		st := bold
		if t.Status == deck.StatusNeedsYou {
			st = needsStyle
		}
		secs = append(secs, sec{"Next", st, func() {
			for _, n := range t.Next {
				g := words(n, plain)
				g.items = append([]item{span("→", bold)}, g.items...)
				g.hang = 2
				d.flow(g)
			}
		}})
	}
	if hasThread && t.PR != nil {
		secs = append(secs, sec{"PR", bold, func() { m.prSection(d, *t.PR, links, prLink, narrow) }})
	}
	if r.task != nil && r.task.Notes != "" {
		secs = append(secs, sec{"Note", bold, func() {
			for _, n := range strings.Split(r.task.Notes, "\n") {
				d.flow(words(n, plain))
			}
		}})
	}
	if g, unlinked := m.chips(d, links, prLink, func(l deck.Link) bool { return l.Kind != deck.LinkLocalhost }); len(g.items) > 0 {
		secs = append(secs, sec{"Links", bold, func() {
			g, _ := m.chips(d, links, prLink, func(l deck.Link) bool { return l.Kind != deck.LinkLocalhost })
			d.flow(g)
			if unlinked {
				d.flow(words(noWorkspace, dim))
			}
		}})
	}
	local, _ := m.chips(d, links, prLink, func(l deck.Link) bool { return l.Kind == deck.LinkLocalhost })
	hasDev := hasThread && (len(t.DevServers) > 0 || t.DevNote != "" || t.DevUp != nil)
	if hasDev || len(local.items) > 0 {
		secs = append(secs, sec{"Dev", bold, func() {
			if hasDev {
				g := devGroup(t)
				if narrow && g.sep == "   " {
					g.sep = "  "
				}
				if t.DevUp == nil && len(t.DevServers) == 0 && strings.HasPrefix(t.DevNote, "not started") {
					g.items = append(g.items, span("·", dim), span("u starts it", dim))
				}
				d.flow(g)
			}
			g, _ := m.chips(d, links, prLink, func(l deck.Link) bool { return l.Kind == deck.LinkLocalhost })
			d.flow(g)
		}})
	}
	if hasThread {
		secs = append(secs, sec{"Thread", bold, func() { m.threadSection(d, r, t) }})
	}

	build := func(blanks bool) {
		d.lines, d.stops, d.sections, d.given = nil, nil, nil, 0
		if n := r.updates; n > 0 {
			noun := "updates"
			if n == 1 {
				noun = "update"
			}
			text := fmt.Sprintf("✉ %d %s · see Log", n, noun)
			d.lines = append(d.lines, dline{
				text:  fit(" "+inboxStyle.Render(text), d.width),
				zones: []zone{{x0: 1, x1: 1 + ansi.StringWidth(text), act: action{kind: actTab, n: int(tabLog)}}},
			})
		}
		full := m.effectiveSize() == sizeFull
		for i, s := range secs {
			if blanks && (i > 0 || full) {
				d.line("")
			}
			d.sections = append(d.sections, section{name: s.name, at: len(d.lines)})
			d.line(" " + dim.Render("── ") + s.style.Render(s.name) + dim.Render(" ──"))
			s.body()
		}
		if blanks && full && len(secs) > 0 {
			d.line("")
		}
	}
	build(false)
	if len(d.lines)+len(secs)+1 <= h {
		build(true)
	}
}

// flow adds groups that wrap at the drawer's width, from column 1.
func (d *drawer) flow(groups ...group) {
	lw := d.labelW
	d.labelW = 0
	d.field("", false, groups...)
	d.labelW = lw
}

// chips are the links that keep returns true as one wrapping group of
// chips, `[1 ABC-1246 done]`, leaving out the PR's own (prLink), which
// the PR section shows. unlinked says a Linear ID lacks its URL.
func (m Model) chips(d *drawer, links []deck.Link, prLink int, keep func(deck.Link) bool) (g group, unlinked bool) {
	g.sep = " "
	prev := deck.LinkKind(-1)
	for i, l := range links {
		if i == prLink || !keep(l) {
			continue
		}
		g.items = append(g.items, m.chip(d, l, i, l.Kind != prev))
		prev = l.Kind
		unlinked = unlinked || l.Kind == deck.LinkLinear && l.URL == ""
	}
	return g, unlinked
}

// chip is one link as a chip: dim brackets, the digit bold, the label in
// its kind's colour, a Linear issue's state in its state colour. The kind
// word leads only the first chip of a run of one kind (never Linear's).
func (m Model) chip(d *drawer, l deck.Link, i int, first bool) item {
	var b strings.Builder
	b.WriteString(dim.Render("["))
	if i < 9 {
		b.WriteString(bold.Render(fmt.Sprint(i+1)) + " ")
	}
	st := chipStyle(l.Kind)
	closed := l.Issue != nil && issueClosed(*l.Issue)
	if first && l.Kind != deck.LinkLinear && l.Kind != deck.LinkLocalhost {
		b.WriteString(st.Render(l.Kind.String()) + " ")
	}
	label := l.Label
	if l.Kind == deck.LinkGitHub {
		label = strings.TrimPrefix(label, "PR ")
	}
	b.WriteString(st.Render(label))
	if l.Issue != nil {
		b.WriteString(" " + issueStyle(*l.Issue).Render(shortState(l.Issue.State)))
	}
	if l.Kind == deck.LinkLocalhost {
		b.WriteString(" " + devDot(!l.Down))
	}
	b.WriteString(dim.Render("]"))
	text := b.String()
	style := plain
	if closed || l.Kind == deck.LinkLocalhost && l.Down {
		style = dim
	}
	it := item{text: text, style: style, act: action{kind: actLink, n: i}, stop: true}
	it.sel = d.nextStop() || m.choosing == l.Kind
	return it
}

// chipStyle colours a chip's label: Linear blue, Figma magenta, GitHub
// magenta like the PR column, the rest plain.
func chipStyle(k deck.LinkKind) lipgloss.Style {
	switch k {
	case deck.LinkLinear:
		return lipgloss.NewStyle().Foreground(colBlue)
	case deck.LinkFigma, deck.LinkGitHub:
		return reviewStyle
	}
	return plain
}

// prSection is the PR's chip with its state, review and comments, then
// its checks; at full height also when herdr-projects last looked.
func (m Model) prSection(d *drawer, pr deck.PullRequest, links []deck.Link, prLink int, narrow bool) {
	var g group
	g.sep = " "
	if prLink >= 0 {
		g.items = append(g.items, m.chip(d, links[prLink], prLink, false))
	} else {
		g.items = append(g.items, span(fmt.Sprintf("#%d", pr.Number), reviewStyle))
	}
	add := func(text string, st lipgloss.Style) {
		g.items = append(g.items, span("·", dim), span(text, st))
	}
	if s := strings.ToLower(pr.State); s != "" {
		g.items = append(g.items, span(s, plain))
	}
	if pr.State == "OPEN" && pr.Review != "" {
		add(reviewText(pr), reviewTextStyle(pr.Review))
	}
	if n := pr.Comments; n > 0 {
		switch {
		case narrow:
			add(fmt.Sprintf("%dc", n), plain)
		case len(pr.Commenters) > 0:
			add(fmt.Sprintf("%d %s (%s)", n, plural(n, "comment"), strings.Join(pr.Commenters, ", ")), plain)
		default:
			add(fmt.Sprintf("%d %s", n, plural(n, "comment")), plain)
		}
	}
	d.flow(g)
	if pr.State != "" {
		if n := len(pr.FailingChecks); n > 0 {
			d.flow(words(fmt.Sprintf("✕ %d failing: %s", n, strings.Join(pr.FailingChecks, ", ")), failStyle))
		} else if pr.State == "OPEN" {
			d.flow(words("✓ no failing checks", okStyle))
		}
	}
	if m.effectiveSize() == sizeFull && !pr.CheckedAt.IsZero() {
		d.flow(words("checked "+ago(m.opt.Now().Sub(pr.CheckedAt)), dim))
	}
}

// reviewTextStyle colours a review decision: required magenta, approved
// green, changes requested red.
func reviewTextStyle(review string) lipgloss.Style {
	switch review {
	case "APPROVED":
		return okStyle
	case "CHANGES_REQUESTED":
		return failStyle
	}
	return reviewStyle
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// ago is an age in words: "now", "1m ago".
func ago(d time.Duration) string {
	if a := ageText(d); a != "now" {
		return a + " ago"
	}
	return "just now"
}

// threadSection is the metadata the card has no room for, under dim
// labels: the other threads of a task, the pane and its agent, branch,
// report, base and the dev log.
func (m Model) threadSection(d *drawer, r row, t deck.Thread) {
	narrow := d.width < wideMin
	if len(r.threads) > 1 {
		for _, th := range r.threads {
			g := group{sep: " ", items: []item{span(th.ID, bold)}}
			g.items = append(g.items, words(th.Title, plain).items...)
			st, ss := statusPill(th)
			g.items = append(g.items, span("·", dim), span(st, ss))
			d.field("thread", false, g)
		}
	}
	if items := m.paneValue(t); len(items) > 0 {
		d.field("pane", false, group{sep: " ", items: items})
	}
	if t.Branch != "" {
		d.field("branch", false, words(t.Branch, plain))
	}
	if t.Report != "" {
		g := group{sep: " ", items: []item{span("threads/"+t.ID+".md", plain)}}
		if !t.LastReportChange.IsZero() {
			g.items = append(g.items, span("·", dim), span("changed "+m.clock(t.LastReportChange, false), dim))
		}
		hint := "r shows it"
		if narrow {
			hint = "r"
		}
		g.items = append(g.items, span("·", dim), span(hint, dim))
		d.field("report", false, g)
	}
	if !narrow {
		base := t.Base
		mb := ""
		if dt, df, ok := m.diff(); ok && dt.ID == t.ID {
			if df.Base != "" {
				base = df.Base
			}
			mb = df.MergeBase
		}
		if base != "" {
			g := words(base, plain)
			if mb != "" {
				g.items = append(g.items, span("·", dim), span("merge-base "+shortSHA(mb), dim))
			}
			d.field("base", false, g)
		}
	}
	if t.DevUp != nil {
		d.field("log", false, words(tilde(t.DevUp.Log), dim))
	}
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

// paneValue is the thread's herdr pane and its agent's live state; "closed"
// when herdr was read and does not show the recorded pane.
func (m Model) paneValue(t deck.Thread) []item {
	switch {
	case t.Pane != nil:
		items := []item{span(t.Pane.ID, plain)}
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
		return []item{span(t.PaneID, plain), span("closed", dim)}
	}
	return []item{span(t.PaneID, plain)}
}

// clock is a time of day in the deck's zone, with the weekday when it is
// not today (unless sameDay says the caller shows the day already).
func (m Model) clock(t time.Time, sameDay bool) string {
	lt := t.In(m.loc())
	now := m.opt.Now().In(m.loc())
	if sameDay || lt.YearDay() == now.YearDay() && lt.Year() == now.Year() {
		return lt.Format("15:04")
	}
	return lt.Format("Mon 15:04")
}

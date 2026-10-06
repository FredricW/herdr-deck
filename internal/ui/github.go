package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// checkLog is a failed check's log as the log view shows it: read, being
// read, or why it could not be.
type checkLog struct {
	log     deck.CheckLog
	err     string
	loading bool
}

type checkLogMsg struct {
	job int64
	log deck.CheckLog
	err error
}

// checkCounts counts a PR's checks by state.
func checkCounts(pr deck.PullRequest) (failed, running, queued, passed int) {
	for _, c := range pr.Checks {
		switch c.State {
		case deck.CheckFailed:
			failed++
		case deck.CheckRunning:
			running++
		case deck.CheckQueued:
			queued++
		case deck.CheckPassed:
			passed++
		}
	}
	return
}

// checksGroup is the PR section's checks line from the deck's own read:
// failing checks as chips that open their log, then running, queued and
// passed counts: `✕ 1 failing: [lint]  ◌ 1 running: test (macos-latest) 4m  ✓ 3 passed`.
func (m Model) checksGroup(d *drawer, pr deck.PullRequest) group {
	g := group{sep: " "}
	failed, running, queued, passed := checkCounts(pr)
	gap := func() {
		if len(g.items) > 0 {
			g.items = append(g.items, span("", plain))
		}
	}
	if failed > 0 {
		g.items = append(g.items, span(fmt.Sprintf("✕ %d failing:", failed), failStyle))
		for i, c := range pr.Checks {
			if c.State != deck.CheckFailed {
				continue
			}
			it := item{text: dim.Render("[") + failStyle.Render(c.Name) + dim.Render("]"), style: plain,
				act: action{kind: actCheck, n: i}, stop: true}
			it.sel = d.nextStop()
			g.items = append(g.items, it)
		}
	}
	if running > 0 {
		gap()
		g.items = append(g.items, span(fmt.Sprintf("◌ %d running:", running), workStyle))
		for _, c := range pr.Checks {
			if c.State != deck.CheckRunning {
				continue
			}
			text := workStyle.Render(c.Name)
			if !c.Started.IsZero() {
				text += " " + dim.Render(ageText(m.opt.Now().Sub(c.Started)))
			}
			g.items = append(g.items, span(text, plain))
		}
	}
	if queued > 0 {
		gap()
		g.items = append(g.items, span(fmt.Sprintf("◌ %d queued", queued), warnStyle))
	}
	if passed > 0 {
		gap()
		text := fmt.Sprintf("✓ %d passed", passed)
		if failed+running+queued == 0 {
			text = fmt.Sprintf("✓ all %d checks passed", passed)
		}
		g.items = append(g.items, span(text, okStyle))
	}
	return g
}

// mergeReason says in words what stands between an open PR and its merge,
// from the deck's own read of GitHub's merge state, checks, reviews and
// auto-merge; "" when there is nothing to say or GitHub is still working
// it out.
func mergeReason(pr deck.PullRequest) (string, lipgloss.Style) {
	if !pr.Live || pr.State != "OPEN" {
		return "", plain
	}
	base := pr.Base
	if base == "" {
		base = "the base"
	}
	failed, running, queued, _ := checkCounts(pr)
	auto := ""
	if pr.AutoMerge {
		auto = "; auto-merge merges it once that is done"
	}
	switch {
	case pr.Draft:
		return "◇ draft: mark it ready for review before it can merge", dim
	case pr.MergeState == "DIRTY" || pr.Mergeable == "CONFLICTING":
		return "✕ conflicts with " + base + ": resolve them before it can merge", failStyle
	case pr.MergeState == "BEHIND":
		return "⚠ behind " + base + ": update the branch before it can merge" + auto, warnStyle
	case pr.MergeState == "BLOCKED":
		var why []string
		bad := pr.Review == "CHANGES_REQUESTED" || failed > 0
		if pr.Review == "CHANGES_REQUESTED" {
			why = append(why, "changes requested")
		}
		if failed > 0 {
			why = append(why, "checks failing")
		}
		if running+queued > 0 {
			why = append(why, "checks running")
		}
		if pr.Review == "REVIEW_REQUIRED" {
			w := "review required"
			if len(pr.ReviewRequests) > 0 {
				w += " (" + strings.Join(pr.ReviewRequests, ", ") + ")"
			}
			why = append(why, w)
		}
		if len(why) == 0 {
			return "⚠ blocked by " + base + "'s branch rules", warnStyle
		}
		if bad {
			return "✕ blocked: " + strings.Join(why, ", ") + auto, failStyle
		}
		return "⚠ blocked: " + strings.Join(why, ", ") + auto, warnStyle
	case pr.MergeState == "UNSTABLE":
		return "⚠ can merge, but some checks are not passing", warnStyle
	case pr.MergeState == "CLEAN" || pr.MergeState == "HAS_HOOKS":
		if pr.AutoMerge {
			return "✓ ready: auto-merge is merging it", okStyle
		}
		return "✓ ready to merge", okStyle
	}
	return "", plain
}

// threadCounts counts a PR's unresolved review threads: current and
// outdated.
func threadCounts(pr deck.PullRequest) (current, outdated int) {
	for _, t := range pr.Threads {
		if t.Outdated {
			outdated++
		} else {
			current++
		}
	}
	return
}

// prOf is the selected row's thread PR, when it has one.
func (m Model) prOf() (deck.PullRequest, bool) {
	r, ok := m.selected()
	if !ok || r.kind != rowWork {
		return deck.PullRequest{}, false
	}
	t, ok := r.thread()
	if !ok || t.PR == nil {
		return deck.PullRequest{}, false
	}
	return *t.PR, true
}

// watchPR tells the GitHub reader which PR the drawer shows, and which
// the PR tab shows, when that changed: it keeps the first fresher and
// reads the second's description and conversation. It calls FocusPR and
// DetailPR at once, not as a command: commands run in no set order, and
// an older PR's call landing last would keep the reader on it. Neither
// waits on the network.
func (m *Model) watchPR() tea.Cmd {
	pr, ok := m.prOf()
	focus, detail := "", ""
	if ok && m.size != sizeHidden {
		focus = pr.URL
		if m.mode == modeRow && m.curTab() == tabPR {
			detail = pr.URL
		}
	}
	// The detail first: Focus may start a read, which then asks for the
	// detail too rather than reading the PR twice.
	if m.opt.DetailPR != nil && detail != m.detailed {
		m.detailed = detail
		m.opt.DetailPR(detail)
	}
	if m.opt.FocusPR != nil && focus != m.watched {
		m.watched = focus
		m.opt.FocusPR(focus)
	}
	return nil
}

// failedChecks are the indexes of the PR's failed checks.
func failedChecks(pr deck.PullRequest) []int {
	var out []int
	for i, c := range pr.Checks {
		if c.State == deck.CheckFailed {
			out = append(out, i)
		}
	}
	return out
}

// checkKey opens the log of the selected PR's first failed check, or in
// the log view the next one.
func (m *Model) checkKey() tea.Cmd {
	pr, ok := m.prOf()
	if !ok {
		m.status = "no pull request on this row"
		return nil
	}
	failed := failedChecks(pr)
	if len(failed) == 0 {
		if pr.Live {
			m.status = fmt.Sprintf("#%d has no failed checks", pr.Number)
		} else {
			m.status = fmt.Sprintf("#%d: no check details; GitHub is not read (see !)", pr.Number)
		}
		return nil
	}
	next := failed[0]
	if m.mode == modeCheck {
		for k, i := range failed {
			if pr.Checks[i].Name == m.logCheck.Name {
				next = failed[(k+1)%len(failed)]
			}
		}
	}
	return m.openCheck(pr, next)
}

// openCheck shows check i's log in the drawer, reading it unless it was
// read before. A check outside GitHub Actions has no log: its page opens
// instead.
func (m *Model) openCheck(pr deck.PullRequest, i int) tea.Cmd {
	if i < 0 || i >= len(pr.Checks) {
		return nil
	}
	c := pr.Checks[i]
	if c.JobID == 0 || m.opt.CheckLog == nil {
		return m.openURL(c.URL, c.Name)
	}
	m.logCheck, m.logPR = c, pr.URL
	m.setMode(modeCheck)
	if l, ok := m.logs[c.JobID]; ok && (l.loading || l.err == "") {
		return nil
	}
	m.logs[c.JobID] = checkLog{loading: true}
	read, url := m.opt.CheckLog, pr.URL
	return func() tea.Msg {
		log, err := read(context.Background(), url, c)
		return checkLogMsg{job: c.JobID, log: log, err: err}
	}
}

func (m *Model) setCheckLog(msg checkLogMsg) {
	l := checkLog{log: msg.log}
	if msg.err != nil {
		l.err = msg.err.Error()
		if errors.Is(msg.err, context.DeadlineExceeded) {
			l.err = "timed out"
		}
	}
	m.logs[msg.job] = l
}

// checkDrawer is a failed check's log tail, full height under the card,
// with the check, its state and the failing step where the tab bar was.
func (m Model) checkDrawer(r row, width int) *drawer {
	d := newDrawer(width, "")
	d.card = m.card(r, width)
	d.light = m.light
	c := m.logCheck
	head := " " + bold.Render(c.Name) + dim.Render(" · ") + failStyle.Render("✕ failed")
	if !c.Completed.IsZero() {
		head += dim.Render(" " + ago(m.opt.Now().Sub(c.Completed)))
	}
	l := m.logs[c.JobID]
	if l.log.Step != "" {
		head += dim.Render(` · step "` + l.log.Step + `"`)
	}
	d.tabs = fit(head+dim.Render(" · esc returns"), width)
	switch {
	case l.loading:
		d.text("reading the log…", dim)
	case l.err != "":
		d.text("could not read the log: "+l.err, warnStyle)
	case len(l.log.Lines) == 0:
		d.text("the log is empty", dim)
	default:
		if l.log.More > 0 {
			d.line(dim.Render(fmt.Sprintf(" … %d earlier %s", l.log.More, plural(l.log.More, "line"))))
		}
		for _, line := range l.log.Lines {
			st := plain
			switch {
			case strings.HasPrefix(line, "Error: "):
				st = failStyle
			case strings.HasPrefix(line, "Warning: "):
				st = warnStyle
			}
			// Wrapped, not cut: the end of a long line is often the point.
			wrapped := ansi.Wrap(strings.ReplaceAll(line, "\t", "    "), max(width-1, 10), "")
			for _, l := range strings.Split(wrapped, "\n") {
				d.line(" " + st.Render(l))
			}
		}
	}
	d.line("")
	d.line(dim.Render(" ↵ opens the job on GitHub · c the next failed check"))
	return d
}

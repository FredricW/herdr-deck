package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// logTab is the Log tab: the thread's timeline, newest first, one line per
// event with its age, glyph and text, and at 80 columns its clock time. An
// item still in inbox/ ends in a yellow ✉. At full height a rule heads each
// day, a dim │ marks a gap of over an hour, and a last line names the
// sources.
func (m Model) logTab(d *drawer, t deck.Thread, links []deck.Link) {
	if len(t.Log) == 0 {
		d.line(" " + dim.Render("nothing recorded for "+t.ID+" yet"))
		return
	}
	rich := m.effectiveSize() == sizeFull
	now := m.opt.Now()
	prLink := -1
	if t.PR != nil {
		for i, l := range links {
			if l.Kind == deck.LinkGitHub && l.URL == t.PR.URL {
				prLink = i
			}
		}
	}
	report := -1 // the newest report event, which r shows
	for i, e := range t.Log {
		if e.Kind == deck.EventReport && t.Report != "" {
			report = i
			break
		}
	}
	day := ""
	items, unhandled := 0, 0
	for i, e := range t.Log {
		if strings.HasPrefix(e.Source, "inbox/") {
			items++
			if e.Unhandled {
				unhandled++
			}
		}
		if rich {
			if dd := m.dayName(e.At); dd != day {
				day = dd
				d.line(" " + dim.Render("── "+dd+" ──"))
			} else if i > 0 && t.Log[i-1].At.Sub(e.At) > time.Hour {
				d.line(" " + dim.Render("      │"))
			}
		}
		m.eventLine(d, t, e, i, i == report, prLink, links, now, rich)
	}
	if rich {
		src := "from threads/" + t.ID + ".toml"
		if items > 0 {
			src += fmt.Sprintf(" and %d inbox %s", items, plural(items, "item"))
		}
		if unhandled > 0 {
			src += fmt.Sprintf(", %d unhandled", unhandled)
		}
		d.line("")
		d.line(" " + dim.Render(src))
	}
}

// eventLine is one event of the Log tab.
func (m Model) eventLine(d *drawer, t deck.Thread, e deck.LogEvent, i int, latestReport bool, prLink int, links []deck.Link, now time.Time, rich bool) {
	wide := d.width >= wideMin
	glyph, gst := eventGlyph(e)
	age := ""
	if !e.At.IsZero() {
		age = ageText(now.Sub(e.At))
	}
	left := " " + dim.Render(fmt.Sprintf("%4s", age)) + "  " + gst.Render(glyph) + " "
	var body []string
	isPR := e.Kind == deck.EventPROpened || e.Kind == deck.EventPRUpdated || e.Kind == deck.EventMerged || e.Kind == deck.EventChecksFailing
	if isPR && prLink >= 0 && e.Kind != deck.EventChecksFailing {
		body = append(body, m.chip(&drawer{cur: -1}, links[prLink], prLink, false).text)
	}
	text := e.Text
	if isPR && prLink < 0 && e.Kind != deck.EventChecksFailing {
		text = "PR " + text
	}
	tst := plain
	if e.Kind == deck.EventChecksFailing {
		tst = failStyle
	}
	body = append(body, tst.Render(text))
	var details []string
	if e.Detail != "" {
		details = append(details, e.Detail)
	}
	if e.Kind == deck.EventLaunched && !t.BriefSeenAt.IsZero() {
		if wide {
			details = append(details, "brief read "+m.clock(t.BriefSeenAt, true))
		} else {
			details = append(details, "brief read")
		}
	}
	if latestReport {
		details = append(details, "r shows it")
	}
	for _, det := range details {
		body = append(body, dim.Render("· "+det))
	}

	mark := " "
	if e.Unhandled {
		mark = inboxStyle.Render("✉")
	}
	right := mark + " "
	if wide && !e.At.IsZero() {
		right = dim.Render(m.clock(e.At, rich)) + " " + right
	}
	line := spread(left+strings.Join(body, " "), right, d.width)
	act := action{kind: actEvent, n: i}
	d.stopLine(line, act, []zone{{x0: 0, x1: d.width, act: act}})
}

// eventGlyph is an event's glyph and colour: what happened, whether or not
// it is handled.
func eventGlyph(e deck.LogEvent) (string, lipgloss.Style) {
	switch e.Kind {
	case deck.EventCreated:
		return "+", dim
	case deck.EventLaunched:
		return "▶", dim
	case deck.EventReport:
		return "≡", plain
	case deck.EventWaiting, deck.EventBlocked:
		return "●", failStyle
	case deck.EventPROpened, deck.EventPRUpdated:
		return "◇", reviewStyle
	case deck.EventChecksFailing:
		return "✕", failStyle
	case deck.EventMerged:
		return "✓", okStyle
	case deck.EventResolved:
		return "✓", dim
	case deck.EventRoutine:
		return "»", dim
	}
	return "·", dim
}

// dayName heads a day in the Log tab: "Today · Fri 2 Oct", "Yesterday ·
// Thu 1 Oct", else the date.
func (m Model) dayName(t time.Time) string {
	lt := t.In(m.loc())
	now := m.opt.Now().In(m.loc())
	date := lt.Format("Mon 2 Jan")
	y, mo, dd := now.Date()
	today := time.Date(y, mo, dd, 0, 0, 0, 0, m.loc())
	switch {
	case !lt.Before(today):
		return "Today · " + date
	case !lt.Before(today.AddDate(0, 0, -1)):
		return "Yesterday · " + date
	}
	return date
}

// eventAction does what enter or a click on Log event i does: a report
// shows the report, a PR event opens the PR, the rest focus the pane.
func (m *Model) eventAction(i int) (cmdKind, int) {
	r, _ := m.selected()
	t, ok := r.thread()
	if !ok || i < 0 || i >= len(t.Log) {
		return cmdNone, 0
	}
	switch t.Log[i].Kind {
	case deck.EventReport:
		if t.Report != "" {
			return cmdReport, 0
		}
	case deck.EventPROpened, deck.EventPRUpdated, deck.EventChecksFailing, deck.EventMerged:
		for j, l := range rowLinks(r) {
			if t.PR != nil && l.URL == t.PR.URL {
				return cmdLink, j
			}
		}
	}
	return cmdPane, 0
}

type cmdKind int

const (
	cmdNone cmdKind = iota
	cmdReport
	cmdLink
	cmdPane
)

package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// withIssues gives the Users page's Linear links the titles, assignees and
// URLs Linear's API would, as with an API key and no linear.workspace.
func withIssues() deck.Snapshot {
	s := calm()
	links := s.TaskLists[0].Tasks[0].Links
	set := func(i int, title, who, initials string) {
		is := *links[i].Issue
		is.Title, is.Assignee, is.AssigneeInitials, is.Team = title, who, initials, "Admin"
		is.URL = "https://linear.app/acme/issue/" + links[i].Label + "/" + strings.ReplaceAll(strings.ToLower(title), " ", "-")
		links[i].Issue, links[i].URL = &is, is.URL
	}
	set(0, "Users list: columns and sorting", "sam", "S")
	set(1, "Users overview layout with the activity panel and filters", "ana.lopez", "AL")
	set(2, "Invite flow", "", "")
	links[3].URL = "" // ABC-1250: Linear has not answered yet
	s.LinearKey = true
	return s
}

func TestGoldenLinearIssues(t *testing.T) {
	cases := []struct {
		name string
		keys []tea.Msg
	}{
		{name: "linear-issues"},
		{name: "linear-issues-full", keys: keys("z")},
	}
	for _, c := range cases {
		for _, w := range []int{80, 60} {
			name := c.name + "-" + map[int]string{80: "80", 60: "60"}[w]
			t.Run(name, func(t *testing.T) {
				m, _ := newModel(t, deck.Snapshot{}, w, 28)
				m, _ = press(m, snapshotMsg(withIssues()))
				m, _ = press(m, c.keys...)
				golden(t, name, m)
			})
		}
	}
}

func TestIssueChipShortens(t *testing.T) {
	l := deck.Link{Kind: deck.LinkLinear, Label: "ABC-123", URL: "https://linear.app/acme/issue/ABC-123/fix-login", Issue: &deck.Issue{
		State: "In Progress", StateType: "started", Title: "Fix login on the admin page", Assignee: "ana.lopez", AssigneeInitials: "AL",
	}}
	cases := []struct {
		width int
		want  string
	}{
		{80, "[1 ABC-123 Fix login on the admin page · in progress · ana.lopez]"},
		{60, "[1 ABC-123 Fix login on the admin page · in progress · AL]"},
		{50, "[1 ABC-123 Fix login on the… · in progress · AL]"},
		{40, "[1 ABC-123 Fix log… · in progress · AL]"},
		{36, "[1 ABC-123 · in progress · AL]"},
		{28, "[1 ABC-123 · in progress]"},
	}
	m, _ := newModel(t, deck.Snapshot{}, 80, 28)
	for _, c := range cases {
		d := newDrawer(c.width, "")
		got := ansi.Strip(m.issueChip(d, l, 0).text)
		if got != c.want {
			t.Errorf("width %d: %q, want %q", c.width, got, c.want)
		}
		if w := ansi.StringWidth(got); w > c.width-1 && c.width > 28 {
			t.Errorf("width %d: chip is %d wide", c.width, w)
		}
	}
}

// A titled chip opens Linear's own URL, and with a key a link Linear has
// not answered for yet waits rather than asking for a workspace.
func TestLinearIssueLinks(t *testing.T) {
	m, o := newModel(t, deck.Snapshot{}, 80, 28)
	m, _ = press(m, snapshotMsg(withIssues()))
	if s := screen(m); strings.Contains(s, "no Linear workspace") {
		t.Errorf("with a key the drawer asks for a workspace:\n%s", s)
	}
	m, _ = press(m, keys("2")...)
	if len(o.urls) != 1 || o.urls[0] != "https://linear.app/acme/issue/ABC-1256/users-overview-layout-with-the-activity-panel-and-filters" {
		t.Errorf("opened %q", o.urls)
	}
	m, _ = press(m, keys("4")...)
	if !strings.Contains(m.status, "no URL from Linear yet") {
		t.Errorf("status = %q", m.status)
	}
}

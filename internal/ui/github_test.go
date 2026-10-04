package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/fake"
)

// live is calm with the deck's own GitHub read laid over t-0004's PR.
func live() deck.Snapshot {
	s := calm()
	fake.GitHub(&s, now)
	return s
}

// livePR changes t-0004's live PR.
func livePR(change func(*deck.PullRequest)) deck.Snapshot {
	s := live()
	change(s.Threads[3].PR)
	return s
}

// withIssue gives t-0004's Linear link a titled issue, so Linear's line
// and the live PR section show together.
func withIssue(s deck.Snapshot) deck.Snapshot {
	issue := &deck.Issue{State: "In Review", StateType: "started", Title: "Summary document picker", Assignee: "Sam Lee", AssigneeInitials: "SL"}
	set := func(links []deck.Link) {
		for i, l := range links {
			if l.Kind == deck.LinkLinear && l.Label == "ABC-1191" {
				links[i].Issue = issue
			}
		}
	}
	set(s.Threads[3].Links)
	for li := range s.TaskLists {
		for ti := range s.TaskLists[li].Tasks {
			set(s.TaskLists[li].Tasks[ti].Links)
		}
	}
	return s
}

func TestGitHubGolden(t *testing.T) {
	behind := livePR(func(pr *deck.PullRequest) {
		pr.MergeState, pr.Review, pr.AutoMerge, pr.Threads = "BEHIND", "APPROVED", true, nil
		pr.Checks = pr.Checks[2:] // all passed
		pr.FailingChecks = nil
	})
	draft := livePR(func(pr *deck.PullRequest) {
		pr.Draft, pr.MergeState, pr.Mergeable = true, "DIRTY", "CONFLICTING"
		pr.Checks = append(pr.Checks, deck.Check{Name: "deploy-preview", State: deck.CheckQueued})
	})
	cases := []struct {
		name string
		snap deck.Snapshot
		keys []tea.Msg
	}{
		{name: "pr-live", snap: live(), keys: keys("jj")},
		{name: "pr-live-full", snap: live(), keys: keys("jjz")},
		{name: "pr-behind", snap: behind, keys: keys("jj")},
		{name: "pr-draft", snap: draft, keys: keys("jjz")},
		{name: "pr-check-log", snap: live(), keys: keys("jjc")},
		{name: "pr-live-linear", snap: withIssue(live()), keys: keys("jjz")},
	}
	for _, c := range cases {
		for _, w := range []int{80, 60} {
			name := c.name + "-" + map[int]string{80: "80", 60: "60"}[w]
			t.Run(name, func(t *testing.T) {
				m, _ := newModelWith(t, deck.Snapshot{}, w, 28, func(o *Options) { o.CheckLog = fake.CheckLog })
				m, _ = press(m, snapshotMsg(c.snap))
				m, _ = press(m, c.keys...)
				golden(t, name, m)
			})
		}
	}
}

func TestMergeReason(t *testing.T) {
	base := deck.PullRequest{Live: true, State: "OPEN", Base: "main"}
	pass := []deck.Check{{Name: "lint", State: deck.CheckPassed}}
	fail := []deck.Check{{Name: "lint", State: deck.CheckFailed}}
	run := []deck.Check{{Name: "test", State: deck.CheckRunning}}
	for _, c := range []struct {
		name string
		pr   func(*deck.PullRequest)
		want string
	}{
		{"not live", func(p *deck.PullRequest) { p.Live = false; p.MergeState = "BEHIND" }, ""},
		{"merged", func(p *deck.PullRequest) { p.State = "MERGED"; p.MergeState = "BEHIND" }, ""},
		{"unknown", func(p *deck.PullRequest) { p.MergeState = "UNKNOWN" }, ""},
		{"draft", func(p *deck.PullRequest) { p.Draft = true; p.MergeState = "DIRTY" }, "◇ draft: mark it ready for review before it can merge"},
		{"dirty", func(p *deck.PullRequest) { p.MergeState = "DIRTY" }, "✕ conflicts with main: resolve them before it can merge"},
		{"conflicting", func(p *deck.PullRequest) { p.MergeState = "BLOCKED"; p.Mergeable = "CONFLICTING" }, "✕ conflicts with main: resolve them before it can merge"},
		{"behind", func(p *deck.PullRequest) { p.MergeState = "BEHIND" }, "⚠ behind main: update the branch before it can merge"},
		{"behind auto", func(p *deck.PullRequest) { p.MergeState = "BEHIND"; p.AutoMerge = true },
			"⚠ behind main: update the branch before it can merge; auto-merge merges it once that is done"},
		{"review", func(p *deck.PullRequest) {
			p.MergeState, p.Review, p.ReviewRequests, p.Checks = "BLOCKED", "REVIEW_REQUIRED", []string{"sam", "frontend"}, pass
		}, "⚠ blocked: review required (sam, frontend)"},
		{"changes", func(p *deck.PullRequest) { p.MergeState, p.Review = "BLOCKED", "CHANGES_REQUESTED" }, "✕ blocked: changes requested"},
		{"failing and review", func(p *deck.PullRequest) { p.MergeState, p.Review, p.Checks = "BLOCKED", "REVIEW_REQUIRED", fail },
			"✕ blocked: checks failing, review required"},
		{"running auto", func(p *deck.PullRequest) { p.MergeState, p.Checks, p.AutoMerge = "BLOCKED", run, true },
			"⚠ blocked: checks running; auto-merge merges it once that is done"},
		{"blocked", func(p *deck.PullRequest) { p.MergeState, p.Checks = "BLOCKED", pass }, "⚠ blocked by main's branch rules"},
		{"unstable", func(p *deck.PullRequest) { p.MergeState = "UNSTABLE" }, "⚠ can merge, but some checks are not passing"},
		{"clean", func(p *deck.PullRequest) { p.MergeState = "CLEAN" }, "✓ ready to merge"},
		{"hooks auto", func(p *deck.PullRequest) { p.MergeState, p.AutoMerge = "HAS_HOOKS", true }, "✓ ready: auto-merge is merging it"},
		{"no base", func(p *deck.PullRequest) { p.MergeState, p.Base = "BEHIND", "" }, "⚠ behind the base: update the branch before it can merge"},
	} {
		pr := base
		c.pr(&pr)
		if got, _ := mergeReason(pr); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestCheckLogKey(t *testing.T) {
	reads := 0
	m, o := newModelWith(t, live(), 80, 28, func(opt *Options) {
		opt.CheckLog = func(ctx context.Context, url string, c deck.Check) (deck.CheckLog, error) {
			reads++
			if url != "https://github.com/acme/webshop/pull/2320" {
				t.Errorf("CheckLog got %s", url)
			}
			return fake.CheckLog(ctx, url, c)
		}
	})
	m, _ = press(m, keys("jjc")...)
	if m.mode != modeCheck || m.logCheck.Name != "lint" {
		t.Fatalf("c: mode %v, check %q", m.mode, m.logCheck.Name)
	}
	if sc := screen(m); !strings.Contains(sc, `step "golangci-lint run ./..."`) || !strings.Contains(sc, "Error: issues found") {
		t.Errorf("the log view lacks the step or the error:\n%s", sc)
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(o.urls) != 1 || !strings.HasSuffix(o.urls[0], "/job/9001") {
		t.Errorf("↵ opened %q, want the job", o.urls)
	}
	m, _ = press(m, esc)
	if m.mode != modeRow {
		t.Errorf("esc: mode %v", m.mode)
	}
	// Open again: the log is not read twice.
	m, _ = press(m, keys("c")...)
	if reads != 1 || m.mode != modeCheck {
		t.Errorf("%d reads, mode %v", reads, m.mode)
	}
	// Moving to another row leaves the log view.
	m, _ = press(m, keys("k")...)
	if m.mode != modeRow {
		t.Errorf("another row: mode %v", m.mode)
	}
	// A row without failed checks says so.
	m, _ = press(m, keys("c")...)
	if m.mode == modeCheck || m.Status() == "" {
		t.Errorf("c on a row without a PR: mode %v, status %q", m.mode, m.Status())
	}
}

func TestCheckLogError(t *testing.T) {
	m, _ := newModelWith(t, live(), 80, 28, func(opt *Options) {
		opt.CheckLog = func(context.Context, string, deck.Check) (deck.CheckLog, error) {
			return deck.CheckLog{}, errors.New("gh: HTTP 404")
		}
	})
	m, _ = press(m, keys("jjc")...)
	if sc := screen(m); !strings.Contains(sc, "could not read the log: gh: HTTP 404") {
		t.Errorf("no error shown:\n%s", sc)
	}
}

func TestCheckChipAndThreadClick(t *testing.T) {
	m, o := newModelWith(t, live(), 80, 28, func(opt *Options) { opt.CheckLog = fake.CheckLog })
	m, _ = press(m, keys("jjz")...)
	x, y := find(t, m, "src/pages/users/table.tsx:12")
	m, _ = press(m, click(x, y))
	if len(o.urls) != 1 || o.urls[0] != "https://github.com/acme/webshop/pull/2320#discussion_r102" {
		t.Errorf("a click on a review thread opened %q", o.urls)
	}
	x, y = find(t, m, "[lint]")
	m, _ = press(m, click(x+1, y))
	if m.mode != modeCheck || m.logCheck.Name != "lint" {
		t.Errorf("a click on the check chip: mode %v, check %q", m.mode, m.logCheck.Name)
	}
}

func TestCheckWithoutJobOpensPage(t *testing.T) {
	s := livePR(func(pr *deck.PullRequest) {
		pr.Checks = []deck.Check{{Name: "deploy-preview", State: deck.CheckFailed, URL: "https://ci.example.com/acme/webshop/77"}}
	})
	m, o := newModelWith(t, s, 80, 28, func(opt *Options) { opt.CheckLog = fake.CheckLog })
	m, _ = press(m, keys("jjc")...)
	if m.mode == modeCheck || len(o.urls) != 1 || o.urls[0] != "https://ci.example.com/acme/webshop/77" {
		t.Errorf("mode %v, opened %q; want the check's page", m.mode, o.urls)
	}
}

func TestFocusPR(t *testing.T) {
	var focused []string
	m, _ := newModelWith(t, deck.Snapshot{}, 80, 28, func(opt *Options) {
		opt.FocusPR = func(url string) { focused = append(focused, url) }
	})
	m, _ = press(m, snapshotMsg(live()))
	m, _ = press(m, keys("jj")...)
	m, _ = press(m, keys("j")...)
	m, _ = press(m, keys("k")...)
	_ = m
	want := []string{"https://github.com/acme/webshop/pull/2320", "", "https://github.com/acme/webshop/pull/2320"}
	if strings.Join(focused, " ") != strings.Join(want, " ") {
		t.Errorf("FocusPR got %q, want %q", focused, want)
	}
}

func TestTickerOnlyPRUnchanged(t *testing.T) {
	// Without the deck's read, the PR section is as before: no Review
	// section, no merge reason.
	m, _ := newModel(t, calm(), 80, 28)
	m, _ = press(m, keys("jjz")...)
	sc := screen(m)
	if strings.Contains(sc, "── Review") || strings.Contains(sc, "waiting on") {
		t.Errorf("ticker-only PR shows the deck's read:\n%s", sc)
	}
}

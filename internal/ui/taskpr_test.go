package ui

import (
	"strings"
	"testing"

	"github.com/FredricW/herdr-deck/internal/deck"
)

const cartURL = "https://github.com/acme/storefront/pull/77"

func ghLink(url string) deck.Link {
	return deck.Link{Kind: deck.LinkGitHub, Label: "PR", URL: url}
}

// A row's PR is its thread's; without one, the PR its task links.
func TestRowPR(t *testing.T) {
	threadPR := &deck.PullRequest{URL: "https://github.com/acme/webshop/pull/12", Number: 12}
	taskPR := &deck.PullRequest{URL: cartURL, Number: 77}
	cases := []struct {
		name    string
		task    *deck.PullRequest
		threads []deck.Thread
		want    int
		own     bool
	}{
		{name: "no thread", task: taskPR, want: 77},
		{name: "thread PR wins", task: taskPR, threads: []deck.Thread{{ID: "t-0001", PR: threadPR}}, want: 12, own: true},
		{name: "thread without PR", task: taskPR, threads: []deck.Thread{{ID: "t-0002"}}, want: 77},
		{name: "none", threads: []deck.Thread{{ID: "t-0002"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := row{kind: rowWork, task: &deck.Task{Title: "x", PR: c.task}, threads: c.threads}
			pr, own, ok := r.pr()
			n := 0
			if ok {
				n = pr.Number
			}
			if ok != (c.want > 0) || n != c.want || own != c.own {
				t.Errorf("pr() = #%d own %v ok %v, want #%d own %v", n, own, ok, c.want, c.own)
			}
		})
	}
}

// withTaskPRs is calm plus a review task with no thread, and a PR on
// t-0003's task, whose thread has none. Both PRs fail a check.
func withTaskPRs() deck.Snapshot {
	s := calm()
	tasks := s.TaskLists[0].Tasks
	for i := range tasks {
		if tasks[i].Title == "Templates page" {
			url := "https://github.com/acme/webshop/pull/88"
			tasks[i].Links = append(tasks[i].Links, ghLink(url))
			tasks[i].PR = &deck.PullRequest{URL: url, Number: 88, State: "OPEN", FailingChecks: []string{"lint"}}
		}
	}
	s.TaskLists[0].Tasks = append(tasks, deck.Task{
		Title: "Review the cart rewrite",
		Links: []deck.Link{ghLink(cartURL)},
		Notes: cartURL + " · review requested",
		PR:    &deck.PullRequest{URL: cartURL, Number: 77, State: "OPEN", FailingChecks: []string{"test"}},
	})
	return s
}

// The PR column and its click target show a task's PR, without the ✕
// the ticker gives thread PRs; a click shows its PR tab, and a second
// opens it on GitHub, as for a thread PR.
func TestTaskPRColumn(t *testing.T) {
	for _, w := range []int{80, 60} {
		for _, c := range []struct{ num, title, url string }{
			{"#77", "Review the cart rewrite", cartURL},
			{"#88", "Templates page", "https://github.com/acme/webshop/pull/88"},
		} {
			var detail []string
			m, o := newModelWith(t, withTaskPRs(), w, 28, func(o *Options) {
				o.DetailPR = func(url string) { detail = append(detail, url) }
			})
			x, y := find(t, m, c.num)
			line := strings.Split(screen(m), "\n")[y]
			if !strings.Contains(line, c.title[:10]) || strings.Contains(line, c.num+"✕") {
				t.Fatalf("%d: %s shows on %q", w, c.num, line)
			}
			i := m.listOff + y - m.layout().listTop
			if x0, x1, ok := m.prCell(m.rows[i]); !ok || x < x0 || x >= x1 {
				t.Fatalf("%d: %s at %d, click target [%d,%d) ok %v", w, c.num, x, x0, x1, ok)
			}
			m, _ = press(m, click(x, y))
			if selectedTitle(m) != c.title || m.curTab() != tabPR || len(o.urls) != 0 {
				t.Fatalf("%d: a click on %s: %q, tab %v, opened %q", w, c.num, selectedTitle(m), m.curTab(), o.urls)
			}
			// The PR tab asks the reader for this PR's detail.
			if len(detail) == 0 || detail[len(detail)-1] != c.url {
				t.Errorf("%d: DetailPR got %q, want %s", w, detail, c.url)
			}
			m, _ = press(m, click(x, y))
			if len(o.urls) != 1 || o.urls[0] != c.url {
				t.Errorf("%d: a second click on %s opened %q", w, c.num, o.urls)
			}
		}
	}
}

// A task's PR that gh cannot read says so in its PR tab, in one line.
func TestTaskPRTabError(t *testing.T) {
	s := withTaskPRs()
	last := &s.TaskLists[0].Tasks[len(s.TaskLists[0].Tasks)-1]
	last.PR.Live = true
	last.PR.DetailNote = "GitHub has no such pull request, or gh's account cannot see it"
	m, _ := newModelWith(t, s, 80, 28, func(o *Options) { o.DetailPR = func(string) {} })
	x, y := find(t, m, "#77")
	m, _ = press(m, click(x, y))
	if m.curTab() != tabPR {
		t.Fatalf("tab %v, want PR", m.curTab())
	}
	sc := screen(m)
	if !strings.Contains(sc, "GitHub has no such pull request, or gh's account cannot see it") {
		t.Errorf("the PR tab does not say why:\n%s", sc)
	}
	// The drawer card shows the PR and its failing check.
	if card := strings.Join(m.card(m.rows[m.cursor], 80), "\n"); !strings.Contains(card, "#77") || !strings.Contains(card, "✕ 1 failing") {
		t.Errorf("the card:\n%s", card)
	}
}

// A thread's PR wins over the task's link even when a more pressing
// thread of the row has none: the GitHub reader skips the task's PR then.
func TestRowPRAnyThread(t *testing.T) {
	r := row{kind: rowWork, task: &deck.Task{Title: "x", PR: &deck.PullRequest{URL: cartURL, Number: 77}}, threads: []deck.Thread{
		{ID: "t-0001", Status: deck.StatusNeedsYou},
		{ID: "t-0002", Status: deck.StatusWorking, PR: &deck.PullRequest{URL: "https://github.com/acme/webshop/pull/12", Number: 12}},
	}}
	if pr, own, ok := r.pr(); !ok || !own || pr.Number != 12 {
		t.Errorf("pr() = %+v own %v ok %v, want t-0002's #12", pr, own, ok)
	}
}

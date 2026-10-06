package github

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

var now = time.Date(2026, 10, 2, 14, 41, 0, 0, time.UTC)

const (
	prURL   = "https://github.com/acme/webshop/pull/2320"
	goneURL = "https://github.com/acme/webshop/pull/99"
)

// fakeGh answers gh calls from fixtures and records them.
type fakeGh struct {
	mu    sync.Mutex
	calls [][]string
	out   []byte
	err   error
}

func (f *fakeGh) run(_ context.Context, args []string, _ int) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, args)
	return f.out, f.err
}

func (f *fakeGh) n() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func snap(ticker time.Time) deck.Snapshot {
	return deck.Snapshot{Threads: []deck.Thread{
		{ID: "t-0004", Status: deck.StatusReview, PR: &deck.PullRequest{
			URL: prURL, Number: 2320, State: "OPEN", Review: "REVIEW_REQUIRED", Comments: 2, CheckedAt: ticker,
		}},
		{ID: "t-0005", Status: deck.StatusWorking, PR: &deck.PullRequest{URL: goneURL, Number: 99, State: "OPEN"}},
		{ID: "t-0006", Status: deck.StatusDone, PR: &deck.PullRequest{URL: "https://github.com/acme/webshop/pull/7", State: "OPEN"}},
		{ID: "t-0007", Status: deck.StatusWorking, PR: &deck.PullRequest{URL: "https://github.com/acme/webshop/pull/8", State: "MERGED"}},
	}}
}

func newReader(f *fakeGh, clock *time.Time) *Reader {
	return &Reader{Run: f.run, Now: func() time.Time { return *clock }}
}

func TestApplyReadsAndMerges(t *testing.T) {
	f := &fakeGh{out: fixture(t, "prs.json")}
	clock := now
	r := newReader(f, &clock)
	s := snap(now.Add(-time.Minute))
	r.Apply(&s)
	if s.Threads[0].PR.Live {
		t.Fatal("Apply waited on the network")
	}
	r.Wait()
	if f.n() != 1 {
		t.Fatalf("%d gh calls, want one batch", f.n())
	}
	args := strings.Join(f.calls[0], " ")
	for _, want := range []string{"api graphql --hostname github.com", "o0=acme", "r0=webshop", "n0=2320", "n1=99", "reviewThreads"} {
		if !strings.Contains(args, want) {
			t.Errorf("query lacks %q: %s", want, args)
		}
	}
	// The resolved thread's PR and the merged PR are never asked about.
	if strings.Contains(args, "n2=") || strings.Contains(args, "=7 ") || strings.Contains(args, "n1=8") {
		t.Errorf("asked about a resolved thread's or merged PR: %s", args)
	}

	s = snap(now.Add(-time.Minute))
	r.Apply(&s)
	pr := s.Threads[0].PR
	if !pr.Live || pr.MergeState != "BEHIND" || pr.Base != "main" || !pr.AutoMerge || pr.Draft {
		t.Errorf("merged PR = %+v", pr)
	}
	if !pr.CheckedAt.Equal(now) {
		t.Errorf("CheckedAt = %v, want the deck's read %v", pr.CheckedAt, now)
	}
	if !slices.Equal(pr.ReviewRequests, []string{"sam", "frontend"}) {
		t.Errorf("ReviewRequests = %q", pr.ReviewRequests)
	}
	if !slices.Equal(pr.FailingChecks, []string{"lint"}) {
		t.Errorf("FailingChecks = %q, want lint (the re-run test passed)", pr.FailingChecks)
	}
	var got []string
	for _, c := range pr.Checks {
		got = append(got, c.Name+":"+[]string{"queued", "running", "passed", "failed", "skipped"}[c.State])
	}
	// Docs' own lint job passed later; it does not hide CI's failing one.
	want := []string{"coverage:passed", "deploy-preview:queued", "docs:skipped", "lint:failed", "lint:passed", "test (macos-latest):running", "test (ubuntu-latest):passed"}
	if !slices.Equal(got, want) {
		t.Errorf("checks = %q\nwant %q", got, want)
	}
	if pr.Checks[3].JobID != 9001 || pr.Checks[4].JobID != 9100 || pr.Checks[1].JobID != 0 {
		t.Errorf("job IDs = %d, %d, %d; want 9001, 9100 and 0 for a check outside Actions", pr.Checks[3].JobID, pr.Checks[4].JobID, pr.Checks[1].JobID)
	}
	if s.Threads[1].PR.Live {
		t.Error("a PR GitHub does not know was marked live")
	}
	if len(s.Missing) != 0 || len(s.Notes) != 0 {
		t.Errorf("Missing %q, Notes %q; want none", s.Missing, s.Notes)
	}
}

func TestReviewThreadsParse(t *testing.T) {
	prs, _, err := parseResponse(fixture(t, "prs.json"), 2)
	if err != nil {
		t.Fatal(err)
	}
	if prs[1] != nil {
		t.Error("the missing PR parsed")
	}
	th := prs[0].Threads
	if len(th) != 3 {
		t.Fatalf("%d threads, want the 3 unresolved: %+v", len(th), th)
	}
	if th[0].Path != "src/pages/users/index.ts" || th[0].Line != 41 || th[0].Author != "sam" || th[0].Replies != 0 ||
		!strings.HasPrefix(th[0].Body, "Should this be **paginated**?") || !strings.HasSuffix(th[0].URL, "#discussion_r101") {
		t.Errorf("first thread = %+v", th[0])
	}
	if th[1].Replies != 2 || th[1].Author != "alex" {
		t.Errorf("second thread = %+v, want alex with 2 replies", th[1])
	}
	if !th[2].Outdated || th[2].Line != 7 || th[2].Path != "src/api/users.ts" {
		t.Errorf("outdated thread = %+v, want it last, on its original line", th[2])
	}
}

func TestTickerNewerWins(t *testing.T) {
	f := &fakeGh{out: fixture(t, "prs.json")}
	clock := now
	r := newReader(f, &clock)
	s := snap(time.Time{})
	r.Apply(&s)
	r.Wait()

	// The ticker looked a minute after the deck and saw the PR merged.
	clock = now.Add(90 * time.Second)
	s = snap(now.Add(time.Minute))
	s.Threads[0].PR.State = "MERGED"
	s.Threads[0].PR.FailingChecks = nil
	r.Apply(&s)
	pr := s.Threads[0].PR
	if pr.State != "MERGED" || !pr.CheckedAt.Equal(now.Add(time.Minute)) {
		t.Errorf("state %s checked %v; want the newer ticker's MERGED", pr.State, pr.CheckedAt)
	}
	if pr.Checks != nil {
		t.Error("the deck's older checks stayed beside the newer ticker's")
	}
	r.Wait()
	// Merged now, so it is not asked again, though the entry is stale.
	if f.n() != 1 {
		t.Errorf("%d calls; a merged PR was read again", f.n())
	}

	// A newer ticker that agrees keeps the deck's checks.
	s = snap(now.Add(time.Minute))
	s.Threads[0].PR.FailingChecks = []string{"lint"}
	r2 := newReader(&fakeGh{out: fixture(t, "prs.json")}, &clock)
	s0 := snap(time.Time{})
	r2.Apply(&s0)
	r2.Wait()
	r2.Apply(&s)
	if s.Threads[0].PR.Checks == nil {
		t.Error("a newer ticker that agrees dropped the deck's checks")
	}
}

func TestCacheAndFocus(t *testing.T) {
	f := &fakeGh{out: fixture(t, "prs.json")}
	clock := now
	r := newReader(f, &clock)
	s := snap(time.Time{})
	r.Apply(&s)
	r.Wait()

	// Within the TTL nothing is read again.
	clock = now.Add(time.Minute)
	s = snap(time.Time{})
	r.Apply(&s)
	r.Wait()
	if f.n() != 1 {
		t.Fatalf("%d calls within the TTL", f.n())
	}
	// The drawer shows the PR: past FocusTTL it is read again, alone.
	r.Focus(prURL)
	r.Wait()
	if f.n() != 2 {
		t.Fatalf("%d calls; Focus past FocusTTL did not read", f.n())
	}
	if args := strings.Join(f.calls[1], " "); strings.Contains(args, "n1=") {
		t.Errorf("Focus read more than its PR: %s", args)
	}
	r.Focus(prURL)
	r.Wait()
	if f.n() != 2 {
		t.Error("Focus read a fresh PR again")
	}
	// Past the TTL every open PR is read again.
	clock = now.Add(4 * time.Minute)
	r.Focus("")
	s = snap(time.Time{})
	r.Apply(&s)
	r.Wait()
	if f.n() != 3 {
		t.Errorf("%d calls; past the TTL nothing was read", f.n())
	}
}

func TestBackoff(t *testing.T) {
	f := &fakeGh{err: &RunError{Code: 1, Stderr: "error connecting to api.github.com"}}
	clock := now
	r := newReader(f, &clock)
	s := snap(time.Time{})
	r.Apply(&s)
	r.Wait()
	s = snap(time.Time{})
	r.Apply(&s)
	r.Wait()
	if f.n() != 1 {
		t.Fatalf("%d calls; no back-off after a failure", f.n())
	}
	if len(s.Missing) != 1 || !strings.Contains(s.Missing[0], "error connecting") {
		t.Errorf("Missing = %q", s.Missing)
	}
	if s.Threads[0].PR.Live {
		t.Error("a failed read marked the PR live")
	}
	// After a minute it tries again, then waits two.
	clock = now.Add(61 * time.Second)
	r.Apply(&s)
	r.Wait()
	clock = now.Add(2 * time.Minute)
	r.Apply(&s)
	r.Wait()
	if f.n() != 2 {
		t.Fatalf("%d calls, want a retry after a minute and then a longer wait", f.n())
	}
	// It works again: the problem goes.
	f.err, f.out = nil, fixture(t, "prs.json")
	clock = now.Add(4 * time.Minute)
	s = snap(time.Time{})
	r.Apply(&s)
	r.Wait()
	s = snap(time.Time{})
	r.Apply(&s)
	if len(s.Missing) != 0 || !s.Threads[0].PR.Live {
		t.Errorf("after recovery: Missing %q, live %v", s.Missing, s.Threads[0].PR.Live)
	}
}

func TestRateLimit(t *testing.T) {
	reset := now.Add(20 * time.Minute)
	f := &fakeGh{
		out: []byte(`{"data":{"rateLimit":{"remaining":0,"resetAt":"` + reset.Format(time.RFC3339) + `"}},"errors":[{"type":"RATE_LIMITED","message":"API rate limit exceeded"}]}`),
		err: &RunError{Code: 1, Stderr: "gh: API rate limit exceeded"},
	}
	clock := now
	r := newReader(f, &clock)
	s := snap(time.Time{})
	r.Apply(&s)
	r.Wait()
	clock = reset.Add(-time.Second)
	r.Apply(&s)
	r.Wait()
	if f.n() != 1 {
		t.Errorf("%d calls before the reset", f.n())
	}
	clock = reset.Add(time.Second)
	r.Apply(&s)
	r.Wait()
	if f.n() != 2 {
		t.Errorf("%d calls; nothing after the reset", f.n())
	}

	// A budget nearly spent waits for its reset too.
	low := strings.Replace(string(fixture(t, "prs.json")), `"remaining": 4321`, `"remaining": 12`, 1)
	f2 := &fakeGh{out: []byte(low)}
	clock = now
	r2 := newReader(f2, &clock)
	r2.TTL = time.Minute
	s = snap(time.Time{})
	r2.Apply(&s)
	r2.Wait()
	clock = now.Add(5 * time.Minute)
	r2.Apply(&s)
	r2.Wait()
	if f2.n() != 1 {
		t.Errorf("%d calls with 12 points left before the reset", f2.n())
	}
}

func TestGhMissingOrLoggedOut(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		want string
	}{
		{"missing", &exec.Error{Name: "gh", Err: exec.ErrNotFound}, "gh is not installed"},
		{"logged out", &RunError{Code: 4, Stderr: "To get started with GitHub CLI, please run:  gh auth login"}, "gh is not logged in"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeGh{err: c.err}
			clock := now
			r := newReader(f, &clock)
			s := snap(time.Time{})
			r.Apply(&s)
			r.Wait()
			s = snap(time.Time{})
			r.Apply(&s)
			if len(s.Missing) != 0 || len(s.Notes) != 1 || !strings.Contains(s.Notes[0], c.want) || !strings.Contains(s.Notes[0], "ticker") {
				t.Errorf("Missing %q, Notes %q; want one note saying %q", s.Missing, s.Notes, c.want)
			}
			if s.Threads[0].PR.Live || s.Threads[0].PR.State != "OPEN" {
				t.Error("the ticker's PR data did not stay")
			}
		})
	}
}

func TestNoPRsNoCalls(t *testing.T) {
	f := &fakeGh{}
	r := &Reader{Run: f.run}
	s := deck.Snapshot{Threads: []deck.Thread{{ID: "t-0001"}}}
	r.Apply(&s)
	r.Wait()
	if f.n() != 0 || len(s.Notes) != 0 {
		t.Errorf("%d calls, notes %q, for a snapshot without PRs", f.n(), s.Notes)
	}
}

func TestTail(t *testing.T) {
	l := Tail(string(fixture(t, "job.log")), 200)
	if l.Step != "go test ./..." {
		t.Errorf("Step = %q", l.Step)
	}
	want := []string{
		"ok  \tacme/webshop/internal/admin\t0.101s",
		"--- FAIL: TestUsersPage (0.02s)",
		"    users_test.go:41: got 3 rows, want 4",
		"FAIL",
		"FAIL\tacme/webshop/internal/users\t0.214s",
		"Error: Process completed with exit code 1.",
	}
	if !slices.Equal(l.Lines, want) || l.More != 0 {
		t.Errorf("Lines = %q (more %d)\nwant %q", l.Lines, l.More, want)
	}

	short := Tail(string(fixture(t, "job.log")), 2)
	if short.More != 4 || !slices.Equal(short.Lines, want[4:]) {
		t.Errorf("a 2-line tail = %q, more %d", short.Lines, short.More)
	}

	// Without an error line, the log's last lines.
	plain := Tail("2026-10-02T14:30:01.0000000Z one\n2026-10-02T14:30:02.0000000Z ##[group]two\n2026-10-02T14:30:03.0000000Z three\n", 2)
	if plain.Step != "" || !slices.Equal(plain.Lines, []string{"two", "three"}) || plain.More != 1 {
		t.Errorf("no error: %+v", plain)
	}
}

func TestLog(t *testing.T) {
	f := &fakeGh{out: fixture(t, "job.log")}
	r := &Reader{Run: f.run}
	check := deck.Check{Name: "test (ubuntu-latest)", State: deck.CheckFailed, JobID: 9002}
	l, err := r.Log(context.Background(), prURL, check)
	if err != nil || l.Step != "go test ./..." {
		t.Fatalf("Log = %+v, %v", l, err)
	}
	if args := strings.Join(f.calls[0], " "); args != "api --hostname github.com --allow-escape-sequences repos/acme/webshop/actions/jobs/9002/logs" {
		t.Errorf("args = %s", args)
	}
	if _, err := r.Log(context.Background(), prURL, check); err != nil || f.n() != 1 {
		t.Errorf("a finished job's log was fetched again (%d calls, %v)", f.n(), err)
	}
	if _, err := r.Log(context.Background(), prURL, deck.Check{Name: "deploy-preview", State: deck.CheckFailed}); !errors.Is(err, ErrNoLog) {
		t.Errorf("a check outside Actions: %v, want ErrNoLog", err)
	}

	// An older gh without --allow-escape-sequences is asked without it.
	old := &fakeGh{}
	calls := 0
	r2 := &Reader{Run: func(ctx context.Context, args []string, limit int) ([]byte, error) {
		calls++
		if slices.Contains(args, "--allow-escape-sequences") {
			return nil, &RunError{Code: 1, Stderr: "unknown flag: --allow-escape-sequences"}
		}
		return old.run(ctx, args, limit)
	}}
	old.out = fixture(t, "job.log")
	if l, err := r2.Log(context.Background(), prURL, check); err != nil || calls != 2 || len(l.Lines) == 0 {
		t.Errorf("older gh: %d calls, %v", calls, err)
	}
}

func TestParsePR(t *testing.T) {
	for raw, ok := range map[string]bool{
		prURL: true,
		"https://ghe.example.com/acme/shop/pull/3": true,
		"https://github.com/acme/webshop/issues/3": false,
		"https://github.com/acme/webshop/pull/x":   false,
		"PR #2320":                                 false,
	} {
		if _, got := parsePR(raw); got != ok {
			t.Errorf("parsePR(%q) = %v, want %v", raw, got, ok)
		}
	}
	if ref, _ := parsePR("https://ghe.example.com/acme/shop/pull/3"); ref.Host != "ghe.example.com" || query([]prRef{ref})[3] != "ghe.example.com" {
		t.Errorf("an enterprise host is not passed to gh: %+v", ref)
	}
}

func TestTailWriter(t *testing.T) {
	w := &tailWriter{limit: 10}
	for range 5 {
		_, _ = w.Write([]byte("line one\nline two\n"))
	}
	if got := string(w.Bytes()); got != "line two\n" {
		t.Errorf("tail = %q, want the last whole line", got)
	}
}

func TestDetailParse(t *testing.T) {
	prs, _, err := parseResponse(fixture(t, "prs-detail.json"), 2)
	if err != nil {
		t.Fatal(err)
	}
	d := prs[0].Detail
	if d == nil {
		t.Fatal("no detail parsed")
	}
	if d.Title != "Document select for summary" || d.Author != "robin" || d.Head != "hp/admin-rebuild/t-0004-summary" || d.Base != "main" ||
		d.Additions != 214 || d.Deletions != 12 || d.ChangedFiles != 11 || !strings.HasPrefix(d.Body, "## Summary") {
		t.Errorf("detail = %+v", d)
	}
	if !slices.Equal(d.Labels, []string{"frontend", "needs-review"}) {
		t.Errorf("labels = %q", d.Labels)
	}
	// 53 comments, 3 read; 4 reviews, all read.
	if d.Earlier != 50 {
		t.Errorf("Earlier = %d, want 50", d.Earlier)
	}
	var got []string
	for _, c := range d.Comments {
		s := []string{"issue", "review", "thread"}[c.Kind] + ":" + c.Author
		switch {
		case c.Bot:
			s += ":bot"
		case c.Kind == deck.CommentReview:
			s += ":" + c.State
		case c.Resolved:
			s += ":resolved"
		}
		got = append(got, s)
	}
	// Time order; the pending review is the viewer's own draft and left
	// out; a deleted account is "ghost".
	want := []string{"issue:vercel:bot", "issue:alex", "review:sam:CHANGES_REQUESTED", "issue:ghost", "thread:alex:resolved",
		"thread:sam", "review:sam:COMMENTED", "thread:sam", "thread:alex", "review:alex:APPROVED"}
	if !slices.Equal(got, want) {
		t.Errorf("comments = %q\nwant %q", got, want)
	}
	if c := d.Comments[6]; c.Inline != 1 || c.URL == "" {
		t.Errorf("the comment-only review = %+v", c)
	}
	// sam's later comment-only review keeps the request for changes.
	if !slices.Equal(d.Reviewers, []deck.Reviewer{{Login: "sam", State: "CHANGES_REQUESTED"}, {Login: "alex", State: "APPROVED"}}) {
		t.Errorf("reviewers = %+v", d.Reviewers)
	}
	// Without the detail fields there is no detail.
	plain, _, _ := parseResponse(fixture(t, "prs.json"), 2)
	if plain[0].Detail != nil {
		t.Error("a read without the detail fields has a detail")
	}
}

func TestDetailOnDemand(t *testing.T) {
	f := &fakeGh{out: fixture(t, "prs.json")}
	clock := now
	r := newReader(f, &clock)
	s := snap(time.Time{})
	r.Apply(&s)
	r.Wait()
	if args := strings.Join(f.calls[0], " "); strings.Contains(args, "deckDetail") {
		t.Errorf("the detail was asked for before the PR tab showed: %s", args)
	}

	// The PR tab opens: the PR is read at once, with its detail, though
	// the cache is fresh.
	r.Focus(prURL)
	f.mu.Lock()
	f.out = fixture(t, "prs-detail.json")
	f.mu.Unlock()
	r.Detail(prURL)
	r.Wait()
	if f.n() != 2 {
		t.Fatalf("%d calls; opening the tab did not read the detail", f.n())
	}
	args := strings.Join(f.calls[1], " ")
	if !strings.Contains(args, "...deckPR ...deckDetail") || !strings.Contains(args, "fragment deckDetail on PullRequest") {
		t.Errorf("the detail read lacks the fragment: %s", args)
	}
	s = snap(time.Time{})
	r.Apply(&s)
	if d := s.Threads[0].PR.Detail; d == nil || d.Title == "" || s.Threads[0].PR.DetailNote != "" {
		t.Fatalf("detail = %+v, note %q", d, s.Threads[0].PR.DetailNote)
	}
	r.Wait()
	if f.n() != 2 {
		t.Errorf("%d calls; a cached detail was read again", f.n())
	}

	// The PR's own refresh reads the detail too.
	clock = now.Add(FocusTTL + time.Second)
	r.Focus(prURL)
	r.Wait()
	if f.n() != 3 || !strings.Contains(strings.Join(f.calls[2], " "), "deckDetail") {
		t.Errorf("%d calls; the focused refresh did not ask for the detail", f.n())
	}

	// The tab closes: later reads leave the detail out, but keep the one
	// read before.
	r.Detail("")
	f.mu.Lock()
	f.out = fixture(t, "prs.json")
	f.mu.Unlock()
	clock = now.Add(2*FocusTTL + 2*time.Second)
	r.Focus(prURL)
	r.Wait()
	if f.n() != 4 || strings.Contains(strings.Join(f.calls[3], " "), "deckDetail") {
		t.Errorf("%d calls; the detail was asked for with the tab closed", f.n())
	}
	s = snap(time.Time{})
	r.Apply(&s)
	if s.Threads[0].PR.Detail == nil {
		t.Error("a read without the detail dropped the one read before")
	}
}

func TestDetailNotes(t *testing.T) {
	// A merged PR's detail is read once, though merged PRs are left alone.
	f := &fakeGh{out: fixture(t, "prs-detail.json")}
	clock := now
	r := newReader(f, &clock)
	merged := "https://github.com/acme/webshop/pull/8"
	r.Detail(merged)
	r.Wait()
	if f.n() != 1 {
		t.Fatalf("%d calls for a merged PR's detail", f.n())
	}

	// A failed read says why, and the reader backs off.
	f2 := &fakeGh{err: &RunError{Code: 1, Stderr: "error connecting to api.github.com"}}
	r2 := newReader(f2, &clock)
	r2.Detail(prURL)
	r2.Wait()
	s := snap(time.Time{})
	r2.Apply(&s)
	r2.Wait()
	if note := s.Threads[0].PR.DetailNote; !strings.Contains(note, "could not read it: gh: error connecting") {
		t.Errorf("note = %q", note)
	}
	if f2.n() != 1 {
		t.Errorf("%d calls; no back-off for the detail", f2.n())
	}

	// GitHub does not know the PR.
	f3 := &fakeGh{out: fixture(t, "prs.json")}
	r3 := newReader(f3, &clock)
	s = snap(time.Time{})
	r3.Apply(&s)
	r3.Wait()
	r3.Detail(goneURL)
	r3.Wait()
	s = snap(time.Time{})
	r3.Apply(&s)
	if note := s.Threads[1].PR.DetailNote; !strings.Contains(note, "no such pull request") {
		t.Errorf("note for a PR GitHub does not know = %q", note)
	}
	r3.Wait()
}

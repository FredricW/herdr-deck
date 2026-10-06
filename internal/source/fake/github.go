package fake

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// GitHub lays what the deck's own GitHub read would add over the sample's
// PR (t-0004's #2320): a failing and a running check, a review asked of
// sam and two unresolved review threads.
func GitHub(snap *deck.Snapshot, now time.Time) {
	for i := range snap.Threads {
		pr := snap.Threads[i].PR
		if pr == nil || pr.Number != 2320 {
			continue
		}
		run := "https://github.com/acme/webshop/actions/runs/4242/job/"
		pr.Live = true
		pr.CheckedAt = now.Add(-20 * time.Second)
		pr.Base = "main"
		pr.MergeState, pr.Mergeable = "BLOCKED", "MERGEABLE"
		pr.ReviewRequests = []string{"sam"}
		pr.Checks = []deck.Check{
			{Name: "lint", State: deck.CheckFailed, URL: run + "9001", JobID: 9001, Started: now.Add(-6 * time.Minute), Completed: now.Add(-4 * time.Minute)},
			{Name: "test (macos-latest)", State: deck.CheckRunning, URL: run + "9003", JobID: 9003, Started: now.Add(-6 * time.Minute)},
			{Name: "test (ubuntu-latest)", State: deck.CheckPassed, URL: run + "9002", JobID: 9002, Started: now.Add(-6 * time.Minute), Completed: now.Add(-2 * time.Minute)},
			{Name: "version", State: deck.CheckPassed, URL: run + "9004", JobID: 9004, Started: now.Add(-6 * time.Minute), Completed: now.Add(-5 * time.Minute)},
		}
		pr.FailingChecks = []string{"lint"}
		pr.Threads = []deck.ReviewThread{
			{Path: "src/pages/users/index.ts", Line: 41, Author: "sam", Body: "Should this be **paginated**? The list can get long.",
				URL: pr.URL + "#discussion_r101", At: now.Add(-2 * time.Hour)},
			{Path: "src/pages/users/table.tsx", Line: 12, Author: "alex", Body: "Rename to `UserRow`.", Replies: 2,
				URL: pr.URL + "#discussion_r102", At: now.Add(-90 * time.Minute)},
			{Path: "src/api/users.ts", Line: 7, Outdated: true, Author: "sam", Body: "Drop the `any` here.",
				URL: pr.URL + "#discussion_r103", At: now.Add(-3 * time.Hour)},
		}
		pr.Detail = gitHubDetail(pr, now)
	}
}

// gitHubDetail is the sample PR's description and conversation, for the
// PR tab: a deploy bot, a question, a request for changes, the review
// threads (one of them resolved) and an approval.
func gitHubDetail(pr *deck.PullRequest, now time.Time) *deck.PRDetail {
	d := &deck.PRDetail{
		Title: "Document select for summary", Author: "robin",
		Head: "hp/admin-rebuild/t-0004-summary", Base: "main",
		Body: "## Summary\n\nAdds the document picker to the summary page (ABC-1191).\n\n" +
			"- [x] picker with search\n- [ ] empty state\n\nDesign: Figma 598-48083.",
		Created: now.Add(-50 * time.Hour), Updated: now.Add(-20 * time.Minute),
		Labels:    []string{"frontend", "needs-review"},
		Additions: 214, Deletions: 12, ChangedFiles: 11,
		Reviewers: []deck.Reviewer{{Login: "alex", State: "COMMENTED"}, {Login: "kim", State: "APPROVED"}},
	}
	c := func(c deck.PRComment) { d.Comments = append(d.Comments, c) }
	c(deck.PRComment{Kind: deck.CommentIssue, Author: "deploy-preview", Bot: true, At: now.Add(-49 * time.Hour),
		Body: "Preview deployed to https://preview.example.com/2320", URL: pr.URL + "#issuecomment-201"})
	c(deck.PRComment{Kind: deck.CommentIssue, Author: "alex", At: now.Add(-5 * time.Hour),
		Body: "Can we keep the old picker behind a flag for a week?", URL: pr.URL + "#issuecomment-202"})
	c(deck.PRComment{Kind: deck.CommentThread, Author: "kim", At: now.Add(-4 * time.Hour), Resolved: true,
		Path: "src/pages/summary/picker.tsx", Line: 18, Replies: 1, Body: "Typo in the label.", URL: pr.URL + "#discussion_r100"})
	for _, t := range pr.Threads {
		c(deck.PRComment{Kind: deck.CommentThread, Author: t.Author, At: t.At, Path: t.Path, Line: t.Line,
			Outdated: t.Outdated, Replies: t.Replies, Body: t.Body, URL: t.URL})
	}
	c(deck.PRComment{Kind: deck.CommentReview, Author: "alex", State: "COMMENTED", Inline: 1, At: now.Add(-90 * time.Minute),
		URL: pr.URL + "#pullrequestreview-301"})
	c(deck.PRComment{Kind: deck.CommentReview, Author: "kim", State: "APPROVED", At: now.Add(-time.Hour),
		Body: "Looks good once **lint** passes.", URL: pr.URL + "#pullrequestreview-302"})
	slices.SortStableFunc(d.Comments, func(a, b deck.PRComment) int { return a.At.Compare(b.At) })
	return d
}

// CheckLog is the sample's failing lint job's log tail.
func CheckLog(_ context.Context, _ string, c deck.Check) (deck.CheckLog, error) {
	if c.JobID != 9001 {
		return deck.CheckLog{}, errors.New("the sample has no log for " + c.Name)
	}
	return deck.CheckLog{
		Step: "golangci-lint run ./...",
		Lines: []string{
			"level=info msg=\"[runner] linters took 4.2s\"",
			"internal/users/page.go:88:2: ineffectual assignment to rows (ineffassign)",
			"\trows = filter(rows)",
			"\t^",
			"internal/users/table.go:12:6: type `userRow` is unused (unused)",
			"2 issues:",
			"* ineffassign: 1",
			"* unused: 1",
			"Error: issues found",
			"Error: Process completed with exit code 1.",
		},
	}, nil
}

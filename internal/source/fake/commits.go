package fake

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/diff"
)

// sampleCommit is one of t-0002's made-up commits: its files are keys of
// patches, shown as the commit's diff.
type sampleCommit struct {
	sha, subject, body string
	age                time.Duration
	merge              bool
	added, deleted     int
	files              []string
}

var sampleCommits = []sampleCommit{
	{sha: "9c4e1f7a2b", subject: "Test the users table and its filters", age: 25 * time.Minute, added: 64,
		files: []string{"src/admin/users/users.test.tsx"}},
	{sha: "7a03d5e8c1", subject: "Show the overview cards above the table", age: 70 * time.Minute, added: 58, deleted: 12,
		body:  "The cards count active and invited users (ABC-1256). They\nread the same query as the table, so the filters apply to both.",
		files: []string{"src/admin/users/UsersPage.tsx", "src/admin/users/overview.ts"}},
	{sha: "e51b9024dd", subject: "Merge origin/main into the users page", age: 3 * time.Hour, merge: true},
	{sha: "3f8a6c1e90", subject: "Replace the members list with a users table", age: 5 * time.Hour, added: 180, deleted: 75,
		body:  "UsersTable sorts and filters on the server; the old list\nloaded every member at once.",
		files: []string{"src/admin/users/UsersTable.tsx", "src/admin/users/UsersList.tsx", "src/api/users.ts"}},
	{sha: "b2d7e40f13", subject: "Rename the members page to users", age: 21 * time.Hour, added: 9, deleted: 9,
		files: []string{"src/admin/users/UsersPage.tsx"}},
}

// Commits is the Commits tab's sample: t-0002's branch with a few
// made-up commits, a merge among them, and two files not committed yet.
func Commits(_ context.Context, t deck.Thread) deck.Commits {
	if t.ID != "t-0002" {
		return deck.Commits{Base: "origin/main"}
	}
	now := time.Now()
	cs := deck.Commits{Base: "origin/main", MergeBase: "4f1c2a9", Head: sampleCommits[0].sha, Uncommitted: 2, Upstream: true}
	for i, c := range sampleCommits {
		cs.List = append(cs.List, deck.Commit{
			SHA: c.sha, Short: c.sha[:7], Author: "Agent", Time: now.Add(-c.age), Subject: c.subject,
			Merge: c.merge, Added: c.added, Deleted: c.deleted, Pushed: i > 0,
		})
	}
	return cs
}

// CommitPatch is the preview of one of the sample's commits.
func CommitPatch(_ context.Context, t deck.Thread, sha string) deck.CommitPatch {
	for _, c := range sampleCommits {
		if c.sha != sha || t.ID != "t-0002" {
			continue
		}
		var b strings.Builder
		b.WriteString("\x1e" + c.body + "\x1e\n")
		files := c.files
		if c.merge {
			files = []string{"README.md"}
		}
		for _, f := range files {
			body, ok := patches[f]
			if !ok {
				body = "@@ -1,3 +1,4 @@\n # Admin\n \n+The users page lists everyone in the workspace.\n Run `pnpm dev` to start it.\n"
			}
			fmt.Fprintf(&b, "diff --git a/%s b/%s\n%s", f, f, body)
		}
		return diff.ParseShow([]byte(b.String()), diff.MaxPatchLines)
	}
	return deck.CommitPatch{Patch: deck.Patch{Note: "no such commit"}}
}

package ui

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/diff"
)

// sampleCommits is t-0002's made-up branch: two commits, a merge of main
// and the first commit, with two files not committed yet.
func sampleCommits() deck.Commits {
	return deck.Commits{
		Base: "origin/main", MergeBase: "4b825dc", Head: "c3a91f0e", Uncommitted: 2, Upstream: true,
		List: []deck.Commit{
			{SHA: "c3a91f0e", Short: "c3a91f0", Author: "Agent", Time: now.Add(-12 * time.Minute), Subject: "Show the overview cards above the users table", Added: 214, Deleted: 12},
			{SHA: "8be2d417", Short: "8be2d41", Author: "Agent", Time: now.Add(-2 * time.Hour), Subject: "Add the users columns", Added: 48, Pushed: true},
			{SHA: "51f0c9aa", Short: "51f0c9a", Author: "Agent", Time: now.Add(-5 * time.Hour), Subject: "Merge origin/main into the users page", Merge: true, Pushed: true},
			{SHA: "a07de3b5", Short: "a07de3b", Author: "Agent", Time: now.Add(-26 * time.Hour), Subject: "Rename members to users", Added: 1, Deleted: 1, Pushed: true},
		},
	}
}

// commitShow is a made-up `git show` of the newest sample commit.
const commitShow = "\x1eThe cards count active and invited users (ABC-1256).\n\nThey read the table's query.\n\x1e\n" +
	"diff --git a/apps/admin/src/pages/users/UsersOverviewPage.tsx b/apps/admin/src/pages/users/UsersOverviewPage.tsx\n" +
	"--- a/apps/admin/src/pages/users/UsersOverviewPage.tsx\n+++ b/apps/admin/src/pages/users/UsersOverviewPage.tsx\n" +
	"@@ -1,4 +1,5 @@\n import { useState } from \"react\";\n+import { overview } from \"./overview\";\n \n-export function MembersPage() {\n+export function UsersOverviewPage() {\n" +
	"diff --git a/apps/admin/src/pages/users/overview.ts b/apps/admin/src/pages/users/overview.ts\n" +
	"new file mode 100644\n--- /dev/null\n+++ b/apps/admin/src/pages/users/overview.ts\n" +
	"@@ -0,0 +1,3 @@\n+export function overview(users: User[]) {\n+  return { active: users.length };\n+}\n"

// withCommits gives the model changes() and sampleCommits() for t-0002,
// with a CommitPatch that records the shas asked for and an OpenCommit
// that records what it opened.
func withCommits(cs deck.Commits, shas *[]string) func(*Options) {
	return func(o *Options) {
		withPatches(nil)(o)
		o.Commits = func(_ context.Context, t deck.Thread) deck.Commits {
			if t.Worktree != worktree2 {
				return deck.Commits{Base: "origin/main"}
			}
			return cs
		}
		o.CommitFiles = func(_ context.Context, _ deck.Thread, sha string) ([]deck.DiffFile, error) {
			if sha == "c3a91f0e" {
				return []deck.DiffFile{
					{Path: "apps/admin/src/pages/users/UsersOverviewPage.tsx", Added: 166, Deleted: 12},
					{Path: "apps/admin/src/pages/users/overview.ts", Change: deck.ChangeAdded, Added: 48},
					{Path: "apps/admin/src/pages/users/index.ts", OldPath: "apps/admin/src/pages/members/index.ts", Change: deck.ChangeRenamed},
					{Path: "apps/admin/src/old.ts", Change: deck.ChangeDeleted, Deleted: 9},
				}, nil
			}
			return []deck.DiffFile{{Path: "apps/admin/src/pages/users/columns.ts", Change: deck.ChangeAdded, Added: 48}}, nil
		}
		o.CommitFilePatch = func(_ context.Context, _ deck.Thread, sha string, f deck.DiffFile) deck.Patch {
			if shas != nil {
				*shas = append(*shas, sha+" "+f.Path)
			}
			return diff.ParsePatch([]byte("@@ -0,0 +1 @@\n+export const "+path.Base(f.Path)+" = 1;\n"), diff.MaxPatchLines)
		}
		o.CommitPatch = func(_ context.Context, _ deck.Thread, sha string) deck.CommitPatch {
			if shas != nil {
				*shas = append(*shas, sha)
			}
			return diff.ParseShow([]byte(commitShow), diff.MaxPatchLines)
		}
	}
}

func commitsModel(t *testing.T, w, h int, cs deck.Commits, shas *[]string) (Model, *[]string) {
	t.Helper()
	var opened []string
	m, _ := newModelWith(t, deck.Snapshot{}, w, h, func(o *Options) {
		withCommits(cs, shas)(o)
		o.OpenCommit = func(path, sha string, files []string) error {
			opened = append(opened, strings.TrimSpace(path+" "+sha+" "+strings.Join(files, " ")))
			return nil
		}
	})
	m, _ = press(m, snapshotMsg(calm()))
	return m, &opened
}

func TestCommitsGolden(t *testing.T) {
	gone := deck.Commits{Base: "origin/main", Note: "the worktree is gone"}
	empty := deck.Commits{Base: "origin/main", MergeBase: "4b825dc"}
	many := sampleCommits()
	for i := range 12 {
		many.List = append(many.List, deck.Commit{SHA: fmt.Sprintf("%08x", i), Short: fmt.Sprintf("%07x", i), Time: now.Add(-time.Duration(30+i) * time.Hour), Subject: fmt.Sprintf("Step %d of the table", i+1), Added: i, Deleted: 1})
	}
	many.More = 23
	cases := []struct {
		name string
		cs   deck.Commits
		keys []tea.Msg
	}{
		{name: "commits", cs: sampleCommits(), keys: keys("]]")},
		{name: "commits-cursor", cs: sampleCommits(), keys: append(keys("]]"), tabKey, tea.KeyPressMsg{Code: 'j', Text: "j"})},
		{name: "commits-full", cs: many, keys: keys("]]z")},
		{name: "commits-empty", cs: empty, keys: keys("]]")},
		{name: "commits-note", cs: gone, keys: keys("]]")},
		{name: "commits-preview", cs: sampleCommits(), keys: keys("]]1")},
		{name: "commits-preview-scrolled", cs: sampleCommits(), keys: keys("]]1JJJJ")},
		{name: "commits-expanded", cs: sampleCommits(), keys: append(append(keys("]]"), tabKey), keys("j jjjjj ")...)},
		{name: "commits-expanded-tree", cs: sampleCommits(), keys: keys("dt]j ")},
		{name: "commits-file-preview", cs: sampleCommits(), keys: append(append(keys("]]"), tabKey), append(keys("j j"), enterKey)...)},
	}
	for _, c := range cases {
		for _, w := range []int{80, 60} {
			name := fmt.Sprintf("%s-%d", c.name, w)
			t.Run(name, func(t *testing.T) {
				m, _ := commitsModel(t, w, 28, c.cs, nil)
				m, _ = press(m, c.keys...)
				golden(t, name, m)
			})
		}
	}
}

// t-0003 is resolved or has no worktree: the tab says so instead.
func TestCommitsNotes(t *testing.T) {
	m, _ := commitsModel(t, 80, 28, sampleCommits(), nil)
	m, _ = press(m, keys("]]")...)
	if !strings.Contains(screen(m), "Commits 4") {
		t.Fatalf("the tab does not count the commits:\n%s", screen(m))
	}
	s := calm()
	i := slices.IndexFunc(s.Threads, func(t deck.Thread) bool { return t.ID == "t-0002" })
	s.Threads[i].Status = deck.StatusDone
	m, _ = press(m, snapshotMsg(s))
	if m.curTab() != tabCommits || !strings.Contains(screen(m), "t-0002 is resolved") {
		t.Errorf("a resolved thread:\n%s", screen(m))
	}
	s = calm()
	s.Threads[i].Worktree = ""
	m, _ = press(m, snapshotMsg(s))
	if !strings.Contains(screen(m), "no worktree") {
		t.Errorf("no worktree:\n%s", screen(m))
	}
}

func TestCommitsKeysAndFocus(t *testing.T) {
	var shas []string
	m, opened := commitsModel(t, 80, 28, sampleCommits(), &shas)
	m, _ = press(m, keys("]]")...)
	if m.curTab() != tabCommits || m.dfocus {
		t.Fatalf("] ] did not show Commits: %v", m.curTab())
	}
	// ↵ on the uncommitted row goes to the Files tab.
	m, _ = press(m, tabKey, enterKey)
	if m.curTab() != tabFiles || !m.dfocus {
		t.Fatalf("↵ on the uncommitted row: tab %v focus %v", m.curTab(), m.dfocus)
	}
	m, _ = press(m, esc)
	m, _ = press(m, keys("]")...)
	if m.curTab() != tabCommits {
		t.Fatalf("back on Commits: %v", m.curTab())
	}
	// A digit previews that commit and puts the cursor on it.
	m, _ = press(m, keys("2")...)
	if !m.preview || !m.dfocus || m.dcur != 2 || shas[len(shas)-1] != "8be2d417" {
		t.Fatalf("2: preview %v focus %v cursor %d shas %v", m.preview, m.dfocus, m.dcur, shas)
	}
	if !strings.Contains(screen(m), "8be2d41 Add the users columns") {
		t.Errorf("the preview's header:\n%s", screen(m))
	}
	// k moves to the newest commit, but not onto the uncommitted row.
	m, _ = press(m, keys("kk")...)
	if m.dcur != 1 || !m.preview || shas[len(shas)-1] != "c3a91f0e" {
		t.Errorf("k k: cursor %d preview %v shas %v", m.dcur, m.preview, shas)
	}
	// d opens the commit in the diff tool; g on GitHub, in the PR.
	m, _ = press(m, keys("d")...)
	if !slices.Equal(*opened, []string{worktree2 + " c3a91f0e"}) {
		t.Errorf("d opened %v", *opened)
	}
	s := calm()
	s.Threads[slices.IndexFunc(s.Threads, func(t deck.Thread) bool { return t.ID == "t-0002" })].PR = &deck.PullRequest{URL: "https://github.com/acme/webshop/pull/2320", Number: 2320}
	m, o := newModelWith(t, deck.Snapshot{}, 80, 28, withCommits(sampleCommits(), nil))
	m, _ = press(m, snapshotMsg(s))
	m, _ = press(m, keys("]]")...)
	m, _ = press(m, tabKey)
	m, _ = press(m, keys("jg")...)
	if len(o.urls) != 0 || !strings.Contains(m.Status(), "not pushed yet") {
		t.Errorf("g on an unpushed commit: %v %q", o.urls, m.Status())
	}
	m, _ = press(m, keys("jg")...)
	if !slices.Equal(o.urls, []string{"https://github.com/acme/webshop/pull/2320/commits/8be2d417"}) {
		t.Errorf("g opened %v", o.urls)
	}
	// From the list, g is the PR's link key again.
	m, _ = press(m, esc)
	m, _ = press(m, keys("g")...)
	if len(o.urls) != 1 || strings.Contains(m.Status(), "commit") {
		t.Errorf("g from the list opened %v (%q)", o.urls, m.Status())
	}
	// v toggles; esc leaves the preview and gives the list back.
	m, _ = press(m, keys("v")...)
	if !m.preview || m.dcur != 2 {
		t.Fatalf("v: preview %v cursor %d", m.preview, m.dcur)
	}
	m, _ = press(m, esc)
	if m.preview || m.dfocus || !strings.Contains(screen(m), "WORK") {
		t.Errorf("esc did not restore the list:\n%s", screen(m))
	}
}

func TestCommitsMergeAndMore(t *testing.T) {
	m, _ := commitsModel(t, 80, 40, sampleCommits(), nil)
	m, _ = press(m, keys("]]")...)
	out := screen(m)
	if !strings.Contains(out, "⋔ Merge origin/main") || !strings.Contains(out, "● uncommitted · 2 files") {
		t.Errorf("merge or uncommitted row missing:\n%s", out)
	}
	if strings.Contains(out, "Merge origin/main into the users page  +0") {
		t.Errorf("a merge shows counts:\n%s", out)
	}
}

func TestCommitURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/acme/webshop/pull/12":       "https://github.com/acme/webshop/pull/12/commits/abc",
		"https://github.com/acme/webshop/pull/12/files": "https://github.com/acme/webshop/pull/12/commits/abc",
		"https://gitlab.com/acme/webshop/pull/12":       "",
		"https://github.com/acme/webshop":               "",
	}
	for in, want := range cases {
		if got := commitURL(deck.PullRequest{URL: in}, "abc"); got != want {
			t.Errorf("commitURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// Commits are read when the selection moves to another thread and on every
// reload, one read at a time.
func TestCommitsReadsFollowSelection(t *testing.T) {
	var calls []string
	m, _ := newModelWith(t, deck.Snapshot{}, 80, 28, func(o *Options) {
		o.Commits = func(_ context.Context, t deck.Thread) deck.Commits {
			calls = append(calls, t.ID)
			return deck.Commits{Base: "origin/main"}
		}
	})
	m, _ = press(m, snapshotMsg(calm()))
	m, _ = press(m, keys("j")...)
	m, _ = press(m, keys("k")...)
	m, _ = press(m, snapshotMsg(calm()))
	// t-0003 has no worktree, and back on t-0002 the read is not repeated.
	if want := []string{"t-0002", "t-0002"}; !slices.Equal(calls, want) {
		t.Errorf("reads %v, want %v", calls, want)
	}
	// While a read runs, a move only marks another one.
	m.committing = true
	m.commitsSel = ""
	if cmd := m.readCommits(false); cmd != nil || !m.commitsAgain {
		t.Errorf("a second read started while one ran")
	}
}

// Another tab ends the preview, as on Files.
func TestCommitsPreviewEndsOnTabSwitch(t *testing.T) {
	cs := sampleCommits()
	cs.Uncommitted = 0
	m, _ := commitsModel(t, 80, 28, cs, nil)
	m, _ = press(m, keys("d")...)
	m, _ = press(m, keys("v")...)
	if !m.preview {
		t.Fatal("no file preview")
	}
	m, _ = press(m, keys("]")...)
	if m.preview || m.curTab() != tabCommits {
		t.Errorf("] kept the preview on %v", m.curTab())
	}
	m, _ = press(m, keys("v")...)
	if !m.preview || !strings.Contains(screen(m), "c3a91f0 Show the overview") {
		t.Fatalf("v on Commits without an uncommitted row:\n%s", screen(m))
	}
	m, _ = press(m, keys("[")...)
	if m.preview {
		t.Error("[ kept the commit preview")
	}
}

// When rows come or go above it, the cursor and the preview stay on the
// same commit.
func TestCommitsCursorFollowsCommit(t *testing.T) {
	cs := sampleCommits()
	cs.Uncommitted = 0
	m, _ := newModelWith(t, deck.Snapshot{}, 80, 28, withCommits(cs, nil))
	m, _ = press(m, snapshotMsg(calm()))
	m, _ = press(m, keys("]]2")...)
	if !m.preview || m.dcur != 1 {
		t.Fatalf("2: preview %v cursor %d", m.preview, m.dcur)
	}
	more := sampleCommits() // with the uncommitted row, and a new commit
	more.List = append([]deck.Commit{{SHA: "ffff0000", Short: "ffff000", Subject: "Newer"}}, more.List...)
	m, _ = press(m, commitsMsg{key: diffKey(m.snap.Threads[1]), commits: more})
	if _, c, ok := m.previewCommitAt(); !ok || c.SHA != "8be2d417" || !m.preview || m.dcur != 3 {
		t.Errorf("the cursor moved off 8be2d41: %+v cursor %d preview %v", c, m.dcur, m.preview)
	}
}

func TestCommitsExpand(t *testing.T) {
	var shas []string
	m, opened := commitsModel(t, 80, 28, sampleCommits(), &shas)
	m, _ = press(m, keys("]]")...)
	// From the list, l is still Linear's key and space folds.
	m, _ = press(m, keys("l")...)
	if len(m.expand) != 0 || m.choosing != deck.LinkLinear {
		t.Fatalf("l from the list: expanded %v, chooser %v", m.expand, m.choosing)
	}
	m, _ = press(m, esc)
	m, _ = press(m, keys("]]")...)
	m, _ = press(m, tabKey)
	m, _ = press(m, keys("jl")...)
	if !strings.Contains(screen(m), "▾ c3a91f0") || !strings.Contains(screen(m), "M  apps/admin/src/pages/users/UsersOverviewPage.tsx") {
		t.Fatalf("l did not expand c3a91f0:\n%s", screen(m))
	}
	// The digits stay the commits'.
	if !strings.Contains(screen(m), "2 ▸ 8be2d41") {
		t.Errorf("the second commit lost its digit:\n%s", screen(m))
	}
	// j onto the first file, ↵ previews its change in that commit.
	m, _ = press(m, keys("j")...)
	m, _ = press(m, enterKey)
	if !m.preview || shas[len(shas)-1] != "c3a91f0e apps/admin/src/pages/users/UsersOverviewPage.tsx" {
		t.Fatalf("↵ on a file: preview %v, reads %v", m.preview, shas)
	}
	if !strings.Contains(screen(m), "c3a91f0 apps/admin/src/pages/users/UsersOverviewPage.tsx  +166 −12") {
		t.Errorf("the file preview's header:\n%s", screen(m))
	}
	// d opens that file at that commit; a rename passes both paths.
	m, _ = press(m, keys("d")...)
	m, _ = press(m, keys("jj")...)
	m, _ = press(m, keys("d")...)
	want := []string{
		worktree2 + " c3a91f0e apps/admin/src/pages/users/UsersOverviewPage.tsx",
		worktree2 + " c3a91f0e apps/admin/src/pages/members/index.ts apps/admin/src/pages/users/index.ts",
	}
	if !slices.Equal(*opened, want) {
		t.Errorf("d opened %q, want %q", *opened, want)
	}
	// h collapses from a file and puts the cursor on its commit.
	m, _ = press(m, keys("h")...)
	if m.dcur != 1 || strings.Contains(screen(m), "overview.ts") {
		t.Errorf("h: cursor %d\n%s", m.dcur, screen(m))
	}
	// A click on the marker expands; another collapses.
	x, y := find(t, m, "▸ 8be2d41")
	m, _ = press(m, click(x, y))
	if !strings.Contains(screen(m), "columns.ts") {
		t.Errorf("a click on ▸ did not expand:\n%s", screen(m))
	}
	m, _ = press(m, click(x, y))
	if strings.Contains(screen(m), "columns.ts") {
		t.Errorf("a click on ▾ did not collapse:\n%s", screen(m))
	}
}

// Expanding or collapsing a commit above the cursor, files arriving and
// commits added above keep the cursor on the same row.
func TestCommitsExpandKeepsCursor(t *testing.T) {
	m, _ := commitsModel(t, 80, 40, sampleCommits(), nil)
	m, _ = press(m, keys("]]")...)
	m, _ = press(m, tabKey)
	m, _ = press(m, keys("j j")...) // expand c3a91f0, then onto its first file
	m, _ = press(m, keys("jjjj")...)
	if f, ok := m.cursorCommitFile(); ok || m.cursorCommit(sampleCommits()) != 1 {
		t.Fatalf("not on 8be2d41: %v %v", f, m.dcur)
	}
	m, _ = press(m, keys(" ")...) // expand 8be2d41
	m, _ = press(m, keys("j")...) // its file
	before := m.dcur
	// Files of the commit above collapse: the cursor follows its row.
	m.keepCursor(func() { delete(m.expand, commitKey(m.snap.Threads[1], "c3a91f0e")) })
	if f, ok := m.cursorCommitFile(); !ok || f.Path != "apps/admin/src/pages/users/columns.ts" || m.dcur != before-4 {
		t.Errorf("after collapsing above: %v %v cursor %d (was %d)", f, ok, m.dcur, before)
	}
	more := sampleCommits()
	more.List = append([]deck.Commit{{SHA: "ffff0000", Short: "ffff000", Subject: "Newer"}}, more.List...)
	m, _ = press(m, commitsMsg{key: diffKey(m.snap.Threads[1]), commits: more})
	if f, ok := m.cursorCommitFile(); !ok || f.Path != "apps/admin/src/pages/users/columns.ts" {
		t.Errorf("after a new commit: %v %v", f, ok)
	}
}

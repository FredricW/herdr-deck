package projects

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// testdata/admin-rebuild is a made-up project in the shape herdr-projects
// writes (PROJECT.md, threads, ticker and handled inbox); testdata/thread-list.json
// is what `herdr-projects thread list admin-rebuild --json` prints for it.
// testdata/busy is hand-written: a PR, a resolved and a failed thread, a
// broken thread file and unhandled inbox items.

// jsonRun answers `thread list` with the captured JSON.
func jsonRun(t *testing.T) RunFunc {
	t.Helper()
	out, err := os.ReadFile(filepath.Join("testdata", "thread-list.json"))
	if err != nil {
		t.Fatal(err)
	}
	return func(context.Context, string, ...string) ([]byte, error) { return out, nil }
}

func failRun(context.Context, string, ...string) ([]byte, error) {
	return nil, &exec.ExitError{Stderr: []byte("herdr-projects: no session\nmore detail\n")}
}

func reader(slug string, run RunFunc) Reader {
	return Reader{Root: "testdata", Slug: slug, Run: run}
}

func ids(threads []deck.Thread) []string {
	var out []string
	for _, t := range threads {
		out = append(out, t.ID)
	}
	return out
}

func TestReadAdminRebuildFromThreadList(t *testing.T) {
	snap := reader("admin-rebuild", jsonRun(t)).Read(context.Background())

	if len(snap.Missing) != 0 {
		t.Errorf("Missing = %q, want none", snap.Missing)
	}
	p := snap.Project
	if p.Slug != "admin-rebuild" || p.Name != "Admin rebuild" || p.Title() != "Admin rebuild" {
		t.Errorf("Project = %+v", p)
	}
	if !strings.HasPrefix(p.Goal, "Complete the Linear project 'Admin rebuild'") {
		t.Errorf("Goal = %q", p.Goal)
	}
	if want := []string{"/home/dev/src/webshop"}; !reflect.DeepEqual(p.Repos, want) {
		t.Errorf("Repos = %q, want %q", p.Repos, want)
	}

	if got, want := ids(snap.Threads), []string{"t-0001", "t-0002", "t-0003", "t-0004"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("thread ids = %q, want %q", got, want)
	}
	for _, th := range snap.Threads {
		if th.Status != deck.StatusNeedsYou {
			t.Errorf("%s Status = %v, want needs you", th.ID, th.Status)
		}
		if th.PR != nil || len(th.Links) != 0 {
			t.Errorf("%s has PR %+v / links %+v, want none", th.ID, th.PR, th.Links)
		}
	}
	t2 := snap.Threads[1]
	if t2.Title != "Members /admin/users" || t2.StateLine != "needs you · ~95%" || t2.PaneID != "w1Z:p1" ||
		t2.Activity != "Waiting for you" || t2.Branch != "hp/admin-rebuild/t-0002-members-admin-users" || t2.Base != "origin/main" ||
		t2.Worktree != "/home/dev/worktrees/webshop/hp-admin-rebuild-t-0002-members-admin-users" {
		t.Errorf("t-0002 = %+v", t2)
	}
	wantNext := []string{
		"Resume once the merge decision is made, and tell me which data source to use",
		`Decide what the "Archive plan" button should do (see findings.md)`,
		"Remove the worktree and branch if ABC-1472 is re-scoped",
	}
	if got := snap.Threads[0].Next; !reflect.DeepEqual(got, wantNext) {
		t.Errorf("t-0001 Next = %q, want %q", got, wantNext)
	}
	if r := snap.Threads[0].Report; !strings.Contains(r, "## Next") {
		t.Errorf("t-0001 Report = %q, want threads/t-0001.md", r)
	}
	if want := filepath.Join("testdata", "admin-rebuild"); p.Dir != want {
		t.Errorf("Project.Dir = %q, want %q", p.Dir, want)
	}
	if !snap.ThreadsAsOf.IsZero() {
		t.Errorf("ThreadsAsOf = %v from the live list, want zero", snap.ThreadsAsOf)
	}

	// Every inbox item is in inbox/done/: handled, so not shown.
	if len(snap.Inbox) != 0 {
		t.Errorf("Inbox = %+v, want empty", snap.Inbox)
	}
}

func TestThreadListArgs(t *testing.T) {
	var gotName string
	var gotArgs []string
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		gotName, gotArgs = name, args
		return []byte("[]"), nil
	}
	Reader{Root: "testdata", Slug: "admin-rebuild", Bin: "/opt/hp", Run: run}.Read(context.Background())
	if gotName != "/opt/hp" {
		t.Errorf("bin = %q", gotName)
	}
	if want := []string{"--root", "testdata", "thread", "list", "admin-rebuild", "--json"}; !reflect.DeepEqual(gotArgs, want) {
		t.Errorf("args = %q, want %q", gotArgs, want)
	}

}

func TestDefaultBin(t *testing.T) {
	var got string
	run := func(_ context.Context, name string, _ ...string) ([]byte, error) {
		got = name
		return []byte("[]"), nil
	}
	reader("admin-rebuild", run).Read(context.Background())
	if got != DefaultBin {
		t.Errorf("bin = %q, want %q", got, DefaultBin)
	}
}

// The TOML fallback gives the same threads as the JSON, apart from the live
// group, which it takes from last_group.
func TestTOMLFallbackMatchesThreadList(t *testing.T) {
	fromJSON := reader("admin-rebuild", jsonRun(t)).Read(context.Background())
	fromTOML := reader("admin-rebuild", failRun).Read(context.Background())

	if len(fromTOML.Missing) != 1 || !strings.Contains(fromTOML.Missing[0], "thread list: herdr-projects: no session;") {
		t.Errorf("Missing = %q, want one thread list note with the first stderr line", fromTOML.Missing)
	}
	if fromTOML.ThreadsAsOf.IsZero() {
		t.Errorf("ThreadsAsOf is zero for threads read from TOML")
	}
	if !reflect.DeepEqual(fromTOML.Threads, fromJSON.Threads) {
		t.Errorf("TOML threads differ from JSON threads:\n toml %+v\n json %+v", fromTOML.Threads, fromJSON.Threads)
	}
}

func TestMissingBinaryFallsBack(t *testing.T) {
	r := Reader{Root: "testdata", Slug: "admin-rebuild", Bin: filepath.Join(t.TempDir(), "no-such-herdr-projects")}
	snap := r.Read(context.Background())
	if got := ids(snap.Threads); len(got) != 4 {
		t.Errorf("threads = %q, want 4 from TOML", got)
	}
	if len(snap.Missing) != 1 || !strings.HasPrefix(snap.Missing[0], "thread list:") {
		t.Errorf("Missing = %q", snap.Missing)
	}
}

func TestBadJSONFallsBack(t *testing.T) {
	run := func(context.Context, string, ...string) ([]byte, error) { return []byte("not json"), nil }
	snap := reader("admin-rebuild", run).Read(context.Background())
	if len(snap.Threads) != 4 || len(snap.Missing) != 1 {
		t.Errorf("threads %d, Missing %q", len(snap.Threads), snap.Missing)
	}
}

func TestReadBusy(t *testing.T) {
	snap := reader("busy", failRun).Read(context.Background())

	if want := []string{"/src/app", "/src/api"}; !reflect.DeepEqual(snap.Project.Repos, want) {
		t.Errorf("Repos = %q", snap.Project.Repos)
	}

	// t-0003.toml is broken: noted and skipped, the rest still read.
	if got, want := ids(snap.Threads), []string{"t-0001", "t-0002", "t-0004"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("thread ids = %q, want %q", got, want)
	}
	pr := snap.Threads[0]
	if pr.Status != deck.StatusReview || pr.StateLine != "Done" {
		t.Errorf("t-0001 status %v line %q", pr.Status, pr.StateLine)
	}
	wantPR := &deck.PullRequest{
		URL:           "https://github.com/acme/app/pull/2320",
		Number:        2320,
		State:         "OPEN",
		Review:        "CHANGES_REQUESTED",
		FailingChecks: []string{"lint", "test"},
		Comments:      3,
		Commenters:    []string{"alice", "bob"},
		CheckedAt:     time.Date(2026, 10, 2, 15, 2, 5, 795169000, time.UTC),
	}
	if !reflect.DeepEqual(pr.PR, wantPR) {
		t.Errorf("t-0001 PR = %+v, want %+v", pr.PR, wantPR)
	}
	wantLinks := []deck.Link{{Kind: deck.LinkGitHub, Label: "PR #2320", URL: "https://github.com/acme/app/pull/2320"}}
	if !reflect.DeepEqual(pr.Links, wantLinks) {
		t.Errorf("t-0001 Links = %+v", pr.Links)
	}
	if want := []string{"Merge the PR", "Remove the worktree and branch"}; !reflect.DeepEqual(pr.Next, want) {
		t.Errorf("t-0001 Next = %q", pr.Next)
	}

	done := snap.Threads[1]
	if done.Status != deck.StatusDone || done.StateLine != "resolved · merged" || done.Worktree != "/src/worktrees/t-0002" {
		t.Errorf("t-0002 = %+v", done)
	}
	failed := snap.Threads[2]
	if failed.Status != deck.StatusUnknown || failed.StateLine != "failed: worktree add failed" {
		t.Errorf("t-0004 = %+v", failed)
	}

	// Newest first; stray.md has no front matter but is still listed;
	// notes.txt and done/ are not.
	var inbox []string
	for _, it := range snap.Inbox {
		inbox = append(inbox, it.ID+"|"+it.Thread)
	}
	want := []string{"20261002T150000Z-note-7|", "20261002T141117Z-thread-state-t-0001-4|t-0001", "stray|"}
	if !reflect.DeepEqual(inbox, want) {
		t.Errorf("inbox = %q, want %q", inbox, want)
	}
	it := snap.Inbox[1]
	if it.Kind != "thread-state" || it.Subject != "t-0001" || it.Summary != `t-0001 "Review me" is now Ready for review` ||
		!it.Created.Equal(time.Date(2026, 10, 2, 14, 11, 17, 0, time.UTC)) {
		t.Errorf("inbox item = %+v", it)
	}

	for _, sub := range []string{"thread list:", "threads/t-0003.toml:", "inbox/stray.md: no +++ front matter"} {
		if !containsPrefix(snap.Missing, sub) {
			t.Errorf("Missing %q has no %q", snap.Missing, sub)
		}
	}
	if len(snap.Missing) != 3 {
		t.Errorf("Missing = %q, want 3 notes", snap.Missing)
	}
}

func containsPrefix(list []string, prefix string) bool {
	for _, s := range list {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func TestMissingProjectFolder(t *testing.T) {
	snap := Reader{Root: t.TempDir(), Slug: "gone", Run: failRun}.Read(context.Background())
	if snap.Project.Slug != "gone" || snap.Project.Title() != "gone" {
		t.Errorf("Project = %+v", snap.Project)
	}
	if len(snap.Missing) != 1 || !strings.Contains(snap.Missing[0], "not found") {
		t.Errorf("Missing = %q", snap.Missing)
	}
}

func TestEmptyProjectFolder(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	snap := Reader{Root: root, Slug: "empty", Run: failRun}.Read(context.Background())
	if len(snap.Threads) != 0 || len(snap.Inbox) != 0 || snap.Project.Slug != "empty" {
		t.Errorf("snap = %+v", snap)
	}
	for _, sub := range []string{"PROJECT.md: no such file", ".state/ticker.json: no such file", "thread list:"} {
		if !containsPrefix(snap.Missing, sub) {
			t.Errorf("Missing %q has no %q", snap.Missing, sub)
		}
	}
}

func TestBadProjectAndTicker(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "odd")
	write := func(rel, text string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("PROJECT.md", "+++\nname = \"Odd\"\n")
	write(".state/ticker.json", "{")
	snap := Reader{Root: root, Slug: "odd", Run: jsonRun(t)}.Read(context.Background())
	if snap.Project.Name != "" {
		t.Errorf("Name = %q, want empty from unclosed front matter", snap.Project.Name)
	}
	if !containsPrefix(snap.Missing, "PROJECT.md: unclosed") || !containsPrefix(snap.Missing, ".state/ticker.json:") {
		t.Errorf("Missing = %q", snap.Missing)
	}
	if len(snap.Threads) != 4 {
		t.Errorf("threads = %d, want 4 from thread list", len(snap.Threads))
	}
}

// Reading must not change anything in the project folder.
func TestReadWritesNothing(t *testing.T) {
	before := tree(t, "testdata")
	reader("admin-rebuild", jsonRun(t)).Read(context.Background())
	reader("admin-rebuild", failRun).Read(context.Background())
	reader("busy", failRun).Read(context.Background())
	if after := tree(t, "testdata"); !reflect.DeepEqual(before, after) {
		t.Errorf("testdata changed:\nbefore %v\nafter  %v", before, after)
	}
}

func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out[p] = fmt.Sprint(info.ModTime(), info.Mode(), info.Size())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestNextLines(t *testing.T) {
	// The case from herdr-projects src/thread.rs.
	report := "PR: https://github.com/o/r/pull/1\n## Report\n- not this\n## Next\n- Merge the PR\n* Fix CI\n3. Confirm assumption X\n\n## Remember\n- nor this\n"
	if got, want := nextLines(report), []string{"Merge the PR", "Fix CI", "Confirm assumption X"}; !reflect.DeepEqual(got, want) {
		t.Errorf("nextLines = %q, want %q", got, want)
	}
	if got := nextLines("## Report\nnothing\n"); len(got) != 0 {
		t.Errorf("no Next section: %q", got)
	}
	if got := nextLines("## Next\n"); len(got) != 0 {
		t.Errorf("empty Next section: %q", got)
	}
}

func TestStatus(t *testing.T) {
	cases := []struct {
		status, group string
		want          deck.ThreadStatus
	}{
		{"open", "waiting-on-you", deck.StatusNeedsYou},
		{"open", "working", deck.StatusWorking},
		{"starting", "working", deck.StatusWorking},
		{"open", "ready-for-review", deck.StatusReview},
		{"open", "landing", deck.StatusReview},
		{"open", "idle", deck.StatusUnknown},
		{"open", "", deck.StatusUnknown},
		{"open", "resolved", deck.StatusDone},
		{"resolved", "waiting-on-you", deck.StatusDone},
	}
	for _, c := range cases {
		if got := status(c.status, c.group); got != c.want {
			t.Errorf("status(%q, %q) = %v, want %v", c.status, c.group, got, c.want)
		}
	}
}

func TestPRNumber(t *testing.T) {
	for url, want := range map[string]int{
		"https://github.com/acme/app/pull/2320":        2320,
		"https://github.com/acme/app/pull/7/files":     7,
		"https://gitlab.com/acme/app/-/merge_requests": 0,
	} {
		if got := prNumber(url); got != want {
			t.Errorf("prNumber(%q) = %d, want %d", url, got, want)
		}
	}
}

func TestExitErrorWithoutStderr(t *testing.T) {
	run := func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("boom") }
	snap := reader("admin-rebuild", run).Read(context.Background())
	if !containsPrefix(snap.Missing, "thread list: boom;") {
		t.Errorf("Missing = %q", snap.Missing)
	}
}

package diff

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

func TestParseLog(t *testing.T) {
	out := "\x1eaaaa\x1fa1\x1fp1 p2\x1f1759500000\x1fAda\x1fMerge main into thread\n" +
		"\x1ebbbb\x1fb2\x1fp1\x1f1759400000\x1fAda\x1fAdd the \x1ftable\n\n" +
		"3\t1\tsrc/a.go\n-\t-\tlogo.png\n10\t0\tsrc/{old => new}.go\n" +
		"\x1ecccc\x1fc3\x1f\x1f1759300000\x1fBo\x1fFirst\n"
	got := ParseLog([]byte(out))
	want := []deck.Commit{
		{SHA: "aaaa", Short: "a1", Author: "Ada", Subject: "Merge main into thread", Merge: true, Time: time.Unix(1759500000, 0)},
		{SHA: "bbbb", Short: "b2", Author: "Ada", Subject: "Add the ", Added: 13, Deleted: 1, Time: time.Unix(1759400000, 0)},
		{SHA: "cccc", Short: "c3", Author: "Bo", Subject: "First", Time: time.Unix(1759300000, 0)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseLog =\n%+v\nwant\n%+v", got, want)
	}
	if got := ParseLog(nil); got != nil {
		t.Errorf("empty log = %+v", got)
	}
}

func TestCountStatus(t *testing.T) {
	out := " M app.go\x00R  new.txt\x00old.txt\x00?? notes/todo.md\x00A  staged.go\x00"
	if n := CountStatus([]byte(out)); n != 4 {
		t.Errorf("CountStatus = %d, want 4", n)
	}
	if n := CountStatus(nil); n != 0 {
		t.Errorf("clean = %d", n)
	}
}

func TestParseShow(t *testing.T) {
	out := "\x1eThe body.\n\nSecond paragraph.\n\x1e\n" +
		"diff --git a/app.go b/app.go\nindex 1..2 100644\n--- a/app.go\n+++ b/app.go\n" +
		"@@ -1,2 +1,3 @@\n package app\n+func B() {}\n-func A() {}\n" +
		"diff --git a/old.txt b/new.txt\nsimilarity index 100%\nrename from old.txt\nrename to new.txt\n" +
		"diff --git a/logo.png b/logo.png\nnew file mode 100644\nBinary files /dev/null and b/logo.png differ\n"
	cp := ParseShow([]byte(out), 100)
	if cp.Body != "The body.\n\nSecond paragraph." {
		t.Errorf("body %q", cp.Body)
	}
	want := []deck.PatchLine{
		{Kind: deck.LineFile, Text: "app.go"},
		{Kind: deck.LineHunk, Text: "@@ -1,2 +1,3 @@"},
		{Kind: deck.LineContext, Text: "package app"},
		{Kind: deck.LineAdded, Text: "func B() {}"},
		{Kind: deck.LineDeleted, Text: "func A() {}"},
		{Kind: deck.LineFile, Text: "old.txt → new.txt"},
		{Kind: deck.LineNote, Text: "renamed, content unchanged"},
		{Kind: deck.LineFile, Text: "logo.png"},
		{Kind: deck.LineNote, Text: "binary file"},
	}
	if !reflect.DeepEqual(cp.Patch.Lines, want) {
		t.Errorf("lines =\n%+v\nwant\n%+v", cp.Patch.Lines, want)
	}
	if cp := ParseShow([]byte(out), 3); len(cp.Patch.Lines) != 3 || cp.Patch.More == 0 {
		t.Errorf("capped: %d lines, %d more", len(cp.Patch.Lines), cp.Patch.More)
	}
	if cp := ParseShow([]byte("\x1e\x1e\n"), 100); cp.Patch.Note != "no changes" || cp.Body != "" {
		t.Errorf("empty commit: %+v", cp)
	}
}

// gitIn runs git in dir with a fixed identity and no user config.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// clean drops the repo's uncommitted changes.
func clean(t *testing.T, dir string) {
	t.Helper()
	gitIn(t, dir, "reset", "-q", "--hard")
	gitIn(t, dir, "clean", "-q", "-fdx")
}

func TestReadCommitsTempRepo(t *testing.T) {
	dir := repo(t)
	r := &Reader{}
	ctx := context.Background()
	th := deck.Thread{Worktree: dir, Base: "main"}
	cs := r.ReadCommits(ctx, th)
	if cs.Note != "" || len(cs.List) != 1 || cs.List[0].Subject != "work" || cs.List[0].Merge {
		t.Fatalf("commits %+v", cs)
	}
	if c := cs.List[0]; c.Added != 2 || c.Deleted != 1 || len(c.Short) < 7 {
		t.Errorf("work commit %+v", c)
	}
	// README.md, staged.go, .gitignore, logo.png, notes/todo.md.
	if cs.Uncommitted != 5 || cs.Upstream {
		t.Errorf("uncommitted %d, upstream %v", cs.Uncommitted, cs.Upstream)
	}

	// A merge of main into the thread lists dim, without counts.
	clean(t, dir)
	gitIn(t, dir, "merge", "-q", "--no-edit", "--no-ff", "main")
	r = &Reader{}
	cs = r.ReadCommits(ctx, th)
	if len(cs.List) != 2 || !cs.List[0].Merge || cs.List[0].Added != 0 || cs.Uncommitted != 0 {
		t.Fatalf("after the merge: %+v", cs)
	}
	cp := r.ReadCommitPatch(ctx, th, cs.List[0].SHA)
	if len(cp.Patch.Lines) == 0 || cp.Patch.Lines[0] != (deck.PatchLine{Kind: deck.LineFile, Text: "later.txt"}) {
		t.Errorf("the merge's patch against its first parent: %+v", cp.Patch)
	}
	cp = r.ReadCommitPatch(ctx, th, cs.List[1].SHA)
	if cp.Patch.Note != "" || len(cp.Patch.Lines) < 4 {
		t.Errorf("work's patch: %+v", cp.Patch)
	}

	// An empty branch: main itself.
	if cs := r.ReadCommits(ctx, deck.Thread{Worktree: dir, Base: "HEAD"}); cs.Note != "" || len(cs.List) != 0 {
		t.Errorf("empty branch: %+v", cs)
	}
}

func TestReadCommitsNotes(t *testing.T) {
	r := &Reader{}
	ctx := context.Background()
	if cs := r.ReadCommits(ctx, deck.Thread{}); cs.Note != "no worktree" {
		t.Errorf("no worktree: %+v", cs)
	}
	if cs := r.ReadCommits(ctx, deck.Thread{Worktree: filepath.Join(t.TempDir(), "nope")}); cs.Note != "the worktree is gone" || cs.Base != DefaultBase {
		t.Errorf("gone: %+v", cs)
	}
}

func TestReadCommitsPushed(t *testing.T) {
	dir := repo(t)
	clean(t, dir)
	remote := t.TempDir()
	gitIn(t, remote, "init", "-q", "--bare")
	gitIn(t, dir, "remote", "add", "origin", remote)
	gitIn(t, dir, "push", "-q", "-u", "origin", "thread")
	gitIn(t, dir, "commit", "-q", "--allow-empty", "-m", "local only")
	cs := (&Reader{}).ReadCommits(context.Background(), deck.Thread{Worktree: dir, Base: "main"})
	if !cs.Upstream || len(cs.List) != 2 || cs.List[0].Pushed || !cs.List[1].Pushed {
		t.Errorf("pushed: %+v", cs)
	}
}

// The list is read again only when HEAD moves; the uncommitted count is
// read every time.
func TestReadCommitsCachesByHead(t *testing.T) {
	clock := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	head := "h1"
	var calls []string
	r := &Reader{
		Now: func() time.Time { return clock },
		Git: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			calls = append(calls, args[0])
			switch args[0] {
			case "rev-parse":
				if args[1] == "HEAD" {
					return []byte(head + "\n"), nil
				}
				return nil, os.ErrNotExist // no upstream
			case "status":
				return []byte("?? a\x00"), nil
			case "merge-base":
				return []byte("mb\n"), nil
			case "log":
				return []byte("\x1e" + head + "\x1fh\x1fp\x1f0\x1fA\x1fs\n"), nil
			}
			return nil, nil
		},
	}
	th := deck.Thread{Worktree: t.TempDir()}
	if cs := r.ReadCommits(context.Background(), th); len(cs.List) != 1 || cs.Uncommitted != 1 || cs.Head != "h1" {
		t.Fatalf("first read %+v", cs)
	}
	count := func(name string) int {
		n := 0
		for _, c := range calls {
			if c == name {
				n++
			}
		}
		return n
	}
	r.ReadCommits(context.Background(), th)
	if count("status") != 1 {
		t.Errorf("a read within the TTL ran git: %v", calls)
	}
	clock = clock.Add(cacheTTL)
	r.ReadCommits(context.Background(), th)
	if count("status") != 2 || count("log") != 1 {
		t.Errorf("same HEAD after the TTL: %v", calls)
	}
	head = "h2"
	clock = clock.Add(cacheTTL)
	if cs := r.ReadCommits(context.Background(), th); count("log") != 2 || cs.List[0].SHA != "h2" {
		t.Errorf("a moved HEAD did not read the log again: %v %+v", calls, cs)
	}
}

// A moved base reads the list again; a failed read is never kept.
func TestReadCommitsBaseAndFailures(t *testing.T) {
	clock := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	baseSHA, mbFails := "b1", true
	logs := 0
	r := &Reader{
		Now: func() time.Time { return clock },
		Git: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			switch args[0] {
			case "rev-parse":
				switch {
				case args[1] == "HEAD":
					return []byte("h1\n"), nil
				case strings.HasSuffix(args[len(args)-1], "^{commit}"):
					return []byte(baseSHA + "\n"), nil
				}
				return nil, os.ErrNotExist
			case "merge-base":
				if mbFails {
					return nil, errors.New("timeout")
				}
				return []byte("mb\n"), nil
			case "log":
				logs++
				return []byte("\x1eh1\x1fh\x1fp\x1f0\x1fA\x1fs\n"), nil
			}
			return nil, nil
		},
	}
	th := deck.Thread{Worktree: t.TempDir()}
	read := func() deck.Commits {
		clock = clock.Add(cacheTTL)
		return r.ReadCommits(context.Background(), th)
	}
	if cs := read(); !strings.HasPrefix(cs.Note, "cannot compare") {
		t.Fatalf("failed merge-base: %+v", cs)
	}
	mbFails = false
	if cs := read(); cs.Note != "" || len(cs.List) != 1 {
		t.Fatalf("a failure was cached: %+v", cs)
	}
	read()
	if logs != 1 {
		t.Errorf("same HEAD and base read the log %d times", logs)
	}
	baseSHA = "b2"
	read()
	if logs != 2 {
		t.Errorf("a moved base did not read the log again: %d", logs)
	}
}

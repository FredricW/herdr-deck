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

func TestParseNumstat(t *testing.T) {
	out := "3\t1\tsrc/app.go\x00" +
		"-\t-\tlogo.png\x00" +
		"0\t0\t\x00old name.txt\x00new name.txt\x00" +
		"2\t5\tdocs/ü tab\t.md\x00"
	got := ParseNumstat([]byte(out))
	want := []deck.DiffFile{
		{Path: "src/app.go", Added: 3, Deleted: 1},
		{Path: "logo.png", Binary: true},
		{Path: "new name.txt", OldPath: "old name.txt"},
		{Path: "docs/ü tab\t.md", Added: 2, Deleted: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseNumstat =\n%+v\nwant\n%+v", got, want)
	}
	if got := ParseNumstat(nil); got != nil {
		t.Errorf("empty output = %+v", got)
	}
}

// repo makes a git repository with a main branch and a thread branch
// checked out in it, and returns its folder.
func repo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	write := func(name, content string) {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", "main")
	write("app.go", "package app\n\nfunc A() {}\n")
	write("README.md", "# Webshop\n\nA shop.\n")
	write("old.txt", "one\ntwo\nthree\nfour\nfive\nsix\n")
	write("gone.txt", "bye\n")
	git("add", ".")
	git("commit", "-q", "-m", "start")
	git("checkout", "-q", "-b", "thread")
	// Committed on the thread's branch.
	write("app.go", "package app\n\nfunc A() {}\n\nfunc B() {}\n")
	git("mv", "old.txt", "renamed.txt")
	git("rm", "-q", "gone.txt")
	git("add", "app.go")
	git("commit", "-q", "-m", "work")
	// main moves on after the fork: not the thread's change.
	git("checkout", "-q", "main")
	write("later.txt", "main only\n")
	git("add", ".")
	git("commit", "-q", "-m", "later")
	git("checkout", "-q", "thread")
	// Uncommitted: an edit, a staged new file, untracked text and binary
	// files, and an ignored one.
	write("README.md", "# Webshop\n")
	write("staged.go", "package app\n")
	git("add", "staged.go")
	write("notes/todo.md", "a\nb\nno newline")
	write("logo.png", "\x89PNG\x00\x01\x02")
	write(".gitignore", "*.log\n")
	write("debug.log", "ignored\n")
	return dir
}

func TestReadTempRepo(t *testing.T) {
	dir := repo(t)
	r := &Reader{}
	d := r.Read(context.Background(), deck.Thread{Worktree: dir, Base: "main"})
	if d.Note != "" {
		t.Fatalf("note %q", d.Note)
	}
	if d.Base != "main" || len(d.MergeBase) != 40 {
		t.Errorf("base %q, merge base %q", d.Base, d.MergeBase)
	}
	want := []deck.DiffFile{
		{Path: ".gitignore", Added: 1, Untracked: true},
		{Path: "README.md", Deleted: 2},
		{Path: "app.go", Added: 2},
		{Path: "gone.txt", Deleted: 1},
		{Path: "logo.png", Binary: true, Untracked: true},
		{Path: "notes/todo.md", Added: 3, Untracked: true},
		{Path: "renamed.txt", OldPath: "old.txt"},
		{Path: "staged.go", Added: 1},
	}
	if !reflect.DeepEqual(d.Files, want) {
		t.Errorf("files =\n%+v\nwant\n%+v", d.Files, want)
	}
	if a, del := d.Totals(); a != 7 || del != 3 {
		t.Errorf("totals +%d -%d", a, del)
	}
}

func TestReadNotes(t *testing.T) {
	dir := repo(t)
	r := &Reader{}
	ctx := context.Background()
	if d := r.Read(ctx, deck.Thread{}); d.Note != "no worktree" {
		t.Errorf("no worktree: %+v", d)
	}
	if d := r.Read(ctx, deck.Thread{Worktree: filepath.Join(dir, "nope")}); d.Note != "the worktree is gone" || d.Base != DefaultBase {
		t.Errorf("gone: %+v", d)
	}
	d := r.Read(ctx, deck.Thread{Worktree: dir, Base: "origin/main"})
	if !strings.HasPrefix(d.Note, "cannot compare with origin/main: ") || len(d.Files) != 0 {
		t.Errorf("unknown base: %+v", d)
	}
}

func TestReadCachesBriefly(t *testing.T) {
	clock := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	var calls []string
	r := &Reader{
		Now: func() time.Time { return clock },
		Git: func(_ context.Context, dir string, args ...string) ([]byte, error) {
			calls = append(calls, args[0])
			switch args[0] {
			case "merge-base":
				return []byte("abc123\n"), nil
			case "diff":
				if args[len(args)-2] != "abc123" {
					t.Errorf("diff against %q, want the merge base", args[len(args)-2])
				}
				return []byte("1\t0\ta.go\x00"), nil
			}
			return nil, nil
		},
	}
	th := deck.Thread{Worktree: t.TempDir(), Base: "origin/main"}
	d := r.Read(context.Background(), th)
	if len(d.Files) != 1 || d.MergeBase != "abc123" {
		t.Fatalf("diff %+v", d)
	}
	r.Read(context.Background(), th)
	if len(calls) != 3 {
		t.Errorf("a second read within the TTL ran git: %v", calls)
	}
	clock = clock.Add(cacheTTL)
	r.Read(context.Background(), th)
	if len(calls) != 6 {
		t.Errorf("a read after the TTL did not run git: %v", calls)
	}
}

func TestReadGitFails(t *testing.T) {
	r := &Reader{Git: func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("not a git repository")
	}}
	d := r.Read(context.Background(), deck.Thread{Worktree: t.TempDir()})
	if d.Note != "cannot compare with origin/HEAD: not a git repository" {
		t.Errorf("note %q", d.Note)
	}
}

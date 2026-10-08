package diff

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

func TestParsePatch(t *testing.T) {
	out := "diff --git a/app.go b/app.go\n" +
		"index 1111111..2222222 100644\n" +
		"--- a/app.go\n" +
		"+++ b/app.go\n" +
		"@@ -1,3 +1,4 @@ package app\n" +
		" package app\n" +
		"-func A() {}\r\n" +
		"+func A() int { return 1 }\n" +
		"+\n" +
		"\n" +
		"@@ -9 +10 @@ func C() {\n" +
		"-}\n" +
		"\\ No newline at end of file\n" +
		"+}\n"
	got := ParsePatch([]byte(out), 100)
	want := deck.Patch{Lines: []deck.PatchLine{
		{Kind: deck.LineHunk, Text: "@@ -1,3 +1,4 @@ package app"},
		{Kind: deck.LineContext, Text: "package app"},
		{Kind: deck.LineDeleted, Text: "func A() {}"},
		{Kind: deck.LineAdded, Text: "func A() int { return 1 }"},
		{Kind: deck.LineAdded},
		{Kind: deck.LineContext},
		{Kind: deck.LineHunk, Text: "@@ -9 +10 @@ func C() {"},
		{Kind: deck.LineDeleted, Text: "}"},
		{Kind: deck.LineNote, Text: "No newline at end of file"},
		{Kind: deck.LineAdded, Text: "}"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParsePatch =\n%+v\nwant\n%+v", got, want)
	}

	// The cap keeps the first lines and counts the rest.
	got = ParsePatch([]byte(out), 4)
	if len(got.Lines) != 4 || got.More != 6 {
		t.Errorf("capped: %d lines, %d more", len(got.Lines), got.More)
	}

	binary := "diff --git a/logo.png b/logo.png\nindex 1..2 100644\nBinary files a/logo.png and b/logo.png differ\n"
	if got := ParsePatch([]byte(binary), 100); !got.Binary || len(got.Lines) != 0 {
		t.Errorf("binary: %+v", got)
	}
	rename := "diff --git a/old.txt b/new.txt\nsimilarity index 100%\nrename from old.txt\nrename to new.txt\n"
	if got := ParsePatch([]byte(rename), 100); got.Note != "renamed, content unchanged" {
		t.Errorf("rename: %+v", got)
	}
	mode := "diff --git a/run.sh b/run.sh\nold mode 100644\nnew mode 100755\n"
	if got := ParsePatch([]byte(mode), 100); got.Note != "no changes in content" {
		t.Errorf("mode change: %+v", got)
	}
	if got := ParsePatch(nil, 100); got.Note != "no changes" {
		t.Errorf("empty: %+v", got)
	}
	// A rename git cannot pair prints two sections; both show.
	split := "diff --git a/a.txt b/a.txt\ndeleted file mode 100644\n--- a/a.txt\n+++ /dev/null\n@@ -1 +0,0 @@\n-old\n" +
		"diff --git a/b.txt b/b.txt\nnew file mode 100644\n--- /dev/null\n+++ b/b.txt\n@@ -0,0 +1 @@\n+new\n"
	if got := ParsePatch([]byte(split), 100); len(got.Lines) != 4 || got.Lines[3].Text != "new" {
		t.Errorf("two sections: %+v", got)
	}
}

func TestCutLine(t *testing.T) {
	long := strings.Repeat("é", maxLineBytes) // two bytes each
	got := cutLine(long)
	if len(got) > maxLineBytes || !strings.HasPrefix(long, got) || len(got) < maxLineBytes-1 {
		t.Errorf("cut to %d bytes", len(got))
	}
}

func TestUntrackedPatch(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	got := untrackedPatch(write("todo.md", "# Todo\n\n- one"))
	want := deck.Patch{Lines: []deck.PatchLine{
		{Kind: deck.LineHunk, Text: "@@ -0,0 +1,3 @@"},
		{Kind: deck.LineAdded, Text: "# Todo"},
		{Kind: deck.LineAdded},
		{Kind: deck.LineAdded, Text: "- one"},
		{Kind: deck.LineNote, Text: "No newline at end of file"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("untracked =\n%+v\nwant\n%+v", got, want)
	}
	if got := untrackedPatch(write("logo.png", "\x89PNG\x00\x01")); !got.Binary {
		t.Errorf("binary: %+v", got)
	}
	if got := untrackedPatch(write("empty.txt", "")); got.Note != "empty file" {
		t.Errorf("empty: %+v", got)
	}
	big := untrackedPatch(write("big.txt", strings.Repeat("x\n", MaxPatchLines+10)))
	if len(big.Lines) != MaxPatchLines || big.More != 11 || big.Cut {
		t.Errorf("big: %d lines, %d more, cut %v", len(big.Lines), big.More, big.Cut)
	}
	if got := untrackedPatch(filepath.Join(dir, "nope")); !strings.HasPrefix(got.Note, "cannot read the file") {
		t.Errorf("missing: %+v", got)
	}
}

func TestReadPatchTempRepo(t *testing.T) {
	dir := repo(t)
	r := &Reader{}
	ctx := context.Background()
	th := deck.Thread{Worktree: dir, Base: "main"}
	d := r.Read(ctx, th)
	byPath := map[string]deck.DiffFile{}
	for _, f := range d.Files {
		byPath[f.Path] = f
	}
	p := r.ReadPatch(ctx, th, d.MergeBase, byPath["app.go"])
	want := []deck.PatchLine{
		{Kind: deck.LineHunk, Text: "@@ -1,3 +1,5 @@"},
		{Kind: deck.LineContext, Text: "package app"},
		{Kind: deck.LineContext},
		{Kind: deck.LineContext, Text: "func A() {}"},
		{Kind: deck.LineAdded},
		{Kind: deck.LineAdded, Text: "func B() {}"},
	}
	if !reflect.DeepEqual(p.Lines, want) || p.Note != "" {
		t.Errorf("app.go =\n%+v\nwant\n%+v (note %q)", p.Lines, want, p.Note)
	}
	// Uncommitted edits count: README.md lost two lines in the worktree.
	if p := r.ReadPatch(ctx, th, d.MergeBase, byPath["README.md"]); len(p.Lines) != 4 || p.Lines[2].Kind != deck.LineDeleted {
		t.Errorf("README.md: %+v", p)
	}
	if p := r.ReadPatch(ctx, th, d.MergeBase, byPath["renamed.txt"]); p.Note != "renamed, content unchanged" {
		t.Errorf("rename: %+v", p)
	}
	if p := r.ReadPatch(ctx, th, d.MergeBase, byPath["gone.txt"]); len(p.Lines) != 2 || p.Lines[1] != (deck.PatchLine{Kind: deck.LineDeleted, Text: "bye"}) {
		t.Errorf("deleted: %+v", p)
	}
	if p := r.ReadPatch(ctx, th, d.MergeBase, byPath["logo.png"]); !p.Binary {
		t.Errorf("untracked binary: %+v", p)
	}
	if p := r.ReadPatch(ctx, th, d.MergeBase, byPath["notes/todo.md"]); len(p.Lines) != 5 || p.Lines[1].Kind != deck.LineAdded {
		t.Errorf("untracked: %+v", p)
	}
	if p := r.ReadPatch(ctx, deck.Thread{}, d.MergeBase, byPath["app.go"]); p.Note != "no worktree" {
		t.Errorf("no worktree: %+v", p)
	}
}

func TestReadPatchCachesByContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	r := &Reader{Git: func(context.Context, string, ...string) ([]byte, error) {
		calls++
		return []byte("@@ -1 +1 @@\n-zero\n+one\n"), nil
	}}
	th := deck.Thread{Worktree: dir}
	f := deck.DiffFile{Path: "a.go"}
	r.ReadPatch(context.Background(), th, "abc", f)
	r.ReadPatch(context.Background(), th, "abc", f)
	if calls != 1 {
		t.Errorf("the same content ran git %d times", calls)
	}
	if err := os.WriteFile(path, []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.ReadPatch(context.Background(), th, "abc", f)
	if calls != 2 {
		t.Errorf("changed content ran git %d times, want 2", calls)
	}
	r.ReadPatch(context.Background(), th, "def", f)
	if calls != 3 {
		t.Errorf("another merge-base ran git %d times, want 3", calls)
	}
}

func TestLimitWriter(t *testing.T) {
	w := &limitWriter{limit: 5}
	_, _ = w.Write([]byte("abc"))
	_, _ = w.Write([]byte("defg"))
	if w.buf.String() != "abcde" || !w.cut {
		t.Errorf("kept %q, cut %v", w.buf.String(), w.cut)
	}
}

// A FIFO never ends: neither the hash nor the read may open it.
func TestReadPatchSkipsFIFO(t *testing.T) {
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o644); err != nil {
		t.Skip("no FIFOs here:", err)
	}
	done := make(chan deck.Patch, 1)
	go func() {
		r := &Reader{}
		done <- r.ReadPatch(context.Background(), deck.Thread{Worktree: dir}, "", deck.DiffFile{Path: "pipe", Untracked: true})
	}()
	select {
	case p := <-done:
		if p.Note != "not a regular file" {
			t.Errorf("FIFO: %+v", p)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ReadPatch blocked on a FIFO")
	}
}

func TestReadFileAt(t *testing.T) {
	var got []string
	r := &Reader{Git: func(_ context.Context, dir string, args ...string) ([]byte, error) {
		got = append(got, dir+" "+strings.Join(args, " "))
		return []byte("package a\n"), nil
	}}
	b, err := r.ReadFileAt(context.Background(), deck.Thread{Worktree: "/wt"}, "abc123", "a/a.go")
	if err != nil || string(b) != "package a\n" || len(got) != 1 || got[0] != "/wt show abc123:a/a.go" {
		t.Errorf("%q %v %v", b, err, got)
	}
	if _, err := r.ReadFileAt(context.Background(), deck.Thread{}, "abc123", "a/a.go"); err == nil {
		t.Error("read without a worktree")
	}
}

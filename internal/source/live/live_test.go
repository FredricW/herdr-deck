package live

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/dev"
	"github.com/FredricW/herdr-deck/internal/source/projects"
)

// testdata/demo is hand-written: one task list naming a thread with a PR and
// a report, a backlog, and a thread no task names whose brief has a Notion
// link. The thread list command always fails, so threads come from TOML.

var readAt = time.Date(2026, 10, 2, 14, 41, 0, 0, time.UTC)

func noBinary(context.Context, string, ...string) ([]byte, error) {
	return nil, errors.New("not installed")
}

func source(root, slug string) Source {
	return Source{
		Projects: projects.Reader{Root: root, Slug: slug, Run: noBinary},
		Now:      func() time.Time { return readAt },
	}
}

func labels(links []deck.Link) []string {
	var out []string
	for _, l := range links {
		out = append(out, l.Kind.String()+" "+l.Label)
	}
	return out
}

func TestReadCombinesSources(t *testing.T) {
	snap := source("testdata", "demo").Read(context.Background())

	if snap.Project.Name != "Demo" || snap.Project.Dir != filepath.Join("testdata", "demo") {
		t.Errorf("Project = %+v", snap.Project)
	}
	if !snap.ReadAt.Equal(readAt) {
		t.Errorf("ReadAt = %v", snap.ReadAt)
	}
	if len(snap.TaskLists) != 2 || snap.TaskLists[0].Name != "In progress" || snap.TaskLists[1].Name != "Backlog" {
		t.Fatalf("TaskLists = %+v", snap.TaskLists)
	}
	task := snap.TaskLists[0].Tasks[0]
	if task.Title != "Users page" || strings.Join(task.Threads, ",") != "t-0001" {
		t.Errorf("task = %+v", task)
	}
	if len(snap.Threads) != 2 {
		t.Fatalf("Threads = %+v", snap.Threads)
	}
	t1, t2 := snap.Threads[0], snap.Threads[1]
	want := "GitHub PR #7|Figma Users · node 1-2|Linear ABC-12"
	if got := strings.Join(labels(t1.Links), "|"); got != want {
		t.Errorf("t-0001 links = %q, want %q (the PR once, then the report's links)", got, want)
	}
	if !strings.Contains(t1.Report, "Built the overview") {
		t.Errorf("t-0001 Report = %q", t1.Report)
	}
	if got := strings.Join(labels(t2.Links), "|"); got != "Notion Spike notes" {
		t.Errorf("t-0002 links = %q", got)
	}
	// Only the missing thread list binary is reported; there is no ticker
	// or inbox in the fixture, and an absent inbox is not a missing source.
	for _, m := range snap.Missing {
		if strings.Contains(m, "TASKS.md") {
			t.Errorf("Missing has %q", m)
		}
	}
}

// Bare Linear IDs without a workspace have no URL, and Missing says how to
// set one. With a workspace they link and nothing is noted.
func TestLinearWorkspace(t *testing.T) {
	src := source("testdata", "demo")
	snap := src.Read(context.Background())
	task := snap.TaskLists[0].Tasks[0]
	if len(task.Links) == 0 || task.Links[0].Label != "ABC-12" || task.Links[0].URL != "" {
		t.Errorf("links = %+v, want ABC-12 without a URL", task.Links)
	}
	if !containsSub(snap.Missing, "Linear: no workspace set, so ABC-12") || !containsSub(snap.Missing, deck.EnvLinearWorkspace) {
		t.Errorf("Missing = %q, want a Linear workspace note", snap.Missing)
	}

	src.LinearWorkspace = "acme"
	snap = src.Read(context.Background())
	if got := snap.TaskLists[0].Tasks[0].Links[0].URL; got != "https://linear.app/acme/issue/ABC-12" {
		t.Errorf("URL = %q", got)
	}
	if containsSub(snap.Missing, "Linear:") {
		t.Errorf("Missing = %q, want no Linear note", snap.Missing)
	}
}

// A full Linear URL anywhere wins over the same bare ID: one link, and no
// Missing note for it.
func TestLinearURLWinsOverBareID(t *testing.T) {
	snap := deck.Snapshot{
		TaskLists: []deck.TaskList{{Tasks: []deck.Task{{Links: []deck.Link{{Kind: deck.LinkLinear, Label: "ABC-12", URL: "https://linear.app/acme/issue/ABC-12"}}}}}},
		Threads:   []deck.Thread{{Links: []deck.Link{{Kind: deck.LinkLinear, Label: "ABC-12"}}}},
	}
	if id := unlinkedLinearID(snap); id != "" {
		t.Errorf("unlinkedLinearID = %q, want none", id)
	}
	got := deck.AppendLink([]deck.Link{{Kind: deck.LinkLinear, Label: "ABC-12"}}, snap.TaskLists[0].Tasks[0].Links[0])
	if len(got) != 1 || got[0].URL == "" {
		t.Errorf("AppendLink = %+v, want the URL link once", got)
	}
}

func containsSub(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func TestMissingTasksFileIsNotMissingSource(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	snap := source(root, "empty").Read(context.Background())
	for _, m := range snap.Missing {
		if strings.Contains(m, "TASKS.md") {
			t.Errorf("Missing = %q, want no TASKS.md entry", snap.Missing)
		}
	}
	if len(snap.TaskLists) != 0 {
		t.Errorf("TaskLists = %+v", snap.TaskLists)
	}
}

func TestUnreadableTasksFileIsMissing(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p")
	// A folder where TASKS.md should be cannot be read as a file.
	if err := os.MkdirAll(filepath.Join(dir, "TASKS.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	snap := source(root, "p").Read(context.Background())
	found := false
	for _, m := range snap.Missing {
		found = found || strings.HasPrefix(m, "TASKS.md: ")
	}
	if !found {
		t.Errorf("Missing = %q, want a TASKS.md entry", snap.Missing)
	}
}

func TestWatchDebouncesAndFollowsNewFolders(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changed := make(chan struct{}, 10)
	if err := Watch(ctx, dir, 50*time.Millisecond, func() { changed <- struct{}{} }); err != nil {
		t.Fatal(err)
	}
	wait := func(what string) {
		t.Helper()
		select {
		case <-changed:
		case <-time.After(3 * time.Second):
			t.Fatalf("no change reported after %s", what)
		}
	}
	quiet := func(what string) {
		t.Helper()
		select {
		case <-changed:
			t.Fatalf("a second change was reported after %s", what)
		case <-time.After(200 * time.Millisecond):
		}
	}

	for i := range 5 {
		write(t, filepath.Join(dir, "TASKS.md"), strings.Repeat("x", i+1))
	}
	wait("a burst of writes")
	quiet("a burst of writes")

	// inbox/ does not exist at start; files in it are seen once it does.
	if err := os.Mkdir(filepath.Join(dir, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	wait("creating inbox/")
	write(t, filepath.Join(dir, "inbox", "item.md"), "+++\n+++\n")
	wait("a new inbox item")
}

func TestWatchMissingFolder(t *testing.T) {
	if err := Watch(context.Background(), filepath.Join(t.TempDir(), "gone"), time.Millisecond, func() {}); err == nil {
		t.Fatal("Watch on a missing folder: no error")
	}
}

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadDevServers(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "demo")
	if err := os.CopyFS(proj, os.DirFS(filepath.Join("testdata", "demo"))); err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(root, "worktrees", "t-0001-users")
	if err := os.MkdirAll(filepath.Join(wt, ".herdr-deck"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(wt, ".herdr-deck", "dev.json"), `{"state": {"ports": {"web": 4321}}, "links": [{"title": "Web", "url": "http://localhost:$PORT_web"}]}`)
	toml := filepath.Join(proj, "threads", "t-0001.toml")
	b, err := os.ReadFile(toml)
	if err != nil {
		t.Fatal(err)
	}
	write(t, toml, string(b)+"worktree_path = \""+wt+"\"\n")

	src := source(root, "demo")
	src.Dev = &dev.Reader{Prober: dev.Prober{Dial: func(context.Context, string) error { return nil }}}
	snap := src.Read(context.Background())
	t1 := snap.Threads[0]
	if len(t1.DevServers) != 1 || t1.DevServers[0] != (deck.DevServer{Name: "web", Port: 4321, Running: true}) {
		t.Errorf("servers %+v", t1.DevServers)
	}
	if got := labels(t1.Links); !strings.Contains(strings.Join(got, ","), "localhost Web") {
		t.Errorf("links %q, want the manifest's Web link", got)
	}
}

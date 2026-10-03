package projects

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// rosterFS is a made-up projects root: an active project with a thread
// waiting on the user and an inbox item, a paused one, an archived one, a
// project whose thread file does not parse, and folders that are not
// projects.
func rosterFS() fstest.MapFS {
	file := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }
	return fstest.MapFS{
		"admin-rebuild/PROJECT.md":          file("+++\nname = \"Admin rebuild\"\n+++\n"),
		"admin-rebuild/threads/t-0002.toml": file("id = \"t-0002\"\ntitle = \"Users page\"\nstatus = \"open\"\nlast_group = \"working\"\n"),

		"billing-export/PROJECT.md":              file("+++\nname = \"Billing export\"\ngoal = \"Export invoices as CSV.\"\n+++\n"),
		"billing-export/.state/project.json":     file(`{"status": "active"}`),
		"billing-export/.state/coordinator.json": file(`{"pane_id": "w4J:p1", "workspace_id": "w4J"}`),
		"billing-export/threads/t-0003.toml": file("id = \"t-0003\"\ntitle = \"CSV export job\"\nstatus = \"open\"\npane_id = \"w4K:p3\"\n" +
			"last_group = \"waiting-on-you\"\nlast_state_change = \"2026-10-02T14:39:00Z\"\n"),
		"billing-export/threads/t-0004.toml": file("id = \"t-0004\"\ntitle = \"Ledger totals fix\"\nstatus = \"open\"\nlast_group = \"landing\"\n"),
		"billing-export/threads/t-0001.toml": file("id = \"t-0001\"\ntitle = \"Old job\"\nstatus = \"resolved\"\n"),
		"billing-export/inbox/20261002T143300Z-thread-state-t-0003-3.md": file("+++\nid = \"20261002T143300Z-thread-state-t-0003-3\"\nkind = \"thread-state\"\n" +
			"subject = \"t-0003\"\nsummary = \"t-0003 is now Waiting on you\"\ncreated = \"2026-10-02T14:33:00Z\"\n+++\n"),
		"billing-export/inbox/done/20261002T100000Z-note-1.md": file("+++\nid = \"old\"\n+++\n"),

		"search-spike/PROJECT.md":          file("+++\nname = \"Search spike\"\n+++\n"),
		"search-spike/.state/project.json": file(`{"status": "paused"}`),

		"mobile-onboarding/PROJECT.md":          file("+++\nname = \"Mobile onboarding\"\n+++\n"),
		"mobile-onboarding/.state/project.json": file(`{"status": "archived"}`),

		"broken/PROJECT.md":          file("+++\nname = \"Broken\"\n+++\n"),
		"broken/threads/t-0001.toml": file("id = \"t-0001\nnot toml"),

		".hidden/PROJECT.md":  file("+++\nname = \"Hidden\"\n+++\n"),
		"no-project/notes.md": file("not a project"),
		"README.md":           file("a file, not a folder"),
	}
}

func TestRosterReadsEveryProject(t *testing.T) {
	r := &Roster{Root: "/root", FS: rosterFS()}
	list := r.Read(deck.Snapshot{Project: deck.Project{Slug: "other"}})
	var slugs []string
	by := map[string]deck.ProjectInfo{}
	for _, p := range list {
		slugs = append(slugs, p.Slug)
		by[p.Slug] = p
	}
	want := []string{"admin-rebuild", "billing-export", "broken", "mobile-onboarding", "search-spike"}
	if !slices.Equal(slugs, want) {
		t.Fatalf("slugs %v, want %v", slugs, want)
	}

	b := by["billing-export"]
	if b.Name != "Billing export" || b.Goal != "Export invoices as CSV." || b.Dir != "/root/billing-export" || b.Status != "active" || b.RecordedPane != "w4J:p1" || b.Problem != "" {
		t.Errorf("billing-export: %+v", b)
	}
	threads, inbox := b.Needs()
	if len(threads) != 1 || threads[0].ID != "t-0003" || threads[0].PaneID != "w4K:p3" ||
		!threads[0].Changed.Equal(time.Date(2026, 10, 2, 14, 39, 0, 0, time.UTC)) {
		t.Errorf("billing-export needs %+v", threads)
	}
	if len(inbox) != 1 || inbox[0].Thread != "t-0003" || inbox[0].Summary != "t-0003 is now Waiting on you" {
		t.Errorf("billing-export inbox %+v", inbox)
	}
	if len(b.Threads) != 3 || b.Threads[0].Status != deck.StatusDone || b.Threads[2].Status != deck.StatusReview {
		t.Errorf("billing-export threads %+v", b.Threads)
	}

	if s := by["search-spike"]; s.Status != "paused" || s.NeedsYou() {
		t.Errorf("search-spike: %+v", s)
	}
	if a := by["mobile-onboarding"]; !a.Archived() {
		t.Errorf("mobile-onboarding: %+v", a)
	}
	// Without .state/project.json a project is active.
	if a := by["admin-rebuild"]; a.Status != "active" || a.NeedsYou() {
		t.Errorf("admin-rebuild: %+v", a)
	}
	if p := by["broken"]; p.Problem == "" || len(p.Threads) != 0 {
		t.Errorf("broken: problem %q, threads %+v", p.Problem, p.Threads)
	}
}

func TestRosterTakesCurrentFromSnapshot(t *testing.T) {
	r := &Roster{Root: "/root", FS: rosterFS()}
	cur := deck.Snapshot{
		Project: deck.Project{Slug: "billing-export", Name: "Billing export", PaneID: "w9:p1"},
		Threads: []deck.Thread{{ID: "t-0003", Status: deck.StatusWorking}},
	}
	for _, p := range r.Read(cur) {
		if p.Slug != "billing-export" {
			continue
		}
		if p.PaneID != "w9:p1" || p.RecordedPane != "w4J:p1" || p.Status != "active" || p.NeedsYou() || len(p.Threads) != 1 {
			t.Errorf("current project %+v", p)
		}
	}
}

func TestRosterCachesAndCopies(t *testing.T) {
	fsys := rosterFS()
	clock := time.Date(2026, 10, 2, 14, 41, 0, 0, time.UTC)
	r := &Roster{Root: "/root", FS: fsys, Every: 20 * time.Second, Now: func() time.Time { return clock }}
	first := r.Read(deck.Snapshot{})
	first[1].Threads[0].Title = "changed by a caller"

	delete(fsys, "billing-export/inbox/20261002T143300Z-thread-state-t-0003-3.md")
	clock = clock.Add(10 * time.Second)
	again := r.Read(deck.Snapshot{})
	if again[1].Threads[0].Title == "changed by a caller" {
		t.Error("a caller's change reached the cache")
	}
	if len(again[1].Inbox) != 1 {
		t.Errorf("read again before Every: inbox %+v", again[1].Inbox)
	}
	clock = clock.Add(10 * time.Second)
	if later := r.Read(deck.Snapshot{}); len(later[1].Inbox) != 0 {
		t.Errorf("not read again after Every: inbox %+v", later[1].Inbox)
	}
}

func TestRosterWithoutRoot(t *testing.T) {
	r := NewRoster("/no/such/root")
	if list := r.Read(deck.Snapshot{}); list == nil || len(list) != 0 {
		t.Errorf("got %+v, want an empty, non-nil list", list)
	}
}

func TestOpen(t *testing.T) {
	var got []string
	r := Reader{Root: "/root", Slug: "docs-site", Bin: "hp", Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		got = append([]string{name}, args...)
		return []byte("opened `docs-site`\n"), nil
	}}
	if err := r.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"hp", "--root", "/root", "open", "docs-site", "--tab"}; !slices.Equal(got, want) {
		t.Errorf("ran %v, want %v", got, want)
	}

	r.Run = func(context.Context, string, ...string) ([]byte, error) {
		return nil, &exec.ExitError{Stderr: []byte("error: `docs-site` belongs to the herdr session at /x.sock\nmore\n")}
	}
	if err := r.Open(context.Background()); err == nil || err.Error() != "error: `docs-site` belongs to the herdr session at /x.sock" {
		t.Errorf("err %v", err)
	}
	r.Run = func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("not installed") }
	if err := r.Open(context.Background()); err == nil {
		t.Error("no error when herdr-projects cannot run")
	}
}

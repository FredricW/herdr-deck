package dev

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// upManifest has an up command with every variable and a literal $.
const upManifest = `{
  "state": { "file": ".dev/$DIRNAME/state.json", "ports": { "api": "api_port" } },
  "up": "scripts/dev-up --name $DIRNAME --branch ${BRANCH} --repo $REPO --dir $WORKTREE --cost $$5"
}`

// fakeStarter records the commands it is asked to run and starts nothing.
type fakeStarter struct {
	cmds  []Command
	alive map[int]bool
}

func (f *fakeStarter) start(c Command) (int, error) {
	f.cmds = append(f.cmds, c)
	pid := 4200 + len(f.cmds)
	f.alive[pid] = true
	return pid, nil
}

func upReader(t *testing.T, up ...int) (*Reader, *fakeStarter) {
	t.Helper()
	f := &fakeStarter{alive: map[int]bool{}}
	return &Reader{
		Prober: Prober{Dial: fakeDial(new(atomic.Int32), up...)},
		Logs:   filepath.Join(t.TempDir(), "state", "herdr-deck", "logs"),
		Start:  f.start,
		Alive:  func(pid int, _ time.Time) bool { return f.alive[pid] },
	}, f
}

func TestUpStartsOnceAndShowsInDrawer(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(main, ManifestPath), upManifest)
	r, f := upReader(t)
	th := deck.Thread{ID: "t-0003", Status: deck.StatusWorking, Worktree: wt, Repo: main, Branch: "abc-12-templates"}

	msg, err := r.Up(context.Background(), "admin-rebuild", th)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.cmds) != 1 {
		t.Fatalf("started %d commands, want 1", len(f.cmds))
	}
	c := f.cmds[0]
	want := "scripts/dev-up --name t-0003-templates --branch abc-12-templates --repo " + main + " --dir " + wt + " --cost $5"
	if c.Line != want || c.Dir != wt {
		t.Errorf("command %q in %q, want %q in %q", c.Line, c.Dir, want, wt)
	}
	if log := filepath.Join(r.Logs, "admin-rebuild-t-0003.log"); c.Log != log {
		t.Errorf("log %q, want %q", c.Log, log)
	}
	if !strings.Contains(msg, "started") || !strings.Contains(msg, "t-0003") {
		t.Errorf("status %q", msg)
	}

	// The drawer learns about it from the pid file, also after a restart.
	snap := deck.Snapshot{Project: deck.Project{Slug: "admin-rebuild"}, Threads: []deck.Thread{th}}
	r.Apply(context.Background(), &snap)
	if u := snap.Threads[0].DevUp; u == nil || !u.Alive || u.Log != c.Log {
		t.Fatalf("DevUp = %+v, want alive with log %q", u, c.Log)
	}

	// Pressed again while it runs: not started twice.
	msg, err = r.Up(context.Background(), "admin-rebuild", th)
	if err != nil || len(f.cmds) != 1 || !strings.Contains(msg, "still runs (pid 4201)") {
		t.Errorf("second Up: %q, %v, %d commands", msg, err, len(f.cmds))
	}

	// Once it has exited, u starts it again.
	f.alive[4201] = false
	snap = deck.Snapshot{Project: deck.Project{Slug: "admin-rebuild"}, Threads: []deck.Thread{th}}
	r.Apply(context.Background(), &snap)
	if u := snap.Threads[0].DevUp; u == nil || u.Alive {
		t.Errorf("DevUp = %+v after exit, want not alive", u)
	}
	if _, err := r.Up(context.Background(), "admin-rebuild", th); err != nil || len(f.cmds) != 2 {
		t.Errorf("Up after exit: %v, %d commands, want 2", err, len(f.cmds))
	}
}

func TestUpNotWhenPortsAnswer(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(main, ManifestPath), upManifest)
	r, f := upReader(t, 8011)
	th := deck.Thread{ID: "t-0003", Worktree: wt, Repo: main, DevServers: []deck.DevServer{{Name: "api", Port: 8011}}}
	msg, err := r.Up(context.Background(), "admin-rebuild", th)
	if err != nil || len(f.cmds) != 0 || !strings.Contains(msg, "already answer (:8011)") {
		t.Errorf("Up: %q, %v, %d commands", msg, err, len(f.cmds))
	}
}

func TestUpWithoutCommand(t *testing.T) {
	main, wt := repo(t) // the shared manifest has no up
	r, f := upReader(t)
	th := deck.Thread{ID: "t-0003", Worktree: wt, Repo: main}
	msg, err := r.Up(context.Background(), "admin-rebuild", th)
	if err != nil || len(f.cmds) != 0 || !strings.Contains(msg, `has no "up" command: add one`) {
		t.Errorf("no up: %q, %v", msg, err)
	}

	if err := os.Remove(filepath.Join(main, ManifestPath)); err != nil {
		t.Fatal(err)
	}
	msg, err = r.Up(context.Background(), "admin-rebuild", th)
	if err != nil || len(f.cmds) != 0 || !strings.Contains(msg, "no "+ManifestPath) {
		t.Errorf("no manifest: %q, %v", msg, err)
	}

	write(t, filepath.Join(main, ManifestPath), `{"state": `)
	if _, err := r.Up(context.Background(), "admin-rebuild", th); err == nil || len(f.cmds) != 0 {
		t.Errorf("broken manifest: err %v, %d commands", err, len(f.cmds))
	}
}

func TestSafeName(t *testing.T) {
	if got := safeName("a/b c.d_e-1"); got != "a_b_c.d_e-1" {
		t.Errorf("safeName = %q", got)
	}
}

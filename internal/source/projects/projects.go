// Package projects reads one herdr-projects project folder into a
// deck.Snapshot: its settings, threads, pull requests and unhandled inbox
// items. Tasks (TASKS.md) are read by internal/source/tasks.
//
// The reader never writes under the projects root, and the only command it
// runs is `herdr-projects --root <root> thread list <slug> --json`. Open,
// which starts a coordinator, runs only on the user's choice. Every
// source may be missing: what can be read is returned, and what could not is
// listed in Snapshot.Missing. Read never fails.
package projects

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// DefaultBin is the herdr-projects binary looked up in $PATH.
const DefaultBin = "herdr-projects"

// commandTimeout bounds one `thread list` call; the TOML fallback is used
// when it runs out.
const commandTimeout = 5 * time.Second

// RunFunc runs a command and returns its standard output.
type RunFunc func(ctx context.Context, name string, args ...string) ([]byte, error)

// Reader reads the project Slug under the projects Root.
type Reader struct {
	Root string
	Slug string
	// Bin is the herdr-projects binary; empty means DefaultBin. When it
	// cannot be run, threads are read from threads/t-*.toml instead.
	Bin string
	// Run runs Bin; nil means exec. Tests replace it.
	Run RunFunc
	// History caches the inbox items read for the threads' logs, by
	// file, across reads; nil reads them every time.
	History *History
}

// New returns a Reader for the project slug under root.
func New(root, slug string) Reader {
	return Reader{Root: root, Slug: slug, History: &History{}}
}

// Dir is the project folder, <root>/<slug>.
func (r Reader) Dir() string {
	return filepath.Join(r.Root, r.Slug)
}

// Read returns what can be read of the project now. Sources that are missing
// or unreadable are named in Snapshot.Missing; the rest is still filled in.
func (r Reader) Read(ctx context.Context) deck.Snapshot {
	var missing []string
	note := func(format string, a ...any) { missing = append(missing, fmt.Sprintf(format, a...)) }

	snap := deck.Snapshot{Project: deck.Project{Slug: r.Slug, Dir: r.Dir()}}
	if fi, err := os.Stat(r.Dir()); err != nil || !fi.IsDir() {
		note("project folder %s not found", r.Dir())
		snap.Missing = missing
		return snap
	}

	snap.Project = r.readProject(note)
	snap.Project.Dir = r.Dir()
	tk := r.readTicker(note)
	snap.Threads, snap.ThreadsAsOf = r.readThreads(ctx, tk, note)
	snap.Inbox = r.readInbox(note)
	r.addLogs(snap.Threads)
	snap.Missing = missing
	return snap
}

func execRun(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// openTimeout bounds `herdr-projects open`, which waits for the new
// coordinator's shell before it starts the agent.
const openTimeout = 90 * time.Second

// Open runs `herdr-projects --root <root> open <slug> --tab`, which focuses
// the project's running coordinator, or starts one in a new tab of the
// project's workspace (making the workspace if needed) and focuses it.
// herdr-projects records the coordinator under the project folder, so this
// is the one command the deck runs that writes there, and it runs only when
// the user picks a project with no coordinator. --tab keeps the agent out of
// the deck's own pane.
func (r Reader) Open(ctx context.Context) error {
	bin := r.Bin
	if bin == "" {
		bin = DefaultBin
	}
	run := r.Run
	if run == nil {
		run = execRun
	}
	ctx, cancel := context.WithTimeout(ctx, openTimeout)
	defer cancel()
	_, err := run(ctx, bin, "--root", r.Root, "open", r.Slug, "--tab")
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		return errors.New(firstLine(string(ee.Stderr)))
	}
	return err
}

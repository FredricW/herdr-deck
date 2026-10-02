// Package projects reads one herdr-projects project folder into a
// deck.Snapshot: its settings, threads, pull requests and unhandled inbox
// items. Tasks (TASKS.md) are read by internal/source/tasks.
//
// The reader never writes under the projects root, and the only command it
// runs is `herdr-projects --root <root> thread list <slug> --json`. Every
// source may be missing: what can be read is returned, and what could not is
// listed in Snapshot.Missing. Read never fails.
package projects

import (
	"context"
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
}

// New returns a Reader for the project slug under root.
func New(root, slug string) Reader {
	return Reader{Root: root, Slug: slug}
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
	prs := r.readTicker(note)
	snap.Threads, snap.ThreadsAsOf = r.readThreads(ctx, prs, note)
	snap.Inbox = r.readInbox(note)
	snap.Missing = missing
	return snap
}

func execRun(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

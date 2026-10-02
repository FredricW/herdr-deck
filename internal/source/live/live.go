// Package live reads every source of one project into a deck.Snapshot and
// watches the project folder for changes.
package live

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/dev"
	"github.com/FredricW/herdr-deck/internal/source/herdr"
	"github.com/FredricW/herdr-deck/internal/source/projects"
	"github.com/FredricW/herdr-deck/internal/source/tasks"
)

// Source reads one project: herdr-projects' files and thread list through
// Projects, and TASKS.md through the tasks parser.
type Source struct {
	Projects projects.Reader
	// LinearWorkspace is the slug bare Linear IDs link into. Empty leaves
	// them without a URL, and Read names that in Snapshot.Missing.
	LinearWorkspace string
	// Herdr lays herdr's live panes and agent states over the threads; nil
	// leaves the deck on herdr-projects' data alone.
	Herdr *herdr.Reader
	// Dev reads each worktree's dev servers and probes their ports; nil
	// leaves threads without them.
	Dev *dev.Reader
	// Now stamps Snapshot.ReadAt; nil means time.Now.
	Now func() time.Time
}

// New returns a Source for the project slug under root.
func New(root, slug, linearWorkspace string) Source {
	return Source{Projects: projects.New(root, slug), LinearWorkspace: linearWorkspace}
}

// Read returns what can be read of the project now. Like projects.Reader.Read
// it never fails: unreadable sources are named in Snapshot.Missing. A missing
// TASKS.md is not a missing source; the project simply has no tasks yet.
func (s Source) Read(ctx context.Context) deck.Snapshot {
	snap := s.Projects.Read(ctx)
	dir := s.Projects.Dir()
	threadText := tasks.ThreadFiles(filepath.Join(dir, "threads"))

	f, err := tasks.Load(dir, tasks.Options{LinearWorkspace: s.LinearWorkspace, ThreadText: threadText})
	switch {
	case err == nil:
		snap.TaskLists = f.Lists
	case !errors.Is(err, fs.ErrNotExist):
		snap.Missing = append(snap.Missing, "TASKS.md: "+short(err).Error())
	}

	// A thread's own brief and report give its links too, so a thread no
	// task names still offers them.
	for i := range snap.Threads {
		t := &snap.Threads[i]
		for _, l := range tasks.Scrape(threadText(t.ID), s.LinearWorkspace) {
			t.Links = deck.AppendLink(t.Links, l)
		}
	}

	if s.LinearWorkspace == "" {
		if id := unlinkedLinearID(snap); id != "" {
			snap.Missing = append(snap.Missing, "Linear: no workspace set, so "+id+" and other bare IDs cannot open; set --linear-workspace or $"+deck.EnvLinearWorkspace)
		}
	}

	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	snap.ReadAt = now()
	if s.Herdr != nil {
		s.Herdr.Apply(ctx, &snap, snap.ReadAt)
	}
	if s.Dev != nil {
		// After herdr: its workspace port token is the fallback port.
		s.Dev.Apply(ctx, &snap)
	}
	return snap
}

// unlinkedLinearID returns the first Linear ID in snap that has no URL
// anywhere in it, or "".
func unlinkedLinearID(snap deck.Snapshot) string {
	var all []deck.Link
	for _, list := range snap.TaskLists {
		for _, t := range list.Tasks {
			all = append(all, t.Links...)
		}
	}
	for _, t := range snap.Threads {
		all = append(all, t.Links...)
	}
	linked := map[string]bool{}
	for _, l := range all {
		if l.Kind == deck.LinkLinear && l.URL != "" {
			linked[l.Label] = true
		}
	}
	for _, l := range all {
		if l.Kind == deck.LinkLinear && l.URL == "" && !linked[l.Label] {
			return l.Label
		}
	}
	return ""
}

// short drops the path from a file error; the note already names the file.
func short(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

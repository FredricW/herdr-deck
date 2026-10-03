package projects

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// DefaultRosterEvery is how often a Roster reads the projects root again.
const DefaultRosterEvery = 20 * time.Second

// Roster reads every project under the projects root for the project
// picker. It reads only the files herdr-projects keeps (PROJECT.md,
// .state/project.json, .state/coordinator.json, threads/t-*.toml and
// inbox/) and runs nothing, so it is cheap enough for every reload; still it
// reads the folders again only when its last read is older than Every. The
// deck's own project comes from its snapshot instead, which the project
// folder's watch keeps fresh. It never writes.
type Roster struct {
	// Root is the projects root. FS reads it; nil means os.DirFS(Root).
	Root string
	FS   fs.FS
	// Every is how long a read is reused; zero means DefaultRosterEvery.
	Every time.Duration
	// Now is the clock; nil means time.Now.
	Now func() time.Time

	mu     sync.Mutex
	at     time.Time
	cached []deck.ProjectInfo
}

// NewRoster returns a Roster on the projects root.
func NewRoster(root string) *Roster {
	return &Roster{Root: root}
}

// Read returns every project under the root in slug order, with current's
// entry taken from its snapshot: its threads, inbox and coordinator pane as
// the deck shows them. Each call returns its own copy.
func (r *Roster) Read(current deck.Snapshot) []deck.ProjectInfo {
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	every := r.Every
	if every == 0 {
		every = DefaultRosterEvery
	}
	r.mu.Lock()
	if r.cached == nil || now().Sub(r.at) >= every {
		r.cached, r.at = r.readAll(), now()
	}
	list := make([]deck.ProjectInfo, len(r.cached))
	for i, p := range r.cached {
		p.Threads = slices.Clone(p.Threads)
		p.Inbox = slices.Clone(p.Inbox)
		p.Repos = slices.Clone(p.Repos)
		list[i] = p
	}
	r.mu.Unlock()

	for i := range list {
		p := &list[i]
		if p.Slug != current.Project.Slug {
			continue
		}
		recorded := p.RecordedPane
		p.Project = current.Project
		p.RecordedPane = recorded
		p.Threads = slices.Clone(current.Threads)
		p.Inbox = slices.Clone(current.Inbox)
		p.Problem = ""
	}
	return list
}

func (r *Roster) fsys() fs.FS {
	if r.FS != nil {
		return r.FS
	}
	return os.DirFS(r.Root)
}

// readAll reads every folder under the root that has a PROJECT.md, as
// herdr-projects lists them (src/project.rs, list_slugs).
func (r *Roster) readAll() []deck.ProjectInfo {
	root := r.fsys()
	entries, err := fs.ReadDir(root, ".")
	if err != nil {
		return []deck.ProjectInfo{}
	}
	list := []deck.ProjectInfo{}
	for _, e := range entries {
		slug := e.Name()
		if strings.HasPrefix(slug, ".") || !e.IsDir() {
			continue
		}
		if fi, err := fs.Stat(root, path.Join(slug, "PROJECT.md")); err != nil || fi.IsDir() {
			continue
		}
		sub, err := fs.Sub(root, slug)
		if err != nil {
			continue
		}
		list = append(list, r.readOne(sub, slug))
	}
	return list
}

func (r *Roster) readOne(project fs.FS, slug string) deck.ProjectInfo {
	var problems []string
	note := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	p := deck.ProjectInfo{Project: projectFile(project, slug, note), Status: "active"}
	if r.Root != "" {
		p.Dir = filepath.Join(r.Root, slug)
	}
	var st struct {
		Status string `json:"status"`
	}
	if err := readJSON(project, ".state/project.json", &st); err == nil && st.Status != "" {
		p.Status = st.Status
	}
	var coord struct {
		PaneID string `json:"pane_id"`
	}
	if err := readJSON(project, ".state/coordinator.json", &coord); err == nil {
		p.RecordedPane = coord.PaneID
	}
	recs, _ := threadRecords(project, false, note)
	for _, rec := range recs {
		p.Threads = append(p.Threads, toThread(rec, nil))
	}
	p.Inbox = inboxItems(project, note)
	if len(problems) > 0 {
		p.Problem = strings.Join(problems, "; ")
	}
	return p
}

// readJSON decodes a project file; a missing file is fs.ErrNotExist.
func readJSON(project fs.FS, name string, v any) error {
	data, err := fs.ReadFile(project, name)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return errors.New(name + ": " + err.Error())
	}
	return nil
}

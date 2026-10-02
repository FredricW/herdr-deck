package dev

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// Reader fills in each thread's dev servers and localhost links.
type Reader struct {
	Prober Prober
	// Logs is the folder for the logs and pid files of the `up` commands
	// the deck starts; "" turns `up` off.
	Logs string
	// Start runs an `up` command detached and returns its pid; nil runs it
	// for real. Alive tells whether a pid still runs; nil asks the system.
	// Tests replace both, so no dev server is ever started.
	Start func(Command) (int, error)
	Alive func(pid int) bool

	upMu sync.Mutex // one Up at a time, so a key pressed twice starts once
}

// NewReader returns a Reader that probes real ports.
func NewReader() *Reader { return &Reader{} }

// Apply sets DevServers, DevNote and localhost links on every open thread
// with a worktree. Ports come from the worktree's manifest and state file,
// else from the thread's PortToken (herdr's workspace token, so run it after
// herdr's Apply). A broken manifest or state file is named in snap.Missing,
// a repo without a manifest in snap.Notes; neither stops the rest.
func (r *Reader) Apply(ctx context.Context, snap *deck.Snapshot) {
	seen := map[string]bool{}
	once := func(list *[]string, line string) {
		if !seen[line] {
			seen[line] = true
			*list = append(*list, line)
		}
	}
	type pending struct {
		t     *deck.Thread
		links []linkPlan
	}
	var todo []pending
	var ports []int
	for i := range snap.Threads {
		t := &snap.Threads[i]
		if t.Status == deck.StatusDone || t.Worktree == "" {
			continue
		}
		r.applyUp(snap.Project.Slug, t)
		if fi, err := os.Stat(t.Worktree); err != nil || !fi.IsDir() {
			t.DevNote = "worktree not found"
			continue
		}
		servers, links := r.servers(t, snap, once)
		if len(servers) == 0 && t.PortToken > 0 {
			servers = []deck.DevServer{{Name: "port", Port: t.PortToken, Fallback: true}}
			links = []linkPlan{{label: fmt.Sprintf("~:%d", t.PortToken), url: localURL(t.PortToken), needs: []int{t.PortToken}}}
			t.DevNote = ""
		}
		t.DevServers = servers
		for _, s := range servers {
			ports = append(ports, s.Port)
		}
		todo = append(todo, pending{t: t, links: links})
	}
	if len(todo) == 0 {
		return
	}

	up := r.Prober.Listening(ctx, ports)
	for _, p := range todo {
		for i := range p.t.DevServers {
			p.t.DevServers[i].Running = up[p.t.DevServers[i].Port]
		}
		for _, lp := range p.links {
			down := false
			for _, port := range lp.needs {
				down = down || !up[port]
			}
			p.t.Links = deck.AppendLink(p.t.Links, deck.Link{Kind: deck.LinkLocalhost, Label: lp.label, URL: lp.url, Down: down})
		}
	}
}

// linkPlan is a localhost link before its ports are probed.
type linkPlan struct {
	label, url string
	needs      []int // the ports that must listen
}

// servers reads the thread's manifest and state file.
func (r *Reader) servers(t *deck.Thread, snap *deck.Snapshot, once func(*[]string, string)) ([]deck.DevServer, []linkPlan) {
	repo := t.Repo
	if repo == "" {
		repo = t.Worktree
	}
	path, m, err := FindManifest(t.Worktree, t.Repo)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		t.DevNote = "no " + ManifestPath
		once(&snap.Notes, fmt.Sprintf("dev servers: %s has no %s; only herdr's port token, if any", filepath.Base(repo), ManifestPath))
		return nil, nil
	case err != nil:
		t.DevNote = ManifestPath + " is invalid (! shows why)"
		once(&snap.Missing, fmt.Sprintf("dev servers: %s: %v", shortPath(path), err))
		return nil, nil
	}

	v := vars{worktree: t.Worktree, repo: repo, branch: t.Branch, ports: map[string]int{}}
	keyed := false
	for _, p := range m.State.Ports {
		if p.Fixed > 0 {
			v.ports[p.Name] = p.Fixed
		}
		keyed = keyed || p.Key != ""
	}
	if keyed {
		file, unknown := v.expand(m.State.File)
		if len(unknown) > 0 {
			once(&snap.Missing, fmt.Sprintf("dev servers: %s: state.file uses unknown $%s", shortPath(path), unknown[0]))
			t.DevNote = ManifestPath + " is invalid (! shows why)"
			return nil, nil
		}
		if !filepath.IsAbs(file) {
			file = filepath.Join(repo, file)
		}
		got, err := readState(file, m.State.Ports)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			t.DevNote = "not started: no " + relTo(file, repo, t.Worktree)
		case err != nil:
			t.DevNote = "state file unreadable (! shows why)"
			once(&snap.Missing, fmt.Sprintf("dev servers: %s: %v", relTo(file, repo, t.Worktree), short(err)))
		}
		for name, port := range got {
			v.ports[name] = port
		}
	}

	var servers []deck.DevServer
	for _, p := range m.State.Ports {
		if port, ok := v.ports[p.Name]; ok {
			servers = append(servers, deck.DevServer{Name: p.Name, Port: port})
		}
	}
	if len(servers) == 0 {
		if t.DevNote == "" {
			t.DevNote = "the state file names no ports yet"
		}
		return nil, nil
	}
	t.DevNote = ""

	var links []linkPlan
	if len(m.Links) == 0 {
		for _, s := range servers {
			links = append(links, linkPlan{label: fmt.Sprintf(":%d %s", s.Port, s.Name), url: localURL(s.Port), needs: []int{s.Port}})
		}
		return servers, links
	}
	for _, spec := range m.Links {
		url, unknown := v.expand(spec.URL)
		if len(unknown) > 0 {
			continue // a $PORT_ of a server whose port is not known yet
		}
		needs := spec.Needs
		if len(needs) == 0 {
			needs = portRefs(spec.URL)
		}
		lp := linkPlan{label: spec.Title, url: url}
		if lp.label == "" {
			lp.label = strings.TrimPrefix(strings.TrimPrefix(url, "http://"), "https://")
		}
		for _, n := range needs {
			// A server without a port yet has not started: port 0
			// never listens, so the link shows as down.
			lp.needs = append(lp.needs, v.ports[n])
		}
		links = append(links, lp)
	}
	return servers, links
}

func localURL(port int) string { return "http://localhost:" + strconv.Itoa(port) }

// shortPath names a manifest by its repo folder: "webshop/.herdr-deck/dev.json".
func shortPath(path string) string {
	return filepath.Join(filepath.Base(filepath.Dir(filepath.Dir(path))), ManifestPath)
}

// relTo shows path relative to the repo or worktree when it is inside one.
func relTo(path string, dirs ...string) string {
	for _, d := range dirs {
		if rel, err := filepath.Rel(d, path); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return path
}

// short drops the path from a file error; the line already names the file.
func short(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

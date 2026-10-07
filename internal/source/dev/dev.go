// Package dev finds the dev servers of each thread's worktree from its dev
// manifest (.config/dev.json, docs/dev-manifest.md; or the legacy
// .herdr-deck/dev.json), probes their ports, and starts and stops them.
// What it starts gets run records and logs in the shared state folder, so
// other tools that read the manifest see it too.
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
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/dev/manifest"
)

// Reader fills in each thread's dev servers and localhost links, and runs
// the manifest's dev and stop.
type Reader struct {
	Prober Prober
	// State is the shared dev-manifest state folder; "" turns starting
	// and stopping off.
	State string
	// Start runs a command detached and returns its pid; Run runs one and
	// waits for it; Signal sends SIGTERM (SIGKILL when force is set) to a
	// process group; Alive tells whether a pid still runs. nil does each
	// for real. Tests replace them, so no dev server is ever started.
	Start  func(Command) (int, error)
	Run    func(context.Context, Command) error
	Signal func(pid int, force bool) error
	Alive  func(pid int, started time.Time) bool

	mu    sync.Mutex
	busy  map[string]bool // worktrees with a dev or stop running
	alive map[aliveKey]probe
}

// NewReader returns a Reader that probes real ports.
func NewReader() *Reader { return &Reader{} }

func (r *Reader) store() Store { return Store{Dir: r.State, Alive: r.Alive} }

// aliveTTL is how long a refresh reuses a process check: each one runs ps.
const aliveTTL = 2 * time.Second

// cachedStore is store() with process checks remembered for aliveTTL, for
// Apply's refreshes; starting and stopping ask afresh.
func (r *Reader) cachedStore() Store {
	st := r.store()
	alive := st.IsAlive
	st.Alive = func(pid int, started time.Time) bool {
		k := aliveKey{pid, started.Unix()}
		r.mu.Lock()
		c, ok := r.alive[k]
		r.mu.Unlock()
		if ok && time.Since(c.at) < aliveTTL {
			return c.up
		}
		up := alive(Record{PID: pid, Started: started.UTC().Format(time.RFC3339)})
		r.mu.Lock()
		if r.alive == nil {
			r.alive = map[aliveKey]probe{}
		}
		r.alive[k] = probe{up: up, at: time.Now()}
		r.mu.Unlock()
		return up
	}
	return st
}

type aliveKey struct {
	pid     int
	started int64
}

// Apply sets DevServers, DevNote, DevUp and localhost links on every open
// thread with a worktree. Without a manifest (or a port), the thread's
// PortToken (herdr's workspace token, so run it after herdr's Apply) is
// the fallback. A broken manifest or state file is named in snap.Missing;
// a repo without a manifest, a legacy one and ports the deck cannot know
// yet in snap.Notes. None of them stops the rest.
func (r *Reader) Apply(ctx context.Context, snap *deck.Snapshot) {
	seen := map[string]bool{}
	once := func(list *[]string, line string) {
		if !seen[line] {
			seen[line] = true
			*list = append(*list, line)
		}
	}
	type pending struct {
		t *deck.Thread
		p *plan
	}
	var todo []pending
	var ports []int
	var checks []check
	for i := range snap.Threads {
		t := &snap.Threads[i]
		if t.Status == deck.StatusDone || t.Worktree == "" {
			continue
		}
		r.devUp(t)
		if fi, err := os.Stat(t.Worktree); err != nil || !fi.IsDir() {
			t.DevNote = "worktree not found"
			continue
		}
		p := r.read(t, snap, once)
		if p != nil {
			for _, n := range p.vars.Ports {
				ports = append(ports, n)
			}
			for i := range p.m.Services {
				checks = append(checks, p.readyCheck(&p.m.Services[i]))
			}
		}
		if t.PortToken > 0 && (p == nil || !p.anyMainPort()) {
			ports = append(ports, t.PortToken) // the fallback when the manifest gives no port
		}
		todo = append(todo, pending{t, p})
	}
	if len(todo) == 0 {
		return
	}

	up := r.Prober.Listening(ctx, ports)
	httpOK := r.Prober.Ready(ctx, checks)
	for _, x := range todo {
		if x.p != nil {
			r.fill(x.t, x.p, up, httpOK)
		}
		if len(x.t.DevServers) == 0 && x.t.PortToken > 0 && x.t.DevUp == nil {
			port := x.t.PortToken
			x.t.DevServers = []deck.DevServer{{Name: "port", Port: port, Running: up[port], Fallback: true}}
			x.t.Links = deck.AppendLink(x.t.Links, deck.Link{Kind: deck.LinkLocalhost, Label: fmt.Sprintf("~:%d", port), URL: localURL(port), Down: !up[port]})
			x.t.DevNote = ""
		}
	}
}

// devUp sets DevUp from the dev command's run record, also when the
// worktree is gone.
func (r *Reader) devUp(t *deck.Thread) {
	if r.State == "" {
		return
	}
	st := r.cachedStore()
	if rec, ok := st.Record(Key(resolved(t.Worktree)), "dev."+DefaultGroup); ok {
		t.DevUp = &deck.DevUp{Log: rec.Log, Alive: st.IsAlive(rec)}
	}
}

// read finds a thread's manifest and reads its ports; nil when the thread
// has no usable manifest, with DevNote saying why.
func (r *Reader) read(t *deck.Thread, snap *deck.Snapshot, once func(*[]string, string)) *plan {
	p, err := newPlan(*t)
	repo := t.Repo
	if repo == "" {
		repo = t.Worktree
	}
	switch {
	case errors.Is(err, fs.ErrNotExist):
		t.DevNote = "no " + manifest.Path
		once(&snap.Notes, fmt.Sprintf("dev servers: %s has no %s; only herdr's port token, if any", filepath.Base(repo), manifest.Path))
		return nil
	case err != nil:
		t.DevNote = p.found.Rel + " is invalid (! shows why)"
		once(&snap.Missing, fmt.Sprintf("dev servers: %s: %v", p.found.Short(), short(err)))
		return nil
	}
	if p.found.Legacy {
		once(&snap.Notes, fmt.Sprintf("dev servers: %s is legacy; move it to %s (docs/dev-manifest.md, section 15)", p.found.Short(), manifest.Path))
	}
	if len(p.m.Unknown) > 0 {
		once(&snap.Notes, fmt.Sprintf("dev servers: %s: ignored unknown keys %s", p.found.Short(), strings.Join(p.m.Unknown, ", ")))
	}
	var store []string
	for _, port := range p.m.Ports {
		if port.Base > 0 {
			store = append(store, port.Name)
		}
	}
	if len(store) > 0 {
		once(&snap.Notes, fmt.Sprintf("dev servers: %s: ports %s come from the shared port store, which the deck does not support yet", p.found.Short(), strings.Join(store, ", ")))
	}
	for _, pr := range p.problems {
		once(&snap.Missing, "dev servers: "+pr)
	}
	return p
}

// fill sets a thread's dev servers, DevUp and links from its plan and the
// probes' answers.
func (r *Reader) fill(t *deck.Thread, p *plan, up map[int]bool, httpOK map[check]bool) {
	st := r.cachedStore()
	ready := map[string]bool{}
	known, records := false, false
	var servers []deck.DevServer
	for i := range p.m.Services {
		s := &p.m.Services[i]
		ds := deck.DevServer{Name: s.Name, Title: s.Title}
		_, ds.Port, ds.Pending = p.mainPort(s)
		known = known || ds.Port > 0
		c := p.readyCheck(s)
		var rec Record
		hasRec, alive := false, false
		if r.State != "" {
			rec, hasRec = st.Record(p.key, "service."+s.Name)
			alive = hasRec && st.IsAlive(rec)
			records = records || hasRec
		}
		switch {
		case c.Path != "":
			ds.Running = httpOK[c]
		case c.Port > 0:
			ds.Running = up[c.Port]
		case len(s.Ports) == 0:
			ds.Running = alive
		}
		ds.Starting = alive && !ds.Running
		ds.Exited = hasRec && !alive && !ds.Running
		ready[s.Name] = ds.Running
		if hasRec && rec.Log != "" {
			ds.Log = rec.Log
		} else if log, err := p.logOf(st, s); err == nil && exists(log) {
			ds.Log = log
		}
		servers = append(servers, ds)
	}
	_, devCmd := p.m.Command("dev")
	switch {
	case len(p.m.Ports) > 0 && !known && !records:
		// No port has a number yet: nothing has started.
		switch {
		case p.stateMissing:
			t.DevNote = "not started: no " + p.rel(p.stateFile)
		case p.m.StateFile != "" && len(p.pending) > 0 && allPending(p, "not in the state file yet"):
			t.DevNote = "the state file names no ports yet"
		default:
			t.DevNote = "ports not known yet (! shows why)"
		}
		servers = nil
	case len(servers) == 0 && devCmd:
		if c, _ := p.m.Command("dev"); !c.Null {
			t.DevNote = "not started"
		}
	default:
		t.DevNote = ""
	}
	t.DevServers = servers

	answers := func(name string) bool {
		if _, ok := ready[name]; ok {
			return ready[name]
		}
		n, ok := p.vars.Ports[name]
		return ok && up[n]
	}
	if len(p.m.Links) == 0 {
		for i := range p.m.Services {
			s := &p.m.Services[i]
			for _, name := range s.Ports {
				if n, ok := p.vars.Ports[name]; ok {
					t.Links = deck.AppendLink(t.Links, deck.Link{Kind: deck.LinkLocalhost, Label: fmt.Sprintf(":%d %s", n, name), URL: localURL(n), Down: !up[n]})
				}
			}
		}
		return
	}
	for _, l := range p.m.Links {
		url, err := p.vars.Expand(l.URL)
		if err != nil {
			continue // a port that is not known yet
		}
		label := strings.TrimPrefix(strings.TrimPrefix(url, "http://"), "https://")
		if l.Title != "" {
			if title, err := p.vars.Expand(l.Title); err == nil {
				label = title
			}
		}
		needs := l.Needs
		if len(needs) == 0 {
			needs = manifest.PortRefs(l.URL)
		}
		down := false
		for _, n := range needs {
			down = down || !answers(n)
		}
		t.Links = deck.AppendLink(t.Links, deck.Link{Kind: deck.LinkLocalhost, Label: label, URL: url, Down: down})
	}
}

func allPending(p *plan, why string) bool {
	for _, w := range p.pending {
		if w != why {
			return false
		}
	}
	return true
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func localURL(port int) string { return "http://localhost:" + strconv.Itoa(port) }

// short drops the path from a file error; the line already names the file.
func short(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

package dev

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/dev/manifest"
)

// plan is a worktree's manifest with what the deck knows of it: the
// variables, which ports have numbers and why the others do not.
type plan struct {
	found    manifest.Found
	m        *manifest.Manifest
	vars     manifest.Vars
	worktree string
	key      string // the worktree's name in the shared state folder
	// pending says, per port name, why its number is not known.
	pending map[string]string
	// stateFile is the state file's path; stateMissing is set when it
	// does not exist (yet).
	stateFile    string
	stateMissing bool
	// problems are worth showing in the Sources view: a state file that
	// cannot be read, a template that cannot be expanded.
	problems []string
}

// errNoStateFile says a port wants the state file, but its path cannot be
// worked out.
var errNoStateFile = errors.New("state.file cannot be expanded")

// newPlan reads a thread's manifest and its ports. An error is the
// manifest's: fs.ErrNotExist without one, else a broken file.
func newPlan(t deck.Thread) (*plan, error) {
	found, err := manifest.Find(t.Worktree, t.Repo)
	if err != nil {
		return &plan{found: found}, err
	}
	wt := resolved(t.Worktree)
	repo := wt
	if t.Repo != "" {
		repo = resolved(t.Repo)
	}
	p := &plan{
		found:    found,
		m:        found.Manifest,
		worktree: wt,
		key:      Key(wt),
		pending:  map[string]string{},
		vars:     manifest.Vars{Worktree: wt, Repo: repo, Branch: t.Branch, Ports: map[string]int{}},
	}
	// state.file may use $env(…); the env files may use the ports.
	p.vars.Env = p.m.LoadEnv(p.vars)

	var state map[string]any
	stateErr := error(nil)
	for _, port := range p.m.Ports {
		switch {
		case port.Fixed > 0:
			p.vars.Ports[port.Name] = port.Fixed
		case port.Base > 0:
			p.pending[port.Name] = "port store not supported yet"
		case port.State != "":
			if state == nil && stateErr == nil {
				state, stateErr = p.readState()
			}
			if n, ok := statePort(state[port.State]); ok {
				p.vars.Ports[port.Name] = n
			} else {
				p.pending[port.Name] = "not in the state file yet"
			}
		}
	}
	p.vars.Env = p.m.LoadEnv(p.vars)
	switch {
	case errors.Is(stateErr, fs.ErrNotExist):
		p.stateMissing = true
	case errors.Is(stateErr, errNoStateFile):
	case stateErr != nil:
		p.problems = append(p.problems, fmt.Sprintf("%s: %v", p.rel(p.stateFile), short(stateErr)))
	}
	return p, nil
}

// resolved is a path with its symlinks resolved, as $WORKTREE and $REPO
// are.
func resolved(path string) string {
	if r, err := filepath.EvalSymlinks(path); err == nil {
		return r
	}
	// A worktree that is gone: resolve the nearest folder that exists, so
	// its run records keep their key.
	if parent := filepath.Dir(path); parent != path {
		return filepath.Join(resolved(parent), filepath.Base(path))
	}
	return path
}

func (p *plan) readState() (map[string]any, error) {
	file, err := p.vars.Path(p.m.StateFile)
	if err != nil {
		p.problems = append(p.problems, fmt.Sprintf("%s: state.file: %v", p.found.Short(), err))
		return nil, errNoStateFile
	}
	p.stateFile = file
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// statePort reads a state file value: a port number, or a string holding
// one.
func statePort(v any) (int, bool) {
	var n int
	switch v := v.(type) {
	case float64:
		if v != float64(int(v)) {
			return 0, false
		}
		n = int(v)
	case string:
		n, _ = strconv.Atoi(strings.TrimSpace(v))
	}
	return n, n > 0 && n < 65536
}

// rel shows a path relative to the worktree or the main checkout when it
// is inside one.
func (p *plan) rel(path string) string {
	for _, d := range []string{p.vars.Worktree, p.vars.Repo} {
		if rel, err := filepath.Rel(d, path); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return path
}

// check is how the deck tells a service is ready: a TCP connect to Port,
// or a GET of Path on it. Port 0 means it cannot ask.
type check struct {
	Port int
	Path string
}

// readyCheck is a service's readiness check (spec section 6.1); the zero
// check when the service has no port, or its port has no number yet.
func (p *plan) readyCheck(s *manifest.Service) check {
	name := s.Ready.Port
	if name == "" && len(s.Ports) > 0 {
		name = s.Ports[0]
	}
	n, ok := p.vars.Ports[name]
	if !ok {
		return check{}
	}
	c := check{Port: n}
	if s.Ready.HTTP != "" {
		path, err := p.vars.Expand(s.Ready.HTTP)
		if err != nil {
			return check{}
		}
		c.Path = path
	}
	return c
}

// mainPort is a service's first port, its number (0 when not known) and
// why it is not known.
func (p *plan) mainPort(s *manifest.Service) (name string, n int, pending string) {
	if len(s.Ports) == 0 {
		return "", 0, ""
	}
	name = s.Ports[0]
	return name, p.vars.Ports[name], p.pending[name]
}

// anyMainPort reports whether some service's main port has a number.
func (p *plan) anyMainPort() bool {
	for i := range p.m.Services {
		if _, n, _ := p.mainPort(&p.m.Services[i]); n > 0 {
			return true
		}
	}
	return false
}

// logOf is a service's log: its log field, else the shared state folder's.
func (p *plan) logOf(store Store, s *manifest.Service) (string, error) {
	if s.Log != "" {
		return p.vars.Path(s.Log)
	}
	if store.Dir == "" {
		return "", errors.New("no folder for logs: $HOME is not set")
	}
	return store.Log(p.key, "service."+s.Name), nil
}

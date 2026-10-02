// Package dev finds the dev servers of each thread's worktree: it reads the
// worktree's dev-server manifest (.herdr-deck/dev.json), the state file the
// manifest points at, and probes the ports it names. The manifest format is
// documented in the README.
package dev

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ManifestPath is where the manifest lives, relative to a worktree or repo.
const ManifestPath = ".herdr-deck/dev.json"

// Manifest says where a worktree's dev-server ports are and which links
// they serve.
type Manifest struct {
	State State      `json:"state"`
	Links []LinkSpec `json:"links"`
	// Up is a shell command that starts the worktree's dev servers, run
	// in the worktree by `u`; "" when the manifest gives none.
	Up string `json:"up"`
}

// State names the per-worktree state file and the ports in it.
type State struct {
	// File is a path template, e.g. ".dev/$DIRNAME/state.json". Relative
	// paths are under the repository's main checkout.
	File  string `json:"file"`
	Ports Ports  `json:"ports"`
}

// Port is one named dev server: the state file's key that holds its port,
// or a fixed port number.
type Port struct {
	Name  string
	Key   string
	Fixed int
}

// Ports keeps the manifest's order, which is the order the deck shows the
// servers in.
type Ports []Port

// UnmarshalJSON reads an object of name → key (a string) or name → port (a
// number), in order.
func (p *Ports) UnmarshalJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return errors.New("ports: want an object of server name → state-file key")
	}
	*p = nil
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		name := tok.(string)
		var v any
		if err := dec.Decode(&v); err != nil {
			return err
		}
		port := Port{Name: name}
		switch v := v.(type) {
		case string:
			port.Key = v
		case float64:
			port.Fixed = int(v)
			if float64(port.Fixed) != v || !validPort(port.Fixed) {
				return fmt.Errorf("ports.%s: %v is not a port number", name, v)
			}
		default:
			return fmt.Errorf("ports.%s: want a state-file key or a port number", name)
		}
		*p = append(*p, port)
	}
	_, err := dec.Token()
	return err
}

// LinkSpec is a URL template with `$PORT_<name>` placeholders, and the
// servers that must listen for it to work.
type LinkSpec struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Needs Needs  `json:"needs"`
}

// Needs is one server name or a list of them.
type Needs []string

func (n *Needs) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*n = Needs{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return errors.New("needs: want a server name or a list of them")
	}
	*n = many
	return nil
}

// ParseManifest reads and checks a manifest.
func ParseManifest(b []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, err
	}
	return m, m.check()
}

func (m Manifest) check() error {
	if len(m.State.Ports) == 0 {
		return errors.New("state.ports names no servers")
	}
	known := map[string]bool{}
	keyed := false
	for _, p := range m.State.Ports {
		if p.Name == "" || !nameRe.MatchString(p.Name) {
			return fmt.Errorf("state.ports: %q is not a server name (letters, digits, _)", p.Name)
		}
		known[p.Name] = true
		keyed = keyed || p.Key != ""
	}
	if keyed && m.State.File == "" {
		return errors.New("state.file is empty, but state.ports names keys in it")
	}
	for i, l := range m.Links {
		if l.URL == "" {
			return fmt.Errorf("links[%d]: no url", i)
		}
		for _, n := range l.Needs {
			if !known[n] {
				return fmt.Errorf("links[%d]: needs %q, which state.ports does not name", i, n)
			}
		}
		if err := checkVars(l.URL, known); err != nil {
			return fmt.Errorf("links[%d]: %w", i, err)
		}
	}
	if err := checkVars(m.State.File, nil); err != nil {
		return fmt.Errorf("state.file: %w", err)
	}
	if err := checkVars(m.Up, nil); err != nil {
		return fmt.Errorf("up: %w", err)
	}
	return nil
}

// checkVars reports the first placeholder in a template that expand cannot
// fill: an unknown name, or a $PORT_ of a server not in known.
func checkVars(s string, known map[string]bool) error {
	var err error
	os.Expand(s, func(name string) string {
		if err != nil || name == "$" || pathVars[name] {
			return ""
		}
		if n, ok := strings.CutPrefix(name, "PORT_"); ok && known != nil {
			if !known[n] {
				err = fmt.Errorf("$PORT_%s, but state.ports names no %q", n, n)
			}
			return ""
		}
		err = fmt.Errorf("unknown $%s (write $$ for a literal $)", name)
		return ""
	})
	return err
}

// pathVars are the placeholders every template may use.
var pathVars = map[string]bool{"DIRNAME": true, "WORKTREE": true, "REPO": true, "BRANCH": true}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// FindManifest reads the manifest of a worktree: its own, else its
// repository's main checkout's. It returns the path read, and an error
// wrapping fs.ErrNotExist when neither has one.
func FindManifest(worktree, repo string) (string, Manifest, error) {
	for _, dir := range []string{worktree, repo} {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, ManifestPath)
		b, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return path, Manifest{}, err
		}
		m, err := ParseManifest(b)
		return path, m, err
	}
	return "", Manifest{}, fs.ErrNotExist
}

// vars are the values path and URL templates may use.
type vars struct {
	worktree, repo, branch string
	ports                  map[string]int
}

// expand fills in $DIRNAME, $WORKTREE, $REPO, $BRANCH and $PORT_<name>
// (also as ${…}); $$ is a literal $. It returns the names it could not fill.
func (v vars) expand(s string) (string, []string) {
	var unknown []string
	out := os.Expand(s, func(name string) string {
		switch name {
		case "DIRNAME":
			return filepath.Base(v.worktree)
		case "WORKTREE":
			return v.worktree
		case "REPO":
			return v.repo
		case "BRANCH":
			return v.branch
		case "$":
			return "$" // $$ is a literal $
		}
		if n, ok := strings.CutPrefix(name, "PORT_"); ok {
			if p, ok := v.ports[n]; ok {
				return strconv.Itoa(p)
			}
		}
		unknown = append(unknown, name)
		return ""
	})
	return out, unknown
}

// portRefs lists the server names a URL template's $PORT_<name>
// placeholders use.
func portRefs(s string) []string {
	var names []string
	os.Expand(s, func(name string) string {
		if n, ok := strings.CutPrefix(name, "PORT_"); ok {
			names = append(names, n)
		}
		return ""
	})
	return names
}

// readState reads the ports from a state file: each key's value, a number
// or a numeric string. Keys missing from the file are left out.
func readState(path string, ports Ports) (map[string]int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, p := range ports {
		if p.Key == "" {
			continue
		}
		var n int
		switch v := obj[p.Key].(type) {
		case float64:
			n = int(v)
		case string:
			n, _ = strconv.Atoi(strings.TrimSpace(v))
		}
		if validPort(n) {
			out[p.Name] = n
		}
	}
	return out, nil
}

func validPort(n int) bool { return n > 0 && n < 65536 }

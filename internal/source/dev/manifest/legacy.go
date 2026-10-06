package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// legacy mirrors .herdr-deck/dev.json, the deck's own manifest before the
// shared one.
type legacy struct {
	State struct {
		// File is relative to the repository's main checkout.
		File  string          `json:"file"`
		Ports json.RawMessage `json:"ports"`
	} `json:"state"`
	Links []struct {
		Title string `json:"title"`
		URL   string `json:"url"`
		Needs needs  `json:"needs"`
	} `json:"links"`
	Up string `json:"up"`
}

var legacyName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// ParseLegacy reads a .herdr-deck/dev.json and converts it as the spec's
// section 15 says: state.ports become ports (a state key or a fixed
// number), a relative state.file gets $REPO/ in front, and up becomes
// commands.dev. Each port is a service of its own.
func ParseLegacy(b []byte) (*Manifest, error) {
	var l legacy
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, err
	}
	m := &Manifest{}
	ports, err := ordered(l.State.Ports)
	if err != nil {
		return nil, fmt.Errorf("state.ports: %w", err)
	}
	if len(ports) == 0 {
		return nil, errors.New("state.ports names no servers")
	}
	known := map[string]bool{}
	keyed := false
	for _, kv := range ports {
		if !legacyName.MatchString(kv.name) {
			return nil, fmt.Errorf("state.ports: %q is not a server name (letters, digits, _)", kv.name)
		}
		p := Port{Name: kv.name}
		var key string
		var n float64
		switch {
		case json.Unmarshal(kv.raw, &key) == nil:
			p.State = key
			keyed = true
		case json.Unmarshal(kv.raw, &n) == nil:
			p.Fixed = int(n)
			if float64(p.Fixed) != n || p.Fixed < 1 || p.Fixed > 65535 {
				return nil, fmt.Errorf("state.ports.%s: %v is not a port number", kv.name, n)
			}
		default:
			return nil, fmt.Errorf("state.ports.%s: want a state-file key or a port number", kv.name)
		}
		known[p.Name] = true
		m.Ports = append(m.Ports, p)
		m.Services = append(m.Services, Service{Name: p.Name, Title: p.Name, Ports: []string{p.Name}, Ready: Ready{Timeout: DefaultTimeout}, Implicit: true})
	}
	if keyed && l.State.File == "" {
		return nil, errors.New("state.file is empty, but state.ports names keys in it")
	}
	if f := l.State.File; f != "" {
		if !filepath.IsAbs(f) && !strings.HasPrefix(f, "$") {
			f = "$REPO/" + f
		}
		m.StateFile = f
	}
	portVars := map[string]map[string]any{}
	for n := range known {
		portVars[n] = nil
	}
	check := func(path, s string, ports map[string]map[string]any) error {
		if strings.Contains(s, "$env(") {
			return fmt.Errorf("%s: $env(…) needs .config/dev.json", path)
		}
		if p := templateProblems(s, ports); len(p) > 0 {
			return fmt.Errorf("%s: %s", path, p[0])
		}
		return nil
	}
	for i, ln := range l.Links {
		if ln.URL == "" {
			return nil, fmt.Errorf("links[%d]: no url", i)
		}
		for _, n := range ln.Needs {
			if !known[n] {
				return nil, fmt.Errorf("links[%d]: needs %q, which state.ports does not name", i, n)
			}
		}
		if err := check(fmt.Sprintf("links[%d]", i), ln.URL, portVars); err != nil {
			return nil, err
		}
		m.Links = append(m.Links, Link{Title: ln.Title, URL: ln.URL, Needs: ln.Needs})
	}
	if err := check("state.file", l.State.File, nil); err != nil {
		return nil, err
	}
	if l.Up != "" {
		if err := check("up", l.Up, nil); err != nil {
			return nil, err
		}
		m.Commands = append(m.Commands, Command{Name: "dev", Run: &Run{Shell: l.Up}})
	}
	return m, nil
}

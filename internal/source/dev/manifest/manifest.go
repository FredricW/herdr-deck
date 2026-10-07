package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Manifest is a decoded, checked manifest. Lists keep the file's order.
type Manifest struct {
	Name string
	// EnvFiles are dotenv file templates; EnvVars the variables set for
	// every command (templates).
	EnvFiles []string
	EnvVars  map[string]string
	// Path lists folder templates put in front of PATH.
	Path []string
	// StateFile is the template of the project's per-worktree state file;
	// "" when there is none.
	StateFile string
	Ports     []Port
	// Services holds the declared services, then a service for each port
	// that no service lists (spec section 6.1), in the order of ports.
	Services []Service
	Groups   []Group
	Commands []Command
	Links    []Link
	// Unknown lists keys the schema does not know; they are ignored.
	Unknown []string
}

// Port is a named port and where its number comes from: exactly one of
// Fixed, Base or State is set.
type Port struct {
	Name  string
	Fixed int
	Base  int
	Range int // with Base; 100 when the file gives none
	State string
}

// Service is a long-running process.
type Service struct {
	Name  string
	Title string // the name when the file gives none
	Dir   string
	Run   *Run // nil: something else starts it
	Stop  *Run
	// Ports are the service's port names, main one first.
	Ports   []string
	Needs   []string
	Ready   Ready
	Env     map[string]string
	LoadEnv bool
	Log     string // a template; "" means the shared state folder's
	// Implicit is set on a service made for a port that no service lists.
	Implicit bool
}

// Ready says when a service counts as ready (spec section 6.1).
type Ready struct {
	Port    string // "" means the main port
	HTTP    string // a path; "" means a TCP connect
	Timeout time.Duration
}

// DefaultTimeout is how long a service may take to get ready.
const DefaultTimeout = 60 * time.Second

// Run is one runnable command: a shell string or an argv, with options.
type Run struct {
	Shell string // run by /bin/sh -c; "" when Argv is set
	Argv  []string
	Dir   string
	Env   map[string]string
	// LoadEnv also loads EnvFiles into the environment.
	LoadEnv  bool
	Terminal *bool // nil: the command's default
	Needs    []string
	Title    string
}

// Text is the command as the file writes it, for logs and messages.
func (r *Run) Text() string {
	if r.Shell != "" {
		return r.Shell
	}
	return strings.Join(r.Argv, " ")
}

// Command is a named command: Null (the repository has none), one Run, or
// a per-service map in Entries.
type Command struct {
	Name    string
	Null    bool
	Run     *Run
	Entries []Entry
}

// Entry is one key of a per-service command map: a service name or "*".
type Entry struct {
	Service string
	Run     *Run
}

// Group is a named list of services.
type Group struct {
	Name     string
	Services []string
}

// Link is a page to open.
type Link struct {
	Title string
	URL   string
	Needs []string
}

// Command returns the named command, and whether the file has the name at
// all (a null command is there).
func (m *Manifest) Command(name string) (Command, bool) {
	for _, c := range m.Commands {
		if c.Name == name {
			return c, true
		}
	}
	return Command{}, false
}

// Service returns the named service, or nil.
func (m *Manifest) Service(name string) *Service {
	for i := range m.Services {
		if m.Services[i].Name == name {
			return &m.Services[i]
		}
	}
	return nil
}

// Port returns the named port, or nil.
func (m *Manifest) Port(name string) *Port {
	for i := range m.Ports {
		if m.Ports[i].Name == name {
			return &m.Ports[i]
		}
	}
	return nil
}

// DefaultGroup is the group called default, else every service.
func (m *Manifest) DefaultGroup() []string {
	for _, g := range m.Groups {
		if g.Name == "default" {
			return g.Services
		}
	}
	var all []string
	for _, s := range m.Services {
		all = append(all, s.Name)
	}
	return all
}

// StartOrder returns the services in names plus everything they need,
// transitively, each after what it needs; ties keep the manifest's order.
// The checker has ruled out cycles.
func (m *Manifest) StartOrder(names []string) []string {
	want := map[string]bool{}
	var add func(string)
	add = func(n string) {
		if want[n] || m.Service(n) == nil {
			return
		}
		want[n] = true
		for _, d := range m.Service(n).Needs {
			add(d)
		}
	}
	for _, n := range names {
		add(n)
	}
	var out []string
	done := map[string]bool{}
	var visit func(string)
	visit = func(n string) {
		if done[n] {
			return
		}
		done[n] = true
		for _, d := range m.Service(n).Needs {
			visit(d)
		}
		out = append(out, n)
	}
	for _, s := range m.Services {
		if want[s.Name] {
			visit(s.Name)
		}
	}
	return out
}

// Parse checks and decodes a manifest's content. An error lists every
// problem Check found.
func Parse(b []byte) (*Manifest, error) {
	p := Check(b)
	if len(p.Errors) > 0 {
		return nil, problemsError(p.Errors)
	}
	m, err := decode(b)
	if err != nil {
		return nil, err
	}
	m.Unknown = p.Unknown
	return m, nil
}

// problemsError joins a file's problems into one error, the first first.
func problemsError(errs []string) error {
	if len(errs) == 1 {
		return errors.New(errs[0])
	}
	return fmt.Errorf("%s (and %d more)", errs[0], len(errs)-1)
}

// file mirrors the manifest's JSON; the ordered objects stay raw.
type file struct {
	Name string `json:"name"`
	Env  struct {
		Files []string          `json:"files"`
		Vars  map[string]string `json:"vars"`
	} `json:"env"`
	Path  []string `json:"path"`
	State struct {
		File string `json:"file"`
	} `json:"state"`
	Ports    json.RawMessage `json:"ports"`
	Services json.RawMessage `json:"services"`
	Groups   json.RawMessage `json:"groups"`
	Commands json.RawMessage `json:"commands"`
	Links    []struct {
		Title string `json:"title"`
		URL   string `json:"url"`
		Needs needs  `json:"needs"`
	} `json:"links"`
}

func decode(b []byte) (*Manifest, error) {
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	m := &Manifest{Name: f.Name, EnvFiles: f.Env.Files, EnvVars: f.Env.Vars, Path: f.Path, StateFile: f.State.File}

	ports, err := ordered(f.Ports)
	if err != nil {
		return nil, fmt.Errorf("ports: %w", err)
	}
	for _, kv := range ports {
		p, err := decodePort(kv.name, kv.raw)
		if err != nil {
			return nil, err
		}
		m.Ports = append(m.Ports, p)
	}

	services, err := ordered(f.Services)
	if err != nil {
		return nil, fmt.Errorf("services: %w", err)
	}
	owned := map[string]bool{}
	for _, kv := range services {
		s, err := decodeService(kv.name, kv.raw, m)
		if err != nil {
			return nil, err
		}
		for _, p := range s.Ports {
			owned[p] = true
		}
		m.Services = append(m.Services, s)
	}
	for _, p := range m.Ports {
		if !owned[p.Name] {
			m.Services = append(m.Services, Service{Name: p.Name, Title: p.Name, Ports: []string{p.Name}, Ready: Ready{Timeout: DefaultTimeout}, Implicit: true})
		}
	}

	groups, err := ordered(f.Groups)
	if err != nil {
		return nil, fmt.Errorf("groups: %w", err)
	}
	for _, kv := range groups {
		var list []string
		if err := json.Unmarshal(kv.raw, &list); err != nil {
			return nil, fmt.Errorf("groups.%s: %w", kv.name, err)
		}
		m.Groups = append(m.Groups, Group{Name: kv.name, Services: list})
	}

	commands, err := ordered(f.Commands)
	if err != nil {
		return nil, fmt.Errorf("commands: %w", err)
	}
	for _, kv := range commands {
		c, err := decodeCommand(kv.name, kv.raw)
		if err != nil {
			return nil, err
		}
		m.Commands = append(m.Commands, c)
	}

	for _, l := range f.Links {
		m.Links = append(m.Links, Link{Title: l.Title, URL: l.URL, Needs: l.Needs})
	}
	return m, nil
}

func decodePort(name string, raw json.RawMessage) (Port, error) {
	p := Port{Name: name}
	var n int
	if json.Unmarshal(raw, &n) == nil {
		p.Fixed = n
		return p, nil
	}
	var o struct {
		Fixed int    `json:"fixed"`
		Base  int    `json:"base"`
		Range int    `json:"range"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(raw, &o); err != nil {
		return p, fmt.Errorf("ports.%s: %w", name, err)
	}
	p.Fixed, p.Base, p.Range, p.State = o.Fixed, o.Base, o.Range, o.State
	if p.Base > 0 && p.Range == 0 {
		p.Range = 100
	}
	return p, nil
}

func decodeService(name string, raw json.RawMessage, m *Manifest) (Service, error) {
	var o struct {
		Title   string            `json:"title"`
		Dir     string            `json:"dir"`
		Run     json.RawMessage   `json:"run"`
		Stop    json.RawMessage   `json:"stop"`
		Ports   *[]string         `json:"ports"`
		Needs   needs             `json:"needs"`
		Env     map[string]string `json:"env"`
		LoadEnv bool              `json:"loadEnv"`
		Log     string            `json:"log"`
		Ready   *struct {
			Port    string `json:"port"`
			HTTP    string `json:"http"`
			Timeout string `json:"timeout"`
		} `json:"ready"`
	}
	if err := json.Unmarshal(raw, &o); err != nil {
		return Service{}, fmt.Errorf("services.%s: %w", name, err)
	}
	s := Service{Name: name, Title: o.Title, Dir: o.Dir, Needs: o.Needs, Env: o.Env, LoadEnv: o.LoadEnv, Log: o.Log, Ready: Ready{Timeout: DefaultTimeout}}
	if s.Title == "" {
		s.Title = name
	}
	var err error
	if s.Run, err = decodeRun(o.Run); err != nil {
		return s, fmt.Errorf("services.%s.run: %w", name, err)
	}
	if s.Stop, err = decodeRun(o.Stop); err != nil {
		return s, fmt.Errorf("services.%s.stop: %w", name, err)
	}
	switch {
	case o.Ports != nil:
		s.Ports = *o.Ports
	case m.Port(name) != nil:
		s.Ports = []string{name}
	}
	if r := o.Ready; r != nil {
		s.Ready.Port, s.Ready.HTTP = r.Port, r.HTTP
		if r.Timeout != "" {
			d, err := ParseDuration(r.Timeout)
			if err != nil {
				return s, fmt.Errorf("services.%s.ready.timeout: %w", name, err)
			}
			s.Ready.Timeout = d
		}
	}
	return s, nil
}

// ParseDuration reads a timeout: a whole number with ms, s or m.
func ParseDuration(s string) (time.Duration, error) {
	for _, u := range []struct {
		suffix string
		unit   time.Duration
	}{{"ms", time.Millisecond}, {"s", time.Second}, {"m", time.Minute}} {
		if n, ok := strings.CutSuffix(s, u.suffix); ok {
			v, err := strconv.Atoi(n)
			if err != nil || v < 0 {
				break
			}
			return time.Duration(v) * u.unit, nil
		}
	}
	return 0, fmt.Errorf("%q is not a duration like 90s", s)
}

func decodeCommand(name string, raw json.RawMessage) (Command, error) {
	c := Command{Name: name}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		c.Null = true
		return c, nil
	}
	if isPerServiceRaw(raw) {
		entries, err := ordered(raw)
		if err != nil {
			return c, fmt.Errorf("commands.%s: %w", name, err)
		}
		for _, kv := range entries {
			r, err := decodeRun(kv.raw)
			if err != nil {
				return c, fmt.Errorf("commands.%s.%s: %w", name, kv.name, err)
			}
			c.Entries = append(c.Entries, Entry{Service: kv.name, Run: r})
		}
		return c, nil
	}
	r, err := decodeRun(raw)
	if err != nil {
		return c, fmt.Errorf("commands.%s: %w", name, err)
	}
	c.Run = r
	return c, nil
}

// isPerServiceRaw reports whether a command is an object without run.
func isPerServiceRaw(raw json.RawMessage) bool {
	var o map[string]json.RawMessage
	if json.Unmarshal(raw, &o) != nil {
		return false
	}
	_, hasRun := o["run"]
	return !hasRun
}

// decodeRun reads a string, an argv or an object with run; nil for none.
func decodeRun(raw json.RawMessage) (*Run, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var shell string
	if json.Unmarshal(raw, &shell) == nil {
		return &Run{Shell: shell}, nil
	}
	var argv []string
	if json.Unmarshal(raw, &argv) == nil {
		return &Run{Argv: argv}, nil
	}
	var o struct {
		Run      json.RawMessage   `json:"run"`
		Dir      string            `json:"dir"`
		Env      map[string]string `json:"env"`
		LoadEnv  bool              `json:"loadEnv"`
		Terminal *bool             `json:"terminal"`
		Needs    needs             `json:"needs"`
		Title    string            `json:"title"`
	}
	if err := json.Unmarshal(raw, &o); err != nil {
		return nil, err
	}
	r, err := decodeRun(o.Run)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, errors.New("no run")
	}
	r.Dir, r.Env, r.LoadEnv, r.Terminal, r.Needs, r.Title = o.Dir, o.Env, o.LoadEnv, o.Terminal, o.Needs, o.Title
	return r, nil
}

// needs is a name or a list of them.
type needs []string

func (n *needs) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*n = needs{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return errors.New("needs: want a name or a list of them")
	}
	*n = many
	return nil
}

type keyed struct {
	name string
	raw  json.RawMessage
}

// ordered reads a JSON object's members in the file's order (spec section
// 2.1); nothing for an absent object.
func ordered(raw json.RawMessage) ([]keyed, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, errors.New("want an object")
	}
	var out []keyed
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		name, _ := tok.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		if strings.HasPrefix(name, "x-") || name == "$comment" {
			continue
		}
		out = append(out, keyed{name, v})
	}
	return out, nil
}

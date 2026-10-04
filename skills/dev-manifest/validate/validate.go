package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/FredricW/herdr-deck/schema"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Result holds a manifest's problems. Errors break the schema or a MUST of
// the spec; warnings are dangling references found by the dry check.
type Result struct {
	Errors   []string
	Warnings []string
}

const schemaURL = "https://raw.githubusercontent.com/FredricW/herdr-deck/main/schema/v1/dev.schema.json"

var compiled *jsonschema.Schema

func devSchema() (*jsonschema.Schema, error) {
	if compiled != nil {
		return compiled, nil
	}
	b, err := schema.FS.ReadFile("v1/dev.schema.json")
	if err != nil {
		return nil, err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	if err := c.AddResource(schemaURL, doc); err != nil {
		return nil, err
	}
	s, err := c.Compile(schemaURL)
	if err != nil {
		return nil, err
	}
	compiled = s
	return s, nil
}

// Validate checks the manifest in file. With a non-empty root it also runs
// the dry check against that repository.
func Validate(file, root string) Result {
	b, err := os.ReadFile(file)
	if err != nil {
		return Result{Errors: []string{err.Error()}}
	}
	return ValidateBytes(b, root)
}

// ValidateBytes is Validate on a manifest's content.
func ValidateBytes(b []byte, root string) Result {
	var res Result
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		res.Errors = append(res.Errors, "not JSON: "+err.Error())
		return res
	}
	s, err := devSchema()
	if err != nil {
		res.Errors = append(res.Errors, "schema: "+err.Error())
		return res
	}
	if err := s.Validate(doc); err != nil {
		res.Errors = append(res.Errors, "schema: "+indent(err.Error()))
		// The rules below assume the schema's shapes.
		return res
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		res.Errors = append(res.Errors, "not JSON: "+err.Error())
		return res
	}
	c := newChecker(m)
	c.check()
	res.Errors = c.errs
	if root != "" {
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
		d := dryChecker{checker: c, root: root}
		d.check()
		res.Warnings = d.warns
	}
	return res
}

func indent(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "\n", "\n    ")
}

type checker struct {
	m        map[string]any
	errs     []string
	ports    map[string]map[string]any // nil value: a plain number
	services map[string]map[string]any // declared services
	owned    map[string][]string       // service -> its ports, main first
	implicit map[string]bool           // ports that no service lists
}

func newChecker(m map[string]any) *checker {
	c := &checker{
		m:        m,
		ports:    map[string]map[string]any{},
		services: map[string]map[string]any{},
		owned:    map[string][]string{},
		implicit: map[string]bool{},
	}
	for name, p := range obj(m["ports"]) {
		po, _ := p.(map[string]any)
		c.ports[name] = po
	}
	for name, s := range obj(m["services"]) {
		c.services[name] = obj(s)
	}
	return c
}

func (c *checker) errf(format string, args ...any) {
	c.errs = append(c.errs, fmt.Sprintf(format, args...))
}

func obj(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func sorted[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// names reads a needs value: a name or a list of them.
func names(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case []any:
		var out []string
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func (c *checker) isService(name string) bool {
	_, ok := c.services[name]
	return ok || c.implicit[name]
}

func (c *checker) check() {
	c.checkPorts()
	c.checkServices()
	c.checkGroups()
	c.checkCommands()
	c.checkLinks()
	c.checkTemplates()
}

func (c *checker) checkPorts() {
	usesState := false
	for _, name := range sorted(c.ports) {
		p := c.ports[name]
		if p == nil {
			continue
		}
		if _, ok := p["state"]; ok {
			usesState = true
		}
		if base, ok := p["base"].(float64); ok {
			rng := 100.0
			if r, ok := p["range"].(float64); ok {
				rng = r
			}
			if base+rng-1 > 65535 {
				c.errf("ports.%s: base %v plus range %v goes above 65535", name, base, rng)
			}
		}
	}
	if usesState && c.m["state"] == nil {
		c.errf("ports: a port uses a state key, but there is no state.file")
	}
}

func (c *checker) checkServices() {
	owner := map[string]string{}
	for _, name := range sorted(c.services) {
		s := c.services[name]
		var ports []string
		if list, ok := s["ports"]; ok {
			ports = names(list)
		} else if _, ok := c.ports[name]; ok {
			ports = []string{name}
		}
		for _, p := range ports {
			if _, ok := c.ports[p]; !ok {
				c.errf("services.%s.ports: no port %q in ports", name, p)
				continue
			}
			if o, ok := owner[p]; ok {
				c.errf("services.%s.ports: port %q already belongs to service %q", name, p, o)
				continue
			}
			owner[p] = name
		}
		c.owned[name] = ports
	}
	for _, p := range sorted(c.ports) {
		o, ok := owner[p]
		if !ok {
			if _, isSvc := c.services[p]; isSvc {
				c.errf("ports.%s: has service %q's name, so it must be one of that service's ports", p, p)
			}
			c.implicit[p] = true
			continue
		}
		if _, isSvc := c.services[p]; isSvc && o != p {
			c.errf("ports.%s: has service %q's name but belongs to service %q", p, p, o)
		}
	}

	for _, name := range sorted(c.services) {
		s := c.services[name]
		for _, n := range names(s["needs"]) {
			if !c.isService(n) {
				c.errf("services.%s.needs: %q is not a service or a port", name, n)
			}
			if n == name {
				c.errf("services.%s.needs: a service cannot need itself", name)
			}
		}
		if r := obj(s["ready"]); r != nil {
			if p, ok := r["port"].(string); ok && !contains(c.owned[name], p) {
				c.errf("services.%s.ready.port: %q is not one of the service's ports", name, p)
			}
			if _, ok := r["http"]; ok && len(c.owned[name]) == 0 {
				c.errf("services.%s.ready.http: the service has no port to ask", name)
			}
		}
	}
	if cycle := c.findCycle(); cycle != nil {
		c.errf("services: needs form a cycle: %s", strings.Join(cycle, " -> "))
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// findCycle returns one cycle in the services' needs, or nil.
func (c *checker) findCycle() []string {
	const (
		unseen = iota
		active
		done
	)
	state := map[string]int{}
	var stack []string
	var visit func(string) []string
	visit = func(n string) []string {
		switch state[n] {
		case active:
			for i, s := range stack {
				if s == n {
					return append(append([]string{}, stack[i:]...), n)
				}
			}
		case done:
			return nil
		}
		state[n] = active
		stack = append(stack, n)
		for _, d := range names(c.services[n]["needs"]) {
			if _, ok := c.services[d]; !ok || d == n {
				continue
			}
			if cyc := visit(d); cyc != nil {
				return cyc
			}
		}
		stack = stack[:len(stack)-1]
		state[n] = done
		return nil
	}
	for _, n := range sorted(c.services) {
		if cyc := visit(n); cyc != nil {
			return cyc
		}
	}
	return nil
}

func (c *checker) checkGroups() {
	groups := obj(c.m["groups"])
	for _, g := range sorted(groups) {
		for _, n := range names(groups[g]) {
			if !c.isService(n) {
				c.errf("groups.%s: %q is not a service", g, n)
			}
		}
	}
}

// isPerService reports whether a command value is a per-service map.
func isPerService(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	_, hasRun := m["run"]
	return !hasRun
}

func (c *checker) checkCommands() {
	cmds := obj(c.m["commands"])
	for _, name := range sorted(cmds) {
		v := cmds[name]
		if isPerService(v) {
			entries := obj(v)
			for _, key := range sorted(entries) {
				if key != "*" && !c.isService(key) {
					c.errf("commands.%s.%s: %q is not a service", name, key, key)
				}
				c.checkCommandNeeds("commands."+name+"."+key, entries[key])
			}
			continue
		}
		c.checkCommandNeeds("commands."+name, v)
	}
}

func (c *checker) checkCommandNeeds(path string, v any) {
	for _, n := range names(obj(v)["needs"]) {
		if !c.isService(n) {
			c.errf("%s.needs: %q is not a service or a port", path, n)
		}
	}
}

func (c *checker) checkLinks() {
	links, _ := c.m["links"].([]any)
	for i, l := range links {
		for _, n := range names(obj(l)["needs"]) {
			if _, isPort := c.ports[n]; !isPort && !c.isService(n) {
				c.errf("links[%d].needs: %q is not a service or a port", i, n)
			}
		}
	}
}

// checkTemplates checks every template string's variables (spec section 3).
func (c *checker) checkTemplates() {
	t := func(path string, v any) {
		if s, ok := v.(string); ok {
			for _, p := range templateProblems(s, c.ports) {
				c.errf("%s: %s", path, p)
			}
		}
	}
	envMap := func(path string, v any) {
		m := obj(v)
		for _, k := range sorted(m) {
			t(path+"."+k, m[k])
		}
	}
	list := func(path string, v any) {
		l, _ := v.([]any)
		for i, x := range l {
			t(fmt.Sprintf("%s[%d]", path, i), x)
		}
	}
	// run reads a string, an argv or an object with run, dir and env.
	run := func(path string, v any) {
		switch v := v.(type) {
		case string:
			t(path, v)
		case []any:
			list(path, v)
		case map[string]any:
			if s, ok := v["run"].(string); ok {
				t(path+".run", s)
			} else {
				list(path+".run", v["run"])
			}
			t(path+".dir", v["dir"])
			envMap(path+".env", v["env"])
		}
	}

	env := obj(c.m["env"])
	list("env.files", env["files"])
	envMap("env.vars", env["vars"])
	list("path", c.m["path"])
	t("state.file", obj(c.m["state"])["file"])
	for _, name := range sorted(c.services) {
		s := c.services[name]
		p := "services." + name
		t(p+".dir", s["dir"])
		t(p+".log", s["log"])
		envMap(p+".env", s["env"])
		run(p+".run", s["run"])
		run(p+".stop", s["stop"])
		t(p+".ready.http", obj(s["ready"])["http"])
	}
	cmds := obj(c.m["commands"])
	for _, name := range sorted(cmds) {
		v := cmds[name]
		if isPerService(v) {
			entries := obj(v)
			for _, k := range sorted(entries) {
				run("commands."+name+"."+k, entries[k])
			}
			continue
		}
		run("commands."+name, v)
	}
	links, _ := c.m["links"].([]any)
	for i, l := range links {
		t(fmt.Sprintf("links[%d].url", i), obj(l)["url"])
		t(fmt.Sprintf("links[%d].title", i), obj(l)["title"])
	}
}

var (
	varNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*`)
	envNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// templateProblems lists the template's variables that the spec does not
// define (section 3).
func templateProblems(s string, ports map[string]map[string]any) []string {
	var out []string
	known := func(name string) {
		switch name {
		case "WORKTREE", "REPO", "DIRNAME", "BRANCH":
			return
		}
		if p, ok := strings.CutPrefix(name, "PORT_"); ok {
			if _, ok := ports[p]; ok {
				return
			}
			out = append(out, fmt.Sprintf("$%s: no port %q in ports", name, p))
			return
		}
		out = append(out, fmt.Sprintf("unknown variable $%s (write $$%s for the shell to see it)", name, name))
	}
	for i := 0; i < len(s); {
		if s[i] != '$' {
			i++
			continue
		}
		rest := s[i+1:]
		switch {
		case strings.HasPrefix(rest, "$"):
			i += 2
		case strings.HasPrefix(rest, "env("):
			end := strings.IndexByte(rest, ')')
			if end < 0 {
				out = append(out, "$env( without a closing )")
				return out
			}
			name, _, _ := strings.Cut(rest[len("env("):end], "|")
			if !envNameRe.MatchString(name) {
				out = append(out, fmt.Sprintf("$env(%s): not a variable name", name))
			}
			i += 1 + end + 1
		case strings.HasPrefix(rest, "{"):
			end := strings.IndexByte(rest, '}')
			if end < 0 {
				out = append(out, "${ without a closing }")
				return out
			}
			known(rest[1:end])
			i += 1 + end + 1
		default:
			name := varNameRe.FindString(rest)
			if name == "" {
				out = append(out, "a lone $ (write $$ for a literal $)")
				i++
				continue
			}
			known(name)
			i += 1 + len(name)
		}
	}
	return out
}

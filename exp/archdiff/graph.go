package main

import (
	"cmp"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// module is one go.mod in the tree.
type module struct {
	dir, path string
	requires  map[string]string // module path -> version
}

var (
	modLine = regexp.MustCompile(`(?m)^module\s+(\S+)`)
	reqLine = regexp.MustCompile(`(?m)^\s*(?:require\s+)?([a-zA-Z0-9][^\s()]*\.[^\s()]+)\s+(v\S+)`)
)

func parseMod(dir string, src []byte) module {
	m := module{dir: dir, requires: map[string]string{}}
	if s := modLine.FindSubmatch(src); s != nil {
		m.path = string(s[1])
	}
	for _, s := range reqLine.FindAllSubmatch(src, -1) {
		if string(s[1]) != m.path {
			m.requires[string(s[1])] = string(s[2])
		}
	}
	return m
}

// site is where an import or touchpoint is written.
type site struct {
	file string
	line int
	uses string
}

// edgeKey is a dependency between two nodes. A node is a package directory
// relative to the repository root ("." for the root), "ext:<module>" for a
// third-party module or "std:<path>" for the standard library.
type edgeKey struct{ from, to string }

// snapshot is the package graph of one commit.
type snapshot struct {
	rev     string
	modules []module
	files   map[string]*goFile  // path -> parsed file
	pkgs    map[string][]string // dir -> its files
	edges   map[edgeKey][]site
}

// cache maps a blob SHA to its parsed file; it is what makes the second
// commit cheap.
type cache map[string]*goFile

func skipPath(p string, tests bool) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == "testdata" || seg == "vendor" || strings.HasPrefix(seg, "_") || (strings.HasPrefix(seg, ".") && seg != ".") {
			return true
		}
	}
	return !tests && strings.HasSuffix(p, "_test.go")
}

// lang picks the reader: "go", or "ts" for TypeScript and JavaScript.
// src limits a TypeScript graph to the files under one folder.
type lang struct {
	name, src string
}

func build(repo, rev string, bl *blobs, c cache, tests bool, ln lang) (*snapshot, error) {
	es, err := lsTree(repo, rev)
	if err != nil {
		return nil, err
	}
	if ln.name == "ts" {
		return buildTS(rev, es, bl, c, tests, ln.src)
	}
	s := &snapshot{rev: rev, files: map[string]*goFile{}, pkgs: map[string][]string{}, edges: map[edgeKey][]site{}}
	for _, e := range es {
		if path.Base(e.path) != "go.mod" || skipPath(e.path, true) {
			continue
		}
		src, err := bl.read(e.blob)
		if err != nil {
			return nil, err
		}
		s.modules = append(s.modules, parseMod(path.Dir(e.path), src))
	}
	for _, e := range es {
		if !strings.HasSuffix(e.path, ".go") || skipPath(e.path, tests) {
			continue
		}
		g, ok := c[e.blob]
		if !ok {
			src, err := bl.read(e.blob)
			if err != nil {
				return nil, err
			}
			g = parseGo(src)
			c[e.blob] = g
		}
		if g.ignored || g.pkg == "main" && strings.HasSuffix(e.path, "_test.go") {
			continue
		}
		s.files[e.path] = g
		dir := path.Dir(e.path)
		s.pkgs[dir] = append(s.pkgs[dir], e.path)
	}
	for file, g := range s.files {
		from := path.Dir(file)
		for _, im := range g.imports {
			to := s.resolve(from, im.path)
			if to == from {
				continue
			}
			k := edgeKey{from, to}
			s.edges[k] = append(s.edges[k], site{file, im.line, im.uses})
		}
	}
	return s, nil
}

// buildTS is build for TypeScript: the tsconfig files give the aliases,
// the package.json files the dependencies, and each import resolves to
// the directory of the file it names.
func buildTS(rev string, es []entry, bl *blobs, c cache, tests bool, src string) (*snapshot, error) {
	s := &snapshot{rev: rev, files: map[string]*goFile{}, pkgs: map[string][]string{}, edges: map[edgeKey][]site{}}
	res := &tsResolver{files: map[string]bool{}}
	for _, e := range es {
		res.files[e.path] = true
		base := path.Base(e.path)
		isConfig := strings.HasPrefix(base, "tsconfig") && strings.HasSuffix(base, ".json")
		if strings.Contains(e.path, "node_modules/") || (!isConfig && base != "package.json") {
			continue
		}
		data, err := bl.read(e.blob)
		if err != nil {
			return nil, err
		}
		if base == "package.json" {
			s.modules = append(s.modules, parsePackageJSON(path.Dir(e.path), data))
		} else if cfg := parseTSConfig(path.Dir(e.path), data); cfg != nil {
			res.configs = append(res.configs, cfg)
		}
	}
	under := func(p string) bool { return src == "" || p == src || strings.HasPrefix(p, src+"/") }
	for _, e := range es {
		if !isTSFile(e.path) || !under(e.path) || skipTS(e.path, tests) {
			continue
		}
		g, ok := c[e.blob]
		if !ok {
			data, err := bl.read(e.blob)
			if err != nil {
				return nil, err
			}
			g = parseTS(data)
			c[e.blob] = g
		}
		s.files[e.path] = g
		dir := path.Dir(e.path)
		s.pkgs[dir] = append(s.pkgs[dir], e.path)
	}
	for file, g := range s.files {
		from := path.Dir(file)
		for _, im := range g.imports {
			to := res.resolve(file, im.path)
			if to == "" || to == from || (internal(to) && !under(to)) {
				continue
			}
			k := edgeKey{from, to}
			s.edges[k] = append(s.edges[k], site{file, im.line, im.uses})
		}
	}
	return s, nil
}

// moduleOf is the go.mod nearest above a directory.
func (s *snapshot) moduleOf(dir string) *module {
	var best *module
	for i := range s.modules {
		m := &s.modules[i]
		if m.dir == "." || dir == m.dir || strings.HasPrefix(dir, m.dir+"/") {
			if best == nil || len(m.dir) > len(best.dir) || best.dir == "." {
				best = m
			}
		}
	}
	return best
}

// resolve turns an import path into a node.
func (s *snapshot) resolve(fromDir, imp string) string {
	var best *module
	for i := range s.modules {
		m := &s.modules[i]
		if m.path != "" && (imp == m.path || strings.HasPrefix(imp, m.path+"/")) {
			if best == nil || len(m.path) > len(best.path) {
				best = m
			}
		}
	}
	if best != nil {
		return path.Join(best.dir, strings.TrimPrefix(strings.TrimPrefix(imp, best.path), "/"))
	}
	first, _, _ := strings.Cut(imp, "/")
	if !strings.Contains(first, ".") {
		return "std:" + imp
	}
	if m := s.moduleOf(fromDir); m != nil {
		dep := ""
		for r := range m.requires {
			if (imp == r || strings.HasPrefix(imp, r+"/")) && len(r) > len(dep) {
				dep = r
			}
		}
		if dep != "" {
			return "ext:" + dep
		}
	}
	return "ext:" + imp
}

func internal(n string) bool { return !strings.HasPrefix(n, "ext:") && !strings.HasPrefix(n, "std:") }

// ---- layers

// layerConfig is how a repository declares its lanes, top (entry points)
// to bottom (the core). A package may import its own lane and any lane
// below; importing upwards is a violation. Skipping over a closed lane is
// a warning: callers above it are meant to go through it.
type layerConfig struct {
	Layers []struct {
		Name   string   `json:"name"`
		Paths  []string `json:"paths"`
		Closed bool     `json:"closed,omitempty"`
	} `json:"layers"`
}

// lanes assigns each internal node a lane. Without a config the lanes are
// inferred: a package's depth is the longest import chain below it, and
// deeper packages sit higher.
type lanes struct {
	names    []string
	closed   []bool
	of       map[string]int
	inferred bool // from the graph, not a config: nothing can violate them
}

func globMatch(pat, dir string) bool {
	if rest, ok := strings.CutSuffix(pat, "/**"); ok {
		return dir == rest || strings.HasPrefix(dir, rest+"/")
	}
	ok, _ := path.Match(pat, dir)
	return ok
}

func configLanes(cfg *layerConfig, nodes []string) lanes {
	l := lanes{of: map[string]int{}}
	for _, ly := range cfg.Layers {
		l.names = append(l.names, ly.Name)
		l.closed = append(l.closed, ly.Closed)
	}
	other := -1
	for _, n := range nodes {
		lane := -1
		for i, ly := range cfg.Layers {
			if slices.ContainsFunc(ly.Paths, func(p string) bool { return globMatch(p, n) }) {
				lane = i
				break
			}
		}
		if lane < 0 {
			if other < 0 {
				l.names = append(l.names, "unassigned")
				l.closed = append(l.closed, false)
				other = len(l.names) - 1
			}
			lane = other
		}
		l.of[n] = lane
	}
	return l
}

func inferredLanes(edges map[edgeKey][]site, nodes []string) lanes {
	out := map[string][]string{}
	for k := range edges {
		if internal(k.to) {
			out[k.from] = append(out[k.from], k.to)
		}
	}
	depth := map[string]int{}
	var walk func(n string, seen map[string]bool) int
	walk = func(n string, seen map[string]bool) int {
		if d, ok := depth[n]; ok {
			return d
		}
		if seen[n] {
			return 0 // a cycle (not in compiling Go)
		}
		seen[n] = true
		d := 0
		for _, m := range out[n] {
			d = max(d, walk(m, seen)+1)
		}
		depth[n] = d
		return d
	}
	top := 0
	for _, n := range nodes {
		top = max(top, walk(n, map[string]bool{}))
	}
	l := lanes{of: map[string]int{}, inferred: true}
	for d := top; d >= 0; d-- {
		l.names = append(l.names, "depth "+strconv.Itoa(d))
		l.closed = append(l.closed, false)
	}
	for _, n := range nodes {
		l.of[n] = top - depth[n]
	}
	return l
}

// verdict on one internal edge: "" fine, "skip" over a closed lane, "up"
// against the layering.
func (l lanes) check(e edgeKey) (string, string) {
	a, b := l.of[e.from], l.of[e.to]
	if l.inferred {
		return "", "" // inferred lanes order the view; with cycles (TS) they would flag noise
	}
	if l.names[a] == "unassigned" || l.names[b] == "unassigned" {
		return "", ""
	}
	if b < a {
		return "up", l.names[a] + " → " + l.names[b]
	}
	for i := a + 1; i < b; i++ {
		if l.closed[i] {
			return "skip", "skips " + l.names[i]
		}
	}
	return "", ""
}

// ---- components and cycles

// component is the coarser zoom level: internal/source/dev/manifest ->
// internal/source, cmd/herdr-deck -> cmd. Go forbids cycles between
// packages, so a cycle can only show at this level (or, in TypeScript and
// Python, at any level).
func component(n string) string {
	parts := strings.Split(n, "/")
	if parts[0] == "internal" && len(parts) > 1 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

func compEdges(edges map[edgeKey][]site) map[edgeKey]bool {
	ce := map[edgeKey]bool{}
	for k := range edges {
		if !internal(k.to) {
			continue
		}
		a, b := component(k.from), component(k.to)
		if a != b {
			ce[edgeKey{a, b}] = true
		}
	}
	return ce
}

// pathBetween is the shortest component path from a to b, or nil.
func pathBetween(ce map[edgeKey]bool, a, b string) []string {
	next := map[string][]string{}
	for k := range ce {
		next[k.from] = append(next[k.from], k.to)
	}
	for _, v := range next {
		slices.Sort(v)
	}
	prev := map[string]string{a: ""}
	q := []string{a}
	for len(q) > 0 {
		n := q[0]
		q = q[1:]
		if n == b {
			p := []string{b}
			for p[0] != a {
				p = append([]string{prev[p[0]]}, p...)
			}
			return p
		}
		for _, m := range next[n] {
			if _, ok := prev[m]; !ok {
				prev[m] = n
				q = append(q, m)
			}
		}
	}
	return nil
}

// newCycles lists component cycles that the head has and the base has
// not, each as a path that starts and ends at the same component and
// goes through one new component edge.
func newCycles(base, head map[edgeKey]bool) [][]string {
	var out [][]string
	seen := map[string]bool{}
	for _, k := range sortedKeys(head) {
		if base[k] {
			continue
		}
		back := pathBetween(head, k.to, k.from)
		if back == nil {
			continue
		}
		cyc := append([]string{k.from}, back...)
		id := cycleID(cyc)
		if !seen[id] {
			seen[id] = true
			out = append(out, cyc)
		}
	}
	return out
}

// cycleID names a cycle the same way whichever node it starts at.
func cycleID(c []string) string {
	ring := c[:len(c)-1]
	lo := 0
	for i, n := range ring {
		if n < ring[lo] {
			lo = i
		}
	}
	rot := append(slices.Clone(ring[lo:]), ring[:lo]...)
	return strings.Join(append(rot, rot[0]), "→")
}

func sortedKeys(m map[edgeKey]bool) []edgeKey {
	ks := slices.Collect(maps.Keys(m))
	slices.SortFunc(ks, func(a, b edgeKey) int {
		return cmp.Or(cmp.Compare(a.from, b.from), cmp.Compare(a.to, b.to))
	})
	return ks
}

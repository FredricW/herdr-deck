package arch

import (
	"cmp"
	"errors"
	"iter"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// module is one go.mod or package.json in the tree.
type module struct {
	dir, path string
	requires  map[string]string // module or package -> version
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

// edgeKey is a dependency between two nodes. A node is a package directory
// relative to the repository root ("." for the root), "ext:<module>" for a
// third-party module or package, or "std:<path>" for the standard library.
type edgeKey struct{ from, to string }

// snapshot is the package graph of one commit.
type snapshot struct {
	modules []module
	files   map[string]*facts   // path -> parsed file
	pkgs    map[string][]string // dir -> its files
	edges   map[edgeKey][]Site
}

func newSnapshot() *snapshot {
	return &snapshot{files: map[string]*facts{}, pkgs: map[string][]string{}, edges: map[edgeKey][]Site{}}
}

// factCache maps a language and blob SHA to the file's facts; it is what
// makes the second commit, and every later read, cheap.
type factCache struct {
	mu sync.Mutex
	m  map[string]*facts
}

// maxFacts bounds the cache; past it, it starts over.
const maxFacts = 50000

func (c *factCache) get(lang, blob string) (*facts, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f, ok := c.m[lang+":"+blob]
	return f, ok
}

func (c *factCache) put(lang, blob string, f *facts) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil || len(c.m) >= maxFacts {
		c.m = map[string]*facts{}
	}
	c.m[lang+":"+blob] = f
}

// counts is how many files a build parsed and how many it took from the
// cache.
type counts struct{ parsed, reused int }

// facts reads and parses a blob, or takes it from the cache.
func (c *factCache) facts(lang string, e entry, bl blobReader, n *counts) (*facts, error) {
	if f, ok := c.get(lang, e.blob); ok {
		n.reused++
		return f, nil
	}
	src, err := bl.read(e.blob)
	if errors.Is(err, errTooBig) {
		f := &facts{ignored: true}
		c.put(lang, e.blob, f)
		return f, nil
	}
	if err != nil {
		return nil, err
	}
	f := parseTS(src)
	if lang == LangGo {
		f = parseGo(src)
	}
	c.put(lang, e.blob, f)
	n.parsed++
	return f, nil
}

// skipGo leaves out test data, vendored code, hidden and underscore
// folders, and tests unless asked for.
func skipGo(p string, tests bool) bool {
	for seg := range strings.SplitSeq(p, "/") {
		if seg == "testdata" || seg == "vendor" || strings.HasPrefix(seg, "_") || (strings.HasPrefix(seg, ".") && seg != ".") {
			return true
		}
	}
	return !tests && strings.HasSuffix(p, "_test.go")
}

// under says whether p lies in one of roots; no roots take every path.
func under(p string, roots []string) bool {
	if len(roots) == 0 {
		return true
	}
	for _, r := range roots {
		if r == "." || p == r || strings.HasPrefix(p, r+"/") {
			return true
		}
	}
	return false
}

// buildGo is one commit's Go package graph.
func buildGo(es []entry, bl blobReader, c *factCache, cfg Config, n *counts) (*snapshot, error) {
	s := newSnapshot()
	for _, e := range es {
		if path.Base(e.path) != "go.mod" || skipGo(e.path, true) {
			continue
		}
		src, err := bl.read(e.blob)
		if err != nil {
			return nil, err
		}
		s.modules = append(s.modules, parseMod(path.Dir(e.path), src))
	}
	for _, e := range es {
		if !strings.HasSuffix(e.path, ".go") || skipGo(e.path, cfg.Tests) || !under(e.path, cfg.Roots) {
			continue
		}
		g, err := c.facts(LangGo, e, bl, n)
		if err != nil {
			return nil, err
		}
		if g.ignored || g.pkg == "main" && strings.HasSuffix(e.path, "_test.go") {
			continue
		}
		s.files[e.path] = g
		dir := path.Dir(e.path)
		s.pkgs[dir] = append(s.pkgs[dir], e.path)
	}
	for _, file := range slices.Sorted(maps.Keys(s.files)) {
		from := path.Dir(file)
		for _, im := range s.files[file].imports {
			to := s.resolveGo(from, im.path)
			if to == from || (internal(to) && !under(to, cfg.Roots)) {
				continue
			}
			k := edgeKey{from, to}
			s.edges[k] = append(s.edges[k], Site{File: file, Line: im.line, Uses: im.uses})
		}
	}
	return s, nil
}

// buildTS is buildGo for TypeScript: tsconfig files give the aliases,
// package.json files the dependencies and workspace packages, and each
// import resolves to the directory of the file it names.
func buildTS(es []entry, bl blobReader, c *factCache, cfg Config, n *counts) (*snapshot, error) {
	s := newSnapshot()
	res := &tsResolver{files: map[string]bool{}, workspaces: map[string]string{}}
	for _, e := range es {
		res.files[e.path] = true
		base := path.Base(e.path)
		isConfig := strings.HasPrefix(base, "tsconfig") && strings.HasSuffix(base, ".json")
		if strings.Contains(e.path, "node_modules/") || (!isConfig && base != "package.json") {
			continue
		}
		data, err := bl.read(e.blob)
		if errors.Is(err, errTooBig) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if base == "package.json" {
			m := parsePackageJSON(path.Dir(e.path), data)
			s.modules = append(s.modules, m)
			if m.path != "" && m.dir != "." {
				res.workspaces[m.path] = m.dir
			}
		} else if tc := parseTSConfig(path.Dir(e.path), data); tc != nil {
			res.configs = append(res.configs, tc)
		}
	}
	slices.SortFunc(res.configs, func(a, b *tsconfig) int { return cmp.Compare(a.dir, b.dir) })
	for _, e := range es {
		if !isTSFile(e.path) || !under(e.path, cfg.Roots) || skipTS(e.path, cfg.Tests) {
			continue
		}
		g, err := c.facts(LangTS, e, bl, n)
		if err != nil {
			return nil, err
		}
		if g.ignored {
			continue
		}
		s.files[e.path] = g
		dir := path.Dir(e.path)
		s.pkgs[dir] = append(s.pkgs[dir], e.path)
	}
	for _, file := range slices.Sorted(maps.Keys(s.files)) {
		from := path.Dir(file)
		for _, im := range s.files[file].imports {
			to := res.resolve(file, im.path)
			if to == "" || to == from || (internal(to) && !under(to, cfg.Roots)) {
				continue
			}
			k := edgeKey{from, to}
			s.edges[k] = append(s.edges[k], Site{File: file, Line: im.line, Uses: im.uses})
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
			if best == nil || best.dir == "." || len(m.dir) > len(best.dir) {
				best = m
			}
		}
	}
	return best
}

// resolveGo turns an import path into a node.
func (s *snapshot) resolveGo(fromDir, imp string) string {
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

// internal says whether a node is a package of the repository.
func internal(n string) bool { return !strings.HasPrefix(n, "ext:") && !strings.HasPrefix(n, "std:") }

// ---- lanes

// lanes assigns each internal node a lane, top (entry points) to bottom
// (the core). Without layers in the config they are inferred: a package's
// depth is the longest import chain below it, and deeper packages sit
// higher.
type lanes struct {
	names    []string
	closed   []bool
	of       map[string]int
	inferred bool // from the graph, not a config: nothing can violate them
}

// globMatch matches a package directory against a layer's path: "dir/**"
// takes dir and everything under it; other patterns are path.Match's.
func globMatch(pat, dir string) bool {
	pat = strings.TrimSuffix(pat, "/")
	if rest, ok := strings.CutSuffix(pat, "/**"); ok {
		return dir == rest || strings.HasPrefix(dir, rest+"/")
	}
	if ok, _ := path.Match(pat, dir); ok {
		return true
	}
	// A file named as a layer's path puts its folder there.
	return path.Ext(pat) != "" && path.Dir(pat) == dir
}

func configLanes(layers []Layer, nodes []string) lanes {
	l := lanes{of: map[string]int{}}
	for _, ly := range layers {
		l.names = append(l.names, ly.Name)
		l.closed = append(l.closed, ly.Closed)
	}
	other := -1
	for _, n := range nodes {
		lane := -1
		for i, ly := range layers {
			if slices.ContainsFunc(ly.Paths, func(p string) bool { return globMatch(p, n) }) {
				lane = i
				break
			}
		}
		if lane < 0 {
			if other < 0 {
				l.names = append(l.names, unassigned)
				l.closed = append(l.closed, false)
				other = len(l.names) - 1
			}
			lane = other
		}
		l.of[n] = lane
	}
	return l
}

// unassigned is the lane of packages no layer names; it never produces a
// violation.
const unassigned = "unassigned"

func inferredLanes(edges map[edgeKey][]Site, nodes []string) lanes {
	out := map[string][]string{}
	for _, k := range sortedEdges(edges) {
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
			return 0 // a cycle (TypeScript; not compiling Go)
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

// check is the verdict on one internal edge, and why: an upward edge goes
// against the layering, a skip jumps over a closed lane.
func (l lanes) check(e edgeKey) (Verdict, string) {
	if l.inferred {
		return VerdictNone, "" // inferred lanes order the view; with cycles they would flag noise
	}
	a, aok := l.of[e.from]
	b, bok := l.of[e.to]
	if !aok || !bok || l.names[a] == unassigned || l.names[b] == unassigned {
		return VerdictNone, ""
	}
	if b < a {
		return VerdictUp, "upward: " + l.names[a] + " → " + l.names[b]
	}
	for i := a + 1; i < b; i++ {
		if l.closed[i] {
			return VerdictSkip, "skips " + l.names[i]
		}
	}
	return VerdictNone, ""
}

// ---- components and cycles

// component is the coarser zoom level: internal/source/dev/manifest ->
// internal/source, cmd/herdr-deck -> cmd. Go forbids cycles between
// packages, so in Go a cycle can only show at this level.
func component(n string) string {
	parts := strings.Split(n, "/")
	if (parts[0] == "internal" || parts[0] == "src" || parts[0] == "apps" || parts[0] == "packages") && len(parts) > 1 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

func compEdges(edges map[edgeKey][]Site) map[edgeKey]bool {
	ce := map[edgeKey]bool{}
	for k := range edges {
		if !internal(k.to) {
			continue
		}
		if a, b := component(k.from), component(k.to); a != b {
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

// newCycles lists the component cycles that the head has and the base has
// not, each as a path that starts and ends at the same component and goes
// through one new component edge.
func newCycles(base, head map[edgeKey]bool) [][]string {
	var out [][]string
	seen := map[string]bool{}
	for _, k := range slices.SortedFunc(maps.Keys(head), cmpEdge) {
		if base[k] {
			continue
		}
		back := pathBetween(head, k.to, k.from)
		if back == nil {
			continue
		}
		cyc := append([]string{k.from}, back...)
		if id := cycleID(cyc); !seen[id] {
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

func cmpEdge(a, b edgeKey) int {
	return cmp.Or(cmp.Compare(a.from, b.from), cmp.Compare(a.to, b.to))
}

func sortedEdges[V any](m map[edgeKey]V) []edgeKey {
	return slices.SortedFunc(maps.Keys(m), cmpEdge)
}

func mapKeys[V any](m map[string]V) iter.Seq[string] { return maps.Keys(m) }

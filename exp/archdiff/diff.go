package main

import (
	"cmp"
	"path"
	"slices"
	"strings"
)

// edgeChange is one dependency that the change adds or removes.
type edgeChange struct {
	edgeKey
	added   bool
	sites   []site // where the import is written (head for added, base for removed)
	verdict string // "", "skip", "up"; "same" for an unchanged edge drawn as context
	changed bool   // in both base and head, but what its importers use through it changed
	why     string
	risk    int
}

// surface is one package's exported API change.
type surface struct {
	added, removed, changed []string
}

func (s surface) empty() bool { return len(s.added)+len(s.removed)+len(s.changed) == 0 }

// weight is one package's share of the diff.
type weight struct {
	files, add, del int
}

// touchChange is a touchpoint the change adds or removes.
type touchChange struct {
	pkg   string
	touch touch
	file  string
	added bool
	risk  int
}

// depChange is a go.mod requirement the change adds, removes or bumps.
type depChange struct {
	mod, from, to string
}

// moves sums up how much of the change is a move.
type moves struct {
	files      int // renamed with no content change (R100)
	decls      int // declarations whose text moved to another file or package unchanged
	same       int // declarations in changed files whose text did not change
	changed    int // declarations whose text changed in place
	added      int
	removed    int
	renamed    int // moved or renamed with the same body
	movedDecls []moved
}

// moved is one declaration that moved (and maybe was renamed) unchanged.
type moved struct {
	from, to       string // packages
	oldKey, newKey string
}

type result struct {
	base, head    *snapshot
	lanes         lanes
	pkgsAdded     []string
	pkgsRemoved   []string
	changedPkgs   map[string]bool
	weights       map[string]weight
	edges         []edgeChange
	changedEdges  []edgeChange // internal edges in both, whose fingerprint changed
	cycles        [][]string
	touches       []touchChange
	deps          []depChange
	surfaces      map[string]surface
	moves         moves
	unchangedPkgs int
}

func analyse(base, head *snapshot, cs []change, cfg *layerConfig) *result {
	r := &result{base: base, head: head, changedPkgs: map[string]bool{}, weights: map[string]weight{}, surfaces: map[string]surface{}}

	// Nodes: every internal package of either side.
	all := map[string]bool{}
	for d := range base.pkgs {
		all[d] = true
	}
	for d := range head.pkgs {
		all[d] = true
	}
	nodes := slices.Sorted(func(yield func(string) bool) {
		for n := range all {
			if !yield(n) {
				return
			}
		}
	})
	for _, n := range nodes {
		_, inBase := base.pkgs[n]
		_, inHead := head.pkgs[n]
		switch {
		case !inBase:
			r.pkgsAdded = append(r.pkgsAdded, n)
		case !inHead:
			r.pkgsRemoved = append(r.pkgsRemoved, n)
		}
	}

	// Lanes are computed on the head's graph, plus the base's for removed nodes.
	union := map[edgeKey][]site{}
	for k, v := range base.edges {
		union[k] = v
	}
	for k, v := range head.edges {
		union[k] = v
	}
	if cfg != nil && len(cfg.Layers) > 0 {
		r.lanes = configLanes(cfg, nodes)
	} else {
		r.lanes = inferredLanes(union, nodes)
	}

	// Weights from the changed source files the graph has read.
	for _, c := range cs {
		for _, p := range []string{c.old, c.new} {
			if p == "" {
				continue
			}
			if _, ok := base.files[p]; !ok {
				if _, ok := head.files[p]; !ok {
					continue // test files, testdata, ignored
				}
			}
			d := path.Dir(p)
			r.changedPkgs[d] = true
			if p == c.new || c.new == "" {
				w := r.weights[d]
				w.files++
				w.add += c.add
				w.del += c.del
				r.weights[d] = w
			}
		}
	}

	// Edges.
	for k, s := range head.edges {
		if _, ok := base.edges[k]; !ok && (!strings.HasPrefix(k.to, "std:") || stdTouch(k.to)) {
			r.edges = append(r.edges, edgeChange{edgeKey: k, added: true, sites: s})
		}
	}
	for k, s := range base.edges {
		if _, ok := head.edges[k]; !ok && (!strings.HasPrefix(k.to, "std:") || stdTouch(k.to)) {
			r.edges = append(r.edges, edgeChange{edgeKey: k, added: false, sites: s})
		}
	}
	for i := range r.edges {
		e := &r.edges[i]
		switch {
		case internal(e.to):
			e.verdict, e.why = r.lanes.check(e.edgeKey)
			e.risk = 30
			switch e.verdict {
			case "up":
				e.risk = 100
			case "skip":
				e.risk = 60
			}
		case strings.HasPrefix(e.to, "ext:"):
			e.risk = 45
			e.why = "third-party"
		default:
			e.risk = 40
			e.why = stdTouchKind(e.to)
		}
		if !e.added {
			e.risk /= 3
		}
	}
	slices.SortFunc(r.edges, func(a, b edgeChange) int {
		return cmp.Or(cmp.Compare(b.risk, a.risk), cmp.Compare(a.from, b.from), cmp.Compare(a.to, b.to))
	})

	r.cycles = newCycles(compEdges(base.edges), compEdges(head.edges))
	for _, k := range sortedKeys(boolKeys(head.edges)) {
		b, ok := base.edges[k]
		if ok && internal(k.to) && fingerprint(b) != fingerprint(head.edges[k]) {
			e := edgeChange{edgeKey: k, added: true, changed: true, sites: head.edges[k], risk: 15}
			e.verdict, e.why = r.lanes.check(k)
			r.changedEdges = append(r.changedEdges, e)
		}
	}

	// Declarations and touchpoints of the changed files, both sides.
	movedFrom := map[keyed]bool{}
	bd, hd := map[keyed]decl{}, map[keyed]decl{}
	bt, ht := map[string]int{}, map[string]int{}
	htSite := map[string]site{}
	for _, c := range cs {
		if g := base.files[c.old]; c.old != "" && g != nil {
			for _, d := range g.decls {
				bd[keyed{path.Dir(c.old), d.key}] = d
			}
			for _, t := range g.touches {
				bt[path.Dir(c.old)+"\x00"+t.String()]++
			}
		}
		if g := head.files[c.new]; c.new != "" && g != nil {
			for _, d := range g.decls {
				hd[keyed{path.Dir(c.new), d.key}] = d
			}
			for _, t := range g.touches {
				k := path.Dir(c.new) + "\x00" + t.String()
				ht[k]++
				if _, ok := htSite[k]; !ok {
					htSite[k] = site{file: c.new, line: t.line}
				}
			}
		}
		if c.status == 'R' && c.score == 100 {
			r.moves.files++
		}
	}

	byHash, byShape := map[string][]keyed{}, map[string][]keyed{}
	for _, k := range sortedKeyed(bd) {
		if _, ok := hd[k]; !ok {
			byHash[bd[k].hash] = append(byHash[bd[k].hash], k)
			byShape[bd[k].shape] = append(byShape[bd[k].shape], k)
		}
	}
	take := func(m map[string][]keyed, h string) (keyed, bool) {
		for len(m[h]) > 0 {
			k := m[h][0]
			m[h] = m[h][1:]
			if !movedFrom[k] {
				return k, true
			}
		}
		return keyed{}, false
	}
	for _, k := range sortedKeyed(hd) {
		d := hd[k]
		b, ok := bd[k]
		switch {
		case ok && b.hash == d.hash:
			r.moves.same++
		case ok:
			r.moves.changed++
			if d.exported && b.sig != d.sig {
				s := r.surfaces[k.pkg]
				s.changed = append(s.changed, d.key)
				r.surfaces[k.pkg] = s
			}
		default:
			from, ok := take(byHash, d.hash)
			if ok {
				r.moves.decls++
			} else if from, ok = take(byShape, d.shape); ok {
				r.moves.renamed++
			}
			if ok {
				movedFrom[from] = true
				r.moves.movedDecls = append(r.moves.movedDecls, moved{from.pkg, k.pkg, from.key, k.key})
			} else {
				r.moves.added++
			}
			if d.exported {
				s := r.surfaces[k.pkg]
				s.added = append(s.added, d.key)
				r.surfaces[k.pkg] = s
			}
		}
	}
	for k, d := range bd {
		if _, ok := hd[k]; ok {
			continue
		}
		if !movedFrom[k] {
			r.moves.removed++
		}
		if d.exported {
			s := r.surfaces[k.pkg]
			s.removed = append(s.removed, d.key)
			r.surfaces[k.pkg] = s
		}
	}
	for p, s := range r.surfaces {
		slices.Sort(s.added)
		slices.Sort(s.removed)
		slices.Sort(s.changed)
		r.surfaces[p] = s
	}
	slices.SortFunc(r.moves.movedDecls, func(a, b moved) int {
		return cmp.Or(cmp.Compare(a.from, b.from), cmp.Compare(a.to, b.to), cmp.Compare(a.oldKey, b.oldKey))
	})

	for k, n := range ht {
		if n > bt[k] {
			pkg, t, _ := strings.Cut(k, "\x00")
			kind, target, _ := strings.Cut(t, " ")
			s := htSite[k]
			r.touches = append(r.touches, touchChange{pkg: pkg, touch: touch{kind: kind, target: target, line: s.line}, file: s.file, added: true, risk: touchRisk(kind)})
		}
	}
	for k, n := range bt {
		if n > ht[k] {
			pkg, t, _ := strings.Cut(k, "\x00")
			kind, target, _ := strings.Cut(t, " ")
			r.touches = append(r.touches, touchChange{pkg: pkg, touch: touch{kind: kind, target: target}, risk: touchRisk(kind) / 3})
		}
	}
	slices.SortFunc(r.touches, func(a, b touchChange) int {
		return cmp.Or(cmp.Compare(b.risk, a.risk), cmp.Compare(a.touch.kind, b.touch.kind), cmp.Compare(a.touch.target, b.touch.target), cmp.Compare(a.pkg, b.pkg))
	})

	// go.mod requirements.
	bm, hm := map[string]string{}, map[string]string{}
	for _, m := range base.modules {
		addRequires(bm, m)
	}
	for _, m := range head.modules {
		addRequires(hm, m)
	}
	for _, k := range sortedStrings(bm, hm) {
		if bm[k] != hm[k] {
			r.deps = append(r.deps, depChange{mod: k, from: bm[k], to: hm[k]})
		}
	}

	// Everything not changed and not one step from a change folds away.
	shown := r.shownNodes()
	nb := r.neighbours(shown)
	for _, n := range nodes {
		if !shown[n] && !nb[n] {
			r.unchangedPkgs++
		}
	}
	return r
}

// keyed is a declaration's place: its package and key.
type keyed struct{ pkg, key string }

func sortedKeyed(m map[keyed]decl) []keyed {
	ks := make([]keyed, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	slices.SortFunc(ks, func(a, b keyed) int { return cmp.Or(cmp.Compare(a.pkg, b.pkg), cmp.Compare(a.key, b.key)) })
	return ks
}

// fingerprint is what an edge consists of: which files import, and what
// each uses through it. Line numbers are left out, so code moving within
// a file does not count as a change.
func fingerprint(ss []site) string {
	var parts []string
	for _, s := range ss {
		parts = append(parts, s.file+"|"+s.uses)
	}
	slices.Sort(parts)
	return strings.Join(parts, "\n")
}

func addRequires(dst map[string]string, m module) {
	for k, v := range m.requires {
		dst[k] = v
	}
}

func sortedStrings(a, b map[string]string) []string {
	var ks []string
	for k := range a {
		ks = append(ks, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			ks = append(ks, k)
		}
	}
	slices.Sort(ks)
	return ks
}

// stdTouch says whether a standard-library import is itself a touchpoint
// worth showing as an edge: the network, processes, databases, unsafe.
func stdTouch(n string) bool { return stdTouchKind(n) != "" }

func stdTouchKind(n string) string {
	switch strings.TrimPrefix(n, "std:") {
	case "net/http", "net", "net/rpc", "net/smtp":
		return "network"
	case "os/exec", "syscall":
		return "process"
	case "database/sql":
		return "database"
	case "unsafe", "plugin", "reflect":
		return "unsafe"
	case "os/signal":
		return "signals"
	}
	return ""
}

func touchRisk(kind string) int {
	switch kind {
	case "sql", "http":
		return 50
	case "exec":
		return 45
	case "env", "flag":
		return 35
	case "fs":
		return 25
	}
	return 20
}

// shownNodes is the default zoom: changed packages and the packages at
// either end of a changed edge.
func (r *result) shownNodes() map[string]bool {
	core := map[string]bool{}
	for p := range r.changedPkgs {
		core[p] = true
	}
	for _, e := range r.edges {
		core[e.from] = true
		if internal(e.to) {
			core[e.to] = true
		}
	}
	return core
}

// neighbours are the unchanged packages one step from a shown one. They
// fold into one line per lane: a hub such as cmd/ imports nearly every
// package, so drawing them all would show the whole graph.
func (r *result) neighbours(shown map[string]bool) map[string]bool {
	nb := map[string]bool{}
	for k := range r.head.edges {
		if !internal(k.to) {
			continue
		}
		if shown[k.from] && !shown[k.to] {
			nb[k.to] = true
		}
		if shown[k.to] && !shown[k.from] {
			nb[k.from] = true
		}
	}
	return nb
}

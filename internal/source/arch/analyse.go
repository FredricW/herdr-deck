package arch

import (
	"cmp"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// analyse compares the base's graph with the head's, given the files the
// change touched, and fills r with packages, edges, findings and the rest.
func analyse(r *Result, base, head *snapshot, files []deck.DiffFile, cfg Config) {
	// Nodes: every internal package of either side.
	all := map[string]bool{}
	for d := range base.pkgs {
		all[d] = true
	}
	for d := range head.pkgs {
		all[d] = true
	}
	nodes := slices.Sorted(maps.Keys(all))

	union := map[edgeKey][]Site{}
	maps.Copy(union, base.edges)
	maps.Copy(union, head.edges)
	var ln lanes
	if len(cfg.Layers) > 0 {
		ln = configLanes(cfg.Layers, nodes)
	} else {
		ln = inferredLanes(union, nodes)
	}
	for i, name := range ln.names {
		r.Lanes = append(r.Lanes, Lane{Name: name, Closed: ln.closed[i]})
	}
	r.Inferred = ln.inferred

	pkgs := map[string]*Package{}
	for _, n := range nodes {
		p := &Package{Path: n, Lane: ln.of[n]}
		_, inBase := base.pkgs[n]
		_, inHead := head.pkgs[n]
		switch {
		case !inBase:
			p.Status = Added
		case !inHead:
			p.Status = Removed
		}
		pkgs[n] = p
	}

	// Weights, from the changed source files the graphs read.
	changedPkgs := map[string]bool{}
	for _, f := range files {
		for _, p := range []string{f.OldPath, f.Path} {
			if p == "" || (base.files[p] == nil && head.files[p] == nil) {
				continue // test files, test data, other languages
			}
			changedPkgs[path.Dir(p)] = true
		}
		p := f.Path
		if head.files[p] == nil && base.files[p] == nil {
			p = f.OldPath
		}
		if pk := pkgs[path.Dir(p)]; p != "" && pk != nil && (head.files[p] != nil || base.files[p] != nil) {
			pk.Files++
			pk.Added += f.Added
			pk.Deleted += f.Deleted
			pk.Changed = append(pk.Changed, p)
		}
		if f.Change == deck.ChangeRenamed && f.Added == 0 && f.Deleted == 0 {
			r.Moves.Files++
		}
	}
	for p := range changedPkgs {
		if pk := pkgs[p]; pk != nil && pk.Status == Unchanged {
			pk.Status = Changed
		}
	}

	// Edges the change added or removed: internal ones, third-party ones,
	// and the standard-library imports that are touchpoints themselves.
	var changed []Edge
	for _, k := range sortedEdges(head.edges) {
		if _, ok := base.edges[k]; !ok && worthAnEdge(k.to) {
			changed = append(changed, Edge{From: k.from, To: k.to, Status: Added, Sites: head.edges[k]})
		}
	}
	for _, k := range sortedEdges(base.edges) {
		if _, ok := head.edges[k]; !ok && worthAnEdge(k.to) {
			changed = append(changed, Edge{From: k.from, To: k.to, Status: Removed, Sites: baseSites(base.edges[k])})
		}
	}
	for i := range changed {
		e := &changed[i]
		switch {
		case internal(e.To):
			e.Verdict, e.Why = ln.check(edgeKey{e.From, e.To})
			e.Risk = 30
			switch e.Verdict {
			case VerdictUp:
				e.Risk = 100
			case VerdictSkip:
				e.Risk = 60
			}
		case strings.HasPrefix(e.To, "ext:"):
			e.Risk, e.Why = 45, "third-party"
		default:
			e.Risk, e.Why = 40, stdKind(e.To)
		}
		if e.Status == Removed {
			e.Risk /= 3
		}
	}
	// Edges in both whose importers use something else through them.
	var same []Edge
	for _, k := range sortedEdges(head.edges) {
		b, ok := base.edges[k]
		if !ok || !internal(k.to) {
			continue
		}
		e := Edge{From: k.from, To: k.to, Status: Unchanged, Sites: head.edges[k]}
		e.Verdict, e.Why = ln.check(k)
		if fingerprint(b) != fingerprint(head.edges[k]) {
			e.Status, e.Risk = Changed, 15
			changed = append(changed, e)
			continue
		}
		same = append(same, e)
	}

	r.Cycles = newCycles(compEdges(base.edges), compEdges(head.edges))

	// Declarations and touchpoints of the changed files, both sides.
	bd, hd := map[keyed]decl{}, map[keyed]decl{}
	bt, ht := map[string]int{}, map[string]int{}
	htSite, btSite := map[string]Site{}, map[string]Site{}
	for _, f := range files {
		old := f.OldPath
		if old == "" && f.Change != deck.ChangeAdded {
			old = f.Path
		}
		if g := base.files[old]; old != "" && g != nil {
			for _, d := range g.decls {
				bd[keyed{path.Dir(old), d.key}] = d
			}
			for _, t := range g.touches {
				k := path.Dir(old) + "\x00" + t.key()
				bt[k]++
				if _, ok := btSite[k]; !ok {
					btSite[k] = Site{File: old, Line: t.line, Base: true}
				}
			}
		}
		if f.Change == deck.ChangeDeleted {
			continue
		}
		if g := head.files[f.Path]; g != nil {
			for _, d := range g.decls {
				hd[keyed{path.Dir(f.Path), d.key}] = d
			}
			for _, t := range g.touches {
				k := path.Dir(f.Path) + "\x00" + t.key()
				ht[k]++
				if _, ok := htSite[k]; !ok {
					htSite[k] = Site{File: f.Path, Line: t.line}
				}
			}
		}
	}
	moved := r.declMoves(bd, hd, pkgs)

	// An edge that left with its code: X → D removed and Y → D added while
	// declarations moved from X to Y.
	for i := range changed {
		a := &changed[i]
		if a.Status != Added {
			continue
		}
		for j := range changed {
			b := &changed[j]
			if b.Status == Removed && b.To == a.To && moved[b.From+"\x00"+a.From] {
				a.MovedWithCode, b.MovedWithCode = true, true
				a.Risk, b.Risk = 8, 2
			}
		}
	}
	slices.SortStableFunc(changed, func(a, b Edge) int {
		return cmp.Or(cmp.Compare(b.Risk, a.Risk), cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
	})
	r.Edges = append(changed, same...)

	for _, k := range slices.Sorted(maps.Keys(ht)) {
		if ht[k] > bt[k] {
			r.Touches = append(r.Touches, touchOf(k, htSite[k], false))
		}
	}
	for _, k := range slices.Sorted(maps.Keys(bt)) {
		if bt[k] > ht[k] {
			r.Touches = append(r.Touches, touchOf(k, btSite[k], true))
		}
	}
	slices.SortStableFunc(r.Touches, func(a, b Touch) int { return cmp.Compare(b.Risk, a.Risk) })

	// Requirements in go.mod and package.json.
	bm, hm := requirements(base.modules), requirements(head.modules)
	for _, k := range slices.Sorted(maps.Keys(union2(bm, hm))) {
		if bm[k] != hm[k] {
			r.Deps = append(r.Deps, Dep{Module: k, From: bm[k], To: hm[k]})
		}
	}

	// What shows on a package's box.
	for _, e := range changed {
		if e.Status == Changed {
			continue
		}
		if pk := pkgs[e.From]; pk != nil {
			pk.Impacted = true
			if e.Status == Added && strings.HasPrefix(e.To, "ext:") {
				pk.NewDeps++
			}
		}
		if pk := pkgs[e.To]; pk != nil {
			pk.Impacted = true
		}
	}
	for p := range changedPkgs {
		if pk := pkgs[p]; pk != nil {
			pk.Impacted = true
		}
	}
	for _, t := range r.Touches {
		if pk := pkgs[t.Package]; pk != nil && !t.Removed && !slices.Contains(pk.Touches, t.Kind) {
			pk.Touches = append(pk.Touches, t.Kind)
		}
	}
	for _, n := range nodes {
		pk := pkgs[n]
		slices.Sort(pk.Changed)
		slices.Sort(pk.Touches)
		slices.Sort(pk.MovedFrom)
		pk.MovedFrom = slices.Compact(pk.MovedFrom)
		if pk.Status == Unchanged && pk.MovedIn > 0 {
			pk.Status = Moved
			pk.Impacted = true
		}
		r.Packages = append(r.Packages, *pk)
	}
	r.findings()
}

// worthAnEdge says whether an edge to node to is shown: every internal or
// third-party one, and the standard-library imports that are touchpoints.
func worthAnEdge(to string) bool { return !strings.HasPrefix(to, "std:") || stdKind(to) != "" }

func baseSites(ss []Site) []Site {
	out := slices.Clone(ss)
	for i := range out {
		out[i].Base = true
	}
	return out
}

// keyed is a declaration's place: its package and key.
type keyed struct{ pkg, key string }

func cmpKeyed(a, b keyed) int { return cmp.Or(cmp.Compare(a.pkg, b.pkg), cmp.Compare(a.key, b.key)) }

// declMoves matches the declarations of the changed files across the two
// sides: the same key in the same package is unchanged or changed in
// place; a new one whose text (or text with the name blanked) matches a
// gone one moved, or moved and was renamed. It fills the packages' API
// surfaces and moves, and returns the moves between packages as
// "from\x00to".
func (r *Result) declMoves(bd, hd map[keyed]decl, pkgs map[string]*Package) map[string]bool {
	byHash, byShape := map[string][]keyed{}, map[string][]keyed{}
	for _, k := range slices.SortedFunc(maps.Keys(bd), cmpKeyed) {
		if _, ok := hd[k]; !ok {
			byHash[bd[k].hash] = append(byHash[bd[k].hash], k)
			byShape[bd[k].shape] = append(byShape[bd[k].shape], k)
		}
	}
	taken := map[keyed]bool{}
	take := func(m map[string][]keyed, h string) (keyed, bool) {
		for len(m[h]) > 0 {
			k := m[h][0]
			m[h] = m[h][1:]
			if !taken[k] {
				return k, true
			}
		}
		return keyed{}, false
	}
	api := func(p string) *Surface {
		if pk := pkgs[p]; pk != nil {
			return &pk.API
		}
		return &Surface{}
	}
	moved := map[string]bool{}
	for _, k := range slices.SortedFunc(maps.Keys(hd), cmpKeyed) {
		d := hd[k]
		b, ok := bd[k]
		switch {
		case ok && b.hash == d.hash:
		case ok:
			r.Moves.Changed++
			if d.exported && b.sig != d.sig {
				s := api(k.pkg)
				s.Changed = append(s.Changed, d.key)
			}
		default:
			from, ok := take(byHash, d.hash)
			if ok {
				r.Moves.Decls++
			} else if from, ok = take(byShape, d.shape); ok {
				r.Moves.Renamed++
			}
			if ok {
				taken[from] = true
				if from.pkg != k.pkg {
					moved[from.pkg+"\x00"+k.pkg] = true
					if pk := pkgs[k.pkg]; pk != nil {
						pk.MovedIn++
						pk.MovedFrom = append(pk.MovedFrom, from.pkg)
					}
				}
			} else {
				r.Moves.Added++
			}
			if d.exported {
				s := api(k.pkg)
				s.Added = append(s.Added, d.key)
			}
		}
	}
	for _, k := range slices.SortedFunc(maps.Keys(bd), cmpKeyed) {
		if _, ok := hd[k]; ok {
			continue
		}
		if !taken[k] {
			r.Moves.Removed++
		}
		if bd[k].exported {
			s := api(k.pkg)
			s.Removed = append(s.Removed, bd[k].key)
		}
	}
	for _, pk := range pkgs {
		slices.Sort(pk.API.Added)
		slices.Sort(pk.API.Removed)
		slices.Sort(pk.API.Changed)
	}
	return moved
}

func touchOf(k string, s Site, removed bool) Touch {
	pkg, t, _ := strings.Cut(k, "\x00")
	kind, target, _ := strings.Cut(t, " ")
	risk := touchRisk(kind)
	if removed {
		risk /= 3
	}
	return Touch{Package: pkg, Kind: kind, Target: target, Site: s, Removed: removed, Risk: risk}
}

// fingerprint is what an edge consists of: which files import, and what
// each uses through it. Lines are left out, so code moving within a file
// is not a change.
func fingerprint(ss []Site) string {
	parts := make([]string, 0, len(ss))
	for _, s := range ss {
		parts = append(parts, s.File+"|"+s.Uses)
	}
	slices.Sort(parts)
	return strings.Join(parts, "\n")
}

func requirements(ms []module) map[string]string {
	out := map[string]string{}
	for _, m := range ms {
		maps.Copy(out, m.requires)
	}
	return out
}

func union2(a, b map[string]string) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}

// stdKind is what a standard-library import does when it is a touchpoint
// worth showing as an edge, else "".
func stdKind(n string) string {
	switch strings.TrimPrefix(n, "std:") {
	case "net/http", "net", "net/rpc", "net/smtp", "http", "https":
		return "network"
	case "os/exec", "syscall", "child_process":
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

// findings makes Look here first: everything that changed the shape, by
// risk: upward edges, cycles, layer skips, new dependencies, touchpoints,
// new internal edges, then API changes (a package being written changes
// its API all the time; a new edge between packages is news).
func (r *Result) findings() {
	var fs []Finding
	for _, c := range r.Cycles {
		f := Finding{Kind: FindCycle, Sign: "↻", Text: strings.Join(c, " → "), Why: "new cycle", Risk: 95, Edge: -1, ID: "cycle:" + cycleID(c)}
		// The cycle's new edge: its first step, between packages.
		for i, e := range r.Edges {
			if e.Status == Added && e.Internal() && component(e.From) == c[0] && component(e.To) == c[1] {
				f.Package, f.Edge, f.Site = e.From, i, e.Sites[0]
				break
			}
		}
		fs = append(fs, f)
	}
	for i, e := range r.Edges {
		if e.Status != Added && e.Status != Removed {
			continue
		}
		if e.MovedWithCode && e.Status == Removed {
			continue // its added twin says it
		}
		f := Finding{Kind: FindEdge, Sign: "+", Text: e.From + " → " + Short(e.To), Why: e.Why, Verdict: e.Verdict,
			Risk: e.Risk, Package: e.From, Edge: i, Site: e.Sites[0], ID: "edge:" + e.From + "→" + Short(e.To)}
		switch {
		case e.MovedWithCode:
			f.Sign, f.Why = "≡", "moved with its code"
		case e.Status == Removed:
			f.Sign = "−"
		}
		fs = append(fs, f)
	}
	for _, t := range r.Touches {
		target := t.Target
		if target == "" {
			target = "(dynamic)"
		}
		f := Finding{Kind: FindTouch, Sign: "+", Text: t.Kind + " " + target, Why: "in " + t.Package,
			Risk: t.Risk, Package: t.Package, Edge: -1, Site: t.Site, ID: "touch:" + t.Kind + ":" + t.Target + "@" + t.Package}
		if t.Removed {
			f.Sign = "−"
		}
		fs = append(fs, f)
	}
	for _, d := range r.Deps {
		f := Finding{Kind: FindDep, Risk: 48, Edge: -1, ID: "dep:" + d.Module, Why: "dependency"}
		switch {
		case d.From == "":
			f.Sign, f.Text = "+", "dep "+d.Module+" "+d.To
		case d.To == "":
			f.Sign, f.Text, f.Risk = "−", "dep "+d.Module, 16
		default:
			f.Sign, f.Text, f.Risk = "~", "dep "+d.Module+" "+d.From+" → "+d.To, 20
		}
		for i, e := range r.Edges {
			if e.Status == Added && (e.To == "ext:"+d.Module || strings.HasPrefix(e.To, "ext:"+d.Module+"/")) {
				f.Package, f.Edge = e.From, i
				break
			}
		}
		fs = append(fs, f)
	}
	for _, p := range r.Packages {
		if s := p.API; len(s.Removed)+len(s.Changed) > 0 {
			fs = append(fs, Finding{Kind: FindAPI, Sign: "~", Text: "api " + p.Path,
				Why:  fmt.Sprintf("−%d ~%d +%d", len(s.Removed), len(s.Changed), len(s.Added)),
				Risk: 28, Package: p.Path, Edge: -1, ID: "api:" + p.Path})
		}
	}
	slices.SortStableFunc(fs, func(a, b Finding) int { return cmp.Compare(b.Risk, a.Risk) })
	r.Findings = fs
}

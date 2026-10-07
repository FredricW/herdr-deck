package main

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// The views render as plain text at a given width, the way the drawer's
// tab content would before colour: one string per line, cut with … when
// too long. The real tab would style them (new edges bold, removed dim,
// verdicts in the status colours).

type out struct {
	w     int
	lines []string
}

func (o *out) add(format string, a ...any) {
	s := fmt.Sprintf(format, a...)
	if utf8.RuneCountInString(s) > o.w {
		r := []rune(s)
		s = string(r[:o.w-1]) + "…"
	}
	o.lines = append(o.lines, s)
}

// rule is an Overview-style section rule: ── Name ──.
func (o *out) rule(name string) { o.add(" ── %s ──", name) }

// pad puts right at the right edge of a line that starts with left.
func pad(w int, left, right string) string {
	n := w - utf8.RuneCountInString(left) - utf8.RuneCountInString(right)
	if n < 1 {
		n = 1
	}
	return left + strings.Repeat(" ", n) + right
}

func short(n string) string {
	switch {
	case strings.HasPrefix(n, "ext:"):
		return strings.TrimPrefix(n, "ext:")
	case strings.HasPrefix(n, "std:"):
		return strings.TrimPrefix(n, "std:")
	}
	return n
}

func (r *result) summary(o *out) {
	var add, del, up, skip int
	for _, e := range r.edges {
		if e.added {
			add++
		} else {
			del++
		}
		if e.added && e.verdict == "up" {
			up++
		}
		if e.added && e.verdict == "skip" {
			skip++
		}
	}
	var ta int
	for _, t := range r.touches {
		if t.added {
			ta++
		}
	}
	var sa, sr, sc int
	for _, s := range r.surfaces {
		sa += len(s.added)
		sr += len(s.removed)
		sc += len(s.changed)
	}
	left := fmt.Sprintf(" edges +%d −%d", add, del)
	if up > 0 {
		left += fmt.Sprintf("  ✕ %d upward", up)
	}
	if skip > 0 {
		left += fmt.Sprintf("  ⚠ %d skip", skip)
	}
	if len(r.cycles) > 0 {
		left += fmt.Sprintf("  ↻ %d cycle", len(r.cycles))
	}
	left += fmt.Sprintf("  touch +%d  api +%d −%d ~%d", ta, sa, sr, sc)
	o.add("%s", left)
	m := r.moves
	o.add(" decls: %d moved · %d renamed · %d changed · %d new · %d gone · %d files moved", m.decls, m.renamed, m.changed, m.added, m.removed, m.files)
	if m.changed+m.added+m.removed == 0 && len(r.edges) == 0 && m.files+m.decls+m.renamed > 0 {
		o.add(" ≡ pure move: no declaration or dependency changed")
	}
}

// first is the "look here first" list: everything that changed the
// shape, by risk, with its stable ID.
func (r *result) first(o *out, limit int) {
	type item struct {
		risk      int
		text, id  string
		site      string
		addedItem bool
	}
	var items []item
	for _, c := range r.cycles {
		items = append(items, item{95, "↻ cycle " + strings.Join(c, " → "), "cycle:" + cycleID(c), "", true})
	}
	for _, e := range r.edges {
		g := "+"
		if !e.added {
			g = "−"
		}
		tag := ""
		switch e.verdict {
		case "up":
			tag = "  ✕ upward " + e.why
		case "skip":
			tag = "  ⚠ " + e.why
		default:
			if e.why != "" {
				tag = "  " + e.why
			}
		}
		s := e.sites[0]
		items = append(items, item{e.risk, fmt.Sprintf("%s %s → %s%s", g, e.from, short(e.to), tag), "edge:" + e.from + "→" + short(e.to), fmt.Sprintf("%s:%d", s.file, s.line), e.added})
	}
	for _, t := range r.touches {
		g := "+"
		if !t.added {
			g = "−"
		}
		site := ""
		if t.added {
			site = fmt.Sprintf("%s:%d", t.file, t.touch.line)
		}
		items = append(items, item{t.risk, fmt.Sprintf("%s %-4s %s  in %s", g, t.touch.kind, orDash(t.touch.target), t.pkg), "touch:" + t.touch.kind + ":" + t.touch.target + "@" + t.pkg, site, t.added})
	}
	for _, d := range r.deps {
		text := "+ dep  " + d.mod + " " + d.to
		switch {
		case d.to == "":
			text = "− dep  " + d.mod
		case d.from != "":
			text = "~ dep  " + d.mod + " " + d.from + " → " + d.to
		}
		items = append(items, item{48, text, "dep:" + d.mod, "go.mod", d.to != ""})
	}
	for _, p := range slices.Sorted(keys(r.surfaces)) {
		s := r.surfaces[p]
		if len(s.removed)+len(s.changed) > 0 {
			items = append(items, item{38, fmt.Sprintf("~ api  %s  −%d ~%d +%d", p, len(s.removed), len(s.changed), len(s.added)), "api:" + p, "", true})
		}
	}
	slices.SortStableFunc(items, func(a, b item) int { return b.risk - a.risk })
	o.rule("Look here first")
	if len(items) == 0 {
		o.add(" nothing changed the shape")
	}
	for i, it := range items {
		if i == limit {
			o.add(" ⋯ %d more", len(items)-limit)
			break
		}
		if o.w >= 100 && it.site != "" {
			o.add("%s", pad(o.w, fmt.Sprintf(" %-2d %s", i+1, it.text), it.site))
		} else {
			o.add(" %-2d %s", i+1, it.text)
		}
	}
}

func orDash(s string) string {
	if s == "" {
		return "(dynamic)"
	}
	return s
}

func keys[V any](m map[string]V) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// lanesView draws the default zoom as lanes: one section per layer, the
// shown packages in it, and under each package the edges the change adds
// (━▸) or removes (┄▸). Unchanged edges are not drawn; a count says how
// many there are.
func (r *result) lanesView(o *out) {
	shown := r.shownNodes()
	byLane := map[int][]string{}
	for n := range shown {
		byLane[r.lanes.of[n]] = append(byLane[r.lanes.of[n]], n)
	}
	nbLane := map[int][]string{}
	for n := range r.neighbours(shown) {
		nbLane[r.lanes.of[n]] = append(nbLane[r.lanes.of[n]], n)
	}
	out := map[string][]edgeChange{}
	for _, e := range r.edges {
		out[e.from] = append(out[e.from], e)
	}
	added := map[string]bool{}
	for _, p := range r.pkgsAdded {
		added[p] = true
	}
	removed := map[string]bool{}
	for _, p := range r.pkgsRemoved {
		removed[p] = true
	}
	for i, name := range r.lanes.names {
		ns, nbs := byLane[i], nbLane[i]
		if len(ns)+len(nbs) == 0 {
			continue
		}
		slices.Sort(ns)
		title := name
		if r.lanes.closed[i] {
			title += " (closed)"
		}
		o.rule(title)
		for _, n := range ns {
			glyph := "◌"
			switch {
			case added[n]:
				glyph = "✚"
			case removed[n]:
				glyph = "✕"
			case r.changedPkgs[n]:
				glyph = "●"
			}
			right := ""
			if w, ok := r.weights[n]; ok {
				right = fmt.Sprintf("+%d −%d", w.add, w.del)
			}
			if s, ok := r.surfaces[n]; ok && !s.empty() {
				right = fmt.Sprintf("api +%d −%d ~%d  ", len(s.added), len(s.removed), len(s.changed)) + right
			}
			o.add("%s", pad(o.w, fmt.Sprintf(" %s %s", glyph, n), right+" "))
			for _, e := range out[n] {
				arrow := "━▸"
				if !e.added {
					arrow = "┄▸"
				}
				tag := ""
				switch e.verdict {
				case "up":
					tag = "  ✕ upward"
				case "skip":
					tag = "  ⚠ " + e.why
				}
				if !internal(e.to) {
					tag = "  " + e.why
				}
				o.add("     %s %s%s", arrow, short(e.to), tag)
			}
		}
		if len(nbs) > 0 {
			slices.Sort(nbs)
			names := make([]string, len(nbs))
			for j, n := range nbs {
				names[j] = lastSeg(n)
			}
			o.add(" ◌ %d unchanged: %s", len(nbs), strings.Join(names, " "))
		}
	}
	if r.unchangedPkgs > 0 {
		o.add(" ⋯ %d more packages, not next to the change", r.unchangedPkgs)
	}
}

// matrixView is the dense fallback: lane by lane, how many edges go from
// a row's lane to a column's that were there before, and how many are
// new (+n). Cells below the diagonal point upwards (✕).
func (r *result) matrixView(o *out) {
	n := len(r.lanes.names)
	count := make([][]int, n)
	fresh := make([][]int, n)
	for i := range count {
		count[i] = make([]int, n)
		fresh[i] = make([]int, n)
	}
	for k := range r.head.edges {
		if !internal(k.to) {
			continue
		}
		a, b := r.lanes.of[k.from], r.lanes.of[k.to]
		count[a][b]++
		if _, ok := r.base.edges[k]; !ok {
			fresh[a][b]++
		}
	}
	label := 0
	for _, nm := range r.lanes.names {
		label = max(label, utf8.RuneCountInString(nm))
	}
	label = min(label, 14)
	cell := 6
	head := fmt.Sprintf(" %-*s", label, "from \\ to")
	for i := range r.lanes.names {
		head += fmt.Sprintf("%*s", cell, abbrev(r.lanes.names[i], cell-1))
	}
	o.rule("Matrix")
	o.add("%s", head)
	for i, nm := range r.lanes.names {
		line := fmt.Sprintf(" %-*s", label, abbrev(nm, label))
		for j := range r.lanes.names {
			c := "·"
			if count[i][j] > 0 {
				// unchanged edges, then +new ones: "3+1", or "+1" alone
				c = ""
				if old := count[i][j] - fresh[i][j]; old > 0 {
					c = fmt.Sprint(old)
				}
				if fresh[i][j] > 0 {
					c += fmt.Sprintf("+%d", fresh[i][j])
				}
				if j < i {
					c = "✕" + c
				}
			}
			line += fmt.Sprintf("%*s", cell, c)
		}
		o.add("%s", line)
	}
}

func lastSeg(n string) string {
	if i := strings.LastIndex(n, "/"); i >= 0 {
		return n[i+1:]
	}
	return n
}

// name is a declaration key's name: "method T.Name" -> T.Name.
func name(key string) string {
	_, n, _ := strings.Cut(key, " ")
	return n
}

func abbrev(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

// detail prints the sections behind a selection: the API changes per
// package and the moved declarations.
func (r *result) detail(o *out) {
	if len(r.surfaces) > 0 {
		o.rule("Public surface")
		for _, p := range slices.Sorted(keys(r.surfaces)) {
			s := r.surfaces[p]
			if s.empty() {
				continue
			}
			o.add(" %s", p)
			for _, k := range s.added {
				o.add("   + %s", k)
			}
			for _, k := range s.removed {
				o.add("   − %s", k)
			}
			for _, k := range s.changed {
				o.add("   ~ %s", k)
			}
		}
	}
	if len(r.moves.movedDecls) > 0 {
		o.rule("Moved unchanged")
		type group struct{ from, to string }
		var order []group
		count := map[group]int{}
		var renames []string
		for _, m := range r.moves.movedDecls {
			g := group{m.from, m.to}
			if count[g] == 0 {
				order = append(order, g)
			}
			count[g]++
			if name(m.oldKey) != name(m.newKey) {
				renames = append(renames, name(m.oldKey)+" → "+name(m.newKey))
			}
		}
		for _, g := range order {
			if g.from == g.to {
				o.add(" ≡ %d decls within %s", count[g], g.from)
			} else {
				o.add(" ≡ %d decls %s → %s", count[g], g.from, g.to)
			}
		}
		if len(renames) > 0 {
			o.add("   renamed: %s", strings.Join(renames, ", "))
		}
	}
}

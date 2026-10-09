package main

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

type renderer struct {
	a         *Analysis
	w         int
	out       []string
	maxDepth  int
	printed   map[string]bool
	entryMemo map[string][]string
	hubs      []string
	inHubs    bool
	flowCount int
	hubsSeen  []string
}

func slicesContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func width(s string) int { return utf8.RuneCountInString(s) } // the glyphs used are single-cell

func cut(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if width(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

// row puts left and right on one line of width w, truncating left first.
func (r *renderer) row(left, right string) {
	if right == "" {
		r.out = append(r.out, cut(left, r.w))
		return
	}
	right = cut(right, r.w/2)
	room := r.w - width(right) - 1
	left = cut(left, room)
	r.out = append(r.out, left+strings.Repeat(" ", r.w-width(left)-width(right))+right)
}

func (r *renderer) blank() { r.out = append(r.out, "") }

func glyph(f *Finding) string {
	switch {
	case f.Warn:
		return "⚠"
	case f.Refactor:
		return "≈"
	case f.Kind == "added":
		return "+"
	case f.Kind == "removed":
		return "−"
	case f.Kind == "renamed":
		return "»"
	}
	return "~"
}

func (r *renderer) entryName(id string) string {
	if fn := r.a.Head.Funcs[id]; fn != nil {
		return flowName(fn)
	}
	if fn := r.a.Base.Funcs[id]; fn != nil {
		return flowName(fn)
	}
	return id
}

// flowName drops the receiver for split cases: "Update ▸ diffMsg", "handleKey ▸ key Quit".
func flowName(fn *Func) string {
	n := fn.Name
	if i := strings.Index(n, ".Model."); i >= 0 {
		n = n[i+len(".Model."):]
	}
	n = strings.Replace(n, " ▸ case ", " ▸ ", 1)
	return n
}

func (r *renderer) lookHere(limit int) {
	a := r.a
	var shown, refactors, unreached int
	r.row(" Look here first", fmt.Sprintf("%d frames · %d flows ", len(a.Findings), len(r.flows())))
	for _, f := range a.Findings {
		if f.Refactor {
			refactors++
			continue
		}
		if shown >= limit {
			continue
		}
		shown++
		flows := fmt.Sprintf("%d flows", len(f.Entries))
		if len(f.Entries) == 1 {
			flows = "1 flow"
		}
		reason := strings.Join(f.Reasons, " · ")
		left := fmt.Sprintf(" %d %s %s", shown, glyph(f), f.Name)
		// Name and reason share the left side; the flow count is right-aligned.
		if r.w >= 100 {
			left = fmt.Sprintf("%-46s %s", left, reason)
			r.row(left, flows+" ")
		} else {
			r.row(left, flows+" ")
			r.row("      "+reason, "")
		}
		if len(f.Entries) == 0 {
			unreached++
		}
	}
	more := len(a.Findings) - shown - refactors
	var foot []string
	if more > 0 {
		foot = append(foot, fmt.Sprintf("⋯ %d more", more))
	}
	if refactors > 0 {
		foot = append(foot, fmt.Sprintf("≈ %d refactors collapsed", refactors))
	}
	if len(foot) > 0 {
		r.row("   "+strings.Join(foot, " · "), "")
	}
}

// flows are the entries reached by any finding, ranked by their best finding.
func (r *renderer) flows() []string {
	best := map[string]float64{}
	for _, f := range r.a.Findings {
		for _, e := range f.Entries {
			if f.Score > best[e] {
				best[e] = f.Score
			}
		}
	}
	var out []string
	for e := range best {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if best[out[i]] != best[out[j]] {
			return best[out[i]] > best[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

// flow renders one entry's tree diff. It reports false (and prints nothing)
// when the entry only reaches the change through shared frames.
func (r *renderer) flow(entry string) bool {
	a := r.a
	b := a.toBase[entry]
	r.hubsSeen = nil
	r.printed = map[string]bool{}
	nodes := compress(prune(r.build(b, entry, 1, map[string]bool{entry: true})))
	if len(nodes) == 0 {
		return false
	}
	hEff := a.syncHead.of(entry) &^ (EffUnknown | EffEnv)
	var bEff Effect
	if b != "" {
		bEff = a.syncBase.of(b) &^ (EffUnknown | EffEnv)
	}
	right := ""
	if hEff != bEff {
		right = fmt.Sprintf("sync %s → %s ", orDash(bEff.icons()), orDash(hEff.icons()))
	}
	r.blank()
	r.row(fmt.Sprintf(" ▸ %s", r.entryName(entry)), right)
	if fn := a.Head.Funcs[entry]; fn != nil {
		r.row(fmt.Sprintf("   %s:%d", fn.File, fn.Line), "")
	}
	if len(r.hubsSeen) > 0 {
		var names []string
		for _, h := range r.hubsSeen {
			names = append(names, lastPart(r.a.Head.Funcs[h].Name))
		}
		r.row("   via ◇ "+strings.Join(names, ", "), "")
	}
	r.print(nodes, 1)
	return true
}

// prune drops unchanged frames that only lead to a shared frame or repeat one shown above.
func prune(ns []*node) []*node {
	out := ns[:0]
	for _, n := range ns {
		n.children = prune(n.children)
		// An unchanged frame is only there to lead somewhere: without children left, it goes.
		if n.mark == " " && !n.changed && len(n.children) == 0 {
			continue
		}
		n.right = strings.TrimSpace(strings.ReplaceAll(n.right, "↑ above", "↑"))
		// A lone "deeper frames folded" child becomes a ⋯ on its parent.
		if len(n.children) == 1 && n.children[0].mark == "⋯" {
			n.children = nil
			n.right = strings.TrimSpace(n.right + " ⋯")
		}
		out = append(out, n)
	}
	return out
}

// entries is memoised: hubs need it for every relevant frame.
func (r *renderer) entries(id string) []string {
	if r.entryMemo == nil {
		r.entryMemo = map[string][]string{}
	}
	if e, ok := r.entryMemo[id]; ok {
		return e
	}
	e := entriesOf(r.a.Head, r.a.callers, id, false)
	r.entryMemo[id] = e
	return e
}

// A hub is a frame most flows go through (in a TUI: the layout and render path).
// Flow trees stop at hubs; each hub's tree is shown once, under "Shared frames".
func (r *renderer) isHub(id string) bool {
	n := len(r.entries(id))
	return n >= 6 && n*2 >= r.flowCount
}

func (r *renderer) useHub(id string) {
	for _, h := range r.hubs {
		if h == id {
			return
		}
	}
	r.hubs = append(r.hubs, id)
}

func (r *renderer) sharedFrames(limit int) {
	// Changed hubs first: their own diff is the shared part every flow inherits.
	sort.SliceStable(r.hubs, func(i, j int) bool {
		ci, cj := r.a.Changed[r.hubs[i]], r.a.Changed[r.hubs[j]]
		if ci != cj {
			return ci
		}
		return len(r.entries(r.hubs[i])) > len(r.entries(r.hubs[j]))
	})
	// Inside this section hubs expand like any frame; each frame is shown once across it.
	r.inHubs = true
	defer func() { r.inHubs = false }()
	r.printed = map[string]bool{}
	shown := 0
	for _, h := range r.hubs {
		if r.printed[h] {
			continue
		}
		if shown == limit {
			break
		}
		r.printed[h] = true
		nodes := compress(prune(r.build(r.a.toBase[h], h, 1, map[string]bool{h: true})))
		if len(nodes) == 0 && !r.a.Changed[h] {
			continue
		}
		shown++
		r.blank()
		fn := r.a.Head.Funcs[h]
		mark := " "
		if r.a.Changed[h] {
			mark = "~"
		}
		r.row(fmt.Sprintf(" ◇ %s %s", mark, fn.Name), fmt.Sprintf("shared by %d flows ", len(r.entries(h))))
		r.row(fmt.Sprintf("   %s:%d", fn.File, fn.Line), "")
		r.print(nodes, 1)
	}
}

func orDash(s string) string {
	if s == "" {
		return "–"
	}
	return s
}

func (r *renderer) indent(depth int) string { return strings.Repeat("  ", depth+1) }

// node is one row of a flow tree before it is printed.
type node struct {
	mark     string // " ", "+", "−", "~", "≈", "⋯"
	crumbs   []string
	right    string
	changed  bool // the callee itself changed: never merged into a breadcrumb
	children []*node
}

func (r *renderer) build(bID, hID string, depth int, path map[string]bool) []*node {
	a := r.a
	var bf, hf *Func
	if bID != "" {
		bf = a.Base.Funcs[bID]
	}
	if hID != "" {
		hf = a.Head.Funcs[hID]
	}
	var out []*node
	if depth > r.maxDepth {
		return []*node{{mark: "⋯", crumbs: []string{"deeper frames folded"}}}
	}
	var pureAdd, pureDel int
	defer func() {
		if pureAdd+pureDel > 0 {
			out = append(out, &node{mark: "·", crumbs: []string{fmt.Sprintf("%+d −%d pure calls", pureAdd, pureDel)}})
		}
	}()
	for _, row := range diffSites(a, bf, hf) {
		s := row.h
		if row.op == '-' {
			s = row.b
		}
		relevant := r.relevant(row)
		if row.op == ' ' && !relevant {
			continue
		}
		eff := r.effOf(row)
		// Calls outside the module with no side effect are noise in a call tree: count them.
		if s.Kind == SiteExt && eff == 0 && row.op != '~' {
			if row.op == '+' {
				pureAdd++
			} else if row.op == '-' {
				pureDel++
			}
			continue
		}
		note := ctxNote(s.Ctx)
		switch row.op {
		case '~':
			note = moveNote(row.b, row.h)
		case '+':
			if c := firstCallee(s); s.Kind == SiteCall && a.Changed[c] && a.toBase[c] == "" {
				note = strings.TrimSpace(note + " new")
			}
		}
		if s.Kind == SiteCall && len(s.Callees) > 1 {
			note = strings.TrimSpace(fmt.Sprintf("%s %d impls", note, len(s.Callees)))
		}
		mark := string(row.op)
		if row.op == '-' {
			mark = "−"
		}
		label := strings.TrimPrefix(s.Label, "▸ ")
		icons := eff.icons()
		if row.op == ' ' || row.op == '~' {
			icons = effDelta(r.effOfBase(row), eff) // unchanged calls only show what their effects gained or lost
		}
		n := &node{mark: mark, crumbs: []string{label}, right: strings.TrimSpace(icons + " " + note)}
		for _, c := range s.Callees {
			if a.Changed[c] {
				n.changed = true
				if a.Refactor[c] && mark == " " {
					n.mark = "≈"
				} else if mark == " " {
					n.mark = "~"
				}
			}
		}
		out = append(out, n)
		if !relevant || row.op == '-' {
			continue
		}
		// A brand-new subtree is summarised on one row: its shape has nothing to diff against.
		if row.op == '+' && s.Kind == SiteCall && len(s.Callees) == 1 && a.toBase[s.Callees[0]] == "" && a.Changed[s.Callees[0]] {
			if k := r.newFrames(s.Callees[0], map[string]bool{}); k > 1 {
				n.right = strings.TrimSpace(n.right + fmt.Sprintf(" · %d frames", k))
			}
			continue
		}
		for _, c := range s.Callees {
			if path[c] || !a.relHead[c] {
				continue
			}
			if r.isHub(c) && !r.inHubs {
				n.right = strings.TrimSpace(n.right + " ◇")
				r.useHub(c)
				if !slicesContains(r.hubsSeen, c) {
					r.hubsSeen = append(r.hubsSeen, c)
				}
				continue
			}
			if r.printed[c] {
				n.right = strings.TrimSpace(n.right + " ↑ above")
				continue
			}
			r.printed[c] = true
			path[c] = true
			n.children = append(n.children, r.build(a.toBase[c], c, depth+1, path)...)
			delete(path, c)
		}
	}
	return out
}

// newFrames counts the new functions reachable from a new function through other new ones.
func (r *renderer) newFrames(id string, seen map[string]bool) int {
	if seen[id] || r.a.toBase[id] != "" || !r.a.Changed[id] {
		return 0
	}
	seen[id] = true
	n := 1
	for _, s := range r.a.Head.Funcs[id].Sites {
		for _, c := range s.Callees {
			n += r.newFrames(c, seen)
		}
	}
	return n
}

// compress merges chains of unchanged frames with one relevant child into a breadcrumb.
func compress(ns []*node) []*node {
	for _, n := range ns {
		for n.mark == " " && !n.changed && len(n.children) == 1 && n.right == "" && n.children[0].mark != "⋯" {
			c := n.children[0]
			n.crumbs = append(n.crumbs, c.crumbs...)
			n.mark, n.right, n.changed, n.children = c.mark, c.right, c.changed, c.children
		}
		n.children = compress(n.children)
	}
	return ns
}

func (r *renderer) print(ns []*node, depth int) {
	for _, n := range ns {
		right := n.right
		if right != "" {
			right += " "
		}
		room := r.w - width(right) - 1 - width(r.indent(depth)) - 2
		r.row(fmt.Sprintf("%s%s %s", r.indent(depth), n.mark, crumbs(n.crumbs, room)), right)
		r.print(n.children, depth+1)
	}
}

// crumbs joins a breadcrumb, eliding the middle when it doesn't fit: "a › … › d".
func crumbs(cs []string, room int) string {
	short := make([]string, len(cs))
	for i, c := range cs {
		short[i] = c
		if i < len(cs)-1 {
			short[i] = lastPart(c)
		}
	}
	s := strings.Join(short, " › ")
	for width(s) > room && len(short) > 2 {
		short = append([]string{short[0], "…"}, short[min(len(short)-1, 3):]...)
		if len(short) == 3 && short[1] == "…" {
			s = strings.Join(short, " › ")
			break
		}
		s = strings.Join(short, " › ")
	}
	return s
}

// lastPart shortens "ui.Model.layout" to "layout" inside a breadcrumb.
func lastPart(s string) string {
	if i := strings.LastIndex(s, "."); i >= 0 && i < len(s)-1 {
		return s[i+1:]
	}
	return s
}

func firstCallee(s Site) string {
	if len(s.Callees) == 0 {
		return ""
	}
	return s.Callees[0]
}

func (r *renderer) relevant(row siteRow) bool {
	s := row.h
	if row.op == '-' {
		s = row.b
		for _, c := range s.Callees {
			if r.a.relBase[c] {
				return true
			}
		}
		return false
	}
	for _, c := range s.Callees {
		if r.a.relHead[c] {
			return true
		}
	}
	return false
}

func (r *renderer) effOf(row siteRow) Effect {
	a := r.a
	if row.op == '-' {
		return a.siteEff(a.Base, a.effBase, row.b) &^ (EffUnknown | EffEnv)
	}
	return a.siteEff(a.Head, a.effHead, row.h) &^ (EffUnknown | EffEnv)
}

func (r *renderer) effOfBase(row siteRow) Effect {
	return r.a.siteEff(r.a.Base, r.a.effBase, row.b) &^ (EffUnknown | EffEnv)
}

func effDelta(b, h Effect) string {
	var s []string
	if x := h &^ b; x != 0 {
		s = append(s, "+"+x.icons())
	}
	if x := b &^ h; x != 0 {
		s = append(s, "−"+x.icons())
	}
	return strings.Join(s, " ")
}

func ctxNote(c Ctx) string {
	var w []string
	for _, x := range ctxKey(c).words() {
		if x == "if" && c&CtxErr != 0 {
			continue // "err" says it
		}
		w = append(w, x)
	}
	if c&CtxLoop != 0 {
		for i, x := range w {
			if x == "loop" {
				w[i] = "↻"
			}
		}
	}
	return strings.Join(w, " ")
}

// blast is the inverted tree: who calls this frame, up to the entries.
func (r *renderer) blast(id string, depth int, seen map[string]bool) {
	a := r.a
	if depth == 0 {
		fn := a.Head.Funcs[id]
		r.row(" Blast radius ▸ "+fn.Name, fmt.Sprintf("%d flows ", len(entriesOf(a.Head, a.callers, id, false))))
	}
	if depth > 5 {
		r.row(r.indent(depth)+"⋯", "")
		return
	}
	callers := a.callers[id]
	const show = 4
	for i, c := range callers {
		if i == show {
			r.row(fmt.Sprintf("%s⋯ %d more callers", r.indent(depth), len(callers)-show), "")
			break
		}
		fn := a.Head.Funcs[c]
		mark := "↑"
		if fn.Entry {
			mark = "◆"
		}
		label := fn.Name
		if fn.Entry {
			label = flowName(fn)
		}
		right := ""
		if a.Changed[c] {
			right = "changed "
		}
		r.row(fmt.Sprintf("%s%s %s", r.indent(depth), mark, label), right)
		if fn.Entry || seen[c] {
			continue
		}
		seen[c] = true
		r.blast(c, depth+1, seen)
	}
}

package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Analysis is the diff between two loaded versions.
type Analysis struct {
	Base, Head *Program
	Changed    map[string]bool   // head IDs whose body changed, plus added ones
	Removed    map[string]bool   // base IDs that are gone
	Renamed    map[string]string // base ID → head ID
	RenameHint map[string]bool   // head IDs matched only by similarity (a hint, not a fact)
	Refactor   map[string]bool   // changed, but the same calls, contexts and errors
	toBase     map[string]string // head ID → base ID
	relHead    map[string]bool   // head funcs that reach a changed frame
	relBase    map[string]bool
	effHead    *effects
	effBase    *effects
	syncHead   *effects
	syncBase   *effects
	callers    map[string][]string // head reverse edges
	callersB   map[string][]string // base reverse edges
	Findings   []*Finding
}

// Finding is one changed frame, scored for the "look here first" list.
type Finding struct {
	ID       string
	Name     string
	File     string
	Line     int
	Kind     string // "changed", "added", "removed"
	Entries  []string
	Reasons  []string
	Warn     bool
	Score    float64
	NewEff   Effect
	GoneEff  Effect
	Refactor bool
}

func analyze(base, head *Program) *Analysis {
	a := &Analysis{Base: base, Head: head, Changed: map[string]bool{}, Removed: map[string]bool{},
		Renamed: map[string]string{}, RenameHint: map[string]bool{}, Refactor: map[string]bool{}, toBase: map[string]string{}}
	var added []string
	for id, hf := range head.Funcs {
		bf, ok := base.Funcs[id]
		switch {
		case !ok:
			added = append(added, id)
		case bf.BodyHash != hf.BodyHash:
			a.Changed[id] = true
			a.toBase[id] = id
		default:
			a.toBase[id] = id
		}
	}
	for id := range base.Funcs {
		if _, ok := head.Funcs[id]; !ok {
			a.Removed[id] = true
		}
	}
	a.matchRenames(added)
	for _, id := range added {
		a.Changed[id] = true
	}
	for id := range a.Changed {
		if b, ok := a.toBase[id]; ok && sameShape(base.Funcs[b], head.Funcs[id], a) {
			a.Refactor[id] = true
		}
	}
	a.callers = reverse(head)
	a.callersB = reverse(base)
	// Trees only grow toward frames whose behaviour may have changed: refactors don't pull paths in.
	var hc, bc []string
	for _, id := range keys(a.Changed) {
		if a.Refactor[id] {
			continue
		}
		hc = append(hc, id)
		if b, ok := a.toBase[id]; ok {
			bc = append(bc, b)
		}
	}
	a.relHead = reach(a.callers, hc)
	bc = append(bc, keys(a.Removed)...)
	a.relBase = reach(a.callersB, bc)
	a.effHead, a.effBase = newEffects(head, false), newEffects(base, false)
	a.syncHead, a.syncBase = newEffects(head, true), newEffects(base, true)
	a.score()
	return a
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// matchRenames pairs removed with added functions: same body → renamed; similar calls → a rename hint.
func (a *Analysis) matchRenames(added []string) {
	sort.Strings(added)
	byHash := map[string]string{}
	for id := range a.Removed {
		byHash[a.Base.Funcs[id].BodyHash] = id
	}
	var rest []string
	for _, id := range added {
		if b, ok := byHash[a.Head.Funcs[id].BodyHash]; ok && a.Removed[b] {
			a.pair(b, id, false)
			continue
		}
		rest = append(rest, id)
	}
	for _, id := range rest {
		hf := a.Head.Funcs[id]
		best, bestScore := "", 0.0
		for _, b := range keys(a.Removed) {
			bf := a.Base.Funcs[b]
			if bf.Pkg != hf.Pkg || len(bf.Sites) < 3 {
				continue
			}
			if s := jaccard(bf.Sites, hf.Sites); s > bestScore {
				best, bestScore = b, s
			}
		}
		if bestScore >= 0.6 {
			a.pair(best, id, true)
		}
	}
}

func (a *Analysis) pair(b, h string, hint bool) {
	delete(a.Removed, b)
	a.Renamed[b] = h
	a.toBase[h] = b
	if hint {
		a.RenameHint[h] = true
	}
}

func jaccard(x, y []Site) float64 {
	sx, sy := map[string]bool{}, map[string]bool{}
	for _, s := range x {
		sx[s.Label] = true
	}
	for _, s := range y {
		sy[s.Label] = true
	}
	inter := 0
	for k := range sx {
		if sy[k] {
			inter++
		}
	}
	union := len(sx) + len(sy) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func (a *Analysis) baseLabel(s Site) string {
	if s.Kind == SiteCall && len(s.Callees) == 1 {
		if h, ok := a.Renamed[s.Callees[0]]; ok {
			return a.Head.Funcs[h].Name
		}
	}
	return s.Label
}

// sameShape: identical call sequence, contexts and error returns, so the change can't alter behaviour
// at this level (it may still change values passed along: that is the honest limit of the check).
func sameShape(b, h *Func, a *Analysis) bool {
	if b == nil || h == nil || len(b.Sites) != len(h.Sites) {
		return false
	}
	for i := range b.Sites {
		if a.baseLabel(b.Sites[i]) != h.Sites[i].Label || b.Sites[i].Ctx&^CtxRef != h.Sites[i].Ctx&^CtxRef {
			return false
		}
	}
	return true
}

func reverse(p *Program) map[string][]string {
	r := map[string][]string{}
	for id, fn := range p.Funcs {
		for _, s := range fn.Sites {
			if s.Ctx&CtxBind != 0 {
				continue // storing a callback isn't calling it
			}
			for _, c := range s.Callees {
				r[c] = append(r[c], id)
			}
		}
	}
	for k := range r {
		sort.Strings(r[k])
		r[k] = dedup(r[k])
	}
	return r
}

func dedup(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// reach is every function that can reach one of the targets (targets included).
func reach(callers map[string][]string, targets []string) map[string]bool {
	seen := map[string]bool{}
	q := append([]string(nil), targets...)
	for len(q) > 0 {
		id := q[0]
		q = q[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		q = append(q, callers[id]...)
	}
	return seen
}

// entriesOf walks callers up to the nearest entries. syncOnly skips async hops (tea.Cmd, go).
func entriesOf(p *Program, callers map[string][]string, id string, syncOnly bool) []string {
	seen := map[string]bool{}
	var out []string
	q := []string{id}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		if seen[cur] {
			continue
		}
		seen[cur] = true
		if fn := p.Funcs[cur]; fn != nil && fn.Entry {
			out = append(out, cur)
			continue
		}
		for _, c := range callers[cur] {
			if syncOnly && !syncEdge(p.Funcs[c], cur) {
				continue
			}
			q = append(q, c)
		}
	}
	sort.Strings(out)
	return out
}

func syncEdge(caller *Func, callee string) bool {
	if caller == nil {
		return false
	}
	for _, s := range caller.Sites {
		if s.Ctx&CtxAsync != 0 {
			continue
		}
		for _, c := range s.Callees {
			if c == callee {
				return true
			}
		}
	}
	return false
}

func (a *Analysis) score() {
	add := func(f *Finding) { a.Findings = append(a.Findings, f) }
	for _, id := range keys(a.Changed) {
		hf := a.Head.Funcs[id]
		f := &Finding{ID: id, Name: hf.Name, File: hf.File, Line: hf.Line, Kind: "changed"}
		b, hasBase := a.toBase[id]
		if !hasBase {
			f.Kind = "added"
		} else if b != id {
			f.Kind = "renamed"
		}
		f.Entries = entriesOf(a.Head, a.callers, id, false)
		f.Refactor = a.Refactor[id]
		var bf *Func
		if hasBase {
			bf = a.Base.Funcs[b]
		}
		var baseEff, baseSync Effect
		if hasBase {
			baseEff, baseSync = a.effBase.of(b), a.syncBase.of(b)
		}
		headEff, headSync := a.effHead.of(id), a.syncHead.of(id)
		f.NewEff = headEff &^ baseEff
		f.GoneEff = baseEff &^ headEff
		score := 1.0
		if f.Kind == "renamed" {
			if a.RenameHint[id] {
				f.Reasons = append(f.Reasons, "renamed? from "+a.Base.Funcs[b].Name+" (similar calls, hint)")
			} else {
				f.Reasons = append(f.Reasons, "renamed from "+a.Base.Funcs[b].Name)
			}
		}
		if ne := f.NewEff &^ EffUnknown; ne != 0 {
			f.Reasons = append(f.Reasons, "+"+ne.words())
			score += 20 * float64(popcount(uint16(ne)))
		}
		if ge := f.GoneEff &^ EffUnknown; ge != 0 {
			f.Reasons = append(f.Reasons, "−"+ge.words())
			score += 8
		}
		// New blocking work on the UI goroutine is the deck's own rule (fetching runs off the UI goroutine).
		if nb := (headSync &^ baseSync).blocking(); nb != 0 {
			var ui []string
			for _, e := range entriesOf(a.Head, a.callers, id, true) {
				if a.Head.Funcs[e].UI {
					ui = append(ui, e)
				}
			}
			if len(ui) > 0 {
				f.Warn = true
				f.Reasons = append(f.Reasons, fmt.Sprintf("blocks UI (%s) in %d flows", nb.icons(), len(ui)))
				score += 60
			}
		}
		d := diffSites(a, bf, hf)
		var errAdd, errDel, moved, loop int
		for _, r := range d {
			switch {
			case r.op == '+' && (r.h.Kind == SiteErr || r.h.Kind == SitePanic):
				errAdd++
			case r.op == '-' && (r.b.Kind == SiteErr || r.b.Kind == SitePanic):
				errDel++
			case r.op == '~':
				moved++
			}
			if r.op == '+' && r.h.Ctx&CtxLoop != 0 && a.siteEff(a.Head, a.effHead, r.h).blocking() != 0 {
				loop++
			}
		}
		if hasBase && errAdd > 0 {
			f.Reasons = append(f.Reasons, fmt.Sprintf("+%d error path", errAdd))
			score += 6 * float64(errAdd)
		}
		if errDel > 0 {
			f.Reasons = append(f.Reasons, fmt.Sprintf("−%d error path", errDel))
			score += 10 * float64(errDel)
		}
		if moved > 0 {
			f.Reasons = append(f.Reasons, fmt.Sprintf("%d call moved in/out of a branch", moved))
			score += 5 * float64(moved)
		}
		if loop > 0 {
			f.Reasons = append(f.Reasons, fmt.Sprintf("↻ %d blocking call in a loop", loop))
			score += 25 * float64(loop)
		}
		if f.Refactor {
			f.Reasons = []string{"≈ same calls, contexts and errors"}
			score = 0.1
		}
		if len(f.Entries) == 0 {
			f.Reasons = append(f.Reasons, "no entry reaches it")
			score *= 0.3
		}
		f.Score = score * (1 + math.Log2(1+float64(len(f.Entries))))
		add(f)
	}
	for _, id := range keys(a.Removed) {
		bf := a.Base.Funcs[id]
		f := &Finding{ID: id, Name: bf.Name, File: bf.File, Line: bf.Line, Kind: "removed",
			Entries: entriesOf(a.Base, a.callersB, id, false)}
		f.GoneEff = a.effBase.of(id) &^ EffUnknown
		f.Reasons = []string{"removed"}
		if f.GoneEff != 0 {
			f.Reasons = append(f.Reasons, "−"+f.GoneEff.words())
		}
		f.Score = (2 + 8*float64(popcount(uint16(f.GoneEff)))) * (1 + math.Log2(1+float64(len(f.Entries))))
		add(f)
	}
	sort.SliceStable(a.Findings, func(i, j int) bool {
		if a.Findings[i].Score != a.Findings[j].Score {
			return a.Findings[i].Score > a.Findings[j].Score
		}
		return a.Findings[i].ID < a.Findings[j].ID
	})
}

func popcount(x uint16) int {
	n := 0
	for ; x != 0; x &= x - 1 {
		n++
	}
	return n
}

func (a *Analysis) siteEff(p *Program, e *effects, s Site) Effect {
	_ = p
	return e.site(Site{Kind: s.Kind, Callees: s.Callees, Ext: s.Ext}) // ignore ctx: what the call does, wherever it sits
}

// siteRow is one aligned pair from the LCS of a frame's base and head call sites.
type siteRow struct {
	op   byte // ' ' same, '+' added, '-' removed, '~' same call but its context moved
	b, h Site
}

// diffSites aligns two call sequences by LCS on the call label. A call that is
// removed in one place and added in another becomes a reorder ('~').
func diffSites(a *Analysis, bf, hf *Func) []siteRow {
	var bs, hs []Site
	if bf != nil {
		bs = bf.Sites
	}
	if hf != nil {
		hs = hf.Sites
	}
	n, m := len(bs), len(hs)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a.baseLabel(bs[i]) == hs[j].Label {
				dp[i][j] = dp[i+1][j+1] + 1
			} else {
				dp[i][j] = max(dp[i+1][j], dp[i][j+1])
			}
		}
	}
	var rows []siteRow
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && a.baseLabel(bs[i]) == hs[j].Label:
			op := byte(' ')
			if ctxKey(bs[i].Ctx) != ctxKey(hs[j].Ctx) {
				op = '~'
			}
			rows = append(rows, siteRow{op: op, b: bs[i], h: hs[j]})
			i++
			j++
		case j < m && (i == n || dp[i][j+1] >= dp[i+1][j]):
			rows = append(rows, siteRow{op: '+', h: hs[j]})
			j++
		default:
			rows = append(rows, siteRow{op: '-', b: bs[i]})
			i++
		}
	}
	// Pair a removed and an added call with the same label: it moved (reordered).
	for x := range rows {
		if rows[x].op != '-' {
			continue
		}
		for y := range rows {
			if rows[y].op == '+' && rows[y].h.Label == a.baseLabel(rows[x].b) {
				rows[y] = siteRow{op: '~', b: rows[x].b, h: rows[y].h}
				rows[x].op = 'x' // dropped: shown at its new place
				break
			}
		}
	}
	out := rows[:0]
	for _, r := range rows {
		if r.op != 'x' {
			out = append(out, r)
		}
	}
	return out
}

func ctxKey(c Ctx) Ctx { return c &^ (CtxRef | CtxClosure | CtxBind) }

func moveNote(b, h Site) string {
	bw, hw := ctxKey(b.Ctx), ctxKey(h.Ctx)
	var parts []string
	for _, p := range []struct {
		c    Ctx
		word string
	}{{CtxLoop, "loop"}, {CtxCond, "branch"}, {CtxErr, "error branch"}, {CtxAsync, "async"}, {CtxDefer, "defer"}} {
		switch {
		case hw&p.c != 0 && bw&p.c == 0:
			parts = append(parts, "into "+p.word)
		case bw&p.c != 0 && hw&p.c == 0:
			parts = append(parts, "out of "+p.word)
		}
	}
	if len(parts) == 0 {
		return "reordered"
	}
	return "moved " + strings.Join(parts, ", ")
}

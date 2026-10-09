package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Ctx is where a call sits inside its function.
type Ctx uint16

const (
	CtxLoop    Ctx = 1 << iota // inside for / range
	CtxCond                    // inside if / switch / select body
	CtxErr                     // inside an `if err != nil` body
	CtxDefer                   // deferred
	CtxAsync                   // inside a goroutine or a tea.Cmd closure: off the caller's goroutine
	CtxClosure                 // inside some other func literal
	CtxRef                     // a function value handed on (e.g. returned as a tea.Cmd), not a call
	CtxBind                    // a callback stored in a struct field: called later by whoever calls the field
)

func (c Ctx) words() []string {
	var w []string
	for _, p := range []struct {
		c Ctx
		s string
	}{{CtxLoop, "loop"}, {CtxCond, "if"}, {CtxErr, "err"}, {CtxDefer, "defer"}, {CtxAsync, "async"}} {
		if c&p.c != 0 {
			w = append(w, p.s)
		}
	}
	return w
}

// SiteKind separates real calls from the pseudo-sites the diff also tracks.
type SiteKind uint8

const (
	SiteCall  SiteKind = iota // call to a function in the module
	SiteExt                   // call outside the module (stdlib or dependency)
	SiteDyn                   // call through a func value that static analysis can't resolve
	SiteErr                   // returns a non-nil error
	SitePanic                 // panic(...)
)

// Site is one call (or error return) in a function body, in source order.
type Site struct {
	Kind    SiteKind
	Callees []string // module function IDs (several for an interface call)
	Label   string   // display and diff key: "readDiff", "os.ReadFile", "return fmt.Errorf(…)"
	Ext     string   // fully qualified external name for the rule table
	Ctx     Ctx
	Line    int
	field   string // for SiteDyn through a struct field: resolved after loading
}

// Func is one function (or one switch case of a split function) in one version of the tree.
type Func struct {
	ID       string // stable: "pkgpath.Name", "pkgpath.Type.Method", "…#case X"
	Name     string // short display name
	Pkg      string
	File     string // repo-relative
	Line     int
	EndLine  int
	Sites    []Site
	BodyHash string
	Entry    bool // an entry point (from the entry config)
	UI       bool // runs on the UI goroutine (Bubble Tea Update/View)
	Split    bool // a pseudo-function made from a switch case
}

// Program is one loaded version (base or head).
type Program struct {
	Root  string
	Funcs map[string]*Func
	// methods by name, for resolving interface calls to the module's implementations.
	methods map[string][]methodImpl
	module  string
	// fieldFuncs: functions assigned to a func-typed struct field (callbacks, injected readers),
	// so a call through the field can be resolved. A cheap, field-based stand-in for VTA.
	fieldFuncs map[string][]string
}

type methodImpl struct {
	id   string
	recv types.Type
}

// EntryConfig says which functions are entry points. Split functions have
// each case of their top-level switch turned into its own entry.
type EntryConfig struct {
	Funcs []*regexp.Regexp // whole functions that are entries
	Split []*regexp.Regexp // functions whose switch cases are entries
	UI    []*regexp.Regexp // entries on the UI goroutine
}

func defaultEntries() EntryConfig {
	re := regexp.MustCompile
	return EntryConfig{
		Funcs: []*regexp.Regexp{re(`^main\.main$`), re(`\.Model\.(View|Init)$`)},
		Split: []*regexp.Regexp{re(`\.Model\.(Update|handleKey)$`)},
		UI:    []*regexp.Regexp{re(`\.Model\.`)},
	}
}

// materialize writes the tree at rev into dir with git archive (read-only on the repo).
func materialize(repo, rev, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	arch := exec.Command("git", "-C", repo, "archive", rev)
	untar := exec.Command("tar", "-x", "-C", dir)
	pipe, err := arch.StdoutPipe()
	if err != nil {
		return err
	}
	untar.Stdin = pipe
	if err := untar.Start(); err != nil {
		return err
	}
	if err := arch.Run(); err != nil {
		return fmt.Errorf("git archive %s: %w", rev, err)
	}
	return untar.Wait()
}

func load(root string, cfg EntryConfig) (*Program, error) {
	pcfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedModule,
		Dir:   root,
		Tests: false,
		Env:   append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off"),
	}
	pkgs, err := packages.Load(pcfg, "./...")
	if err != nil {
		return nil, err
	}
	p := &Program{Root: root, Funcs: map[string]*Func{}, methods: map[string][]methodImpl{}, fieldFuncs: map[string][]string{}}
	for _, pkg := range pkgs {
		for _, e := range pkg.Errors {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", pkg.PkgPath, e)
		}
		if pkg.Module != nil && p.module == "" {
			p.module = pkg.Module.Path
		}
	}
	// Index methods first so interface calls can resolve while walking bodies.
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, f := range pkg.Syntax {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Recv == nil {
					continue
				}
				obj, _ := pkg.TypesInfo.Defs[fd.Name].(*types.Func)
				if obj == nil {
					continue
				}
				recv := obj.Type().(*types.Signature).Recv().Type()
				p.methods[fd.Name.Name] = append(p.methods[fd.Name.Name], methodImpl{funcID(obj), recv})
			}
		}
	}
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, f := range pkg.Syntax {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				obj, _ := pkg.TypesInfo.Defs[fd.Name].(*types.Func)
				if obj == nil {
					continue
				}
				p.addFunc(pkg, fd, obj, cfg)
			}
		}
	}
	p.resolveFields()
	return p, nil
}

func (p *Program) resolveFields() {
	for _, fn := range p.Funcs {
		for i, s := range fn.Sites {
			if s.Kind != SiteDyn || s.field == "" {
				continue
			}
			if ids := p.fieldFuncs[s.field]; len(ids) > 0 {
				fn.Sites[i].Kind = SiteCall
				fn.Sites[i].Callees = dedup(append([]string(nil), ids...))
			}
		}
	}
}

func (p *Program) rel(fset *token.FileSet, pos token.Pos) (string, int) {
	ps := fset.Position(pos)
	r, err := filepath.Rel(p.Root, ps.Filename)
	if err != nil {
		r = ps.Filename
	}
	return r, ps.Line
}

func (p *Program) addFunc(pkg *packages.Package, fd *ast.FuncDecl, obj *types.Func, cfg EntryConfig) {
	id := funcID(obj)
	file, line := p.rel(pkg.Fset, fd.Pos())
	_, end := p.rel(pkg.Fset, fd.End())
	fn := &Func{ID: id, Name: shortName(obj), Pkg: pkg.PkgPath, File: file, Line: line, EndLine: end,
		BodyHash: hashNode(pkg.Fset, fd.Body)}
	fn.Entry = matchAny(cfg.Funcs, id) || matchAny(cfg.Funcs, fn.Name)
	fn.UI = matchAny(cfg.UI, id)
	w := &walker{p: p, pkg: pkg, fn: id, file: file}
	if matchAny(cfg.Split, id) {
		// The function itself keeps what's outside its top-level switches;
		// each case becomes an entry of its own, reached from the function.
		var rest []ast.Stmt
		var labels []string
		for _, st := range fd.Body.List {
			clauses, ok := switchClauses(st)
			if !ok {
				rest = append(rest, st)
				w.walk(st, 0)
				continue
			}
			for _, cc := range clauses {
				label := caseLabel(pkg, cc)
				labels = append(labels, label)
				cf := &Func{ID: id + "#case " + label, Name: fn.Name + " ▸ " + label, Pkg: pkg.PkgPath,
					File: file, Split: true, Entry: true, UI: fn.UI}
				cf.File, cf.Line = p.rel(pkg.Fset, cc.Pos())
				_, cf.EndLine = p.rel(pkg.Fset, cc.End())
				cw := &walker{p: p, pkg: pkg, fn: id, file: file}
				for _, s := range cc.Body {
					cw.walk(s, 0)
				}
				cf.Sites = cw.sites
				cf.BodyHash = hashStmts(pkg.Fset, cc.Body)
				p.Funcs[cf.ID] = cf
				w.sites = append(w.sites, Site{Kind: SiteCall, Callees: []string{cf.ID}, Label: "▸ " + label, Ctx: CtxCond, Line: cf.Line})
			}
		}
		// The split function's own hash leaves its cases out: they are frames of their own.
		fn.BodyHash = hashStmts(pkg.Fset, rest) + hashString(strings.Join(labels, "\n"))
	} else {
		w.walk(fd.Body, 0)
	}
	fn.Sites = w.sites
	p.Funcs[id] = fn
}

type caseClause struct {
	pos, end token.Pos
	list     []ast.Expr
	Body     []ast.Stmt
}

func (c caseClause) Pos() token.Pos { return c.pos }
func (c caseClause) End() token.Pos { return c.end }

func switchClauses(st ast.Stmt) ([]caseClause, bool) {
	var body *ast.BlockStmt
	switch s := st.(type) {
	case *ast.SwitchStmt:
		body = s.Body
	case *ast.TypeSwitchStmt:
		body = s.Body
	default:
		return nil, false
	}
	var out []caseClause
	for _, c := range body.List {
		cc := c.(*ast.CaseClause)
		out = append(out, caseClause{cc.Pos(), cc.End(), cc.List, cc.Body})
	}
	return out, true
}

var keysRe = regexp.MustCompile(`keys\.(\w+)`)

func caseLabel(pkg *packages.Package, cc caseClause) string {
	if len(cc.list) == 0 {
		return "default"
	}
	var parts []string
	for _, e := range cc.list {
		s := exprString(pkg.Fset, e)
		if m := keysRe.FindStringSubmatch(s); m != nil {
			s = "key " + m[1]
		}
		parts = append(parts, s)
	}
	s := strings.Join(parts, ", ")
	if len(s) > 40 {
		s = s[:39] + "…"
	}
	return s
}

func matchAny(rs []*regexp.Regexp, s string) bool {
	for _, r := range rs {
		if r.MatchString(s) {
			return true
		}
	}
	return false
}

func funcID(f *types.Func) string {
	pkg := ""
	if f.Pkg() != nil {
		pkg = f.Pkg().Path()
	}
	sig, _ := f.Type().(*types.Signature)
	if sig != nil && sig.Recv() != nil {
		return pkg + "." + recvName(sig.Recv().Type()) + "." + f.Name()
	}
	return pkg + "." + f.Name()
}

func recvName(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	switch n := t.(type) {
	case *types.Named:
		return n.Obj().Name()
	case *types.Alias:
		return n.Obj().Name()
	}
	return t.String()
}

func shortName(f *types.Func) string {
	pkg := ""
	if f.Pkg() != nil {
		pkg = f.Pkg().Name()
	}
	sig, _ := f.Type().(*types.Signature)
	if sig != nil && sig.Recv() != nil {
		return pkg + "." + recvName(sig.Recv().Type()) + "." + f.Name()
	}
	return pkg + "." + f.Name()
}

func hashNode(fset *token.FileSet, n ast.Node) string {
	var b bytes.Buffer
	_ = printer.Fprint(&b, fset, n) // comments are only printed for *ast.File / CommentedNode
	s := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(s[:8])
}

func hashStmts(fset *token.FileSet, ss []ast.Stmt) string {
	var b bytes.Buffer
	for _, s := range ss {
		_ = printer.Fprint(&b, fset, s)
		b.WriteByte('\n')
	}
	s := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(s[:8])
}

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

func exprString(fset *token.FileSet, e ast.Expr) string {
	var b bytes.Buffer
	_ = printer.Fprint(&b, fset, e)
	return strings.Join(strings.Fields(b.String()), " ")
}

// walker collects call sites in source order with their context.
type walker struct {
	p     *Program
	pkg   *packages.Package
	fn    string // enclosing function ID, for naming callback pseudo-functions
	file  string
	sites []Site
	alias map[*types.Var]string // local variables holding a func-typed field: `start := m.opt.StartDev`
}

// fieldKey identifies a struct field within one loaded program.
func (w *walker) fieldKey(v *types.Var) string {
	return w.pkg.Fset.Position(v.Pos()).String()
}

// fieldOf returns the func-typed struct field an expression names, if any.
func (w *walker) fieldOf(e ast.Expr) *types.Var {
	var v *types.Var
	switch e := ast.Unparen(e).(type) {
	case *ast.SelectorExpr:
		if sel, ok := w.pkg.TypesInfo.Selections[e]; ok && sel.Kind() == types.FieldVal {
			v, _ = sel.Obj().(*types.Var)
		}
	case *ast.Ident:
		v, _ = w.pkg.TypesInfo.Uses[e].(*types.Var)
		if v != nil && !v.IsField() {
			v = nil
		}
	}
	if v == nil {
		return nil
	}
	if _, ok := v.Type().Underlying().(*types.Signature); !ok {
		return nil
	}
	return v
}

// bind records that rhs is assigned to the func-typed field v. A func literal
// becomes a pseudo-function of its own, so its calls run when the field is called,
// not when it is assigned.
func (w *walker) bind(v *types.Var, rhs ast.Expr, ctx Ctx) {
	key := w.fieldKey(v)
	if lit, ok := ast.Unparen(rhs).(*ast.FuncLit); ok {
		id := w.fn + "$" + v.Name()
		for n := 2; w.p.Funcs[id] != nil; n++ {
			id = fmt.Sprintf("%s$%s#%d", w.fn, v.Name(), n)
		}
		cw := &walker{p: w.p, pkg: w.pkg, fn: id, file: w.file}
		cw.walk(lit.Body, 0)
		file, line := w.p.rel(w.pkg.Fset, lit.Pos())
		_, end := w.p.rel(w.pkg.Fset, lit.End())
		short := w.fn[strings.LastIndex(w.fn, "/")+1:]
		w.p.Funcs[id] = &Func{ID: id, Name: short + "$" + v.Name(), Pkg: w.pkg.PkgPath, File: file, Line: line, EndLine: end,
			Sites: cw.sites, BodyHash: hashNode(w.pkg.Fset, lit.Body)}
		w.p.fieldFuncs[key] = append(w.p.fieldFuncs[key], id)
		w.sites = append(w.sites, Site{Kind: SiteCall, Callees: []string{id}, Label: "⇐ " + v.Name(), Ctx: ctx | CtxRef | CtxAsync | CtxBind, Line: line})
		return
	}
	if f := w.funcObj(rhs); f != nil && w.inModule(f) {
		w.p.fieldFuncs[key] = append(w.p.fieldFuncs[key], funcID(f))
	}
	w.walk(rhs, ctx)
}

func (w *walker) line(pos token.Pos) int { return w.pkg.Fset.Position(pos).Line }

func (w *walker) walk(n ast.Node, ctx Ctx) {
	if n == nil {
		return
	}
	switch s := n.(type) {
	case *ast.ForStmt:
		w.walk(s.Init, ctx)
		w.walk(s.Cond, ctx|CtxLoop)
		w.walk(s.Post, ctx|CtxLoop)
		w.walk(s.Body, ctx|CtxLoop)
		return
	case *ast.RangeStmt:
		w.walk(s.X, ctx)
		w.walk(s.Body, ctx|CtxLoop)
		return
	case *ast.IfStmt:
		w.walk(s.Init, ctx)
		w.walk(s.Cond, ctx)
		body := ctx | CtxCond
		if isErrCheck(s.Cond) {
			body |= CtxErr
		}
		w.walk(s.Body, body)
		w.walk(s.Else, ctx|CtxCond)
		return
	case *ast.CaseClause:
		for _, e := range s.List {
			w.walk(e, ctx)
		}
		for _, st := range s.Body {
			w.walk(st, ctx|CtxCond)
		}
		return
	case *ast.CommClause:
		w.walk(s.Comm, ctx|CtxCond)
		for _, st := range s.Body {
			w.walk(st, ctx|CtxCond)
		}
		return
	case *ast.DeferStmt:
		w.call(s.Call, ctx|CtxDefer)
		return
	case *ast.GoStmt:
		w.call(s.Call, ctx|CtxAsync)
		return
	case *ast.FuncLit:
		c := ctx | CtxClosure
		if w.isCmd(s) {
			c |= CtxAsync
		}
		w.walk(s.Body, c)
		return
	case *ast.ReturnStmt:
		for _, r := range s.Results {
			w.walk(r, ctx)
		}
		if ctx&CtxClosure == 0 && len(s.Results) > 0 {
			last := s.Results[len(s.Results)-1]
			if w.isNonNilError(last) {
				w.sites = append(w.sites, Site{Kind: SiteErr, Label: "↯ return " + errLabel(w.pkg.Fset, last), Ctx: ctx, Line: w.line(s.Pos())})
			}
		}
		return
	case *ast.CallExpr:
		w.call(s, ctx)
		return
	case *ast.AssignStmt:
		for i, l := range s.Lhs {
			if i < len(s.Rhs) && len(s.Lhs) == len(s.Rhs) {
				if id, ok := l.(*ast.Ident); ok {
					if lv, _ := w.pkg.TypesInfo.ObjectOf(id).(*types.Var); lv != nil && !lv.IsField() {
						if fv := w.fieldOf(s.Rhs[i]); fv != nil {
							if w.alias == nil {
								w.alias = map[*types.Var]string{}
							}
							w.alias[lv] = w.fieldKey(fv)
						}
					}
				}
				if v := w.fieldOf(l); v != nil {
					w.bind(v, s.Rhs[i], ctx)
					continue
				}
				w.walk(s.Rhs[i], ctx)
			}
		}
		if len(s.Lhs) != len(s.Rhs) {
			for _, r := range s.Rhs {
				w.walk(r, ctx)
			}
		}
		return
	case *ast.KeyValueExpr:
		if k, ok := s.Key.(*ast.Ident); ok {
			if v := w.fieldOf(k); v != nil {
				w.bind(v, s.Value, ctx)
				return
			}
		}
		w.walk(s.Value, ctx)
		return
	case *ast.SelectorExpr, *ast.Ident:
		// A module function used as a value: a tea.Cmd or callback handed on.
		if f := w.funcObj(s.(ast.Expr)); f != nil && w.inModule(f) {
			w.sites = append(w.sites, Site{Kind: SiteCall, Callees: []string{funcID(f)}, Label: shortName(f), Ctx: ctx | CtxRef | CtxAsync, Line: w.line(s.Pos())})
		}
		if sel, ok := s.(*ast.SelectorExpr); ok {
			w.walk(sel.X, ctx)
		}
		return
	}
	ast.Inspect(n, func(c ast.Node) bool {
		if c == nil || c == n {
			return true
		}
		switch c.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.IfStmt, *ast.CaseClause, *ast.CommClause,
			*ast.DeferStmt, *ast.GoStmt, *ast.FuncLit, *ast.ReturnStmt, *ast.CallExpr,
			*ast.SelectorExpr, *ast.Ident, *ast.AssignStmt, *ast.KeyValueExpr:
			w.walk(c, ctx)
			return false
		}
		return true
	})
}

func (w *walker) call(c *ast.CallExpr, ctx Ctx) {
	// Arguments run before the call itself.
	for _, a := range c.Args {
		w.walk(a, ctx)
	}
	fun := ast.Unparen(c.Fun)
	if ix, ok := fun.(*ast.IndexExpr); ok {
		fun = ix.X
	}
	if lit, ok := fun.(*ast.FuncLit); ok {
		w.walk(lit.Body, ctx|CtxClosure)
		return
	}
	line := w.line(c.Pos())
	if tv, ok := w.pkg.TypesInfo.Types[fun]; ok && tv.IsType() {
		w.walk(fun, ctx) // a conversion
		return
	}
	if sel, ok := fun.(*ast.SelectorExpr); ok {
		w.walkRecv(sel.X, ctx)
	}
	if id, ok := fun.(*ast.Ident); ok {
		if b, ok := w.pkg.TypesInfo.Uses[id].(*types.Builtin); ok {
			if b.Name() == "panic" {
				w.sites = append(w.sites, Site{Kind: SitePanic, Label: "↯ panic", Ctx: ctx, Line: line})
			}
			return
		}
	}
	f := w.funcObj(fun)
	if f == nil {
		site := Site{Kind: SiteDyn, Label: "ƒ " + trim(exprString(w.pkg.Fset, fun), 28), Ctx: ctx, Line: line}
		if v := w.fieldOf(fun); v != nil {
			site.field = w.fieldKey(v)
			site.Label = "ƒ " + v.Name()
		} else if id, ok := fun.(*ast.Ident); ok {
			if lv, _ := w.pkg.TypesInfo.Uses[id].(*types.Var); lv != nil && w.alias[lv] != "" {
				site.field = w.alias[lv]
			}
		}
		w.sites = append(w.sites, site)
		return
	}
	if w.inModule(f) {
		sig := f.Type().(*types.Signature)
		if sig.Recv() != nil && types.IsInterface(sig.Recv().Type()) {
			ids := w.p.implementations(f)
			if len(ids) == 0 {
				w.sites = append(w.sites, Site{Kind: SiteDyn, Label: "ƒ " + shortName(f), Ctx: ctx, Line: line})
				return
			}
			w.sites = append(w.sites, Site{Kind: SiteCall, Callees: ids, Label: shortName(f), Ctx: ctx, Line: line})
			return
		}
		w.sites = append(w.sites, Site{Kind: SiteCall, Callees: []string{funcID(f)}, Label: shortName(f), Ctx: ctx, Line: line})
		return
	}
	ext := funcID(f)
	label := shortName(f)
	if ext == ".error.Error" || strings.HasSuffix(ext, "fmt.Stringer.String") {
		// Resolving these to every implementation in the module only adds noise.
		w.sites = append(w.sites, Site{Kind: SiteExt, Label: label, Ext: ext, Ctx: ctx, Line: line})
		return
	}
	if sig := f.Type().(*types.Signature); sig.Recv() != nil && types.IsInterface(sig.Recv().Type()) {
		// An interface from outside the module may still be implemented inside it.
		if ids := w.p.implementations(f); len(ids) > 0 {
			w.sites = append(w.sites, Site{Kind: SiteCall, Callees: ids, Label: label, Ctx: ctx, Line: line})
			return
		}
	}
	w.sites = append(w.sites, Site{Kind: SiteExt, Label: label, Ext: ext, Ctx: ctx, Line: line})
}

// walkRecv walks a call's receiver expression without recording the receiver itself as a reference.
func (w *walker) walkRecv(x ast.Expr, ctx Ctx) {
	switch x := ast.Unparen(x).(type) {
	case *ast.Ident:
		return
	case *ast.SelectorExpr:
		w.walkRecv(x.X, ctx)
	default:
		w.walk(x, ctx)
	}
}

func (w *walker) funcObj(e ast.Expr) *types.Func {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		f, _ := w.pkg.TypesInfo.Uses[e].(*types.Func)
		return f
	case *ast.SelectorExpr:
		if sel, ok := w.pkg.TypesInfo.Selections[e]; ok {
			if sel.Kind() == types.FieldVal {
				return nil
			}
			f, _ := sel.Obj().(*types.Func)
			return f
		}
		f, _ := w.pkg.TypesInfo.Uses[e.Sel].(*types.Func)
		return f
	}
	return nil
}

func (w *walker) inModule(f *types.Func) bool {
	return f.Pkg() != nil && w.p.module != "" && strings.HasPrefix(f.Pkg().Path(), w.p.module)
}

// isCmd reports whether a func literal is a Bubble Tea command (func() tea.Msg), which runs off the UI goroutine.
func (w *walker) isCmd(lit *ast.FuncLit) bool {
	tv, ok := w.pkg.TypesInfo.Types[lit]
	if !ok {
		return false
	}
	sig, ok := tv.Type.(*types.Signature)
	if !ok || sig.Params().Len() != 0 || sig.Results().Len() != 1 {
		return false
	}
	return strings.Contains(sig.Results().At(0).Type().String(), "bubbletea")
}

var errType = types.Universe.Lookup("error").Type()

func (w *walker) isNonNilError(e ast.Expr) bool {
	if id, ok := e.(*ast.Ident); ok && id.Name == "nil" {
		return false
	}
	tv, ok := w.pkg.TypesInfo.Types[e]
	return ok && tv.Type != nil && types.Identical(tv.Type, errType)
}

func isErrCheck(e ast.Expr) bool {
	b, ok := e.(*ast.BinaryExpr)
	if !ok || b.Op != token.NEQ {
		return false
	}
	id, ok := b.X.(*ast.Ident)
	n, ok2 := b.Y.(*ast.Ident)
	return ok && ok2 && n.Name == "nil" && strings.Contains(strings.ToLower(id.Name), "err")
}

func errLabel(fset *token.FileSet, e ast.Expr) string {
	if c, ok := e.(*ast.CallExpr); ok {
		s := exprString(fset, c.Fun)
		if len(c.Args) > 0 {
			if bl, ok := c.Args[0].(*ast.BasicLit); ok {
				return s + "(" + trim(bl.Value, 24) + ")"
			}
		}
		return s + "(…)"
	}
	return trim(exprString(fset, e), 24)
}

func trim(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// implementations resolves an interface method to the module's methods that implement it (class hierarchy analysis).
func (p *Program) implementations(m *types.Func) []string {
	iface, _ := m.Type().(*types.Signature).Recv().Type().Underlying().(*types.Interface)
	if iface == nil {
		return nil
	}
	var ids []string
	for _, impl := range p.methods[m.Name()] {
		t := impl.recv
		if types.Implements(t, iface) || types.Implements(types.NewPointer(t), iface) {
			ids = append(ids, impl.id)
		}
	}
	sort.Strings(ids)
	return ids
}

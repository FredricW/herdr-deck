package arch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// facts is what one source file contributes to the graph. It depends only
// on the blob's content, so it is cached by blob SHA: base and head share
// every file the change did not touch, and a later commit reuses them too.
type facts struct {
	pkg     string // Go: the package clause; "ts" for TypeScript
	ignored bool   // //go:build ignore, or it did not parse
	imports []imp
	decls   []decl
	touches []touch
}

// imp is one import: the path as written, its line, and what the file uses
// through it, sorted ("Snapshot Thread"): the edge's fingerprint.
type imp struct {
	path string
	line int
	uses string
}

// decl is one top-level declaration: its key (kind, receiver, name), a
// printable signature for exported ones, and a hash of its whole text
// without comments, to tell a move from a change.
type decl struct {
	key      string // "func Name", "method T.Name", "type Name", "var Name", "const Name"
	exported bool
	sig      string
	hash     string
	shape    string // hash of the text with the declared name blanked, to match a rename
	line     int
}

// touch is a place where the code reaches outside itself.
type touch struct {
	kind   string // env, exec, http, sql, flag, fs
	target string // the variable, binary, host, table, flag; "" when not a literal
	line   int
}

func (t touch) key() string { return t.kind + " " + t.target }

var buildIgnore = regexp.MustCompile(`(?m)^//go:build\s+ignore\b`)

// parseGo parses one Go file fully. A few hundred files take tens of
// milliseconds, so there is no imports-only fast path.
func parseGo(src []byte) *facts {
	g := &facts{}
	if buildIgnore.Match(src) {
		g.ignored = true
		return g
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, parser.SkipObjectResolution)
	if err != nil {
		g.ignored = true
		return g
	}
	g.pkg = f.Name.Name
	consts := map[string]string{} // top-level string constants, for env names
	local := map[string]string{}  // import name -> path
	for _, s := range f.Imports {
		p, _ := strconv.Unquote(s.Path.Value)
		g.imports = append(g.imports, imp{path: p, line: fset.Position(s.Pos()).Line})
		name := path.Base(p)
		if len(name) > 1 && name[0] == 'v' && name[1] >= '0' && name[1] <= '9' {
			name = path.Base(path.Dir(p)) // charm.land/lipgloss/v2 -> lipgloss
		}
		if s.Name != nil {
			name = s.Name.Name
		}
		local[name] = p
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			g.decls = append(g.decls, funcDecl(fset, d))
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					full := hash(render(fset, s))
					name := s.Name
					s.Name = ast.NewIdent("_")
					shape := hash(render(fset, s))
					s.Name = name
					g.decls = append(g.decls, decl{key: "type " + name.Name, exported: name.IsExported(),
						sig: "type " + render(fset, publicView(s)), hash: full, shape: shape, line: fset.Position(s.Pos()).Line})
				case *ast.ValueSpec:
					kind := "var"
					if d.Tok == token.CONST {
						kind = "const"
					}
					text := hash(render(fset, s))
					for i, n := range s.Names {
						if n.Name == "_" {
							continue
						}
						g.decls = append(g.decls, decl{key: kind + " " + n.Name, exported: n.IsExported(),
							sig: kind + " " + n.Name + typeSuffix(fset, s.Type), hash: text, shape: text, line: fset.Position(n.Pos()).Line})
						if kind == "const" && i < len(s.Values) {
							if v, ok := strLit(s.Values[i]); ok {
								consts[n.Name] = v
							}
						}
					}
				}
			}
		}
	}
	g.touches = goTouches(fset, f, local, consts)
	// What each import is used for: the selectors on its name.
	used := map[string]map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok {
				if p, ok := local[x.Name]; ok {
					if used[p] == nil {
						used[p] = map[string]bool{}
					}
					used[p][sel.Sel.Name] = true
				}
			}
		}
		return true
	})
	for i, im := range g.imports {
		names := make([]string, 0, len(used[im.path]))
		for n := range used[im.path] {
			names = append(names, n)
		}
		slices.Sort(names)
		g.imports[i].uses = strings.Join(names, " ")
	}
	return g
}

// funcDecl is a function or method: its signature without the body, and
// hashes of its whole text and of its text with the name blanked.
func funcDecl(fset *token.FileSet, d *ast.FuncDecl) decl {
	key, exp := "func "+d.Name.Name, d.Name.IsExported()
	if d.Recv != nil && len(d.Recv.List) > 0 {
		r := recvName(d.Recv.List[0].Type)
		key = "method " + r + "." + d.Name.Name
		exp = exp && ast.IsExported(r)
	}
	body := d.Body
	d.Body = nil
	sig := render(fset, d)
	d.Body = body
	full := hash(render(fset, d))
	name := d.Name
	d.Name = ast.NewIdent("_")
	shape := hash(render(fset, d))
	d.Name = name
	return decl{key: key, exported: exp, sig: sig, hash: full, shape: shape, line: fset.Position(d.Pos()).Line}
}

// publicView is a type as another package sees it: a struct without its
// unexported fields, so adding a private field is not an API change.
func publicView(s *ast.TypeSpec) *ast.TypeSpec {
	st, ok := s.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return s
	}
	var fields []*ast.Field
	for _, f := range st.Fields.List {
		var names []*ast.Ident
		for _, n := range f.Names {
			if n.IsExported() {
				names = append(names, n)
			}
		}
		if len(f.Names) == 0 || len(names) > 0 {
			c := *f
			c.Names = names
			c.Doc, c.Comment = nil, nil
			fields = append(fields, &c)
		}
	}
	c := *s
	c.Type = &ast.StructType{Struct: st.Struct, Fields: &ast.FieldList{Opening: st.Fields.Opening, List: fields, Closing: st.Fields.Closing}}
	return &c
}

func recvName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvName(t.X)
	case *ast.IndexExpr:
		return recvName(t.X)
	case *ast.IndexListExpr:
		return recvName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return "?"
}

func typeSuffix(fset *token.FileSet, t ast.Expr) string {
	if t == nil {
		return ""
	}
	return " " + render(fset, t)
}

func render(fset *token.FileSet, n any) string {
	var b bytes.Buffer
	_ = printer.Fprint(&b, fset, n)
	return b.String()
}

func hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

func strLit(e ast.Expr) (string, bool) {
	l, ok := e.(*ast.BasicLit)
	if !ok || l.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(l.Value)
	return v, err == nil
}

// goRules are the touchpoint rules: a call to one of these functions,
// resolved through the file's imports, is a touchpoint of that kind; its
// target is the argument at index arg when that is a string literal or a
// constant of the file.
var goRules = []struct {
	pkg, fn, kind string
	arg           int
}{
	{"os", "Getenv", "env", 0},
	{"os", "LookupEnv", "env", 0},
	{"os", "Setenv", "env", 0},
	{"os/exec", "Command", "exec", 0},
	{"os/exec", "CommandContext", "exec", 1},
	{"net/http", "Get", "http", 0},
	{"net/http", "Post", "http", 0},
	{"net/http", "NewRequest", "http", 1},
	{"net/http", "NewRequestWithContext", "http", 2},
	{"flag", "String", "flag", 0},
	{"flag", "Bool", "flag", 0},
	{"flag", "Int", "flag", 0},
	{"flag", "Duration", "flag", 0},
	{"os", "WriteFile", "fs", -1},
	{"os", "Create", "fs", -1},
	{"os", "MkdirAll", "fs", -1},
	{"os", "Remove", "fs", -1},
	{"os", "RemoveAll", "fs", -1},
	{"os", "Rename", "fs", -1},
}

// sqlTable finds table names in string literals that read as SQL.
var (
	sqlTable = regexp.MustCompile(`(?is)\b(?:select\b.*?\bfrom|insert\s+into|update|delete\s+from|join)\s+["` + "`" + `]?([a-z_][a-z0-9_.]*)`)
	sqlLike  = regexp.MustCompile(`(?i)^\s*(select|insert|update|delete|with)\b`)
)

func goTouches(fset *token.FileSet, f *ast.File, local, consts map[string]string) []touch {
	var ts []touch
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			sel, ok := n.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			x, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			p := local[x.Name]
			for _, r := range goRules {
				if r.pkg != p || r.fn != sel.Sel.Name {
					continue
				}
				t := touch{kind: r.kind, line: fset.Position(n.Pos()).Line}
				if r.arg >= 0 && r.arg < len(n.Args) {
					t.target = argValue(n.Args[r.arg], consts)
					if r.kind == "http" {
						t.target = host(t.target)
					}
				}
				ts = append(ts, t)
			}
		case *ast.BasicLit:
			if v, ok := strLit(n); ok && sqlLike.MatchString(v) {
				for _, m := range sqlTable.FindAllStringSubmatch(v, -1) {
					ts = append(ts, touch{kind: "sql", target: strings.ToLower(m[1]), line: fset.Position(n.Pos()).Line})
				}
			}
		}
		return true
	})
	return ts
}

func argValue(e ast.Expr, consts map[string]string) string {
	if v, ok := strLit(e); ok {
		return v
	}
	if id, ok := e.(*ast.Ident); ok {
		return consts[id.Name]
	}
	return ""
}

// host is a URL's host, or the text itself when it is not a URL.
func host(u string) string {
	if _, rest, ok := strings.Cut(u, "://"); ok {
		h, _, _ := strings.Cut(rest, "/")
		return h
	}
	return u
}

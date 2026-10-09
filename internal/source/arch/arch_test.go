package arch

import (
	"context"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// The fixtures are a made-up webshop: a Go module and a TypeScript
// monorepo, written into a fresh repository per test.

// repo is a scratch git repository.
type repo struct {
	t   *testing.T
	dir string
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	r := &repo{t: t, dir: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	return r
}

func (r *repo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// write writes files (path → content; "" deletes) and commits them.
func (r *repo) commit(msg string, files map[string]string) string {
	r.t.Helper()
	for p, c := range files {
		full := filepath.Join(r.dir, p)
		if c == "" {
			if err := os.Remove(full); err != nil {
				r.t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			r.t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			r.t.Fatal(err)
		}
	}
	r.git("add", "-A")
	r.git("commit", "-q", "-m", msg)
	return r.git("rev-parse", "HEAD")
}

var goBase = map[string]string{
	"go.mod": "module example.com/webshop\n\ngo 1.24\n\nrequire github.com/acme/money v1.2.0\n",
	"cmd/shop/main.go": `package main

import (
	"example.com/webshop/internal/api"
	"example.com/webshop/internal/ui"
)

func main() { ui.Run(api.New()) }
`,
	"internal/ui/ui.go": `package ui

import "example.com/webshop/internal/api"

// Run shows the shop.
func Run(c *api.Client) { _ = c.Orders() }
`,
	"internal/api/api.go": `package api

import "example.com/webshop/internal/store"

// Client talks to the store.
type Client struct{ s *store.DB }

func New() *Client { return &Client{s: store.Open()} }

func (c *Client) Orders() []string { return c.s.All() }
`,
	"internal/store/store.go": `package store

type DB struct{}

func Open() *DB { return &DB{} }

func (d *DB) All() []string { return nil }
`,
	"internal/money/money.go": `package money

func Format(c int) string { return "" }
`,
}

var goLayers = `{
  "services": {},
  "x-herdr-deck": {
    "architecture": {
      "layers": [
        { "name": "entry", "paths": ["cmd/**"] },
        { "name": "ui", "paths": ["internal/ui/**"] },
        { "name": "api", "paths": ["internal/api/**"], "closed": true },
        { "name": "core", "paths": ["internal/store/**", "internal/money/**"] }
      ]
    }
  }
}
`

func goHead() map[string]string {
	return map[string]string{
		// store now imports ui: upward, and a component cycle.
		"internal/store/store.go": `package store

import (
	"database/sql"
	"os"

	"example.com/webshop/internal/ui"
)

const tokenEnv = "ABC_STORE_TOKEN"

type DB struct{ db *sql.DB }

func Open() *DB { _ = os.Getenv(tokenEnv); ui.Run(nil); return &DB{} }

func (d *DB) All() []string { _ = "SELECT id FROM orders WHERE open"; return nil }
`,
		// ui skips the closed api layer, and adds a third-party import.
		"internal/ui/ui.go": `package ui

import (
	"net/http"

	"example.com/webshop/internal/api"
	"example.com/webshop/internal/store"
	"github.com/acme/money"
)

// Run shows the shop.
func Run(c *api.Client) { _ = c.Orders(); _ = store.Open(); _ = money.Cents; _, _ = http.Get("https://pay.example.com/v1") }
`,
	}
}

func readAt(t *testing.T, rd *Reader, dir string) *Result {
	t.Helper()
	res := rd.Read(context.Background(), deck.Thread{Worktree: dir, Base: "base"})
	if res.Note != "" {
		t.Fatalf("note: %s", res.Note)
	}
	return res
}

func edge(r *Result, from, to string) (Edge, bool) {
	for _, e := range r.Edges {
		if e.From == from && e.To == to {
			return e, true
		}
	}
	return Edge{}, false
}

func TestGoChange(t *testing.T) {
	r := newRepo(t)
	files := maps.Clone(goBase)
	files[".config/dev.json"] = goLayers
	r.commit("base", files)
	r.git("branch", "base")
	r.commit("change", goHead())

	res := readAt(t, &Reader{}, r.dir)
	if res.Language != LangGo || res.Name != "webshop" || !res.Configured {
		t.Fatalf("language %q name %q configured %v", res.Language, res.Name, res.Configured)
	}
	up, ok := edge(res, "internal/store", "internal/ui")
	if !ok || up.Status != Added || up.Verdict != VerdictUp || up.Sites[0].File != "internal/store/store.go" || up.Sites[0].Line != 7 {
		t.Errorf("upward edge = %+v, %v", up, ok)
	}
	skip, ok := edge(res, "internal/ui", "internal/store")
	if !ok || skip.Verdict != VerdictSkip || skip.Why != "skips api" {
		t.Errorf("skip edge = %+v", skip)
	}
	if e, ok := edge(res, "internal/ui", "ext:github.com/acme/money"); !ok || e.Why != "third-party" {
		t.Errorf("third-party edge = %+v, %v", e, ok)
	}
	if e, ok := edge(res, "internal/ui", "internal/api"); !ok || e.Status != Unchanged {
		t.Errorf("ui → api = %+v, %v; want unchanged (same file, same uses)", e, ok)
	}
	if e, ok := edge(res, "internal/store", "std:database/sql"); !ok || e.Why != "database" {
		t.Errorf("database edge = %+v, %v", e, ok)
	}
	if len(res.Cycles) != 1 || cycleID(res.Cycles[0]) != "internal/store→internal/ui→internal/store" {
		t.Errorf("cycles = %v", res.Cycles)
	}
	var touches []string
	for _, tc := range res.Touches {
		touches = append(touches, tc.Kind+" "+tc.Target+" "+tc.Package)
	}
	slices.Sort(touches)
	want := []string{"env ABC_STORE_TOKEN internal/store", "http pay.example.com internal/ui", "sql orders internal/store"}
	if !slices.Equal(touches, want) {
		t.Errorf("touches = %q, want %q", touches, want)
	}
	// The riskiest first: the upward edge, then the cycle.
	if len(res.Findings) < 3 || res.Findings[0].ID != "edge:internal/store→internal/ui" || res.Findings[1].Kind != FindCycle {
		t.Errorf("findings = %v", res.Findings)
	}
	if res.Findings[1].Package != "internal/store" {
		t.Errorf("cycle selects %q", res.Findings[1].Package)
	}
	st, _ := res.Package("internal/store")
	if st.Status != Changed || !st.Impacted || st.Files != 1 || !slices.Equal(st.Touches, []string{"env", "sql"}) {
		t.Errorf("store = %+v", st)
	}
	if m, _ := res.Package("internal/money"); m.Impacted {
		t.Errorf("money is impacted: %+v", m)
	}
	if ui, _ := res.Package("internal/ui"); ui.NewDeps != 1 {
		t.Errorf("ui new deps = %d", ui.NewDeps)
	}
	if res.Lanes[0].Name != "entry" || res.Inferred {
		t.Errorf("lanes = %+v", res.Lanes)
	}
}

func TestInferredLanesNeverFlag(t *testing.T) {
	r := newRepo(t)
	r.commit("base", goBase)
	r.git("branch", "base")
	r.commit("change", goHead())
	res := readAt(t, &Reader{}, r.dir)
	if !res.Inferred || res.Configured {
		t.Errorf("inferred %v configured %v", res.Inferred, res.Configured)
	}
	for _, e := range res.Edges {
		if e.Verdict != VerdictNone {
			t.Errorf("%s → %s flagged %v without layers", e.From, e.To, e.Verdict)
		}
	}
}

func TestCaching(t *testing.T) {
	r := newRepo(t)
	r.commit("base", goBase)
	r.git("branch", "base")
	r.commit("change", goHead())
	rd := &Reader{}
	first := readAt(t, rd, r.dir)
	if first.Parsed == 0 {
		t.Fatal("nothing parsed")
	}
	if again := readAt(t, rd, r.dir); again != first {
		t.Error("the same commits read again")
	}
	// One more commit: only its new blob is parsed.
	r.commit("more", map[string]string{"internal/money/money.go": "package money\n\nfunc Format(c int) string { return \"€\" }\n"})
	next := readAt(t, rd, r.dir)
	if next == first || next.Parsed != 1 || next.Reused == 0 {
		t.Errorf("parsed %d reused %d", next.Parsed, next.Reused)
	}
	// Counting tests is another result.
	rd.Tests = true
	if tests := readAt(t, rd, r.dir); tests == next {
		t.Error("tests share the result without tests")
	}
}

func TestReadOnly(t *testing.T) {
	r := newRepo(t)
	r.commit("base", goBase)
	r.git("branch", "base")
	r.commit("change", goHead())
	before := r.git("status", "--porcelain", "--ignored")
	// git status itself refreshes the index; the read must not.
	index, _ := os.Stat(filepath.Join(r.dir, ".git", "index"))
	readAt(t, &Reader{}, r.dir)
	if now, _ := os.Stat(filepath.Join(r.dir, ".git", "index")); !now.ModTime().Equal(index.ModTime()) {
		t.Error("the index was written")
	}
	if after := r.git("status", "--porcelain", "--ignored"); after != before {
		t.Errorf("status changed: %q → %q", before, after)
	}
}

func TestMovesAndAPI(t *testing.T) {
	r := newRepo(t)
	r.commit("base", goBase)
	r.git("branch", "base")
	// Orders moves from api to a new orders package with its store
	// import; New's signature changes.
	r.commit("move", map[string]string{
		"internal/api/api.go": `package api

import "example.com/webshop/internal/orders"

// Client talks to the store.
type Client struct{ o *orders.Book }

func New(limit int) *Client { return &Client{o: orders.Open()} }

func (c *Client) Orders() []string { return c.o.All() }
`,
		"internal/orders/orders.go": `package orders

import "example.com/webshop/internal/store"

type Book struct{ s *store.DB }

func Open() *Book { return &Book{s: store.Open()} }

func (b *Book) All() []string { return b.s.All() }
`,
	})
	res := readAt(t, &Reader{}, r.dir)
	api, _ := res.Package("internal/api")
	if !slices.Equal(api.API.Changed, []string{"func New"}) {
		t.Errorf("api surface = %+v", api.API)
	}
	if o, _ := res.Package("internal/orders"); o.Status != Added || !o.Impacted {
		t.Errorf("orders = %+v", o)
	}
	if e, ok := edge(res, "internal/api", "internal/store"); !ok || e.Status != Removed {
		t.Errorf("api → store = %+v, %v", e, ok)
	}
}

func TestPureMove(t *testing.T) {
	r := newRepo(t)
	r.commit("base", goBase)
	r.git("branch", "base")
	r.git("mv", "internal/money/money.go", "internal/money/format.go")
	r.git("commit", "-q", "-m", "rename")
	res := readAt(t, &Reader{}, r.dir)
	if !res.PureMove() || res.Moves.Files != 1 || len(res.Findings) != 0 {
		t.Errorf("pure move %v moves %+v findings %v", res.PureMove(), res.Moves, res.Findings)
	}
}

func TestNotes(t *testing.T) {
	rd := &Reader{}
	if n := rd.Read(context.Background(), deck.Thread{}).Note; n != "no worktree" {
		t.Errorf("note = %q", n)
	}
	if n := rd.Read(context.Background(), deck.Thread{Worktree: filepath.Join(t.TempDir(), "gone")}).Note; n != "the worktree is gone" {
		t.Errorf("note = %q", n)
	}
	r := newRepo(t)
	r.commit("base", map[string]string{"README.md": "# hi\n"})
	r.git("branch", "base")
	if n := rd.Read(context.Background(), deck.Thread{Worktree: r.dir, Base: "base"}).Note; n != "no Go or TypeScript files to read" {
		t.Errorf("note = %q", n)
	}
	if n := rd.Read(context.Background(), deck.Thread{Worktree: r.dir, Base: "nope"}).Note; !strings.HasPrefix(n, "cannot compare with nope") {
		t.Errorf("note = %q", n)
	}
}

var tsBase = map[string]string{
	"package.json":                `{"name": "webshop", "private": true}`,
	"apps/admin/package.json":     `{"name": "@acme/admin", "dependencies": {"react": "19.0.0"}}`,
	"packages/db/package.json":    `{"name": "@acme/db"}`,
	"packages/db/src/index.ts":    "export function userCounts() { return 1 }\n",
	"apps/admin/tsconfig.json":    "{\n  // aliases\n  \"compilerOptions\": {\"baseUrl\": \".\", \"paths\": {\"@/*\": [\"src/*\"]},},\n}\n",
	"apps/admin/src/routes.tsx":   "import { UsersPage } from '@/pages/users'\nexport const routes = [UsersPage]\n",
	"apps/admin/src/api/users.ts": "export async function fetchUsers() { return [] }\n",
	"apps/admin/src/pages/users/index.tsx": `import React from 'react'
import { fetchUsers } from '../../api/users'
/* import { nope } from 'commented-out' */
export function UsersPage() { return fetchUsers() }
`,
	"apps/admin/src/pages/users/index.test.tsx": "import { UsersPage } from '.'\n",
}

func TestTypeScript(t *testing.T) {
	r := newRepo(t)
	files := maps.Clone(tsBase)
	files[".config/dev.json"] = `{"x-herdr-deck": {"architecture": {"roots": ["apps/admin/src", "packages"], "layers": [
		{"name": "entry", "paths": ["apps/admin/src"]},
		{"name": "pages", "paths": ["apps/admin/src/pages/**"]},
		{"name": "api", "paths": ["apps/admin/src/api/**"], "closed": true},
		{"name": "infra", "paths": ["packages/**"]}]}}}`
	r.commit("base", files)
	r.git("branch", "base")
	r.commit("change", map[string]string{
		"apps/admin/src/pages/users/index.tsx": `import React from 'react'
import { fetchUsers } from '../../api/users'
import type { Row } from '@tanstack/react-table'
import { userCounts } from '@acme/db'
export function UsersPage() { const n = process.env.VITE_PAGE_SIZE; return fetch("https://api.example.com/users") }
`,
	})
	res := readAt(t, &Reader{}, r.dir)
	if res.Language != LangTS {
		t.Fatalf("language %q", res.Language)
	}
	if e, ok := edge(res, "apps/admin/src/pages/users", "packages/db/src"); !ok || e.Verdict != VerdictSkip || e.Sites[0].Line != 4 || e.Sites[0].Uses != "userCounts" {
		t.Errorf("skip edge = %+v, %v", e, ok)
	}
	if _, ok := edge(res, "apps/admin/src/pages/users", "ext:@tanstack/react-table"); !ok {
		t.Error("no type-only import edge")
	}
	if _, ok := edge(res, "apps/admin/src", "apps/admin/src/pages/users"); !ok {
		t.Error("the tsconfig alias did not resolve")
	}
	for _, e := range res.Edges {
		if strings.Contains(e.To, "commented-out") {
			t.Error("an import in a comment counted")
		}
	}
	var kinds []string
	for _, tc := range res.Touches {
		kinds = append(kinds, tc.Kind+" "+tc.Target)
	}
	slices.Sort(kinds)
	if !slices.Equal(kinds, []string{"env VITE_PAGE_SIZE", "http api.example.com"}) {
		t.Errorf("touches = %q", kinds)
	}
}

func TestParseConfig(t *testing.T) {
	c, err := ParseConfig([]byte(`{"x-herdr-deck": {"architecture": {"language": "TypeScript", "roots": ["src/", "./lib"], "layers": [{"name": "a", "paths": ["src/a/**"]}]}}}`))
	if err != nil || c.Language != LangTS || !slices.Equal(c.Roots, []string{"src", "lib"}) || len(c.Layers) != 1 {
		t.Errorf("config = %+v, %v", c, err)
	}
	if c, err := ParseConfig([]byte(`{"services": {}}`)); err != nil || len(c.Layers) != 0 {
		t.Errorf("no block = %+v, %v", c, err)
	}
	if _, err := ParseConfig([]byte(`{"x-herdr-deck": {"architecture": {"language": "cobol"}}}`)); err == nil {
		t.Error("cobol read")
	}
	if _, err := ParseConfig([]byte(`{"x-herdr-deck": {"architecture": {"layers": [{"paths": []}]}}}`)); err == nil {
		t.Error("a nameless layer read")
	}
}

func TestLanesCheck(t *testing.T) {
	l := configLanes([]Layer{
		{Name: "api", Paths: []string{"api/**"}},
		{Name: "domain", Paths: []string{"domain/**"}, Closed: true},
		{Name: "infra", Paths: []string{"infra/**"}},
	}, []string{"api/routes", "domain/export", "infra/s3", "tools"})
	for _, c := range []struct {
		from, to string
		want     Verdict
	}{
		{"api/routes", "domain/export", VerdictNone},
		{"domain/export", "infra/s3", VerdictNone},
		{"api/routes", "infra/s3", VerdictSkip},
		{"infra/s3", "domain/export", VerdictUp},
		{"tools", "api/routes", VerdictNone},
	} {
		if got, _ := l.check(edgeKey{c.from, c.to}); got != c.want {
			t.Errorf("%s → %s: %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestStripComments(t *testing.T) {
	src := "a // x 'y'\nb = '/* not */' /* gone\nstill */ c `//keep`\n"
	got := string(stripComments([]byte(src)))
	if strings.Contains(got, "gone") || strings.Contains(got, "still") || !strings.Contains(got, "'/* not */'") || !strings.Contains(got, "`//keep`") {
		t.Errorf("got %q", got)
	}
	if strings.Count(got, "\n") != strings.Count(src, "\n") || len(got) != len(src) {
		t.Error("lines or offsets moved")
	}
}

// A removed upward import is not a violation: the finding says it went.
func TestRemovedViolation(t *testing.T) {
	cfg := `{"x-herdr-deck": {"architecture": {"layers": [
		{"name": "ui", "paths": ["ui/**"]}, {"name": "core", "paths": ["core/**"]}]}}}`
	base := map[string]string{"go.mod": "module example.com/m\n", ".config/dev.json": cfg,
		"ui/ui.go":     "package ui\n\nfunc Run() {}\n",
		"core/core.go": "package core\n\nimport \"example.com/m/ui\"\n\nfunc Run() { ui.Run() }\n"}
	head := maps.Clone(base)
	head["core/core.go"] = "package core\n\nfunc Run() {}\n"
	res := FromFiles("m", "main", base, head, Config{})
	e, ok := edge(res, "core", "ui")
	if !ok || e.Status != Removed || e.Verdict != VerdictUp || e.Risky() {
		t.Fatalf("edge = %+v", e)
	}
	if len(res.Findings) == 0 || res.Findings[0].Verdict != VerdictNone || res.Findings[0].Why != "an upward import gone" {
		t.Errorf("findings = %+v", res.Findings)
	}
}

func TestTSResolution(t *testing.T) {
	res := &tsResolver{files: map[string]bool{
		"src/db/client.ts": true, "src/ui/components/Button.tsx": true, "src/components/Card.tsx": true,
	}, configs: []*tsconfig{{dir: ".", baseURL: ".", paths: map[string][]string{
		"@/*": {"src/*"}, "@/components/*": {"src/ui/components/*"},
	}}}}
	for _, c := range []struct{ spec, want string }{
		{"./db/client.js", "src/db"},                 // ESM: .js for a .ts source
		{"@/components/Button", "src/ui/components"}, // the longest prefix wins
		{"@/components/Card", ""},                    // not where the longer pattern points
		{"@/db/client", "src/db"},
	} {
		if got := res.resolve("src/main.ts", c.spec); got != c.want {
			t.Errorf("%s: %q, want %q", c.spec, got, c.want)
		}
	}
}

func TestParseModBlocks(t *testing.T) {
	m := parseMod(".", []byte(`module example.com/m

go 1.24

require github.com/acme/one v1.0.0

require (
	github.com/acme/two v2.0.0 // indirect
)

replace (
	github.com/acme/one v1.0.0 => ./one
	github.com/acme/three v0.1.0 => ./three
)

exclude github.com/acme/four v0.9.0
`))
	want := map[string]string{"github.com/acme/one": "v1.0.0", "github.com/acme/two": "v2.0.0"}
	if !maps.Equal(m.requires, want) {
		t.Errorf("requires = %v, want %v", m.requires, want)
	}
}

func TestShapesAndSQL(t *testing.T) {
	a := parseTS([]byte("export const port = 1\n"))
	b := parseTS([]byte("export const host = 1\n"))
	if a.decls[0].shape != b.decls[0].shape || a.decls[0].hash == b.decls[0].hash {
		t.Errorf("a rename is not one shape: %+v %+v", a.decls[0], b.decls[0])
	}
	g := parseGo([]byte("package q\n\nconst all = `SELECT id,\n  name\nFROM users WHERE open`\n"))
	if len(g.touches) != 1 || g.touches[0].target != "users" {
		t.Errorf("touches = %+v", g.touches)
	}
}

// A move that also changes a dependency is no pure move.
func TestPureMoveWithADependency(t *testing.T) {
	base := map[string]string{"go.mod": "module example.com/m\n", "a/a.go": "package a\n\nfunc A() {}\n"}
	head := map[string]string{"go.mod": "module example.com/m\n\nrequire github.com/acme/x v1.0.0\n", "b/a.go": "package a\n\nfunc A() {}\n"}
	res := FromFiles("m", "main", base, head, Config{})
	if res.PureMove() || len(res.Deps) != 1 {
		t.Errorf("pure move %v, deps %v", res.PureMove(), res.Deps)
	}
}

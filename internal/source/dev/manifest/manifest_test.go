package manifest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// shop is a made-up monorepo's manifest, with its keys in an order that
// sorting would change.
const shop = `{
  "version": 1,
  "env": { "files": [".env"], "vars": { "APP_ENV": "development" } },
  "path": ["node_modules/.bin"],
  "state": { "file": "$REPO/.dev/$DIRNAME/state.json" },
  "ports": {
    "web": { "state": "web_port" },
    "api": { "state": "api_port" },
    "db": 54321,
    "docs": { "base": 6100, "range": 10 },
    "mail": { "fixed": 8025 }
  },
  "services": {
    "web": { "dir": "apps/web", "run": "pnpm dev", "needs": "api" },
    "api": { "title": "API", "run": ["go", "run", "./cmd/api"], "needs": ["db"], "ready": { "http": "/healthz", "timeout": "90s" }, "log": ".dev/api.log" },
    "worker": { "run": { "run": "make worker", "env": { "QUEUE": "dev" } }, "needs": "db" },
    "docs": {}
  },
  "groups": { "default": ["web"], "all": ["web", "worker"] },
  "commands": {
    "test": { "*": "make test", "web": "pnpm test" },
    "dev": null,
    "lint": { "run": "make lint", "loadEnv": true },
    "x-tool": "ignored"
  },
  "links": [{ "title": "Shop", "url": "http://localhost:$PORT_web", "needs": "web" }],
  "x-mytool": { "anything": true }
}`

func TestParseKeepsOrder(t *testing.T) {
	m, err := Parse([]byte(shop))
	if err != nil {
		t.Fatal(err)
	}
	var ports, services, groups, commands []string
	for _, p := range m.Ports {
		ports = append(ports, p.Name)
	}
	for _, s := range m.Services {
		services = append(services, s.Name)
	}
	for _, g := range m.Groups {
		groups = append(groups, g.Name)
	}
	for _, c := range m.Commands {
		commands = append(commands, c.Name)
	}
	for _, c := range []struct {
		what      string
		got, want []string
	}{
		{"ports", ports, []string{"web", "api", "db", "docs", "mail"}},
		// db and mail have no service: they come last, in port order.
		{"services", services, []string{"web", "api", "worker", "docs", "db", "mail"}},
		{"groups", groups, []string{"default", "all"}},
		{"commands", commands, []string{"test", "dev", "lint"}},
	} {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s %q, want %q", c.what, c.got, c.want)
		}
	}

	if p := m.Port("docs"); p.Base != 6100 || p.Range != 10 {
		t.Errorf("docs port %+v", p)
	}
	if p := m.Port("mail"); p.Fixed != 8025 {
		t.Errorf("mail port %+v", p)
	}
	web := m.Service("web")
	if web.Title != "web" || web.Ports[0] != "web" || web.Run.Shell != "pnpm dev" || web.Dir != "apps/web" || web.Ready.Timeout != DefaultTimeout {
		t.Errorf("web %+v", web)
	}
	api := m.Service("api")
	if api.Title != "API" || api.Ready.HTTP != "/healthz" || api.Ready.Timeout != 90*time.Second || !reflect.DeepEqual(api.Run.Argv, []string{"go", "run", "./cmd/api"}) {
		t.Errorf("api %+v", api)
	}
	if w := m.Service("worker"); len(w.Ports) != 0 || w.Run.Shell != "make worker" || w.Run.Env["QUEUE"] != "dev" {
		t.Errorf("worker %+v", w)
	}
	if db := m.Service("db"); !db.Implicit || db.Run != nil || db.Ports[0] != "db" {
		t.Errorf("db %+v", db)
	}
	if c, ok := m.Command("dev"); !ok || !c.Null {
		t.Errorf("dev %+v, %v: want null", c, ok)
	}
	if c, _ := m.Command("test"); len(c.Entries) != 2 || c.Entries[0].Service != "*" || c.Entries[1].Run.Shell != "pnpm test" {
		t.Errorf("test %+v", c)
	}
	if c, _ := m.Command("lint"); c.Run == nil || !c.Run.LoadEnv {
		t.Errorf("lint %+v", c)
	}
	if _, ok := m.Command("stop"); ok {
		t.Error("stop is not in the file")
	}
}

func TestStartOrder(t *testing.T) {
	m, err := Parse([]byte(shop))
	if err != nil {
		t.Fatal(err)
	}
	// web needs api, api needs db: what is needed comes first.
	if got, want := m.StartOrder(m.DefaultGroup()), []string{"db", "api", "web"}; !reflect.DeepEqual(got, want) {
		t.Errorf("default %q, want %q", got, want)
	}
	if got, want := m.StartOrder([]string{"worker", "web"}), []string{"db", "api", "web", "worker"}; !reflect.DeepEqual(got, want) {
		t.Errorf("worker, web %q, want %q", got, want)
	}
	// Without a default group, every service in manifest order.
	m.Groups = nil
	if got := m.DefaultGroup(); len(got) != 6 || got[0] != "web" {
		t.Errorf("default group %q", got)
	}
}

func TestParseRefuses(t *testing.T) {
	for _, c := range []struct{ name, file, want string }{
		{"version", `{"version": 2}`, "version 2 is not supported"},
		{"no version", `{}`, "schema"},
		{"cycle", `{"version": 1, "services": {"a": {"needs": "b"}, "b": {"needs": "a"}}}`, "cycle"},
		{"variable", `{"version": 1, "commands": {"dev": "PORT=$HOME make"}}`, "unknown variable $HOME"},
		{"several", `{"version": 1, "groups": {"default": ["x"]}, "commands": {"dev": "echo $HOME"}}`, "(and 1 more)"},
		{"syntax", `{"version": 1,`, "not JSON"},
	} {
		if _, err := Parse([]byte(c.file)); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
	}
}

// A key the schema does not know is ignored (spec section 2.3), so a
// newer version-1 file still works, and it is reported.
func TestParseIgnoresUnknownKeys(t *testing.T) {
	m, err := Parse([]byte(`{"version": 1, "future": true, "services": {"web": {"run": "x", "restart": "always"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"future", "services.web.restart"}; !reflect.DeepEqual(m.Unknown, want) {
		t.Errorf("unknown %q, want %q", m.Unknown, want)
	}
	if m.Service("web").Run.Shell != "x" {
		t.Errorf("web %+v", m.Service("web"))
	}
}

func TestExpand(t *testing.T) {
	v := Vars{
		Worktree: "/src/wt/abc-12-cart", Repo: "/src/shop", Branch: "abc-12-cart",
		Ports: map[string]int{"web": 3101},
		Env:   map[string]string{"DOMAIN": "shop.test", "EMPTY": ""},
	}
	for in, want := range map[string]string{
		"$WORKTREE $REPO $DIRNAME $BRANCH":       "/src/wt/abc-12-cart /src/shop abc-12-cart abc-12-cart",
		"http://localhost:${PORT_web}/x":         "http://localhost:3101/x",
		"$$HOME costs $$5":                       "$HOME costs $5",
		"https://$env(DOMAIN)/$env(NOPE|x.test)": "https://shop.test/x.test",
		"[$env(EMPTY|fallback)] [$env(NOPE)]":    "[] []",
		"${DIRNAME}.log":                         "abc-12-cart.log",
	} {
		got, err := v.Expand(in)
		if err != nil || got != want {
			t.Errorf("Expand(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	var mp *MissingPortError
	if _, err := v.Expand("http://localhost:$PORT_api"); !errors.As(err, &mp) || mp.Port != "api" {
		t.Errorf("unknown port: %v", err)
	}
	if _, err := v.Expand("$HOME"); err == nil {
		t.Error("$HOME expanded")
	}
	if p, _ := v.Path(".dev/$DIRNAME/state.json"); p != "/src/wt/abc-12-cart/.dev/abc-12-cart/state.json" {
		t.Errorf("relative path %q", p)
	}
	if got := PortRefs("http://localhost:$PORT_web/?api=${PORT_api}&x=$$PORT_no"); !reflect.DeepEqual(got, []string{"web", "api"}) {
		t.Errorf("PortRefs %q", got)
	}
}

func TestDotenv(t *testing.T) {
	got := ParseDotenv([]byte(`# comment
A=1
export B = two words
C="quoted # not a comment\n" # comment
D='single $x'
E=plain # comment
bad-key=x
F
`))
	want := map[string]string{"A": "1", "B": "two words", "C": "quoted # not a comment\n", "D": "single $x", "E": "plain"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dotenv %q, want %q", got, want)
	}
}

func TestLoadEnvLastFileWins(t *testing.T) {
	dir := t.TempDir()
	repo, wt := filepath.Join(dir, "shop"), filepath.Join(dir, "wt")
	writeFile(t, filepath.Join(repo, ".env"), "A=repo\nB=repo\n")
	writeFile(t, filepath.Join(wt, ".env.local"), "B=worktree\n")
	m := &Manifest{EnvFiles: []string{"$REPO/.env", ".env.local", ".env.missing"}}
	got := m.LoadEnv(Vars{Worktree: wt, Repo: repo})
	if want := map[string]string{"A": "repo", "B": "worktree"}; !reflect.DeepEqual(got, want) {
		t.Errorf("env %q, want %q", got, want)
	}
}

func TestLegacy(t *testing.T) {
	m, err := ParseLegacy([]byte(`{"state": {"file": ".dev/$DIRNAME/state.json", "ports": {"frontend": "frontend_port", "docs": 6060}},
		"links": [{"title": "Docs", "url": "http://localhost:$PORT_docs", "needs": "docs"}],
		"up": "scripts/dev-up --name $DIRNAME"}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.StateFile != "$REPO/.dev/$DIRNAME/state.json" {
		t.Errorf("state file %q: relative to the main checkout, so $REPO/ in front", m.StateFile)
	}
	if len(m.Ports) != 2 || m.Ports[0].State != "frontend_port" || m.Ports[1].Fixed != 6060 {
		t.Errorf("ports %+v", m.Ports)
	}
	if len(m.Services) != 2 || !m.Services[0].Implicit || m.Services[1].Name != "docs" {
		t.Errorf("services %+v", m.Services)
	}
	if c, ok := m.Command("dev"); !ok || c.Run.Shell != "scripts/dev-up --name $DIRNAME" {
		t.Errorf("dev %+v", c)
	}
	for _, c := range []struct{ file, want string }{
		{`{"state": {"file": "s.json"}}`, "names no servers"},
		{`{"state": {"ports": {"api": "api_port"}}}`, "state.file is empty"},
		{`{"state": {"ports": {"api": 70000}}}`, "not a port number"},
		{`{"state": {"ports": {"api": 8000}}, "up": "PORT=$HOME make"}`, "up: unknown variable $HOME"},
		{`{"state": {"ports": {"api": 8000}}, "links": [{"url": "http://localhost:$PORT_web"}]}`, `no port "web"`},
		{`{"state": {"ports": {"api": 8000}}, "links": [{"url": "x", "needs": "web"}]}`, `needs "web"`},
	} {
		if _, err := ParseLegacy([]byte(c.file)); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.file, err, c.want)
		}
	}
}

func TestFind(t *testing.T) {
	dir := t.TempDir()
	repo, wt := filepath.Join(dir, "shop"), filepath.Join(dir, "wt")
	ok := `{"version": 1, "ports": {"web": 3000}}`
	legacy := `{"state": {"ports": {"web": 3000}}}`
	must := func(rel string) {
		t.Helper()
		f, err := Find(wt, repo)
		if err != nil {
			t.Fatalf("want %s: %v", rel, err)
		}
		if got := f.File(); got != rel {
			t.Errorf("found %s, want %s", got, rel)
		}
	}
	if _, err := Find(wt, repo); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("no files: %v", err)
	}
	writeFile(t, filepath.Join(repo, LegacyPath), legacy)
	must(filepath.Join(repo, LegacyPath))
	writeFile(t, filepath.Join(wt, LegacyPath), legacy)
	must(filepath.Join(wt, LegacyPath))
	// Any .config/dev.json comes before every legacy file.
	writeFile(t, filepath.Join(repo, Path), ok)
	must(filepath.Join(repo, Path))
	writeFile(t, filepath.Join(wt, Path), ok)
	must(filepath.Join(wt, Path))
	if f, _ := Find(wt, repo); f.Legacy || f.Short() != filepath.Join("wt", Path) {
		t.Errorf("found %+v", f)
	}

	// A broken file is the manifest: no falling back to the next one.
	writeFile(t, filepath.Join(wt, Path), `{"version": 1, "services": {"web": {"run": "echo $HOME"}}}`)
	f, err := Find(wt, repo)
	if err == nil || errors.Is(err, fs.ErrNotExist) || f.File() != filepath.Join(wt, Path) {
		t.Errorf("broken worktree file: %+v, %v", f, err)
	}
	if err := os.Remove(filepath.Join(wt, Path)); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, Path), `{"version": 1,`)
	if f, err := Find(wt, repo); err == nil || f.File() != filepath.Join(repo, Path) {
		t.Errorf("broken main file: %+v, %v", f, err)
	}

	// The main checkout itself: its own file, once.
	if f, err := Find(repo, repo); err == nil || f.Dir != repo {
		t.Errorf("main checkout: %+v, %v", f, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

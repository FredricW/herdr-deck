package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFixtures checks the manifests the skill wrote for its fixture repos,
// and herdr-deck's own: valid, and nothing they point at is missing.
func TestFixtures(t *testing.T) {
	files, err := filepath.Glob("../testdata/*/.config/dev.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 3 {
		t.Fatalf("found %d fixture manifests, want at least 3", len(files))
	}
	files = append(files, "../../../.config/dev.json")
	for _, f := range files {
		res := Validate(f, repoOf(f))
		for _, e := range res.Errors {
			t.Errorf("%s: %s", f, e)
		}
		for _, w := range res.Warnings {
			t.Errorf("%s: warning: %s", f, w)
		}
	}
}

func TestSpecExamples(t *testing.T) {
	b, err := os.ReadFile("../../../docs/dev-manifest.md")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(b), "<!-- example: dev.json -->\n```json\n")
	if len(parts) < 5 {
		t.Fatalf("found %d examples, want at least 4", len(parts)-1)
	}
	for i, p := range parts[1:] {
		body, _, _ := strings.Cut(p, "\n```")
		if res := ValidateBytes([]byte(body), ""); len(res.Errors) > 0 {
			t.Errorf("example %d: %v", i+1, res.Errors)
		}
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name, manifest, want string
	}{
		{"schema", `{"version": 2}`, "schema:"},
		{"unknown variable", `{"version": 1, "commands": {"x": "echo $HOME"}}`, "unknown variable $HOME"},
		{"lone dollar", `{"version": 1, "commands": {"x": "echo $"}}`, "a lone $"},
		{"undefined port", `{"version": 1, "links": [{"url": "http://localhost:$PORT_web"}]}`, `no port "web"`},
		{"braces", `{"version": 1, "ports": {"web": 3000}, "path": ["${NOPE}/bin"]}`, "unknown variable $NOPE"},
		{"bad env name", `{"version": 1, "links": [{"url": "$env(A-B)"}]}`, "not a variable name"},
		{"needs", `{"version": 1, "services": {"web": {"needs": "db"}}}`, `"db" is not a service`},
		{"cycle", `{"version": 1, "services": {"a": {"needs": "b"}, "b": {"needs": "a"}}}`, "cycle"},
		{"group", `{"version": 1, "groups": {"default": ["web"]}}`, `"web" is not a service`},
		{"per-service key", `{"version": 1, "commands": {"test": {"web": "npm test"}}}`, `"web" is not a service`},
		{"port owner", `{"version": 1, "ports": {"web": 1}, "services": {"a": {"ports": ["web"]}, "b": {"ports": ["web"]}}}`, "already belongs"},
		{"port with a service's name", `{"version": 1, "ports": {"web": 1, "x": 2}, "services": {"web": {"ports": ["x"]}}}`, "service \"web\"'s name"},
		{"missing port", `{"version": 1, "services": {"web": {"ports": ["web"]}}}`, `no port "web"`},
		{"ready port", `{"version": 1, "ports": {"a": 1, "b": 2}, "services": {"web": {"ports": ["a"], "ready": {"port": "b"}}}}`, "not one of the service's ports"},
		{"range", `{"version": 1, "ports": {"web": {"base": 65500}}}`, "above 65535"},
		{"state without file", `{"version": 1, "ports": {"web": {"state": "web_port"}}}`, "no state.file"},
	}
	for _, tt := range tests {
		res := ValidateBytes([]byte(tt.manifest), "")
		if !strings.Contains(strings.Join(res.Errors, "\n"), tt.want) {
			t.Errorf("%s: errors %q, want one containing %q", tt.name, res.Errors, tt.want)
		}
	}
}

func TestValid(t *testing.T) {
	for _, m := range []string{
		`{"version": 1}`,
		`{"version": 1, "commands": {"x": "echo $$HOME $${HOME} $DIRNAME ${BRANCH} $env(A|b c)"}}`,
		`{"version": 1, "ports": {"db": 5432}, "services": {"api": {"needs": "db"}}, "groups": {"default": ["db"]}}`,
	} {
		if res := ValidateBytes([]byte(m), ""); len(res.Errors) > 0 {
			t.Errorf("%s: %v", m, res.Errors)
		}
	}
}

func TestDangling(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", `{"scripts": {"dev": "vite"}}`)
	write("Makefile", "VAR := 1\ntest:\n\tgo test\n")
	write("justfile", "setup arg='x':\n    true\n")
	write("scripts/ok", "")
	m := `{"version": 1,
	  "services": {
	    "web": {"run": "npm run dev"},
	    "gone": {"dir": "nope", "run": "npm run dev"}
	  },
	  "commands": {
	    "setup": ["just", "setup"],
	    "test": "make test && echo done",
	    "lint": "make lint",
	    "format": "npm run format",
	    "seed": ["scripts/seed"],
	    "migrate": "scripts/ok migrate",
	    "teardown": "just teardown",
	    "open": "VAR",
	    "x": "make VAR=1",
	    "y": "\"$$HOME/bin/x\""
	  }}`
	res := ValidateBytes([]byte(m), root)
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	got := strings.Join(res.Warnings, "\n")
	for _, want := range []string{
		"dangling:services.gone.dir",
		"dangling:commands.lint:",
		"dangling:commands.format:",
		"dangling:commands.seed:",
		"dangling:commands.teardown:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("warnings %q, want %s", got, want)
		}
	}
	if len(res.Warnings) != 5 {
		t.Errorf("got %d warnings, want 5:\n%s", len(res.Warnings), got)
	}
}

func TestNotDangling(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Makefile", "dev up: deps\n\ttrue\n")
	write("justfile", "test:\n    true\nalias t := test\n")
	write("api/main.go", "")
	write("package.json", `{"scripts": {}}`)
	m := `{"version": 1,
	  "services": {"api": {"dir": "api", "run": "just test"}},
	  "commands": {
	    "dev": "make up",
	    "test": {"api": "just t"},
	    "lint": "bun run lint.ts",
	    "format": "npm run format -w apps/web",
	    "seed": {"run": "make seed"},
	    "migrate": "scripts/migrate"
	  },
	  "detect": {"ignore": ["dangling:commands.seed", "dangling:commands.mig*"]}}`
	res := ValidateBytes([]byte(m), root)
	if len(res.Errors) > 0 || len(res.Warnings) > 0 {
		t.Errorf("errors %q, warnings %q, want none", res.Errors, res.Warnings)
	}
	off := `{"version": 1, "commands": {"x": "scripts/nope"}, "detect": {"drift": false}}`
	if res := ValidateBytes([]byte(off), root); len(res.Warnings) > 0 {
		t.Errorf("drift off: warnings %q", res.Warnings)
	}
}

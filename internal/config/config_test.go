package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// noHunk and withHunk are fake PATH lookups, so tests do not depend on
// what is installed.
func noHunk(string) (string, error) { return "", errors.New("not found") }

func withHunk(name string) (string, error) {
	if name == "hunk" {
		return "/fake/bin/hunk", nil
	}
	return "", errors.New("not found")
}

// env is a fake environment rooted in a temp folder, so tests never read
// the real ~/.config.
func env(t *testing.T, vars map[string]string) (func(string) string, string) {
	t.Helper()
	home := t.TempDir()
	m := map[string]string{"HOME": home}
	for k, v := range vars {
		m[k] = v
	}
	return func(k string) string { return m[k] }, home
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPath(t *testing.T) {
	xdg := t.TempDir()
	tests := []struct {
		name  string
		flag  string
		vars  map[string]string
		want  func(home string) string
		named bool
	}{
		{"default", "", nil, func(h string) string { return filepath.Join(h, ".config", "herdr-deck", "config.toml") }, false},
		{"xdg", "", map[string]string{"XDG_CONFIG_HOME": xdg}, func(string) string { return filepath.Join(xdg, "herdr-deck", "config.toml") }, false},
		{"relative xdg is ignored", "", map[string]string{"XDG_CONFIG_HOME": "rel/cfg"}, func(h string) string { return filepath.Join(h, ".config", "herdr-deck", "config.toml") }, false},
		{"env", "", map[string]string{"XDG_CONFIG_HOME": xdg, EnvPath: "/etc/deck.toml"}, func(string) string { return "/etc/deck.toml" }, true},
		{"flag beats env", "~/deck.toml", map[string]string{EnvPath: "/etc/deck.toml"}, func(h string) string { return filepath.Join(h, "deck.toml") }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv, home := env(t, tt.vars)
			got, named := Path(tt.flag, getenv)
			if want := tt.want(home); got != want || named != tt.named {
				t.Errorf("Path = %q, %v; want %q, %v", got, named, want, tt.named)
			}
		})
	}
}

func TestResolveMissingFileIsSilent(t *testing.T) {
	getenv, home := env(t, nil)
	s, err := Resolve(Flags{}, getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Problems) != 0 {
		t.Errorf("problems = %q, want none", s.Problems)
	}
	if s.LinearWorkspace != "" || s.RefreshInterval != DefaultRefreshInterval || s.ProjectsRoot != filepath.Join(home, ".herdr-projects") {
		t.Errorf("settings = %+v, want defaults", s)
	}
}

func TestResolveNamedMissingFileIsReported(t *testing.T) {
	getenv, home := env(t, nil)
	s, err := Resolve(Flags{Config: filepath.Join(home, "nope.toml")}, getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Problems) != 1 || !strings.Contains(s.Problems[0], "nope.toml") {
		t.Errorf("problems = %q, want one about nope.toml", s.Problems)
	}
}

func TestResolveReadsXDGFile(t *testing.T) {
	xdg := t.TempDir()
	getenv, _ := env(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	write(t, filepath.Join(xdg, "herdr-deck", "config.toml"), `
linear_workspace = "acme"
refresh_interval = "30s"
projects_root = "~/work/projects"
`)
	s, err := Resolve(Flags{}, getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Problems) != 0 {
		t.Errorf("problems = %q", s.Problems)
	}
	if s.LinearWorkspace != "acme" || s.RefreshInterval != 30*time.Second {
		t.Errorf("settings = %+v", s)
	}
	if want := filepath.Join(getenv("HOME"), "work", "projects"); s.ProjectsRoot != want {
		t.Errorf("ProjectsRoot = %q, want %q", s.ProjectsRoot, want)
	}
}

func TestResolvePrecedence(t *testing.T) {
	const file = `
linear_workspace = "from-file"
refresh_interval = "20s"
projects_root = "/file/root"
`
	tests := []struct {
		name  string
		flags Flags
		vars  map[string]string
		want  Settings
	}{
		{"file beats default", Flags{}, nil,
			Settings{LinearWorkspace: "from-file", RefreshInterval: 20 * time.Second, ProjectsRoot: "/file/root"}},
		{"env beats file", Flags{}, map[string]string{EnvLinearWorkspace: "from-env", EnvRefreshInterval: "15s", EnvProjectsRoot: "/env/root"},
			Settings{LinearWorkspace: "from-env", RefreshInterval: 15 * time.Second, ProjectsRoot: "/env/root"}},
		{"flag beats env", Flags{LinearWorkspace: "from-flag", RefreshInterval: "10s", ProjectsRoot: "/flag/root"},
			map[string]string{EnvLinearWorkspace: "from-env", EnvRefreshInterval: "15s", EnvProjectsRoot: "/env/root"},
			Settings{LinearWorkspace: "from-flag", RefreshInterval: 10 * time.Second, ProjectsRoot: "/flag/root"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv, home := env(t, tt.vars)
			path := filepath.Join(home, ".config", "herdr-deck", "config.toml")
			write(t, path, file)
			s, err := Resolve(tt.flags, getenv, noHunk)
			if err != nil {
				t.Fatal(err)
			}
			if len(s.Problems) != 0 {
				t.Errorf("problems = %q", s.Problems)
			}
			tt.want.Path = path
			if s.Path != tt.want.Path || s.LinearWorkspace != tt.want.LinearWorkspace ||
				s.RefreshInterval != tt.want.RefreshInterval || s.ProjectsRoot != tt.want.ProjectsRoot {
				t.Errorf("settings = %+v, want %+v", s, tt.want)
			}
		})
	}
}

func TestResolveMalformedFile(t *testing.T) {
	getenv, home := env(t, map[string]string{EnvLinearWorkspace: "from-env"})
	write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), "linear_workspace = \"acme\"\nrefresh_interval = \n")
	s, err := Resolve(Flags{}, getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Problems) != 1 || !strings.Contains(s.Problems[0], "config: ~/.config/herdr-deck/config.toml: line 2") || !strings.Contains(s.Problems[0], "ignored") {
		t.Errorf("problems = %q, want one naming line 2", s.Problems)
	}
	if s.LinearWorkspace != "from-env" || s.RefreshInterval != DefaultRefreshInterval {
		t.Errorf("settings = %+v, want env and defaults", s)
	}
}

func TestResolveUnknownKeysAndBadValues(t *testing.T) {
	getenv, home := env(t, nil)
	write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), `
linear_workspace = "acme"
linear_workpsace = "typo"
refresh_interval = "soon"
projects_root = "relative/root"
[update]
hint = true
`)
	s, err := Resolve(Flags{}, getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	if s.LinearWorkspace != "acme" {
		t.Errorf("LinearWorkspace = %q: a bad key must not lose the good ones", s.LinearWorkspace)
	}
	if s.RefreshInterval != DefaultRefreshInterval || s.ProjectsRoot != filepath.Join(home, ".herdr-projects") {
		t.Errorf("settings = %+v, want defaults for the bad values", s)
	}
	all := strings.Join(s.Problems, "\n")
	for _, want := range []string{`unknown key "linear_workpsace"`, `unknown key "update"`, `refresh_interval: "soon" is not a duration`, `projects_root "relative/root" is not an absolute path`} {
		if !strings.Contains(all, want) {
			t.Errorf("problems lack %q:\n%s", want, all)
		}
	}
	if len(s.Problems) != 4 {
		t.Errorf("got %d problems, want 4:\n%s", len(s.Problems), all)
	}
}

func TestResolveRefreshIntervalRange(t *testing.T) {
	getenv, home := env(t, map[string]string{EnvRefreshInterval: "100ms"})
	write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), `refresh_interval = "2h"`)
	s, err := Resolve(Flags{}, getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	if s.RefreshInterval != DefaultRefreshInterval {
		t.Errorf("RefreshInterval = %v, want the default", s.RefreshInterval)
	}
	if len(s.Problems) != 2 || !strings.Contains(s.Problems[0], EnvRefreshInterval) || !strings.Contains(s.Problems[1], "outside") {
		t.Errorf("problems = %q, want the env value then the file value", s.Problems)
	}

	if _, err := Resolve(Flags{RefreshInterval: "0s"}, getenv, noHunk); err == nil {
		t.Error("a bad --refresh-interval must be an error")
	}
	if s, err := Resolve(Flags{RefreshInterval: "1m"}, getenv, noHunk); err != nil || s.RefreshInterval != time.Minute {
		t.Errorf("--refresh-interval 1m = %v, %v", s.RefreshInterval, err)
	}
}

func TestResolveProgramDefaults(t *testing.T) {
	getenv, _ := env(t, nil)
	s, err := Resolve(Flags{}, getenv, withHunk)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(s.Editor.Argv, " "); got != DefaultEditor || s.Editor.Terminal {
		t.Errorf("editor = %q terminal %v, want %q in no terminal", got, s.Editor.Terminal, DefaultEditor)
	}
	if got := strings.Join(s.Diff.Argv, " "); got != DefaultDiffTool || !s.Diff.Terminal {
		t.Errorf("diff = %q terminal %v, want hunk in a terminal", got, s.Diff.Terminal)
	}

	s, err = Resolve(Flags{}, getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(s.Diff.Argv, " "); got != FallbackDiffTool || !s.Diff.Terminal {
		t.Errorf("diff without hunk = %q terminal %v, want %q", got, s.Diff.Terminal, FallbackDiffTool)
	}
}

func TestResolveProgramPrecedence(t *testing.T) {
	const file = `
[editor]
command = "nvim {path}"
terminal = true

[diff]
command = "difft-wrapper --base {base} {path}"
terminal = false
`
	yes := true
	tests := []struct {
		name       string
		flags      Flags
		vars       map[string]string
		editor     string
		editorTerm bool
		diff       string
		diffTerm   bool
	}{
		{"file", Flags{}, nil, "nvim {path}", true, "difft-wrapper --base {base} {path}", false},
		{"env command drops the file's terminal", Flags{},
			map[string]string{EnvEditor: "zed {path}", EnvDiffTool: "lazygit"},
			"zed {path}", false, "lazygit", true},
		{"env terminal with the file command", Flags{},
			map[string]string{EnvEditorTerminal: "false"},
			"nvim {path}", false, "difft-wrapper --base {base} {path}", false},
		{"flag beats env", Flags{Editor: "cursor '{path}'", DiffTool: "tig", DiffTerminal: &yes},
			map[string]string{EnvEditor: "zed {path}", EnvEditorTerminal: "true", EnvDiffTool: "lazygit"},
			"cursor {path}", false, "tig", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv, home := env(t, tt.vars)
			write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), file)
			s, err := Resolve(tt.flags, getenv, withHunk)
			if err != nil {
				t.Fatal(err)
			}
			if len(s.Problems) != 0 {
				t.Errorf("problems = %q", s.Problems)
			}
			if got := strings.Join(s.Editor.Argv, " "); got != tt.editor || s.Editor.Terminal != tt.editorTerm {
				t.Errorf("editor = %q terminal %v, want %q terminal %v", got, s.Editor.Terminal, tt.editor, tt.editorTerm)
			}
			if got := strings.Join(s.Diff.Argv, " "); got != tt.diff || s.Diff.Terminal != tt.diffTerm {
				t.Errorf("diff = %q terminal %v, want %q terminal %v", got, s.Diff.Terminal, tt.diff, tt.diffTerm)
			}
		})
	}
}

func TestResolveProgramBadValues(t *testing.T) {
	getenv, home := env(t, map[string]string{EnvDiffTool: "hunk diff {branch}", EnvDiffTerminal: "maybe"})
	write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), `
[editor]
command = "zed '{path}"
comand = "typo"

[diff]
terminal = "yes"
`)
	s, err := Resolve(Flags{}, getenv, withHunk)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(s.Editor.Argv, " "); got != DefaultEditor {
		t.Errorf("editor = %q, want the default", got)
	}
	if got := strings.Join(s.Diff.Argv, " "); got != DefaultDiffTool || !s.Diff.Terminal {
		t.Errorf("diff = %q terminal %v, want the default", got, s.Diff.Terminal)
	}
	all := strings.Join(s.Problems, "\n")
	for _, want := range []string{
		`unknown key "editor.comand"`,
		`diff: `, // the table did not decode: terminal is not a bool
		`$` + EnvDiffTerminal + `: "maybe" is not true or false`,
		`$` + EnvDiffTool + `: unknown placeholder {branch}`,
		`editor.command: unterminated ' quote`,
	} {
		if !strings.Contains(all, want) {
			t.Errorf("problems lack %q:\n%s", want, all)
		}
	}

	if _, err := Resolve(Flags{Editor: "zed {file}"}, getenv, withHunk); err == nil || !strings.Contains(err.Error(), "--editor") {
		t.Errorf("a bad --editor must be an error, got %v", err)
	}
}

func TestResolveRelativeProjectsRootFlag(t *testing.T) {
	getenv, _ := env(t, nil)
	s, err := Resolve(Flags{ProjectsRoot: "rel/projects"}, getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	if want := filepath.Join(cwd, "rel", "projects"); s.ProjectsRoot != want {
		t.Errorf("ProjectsRoot = %q, want %q", s.ProjectsRoot, want)
	}
}

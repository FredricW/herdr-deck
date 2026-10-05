package config

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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

// fls are flags as given: pairs of a Spec.Key and its value, a string or
// a *bool; "" and nil are not given.
func fls(kv ...any) Flags {
	var fl Flags
	for i := 0; i < len(kv); i += 2 {
		key := kv[i].(string)
		switch v := kv[i+1].(type) {
		case string:
			if v != "" {
				fl.Set(key, "", v)
			}
		case *bool:
			if v != nil {
				fl.Set(key, "", strconv.FormatBool(*v))
			}
		}
	}
	return fl
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

func TestLogDir(t *testing.T) {
	state := t.TempDir()
	getenv, home := env(t, nil)
	if got, want := LogDir(getenv), filepath.Join(home, ".local", "state", "herdr-deck", "logs"); got != want {
		t.Errorf("LogDir = %q, want %q", got, want)
	}
	getenv, _ = env(t, map[string]string{"XDG_STATE_HOME": state})
	if got, want := LogDir(getenv), filepath.Join(state, "herdr-deck", "logs"); got != want {
		t.Errorf("LogDir with XDG_STATE_HOME = %q, want %q", got, want)
	}
	getenv, home = env(t, map[string]string{"XDG_STATE_HOME": "rel/state"})
	if got, want := LogDir(getenv), filepath.Join(home, ".local", "state", "herdr-deck", "logs"); got != want {
		t.Errorf("LogDir with a relative XDG_STATE_HOME = %q, want %q", got, want)
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
[linear]
workspace = "acme"

[ui]
refresh_interval = "30s"

[projects]
root = "~/work/projects"
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
linear.workspace = "from-file"
ui.refresh_interval = "20s"
projects.root = "/file/root"
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
		{"flag beats env", fls(KeyLinearWorkspace, "from-flag", KeyRefreshInterval, "10s", KeyProjectsRoot, "/flag/root"),
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
	write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), "[linear]\nworkspace = \"acme\"\n[ui]\nrefresh_interval = \n")
	s, err := Resolve(Flags{}, getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Problems) != 1 || !strings.Contains(s.Problems[0], "config: ~/.config/herdr-deck/config.toml: line 4") || !strings.Contains(s.Problems[0], "ignored") {
		t.Errorf("problems = %q, want one naming line 4", s.Problems)
	}
	if s.LinearWorkspace != "from-env" || s.RefreshInterval != DefaultRefreshInterval {
		t.Errorf("settings = %+v, want env and defaults", s)
	}
}

func TestResolveUnknownKeysAndBadValues(t *testing.T) {
	getenv, home := env(t, nil)
	write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), `
[linear]
workspace = "acme"
workpsace = "typo"

[ui]
refresh_interval = "soon"

[projects]
root = "relative/root"

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
	for _, want := range []string{`unknown key "linear.workpsace"`, `unknown key "update"`, `ui.refresh_interval: "soon" is not a duration`, `projects.root: "relative/root" is not an absolute path`} {
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
	write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), "[ui]\nrefresh_interval = \"2h\"\n")
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

	if _, err := Resolve(fls(KeyRefreshInterval, "0s"), getenv, noHunk); err == nil {
		t.Error("a bad --refresh-interval must be an error")
	}
	if s, err := Resolve(fls(KeyRefreshInterval, "1m"), getenv, noHunk); err != nil || s.RefreshInterval != time.Minute {
		t.Errorf("--refresh-interval 1m = %v, %v", s.RefreshInterval, err)
	}
}

func TestResolveDrawerHeight(t *testing.T) {
	getenv, home := env(t, map[string]string{EnvDrawerHeight: "1.5"})
	path := filepath.Join(home, ".config", "herdr-deck", "config.toml")
	write(t, path, "[ui]\ndrawer_height = 0.1\n")
	s, err := Resolve(Flags{}, getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	if s.DrawerHeight != DefaultDrawerHeight {
		t.Errorf("DrawerHeight = %v, want the default", s.DrawerHeight)
	}
	if len(s.Problems) != 2 || !strings.Contains(s.Problems[0], EnvDrawerHeight) || !strings.Contains(s.Problems[1], "0.1 is outside 0.2–0.8") {
		t.Errorf("problems = %q, want the env value then the file value", s.Problems)
	}
	if _, err := Resolve(fls(KeyDrawerHeight, "big"), getenv, noHunk); err == nil {
		t.Error("a bad --ui-drawer-height must be an error")
	}
	if s, err := Resolve(fls(KeyDrawerHeight, "0.3"), getenv, noHunk); err != nil || s.DrawerHeight != 0.3 {
		t.Errorf("--ui-drawer-height 0.3 = %v, %v", s.DrawerHeight, err)
	}

	// The settings page writes a TOML float, which reads back.
	if err := Save(path, KeyDrawerHeight, ptr(" .65 ")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "drawer_height = 0.65\n") {
		t.Errorf("file = %q", b)
	}
	noEnv := func(k string) string {
		if k == "HOME" {
			return home
		}
		return ""
	}
	if s, err := Resolve(Flags{}, noEnv, noHunk); err != nil || s.DrawerHeight != 0.65 || s.Values[KeyDrawerHeight].Text != "0.65" {
		t.Errorf("after saving 0.65: %v, %+v, %v", s.DrawerHeight, s.Values[KeyDrawerHeight], err)
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
			map[string]string{EnvEditorCommand: "zed {path}", EnvDiffCommand: "lazygit"},
			"zed {path}", false, "lazygit", true},
		{"env terminal with the file command", Flags{},
			map[string]string{EnvEditorTerminal: "false"},
			"nvim {path}", false, "difft-wrapper --base {base} {path}", false},
		{"flag beats env", fls(KeyEditorCommand, "cursor '{path}'", KeyDiffCommand, "tig", KeyDiffTerminal, &yes),
			map[string]string{EnvEditorCommand: "zed {path}", EnvEditorTerminal: "true", EnvDiffCommand: "lazygit"},
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
	getenv, home := env(t, map[string]string{EnvDiffCommand: "hunk diff {branch}", EnvDiffTerminal: "maybe"})
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
		`diff.terminal: `, // not a bool
		`$` + EnvDiffTerminal + `: "maybe" is not true or false`,
		`$` + EnvDiffCommand + `: unknown placeholder {branch}`,
		`editor.command: unterminated ' quote`,
	} {
		if !strings.Contains(all, want) {
			t.Errorf("problems lack %q:\n%s", want, all)
		}
	}

	if _, err := Resolve(fls(KeyEditorCommand, "zed {file}"), getenv, withHunk); err == nil || !strings.Contains(err.Error(), "--editor-command") {
		t.Errorf("a bad --editor-command must be an error, got %v", err)
	}
}

func TestResolveRelativeProjectsRootFlag(t *testing.T) {
	getenv, _ := env(t, nil)
	s, err := Resolve(fls(KeyProjectsRoot, "rel/projects"), getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	if want := filepath.Join(cwd, "rel", "projects"); s.ProjectsRoot != want {
		t.Errorf("ProjectsRoot = %q, want %q", s.ProjectsRoot, want)
	}
}

func TestResolveReuseTabs(t *testing.T) {
	off, on := false, true
	tests := []struct {
		name     string
		flag     *bool
		env      string
		file     string
		want     bool
		problems int
	}{
		{name: "default", want: true},
		{name: "file", file: "[browser]\nreuse_tabs = false\n", want: false},
		{name: "env beats file", env: "true", file: "[browser]\nreuse_tabs = false\n", want: true},
		{name: "flag beats env", flag: &off, env: "true", want: false},
		{name: "flag on", flag: &on, file: "[browser]\nreuse_tabs = false\n", want: true},
		{name: "bad env falls back to file", env: "maybe", file: "[browser]\nreuse_tabs = false\n", want: false, problems: 1},
		{name: "bad file value", file: "[browser]\nreuse_tabs = \"no\"\n", want: true, problems: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv, home := env(t, map[string]string{EnvReuseTabs: tt.env})
			if tt.file != "" {
				write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), tt.file)
			}
			s, err := Resolve(fls(KeyReuseTabs, tt.flag), getenv, noHunk)
			if err != nil {
				t.Fatal(err)
			}
			if s.ReuseTabs != tt.want || len(s.Problems) != tt.problems {
				t.Errorf("ReuseTabs = %v with problems %q; want %v with %d", s.ReuseTabs, s.Problems, tt.want, tt.problems)
			}
		})
	}
}

func TestResolveSwitches(t *testing.T) {
	off, on := false, true
	tests := []struct {
		name       string
		flag       *bool
		vars       map[string]string
		file       string
		want       bool
		problemHas string
	}{
		{"default on", nil, nil, "", true, ""},
		{"file", nil, nil, "[updates]\ncheck = false\nauto_restart = false\n", false, ""},
		{"env beats file", nil, map[string]string{EnvUpdateCheck: "true", EnvAutoRestart: "1"}, "[updates]\ncheck = false\nauto_restart = false\n", true, ""},
		{"flag beats env", &off, map[string]string{EnvUpdateCheck: "true", EnvAutoRestart: "true"}, "", false, ""},
		{"flag on", &on, nil, "[updates]\ncheck = false\nauto_restart = false\n", true, ""},
		{"bad env falls back to file", nil, map[string]string{EnvUpdateCheck: "maybe", EnvAutoRestart: "maybe"}, "[updates]\ncheck = false\nauto_restart = false\n", false, `"maybe" is not true or false`},
		{"bad file value", nil, nil, "[updates]\ncheck = \"no\"\nauto_restart = 0\n", true, "updates.check"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv, home := env(t, tt.vars)
			if tt.file != "" {
				write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), tt.file)
			}
			s, err := Resolve(fls(KeyUpdateCheck, tt.flag, KeyAutoRestart, tt.flag), getenv, noHunk)
			if err != nil {
				t.Fatal(err)
			}
			if s.UpdateCheck != tt.want || s.AutoRestart != tt.want {
				t.Errorf("UpdateCheck, AutoRestart = %v, %v; want %v", s.UpdateCheck, s.AutoRestart, tt.want)
			}
			got := strings.Join(s.Problems, "\n")
			if tt.problemHas == "" && got != "" {
				t.Errorf("problems = %q, want none", got)
			}
			if !strings.Contains(got, tt.problemHas) {
				t.Errorf("problems = %q, want one containing %q", got, tt.problemHas)
			}
		})
	}
}

func TestResolveFigmaDesktop(t *testing.T) {
	tests := []struct {
		name, file, env string
		want            bool
		problems        int
	}{
		{"default off", "", "", false, 0},
		{"file", "figma.desktop = true\n", "", true, 0},
		{"env beats file", "figma.desktop = true\n", "false", false, 0},
		{"bad env falls back to the file", "figma.desktop = true\n", "yes please", true, 1},
		{"bad file value", "figma.desktop = \"yes\"\n", "", false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv, home := env(t, map[string]string{EnvFigmaDesktop: tt.env})
			write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), tt.file)
			s, err := Resolve(Flags{}, getenv, noHunk)
			if err != nil {
				t.Fatal(err)
			}
			if s.FigmaDesktop != tt.want || len(s.Problems) != tt.problems {
				t.Errorf("FigmaDesktop = %v, problems %q; want %v and %d problems", s.FigmaDesktop, s.Problems, tt.want, tt.problems)
			}
		})
	}
}

func TestResolveDiffView(t *testing.T) {
	tests := []struct {
		name, file, env, flag string
		want                  string
		problems              int
	}{
		{"default list", "", "", "", DiffViewList, 0},
		{"file", "[diff]\nview = \"tree\"\n", "", "", DiffViewTree, 0},
		{"env beats file", "[diff]\nview = \"tree\"\n", "list", "", DiffViewList, 0},
		{"flag beats env", "", "list", "tree", DiffViewTree, 0},
		{"bad env falls back to the file", "[diff]\nview = \"tree\"\n", "grid", "", DiffViewTree, 1},
		{"bad file value", "[diff]\nview = \"folders\"\n", "", "", DiffViewList, 1},
		{"wrong type", "[diff]\nview = true\n", "", "", DiffViewList, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv, home := env(t, map[string]string{EnvDiffView: tt.env})
			write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), tt.file)
			s, err := Resolve(fls(KeyDiffView, tt.flag), getenv, noHunk)
			if err != nil {
				t.Fatal(err)
			}
			if s.DiffView != tt.want || len(s.Problems) != tt.problems {
				t.Errorf("DiffView = %q, problems %q; want %q and %d problems", s.DiffView, s.Problems, tt.want, tt.problems)
			}
		})
	}
	getenv, _ := env(t, nil)
	if _, err := Resolve(fls(KeyDiffView, "grid"), getenv, noHunk); err == nil || err.Error() != `--diff-view: "grid" is not list or tree` {
		t.Errorf("bad flag: %v", err)
	}
}

func TestResolveDiffLayout(t *testing.T) {
	tests := []struct {
		name, file, env, flag string
		want                  string
		problems              int
	}{
		{"default unified", "", "", "", DiffLayoutUnified, 0},
		{"file", "[diff]\nlayout = \"split\"\n", "", "", DiffLayoutSplit, 0},
		{"env beats file", "[diff]\nlayout = \"split\"\n", "unified", "", DiffLayoutUnified, 0},
		{"flag beats env", "", "unified", "split", DiffLayoutSplit, 0},
		{"bad env falls back to the file", "[diff]\nlayout = \"split\"\n", "columns", "", DiffLayoutSplit, 1},
		{"bad file value", "[diff]\nlayout = \"side\"\n", "", "", DiffLayoutUnified, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv, home := env(t, map[string]string{EnvDiffLayout: tt.env})
			write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), tt.file)
			s, err := Resolve(fls(KeyDiffLayout, tt.flag), getenv, noHunk)
			if err != nil {
				t.Fatal(err)
			}
			if s.DiffLayout != tt.want || len(s.Problems) != tt.problems {
				t.Errorf("DiffLayout = %q, problems %q; want %q and %d problems", s.DiffLayout, s.Problems, tt.want, tt.problems)
			}
		})
	}
	getenv, _ := env(t, nil)
	if _, err := Resolve(fls(KeyDiffLayout, "side"), getenv, noHunk); err == nil || err.Error() != `--diff-layout: "side" is not unified or split` {
		t.Errorf("bad flag: %v", err)
	}
}

func TestResolveFoldedLists(t *testing.T) {
	tests := []struct {
		name, file, env, flag string
		want                  []string
		text                  string
		problems              int
	}{
		{"default", "", "", "", []string{"Backlog", "Resolved"}, "Backlog, Resolved", 0},
		{"file", "[ui]\nfolded_lists = [\"Later\", \" Done \"]\n", "", "", []string{"Later", "Done"}, "Later, Done", 0},
		{"empty file list folds none", "ui.folded_lists = []\n", "", "", []string{}, "none", 0},
		{"env beats file", "[ui]\nfolded_lists = [\"Later\"]\n", "Icebox, backlog", "", []string{"Icebox", "backlog"}, "Icebox, backlog", 0},
		{"env none", "", "None", "", []string{}, "none", 0},
		{"flag beats env", "", "Icebox", "Later,Done", []string{"Later", "Done"}, "Later, Done", 0},
		{"repeats dropped, ignoring case", "", "", "Backlog, backlog", []string{"Backlog"}, "Backlog", 0},
		{"bad env falls back to the file", "[ui]\nfolded_lists = [\"Later\"]\n", " , ", "", []string{"Later"}, "Later", 1},
		{"empty name in the file", "[ui]\nfolded_lists = [\"\"]\n", "", "", []string{"Backlog", "Resolved"}, "Backlog, Resolved", 1},
		{"unknown key in [ui] is skipped on its own", "[ui]\nfolded_lists = [\"Later\"]\ncolour = 1\n", "", "", []string{"Later"}, "Later", 1},
		{"ui not a table", "ui = 3\n", "", "", []string{"Backlog", "Resolved"}, "Backlog, Resolved", 1},
		{"wrong type", "[ui]\nfolded_lists = \"Backlog\"\n", "", "", []string{"Backlog", "Resolved"}, "Backlog, Resolved", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv, home := env(t, map[string]string{EnvFoldedLists: tt.env})
			write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), tt.file)
			s, err := Resolve(fls(KeyFoldedLists, tt.flag), getenv, noHunk)
			if err != nil {
				t.Fatal(err)
			}
			if s.FoldedLists == nil || !slices.Equal(s.FoldedLists, tt.want) || s.Values[KeyFoldedLists].Text != tt.text || len(s.Problems) != tt.problems {
				t.Errorf("FoldedLists = %q (%q), problems %q; want %q (%q) and %d problems",
					s.FoldedLists, s.Values[KeyFoldedLists].Text, s.Problems, tt.want, tt.text, tt.problems)
			}
		})
	}
	for file, hand := range map[string]bool{
		"[ui]\nfolded_lists = [\"Ideas, later\"]\n":    true,
		"[ui]\nfolded_lists = [\"none\"]\n":            true,
		"[ui]\nfolded_lists = [\"none\", \"Later\"]\n": false,
	} {
		getenv, home := env(t, nil)
		write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), file)
		s, err := Resolve(Flags{}, getenv, noHunk)
		if err != nil {
			t.Fatal(err)
		}
		if got := s.Values[KeyFoldedLists].HandEdit; got != hand {
			t.Errorf("%q: HandEdit = %v, want %v", file, got, hand)
		}
	}
	getenv, _ := env(t, nil)
	if _, err := Resolve(fls(KeyFoldedLists, ","), getenv, noHunk); err == nil || err.Error() != "--ui-folded-lists: a list name is empty" {
		t.Errorf("bad flag: %v", err)
	}
}

func TestCacheDir(t *testing.T) {
	getenv, home := env(t, nil)
	if got, want := CacheDir(getenv), filepath.Join(home, ".cache", "herdr-deck"); got != want {
		t.Errorf("CacheDir = %q, want %q", got, want)
	}
	getenv, _ = env(t, map[string]string{"XDG_CACHE_HOME": "/var/cache/me"})
	if got, want := CacheDir(getenv), filepath.Join("/var/cache/me", "herdr-deck"); got != want {
		t.Errorf("CacheDir = %q, want %q", got, want)
	}
}

func TestResolveLinearStatus(t *testing.T) {
	getenv, home := env(t, nil)
	s, err := Resolve(Flags{}, getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	if !s.LinearStatus || s.LinearAPIKeyCommand != nil {
		t.Errorf("default: LinearStatus = %v, command = %q; want on, none", s.LinearStatus, s.LinearAPIKeyCommand)
	}

	write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), `
[linear]
status = false
api_key_command = "op read 'op://Private/Linear API/credential'"
`)
	s, err = Resolve(Flags{}, getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Problems) != 0 {
		t.Errorf("problems = %q", s.Problems)
	}
	want := []string{"op", "read", "op://Private/Linear API/credential"}
	if s.LinearStatus || !slices.Equal(s.LinearAPIKeyCommand, want) {
		t.Errorf("file: LinearStatus = %v, command = %q; want off, %q", s.LinearStatus, s.LinearAPIKeyCommand, want)
	}

	// The environment variable wins over the file; a bad one falls back to it.
	getenv2 := func(k string) string {
		if k == EnvLinearStatus {
			return "true"
		}
		return getenv(k)
	}
	if s, _ = Resolve(Flags{}, getenv2, noHunk); !s.LinearStatus {
		t.Error("$" + EnvLinearStatus + "=true did not win over the file")
	}
	getenv3 := func(k string) string {
		if k == EnvLinearStatus {
			return "maybe"
		}
		return getenv(k)
	}
	if s, _ = Resolve(Flags{}, getenv3, noHunk); s.LinearStatus || len(s.Problems) != 1 {
		t.Errorf("bad env: LinearStatus = %v, problems = %q; want the file's false and one problem", s.LinearStatus, s.Problems)
	}
}

// A command that does not parse is reported without quoting it: a key
// pasted there by mistake must not reach the Sources view.
func TestResolveLinearKeyCommandNeverQuoted(t *testing.T) {
	getenv, home := env(t, nil)
	write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), `
[linear]
api_key_command = "lin_api_SECRETSECRET 'unterminated"
`)
	s, err := Resolve(Flags{}, getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	if s.LinearAPIKeyCommand != nil || len(s.Problems) != 1 {
		t.Fatalf("command = %q, problems = %q", s.LinearAPIKeyCommand, s.Problems)
	}
	if strings.Contains(s.Problems[0], "SECRET") || !strings.Contains(s.Problems[0], "linear.api_key_command") {
		t.Errorf("problem = %q", s.Problems[0])
	}
}

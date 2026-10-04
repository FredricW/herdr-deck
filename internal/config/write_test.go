package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func ptr(s string) *string { return &s }

// sample is a hand-written file with comments, an unknown key and an
// unknown table, which every save must keep.
const sample = `# herdr-deck settings

# Bare IDs link here.
linear_workspace = "acme"   # the team's workspace
future_key = 3

# Reload often.
refresh_interval = "5s"

[editor]
# Zed, not VS Code.
command = "zed {path}"

[plugins.extra]
name = "kept"
`

func TestSaveKeepsCommentsAndUnknownKeys(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value *string
		want  string
	}{
		{"replace keeps the trailing comment", KeyLinearWorkspace, ptr("globex"),
			strings.Replace(sample, `"acme"`, `"globex"`, 1)},
		{"replace in a table", KeyEditorCommand, ptr("nvim {path}"),
			strings.Replace(sample, `"zed {path}"`, `"nvim {path}"`, 1)},
		{"new top-level key after the last one", KeyFigmaDesktop, ptr("true"),
			strings.Replace(sample, "refresh_interval = \"5s\"\n", "refresh_interval = \"5s\"\nfigma_desktop = true\n", 1)},
		{"new key in an existing table", KeyEditorTerminal, ptr("T"),
			strings.Replace(sample, "command = \"zed {path}\"\n", "command = \"zed {path}\"\nterminal = true\n", 1)},
		{"new table at the end", KeyDiffCommand, ptr(`git diff "{base}"`),
			sample + "\n[diff]\ncommand = 'git diff \"{base}\"'\n"},
		{"remove a key", KeyRefreshInterval, nil,
			strings.Replace(sample, "refresh_interval = \"5s\"\n", "", 1)},
		{"remove a missing key changes nothing", KeyAutoRestart, nil, sample},
		{"same value changes nothing", KeyLinearWorkspace, ptr("acme"), sample},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "herdr-deck", "config.toml")
			write(t, path, sample)
			if err := Save(path, tt.key, tt.value); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(path)
			if string(got) != tt.want {
				t.Errorf("file =\n%s\nwant\n%s", got, tt.want)
			}
			// The deck still reads every other value from it.
			if _, problems := Load(path, true); len(problems) != 1 { // future_key
				t.Errorf("problems = %q, want only future_key", problems)
			}
		})
	}
}

func TestSaveCreatesFileAndFolder(t *testing.T) {
	getenv, home := env(t, map[string]string{})
	path, _ := Path("", getenv)
	if !strings.HasPrefix(path, home) {
		t.Fatalf("path %q is outside the test home %q", path, home)
	}
	if err := Save(path, KeyEditorCommand, ptr("zed {path}")); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, KeyLinearWorkspace, ptr("acme")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	want := "linear_workspace = \"acme\"\n\n[editor]\ncommand = \"zed {path}\"\n"
	if string(got) != want {
		t.Errorf("file =\n%q\nwant\n%q", got, want)
	}
	s, err := Resolve(Flags{}, getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	if s.LinearWorkspace != "acme" || s.Editor.Argv[0] != "zed" || len(s.Problems) > 0 {
		t.Errorf("Resolve after Save = %q, %q, %q", s.LinearWorkspace, s.Editor.Argv, s.Problems)
	}
	if runtime.GOOS != "windows" {
		if st, _ := os.Stat(path); st.Mode().Perm() != 0o644 {
			t.Errorf("mode = %v, want 0644", st.Mode().Perm())
		}
	}
	// No temp file is left behind.
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("folder holds %d files, want only config.toml", len(entries))
	}
}

func TestSaveKeepsModeAndWritesThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "dotfiles", "deck.toml")
	write(t, real, "update_check = true\n")
	if err := os.Chmod(real, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "config", "config.toml")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Skip("no symlinks:", err)
	}
	if err := Save(link, KeyUpdateCheck, ptr("false")); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Error("the symlink was replaced by a file")
	}
	got, _ := os.ReadFile(real)
	if string(got) != "update_check = false\n" {
		t.Errorf("file = %q", got)
	}
	if st, _ := os.Stat(real); st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600 kept", st.Mode().Perm())
	}
}

func TestSaveLayouts(t *testing.T) {
	tests := []struct {
		name, file, key string
		value           *string
		want            string
	}{
		{"top-level key goes above the first table and its comments", "# Editor\n[editor]\ncommand = \"vi\"\n", KeyUpdateCheck, ptr("false"),
			"update_check = false\n\n# Editor\n[editor]\ncommand = \"vi\"\n"},
		{"dotted keys stay dotted", "editor.command = \"vi\"\n", KeyEditorTerminal, ptr("true"),
			"editor.command = \"vi\"\neditor.terminal = true\n"},
		{"dotted prefix added once", "editor.command = \"vi\"\neditor.extra = 1\n", KeyEditorTerminal, ptr("true"),
			"editor.command = \"vi\"\neditor.extra = 1\neditor.terminal = true\n"},
		{"dotted key replaced", "editor.command = \"vi\"\n", KeyEditorCommand, ptr("nvim"),
			"editor.command = \"nvim\"\n"},
		{"CRLF kept", "update_check = true\r\n", KeyAutoRestart, ptr("false"),
			"update_check = true\r\nauto_restart = false\r\n"},
		{"no final newline", "update_check = true", KeyAutoRestart, ptr("false"),
			"update_check = true\nauto_restart = false\n"},
		{"multi-line string replaced whole", "linear_api_key_command = \"\"\"\nop read x\"\"\"\nupdate_check = true\n", KeyLinearAPIKeyCommand, ptr("op read op://Vault/Item/key"),
			"linear_api_key_command = \"op read op://Vault/Item/key\"\nupdate_check = true\n"},
		{"array before is skipped", "list = [\n  \"a\", # [x]\n  \"b\",\n]\n[editor]\n", KeyEditorCommand, ptr("vi"),
			"list = [\n  \"a\", # [x]\n  \"b\",\n]\n[editor]\ncommand = \"vi\"\n"},
		{"quoted key matched", "\"linear_workspace\" = 'acme'\n", KeyLinearWorkspace, ptr("globex"),
			"\"linear_workspace\" = \"globex\"\n"},
		{"key in an array of tables is not ours", "[[editor]]\ncommand = \"vi\"\n", KeyUpdateCheck, ptr("true"),
			"update_check = true\n\n[[editor]]\ncommand = \"vi\"\n"},
		{"list written as an array", "", KeyFoldedLists, ptr(" Backlog ,Done, backlog"),
			"[ui]\nfolded_lists = [\"Backlog\", \"Done\"]\n"},
		{"none is an empty array", "[ui]\nfolded_lists = [\n  \"Backlog\",\n]\nother = true\n", KeyFoldedLists, ptr("none"),
			"[ui]\nfolded_lists = []\nother = true\n"},
		{"remove the last key of a table", "[editor]\ncommand = \"vi\"\n", KeyEditorCommand, nil,
			"[editor]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			write(t, path, tt.file)
			if err := Save(path, tt.key, tt.value); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(path)
			if string(got) != tt.want {
				t.Errorf("file =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestSaveRefuses(t *testing.T) {
	tests := []struct {
		name, file, key string
		value           *string
		err             string
	}{
		{"bad duration", "", KeyRefreshInterval, ptr("soon"), "is not a duration"},
		{"duration out of range", "", KeyRefreshInterval, ptr("1h"), "outside"},
		{"unknown placeholder", "", KeyEditorCommand, ptr("zed {file}"), "unknown placeholder {file}"},
		{"unbalanced quote", "", KeyDiffCommand, ptr(`git diff "{base}`), "quote"},
		{"relative path", "", KeyProjectsRoot, ptr("projects"), "not an absolute path"},
		{"not a bool", "", KeyReuseTabs, ptr("maybe"), "not true or false"},
		{"workspace with a slash", "", KeyLinearWorkspace, ptr("acme/team"), "single word"},
		{"empty", "", KeyLinearWorkspace, ptr("  "), "needs a value"},
		{"empty list name", "", KeyFoldedLists, ptr("Backlog,,Done"), "a list name is empty"},
		{"unknown key", "", "colour", ptr("red"), "unknown setting"},
		{"broken file", "editor = [\n", KeyUpdateCheck, ptr("true"), "does not parse"},
		{"inline table", "editor = { command = \"vi\" }\n", KeyEditorTerminal, ptr("true"), "edit editor.terminal by hand"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			write(t, path, tt.file)
			err := Save(path, tt.key, tt.value)
			if err == nil || !strings.Contains(err.Error(), tt.err) {
				t.Fatalf("Save = %v, want an error with %q", err, tt.err)
			}
			got, _ := os.ReadFile(path)
			if string(got) != tt.file {
				t.Errorf("the file changed to %q", got)
			}
		})
	}
}

func TestTOMLString(t *testing.T) {
	for in, want := range map[string]string{
		"code {path}":         `"code {path}"`,
		`sh -c "x"`:           `'sh -c "x"'`,
		`it's "quoted"`:       `"it's \"quoted\""`,
		"tab\there":           `"tab\there"`,
		`C:\path`:             `'C:\path'`,
		"op read op://a/b/cd": `"op read op://a/b/cd"`,
	} {
		if got := tomlString(in); got != want {
			t.Errorf("tomlString(%q) = %s, want %s", in, got, want)
		}
	}
}

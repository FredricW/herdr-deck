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
future_key = 3

[linear]
# Bare IDs link here.
workspace = "acme"   # the team's workspace

[ui]
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
		{"new key after the last one of its table", KeyFoldedLists, ptr("Later"),
			strings.Replace(sample, "refresh_interval = \"5s\"\n", "refresh_interval = \"5s\"\nfolded_lists = [\"Later\"]\n", 1)},
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

// oldSample is sample with the settings under their earlier flat keys.
const oldSample = `# herdr-deck settings

# Bare IDs link here.
linear_workspace = "acme"   # the team's workspace
future_key = 3

# Reload often.
refresh_interval = "5s"
update_check = false

[editor]
# Zed, not VS Code.
command = "zed {path}"
`

// Saving a key the file sets under its earlier name moves it into its
// table, with its comments; removing it removes both forms.
func TestSaveMovesOldKeys(t *testing.T) {
	tests := []struct {
		name, file, key string
		value           *string
		want            string
	}{
		{"moved with its comments", oldSample, KeyLinearWorkspace, ptr("globex"), `# herdr-deck settings

future_key = 3

# Reload often.
refresh_interval = "5s"
update_check = false

[editor]
# Zed, not VS Code.
command = "zed {path}"

[linear]
# Bare IDs link here.
workspace = "globex"   # the team's workspace
`},
		{"same value still moves", "update_check = false\n[updates]\nauto_restart = true\n", KeyUpdateCheck, ptr("false"),
			"[updates]\nauto_restart = true\ncheck = false\n"},
		{"switch toggled", "refresh_interval = \"5s\"\nupdate_check = false\n", KeyUpdateCheck, ptr("true"),
			"refresh_interval = \"5s\"\n\n[updates]\ncheck = true\n"},
		{"removed", oldSample, KeyRefreshInterval, nil,
			strings.Replace(oldSample, "# Reload often.\nrefresh_interval = \"5s\"\n", "", 1)},
		{"both set: the old line goes, the table's value changes", "# old\nlinear_workspace = \"a\"\n\n[linear]\nworkspace = \"b\"\n", KeyLinearWorkspace, ptr("c"),
			"# old\n\n[linear]\nworkspace = \"c\"\n"},
		{"both set and removed", "linear_workspace = \"a\"\n\n[linear]\nworkspace = \"b\"\n", KeyLinearWorkspace, nil,
			"\n[linear]\n"},
		{"the file's opening comment stays", "# My deck.\nlinear_workspace = \"a\"\n", KeyLinearWorkspace, ptr("b"),
			"# My deck.\n\n[linear]\nworkspace = \"b\"\n"},
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
				t.Errorf("file =\n%s\nwant\n%s", got, tt.want)
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
	want := "[editor]\ncommand = \"zed {path}\"\n\n[linear]\nworkspace = \"acme\"\n"
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
	write(t, real, "[updates]\ncheck = true\n")
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
	if string(got) != "[updates]\ncheck = false\n" {
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
		{"dotted keys stay dotted", "editor.command = \"vi\"\n", KeyEditorTerminal, ptr("true"),
			"editor.command = \"vi\"\neditor.terminal = true\n"},
		{"dotted prefix added once", "editor.command = \"vi\"\neditor.extra = 1\n", KeyEditorTerminal, ptr("true"),
			"editor.command = \"vi\"\neditor.extra = 1\neditor.terminal = true\n"},
		{"dotted key replaced", "editor.command = \"vi\"\n", KeyEditorCommand, ptr("nvim"),
			"editor.command = \"nvim\"\n"},
		{"CRLF kept", "[updates]\r\ncheck = true\r\n", KeyAutoRestart, ptr("false"),
			"[updates]\r\ncheck = true\r\nauto_restart = false\r\n"},
		{"new table after CRLF", "x = 1\r\n", KeyAutoRestart, ptr("false"),
			"x = 1\r\n\r\n[updates]\r\nauto_restart = false\r\n"},
		{"no final newline", "[updates]\ncheck = true", KeyAutoRestart, ptr("false"),
			"[updates]\ncheck = true\nauto_restart = false\n"},
		{"multi-line string replaced whole", "[linear]\napi_key_command = \"\"\"\nop read x\"\"\"\nstatus = true\n", KeyLinearAPIKeyCommand, ptr("op read op://Vault/Item/key"),
			"[linear]\napi_key_command = \"op read op://Vault/Item/key\"\nstatus = true\n"},
		{"array before is skipped", "list = [\n  \"a\", # [x]\n  \"b\",\n]\n[editor]\n", KeyEditorCommand, ptr("vi"),
			"list = [\n  \"a\", # [x]\n  \"b\",\n]\n[editor]\ncommand = \"vi\"\n"},
		{"quoted key matched", "[\"linear\"]\n\"workspace\" = 'acme'\n", KeyLinearWorkspace, ptr("globex"),
			"[\"linear\"]\n\"workspace\" = \"globex\"\n"},
		{"key in an array of tables is not ours", "[[editor]]\ncommand = \"vi\"\n", KeyUpdateCheck, ptr("true"),
			"[[editor]]\ncommand = \"vi\"\n\n[updates]\ncheck = true\n"},
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
		{"an array of tables where the table goes", "[[updates]]\ncheck = false\n", KeyUpdateCheck, ptr("true"), "would break the file"},
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

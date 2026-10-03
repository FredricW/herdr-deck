package config

import (
	"path/filepath"
	"testing"
)

func TestResolveValuesAndSources(t *testing.T) {
	getenv, home := env(t, map[string]string{
		EnvEditor:          "nvim {path}",
		EnvEditorTerminal:  "true",
		EnvRefreshInterval: "soon", // bad: the file's value is used
		EnvLinearAPIKey:    "lin_api_never_shown",
	})
	write(t, filepath.Join(home, ".config", "herdr-deck", "config.toml"), `
linear_workspace = "acme"
refresh_interval = "30s"
linear_api_key_command = "op read op://Vault/Linear/key"
figma_desktop = true
diff_view = "tree"

[editor]
command = "zed {path}"

[diff]
terminal = false
`)
	no := false
	s, err := Resolve(Flags{UpdateCheck: &no, DiffTool: "git diff {base}"}, getenv, noHunk)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Value{
		KeyLinearWorkspace:     {Text: "acme", Source: FromFile},
		KeyLinearStatus:        {Text: "true", Source: FromDefault},
		KeyLinearAPIKeyCommand: {Text: "op read op://Vault/Linear/key", Source: FromFile, Note: "$LINEAR_API_KEY is set and wins over the command"},
		KeyEditorCommand:       {Text: "nvim {path}", Source: FromEnv},
		KeyEditorTerminal:      {Text: "true", Source: FromEnv},
		// The flag gives the command, so the file's terminal is not used.
		KeyDiffCommand:     {Text: "git diff {base}", Source: FromFlag},
		KeyDiffTerminal:    {Text: "true", Source: FromDefault},
		KeyDiffView:        {Text: "tree", Source: FromFile},
		KeyUpdateCheck:     {Text: "false", Source: FromFlag},
		KeyAutoRestart:     {Text: "true", Source: FromDefault},
		KeyReuseTabs:       {Text: "true", Source: FromDefault},
		KeyFigmaDesktop:    {Text: "true", Source: FromFile},
		KeyRefreshInterval: {Text: "30s", Source: FromFile},
		KeyProjectsRoot:    {Text: filepath.Join(home, ".herdr-projects"), Source: FromDefault},
	}
	for _, sp := range Specs {
		got := s.Values[sp.Key]
		got.File, got.InFile = "", false
		if got != want[sp.Key] {
			t.Errorf("%s = %+v, want %+v", sp.Key, got, want[sp.Key])
		}
	}
	// The file's own values show even where a flag or env var hides them.
	for k, f := range map[string]string{KeyEditorCommand: "zed {path}", KeyDiffTerminal: "false", KeyRefreshInterval: "30s"} {
		if v := s.Values[k]; !v.InFile || v.File != f {
			t.Errorf("%s file value = %q, %v; want %q", k, v.File, v.InFile, f)
		}
	}
	if v := s.Values[KeyUpdateCheck]; v.InFile {
		t.Errorf("update_check is not in the file, got %q", v.File)
	}
	if len(s.Values) != len(Specs) {
		t.Errorf("%d values for %d specs", len(s.Values), len(Specs))
	}
	for _, v := range s.Values {
		if v.Text == "lin_api_never_shown" || v.Note == "lin_api_never_shown" {
			t.Error("the API key shows in a value")
		}
	}
}

func TestResolveDefaultSources(t *testing.T) {
	getenv, _ := env(t, nil)
	s, err := Resolve(Flags{}, getenv, withHunk)
	if err != nil {
		t.Fatal(err)
	}
	for _, sp := range Specs {
		if v := s.Values[sp.Key]; v.Source != FromDefault {
			t.Errorf("%s comes from %v, want the default", sp.Key, v.Source)
		}
	}
	if got := s.Values[KeyDiffCommand].Text; got != DefaultDiffTool {
		t.Errorf("diff.command = %q, want %q", got, DefaultDiffTool)
	}
	if got := s.Values[KeyLinearWorkspace].Text; got != "" {
		t.Errorf("linear_workspace = %q, want none", got)
	}
}

func TestSpecs(t *testing.T) {
	seen := map[string]bool{}
	for _, sp := range Specs {
		if seen[sp.Key] {
			t.Errorf("%s twice", sp.Key)
		}
		seen[sp.Key] = true
		if sp.Group == "" || sp.Help == "" {
			t.Errorf("%s has no group or help", sp.Key)
		}
	}
	for _, sp := range Specs {
		if (sp.Kind == KindChoice) != (len(sp.Choices) > 0) {
			t.Errorf("%s: kind %v with choices %q", sp.Key, sp.Kind, sp.Choices)
		}
	}
	if err := Check(KeyDiffView, "tree"); err != nil {
		t.Errorf("Check(diff_view, tree) = %v", err)
	}
	if err := Check(KeyDiffView, "grid"); err == nil || err.Error() != `"grid" is not list or tree` {
		t.Errorf("Check(diff_view, grid) = %v", err)
	}
	if sp, _ := SpecFor(KeyEditorCommand); sp.Override(FromFlag) != "--editor" || sp.Override(FromEnv) != "$"+EnvEditor || sp.Override(FromFile) != "" {
		t.Errorf("Override = %q, %q", sp.Override(FromFlag), sp.Override(FromEnv))
	}
}

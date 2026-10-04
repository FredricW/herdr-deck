package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigMigrate(t *testing.T) {
	// Resolved, as the .bak path is: on macOS /var is a symlink.
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	getenv := func(k string) string {
		if k == "HOME" {
			return home
		}
		return ""
	}
	path := filepath.Join(home, ".config", "herdr-deck", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# Mine.\nlinear_workspace = \"acme\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := runConfig([]string{"migrate"}, getenv, &out); err != nil {
		t.Fatal(err)
	}
	want := "Would move 1 setting(s) in " + path + ":\n" +
		"  linear_workspace → linear.workspace\n\n" +
		"--- " + path + "\n+++ " + path + " (migrated)\n" +
		"@@ -1,2 +1,4 @@\n # Mine.\n-linear_workspace = \"acme\"\n+\n+[linear]\n+workspace = \"acme\"\n\n" +
		"Nothing was written: run `herdr-deck config migrate --write` to apply it, keeping the old file as .bak.\n"
	if out.String() != want {
		t.Errorf("dry run printed\n%s\nwant\n%s", out.String(), want)
	}
	if b, _ := os.ReadFile(path); string(b) != "# Mine.\nlinear_workspace = \"acme\"\n" {
		t.Errorf("the dry run wrote %q", b)
	}

	other := filepath.Join(home, "other.toml")
	if err := os.WriteFile(other, []byte("update_check = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The hint keeps the file named.
	out.Reset()
	if err := runConfig([]string{"migrate", "--config", other}, getenv, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "run `herdr-deck config migrate --config "+other+" --write`") {
		t.Errorf("dry run hint lacks --config:\n%s", out.String())
	}
	out.Reset()
	if err := runConfig([]string{"migrate", "--config", other, "--write"}, getenv, &out); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(other); string(b) != "[updates]\ncheck = false\n" {
		t.Errorf("--write wrote %q", b)
	}
	if b, _ := os.ReadFile(other + ".bak"); string(b) != "update_check = false\n" {
		t.Errorf(".bak = %q", b)
	}
	if !strings.HasPrefix(out.String(), "Moved 1 setting(s) in "+other) || !strings.HasSuffix(out.String(), "kept as "+other+".bak.\n") {
		t.Errorf("--write printed\n%s", out.String())
	}

	// A symlinked file keeps its .bak next to the real one, and says so.
	real := filepath.Join(home, "dotfiles", "deck.toml")
	link := filepath.Join(home, "link.toml")
	if err := os.MkdirAll(filepath.Dir(real), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, []byte("diff_view = \"tree\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err == nil {
		out.Reset()
		if err := runConfig([]string{"migrate", "--config", link, "--write"}, getenv, &out); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(real + ".bak"); err != nil || !strings.HasSuffix(out.String(), "kept as "+real+".bak.\n") {
			t.Errorf("symlink: .bak %v, printed\n%s", err, out.String())
		}
	}

	out.Reset()
	if err := runConfig([]string{"migrate", "--config", other}, getenv, &out); err != nil || !strings.Contains(out.String(), "nothing to migrate") {
		t.Errorf("second run: %v, %q", err, out.String())
	}
	out.Reset()
	if err := runConfig([]string{"migrate", "--config", filepath.Join(home, "none.toml")}, getenv, &out); err != nil || !strings.Contains(out.String(), "no config file") {
		t.Errorf("missing file: %v, %q", err, out.String())
	}
	if err := runConfig([]string{"show"}, getenv, &out); err == nil {
		t.Error("an unknown config command must be an error")
	}
}

// run hands `config` to runConfig rather than starting a deck.
func TestRunDispatchesConfig(t *testing.T) {
	if err := run([]string{"config", "show"}); err == nil || !strings.Contains(err.Error(), "config migrate") {
		t.Errorf("run(config show) = %v, want runConfig's error", err)
	}
	if err := run([]string{"--fake", "confg"}); err == nil || !strings.Contains(err.Error(), `unexpected argument "confg"`) {
		t.Errorf("run with a stray argument = %v", err)
	}
}

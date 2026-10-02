package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// herdr-plugin.toml's build must stamp the manifest version into the
// binary: -X silently does nothing when main.version does not exist.
func TestManifestBuildStampsVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	var m struct {
		Version string `toml:"version"`
		Build   []struct {
			Command []string `toml:"command"`
		} `toml:"build"`
	}
	if _, err := toml.DecodeFile(filepath.Join("..", "..", "herdr-plugin.toml"), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Build) != 1 || len(m.Build[0].Command) < 2 || m.Build[0].Command[0] != "go" {
		t.Fatalf("build = %v, want one go command", m.Build)
	}
	bin := filepath.Join(t.TempDir(), "herdr-deck")
	args := append([]string{}, m.Build[0].Command[1:]...)
	for i, a := range args {
		if a == "-o" && i+1 < len(args) {
			args[i+1] = bin
		}
	}
	build := exec.Command("go", args...)
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("%v: %v\n%s", build.Args, err, out)
	}
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), " "+m.Version) {
		t.Errorf("--version = %q, want version %s", out, m.Version)
	}
}

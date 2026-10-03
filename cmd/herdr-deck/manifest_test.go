package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// herdr-plugin.toml's build runs scripts/build.sh, whose source build
// stamps the version with -X main.version: -X silently does nothing when
// main.version does not exist, so build the same way and check.
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
	root := filepath.Join("..", "..")
	if _, err := toml.DecodeFile(filepath.Join(root, "herdr-plugin.toml"), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Build) != 1 || !slices.Equal(m.Build[0].Command, []string{"sh", "scripts/build.sh"}) {
		t.Fatalf("build = %v, want sh scripts/build.sh", m.Build)
	}
	script, err := os.ReadFile(filepath.Join(root, "scripts", "build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), `go build -ldflags "-X main.version=$stamp"`) {
		t.Fatal("scripts/build.sh no longer builds with -X main.version=$stamp")
	}
	bin := filepath.Join(t.TempDir(), "herdr-deck")
	build := exec.Command("go", "build", "-ldflags", "-X main.version=v"+m.Version, "-o", bin, "./cmd/herdr-deck")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("%v: %v\n%s", build.Args, err, out)
	}
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if f := strings.Fields(string(out)); len(f) < 2 || f[1] != "v"+m.Version {
		t.Errorf("--version = %q, want version v%s", out, m.Version)
	}
}

package config

import (
	"flag"
	"os"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the README's settings table")

const (
	readme     = "../../README.md"
	tableStart = "<!-- settings table: generated, do not edit -->\n"
	tableEnd   = "<!-- end of settings table -->\n"
)

// settingsTable is the README's table of every setting, from Specs.
func settingsTable() string {
	code := func(s string) string { return "`" + s + "`" }
	codes := func(names ...string) string {
		var out []string
		for _, n := range names {
			if n != "" {
				out = append(out, code(n))
			}
		}
		if len(out) == 0 {
			return ""
		}
		return strings.Join(out, ", ")
	}
	var b strings.Builder
	b.WriteString("| Setting | Flag | Environment variable | Default | Earlier names |\n|---|---|---|---|---|\n")
	for _, sp := range Specs {
		def := sp.Default
		switch {
		case def == "none":
		case strings.Contains(def, DefaultDiffTool):
			def = code(DefaultDiffTool) + " with hunk installed, else " + code(FallbackDiffTool)
		case sp.Kind == KindList:
			l, _ := ParseList(def)
			def = code(tomlArray(l))
		default:
			def = code(def)
		}
		env := code(sp.Env)
		if sp.Key == KeyLinearAPIKeyCommand {
			env += " (`" + EnvLinearAPIKey + "` holds the key itself and wins)"
		}
		old := []string{sp.Old}
		old = append(append(old, sp.FlagAliases...), sp.EnvAliases...)
		b.WriteString("| " + code(sp.Key) + " | " + code(sp.Flag) + " | " + env + " | " + def + " | " + codes(old...) + " |\n")
	}
	return b.String()
}

// The README's table of settings is generated from Specs, so it never
// drifts; -update rewrites it.
func TestREADMESettingsTable(t *testing.T) {
	b, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i, j := strings.Index(s, tableStart), strings.Index(s, tableEnd)
	if i < 0 || j < i {
		t.Fatalf("README.md has no %q … %q", tableStart, tableEnd)
	}
	want := s[:i+len(tableStart)] + settingsTable() + s[j:]
	if want == s {
		return
	}
	if *update {
		if err := os.WriteFile(readme, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Error("README.md's settings table is out of date; run go test ./internal/config -run TestREADMESettingsTable -update")
}

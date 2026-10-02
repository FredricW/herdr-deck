package plugin

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/BurntSushi/toml"
)

// The manifest names what this package assumes: the plugin id, the deck
// entrypoint, the hook commands, and the version the build stamps in.
func TestManifest(t *testing.T) {
	type command struct {
		ID      string   `toml:"id"`
		On      string   `toml:"on"`
		Command []string `toml:"command"`
	}
	var m struct {
		ID      string    `toml:"id"`
		Version string    `toml:"version"`
		Build   []command `toml:"build"`
		Panes   []command `toml:"panes"`
		Actions []command `toml:"actions"`
		Events  []command `toml:"events"`
	}
	if _, err := toml.DecodeFile(filepath.Join("..", "..", "herdr-plugin.toml"), &m); err != nil {
		t.Fatal(err)
	}
	if m.ID != ID {
		t.Errorf("id = %q, want %q", m.ID, ID)
	}
	if len(m.Build) != 1 || !slices.Contains(m.Build[0].Command, "-X main.version="+m.Version) {
		t.Errorf("build %v does not stamp version %s", m.Build, m.Version)
	}
	if len(m.Panes) != 1 || m.Panes[0].ID != Entrypoint {
		t.Errorf("panes = %v, want one %q", m.Panes, Entrypoint)
	}
	if len(m.Actions) != 1 || m.Actions[0].ID != "toggle" || !slices.Equal(m.Actions[0].Command[1:], []string{"plugin", "toggle"}) {
		t.Errorf("actions = %v", m.Actions)
	}
	if len(m.Events) != 1 || m.Events[0].On != "pane.agent_detected" || !slices.Equal(m.Events[0].Command[1:], []string{"plugin", "agent-detected"}) {
		t.Errorf("events = %v", m.Events)
	}
}

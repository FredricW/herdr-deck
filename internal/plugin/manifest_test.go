package plugin

import (
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/BurntSushi/toml"
)

// The manifest names what this package assumes: the plugin id, the deck
// entrypoint, the hook commands, the build script, and link
// handlers that send Figma links to the open-link action.
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
		Links   []struct {
			ID      string `toml:"id"`
			Pattern string `toml:"pattern"`
			Action  string `toml:"action"`
		} `toml:"link_handlers"`
	}
	if _, err := toml.DecodeFile(filepath.Join("..", "..", "herdr-plugin.toml"), &m); err != nil {
		t.Fatal(err)
	}
	if m.ID != ID {
		t.Errorf("id = %q, want %q", m.ID, ID)
	}
	if m.Version == "" || len(m.Build) != 1 || !slices.Equal(m.Build[0].Command, []string{"sh", "scripts/build.sh"}) {
		t.Errorf("version %q, build %v: want a version and sh scripts/build.sh", m.Version, m.Build)
	}
	if len(m.Panes) != 1 || m.Panes[0].ID != Entrypoint {
		t.Errorf("panes = %v, want one %q", m.Panes, Entrypoint)
	}
	actions := map[string][]string{}
	for _, a := range m.Actions {
		actions[a.ID] = a.Command[1:]
	}
	if len(actions) != 2 || !slices.Equal(actions["toggle"], []string{"plugin", "toggle"}) ||
		!slices.Equal(actions["open-link"], []string{"plugin", "open-link"}) {
		t.Errorf("actions = %v", m.Actions)
	}
	if len(m.Events) != 1 || m.Events[0].On != "pane.agent_detected" || !slices.Equal(m.Events[0].Command[1:], []string{"plugin", "agent-detected"}) {
		t.Errorf("events = %v", m.Events)
	}
	if len(m.Links) != 1 || m.Links[0].Action != "open-link" {
		t.Fatalf("link_handlers = %v, want one for open-link", m.Links)
	}
	// herdr matches the whole clicked URL with a Rust regex; this one is
	// also valid Go.
	re := regexp.MustCompile(m.Links[0].Pattern)
	for url, want := range map[string]bool{
		"https://www.figma.com/design/AbC123/Admin?node-id=1-2": true,
		"https://figma.com/file/AbC123/Admin":                   true,
		"https://www.figma.com/proto/AbC123/Admin":              true,
		"https://www.figma.com/board/AbC123/Flows":              true,
		"https://www.figma.com/":                                false,
		"https://example.com/?u=https://www.figma.com/design/x": false,
	} {
		if re.MatchString(url) != want {
			t.Errorf("pattern %q on %s = %v, want %v", m.Links[0].Pattern, url, !want, want)
		}
	}
}

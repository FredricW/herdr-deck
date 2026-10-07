package main

import (
	"testing"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// The pane O opens runs this binary with exec, so q closes the pane, and
// quotes what the shell would split.
func TestImpactCommand(t *testing.T) {
	th := deck.Thread{ID: "t-0002", Worktree: "/src/worktrees/t-0002"}
	got := impactCommand("/opt/herdr deck/bin/herdr-deck", []string{"arch", "--project", "admin-rebuild", "--config", "/etc/it's.toml"}, th)
	want := `exec '/opt/herdr deck/bin/herdr-deck' arch --project admin-rebuild --config '/etc/it'\''s.toml' --thread t-0002`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	if got := impactCommand("/bin/herdr-deck", []string{"arch", "--fake"}, th); got != "exec /bin/herdr-deck arch --fake --thread t-0002" {
		t.Errorf("fake: %s", got)
	}
}

func TestRunArchNeedsAThread(t *testing.T) {
	t.Setenv("HERDR_PROJECTS_ROOT", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // not the user's config file
	if err := runArch(nil); err == nil || err.Error() != "arch: --thread is required" {
		t.Errorf("err = %v", err)
	}
	if err := runArch([]string{"--fake", "--thread", "t-0404"}); err == nil || err.Error() != "arch: no thread t-0404" {
		t.Errorf("err = %v", err)
	}
	if err := runArch([]string{"--fake", "--thread", "t-0003"}); err == nil || err.Error() != "arch: t-0003 has no worktree" {
		t.Errorf("err = %v", err)
	}
}

// The pane gets the deck's settings from its environment, not secrets.
func TestSettingsEnv(t *testing.T) {
	got := settingsEnv([]string{"HERDR_DECK_ARCH_TESTS=true", "HERDR_PROJECTS_ROOT=/p", "XDG_CONFIG_HOME=/c",
		"LINEAR_API_KEY=secret", "PATH=/bin", "HERDR_PANE_ID=w1:p1"})
	want := map[string]string{"HERDR_DECK_ARCH_TESTS": "true", "HERDR_PROJECTS_ROOT": "/p", "XDG_CONFIG_HOME": "/c"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

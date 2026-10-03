package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/FredricW/herdr-deck/internal/changelog"
	"github.com/FredricW/herdr-deck/internal/deck"
)

// testChangelog is a made-up changelog, so the goldens do not change with
// the repository's CHANGELOG.md.
func testChangelog(t *testing.T) changelog.Log {
	t.Helper()
	log, problems := changelog.Parse(`# Changelog

## [Unreleased]

### Added

- A view that is not released yet.

## [0.2.0] - 2026-09-30

### Added

- Tasks fold under their list heading, and space unfolds them again when
  the list is long.
- ABC-123 style IDs open in Linear.

### Fixed

- The drawer no longer flickers on reload.

## [0.1.0] - 2026-09-01

The first release.

### Added

- A list of tasks and threads.
`)
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	return log
}

func newsUpdate(t *testing.T) Update {
	remote, _ := changelog.Parse(`## [0.3.0] - 2026-10-09
### Added
- Threads show their changed files.
### Changed
- Reloads are faster.
## [0.2.0] - 2026-09-30
### Added
- ABC-123 style IDs open in Linear.
`)
	return Update{Available: "v0.3.0", News: changelog.Newer(remote, testChangelog(t), "v0.2.0")}
}

func TestNewsGolden(t *testing.T) {
	cases := []struct {
		name    string
		version string
		update  *Update
		keys    []tea.Msg
		updated string
	}{
		{name: "news", version: "v0.2.0", keys: keys("w")},
		{name: "news-dev", version: "v0.2.0-3-gabc1234", keys: keys("wz")},
		{name: "news-update", version: "v0.2.0", update: &Update{}, keys: keys("wz")},
		{name: "updated-note", version: "v0.2.0", updated: "0.2.0"},
	}
	for _, c := range cases {
		for _, w := range []int{80, 60} {
			name := c.name + "-" + map[int]string{80: "80", 60: "60"}[w]
			t.Run(name, func(t *testing.T) {
				m, _ := newModelWith(t, deck.Snapshot{}, w, 28, func(o *Options) {
					o.Changelog = testChangelog(t)
					o.Version = c.version
					o.Updated = c.updated
				})
				m, _ = press(m, snapshotMsg(calm()))
				if c.update != nil {
					m.update = newsUpdate(t)
				}
				m, _ = press(m, c.keys...)
				golden(t, name, m)
			})
		}
	}
}

func TestNewsKey(t *testing.T) {
	m, _ := newModelWith(t, calm(), 80, 28, func(o *Options) {
		o.Changelog = testChangelog(t)
		o.Version = "v0.1.0"
	})
	m, _ = press(m, keys("w")...)
	if m.mode != modeNews {
		t.Fatalf("w: mode %d, want What's new", m.mode)
	}
	if sc := screen(m); !strings.Contains(sc, "v0.1.0 · 2026-09-01 · ● running") {
		t.Errorf("the running release is not marked:\n%s", sc)
	}
	m, _ = press(m, keys("w")...)
	if m.mode != modeRow {
		t.Errorf("w again: mode %d, want the row", m.mode)
	}
	m, _ = press(m, keys("?z")...)
	if sc := screen(m); !strings.Contains(sc, "what's new") {
		t.Errorf("help does not list w:\n%s", sc)
	}
}

func TestNewsWithoutChangelog(t *testing.T) {
	m, _ := newModel(t, calm(), 80, 28)
	m, _ = press(m, keys("w")...)
	if sc := screen(m); !strings.Contains(sc, "This build has no changelog.") {
		t.Errorf("no changelog:\n%s", sc)
	}
}

func TestUpdatedNoteOnce(t *testing.T) {
	m, _ := newModelWith(t, calm(), 80, 28, func(o *Options) { o.Updated = "0.2.0" })
	if sc := screen(m); !strings.Contains(sc, "Updated to v0.2.0 · w what's new") {
		t.Fatalf("no note:\n%s", sc)
	}
	m, _ = press(m, keys("j")...)
	if sc := screen(m); strings.Contains(sc, "Updated to") {
		t.Errorf("the note outlived a key:\n%s", sc)
	}

	m, _ = newModelWith(t, calm(), 80, 28, func(o *Options) { o.Updated = "0.2.0" })
	m, _ = press(m, click(0, 27))
	if strings.Contains(screen(m), "Updated to") {
		t.Error("the note outlived a click")
	}
}

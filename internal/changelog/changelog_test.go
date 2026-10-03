package changelog

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	herdrdeck "github.com/FredricW/herdr-deck"
)

const sample = `# Changelog

Intro text that belongs to no release.

## [Unreleased]

### Added

- A new view.

## [0.1.0] - 2026-10-01

The first release.

### Added

- A list of tasks
  that wraps onto a second line.
- Links:
  - nested item

### Fixed

- A crash.

## [0.2.0] - 2026-10-05

### Changed

- Faster reloads.

[0.1.0]: https://example.com/releases/v0.1.0
`

func TestParse(t *testing.T) {
	log, problems := Parse(sample)
	if len(problems) > 0 {
		t.Fatalf("problems: %v", problems)
	}
	want := []Release{
		{Sections: []Section{{Name: "Added", Items: []string{"A new view."}}}},
		{Version: "0.2.0", Date: "2026-10-05", Sections: []Section{{Name: "Changed", Items: []string{"Faster reloads."}}}},
		{Version: "0.1.0", Date: "2026-10-01", Summary: "The first release.", Sections: []Section{
			{Name: "Added", Items: []string{"A list of tasks that wraps onto a second line.", "Links:", "nested item"}},
			{Name: "Fixed", Items: []string{"A crash."}},
		}},
	}
	if !reflect.DeepEqual(log.Releases, want) {
		t.Fatalf("got  %#v\nwant %#v", log.Releases, want)
	}
}

func TestParseBadSections(t *testing.T) {
	text := `## [Unreleased]
### Added
- kept
## [0.3.0]
### Added
- no date
## [1.2] - 2026-10-01
- not a version
## [0.2.0] - 2026-13-45
- bad date
## Notes
- not a release
## [0.1.0] - 2026-10-01
### Fixed
- kept too
## [0.1.0] - 2026-10-02
- twice
`
	log, problems := Parse(text)
	var titles []string
	for _, r := range log.Releases {
		titles = append(titles, r.Title())
	}
	if got := strings.Join(titles, " "); got != "Unreleased v0.1.0" {
		t.Errorf("releases = %q", got)
	}
	if r, _ := log.Find("v0.1.0"); r.Date != "2026-10-01" || r.Sections[0].Items[0] != "kept too" {
		t.Errorf("the first v0.1.0 should win: %#v", r)
	}
	wantProblems := []string{
		"line 4: v0.3.0 has no date",
		"line 7: \"1.2\" is not a version",
		"line 9: v0.2.0: \"2026-13-45\" is not a YYYY-MM-DD date",
		"line 11: \"Notes\" is not a version",
		"line 16: v0.1.0 appears twice",
	}
	if len(problems) != len(wantProblems) {
		t.Fatalf("problems = %q", problems)
	}
	for i, p := range wantProblems {
		if !strings.HasPrefix(problems[i], p) {
			t.Errorf("problem %d = %q, want it to start with %q", i, problems[i], p)
		}
	}
}

// The embedded CHANGELOG.md must parse cleanly: a bad section would vanish
// from What's new and from a release's notes.
func TestRepoChangelog(t *testing.T) {
	log, problems := Parse(herdrdeck.Changelog)
	if len(problems) > 0 {
		t.Fatalf("CHANGELOG.md: %v", problems)
	}
	if len(log.Releases) == 0 || !log.Releases[0].Unreleased() {
		t.Fatalf("CHANGELOG.md should start with an Unreleased section")
	}
	if _, ok := log.Find("0.1.0"); !ok {
		t.Errorf("CHANGELOG.md has no 0.1.0 section")
	}
}

func TestCore(t *testing.T) {
	for _, c := range []struct {
		in    string
		core  string
		exact bool
	}{
		{"v0.1.0", "0.1.0", true},
		{"0.1.0", "0.1.0", true},
		{"v0.1.0-dirty", "0.1.0", true},
		{"v0.1.0+dirty", "0.1.0", true},
		{"v0.1.0-3-gabc1234", "0.1.0", false},
		{"v0.1.0-3-gabc1234-dirty", "0.1.0", false},
		{"v0.2.0-rc.1", "0.2.0-rc.1", true},
		{"abc1234", "", false},
		{"devel", "", false},
		{"", "", false},
	} {
		core, exact := Core(c.in)
		if core != c.core || exact != c.exact {
			t.Errorf("Core(%q) = %q, %v; want %q, %v", c.in, core, exact, c.core, c.exact)
		}
	}
}

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"0.1.0", "0.1.0", 0},
		{"0.1.1", "0.1.0", 1},
		{"0.2.0", "0.10.0", -1},
		{"1.0.0", "0.9.9", 1},
		{"0.2.0-rc.1", "0.2.0", -1},
		{"v0.2.0", "0.2.0", 0},
		{"junk", "0.0.1", -1},
	} {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestRunning(t *testing.T) {
	log, _ := Parse(sample)
	for v, want := range map[string]string{
		"v0.1.0":            "v0.1.0",
		"v0.2.0-dirty":      "v0.2.0",
		"v0.2.0-4-gabc1234": "Unreleased",
		"abc1234":           "Unreleased",
		"v9.9.9":            "",
	} {
		got := ""
		if i := log.Running(v); i >= 0 {
			got = log.Releases[i].Title()
		}
		if got != want {
			t.Errorf("Running(%q) = %q, want %q", v, got, want)
		}
	}
}

func TestNewer(t *testing.T) {
	local, _ := Parse(`## [Unreleased]
### Added
- Already here.
## [0.1.0] - 2026-10-01
### Added
- First.
`)
	remote, _ := Parse(`## [Unreleased]
### Added
- Already here.
- Brand new.
### Fixed
- Already here.
## [0.2.0] - 2026-10-09
### Added
- Second.
## [0.1.1] - 2026-10-05
### Fixed
- A fix.
## [0.1.0] - 2026-10-01
### Added
- First.
`)
	titles := func(rs []Release) string {
		var s []string
		for _, r := range rs {
			s = append(s, r.Title())
		}
		return strings.Join(s, " ")
	}
	got := Newer(remote, local, "v0.1.0")
	if titles(got) != "Unreleased v0.2.0 v0.1.1" {
		t.Fatalf("Newer = %q", titles(got))
	}
	if want := []Section{{Name: "Added", Items: []string{"Brand new."}}}; !reflect.DeepEqual(got[0].Sections, want) {
		t.Errorf("Unreleased = %#v, want only what local lacks", got[0].Sections)
	}
	if got := titles(Newer(remote, local, "v0.1.1")); got != "Unreleased v0.2.0" {
		t.Errorf("from v0.1.1: %q", got)
	}
	// A bare commit compares from local's newest release.
	if got := titles(Newer(remote, local, "abc1234")); got != "Unreleased v0.2.0 v0.1.1" {
		t.Errorf("from a commit: %q", got)
	}
	if got := Newer(local, local, "v0.1.0"); got != nil {
		t.Errorf("nothing is newer than itself: %q", titles(got))
	}
}

func TestSeen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", SeenFile)
	steps := []struct {
		running, announce, recorded string
	}{
		{"v0.1.0", "", "0.1.0"},            // first run: record only
		{"v0.1.0", "", "0.1.0"},            // same version
		{"v0.1.1", "0.1.1", "0.1.1"},       // updated
		{"v0.1.1", "", "0.1.1"},            // once only
		{"abc1234", "", "0.1.1"},           // a bare commit is not recorded
		{"v0.1.1-2-gabc1234", "", "0.1.1"}, // past the same release
		{"v0.1.0", "", "0.1.0"},            // a downgrade is recorded quietly
		{"v0.2.0-dirty", "0.2.0", "0.2.0"}, // a dirty release build counts
		{"v0.2.1-3-gdef5678", "0.2.1", "0.2.1"},
	}
	for i, s := range steps {
		if got := Seen(path, s.running); got != s.announce {
			t.Errorf("step %d (%s): announced %q, want %q", i, s.running, got, s.announce)
		}
		b, _ := os.ReadFile(path)
		if got := strings.TrimSpace(string(b)); got != s.recorded {
			t.Errorf("step %d (%s): recorded %q, want %q", i, s.running, got, s.recorded)
		}
	}
}

func TestSeenUnreadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), SeenFile)
	if err := os.WriteFile(path, []byte("garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Seen(path, "v0.1.0"); got != "" {
		t.Errorf("a garbled file announced %q", got)
	}
	if got := Seen(path, "v0.2.0"); got != "0.2.0" {
		t.Errorf("after rewriting it, announced %q", got)
	}
	if got := Seen("", "v0.2.0"); got != "" {
		t.Errorf("no path announced %q", got)
	}
}

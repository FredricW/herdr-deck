// Package changelog reads CHANGELOG.md, written in the Keep a Changelog
// style: a `## [Unreleased]` section, then one `## [X.Y.Z] - YYYY-MM-DD`
// section per release, each with `### Added`, `### Changed`, `### Fixed`
// and so on holding `- ` items. The deck embeds the file and shows it as
// What's new.
package changelog

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Release is one `## ` section of the changelog.
type Release struct {
	// Version is X.Y.Z without a v; "" for Unreleased.
	Version string
	// Date is the release's YYYY-MM-DD; "" for Unreleased.
	Date string
	// Summary is the text between the heading and the first `###`.
	Summary string
	// Sections are the `###` groups in file order.
	Sections []Section
}

// Section is a `### Added`-style group of items.
type Section struct {
	Name  string
	Items []string
}

// Unreleased says r is the `## [Unreleased]` section.
func (r Release) Unreleased() bool { return r.Version == "" }

// Title is "v0.1.0", or "Unreleased".
func (r Release) Title() string {
	if r.Unreleased() {
		return "Unreleased"
	}
	return "v" + r.Version
}

// Empty says r has nothing to show.
func (r Release) Empty() bool {
	if r.Summary != "" {
		return false
	}
	for _, s := range r.Sections {
		if len(s.Items) > 0 {
			return false
		}
	}
	return true
}

// Log is a parsed changelog: Unreleased first, then releases newest first.
type Log struct {
	Releases []Release
}

var (
	heading = regexp.MustCompile(`^##\s+\[?([^\]\s]+)\]?(?:\s+-\s+(\S+))?\s*$`)
	linkRef = regexp.MustCompile(`^\[[^\]]+\]:\s`)
	bullet  = regexp.MustCompile(`^[-*+]\s+`)
)

// Parse reads a changelog. A `## ` section it cannot read (no version, no
// date, a duplicate) is left out and named in problems, one line each;
// everything else is kept.
func Parse(text string) (Log, []string) {
	var (
		log      Log
		problems []string
		cur      *Release // nil outside a release, or in a skipped one
		item     *string  // the item continuation lines join
	)
	seen := map[string]bool{}
	flush := func() {
		if cur != nil {
			log.Releases = append(log.Releases, *cur)
		}
		cur, item = nil, nil
	}
	for n, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "## "):
			flush()
			r, err := parseHeading(line)
			key := r.Version
			switch {
			case err != nil:
				problems = append(problems, fmt.Sprintf("line %d: %v", n+1, err))
			case seen[key]:
				problems = append(problems, fmt.Sprintf("line %d: %s appears twice", n+1, r.Title()))
			default:
				seen[key] = true
				cur = &r
			}
		case cur == nil, trimmed == "", linkRef.MatchString(line),
			strings.HasPrefix(trimmed, "<!--") && strings.HasSuffix(trimmed, "-->"):
			if trimmed == "" && cur != nil && len(cur.Sections) == 0 && cur.Summary != "" && !strings.HasSuffix(cur.Summary, "\n") {
				// A blank line separates summary paragraphs.
				cur.Summary += "\n"
			}
		case strings.HasPrefix(line, "### "):
			cur.Sections = append(cur.Sections, Section{Name: strings.TrimSpace(line[4:])})
			item = nil
		case len(cur.Sections) == 0:
			s := strings.TrimLeft(trimmed, "# ")
			switch {
			case cur.Summary == "", strings.HasSuffix(cur.Summary, "\n"):
				cur.Summary += s
			default:
				cur.Summary += " " + s
			}
		case line[0] == ' ' || line[0] == '\t':
			sec := &cur.Sections[len(cur.Sections)-1]
			if item != nil && !bullet.MatchString(trimmed) {
				*item += " " + trimmed
				break
			}
			// A nested item, or text before the first item: an item of
			// its own.
			sec.Items = append(sec.Items, bullet.ReplaceAllString(trimmed, ""))
			item = &sec.Items[len(sec.Items)-1]
		default:
			sec := &cur.Sections[len(cur.Sections)-1]
			sec.Items = append(sec.Items, bullet.ReplaceAllString(trimmed, ""))
			item = &sec.Items[len(sec.Items)-1]
		}
	}
	flush()
	for i := range log.Releases {
		log.Releases[i].Summary = strings.TrimSpace(log.Releases[i].Summary)
	}
	slices.SortStableFunc(log.Releases, func(a, b Release) int {
		switch {
		case a.Unreleased() && b.Unreleased():
			return 0
		case a.Unreleased():
			return -1
		case b.Unreleased():
			return 1
		}
		return Compare(b.Version, a.Version)
	})
	return log, problems
}

func parseHeading(line string) (Release, error) {
	m := heading.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return Release{}, fmt.Errorf("%q is not `## [X.Y.Z] - YYYY-MM-DD` or `## [Unreleased]`", strings.TrimSpace(line))
	}
	if strings.EqualFold(m[1], "unreleased") {
		if m[2] != "" {
			return Release{}, fmt.Errorf("Unreleased has a date")
		}
		return Release{}, nil
	}
	v := strings.TrimPrefix(m[1], "v")
	if _, ok := parse(v); !ok {
		return Release{}, fmt.Errorf("%q is not a version like 1.2.3", m[1])
	}
	if m[2] == "" {
		return Release{}, fmt.Errorf("v%s has no date: `## [%s] - YYYY-MM-DD`", v, v)
	}
	if _, err := time.Parse(time.DateOnly, m[2]); err != nil {
		return Release{}, fmt.Errorf("v%s: %q is not a YYYY-MM-DD date", v, m[2])
	}
	return Release{Version: v, Date: m[2]}, nil
}

// Find returns the release with version v (with or without a v); "" finds
// Unreleased.
func (l Log) Find(v string) (Release, bool) {
	v = strings.TrimPrefix(v, "v")
	for _, r := range l.Releases {
		if r.Version == v {
			return r, true
		}
	}
	return Release{}, false
}

// Running is the index of the release a deck running version v shows as
// its own, or -1. A release build (v0.1.0, also when dirty) is that
// release. A build past a release (a git describe such as v0.1.0-3-gabc)
// or a bare commit runs what Unreleased lists.
func (l Log) Running(v string) int {
	core, exact := Core(v)
	for i, r := range l.Releases {
		if exact && r.Version == core || !exact && r.Unreleased() {
			return i
		}
	}
	return -1
}

// describe is the end of a git describe past a tag: commits and the hash.
var describe = regexp.MustCompile(`-\d+-g[0-9a-f]+$`)

// Core reads a running version: the X.Y.Z (or X.Y.Z-pre) it is or comes
// after, and whether the build is exactly that release. A build's -dirty
// or +dirty is ignored. Core is "" for a version it cannot read, such as a
// bare commit.
func Core(v string) (core string, exact bool) {
	v = strings.TrimSuffix(strings.TrimSuffix(v, "+dirty"), "-dirty")
	exact = true
	if loc := describe.FindStringIndex(v); loc != nil {
		v, exact = v[:loc[0]], false
	}
	v = strings.TrimPrefix(v, "v")
	if _, ok := parse(v); !ok {
		return "", false
	}
	return v, exact
}

// Newer is what remote, the changelog of a newer deck, lists beyond what
// the deck running version v with changelog local has: remote's Unreleased
// and its releases newer than v, without the items local lists anywhere.
// A release left with no items is left out. For a version that cannot be read, local's newest release
// stands in. Newest first, Unreleased first; nil when there is nothing.
func Newer(remote, local Log, v string) []Release {
	base, _ := Core(v)
	if base == "" {
		for _, r := range local.Releases {
			if !r.Unreleased() {
				base = r.Version
				break
			}
		}
	}
	have := map[string]bool{}
	for _, r := range local.Releases {
		for _, s := range r.Sections {
			for _, it := range s.Items {
				have[it] = true
			}
		}
	}
	var out []Release
	for _, r := range remote.Releases {
		if !r.Unreleased() && base != "" && Compare(r.Version, base) <= 0 {
			continue
		}
		// A build past a release already has what its Unreleased lists,
		// also once a newer release lists it.
		n := Release{Version: r.Version, Date: r.Date, Summary: r.Summary}
		for _, s := range r.Sections {
			var items []string
			for _, it := range s.Items {
				if !have[it] {
					items = append(items, it)
				}
			}
			if len(items) > 0 {
				n.Sections = append(n.Sections, Section{Name: s.Name, Items: items})
			}
		}
		// A release that is only a summary has nothing to filter.
		if len(n.Sections) > 0 || len(r.Sections) == 0 && n.Summary != "" {
			out = append(out, n)
		}
	}
	return out
}

// version is a parsed X.Y.Z[-pre].
type version struct {
	n   [3]int
	pre string
}

func parse(s string) (version, bool) {
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	core, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return version{}, false
	}
	var v version
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p == "" {
			return version{}, false
		}
		v.n[i] = n
	}
	v.pre = pre
	return v, true
}

// Compare orders two versions like semver: -1, 0 or 1. A pre-release comes
// before its release. A version that cannot be read comes before any that
// can.
func Compare(a, b string) int {
	va, oka := parse(a)
	vb, okb := parse(b)
	switch {
	case !oka && !okb:
		return strings.Compare(a, b)
	case !oka:
		return -1
	case !okb:
		return 1
	}
	for i := range va.n {
		if c := va.n[i] - vb.n[i]; c != 0 {
			if c < 0 {
				return -1
			}
			return 1
		}
	}
	switch {
	case va.pre == vb.pre:
		return 0
	case va.pre == "":
		return 1
	case vb.pre == "":
		return -1
	}
	return strings.Compare(va.pre, vb.pre)
}

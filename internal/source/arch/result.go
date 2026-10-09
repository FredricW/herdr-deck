// Package arch reads what a thread's branch did to the shape of its
// repository: the package graphs of the merge-base and the branch's head,
// read straight from git objects (no checkout, nothing written), and how
// they differ. Packages are directories; edges are imports between them.
// It reads Go with go/parser and TypeScript with a small import scanner,
// caches each file's facts by blob SHA and each result by commit, and is
// what the drawer's Impact tab shows (docs/research/architecture-tab.md).
package arch

import (
	"strings"
	"time"
)

// Languages a repository is read as.
const (
	LangGo = "go"
	LangTS = "ts"
)

// Status is what the change did to a package or an edge.
type Status int

const (
	Unchanged Status = iota
	Added
	Removed
	// Changed is a package whose files changed, or an edge in both
	// commits whose importers now use something else through it.
	Changed
	// Moved is a package that only received code moved from elsewhere.
	Moved
)

// Verdict is how an internal edge stands against the declared layers.
type Verdict int

const (
	VerdictNone Verdict = iota
	VerdictSkip         // it jumps over a closed layer
	VerdictUp           // it imports a higher layer
)

// Risky says the edge goes upward or skips a layer.
func (v Verdict) Risky() bool { return v != VerdictNone }

// Site is where an import or touchpoint is written: the file and line in
// the head, or in the base for something the change removed.
type Site struct {
	File string
	Line int
	// Uses is what the file uses through an import, sorted ("Get Post").
	Uses string
	Base bool
}

// Surface is a package's exported API change: declaration keys such as
// "func Parse" or "method Reader.Read".
type Surface struct{ Added, Removed, Changed []string }

// Empty says nothing exported changed.
func (s Surface) Empty() bool { return len(s.Added)+len(s.Removed)+len(s.Changed) == 0 }

// Package is one directory with source files in the base, the head or both.
type Package struct {
	Path   string // relative to the repository root; "." is the root
	Status Status
	// Impacted is a package the change touched, or at either end of an
	// internal edge it added, removed or changed.
	Impacted bool
	Lane     int
	// Files, Added and Deleted are the package's changed source files and
	// their lines; Changed lists those files (head paths, base paths for
	// deleted ones), sorted.
	Files, Added, Deleted int
	Changed               []string
	API                   Surface
	// MovedIn counts declarations moved here unchanged, from the packages
	// in MovedFrom.
	MovedIn   int
	MovedFrom []string
	// Touches are the kinds of the touchpoints the change added here
	// (env, http, sql, exec, fs, flag), sorted, once each.
	Touches []string
	// NewDeps counts third-party packages it newly imports.
	NewDeps int
}

// Edge is a dependency between two packages, or from a package to a
// third-party ("ext:…") or standard-library ("std:…") one.
type Edge struct {
	From, To string
	Status   Status // Added, Removed, Changed or Unchanged
	Verdict  Verdict
	// Why says what the edge is: the verdict's reason, "third-party", or
	// what a standard-library import does ("network", "process", …).
	Why string
	// Risk orders the Look here first list; higher first.
	Risk int
	// Sites are the imports that make the edge (the base's for a removed
	// edge), sorted by file and line.
	Sites []Site
	// MovedWithCode marks an added or removed edge whose twin the change
	// also has, at another package its code moved to or from.
	MovedWithCode bool
	// Gained and Lost are, for a changed edge, the names its importers
	// use through it now and did not before, and the other way round.
	Gained, Lost []string
}

// Internal says the edge is between two packages of the repository.
func (e Edge) Internal() bool { return internal(e.To) }

// Risky says the edge goes upward or skips a layer, and the change did not
// remove it: a removed one is good news.
func (e Edge) Risky() bool { return e.Verdict.Risky() && e.Status != Removed }

// Touch is a touchpoint the change added (or, with Removed, took away).
type Touch struct {
	Package string
	Kind    string // env, http, sql, exec, fs, flag
	Target  string // "" when not a literal: dynamic
	Site    Site
	Removed bool
	Risk    int
}

// Dep is a go.mod or package.json requirement the change added, removed
// or bumped: From is "" when added, To "" when removed.
type Dep struct{ Module, From, To string }

// FindingKind is what a Look here first item is about.
type FindingKind int

const (
	FindEdge FindingKind = iota
	FindCycle
	FindTouch
	FindDep
	FindAPI
)

// Finding is one item of Look here first: something that changed the
// shape, with what to select and where to look.
type Finding struct {
	Kind FindingKind
	// Sign is + added, − removed, ~ changed, ↻ cycle.
	Sign string
	// Text is the finding (`internal/ui → internal/deck`, `env ABC_TOKEN`)
	// and Why the short reason (`skips api`, `third-party`, `in deck`).
	Text, Why string
	Verdict   Verdict
	Risk      int
	// Package is the box to select on the canvas; Edge an index into
	// Result.Edges, or -1.
	Package string
	Edge    int
	Site    Site
	// ID is stable across rebases: edge:a→b, touch:env:X@pkg, …
	ID string
}

// Lane is one declared or inferred layer.
type Lane struct {
	Name   string
	Closed bool
}

// Moves sums up how much of the change is a move.
type Moves struct {
	Files   int // renamed with no content change
	Decls   int // moved to another file or package unchanged
	Renamed int // moved or renamed with the same body
	Changed int // changed in place
	Added   int
	Removed int
}

// Result is what a branch did to its repository's shape.
type Result struct {
	// Note says why there is nothing to show ("no worktree", "no Go or
	// TypeScript files"); the rest is then empty.
	Note string
	// Base is the ref compared with; MergeBase and Head the commits read.
	Base, MergeBase, Head string
	// Name is the repository's: its Go module's last element, else its
	// folder's name.
	Name     string
	Language string
	// Lanes are the layers, top first; Inferred when no config declared
	// them, and then nothing can violate them.
	Lanes    []Lane
	Inferred bool
	// Configured says layers or roots came from .config/dev.json.
	Configured bool
	// Packages is every package of either side, by path.
	Packages []Package
	// Edges are the changed edges by risk (added, removed and changed),
	// then the unchanged internal edges by path.
	Edges    []Edge
	Cycles   [][]string
	Touches  []Touch
	Deps     []Dep
	Findings []Finding
	Moves    Moves
	// Parsed and Reused count files parsed for this result and taken from
	// the cache; Took is how long the read took.
	Parsed, Reused int
	Took           time.Duration
}

// Package returns the package at path p.
func (r *Result) Package(p string) (Package, bool) {
	for _, pk := range r.Packages {
		if pk.Path == p {
			return pk, true
		}
	}
	return Package{}, false
}

// ChangedEdges counts the edges the change added, removed or changed.
func (r *Result) ChangedEdges() (added, removed, changed int) {
	for _, e := range r.Edges {
		switch e.Status {
		case Added:
			added++
		case Removed:
			removed++
		case Changed:
			changed++
		}
	}
	return added, removed, changed
}

// PureMove says the change only moved code: files or declarations moved,
// and no declaration, import, touchpoint or dependency changed.
func (r *Result) PureMove() bool {
	m := r.Moves
	a, rm, _ := r.ChangedEdges()
	return m.Changed+m.Added+m.Removed == 0 && a+rm == 0 && len(r.Touches)+len(r.Deps)+len(r.Cycles) == 0 &&
		m.Files+m.Decls+m.Renamed > 0
}

// Short is a node's name in text: a package's path, or a third-party or
// standard-library package without its prefix.
func Short(n string) string {
	if s, ok := strings.CutPrefix(n, "ext:"); ok {
		return s
	}
	if s, ok := strings.CutPrefix(n, "std:"); ok {
		return s
	}
	return n
}

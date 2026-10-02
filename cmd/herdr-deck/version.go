package main

import (
	"regexp"
	"runtime/debug"
	"strings"
)

// version is set at build time with -ldflags "-X main.version=…"; the
// Makefile passes `git describe --tags --always --dirty`.
var version string

// versionString is what --version prints: "herdr-deck <version> (<commit>)".
// Without -ldflags it falls back to the module version and the VCS revision
// Go stamps into the binary.
func versionString() string {
	info, _ := debug.ReadBuildInfo()
	return formatVersion(version, info)
}

// shortVersionString is the footer's version: a tag like v0.1.0 or a short
// commit, without "-dirty".
func shortVersionString() string {
	info, _ := debug.ReadBuildInfo()
	return shortVersion(version, info)
}

// buildVersion is the version, commit and dirty flag from -ldflags and the
// build info; v is empty when neither names one.
func buildVersion(ldVersion string, info *debug.BuildInfo) (v, rev string, modified bool) {
	v = ldVersion
	if info != nil {
		if v == "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				modified = s.Value == "true"
			}
		}
	}
	return v, rev, modified
}

func formatVersion(ldVersion string, info *debug.BuildInfo) string {
	v, rev, modified := buildVersion(ldVersion, info)
	if v == "" {
		v = "(devel)"
	}
	out := "herdr-deck " + v
	if rev != "" {
		if len(rev) > 12 {
			rev = rev[:12]
		}
		if modified {
			rev += "-dirty"
		}
		out += " (" + rev + ")"
	}
	return out
}

// pseudoVersion matches the end of a Go pseudo-version: a timestamp and a
// 12-character commit.
var pseudoVersion = regexp.MustCompile(`[-.]\d{14}-([0-9a-f]{12})$`)

func shortVersion(ldVersion string, info *debug.BuildInfo) string {
	v, rev, _ := buildVersion(ldVersion, info)
	// git describe marks a dirty tree with -dirty, Go's build stamp with +dirty.
	v = strings.TrimSuffix(strings.TrimSuffix(v, "+dirty"), "-dirty")
	// A pseudo-version (v0.0.0-20261003000000-abc1234def56) says no more
	// than its commit.
	if m := pseudoVersion.FindStringSubmatch(v); m != nil {
		v = ""
		if rev == "" {
			rev = m[1]
		}
	}
	switch {
	case v != "":
		return v
	case len(rev) > 7:
		return rev[:7]
	case rev != "":
		return rev
	}
	return "devel"
}

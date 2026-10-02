package main

import (
	"runtime/debug"
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

func formatVersion(ldVersion string, info *debug.BuildInfo) string {
	v := ldVersion
	var rev string
	var modified bool
	if info != nil {
		if v == "" && info.Main.Version != "" {
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

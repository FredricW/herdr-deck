package main

import (
	"runtime/debug"
	"testing"
)

func TestFormatVersion(t *testing.T) {
	vcs := func(rev, modified string) []debug.BuildSetting {
		return []debug.BuildSetting{
			{Key: "vcs.revision", Value: rev},
			{Key: "vcs.modified", Value: modified},
		}
	}
	tests := []struct {
		name string
		ld   string
		info *debug.BuildInfo
		want string
	}{
		{"no info", "", nil, "herdr-deck (devel)"},
		{"ldflags only", "v1.2.3", nil, "herdr-deck v1.2.3"},
		{
			"ldflags wins over module version",
			"v1.2.3-4-gabc1234",
			&debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20261003000000-abc1234def56"}, Settings: vcs("abc1234def5678901234", "false")},
			"herdr-deck v1.2.3-4-gabc1234 (abc1234def56)",
		},
		{
			"go install from module zip",
			"",
			&debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}},
			"herdr-deck v1.2.3",
		},
		{
			"local dirty build",
			"",
			&debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: vcs("0123456789abcdef", "true")},
			"herdr-deck (devel) (0123456789ab-dirty)",
		},
		{
			"empty module version",
			"",
			&debug.BuildInfo{Settings: vcs("abc", "false")},
			"herdr-deck (devel) (abc)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatVersion(tt.ld, tt.info); got != tt.want {
				t.Errorf("formatVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

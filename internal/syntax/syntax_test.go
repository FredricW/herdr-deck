package syntax

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestLinesColoursKnownLanguages(t *testing.T) {
	src := []string{"package app", "", "// A says hi.", "func A() string { return \"hi\" }"}
	got := Lines("internal/app/app.go", src)
	if len(got) != len(src) {
		t.Fatalf("%d lines, want %d", len(got), len(src))
	}
	for i := range got {
		if ansi.Strip(got[i]) != src[i] {
			t.Errorf("line %d text %q, want %q", i, ansi.Strip(got[i]), src[i])
		}
	}
	if !strings.Contains(got[0], "\x1b[35mpackage") {
		t.Errorf("keyword not magenta: %q", got[0])
	}
	if !strings.Contains(got[2], "\x1b[2m") {
		t.Errorf("comment not dim: %q", got[2])
	}
	if !strings.Contains(got[3], "\x1b[33m") {
		t.Errorf("string not yellow: %q", got[3])
	}
	if ts := Lines("src/UsersPage.tsx", []string{"const n: number = 1;"}); !strings.Contains(ts[0], "\x1b[") {
		t.Errorf("tsx not coloured: %q", ts[0])
	}
}

func TestLinesFallBackToPlain(t *testing.T) {
	src := []string{"some notes", "\tindented\x1b[31m"}
	got := Lines("NOTES.unknownext", src)
	want := []string{"some notes", "    indented�[31m"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
	if Known("NOTES.unknownext") || Known("a.txt") || !Known("a.go") {
		t.Error("Known is wrong")
	}
	if got := Lines("a.go", nil); len(got) != 0 {
		t.Errorf("no lines: %q", got)
	}
}

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"a\tb":    "a   b",
		"abcd\te": "abcd    e",
		"x\ry":    "x�y",
		"plain":   "plain",
		"é\t|":    "é   |",
	} {
		if got := Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
}

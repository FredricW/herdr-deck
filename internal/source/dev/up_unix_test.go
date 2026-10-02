//go:build unix

package dev

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestStartDetached runs a short shell line, not a dev server, through the
// real starter: its output lands in the log, it leads its own process
// group, and processAlive reaps it once it exits.
func TestStartDetached(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "logs", "x-t-0001.log")
	started := time.Now()
	pid, err := startDetached(Command{Line: `echo "in $(pwd)"; sleep 1`, Dir: dir, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	if !processAlive(pid, started) {
		t.Errorf("pid %d is not alive right after start", pid)
	}
	// Recorded a day earlier, the pid is another process's.
	if processAlive(pid, started.Add(-24*time.Hour)) {
		t.Errorf("pid %d with a start a day off counts as alive", pid)
	}
	deadline := time.Now().Add(5 * time.Second)
	for processAlive(pid, started) {
		if time.Now().After(deadline) {
			t.Fatalf("pid %d still alive after 5s", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	real, _ := filepath.EvalSymlinks(dir)
	if s := string(b); !strings.Contains(s, "in "+real) || !strings.Contains(s, "# in "+dir) {
		t.Errorf("log:\n%s", s)
	}
}

func TestParseEtime(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"00:05":      5 * time.Second,
		"12:34":      12*time.Minute + 34*time.Second,
		"1:02:03":    time.Hour + 2*time.Minute + 3*time.Second,
		"2-01:00:00": 49 * time.Hour,
		"":           -1,
		"5":          -1,
		"x-01:00:00": -1,
		"1:2:3:4":    -1,
	} {
		got, ok := parseEtime(in)
		if want < 0 && ok || want >= 0 && (!ok || got != want) {
			t.Errorf("parseEtime(%q) = %v, %v; want %v", in, got, ok, want)
		}
	}
}

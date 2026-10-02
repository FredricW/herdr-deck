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
	pid, err := startDetached(Command{Line: `echo "in $(pwd)"; sleep 0.2`, Dir: dir, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	if !processAlive(pid) {
		t.Errorf("pid %d is not alive right after start", pid)
	}
	deadline := time.Now().Add(5 * time.Second)
	for processAlive(pid) {
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

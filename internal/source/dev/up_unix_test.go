//go:build unix

package dev

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestStartDetached runs a short shell line, not a dev server, through the
// real starter: its output lands in the log after the start line, it leads
// its own process group, and processAlive reaps it once it exits.
func TestStartDetached(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "logs", "key", "service.web.log")
	started := time.Now()
	pid, err := startDetached(Command{Name: "service.web", Shell: `echo "in $(pwd) as $DEV_SERVICE"; sleep 1`, Dir: dir, Env: []string{"DEV_SERVICE=web"}, Log: log})
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
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "# ") || !strings.HasSuffix(lines[0], ` herdr-deck start service.web: echo "in $(pwd) as $DEV_SERVICE"; sleep 1`) || lines[1] != "in "+real+" as web" {
		t.Errorf("log:\n%s", b)
	}
}

// TestStopGroup stops a real process group, a shell with a child sleep,
// with SIGTERM.
func TestStopGroup(t *testing.T) {
	dir := t.TempDir()
	started := time.Now()
	pid, err := startDetached(Command{Name: "service.x", Shell: "sleep 30 & wait", Dir: dir, Log: filepath.Join(dir, "x.log")})
	if err != nil {
		t.Fatal(err)
	}
	if err := signalGroup(pid, false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for processAlive(pid, started) {
		if time.Now().After(deadline) {
			_ = signalGroup(pid, true)
			t.Fatalf("pid %d still alive 5s after SIGTERM", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Signalling a group that is gone is not an error.
	if err := signalGroup(pid, false); err != nil {
		t.Errorf("signal a gone group: %v", err)
	}
}

func TestRunLogged(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "stop.log")
	if err := runLogged(context.Background(), Command{Name: "command.stop", Argv: []string{"/bin/sh", "-c", "echo bye; exit 3"}, Dir: dir, Log: log}); err == nil || !strings.Contains(err.Error(), "exit status 3") {
		t.Errorf("err %v, want exit status 3", err)
	}
	if b, _ := os.ReadFile(log); !strings.Contains(string(b), "start command.stop: /bin/sh -c echo bye; exit 3\nbye\n") {
		t.Errorf("log:\n%s", b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := runLogged(ctx, Command{Name: "command.stop", Shell: "sleep 30", Dir: dir, Log: log}); err == nil || !strings.Contains(err.Error(), "killed") {
		t.Errorf("hanging stop: %v", err)
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

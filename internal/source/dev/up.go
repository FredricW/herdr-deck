package dev

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// Command is a manifest's `up` command, ready to run.
type Command struct {
	Line string // run by /bin/sh -c
	Dir  string // the worktree
	Log  string // stdout and stderr are appended here
}

// Up runs the manifest's `up` command in the thread's worktree, detached so
// it outlives the deck, unless its dev servers already answer or a command
// the deck started there still runs. It returns the line the status bar
// shows; an error means the command could not be found or started.
func (r *Reader) Up(ctx context.Context, slug string, t deck.Thread) (string, error) {
	if t.Worktree == "" {
		return "", errors.New("no worktree on this row")
	}
	path, m, err := FindManifest(t.Worktree, t.Repo)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Sprintf(`no %s here: add one with an "up" command to start dev servers (see the README)`, ManifestPath), nil
	case err != nil:
		return "", fmt.Errorf("%s: %w", shortPath(path), err)
	case m.Up == "":
		return fmt.Sprintf(`%s has no "up" command: add one, e.g. "up": "make dev" (see the README)`, shortPath(path)), nil
	}
	if r.Logs == "" {
		return "", errors.New("no folder for logs: $HOME is not set")
	}

	r.upMu.Lock()
	defer r.upMu.Unlock()
	var ports []int
	for _, s := range t.DevServers {
		ports = append(ports, s.Port)
	}
	up := r.Prober.Listening(ctx, ports)
	var running []string
	for _, s := range t.DevServers {
		if up[s.Port] {
			running = append(running, fmt.Sprintf(":%d", s.Port))
		}
	}
	if len(running) > 0 {
		return fmt.Sprintf("%s's dev servers already answer (%s); not starting %q", t.ID, strings.Join(running, " "), m.Up), nil
	}
	log, pidFile := r.upFiles(slug, t.ID)
	if pid, ok := readPID(pidFile); ok && r.alive(pid) {
		return fmt.Sprintf("%s's up command still runs (pid %d); its log is in the drawer", t.ID, pid), nil
	}

	repo := t.Repo
	if repo == "" {
		repo = t.Worktree
	}
	line, _ := vars{worktree: t.Worktree, repo: repo, branch: t.Branch}.expand(m.Up)
	if err := os.MkdirAll(r.Logs, 0o755); err != nil {
		return "", err
	}
	start := r.Start
	if start == nil {
		start = startDetached
	}
	pid, err := start(Command{Line: line, Dir: t.Worktree, Log: log})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("started %q as pid %d, but could not record it: %w", line, pid, err)
	}
	return fmt.Sprintf("started %q for %s; its log is in the drawer", line, t.ID), nil
}

// applyUp sets DevUp on a thread whose worktree the deck ran `up` in.
func (r *Reader) applyUp(slug string, t *deck.Thread) {
	if r.Logs == "" {
		return
	}
	log, pidFile := r.upFiles(slug, t.ID)
	pid, ok := readPID(pidFile)
	if !ok {
		return
	}
	t.DevUp = &deck.DevUp{Log: log, Alive: r.alive(pid)}
}

// upFiles are the log and pid files of a thread's `up` command:
// <Logs>/<slug>-<thread>.log and .pid.
func (r *Reader) upFiles(slug, thread string) (log, pid string) {
	base := filepath.Join(r.Logs, safeName(slug)+"-"+safeName(thread))
	return base + ".log", base + ".pid"
}

func (r *Reader) alive(pid int) bool {
	if r.Alive != nil {
		return r.Alive(pid)
	}
	return processAlive(pid)
}

func readPID(path string) (int, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	return pid, err == nil && pid > 0
}

// safeName keeps a slug or thread id usable as part of a file name.
func safeName(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		}
		return '_'
	}, s)
}

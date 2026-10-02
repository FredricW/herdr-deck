//go:build unix

package dev

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"
)

// startDetached runs c in its own session (and so its own process group),
// with its output appended to c.Log, and does not wait for it: the command
// survives the deck quitting or re-executing itself.
func startDetached(c Command) (int, error) {
	if err := os.MkdirAll(filepath.Dir(c.Log), 0o755); err != nil {
		return 0, err
	}
	f, err := os.OpenFile(c.Log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	fmt.Fprintf(f, "\n# %s  %s\n# in %s\n", time.Now().Format(time.RFC3339), c.Line, c.Dir)

	cmd := exec.Command("/bin/sh", "-c", c.Line)
	cmd.Dir = c.Dir
	cmd.Stdout, cmd.Stderr = f, f
	// The deck's pane is not the dev servers' pane.
	cmd.Env = slices.DeleteFunc(os.Environ(), func(kv string) bool { return strings.HasPrefix(kv, "HERDR_PANE_ID=") })
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	// Nothing waits for it: processAlive reaps it once it exits.
	_ = cmd.Process.Release()
	return pid, nil
}

// processAlive tells whether pid still runs as the leader of its own
// process group, as startDetached leaves it; a pid reused by another
// process seldom is. A child of this process that exited is reaped here.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	var ws syscall.WaitStatus
	if got, err := syscall.Wait4(pid, &ws, syscall.WNOHANG, nil); err == nil && got == pid {
		return false
	}
	if err := syscall.Kill(pid, 0); err != nil && !errors.Is(err, syscall.EPERM) {
		return false
	}
	pgid, err := syscall.Getpgid(pid)
	return err == nil && pgid == pid
}

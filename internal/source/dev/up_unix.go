//go:build unix

package dev

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// startDetached runs c in its own session (and so its own process group),
// with stdin from /dev/null and its output appended to c.Log, and does not
// wait for it: the command survives the deck quitting or re-executing
// itself.
func startDetached(c Command) (int, error) {
	cmd, f, err := prepare(c)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	// Nothing waits for it: processAlive reaps it once it exits.
	_ = cmd.Process.Release()
	return pid, nil
}

// runLogged runs c like startDetached but waits for it to end, up to the
// context's deadline; then it kills c's process group.
func runLogged(ctx context.Context, c Command) error {
	cmd, f, err := prepare(c)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-done
		return fmt.Errorf("still running after %s: killed", stopCommandWait)
	}
}

// prepare builds c's process and opens its log, after writing the start
// line the spec asks for (section 9.2).
func prepare(c Command) (*exec.Cmd, *os.File, error) {
	if err := os.MkdirAll(filepath.Dir(c.Log), 0o755); err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(c.Log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, err
	}
	fmt.Fprintf(f, "# %s %s start %s: %s\n", time.Now().Format(time.RFC3339), Tool, c.Name, c.Text())
	var cmd *exec.Cmd
	if c.Shell != "" {
		cmd = exec.Command("/bin/sh", "-c", c.Shell)
	} else {
		cmd = exec.Command(c.Argv[0], c.Argv[1:]...)
	}
	cmd.Dir = c.Dir
	cmd.Env = c.Env
	cmd.Stdout, cmd.Stderr = f, f
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd, f, nil
}

// signalGroup sends SIGTERM to the process group pid leads, or SIGKILL
// when force is set. A group that is already gone is not an error.
func signalGroup(pid int, force bool) error {
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	if err := syscall.Kill(-pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

// pidExists tells whether a process with this pid exists.
func pidExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// processAlive tells whether pid still runs as the leader of its own
// process group, as startDetached leaves it, and (when started is known)
// began then: a pid reused by another process, e.g. after a reboot, is not
// taken for it. A child of this process that exited is reaped here.
func processAlive(pid int, started time.Time) bool {
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
	if pgid, err := syscall.Getpgid(pid); err != nil || pgid != pid {
		return false
	}
	if started.IsZero() {
		return true
	}
	// ps's etime is POSIX, unlike the start time; without ps, trust the pid.
	out, err := exec.Command("ps", "-o", "etime=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return true
	}
	elapsed, ok := parseEtime(strings.TrimSpace(string(out)))
	if !ok {
		return true
	}
	began := time.Now().Add(-elapsed)
	return began.Sub(started).Abs() <= startSlack
}

// startSlack is how far ps's idea of a process's start may be from the
// record's: the spec's 2 s, plus a second because etime has whole seconds.
const startSlack = 3 * time.Second

// parseEtime reads ps's elapsed time, [[dd-]hh:]mm:ss.
func parseEtime(s string) (time.Duration, bool) {
	var days int
	if d, rest, ok := strings.Cut(s, "-"); ok {
		n, err := strconv.Atoi(d)
		if err != nil {
			return 0, false
		}
		days, s = n, rest
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	secs := 0
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return 0, false
		}
		secs = secs*60 + n
	}
	return time.Duration(days)*24*time.Hour + time.Duration(secs)*time.Second, true
}

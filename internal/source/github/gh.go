package github

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runner runs gh with args and returns its standard output, of which it
// keeps at most the last limit bytes (zero keeps all). A failed run's
// error is a *RunError, or wraps exec.ErrNotFound when gh is missing; the
// output read so far comes with it, since `gh api` prints GraphQL errors'
// bodies too.
type Runner func(ctx context.Context, args []string, limit int) ([]byte, error)

// RunError is gh exiting with an error: its exit code and the first line
// of what it printed to standard error.
type RunError struct {
	Code   int
	Stderr string
}

func (e *RunError) Error() string {
	if e.Stderr != "" {
		return e.Stderr
	}
	return fmt.Sprintf("gh exited with status %d", e.Code)
}

// execGh runs gh without a shell, never asking anything on the terminal.
func execGh(ctx context.Context, args []string, limit int) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1", "GH_NO_UPDATE_NOTIFIER=1", "NO_COLOR=1", "CLICOLOR=0")
	out := &tailWriter{limit: limit}
	var stderr bytes.Buffer
	cmd.Stdout = out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return out.Bytes(), nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return nil, err
	}
	if ctx.Err() != nil {
		return out.Bytes(), ctx.Err()
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return out.Bytes(), &RunError{Code: ee.ExitCode(), Stderr: firstLine(stderr.String())}
	}
	return out.Bytes(), err
}

// tailWriter keeps the last limit bytes written to it (all with limit 0),
// so a log of many megabytes never sits in memory whole.
type tailWriter struct {
	limit int
	buf   []byte
	cut   bool
}

func (w *tailWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	if w.limit > 0 && len(w.buf) > 2*w.limit {
		w.buf = append(w.buf[:0], w.buf[len(w.buf)-w.limit:]...)
		w.cut = true
	}
	return len(p), nil
}

// Bytes is the tail kept; when it was cut, it starts at a whole line.
func (w *tailWriter) Bytes() []byte {
	b := w.buf
	if w.limit > 0 && len(b) > w.limit {
		b = b[len(b)-w.limit:]
		w.cut = true
	}
	if w.cut {
		if i := bytes.IndexByte(b, '\n'); i >= 0 {
			b = b[i+1:]
		}
	}
	return b
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return oneLine(s)
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}

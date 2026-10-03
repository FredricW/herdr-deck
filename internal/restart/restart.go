// Package restart notices when the deck's own binary is replaced (an update
// installed or rebuilt it) so the deck can exec the new one in place: same
// process id, so the herdr pane and its metadata stay.
//
// The binary's path is resolved once, at start. Asking os.Executable later
// is wrong on Linux: after herdr swaps the plugin folder, /proc/self/exe
// points into the deleted old folder.
package restart

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DefaultSettle is how long a new binary must stay unchanged before the
// deck runs it, so a file still being written is never executed.
const DefaultSettle = 2 * time.Second

// Watcher compares the binary at Path with the one the deck started from.
// It is not safe for concurrent use; the deck checks from one goroutine at
// a time.
type Watcher struct {
	// Path is the binary's absolute path.
	Path string
	// Settle is how long a changed file must stay the same; zero is
	// DefaultSettle.
	Settle time.Duration
	// Stat, Probe and Now are the filesystem, the --version check and the
	// clock; tests replace them.
	Stat  func(string) (os.FileInfo, error)
	Probe func(ctx context.Context, path string) error
	Now   func() time.Time

	start     os.FileInfo // the file the deck started from
	cand      os.FileInfo // a changed file waiting to settle
	candSince time.Time
}

// New records the binary at path as the running one.
func New(path string) (*Watcher, error) {
	w := &Watcher{Path: path}
	return w, w.init()
}

func (w *Watcher) init() error {
	if w.Stat == nil {
		w.Stat = os.Stat
	}
	if w.Probe == nil {
		w.Probe = Probe
	}
	if w.Now == nil {
		w.Now = time.Now
	}
	if w.Settle == 0 {
		w.Settle = DefaultSettle
	}
	fi, err := w.Stat(w.Path)
	if err != nil {
		return err
	}
	w.start = fi
	return nil
}

// same says whether a and b are the same file with the same contents, as
// far as inode, size and modification time tell.
func same(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

// Changed says whether a new binary is at Path and ready to run: it differs
// from the one the deck started from, has not changed for Settle, and
// answers --version. A missing file, or one that fails the probe, is not
// ready; the next call looks again.
func (w *Watcher) Changed(ctx context.Context) bool {
	fi, err := w.Stat(w.Path)
	if err != nil || same(fi, w.start) {
		w.cand = nil
		return false
	}
	now := w.Now()
	if w.cand == nil || !same(fi, w.cand) {
		w.cand, w.candSince = fi, now
		return false
	}
	if now.Sub(w.candSince) < w.Settle {
		return false
	}
	return w.Probe(ctx, w.Path) == nil
}

// Probe runs `path --version` and checks it names herdr-deck.
func Probe(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(out), "herdr-deck ") {
		return errors.New("--version does not name herdr-deck")
	}
	return nil
}

// Executable is the running binary's absolute path, to be called once at
// start. Inside a herdr plugin it prefers $HERDR_PLUGIN_ROOT/bin/herdr-deck
// when that is the same file: it is the path an update replaces.
func Executable(getenv func(string) string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if exe, err = filepath.Abs(exe); err != nil {
		return "", err
	}
	if root := getenv("HERDR_PLUGIN_ROOT"); root != "" {
		p := filepath.Join(root, "bin", "herdr-deck")
		a, aerr := os.Stat(p)
		b, berr := os.Stat(exe)
		if aerr == nil && berr == nil && os.SameFile(a, b) {
			return p, nil
		}
	}
	return exe, nil
}

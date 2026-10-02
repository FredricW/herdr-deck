//go:build unix

package plugin

import (
	"os"
	"path/filepath"
	"syscall"
)

// withLock runs fn while holding an exclusive lock on a file in dir, so two
// hooks that fire together (a session restore starts many agents) do not
// both open a deck in one workspace. Without dir, or when the lock cannot
// be taken, fn runs anyway.
func withLock(dir string, fn func() error) error {
	if dir == "" {
		return fn()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fn()
	}
	f, err := os.OpenFile(filepath.Join(dir, "open.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fn()
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fn()
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}

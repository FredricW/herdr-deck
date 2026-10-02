package restart

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) add(d time.Duration)     { c.t = c.t.Add(d) }
func write(t *testing.T, p, body string) { writeAt(t, p, body, time.Now()) }

func writeAt(t *testing.T, p, body string, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

// replace puts a new file at p through a rename, as an install or build does.
func replace(t *testing.T, p, body string, mtime time.Time) {
	t.Helper()
	tmp := p + ".tmp"
	writeAt(t, tmp, body, mtime)
	if err := os.Rename(tmp, p); err != nil {
		t.Fatal(err)
	}
}

func newWatcher(t *testing.T, probe func(context.Context, string) error) (*Watcher, *clock, string) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "herdr-deck")
	write(t, bin, "old")
	c := &clock{t: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)}
	if probe == nil {
		probe = func(context.Context, string) error { return nil }
	}
	w := &Watcher{Path: bin, Now: c.now, Probe: probe}
	if err := w.init(); err != nil {
		t.Fatal(err)
	}
	return w, c, bin
}

func TestUnchangedBinary(t *testing.T) {
	w, c, _ := newWatcher(t, nil)
	for range 3 {
		c.add(5 * time.Second)
		if w.Changed(context.Background()) {
			t.Fatal("Changed with the same binary")
		}
	}
}

func TestReplacedBinarySettlesFirst(t *testing.T) {
	w, c, bin := newWatcher(t, nil)
	ctx := context.Background()
	replace(t, bin, "new", time.Now().Add(time.Second))
	if w.Changed(ctx) {
		t.Fatal("Changed on first sight; want it to wait for the file to settle")
	}
	c.add(time.Second)
	if w.Changed(ctx) {
		t.Fatal("Changed before Settle passed")
	}
	c.add(2 * time.Second)
	if !w.Changed(ctx) {
		t.Fatal("not Changed after the new file settled")
	}
}

func TestFileStillBeingWrittenWaits(t *testing.T) {
	w, c, bin := newWatcher(t, nil)
	ctx := context.Background()
	base := time.Now()
	replace(t, bin, "ne", base.Add(time.Second))
	w.Changed(ctx)
	// The file grows in place between checks: the wait starts again.
	c.add(5 * time.Second)
	writeAt(t, bin, "new", base.Add(2*time.Second))
	if w.Changed(ctx) {
		t.Fatal("Changed while the file was still changing")
	}
	c.add(5 * time.Second)
	if !w.Changed(ctx) {
		t.Fatal("not Changed once the file stopped changing")
	}
}

func TestInPlaceOverwriteCounts(t *testing.T) {
	w, c, bin := newWatcher(t, nil)
	ctx := context.Background()
	// Same inode, new contents.
	writeAt(t, bin, "newer", time.Now().Add(time.Minute))
	w.Changed(ctx)
	c.add(5 * time.Second)
	if !w.Changed(ctx) {
		t.Fatal("an in-place overwrite was not seen")
	}
}

func TestProbeFailureAndMissingFile(t *testing.T) {
	probeErr := errors.New("exec format error")
	w, c, bin := newWatcher(t, func(context.Context, string) error { return probeErr })
	ctx := context.Background()
	replace(t, bin, "broken", time.Now().Add(time.Second))
	w.Changed(ctx)
	c.add(5 * time.Second)
	if w.Changed(ctx) {
		t.Fatal("Changed although --version failed")
	}
	probeErr = nil
	c.add(5 * time.Second)
	if !w.Changed(ctx) {
		t.Fatal("not Changed once --version works")
	}

	if err := os.Remove(bin); err != nil {
		t.Fatal(err)
	}
	c.add(5 * time.Second)
	if w.Changed(ctx) {
		t.Fatal("Changed with no binary at the path")
	}
}

func TestExecutableIsAbsolute(t *testing.T) {
	p, err := Executable(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(p) {
		t.Errorf("Executable = %q, want an absolute path", p)
	}
}

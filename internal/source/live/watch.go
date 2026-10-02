package live

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// watchedDirs are the folders under the project folder whose files the deck
// shows. fsnotify does not recurse, so each is watched on its own.
var watchedDirs = []string{".", "threads", "inbox", ".state"}

// Watch calls changed once after files in the project folder dir change,
// waiting until debounce has passed without further changes, so a burst of
// writes causes one reload. It only reads: watching never writes a file. It
// returns when ctx is done, or at once with an error when no watch could be
// set up; folders that appear later (inbox/ in a new project) are added when
// they do.
func Watch(ctx context.Context, dir string, debounce time.Duration, changed func()) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := w.Add(dir); err != nil {
		w.Close()
		return err
	}
	for _, sub := range watchedDirs[1:] {
		_ = w.Add(filepath.Join(dir, sub)) // may not exist yet
	}

	go func() {
		defer w.Close()
		timer := time.NewTimer(debounce)
		timer.Stop()
		for {
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if ev.Has(fsnotify.Create) && filepath.Dir(ev.Name) == filepath.Clean(dir) && watched(filepath.Base(ev.Name)) {
					if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
						_ = w.Add(ev.Name)
					}
				}
				timer.Reset(debounce)
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			case <-timer.C:
				changed()
			}
		}
	}()
	return nil
}

func watched(name string) bool {
	for _, d := range watchedDirs[1:] {
		if d == name {
			return true
		}
	}
	return false
}

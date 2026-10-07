package manifest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Paths are where a manifest lives in a checkout: the shared one, and the
// deck's legacy one.
const (
	Path       = ".config/dev.json"
	LegacyPath = ".herdr-deck/dev.json"
)

// Found is the manifest a lookup found.
type Found struct {
	// Dir is the checkout holding it (the worktree or the main checkout)
	// and Rel its path there, Path or LegacyPath.
	Dir, Rel string
	Legacy   bool
	Manifest *Manifest
}

// File is the manifest's full path.
func (f Found) File() string { return filepath.Join(f.Dir, f.Rel) }

// Short names the manifest by its checkout's folder:
// "webshop/.config/dev.json".
func (f Found) Short() string { return filepath.Join(filepath.Base(f.Dir), f.Rel) }

// Find looks up a worktree's manifest (spec section 2.2): the worktree's
// .config/dev.json, the main checkout's, then the legacy .herdr-deck/dev.json
// in the same order. The first file that exists is the manifest: when it
// cannot be read or is broken, Find returns its error and does not fall
// back to a later file. Without any file the error wraps fs.ErrNotExist.
// repo may be "" or the worktree itself.
func Find(worktree, repo string) (Found, error) {
	dirs := []string{worktree}
	if repo != "" && repo != worktree {
		dirs = append(dirs, repo)
	}
	for _, rel := range []string{Path, LegacyPath} {
		for _, dir := range dirs {
			f := Found{Dir: dir, Rel: rel, Legacy: rel == LegacyPath}
			b, err := os.ReadFile(f.File())
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return f, err
			}
			if f.Legacy {
				f.Manifest, err = ParseLegacy(b)
			} else {
				f.Manifest, err = Parse(b)
			}
			return f, err
		}
	}
	return Found{}, fs.ErrNotExist
}

package changelog

import (
	"os"
	"path/filepath"
	"strings"
)

// SeenFile is the state file that records the version the deck last ran,
// in the deck's state folder.
const SeenFile = "last-version"

// Seen records that the deck runs version v, in the file at path, and
// returns the version to announce: v's X.Y.Z when it is newer than the
// version recorded before, else "". The first run records v and announces
// nothing, and so does a version that cannot be read, such as a bare
// commit (it is not recorded). A file that cannot be read or written only
// costs the note.
func Seen(path, v string) string {
	core, _ := Core(v)
	if path == "" || core == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	last := strings.TrimSpace(string(b))
	if err == nil && last == core {
		return ""
	}
	writeSeen(path, core)
	if _, ok := parse(last); err != nil || !ok || Compare(core, last) <= 0 {
		return ""
	}
	return core
}

// writeSeen replaces the file through a rename, so another deck never
// reads half of it.
func writeSeen(path, core string) {
	dir := filepath.Dir(path)
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, ".last-version-*")
	if err != nil {
		return
	}
	_, werr := tmp.WriteString(core + "\n")
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), path) != nil {
		os.Remove(tmp.Name())
	}
}

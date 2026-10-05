package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DrawerFile is the state file that keeps the drawer's dragged height, a
// share of the pane such as 0.42, in the deck's state folder (StateDir).
// It is state, not a setting: ui.drawer_height is the default until the
// first drag, and dragging never writes the config file.
const DrawerFile = "drawer-height"

// A dragged height may go beyond ui.drawer_height's range, since the
// deck's own limits are in rows; it only has to be a share of the pane.

// ReadDrawerHeight returns the height kept in the file at path, and false
// when there is none or it does not hold a share between 0 and 1.
func ReadDrawerHeight(path string) (float64, bool) {
	if path == "" {
		return 0, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	return v, err == nil && v > 0 && v < 1
}

// WriteDrawerHeight keeps v in the file at path, creating its folder. It
// replaces the file through a rename, so another deck never reads half of
// it.
func WriteDrawerHeight(path string, v float64) error {
	if !(v > 0 && v < 1) {
		return fmt.Errorf("drawer height %v is not a share between 0 and 1", v)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+DrawerFile+"-*")
	if err != nil {
		return err
	}
	_, werr := tmp.WriteString(strconv.FormatFloat(v, 'f', 4, 64) + "\n")
	cerr := tmp.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Rename(tmp.Name(), path)
	}
	if werr != nil {
		os.Remove(tmp.Name())
	}
	return werr
}

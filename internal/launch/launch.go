// Package launch opens URLs and folders in the user's desktop apps.
package launch

import (
	"os/exec"
	"runtime"
)

// URL opens url with the system's handler: `open` on macOS, else `xdg-open`.
// It returns once the handler has started, without waiting for it.
func URL(url string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	return start(exec.Command(name, url))
}

// Editor opens the folder path in VS Code (`code <path>`). A terminal editor
// from $VISUAL would fight the deck for its pane, so it is not used.
func Editor(path string) error {
	return start(exec.Command("code", path))
}

func start(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // reap the child; its exit status does not matter
	return nil
}

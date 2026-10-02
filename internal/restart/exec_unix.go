//go:build unix

package restart

import (
	"os"
	"syscall"
)

// Supported says whether this platform can restart in place.
const Supported = true

// Exec replaces the process with the binary at path, keeping the arguments,
// environment and process id. It returns only on failure.
func Exec(path string) error {
	return syscall.Exec(path, os.Args, os.Environ())
}

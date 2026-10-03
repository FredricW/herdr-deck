//go:build !unix

package restart

import "errors"

// Supported says whether this platform can restart in place.
const Supported = false

// Exec is not available here: the deck runs on macOS and Linux only.
func Exec(string) error {
	return errors.New("restarting in place is not supported on this platform")
}

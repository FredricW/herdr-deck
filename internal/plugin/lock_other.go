//go:build !unix

package plugin

// withLock runs fn; the plugin runs on macOS and Linux only.
func withLock(_ string, fn func() error) error { return fn() }

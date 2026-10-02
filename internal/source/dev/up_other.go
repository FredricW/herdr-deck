//go:build !unix

package dev

import "errors"

func startDetached(Command) (int, error) {
	return 0, errors.New("starting dev servers needs a Unix system")
}

func processAlive(int) bool { return false }

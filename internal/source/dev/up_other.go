//go:build !unix

package dev

import (
	"context"
	"errors"
	"time"
)

var errUnix = errors.New("starting dev servers needs a Unix system")

func startDetached(Command) (int, error)       { return 0, errUnix }
func runLogged(context.Context, Command) error { return errUnix }
func signalGroup(int, bool) error              { return errUnix }
func pidExists(int) bool                       { return false }
func processAlive(int, time.Time) bool         { return false }

package dev

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Tool is the name the deck writes into run records, locks and logs.
const Tool = "herdr-deck"

// Store is the shared state folder every tool that reads dev manifests
// uses (spec section 10): $XDG_STATE_HOME/dev-manifest, with a lock, run
// records under runs/<key>/ and logs under logs/<key>/. The deck does not
// read or write the port store (ports.json) yet.
type Store struct {
	Dir string
	// Alive tells whether a record's process still runs; nil asks the
	// system (processAlive).
	Alive func(pid int, started time.Time) bool
}

// Key names a worktree in the store: its folder name (characters outside
// A-Za-z0-9._- made _), a dash, and the first 8 hex digits of the SHA-256
// of its absolute path.
func Key(worktree string) string {
	sum := sha256.Sum256([]byte(worktree))
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			return r
		}
		return '_'
	}, filepath.Base(worktree))
	return name + "-" + hex.EncodeToString(sum[:4])
}

// Log is the default log of what ran under name (service.api, dev.default,
// command.stop) in a worktree.
func (s Store) Log(key, name string) string {
	return filepath.Join(s.Dir, "logs", key, name+".log")
}

func (s Store) recordPath(key, name string) string {
	return filepath.Join(s.Dir, "runs", key, name+".json")
}

// Record says a tool started a long-running background process (spec
// section 10.3).
type Record struct {
	Version  int    `json:"version"`
	Worktree string `json:"worktree"`
	Name     string `json:"name"`
	PID      int    `json:"pid"`
	Started  string `json:"started"`
	Command  string `json:"command"`
	Dir      string `json:"dir"`
	Log      string `json:"log"`
	Tool     string `json:"tool"`
}

// StartedAt is the record's start time, zero when it has none.
func (r Record) StartedAt() time.Time {
	t, _ := time.Parse(time.RFC3339, r.Started)
	return t
}

// Record reads the run record of name in a worktree.
func (s Store) Record(key, name string) (Record, bool) {
	b, err := os.ReadFile(s.recordPath(key, name))
	if err != nil {
		return Record{}, false
	}
	var r Record
	if json.Unmarshal(b, &r) != nil || r.PID <= 0 {
		return Record{}, false
	}
	if r.Name == "" {
		r.Name = name
	}
	return r, true
}

// Records reads every run record of a worktree, in file name order.
func (s Store) Records(key string) []Record {
	entries, err := os.ReadDir(filepath.Join(s.Dir, "runs", key))
	if err != nil {
		return nil
	}
	var out []Record
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || e.IsDir() {
			continue
		}
		if r, ok := s.Record(key, name); ok {
			out = append(out, r)
		}
	}
	return out
}

// IsAlive tells whether a record's process still runs: the pid exists,
// leads its own process group and started when the record says.
func (s Store) IsAlive(r Record) bool {
	if s.Alive != nil {
		return s.Alive(r.PID, r.StartedAt())
	}
	return processAlive(r.PID, r.StartedAt())
}

// writeRecord writes a run record atomically; hold the lock.
func (s Store) writeRecord(key string, r Record) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(s.recordPath(key, r.Name), append(b, '\n'))
}

// removeRecord removes a run record; hold the lock.
func (s Store) removeRecord(key, name string) error {
	err := os.Remove(s.recordPath(key, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func writeAtomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		os.Remove(f.Name())
		return err
	}
	return nil
}

// Lock timing (spec section 10.1).
var (
	lockRetry = 50 * time.Millisecond
	lockWait  = 5 * time.Second
	lockStale = 30 * time.Second
)

type lockFile struct {
	PID   int    `json:"pid"`
	Host  string `json:"host"`
	Tool  string `json:"tool"`
	Time  string `json:"time"`
	Token string `json:"token"`
}

// Lock takes the store's lock: it creates the lock file exclusively, and
// takes a stale one away atomically (spec section 10.1). The returned func
// releases it. Hold it only for a moment, never while a service starts up.
func (s Store) Lock() (func(), error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(s.Dir, "lock")
	host, _ := os.Hostname()
	token := randomHex(8)
	body, err := json.Marshal(lockFile{PID: os.Getpid(), Host: host, Tool: Tool, Time: time.Now().UTC().Format(time.RFC3339), Token: token})
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(lockWait)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, werr := f.Write(body)
			cerr := f.Close()
			if werr != nil || cerr != nil {
				os.Remove(path)
				return nil, errors.Join(werr, cerr)
			}
			return func() {
				// Release only our own lock.
				if b, err := os.ReadFile(path); err == nil && bytes.Equal(b, body) {
					os.Remove(path)
				}
			}, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		if s.breakStale(path, host) {
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("the dev-manifest lock %s is held by another tool", path)
		}
		time.Sleep(lockRetry)
	}
}

// breakStale removes a stale lock and reports whether it did: one older
// than lockStale, or one of this host whose pid is gone. It renames the
// lock first, so of two tools that both judge it stale only one removes
// it, and puts back a live lock that replaced it in between.
func (s Store) breakStale(path, host string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return errors.Is(err, fs.ErrNotExist) // gone: retry at once
	}
	var l lockFile
	stale := false
	fi, statErr := os.Stat(path)
	if json.Unmarshal(content, &l) != nil {
		// Half written, or not ours to read: only its age tells.
		stale = statErr == nil && time.Since(fi.ModTime()) > lockStale
	} else {
		t, terr := time.Parse(time.RFC3339, l.Time)
		stale = (terr == nil && time.Since(t) > lockStale) ||
			(terr != nil && statErr == nil && time.Since(fi.ModTime()) > lockStale) ||
			(l.Host == host && l.PID > 0 && !pidExists(l.PID))
	}
	if !stale {
		return false
	}
	aside := fmt.Sprintf("%s.%d.%s", path, os.Getpid(), randomHex(4))
	if err := os.Rename(path, aside); err != nil {
		return false
	}
	got, err := os.ReadFile(aside)
	if err == nil && bytes.Equal(got, content) {
		os.Remove(aside)
		return true
	}
	// A live lock replaced the stale one: put it back, unless yet another
	// lock was taken meanwhile.
	_ = os.Link(aside, path)
	os.Remove(aside)
	return true
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

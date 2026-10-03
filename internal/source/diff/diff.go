// Package diff reads what a thread's worktree changed against its base
// branch: every file of `git diff <merge-base>` with its line counts,
// uncommitted changes and untracked files included.
package diff

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// Timeout bounds the git commands of one Read together.
const Timeout = 5 * time.Second

// cacheTTL is how long an answer is reused. The deck asks on every reload
// and whenever the selection moves; a worktree rarely changes faster.
const cacheTTL = 2 * time.Second

// maxUntracked is how many untracked files get their lines counted; the
// rest are listed without counts.
const maxUntracked = 200

// DefaultBase is the base a thread without one is compared with: the
// remote's default branch.
const DefaultBase = "origin/HEAD"

// Reader reads worktree diffs, remembering each answer for a short while.
type Reader struct {
	// Git runs git with args in dir and returns its standard output; nil
	// runs the real git. Tests replace it.
	Git func(ctx context.Context, dir string, args ...string) ([]byte, error)
	// Now is the clock for the cache; nil means time.Now.
	Now func() time.Time

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	diff deck.Diff
	at   time.Time
}

// Read returns what t's worktree changed against t.Base (DefaultBase when
// it has none). It never fails: a missing worktree or base is a Note.
func (r *Reader) Read(ctx context.Context, t deck.Thread) deck.Diff {
	base := t.Base
	if base == "" {
		base = DefaultBase
	}
	if t.Worktree == "" {
		return deck.Diff{Base: base, Note: "no worktree"}
	}
	key := t.Worktree + "\x00" + base
	now := r.now()
	r.mu.Lock()
	c, ok := r.cache[key]
	r.mu.Unlock()
	if ok && now.Sub(c.at) < cacheTTL {
		return c.diff
	}
	d := r.read(ctx, t.Worktree, base)
	r.mu.Lock()
	if r.cache == nil {
		r.cache = map[string]cached{}
	}
	r.cache[key] = cached{diff: d, at: now}
	r.mu.Unlock()
	return d
}

func (r *Reader) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Reader) read(ctx context.Context, dir, base string) deck.Diff {
	d := deck.Diff{Base: base}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		d.Note = "the worktree is gone"
		return d
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	git := r.Git
	if git == nil {
		git = execGit
	}
	out, err := git(ctx, dir, "merge-base", base, "HEAD")
	if err != nil {
		d.Note = fmt.Sprintf("cannot compare with %s: %v", base, err)
		return d
	}
	d.MergeBase = strings.TrimSpace(string(out))
	// Against a commit, git diff compares the working tree: committed,
	// staged and unstaged changes in one.
	out, err = git(ctx, dir, "diff", "--numstat", "-z", "-M", d.MergeBase, "--")
	if err != nil {
		d.Note = fmt.Sprintf("git diff: %v", err)
		return d
	}
	d.Files = ParseNumstat(out)
	out, err = git(ctx, dir, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		d.Note = fmt.Sprintf("git ls-files: %v", err)
		return d
	}
	for i, p := range splitZ(out) {
		f := deck.DiffFile{Path: p, Untracked: true}
		if i < maxUntracked {
			f.Added, f.Binary = countLines(filepath.Join(dir, p))
		}
		d.Files = append(d.Files, f)
	}
	sort.SliceStable(d.Files, func(i, j int) bool { return d.Files[i].Path < d.Files[j].Path })
	return d
}

// ParseNumstat reads `git diff --numstat -z`: "added\tdeleted\tpath\0"
// per file, "-\t-\t" for a binary file, and for a rename an empty path
// followed by "old\0new\0".
func ParseNumstat(out []byte) []deck.DiffFile {
	parts := splitZ(out)
	var files []deck.DiffFile
	for i := 0; i < len(parts); i++ {
		fields := strings.SplitN(parts[i], "\t", 3)
		if len(fields) < 3 {
			continue
		}
		f := deck.DiffFile{Path: fields[2]}
		if f.Path == "" {
			if i+2 >= len(parts) {
				break
			}
			f.OldPath, f.Path = parts[i+1], parts[i+2]
			i += 2
		}
		if fields[0] == "-" && fields[1] == "-" {
			f.Binary = true
		} else {
			f.Added, _ = strconv.Atoi(fields[0])
			f.Deleted, _ = strconv.Atoi(fields[1])
		}
		files = append(files, f)
	}
	return files
}

func splitZ(out []byte) []string {
	var parts []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// countLines counts a new file's lines as git would add them; a file with
// a NUL byte in its first 8000 bytes is binary, as git decides.
func countLines(path string) (lines int, binary bool) {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return 0, false
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	br := bufio.NewReaderSize(f, 32<<10)
	if head, _ := br.Peek(8000); bytes.IndexByte(head, 0) >= 0 {
		return 0, true
	}
	buf := make([]byte, 32<<10)
	var last byte
	for {
		n, err := br.Read(buf)
		if n > 0 {
			lines += bytes.Count(buf[:n], []byte{'\n'})
			last = buf[n-1]
		}
		if err != nil {
			break
		}
	}
	if fi.Size() > 0 && last != '\n' {
		lines++
	}
	return lines, false
}

// execGit runs git in dir without taking optional locks, so reading never
// gets in the way of the agent working in the worktree. A failure says
// git's first line of stderr.
func execGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir, "--no-pager"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if line := firstLine(stderr.String()); line != "" {
				return nil, errors.New(line)
			}
		}
		return nil, err
	}
	return out, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimPrefix(s, "fatal: ")
}

package diff

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// MaxPatchLines is how many lines of one file's diff the preview gets; the
// rest are counted in Patch.More.
const MaxPatchLines = 2000

// maxPatchBytes bounds what is read of one file's diff or untracked file,
// so a generated file of many megabytes never fills the memory.
const maxPatchBytes = 4 << 20

// maxLineBytes bounds one line's text: the preview cuts lines at the
// pane's width anyway, and a minified line could be megabytes.
const maxLineBytes = 1024

// patchCacheSize is how many file diffs are remembered.
const patchCacheSize = 64

// ReadPatch returns f's diff in t's worktree against mergeBase: `git diff
// <merge-base> -- <file>` (a rename's old and new path, so git pairs
// them), or an untracked file's lines all added. The answer is cached by
// the file and a hash of its content, so asking again for a file that did
// not change costs a read of the file but no git. It never fails: a
// problem is the Patch's Note.
func (r *Reader) ReadPatch(ctx context.Context, t deck.Thread, mergeBase string, f deck.DiffFile) deck.Patch {
	if t.Worktree == "" {
		return deck.Patch{Note: "no worktree"}
	}
	if mergeBase == "" && !f.Untracked {
		return deck.Patch{Note: "no merge-base to compare with"}
	}
	key := strings.Join([]string{t.Worktree, mergeBase, f.OldPath, f.Path, fmt.Sprint(f.Untracked), contentHash(filepath.Join(t.Worktree, f.Path))}, "\x00")
	r.mu.Lock()
	p, ok := r.patches[key]
	r.mu.Unlock()
	if ok {
		return p
	}
	if f.Untracked {
		p = untrackedPatch(filepath.Join(t.Worktree, f.Path))
	} else {
		p = r.gitPatch(ctx, t.Worktree, mergeBase, f)
	}
	if strings.HasPrefix(p.Note, "git diff:") {
		return p // a timeout or a lock may pass: ask again next time
	}
	r.mu.Lock()
	if len(r.patches) >= patchCacheSize {
		clear(r.patches)
	}
	if r.patches == nil {
		r.patches = map[string]deck.Patch{}
	}
	r.patches[key] = p
	r.mu.Unlock()
	return p
}

func (r *Reader) gitPatch(ctx context.Context, dir, mergeBase string, f deck.DiffFile) deck.Patch {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	args := []string{"diff", "--no-color", "--no-ext-diff", "--no-textconv", "-M", mergeBase, "--"}
	if f.OldPath != "" {
		args = append(args, f.OldPath)
	}
	args = append(args, f.Path)
	var (
		out []byte
		cut bool
		err error
	)
	if r.Git != nil {
		out, err = r.Git(ctx, dir, args...)
	} else {
		out, cut, err = execGitLimit(ctx, dir, maxPatchBytes, args...)
	}
	if err != nil {
		return deck.Patch{Note: fmt.Sprintf("git diff: %v", err)}
	}
	p := ParsePatch(out, MaxPatchLines)
	p.Cut = cut
	return p
}

// ParsePatch reads the unified diff of one file (git may print a rename it
// cannot pair as two), keeping at most limit lines and counting the rest
// in More. Headers before the first hunk are left out; "Binary files …
// differ" marks the patch binary.
func ParsePatch(out []byte, limit int) deck.Patch {
	var p deck.Patch
	inHunk, renamed := false, false
	s := string(out)
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return deck.Patch{Note: "no changes"}
	}
	for line := range strings.SplitSeq(s, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			inHunk = false
			continue
		}
		if !inHunk && !strings.HasPrefix(line, "@@") {
			switch {
			case strings.HasPrefix(line, "Binary files "), line == "GIT binary patch":
				p.Binary = true
			case strings.HasPrefix(line, "rename from "):
				renamed = true
			}
			continue
		}
		var pl deck.PatchLine
		switch {
		case strings.HasPrefix(line, "@@"):
			inHunk = true
			pl = deck.PatchLine{Kind: deck.LineHunk, Text: line}
		case line == "":
			pl = deck.PatchLine{Kind: deck.LineContext}
		default:
			text := line[1:]
			switch line[0] {
			case '+':
				pl = deck.PatchLine{Kind: deck.LineAdded, Text: text}
			case '-':
				pl = deck.PatchLine{Kind: deck.LineDeleted, Text: text}
			case '\\':
				pl = deck.PatchLine{Kind: deck.LineNote, Text: strings.TrimSpace(text)}
			default:
				pl = deck.PatchLine{Kind: deck.LineContext, Text: text}
			}
		}
		if len(p.Lines) >= limit {
			p.More++
			continue
		}
		pl.Text = cutLine(pl.Text)
		p.Lines = append(p.Lines, pl)
	}
	switch {
	case p.Binary:
		p.Lines, p.More = nil, 0
	case len(p.Lines) == 0 && renamed:
		p.Note = "renamed, content unchanged"
	case len(p.Lines) == 0:
		p.Note = "no changes in content"
	}
	return p
}

// untrackedPatch is a new file that git does not know yet: every line
// added, under a hunk header as git would print once it is added.
func untrackedPatch(path string) deck.Patch {
	fi, err := os.Lstat(path)
	switch {
	case err != nil:
		return deck.Patch{Note: "cannot read the file: " + errText(err)}
	case fi.Mode()&os.ModeSymlink != 0:
		target, _ := os.Readlink(path)
		return deck.Patch{Lines: []deck.PatchLine{{Kind: deck.LineHunk, Text: "@@ -0,0 +1 @@"}, {Kind: deck.LineAdded, Text: target}}}
	case !fi.Mode().IsRegular():
		return deck.Patch{Note: "not a regular file"}
	}
	fh, err := os.Open(path)
	if err != nil {
		return deck.Patch{Note: "cannot read the file: " + errText(err)}
	}
	defer fh.Close()
	data, err := io.ReadAll(io.LimitReader(fh, maxPatchBytes+1))
	if err != nil {
		return deck.Patch{Note: "cannot read the file: " + errText(err)}
	}
	if bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
		return deck.Patch{Binary: true}
	}
	var p deck.Patch
	if len(data) > maxPatchBytes {
		data, p.Cut = data[:maxPatchBytes], true
	}
	s := strings.TrimSuffix(string(data), "\n")
	if s == "" {
		return deck.Patch{Note: "empty file"}
	}
	lines := strings.Split(s, "\n")
	total := len(lines)
	if !p.Cut {
		p.Lines = append(p.Lines, deck.PatchLine{Kind: deck.LineHunk, Text: fmt.Sprintf("@@ -0,0 +1,%d @@", total)})
	} else {
		p.Lines = append(p.Lines, deck.PatchLine{Kind: deck.LineHunk, Text: "@@ -0,0 +1 @@"})
	}
	for i, l := range lines {
		if len(p.Lines) >= MaxPatchLines {
			p.More = total - i
			break
		}
		p.Lines = append(p.Lines, deck.PatchLine{Kind: deck.LineAdded, Text: cutLine(l)})
	}
	if !p.Cut && p.More == 0 && len(p.Lines) < MaxPatchLines && !bytes.HasSuffix(data, []byte("\n")) {
		p.Lines = append(p.Lines, deck.PatchLine{Kind: deck.LineNote, Text: "No newline at end of file"})
	}
	return p
}

// cutLine drops a carriage return and keeps at most maxLineBytes of a
// line, on a character boundary.
func cutLine(s string) string {
	s = strings.TrimSuffix(s, "\r")
	if len(s) <= maxLineBytes {
		return s
	}
	s = s[:maxLineBytes]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// contentHash is a hash of the file's content, or "" when it cannot be
// read (deleted, or a folder).
func contentHash(path string) string {
	fh, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer fh.Close()
	h := sha256.New()
	if _, err := io.Copy(h, bufio.NewReader(fh)); err != nil {
		return ""
	}
	return string(h.Sum(nil))
}

func errText(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

// execGitLimit is execGit keeping at most limit bytes of the output; cut
// says there was more.
func execGitLimit(ctx context.Context, dir string, limit int, args ...string) (out []byte, cut bool, err error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir, "--no-pager"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	w := &limitWriter{limit: limit}
	cmd.Stdout = w
	if err := cmd.Run(); err != nil && !w.cut {
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if line := firstLine(stderr.String()); line != "" {
				return nil, false, errors.New(line)
			}
		}
		return nil, false, err
	}
	return w.buf.Bytes(), w.cut, nil
}

// limitWriter keeps the first limit bytes written to it and drops the
// rest, so a huge diff is read in part.
type limitWriter struct {
	buf   bytes.Buffer
	limit int
	cut   bool
}

func (w *limitWriter) Write(b []byte) (int, error) {
	if room := w.limit - w.buf.Len(); room < len(b) {
		w.cut = true
		w.buf.Write(b[:max(room, 0)])
		return len(b), nil
	}
	return w.buf.Write(b)
}

package diff

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// MaxCommits is how many of a branch's commits are listed; the rest are
// counted in Commits.More.
const MaxCommits = 100

// commitPatchCacheSize is how many commits' patches are remembered. A
// commit never changes, so its sha is enough of a key.
const commitPatchCacheSize = 16

// logCache is a branch's commit list as read for one HEAD, upstream and
// base commit.
type logCache struct {
	head, upstream, base string
	c                    deck.Commits
}

// ReadCommits returns the commits of t's branch since its merge-base with
// t.Base (DefaultBase when it has none), newest first, and how many files
// have uncommitted changes. The list is read again only when HEAD or the
// upstream moved; the uncommitted count is read every time. It never
// fails: a missing worktree or base is a Note.
func (r *Reader) ReadCommits(ctx context.Context, t deck.Thread) deck.Commits {
	base := t.Base
	if base == "" {
		base = DefaultBase
	}
	if t.Worktree == "" {
		return deck.Commits{Base: base, Note: "no worktree"}
	}
	key := t.Worktree + "\x00" + base
	now := r.now()
	r.mu.Lock()
	c, ok := r.commits[key]
	r.mu.Unlock()
	if ok && now.Sub(c.at) < cacheTTL {
		return c.c
	}
	cs := r.readCommits(ctx, t.Worktree, base, key)
	r.mu.Lock()
	if r.commits == nil {
		r.commits = map[string]cachedCommits{}
	}
	r.commits[key] = cachedCommits{c: cs, at: now}
	r.mu.Unlock()
	return cs
}

type cachedCommits struct {
	c  deck.Commits
	at time.Time
}

func (r *Reader) readCommits(ctx context.Context, dir, base, key string) deck.Commits {
	cs := deck.Commits{Base: base}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		cs.Note = "the worktree is gone"
		return cs
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	git := r.git()
	out, err := git(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		cs.Note = fmt.Sprintf("git rev-parse: %v", err)
		return cs
	}
	head := strings.TrimSpace(string(out))
	upstream, baseSHA := "", ""
	if out, err := git(ctx, dir, "rev-parse", "--verify", "--quiet", "@{upstream}"); err == nil {
		upstream = strings.TrimSpace(string(out))
	}
	// The base moves too (a fetch, a merge of the branch): the list
	// follows it.
	if out, err := git(ctx, dir, "rev-parse", "--verify", "--quiet", base+"^{commit}"); err == nil {
		baseSHA = strings.TrimSpace(string(out))
	}
	out, err = git(ctx, dir, "status", "--porcelain", "-z", "--untracked-files=all")
	if err != nil {
		cs.Note = fmt.Sprintf("git status: %v", err)
		return cs
	}
	uncommitted := CountStatus(out)

	r.mu.Lock()
	lc, ok := r.logs[key]
	r.mu.Unlock()
	if ok && lc.head == head && lc.upstream == upstream && lc.base == baseSHA {
		cs = lc.c
		cs.Uncommitted = uncommitted
		return cs
	}
	cs, complete := r.readLog(ctx, git, dir, base, head, upstream)
	if !complete {
		cs.Uncommitted = uncommitted
		return cs // a timeout, a lock or a missing base may pass: ask again next time
	}
	r.mu.Lock()
	if r.logs == nil {
		r.logs = map[string]logCache{}
	}
	r.logs[key] = logCache{head: head, upstream: upstream, base: baseSHA, c: cs}
	r.mu.Unlock()
	cs.Uncommitted = uncommitted
	return cs
}

// readLog lists the commits between the merge-base and head and marks the
// ones upstream has. complete is false when a git command failed, so the
// answer is not worth caching.
func (r *Reader) readLog(ctx context.Context, git gitFunc, dir, base, head, upstream string) (cs deck.Commits, complete bool) {
	cs = deck.Commits{Base: base, Head: head, Upstream: upstream != ""}
	out, err := git(ctx, dir, "merge-base", base, head)
	if err != nil {
		cs.Note = fmt.Sprintf("cannot compare with %s: %v", base, err)
		return cs, false
	}
	cs.MergeBase = strings.TrimSpace(string(out))
	span := cs.MergeBase + ".." + head
	out, err = git(ctx, dir, "log", "--no-color", "--no-ext-diff", "--numstat", "--max-count="+strconv.Itoa(MaxCommits),
		"--format=%x1e%H%x1f%h%x1f%P%x1f%at%x1f%an%x1f%s", span, "--")
	if err != nil {
		cs.Note = fmt.Sprintf("git log: %v", err)
		return cs, false
	}
	cs.List = ParseLog(out)
	if len(cs.List) == MaxCommits {
		out, err := git(ctx, dir, "rev-list", "--count", span)
		if err != nil {
			return cs, false
		}
		n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
		cs.More = max(n-MaxCommits, 0)
	}
	if upstream == "" || len(cs.List) == 0 {
		return cs, true
	}
	// The pushed commits are the ones below where HEAD and its upstream
	// meet.
	out, err = git(ctx, dir, "merge-base", head, upstream)
	if err != nil {
		// Without the pushed marks, g would refuse every commit.
		cs.Upstream = false
		return cs, false
	}
	met := strings.TrimSpace(string(out))
	out, err = git(ctx, dir, "rev-list", "--max-count="+strconv.Itoa(MaxCommits+1), cs.MergeBase+".."+met, "--")
	if err != nil {
		cs.Upstream = false
		return cs, false
	}
	pushed := map[string]bool{}
	for _, s := range strings.Fields(string(out)) {
		pushed[s] = true
	}
	for i := range cs.List {
		cs.List[i].Pushed = pushed[cs.List[i].SHA]
	}
	return cs, true
}

// ParseLog reads `git log --numstat` with the format
// "%x1e%H%x1f%h%x1f%P%x1f%at%x1f%an%x1f%s": a record per commit, its
// fields on the first line and its numstat lines after it
// ("added\tdeleted\tpath", "-\t-\tpath" for a binary file). A merge
// commit has two parents and no numstat lines.
func ParseLog(out []byte) []deck.Commit {
	var list []deck.Commit
	for rec := range strings.SplitSeq(string(out), "\x1e") {
		head, rest, _ := strings.Cut(rec, "\n")
		f := strings.Split(head, "\x1f")
		if len(f) < 6 {
			continue
		}
		c := deck.Commit{SHA: f[0], Short: f[1], Author: f[4], Subject: f[5]}
		c.Merge = len(strings.Fields(f[2])) > 1
		if sec, err := strconv.ParseInt(f[3], 10, 64); err == nil {
			c.Time = time.Unix(sec, 0)
		}
		for l := range strings.SplitSeq(rest, "\n") {
			n := strings.SplitN(l, "\t", 3)
			if len(n) < 3 {
				continue
			}
			a, errA := strconv.Atoi(n[0])
			d, errD := strconv.Atoi(n[1])
			if errA == nil && errD == nil {
				c.Added += a
				c.Deleted += d
			}
		}
		list = append(list, c)
	}
	return list
}

// CountStatus counts the files `git status --porcelain -z` lists: one
// record per file, a rename's or copy's followed by its old path.
func CountStatus(out []byte) int {
	parts := splitZ(out)
	n := 0
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		n++
		if len(p) >= 2 && (p[0] == 'R' || p[0] == 'C' || p[1] == 'R' || p[1] == 'C') {
			i++ // the old path
		}
	}
	return n
}

// ReadCommitPatch returns commit sha of t's worktree as the preview shows
// it: `git show` against the first parent, with the message's body. The
// answer is cached by sha. It never fails: a problem is the Patch's Note.
func (r *Reader) ReadCommitPatch(ctx context.Context, t deck.Thread, sha string) deck.CommitPatch {
	if t.Worktree == "" {
		return deck.CommitPatch{Patch: deck.Patch{Note: "no worktree"}}
	}
	key := t.Worktree + "\x00" + sha
	r.mu.Lock()
	p, ok := r.commitPatches[key]
	r.mu.Unlock()
	if ok {
		return p
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	args := []string{"show", "--no-color", "--no-ext-diff", "--no-textconv", "-M", "--diff-merges=first-parent",
		"--format=%x1e%b%x1e", sha, "--"}
	var (
		out []byte
		cut bool
		err error
	)
	if r.Git != nil {
		out, err = r.Git(ctx, t.Worktree, args...)
	} else {
		out, cut, err = execGitLimit(ctx, t.Worktree, maxPatchBytes, args...)
	}
	if err != nil {
		return deck.CommitPatch{Patch: deck.Patch{Note: fmt.Sprintf("git show: %v", err)}}
	}
	p = ParseShow(out, MaxPatchLines)
	p.Patch.Cut = cut
	r.mu.Lock()
	if len(r.commitPatches) >= commitPatchCacheSize {
		clear(r.commitPatches)
	}
	if r.commitPatches == nil {
		r.commitPatches = map[string]deck.CommitPatch{}
	}
	r.commitPatches[key] = p
	r.mu.Unlock()
	return p
}

// ParseShow reads `git show --format=%x1e%b%x1e`: the body between the
// two separators, then the commit's diff, keeping at most limit lines of
// it. Each file starts with a LineFile line naming it; a binary file or a
// pure rename gets a note instead of lines.
func ParseShow(out []byte, limit int) deck.CommitPatch {
	var cp deck.CommitPatch
	s := string(out)
	if _, rest, ok := strings.Cut(s, "\x1e"); ok {
		body, diff, _ := strings.Cut(rest, "\x1e")
		cp.Body = strings.TrimSpace(body)
		s = diff
	}
	s = strings.Trim(s, "\n")
	if s == "" {
		cp.Patch.Note = "no changes"
		return cp
	}
	p := &cp.Patch
	add := func(l deck.PatchLine) {
		if len(p.Lines) >= limit {
			p.More++
			return
		}
		l.Text = cutLine(l.Text)
		p.Lines = append(p.Lines, l)
	}
	file := -1 // the index of the current file's LineFile line
	var from, to string
	inHunk, hunks := false, false
	// done ends a file: one that showed no hunks gets a note.
	done := func(binary, renamed bool) {
		if file < 0 || hunks {
			return
		}
		switch {
		case binary:
			add(deck.PatchLine{Kind: deck.LineNote, Text: "binary file"})
		case renamed:
			add(deck.PatchLine{Kind: deck.LineNote, Text: "renamed, content unchanged"})
		default:
			add(deck.PatchLine{Kind: deck.LineNote, Text: "no changes in content"})
		}
	}
	binary, renamed := false, false
	name := func() string {
		switch {
		case from != "" && to != "" && from != to:
			return from + " → " + to
		case to != "":
			return to
		}
		return from
	}
	setName := func() {
		if file >= 0 && file < len(p.Lines) {
			p.Lines[file].Text = cutLine(name())
		}
	}
	for line := range strings.SplitSeq(s, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			done(binary, renamed)
			from, to = gitPaths(strings.TrimPrefix(line, "diff --git "))
			inHunk, hunks, binary, renamed = false, false, false, false
			if len(p.Lines) >= limit {
				p.More++
				file = -1
				continue
			}
			file = len(p.Lines)
			add(deck.PatchLine{Kind: deck.LineFile, Text: name()})
			continue
		}
		if !inHunk && !strings.HasPrefix(line, "@@") {
			switch {
			case strings.HasPrefix(line, "Binary files "), line == "GIT binary patch":
				binary = true
			case strings.HasPrefix(line, "rename from "):
				renamed = true
				from = strings.TrimPrefix(line, "rename from ")
				setName()
			case strings.HasPrefix(line, "rename to "):
				to = strings.TrimPrefix(line, "rename to ")
				setName()
			}
			continue
		}
		if strings.HasPrefix(line, "@@") {
			inHunk, hunks = true, true
		}
		add(patchLine(line))
	}
	done(binary, renamed)
	return cp
}

// gitPaths splits the paths of a "diff --git a/<old> b/<new>" line.
// Paths with spaces are ambiguous there; the rename lines, when there
// are any, name them exactly.
func gitPaths(s string) (from, to string) {
	s = strings.ReplaceAll(s, `"`, "")
	if i := strings.LastIndex(s, " b/"); i >= 0 && strings.HasPrefix(s, "a/") {
		return s[2:i], s[i+3:]
	}
	return s, s
}

// patchLine is one line of a hunk: its header, or a line with its diff
// marker.
func patchLine(line string) deck.PatchLine {
	switch {
	case strings.HasPrefix(line, "@@"):
		return deck.PatchLine{Kind: deck.LineHunk, Text: line}
	case line == "":
		return deck.PatchLine{Kind: deck.LineContext}
	}
	text := line[1:]
	switch line[0] {
	case '+':
		return deck.PatchLine{Kind: deck.LineAdded, Text: text}
	case '-':
		return deck.PatchLine{Kind: deck.LineDeleted, Text: text}
	case '\\':
		return deck.PatchLine{Kind: deck.LineNote, Text: strings.TrimSpace(text)}
	}
	return deck.PatchLine{Kind: deck.LineContext, Text: text}
}

// commitFilesCacheSize is how many commits' file lists are remembered.
const commitFilesCacheSize = 64

// ReadCommitFiles returns the files commit sha of t's worktree changed,
// against its first parent, with their line counts (`git show --raw
// --numstat`, parsed as Parse does). The answer is cached by sha. A
// problem is the second result; the list is then empty.
func (r *Reader) ReadCommitFiles(ctx context.Context, t deck.Thread, sha string) ([]deck.DiffFile, error) {
	if t.Worktree == "" {
		return nil, fmt.Errorf("no worktree")
	}
	key := t.Worktree + "\x00" + sha
	r.mu.Lock()
	fs, ok := r.commitFiles[key]
	r.mu.Unlock()
	if ok {
		return fs, nil
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	out, err := r.git()(ctx, t.Worktree, "show", "--no-color", "--no-ext-diff", "--format=", "--raw", "--numstat", "-z", "-M",
		"--diff-merges=first-parent", sha, "--")
	if err != nil {
		return nil, fmt.Errorf("git show: %w", err)
	}
	fs = Parse(out)
	r.mu.Lock()
	if len(r.commitFiles) >= commitFilesCacheSize {
		clear(r.commitFiles)
	}
	if r.commitFiles == nil {
		r.commitFiles = map[string][]deck.DiffFile{}
	}
	r.commitFiles[key] = fs
	r.mu.Unlock()
	return fs, nil
}

// ReadCommitFilePatch returns f's change in commit sha of t's worktree,
// against the commit's first parent: `git show <sha> -- [old] <path>`, as
// the preview shows a file. The answer is cached by sha and file.
func (r *Reader) ReadCommitFilePatch(ctx context.Context, t deck.Thread, sha string, f deck.DiffFile) deck.Patch {
	if t.Worktree == "" {
		return deck.Patch{Note: "no worktree"}
	}
	key := strings.Join([]string{t.Worktree, sha, f.OldPath, f.Path}, "\x00")
	r.mu.Lock()
	p, ok := r.patches[key]
	r.mu.Unlock()
	if ok {
		return p
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	args := []string{"show", "--no-color", "--no-ext-diff", "--no-textconv", "--format=", "-M", "--diff-merges=first-parent", sha, "--"}
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
		out, err = r.Git(ctx, t.Worktree, args...)
	} else {
		out, cut, err = execGitLimit(ctx, t.Worktree, maxPatchBytes, args...)
	}
	if err != nil {
		return deck.Patch{Note: fmt.Sprintf("git show: %v", err)}
	}
	p = ParsePatch(out, MaxPatchLines)
	p.Cut = cut
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

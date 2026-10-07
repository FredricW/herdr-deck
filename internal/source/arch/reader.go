package arch

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/diff"
)

// Timeout bounds one read: git and parsing together.
const Timeout = 30 * time.Second

// maxResults is how many results the reader keeps, most recent first.
const maxResults = 16

type gitFunc func(ctx context.Context, dir string, args ...string) ([]byte, error)

// Reader reads what a thread's branch did to its repository's shape. It
// keeps every file's facts by blob SHA and the last results by merge-base,
// head and manifest, so a reload whose HEAD did not move costs three
// small git calls, and one that moved parses only the files that changed.
// It is safe for concurrent use.
type Reader struct {
	// Tests counts test files too (arch.tests).
	Tests bool
	// Git runs git with args in dir; nil runs the real git. OpenBlobs
	// reads blobs; nil runs one `git cat-file --batch` per read.
	Git       gitFunc
	OpenBlobs func(ctx context.Context, dir string) (blobReader, error)

	facts   factCache
	mu      sync.Mutex
	results []cachedResult
}

type cachedResult struct {
	key string
	res *Result
}

func (r *Reader) git() gitFunc {
	if r.Git != nil {
		return r.Git
	}
	return execGit
}

func (r *Reader) open(ctx context.Context, dir string) (blobReader, error) {
	if r.OpenBlobs != nil {
		return r.OpenBlobs(ctx, dir)
	}
	return openCatFile(ctx, dir)
}

// Read compares t's HEAD with its merge-base with t.Base (else origin/HEAD,
// as the Files tab does). It never fails: what stops it is a Note. The
// result is shared; callers must not change it.
func (r *Reader) Read(ctx context.Context, t deck.Thread) *Result {
	start := time.Now()
	base := t.Base
	if base == "" {
		base = diff.DefaultBase
	}
	res := &Result{Base: base}
	if t.Worktree == "" {
		res.Note = "no worktree"
		return res
	}
	if fi, err := os.Stat(t.Worktree); err != nil || !fi.IsDir() {
		res.Note = "the worktree is gone"
		return res
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	git, dir := r.git(), t.Worktree
	out, err := git(ctx, dir, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		res.Note = "no commit yet: " + err.Error()
		return res
	}
	res.Head = strings.TrimSpace(string(out))
	if out, err = git(ctx, dir, "merge-base", base, res.Head); err != nil {
		res.Note = fmt.Sprintf("cannot compare with %s: %v", base, err)
		return res
	}
	res.MergeBase = strings.TrimSpace(string(out))
	// The manifest's blob is part of the cache key: until HEAD, the
	// merge-base or the manifest moves, the last result stands, and a
	// reload costs these three git calls.
	cfgBlob := ""
	if out, err := git(ctx, dir, "rev-parse", "--verify", "--quiet", res.Head+":"+ManifestPath); err == nil {
		cfgBlob = strings.TrimSpace(string(out))
	}
	key := fmt.Sprintf("%s %s %s tests=%t", res.MergeBase, res.Head, cfgBlob, r.Tests)
	if c, ok := r.cached(key); ok {
		return c
	}
	headTree, err := lsTree(ctx, git, dir, res.Head)
	if err != nil {
		res.Note = "git ls-tree: " + err.Error()
		return res
	}
	bl, err := r.open(ctx, dir)
	if err != nil {
		res.Note = "git cat-file: " + err.Error()
		return res
	}
	defer bl.close()

	cfg, configured, err := r.config(cfgBlob, bl)
	if err != nil {
		res.Note = ManifestPath + ": " + err.Error()
		return res
	}
	res.Configured = configured
	res.Language = cfg.Language
	if res.Language == "" {
		res.Language = detect(headTree, cfg)
	}
	if res.Language == "" {
		res.Note = "no Go or TypeScript files to read"
		return res
	}
	baseTree, err := lsTree(ctx, git, dir, res.MergeBase)
	if err != nil {
		res.Note = "git ls-tree: " + err.Error()
		return res
	}
	build := buildGo
	if res.Language == LangTS {
		build = buildTS
	}
	var n counts
	bs, err := build(baseTree, bl, &r.facts, cfg, &n)
	if err == nil && ctx.Err() == nil {
		var hs *snapshot
		if hs, err = build(headTree, bl, &r.facts, cfg, &n); err == nil {
			var out []byte
			if out, err = git(ctx, dir, "diff", "--raw", "--numstat", "-z", "-M", "--no-ext-diff", res.MergeBase, res.Head, "--"); err == nil {
				analyse(res, bs, hs, diff.Parse(out), cfg)
				res.Name = r.name(ctx, dir, hs)
			}
		}
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return &Result{Base: base, MergeBase: res.MergeBase, Head: res.Head, Note: "reading the graph: " + err.Error()}
	}
	res.Parsed, res.Reused, res.Took = n.parsed, n.reused, time.Since(start)
	r.keep(key, res)
	return res
}

// config reads the manifest's architecture block from its blob ("" for
// no manifest), and says whether the block says anything.
func (r *Reader) config(blob string, bl blobReader) (cfg Config, configured bool, err error) {
	if blob != "" {
		src, err := bl.read(blob)
		if err != nil {
			return cfg, false, err
		}
		if cfg, err = ParseConfig(src); err != nil {
			return cfg, false, err
		}
		configured = len(cfg.Layers)+len(cfg.Roots) > 0 || cfg.Language != ""
	}
	cfg.Tests = r.Tests
	return cfg, configured, nil
}

func (r *Reader) cached(key string) (*Result, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, c := range r.results {
		if c.key == key {
			// Most recent first.
			copy(r.results[1:i+1], r.results[:i])
			r.results[0] = c
			return c.res, true
		}
	}
	return nil, false
}

func (r *Reader) keep(key string, res *Result) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = append([]cachedResult{{key, res}}, r.results...)
	if len(r.results) > maxResults {
		r.results = r.results[:maxResults]
	}
}

// detect picks the language with more source files at the head; Go wins
// a tie, and needs a go.mod.
func detect(tree []entry, cfg Config) string {
	var gofiles, tsfiles int
	mod := false
	for _, e := range tree {
		switch {
		case path.Base(e.path) == "go.mod":
			mod = true
		case !under(e.path, cfg.Roots):
		case strings.HasSuffix(e.path, ".go") && !skipGo(e.path, cfg.Tests):
			gofiles++
		case isTSFile(e.path) && !skipTS(e.path, cfg.Tests):
			tsfiles++
		}
	}
	switch {
	case mod && gofiles > 0 && gofiles >= tsfiles:
		return LangGo
	case tsfiles > 0:
		return LangTS
	case gofiles > 0:
		return LangGo
	}
	return ""
}

// name is the repository's name: its root Go module's last element, its
// root package.json's name, else its main checkout's folder.
func (r *Reader) name(ctx context.Context, dir string, s *snapshot) string {
	for _, m := range s.modules {
		if m.dir == "." && m.path != "" {
			if strings.HasPrefix(m.path, "@") || !strings.Contains(m.path, "/") {
				return m.path
			}
			p := m.path
			if b := path.Base(p); len(b) > 1 && b[0] == 'v' && b[1] >= '0' && b[1] <= '9' {
				p = path.Dir(p)
			}
			return path.Base(p)
		}
	}
	if out, err := r.git()(ctx, dir, "rev-parse", "--path-format=absolute", "--git-common-dir"); err == nil {
		common := strings.TrimSpace(string(out))
		if filepath.Base(common) == ".git" {
			return filepath.Base(filepath.Dir(common))
		}
		return strings.TrimSuffix(filepath.Base(common), ".git")
	}
	return filepath.Base(dir)
}

package arch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// FromFiles is Read for two sets of files in memory (path → content), for
// the deck's sample data and for tests: the same graphs and analysis,
// without git. Line counts come from comparing the lines of each file as
// multisets, which is close enough for made-up changes.
func FromFiles(name, base string, before, after map[string]string, cfg Config) *Result {
	res := &Result{Base: base, MergeBase: "0000000", Head: "1111111", Name: name}
	bl := memBlobs{}
	tree := func(files map[string]string) []entry {
		var es []entry
		for _, p := range slices.Sorted(maps.Keys(files)) {
			sum := sha256.Sum256([]byte(files[p]))
			sha := hex.EncodeToString(sum[:])
			bl[sha] = []byte(files[p])
			es = append(es, entry{path: p, blob: sha})
		}
		return es
	}
	bt, ht := tree(before), tree(after)
	if src, ok := after[ManifestPath]; ok {
		c, err := ParseConfig([]byte(src))
		if err != nil {
			res.Note = ManifestPath + ": " + err.Error()
			return res
		}
		res.Configured = len(c.Layers)+len(c.Roots) > 0 || c.Language != ""
		c.Tests = cfg.Tests
		cfg = c
	}
	res.Language = cfg.Language
	if res.Language == "" {
		res.Language = detect(ht, cfg)
	}
	build := buildGo
	if res.Language == LangTS {
		build = buildTS
	}
	var c factCache
	var n counts
	bs, err := build(bt, bl, &c, cfg, &n)
	if err != nil {
		res.Note = err.Error()
		return res
	}
	hs, err := build(ht, bl, &c, cfg, &n)
	if err != nil {
		res.Note = err.Error()
		return res
	}
	analyse(res, bs, hs, changes(before, after), cfg)
	res.Parsed, res.Reused = n.parsed, n.reused
	return res
}

// changes lists the files that differ between two sets, as git would
// without rename detection.
func changes(before, after map[string]string) []deck.DiffFile {
	var out []deck.DiffFile
	for _, p := range slices.Sorted(maps.Keys(union(before, after))) {
		b, inB := before[p]
		a, inA := after[p]
		if inB && inA && a == b {
			continue
		}
		f := deck.DiffFile{Path: p}
		switch {
		case !inB:
			f.Change = deck.ChangeAdded
		case !inA:
			f.Change = deck.ChangeDeleted
		}
		f.Added, f.Deleted = lineDelta(b, a)
		out = append(out, f)
	}
	return out
}

func union(a, b map[string]string) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}

// lineDelta counts the lines of after that before lacks, and the other
// way round.
func lineDelta(before, after string) (added, deleted int) {
	count := map[string]int{}
	for l := range strings.Lines(before) {
		count[l]++
	}
	for l := range strings.Lines(after) {
		if count[l] > 0 {
			count[l]--
		} else {
			added++
		}
	}
	for _, n := range count {
		deleted += n
	}
	return added, deleted
}

// memBlobs serves blobs from memory.
type memBlobs map[string][]byte

func (m memBlobs) read(sha string) ([]byte, error) {
	b, ok := m[sha]
	if !ok {
		return nil, fmt.Errorf("no blob %s", sha)
	}
	return b, nil
}

func (memBlobs) close() {}

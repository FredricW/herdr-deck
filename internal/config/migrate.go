package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Migration is what MigrateFile found: the file's text before and after,
// and one line per key it moves.
type Migration struct {
	Path          string
	Before, After string
	Moved         []string
	// Backup is where the old file was kept: next to the file a symlink
	// points to. "" when nothing was written.
	Backup string
}

// MigrateFile reads the config file at path and moves every earlier flat
// key into its table (Migrate). With write, it keeps the old file as
// <path>.bak and writes the new one in its place, through a symlink. A
// missing file has nothing to move.
func MigrateFile(path string, write bool) (Migration, error) {
	m := Migration{Path: path}
	if path == "" {
		return m, errors.New("no config file: no home folder to find it in")
	}
	target := path
	if p, err := filepath.EvalSymlinks(path); err == nil {
		target = p
	}
	b, err := os.ReadFile(target)
	if errors.Is(err, fs.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	m.Before = string(b)
	if m.After, m.Moved, err = Migrate(m.Before); err != nil {
		return m, err
	}
	if !write || len(m.Moved) == 0 {
		return m, nil
	}
	mode := fs.FileMode(0o644)
	if st, err := os.Stat(target); err == nil {
		mode = st.Mode().Perm()
	}
	if err := os.WriteFile(target+".bak", b, mode); err != nil {
		return m, fmt.Errorf("keeping the old file: %w", err)
	}
	m.Backup = target + ".bak"
	return m, writeAtomic(target, []byte(m.After))
}

// UnifiedDiff is a unified diff of a and b, line by line, with three lines
// of context, or "" when they are the same.
func UnifiedDiff(a, b, from, to string) string {
	x, y := splitLines(a), splitLines(b)
	// The longest common subsequence, from the end: lcs[i][j] is its length
	// for x[i:] and y[j:].
	lcs := make([][]int, len(x)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(y)+1)
	}
	for i := len(x) - 1; i >= 0; i-- {
		for j := len(y) - 1; j >= 0; j-- {
			if x[i] == y[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	type op struct {
		kind byte // ' ', '-' or '+'
		line string
		i, j int // the line numbers before it, in a and b
	}
	var ops []op
	i, j := 0, 0
	for i < len(x) || j < len(y) {
		switch {
		case i < len(x) && j < len(y) && x[i] == y[j]:
			ops = append(ops, op{' ', x[i], i, j})
			i, j = i+1, j+1
		case i < len(x) && (j == len(y) || lcs[i+1][j] >= lcs[i][j+1]):
			ops = append(ops, op{'-', x[i], i, j})
			i++
		default:
			ops = append(ops, op{'+', y[j], i, j})
			j++
		}
	}
	const context = 3
	var out strings.Builder
	for k := 0; k < len(ops); {
		if ops[k].kind == ' ' {
			k++
			continue
		}
		// A hunk: from context lines before this change to context lines
		// after the last change within reach.
		start := max(k-context, 0)
		end := k
		for end < len(ops) {
			if ops[end].kind != ' ' {
				end++
				continue
			}
			run := end
			for run < len(ops) && ops[run].kind == ' ' {
				run++
			}
			if run == len(ops) || run-end > 2*context {
				end = min(end+context, len(ops))
				break
			}
			end = run
		}
		if out.Len() == 0 {
			fmt.Fprintf(&out, "--- %s\n+++ %s\n", from, to)
		}
		na, nb := 0, 0
		for _, o := range ops[start:end] {
			if o.kind != '+' {
				na++
			}
			if o.kind != '-' {
				nb++
			}
		}
		fmt.Fprintf(&out, "@@ -%s +%s @@\n", hunkRange(ops[start].i, na), hunkRange(ops[start].j, nb))
		for _, o := range ops[start:end] {
			out.WriteByte(o.kind)
			out.WriteString(o.line)
			out.WriteByte('\n')
		}
		k = end
	}
	return out.String()
}

// hunkRange is a hunk's "start,count", 1-based; an empty range names the
// line before it.
func hunkRange(before, n int) string {
	if n == 0 {
		return fmt.Sprintf("%d,0", before)
	}
	if n == 1 {
		return fmt.Sprintf("%d", before+1)
	}
	return fmt.Sprintf("%d,%d", before+1, n)
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

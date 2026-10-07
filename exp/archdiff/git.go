package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// git runs git in the repository and returns its output.
func git(repo string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// entry is one file in a commit's tree.
type entry struct{ path, blob string }

// lsTree lists every blob in a commit's tree, without checking it out.
func lsTree(repo, rev string) ([]entry, error) {
	out, err := git(repo, "ls-tree", "-r", "-z", "--full-tree", rev)
	if err != nil {
		return nil, err
	}
	var es []entry
	for rec := range strings.SplitSeq(string(out), "\x00") {
		// "<mode> <type> <sha>\t<path>"
		meta, path, ok := strings.Cut(rec, "\t")
		if !ok {
			continue
		}
		f := strings.Fields(meta)
		if len(f) != 3 || f[1] != "blob" {
			continue
		}
		es = append(es, entry{path: path, blob: f[2]})
	}
	return es, nil
}

// blobs reads blob contents through one long-lived `git cat-file --batch`,
// so a whole tree costs one process, not one per file.
type blobs struct {
	cmd *exec.Cmd
	in  io.WriteCloser
	out *bufio.Reader
}

func openBlobs(repo string) (*blobs, error) {
	cmd := exec.Command("git", "-C", repo, "cat-file", "--batch")
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &blobs{cmd: cmd, in: in, out: bufio.NewReaderSize(out, 1<<16)}, nil
}

func (b *blobs) read(sha string) ([]byte, error) {
	if _, err := fmt.Fprintln(b.in, sha); err != nil {
		return nil, err
	}
	head, err := b.out.ReadString('\n')
	if err != nil {
		return nil, err
	}
	f := strings.Fields(head) // "<sha> <type> <size>" or "<sha> missing"
	if len(f) != 3 {
		return nil, fmt.Errorf("cat-file: %s", strings.TrimSpace(head))
	}
	n, err := strconv.Atoi(f[2])
	if err != nil {
		return nil, err
	}
	buf := make([]byte, n+1) // the content and its trailing newline
	if _, err := io.ReadFull(b.out, buf); err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (b *blobs) close() {
	b.in.Close()
	_ = b.cmd.Wait() // the batch ends when its input closes
}

// change is one file in the diff between base and head.
type change struct {
	status   byte // A M D R
	score    int  // similarity for renames, 0-100
	old, new string
	add, del int
	binary   bool
}

// diffFiles reads the base..head file list with renames and line counts:
// --raw for the status and similarity, --numstat for the counts.
func diffFiles(repo, base, head string) ([]change, error) {
	out, err := git(repo, "diff", "--raw", "--numstat", "-z", "-M", "--no-ext-diff", base, head)
	if err != nil {
		return nil, err
	}
	recs := strings.Split(string(out), "\x00")
	var cs []change
	byPath := map[string]*change{}
	i := 0
	for i < len(recs) {
		r := recs[i]
		if strings.HasPrefix(r, ":") {
			// ":old new oldsha newsha S\0path\0" or "R86\0old\0new\0"
			f := strings.Fields(r)
			st := f[len(f)-1]
			c := change{status: st[0]}
			if len(st) > 1 {
				c.score, _ = strconv.Atoi(st[1:])
			}
			if c.status == 'R' || c.status == 'C' {
				c.old, c.new = recs[i+1], recs[i+2]
				i += 3
			} else {
				c.old, c.new = recs[i+1], recs[i+1]
				i += 2
			}
			if c.status == 'A' {
				c.old = ""
			}
			if c.status == 'D' {
				c.new = ""
			}
			cs = append(cs, c)
			continue
		}
		if r == "" {
			i++
			continue
		}
		// numstat: "add\tdel\tpath" or, for a rename, "add\tdel\t" then old, new.
		f := strings.SplitN(r, "\t", 3)
		if len(f) < 3 {
			i++
			continue
		}
		path := f[2]
		step := 1
		if path == "" {
			path = recs[i+2]
			step = 3
		}
		for k := range cs {
			p := cs[k].new
			if p == "" {
				p = cs[k].old
			}
			byPath[p] = &cs[k]
		}
		if c := byPath[path]; c != nil {
			if f[0] == "-" {
				c.binary = true
			} else {
				c.add, _ = strconv.Atoi(f[0])
				c.del, _ = strconv.Atoi(f[1])
			}
		}
		i += step
	}
	return cs, nil
}

package arch

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Everything here reads git objects: no checkout, no index refresh
// (GIT_OPTIONAL_LOCKS=0), nothing written to the repository.

// gitEnv keeps git from taking optional locks or asking for credentials.
var gitEnv = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")

// execGit runs git in dir and returns its standard output; a failure says
// git's first line of stderr.
func execGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir, "--no-pager"}, args...)...)
	cmd.Env = gitEnv
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if line, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n"); line != "" {
				return nil, errors.New(line)
			}
		}
		return nil, err
	}
	return out, nil
}

// entry is one file in a commit's tree.
type entry struct{ path, blob string }

// lsTree lists every blob in a commit's tree.
func lsTree(ctx context.Context, git gitFunc, dir, rev string) ([]entry, error) {
	out, err := git(ctx, dir, "ls-tree", "-r", "-z", "--full-tree", rev)
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

// blobReader reads blob contents by SHA.
type blobReader interface {
	read(sha string) ([]byte, error)
	close()
}

// catFile reads blobs through one long-lived `git cat-file --batch`, so a
// whole tree costs one process, not one per file.
type catFile struct {
	cmd *exec.Cmd
	in  io.WriteCloser
	out *bufio.Reader
}

func openCatFile(ctx context.Context, dir string) (*catFile, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "cat-file", "--batch")
	cmd.Env = gitEnv
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
	return &catFile{cmd: cmd, in: in, out: bufio.NewReaderSize(out, 1<<16)}, nil
}

// maxBlob is the largest file read; a bigger one is not source worth
// parsing (generated code, bundles), and is skipped.
const maxBlob = 2 << 20

func (b *catFile) read(sha string) ([]byte, error) {
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
	if n > maxBlob {
		return nil, errTooBig
	}
	return buf[:n], nil
}

var errTooBig = errors.New("file too big")

func (b *catFile) close() {
	_ = b.in.Close()
	_ = b.cmd.Wait() // the batch ends when its input closes
}

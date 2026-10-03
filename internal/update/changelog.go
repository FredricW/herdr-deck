package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RawBase is where GitHub serves a repository's files at a ref.
const RawBase = "https://raw.githubusercontent.com"

// maxChangelog caps how much of a remote CHANGELOG.md is read.
const maxChangelog = 1 << 20

// Changelog reads the CHANGELOG.md of the newer version st found, for the
// What's new view. A GitHub install reads it at the newer tag from GitHub's
// raw host; a linked checkout reads origin's commit with `git show`, which
// works once the checkout has fetched it, else origin's branch as last
// fetched. It only reads.
func (u Updater) Changelog(ctx context.Context, st Status) ([]byte, error) {
	switch st.Install.Kind {
	case GitHub:
		if st.Remote.Tag == "" {
			return nil, errors.New("no newer tag")
		}
		return u.rawChangelog(ctx, st.Install.Repo, st.Remote.Tag)
	case Local:
		root := st.Install.Root
		var err error
		for _, rev := range []string{st.Remote.Head, "refs/remotes/origin/" + st.Remote.Branch} {
			if rev == "" || rev == "refs/remotes/origin/" {
				continue
			}
			var out []byte
			if out, err = u.Exec.Output(ctx, root, "git", "show", rev+":CHANGELOG.md"); err == nil {
				return out, nil
			}
		}
		if err == nil {
			err = errors.New("no origin commit")
		}
		return nil, fmt.Errorf("git show CHANGELOG.md: %w", err)
	}
	return nil, fmt.Errorf("cannot read the changelog of a %q install", st.Install.Kind)
}

// rawChangelog fetches repo's (OWNER/REPO[/SUBDIR]) CHANGELOG.md at ref.
func (u Updater) rawChangelog(ctx context.Context, repo, ref string) ([]byte, error) {
	parts := strings.SplitN(repo, "/", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("no repository in %q", repo)
	}
	path := "CHANGELOG.md"
	if len(parts) == 3 {
		path = parts[2] + "/" + path
	}
	base := u.RawBase
	if base == "" {
		base = RawBase
	}
	url := strings.TrimSuffix(base, "/") + "/" + parts[0] + "/" + parts[1] + "/" + ref + "/" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := u.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxChangelog))
}

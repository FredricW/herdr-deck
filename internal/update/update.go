// Package update finds out how herdr installed the deck, whether a newer
// version exists, and updates it: through `herdr plugin install --ref` for
// a plugin installed from GitHub, and by pulling and rebuilding for a
// checkout linked with `herdr plugin link`.
//
// Nothing here runs on its own: the deck only checks (Checker), and only the
// `herdr-deck update` command changes anything (Updater.Apply). Every
// command goes through Exec, so tests never run git, go or herdr.
package update

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// PluginID is the deck's id in herdr-plugin.toml.
const PluginID = "herdr-deck"

// Binary is where the plugin's build puts the deck, relative to its root.
var Binary = filepath.Join("bin", "herdr-deck")

// Exec runs the commands an update needs.
type Exec interface {
	// Output runs argv in dir ("" for the current folder) and returns its
	// standard output.
	Output(ctx context.Context, dir string, argv ...string) ([]byte, error)
	// Run runs argv in dir and shows its output to the user.
	Run(ctx context.Context, dir string, argv ...string) error
}

// OSExec runs real commands. Run writes their output to Stdout and Stderr.
type OSExec struct {
	Stdout, Stderr io.Writer
}

// Output runs argv and returns its standard output. A failure carries the
// command's last line of standard error.
func (OSExec) Output(ctx context.Context, dir string, argv ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	// Never ask for a password: the deck checks from inside its TUI.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if os.Getenv("GIT_SSH_COMMAND") == "" {
		cmd.Env = append(cmd.Env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := lastLine(stderr.String()); msg != "" {
			return out, fmt.Errorf("%s: %w: %s", argv[0], err, msg)
		}
		return out, fmt.Errorf("%s: %w", argv[0], err)
	}
	return out, nil
}

// Run runs argv with its output going to e.Stdout and e.Stderr.
func (e OSExec) Run(ctx context.Context, dir string, argv ...string) error {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = e.Stdout, e.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
	}
	return nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// Kind is how herdr installed the plugin.
type Kind string

const (
	// GitHub is `herdr plugin install OWNER/REPO`: a managed checkout that
	// herdr builds and replaces on every install.
	GitHub Kind = "github"
	// Local is `herdr plugin link <folder>`: the user's own checkout.
	Local Kind = "local"
)

// Install is the deck's entry in `herdr plugin list`.
type Install struct {
	Kind Kind
	// Root is the plugin folder; the deck is Root/bin/herdr-deck.
	Root string
	// Repo is OWNER/REPO[/SUBDIR] for a GitHub install.
	Repo string
	// Version is the installed manifest's version.
	Version string
}

// ErrNotInstalled means herdr has no plugin named herdr-deck.
var ErrNotInstalled = errors.New("herdr-deck is not installed as a herdr plugin")

// source names where updates come from; the cache is kept per source.
func (in Install) source() string {
	if in.Kind == GitHub {
		return "github:" + in.Repo
	}
	return string(in.Kind) + ":" + in.Root
}

// repoURL is the GitHub repository's clone URL, without any subdirectory.
func (in Install) repoURL() string {
	parts := strings.SplitN(in.Repo, "/", 3)
	if len(parts) < 2 {
		return ""
	}
	return "https://github.com/" + parts[0] + "/" + parts[1] + ".git"
}

// Running is the deck that is running now.
type Running struct {
	// Version is the version stamped in with -ldflags, such as 0.1.0 or a
	// git describe; "" when there is none.
	Version string
	// Commit is the VCS revision Go stamped in; "" when unknown.
	Commit string
}

// Remote is what the update source offers. It is what the cache keeps.
type Remote struct {
	// Tag is the newest release tag (vX.Y.Z) for a GitHub install.
	Tag string `json:"tag,omitempty"`
	// Branch and Head are origin's default branch and its commit, for a
	// linked checkout.
	Branch string `json:"branch,omitempty"`
	Head   string `json:"head,omitempty"`
}

// Status is the outcome of a check.
type Status struct {
	Install Install
	Remote  Remote
	// Current is the running version (GitHub) or commit (linked).
	Current string
	// Newer says the source has something newer than the running deck.
	Newer bool
}

// Label is the newer version for display: a tag, or a short commit.
func (s Status) Label() string {
	if s.Install.Kind == GitHub {
		return s.Remote.Tag
	}
	return short(s.Remote.Head)
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// Summary is one line for `herdr-deck update --check`.
func (s Status) Summary() string {
	switch s.Install.Kind {
	case GitHub:
		if s.Newer {
			return fmt.Sprintf("herdr-deck %s is installed from %s; %s is available: run `herdr-deck update`.", s.Current, s.Install.Repo, s.Remote.Tag)
		}
		return fmt.Sprintf("herdr-deck %s is up to date (newest release %s).", s.Current, s.Remote.Tag)
	case Local:
		if s.Newer {
			return fmt.Sprintf("herdr-deck %s runs from the linked checkout %s; origin/%s is at %s: run `herdr-deck update` to pull and rebuild.", short(s.Current), s.Install.Root, s.Remote.Branch, short(s.Remote.Head))
		}
		return fmt.Sprintf("herdr-deck %s is up to date with origin/%s.", short(s.Current), s.Remote.Branch)
	}
	return ""
}

// Updater checks for and applies updates.
type Updater struct {
	Exec Exec
	// Herdr is the herdr command; "herdr" when empty.
	Herdr   string
	Running Running
	// Out gets one line per step of an update.
	Out io.Writer
}

func (u Updater) herdr() string {
	if u.Herdr == "" {
		return "herdr"
	}
	return u.Herdr
}

func (u Updater) say(format string, args ...any) {
	if u.Out != nil {
		fmt.Fprintf(u.Out, format+"\n", args...)
	}
}

// pluginList is the part of `herdr plugin list --json` the deck reads.
type pluginList struct {
	Result struct {
		Plugins []struct {
			ID      string `json:"plugin_id"`
			Root    string `json:"plugin_root"`
			Version string `json:"version"`
			Source  struct {
				Kind   string `json:"kind"`
				Owner  string `json:"owner"`
				Repo   string `json:"repo"`
				Subdir string `json:"subdir"`
			} `json:"source"`
		} `json:"plugins"`
	} `json:"result"`
}

// Install asks herdr how the deck is installed.
func (u Updater) Install(ctx context.Context) (Install, error) {
	out, err := u.Exec.Output(ctx, "", u.herdr(), "plugin", "list", "--plugin", PluginID, "--json")
	if err != nil {
		return Install{}, fmt.Errorf("herdr plugin list: %w", err)
	}
	var l pluginList
	if err := json.Unmarshal(out, &l); err != nil {
		return Install{}, fmt.Errorf("herdr plugin list: %w", err)
	}
	for _, p := range l.Result.Plugins {
		if p.ID != PluginID {
			continue
		}
		in := Install{Kind: Kind(p.Source.Kind), Root: p.Root, Version: p.Version}
		switch in.Kind {
		case GitHub:
			if p.Source.Owner == "" || p.Source.Repo == "" {
				return Install{}, errors.New("herdr plugin list: the GitHub install names no repository")
			}
			in.Repo = p.Source.Owner + "/" + p.Source.Repo
			if p.Source.Subdir != "" {
				in.Repo += "/" + strings.Trim(p.Source.Subdir, "/")
			}
		case Local:
			if in.Root == "" {
				return Install{}, errors.New("herdr plugin list: the linked install names no folder")
			}
		default:
			return Install{}, fmt.Errorf("herdr-deck is installed from %q, which the deck cannot update", p.Source.Kind)
		}
		return in, nil
	}
	return Install{}, ErrNotInstalled
}

// Latest asks the source what it offers: the newest vX.Y.Z tag on GitHub,
// or origin's default branch and commit for a linked checkout. It only
// reads (git ls-remote).
func (u Updater) Latest(ctx context.Context, in Install) (Remote, error) {
	switch in.Kind {
	case GitHub:
		url := in.repoURL()
		out, err := u.Exec.Output(ctx, "", "git", "ls-remote", "--tags", "--refs", url)
		if err != nil {
			return Remote{}, fmt.Errorf("git ls-remote %s: %w", url, err)
		}
		tag := newestTag(string(out))
		if tag == "" {
			return Remote{}, fmt.Errorf("%s has no release tags", in.Repo)
		}
		return Remote{Tag: tag}, nil
	case Local:
		out, err := u.Exec.Output(ctx, in.Root, "git", "ls-remote", "--symref", "origin", "HEAD")
		if err != nil {
			return Remote{}, fmt.Errorf("git ls-remote origin: %w", err)
		}
		r := parseSymref(string(out))
		if r.Branch == "" || r.Head == "" {
			return Remote{}, errors.New("git ls-remote origin: no default branch")
		}
		return r, nil
	}
	return Remote{}, fmt.Errorf("cannot update a %q install", in.Kind)
}

// Compare decides whether r is newer than the running deck.
func (u Updater) Compare(ctx context.Context, in Install, r Remote) Status {
	st := Status{Install: in, Remote: r}
	switch in.Kind {
	case GitHub:
		st.Current = u.Running.Version
		if _, ok := parseVersion(st.Current); !ok {
			st.Current = in.Version
		}
		st.Newer = newer(r.Tag, st.Current)
	case Local:
		st.Current = u.commit(ctx, in)
		st.Newer = r.Head != "" && r.Head != st.Current && !u.isAncestor(ctx, in.Root, r.Head, st.Current)
	}
	return st
}

// Check is Install, Latest and Compare in one, without a cache.
func (u Updater) Check(ctx context.Context) (Status, error) {
	in, err := u.Install(ctx)
	if err != nil {
		return Status{}, err
	}
	r, err := u.Latest(ctx, in)
	if err != nil {
		return Status{Install: in}, err
	}
	return u.Compare(ctx, in, r), nil
}

// commit is the running deck's commit, else the checkout's HEAD.
func (u Updater) commit(ctx context.Context, in Install) string {
	if u.Running.Commit != "" {
		return u.Running.Commit
	}
	return u.head(ctx, in.Root)
}

func (u Updater) head(ctx context.Context, root string) string {
	out, err := u.Exec.Output(ctx, root, "git", "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// isAncestor says whether commit a is in b's history, so b is not behind.
// An unknown commit (one origin has and the checkout has not fetched) is
// not.
func (u Updater) isAncestor(ctx context.Context, root, a, b string) bool {
	if b == "" {
		return false
	}
	_, err := u.Exec.Output(ctx, root, "git", "merge-base", "--is-ancestor", a, b)
	return err == nil
}

// Apply updates the deck to what st found. A GitHub install is reinstalled
// by herdr at the newer tag. A linked checkout is pulled (only on origin's
// default branch with no local changes) and rebuilt into bin/herdr-deck
// with an atomic rename. Running decks see the new binary and restart.
func (u Updater) Apply(ctx context.Context, st Status) error {
	switch st.Install.Kind {
	case GitHub:
		if !st.Newer {
			u.say("herdr-deck %s is up to date (newest release %s).", st.Current, st.Remote.Tag)
			return nil
		}
		u.say("Installing herdr-deck %s through herdr…", st.Remote.Tag)
		return u.Exec.Run(ctx, "", u.herdr(), "plugin", "install", st.Install.Repo, "--ref", st.Remote.Tag, "--yes")
	case Local:
		return u.applyLocal(ctx, st)
	}
	return fmt.Errorf("cannot update a %q install", st.Install.Kind)
}

func (u Updater) applyLocal(ctx context.Context, st Status) error {
	root := st.Install.Root
	out, err := u.Exec.Output(ctx, root, "git", "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return fmt.Errorf("%s is not on a branch (detached HEAD); check out %s there, or update it yourself", root, st.Remote.Branch)
	}
	if branch := strings.TrimSpace(string(out)); branch != st.Remote.Branch {
		return fmt.Errorf("%s is on branch %s, not origin's default branch %s; switch to %s, or update it yourself", root, branch, st.Remote.Branch, st.Remote.Branch)
	}
	out, err = u.Exec.Output(ctx, root, "git", "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("%s has uncommitted changes; commit or stash them, or update it yourself", root)
	}
	if st.Newer {
		u.say("Pulling %s into %s…", st.Remote.Branch, root)
		if err := u.Exec.Run(ctx, root, "git", "pull", "--ff-only", "origin", st.Remote.Branch); err != nil {
			return err
		}
	}
	head := u.head(ctx, root)
	bin := filepath.Join(root, Binary)
	if head != "" && head == u.Running.Commit {
		if _, err := os.Stat(bin); err == nil {
			u.say("herdr-deck %s is up to date with origin/%s.", short(head), st.Remote.Branch)
			return nil
		}
	}
	return u.build(ctx, root, bin)
}

// build compiles the checkout into a temporary file next to bin and renames
// it over bin, so a deck never sees a half-written binary.
func (u Updater) build(ctx context.Context, root, bin string) error {
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(bin), ".herdr-deck-*")
	if err != nil {
		return err
	}
	tmp.Close()
	defer os.Remove(tmp.Name()) // a no-op once renamed
	// The version make build stamps in.
	describe, _ := u.Exec.Output(ctx, root, "git", "describe", "--tags", "--always", "--dirty")
	argv := []string{"go", "build"}
	if v := strings.TrimSpace(string(describe)); v != "" {
		argv = append(argv, "-ldflags", "-X main.version="+v)
	}
	argv = append(argv, "-o", tmp.Name(), "./cmd/herdr-deck")
	u.say("Building %s…", bin)
	if err := u.Exec.Run(ctx, root, argv...); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), bin); err != nil {
		return err
	}
	u.say("Built %s; running decks restart with it.", bin)
	return nil
}

// newestTag picks the highest vX.Y.Z tag from `git ls-remote --tags`.
// Pre-releases and other tags are skipped.
func newestTag(lsRemote string) string {
	best, bestV := "", version{}
	for _, line := range strings.Split(lsRemote, "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		tag, ok := strings.CutPrefix(f[1], "refs/tags/")
		if !ok || !strings.HasPrefix(tag, "v") {
			continue
		}
		v, ok := parseVersion(tag)
		if !ok || v.suffix {
			continue
		}
		if best == "" || bestV.less(v) {
			best, bestV = tag, v
		}
	}
	return best
}

// parseSymref reads `git ls-remote --symref origin HEAD`.
func parseSymref(out string) Remote {
	var r Remote
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		switch {
		case len(f) == 3 && f[0] == "ref:" && f[2] == "HEAD":
			r.Branch = strings.TrimPrefix(f[1], "refs/heads/")
		case len(f) == 2 && f[1] == "HEAD":
			r.Head = f[0]
		}
	}
	return r
}

// version is the X.Y.Z of a version string. suffix says something
// followed it, such as a pre-release or a git describe's "-3-gabc1234".
type version struct {
	major, minor, patch int
	suffix              bool
}

func (a version) less(b version) bool {
	if a.major != b.major {
		return a.major < b.major
	}
	if a.minor != b.minor {
		return a.minor < b.minor
	}
	return a.patch < b.patch
}

// parseVersion reads "1.2.3" or "v1.2.3", with anything after a '-' or '+'.
func parseVersion(s string) (version, bool) {
	s = strings.TrimPrefix(s, "v")
	core, _, cut := strings.Cut(s, "-")
	if !cut {
		core, _, cut = strings.Cut(s, "+")
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return version{}, false
	}
	var n [3]int
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 {
			return version{}, false
		}
		n[i] = v
	}
	return version{n[0], n[1], n[2], cut}, true
}

// newer says whether tag is a higher X.Y.Z than current. A describe of a
// later commit (0.1.0-3-gabc) counts as 0.1.0. An unreadable current
// version makes any tag newer.
func newer(tag, current string) bool {
	t, ok := parseVersion(tag)
	if !ok {
		return false
	}
	c, ok := parseVersion(current)
	if !ok {
		return true
	}
	return c.less(t)
}

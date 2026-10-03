package update

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// fakeExec answers commands from a table and records every call, so no
// test runs git, go or herdr.
type fakeExec struct {
	out   map[string]string // "dir|argv..." → stdout
	fail  map[string]error  // "dir|argv..." → error
	calls []string
	// onRun runs for Run calls, e.g. to write the file a build makes.
	onRun func(dir string, argv []string) error
}

func key(dir string, argv []string) string { return dir + "|" + strings.Join(argv, " ") }

func (f *fakeExec) Output(_ context.Context, dir string, argv ...string) ([]byte, error) {
	k := key(dir, argv)
	f.calls = append(f.calls, "output "+k)
	if err, ok := f.fail[k]; ok {
		return nil, err
	}
	if out, ok := f.out[k]; ok {
		return []byte(out), nil
	}
	return nil, fmt.Errorf("unexpected command %q", k)
}

func (f *fakeExec) Run(_ context.Context, dir string, argv ...string) error {
	k := key(dir, argv)
	f.calls = append(f.calls, "run "+k)
	if err, ok := f.fail[k]; ok {
		return err
	}
	if f.onRun != nil {
		return f.onRun(dir, argv)
	}
	return nil
}

func (f *fakeExec) ran(prefix string) bool {
	return slices.ContainsFunc(f.calls, func(c string) bool { return strings.HasPrefix(c, prefix) })
}

const (
	githubList = `{"id":"cli:plugin","result":{"plugins":[{"plugin_id":"herdr-deck","plugin_root":"/plugins/github/herdr-deck-0123","version":"0.1.0","source":{"kind":"github","owner":"acme","repo":"herdr-deck","resolved_commit":"aaaa"}}],"type":"plugin_list"}}`
	tags       = "1111111\trefs/tags/v0.1.0\n2222222\trefs/tags/v0.10.0\n3333333\trefs/tags/v0.9.1\n4444444\trefs/tags/v1.0.0-rc.1\n5555555\trefs/tags/latest\n"
	listCmd    = "|herdr plugin list --plugin herdr-deck --json"
	tagsCmd    = "|git ls-remote --tags --refs https://github.com/acme/herdr-deck.git"
	oldCommit  = "1234567890abcdef1234567890abcdef12345678"
	newCommit  = "fedcba0987654321fedcba0987654321fedcba09"
)

func localList(root string) string {
	return `{"id":"cli:plugin","result":{"plugins":[{"plugin_id":"herdr-deck","plugin_root":"` + root + `","version":"0.1.0","source":{"kind":"local"}}],"type":"plugin_list"}}`
}

func TestInstall(t *testing.T) {
	tests := []struct {
		name, list string
		want       Install
		wantErr    string
	}{
		{"github", githubList, Install{Kind: GitHub, Root: "/plugins/github/herdr-deck-0123", Repo: "acme/herdr-deck", Version: "0.1.0"}, ""},
		{"github subdir", strings.Replace(githubList, `"repo":"herdr-deck"`, `"repo":"tools","subdir":"deck/"`, 1), Install{Kind: GitHub, Root: "/plugins/github/herdr-deck-0123", Repo: "acme/tools/deck", Version: "0.1.0"}, ""},
		{"local", localList("/src/herdr-deck"), Install{Kind: Local, Root: "/src/herdr-deck", Version: "0.1.0"}, ""},
		{"not installed", `{"result":{"plugins":[]}}`, Install{}, "not installed"},
		{"other kind", strings.Replace(githubList, `"kind":"github"`, `"kind":"tarball"`, 1), Install{}, `"tarball"`},
		{"bad json", `herdr: no server`, Install{}, "herdr plugin list"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := &fakeExec{out: map[string]string{listCmd: tt.list}}
			got, err := Updater{Exec: fe}.Install(context.Background())
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("Install = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestInstallUsesHerdrCommand(t *testing.T) {
	fe := &fakeExec{out: map[string]string{"|/opt/herdr plugin list --plugin herdr-deck --json": githubList}}
	if _, err := (Updater{Exec: fe, Herdr: "/opt/herdr"}).Install(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestNewestTag(t *testing.T) {
	if got := newestTag(tags); got != "v0.10.0" {
		t.Errorf("newestTag = %q, want v0.10.0 (numeric order, no pre-releases)", got)
	}
	if got := newestTag("abc\trefs/tags/nightly\n"); got != "" {
		t.Errorf("newestTag = %q, want none", got)
	}
}

func TestNewer(t *testing.T) {
	tests := []struct {
		tag, current string
		want         bool
	}{
		{"v0.2.0", "0.1.0", true},
		{"v0.1.0", "0.1.0", false},
		{"v0.1.0", "v0.2.0", false},
		{"v0.10.0", "0.9.9", true},
		{"v1.0.0", "0.99.0", true},
		{"v0.1.0", "v0.1.0-3-gabc1234-dirty", false},
		{"v0.1.1", "v0.1.0-3-gabc1234", true},
		{"v0.1.0", "devel", true},
		{"nightly", "0.1.0", false},
	}
	for _, tt := range tests {
		if got := newer(tt.tag, tt.current); got != tt.want {
			t.Errorf("newer(%q, %q) = %v, want %v", tt.tag, tt.current, got, tt.want)
		}
	}
}

func TestCheckGitHub(t *testing.T) {
	tests := []struct {
		name, running string
		wantCurrent   string
		wantNewer     bool
	}{
		{"older", "0.9.1", "0.9.1", true},
		{"same", "v0.10.0", "v0.10.0", false},
		{"no stamped version uses the manifest's", "", "0.1.0", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := &fakeExec{out: map[string]string{listCmd: githubList, tagsCmd: tags}}
			u := Updater{Exec: fe, Running: Running{Version: tt.running}}
			st, err := u.Check(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if st.Current != tt.wantCurrent || st.Newer != tt.wantNewer || st.Label() != "v0.10.0" {
				t.Errorf("Check = current %q newer %v label %q; want %q %v v0.10.0", st.Current, st.Newer, st.Label(), tt.wantCurrent, tt.wantNewer)
			}
		})
	}
}

func TestApplyGitHub(t *testing.T) {
	fe := &fakeExec{out: map[string]string{listCmd: githubList, tagsCmd: tags}}
	var out bytes.Buffer
	u := Updater{Exec: fe, Running: Running{Version: "0.1.0"}, Out: &out}
	st, err := u.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Apply(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	if want := "run |herdr plugin install acme/herdr-deck --ref v0.10.0 --yes"; !fe.ran(want) {
		t.Errorf("calls = %q, want %q", fe.calls, want)
	}

	// Up to date: nothing is installed.
	fe.calls = nil
	u.Running.Version = "0.10.0"
	st, _ = u.Check(context.Background())
	if err := u.Apply(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	if fe.ran("run ") {
		t.Errorf("an up-to-date deck ran %q", fe.calls)
	}
	if !strings.Contains(out.String(), "up to date") {
		t.Errorf("output = %q, want it to say up to date", out.String())
	}
}

// localExec is a linked checkout at root on branch main, clean, with
// origin's main at newCommit.
func localExec(root string) *fakeExec {
	return &fakeExec{
		out: map[string]string{
			listCmd: localList(root),
			root + "|git ls-remote --symref origin HEAD":                          "ref: refs/heads/main\tHEAD\n" + newCommit + "\tHEAD\n",
			root + "|git symbolic-ref --quiet --short HEAD":                       "main\n",
			root + "|git status --porcelain --untracked-files=no":                 "",
			root + "|git rev-parse HEAD":                                          newCommit + "\n",
			root + "|git describe --tags --always --dirty":                        "v0.1.0-4-gfedcba0\n",
			root + "|git merge-base --is-ancestor " + oldCommit + " " + newCommit: "",
		},
		fail: map[string]error{
			root + "|git merge-base --is-ancestor " + newCommit + " " + oldCommit: errors.New("exit status 1"),
		},
		onRun: func(dir string, argv []string) error {
			if argv[0] == "go" {
				// go build -o <tmp>: write the "binary".
				return os.WriteFile(argv[len(argv)-2], []byte("new deck"), 0o755)
			}
			return nil
		},
	}
}

func TestCheckLocal(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name, running string
		want          bool
	}{
		{"behind origin", oldCommit, true},
		{"at origin", newCommit, false},
		{"unknown commit falls back to HEAD", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := Updater{Exec: localExec(root), Running: Running{Commit: tt.running}}
			st, err := u.Check(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if st.Newer != tt.want || st.Label() != newCommit[:7] || st.Remote.Branch != "main" {
				t.Errorf("Check = newer %v label %q branch %q; want %v %q main", st.Newer, st.Label(), st.Remote.Branch, tt.want, newCommit[:7])
			}
		})
	}

	// A checkout ahead of origin (the running commit has origin's head in
	// its history) has nothing newer.
	fe := localExec(root)
	ahead := "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
	fe.out[root+"|git merge-base --is-ancestor "+newCommit+" "+ahead] = ""
	st, err := Updater{Exec: fe, Running: Running{Commit: ahead}}.Check(context.Background())
	if err != nil || st.Newer {
		t.Errorf("ahead of origin: newer %v, err %v; want false, nil", st.Newer, err)
	}
}

func TestApplyLocalPullsAndBuildsAtomically(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin", "herdr-deck")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("old deck"), 0o755); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(bin)
	fe := localExec(root)
	u := Updater{Exec: fe, Running: Running{Commit: oldCommit}, Out: &bytes.Buffer{}}
	st, err := u.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Apply(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	if !fe.ran("run " + root + "|git pull --ff-only origin main") {
		t.Errorf("calls = %q, want a pull", fe.calls)
	}
	if !fe.ran("run " + root + "|go build -ldflags -X main.version=v0.1.0-4-gfedcba0 -o " + filepath.Join(root, "bin", ".herdr-deck-")) {
		t.Errorf("calls = %q, want a build to a temporary file in bin/", fe.calls)
	}
	b, _ := os.ReadFile(bin)
	after, _ := os.Stat(bin)
	if string(b) != "new deck" || os.SameFile(before, after) {
		t.Errorf("bin/herdr-deck = %q (same file %v), want the new build renamed over it", b, os.SameFile(before, after))
	}
	left, _ := filepath.Glob(filepath.Join(root, "bin", ".herdr-deck-*"))
	if len(left) != 0 {
		t.Errorf("temporary files left: %q", left)
	}
}

func TestApplyLocalUpToDate(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "herdr-deck"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	fe := localExec(root)
	u := Updater{Exec: fe, Running: Running{Commit: newCommit}, Out: &bytes.Buffer{}}
	st, _ := u.Check(context.Background())
	if err := u.Apply(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	if fe.ran("run ") {
		t.Errorf("an up-to-date checkout ran %q", fe.calls)
	}
}

func TestApplyLocalBuildFailureKeepsBinary(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin", "herdr-deck")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("old deck"), 0o755); err != nil {
		t.Fatal(err)
	}
	fe := localExec(root)
	fe.onRun = func(_ string, argv []string) error {
		if argv[0] == "go" {
			return errors.New("build failed")
		}
		return nil
	}
	u := Updater{Exec: fe, Running: Running{Commit: oldCommit}, Out: &bytes.Buffer{}}
	st, _ := u.Check(context.Background())
	if err := u.Apply(context.Background(), st); err == nil {
		t.Fatal("Apply succeeded, want the build error")
	}
	if b, _ := os.ReadFile(bin); string(b) != "old deck" {
		t.Errorf("bin/herdr-deck = %q, want the old binary kept", b)
	}
	left, _ := filepath.Glob(filepath.Join(root, "bin", ".herdr-deck-*"))
	if len(left) != 0 {
		t.Errorf("temporary files left: %q", left)
	}
}

func TestApplyLocalRefuses(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name    string
		change  func(*fakeExec)
		wantErr string
	}{
		{"other branch", func(f *fakeExec) { f.out[root+"|git symbolic-ref --quiet --short HEAD"] = "feature\n" }, "on branch feature, not origin's default branch main"},
		{"detached", func(f *fakeExec) {
			f.fail[root+"|git symbolic-ref --quiet --short HEAD"] = errors.New("exit status 1")
		}, "detached HEAD"},
		{"dirty", func(f *fakeExec) { f.out[root+"|git status --porcelain --untracked-files=no"] = " M main.go\n" }, "uncommitted changes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := localExec(root)
			tt.change(fe)
			u := Updater{Exec: fe, Running: Running{Commit: oldCommit}, Out: &bytes.Buffer{}}
			st, err := u.Check(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			err = u.Apply(context.Background(), st)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Apply err = %v, want one containing %q", err, tt.wantErr)
			}
			if fe.ran("run ") {
				t.Errorf("a refused update ran %q", fe.calls)
			}
		})
	}
}

func TestCheckerCachesAnHour(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cache", "update.json")
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	fe := &fakeExec{out: map[string]string{listCmd: githubList, tagsCmd: tags}}
	c := Checker{Updater: Updater{Exec: fe, Running: Running{Version: "0.1.0"}}, CachePath: cache, Now: func() time.Time { return now }}
	remoteCalls := func() int {
		n := 0
		for _, call := range fe.calls {
			if strings.Contains(call, "ls-remote") {
				n++
			}
		}
		return n
	}

	st, err := c.Check(context.Background())
	if err != nil || !st.Newer || st.Label() != "v0.10.0" {
		t.Fatalf("Check = %+v, %v; want v0.10.0 newer", st, err)
	}
	now = now.Add(59 * time.Minute)
	// Another deck, same cache: no second ls-remote.
	fe.out[tagsCmd] = tags + "6666666\trefs/tags/v0.11.0\n"
	st, _ = c.Check(context.Background())
	if remoteCalls() != 1 || st.Label() != "v0.10.0" {
		t.Errorf("within the hour: %d ls-remote calls, label %q; want 1, v0.10.0", remoteCalls(), st.Label())
	}
	now = now.Add(2 * time.Minute)
	st, _ = c.Check(context.Background())
	if remoteCalls() != 2 || st.Label() != "v0.11.0" {
		t.Errorf("after the hour: %d ls-remote calls, label %q; want 2, v0.11.0", remoteCalls(), st.Label())
	}

	// The cache keeps the remote answer, not the verdict: after an update
	// the same answer is no longer newer.
	c.Updater.Running.Version = "0.11.0"
	if st, _ = c.Check(context.Background()); st.Newer {
		t.Error("the running version equals the cached tag, but Check says newer")
	}
}

func TestCheckerFailureIsNotCached(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "update.json")
	fe := &fakeExec{
		out:  map[string]string{listCmd: githubList},
		fail: map[string]error{tagsCmd: errors.New("could not resolve host")},
	}
	c := Checker{Updater: Updater{Exec: fe}, CachePath: cache}
	if _, err := c.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "could not resolve host") {
		t.Errorf("err = %v, want the ls-remote failure", err)
	}
	if _, err := os.Stat(cache); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a failed check wrote the cache (%v)", err)
	}
}

func TestCheckerIgnoresOtherSourcesCache(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "update.json")
	if err := os.WriteFile(cache, []byte(`{"source":"github:other/repo","checked_at":"2026-10-03T12:00:00Z","remote":{"tag":"v9.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fe := &fakeExec{out: map[string]string{listCmd: githubList, tagsCmd: tags}}
	now := time.Date(2026, 10, 3, 12, 1, 0, 0, time.UTC)
	c := Checker{Updater: Updater{Exec: fe, Running: Running{Version: "0.1.0"}}, CachePath: cache, Now: func() time.Time { return now }}
	st, err := c.Check(context.Background())
	if err != nil || st.Label() != "v0.10.0" {
		t.Errorf("Check = %q, %v; want v0.10.0 from the remote", st.Label(), err)
	}
}

func TestSummary(t *testing.T) {
	gh := Install{Kind: GitHub, Repo: "acme/herdr-deck"}
	local := Install{Kind: Local, Root: "/src/herdr-deck"}
	tests := []struct {
		st   Status
		want string
	}{
		{Status{Install: gh, Current: "0.1.0", Remote: Remote{Tag: "v0.2.0"}, Newer: true}, "herdr-deck 0.1.0 is installed from acme/herdr-deck; v0.2.0 is available: run `herdr-deck update`."},
		{Status{Install: gh, Current: "0.2.0", Remote: Remote{Tag: "v0.2.0"}}, "herdr-deck 0.2.0 is up to date (newest release v0.2.0)."},
		{Status{Install: local, Current: oldCommit, Remote: Remote{Branch: "main", Head: newCommit}, Newer: true}, "herdr-deck 1234567 runs from the linked checkout /src/herdr-deck; origin/main is at fedcba0: run `herdr-deck update` to pull and rebuild."},
		{Status{Install: local, Current: newCommit, Remote: Remote{Branch: "main", Head: newCommit}}, "herdr-deck fedcba0 is up to date with origin/main."},
	}
	for _, tt := range tests {
		if got := tt.st.Summary(); got != tt.want {
			t.Errorf("Summary = %q\nwant      %q", got, tt.want)
		}
	}
}

func notInstalled(string) bool { return false }

func TestOtherBinaryAsksTheInstalledOne(t *testing.T) {
	// `herdr-deck update` typed in a shell runs a newer build from PATH;
	// the installed plugin is still 0.1.0.
	fe := &fakeExec{out: map[string]string{
		listCmd: githubList,
		tagsCmd: tags,
		"/plugins/github/herdr-deck-0123|" + filepath.Join("/plugins/github/herdr-deck-0123", "bin", "herdr-deck") + " --version": "herdr-deck 0.1.0 (aaaaaaaaaaaa)\n",
	}}
	u := Updater{Exec: fe, Running: Running{Version: "0.10.0"}, IsInstalled: notInstalled, Out: &bytes.Buffer{}}
	st, err := u.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Current != "0.1.0" || !st.Newer {
		t.Errorf("Check = current %q newer %v; want the installed 0.1.0, newer", st.Current, st.Newer)
	}

	// No installed binary to ask: the manifest's version.
	delete(fe.out, "/plugins/github/herdr-deck-0123|"+filepath.Join("/plugins/github/herdr-deck-0123", "bin", "herdr-deck")+" --version")
	if st, _ = u.Check(context.Background()); st.Current != "0.1.0" || !st.Newer {
		t.Errorf("without a binary: current %q newer %v; want 0.1.0, newer", st.Current, st.Newer)
	}
}

func TestOtherBinaryLinkedCheckout(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin", "herdr-deck")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("old deck"), 0o755); err != nil {
		t.Fatal(err)
	}
	fe := localExec(root)
	fe.out[root+"|"+bin+" --version"] = "herdr-deck v0.1.0 (" + oldCommit[:12] + "-dirty)\n"
	fe.out[root+"|git rev-parse --verify --quiet "+oldCommit[:12]+"^{commit}"] = oldCommit + "\n"
	// The build on PATH is at origin's head; the installed one is behind.
	u := Updater{Exec: fe, Running: Running{Commit: newCommit}, IsInstalled: notInstalled, Out: &bytes.Buffer{}}
	st, err := u.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Current != oldCommit || !st.Newer {
		t.Fatalf("Check = current %q newer %v; want the installed %q, newer", st.Current, st.Newer, oldCommit)
	}
	if err := u.Apply(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	if !fe.ran("run "+root+"|git pull") || !fe.ran("run "+root+"|go build") {
		t.Errorf("calls = %q, want a pull and a build", fe.calls)
	}

	// Installed binary already at HEAD: no rebuild, so no deck restarts.
	fe = localExec(root)
	fe.out[root+"|"+bin+" --version"] = "herdr-deck v0.1.0 (" + newCommit[:12] + ")\n"
	fe.out[root+"|git rev-parse --verify --quiet "+newCommit[:12]+"^{commit}"] = newCommit + "\n"
	u = Updater{Exec: fe, Running: Running{Commit: oldCommit}, IsInstalled: notInstalled, Out: &bytes.Buffer{}}
	st, _ = u.Check(context.Background())
	if err := u.Apply(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	if st.Newer || fe.ran("run ") {
		t.Errorf("up-to-date install: newer %v, calls %q; want no pull or build", st.Newer, fe.calls)
	}
}

func TestParseVersionLine(t *testing.T) {
	tests := []struct {
		in   string
		want Running
	}{
		{"herdr-deck 0.1.0 (abcdef123456)\n", Running{Version: "0.1.0", Commit: "abcdef123456"}},
		{"herdr-deck v0.1.0-2-gabc-dirty (abcdef123456-dirty)", Running{Version: "v0.1.0-2-gabc-dirty", Commit: "abcdef123456"}},
		{"herdr-deck (devel)", Running{}},
		{"something else 1.0", Running{}},
	}
	for _, tt := range tests {
		if got := parseVersionLine(tt.in); got != tt.want {
			t.Errorf("parseVersionLine(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

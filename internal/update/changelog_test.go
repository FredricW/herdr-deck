package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChangelogGitHub(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/acme/herdr-deck/v0.2.0/CHANGELOG.md" || r.URL.Path == "/acme/tools/v0.2.0/deck/CHANGELOG.md" {
			_, _ = w.Write([]byte("## [0.2.0] - 2026-10-09\n"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	u := Updater{Exec: &fakeExec{}, RawBase: srv.URL + "/", HTTP: srv.Client()}
	st := Status{Install: Install{Kind: GitHub, Repo: "acme/herdr-deck"}, Remote: Remote{Tag: "v0.2.0"}}
	got, err := u.Changelog(context.Background(), st)
	if err != nil || !strings.HasPrefix(string(got), "## [0.2.0]") {
		t.Fatalf("Changelog = %q, %v", got, err)
	}
	st.Install.Repo = "acme/tools/deck"
	if _, err := u.Changelog(context.Background(), st); err != nil {
		t.Errorf("subdir: %v", err)
	}
	st.Remote.Tag = "v0.3.0"
	if _, err := u.Changelog(context.Background(), st); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("a missing file should fail with its status: %v", err)
	}
	if want := []string{"/acme/herdr-deck/v0.2.0/CHANGELOG.md", "/acme/tools/v0.2.0/deck/CHANGELOG.md", "/acme/tools/v0.3.0/deck/CHANGELOG.md"}; strings.Join(paths, " ") != strings.Join(want, " ") {
		t.Errorf("fetched %q", paths)
	}
}

func TestChangelogLocal(t *testing.T) {
	root := "/src/herdr-deck"
	st := Status{Install: Install{Kind: Local, Root: root}, Remote: Remote{Branch: "main", Head: newCommit}}
	fe := &fakeExec{out: map[string]string{
		root + "|git show " + newCommit + ":CHANGELOG.md": "head's\n",
	}}
	u := Updater{Exec: fe}
	if got, err := u.Changelog(context.Background(), st); err != nil || string(got) != "head's\n" {
		t.Fatalf("Changelog = %q, %v", got, err)
	}

	// Origin's commit is not fetched yet: the remote branch as last fetched.
	fe = &fakeExec{
		out:  map[string]string{root + "|git show refs/remotes/origin/main:CHANGELOG.md": "branch's\n"},
		fail: map[string]error{root + "|git show " + newCommit + ":CHANGELOG.md": errors.New("bad object")},
	}
	u.Exec = fe
	if got, err := u.Changelog(context.Background(), st); err != nil || string(got) != "branch's\n" {
		t.Fatalf("fallback Changelog = %q, %v", got, err)
	}

	u.Exec = &fakeExec{}
	if _, err := u.Changelog(context.Background(), st); err == nil {
		t.Error("no file anywhere should fail")
	}
}

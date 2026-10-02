package project

import (
	"errors"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestSlug(t *testing.T) {
	root := "/home/u/.herdr-projects"
	tests := []struct {
		name string
		flag string
		env  map[string]string
		cwd  string
		want string
		err  error
	}{
		{name: "flag wins", flag: "a", env: map[string]string{EnvProject: "b"}, cwd: root + "/c", want: "a"},
		{name: "env before cwd", env: map[string]string{EnvProject: "b"}, cwd: root + "/c", want: "b"},
		{name: "cwd is project folder", cwd: root + "/admin-rebuild", want: "admin-rebuild"},
		{name: "cwd below project folder", cwd: root + "/admin-rebuild/threads", want: "admin-rebuild"},
		{name: "cwd is root", cwd: root, err: ErrNoProject},
		{name: "cwd is hidden folder", cwd: root + "/.cache", err: ErrNoProject},
		{name: "cwd outside root", cwd: "/home/u/Dev", err: ErrNoProject},
		{name: "cwd is sibling with root prefix", cwd: "/home/u/.herdr-projects-old/x", err: ErrNoProject},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Slug(tt.flag, env(tt.env), tt.cwd, root)
			if !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v", err, tt.err)
			}
			if got != tt.want {
				t.Fatalf("slug = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRootFromEnv(t *testing.T) {
	got, err := Root(env(map[string]string{"HERDR_PROJECTS_ROOT": "/x"}))
	if err != nil || got != "/x" {
		t.Fatalf("Root = %q, %v", got, err)
	}
}

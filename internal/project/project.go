// Package project works out which herdr-projects project the deck shows.
package project

import (
	"errors"
	"path/filepath"
	"strings"
)

// EnvProject names the environment variable that selects the project when
// --project is not given.
const EnvProject = "HERDR_DECK_PROJECT"

// ErrNoProject means no project was named and the working directory is not
// inside a project folder.
var ErrNoProject = errors.New("no project: pass --project <slug>, set " + EnvProject + ", or run inside <projects root>/<slug>")

// Slug picks the project slug, in order: the --project flag, $HERDR_DECK_PROJECT,
// then the first path element of cwd below root (so a pane started in
// ~/.herdr-projects/<slug> or any folder under it finds its project).
func Slug(flag string, getenv func(string) string, cwd, root string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if s := getenv(EnvProject); s != "" {
		return s, nil
	}
	if root != "" && cwd != "" {
		rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(cwd))
		if err == nil && rel != "." && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
			first := strings.Split(rel, string(filepath.Separator))[0]
			if first != "" && !strings.HasPrefix(first, ".") {
				return first, nil
			}
		}
	}
	return "", ErrNoProject
}

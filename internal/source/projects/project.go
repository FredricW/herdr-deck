package projects

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/FredricW/herdr-deck/internal/deck"
)

type projectFile struct {
	Name  string `toml:"name"`
	Goal  string `toml:"goal"`
	Repos []struct {
		Path string `toml:"path"`
	} `toml:"repos"`
}

// readProject reads PROJECT.md's front matter. Without it the project still
// has its slug.
func (r Reader) readProject(note func(string, ...any)) deck.Project {
	p := deck.Project{Slug: r.Slug}
	data, err := os.ReadFile(filepath.Join(r.Dir(), "PROJECT.md"))
	if err != nil {
		note("PROJECT.md: %v", short(err))
		return p
	}
	var f projectFile
	if _, err := frontMatter(string(data), &f); err != nil {
		note("PROJECT.md: %v", err)
		return p
	}
	p.Name, p.Goal = f.Name, f.Goal
	for _, repo := range f.Repos {
		if repo.Path != "" {
			p.Repos = append(p.Repos, repo.Path)
		}
	}
	return p
}

// short drops the path from a file error; the note already names the file.
func short(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

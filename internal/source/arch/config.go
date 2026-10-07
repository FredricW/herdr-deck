package arch

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

// ManifestPath is the shared dev manifest, read at the head commit. Its
// `x-herdr-deck.architecture` block is the deck's own; the manifest's
// other readers ignore x- keys.
const ManifestPath = ".config/dev.json"

// Config is how a repository describes its architecture: its layers, top
// (entry points) to bottom (the core), the folders that hold its source,
// and its language. All of it is optional.
type Config struct {
	// Language is LangGo or LangTS; "" picks the one with more files.
	Language string
	// Roots limit the graph to the files under these folders; none takes
	// the whole repository.
	Roots []string
	// Layers: a package may import its own layer and any below it.
	Layers []Layer
	// Tests counts test files (a deck setting, not the manifest's).
	Tests bool
}

// Layer is one declared layer: the package folders it holds, as globs
// ("internal/ui/**", "cmd/*"), and whether callers above must go through
// it.
type Layer struct {
	Name   string   `json:"name"`
	Paths  []string `json:"paths"`
	Closed bool     `json:"closed,omitempty"`
}

// ParseConfig reads the `x-herdr-deck.architecture` block of a dev
// manifest. A manifest without one gives an empty Config; a block that
// does not read as one is an error.
func ParseConfig(manifest []byte) (Config, error) {
	var raw struct {
		Deck struct {
			Architecture *struct {
				Language string   `json:"language"`
				Roots    []string `json:"roots"`
				Layers   []Layer  `json:"layers"`
			} `json:"architecture"`
		} `json:"x-herdr-deck"`
	}
	if err := json.Unmarshal(manifest, &raw); err != nil {
		return Config{}, err
	}
	a := raw.Deck.Architecture
	if a == nil {
		return Config{}, nil
	}
	c := Config{Language: strings.ToLower(a.Language), Layers: a.Layers}
	switch c.Language {
	case "", LangGo, LangTS:
	case "typescript", "javascript", "js":
		c.Language = LangTS
	default:
		return Config{}, fmt.Errorf("x-herdr-deck.architecture.language: %q is not go or ts", a.Language)
	}
	for _, r := range a.Roots {
		if r = strings.Trim(path.Clean(r), "/"); r != "" && r != "." {
			c.Roots = append(c.Roots, r)
		}
	}
	for i, l := range c.Layers {
		if l.Name == "" {
			return Config{}, fmt.Errorf("x-herdr-deck.architecture.layers[%d]: no name", i)
		}
	}
	return c, nil
}

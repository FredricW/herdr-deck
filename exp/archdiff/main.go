// Command archdiff is a throwaway prototype for the drawer's Architecture
// tab (docs/research/architecture-tab.md): it reads two commits of a Go
// repository straight from git, without a checkout, builds both package
// graphs, and prints what the change did to the system's shape as the
// tab's text at a given width.
//
//	go run ./exp/archdiff -repo . -base 4354826 -head 9fc47b3 -width 80
//
// It is not part of the deck and is not built into it.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"
	"time"
)

func main() {
	repo := flag.String("repo", ".", "repository")
	base := flag.String("base", "", "base commit (default: merge-base of origin/main and head)")
	head := flag.String("head", "HEAD", "head commit")
	width := flag.Int("width", 80, "columns")
	layersFile := flag.String("layers", "", "layers config (JSON: {\"layers\":[{\"name\",\"paths\",\"closed\"}]}); without it lanes are inferred")
	view := flag.String("view", "all", "first, lanes, matrix, detail, all, or canvas")
	edges := flag.String("edges", "ports", "canvas edges: ports, lines or focus")
	selected := flag.String("select", "", "canvas focus box (default: the riskiest edge's source)")
	color := flag.Bool("color", false, "canvas in ANSI colours")
	tests := flag.Bool("tests", false, "count test files")
	language := flag.String("lang", "go", "go, or ts for TypeScript and JavaScript")
	src := flag.String("src", "", "ts: only the files under this folder (e.g. frontend/src)")
	timing := flag.Bool("time", false, "print timings to stderr")
	flag.Parse()

	c := canvasOpts{mode: edgeMode(*edges), selected: *selected, color: *color, lang: lang{*language, strings.TrimSuffix(*src, "/")}}
	if err := run(*repo, *base, *head, *width, *layersFile, *view, *tests, *timing, c); err != nil {
		fmt.Fprintln(os.Stderr, "archdiff:", err)
		os.Exit(1)
	}
}

type canvasOpts struct {
	mode     edgeMode
	selected string
	color    bool
	lang     lang
}

func run(repo, base, head string, width int, layersFile, view string, tests, timing bool, co canvasOpts) error {
	t0 := time.Now()
	resolve := func(rev string) (string, error) {
		out, err := git(repo, "rev-parse", "--verify", rev+"^{commit}")
		return strings.TrimSpace(string(out)), err
	}
	h, err := resolve(head)
	if err != nil {
		return err
	}
	if base == "" {
		out, err := git(repo, "merge-base", "origin/main", h)
		if err != nil {
			return err
		}
		base = strings.TrimSpace(string(out))
	}
	b, err := resolve(base)
	if err != nil {
		return err
	}

	var cfg *layerConfig
	if layersFile != "" {
		data, err := os.ReadFile(layersFile)
		if err != nil {
			return err
		}
		cfg = &layerConfig{}
		if err := json.Unmarshal(data, cfg); err != nil {
			return fmt.Errorf("%s: %w", layersFile, err)
		}
	}

	if co.lang.src != "" {
		labelPrefix = co.lang.src + "/"
	}
	bl, err := openBlobs(repo)
	if err != nil {
		return err
	}
	defer bl.close()
	c := cache{}
	bs, err := build(repo, b, bl, c, tests, co.lang)
	if err != nil {
		return err
	}
	t1 := time.Now()
	parsedBase := len(c)
	hs, err := build(repo, h, bl, c, tests, co.lang)
	if err != nil {
		return err
	}
	t2 := time.Now()
	cs, err := diffFiles(repo, b, h)
	if err != nil {
		return err
	}
	r := analyse(bs, hs, cs, cfg)
	t3 := time.Now()

	o := &out{w: width}
	o.add("%s", pad(width, fmt.Sprintf(" %d packages · base %.7s → head %.7s", len(hs.pkgs), b, h), "f first · l lanes · m matrix "))
	r.summary(o)
	o.add("")
	if view == "first" || view == "all" {
		r.first(o, 12)
		o.add("")
	}
	if view == "lanes" || view == "all" {
		r.lanesView(o)
		o.add("")
	}
	if view == "matrix" || view == "all" {
		r.matrixView(o)
		o.add("")
	}
	if view == "detail" || view == "all" {
		r.detail(o)
	}
	if view == "canvas" {
		name := "repo"
		for _, m := range hs.modules {
			if m.dir == "." && m.path != "" {
				name = path.Base(m.path) // the Go module
			}
		}
		if name == "repo" {
			if top, err := git(repo, "rev-parse", "--show-toplevel"); err == nil {
				name = path.Base(strings.TrimSpace(string(top)))
			}
		}
		r.canvasView(o, name, co.mode, co.selected, co.color)
	}
	for _, l := range o.lines {
		fmt.Println(strings.TrimRight(l, " "))
	}
	if timing {
		fmt.Fprintf(os.Stderr, "base graph %v (%d files parsed) · head graph %v (%d more parsed) · diff+analysis %v\n",
			t1.Sub(t0).Round(time.Millisecond), parsedBase, t2.Sub(t1).Round(time.Millisecond), len(c)-parsedBase, t3.Sub(t2).Round(time.Millisecond))
	}
	return nil
}

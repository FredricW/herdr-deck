// Command callstack is a throwaway prototype for the deck's Call stack tab:
// it loads a Go module at two revisions, diffs the call trees of the entry
// points a change reaches, and prints the tab's text at the given widths.
//
//	go run . -repo ../.. -base <rev> -head <rev> -width 80
//
// It lives in its own module so golang.org/x/tools stays out of the deck.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	repo := flag.String("repo", ".", "git repository")
	base := flag.String("base", "", "base revision")
	head := flag.String("head", "HEAD", "head revision")
	widths := flag.String("width", "80", "comma-separated widths to render")
	flows := flag.Int("flows", 3, "flow trees to show")
	limit := flag.Int("top", 6, "rows in the look-here-first list")
	work := flag.String("work", "", "scratch folder for the two trees (default: a temp dir)")
	unknown := flag.Bool("unknown", false, "list external calls the rule table can't classify")
	flag.Parse()
	if *base == "" {
		fmt.Fprintln(os.Stderr, "need -base")
		os.Exit(2)
	}
	if err := run(*repo, *base, *head, *widths, *work, *flows, *limit, *unknown); err != nil {
		fmt.Fprintln(os.Stderr, "callstack:", err)
		os.Exit(1)
	}
}

func run(repo, base, head, widths, work string, flows, limit int, unknown bool) error {
	if work == "" {
		d, err := os.MkdirTemp("", "callstack-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(d)
		work = d
	}
	t0 := time.Now()
	bdir, hdir := filepath.Join(work, "base"), filepath.Join(work, "head")
	if err := materialize(repo, base, bdir); err != nil {
		return err
	}
	if err := materialize(repo, head, hdir); err != nil {
		return err
	}
	t1 := time.Now()
	cfg := defaultEntries()
	bp, err := load(bdir, cfg)
	if err != nil {
		return err
	}
	t2 := time.Now()
	hp, err := load(hdir, cfg)
	if err != nil {
		return err
	}
	t3 := time.Now()
	a := analyze(bp, hp)
	t4 := time.Now()
	fmt.Fprintf(os.Stderr, "archive %v · load base %v (%d funcs) · load head %v (%d funcs) · analyze %v\n",
		t1.Sub(t0).Round(time.Millisecond), t2.Sub(t1).Round(time.Millisecond), len(bp.Funcs),
		t3.Sub(t2).Round(time.Millisecond), len(hp.Funcs), t4.Sub(t3).Round(time.Millisecond))

	for _, ws := range strings.Split(widths, ",") {
		w, err := strconv.Atoi(strings.TrimSpace(ws))
		if err != nil {
			return err
		}
		r := &renderer{a: a, w: w, maxDepth: 5}
		r.flowCount = len(r.flows())
		r.lookHere(limit)
		shown, quiet := 0, 0
		for _, e := range r.flows() {
			if shown == flows {
				break
			}
			if r.flow(e) {
				shown++
			} else {
				quiet++
			}
		}
		if rest := len(r.flows()) - shown - quiet; rest > 0 || quiet > 0 {
			r.row(fmt.Sprintf(" ⋯ %d more flows · %d reach it only through shared frames", rest, quiet), "")
		}
		r.sharedFrames(2)
		for _, f := range a.Findings {
			if f.Kind != "removed" && !f.Refactor && len(f.Entries) > 1 {
				r.blank()
				r.blast(f.ID, 0, map[string]bool{})
				break
			}
		}
		fmt.Printf("── width %d %s\n", w, strings.Repeat("─", max(0, w-12-len(ws))))
		for _, l := range r.out {
			fmt.Println(l)
		}
		fmt.Println()
	}
	if unknown {
		fmt.Println("── external calls the rule table doesn't know (changed frames):")
		for _, u := range unknownExts(hp, keys(a.Changed)) {
			fmt.Println("  ", u)
		}
	}
	return nil
}

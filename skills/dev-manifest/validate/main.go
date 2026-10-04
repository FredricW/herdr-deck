// Command validate checks dev manifests (.config/dev.json, see
// docs/dev-manifest.md) against schema/v1, and against the spec's rules
// that a JSON Schema cannot express: variables, references between ports,
// services and groups, and dependency cycles. It reads files only; it never
// runs anything from the repository.
//
// With -dry (the default) it also checks, as warnings, that the folders,
// scripts and task-runner targets the manifest points at exist (the spec's
// dangling drift, section 12.4). The repository is the folder holding
// .config/, or -root.
//
//	go run ./skills/dev-manifest/validate [-root <repo>] [-dry=false] <file>...
//
// It prints one line per problem and "<file>: ok" for a valid file, and
// exits 1 when any file has an error. Warnings do not change the exit code.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	root := flag.String("root", "", "the repository the manifests describe (default: the folder holding .config/)")
	dry := flag.Bool("dry", true, "also check that folders, scripts and task-runner targets exist")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "usage: validate [-root <repo>] [-dry=false] <file>...")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	failed := false
	for _, file := range flag.Args() {
		r := *root
		if r == "" {
			r = repoOf(file)
		}
		if !*dry {
			r = ""
		}
		res := Validate(file, r)
		for _, e := range res.Errors {
			fmt.Printf("%s: error: %s\n", file, e)
		}
		for _, w := range res.Warnings {
			fmt.Printf("%s: warning: %s\n", file, w)
		}
		if len(res.Errors) > 0 {
			failed = true
		} else {
			fmt.Printf("%s: ok\n", file)
		}
	}
	if failed {
		os.Exit(1)
	}
}

// repoOf returns the repository of a manifest at <repo>/.config/dev.json,
// else the file's folder.
func repoOf(file string) string {
	dir := filepath.Dir(file)
	if filepath.Base(dir) == ".config" {
		return filepath.Dir(dir)
	}
	return dir
}

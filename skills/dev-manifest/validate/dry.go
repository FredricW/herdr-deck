package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// dryChecker looks for the spec's dangling drift (section 12.4): folders,
// scripts and task-runner targets the manifest names that do not exist in
// root. It reads files only. A command it cannot read is skipped.
type dryChecker struct {
	m        map[string]any
	services map[string]map[string]any
	root     string
	warns    []string
}

func (d *dryChecker) warnf(id, format string, args ...any) {
	if d.silenced("dangling:" + id) {
		return
	}
	d.warns = append(d.warns, fmt.Sprintf("dangling:%s: %s", id, fmt.Sprintf(format, args...)))
}

// silenced reports whether the manifest's detect settings turn a drift ID
// off (spec section 12.4): drift false, or an ignore entry where * matches
// any characters.
func (d *dryChecker) silenced(id string) bool {
	det := obj(d.m["detect"])
	if drift, ok := det["drift"].(bool); ok && !drift {
		return true
	}
	ignore, _ := det["ignore"].([]any)
	for _, x := range ignore {
		pat, _ := x.(string)
		parts := strings.Split(pat, "*")
		for i, p := range parts {
			parts[i] = regexp.QuoteMeta(p)
		}
		if regexp.MustCompile("^" + strings.Join(parts, ".*") + "$").MatchString(id) {
			return true
		}
	}
	return false
}

// resolve expands $WORKTREE and $REPO to the root and makes a relative path
// absolute. ok is false when other variables are left.
func (d *dryChecker) resolve(base, p string) (string, bool) {
	for _, v := range []string{"${WORKTREE}", "$WORKTREE", "${REPO}", "$REPO"} {
		p = strings.ReplaceAll(p, v, d.root)
	}
	if strings.Contains(p, "$") {
		return "", false
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	return p, true
}

func (d *dryChecker) check() {
	dirs := map[string]string{}
	for _, name := range sorted(d.services) {
		s := d.services[name]
		dir := d.dir("services."+name+".dir", d.root, s["dir"])
		dirs[name] = dir
		d.command("services."+name+".run", dir, s["run"])
		d.command("services."+name+".stop", dir, s["stop"])
	}
	cmds := obj(d.m["commands"])
	for _, name := range sorted(cmds) {
		v := cmds[name]
		if !isPerService(v) {
			d.command("commands."+name, d.root, v)
			continue
		}
		entries := obj(v)
		for _, key := range sorted(entries) {
			base := d.root
			if dir, ok := dirs[key]; ok {
				base = dir
			}
			d.command("commands."+name+"."+key, base, entries[key])
		}
	}
}

// dir returns the folder a dir field names (base when there is none), and
// warns when it does not exist.
func (d *dryChecker) dir(id, base string, v any) string {
	s, ok := v.(string)
	if !ok {
		return base
	}
	p, ok := d.resolve(d.root, s)
	if !ok {
		return ""
	}
	if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
		d.warnf(id, "no folder %s", s)
		return ""
	}
	return p
}

// command checks a run value: a string, an argv or an object with run and dir.
func (d *dryChecker) command(id string, dir string, v any) {
	if m, ok := v.(map[string]any); ok {
		if _, has := m["dir"]; has {
			dir = d.dir(id+".dir", d.root, m["dir"])
		}
		v = m["run"]
	}
	if dir == "" {
		return // its folder is unknown or already reported
	}
	var argv []string
	switch v := v.(type) {
	case string:
		argv = shellWords(v)
	case []any:
		for _, x := range v {
			s, _ := x.(string)
			argv = append(argv, s)
		}
	}
	if len(argv) == 0 || strings.ContainsAny(argv[0], "$\"'") {
		return
	}
	if strings.Contains(argv[0], "/") {
		p, ok := d.resolve(dir, argv[0])
		if !ok {
			return
		}
		if _, err := os.Stat(p); err != nil {
			d.warnf(id, "no file %s", argv[0])
		}
		return
	}
	d.target(id, dir, argv)
}

// shellWords splits the first simple command of a shell string into words.
// It returns nil when the string uses anything it does not recognise.
func shellWords(s string) []string {
	var words []string
	for _, w := range strings.Fields(s) {
		if w == "&&" || w == "||" || w == ";" || w == "|" {
			break
		}
		if strings.ContainsAny(w, "\"'`\\(){}<>") {
			return nil
		}
		words = append(words, strings.TrimSuffix(w, ";"))
		if strings.HasSuffix(w, ";") {
			break
		}
	}
	return words
}

// target checks a task-runner call (npm/pnpm/yarn/bun run x, npm test,
// npm start, just x, make x, task x, mise run x) against the runner's file
// in dir.
func (d *dryChecker) target(id, dir string, argv []string) {
	runner, rest := argv[0], argv[1:]
	switch runner {
	case "npm", "pnpm", "yarn", "bun":
		var name string
		switch {
		case len(rest) >= 2 && (rest[0] == "run" || rest[0] == "run-script"):
			name = rest[1]
		case len(rest) >= 1 && runner == "npm" && (rest[0] == "test" || rest[0] == "start"):
			name = rest[0]
		default:
			return
		}
		// Flags before -- may pick another package (-w, --filter, --prefix),
		// and bun runs files as well as scripts.
		for _, a := range rest {
			if a == "--" {
				break
			}
			if strings.HasPrefix(a, "-") {
				return
			}
		}
		if strings.ContainsAny(name, "./") {
			return
		}
		scripts, err := packageScripts(filepath.Join(dir, "package.json"))
		if err != nil {
			d.warnf(id, "%s %s: %v", runner, name, err)
			return
		}
		if _, ok := scripts[name]; !ok {
			d.warnf(id, "package.json has no script %q", name)
		}
	case "just", "make", "task", "mise":
		if runner == "mise" {
			if len(rest) < 2 || rest[0] != "run" {
				return
			}
			rest = rest[1:]
		}
		if len(rest) == 0 || strings.HasPrefix(rest[0], "-") || strings.Contains(rest[0], "=") {
			return
		}
		names, file, ok := runnerTargets(runner, dir, d.root)
		if !ok {
			if runner != "mise" {
				d.warnf(id, "%s %s: no %s file", runner, rest[0], runner)
			}
			return
		}
		if names != nil && !names[rest[0]] {
			d.warnf(id, "%s has no target %q", file, rest[0])
		}
	}
}

func packageScripts(file string) (map[string]any, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("no package.json")
	}
	var pkg struct {
		Scripts map[string]any `json:"scripts"`
	}
	if err := json.Unmarshal(b, &pkg); err != nil {
		return nil, fmt.Errorf("package.json: %v", err)
	}
	return pkg.Scripts, nil
}

var (
	makeTargetRe = regexp.MustCompile(`(?m)^([A-Za-z0-9][A-Za-z0-9_. -]*?)\s*:([^=]|$)`)
	justAliasRe  = regexp.MustCompile(`(?m)^alias\s+([A-Za-z0-9_-]+)\s*:=`)
	justRecipeRe = regexp.MustCompile(`(?m)^@?([A-Za-z0-9_-]+)( [^:]*)?:([^=]|$)`)
	taskKeyRe    = regexp.MustCompile(`(?m)^  ([A-Za-z0-9_:.-]+):`)
)

// runnerTargets returns the targets a task runner's file defines, and the
// file's path relative to root. names is nil when the file exists but the
// runner has other ways to define tasks (mise's task folders). ok is false
// without a file. just, task and mise also look in dir's parents, up to root.
func runnerTargets(runner, dir, root string) (names map[string]bool, file string, ok bool) {
	for {
		names, file, ok = runnerFile(runner, dir)
		if ok || runner == "make" || dir == root || !strings.HasPrefix(dir, root) {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if ok {
		if rel, err := filepath.Rel(root, filepath.Join(dir, file)); err == nil {
			file = rel
		}
	}
	return names, file, ok
}

func runnerFile(runner, dir string) (names map[string]bool, file string, ok bool) {
	candidates := map[string][]string{
		"make": {"GNUmakefile", "makefile", "Makefile"},
		"just": {"justfile", ".justfile", "Justfile"},
		"task": {"Taskfile.yml", "taskfile.yml", "Taskfile.yaml", "taskfile.yaml", "Taskfile.dist.yml", "taskfile.dist.yml", "Taskfile.dist.yaml", "taskfile.dist.yaml"},
		"mise": {"mise.toml", ".mise.toml"},
	}[runner]
	// Compare names exactly: macOS folders ignore case, and make reads
	// GNUmakefile, makefile and Makefile in that order.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, "", false
	}
	present := map[string]bool{}
	for _, e := range entries {
		present[e.Name()] = true
	}
	for _, f := range candidates {
		if !present[f] {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		names := map[string]bool{}
		switch runner {
		case "make":
			for _, m := range makeTargetRe.FindAllStringSubmatch(string(b), -1) {
				for _, n := range strings.Fields(m[1]) {
					names[n] = true
				}
			}
		case "just":
			for _, m := range justRecipeRe.FindAllStringSubmatch(string(b), -1) {
				names[m[1]] = true
			}
			for _, m := range justAliasRe.FindAllStringSubmatch(string(b), -1) {
				names[m[1]] = true
			}
		case "task":
			_, after, found := strings.Cut(string(b), "\ntasks:")
			if !found {
				return nil, f, true
			}
			for _, m := range taskKeyRe.FindAllStringSubmatch(after, -1) {
				names[m[1]] = true
			}
		case "mise":
			return nil, f, true
		}
		return names, f, true
	}
	return nil, "", false
}

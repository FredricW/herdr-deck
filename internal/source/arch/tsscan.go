package arch

import (
	"encoding/json"
	"path"
	"regexp"
	"slices"
	"strings"
)

// A small TypeScript and JavaScript reader in plain Go: it blanks out
// comments, finds imports with regular expressions, and resolves them as a
// bundler would: relative paths, tsconfig `paths` aliases, index files;
// bare specifiers are packages. It does not parse TypeScript, so a regex
// literal holding a quote, or an import written inside a template string,
// can fool it; re-exports through barrel files are plain edges.

// tsExts are the source files the reader takes, in resolution order.
var tsExts = []string{".ts", ".tsx", ".js", ".jsx", ".mts", ".cts", ".mjs", ".cjs"}

func isTSFile(p string) bool {
	if strings.HasSuffix(p, ".d.ts") {
		return false
	}
	return slices.Contains(tsExts, path.Ext(p))
}

// skipTS leaves out generated and vendored folders, stories, and tests
// unless asked for.
func skipTS(p string, tests bool) bool {
	for seg := range strings.SplitSeq(path.Dir(p), "/") {
		switch seg {
		case "node_modules", "dist", "build", ".next", "coverage", "out":
			return true
		case "__tests__", "__mocks__":
			if !tests {
				return true
			}
		}
	}
	base := path.Base(p)
	if !tests && (strings.Contains(base, ".test.") || strings.Contains(base, ".spec.")) {
		return true
	}
	return strings.Contains(base, ".stories.")
}

// stripComments blanks // and /* */ comments with spaces, keeping string
// and template literals and every newline, so offsets and lines stay put.
func stripComments(src []byte) []byte {
	out := slices.Clone(src)
	for i := 0; i < len(out); i++ {
		switch c := out[i]; c {
		case '\'', '"', '`':
			for i++; i < len(out) && out[i] != c; i++ {
				if out[i] == '\\' {
					i++
				} else if out[i] == '\n' && c != '`' {
					break // an unterminated string ends at the line
				}
			}
		case '/':
			if i+1 >= len(out) {
				continue
			}
			switch out[i+1] {
			case '/':
				for ; i < len(out) && out[i] != '\n'; i++ {
					out[i] = ' '
				}
			case '*':
				out[i], out[i+1] = ' ', ' '
				for i += 2; i < len(out); i++ {
					if out[i] == '*' && i+1 < len(out) && out[i+1] == '/' {
						out[i], out[i+1] = ' ', ' '
						i++
						break
					}
					if out[i] != '\n' {
						out[i] = ' '
					}
				}
			}
		}
	}
	return out
}

var (
	tsFrom     = regexp.MustCompile(`\b(?:import|export)\s+(?:type\s+)?([\w*{}\s,$]+?)\s+from\s*['"]([^'"\n]+)['"]`)
	tsBare     = regexp.MustCompile(`\bimport\s*['"]([^'"\n]+)['"]`)
	tsDynamic  = regexp.MustCompile(`\bimport\s*\(\s*['"]([^'"\n]+)['"]\s*\)`)
	tsRequire  = regexp.MustCompile(`\brequire\s*\(\s*['"]([^'"\n]+)['"]\s*\)`)
	tsWord     = regexp.MustCompile(`[A-Za-z_$][\w$]*`)
	tsDecl     = regexp.MustCompile(`(?m)^(export\s+)?(?:default\s+)?(?:declare\s+)?(?:async\s+)?(?:abstract\s+)?(function\*?|const|let|var|class|interface|type|enum)\s+([A-Za-z_$][\w$]*)`)
	tsTopLevel = regexp.MustCompile(`(?m)^(?:export|function|async|const|let|var|class|interface|type|enum|abstract|declare)\b`)
	tsEnv      = regexp.MustCompile(`\b(?:process\.env|import\.meta\.env)(?:\.([A-Za-z_]\w*)|\[\s*['"]([^'"]+)['"]\s*\])`)
	tsFetch    = regexp.MustCompile("\\bfetch\\(\\s*['\"`]([^'\"`]+)")
)

// lineAt is the 1-based line of byte offset off.
func lineAt(src []byte, off int) int { return 1 + strings.Count(string(src[:off]), "\n") }

// parseTS reads one TypeScript or JavaScript file. An import's specifier
// goes in imp.path, unresolved; the graph resolves it against the tree.
func parseTS(src []byte) *facts {
	g := &facts{pkg: "ts"}
	code := stripComments(src)
	add := func(spec string, off int, uses string) {
		g.imports = append(g.imports, imp{path: spec, line: lineAt(code, off), uses: uses})
	}
	for _, m := range tsFrom.FindAllSubmatchIndex(code, -1) {
		words := tsWord.FindAllString(string(code[m[2]:m[3]]), -1)
		words = slices.DeleteFunc(words, func(w string) bool { return w == "as" || w == "type" })
		slices.Sort(words)
		add(string(code[m[4]:m[5]]), m[0], strings.Join(slices.Compact(words), " "))
	}
	for _, re := range []*regexp.Regexp{tsBare, tsDynamic, tsRequire} {
		for _, m := range re.FindAllSubmatchIndex(code, -1) {
			add(string(code[m[2]:m[3]]), m[0], "")
		}
	}
	slices.SortStableFunc(g.imports, func(a, b imp) int { return a.line - b.line })
	// Top-level declarations: each runs to the next top-level statement.
	starts := tsTopLevel.FindAllIndex(code, -1)
	for _, m := range tsDecl.FindAllSubmatchIndex(code, -1) {
		end := len(code)
		for _, s := range starts {
			if s[0] > m[0] {
				end = s[0]
				break
			}
		}
		text := string(code[m[0]:end])
		kind, name := string(code[m[4]:m[5]]), string(code[m[6]:m[7]])
		sig, _, _ := strings.Cut(text, "\n")
		at := m[6] - m[0] // the name, not an earlier substring such as "port" in "export"
		blank := text[:at] + "_" + text[at+len(name):]
		g.decls = append(g.decls, decl{
			key: kind + " " + name, exported: m[2] >= 0, sig: strings.TrimSpace(sig),
			hash: hash(text), shape: hash(blank), line: lineAt(code, m[0]),
		})
	}
	for _, m := range tsEnv.FindAllSubmatchIndex(code, -1) {
		name := ""
		switch {
		case m[2] >= 0:
			name = string(code[m[2]:m[3]])
		case m[4] >= 0:
			name = string(code[m[4]:m[5]])
		}
		g.touches = append(g.touches, touch{kind: "env", target: name, line: lineAt(code, m[0])})
	}
	for _, m := range tsFetch.FindAllSubmatchIndex(code, -1) {
		g.touches = append(g.touches, touch{kind: "http", target: host(string(code[m[2]:m[3]])), line: lineAt(code, m[0])})
	}
	return g
}

// tsconfig is the part of a tsconfig.json the resolver needs.
type tsconfig struct {
	dir     string
	baseURL string
	paths   map[string][]string
}

var trailingComma = regexp.MustCompile(`,(\s*[}\]])`)

// parseTSConfig reads a tsconfig.json, which allows comments and trailing
// commas. `extends` and project references are not followed.
func parseTSConfig(dir string, src []byte) *tsconfig {
	var raw struct {
		CompilerOptions struct {
			BaseURL string              `json:"baseUrl"`
			Paths   map[string][]string `json:"paths"`
		} `json:"compilerOptions"`
	}
	clean := trailingComma.ReplaceAll(stripComments(src), []byte("$1"))
	if json.Unmarshal(clean, &raw) != nil {
		return nil
	}
	return &tsconfig{dir: dir, baseURL: raw.CompilerOptions.BaseURL, paths: raw.CompilerOptions.Paths}
}

// parsePackageJSON makes a package.json's dependencies a module's
// requirements, so dependency changes show as they do for go.mod.
func parsePackageJSON(dir string, src []byte) module {
	var raw struct {
		Name            string            `json:"name"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	m := module{dir: dir, requires: map[string]string{}}
	if json.Unmarshal(src, &raw) == nil {
		m.path = raw.Name
		for k, v := range raw.Dependencies {
			m.requires[k] = v
		}
		for k, v := range raw.DevDependencies {
			m.requires[k] = v
		}
	}
	return m
}

// tsResolver turns a specifier into a node: a directory in the tree,
// "ext:<package>", "std:<builtin>", or "" for an asset or something it
// cannot find.
type tsResolver struct {
	files   map[string]bool
	configs []*tsconfig
	// workspaces maps a workspace package's name to its folder, so
	// `@acme/db` resolves into the monorepo rather than to a package.
	workspaces map[string]string
}

var nodeBuiltins = map[string]bool{
	"fs": true, "path": true, "os": true, "http": true, "https": true, "crypto": true,
	"child_process": true, "url": true, "util": true, "stream": true, "events": true,
	"zlib": true, "net": true, "buffer": true, "process": true, "worker_threads": true,
}

func (t *tsResolver) resolve(fromFile, spec string) string {
	if strings.HasPrefix(spec, ".") {
		return t.file(path.Join(path.Dir(fromFile), spec))
	}
	if cfg := t.configFor(fromFile); cfg != nil {
		// As TypeScript does: an exact pattern first, then the longest
		// prefix before the *.
		pats := slices.Sorted(mapKeys(cfg.paths))
		slices.SortStableFunc(pats, func(a, b string) int {
			ea, eb := !strings.Contains(a, "*"), !strings.Contains(b, "*")
			switch {
			case ea != eb && ea:
				return -1
			case ea != eb:
				return 1
			}
			return len(b) - len(a)
		})
		for _, pat := range pats {
			prefix, star := strings.CutSuffix(pat, "*")
			if (star && strings.HasPrefix(spec, prefix)) || (!star && spec == pat) {
				// Only the best pattern counts: its targets, in order, or
				// nothing (an alias that resolves nowhere is no package).
				rest := strings.TrimPrefix(spec, prefix)
				for _, tg := range cfg.paths[pat] {
					p := path.Join(cfg.dir, cfg.baseURL, strings.Replace(tg, "*", rest, 1))
					if n := t.file(p); n != "" {
						return n
					}
				}
				return ""
			}
		}
	}
	name := strings.TrimPrefix(spec, "node:")
	if first, _, _ := strings.Cut(name, "/"); nodeBuiltins[first] {
		return "std:" + name
	}
	parts := strings.Split(spec, "/")
	pkg := parts[0]
	if strings.HasPrefix(spec, "@") && len(parts) > 1 {
		pkg = parts[0] + "/" + parts[1]
	}
	if dir, ok := t.workspaces[pkg]; ok {
		sub := strings.TrimPrefix(strings.TrimPrefix(spec, pkg), "/")
		for _, p := range []string{path.Join(dir, sub), path.Join(dir, "src", sub)} {
			if n := t.file(p); n != "" {
				return n
			}
		}
		return dir
	}
	return "ext:" + pkg
}

// configFor is the tsconfig nearest above a file.
func (t *tsResolver) configFor(file string) *tsconfig {
	var best *tsconfig
	for _, c := range t.configs {
		if c.dir == "." || strings.HasPrefix(file, c.dir+"/") {
			if best == nil || best.dir == "." || len(c.dir) > len(best.dir) {
				best = c
			}
		}
	}
	return best
}

// file finds the source file a path names: itself, with an extension, or
// its folder's index file. It returns that file's directory.
func (t *tsResolver) file(p string) string {
	if isTSFile(p) && t.files[p] {
		return path.Dir(p)
	}
	// ESM and NodeNext write the output's extension: ./client.js for
	// client.ts.
	for js, srcs := range map[string][]string{".js": {".ts", ".tsx"}, ".jsx": {".tsx"}, ".mjs": {".mts"}, ".cjs": {".cts"}} {
		if stem, ok := strings.CutSuffix(p, js); ok {
			for _, ext := range srcs {
				if t.files[stem+ext] {
					return path.Dir(stem + ext)
				}
			}
		}
	}
	for _, ext := range tsExts {
		if t.files[p+ext] {
			return path.Dir(p + ext)
		}
	}
	for _, ext := range tsExts {
		if t.files[p+"/index"+ext] {
			return p
		}
	}
	return "" // an asset (CSS, JSON, SVG) or not in the tree
}

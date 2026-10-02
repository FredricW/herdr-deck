// Package config reads the deck's settings file and resolves each setting
// from, in order: the command-line flag, the environment variable, the
// config file and the built-in default.
//
// The file is $XDG_CONFIG_HOME/herdr-deck/config.toml, else
// ~/.config/herdr-deck/config.toml (on macOS too); --config or
// $HERDR_DECK_CONFIG names another. The deck only reads it. A missing file
// is fine; a file that cannot be parsed, an unknown key or a bad value never
// stops the deck: they come back as problems for the Sources view (or
// stderr) and the setting falls back to the next source.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/launch"
)

// Environment variables. HERDR_PROJECTS_ROOT is herdr-projects' own.
const (
	EnvPath            = "HERDR_DECK_CONFIG"
	EnvLinearWorkspace = deck.EnvLinearWorkspace
	EnvRefreshInterval = "HERDR_DECK_REFRESH_INTERVAL"
	EnvProjectsRoot    = "HERDR_PROJECTS_ROOT"
	EnvEditor          = "HERDR_DECK_EDITOR"
	EnvEditorTerminal  = "HERDR_DECK_EDITOR_TERMINAL"
	EnvDiffTool        = "HERDR_DECK_DIFF_TOOL"
	EnvDiffTerminal    = "HERDR_DECK_DIFF_TERMINAL"
	EnvReuseTabs       = "HERDR_DECK_REUSE_BROWSER_TABS"
	EnvFigmaDesktop    = "HERDR_DECK_FIGMA_DESKTOP"
)

// Default editor and diff tool commands. The diff tool is hunk when it is
// installed, else git diff, which pages itself in a terminal. Both diffs
// include uncommitted changes.
const (
	DefaultEditor    = "code {path}"
	DefaultDiffTool  = "hunk diff {base} -- {file}"
	FallbackDiffTool = "git -C {path} diff --merge-base {base} -- {file}"
)

// Placeholders each command may use.
var (
	EditorPlaceholders = []string{"path"}
	DiffPlaceholders   = []string{"path", "base", "file"}
)

// Refresh interval default and allowed range.
const (
	DefaultRefreshInterval = 5 * time.Second
	MinRefreshInterval     = time.Second
	MaxRefreshInterval     = 10 * time.Minute
)

// File is config.toml as written. Every key is optional: a nil field was not
// set. New keys are added as new optional fields, so an older deck reading
// a newer file only reports the keys it does not know.
type File struct {
	LinearWorkspace *string   `toml:"linear_workspace"`
	RefreshInterval *Duration `toml:"refresh_interval"`
	ProjectsRoot    *string   `toml:"projects_root"`
	Editor          *Program  `toml:"editor"`
	Diff            *Program  `toml:"diff"`
	ReuseTabs       *bool     `toml:"reuse_browser_tabs"`
	FigmaDesktop    *bool     `toml:"figma_desktop"`
}

// Program is a table such as [editor]: a command line with placeholders,
// split into argv without a shell, and whether it is a terminal program
// that opens in a new herdr pane next to the deck.
type Program struct {
	Command  *string `toml:"command"`
	Terminal *bool   `toml:"terminal"`
}

// Duration is a TOML string such as "5s" or "1m30s".
type Duration struct{ time.Duration }

// UnmarshalText parses a Go duration string.
func (d *Duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	if err != nil {
		return fmt.Errorf("%q is not a duration such as \"5s\"", b)
	}
	d.Duration = v
	return nil
}

// Path returns the config file path and whether it was named explicitly:
// flag, else $HERDR_DECK_CONFIG, else $XDG_CONFIG_HOME/herdr-deck/config.toml,
// else ~/.config/herdr-deck/config.toml. A relative $XDG_CONFIG_HOME is
// ignored, as the XDG spec says. It returns "" when no home is known.
func Path(flag string, getenv func(string) string) (path string, named bool) {
	if flag != "" {
		return expandHome(flag, getenv), true
	}
	if p := getenv(EnvPath); p != "" {
		return expandHome(p, getenv), true
	}
	if x := getenv("XDG_CONFIG_HOME"); x != "" && filepath.IsAbs(x) {
		return filepath.Join(x, "herdr-deck", "config.toml"), false
	}
	home := homeDir(getenv)
	if home == "" {
		return "", false
	}
	return filepath.Join(home, ".config", "herdr-deck", "config.toml"), false
}

// LogDir is where the deck keeps the logs of the commands it starts:
// $XDG_STATE_HOME/herdr-deck/logs, else ~/.local/state/herdr-deck/logs. A
// relative $XDG_STATE_HOME is ignored. It returns "" when no home is known.
func LogDir(getenv func(string) string) string {
	if x := getenv("XDG_STATE_HOME"); x != "" && filepath.IsAbs(x) {
		return filepath.Join(x, "herdr-deck", "logs")
	}
	home := homeDir(getenv)
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "state", "herdr-deck", "logs")
}

// Load reads the file at path. A missing file gives an empty File and no
// problems, unless it was named explicitly. A file that does not parse
// gives an empty File. Each top-level key decodes on its own, so a bad
// value or an unknown key is reported and skipped without losing the rest.
func Load(path string, named bool) (File, []string) {
	var f File
	if path == "" {
		return f, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !named {
			return f, nil
		}
		return f, []string{"config: " + shortErr(path, err)}
	}
	var raw map[string]toml.Primitive
	md, err := toml.Decode(string(b), &raw)
	if err != nil {
		return File{}, []string{"config: " + path + ": " + parseErr(err) + "; the file is ignored"}
	}
	fields := map[string]func(toml.Primitive) error{
		"linear_workspace":   func(p toml.Primitive) error { return decode(md, p, &f.LinearWorkspace) },
		"refresh_interval":   func(p toml.Primitive) error { return decode(md, p, &f.RefreshInterval) },
		"projects_root":      func(p toml.Primitive) error { return decode(md, p, &f.ProjectsRoot) },
		"editor":             func(p toml.Primitive) error { return decode(md, p, &f.Editor) },
		"diff":               func(p toml.Primitive) error { return decode(md, p, &f.Diff) },
		"reuse_browser_tabs": func(p toml.Primitive) error { return decode(md, p, &f.ReuseTabs) },
		"figma_desktop":      func(p toml.Primitive) error { return decode(md, p, &f.FigmaDesktop) },
	}
	var problems []string
	failed := map[string]bool{}
	for _, k := range md.Keys() {
		if len(k) != 1 {
			continue
		}
		dec, ok := fields[k[0]]
		if !ok {
			// An unknown table is one problem, not one per key in it.
			problems = append(problems, fmt.Sprintf("config: %s: unknown key %q, ignored", path, k[0]))
			continue
		}
		if err := dec(raw[k[0]]); err != nil {
			failed[k[0]] = true
			problems = append(problems, fmt.Sprintf("config: %s: %s: %s; ignored", path, k[0], valueErr(err)))
		}
	}
	// Keys left inside known tables, such as [editor] comand = "…".
	for _, k := range md.Undecoded() {
		if _, known := fields[k[0]]; known && len(k) > 1 && !failed[k[0]] {
			problems = append(problems, fmt.Sprintf("config: %s: unknown key %q, ignored", path, k.String()))
		}
	}
	return f, problems
}

// decode decodes p into a fresh T and sets *dst only when that works.
func decode[T any](md toml.MetaData, p toml.Primitive, dst **T) error {
	var v T
	if err := md.PrimitiveDecode(p, &v); err != nil {
		return err
	}
	*dst = &v
	return nil
}

// Flags are the command-line values; "" means the flag was not given.
type Flags struct {
	Config          string
	LinearWorkspace string
	RefreshInterval string
	ProjectsRoot    string
	Editor          string
	DiffTool        string
	// EditorTerminal, DiffTerminal and ReuseTabs are nil when the flag was
	// not given.
	EditorTerminal *bool
	DiffTerminal   *bool
	ReuseTabs      *bool
}

// Settings are the resolved values the deck runs with.
type Settings struct {
	// Path is the config file read, or "" when none could be located.
	Path            string
	LinearWorkspace string
	RefreshInterval time.Duration
	// ProjectsRoot is the herdr-projects root; "" when no home is known.
	ProjectsRoot string
	// Editor opens a worktree ({path}); Diff shows a worktree's changes
	// ({path}, {base}, optional {file}).
	Editor launch.Command
	Diff   launch.Command
	// ReuseTabs opens a web link by focusing a browser tab that already
	// shows it, where the browser allows (launch.Browser).
	ReuseTabs bool
	// FigmaDesktop opens Figma links in the Figma desktop app: the deck's
	// own, and those herdr hands the plugin (a Ctrl+click, the open-link
	// action).
	FigmaDesktop bool
	// Problems are one line each about the file or a bad value that was
	// skipped. They never stop the deck.
	Problems []string
}

// Resolve reads the config file and settles every setting: flag > env >
// file > default. A bad flag value is an error, since the user just typed
// it; a bad environment or file value is a problem and the next source is
// used. lookPath finds a program on $PATH (exec.LookPath); it picks the
// default diff tool.
func Resolve(fl Flags, getenv func(string) string, lookPath func(string) (string, error)) (Settings, error) {
	var s Settings
	var named bool
	s.Path, named = Path(fl.Config, getenv)
	f, problems := Load(s.Path, named)
	s.Problems = problems

	switch {
	case fl.LinearWorkspace != "":
		s.LinearWorkspace = fl.LinearWorkspace
	case getenv(EnvLinearWorkspace) != "":
		s.LinearWorkspace = getenv(EnvLinearWorkspace)
	case f.LinearWorkspace != nil:
		s.LinearWorkspace = strings.TrimSpace(*f.LinearWorkspace)
	}

	s.RefreshInterval = DefaultRefreshInterval
	if fl.RefreshInterval != "" {
		d, err := parseInterval(fl.RefreshInterval)
		if err != nil {
			return s, fmt.Errorf("--refresh-interval: %w", err)
		}
		s.RefreshInterval = d
	} else if v := getenv(EnvRefreshInterval); v != "" {
		if d, err := parseInterval(v); err == nil {
			s.RefreshInterval = d
		} else {
			s.Problems = append(s.Problems, "$"+EnvRefreshInterval+": "+err.Error()+"; ignored")
			s.RefreshInterval = fileInterval(f, s.Path, &s.Problems)
		}
	} else {
		s.RefreshInterval = fileInterval(f, s.Path, &s.Problems)
	}

	switch {
	case fl.ProjectsRoot != "":
		// A relative flag is relative to the working directory.
		p, err := filepath.Abs(expandHome(fl.ProjectsRoot, getenv))
		if err != nil {
			return s, fmt.Errorf("--projects-root: %w", err)
		}
		s.ProjectsRoot = p
	case getenv(EnvProjectsRoot) != "":
		s.ProjectsRoot = getenv(EnvProjectsRoot)
	default:
		if f.ProjectsRoot != nil {
			if p := expandHome(strings.TrimSpace(*f.ProjectsRoot), getenv); filepath.IsAbs(p) {
				s.ProjectsRoot = p
				break
			}
			s.Problems = append(s.Problems, fmt.Sprintf("config: %s: projects_root %q is not an absolute path; ignored", s.Path, *f.ProjectsRoot))
		}
		if home := homeDir(getenv); home != "" {
			s.ProjectsRoot = filepath.Join(home, ".herdr-projects")
		}
	}

	if v := getenv(EnvFigmaDesktop); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			s.FigmaDesktop = b
		} else {
			s.Problems = append(s.Problems, fmt.Sprintf("$%s: %q is not true or false; ignored", EnvFigmaDesktop, v))
			s.FigmaDesktop = f.FigmaDesktop != nil && *f.FigmaDesktop
		}
	} else if f.FigmaDesktop != nil {
		s.FigmaDesktop = *f.FigmaDesktop
	}

	var err error
	if s.Editor, err = resolveProgram(programSources{
		what: "editor", flag: fl.Editor, flagTerm: fl.EditorTerminal, flagName: "--editor",
		env: EnvEditor, envTerm: EnvEditorTerminal, file: f.Editor, path: s.Path,
		allowed: EditorPlaceholders, def: DefaultEditor, defTerm: false,
	}, getenv, &s.Problems); err != nil {
		return s, err
	}
	def := DefaultDiffTool
	if _, err := lookPath("hunk"); err != nil {
		def = FallbackDiffTool
	}
	if s.Diff, err = resolveProgram(programSources{
		what: "diff", flag: fl.DiffTool, flagTerm: fl.DiffTerminal, flagName: "--diff-tool",
		env: EnvDiffTool, envTerm: EnvDiffTerminal, file: f.Diff, path: s.Path,
		allowed: DiffPlaceholders, def: def, defTerm: true,
	}, getenv, &s.Problems); err != nil {
		return s, err
	}
	s.ReuseTabs = true
	switch v := getenv(EnvReuseTabs); {
	case fl.ReuseTabs != nil:
		s.ReuseTabs = *fl.ReuseTabs
	case v != "":
		if b, err := strconv.ParseBool(v); err == nil {
			s.ReuseTabs = b
			break
		}
		s.Problems = append(s.Problems, fmt.Sprintf("$%s: %q is not true or false; ignored", EnvReuseTabs, v))
		fallthrough
	default:
		if f.ReuseTabs != nil {
			s.ReuseTabs = *f.ReuseTabs
		}
	}

	// The Sources view is narrow: show the file as ~/… where it fits.
	if home := homeDir(getenv); home != "" && strings.HasPrefix(s.Path, home+string(filepath.Separator)) {
		short := "~" + strings.TrimPrefix(s.Path, home)
		for i, p := range s.Problems {
			s.Problems[i] = strings.ReplaceAll(p, s.Path, short)
		}
	}
	return s, nil
}

type programSources struct {
	what, flag, flagName string
	flagTerm             *bool
	env, envTerm         string
	file                 *Program
	path                 string
	allowed              []string
	def                  string
	defTerm              bool
}

// resolveProgram settles a command and its terminal option as one setting:
// the highest source with a valid command wins, and terminal comes from
// that source or a higher one, else the default. So --editor "zed {path}"
// is not opened in a pane because the file said terminal = true for nvim.
func resolveProgram(ps programSources, getenv func(string) string, problems *[]string) (launch.Command, error) {
	var term *bool
	setTerm := func(b *bool) {
		if term == nil && b != nil {
			term = b
		}
	}
	pick := func(argv []string) launch.Command {
		c := launch.Command{Argv: argv, Terminal: ps.defTerm}
		if term != nil {
			c.Terminal = *term
		}
		return c
	}

	setTerm(ps.flagTerm)
	if ps.flag != "" {
		argv, err := launch.ParseCommand(ps.flag, ps.allowed...)
		if err != nil {
			return launch.Command{}, fmt.Errorf("%s: %w", ps.flagName, err)
		}
		return pick(argv), nil
	}

	if v := getenv(ps.envTerm); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			setTerm(&b)
		} else {
			*problems = append(*problems, fmt.Sprintf("$%s: %q is not true or false; ignored", ps.envTerm, v))
		}
	}
	if v := getenv(ps.env); v != "" {
		argv, err := launch.ParseCommand(v, ps.allowed...)
		if err == nil {
			return pick(argv), nil
		}
		*problems = append(*problems, fmt.Sprintf("$%s: %v; ignored", ps.env, err))
	}

	if ps.file != nil {
		setTerm(ps.file.Terminal)
		if ps.file.Command != nil {
			argv, err := launch.ParseCommand(*ps.file.Command, ps.allowed...)
			if err == nil {
				return pick(argv), nil
			}
			*problems = append(*problems, fmt.Sprintf("config: %s: %s.command: %v; ignored", ps.path, ps.what, err))
		}
	}

	argv, err := launch.ParseCommand(ps.def, ps.allowed...)
	if err != nil {
		panic("config: bad default " + ps.what + " command: " + err.Error())
	}
	return pick(argv), nil
}

// fileInterval is the file's refresh_interval when it is valid, else the
// default.
func fileInterval(f File, path string, problems *[]string) time.Duration {
	if f.RefreshInterval == nil {
		return DefaultRefreshInterval
	}
	d := f.RefreshInterval.Duration
	if err := checkInterval(d); err != nil {
		*problems = append(*problems, fmt.Sprintf("config: %s: refresh_interval: %v; using %v", path, err, DefaultRefreshInterval))
		return DefaultRefreshInterval
	}
	return d
}

func parseInterval(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("%q is not a duration such as \"5s\"", s)
	}
	return d, checkInterval(d)
}

func checkInterval(d time.Duration) error {
	if d < MinRefreshInterval || d > MaxRefreshInterval {
		return fmt.Errorf("%v is outside %v–%v", d, MinRefreshInterval, MaxRefreshInterval)
	}
	return nil
}

// homeDir is $HOME, else the OS's idea of it, else "".
func homeDir(getenv func(string) string) string {
	if h := getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

// expandHome turns a leading "~/" (or a lone "~") into the home folder.
func expandHome(p string, getenv func(string) string) string {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p
	}
	home := homeDir(getenv)
	if home == "" {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}

// shortErr drops the operation from a *PathError so the line reads
// "<path>: <reason>".
func shortErr(path string, err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return path + ": " + pe.Err.Error()
	}
	return err.Error()
}

// valueErr is why one value did not decode, without the library's prefix.
func valueErr(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 && strings.HasPrefix(msg, "toml:") {
		msg = msg[i+2:]
	}
	return msg
}

// parseErr is a TOML error on one line, with its position.
func parseErr(err error) string {
	var pe toml.ParseError
	if errors.As(err, &pe) {
		return fmt.Sprintf("line %d: %s", pe.Position.Line, pe.Message)
	}
	return strings.ReplaceAll(err.Error(), "\n", " ")
}

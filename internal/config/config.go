// Package config reads the deck's settings file and resolves each setting
// from, in order: the command-line flag, the environment variable, the
// config file and the built-in default.
//
// The file is $XDG_CONFIG_HOME/herdr-deck/config.toml, else
// ~/.config/herdr-deck/config.toml (on macOS too); --config or
// $HERDR_DECK_CONFIG names another. Only the settings page writes it, one
// key at a time (Save). A missing file is fine; a file that cannot be
// parsed, an unknown key or a bad value never stops the deck: they come
// back as problems for the Sources view (or stderr) and the setting falls
// back to the next source.
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
	EnvDiffView        = "HERDR_DECK_DIFF_VIEW"
	EnvReuseTabs       = "HERDR_DECK_REUSE_BROWSER_TABS"
	EnvFigmaDesktop    = "HERDR_DECK_FIGMA_DESKTOP"
	EnvUpdateCheck     = "HERDR_DECK_UPDATE_CHECK"
	EnvAutoRestart     = "HERDR_DECK_AUTO_RESTART"
	EnvLinearStatus    = "HERDR_DECK_LINEAR_STATUS"
	// EnvLinearAPIKey is internal/source/linear's: the key itself, which
	// wins over linear_api_key_command.
	EnvLinearAPIKey = "LINEAR_API_KEY"
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

// The Files section's views: a flat list, or the files under their
// folders.
const (
	DiffViewList = "list"
	DiffViewTree = "tree"
)

// DiffViews are diff_view's values, in the order the settings page cycles
// them.
var DiffViews = []string{DiffViewList, DiffViewTree}

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
	DiffView        *string   `toml:"diff_view"`
	ReuseTabs       *bool     `toml:"reuse_browser_tabs"`
	FigmaDesktop    *bool     `toml:"figma_desktop"`
	UpdateCheck     *bool     `toml:"update_check"`
	AutoRestart     *bool     `toml:"auto_restart"`
	// LinearStatus turns the Linear issue status next to IDs on or off.
	LinearStatus *bool `toml:"linear_status"`
	// LinearAPIKeyCommand prints the Linear API key, e.g. `op read …`. The
	// key itself never goes in this file.
	LinearAPIKeyCommand *string `toml:"linear_api_key_command"`
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

// StateDir is where the deck keeps state that outlives it, such as the
// version it last ran: $XDG_STATE_HOME/herdr-deck, else
// ~/.local/state/herdr-deck. A relative $XDG_STATE_HOME is ignored. It
// returns "" when no home is known.
func StateDir(getenv func(string) string) string {
	if x := getenv("XDG_STATE_HOME"); x != "" && filepath.IsAbs(x) {
		return filepath.Join(x, "herdr-deck")
	}
	home := homeDir(getenv)
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "state", "herdr-deck")
}

// LogDir is where the deck keeps the logs of the commands it starts:
// StateDir's logs folder. It returns "" when no home is known.
func LogDir(getenv func(string) string) string {
	if d := StateDir(getenv); d != "" {
		return filepath.Join(d, "logs")
	}
	return ""
}

// CacheDir is where the deck keeps throwaway state, such as the last update
// check: $XDG_CACHE_HOME/herdr-deck, else ~/.cache/herdr-deck (on macOS
// too). It returns "" when no home is known.
func CacheDir(getenv func(string) string) string {
	if x := getenv("XDG_CACHE_HOME"); x != "" && filepath.IsAbs(x) {
		return filepath.Join(x, "herdr-deck")
	}
	home := homeDir(getenv)
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".cache", "herdr-deck")
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
		"diff_view":          func(p toml.Primitive) error { return decode(md, p, &f.DiffView) },
		"reuse_browser_tabs": func(p toml.Primitive) error { return decode(md, p, &f.ReuseTabs) },
		"figma_desktop":      func(p toml.Primitive) error { return decode(md, p, &f.FigmaDesktop) },
		"update_check":       func(p toml.Primitive) error { return decode(md, p, &f.UpdateCheck) },
		"auto_restart":       func(p toml.Primitive) error { return decode(md, p, &f.AutoRestart) },
		"linear_status":      func(p toml.Primitive) error { return decode(md, p, &f.LinearStatus) },
		"linear_api_key_command": func(p toml.Primitive) error {
			return decode(md, p, &f.LinearAPIKeyCommand)
		},
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
	DiffView        string
	// EditorTerminal, DiffTerminal, ReuseTabs, UpdateCheck and AutoRestart
	// are nil when the flag was not given.
	EditorTerminal *bool
	DiffTerminal   *bool
	ReuseTabs      *bool
	UpdateCheck    *bool
	AutoRestart    *bool
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
	// DiffView is the Files section's view when the deck starts:
	// DiffViewList or DiffViewTree.
	DiffView string
	// ReuseTabs opens a web link by focusing a browser tab that already
	// shows it, where the browser allows (launch.Browser).
	ReuseTabs bool
	// FigmaDesktop opens Figma links in the Figma desktop app: the deck's
	// own, and those herdr hands the plugin (a Ctrl+click, the open-link
	// action).
	FigmaDesktop bool
	// UpdateCheck shows "update available" in the header; AutoRestart
	// restarts the deck when its binary is replaced.
	UpdateCheck bool
	AutoRestart bool
	// LinearStatus shows each Linear issue's status next to its ID.
	LinearStatus bool
	// LinearAPIKeyCommand is the argv of the command that prints the Linear
	// API key, or nil. $LINEAR_API_KEY wins over it (internal/source/linear).
	LinearAPIKeyCommand []string
	// Problems are one line each about the file or a bad value that was
	// skipped. They never stop the deck.
	Problems []string
	// Values are each setting's effective value as text and where it came
	// from, keyed by Spec.Key, for the settings page.
	Values map[string]Value
}

// Resolve reads the config file and settles every setting: flag > env >
// file > default. A bad flag value is an error, since the user just typed
// it; a bad environment or file value is a problem and the next source is
// used. lookPath finds a program on $PATH (exec.LookPath); it picks the
// default diff tool.
func Resolve(fl Flags, getenv func(string) string, lookPath func(string) (string, error)) (Settings, error) {
	s := Settings{Values: map[string]Value{}}
	var named bool
	s.Path, named = Path(fl.Config, getenv)
	f, problems := Load(s.Path, named)
	s.Problems = problems

	switch {
	case fl.LinearWorkspace != "":
		s.LinearWorkspace = fl.LinearWorkspace
		s.note(KeyLinearWorkspace, s.LinearWorkspace, FromFlag)
	case getenv(EnvLinearWorkspace) != "":
		s.LinearWorkspace = getenv(EnvLinearWorkspace)
		s.note(KeyLinearWorkspace, s.LinearWorkspace, FromEnv)
	case f.LinearWorkspace != nil:
		s.LinearWorkspace = strings.TrimSpace(*f.LinearWorkspace)
		s.note(KeyLinearWorkspace, s.LinearWorkspace, FromFile)
	default:
		s.note(KeyLinearWorkspace, "", FromDefault)
	}

	var from Source
	if fl.RefreshInterval != "" {
		d, err := parseInterval(fl.RefreshInterval)
		if err != nil {
			return s, fmt.Errorf("--refresh-interval: %w", err)
		}
		s.RefreshInterval, from = d, FromFlag
	} else if v := getenv(EnvRefreshInterval); v != "" {
		if d, err := parseInterval(v); err == nil {
			s.RefreshInterval, from = d, FromEnv
		} else {
			s.Problems = append(s.Problems, "$"+EnvRefreshInterval+": "+err.Error()+"; ignored")
			s.RefreshInterval, from = fileInterval(f, s.Path, &s.Problems)
		}
	} else {
		s.RefreshInterval, from = fileInterval(f, s.Path, &s.Problems)
	}
	s.note(KeyRefreshInterval, s.RefreshInterval.String(), from)

	switch {
	case fl.ProjectsRoot != "":
		// A relative flag is relative to the working directory.
		p, err := filepath.Abs(expandHome(fl.ProjectsRoot, getenv))
		if err != nil {
			return s, fmt.Errorf("--projects-root: %w", err)
		}
		s.ProjectsRoot = p
		s.note(KeyProjectsRoot, p, FromFlag)
	case getenv(EnvProjectsRoot) != "":
		s.ProjectsRoot = getenv(EnvProjectsRoot)
		s.note(KeyProjectsRoot, s.ProjectsRoot, FromEnv)
	default:
		if f.ProjectsRoot != nil {
			if p := expandHome(strings.TrimSpace(*f.ProjectsRoot), getenv); filepath.IsAbs(p) {
				s.ProjectsRoot = p
				s.note(KeyProjectsRoot, p, FromFile)
				break
			}
			s.Problems = append(s.Problems, fmt.Sprintf("config: %s: projects_root %q is not an absolute path; ignored", s.Path, *f.ProjectsRoot))
		}
		if home := homeDir(getenv); home != "" {
			s.ProjectsRoot = filepath.Join(home, ".herdr-projects")
		}
		s.note(KeyProjectsRoot, s.ProjectsRoot, FromDefault)
	}

	s.FigmaDesktop = s.resolveBool(KeyFigmaDesktop, nil, EnvFigmaDesktop, f.FigmaDesktop, false, getenv)

	var err error
	if s.Editor, err = s.resolveProgram(programSources{
		what: "editor", flag: fl.Editor, flagTerm: fl.EditorTerminal, flagName: "--editor",
		env: EnvEditor, envTerm: EnvEditorTerminal, file: f.Editor, path: s.Path,
		allowed: EditorPlaceholders, def: DefaultEditor, defTerm: false,
	}, getenv); err != nil {
		return s, err
	}
	def := DefaultDiffTool
	if _, err := lookPath("hunk"); err != nil {
		def = FallbackDiffTool
	}
	if s.Diff, err = s.resolveProgram(programSources{
		what: "diff", flag: fl.DiffTool, flagTerm: fl.DiffTerminal, flagName: "--diff-tool",
		env: EnvDiffTool, envTerm: EnvDiffTerminal, file: f.Diff, path: s.Path,
		allowed: DiffPlaceholders, def: def, defTerm: true,
	}, getenv); err != nil {
		return s, err
	}
	if s.DiffView, err = s.resolveChoice(KeyDiffView, fl.DiffView, "--diff-view", EnvDiffView, f.DiffView, DiffViews, getenv); err != nil {
		return s, err
	}
	s.ReuseTabs = s.resolveBool(KeyReuseTabs, fl.ReuseTabs, EnvReuseTabs, f.ReuseTabs, true, getenv)
	s.LinearStatus = s.resolveBool(KeyLinearStatus, nil, EnvLinearStatus, f.LinearStatus, true, getenv)
	// The command takes no placeholders and runs as written, without a
	// shell. Problems never quote it, in case a key was pasted there.
	s.note(KeyLinearAPIKeyCommand, "", FromDefault)
	if f.LinearAPIKeyCommand != nil && strings.TrimSpace(*f.LinearAPIKeyCommand) != "" {
		if argv, err := launch.ParseCommand(*f.LinearAPIKeyCommand); err == nil {
			s.LinearAPIKeyCommand = argv
			s.note(KeyLinearAPIKeyCommand, strings.TrimSpace(*f.LinearAPIKeyCommand), FromFile)
		} else {
			s.Problems = append(s.Problems, fmt.Sprintf("config: %s: linear_api_key_command does not parse; ignored", s.Path))
		}
	}
	if getenv(EnvLinearAPIKey) != "" {
		v := s.Values[KeyLinearAPIKeyCommand]
		v.Note = "$" + EnvLinearAPIKey + " is set and wins over the command"
		s.Values[KeyLinearAPIKeyCommand] = v
	}

	s.UpdateCheck = s.resolveBool(KeyUpdateCheck, fl.UpdateCheck, EnvUpdateCheck, f.UpdateCheck, true, getenv)
	s.AutoRestart = s.resolveBool(KeyAutoRestart, fl.AutoRestart, EnvAutoRestart, f.AutoRestart, true, getenv)
	for k, text := range fileValues(f) {
		v := s.Values[k]
		v.File, v.InFile = text, true
		s.Values[k] = v
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
func (s *Settings) resolveProgram(ps programSources, getenv func(string) string) (launch.Command, error) {
	var term *bool
	termFrom := FromDefault
	setTerm := func(b *bool, from Source) {
		if term == nil && b != nil {
			term, termFrom = b, from
		}
	}
	pick := func(argv []string, text string, from Source) launch.Command {
		c := launch.Command{Argv: argv, Terminal: ps.defTerm}
		if term != nil {
			c.Terminal = *term
		}
		s.note(ps.what+".command", strings.TrimSpace(text), from)
		s.note(ps.what+".terminal", strconv.FormatBool(c.Terminal), termFrom)
		return c
	}

	setTerm(ps.flagTerm, FromFlag)
	if ps.flag != "" {
		argv, err := launch.ParseCommand(ps.flag, ps.allowed...)
		if err != nil {
			return launch.Command{}, fmt.Errorf("%s: %w", ps.flagName, err)
		}
		return pick(argv, ps.flag, FromFlag), nil
	}

	if v := getenv(ps.envTerm); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			setTerm(&b, FromEnv)
		} else {
			s.Problems = append(s.Problems, fmt.Sprintf("$%s: %q is not true or false; ignored", ps.envTerm, v))
		}
	}
	if v := getenv(ps.env); v != "" {
		argv, err := launch.ParseCommand(v, ps.allowed...)
		if err == nil {
			return pick(argv, v, FromEnv), nil
		}
		s.Problems = append(s.Problems, fmt.Sprintf("$%s: %v; ignored", ps.env, err))
	}

	if ps.file != nil {
		setTerm(ps.file.Terminal, FromFile)
		if ps.file.Command != nil {
			argv, err := launch.ParseCommand(*ps.file.Command, ps.allowed...)
			if err == nil {
				return pick(argv, *ps.file.Command, FromFile), nil
			}
			s.Problems = append(s.Problems, fmt.Sprintf("config: %s: %s.command: %v; ignored", ps.path, ps.what, err))
		}
	}

	argv, err := launch.ParseCommand(ps.def, ps.allowed...)
	if err != nil {
		panic("config: bad default " + ps.what + " command: " + err.Error())
	}
	return pick(argv, ps.def, FromDefault), nil
}

// resolveBool settles an on/off setting: flag > env > file > default. A bad
// environment value is a problem and the file or default is used.
func (s *Settings) resolveBool(key string, flag *bool, env string, file *bool, def bool, getenv func(string) string) bool {
	v, from := def, FromDefault
	switch e := getenv(env); {
	case flag != nil:
		v, from = *flag, FromFlag
	case e != "":
		if b, err := strconv.ParseBool(e); err == nil {
			v, from = b, FromEnv
			break
		}
		s.Problems = append(s.Problems, fmt.Sprintf("$%s: %q is not true or false; ignored", env, e))
		fallthrough
	default:
		if file != nil {
			v, from = *file, FromFile
		}
	}
	s.note(key, strconv.FormatBool(v), from)
	return v
}

// resolveChoice settles a setting that takes one of choices, the first
// being the default: flag > env > file > default. A bad flag value is an
// error; a bad environment or file value is a problem and the next source
// is used.
func (s *Settings) resolveChoice(key, flag, flagName, env string, file *string, choices []string, getenv func(string) string) (string, error) {
	if flag != "" {
		if err := checkChoice(flag, choices); err != nil {
			return "", fmt.Errorf("%s: %w", flagName, err)
		}
		s.note(key, flag, FromFlag)
		return flag, nil
	}
	if e := strings.TrimSpace(getenv(env)); e != "" {
		err := checkChoice(e, choices)
		if err == nil {
			s.note(key, e, FromEnv)
			return e, nil
		}
		s.Problems = append(s.Problems, fmt.Sprintf("$%s: %v; ignored", env, err))
	}
	if file != nil {
		v := strings.TrimSpace(*file)
		err := checkChoice(v, choices)
		if err == nil {
			s.note(key, v, FromFile)
			return v, nil
		}
		s.Problems = append(s.Problems, fmt.Sprintf("config: %s: %s: %v; ignored", s.Path, key, err))
	}
	s.note(key, choices[0], FromDefault)
	return choices[0], nil
}

func checkChoice(v string, choices []string) error {
	for _, c := range choices {
		if v == c {
			return nil
		}
	}
	return fmt.Errorf("%q is not %s", v, strings.Join(choices, " or "))
}

// note records a setting's effective value and source for the settings
// page.
func (s *Settings) note(key, text string, from Source) {
	s.Values[key] = Value{Text: text, Source: from}
}

// fileInterval is the file's refresh_interval when it is valid, else the
// default.
func fileInterval(f File, path string, problems *[]string) (time.Duration, Source) {
	if f.RefreshInterval == nil {
		return DefaultRefreshInterval, FromDefault
	}
	d := f.RefreshInterval.Duration
	if err := checkInterval(d); err != nil {
		*problems = append(*problems, fmt.Sprintf("config: %s: refresh_interval: %v; using %v", path, err, DefaultRefreshInterval))
		return DefaultRefreshInterval, FromDefault
	}
	return d, FromFile
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

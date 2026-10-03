package config

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/FredricW/herdr-deck/internal/launch"
)

// Source is where a setting's effective value comes from.
type Source int

const (
	FromDefault Source = iota
	FromFile
	FromEnv
	FromFlag
)

func (s Source) String() string {
	switch s {
	case FromFile:
		return "file"
	case FromEnv:
		return "env"
	case FromFlag:
		return "flag"
	}
	return "default"
}

// Value is a setting's effective value as text and where it came from.
type Value struct {
	// Text is the value as the file would hold it: "true", "5s", a
	// command line. "" is unset (no default).
	Text   string
	Source Source
	// Note says what else decides the setting, such as $LINEAR_API_KEY
	// winning over the key command.
	Note string
}

// Kind is how a setting is written and edited.
type Kind int

const (
	KindBool     Kind = iota // true or false; the page toggles it
	KindWord                 // a word such as a workspace slug
	KindPath                 // an absolute path; ~/ is the home folder
	KindDuration             // a Go duration such as "5s"
	KindCommand              // a command line, split without a shell
)

// Setting keys, as Spec.Key and Settings.Values name them. A key in a table
// is written table.key.
const (
	KeyLinearWorkspace     = "linear_workspace"
	KeyLinearStatus        = "linear_status"
	KeyLinearAPIKeyCommand = "linear_api_key_command"
	KeyReuseTabs           = "reuse_browser_tabs"
	KeyFigmaDesktop        = "figma_desktop"
	KeyEditorCommand       = "editor.command"
	KeyEditorTerminal      = "editor.terminal"
	KeyDiffCommand         = "diff.command"
	KeyDiffTerminal        = "diff.terminal"
	KeyUpdateCheck         = "update_check"
	KeyAutoRestart         = "auto_restart"
	KeyRefreshInterval     = "refresh_interval"
	KeyProjectsRoot        = "projects_root"
)

// Spec describes one setting for the settings page.
type Spec struct {
	Key   string
	Group string
	Kind  Kind
	// Flag and Env are what override the file; "" when there is none.
	Flag string
	Env  string
	// Help is one line about the setting.
	Help string
	// Placeholders a command may use.
	Placeholders []string
	// Restart says a running deck only picks a change up when it starts.
	Restart bool
}

// Specs are every setting, grouped, in the settings page's order.
var Specs = []Spec{
	{Key: KeyLinearWorkspace, Group: "Links and Linear", Kind: KindWord, Flag: "--linear-workspace", Env: EnvLinearWorkspace,
		Help: "Linear workspace that bare IDs such as ABC-123 link into"},
	{Key: KeyLinearStatus, Group: "Links and Linear", Kind: KindBool, Env: EnvLinearStatus,
		Help: "show each Linear issue's state next to its ID (needs an API key)"},
	{Key: KeyLinearAPIKeyCommand, Group: "Links and Linear", Kind: KindCommand,
		Help: "a command that prints the Linear API key, such as op read …; never the key itself"},
	{Key: KeyEditorCommand, Group: "Editor and diff", Kind: KindCommand, Flag: "--editor", Env: EnvEditor, Placeholders: EditorPlaceholders,
		Help: "what e opens a worktree with; {path} is the folder"},
	{Key: KeyEditorTerminal, Group: "Editor and diff", Kind: KindBool, Flag: "--editor-terminal", Env: EnvEditorTerminal,
		Help: "the editor is a terminal program: open it in a new herdr pane"},
	{Key: KeyDiffCommand, Group: "Editor and diff", Kind: KindCommand, Flag: "--diff-tool", Env: EnvDiffTool, Placeholders: DiffPlaceholders,
		Help: "shows a worktree's changes: {path}, {base} and an optional {file}"},
	{Key: KeyDiffTerminal, Group: "Editor and diff", Kind: KindBool, Flag: "--diff-terminal", Env: EnvDiffTerminal,
		Help: "the diff tool is a terminal program: open it in a new herdr pane"},
	{Key: KeyUpdateCheck, Group: "Updates", Kind: KindBool, Flag: "--update-check", Env: EnvUpdateCheck,
		Help: "show ↑ <version> in the header when a newer deck exists"},
	{Key: KeyAutoRestart, Group: "Updates", Kind: KindBool, Flag: "--auto-restart", Env: EnvAutoRestart,
		Help: "restart the deck in place when its binary is replaced"},
	{Key: KeyReuseTabs, Group: "Browser", Kind: KindBool, Flag: "--reuse-browser-tabs", Env: EnvReuseTabs,
		Help: "open a web link in a browser tab that already shows it (macOS)"},
	{Key: KeyFigmaDesktop, Group: "Browser", Kind: KindBool, Env: EnvFigmaDesktop,
		Help: "open Figma links in the Figma desktop app instead of the browser"},
	{Key: KeyRefreshInterval, Group: "Projects and refresh", Kind: KindDuration, Flag: "--refresh-interval", Env: EnvRefreshInterval,
		Help: "how often the deck reloads when no file change says to; 1s to 10m"},
	{Key: KeyProjectsRoot, Group: "Projects and refresh", Kind: KindPath, Flag: "--projects-root", Env: EnvProjectsRoot, Restart: true,
		Help: "the herdr-projects root; ~/ is your home folder"},
}

// SpecFor returns the spec of key.
func SpecFor(key string) (Spec, bool) {
	for _, s := range Specs {
		if s.Key == key {
			return s, true
		}
	}
	return Spec{}, false
}

// Override names the flag or environment variable a value came from, or ""
// when it came from the file or the default.
func (sp Spec) Override(from Source) string {
	switch from {
	case FromFlag:
		return sp.Flag
	case FromEnv:
		return "$" + sp.Env
	}
	return ""
}

// Check validates text as a value for key, the way Resolve reads it from
// the file.
func Check(key, text string) error {
	sp, ok := SpecFor(key)
	if !ok {
		return fmt.Errorf("unknown setting %q", key)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("%s needs a value", key)
	}
	switch sp.Kind {
	case KindBool:
		if _, err := strconv.ParseBool(text); err != nil {
			return fmt.Errorf("%q is not true or false", text)
		}
	case KindWord:
		if strings.ContainsAny(text, " \t/") {
			return fmt.Errorf("%q is not a single word such as acme", text)
		}
	case KindPath:
		if text != "~" && !strings.HasPrefix(text, "~/") && !filepath.IsAbs(text) {
			return fmt.Errorf("%q is not an absolute path; ~/ works", text)
		}
	case KindDuration:
		if _, err := parseInterval(text); err != nil {
			return err
		}
	case KindCommand:
		if _, err := launch.ParseCommand(text, sp.Placeholders...); err != nil {
			return err
		}
	}
	return nil
}

// Command herdr-deck is a status pane for one herdr-projects project.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"

	herdrdeck "github.com/FredricW/herdr-deck"
	"github.com/FredricW/herdr-deck/internal/changelog"
	"github.com/FredricW/herdr-deck/internal/config"
	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/launch"
	"github.com/FredricW/herdr-deck/internal/plugin"
	"github.com/FredricW/herdr-deck/internal/project"
	"github.com/FredricW/herdr-deck/internal/restart"
	"github.com/FredricW/herdr-deck/internal/source/dev"
	"github.com/FredricW/herdr-deck/internal/source/diff"
	"github.com/FredricW/herdr-deck/internal/source/fake"
	"github.com/FredricW/herdr-deck/internal/source/herdr"
	"github.com/FredricW/herdr-deck/internal/source/linear"
	"github.com/FredricW/herdr-deck/internal/source/live"
	"github.com/FredricW/herdr-deck/internal/source/projects"
	"github.com/FredricW/herdr-deck/internal/ui"
	"github.com/FredricW/herdr-deck/internal/update"
)

// debounce is how long the project folder must be quiet after a change
// before the deck reloads.
const debounce = 250 * time.Millisecond

func main() {
	if err := run(os.Args[1:]); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, "herdr-deck:", err)
		}
		os.Exit(2)
	}
}

func run(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "plugin":
			return runPlugin(args[1:])
		case "update":
			return runUpdate(args[1:])
		case "config":
			return runConfig(args[1:], os.Getenv, os.Stdout)
		}
	}
	fs := flag.NewFlagSet("herdr-deck", flag.ContinueOnError)
	var fl config.Flags
	fs.StringVar(&fl.Config, "config", "", "config file (default: $"+config.EnvPath+", else $XDG_CONFIG_HOME/herdr-deck/config.toml or ~/.config/herdr-deck/config.toml)")
	slugFlag := fs.String("project", "", "project slug (default: $"+project.EnvProject+", else the project folder containing the working directory)")
	demo := fs.Bool("fake", false, "show built-in sample data instead of the project")
	showVersion := fs.Bool("version", false, "print the version and commit, then exit")
	hidden := fl.Register(fs)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: herdr-deck [flags]\n       herdr-deck config migrate [--write]\n       herdr-deck update [--check]\n\nEvery setting's flag is --<table>-<key>, for the config file's [table] key.")
		config.PrintDefaults(fs, hidden)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q; the commands are config migrate, update and plugin", fs.Arg(0))
	}
	if *showVersion {
		fmt.Println(versionString())
		return nil
	}
	cfg, err := config.Resolve(fl, os.Getenv, exec.LookPath)
	if err != nil {
		return err
	}

	client := herdr.Client{Socket: herdr.SocketPath(os.Getenv)}
	runner := launch.Runner{}
	if self := os.Getenv("HERDR_PANE_ID"); self != "" {
		runner.Pane = func(argv []string, dir string) error {
			return client.RunInPane(context.Background(), self, dir, launch.ShellLine(argv))
		}
	}
	// The settings page swaps cur when it saves; everything that runs
	// later reads it there.
	var cur atomic.Pointer[config.Settings]
	cur.Store(&cfg)
	opt := ui.Options{
		OpenURL: func(url string) error {
			return launch.Browser{Reuse: cur.Load().ReuseTabs}.OpenURL(url)
		},
		FigmaDesktop: cfg.FigmaDesktop,
		OpenEditor: func(path string) error {
			ed := cur.Load().Editor
			return runner.Run(ed, launch.EditorArgv(ed, path), path)
		},
		OpenDiff: func(path, base string, files []string) error {
			df := cur.Load().Diff
			return runner.Run(df, launch.DiffArgv(df, path, base, files...), path)
		},
		OpenCommit: func(path, sha string, files []string) error {
			df := cur.Load().Diff
			return runner.Run(df, launch.CommitArgv(df, path, sha, files...), path)
		},
		DiffTree:    cfg.DiffView == config.DiffViewTree,
		DiffSplit:   cfg.DiffLayout == config.DiffLayoutSplit,
		FoldedLists: cfg.FoldedLists,
		Tick:        cfg.RefreshInterval,
		Version:     shortVersionString(),
		Changelog:   ownChangelog,
		Settings: &ui.SettingsHooks{
			Resolve: func() (config.Settings, error) { return config.Resolve(fl, os.Getenv, exec.LookPath) },
			Save:    config.Save,
			Apply:   func(s config.Settings) { cur.Store(&s) },
		},
	}
	// The sample deck notes an update too, so `make demo` can show it.
	if dir := config.StateDir(os.Getenv); dir != "" {
		opt.Updated = changelog.Seen(filepath.Join(dir, changelog.SeenFile), opt.Version)
	}
	if *demo {
		slug := *slugFlag
		if slug == "" {
			slug = "admin-rebuild"
		}
		snap := fake.Snapshot(slug, time.Now())
		snap.Missing = append(snap.Missing, cfg.Problems...)
		snap.Notes = append(snap.Notes, cfg.Notes...)
		snap.Projects = fake.Projects(snap, time.Now())
		// Loading the same sample again reads its diff, as a live deck's
		// first load does.
		opt.Load = func(context.Context) deck.Snapshot { return snap }
		opt.Diff = fake.Diff
		opt.Patch = fake.Patch
		opt.Commits = fake.Commits
		opt.CommitPatch = fake.CommitPatch
		opt.CommitFiles = fake.CommitFiles
		opt.CommitFilePatch = fake.CommitFilePatch
		// The sample's worktrees do not exist: e and d only say they opened.
		opt.OpenEditor = func(string) error { return nil }
		opt.OpenDiff = func(string, string, []string) error { return nil }
		opt.OpenCommit = func(string, string, []string) error { return nil }
		_, err := tea.NewProgram(ui.New(snap, opt)).Run()
		return err
	}

	if cfg.ProjectsRoot == "" {
		return errors.New("no projects root: set --projects-root, $" + config.EnvProjectsRoot + " or " + config.KeyProjectsRoot + " in the config file")
	}
	root := cfg.ProjectsRoot
	cwd, _ := os.Getwd()
	slug, err := project.Slug(*slugFlag, os.Getenv, cwd, root)
	if err != nil {
		return err
	}

	src := live.New(root, slug, cfg.LinearWorkspace)
	src.Herdr = herdr.NewReader(client.Socket)
	devs := dev.NewReader()
	devs.Logs = config.LogDir(os.Getenv)
	src.Dev = devs
	src.Roster = projects.NewRoster(root)
	go plugin.MarkSelf(context.Background(), plugin.Socket{Client: client}, os.Getenv, slug)

	// The program changes when a failed restart starts the deck again.
	var prog atomic.Pointer[tea.Program]
	refresh := func() {
		if p := prog.Load(); p != nil {
			p.Send(ui.RefreshMsg{})
		}
	}
	// Loads run one at a time, so this is the only place src changes:
	// it takes the latest settings before each read.
	var lin *linear.Reader
	opt.Load = func(ctx context.Context) deck.Snapshot {
		c := cur.Load()
		src.LinearWorkspace = c.LinearWorkspace
		src.Linear = nil
		if c.LinearStatus {
			if lin == nil || !slices.Equal(lin.Key.Command, c.LinearAPIKeyCommand) {
				lin = linear.NewReader(os.Getenv, c.LinearAPIKeyCommand)
				// A background fetch finished: reload to show its statuses.
				lin.OnUpdate = refresh
			}
			src.Linear = lin
		}
		snap := src.Read(ctx)
		// Config problems show in the Sources view with the sources' own.
		snap.Missing = append(slices.Clone(c.Problems), snap.Missing...)
		snap.Notes = append(slices.Clone(c.Notes), snap.Notes...)
		return snap
	}
	opt.FocusPane = func(id string) error { return client.Focus(context.Background(), id) }
	opt.OpenProject = func(slug string) error {
		return projects.New(root, slug).Open(context.Background())
	}
	diffs := &diff.Reader{}
	opt.Diff = diffs.Read
	opt.Patch = diffs.ReadPatch
	opt.Commits = diffs.ReadCommits
	opt.CommitPatch = diffs.ReadCommitPatch
	opt.CommitFiles = diffs.ReadCommitFiles
	opt.CommitFilePatch = diffs.ReadCommitFilePatch
	opt.StartDev = func(t deck.Thread) (string, error) { return devs.Up(context.Background(), slug, t) }
	// Both switches can change while the deck runs, so the checks are
	// always set up and ask cur each time.
	hint := updateHint(config.CacheDir(os.Getenv))
	opt.CheckUpdate = func(ctx context.Context) ui.Update {
		if !cur.Load().UpdateCheck {
			return ui.Update{}
		}
		return hint(ctx)
	}
	// The binary's path is taken now, before an update can move it.
	var exe string
	if restart.Supported {
		if exe, err = restart.Executable(os.Getenv); err == nil {
			if w, err := restart.New(exe); err == nil {
				opt.BinaryChanged = func(ctx context.Context) bool {
					return cur.Load().AutoRestart && w.Changed(ctx)
				}
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Without a watch (no project folder yet) the tick still reloads.
	_ = live.Watch(ctx, src.Projects.Dir(), debounce, refresh)
	// herdr's events reload too; without herdr this waits quietly.
	go client.Watch(ctx, herdr.WatchOptions{}, refresh)

	model := ui.New(deck.Snapshot{Project: deck.Project{Slug: slug}}, opt)
	for {
		p := tea.NewProgram(model)
		prog.Store(p)
		final, err := p.Run()
		if err != nil {
			return err
		}
		m, ok := final.(ui.Model)
		if !ok || !m.Restart() {
			return nil
		}
		// Bubble Tea has restored the terminal; the new binary takes over
		// this process, pane and all. Exec returns only on failure: then
		// the old deck carries on and says so.
		err = restart.Exec(exe)
		opt.BinaryChanged = nil
		opt.RestartFailed = err.Error()
		// Keep what the settings page saved since the start.
		opt.Tick, opt.FigmaDesktop = cur.Load().RefreshInterval, cur.Load().FigmaDesktop
		opt.FoldedLists = cur.Load().FoldedLists
		opt.Updated = ""
		model = ui.New(m.Snapshot(), opt)
	}
}

// running is this binary's version and commit, for update checks.
func running() update.Running {
	info, _ := debug.ReadBuildInfo()
	v, rev, _ := buildVersion(version, info)
	return update.Running{Version: v, Commit: rev}
}

// newUpdater runs real git, go and herdr commands; out gets their output.
func newUpdater(out io.Writer) update.Updater {
	return update.Updater{
		Exec:    update.OSExec{Stdout: out, Stderr: os.Stderr},
		Herdr:   os.Getenv("HERDR_BIN_PATH"),
		Running: running(),
		Out:     out,
	}
}

// ownChangelog is the embedded CHANGELOG.md. Its tests keep it free of
// problems.
var ownChangelog, _ = changelog.Parse(herdrdeck.Changelog)

// updateHint is the deck's update check, sharing one cache file between
// decks. A failure only shows in the Sources view. When something newer
// exists it also reads that version's changelog for What's new, once per
// newer version; a failure there only leaves the list out.
func updateHint(cacheDir string) func(context.Context) ui.Update {
	c := update.Checker{Updater: newUpdater(io.Discard)}
	if cacheDir != "" {
		c.CachePath = filepath.Join(cacheDir, "update.json")
	}
	var (
		mu       sync.Mutex
		newsFor  string // the label News was read for
		lastNews []changelog.Release
	)
	return func(ctx context.Context) ui.Update {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		st, err := c.Check(ctx)
		switch {
		case err != nil:
			return ui.Update{Problem: err.Error()}
		case !st.Newer:
			return ui.Update{}
		}
		mu.Lock()
		defer mu.Unlock()
		if newsFor != st.Label() {
			lastNews = nil // an older version's list, or none
			if text, final, err := c.Updater.Changelog(ctx, st); err == nil {
				remote, _ := changelog.Parse(string(text))
				lastNews = changelog.Newer(remote, ownChangelog, shortVersionString())
				if final {
					// A stand-in (origin's branch as last fetched) is
					// read again on the next check.
					newsFor = st.Label()
				}
			}
		}
		return ui.Update{Available: st.Label(), News: lastNews}
	}
}

// runConfig is `herdr-deck config migrate [--config <file>] [--write]`.
func runConfig(args []string, getenv func(string) string, out io.Writer) error {
	if len(args) == 0 || args[0] != "migrate" {
		return errors.New("config: the one command is `herdr-deck config migrate`")
	}
	fs := flag.NewFlagSet("herdr-deck config migrate", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: herdr-deck config migrate [--config <file>] [--write]")
		fmt.Fprintln(fs.Output(), "\nMoves the settings the config file still sets under their earlier flat names,\nsuch as linear_workspace, into their tables, such as [linear] workspace.\nIt prints the change; --write makes it, keeping the old file as <file>.bak.")
		fs.PrintDefaults()
	}
	path := fs.String("config", "", "config file (default: $"+config.EnvPath+", else $XDG_CONFIG_HOME/herdr-deck/config.toml or ~/.config/herdr-deck/config.toml)")
	write := fs.Bool("write", false, "rewrite the file instead of only showing the change")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("config migrate: unexpected argument %q", fs.Arg(0))
	}
	p, _ := config.Path(*path, getenv)
	m, err := config.MigrateFile(p, *write)
	if err != nil {
		return fmt.Errorf("config migrate: %s: %w", p, err)
	}
	switch {
	case m.Before == "" && len(m.Moved) == 0:
		fmt.Fprintf(out, "%s: no config file, so nothing to migrate.\n", p)
		return nil
	case len(m.Moved) == 0:
		fmt.Fprintf(out, "%s: every setting is already in its table; nothing to migrate.\n", p)
		return nil
	}
	verb := "Would move"
	if *write {
		verb = "Moved"
	}
	fmt.Fprintf(out, "%s %d setting(s) in %s:\n", verb, len(m.Moved), p)
	for _, l := range m.Moved {
		fmt.Fprintln(out, "  "+l)
	}
	fmt.Fprintln(out)
	fmt.Fprint(out, config.UnifiedDiff(m.Before, m.After, p, p+" (migrated)"))
	if *write {
		fmt.Fprintf(out, "\nThe old file is kept as %s.\n", m.Backup)
	} else {
		cmd := "herdr-deck config migrate --write"
		if *path != "" {
			cmd = "herdr-deck config migrate --config " + launch.ShellLine([]string{*path}) + " --write"
		}
		fmt.Fprintln(out, "\nNothing was written: run `"+cmd+"` to apply it, keeping the old file as .bak.")
	}
	return nil
}

// runUpdate is `herdr-deck update [--check]`.
func runUpdate(args []string) error {
	fs := flag.NewFlagSet("herdr-deck update", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: herdr-deck update [--check]")
		fmt.Fprintln(fs.Output(), "\nUpdates the herdr plugin: a GitHub install is reinstalled by herdr at the newest\nrelease tag; a linked checkout on its default branch is pulled and rebuilt.\nRunning decks restart with the new binary.")
		fs.PrintDefaults()
	}
	check := fs.Bool("check", false, "only say whether a newer version exists")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("update: unexpected argument %q", fs.Arg(0))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	u := newUpdater(os.Stdout)
	// Typed in a shell, this may be another build than the plugin's.
	self, selfErr := os.Executable()
	u.IsInstalled = func(bin string) bool {
		a, aerr := os.Stat(self)
		b, berr := os.Stat(bin)
		return selfErr == nil && aerr == nil && berr == nil && os.SameFile(a, b)
	}
	st, err := u.Check(ctx)
	if errors.Is(err, update.ErrNotInstalled) {
		return fmt.Errorf("%w; install it with `herdr plugin install FredricW/herdr-deck`, or update a `go install` with `go install github.com/FredricW/herdr-deck/cmd/herdr-deck@latest`", err)
	}
	if err != nil {
		return err
	}
	if *check {
		fmt.Println(st.Summary())
		return nil
	}
	return u.Apply(ctx, st)
}

// runPlugin runs a command herdr-plugin.toml gives herdr: the toggle and
// open-link actions or the auto-open hook. herdr logs its output (`herdr
// plugin log list`), config problems included; they never stop the hook.
func runPlugin(args []string) error {
	cfg, err := config.Resolve(config.Flags{}, os.Getenv, exec.LookPath)
	if err != nil {
		return err
	}
	for _, p := range cfg.Problems {
		fmt.Fprintln(os.Stderr, "herdr-deck:", p)
	}
	if cfg.ProjectsRoot == "" {
		return errors.New("no projects root: set $" + config.EnvProjectsRoot + " or " + config.KeyProjectsRoot + " in the config file")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	h := plugin.Socket{Client: herdr.Client{Socket: herdr.SocketPath(os.Getenv)}}
	env := plugin.Env{
		Getenv:  os.Getenv,
		Root:    cfg.ProjectsRoot,
		Links:   plugin.LinkSettings{LinearWorkspace: cfg.LinearWorkspace, FigmaDesktop: cfg.FigmaDesktop},
		OpenURL: launch.Browser{Reuse: cfg.ReuseTabs}.OpenURL,
	}
	if lin := linear.NewReader(os.Getenv, cfg.LinearAPIKeyCommand); cfg.LinearStatus && lin.Key.Configured() {
		env.Links.LinearKeyWorkspace = func() (string, error) { return lin.Workspace(ctx) }
	}
	return plugin.Run(ctx, h, env, args)
}

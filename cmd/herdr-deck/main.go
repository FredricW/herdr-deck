// Command herdr-deck is a status pane for one herdr-projects project.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FredricW/herdr-deck/internal/config"
	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/launch"
	"github.com/FredricW/herdr-deck/internal/plugin"
	"github.com/FredricW/herdr-deck/internal/project"
	"github.com/FredricW/herdr-deck/internal/source/dev"
	"github.com/FredricW/herdr-deck/internal/source/fake"
	"github.com/FredricW/herdr-deck/internal/source/herdr"
	"github.com/FredricW/herdr-deck/internal/source/live"
	"github.com/FredricW/herdr-deck/internal/ui"
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
	if len(args) > 0 && args[0] == "plugin" {
		return runPlugin(args[1:])
	}
	fs := flag.NewFlagSet("herdr-deck", flag.ContinueOnError)
	var fl config.Flags
	fs.StringVar(&fl.Config, "config", "", "config file (default: $"+config.EnvPath+", else $XDG_CONFIG_HOME/herdr-deck/config.toml or ~/.config/herdr-deck/config.toml)")
	slugFlag := fs.String("project", "", "project slug (default: $"+project.EnvProject+", else the project folder containing the working directory)")
	fs.StringVar(&fl.ProjectsRoot, "projects-root", "", "herdr-projects root (default: $"+config.EnvProjectsRoot+", else projects_root in the config file, else ~/.herdr-projects)")
	fs.StringVar(&fl.LinearWorkspace, "linear-workspace", "", "Linear workspace slug that bare issue IDs link into (default: $"+config.EnvLinearWorkspace+", else linear_workspace in the config file)")
	fs.StringVar(&fl.RefreshInterval, "refresh-interval", "", "reload interval, "+config.MinRefreshInterval.String()+" to "+config.MaxRefreshInterval.String()+" (default: $"+config.EnvRefreshInterval+", else refresh_interval in the config file, else "+config.DefaultRefreshInterval.String()+")")
	fs.StringVar(&fl.Editor, "editor", "", "command that opens a worktree, {path} is the folder (default: $"+config.EnvEditor+", else [editor] command in the config file, else \""+config.DefaultEditor+"\")")
	editorTerm := fs.Bool("editor-terminal", false, "the editor is a terminal program: open it in a new herdr pane (default: $"+config.EnvEditorTerminal+", else [editor] terminal)")
	fs.StringVar(&fl.DiffTool, "diff-tool", "", "command that shows a worktree's diff: {path}, {base}, optional {file} (default: $"+config.EnvDiffTool+", else [diff] command, else hunk or git diff)")
	diffTerm := fs.Bool("diff-terminal", false, "the diff tool is a terminal program (default: $"+config.EnvDiffTerminal+", else [diff] terminal)")
	reuseTabs := fs.Bool("reuse-browser-tabs", true, "open a web link in a browser tab that already shows it, on macOS (default: $"+config.EnvReuseTabs+", else reuse_browser_tabs in the config file, else true)")
	demo := fs.Bool("fake", false, "show built-in sample data instead of the project")
	showVersion := fs.Bool("version", false, "print the version and commit, then exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Println(versionString())
		return nil
	}
	// A bool flag only counts when it was given.
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "editor-terminal":
			fl.EditorTerminal = editorTerm
		case "diff-terminal":
			fl.DiffTerminal = diffTerm
		case "reuse-browser-tabs":
			fl.ReuseTabs = reuseTabs
		}
	})
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
	opt := ui.Options{
		OpenURL: launch.Browser{Reuse: cfg.ReuseTabs}.OpenURL,
		OpenEditor: func(path string) error {
			return runner.Run(cfg.Editor, launch.EditorArgv(cfg.Editor, path), path)
		},
		Tick:    cfg.RefreshInterval,
		Version: shortVersionString(),
	}
	if *demo {
		slug := *slugFlag
		if slug == "" {
			slug = "admin-rebuild"
		}
		snap := fake.Snapshot(slug, time.Now())
		snap.Missing = append(snap.Missing, cfg.Problems...)
		_, err := tea.NewProgram(ui.New(snap, opt)).Run()
		return err
	}

	if cfg.ProjectsRoot == "" {
		return errors.New("no projects root: set --projects-root, $" + config.EnvProjectsRoot + " or projects_root in the config file")
	}
	root := cfg.ProjectsRoot
	cwd, _ := os.Getwd()
	slug, err := project.Slug(*slugFlag, os.Getenv, cwd, root)
	if err != nil {
		return err
	}

	src := live.New(root, slug, cfg.LinearWorkspace)
	src.Herdr = herdr.NewReader(client.Socket)
	src.Dev = dev.NewReader()
	go plugin.MarkSelf(context.Background(), plugin.Socket{Client: client}, os.Getenv, slug)
	// Config problems show in the Sources view with the sources' own.
	opt.Load = func(ctx context.Context) deck.Snapshot {
		snap := src.Read(ctx)
		snap.Missing = append(slices.Clone(cfg.Problems), snap.Missing...)
		return snap
	}
	opt.FocusPane = func(id string) error { return client.Focus(context.Background(), id) }
	p := tea.NewProgram(ui.New(deck.Snapshot{Project: deck.Project{Slug: slug}}, opt))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Without a watch (no project folder yet) the tick still reloads.
	_ = live.Watch(ctx, src.Projects.Dir(), debounce, func() { p.Send(ui.RefreshMsg{}) })
	// herdr's events reload too; without herdr this waits quietly.
	go client.Watch(ctx, herdr.WatchOptions{}, func() { p.Send(ui.RefreshMsg{}) })

	_, err = p.Run()
	return err
}

// runPlugin runs a command herdr-plugin.toml gives herdr: the toggle action
// or the auto-open hook. herdr logs its output (`herdr plugin log list`),
// config problems included; they never stop the hook.
func runPlugin(args []string) error {
	cfg, err := config.Resolve(config.Flags{}, os.Getenv, exec.LookPath)
	if err != nil {
		return err
	}
	for _, p := range cfg.Problems {
		fmt.Fprintln(os.Stderr, "herdr-deck:", p)
	}
	if cfg.ProjectsRoot == "" {
		return errors.New("no projects root: set $" + config.EnvProjectsRoot + " or projects_root in the config file")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	h := plugin.Socket{Client: herdr.Client{Socket: herdr.SocketPath(os.Getenv)}}
	return plugin.Run(ctx, h, plugin.Env{Getenv: os.Getenv, Root: cfg.ProjectsRoot}, args)
}

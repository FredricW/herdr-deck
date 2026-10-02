// Command herdr-deck is a status pane for one herdr-projects project.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

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
	slugFlag := fs.String("project", "", "project slug (default: $"+project.EnvProject+", else the project folder containing the working directory)")
	linear := fs.String("linear-workspace", os.Getenv(deck.EnvLinearWorkspace), "Linear workspace slug that bare issue IDs link into (default: $"+deck.EnvLinearWorkspace+")")
	demo := fs.Bool("fake", false, "show built-in sample data instead of the project")
	showVersion := fs.Bool("version", false, "print the version and commit, then exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Println(versionString())
		return nil
	}

	opt := ui.Options{OpenURL: launch.URL, OpenEditor: launch.Editor, Version: shortVersionString()}
	if *demo {
		slug := *slugFlag
		if slug == "" {
			slug = "admin-rebuild"
		}
		_, err := tea.NewProgram(ui.New(fake.Snapshot(slug, time.Now()), opt)).Run()
		return err
	}

	root, err := project.Root(os.Getenv)
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	slug, err := project.Slug(*slugFlag, os.Getenv, cwd, root)
	if err != nil {
		return err
	}

	src := live.New(root, slug, *linear)
	src.Herdr = herdr.NewReader(herdr.SocketPath(os.Getenv))
	src.Dev = dev.NewReader()
	client := src.Herdr.Client
	go plugin.MarkSelf(context.Background(), plugin.Socket{Client: client}, os.Getenv, slug)
	opt.Load = src.Read
	opt.FocusPane = func(id string) error { return client.Focus(context.Background(), id) }
	p := tea.NewProgram(ui.New(deck.Snapshot{Project: deck.Project{Slug: slug}}, opt))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Without a watch (no project folder yet) the 5 s tick still reloads.
	_ = live.Watch(ctx, src.Projects.Dir(), debounce, func() { p.Send(ui.RefreshMsg{}) })
	// herdr's events reload too; without herdr this waits quietly.
	go client.Watch(ctx, herdr.WatchOptions{}, func() { p.Send(ui.RefreshMsg{}) })

	_, err = p.Run()
	return err
}

// runPlugin runs a command herdr-plugin.toml gives herdr: the toggle action
// or the auto-open hook. herdr logs its output (`herdr plugin log list`).
func runPlugin(args []string) error {
	root, err := project.Root(os.Getenv)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	h := plugin.Socket{Client: herdr.Client{Socket: herdr.SocketPath(os.Getenv)}}
	return plugin.Run(ctx, h, plugin.Env{Getenv: os.Getenv, Root: root}, args)
}

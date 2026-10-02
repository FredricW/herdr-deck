package plugin

import (
	"context"

	"github.com/FredricW/herdr-deck/internal/source/herdr"
)

// ID is the plugin id in herdr-plugin.toml, and Entrypoint its deck pane.
const (
	ID         = "herdr-deck"
	Entrypoint = "deck"
)

// Token is the pane token that marks a deck pane; its value is the project
// slug. The hooks set it right after opening a deck, and the deck sets it on
// its own pane at start, so a deck herdr restores is found too.
const Token = "herdr_deck"

// Host is the part of herdr the hooks use.
type Host interface {
	Snapshot(ctx context.Context) (herdr.State, error)
	// Open opens a deck pane and returns its id.
	Open(ctx context.Context, o Open) (string, error)
	Close(ctx context.Context, paneID string) error
	// Focus focuses a pane.
	Focus(ctx context.Context, paneID string) error
	// LeftOf is the pane left of paneID, or "" when there is none.
	LeftOf(ctx context.Context, paneID string) (string, error)
	// Mark sets Token on a pane.
	Mark(ctx context.Context, paneID, slug string) error
	// Layout is the layout of the tab that holds paneID.
	Layout(ctx context.Context, paneID string) (Layout, error)
	// SetRatio sets the ratio of the split at path in paneID's tab.
	SetRatio(ctx context.Context, paneID string, path []bool, ratio float64) error
	Notify(ctx context.Context, title, body string) error
}

// Open says where a deck opens and what it shows.
type Open struct {
	Target string            // the pane the deck opens to the right of
	Cwd    string            // the project folder
	Env    map[string]string // HERDR_DECK_PROJECT, HERDR_PROJECTS_ROOT, HERDR_DECK_CONFIG
	Focus  bool              // focus the deck; false keeps focus on Target
}

// Socket is the Host that talks to a running herdr over its socket.
type Socket struct {
	Client herdr.Client
}

func (s Socket) Snapshot(ctx context.Context) (herdr.State, error) {
	return s.Client.Snapshot(ctx)
}

func (s Socket) Open(ctx context.Context, o Open) (string, error) {
	var res struct {
		PluginPane struct {
			Pane struct {
				ID string `json:"pane_id"`
			} `json:"pane"`
		} `json:"plugin_pane"`
	}
	err := s.Client.Call(ctx, "plugin.pane.open", map[string]any{
		"plugin_id":      ID,
		"entrypoint":     Entrypoint,
		"placement":      "split",
		"direction":      "right",
		"target_pane_id": o.Target,
		"cwd":            o.Cwd,
		"env":            o.Env,
		"focus":          o.Focus,
	}, &res)
	return res.PluginPane.Pane.ID, err
}

func (s Socket) Close(ctx context.Context, paneID string) error {
	return s.Client.Call(ctx, "plugin.pane.close", map[string]string{"pane_id": paneID}, nil)
}

func (s Socket) Focus(ctx context.Context, paneID string) error {
	return s.Client.Focus(ctx, paneID)
}

func (s Socket) LeftOf(ctx context.Context, paneID string) (string, error) {
	var res struct {
		Neighbor struct {
			ID string `json:"neighbor_pane_id"` // null when there is none
		} `json:"neighbor"`
	}
	err := s.Client.Call(ctx, "pane.neighbor", map[string]string{"pane_id": paneID, "direction": "left"}, &res)
	return res.Neighbor.ID, err
}

func (s Socket) Mark(ctx context.Context, paneID, slug string) error {
	return s.Client.Call(ctx, "pane.report_metadata", map[string]any{
		"pane_id": paneID,
		"source":  ID,
		"tokens":  map[string]string{Token: slug},
	}, nil)
}

func (s Socket) Layout(ctx context.Context, paneID string) (Layout, error) {
	var tree struct {
		Layout struct {
			Root Node `json:"root"`
		} `json:"layout"`
	}
	if err := s.Client.Call(ctx, "layout.export", map[string]string{"pane_id": paneID}, &tree); err != nil {
		return Layout{}, err
	}
	var rects struct {
		Layout struct {
			Panes []struct {
				ID   string `json:"pane_id"`
				Rect Rect   `json:"rect"`
			} `json:"panes"`
		} `json:"layout"`
	}
	if err := s.Client.Call(ctx, "pane.layout", map[string]string{"pane_id": paneID}, &rects); err != nil {
		return Layout{}, err
	}
	l := Layout{Root: tree.Layout.Root, Rects: map[string]Rect{}}
	for _, p := range rects.Layout.Panes {
		l.Rects[p.ID] = p.Rect
	}
	return l, nil
}

func (s Socket) SetRatio(ctx context.Context, paneID string, path []bool, ratio float64) error {
	if path == nil {
		path = []bool{}
	}
	return s.Client.Call(ctx, "layout.set_split_ratio", map[string]any{
		"pane_id": paneID,
		"path":    path,
		"ratio":   ratio,
	}, nil)
}

func (s Socket) Notify(ctx context.Context, title, body string) error {
	return s.Client.Call(ctx, "notification.show", map[string]string{"title": title, "body": body}, nil)
}

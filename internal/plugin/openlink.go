package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/FredricW/herdr-deck/internal/config"
	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/tasks"
)

// LinkSettings are the settings the open-link action reads.
type LinkSettings struct {
	// LinearWorkspace is the slug bare Linear IDs link into; "" leaves
	// them without a URL.
	LinearWorkspace string
	// FigmaDesktop opens Figma links in the desktop app.
	FigmaDesktop bool
}

// OpenLink opens what herdr handed the open-link action: the URL of a
// Ctrl+clicked link that matched one of the manifest's link handlers, else
// the text selected when a keybinding ran the action, else text given on
// the command line. When it cannot open anything it says why in a herdr
// notification as well as in the returned error, which herdr logs.
func OpenLink(ctx context.Context, h Host, env Env, args []string) error {
	text := strings.Join(args, " ")
	if text == "" {
		text = linkText(env.Getenv)
	}
	url, err := LinkURL(text, env.Links)
	if err == nil {
		if env.OpenURL == nil {
			return errors.New("open-link: no way to open links")
		}
		err = env.OpenURL(url)
	}
	if err != nil {
		_ = h.Notify(ctx, "Deck: nothing opened", err.Error())
		return fmt.Errorf("open-link: %w", err)
	}
	return nil
}

// linkText is the clicked URL or the selected text herdr passed the action.
func linkText(getenv func(string) string) string {
	if u := getenv("HERDR_PLUGIN_CLICKED_URL"); u != "" {
		return u
	}
	var c struct {
		ClickedURL   string `json:"clicked_url"`
		SelectedText string `json:"selected_text"`
	}
	if json.Unmarshal([]byte(getenv("HERDR_PLUGIN_CONTEXT_JSON")), &c) != nil {
		return ""
	}
	if c.ClickedURL != "" {
		return c.ClickedURL
	}
	return c.SelectedText
}

// LinkURL is the URL to open for text: its first link or Linear ID, by the
// rules the deck links them with, so a selected ABC-123 opens in the Linear
// workspace. With FigmaDesktop a Figma link becomes its figma:// URL.
func LinkURL(text string, s LinkSettings) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("no link: Ctrl+click a link, or select a Linear ID before running the action")
	}
	l, known, ok := tasks.Pick(text, s.LinearWorkspace)
	switch {
	case !ok:
		return "", fmt.Errorf("no link or Linear ID in %q", clip(text, 40))
	case !known:
		return l.URL, nil // a site the deck does not know
	case l.Kind == deck.LinkLinear && l.URL == "":
		return "", fmt.Errorf("%s: no Linear workspace set; set linear.workspace in the deck's config file or $%s", l.Label, config.EnvLinearWorkspace)
	}
	if s.FigmaDesktop {
		if d := l.DesktopURL(); d != "" {
			return d, nil
		}
	}
	return l.URL, nil
}

// clip shortens s to n runes for a message.
func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

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

// LinkURL is the URL to open for text. A lone http(s) URL opens as it is;
// otherwise the first link the deck's scraper finds in text does, so a
// selected ABC-123 opens in the Linear workspace by the same rules the deck
// links it. With FigmaDesktop a Figma link becomes its figma:// URL.
func LinkURL(text string, s LinkSettings) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("no link: Ctrl+click a link, or select a Linear ID before running the action")
	}
	found := tasks.Scrape(text, s.LinearWorkspace)
	if isURL(text) && (len(found) == 0 || found[0].URL != text) {
		return text, nil // a URL of a kind the deck does not know
	}
	if len(found) == 0 {
		return "", fmt.Errorf("no link or Linear ID in %q", clip(text, 40))
	}
	l := found[0]
	if l.Kind == deck.LinkLinear && l.URL == "" {
		return "", fmt.Errorf("%s: no Linear workspace set; set linear_workspace in the deck's config file or $%s", l.Label, config.EnvLinearWorkspace)
	}
	if s.FigmaDesktop {
		if d := l.DesktopURL(); d != "" {
			return d, nil
		}
	}
	return l.URL, nil
}

// isURL reports whether s is one http(s) URL and nothing else.
func isURL(s string) bool {
	return (strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://")) && !strings.ContainsAny(s, " \t\n")
}

// clip shortens s to n runes for a message.
func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

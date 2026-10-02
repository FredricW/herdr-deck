package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// Named ANSI colours, so the terminal theme decides the shades
// (docs/design/README.md#colours).
var (
	colRed     = lipgloss.Red
	colGreen   = lipgloss.Green
	colYellow  = lipgloss.Yellow
	colBlue    = lipgloss.Blue
	colMagenta = lipgloss.Magenta
	colCyan    = lipgloss.Cyan

	plain       = lipgloss.NewStyle()
	bold        = lipgloss.NewStyle().Bold(true)
	dim         = lipgloss.NewStyle().Faint(true)
	needsStyle  = lipgloss.NewStyle().Foreground(colRed).Bold(true)
	inboxStyle  = lipgloss.NewStyle().Foreground(colYellow)
	workStyle   = lipgloss.NewStyle().Foreground(colCyan)
	reviewStyle = lipgloss.NewStyle().Foreground(colMagenta)
	okStyle     = lipgloss.NewStyle().Foreground(colGreen)
	failStyle   = lipgloss.NewStyle().Foreground(colRed)
	warnStyle   = lipgloss.NewStyle().Foreground(colYellow)
)

// The selection's background: a subtle grey from the 256-colour palette,
// dark on a dark terminal and light on a light one, under the text's own
// colours.
const (
	selDark  = 237
	selLight = 254
)

// highlight puts the selection background under s, which may already hold
// styled text: the background is set again after each of its resets, so
// red, cyan, magenta and dim text keep their colours on it.
func highlight(s string, light bool) string {
	bg := selDark
	if light {
		bg = selLight
	}
	on := fmt.Sprintf("\x1b[48;5;%dm", bg)
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+on)
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+on)
	return on + s + "\x1b[m"
}

func statusStyle(s deck.ThreadStatus) lipgloss.Style {
	switch s {
	case deck.StatusNeedsYou:
		return needsStyle
	case deck.StatusWorking:
		return workStyle
	case deck.StatusReview:
		return reviewStyle
	}
	return dim
}

func statusGlyph(s deck.ThreadStatus) string {
	switch s {
	case deck.StatusNeedsYou:
		return "●"
	case deck.StatusWorking:
		return "◐"
	case deck.StatusReview:
		return "◇"
	case deck.StatusDone:
		return "✓"
	}
	return "○"
}

func linkStyle(k deck.LinkKind) lipgloss.Style {
	switch k {
	case deck.LinkLinear:
		return bold.Foreground(colBlue)
	case deck.LinkFigma:
		return bold.Foreground(colMagenta)
	case deck.LinkLocalhost:
		return bold.Foreground(colGreen)
	}
	return bold
}

package plugin

import "math"

// Layout is one tab's split tree (layout.export) with each pane's rectangle
// (pane.layout).
type Layout struct {
	Root  Node
	Rects map[string]Rect
}

// Node is a split or a pane in a layout tree.
type Node struct {
	Type      string  `json:"type"` // "split" or "pane"
	PaneID    string  `json:"pane_id"`
	Direction string  `json:"direction"` // a split's: "right" or "down"
	Ratio     float64 `json:"ratio"`
	First     *Node   `json:"first"`
	Second    *Node   `json:"second"`
}

// Rect is a pane's position and size in cells.
type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Deck widths in columns: the deck aims for 40 % of the split, kept between
// minDeck and maxDeck, and the pane beside it keeps at least minNeighbour.
// Below that the two share the split evenly.
const (
	minDeck      = 60
	maxDeck      = 80
	minNeighbour = 60
)

// deckSplit finds the split whose second child is the deck pane, as a path
// of booleans from the root (true = second child), the form
// layout.set_split_ratio takes, and that split's width in cells. The deck
// opens as the right half of a new split, so this is the split it made.
func deckSplit(l Layout, deck string) (path []bool, width int, ok bool) {
	var walk func(n *Node, path []bool) bool
	walk = func(n *Node, p []bool) bool {
		if n == nil || n.Type != "split" {
			return false
		}
		if n.Direction == "right" && n.Second != nil && n.Second.Type == "pane" && n.Second.PaneID == deck {
			r, found := l.Rects[deck]
			left, any := leftEdge(n.First, l.Rects)
			if !found || !any {
				return false
			}
			path, width = append([]bool{}, p...), r.X+r.Width-left
			return true
		}
		return walk(n.First, append(p, false)) || walk(n.Second, append(p, true))
	}
	ok = walk(&l.Root, nil)
	return path, width, ok
}

// leftEdge is the smallest x of the panes under n.
func leftEdge(n *Node, rects map[string]Rect) (int, bool) {
	if n == nil {
		return 0, false
	}
	if n.Type == "pane" {
		r, ok := rects[n.PaneID]
		return r.X, ok
	}
	a, okA := leftEdge(n.First, rects)
	b, okB := leftEdge(n.Second, rects)
	switch {
	case okA && okB:
		return min(a, b), true
	case okA:
		return a, true
	default:
		return b, okB
	}
}

// ratio is the split ratio (the left pane's share) that gives the deck its
// width in a split width cells wide.
func ratio(width int) float64 {
	if width <= 0 {
		return 0.5
	}
	deck := int(math.Round(float64(width) * 0.4))
	deck = max(minDeck, min(maxDeck, deck))
	if width-deck < minNeighbour {
		return 0.5
	}
	return float64(width-deck) / float64(width)
}

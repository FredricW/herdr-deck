package ui

import (
	"math"

	"charm.land/lipgloss/v2"
)

// The scrollbar: one column at the right edge of a view whose lines
// overflow it, a thin grey track (│) with a heavier thumb (┃) whose length
// is the share of the lines that shows and whose place is where they are.
// It shows only while there is something to scroll, and the view's lines
// give it that column. A press on the thumb grabs it and a drag scrolls;
// a press on the track jumps so the thumb centres there, and the drag
// goes on from there. While dragged, the thumb turns blue, as the rule
// above the drawer does.

// barMinWidth is the narrowest view that gets a scrollbar: below it, the
// column is worth more to the lines.
const barMinWidth = 20

// The track and the thumb: fixed greys from the 256-colour palette. The
// track as faint as the split diff's hatching, so it reads as an edge
// rather than a line; the thumb clearly brighter (darker on a light
// terminal), but below the text.
const (
	trackDark  = 237
	trackLight = 253
	thumbDark  = 244
	thumbLight = 247
)

// barKind is which view's scrollbar is being dragged.
type barKind int

const (
	barNone    barKind = iota
	barPreview         // the diff preview
	barDrawer          // a full view's lines: a report, What's new, settings…
)

// scrollbar is a view's place in its lines: h of total lines show from
// off.
type scrollbar struct{ total, h, off int }

// shown says whether the view overflows, so it gets a scrollbar.
func (s scrollbar) shown() bool { return s.h > 0 && s.total > s.h }

// most is the last offset: the one that shows the last line at the end.
func (s scrollbar) most() int { return max(s.total-s.h, 0) }

// thumb is the thumb's first row and its length. Its length is the share
// of the lines that shows, at least a row and short of the whole track,
// and it touches the track's ends only at the view's ends, so a thumb at
// the top or bottom always means there is nothing further that way.
func (s scrollbar) thumb() (top, size int) {
	if !s.shown() {
		return 0, max(s.h, 0)
	}
	size = clamp(int(math.Round(float64(s.h)*float64(s.h)/float64(s.total))), 1, s.h-1)
	room := s.h - size
	off := clamp(s.off, 0, s.most())
	top = int(math.Round(float64(off) * float64(room) / float64(s.most())))
	switch {
	case room < 2:
	case off > 0 && top == 0:
		top = 1
	case off < s.most() && top == room:
		top = room - 1
	}
	return top, size
}

// offsetAt is the offset that puts the thumb's first row at row top of
// the track, within the view's lines.
func (s scrollbar) offsetAt(top int) int {
	_, size := s.thumb()
	room := s.h - size
	if !s.shown() || room <= 0 {
		return 0
	}
	top = clamp(top, 0, room)
	return int(math.Round(float64(top) * float64(s.most()) / float64(room)))
}

// grab is where a press on row y of the track takes hold of the thumb:
// the row within the thumb, and the offset to scroll to. On the thumb it
// stays put; on the track the thumb jumps so its middle is under y.
func (s scrollbar) grab(y int) (at, off int) {
	top, size := s.thumb()
	if y >= top && y < top+size {
		return y - top, clamp(s.off, 0, s.most())
	}
	at = size / 2
	return at, s.offsetAt(y - at)
}

// cells are the track's h cells, top to bottom; the thumb blue while
// dragged.
func (s scrollbar) cells(dragged, light bool) []string {
	top, size := s.thumb()
	track := greyStyle(trackDark, trackLight, light).Render("│")
	th := greyStyle(thumbDark, thumbLight, light)
	if dragged {
		th = lipgloss.NewStyle().Foreground(colBlue)
	}
	thumb := th.Render("┃")
	out := make([]string, max(s.h, 0))
	for i := range out {
		out[i] = track
		if i >= top && i < top+size {
			out[i] = thumb
		}
	}
	return out
}

// withBar puts the scrollbar s in the last of w columns of lines, which
// it cuts or pads to the w-1 columns before it. lines has s.h entries.
func withBar(lines []string, s scrollbar, w int, dragged, light bool) []string {
	cells := s.cells(dragged, light)
	out := make([]string, len(lines))
	for i, l := range lines {
		bar := ""
		if i < len(cells) {
			bar = cells[i]
		}
		out[i] = fit(l, w-1) + bar
	}
	return out
}

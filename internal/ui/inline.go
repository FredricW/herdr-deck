package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/FredricW/herdr-deck/internal/markdown"
)

// Inline code in task text: a warm colour from the 256-colour palette,
// like glow's, a step darker on a light terminal.
const (
	codeDark  = 209
	codeLight = 166
)

// inline renders task text, a title or a Next line, with its inline
// Markdown (markdown.Inline) on top of base: each run keeps base's colour
// and adds its own bold, italics and so on, so a row's colour still says
// what the row is. Code and links take a colour of their own only where
// base has none. Links carry their URL as an OSC 8 hyperlink. Measure and
// cut the result with ansi.StringWidth and ansi.Truncate, which skip the
// escapes.
func inline(text string, base lipgloss.Style, light bool) string {
	spans := markdown.Inline(text)
	if len(spans) == 1 && spans[0].Attr == 0 && spans[0].URL == "" {
		return base.Render(spans[0].Text)
	}
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(spanStyle(s, base, light).Render(s.Text))
	}
	return b.String()
}

// inlineWords is words for task text: the rendered words of text, each an
// item of its own (raw, as they are styled already) so the drawer can wrap
// them. A word styled in parts, like **ABC-123**:, stays one item.
func inlineWords(text string, base lipgloss.Style, light bool) group {
	g := group{sep: " "}
	var word strings.Builder
	end := func() {
		if word.Len() > 0 {
			g.items = append(g.items, item{text: word.String(), raw: true})
			word.Reset()
		}
	}
	for _, s := range markdown.Inline(text) {
		st := spanStyle(s, base, light)
		for i, part := range strings.Split(s.Text, " ") {
			if i > 0 {
				end()
			}
			if part != "" {
				word.WriteString(st.Render(part))
			}
		}
	}
	end()
	return g
}

// spanStyle is base with a span's own styles added.
func spanStyle(s markdown.Span, base lipgloss.Style, light bool) lipgloss.Style {
	st := base
	if s.Attr&markdown.Bold != 0 {
		st = st.Bold(true)
	}
	if s.Attr&markdown.Italic != 0 {
		st = st.Italic(true)
	}
	if s.Attr&markdown.Strike != 0 {
		st = st.Strikethrough(true)
	}
	if s.Attr&markdown.Underline != 0 {
		st = st.Underline(true)
	}
	_, noFg := base.GetForeground().(lipgloss.NoColor)
	if s.Attr&markdown.Code != 0 && noFg {
		var c color.Color = lipgloss.ANSIColor(codeDark)
		if light {
			c = lipgloss.ANSIColor(codeLight)
		}
		st = st.Foreground(c)
	}
	if s.URL != "" {
		if noFg && s.Attr&markdown.Code == 0 {
			st = st.Foreground(colBlue)
		}
		st = st.Hyperlink(s.URL)
	}
	return st
}

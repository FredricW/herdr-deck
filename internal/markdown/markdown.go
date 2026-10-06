// Package markdown renders Markdown for the terminal the way glow does,
// with glamour's dark and light styles tuned for a narrow pane.
package markdown

import (
	"regexp"
	"strings"
	"sync"

	"charm.land/glamour/v2"
	gansi "charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"github.com/charmbracelet/x/ansi"
)

// key is one cached rendering.
type key struct {
	src   string
	width int
	dark  bool
}

// maxCached bounds the cache; past it the cache starts over. A deck shows a
// handful of documents at a couple of widths, plus the PR tab's comments,
// up to about a hundred, each rendered on its own; this leaves room for a
// few PRs of those at two widths.
const maxCached = 512

var (
	mu    sync.Mutex
	cache = map[key][]string{}
	// renders counts glamour runs, for tests of the cache.
	renders int
	// render is glamour, replaced in tests to make it fail.
	render = renderGlamour
)

// Render renders src wrapped to width columns, with the dark or the light
// style, and returns its lines without leading or trailing blank lines. The
// result is cached by source, width and style. If glamour fails, it returns
// src as plain text wrapped to width.
func Render(src string, width int, dark bool) []string {
	width = max(width, 10)
	k := key{src, width, dark}
	mu.Lock()
	defer mu.Unlock()
	if lines, ok := cache[k]; ok {
		return lines
	}
	renders++
	lines, err := renderLines(src, width, dark)
	// glamour's word wrap can overshoot by a column in lists and quotes;
	// one column less fixes that, and fit catches anything left over.
	if err == nil && wider(lines, width) {
		lines, err = renderLines(src, width-1, dark)
		lines = fit(lines, width)
	}
	if err != nil {
		lines = plain(src, width)
	}
	if len(cache) >= maxCached {
		cache = map[key][]string{}
	}
	cache[k] = lines
	return lines
}

// renderLines renders src with glamour and tidies its lines.
func renderLines(src string, width int, dark bool) ([]string, error) {
	out, err := render(src, width, dark)
	if err != nil {
		return nil, err
	}
	return trim(strings.Split(out, "\n")), nil
}

// wider reports whether any line is wider than width.
func wider(lines []string, width int) bool {
	for _, l := range lines {
		if ansi.StringWidth(l) > width {
			return true
		}
	}
	return false
}

func renderGlamour(src string, width int, dark bool) (string, error) {
	r, err := glamour.NewTermRenderer(glamour.WithStyles(style(dark)), glamour.WithWordWrap(width))
	if err != nil {
		return "", err
	}
	return r.Render(src)
}

// style is glamour's dark or light style without the margin that a narrow
// pane can't spare: the document sits at the left edge, only code blocks
// are indented, and nested lists indent by two.
func style(dark bool) gansi.StyleConfig {
	s := styles.LightStyleConfig
	if dark {
		s = styles.DarkStyleConfig
	}
	zero := uint(0)
	s.Document.Margin = &zero
	s.List.LevelIndent = 2
	return s
}

// fit wraps again any line glamour leaves wider than width, which the
// drawer would otherwise cut off.
func fit(lines []string, width int) []string {
	out := lines[:0:0]
	for _, l := range lines {
		if ansi.StringWidth(l) <= width {
			out = append(out, l)
			continue
		}
		out = append(out, strings.Split(ansi.Wrap(l, width, ""), "\n")...)
	}
	return out
}

// plain wraps src to width without styles: the fallback when glamour fails.
func plain(src string, width int) []string {
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(src), "\n") {
		lines = append(lines, strings.Split(ansi.Wrap(strings.TrimRight(l, " "), width, ""), "\n")...)
	}
	return lines
}

// trim tidies glamour's lines: it drops the trailing spaces and empty
// styled runs it pads lines with, and blank lines at either end.
func trim(lines []string) []string {
	for i, l := range lines {
		lines[i] = squeeze(trimRight(l))
		if ansi.Strip(lines[i]) == "" {
			lines[i] = ""
		}
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

var (
	emptyRun = regexp.MustCompile(`\x1b\[[0-9;]*m\x1b\[0?m`)
	resets   = regexp.MustCompile(`(\x1b\[0?m)+`)
)

// squeeze replaces each style that is set and reset with nothing in
// between by a single reset, then collapses runs of resets.
func squeeze(l string) string {
	for {
		s := emptyRun.ReplaceAllString(l, "\x1b[m")
		if s == l {
			break
		}
		l = s
	}
	return resets.ReplaceAllString(l, "\x1b[m")
}

// trimRight drops trailing spaces from a styled line, keeping its escape
// sequences.
func trimRight(l string) string {
	plain := ansi.Strip(l)
	w := ansi.StringWidth(strings.TrimRight(plain, " "))
	if w == ansi.StringWidth(plain) {
		return l
	}
	return ansi.Cut(l, 0, w) + "\x1b[m"
}

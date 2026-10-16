package ui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// formatted is calm with inline Markdown in task titles, a note and a
// Next line, and literal markers that must stay as written.
func formatted() deck.Snapshot {
	s := calm()
	tasks := s.TaskLists[0].Tasks
	tasks[0].Title = "Fix **ledger** totals in `ledger.go`"
	tasks[0].Notes = "See [the spec](https://example.com/spec), keep snake_case_names and 2*3*4 as written.\n- a list item with ~~old~~ <u>new</u> text"
	tasks[1].Title = "Rename snake_case_names ~~now~~ <u>soon</u>"
	tasks[2].Title = "Read [the summary design spec](https://example.com/spec) *carefully* before starting"
	s.Threads[1].Next = []string{"Merge **the PR** after `make test` passes"}
	return s
}

func TestInlineGolden(t *testing.T) {
	for _, w := range []int{80, 60} {
		m, _ := newModel(t, formatted(), w, 34)
		golden(t, fmt.Sprintf("inline-%d", w), m)
	}
}

// Markers never show, widths come from the rendered text so the columns
// after WORK line up, and a cut title ends in an ellipsis.
func TestInlineRows(t *testing.T) {
	for _, w := range []int{80, 60} {
		m, _ := newModel(t, formatted(), w, 34)
		scr := screen(m)
		for _, want := range []string{"Fix ledger totals in", "Rename snake_case_names n", "Read the summary"} {
			if !strings.Contains(scr, want) {
				t.Errorf("width %d: %q not on screen:\n%s", w, want, scr)
			}
		}
		list := strings.SplitN(scr, "────", 3)[1]
		for _, marker := range []string{"**", "`", "~~", "<u>", "](", "*carefully*"} {
			if strings.Contains(list, marker) {
				t.Errorf("width %d: marker %q shows in the list:\n%s", w, marker, list)
			}
		}
		// The column after WORK: thread ids at 80, the status at 60.
		after := []string{"t-0002", "t-0003", "t-0004"}
		if w < wideMin {
			after = []string{"Building", "Writing", "#2320"}
		}
		x0, _ := find(t, m, after[0])
		for _, s := range after[1:] {
			if x, _ := find(t, m, s); x != x0 {
				t.Errorf("width %d: %q at column %d, want %d like %q", w, s, x, x0, after[0])
			}
		}
		if _, y := find(t, m, "Read the summary"); !strings.Contains(strings.Split(scr, "\n")[y], "…") {
			t.Errorf("width %d: the long title is cut without an ellipsis: %q", w, strings.Split(scr, "\n")[y])
		}
	}
}

// Inline styles layer on the row's own: the selected row keeps its
// background after every styled run, bold and code are styled, and a
// link is an OSC 8 hyperlink.
func TestInlineStyles(t *testing.T) {
	m, _ := newModel(t, formatted(), 80, 34)
	lines := strings.Split(m.View().Content, "\n")
	var sel, link string
	for _, l := range lines {
		plain := ansi.Strip(l)
		if strings.Contains(plain, "▸") && strings.Contains(plain, "Fix ledger") {
			sel = l
		}
		if strings.Contains(plain, "Read the summary") {
			link = l
		}
	}
	if sel == "" || link == "" {
		t.Fatalf("rows not found:\n%s", screen(m))
	}
	on := fmt.Sprintf("\x1b[48;5;%dm", selDark)
	for i, part := range strings.Split(sel, "\x1b[m")[1:] {
		if part != "" && !strings.HasPrefix(part, on) {
			t.Errorf("run %d of the selected row lost its background: %q", i+1, part)
		}
	}
	if !strings.Contains(sel, "1mledger") {
		t.Errorf("ledger is not bold: %q", sel)
	}
	if !strings.Contains(link, ansi.SetHyperlink("https://example.com/spec")) {
		t.Errorf("the link has no OSC 8 hyperlink: %q", link)
	}
}

// The drawer's Next line and the note's inline line show their text
// without markers; a list item in the note still goes through glamour.
func TestInlineDrawer(t *testing.T) {
	m, _ := newModel(t, formatted(), 80, 40)
	scr := screen(m)
	for _, want := range []string{
		"→ Merge the PR after make test passes",
		"See the spec, keep snake_case_names and 2*3*4 as written.",
		"• a list item with old new text",
	} {
		if !strings.Contains(scr, want) {
			t.Errorf("%q not in the drawer:\n%s", want, scr)
		}
	}
	if strings.Contains(scr, "https://example.com/spec") {
		t.Errorf("the link's URL shows:\n%s", scr)
	}
}

// inline keeps base's colour on styled runs and colours code and links
// only when base has none.
func TestInlineSpanStyle(t *testing.T) {
	red := lipgloss.NewStyle().Foreground(colRed)
	got := inline("a `b` [c](https://example.com)", red, false)
	for _, run := range []string{"b", "c"} {
		if !strings.Contains(got, "\x1b[31m"+run+"\x1b[m") {
			t.Errorf("red base: %s is not plain red: %q", run, got)
		}
	}
	got = inline("a `b`", plain, false)
	if !strings.Contains(got, fmt.Sprintf("38;5;%dm", codeDark)) {
		t.Errorf("plain base: code is not coloured: %q", got)
	}
	if got := inline("snake_case", red, false); got != red.Render("snake_case") {
		t.Errorf("plain text = %q, want %q", got, red.Render("snake_case"))
	}
}

// inlineWords splits on any whitespace and keeps a word styled in parts
// as one item.
func TestInlineWords(t *testing.T) {
	var got []string
	for _, it := range inlineWords("step1\tstep2 **ABC-1**: done  `x y`", plain, false).items {
		got = append(got, ansi.Strip(it.text))
	}
	want := []string{"step1", "step2", "ABC-1:", "done", "x", "y"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("words = %q, want %q", got, want)
	}
}

package markdown

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

const sample = `PR: https://github.com/acme/webshop/pull/12

## Report

Added the **checkout** page; ` + "`make test`" + ` passes, and the order summary
wraps onto a second line in the source.

- A list item that is long enough to wrap in a narrow sixty-column deck pane.
  - nested
- [ABC-123](https://linear.app/acme/issue/ABC-123)

` + "```go\nfunc main() {}\n```" + `

## Next

- Merge the PR
`

// reset clears the cache, so each test counts its own renders.
func reset(t *testing.T) {
	t.Helper()
	mu.Lock()
	cache = map[key][]string{}
	renders = 0
	mu.Unlock()
}

func TestRender(t *testing.T) {
	for _, width := range []int{60, 80} {
		for _, dark := range []bool{true, false} {
			reset(t)
			lines := Render(sample, width, dark)
			text := ansi.Strip(strings.Join(lines, "\n"))
			for i, l := range lines {
				if w := ansi.StringWidth(l); w > width {
					t.Errorf("width %d dark %v: line %d is %d wide: %q", width, dark, i, w, ansi.Strip(l))
				}
				if strings.HasSuffix(ansi.Strip(l), " ") {
					t.Errorf("width %d dark %v: line %d ends in a space: %q", width, dark, i, ansi.Strip(l))
				}
			}
			if lines[0] == "" || lines[len(lines)-1] == "" {
				t.Errorf("width %d dark %v: blank line at an end: %q", width, dark, lines)
			}
			for _, want := range []string{"## Report", "• Merge the PR", "make test", "func main() {}", "the order summary wraps"} {
				if !strings.Contains(strings.ReplaceAll(text, "\n", " "), want) {
					t.Errorf("width %d dark %v: %q missing from\n%s", width, dark, want, text)
				}
			}
			if strings.Contains(text, "**") {
				t.Errorf("width %d dark %v: emphasis markers left in:\n%s", width, dark, text)
			}
			// The document has no left margin: paragraphs start at column 0.
			if !strings.Contains(text, "\nAdded the checkout") {
				t.Errorf("width %d dark %v: the paragraph is indented:\n%s", width, dark, text)
			}
		}
	}
}

func TestRenderStyles(t *testing.T) {
	reset(t)
	dark := strings.Join(Render(sample, 60, true), "\n")
	light := strings.Join(Render(sample, 60, false), "\n")
	if dark == light {
		t.Fatal("the dark and light styles render the same")
	}
	if ansi.Strip(dark) != ansi.Strip(light) {
		t.Errorf("the styles should differ only in colour:\ndark:\n%s\nlight:\n%s", ansi.Strip(dark), ansi.Strip(light))
	}
	if !strings.Contains(dark, "\x1b[") {
		t.Error("the dark style has no escape sequences")
	}
}

func TestRenderNarrowWraps(t *testing.T) {
	reset(t)
	wide := Render(sample, 80, true)
	narrow := Render(sample, 60, true)
	if len(narrow) <= len(wide) {
		t.Errorf("60 columns gave %d lines, 80 gave %d; want more at 60", len(narrow), len(wide))
	}
}

func TestRenderFallback(t *testing.T) {
	reset(t)
	render = func(string, int, bool) (string, error) { return "", errors.New("broken") }
	t.Cleanup(func() { render = renderGlamour })
	src := "## Title\n\nA paragraph long enough to wrap at a width of twenty columns.\n"
	got := Render(src, 20, true)
	want := []string{"## Title", "", "A paragraph long", "enough to wrap at a", "width of twenty", "columns."}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("fallback = %q, want %q", got, want)
	}
}

func TestRenderCache(t *testing.T) {
	reset(t)
	first := Render(sample, 60, true)
	Render(sample, 60, true)
	if renders != 1 {
		t.Fatalf("renders = %d after the same call twice, want 1", renders)
	}
	if again := Render(sample, 60, true); strings.Join(again, "\n") != strings.Join(first, "\n") {
		t.Error("the cached rendering differs")
	}
	Render(sample, 80, true)
	Render(sample, 60, false)
	Render(sample+"\nMore.\n", 60, true)
	if renders != 4 {
		t.Errorf("renders = %d, want one per width, style and source (4)", renders)
	}
	for range maxCached + 1 {
		Render(sample+strings.Repeat(" ", renders), 60, true)
	}
	mu.Lock()
	n := len(cache)
	mu.Unlock()
	if n > maxCached {
		t.Errorf("cache holds %d entries, want at most %d", n, maxCached)
	}
}

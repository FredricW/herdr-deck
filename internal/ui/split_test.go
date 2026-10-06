package ui

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/config"
	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/diff"
)

// layout writes a splitView's rows as text, one per row: "full" rows by
// their line, pairs as left|right with the lines' texts and numbers, _
// for filler, "" for a blank row.
func layout(lines []deck.PatchLine, sv splitView) []string {
	side := func(i int, nos []int) string {
		switch {
		case i < 0:
			return "_"
		case lines[i].Kind == deck.LineNote:
			return `\`
		}
		return fmt.Sprintf("%d:%s", nos[i], lines[i].Text)
	}
	var out []string
	for _, r := range sv.rows {
		switch {
		case r.gap:
			out = append(out, "")
		case r.full >= 0:
			out = append(out, lines[r.full].Text)
		case r.only != 0:
			out = append(out, fmt.Sprintf("only%+d %s", r.only, side(max(r.l, r.r), map[bool][]int{true: sv.oldNo, false: sv.newNo}[r.only < 0])))
		default:
			out = append(out, side(r.l, sv.oldNo)+" | "+side(r.r, sv.newNo))
		}
	}
	return out
}

func TestPairLines(t *testing.T) {
	cases := []struct {
		name  string
		patch string
		want  []string
	}{
		{
			name:  "context on both sides",
			patch: "@@ -3,2 +3,2 @@\n a\n b\n",
			want:  []string{"@@ -3,2 +3,2 @@", "3:a | 3:a", "4:b | 4:b"},
		},
		{
			name:  "adds only",
			patch: "@@ -1,2 +1,4 @@\n a\n+x\n+y\n b\n",
			want:  []string{"@@ -1,2 +1,4 @@", "1:a | 1:a", "_ | 2:x", "_ | 3:y", "2:b | 4:b"},
		},
		{
			name:  "removes only",
			patch: "@@ -1,3 +1,1 @@\n-x\n-y\n a\n",
			want:  []string{"@@ -1,3 +1,1 @@", "1:x | _", "2:y | _", "3:a | 1:a"},
		},
		{
			name:  "more removed than added",
			patch: "@@ -10,4 +10,2 @@\n-a\n-b\n-c\n+A\n d\n",
			want:  []string{"@@ -10,4 +10,2 @@", "10:a | 10:A", "11:b | _", "12:c | _", "13:d | 11:d"},
		},
		{
			name:  "more added than removed, two runs",
			patch: "@@ -1,3 +1,5 @@\n-a\n+A\n+B\n+C\n m\n-z\n+Z\n",
			want:  []string{"@@ -1,3 +1,5 @@", "1:a | 1:A", "_ | 2:B", "_ | 3:C", "2:m | 4:m", "3:z | 5:Z"},
		},
		{
			name:  "no newline at the end of the old file",
			patch: "@@ -1 +1 @@\n-a\n\\ No newline at end of file\n+a\n",
			want:  []string{"@@ -1 +1 @@", "1:a | 1:a", `\ | _`},
		},
		{
			name:  "no newline at the end of either",
			patch: "@@ -1,2 +1,2 @@\n x\n-a\n\\ No newline at end of file\n+b\n\\ No newline at end of file\n",
			want:  []string{"@@ -1,2 +1,2 @@", "1:x | 1:x", "2:a | 2:b", `\ | \`},
		},
		{
			name:  "no newline on a context line",
			patch: "@@ -1,2 +1,2 @@\n-a\n+b\n z\n\\ No newline at end of file\n",
			want:  []string{"@@ -1,2 +1,2 @@", "1:a | 1:b", "2:z | 2:z", `\ | \`},
		},
		{
			name:  "a blank row between hunks",
			patch: "@@ -1,1 +1,1 @@\n-a\n+b\n@@ -9,1 +9,1 @@\n c\n",
			want:  []string{"@@ -1,1 +1,1 @@", "1:a | 1:b", "", "@@ -9,1 +9,1 @@", "9:c | 9:c"},
		},
		{
			name:  "an added file has only its new side",
			patch: "@@ -0,0 +1,2 @@\n+a\n+b\n",
			want:  []string{"@@ -0,0 +1,2 @@", "only+1 1:a", "only+1 2:b"},
		},
		{
			name:  "a deleted file has only its old side",
			patch: "@@ -1,2 +0,0 @@\n-a\n-b\n\\ No newline at end of file\n",
			want:  []string{"@@ -1,2 +0,0 @@", "only-1 1:a", "only-1 2:b", `only-1 \`},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := diff.ParsePatch([]byte(c.patch), diff.MaxPatchLines)
			got := layout(p.Lines, pairLines(p.Lines))
			if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
				t.Errorf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(c.want, "\n"))
			}
		})
	}
}

// A commit's files each get their own sides and numbers.
func TestPairLinesCommit(t *testing.T) {
	lines := []deck.PatchLine{
		{Kind: deck.LineFile, Text: "new.go"},
		{Kind: deck.LineHunk, Text: "@@ -0,0 +1 @@"},
		{Kind: deck.LineAdded, Text: "package x"},
		{Kind: deck.LineFile, Text: "old.go"},
		{Kind: deck.LineHunk, Text: "@@ -120,2 +120,2 @@"},
		{Kind: deck.LineDeleted, Text: "a"},
		{Kind: deck.LineAdded, Text: "b"},
		{Kind: deck.LineContext, Text: "c"},
	}
	sv := pairLines(lines)
	want := []string{"new.go", "@@ -0,0 +1 @@", "only+1 1:package x", "", "old.go", "@@ -120,2 +120,2 @@", "120:a | 120:b", "121:c | 121:c"}
	if got := layout(lines, sv); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if sv.numW != 3 {
		t.Errorf("numW = %d, want 3", sv.numW)
	}
	// Each patch line maps to the row that shows it, a file's or hunk's
	// line to the blank row before it.
	for k, want := range map[int]int{0: 0, 2: 2, 3: 3, 5: 6, 6: 6, 7: 7} {
		if got := rowOf(sv.rows, k); got != want {
			t.Errorf("rowOf(%d) = %d, want %d", k, got, want)
		}
	}
	// The unified rows: every line, with the same blank row.
	var uni []string
	for _, r := range sv.uni {
		if r.gap {
			uni = append(uni, "")
		} else {
			uni = append(uni, lines[r.full].Text)
		}
	}
	if want := []string{"new.go", "@@ -0,0 +1 @@", "package x", "", "old.go", "@@ -120,2 +120,2 @@", "a", "b", "c"}; strings.Join(uni, "|") != strings.Join(want, "|") {
		t.Errorf("unified rows %q, want %q", uni, want)
	}
}

func TestParseHunk(t *testing.T) {
	for s, want := range map[string]hunk{
		"@@ -1,9 +1,14 @@ import x": {1, 9, 1, 14},
		"@@ -0,0 +1 @@":             {0, 0, 1, 1},
		"@@ -5 +5,0 @@":             {5, 1, 5, 0},
	} {
		if got, ok := parseHunk(s); !ok || got != want {
			t.Errorf("parseHunk(%q) = %v %v, want %v", s, got, ok, want)
		}
	}
	if _, ok := parseHunk("@@ bogus @@"); ok {
		t.Error("a bogus header parsed")
	}
}

func splitModel(t *testing.T, w, h int) Model {
	t.Helper()
	m, _ := newModelWith(t, deck.Snapshot{}, w, h, func(o *Options) {
		withPatches(nil)(o)
		o.DiffSplit = true
	})
	m, _ = press(m, snapshotMsg(calm()))
	return m
}

func TestSplitGolden(t *testing.T) {
	cases := []struct {
		name string
		keys []tea.Msg
	}{
		{name: "split", keys: keys("dv")},
		{name: "split-added", keys: keys("djv")},
		{name: "split-scrolled", keys: keys("dvJJJ")},
	}
	for _, c := range cases {
		for _, w := range []int{120, 80, 60} {
			name := fmt.Sprintf("%s-%d", c.name, w)
			t.Run(name, func(t *testing.T) {
				m := splitModel(t, w, 28)
				m, _ = press(m, c.keys...)
				golden(t, name, m)
			})
		}
	}
	// A commit's preview, and one file of it.
	for _, w := range []int{120, 80} {
		m, _ := commitsModel(t, w, 28, sampleCommits(), nil)
		m.split = true
		m, _ = press(m, keys("]]1")...)
		golden(t, fmt.Sprintf("split-commit-%d", w), m)
		m, _ = commitsModel(t, w, 28, sampleCommits(), nil)
		m.split = true
		m, _ = press(m, append(append(keys("]]"), tabKey), append(keys("j j"), enterKey)...)...)
		golden(t, fmt.Sprintf("split-commit-file-%d", w), m)
	}
}

// The split layout keeps the colours: removed lines tinted on the left,
// added ones on the right, code coloured by its language; long lines are
// cut per side.
func TestSplitColoursAndCuts(t *testing.T) {
	m := splitModel(t, 120, 28)
	m, _ = press(m, keys("dv")...)
	out := m.View().Content
	for what, want := range map[string]string{
		"added tint":   "\x1b[" + addBgDark + "m",
		"removed tint": "\x1b[" + delBgDark + "m",
		"keyword":      "\x1b[35mimport",
		"line number":  fmt.Sprintf("\x1b[38;5;%dm 1", lineNoDark),
		"divider":      fmt.Sprintf("\x1b[38;5;%dm│", dividerDark),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s (%q) in the split preview", what, want)
		}
	}
	// Filler is hatched in a dim grey, its number and sign columns blank,
	// and never tinted.
	hatched := 0
	for _, l := range strings.Split(out, "\n") {
		left, _, _ := strings.Cut(l, "│")
		if !strings.Contains(left, "╱") {
			continue
		}
		hatched++
		if !strings.Contains(left, fmt.Sprintf("\x1b[38;5;%dm╱", hatchDark)) {
			t.Errorf("hatching not in grey %d: %q", hatchDark, left)
		}
		if strings.Contains(left, addBgDark) || strings.Contains(left, delBgDark) {
			t.Errorf("tinted hatching: %q", left)
		}
		if plain := ansi.Strip(left); !strings.HasPrefix(plain, "      ╱") || !strings.HasSuffix(plain, "╱ ") {
			t.Errorf("hatching over the number, sign or last column: %q", plain)
		}
	}
	if hatched == 0 {
		t.Error("no hatched filler in the split preview")
	}
	if strings.Contains(out, "\x1b[2m 1") {
		t.Error("line numbers are faint rather than the fixed grey")
	}
	light, _ := press(m, tea.BackgroundColorMsg{Color: color.White})
	out = light.View().Content
	for what, want := range map[string]string{
		"line number": fmt.Sprintf("\x1b[38;5;%dm 1", lineNoLight),
		"divider":     fmt.Sprintf("\x1b[38;5;%dm│", dividerLight),
		"hatching":    fmt.Sprintf("\x1b[38;5;%dm╱", hatchLight),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("light terminal: no %s (%q)", what, want)
		}
	}
	for _, l := range strings.Split(ansi.Strip(out), "\n") {
		left, right, ok := strings.Cut(l, "│")
		if !ok || !strings.Contains(right, "overview cards") {
			continue
		}
		if !strings.HasSuffix(strings.TrimRight(right, " "), "…") || strings.Contains(left, "…") {
			t.Errorf("the long line is not cut on its side: %q", l)
		}
		if w := ansi.StringWidth(left); w != (120-1)/2 {
			t.Errorf("left side %d wide, want %d", w, (120-1)/2)
		}
	}
}

// S switches layouts for the session and keeps the place; the footer
// names the other layout. It never writes the config file.
func TestSplitToggle(t *testing.T) {
	m, _ := previewModel(t, 120, 14, nil)
	m, _ = press(m, keys("dv")...)
	if m.splitShown() || !strings.Contains(screen(m), "S split") {
		t.Fatalf("starts split, or no S in the footer:\n%s", screen(m))
	}
	// Scroll to the blank row before the second hunk's header (patch
	// line 14).
	m, _ = press(m, keys(strings.Repeat("J", 14))...)
	if !atSecondHunk(screen(m)) {
		t.Fatalf("not at the second hunk:\n%s", screen(m))
	}
	m, _ = press(m, keys("S")...)
	if !m.splitShown() || !strings.Contains(screen(m), "S unified") {
		t.Fatalf("S did not switch to split:\n%s", screen(m))
	}
	p := m.patches[m.prevKey]
	if r := p.split.rows[m.prevOff]; r.full != 14 || !r.gap {
		t.Errorf("split: the top row is %+v, want the blank row before the second hunk's header", r)
	}
	if !atSecondHunk(screen(m)) {
		t.Errorf("split lost the place:\n%s", screen(m))
	}
	m, _ = press(m, keys("|")...)
	if m.splitShown() || m.prevOff != 14 {
		t.Errorf("| back to unified: split %v, offset %d, want 14", m.splitShown(), m.prevOff)
	}
	// J K, pgdn and end scroll the split rows.
	m, _ = press(m, keys("S")...)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyHome})
	m, _ = press(m, keys("JJK")...)
	if m.prevOff != 1 {
		t.Errorf("J J K: offset %d", m.prevOff)
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnd})
	if want := max(len(p.split.rows)-m.layout().listH, 0); m.prevOff != want {
		t.Errorf("end: offset %d, want %d", m.prevOff, want)
	}
}

// atSecondHunk says whether the preview starts with the blank row before
// the sample file's second hunk.
func atSecondHunk(s string) bool {
	l := strings.Split(s, "\n")
	return strings.Trim(l[3], " │┃") == "" && strings.HasPrefix(l[4], " @@ -40,3")
}

// A pane under splitMinWidth shows unified and says why; widening it
// brings the split back, near the same place.
func TestSplitNarrowFallback(t *testing.T) {
	m := splitModel(t, 80, 14)
	m, _ = press(m, keys("dv")...)
	if m.splitShown() || !strings.Contains(screen(m), fmt.Sprintf("split needs ≥ %d cols", splitMinWidth)) {
		t.Fatalf("80 columns: no fallback note:\n%s", screen(m))
	}
	if !strings.Contains(screen(m), "S unified") {
		t.Errorf("the footer does not offer unified:\n%s", screen(m))
	}
	m, _ = press(m, keys(strings.Repeat("J", 14))...)
	m, _ = press(m, tea.WindowSizeMsg{Width: splitMinWidth, Height: 14})
	if !m.splitShown() || strings.Contains(screen(m), "split needs") {
		t.Fatalf("%d columns: not split:\n%s", splitMinWidth, screen(m))
	}
	if r := m.patches[m.prevKey].split.rows[m.prevOff]; r.full != 14 || !r.gap {
		t.Errorf("widening lost the place: top row %+v", r)
	}
	m, _ = press(m, tea.WindowSizeMsg{Width: splitMinWidth - 1, Height: 14})
	if m.splitShown() || m.prevOff != 14 {
		t.Errorf("narrowing: split %v, offset %d", m.splitShown(), m.prevOff)
	}
	// Choosing unified drops the note.
	m, _ = press(m, keys("S")...)
	if strings.Contains(screen(m), "split needs") {
		t.Errorf("the note stays with unified chosen:\n%s", screen(m))
	}
}

// ↵ on the settings page cycles diff.layout, and the saved layout applies
// at once.
func TestSettingsCyclesDiffLayout(t *testing.T) {
	e := newSettingsEnv(t, "", nil)
	m := settingsModel(t, e, config.Flags{}, 80, 28)
	m = moveTo(t, m, config.KeyDiffLayout)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := e.file(t); got != "[diff]\nlayout = \"split\"\n" || !m.split {
		t.Fatalf("first press: split %v, file %q", m.split, got)
	}
	if m.Status() != "saved diff.layout = split" {
		t.Errorf("status = %q", m.Status())
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := e.file(t); got != "[diff]\nlayout = \"unified\"\n" || m.split {
		t.Errorf("second press: split %v, file %q", m.split, got)
	}
}

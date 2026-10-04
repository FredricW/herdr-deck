package ui

import (
	"context"
	"fmt"
	"image/color"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/diff"
)

const worktree2 = "/src/worktrees/t-0002"

// samplePatches are made-up diffs of some of changes()' files.
func samplePatches() map[string]deck.Patch {
	tsx := "@@ -1,9 +1,14 @@ import { Page } from \"@acme/ui\";\n" +
		" import { useState } from \"react\";\n" +
		"-import { MembersList } from \"../members/MembersList\";\n" +
		"+import { UsersTable } from \"./UsersTable\";\n" +
		"+import { overview } from \"./overview\";\n" +
		" \n" +
		"-// Members: everyone in the workspace.\n" +
		"-export function MembersPage() {\n" +
		"+// Users page: the table, its filters and the overview cards, which take a very long line to describe.\n" +
		"+export function UsersOverviewPage() {\n" +
		"   const [query, setQuery] = useState(\"\");\n" +
		"+  const stats = overview(users);\n" +
		"   return (\n" +
		"\t\t<Page title=\"Users\">\n" +
		"@@ -40,3 +45,3 @@ function Invite() {\n" +
		"-  return <Button>Invite member</Button>;\n" +
		"+  return <Button>Invite user</Button>;\n" +
		" }\n" +
		"\\ No newline at end of file\n"
	var long strings.Builder
	long.WriteString("@@ -0,0 +1,40 @@\n")
	for i := range 40 {
		fmt.Fprintf(&long, "+  \"pkg-%02d\": \"^1.%d.0\",\n", i, i)
	}
	capped := diff.ParsePatch([]byte(long.String()), 30)
	return map[string]deck.Patch{
		"apps/admin/src/pages/users/UsersOverviewPage.tsx": diff.ParsePatch([]byte(tsx), diff.MaxPatchLines),
		"apps/admin/src/pages/users/columns.ts":            diff.ParsePatch([]byte("@@ -0,0 +1 @@\n+export const columns = [];\n"), diff.MaxPatchLines),
		"apps/admin/public/empty-state.png":                {Binary: true},
		"pnpm-lock.yaml":                                   capped,
		"apps/admin/src/pages/users/index.ts":              {Note: "renamed, content unchanged"},
	}
}

// withPatches gives the model changes() for t-0002 and a Patch that
// answers from samplePatches, recording the files it was asked for.
func withPatches(calls *[]string) func(*Options) {
	patches := samplePatches()
	return func(o *Options) {
		withDiffs(map[string]deck.Diff{worktree2: changes()}, nil)(o)
		o.Patch = func(_ context.Context, t deck.Thread, base string, f deck.DiffFile) deck.Patch {
			if calls != nil {
				*calls = append(*calls, f.Path)
			}
			if base != "4b825dc" {
				return deck.Patch{Note: "wrong base " + base}
			}
			return patches[f.Path]
		}
	}
}

func previewModel(t *testing.T, w, h int, calls *[]string) (Model, *opened) {
	t.Helper()
	m, o := newModelWith(t, deck.Snapshot{}, w, h, withPatches(calls))
	m, _ = press(m, snapshotMsg(calm()))
	return m, o
}

func TestPreviewGolden(t *testing.T) {
	cases := []struct {
		name string
		keys []tea.Msg
	}{
		{name: "preview", keys: keys("dv")},
		{name: "preview-scrolled", keys: keys("dvJJJ")},
		{name: "preview-binary", keys: keys("djjjjv")},
		{name: "preview-cap", keys: append(keys("dv"), append(keys("jjjjjjjj"), tea.KeyPressMsg{Code: tea.KeyPgDown})...)},
		{name: "preview-off", keys: keys("dvJv")},
	}
	for _, c := range cases {
		for _, w := range []int{80, 60} {
			name := fmt.Sprintf("%s-%d", c.name, w)
			t.Run(name, func(t *testing.T) {
				m, _ := previewModel(t, w, 28, nil)
				m, _ = press(m, c.keys...)
				golden(t, name, m)
			})
		}
	}
}

func TestPreviewShowsAndFollowsFile(t *testing.T) {
	var calls []string
	m, o := previewModel(t, 80, 28, &calls)
	m, _ = press(m, keys("dv")...)
	if !m.preview || !strings.Contains(screen(m), "UsersOverviewPage.tsx  +214 −12") {
		t.Fatalf("no preview:\n%s", screen(m))
	}
	if strings.Contains(screen(m), "WORK") {
		t.Errorf("the list shows under the preview:\n%s", screen(m))
	}
	// j moves the drawer cursor; the preview follows it and reads the
	// new file once.
	m, _ = press(m, keys("j")...)
	if !strings.Contains(screen(m), "+export const columns = [];") && !strings.Contains(screen(m), "+ export const columns = [];") {
		t.Errorf("the preview did not follow j:\n%s", screen(m))
	}
	if want := []string{"apps/admin/src/pages/users/UsersOverviewPage.tsx", "apps/admin/src/pages/users/columns.ts"}; !slices.Equal(calls, want) {
		t.Errorf("reads %q, want %q", calls, want)
	}
	// A reload reads the shown file again, so the preview keeps up.
	m, _ = press(m, snapshotMsg(calm()))
	if len(calls) != 3 || calls[2] != "apps/admin/src/pages/users/columns.ts" {
		t.Errorf("a reload read %q", calls)
	}
	// A click on a file previews it rather than opening the diff tool.
	x, y := find(t, m, "routes.tsx")
	m, _ = press(m, click(x, y))
	if len(o.diffs) != 0 || !strings.Contains(screen(m), "routes.tsx  +6 −2") {
		t.Errorf("click: opened %q\n%s", o.diffs, screen(m))
	}
	// enter previews too; d opens the file in the diff tool.
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(o.diffs) != 0 || !m.preview {
		t.Errorf("enter: opened %q, preview %v", o.diffs, m.preview)
	}
	m, _ = press(m, keys("d")...)
	if want := []string{worktree2 + " 4b825dc apps/admin/src/routes.tsx"}; !slices.Equal(o.diffs, want) || !m.preview {
		t.Errorf("d opened %q, want %q", o.diffs, want)
	}
	// A rename's note and a binary file's line.
	m, _ = press(m, keys("kk")...)
	if !strings.Contains(screen(m), "renamed, content unchanged") {
		t.Errorf("rename:\n%s", screen(m))
	}
}

func TestPreviewColours(t *testing.T) {
	m, _ := previewModel(t, 80, 28, nil)
	m, _ = press(m, keys("dv")...)
	out := m.View().Content
	for what, want := range map[string]string{
		"added tint":   "\x1b[" + addBgDark + "m",
		"removed tint": "\x1b[" + delBgDark + "m",
		"keyword":      "\x1b[35mimport",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s (%q) in the preview", what, want)
		}
	}
	m, _ = press(m, tea.BackgroundColorMsg{Color: color.White})
	if out := m.View().Content; !strings.Contains(out, addBgLight) || strings.Contains(out, addBgDark) {
		t.Error("a light terminal keeps the dark tint")
	}
}

func TestPreviewScrolls(t *testing.T) {
	m, _ := previewModel(t, 80, 28, nil)
	m, _ = press(m, keys("djjjjjjjjv")...) // pnpm-lock.yaml: 30 lines and a cap
	if !strings.Contains(screen(m), "pnpm-lock.yaml") {
		t.Fatalf("not on the lock file:\n%s", screen(m))
	}
	h := m.layout().listH
	m, _ = press(m, keys("JJ")...)
	m, _ = press(m, keys("K")...)
	if m.prevOff != 1 {
		t.Errorf("J J K: offset %d, want 1", m.prevOff)
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	if m.prevOff != 1+h-1 {
		t.Errorf("pgdn: offset %d, want %d", m.prevOff, h)
	}
	// The wheel over the preview scrolls it, not the list.
	cursor := m.cursor
	m, _ = press(m, tea.MouseWheelMsg{Button: tea.MouseWheelDown, Y: 5})
	if m.prevOff != h+3 || m.cursor != cursor {
		t.Errorf("wheel: offset %d cursor %d", m.prevOff, m.cursor)
	}
	// The end is the cap's line: 31 lines (30 and "… 11 more lines").
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnd})
	if m.prevOff != 31-h || !strings.Contains(screen(m), "… 11 more lines") {
		t.Errorf("end: offset %d\n%s", m.prevOff, screen(m))
	}
	// Another file starts at its top.
	m, _ = press(m, keys("k")...)
	if m.prevOff != 0 {
		t.Errorf("another file: offset %d", m.prevOff)
	}
}

// The list's cursor and scroll are as they were when the preview goes,
// however it goes.
func TestPreviewRestoresList(t *testing.T) {
	s := calm()
	for i := range 20 {
		s.TaskLists[0].Tasks = append(s.TaskLists[0].Tasks, deck.Task{Title: fmt.Sprintf("Task %02d", i)})
	}
	leave := map[string][]tea.Msg{
		"v":   keys("v"),
		"esc": {esc},
		"]":   keys("]"),
		"tab": {tea.KeyPressMsg{Code: tea.KeyTab}},
		"z":   keys("z"),
		"?":   keys("?"),
	}
	for name, msgs := range leave {
		t.Run(name, func(t *testing.T) {
			m, _ := newModelWith(t, deck.Snapshot{}, 80, 22, withPatches(nil))
			m, _ = press(m, snapshotMsg(s))
			// Scroll the list so the cursor's row is t-0002 near the
			// bottom of the view.
			m, _ = press(m, keys(strings.Repeat("j", 30))...)
			m, _ = press(m, keys(strings.Repeat("k", 30))...)
			for selectedTitle(m) != "Users page" {
				m, _ = press(m, keys("j")...)
			}
			cursor, off := m.cursor, m.listOff
			before := screen(m)
			m, _ = press(m, keys("d")...)
			beforeFiles := screen(m)
			m, _ = press(m, keys("vJJjjJ")...)
			if !m.preview {
				t.Fatal("no preview")
			}
			m, _ = press(m, msgs...)
			if m.preview {
				t.Fatalf("%s left the preview on", name)
			}
			if m.cursor != cursor || m.listOff != off {
				t.Errorf("cursor %d off %d, want %d %d", m.cursor, m.listOff, cursor, off)
			}
			if name == "v" {
				// Only the drawer's cursor moved (two files down).
				if got := screen(m); strings.Split(got, "\n")[3] != strings.Split(beforeFiles, "\n")[3] {
					t.Errorf("the list changed:\n%s\nwas\n%s", got, before)
				}
			}
		})
	}
}

func TestPreviewEndsWhenThreadChanges(t *testing.T) {
	m, _ := previewModel(t, 80, 28, nil)
	m, _ = press(m, keys("dv")...)
	// A reload whose needs-you row pulls the cursor to another thread.
	s := calm()
	s.Threads[2].Status = deck.StatusNeedsYou
	m.moved = false
	m, _ = press(m, snapshotMsg(s))
	if selectedTitle(m) == "Users page" {
		t.Skip("the cursor stayed")
	}
	if m.preview {
		t.Errorf("the preview stayed on another thread:\n%s", screen(m))
	}
}

func TestPreviewKeyEdges(t *testing.T) {
	// Without Patch, v says the preview is off.
	m, _ := newModelWith(t, deck.Snapshot{}, 80, 28, withDiffs(map[string]deck.Diff{worktree2: changes()}, nil))
	m, _ = press(m, snapshotMsg(calm()))
	m, _ = press(m, keys("dv")...)
	if m.preview || m.Status() != "the diff preview is off" {
		t.Errorf("no Patch: preview %v, status %q", m.preview, m.Status())
	}
	// v outside Files does nothing; on Files with no changes, it says so.
	m, _ = newModelWith(t, deck.Snapshot{}, 80, 28, withPatches(nil))
	m, _ = press(m, snapshotMsg(calm()))
	m, _ = press(m, keys("v")...)
	if m.preview {
		t.Error("v on Overview turned the preview on")
	}
	m.diffs[diffKey(m.snap.Threads[1])] = deck.Diff{Base: "origin/main"}
	m, _ = press(m, keys("dv")...)
	if m.preview || m.Status() != "no file to preview" {
		t.Errorf("no files: preview %v, status %q", m.preview, m.Status())
	}
	// From the full-height drawer, v gives the drawer its normal height.
	m, _ = previewModel(t, 80, 28, nil)
	m, _ = press(m, keys("zdv")...)
	if !m.preview || m.size != sizeNormal {
		t.Errorf("from full height: preview %v, size %v", m.preview, m.size)
	}
	// While the preview shows, d opens the file in the diff tool and a the
	// whole diff, and the preview stays.
	m, o := previewModel(t, 80, 28, nil)
	m, _ = press(m, keys("dvda")...)
	if want := []string{worktree2 + " 4b825dc apps/admin/src/pages/users/UsersOverviewPage.tsx", worktree2 + " 4b825dc"}; !m.preview || !slices.Equal(o.diffs, want) {
		t.Errorf("d a in the preview: preview %v, opened %q", m.preview, o.diffs)
	}
}

func TestDiffLineCutsLongLines(t *testing.T) {
	m, _ := previewModel(t, 60, 28, nil)
	l := deck.PatchLine{Kind: deck.LineAdded, Text: strings.Repeat("x", 100)}
	got := m.diffLine(l, l.Text, 60)
	if !strings.Contains(got, "…") {
		t.Errorf("no … on a long line: %q", got)
	}
	if w := len([]rune(ansi.Strip(got))); w != 60 {
		t.Errorf("width %d, want 60", w)
	}
}

// With the preview, the Files tab previews by default: d then a digit, a
// click or ↵, in the tree's numbering in the tree view; d d opens the file
// under the cursor (the first, after t) in the diff tool.
func TestFilesPreviewByDefault(t *testing.T) {
	m, o := previewModel(t, 80, 28, nil)
	m, _ = press(m, keys("d2")...)
	if !m.preview || len(o.diffs) != 0 || !strings.Contains(screen(m), "columns.ts  +48 −0") {
		t.Fatalf("d 2: preview %v, opened %q\n%s", m.preview, o.diffs, screen(m))
	}
	m, _ = press(m, esc)
	m, _ = press(m, keys("t2")...)
	if !m.preview || !strings.Contains(screen(m), "api/users.ts  +31 −9") {
		t.Errorf("t 2 did not preview the tree's second file:\n%s", screen(m))
	}
	m, _ = press(m, esc)
	m, _ = press(m, keys("tdd")...)
	if want := []string{worktree2 + " 4b825dc apps/admin/src/pages/users/UsersOverviewPage.tsx"}; !slices.Equal(o.diffs, want) {
		t.Errorf("d d opened %q, want %q (t puts the cursor on file 1)", o.diffs, want)
	}
	// On Commits, a opens the branch's whole diff in the diff tool too.
	m, _ = commitsModel(t, 80, 28, sampleCommits(), nil)
	m, _ = press(m, keys("]]a")...)
	if !strings.Contains(m.Status(), "opened the diff of t-0002") {
		t.Errorf("a on Commits: %q", m.Status())
	}
}

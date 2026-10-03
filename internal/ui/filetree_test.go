package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// treeText is the tree's lines as "depth name" text: folders end in /.
func treeText(files []deck.DiffFile) []string {
	var out []string
	for _, l := range fileTree(files) {
		name := ""
		if l.dir != nil {
			name = l.dir.name + "/"
		} else {
			name = treeName(files[l.file])
		}
		out = append(out, strings.Repeat("  ", l.depth)+name)
	}
	return out
}

func TestFileTree(t *testing.T) {
	got := treeText(changes().Files)
	want := []string{
		"apps/admin/",
		"  public/",
		"    empty-state.png",
		"  src/",
		"    api/",
		"      users.ts",
		"    pages/users/",
		"      UsersOverviewPage.tsx",
		"      columns.ts",
		"      apps/admin/src/pages/members/index.ts → index.ts",
		"    routes.tsx",
		"docs/",
		"  users-page.md",
		"scripts/",
		"  seed-users.ts",
		"package.json",
		"pnpm-lock.yaml",
		"tsconfig.json",
	}
	if !slices.Equal(got, want) {
		t.Errorf("tree =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestFileTreeEdges(t *testing.T) {
	files := []deck.DiffFile{
		{Path: "z.go"},
		{Path: "a/b/c/deep.go", Added: 2},
		{Path: "a/b/c/d/deeper.go", Added: 1, Deleted: 4},
		{Path: "lib/old.go", OldPath: "lib/older.go"},
		{Path: "a.go"},
	}
	got := treeText(files)
	// a/b/c holds a file, so the chain stops there; lib's rename stays in
	// one folder and shows both base names.
	want := []string{
		"a/b/c/",
		"  d/",
		"    deeper.go",
		"  deep.go",
		"lib/",
		"  older.go → old.go",
		"a.go",
		"z.go",
	}
	if !slices.Equal(got, want) {
		t.Errorf("tree =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	l := fileTree(files)[0]
	if l.dir.added != 3 || l.dir.deleted != 4 {
		t.Errorf("a/b/c sums +%d -%d, want +3 -4", l.dir.added, l.dir.deleted)
	}
	// A collapsed chain sorts by its shown name: "a-b" < "a/x".
	if got := treeText([]deck.DiffFile{{Path: "a/x/f.go"}, {Path: "a-b/g.go"}}); !slices.Equal(got, []string{"a-b/", "  g.go", "a/x/", "  f.go"}) {
		t.Errorf("collapsed sort = %q", got)
	}
	if fileTree(nil) != nil {
		t.Error("an empty diff has tree lines")
	}
}

func TestFileOrder(t *testing.T) {
	files := changes().Files
	if got := fileOrder(files, false); !slices.Equal(paths(got), paths(files)) {
		t.Errorf("list order changed: %q", paths(got))
	}
	got := paths(fileOrder(files, true))
	want := []string{
		"apps/admin/public/empty-state.png",
		"apps/admin/src/api/users.ts",
		"apps/admin/src/pages/users/UsersOverviewPage.tsx",
		"apps/admin/src/pages/users/columns.ts",
		"apps/admin/src/pages/users/index.ts",
		"apps/admin/src/routes.tsx",
		"docs/users-page.md",
		"scripts/seed-users.ts",
		"package.json",
		"pnpm-lock.yaml",
		"tsconfig.json",
	}
	if !slices.Equal(got, want) {
		t.Errorf("tree order = %q", got)
	}
}

func paths(files []deck.DiffFile) []string {
	var out []string
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

func TestFileCountsByChange(t *testing.T) {
	tests := []struct {
		f    deck.DiffFile
		want string
	}{
		{deck.DiffFile{Added: 3, Deleted: 1}, "+3 -1"},
		{deck.DiffFile{Added: 3}, "+3 -0"},
		{deck.DiffFile{Change: deck.ChangeAdded, Added: 3}, "+3"},
		{deck.DiffFile{Change: deck.ChangeDeleted, Deleted: 7}, "-7"},
		{deck.DiffFile{Change: deck.ChangeRenamed, OldPath: "a", Added: 1, Deleted: 2}, "+1 -2"},
		{deck.DiffFile{Change: deck.ChangeAdded, Added: 4, Untracked: true}, "+4 untracked"},
		{deck.DiffFile{Change: deck.ChangeAdded, Binary: true}, "binary"},
		{deck.DiffFile{Change: deck.ChangeAdded, Binary: true, Untracked: true}, "binary untracked"},
	}
	for _, tt := range tests {
		if got, _ := fileCounts(tt.f); got != tt.want {
			t.Errorf("fileCounts(%+v) = %q, want %q", tt.f, got, tt.want)
		}
	}
	if got, _ := sumCounts(0, 0); got != "" {
		t.Errorf("sumCounts(0, 0) = %q", got)
	}
	if got, _ := sumCounts(0, 5); got != "-5" {
		t.Errorf("sumCounts(0, 5) = %q", got)
	}
}

// The marks and counts carry the terminal's own named colours: green 32,
// yellow 33, red 31, cyan 36, and faint 2 for untracked, binary and folder
// sums.
func TestFileColours(t *testing.T) {
	marks := []struct {
		f    deck.DiffFile
		want string
	}{
		{deck.DiffFile{Change: deck.ChangeAdded}, "\x1b[32mA"},
		{deck.DiffFile{}, "\x1b[33mM"},
		{deck.DiffFile{Change: deck.ChangeDeleted}, "\x1b[31mD"},
		{deck.DiffFile{Change: deck.ChangeRenamed, OldPath: "a"}, "\x1b[36mR"},
		{deck.DiffFile{Change: deck.ChangeAdded, Untracked: true}, "\x1b[2;32m?"},
		{deck.DiffFile{Change: deck.ChangeAdded, Binary: true}, "\x1b[2;32mA"},
	}
	for _, tt := range marks {
		if got := changeMark(tt.f); !strings.HasPrefix(got, tt.want) {
			t.Errorf("changeMark(%+v) = %q, want it to start %q", tt.f, got, tt.want)
		}
	}
	_, styled := fileCounts(deck.DiffFile{Added: 3, Deleted: 1})
	if !strings.Contains(styled, "\x1b[32m+3") || !strings.Contains(styled, "\x1b[31m-1") {
		t.Errorf("counts %q are not green and red", styled)
	}
	_, styled = sumCounts(3, 1)
	if !strings.Contains(styled, "\x1b[2;32m+3") || !strings.Contains(styled, "\x1b[2;31m-1") {
		t.Errorf("folder sums %q are not faint green and red", styled)
	}
}

package ui

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// The Files section's change colours: added green, modified yellow,
// deleted red, renamed cyan (the palette's other neutral colour, next to
// magenta for review), untracked green but faint. Binary files are faint
// in their change's colour. Line counts are green and red; a folder's sums
// in the tree view are faint.
var (
	addedStyle    = lipgloss.NewStyle().Foreground(colGreen)
	modifiedStyle = lipgloss.NewStyle().Foreground(colYellow)
	deletedStyle  = lipgloss.NewStyle().Foreground(colRed)
	renamedStyle  = lipgloss.NewStyle().Foreground(colCyan)
	sumAddStyle   = addedStyle.Faint(true)
	sumDelStyle   = deletedStyle.Faint(true)
)

// changeMark is a changed file's one-letter status, as git status writes
// it, in its colour.
func changeMark(f deck.DiffFile) string {
	mark, st := "M", modifiedStyle
	switch {
	case f.Untracked:
		mark, st = "?", addedStyle.Faint(true)
	case f.Change == deck.ChangeAdded:
		mark, st = "A", addedStyle
	case f.Change == deck.ChangeDeleted:
		mark, st = "D", deletedStyle
	case f.Change == deck.ChangeRenamed || f.OldPath != "":
		mark, st = "R", renamedStyle
	}
	if f.Binary {
		st = st.Faint(true)
	}
	return st.Render(mark)
}

// fileCounts is a changed file's counts, as plain text and styled: "+12"
// for an added or untracked file, "-3" for a deleted one, "+12 -3"
// otherwise; "binary" for a binary file.
func fileCounts(f deck.DiffFile) (text, styled string) {
	var parts []item
	switch {
	case f.Binary:
		parts = append(parts, span("binary", dim))
	case f.Untracked || f.Change == deck.ChangeAdded:
		parts = append(parts, span(fmt.Sprintf("+%d", f.Added), addedStyle))
	case f.Change == deck.ChangeDeleted:
		parts = append(parts, span(fmt.Sprintf("-%d", f.Deleted), deletedStyle))
	default:
		parts = append(parts, span(fmt.Sprintf("+%d", f.Added), addedStyle), span(fmt.Sprintf("-%d", f.Deleted), deletedStyle))
	}
	return joinItems(parts)
}

// sumCounts is a folder's summed counts, faint, leaving out a zero side;
// "" when its files count no lines.
func sumCounts(added, deleted int) (text, styled string) {
	var parts []item
	if added > 0 {
		parts = append(parts, span(fmt.Sprintf("+%d", added), sumAddStyle))
	}
	if deleted > 0 {
		parts = append(parts, span(fmt.Sprintf("-%d", deleted), sumDelStyle))
	}
	return joinItems(parts)
}

func joinItems(parts []item) (text, styled string) {
	texts := make([]string, len(parts))
	styles := make([]string, len(parts))
	for i, p := range parts {
		texts[i], styles[i] = p.text, p.style.Render(p.text)
	}
	return strings.Join(texts, " "), strings.Join(styles, " ")
}

// fileDir is a folder in the Files section's tree view. Its name is its
// path below its parent: a chain of folders that each hold just one
// folder is one fileDir, such as "src/admin/users".
type fileDir struct {
	name           string
	dirs           []*fileDir
	files          []int // indexes into the diff's files
	added, deleted int   // every file below it
}

// treeLine is one line of the tree view: a folder, or with dir nil a file.
type treeLine struct {
	depth int
	dir   *fileDir
	file  int // index into the diff's files
}

// fileTree puts files under their folders (a rename under its new path),
// with single-child chains collapsed, and lists the lines in display
// order: at each level folders first, then files, each by name.
func fileTree(files []deck.DiffFile) []treeLine {
	root := &fileDir{}
	for i, f := range files {
		d := root
		parts := strings.Split(f.Path, "/")
		for _, name := range parts[:len(parts)-1] {
			var sub *fileDir
			for _, c := range d.dirs {
				if c.name == name {
					sub = c
					break
				}
			}
			if sub == nil {
				sub = &fileDir{name: name}
				d.dirs = append(d.dirs, sub)
			}
			d.added += f.Added
			d.deleted += f.Deleted
			d = sub
		}
		d.added += f.Added
		d.deleted += f.Deleted
		d.files = append(d.files, i)
	}
	var lines []treeLine
	var walk func(d *fileDir, depth int)
	walk = func(d *fileDir, depth int) {
		// Chains collapse first, so folders sort by the name they show.
		for _, c := range d.dirs {
			for len(c.files) == 0 && len(c.dirs) == 1 {
				only := c.dirs[0]
				c.name, c.dirs, c.files = c.name+"/"+only.name, only.dirs, only.files
			}
		}
		sort.Slice(d.dirs, func(i, j int) bool { return d.dirs[i].name < d.dirs[j].name })
		for _, c := range d.dirs {
			lines = append(lines, treeLine{depth: depth, dir: c})
			walk(c, depth+1)
		}
		sort.SliceStable(d.files, func(i, j int) bool {
			return path.Base(files[d.files[i]].Path) < path.Base(files[d.files[j]].Path)
		})
		for _, i := range d.files {
			lines = append(lines, treeLine{depth: depth, file: i})
		}
	}
	walk(root, 0)
	return lines
}

// fileOrder is the files in the order the Files section shows and numbers
// them: as the diff lists them, or in the tree view the tree's order.
func fileOrder(files []deck.DiffFile, tree bool) []deck.DiffFile {
	if !tree {
		return files
	}
	out := make([]deck.DiffFile, 0, len(files))
	for _, l := range fileTree(files) {
		if l.dir == nil {
			out = append(out, files[l.file])
		}
	}
	return out
}

// treeName is how the tree view names a file below its folder: its base
// name, and for a rename the old name too, in full when it moved folders.
func treeName(f deck.DiffFile) string {
	name := path.Base(f.Path)
	if f.OldPath == "" {
		return name
	}
	old := f.OldPath
	if path.Dir(old) == path.Dir(f.Path) {
		old = path.Base(old)
	}
	return old + " → " + name
}

// filesTab is the Files tab: a total line, then every changed file as a
// diffstat row, in a list or a folder tree. Files 1-9 take the digits;
// every file is a stop of the drawer cursor.
func (m Model) filesTab(d *drawer) {
	_, df, ok := m.diff()
	switch {
	case !ok:
		d.line(" " + dim.Render("reading the changes…"))
		return
	case df.Note != "":
		d.line(" " + dim.Render(df.Note))
		return
	case len(df.Files) == 0:
		d.line(" " + dim.Render("no changes against "+df.Base))
		return
	}
	added, deleted := df.Totals()
	noun := "files"
	if len(df.Files) == 1 {
		noun = "file"
	}
	view := "list · t tree"
	if m.tree {
		view = "tree · t list"
	}
	total := " " + fmt.Sprintf("%d %s", len(df.Files), noun) + "  " + addedStyle.Render(fmt.Sprintf("+%d", added)) +
		" " + deletedStyle.Render(fmt.Sprintf("-%d", deleted)) + "  " + dim.Render("vs "+df.Base)
	d.lines = append(d.lines, dline{
		text:  spread(total, dim.Render(view)+" ", d.width),
		zones: []zone{{x0: 0, x1: d.width, act: action{kind: actDiff}}},
	})
	big := 0
	for _, f := range df.Files {
		if !f.Binary {
			big = max(big, f.Added+f.Deleted)
		}
	}
	if !m.tree {
		for i, f := range df.Files {
			d.fileLine(i+1, f, -1, listName(f), big)
		}
		return
	}
	n := 0
	for _, l := range fileTree(df.Files) {
		if l.dir != nil {
			d.dirLine(l.dir, l.depth)
			continue
		}
		n++
		d.fileLine(n, df.Files[l.file], l.depth, treeName(df.Files[l.file]), big)
	}
}

// listName is a file's path in the list view; a rename folds the parts
// its old and new path share, as git diff --stat does:
// src/pages/{members → users}/index.ts.
func listName(f deck.DiffFile) string {
	if f.OldPath == "" {
		return f.Path
	}
	o, n := strings.Split(f.OldPath, "/"), strings.Split(f.Path, "/")
	pre := 0
	for pre < len(o)-1 && pre < len(n)-1 && o[pre] == n[pre] {
		pre++
	}
	suf := 0
	for suf < len(o)-pre && suf < len(n)-pre && o[len(o)-1-suf] == n[len(n)-1-suf] {
		suf++
	}
	mid := "{" + strings.Join(o[pre:len(o)-suf], "/") + " → " + strings.Join(n[pre:len(n)-suf], "/") + "}"
	parts := append(append(append([]string(nil), n[:pre]...), mid), n[len(n)-suf:]...)
	return strings.Join(parts, "/")
}

// barWidth is the diffstat bar's cells: 8 at 80 columns, 5 at 60.
func (d *drawer) barWidth() int {
	if d.width < wideMin {
		return 5
	}
	return 8
}

// diffBar is a file's share of the thread's largest change (big lines):
// green cells for added lines, then red for deleted, at least one for any
// change, the rest a dim ▁.
func diffBar(f deck.DiffFile, big, cells int) string {
	total := f.Added + f.Deleted
	if f.Binary || total == 0 || big == 0 {
		return strings.Repeat(" ", cells)
	}
	n := clamp((total*cells+big-1)/big, 1, cells)
	a := (2*n*f.Added + total) / (2 * total) // rounded
	switch {
	case f.Deleted > 0 && a == n && n > 1:
		a = n - 1
	case f.Added > 0 && a == 0:
		a = 1
	}
	return addedStyle.Render(strings.Repeat("▇", a)) + deletedStyle.Render(strings.Repeat("▇", n-a)) +
		dim.Render(strings.Repeat("▁", cells-n))
}

// fileLine is one changed file: its digit (a dim · after 9), change letter
// and name, indented by depth in the tree view (depth -1 is the list
// view), with its counts and bar at the right edge. A long name loses its
// start, so the file's own name stays.
func (d *drawer) fileLine(n int, f deck.DiffFile, depth int, name string, big int) {
	num := dim.Render("·")
	if n <= 9 {
		num = bold.Render(fmt.Sprint(n))
	}
	lead := " " + num + " " + changeMark(f) + "  "
	leadW := 6
	if depth >= 0 {
		lead += " " + strings.Repeat(" ", 2*depth)
		leadW += 1 + 2*depth
	}
	ctext, counts := fileCounts(f)
	cells := d.barWidth()
	right := counts + "  " + diffBar(f, big, cells) + " "
	rightW := ansi.StringWidth(ctext) + 2 + cells + 1
	room := max(d.width-leadW-rightW-2, 4)
	name = truncateLeft(name, room)
	shown := plain.Render(name)
	if dir, base := path.Split(name); depth < 0 && dir != "" {
		shown = dim.Render(dir) + plain.Render(base)
	}
	gap := max(d.width-leadW-ansi.StringWidth(name)-rightW, 1)
	text := lead + shown + strings.Repeat(" ", gap) + right
	act := action{kind: actFile, n: n}
	d.stopLine(text, act, []zone{{x0: 0, x1: d.width, act: act}})
}

// dirLine is a folder in the tree view, lined up with the file names: its
// name faint, so the changed files stand out, and its summed counts faint
// where the files' counts sit.
func (d *drawer) dirLine(dir *fileDir, depth int) {
	ctext, counts := sumCounts(dir.added, dir.deleted)
	leadW := 7 + 2*depth
	rightW := ansi.StringWidth(ctext) + 2 + d.barWidth() + 1
	room := max(d.width-leadW-rightW-2, 4)
	name := truncateLeft(dir.name+"/", room)
	gap := max(d.width-leadW-ansi.StringWidth(name)-rightW, 1)
	text := strings.Repeat(" ", leadW) + dim.Render(name) + strings.Repeat(" ", gap) + counts + strings.Repeat(" ", 2+d.barWidth()+1)
	d.lines = append(d.lines, dline{text: fit(text, d.width)})
}

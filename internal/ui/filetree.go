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
// for an added file, "-3" for a deleted one, "+12 -3" otherwise; "binary",
// and "untracked" after a file git does not track yet.
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
	if f.Untracked {
		parts = append(parts, span("untracked", dim))
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
		sort.Slice(d.dirs, func(i, j int) bool { return d.dirs[i].name < d.dirs[j].name })
		for _, c := range d.dirs {
			for len(c.files) == 0 && len(c.dirs) == 1 {
				only := c.dirs[0]
				c.name, c.dirs, c.files = c.name+"/"+only.name, only.dirs, only.files
			}
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

// fileLine is one numbered changed file: its number, change mark and name,
// indented by depth in the tree view, with its counts at the right edge.
// A long name loses its start, so the file's own name stays.
func (d *drawer) fileLine(n int, f deck.DiffFile, depth int, name string) {
	x0 := 1 + d.labelW
	avail := d.width - x0 - 1
	ctext, counts := fileCounts(f)
	num := fmt.Sprintf("%d ", n)
	lead := len(num) + 2 + 2*depth
	room := max(avail-lead-2-ansi.StringWidth(ctext), 4)
	name = truncateLeft(name, room)
	gap := max(avail-lead-ansi.StringWidth(name)-ansi.StringWidth(ctext), 1)
	text := strings.Repeat(" ", x0) + num + changeMark(f) + " " + strings.Repeat(" ", 2*depth) +
		plain.Render(name) + strings.Repeat(" ", gap) + counts
	d.lines = append(d.lines, dline{
		text:  ansi.Truncate(text, d.width, "…"),
		zones: []zone{{x0: x0, x1: d.width, link: -1, file: n}},
	})
}

// dirLine is a folder in the tree view, lined up with the file names, with
// its summed counts faint at the right edge.
func (d *drawer) dirLine(dir *fileDir, depth int) {
	x0 := 1 + d.labelW
	avail := d.width - x0 - 1
	ctext, counts := sumCounts(dir.added, dir.deleted)
	lead := 4 + 2*depth // under "1 M "
	room := max(avail-lead-2-ansi.StringWidth(ctext), 4)
	name := truncateLeft(dir.name+"/", room)
	gap := max(avail-lead-ansi.StringWidth(name)-ansi.StringWidth(ctext), 1)
	text := strings.Repeat(" ", x0+lead) + plain.Render(name) + strings.Repeat(" ", gap) + counts
	d.lines = append(d.lines, dline{text: ansi.Truncate(text, d.width, "…")})
}

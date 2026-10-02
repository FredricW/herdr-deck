// Package tasks reads a project's TASKS.md loosely and scrapes links from its
// tasks and their threads.
//
// herdr-projects expects `## <List>` headings over `- [ ] Title (owner) ·
// t-0007` lines with notes indented two spaces, but coordinators write the
// file freely: plain `- ` bullets, "thread t-0002" in prose, paragraphs
// between lists. Both shapes parse; anything else degrades to notes.
package tasks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// Options tunes parsing and link scraping.
type Options struct {
	// LinearWorkspace is the slug bare Linear IDs link into; empty leaves
	// them without a URL.
	LinearWorkspace string
	// ThreadText returns the text of a thread's brief and report, scraped
	// for links of tasks that name the thread. Nil scrapes nothing extra.
	ThreadText func(id string) string
}

// File is a parsed TASKS.md.
type File struct {
	// Note is the paragraph text above the first list.
	Note  string
	Lists []deck.TaskList
}

var (
	heading  = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	bullet   = regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s+(.*)$`)
	checkbox = regexp.MustCompile(`^\[([ xX])\]\s*`)
	threadID = regexp.MustCompile(`\bt-\d{4}\b`)
	suffix   = regexp.MustCompile(`\s*·\s*t-\d{4}(?:\s*,\s*t-\d{4})*\s*$`)
	owner    = regexp.MustCompile(`\s*[(\[]([^()\[\]\s/,:]+)[)\]]\s*$`)
	parens   = regexp.MustCompile(`\s*\([^()]*\)`)
	tailRefs = regexp.MustCompile(`(?:[\s,;]*(?:[A-Z][A-Z0-9]{1,5}-\d+|t-\d{4}))+\s*$`)
)

// Load reads <projectDir>/TASKS.md and scrapes the linked threads' files under
// <projectDir>/threads unless opt.ThreadText is set.
func Load(projectDir string, opt Options) (File, error) {
	src, err := os.ReadFile(filepath.Join(projectDir, "TASKS.md"))
	if err != nil {
		return File{}, err
	}
	if opt.ThreadText == nil {
		opt.ThreadText = ThreadFiles(filepath.Join(projectDir, "threads"))
	}
	return Parse(string(src), opt), nil
}

// ThreadFiles returns a ThreadText that reads t-NNNN.task.md and t-NNNN.md
// from dir. Missing files read as empty.
func ThreadFiles(dir string) func(id string) string {
	return func(id string) string {
		var b strings.Builder
		for _, name := range []string{id + ".task.md", id + ".md"} {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			b.Write(data)
			b.WriteByte('\n')
		}
		return b.String()
	}
}

// item is a list item still collecting its notes.
type item struct {
	text  string // the line after the bullet
	notes []string
}

type parser struct {
	opt   Options
	file  File
	list  *deck.TaskList // nil before the first ## heading
	item  *item
	para  []string // lines of the paragraph being read
	notes []string // finished paragraphs of the current list or preamble
}

// Parse reads TASKS.md text. It never fails: lines it does not understand
// become notes.
func Parse(src string, opt Options) File {
	p := &parser{opt: opt}
	for _, line := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		p.line(line)
	}
	p.endList()
	return p.file
}

func (p *parser) line(line string) {
	trimmed := strings.TrimSpace(line)
	indented := strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t")

	switch {
	case trimmed == "":
		p.endPara()
	case indented && p.item != nil:
		p.item.notes = append(p.item.notes, trimmed)
	case heading.MatchString(trimmed):
		m := heading.FindStringSubmatch(trimmed)
		switch len(m[1]) {
		case 1: // the file's title
			p.endItem()
			p.endPara()
		case 2:
			p.endList()
			p.list = &deck.TaskList{Name: m[2]}
		default: // a sub-heading reads as text of the current list
			p.endItem()
			p.endPara()
			p.para = append(p.para, m[2])
			p.endPara()
		}
	case bullet.MatchString(trimmed) && !indented:
		p.endItem()
		p.endPara()
		p.item = &item{text: bullet.FindStringSubmatch(trimmed)[1]}
	default:
		p.endItem()
		p.para = append(p.para, trimmed)
	}
}

func (p *parser) endPara() {
	if len(p.para) > 0 {
		p.notes = append(p.notes, strings.Join(p.para, " "))
		p.para = nil
	}
}

func (p *parser) endItem() {
	if p.item == nil {
		return
	}
	t := p.task(*p.item)
	p.item = nil
	if p.list == nil {
		// An item above the first heading still belongs somewhere, but the
		// paragraphs before it stay the file's note.
		p.file.Note = strings.Join(p.notes, "\n\n")
		p.notes = nil
		p.list = &deck.TaskList{}
	}
	p.list.Tasks = append(p.list.Tasks, t)
}

func (p *parser) endList() {
	p.endItem()
	p.endPara()
	note := strings.Join(p.notes, "\n\n")
	p.notes = nil
	if p.list == nil {
		p.file.Note = note
		return
	}
	p.list.Note = note
	p.file.Lists = append(p.file.Lists, *p.list)
	p.list = nil
}

// task turns a list item into a Task. A checkbox item is in the strict format,
// so its text is the title once the owner and thread suffix are off. A plain
// bullet is prose: the title is the text before its first ": " or ". ", with
// parentheses and trailing IDs dropped, and the rest of the line is a note.
func (p *parser) task(it item) deck.Task {
	all := strings.Join(append([]string{it.text}, it.notes...), "\n")
	t := deck.Task{
		Threads: unique(threadID.FindAllString(all, -1)),
		Links:   Scrape(all, p.opt.LinearWorkspace),
	}

	text := it.text
	notes := it.notes
	strict := false
	if m := checkbox.FindStringSubmatch(text); m != nil {
		strict = true
		t.Done = m[1] != " "
		text = text[len(m[0]):]
	}
	fallback := strings.TrimSpace(text)
	text = suffix.ReplaceAllString(text, "")
	if strict {
		if m := owner.FindStringSubmatchIndex(text); m != nil && !isRef(text[m[2]:m[3]]) {
			t.Owner = text[m[2]:m[3]]
			text = text[:m[0]]
		}
	} else {
		if start, end := firstStop(text); start >= 0 {
			rest := strings.TrimSpace(text[end:])
			if rest != "" {
				notes = append([]string{rest}, notes...)
			}
			text = text[:start]
		}
		text = parens.ReplaceAllString(text, "")
		text = tailRefs.ReplaceAllString(text, "")
	}
	t.Title = strings.TrimRight(strings.TrimSpace(text), ".,;:")
	if t.Title == "" {
		t.Title = fallback
	}
	t.Notes = strings.Join(notes, "\n")

	if p.opt.ThreadText != nil {
		for _, id := range t.Threads {
			for _, l := range Scrape(p.opt.ThreadText(id), p.opt.LinearWorkspace) {
				t.Links = appendLink(t.Links, l, p.opt.LinearWorkspace)
			}
		}
	}
	return t
}

// abbreviations end in a dot without ending a sentence.
var abbreviations = map[string]bool{"e.g": true, "i.e": true, "etc": true, "vs": true, "cf": true, "approx": true}

// firstStop finds where a plain bullet's title ends: the first ": " or ". "
// outside parentheses that does not end an abbreviation. It returns the
// separator's start and end, or -1, -1.
func firstStop(text string) (int, int) {
	depth := 0
	for i := 0; i+1 < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ':', '.':
			if depth > 0 || (text[i+1] != ' ' && text[i+1] != '\t') {
				continue
			}
			if text[i] == '.' {
				word := text[strings.LastIndexAny(text[:i], " \t(")+1 : i]
				if abbreviations[strings.ToLower(word)] {
					continue
				}
			}
			return i, i + 2
		}
	}
	return -1, -1
}

// isRef reports whether s is a Linear ID or thread id rather than an owner.
func isRef(s string) bool {
	return threadID.MatchString(s) || linearID.MatchString(s)
}

func unique(ids []string) []string {
	var out []string
	for _, id := range ids {
		dup := false
		for _, have := range out {
			if have == id {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, id)
		}
	}
	return out
}

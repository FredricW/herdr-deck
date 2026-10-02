package ui

import (
	"sort"
	"strings"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// rowKind says what a list row shows.
type rowKind int

const (
	rowHeading rowKind = iota // a list heading, or the pinned Needs you group
	rowWork                   // a task with its threads, or a thread no task names
	rowInbox                  // an unhandled inbox item
	rowGap                    // the blank line between two groups
)

// The lists the deck makes itself, next to TASKS.md's own.
const (
	listNeedsYou = "Needs you"
	listThreads  = "Other threads"
	listResolved = "Resolved"
)

// row is one line of the list. Gaps are rows too, so a row index is always a
// line of the list: scrolling and clicks need no other mapping.
type row struct {
	kind rowKind
	key  string // identifies the row across refreshes
	list string // the list the row is in, or the heading's own name

	// Headings.
	count    int                 // rows in the list
	folded   bool                // the list's rows are hidden
	foldable bool                // Needs you never folds
	glyphs   []deck.ThreadStatus // a folded list's open threads

	// Work rows. task is nil for a thread no task names.
	task    *deck.Task
	threads []deck.Thread

	inbox *deck.InboxItem
}

// title is the row's text in the WORK column and the drawer's title.
func (r row) title() string {
	switch {
	case r.kind == rowHeading:
		return r.list
	case r.kind == rowGap:
		return ""
	case r.inbox != nil:
		return "inbox: " + inboxText(*r.inbox)
	case r.task != nil:
		return r.task.Title
	case len(r.threads) > 0:
		t := r.threads[0]
		if t.Title != "" {
			return t.Title
		}
		return t.ID
	}
	return ""
}

// thread is the row's most pressing thread: the one whose state the row
// shows.
func (r row) thread() (deck.Thread, bool) {
	if len(r.threads) == 0 {
		return deck.Thread{}, false
	}
	best := r.threads[0]
	for _, t := range r.threads[1:] {
		if urgency(t.Status) < urgency(best.Status) {
			best = t
		}
	}
	return best, true
}

func (r row) needsYou() bool {
	if r.kind == rowInbox {
		return true
	}
	for _, t := range r.threads {
		if t.Status == deck.StatusNeedsYou {
			return true
		}
	}
	return false
}

// urgency orders statuses for picking a row's glyph: lower comes first.
func urgency(s deck.ThreadStatus) int {
	switch s {
	case deck.StatusNeedsYou:
		return 0
	case deck.StatusWorking:
		return 1
	case deck.StatusReview:
		return 2
	case deck.StatusUnknown:
		return 3
	}
	return 4
}

// foldedByDefault names the lists that start folded: long lists the user
// rarely needs at a glance.
func foldedByDefault(list string) bool {
	l := strings.ToLower(list)
	return strings.Contains(l, "backlog") || l == strings.ToLower(listResolved)
}

// inboxText is an inbox item's one-line description.
func inboxText(it deck.InboxItem) string {
	text := it.Summary
	if text == "" {
		text = it.Subject
	}
	if text == "" {
		text = it.ID
	}
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	return text
}

// buildRows lays out the list: the Needs you group (threads waiting on the
// user and unhandled inbox items) pinned on top, then TASKS.md's lists in
// order, then the threads no task names, with a gap between two groups.
// folds holds the user's fold toggles by list name; lists without one use
// foldedByDefault.
func buildRows(snap deck.Snapshot, folds map[string]bool) []row {
	byID := make(map[string]deck.Thread, len(snap.Threads))
	for _, t := range snap.Threads {
		byID[t.ID] = t
	}

	type list struct {
		name string
		rows []row
	}
	var lists []list
	named := map[string]bool{}
	for li := range snap.TaskLists {
		tl := &snap.TaskLists[li]
		name := tl.Name
		if name == "" {
			name = "Tasks"
		}
		l := list{name: name}
		for ti := range tl.Tasks {
			task := &tl.Tasks[ti]
			r := row{kind: rowWork, list: name, task: task, key: "task:" + name + "/" + task.Title}
			for _, id := range task.Threads {
				named[id] = true
				if t, ok := byID[id]; ok {
					r.threads = append(r.threads, t)
				}
			}
			l.rows = append(l.rows, r)
		}
		lists = append(lists, l)
	}

	open, resolved := list{name: listThreads}, list{name: listResolved}
	if len(lists) == 0 {
		open.name = "Threads"
	}
	for _, t := range snap.Threads {
		if named[t.ID] {
			continue
		}
		r := row{kind: rowWork, threads: []deck.Thread{t}, key: "thread:" + t.ID}
		if t.Status == deck.StatusDone {
			r.list = resolved.name
			resolved.rows = append(resolved.rows, r)
		} else {
			r.list = open.name
			open.rows = append(open.rows, r)
		}
	}
	for _, l := range []list{open, resolved} {
		if len(l.rows) > 0 {
			lists = append(lists, l)
		}
	}

	// Pin what needs the user, wherever it is listed.
	var pinned []row
	for li := range lists {
		kept := lists[li].rows[:0:0]
		for _, r := range lists[li].rows {
			if r.needsYou() {
				pinned = append(pinned, r)
			} else {
				kept = append(kept, r)
			}
		}
		lists[li].rows = kept
	}
	for i := range snap.Inbox {
		it := &snap.Inbox[i]
		pinned = append(pinned, row{kind: rowInbox, list: listNeedsYou, inbox: it, key: "inbox:" + it.ID})
	}

	var rows []row
	if len(pinned) > 0 {
		rows = append(rows, row{kind: rowHeading, list: listNeedsYou, key: "head:" + listNeedsYou, count: len(pinned)})
		rows = append(rows, pinned...)
	}
	for _, l := range lists {
		folded, ok := folds[l.name]
		if !ok {
			folded = foldedByDefault(l.name)
		}
		if len(rows) > 0 {
			rows = append(rows, row{kind: rowGap, key: "gap:" + l.name})
		}
		h := row{kind: rowHeading, list: l.name, key: "head:" + l.name, count: len(l.rows), folded: folded, foldable: true}
		if folded {
			for _, r := range l.rows {
				for _, t := range r.threads {
					if t.Status != deck.StatusDone {
						h.glyphs = append(h.glyphs, t.Status)
					}
				}
			}
		}
		rows = append(rows, h)
		if !folded {
			rows = append(rows, l.rows...)
		}
	}
	return rows
}

// rowLinks are the links of a row in the drawer's order: Linear, Figma,
// Notion, GitHub, localhost, each kind in the order found. A task offers its
// own links and its threads'; an inbox item its subject thread's.
func rowLinks(r row, snap deck.Snapshot) []deck.Link {
	var links []deck.Link
	add := func(ls []deck.Link) {
		for _, l := range ls {
			links = deck.AppendLink(links, l)
		}
	}
	switch r.kind {
	case rowWork:
		if r.task != nil {
			add(r.task.Links)
		}
		for _, t := range r.threads {
			add(t.Links)
		}
	case rowInbox:
		if t, ok := findThread(snap, r.inbox.Thread); ok {
			add(t.Links)
		}
	}
	sort.SliceStable(links, func(i, j int) bool { return links[i].Kind < links[j].Kind })
	return links
}

func findThread(snap deck.Snapshot, id string) (deck.Thread, bool) {
	if id == "" {
		return deck.Thread{}, false
	}
	for _, t := range snap.Threads {
		if t.ID == id {
			return t, true
		}
	}
	return deck.Thread{}, false
}

package projects

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

var threadIDRe = regexp.MustCompile(`\bt-\d{4}\b`)

type inboxFile struct {
	ID      string `toml:"id"`
	Kind    string `toml:"kind"`
	Subject string `toml:"subject"`
	Created string `toml:"created"`
	Summary string `toml:"summary"`
	Event   string `toml:"event"`
}

// readInbox lists the unhandled items: inbox/*.md, not inbox/done/. A missing
// inbox folder is an empty inbox. Items are newest first.
func (r Reader) readInbox(note func(string, ...any)) []deck.InboxItem {
	return inboxItems(os.DirFS(r.Dir()), note)
}

// inboxItems reads inbox/ in a project folder, as readInbox does.
func inboxItems(project fs.FS, note func(string, ...any)) []deck.InboxItem {
	entries, err := fs.ReadDir(project, "inbox")
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		note("inbox: %v", short(err))
		return nil
	}
	var items []deck.InboxItem
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		item := deck.InboxItem{ID: name}
		data, err := fs.ReadFile(project, path.Join("inbox", e.Name()))
		if err != nil {
			note("inbox/%s: %v", e.Name(), short(err))
			items = append(items, item)
			continue
		}
		var f inboxFile
		if _, err := frontMatter(string(data), &f); err != nil {
			note("inbox/%s: %v", e.Name(), err)
		}
		if f.ID != "" {
			item.ID = f.ID
		}
		item.Kind, item.Subject, item.Summary, item.Event = f.Kind, f.Subject, f.Summary, f.Event
		if t, err := time.Parse(time.RFC3339, f.Created); err == nil {
			item.Created = t
		}
		item.Thread = threadIDRe.FindString(f.Subject)
		if item.Thread == "" {
			item.Thread = threadIDRe.FindString(item.ID)
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].Created.Equal(items[j].Created) {
			return items[i].Created.After(items[j].Created)
		}
		return items[i].ID > items[j].ID
	})
	return items
}

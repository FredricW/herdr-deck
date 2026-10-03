package projects

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// History caches parsed inbox items by their path under the project
// folder. An item's file never changes once written; handling it moves it
// to inbox/done/, which is a new path. Safe for concurrent use.
type History struct {
	mu    sync.Mutex
	items map[string]inboxFile
}

func (h *History) get(path string) (inboxFile, bool) {
	if h == nil {
		return inboxFile{}, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	f, ok := h.items[path]
	return f, ok
}

func (h *History) put(path string, f inboxFile) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.items == nil {
		h.items = map[string]inboxFile{}
	}
	h.items[path] = f
}

// historyItem is an inbox item read for a thread's log.
type historyItem struct {
	inboxFile
	rel       string // path under the project folder
	unhandled bool   // still in inbox/, not in inbox/done/
}

// addLogs gives every thread its timeline: the thread file's timestamps and
// the inbox items about it from inbox/ and inbox/done/. Only files whose
// name holds one of the threads' ids are read, plus routine items, whose
// summary names the thread they prompted. Read-only, like everything here;
// an unreadable item is left out of the log rather than reported, since
// the inbox itself is checked by readInbox.
func (r Reader) addLogs(threads []deck.Thread) {
	if len(threads) == 0 {
		return
	}
	ids := make(map[string]int, len(threads))
	for i, t := range threads {
		ids[t.ID] = i
	}
	byThread := map[string][]historyItem{}
	for _, sub := range []string{"inbox", filepath.Join("inbox", "done")} {
		entries, err := os.ReadDir(filepath.Join(r.Dir(), sub))
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".md") {
				continue
			}
			id := threadIDRe.FindString(name)
			_, named := ids[id]
			routine := strings.Contains(name, "-routine-")
			if !named && !routine {
				continue
			}
			rel := filepath.Join(sub, name)
			f, ok := r.History.get(rel)
			if !ok {
				data, err := os.ReadFile(filepath.Join(r.Dir(), rel))
				if err != nil {
					continue
				}
				if _, err := frontMatter(string(data), &f); err != nil {
					continue
				}
				r.History.put(rel, f)
			}
			owner := threadIDRe.FindString(f.Subject)
			if f.Kind == "routine" {
				owner = threadIDRe.FindString(f.Summary)
			} else if owner == "" {
				owner = id
			}
			if _, ok := ids[owner]; !ok {
				continue
			}
			byThread[owner] = append(byThread[owner], historyItem{inboxFile: f, rel: filepath.ToSlash(rel), unhandled: sub == "inbox"})
		}
	}
	for id, i := range ids {
		threads[i].Log = threadLog(threads[i], byThread[id])
	}
}

// threadLog builds a thread's timeline, newest first.
func threadLog(t deck.Thread, items []historyItem) []deck.LogEvent {
	file := "threads/" + t.ID + ".toml"
	var log []deck.LogEvent
	if !t.Created.IsZero() {
		e := deck.LogEvent{At: t.Created, Kind: deck.EventCreated, Text: "created", Source: file}
		if t.Base != "" {
			e.Text += " from " + t.Base
		}
		log = append(log, e)
	}
	if !t.LaunchedAt.IsZero() {
		e := deck.LogEvent{At: t.LaunchedAt, Kind: deck.EventLaunched, Text: "launched", Source: file}
		if t.PaneID != "" {
			e.Text += " in pane " + t.PaneID
		}
		log = append(log, e)
	}
	for _, it := range items {
		at, err := time.Parse(time.RFC3339, it.Created)
		if err != nil {
			continue
		}
		e := itemEvent(it.inboxFile)
		e.At, e.Unhandled, e.Source = at, it.unhandled, it.rel
		log = append(log, e)
	}
	// The latest report shows even when its inbox item is gone.
	if rc := t.LastReportChange; !rc.IsZero() {
		seen := false
		for _, e := range log {
			if e.Kind == deck.EventReport && absDur(e.At.Sub(rc)) <= 2*time.Minute {
				seen = true
				break
			}
		}
		if !seen {
			log = append(log, deck.LogEvent{At: rc, Kind: deck.EventReport, Text: "new report", Source: file})
		}
	}
	sort.SliceStable(log, func(i, j int) bool { return log[i].At.After(log[j].At) })
	return log
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

var (
	// summaryHead is the `t-0011 "Title"` an item's summary starts with.
	summaryHead = regexp.MustCompile(`^t-\d{4}(?: "[^"]*")?:?\s*`)
	paneRe      = regexp.MustCompile(`\bpane (\S+?)[,;:.]?(?:\s|$)`)
	prStateRe   = regexp.MustCompile(`pull request state (\w+)`)
	failingRe   = regexp.MustCompile(`failing checks: ([^;]+)`)
	commentsRe  = regexp.MustCompile(`(\d+) comment\(s\)`)
	reasonRe    = regexp.MustCompile(`^resolved \(([^)]+)\)`)
	aboutRe     = regexp.MustCompile(`about: (.+)$`)
)

// itemEvent reads what happened from an inbox item's event and summary
// (herdr-projects writes both).
func itemEvent(f inboxFile) deck.LogEvent {
	rest := summaryHead.ReplaceAllString(f.Summary, "")
	pane := ""
	if m := paneRe.FindStringSubmatch(f.Summary); m != nil {
		pane = "pane " + m[1]
	}
	switch f.Event {
	case "new report":
		return deck.LogEvent{Kind: deck.EventReport, Text: "new report"}
	case "waiting on you":
		return deck.LogEvent{Kind: deck.EventWaiting, Text: "waiting on you", Detail: pane}
	case "blocked on a prompt":
		return deck.LogEvent{Kind: deck.EventBlocked, Text: "blocked on a prompt", Detail: pane}
	case "PR opened", "PR updated":
		e := deck.LogEvent{Kind: deck.EventPROpened, Text: "opened"}
		if f.Event == "PR updated" {
			e.Kind, e.Text = deck.EventPRUpdated, "updated"
		}
		var parts []string
		if m := prStateRe.FindStringSubmatch(f.Summary); m != nil && (f.Event == "PR updated" || m[1] != "OPEN") {
			parts = append(parts, strings.ToLower(m[1]))
		}
		if m := commentsRe.FindStringSubmatch(f.Summary); m != nil && m[1] != "0" {
			noun := " comments"
			if m[1] == "1" {
				noun = " comment"
			}
			parts = append(parts, m[1]+noun)
		}
		e.Detail = strings.Join(parts, " · ")
		return e
	case "PR checks failing":
		e := deck.LogEvent{Kind: deck.EventChecksFailing, Text: "checks failing"}
		if m := failingRe.FindStringSubmatch(f.Summary); m != nil {
			e.Text += ": " + strings.TrimSpace(m[1])
		}
		return e
	case "PR merged":
		return deck.LogEvent{Kind: deck.EventMerged, Text: "merged"}
	case "resolved":
		e := deck.LogEvent{Kind: deck.EventResolved, Text: "resolved"}
		if m := reasonRe.FindStringSubmatch(rest); m != nil {
			e.Detail = m[1]
		}
		return e
	case "prompted its thread":
		e := deck.LogEvent{Kind: deck.EventRoutine, Text: "prompted by routine " + f.Subject}
		if m := aboutRe.FindStringSubmatch(f.Summary); m != nil {
			e.Detail = m[1]
		}
		return e
	}
	text := f.Event
	if text == "" {
		text = f.Kind
	}
	return deck.LogEvent{Kind: deck.EventOther, Text: text, Detail: firstLine(rest)}
}

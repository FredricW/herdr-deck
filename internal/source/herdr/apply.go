package herdr

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// BlockedAfter is how long an agent must stay blocked before its thread
// counts as needing the user. Permission prompts that are answered at once
// should not pull the deck's cursor around. herdr-projects itself waits 30 s
// (src/thread.rs, BLOCKED_DEBOUNCE_SECS).
const BlockedAfter = 5 * time.Second

// Reader reads herdr's live state for the deck and remembers since when
// each pane's agent has had its status.
type Reader struct {
	Client Client

	mu    sync.Mutex
	since map[string]seen // by pane id
}

type seen struct {
	status string
	at     time.Time
}

// NewReader returns a Reader on the socket path.
func NewReader(socket string) *Reader {
	return &Reader{Client: Client{Socket: socket}}
}

// Apply reads herdr's live state and lays it over snap: the coordinator's
// pane, each thread's pane and agent status, and the status the live agent
// implies. When herdr cannot be read, snap is left as it is and a line is
// added to snap.Missing.
func (r *Reader) Apply(ctx context.Context, snap *deck.Snapshot, now time.Time) {
	st, err := r.Client.Snapshot(ctx)
	if err != nil {
		snap.Missing = append(snap.Missing, missingLine(err))
		return
	}
	r.apply(snap, st, now)
}

func (r *Reader) apply(snap *deck.Snapshot, st State, now time.Time) {
	r.track(st, now)
	snap.Herdr = true
	slug := snap.Project.Slug
	if p, ok := coordinatorPane(st, slug, snap.Project.Dir); ok {
		snap.Project.PaneID = p.ID
	}
	for i := range snap.Threads {
		t := &snap.Threads[i]
		t.PortToken = portToken(st, t.Worktree)
		p, ok := threadPane(st, slug, *t)
		if !ok {
			continue
		}
		t.Pane = &deck.Pane{ID: p.ID, Agent: p.Agent, AgentStatus: p.AgentStatus, Since: r.sinceOf(p)}
		if s := liveStatus(*t, now); s != t.Status {
			// herdr-projects' state line describes the status it gave.
			t.Status, t.StateLine = s, liveStateLine(s)
		}
	}
}

func missingLine(err error) string {
	if errors.Is(err, ErrNoSocket) {
		return "herdr socket: not found: ↵ cannot focus panes, status from files only"
	}
	return "herdr socket: " + err.Error() + ": status from files only"
}

// track notes each pane's status and since when it has had it.
func (r *Reader) track(st State, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	next := make(map[string]seen, len(st.Panes))
	for _, p := range st.Panes {
		s, ok := r.since[p.ID]
		if !ok || s.status != p.AgentStatus {
			s = seen{status: p.AgentStatus, at: now}
		}
		next[p.ID] = s
	}
	r.since = next
}

func (r *Reader) sinceOf(p Pane) time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.since[p.ID].at
}

// liveStatus is the thread's status with its live agent taken into account.
// An agent blocked on a question or permission prompt needs the user; an
// agent at work is working, whatever the last group said, unless the thread
// failed. Otherwise herdr-projects' group stands: idle and done agents need
// its reports and pull requests to tell review from idle from waiting.
func liveStatus(t deck.Thread, now time.Time) deck.ThreadStatus {
	if t.Status == deck.StatusDone || t.Pane == nil {
		return t.Status
	}
	switch t.Pane.AgentStatus {
	case "blocked":
		if !t.Pane.Since.IsZero() && now.Sub(t.Pane.Since) >= BlockedAfter {
			return deck.StatusNeedsYou
		}
	case "working":
		if t.Status == deck.StatusUnknown || t.Status == deck.StatusNeedsYou && !strings.HasPrefix(t.StateLine, "failed") {
			return deck.StatusWorking
		}
	}
	return t.Status
}

// liveStateLine replaces herdr-projects' state line when the live agent
// changed the thread's status.
func liveStateLine(s deck.ThreadStatus) string {
	if s == deck.StatusNeedsYou {
		return "needs you · blocked on a prompt"
	}
	return "working"
}

// threadPane finds the thread's pane. herdr-projects marks its panes with
// tokens: hp_project is the slug and hp_group ends in the thread id
// ("<slug>!1!<rank>!t-0006"). Without tokens (an older herdr-projects, or
// before the ticker ran), the pane with the thread's recorded id counts when
// its working directory is the thread's worktree, as herdr-projects matches
// it (src/thread.rs, pane_matches).
func threadPane(st State, slug string, t deck.Thread) (Pane, bool) {
	for _, p := range st.Panes {
		if p.Tokens["hp_project"] == slug && lastField(p.Tokens["hp_group"]) == t.ID {
			return p, true
		}
	}
	if t.PaneID == "" {
		return Pane{}, false
	}
	for _, p := range st.Panes {
		if p.ID == t.PaneID && (t.Worktree == "" || samePath(p.Cwd, t.Worktree)) {
			return p, true
		}
	}
	return Pane{}, false
}

// coordinatorPane finds the project's coordinator: hp_group is
// "<slug>!0!<pane id>", else an agent pane whose working directory is the
// project folder.
func coordinatorPane(st State, slug, dir string) (Pane, bool) {
	for _, p := range st.Panes {
		if p.Tokens["hp_project"] == slug && strings.HasPrefix(p.Tokens["hp_group"], slug+"!0!") {
			return p, true
		}
	}
	for _, p := range st.Panes {
		if p.Agent != "" && dir != "" && samePath(p.Cwd, dir) {
			return p, true
		}
	}
	return Pane{}, false
}

// portToken is the `port` token of the workspace opened on the worktree, or
// 0. A worktree plugin sets it (e.g. from a hash of the branch) as the port
// its dev server should use; the deck falls back to it when the worktree's
// manifest gives no port.
func portToken(st State, worktree string) int {
	if worktree == "" {
		return 0
	}
	for _, w := range st.Workspaces {
		if w.Worktree == nil || !samePath(w.Worktree.Path, worktree) {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimSpace(w.Tokens["port"])); err == nil && n > 0 && n < 65536 {
			return n
		}
	}
	return 0
}

func lastField(group string) string {
	if i := strings.LastIndexByte(group, '!'); i >= 0 {
		return group[i+1:]
	}
	return ""
}

func samePath(a, b string) bool {
	return strings.TrimSuffix(a, "/") == strings.TrimSuffix(b, "/")
}

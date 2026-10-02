// Package deck holds the plain data the deck shows. Sources under
// internal/source produce a Snapshot; internal/ui only renders it.
package deck

import "time"

// Snapshot is everything the deck knows about one project at one moment.
type Snapshot struct {
	Project   Project
	TaskLists []TaskList
	Threads   []Thread
	Inbox     []InboxItem
	// Missing lists, one line each, the sources that could not be read in
	// full, so the UI can mark the data as partial instead of failing.
	Missing []string
	// ReadAt is when the snapshot was read.
	ReadAt time.Time
	// ThreadsAsOf is set when the live thread list could not be read and the
	// threads are as herdr-projects last recorded them, at this time.
	ThreadsAsOf time.Time
	// Herdr is set when herdr's live state was read, so a thread without a
	// Pane has no pane open.
	Herdr bool
	// Notes are things worth knowing about the sources that are not
	// missing data, such as a repo without a dev-server manifest. The
	// Sources view lists them; they do not count as missing.
	Notes []string
}

// Project identifies a herdr-projects project.
type Project struct {
	Slug  string
	Name  string
	Goal  string
	Repos []string // repository paths from PROJECT.md
	Dir   string   // the project folder, <root>/<slug>
	// PaneID is the coordinator's herdr pane, when herdr shows one.
	PaneID string
}

// Title returns the project's display name, falling back to its slug.
func (p Project) Title() string {
	if p.Name != "" {
		return p.Name
	}
	return p.Slug
}

// TaskList is one `## <List>` heading in TASKS.md and the tasks under it.
type TaskList struct {
	Name  string
	Note  string // paragraph text under the heading that is not a task
	Tasks []Task
}

// Task is one list item in TASKS.md.
type Task struct {
	Title   string
	Done    bool
	Owner   string
	Threads []string // thread ids such as "t-0002"
	Links   []Link
	Notes   string // the item's own detail and indented notes, one line each
}

// ThreadStatus is the coarse state of a thread, used for its glyph and colour.
type ThreadStatus int

const (
	StatusUnknown ThreadStatus = iota
	StatusNeedsYou
	StatusWorking
	StatusReview
	StatusDone
)

// Thread is one herdr-projects thread.
type Thread struct {
	ID         string
	Title      string
	Status     ThreadStatus
	StateLine  string // e.g. "needs you · ~95%"
	PaneID     string // the pane herdr-projects recorded for the thread
	Pane       *Pane  // the thread's live herdr pane; nil when not known
	Worktree   string
	Repo       string // the repository the worktree belongs to (its main checkout)
	Branch     string
	Activity   string       // e.g. "Writing tests"
	PR         *PullRequest // nil when the thread has no pull request
	Next       []string     // the report's `## Next` lines
	Report     string       // the thread's report (threads/t-NNNN.md), or ""
	Links      []Link
	DevServers []DevServer
	// DevNote says why DevServers is empty, e.g. "no .herdr-deck/dev.json".
	DevNote string
	// PortToken is the herdr workspace token `port` of the thread's
	// worktree, or 0: the dev-server port used when the manifest gives none.
	PortToken int
	// DevUp is the dev manifest's `up` command the deck started for the
	// worktree; nil when it started none.
	DevUp *DevUp
}

// Pane is a thread's herdr pane as herdr shows it now.
type Pane struct {
	ID          string
	Agent       string    // e.g. "claude"
	AgentStatus string    // herdr's agent status: idle, working, blocked, done, unknown
	Since       time.Time // when the deck first saw AgentStatus
}

// PullRequest is a thread's pull request as herdr-projects last saw it.
type PullRequest struct {
	URL           string
	Number        int
	State         string // OPEN, MERGED, CLOSED
	Review        string // APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED, or ""
	FailingChecks []string
	Comments      int
}

// DevServer is one named port of a thread's worktree.
type DevServer struct {
	Name    string
	Port    int
	Running bool // something listens on 127.0.0.1:Port
	// Fallback is set when the port is herdr's workspace token, not the
	// manifest's: a guess at where the worktree's server listens.
	Fallback bool
}

// DevUp is an `up` command the deck started in a thread's worktree.
type DevUp struct {
	Log   string // the file the command writes its output to
	Alive bool   // the command's process still runs
}

// InboxItem is one unhandled item in the project's inbox.
type InboxItem struct {
	ID      string
	Kind    string // e.g. "thread-state"
	Thread  string
	Subject string
	Summary string
	Created time.Time
}

// LinkKind says which service a link points at.
type LinkKind int

const (
	LinkLinear LinkKind = iota
	LinkFigma
	LinkNotion
	LinkGitHub
	LinkLocalhost
)

// Badge is the short inline marker for the kind, e.g. "L" for Linear.
func (k LinkKind) Badge() string {
	switch k {
	case LinkLinear:
		return "L"
	case LinkFigma:
		return "F"
	case LinkNotion:
		return "N"
	case LinkGitHub:
		return "G"
	case LinkLocalhost:
		return "o"
	}
	return "?"
}

func (k LinkKind) String() string {
	switch k {
	case LinkLinear:
		return "Linear"
	case LinkFigma:
		return "Figma"
	case LinkNotion:
		return "Notion"
	case LinkGitHub:
		return "GitHub"
	case LinkLocalhost:
		return "localhost"
	}
	return "unknown"
}

// EnvLinearWorkspace names the environment variable that sets the Linear
// workspace bare issue IDs link into, when --linear-workspace does not.
const EnvLinearWorkspace = "HERDR_DECK_LINEAR_WORKSPACE"

// Link is a URL with the label shown for it (a Linear ID, "PR #2320", …).
// A bare Linear ID read while no Linear workspace is set has no URL.
type Link struct {
	Kind  LinkKind
	Label string
	URL   string
	// Down is set on a localhost link whose dev server does not listen.
	Down bool
	// Issue is a Linear link's issue status, when Linear's API gave one.
	Issue *Issue
}

// Issue is a Linear issue's workflow state as Linear's API last gave it.
type Issue struct {
	State string // the state's name, e.g. "In Progress"
	// StateType is Linear's kind of state: triage, backlog, unstarted,
	// started, completed or canceled.
	StateType string
}

// Same reports whether l and o are one link: Linear links with the same ID
// (with or without a URL), else equal URLs, or without a URL the same kind
// and label.
func (l Link) Same(o Link) bool {
	if l.Kind == LinkLinear && o.Kind == LinkLinear && l.Label == o.Label {
		return true
	}
	if l.URL == "" || o.URL == "" {
		return l.URL == o.URL && l.Kind == o.Kind && l.Label == o.Label
	}
	return l.URL == o.URL
}

// AppendLink adds l to links unless the same link is there; a link with a
// URL replaces the same link without one.
func AppendLink(links []Link, l Link) []Link {
	for i, have := range links {
		if have.Same(l) {
			if have.URL == "" && l.URL != "" {
				links[i] = l
			}
			return links
		}
	}
	return append(links, l)
}

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
	// Projects is every project under the projects root, this one
	// included, for the project picker; nil when they were not read.
	Projects []ProjectInfo
	// LinearKey is set when a Linear API key is configured and Linear
	// gives the links their URLs (or its first answer is pending), so no
	// linear.workspace is needed.
	LinearKey bool
}

// ProjectInfo is one project under the projects root as the project picker
// shows it. Other projects are read cheaply (thread files and inbox, with
// herdr's live state over them), so they can lag their own deck a little.
type ProjectInfo struct {
	// Project's PaneID is the coordinator's live herdr pane, when herdr
	// shows one.
	Project
	// Status is herdr-projects' project status: active, paused or
	// archived.
	Status string
	// RecordedPane is the coordinator pane herdr-projects last recorded
	// (.state/coordinator.json); herdr's live state says whether it runs.
	RecordedPane string
	Threads      []Thread
	Inbox        []InboxItem
	// Problem says what could not be read, or "".
	Problem string
}

// Archived reports whether herdr-projects archived the project.
func (p ProjectInfo) Archived() bool { return p.Status == "archived" }

// Needs is the project's threads waiting on the user. Inbox items are
// progress updates (a report, a PR opened or merged), so they never count;
// an item about a waiting thread counts through the thread.
func (p ProjectInfo) Needs() []Thread {
	var threads []Thread
	for _, t := range p.Threads {
		if t.Status == StatusNeedsYou {
			threads = append(threads, t)
		}
	}
	return threads
}

// NeedsYou reports whether a thread in the project waits on the user.
func (p ProjectInfo) NeedsYou() bool { return len(p.Needs()) > 0 }

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
	ID        string
	Title     string
	Status    ThreadStatus
	StateLine string // e.g. "needs you · ~95%"
	// Group is herdr-projects' group token: waiting-on-you, working,
	// ready-for-review, landing, idle or resolved; "" when unknown.
	Group string
	// Percent is the thread's own estimate of how far along it is; nil
	// when it gave none.
	Percent  *int
	PaneID   string // the pane herdr-projects recorded for the thread
	Pane     *Pane  // the thread's live herdr pane; nil when not known
	Worktree string
	Repo     string // the repository the worktree belongs to (its main checkout)
	Branch   string
	// Base is the branch the thread's worktree started from, e.g.
	// "origin/main"; the diff section compares the worktree with it.
	Base     string
	Activity string // e.g. "Writing tests"
	// Changed is when herdr-projects last saw the thread's state change.
	Changed    time.Time
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

	// What herdr-projects recorded of the thread's life; zero when not.
	Created          time.Time
	LaunchedAt       time.Time
	BriefSeenAt      time.Time
	LastReportChange time.Time
	ResolvedReason   string
	// Log is the thread's timeline, newest first: the thread file's
	// timestamps and the inbox items about it, handled or not.
	Log []LogEvent
}

// EventKind says what happened in a LogEvent.
type EventKind int

const (
	EventOther EventKind = iota
	EventCreated
	EventLaunched
	EventReport
	EventWaiting // waiting on you
	EventBlocked // blocked on a prompt
	EventPROpened
	EventPRUpdated
	EventChecksFailing
	EventMerged
	EventResolved
	EventRoutine // a routine prompted the thread
)

// LogEvent is one thing that happened to a thread, as herdr-projects wrote
// it down.
type LogEvent struct {
	At     time.Time
	Kind   EventKind
	Text   string // e.g. "checks failing: lint"
	Detail string // shown dim after the text, e.g. "pane w1Z:p1"
	// Unhandled is set on an inbox item still in inbox/, not yet in done/.
	Unhandled bool
	// Source is the file the event comes from, relative to the project
	// folder, e.g. "inbox/done/….md" or "threads/t-0002.toml".
	Source string
}

// Diff is what a thread's worktree changed against its base: the files of
// `git diff <merge-base>`, uncommitted changes and untracked files included.
type Diff struct {
	// Base is the ref the worktree is compared with, e.g. "origin/main",
	// and MergeBase the commit it and HEAD share: the {base} a diff tool
	// gets, so it shows the same changes as Files.
	Base      string
	MergeBase string
	Files     []DiffFile
	// Note says why there are no files to show, e.g. "no worktree"; a
	// note is never an error, the section just shows it dim.
	Note string
}

// Totals adds up the lines the files added and deleted; binary files
// count no lines.
func (d Diff) Totals() (added, deleted int) {
	for _, f := range d.Files {
		added += f.Added
		deleted += f.Deleted
	}
	return added, deleted
}

// DiffFile is one changed file. Path is relative to the worktree.
type DiffFile struct {
	Path    string
	OldPath string // the path before a rename, or ""
	// Change is how the file changed; an untracked file is ChangeAdded.
	Change  Change
	Added   int
	Deleted int
	// Binary files have no line counts.
	Binary bool
	// Untracked files are new and not yet added to git.
	Untracked bool
}

// Change is how a file changed, as git's diff status letter says.
type Change int

const (
	ChangeModified Change = iota // M, and a type change (T)
	ChangeAdded                  // A, and an untracked file
	ChangeDeleted                // D
	ChangeRenamed                // R: OldPath holds the old name
)

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
	Commenters    []string // the logins that commented
	// CheckedAt is when herdr-projects last asked GitHub about PRs.
	CheckedAt time.Time
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
	Event   string // e.g. "new report", "PR checks failing"
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

// Issue is a Linear issue as Linear's API last gave it.
type Issue struct {
	State string // the state's name, e.g. "In Progress"
	// StateType is Linear's kind of state: triage, backlog, unstarted,
	// started, completed or canceled.
	StateType string
	Title     string
	// URL is the issue's page as Linear gives it; a link without a URL of
	// its own takes it.
	URL  string
	Team string // the team's name, e.g. "Admin"
	// Assignee is the assignee's display name and AssigneeInitials their
	// initials; both are empty for an unassigned issue.
	Assignee         string
	AssigneeInitials string
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

// Patch is one changed file's diff as the preview shows it: the unified
// diff of the file against the merge-base, or an untracked file's lines
// all added.
type Patch struct {
	Lines []PatchLine
	// More is how many lines of the diff were left out past the preview's
	// cap; with Cut set the diff was larger than was read, so More is a
	// lower bound.
	More int
	Cut  bool
	// Binary files have no lines.
	Binary bool
	// Note says why there are no lines, e.g. "renamed, content unchanged"
	// or a git error; it is never shown as a failure.
	Note string
}

// PatchLine is one line of a Patch.
type PatchLine struct {
	Kind LineKind
	// Text is the line without its diff marker; a hunk's line is the whole
	// `@@ -a,b +c,d @@ …` header.
	Text string
}

// LineKind is what a PatchLine is.
type LineKind int

const (
	LineContext LineKind = iota // unchanged, around the changes
	LineAdded
	LineDeleted
	LineHunk // a hunk header
	LineNote // git's `\ No newline at end of file`
	// LineFile starts one file's part of a commit's patch; its Text is
	// the file's path, a rename's as "old → new".
	LineFile
)

// Commits is a thread branch's own commits: `git log <merge-base>..HEAD`
// in its worktree, newest first.
type Commits struct {
	// Base is the ref the branch is compared with, MergeBase the commit
	// they share and Head the branch's tip.
	Base      string
	MergeBase string
	Head      string
	List      []Commit
	// More is how many older commits were left out past the cap.
	More int
	// Uncommitted is how many files have changes not committed yet,
	// untracked ones included.
	Uncommitted int
	// Upstream is set when the branch has an upstream, so each commit's
	// Pushed says whether the remote has it.
	Upstream bool
	// Note says why there are no commits to show, e.g. "no worktree"; it
	// is never an error.
	Note string
}

// Commit is one commit of a thread's branch.
type Commit struct {
	SHA     string
	Short   string
	Author  string
	Time    time.Time
	Subject string
	// Merge commits have no line counts.
	Merge          bool
	Added, Deleted int
	// Pushed is set when the branch's upstream has the commit.
	Pushed bool
}

// CommitPatch is one commit as the preview shows it: its message's body
// and its diff against its first parent, each file's part starting with a
// LineFile line.
type CommitPatch struct {
	Body  string
	Patch Patch
}

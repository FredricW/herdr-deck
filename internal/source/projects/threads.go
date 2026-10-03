package projects

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// threadRecord holds the fields the deck uses from a thread, both as
// herdr-projects stores it (threads/t-NNNN.toml) and as `thread list --json`
// prints it (the same fields plus group, group_token, next, report).
type threadRecord struct {
	ID             string `json:"id" toml:"id"`
	Title          string `json:"title" toml:"title"`
	Status         string `json:"status" toml:"status"` // starting, open, failed, resolved
	Error          string `json:"error" toml:"error"`
	ResolvedReason string `json:"resolved_reason" toml:"resolved_reason"`
	Branch         string `json:"branch" toml:"branch"`
	Base           string `json:"base" toml:"base"`
	WorktreePath   string `json:"worktree_path" toml:"worktree_path"`
	Repo           string `json:"repo" toml:"repo"`
	Cwd            string `json:"cwd" toml:"cwd"`
	PaneID         string `json:"pane_id" toml:"pane_id"`
	LastGroup      string `json:"last_group" toml:"last_group"`
	LastChange     string `json:"last_state_change" toml:"last_state_change"`
	StateLine      string `json:"state_line" toml:"state_line"`
	Activity       string `json:"activity" toml:"activity"`
	PR             string `json:"pr" toml:"pr"`
	PRState        string `json:"pr_state" toml:"pr_state"`
	PRReview       string `json:"pr_review" toml:"pr_review"`
	Percent        *int   `json:"percent" toml:"percent"`

	// Timestamps (RFC 3339), "" when not recorded.
	Created          string `json:"created" toml:"created"`
	LaunchedAt       string `json:"launched_at" toml:"launched_at"`
	BriefSeenAt      string `json:"brief_seen_at" toml:"brief_seen_at"`
	LastReportChange string `json:"last_report_change" toml:"last_report_change"`

	// Only in `thread list --json`.
	GroupToken string   `json:"group_token" toml:"-"`
	Next       []string `json:"next" toml:"-"`
}

// readThreads asks herdr-projects for the thread list, which includes the
// live group; when that fails it reads the thread files the ticker last wrote
// and returns, as asOf, when the newest of them was written. Each thread's
// report is read from threads/t-NNNN.md.
func (r Reader) readThreads(ctx context.Context, tk ticker, note func(string, ...any)) (threads []deck.Thread, asOf time.Time) {
	recs, err := r.listThreads(ctx)
	if err != nil {
		note("thread list: %v; showing threads/*.toml as last recorded", err)
		recs, asOf = r.threadFiles(note)
	}
	threads = make([]deck.Thread, 0, len(recs))
	for _, rec := range recs {
		t := toThread(rec, tk)
		if report, err := os.ReadFile(filepath.Join(r.Dir(), "threads", rec.ID+".md")); err == nil {
			t.Report = string(report)
		}
		threads = append(threads, t)
	}
	return threads, asOf
}

func (r Reader) listThreads(ctx context.Context) ([]threadRecord, error) {
	bin := r.Bin
	if bin == "" {
		bin = DefaultBin
	}
	run := r.Run
	if run == nil {
		run = execRun
	}
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	// Read-only: `thread list` only reads records. Never `context` here.
	out, err := run(ctx, bin, "--root", r.Root, "thread", "list", r.Slug, "--json")
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return nil, errors.New(firstLine(string(ee.Stderr)))
		}
		return nil, err
	}
	var recs []threadRecord
	if err := json.Unmarshal(out, &recs); err != nil {
		return nil, err
	}
	return recs, nil
}

// threadFiles reads threads/t-*.toml in id order, taking Next from each
// thread's home report threads/t-NNNN.md.
func (r Reader) threadFiles(note func(string, ...any)) ([]threadRecord, time.Time) {
	return threadRecords(os.DirFS(r.Dir()), true, note)
}

// threadRecords reads threads/t-*.toml in a project folder in id order and
// returns when the newest was written. With next it takes Next from each
// thread's home report threads/t-NNNN.md.
func threadRecords(project fs.FS, next bool, note func(string, ...any)) ([]threadRecord, time.Time) {
	paths, _ := fs.Glob(project, "threads/t-*.toml")
	sort.Strings(paths)
	recs := make([]threadRecord, 0, len(paths))
	var newest time.Time
	for _, p := range paths {
		if fi, err := fs.Stat(project, p); err == nil && fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
		data, err := fs.ReadFile(project, p)
		if err != nil {
			note("threads/%s: %v", path.Base(p), short(err))
			continue
		}
		var rec threadRecord
		if _, err := toml.Decode(string(data), &rec); err != nil {
			note("threads/%s: %v", path.Base(p), err)
			continue
		}
		if rec.ID == "" {
			rec.ID = strings.TrimSuffix(path.Base(p), ".toml")
		}
		if next {
			if report, err := fs.ReadFile(project, path.Join("threads", rec.ID+".md")); err == nil {
				rec.Next = nextLines(string(report))
			}
		}
		recs = append(recs, rec)
	}
	return recs, newest
}

func toThread(rec threadRecord, tk ticker) deck.Thread {
	group := rec.GroupToken
	if group == "" {
		group = rec.LastGroup
	}
	t := deck.Thread{
		ID:        rec.ID,
		Title:     rec.Title,
		Status:    status(rec.Status, group),
		StateLine: rec.StateLine,
		PaneID:    rec.PaneID,
		Worktree:  rec.WorktreePath,
		Repo:      rec.Repo,
		Branch:    rec.Branch,
		Base:      rec.Base,
		Activity:  rec.Activity,
		Next:      rec.Next,
		Group:     group,
		Percent:   rec.Percent,

		Created:          stamp(rec.Created),
		LaunchedAt:       stamp(rec.LaunchedAt),
		BriefSeenAt:      stamp(rec.BriefSeenAt),
		LastReportChange: stamp(rec.LastReportChange),
		ResolvedReason:   rec.ResolvedReason,
	}
	if t.Worktree == "" {
		t.Worktree = rec.Cwd
	}
	if at, err := time.Parse(time.RFC3339, rec.LastChange); err == nil {
		t.Changed = at
	}
	if t.StateLine == "" {
		switch {
		case rec.Status == "failed" && rec.Error != "":
			t.StateLine = "failed: " + rec.Error
		case rec.Status == "resolved" && rec.ResolvedReason != "":
			t.StateLine = "resolved · " + rec.ResolvedReason
		default:
			t.StateLine = rec.Activity
		}
	}
	if rec.PR != "" {
		pr := &deck.PullRequest{URL: rec.PR, Number: prNumber(rec.PR), State: rec.PRState, Review: rec.PRReview, CheckedAt: tk.checked}
		if s, ok := tk.prs[rec.ID]; ok {
			if s.State != "" {
				pr.State = s.State
			}
			if s.ReviewDecision != "" {
				pr.Review = s.ReviewDecision
			}
			pr.FailingChecks = s.FailingChecks
			pr.Comments = s.CommentCount
			pr.Commenters = s.Commenters
		}
		t.PR = pr
		label := "PR"
		if pr.Number > 0 {
			label = "PR #" + strconv.Itoa(pr.Number)
		}
		t.Links = append(t.Links, deck.Link{Kind: deck.LinkGitHub, Label: label, URL: pr.URL})
	}
	return t
}

// status maps a thread's record status and herdr-projects group token
// (src/thread.rs, Group::token) onto the deck's coarse status.
func status(recStatus, group string) deck.ThreadStatus {
	if recStatus == "resolved" {
		return deck.StatusDone
	}
	switch group {
	case "waiting-on-you":
		return deck.StatusNeedsYou
	case "working":
		return deck.StatusWorking
	case "ready-for-review", "landing":
		return deck.StatusReview
	case "resolved":
		return deck.StatusDone
	}
	return deck.StatusUnknown
}

// stamp parses an RFC 3339 time, zero when s is empty or not one.
func stamp(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

var prNumberRe = regexp.MustCompile(`/pull/(\d+)`)

func prNumber(url string) int {
	m := prNumberRe.FindStringSubmatch(url)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// nextLines returns the items of a report's `## Next` section, as
// herdr-projects reads them (src/thread.rs, next_lines).
func nextLines(report string) []string {
	var lines []string
	inside := false
	for _, line := range strings.Split(report, "\n") {
		if strings.HasPrefix(line, "#") {
			inside = strings.HasPrefix(line, "## ") && strings.TrimSpace(line) == "## Next"
			continue
		}
		if !inside {
			continue
		}
		text := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(text, "- "):
			text = text[2:]
		case strings.HasPrefix(text, "* "):
			text = text[2:]
		default:
			if n, rest, ok := strings.Cut(text, ". "); ok && n != "" && strings.Trim(n, "0123456789") == "" {
				text = rest
			}
		}
		if text = strings.TrimSpace(text); text != "" {
			lines = append(lines, text)
		}
	}
	return lines
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

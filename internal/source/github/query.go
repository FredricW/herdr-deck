package github

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// prRef is a pull request named by its URL.
type prRef struct {
	URL    string
	Host   string // e.g. "github.com"
	Owner  string
	Repo   string
	Number int
	// Detail asks for the description and conversation too.
	Detail bool
}

// parsePR reads https://<host>/<owner>/<repo>/pull/<number>.
func parsePR(raw string) (prRef, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" && u.Scheme != "http" || u.Host == "" {
		return prRef{}, false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 || parts[2] != "pull" || parts[0] == "" || parts[1] == "" {
		return prRef{}, false
	}
	n, err := strconv.Atoi(parts[3])
	if err != nil || n <= 0 {
		return prRef{}, false
	}
	return prRef{URL: raw, Host: u.Host, Owner: parts[0], Repo: parts[1], Number: n}, true
}

// jobURL is an Actions job's page: …/actions/runs/<run>/job/<job>.
var jobURL = regexp.MustCompile(`/actions/runs/\d+/job/(\d+)`)

// JobID is the Actions job a check's URL names, or 0.
func JobID(detailsURL string) int64 {
	m := jobURL.FindStringSubmatch(detailsURL)
	if m == nil {
		return 0
	}
	id, _ := strconv.ParseInt(m[1], 10, 64)
	return id
}

// prFields is what the deck asks about each PR: about 3 points of
// GitHub's GraphQL budget (contexts and threads are the nodes that cost).
const prFields = `state isDraft mergeable mergeStateStatus reviewDecision baseRefName
autoMergeRequest { enabledAt }
reviewRequests(first: 10) { nodes { requestedReviewer { __typename ... on User { login } ... on Team { name } ... on Bot { login } ... on Mannequin { login } } } }
commits(last: 1) { nodes { commit { statusCheckRollup { contexts(first: 60) { nodes { __typename
  ... on CheckRun { name status conclusion detailsUrl startedAt completedAt checkSuite { workflowRun { workflow { name } } } }
  ... on StatusContext { context state targetUrl createdAt } } } } } } }
reviewThreads(first: 50) { nodes { isResolved isOutdated path line originalLine
  comments(first: 1) { totalCount nodes { author { login } body url createdAt } } } }`

// detailFields is what the PR tab adds: the description and the
// conversation, about 3 points more. Review threads come from prFields,
// resolved ones included.
const detailFields = `title body createdAt updatedAt headRefName additions deletions changedFiles
author { __typename login }
labels(first: 20) { nodes { name } }
comments(last: 50) { totalCount nodes { author { __typename login } body url createdAt } }
reviews(last: 50) { totalCount nodes { author { __typename login } state body url submittedAt comments { totalCount } } }`

// query asks for refs, all on one host, in one request: one aliased
// repository { pullRequest } per PR, plus the rate limit left; the refs
// with Detail set get detailFields too. It returns gh's arguments.
func query(refs []prRef) []string {
	var params, fields []string
	args := []string{"api", "graphql", "--hostname", refs[0].Host}
	detail := false
	for i, ref := range refs {
		o, r, n := "o"+strconv.Itoa(i), "r"+strconv.Itoa(i), "n"+strconv.Itoa(i)
		params = append(params, "$"+o+": String!", "$"+r+": String!", "$"+n+": Int!")
		spread := "...deckPR"
		if ref.Detail {
			spread += " ...deckDetail"
			detail = true
		}
		fields = append(fields, fmt.Sprintf("p%d: repository(owner: $%s, name: $%s) { pullRequest(number: $%s) { %s } }", i, o, r, n, spread))
		args = append(args, "-f", o+"="+ref.Owner, "-f", r+"="+ref.Repo, "-F", n+"="+strconv.Itoa(ref.Number))
	}
	q := "query DeckPRs(" + strings.Join(params, ", ") + ") { rateLimit { remaining resetAt } " +
		strings.Join(fields, " ") + " } fragment deckPR on PullRequest { " + strings.Join(strings.Fields(prFields), " ") + " }"
	// GraphQL refuses a fragment the query does not use.
	if detail {
		q += " fragment deckDetail on PullRequest { " + strings.Join(strings.Fields(detailFields), " ") + " }"
	}
	return append(args, "-f", "query="+q)
}

type gqlResponse struct {
	Data *struct {
		RateLimit *struct {
			Remaining int       `json:"remaining"`
			ResetAt   time.Time `json:"resetAt"`
		} `json:"rateLimit"`
	} `json:"data"`
	Errors []gqlError `json:"errors"`
}

type gqlError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Path    []any  `json:"path"`
}

type gqlRepo struct {
	PullRequest *gqlPR `json:"pullRequest"`
}

type gqlPR struct {
	State            string `json:"state"`
	IsDraft          bool   `json:"isDraft"`
	Mergeable        string `json:"mergeable"`
	MergeStateStatus string `json:"mergeStateStatus"`
	ReviewDecision   string `json:"reviewDecision"`
	BaseRefName      string `json:"baseRefName"`
	AutoMergeRequest *struct {
		EnabledAt time.Time `json:"enabledAt"`
	} `json:"autoMergeRequest"`
	ReviewRequests struct {
		Nodes []struct {
			RequestedReviewer *struct {
				Login string `json:"login"`
				Name  string `json:"name"`
			} `json:"requestedReviewer"`
		} `json:"nodes"`
	} `json:"reviewRequests"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					Contexts struct {
						Nodes []gqlContext `json:"nodes"`
					} `json:"contexts"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
	gqlDetail
	ReviewThreads struct {
		Nodes []struct {
			IsResolved   bool   `json:"isResolved"`
			IsOutdated   bool   `json:"isOutdated"`
			Path         string `json:"path"`
			Line         *int   `json:"line"`
			OriginalLine *int   `json:"originalLine"`
			Comments     struct {
				TotalCount int `json:"totalCount"`
				Nodes      []struct {
					Author *struct {
						Login string `json:"login"`
					} `json:"author"`
					Body      string    `json:"body"`
					URL       string    `json:"url"`
					CreatedAt time.Time `json:"createdAt"`
				} `json:"nodes"`
			} `json:"comments"`
		} `json:"nodes"`
	} `json:"reviewThreads"`
}

// gqlDetail is detailFields' answer; Title is empty when they were not
// asked for.
type gqlDetail struct {
	Title        string     `json:"title"`
	Body         string     `json:"body"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	HeadRefName  string     `json:"headRefName"`
	Additions    int        `json:"additions"`
	Deletions    int        `json:"deletions"`
	ChangedFiles int        `json:"changedFiles"`
	Author       *gqlAuthor `json:"author"`
	Labels       *struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	Comments *struct {
		TotalCount int `json:"totalCount"`
		Nodes      []struct {
			Author    *gqlAuthor `json:"author"`
			Body      string     `json:"body"`
			URL       string     `json:"url"`
			CreatedAt time.Time  `json:"createdAt"`
		} `json:"nodes"`
	} `json:"comments"`
	Reviews *struct {
		TotalCount int `json:"totalCount"`
		Nodes      []struct {
			Author      *gqlAuthor `json:"author"`
			State       string     `json:"state"`
			Body        string     `json:"body"`
			URL         string     `json:"url"`
			SubmittedAt time.Time  `json:"submittedAt"`
			Comments    struct {
				TotalCount int `json:"totalCount"`
			} `json:"comments"`
		} `json:"nodes"`
	} `json:"reviews"`
}

// gqlAuthor is a comment's author: a User, a Bot (an app), a Mannequin
// or an Organization; null when the account is gone.
type gqlAuthor struct {
	Typename string `json:"__typename"`
	Login    string `json:"login"`
}

// login is the author's login, "ghost" for a deleted account, as GitHub
// shows it; bot says it is an app.
func (a *gqlAuthor) login() (login string, bot bool) {
	if a == nil || a.Login == "" {
		return "ghost", false
	}
	return a.Login, a.Typename == "Bot" || strings.HasSuffix(a.Login, "[bot]")
}

// gqlContext is a CheckRun or a StatusContext of the rollup.
type gqlContext struct {
	Typename string `json:"__typename"`
	// CheckRun
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	DetailsURL  string    `json:"detailsUrl"`
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt"`
	CheckSuite  *struct {
		WorkflowRun *struct {
			Workflow struct {
				Name string `json:"name"`
			} `json:"workflow"`
		} `json:"workflowRun"`
	} `json:"checkSuite"`
	// StatusContext
	Context   string    `json:"context"`
	State     string    `json:"state"`
	TargetURL string    `json:"targetUrl"`
	CreatedAt time.Time `json:"createdAt"`
}

// parseResponse reads gh's output for a query of n PRs. Each PR's slot is
// nil when GitHub had no such PR (or no access to it).
func parseResponse(raw []byte, n int) (prs []*pullData, resp gqlResponse, err error) {
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, resp, fmt.Errorf("gh's answer is not GraphQL JSON")
	}
	var data map[string]json.RawMessage
	var outer struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &outer) == nil {
		data = outer.Data
	}
	if data == nil {
		return nil, resp, nil
	}
	prs = make([]*pullData, n)
	for i := range n {
		rawRepo, ok := data["p"+strconv.Itoa(i)]
		if !ok || string(rawRepo) == "null" {
			continue
		}
		var repo gqlRepo
		if err := json.Unmarshal(rawRepo, &repo); err != nil {
			return nil, resp, fmt.Errorf("gh's answer for PR %d does not parse: %v", i, err)
		}
		if repo.PullRequest != nil {
			prs[i] = repo.PullRequest.data()
		}
	}
	return prs, resp, nil
}

// pullData is what GitHub said about one PR.
type pullData struct {
	State      string
	Review     string
	Draft      bool
	Base       string
	MergeState string
	Mergeable  string
	AutoMerge  bool
	Requests   []string
	Checks     []deck.Check
	Threads    []deck.ReviewThread
	// Detail is the description and conversation, when asked for.
	Detail *deck.PRDetail
}

// Failing names the failed checks, the way herdr-projects' ticker does.
func (p *pullData) Failing() []string {
	var out []string
	for _, c := range p.Checks {
		if c.State == deck.CheckFailed {
			out = append(out, c.Name)
		}
	}
	return out
}

func (g *gqlPR) data() *pullData {
	p := &pullData{
		State:      g.State,
		Review:     g.ReviewDecision,
		Draft:      g.IsDraft,
		Base:       g.BaseRefName,
		MergeState: g.MergeStateStatus,
		Mergeable:  g.Mergeable,
		AutoMerge:  g.AutoMergeRequest != nil,
	}
	for _, n := range g.ReviewRequests.Nodes {
		if r := n.RequestedReviewer; r != nil {
			name := r.Login
			if name == "" {
				name = r.Name
			}
			if name != "" {
				p.Requests = append(p.Requests, name)
			}
		}
	}
	for _, c := range g.Commits.Nodes {
		if r := c.Commit.StatusCheckRollup; r != nil {
			p.Checks = checks(r.Contexts.Nodes)
		}
	}
	var outdated []deck.ReviewThread
	var resolved []deck.ReviewThread
	for _, t := range g.ReviewThreads.Nodes {
		rt := deck.ReviewThread{Path: t.Path, Outdated: t.IsOutdated, Replies: max(t.Comments.TotalCount-1, 0)}
		switch {
		case t.Line != nil:
			rt.Line = *t.Line
		case t.OriginalLine != nil && t.IsOutdated:
			rt.Line = *t.OriginalLine
		}
		if len(t.Comments.Nodes) > 0 {
			c := t.Comments.Nodes[0]
			if c.Author != nil {
				rt.Author = c.Author.Login
			}
			rt.Body, rt.URL, rt.At = strings.TrimSpace(c.Body), c.URL, c.CreatedAt
		}
		switch {
		case t.IsResolved:
			resolved = append(resolved, rt)
		case rt.Outdated:
			outdated = append(outdated, rt)
		default:
			p.Threads = append(p.Threads, rt)
		}
	}
	p.Threads = append(p.Threads, outdated...)
	if g.Title != "" {
		p.Detail = g.detail(p.Threads, resolved)
	}
	return p
}

// detail is the PR tab's data: detailFields' answer, with the review
// threads (open and resolved) laid into the conversation by time.
func (g *gqlPR) detail(open, resolved []deck.ReviewThread) *deck.PRDetail {
	d := &deck.PRDetail{
		Title: g.Title, Body: strings.TrimSpace(g.Body), Head: g.HeadRefName, Base: g.BaseRefName,
		Created: g.CreatedAt, Updated: g.UpdatedAt,
		Additions: g.Additions, Deletions: g.Deletions, ChangedFiles: g.ChangedFiles,
	}
	if g.Author != nil {
		d.Author, _ = g.Author.login()
	}
	if g.Labels != nil {
		for _, l := range g.Labels.Nodes {
			d.Labels = append(d.Labels, l.Name)
		}
	}
	if c := g.Comments; c != nil {
		d.Earlier += max(c.TotalCount-len(c.Nodes), 0)
		for _, n := range c.Nodes {
			who, bot := n.Author.login()
			d.Comments = append(d.Comments, deck.PRComment{Kind: deck.CommentIssue, Author: who, Bot: bot,
				Body: strings.TrimSpace(n.Body), URL: n.URL, At: n.CreatedAt})
		}
	}
	if r := g.Reviews; r != nil {
		d.Earlier += max(r.TotalCount-len(r.Nodes), 0)
		at := map[string]int{}
		for _, n := range r.Nodes {
			if n.State == "PENDING" { // a draft review, the viewer's own
				continue
			}
			who, bot := n.Author.login()
			d.Comments = append(d.Comments, deck.PRComment{Kind: deck.CommentReview, Author: who, Bot: bot,
				Body: strings.TrimSpace(n.Body), URL: n.URL, At: n.SubmittedAt, State: n.State, Inline: n.Comments.TotalCount})
			// A later comment-only review keeps an approval or a request
			// for changes, as on GitHub.
			i, seen := at[who]
			switch {
			case !seen:
				at[who] = len(d.Reviewers)
				d.Reviewers = append(d.Reviewers, deck.Reviewer{Login: who, State: n.State})
			case n.State != "COMMENTED":
				d.Reviewers[i].State = n.State
			}
		}
	}
	thread := func(t deck.ReviewThread, done bool) {
		d.Comments = append(d.Comments, deck.PRComment{Kind: deck.CommentThread, Author: t.Author,
			Body: t.Body, URL: t.URL, At: t.At, Path: t.Path, Line: t.Line, Outdated: t.Outdated,
			Resolved: done, Replies: t.Replies})
	}
	for _, t := range open {
		thread(t, false)
	}
	for _, t := range resolved {
		thread(t, true)
	}
	slices.SortStableFunc(d.Comments, func(a, b deck.PRComment) int { return a.At.Compare(b.At) })
	return d
}

// checks turns the rollup into one check per workflow and name: a re-run
// leaves the earlier run in the rollup too, and the latest one counts. Two
// workflows' jobs of the same name stay apart.
func checks(nodes []gqlContext) []deck.Check {
	var out []deck.Check
	at := map[string]int{}
	for _, n := range nodes {
		var c deck.Check
		key := n.Typename
		switch n.Typename {
		case "CheckRun":
			c = deck.Check{Name: n.Name, State: runState(n.Status, n.Conclusion), URL: n.DetailsURL,
				JobID: JobID(n.DetailsURL), Started: n.StartedAt, Completed: n.CompletedAt}
			if s := n.CheckSuite; s != nil && s.WorkflowRun != nil {
				key += "\x00" + s.WorkflowRun.Workflow.Name
			}
		case "StatusContext":
			c = deck.Check{Name: n.Context, State: statusState(n.State), URL: n.TargetURL, Started: n.CreatedAt}
			if c.State == deck.CheckPassed || c.State == deck.CheckFailed {
				c.Completed = n.CreatedAt
			}
		default:
			continue
		}
		if c.Name == "" {
			continue
		}
		key += "\x00" + c.Name
		if i, ok := at[key]; ok {
			if c.Started.After(out[i].Started) {
				out[i] = c
			}
			continue
		}
		at[key] = len(out)
		out = append(out, c)
	}
	slices.SortStableFunc(out, func(a, b deck.Check) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// runState reads a CheckRun's status and conclusion.
func runState(status, conclusion string) deck.CheckState {
	switch status {
	case "COMPLETED":
	case "IN_PROGRESS":
		return deck.CheckRunning
	default: // QUEUED, WAITING, PENDING, REQUESTED
		return deck.CheckQueued
	}
	switch conclusion {
	case "SUCCESS", "NEUTRAL":
		return deck.CheckPassed
	case "SKIPPED", "STALE":
		return deck.CheckSkipped
	}
	return deck.CheckFailed // FAILURE, TIMED_OUT, CANCELLED, ACTION_REQUIRED, STARTUP_FAILURE
}

// statusState reads a commit status's state.
func statusState(state string) deck.CheckState {
	switch state {
	case "SUCCESS":
		return deck.CheckPassed
	case "FAILURE", "ERROR":
		return deck.CheckFailed
	case "PENDING":
		return deck.CheckRunning
	}
	return deck.CheckQueued // EXPECTED
}

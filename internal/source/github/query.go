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
  ... on CheckRun { name status conclusion detailsUrl startedAt completedAt }
  ... on StatusContext { context state targetUrl createdAt } } } } } } }
reviewThreads(first: 50) { nodes { isResolved isOutdated path line originalLine
  comments(first: 1) { totalCount nodes { author { login } body url createdAt } } } }`

// query asks for refs, all on one host, in one request: one aliased
// repository { pullRequest } per PR, plus the rate limit left. It returns
// gh's arguments.
func query(refs []prRef) []string {
	var params, fields []string
	args := []string{"api", "graphql", "--hostname", refs[0].Host}
	for i, ref := range refs {
		o, r, n := "o"+strconv.Itoa(i), "r"+strconv.Itoa(i), "n"+strconv.Itoa(i)
		params = append(params, "$"+o+": String!", "$"+r+": String!", "$"+n+": Int!")
		fields = append(fields, fmt.Sprintf("p%d: repository(owner: $%s, name: $%s) { pullRequest(number: $%s) { ...deckPR } }", i, o, r, n))
		args = append(args, "-f", o+"="+ref.Owner, "-f", r+"="+ref.Repo, "-F", n+"="+strconv.Itoa(ref.Number))
	}
	q := "query DeckPRs(" + strings.Join(params, ", ") + ") { rateLimit { remaining resetAt } " +
		strings.Join(fields, " ") + " } fragment deckPR on PullRequest { " + strings.Join(strings.Fields(prFields), " ") + " }"
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
	for _, t := range g.ReviewThreads.Nodes {
		if t.IsResolved {
			continue
		}
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
		if rt.Outdated {
			outdated = append(outdated, rt)
		} else {
			p.Threads = append(p.Threads, rt)
		}
	}
	p.Threads = append(p.Threads, outdated...)
	return p
}

// checks turns the rollup into one check per name: a re-run leaves the
// earlier run in the rollup too, and the latest one counts.
func checks(nodes []gqlContext) []deck.Check {
	var out []deck.Check
	at := map[string]int{}
	for _, n := range nodes {
		var c deck.Check
		switch n.Typename {
		case "CheckRun":
			c = deck.Check{Name: n.Name, State: runState(n.Status, n.Conclusion), URL: n.DetailsURL,
				JobID: JobID(n.DetailsURL), Started: n.StartedAt, Completed: n.CompletedAt}
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
		if i, ok := at[c.Name]; ok {
			if c.Started.After(out[i].Started) {
				out[i] = c
			}
			continue
		}
		at[c.Name] = len(out)
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

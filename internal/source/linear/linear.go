// Package linear reads the workflow state of the Linear issues a snapshot
// links to, through Linear's GraphQL API, and lays it over the snapshot's
// Linear links.
//
// Fetches run in the background, batched and cached, so a reload never
// waits on the network: Apply shows what the cache holds and starts a fetch
// for the IDs it lacks or holds for longer than TTL. Any failure (offline,
// a refused key, rate limits) leaves the IDs without a status and is named
// in Snapshot.Missing.
//
// The API key comes from $LINEAR_API_KEY, else from a command that prints
// it (such as `op read …`). It is kept in memory only and never appears in
// the snapshot, errors or logs.
package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// EnvAPIKey is the environment variable that holds a Linear API key.
const EnvAPIKey = "LINEAR_API_KEY"

// Endpoint is Linear's GraphQL API.
const Endpoint = "https://api.linear.app/graphql"

// Defaults for Reader's zero fields.
const (
	// TTL is how long a fetched status is shown before it is fetched again.
	TTL = 3 * time.Minute
	// RequestTimeout bounds one GraphQL request.
	RequestTimeout = 15 * time.Second
	// CommandTimeout bounds the key command; `op` may wait for an unlock.
	CommandTimeout = 30 * time.Second
	// batchSize is how many issues one request asks for.
	batchSize = 50
	// errorBackoff is how long the deck waits after a failed fetch, and
	// authBackoff after a refused key, before it tries again.
	errorBackoff = time.Minute
	authBackoff  = 5 * time.Minute
	// maxBody caps the response read.
	maxBody = 4 << 20
)

// idPattern is a Linear issue ID, the shape internal/source/tasks scrapes.
var idPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,5}-\d+$`)

// Key finds the API key: $LINEAR_API_KEY, else the output of Command. The
// command runs without a shell, at most once until Invalidate says the key
// was refused.
type Key struct {
	Getenv  func(string) string
	Command []string // argv; nil means no command
	// Run runs argv and returns its standard output; nil runs it with
	// os/exec. Tests replace it.
	Run     func(ctx context.Context, argv []string) ([]byte, error)
	Timeout time.Duration // zero means CommandTimeout

	mu     sync.Mutex
	ran    bool
	key    string
	err    error
	stale  bool // the key was refused; the command may run again
	source string
}

// errNoKey says that neither source gives a key.
var errNoKey = errors.New("no API key")

// Configured reports whether a key source exists, without running it.
func (k *Key) Configured() bool {
	return k.Getenv != nil && k.Getenv(EnvAPIKey) != "" || len(k.Command) > 0
}

// Get returns the key and where it came from ("$LINEAR_API_KEY" or the
// command's name). The environment wins. A failed command is not run again
// until Invalidate.
func (k *Key) Get(ctx context.Context) (key, source string, err error) {
	if k.Getenv != nil {
		if v := strings.TrimSpace(k.Getenv(EnvAPIKey)); v != "" {
			return v, "$" + EnvAPIKey, nil
		}
	}
	if len(k.Command) == 0 {
		return "", "", errNoKey
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.ran && !k.stale {
		return k.key, k.source, k.err
	}
	k.ran, k.stale = true, false
	k.source = "linear_api_key_command (" + k.Command[0] + ")"
	k.key, k.err = k.runCommand(ctx)
	return k.key, k.source, k.err
}

// Invalidate says the key was refused, so the next Get runs the command
// again (the environment variable is read on every Get anyway).
func (k *Key) Invalidate() {
	k.mu.Lock()
	k.stale = true
	k.mu.Unlock()
}

func (k *Key) runCommand(ctx context.Context) (string, error) {
	timeout := k.Timeout
	if timeout == 0 {
		timeout = CommandTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	run := k.Run
	if run == nil {
		run = execRun
	}
	out, err := run(ctx, k.Command)
	key := firstLine(string(out))
	if err != nil {
		// Never echo the output: it may hold the key.
		if ctx.Err() != nil {
			return "", fmt.Errorf("linear_api_key_command timed out after %v", timeout)
		}
		return "", fmt.Errorf("linear_api_key_command failed: %s", redact(err.Error(), key))
	}
	if key == "" {
		return "", errors.New("linear_api_key_command printed nothing")
	}
	return key, nil
}

// execRun runs argv without a shell. Its error carries the first line of
// standard error, which says why (e.g. op not being signed in).
func execRun(ctx context.Context, argv []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := firstLine(stderr.String()); msg != "" {
			if len(msg) > 120 {
				msg = msg[:120] + "…"
			}
			return out, fmt.Errorf("%w: %s", err, msg)
		}
		return out, err
	}
	return out, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

// redact removes the key from s.
func redact(s, key string) string {
	if key == "" {
		return s
	}
	return strings.ReplaceAll(s, key, "[redacted]")
}

// Reader fetches issue states and caches them. It is safe for concurrent
// use; create it with NewReader or fill Key.
type Reader struct {
	Key *Key
	// HTTP sends the requests; nil uses a client with RequestTimeout.
	HTTP *http.Client
	// Endpoint is the GraphQL URL; empty means Endpoint.
	Endpoint string
	// TTL is how long a status is fresh; zero means TTL.
	TTL time.Duration
	// OnUpdate is called after a background fetch finishes, so the deck
	// reloads and shows what it got. Nil does nothing.
	OnUpdate func()
	// Now is the clock; nil means time.Now.
	Now func() time.Time

	mu       sync.Mutex
	cache    map[string]entry
	inflight bool
	retryAt  time.Time
	problem  string // why the last fetch failed, for Snapshot.Missing
	wg       sync.WaitGroup
}

type entry struct {
	issue *deck.Issue // nil when Linear has no such issue
	at    time.Time
}

// NewReader returns a Reader that takes the key from getenv's
// $LINEAR_API_KEY or from command.
func NewReader(getenv func(string) string, command []string) *Reader {
	return &Reader{Key: &Key{Getenv: getenv, Command: command}}
}

func (r *Reader) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// Apply sets Issue on every Linear link in snap the cache knows, and starts
// a background fetch for the IDs it does not know or knows from longer ago
// than the TTL. It never waits on the network.
func (r *Reader) Apply(snap *deck.Snapshot) {
	links := linearLinks(snap)
	if len(links) == 0 {
		return
	}
	if r.Key == nil || !r.Key.Configured() {
		snap.Notes = append(snap.Notes, "Linear status: off, no API key; set $"+EnvAPIKey+" or linear_api_key_command in the config file")
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	ttl := r.TTL
	if ttl == 0 {
		ttl = TTL
	}
	var stale []string
	seen := map[string]bool{}
	for _, l := range links {
		e, ok := r.cache[l.Label]
		if ok {
			l.Issue = e.issue
		}
		if (!ok || now.Sub(e.at) >= ttl) && !seen[l.Label] {
			seen[l.Label] = true
			stale = append(stale, l.Label)
		}
	}
	if r.problem != "" {
		snap.Missing = append(snap.Missing, "Linear status: "+r.problem)
	}
	if len(stale) == 0 || r.inflight || now.Before(r.retryAt) {
		return
	}
	slices.Sort(stale)
	r.inflight = true
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.Fetch(context.Background(), stale)
		if r.OnUpdate != nil {
			r.OnUpdate()
		}
	}()
}

// Wait waits for a background fetch to finish; tests use it.
func (r *Reader) Wait() { r.wg.Wait() }

// linearLinks returns pointers to every Linear link in snap whose label is
// an issue ID.
func linearLinks(snap *deck.Snapshot) []*deck.Link {
	var out []*deck.Link
	add := func(ls []deck.Link) {
		for i := range ls {
			if ls[i].Kind == deck.LinkLinear && idPattern.MatchString(ls[i].Label) {
				out = append(out, &ls[i])
			}
		}
	}
	for li := range snap.TaskLists {
		for ti := range snap.TaskLists[li].Tasks {
			add(snap.TaskLists[li].Tasks[ti].Links)
		}
	}
	for i := range snap.Threads {
		add(snap.Threads[i].Links)
	}
	return out
}

// Fetch asks Linear for ids now, in batches, and stores the answers in the
// cache. A failure is kept for Snapshot.Missing and delays the next try.
func (r *Reader) Fetch(ctx context.Context, ids []string) {
	err := r.fetch(ctx, ids)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inflight = false
	if err == nil {
		r.problem = ""
		return
	}
	var fe *fetchError
	if !errors.As(err, &fe) {
		fe = &fetchError{msg: err.Error()}
	}
	r.problem = fe.msg
	now := r.now()
	switch {
	case !fe.retryAt.IsZero():
		r.retryAt = fe.retryAt
	case fe.auth:
		r.retryAt = now.Add(authBackoff)
	default:
		r.retryAt = now.Add(errorBackoff)
	}
}

// fetchError is a failed fetch: a message fit for the Sources view (never
// holding the key), whether the key was refused, and when Linear says to
// try again.
type fetchError struct {
	msg     string
	auth    bool
	retryAt time.Time
}

func (e *fetchError) Error() string { return e.msg }

func (r *Reader) fetch(ctx context.Context, ids []string) error {
	key, source, err := r.Key.Get(ctx)
	if err != nil {
		if errors.Is(err, errNoKey) {
			return &fetchError{msg: "no API key; set $" + EnvAPIKey + " or linear_api_key_command in the config file"}
		}
		// The command does not run again by itself: say how to retry.
		return &fetchError{msg: err.Error() + "; restart the deck to try again", retryAt: farFuture}
	}
	for start := 0; start < len(ids); start += batchSize {
		batch := ids[start:min(start+batchSize, len(ids))]
		got, err := r.request(ctx, key, batch)
		if err != nil {
			var fe *fetchError
			if errors.As(err, &fe) {
				fe.msg = redact(fe.msg, key)
				if fe.auth {
					r.Key.Invalidate()
					fe.msg = "Linear refused the API key from " + source + " (" + fe.msg + ")"
				}
				return fe
			}
			return &fetchError{msg: redact(err.Error(), key)}
		}
		now := r.now()
		r.mu.Lock()
		if r.cache == nil {
			r.cache = map[string]entry{}
		}
		for _, id := range batch {
			r.cache[id] = entry{issue: got[id], at: now}
		}
		r.mu.Unlock()
	}
	return nil
}

// farFuture keeps a failed key command from running again.
var farFuture = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)

// query builds one request: per team key, an aliased issues() search for
// its issue numbers, with keys and numbers passed as variables. issues()
// answers an unknown ID with fewer nodes; issue(id:) would fail the whole
// request, since it returns a non-null Issue!.
func query(ids []string) ([]byte, error) {
	var teams []string
	numbers := map[string][]int{}
	for _, id := range ids {
		team, num, _ := strings.Cut(id, "-")
		n, err := strconv.Atoi(num)
		if err != nil {
			return nil, fmt.Errorf("bad issue ID %q", id)
		}
		if _, ok := numbers[team]; !ok {
			teams = append(teams, team)
		}
		numbers[team] = append(numbers[team], n)
	}
	var params, fields []string
	vars := map[string]any{}
	for i, team := range teams {
		k, n := "k"+strconv.Itoa(i), "n"+strconv.Itoa(i)
		params = append(params, "$"+k+": String!", "$"+n+": [Float!]")
		fields = append(fields, fmt.Sprintf("t%d: issues(first: %d, includeArchived: true, filter: { team: { key: { eq: $%s } }, number: { in: $%s } }) { nodes { identifier state { name type } } }",
			i, len(numbers[team]), k, n))
		vars[k] = team
		vars[n] = numbers[team]
	}
	q := "query DeckIssues(" + strings.Join(params, ", ") + ") { " + strings.Join(fields, " ") + " }"
	return json.Marshal(map[string]any{"query": q, "variables": vars})
}

type gqlResponse struct {
	Data   map[string]*gqlIssues `json:"data"`
	Errors []gqlError            `json:"errors"`
}

type gqlIssues struct {
	Nodes []struct {
		Identifier string `json:"identifier"`
		State      *struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"state"`
	} `json:"nodes"`
}

type gqlError struct {
	Message    string `json:"message"`
	Path       []any  `json:"path"`
	Extensions struct {
		Code string `json:"code"`
		Type string `json:"type"`
	} `json:"extensions"`
}

// request asks for one batch. The map holds every ID asked for; an ID
// Linear does not know maps to nil.
func (r *Reader) request(ctx context.Context, key string, ids []string) (map[string]*deck.Issue, error) {
	body, err := query(ids)
	if err != nil {
		return nil, err
	}
	endpoint := r.Endpoint
	if endpoint == "" {
		endpoint = Endpoint
	}
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", key)
	client := r.HTTP
	if client == nil {
		client = &http.Client{Timeout: RequestTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &fetchError{msg: "cannot reach Linear: " + netReason(err)}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, &fetchError{msg: "reading Linear's answer: " + err.Error()}
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, &fetchError{msg: "HTTP " + strconv.Itoa(resp.StatusCode), auth: true}
	case resp.StatusCode == http.StatusTooManyRequests:
		at := r.resetAt(resp.Header)
		return nil, &fetchError{msg: "rate limited until " + at.Local().Format("15:04"), retryAt: at}
	case resp.StatusCode >= 500:
		return nil, &fetchError{msg: "Linear answered HTTP " + strconv.Itoa(resp.StatusCode)}
	}

	var gr gqlResponse
	if err := json.Unmarshal(raw, &gr); err != nil {
		return nil, &fetchError{msg: fmt.Sprintf("Linear's answer (HTTP %d) is not GraphQL JSON", resp.StatusCode)}
	}
	for _, e := range gr.Errors {
		code := strings.ToUpper(e.Extensions.Code)
		switch {
		case code == "AUTHENTICATION_ERROR" || strings.Contains(strings.ToLower(e.Extensions.Type), "authentication"):
			return nil, &fetchError{msg: "authentication error", auth: true}
		case code == "RATELIMITED":
			at := r.resetAt(resp.Header)
			return nil, &fetchError{msg: "rate limited until " + at.Local().Format("15:04"), retryAt: at}
		}
	}
	// Without data the request failed as a whole; with it, an error about
	// one team's search only leaves that team's IDs without a state.
	if gr.Data == nil {
		if len(gr.Errors) > 0 {
			return nil, &fetchError{msg: "Linear: " + oneLine(gr.Errors[0].Message)}
		}
		return nil, &fetchError{msg: "Linear answered HTTP " + strconv.Itoa(resp.StatusCode) + " without data"}
	}

	byID := map[string]*deck.Issue{}
	for _, res := range gr.Data {
		if res == nil {
			continue
		}
		for _, n := range res.Nodes {
			if n.State != nil {
				byID[n.Identifier] = &deck.Issue{State: n.State.Name, StateType: n.State.Type}
			}
		}
	}
	out := map[string]*deck.Issue{}
	for _, id := range ids {
		out[id] = byID[id] // nil: Linear has no such issue
	}
	// A rate limit spent to the last request: wait for its reset.
	if resp.Header.Get("X-RateLimit-Requests-Remaining") == "0" {
		at := r.resetAt(resp.Header)
		r.mu.Lock()
		r.retryAt = at
		r.mu.Unlock()
	}
	return out, nil
}

// resetAt is when Linear's rate limit resets (X-RateLimit-Requests-Reset,
// milliseconds since the epoch, or Retry-After seconds), else in a minute.
func (r *Reader) resetAt(h http.Header) time.Time {
	now := r.now()
	if ms, err := strconv.ParseInt(h.Get("X-RateLimit-Requests-Reset"), 10, 64); err == nil {
		if at := time.UnixMilli(ms); at.After(now) && at.Before(now.Add(2*time.Hour)) {
			return at
		}
	}
	if s, err := strconv.Atoi(h.Get("Retry-After")); err == nil && s > 0 && s < 7200 {
		return now.Add(time.Duration(s) * time.Second)
	}
	return now.Add(errorBackoff)
}

// netReason is a network error without the URL around it.
func netReason(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timed out"
	}
	var ue interface{ Unwrap() error }
	if errors.As(err, &ue) && ue.Unwrap() != nil {
		return oneLine(ue.Unwrap().Error())
	}
	return oneLine(err.Error())
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}

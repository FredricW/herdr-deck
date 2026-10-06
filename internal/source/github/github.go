// Package github reads the thread PRs a snapshot names from GitHub, through
// the user's gh, and lays what it learns over the snapshot's PRs: why a PR
// is not merging (merge state, every check, auto-merge, review requests),
// its unresolved review threads, and on demand a failed job's log tail.
//
// The deck never sees a token: gh holds it, and handles hosts, accounts
// and the keyring. Reads run in the background, batched into one GraphQL
// request per host, and cached per PR, so a reload never waits on the
// network: Apply shows what the cache holds and starts a read for the PRs
// it lacks or holds for too long. The PR the drawer shows (Focus) is read
// again after FocusTTL, the other open PRs after TTL; resolved threads and
// merged or closed PRs are left alone. The PR tab's description and
// conversation (Detail) are asked for only while the tab shows the PR,
// in the same request as the PR's own fields, and refreshed with them. After an error the reader backs off,
// and after a rate limit it waits for the reset.
//
// herdr-projects' ticker reads the same PRs every couple of minutes. The
// newer of the two wins: the deck's read replaces the ticker's state,
// review and failing checks once it is newer, and a newer ticker that
// disagrees makes the deck read the PR again.
package github

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// Defaults for Reader's zero fields.
const (
	// FocusTTL is how often the PR the drawer shows is read; TTL how
	// often the other open PRs are.
	FocusTTL = 45 * time.Second
	TTL      = 3 * time.Minute
	// Timeout bounds one gh call; LogTimeout one log download.
	Timeout    = 15 * time.Second
	LogTimeout = 30 * time.Second
	// batchSize is how many PRs one request asks about.
	batchSize = 10
	// minBackoff is the wait after a first failure, doubling up to
	// maxBackoff; offBackoff the wait while gh is missing or logged out.
	minBackoff = time.Minute
	maxBackoff = 10 * time.Minute
	offBackoff = 5 * time.Minute
	// lowBudget is the GraphQL points left below which the reader waits
	// for the reset: herdr-projects and the user's own gh share it.
	lowBudget = 100
	// maxOutput caps a query's answer, maxLog the log tail kept.
	maxOutput = 4 << 20
	maxLog    = 4 << 20
)

// Reader reads PRs through gh and caches them. It is safe for concurrent
// use; the zero value is ready.
type Reader struct {
	// Run runs gh; nil runs the gh on $PATH.
	Run Runner
	// FocusTTL and TTL override the defaults when non-zero.
	FocusTTL, TTL time.Duration
	// OnUpdate is called after a background read finishes, so the deck
	// reloads and shows it. Nil does nothing.
	OnUpdate func()
	// Now is the clock; nil means time.Now.
	Now func() time.Time

	mu      sync.Mutex
	cache   map[string]*entry
	focus   string
	detail  string
	queue   []prRef
	queued  map[string]bool
	running bool
	retryAt time.Time
	fails   int
	// off says why gh cannot be used at all (missing, logged out), for
	// a Sources note; problem why the last read failed otherwise.
	off, problem string
	logs         map[int64]deck.CheckLog
	wg           sync.WaitGroup
}

type entry struct {
	pr *pullData // nil when GitHub has no such PR for the user
	at time.Time
	// stale asks for a read on the next Apply: a newer ticker disagreed.
	stale bool
}

func (r *Reader) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Reader) run(ctx context.Context, args []string, limit int) ([]byte, error) {
	if r.Run != nil {
		return r.Run(ctx, args, limit)
	}
	return execGh(ctx, args, limit)
}

func (r *Reader) ttl(url string) time.Duration {
	if url == r.focus {
		if r.FocusTTL > 0 {
			return r.FocusTTL
		}
		return FocusTTL
	}
	if r.TTL > 0 {
		return r.TTL
	}
	return TTL
}

// Apply lays what the cache holds over every thread PR in snap and starts
// a background read of the open PRs it lacks or holds for too long. It
// never waits on the network.
func (r *Reader) Apply(snap *deck.Snapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	has := false
	now := r.now()
	var want []prRef
	for i := range snap.Threads {
		t := &snap.Threads[i]
		if t.PR == nil {
			continue
		}
		ref, ok := parsePR(t.PR.URL)
		if !ok {
			continue
		}
		has = true
		e := r.cache[ref.URL]
		if e != nil && e.pr != nil {
			if merge(t.PR, e.pr, e.at) {
				e.stale = true
			}
		}
		if ref.URL == r.detail && t.PR.Detail == nil {
			t.PR.DetailNote = r.detailNote(e)
			if e == nil || e.pr != nil {
				want = append(want, ref)
				continue
			}
		}
		if t.Status == deck.StatusDone && ref.URL != r.focus || !open(t.PR) {
			continue
		}
		if e == nil || e.stale || now.Sub(e.at) >= r.ttl(ref.URL) {
			want = append(want, ref)
		}
	}
	if !has {
		return
	}
	switch {
	case r.off != "":
		snap.Notes = append(snap.Notes, "GitHub: "+r.off+"; PRs show herdr-projects' ticker data only")
	case r.problem != "":
		snap.Missing = append(snap.Missing, "GitHub: "+r.problem)
	}
	r.want(want)
}

// detailNote says why the PR tab's PR has no detail yet: "" while it is
// being read. The caller holds r.mu.
func (r *Reader) detailNote(e *entry) string {
	switch {
	case e != nil && e.pr == nil:
		return "GitHub has no such pull request, or gh's account cannot see it"
	case r.off != "":
		return "GitHub: " + r.off
	case r.problem != "" && !r.running:
		return "could not read it: " + r.problem
	}
	return ""
}

// open reports whether pr is still open as far as anyone knows.
func open(pr *deck.PullRequest) bool {
	return pr.State == "" || pr.State == "OPEN"
}

// merge lays d, read at, over pr. The newer source wins: d replaces the
// ticker's state, review and failing checks when it is newer; when the
// ticker is newer and disagrees, its fields stay, d's checks (which no
// longer match) go, and merge returns true so d is read again.
func merge(pr *deck.PullRequest, d *pullData, at time.Time) (disagrees bool) {
	pr.Live = true
	pr.Draft, pr.Base = d.Draft, d.Base
	pr.MergeState, pr.Mergeable, pr.AutoMerge = d.MergeState, d.Mergeable, d.AutoMerge
	pr.ReviewRequests = d.Requests
	pr.Threads = d.Threads
	pr.Checks = d.Checks
	pr.Detail = d.Detail
	if pr.CheckedAt.IsZero() || !at.Before(pr.CheckedAt) {
		pr.State, pr.Review, pr.FailingChecks = d.State, d.Review, d.Failing()
		pr.CheckedAt = at
		return false
	}
	failing := d.Failing()
	if pr.State != d.State || pr.Review != d.Review || !sameSet(pr.FailingChecks, failing) {
		pr.Checks = nil
		return true
	}
	return false
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// Focus says the drawer shows url's PR now, so it is kept fresher, and
// reads it at once when the cache holds it for longer than FocusTTL. ""
// says no PR is shown.
func (r *Reader) Focus(url string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.focus = url
	ref, ok := parsePR(url)
	if !ok {
		return
	}
	e := r.cache[url]
	if e != nil && e.pr != nil && e.pr.State != "OPEN" {
		return
	}
	if e == nil || e.stale || r.now().Sub(e.at) >= r.ttl(url) {
		r.want([]prRef{ref})
	}
}

// Detail says the PR tab shows url's PR now ("" for none), so its
// description and conversation are read with it: at once when the cache
// has none, and after that with the PR's own reads, which Focus keeps
// fresh. A merged or closed PR is read once.
func (r *Reader) Detail(url string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.detail = url
	ref, ok := parsePR(url)
	if !ok {
		return
	}
	if e := r.cache[url]; e == nil || e.pr != nil && e.pr.Detail == nil {
		r.want([]prRef{ref})
	}
}

// want queues refs and starts the worker, unless the reader backs off.
// The caller holds r.mu.
func (r *Reader) want(refs []prRef) {
	if r.queued == nil {
		r.queued = map[string]bool{}
	}
	for _, ref := range refs {
		if !r.queued[ref.URL] {
			r.queued[ref.URL] = true
			r.queue = append(r.queue, ref)
		}
	}
	if r.running || len(r.queue) == 0 || r.now().Before(r.retryAt) {
		return
	}
	r.running = true
	r.wg.Add(1)
	go r.work()
}

// Wait waits for the background reads to finish; tests use it.
func (r *Reader) Wait() { r.wg.Wait() }

// work reads the queue one batch at a time, the focused PR's first.
func (r *Reader) work() {
	defer r.wg.Done()
	for {
		r.mu.Lock()
		if len(r.queue) == 0 || r.now().Before(r.retryAt) {
			r.running = false
			r.queue, r.queued = nil, nil
			r.mu.Unlock()
			return
		}
		batch := r.take()
		r.mu.Unlock()

		prs, err := r.read(context.Background(), batch)
		r.store(batch, prs, err)
		if r.OnUpdate != nil {
			r.OnUpdate()
		}
	}
}

// take removes the next batch from the queue: the focused PR first, then
// others on its host. The caller holds r.mu.
func (r *Reader) take() []prRef {
	if i := slices.IndexFunc(r.queue, func(p prRef) bool { return p.URL == r.focus }); i > 0 {
		r.queue[0], r.queue[i] = r.queue[i], r.queue[0]
	}
	host := r.queue[0].Host
	var batch, rest []prRef
	for _, ref := range r.queue {
		if ref.Host == host && len(batch) < batchSize {
			ref.Detail = ref.URL == r.detail
			batch = append(batch, ref)
			delete(r.queued, ref.URL)
		} else {
			rest = append(rest, ref)
		}
	}
	r.queue = rest
	return batch
}

// readError is a failed read: a message for the Sources view, whether gh
// cannot be used at all, and when to try again (zero: back off as usual).
type readError struct {
	msg     string
	off     bool
	retryAt time.Time
}

func (e *readError) Error() string { return e.msg }

// read asks GitHub about batch, all on one host.
func (r *Reader) read(ctx context.Context, batch []prRef) ([]*pullData, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	out, runErr := r.run(ctx, query(batch), maxOutput)
	if runErr != nil {
		// `gh api graphql` fails on GraphQL errors too, but still prints
		// the answer, which says more (a rate limit's reset, say).
		if e := r.ghError(runErr); e.off || len(out) == 0 {
			return nil, e
		}
	}
	prs, resp, err := parseResponse(out, len(batch))
	if err != nil {
		if runErr != nil {
			return nil, r.ghError(runErr)
		}
		return nil, &readError{msg: err.Error()}
	}
	for _, e := range resp.Errors {
		if e.Type == "RATE_LIMITED" {
			return nil, r.rateLimited(resp)
		}
	}
	if prs == nil {
		if len(resp.Errors) > 0 {
			return nil, &readError{msg: oneLine(resp.Errors[0].Message)}
		}
		if runErr != nil {
			return nil, r.ghError(runErr)
		}
		return nil, &readError{msg: "GitHub answered without data"}
	}
	if d := resp.Data; d != nil && d.RateLimit != nil && d.RateLimit.Remaining < lowBudget {
		at := d.RateLimit.ResetAt
		r.mu.Lock()
		if at.After(r.now()) && at.Before(r.now().Add(2*time.Hour)) {
			r.retryAt = at
		}
		r.mu.Unlock()
	}
	return prs, nil
}

func (r *Reader) rateLimited(resp gqlResponse) *readError {
	at := r.now().Add(15 * time.Minute)
	if d := resp.Data; d != nil && d.RateLimit != nil && d.RateLimit.ResetAt.After(r.now()) {
		at = d.RateLimit.ResetAt
	}
	return &readError{msg: "rate limited until " + at.Local().Format("15:04"), retryAt: at}
}

// ghError says why a gh call failed, in words for the Sources view.
func (r *Reader) ghError(err error) *readError {
	var re *RunError
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return &readError{msg: "off, gh is not installed (https://cli.github.com)", off: true}
	case errors.Is(err, context.DeadlineExceeded):
		return &readError{msg: "gh timed out"}
	case errors.As(err, &re):
		low := strings.ToLower(re.Stderr)
		switch {
		case re.Code == 4 || strings.Contains(low, "gh auth login") || strings.Contains(low, "not logged in") || strings.Contains(low, "bad credentials"):
			return &readError{msg: "off, gh is not logged in; run gh auth login", off: true}
		case strings.Contains(low, "rate limit"):
			at := r.now().Add(15 * time.Minute)
			return &readError{msg: "rate limited until " + at.Local().Format("15:04"), retryAt: at}
		}
		return &readError{msg: "gh: " + re.Error()}
	}
	return &readError{msg: "gh: " + oneLine(err.Error())}
}

// store keeps a batch's answers, or its failure and the back-off.
func (r *Reader) store(batch []prRef, prs []*pullData, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	if err == nil {
		if r.cache == nil {
			r.cache = map[string]*entry{}
		}
		for i, ref := range batch {
			// A read without the detail keeps the one read before.
			if old := r.cache[ref.URL]; prs[i] != nil && prs[i].Detail == nil && old != nil && old.pr != nil {
				prs[i].Detail = old.pr.Detail
			}
			r.cache[ref.URL] = &entry{pr: prs[i], at: now}
		}
		r.off, r.problem, r.fails = "", "", 0
		return
	}
	var re *readError
	if !errors.As(err, &re) {
		re = &readError{msg: err.Error()}
	}
	r.off, r.problem = "", ""
	switch {
	case re.off:
		r.off = strings.TrimPrefix(re.msg, "off, ")
		r.retryAt = now.Add(offBackoff)
	case !re.retryAt.IsZero():
		r.problem = re.msg
		r.retryAt = re.retryAt
	default:
		r.problem = re.msg
		r.fails++
		wait := minBackoff << min(r.fails-1, 4)
		r.retryAt = now.Add(min(wait, maxBackoff))
	}
}

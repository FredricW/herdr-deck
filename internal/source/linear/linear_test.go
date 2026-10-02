package linear

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// testKey is a made-up key; tests check it never leaks.
const testKey = "lin_api_TESTKEY0123456789"

// fakeLinear answers GraphQL requests from a table of known issues, the
// way Linear does: an unknown ID is a null alias with an error naming it.
// Nothing ever goes over the network.
type fakeLinear struct {
	mu       sync.Mutex
	issues   map[string]deck.Issue
	requests []map[string]string // each request's variables
	auth     []string
	// respond, when set, answers instead of the table, unless it returns
	// neither a response nor an error.
	respond func(req *http.Request) (*http.Response, error)
}

func (f *fakeLinear) RoundTrip(req *http.Request) (*http.Response, error) {
	var body struct {
		Query     string            `json:"query"`
		Variables map[string]string `json:"variables"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.requests = append(f.requests, body.Variables)
	f.auth = append(f.auth, req.Header.Get("Authorization"))
	f.mu.Unlock()
	if f.respond != nil {
		if resp, err := f.respond(req); resp != nil || err != nil {
			return resp, err
		}
	}
	data := map[string]any{}
	var errs []any
	for alias, id := range body.Variables {
		if !strings.Contains(body.Query, alias+": issue(id: $"+alias+")") {
			return nil, fmt.Errorf("query does not ask for %s", alias)
		}
		is, ok := f.issues[id]
		if !ok {
			data[alias] = nil
			errs = append(errs, map[string]any{
				"message":    "Entity not found: Issue",
				"path":       []string{alias},
				"extensions": map[string]any{"type": "invalid input", "code": "INPUT_ERROR"},
			})
			continue
		}
		data[alias] = map[string]any{"identifier": id, "state": map[string]string{"name": is.State, "type": is.StateType}}
	}
	out := map[string]any{"data": data}
	if errs != nil {
		out["errors"] = errs
	}
	return jsonResponse(http.StatusOK, out, nil), nil
}

func (f *fakeLinear) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func jsonResponse(code int, v any, h http.Header) *http.Response {
	b, _ := json.Marshal(v)
	if h == nil {
		h = http.Header{}
	}
	return &http.Response{StatusCode: code, Header: h, Body: io.NopCloser(strings.NewReader(string(b)))}
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time      { return c.t }
func (c *clock) add(d time.Duration) { c.t = c.t.Add(d) }
func newClock() *clock               { return &clock{t: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)} }
func envWith(key string) func(string) string {
	return func(n string) string {
		if n == EnvAPIKey {
			return key
		}
		return ""
	}
}

func newTestReader(f *fakeLinear, c *clock, key *Key) *Reader {
	return &Reader{Key: key, HTTP: &http.Client{Transport: f}, Endpoint: "https://linear.invalid/graphql", Now: c.now}
}

func linear(id string) deck.Link {
	return deck.Link{Kind: deck.LinkLinear, Label: id, URL: "https://linear.app/acme/issue/" + id}
}

func snapshot() deck.Snapshot {
	return deck.Snapshot{
		TaskLists: []deck.TaskList{{Name: "Now", Tasks: []deck.Task{
			{Title: "Users page", Links: []deck.Link{linear("ABC-1"), linear("ABC-2"), {Kind: deck.LinkFigma, Label: "1-2", URL: "https://figma.com/design/x"}}},
		}}},
		Threads: []deck.Thread{{ID: "t-0001", Links: []deck.Link{linear("ABC-2"), linear("ABC-404")}}},
	}
}

// apply runs Apply, waits for the fetch it starts, and applies again.
func apply(r *Reader) deck.Snapshot {
	s := snapshot()
	r.Apply(&s)
	r.Wait()
	s = snapshot()
	r.Apply(&s)
	return s
}

func noLeak(t *testing.T, s deck.Snapshot) {
	t.Helper()
	all := strings.Join(append(append([]string{}, s.Missing...), s.Notes...), "\n")
	if strings.Contains(all, testKey) || strings.Contains(all, "TESTKEY") {
		t.Errorf("the key leaked into the snapshot: %q", all)
	}
}

func TestApplyFetchesAndCaches(t *testing.T) {
	f := &fakeLinear{issues: map[string]deck.Issue{
		"ABC-1": {State: "In Progress", StateType: "started"},
		"ABC-2": {State: "Done", StateType: "completed"},
	}}
	c := newClock()
	r := newTestReader(f, c, &Key{Getenv: envWith(testKey)})
	updates := 0
	r.OnUpdate = func() { updates++ }

	first := snapshot()
	r.Apply(&first)
	if first.TaskLists[0].Tasks[0].Links[0].Issue != nil {
		t.Error("Apply waited for the network; the first snapshot should have no status yet")
	}
	r.Wait()
	if updates != 1 {
		t.Errorf("OnUpdate called %d times, want 1", updates)
	}

	s := snapshot()
	r.Apply(&s)
	task := s.TaskLists[0].Tasks[0]
	if got := task.Links[0].Issue; got == nil || got.State != "In Progress" || got.StateType != "started" {
		t.Errorf("ABC-1 issue = %+v", got)
	}
	if got := task.Links[1].Issue; got == nil || got.State != "Done" {
		t.Errorf("ABC-2 on the task = %+v", got)
	}
	if task.Links[2].Issue != nil {
		t.Error("a Figma link got an issue")
	}
	if got := s.Threads[0].Links[0].Issue; got == nil || got.State != "Done" {
		t.Errorf("ABC-2 on the thread = %+v", got)
	}
	if got := s.Threads[0].Links[1].Issue; got != nil {
		t.Errorf("unknown ABC-404 = %+v, want nil", got)
	}
	if len(s.Missing) != 0 {
		t.Errorf("Missing = %q; an unknown issue is not a failure", s.Missing)
	}

	if f.count() != 1 {
		t.Fatalf("%d requests, want 1 batch", f.count())
	}
	if f.auth[0] != testKey {
		t.Errorf("Authorization = %q", f.auth[0])
	}
	vars := f.requests[0]
	var ids []string
	for _, v := range vars {
		ids = append(ids, v)
	}
	if len(ids) != 3 {
		t.Errorf("asked for %v, want ABC-1, ABC-2 and ABC-404 once each", ids)
	}

	// Within the TTL, the cache answers.
	c.add(TTL - time.Second)
	s = snapshot()
	r.Apply(&s)
	r.Wait()
	if f.count() != 1 {
		t.Errorf("%d requests within the TTL, want 1", f.count())
	}
	// After it, the stale statuses still show while a new fetch runs.
	c.add(2 * time.Second)
	s = snapshot()
	r.Apply(&s)
	if s.TaskLists[0].Tasks[0].Links[0].Issue == nil {
		t.Error("a stale status was dropped before the new fetch finished")
	}
	r.Wait()
	if f.count() != 2 {
		t.Errorf("%d requests after the TTL, want 2", f.count())
	}
}

func TestApplyWithoutKeyIsANote(t *testing.T) {
	f := &fakeLinear{}
	r := newTestReader(f, newClock(), &Key{Getenv: envWith("")})
	s := apply(r)
	if f.count() != 0 {
		t.Errorf("%d requests without a key", f.count())
	}
	if len(s.Missing) != 0 || len(s.Notes) != 1 || !strings.Contains(s.Notes[0], EnvAPIKey) {
		t.Errorf("Missing = %q, Notes = %q; want one note naming %s", s.Missing, s.Notes, EnvAPIKey)
	}

	// No Linear links, no note.
	var empty deck.Snapshot
	r.Apply(&empty)
	if len(empty.Notes) != 0 {
		t.Errorf("Notes = %q for a snapshot without Linear links", empty.Notes)
	}
}

func TestBatches(t *testing.T) {
	f := &fakeLinear{issues: map[string]deck.Issue{}}
	r := newTestReader(f, newClock(), &Key{Getenv: envWith(testKey)})
	var links []deck.Link
	for i := range 120 {
		id := "ABC-" + strconv.Itoa(i+1)
		f.issues[id] = deck.Issue{State: "Todo", StateType: "unstarted"}
		links = append(links, linear(id))
	}
	s := deck.Snapshot{Threads: []deck.Thread{{ID: "t-0001", Links: links}}}
	r.Apply(&s)
	r.Wait()
	if f.count() != 3 {
		t.Errorf("%d requests for 120 IDs, want 3", f.count())
	}
	r.Apply(&s)
	for _, l := range s.Threads[0].Links {
		if l.Issue == nil {
			t.Fatalf("%s has no status", l.Label)
		}
	}
}

func TestRefusedKeyRerunsCommand(t *testing.T) {
	runs := 0
	key := &Key{
		Getenv:  envWith(""),
		Command: []string{"op", "read", "op://Private/Linear/credential"},
		Run: func(_ context.Context, argv []string) ([]byte, error) {
			runs++
			return []byte(testKey + "\n"), nil
		},
	}
	f := &fakeLinear{respond: func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusUnauthorized, map[string]any{"errors": []any{map[string]any{"message": "Authentication required, not authenticated: " + testKey}}}, nil), nil
	}}
	c := newClock()
	r := newTestReader(f, c, key)
	s := apply(r)
	if runs != 1 {
		t.Errorf("the key command ran %d times, want 1", runs)
	}
	if len(s.Missing) != 1 || !strings.Contains(s.Missing[0], "refused the API key from linear_api_key_command (op)") {
		t.Errorf("Missing = %q", s.Missing)
	}
	noLeak(t, s)
	for _, l := range s.TaskLists[0].Tasks[0].Links {
		if l.Issue != nil {
			t.Errorf("%s has a status after a refused key", l.Label)
		}
	}

	// Within the backoff nothing runs or fetches.
	c.add(time.Minute)
	apply(r)
	if f.count() != 1 || runs != 1 {
		t.Errorf("within the backoff: %d requests, %d runs; want 1, 1", f.count(), runs)
	}
	// After it, the command runs again for a new key.
	c.add(authBackoff)
	apply(r)
	if f.count() != 2 || runs != 2 {
		t.Errorf("after the backoff: %d requests, %d runs; want 2, 2", f.count(), runs)
	}
}

func TestGraphQLAuthenticationError(t *testing.T) {
	f := &fakeLinear{respond: func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusBadRequest, map[string]any{"errors": []any{map[string]any{
			"message":    "Authentication required, not authenticated",
			"extensions": map[string]any{"code": "AUTHENTICATION_ERROR", "type": "authentication error"},
		}}}, nil), nil
	}}
	r := newTestReader(f, newClock(), &Key{Getenv: envWith(testKey)})
	s := apply(r)
	if len(s.Missing) != 1 || !strings.Contains(s.Missing[0], "refused the API key from $LINEAR_API_KEY") {
		t.Errorf("Missing = %q", s.Missing)
	}
	noLeak(t, s)
}

func TestRateLimitWaitsForReset(t *testing.T) {
	c := newClock()
	reset := c.t.Add(10 * time.Minute)
	h := http.Header{}
	h.Set("X-RateLimit-Requests-Reset", strconv.FormatInt(reset.UnixMilli(), 10))
	limited := true
	f := &fakeLinear{issues: map[string]deck.Issue{"ABC-1": {State: "Todo", StateType: "unstarted"}}}
	f.respond = func(req *http.Request) (*http.Response, error) {
		if limited {
			return jsonResponse(http.StatusBadRequest, map[string]any{"errors": []any{map[string]any{
				"message": "Rate limit exceeded", "extensions": map[string]any{"code": "RATELIMITED"},
			}}}, h), nil
		}
		return nil, nil
	}
	r := newTestReader(f, c, &Key{Getenv: envWith(testKey)})
	s := apply(r)
	if len(s.Missing) != 1 || !strings.Contains(s.Missing[0], "rate limited until") {
		t.Errorf("Missing = %q", s.Missing)
	}
	c.add(5 * time.Minute)
	apply(r)
	if f.count() != 1 {
		t.Errorf("%d requests before the reset, want 1", f.count())
	}
	limited = false
	c.add(5 * time.Minute)
	s = apply(r)
	if f.count() != 2 {
		t.Errorf("%d requests after the reset, want 2", f.count())
	}
	if len(s.Missing) != 0 {
		t.Errorf("Missing = %q after a good fetch", s.Missing)
	}
	if s.TaskLists[0].Tasks[0].Links[0].Issue == nil {
		t.Error("no status after the rate limit reset")
	}
}

func TestOfflineNeverLeaksKey(t *testing.T) {
	f := &fakeLinear{respond: func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial tcp: lookup api.linear.app: no such host (" + testKey + ")")
	}}
	r := newTestReader(f, newClock(), &Key{Getenv: envWith(testKey)})
	s := apply(r)
	if len(s.Missing) != 1 || !strings.Contains(s.Missing[0], "cannot reach Linear") {
		t.Errorf("Missing = %q", s.Missing)
	}
	noLeak(t, s)
}

func TestServerErrorAndBadJSON(t *testing.T) {
	for _, tt := range []struct {
		name string
		resp *http.Response
		want string
	}{
		{"5xx", jsonResponse(http.StatusBadGateway, map[string]any{}, nil), "HTTP 502"},
		{"not JSON", &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("<html>"))}, "not GraphQL JSON"},
		{"query error", jsonResponse(http.StatusOK, map[string]any{"errors": []any{map[string]any{"message": "Syntax\nerror"}}}, nil), "Linear: Syntax error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeLinear{respond: func(*http.Request) (*http.Response, error) { return tt.resp, nil }}
			s := apply(newTestReader(f, newClock(), &Key{Getenv: envWith(testKey)}))
			if len(s.Missing) != 1 || !strings.Contains(s.Missing[0], tt.want) {
				t.Errorf("Missing = %q, want %q", s.Missing, tt.want)
			}
		})
	}
}

func TestKeyEnvironmentWins(t *testing.T) {
	ran := false
	k := &Key{Getenv: envWith(" " + testKey + " "), Command: []string{"op"}, Run: func(context.Context, []string) ([]byte, error) {
		ran = true
		return []byte("other"), nil
	}}
	key, source, err := k.Get(context.Background())
	if err != nil || key != testKey || source != "$LINEAR_API_KEY" || ran {
		t.Errorf("Get = %q, %q, %v; ran = %v", key, source, err, ran)
	}
}

func TestKeyCommandRunsOnce(t *testing.T) {
	runs := 0
	k := &Key{Getenv: envWith(""), Command: []string{"op", "read", "x"}, Run: func(_ context.Context, argv []string) ([]byte, error) {
		runs++
		if len(argv) != 3 || argv[0] != "op" {
			t.Errorf("argv = %q", argv)
		}
		return []byte("  " + testKey + "\nsecond line\n"), nil
	}}
	for range 3 {
		key, _, err := k.Get(context.Background())
		if err != nil || key != testKey {
			t.Fatalf("Get = %q, %v", key, err)
		}
	}
	if runs != 1 {
		t.Errorf("ran %d times, want 1", runs)
	}
	k.Invalidate()
	_, _, _ = k.Get(context.Background())
	if runs != 2 {
		t.Errorf("ran %d times after Invalidate, want 2", runs)
	}
}

func TestKeyCommandFailure(t *testing.T) {
	runs := 0
	k := &Key{Getenv: envWith(""), Command: []string{"op"}, Run: func(context.Context, []string) ([]byte, error) {
		runs++
		// A failing command that still printed the key must not echo it.
		return []byte(testKey), errors.New("exit status 1: " + testKey)
	}}
	_, _, err := k.Get(context.Background())
	if err == nil || strings.Contains(err.Error(), testKey) {
		t.Errorf("err = %v", err)
	}
	_, _, _ = k.Get(context.Background())
	if runs != 1 {
		t.Errorf("a failed command ran %d times, want 1", runs)
	}

	// The reader says so once and does not retry by itself.
	f := &fakeLinear{}
	c := newClock()
	r := newTestReader(f, c, k)
	s := apply(r)
	if len(s.Missing) != 1 || !strings.Contains(s.Missing[0], "linear_api_key_command failed") || !strings.Contains(s.Missing[0], "restart") {
		t.Errorf("Missing = %q", s.Missing)
	}
	noLeak(t, s)
	c.add(24 * time.Hour)
	apply(r)
	if runs != 1 || f.count() != 0 {
		t.Errorf("%d runs, %d requests; want 1, 0", runs, f.count())
	}
}

func TestKeyCommandEmptyAndTimeout(t *testing.T) {
	k := &Key{Getenv: envWith(""), Command: []string{"op"}, Run: func(context.Context, []string) ([]byte, error) { return []byte("\n"), nil }}
	if _, _, err := k.Get(context.Background()); err == nil || !strings.Contains(err.Error(), "printed nothing") {
		t.Errorf("empty output: err = %v", err)
	}
	k = &Key{Getenv: envWith(""), Command: []string{"op"}, Timeout: 10 * time.Millisecond, Run: func(ctx context.Context, _ []string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	if _, _, err := k.Get(context.Background()); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("slow command: err = %v", err)
	}
}

// execRun really runs a program, without a shell: a ; is just an argument.
func TestExecRunNoShell(t *testing.T) {
	out, err := execRun(context.Background(), []string{"echo", "a;", "$HOME"})
	if err != nil {
		t.Skip("no echo:", err)
	}
	if got := strings.TrimSpace(string(out)); got != "a; $HOME" {
		t.Errorf("output = %q", got)
	}
}

func TestQueryUsesVariables(t *testing.T) {
	b, err := query([]string{"ABC-1", "ABC-22"})
	if err != nil {
		t.Fatal(err)
	}
	var q struct {
		Query     string            `json:"query"`
		Variables map[string]string `json:"variables"`
	}
	if err := json.Unmarshal(b, &q); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(q.Query, "ABC-") {
		t.Errorf("IDs are in the query text: %s", q.Query)
	}
	if q.Variables["i0"] != "ABC-1" || q.Variables["i1"] != "ABC-22" {
		t.Errorf("variables = %v", q.Variables)
	}
}

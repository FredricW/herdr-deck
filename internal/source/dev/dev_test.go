package dev

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/dev/manifest"
)

// shop is a made-up repo's manifest: ports in a state file the project
// writes per worktree, services that its own dev script starts.
const shop = `{
  "version": 1,
  "state": { "file": "$REPO/.dev/$DIRNAME/state.json" },
  "env": { "files": ["$REPO/.env"] },
  "ports": {
    "frontend": { "state": "frontend_port" },
    "api": { "state": "api_port" },
    "db": { "state": "db_port" }
  },
  "services": {
    "frontend": { "title": "Frontend", "log": "$REPO/.dev/$DIRNAME/logs/frontend.log" },
    "api": { "title": "API", "log": "$REPO/.dev/$DIRNAME/logs/api.log" },
    "worker": { "title": "Worker", "log": "$REPO/.dev/$DIRNAME/logs/worker.log" }
  },
  "groups": { "default": ["api", "frontend"] },
  "commands": { "dev": ["scripts/dev", "up"], "stop": ["scripts/dev", "down"] },
  "links": [
    { "title": "Frontend", "url": "http://localhost:$PORT_frontend", "needs": "frontend" },
    { "title": "API docs", "url": "http://localhost:${PORT_api}/docs", "needs": ["api"] },
    { "title": "Both", "url": "http://localhost:$PORT_frontend/?api=$PORT_api" },
    { "title": "Queue on $env(QUEUE_HOST|queue.test)", "url": "https://$env(QUEUE_HOST|queue.test)/$BRANCH" }
  ]
}`

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fakeDial says the listed ports listen and counts the dials.
func fakeDial(n *atomic.Int32, up ...int) func(context.Context, string) error {
	return func(_ context.Context, addr string) error {
		n.Add(1)
		_, p, _ := net.SplitHostPort(addr)
		port, _ := strconv.Atoi(p)
		for _, u := range up {
			if u == port {
				return nil
			}
		}
		return errors.New("connection refused")
	}
}

// repo lays out a main checkout with the shop manifest and a worktree
// next to it, symlinks resolved (macOS's temp folder is behind one).
func repo(t *testing.T) (main, wt string) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	main, wt = filepath.Join(dir, "webshop"), filepath.Join(dir, "worktrees", "t-0003-templates")
	write(t, filepath.Join(main, manifest.Path), shop)
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	return main, wt
}

func TestApplyManifest(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(main, ".dev/t-0003-templates/state.json"), `{"frontend_port": 5181, "api_port": "8011", "db_port": 5441, "other": true}`)
	write(t, filepath.Join(main, ".dev/t-0003-templates/logs/api.log"), "listening\n")
	write(t, filepath.Join(main, ".env"), "QUEUE_HOST=queue.shop.test\n")

	var dials atomic.Int32
	r := &Reader{Prober: Prober{Dial: fakeDial(&dials, 5181, 5441)}}
	snap := deck.Snapshot{Threads: []deck.Thread{
		{ID: "t-0003", Status: deck.StatusWorking, Worktree: wt, Repo: main, Branch: "abc-12-templates", PortToken: 14000},
		{ID: "t-0009", Status: deck.StatusDone, Worktree: wt, Repo: main},
	}}
	r.Apply(context.Background(), &snap)

	th := snap.Threads[0]
	want := []deck.DevServer{
		{Name: "frontend", Title: "Frontend", Port: 5181, Running: true},
		{Name: "api", Title: "API", Port: 8011, Log: filepath.Join(main, ".dev/t-0003-templates/logs/api.log")},
		{Name: "worker", Title: "Worker"},
		// db has no service: it is its own, after the declared ones.
		{Name: "db", Title: "db", Port: 5441, Running: true},
	}
	if !reflect.DeepEqual(th.DevServers, want) {
		t.Errorf("servers\n%+v, want\n%+v", th.DevServers, want)
	}
	wantLinks := []deck.Link{
		{Kind: deck.LinkLocalhost, Label: "Frontend", URL: "http://localhost:5181"},
		{Kind: deck.LinkLocalhost, Label: "API docs", URL: "http://localhost:8011/docs", Down: true},
		{Kind: deck.LinkLocalhost, Label: "Both", URL: "http://localhost:5181/?api=8011", Down: true},
		{Kind: deck.LinkLocalhost, Label: "Queue on queue.shop.test", URL: "https://queue.shop.test/abc-12-templates"},
	}
	if !reflect.DeepEqual(th.Links, wantLinks) {
		t.Errorf("links\n%+v, want\n%+v", th.Links, wantLinks)
	}
	if th.DevNote != "" || len(snap.Missing) > 0 || len(snap.Notes) > 0 {
		t.Errorf("note %q, missing %q, notes %q", th.DevNote, snap.Missing, snap.Notes)
	}
	if snap.Threads[1].DevServers != nil {
		t.Errorf("a resolved thread got servers: %+v", snap.Threads[1].DevServers)
	}
	if n := dials.Load(); n != 6 {
		t.Errorf("%d dials, want 6 (3 ports on IPv4 and IPv6)", n)
	}

	// Within the cache's lifetime the ports are not probed again.
	again := deck.Snapshot{Threads: []deck.Thread{{ID: "t-0003", Worktree: wt, Repo: main}}}
	r.Apply(context.Background(), &again)
	if n := dials.Load(); n != 6 {
		t.Errorf("%d dials after a cached reload, want 6", n)
	}
}

// The lookup order of spec section 2.2, with the legacy file's note.
func TestApplyLookup(t *testing.T) {
	main, wt := repo(t)
	apply := func() deck.Snapshot {
		r := &Reader{Prober: Prober{Dial: fakeDial(new(atomic.Int32), 6060)}}
		snap := deck.Snapshot{Threads: []deck.Thread{{ID: "t-0003", Worktree: wt, Repo: main}}}
		r.Apply(context.Background(), &snap)
		return snap
	}
	names := func(s deck.Snapshot) string {
		var out []string
		for _, d := range s.Threads[0].DevServers {
			out = append(out, d.Name)
		}
		return strings.Join(out, " ")
	}

	// A legacy file in the worktree loses to the main checkout's
	// .config/dev.json.
	write(t, filepath.Join(wt, manifest.LegacyPath), `{"state": {"ports": {"docs": 6060}}}`)
	if s := apply(); names(s) != "" || !strings.HasPrefix(s.Threads[0].DevNote, "not started: no ") {
		t.Errorf("legacy in the worktree: servers %q, note %q", names(s), s.Threads[0].DevNote)
	}

	// Without .config/dev.json, the legacy file works, with a note.
	if err := os.Remove(filepath.Join(main, manifest.Path)); err != nil {
		t.Fatal(err)
	}
	s := apply()
	want := deck.DevServer{Name: "docs", Title: "docs", Port: 6060, Running: true}
	if len(s.Threads[0].DevServers) != 1 || s.Threads[0].DevServers[0] != want {
		t.Errorf("legacy: servers %+v", s.Threads[0].DevServers)
	}
	if len(s.Notes) != 1 || !strings.Contains(s.Notes[0], "t-0003-templates/.herdr-deck/dev.json is legacy") {
		t.Errorf("legacy: notes %q", s.Notes)
	}
	// Without links, each port gets one.
	if l := s.Threads[0].Links; len(l) != 1 || l[0] != (deck.Link{Kind: deck.LinkLocalhost, Label: ":6060 docs", URL: "http://localhost:6060"}) {
		t.Errorf("legacy: links %+v", l)
	}

	// The worktree's own .config/dev.json wins over everything.
	write(t, filepath.Join(main, manifest.Path), shop)
	write(t, filepath.Join(wt, manifest.Path), `{"version": 1, "ports": {"web": 6060}}`)
	if s := apply(); names(s) != "web" || len(s.Notes) > 0 {
		t.Errorf("worktree file: servers %q, notes %q", names(s), s.Notes)
	}
}

// A broken file is reported, and the deck does not fall back to the next
// one in the lookup.
func TestApplyBrokenDoesNotFallBack(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(wt, manifest.Path), `{"version": 1, "services": {"web": {"run": "PORT=$HOME npm run dev"}}}`)
	write(t, filepath.Join(main, ".dev/t-0003-templates/state.json"), `{"frontend_port": 5181}`)
	r := &Reader{Prober: Prober{Dial: fakeDial(new(atomic.Int32))}}
	snap := deck.Snapshot{Threads: []deck.Thread{{ID: "t-0003", Worktree: wt, Repo: main, PortToken: 14437}}}
	r.Apply(context.Background(), &snap)
	if len(snap.Missing) != 1 || !strings.Contains(snap.Missing[0], "t-0003-templates/.config/dev.json: services.web.run: unknown variable $HOME") {
		t.Errorf("missing %q", snap.Missing)
	}
	// Degrade: the fallback port still shows, not the main checkout's.
	th := snap.Threads[0]
	if len(th.DevServers) != 1 || !th.DevServers[0].Fallback {
		t.Errorf("servers %+v, want the fallback port", th.DevServers)
	}
}

func TestApplyNotStartedFallsBackToPortToken(t *testing.T) {
	main, wt := repo(t)
	var dials atomic.Int32
	r := &Reader{Prober: Prober{Dial: fakeDial(&dials, 14437)}}
	snap := deck.Snapshot{Threads: []deck.Thread{
		{ID: "t-0003", Worktree: wt, Repo: main, PortToken: 14437},
		{ID: "t-0004", Worktree: wt, Repo: main},
	}}
	r.Apply(context.Background(), &snap)

	th := snap.Threads[0]
	if len(th.DevServers) != 1 || th.DevServers[0] != (deck.DevServer{Name: "port", Port: 14437, Running: true, Fallback: true}) {
		t.Errorf("servers %+v", th.DevServers)
	}
	// The link without ports still shows.
	if len(th.Links) != 2 || th.Links[0].Label != "Queue on queue.test" || th.Links[1].URL != "http://localhost:14437" || th.Links[1].Down {
		t.Errorf("links %+v", th.Links)
	}
	other := snap.Threads[1]
	if other.DevServers != nil || other.DevNote != "not started: no .dev/t-0003-templates/state.json" {
		t.Errorf("servers %+v, note %q", other.DevServers, other.DevNote)
	}
	if len(snap.Missing) > 0 {
		t.Errorf("missing %q: a server not started yet is not a missing source", snap.Missing)
	}
}

func TestApplyWithoutManifest(t *testing.T) {
	dir := t.TempDir()
	main, wt := filepath.Join(dir, "billing"), filepath.Join(dir, "wt1")
	for _, d := range []string{main, wt} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r := &Reader{Prober: Prober{Dial: fakeDial(new(atomic.Int32))}}
	snap := deck.Snapshot{Threads: []deck.Thread{
		{ID: "t-0001", Worktree: wt, Repo: main},
		{ID: "t-0002", Worktree: wt, Repo: main},
		{ID: "t-0003", Worktree: filepath.Join(dir, "gone"), Repo: main},
	}}
	r.Apply(context.Background(), &snap)
	if snap.Threads[0].DevNote != "no .config/dev.json" || snap.Threads[2].DevNote != "worktree not found" {
		t.Errorf("notes %q, %q", snap.Threads[0].DevNote, snap.Threads[2].DevNote)
	}
	if len(snap.Notes) != 1 || !strings.Contains(snap.Notes[0], "billing has no .config/dev.json") {
		t.Errorf("notes %q, want one line for the repo", snap.Notes)
	}
	if len(snap.Missing) > 0 {
		t.Errorf("missing %q", snap.Missing)
	}
}

// Fixed ports are known everywhere; port store ports are not read yet, so
// they are named in the Sources view and their links are left out.
func TestApplyFixedAndStorePorts(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(wt, manifest.Path), `{"version": 1,
		"ports": {"web": {"base": 3100}, "mail": 8025, "docs": {"fixed": 6060}},
		"services": {"web": {"run": "npm run dev"}},
		"links": [{"title": "Shop", "url": "http://localhost:$PORT_web"}, {"title": "Mail", "url": "http://localhost:$PORT_mail", "needs": "mail"}]}`)
	r := &Reader{Prober: Prober{Dial: fakeDial(new(atomic.Int32), 8025)}}
	snap := deck.Snapshot{Threads: []deck.Thread{{ID: "t-0003", Worktree: wt, Repo: main}}}
	r.Apply(context.Background(), &snap)
	th := snap.Threads[0]
	want := []deck.DevServer{
		{Name: "web", Title: "web", Pending: "port store not supported yet"},
		{Name: "mail", Title: "mail", Port: 8025, Running: true},
		{Name: "docs", Title: "docs", Port: 6060},
	}
	if !reflect.DeepEqual(th.DevServers, want) {
		t.Errorf("servers\n%+v, want\n%+v", th.DevServers, want)
	}
	if len(th.Links) != 1 || th.Links[0].Label != "Mail" || th.Links[0].Down {
		t.Errorf("links %+v", th.Links)
	}
	if len(snap.Notes) != 1 || !strings.Contains(snap.Notes[0], "ports web come from the shared port store, which the deck does not support yet") {
		t.Errorf("notes %q", snap.Notes)
	}
}

func TestApplyStateFileProblems(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(main, ".dev/t-0003-templates/state.json"), `[1, 2`)
	r := &Reader{Prober: Prober{Dial: fakeDial(new(atomic.Int32))}}
	snap := deck.Snapshot{Threads: []deck.Thread{{ID: "t-0003", Worktree: wt, Repo: main}}}
	r.Apply(context.Background(), &snap)
	if len(snap.Missing) != 1 || !strings.Contains(snap.Missing[0], "state.json: unexpected end of JSON input") {
		t.Errorf("missing %q", snap.Missing)
	}
}

// A dev command of null says the repo has no dev server: no Dev line.
func TestApplyNullDev(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(wt, manifest.Path), `{"version": 1, "commands": {"dev": null, "test": "make test"}}`)
	r := &Reader{Prober: Prober{Dial: fakeDial(new(atomic.Int32))}}
	snap := deck.Snapshot{Threads: []deck.Thread{{ID: "t-0003", Worktree: wt, Repo: main}}}
	r.Apply(context.Background(), &snap)
	if th := snap.Threads[0]; th.DevNote != "" || th.DevServers != nil || th.Links != nil {
		t.Errorf("thread %+v", th)
	}
}

func TestApplyReadyHTTP(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(wt, manifest.Path), `{"version": 1,
		"ports": {"api": 8100, "web": 3100},
		"services": {"api": {"run": "x", "ready": {"http": "/healthz"}}, "web": {"run": "y"}}}`)
	var gets []string
	r := &Reader{Prober: Prober{
		Dial: fakeDial(new(atomic.Int32), 8100, 3100),
		Get: func(_ context.Context, url string) (int, error) {
			gets = append(gets, url)
			return 503, nil
		},
	}}
	snap := deck.Snapshot{Threads: []deck.Thread{{ID: "t-0003", Worktree: wt, Repo: main}}}
	r.Apply(context.Background(), &snap)
	s := snap.Threads[0].DevServers
	// api's port answers, but /healthz says not yet.
	if s[0].Running || !s[1].Running {
		t.Errorf("servers %+v", s)
	}
	if len(gets) != 2 || gets[0] != "http://127.0.0.1:8100/healthz" || gets[1] != "http://[::1]:8100/healthz" {
		t.Errorf("gets %q", gets)
	}
}

func TestProberIPv6Only(t *testing.T) {
	var dials atomic.Int32
	p := Prober{Dial: func(_ context.Context, addr string) error {
		dials.Add(1)
		if addr == "[::1]:5173" {
			return nil
		}
		return errors.New("connection refused")
	}}
	if got := p.Listening(context.Background(), []int{5173}); !got[5173] {
		t.Errorf("a server on ::1 only reads as down")
	}
}

func TestProberReal(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("cannot listen:", err)
	}
	up := ln.Addr().(*net.TCPAddr).Port
	ln2, _ := net.Listen("tcp", "127.0.0.1:0")
	down := ln2.Addr().(*net.TCPAddr).Port
	ln2.Close()
	defer ln.Close()

	var p Prober
	start := time.Now()
	got := p.Listening(context.Background(), []int{up, down, up})
	if !got[up] || got[down] {
		t.Errorf("listening %v: want %d up, %d down", got, up, down)
	}
	if d := time.Since(start); d > 2*ProbeTimeout {
		t.Errorf("probing took %v", d)
	}
}

func TestProberCacheExpires(t *testing.T) {
	var dials atomic.Int32
	at := time.Unix(0, 0)
	p := Prober{Dial: fakeDial(&dials), Now: func() time.Time { return at }}
	p.Listening(context.Background(), []int{1234})
	p.Listening(context.Background(), []int{1234})
	at = at.Add(probeTTL)
	p.Listening(context.Background(), []int{1234})
	if n := dials.Load(); n != 4 {
		t.Errorf("%d dials, want 4 (twice on IPv4 and IPv6)", n)
	}
}

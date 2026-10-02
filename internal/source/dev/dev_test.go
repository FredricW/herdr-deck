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
)

// manifest is a made-up repo's manifest: ports in a state file under the
// main checkout, one per worktree.
const manifest = `{
  "state": {
    "file": ".dev/$DIRNAME/state.json",
    "ports": { "frontend": "frontend_port", "api": "api_port", "db": "db_port" }
  },
  "links": [
    { "title": "Frontend", "url": "http://localhost:$PORT_frontend", "needs": "frontend" },
    { "title": "API docs", "url": "http://localhost:${PORT_api}/docs", "needs": ["api"] },
    { "title": "Both", "url": "http://localhost:$PORT_frontend/?api=$PORT_api" }
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

// repo lays out a main checkout and a worktree next to it.
func repo(t *testing.T) (main, wt string) {
	dir := t.TempDir()
	main, wt = filepath.Join(dir, "webshop"), filepath.Join(dir, "worktrees", "t-0003-templates")
	write(t, filepath.Join(main, ManifestPath), manifest)
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	return main, wt
}

func TestApplyManifest(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(main, ".dev/t-0003-templates/state.json"), `{"frontend_port": 5181, "api_port": "8011", "db_port": 5441, "other": true}`)

	var dials atomic.Int32
	r := &Reader{Prober: Prober{Dial: fakeDial(&dials, 5181, 5441)}}
	snap := deck.Snapshot{Threads: []deck.Thread{
		{ID: "t-0003", Status: deck.StatusWorking, Worktree: wt, Repo: main, PortToken: 14000},
		{ID: "t-0009", Status: deck.StatusDone, Worktree: wt, Repo: main},
	}}
	r.Apply(context.Background(), &snap)

	th := snap.Threads[0]
	want := []deck.DevServer{{Name: "frontend", Port: 5181, Running: true}, {Name: "api", Port: 8011}, {Name: "db", Port: 5441, Running: true}}
	if !reflect.DeepEqual(th.DevServers, want) {
		t.Errorf("servers %+v, want %+v", th.DevServers, want)
	}
	wantLinks := []deck.Link{
		{Kind: deck.LinkLocalhost, Label: "Frontend", URL: "http://localhost:5181"},
		{Kind: deck.LinkLocalhost, Label: "API docs", URL: "http://localhost:8011/docs", Down: true},
		{Kind: deck.LinkLocalhost, Label: "Both", URL: "http://localhost:5181/?api=8011", Down: true},
	}
	if !reflect.DeepEqual(th.Links, wantLinks) {
		t.Errorf("links %+v, want %+v", th.Links, wantLinks)
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

func TestApplyWorktreeManifestWins(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(wt, ManifestPath), `{"state": {"ports": {"docs": 6060}}}`)
	var dials atomic.Int32
	r := &Reader{Prober: Prober{Dial: fakeDial(&dials, 6060)}}
	snap := deck.Snapshot{Threads: []deck.Thread{{ID: "t-0003", Worktree: wt, Repo: main}}}
	r.Apply(context.Background(), &snap)
	th := snap.Threads[0]
	if len(th.DevServers) != 1 || th.DevServers[0] != (deck.DevServer{Name: "docs", Port: 6060, Running: true}) {
		t.Errorf("servers %+v", th.DevServers)
	}
	// Without links, each server gets one.
	want := deck.Link{Kind: deck.LinkLocalhost, Label: ":6060 docs", URL: "http://localhost:6060"}
	if len(th.Links) != 1 || th.Links[0] != want {
		t.Errorf("links %+v", th.Links)
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
	if len(th.Links) != 1 || th.Links[0].URL != "http://localhost:14437" || th.Links[0].Down {
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
	if snap.Threads[0].DevNote != "no .herdr-deck/dev.json" || snap.Threads[2].DevNote != "worktree not found" {
		t.Errorf("notes %q, %q", snap.Threads[0].DevNote, snap.Threads[2].DevNote)
	}
	if len(snap.Notes) != 1 || !strings.Contains(snap.Notes[0], "billing has no .herdr-deck/dev.json") {
		t.Errorf("notes %q, want one line for the repo", snap.Notes)
	}
	if len(snap.Missing) > 0 {
		t.Errorf("missing %q", snap.Missing)
	}
}

func TestApplyInvalid(t *testing.T) {
	for _, c := range []struct {
		name, manifest, state, want string
	}{
		{"syntax", `{"state": `, "", "webshop/.herdr-deck/dev.json: unexpected end of JSON input"},
		{"no ports", `{"state": {"file": "s.json"}}`, "", "state.ports names no servers"},
		{"no file", `{"state": {"ports": {"api": "api_port"}}}`, "", "state.file is empty"},
		{"unknown need", `{"state": {"file": "s.json", "ports": {"api": "api_port"}}, "links": [{"url": "http://localhost:$PORT_api", "needs": "web"}]}`, "", `needs "web"`},
		{"unknown port", `{"state": {"file": "s.json", "ports": {"api": "api_port"}}, "links": [{"url": "http://localhost:$PORT_web"}]}`, "", "$PORT_web"},
		{"bad port", `{"state": {"ports": {"api": 70000}}}`, "", "not a port number"},
		{"unknown var", `{"state": {"file": "$HOME/s.json", "ports": {"api": "api_port"}}}`, "", "state.file: unknown $HOME"},
		{"link var", `{"state": {"ports": {"api": 8000}}, "links": [{"url": "http://localhost:$PORT_api/$metadata"}]}`, "", "links[0]: unknown $metadata (write $$ for a literal $)"},
		{"bad state", `{"state": {"file": "s.json", "ports": {"api": "api_port"}}}`, "[1, 2", "s.json: unexpected end of JSON input"},
	} {
		t.Run(c.name, func(t *testing.T) {
			main, wt := repo(t)
			write(t, filepath.Join(main, ManifestPath), c.manifest)
			if c.state != "" {
				write(t, filepath.Join(main, "s.json"), c.state)
			}
			r := &Reader{Prober: Prober{Dial: fakeDial(new(atomic.Int32))}}
			snap := deck.Snapshot{Threads: []deck.Thread{{ID: "t-0003", Worktree: wt, Repo: main, PortToken: 14437}}}
			r.Apply(context.Background(), &snap)
			if len(snap.Missing) != 1 || !strings.Contains(snap.Missing[0], c.want) {
				t.Errorf("missing %q, want a line with %q", snap.Missing, c.want)
			}
			// Degrade: the fallback port still shows.
			if th := snap.Threads[0]; len(th.DevServers) != 1 || !th.DevServers[0].Fallback {
				t.Errorf("servers %+v, want the fallback port", th.DevServers)
			}
		})
	}
}

func TestApplyLinkNeedsUnstartedServer(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(main, ManifestPath), `{"state": {"file": "s.json", "ports": {"web": "web_port", "api": "api_port"}},
		"links": [{"title": "Web", "url": "http://localhost:$PORT_web/$$top", "needs": ["web", "api"]}]}`)
	write(t, filepath.Join(main, "s.json"), `{"web_port": 5181}`)
	var dials atomic.Int32
	r := &Reader{Prober: Prober{Dial: fakeDial(&dials, 5181)}}
	snap := deck.Snapshot{Threads: []deck.Thread{{ID: "t-0003", Worktree: wt, Repo: main}}}
	r.Apply(context.Background(), &snap)
	want := deck.Link{Kind: deck.LinkLocalhost, Label: "Web", URL: "http://localhost:5181/$top", Down: true}
	if th := snap.Threads[0]; len(th.Links) != 1 || th.Links[0] != want {
		t.Errorf("links %+v, want %+v", th.Links, want)
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

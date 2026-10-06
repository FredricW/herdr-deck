package dev

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/dev/manifest"
)

// world fakes processes and ports: Start records a command and, after it,
// makes the ports in opens[command name] answer. Nothing really runs.
type world struct {
	mu      sync.Mutex
	cmds    []Command
	alive   map[int]bool
	up      map[int]bool
	opens   map[string][]int
	signals []string // "<pid> TERM" or "<pid> KILL"
	runs    []Command
	// ignoreTerm keeps a pid alive after SIGTERM.
	ignoreTerm map[int]bool
}

func newWorld(up ...int) *world {
	w := &world{alive: map[int]bool{}, up: map[int]bool{}, opens: map[string][]int{}, ignoreTerm: map[int]bool{}}
	for _, p := range up {
		w.up[p] = true
	}
	return w
}

func (w *world) start(c Command) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cmds = append(w.cmds, c)
	pid := 4200 + len(w.cmds)
	w.alive[pid] = true
	for _, p := range w.opens[c.Name] {
		w.up[p] = true
	}
	return pid, nil
}

func (w *world) dial(_ context.Context, addr string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, p, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(p)
	if w.up[port] {
		return nil
	}
	return errors.New("connection refused")
}

func (w *world) signal(pid int, force bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if force {
		w.signals = append(w.signals, strconv.Itoa(pid)+" KILL")
		w.alive[pid] = false
		return nil
	}
	w.signals = append(w.signals, strconv.Itoa(pid)+" TERM")
	if !w.ignoreTerm[pid] {
		w.alive[pid] = false
	}
	return nil
}

func (w *world) isAlive(pid int, _ time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.alive[pid]
}

func (w *world) run(_ context.Context, c Command) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.runs = append(w.runs, c)
	return nil
}

func (w *world) names() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []string
	for _, c := range w.cmds {
		out = append(out, c.Name)
	}
	return out
}

func (w *world) reader(t *testing.T) *Reader {
	t.Helper()
	return &Reader{
		Prober: Prober{Dial: w.dial},
		State:  filepath.Join(t.TempDir(), "state", "dev-manifest"),
		Start:  w.start,
		Run:    w.run,
		Signal: w.signal,
		Alive:  w.isAlive,
	}
}

func env(c Command, key string) (string, bool) {
	for _, kv := range c.Env {
		if k, v, _ := strings.Cut(kv, "="); k == key {
			return v, true
		}
	}
	return "", false
}

func TestUpRunsDevCommandOnce(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(main, ".dev/t-0003-templates/state.json"), `{"frontend_port": 5181, "api_port": 8011}`)
	w := newWorld()
	r := w.reader(t)
	th := deck.Thread{ID: "t-0003", Status: deck.StatusWorking, Worktree: wt, Repo: main, Branch: "abc-12-templates"}
	t.Setenv("HERDR_PANE_ID", "w1:p1")

	msg, err := r.Up(context.Background(), th)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.cmds) != 1 {
		t.Fatalf("started %d commands, want 1", len(w.cmds))
	}
	c := w.cmds[0]
	// argv[0] with a / is relative to the command's folder.
	if c.Name != "dev.default" || !reflect.DeepEqual(c.Argv, []string{filepath.Join(wt, "scripts/dev"), "up"}) || c.Dir != wt {
		t.Errorf("command %+v", c)
	}
	key := Key(wt)
	if log := filepath.Join(r.State, "logs", key, "dev.default.log"); c.Log != log {
		t.Errorf("log %q, want %q", c.Log, log)
	}
	for k, want := range map[string]string{
		"DEV_GROUP": "default", "DEV_TOOL": "herdr-deck", "DEV_WORKTREE": wt, "DEV_REPO": main,
		"DEV_DIRNAME": "t-0003-templates", "DEV_BRANCH": "abc-12-templates",
		"PORT_frontend": "5181", "PORT_api": "8011",
	} {
		if got, _ := env(c, k); got != want {
			t.Errorf("%s=%q, want %q", k, got, want)
		}
	}
	for _, k := range []string{"HERDR_PANE_ID", "PORT_db", "PORT", "DEV_SERVICE"} {
		if v, ok := env(c, k); ok {
			t.Errorf("%s=%q is set", k, v)
		}
	}
	if !strings.Contains(msg, "started") || !strings.Contains(msg, "t-0003") {
		t.Errorf("status %q", msg)
	}

	// The run record is the spec's, so other tools see it.
	rec, ok := r.store().Record(key, "dev.default")
	if !ok || rec.PID != 4201 || rec.Worktree != wt || rec.Tool != "herdr-deck" || rec.Version != 1 || rec.Command != filepath.Join(wt, "scripts/dev")+" up" || rec.StartedAt().IsZero() {
		t.Errorf("record %+v", rec)
	}

	// The drawer learns about it from the record, also after a restart.
	snap := deck.Snapshot{Threads: []deck.Thread{th}}
	r.Apply(context.Background(), &snap)
	if u := snap.Threads[0].DevUp; u == nil || !u.Alive || u.Log != c.Log {
		t.Fatalf("DevUp = %+v, want alive with log %q", u, c.Log)
	}

	// Pressed again while it runs: not started twice.
	msg, err = r.Up(context.Background(), th)
	if err != nil || len(w.cmds) != 1 || !strings.Contains(msg, "still runs (pid 4201)") {
		t.Errorf("second Up: %q, %v, %d commands", msg, err, len(w.cmds))
	}

	// Once it has exited, u starts it again.
	w.alive[4201] = false
	if _, err := r.Up(context.Background(), th); err != nil || len(w.cmds) != 2 {
		t.Errorf("Up after exit: %v, %d commands, want 2", err, len(w.cmds))
	}

	// With the default group ready, dev does not run.
	w.alive[4202] = false
	w.up[5181], w.up[8011] = true, true
	msg, err = r.Up(context.Background(), th)
	if err != nil || len(w.cmds) != 2 || !strings.Contains(msg, "already answer (api, frontend)") {
		t.Errorf("Up when ready: %q, %v, %d commands", msg, err, len(w.cmds))
	}
}

// services is a made-up monorepo without a dev command: dev starts the
// default group's services and what they need, in needs order.
const services = `{
  "version": 1,
  "path": ["bin"],
  "env": { "vars": { "APP_ENV": "dev", "API_URL": "http://localhost:$PORT_api" } },
  "ports": { "web": 3100, "api": 8100, "db": 5432, "docs": 6060 },
  "services": {
    "web": { "dir": "apps/web", "run": "pnpm dev --port $$PORT", "needs": "api", "env": { "APP_ENV": "web" } },
    "api": { "run": ["go", "run", "./cmd/api"], "needs": ["db", "cache"], "ready": { "timeout": "300ms" }, "log": ".dev/api.log" },
    "cache": { "run": "redis-server" },
    "db": { "run": ["scripts/db"], "ready": { "timeout": "300ms" } },
    "docs": { "run": "godoc" }
  },
  "groups": { "default": ["web"] }
}`

func TestUpStartsServicesInNeedsOrder(t *testing.T) {
	pollEvery = 10 * time.Millisecond
	main, wt := repo(t)
	write(t, filepath.Join(wt, manifest.Path), services)
	write(t, filepath.Join(wt, "bin", "go"), "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(wt, "bin", "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	w := newWorld()
	w.opens = map[string][]int{"service.db": {5432}, "service.api": {8100}, "service.web": {3100}}
	r := w.reader(t)
	th := deck.Thread{ID: "t-0003", Worktree: wt, Repo: main}

	msg, err := r.Up(context.Background(), th)
	if err != nil {
		t.Fatal(err)
	}
	// cache has no port: it is ready once it runs. docs is not in the
	// default group.
	if got, want := w.names(), []string{"service.db", "service.cache", "service.api", "service.web"}; !reflect.DeepEqual(got, want) {
		t.Errorf("started %q, want %q", got, want)
	}
	if msg != "started db, cache, api, web for t-0003" {
		t.Errorf("status %q", msg)
	}
	web, api := w.cmds[3], w.cmds[2]
	if web.Dir != filepath.Join(wt, "apps/web") || web.Shell != "pnpm dev --port $PORT" {
		t.Errorf("web %+v", web)
	}
	for k, want := range map[string]string{"PORT": "3100", "PORT_web": "3100", "PORT_db": "5432", "DEV_SERVICE": "web", "APP_ENV": "web", "API_URL": "http://localhost:8100"} {
		if got, _ := env(web, k); got != want {
			t.Errorf("web %s=%q, want %q", k, got, want)
		}
	}
	// argv[0] is looked up in PATH with the manifest's path in front.
	if api.Argv[0] != filepath.Join(wt, "bin", "go") || api.Log != filepath.Join(wt, ".dev/api.log") {
		t.Errorf("api %+v", api)
	}
	if path, _ := env(api, "PATH"); !strings.HasPrefix(path, filepath.Join(wt, "bin")+string(os.PathListSeparator)) {
		t.Errorf("api PATH %q", path)
	}
	if got, _ := env(api, "PORT"); got != "8100" {
		t.Errorf("api PORT %q", got)
	}
	if db := w.cmds[0]; db.Log != filepath.Join(r.State, "logs", Key(wt), "service.db.log") {
		t.Errorf("db log %q", db.Log)
	}

	// The drawer shows each one's state from its record and port.
	snap := deck.Snapshot{Threads: []deck.Thread{th}}
	r.Prober = Prober{Dial: w.dial} // a fresh cache
	r.Apply(context.Background(), &snap)
	var states []string
	for _, s := range snap.Threads[0].DevServers {
		state := "stopped"
		switch {
		case s.Running:
			state = "ready"
		case s.Starting:
			state = "starting"
		case s.Exited:
			state = "exited"
		}
		states = append(states, s.Name+" "+state)
	}
	if want := []string{"web ready", "api ready", "cache ready", "db ready", "docs stopped"}; !reflect.DeepEqual(states, want) {
		t.Errorf("states %q, want %q", states, want)
	}

	// Again: everything is ready or starting, nothing starts twice.
	msg, _ = r.Up(context.Background(), th)
	if len(w.cmds) != 4 || !strings.Contains(msg, "already run") {
		t.Errorf("second Up: %q, %d commands", msg, len(w.cmds))
	}
}

func TestUpStopsAtAServiceThatIsNotReady(t *testing.T) {
	pollEvery = 10 * time.Millisecond
	main, wt := repo(t)
	write(t, filepath.Join(wt, manifest.Path), services)
	w := newWorld()
	// db starts but its port never answers.
	r := w.reader(t)
	th := deck.Thread{ID: "t-0003", Worktree: wt, Repo: main}
	msg, err := r.Up(context.Background(), th)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := w.names(), []string{"service.db", "service.cache"}; !reflect.DeepEqual(got, want) {
		t.Errorf("started %q, want %q", got, want)
	}
	if !strings.Contains(msg, "api not started: db is not ready after 300ms") || !strings.Contains(msg, "web not started: api was not started") {
		t.Errorf("status %q", msg)
	}
}

func TestUpMessages(t *testing.T) {
	main, wt := repo(t)
	w := newWorld()
	r := w.reader(t)
	th := deck.Thread{ID: "t-0003", Worktree: wt, Repo: main}
	for _, c := range []struct{ file, want string }{
		{`{"version": 1, "commands": {"dev": null}}`, "says the repo has no dev server (commands.dev is null)"},
		{`{"version": 1, "ports": {"web": 3000}, "services": {"web": {}}}`, `has no "dev" command and no service with a "run"`},
		{`{"version": 1, "ports": {"web": {"base": 3100}}, "commands": {"dev": "npm run dev -- --port $PORT_web"}}`, ""},
	} {
		write(t, filepath.Join(wt, manifest.Path), c.file)
		msg, err := r.Up(context.Background(), th)
		if c.want == "" {
			// A port from the store has no number yet: dev is not run.
			if err == nil || !strings.Contains(err.Error(), "port web: port store not supported yet") {
				t.Errorf("%s: %q, %v", c.file, msg, err)
			}
			continue
		}
		if err != nil || !strings.Contains(msg, c.want) {
			t.Errorf("%s: %q, %v; want %q", c.file, msg, err, c.want)
		}
	}
	if len(w.cmds) != 0 {
		t.Errorf("started %q", w.names())
	}

	if err := os.Remove(filepath.Join(wt, manifest.Path)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(main, manifest.Path)); err != nil {
		t.Fatal(err)
	}
	msg, err := r.Up(context.Background(), th)
	if err != nil || !strings.Contains(msg, "no .config/dev.json here") {
		t.Errorf("no manifest: %q, %v", msg, err)
	}
	write(t, filepath.Join(main, manifest.Path), `{"version": 1,`)
	if _, err := r.Up(context.Background(), th); err == nil || !strings.Contains(err.Error(), "webshop/.config/dev.json") {
		t.Errorf("broken manifest: %v", err)
	}
}

// A legacy manifest's up runs as commands.dev, with its variables.
func TestUpLegacy(t *testing.T) {
	main, wt := repo(t)
	if err := os.Remove(filepath.Join(main, manifest.Path)); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(main, manifest.LegacyPath), `{
	  "state": { "file": ".dev/$DIRNAME/state.json", "ports": { "api": "api_port" } },
	  "up": "scripts/dev-up --name $DIRNAME --branch ${BRANCH} --repo $REPO --dir $WORKTREE --cost $$5"
	}`)
	w := newWorld()
	r := w.reader(t)
	th := deck.Thread{ID: "t-0003", Worktree: wt, Repo: main, Branch: "abc-12-templates"}
	if _, err := r.Up(context.Background(), th); err != nil {
		t.Fatal(err)
	}
	want := "scripts/dev-up --name t-0003-templates --branch abc-12-templates --repo " + main + " --dir " + wt + " --cost $5"
	if len(w.cmds) != 1 || w.cmds[0].Shell != want || w.cmds[0].Name != "dev.default" {
		t.Errorf("commands %+v, want %q", w.cmds, want)
	}
}

func TestStop(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(wt, manifest.Path), `{"version": 1,
	  "ports": {"web": 3100, "api": 8100, "db": 5432},
	  "services": {"web": {"run": "a", "needs": "api"}, "api": {"run": "b", "needs": "db"}, "db": {"run": "c"}},
	  "commands": {"stop": "scripts/down --name $DIRNAME"}}`)
	w := newWorld()
	r := w.reader(t)
	st := r.store()
	key := Key(wt)
	// Records as some tool left them, in the wrong order on disk.
	for i, name := range []string{"service.db", "service.web", "dev.default", "service.api", "service.gone"} {
		pid := 100 + i
		w.alive[pid] = name != "service.gone"
		if err := st.writeRecord(key, Record{Version: 1, Worktree: wt, Name: name, PID: pid, Started: time.Now().UTC().Format(time.RFC3339)}); err != nil {
			t.Fatal(err)
		}
	}
	w.ignoreTerm[100] = true // db ignores SIGTERM
	stopWait = 50 * time.Millisecond
	pollEvery = 10 * time.Millisecond

	th := deck.Thread{ID: "t-0003", Worktree: wt, Repo: main}
	msg, err := r.Stop(context.Background(), th)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.runs) != 1 || w.runs[0].Shell != "scripts/down --name t-0003-templates" || w.runs[0].Name != "command.stop" || w.runs[0].Log != filepath.Join(r.State, "logs", key, "command.stop.log") {
		t.Errorf("stop command %+v", w.runs)
	}
	// The dev command first, then dependents before what they need; db
	// gets SIGKILL after the wait.
	want := []string{"102 TERM", "101 TERM", "103 TERM", "100 TERM", "100 KILL"}
	if !reflect.DeepEqual(w.signals, want) {
		t.Errorf("signals %q, want %q", w.signals, want)
	}
	if got := st.Records(key); len(got) != 0 {
		t.Errorf("records left: %+v", got)
	}
	if msg != `t-0003: ran "scripts/down --name t-0003-templates"; stopped dev.default, web, api, db` {
		t.Errorf("status %q", msg)
	}

	msg, _ = r.Stop(context.Background(), deck.Thread{ID: "t-0003", Worktree: wt, Repo: main})
	if len(w.signals) != 5 || !strings.HasPrefix(msg, `t-0003: ran "scripts/down`) {
		t.Errorf("second stop: %q, signals %q", msg, w.signals)
	}
}

// With a broken manifest the deck cannot run commands.stop, but still
// stops what the run records name.
func TestStopWithBrokenManifest(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(wt, manifest.Path), `{"version": 1,`)
	w := newWorld()
	r := w.reader(t)
	w.alive[100] = true
	if err := r.store().writeRecord(Key(wt), Record{Version: 1, Name: "service.web", PID: 100}); err != nil {
		t.Fatal(err)
	}
	msg, err := r.Stop(context.Background(), deck.Thread{ID: "t-0003", Worktree: wt, Repo: main})
	if err != nil || msg != "t-0003: stopped web" || len(w.runs) != 0 {
		t.Errorf("stop: %q, %v, runs %+v", msg, err, w.runs)
	}
	msg, _ = r.Stop(context.Background(), deck.Thread{ID: "t-0003", Worktree: wt, Repo: main})
	if !strings.HasPrefix(msg, "nothing to stop") {
		t.Errorf("nothing left: %q", msg)
	}
}

func TestKey(t *testing.T) {
	k := Key("/home/sam/src/acme-shop-worktrees/abc-123 cart")
	if !strings.HasPrefix(k, "abc-123_cart-") || len(k) != len("abc-123_cart-")+8 {
		t.Errorf("key %q", k)
	}
	if Key("/a/x") == Key("/b/x") {
		t.Error("two worktrees with one folder name share a key")
	}
}

func TestLock(t *testing.T) {
	st := Store{Dir: t.TempDir()}
	unlock, err := st.Lock()
	if err != nil {
		t.Fatal(err)
	}
	lockWait, lockRetry = 100*time.Millisecond, 10*time.Millisecond
	if _, err := st.Lock(); err == nil || !strings.Contains(err.Error(), "held by another tool") {
		t.Errorf("second lock: %v", err)
	}
	unlock()
	unlock2, err := st.Lock()
	if err != nil {
		t.Fatalf("after unlock: %v", err)
	}
	// Releasing an old lock does not remove a newer one.
	unlock()
	if _, err := os.Stat(filepath.Join(st.Dir, "lock")); err != nil {
		t.Errorf("a stale release removed the lock: %v", err)
	}
	unlock2()

	// A lock of a gone process on this host is stale and taken away.
	host, _ := os.Hostname()
	write(t, filepath.Join(st.Dir, "lock"), `{"pid": 999999999, "host": "`+host+`", "tool": "x", "time": "`+time.Now().UTC().Format(time.RFC3339)+`", "token": "t"}`)
	unlock3, err := st.Lock()
	if err != nil {
		t.Fatalf("stale lock: %v", err)
	}
	unlock3()
	// So is one older than 30 s, from any host.
	write(t, filepath.Join(st.Dir, "lock"), `{"pid": 1, "host": "elsewhere", "tool": "x", "time": "2020-01-01T00:00:00Z", "token": "t"}`)
	unlock4, err := st.Lock()
	if err != nil {
		t.Fatalf("old lock: %v", err)
	}
	unlock4()
	if left, _ := filepath.Glob(filepath.Join(st.Dir, "lock*")); len(left) != 0 {
		t.Errorf("left %q", left)
	}
}

func TestStopOrderKeepsUnknownLast(t *testing.T) {
	m, err := manifest.Parse([]byte(services))
	if err != nil {
		t.Fatal(err)
	}
	p := &plan{m: m}
	var recs []Record
	for _, n := range []string{"service.zzz", "service.db", "service.web", "dev.default", "service.cache"} {
		recs = append(recs, Record{Name: n})
	}
	var got []string
	for _, r := range (&Reader{}).stopOrder(p, recs) {
		got = append(got, r.Name)
	}
	if want := []string{"dev.default", "service.web", "service.cache", "service.db", "service.zzz"}; !slices.Equal(got, want) {
		t.Errorf("order %q, want %q", got, want)
	}
}

// Findings from review: a stop command that cannot be built still stops
// the recorded processes; env files may use ports; a gone worktree keeps
// its records and its dev command's state.
func TestStopWhenStopCommandCannotRun(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(wt, manifest.Path), `{"version": 1, "ports": {"db": {"base": 5400}},
	  "commands": {"stop": "down -p x-$PORT_db"}}`)
	w := newWorld()
	r := w.reader(t)
	w.alive[100] = true
	if err := r.store().writeRecord(Key(wt), Record{Version: 1, Name: "dev.default", PID: 100}); err != nil {
		t.Fatal(err)
	}
	msg, err := r.Stop(context.Background(), deck.Thread{ID: "t-0003", Worktree: wt, Repo: main})
	if err != nil || !strings.Contains(msg, "commands.stop not run: port db: port store not supported yet") || !strings.Contains(msg, "stopped dev.default") {
		t.Errorf("stop: %q, %v", msg, err)
	}
}

func TestEnvFilesUsePorts(t *testing.T) {
	main, wt := repo(t)
	write(t, filepath.Join(wt, manifest.Path), `{"version": 1, "env": {"files": [".env.$PORT_web"]}, "ports": {"web": 3000},
	  "links": [{"title": "$env(NAME|none)", "url": "http://localhost:$PORT_web"}]}`)
	write(t, filepath.Join(wt, ".env.3000"), "NAME=Shop\n")
	r := &Reader{Prober: Prober{Dial: fakeDial(new(atomic.Int32))}}
	snap := deck.Snapshot{Threads: []deck.Thread{{ID: "t-0003", Worktree: wt, Repo: main}}}
	r.Apply(context.Background(), &snap)
	if l := snap.Threads[0].Links; len(l) != 1 || l[0].Label != "Shop" {
		t.Errorf("links %+v", l)
	}
}

func TestGoneWorktreeKeepsDevUp(t *testing.T) {
	main, wt := repo(t)
	w := newWorld()
	r := w.reader(t)
	w.alive[100] = true
	if err := r.store().writeRecord(Key(wt), Record{Version: 1, Name: "dev.default", PID: 100, Log: "/x.log"}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(wt); err != nil {
		t.Fatal(err)
	}
	if got := resolved(wt); got != wt {
		t.Errorf("resolved gone path %q, want %q", got, wt)
	}
	snap := deck.Snapshot{Threads: []deck.Thread{{ID: "t-0003", Worktree: wt, Repo: main}}}
	r.Apply(context.Background(), &snap)
	th := snap.Threads[0]
	if th.DevNote != "worktree not found" || th.DevUp == nil || !th.DevUp.Alive {
		t.Errorf("thread note %q, DevUp %+v", th.DevNote, th.DevUp)
	}
	msg, _ := r.Stop(context.Background(), deck.Thread{ID: "t-0003", Worktree: wt, Repo: main})
	if msg != "t-0003: stopped dev.default" {
		t.Errorf("stop in a gone worktree: %q", msg)
	}
}

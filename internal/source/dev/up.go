package dev

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/dev/manifest"
)

// Command is something from the manifest, ready to run.
type Command struct {
	// Name says what runs, as run records and logs name it: service.api,
	// dev.default, command.stop.
	Name  string
	Shell string // run by /bin/sh -c; "" when Argv is set
	Argv  []string
	Dir   string
	Env   []string
	Log   string // stdout and stderr are appended here
}

// Text is the command line, for logs and messages.
func (c Command) Text() string {
	if c.Shell != "" {
		return c.Shell
	}
	return strings.Join(c.Argv, " ")
}

// Timing of stopping (spec section 8) and of waiting for readiness.
var (
	stopWait        = 10 * time.Second
	stopCommandWait = 60 * time.Second
	pollEvery       = 250 * time.Millisecond
)

// DefaultGroup is the group plain dev starts.
const DefaultGroup = "default"

// claim marks a worktree busy with a dev or stop; false when it already is.
func (r *Reader) claim(wt string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.busy[wt] {
		return false
	}
	if r.busy == nil {
		r.busy = map[string]bool{}
	}
	r.busy[wt] = true
	return true
}

func (r *Reader) release(wt string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.busy, wt)
}

// prepare reads a thread's manifest for Up and Stop. A non-empty message
// is the answer to show instead.
func (r *Reader) prepare(t deck.Thread) (*plan, string, error) {
	if t.Worktree == "" {
		return nil, "", errors.New("no worktree on this row")
	}
	p, err := newPlan(t)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Sprintf("no %s here: add one to start dev servers (see the README)", manifest.Path), nil
	case err != nil:
		return p, "", fmt.Errorf("%s: %w", p.found.Short(), err)
	}
	if r.State == "" {
		return nil, "", errors.New("no folder for run records: $HOME is not set")
	}
	return p, "", nil
}

// Up runs dev for a thread's worktree (spec section 8): the manifest's
// commands.dev once, detached, unless it still runs or the default group
// is ready; else each service of the default group with a run, plus what
// they need, in needs order, waiting for what a service needs to get ready
// before starting it. It returns the line the status bar shows.
func (r *Reader) Up(ctx context.Context, t deck.Thread) (string, error) {
	p, msg, err := r.prepare(t)
	if msg != "" || err != nil {
		return msg, err
	}
	if !r.claim(p.worktree) {
		return t.ID + "'s dev servers are being started or stopped already", nil
	}
	defer r.release(p.worktree)

	group := p.m.DefaultGroup()
	if dev, ok := p.m.Command("dev"); ok {
		if dev.Null {
			return fmt.Sprintf("%s says the repo has no dev server (commands.dev is null)", p.found.Short()), nil
		}
		return r.upCommand(ctx, t, p, dev.Run, group)
	}
	order := p.m.StartOrder(group)
	runnable := false
	for _, n := range order {
		runnable = runnable || p.m.Service(n).Run != nil
	}
	if !runnable {
		return fmt.Sprintf(`%s has no "dev" command and no service with a "run": add one (see the README)`, p.found.Short()), nil
	}

	st := r.store()
	var started, skipped, failed []string
	broken := map[string]string{} // service -> why it did not get ready
	for _, name := range order {
		s := p.m.Service(name)
		why := ""
		for _, need := range s.Needs {
			if w, bad := broken[need]; bad {
				why = need + " " + w
				break
			}
			if !r.waitReady(ctx, p, p.m.Service(need)) {
				broken[need] = fmt.Sprintf("is not ready after %s", p.m.Service(need).Ready.Timeout)
				if log := r.serviceLog(p, p.m.Service(need)); log != "" {
					broken[need] += " (log: " + tilde(log) + ")"
				}
				why = need + " " + broken[need]
				break
			}
		}
		if why != "" {
			broken[name] = "was not started"
			failed = append(failed, fmt.Sprintf("%s not started: %s", name, why))
			continue
		}
		if s.Run == nil {
			continue // started by something else; only waited for
		}
		if r.ready(ctx, p, s) {
			skipped = append(skipped, name+" (ready)")
			continue
		}
		cmd, err := r.serviceCommand(p, s)
		if err != nil {
			broken[name] = "was not started"
			failed = append(failed, fmt.Sprintf("%s not started: %v", name, err))
			continue
		}
		ok, err := r.startRecorded(p, cmd, st)
		switch {
		case err != nil:
			broken[name] = "was not started"
			failed = append(failed, fmt.Sprintf("%s: %v", name, err))
		case !ok:
			skipped = append(skipped, name+" (starting)")
		default:
			started = append(started, name)
		}
	}
	var parts []string
	if len(started) > 0 {
		parts = append(parts, "started "+strings.Join(started, ", ")+" for "+t.ID)
	}
	if len(skipped) > 0 {
		parts = append(parts, "skipped "+strings.Join(skipped, ", "))
	}
	parts = append(parts, failed...)
	if len(started) == 0 && len(failed) == 0 {
		parts = []string{t.ID + "'s dev servers already run: " + strings.Join(skipped, ", ")}
	}
	return strings.Join(parts, "; "), nil
}

// upCommand runs commands.dev for a group, unless the process it started
// still runs or the group is ready.
func (r *Reader) upCommand(ctx context.Context, t deck.Thread, p *plan, run *manifest.Run, group []string) (string, error) {
	st := r.store()
	name := "dev." + DefaultGroup
	if rec, ok := st.Record(p.key, name); ok && st.IsAlive(rec) {
		return fmt.Sprintf("%s's dev command still runs (pid %d); its log is in the drawer", t.ID, rec.PID), nil
	}
	if len(group) > 0 {
		all := true
		for _, n := range group {
			all = all && r.ready(ctx, p, p.m.Service(n))
		}
		if all {
			return fmt.Sprintf("%s's dev servers already answer (%s); not running %q", t.ID, strings.Join(group, ", "), run.Text()), nil
		}
	}
	cmd, err := r.command(p, name, nil, run, map[string]string{"DEV_GROUP": DefaultGroup})
	if err != nil {
		return "", fmt.Errorf("commands.dev: %w", err)
	}
	cmd.Log = st.Log(p.key, name)
	ok, err := r.startRecorded(p, cmd, st)
	switch {
	case err != nil:
		return "", err
	case !ok:
		return fmt.Sprintf("%s's dev command was just started by another tool", t.ID), nil
	}
	return fmt.Sprintf("started %q for %s; its log is in the drawer", cmd.Text(), t.ID), nil
}

// startRecorded starts a long-running command under the lock and writes
// its run record, unless a live record for it exists (then ok is false).
func (r *Reader) startRecorded(p *plan, cmd Command, st Store) (ok bool, err error) {
	unlock, err := st.Lock()
	if err != nil {
		return false, err
	}
	defer unlock()
	if rec, has := st.Record(p.key, cmd.Name); has && st.IsAlive(rec) {
		return false, nil
	}
	start := r.Start
	if start == nil {
		start = startDetached
	}
	began := time.Now()
	pid, err := start(cmd)
	if err != nil {
		return false, err
	}
	rec := Record{
		Version: 1, Worktree: p.worktree, Name: cmd.Name, PID: pid,
		Started: began.UTC().Format(time.RFC3339), Command: cmd.Text(),
		Dir: cmd.Dir, Log: cmd.Log, Tool: Tool,
	}
	if err := st.writeRecord(p.key, rec); err != nil {
		return true, fmt.Errorf("started %q as pid %d, but could not record it: %w", cmd.Text(), pid, err)
	}
	return true, nil
}

// ready checks a service now: its check, else (without ports) whether the
// process the deck's record names runs.
func (r *Reader) ready(ctx context.Context, p *plan, s *manifest.Service) bool {
	if c := p.readyCheck(s); c.Port > 0 {
		return r.Prober.Check(ctx, c)
	}
	if len(s.Ports) > 0 {
		return false // its port has no number yet
	}
	st := r.store()
	rec, ok := st.Record(p.key, "service."+s.Name)
	return ok && st.IsAlive(rec)
}

// waitReady waits up to the service's timeout for it to get ready, and
// gives up early when the process the deck started for it exits.
func (r *Reader) waitReady(ctx context.Context, p *plan, s *manifest.Service) bool {
	deadline := time.Now().Add(s.Ready.Timeout)
	st := r.store()
	for {
		if r.ready(ctx, p, s) {
			return true
		}
		if s.Run != nil {
			if rec, ok := st.Record(p.key, "service."+s.Name); ok && !st.IsAlive(rec) {
				return false
			}
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(pollEvery):
		}
	}
}

func (r *Reader) serviceLog(p *plan, s *manifest.Service) string {
	log, _ := p.logOf(r.store(), s)
	return log
}

// serviceCommand builds a service's run.
func (r *Reader) serviceCommand(p *plan, s *manifest.Service) (Command, error) {
	cmd, err := r.command(p, "service."+s.Name, s, s.Run, map[string]string{"DEV_SERVICE": s.Name})
	if err != nil {
		return cmd, err
	}
	cmd.Log, err = p.logOf(r.store(), s)
	return cmd, err
}

// command builds a Command: its folder, argv or shell line and its
// environment (spec section 9). A template with a port that has no number
// yet is an error naming the port.
func (r *Reader) command(p *plan, name string, s *manifest.Service, run *manifest.Run, inject map[string]string) (Command, error) {
	cmd := Command{Name: name, Dir: p.worktree}
	expand := func(v string) (string, error) {
		out, err := p.vars.Expand(v)
		var mp *manifest.MissingPortError
		if errors.As(err, &mp) {
			why := p.pending[mp.Port]
			if why == "" {
				why = "not known yet"
			}
			return "", fmt.Errorf("port %s: %s", mp.Port, why)
		}
		return out, err
	}
	dir := run.Dir
	if dir == "" && s != nil {
		dir = s.Dir
	}
	if dir != "" {
		d, err := expand(dir)
		if err != nil {
			return cmd, err
		}
		if !filepath.IsAbs(d) {
			d = filepath.Join(p.worktree, d)
		}
		cmd.Dir = d
	}

	env, path, err := r.environ(p, s, run, inject, expand)
	if err != nil {
		return cmd, err
	}
	cmd.Env = env

	if run.Shell != "" {
		line, err := expand(run.Shell)
		if err != nil {
			return cmd, err
		}
		cmd.Shell = line
		return cmd, nil
	}
	for _, a := range run.Argv {
		v, err := expand(a)
		if err != nil {
			return cmd, err
		}
		cmd.Argv = append(cmd.Argv, v)
	}
	switch arg0 := cmd.Argv[0]; {
	case strings.Contains(arg0, "/"):
		if !filepath.IsAbs(arg0) {
			cmd.Argv[0] = filepath.Join(cmd.Dir, arg0)
		}
	default:
		found, err := lookPath(arg0, path)
		if err != nil {
			return cmd, err
		}
		cmd.Argv[0] = found
	}
	return cmd, nil
}

// environ builds a command's environment (spec section 9.3), and returns
// its PATH too.
func (r *Reader) environ(p *plan, s *manifest.Service, run *manifest.Run, inject map[string]string, expand func(string) (string, error)) ([]string, string, error) {
	env := map[string]string{}
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		env[k] = v
	}
	// The deck's pane is not the command's.
	delete(env, "HERDR_PANE_ID")

	var dirs []string
	for _, d := range p.m.Path {
		v, err := expand(d)
		if err != nil {
			return nil, "", fmt.Errorf("path: %w", err)
		}
		if !filepath.IsAbs(v) {
			v = filepath.Join(p.worktree, v)
		}
		dirs = append(dirs, v)
	}
	if env["PATH"] != "" {
		dirs = append(dirs, env["PATH"])
	}
	env["PATH"] = strings.Join(dirs, string(os.PathListSeparator))

	if run.LoadEnv || (s != nil && s.LoadEnv) {
		for k, v := range p.vars.Env {
			env[k] = v
		}
	}
	set := func(m map[string]string, what string) error {
		for k, v := range m {
			x, err := expand(v)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", what, k, err)
			}
			env[k] = x
		}
		return nil
	}
	if err := set(p.m.EnvVars, "env.vars"); err != nil {
		return nil, "", err
	}
	if s != nil {
		if err := set(s.Env, "services."+s.Name+".env"); err != nil {
			return nil, "", err
		}
	}
	if err := set(run.Env, "env"); err != nil {
		return nil, "", err
	}

	env["DEV_WORKTREE"] = p.vars.Worktree
	env["DEV_REPO"] = p.vars.Repo
	env["DEV_DIRNAME"] = filepath.Base(p.vars.Worktree)
	env["DEV_BRANCH"] = p.vars.Branch
	env["DEV_TOOL"] = Tool
	delete(env, "PORT")
	for name, n := range p.vars.Ports {
		env["PORT_"+name] = strconv.Itoa(n)
	}
	switch {
	case s != nil && len(s.Ports) > 0:
		if n, ok := p.vars.Ports[s.Ports[0]]; ok {
			env["PORT"] = strconv.Itoa(n)
		}
	case s == nil && len(p.m.Ports) == 1:
		if n, ok := p.vars.Ports[p.m.Ports[0].Name]; ok {
			env["PORT"] = strconv.Itoa(n)
		}
	}
	for k, v := range inject {
		env[k] = v
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	slices.Sort(out)
	return out, env["PATH"], nil
}

// lookPath finds an argv[0] in a PATH list.
func lookPath(name, path string) (string, error) {
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			dir = "."
		}
		p := filepath.Join(dir, name)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s: not found in PATH", name)
}

// Stop stops what dev started in a thread's worktree (spec section 8):
// commands.stop first, if the manifest has one, waiting for it to end;
// then every process that still has a run record there, the dev command
// first and services before what they need, with SIGTERM to the process
// group and SIGKILL after stopWait. It returns the status bar's line.
func (r *Reader) Stop(ctx context.Context, t deck.Thread) (string, error) {
	p, msg, err := r.prepare(t)
	if msg != "" {
		return msg, nil
	}
	if err != nil {
		// A broken manifest can't say how to stop, but the run records
		// still say what runs.
		if p == nil || r.State == "" {
			return "", err
		}
		wt := resolved(t.Worktree)
		p = &plan{worktree: wt, key: Key(wt), m: &manifest.Manifest{}}
	}
	if !r.claim(p.worktree) {
		return t.ID + "'s dev servers are being started or stopped already", nil
	}
	defer r.release(p.worktree)

	st := r.store()
	var parts []string
	// In a worktree that is gone, nothing can run; the signals still go.
	if c, ok := p.m.Command("stop"); ok && !c.Null && c.Run != nil && exists(p.worktree) {
		// A stop command that cannot run does not keep the recorded
		// processes running.
		cmd, err := r.command(p, "command.stop", nil, c.Run, nil)
		if err != nil {
			parts = append(parts, fmt.Sprintf("commands.stop not run: %v", err))
		} else {
			cmd.Log = st.Log(p.key, cmd.Name)
			run := r.Run
			if run == nil {
				run = runLogged
			}
			cctx, cancel := context.WithTimeout(ctx, stopCommandWait)
			err = run(cctx, cmd)
			cancel()
			if err != nil {
				parts = append(parts, fmt.Sprintf("%q failed: %v (log: %s)", cmd.Text(), err, tilde(cmd.Log)))
			} else {
				parts = append(parts, fmt.Sprintf("ran %q", cmd.Text()))
			}
		}
	}

	var stopped []string
	for _, rec := range r.stopOrder(p, st.Records(p.key)) {
		if !st.IsAlive(rec) {
			r.forget(st, p.key, rec.Name)
			continue
		}
		if err := r.stopProcess(ctx, st, rec); err != nil {
			parts = append(parts, fmt.Sprintf("%s: %v", rec.Name, err))
			continue
		}
		r.forget(st, p.key, rec.Name)
		stopped = append(stopped, strings.TrimPrefix(rec.Name, "service."))
	}
	if len(stopped) > 0 {
		parts = append(parts, "stopped "+strings.Join(stopped, ", "))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("nothing to stop: no process the deck or another tool started runs in %s", t.ID), nil
	}
	return t.ID + ": " + strings.Join(parts, "; "), nil
}

// stopOrder sorts run records for stopping: dev commands first, then
// services, dependents before what they need, then anything else.
func (r *Reader) stopOrder(p *plan, recs []Record) []Record {
	rank := map[string]int{}
	var all []string
	for _, s := range p.m.Services {
		all = append(all, s.Name)
	}
	order := p.m.StartOrder(all)
	for i, n := range order {
		rank["service."+n] = i + 1 // started later, stopped earlier
	}
	weight := func(rec Record) int {
		switch {
		case strings.HasPrefix(rec.Name, "dev."):
			return 1 << 20
		case rank[rec.Name] > 0:
			return rank[rec.Name]
		}
		return 0
	}
	out := slices.Clone(recs)
	slices.SortStableFunc(out, func(a, b Record) int { return weight(b) - weight(a) })
	return out
}

// stopProcess signals a record's process group and waits for it to go.
func (r *Reader) stopProcess(ctx context.Context, st Store, rec Record) error {
	signal := r.Signal
	if signal == nil {
		signal = signalGroup
	}
	if err := signal(rec.PID, false); err != nil {
		return err
	}
	deadline := time.Now().Add(stopWait)
	for st.IsAlive(rec) {
		if time.Now().After(deadline) || ctx.Err() != nil {
			if err := signal(rec.PID, true); err != nil {
				return err
			}
			time.Sleep(pollEvery)
			if st.IsAlive(rec) {
				return fmt.Errorf("pid %d is still running after SIGKILL", rec.PID)
			}
			return nil
		}
		time.Sleep(pollEvery / 5)
	}
	return nil
}

// forget removes a run record under the lock.
func (r *Reader) forget(st Store, key, name string) {
	unlock, err := st.Lock()
	if err != nil {
		return
	}
	defer unlock()
	_ = st.removeRecord(key, name)
}

// tilde shortens a path under $HOME to ~/….
func tilde(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if rel, ok := strings.CutPrefix(path, home+string(os.PathSeparator)); ok {
		return "~/" + rel
	}
	return path
}

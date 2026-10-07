package main

import (
	"sort"
	"strings"
)

// Effect is a side-effect class. Icons are single-cell glyphs that render in a herdr pane.
type Effect uint16

const (
	EffFSRead Effect = 1 << iota
	EffFSWrite
	EffExec
	EffNet
	EffDB
	EffLock
	EffSleep
	EffEnv
	EffUnknown // a call outside the module the rule table doesn't know
)

var effectInfo = []struct {
	e     Effect
	icon  string
	word  string
	block bool // blocks the calling goroutine
}{
	{EffFSRead, "▤", "fs read", true},
	{EffFSWrite, "▦", "fs write", true},
	{EffExec, "⚙", "exec", true},
	{EffNet, "⇄", "network", true},
	{EffDB, "⛁", "db", true},
	{EffLock, "⧗", "lock", false}, // short critical sections: shown, not counted as blocking,
	{EffSleep, "◷", "sleep", true},
	{EffEnv, "$", "env", false},
	{EffUnknown, "?", "unknown", false},
}

func (e Effect) icons() string {
	var b strings.Builder
	for _, i := range effectInfo {
		if e&i.e != 0 {
			b.WriteString(i.icon)
		}
	}
	return b.String()
}

func (e Effect) words() string {
	var w []string
	for _, i := range effectInfo {
		if e&i.e != 0 {
			w = append(w, i.icon+" "+i.word)
		}
	}
	return strings.Join(w, ", ")
}

func (e Effect) blocking() Effect {
	var b Effect
	for _, i := range effectInfo {
		if i.block && e&i.e != 0 {
			b |= i.e
		}
	}
	return b
}

// rules map a fully qualified external name prefix to an effect. First match wins.
// This is the whole "side-effect classifier": a table, not a model.
var rules = []struct {
	prefix string
	eff    Effect
}{
	{"os.ReadFile", EffFSRead}, {"os.Open", EffFSRead}, {"os.Stat", EffFSRead}, {"os.Lstat", EffFSRead},
	{"os.ReadDir", EffFSRead}, {"os.File.Read", EffFSRead}, {"path/filepath.Walk", EffFSRead},
	{"path/filepath.Glob", EffFSRead}, {"path/filepath.EvalSymlinks", EffFSRead}, {"io.ReadAll", EffFSRead},
	{"bufio.Scanner.Scan", EffFSRead}, {"os.Getwd", EffFSRead}, {"os.Executable", EffFSRead},
	{"os.WriteFile", EffFSWrite}, {"os.Create", EffFSWrite}, {"os.Remove", EffFSWrite}, {"os.Rename", EffFSWrite},
	{"os.Mkdir", EffFSWrite}, {"os.OpenFile", EffFSWrite}, {"os.Chmod", EffFSWrite}, {"os.File.Write", EffFSWrite},
	{"os.File.Close", EffFSWrite}, {"os.File.Sync", EffFSWrite}, {"os.Symlink", EffFSWrite}, {"os.Link", EffFSWrite},
	{"os/exec.", EffExec}, {"os.StartProcess", EffExec}, {"os.FindProcess", EffExec}, {"os.Process.", EffExec},
	{"syscall.Kill", EffExec}, {"syscall.Exec", EffExec},
	{"net/http.", EffNet}, {"net.", EffNet},
	{"database/sql.", EffDB},
	{"sync.Mutex.Lock", EffLock}, {"sync.RWMutex.Lock", EffLock}, {"sync.RWMutex.RLock", EffLock},
	{"sync.WaitGroup.Wait", EffLock}, {"golang.org/x/sync/errgroup.Group.Wait", EffLock},
	{"time.Sleep", EffSleep},
	{"os.Getenv", EffEnv}, {"os.LookupEnv", EffEnv}, {"os.Setenv", EffEnv}, {"os.Environ", EffEnv},
	{"os.UserHomeDir", EffEnv}, {"os.Hostname", EffEnv}, {"os.UserConfigDir", EffEnv},
}

// pure packages: calls into these have no side effect worth showing.
var purePrefixes = []string{
	"strings.", "strconv.", "fmt.Sprint", "fmt.Errorf", "sort.", "slices.", "maps.", "bytes.", "unicode",
	"errors.", "math", "regexp.", "path.", "path/filepath.Join", "path/filepath.Base", "path/filepath.Dir",
	"path/filepath.Rel", "path/filepath.Clean", "path/filepath.Ext", "path/filepath.Abs", "path/filepath.IsAbs",
	"path/filepath.Split", "path/filepath.ToSlash", "path/filepath.FromSlash",
	"time.", "context.", "encoding/", "hash", "crypto/", "cmp.", "iter.", "sync.Once", "sync.Mutex.Unlock",
	"sync.RWMutex.RUnlock", "sync.RWMutex.Unlock", "sync.WaitGroup.Add", "sync.WaitGroup.Go", "sync.WaitGroup.Done", "sync/atomic.",
	"os.IsNotExist", "os.IsExist", "io/fs.", "bufio.NewScanner", "bufio.Scanner.Text", "bufio.Scanner.Buffer",
	"bufio.Scanner.Err", "os/signal.", "runtime/debug.", "runtime.", "fmt.Fprint", "fmt.Print", "flag.", "io.",
	"charm.land/", "github.com/charmbracelet/", "github.com/alecthomas/chroma", "github.com/BurntSushi/toml",
	"github.com/santhosh-tekuri/jsonschema", "github.com/fsnotify/", "github.com/mattn/go-runewidth",
	"github.com/rivo/uniseg", "net/url.", "html", "text/", "reflect.", "unsafe.", "log.", "os.Exit", "os.Getpid",
	"os.Getuid", "os.File.Name", "os.File.Fd", "os.FileInfo", "os.FileMode", "os.DirEntry", "os.ProcAttr",
	"os.File.Stat", ".error.Error",
}

func classify(ext string) Effect {
	for _, r := range rules {
		if strings.HasPrefix(ext, r.prefix) {
			return r.eff
		}
	}
	for _, p := range purePrefixes {
		if strings.HasPrefix(ext, p) {
			return 0
		}
	}
	return EffUnknown
}

// effects computes, per function, the side effects it can reach.
// sync: only through calls that stay on the caller's goroutine.
type effects struct {
	p      *Program
	memo   map[string]Effect
	onPath map[string]bool
	sync   bool
}

func newEffects(p *Program, sync bool) *effects {
	return &effects{p: p, memo: map[string]Effect{}, onPath: map[string]bool{}, sync: sync}
}

func (e *effects) of(id string) Effect {
	if v, ok := e.memo[id]; ok {
		return v
	}
	if e.onPath[id] {
		return 0 // a cycle: the outer frame accounts for it (prototype: no SCC fixpoint)
	}
	fn := e.p.Funcs[id]
	if fn == nil {
		return 0
	}
	e.onPath[id] = true
	var out Effect
	for _, s := range fn.Sites {
		out |= e.site(s)
	}
	delete(e.onPath, id)
	e.memo[id] = out
	return out
}

func (e *effects) site(s Site) Effect {
	if e.sync && s.Ctx&CtxAsync != 0 {
		return 0
	}
	switch s.Kind {
	case SiteExt:
		return classify(s.Ext)
	case SiteCall:
		var out Effect
		for _, c := range s.Callees {
			out |= e.of(c)
		}
		return out
	}
	return 0
}

// direct is a function's own effects, not through other module functions.
func direct(fn *Func, sync bool) Effect {
	var out Effect
	for _, s := range fn.Sites {
		if s.Kind == SiteExt && (!sync || s.Ctx&CtxAsync == 0) {
			out |= classify(s.Ext)
		}
	}
	return out
}

// unknownExts lists external calls the rule table can't classify: the only
// place a classifier model could add anything.
func unknownExts(p *Program, ids []string) []string {
	seen := map[string]bool{}
	for _, id := range ids {
		fn := p.Funcs[id]
		if fn == nil {
			continue
		}
		for _, s := range fn.Sites {
			if s.Kind == SiteExt && classify(s.Ext) == EffUnknown {
				seen[s.Ext] = true
			}
		}
	}
	var out []string
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

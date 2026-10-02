package launch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Browser opens web links, focusing a tab that already shows the same page
// instead of opening another one when it can. Tab reuse works on macOS with
// a Chrome-family browser (Chrome, Chromium, Brave, Edge, Vivaldi), Arc or
// Safari as the default browser, through AppleScript. Anything else, a
// browser that is not running, no tab for the page, a refused Automation
// permission or any error opens the link with URL instead.
//
// The scripts read tab URLs only to compare them in the deck; nothing is
// stored or sent anywhere. Every func is injectable so tests never drive a
// real browser.
type Browser struct {
	// Reuse turns tab reuse on; false always opens a new tab.
	Reuse bool
	// GOOS is the operating system; "" means runtime.GOOS.
	GOOS string
	// DefaultBrowser is the default browser's bundle id, lowercase, or ""
	// when it is unknown; nil means DefaultBrowserID.
	DefaultBrowser func() string
	// Script runs an AppleScript with arguments and returns its output;
	// nil means osascript.
	Script func(ctx context.Context, script string, args ...string) (string, error)
	// Open opens a URL without tab reuse; nil means URL.
	Open func(url string) error
}

// scriptTimeout bounds each AppleScript run. The first list may wait on the
// Automation prompt; past this the link opens in a new tab.
const scriptTimeout = 10 * time.Second

// OpenURL focuses a tab showing the same page as u, or opens u.
func (b Browser) OpenURL(u string) error {
	if b.focus(u) {
		return nil
	}
	if b.Open != nil {
		return b.Open(u)
	}
	return URL(u)
}

// focus reports whether it found and focused a tab for u.
func (b Browser) focus(u string) bool {
	goos := b.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if !b.Reuse || goos != "darwin" || !isWeb(u) {
		return false
	}
	def := b.DefaultBrowser
	if def == nil {
		def = DefaultBrowserID
	}
	d := dialectOf(def())
	if d == none {
		return false
	}
	run := b.Script
	if run == nil {
		run = osascript
	}
	// Each script gets its own deadline: a list that waited on the
	// Automation prompt must not leave the focus script too little time to
	// report, or the link would open again next to the tab it focused.
	timed := func(script string, args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
		defer cancel()
		return run(ctx, script, args...)
	}
	out, err := timed(listScript(d.id))
	if err != nil {
		return false // no permission, browser quit, …: a new tab will do
	}
	t, ok := pick(parseTabs(out), u)
	if !ok {
		return false
	}
	out, err = timed(d.focusScript(), strconv.Itoa(t.window), strconv.Itoa(t.tab), t.url)
	return err == nil && strings.TrimSpace(out) == "focused"
}

// isWeb reports whether u is an http or https URL; other schemes (figma://
// and the like) belong to apps, not browser tabs.
func isWeb(u string) bool {
	p, err := url.Parse(u)
	return err == nil && (p.Scheme == "http" || p.Scheme == "https") && p.Host != ""
}

// dialect is how one browser is scripted.
type dialect struct {
	id   string // bundle id, as in `tell application id "…"`
	kind int
}

const (
	chromeKind = iota + 1
	arcKind
	safariKind
)

var none = dialect{}

// Chrome-family browsers share Chrome's AppleScript dictionary.
var chromeFamily = map[string]bool{
	"com.google.chrome":        true,
	"com.google.chrome.beta":   true,
	"com.google.chrome.dev":    true,
	"com.google.chrome.canary": true,
	"org.chromium.chromium":    true,
	"com.brave.browser":        true,
	"com.microsoft.edgemac":    true,
	"com.vivaldi.vivaldi":      true,
}

// dialectOf is the scripting dialect of the browser with bundle id id, or
// none for a browser without tab scripting (Firefox, for one).
func dialectOf(id string) dialect {
	id = strings.ToLower(id)
	switch {
	case chromeFamily[id]:
		return dialect{id, chromeKind}
	case id == "company.thebrowser.browser":
		return dialect{id, arcKind}
	case id == "com.apple.safari" || id == "com.apple.safaritechnologypreview":
		return dialect{id, safariKind}
	}
	return none
}

// listScript prints one line per tab of every window: window index, tab
// index and URL, tab-separated. It prints nothing when the browser is not
// running, so it never starts one. Each window's URLs come in one Apple
// event, which stays fast with hundreds of tabs; a window without tabs
// (Safari's downloads, say) is skipped.
//
// The tab character is made before the tell block: inside it, "tab" is the
// browser's tab class.
func listScript(id string) string {
	return `on run argv
	if application id "` + id + `" is not running then return ""
	set sep to character id 9
	set out to {}
	tell application id "` + id + `"
		repeat with wi from 1 to count of windows
			try
				set urls to URL of every tab of window wi
				repeat with ti from 1 to count of urls
					set u to item ti of urls
					if u is not missing value then set end of out to (wi as text) & sep & (ti as text) & sep & (u as text)
				end repeat
			end try
		end repeat
	end tell
	set AppleScript's text item delimiters to linefeed
	return out as text
end run
`
}

// focusScript takes a window index, a tab index and the URL the list saw
// there. It checks that the tab still shows that URL, since tabs may have
// moved in between, then selects it, raises its window and brings the
// browser forward. It prints "focused", or "moved" when the tab is gone.
//
// Windows are named by index each time, never bound to a variable: Arc
// cannot resolve the window-by-id reference that binding produces (-1700).
func (d dialect) focusScript() string {
	var sel string
	switch d.kind {
	case chromeKind:
		sel = `set active tab index of window wi to ti
		try
			set minimized of window wi to false
		end try
		set index of window wi to 1`
	case safariKind:
		sel = `set current tab of window wi to tab ti of window wi
		try
			set miniaturized of window wi to false
		end try
		set index of window wi to 1`
	case arcKind:
		// Arc has no active tab index, and setting a window's index is an
		// error there; selecting the tab raises its window.
		sel = `tell tab ti of window wi to select`
	}
	return `on run argv
	set wi to (item 1 of argv) as integer
	set ti to (item 2 of argv) as integer
	set want to item 3 of argv
	tell application id "` + d.id + `"
		if (count of windows) < wi then return "moved"
		if (count of tabs of window wi) < ti then return "moved"
		set have to URL of tab ti of window wi
		if have is missing value then return "moved"
		considering case
			if (have as text) is not want then return "moved"
		end considering
		` + sel + `
		activate
	end tell
	return "focused"
end run
`
}

func osascript(ctx context.Context, script string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "osascript", append([]string{"-"}, args...)...)
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("osascript: %w", err)
	}
	return string(out), nil
}

// tabRef is one browser tab: 1-based window and tab indexes and its URL.
type tabRef struct {
	window, tab int
	url         string
}

func parseTabs(out string) []tabRef {
	var tabs []tabRef
	for _, line := range strings.Split(out, "\n") {
		f := strings.SplitN(strings.TrimRight(line, "\r"), "\t", 3)
		if len(f) != 3 {
			continue
		}
		w, err1 := strconv.Atoi(f[0])
		t, err2 := strconv.Atoi(f[1])
		if err1 != nil || err2 != nil || f[2] == "" {
			continue
		}
		tabs = append(tabs, tabRef{w, t, f[2]})
	}
	return tabs
}

// pick is the tab to focus for u: the first one showing exactly u (after
// normalize), else the first one showing the same page (SamePage).
func pick(tabs []tabRef, u string) (tabRef, bool) {
	want := normalize(u)
	for _, t := range tabs {
		if normalize(t.url) == want {
			return t, true
		}
	}
	key := pageKey(u)
	for _, t := range tabs {
		if pageKey(t.url) == key {
			return t, true
		}
	}
	return tabRef{}, false
}

// normalize is u with the scheme and host lowercased, a default port, the
// fragment and a trailing slash dropped. A URL that does not parse comes
// back as it is.
func normalize(u string) string {
	p, err := url.Parse(u)
	if err != nil || p.Host == "" {
		return u
	}
	p.Scheme = strings.ToLower(p.Scheme)
	p.Host = strings.ToLower(p.Host)
	if port := p.Port(); (p.Scheme == "http" && port == "80") || (p.Scheme == "https" && port == "443") {
		p.Host = p.Hostname()
	}
	p.Fragment, p.RawFragment = "", ""
	if len(p.Path) > 1 {
		p.Path = strings.TrimRight(p.Path, "/")
		p.RawPath = ""
	} else {
		p.Path = ""
	}
	return p.String()
}

var (
	linearIssue = regexp.MustCompile(`^/([^/]+)/issue/([A-Za-z][A-Za-z0-9]*-[0-9]+)(?:/|$)`)
	githubItem  = regexp.MustCompile(`^/([^/]+)/([^/]+)/(pull|issues)/([0-9]+)(?:/|$)`)
	figmaFile   = regexp.MustCompile(`^/(file|design|proto|board|slides|deck)/([A-Za-z0-9]+)(?:/[^/]*/branch/([A-Za-z0-9]+)|/branch/([A-Za-z0-9]+))?(?:/|$)`)
	notionID    = regexp.MustCompile(`([0-9a-f]{32})$`)
)

// pageKey is what makes two URLs the same page for tab reuse:
//
//   - Linear: the workspace and issue ID; the title slug after it may differ.
//   - GitHub: the repository and pull request (or issue) number, whichever
//     tab of it (files, commits, checks) is showing.
//   - Figma: the file key (or branch key), whatever frame or page is
//     selected; /file/ and /design/ are the same view, a prototype is not.
//   - Notion: the page ID, from ?p= when a page is open over a database.
//   - A dev server on localhost (or 127.0.0.1, [::1], *.localhost) with a
//     port: the origin, so a tab that has wandered within the app counts.
//   - Anything else: the whole URL, after normalize.
func pageKey(u string) string {
	p, err := url.Parse(u)
	if err != nil || p.Host == "" {
		return u
	}
	host := strings.TrimPrefix(strings.ToLower(p.Hostname()), "www.")
	switch {
	case host == "linear.app":
		if m := linearIssue.FindStringSubmatch(p.Path); m != nil {
			return "linear:" + strings.ToLower(m[1]) + "/" + strings.ToUpper(m[2])
		}
	case host == "github.com":
		if m := githubItem.FindStringSubmatch(p.Path); m != nil {
			return "github:" + strings.ToLower(m[1]+"/"+m[2]) + "/" + m[3] + "/" + m[4]
		}
	case host == "figma.com":
		if m := figmaFile.FindStringSubmatch(p.Path); m != nil {
			kind, key := m[1], m[2]
			if kind == "file" {
				kind = "design"
			}
			if b := m[3] + m[4]; b != "" {
				key = b
			}
			return "figma:" + kind + "/" + key
		}
	case host == "notion.so" || strings.HasSuffix(host, ".notion.site"):
		if id := notionPage(p); id != "" {
			return "notion:" + id
		}
	case isLoopback(host) && p.Port() != "":
		return "local:" + strings.ToLower(p.Scheme) + "://" + host + ":" + p.Port()
	}
	return normalize(u)
}

// notionPage is the 32-hex page ID of a Notion URL, or "".
func notionPage(p *url.URL) string {
	for _, s := range []string{p.Query().Get("p"), p.Path[strings.LastIndex(p.Path, "/")+1:]} {
		s = strings.ToLower(strings.ReplaceAll(s, "-", ""))
		if m := notionID.FindString(s); m != "" {
			return m
		}
	}
	return ""
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasSuffix(host, ".localhost")
}

// DefaultBrowserID is the bundle id, lowercase, of the app Launch Services
// opens https links with, read from its per-user handler list. A Mac where
// no browser was ever chosen has no entry and uses Safari. It returns ""
// when the list cannot be read.
func DefaultBrowserID() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	plist := filepath.Join(home, "Library", "Preferences", "com.apple.LaunchServices", "com.apple.launchservices.secure.plist")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := os.Stat(plist); errors.Is(err, fs.ErrNotExist) {
		return "com.apple.safari"
	}
	out, err := exec.CommandContext(ctx, "plutil", "-convert", "json", "-o", "-", plist).Output()
	if err != nil {
		return ""
	}
	return handlerFor(out)
}

// handlerFor reads the https (else http) handler's bundle id from Launch
// Services' handler list as JSON; with neither, Safari.
func handlerFor(plistJSON []byte) string {
	var ls struct {
		LSHandlers []struct {
			Scheme string `json:"LSHandlerURLScheme"`
			Role   string `json:"LSHandlerRoleAll"`
		}
	}
	if err := json.Unmarshal(plistJSON, &ls); err != nil {
		return ""
	}
	byScheme := map[string]string{}
	for _, h := range ls.LSHandlers {
		if h.Role != "" && h.Role != "-" {
			byScheme[strings.ToLower(h.Scheme)] = strings.ToLower(h.Role)
		}
	}
	for _, s := range []string{"https", "http"} {
		if id := byScheme[s]; id != "" {
			return id
		}
	}
	return "com.apple.safari"
}

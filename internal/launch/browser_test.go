package launch

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestPageKey(t *testing.T) {
	same := [][2]string{
		{"https://linear.app/acme/issue/ABC-123/fix-the-login", "https://linear.app/acme/issue/abc-123"},
		{"https://github.com/acme/webshop/pull/42", "https://github.com/Acme/WebShop/pull/42/files#diff-1"},
		{"https://github.com/acme/webshop/issues/7", "https://github.com/acme/webshop/issues/7/"},
		{"https://www.figma.com/design/AbC123xyz/Checkout?node-id=1-2", "https://figma.com/file/AbC123xyz/Checkout?node-id=9-9"},
		{"https://www.figma.com/design/AbC123xyz/Checkout/branch/BrK999/Checkout", "https://www.figma.com/design/AbC123xyz/x/branch/BrK999/y?node-id=4-4"},
		{"https://www.notion.so/acme/Launch-plan-0123456789abcdef0123456789abcdef", "https://acme.notion.site/0123456789abcdef0123456789abcdef?pvs=4"},
		{"https://www.notion.so/acme/db-ffffffffffffffffffffffffffffffff?p=01234567-89ab-cdef-0123-456789abcdef", "https://www.notion.so/0123456789abcdef0123456789abcdef"},
		{"http://localhost:3000/", "http://localhost:3000/admin/orders?page=2"},
		{"http://shop.localhost:5173", "http://shop.localhost:5173/cart"},
		{"https://example.com/a/", "HTTPS://Example.com:443/a#top"},
	}
	for _, p := range same {
		if a, b := pageKey(p[0]), pageKey(p[1]); a != b {
			t.Errorf("pageKey(%q) = %q, pageKey(%q) = %q; want the same page", p[0], a, p[1], b)
		}
	}
	different := [][2]string{
		{"https://linear.app/acme/issue/ABC-123", "https://linear.app/acme/issue/ABC-124"},
		{"https://linear.app/acme/issue/ABC-123", "https://linear.app/other/issue/ABC-123"},
		{"https://github.com/acme/webshop/pull/42", "https://github.com/acme/webshop/pull/43"},
		{"https://github.com/acme/webshop/pull/42", "https://github.com/acme/webshop/issues/42"},
		{"https://www.figma.com/design/AbC123xyz/x", "https://www.figma.com/proto/AbC123xyz/x"},
		{"https://www.figma.com/design/AbC123xyz/x", "https://www.figma.com/design/AbC123xyz/x/branch/BrK999/x"},
		{"http://localhost:3000", "http://localhost:3001"},
		{"http://localhost:3000", "https://localhost:3000"},
		{"https://example.com/a", "https://example.com/a?b=1"},
		{"https://github.com/acme/webshop", "https://github.com/acme/webshop/pulls"},
	}
	for _, p := range different {
		if a, b := pageKey(p[0]), pageKey(p[1]); a == b {
			t.Errorf("pageKey(%q) = pageKey(%q) = %q; want different pages", p[0], p[1], a)
		}
	}
}

func TestPickPrefersExactURL(t *testing.T) {
	tabs := parseTabs("1\t1\thttps://github.com/acme/webshop/pull/42/files\n" +
		"2\t3\thttps://github.com/acme/webshop/pull/42\r\n" +
		"garbage line\n\n")
	if len(tabs) != 2 {
		t.Fatalf("parseTabs = %v, want 2 tabs", tabs)
	}
	if got, ok := pick(tabs, "https://github.com/acme/webshop/pull/42/"); !ok || got.window != 2 || got.tab != 3 {
		t.Errorf("pick exact = %v %v, want window 2 tab 3", got, ok)
	}
	if got, ok := pick(tabs, "https://github.com/acme/webshop/pull/42/commits"); !ok || got.window != 1 || got.tab != 1 {
		t.Errorf("pick same page = %v %v, want window 1 tab 1", got, ok)
	}
	if _, ok := pick(tabs, "https://github.com/acme/webshop/pull/9"); ok {
		t.Error("pick found a tab for another PR")
	}
}

func TestDialectOf(t *testing.T) {
	for id, kind := range map[string]int{
		"com.google.Chrome":          chromeKind,
		"com.brave.browser":          chromeKind,
		"com.microsoft.edgemac":      chromeKind,
		"company.thebrowser.browser": arcKind,
		"com.apple.Safari":           safariKind,
		"org.mozilla.firefox":        0,
		"":                           0,
	} {
		if got := dialectOf(id).kind; got != kind {
			t.Errorf("dialectOf(%q).kind = %d, want %d", id, got, kind)
		}
	}
}

func TestHandlerFor(t *testing.T) {
	js := `{"LSHandlers":[
		{"LSHandlerContentType":"public.html","LSHandlerRoleAll":"com.apple.safari"},
		{"LSHandlerURLScheme":"http","LSHandlerRoleAll":"com.brave.browser"},
		{"LSHandlerURLScheme":"https","LSHandlerRoleAll":"Company.TheBrowser.Browser","LSHandlerPreferredVersions":{"LSHandlerRoleAll":"-"}}]}`
	if got := handlerFor([]byte(js)); got != "company.thebrowser.browser" {
		t.Errorf("handlerFor = %q, want the https handler", got)
	}
	if got := handlerFor([]byte(`{"LSHandlers":[]}`)); got != "com.apple.safari" {
		t.Errorf("handlerFor with no handler = %q, want Safari", got)
	}
	if got := handlerFor([]byte(`not json`)); got != "" {
		t.Errorf("handlerFor(bad) = %q, want \"\"", got)
	}
}

// fakeBrowser records what a Browser does, with a scripted osascript.
type fakeBrowser struct {
	tabs    string // what the list script prints
	listErr error
	focus   string // what the focus script prints
	scripts []string
	args    [][]string
	opened  []string
}

func (f *fakeBrowser) browser(id string) Browser {
	return Browser{
		Reuse:          true,
		GOOS:           "darwin",
		DefaultBrowser: func() string { return id },
		Script: func(_ context.Context, script string, args ...string) (string, error) {
			f.scripts = append(f.scripts, script)
			f.args = append(f.args, args)
			if len(args) == 0 {
				return f.tabs, f.listErr
			}
			return f.focus, nil
		},
		Open: func(u string) error { f.opened = append(f.opened, u); return nil },
	}
}

func TestBrowserFocusesExistingTab(t *testing.T) {
	f := &fakeBrowser{
		tabs:  "1\t1\thttps://example.com/\n1\t2\thttps://linear.app/acme/issue/ABC-123/old-title\n",
		focus: "focused\n",
	}
	if err := f.browser("com.google.chrome").OpenURL("https://linear.app/acme/issue/ABC-123/new-title"); err != nil {
		t.Fatal(err)
	}
	if f.opened != nil {
		t.Errorf("opened %q, want the existing tab focused", f.opened)
	}
	if want := []string{"1", "2", "https://linear.app/acme/issue/ABC-123/old-title"}; len(f.args) != 2 || !reflect.DeepEqual(f.args[1], want) {
		t.Errorf("focus args = %q, want %q", f.args, want)
	}
	if !strings.Contains(f.scripts[1], `tell application id "com.google.chrome"`) || !strings.Contains(f.scripts[1], "active tab index") {
		t.Errorf("focus script is not Chrome's:\n%s", f.scripts[1])
	}
}

func TestBrowserDialectScripts(t *testing.T) {
	for id, want := range map[string]string{
		"company.thebrowser.browser": "to select",
		"com.apple.safari":           "set current tab of window wi",
	} {
		f := &fakeBrowser{tabs: "1\t1\thttps://example.com\n", focus: "focused"}
		if err := f.browser(id).OpenURL("https://example.com"); err != nil || f.opened != nil {
			t.Fatalf("%s: err %v, opened %q", id, err, f.opened)
		}
		if !strings.Contains(f.scripts[1], want) {
			t.Errorf("%s focus script lacks %q:\n%s", id, want, f.scripts[1])
		}
	}
}

func TestBrowserFallsBackToOpen(t *testing.T) {
	const u = "https://example.com/page"
	tests := []struct {
		name    string
		f       fakeBrowser
		id      string
		mod     func(*Browser)
		scripts int
	}{
		{name: "no tab", f: fakeBrowser{tabs: "1\t1\thttps://example.com/other\n"}, id: "com.google.chrome", scripts: 1},
		{name: "browser not running", f: fakeBrowser{tabs: ""}, id: "com.apple.safari", scripts: 1},
		{name: "permission denied", f: fakeBrowser{listErr: errors.New("execution error: Not authorized to send Apple events to Google Chrome. (-1743)")}, id: "com.google.chrome", scripts: 1},
		{name: "tab moved", f: fakeBrowser{tabs: "1\t1\t" + u + "\n", focus: "moved"}, id: "com.google.chrome", scripts: 2},
		{name: "firefox", id: "org.mozilla.firefox"},
		{name: "unknown default browser", id: ""},
		{name: "reuse off", id: "com.google.chrome", mod: func(b *Browser) { b.Reuse = false }},
		{name: "linux", id: "com.google.chrome", mod: func(b *Browser) { b.GOOS = "linux" }},
	}
	for _, tt := range tests {
		f := tt.f
		b := f.browser(tt.id)
		if tt.mod != nil {
			tt.mod(&b)
		}
		if err := b.OpenURL(u); err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if !reflect.DeepEqual(f.opened, []string{u}) || len(f.scripts) != tt.scripts {
			t.Errorf("%s: opened %q after %d scripts, want %q opened after %d", tt.name, f.opened, len(f.scripts), u, tt.scripts)
		}
	}
}

func TestBrowserLeavesAppLinksAlone(t *testing.T) {
	f := &fakeBrowser{tabs: "1\t1\thttps://www.figma.com/design/AbC123xyz/x\n", focus: "focused"}
	u := "figma://design/AbC123xyz/x"
	if err := f.browser("com.google.chrome").OpenURL(u); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.opened, []string{u}) || f.scripts != nil {
		t.Errorf("a figma:// link ran %d scripts and opened %q; want it opened directly", len(f.scripts), f.opened)
	}
}

package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// everyOld sets every setting under its earlier flat key, with comments.
const everyOld = `# My deck.

# Bare IDs link here.
linear_workspace = "acme"   # the team
linear_status = false
linear_api_key_command = "op read op://Vault/Linear/key"
figma_desktop = true
refresh_interval = "30s"
projects_root = "/work/projects"
reuse_browser_tabs = false
update_check = false
auto_restart = false
diff_view = "tree"
future_key = 1

[editor]
command = "zed {path}"

[ui]
folded_lists = ["Later"]
`

const everyMigrated = `# My deck.

future_key = 1

[editor]
command = "zed {path}"

[ui]
folded_lists = ["Later"]
refresh_interval = "30s"

[projects]
root = "/work/projects"

[linear]
# Bare IDs link here.
workspace = "acme"   # the team
status = false
api_key_command = "op read op://Vault/Linear/key"

[figma]
desktop = true

[browser]
reuse_tabs = false

[updates]
check = false
auto_restart = false

[diff]
view = "tree"
`

func TestMigrate(t *testing.T) {
	got, moved, err := Migrate(everyOld)
	if err != nil {
		t.Fatal(err)
	}
	if got != everyMigrated {
		t.Errorf("migrated =\n%s\nwant\n%s", got, everyMigrated)
	}
	if len(moved) != 10 || moved[0] != "refresh_interval → ui.refresh_interval" {
		t.Errorf("moved = %q", moved)
	}
	// It reads the same, now without notes.
	before := resolveFile(t, everyOld, Flags{}, nil)
	after := resolveFile(t, got, Flags{}, nil)
	if len(before.Notes) != 10 || len(after.Notes) != 0 || len(after.Problems) != 1 { // future_key
		t.Errorf("notes %d → %d, problems after %q", len(before.Notes), len(after.Notes), after.Problems)
	}
	for _, sp := range Specs {
		b, a := before.Values[sp.Key], after.Values[sp.Key]
		b.Old = ""
		if !reflect.DeepEqual(b, a) {
			t.Errorf("%s: %+v before, %+v after", sp.Key, b, a)
		}
	}
	// Once is enough.
	if again, moved, err := Migrate(got); err != nil || again != got || moved != nil {
		t.Errorf("second Migrate = %v, %q", err, moved)
	}
}

func TestMigrateEdgeCases(t *testing.T) {
	tests := []struct {
		name, in, want string
		moved          []string
	}{
		{"shadowed old key is dropped", "linear_workspace = \"old\"\n\n[linear]\nworkspace = \"new\"\n",
			"\n[linear]\nworkspace = \"new\"\n", []string{"linear_workspace removed: linear.workspace is set, and wins"}},
		{"a bad value moves as written", "refresh_interval = \"soon\"\n",
			"[ui]\nrefresh_interval = \"soon\"\n", []string{"refresh_interval → ui.refresh_interval"}},
		{"multi-line value", "linear_api_key_command = '''\nop read x'''\n",
			"[linear]\napi_key_command = '''\nop read x'''\n", []string{"linear_api_key_command → linear.api_key_command"}},
		{"into dotted keys", "update_check = true\nupdates.auto_restart = false\n",
			"updates.auto_restart = false\nupdates.check = true\n", []string{"update_check → updates.check"}},
		{"nothing to do", "[linear]\nworkspace = \"acme\"\n", "[linear]\nworkspace = \"acme\"\n", nil},
		{"empty", "", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, moved, err := Migrate(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want || !reflect.DeepEqual(moved, tt.moved) {
				t.Errorf("Migrate =\n%q, %q\nwant\n%q, %q", got, moved, tt.want, tt.moved)
			}
		})
	}
	if _, _, err := Migrate("linear_workspace = \n"); err == nil || !strings.Contains(err.Error(), "does not parse") {
		t.Errorf("broken file: %v", err)
	}
	if _, _, err := Migrate("linear_workspace = \"a\"\nlinear = 3\n"); err == nil {
		t.Error("linear not a table: want an error, not a broken file")
	}
}

func TestMigrateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	write(t, path, everyOld)
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}

	// A dry run writes nothing.
	m, err := MigrateFile(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.Before != everyOld || m.After != everyMigrated || len(m.Moved) != 10 {
		t.Errorf("dry run = %+v", m)
	}
	if b, _ := os.ReadFile(path); string(b) != everyOld {
		t.Error("the dry run changed the file")
	}
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Error("the dry run left a .bak")
	}

	if _, err := MigrateFile(path, true); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != everyMigrated {
		t.Errorf("file = %s", b)
	}
	if b, _ := os.ReadFile(path + ".bak"); string(b) != everyOld {
		t.Errorf(".bak = %s", b)
	}
	for _, p := range []string{path, path + ".bak"} {
		if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
			t.Errorf("%s mode = %v, want 0600", filepath.Base(p), st.Mode().Perm())
		}
	}

	// Nothing left: no second .bak over the first.
	if err := os.Remove(path + ".bak"); err != nil {
		t.Fatal(err)
	}
	if m, err := MigrateFile(path, true); err != nil || len(m.Moved) != 0 {
		t.Errorf("second run = %+v, %v", m, err)
	}
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Error("a run with nothing to move wrote a .bak")
	}

	// A missing file has nothing to move.
	if m, err := MigrateFile(filepath.Join(dir, "none.toml"), true); err != nil || m.Before != "" || len(m.Moved) != 0 {
		t.Errorf("missing file = %+v, %v", m, err)
	}
}

func TestUnifiedDiff(t *testing.T) {
	a := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"
	b := "a\nB\nc\nd\ne\nf\ng\nh\ni\nj\nk\n"
	want := `--- x
+++ y
@@ -1,5 +1,5 @@
 a
-b
+B
 c
 d
 e
@@ -8,3 +8,4 @@
 h
 i
 j
+k
`
	if got := UnifiedDiff(a, b, "x", "y"); got != want {
		t.Errorf("diff =\n%s\nwant\n%s", got, want)
	}
	if got := UnifiedDiff(a, a, "x", "y"); got != "" {
		t.Errorf("same text: %q", got)
	}
	if got := UnifiedDiff("", "x\n", "x", "y"); got != "--- x\n+++ y\n@@ -0,0 +1 @@\n+x\n" {
		t.Errorf("from nothing: %q", got)
	}
}

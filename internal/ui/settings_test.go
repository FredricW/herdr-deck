package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FredricW/herdr-deck/internal/config"
)

// settingsEnv is a fake environment in a temp home, so no test reads or
// writes the real ~/.config.
type settingsEnv struct {
	vars    map[string]string
	path    string
	applied []config.Settings
}

func newSettingsEnv(t *testing.T, file string, vars map[string]string) *settingsEnv {
	t.Helper()
	home := t.TempDir()
	e := &settingsEnv{vars: map[string]string{"HOME": home, "XDG_CONFIG_HOME": filepath.Join(home, "cfg")}}
	for k, v := range vars {
		e.vars[k] = v
	}
	e.path = filepath.Join(home, "cfg", "herdr-deck", "config.toml")
	if file != "" {
		if err := os.MkdirAll(filepath.Dir(e.path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(e.path, []byte(file), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func (e *settingsEnv) hooks(fl config.Flags) *SettingsHooks {
	return &SettingsHooks{
		Resolve: func() (config.Settings, error) {
			return config.Resolve(fl, func(k string) string { return e.vars[k] }, func(string) (string, error) { return "/bin/hunk", nil })
		},
		Save:  config.Save,
		Apply: func(s config.Settings) { e.applied = append(e.applied, s) },
	}
}

func (e *settingsEnv) file(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(e.path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const settingsFile = `# My deck.
linear_workspace = "acme"   # the team
future_key = 1

[editor]
command = "zed {path}"
`

func settingsModel(t *testing.T, e *settingsEnv, fl config.Flags, w, h int) Model {
	t.Helper()
	m, _ := newModelWith(t, calm(), w, h, func(o *Options) { o.Settings = e.hooks(fl) })
	m, _ = press(m, keys("s")...)
	if m.mode != modeSettings || !m.set.loaded {
		t.Fatalf("s: mode %v, loaded %v, err %q", m.mode, m.set.loaded, m.set.err)
	}
	return m
}

// goldenSettings renders the page with the temp path shown as a fixed one.
func goldenSettings(t *testing.T, name string, m Model) {
	t.Helper()
	m.set.cfg.Path = "/home/ada/.config/herdr-deck/config.toml"
	for k, v := range m.set.cfg.Values {
		v.Text = strings.ReplaceAll(v.Text, m.set.cfg.Values[config.KeyProjectsRoot].Text, "/home/ada/.herdr-projects")
		m.set.cfg.Values[k] = v
	}
	golden(t, name, m)
}

func TestSettingsGolden(t *testing.T) {
	var noCheck config.Flags
	noCheck.Set(config.KeyUpdateCheck, "", "false")
	to := func(key string) []tea.Msg { return keys(strings.Repeat("j", indexOf(key))) }
	cases := []struct {
		name string
		vars map[string]string
		fl   config.Flags
		keys []tea.Msg
	}{
		{name: "settings"},
		// An earlier env name shows as it is.
		{name: "settings-override", vars: map[string]string{"HERDR_DECK_EDITOR": "nvim {path}", config.EnvLinearAPIKey: "lin_api_secret"},
			fl: noCheck, keys: to(config.KeyEditorCommand)},
		{name: "settings-renamed", keys: to(config.KeyLinearWorkspace)},
		{name: "settings-edit", keys: append(to(config.KeyEditorCommand), tea.KeyPressMsg{Code: tea.KeyEnter}, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: '{', Text: "{"}, tea.KeyPressMsg{Code: 'x', Text: "x"}, tea.KeyPressMsg{Code: '}', Text: "}"})},
		{name: "settings-bottom", keys: keys("jjjjjjjjjjjjjjjj")},
	}
	for _, c := range cases {
		for _, w := range []int{80, 60} {
			name := c.name + "-" + map[int]string{80: "80", 60: "60"}[w]
			t.Run(name, func(t *testing.T) {
				e := newSettingsEnv(t, settingsFile, c.vars)
				m := settingsModel(t, e, c.fl, w, 28)
				m, _ = press(m, c.keys...)
				goldenSettings(t, name, m)
				if strings.Contains(screen(m), "lin_api_secret") {
					t.Error("the API key shows on the page")
				}
			})
		}
	}
}

func TestSettingsToggleAndEditKeepFile(t *testing.T) {
	e := newSettingsEnv(t, settingsFile, nil)
	m := settingsModel(t, e, config.Flags{}, 80, 28)

	// linear.status is a switch: ↵ writes false into a new [linear].
	m = moveTo(t, m, config.KeyLinearStatus)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	want := settingsFile + "\n[linear]\nstatus = false\n"
	if got := e.file(t); got != want {
		t.Fatalf("file after toggle:\n%s\nwant\n%s", got, want)
	}
	if m.Status() != "saved linear.status = false" {
		t.Errorf("status = %q", m.Status())
	}
	if len(e.applied) != 1 || e.applied[0].LinearStatus {
		t.Errorf("applied = %+v", e.applied)
	}

	// ui.refresh_interval: edit to 30s; the deck's tick follows at once.
	m = moveTo(t, m, config.KeyRefreshInterval)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.set.editing || m.set.input.Value() != "5s" {
		t.Fatalf("editing %v, value %q", m.set.editing, m.set.input.Value())
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace})
	m, _ = press(m, keys("30s")...)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.opt.Tick != 30*time.Second {
		t.Errorf("tick = %v, want 30s", m.opt.Tick)
	}
	if !strings.Contains(e.file(t), "refresh_interval = \"30s\"\n") || !strings.HasPrefix(e.file(t), "# My deck.\n") {
		t.Errorf("file:\n%s", e.file(t))
	}

	// x removes it again, back to the default.
	m, _ = press(m, keys("x")...)
	if strings.Contains(e.file(t), "refresh_interval") || m.opt.Tick != config.DefaultRefreshInterval {
		t.Errorf("x: tick %v, file:\n%s", m.opt.Tick, e.file(t))
	}
	if !strings.Contains(m.Status(), "removed ui.refresh_interval from the file; now 5s") {
		t.Errorf("status = %q", m.Status())
	}
}

// Saving a setting the file holds under its earlier name moves it into
// its table, comment and all, and says so.
func TestSettingsSaveMovesOldKey(t *testing.T) {
	e := newSettingsEnv(t, settingsFile, nil)
	m := settingsModel(t, e, config.Flags{}, 80, 28)
	m = moveTo(t, m, config.KeyLinearWorkspace)
	if !strings.Contains(screen(m), "the file calls it linear_workspace") {
		t.Errorf("no rename note under the row:\n%s", screen(m))
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m.set.input.SetValue("globex")
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	want := "# My deck.\nfuture_key = 1\n\n[editor]\ncommand = \"zed {path}\"\n\n[linear]\nworkspace = \"globex\"   # the team\n"
	if got := e.file(t); got != want {
		t.Errorf("file =\n%s\nwant\n%s", got, want)
	}
	if m.Status() != "saved linear.workspace = globex, moved from linear_workspace" {
		t.Errorf("status = %q", m.Status())
	}
	if v := m.set.cfg.Values[config.KeyLinearWorkspace]; v.Old != "" || len(m.set.cfg.Notes) != 0 {
		t.Errorf("after the move: %+v, notes %q", v, m.set.cfg.Notes)
	}
}

func TestSettingsInvalidValueIsNotSaved(t *testing.T) {
	e := newSettingsEnv(t, settingsFile, nil)
	m := settingsModel(t, e, config.Flags{}, 80, 28)
	m = moveTo(t, m, config.KeyRefreshInterval)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter}, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace})
	m, _ = press(m, keys("1h")...)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.set.editing || !strings.Contains(m.set.invalid, "outside 1s–10m0s") {
		t.Errorf("editing %v, invalid %q", m.set.editing, m.set.invalid)
	}
	if !strings.Contains(screen(m), "✕ 1h0m0s is outside") {
		t.Errorf("no error on screen:\n%s", screen(m))
	}
	m, _ = press(m, esc)
	if m.set.editing || e.file(t) != settingsFile {
		t.Errorf("esc: editing %v, file changed:\n%s", m.set.editing, e.file(t))
	}
	// Digits and link keys do nothing on the page.
	m, _ = press(m, keys("1l")...)
	if m.mode != modeSettings {
		t.Errorf("mode = %v after 1 l", m.mode)
	}
}

func TestSettingsOverrideStillWins(t *testing.T) {
	e := newSettingsEnv(t, "", map[string]string{config.EnvReuseTabs: "false"})
	m := settingsModel(t, e, config.Flags{}, 80, 28)
	m = moveTo(t, m, config.KeyReuseTabs)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	// The effective value is false (env), so the toggle writes true.
	if got := e.file(t); got != "[browser]\nreuse_tabs = true\n" {
		t.Errorf("file = %q", got)
	}
	if !strings.Contains(m.Status(), "$"+config.EnvReuseTabs+" still wins") {
		t.Errorf("status = %q", m.Status())
	}
}

// The toggle flips the file's value, not the overriding env var's, so it
// can go both ways while the override stays.
func TestSettingsToggleUnderOverrideFlipsFileValue(t *testing.T) {
	e := newSettingsEnv(t, "[updates]\ncheck = true\n", map[string]string{config.EnvUpdateCheck: "true"})
	m := settingsModel(t, e, config.Flags{}, 80, 28)
	m = moveTo(t, m, config.KeyUpdateCheck)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := e.file(t); got != "[updates]\ncheck = false\n" {
		t.Fatalf("first toggle: file = %q", got)
	}
	_, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := e.file(t); got != "[updates]\ncheck = true\n" {
		t.Errorf("second toggle: file = %q", got)
	}
	// Editing starts from the file's value too.
	e = newSettingsEnv(t, "[editor]\ncommand = \"zed {path}\"\n", map[string]string{config.EnvEditorCommand: "nvim {path}"})
	m = settingsModel(t, e, config.Flags{}, 80, 28)
	m = moveTo(t, m, config.KeyEditorCommand)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := m.set.input.Value(); got != "zed {path}" {
		t.Errorf("edit starts from %q, want the file's value", got)
	}
}

// While a save runs, the page takes no other change: two quick presses
// would both start from the old value.
func TestSettingsOneSaveAtATime(t *testing.T) {
	e := newSettingsEnv(t, "", nil)
	m := settingsModel(t, e, config.Flags{}, 80, 28)
	m = moveTo(t, m, config.KeyLinearStatus)
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil || !m.set.saving {
		t.Fatal("no save started")
	}
	next, again := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	if again != nil {
		t.Error("a second save started while the first ran")
	}
	m = run(m, cmd)
	if m.set.saving || e.file(t) != "[linear]\nstatus = false\n" {
		t.Errorf("saving %v, file %q", m.set.saving, e.file(t))
	}
}

func TestSettingsRestartNeeded(t *testing.T) {
	e := newSettingsEnv(t, "", nil)
	m := settingsModel(t, e, config.Flags{}, 80, 28)
	m = moveTo(t, m, config.KeyProjectsRoot)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m.set.input.SetValue("/srv/projects")
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.HasSuffix(m.Status(), "restart the deck to use it") {
		t.Errorf("status = %q", m.Status())
	}
}

func TestSettingsUpdateCheckOff(t *testing.T) {
	e := newSettingsEnv(t, "", nil)
	m, _ := newModelWith(t, calm(), 80, 28, func(o *Options) { o.Settings = e.hooks(config.Flags{}) })
	// Through Update, not press: the hourly tick it returns would block.
	next, _ := m.Update(updateMsg{Available: "v1.2.0"})
	m, _ = press(next.(Model), keys("s")...)
	m = moveTo(t, m, config.KeyUpdateCheck)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.update.Available != "" || strings.Contains(screen(m), "↑ v1.2.0") {
		t.Errorf("hint stays after updates.check = false: %+v", m.update)
	}
}

func TestSettingsClickSelects(t *testing.T) {
	e := newSettingsEnv(t, "", nil)
	m := settingsModel(t, e, config.Flags{}, 80, 28)
	x, y := find(t, m, "browser.reuse_tabs")
	m, _ = press(m, click(x, y))
	if m.set.cursor != indexOf(config.KeyReuseTabs) {
		t.Errorf("cursor = %d", m.set.cursor)
	}
	m, _ = press(m, esc)
	if m.mode != modeRow {
		t.Errorf("esc: mode %v", m.mode)
	}
}

func TestSettingsOff(t *testing.T) {
	m, _ := newModel(t, calm(), 80, 28)
	m, _ = press(m, keys("s")...)
	if m.mode != modeRow || m.Status() != "the settings page is off" {
		t.Errorf("mode %v, status %q", m.mode, m.Status())
	}
}

// moveTo moves the settings page's cursor to key with j or k.
func moveTo(t *testing.T, m Model, key string) Model {
	t.Helper()
	want := indexOf(key)
	for i := 0; m.set.cursor != want && i < len(config.Specs); i++ {
		k := "j"
		if m.set.cursor > want {
			k = "k"
		}
		m, _ = press(m, keys(k)...)
	}
	if m.set.cursor != want {
		t.Fatalf("cursor %d, want %s at %d", m.set.cursor, key, want)
	}
	return m
}

func indexOf(key string) int {
	for i, sp := range config.Specs {
		if sp.Key == key {
			return i
		}
	}
	return -1
}

// ↵ cycles diff.view through its values, and the saved view applies at
// once; d t alone never writes the file.
func TestSettingsCyclesDiffView(t *testing.T) {
	e := newSettingsEnv(t, "", nil)
	m := settingsModel(t, e, config.Flags{}, 80, 28)
	m = moveTo(t, m, config.KeyDiffView)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := e.file(t); got != "[diff]\nview = \"tree\"\n" || !m.tree {
		t.Fatalf("first press: tree %v, file %q", m.tree, got)
	}
	if m.Status() != "saved diff.view = tree" {
		t.Errorf("status = %q", m.Status())
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := e.file(t); got != "[diff]\nview = \"list\"\n" || m.tree {
		t.Errorf("second press: tree %v, file %q", m.tree, got)
	}
}

// ui.folded_lists is edited as comma-separated text and applies at once: lists
// the user has not folded or unfolded by hand follow it, and manual folds
// stay.
func TestSettingsEditsFoldedLists(t *testing.T) {
	e := newSettingsEnv(t, "", nil)
	m, _ := newModelWith(t, calm(), 80, 40, func(o *Options) { o.Settings = e.hooks(config.Flags{}) })
	// Unfold Backlog by hand.
	m, _ = press(m, keys("jjjjjj ")...)
	if !strings.Contains(screen(m), "Settings page") {
		t.Fatalf("space did not unfold Backlog:\n%s", screen(m))
	}
	m, _ = press(m, keys("s")...)
	m = moveTo(t, m, config.KeyFoldedLists)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := m.set.input.Value(); got != "Backlog, Resolved" {
		t.Fatalf("edit starts from %q", got)
	}
	m.set.input.SetValue("in progress, Backlog,")
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.set.editing || m.set.invalid != "a list name is empty" {
		t.Fatalf("editing %v, invalid %q", m.set.editing, m.set.invalid)
	}
	m.set.input.SetValue("in progress, Backlog")
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := e.file(t); got != "[ui]\nfolded_lists = [\"in progress\", \"Backlog\"]\n" {
		t.Fatalf("file = %q", got)
	}
	if m.Status() != "saved ui.folded_lists = in progress, Backlog" {
		t.Errorf("status = %q", m.Status())
	}
	m, _ = press(m, esc)
	if s := screen(m); !strings.Contains(s, "+ In progress (3)") || !strings.Contains(s, "Settings page") {
		t.Errorf("In progress should fold and the hand-unfolded Backlog stay open:\n%s", s)
	}

	// none folds nothing.
	m, _ = press(m, keys("s")...)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m.set.input.SetValue("none")
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := e.file(t); got != "[ui]\nfolded_lists = []\n" {
		t.Fatalf("file = %q", got)
	}
	m, _ = press(m, esc)
	if s := screen(m); strings.Contains(s, "+ In progress") || !strings.Contains(s, "Settings page") {
		t.Errorf("none should unfold In progress:\n%s", s)
	}
}

// A list name the comma-separated text cannot hold is never rewritten from
// the page.
func TestSettingsFoldedListsHandEdit(t *testing.T) {
	for _, file := range []string{"[ui]\nfolded_lists = [\"Ideas, later\"]\n", "[ui]\nfolded_lists = [\"None\"]\n"} {
		e := newSettingsEnv(t, file, nil)
		m := settingsModel(t, e, config.Flags{}, 80, 28)
		m = moveTo(t, m, config.KeyFoldedLists)
		m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.set.editing || !strings.HasSuffix(m.Status(), "edit the file") || e.file(t) != file {
			t.Errorf("%q: editing %v, status %q, file %q", file, m.set.editing, m.Status(), e.file(t))
		}
	}
}

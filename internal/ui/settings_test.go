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
	no := false
	cases := []struct {
		name string
		vars map[string]string
		fl   config.Flags
		keys []tea.Msg
	}{
		{name: "settings"},
		{name: "settings-override", vars: map[string]string{config.EnvEditor: "nvim {path}", config.EnvLinearAPIKey: "lin_api_secret"},
			fl: config.Flags{UpdateCheck: &no}, keys: keys("jjj")},
		{name: "settings-edit", keys: append(keys("jjj"), tea.KeyPressMsg{Code: tea.KeyEnter}, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: '{', Text: "{"}, tea.KeyPressMsg{Code: 'x', Text: "x"}, tea.KeyPressMsg{Code: '}', Text: "}"})},
		{name: "settings-bottom", keys: keys("jjjjjjjjjjjjjjj")},
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

	// linear_status (row 2) is a switch: ↵ writes false.
	m, _ = press(m, keys("j")...)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	want := strings.Replace(settingsFile, "future_key = 1\n", "future_key = 1\nlinear_status = false\n", 1)
	if got := e.file(t); got != want {
		t.Fatalf("file after toggle:\n%s\nwant\n%s", got, want)
	}
	if !strings.Contains(m.Status(), "saved linear_status = false") {
		t.Errorf("status = %q", m.Status())
	}
	if len(e.applied) != 1 || e.applied[0].LinearStatus {
		t.Errorf("applied = %+v", e.applied)
	}

	// refresh_interval: edit to 30s; the deck's tick follows at once.
	for m.set.cursor != indexOf(config.KeyRefreshInterval) {
		m, _ = press(m, keys("j")...)
	}
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
	if !strings.Contains(m.Status(), "removed refresh_interval from the file; now 5s") {
		t.Errorf("status = %q", m.Status())
	}
}

func TestSettingsInvalidValueIsNotSaved(t *testing.T) {
	e := newSettingsEnv(t, settingsFile, nil)
	m := settingsModel(t, e, config.Flags{}, 80, 28)
	for m.set.cursor != indexOf(config.KeyRefreshInterval) {
		m, _ = press(m, keys("j")...)
	}
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
	for m.set.cursor != indexOf(config.KeyReuseTabs) {
		m, _ = press(m, keys("j")...)
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	// The effective value is false (env), so the toggle writes true.
	if got := e.file(t); got != "reuse_browser_tabs = true\n" {
		t.Errorf("file = %q", got)
	}
	if !strings.Contains(m.Status(), "$"+config.EnvReuseTabs+" still wins") {
		t.Errorf("status = %q", m.Status())
	}
}

// The toggle flips the file's value, not the overriding env var's, so it
// can go both ways while the override stays.
func TestSettingsToggleUnderOverrideFlipsFileValue(t *testing.T) {
	e := newSettingsEnv(t, "update_check = true\n", map[string]string{config.EnvUpdateCheck: "true"})
	m := settingsModel(t, e, config.Flags{}, 80, 28)
	for m.set.cursor != indexOf(config.KeyUpdateCheck) {
		m, _ = press(m, keys("j")...)
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := e.file(t); got != "update_check = false\n" {
		t.Fatalf("first toggle: file = %q", got)
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := e.file(t); got != "update_check = true\n" {
		t.Errorf("second toggle: file = %q", got)
	}
	// Editing starts from the file's value too.
	e = newSettingsEnv(t, "[editor]\ncommand = \"zed {path}\"\n", map[string]string{config.EnvEditor: "nvim {path}"})
	m = settingsModel(t, e, config.Flags{}, 80, 28)
	for m.set.cursor != indexOf(config.KeyEditorCommand) {
		m, _ = press(m, keys("j")...)
	}
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
	m, _ = press(m, keys("j")...) // linear_status
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
	if m.set.saving || e.file(t) != "linear_status = false\n" {
		t.Errorf("saving %v, file %q", m.set.saving, e.file(t))
	}
}

func TestSettingsRestartNeeded(t *testing.T) {
	e := newSettingsEnv(t, "", nil)
	m := settingsModel(t, e, config.Flags{}, 80, 28)
	for m.set.cursor != indexOf(config.KeyProjectsRoot) {
		m, _ = press(m, keys("j")...)
	}
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
	for m.set.cursor != indexOf(config.KeyUpdateCheck) {
		m, _ = press(m, keys("j")...)
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.update.Available != "" || strings.Contains(screen(m), "↑ v1.2.0") {
		t.Errorf("hint stays after update_check = false: %+v", m.update)
	}
}

func TestSettingsClickSelects(t *testing.T) {
	e := newSettingsEnv(t, "", nil)
	m := settingsModel(t, e, config.Flags{}, 80, 28)
	x, y := find(t, m, "update_check")
	m, _ = press(m, click(x, y))
	if m.set.cursor != indexOf(config.KeyUpdateCheck) {
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

func indexOf(key string) int {
	for i, sp := range config.Specs {
		if sp.Key == key {
			return i
		}
	}
	return -1
}

// ↵ cycles diff_view through its values, and the saved view applies at
// once; d t alone never writes the file.
func TestSettingsCyclesDiffView(t *testing.T) {
	e := newSettingsEnv(t, "", nil)
	m := settingsModel(t, e, config.Flags{}, 80, 28)
	for m.set.cursor != indexOf(config.KeyDiffView) {
		m, _ = press(m, keys("j")...)
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := e.file(t); got != "diff_view = \"tree\"\n" || !m.tree {
		t.Fatalf("first press: tree %v, file %q", m.tree, got)
	}
	if m.Status() != "saved diff_view = tree" {
		t.Errorf("status = %q", m.Status())
	}
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := e.file(t); got != "diff_view = \"list\"\n" || m.tree {
		t.Errorf("second press: tree %v, file %q", m.tree, got)
	}
}

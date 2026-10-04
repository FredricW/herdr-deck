package ui

import (
	"context"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/config"
)

// SettingsHooks back the settings page (s), which shows every setting with
// its effective value and source and edits the config file.
type SettingsHooks struct {
	// Resolve settles every setting again: the config file under the
	// deck's own flags and environment.
	Resolve func() (config.Settings, error)
	// Save sets key in the config file at path, or removes it when value
	// is nil (config.Save). Tests point it at a temp folder.
	Save func(path, key string, value *string) error
	// Apply hands the deck the settings after a save, for what the UI
	// does not own: how links and the editor open, Linear. Nil is fine.
	Apply func(config.Settings)
}

// settingsPage is the settings page's state.
type settingsPage struct {
	cfg     config.Settings
	loaded  bool
	err     string // why the settings could not be read
	cursor  int    // index into config.Specs
	editing bool
	saving  bool // a save runs; the page takes no other change meanwhile
	input   textinput.Model
	invalid string // why the input does not validate
}

type (
	settingsMsg struct {
		cfg config.Settings
		err error
	}
	settingsSavedMsg struct {
		key     string
		removed bool
		before  config.Value
		cfg     config.Settings
		err     error
	}
)

// openSettings shows the page and reads the settings afresh.
func (m *Model) openSettings() tea.Cmd {
	hooks := m.opt.Settings
	if hooks == nil || hooks.Resolve == nil {
		m.status = "the settings page is off"
		return nil
	}
	if m.mode == modeSettings {
		m.set.editing = false
		m.setMode(modeRow)
		return nil
	}
	m.choosing, m.dfocus = noKind, false
	m.set.editing = false
	m.setMode(modeSettings)
	return func() tea.Msg {
		cfg, err := hooks.Resolve()
		return settingsMsg{cfg: cfg, err: err}
	}
}

// settingsKey handles a key on the settings page. done is false when the
// key is left to the deck's own keys (quit, help, scrolling).
func (m *Model) settingsKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if m.set.editing {
		switch msg.String() {
		case "esc":
			m.set.editing = false
			return nil, true
		case "enter":
			v := strings.TrimSpace(m.set.input.Value())
			sp := config.Specs[m.set.cursor]
			if v == "" {
				m.set.editing = false
				return m.saveSetting(sp.Key, nil), true
			}
			if err := config.Check(sp.Key, v); err != nil {
				m.set.invalid = err.Error()
				m.followSetting() // the error shows under the value
				return nil, true
			}
			m.set.editing = false
			return m.saveSetting(sp.Key, &v), true
		}
		var cmd tea.Cmd
		m.set.input, cmd = m.set.input.Update(msg)
		m.set.invalid = ""
		if v := strings.TrimSpace(m.set.input.Value()); v != "" {
			if err := config.Check(config.Specs[m.set.cursor].Key, v); err != nil {
				m.set.invalid = err.Error()
			}
		}
		m.followSetting()
		return cmd, true
	}
	switch {
	case key.Matches(msg, m.keys.Down):
		m.moveSetting(1)
	case key.Matches(msg, m.keys.Up):
		m.moveSetting(-1)
	case msg.String() == "enter" || msg.String() == "space":
		return m.editSetting(), true
	case msg.String() == "x":
		if !m.set.loaded || m.set.saving {
			return nil, true
		}
		return m.saveSetting(config.Specs[m.set.cursor].Key, nil), true
	case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Settings):
		m.setMode(modeRow)
	case key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Help),
		key.Matches(msg, m.keys.Sources), key.Matches(msg, m.keys.PageDown),
		key.Matches(msg, m.keys.PageUp):
		return nil, false
	}
	return nil, true
}

func (m *Model) moveSetting(delta int) {
	m.set.cursor = clamp(m.set.cursor+delta, 0, len(config.Specs)-1)
	m.followSetting()
}

// followSetting scrolls the page so the selected setting and its notes
// show.
func (m *Model) followSetting() {
	l := m.layout()
	if l.drawer == nil || l.drawer.focus < 0 || l.drawerH <= 0 {
		return
	}
	top, end := l.drawer.focus, l.drawer.focusEnd
	if top > 0 && l.drawer.lines[top-1].text != "" {
		top-- // the group's heading
	}
	if end >= m.drawerOff+l.drawerH {
		m.drawerOff = end - l.drawerH + 1
	}
	if top < m.drawerOff {
		m.drawerOff = top
	}
	m.scrollDrawer(0)
}

// editSetting toggles a switch, cycles a choice or starts editing a value
// in place.
func (m *Model) editSetting() tea.Cmd {
	if !m.set.loaded || m.set.saving {
		return nil
	}
	sp := config.Specs[m.set.cursor]
	v := m.set.cfg.Values[sp.Key]
	// The file's own value is what changes, even when a flag or env var
	// hides it.
	cur := v.Text
	if v.InFile {
		cur = v.File
	}
	switch sp.Kind {
	case config.KindBool:
		next := "true"
		if cur == "true" {
			next = "false"
		}
		return m.saveSetting(sp.Key, &next)
	case config.KindChoice:
		next := sp.Choices[0]
		for i, c := range sp.Choices {
			if c == cur {
				next = sp.Choices[(i+1)%len(sp.Choices)]
			}
		}
		return m.saveSetting(sp.Key, &next)
	}
	in := textinput.New()
	in.Prompt = ""
	in.CharLimit = 1024
	in.SetValue(cur)
	in.CursorEnd()
	m.set.input = in
	m.set.editing = true
	m.set.invalid = ""
	return m.set.input.Focus()
}

// saveSetting writes key (nil removes it) and settles the settings again.
func (m *Model) saveSetting(key string, value *string) tea.Cmd {
	hooks := m.opt.Settings
	if hooks == nil || hooks.Save == nil {
		m.status = "saving settings is off"
		return nil
	}
	path := m.set.cfg.Path
	before := m.set.cfg.Values[key]
	m.set.saving = true
	return func() tea.Msg {
		msg := settingsSavedMsg{key: key, removed: value == nil, before: before}
		if msg.err = hooks.Save(path, key, value); msg.err != nil {
			return msg
		}
		msg.cfg, msg.err = hooks.Resolve()
		return msg
	}
}

// settingsSaved applies new settings and says what came of the save.
func (m *Model) settingsSaved(msg settingsSavedMsg) tea.Cmd {
	m.set.saving = false
	if msg.err != nil {
		m.status = "could not save " + msg.key + ": " + msg.err.Error()
		return nil
	}
	cmd := m.applySettings(msg.cfg)
	if msg.key == config.KeyDiffView {
		// A saved view applies now; d t alone never writes the file.
		m.tree = msg.cfg.DiffView == config.DiffViewTree
	}
	sp, _ := config.SpecFor(msg.key)
	v := msg.cfg.Values[msg.key]
	what := "saved " + msg.key + " = " + valueText(v, sp)
	if msg.removed {
		what = "removed " + msg.key + " from the file; now " + valueText(v, sp)
	}
	switch {
	case sp.Override(v.Source) != "":
		m.status = what + "; " + sp.Override(v.Source) + " still wins"
	case sp.Restart && v.Text != msg.before.Text:
		m.status = what + "; restart the deck to use it"
	default:
		m.status = what
	}
	return cmd
}

// applySettings takes new settings into the running deck: what the UI owns
// here, the rest through SettingsHooks.Apply.
func (m *Model) applySettings(cfg config.Settings) tea.Cmd {
	was := m.set.cfg
	m.set.cfg, m.set.loaded, m.set.err = cfg, true, ""
	m.opt.Tick = cfg.RefreshInterval
	m.opt.FigmaDesktop = cfg.FigmaDesktop
	if cfg.FoldedLists != nil && !slices.Equal(cfg.FoldedLists, m.opt.FoldedLists) {
		// Lists the user folded or unfolded by hand keep that.
		m.opt.FoldedLists = cfg.FoldedLists
		m.SetSnapshot(m.snap)
	}
	if h := m.opt.Settings; h != nil && h.Apply != nil {
		h.Apply(cfg)
	}
	var cmds []tea.Cmd
	if !cfg.UpdateCheck {
		m.update = Update{}
	} else if !was.UpdateCheck && m.opt.CheckUpdate != nil {
		check := m.opt.CheckUpdate
		cmds = append(cmds, func() tea.Msg { return updateOnceMsg(check(context.Background())) })
	}
	// Config problems and the Linear workspace show after a reload.
	cmds = append(cmds, m.refresh())
	return tea.Batch(cmds...)
}

// valueText is a value as the page shows it.
func valueText(v config.Value, sp config.Spec) string {
	switch {
	case v.Text == "":
		return "none"
	case sp.Kind == config.KindPath:
		return tilde(v.Text)
	}
	return v.Text
}

// settingsDrawer is the settings page.
func (m Model) settingsDrawer(width int) *drawer {
	title := "Settings"
	if p := m.set.cfg.Path; p != "" {
		title += " · " + tilde(p)
	}
	d := newDrawer(width, title)
	switch {
	case m.set.err != "":
		d.styledField("!", warnStyle.Bold(true), false, words("could not read the settings: "+m.set.err, plain))
		return d
	case !m.set.loaded:
		d.line(dim.Render(" Reading the settings…"))
		return d
	}

	const keyW = 22
	srcW := 7
	valW := max(width-1-2-keyW-2-2-srcW, 8)
	group := ""
	for i, sp := range config.Specs {
		if sp.Group != group {
			if group != "" {
				d.line("")
			}
			group = sp.Group
			d.line(" " + dim.Render(group))
		}
		v := m.set.cfg.Values[sp.Key]
		sel := i == m.set.cursor
		mark := "  "
		if sel {
			mark = "▸ "
		}
		var val string
		switch {
		case sel && m.set.editing:
			m.set.input.SetWidth(valW - 1)
			val = fit(m.set.input.View(), valW)
		case v.Text == "":
			val = dim.Render(pad("none", valW))
		default:
			val = pad(valueText(v, sp), valW)
		}
		src := v.Source.String()
		srcSt := dim
		if v.Source == config.FromFlag || v.Source == config.FromEnv {
			srcSt = warnStyle
		}
		line := fit(" "+mark+pad(sp.Key, keyW)+"  "+val+"  "+srcSt.Render(src), width)
		if sel {
			d.focus = len(d.lines)
			line = highlight(line, m.light)
		}
		d.lines = append(d.lines, dline{text: line, setting: i + 1})
		if sel {
			m.settingNotes(d, sp, v)
			d.focusEnd = len(d.lines) - 1
		}
	}
	return d
}

// settingNotes are the selected setting's lines under it: what it does,
// what overrides the file, and the edit's state.
func (m Model) settingNotes(d *drawer, sp config.Spec, v config.Value) {
	note := func(s string, st lipgloss.Style) {
		wrapped := ansi.Wrap(s, max(d.width-6, 10), "")
		for _, l := range strings.Split(wrapped, "\n") {
			d.lines = append(d.lines, dline{text: "     " + st.Render(l)})
		}
	}
	note(sp.Help, dim)
	if o := sp.Override(v.Source); o != "" {
		note("set by "+o+", which wins over the file; a saved value applies once it is gone", warnStyle)
	}
	if v.Note != "" {
		note(v.Note, warnStyle)
	}
	if sp.Restart {
		note("a change applies when the deck restarts", dim)
	}
	if m.set.editing {
		if m.set.invalid != "" {
			note("✕ "+m.set.invalid, failStyle)
		}
		note("↵ saves · esc cancels · empty removes it from the file", dim)
	}
}

package plugin

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestLinkURL(t *testing.T) {
	const figma = "https://www.figma.com/design/AbC123/Admin?node-id=1-2"
	acme := LinkSettings{LinearWorkspace: "acme"}
	desktop := LinkSettings{LinearWorkspace: "acme", FigmaDesktop: true}
	keyWS := LinkSettings{LinearKeyWorkspace: func() (string, error) { return "keyco", nil }}
	keyFails := LinkSettings{LinearKeyWorkspace: func() (string, error) { return "", errors.New("cannot reach Linear: timed out") }}
	bothWS := LinkSettings{LinearWorkspace: "acme", LinearKeyWorkspace: keyWS.LinearKeyWorkspace}
	tests := []struct {
		name, text string
		s          LinkSettings
		want       string
		err        string
	}{
		{"selected ID", "ABC-123", acme, "https://linear.app/acme/issue/ABC-123", ""},
		{"ID in a selected line", "  see ABC-123 and ABC-9.\n", acme, "https://linear.app/acme/issue/ABC-123", ""},
		{"ID after a dash is not one", "P-ABC-49", acme, "", "no link or Linear ID"},
		{"standard, not a ticket", "UTF-8", acme, "", "no link or Linear ID"},
		{"no workspace", "ABC-123", LinkSettings{}, "", "ABC-123: no Linear workspace set"},
		{"workspace from the key", "ABC-123", keyWS, "https://linear.app/keyco/issue/ABC-123", ""},
		{"set workspace wins over asking", "ABC-123", bothWS, "https://linear.app/acme/issue/ABC-123", ""},
		{"asking Linear fails", "ABC-123", keyFails, "", "asking Linear failed: cannot reach Linear"},
		{"Linear URL needs no workspace", "https://linear.app/other/issue/ABC-7", LinkSettings{}, "https://linear.app/other/issue/ABC-7", ""},
		{"Figma in the browser", figma, acme, figma, ""},
		{"Figma in the desktop app", figma, desktop, "figma://design/AbC123/Admin?node-id=1-2", ""},
		{"selected Figma link", "mockup: " + figma, desktop, "figma://design/AbC123/Admin?node-id=1-2", ""},
		{"other URL as it is", "https://example.com/a?b=1", desktop, "https://example.com/a?b=1", ""},
		{"ID in code", "fixed in `ABC-123`", acme, "https://linear.app/acme/issue/ABC-123", ""},
		{"ID under Remember", "## Remember\n- ABC-123 needs a migration", acme, "https://linear.app/acme/issue/ABC-123", ""},
		{"URL before an ID", "see https://example.com/x for ABC-1", acme, "https://example.com/x", ""},
		{"ID before a URL", "ABC-1: https://example.com/x", acme, "https://linear.app/acme/issue/ABC-1", ""},
		{"ID inside a URL is not first", "https://example.com/ABC-1", acme, "https://example.com/ABC-1", ""},
		{"trailing period", figma + ".", desktop, "figma://design/AbC123/Admin?node-id=1-2", ""},
		{"nothing", "  ", acme, "", "no link"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LinkURL(tt.text, tt.s)
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("LinkURL(%q) = %q, %v; want error %q", tt.text, got, err, tt.err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("LinkURL(%q) = %q, %v; want %q", tt.text, got, err, tt.want)
			}
		})
	}
}

func TestRunOpenLink(t *testing.T) {
	tests := []struct {
		name  string
		vars  map[string]string
		args  []string
		want  []string
		fails bool
	}{
		{"clicked URL", map[string]string{
			"HERDR_PLUGIN_CLICKED_URL":  "https://www.figma.com/file/AbC123/Admin",
			"HERDR_PLUGIN_CONTEXT_JSON": `{"selected_text":"ABC-1"}`,
		}, nil, []string{"figma://file/AbC123/Admin"}, false},
		// The shape herdr 0.9.3 passes when a key runs the action on a
		// selection.
		{"selection", map[string]string{
			"HERDR_PLUGIN_CONTEXT_JSON": `{"focused_pane_id":"w1:p1","invocation_source":"keybinding","selected_text":"ABC-123"}`,
		}, nil, []string{"https://linear.app/acme/issue/ABC-123"}, false},
		{"argument", nil, []string{"ABC-5"}, []string{"https://linear.app/acme/issue/ABC-5"}, false},
		{"nothing selected", map[string]string{
			"HERDR_PLUGIN_CONTEXT_JSON": `{"focused_pane_id":"w1:p1","invocation_source":"keybinding"}`,
		}, nil, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opened []string
			h := &fakeHost{}
			env := Env{
				Getenv:  func(k string) string { return tt.vars[k] },
				Links:   LinkSettings{LinearWorkspace: "acme", FigmaDesktop: true},
				OpenURL: func(u string) error { opened = append(opened, u); return nil },
			}
			err := Run(context.Background(), h, env, append([]string{"open-link"}, tt.args...))
			if (err != nil) != tt.fails || !slices.Equal(opened, tt.want) {
				t.Errorf("err %v, opened %q; want %q", err, opened, tt.want)
			}
			// A failure is shown in herdr, not only logged.
			if tt.fails != (len(h.notes) == 1) {
				t.Errorf("notes = %q", h.notes)
			}
		})
	}
}

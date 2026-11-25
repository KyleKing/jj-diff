package config_test

import (
	"testing"

	"github.com/kyleking/aragonite/tui/icons"

	"github.com/kyleking/jj-diff/internal/config"
)

// terminalEnv is every variable icons.NerdTerminal reads, plus ARAGONITE_ICONS, which each case
// clears so the runner's own terminal cannot decide it.
//
//nolint:gochecknoglobals // a fixture
var terminalEnv = []string{
	"TERM", "TERM_PROGRAM", "WEZTERM_EXECUTABLE", "WEZTERM_PANE",
	"KITTY_WINDOW_ID", "GHOSTTY_RESOURCES_DIR", icons.EnvOverride,
}

// JJ_DIFF_ICONS wins over ARAGONITE_ICONS, which wins over the terminal, and the explanation names
// the variable that decided. A JJ_DIFF_ICONS of auto, or one that does not parse, defers.
func TestExplainIcons(t *testing.T) {
	tests := []struct {
		env     map[string]string
		name    string
		setting string
		says    string
		mode    icons.Mode
	}{
		{
			name: "auto in WezTerm", env: map[string]string{"TERM_PROGRAM": "WezTerm"},
			mode: icons.Nerd, says: "nerd (auto: WezTerm)",
		},
		{
			name: "auto elsewhere", env: map[string]string{"TERM_PROGRAM": "Apple_Terminal"},
			mode: icons.Unicode, says: "unicode (auto: no Nerd terminal detected)",
		},
		{
			name: "ARAGONITE_ICONS over the terminal",
			env:  map[string]string{"TERM_PROGRAM": "WezTerm", icons.EnvOverride: "off"},
			mode: icons.None, says: "none (ARAGONITE_ICONS=off)",
		},
		{
			name: "JJ_DIFF_ICONS over ARAGONITE_ICONS", setting: "plain",
			env:  map[string]string{"TERM_PROGRAM": "WezTerm", icons.EnvOverride: "nerd"},
			mode: icons.Unicode, says: "unicode (JJ_DIFF_ICONS=plain)",
		},
		{
			name: "JJ_DIFF_ICONS auto defers", setting: "auto",
			env:  map[string]string{"TERM_PROGRAM": "WezTerm", icons.EnvOverride: "plain"},
			mode: icons.Unicode, says: "unicode (ARAGONITE_ICONS=plain)",
		},
		{
			name: "JJ_DIFF_ICONS unreadable defers", setting: "fancy",
			env:  map[string]string{"KITTY_WINDOW_ID": "1"},
			mode: icons.Nerd, says: "nerd (auto: kitty)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range terminalEnv {
				t.Setenv(k, tt.env[k])
			}

			got := config.ExplainIcons(tt.setting)
			if got.Mode != tt.mode {
				t.Errorf("mode %v, want %v", got.Mode, tt.mode)
			}

			if says := config.SayIcons(got); says != tt.says {
				t.Errorf("says %q, want %q", says, tt.says)
			}
		})
	}
}

package statusbar_test

import (
	"os"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/jj-diff/internal/components/statusbar"
	"github.com/kyleking/jj-diff/internal/theme"
)

// The theme is process-global, so it is pinned once here rather than per test, which is what lets
// every test below run in parallel.
func TestMain(m *testing.M) {
	if err := os.Setenv("CATPPUCCIN_THEME", "macchiato"); err != nil {
		panic(err)
	}

	theme.Init()
	os.Exit(m.Run())
}

// The hints name keys that only work in that mode and panel, so a mode whose keys never appear is
// a mode the user cannot discover without opening the help overlay.
func TestViewWithContext_AdvertisesTheKeysThatWorkHere(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		ctx   statusbar.Context
		wants []string
		omits []string
	}{
		{
			name:  "interactive on the file list offers the destination and apply",
			ctx:   statusbar.Context{Mode: "Interactive", Source: "@", FocusedPanel: "files"},
			wants: []string{"d:dest", "a:apply", "?:help"},
			omits: []string{"/:search"},
		},
		{
			name:  "interactive on the diff offers line selection",
			ctx:   statusbar.Context{Mode: "Interactive", Source: "@", FocusedPanel: "diff"},
			wants: []string{"Space:select", "v:lines", "a:apply"},
		},
		{
			name:  "browse on the file list offers search rather than apply",
			ctx:   statusbar.Context{Mode: "Browse", Source: "@", FocusedPanel: "files"},
			wants: []string{"/:search", "f:find"},
			omits: []string{"a:apply"},
		},
		{
			name:  "visual mode offers only the selection keys",
			ctx:   statusbar.Context{Mode: "Interactive", Source: "@", IsVisualMode: true},
			wants: []string{"VISUAL", "Space:confirm", "Esc:cancel"},
			omits: []string{"a:apply"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ansiless(statusbar.New().ViewWithContext(200, tt.ctx))
			for _, want := range tt.wants {
				if !strings.Contains(got, want) {
					t.Errorf("hint %q missing from %q", want, got)
				}
			}

			for _, omit := range tt.omits {
				if strings.Contains(got, omit) {
					t.Errorf("hint %q should not appear in %q", omit, got)
				}
			}
		})
	}
}

// The help hint is the only route to the rest of the keymap, so it has to survive every width the
// footer is asked to render at.
func TestViewWithContext_KeepsHelpAndMarksWhatItDropped(t *testing.T) {
	t.Parallel()

	ctx := statusbar.Context{Mode: "Interactive", Source: "@", FocusedPanel: "diff"}

	tests := []struct {
		width        int
		wantEllipsis bool
		wantApply    bool
	}{
		// 40 is the one width where the ellipsis itself does not fit, and showing the help key
		// alone beats spending the row on a marker.
		{width: 40, wantEllipsis: false, wantApply: false},
		{width: 60, wantEllipsis: true, wantApply: false},
		{width: 80, wantEllipsis: true, wantApply: false},
		{width: 120, wantEllipsis: false, wantApply: true},
	}

	for _, tt := range tests {
		got := ansiless(statusbar.New().ViewWithContext(tt.width, ctx))

		if !strings.Contains(got, "?:help") {
			t.Errorf("width %d dropped the help hint: %q", tt.width, got)
		}

		if lipgloss.Width(got) != tt.width {
			t.Errorf("width %d rendered %d cells: %q", tt.width, lipgloss.Width(got), got)
		}

		if strings.Contains(got, "…") != tt.wantEllipsis {
			t.Errorf("width %d ellipsis: want %v, got %q", tt.width, tt.wantEllipsis, got)
		}

		if strings.Contains(got, "a:apply") != tt.wantApply {
			t.Errorf("width %d apply hint: want %v, got %q", tt.width, tt.wantApply, got)
		}
	}
}

// A wide glyph counts as two cells, so measuring the source in bytes overflows the row.
func TestViewWithContext_MeasuresWideGlyphsInCells(t *testing.T) {
	t.Parallel()

	//nolint:gosmopolitan // a CJK revision description is the realistic source of wide glyphs
	ctx := statusbar.Context{Mode: "Browse", Source: "更新した変更", FocusedPanel: "files"}

	for _, width := range []int{20, 37, 80} {
		got := ansiless(statusbar.New().ViewWithContext(width, ctx))
		if lipgloss.Width(got) != width {
			t.Errorf("width %d rendered %d cells: %q", width, lipgloss.Width(got), got)
		}
	}
}

func ansiless(s string) string {
	var b strings.Builder

	inEscape := false

	for _, r := range s {
		switch {
		case r == '\x1b':
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case !inEscape:
			b.WriteRune(r)
		}
	}

	return b.String()
}

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
			wants: []string{"d dest", "a apply", "? help"},
			omits: []string{"/ search"},
		},
		{
			name:  "interactive on the diff offers line selection",
			ctx:   statusbar.Context{Mode: "Interactive", Source: "@", FocusedPanel: "diff"},
			wants: []string{"space select", "v lines", "a apply"},
		},
		{
			name:  "browse on the file list offers search rather than apply",
			ctx:   statusbar.Context{Mode: "Browse", Source: "@", FocusedPanel: "files"},
			wants: []string{"/ search", "f find"},
			omits: []string{"a apply"},
		},
		{
			name:  "visual mode offers only the selection keys",
			ctx:   statusbar.Context{Mode: "Interactive", Source: "@", IsVisualMode: true},
			wants: []string{"VISUAL", "space confirm", "esc cancel"},
			omits: []string{"a apply"},
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
// footer is asked to render at. Hints shed whole from the end of the list, never clipped, and the
// facts give way once only help is left to keep.
func TestViewWithContext_KeepsHelpAndShedsFromTheEnd(t *testing.T) {
	t.Parallel()

	ctx := statusbar.Context{Mode: "Interactive", Source: "@", FocusedPanel: "diff"}

	tests := []struct {
		width     int
		wantApply bool
		wantDest  bool
		wantMode  bool
	}{
		{width: 18, wantMode: false},
		{width: 40, wantMode: true},
		{width: 80, wantMode: true, wantDest: true},
		{width: 120, wantMode: true, wantDest: true, wantApply: true},
	}

	for _, tt := range tests {
		got := ansiless(statusbar.New().ViewWithContext(tt.width, ctx))

		if !strings.Contains(got, "? help") {
			t.Errorf("width %d dropped the help hint: %q", tt.width, got)
		}

		if lipgloss.Width(got) != tt.width {
			t.Errorf("width %d rendered %d cells: %q", tt.width, lipgloss.Width(got), got)
		}

		wants := map[string]bool{"a apply": tt.wantApply, "d dest": tt.wantDest, "Interactive": tt.wantMode}
		for hint, want := range wants {
			if strings.Contains(got, hint) != want {
				t.Errorf("width %d %q: want %v, got %q", tt.width, hint, want, got)
			}
		}

		if strings.Contains(got, "|") || strings.Contains(got, ":") {
			t.Errorf("width %d: want space-separated hints, got %q", tt.width, got)
		}
	}
}

// The key wears the accent and its word the muted face, so the two read apart without a colon.
func TestViewWithContext_DrawsKeysInTheAccent(t *testing.T) {
	t.Parallel()

	got := statusbar.New().ViewWithContext(120, statusbar.Context{Mode: "Browse", Source: "@", FocusedPanel: "files"})
	if want := theme.Skin.Accent.Render("?"); !strings.Contains(got, want) {
		t.Errorf("footer %q, want the help key drawn as %q", got, want)
	}

	if want := theme.Skin.Muted.Render("help"); !strings.Contains(got, want) {
		t.Errorf("footer %q, want the help word drawn as %q", got, want)
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

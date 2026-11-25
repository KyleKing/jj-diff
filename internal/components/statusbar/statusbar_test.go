package statusbar_test

import (
	"os"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/hermetic"
	"github.com/kyleking/aragonite/tui/keyhint"

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
	hermetic.Main(m, hermetic.App("jj-diff", "jojo"))
}

// hints builds a footer's hints the way the model does: most useful first, the last pinned.
func hints(pairs ...string) []keyhint.Hint {
	out := make([]keyhint.Hint, 0, len(pairs)/2)

	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, keyhint.Hint{Key: pairs[i], What: pairs[i+1]})
	}

	for i := range out {
		out[i].Priority = len(out) - i
	}

	out[len(out)-1].Pin = true

	return out
}

//nolint:gochecknoglobals // a fixture: the interactive diff pane's footer.
var interactive = hints("space", "select", "v", "lines", "s", "squash", "S", "split", "?", "help")

// The footer draws what the model's keymap passed it, keys bare in front of their words.
func TestViewWithContext_DrawsTheHintsItIsGiven(t *testing.T) {
	t.Parallel()

	got := ansiless(statusbar.New().ViewWithContext(200, &statusbar.Context{
		Mode: "Interactive", Source: "@", Hints: interactive,
	}))

	for _, want := range []string{"Interactive", "space select", "v lines", "s squash", "? help"} {
		if !strings.Contains(got, want) {
			t.Errorf("hint %q missing from %q", want, got)
		}
	}
}

// A notice takes the facts' place, and the keys stay.
func TestViewWithContext_ShowsANoticeInPlaceOfTheFacts(t *testing.T) {
	t.Parallel()

	got := ansiless(statusbar.New().ViewWithContext(120, &statusbar.Context{
		Mode: "Interactive", Source: "umnpntqw", Notice: "squashed 1 hunk into abc", Hints: interactive,
	}))

	if !strings.Contains(got, "squashed 1 hunk into abc") || strings.Contains(got, "umnpntqw") {
		t.Errorf("footer %q, want the notice in place of the source", got)
	}

	if !strings.Contains(got, "? help") {
		t.Errorf("the notice pushed help out: %q", got)
	}
}

// The help hint is the only route to the rest of the keymap, so it has to survive every width the
// footer is asked to render at. Hints shed whole from the end of the list, never clipped, and the
// facts give way once only help is left to keep.
func TestViewWithContext_KeepsHelpAndShedsFromTheEnd(t *testing.T) {
	t.Parallel()

	ctx := statusbar.Context{Mode: "Interactive", Source: "@", Hints: interactive}

	tests := []struct {
		width      int
		wantSplit  bool
		wantSelect bool
		wantMode   bool
	}{
		{width: 18, wantMode: false},
		{width: 30, wantMode: true},
		{width: 40, wantMode: true, wantSelect: true},
		{width: 80, wantMode: true, wantSelect: true, wantSplit: true},
	}

	for _, tt := range tests {
		got := ansiless(statusbar.New().ViewWithContext(tt.width, &ctx))

		if !strings.Contains(got, "? help") {
			t.Errorf("width %d dropped the help hint: %q", tt.width, got)
		}

		if lipgloss.Width(got) != tt.width {
			t.Errorf("width %d rendered %d cells: %q", tt.width, lipgloss.Width(got), got)
		}

		wants := map[string]bool{"S split": tt.wantSplit, "space select": tt.wantSelect, "Interactive": tt.wantMode}
		for hint, want := range wants {
			if strings.Contains(got, hint) != want {
				t.Errorf("width %d %q: want %v, got %q", tt.width, hint, want, got)
			}
		}
	}
}

// A visual selection's way out is esc, so it survives where the selection keys before it do not.
func TestViewWithContext_KeepsEscInAVisualSelection(t *testing.T) {
	t.Parallel()

	ctx := statusbar.Context{
		Mode: "Interactive", Source: "umnpntqw (no description)", IsVisualMode: true,
		Hints: hints("j/k", "extend", "space", "confirm", "esc", "cancel"),
	}

	for _, width := range []int{40, 80} {
		got := ansiless(statusbar.New().ViewWithContext(width, &ctx))
		if !strings.Contains(got, "esc cancel") {
			t.Errorf("width %d dropped esc: %q", width, got)
		}
	}
}

// The key wears the accent and its word the muted face, so the two read apart without a colon. The
// space between them is drawn in the word's face, so the cell under it matches the word's.
func TestViewWithContext_DrawsKeysInTheAccent(t *testing.T) {
	t.Parallel()

	got := statusbar.New().ViewWithContext(120, &statusbar.Context{
		Mode: "Browse", Source: "@", Hints: hints("j/k", "nav", "?", "help"),
	})
	if want := theme.Skin.Accent.Render("?"); !strings.Contains(got, want) {
		t.Errorf("footer %q, want the help key drawn as %q", got, want)
	}

	if want := theme.Skin.Muted.Render(" help"); !strings.Contains(got, want) {
		t.Errorf("footer %q, want the help word drawn as %q", got, want)
	}
}

// A wide glyph counts as two cells, so measuring the source in bytes overflows the row.
func TestViewWithContext_MeasuresWideGlyphsInCells(t *testing.T) {
	t.Parallel()

	//nolint:gosmopolitan // a CJK revision description is the realistic source of wide glyphs
	ctx := statusbar.Context{Mode: "Browse", Source: "更新した変更", Hints: hints("?", "help")}

	for _, width := range []int{20, 37, 80} {
		got := ansiless(statusbar.New().ViewWithContext(width, &ctx))
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

// A long revision name gives way to the footer's leading keys: at 80 and 120 columns the first hints
// after the motion are drawn, and the name is cut instead.
func TestViewWithContext_LongSourceYieldsToTheLeadingHints(t *testing.T) {
	t.Parallel()

	for _, width := range []int{80, 120} {
		ctx := statusbar.Context{
			Mode:   "Interactive",
			Source: "swktoryx refactor: rename err to errv across internal (big diff)",
			Hints:  hints("j/k", "nav", "space", "select", "s", "squash", "?", "help"),
		}

		line := ansiless(statusbar.New().ViewWithContext(width, &ctx))
		for _, want := range []string{"space select", "s squash", "? help"} {
			if !strings.Contains(line, want) {
				t.Errorf("at %d the footer dropped %q for the revision's name: %q", width, want, line)
			}
		}
	}
}

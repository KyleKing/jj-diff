package modal_test

import (
	"os"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/keyhint"

	"github.com/kyleking/jj-diff/internal/components/modal"
	"github.com/kyleking/jj-diff/internal/theme"
)

func TestMain(m *testing.M) {
	theme.Init()
	os.Exit(m.Run())
}

// Every row of a box is the same width, so the frame's right edge is one straight line whatever the
// rows hold, and the title and the hints are both in it.
func TestBoxIsOneWidthAndCarriesTitleAndHints(t *testing.T) {
	t.Parallel()

	body := []string{"short", strings.Repeat("long ", 20), theme.Skin.Accent.Render("styled")}
	hints := []keyhint.Hint{{Key: "enter", What: "open"}, {Key: "esc", What: "cancel"}}
	box := modal.Box("Find file", "2", body, hints, 30)
	lines := strings.Split(box, "\n")

	for i, line := range lines {
		if w := lipgloss.Width(line); w != 30+modal.ChromeCols {
			t.Errorf("line %d %q is %d cells, want %d", i, ansi.Strip(line), w, 30+modal.ChromeCols)
		}
	}

	plain := ansi.Strip(box)
	for _, want := range []string{"── Find file 2 ─", "enter open", "esc cancel", "long long…"} {
		if !strings.Contains(plain, want) {
			t.Errorf("box does not contain %q:\n%s", want, plain)
		}
	}
}

// The cursor row keeps its tint across every styled part of it, rather than losing it at the first
// part's reset.
func TestRowKeepsTheSelectionAcrossStyledParts(t *testing.T) {
	t.Parallel()

	open, _, _ := strings.Cut(theme.Skin.Selection.Render("\x00"), "\x00")
	row := modal.Row(theme.Skin.Accent.Render("ab")+theme.Skin.Muted.Render("cd"), 10, true)

	if !strings.HasPrefix(row, open) {
		t.Fatalf("row %q does not open on the selection", row)
	}

	if strings.Count(row, open) < 3 {
		t.Errorf("row %q sets the selection %d times, want it after each part's reset", row, strings.Count(row, open))
	}

	if w := lipgloss.Width(row); w != 10 {
		t.Errorf("row is %d cells, want 10", w)
	}
}

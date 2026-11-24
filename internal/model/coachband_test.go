//nolint:testpackage // white-box: the band is switched on through the unexported field the config sets.
package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/jj-diff/internal/diff"
)

// manyHunks is one file of n single-line hunks, far more rows than any frame shows.
func manyHunks(n int) []diff.FileChange {
	var b strings.Builder

	b.WriteString("diff --git a/long.go b/long.go\n--- a/long.go\n+++ b/long.go\n")

	for i := range n {
		at := i*10 + 1
		fmt.Fprintf(&b, "@@ -%d,2 +%d,3 @@\n line %d\n+added %d\n line %d\n", at, at, at, i, at+1)
	}

	return diff.Parse(b.String())
}

// The band takes rows from the screen rather than covering them: with the cursor moved to the
// last hunk, its row is still drawn above the band, and the frame is exactly the terminal's height.
func TestCoachBand_ReservesRowsSoTheCursorStaysInView(t *testing.T) {
	t.Parallel()

	for _, size := range []struct{ w, h int }{{80, 24}, {120, 40}} {
		t.Run(fmt.Sprintf("%dx%d", size.w, size.h), func(t *testing.T) {
			t.Parallel()

			m := NewTestModel(t, ModeInteractive).WithChanges(manyHunks(60))
			m.focusedPanel = PanelDiffView
			m.width, m.height = size.w, size.h
			m.band = true

			band := len(m.bandLines())
			if band == 0 {
				t.Fatal("the band has nothing to say, so the test proves nothing")
			}

			m = Press(t, m, "G")

			lines := strings.Split(ansi.Strip(m.render()), "\n")
			if len(lines) != size.h {
				t.Fatalf("frame is %d rows, want %d", len(lines), size.h)
			}

			above := lines[:size.h-1-band]
			if !strings.Contains(strings.Join(above, "\n"), "added 59") {
				t.Errorf("the last hunk is not drawn above the band:\n%s", strings.Join(lines, "\n"))
			}
		})
	}
}

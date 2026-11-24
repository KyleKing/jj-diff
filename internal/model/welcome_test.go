//nolint:testpackage // white-box: these tests read the welcome card's state and the palette's.
package model

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/coach"
	"github.com/kyleking/aragonite/tui/palette"
)

// firstRun is a model on a ledger's first session, which is what a first launch opens.
func firstRun(t *testing.T, mode OperatingMode, width, height int) (*Model, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "coach.json")

	l, err := coach.LedgerAt(path)
	if err != nil {
		t.Fatal(err)
	}

	m := NewTestModel(t, mode).WithChanges(TestChanges()).WithLedger(l)
	m.width, m.height = width, height

	return m, path
}

// The first launch opens on the welcome card in browse and in -i, at both sizes a terminal opens at,
// with the starter keys the mode's keymap binds.
func TestWelcome_Golden(t *testing.T) {
	t.Parallel()

	for _, mode := range []OperatingMode{ModeBrowse, ModeInteractive, ModeDiffEditor} {
		for _, size := range frameSizes {
			word := map[OperatingMode]string{
				ModeBrowse: "browse", ModeInteractive: "interactive", ModeDiffEditor: "editor",
			}
			name := fmt.Sprintf("welcome_%s_%dx%d", word[mode], size.w, size.h)

			t.Run(name, func(t *testing.T) {
				t.Parallel()

				m, _ := firstRun(t, mode, size.w, size.h)

				frame := ansi.Strip(m.render())
				if !strings.Contains(frame, "Welcome to jj-diff") || !strings.Contains(frame, "]h") {
					t.Fatalf("the first launch did not open on the card:\n%s", frame)
				}

				frameGolden(t, filepath.Join("testdata", name+".golden"), frame+"\n", size.w)
			})
		}
	}
}

// Any key takes the card down and still acts, esc only takes it down, the dismissal is saved, and
// the next launch is not greeted.
func TestWelcome_AnyKeyDismissesAndActsOnce(t *testing.T) {
	t.Parallel()

	m, path := firstRun(t, ModeBrowse, 100, 30)
	m = Press(t, m, "?")

	if m.welcome.Showing() || !m.helpOpen() {
		t.Fatalf("? should close the card and open the legend: card %v, legend %v", m.welcome.Showing(), m.helpOpen())
	}

	again, err := coach.LedgerAt(path)
	if err != nil {
		t.Fatal(err)
	}

	if coach.Due(again) {
		t.Error("the dismissal was not saved, so the next launch greets again")
	}

	esc, _ := firstRun(t, ModeBrowse, 100, 30)
	esc = Press(t, esc, "esc")

	if esc.welcome.Showing() || esc.helpOpen() {
		t.Error("esc should only close the card")
	}
}

// :welcome opens the card again, and a terminal too small for it says it in the footer.
func TestWelcome_ReopensAndFallsBackToALine(t *testing.T) {
	t.Parallel()

	m, _ := firstRun(t, ModeBrowse, 100, 30)
	m = Press(t, m, "esc", ":", "w", "e", "l", "c", "o", "m", "e")

	if sel, ok := m.pal.Selected(); !ok || sel.Action != coach.WelcomeAction {
		t.Fatalf("typing welcome selected %q, want :welcome", sel.Action)
	}

	m.pal.Close() // the palette closes as it sends what it ran
	m = Update(t, m, palette.RunMsg{Action: coach.WelcomeAction, From: palette.Run})

	if !m.welcome.Showing() {
		t.Fatal(":welcome did not reopen the card")
	}

	m.width, m.height = 70, 14

	frame := ansi.Strip(m.render())
	if strings.Contains(frame, "Welcome to jj-diff") || !strings.Contains(frame, "◇ welcome") {
		t.Errorf("a 70x14 terminal should carry the card as a footer line:\n%s", frame)
	}
}

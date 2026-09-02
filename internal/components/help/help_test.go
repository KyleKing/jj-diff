package help_test

import (
	"os"
	"strings"
	"testing"

	"github.com/kyleking/jj-diff/internal/components/help"
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

// Interactive is the mode with the most bindings, so it is what overflows a short terminal.
func shown(t *testing.T) *help.Model {
	t.Helper()

	m := help.New()
	m.Show("Interactive")

	return &m
}

// A terminal shorter than the overlay used to clip the bindings off both ends with nothing saying
// so, which left the keys below the fold unreachable.
func TestView_ShortTerminalScrollsInsteadOfClipping(t *testing.T) {
	t.Parallel()

	m := shown(t)

	top := m.View(80, 24)
	if !strings.Contains(top, "Keybindings") {
		t.Error("the title should be visible before scrolling")
	}

	if strings.Contains(top, "Interactive Mode") {
		t.Fatal("this test assumes the guide starts below the fold at 24 rows")
	}

	if !strings.Contains(top, "j/k to scroll") {
		t.Error("the footer should say the overlay scrolls")
	}

	m.ScrollToEnd()

	end := m.View(80, 24)
	if !strings.Contains(end, "Interactive Mode") {
		t.Error("scrolling to the end should reach the guide that starts below the fold")
	}

	if !strings.Contains(end, "100%") {
		t.Error("the footer should report the position after scrolling to the end")
	}
}

// The close hint is the only thing telling a first-timer how to get out, so it survives every size.
func TestView_AlwaysOffersTheCloseHint(t *testing.T) {
	t.Parallel()

	for _, height := range []int{8, 16, 24, 40, 80} {
		got := shown(t).View(80, height)
		if !strings.Contains(got, "? or Esc to close") {
			t.Errorf("height %d dropped the close hint", height)
		}
	}
}

// View clamps the offset, so a caller may scroll past either end without the overlay going blank.
func TestView_ClampsScrollPastEitherEnd(t *testing.T) {
	t.Parallel()

	m := shown(t)

	m.ScrollToEnd()
	atEnd := m.View(80, 24)

	m.ScrollBy(500)

	if got := m.View(80, 24); got != atEnd {
		t.Error("scrolling past the end should render the same frame as the end")
	}

	m.ScrollToStart()
	atStart := m.View(80, 24)

	m.ScrollBy(-500)

	if got := m.View(80, 24); got != atStart {
		t.Error("scrolling above the start should render the same frame as the start")
	}
}

// A terminal tall enough for every binding should not offer scrolling at all.
func TestView_TallTerminalShowsEverythingWithoutScrolling(t *testing.T) {
	t.Parallel()

	got := shown(t).View(80, 60)

	for _, want := range []string{"Keybindings", "Navigation", "Interactive Mode", "Press ? or Esc to close"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q missing from a 60-row render", want)
		}
	}

	if strings.Contains(got, "j/k to scroll") {
		t.Error("a terminal that fits the overlay should not offer scrolling")
	}
}

package diffview_test

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/jj-diff/internal/components/diffview"
	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
)

// A large file's first frame draws its code plain rather than waiting on the lexer, and the colors
// arrive on a later frame without moving a single cell.
func TestFirstFrameDrawsPlainThenColorsArrive(t *testing.T) {
	t.Parallel()

	m := diffview.New(config.DefaultConfig())
	m.SetFileChange(realisticFile(4000))

	first := m.View(120, 30, true)

	select {
	case <-m.Highlighted():
		t.Skip("the lexer finished before the first frame, so there is no plain frame to check")
	default:
	}

	cmd := m.HighlightCmd()
	if cmd == nil {
		t.Fatal("HighlightCmd answered nil while the file's colors were still on their way")
	}

	msg, ok := cmd().(diffview.HighlightedMsg)
	if !ok || msg.Path != "svc.go" {
		t.Fatalf("HighlightCmd's command answered %#v, want HighlightedMsg for svc.go", msg)
	}

	colored := m.View(120, 30, true)

	if ansi.Strip(first) != ansi.Strip(colored) {
		t.Errorf("colors moved text\n--- plain\n%s\n--- colored\n%s", ansi.Strip(first), ansi.Strip(colored))
	}

	if first == colored {
		t.Error("the frame after the colors landed is the plain frame, want syntax colors")
	}
}

func TestHighlightCmdIsHandedOutOncePerFile(t *testing.T) {
	t.Parallel()

	m := diffview.New(config.DefaultConfig())

	if m.HighlightCmd() != nil {
		t.Error("a pane with no file handed out a wait for colors")
	}

	m.SetFileChange(realisticFile(2000))

	// Loading a file lexes nothing until the parent asks, so a file stepped past costs no lexing.
	if m.HighlightCmd() != nil || m.Lexing() {
		t.Fatal("loading a file started lexing it before Lex")
	}

	m.Lex()

	if m.HighlightCmd() == nil {
		t.Fatal("lexing a large file handed out no wait for its colors")
	}

	if m.HighlightCmd() != nil {
		t.Error("a second ask for the same file handed out another wait")
	}

	<-m.Highlighted()
	m.SetFileChange(realisticFile(10))
	<-m.Highlighted()

	if m.HighlightCmd() != nil {
		t.Error("a file whose colors already landed handed out a wait")
	}
}

// replay is one step of what a parent does to the pane between frames.
type replay struct {
	apply func(*diffview.Model)
	name  string
}

// Rows drawn once are kept and reused, so every frame after a change of state must match a pane
// that never drew before and was put in that state directly.
func TestCachedRowsMatchAFreshDraw(t *testing.T) {
	t.Parallel()

	file := mixedDiffFile(t)
	steps := []replay{
		{name: "scroll down", apply: func(m *diffview.Model) { m.Scroll(3) }},
		{name: "scroll back", apply: func(m *diffview.Model) { m.Scroll(-3) }},
		{name: "select hunk", apply: func(m *diffview.Model) {
			m.SetSelection(0, func(h int) bool { return h == 0 })
		}},
		{name: "visual range", apply: func(m *diffview.Model) {
			m.SetVisualState(4, true, 2, func(_, l int) bool { return l == 3 })
		}},
		{name: "cursor moves", apply: func(m *diffview.Model) {
			m.SetVisualState(5, true, 2, func(_, l int) bool { return l == 3 })
		}},
		{name: "pick a line", apply: func(m *diffview.Model) {
			m.SetVisualState(0, false, 0, func(_, l int) bool { return l == 1 })
		}},
		{name: "pick another", apply: func(m *diffview.Model) {
			m.SetVisualState(0, false, 0, func(_, l int) bool { return l == 2 })
		}},
		{name: "search", apply: func(m *diffview.Model) {
			m.SetSearchState(true, func(_, l int) []diffview.MatchRange {
				return []diffview.MatchRange{{Start: 1, End: 1 + l%3}}
			})
		}},
		{name: "search ends", apply: func(m *diffview.Model) { m.SetSearchState(false, nil) }},
		{name: "line numbers", apply: func(m *diffview.Model) { m.ToggleLineNumbers() }},
		{name: "word diff", apply: func(m *diffview.Model) { m.ToggleWordDiff() }},
		{name: "whitespace", apply: func(m *diffview.Model) { m.ToggleWhitespace() }},
		{name: "side by side", apply: func(m *diffview.Model) { m.ToggleSideBySide() }},
		{name: "unified again", apply: func(m *diffview.Model) { m.ToggleSideBySide() }},
	}

	for _, width := range []int{40, 100} {
		cached := diffview.New(config.DefaultConfig())
		cached.SetFileChange(file)
		waitColors(t, &cached)
		cached.View(width, 20, true)

		for i, step := range steps {
			step.apply(&cached)

			fresh := diffview.New(config.DefaultConfig())
			fresh.SetFileChange(file)
			waitColors(t, &fresh)

			for _, earlier := range steps[:i+1] {
				earlier.apply(&fresh)
			}

			if got, want := cached.View(width, 20, true), fresh.View(width, 20, true); got != want {
				t.Errorf("width %d after %q: a pane reusing rows drew\n%s\nwant\n%s", width, step.name, got, want)
			}
		}
	}
}

// A search hit at the very end of a line used to draw the line's text a second time.
func TestSearchHitAtTheEndOfALineDrawsTheTextOnce(t *testing.T) {
	t.Parallel()

	m := diffview.New(config.DefaultConfig())
	m.SetFileChange(diff.FileChange{
		Path: "notes.txt", ChangeType: diff.ChangeTypeModified,
		Hunks: []diff.Hunk{{Header: "@@ -1 +1 @@", Lines: []diff.Line{
			{Type: diff.LineContext, Content: "}", OldLineNum: 1, NewLineNum: 1},
		}}},
	})
	m.SetSearchState(true, func(_, _ int) []diffview.MatchRange { return []diffview.MatchRange{{Start: 1, End: 4}} })

	row := rowContaining(ansi.Strip(m.View(40, 4, true)), "}")
	if strings.Count(row, "}") != 1 {
		t.Errorf("row %q draws the line's text more than once", row)
	}
}

func mixedDiffFile(t *testing.T) diff.FileChange {
	t.Helper()

	return fileNamed(t, "main.go")
}

// waitColors bounds a test's wait for the lexer, which is milliseconds for a fixture.
func waitColors(t *testing.T, m *diffview.Model) {
	t.Helper()

	select {
	case <-m.Highlighted():
	case <-time.After(10 * time.Second):
		t.Fatal("the file's colors never landed")
	}
}

// A file shown before is not lexed again: its colors are ready the moment it comes back, as long as
// it is the same parse of the diff.
func TestLex_AFileShownBeforeAnswersAtOnce(t *testing.T) {
	t.Parallel()

	m := diffview.New(config.DefaultConfig())
	big, small := realisticFile(2000), realisticFile(10)

	m.SetFileChange(big)
	<-m.Highlighted()
	m.SetFileChange(small)
	<-m.Highlighted()

	m.SetFileChange(big)
	m.Lex()

	if m.HighlightCmd() != nil {
		t.Error("stepping back to a lexed file waited for its colors again")
	}
}

package render_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/jj-diff/render"
)

const sample = `diff --git a/main.go b/main.go
index 111..222 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@
 package main
-const limit = 10
+const limit = 20
`

func TestLinesRendersEveryPartOfADiff(t *testing.T) {
	t.Parallel()

	lines := render.Lines(sample, render.Options{})

	joined := strings.Join(lines, "\n")
	want := []string{"main.go", "@@ -1,3 +1,3 @@", " package main", "-const limit = 10", "+const limit = 20"}

	for _, want := range want {
		if !strings.Contains(joined, want) {
			t.Errorf("Lines() = %q, want it to contain %q", joined, want)
		}
	}
}

// jj's own diff format is not git format, and a caller passing it would otherwise get a blank pane
// rather than a reason.
func TestLinesReturnsNothingForInputThatIsNotAGitDiff(t *testing.T) {
	t.Parallel()

	if lines := render.Lines("Added regular file c.txt:\n        1: c\n", render.Options{}); lines != nil {
		t.Errorf("Lines() = %#v, want nothing for jj's own diff format", lines)
	}
}

func countMarked(t *testing.T, lines []string, width int) int {
	t.Helper()

	var marked int

	for _, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Errorf("line %q is %d cells wide, want at most %d", line, got, width)
		}

		if strings.Contains(line, "…") {
			marked++
		}
	}

	return marked
}

// Cut lines carry a marker, lines that fit carry none, and marking never changes the row count.
func TestLinesTruncatesToDisplayCellsAndMarksTheCut(t *testing.T) {
	t.Parallel()

	palette := render.Palette{Added: lipgloss.Color("2"), Removed: lipgloss.Color("1")}

	for _, tc := range []struct {
		name       string
		width      int
		sideBySide bool
	}{
		{name: "unified", width: 12},
		{name: "side by side", width: 30, sideBySide: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cut := render.Lines(sample, render.Options{Width: tc.width, SideBySide: tc.sideBySide, Palette: palette})
			whole := render.Lines(sample, render.Options{Width: 200, SideBySide: tc.sideBySide, Palette: palette})

			if len(cut) != len(whole) {
				t.Fatalf("truncating changed the row count: got %d, want %d", len(cut), len(whole))
			}

			if countMarked(t, cut, tc.width) == 0 {
				t.Errorf("no line carries the truncation marker: %q", cut)
			}

			if got := countMarked(t, whole, 200); got != 0 {
				t.Errorf("%d lines are marked as cut though they fit: %q", got, whole)
			}
		})
	}
}

// A wide glyph counts as two cells, so a line of them must still fit the pane it is drawn into.
func TestLinesTruncatesWideGlyphsByWidthNotCount(t *testing.T) {
	t.Parallel()

	const width = 8

	//nolint:gosmopolitan // a wide glyph is the point: it counts as two cells and one rune.
	wide := "diff --git a/w.txt b/w.txt\n--- a/w.txt\n+++ b/w.txt\n@@ -1,1 +1,1 @@\n+一二三四五六七八九十\n"

	for _, line := range render.Lines(wide, render.Options{Width: width}) {
		if got := lipgloss.Width(line); got > width {
			t.Errorf("line %q is %d cells wide, want at most %d", line, got, width)
		}
	}
}

// Word diff marks what changed inside the line rather than the whole line, which is the reason to
// render a diff here instead of printing jj's output.
func TestWordDiffMarksOnlyTheChangedRun(t *testing.T) {
	t.Parallel()

	opts := render.Options{
		WordDiff: true,
		Palette:  render.Palette{AddedWord: lipgloss.Color("2"), RemovedWord: lipgloss.Color("1")},
	}

	plain := render.Lines(sample, render.Options{})
	marked := render.Lines(sample, opts)

	if len(plain) != len(marked) {
		t.Fatalf("word diff changed the line count: %d, want %d", len(marked), len(plain))
	}

	var found bool

	for i, line := range marked {
		if strings.Contains(line, "\x1b[") && !strings.Contains(plain[i], "\x1b[") {
			found = true
		}

		if lipgloss.Width(line) != lipgloss.Width(plain[i]) {
			t.Errorf("word diff changed line %d's width from %d to %d",
				i, lipgloss.Width(plain[i]), lipgloss.Width(line))
		}
	}

	if !found {
		t.Error("word diff marked nothing in a line that changed one number")
	}
}

func TestLineNumbersPutBothSidesInTheGutter(t *testing.T) {
	t.Parallel()

	lines := render.Lines(sample, render.Options{LineNumbers: true})

	var addition string

	for _, line := range lines {
		if strings.Contains(line, "const limit = 20") {
			addition = line
		}
	}

	if addition == "" {
		t.Fatal("the addition was not rendered")
	}

	// An addition exists only on the new side, so the old column stays blank.
	if !strings.HasPrefix(addition, strings.Repeat(" ", 6)) {
		t.Errorf("addition = %q, want a blank old-side column", addition)
	}

	if !strings.Contains(addition, "2 +") {
		t.Errorf("addition = %q, want its new line number in the gutter", addition)
	}
}

// Side-by-side has to keep the separator in one column at every row, or the two panes shear apart.
func TestSideBySideKeepsTheColumnsAligned(t *testing.T) {
	t.Parallel()

	// (40 - 3) / 2 panes = 18 cells, plus the separator's own leading space.
	const (
		width           = 40
		separatorColumn = 19
	)

	lines := render.Lines(sample, render.Options{Width: width, SideBySide: true})
	if len(lines) == 0 {
		t.Fatal("side by side rendered nothing")
	}

	var rows int

	for _, line := range lines {
		left, _, split := strings.Cut(line, "│")
		if !split {
			continue
		}

		rows++

		if got := lipgloss.Width(line); got > width {
			t.Errorf("row %q is %d cells wide, want at most %d", line, got, width)
		}

		if got := lipgloss.Width(left); got != separatorColumn {
			t.Errorf("row %q puts the separator at column %d, want %d", line, got, separatorColumn)
		}
	}

	if rows == 0 {
		t.Error("no two-column rows were drawn")
	}
}

// A modified line has to sit opposite what replaced it, which is the whole reason for the layout.
func TestSideBySidePutsAReplacementOppositeWhatItReplaced(t *testing.T) {
	t.Parallel()

	lines := render.Lines(sample, render.Options{Width: 60, SideBySide: true})

	var found bool

	for _, line := range lines {
		if strings.Contains(line, "limit = 10") && strings.Contains(line, "limit = 20") {
			found = true
		}
	}

	if !found {
		t.Errorf("the deletion and its addition are not on one row: %q", lines)
	}
}

// Lines added above a context line shift its new number but not its old one.
func TestSideBySideNumbersEachColumnByItsOwnFile(t *testing.T) {
	t.Parallel()

	const insertion = `diff --git a/f.go b/f.go
--- a/f.go
+++ b/f.go
@@ -58,6 +58,8 @@
 line58
 line59
 line60
+added1
+added2
 line61
 line62
 line63
`

	lines := render.Lines(insertion, render.Options{Width: 60, SideBySide: true, LineNumbers: true})

	for _, want := range []struct{ text, old, next string }{
		{"line60", "60", "60"},
		{"line61", "61", "63"},
		{"line63", "63", "65"},
	} {
		var row string

		for _, line := range lines {
			if strings.Contains(line, want.text) {
				row = line
			}
		}

		left, right, _ := strings.Cut(row, "│")
		if !strings.Contains(left, want.old+" "+want.text) || !strings.Contains(right, want.next+" "+want.text) {
			t.Errorf("row %q numbers %s as %s | %s, want %s | %s", row, want.text, left, right, want.old, want.next)
		}
	}
}

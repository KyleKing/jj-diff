//nolint:testpackage // white-box: these tests read the columns' state.
package model

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/keys"
)

// inColumns is an interactive model on TestChanges with the split columns open, the diff pane
// focused as S needs it.
func inColumns(t *testing.T) *Model {
	t.Helper()

	m := NewTestModel(t, ModeInteractive).WithChanges(TestChanges())
	m.focusedPanel = PanelDiffView

	return Press(t, m, "S")
}

func owners(m *Model) string {
	out := make([]string, 0, m.cols.count())

	for col := range m.cols.count() {
		h, a, r := m.columnCounts(col)
		out = append(out, fmt.Sprintf("%d:%dh+%d-%d", col+1, h, a, r))
	}

	return strings.Join(out, " ")
}

func TestColumns_SOpensWithTheHunkAtTheCursorInANewCommit(t *testing.T) {
	t.Parallel()

	m := inColumns(t)

	if m.cols == nil || m.context() != keys.Split {
		t.Fatal("S did not open the columns")
	}

	if got := owners(m); got != "1:3h+3-2 2:1h+1-0" {
		t.Errorf("columns = %s, want the first hunk alone in column 2", got)
	}

	if m.cols.focus != 1 {
		t.Errorf("focus = %d, want column 2", m.cols.focus+1)
	}
}

func TestColumns_MoveAndSend(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		keys []string
	}{
		{"> from the last column makes one", "1:3h+3-2 2:0h+0-0 3:1h+1-0", []string{">"}},
		{"< moves back to the source", "1:4h+4-2 2:0h+0-0", []string{"<"}},
		{"a digit one past the last makes one", "1:3h+3-2 2:0h+0-0 3:1h+1-0", []string{"3"}},
		{"a digit further out is refused", "1:3h+3-2 2:1h+1-0", []string{"5"}},
		{"h then > sends the source's hunk at its cursor", "1:2h+2-2 2:2h+2-0", []string{"h", ">"}},
		{
			"a selection of three hunks moves together", "1:0h+0-0 2:4h+4-2",
			[]string{"h", "space", "j", "space", "j", "space", "2"},
		},
		{"undo puts it back", "1:3h+3-2 2:1h+1-0", []string{">", "u"}},
		{"redo moves it again", "1:3h+3-2 2:0h+0-0 3:1h+1-0", []string{">", "u", "ctrl+r"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := Press(t, inColumns(t), tt.keys...)
			if got := owners(m); got != tt.want {
				t.Errorf("columns = %s, want %s", got, tt.want)
			}
		})
	}
}

// A line range moves only its lines, leaving the rest of the hunk where it was.
func TestColumns_VisualMovesLines(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive).WithChanges(TestChanges())
	m.focusedPanel = PanelDiffView
	m.openColumns(nil)

	// file2.txt's hunk adds two lines: take just the first.
	m = Press(t, m, "]", "f", "v", "space", ">")

	if got := owners(m); got != "1:4h+3-2 2:1h+1-0" {
		t.Errorf("columns = %s", got)
	}
}

func TestColumns_DescribeInline(t *testing.T) {
	t.Parallel()

	m := Press(t, inColumns(t), "d", "a", "d", "d", " ", "x", "backspace", "enter")
	if got := m.cols.descs[1]; got != "add " {
		t.Errorf("description = %q", got)
	}

	m = Press(t, m, "d", "z", "esc")
	if got := m.cols.descs[1]; got != "add " {
		t.Errorf("esc kept the draft: %q", got)
	}

	m = Press(t, m, "h", "d")
	if m.cols.describing {
		t.Error("the source's column took a description")
	}
}

func TestColumns_EscBacksOutOneLevel(t *testing.T) {
	t.Parallel()

	m := Press(t, inColumns(t), "space")
	if m.cols.sel.IsEmpty() {
		t.Fatal("space selected nothing")
	}

	m = Press(t, m, "esc")
	if m.cols == nil || !m.cols.sel.IsEmpty() {
		t.Fatal("esc did not clear the selection first")
	}

	if m = Press(t, m, "esc"); m.cols != nil {
		t.Error("esc did not leave the columns")
	}
}

// A chain's patches each take in the columns before them; parallel ones stand alone, and an empty
// column makes no commit.
func TestColumns_GroupsForTheSplit(t *testing.T) {
	t.Parallel()

	for _, parallel := range []bool{false, true} {
		m := Press(t, inColumns(t), "h", "]", "f", ">", "l", ">", "l", "d", "t", "w", "o", "enter")
		m.cols.addColumn()
		m.cols.parallel = parallel

		groups := m.splitGroups()
		if len(groups) != 2 {
			t.Fatalf("parallel=%v: %d groups, want 2 with the empty column dropped", parallel, len(groups))
		}

		// Column 2 holds file2.txt, column 3 file1.txt's first hunk.
		second := groups[1].Through
		if got := strings.Contains(second, "+first line"); got == parallel {
			t.Errorf("parallel=%v: the second patch holds column 2's hunk: %v\n%s", parallel, got, second)
		}

		if !strings.Contains(second, "+new line") || groups[1].Description != "two" {
			t.Errorf("parallel=%v: the second group is %+v", parallel, groups[1])
		}
	}
}

func TestColumns_ParallelFlagShowsInTheCommand(t *testing.T) {
	t.Parallel()

	m := inColumns(t)
	if got := m.splitCommand(); !strings.Contains(got, "chain 2 → 1") || strings.Contains(got, "--parallel") {
		t.Errorf("command = %q", got)
	}

	m = Press(t, m, "-", "p")
	if got := m.splitCommand(); !strings.Contains(got, "--parallel") || !strings.Contains(got, "siblings 2 · 1") {
		t.Errorf("command after -p = %q", got)
	}
}

func TestColumns_ApplyWithNothingMovedWritesNothing(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive).WithChanges(TestChanges())
	m.revision = "@"
	m.openColumns(nil)

	if cmd := m.dispatch(actionApply); cmd != nil || m.cols == nil || !m.alarm {
		t.Errorf("an empty split ran: cmd=%v notice=%q", cmd != nil, m.notice)
	}
}

func TestColumns_Golden(t *testing.T) {
	t.Parallel()

	for _, size := range []struct{ w, h int }{{80, 24}, {120, 40}} {
		t.Run(fmt.Sprintf("%dx%d", size.w, size.h), func(t *testing.T) {
			t.Parallel()

			m := Press(t, inColumns(t), "d", "a", "d", "d", "enter", ">", "h", "]", "f", "v", "space")
			m.width, m.height = size.w, size.h

			frame := ansi.Strip(m.render())
			if lines := strings.Split(frame, "\n"); len(lines) != size.h {
				t.Errorf("the frame is %d rows, want %d", len(lines), size.h)
			}

			path := filepath.Join("testdata", fmt.Sprintf("columns_%dx%d.golden", size.w, size.h))
			frameGolden(t, path, frame+"\n", size.w)
		})
	}
}

// An empty column keeps only a strip, so the columns holding hunks get its width: with column 2
// emptied, both full columns are wider than an even three-way split would leave them.
func TestColumns_EmptyColumnCollapsesToAStrip(t *testing.T) {
	t.Parallel()

	for _, width := range []int{80, 100, 120, 160} {
		t.Run(fmt.Sprintf("%dx24", width), func(t *testing.T) {
			t.Parallel()

			m := Press(t, inColumns(t), "d", "a", "d", "d", "enter", ">", "h")
			m.width, m.height = width, 24
			inner := width - 2*paneMargin

			if got := owners(m); !strings.HasPrefix(got, "1:") || !strings.Contains(got, " 2:0h+0-0 ") {
				t.Fatalf("owners = %q, want column 2 empty", got)
			}

			first, shown := m.columnWindow(inner)
			if first != 0 || shown != m.cols.count() {
				t.Fatalf("columnWindow(%d) = %d, %d, want every column shown once one is a strip", inner, first, shown)
			}

			checkStripWidths(t, m.columnWidths(first, shown, inner), inner)

			for i, line := range strings.Split(ansi.Strip(m.render()), "\n") {
				if got := ansi.StringWidth(line); got > width {
					t.Errorf("row %d is %d cells, want at most %d: %q", i, got, width, line)
				}
			}
		})
	}
}

// checkStripWidths asserts column 2 is the strip, the others are wider than an even split, and the
// columns and their separators fill inner exactly.
func checkStripWidths(t *testing.T, widths []int, inner int) {
	t.Helper()

	shown := len(widths)
	even := (inner - (shown-1)*columnSeparatorWidth) / shown

	if widths[1] != emptyColumnWidth {
		t.Errorf("empty column is %d cells, want the %d-cell strip", widths[1], emptyColumnWidth)
	}

	total := (shown - 1) * columnSeparatorWidth

	for i, w := range widths {
		total += w

		if i != 1 && w <= even {
			t.Errorf("column %d is %d cells, want more than the even share %d", i+1, w, even)
		}
	}

	if total != inner {
		t.Errorf("columns and separators fill %d cells, want %d", total, inner)
	}
}

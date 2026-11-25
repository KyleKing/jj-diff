//nolint:testpackage // white-box: these tests read the columns' draft and the footer.
package model

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// The prompt contract: in the column describe, enter submits, ctrl+j and alt+enter each start a new
// line, and esc cancels.
func TestPrompt_DescribeTakesCtrlJAndAltEnterAsNewline(t *testing.T) {
	t.Parallel()

	m := Press(t, inColumns(t), "d", "a", "ctrl+j", "b")
	m = Update(t, m, tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	m = Press(t, m, "c")

	if !m.cols.describing {
		t.Fatal("a newline key submitted the description")
	}

	m = Press(t, m, "enter")
	if got := m.cols.descs[1]; got != "a\nb\nc" {
		t.Errorf("description = %q, want three lines", got)
	}

	// The header draws the first line, and the frame keeps its height.
	frame := m.View().Content
	if !strings.Contains(frame, " a") || strings.Count(frame, "\n") != m.height-1 {
		t.Errorf("a multi-line description broke the header:\n%s", frame)
	}
}

// While a prompt has the keys, the footer says the contract: enter, ctrl+j where a newline means
// something, and esc.
func TestPrompt_FooterSaysTheContract(t *testing.T) {
	t.Parallel()

	cases := []struct {
		open  func(t *testing.T) *Model
		name  string
		never string
		want  []string
	}{
		{
			name:  "describe",
			open:  func(t *testing.T) *Model { t.Helper(); return Press(t, inColumns(t), "d") },
			want:  []string{"enter set", "ctrl+j newline", "esc cancel"},
			never: "",
		},
		{
			name: "file filter",
			open: func(t *testing.T) *Model {
				t.Helper()

				m := NewTestModel(t, ModeInteractive)
				m.loaded(TestChanges())

				return Press(t, m, "f")
			},
			want:  []string{"enter open", "esc cancel"},
			never: "newline",
		},
		{
			name: "search",
			open: func(t *testing.T) *Model {
				t.Helper()

				m := NewTestModel(t, ModeInteractive)
				m.loaded(TestChanges())

				return Press(t, m, "/")
			},
			want:  []string{"enter keep", "esc cancel"},
			never: "newline",
		},
	}

	for _, tc := range cases {
		for _, width := range []int{80, 120} {
			m := Update(t, tc.open(t), tea.WindowSizeMsg{Width: width, Height: 24})

			foot := lastRow(m)
			for _, want := range tc.want {
				if !strings.Contains(foot, want) {
					t.Errorf("%s at %d: footer lacks %q: %q", tc.name, width, want, foot)
				}
			}

			if tc.never != "" && strings.Contains(foot, tc.never) {
				t.Errorf("%s at %d: a single-line prompt offers %q: %q", tc.name, width, tc.never, foot)
			}
		}
	}
}

// Enter in the file filter opens the top match, not the file the cursor sat on before the filter
// left it out.
func TestPrompt_FilterEnterOpensTheTopMatch(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive)
	m.loaded(TestChanges())

	m = Press(t, m, "f", "f", "i", "l", "e", "3", "enter")
	if m.selectedFile != 2 || m.focusedPanel != PanelDiffView {
		t.Errorf("enter opened file %d (focus %d), want file3.txt in the diff", m.selectedFile, m.focusedPanel)
	}
}

// A paste lands in the prompt that has the keys, keeping its newlines only where the prompt takes
// them.
func TestPrompt_PasteTypesIntoThePrompt(t *testing.T) {
	t.Parallel()

	m := Press(t, inColumns(t), "d")
	m = Update(t, m, tea.PasteMsg{Content: "fix: thé bug\r\n\r\nbody ✓"})
	m = Press(t, m, "enter")

	if got := m.cols.descs[1]; got != "fix: thé bug\n\nbody ✓" {
		t.Errorf("pasted description = %q", got)
	}

	f := NewTestModel(t, ModeInteractive)
	f.loaded(TestChanges())
	f = Press(t, f, "f")
	f = Update(t, f, tea.PasteMsg{Content: "file\n2"})

	if got := f.fileList.FilterQuery(); got != "file 2" {
		t.Errorf("a paste into the filter left %q, want one line", got)
	}
}

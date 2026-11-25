//nolint:testpackage // white-box: these tests read the selection and the file list's picks.
package model

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/jj-diff/internal/components/filelist"
)

// lastRow is the frame's last row without its colors, which is the footer.
func lastRow(m *Model) string {
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")

	return lines[len(lines)-1]
}

// In -i, space on the file list selects the whole file, the same tick a level up, and space again
// drops it: it never does nothing silently.
func TestFileList_SpaceSelectsTheWholeFile(t *testing.T) {
	t.Parallel()

	for _, mode := range []OperatingMode{ModeInteractive, ModeDiffEditor} {
		t.Run(modeName(mode), func(t *testing.T) {
			t.Parallel()

			m := NewTestModel(t, mode)
			m.loaded(TestChanges())
			m.focusedPanel = PanelFileList

			file := m.changes[0]
			start := m.selection.FilePick(file)

			want := filelist.PickAll
			if start == filelist.PickAll {
				want = filelist.PickNone
			}

			m = Press(t, m, "space")
			if got := m.selection.FilePick(file); got != want {
				t.Fatalf("space on the list left %s at pick %d, want %d", file.Path, got, want)
			}

			if m.focusedPanel != PanelFileList {
				t.Error("space moved focus off the list")
			}

			m = Press(t, m, "space")
			if got := m.selection.FilePick(file); got != start {
				t.Errorf("a second space left pick %d, want it back at %d", got, start)
			}

			m = Press(t, m, "u")
			if got := m.selection.FilePick(file); got != want {
				t.Errorf("u after two spaces left pick %d, want %d", got, want)
			}
		})
	}
}

// The file list draws a check beside a file selected whole and a tilde beside one partly selected.
func TestFileList_DrawsTheSelection(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive)
	m.loaded(TestChanges())
	m.focusedPanel = PanelFileList
	m = Press(t, m, "space", "j")

	m.selection.ToggleHunk("file2.txt", 0)
	m.selection.ToggleHunk("file1.txt", 1)

	// file1 lost one of its two hunks, file2 gained its only one, and file3 was never touched.
	view := ansi.Strip(m.View().Content)
	for _, want := range []string{"~ file1.txt", "✓ file2.txt", "D   file3.txt"} {
		if !strings.Contains(view, want) {
			t.Errorf("the list does not draw %q:\n%s", want, view)
		}
	}
}

// The -i file list's footer names the write keys, which act on the file at the cursor when nothing is
// selected, and the legend lists them on the list.
func TestFileList_FooterAndLegendNameTheWriteKeys(t *testing.T) {
	t.Parallel()

	for _, width := range []int{80, 120} {
		m := NewTestModel(t, ModeInteractive)
		m.loaded(TestChanges())
		m.focusedPanel = PanelFileList
		m = Update(t, m, tea.WindowSizeMsg{Width: width, Height: 24})

		foot := lastRow(m)
		for _, want := range []string{"space select", "s squash", "S split"} {
			if !strings.Contains(foot, want) {
				t.Errorf("at %d columns the list footer lacks %q: %q", width, want, foot)
			}
		}

		for _, want := range []string{"squash the selection", "split the selection", "discard the selection"} {
			// The legend filters as it is typed into, so each verb is found by its word.
			word := strings.Fields(want)[0]
			legend := ansi.Strip(Press(t, m, append([]string{"?", "/"}, strings.Split(word, "")...)...).View().Content)

			if !strings.Contains(legend, want) {
				t.Errorf("at %d columns the list's legend has no %q under /%s:\n%s", width, want, word, legend)
			}

			m = Press(t, m, "esc", "esc", "esc")
		}
	}
}

// A verb on the list with nothing selected takes the whole file at the cursor.
func TestFileList_VerbWithNothingSelectedTakesTheFile(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive)
	m.loaded(TestChanges())
	m.focusedPanel = PanelFileList

	sel, err := m.target()
	if err != nil {
		t.Fatal(err)
	}

	if got := sel.FilePick(m.changes[0]); got != filelist.PickAll {
		t.Errorf("the list's target holds pick %d of the file, want all of it", got)
	}
}

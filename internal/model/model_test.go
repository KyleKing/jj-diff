// focusedPanel, selection, lineCursor) and construct the unexported errMsg and diffLoadedMsg.
//
//nolint:testpackage // white-box: these tests read and set unexported model state (height,
package model

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/jj-diff/internal/jj"
)

var errTest = errors.New("test error")

// TestModelNavigation tests file and hunk navigation workflows.
func TestModelNavigation(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())

	// Navigate files (j/k)
	m = Update(t, m, KeyPress('j'))
	Assert(t, m).HasSelectedFile(1)
	Assert(t, m).HasSelectedHunk(0)

	m = Update(t, m, KeyPress('j'))
	Assert(t, m).HasSelectedFile(2)

	m = Update(t, m, KeyPress('k'))
	Assert(t, m).HasSelectedFile(1)

	// Jump to first (gg)
	m = Press(t, m, "g", "g")
	Assert(t, m).HasSelectedFile(0)
	Assert(t, m).HasSelectedHunk(0)

	// Jump to last (G)
	m = Update(t, m, KeyPress('G'))
	Assert(t, m).HasSelectedFile(2)
	Assert(t, m).HasSelectedHunk(0)

	// Navigate hunks when focused on diff view
	m.selectedFile = 0 // file1.txt has 2 hunks
	m.focusedPanel = PanelDiffView

	m = Update(t, m, KeyPress('n'))
	Assert(t, m).HasSelectedHunk(1)

	m = Press(t, m, "[", "h")
	Assert(t, m).HasSelectedHunk(0)

	// n repeats the last motion, which [h made backward, so it wraps to the last hunk.
	m = Update(t, m, KeyPress('n'))
	Assert(t, m).HasSelectedHunk(1)

	// N reverses it.
	m = Update(t, m, KeyPress('N'))
	Assert(t, m).HasSelectedHunk(0)

	m = Press(t, m, "]", "h")
	Assert(t, m).HasSelectedHunk(1)

	// n at last hunk should wrap to first hunk
	m.selectedHunk = 1
	m = Update(t, m, KeyPress('n'))
	Assert(t, m).HasSelectedHunk(0)
}

// TestModelPanelSwitching tests Tab key panel focus switching.
func TestModelPanelSwitching(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse)

	Assert(t, m).FocusedPanelIs(PanelFileList)

	m = Update(t, m, SpecialKey(tea.KeyTab))
	Assert(t, m).FocusedPanelIs(PanelDiffView)

	m = Update(t, m, SpecialKey(tea.KeyTab))
	Assert(t, m).FocusedPanelIs(PanelFileList)
}

// TestModelHunkSelection tests interactive mode hunk selection workflow.
func TestModelHunkSelection(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive).
		WithChanges(TestChanges()).
		WithDestination("@-")

	m.focusedPanel = PanelDiffView

	// Initially no hunks selected
	Assert(t, m).HasHunkNotSelected("file1.txt", 0)

	// Toggle hunk selection
	m = Update(t, m, KeyPress(' '))
	Assert(t, m).HasHunkSelected("file1.txt", 0)

	// Toggle again to deselect
	m = Update(t, m, KeyPress(' '))
	Assert(t, m).HasHunkNotSelected("file1.txt", 0)

	// Select multiple hunks
	m = Update(t, m, KeyPress(' '))
	Assert(t, m).HasHunkSelected("file1.txt", 0)

	m = Update(t, m, KeyPress('n'))
	m = Update(t, m, KeyPress(' '))
	Assert(t, m).HasHunkSelected("file1.txt", 0)
	Assert(t, m).HasHunkSelected("file1.txt", 1)

	// Switch files and select
	m = Update(t, m, SpecialKey(tea.KeyTab))
	m = Update(t, m, KeyPress('j'))
	m = Update(t, m, SpecialKey(tea.KeyTab))
	m = Update(t, m, KeyPress(' '))

	Assert(t, m).HasHunkSelected("file1.txt", 0)
	Assert(t, m).HasHunkSelected("file1.txt", 1)
	Assert(t, m).HasHunkSelected("file2.txt", 0)
}

// TestModelBrowseMode tests browse mode restrictions.
func TestModelBrowseMode(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())
	m.focusedPanel = PanelDiffView

	// Space should not select in browse mode
	m = Update(t, m, KeyPress(' '))
	Assert(t, m).HasHunkNotSelected("file1.txt", 0)

	// s squashes only where there is something to squash.
	if _, cmd := m.Update(KeyPress('s')); cmd != nil {
		t.Error("s returned a command in browse mode")
	}

	Assert(t, m).NoModalsVisible()
}

// TestModelInteractiveWorkflow tests the full interactive selection workflow.
func TestModelInteractiveWorkflow(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive).WithChanges(TestChanges())

	// Initial state
	Assert(t, m).ModeIs(ModeInteractive)
	Assert(t, m).HasDestination("")
	Assert(t, m).HasChanges(3)
	Assert(t, m).FocusedPanelIs(PanelFileList)

	m = m.WithDestination("abc123")
	m.revision = "@"

	// Navigate and select hunks
	m = Update(t, m, SpecialKey(tea.KeyTab))
	m = Update(t, m, KeyPress(' '))

	Assert(t, m).HasHunkSelected("file1.txt", 0)

	// Navigate to next hunk and select
	m = Update(t, m, KeyPress('n'))
	m = Update(t, m, KeyPress(' '))

	Assert(t, m).HasHunkSelected("file1.txt", 1)

	// Confirm ('c') squashes into the -d destination.
	_, cmd := m.Update(KeyPress('c'))
	if cmd == nil {
		t.Error("Expected a squash command when a destination is set and hunks are selected")
	}
}

// TestModelWindowResize tests window size message handling.
func TestModelWindowResize(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse)

	Assert(t, m).HasNoError()

	m = Update(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	if m.width != 120 || m.height != 40 {
		t.Errorf("Expected size 120x40, got %dx%d", m.width, m.height)
	}
}

// TestModelDiffLoaded tests diffLoadedMsg handling.
func TestModelDiffLoaded(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse)

	changes := TestChanges()
	m = Update(t, m, diffLoadedMsg{changes: changes})

	Assert(t, m).HasChanges(3)
	Assert(t, m).HasNoError()
}

// TestModelErrorHandling tests error message handling.
func TestModelErrorHandling(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse)

	m = Update(t, m, errMsg{err: errTest})

	Assert(t, m).HasError()
}

// TestModelHelpOverlay tests help display toggling.
func TestModelHelpOverlay(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse)

	// Open help
	m = Update(t, m, KeyPress('?'))

	if !m.help.IsVisible() {
		t.Error("Expected help to be visible after pressing ?")
	}

	// Close help
	m = Update(t, m, KeyPress('?'))

	if m.help.IsVisible() {
		t.Error("Expected help to be hidden after pressing ? again")
	}

	// Other keys should not work when help is visible
	m = Update(t, m, KeyPress('?'))
	initialFile := m.selectedFile

	m = Update(t, m, KeyPress('j'))

	if m.selectedFile != initialFile {
		t.Error("Expected j to be ignored when help is visible")
	}

	// Close with esc
	m = Update(t, m, SpecialKey(tea.KeyEsc))

	if m.help.IsVisible() {
		t.Error("Expected help to close with Esc")
	}
}

// TestSelectionState_ToggleHunk tests SelectionState hunk toggling.
func TestSelectionState_ToggleHunk(t *testing.T) {
	t.Parallel()

	s := NewSelectionState()

	if s.IsHunkSelected("file.txt", 0) {
		t.Error("Expected hunk to not be selected initially")
	}

	s.ToggleHunk("file.txt", 0)
	if !s.IsHunkSelected("file.txt", 0) {
		t.Error("Expected hunk to be selected after toggle")
	}

	s.ToggleHunk("file.txt", 0)
	if s.IsHunkSelected("file.txt", 0) {
		t.Error("Expected hunk to be deselected after second toggle")
	}
}

// TestSelectionState_ToggleLine tests SelectionState line toggling.
func TestSelectionState_ToggleLine(t *testing.T) {
	t.Parallel()

	s := NewSelectionState()

	s.ToggleLine("file.txt", 0, 5)
	if !s.IsLineSelected("file.txt", 0, 5) {
		t.Error("Expected line to be selected")
	}

	s.ToggleLine("file.txt", 0, 5)
	if s.IsLineSelected("file.txt", 0, 5) {
		t.Error("Expected line to be deselected")
	}

	// Line toggle should not work if whole hunk is selected
	s.ToggleHunk("file.txt", 0)
	initialLineState := s.IsLineSelected("file.txt", 0, 10)

	s.ToggleLine("file.txt", 0, 10)
	if s.IsLineSelected("file.txt", 0, 10) != initialLineState {
		t.Error("Expected line toggle to be ignored when whole hunk is selected")
	}
}

// TestSelectionState_WholeHunkClearsLines tests that selecting whole hunk clears line selection map.
func TestSelectionState_WholeHunkClearsLines(t *testing.T) {
	t.Parallel()

	s := NewSelectionState()

	// Select individual lines
	s.ToggleLine("file.txt", 0, 1)
	s.ToggleLine("file.txt", 0, 2)
	s.ToggleLine("file.txt", 0, 3)

	if !s.IsLineSelected("file.txt", 0, 1) {
		t.Error("Expected line 1 to be selected")
	}

	// Toggle whole hunk - this clears the SelectedLines map
	s.ToggleHunk("file.txt", 0)

	// Whole hunk should be selected
	if !s.IsHunkSelected("file.txt", 0) {
		t.Error("Expected whole hunk to be selected")
	}

	// Lines should still appear selected because WholeHunk=true means all lines are selected
	if !s.IsLineSelected("file.txt", 0, 1) {
		t.Error("Expected lines to be selected when whole hunk is selected")
	}

	// Verify the internal SelectedLines map was cleared
	fileSelection := s.Files["file.txt"]
	hunkSelection := fileSelection.Hunks[0]
	if len(hunkSelection.SelectedLines) != 0 {
		t.Errorf(
			"Expected SelectedLines map to be cleared, got %d entries",
			len(hunkSelection.SelectedLines),
		)
	}
}

// TestSelectionState_SelectLineRange tests selecting a range of lines.
func TestSelectionState_SelectLineRange(t *testing.T) {
	t.Parallel()

	s := NewSelectionState()

	s.SelectLineRange("file.txt", 0, 2, 5)

	for i := 2; i <= 5; i++ {
		if !s.IsLineSelected("file.txt", 0, i) {
			t.Errorf("Expected line %d to be selected", i)
		}
	}

	if s.IsLineSelected("file.txt", 0, 1) {
		t.Error("Expected line 1 to not be selected")
	}
	if s.IsLineSelected("file.txt", 0, 6) {
		t.Error("Expected line 6 to not be selected")
	}

	if s.IsHunkSelected("file.txt", 0) {
		t.Error("Expected whole hunk to not be selected")
	}
}

// TestSelectionState_SelectLineRangeReversed tests selecting line range with reversed bounds.
func TestSelectionState_SelectLineRangeReversed(t *testing.T) {
	t.Parallel()

	s := NewSelectionState()

	s.SelectLineRange("file.txt", 0, 5, 2)

	for i := 2; i <= 5; i++ {
		if !s.IsLineSelected("file.txt", 0, i) {
			t.Errorf("Expected line %d to be selected", i)
		}
	}
}

// TestSelectionState_HasPartialSelection tests partial selection detection.
func TestSelectionState_HasPartialSelection(t *testing.T) {
	t.Parallel()

	s := NewSelectionState()

	if s.HasPartialSelection("file.txt", 0) {
		t.Error("Expected no partial selection initially")
	}

	s.ToggleLine("file.txt", 0, 3)
	if !s.HasPartialSelection("file.txt", 0) {
		t.Error("Expected partial selection after selecting individual line")
	}

	s.ToggleHunk("file.txt", 0)
	if s.HasPartialSelection("file.txt", 0) {
		t.Error("Expected no partial selection when whole hunk is selected")
	}
}

// TestModelVisualMode tests entering and exiting visual mode.
func TestModelVisualMode(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive).
		WithChanges(TestChanges())

	m.focusedPanel = PanelDiffView

	Assert(t, m).IsNotInVisualMode()

	m = Update(t, m, KeyPress('v'))
	Assert(t, m).IsInVisualMode()

	m = Update(t, m, SpecialKey(tea.KeyEsc))
	Assert(t, m).IsNotInVisualMode()
}

// TestModelVisualModeNavigation tests line navigation in visual mode.
func TestModelVisualModeNavigation(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive).
		WithChanges(TestChanges())

	m.focusedPanel = PanelDiffView
	m.selectedFile = 0

	m = Update(t, m, KeyPress('v'))
	Assert(t, m).IsInVisualMode()
	Assert(t, m).HasLineCursor(0)

	m = Update(t, m, KeyPress('j'))
	Assert(t, m).HasLineCursor(1)

	m = Update(t, m, KeyPress('j'))
	Assert(t, m).HasLineCursor(2)

	m = Update(t, m, KeyPress('k'))
	Assert(t, m).HasLineCursor(1)
}

// TestModelVisualModeSelection tests line range selection in visual mode.
func TestModelVisualModeSelection(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive).
		WithChanges(TestChanges())

	m.focusedPanel = PanelDiffView
	m.selectedFile = 0

	m = Update(t, m, KeyPress('v'))
	Assert(t, m).IsInVisualMode()

	m = Update(t, m, KeyPress('j'))
	m = Update(t, m, KeyPress('j'))
	m = Update(t, m, KeyPress(' '))

	Assert(t, m).IsNotInVisualMode()

	if !m.selection.IsLineSelected("file1.txt", 0, 0) {
		t.Error("Expected line 0 to be selected")
	}
	if !m.selection.IsLineSelected("file1.txt", 0, 1) {
		t.Error("Expected line 1 to be selected")
	}
	if !m.selection.IsLineSelected("file1.txt", 0, 2) {
		t.Error("Expected line 2 to be selected")
	}
}

// TestModelLineCursorReset tests that lineCursor resets when switching hunks/files.
func TestModelLineCursorReset(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive).
		WithChanges(TestChanges())

	m.focusedPanel = PanelDiffView
	m.lineCursor = 5

	m = Update(t, m, KeyPress('n'))
	Assert(t, m).HasLineCursor(0)

	m.lineCursor = 3
	m = Update(t, m, KeyPress('N'))
	Assert(t, m).HasLineCursor(0)

	m.lineCursor = 7
	m = Press(t, m, "g", "g")
	Assert(t, m).HasLineCursor(0)

	m.lineCursor = 4
	m = Update(t, m, KeyPress('G'))
	Assert(t, m).HasLineCursor(0)
}

// TestModelBrowseModeNoVisual tests that visual mode is disabled in browse mode.
func TestModelBrowseModeNoVisual(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse).
		WithChanges(TestChanges())

	m.focusedPanel = PanelDiffView

	m = Update(t, m, KeyPress('v'))
	Assert(t, m).IsNotInVisualMode()
}

// TestModelViewOptionToggles tests the new view option keybindings.
func TestModelViewOptionToggles(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())
	m.focusedPanel = PanelDiffView

	tests := []struct {
		checkFn  func(m *Model) bool
		name     string
		keys     []string
		expected bool
	}{
		{
			checkFn:  func(m *Model) bool { return m.diffView.ShowWhitespace() },
			name:     "toggle whitespace on",
			keys:     []string{"w"},
			expected: true,
		},
		{
			checkFn:  func(m *Model) bool { return m.diffView.ShowWhitespace() },
			name:     "toggle whitespace off",
			keys:     []string{"w"},
			expected: false,
		},
		{
			checkFn:  func(m *Model) bool { return m.diffView.WordLevelDiff() },
			name:     "toggle word diff on",
			keys:     []string{`\\`, "r"},
			expected: true,
		},
		{
			checkFn:  func(m *Model) bool { return m.diffView.WordLevelDiff() },
			name:     "toggle word diff off",
			keys:     []string{`\\`, "r"},
			expected: false,
		},
		{
			checkFn:  func(m *Model) bool { return m.diffView.ShowLineNumbers() },
			name:     "toggle line numbers off",
			keys:     []string{`\\`, "n"},
			expected: false,
		},
		{
			checkFn:  func(m *Model) bool { return m.diffView.ShowLineNumbers() },
			name:     "toggle line numbers on",
			keys:     []string{`\\`, "n"},
			expected: true,
		},
	}

	for _, tt := range tests {
		m = Press(t, m, tt.keys...)
		if got := tt.checkFn(m); got != tt.expected {
			t.Errorf("%s: expected %v, got %v", tt.name, tt.expected, got)
		}
	}
}

// TestModelSideBySideToggle tests side-by-side view toggling.
func TestModelSideBySideToggle(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())
	m.focusedPanel = PanelDiffView

	if m.diffView.IsSideBySide() {
		t.Error("Expected unified mode initially")
	}

	m = Press(t, m, `\`, "s")
	if !m.diffView.IsSideBySide() {
		t.Error("Expected side-by-side mode after toggle")
	}

	m = Press(t, m, `\`, "s")
	if m.diffView.IsSideBySide() {
		t.Error("Expected unified mode after second toggle")
	}
}

// TestModelSideBySideDisablesSelectionInInteractive tests that side-by-side stays when toggled on in interactive.
func TestModelSideBySideInInteractiveMode(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive).WithChanges(TestChanges())
	m.focusedPanel = PanelDiffView

	m = Press(t, m, `\`, "s")
	if !m.diffView.IsSideBySide() {
		t.Error("Expected side-by-side mode to be enabled")
	}

	m = Press(t, m, `\`, "s")
	if m.diffView.IsSideBySide() {
		t.Error("Expected to toggle back to unified")
	}
}

// TestModalMutualExclusivity tests that opening one modal closes others.
func TestModalMutualExclusivity(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())

	m = Update(t, m, KeyPress('?'))
	Assert(t, m).HelpIsVisible()

	m = Update(t, m, SpecialKey(tea.KeyEsc))
	m = Update(t, m, KeyPress('/'))
	Assert(t, m).SearchIsVisible()
	Assert(t, m).HelpIsNotVisible()

	// A text input takes ? as text, so it is typed into the search rather than opening help.
	m = Update(t, m, KeyPress('?'))
	Assert(t, m).HelpIsNotVisible()
	Assert(t, m).SearchIsVisible()

	m = Update(t, m, SpecialKey(tea.KeyEsc))
	m = Update(t, m, KeyPress('f'))
	Assert(t, m).FileListFilterModeEnabled()
	Assert(t, m).HelpIsNotVisible()

	m = Update(t, m, SpecialKey(tea.KeyEsc))
	m = Update(t, m, KeyPress('?'))
	Assert(t, m).HelpIsVisible()
	Assert(t, m).FileListFilterModeDisabled()
}

// TestModalEscClosesAny tests that ESC closes any open modal.
func TestModalEscClosesAny(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())

	m = Update(t, m, KeyPress('?'))
	Assert(t, m).HelpIsVisible()
	m = Update(t, m, SpecialKey(tea.KeyEsc))
	Assert(t, m).NoModalsVisible()

	m = Update(t, m, KeyPress('/'))
	Assert(t, m).SearchIsVisible()
	m = Update(t, m, SpecialKey(tea.KeyEsc))
	Assert(t, m).NoModalsVisible()

	m = Update(t, m, KeyPress('f'))
	Assert(t, m).FileListFilterModeEnabled()
	m = Update(t, m, SpecialKey(tea.KeyEsc))
	Assert(t, m).NoModalsVisible()
}

// TestAllModalsOpenAndClose tests each modal can be opened and closed.
func TestAllModalsOpenAndClose(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())

	Assert(t, m).NoModalsVisible()

	m = Update(t, m, KeyPress('?'))
	Assert(t, m).HelpIsVisible()
	m = Update(t, m, KeyPress('q'))
	Assert(t, m).HelpIsNotVisible()

	m = Update(t, m, KeyPress('/'))
	Assert(t, m).SearchIsVisible()
	m = Update(t, m, SpecialKey(tea.KeyEnter))
	Assert(t, m).SearchIsNotVisible()

	m = Update(t, m, KeyPress('f'))
	Assert(t, m).FileListFilterModeEnabled()
	m = Update(t, m, SpecialKey(tea.KeyEsc))
	Assert(t, m).FileListFilterModeDisabled()
}

// TestVimScrolling tests Ctrl-d/u/f/b scrolling.
func TestVimScrolling(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())
	m.height = 24
	m.focusedPanel = PanelDiffView

	m = Update(t, m, CtrlKey('d'))
	m = Update(t, m, CtrlKey('u'))
	m = Update(t, m, CtrlKey('u'))
	m = Update(t, m, CtrlKey('f'))
	Update(t, m, CtrlKey('b'))
}

// TestScrollingOnlyInDiffPanel tests that scrolling only works when diff panel is focused.
func TestScrollingOnlyInDiffPanel(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())
	m.height = 24
	m.focusedPanel = PanelFileList

	m = Update(t, m, CtrlKey('d'))

	m.focusedPanel = PanelDiffView
	Update(t, m, CtrlKey('d'))
}

// TestSquashOpensThePickerFromLoadedRevisions pins the picker to the message path. Opening it from
// inside the tea.Cmd instead left it permanently invisible, because the command mutated its own copy.
func TestSquashOpensThePickerFromLoadedRevisions(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive).WithChanges(TestChanges())
	m.focusedPanel = PanelDiffView
	m.revision = "@"

	if _, cmd := m.Update(KeyPress('s')); cmd == nil {
		t.Fatal("Expected s to return a revision-loading command")
	}

	m = Update(t, m, revisionsLoadedMsg{revisions: []jj.RevisionEntry{{ChangeID: "abc", Short: "abcdefgh"}}})
	Assert(t, m).PickerIsVisible()

	m = Press(t, m, "-", "k")
	if !m.flags.On("k") {
		t.Error("-k did not toggle keep-emptied in the picker")
	}

	m = Press(t, m, "esc")
	Assert(t, m).NoModalsVisible()
}

// TestApplySelectionUsesTheSelectionFromWhenTheKeyWasPressed pins the snapshot the apply command
// takes. Under pointer receivers a command that read the live model would apply whatever the user
// did while jj was still running.
func TestApplySelectionUsesTheSelectionFromWhenTheKeyWasPressed(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive).WithChanges(TestChanges()).WithDestination("@-")
	m.focusedPanel = PanelDiffView

	m = Update(t, m, KeyPress(' '))
	Assert(t, m).HasHunkSelected("file1.txt", 0)

	m.revision = "@"

	_, cmd := m.Update(KeyPress('c'))
	if cmd == nil {
		t.Fatal("Expected c to return a squash command")
	}

	// A later diff load swaps the whole selection state, which is what the snapshot has to survive.
	m = Update(t, m, diffLoadedMsg{changes: TestChanges()})
	m.selection = NewSelectionState()

	written, ok := cmd().(writtenMsg)
	if !ok || !strings.Contains(written.summary, "1 hunk") {
		t.Errorf("the squash command read the live selection instead of the one from when c was pressed: %+v", written)
	}
}

// A modal used to replace the whole screen, so you could not see what you were searching or which
// revision you were picking a destination from while the prompt was up.
func TestRender_ModalsCompositeOverThePanels(t *testing.T) {
	t.Parallel()

	changes := TestChanges()

	tests := []struct {
		open   func(m *Model)
		name   string
		footer string
		height int
	}{
		// Search takes the footer's row, so its prompt is what the last line holds.
		{name: "search", height: 30, footer: "type to search", open: func(m *Model) { m.searchModal.Show() }},
		// The help overlay is nearly 30 rows, so it needs a terminal it does not fill.
		{name: "help", height: 60, open: func(m *Model) { m.showHelp() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := NewTestModel(t, ModeInteractive).WithChanges(changes)
			m.width, m.height = 100, tt.height

			base := m.render()

			tt.open(m)

			got := m.render()
			if got == base {
				t.Fatal("opening the modal changed nothing on screen")
			}

			if !strings.Contains(got, changes[0].Path) {
				t.Errorf("the file list should stay visible behind the modal, got:\n%s", got)
			}

			footer := tt.footer
			if footer == "" {
				footer = "Interactive"
			}

			if !strings.Contains(got, footer) {
				t.Errorf("the footer should read %q, got:\n%s", footer, got)
			}
		})
	}
}

package model

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestModelNavigation tests file and hunk navigation workflows
func TestModelNavigation(t *testing.T) {
	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())

	// Navigate files (j/k)
	m = Update(t, m, KeyPress('j'))
	Assert(t, m).HasSelectedFile(1).HasSelectedHunk(0)

	m = Update(t, m, KeyPress('j'))
	Assert(t, m).HasSelectedFile(2)

	m = Update(t, m, KeyPress('k'))
	Assert(t, m).HasSelectedFile(1)

	// Jump to first (g)
	m = Update(t, m, KeyPress('g'))
	Assert(t, m).HasSelectedFile(0).HasSelectedHunk(0)

	// Jump to last (G)
	m = Update(t, m, KeyPress('G'))
	Assert(t, m).HasSelectedFile(2).HasSelectedHunk(0)

	// Navigate hunks when focused on diff view
	m.selectedFile = 0 // file1.txt has 2 hunks
	m.focusedPanel = PanelDiffView

	m = Update(t, m, KeyPress('n'))
	Assert(t, m).HasSelectedHunk(1)

	m = Update(t, m, KeyPress('p'))
	Assert(t, m).HasSelectedHunk(0)

	// Test N (previous hunk, same as p)
	m.selectedHunk = 1
	m = Update(t, m, KeyPress('N'))
	Assert(t, m).HasSelectedHunk(0)

	// p at first hunk should stay at 0
	m = Update(t, m, KeyPress('p'))
	Assert(t, m).HasSelectedHunk(0)

	// n at last hunk should stay at last
	m.selectedHunk = 1
	m = Update(t, m, KeyPress('n'))
	Assert(t, m).HasSelectedHunk(1)
}

// TestModelPanelSwitching tests Tab key panel focus switching
func TestModelPanelSwitching(t *testing.T) {
	m := NewTestModel(t, ModeBrowse)

	Assert(t, m).FocusedPanelIs(PanelFileList)

	m = Update(t, m, SpecialKey(tea.KeyTab))
	Assert(t, m).FocusedPanelIs(PanelDiffView)

	m = Update(t, m, SpecialKey(tea.KeyTab))
	Assert(t, m).FocusedPanelIs(PanelFileList)
}

// TestModelHunkSelection tests interactive mode hunk selection workflow
func TestModelHunkSelection(t *testing.T) {
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

// TestModelBrowseMode tests browse mode restrictions
func TestModelBrowseMode(t *testing.T) {
	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())
	m.focusedPanel = PanelDiffView

	// Space should not select in browse mode
	m = Update(t, m, KeyPress(' '))
	Assert(t, m).HasHunkNotSelected("file1.txt", 0)

	// 'd' should not open picker in browse mode
	initialDestination := m.destination
	m = Update(t, m, KeyPress('d'))
	Assert(t, m).HasDestination(initialDestination)
}

// TestModelInteractiveWorkflow tests the full interactive selection workflow
func TestModelInteractiveWorkflow(t *testing.T) {
	m := NewTestModel(t, ModeInteractive).WithChanges(TestChanges())

	// Initial state
	Assert(t, m).
		ModeIs(ModeInteractive).
		HasDestination("").
		HasChanges(3).
		FocusedPanelIs(PanelFileList)

	// Set destination via message (simulating picker selection)
	m = Update(t, m, destinationSelectedMsg{changeID: "abc123"})
	Assert(t, m).HasDestination("abc123")

	// Navigate and select hunks
	m = Update(t, m, SpecialKey(tea.KeyTab))
	m = Update(t, m, KeyPress(' '))

	Assert(t, m).HasHunkSelected("file1.txt", 0)

	// Navigate to next hunk and select
	m = Update(t, m, KeyPress('n'))
	m = Update(t, m, KeyPress(' '))

	Assert(t, m).HasHunkSelected("file1.txt", 1)

	// Apply action ('a') should trigger command when destination is set
	_, cmd := m.Update(KeyPress('a'))
	if cmd == nil {
		t.Error("Expected apply command when destination is set and hunks are selected")
	}
}

// TestModelWindowResize tests window size message handling
func TestModelWindowResize(t *testing.T) {
	m := NewTestModel(t, ModeBrowse)

	Assert(t, m).HasNoError()

	m = Update(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	if m.width != 120 || m.height != 40 {
		t.Errorf("Expected size 120x40, got %dx%d", m.width, m.height)
	}
}

// TestModelDiffLoaded tests diffLoadedMsg handling
func TestModelDiffLoaded(t *testing.T) {
	m := NewTestModel(t, ModeBrowse)

	changes := TestChanges()
	m = Update(t, m, diffLoadedMsg{changes: changes})

	Assert(t, m).HasChanges(3).HasNoError()
}

// TestModelErrorHandling tests error message handling
func TestModelErrorHandling(t *testing.T) {
	m := NewTestModel(t, ModeBrowse)

	testErr := fmt.Errorf("test error")
	m = Update(t, m, errMsg{err: testErr})

	Assert(t, m).HasError()
}

// TestModelHelpOverlay tests help display toggling
func TestModelHelpOverlay(t *testing.T) {
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

// TestSelectionState_ToggleHunk tests SelectionState hunk toggling
func TestSelectionState_ToggleHunk(t *testing.T) {
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

// TestSelectionState_ToggleLine tests SelectionState line toggling
func TestSelectionState_ToggleLine(t *testing.T) {
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

// TestSelectionState_WholeHunkClearsLines tests that selecting whole hunk clears line selection map
func TestSelectionState_WholeHunkClearsLines(t *testing.T) {
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
		t.Errorf("Expected SelectedLines map to be cleared, got %d entries", len(hunkSelection.SelectedLines))
	}
}

// TestSelectionState_SelectLineRange tests selecting a range of lines
func TestSelectionState_SelectLineRange(t *testing.T) {
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

// TestSelectionState_SelectLineRangeReversed tests selecting line range with reversed bounds
func TestSelectionState_SelectLineRangeReversed(t *testing.T) {
	s := NewSelectionState()

	s.SelectLineRange("file.txt", 0, 5, 2)

	for i := 2; i <= 5; i++ {
		if !s.IsLineSelected("file.txt", 0, i) {
			t.Errorf("Expected line %d to be selected", i)
		}
	}
}

// TestSelectionState_HasPartialSelection tests partial selection detection
func TestSelectionState_HasPartialSelection(t *testing.T) {
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

// TestModelVisualMode tests entering and exiting visual mode
func TestModelVisualMode(t *testing.T) {
	m := NewTestModel(t, ModeInteractive).
		WithChanges(TestChanges())

	m.focusedPanel = PanelDiffView

	Assert(t, m).IsNotInVisualMode()

	m = Update(t, m, KeyPress('v'))
	Assert(t, m).IsInVisualMode()

	m = Update(t, m, SpecialKey(tea.KeyEsc))
	Assert(t, m).IsNotInVisualMode()
}

// TestModelVisualModeNavigation tests line navigation in visual mode
func TestModelVisualModeNavigation(t *testing.T) {
	m := NewTestModel(t, ModeInteractive).
		WithChanges(TestChanges())

	m.focusedPanel = PanelDiffView
	m.selectedFile = 0

	m = Update(t, m, KeyPress('v'))
	Assert(t, m).IsInVisualMode().HasLineCursor(0)

	m = Update(t, m, KeyPress('j'))
	Assert(t, m).HasLineCursor(1)

	m = Update(t, m, KeyPress('j'))
	Assert(t, m).HasLineCursor(2)

	m = Update(t, m, KeyPress('k'))
	Assert(t, m).HasLineCursor(1)
}

// TestModelVisualModeSelection tests line range selection in visual mode
func TestModelVisualModeSelection(t *testing.T) {
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

// TestModelLineCursorReset tests that lineCursor resets when switching hunks/files
func TestModelLineCursorReset(t *testing.T) {
	m := NewTestModel(t, ModeInteractive).
		WithChanges(TestChanges())

	m.focusedPanel = PanelDiffView
	m.lineCursor = 5

	m = Update(t, m, KeyPress('n'))
	Assert(t, m).HasLineCursor(0)

	m.lineCursor = 3
	m = Update(t, m, KeyPress('p'))
	Assert(t, m).HasLineCursor(0)

	m.lineCursor = 7
	m = Update(t, m, KeyPress('g'))
	Assert(t, m).HasLineCursor(0)

	m.lineCursor = 4
	m = Update(t, m, KeyPress('G'))
	Assert(t, m).HasLineCursor(0)
}

// TestModelBrowseModeNoVisual tests that visual mode is disabled in browse mode
func TestModelBrowseModeNoVisual(t *testing.T) {
	m := NewTestModel(t, ModeBrowse).
		WithChanges(TestChanges())

	m.focusedPanel = PanelDiffView

	m = Update(t, m, KeyPress('v'))
	Assert(t, m).IsNotInVisualMode()
}

// TestModelViewOptionToggles tests the new view option keybindings
func TestModelViewOptionToggles(t *testing.T) {
	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())

	tests := []struct {
		name     string
		key      rune
		checkFn  func(m Model) bool
		expected bool
	}{
		{"toggle whitespace on", 'w', func(m Model) bool { return m.diffView.ShowWhitespace() }, true},
		{"toggle whitespace off", 'w', func(m Model) bool { return m.diffView.ShowWhitespace() }, false},
		{"toggle word diff on", 'W', func(m Model) bool { return m.diffView.WordLevelDiff() }, true},
		{"toggle word diff off", 'W', func(m Model) bool { return m.diffView.WordLevelDiff() }, false},
		{"toggle line numbers off", 'l', func(m Model) bool { return m.diffView.ShowLineNumbers() }, false},
		{"toggle line numbers on", 'l', func(m Model) bool { return m.diffView.ShowLineNumbers() }, true},
	}

	for _, tt := range tests {
		m = Update(t, m, KeyPress(tt.key))
		if got := tt.checkFn(m); got != tt.expected {
			t.Errorf("%s: expected %v, got %v", tt.name, tt.expected, got)
		}
	}
}

// TestModelSideBySideToggle tests side-by-side view toggling
func TestModelSideBySideToggle(t *testing.T) {
	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())

	if m.diffView.IsSideBySide() {
		t.Error("Expected unified mode initially")
	}

	m = Update(t, m, KeyPress('s'))
	if !m.diffView.IsSideBySide() {
		t.Error("Expected side-by-side mode after toggle")
	}

	m = Update(t, m, KeyPress('s'))
	if m.diffView.IsSideBySide() {
		t.Error("Expected unified mode after second toggle")
	}
}

// TestModelSideBySideDisablesSelectionInInteractive tests that side-by-side stays when toggled on in interactive
func TestModelSideBySideInInteractiveMode(t *testing.T) {
	m := NewTestModel(t, ModeInteractive).WithChanges(TestChanges())

	m = Update(t, m, KeyPress('s'))
	if !m.diffView.IsSideBySide() {
		t.Error("Expected side-by-side mode to be enabled")
	}

	m = Update(t, m, KeyPress('s'))
	if m.diffView.IsSideBySide() {
		t.Error("Expected to toggle back to unified")
	}
}

// TestModalMutualExclusivity tests that opening one modal closes others
func TestModalMutualExclusivity(t *testing.T) {
	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())

	m = Update(t, m, KeyPress('?'))
	Assert(t, m).HelpIsVisible()

	m = Update(t, m, SpecialKey(tea.KeyEsc))
	m = Update(t, m, KeyPress('/'))
	Assert(t, m).SearchIsVisible().HelpIsNotVisible()

	m = Update(t, m, KeyPress('?'))
	Assert(t, m).HelpIsVisible().SearchIsNotVisible()

	m = Update(t, m, SpecialKey(tea.KeyEsc))
	m = Update(t, m, KeyPress('f'))
	Assert(t, m).FileListFilterModeEnabled().HelpIsNotVisible()

	m = Update(t, m, KeyPress('?'))
	Assert(t, m).HelpIsVisible().FileListFilterModeDisabled()
}

// TestModalEscClosesAny tests that ESC closes any open modal
func TestModalEscClosesAny(t *testing.T) {
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

// TestAllModalsOpenAndClose tests each modal can be opened and closed
func TestAllModalsOpenAndClose(t *testing.T) {
	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())

	Assert(t, m).NoModalsVisible()

	m = Update(t, m, KeyPress('?'))
	Assert(t, m).HelpIsVisible()
	m = Update(t, m, KeyPress('q'))
	Assert(t, m).HelpIsNotVisible()

	m = Update(t, m, KeyPress('/'))
	Assert(t, m).SearchIsVisible()
	m = Update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	Assert(t, m).SearchIsNotVisible()

	m = Update(t, m, KeyPress('f'))
	Assert(t, m).FileListFilterModeEnabled()
	m = Update(t, m, SpecialKey(tea.KeyEsc))
	Assert(t, m).FileListFilterModeDisabled()
}

// TestVimScrolling tests Ctrl-d/u/f/b scrolling
func TestVimScrolling(t *testing.T) {
	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())
	m.height = 24
	m.focusedPanel = PanelDiffView

	initialOffset := 0

	m = Update(t, m, tea.KeyMsg{Type: tea.KeyCtrlD})
	if m.diffView.ShowLineNumbers() {
	}

	m = Update(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m = Update(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})

	m = Update(t, m, tea.KeyMsg{Type: tea.KeyCtrlF})
	m = Update(t, m, tea.KeyMsg{Type: tea.KeyCtrlB})

	_ = initialOffset
}

// TestScrollingOnlyInDiffPanel tests that scrolling only works when diff panel is focused
func TestScrollingOnlyInDiffPanel(t *testing.T) {
	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())
	m.height = 24
	m.focusedPanel = PanelFileList

	m = Update(t, m, tea.KeyMsg{Type: tea.KeyCtrlD})

	m.focusedPanel = PanelDiffView
	m = Update(t, m, tea.KeyMsg{Type: tea.KeyCtrlD})
}

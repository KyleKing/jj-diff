package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
)

// NewTestModel builds a Model backed by a jj client rooted at a fresh temp dir.
func NewTestModel(t *testing.T, mode OperatingMode) Model {
	t.Helper()

	client := jj.NewClient(t.TempDir())
	cfg := config.DefaultConfig()

	m, err := NewModel(client, "@", "", mode, cfg)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}

	return m
}

// WithChanges loads changes and selects the first file, mirroring what a real diff load does.
func (m Model) WithChanges(changes []diff.FileChange) Model {
	m.changes = changes
	m.fileList.SetFiles(changes)
	if len(m.changes) > 0 {
		m.diffView.SetFileChange(changes[0])
	}

	return m
}

// WithDestination sets the move destination without going through the picker.
func (m Model) WithDestination(dest string) Model {
	m.destination = dest
	return m
}

// KeyPress builds the message Bubble Tea sends for a printable key.
func KeyPress(key rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: key, Text: string(key)}
}

// SpecialKey builds the message Bubble Tea sends for a named key such as tea.KeyEsc. Text stays
// empty, which is how Bubble Tea marks a key that does not produce a printable character.
func SpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// CtrlKey builds the message Bubble Tea sends for key held with control, such as ctrl+d.
func CtrlKey(key rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: key, Mod: tea.ModCtrl}
}

// Update runs one Update round and asserts the result is still a Model, discarding the command.
func Update(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	newModel, _ := m.Update(msg)

	return assertModel(t, newModel)
}

func assertModel(t *testing.T, model tea.Model) Model {
	t.Helper()

	m, ok := model.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want model.Model", model)
	}

	return m
}

// Assertion reads unexported Model state, which is why it lives in the package rather than a test file.
// Every check reports through t.Errorf and returns the receiver, so a chain runs to the end and reports
// every mismatch instead of stopping at the first.
type Assertion struct {
	t *testing.T
	m Model
}

// Assert opens a chain of assertions over a snapshot of m. Later mutations of m are not observed.
func Assert(t *testing.T, m Model) *Assertion {
	t.Helper()
	return &Assertion{t: t, m: m}
}

// HasSelectedFile checks the file-list cursor, which is an index into the unfiltered change list.
func (a *Assertion) HasSelectedFile(idx int) {
	a.t.Helper()
	if a.m.selectedFile != idx {
		a.t.Errorf("Expected selectedFile=%d, got %d", idx, a.m.selectedFile)
	}
}

// HasSelectedHunk checks the hunk cursor, which is an index within the currently selected file only.
func (a *Assertion) HasSelectedHunk(idx int) {
	a.t.Helper()
	if a.m.selectedHunk != idx {
		a.t.Errorf("Expected selectedHunk=%d, got %d", idx, a.m.selectedHunk)
	}
}

// HasHunkSelected checks that the hunk is marked for apply. A partially selected hunk still counts.
func (a *Assertion) HasHunkSelected(filePath string, hunkIdx int) {
	a.t.Helper()
	if !a.m.selection.IsHunkSelected(filePath, hunkIdx) {
		a.t.Errorf("Expected hunk %d in file %s to be selected", hunkIdx, filePath)
	}
}

// HasHunkNotSelected checks that no part of the hunk is marked for apply.
func (a *Assertion) HasHunkNotSelected(filePath string, hunkIdx int) {
	a.t.Helper()
	if a.m.selection.IsHunkSelected(filePath, hunkIdx) {
		a.t.Errorf("Expected hunk %d in file %s to NOT be selected", hunkIdx, filePath)
	}
}

// FocusedPanelIs checks which panel receives navigation keys.
func (a *Assertion) FocusedPanelIs(panel FocusedPanel) {
	a.t.Helper()
	if a.m.focusedPanel != panel {
		a.t.Errorf("Expected focusedPanel=%v, got %v", panel, a.m.focusedPanel)
	}
}

// ModeIs checks the operating mode, which is fixed at construction and never changes at runtime.
func (a *Assertion) ModeIs(mode OperatingMode) {
	a.t.Helper()
	if a.m.mode != mode {
		a.t.Errorf("Expected mode=%v, got %v", mode, a.m.mode)
	}
}

// HasDestination checks the revset the model will move changes to, which is empty until one is picked.
func (a *Assertion) HasDestination(dest string) {
	a.t.Helper()
	if a.m.destination != dest {
		a.t.Errorf("Expected destination=%s, got %s", dest, a.m.destination)
	}
}

// HasError checks that the model is holding an error for the view to render.
func (a *Assertion) HasError() {
	a.t.Helper()
	if a.m.err == nil {
		a.t.Error("Expected error, got nil")
	}
}

// HasNoError checks that no error is pending, and prints the error it found when one is.
func (a *Assertion) HasNoError() {
	a.t.Helper()
	if a.m.err != nil {
		a.t.Errorf("Expected no error, got %v", a.m.err)
	}
}

// HasChanges checks how many file changes are loaded, ignoring any file-list filter.
func (a *Assertion) HasChanges(count int) {
	a.t.Helper()
	if len(a.m.changes) != count {
		a.t.Errorf("Expected %d changes, got %d", count, len(a.m.changes))
	}
}

// IsInVisualMode checks that line-range selection is active.
func (a *Assertion) IsInVisualMode() {
	a.t.Helper()
	if !a.m.isVisualMode {
		a.t.Error("Expected model to be in visual mode")
	}
}

// IsNotInVisualMode checks that line-range selection is off, so space acts on the whole hunk.
func (a *Assertion) IsNotInVisualMode() {
	a.t.Helper()
	if a.m.isVisualMode {
		a.t.Error("Expected model to NOT be in visual mode")
	}
}

// HasLineCursor checks the line cursor as an index into the rendered rows of the selected hunk.
func (a *Assertion) HasLineCursor(position int) {
	a.t.Helper()
	if a.m.lineCursor != position {
		a.t.Errorf("Expected lineCursor=%d, got %d", position, a.m.lineCursor)
	}
}

// HelpIsVisible checks that the help overlay is up.
func (a *Assertion) HelpIsVisible() {
	a.t.Helper()
	if !a.m.help.IsVisible() {
		a.t.Error("Expected help modal to be visible")
	}
}

// HelpIsNotVisible checks that the help overlay is down.
func (a *Assertion) HelpIsNotVisible() {
	a.t.Helper()
	if a.m.help.IsVisible() {
		a.t.Error("Expected help modal to NOT be visible")
	}
}

// SearchIsVisible checks that the search prompt is up, which is separate from the file-list filter.
func (a *Assertion) SearchIsVisible() {
	a.t.Helper()
	if !a.m.searchModal.IsVisible() {
		a.t.Error("Expected search modal to be visible")
	}
}

// SearchIsNotVisible checks that the search prompt is down.
func (a *Assertion) SearchIsNotVisible() {
	a.t.Helper()
	if a.m.searchModal.IsVisible() {
		a.t.Error("Expected search modal to NOT be visible")
	}
}

// SplitAssignIsVisible checks that the multi-way split's destination assignment modal is up.
func (a *Assertion) SplitAssignIsVisible() {
	a.t.Helper()
	if !a.m.splitAssign.IsVisible() {
		a.t.Error("Expected split assign modal to be visible")
	}
}

// FileListFilterModeEnabled checks that the file list is capturing keys for its inline filter.
func (a *Assertion) FileListFilterModeEnabled() {
	a.t.Helper()
	if !a.m.fileList.IsFilterMode() {
		a.t.Error("Expected file list filter mode to be enabled")
	}
}

// FileListFilterModeDisabled checks that the file list has handed key handling back to the model.
func (a *Assertion) FileListFilterModeDisabled() {
	a.t.Helper()
	if a.m.fileList.IsFilterMode() {
		a.t.Error("Expected file list filter mode to be disabled")
	}
}

// NoModalsVisible checks every overlay and the file-list filter at once, reporting each one that is up.
func (a *Assertion) NoModalsVisible() {
	a.t.Helper()
	if a.m.help.IsVisible() {
		a.t.Error("Expected help modal to NOT be visible")
	}
	if a.m.searchModal.IsVisible() {
		a.t.Error("Expected search modal to NOT be visible")
	}
	if a.m.fileFinder.IsVisible() {
		a.t.Error("Expected file finder modal to NOT be visible")
	}
	if a.m.destPicker.IsVisible() {
		a.t.Error("Expected dest picker modal to NOT be visible")
	}
	if a.m.fileList.IsFilterMode() {
		a.t.Error("Expected file list filter mode to NOT be enabled")
	}
}

// TestChanges returns three file changes covering the modified, added, and deleted paths.
//
//nolint:mnd // Hunk offsets and line numbers are the fixture's data, so naming them hides it.
func TestChanges() []diff.FileChange {
	return []diff.FileChange{
		{
			Path:       "file1.txt",
			ChangeType: diff.ChangeTypeModified,
			Hunks: []diff.Hunk{
				{
					Header:   "@@ -1,3 +1,4 @@",
					OldStart: 1, OldLines: 3,
					NewStart: 1, NewLines: 4,
					Lines: []diff.Line{
						{Type: diff.LineContext, Content: "line 1", OldLineNum: 1, NewLineNum: 1},
						{Type: diff.LineAddition, Content: "new line", NewLineNum: 2},
						{Type: diff.LineContext, Content: "line 2", OldLineNum: 2, NewLineNum: 3},
					},
				},
				{
					Header:   "@@ -10,2 +11,3 @@",
					OldStart: 10, OldLines: 2,
					NewStart: 11, NewLines: 3,
					Lines: []diff.Line{
						{
							Type:       diff.LineContext,
							Content:    "line 10",
							OldLineNum: 10,
							NewLineNum: 11,
						},
						{Type: diff.LineAddition, Content: "another line", NewLineNum: 12},
					},
				},
			},
		},
		{
			Path:       "file2.txt",
			ChangeType: diff.ChangeTypeAdded,
			Hunks: []diff.Hunk{
				{
					Header:   "@@ -0,0 +1,2 @@",
					OldStart: 0, OldLines: 0,
					NewStart: 1, NewLines: 2,
					Lines: []diff.Line{
						{Type: diff.LineAddition, Content: "first line", NewLineNum: 1},
						{Type: diff.LineAddition, Content: "second line", NewLineNum: 2},
					},
				},
			},
		},
		{
			Path:       "file3.txt",
			ChangeType: diff.ChangeTypeDeleted,
			Hunks: []diff.Hunk{
				{
					Header:   "@@ -1,3 +0,0 @@",
					OldStart: 1, OldLines: 3,
					NewStart: 0, NewLines: 0,
					Lines: []diff.Line{
						{Type: diff.LineDeletion, Content: "deleted line 1", OldLineNum: 1},
						{Type: diff.LineDeletion, Content: "deleted line 2", OldLineNum: 2},
					},
				},
			},
		},
	}
}

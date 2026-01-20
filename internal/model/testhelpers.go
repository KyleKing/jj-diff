package model

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
)

// TestModel creates a model with mock client for testing
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

// WithChanges sets up the model with test file changes
func (m Model) WithChanges(changes []diff.FileChange) Model {
	m.changes = changes
	m.fileList.SetFiles(changes)
	if len(m.changes) > 0 {
		m.diffView.SetFileChange(changes[0])
	}
	return m
}

// WithDestination sets a destination for the model
func (m Model) WithDestination(dest string) Model {
	m.destination = dest
	return m
}

// KeyPress creates a tea.KeyMsg for single character keys
func KeyPress(key rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}}
}

// SpecialKey creates a tea.KeyMsg for special keys
func SpecialKey(keyType tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: keyType}
}

// Update is a helper that processes a message and returns the updated model
func Update(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	newModel, _ := m.Update(msg)
	return newModel.(Model)
}

// UpdateWithCmd processes a message and executes any returned command synchronously
func UpdateWithCmd(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	if cmd != nil {
		resultMsg := cmd()
		if resultMsg != nil {
			newModel, _ = m.Update(resultMsg)
			m = newModel.(Model)
		}
	}

	return m
}

// ModelAssertion provides fluent assertions on model state
type ModelAssertion struct {
	t *testing.T
	m Model
}

// Assert creates a new ModelAssertion for fluent testing
func Assert(t *testing.T, m Model) *ModelAssertion {
	t.Helper()
	return &ModelAssertion{t: t, m: m}
}

func (a *ModelAssertion) HasSelectedFile(idx int) *ModelAssertion {
	a.t.Helper()
	if a.m.selectedFile != idx {
		a.t.Errorf("Expected selectedFile=%d, got %d", idx, a.m.selectedFile)
	}
	return a
}

func (a *ModelAssertion) HasSelectedHunk(idx int) *ModelAssertion {
	a.t.Helper()
	if a.m.selectedHunk != idx {
		a.t.Errorf("Expected selectedHunk=%d, got %d", idx, a.m.selectedHunk)
	}
	return a
}

func (a *ModelAssertion) HasHunkSelected(filePath string, hunkIdx int) *ModelAssertion {
	a.t.Helper()
	if !a.m.selection.IsHunkSelected(filePath, hunkIdx) {
		a.t.Errorf("Expected hunk %d in file %s to be selected", hunkIdx, filePath)
	}
	return a
}

func (a *ModelAssertion) HasHunkNotSelected(filePath string, hunkIdx int) *ModelAssertion {
	a.t.Helper()
	if a.m.selection.IsHunkSelected(filePath, hunkIdx) {
		a.t.Errorf("Expected hunk %d in file %s to NOT be selected", hunkIdx, filePath)
	}
	return a
}

func (a *ModelAssertion) FocusedPanelIs(panel FocusedPanel) *ModelAssertion {
	a.t.Helper()
	if a.m.focusedPanel != panel {
		a.t.Errorf("Expected focusedPanel=%v, got %v", panel, a.m.focusedPanel)
	}
	return a
}

func (a *ModelAssertion) ModeIs(mode OperatingMode) *ModelAssertion {
	a.t.Helper()
	if a.m.mode != mode {
		a.t.Errorf("Expected mode=%v, got %v", mode, a.m.mode)
	}
	return a
}

func (a *ModelAssertion) HasDestination(dest string) *ModelAssertion {
	a.t.Helper()
	if a.m.destination != dest {
		a.t.Errorf("Expected destination=%s, got %s", dest, a.m.destination)
	}
	return a
}

func (a *ModelAssertion) HasError() *ModelAssertion {
	a.t.Helper()
	if a.m.err == nil {
		a.t.Error("Expected error, got nil")
	}
	return a
}

func (a *ModelAssertion) HasNoError() *ModelAssertion {
	a.t.Helper()
	if a.m.err != nil {
		a.t.Errorf("Expected no error, got %v", a.m.err)
	}
	return a
}

func (a *ModelAssertion) HasChanges(count int) *ModelAssertion {
	a.t.Helper()
	if len(a.m.changes) != count {
		a.t.Errorf("Expected %d changes, got %d", count, len(a.m.changes))
	}
	return a
}

func (a *ModelAssertion) IsInVisualMode() *ModelAssertion {
	a.t.Helper()
	if !a.m.isVisualMode {
		a.t.Error("Expected model to be in visual mode")
	}
	return a
}

func (a *ModelAssertion) IsNotInVisualMode() *ModelAssertion {
	a.t.Helper()
	if a.m.isVisualMode {
		a.t.Error("Expected model to NOT be in visual mode")
	}
	return a
}

func (a *ModelAssertion) HasLineCursor(position int) *ModelAssertion {
	a.t.Helper()
	if a.m.lineCursor != position {
		a.t.Errorf("Expected lineCursor=%d, got %d", position, a.m.lineCursor)
	}
	return a
}

// TestChanges creates sample file changes for testing
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
						{Type: diff.LineContext, Content: "line 10", OldLineNum: 10, NewLineNum: 11},
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

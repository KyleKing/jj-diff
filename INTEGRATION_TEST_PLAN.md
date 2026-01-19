# Integration Testing Plan for jj-diff TUI

## Overview

This document outlines the strategy for integration testing the jj-diff Bubbletea TUI, including test repository setup, TUI interaction simulation, and state verification.

## Testing Architecture

### Three-Layer Testing Strategy

```
┌─────────────────────────────────────────────────────────┐
│ Layer 3: Full Integration Tests (E2E)                  │
│ - Real jj repositories in temp directories              │
│ - Simulated user interactions                           │
│ - Verify actual jj state changes                        │
└─────────────────────────────────────────────────────────┘
                          ▲
                          │
┌─────────────────────────────────────────────────────────┐
│ Layer 2: Model Update Tests (Interaction)              │
│ - Direct Model.Update() testing with tea.Msg           │
│ - Mock jj client responses                              │
│ - Focus on UI state transitions                         │
└─────────────────────────────────────────────────────────┘
                          ▲
                          │
┌─────────────────────────────────────────────────────────┐
│ Layer 1: Unit Tests (Current)                          │
│ - Pure functions (diff parser, patch generator)        │
│ - Component rendering logic                             │
│ - No external dependencies                              │
└─────────────────────────────────────────────────────────┘
```

---

## Layer 1: Unit Tests (Already Implemented)

**Status**: ✅ Complete for diff parser

**Example**: `internal/diff/parser_test.go`

**Coverage**:
- ✅ Diff parsing logic
- ⚠️ Patch generation (needs tests)
- ⚠️ Selection state management (needs tests)

**To Add**:
```go
// internal/diff/patch_test.go
func TestGeneratePatch_SingleHunk(t *testing.T) { ... }
func TestGeneratePatch_MultipleFiles(t *testing.T) { ... }
func TestGeneratePatch_PartialHunks(t *testing.T) { ... }

// internal/model/selection_test.go
func TestSelectionState_ToggleHunk(t *testing.T) { ... }
func TestSelectionState_ToggleLine(t *testing.T) { ... }
```

---

## Layer 2: Model Update Tests (Recommended Approach)

### Strategy: Direct Model Testing

Test the `Model.Update()` function with manually crafted messages, avoiding full TUI initialization.

### Testing Library: None Required

Use standard Go testing with helper functions to create messages.

### Implementation Plan

#### File Structure
```
internal/model/
├── model.go
├── model_test.go          # New: Model update tests
└── testhelpers.go         # New: Test utilities
```

#### Test Helper Functions

```go
// internal/model/testhelpers.go
package model

import (
	"testing"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
)

// TestModel creates a model with mock data for testing
func TestModel(t *testing.T) Model {
	t.Helper()

	// Create mock client (could be interface for proper mocking)
	client := &jj.Client{}

	m, err := NewModel(client, "@", "", ModeInteractive)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}

	return m
}

// WithChanges sets up the model with test file changes
func (m Model) WithChanges(changes []diff.FileChange) Model {
	m.changes = changes
	m.fileList.SetFiles(changes)
	if len(changes) > 0 {
		m.diffView.SetFileChange(changes[0])
	}
	return m
}

// KeyPress creates a KeyMsg for testing
func KeyPress(key string) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(key[0])}}
}

// SpecialKey creates a KeyMsg for special keys
func SpecialKey(keyType tea.KeyType) tea.Msg {
	return tea.KeyMsg{Type: keyType}
}

// MockDiffLoaded creates a diffLoadedMsg for testing
func MockDiffLoaded(changes []diff.FileChange) tea.Msg {
	return diffLoadedMsg{changes: changes}
}

// MockRevisionSelected creates a destinationSelectedMsg
func MockRevisionSelected(changeID string) tea.Msg {
	return destinationSelectedMsg{changeID: changeID}
}

// AssertModelState provides fluent assertions on model state
type ModelAssertion struct {
	t *testing.T
	m Model
}

func AssertModel(t *testing.T, m Model) *ModelAssertion {
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

func (a *ModelAssertion) FocusedPanelIs(panel FocusedPanel) *ModelAssertion {
	a.t.Helper()
	if a.m.focusedPanel != panel {
		a.t.Errorf("Expected focusedPanel=%v, got %v", panel, a.m.focusedPanel)
	}
	return a
}
```

#### Example Model Tests

```go
// internal/model/model_test.go
package model

import (
	"testing"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/jj-diff/internal/diff"
)

func TestModel_NavigateFiles(t *testing.T) {
	// Setup
	m := TestModel(t).WithChanges([]diff.FileChange{
		{Path: "file1.txt", ChangeType: diff.ChangeTypeModified},
		{Path: "file2.txt", ChangeType: diff.ChangeTypeAdded},
		{Path: "file3.txt", ChangeType: diff.ChangeTypeDeleted},
	})

	// Navigate down
	newModel, _ := m.Update(KeyPress("j"))
	m = newModel.(Model)

	// Assert
	AssertModel(t, m).HasSelectedFile(1)

	// Navigate down again
	newModel, _ = m.Update(KeyPress("j"))
	m = newModel.(Model)
	AssertModel(t, m).HasSelectedFile(2)

	// Navigate up
	newModel, _ = m.Update(KeyPress("k"))
	m = newModel.(Model)
	AssertModel(t, m).HasSelectedFile(1)
}

func TestModel_ToggleHunkSelection(t *testing.T) {
	// Setup
	changes := []diff.FileChange{
		{
			Path:       "file1.txt",
			ChangeType: diff.ChangeTypeModified,
			Hunks: []diff.Hunk{
				{Header: "@@ -1,3 +1,4 @@", Lines: []diff.Line{
					{Type: diff.LineAddition, Content: "added line"},
				}},
			},
		},
	}
	m := TestModel(t).WithChanges(changes)
	m.focusedPanel = PanelDiffView

	// Toggle hunk selection
	newModel, _ := m.Update(KeyPress(" "))
	m = newModel.(Model)

	// Assert
	AssertModel(t, m).HasHunkSelected("file1.txt", 0)

	// Toggle again (deselect)
	newModel, _ = m.Update(KeyPress(" "))
	m = newModel.(Model)

	if m.selection.IsHunkSelected("file1.txt", 0) {
		t.Error("Expected hunk to be deselected")
	}
}

func TestModel_NavigateHunks(t *testing.T) {
	// Setup with multiple hunks
	changes := []diff.FileChange{
		{
			Path:       "file1.txt",
			ChangeType: diff.ChangeTypeModified,
			Hunks: []diff.Hunk{
				{Header: "@@ -1,3 +1,4 @@"},
				{Header: "@@ -10,2 +11,3 @@"},
				{Header: "@@ -20,1 +22,2 @@"},
			},
		},
	}
	m := TestModel(t).WithChanges(changes)
	m.focusedPanel = PanelDiffView

	// Navigate to next hunk
	newModel, _ := m.Update(KeyPress("n"))
	m = newModel.(Model)
	AssertModel(t, m).HasSelectedHunk(1)

	// Navigate to next hunk
	newModel, _ = m.Update(KeyPress("n"))
	m = newModel.(Model)
	AssertModel(t, m).HasSelectedHunk(2)

	// Try to navigate beyond last hunk (should stay at 2)
	newModel, _ = m.Update(KeyPress("n"))
	m = newModel.(Model)
	AssertModel(t, m).HasSelectedHunk(2)

	// Navigate back
	newModel, _ = m.Update(KeyPress("p"))
	m = newModel.(Model)
	AssertModel(t, m).HasSelectedHunk(1)
}

func TestModel_SwitchPanelFocus(t *testing.T) {
	m := TestModel(t)

	// Initial focus should be on file list
	AssertModel(t, m).FocusedPanelIs(PanelFileList)

	// Switch to diff view
	newModel, _ := m.Update(SpecialKey(tea.KeyTab))
	m = newModel.(Model)
	AssertModel(t, m).FocusedPanelIs(PanelDiffView)

	// Switch back
	newModel, _ = m.Update(SpecialKey(tea.KeyTab))
	m = newModel.(Model)
	AssertModel(t, m).FocusedPanelIs(PanelFileList)
}

func TestModel_DestinationPicker(t *testing.T) {
	m := TestModel(t)

	// Open destination picker
	newModel, cmd := m.Update(KeyPress("d"))
	m = newModel.(Model)

	// Command should load revisions
	if cmd == nil {
		t.Error("Expected loadRevisions command")
	}

	// Simulate revisions loaded
	revisions := []jj.RevisionEntry{
		{ChangeID: "abc123", Description: "First commit"},
		{ChangeID: "def456", Description: "Second commit"},
	}
	newModel, _ = m.Update(MockRevisionSelected("abc123"))
	m = newModel.(Model)

	// Destination should be set
	if m.destination != "abc123" {
		t.Errorf("Expected destination='abc123', got '%s'", m.destination)
	}
}

func TestModel_JumpToFirstLast(t *testing.T) {
	changes := []diff.FileChange{
		{Path: "file1.txt"},
		{Path: "file2.txt"},
		{Path: "file3.txt"},
		{Path: "file4.txt"},
		{Path: "file5.txt"},
	}
	m := TestModel(t).WithChanges(changes)
	m.selectedFile = 2 // Start in middle

	// Jump to last
	newModel, _ := m.Update(KeyPress("G"))
	m = newModel.(Model)
	AssertModel(t, m).HasSelectedFile(4)

	// Jump to first
	newModel, _ = m.Update(KeyPress("g"))
	m = newModel.(Model)
	AssertModel(t, m).HasSelectedFile(0)
}
```

### Advantages of Layer 2 Testing

✅ **Fast**: No actual TUI rendering, no sleeps
✅ **Deterministic**: Predictable, no timing issues
✅ **Isolated**: Test logic without jj repository
✅ **Easy to debug**: Standard Go test debugging
✅ **Good coverage**: Test all UI state transitions

### Disadvantages

❌ **Doesn't test rendering**: View() output not verified
❌ **Doesn't test jj integration**: MoveChanges not exercised
❌ **Doesn't test real input**: tea.KeyMsg manually created

**Recommendation**: Layer 2 tests are the sweet spot for most UI logic testing.

---

## Layer 3: Full Integration Tests (E2E)

### Strategy: Real jj Repository Tests

Test the complete workflow: setup repo → run TUI → verify jj state.

### Testing Approaches Comparison

#### Option A: teatest (Experimental Charm Library)

**Library**: `github.com/charmbracelet/x/exp/teatest`

**Pros**:
- ✅ Official Charm library
- ✅ Send keypresses and verify output
- ✅ WaitFor() conditions for async operations
- ✅ Golden file support for output verification

**Cons**:
- ⚠️ Experimental (may change)
- ⚠️ Limited documentation
- ⚠️ Requires fixed terminal size
- ⚠️ ASCII color profile needed for CI

**Use Case**: Testing full program output and final model state.

#### Option B: catwalk (Data-Driven Testing)

**Library**: `github.com/knz/catwalk`

**Pros**:
- ✅ Data-driven test files (separate test data from code)
- ✅ Built-in commands: `type`, `key`, `paste`, `resize`
- ✅ Auto-regenerate expected output with `-rewrite`
- ✅ Support for custom observers

**Cons**:
- ⚠️ Third-party library
- ⚠️ Requires learning datadriven format
- ⚠️ Better for component testing than full integration

**Use Case**: Testing individual components with many scenarios.

#### Option C: Direct Model Testing + jj Verification (Recommended)

**No special library needed** - combine Layer 2 model tests with jj state checks.

**Pros**:
- ✅ No dependencies
- ✅ Standard Go testing
- ✅ Full control over test flow
- ✅ Easy to debug
- ✅ Test actual jj operations

**Cons**:
- ⚠️ Requires careful test isolation
- ⚠️ Slower than pure unit tests
- ⚠️ Need helper functions for repo setup

**Use Case**: Integration tests that verify jj repository changes.

### Recommended Approach: Option C

Test the `jj.Client` methods directly with real temporary repositories, then test Model with mocked client for UI logic.

---

## Implementation Plan for Layer 3 Tests

### File Structure

```
tests/
├── integration/
│   ├── testhelpers.go       # Repository setup utilities
│   ├── client_test.go       # jj.Client integration tests
│   └── e2e_test.go          # Full workflow tests
└── fixtures/
    ├── simple-diff.txt      # Sample diff outputs
    └── multi-file-diff.txt  # Complex scenarios
```

### Test Repository Setup Utilities

```go
// tests/integration/testhelpers.go
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestRepo represents a temporary jj repository for testing
type TestRepo struct {
	Dir     string
	t       *testing.T
	cleanup func()
}

// NewTestRepo creates a new temporary jj repository
func NewTestRepo(t *testing.T) *TestRepo {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "jj-diff-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Initialize jj repo
	cmd := exec.Command("jj", "git", "init", tmpDir)
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to init jj repo: %v", err)
	}

	// Set test user config (avoid using global config)
	configPath := filepath.Join(tmpDir, ".jj", "repo", "config.toml")
	configContent := `
[user]
name = "Test User"
email = "test@example.com"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to write config: %v", err)
	}

	repo := &TestRepo{
		Dir: tmpDir,
		t:   t,
		cleanup: func() {
			os.RemoveAll(tmpDir)
		},
	}

	t.Cleanup(repo.cleanup)
	return repo
}

// WriteFile writes a file to the repository
func (r *TestRepo) WriteFile(path, content string) {
	r.t.Helper()
	fullPath := filepath.Join(r.Dir, path)

	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		r.t.Fatalf("Failed to create directories: %v", err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		r.t.Fatalf("Failed to write file %s: %v", path, err)
	}
}

// ReadFile reads a file from the repository
func (r *TestRepo) ReadFile(path string) string {
	r.t.Helper()
	fullPath := filepath.Join(r.Dir, path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		r.t.Fatalf("Failed to read file %s: %v", path, err)
	}
	return string(content)
}

// Commit creates a new commit with current changes
func (r *TestRepo) Commit(message string) string {
	r.t.Helper()

	// Create new commit
	cmd := exec.Command("jj", "commit", "-m", message)
	cmd.Dir = r.Dir
	if err := cmd.Run(); err != nil {
		r.t.Fatalf("Failed to commit: %v", err)
	}

	// Get the change ID
	cmd = exec.Command("jj", "log", "-r", "@-", "--no-graph", "-T", "change_id")
	cmd.Dir = r.Dir
	output, err := cmd.Output()
	if err != nil {
		r.t.Fatalf("Failed to get change ID: %v", err)
	}

	return string(output)
}

// Run executes a jj command in the repository
func (r *TestRepo) Run(args ...string) (string, error) {
	r.t.Helper()
	cmd := exec.Command("jj", args...)
	cmd.Dir = r.Dir
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// MustRun executes a jj command and fails test on error
func (r *TestRepo) MustRun(args ...string) string {
	r.t.Helper()
	output, err := r.Run(args...)
	if err != nil {
		r.t.Fatalf("Command 'jj %v' failed: %v\nOutput: %s", args, err, output)
	}
	return output
}

// GetDiff returns the diff for a revision
func (r *TestRepo) GetDiff(revision string) string {
	r.t.Helper()
	return r.MustRun("diff", "-r", revision, "--git", "--color=never")
}

// GetLog returns the log for a revision
func (r *TestRepo) GetLog(revision string) string {
	r.t.Helper()
	return r.MustRun("log", "-r", revision, "--no-graph")
}

// AssertFileContent verifies file contents
func (r *TestRepo) AssertFileContent(path, expected string) {
	r.t.Helper()
	actual := r.ReadFile(path)
	if actual != expected {
		r.t.Errorf("File %s content mismatch:\nExpected: %q\nActual: %q", path, expected, actual)
	}
}

// AssertFileNotExists verifies file doesn't exist
func (r *TestRepo) AssertFileNotExists(path string) {
	r.t.Helper()
	fullPath := filepath.Join(r.Dir, path)
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		r.t.Errorf("Expected file %s to not exist, but it does", path)
	}
}

// CreateScenario sets up a common test scenario
type ScenarioOption func(*TestRepo)

func WithInitialCommit(files map[string]string) ScenarioOption {
	return func(r *TestRepo) {
		for path, content := range files {
			r.WriteFile(path, content)
		}
		r.Commit("Initial commit")
	}
}

func WithWorkingCopyChanges(changes map[string]string) ScenarioOption {
	return func(r *TestRepo) {
		for path, content := range changes {
			r.WriteFile(path, content)
		}
	}
}

func CreateScenario(t *testing.T, opts ...ScenarioOption) *TestRepo {
	t.Helper()
	repo := NewTestRepo(t)
	for _, opt := range opts {
		opt(repo)
	}
	return repo
}
```

### jj.Client Integration Tests

```go
// tests/integration/client_test.go
package integration

import (
	"testing"
	"github.com/kyleking/jj-diff/internal/jj"
)

func TestClient_MoveChanges_SimplePatch(t *testing.T) {
	// Setup: Create repo with two commits
	repo := CreateScenario(t,
		WithInitialCommit(map[string]string{
			"file1.txt": "line 1\nline 2\nline 3\n",
		}),
	)

	// Add changes to working copy
	repo.WriteFile("file1.txt", "line 1\nline 2\nNEW LINE\nline 3\n")
	repo.Commit("Add new line")

	// Create another change in working copy
	repo.WriteFile("file1.txt", "line 1\nline 2\nNEW LINE\nline 3\nANOTHER LINE\n")

	// Create patch for the working copy change
	patch := repo.GetDiff("@")

	client := jj.NewClient(repo.Dir)

	// Move changes from @ to @-
	err := client.MoveChanges(patch, "@", "@-")
	if err != nil {
		t.Fatalf("MoveChanges failed: %v", err)
	}

	// Verify: Check that @- now has the changes
	diff := repo.GetDiff("@-")
	if diff == "" {
		t.Error("Expected @- to have changes, but diff is empty")
	}

	// Verify: Check that @ no longer has those specific changes
	currentDiff := repo.GetDiff("@")
	if currentDiff == patch {
		t.Error("Expected @ diff to change after moving hunks")
	}
}

func TestClient_MoveChanges_MultipleFiles(t *testing.T) {
	repo := CreateScenario(t,
		WithInitialCommit(map[string]string{
			"file1.txt": "content 1\n",
			"file2.txt": "content 2\n",
		}),
	)

	// Modify both files
	repo.WriteFile("file1.txt", "modified 1\n")
	repo.WriteFile("file2.txt", "modified 2\n")
	repo.Commit("Modify both files")

	// Add more changes
	repo.WriteFile("file1.txt", "modified 1\nextra line\n")
	repo.WriteFile("file2.txt", "modified 2\nextra line\n")

	patch := repo.GetDiff("@")
	client := jj.NewClient(repo.Dir)

	err := client.MoveChanges(patch, "@", "@-")
	if err != nil {
		t.Fatalf("MoveChanges failed: %v", err)
	}

	// Verify both files were moved
	diff := repo.GetDiff("@-")
	if diff == "" {
		t.Error("Expected @- to have changes from multiple files")
	}
}

func TestClient_MoveChanges_NewFile(t *testing.T) {
	repo := CreateScenario(t,
		WithInitialCommit(map[string]string{
			"existing.txt": "content\n",
		}),
	)

	// Add new file in working copy
	repo.WriteFile("newfile.txt", "new content\n")

	patch := repo.GetDiff("@")
	client := jj.NewClient(repo.Dir)

	err := client.MoveChanges(patch, "@", "@-")
	if err != nil {
		t.Fatalf("MoveChanges failed: %v", err)
	}

	// Verify new file appears in @-
	diff := repo.GetDiff("@-")
	if diff == "" {
		t.Error("Expected @- to have new file addition")
	}
}

func TestClient_MoveChanges_DeleteFile(t *testing.T) {
	repo := CreateScenario(t,
		WithInitialCommit(map[string]string{
			"file1.txt": "content\n",
			"file2.txt": "content 2\n",
		}),
	)

	repo.Commit("Both files exist")

	// Delete file in working copy
	repo.MustRun("file", "rm", "file1.txt")

	patch := repo.GetDiff("@")
	client := jj.NewClient(repo.Dir)

	err := client.MoveChanges(patch, "@", "@-")
	if err != nil {
		t.Fatalf("MoveChanges failed: %v", err)
	}

	// Verify deletion appears in @-
	diff := repo.GetDiff("@-")
	if diff == "" {
		t.Error("Expected @- to have file deletion")
	}
}

func TestClient_MoveChanges_RollbackOnError(t *testing.T) {
	repo := CreateScenario(t,
		WithInitialCommit(map[string]string{
			"file1.txt": "line 1\nline 2\n",
		}),
	)

	// Get original working copy change ID
	originalWC := repo.MustRun("log", "-r", "@", "--no-graph", "-T", "change_id")

	client := jj.NewClient(repo.Dir)

	// Create invalid patch that will fail to apply
	invalidPatch := "invalid patch content"

	err := client.MoveChanges(invalidPatch, "@", "@-")
	if err == nil {
		t.Fatal("Expected MoveChanges to fail with invalid patch")
	}

	// Verify working copy was restored
	currentWC := repo.MustRun("log", "-r", "@", "--no-graph", "-T", "change_id")
	if currentWC != originalWC {
		t.Errorf("Working copy not restored after error.\nExpected: %s\nActual: %s", originalWC, currentWC)
	}
}

func TestClient_MoveChanges_PreservesWorkingCopy(t *testing.T) {
	repo := CreateScenario(t,
		WithInitialCommit(map[string]string{
			"file1.txt": "content\n",
		}),
	)

	// Make changes
	repo.WriteFile("file1.txt", "modified\n")
	repo.Commit("Modify file")
	repo.WriteFile("file1.txt", "modified again\n")

	originalWC := repo.MustRun("log", "-r", "@", "--no-graph", "-T", "change_id")

	patch := repo.GetDiff("@")
	client := jj.NewClient(repo.Dir)

	err := client.MoveChanges(patch, "@", "@-")
	if err != nil {
		t.Fatalf("MoveChanges failed: %v", err)
	}

	// Verify we're still at the same working copy
	currentWC := repo.MustRun("log", "-r", "@", "--no-graph", "-T", "change_id")
	if currentWC != originalWC {
		t.Errorf("Working copy changed.\nExpected: %s\nActual: %s", originalWC, currentWC)
	}
}
```

### Full E2E Workflow Tests

```go
// tests/integration/e2e_test.go
package integration

import (
	"testing"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/model"
)

func TestE2E_SelectAndMoveHunk(t *testing.T) {
	// Setup repository
	repo := CreateScenario(t,
		WithInitialCommit(map[string]string{
			"file1.txt": "line 1\nline 2\nline 3\n",
		}),
	)

	// Create changes in working copy
	repo.WriteFile("file1.txt", "line 1\nNEW LINE\nline 2\nline 3\n")
	repo.Commit("Add line")
	repo.WriteFile("file1.txt", "line 1\nNEW LINE\nline 2\nANOTHER LINE\nline 3\n")

	// Initialize model
	client := jj.NewClient(repo.Dir)
	m, err := model.NewModel(client, "@", "@-", model.ModeInteractive)
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}

	// Load diff
	cmd := m.Init()
	msg := executeCmdSync(t, cmd)
	newModel, _ := m.Update(msg)
	m = newModel.(model.Model)

	// Verify changes loaded
	if len(m.GetChanges()) == 0 {
		t.Fatal("No changes loaded")
	}

	// Switch to diff view
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newModel.(model.Model)

	// Select hunk
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = newModel.(model.Model)

	// Verify hunk selected
	if !m.GetSelection().IsHunkSelected("file1.txt", 0) {
		t.Error("Hunk not selected")
	}

	// Apply changes
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	msg = executeCmdSync(t, cmd)
	newModel, _ = m.Update(msg)

	// Verify changes applied in repository
	diff := repo.GetDiff("@-")
	if diff == "" {
		t.Error("Expected @- to have changes after apply")
	}
}

// executeCmdSync executes a tea.Cmd and returns the result message
// This is a helper for testing async commands
func executeCmdSync(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("Command is nil")
	}
	return cmd()
}
```

---

## Testing Workflow

### Development Cycle

```bash
# Run unit tests (fast)
go test ./internal/diff/...

# Run model tests (medium)
go test ./internal/model/...

# Run integration tests (slow)
go test ./tests/integration/...

# Run all tests
go test ./...

# Run with verbose output
go test -v ./tests/integration/...

# Run specific test
go test -v -run TestClient_MoveChanges_SimplePatch ./tests/integration/...
```

### CI/CD Integration

```yaml
# .github/workflows/test.yml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.22'

      - name: Install jj
        run: |
          cargo install jj-cli
          jj --version

      - name: Run unit tests
        run: go test -v ./internal/...

      - name: Run integration tests
        run: go test -v ./tests/integration/...

      - name: Test coverage
        run: go test -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.out
```

---

## Test Coverage Goals

### Current Coverage
- ✅ **Diff parsing**: 100% (8 tests)
- ⚠️ **Patch generation**: 0% (needs tests)
- ⚠️ **Selection state**: 0% (needs tests)
- ⚠️ **Model updates**: 0% (needs tests)
- ⚠️ **jj integration**: 0% (needs tests)

### Target Coverage (Phase 1 Complete)
- ✅ **Unit tests**: >90% for core logic
- ✅ **Model tests**: >80% for UI state transitions
- ✅ **Integration tests**: Critical workflows covered
  - Move single hunk
  - Move multiple hunks
  - Move from multiple files
  - Handle new/deleted files
  - Rollback on errors
  - Preserve working copy state

---

## Test Maintenance

### Best Practices

1. **Keep tests fast**: Unit tests < 1s, integration tests < 5s each
2. **Isolate state**: Each test gets fresh repository
3. **Clear naming**: `TestComponent_Action_ExpectedResult`
4. **Arrange-Act-Assert**: Clear test structure
5. **Test helpers**: DRY with shared utilities
6. **Golden files**: For complex output validation (if needed)

### Common Pitfalls to Avoid

❌ **Don't use sleep()**: Use WaitFor conditions or sync execution
❌ **Don't share test state**: Each test creates its own repo
❌ **Don't test implementation details**: Test behavior, not internals
❌ **Don't make tests brittle**: Avoid exact string matching where possible
❌ **Don't forget cleanup**: Use t.Cleanup() for temp resources

---

## Next Steps

### Immediate (Week 1)
1. ✅ Create `internal/model/testhelpers.go` with test utilities
2. ✅ Write 5-10 model update tests for core interactions
3. ✅ Create `tests/integration/testhelpers.go` for repo setup
4. ✅ Write 3-5 client integration tests for MoveChanges

### Short-term (Week 2)
1. ✅ Add patch generation unit tests
2. ✅ Add selection state unit tests
3. ✅ Add E2E workflow tests
4. ✅ Setup CI/CD pipeline

### Medium-term (Month 2)
1. ⏳ Add component tests for diffview/filelist if needed
2. ⏳ Consider teatest for golden file output validation
3. ⏳ Performance testing for large diffs
4. ⏳ Fuzz testing for diff parser

---

## References

- [Testing Bubble Tea Interfaces](https://patternmatched.substack.com/p/testing-bubble-tea-interfaces)
- [Writing Bubble Tea Tests (teatest)](https://carlosbecker.com/posts/teatest/)
- [Catwalk: Test Library for Bubbletea](https://github.com/knz/catwalk)
- [Bubbletea GitHub](https://github.com/charmbracelet/bubbletea)
- [Go Testing Package](https://pkg.go.dev/testing)
- [Charm Blog: teatest](https://charm.land/blog/teatest/)

---

## Summary

**Recommended Approach**: Three-layer testing strategy

1. **Layer 1 (Unit)**: Test pure functions (parser, patch generator) ✅ Partially done
2. **Layer 2 (Model)**: Test UI logic with direct Model.Update() calls ⚠️ To implement
3. **Layer 3 (Integration)**: Test jj operations with real repositories ⚠️ To implement

**No special TUI testing library needed** - Standard Go testing with helper functions provides the best balance of simplicity, control, and maintainability.

**Key insight**: Test the model's `Update()` function directly rather than trying to simulate a full TUI. This gives deterministic, fast, and easy-to-debug tests. Add integration tests for jj repository operations to ensure end-to-end correctness.

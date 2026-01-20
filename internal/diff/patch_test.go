package diff

import (
	"strings"
	"testing"
)

// mockSelectionState implements the selection state interface for testing
type mockSelectionState struct {
	selections      map[string]map[int]bool
	lineSelections  map[string]map[int]map[int]bool
	partialHunks    map[string]map[int]bool
}

func (m *mockSelectionState) IsHunkSelected(filePath string, hunkIdx int) bool {
	if fileHunks, ok := m.selections[filePath]; ok {
		return fileHunks[hunkIdx]
	}
	return false
}

func (m *mockSelectionState) HasPartialSelection(filePath string, hunkIdx int) bool {
	if filePartials, ok := m.partialHunks[filePath]; ok {
		return filePartials[hunkIdx]
	}
	return false
}

func (m *mockSelectionState) IsLineSelected(filePath string, hunkIdx, lineIdx int) bool {
	if fileLines, ok := m.lineSelections[filePath]; ok {
		if hunkLines, ok := fileLines[hunkIdx]; ok {
			return hunkLines[lineIdx]
		}
	}
	return false
}

func newMockSelection(selections map[string]map[int]bool) *mockSelectionState {
	return &mockSelectionState{
		selections:      selections,
		lineSelections:  make(map[string]map[int]map[int]bool),
		partialHunks:    make(map[string]map[int]bool),
	}
}

// TestGeneratePatch_SingleHunk tests patch generation with a single selected hunk
func TestGeneratePatch_SingleHunk(t *testing.T) {
	files := []FileChange{
		{
			Path:       "file.txt",
			ChangeType: ChangeTypeModified,
			Hunks: []Hunk{
				{
					Header: "@@ -1,3 +1,4 @@",
					Lines: []Line{
						{Type: LineContext, Content: "line 1"},
						{Type: LineAddition, Content: "new line"},
						{Type: LineContext, Content: "line 2"},
					},
				},
			},
		},
	}

	selection := map[string]map[int]bool{
		"file.txt": {0: true},
	}

	patch := GeneratePatch(files, newMockSelection(selection))

	// Verify patch structure
	if !strings.Contains(patch, "diff --git a/file.txt b/file.txt") {
		t.Error("Patch missing diff header")
	}
	if !strings.Contains(patch, "--- a/file.txt") {
		t.Error("Patch missing --- line")
	}
	if !strings.Contains(patch, "+++ b/file.txt") {
		t.Error("Patch missing +++ line")
	}
	if !strings.Contains(patch, "@@ -1,3 +1,4 @@") {
		t.Error("Patch missing hunk header")
	}
	if !strings.Contains(patch, " line 1") {
		t.Error("Patch missing context line")
	}
	if !strings.Contains(patch, "+new line") {
		t.Error("Patch missing addition line")
	}
}

// TestGeneratePatch_MultipleHunks tests selecting multiple hunks from same file
func TestGeneratePatch_MultipleHunks(t *testing.T) {
	files := []FileChange{
		{
			Path:       "file.txt",
			ChangeType: ChangeTypeModified,
			Hunks: []Hunk{
				{
					Header: "@@ -1,2 +1,3 @@",
					Lines: []Line{
						{Type: LineAddition, Content: "first hunk"},
					},
				},
				{
					Header: "@@ -10,2 +11,3 @@",
					Lines: []Line{
						{Type: LineAddition, Content: "second hunk"},
					},
				},
			},
		},
	}

	selection := map[string]map[int]bool{
		"file.txt": {0: true, 1: true},
	}

	patch := GeneratePatch(files, newMockSelection(selection))

	if !strings.Contains(patch, "@@ -1,2 +1,3 @@") {
		t.Error("Patch missing first hunk")
	}
	if !strings.Contains(patch, "@@ -10,2 +11,3 @@") {
		t.Error("Patch missing second hunk")
	}
}

// TestGeneratePatch_MultipleFiles tests selecting hunks from multiple files
func TestGeneratePatch_MultipleFiles(t *testing.T) {
	files := []FileChange{
		{
			Path:       "file1.txt",
			ChangeType: ChangeTypeModified,
			Hunks: []Hunk{
				{
					Header: "@@ -1,2 +1,3 @@",
					Lines:  []Line{{Type: LineAddition, Content: "file1 change"}},
				},
			},
		},
		{
			Path:       "file2.txt",
			ChangeType: ChangeTypeModified,
			Hunks: []Hunk{
				{
					Header: "@@ -5,2 +5,3 @@",
					Lines:  []Line{{Type: LineAddition, Content: "file2 change"}},
				},
			},
		},
	}

	selection := map[string]map[int]bool{
		"file1.txt": {0: true},
		"file2.txt": {0: true},
	}

	patch := GeneratePatch(files, newMockSelection(selection))

	if !strings.Contains(patch, "diff --git a/file1.txt b/file1.txt") {
		t.Error("Patch missing file1 header")
	}
	if !strings.Contains(patch, "diff --git a/file2.txt b/file2.txt") {
		t.Error("Patch missing file2 header")
	}
	if !strings.Contains(patch, "file1 change") {
		t.Error("Patch missing file1 content")
	}
	if !strings.Contains(patch, "file2 change") {
		t.Error("Patch missing file2 content")
	}
}

// TestGeneratePatch_NewFile tests patch generation for added files
func TestGeneratePatch_NewFile(t *testing.T) {
	files := []FileChange{
		{
			Path:       "newfile.txt",
			ChangeType: ChangeTypeAdded,
			Hunks: []Hunk{
				{
					Header: "@@ -0,0 +1,2 @@",
					Lines: []Line{
						{Type: LineAddition, Content: "first line"},
						{Type: LineAddition, Content: "second line"},
					},
				},
			},
		},
	}

	selection := map[string]map[int]bool{
		"newfile.txt": {0: true},
	}

	patch := GeneratePatch(files, newMockSelection(selection))

	if !strings.Contains(patch, "new file mode 100644") {
		t.Error("Patch missing new file mode")
	}
	if !strings.Contains(patch, "--- /dev/null") {
		t.Error("Patch missing /dev/null for new file")
	}
	if !strings.Contains(patch, "+++ b/newfile.txt") {
		t.Error("Patch missing +++ line for new file")
	}
}

// TestGeneratePatch_DeletedFile tests patch generation for deleted files
func TestGeneratePatch_DeletedFile(t *testing.T) {
	files := []FileChange{
		{
			Path:       "deleted.txt",
			ChangeType: ChangeTypeDeleted,
			Hunks: []Hunk{
				{
					Header: "@@ -1,2 +0,0 @@",
					Lines: []Line{
						{Type: LineDeletion, Content: "deleted line"},
					},
				},
			},
		},
	}

	selection := map[string]map[int]bool{
		"deleted.txt": {0: true},
	}

	patch := GeneratePatch(files, newMockSelection(selection))

	if !strings.Contains(patch, "deleted file mode 100644") {
		t.Error("Patch missing deleted file mode")
	}
	if !strings.Contains(patch, "--- a/deleted.txt") {
		t.Error("Patch missing --- line for deleted file")
	}
	if !strings.Contains(patch, "+++ /dev/null") {
		t.Error("Patch missing /dev/null for deleted file")
	}
}

// TestGeneratePatch_NoSelection tests that empty selection produces empty patch
func TestGeneratePatch_NoSelection(t *testing.T) {
	files := []FileChange{
		{
			Path:       "file.txt",
			ChangeType: ChangeTypeModified,
			Hunks:      []Hunk{{Header: "@@ -1,2 +1,3 @@"}},
		},
	}

	selection := map[string]map[int]bool{}

	patch := GeneratePatch(files, newMockSelection(selection))

	if patch != "" {
		t.Errorf("Expected empty patch, got: %s", patch)
	}
}

// TestGeneratePatch_PartialSelection tests selecting only some hunks
func TestGeneratePatch_PartialSelection(t *testing.T) {
	files := []FileChange{
		{
			Path:       "file.txt",
			ChangeType: ChangeTypeModified,
			Hunks: []Hunk{
				{
					Header: "@@ -1,2 +1,3 @@",
					Lines:  []Line{{Type: LineAddition, Content: "hunk 0"}},
				},
				{
					Header: "@@ -10,2 +11,3 @@",
					Lines:  []Line{{Type: LineAddition, Content: "hunk 1"}},
				},
				{
					Header: "@@ -20,2 +21,3 @@",
					Lines:  []Line{{Type: LineAddition, Content: "hunk 2"}},
				},
			},
		},
	}

	// Select only hunks 0 and 2, skip hunk 1
	selection := map[string]map[int]bool{
		"file.txt": {0: true, 2: true},
	}

	patch := GeneratePatch(files, newMockSelection(selection))

	if !strings.Contains(patch, "hunk 0") {
		t.Error("Patch missing selected hunk 0")
	}
	if strings.Contains(patch, "hunk 1") {
		t.Error("Patch should not contain unselected hunk 1")
	}
	if !strings.Contains(patch, "hunk 2") {
		t.Error("Patch missing selected hunk 2")
	}
}

// TestGetSelectedHunksMap tests conversion from selection state to map
func TestGetSelectedHunksMap(t *testing.T) {
	files := []FileChange{
		{
			Path:  "file1.txt",
			Hunks: make([]Hunk, 3),
		},
		{
			Path:  "file2.txt",
			Hunks: make([]Hunk, 2),
		},
	}

	mock := newMockSelection(map[string]map[int]bool{
		"file1.txt": {0: true, 2: true},
		"file2.txt": {1: true},
	})

	result := GetSelectedHunksMap(files, mock)

	// Check file1.txt selections
	if !result["file1.txt"][0] {
		t.Error("Expected file1.txt hunk 0 to be selected")
	}
	if result["file1.txt"][1] {
		t.Error("Expected file1.txt hunk 1 to NOT be selected")
	}
	if !result["file1.txt"][2] {
		t.Error("Expected file1.txt hunk 2 to be selected")
	}

	// Check file2.txt selections
	if result["file2.txt"][0] {
		t.Error("Expected file2.txt hunk 0 to NOT be selected")
	}
	if !result["file2.txt"][1] {
		t.Error("Expected file2.txt hunk 1 to be selected")
	}
}

// TestGetSelectedHunksMap_NoSelections tests with no selections
func TestGetSelectedHunksMap_NoSelections(t *testing.T) {
	files := []FileChange{
		{
			Path:  "file.txt",
			Hunks: make([]Hunk, 2),
		},
	}

	mock := newMockSelection(map[string]map[int]bool{})

	result := GetSelectedHunksMap(files, mock)

	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d entries", len(result))
	}
}

// TestGeneratePatch_PartialHunk tests generating patch with line-level selection
func TestGeneratePatch_PartialHunk(t *testing.T) {
	files := []FileChange{
		{
			Path:       "file.txt",
			ChangeType: ChangeTypeModified,
			Hunks: []Hunk{
				{
					Header: "@@ -1,10 +1,11 @@",
					Lines: []Line{
						{Type: LineContext, Content: "line 1", OldLineNum: 1, NewLineNum: 1},
						{Type: LineContext, Content: "line 2", OldLineNum: 2, NewLineNum: 2},
						{Type: LineContext, Content: "line 3", OldLineNum: 3, NewLineNum: 3},
						{Type: LineAddition, Content: "added line", OldLineNum: 0, NewLineNum: 4},
						{Type: LineContext, Content: "line 4", OldLineNum: 4, NewLineNum: 5},
						{Type: LineContext, Content: "line 5", OldLineNum: 5, NewLineNum: 6},
						{Type: LineContext, Content: "line 6", OldLineNum: 6, NewLineNum: 7},
						{Type: LineContext, Content: "line 7", OldLineNum: 7, NewLineNum: 8},
						{Type: LineContext, Content: "line 8", OldLineNum: 8, NewLineNum: 9},
						{Type: LineContext, Content: "line 9", OldLineNum: 9, NewLineNum: 10},
					},
				},
			},
		},
	}

	// Select only line index 3 (the addition)
	mock := &mockSelectionState{
		selections:     make(map[string]map[int]bool),
		lineSelections: make(map[string]map[int]map[int]bool),
		partialHunks:   make(map[string]map[int]bool),
	}
	mock.partialHunks["file.txt"] = map[int]bool{0: true}
	mock.lineSelections["file.txt"] = map[int]map[int]bool{
		0: {3: true}, // Select only the added line
	}

	patch := GeneratePatch(files, mock)

	// Should include the added line plus 3 lines of context before and after
	if !strings.Contains(patch, "+added line") {
		t.Error("Patch missing selected addition")
	}
	if !strings.Contains(patch, "line 1") {
		t.Error("Patch missing context before")
	}
	if !strings.Contains(patch, "line 6") {
		t.Error("Patch missing context after")
	}
}

// TestExpandWithContext tests context expansion algorithm
func TestExpandWithContext(t *testing.T) {
	tests := []struct {
		name         string
		selected     map[int]bool
		totalLines   int
		contextLines int
		expected     map[int]bool
	}{
		{
			name:         "single line with context",
			selected:     map[int]bool{5: true},
			totalLines:   10,
			contextLines: 2,
			expected:     map[int]bool{3: true, 4: true, 5: true, 6: true, 7: true},
		},
		{
			name:         "line at start",
			selected:     map[int]bool{0: true},
			totalLines:   10,
			contextLines: 3,
			expected:     map[int]bool{0: true, 1: true, 2: true, 3: true},
		},
		{
			name:         "line at end",
			selected:     map[int]bool{9: true},
			totalLines:   10,
			contextLines: 3,
			expected:     map[int]bool{6: true, 7: true, 8: true, 9: true},
		},
		{
			name:         "adjacent selections merge",
			selected:     map[int]bool{3: true, 5: true},
			totalLines:   10,
			contextLines: 1,
			expected:     map[int]bool{2: true, 3: true, 4: true, 5: true, 6: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandWithContext(tt.selected, tt.totalLines, tt.contextLines)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d lines, got %d", len(tt.expected), len(result))
			}

			for line := range tt.expected {
				if !result[line] {
					t.Errorf("Expected line %d to be included", line)
				}
			}
		})
	}
}

// TestRecalculateHunkHeader tests hunk header recalculation
func TestRecalculateHunkHeader(t *testing.T) {
	tests := []struct {
		name     string
		lines    []Line
		expected string
	}{
		{
			name: "only additions",
			lines: []Line{
				{Type: LineAddition, Content: "line 1", OldLineNum: 0, NewLineNum: 1},
				{Type: LineAddition, Content: "line 2", OldLineNum: 0, NewLineNum: 2},
			},
			expected: "@@ -0,0 +1,2 @@",
		},
		{
			name: "only deletions",
			lines: []Line{
				{Type: LineDeletion, Content: "line 1", OldLineNum: 1, NewLineNum: 0},
				{Type: LineDeletion, Content: "line 2", OldLineNum: 2, NewLineNum: 0},
			},
			expected: "@@ -1,2 +0,0 @@",
		},
		{
			name: "mixed with context",
			lines: []Line{
				{Type: LineContext, Content: "line 1", OldLineNum: 1, NewLineNum: 1},
				{Type: LineAddition, Content: "added", OldLineNum: 0, NewLineNum: 2},
				{Type: LineContext, Content: "line 2", OldLineNum: 2, NewLineNum: 3},
				{Type: LineDeletion, Content: "deleted", OldLineNum: 3, NewLineNum: 0},
				{Type: LineContext, Content: "line 3", OldLineNum: 4, NewLineNum: 4},
			},
			expected: "@@ -1,4 +1,4 @@",
		},
		{
			name:     "empty",
			lines:    []Line{},
			expected: "@@ -0,0 +0,0 @@",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := recalculateHunkHeader(tt.lines)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

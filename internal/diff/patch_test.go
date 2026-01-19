package diff

import (
	"strings"
	"testing"
)

// mockSelectionState implements the selection state interface for testing
type mockSelectionState struct {
	selections map[string]map[int]bool
}

func (m *mockSelectionState) IsHunkSelected(filePath string, hunkIdx int) bool {
	if fileHunks, ok := m.selections[filePath]; ok {
		return fileHunks[hunkIdx]
	}
	return false
}

func newMockSelection(selections map[string]map[int]bool) *mockSelectionState {
	return &mockSelectionState{selections: selections}
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

	patch := GeneratePatch(files, selection)

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

	patch := GeneratePatch(files, selection)

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

	patch := GeneratePatch(files, selection)

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

	patch := GeneratePatch(files, selection)

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

	patch := GeneratePatch(files, selection)

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

	patch := GeneratePatch(files, selection)

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

	patch := GeneratePatch(files, selection)

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

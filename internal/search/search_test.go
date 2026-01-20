package search

import (
	"testing"

	"github.com/kyleking/jj-diff/internal/diff"
)

// TestNewSearchState tests initialization
func TestNewSearchState(t *testing.T) {
	s := NewSearchState()

	if s.Query != "" {
		t.Error("Expected empty query")
	}
	if len(s.Matches) != 0 {
		t.Error("Expected no matches")
	}
	if s.CurrentIdx != -1 {
		t.Error("Expected CurrentIdx to be -1")
	}
	if s.IsActive {
		t.Error("Expected IsActive to be false")
	}
	if s.IsCaseSensitive {
		t.Error("Expected IsCaseSensitive to be false")
	}
}

// TestExecuteSearch_EmptyQuery tests search with empty query
func TestExecuteSearch_EmptyQuery(t *testing.T) {
	files := []diff.FileChange{
		{Path: "file.txt", Hunks: []diff.Hunk{{Lines: []diff.Line{{Content: "hello"}}}}},
	}

	s := NewSearchState()
	s.Query = ""
	s.ExecuteSearch(files)

	if len(s.Matches) != 0 {
		t.Errorf("Expected no matches for empty query, got %d", len(s.Matches))
	}
	if s.CurrentIdx != -1 {
		t.Error("Expected CurrentIdx to be -1 for empty query")
	}
}

// TestExecuteSearch_FilePathMatch tests matching in file paths
func TestExecuteSearch_FilePathMatch(t *testing.T) {
	files := []diff.FileChange{
		{Path: "src/main.go"},
		{Path: "src/test.go"},
		{Path: "pkg/util.go"},
	}

	s := NewSearchState()
	s.Query = "main"
	s.ExecuteSearch(files)

	if len(s.Matches) != 1 {
		t.Errorf("Expected 1 match, got %d", len(s.Matches))
	}
	if s.Matches[0].FileIdx != 0 {
		t.Error("Expected match in file 0")
	}
	if s.Matches[0].HunkIdx != -1 {
		t.Error("Expected HunkIdx to be -1 for file path match")
	}
	if s.Matches[0].StartCol != 4 || s.Matches[0].EndCol != 8 {
		t.Errorf("Expected match at columns 4-8, got %d-%d", s.Matches[0].StartCol, s.Matches[0].EndCol)
	}
}

// TestExecuteSearch_LineContentMatch tests matching in line content
func TestExecuteSearch_LineContentMatch(t *testing.T) {
	files := []diff.FileChange{
		{
			Path: "file.txt",
			Hunks: []diff.Hunk{
				{
					Lines: []diff.Line{
						{Content: "hello world"},
						{Content: "goodbye world"},
					},
				},
			},
		},
	}

	s := NewSearchState()
	s.Query = "world"
	s.ExecuteSearch(files)

	if len(s.Matches) != 2 {
		t.Errorf("Expected 2 matches, got %d", len(s.Matches))
	}

	// First match
	if s.Matches[0].LineIdx != 0 || s.Matches[0].StartCol != 6 {
		t.Error("First match should be at line 0, column 6")
	}

	// Second match
	if s.Matches[1].LineIdx != 1 || s.Matches[1].StartCol != 8 {
		t.Error("Second match should be at line 1, column 8")
	}
}

// TestExecuteSearch_MultipleMatchesPerLine tests multiple matches in same line
func TestExecuteSearch_MultipleMatchesPerLine(t *testing.T) {
	files := []diff.FileChange{
		{
			Path: "file.txt",
			Hunks: []diff.Hunk{
				{
					Lines: []diff.Line{
						{Content: "test test test"},
					},
				},
			},
		},
	}

	s := NewSearchState()
	s.Query = "test"
	s.ExecuteSearch(files)

	if len(s.Matches) != 3 {
		t.Errorf("Expected 3 matches, got %d", len(s.Matches))
	}

	// Verify all matches are in same line
	for i, match := range s.Matches {
		if match.LineIdx != 0 {
			t.Errorf("Match %d should be in line 0", i)
		}
	}

	// Verify positions
	expected := []struct{ start, end int }{
		{0, 4},
		{5, 9},
		{10, 14},
	}
	for i, exp := range expected {
		if s.Matches[i].StartCol != exp.start || s.Matches[i].EndCol != exp.end {
			t.Errorf("Match %d: expected %d-%d, got %d-%d",
				i, exp.start, exp.end, s.Matches[i].StartCol, s.Matches[i].EndCol)
		}
	}
}

// TestExecuteSearch_CaseInsensitive tests case-insensitive search (default)
func TestExecuteSearch_CaseInsensitive(t *testing.T) {
	files := []diff.FileChange{
		{
			Path: "file.txt",
			Hunks: []diff.Hunk{
				{
					Lines: []diff.Line{
						{Content: "Hello World"},
					},
				},
			},
		},
	}

	s := NewSearchState()
	s.Query = "hello"
	s.IsCaseSensitive = false
	s.ExecuteSearch(files)

	if len(s.Matches) != 1 {
		t.Errorf("Expected 1 match (case-insensitive), got %d", len(s.Matches))
	}
}

// TestExecuteSearch_CaseSensitive tests case-sensitive search
func TestExecuteSearch_CaseSensitive(t *testing.T) {
	files := []diff.FileChange{
		{
			Path: "file.txt",
			Hunks: []diff.Hunk{
				{
					Lines: []diff.Line{
						{Content: "Hello World"},
					},
				},
			},
		},
	}

	s := NewSearchState()
	s.Query = "hello"
	s.IsCaseSensitive = true
	s.ExecuteSearch(files)

	if len(s.Matches) != 0 {
		t.Errorf("Expected 0 matches (case-sensitive), got %d", len(s.Matches))
	}
}

// TestNextMatch tests navigation to next match
func TestNextMatch(t *testing.T) {
	s := &SearchState{
		Matches: []MatchLocation{
			{FileIdx: 0},
			{FileIdx: 1},
			{FileIdx: 2},
		},
		CurrentIdx: 0,
	}

	// Move to next
	match := s.NextMatch()
	if match.FileIdx != 1 || s.CurrentIdx != 1 {
		t.Error("NextMatch should move to index 1")
	}

	// Move to next again
	match = s.NextMatch()
	if match.FileIdx != 2 || s.CurrentIdx != 2 {
		t.Error("NextMatch should move to index 2")
	}

	// Wrap around to start
	match = s.NextMatch()
	if match.FileIdx != 0 || s.CurrentIdx != 0 {
		t.Error("NextMatch should wrap to index 0")
	}
}

// TestPrevMatch tests navigation to previous match
func TestPrevMatch(t *testing.T) {
	s := &SearchState{
		Matches: []MatchLocation{
			{FileIdx: 0},
			{FileIdx: 1},
			{FileIdx: 2},
		},
		CurrentIdx: 2,
	}

	// Move to previous
	match := s.PrevMatch()
	if match.FileIdx != 1 || s.CurrentIdx != 1 {
		t.Error("PrevMatch should move to index 1")
	}

	// Move to previous again
	match = s.PrevMatch()
	if match.FileIdx != 0 || s.CurrentIdx != 0 {
		t.Error("PrevMatch should move to index 0")
	}

	// Wrap around to end
	match = s.PrevMatch()
	if match.FileIdx != 2 || s.CurrentIdx != 2 {
		t.Error("PrevMatch should wrap to index 2")
	}
}

// TestNextMatch_NoMatches tests navigation with no matches
func TestNextMatch_NoMatches(t *testing.T) {
	s := &SearchState{
		Matches:    []MatchLocation{},
		CurrentIdx: -1,
	}

	match := s.NextMatch()
	if match != nil {
		t.Error("NextMatch should return nil when no matches")
	}
}

// TestGetCurrentMatch tests getting current match
func TestGetCurrentMatch(t *testing.T) {
	s := &SearchState{
		Matches: []MatchLocation{
			{FileIdx: 0},
			{FileIdx: 1},
		},
		CurrentIdx: 1,
	}

	match := s.GetCurrentMatch()
	if match == nil || match.FileIdx != 1 {
		t.Error("GetCurrentMatch should return match at CurrentIdx")
	}
}

// TestGetCurrentMatch_Invalid tests getting match with invalid index
func TestGetCurrentMatch_Invalid(t *testing.T) {
	s := &SearchState{
		Matches:    []MatchLocation{{FileIdx: 0}},
		CurrentIdx: 5,
	}

	match := s.GetCurrentMatch()
	if match != nil {
		t.Error("GetCurrentMatch should return nil for invalid index")
	}
}

// TestMatchCount tests counting matches
func TestMatchCount(t *testing.T) {
	s := &SearchState{
		Matches: []MatchLocation{
			{FileIdx: 0},
			{FileIdx: 1},
			{FileIdx: 2},
		},
	}

	if s.MatchCount() != 3 {
		t.Errorf("Expected MatchCount to be 3, got %d", s.MatchCount())
	}
}

// TestIsLineMatch tests checking if a line has matches
func TestIsLineMatch(t *testing.T) {
	s := &SearchState{
		Matches: []MatchLocation{
			{FileIdx: 0, HunkIdx: 0, LineIdx: 5},
			{FileIdx: 0, HunkIdx: 1, LineIdx: 3},
		},
	}

	if !s.IsLineMatch(0, 0, 5) {
		t.Error("Should match at file 0, hunk 0, line 5")
	}

	if s.IsLineMatch(0, 0, 6) {
		t.Error("Should not match at file 0, hunk 0, line 6")
	}
}

// TestIsCurrentMatch tests checking if position is current match
func TestIsCurrentMatch(t *testing.T) {
	s := &SearchState{
		Matches: []MatchLocation{
			{FileIdx: 0, HunkIdx: 0, LineIdx: 5},
			{FileIdx: 0, HunkIdx: 1, LineIdx: 3},
		},
		CurrentIdx: 1,
	}

	if !s.IsCurrentMatch(0, 1, 3) {
		t.Error("Should be current match at file 0, hunk 1, line 3")
	}

	if s.IsCurrentMatch(0, 0, 5) {
		t.Error("Should not be current match at file 0, hunk 0, line 5")
	}
}

// TestGetMatchesForLine tests getting all matches for a specific line
func TestGetMatchesForLine(t *testing.T) {
	s := &SearchState{
		Matches: []MatchLocation{
			{FileIdx: 0, HunkIdx: 0, LineIdx: 5, StartCol: 0, EndCol: 4},
			{FileIdx: 0, HunkIdx: 0, LineIdx: 5, StartCol: 10, EndCol: 14},
			{FileIdx: 0, HunkIdx: 0, LineIdx: 6, StartCol: 0, EndCol: 4},
		},
	}

	matches := s.GetMatchesForLine(0, 0, 5)
	if len(matches) != 2 {
		t.Errorf("Expected 2 matches for line 5, got %d", len(matches))
	}

	matches = s.GetMatchesForLine(0, 0, 6)
	if len(matches) != 1 {
		t.Errorf("Expected 1 match for line 6, got %d", len(matches))
	}
}

// TestSaveAndRestoreOriginalState tests navigation state management
func TestSaveAndRestoreOriginalState(t *testing.T) {
	s := NewSearchState()

	original := NavigationState{
		SelectedFile:   5,
		SelectedHunk:   2,
		DiffViewOffset: 10,
		FocusedPanel:   1,
	}

	s.SaveOriginalState(original)
	restored := s.RestoreOriginalState()

	if restored.SelectedFile != 5 || restored.SelectedHunk != 2 ||
		restored.DiffViewOffset != 10 || restored.FocusedPanel != 1 {
		t.Error("Restored state doesn't match saved state")
	}
}

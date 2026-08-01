// Package search finds a literal substring across parsed diffs and tracks the cursor over the hits,
// including the view position to restore when a search is canceled.
package search

import (
	"strings"

	"github.com/kyleking/jj-diff/internal/diff"
)

// MatchLocation is one hit. StartCol and EndCol are byte offsets into MatchText with EndCol
// exclusive, and HunkIdx and LineIdx are -1 on a hit in a file path rather than a diff line.
type MatchLocation struct {
	FilePath  string
	MatchText string
	FileIdx   int
	HunkIdx   int
	LineIdx   int
	StartCol  int
	EndCol    int
}

// NavigationState is the view position a search moves away from, held so canceling can put the
// cursor, scroll offset, and focused panel back where they were.
type NavigationState struct {
	SelectedFile   int
	SelectedHunk   int
	DiffViewOffset int
	FocusedPanel   int
}

// State carries a query and the hits it produced. Query and IsCaseSensitive are inputs, and
// Matches and CurrentIdx only reflect them once ExecuteSearch runs, so editing a field alone leaves
// the match list stale.
type State struct {
	Query           string
	Matches         []MatchLocation
	CurrentIdx      int
	OriginalState   NavigationState
	IsActive        bool
	IsCaseSensitive bool
}

// NewState returns an inactive, case-insensitive state with CurrentIdx at -1, which is the
// "nothing selected" value every match accessor checks for.
func NewState() *State {
	return &State{
		Query:           "",
		Matches:         []MatchLocation{},
		CurrentIdx:      -1,
		IsActive:        false,
		IsCaseSensitive: false,
	}
}

// SaveOriginalState records where to return if the search is canceled, overwriting any earlier save.
// Call it when the search opens, because calling it after the cursor has jumped to a match saves the
// match instead of the origin.
func (s *State) SaveOriginalState(nav NavigationState) {
	s.OriginalState = nav
}

// RestoreOriginalState returns the position saved when the search opened, leaving it saved so a
// repeated cancel is harmless.
func (s *State) RestoreOriginalState() NavigationState {
	return s.OriginalState
}

// ExecuteSearch rebuilds the match list by scanning every file path and diff line, then points
// CurrentIdx at the first hit (or -1 when there is none). Overlapping hits are all reported, because
// the scan resumes one byte past each one, and an empty Query clears the list.
func (s *State) ExecuteSearch(files []diff.FileChange) {
	s.Matches = []MatchLocation{}
	s.CurrentIdx = -1

	if s.Query == "" {
		return
	}

	query := s.normalize(s.Query)

	for fileIdx, file := range files {
		s.appendFileMatches(query, fileIdx, file)
	}

	if len(s.Matches) > 0 {
		s.CurrentIdx = 0
	}
}

func (s *State) normalize(text string) string {
	if s.IsCaseSensitive {
		return text
	}

	return strings.ToLower(text)
}

func (s *State) appendFileMatches(query string, fileIdx int, file diff.FileChange) {
	if idx := strings.Index(s.normalize(file.Path), query); idx != -1 {
		s.Matches = append(s.Matches, MatchLocation{
			FileIdx:   fileIdx,
			HunkIdx:   -1,
			LineIdx:   -1,
			FilePath:  file.Path,
			StartCol:  idx,
			EndCol:    idx + len(s.Query),
			MatchText: file.Path,
		})
	}

	for hunkIdx, hunk := range file.Hunks {
		for lineIdx, line := range hunk.Lines {
			s.appendLineMatches(query, MatchLocation{
				FileIdx:  fileIdx,
				HunkIdx:  hunkIdx,
				LineIdx:  lineIdx,
				FilePath: file.Path,
			}, line.Content)
		}
	}
}

// appendLineMatches records every hit in content, resuming one byte past each one so overlapping
// occurrences are all reported.
func (s *State) appendLineMatches(query string, at MatchLocation, content string) {
	searchContent := s.normalize(content)

	for idx := 0; ; {
		pos := strings.Index(searchContent[idx:], query)
		if pos == -1 {
			return
		}

		absolutePos := idx + pos
		match := at
		match.StartCol = absolutePos
		match.EndCol = absolutePos + len(s.Query)
		match.MatchText = content
		s.Matches = append(s.Matches, match)

		idx = absolutePos + 1
	}
}

// NextMatch advances one match and wraps past the last, returning nil when there are no matches. The
// pointer refers into the match slice, which the next ExecuteSearch replaces.
func (s *State) NextMatch() *MatchLocation {
	if len(s.Matches) == 0 {
		return nil
	}

	s.CurrentIdx = (s.CurrentIdx + 1) % len(s.Matches)

	return &s.Matches[s.CurrentIdx]
}

// PrevMatch steps back one match and wraps past the first, returning nil when there are no matches.
func (s *State) PrevMatch() *MatchLocation {
	if len(s.Matches) == 0 {
		return nil
	}

	s.CurrentIdx--
	if s.CurrentIdx < 0 {
		s.CurrentIdx = len(s.Matches) - 1
	}

	return &s.Matches[s.CurrentIdx]
}

// GetCurrentMatch returns the match CurrentIdx points at, or nil when the index is out of range,
// which covers both a fresh state and a query that found nothing.
func (s *State) GetCurrentMatch() *MatchLocation {
	if s.CurrentIdx >= 0 && s.CurrentIdx < len(s.Matches) {
		return &s.Matches[s.CurrentIdx]
	}

	return nil
}

// MatchCount reports how many hits the last ExecuteSearch found, counting occurrences rather than
// lines, so one line with two hits counts twice.
func (s *State) MatchCount() int {
	return len(s.Matches)
}

// IsLineMatch reports whether any hit falls on one diff line. All three indices are 0-based, and a
// hunkIdx and lineIdx of -1 tests a file-path hit. It walks the whole match list, so cost grows with
// matches times rendered lines.
func (s *State) IsLineMatch(fileIdx, hunkIdx, lineIdx int) bool {
	for _, match := range s.Matches {
		if match.FileIdx == fileIdx && match.HunkIdx == hunkIdx && match.LineIdx == lineIdx {
			return true
		}
	}

	return false
}

// IsCurrentMatch reports whether the hit CurrentIdx points at falls on one diff line, which lets a
// renderer style the active hit apart from the rest. It is false whenever there is no current hit.
func (s *State) IsCurrentMatch(fileIdx, hunkIdx, lineIdx int) bool {
	if s.CurrentIdx < 0 || s.CurrentIdx >= len(s.Matches) {
		return false
	}
	match := s.Matches[s.CurrentIdx]

	return match.FileIdx == fileIdx && match.HunkIdx == hunkIdx && match.LineIdx == lineIdx
}

// GetMatchesForLine returns every hit on one diff line in ascending column order, which is the order
// a renderer needs to highlight the spans in one pass. The result is nil when the line has no hit.
func (s *State) GetMatchesForLine(fileIdx, hunkIdx, lineIdx int) []MatchLocation {
	var matches []MatchLocation
	for _, match := range s.Matches {
		if match.FileIdx == fileIdx && match.HunkIdx == hunkIdx && match.LineIdx == lineIdx {
			matches = append(matches, match)
		}
	}

	return matches
}

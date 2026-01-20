package search

import (
	"strings"

	"github.com/kyleking/jj-diff/internal/diff"
)

type MatchLocation struct {
	FileIdx   int
	HunkIdx   int
	LineIdx   int
	FilePath  string
	StartCol  int
	EndCol    int
	MatchText string
}

type NavigationState struct {
	SelectedFile   int
	SelectedHunk   int
	DiffViewOffset int
	FocusedPanel   int
}

type SearchState struct {
	Query           string
	Matches         []MatchLocation
	CurrentIdx      int
	OriginalState   NavigationState
	IsActive        bool
	IsCaseSensitive bool
}

func NewSearchState() *SearchState {
	return &SearchState{
		Query:           "",
		Matches:         []MatchLocation{},
		CurrentIdx:      -1,
		IsActive:        false,
		IsCaseSensitive: false,
	}
}

func (s *SearchState) SaveOriginalState(nav NavigationState) {
	s.OriginalState = nav
}

func (s *SearchState) RestoreOriginalState() NavigationState {
	return s.OriginalState
}

func (s *SearchState) ExecuteSearch(files []diff.FileChange) {
	s.Matches = []MatchLocation{}
	s.CurrentIdx = -1

	if s.Query == "" {
		return
	}

	query := s.Query
	if !s.IsCaseSensitive {
		query = strings.ToLower(query)
	}

	for fileIdx, file := range files {
		// Search in file path
		filePath := file.Path
		if !s.IsCaseSensitive {
			filePath = strings.ToLower(filePath)
		}
		if idx := strings.Index(filePath, query); idx != -1 {
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

		// Search in diff content
		for hunkIdx, hunk := range file.Hunks {
			for lineIdx, line := range hunk.Lines {
				content := line.Content
				searchContent := content
				if !s.IsCaseSensitive {
					searchContent = strings.ToLower(searchContent)
				}

				idx := 0
				for {
					pos := strings.Index(searchContent[idx:], query)
					if pos == -1 {
						break
					}
					absolutePos := idx + pos

					s.Matches = append(s.Matches, MatchLocation{
						FileIdx:   fileIdx,
						HunkIdx:   hunkIdx,
						LineIdx:   lineIdx,
						FilePath:  file.Path,
						StartCol:  absolutePos,
						EndCol:    absolutePos + len(s.Query),
						MatchText: content,
					})

					idx = absolutePos + 1
				}
			}
		}
	}

	if len(s.Matches) > 0 {
		s.CurrentIdx = 0
	}
}

func (s *SearchState) NextMatch() *MatchLocation {
	if len(s.Matches) == 0 {
		return nil
	}

	s.CurrentIdx = (s.CurrentIdx + 1) % len(s.Matches)
	return &s.Matches[s.CurrentIdx]
}

func (s *SearchState) PrevMatch() *MatchLocation {
	if len(s.Matches) == 0 {
		return nil
	}

	s.CurrentIdx--
	if s.CurrentIdx < 0 {
		s.CurrentIdx = len(s.Matches) - 1
	}
	return &s.Matches[s.CurrentIdx]
}

func (s *SearchState) GetCurrentMatch() *MatchLocation {
	if s.CurrentIdx >= 0 && s.CurrentIdx < len(s.Matches) {
		return &s.Matches[s.CurrentIdx]
	}
	return nil
}

func (s *SearchState) MatchCount() int {
	return len(s.Matches)
}

func (s *SearchState) IsLineMatch(fileIdx, hunkIdx, lineIdx int) bool {
	for _, match := range s.Matches {
		if match.FileIdx == fileIdx && match.HunkIdx == hunkIdx && match.LineIdx == lineIdx {
			return true
		}
	}
	return false
}

func (s *SearchState) IsCurrentMatch(fileIdx, hunkIdx, lineIdx int) bool {
	if s.CurrentIdx < 0 || s.CurrentIdx >= len(s.Matches) {
		return false
	}
	match := s.Matches[s.CurrentIdx]
	return match.FileIdx == fileIdx && match.HunkIdx == hunkIdx && match.LineIdx == lineIdx
}

func (s *SearchState) GetMatchesForLine(fileIdx, hunkIdx, lineIdx int) []MatchLocation {
	var matches []MatchLocation
	for _, match := range s.Matches {
		if match.FileIdx == fileIdx && match.HunkIdx == hunkIdx && match.LineIdx == lineIdx {
			matches = append(matches, match)
		}
	}
	return matches
}

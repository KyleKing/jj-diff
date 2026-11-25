package model

import (
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/jj-diff/internal/components/diffview"
	"github.com/kyleking/jj-diff/internal/components/filelist"
	"github.com/kyleking/jj-diff/internal/search"
)

// cancelSearch is esc out of search: the cursor and focus go back to where the search began.
func (m *Model) cancelSearch() {
	orig := m.searchState.RestoreOriginalState()
	m.selectedFile = orig.SelectedFile
	m.selectedHunk = orig.SelectedHunk
	m.focusedPanel = FocusedPanel(orig.FocusedPanel)
	m.searchModal.Hide()
	m.searchState.IsActive = false
}

func (m *Model) executeSearch() {
	m.searchState.ExecuteSearch(m.changes)
	m.searchState.IsActive = true
	m.searchModal.UpdateResults(m.searchState.MatchCount(), m.searchState.CurrentIdx)

	if match := m.searchState.GetCurrentMatch(); match != nil {
		m.selectedFile = match.FileIdx
		if match.HunkIdx >= 0 {
			m.selectedHunk = match.HunkIdx
		}
		m.focusedPanel = PanelDiffView
		m.fileList.SetSelected(m.selectedFile)
		if m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
			m.diffView.SetFileChange(m.changes[m.selectedFile])
		}
	}
}

func (m *Model) nextSearchMatch() tea.Cmd {
	return m.jumpToMatch(m.searchState.NextMatch())
}

func (m *Model) prevSearchMatch() tea.Cmd {
	return m.jumpToMatch(m.searchState.PrevMatch())
}

// jumpToMatch moves the cursor and the diff view onto match, leaving the model untouched when match
// is nil so a search that ran off the end is a no-op rather than a jump to file zero.
func (m *Model) jumpToMatch(match *search.MatchLocation) tea.Cmd {
	if match != nil {
		m.searchModal.UpdateResults(m.searchState.MatchCount(), m.searchState.CurrentIdx)
		m.selectedFile = match.FileIdx
		if match.HunkIdx >= 0 {
			m.selectedHunk = match.HunkIdx
		}
		m.focusedPanel = PanelDiffView
		m.fileList.SetSelected(m.selectedFile)
		if m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
			m.diffView.SetFileChange(m.changes[m.selectedFile])
		}
	}

	return nil
}

func (m *Model) getFilePathMatches(fileIdx int) []filelist.MatchRange {
	if m.searchState == nil || !m.searchState.IsActive {
		return nil
	}

	var ranges []filelist.MatchRange
	for _, match := range m.searchState.Matches {
		// Only include matches that are in file paths (HunkIdx == -1)
		if match.FileIdx == fileIdx && match.HunkIdx == -1 {
			ranges = append(ranges, filelist.MatchRange{
				Start: match.StartCol,
				End:   match.EndCol,
			})
		}
	}

	return ranges
}

func (m *Model) getLineContentMatches(filePath string, hunkIdx, lineIdx int) []diffview.MatchRange {
	if m.searchState == nil || !m.searchState.IsActive {
		return nil
	}

	return m.matchIndex().lines[lineKey{path: filePath, hunk: hunkIdx, line: lineIdx}]
}

// matchIndex is the current search's hits by line and by file, built once per search. The diff pane
// asks for a line's hits on every row of every frame, which scanning the whole match list for made
// a frame cost a pass over every hit per row.
type matchIndex struct {
	lines map[lineKey][]diffview.MatchRange
	hits  map[string][]diffview.SearchHit
	// from is the match slice the index was built from, told apart by its first element, which the
	// index keeps alive, so a later search's slice can never share its address.
	from  *search.MatchLocation
	count int
}

type lineKey struct {
	path string
	hunk int
	line int
}

func (m *Model) matchIndex() *matchIndex {
	matches := m.searchState.Matches

	var from *search.MatchLocation
	if len(matches) > 0 {
		from = &matches[0]
	}

	if idx := m.searchIndex; idx != nil && idx.lines != nil && idx.from == from && idx.count == len(matches) {
		return idx
	}

	idx := &matchIndex{
		lines: map[lineKey][]diffview.MatchRange{}, hits: map[string][]diffview.SearchHit{},
		from: from, count: len(matches),
	}

	for _, match := range matches {
		if match.HunkIdx < 0 {
			continue
		}

		key := lineKey{path: match.FilePath, hunk: match.HunkIdx, line: match.LineIdx}
		idx.lines[key] = append(idx.lines[key], diffview.MatchRange{Start: match.StartCol, End: match.EndCol})
		idx.hits[match.FilePath] = append(idx.hits[match.FilePath], diffview.SearchHit{
			Hunk: match.HunkIdx, Line: match.LineIdx,
		})
	}

	if m.searchIndex != nil {
		*m.searchIndex = *idx
	}

	return idx
}

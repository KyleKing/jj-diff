package model

import "github.com/kyleking/jj-diff/internal/diff"

// HunkSelection is one hunk's selection. WholeHunk wins over SelectedLines, and selecting the whole
// hunk discards the per-line set, so the two are never both meaningful.
type HunkSelection struct {
	SelectedLines map[int]bool
	WholeHunk     bool
}

// FileSelection holds one file's selected hunks, keyed by the hunk's index in the parsed file. The
// indices go stale when the diff is reloaded.
type FileSelection struct {
	Hunks map[int]*HunkSelection
}

// SelectionState is what the user has picked across every file, keyed by the diff's path. Build it
// with NewSelectionState, because the mutators assume the map exists.
type SelectionState struct {
	Files map[string]*FileSelection
}

// NewSelectionState returns an empty selection.
func NewSelectionState() *SelectionState {
	return &SelectionState{
		Files: make(map[string]*FileSelection),
	}
}

// IsHunkSelected reports whether the whole hunk is selected, which is false for a hunk that only has
// individual lines picked.
func (s *SelectionState) IsHunkSelected(filePath string, hunkIdx int) bool {
	if fileSelection, ok := s.Files[filePath]; ok {
		if hunkSelection, ok := fileSelection.Hunks[hunkIdx]; ok {
			return hunkSelection.WholeHunk
		}
	}

	return false
}

// IsLineSelected reports whether one line is selected, which is true for every line of a hunk
// selected as a whole.
func (s *SelectionState) IsLineSelected(filePath string, hunkIdx, lineIdx int) bool {
	if fileSelection, ok := s.Files[filePath]; ok {
		if hunkSelection, ok := fileSelection.Hunks[hunkIdx]; ok {
			if hunkSelection.WholeHunk {
				return true
			}

			return hunkSelection.SelectedLines[lineIdx]
		}
	}

	return false
}

// ToggleHunk flips whole-hunk selection, creating the file and hunk entries as needed. Selecting a
// hunk discards any lines picked inside it, so a toggle out and back in loses the line selection.
func (s *SelectionState) ToggleHunk(filePath string, hunkIdx int) {
	if _, ok := s.Files[filePath]; !ok {
		s.Files[filePath] = &FileSelection{
			Hunks: make(map[int]*HunkSelection),
		}
	}

	fileSelection := s.Files[filePath]
	if _, ok := fileSelection.Hunks[hunkIdx]; !ok {
		fileSelection.Hunks[hunkIdx] = &HunkSelection{
			SelectedLines: make(map[int]bool),
		}
	}

	hunkSelection := fileSelection.Hunks[hunkIdx]
	hunkSelection.WholeHunk = !hunkSelection.WholeHunk
	if hunkSelection.WholeHunk {
		hunkSelection.SelectedLines = make(map[int]bool)
	}
}

// ToggleLine flips one line's selection. It does nothing while the hunk is selected as a whole,
// because that state has no per-line detail to change.
func (s *SelectionState) ToggleLine(filePath string, hunkIdx, lineIdx int) {
	if _, ok := s.Files[filePath]; !ok {
		s.Files[filePath] = &FileSelection{
			Hunks: make(map[int]*HunkSelection),
		}
	}

	fileSelection := s.Files[filePath]
	if _, ok := fileSelection.Hunks[hunkIdx]; !ok {
		fileSelection.Hunks[hunkIdx] = &HunkSelection{
			SelectedLines: make(map[int]bool),
		}
	}

	hunkSelection := fileSelection.Hunks[hunkIdx]
	if hunkSelection.WholeHunk {
		return
	}

	hunkSelection.SelectedLines[lineIdx] = !hunkSelection.SelectedLines[lineIdx]
}

// SelectLineRange selects an inclusive range of lines, accepting the bounds in either order. It
// clears whole-hunk selection, and it only adds, so lines already selected outside the range stay.
func (s *SelectionState) SelectLineRange(filePath string, hunkIdx, startLine, endLine int) {
	if startLine > endLine {
		startLine, endLine = endLine, startLine
	}

	if _, ok := s.Files[filePath]; !ok {
		s.Files[filePath] = &FileSelection{
			Hunks: make(map[int]*HunkSelection),
		}
	}

	fileSelection := s.Files[filePath]
	if _, ok := fileSelection.Hunks[hunkIdx]; !ok {
		fileSelection.Hunks[hunkIdx] = &HunkSelection{
			SelectedLines: make(map[int]bool),
		}
	}

	hunkSelection := fileSelection.Hunks[hunkIdx]
	hunkSelection.WholeHunk = false

	for i := startLine; i <= endLine; i++ {
		hunkSelection.SelectedLines[i] = true
	}
}

// HasPartialSelection reports whether a hunk has lines picked without being selected as a whole,
// which is what the renderer draws the partial marker for.
func (s *SelectionState) HasPartialSelection(filePath string, hunkIdx int) bool {
	if fileSelection, ok := s.Files[filePath]; ok {
		if hunkSelection, ok := fileSelection.Hunks[hunkIdx]; ok {
			return !hunkSelection.WholeHunk && len(hunkSelection.SelectedLines) > 0
		}
	}

	return false
}

// Clone is a deep copy, which is what the undo history keeps so a later edit cannot reach back into
// an earlier state.
func (s *SelectionState) Clone() *SelectionState {
	out := NewSelectionState()

	for path, file := range s.Files {
		hunks := make(map[int]*HunkSelection, len(file.Hunks))

		for idx, h := range file.Hunks {
			lines := make(map[int]bool, len(h.SelectedLines))
			for l, on := range h.SelectedLines {
				lines[l] = on
			}

			hunks[idx] = &HunkSelection{SelectedLines: lines, WholeHunk: h.WholeHunk}
		}

		out.Files[path] = &FileSelection{Hunks: hunks}
	}

	return out
}

// IsEmpty reports whether nothing at all is selected.
func (s *SelectionState) IsEmpty() bool {
	for _, file := range s.Files {
		for _, h := range file.Hunks {
			if h.WholeHunk {
				return false
			}

			for _, on := range h.SelectedLines {
				if on {
					return false
				}
			}
		}
	}

	return true
}

// Invert flips the selection within one file: a whole hunk goes, an untouched hunk comes in whole,
// and a hunk with some lines picked swaps them for the changed lines it left out.
func (s *SelectionState) Invert(file diff.FileChange) {
	for idx, hunk := range file.Hunks {
		switch {
		case s.IsHunkSelected(file.Path, idx):
			s.ToggleHunk(file.Path, idx)
		case s.HasPartialSelection(file.Path, idx):
			sel := s.Files[file.Path].Hunks[idx]
			flipped := make(map[int]bool)

			for lineIdx, line := range hunk.Lines {
				if line.Type != diff.LineContext && !sel.SelectedLines[lineIdx] {
					flipped[lineIdx] = true
				}
			}

			sel.SelectedLines = flipped
		default:
			s.ToggleHunk(file.Path, idx)
		}
	}
}

// Only is a selection holding just one hunk, which is what a verb acts on when nothing is selected.
func Only(path string, hunkIdx int) *SelectionState {
	s := NewSelectionState()
	s.ToggleHunk(path, hunkIdx)

	return s
}

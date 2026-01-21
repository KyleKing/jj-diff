package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Applier handles writing user selections back to the right directory.
// For diff-editor mode, jj expects the right directory to contain
// only the changes the user wants to keep.
type Applier struct {
	LeftDir  string
	RightDir string
}

func NewApplier(leftDir, rightDir string) *Applier {
	return &Applier{
		LeftDir:  leftDir,
		RightDir: rightDir,
	}
}

// ApplySelections reconstructs files in rightDir based on user selections.
// - Selected hunks: keep the change (content from right)
// - Unselected hunks: revert the change (content from left)
func (a *Applier) ApplySelections(files []FileChange, selection SelectionState) error {
	for _, file := range files {
		if err := a.applyFileSelections(file, selection); err != nil {
			return fmt.Errorf("applying selections for %s: %w", file.Path, err)
		}
	}
	return nil
}

func (a *Applier) applyFileSelections(file FileChange, selection SelectionState) error {
	leftPath := filepath.Join(a.LeftDir, file.Path)
	rightPath := filepath.Join(a.RightDir, file.Path)

	hasAnySelection := false
	for hunkIdx := range file.Hunks {
		if selection.IsHunkSelected(file.Path, hunkIdx) ||
			selection.HasPartialSelection(file.Path, hunkIdx) {
			hasAnySelection = true
			break
		}
	}

	switch file.ChangeType {
	case ChangeTypeAdded:
		return a.handleAddedFile(file, rightPath, selection, hasAnySelection)
	case ChangeTypeDeleted:
		return a.handleDeletedFile(file, leftPath, rightPath, selection, hasAnySelection)
	case ChangeTypeModified, ChangeTypeRenamed:
		return a.handleModifiedFile(file, leftPath, rightPath, selection)
	}

	return nil
}

func (a *Applier) handleAddedFile(file FileChange, rightPath string, selection SelectionState, hasSelection bool) error {
	if !hasSelection {
		return os.Remove(rightPath)
	}

	rightContent, err := os.ReadFile(rightPath)
	if err != nil {
		return err
	}

	reconstructed := a.reconstructAddedFile(file, string(rightContent), selection)
	return a.writeFile(rightPath, reconstructed)
}

func (a *Applier) handleDeletedFile(file FileChange, leftPath, rightPath string, selection SelectionState, hasSelection bool) error {
	if !hasSelection {
		leftContent, err := os.ReadFile(leftPath)
		if err != nil {
			return err
		}
		return a.writeFile(rightPath, string(leftContent))
	}

	leftContent, err := os.ReadFile(leftPath)
	if err != nil {
		return err
	}

	reconstructed := a.reconstructDeletedFile(file, string(leftContent), selection)
	if reconstructed == "" {
		return os.Remove(rightPath)
	}
	return a.writeFile(rightPath, reconstructed)
}

func (a *Applier) handleModifiedFile(file FileChange, leftPath, rightPath string, selection SelectionState) error {
	leftContent, err := os.ReadFile(leftPath)
	if err != nil {
		return err
	}

	rightContent, err := os.ReadFile(rightPath)
	if err != nil {
		return err
	}

	reconstructed := a.reconstructModifiedFile(file, string(leftContent), string(rightContent), selection)
	return a.writeFile(rightPath, reconstructed)
}

func (a *Applier) reconstructAddedFile(file FileChange, rightContent string, selection SelectionState) string {
	rightLines := strings.Split(rightContent, "\n")
	result := make([]string, 0)

	for hunkIdx, hunk := range file.Hunks {
		isSelected := selection.IsHunkSelected(file.Path, hunkIdx)
		hasPartial := selection.HasPartialSelection(file.Path, hunkIdx)

		for lineIdx, line := range hunk.Lines {
			if line.Type == LineAddition {
				keep := isSelected || (hasPartial && selection.IsLineSelected(file.Path, hunkIdx, lineIdx))
				if keep {
					lineNum := line.NewLineNum - 1
					if lineNum >= 0 && lineNum < len(rightLines) {
						result = append(result, rightLines[lineNum])
					}
				}
			}
		}
	}

	return strings.Join(result, "\n")
}

func (a *Applier) reconstructDeletedFile(file FileChange, leftContent string, selection SelectionState) string {
	leftLines := strings.Split(leftContent, "\n")
	deletedLineNums := make(map[int]bool)

	for hunkIdx, hunk := range file.Hunks {
		isSelected := selection.IsHunkSelected(file.Path, hunkIdx)
		hasPartial := selection.HasPartialSelection(file.Path, hunkIdx)

		for lineIdx, line := range hunk.Lines {
			if line.Type == LineDeletion {
				keep := isSelected || (hasPartial && selection.IsLineSelected(file.Path, hunkIdx, lineIdx))
				if keep {
					deletedLineNums[line.OldLineNum] = true
				}
			}
		}
	}

	result := make([]string, 0)
	for i, line := range leftLines {
		lineNum := i + 1
		if !deletedLineNums[lineNum] {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

func (a *Applier) reconstructModifiedFile(file FileChange, leftContent, rightContent string, selection SelectionState) string {
	leftLines := strings.Split(leftContent, "\n")
	rightLines := strings.Split(rightContent, "\n")

	type lineAction struct {
		lineNum  int
		action   string
		content  string
		selected bool
	}

	var actions []lineAction

	for hunkIdx, hunk := range file.Hunks {
		isSelected := selection.IsHunkSelected(file.Path, hunkIdx)
		hasPartial := selection.HasPartialSelection(file.Path, hunkIdx)

		for lineIdx, line := range hunk.Lines {
			selected := isSelected || (hasPartial && selection.IsLineSelected(file.Path, hunkIdx, lineIdx))

			switch line.Type {
			case LineDeletion:
				actions = append(actions, lineAction{
					lineNum:  line.OldLineNum,
					action:   "delete",
					content:  line.Content,
					selected: selected,
				})
			case LineAddition:
				actions = append(actions, lineAction{
					lineNum:  line.NewLineNum,
					action:   "add",
					content:  line.Content,
					selected: selected,
				})
			}
		}
	}

	deletions := make(map[int]bool)
	additions := make(map[int][]string)

	for _, act := range actions {
		if act.action == "delete" && act.selected {
			deletions[act.lineNum] = true
		} else if act.action == "add" && act.selected {
			additions[act.lineNum] = append(additions[act.lineNum], act.content)
		}
	}

	result := make([]string, 0, len(leftLines))
	rightIdx := 0

	for i, line := range leftLines {
		oldLineNum := i + 1

		if deletions[oldLineNum] {
			rightIdx++
			continue
		}

		newLineNum := rightIdx + 1
		if adds, ok := additions[newLineNum]; ok {
			result = append(result, adds...)
			rightIdx += len(adds)
		}

		result = append(result, line)
		rightIdx++
	}

	finalLineNum := len(rightLines)
	if adds, ok := additions[finalLineNum]; ok {
		result = append(result, adds...)
	}

	return strings.Join(result, "\n")
}

func (a *Applier) writeFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	if !strings.HasSuffix(content, "\n") && content != "" {
		content += "\n"
	}

	return os.WriteFile(path, []byte(content), 0644)
}

// SelectAll selects all hunks in all files.
// Used when user wants to keep all changes (default behavior).
func SelectAll(files []FileChange, selection interface {
	ToggleHunk(filePath string, hunkIdx int)
}) {
	for _, file := range files {
		for hunkIdx := range file.Hunks {
			selection.ToggleHunk(file.Path, hunkIdx)
		}
	}
}

// GetUnselectedFiles returns paths of files with no selections.
// These files should be restored to their left state.
func GetUnselectedFiles(files []FileChange, selection SelectionState) []string {
	var unselected []string
	for _, file := range files {
		hasSelection := false
		for hunkIdx := range file.Hunks {
			if selection.IsHunkSelected(file.Path, hunkIdx) ||
				selection.HasPartialSelection(file.Path, hunkIdx) {
				hasSelection = true
				break
			}
		}
		if !hasSelection {
			unselected = append(unselected, file.Path)
		}
	}
	sort.Strings(unselected)
	return unselected
}

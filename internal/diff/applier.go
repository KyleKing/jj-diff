package diff

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrPathEscapesBase reports a diff path that resolves outside the directory it
// was joined to.
var ErrPathEscapesBase = errors.New("path escapes its base directory")

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
// - Unselected hunks: revert the change (content from left).
func (a *Applier) ApplySelections(files []FileChange, selection SelectionState) error {
	for _, file := range files {
		if err := a.applyFileSelections(file, selection); err != nil {
			return fmt.Errorf("applying selections for %s: %w", file.Path, err)
		}
	}

	return nil
}

// containedPath joins a diff path onto a base directory and rejects any result
// that escapes it, so a crafted diff cannot write outside the directories jj
// hands the diff editor.
func containedPath(baseDir, relPath string) (string, error) {
	joined := filepath.Join(baseDir, relPath)

	rel, err := filepath.Rel(baseDir, joined)
	if err != nil {
		return "", fmt.Errorf("resolving %q under %q: %w", relPath, baseDir, err)
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q under %q", ErrPathEscapesBase, relPath, baseDir)
	}

	return joined, nil
}

func (a *Applier) applyFileSelections(file FileChange, selection SelectionState) error {
	leftPath, err := containedPath(a.LeftDir, file.Path)
	if err != nil {
		return err
	}

	rightPath, err := containedPath(a.RightDir, file.Path)
	if err != nil {
		return err
	}

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

func (a *Applier) handleAddedFile(
	file FileChange,
	rightPath string,
	selection SelectionState,
	hasSelection bool,
) error {
	if !hasSelection {
		return os.Remove(rightPath)
	}

	//nolint:gosec // G304: paths come from the directories jj hands the diff editor.
	rightContent, err := os.ReadFile(rightPath)
	if err != nil {
		return err
	}

	reconstructed := a.reconstructAddedFile(file, string(rightContent), selection)

	return a.writeFile(rightPath, reconstructed)
}

func (a *Applier) handleDeletedFile(
	file FileChange,
	leftPath, rightPath string,
	selection SelectionState,
	hasSelection bool,
) error {
	if !hasSelection {
		//nolint:gosec // G304: paths come from the directories jj hands the diff editor.
		leftContent, err := os.ReadFile(leftPath)
		if err != nil {
			return err
		}

		return a.writeFile(rightPath, string(leftContent))
	}

	//nolint:gosec // G304: paths come from the directories jj hands the diff editor.
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

func (a *Applier) handleModifiedFile(
	file FileChange,
	leftPath, rightPath string,
	selection SelectionState,
) error {
	//nolint:gosec // G304: paths come from the directories jj hands the diff editor.
	leftContent, err := os.ReadFile(leftPath)
	if err != nil {
		return err
	}

	reconstructed := a.reconstructModifiedFile(
		file,
		string(leftContent),
		selection,
	)

	return a.writeFile(rightPath, reconstructed)
}

func (a *Applier) reconstructAddedFile(
	file FileChange,
	rightContent string,
	selection SelectionState,
) string {
	rightLines := strings.Split(rightContent, "\n")
	result := make([]string, 0)

	for hunkIdx, hunk := range file.Hunks {
		isSelected := selection.IsHunkSelected(file.Path, hunkIdx)
		hasPartial := selection.HasPartialSelection(file.Path, hunkIdx)

		for lineIdx, line := range hunk.Lines {
			if line.Type == LineAddition {
				keep := isSelected ||
					(hasPartial && selection.IsLineSelected(file.Path, hunkIdx, lineIdx))
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

func (a *Applier) reconstructDeletedFile(
	file FileChange,
	leftContent string,
	selection SelectionState,
) string {
	leftLines := strings.Split(leftContent, "\n")
	deletedLineNums := make(map[int]bool)

	for hunkIdx, hunk := range file.Hunks {
		isSelected := selection.IsHunkSelected(file.Path, hunkIdx)
		hasPartial := selection.HasPartialSelection(file.Path, hunkIdx)

		for lineIdx, line := range hunk.Lines {
			if line.Type == LineDeletion {
				keep := isSelected ||
					(hasPartial && selection.IsLineSelected(file.Path, hunkIdx, lineIdx))
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

// reconstructModifiedFile replays the hunks over the left content, keeping a
// change only where it is selected. Unselected deletions stay, unselected
// additions are dropped, so an empty selection reproduces the left file exactly
// and a full selection reproduces the right file exactly.
func (a *Applier) reconstructModifiedFile(
	file FileChange,
	leftContent string,
	selection SelectionState,
) string {
	leftLines := strings.Split(leftContent, "\n")
	result := make([]string, 0, len(leftLines))
	oldIdx := 0

	for hunkIdx, hunk := range file.Hunks {
		isSelected := selection.IsHunkSelected(file.Path, hunkIdx)
		hasPartial := selection.HasPartialSelection(file.Path, hunkIdx)

		for oldIdx < hunk.OldStart-1 && oldIdx < len(leftLines) {
			result = append(result, leftLines[oldIdx])
			oldIdx++
		}

		for lineIdx, line := range hunk.Lines {
			selected := isSelected ||
				(hasPartial && selection.IsLineSelected(file.Path, hunkIdx, lineIdx))

			switch line.Type {
			case LineContext:
				result = append(result, line.Content)
				oldIdx++
			case LineDeletion:
				if !selected {
					result = append(result, line.Content)
				}

				oldIdx++
			case LineAddition:
				if selected {
					result = append(result, line.Content)
				}
			}
		}
	}

	for oldIdx < len(leftLines) {
		result = append(result, leftLines[oldIdx])
		oldIdx++
	}

	return strings.Join(result, "\n")
}

func (a *Applier) writeFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	if !strings.HasSuffix(content, "\n") && content != "" {
		content += "\n"
	}

	//nolint:gosec // G703: callers resolve path through containedPath first.
	return os.WriteFile(path, []byte(content), 0o600)
}

// SelectAll selects all hunks in all files.
// Used when user wants to keep all changes (default behavior).
func SelectAll(files []FileChange, selection interface {
	ToggleHunk(filePath string, hunkIdx int)
},
) {
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

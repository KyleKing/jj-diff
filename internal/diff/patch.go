package diff

import (
	"fmt"
	"strings"
)

// SelectionState interface for accessing selection state
type SelectionState interface {
	IsHunkSelected(filePath string, hunkIdx int) bool
	HasPartialSelection(filePath string, hunkIdx int) bool
	IsLineSelected(filePath string, hunkIdx, lineIdx int) bool
}

func GeneratePatch(files []FileChange, selection SelectionState) string {
	var patch strings.Builder

	for _, file := range files {
		var fileHunks []string

		for hunkIdx, hunk := range file.Hunks {
			if selection.IsHunkSelected(file.Path, hunkIdx) {
				// Whole hunk selected
				fileHunks = append(fileHunks, renderWholeHunk(hunk))
			} else if selection.HasPartialSelection(file.Path, hunkIdx) {
				// Partial lines selected
				partialHunk := renderPartialHunk(hunk, hunkIdx, file.Path, selection)
				if partialHunk != "" {
					fileHunks = append(fileHunks, partialHunk)
				}
			}
		}

		if len(fileHunks) == 0 {
			continue
		}

		// Write file headers
		patch.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", file.Path, file.Path))

		switch file.ChangeType {
		case ChangeTypeAdded:
			patch.WriteString("new file mode 100644\n")
			patch.WriteString("--- /dev/null\n")
			patch.WriteString(fmt.Sprintf("+++ b/%s\n", file.Path))
		case ChangeTypeDeleted:
			patch.WriteString("deleted file mode 100644\n")
			patch.WriteString(fmt.Sprintf("--- a/%s\n", file.Path))
			patch.WriteString("+++ /dev/null\n")
		default:
			patch.WriteString(fmt.Sprintf("--- a/%s\n", file.Path))
			patch.WriteString(fmt.Sprintf("+++ b/%s\n", file.Path))
		}

		// Write hunks
		for _, hunkStr := range fileHunks {
			patch.WriteString(hunkStr)
		}
	}

	return patch.String()
}

func renderWholeHunk(hunk Hunk) string {
	var buf strings.Builder
	buf.WriteString(hunk.Header)
	buf.WriteString("\n")

	for _, line := range hunk.Lines {
		buf.WriteString(line.Type.String())
		buf.WriteString(line.Content)
		buf.WriteString("\n")
	}

	return buf.String()
}

func renderPartialHunk(hunk Hunk, hunkIdx int, filePath string, selection SelectionState) string {
	// Build selected lines with context
	selectedLines := make(map[int]bool)
	for lineIdx := range hunk.Lines {
		if selection.IsLineSelected(filePath, hunkIdx, lineIdx) {
			selectedLines[lineIdx] = true
		}
	}

	if len(selectedLines) == 0 {
		return ""
	}

	// Expand to include context lines (3 before, 3 after)
	const contextLines = 3
	expandedSelection := expandWithContext(selectedLines, len(hunk.Lines), contextLines)

	// Build lines
	var lines []Line
	for lineIdx, line := range hunk.Lines {
		if expandedSelection[lineIdx] {
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return ""
	}

	// Recalculate hunk header
	newHeader := recalculateHunkHeader(lines)

	var buf strings.Builder
	buf.WriteString(newHeader)
	buf.WriteString("\n")

	for _, line := range lines {
		buf.WriteString(line.Type.String())
		buf.WriteString(line.Content)
		buf.WriteString("\n")
	}

	return buf.String()
}

func expandWithContext(selected map[int]bool, totalLines, contextLines int) map[int]bool {
	expanded := make(map[int]bool)

	for idx := range selected {
		// Add selected line
		expanded[idx] = true

		// Add context before
		for i := idx - contextLines; i < idx; i++ {
			if i >= 0 {
				expanded[i] = true
			}
		}

		// Add context after
		for i := idx + 1; i <= idx+contextLines; i++ {
			if i < totalLines {
				expanded[i] = true
			}
		}
	}

	return expanded
}

func recalculateHunkHeader(selectedLines []Line) string {
	if len(selectedLines) == 0 {
		return "@@ -0,0 +0,0 @@"
	}

	// Calculate new line counts
	oldCount := 0
	newCount := 0
	var oldStart, newStart int
	firstLine := true

	for _, line := range selectedLines {
		if firstLine {
			oldStart = line.OldLineNum
			newStart = line.NewLineNum
			firstLine = false
		}

		switch line.Type {
		case LineContext:
			oldCount++
			newCount++
		case LineDeletion:
			oldCount++
		case LineAddition:
			newCount++
		}
	}

	return fmt.Sprintf("@@ -%d,%d +%d,%d @@", oldStart, oldCount, newStart, newCount)
}

// GetSelectedHunksMap is deprecated but kept for backward compatibility
func GetSelectedHunksMap(files []FileChange, selectionState interface {
	IsHunkSelected(filePath string, hunkIdx int) bool
}) map[string]map[int]bool {
	result := make(map[string]map[int]bool)

	for _, file := range files {
		fileHunks := make(map[int]bool)
		for hunkIdx := range file.Hunks {
			if selectionState.IsHunkSelected(file.Path, hunkIdx) {
				fileHunks[hunkIdx] = true
			}
		}

		if len(fileHunks) > 0 {
			result[file.Path] = fileHunks
		}
	}

	return result
}

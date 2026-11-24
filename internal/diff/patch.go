package diff

import (
	"fmt"
	"strings"
)

// SelectionState interface for accessing selection state.
type SelectionState interface {
	IsHunkSelected(filePath string, hunkIdx int) bool
	HasPartialSelection(filePath string, hunkIdx int) bool
	IsLineSelected(filePath string, hunkIdx, lineIdx int) bool
}

// GeneratePatch renders the selected hunks as a unified diff that git apply accepts against the
// diff's old side. A hunk with only some lines selected keeps the whole hunk as context: an
// unselected deletion stays as a context line, since the line is still there, and an unselected
// addition is dropped, since it never arrives. Headers are recounted for every hunk, and each hunk's
// new-side start follows what the selected hunks above it in the file added and removed. A file with
// nothing selected is left out, so an empty selection returns the empty string.
func GeneratePatch(files []FileChange, selection SelectionState) string {
	var patch strings.Builder

	for _, file := range files {
		var fileHunks strings.Builder

		delta := 0

		for hunkIdx, hunk := range file.Hunks {
			whole := selection.IsHunkSelected(file.Path, hunkIdx)
			if !whole && !selection.HasPartialSelection(file.Path, hunkIdx) {
				continue
			}

			picked := func(lineIdx int) bool {
				return whole || selection.IsLineSelected(file.Path, hunkIdx, lineIdx)
			}

			delta += renderHunk(&fileHunks, hunk, picked, delta)
		}

		if fileHunks.Len() == 0 {
			continue
		}

		writeFileHeader(&patch, file)
		patch.WriteString(fileHunks.String())
	}

	return patch.String()
}

func writeFileHeader(patch *strings.Builder, file FileChange) {
	fmt.Fprintf(patch, "diff --git a/%s b/%s\n", file.Path, file.Path)

	switch file.ChangeType {
	case ChangeTypeAdded:
		patch.WriteString("new file mode 100644\n")
		patch.WriteString("--- /dev/null\n")
		fmt.Fprintf(patch, "+++ b/%s\n", file.Path)
	case ChangeTypeDeleted:
		patch.WriteString("deleted file mode 100644\n")
		fmt.Fprintf(patch, "--- a/%s\n", file.Path)
		patch.WriteString("+++ /dev/null\n")
	case ChangeTypeModified, ChangeTypeRenamed:
		fmt.Fprintf(patch, "--- a/%s\n", file.Path)
		fmt.Fprintf(patch, "+++ b/%s\n", file.Path)
	}
}

// renderHunk writes hunk with only the picked changes left as changes, and answers how many lines it
// adds net, which shifts the new-side start of every hunk below it. A hunk with no picked change
// writes nothing.
func renderHunk(out *strings.Builder, hunk Hunk, picked func(lineIdx int) bool, delta int) int {
	var body strings.Builder

	oldCount, newCount, changes := 0, 0, 0

	for lineIdx, line := range hunk.Lines {
		kind := line.Type

		switch {
		case kind == LineAddition && !picked(lineIdx):
			continue
		case kind == LineDeletion && !picked(lineIdx):
			kind = LineContext
		case kind != LineContext:
			changes++
		}

		if kind != LineAddition {
			oldCount++
		}

		if kind != LineDeletion {
			newCount++
		}

		body.WriteString(kind.String())
		body.WriteString(line.Content)
		body.WriteString("\n")

		if line.NoNewline {
			body.WriteString(noNewlineMarker + "\n")
		}
	}

	if changes == 0 {
		return 0
	}

	writeHunkHeader(out, hunk.OldStart, oldCount, newCount, delta)
	out.WriteString(body.String())

	return newCount - oldCount
}

// writeHunkHeader writes a recounted header, the new side's start shifted by delta. A side with no
// lines names the line before where the hunk sits, as diff writes it.
func writeHunkHeader(out *strings.Builder, oldStart, oldCount, newCount, delta int) {
	if oldStart == 0 && oldCount > 0 {
		oldStart = 1
	}

	newStart := oldStart + delta

	switch {
	case oldCount == 0 && newCount > 0:
		newStart++
	case newCount == 0 && oldCount > 0:
		newStart--
	}

	fmt.Fprintf(out, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, max(newStart, 0), newCount)
}

// GetSelectedHunksMap is deprecated but kept for backward compatibility.
func GetSelectedHunksMap(files []FileChange, selectionState interface {
	IsHunkSelected(filePath string, hunkIdx int) bool
},
) map[string]map[int]bool {
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

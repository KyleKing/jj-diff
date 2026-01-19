package diff

import (
	"fmt"
	"strings"
)

func GeneratePatch(files []FileChange, selection map[string]map[int]bool) string {
	var patch strings.Builder

	for _, file := range files {
		selectedHunks, hasSelection := selection[file.Path]
		if !hasSelection || len(selectedHunks) == 0 {
			continue
		}

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

		for hunkIdx, hunk := range file.Hunks {
			if !selectedHunks[hunkIdx] {
				continue
			}

			patch.WriteString(hunk.Header)
			patch.WriteString("\n")

			for _, line := range hunk.Lines {
				patch.WriteString(line.Type.String())
				patch.WriteString(line.Content)
				patch.WriteString("\n")
			}
		}
	}

	return patch.String()
}

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

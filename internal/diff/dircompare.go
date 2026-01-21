package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	godiff "github.com/sergi/go-diff/diffmatchpatch"
)

// CompareDirectories generates a unified diff comparing two directories.
// Returns git-format diff text suitable for parsing by diff.Parse().
func CompareDirectories(leftDir, rightDir string) (string, error) {
	leftFiles, err := walkDirectory(leftDir)
	if err != nil {
		return "", fmt.Errorf("walking left directory: %w", err)
	}

	rightFiles, err := walkDirectory(rightDir)
	if err != nil {
		return "", fmt.Errorf("walking right directory: %w", err)
	}

	allPaths := mergeFilePaths(leftFiles, rightFiles)
	sort.Strings(allPaths)

	var diffBuilder strings.Builder
	for _, path := range allPaths {
		leftPath := filepath.Join(leftDir, path)
		rightPath := filepath.Join(rightDir, path)

		inLeft := leftFiles[path]
		inRight := rightFiles[path]

		fileDiff, err := generateFileDiff(path, leftPath, rightPath, inLeft, inRight)
		if err != nil {
			return "", fmt.Errorf("generating diff for %s: %w", path, err)
		}
		if fileDiff != "" {
			diffBuilder.WriteString(fileDiff)
		}
	}

	return diffBuilder.String(), nil
}

func walkDirectory(dir string) (map[string]bool, error) {
	files := make(map[string]bool)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		files[relPath] = true
		return nil
	})

	return files, err
}

func mergeFilePaths(left, right map[string]bool) []string {
	seen := make(map[string]bool)
	for path := range left {
		seen[path] = true
	}
	for path := range right {
		seen[path] = true
	}

	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	return paths
}

func generateFileDiff(relPath, leftPath, rightPath string, inLeft, inRight bool) (string, error) {
	var leftContent, rightContent string
	var err error

	if inLeft {
		leftContent, err = readFileContent(leftPath)
		if err != nil {
			return "", err
		}
	}

	if inRight {
		rightContent, err = readFileContent(rightPath)
		if err != nil {
			return "", err
		}
	}

	if leftContent == rightContent {
		return "", nil
	}

	return generateUnifiedDiff(relPath, leftContent, rightContent, inLeft, inRight), nil
}

func readFileContent(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func generateUnifiedDiff(path, leftContent, rightContent string, inLeft, inRight bool) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", path, path))

	if !inLeft {
		builder.WriteString("new file mode 100644\n")
		builder.WriteString("--- /dev/null\n")
		builder.WriteString(fmt.Sprintf("+++ b/%s\n", path))
		builder.WriteString(generateAddedFileHunks(rightContent))
		return builder.String()
	}

	if !inRight {
		builder.WriteString("deleted file mode 100644\n")
		builder.WriteString(fmt.Sprintf("--- a/%s\n", path))
		builder.WriteString("+++ /dev/null\n")
		builder.WriteString(generateDeletedFileHunks(leftContent))
		return builder.String()
	}

	builder.WriteString(fmt.Sprintf("--- a/%s\n", path))
	builder.WriteString(fmt.Sprintf("+++ b/%s\n", path))
	builder.WriteString(generateModifiedFileHunks(leftContent, rightContent))
	return builder.String()
}

func generateAddedFileHunks(content string) string {
	lines := splitLines(content)
	if len(lines) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("@@ -0,0 +1,%d @@\n", len(lines)))
	for _, line := range lines {
		builder.WriteString("+")
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	return builder.String()
}

func generateDeletedFileHunks(content string) string {
	lines := splitLines(content)
	if len(lines) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("@@ -1,%d +0,0 @@\n", len(lines)))
	for _, line := range lines {
		builder.WriteString("-")
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	return builder.String()
}

func generateModifiedFileHunks(leftContent, rightContent string) string {
	leftLines := splitLines(leftContent)
	rightLines := splitLines(rightContent)

	dmp := godiff.New()
	diffs := dmp.DiffMain(leftContent, rightContent, true)
	diffs = dmp.DiffCleanupSemantic(diffs)

	hunks := computeHunks(leftLines, rightLines, diffs)
	return hunks
}

func splitLines(content string) []string {
	if content == "" {
		return []string{}
	}
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func computeHunks(leftLines, rightLines []string, diffs []godiff.Diff) string {
	var builder strings.Builder
	const contextLines = 3

	type diffLine struct {
		lineType rune
		content  string
		oldNum   int
		newNum   int
	}

	var allLines []diffLine
	oldLine := 1
	newLine := 1

	for _, d := range diffs {
		lines := splitLines(d.Text)
		if len(lines) == 0 && d.Text != "" {
			lines = []string{d.Text}
		}

		switch d.Type {
		case godiff.DiffEqual:
			for _, line := range lines {
				allLines = append(allLines, diffLine{
					lineType: ' ',
					content:  line,
					oldNum:   oldLine,
					newNum:   newLine,
				})
				oldLine++
				newLine++
			}
		case godiff.DiffDelete:
			for _, line := range lines {
				allLines = append(allLines, diffLine{
					lineType: '-',
					content:  line,
					oldNum:   oldLine,
					newNum:   0,
				})
				oldLine++
			}
		case godiff.DiffInsert:
			for _, line := range lines {
				allLines = append(allLines, diffLine{
					lineType: '+',
					content:  line,
					oldNum:   0,
					newNum:   newLine,
				})
				newLine++
			}
		}
	}

	changeIndices := make([]int, 0)
	for i, line := range allLines {
		if line.lineType != ' ' {
			changeIndices = append(changeIndices, i)
		}
	}

	if len(changeIndices) == 0 {
		return ""
	}

	type hunkRange struct {
		start int
		end   int
	}
	var hunkRanges []hunkRange

	i := 0
	for i < len(changeIndices) {
		start := changeIndices[i] - contextLines
		if start < 0 {
			start = 0
		}

		end := changeIndices[i] + contextLines
		if end >= len(allLines) {
			end = len(allLines) - 1
		}

		for i < len(changeIndices)-1 {
			nextStart := changeIndices[i+1] - contextLines
			if nextStart <= end+1 {
				newEnd := changeIndices[i+1] + contextLines
				if newEnd >= len(allLines) {
					newEnd = len(allLines) - 1
				}
				end = newEnd
				i++
			} else {
				break
			}
		}

		hunkRanges = append(hunkRanges, hunkRange{start: start, end: end})
		i++
	}

	for _, hr := range hunkRanges {
		hunkLines := allLines[hr.start : hr.end+1]
		if len(hunkLines) == 0 {
			continue
		}

		var oldStart, newStart int
		oldCount := 0
		newCount := 0

		for j, line := range hunkLines {
			if j == 0 {
				if line.oldNum > 0 {
					oldStart = line.oldNum
				} else {
					for k := 0; k < len(hunkLines); k++ {
						if hunkLines[k].oldNum > 0 {
							oldStart = hunkLines[k].oldNum
							break
						}
					}
					if oldStart == 0 {
						oldStart = 1
					}
				}
				if line.newNum > 0 {
					newStart = line.newNum
				} else {
					for k := 0; k < len(hunkLines); k++ {
						if hunkLines[k].newNum > 0 {
							newStart = hunkLines[k].newNum
							break
						}
					}
					if newStart == 0 {
						newStart = 1
					}
				}
			}

			switch line.lineType {
			case ' ':
				oldCount++
				newCount++
			case '-':
				oldCount++
			case '+':
				newCount++
			}
		}

		builder.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount))
		for _, line := range hunkLines {
			builder.WriteString(string(line.lineType))
			builder.WriteString(line.content)
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

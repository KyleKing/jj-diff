package diff

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	godiff "github.com/sergi/go-diff/diffmatchpatch"
)

// CompareDirectories generates a unified diff comparing two directories.
// Returns git-format diff text suitable for parsing by diff.Parse().
//
// A pair present on both sides is read in full only when its bytes differ: sizes are compared from
// a stat first and equal-sized files are streamed against each other, so an unchanged file costs a
// stat or one chunked pass and is never held in memory or handed to the line differ.
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

	cmp := newContentComparer()

	var diffBuilder strings.Builder
	for _, path := range allPaths {
		leftPath := filepath.Join(leftDir, path)
		rightPath := filepath.Join(rightDir, path)

		inLeft := leftFiles[path]
		inRight := rightFiles[path]

		fileDiff, err := generateFileDiff(cmp, path, leftPath, rightPath, inLeft, inRight)
		if err != nil {
			return "", fmt.Errorf("generating diff for %s: %w", path, err)
		}
		if fileDiff != "" {
			diffBuilder.WriteString(fileDiff)
		}
	}

	return diffBuilder.String(), nil
}

// jjInstructionsFile is the scratch file jj writes into the diff editor's right-hand directory with
// instructions for the user. It is not part of the diff and the editor is expected to leave it alone.
const jjInstructionsFile = "JJ-INSTRUCTIONS"

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
			return fmt.Errorf("resolving %s under %s: %w", path, dir, err)
		}

		if relPath == jjInstructionsFile {
			return nil
		}

		files[relPath] = true

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", dir, err)
	}

	return files, nil
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

func generateFileDiff(
	cmp *contentComparer, relPath, leftPath, rightPath string, inLeft, inRight bool,
) (string, error) {
	if inLeft && inRight && cmp.identical(leftPath, rightPath) {
		return "", nil
	}

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

// compareChunkSize is how much of each file one step of the streamed comparison reads.
const compareChunkSize = 32 << 10

// contentComparer decides whether two files hold the same bytes without reading either in full. Its
// buffers are reused across every pair in one comparison, so the cost per unchanged file is the
// reads alone.
type contentComparer struct {
	left  []byte
	right []byte
}

func newContentComparer() *contentComparer {
	return &contentComparer{
		left:  make([]byte, compareChunkSize),
		right: make([]byte, compareChunkSize),
	}
}

// identical reports whether leftPath and rightPath are regular files with the same bytes, following
// symlinks as os.ReadFile does. Any failure answers false rather than an error, so the caller's full
// read reports it exactly as it did before this check existed.
func (c *contentComparer) identical(leftPath, rightPath string) bool {
	leftInfo, err := os.Stat(leftPath)
	if err != nil || !leftInfo.Mode().IsRegular() {
		return false
	}

	rightInfo, err := os.Stat(rightPath)
	if err != nil || !rightInfo.Mode().IsRegular() {
		return false
	}

	if leftInfo.Size() != rightInfo.Size() {
		return false
	}

	if leftInfo.Size() == 0 || os.SameFile(leftInfo, rightInfo) {
		return true
	}

	same, err := c.sameBytes(leftPath, rightPath)

	return err == nil && same
}

func (c *contentComparer) sameBytes(leftPath, rightPath string) (bool, error) {
	//nolint:gosec // G304: paths come from the directories jj hands the diff editor.
	leftFile, err := os.Open(leftPath)
	if err != nil {
		return false, fmt.Errorf("opening %s: %w", leftPath, err)
	}
	defer closeQuietly(leftFile)

	//nolint:gosec // G304: paths come from the directories jj hands the diff editor.
	rightFile, err := os.Open(rightPath)
	if err != nil {
		return false, fmt.Errorf("opening %s: %w", rightPath, err)
	}
	defer closeQuietly(rightFile)

	for {
		leftN, leftErr := io.ReadFull(leftFile, c.left)
		rightN, rightErr := io.ReadFull(rightFile, c.right)

		if leftN != rightN || !bytes.Equal(c.left[:leftN], c.right[:rightN]) {
			return false, nil
		}

		leftDone, err := chunkDone(leftErr, leftPath)
		if err != nil {
			return false, err
		}

		rightDone, err := chunkDone(rightErr, rightPath)
		if err != nil {
			return false, err
		}

		if leftDone || rightDone {
			return leftDone == rightDone, nil
		}
	}
}

// chunkDone reads io.ReadFull's error: a short or empty read is the end of the file, and anything
// else is a real failure.
func chunkDone(err error, path string) (bool, error) {
	switch {
	case err == nil:
		return false, nil
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return true, nil
	default:
		return false, fmt.Errorf("reading %s: %w", path, err)
	}
}

// closeQuietly closes a file opened only for reading, where a close error cannot lose data and the
// comparison's answer is already decided.
func closeQuietly(f *os.File) {
	//nolint:errcheck,gosec // read-only handle; see the doc comment.
	f.Close()
}

func readFileContent(path string) (string, error) {
	//nolint:gosec // G304: paths come from the directories jj hands the diff editor.
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}

	return string(content), nil
}

func generateUnifiedDiff(path, leftContent, rightContent string, inLeft, inRight bool) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "diff --git a/%s b/%s\n", path, path)

	if !inLeft {
		builder.WriteString("new file mode 100644\n")
		builder.WriteString("--- /dev/null\n")
		fmt.Fprintf(&builder, "+++ b/%s\n", path)
		builder.WriteString(generateAddedFileHunks(rightContent))

		return builder.String()
	}

	if !inRight {
		builder.WriteString("deleted file mode 100644\n")
		fmt.Fprintf(&builder, "--- a/%s\n", path)
		builder.WriteString("+++ /dev/null\n")
		builder.WriteString(generateDeletedFileHunks(leftContent))

		return builder.String()
	}

	fmt.Fprintf(&builder, "--- a/%s\n", path)
	fmt.Fprintf(&builder, "+++ b/%s\n", path)
	builder.WriteString(generateModifiedFileHunks(leftContent, rightContent))

	return builder.String()
}

func generateAddedFileHunks(content string) string {
	lines := splitLines(content)
	if len(lines) == 0 {
		return ""
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "@@ -0,0 +1,%d @@\n", len(lines))
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
	fmt.Fprintf(&builder, "@@ -1,%d +0,0 @@\n", len(lines))
	for _, line := range lines {
		builder.WriteString("-")
		builder.WriteString(line)
		builder.WriteString("\n")
	}

	return builder.String()
}

func generateModifiedFileHunks(leftContent, rightContent string) string {
	// Line mode, not DiffMain's checklines heuristic. computeHunks splits every
	// segment on newlines, so a character-level diff hands it fragments such as
	// "p" / "fmt.P" / "rintln(" and the reconstructed file comes out corrupt.
	dmp := godiff.New()
	leftRunes, rightRunes, lineArray := dmp.DiffLinesToRunes(leftContent, rightContent)
	diffs := dmp.DiffCharsToLines(dmp.DiffMainRunes(leftRunes, rightRunes, false), lineArray)

	return computeHunks(diffs)
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

// hunkContextLines is how many unchanged lines are kept on each side of a change, and also the gap
// below which two nearby changes are merged into one hunk.
const hunkContextLines = 3

// diffLine is one line of the flattened diff. A deletion carries no newNum and an addition carries no
// oldNum, so the missing side is left at 0.
type diffLine struct {
	content  string
	oldNum   int
	newNum   int
	lineType rune
}

// hunkRange is an inclusive index range into the flattened diff.
type hunkRange struct {
	start int
	end   int
}

func computeHunks(diffs []godiff.Diff) string {
	allLines := flattenDiff(diffs)

	changed := changedIndices(allLines)
	if len(changed) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, hr := range groupHunkRanges(changed, len(allLines)) {
		writeHunk(&builder, allLines[hr.start:hr.end+1])
	}

	return builder.String()
}

func flattenDiff(diffs []godiff.Diff) []diffLine {
	var allLines []diffLine
	oldLine := 1
	newLine := 1

	for _, d := range diffs {
		lines := splitLines(d.Text)
		if len(lines) == 0 && d.Text != "" {
			lines = []string{d.Text}
		}

		for _, line := range lines {
			switch d.Type {
			case godiff.DiffEqual:
				allLines = append(allLines, diffLine{
					lineType: ' ',
					content:  line,
					oldNum:   oldLine,
					newNum:   newLine,
				})
				oldLine++
				newLine++
			case godiff.DiffDelete:
				allLines = append(allLines, diffLine{
					lineType: '-',
					content:  line,
					oldNum:   oldLine,
					newNum:   0,
				})
				oldLine++
			case godiff.DiffInsert:
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

	return allLines
}

func changedIndices(lines []diffLine) []int {
	indices := make([]int, 0, len(lines))
	for i, line := range lines {
		if line.lineType != ' ' {
			indices = append(indices, i)
		}
	}

	return indices
}

// groupHunkRanges pads each change with context and merges the ranges that end up touching, so two
// changes closer than twice the context land in one hunk rather than two overlapping ones.
func groupHunkRanges(changed []int, total int) []hunkRange {
	var ranges []hunkRange

	i := 0
	for i < len(changed) {
		start := max(changed[i]-hunkContextLines, 0)
		end := min(changed[i]+hunkContextLines, total-1)

		for i < len(changed)-1 {
			if changed[i+1]-hunkContextLines > end+1 {
				break
			}

			end = min(changed[i+1]+hunkContextLines, total-1)
			i++
		}

		ranges = append(ranges, hunkRange{start: start, end: end})
		i++
	}

	return ranges
}

func writeHunk(builder *strings.Builder, hunkLines []diffLine) {
	if len(hunkLines) == 0 {
		return
	}

	oldCount := 0
	newCount := 0

	for _, line := range hunkLines {
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

	oldStart := firstLineNum(hunkLines, func(l diffLine) int { return l.oldNum })
	newStart := firstLineNum(hunkLines, func(l diffLine) int { return l.newNum })

	fmt.Fprintf(builder, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
	for _, line := range hunkLines {
		builder.WriteRune(line.lineType)
		builder.WriteString(line.content)
		builder.WriteString("\n")
	}
}

// firstLineNum returns the first line number pick reports for the hunk, falling back to 1 for a hunk
// that exists on only one side and therefore has none.
func firstLineNum(lines []diffLine, pick func(diffLine) int) int {
	for _, line := range lines {
		if num := pick(line); num > 0 {
			return num
		}
	}

	return 1
}

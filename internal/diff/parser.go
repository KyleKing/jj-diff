// Package diff parses unified diff text into files, hunks, and lines, and turns a user's hunk and
// line selections back into a patch or into reconstructed files on disk.
package diff

import (
	"regexp"
	"strconv"
	"strings"
)

// FileChange holds one file's hunks in the order the diff lists them. Path is the "b/" side of the
// diff header, so a renamed file carries its new path.
type FileChange struct {
	Path       string
	Hunks      []Hunk
	ChangeType ChangeType
}

// ChangeType is what a diff header says happened to a file. String returns the one-letter status
// jj and git print.
type ChangeType int

// Change kinds a file header can describe. ChangeTypeModified is the zero value and the fallback
// when the header carries no "new file", "deleted file", or "rename from" marker.
const (
	ChangeTypeModified ChangeType = iota
	ChangeTypeAdded
	ChangeTypeDeleted
	ChangeTypeRenamed
)

func (ct ChangeType) String() string {
	switch ct {
	case ChangeTypeModified:
		return "M"
	case ChangeTypeAdded:
		return "A"
	case ChangeTypeDeleted:
		return "D"
	case ChangeTypeRenamed:
		return "R"
	default:
		return "?"
	}
}

// Hunk is one @@ section. OldStart and NewStart are 1-based line numbers in the old and new file,
// and Lines interleaves context, additions, and deletions in diff order.
type Hunk struct {
	Header   string
	Lines    []Line
	OldStart int
	OldLines int
	NewStart int
	NewLines int
}

// Line is one diff line with the leading +, -, or space stripped from Content. OldLineNum and
// NewLineNum are 1-based and both are always set, so an addition still records where it falls on
// the old side.
type Line struct {
	Content    string
	Type       LineType
	OldLineNum int
	NewLineNum int
}

// LineType marks a diff line as context, an addition, or a deletion. String returns the diff
// marker, which patch rendering writes back in front of the content.
type LineType int

// Line kinds inside a hunk. LineContext is the zero value, so a line the parser cannot classify
// from its first byte is treated as context and its content is kept whole.
const (
	LineContext LineType = iota
	LineAddition
	LineDeletion
)

func (lt LineType) String() string {
	switch lt {
	case LineContext:
		return " "
	case LineAddition:
		return "+"
	case LineDeletion:
		return "-"
	default:
		return "?"
	}
}

var (
	diffHeaderRE = regexp.MustCompile(`^diff --git a/(.*) b/(.*)$`)
	hunkHeaderRE = regexp.MustCompile(`^@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@(.*)$`)
)

// Parse splits unified diff text into one FileChange per "diff --git" header, dropping any section
// whose header does not parse. Empty input returns an empty slice rather than nil.
func Parse(diffText string) []FileChange {
	if diffText == "" {
		return []FileChange{}
	}

	var files []FileChange
	sections := strings.Split(diffText, "diff --git")

	for _, section := range sections {
		if strings.TrimSpace(section) == "" {
			continue
		}

		section = "diff --git" + section
		file := parseFileChange(section)
		if file != nil {
			files = append(files, *file)
		}
	}

	return files
}

// Submatch counts the header patterns must produce, which is one more than the group count each
// pattern defines.
const (
	diffHeaderGroups = 3
	hunkHeaderGroups = 5
)

func parseFileChange(section string) *FileChange {
	lines := strings.Split(section, "\n")
	if len(lines) == 0 {
		return nil
	}

	match := diffHeaderRE.FindStringSubmatch(lines[0])
	if len(match) < diffHeaderGroups {
		return nil
	}

	file := &FileChange{
		Path:       match[2],
		ChangeType: determineChangeType(section),
	}

	var currentHunk *Hunk
	oldLineNum := 0
	newLineNum := 0

	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "@@") {
			if currentHunk != nil {
				file.Hunks = append(file.Hunks, *currentHunk)
			}

			currentHunk = parseHunkHeader(line)
			if currentHunk != nil {
				oldLineNum = currentHunk.OldStart
				newLineNum = currentHunk.NewStart
			}

			continue
		}

		if currentHunk == nil || line == "" || isMetadataLine(line) {
			continue
		}

		parsed := classifyLine(line)
		parsed.OldLineNum = oldLineNum
		parsed.NewLineNum = newLineNum
		currentHunk.Lines = append(currentHunk.Lines, parsed)

		switch parsed.Type {
		case LineContext:
			oldLineNum++
			newLineNum++
		case LineAddition:
			newLineNum++
		case LineDeletion:
			oldLineNum++
		}
	}

	if currentHunk != nil {
		file.Hunks = append(file.Hunks, *currentHunk)
	}

	return file
}

// isMetadataLine reports the header lines that can appear inside a section after a hunk header and
// carry no diff content, including the ones that start with a + or - marker.
func isMetadataLine(line string) bool {
	return strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") ||
		strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "new file") ||
		strings.HasPrefix(line, "deleted file")
}

// classifyLine reads a non-empty diff line into its kind and its content with the marker stripped,
// leaving the line numbers for the caller. A line whose first byte is not a marker is kept whole as
// context.
func classifyLine(line string) Line {
	switch line[0] {
	case '+':
		return Line{Type: LineAddition, Content: line[1:]}
	case '-':
		return Line{Type: LineDeletion, Content: line[1:]}
	case ' ':
		return Line{Type: LineContext, Content: line[1:]}
	default:
		return Line{Type: LineContext, Content: line}
	}
}

func atoiOrDefault(value string, fallback int) (int, bool) {
	if value == "" {
		return fallback, true
	}

	parsed, err := strconv.Atoi(value)

	return parsed, err == nil
}

func parseHunkHeader(header string) *Hunk {
	match := hunkHeaderRE.FindStringSubmatch(header)
	if len(match) < hunkHeaderGroups {
		return nil
	}

	oldStart, err := strconv.Atoi(match[1])
	if err != nil {
		return nil
	}

	oldLines, ok := atoiOrDefault(match[2], 1)
	if !ok {
		return nil
	}

	newStart, err := strconv.Atoi(match[3])
	if err != nil {
		return nil
	}

	newLines, ok := atoiOrDefault(match[4], 1)
	if !ok {
		return nil
	}

	return &Hunk{
		Header:   header,
		OldStart: oldStart,
		OldLines: oldLines,
		NewStart: newStart,
		NewLines: newLines,
		Lines:    []Line{},
	}
}

func determineChangeType(section string) ChangeType {
	if strings.Contains(section, "new file mode") {
		return ChangeTypeAdded
	}
	if strings.Contains(section, "deleted file mode") {
		return ChangeTypeDeleted
	}
	if strings.Contains(section, "rename from") {
		return ChangeTypeRenamed
	}

	return ChangeTypeModified
}

// TotalLines counts every line across the file's hunks, context included, so it is the height the
// file's hunks occupy rather than the size of the change.
func (fc *FileChange) TotalLines() int {
	total := 0
	for _, hunk := range fc.Hunks {
		total += len(hunk.Lines)
	}

	return total
}

// AddedLines counts added lines across every hunk, which with DeletedLines gives the +/- pair shown
// beside a file. A whitespace-only edit still counts on both sides.
func (fc *FileChange) AddedLines() int {
	count := 0
	for _, hunk := range fc.Hunks {
		for _, line := range hunk.Lines {
			if line.Type == LineAddition {
				count++
			}
		}
	}

	return count
}

// DeletedLines counts deleted lines across every hunk, counting the parsed diff rather than the
// file, so a line replaced by an edit counts here and in AddedLines.
func (fc *FileChange) DeletedLines() int {
	count := 0
	for _, hunk := range fc.Hunks {
		for _, line := range hunk.Lines {
			if line.Type == LineDeletion {
				count++
			}
		}
	}

	return count
}

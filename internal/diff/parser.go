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
	ChangeType ChangeType
	Hunks      []Hunk
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
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Lines    []Line
}

// Line is one diff line with the leading +, -, or space stripped from Content. OldLineNum and
// NewLineNum are 1-based and both are always set, so an addition still records where it falls on
// the old side.
type Line struct {
	Type       LineType
	Content    string
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

func parseFileChange(section string) *FileChange {
	lines := strings.Split(section, "\n")
	if len(lines) == 0 {
		return nil
	}

	file := &FileChange{}

	match := diffHeaderRE.FindStringSubmatch(lines[0])
	if len(match) < 3 {
		return nil
	}
	file.Path = match[2]

	file.ChangeType = determineChangeType(section)

	var currentHunk *Hunk
	oldLineNum := 0
	newLineNum := 0

	for i := 1; i < len(lines); i++ {
		line := lines[i]

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

		if currentHunk == nil {
			continue
		}

		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") ||
			strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "new file") ||
			strings.HasPrefix(line, "deleted file") {
			continue
		}

		if len(line) == 0 {
			continue
		}

		lineType := LineContext
		content := line

		if strings.HasPrefix(line, "+") {
			lineType = LineAddition
			content = line[1:]
		} else if strings.HasPrefix(line, "-") {
			lineType = LineDeletion
			content = line[1:]
		} else if strings.HasPrefix(line, " ") {
			lineType = LineContext
			content = line[1:]
		}

		diffLine := Line{
			Type:       lineType,
			Content:    content,
			OldLineNum: oldLineNum,
			NewLineNum: newLineNum,
		}

		currentHunk.Lines = append(currentHunk.Lines, diffLine)

		switch lineType {
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

func atoiOrDefault(value string, fallback int) (int, bool) {
	if value == "" {
		return fallback, true
	}

	parsed, err := strconv.Atoi(value)

	return parsed, err == nil
}

func parseHunkHeader(header string) *Hunk {
	match := hunkHeaderRE.FindStringSubmatch(header)
	if len(match) < 5 {
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

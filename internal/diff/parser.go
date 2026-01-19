package diff

import (
	"regexp"
	"strconv"
	"strings"
)

type FileChange struct {
	Path       string
	ChangeType ChangeType
	Hunks      []Hunk
}

type ChangeType int

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

type Hunk struct {
	Header   string
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Lines    []Line
}

type Line struct {
	Type       LineType
	Content    string
	OldLineNum int
	NewLineNum int
}

type LineType int

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

func parseHunkHeader(header string) *Hunk {
	match := hunkHeaderRE.FindStringSubmatch(header)
	if len(match) < 5 {
		return nil
	}

	oldStart, _ := strconv.Atoi(match[1])
	oldLines := 1
	if match[2] != "" {
		oldLines, _ = strconv.Atoi(match[2])
	}

	newStart, _ := strconv.Atoi(match[3])
	newLines := 1
	if match[4] != "" {
		newLines, _ = strconv.Atoi(match[4])
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

func (fc *FileChange) TotalLines() int {
	total := 0
	for _, hunk := range fc.Hunks {
		total += len(hunk.Lines)
	}
	return total
}

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

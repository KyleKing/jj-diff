package diff

import (
	"strings"
)

// Glyphs standing in for whitespace when it is rendered visibly.
const (
	TabChar   = '→'
	SpaceChar = '·'
)

// RenderWhitespaceSimple substitutes whitespace glyphs without any styling, treating trailing
// whitespace the same as interior whitespace. The result carries no ANSI codes, which suits a
// caller that needs to slice or measure the output by index.
func RenderWhitespaceSimple(content string, tabWidth int) string {
	if content == "" {
		return content
	}

	var result strings.Builder
	for i := range len(content) {
		switch content[i] {
		case '\t':
			result.WriteRune(TabChar)
			padding := tabWidth - 1
			for range padding {
				result.WriteRune(' ')
			}
		case ' ':
			result.WriteRune(SpaceChar)
		default:
			result.WriteByte(content[i])
		}
	}

	return result.String()
}

// HasTrailingWhitespace reports whether content ends in a space or a tab. An empty string does not,
// and a line that is entirely whitespace does.
func HasTrailingWhitespace(content string) bool {
	if content == "" {
		return false
	}
	last := content[len(content)-1]

	return last == ' ' || last == '\t'
}

// CountTrailingWhitespace counts the trailing spaces and tabs as bytes, so a tab counts once
// whatever tab width the renderer expands it to.
func CountTrailingWhitespace(content string) int {
	count := 0
	for i := len(content) - 1; i >= 0; i-- {
		if content[i] != ' ' && content[i] != '\t' {
			break
		}
		count++
	}

	return count
}

// IsWhitespaceOnlyChange returns true if the two strings differ only in whitespace.
func IsWhitespaceOnlyChange(oldContent, newContent string) bool {
	return strings.TrimSpace(oldContent) == strings.TrimSpace(newContent)
}

// GetLeadingWhitespace extracts the leading whitespace from a string.
func GetLeadingWhitespace(content string) string {
	for i, ch := range content {
		if ch != ' ' && ch != '\t' {
			return content[:i]
		}
	}

	return content
}

// ProcessHunkHideWhitespace transforms a hunk to hide whitespace-only changes
// Returns a new set of lines with whitespace changes handled.
func ProcessHunkHideWhitespace(lines []Line) []Line {
	result := make([]Line, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		isReplacement := line.Type == LineDeletion &&
			i+1 < len(lines) && lines[i+1].Type == LineAddition
		if !isReplacement {
			result = append(result, line)

			continue
		}

		nextLine := lines[i+1]

		if IsWhitespaceOnlyChange(line.Content, nextLine.Content) {
			result = append(result, Line{
				Type:       LineContext,
				Content:    nextLine.Content,
				OldLineNum: line.OldLineNum,
				NewLineNum: nextLine.NewLineNum,
			})
			i++

			continue
		}

		result = append(result, reindentDeletion(line, nextLine))
	}

	return result
}

// reindentDeletion gives a deletion the indentation of the addition replacing it, so a pair that also
// changed indentation reads as a content change alone.
func reindentDeletion(deletion, addition Line) Line {
	oldIndent := GetLeadingWhitespace(deletion.Content)
	newIndent := GetLeadingWhitespace(addition.Content)
	if oldIndent == newIndent {
		return deletion
	}

	return Line{
		Type:       LineDeletion,
		Content:    newIndent + strings.TrimLeft(deletion.Content, " \t"),
		OldLineNum: deletion.OldLineNum,
		NewLineNum: deletion.NewLineNum,
	}
}

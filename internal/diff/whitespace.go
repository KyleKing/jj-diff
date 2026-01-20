package diff

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	TabChar          = '→'
	SpaceChar        = '·'
	TrailingSpaceStr = "␣"
)

type WhitespaceRenderer struct {
	tabWidth          int
	trailingHighlight lipgloss.Style
}

func NewWhitespaceRenderer(tabWidth int, trailingHighlight lipgloss.Style) *WhitespaceRenderer {
	return &WhitespaceRenderer{
		tabWidth:          tabWidth,
		trailingHighlight: trailingHighlight,
	}
}

func (r *WhitespaceRenderer) Render(content string) string {
	if content == "" {
		return content
	}

	trimmed := strings.TrimRight(content, " \t")
	trailing := content[len(trimmed):]

	var result strings.Builder
	for i := 0; i < len(trimmed); i++ {
		switch trimmed[i] {
		case '\t':
			result.WriteRune(TabChar)
			padding := r.tabWidth - 1
			for j := 0; j < padding; j++ {
				result.WriteRune(' ')
			}
		case ' ':
			result.WriteRune(SpaceChar)
		default:
			result.WriteByte(trimmed[i])
		}
	}

	if len(trailing) > 0 {
		trailingRendered := r.renderTrailingWhitespace(trailing)
		result.WriteString(trailingRendered)
	}

	return result.String()
}

func (r *WhitespaceRenderer) renderTrailingWhitespace(trailing string) string {
	var result strings.Builder
	for i := 0; i < len(trailing); i++ {
		switch trailing[i] {
		case '\t':
			result.WriteRune(TabChar)
			padding := r.tabWidth - 1
			for j := 0; j < padding; j++ {
				result.WriteRune(' ')
			}
		case ' ':
			result.WriteString(TrailingSpaceStr)
		}
	}
	return r.trailingHighlight.Render(result.String())
}

func RenderWhitespaceSimple(content string, tabWidth int) string {
	if content == "" {
		return content
	}

	var result strings.Builder
	for i := 0; i < len(content); i++ {
		switch content[i] {
		case '\t':
			result.WriteRune(TabChar)
			padding := tabWidth - 1
			for j := 0; j < padding; j++ {
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

func HasTrailingWhitespace(content string) bool {
	if len(content) == 0 {
		return false
	}
	last := content[len(content)-1]
	return last == ' ' || last == '\t'
}

func CountTrailingWhitespace(content string) int {
	count := 0
	for i := len(content) - 1; i >= 0; i-- {
		if content[i] == ' ' || content[i] == '\t' {
			count++
		} else {
			break
		}
	}
	return count
}

// IsWhitespaceOnlyChange returns true if the two strings differ only in whitespace
func IsWhitespaceOnlyChange(oldContent, newContent string) bool {
	return strings.TrimSpace(oldContent) == strings.TrimSpace(newContent)
}

// GetLeadingWhitespace extracts the leading whitespace from a string
func GetLeadingWhitespace(content string) string {
	for i, ch := range content {
		if ch != ' ' && ch != '\t' {
			return content[:i]
		}
	}
	return content
}

// ProcessHunkHideWhitespace transforms a hunk to hide whitespace-only changes
// Returns a new set of lines with whitespace changes handled
func ProcessHunkHideWhitespace(lines []Line) []Line {
	result := make([]Line, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Check if this is a deletion followed by an addition (potential whitespace change)
		if line.Type == LineDeletion && i+1 < len(lines) && lines[i+1].Type == LineAddition {
			nextLine := lines[i+1]

			// Check if only whitespace differs
			if IsWhitespaceOnlyChange(line.Content, nextLine.Content) {
				// Skip both lines and render as single context line with new indentation
				result = append(result, Line{
					Type:       LineContext,
					Content:    nextLine.Content,
					OldLineNum: line.OldLineNum,
					NewLineNum: nextLine.NewLineNum,
				})
				i++ // Skip the addition line
				continue
			} else {
				// Content changed - use new indentation for both lines
				oldIndent := GetLeadingWhitespace(line.Content)
				newIndent := GetLeadingWhitespace(nextLine.Content)

				if oldIndent != newIndent {
					// Apply new indentation to old content
					oldTrimmed := strings.TrimLeft(line.Content, " \t")
					result = append(result, Line{
						Type:       LineDeletion,
						Content:    newIndent + oldTrimmed,
						OldLineNum: line.OldLineNum,
						NewLineNum: line.NewLineNum,
					})
				} else {
					result = append(result, line)
				}
				// Continue normally, next line will be processed in next iteration
				continue
			}
		}

		result = append(result, line)
	}

	return result
}

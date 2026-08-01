package diff

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Glyphs standing in for whitespace when it is rendered visibly. A trailing space gets its own glyph
// so it stays distinguishable from an interior space.
const (
	TabChar          = '→'
	SpaceChar        = '·'
	TrailingSpaceStr = "␣"
)

// WhitespaceRenderer replaces whitespace with visible glyphs and styles only the run at the end of a
// line, so trailing whitespace reads differently from the whitespace inside a line.
type WhitespaceRenderer struct {
	trailingHighlight lipgloss.Style
	tabWidth          int
}

// NewWhitespaceRenderer builds a renderer that draws a tab as one glyph padded out to tabWidth
// columns and paints the trailing whitespace run with trailingHighlight.
func NewWhitespaceRenderer(tabWidth int, trailingHighlight lipgloss.Style) *WhitespaceRenderer {
	return &WhitespaceRenderer{
		tabWidth:          tabWidth,
		trailingHighlight: trailingHighlight,
	}
}

// Render substitutes glyphs for spaces and tabs, returning the empty string unchanged. Output for a
// line with trailing whitespace carries ANSI styling, so measure it with lipgloss.Width rather than
// len, and it is wider than the input wherever a tab expanded.
func (r *WhitespaceRenderer) Render(content string) string {
	if content == "" {
		return content
	}

	trimmed := strings.TrimRight(content, " \t")
	trailing := content[len(trimmed):]

	var result strings.Builder
	for i := range len(trimmed) {
		switch trimmed[i] {
		case '\t':
			result.WriteRune(TabChar)
			padding := r.tabWidth - 1
			for range padding {
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
	for i := range len(trailing) {
		switch trailing[i] {
		case '\t':
			result.WriteRune(TabChar)
			padding := r.tabWidth - 1
			for range padding {
				result.WriteRune(' ')
			}
		case ' ':
			result.WriteString(TrailingSpaceStr)
		}
	}

	return r.trailingHighlight.Render(result.String())
}

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
	if len(content) == 0 {
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
		if content[i] == ' ' || content[i] == '\t' {
			count++
		} else {
			break
		}
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

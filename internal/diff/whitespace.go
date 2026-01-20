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

package diffview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/jj-diff/internal/diff"
)

type Model struct {
	fileChange *diff.FileChange
	offset     int
}

func New() Model {
	return Model{
		offset: 0,
	}
}

func (m *Model) SetFileChange(file diff.FileChange) {
	m.fileChange = &file
	m.offset = 0
}

func (m *Model) Scroll(delta int) {
	if m.fileChange == nil {
		return
	}

	totalLines := m.calculateTotalLines()
	newOffset := m.offset + delta

	if newOffset < 0 {
		newOffset = 0
	}
	if newOffset > totalLines-1 {
		newOffset = totalLines - 1
	}

	m.offset = newOffset
}

func (m Model) calculateTotalLines() int {
	if m.fileChange == nil {
		return 0
	}

	total := 0
	for _, hunk := range m.fileChange.Hunks {
		total++
		total += len(hunk.Lines)
	}
	return total
}

func (m Model) View(width, height int) string {
	if m.fileChange == nil {
		return padToSize("No file selected", width, height)
	}

	var lines []string

	header := fmt.Sprintf("%s %s", m.fileChange.ChangeType.String(), m.fileChange.Path)
	lines = append(lines, styleHeader(header, width))

	currentLine := 0
	for _, hunk := range m.fileChange.Hunks {
		if currentLine >= m.offset && len(lines) < height {
			lines = append(lines, styleHunkHeader(hunk.Header, width))
		}
		currentLine++

		for _, line := range hunk.Lines {
			if currentLine >= m.offset && len(lines) < height {
				lines = append(lines, m.renderLine(line, width))
			}
			currentLine++
		}
	}

	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderLine(line diff.Line, width int) string {
	lineNum := ""
	if line.Type == diff.LineAddition {
		lineNum = fmt.Sprintf("%4d", line.NewLineNum)
	} else if line.Type == diff.LineDeletion {
		lineNum = fmt.Sprintf("%4d", line.OldLineNum)
	} else {
		lineNum = fmt.Sprintf("%4d", line.NewLineNum)
	}

	prefix := line.Type.String()
	content := line.Content

	maxContentWidth := width - 6
	if len(content) > maxContentWidth {
		content = content[:maxContentWidth]
	}

	lineText := fmt.Sprintf("%s %s %s", lineNum, prefix, content)

	switch line.Type {
	case diff.LineAddition:
		return styleAddition(truncateOrPad(lineText, width))
	case diff.LineDeletion:
		return styleDeletion(truncateOrPad(lineText, width))
	default:
		return truncateOrPad(lineText, width)
	}
}

func styleHeader(text string, width int) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12"))
	return style.Render(truncateOrPad(text, width))
}

func styleHunkHeader(text string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("14"))
	return style.Render(truncateOrPad(text, width))
}

func styleAddition(text string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("22")).
		Render(text)
}

func styleDeletion(text string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("160")).
		Render(text)
}

func truncateOrPad(text string, width int) string {
	visibleLen := lipgloss.Width(text)
	if visibleLen > width {
		return text[:width]
	}
	return text + strings.Repeat(" ", width-visibleLen)
}

func padToSize(text string, width, height int) string {
	lines := []string{text}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

package diffview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/theme"
)

type Model struct {
	fileChange   *diff.FileChange
	offset       int
	selectedHunk int
	isSelected   func(hunkIdx int) bool
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

func (m *Model) SetSelection(selectedHunk int, isSelected func(hunkIdx int) bool) {
	m.selectedHunk = selectedHunk
	m.isSelected = isSelected
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
	for hunkIdx, hunk := range m.fileChange.Hunks {
		if currentLine >= m.offset && len(lines) < height {
			isCurrent := hunkIdx == m.selectedHunk
			isHunkSelected := m.isSelected != nil && m.isSelected(hunkIdx)
			lines = append(lines, m.renderHunkHeader(hunk.Header, width, isCurrent, isHunkSelected))
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
		Foreground(theme.Primary)
	return style.Render(truncateOrPad(text, width))
}

func (m Model) renderHunkHeader(text string, width int, isCurrent, isSelected bool) string {
	prefix := "  "
	if isCurrent {
		prefix = "> "
	}

	suffix := ""
	if isSelected {
		suffix = " [X]"
	}

	displayText := prefix + text + suffix

	style := lipgloss.NewStyle().
		Foreground(theme.Accent)

	if isCurrent {
		style = style.Background(theme.MutedBg)
	}

	return style.Render(truncateOrPad(displayText, width))
}

func styleHunkHeader(text string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(theme.Accent)
	return style.Render(truncateOrPad(text, width))
}

func styleAddition(text string) string {
	return lipgloss.NewStyle().
		Foreground(theme.AddedLine).
		Render(text)
}

func styleDeletion(text string) string {
	return lipgloss.NewStyle().
		Foreground(theme.DeletedLine).
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

package diffview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/highlight"
	"github.com/kyleking/jj-diff/internal/theme"
)

type MatchRange struct {
	Start int
	End   int
}

type Model struct {
	fileChange     *diff.FileChange
	offset         int
	selectedHunk   int
	lineCursor     int
	isVisualMode   bool
	visualAnchor   int
	isSelected     func(hunkIdx int) bool
	isLineSelected func(hunkIdx, lineIdx int) bool
	getMatches     func(hunkIdx, lineIdx int) []MatchRange
	isSearching    bool
	highlighter    *highlight.Highlighter
	enableHighlight bool
}

func New() Model {
	return Model{
		offset:          0,
		highlighter:     highlight.New(),
		enableHighlight: true,
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

func (m *Model) SetVisualState(lineCursor int, isVisualMode bool, visualAnchor int, isLineSelected func(hunkIdx, lineIdx int) bool) {
	m.lineCursor = lineCursor
	m.isVisualMode = isVisualMode
	m.visualAnchor = visualAnchor
	m.isLineSelected = isLineSelected
}

func (m *Model) SetSearchState(isSearching bool, getMatches func(hunkIdx, lineIdx int) []MatchRange) {
	m.isSearching = isSearching
	m.getMatches = getMatches
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

		for lineIdx, line := range hunk.Lines {
			if currentLine >= m.offset && len(lines) < height {
				lines = append(lines, m.renderLine(line, width, hunkIdx, lineIdx))
			}
			currentLine++
		}
	}

	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderLine(line diff.Line, width int, hunkIdx, lineIdx int) string {
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

	maxContentWidth := width - 8
	if len(content) > maxContentWidth {
		content = content[:maxContentWidth]
	}

	// Apply syntax highlighting to context lines only (preserve diff colors for +/-)
	if m.enableHighlight && m.fileChange != nil && line.Type == diff.LineContext {
		highlighted := m.highlighter.HighlightLine(m.fileChange.Path, content)
		if highlighted != "" {
			content = highlighted
		}
	}

	// Apply search match highlighting if searching (takes precedence over syntax)
	if m.isSearching && m.getMatches != nil {
		matches := m.getMatches(hunkIdx, lineIdx)
		if len(matches) > 0 {
			content = m.highlightMatches(content, matches)
		}
	}

	// Check selection state
	isCurrentLine := m.lineCursor == lineIdx && hunkIdx == m.selectedHunk
	isInVisualRange := m.isVisualMode && hunkIdx == m.selectedHunk && m.isLineInVisualRange(lineIdx)
	isSelected := m.isLineSelected != nil && m.isLineSelected(hunkIdx, lineIdx)

	// Determine line indicator
	lineIndicator := "  "
	if isInVisualRange {
		lineIndicator = "█ "
	} else if isSelected {
		lineIndicator = "• "
	} else if isCurrentLine {
		lineIndicator = "> "
	}

	lineText := fmt.Sprintf("%s%s %s %s", lineIndicator, lineNum, prefix, content)

	// Apply styling
	style := lipgloss.NewStyle()
	switch line.Type {
	case diff.LineAddition:
		style = style.Foreground(theme.AddedLine)
	case diff.LineDeletion:
		style = style.Foreground(theme.DeletedLine)
	}

	if isInVisualRange {
		style = style.Background(theme.SelectedBg)
	} else if isCurrentLine {
		style = style.Background(theme.MutedBg)
	}

	return style.Render(truncateOrPad(lineText, width))
}

func (m Model) isLineInVisualRange(lineIdx int) bool {
	if !m.isVisualMode {
		return false
	}
	start := m.visualAnchor
	end := m.lineCursor
	if start > end {
		start, end = end, start
	}
	return lineIdx >= start && lineIdx <= end
}

func (m Model) highlightMatches(content string, matches []MatchRange) string {
	if len(matches) == 0 {
		return content
	}

	var segments []string
	lastEnd := 0

	for _, match := range matches {
		// Add text before match
		if lastEnd < match.Start && match.Start <= len(content) {
			segments = append(segments, content[lastEnd:match.Start])
		}

		// Add highlighted match
		if match.Start < len(content) {
			endIdx := match.End
			if endIdx > len(content) {
				endIdx = len(content)
			}
			matchText := content[match.Start:endIdx]
			highlightedMatch := lipgloss.NewStyle().
				Background(theme.Accent).
				Foreground(theme.ModalBg).
				Render(matchText)
			segments = append(segments, highlightedMatch)
			lastEnd = endIdx
		}
	}

	// Add remaining text after last match
	if lastEnd < len(content) {
		segments = append(segments, content[lastEnd:])
	}

	return strings.Join(segments, "")
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

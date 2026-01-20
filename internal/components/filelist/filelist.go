package filelist

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/theme"
)

type MatchRange struct {
	Start int
	End   int
}

type Model struct {
	files        []diff.FileChange
	selected     int
	getMatches   func(fileIdx int) []MatchRange
	isSearching  bool
}

func New() Model {
	return Model{
		files:    []diff.FileChange{},
		selected: 0,
	}
}

func (m *Model) SetFiles(files []diff.FileChange) {
	m.files = files
}

func (m *Model) SetSelected(idx int) {
	m.selected = idx
}

func (m *Model) SetSearchState(isSearching bool, getMatches func(fileIdx int) []MatchRange) {
	m.isSearching = isSearching
	m.getMatches = getMatches
}

func (m Model) View(width, height int, focused bool) string {
	if len(m.files) == 0 {
		return padToSize("No files", width, height)
	}

	var lines []string
	lines = append(lines, styleHeader("Files", width))

	for i, file := range m.files {
		isSelected := i == m.selected
		var line string
		if m.isSearching {
			line = m.renderFileLineWithMatches(file, i, isSelected, focused)
		} else {
			line = m.renderFileLine(file, isSelected, focused)
		}
		lines = append(lines, truncateOrPad(line, width))

		if len(lines) >= height {
			break
		}
	}

	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderFileLine(file diff.FileChange, selected, focused bool) string {
	changeIndicator := fmt.Sprintf("[%s]", file.ChangeType.String())

	line := fmt.Sprintf("%s %s", changeIndicator, file.Path)

	if selected {
		if focused {
			return styleSelectedFocused(line)
		}
		return styleSelected(line)
	}

	return styleNormal(line)
}

func (m Model) renderFileLineWithMatches(file diff.FileChange, fileIdx int, selected, focused bool) string {
	changeIndicator := fmt.Sprintf("[%s]", file.ChangeType.String())
	prefix := changeIndicator + " "

	// Get matches for this file's path
	var matches []MatchRange
	if m.isSearching && m.getMatches != nil {
		matches = m.getMatches(fileIdx)
	}

	// If no matches or not searching, use simple rendering
	if len(matches) == 0 {
		line := prefix + file.Path
		if selected {
			if focused {
				return styleSelectedFocused(line)
			}
			return styleSelected(line)
		}
		return styleNormal(line)
	}

	// Render with highlighted matches
	var segments []string
	lastEnd := 0

	for _, match := range matches {
		// Adjust match positions to account for prefix
		adjustedStart := match.Start
		adjustedEnd := match.End

		// Add text before match
		if lastEnd < adjustedStart {
			segments = append(segments, file.Path[lastEnd:adjustedStart])
		}

		// Add highlighted match
		matchText := file.Path[adjustedStart:adjustedEnd]
		highlightedMatch := lipgloss.NewStyle().
			Background(theme.Accent).
			Foreground(theme.ModalBg).
			Render(matchText)
		segments = append(segments, highlightedMatch)

		lastEnd = adjustedEnd
	}

	// Add remaining text after last match
	if lastEnd < len(file.Path) {
		segments = append(segments, file.Path[lastEnd:])
	}

	line := prefix + strings.Join(segments, "")

	if selected {
		if focused {
			return styleSelectedFocused(line)
		}
		return styleSelected(line)
	}

	return line
}

func styleHeader(text string, width int) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary)
	return style.Render(truncateOrPad(text, width))
}

func styleSelectedFocused(text string) string {
	return lipgloss.NewStyle().
		Background(theme.SelectedBg).
		Foreground(theme.Text).
		Render(text)
}

func styleSelected(text string) string {
	return lipgloss.NewStyle().
		Background(theme.MutedBg).
		Render(text)
}

func styleNormal(text string) string {
	return text
}

func truncateOrPad(text string, width int) string {
	if len(text) > width {
		return text[:width-3] + "..."
	}
	return text + strings.Repeat(" ", width-len(text))
}

func padToSize(text string, width, height int) string {
	lines := []string{text}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

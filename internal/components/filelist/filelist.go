package filelist

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/fuzzy"
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
	expanded     bool
	scrollOffset int
	filterMode   bool
	filterQuery  string
}

func New() Model {
	return Model{
		files:        []diff.FileChange{},
		selected:     0,
		expanded:     false,
		scrollOffset: 0,
		filterMode:   false,
		filterQuery:  "",
	}
}

func (m *Model) SetExpanded(expanded bool) {
	m.expanded = expanded
}

func (m Model) IsExpanded() bool {
	return m.expanded
}

func (m *Model) SetFilterMode(enabled bool) {
	m.filterMode = enabled
	if !enabled {
		m.filterQuery = ""
	}
}

func (m Model) IsFilterMode() bool {
	return m.filterMode
}

func (m *Model) SetFilterQuery(query string) {
	m.filterQuery = query
}

func (m Model) FilterQuery() string {
	return m.filterQuery
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
		if m.expanded {
			return padToSize("No files", width, height)
		}
		return padToSize("No files", width, 1)
	}

	if m.expanded {
		return m.renderExpanded(width, height, focused)
	}
	return m.renderCollapsed(width, focused)
}

func (m Model) renderCollapsed(width int, focused bool) string {
	if len(m.files) == 0 {
		return padToSize("No files", width, 1)
	}

	file := m.files[m.selected]
	counter := fmt.Sprintf(" [%d/%d]", m.selected+1, len(m.files))

	changeType := file.ChangeType.String()
	path := file.Path

	// Calculate stats
	var additions, deletions int
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if line.Type == diff.LineAddition {
				additions++
			} else if line.Type == diff.LineDeletion {
				deletions++
			}
		}
	}

	stats := fmt.Sprintf("+%d -%d", additions, deletions)

	// Format: [M] path/to/file.go +10 -5 [3/10]
	// Match diff header styling: Primary color, bold
	line := fmt.Sprintf("[%s] %s %s%s", changeType, path, stats, counter)

	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary)
	if focused {
		style = style.Background(theme.MutedBg)
	}

	return style.Render(truncateOrPad(line, width))
}

func (m Model) renderExpanded(width, height int, focused bool) string {
	var lines []string

	// Determine which files to display
	var displayFiles []diff.FileChange
	var displayIndices []int

	if m.filterMode && m.filterQuery != "" {
		// Filter files by query
		filePaths := make([]string, len(m.files))
		fileData := make([]interface{}, len(m.files))
		for i, f := range m.files {
			filePaths[i] = f.Path
			fileData[i] = i
		}

		matches := fuzzy.FilterWithData(m.filterQuery, filePaths, fileData)
		displayFiles = make([]diff.FileChange, len(matches))
		displayIndices = make([]int, len(matches))
		for i, match := range matches {
			idx := match.Original.(int)
			displayFiles[i] = m.files[idx]
			displayIndices[i] = idx
		}
	} else {
		displayFiles = m.files
		displayIndices = make([]int, len(m.files))
		for i := range m.files {
			displayIndices[i] = i
		}
	}

	// Header with file counter
	totalFiles := len(m.files)
	filteredCount := len(displayFiles)
	var header string
	if m.filterMode && filteredCount < totalFiles {
		header = fmt.Sprintf("Files (%d/%d filtered)", filteredCount, totalFiles)
	} else {
		header = fmt.Sprintf("Files (%d/%d)", m.selected+1, totalFiles)
	}
	lines = append(lines, styleHeader(header, width))

	// Table header
	typeColWidth := 4
	statsColWidth := 12
	pathColWidth := width - typeColWidth - statsColWidth - 4

	headerLine := fmt.Sprintf("%-*s  %-*s  %*s",
		typeColWidth, "Type",
		pathColWidth, "Path",
		statsColWidth, "Stats")
	lines = append(lines, lipgloss.NewStyle().Foreground(theme.Secondary).Bold(true).Render(headerLine))

	// Calculate visible range
	visibleHeight := height - 2 // Subtract header rows
	if m.filterMode {
		visibleHeight -= 2 // Subtract filter input + blank line
	}

	// Find selected row in display list
	selectedDisplayIdx := -1
	for i, idx := range displayIndices {
		if idx == m.selected {
			selectedDisplayIdx = i
			break
		}
	}

	// Calculate scroll offset to center the selected row
	var startIdx int
	if selectedDisplayIdx >= 0 {
		// Center the selected row
		centerOffset := selectedDisplayIdx - visibleHeight/2
		if centerOffset < 0 {
			startIdx = 0
		} else if centerOffset+visibleHeight > len(displayFiles) {
			// Don't scroll past the end
			startIdx = len(displayFiles) - visibleHeight
			if startIdx < 0 {
				startIdx = 0
			}
		} else {
			startIdx = centerOffset
		}
	} else {
		startIdx = m.scrollOffset
	}

	endIdx := startIdx + visibleHeight
	if endIdx > len(displayFiles) {
		endIdx = len(displayFiles)
	}

	// Render files
	for i := startIdx; i < endIdx; i++ {
		file := displayFiles[i]
		originalIdx := displayIndices[i]
		isSelected := originalIdx == m.selected

		var additions, deletions int
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if line.Type == diff.LineAddition {
					additions++
				} else if line.Type == diff.LineDeletion {
					deletions++
				}
			}
		}

		changeType := file.ChangeType.String()
		path := file.Path
		if len(path) > pathColWidth {
			path = path[:pathColWidth-3] + "..."
		}

		stats := fmt.Sprintf("+%-3d -%-3d", additions, deletions)

		line := fmt.Sprintf("%-*s  %-*s  %*s", typeColWidth, changeType, pathColWidth, path, statsColWidth, stats)

		// Apply selection styling to content only, not padding
		if isSelected {
			if focused {
				styledLine := lipgloss.NewStyle().
					Background(theme.ModalBg).
					Foreground(theme.Primary).
					Render(line)
				lines = append(lines, styledLine+strings.Repeat(" ", width-len(line)))
			} else {
				styledLine := lipgloss.NewStyle().
					Background(theme.MutedBg).
					Foreground(theme.Text).
					Render(line)
				lines = append(lines, styledLine+strings.Repeat(" ", width-len(line)))
			}
		} else {
			lines = append(lines, truncateOrPad(line, width))
		}
	}

	// Fill remaining space
	targetHeight := height
	if m.filterMode {
		targetHeight -= 2 // Leave space for filter input
	}
	for len(lines) < targetHeight {
		lines = append(lines, strings.Repeat(" ", width))
	}

	// Add filter input at bottom if in filter mode
	if m.filterMode {
		lines = append(lines, "")
		filterLine := fmt.Sprintf("Filter: %s█", m.filterQuery)
		lines = append(lines, lipgloss.NewStyle().
			Foreground(theme.Accent).
			Render(truncateOrPad(filterLine, width)))
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

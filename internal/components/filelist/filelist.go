// Package filelist renders the changed-file list, either as a one-line summary of the selected file
// or as a full list, with an inline filter over the paths.
package filelist

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/fuzzy"
	"github.com/kyleking/jj-diff/internal/theme"
)

// MatchRange is a search hit inside a file path, as byte offsets with End exclusive.
type MatchRange struct {
	Start int
	End   int
}

// Model is the file list's state. Every mutator takes a pointer receiver, so a caller holding a
// value must copy the result back into its own state.
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

// New returns a collapsed, unfiltered list with no files.
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

// SetExpanded switches between the one-line summary and the full list.
func (m *Model) SetExpanded(expanded bool) {
	m.expanded = expanded
}

// IsExpanded reports whether the full list is drawn.
func (m Model) IsExpanded() bool {
	return m.expanded
}

// SetFilterMode opens or closes the inline path filter. Closing it also clears the query, so a
// reopened filter starts empty.
func (m *Model) SetFilterMode(enabled bool) {
	m.filterMode = enabled
	if !enabled {
		m.filterQuery = ""
	}
}

// IsFilterMode reports whether the inline filter is open.
func (m Model) IsFilterMode() bool {
	return m.filterMode
}

// SetFilterQuery replaces the filter text. It does not move the selection, so the diff pane keeps
// showing whatever file was selected before the filter narrowed the list.
func (m *Model) SetFilterQuery(query string) {
	m.filterQuery = query
}

// FilterQuery returns the current filter text.
func (m Model) FilterQuery() string {
	return m.filterQuery
}

// SetFiles replaces the list contents. It leaves the selected index alone, so a caller shrinking the
// list must clamp the selection itself.
func (m *Model) SetFiles(files []diff.FileChange) {
	m.files = files
}

// SetSelected moves the cursor to an index into the unfiltered file slice.
func (m *Model) SetSelected(idx int) {
	m.selected = idx
}

// SetSearchState stores the hit lookup for path highlighting. The render path does not read it yet,
// so setting it has no visible effect.
func (m *Model) SetSearchState(isSearching bool, getMatches func(fileIdx int) []MatchRange) {
	m.isSearching = isSearching
	m.getMatches = getMatches
}

// View renders the list at the given size, using only one row when collapsed regardless of height.
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
			switch line.Type {
			case diff.LineAddition:
				additions++
			case diff.LineDeletion:
				deletions++
			case diff.LineContext:
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
		displayFiles = make([]diff.FileChange, 0, len(matches))
		displayIndices = make([]int, 0, len(matches))
		for _, match := range matches {
			idx, ok := match.Original.(int)
			if !ok || idx < 0 || idx >= len(m.files) {
				continue
			}
			displayFiles = append(displayFiles, m.files[idx])
			displayIndices = append(displayIndices, idx)
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
	pathColWidth := max(width-typeColWidth-statsColWidth-4, len(ellipsis)+1)

	headerLine := fmt.Sprintf("%-*s  %-*s  %*s",
		typeColWidth, "Type",
		pathColWidth, "Path",
		statsColWidth, "Stats")
	lines = append(
		lines,
		lipgloss.NewStyle().Foreground(theme.Secondary).Bold(true).Render(headerLine),
	)

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
				switch line.Type {
				case diff.LineAddition:
					additions++
				case diff.LineDeletion:
					deletions++
				case diff.LineContext:
				}
			}
		}

		changeType := file.ChangeType.String()
		path := file.Path
		if len(path) > pathColWidth {
			path = path[:pathColWidth-3] + "..."
		}

		stats := fmt.Sprintf("+%-3d -%-3d", additions, deletions)

		line := fmt.Sprintf(
			"%-*s  %-*s  %*s",
			typeColWidth,
			changeType,
			pathColWidth,
			path,
			statsColWidth,
			stats,
		)

		// Apply selection styling to content only, not padding
		if isSelected {
			if focused {
				styledLine := lipgloss.NewStyle().
					Background(theme.ModalBg).
					Foreground(theme.Primary).
					Render(line)
				lines = append(lines, styledLine+strings.Repeat(" ", max(width-len(line), 0)))
			} else {
				styledLine := lipgloss.NewStyle().
					Background(theme.MutedBg).
					Foreground(theme.Text).
					Render(line)
				lines = append(lines, styledLine+strings.Repeat(" ", max(width-len(line), 0)))
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
		lines = append(lines, strings.Repeat(" ", max(width, 0)))
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

func styleHeader(text string, width int) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary)

	return style.Render(truncateOrPad(text, width))
}

// truncateOrPad fits text to width. Width below the ellipsis length leaves no
// room for a truncation marker, so the text is cut without one.
const ellipsis = "..."

func truncateOrPad(text string, width int) string {
	if width <= 0 {
		return ""
	}

	if len(text) > width {
		if width <= len(ellipsis) {
			return text[:width]
		}

		return text[:width-len(ellipsis)] + ellipsis
	}

	return text + strings.Repeat(" ", max(width-len(text), 0))
}

func padToSize(text string, width, height int) string {
	lines := []string{text}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", max(width, 0)))
	}

	return strings.Join(lines, "\n")
}

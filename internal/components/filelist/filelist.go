// Package filelist renders the changed-file list, either as a one-line summary of the selected file
// or as a full list, with an inline filter over the paths.
package filelist

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/fuzzy"
	"github.com/kyleking/jj-diff/internal/theme"
)

const (
	centerDivisor = 2
	columnGap     = 2
	filterRows    = 2
	headerRows    = 1
)

// MatchRange is a search hit inside a file path, as byte offsets with End exclusive.
type MatchRange struct {
	Start int
	End   int
}

// Model is the file list's state. Every method takes a pointer receiver.
type Model struct {
	getMatches   func(fileIdx int) []MatchRange
	filterQuery  string
	files        []diff.FileChange
	counts       []changeCounts
	selected     int
	scrollOffset int
	isSearching  bool
	expanded     bool
	filterMode   bool
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
func (m *Model) IsExpanded() bool {
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
func (m *Model) IsFilterMode() bool {
	return m.filterMode
}

// SetFilterQuery replaces the filter text. It does not move the selection, so the diff pane keeps
// showing whatever file was selected before the filter narrowed the list.
func (m *Model) SetFilterQuery(query string) {
	m.filterQuery = query
}

// FilterQuery returns the current filter text.
func (m *Model) FilterQuery() string {
	return m.filterQuery
}

// SetFiles replaces the list contents. It leaves the selected index alone, so a caller shrinking the
// list must clamp the selection itself.
func (m *Model) SetFiles(files []diff.FileChange) {
	m.files = files
	m.counts = make([]changeCounts, len(files))

	for i := range files {
		m.counts[i] = countChanges(files[i].Hunks)
	}
}

// ContentHeight is the rows the expanded list needs for every file. Filtering does not shrink it.
func (m *Model) ContentHeight() int {
	rows := headerRows + len(m.files)
	if m.filterMode {
		rows += filterRows
	}

	return rows
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
func (m *Model) View(width, height int, focused bool) string {
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

func (m *Model) renderCollapsed(width int, focused bool) string {
	if len(m.files) == 0 {
		return padToSize("No files", width, 1)
	}

	file := m.files[m.selected]
	counter := fmt.Sprintf(" [%d/%d]", m.selected+1, len(m.files))

	changeType := file.ChangeType.String()
	path := file.Path

	stats := m.counts[m.selected].plain()

	line := fmt.Sprintf("[%s] %s %s%s", changeType, path, stats, counter)

	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary)
	if focused {
		style = theme.SelectedFocusedStyle
	}

	return style.Render(truncateOrPad(line, width))
}

func (m *Model) renderExpanded(width, height int, focused bool) string {
	displayIndices := m.visibleIndices()
	layout := m.columns(width)

	lines := []string{styleHeader(m.headerText(len(displayIndices)), width)}

	visibleHeight := height - headerRows
	if m.filterMode {
		visibleHeight -= filterRows
	}

	startIdx := m.scrollStart(displayIndices, visibleHeight, len(displayIndices))
	endIdx := min(startIdx+visibleHeight, len(displayIndices))

	for i := startIdx; i < endIdx; i++ {
		lines = append(lines, m.renderRow(displayIndices[i], layout, focused))
	}

	targetHeight := height
	if m.filterMode {
		targetHeight -= filterRows
	}
	for len(lines) < targetHeight {
		lines = append(lines, strings.Repeat(" ", max(width, 0)))
	}

	if m.filterMode {
		filterLine := fmt.Sprintf("Filter: %s\u2588", m.filterQuery)
		lines = append(lines, "", lipgloss.NewStyle().
			Foreground(theme.Accent).
			Render(truncateOrPad(filterLine, width)))
	}

	return strings.Join(lines, "\n")
}

// visibleIndices returns the rows to draw, in display order, as indices into the unfiltered file
// slice. Every index is in range, so callers may subscript m.files with one directly.
func (m *Model) visibleIndices() []int {
	if !m.filterMode || m.filterQuery == "" {
		indices := make([]int, len(m.files))
		for i := range m.files {
			indices[i] = i
		}

		return indices
	}

	filePaths := make([]string, len(m.files))
	fileData := make([]any, len(m.files))
	for i, f := range m.files {
		filePaths[i] = f.Path
		fileData[i] = i
	}

	matches := fuzzy.FilterWithData(m.filterQuery, filePaths, fileData)
	indices := make([]int, 0, len(matches))
	for _, match := range matches {
		idx, ok := match.Original.(int)
		if !ok || idx < 0 || idx >= len(m.files) {
			continue
		}
		indices = append(indices, idx)
	}

	return indices
}

func (m *Model) headerText(filteredCount int) string {
	if m.filterMode && filteredCount < len(m.files) {
		return fmt.Sprintf("Files (%d/%d filtered)", filteredCount, len(m.files))
	}

	return fmt.Sprintf("Files (%d/%d)", m.selected+1, len(m.files))
}

// scrollStart centers the selected row in the visible window, falling back to the stored scroll
// offset when the selection is filtered out of the display list.
func (m *Model) scrollStart(displayIndices []int, visibleHeight, total int) int {
	selectedDisplayIdx := slices.Index(displayIndices, m.selected)
	if selectedDisplayIdx < 0 {
		return m.scrollOffset
	}

	centerOffset := selectedDisplayIdx - visibleHeight/centerDivisor
	switch {
	case centerOffset < 0:
		return 0
	case centerOffset+visibleHeight > total:
		return max(total-visibleHeight, 0)
	default:
		return centerOffset
	}
}

type columnLayout struct {
	width, typeWidth, pathWidth, addWidth, delWidth int
}

func (m *Model) columns(width int) columnLayout {
	layout := columnLayout{width: width, typeWidth: 1}

	for i, file := range m.files {
		layout.typeWidth = max(layout.typeWidth, lipgloss.Width(file.ChangeType.String()))
		layout.addWidth = max(layout.addWidth, len(fmt.Sprintf("+%d", m.counts[i].additions)))
		layout.delWidth = max(layout.delWidth, len(fmt.Sprintf("-%d", m.counts[i].deletions)))
	}

	stats := layout.addWidth + 1 + layout.delWidth
	layout.pathWidth = max(width-layout.typeWidth-stats-2*columnGap, len(ellipsis)+1)

	return layout
}

func (m *Model) renderRow(originalIdx int, layout columnLayout, focused bool) string {
	file := m.files[originalIdx]
	counts := m.counts[originalIdx]

	base := lipgloss.NewStyle()
	if originalIdx == m.selected {
		base = theme.SelectedUnfocusedStyle
		if focused {
			base = theme.SelectedFocusedStyle
		}
	}

	gap := base.Render(strings.Repeat(" ", columnGap))
	added, deleted := base.Foreground(theme.AddedLine), base.Foreground(theme.DeletedLine)

	row := base.Render(fmt.Sprintf("%-*s", layout.typeWidth, file.ChangeType.String())) + gap +
		base.Render(fitPath(file.Path, layout.pathWidth)) + gap +
		statCell("+", counts.additions, layout.addWidth, &added) +
		base.Render(" ") +
		statCell("-", counts.deletions, layout.delWidth, &deleted)

	return ansi.Truncate(row, layout.width, "")
}

// statCell leaves the cell blank for a zero count.
func statCell(sign string, n, width int, style *lipgloss.Style) string {
	if n == 0 {
		return style.Render(strings.Repeat(" ", width))
	}

	return style.Render(fmt.Sprintf("%*s", width, sign+strconv.Itoa(n)))
}

func fitPath(path string, width int) string {
	if lipgloss.Width(path) > width {
		path = ansi.Truncate(path, width-len(ellipsis), "") + ellipsis
	}

	return path + strings.Repeat(" ", width-lipgloss.Width(path))
}

type changeCounts struct {
	additions int
	deletions int
}

func (c changeCounts) plain() string {
	var parts []string
	if c.additions > 0 {
		parts = append(parts, "+"+strconv.Itoa(c.additions))
	}

	if c.deletions > 0 {
		parts = append(parts, "-"+strconv.Itoa(c.deletions))
	}

	return strings.Join(parts, " ")
}

func countChanges(hunks []diff.Hunk) changeCounts {
	var counts changeCounts
	for _, hunk := range hunks {
		for _, line := range hunk.Lines {
			switch line.Type {
			case diff.LineAddition:
				counts.additions++
			case diff.LineDeletion:
				counts.deletions++
			case diff.LineContext:
			}
		}
	}

	return counts
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

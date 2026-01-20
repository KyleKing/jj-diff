package diffview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/highlight"
	"github.com/kyleking/jj-diff/internal/theme"
)

type MatchRange struct {
	Start int
	End   int
}

type ViewModeType string

const (
	ViewModeUnified    ViewModeType = "unified"
	ViewModeSideBySide ViewModeType = "side-by-side"
)

type WordDiffCache struct {
	HunkDiffs map[int]map[int]diff.WordDiffResult
}

type Model struct {
	fileChange      *diff.FileChange
	offset          int
	selectedHunk    int
	lineCursor      int
	isVisualMode    bool
	visualAnchor    int
	isSelected      func(hunkIdx int) bool
	isLineSelected  func(hunkIdx, lineIdx int) bool
	getMatches      func(hunkIdx, lineIdx int) []MatchRange
	isSearching     bool
	highlighter     *highlight.Highlighter
	enableHighlight bool

	viewMode        ViewModeType
	showWhitespace  bool
	showLineNumbers bool
	tabWidth        int
	wordLevelDiff   bool
	wordDiffCache   *WordDiffCache
}

func New(cfg config.Config) Model {
	viewMode := ViewModeUnified
	if cfg.ViewMode == config.ViewModeSideBySide {
		viewMode = ViewModeSideBySide
	}
	return Model{
		offset:          0,
		highlighter:     highlight.New(),
		enableHighlight: true,
		viewMode:        viewMode,
		showWhitespace:  cfg.ShowWhitespace,
		showLineNumbers: cfg.ShowLineNumbers,
		tabWidth:        cfg.TabWidth,
		wordLevelDiff:   cfg.WordLevelDiff,
	}
}

func (m *Model) SetFileChange(file diff.FileChange) {
	m.fileChange = &file
	m.offset = 0
	m.computeWordDiffs()
}

func (m *Model) computeWordDiffs() {
	if m.fileChange == nil || !m.wordLevelDiff {
		m.wordDiffCache = nil
		return
	}

	m.wordDiffCache = &WordDiffCache{
		HunkDiffs: make(map[int]map[int]diff.WordDiffResult),
	}

	for hunkIdx := range m.fileChange.Hunks {
		hunk := &m.fileChange.Hunks[hunkIdx]
		hunkWordDiffs := diff.ComputeHunkWordDiffs(hunk)
		m.wordDiffCache.HunkDiffs[hunkIdx] = hunkWordDiffs
	}
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

func (m *Model) ToggleWhitespace() {
	m.showWhitespace = !m.showWhitespace
}

func (m *Model) ToggleLineNumbers() {
	m.showLineNumbers = !m.showLineNumbers
}

func (m *Model) ToggleWordDiff() {
	m.wordLevelDiff = !m.wordLevelDiff
	m.computeWordDiffs()
}

func (m *Model) ToggleSideBySide() {
	if m.viewMode == ViewModeUnified {
		m.viewMode = ViewModeSideBySide
	} else {
		m.viewMode = ViewModeUnified
	}
}

func (m *Model) IsSideBySide() bool {
	return m.viewMode == ViewModeSideBySide
}

func (m *Model) ShowWhitespace() bool {
	return m.showWhitespace
}

func (m *Model) ShowLineNumbers() bool {
	return m.showLineNumbers
}

func (m *Model) WordLevelDiff() bool {
	return m.wordLevelDiff
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

func (m *Model) ScrollHalfPageDown(viewHeight int) {
	m.Scroll(viewHeight / 2)
}

func (m *Model) ScrollHalfPageUp(viewHeight int) {
	m.Scroll(-viewHeight / 2)
}

func (m *Model) ScrollFullPageDown(viewHeight int) {
	m.Scroll(viewHeight)
}

func (m *Model) ScrollFullPageUp(viewHeight int) {
	m.Scroll(-viewHeight)
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

func (m Model) View(width, height int, focused bool) string {
	if m.fileChange == nil {
		return padToSize("No file selected", width, height)
	}

	if m.viewMode == ViewModeSideBySide {
		ctx := RenderContext{
			Width:           width,
			Height:          height,
			SelectedHunk:    m.selectedHunk,
			LineCursor:      m.lineCursor,
			IsVisualMode:    m.isVisualMode,
			VisualAnchor:    m.visualAnchor,
			ShowWhitespace:  m.showWhitespace,
			ShowLineNumbers: m.showLineNumbers,
			TabWidth:        m.tabWidth,
			WordLevelDiff:   m.wordLevelDiff,
			IsSearching:     m.isSearching,
			IsSelected:      m.isSelected,
			IsLineSelected:  m.isLineSelected,
			GetMatches:      m.getMatches,
			WordDiffCache:   m.wordDiffCache,
			Focused:         focused,
		}
		sbs := NewSideBySideView()
		return sbs.Render(m.fileChange, ctx)
	}

	return m.renderUnified(width, height, focused)
}

func (m Model) renderUnified(width, height int, focused bool) string {
	var lines []string

	currentLine := 0
	for hunkIdx, hunk := range m.fileChange.Hunks {
		if currentLine >= m.offset && len(lines) < height {
			isCurrent := hunkIdx == m.selectedHunk
			isHunkSelected := m.isSelected != nil && m.isSelected(hunkIdx)
			lines = append(lines, m.renderHunkHeader(hunk.Header, width, isCurrent, isHunkSelected))
		}
		currentLine++

		// Process hunk lines to hide whitespace changes if enabled
		hunkLines := hunk.Lines
		if m.showWhitespace {
			hunkLines = diff.ProcessHunkHideWhitespace(hunk.Lines)
		}

		for lineIdx, line := range hunkLines {
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
	lineNumStr := ""
	if m.showLineNumbers {
		switch line.Type {
		case diff.LineAddition:
			lineNumStr = fmt.Sprintf("%4d ", line.NewLineNum)
		case diff.LineDeletion:
			lineNumStr = fmt.Sprintf("%4d ", line.OldLineNum)
		default:
			lineNumStr = fmt.Sprintf("%4d ", line.NewLineNum)
		}
	}

	prefix := line.Type.String()
	content := line.Content

	maxContentWidth := width - 8
	if !m.showLineNumbers {
		maxContentWidth = width - 4
	}
	if len(content) > maxContentWidth {
		content = content[:maxContentWidth]
	}

	if m.wordLevelDiff && m.wordDiffCache != nil && line.Type != diff.LineContext {
		if hunkDiffs, ok := m.wordDiffCache.HunkDiffs[hunkIdx]; ok {
			if wordDiff, ok := hunkDiffs[lineIdx]; ok {
				content = m.applyWordDiffHighlight(line.Content, line.Type, wordDiff)
			}
		}
	} else if m.enableHighlight && m.fileChange != nil && line.Type == diff.LineContext {
		highlighted := m.highlighter.HighlightLine(m.fileChange.Path, content)
		if highlighted != "" {
			content = highlighted
		}
	}

	if m.isSearching && m.getMatches != nil {
		matches := m.getMatches(hunkIdx, lineIdx)
		if len(matches) > 0 {
			content = m.highlightMatches(content, matches)
		}
	}

	isCurrentLine := m.lineCursor == lineIdx && hunkIdx == m.selectedHunk
	isInVisualRange := m.isVisualMode && hunkIdx == m.selectedHunk && m.isLineInVisualRange(lineIdx)
	isSelected := m.isLineSelected != nil && m.isLineSelected(hunkIdx, lineIdx)

	lineIndicator := "  "
	if isInVisualRange {
		lineIndicator = "█ "
	} else if isSelected {
		lineIndicator = "• "
	} else if isCurrentLine {
		lineIndicator = "> "
	}

	lineText := fmt.Sprintf("%s%s%s %s", lineIndicator, lineNumStr, prefix, content)

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

func (m Model) applyWordDiffHighlight(content string, lineType diff.LineType, wordDiff diff.WordDiffResult) string {
	var spans []diff.IntraLineSpan
	switch lineType {
	case diff.LineDeletion:
		spans = wordDiff.OldSpans
	case diff.LineAddition:
		spans = wordDiff.NewSpans
	default:
		return content
	}

	if len(spans) == 0 {
		return content
	}

	var result strings.Builder
	for _, span := range spans {
		text := span.Text
		switch span.Type {
		case diff.SpanEqual:
			result.WriteString(text)
		case diff.SpanDeleted:
			styled := theme.WordDiffDeletedStyle.Render(text)
			result.WriteString(styled)
		case diff.SpanAdded:
			styled := theme.WordDiffAddedStyle.Render(text)
			result.WriteString(styled)
		}
	}

	return result.String()
}

func styleHeader(text string, width int, focused bool) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary)
	if focused {
		style = style.Background(theme.MutedBg)
	}
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

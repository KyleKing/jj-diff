// Package diffview renders one file's hunks as a scrollable pane, either unified or side by side.
// It draws the hunk and line cursors, selection markers, split tags, search matches, and syntax
// or word-level highlighting, and owns no keybindings of its own.
package diffview

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/highlight"
	"github.com/kyleking/jj-diff/internal/theme"
)

// Row layout of the unified view, in terminal cells. A rendered row is the cursor indicator, the
// line-number gutter when it is drawn, the +/-/space marker, and a space, so the content that fits
// is the pane width less that chrome.
const (
	halfDivisor             = 2
	numberedLineChromeWidth = 8
	plainLineChromeWidth    = 4
)

// MatchRange is a half-open byte range within one line's raw content, used to underlay search hits.
// Offsets are byte offsets rather than rune or column positions.
type MatchRange struct {
	Start int
	End   int
}

// ViewModeType names the layout the pane renders in.
type ViewModeType string

// The two layouts a pane can render in. The values match the strings accepted by config.ViewMode.
const (
	ViewModeUnified    ViewModeType = "unified"
	ViewModeSideBySide ViewModeType = "side-by-side"
)

// WordDiffCache holds precomputed intra-line diffs keyed by hunk index, then by line index within
// that hunk. Only added and deleted lines get an entry, so a lookup miss means render the line plain.
type WordDiffCache struct {
	HunkDiffs map[int]map[int]diff.WordDiffResult
}

// SplitTag is the single character a hunk is tagged with during a multi-way split, drawn as "[a]"
// beside the hunk header.
type SplitTag rune

// LineIndex maps a scroll offset to a hunk without walking every hunk. Offsets and counts are in
// rendered rows, and each hunk's first row is its header, so HunkLineCounts is one more than the
// hunk's line count. Hiding whitespace changes the row counts, so the index is rebuilt on that toggle.
type LineIndex struct {
	HunkOffsets    []int
	HunkLineCounts []int
	TotalLines     int
}

// HunkPosition locates a rendered row. HunkIdx is the hunk the row belongs to and LineInHunk is the
// row within it, where row 0 is the hunk header.
type HunkPosition struct {
	HunkIdx    int
	LineInHunk int
}

// FindHunkForOffset maps a rendered row to its hunk and the row within that hunk. An offset past the
// last row clamps to the end of the final hunk, and an empty index returns the zero position.
func (idx *LineIndex) FindHunkForOffset(offset int) HunkPosition {
	if len(idx.HunkOffsets) == 0 {
		return HunkPosition{}
	}

	if offset >= idx.TotalLines {
		lastHunk := len(idx.HunkOffsets) - 1

		return HunkPosition{HunkIdx: lastHunk, LineInHunk: idx.HunkLineCounts[lastHunk]}
	}

	left, right := 0, len(idx.HunkOffsets)-1
	for left < right {
		mid := (left + right + 1) / halfDivisor
		if idx.HunkOffsets[mid] <= offset {
			left = mid
		} else {
			right = mid - 1
		}
	}

	return HunkPosition{HunkIdx: left, LineInHunk: offset - idx.HunkOffsets[left]}
}

// Model is the diff pane. It holds no cursor authority of its own: the parent model pushes the
// selected hunk, line cursor, search state, and tag state in before each render.
type Model struct {
	getHunkTags     func(hunkIdx int) []SplitTag
	lineIndex       *LineIndex
	wordDiffCache   *WordDiffCache
	highlighter     *highlight.Highlighter
	fileChange      *diff.FileChange
	isSelected      func(hunkIdx int) bool
	isLineSelected  func(hunkIdx, lineIdx int) bool
	getMatches      func(hunkIdx, lineIdx int) []MatchRange
	viewMode        ViewModeType
	visualAnchor    int
	lineCursor      int
	tabWidth        int
	selectedHunk    int
	offset          int
	isSearching     bool
	isVisualMode    bool
	enableHighlight bool
	showWhitespace  bool
	showLineNumbers bool
	wordLevelDiff   bool
}

// New builds a pane with no file loaded, so View renders a placeholder until SetFileChange is called.
// Everything the user can toggle at runtime takes its starting value from cfg.
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

// SetFileChange loads a file, scrolls back to the top, and rebuilds the word-diff cache and line
// index. The pane keeps its own copy of file, so later edits to the caller's value are not picked up.
func (m *Model) SetFileChange(file diff.FileChange) {
	m.fileChange = &file
	m.offset = 0
	m.computeWordDiffs()
	m.buildLineIndex()
}

func (m *Model) buildLineIndex() {
	if m.fileChange == nil {
		m.lineIndex = nil
		return
	}

	index := &LineIndex{
		HunkOffsets:    make([]int, len(m.fileChange.Hunks)),
		HunkLineCounts: make([]int, len(m.fileChange.Hunks)),
	}

	totalLines := 0
	for hunkIdx, hunk := range m.fileChange.Hunks {
		index.HunkOffsets[hunkIdx] = totalLines

		hunkLines := hunk.Lines
		if m.showWhitespace {
			hunkLines = diff.ProcessHunkHideWhitespace(hunk.Lines)
		}

		linesInHunk := 1 + len(hunkLines)
		index.HunkLineCounts[hunkIdx] = linesInHunk
		totalLines += linesInHunk
	}

	index.TotalLines = totalLines
	m.lineIndex = index
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

// SetSelection points the hunk cursor at selectedHunk and installs the predicate that decides which
// hunks draw as selected. A nil isSelected draws no hunk as selected, which is the browse-mode case.
func (m *Model) SetSelection(selectedHunk int, isSelected func(hunkIdx int) bool) {
	m.selectedHunk = selectedHunk
	m.isSelected = isSelected
}

// SetVisualState installs the line cursor and visual-mode range. Both lineCursor and visualAnchor are
// line indexes within the hunk the cursor is on, and the range they bound is inclusive at both ends.
func (m *Model) SetVisualState(
	lineCursor int,
	isVisualMode bool,
	visualAnchor int,
	isLineSelected func(hunkIdx, lineIdx int) bool,
) {
	m.lineCursor = lineCursor
	m.isVisualMode = isVisualMode
	m.visualAnchor = visualAnchor
	m.isLineSelected = isLineSelected
}

// SetSearchState turns match highlighting on or off. Highlighting needs both isSearching and a
// non-nil getMatches, so passing false with nil is the way to clear a finished search.
func (m *Model) SetSearchState(
	isSearching bool,
	getMatches func(hunkIdx, lineIdx int) []MatchRange,
) {
	m.isSearching = isSearching
	m.getMatches = getMatches
}

// SetTagState installs the lookup for split tags drawn beside each hunk header. The callback is asked
// once per visible hunk per render, so it must be cheap. A nil callback draws no tags.
func (m *Model) SetTagState(getHunkTags func(hunkIdx int) []SplitTag) {
	m.getHunkTags = getHunkTags
}

// ToggleWhitespace flips visible whitespace glyphs and rebuilds the line index, because the glyphs
// change how wide a rendered line is.
func (m *Model) ToggleWhitespace() {
	m.showWhitespace = !m.showWhitespace
	m.buildLineIndex()
}

// ToggleLineNumbers flips the old/new line-number gutter.
func (m *Model) ToggleLineNumbers() {
	m.showLineNumbers = !m.showLineNumbers
}

// ToggleWordDiff flips intra-line highlighting and recomputes the word diffs for the current file,
// which costs a pass over every changed line pair.
func (m *Model) ToggleWordDiff() {
	m.wordLevelDiff = !m.wordLevelDiff
	m.computeWordDiffs()
}

// ToggleSideBySide switches between the unified and side-by-side layouts.
func (m *Model) ToggleSideBySide() {
	if m.viewMode == ViewModeUnified {
		m.viewMode = ViewModeSideBySide
	} else {
		m.viewMode = ViewModeUnified
	}
}

// IsSideBySide reports the current layout, which the status bar reads to label the toggle.
func (m *Model) IsSideBySide() bool {
	return m.viewMode == ViewModeSideBySide
}

// ShowWhitespace reports whether whitespace glyphs are drawn.
func (m *Model) ShowWhitespace() bool {
	return m.showWhitespace
}

// ShowLineNumbers reports whether the line-number gutter is drawn.
func (m *Model) ShowLineNumbers() bool {
	return m.showLineNumbers
}

// WordLevelDiff reports whether intra-line highlighting is on.
func (m *Model) WordLevelDiff() bool {
	return m.wordLevelDiff
}

// Scroll moves the viewport by delta lines, clamped to the file's rendered height. It does nothing
// when no file is loaded, so callers need not check first.
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

// ScrollHalfPageDown scrolls down half of viewHeight. Pass the pane's height, not the terminal's.
func (m *Model) ScrollHalfPageDown(viewHeight int) {
	m.Scroll(viewHeight / halfDivisor)
}

// ScrollHalfPageUp scrolls up half of viewHeight. Pass the pane's height, not the terminal's.
func (m *Model) ScrollHalfPageUp(viewHeight int) {
	m.Scroll(-viewHeight / halfDivisor)
}

// ScrollFullPageDown scrolls down viewHeight lines. Pass the pane's height, not the terminal's.
func (m *Model) ScrollFullPageDown(viewHeight int) {
	m.Scroll(viewHeight)
}

// ScrollFullPageUp scrolls up viewHeight lines. Pass the pane's height, not the terminal's.
func (m *Model) ScrollFullPageUp(viewHeight int) {
	m.Scroll(-viewHeight)
}

func (m *Model) calculateTotalLines() int {
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

// View renders the pane at the given size, padding out to it when the content is shorter. The
// focused flag only changes cursor styling, so an unfocused pane still shows where the cursor sits.
func (m *Model) View(width, height int, focused bool) string {
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

		return sbs.Render(m.fileChange, &ctx)
	}

	return m.renderUnified(width, height)
}

func (m *Model) renderUnified(width, height int) string {
	var lines []string

	if m.lineIndex == nil || len(m.fileChange.Hunks) == 0 {
		for len(lines) < height {
			lines = append(lines, strings.Repeat(" ", max(width, 0)))
		}

		return strings.Join(lines, "\n")
	}

	start := m.lineIndex.FindHunkForOffset(m.offset)
	lineInHunk := start.LineInHunk

	for hunkIdx := start.HunkIdx; hunkIdx < len(m.fileChange.Hunks) && len(lines) < height; hunkIdx++ {
		hunk := m.fileChange.Hunks[hunkIdx]

		if lineInHunk == 0 {
			isCurrent := hunkIdx == m.selectedHunk
			isHunkSelected := m.isSelected != nil && m.isSelected(hunkIdx)
			lines = append(
				lines,
				m.renderHunkHeader(hunk.Header, width, hunkIdx, isCurrent, isHunkSelected),
			)
		}

		hunkLines := hunk.Lines
		if m.showWhitespace {
			hunkLines = diff.ProcessHunkHideWhitespace(hunk.Lines)
		}

		startLineIdx := lineInHunk
		if lineInHunk > 0 {
			startLineIdx = lineInHunk - 1
		}

		for lineIdx := startLineIdx; lineIdx < len(hunkLines) && len(lines) < height; lineIdx++ {
			line := hunkLines[lineIdx]
			lines = append(lines, m.renderLine(line, width, hunkIdx, lineIdx))
		}

		lineInHunk = 0
	}

	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", max(width, 0)))
	}

	return strings.Join(lines, "\n")
}

func (m *Model) renderLine(line diff.Line, width, hunkIdx, lineIdx int) string {
	isCurrentLine := m.isVisualMode && m.lineCursor == lineIdx && hunkIdx == m.selectedHunk
	isInVisualRange := m.isVisualMode && hunkIdx == m.selectedHunk && m.isLineInVisualRange(lineIdx)
	isSelected := m.isLineSelected != nil && m.isLineSelected(hunkIdx, lineIdx)

	lineText := fmt.Sprintf(
		"%s%s%s %s",
		lineIndicator(isInVisualRange, isSelected, isCurrentLine),
		m.lineNumberGutter(line),
		line.Type.String(),
		m.decorateContent(line, width, hunkIdx, lineIdx),
	)

	style := lineStyle(line.Type, isInVisualRange, isCurrentLine)

	return style.Render(truncateOrPad(lineText, width))
}

func (m *Model) lineNumberGutter(line diff.Line) string {
	if !m.showLineNumbers {
		return ""
	}

	if line.Type == diff.LineDeletion {
		return fmt.Sprintf("%4d ", line.OldLineNum)
	}

	return fmt.Sprintf("%4d ", line.NewLineNum)
}

func (m *Model) decorateContent(line diff.Line, width, hunkIdx, lineIdx int) string {
	maxContentWidth := width - numberedLineChromeWidth
	if !m.showLineNumbers {
		maxContentWidth = width - plainLineChromeWidth
	}

	content := line.Content
	if len(content) > maxContentWidth {
		content = content[:maxContentWidth]
	}

	if wordDiff, ok := m.lookupWordDiff(line.Type, hunkIdx, lineIdx); ok {
		content = applyWordDiffHighlight(line.Content, line.Type, wordDiff)
	} else if m.enableHighlight && m.fileChange != nil && line.Type == diff.LineContext {
		if highlighted := m.highlighter.HighlightLine(m.fileChange.Path, content); highlighted != "" {
			content = highlighted
		}
	}

	if m.isSearching && m.getMatches != nil {
		if matches := m.getMatches(hunkIdx, lineIdx); len(matches) > 0 {
			content = highlightMatches(content, matches)
		}
	}

	return content
}

func (m *Model) lookupWordDiff(
	lineType diff.LineType,
	hunkIdx, lineIdx int,
) (diff.WordDiffResult, bool) {
	if !m.wordLevelDiff || m.wordDiffCache == nil || lineType == diff.LineContext {
		return diff.WordDiffResult{}, false
	}

	hunkDiffs, ok := m.wordDiffCache.HunkDiffs[hunkIdx]
	if !ok {
		return diff.WordDiffResult{}, false
	}

	wordDiff, ok := hunkDiffs[lineIdx]

	return wordDiff, ok
}

func lineIndicator(isInVisualRange, isSelected, isCurrentLine bool) string {
	switch {
	case isInVisualRange:
		return "█ "
	case isSelected:
		return "• "
	case isCurrentLine:
		return "> "
	default:
		return "  "
	}
}

func lineStyle(lineType diff.LineType, isInVisualRange, isCurrentLine bool) lipgloss.Style {
	style := lipgloss.NewStyle()

	switch lineType {
	case diff.LineContext:
	case diff.LineAddition:
		style = style.Foreground(theme.AddedLine)
	case diff.LineDeletion:
		style = style.Foreground(theme.DeletedLine)
	}

	switch {
	case isInVisualRange:
		return style.Background(theme.SelectedBg)
	case isCurrentLine:
		return style.Background(theme.MutedBg)
	}

	return style
}

func (m *Model) isLineInVisualRange(lineIdx int) bool {
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

func highlightMatches(content string, matches []MatchRange) string {
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

func applyWordDiffHighlight(
	content string,
	lineType diff.LineType,
	wordDiff diff.WordDiffResult,
) string {
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

func (m *Model) renderHunkHeader(
	text string,
	width, hunkIdx int,
	isCurrent, isSelected bool,
) string {
	prefix := "  "
	if isCurrent {
		prefix = "> "
	}

	suffix := ""
	if isSelected {
		suffix = " [X]"
	}

	if m.getHunkTags != nil {
		tags := m.getHunkTags(hunkIdx)
		if len(tags) > 0 {
			tagStr := ""
			var tagStrSb590 strings.Builder
			for _, tag := range tags {
				tagStrSb590.WriteString(" [" + string(tag) + "]")
			}
			tagStr += tagStrSb590.String()
			suffix = tagStr + suffix
		}
	}

	displayText := prefix + text + suffix

	style := lipgloss.NewStyle().
		Foreground(theme.Accent)

	if isCurrent {
		style = style.Background(theme.MutedBg)
	}

	return style.Render(truncateOrPad(displayText, width))
}

func truncateOrPad(text string, width int) string {
	if width <= 0 {
		return ""
	}

	visibleLen := lipgloss.Width(text)
	if visibleLen > width {
		return text[:min(width, len(text))]
	}

	return text + strings.Repeat(" ", max(width-visibleLen, 0))
}

func padToSize(text string, width, height int) string {
	lines := []string{text}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", max(width, 0)))
	}

	return strings.Join(lines, "\n")
}

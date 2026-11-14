// Package diffview renders one file's hunks as a scrollable pane, either unified or side by side.
// It draws the hunk and line cursors, selection markers, split tags, search matches, and syntax
// or word-level highlighting, and owns no keybindings of its own.
package diffview

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/generated"

	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/highlight"
	"github.com/kyleking/jj-diff/internal/theme"
)

const halfDivisor = 2

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
	getHunkTags    func(hunkIdx int) []SplitTag
	isSelected     func(hunkIdx int) bool
	isLineSelected func(hunkIdx, lineIdx int) bool
	getMatches     func(hunkIdx, lineIdx int) []MatchRange
	lineIndex      *LineIndex
	wordDiffCache  *WordDiffCache
	highlighter    *highlight.Highlighter
	fileChange     *diff.FileChange
	mechanical     map[int]string
	expanded       map[int]bool
	generated      generated.Set
	viewMode       ViewModeType
	shown          [][]diff.Line
	pairs          [][]diff.SidePair
	visualAnchor   int
	lineCursor     int
	tabWidth       int
	selectedHunk   int
	offset         int
	isSearching    bool
	isVisualMode   bool
	generatedOpen  bool
	// showWhitespace hides whitespace-only changes, so the lines drawn are not the diff's own.
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
		generated:       generated.New(nil),
		viewMode:        viewMode,
		showWhitespace:  cfg.ShowWhitespace,
		showLineNumbers: cfg.ShowLineNumbers,
		tabWidth:        cfg.TabWidth,
		wordLevelDiff:   cfg.WordLevelDiff,
	}
}

// SetFileChange loads a file, scrolls back to the top, folds what is mechanical or generated, and
// rebuilds the lexing, word-diff, and row caches. The pane keeps its own copy of file, so later
// edits to the caller's value are not picked up.
func (m *Model) SetFileChange(file diff.FileChange) {
	m.fileChange = &file
	m.offset = 0
	m.highlighter = highlight.New(file)
	m.expanded = map[int]bool{}
	m.generatedOpen = false
	m.mechanical = map[int]string{}

	for key, reason := range diff.MechanicalHunks([]diff.FileChange{file}) {
		m.mechanical[key.Hunk] = reason
	}

	m.buildShown()
	m.buildPairs()
	m.buildLineIndex()
}

// buildShown keys the word diffs by the index of the lines each hunk draws, not of the diff's own.
func (m *Model) buildShown() {
	m.shown = make([][]diff.Line, len(m.fileChange.Hunks))

	for hunkIdx := range m.fileChange.Hunks {
		lines := m.fileChange.Hunks[hunkIdx].Lines
		if m.showWhitespace {
			lines = diff.ProcessHunkHideWhitespace(lines)
		}

		m.shown[hunkIdx] = lines
	}

	m.computeWordDiffs()
}

func (m *Model) buildPairs() {
	m.pairs = nil

	if m.viewMode != ViewModeSideBySide || m.fileChange == nil {
		return
	}

	m.pairs = make([][]diff.SidePair, len(m.shown))
	for hunkIdx, lines := range m.shown {
		m.pairs[hunkIdx] = diff.PairSides(lines)
	}
}

func (m *Model) buildLineIndex() {
	if m.fileChange == nil {
		m.lineIndex = nil
		return
	}

	index := &LineIndex{
		HunkOffsets:    make([]int, len(m.shown)),
		HunkLineCounts: make([]int, len(m.shown)),
	}

	if m.collapsed() {
		for hunkIdx := range m.shown {
			index.HunkLineCounts[hunkIdx] = 1
		}

		index.TotalLines = 1
		m.lineIndex = index

		return
	}

	totalLines := 0
	for hunkIdx := range m.shown {
		index.HunkOffsets[hunkIdx] = totalLines
		index.HunkLineCounts[hunkIdx] = 1 + m.bodyRows(hunkIdx)
		totalLines += index.HunkLineCounts[hunkIdx]
	}

	index.TotalLines = totalLines
	m.lineIndex = index
}

func (m *Model) bodyRows(hunkIdx int) int {
	switch {
	case m.isFolded(hunkIdx):
		return 1
	case m.viewMode == ViewModeSideBySide:
		return len(m.pairs[hunkIdx])
	default:
		return len(m.shown[hunkIdx])
	}
}

// foldReason is empty for a hunk with a real edit, or whose changes are already hidden.
func (m *Model) foldReason(hunkIdx int) string {
	if slices.ContainsFunc(m.shown[hunkIdx], func(line diff.Line) bool { return line.Type != diff.LineContext }) {
		return m.mechanical[hunkIdx]
	}

	return ""
}

func (m *Model) isFolded(hunkIdx int) bool {
	return m.foldReason(hunkIdx) != "" && !m.expanded[hunkIdx]
}

func (m *Model) collapsed() bool {
	return m.fileChange != nil && len(m.fileChange.Hunks) > 0 &&
		!m.generatedOpen && m.generated.Match(m.fileChange.Path)
}

// relayout applies change and keeps the hunk that was on top on top, since row counts shift.
func (m *Model) relayout(change func()) {
	var top HunkPosition
	if m.lineIndex != nil {
		top = m.lineIndex.FindHunkForOffset(m.offset)
	}

	change()
	m.buildLineIndex()

	if index := m.lineIndex; index != nil && top.HunkIdx < len(index.HunkOffsets) {
		m.offset = max(min(index.HunkOffsets[top.HunkIdx]+top.LineInHunk, index.TotalLines-1), 0)
	}
}

// ToggleFold expands a collapsed generated file, else folds or unfolds hunkIdx if it is mechanical,
// else collapses an expanded generated file.
func (m *Model) ToggleFold(hunkIdx int) {
	if m.fileChange == nil {
		return
	}

	switch {
	case m.collapsed():
		m.generatedOpen = true
		m.relayout(func() {})
	case m.foldReason(hunkIdx) != "":
		m.relayout(func() { m.expanded[hunkIdx] = !m.expanded[hunkIdx] })
	case m.generated.Match(m.fileChange.Path):
		m.generatedOpen = false
		m.relayout(func() {})
	}
}

// Reveal unfolds whatever hides hunkIdx, for a caller about to put a cursor inside it.
func (m *Model) Reveal(hunkIdx int) {
	if m.fileChange == nil || (!m.collapsed() && !m.isFolded(hunkIdx)) {
		return
	}

	m.generatedOpen = true
	m.relayout(func() { m.expanded[hunkIdx] = true })
}

func (m *Model) computeWordDiffs() {
	if m.fileChange == nil || !m.wordLevelDiff {
		m.wordDiffCache = nil
		return
	}

	m.wordDiffCache = &WordDiffCache{
		HunkDiffs: make(map[int]map[int]diff.WordDiffResult),
	}

	for hunkIdx, lines := range m.shown {
		m.wordDiffCache.HunkDiffs[hunkIdx] = diff.ComputeHunkWordDiffs(&diff.Hunk{Lines: lines})
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

	if m.fileChange == nil {
		return
	}

	m.relayout(func() {
		m.buildShown()
		m.buildPairs()
	})
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

	if m.fileChange == nil {
		return
	}

	m.relayout(m.buildPairs)
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

	totalLines := m.lineIndex.TotalLines
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

// View renders the pane at the given size, padding out to it when the content is shorter.
func (m *Model) View(width, height int, _ bool) string {
	if m.fileChange == nil {
		return padToSize("No file selected", width, height)
	}

	var rows []string

	switch {
	case m.collapsed():
		rows = []string{m.renderGeneratedSummary(width)}
	case m.viewMode == ViewModeSideBySide:
		rows = append([]string{renderPaneTitles(width)}, m.renderRows(width, height-1)...)
	default:
		rows = m.renderRows(width, height)
	}

	for len(rows) < height {
		rows = append(rows, strings.Repeat(" ", max(width, 0)))
	}

	return strings.Join(rows[:max(min(len(rows), height), 0)], "\n")
}

func (m *Model) renderRows(width, height int) []string {
	rows := make([]string, 0, max(height, 0))

	if m.lineIndex == nil {
		return rows
	}

	start := m.lineIndex.FindHunkForOffset(m.offset)
	skipped := start.LineInHunk

	for hunkIdx := start.HunkIdx; hunkIdx < len(m.shown) && len(rows) < height; hunkIdx++ {
		if skipped == 0 {
			isHunkSelected := m.isSelected != nil && m.isSelected(hunkIdx)
			rows = append(rows, m.renderHunkHeader(
				m.fileChange.Hunks[hunkIdx].Header, width, hunkIdx, hunkIdx == m.selectedHunk, isHunkSelected,
			))
		}

		rows = append(rows, m.renderBody(hunkIdx, max(skipped-1, 0), width, height-len(rows))...)
		skipped = 0
	}

	return rows
}

func (m *Model) renderBody(hunkIdx, first, width, limit int) []string {
	if limit <= 0 {
		return nil
	}

	if m.isFolded(hunkIdx) {
		if first > 0 {
			return nil
		}

		return []string{m.renderFoldNote(hunkIdx, width)}
	}

	count := m.bodyRows(hunkIdx)
	rows := make([]string, 0, min(limit, count))

	for at := first; at < count && len(rows) < limit; at++ {
		if m.viewMode == ViewModeSideBySide {
			rows = append(rows, m.renderPair(m.pairs[hunkIdx][at], hunkIdx, width))

			continue
		}

		rows = append(rows, m.renderLine(m.shown[hunkIdx][at], width, hunkIdx, at))
	}

	return rows
}

func (m *Model) renderFoldNote(hunkIdx, width int) string {
	changed := 0

	for _, line := range m.shown[hunkIdx] {
		if line.Type != diff.LineContext {
			changed++
		}
	}

	note := fmt.Sprintf("    ▸ %s: %d changed lines (z to expand)", m.foldReason(hunkIdx), changed)

	return foldNoteStyle().Render(truncateOrPad(note, width))
}

func (m *Model) renderGeneratedSummary(width int) string {
	selected := 0

	for hunkIdx := range m.fileChange.Hunks {
		if m.isSelected != nil && m.isSelected(hunkIdx) {
			selected++
		}
	}

	note := fmt.Sprintf(
		"  ▸ %s is generated: +%d -%d in %d hunk%s",
		m.fileChange.Path, m.fileChange.AddedLines(), m.fileChange.DeletedLines(),
		len(m.fileChange.Hunks), plural(len(m.fileChange.Hunks)),
	)
	if selected > 0 {
		note += fmt.Sprintf(", %d selected", selected)
	}

	return foldNoteStyle().Render(truncateOrPad(note+" (z to expand)", width))
}

func plural(n int) string {
	if n == 1 {
		return ""
	}

	return "s"
}

func foldNoteStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Muted).Italic(true)
}

func (m *Model) renderLine(line diff.Line, width, hunkIdx, lineIdx int) string {
	isCurrentLine := m.isVisualMode && m.lineCursor == lineIdx && hunkIdx == m.selectedHunk
	isInVisualRange := m.isVisualMode && hunkIdx == m.selectedHunk && m.isLineInVisualRange(lineIdx)
	isSelected := m.isLineSelected != nil && m.isLineSelected(hunkIdx, lineIdx)

	base := rowBase(line.Type, isInVisualRange || isCurrentLine)
	head := base.Render(lineIndicator(isInVisualRange, isSelected, isCurrentLine)) +
		base.Foreground(theme.Muted).Render(m.lineNumberGutter(line)) +
		markerStyle(line.Type, &base).Render(line.Type.String()+" ")

	content := fitStyled(m.lineContent(line, hunkIdx, lineIdx, &base), width-lipgloss.Width(head), &base)

	return padRow(head+content, width, &base)
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

// lineContent ranks search hits over word-level marks over syntax colors, since each styles the same bytes.
func (m *Model) lineContent(line diff.Line, hunkIdx, lineIdx int, base *lipgloss.Style) string {
	if m.isSearching && m.getMatches != nil {
		if matches := m.getMatches(hunkIdx, lineIdx); len(matches) > 0 {
			return highlightMatches(line.Content, matches, base)
		}
	}

	if wordDiff, ok := m.lookupWordDiff(line.Type, hunkIdx, lineIdx); ok {
		return applyWordDiffHighlight(line.Content, line.Type, wordDiff, base)
	}

	return m.highlighter.Line(m.fileChange.Path, line, base)
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

// rowBase is the style every segment of a row draws on, so a tint survives the resets between tokens.
func rowBase(kind diff.LineType, isCursor bool) lipgloss.Style {
	if isCursor {
		return theme.SelectionStyle
	}

	switch kind {
	case diff.LineAddition:
		return lipgloss.NewStyle().Background(theme.AddedBg)
	case diff.LineDeletion:
		return lipgloss.NewStyle().Background(theme.DeletedBg)
	case diff.LineContext:
	}

	return lipgloss.NewStyle()
}

func markerStyle(kind diff.LineType, base *lipgloss.Style) lipgloss.Style {
	switch kind {
	case diff.LineAddition:
		return base.Foreground(theme.AddedLine).Bold(true)
	case diff.LineDeletion:
		return base.Foreground(theme.DeletedLine).Bold(true)
	case diff.LineContext:
	}

	return *base
}

// fitStyled resets before the marker so the cut token's color does not leak into it.
func fitStyled(styled string, width int, base *lipgloss.Style) string {
	if width <= 0 {
		return ""
	}

	if lipgloss.Width(styled) <= width {
		return styled
	}

	return ansi.Truncate(styled, width-1, "") + "\x1b[m" + base.Faint(true).Render(truncationMarker)
}

const truncationMarker = "…"

func padRow(row string, width int, base *lipgloss.Style) string {
	if gap := width - lipgloss.Width(row); gap > 0 {
		return row + base.Render(strings.Repeat(" ", gap))
	}

	return row
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

func highlightMatches(content string, matches []MatchRange, base *lipgloss.Style) string {
	if len(matches) == 0 {
		return base.Render(content)
	}

	var segments []string
	lastEnd := 0

	for _, match := range matches {
		if lastEnd < match.Start && match.Start <= len(content) {
			segments = append(segments, base.Render(content[lastEnd:match.Start]))
		}

		if match.Start < len(content) {
			endIdx := min(match.End, len(content))
			segments = append(segments, lipgloss.NewStyle().
				Background(theme.Accent).
				Foreground(theme.ModalBg).
				Render(content[match.Start:endIdx]))
			lastEnd = endIdx
		}
	}

	if lastEnd < len(content) {
		segments = append(segments, base.Render(content[lastEnd:]))
	}

	return strings.Join(segments, "")
}

func applyWordDiffHighlight(
	content string,
	lineType diff.LineType,
	wordDiff diff.WordDiffResult,
	base *lipgloss.Style,
) string {
	var spans []diff.IntraLineSpan
	switch lineType {
	case diff.LineDeletion:
		spans = wordDiff.OldSpans
	case diff.LineAddition:
		spans = wordDiff.NewSpans
	case diff.LineContext:
	}

	if len(spans) == 0 {
		return base.Render(content)
	}

	var result strings.Builder
	for _, span := range spans {
		switch span.Type {
		case diff.SpanEqual:
			result.WriteString(base.Render(span.Text))
		case diff.SpanDeleted:
			result.WriteString(theme.WordDiffDeletedStyle.Render(span.Text))
		case diff.SpanAdded:
			result.WriteString(theme.WordDiffAddedStyle.Render(span.Text))
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
		style = theme.SelectedFocusedStyle
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

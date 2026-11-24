// Package diffview renders one file's hunks as a scrollable pane, either unified or side by side.
// It draws the hunk and line cursors, selection markers, split tags, search matches, and syntax
// or word-level highlighting, and owns no keybindings of its own.
package diffview

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/generated"
	"github.com/kyleking/aragonite/tui/glyph"
	"github.com/kyleking/aragonite/tui/overview"
	"github.com/kyleking/aragonite/tui/skin"

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
	getHunkTags func(hunkIdx int) []SplitTag
	isSelected  func(hunkIdx int) bool
	// isRead decides which hunks draw their header muted behind ReadGlyph, nil for none.
	isRead         func(hunkIdx int) bool
	isLineSelected func(hunkIdx, lineIdx int) bool
	getMatches     func(hunkIdx, lineIdx int) []MatchRange
	lineIndex      *LineIndex
	changeMarks    []overview.Mark
	hits           []SearchHit
	wordDiffCache  *WordDiffCache
	lexed          <-chan struct{}
	highlighter    *highlight.Highlighter
	cache          *drawCache
	fileChange     *diff.FileChange
	mechanical     map[int]string
	expanded       map[int]bool
	// hidden is the hunks drawn as their header alone, which is U's view of what has been read.
	hidden        []bool
	generated     generated.Set
	viewMode      ViewModeType
	shown         [][]diff.Line
	pairs         [][]diff.SidePair
	visualAnchor  int
	lineCursor    int
	tabWidth      int
	selectedHunk  int
	offset        int
	isSearching   bool
	isVisualMode  bool
	generatedOpen bool
	// showWhitespace hides whitespace-only changes, so the lines drawn are not the diff's own.
	showWhitespace  bool
	showLineNumbers bool
	wordLevelDiff   bool
	// announced is set once HighlightCmd has handed out the wait for the current file's colors.
	announced bool
}

// New builds a pane with no file loaded, so View renders a placeholder until SetFileChange is called.
// Everything the user can toggle at runtime takes its starting value from cfg.
func New(cfg config.Config) Model {
	viewMode := ViewModeUnified
	if cfg.ViewMode == config.ViewModeSideBySide {
		viewMode = ViewModeSideBySide
	}

	return Model{
		cache:           &drawCache{},
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
	m.lexed = m.highlighter.Start(0)
	m.announced = false

	if m.cache == nil {
		m.cache = &drawCache{}
	}

	m.invalidate()
	m.expanded = map[int]bool{}
	m.hidden = nil
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
		m.changeMarks = nil

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
	m.buildChangeMarks()
}

func (m *Model) bodyRows(hunkIdx int) int {
	switch {
	case m.isHidden(hunkIdx):
		return 0
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

// isHidden reports whether hunkIdx draws its header alone (SetHidden).
func (m *Model) isHidden(hunkIdx int) bool {
	return hunkIdx >= 0 && hunkIdx < len(m.hidden) && m.hidden[hunkIdx]
}

// SetHidden draws the hunks hidden marks as their header alone, one entry per hunk of the loaded
// file; nil draws every hunk whole. It is U's view: the hunks a reader has read shrink to one row
// each, so only what is unread takes room. Indices stay those of the file, so the cursor, the
// selection, and every motion still name the same hunk. Unchanged marks cost a comparison, so the
// caller may push them on every message.
func (m *Model) SetHidden(hidden []bool) {
	if m.fileChange == nil || slices.Equal(hidden, m.hidden) {
		return
	}

	m.relayout(func() { m.hidden = slices.Clone(hidden) })
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
	m.invalidate()
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

// SetFolds folds every mechanical hunk, or opens every one, and collapses or opens a generated file
// with them: zM and zR.
func (m *Model) SetFolds(folded bool) {
	if m.fileChange == nil {
		return
	}

	m.generatedOpen = !folded
	m.relayout(func() {
		for hunkIdx := range m.shown {
			if m.foldReason(hunkIdx) != "" {
				m.expanded[hunkIdx] = !folded
			}
		}
	})
}

// InvertFolds opens every folded hunk and folds every open one that can fold: zi.
func (m *Model) InvertFolds() {
	if m.fileChange == nil {
		return
	}

	m.relayout(func() {
		for hunkIdx := range m.shown {
			if m.foldReason(hunkIdx) != "" {
				m.expanded[hunkIdx] = !m.expanded[hunkIdx]
			}
		}
	})
}

// FramePosition is where Frame puts a row on screen.
type FramePosition int

// The three places zz, zt, and zb put the cursor's row.
const (
	FrameMiddle FramePosition = iota
	FrameTop
	FrameBottom
)

// Frame scrolls so the row of line lineIdx in hunkIdx (the header for a negative lineIdx) sits at
// pos in a pane viewHeight rows tall.
func (m *Model) Frame(hunkIdx, lineIdx int, pos FramePosition, viewHeight int) {
	row, ok := m.rowOf(hunkIdx, lineIdx)
	if !ok {
		return
	}

	switch pos {
	case FrameTop:
	case FrameMiddle:
		row -= viewHeight / halfDivisor
	case FrameBottom:
		row -= viewHeight - 1
	}

	m.offset = max(min(row, m.lineIndex.TotalLines-1), 0)
}

// Show scrolls the least it takes to bring hunkIdx's header into a pane viewHeight rows tall, which
// is what a jump to a hunk does before the reader looks for it.
func (m *Model) Show(hunkIdx, viewHeight int) {
	row, ok := m.rowOf(hunkIdx, -1)
	if !ok {
		return
	}

	switch {
	case row < m.offset:
		m.offset = row
	case row >= m.offset+viewHeight:
		m.offset = max(row-viewHeight/halfDivisor, 0)
	}
}

func (m *Model) rowOf(hunkIdx, lineIdx int) (int, bool) {
	if m.lineIndex == nil || hunkIdx < 0 || hunkIdx >= len(m.lineIndex.HunkOffsets) {
		return 0, false
	}

	row := m.lineIndex.HunkOffsets[hunkIdx]
	if lineIdx >= 0 && !m.isFolded(hunkIdx) && !m.isHidden(hunkIdx) {
		row += 1 + min(lineIdx, m.lineIndex.HunkLineCounts[hunkIdx]-2) //nolint:mnd // the header and the last row
	}

	return row, true
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

// SetReadState installs the lookup for which hunks have been read, whose headers then draw muted
// behind ReadGlyph. It is asked once per visible hunk per render, so it must be cheap; nil draws
// none read, which is the diff editor's case and a revision's before its marks load.
func (m *Model) SetReadState(isRead func(hunkIdx int) bool) {
	m.isRead = isRead
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
	m.invalidate()
}

// ToggleWordDiff flips intra-line highlighting and recomputes the word diffs for the current file,
// which costs a pass over every changed line pair.
func (m *Model) ToggleWordDiff() {
	m.wordLevelDiff = !m.wordLevelDiff
	m.computeWordDiffs()
	m.invalidate()
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

// margin is the columns every row keeps clear on the left, so content is never flush against the
// terminal's edge. The cursor's glyph lives there, which is why it is two cells and not one.
const margin = 2

// View renders the pane at the given size, padding out to it when the content is shorter. Content
// taller than the pane gets the overview track on its right edge, one column taken from every row.
func (m *Model) View(width, height int, _ bool) string {
	if m.fileChange == nil {
		return padToSize(strings.Repeat(" ", margin)+"No file selected", width, height)
	}

	if m.collapsed() {
		rows := []string{m.renderGeneratedSummary(width)}

		return strings.Join(fillRows(rows, width, height), "\n")
	}

	body := height
	if m.viewMode == ViewModeSideBySide {
		body = height - 1
	}

	bar := m.trackCells(body)
	inner := width - min(len(bar), 1)

	// One builder for the whole frame: a row and its track cell are written side by side rather
	// than joined, which was a copy of every row on every frame.
	var out strings.Builder

	out.Grow(height * (width*bytesPerCell + 1))

	drawn := 0
	line := func(row, cell string) {
		if drawn >= height {
			return
		}

		if drawn > 0 {
			out.WriteByte('\n')
		}

		out.WriteString(row)
		out.WriteString(cell)
		drawn++
	}

	if m.viewMode == ViewModeSideBySide {
		line(m.renderPaneTitles(inner), strings.Repeat(" ", min(len(bar), 1)))
	}

	for i, row := range fillRows(m.renderRows(inner, body), inner, max(body, 0)) {
		cell := ""
		if i < len(bar) {
			cell = bar[i]
		}

		line(row, cell)
	}

	for drawn < height {
		line(strings.Repeat(" ", max(width, 0)), "")
	}

	return out.String()
}

// fillRows pads rows with blank ones out to height, and cuts any past it.
func fillRows(rows []string, width, height int) []string {
	for len(rows) < height {
		rows = append(rows, strings.Repeat(" ", max(width, 0)))
	}

	return rows[:max(min(len(rows), height), 0)]
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
			rows = append(rows, m.renderHunkHeader(width, hunkIdx, hunkIdx == m.selectedHunk, isHunkSelected))
		}

		rows = append(rows, m.renderBody(hunkIdx, max(skipped-1, 0), width, height-len(rows))...)
		skipped = 0
	}

	return rows
}

func (m *Model) renderBody(hunkIdx, first, width, limit int) []string {
	if limit <= 0 || m.isHidden(hunkIdx) {
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
			rows = append(rows, m.renderPair(m.pairs[hunkIdx][at], hunkIdx, at, width))

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

	note := fmt.Sprintf("%s▸ %s: %d changed lines  z expands", m.codeIndent(), m.foldReason(hunkIdx), changed)

	return foldNoteStyle().Render(truncateOrPad(note, width))
}

// codeIndent lines a note up with the code column, past the margin and the number and sign gutter.
func (m *Model) codeIndent() string {
	indent := margin + signWidth
	if m.showLineNumbers {
		indent += numberWidth
	}

	return strings.Repeat(" ", indent)
}

func (m *Model) renderGeneratedSummary(width int) string {
	selected := 0

	for hunkIdx := range m.fileChange.Hunks {
		if m.isSelected != nil && m.isSelected(hunkIdx) {
			selected++
		}
	}

	note := fmt.Sprintf(
		"%s▸ %s is generated: +%d -%d in %d hunk%s",
		strings.Repeat(" ", margin),
		glyph.Settle(m.fileChange.Path), m.fileChange.AddedLines(), m.fileChange.DeletedLines(),
		len(m.fileChange.Hunks), plural(len(m.fileChange.Hunks)),
	)
	if selected > 0 {
		note += fmt.Sprintf(", %d selected", selected)
	}

	return foldNoteStyle().Render(truncateOrPad(note+"  z expands", width))
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
	matches := m.matchesFor(hunkIdx, lineIdx)

	key := rowKey{hunk: hunkIdx, at: lineIdx, width: width, state: rowState(isCurrentLine, isInVisualRange, isSelected)}
	if matches == nil {
		if row, ok := m.cachedRow(key); ok {
			return row
		}
	}

	p := m.pensFor()
	rp := &p.row[baseOf(line.Type, isInVisualRange || isCurrentLine)]

	var b strings.Builder

	b.Grow(width * bytesPerCell)
	b.WriteString(lineIndicator(p, isInVisualRange, isSelected, isCurrentLine))

	number := line.NewLineNum
	if line.Type == diff.LineDeletion {
		number = line.OldLineNum
	}

	headWidth := margin + m.writeGutter(&b, rp, line.Type, number)
	ready := m.writeFitted(&b, rp, p, line, hunkIdx, lineIdx, matches, width-headWidth)

	row := b.String()
	if matches == nil && ready {
		m.keepRow(key, row)
	}

	return row
}

func rowState(isCurrentLine, isInVisualRange, isSelected bool) uint8 {
	var state uint8

	if isCurrentLine {
		state |= stateCurrent
	}

	if isInVisualRange {
		state |= stateInRange
	}

	if isSelected {
		state |= stateSelected
	}

	return state
}

// matchesFor is the search hits on a line, or nil when no search is drawing any there.
func (m *Model) matchesFor(hunkIdx, lineIdx int) []MatchRange {
	if !m.isSearching || m.getMatches == nil {
		return nil
	}

	matches := m.getMatches(hunkIdx, lineIdx)
	if len(matches) == 0 {
		return nil
	}

	return matches
}

// writeFitted draws the line's content in room cells, cut with a marker when it is wider, then the
// row's wash out to the room's edge. It reports whether syntax colors were ready for it.
func (m *Model) writeFitted(
	b *strings.Builder, rp *rowPens, p *pens, line diff.Line, hunkIdx, lineIdx int, matches []MatchRange, room int,
) bool {
	if room <= 0 {
		return true
	}

	cells := plainWidth(line.Content)
	if cells >= 0 && cells <= room {
		ready := m.writeContent(b, rp, p, line, hunkIdx, lineIdx, matches)
		rp.faces.Base.Spaces(b, room-cells)

		return ready
	}

	var content strings.Builder

	ready := m.writeContent(&content, rp, p, line, hunkIdx, lineIdx, matches)
	styled := content.String()

	if cells < 0 {
		cells = lipgloss.Width(styled)
	}

	if cells > room {
		styled = ansi.Truncate(styled, room-1, "") + "\x1b[m" + rp.marker.Render(truncationMarker)
		cells = lipgloss.Width(styled)
	}

	b.WriteString(styled)
	rp.faces.Base.Spaces(b, room-cells)

	return ready
}

// The gutter's columns: a four-digit number and its space, then the sign and its space.
const (
	numberWidth = 5
	signWidth   = 2
)

// gutter is the line's number and its sign, drawn on the row's base so the wash runs from the number
// to the pane's edge. The sign carries the side, so the number stays quiet: a changed line's is only
// a step brighter than a context line's, which is all it needs to say "changed".
func (m *Model) writeGutter(b *strings.Builder, rp *rowPens, kind diff.LineType, number int) int {
	cells := 0

	if m.showLineNumbers {
		writeNumber(b, &rp.number[kindOf(kind)], number)
		cells += max(len(strconv.Itoa(number))+1, numberWidth)
	}

	rp.sign[kindOf(kind)].Write(b, sign(kind))
	rp.faces.Base.Write(b, " ")

	return cells + signWidth
}

// sign is the gutter glyph for a line: the sign bar on a changed line, or + and − where NO_COLOR
// leaves nothing else to say which side it is on.
func sign(kind diff.LineType) string {
	switch kind {
	case diff.LineAddition:
		return theme.Diff.Sign(true)
	case diff.LineDeletion:
		return theme.Diff.Sign(false)
	case diff.LineContext:
	}

	return " "
}

// writeContent ranks search hits over word-level marks over syntax colors, since each styles the
// same bytes. It reports whether syntax colors were ready, which they always are for the first two.
func (m *Model) writeContent(
	b *strings.Builder, rp *rowPens, p *pens, line diff.Line, hunkIdx, lineIdx int, matches []MatchRange,
) bool {
	if len(matches) > 0 {
		writeMatches(b, line.Content, matches, rp, p)

		return true
	}

	if wordDiff, ok := m.lookupWordDiff(line.Type, hunkIdx, lineIdx); ok {
		writeWordDiff(b, line.Content, line.Type, wordDiff, rp, p)

		return true
	}

	return m.highlighter.Peek(b, 0, line, rp.faces)
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

// lineIndicator is the margin's glyph for a line under the line cursor, inside a visual range, or
// picked for a split. It sits outside the wash, so it reads against the plain background.
func lineIndicator(p *pens, isInVisualRange, isSelected, isCurrentLine bool) string {
	switch {
	case isInVisualRange:
		return p.inRange
	case isSelected:
		return p.picked
	case isCurrentLine:
		return p.current
	default:
		return strings.Repeat(" ", margin)
	}
}

// rowBase is the style every segment of a row draws on, so a tint survives the resets between
// tokens. A changed line carries the shared wash, and the row under the cursor the selection.
func rowBase(kind diff.LineType, isCursor bool) lipgloss.Style {
	if isCursor {
		return theme.Skin.Selection
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

func signStyle(kind diff.LineType, base *lipgloss.Style) lipgloss.Style {
	switch kind {
	case diff.LineAddition:
		return skin.On(*base, theme.Diff.AddSign)
	case diff.LineDeletion:
		return skin.On(*base, theme.Diff.RemoveSign)
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

func writeMatches(b *strings.Builder, content string, matches []MatchRange, rp *rowPens, p *pens) {
	lastEnd := 0

	// Each byte is drawn once, so a hit at the line's end or overlapping the last one cannot
	// repeat text and push the row past its width.
	for _, match := range matches {
		start := max(match.Start, lastEnd)
		if lastEnd < start && start <= len(content) {
			rp.faces.Base.Write(b, content[lastEnd:start])
			lastEnd = start
		}

		if start < len(content) {
			endIdx := max(min(match.End, len(content)), start)
			p.match.Write(b, content[start:endIdx])
			lastEnd = endIdx
		}
	}

	if lastEnd < len(content) {
		rp.faces.Base.Write(b, content[lastEnd:])
	}
}

func writeWordDiff(
	b *strings.Builder, content string, lineType diff.LineType, wordDiff diff.WordDiffResult, rp *rowPens, p *pens,
) {
	var spans []diff.IntraLineSpan

	switch lineType {
	case diff.LineDeletion:
		spans = wordDiff.OldSpans
	case diff.LineAddition:
		spans = wordDiff.NewSpans
	case diff.LineContext:
	}

	if len(spans) == 0 {
		rp.faces.Base.Write(b, content)

		return
	}

	for _, span := range spans {
		switch span.Type {
		case diff.SpanEqual:
			rp.faces.Base.Write(b, span.Text)
		case diff.SpanDeleted:
			p.wordRemoved.Write(b, span.Text)
		case diff.SpanAdded:
			p.wordAdded.Write(b, span.Text)
		}
	}
}

// renderHunkHeader names a hunk by the code it sits in, the function or type git found above it,
// and the lines it spans, rather than by raw @@ syntax. The current hunk carries the cursor glyph in
// the margin and its name in the accent; tags and a selection mark follow the range.
func (m *Model) renderHunkHeader(width, hunkIdx int, isCurrent, isSelected bool) string {
	var tags []SplitTag
	if m.getHunkTags != nil {
		tags = m.getHunkTags(hunkIdx)
	}

	read := m.isRead != nil && m.isRead(hunkIdx)

	key := headerKey{
		hunk: hunkIdx, width: width, current: isCurrent, selected: isSelected, tags: string(tagRunes(tags)),
		read: read,
	}
	if row, ok := m.cache.headers[key]; ok {
		return row
	}

	row := m.drawHunkHeader(width, hunkIdx, tags, headerState{current: isCurrent, selected: isSelected, read: read})

	if m.cache.headers == nil || len(m.cache.headers) >= maxCachedRows {
		m.cache.headers = map[headerKey]string{}
	}

	m.cache.headers[key] = row

	return row
}

func tagRunes(tags []SplitTag) []rune {
	out := make([]rune, len(tags))
	for i, tag := range tags {
		out[i] = rune(tag)
	}

	return out
}

// headerState is what a hunk header shows besides the hunk: the cursor, the selection, and a read
// mark.
type headerState struct {
	current, selected, read bool
}

// ReadGlyph leads the header of a hunk that has been read. It is the cell jojo's read strip fills
// (▰ read, ▱ unread), and never a check: in -i a selected hunk ends its header in a green ✓, so a
// check for read put two checks on one row that differed only by color and side.
const ReadGlyph = "▰"

func (m *Model) drawHunkHeader(width, hunkIdx int, tags []SplitTag, st headerState) string {
	hunk := &m.fileChange.Hunks[hunkIdx]
	p := m.pensFor()

	lead, name := strings.Repeat(" ", margin), p.subtitle
	if st.current {
		lead, name = p.current, p.cursor
	}

	var b strings.Builder

	// A read hunk is drawn quieter than an unread one, its name muted behind the glyph, so the eye
	// skips it; the cursor still marks it in the margin.
	section := hunk.Section()
	if st.read {
		name = p.muted

		if section == "" {
			p.muted.Write(&b, ReadGlyph+" ")
		} else {
			section = ReadGlyph + " " + section
		}
	}

	p.muted.Write(&b, hunk.Range())

	for _, tag := range tags {
		b.WriteString(" ")
		p.accent.Write(&b, "["+string(tag)+"]")
	}

	if st.selected {
		b.WriteString(" ")
		p.success.Write(&b, "✓")
	}

	tail := b.String()

	if section == "" {
		return padRow(lead+fitStyled(tail, width-margin, &lipgloss.Style{}), width, &lipgloss.Style{})
	}

	room := width - margin - lipgloss.Width(tail) - len(headerGap)
	if room < 1 {
		return padRow(lead+fitStyled(tail, width-margin, &lipgloss.Style{}), width, &lipgloss.Style{})
	}

	if lipgloss.Width(section) > room {
		section = ansi.Truncate(section, room-1, "") + truncationMarker
	}

	return padRow(lead+name.Render(section)+headerGap+tail, width, &lipgloss.Style{})
}

const headerGap = "  "

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

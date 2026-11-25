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
	"github.com/kyleking/aragonite/tui/glyph"
	"github.com/kyleking/aragonite/tui/skin"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/fuzzy"
	"github.com/kyleking/jj-diff/internal/theme"
)

const (
	centerDivisor = 2
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
	getMatches func(fileIdx int) []MatchRange
	// drawn is the last frame View drew and what it drew it from. It is a pointer so a parent
	// drawing from a copy of the list still fills the one cache.
	drawn       *drawn
	filterQuery string
	files       []diff.FileChange
	counts      []changeCounts
	// picks is the selection drawn beside each file, off unless SetPicks turned it on.
	picks picks
	// reads is the read state drawn beside each file, off unless SetReads turned it on.
	reads        reads
	selected     int
	scrollOffset int
	isSearching  bool
	expanded     bool
	filterMode   bool
	// onlyUnread leaves files read whole out of the list, which is U's view.
	onlyUnread bool
}

// New returns a collapsed, unfiltered list with no files.
func New() Model {
	return Model{
		drawn:        &drawn{},
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

	if m.drawn != nil {
		*m.drawn = drawn{}
	}

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
// The list is only expanded while it has focus, so focus changes nothing a row draws.
func (m *Model) View(width, height int, _ bool) string {
	if m.drawn == nil {
		return m.draw(width, height)
	}

	key := m.drawKey(width, height)
	if m.drawn.ok && m.drawn.key == key {
		return m.drawn.view
	}

	view := m.draw(width, height)
	m.drawn.key, m.drawn.view, m.drawn.ok = key, view, true

	return view
}

// drawn holds the last frame and everything it was drawn from, since the list redraws on every key
// the diff pane takes and almost none of them change it.
type drawn struct {
	rows           map[rowKey]string
	view           string
	key            drawKey
	rowsGeneration uint64
	ok             bool
}

type drawKey struct {
	files       *diff.FileChange
	filterQuery string
	generation  uint64
	count       int
	selected    int
	scroll      int
	width       int
	height      int
	expanded    bool
	filterMode  bool
	onlyUnread  bool
	reads       uint64
	picks       uint64
}

func (m *Model) drawKey(width, height int) drawKey {
	key := drawKey{
		filterQuery: m.filterQuery, generation: theme.Generation(), count: len(m.files),
		selected: m.selected, scroll: m.scrollOffset, width: width, height: height,
		expanded: m.expanded, filterMode: m.filterMode, onlyUnread: m.onlyUnread, reads: m.reads.generation,
		picks: m.picks.generation,
	}

	if len(m.files) > 0 {
		key.files = &m.files[0]
	}

	return key
}

func (m *Model) draw(width, height int) string {
	if len(m.files) == 0 {
		empty := strings.Repeat(" ", margin) + theme.Skin.Muted.Render("No files")
		if m.expanded {
			return padToSize(empty, width, height)
		}

		return padToSize(empty, width, 1)
	}

	if m.expanded {
		return m.renderExpanded(width, height)
	}

	return m.renderCollapsed(width)
}

// renderCollapsed is the selected file as the diff pane's heading: on a band lifted off the frame,
// with its type's icon, its directory a step back and its name at full weight, and its counts and
// place in the list at the right edge.
func (m *Model) renderCollapsed(width int) string {
	band := theme.Skin.FileBand
	file := m.files[m.selected]

	right := m.reads.heading(m.selected, &band) + m.counts[m.selected].render(&band, 0, 0) +
		band.Render(headingGap) +
		skin.On(band, theme.Skin.Muted).Render(fmt.Sprintf("%d/%d", m.selected+1, len(m.files))) +
		band.Render(" ")

	lead := band.Render(strings.Repeat(" ", margin)) + statusCell(file.ChangeType, &band) + band.Render(" ")
	room := width - lipgloss.Width(lead) - lipgloss.Width(right) - len(headingGap)

	return fitRow(lead+pathCell(file.Path, max(room, 0), &band, true), right, width, &band)
}

func (m *Model) renderExpanded(width, height int) string {
	displayIndices := m.visibleIndices()
	layout := m.columns(width)

	rule := theme.Skin.Rule("Files", m.headerCount(len(displayIndices)), max(width-margin-rightPad, 0))
	lines := []string{fitRow(strings.Repeat(" ", margin)+rule, "", width, &lipgloss.Style{})}

	visibleHeight := height - headerRows
	if m.filterMode {
		visibleHeight -= filterRows
	}

	startIdx := m.scrollStart(displayIndices, visibleHeight, len(displayIndices))
	endIdx := min(startIdx+visibleHeight, len(displayIndices))

	for i := startIdx; i < endIdx; i++ {
		lines = append(lines, m.renderRow(displayIndices[i], layout))
	}

	targetHeight := height
	if m.filterMode {
		targetHeight -= filterRows
	}
	for len(lines) < targetHeight {
		lines = append(lines, strings.Repeat(" ", max(width, 0)))
	}

	if m.filterMode {
		filterLine := strings.Repeat(" ", margin) + theme.Skin.Muted.Render("filter ") +
			theme.Skin.Body.Render(m.filterQuery) + theme.Skin.Accent.Render("\u2588")
		lines = append(lines, "", fitRow(filterLine, "", width, &lipgloss.Style{}))
	}

	return strings.Join(lines, "\n")
}

// visibleIndices returns the rows to draw, in display order, as indices into the unfiltered file
// slice. Every index is in range, so callers may subscript m.files with one directly.
func (m *Model) visibleIndices() []int {
	indices := m.matchingIndices()
	if !m.onlyUnread || !m.reads.on {
		return indices
	}

	return slices.DeleteFunc(indices, m.reads.fileRead)
}

// matchingIndices is every file, or the ones the inline filter's query matches, best first.
func (m *Model) matchingIndices() []int {
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

// headerCount is what the Files rule counts: the cursor's place, or how many rows a filter kept.
func (m *Model) headerCount(filteredCount int) string {
	if m.filterMode && filteredCount < len(m.files) {
		return fmt.Sprintf("%d/%d filtered", filteredCount, len(m.files)) + m.reads.total()
	}

	if m.onlyUnread && filteredCount < len(m.files) {
		return fmt.Sprintf("%d of %d", filteredCount, len(m.files)) + m.reads.total()
	}

	return fmt.Sprintf("%d/%d", m.selected+1, len(m.files)) + m.reads.total()
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

// The columns a row keeps clear: the margin on the left, the cell after the stats on the right so
// the selection runs past the last digit, and the gap between the path and the stats.
const (
	margin     = 2
	rightPad   = 1
	headingGap = "  "
)

// columnLayout sizes the stat columns to the widest count in the list, so every row's numbers end in
// the same column at the pane's right edge whatever the path's length.
type columnLayout struct {
	width, addWidth, delWidth int
}

func (m *Model) columns(width int) columnLayout {
	layout := columnLayout{width: width}

	for i := range m.counts {
		if n := m.counts[i].additions; n > 0 {
			layout.addWidth = max(layout.addWidth, 1+digits(n))
		}

		if n := m.counts[i].deletions; n > 0 {
			layout.delWidth = max(layout.delWidth, 1+digits(n))
		}
	}

	return layout
}

// digits is how many characters strconv.Itoa writes for a positive n.
func digits(n int) int {
	count := 1

	for ; n >= base10; n /= base10 {
		count++
	}

	return count
}

const base10 = 10

// rowKey names one drawn row of the expanded list.
type rowKey struct {
	layout   columnLayout
	index    int
	selected bool
	reads    bool
	read     bool
	picks    bool
	pick     Pick
}

// renderRow draws one file's row, reusing the row drawn for the same file, layout, and selection,
// since moving the cursor changes two rows of the screen and redrawing all of them cost a lipgloss
// render per cell of every row.
func (m *Model) renderRow(originalIdx int, layout columnLayout) string {
	key := rowKey{
		layout: layout, index: originalIdx, selected: originalIdx == m.selected,
		reads: m.reads.on, read: m.reads.fileRead(originalIdx), picks: m.picks.on, pick: m.picks.at(originalIdx),
	}

	if m.drawn != nil {
		if row, ok := m.drawn.rows[key]; ok && m.drawn.rowsGeneration == theme.Generation() {
			return row
		}
	}

	row := m.drawRow(originalIdx, layout)

	if m.drawn != nil {
		if m.drawn.rows == nil || len(m.drawn.rows) >= maxKeptRows || m.drawn.rowsGeneration != theme.Generation() {
			m.drawn.rows, m.drawn.rowsGeneration = map[rowKey]string{}, theme.Generation()
		}

		m.drawn.rows[key] = row
	}

	return row
}

// maxKeptRows bounds the rows kept across frames, several screens of a long list.
const maxKeptRows = 1024

func (m *Model) drawRow(originalIdx int, layout columnLayout) string {
	file := m.files[originalIdx]
	selected := originalIdx == m.selected

	base := lipgloss.NewStyle()
	right := m.counts[originalIdx].render(&base, layout.addWidth, layout.delWidth) +
		strings.Repeat(" ", rightPad)
	lead := strings.Repeat(" ", margin) + statusCell(file.ChangeType, &base) + " " +
		m.picks.cell(originalIdx, &base) + m.reads.cell(originalIdx, &base)
	room := layout.width - lipgloss.Width(lead) - lipgloss.Width(right) - len(headingGap)
	row := fitRow(lead+pathCell(file.Path, max(room, 0), &base, selected), right, layout.width, &base)

	if !selected {
		return row
	}

	// The skin lays the selection under the drawn row: a tint the cells keep their colors on, or
	// at 16 colors and under NO_COLOR one even reverse bar with no colored cells inside it.
	return theme.Skin.Select(row, layout.width)
}

// fitRow puts right against the row's right edge and pads the gap between in base, so a selection
// or a band runs the full width. Where the two overlap, left is cut rather than right, since the
// counts are what a row is compared by.
func fitRow(left, right string, width int, base *lipgloss.Style) string {
	if width <= 0 {
		return ""
	}

	room := width - lipgloss.Width(right)
	if room < 0 {
		return ansi.Truncate(right, width, "")
	}

	if lipgloss.Width(left) > room {
		left = ansi.Truncate(left, room, "")
	}

	return left + base.Render(strings.Repeat(" ", max(0, room-lipgloss.Width(left)))) + right
}

// statusCell is the one-letter status jj prints, in the color of what happened to the file.
func statusCell(kind diff.ChangeType, base *lipgloss.Style) string {
	hue := theme.Palette.Yellow

	switch kind {
	case diff.ChangeTypeAdded:
		hue = theme.Palette.Green
	case diff.ChangeTypeDeleted:
		hue = theme.Palette.Red
	case diff.ChangeTypeRenamed:
		hue = theme.Palette.Blue
	case diff.ChangeTypeModified:
	}

	return skin.On(*base, lipgloss.NewStyle().Foreground(hue).Bold(true)).Render(kind.String())
}

// pathCell is a path in at most width cells: its type's icon, its directory a step back, and its own
// name at full weight, so the eye lands on the part that differs between two rows. A path too long
// for the cell gives up the front of its directory first, and its name only once that is gone.
func pathCell(path string, width int, base *lipgloss.Style, bright bool) string {
	if width <= 0 {
		return ""
	}

	icon := theme.Icons.File(path)
	// A file name is content: settled, so an emoji in it is drawn as wide as it is counted.
	dir, name := splitPath(glyph.Settle(path))
	prefix := icon.Prefix()

	if lipgloss.Width(prefix)+lipgloss.Width(name) > width {
		prefix = ""
	}

	room := width - lipgloss.Width(prefix)

	if lipgloss.Width(name) > room {
		dir, name = "", ansi.Truncate(name, max(room-1, 0), "")+ellipsis
	}

	if over := lipgloss.Width(dir) + lipgloss.Width(name) - room; over > 0 {
		dir = ellipsis + ansi.TruncateLeft(dir, over+lipgloss.Width(ellipsis), "")
	}

	nameFace := skin.On(*base, theme.Skin.Body)
	if bright {
		nameFace = skin.On(*base, theme.Skin.Title)
	}

	out := ""
	if prefix != "" {
		out = skin.On(*base, lipgloss.NewStyle().Foreground(icon.Color)).Render(prefix)
	}

	return out + skin.On(*base, theme.Skin.Muted).Render(dir) + nameFace.Render(name)
}

// splitPath cuts a path after its last slash, keeping the slash with the directory.
func splitPath(path string) (string, string) {
	at := strings.LastIndex(path, "/")

	return path[:at+1], path[at+1:]
}

type changeCounts struct {
	additions int
	deletions int
}

// render draws the counts right-aligned in columns addWidth and delWidth wide, or as wide as they
// are where a width is 0, leaving a zero count's cell blank.
func (c changeCounts) render(base *lipgloss.Style, addWidth, delWidth int) string {
	addFace := skin.On(*base, lipgloss.NewStyle().Foreground(theme.AddedLine))
	delFace := skin.On(*base, lipgloss.NewStyle().Foreground(theme.DeletedLine))
	added := statCell("+", c.additions, addWidth, &addFace)
	deleted := statCell("-", c.deletions, delWidth, &delFace)

	switch {
	case added == "":
		return deleted
	case deleted == "":
		return added
	}

	return added + base.Render(" ") + deleted
}

// statCell is a count right-aligned in width cells, blank for a zero count, or nothing at all where
// the column is empty in every row.
func statCell(sign string, n, width int, style *lipgloss.Style) string {
	text := ""
	if n > 0 {
		text = sign + strconv.Itoa(n)
	}

	width = max(width, len(text))
	if width == 0 {
		return ""
	}

	return style.Render(fmt.Sprintf("%*s", width, text))
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

// ellipsis marks where a path was cut.
const ellipsis = "…"

func padToSize(text string, width, height int) string {
	lines := []string{text}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", max(width, 0)))
	}

	return strings.Join(lines, "\n")
}

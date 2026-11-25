package model

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/empty"
	"github.com/kyleking/aragonite/tui/glyph"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/theme"
)

// Layout of the split columns: a title row, a gap, two header rows, a hairline, then the bodies.
// An empty column collapses to a strip of emptyColumnWidth, its number and "empty", so the width
// goes to the columns with something to show.
const (
	columnsChrome        = 5
	minColumnWidth       = 24
	emptyColumnWidth     = 11
	columnSeparator      = " │ "
	columnSeparatorWidth = 3
	tabSpaces            = "    "
)

// renderColumns draws the split columns over the whole screen, the footer under them.
func (m *Model) renderColumns() string {
	c := m.cols
	inner := max(m.width-2*paneMargin, 1)
	first, shown := m.columnWindow(inner)
	widths := m.columnWidths(first, shown, inner)
	bodyHeight := max(m.screenHeight()-columnsChrome-1, 1)

	title := theme.Skin.Title.Render("Split") + "  " + theme.Skin.Muted.Render(m.splitCommand())
	if shown < c.count() {
		title += theme.Skin.Muted.Render(fmt.Sprintf("  · columns %d–%d of %d", first+1, first+shown, c.count()))
	}

	cells := make([][]string, 0, shown)
	for i, width := range widths {
		col := first + i
		cells = append(cells, append(m.columnHeader(col, width), m.columnBody(col, width, bodyHeight)...))
	}

	rows := []string{fit(title, inner), ""}

	for i := range 2 + bodyHeight {
		if i == 2 { //nolint:mnd // the hairline under the two header rows
			rows = append(rows, theme.Skin.Hairline.Render(strings.Repeat("─", inner)))
		}

		parts := make([]string, 0, shown)
		for _, cell := range cells {
			parts = append(parts, cell[i])
		}

		rows = append(rows, strings.Join(parts, theme.Skin.Hairline.Render(columnSeparator)))
	}

	margin := strings.Repeat(" ", min(paneMargin, m.width))
	for i := range rows {
		rows[i] = fit(margin+rows[i], m.width)
	}

	return strings.Join(append(rows, m.renderStatusBar()), "\n")
}

// columnWindow is the first column drawn and how many fit at width, keeping the focused one in view.
// A collapsed column needs only its strip, so emptying a column can bring another into view.
func (m *Model) columnWindow(width int) (int, int) {
	c := m.cols

	for shown := c.count(); shown > 1; shown-- {
		first := min(max(c.focus-shown+1, 0), c.count()-shown)
		need := (shown - 1) * columnSeparatorWidth

		for col := first; col < first+shown; col++ {
			need += m.columnNeed(col)
		}

		if need <= width {
			return first, shown
		}
	}

	return c.focus, 1
}

// collapsed says whether col draws as a narrow strip: it owns nothing, and is not being described,
// since a description being typed needs the room to show.
func (m *Model) collapsed(col int) bool {
	return len(m.items(col)) == 0 && (col != m.cols.focus || !m.cols.describing)
}

func (m *Model) columnNeed(col int) int {
	if m.collapsed(col) {
		return emptyColumnWidth
	}

	return minColumnWidth
}

// columnWidths shares width among the shown columns: each collapsed one gets its strip and the rest
// split what is left, the last of them taking the cells an uneven split leaves over. When every
// shown column is collapsed they share the width evenly, since there is nothing to give it to.
func (m *Model) columnWidths(first, shown, width int) []int {
	widths := make([]int, shown)
	free := width - (shown-1)*columnSeparatorWidth
	open := 0

	for i := range widths {
		if m.collapsed(first + i) {
			widths[i] = emptyColumnWidth
			free -= emptyColumnWidth
		} else {
			open++
		}
	}

	if open == 0 {
		free, open = width-(shown-1)*columnSeparatorWidth, shown
		clear(widths)
	}

	last := -1

	for i := range widths {
		if widths[i] == 0 {
			widths[i] = max(free/open, 1)
			last = i
		}
	}

	if last >= 0 {
		widths[last] += max(free-max(free/open, 1)*open, 0)
	}

	return widths
}

// columnHeader is a column's two header rows: its number and description, then what it holds.
func (m *Model) columnHeader(col, width int) []string {
	c := m.cols
	focused := col == c.focus

	name := glyph.Settle(c.descs[col])

	switch {
	case col == 0:
		name = "stays in " + m.source
	case focused && c.describing:
		name = c.draft + "█"
	case name == "":
		name = "(no description)"
	}

	style := theme.Skin.Tab
	if focused {
		style = theme.Skin.TabActive
	}

	head := style.Render(fmt.Sprintf(" %d ", col+1)) + " " + name

	hunks, added, removed := m.columnCounts(col)

	counts := theme.Skin.Muted.Render("empty, dropped on apply")
	if m.collapsed(col) {
		counts = theme.Skin.Muted.Render("empty")
	}

	if hunks > 0 {
		counts = theme.Skin.Muted.Render(plural(hunks, "hunk")+" · ") +
			theme.AdditionStyle.Render(fmt.Sprintf("+%d", added)) + " " +
			theme.DeletionStyle.Render(fmt.Sprintf("−%d", removed))
	}

	return []string{fit(head, width), fit(counts, width)}
}

// columnBody is a column's rows: each hunk it owns a line of, under its file's name, with the
// cursor's hunk marked and scrolled into view.
func (m *Model) columnBody(col, width, height int) []string {
	c := m.cols
	focused := col == c.focus
	items := m.items(col)
	cursor := min(c.cursor[col], len(items)-1)

	var (
		rows        []string
		cursorStart int
		lastFile    = -1
	)

	for i, it := range items {
		file := m.changes[it.file]
		if it.file != lastFile {
			rows = append(rows, fit(theme.Skin.Heading.Render(glyph.Settle(file.Path)), width))
			lastFile = it.file
		}

		if i == cursor {
			cursorStart = len(rows)
		}

		rows = append(rows, m.hunkRows(col, it, width, focused && i == cursor)...)
	}

	if len(items) == 0 && !m.collapsed(col) {
		rows = emptyColumnRows(col, width)
	}

	// Once scrolled, the cursor's hunk sits a quarter of the way down.
	offset := 0
	if cursorStart > height/2 {
		offset = min(cursorStart-height/4, max(len(rows)-height, 0))
	}

	out := make([]string, 0, height)
	for i := range height {
		row := strings.Repeat(" ", width)
		if offset+i < len(rows) {
			row = rows[offset+i]
		}

		out = append(out, row)
	}

	return out
}

// hunkRows is one hunk as column col shows it: its header, its context, and the changed lines the
// column owns. The cursor's hunk carries a bar in the gutter, and selected lines a dot.
func (m *Model) hunkRows(col int, it item, width int, atCursor bool) []string {
	c := m.cols
	file := m.changes[it.file]
	hunk := file.Hunks[it.hunk]

	gutter := func(mark string) string {
		if atCursor && mark == " " {
			return theme.Skin.Accent.Render("▌")
		}

		return mark
	}

	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", hunk.OldStart, hunk.OldLines, hunk.NewStart, hunk.NewLines)
	rows := []string{fit(gutter(" ")+" "+theme.Skin.Muted.Render(header), width)}

	for i, l := range hunk.Lines {
		if !m.shown(col, it, i) {
			continue
		}

		mark := " "

		switch {
		case atCursor && c.visual && i == c.line:
			mark = theme.Skin.Accent.Render(">")
		case col == c.focus && l.Type != diff.LineContext && c.sel.IsLineSelected(file.Path, it.hunk, i):
			mark = theme.Skin.Accent.Render("●")
		}

		const gutterWidth = 2

		content := l.Type.String() + strings.ReplaceAll(l.Content, "\t", tabSpaces)
		text := ansi.Truncate(content, max(width-gutterWidth, 0), "…")

		style := theme.Skin.Muted
		switch l.Type {
		case diff.LineAddition:
			style = theme.AdditionStyle
		case diff.LineDeletion:
			style = theme.DeletionStyle
		case diff.LineContext:
		}

		rows = append(rows, fit(gutter(mark)+" "+style.Render(text), width))
	}

	return rows
}

// fit truncates or pads s to exactly width cells.
func fit(s string, width int) string {
	if w := lipgloss.Width(s); w < width {
		return s + strings.Repeat(" ", max(0, width-w))
	}

	return ansi.Truncate(s, width, "…")
}

// emptyColumnCard is the narrowest column the empty card draws in, below which one line says it.
const emptyColumnCard = 24

// emptyColumnRows is what an empty column shows: the card naming why it is empty and the keys that
// fill it, or one muted line where the column is too narrow for a card.
func emptyColumnRows(col, width int) []string {
	if width >= emptyColumnCard {
		block := empty.Block(theme.Skin, emptyColumn(col), width)
		for i := range block {
			block[i] = fit(block[i], width)
		}

		return append([]string{""}, block...)
	}

	hint := "empty: > or a digit sends hunks here"
	if col == 0 {
		hint = "empty: the source keeps no change"
	}

	return []string{fit(theme.Skin.Muted.Render(hint), width)}
}

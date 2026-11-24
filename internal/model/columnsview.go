package model

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/theme"
)

// Layout of the split columns: a title row, a gap, two header rows, a hairline, then the bodies.
const (
	columnsChrome   = 5
	minColumnWidth  = 24
	columnSeparator = " │ "
	tabSpaces       = "    "
)

// renderColumns draws the split columns over the whole screen, the footer under them.
func (m *Model) renderColumns() string {
	c := m.cols
	inner := max(m.width-2*paneMargin, 1)
	first, shown := m.columnWindow(inner)
	colWidth := max((inner-(shown-1)*len(columnSeparator))/shown, 1)
	bodyHeight := max(m.height-columnsChrome-1, 1)

	title := theme.Skin.Title.Render("Split") + "  " + theme.Skin.Muted.Render(m.splitCommand())
	if shown < c.count() {
		title += theme.Skin.Muted.Render(fmt.Sprintf("  · columns %d–%d of %d", first+1, first+shown, c.count()))
	}

	cells := make([][]string, 0, shown)
	for col := first; col < first+shown; col++ {
		cells = append(cells, append(m.columnHeader(col, colWidth), m.columnBody(col, colWidth, bodyHeight)...))
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
func (m *Model) columnWindow(width int) (int, int) {
	c := m.cols
	shown := min(max((width+len(columnSeparator))/(minColumnWidth+len(columnSeparator)), 1), c.count())
	first := min(max(c.focus-shown+1, 0), c.count()-shown)

	return first, shown
}

// columnHeader is a column's two header rows: its number and description, then what it holds.
func (m *Model) columnHeader(col, width int) []string {
	c := m.cols
	focused := col == c.focus

	name := c.descs[col]

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
			rows = append(rows, fit(theme.Skin.Heading.Render(file.Path), width))
			lastFile = it.file
		}

		if i == cursor {
			cursorStart = len(rows)
		}

		rows = append(rows, m.hunkRows(col, it, width, focused && i == cursor)...)
	}

	if len(items) == 0 {
		hint := "empty: > or a digit sends hunks here"
		if col == 0 {
			hint = "empty: the source keeps no change"
		}

		rows = []string{fit(theme.Skin.Muted.Render(hint), width)}
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
		return s + strings.Repeat(" ", width-w)
	}

	return ansi.Truncate(s, width, "…")
}

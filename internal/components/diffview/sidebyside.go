package diffview

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/theme"
)

const paneSeparator = " │ "

// paneWidths splits what is left of a row after the margin, giving the right column the odd cell so
// every row spans the pane.
func paneWidths(width int) (int, int) {
	room := width - margin - lipgloss.Width(paneSeparator)
	left := max(room/halfDivisor, 0)

	return left, max(room-left, 0)
}

func separator() string {
	return theme.Skin.Hairline.Render(paneSeparator)
}

// renderPaneTitles names the two columns quietly, lined up with the code under them.
func renderPaneTitles(width int) string {
	left, right := paneWidths(width)
	style := theme.Skin.Subtitle

	return strings.Repeat(" ", margin) +
		style.Render(truncateOrPad("Old", left)) + separator() + style.Render(truncateOrPad("New", right))
}

func (m *Model) renderPair(pair diff.SidePair, hunkIdx, width int) string {
	left, right := paneWidths(width)

	return strings.Repeat(" ", margin) +
		m.renderPane(pair.Left, hunkIdx, pair.LeftIdx, false, left) +
		separator() +
		m.renderPane(pair.Right, hunkIdx, pair.RightIdx, true, right)
}

// renderPane numbers the left pane by the old file and the right by the new. A nil line is the blank
// opposite an addition or a deletion with no counterpart.
func (m *Model) renderPane(line *diff.Line, hunkIdx, lineIdx int, isRight bool, width int) string {
	if line == nil {
		return strings.Repeat(" ", width)
	}

	base := rowBase(line.Type, false)

	number := line.OldLineNum
	if isRight {
		number = line.NewLineNum
	}

	head := m.gutter(line.Type, number, &base)
	content := fitStyled(m.lineContent(*line, hunkIdx, lineIdx, &base), width-lipgloss.Width(head), &base)

	return padRow(head+content, width, &base)
}

package diffview

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/theme"
)

const paneSeparator = " │ "

// paneWidths gives the right column the odd cell so every row spans the pane.
func paneWidths(width int) (int, int) {
	left := max((width-lipgloss.Width(paneSeparator))/halfDivisor, 0)

	return left, max(width-lipgloss.Width(paneSeparator)-left, 0)
}

func renderPaneTitles(width int) string {
	left, right := paneWidths(width)
	style := lipgloss.NewStyle().Foreground(theme.Secondary).Bold(true)

	return style.Render(truncateOrPad("OLD", left)) + paneSeparator + style.Render(truncateOrPad("NEW", right))
}

func (m *Model) renderPair(pair diff.SidePair, hunkIdx, width int) string {
	left, right := paneWidths(width)

	return m.renderPane(pair.Left, hunkIdx, pair.LeftIdx, false, left) +
		paneSeparator +
		m.renderPane(pair.Right, hunkIdx, pair.RightIdx, true, right)
}

// renderPane numbers the left pane by the old file and the right by the new. A nil line is the blank
// opposite an addition or a deletion with no counterpart.
func (m *Model) renderPane(line *diff.Line, hunkIdx, lineIdx int, isRight bool, width int) string {
	if line == nil {
		return strings.Repeat(" ", width)
	}

	base := rowBase(line.Type, false)
	head := markerStyle(line.Type, &base).Render(line.Type.String() + " ")

	if m.showLineNumbers {
		number := line.OldLineNum
		if isRight {
			number = line.NewLineNum
		}

		head = base.Foreground(theme.Muted).Render(fmt.Sprintf("%4d ", number)) + head
	}

	content := fitStyled(m.lineContent(*line, hunkIdx, lineIdx, &base), width-lipgloss.Width(head), &base)

	return padRow(head+content, width, &base)
}

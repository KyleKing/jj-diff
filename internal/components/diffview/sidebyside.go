package diffview

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/jj-diff/internal/diff"
)

const paneSeparator = " │ "

// paneWidths splits what is left of a row after the margin, giving the right column the odd cell so
// every row spans the pane.
func paneWidths(width int) (int, int) {
	room := width - margin - lipgloss.Width(paneSeparator)
	left := max(room/halfDivisor, 0)

	return left, max(room-left, 0)
}

// renderPaneTitles names the two columns quietly, lined up with the code under them.
func (m *Model) renderPaneTitles(width int) string {
	left, right := paneWidths(width)
	p := m.pensFor()

	return strings.Repeat(" ", margin) +
		p.subtitle.Render(truncateOrPad("Old", left)) + p.separator + p.subtitle.Render(truncateOrPad("New", right))
}

func (m *Model) renderPair(pair diff.SidePair, hunkIdx, at, width int) string {
	leftMatches := m.paneMatches(pair.Left, hunkIdx, pair.LeftIdx)
	rightMatches := m.paneMatches(pair.Right, hunkIdx, pair.RightIdx)
	cacheable := leftMatches == nil && rightMatches == nil

	key := rowKey{hunk: hunkIdx, at: at, width: width, state: statePair}
	if cacheable {
		if row, ok := m.cachedRow(key); ok {
			return row
		}
	}

	left, right := paneWidths(width)
	p := m.pensFor()

	var b strings.Builder

	b.Grow(width * bytesPerCell)
	b.WriteString(strings.Repeat(" ", margin))

	ready := m.writePane(&b, p, pair.Left, hunkIdx, pair.LeftIdx, false, left, leftMatches)

	b.WriteString(p.separator)

	ready = m.writePane(&b, p, pair.Right, hunkIdx, pair.RightIdx, true, right, rightMatches) && ready

	row := b.String()
	if cacheable && ready {
		m.keepRow(key, row)
	}

	return row
}

func (m *Model) paneMatches(line *diff.Line, hunkIdx, lineIdx int) []MatchRange {
	if line == nil {
		return nil
	}

	return m.matchesFor(hunkIdx, lineIdx)
}

// writePane numbers the left pane by the old file and the right by the new. A nil line is the blank
// opposite an addition or a deletion with no counterpart.
func (m *Model) writePane(
	b *strings.Builder, p *pens, line *diff.Line, hunkIdx, lineIdx int, isRight bool, width int, matches []MatchRange,
) bool {
	if line == nil {
		b.WriteString(strings.Repeat(" ", max(width, 0)))

		return true
	}

	rp := &p.row[baseOf(line.Type, false)]

	number := line.OldLineNum
	if isRight {
		number = line.NewLineNum
	}

	headWidth := m.writeGutter(b, rp, line.Type, number)

	return m.writeFitted(b, rp, p, *line, hunkIdx, lineIdx, matches, width-headWidth)
}

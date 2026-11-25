package filelist

import (
	"slices"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/skin"

	"github.com/kyleking/jj-diff/internal/theme"
)

// Pick is how much of one file the selection holds, which the list draws beside it in the modes that
// select: space on a row picks the whole file, and its hunks can be picked one at a time in the diff.
type Pick uint8

// The picks a row can draw.
const (
	PickNone Pick = iota
	// PickSome is a file some of whose hunks or lines are selected.
	PickSome
	// PickAll is a file every hunk of which is selected.
	PickAll
)

// The glyphs a pick draws: the check a selected hunk carries in the diff, and a tilde for part of a
// file. Both are one cell in every terminal.
const (
	pickAllGlyph  = "✓"
	pickSomeGlyph = "~"
)

// picks is the selection drawn beside each file, off unless SetPicks turned it on.
type picks struct {
	files []Pick
	// generation changes whenever files does, so a frame drawn from an older selection is not reused.
	generation uint64
	on         bool
}

// SetPicks draws the selection, one entry per file in list order; nil turns it off. It is cheap to
// call with an unchanged selection, which keeps the drawn frame.
func (m *Model) SetPicks(files []Pick) {
	on := files != nil
	if on == m.picks.on && slices.Equal(files, m.picks.files) {
		return
	}

	m.picks = picks{files: slices.Clone(files), generation: m.picks.generation + 1, on: on}
}

func (p *picks) at(i int) Pick {
	if !p.on || i < 0 || i >= len(p.files) {
		return PickNone
	}

	return p.files[i]
}

// pickCell is the pick column's width: the glyph and a space.
const pickCell = 2

// cell is the pick column of row i, and nothing at all while picks are off, so the browse list keeps
// its layout.
func (p *picks) cell(i int, base *lipgloss.Style) string {
	if !p.on {
		return ""
	}

	switch p.at(i) {
	case PickAll:
		return skin.On(*base, theme.Skin.Success).Render(pickAllGlyph) + base.Render(" ")
	case PickSome:
		return skin.On(*base, theme.Skin.Success).Render(pickSomeGlyph) + base.Render(" ")
	case PickNone:
	}

	return base.Render(blank(pickCell))
}

package diffview

import (
	"strings"

	"github.com/kyleking/aragonite/tui/overview"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/theme"
)

// SearchHit is a search match inside the loaded file, by hunk and by line index within that hunk,
// which is where the overview track places it.
type SearchHit struct {
	Hunk int
	Line int
}

// SetSearchHits replaces the matches the overview track marks. Pass nil once a search ends.
func (m *Model) SetSearchHits(hits []SearchHit) {
	m.hits = hits
}

// track is the overview column on the pane's right edge for a body height rows tall: where the frame
// sits in the file, each changed row in its side's color, and every search hit. Content that fits
// gets none, and keeps the column.
func (m *Model) track(height int) *overview.Bar {
	if m.lineIndex == nil {
		return nil
	}

	bar := overview.New(height, m.lineIndex.TotalLines, m.offset, theme.Skin.Hairline, theme.Skin.Subtitle)
	if bar == nil {
		return nil
	}

	return bar.Place(m.changeMarks...).Place(m.foundMarks()...)
}

// trackPad is the blank a row the track does not run beside (the column titles) puts in its column.
func trackPad(bar *overview.Bar) string {
	return strings.Repeat(" ", bar.Width())
}

// buildChangeMarks types every changed row once per layout, since they only move when the layout
// does. A pair carrying an addition leans added; a folded hunk shows no rows to mark.
func (m *Model) buildChangeMarks() {
	m.changeMarks = m.changeMarks[:0]

	if m.lineIndex == nil || m.collapsed() {
		return
	}

	added, removed := theme.Palette.Green, theme.Palette.Red

	for hunkIdx := range m.shown {
		if m.isFolded(hunkIdx) {
			continue
		}

		first := m.lineIndex.HunkOffsets[hunkIdx] + 1

		for at := range m.bodyRows(hunkIdx) {
			kind := m.rowKind(hunkIdx, at)

			switch kind {
			case diff.LineAddition:
				m.changeMarks = append(m.changeMarks, overview.Change(first+at, added))
			case diff.LineDeletion:
				m.changeMarks = append(m.changeMarks, overview.Change(first+at, removed))
			case diff.LineContext:
			}
		}
	}
}

// rowKind is the change a body row shows: an addition wherever the new side has one.
func (m *Model) rowKind(hunkIdx, at int) diff.LineType {
	if m.viewMode != ViewModeSideBySide {
		return m.shown[hunkIdx][at].Type
	}

	pair := m.pairs[hunkIdx][at]

	switch {
	case pair.Right != nil && pair.Right.Type == diff.LineAddition:
		return diff.LineAddition
	case pair.Left != nil && pair.Left.Type == diff.LineDeletion:
		return diff.LineDeletion
	}

	return diff.LineContext
}

// foundMarks places each search hit on the row that draws its line, or on its hunk's fold note.
func (m *Model) foundMarks() []overview.Mark {
	if len(m.hits) == 0 || m.lineIndex == nil || m.collapsed() {
		return nil
	}

	out := make([]overview.Mark, 0, len(m.hits))

	for _, hit := range m.hits {
		if hit.Hunk < 0 || hit.Hunk >= len(m.shown) {
			continue
		}

		at := 0
		if !m.isFolded(hit.Hunk) {
			at = min(m.bodyRowOf(hit.Hunk, hit.Line), m.bodyRows(hit.Hunk)-1)
		}

		out = append(out, overview.Found(m.lineIndex.HunkOffsets[hit.Hunk]+1+max(at, 0), theme.Accent))
	}

	return out
}

// bodyRowOf is the body row that draws line lineIdx of a hunk.
func (m *Model) bodyRowOf(hunkIdx, lineIdx int) int {
	if m.viewMode != ViewModeSideBySide {
		return lineIdx
	}

	for at, pair := range m.pairs[hunkIdx] {
		if pair.LeftIdx == lineIdx || pair.RightIdx == lineIdx {
			return at
		}
	}

	return 0
}

// Package destpicker lists candidate revisions so the user can choose where a squash lands. The
// parent model loads the revisions, routes keys here while the picker is visible, and reads the
// highlighted entry back out on confirmation. The rows keep jj log's graph order, a typed filter
// narrows them, and the footer line is the jj command the choice will run.
package destpicker

import (
	"strconv"
	"strings"

	"github.com/kyleking/aragonite/tui/keyhint"

	"github.com/kyleking/jj-diff/internal/components/modal"
	"github.com/kyleking/jj-diff/internal/fuzzy"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/theme"
)

const (
	halfDivisor = 2
	maxRows     = 16
	// The filter line and the command line share the box with the rows.
	extraRows      = 2
	preferredWidth = 72
	// The column a change id takes is the eight characters jj log shows.
	idWidth = 8
	// The log template writes this for a revision with no message.
	noDescription = "(no description)"
)

// Model is the destination picker. Mutators take a pointer receiver, so a parent holding it by value
// must keep the same field rather than a copy.
type Model struct {
	command   string
	query     string
	revisions []jj.RevisionEntry
	shown     []int
	hints     []keyhint.Hint
	selected  int
	visible   bool
	filtering bool
}

// New returns a hidden picker with no revisions, so SetRevisions must run before Show.
func New() Model {
	return Model{}
}

// SetRevisions replaces the candidate list, clearing the filter and starting on the row whose change
// ID start is (in full or as any prefix jj prints), or the first row when none is.
func (m *Model) SetRevisions(revisions []jj.RevisionEntry, start string) {
	m.revisions = revisions
	m.query = ""
	m.filtering = false
	m.refilter()
	m.selected = 0

	for i, idx := range m.shown {
		rev := m.revisions[idx]
		if start != "" && (strings.HasPrefix(start, rev.Short) || strings.HasPrefix(rev.Short, start)) {
			m.selected = i

			break
		}
	}
}

// SetCommand is the jj command line the footer shows, which the parent rebuilds as the cursor and
// the flags change.
func (m *Model) SetCommand(command string) { m.command = command }

// SetHints is the keys the box's footer offers, from the parent's keymap.
func (m *Model) SetHints(hints []keyhint.Hint) { m.hints = hints }

// Show reveals the picker. While it is visible the parent model routes every key here.
func (m *Model) Show() { m.visible = true }

// Hide takes the picker off screen and ends any filter being typed.
func (m *Model) Hide() {
	m.visible = false
	m.filtering = false
}

// IsVisible reports whether keys belong to the picker rather than the main view.
func (m *Model) IsVisible() bool { return m.visible }

// StartFilter makes typed keys narrow the rows until Enter or Esc.
func (m *Model) StartFilter() { m.filtering = true }

// IsFiltering reports whether typed keys go to the filter.
func (m *Model) IsFiltering() bool { return m.filtering }

// StopFilter hands keys back to the picker's bindings, keeping the rows the filter left.
func (m *Model) StopFilter() { m.filtering = false }

// Query is the filter text.
func (m *Model) Query() string { return m.query }

// SetQuery narrows the rows to those the query fuzzily matches, best first.
func (m *Model) SetQuery(query string) {
	m.query = query
	m.refilter()
	m.selected = 0
}

func (m *Model) refilter() {
	m.shown = m.shown[:0]

	if m.query == "" {
		for i := range m.revisions {
			m.shown = append(m.shown, i)
		}

		return
	}

	items := make([]string, len(m.revisions))
	data := make([]any, len(m.revisions))

	for i, r := range m.revisions {
		items[i] = r.Short + " " + r.Description
		data[i] = i
	}

	for _, match := range fuzzy.FilterWithData(m.query, items, data) {
		if i, ok := match.Original.(int); ok {
			m.shown = append(m.shown, i)
		}
	}
}

// Move steps the cursor by delta rows, stopping at both ends.
func (m *Model) Move(delta int) {
	m.selected = max(min(m.selected+delta, len(m.shown)-1), 0)
}

// Top and Bottom jump to the ends of the list.
func (m *Model) Top() { m.selected = 0 }

// Bottom jumps to the last row.
func (m *Model) Bottom() { m.selected = max(len(m.shown)-1, 0) }

// GetSelected returns the highlighted revision, or nil when no row is shown.
func (m *Model) GetSelected() *jj.RevisionEntry {
	if m.selected >= 0 && m.selected < len(m.shown) {
		return &m.revisions[m.shown[m.selected]]
	}

	return nil
}

// View draws the picker for a terminal of the given cell dimensions, scrolling a window of rows
// that keeps the cursor near the middle. It returns an empty string while hidden.
func (m *Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	inner := modal.Width(width, preferredWidth)
	visibleRows := min(modal.BodyRows(height)-extraRows, maxRows)
	startIdx := m.scrollStart(max(visibleRows, 1))
	endIdx := min(startIdx+max(visibleRows, 1), len(m.shown))

	body := make([]string, 0, visibleRows+extraRows)

	if m.filtering || m.query != "" {
		body = append(body, modal.Prompt(m.query))
	}

	for i := startIdx; i < endIdx; i++ {
		body = append(body, modal.Row(Line(&m.revisions[m.shown[i]]), inner, i == m.selected))
	}

	if len(m.shown) == 0 {
		body = append(body, theme.Skin.Muted.Render("No revisions match"))
	}

	if m.command != "" {
		body = append(body, "", theme.Skin.Muted.Render(m.command))
	}

	return modal.Box("Squash into", strconv.Itoa(len(m.shown)), body, m.hints, inner)
}

func (m *Model) scrollStart(visibleRows int) int {
	if len(m.shown) <= visibleRows {
		return 0
	}

	centered := max(m.selected-visibleRows/halfDivisor, 0)

	return min(centered, len(m.shown)-visibleRows)
}

// Line reads like a row of jj log: a mark for @ or an immutable revision, the change id with the
// prefix that names it uniquely in the accent and the rest a step back, then the description, muted
// when there is none. An immutable row is drawn muted whole, since it takes no squash into it.
func Line(rev *jj.RevisionEntry) string {
	mark := "  "

	switch {
	case rev.WorkingCopy:
		mark = theme.Skin.Accent.Render("@") + " "
	case rev.Immutable:
		mark = theme.Skin.Muted.Render("◆") + " "
	}

	rest := strings.TrimPrefix(rev.Short, rev.ChangeID)
	pad := strings.Repeat(" ", max(idWidth-len(rev.Short), 0))

	if rev.Immutable {
		return mark + theme.Skin.Muted.Render(rev.Short+pad+"  "+rev.Description)
	}

	id := theme.Skin.Cursor.Render(rev.ChangeID) + theme.Skin.Muted.Render(rest) + pad

	desc := theme.Skin.Body.Render(rev.Description)
	if rev.Description == noDescription {
		desc = theme.Skin.Muted.Render(rev.Description)
	}

	return mark + id + "  " + desc
}

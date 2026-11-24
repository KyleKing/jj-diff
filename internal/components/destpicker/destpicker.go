// Package destpicker lists candidate revisions so the user can choose where changes move.
// The parent model loads the revisions, routes keys here while the picker is visible, and
// reads the highlighted entry back out on confirmation.
package destpicker

import (
	"strconv"
	"strings"

	"github.com/kyleking/aragonite/tui/keyhint"

	"github.com/kyleking/jj-diff/internal/components/modal"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/theme"
)

const (
	halfDivisor    = 2
	maxRows        = 16
	preferredWidth = 64
	// The column a change id takes is the eight characters jj log shows.
	idWidth = 8
	// The log template writes this for a revision with no message.
	noDescription = "(no description)"
)

// Model is the destination picker. Mutators take a pointer receiver, so a parent holding it
// by value must keep the same field rather than a copy.
type Model struct {
	revisions []jj.RevisionEntry
	selected  int
	visible   bool
}

// New returns a hidden picker with no revisions, so SetRevisions must run before Show.
func New() Model {
	return Model{
		revisions: []jj.RevisionEntry{},
		selected:  0,
		visible:   false,
	}
}

// SetRevisions replaces the candidate list, keeping the cursor where it is when the new list
// is long enough and falling back to the first row otherwise. The slice is retained, not copied.
func (m *Model) SetRevisions(revisions []jj.RevisionEntry) {
	m.revisions = revisions
	if m.selected >= len(revisions) {
		m.selected = 0
	}
}

// Show reveals the picker. While it is visible the parent model routes every key here.
func (m *Model) Show() {
	m.visible = true
}

// Hide takes the picker off screen and leaves the revisions and cursor in place, so a later
// Show resumes on the same row.
func (m *Model) Hide() {
	m.visible = false
}

// IsVisible reports whether keys belong to the picker rather than the main view.
func (m *Model) IsVisible() bool {
	return m.visible
}

// MoveUp moves the cursor one revision earlier in the list and stops at the first, without wrapping.
func (m *Model) MoveUp() {
	if m.selected > 0 {
		m.selected--
	}
}

// MoveDown moves the cursor one revision later in the list and stops at the last, without wrapping.
func (m *Model) MoveDown() {
	if m.selected < len(m.revisions)-1 {
		m.selected++
	}
}

// GetSelected returns a pointer into the revision slice, or nil when the list is empty. The
// pointer is invalidated by the next SetRevisions, so read what is needed before calling it.
func (m Model) GetSelected() *jj.RevisionEntry {
	if m.selected >= 0 && m.selected < len(m.revisions) {
		return &m.revisions[m.selected]
	}

	return nil
}

// View draws the picker for a terminal of the given cell dimensions, scrolling a window of rows
// that keeps the cursor near the middle. It returns an empty string while hidden.
func (m Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	inner := modal.Width(width, preferredWidth)
	visibleRows := min(modal.BodyRows(height), maxRows)
	startIdx := m.scrollStart(visibleRows)
	endIdx := min(startIdx+visibleRows, len(m.revisions))

	body := make([]string, 0, visibleRows)
	for i := startIdx; i < endIdx; i++ {
		body = append(body, modal.Row(Line(&m.revisions[i]), inner, i == m.selected))
	}

	if len(body) == 0 {
		body = append(body, theme.Skin.Muted.Render("No revisions to move into"))
	}

	return modal.Box("Destination", strconv.Itoa(len(m.revisions)), body, []keyhint.Hint{
		{Key: "j/k", What: "move"}, {Key: "enter", What: "choose"}, {Key: "esc", What: "cancel"},
	}, inner)
}

func (m Model) scrollStart(visibleRows int) int {
	if len(m.revisions) <= visibleRows {
		return 0
	}

	centered := max(m.selected-visibleRows/halfDivisor, 0)

	return min(centered, len(m.revisions)-visibleRows)
}

// Line reads like a row of jj log: a mark for @, the change id with the prefix that names it
// uniquely in the accent and the rest a step back, then the description, muted when there is none.
func Line(rev *jj.RevisionEntry) string {
	mark := "  "
	if rev.WorkingCopy {
		mark = theme.Skin.Accent.Render("@") + " "
	}

	rest := strings.TrimPrefix(rev.Short, rev.ChangeID)
	id := theme.Skin.Cursor.Render(rev.ChangeID) + theme.Skin.Muted.Render(rest)
	id += strings.Repeat(" ", max(idWidth-len(rev.Short), 0))

	desc := theme.Skin.Body.Render(rev.Description)
	if rev.Description == noDescription {
		desc = theme.Skin.Muted.Render(rev.Description)
	}

	return mark + id + "  " + desc
}

// Package filefinder is a fuzzy picker modal over a list of items, bound to F.
package filefinder

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kyleking/aragonite/tui/keyhint"

	"github.com/kyleking/jj-diff/internal/components/modal"
	"github.com/kyleking/jj-diff/internal/fuzzy"
	"github.com/kyleking/jj-diff/internal/theme"
)

const (
	// The list is capped so the finder stays a compact box on a tall terminal.
	maxResults          = 12
	preferredModalWidth = 72
)

// Model is the picker's state. The item and data slices are parallel, so a caller passing lists of
// different lengths gets matches whose Original is missing.
type Model struct {
	query       string
	matches     []fuzzy.Match
	items       []string
	itemData    []any
	selectedIdx int
	visible     bool
}

// New returns a hidden picker with no items.
func New() Model {
	return Model{
		visible:     false,
		selectedIdx: 0,
	}
}

// Show opens the picker over items, pairing each with the value at the same index in data, which is
// what GetSelected returns. The query is cleared and every item matches until SetQuery narrows it.
func (m *Model) Show(items []string, data []any) {
	m.visible = true
	m.query = ""
	m.items = items
	m.itemData = data
	m.matches = fuzzy.FilterWithData("", items, data)
	m.selectedIdx = 0
}

// Hide closes the picker, keeping the items so a reopen without Show still has them.
func (m *Model) Hide() {
	m.visible = false
}

// IsVisible reports whether the picker is open, which is how the parent decides to route keys here.
func (m *Model) IsVisible() bool {
	return m.visible
}

// SetQuery re-runs the fuzzy match and resets the cursor to the top match. It ignores the query when
// Show was never called, so the cursor also stays put.
func (m *Model) SetQuery(query string) {
	m.query = query
	if len(m.items) > 0 && len(m.itemData) > 0 {
		m.matches = fuzzy.FilterWithData(query, m.items, m.itemData)
		m.selectedIdx = 0
	}
}

// Query returns the current search text.
func (m *Model) Query() string {
	return m.query
}

// SelectNext moves the cursor down one match, wrapping past the last.
func (m *Model) SelectNext() {
	if len(m.matches) > 0 {
		m.selectedIdx = (m.selectedIdx + 1) % len(m.matches)
	}
}

// SelectPrev moves the cursor up one match, wrapping past the first.
func (m *Model) SelectPrev() {
	if len(m.matches) > 0 {
		m.selectedIdx--
		if m.selectedIdx < 0 {
			m.selectedIdx = len(m.matches) - 1
		}
	}
}

// GetSelected returns the data value paired with the highlighted match, or nil when nothing matches.
func (m *Model) GetSelected() any {
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.matches) {
		return m.matches[m.selectedIdx].Original
	}

	return nil
}

// View renders the picker for a terminal width by height cells, returning the empty string while
// hidden. The list is a window that follows the cursor, so every match can be reached and seen.
func (m *Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	inner := modal.Width(width, preferredModalWidth)
	const promptRows = 2

	body := make([]string, 0, promptRows+maxResults)
	body = append(body, modal.Prompt(m.query), "")
	body = append(body, m.results(inner, min(modal.BodyRows(height)-promptRows, maxResults))...)

	return modal.Box("Find file", m.count(), body, []keyhint.Hint{
		{Key: "↑↓", What: "move"}, {Key: "enter", What: "open"}, {Key: "esc", What: "cancel"},
	}, inner)
}

// count is how many files match out of how many there are, or just the total before any filter.
func (m *Model) count() string {
	if m.query == "" {
		return strconv.Itoa(len(m.items))
	}

	return fmt.Sprintf("%d/%d", len(m.matches), len(m.items))
}

func (m *Model) results(width, rows int) []string {
	if len(m.matches) == 0 {
		return []string{theme.Skin.Muted.Render("No file matches")}
	}

	rows = max(rows, 1)
	start := min(max(m.selectedIdx-rows/halves, 0), max(len(m.matches)-rows, 0))
	end := min(start+rows, len(m.matches))

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		lines = append(lines, modal.Row(matchLine(&m.matches[i]), width, i == m.selectedIdx))
	}

	return lines
}

const halves = 2

// matchLine is a path as the file list draws one, its directory a step back from its name, with
// the characters the query matched in the accent so the reader sees why it is on the list.
func matchLine(match *fuzzy.Match) string {
	text := match.Text
	nameAt := strings.LastIndex(text, "/") + 1
	hit := make(map[int]bool, len(match.Indices))

	for _, i := range match.Indices {
		hit[i] = true
	}

	var out strings.Builder

	out.WriteString(theme.Icons.File(text).Prefix())

	for i, r := range text {
		face := theme.Skin.Muted
		if i >= nameAt {
			face = theme.Skin.Body
		}

		if hit[i] {
			face = theme.Skin.Cursor
		}

		out.WriteString(face.Render(string(r)))
	}

	return out.String()
}

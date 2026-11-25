// Package searchmodal shows the incremental search prompt and its match counter in the footer row. It holds no
// search logic: the parent model matches against the diff and pushes the query and the result
// counts in for display.
package searchmodal

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/vimtext"

	"github.com/kyleking/jj-diff/internal/theme"
)

// margin keeps the prompt off the terminal's edge, in line with the panes and the footer it replaces.
const margin = 2

// Model is the search prompt. It mirrors state the parent model owns, so the query and counts
// it displays are only as current as the last SetQuery and UpdateResults call.
type Model struct {
	query      string
	visible    bool
	matchCount int
	currentIdx int
}

// New returns a hidden prompt with an empty query.
func New() Model {
	return Model{
		visible: false,
	}
}

// Show opens the prompt with the query and counts cleared, so a reopen never displays the previous
// search.
func (m *Model) Show() {
	m.visible = true
	m.query = ""
	m.matchCount = 0
	m.currentIdx = -1
}

// Hide closes the prompt, leaving the query in place for the parent to read.
func (m *Model) Hide() {
	m.visible = false
}

// IsVisible reports whether the prompt is open, which is how the parent decides to route keys here.
func (m Model) IsVisible() bool {
	return m.visible
}

// SetQuery replaces the displayed query. It runs no search, so the counts stay as UpdateResults left
// them until the parent calls it again.
func (m *Model) SetQuery(query string) {
	m.query = query
}

// UpdateResults sets the match counter. The current index is 0-based and is displayed one higher, so
// pass -1 when no match is current.
func (m *Model) UpdateResults(matchCount, currentIdx int) {
	m.matchCount = matchCount
	m.currentIdx = currentIdx
}

// Line renders the prompt as the footer row, exactly width cells, returning the empty string while
// hidden. It takes the footer's place rather than sitting over the diff, so every match it
// highlights stays in view while the query is typed, as / does in less and vim. The field is the
// query being typed, drawn with its cursor and mode, and its keys join the footer's; a nil field
// draws the query alone.
func (m Model) Line(width int, field *vimtext.Field) string {
	if !m.visible || width <= 0 {
		return ""
	}

	lead := strings.Repeat(" ", min(margin, width))
	prompt := theme.Skin.Accent.Render("/") + theme.Skin.Body.Render(m.query) + theme.Skin.Accent.Render("▏")

	if field != nil {
		prompt = theme.Skin.Accent.Render("/") + field.View(inputWidth(m.query, width))
	}

	status := theme.Skin.Subtitle.Render(m.statusText())

	if m.query != "" && m.matchCount == 0 {
		status = theme.Skin.Warning.Render(m.statusText())
	}

	head := prompt + theme.HintGap + status
	if field != nil && field.Badge() != "" {
		head += theme.HintGap + field.Badge()
	}

	room := width - margin - lipgloss.Width(head) - len(theme.HintGap)

	// Listed in the order they are shed: the way out stays longest.
	steps := keyhint.Hint{Key: "ctrl-n/p", What: "next/prev", Priority: 1}
	hints := []keyhint.Hint{steps, {Key: "enter", What: "keep"}, {Key: "esc", What: "cancel", Pin: true}}

	for i := range hints {
		hints[i].Priority = i + 1
	}

	if field != nil {
		hints = append([]keyhint.Hint{steps}, field.Hints()...)
		for i := range hints {
			if hints[i].What == "save" {
				hints[i].What = "keep"
			}
		}
	}

	line := head
	if room > 0 {
		line += theme.HintGap + keyhint.Fit(theme.Hints(), hints, room)
	}

	return fit(lead+line, width)
}

// The query's box is as wide as what it holds plus room to type, at least minInput and at most a
// third of the row, past which it scrolls sideways.
const (
	minInput     = 16
	inputSlack   = 4
	inputDivisor = 3
)

func inputWidth(query string, width int) int {
	return max(min(lipgloss.Width(query)+inputSlack, width/inputDivisor), min(minInput, width/inputDivisor), 1)
}

func (m Model) statusText() string {
	switch {
	case m.matchCount > 0 && m.currentIdx >= 0:
		return fmt.Sprintf("%d of %d", m.currentIdx+1, m.matchCount)
	case m.matchCount > 0:
		return fmt.Sprintf("%d matches", m.matchCount)
	case m.query == "":
		return "type to search"
	}

	return "no matches"
}

// fit cuts a styled line to width cells or pads it out, so the footer never wraps to a second row.
func fit(line string, width int) string {
	if lipgloss.Width(line) > width {
		line = ansi.Truncate(line, width, "")
	}

	return line + strings.Repeat(" ", max(width-lipgloss.Width(line), 0))
}

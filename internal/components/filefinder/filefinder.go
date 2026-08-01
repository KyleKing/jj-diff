// Package filefinder is a fuzzy picker modal over a list of items, bound to F.
package filefinder

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kyleking/jj-diff/internal/fuzzy"
	"github.com/kyleking/jj-diff/internal/theme"
)

const (
	ellipsis            = "..."
	matchTextMargin     = 2
	maxResults          = 10
	modalWidthMargin    = 4
	preferredModalWidth = 80
	viewChromeLines     = 8
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

// View renders the picker centered in the given terminal size, returning the empty string while
// hidden.
func (m *Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	modalWidth := min(preferredModalWidth, width-modalWidthMargin)

	title := "Find File"
	inputLine := fmt.Sprintf("Filter: %s█", m.query)
	footer := "↑↓: navigate | Enter: select | Esc: cancel"

	lines := make([]string, 0, viewChromeLines+maxResults)
	lines = append(lines, styleTitle(title, modalWidth), "", styleInput(inputLine, modalWidth), "")
	lines = append(lines, m.renderResults(modalWidth)...)
	lines = append(lines, "", styleFooter(footer, modalWidth))

	content := strings.Join(lines, "\n")

	return renderModal(content, width, height)
}

func (m *Model) renderResults(width int) []string {
	if len(m.matches) == 0 {
		if m.query == "" {
			return []string{styleHint("Type to filter files...", width)}
		}

		return []string{styleHint("No matches", width)}
	}

	lines := make([]string, 0, min(len(m.matches), maxResults))
	for i := range min(len(m.matches), maxResults) {
		lines = append(lines, renderMatch(m.matches[i], i == m.selectedIdx, width))
	}

	if len(m.matches) > maxResults {
		remaining := len(m.matches) - maxResults
		lines = append(lines, "", styleHint(fmt.Sprintf("... and %d more", remaining), width))
	}

	return lines
}

func renderMatch(match fuzzy.Match, isSelected bool, width int) string {
	prefix := "  "
	if isSelected {
		prefix = "> "
	}

	text := match.Text
	displayText := prefix + text

	maxWidth := width - matchTextMargin
	if len(displayText) > maxWidth {
		displayText = displayText[:maxWidth-len(ellipsis)] + ellipsis
	}

	style := lipgloss.NewStyle().
		Width(width).
		Foreground(theme.Text)

	if isSelected {
		style = style.
			Background(theme.SelectedBg).
			Bold(true)
	}

	if match.Matched && len(match.Indices) > 0 && !isSelected {
		displayText = highlightMatches(displayText, match.Indices, len(prefix))
	}

	return style.Render(displayText)
}

func highlightMatches(text string, indices []int, prefixLen int) string {
	matchedPositions := make(map[int]bool, len(indices))
	for _, idx := range indices {
		matchedPositions[idx+prefixLen] = true
	}

	var result strings.Builder
	for i, ch := range text {
		if matchedPositions[i] {
			highlighted := lipgloss.NewStyle().
				Foreground(theme.Accent).
				Bold(true).
				Render(string(ch))
			result.WriteString(highlighted)
		} else {
			result.WriteRune(ch)
		}
	}

	return result.String()
}

func styleTitle(text string, width int) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary).
		Width(width).
		Align(lipgloss.Center)

	return style.Render(text)
}

func styleInput(text string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(theme.Text).
		Width(width)

	return style.Render(text)
}

func styleHint(text string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(theme.SoftMutedBg).
		Width(width).
		Align(lipgloss.Center)

	return style.Render(text)
}

func styleFooter(text string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(theme.SoftMutedBg).
		Width(width).
		Align(lipgloss.Center)

	return style.Render(text)
}

func renderModal(content string, width, height int) string {
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Padding(0, 1)

	modal := modalStyle.Render(content)

	overlay := lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(theme.ModalBg),
	)

	return overlay
}

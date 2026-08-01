// Package searchmodal shows the incremental search prompt and its match counter. It holds no
// search logic: the parent model matches against the diff and pushes the query and the result
// counts in for display.
package searchmodal

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/jj-diff/internal/theme"
)

const (
	modalWidthMargin    = 4
	preferredModalWidth = 60
	viewLineCount       = 7
)

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

// View renders the prompt centered in the given terminal size, returning the empty string while
// hidden.
func (m Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	modalWidth := min(preferredModalWidth, width-modalWidthMargin)

	title := "Search"
	inputLine := fmt.Sprintf("Query: %s█", m.query)
	footer := "Enter: close search | Esc: cancel | Ctrl-N/P: next/prev"

	lines := make([]string, 0, viewLineCount)
	lines = append(lines,
		styleTitle(title, modalWidth),
		"",
		styleInput(inputLine, modalWidth),
		"",
		styleStatus(m.statusText(), modalWidth),
		"",
		styleFooter(footer, modalWidth),
	)

	content := strings.Join(lines, "\n")

	return renderModal(content, width, height)
}

func (m Model) statusText() string {
	if m.matchCount > 0 {
		return fmt.Sprintf("Match %d of %d", m.currentIdx+1, m.matchCount)
	}

	if m.query == "" {
		return "Type to search..."
	}

	return "No matches"
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

func styleStatus(text string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(theme.Accent).
		Bold(true).
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
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(theme.ModalBg)),
	)

	return overlay
}

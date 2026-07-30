// Package searchmodal shows the incremental search prompt and its match counter. It holds no
// search logic: the parent model matches against the diff and pushes the query and the result
// counts in for display.
package searchmodal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kyleking/jj-diff/internal/theme"
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

	modalWidth := 60
	if modalWidth > width-4 {
		modalWidth = width - 4
	}

	var lines []string

	// Title
	title := "Search"
	lines = append(lines, styleTitle(title, modalWidth))
	lines = append(lines, "")

	// Search input
	inputLine := fmt.Sprintf("Query: %s█", m.query)
	lines = append(lines, styleInput(inputLine, modalWidth))
	lines = append(lines, "")

	// Match count
	var status string
	if m.matchCount == 0 {
		if m.query == "" {
			status = "Type to search..."
		} else {
			status = "No matches"
		}
	} else {
		status = fmt.Sprintf("Match %d of %d", m.currentIdx+1, m.matchCount)
	}
	lines = append(lines, styleStatus(status, modalWidth))
	lines = append(lines, "")

	// Footer
	footer := "Enter: close search | Esc: cancel | Ctrl-N/P: next/prev"
	lines = append(lines, styleFooter(footer, modalWidth))

	content := strings.Join(lines, "\n")

	return renderModal(content, width, height)
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
		lipgloss.WithWhitespaceForeground(theme.ModalBg),
	)

	return overlay
}

package filefinder

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/jj-diff/internal/fuzzy"
	"github.com/kyleking/jj-diff/internal/theme"
)

type Model struct {
	query       string
	visible     bool
	matches     []fuzzy.Match
	selectedIdx int
	items       []string
	itemData    []interface{}
}

func New() Model {
	return Model{
		visible:     false,
		selectedIdx: 0,
	}
}

func (m *Model) Show(items []string, data []interface{}) {
	m.visible = true
	m.query = ""
	m.items = items
	m.itemData = data
	m.matches = fuzzy.FilterWithData("", items, data)
	m.selectedIdx = 0
}

func (m *Model) Hide() {
	m.visible = false
}

func (m Model) IsVisible() bool {
	return m.visible
}

func (m *Model) SetQuery(query string) {
	m.query = query
	if len(m.items) > 0 && len(m.itemData) > 0 {
		m.matches = fuzzy.FilterWithData(query, m.items, m.itemData)
		m.selectedIdx = 0
	}
}

func (m Model) Query() string {
	return m.query
}

func (m *Model) SelectNext() {
	if len(m.matches) > 0 {
		m.selectedIdx = (m.selectedIdx + 1) % len(m.matches)
	}
}

func (m *Model) SelectPrev() {
	if len(m.matches) > 0 {
		m.selectedIdx--
		if m.selectedIdx < 0 {
			m.selectedIdx = len(m.matches) - 1
		}
	}
}

func (m Model) GetSelected() interface{} {
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.matches) {
		return m.matches[m.selectedIdx].Original
	}
	return nil
}

func (m Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	modalWidth := 80
	if modalWidth > width-4 {
		modalWidth = width - 4
	}

	var lines []string

	// Title
	title := "Find File"
	lines = append(lines, styleTitle(title, modalWidth))
	lines = append(lines, "")

	// Search input
	inputLine := fmt.Sprintf("Filter: %s█", m.query)
	lines = append(lines, styleInput(inputLine, modalWidth))
	lines = append(lines, "")

	// Results
	maxResults := 10
	resultCount := len(m.matches)
	if resultCount > maxResults {
		resultCount = maxResults
	}

	if len(m.matches) == 0 {
		if m.query == "" {
			lines = append(lines, styleHint("Type to filter files...", modalWidth))
		} else {
			lines = append(lines, styleHint("No matches", modalWidth))
		}
	} else {
		for i := 0; i < resultCount; i++ {
			match := m.matches[i]
			isSelected := i == m.selectedIdx
			lines = append(lines, m.renderMatch(match, isSelected, modalWidth))
		}

		if len(m.matches) > maxResults {
			remaining := len(m.matches) - maxResults
			lines = append(lines, "")
			lines = append(lines, styleHint(fmt.Sprintf("... and %d more", remaining), modalWidth))
		}
	}

	lines = append(lines, "")

	// Footer
	footer := "↑↓: navigate | Enter: select | Esc: cancel"
	lines = append(lines, styleFooter(footer, modalWidth))

	content := strings.Join(lines, "\n")

	return renderModal(content, width, height)
}

func (m Model) renderMatch(match fuzzy.Match, isSelected bool, width int) string {
	prefix := "  "
	if isSelected {
		prefix = "> "
	}

	text := match.Text
	displayText := prefix + text

	// Truncate if too long
	maxWidth := width - 2
	if len(displayText) > maxWidth {
		displayText = displayText[:maxWidth-3] + "..."
	}

	style := lipgloss.NewStyle().
		Width(width).
		Foreground(theme.Text)

	if isSelected {
		style = style.
			Background(theme.SelectedBg).
			Bold(true)
	}

	// Highlight matched characters
	if match.Matched && len(match.Indices) > 0 && !isSelected {
		displayText = m.highlightMatches(displayText, match.Indices, len(prefix))
	}

	return style.Render(displayText)
}

func (m Model) highlightMatches(text string, indices []int, prefixLen int) string {
	// Convert indices to a map for quick lookup (adjust for prefix)
	matchedPositions := make(map[int]bool)
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

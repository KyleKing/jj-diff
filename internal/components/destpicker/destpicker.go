package destpicker

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/jj-diff/internal/jj"
)

type Model struct {
	revisions []jj.RevisionEntry
	selected  int
	visible   bool
}

func New() Model {
	return Model{
		revisions: []jj.RevisionEntry{},
		selected:  0,
		visible:   false,
	}
}

func (m *Model) SetRevisions(revisions []jj.RevisionEntry) {
	m.revisions = revisions
	if m.selected >= len(revisions) {
		m.selected = 0
	}
}

func (m *Model) Show() {
	m.visible = true
}

func (m *Model) Hide() {
	m.visible = false
}

func (m *Model) IsVisible() bool {
	return m.visible
}

func (m *Model) MoveUp() {
	if m.selected > 0 {
		m.selected--
	}
}

func (m *Model) MoveDown() {
	if m.selected < len(m.revisions)-1 {
		m.selected++
	}
}

func (m Model) GetSelected() *jj.RevisionEntry {
	if m.selected >= 0 && m.selected < len(m.revisions) {
		return &m.revisions[m.selected]
	}
	return nil
}

func (m Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	maxHeight := height - 4
	if maxHeight < 5 {
		maxHeight = 5
	}
	if maxHeight > 20 {
		maxHeight = 20
	}

	modalWidth := width - 20
	if modalWidth < 40 {
		modalWidth = 40
	}
	if modalWidth > 80 {
		modalWidth = 80
	}

	var lines []string
	lines = append(lines, styleHeader("Select Destination", modalWidth))
	lines = append(lines, "")

	startIdx := 0
	endIdx := len(m.revisions)

	if len(m.revisions) > maxHeight-3 {
		startIdx = m.selected - (maxHeight-3)/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + maxHeight - 3
		if endIdx > len(m.revisions) {
			endIdx = len(m.revisions)
			startIdx = endIdx - (maxHeight - 3)
			if startIdx < 0 {
				startIdx = 0
			}
		}
	}

	for i := startIdx; i < endIdx; i++ {
		rev := m.revisions[i]
		isSelected := i == m.selected
		line := m.renderRevisionLine(rev, isSelected, modalWidth-2)
		lines = append(lines, "  "+line)
	}

	lines = append(lines, "")
	lines = append(lines, styleFooter("Enter: Select | Esc: Cancel | j/k: Navigate", modalWidth))

	content := strings.Join(lines, "\n")

	return renderModal(content, width, height)
}

func (m Model) renderRevisionLine(rev jj.RevisionEntry, selected bool, width int) string {
	desc := rev.Description
	if len(desc) > width-15 {
		desc = desc[:width-18] + "..."
	}

	line := fmt.Sprintf("%-10s %s", rev.ChangeID, desc)

	if selected {
		return styleSelected(truncateOrPad(line, width))
	}

	return truncateOrPad(line, width)
}

func styleHeader(text string, width int) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Width(width).
		Align(lipgloss.Center)
	return style.Render(text)
}

func styleFooter(text string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(width).
		Align(lipgloss.Center)
	return style.Render(text)
}

func styleSelected(text string) string {
	return lipgloss.NewStyle().
		Background(lipgloss.Color("25")).
		Foreground(lipgloss.Color("15")).
		Render(text)
}

func truncateOrPad(text string, width int) string {
	if len(text) > width {
		return text[:width-3] + "..."
	}
	return text + strings.Repeat(" ", width-len(text))
}

func renderModal(content string, termWidth, termHeight int) string {
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("12")).
		Padding(1, 2)

	modal := borderStyle.Render(content)

	overlay := lipgloss.Place(
		termWidth,
		termHeight,
		lipgloss.Center,
		lipgloss.Center,
		modal,
	)

	return overlay
}

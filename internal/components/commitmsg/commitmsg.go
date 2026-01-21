package commitmsg

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/jj-diff/internal/theme"
)

type SplitTag rune

type Model struct {
	tag     SplitTag
	message string
	visible bool
}

func New() Model {
	return Model{
		tag:     'A',
		message: "",
		visible: false,
	}
}

func (m *Model) SetTag(tag SplitTag) {
	m.tag = tag
	m.message = ""
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

func (m *Model) AppendChar(ch rune) {
	m.message += string(ch)
}

func (m *Model) Backspace() {
	if len(m.message) > 0 {
		m.message = m.message[:len(m.message)-1]
	}
}

func (m Model) GetMessage() string {
	return m.message
}

func (m Model) GetTag() SplitTag {
	return m.tag
}

func (m Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	modalWidth := width - 20
	if modalWidth < 50 {
		modalWidth = 50
	}
	if modalWidth > 80 {
		modalWidth = 80
	}

	var lines []string
	lines = append(lines, styleHeader(fmt.Sprintf("Commit Message for Tag [%s]", string(m.tag)), modalWidth))
	lines = append(lines, "")

	inputBox := styleInput(m.message, modalWidth-4)
	lines = append(lines, "  "+inputBox)

	lines = append(lines, "")
	lines = append(lines, styleFooter("Enter: Confirm | Esc: Cancel | Type to edit", modalWidth))

	content := strings.Join(lines, "\n")
	return renderModal(content, width, height)
}

func styleHeader(text string, width int) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary).
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

func styleInput(text string, width int) string {
	displayText := text
	if len(displayText) == 0 {
		displayText = "(enter message...)"
	}

	if len(displayText) > width {
		displayText = displayText[:width]
	}

	style := lipgloss.NewStyle().
		Background(theme.MutedBg).
		Foreground(theme.Text).
		Width(width).
		Padding(0, 1)

	return style.Render(displayText)
}

func renderModal(content string, termWidth, termHeight int) string {
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
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

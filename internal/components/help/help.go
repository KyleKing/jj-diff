package help

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	visible bool
	mode    string
}

func New() Model {
	return Model{
		visible: false,
	}
}

func (m *Model) Show(mode string) {
	m.visible = true
	m.mode = mode
}

func (m *Model) Hide() {
	m.visible = false
}

func (m *Model) IsVisible() bool {
	return m.visible
}

func (m Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	modalWidth := 60
	if modalWidth > width-10 {
		modalWidth = width - 10
	}

	var lines []string
	lines = append(lines, styleHeader("Keybindings", modalWidth))
	lines = append(lines, "")

	lines = append(lines, styleSection("Navigation", modalWidth))
	lines = append(lines, keyBinding("j/k or ↓/↑", "Move down/up", modalWidth))
	lines = append(lines, keyBinding("g", "Go to first file/hunk", modalWidth))
	lines = append(lines, keyBinding("G", "Go to last file/hunk", modalWidth))
	lines = append(lines, keyBinding("n", "Next hunk (when in diff view)", modalWidth))
	lines = append(lines, keyBinding("p", "Previous hunk (when in diff view)", modalWidth))
	lines = append(lines, keyBinding("Tab", "Switch focus (file list ↔ diff view)", modalWidth))
	lines = append(lines, "")

	lines = append(lines, styleSection("Actions", modalWidth))
	lines = append(lines, keyBinding("r", "Refresh diff from jj", modalWidth))
	if m.mode == "Interactive" {
		lines = append(lines, keyBinding("d", "Select destination revision", modalWidth))
		lines = append(lines, keyBinding("Space", "Toggle hunk selection", modalWidth))
		lines = append(lines, keyBinding("a", "Apply selected changes to destination", modalWidth))
	}
	lines = append(lines, keyBinding("?", "Toggle this help", modalWidth))
	lines = append(lines, keyBinding("q or Ctrl-C", "Quit", modalWidth))
	lines = append(lines, "")

	if m.mode == "Interactive" {
		lines = append(lines, styleSection("Interactive Mode", modalWidth))
		lines = append(lines, wrapText("1. Press 'd' to select a destination revision", modalWidth))
		lines = append(lines, wrapText("2. Navigate to hunks with 'n'/'p'", modalWidth))
		lines = append(lines, wrapText("3. Press Space to select/deselect hunks", modalWidth))
		lines = append(lines, wrapText("4. Press 'a' to apply selected hunks", modalWidth))
		lines = append(lines, "")
	}

	lines = append(lines, styleFooter("Press ? or Esc to close", modalWidth))

	content := strings.Join(lines, "\n")

	return renderModal(content, width, height)
}

func styleHeader(text string, width int) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Width(width).
		Align(lipgloss.Center)
	return style.Render(text)
}

func styleSection(text string, width int) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("11")).
		Width(width)
	return style.Render(text)
}

func styleFooter(text string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(width).
		Align(lipgloss.Center)
	return style.Render(text)
}

func keyBinding(key, description string, width int) string {
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("14")).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15"))

	keyWidth := 20
	descWidth := width - keyWidth - 2

	keyText := keyStyle.Render(padRight(key, keyWidth))
	descText := descStyle.Render(description)

	if len(descText) > descWidth {
		descText = descText[:descWidth-3] + "..."
	}

	return "  " + keyText + " " + descText
}

func wrapText(text string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Width(width - 4)
	return "  " + style.Render(text)
}

func padRight(text string, width int) string {
	if len(text) >= width {
		return text[:width]
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

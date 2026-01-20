package help

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/jj-diff/internal/theme"
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
	lines = append(lines, keyBinding("Ctrl-d/Ctrl-u", "Half-page down/up", modalWidth))
	lines = append(lines, keyBinding("Ctrl-f/Ctrl-b", "Full-page down/up", modalWidth))
	lines = append(lines, keyBinding("g", "Go to first file/hunk", modalWidth))
	lines = append(lines, keyBinding("G", "Go to last file/hunk", modalWidth))
	lines = append(lines, keyBinding("n", "Next hunk (when in diff view)", modalWidth))
	lines = append(lines, keyBinding("N/p", "Previous hunk (when in diff view)", modalWidth))
	lines = append(lines, keyBinding("[/]", "Previous/next file (when in diff view)", modalWidth))
	lines = append(lines, keyBinding("Tab", "Switch focus (file list ↔ diff view)", modalWidth))
	lines = append(lines, "")

	lines = append(lines, styleSection("Actions", modalWidth))
	lines = append(lines, keyBinding("r", "Refresh diff from jj", modalWidth))
	lines = append(lines, keyBinding("/", "Search in files and diff content", modalWidth))
	lines = append(lines, keyBinding("f", "Filter files (type to search)", modalWidth))
	lines = append(lines, "")

	lines = append(lines, styleSection("View Options", modalWidth))
	lines = append(lines, keyBinding("w", "Hide whitespace-only changes", modalWidth))
	lines = append(lines, keyBinding("W", "Toggle word-level diff highlighting", modalWidth))
	lines = append(lines, keyBinding("s", "Toggle side-by-side view", modalWidth))
	lines = append(lines, keyBinding("l", "Toggle line numbers", modalWidth))
	lines = append(lines, "")

	if m.mode == "Interactive" {
		lines = append(lines, keyBinding("d", "Select destination revision", modalWidth))
		lines = append(lines, keyBinding("Space", "Toggle hunk selection", modalWidth))
		lines = append(lines, keyBinding("v", "Enter visual mode (line selection)", modalWidth))
		lines = append(lines, keyBinding("j/k in visual", "Extend/contract line selection", modalWidth))
		lines = append(lines, keyBinding("Space in visual", "Confirm line selection", modalWidth))
		lines = append(lines, keyBinding("Esc", "Exit visual mode", modalWidth))
		lines = append(lines, keyBinding("a", "Apply selected changes to destination", modalWidth))
	}
	lines = append(lines, keyBinding("?", "Toggle this help", modalWidth))
	lines = append(lines, keyBinding("q or Ctrl-C", "Quit", modalWidth))
	lines = append(lines, "")

	if m.mode == "Interactive" {
		lines = append(lines, styleSection("Interactive Mode", modalWidth))
		lines = append(lines, wrapText("1. Press 'd' to select a destination revision", modalWidth))
		lines = append(lines, wrapText("2. Navigate to hunks with 'n'/'p'", modalWidth))
		lines = append(lines, wrapText("3. Press Space to select whole hunks", modalWidth))
		lines = append(lines, wrapText("4. Press 'v' for line-level selection (visual mode)", modalWidth))
		lines = append(lines, wrapText("   - Use j/k to extend selection range", modalWidth))
		lines = append(lines, wrapText("   - Press Space to confirm selection", modalWidth))
		lines = append(lines, wrapText("5. Press 'a' to apply selected hunks/lines", modalWidth))
		lines = append(lines, "")
	}

	lines = append(lines, styleFooter("Press ? or Esc to close", modalWidth))

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

func styleSection(text string, width int) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Secondary).
		Width(width)
	return style.Render(text)
}

func styleFooter(text string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(theme.SoftMutedBg).
		Width(width).
		Align(lipgloss.Center)
	return style.Render(text)
}

func keyBinding(key, description string, width int) string {
	keyStyle := lipgloss.NewStyle().
		Foreground(theme.Accent).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(theme.Text)

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
		Foreground(theme.Text).
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

package statusbar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/jj-diff/internal/theme"
)

type Model struct{}

func New() Model {
	return Model{}
}

func (m Model) View(width int, modeText, source, destination string) string {

	var parts []string
	parts = append(parts, fmt.Sprintf("[Mode: %s]", modeText))
	parts = append(parts, fmt.Sprintf("Source: %s", source))

	if destination != "" {
		parts = append(parts, fmt.Sprintf("→ Dest: %s", destination))
	}

	parts = append(parts, "Press ? for help")

	content := strings.Join(parts, " | ")

	style := lipgloss.NewStyle().
		Background(theme.SoftMutedBg).
		Foreground(theme.Text).
		Width(width)

	return style.Render(truncateOrPad(content, width))
}

func truncateOrPad(text string, width int) string {
	if len(text) > width {
		return text[:width]
	}
	return text + strings.Repeat(" ", width-len(text))
}

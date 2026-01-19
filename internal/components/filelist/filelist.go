package filelist

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/jj-diff/internal/diff"
)

type Model struct {
	files    []diff.FileChange
	selected int
}

func New() Model {
	return Model{
		files:    []diff.FileChange{},
		selected: 0,
	}
}

func (m *Model) SetFiles(files []diff.FileChange) {
	m.files = files
}

func (m *Model) SetSelected(idx int) {
	m.selected = idx
}

func (m Model) View(width, height int, focused bool) string {
	if len(m.files) == 0 {
		return padToSize("No files", width, height)
	}

	var lines []string
	lines = append(lines, styleHeader("Files", width))

	for i, file := range m.files {
		isSelected := i == m.selected
		line := m.renderFileLine(file, isSelected, focused)
		lines = append(lines, truncateOrPad(line, width))

		if len(lines) >= height {
			break
		}
	}

	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderFileLine(file diff.FileChange, selected, focused bool) string {
	changeIndicator := fmt.Sprintf("[%s]", file.ChangeType.String())

	line := fmt.Sprintf("%s %s", changeIndicator, file.Path)

	if selected {
		if focused {
			return styleSelectedFocused(line)
		}
		return styleSelected(line)
	}

	return styleNormal(line)
}

func styleHeader(text string, width int) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12"))
	return style.Render(truncateOrPad(text, width))
}

func styleSelectedFocused(text string) string {
	return lipgloss.NewStyle().
		Background(lipgloss.Color("25")).
		Foreground(lipgloss.Color("15")).
		Render(text)
}

func styleSelected(text string) string {
	return lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Render(text)
}

func styleNormal(text string) string {
	return text
}

func truncateOrPad(text string, width int) string {
	if len(text) > width {
		return text[:width-3] + "..."
	}
	return text + strings.Repeat(" ", width-len(text))
}

func padToSize(text string, width, height int) string {
	lines := []string{text}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

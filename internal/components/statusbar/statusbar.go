package statusbar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/jj-diff/internal/theme"
)

type Context struct {
	Destination  string
	FocusedPanel string
	IsVisualMode bool
	Mode         string
	Source       string
}

type Model struct{}

func New() Model {
	return Model{}
}

func (m Model) View(width int, modeText, source, destination string, isVisualMode bool) string {
	return m.ViewWithContext(width, Context{
		Mode:         modeText,
		Source:       source,
		Destination:  destination,
		IsVisualMode: isVisualMode,
		FocusedPanel: "files",
	})
}

func (m Model) ViewWithContext(width int, ctx Context) string {
	var parts []string
	if ctx.IsVisualMode {
		parts = append(parts, fmt.Sprintf("[Mode: %s - VISUAL]", ctx.Mode))
	} else {
		parts = append(parts, fmt.Sprintf("[Mode: %s]", ctx.Mode))
	}
	parts = append(parts, fmt.Sprintf("Source: %s", ctx.Source))

	if ctx.Destination != "" {
		parts = append(parts, fmt.Sprintf("→ Dest: %s", ctx.Destination))
	}

	parts = append(parts, m.getContextHints(ctx))

	content := strings.Join(parts, " | ")

	style := lipgloss.NewStyle().
		Background(theme.SoftMutedBg).
		Foreground(theme.Text).
		Width(width)

	return style.Render(truncateOrPad(content, width))
}

func (m Model) getContextHints(ctx Context) string {
	if ctx.IsVisualMode {
		return "j/k:select | Space:confirm | Esc:cancel"
	}

	if ctx.FocusedPanel == "files" {
		return "j/k:nav | Tab:diff | /:search | f:find | ?:help"
	}

	if ctx.Mode == "Interactive" {
		return "j/k:scroll | Space:select | w/s/l:view | ?:help"
	}
	return "j/k:scroll | Ctrl-d/u:page | w:ws | s:sbs | ?:help"
}

func truncateOrPad(text string, width int) string {
	if len(text) > width {
		return text[:width]
	}
	return text + strings.Repeat(" ", width-len(text))
}

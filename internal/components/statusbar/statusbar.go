// Package statusbar renders the single-line footer: the mode, the diff's source and destination, and
// the keybinding hints for whatever the user is doing.
package statusbar

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/jj-diff/internal/theme"
)

const panelFiles = "files"

// Context is what the footer describes. Destination is omitted from the render when empty, and
// FocusedPanel is "files" or the diff pane, which selects which hints are shown.
type Context struct {
	Destination  string
	FocusedPanel string
	Mode         string
	Source       string
	IsVisualMode bool
}

// Model is stateless: the footer is rendered entirely from the Context passed to each call.
type Model struct{}

// New returns the footer renderer.
func New() Model {
	return Model{}
}

// View renders the footer with the file list treated as focused. Use ViewWithContext to say which
// panel has focus.
func (m Model) View(width int, modeText, source, destination string, isVisualMode bool) string {
	return m.ViewWithContext(width, Context{
		Mode:         modeText,
		Source:       source,
		Destination:  destination,
		IsVisualMode: isVisualMode,
		FocusedPanel: panelFiles,
	})
}

// ViewWithContext renders the footer to exactly width columns. Content longer than that is clipped
// from the right with no ellipsis, which drops the rightmost hints without saying so.
func (m Model) ViewWithContext(width int, ctx Context) string {
	var parts []string
	if ctx.IsVisualMode {
		parts = append(parts, fmt.Sprintf("[Mode: %s - VISUAL]", ctx.Mode))
	} else {
		parts = append(parts, fmt.Sprintf("[Mode: %s]", ctx.Mode))
	}
	parts = append(parts, "Source: "+ctx.Source)

	if ctx.Destination != "" {
		parts = append(parts, "→ Dest: "+ctx.Destination)
	}

	parts = append(parts, m.getContextHints(ctx))

	content := strings.Join(parts, " | ")

	style := lipgloss.NewStyle().
		Background(theme.SoftMutedBg).
		Foreground(theme.Text).
		Width(width)

	return style.Render(truncateOrPad(content, width))
}

func (Model) getContextHints(ctx Context) string {
	if ctx.IsVisualMode {
		return "j/k:select | Space:confirm | Esc:cancel"
	}

	if ctx.Mode == "Diff-Editor" {
		if ctx.FocusedPanel == panelFiles {
			return "j/k:nav | Tab:diff | a:apply | ?:help"
		}

		return "j/k:scroll | Space:keep/drop | a:apply | ?:help"
	}

	if ctx.FocusedPanel == panelFiles {
		return "j/k:nav | Tab:diff | /:search | f:find | ?:help"
	}

	if ctx.Mode == "Interactive" {
		return "j/k:scroll | Space:select | a:apply | w/s/l:view | ?:help"
	}

	return "j/k:scroll | Ctrl-d/u:page | w:ws | s:sbs | ?:help"
}

func truncateOrPad(text string, width int) string {
	if width <= 0 {
		return ""
	}

	if len(text) > width {
		return text[:width]
	}

	return text + strings.Repeat(" ", max(width-len(text), 0))
}

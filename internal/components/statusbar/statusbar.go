// Package statusbar renders the single-line footer: the mode, the diff's source and destination, and
// the keybinding hints for whatever the user is doing.
package statusbar

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/jj-diff/internal/theme"
)

const (
	panelFiles = "files"

	modeDiffEditor  = "Diff-Editor"
	modeInteractive = "Interactive"

	hintSep  = " | "
	ellipsis = "…"

	hintApply   = "a:apply"
	hintDest    = "d:dest"
	hintHelp    = "?:help"
	hintNav     = "j/k:nav"
	hintScroll  = "j/k:scroll"
	hintTabDiff = "Tab:diff"
)

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

// ViewWithContext renders the footer to exactly width columns. Hints that do not fit are dropped
// from the right and replaced by an ellipsis, and the help hint always survives because it is the
// only route to the rest of the keymap.
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

	content := fitHints(strings.Join(parts, hintSep), m.getContextHints(ctx), width)

	style := lipgloss.NewStyle().
		Background(theme.SoftMutedBg).
		Foreground(theme.Text).
		Width(width)

	return style.Render(truncateOrPad(content, width))
}

// getContextHints returns the keys that do something right now, most useful first. Mode is checked
// before the focused panel: interactive mode starts on the file list, so testing the panel first
// hides its whole vocabulary behind a Tab press.
func (Model) getContextHints(ctx Context) []string {
	if ctx.IsVisualMode {
		return []string{"j/k:select", "Space:confirm", "Esc:cancel"}
	}

	onFiles := ctx.FocusedPanel == panelFiles

	switch ctx.Mode {
	case modeDiffEditor:
		if onFiles {
			return []string{hintNav, hintTabDiff, hintApply, hintHelp}
		}

		return []string{hintScroll, "Space:keep/drop", hintApply, hintHelp}
	case modeInteractive:
		if onFiles {
			return []string{hintNav, hintTabDiff, hintDest, hintApply, hintHelp}
		}

		return []string{hintScroll, "Space:select", "v:lines", hintDest, hintApply, hintHelp}
	}

	if onFiles {
		return []string{hintNav, hintTabDiff, "/:search", "f:find", hintHelp}
	}

	return []string{hintScroll, "Ctrl-d/u:page", "w:ws", "s:sbs", hintHelp}
}

// fitHints joins prefix and hints, dropping hints from the right until the result fits width. The
// last hint is kept whatever else goes, and an ellipsis marks that something was dropped.
func fitHints(prefix string, hints []string, width int) string {
	join := func(h []string) string {
		if len(h) == 0 {
			return prefix
		}

		return prefix + hintSep + strings.Join(h, hintSep)
	}

	full := join(hints)
	if lipgloss.Width(full) <= width || len(hints) < 2 {
		return full
	}

	lead, last := hints[:len(hints)-1], hints[len(hints)-1]

	for n := len(lead) - 1; n >= 0; n-- {
		kept := make([]string, 0, n+2)
		kept = append(kept, lead[:n]...)
		kept = append(kept, ellipsis, last)

		if candidate := join(kept); lipgloss.Width(candidate) <= width {
			return candidate
		}
	}

	return join([]string{last})
}

// truncateOrPad fits text to exactly width display cells, measuring in cells so a wide glyph cannot
// push the footer onto a second row.
func truncateOrPad(text string, width int) string {
	if width <= 0 {
		return ""
	}

	if used := lipgloss.Width(text); used <= width {
		return text + strings.Repeat(" ", width-used)
	}

	var b strings.Builder

	used := 0

	for _, r := range text {
		w := lipgloss.Width(string(r))
		if used+w > width {
			break
		}

		b.WriteRune(r)

		used += w
	}

	return b.String() + strings.Repeat(" ", width-used)
}

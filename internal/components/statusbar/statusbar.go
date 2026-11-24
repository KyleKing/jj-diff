// Package statusbar renders the single-line footer: the mode, the diff's source and destination, and
// the keybinding hints for whatever the user is doing.
package statusbar

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/keyhint"

	"github.com/kyleking/jj-diff/internal/theme"
)

const (
	panelFiles = "files"

	modeDiffEditor  = "Diff-Editor"
	modeInteractive = "Interactive"

	ellipsis = "…"

	// The footer keeps margin columns off the terminal's edge, in line with the panes above it.
	margin = 2
	// The facts are parted from the hints as one hint is from the next.
	partGap = theme.HintGap
	// Two part one fact from the next.
	factGap = "  "

	keyLines = "j/k"
	keySpace = "space"

	sourceShare    = 2
	minSourceWidth = 12
)

// The hints most screens offer, named once so a mode's list reads as what it adds.
var (
	hintApply   = keyhint.Hint{Key: "a", What: "apply"}
	hintDest    = keyhint.Hint{Key: "d", What: "dest"}
	hintHelp    = keyhint.Hint{Key: "?", What: "help"}
	hintNav     = keyhint.Hint{Key: keyLines, What: "nav"}
	hintScroll  = keyhint.Hint{Key: keyLines, What: "scroll"}
	hintTabDiff = keyhint.Hint{Key: "tab", What: "diff"}
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

// ViewWithContext renders the footer to exactly width cells: the mode and the revisions it moves
// between as quiet facts, then the keys that work here in the shape every sibling tool's footer
// takes, the key in the accent and what it does muted. Hints that do not fit are shed from the end
// of the list, and the help hint always survives because it is the only route to the rest of the
// keymap, as esc does in a visual selection. The facts give way before either does.
func (m Model) ViewWithContext(width int, ctx Context) string {
	if width <= 0 {
		return ""
	}

	lead := strings.Repeat(" ", min(margin, width))
	facts := facts(ctx, width)
	hints := m.getContextHints(ctx)
	room := width - margin - lipgloss.Width(facts) - len(partGap)

	if room < lipgloss.Width(keyhint.Pinned(hintStyles(), hints)) {
		facts, room = "", width-margin
	}

	line := keyhint.Fit(hintStyles(), hints, room)
	if facts != "" {
		line = facts + partGap + line
	}

	return padTo(lead+line, width)
}

// hintStyles is the footer face second-look and jojo share: bare keys in the accent, words muted.
func hintStyles() keyhint.Styles {
	return keyhint.Styles{Key: theme.Skin.Accent, Text: theme.Skin.Muted, Bare: true, Gap: partGap}
}

// facts is the mode, then the source and destination, each a step quieter than the one before.
func facts(ctx Context, width int) string {
	mode := theme.Skin.Subtitle.Render(ctx.Mode)
	if ctx.IsVisualMode {
		mode += " " + theme.Skin.Cursor.Render("VISUAL")
	}

	parts := []string{mode}
	if ctx.Source != "" {
		source := ellipsize(ctx.Source, max(width/sourceShare-margin, minSourceWidth))
		parts = append(parts, theme.Skin.Muted.Render(source))
	}

	if ctx.Destination != "" {
		parts = append(parts, theme.Skin.Accent.Render("→ ")+theme.Skin.Muted.Render(ctx.Destination))
	}

	return strings.Join(parts, factGap)
}

// getContextHints returns the keys that do something right now, most useful first, so the footer
// sheds from the end of the list. Mode is checked before the focused panel: interactive mode starts
// on the file list, so testing the panel first hides its whole vocabulary behind a Tab press.
func (Model) getContextHints(ctx Context) []keyhint.Hint {
	var hints []keyhint.Hint

	switch onFiles := ctx.FocusedPanel == panelFiles; {
	case ctx.IsVisualMode:
		hints = []keyhint.Hint{
			{Key: keyLines, What: "select"}, {Key: keySpace, What: "confirm"}, {Key: "esc", What: "cancel"},
		}
	case ctx.Mode == modeDiffEditor && onFiles:
		hints = []keyhint.Hint{hintNav, hintTabDiff, hintApply, hintHelp}
	case ctx.Mode == modeDiffEditor:
		hints = []keyhint.Hint{hintScroll, {Key: keySpace, What: "keep/drop"}, hintApply, hintHelp}
	case ctx.Mode == modeInteractive && onFiles:
		hints = []keyhint.Hint{hintNav, hintTabDiff, hintDest, hintApply, hintHelp}
	case ctx.Mode == modeInteractive:
		hints = []keyhint.Hint{
			hintScroll, {Key: keySpace, What: "select"}, {Key: "v", What: "lines"}, hintDest, hintApply, hintHelp,
		}
	case onFiles:
		hints = []keyhint.Hint{hintNav, hintTabDiff, {Key: "/", What: "search"}, {Key: "f", What: "find"}, hintHelp}
	default:
		hints = []keyhint.Hint{
			hintScroll, {Key: "ctrl-d/u", What: "page"}, {Key: "w", What: "ws"}, {Key: "s", What: "sbs"}, hintHelp,
		}
	}

	// Fit sheds the lowest priority first, so the order above is the order hints survive in. The
	// last is the way out of where the reader is (help, or esc in a visual selection) and is pinned.
	for i := range hints {
		hints[i].Priority = len(hints) - i
	}

	hints[len(hints)-1].Pin = true

	return hints
}

// ellipsize cuts text to at most width display cells, ending in an ellipsis when it cut.
func ellipsize(text string, width int) string {
	if lipgloss.Width(text) <= width {
		return text
	}

	return ansi.Truncate(text, width-1, "") + ellipsis
}

// padTo fits a styled line to exactly width cells, measuring in cells so a wide glyph cannot push
// the footer onto a second row.
func padTo(line string, width int) string {
	if used := lipgloss.Width(line); used > width {
		line = ansi.Truncate(line, width, "")
	}

	return line + strings.Repeat(" ", max(width-lipgloss.Width(line), 0))
}

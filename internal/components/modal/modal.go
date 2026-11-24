// Package modal draws the frame every jj-diff overlay sits in, so the help, the pickers, and the
// split dialogs read as one family with jojo's and second-look's: a rounded edge, the title as a
// labeled rule, the body, and the keys the overlay answers to as its last row, in the footer's
// own shape.
package modal

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/keyhint"

	"github.com/kyleking/jj-diff/internal/theme"
)

const (
	// ChromeRows is the rows the frame itself takes, its top and bottom border.
	ChromeRows = 2
	// ChromeCols is the columns the frame itself takes, the border and the padding inside it on
	// each side.
	ChromeCols = 4

	// Rows the box spends on its title rule and the blank line over its hints.
	headRows = 1
	footRows = 2

	// A frame keeps this margin off the terminal's edge.
	margin = 4
	// The narrowest a frame's inner width goes, below which nothing in one reads.
	floor = 20

	ellipsis = "…"
)

// Width is the room inside a frame: preferred, shrunk on a narrow terminal so the frame keeps a
// margin on each side, and never below a width anything fits in.
func Width(term, preferred int) int {
	return max(min(preferred, term-ChromeCols-2*margin), floor)
}

// BodyRows is how many body rows fit in a frame on a terminal height rows tall, once the frame, its
// title, and its hints are paid for.
func BodyRows(height int) int {
	return max(height-ChromeRows-headRows-footRows-margin, 1)
}

// Box frames body under a rule naming title, with count beside it, and hints as the last row. Every
// body line is cut or padded to width, the room inside the frame.
func Box(title, count string, body []string, hints []keyhint.Hint, width int) string {
	lines := make([]string, 0, len(body)+headRows+footRows)
	lines = append(lines, theme.Skin.Rule(title, count, width))

	for _, line := range body {
		lines = append(lines, Fit(line, width))
	}

	if len(hints) > 0 {
		lines = append(lines, "", Fit(keyhint.Fit(theme.Hints(), pinLast(hints), width), width))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Skin.Edge.GetForeground()).
		Padding(0, 1).
		Render(strings.Join(lines, "\n"))
}

// pinLast ranks hints in the order given and keeps the last, the way out, whatever else is shed.
func pinLast(hints []keyhint.Hint) []keyhint.Hint {
	out := make([]keyhint.Hint, len(hints))
	copy(out, hints)

	for i := range out {
		out[i].Priority = len(out) - i
	}

	out[len(out)-1].Pin = true

	return out
}

// Row draws one selectable line at width, on the selection tint when it is the cursor. The tint is
// the one the file list and every sibling tool use, quiet enough to read the row through.
func Row(text string, width int, selected bool) string {
	line := Fit(text, width)
	if !selected {
		return line
	}

	return onBackground(line, theme.Skin.Selection)
}

// onBackground lays a styled line on face's background. Each part of the line ends in a reset that
// would drop a background set around it, so the background is set again after every one.
//
//nolint:gocritic // hugeParam: lipgloss.Style is passed by value throughout lipgloss.
func onBackground(line string, face lipgloss.Style) string {
	const probe = "\x00"

	open, _, ok := strings.Cut(face.Render(probe), probe)
	if !ok || open == "" {
		return line
	}

	for _, reset := range []string{"\x1b[0m", "\x1b[m"} {
		line = strings.ReplaceAll(line, reset, reset+open)
	}

	return open + line + "\x1b[m"
}

// Fit cuts a styled line to width cells, marking the cut, or pads it out to width.
func Fit(line string, width int) string {
	if used := lipgloss.Width(line); used > width {
		return ansi.Truncate(line, width-1, "") + "\x1b[m" + theme.Skin.Muted.Render(ellipsis)
	}

	return line + strings.Repeat(" ", width-lipgloss.Width(line))
}

// Prompt is a text field: a chevron in the accent, what has been typed, and the cursor block.
func Prompt(text string) string {
	return theme.Skin.Accent.Render("› ") + theme.Skin.Body.Render(text) + theme.Skin.Accent.Render("▏")
}

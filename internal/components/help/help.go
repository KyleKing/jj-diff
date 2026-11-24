// Package help renders the keybinding overlay. The bindings are written out here by hand, so adding a
// key to the model means adding its row here too.
package help

import (
	"fmt"

	"github.com/kyleking/aragonite/tui/keyhint"

	"github.com/kyleking/jj-diff/internal/components/modal"
	"github.com/kyleking/jj-diff/internal/theme"
)

const (
	preferredWidth = 64

	// Larger than the overlay can ever be, so ScrollToEnd lands past the end and View clamps it.
	scrollEnd = 1 << 20
)

// keyEsc is the way out of the overlay and out of a line selection alike.
const keyEsc = "esc"

const (
	modeDiffEditor  = "Diff-Editor"
	modeInteractive = "Interactive"
)

// Model is the overlay's state. The mode selects which mode-specific bindings are listed and must
// match the mode strings the parent passes to Show.
type Model struct {
	mode    string
	scroll  int
	visible bool
}

// New returns a hidden overlay.
func New() Model {
	return Model{
		visible: false,
	}
}

// Show opens the overlay for a mode, where "Diff-Editor" and "Interactive" each add their own
// bindings and any other value lists only the shared ones.
func (m *Model) Show(mode string) {
	m.visible = true
	m.mode = mode
	m.scroll = 0
}

// ScrollBy moves the overlay's viewport by delta rows. View clamps the result against the content
// it actually has, so callers may pass any delta.
func (m *Model) ScrollBy(delta int) {
	m.scroll = max(m.scroll+delta, 0)
}

// ScrollToStart jumps to the first row of the overlay.
func (m *Model) ScrollToStart() {
	m.scroll = 0
}

// ScrollToEnd jumps to the last row of the overlay.
func (m *Model) ScrollToEnd() {
	m.scroll = scrollEnd
}

// Hide closes the overlay.
func (m *Model) Hide() {
	m.visible = false
}

// IsVisible reports whether the overlay is open, which is how the parent decides to route keys here.
func (m *Model) IsVisible() bool {
	return m.visible
}

// View renders the overlay for the given terminal size, returning the empty string while hidden.
// Content taller than the terminal scrolls, and the title says where the window is, so a short
// terminal never hides bindings without admitting it. It clamps the scroll offset as a side effect.
func (m *Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	inner := modal.Width(width, preferredWidth)
	body := legend(sections(m.mode), inner)
	visible := modal.BodyRows(height)

	if len(body) <= visible {
		m.scroll = 0

		return modal.Box("Keys", "", body, []keyhint.Hint{{Key: keyEsc, What: "close"}}, inner)
	}

	maxScroll := len(body) - visible
	m.scroll = min(m.scroll, maxScroll)
	window := body[m.scroll : m.scroll+visible]
	where := fmt.Sprintf("%d–%d of %d", m.scroll+1, m.scroll+visible, len(body))

	return modal.Box("Keys", where, window, []keyhint.Hint{
		{Key: "j/k", What: "scroll"}, {Key: "g/G", What: "ends"}, {Key: keyEsc, What: "close"},
	}, inner)
}

// section is one group of the legend under a labeled rule. Lead is a sentence drawn before the
// keys, for the group whose keys only make sense in order.
type section struct {
	name string
	lead string
	keys []keyhint.Hint
}

func sections(mode string) []section {
	out := []section{
		{name: "Navigation", keys: []keyhint.Hint{
			{Key: "j/k", What: "move down / up"},
			{Key: "ctrl-d/u", What: "half a page down / up"},
			{Key: "ctrl-f/b", What: "a page down / up"},
			{Key: "g/G", What: "first / last file or hunk"},
			{Key: "n", What: "next hunk"},
			{Key: "N/p", What: "previous hunk"},
			{Key: "[/]", What: "previous / next file"},
			{Key: "tab", What: "switch between the files and the diff"},
		}},
		{name: "Find", keys: []keyhint.Hint{
			{Key: "/", What: "search the files and the diff"},
			{Key: "f", What: "filter the file list"},
			{Key: "F", What: "find a file"},
			{Key: "r", What: "refresh from jj"},
		}},
		{name: "View", keys: []keyhint.Hint{
			{Key: "w", What: "hide whitespace-only changes"},
			{Key: "W", What: "word-level highlighting"},
			{Key: "s", What: "side by side"},
			{Key: "l", What: "line numbers"},
			{Key: "z", What: "fold the hunk or generated file"},
		}},
	}

	switch mode {
	case modeInteractive:
		out = append(out, section{
			name: "Moving changes",
			lead: "choose where they go, select them, then apply",
			keys: []keyhint.Hint{
				{Key: "d", What: "choose the destination"},
				{Key: "space", What: "select the hunk"},
				{Key: "v", What: "select lines: j/k to extend, space to confirm"},
				{Key: keyEsc, What: "leave the line selection"},
				{Key: "a", What: "apply to the destination"},
			},
		})
	case modeDiffEditor:
		out = append(out, section{
			name: "Editing the diff",
			keys: []keyhint.Hint{
				{Key: "space", What: "keep or drop the hunk"},
				{Key: "a", What: "apply and return to jj"},
			},
		})
	}

	return append(out, section{name: "General", keys: []keyhint.Hint{
		{Key: "?", What: "this help"},
		{Key: "q", What: "quit"},
	}})
}

// legend draws every section in one key column, so the keys line up from the first group to the
// last, with each group under a labeled rule and a blank row before it.
func legend(groups []section, width int) []string {
	var all []keyhint.Hint

	for _, g := range groups {
		all = append(all, keyhint.Hint{Head: true, What: g.name})
		if g.lead != "" {
			all = append(all, keyhint.Hint{What: g.lead})
		}

		all = append(all, g.keys...)
	}

	drawn := keyhint.Help(theme.Legend(), all, width)
	out := make([]string, 0, len(drawn)+len(groups))

	for i, h := range all {
		if !h.Head {
			out = append(out, drawn[i])

			continue
		}

		// A blank row before every group, the first included, keeps a group's rule off the title's.
		out = append(out, "", theme.Skin.Rule(h.What, "", width))
	}

	return out
}

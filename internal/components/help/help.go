// Package help renders the keybinding overlay from the legend the parent's keymap draws, so a key
// added to the keymap is listed here without this package hearing about it, and a rebound one is
// listed under its new key.
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

// Model is the overlay's state: the legend it was shown with and how far it has scrolled.
type Model struct {
	title   string
	legend  []keyhint.Hint
	scroll  int
	visible bool
}

// New returns a hidden overlay.
func New() Model {
	return Model{
		visible: false,
	}
}

// Show opens the overlay on legend, a keymap's legend for the screen the user is on: a Head hint
// opens each group and the hints under it are the keys. A hint with no key is a line of prose.
func (m *Model) Show(title string, legend []keyhint.Hint) {
	m.visible = true
	m.title = title
	m.legend = legend
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
	body := draw(m.legend, inner)
	visible := modal.BodyRows(height)

	if len(body) <= visible {
		m.scroll = 0

		return modal.Box(m.heading(), "", body, []keyhint.Hint{{Key: keyEsc, What: "close"}}, inner)
	}

	maxScroll := len(body) - visible
	m.scroll = min(m.scroll, maxScroll)
	window := body[m.scroll : m.scroll+visible]
	where := fmt.Sprintf("%d–%d of %d", m.scroll+1, m.scroll+visible, len(body))

	return modal.Box(m.heading(), where, window, []keyhint.Hint{
		{Key: "j/k", What: "scroll"}, {Key: "g/G", What: "ends"}, {Key: keyEsc, What: "close"},
	}, inner)
}

func (m *Model) heading() string {
	if m.title == "" {
		return "Keys"
	}

	return "Keys · " + m.title
}

// draw lays out every group in one key column, so the keys line up from the first group to the last,
// with each group under a labeled rule and a blank row before it.
func draw(all []keyhint.Hint, width int) []string {
	drawn := keyhint.Help(theme.Legend(), all, width)
	out := make([]string, 0, len(drawn)+len(all))

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

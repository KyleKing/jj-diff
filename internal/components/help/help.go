// Package help renders the keybinding overlay. The bindings are written out here by hand, so adding a
// key to the model means adding its row here too.
package help

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/jj-diff/internal/theme"
)

const (
	descriptionGap      = 3
	ellipsis            = "..."
	estimatedLineCount  = 48
	keyColumnWidth      = 20
	minBodyHeight       = 3
	modalChromeHeight   = 4
	modalHorizontalPad  = 2
	modalWidthMargin    = 10
	preferredModalWidth = 60
	wrapTextMargin      = 4

	// Larger than the overlay can ever be, so ScrollToEnd lands past the end and View clamps it.
	scrollEnd = 1 << 20
)

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

// View renders the overlay centered in the given terminal size, returning the empty string while
// hidden. Content taller than the terminal scrolls, and the footer says so, so a short terminal
// never hides bindings without admitting it. It clamps the scroll offset as a side effect.
func (m *Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	modalWidth := min(preferredModalWidth, width-modalWidthMargin)

	body := make([]string, 0, estimatedLineCount)
	body = append(body, styleHeader("Keybindings", modalWidth), "")
	body = append(body, navigationSection(modalWidth)...)
	body = append(body, actionSection(m.mode, modalWidth)...)
	body = append(body, viewOptionSection(modalWidth)...)
	body = append(body, globalSection(m.mode, modalWidth)...)
	body = append(body, interactiveGuideSection(m.mode, modalWidth)...)

	body, footer := m.window(body, height)

	lines := make([]string, 0, len(body)+1)
	lines = append(lines, body...)
	lines = append(lines, styleFooter(footer, modalWidth))

	return renderModal(strings.Join(lines, "\n"), width, height)
}

// window trims body to what fits above the footer and reports the footer text for that state.
func (m *Model) window(body []string, height int) ([]string, string) {
	const closeHint = "? or Esc to close"

	visible := max(height-modalChromeHeight-1, minBodyHeight)
	if len(body) <= visible {
		m.scroll = 0

		return body, "Press " + closeHint
	}

	maxScroll := len(body) - visible
	m.scroll = min(m.scroll, maxScroll)

	return body[m.scroll : m.scroll+visible],
		fmt.Sprintf("j/k to scroll · %d%% · %s", m.scroll*100/maxScroll, closeHint)
}

func navigationSection(width int) []string {
	return []string{
		styleSection("Navigation", width),
		keyBinding("j/k or ↓/↑", "Move down/up", width),
		keyBinding("Ctrl-d/Ctrl-u", "Half-page down/up", width),
		keyBinding("Ctrl-f/Ctrl-b", "Full-page down/up", width),
		keyBinding("g", "Go to first file/hunk", width),
		keyBinding("G", "Go to last file/hunk", width),
		keyBinding("n", "Next hunk (when in diff view)", width),
		keyBinding("N/p", "Previous hunk (when in diff view)", width),
		keyBinding("[/]", "Previous/next file (when in diff view)", width),
		keyBinding("Tab", "Switch focus (file list ↔ diff view)", width),
		"",
	}
}

func actionSection(mode string, width int) []string {
	lines := []string{
		styleSection("Actions", width),
		keyBinding("r", "Refresh diff from jj", width),
		keyBinding("/", "Search in files and diff content", width),
		keyBinding("f", "Filter files (type to search)", width),
		keyBinding("F", "Find file (fuzzy picker)", width),
	}

	if mode == modeDiffEditor {
		lines = append(lines,
			keyBinding("Space", "Keep or drop the current hunk", width),
			keyBinding("a", "Apply and return to jj", width),
		)
	}

	return append(lines, "")
}

func viewOptionSection(width int) []string {
	return []string{
		styleSection("View Options", width),
		keyBinding("w", "Hide whitespace-only changes", width),
		keyBinding("W", "Toggle word-level diff highlighting", width),
		keyBinding("s", "Toggle side-by-side view", width),
		keyBinding("l", "Toggle line numbers", width),
		"",
	}
}

func globalSection(mode string, width int) []string {
	var lines []string
	if mode == modeInteractive {
		lines = append(lines,
			keyBinding("d", "Select destination revision", width),
			keyBinding("Space", "Toggle hunk selection", width),
			keyBinding("v", "Enter visual mode (line selection)", width),
			keyBinding("j/k in visual", "Extend/contract line selection", width),
			keyBinding("Space in visual", "Confirm line selection", width),
			keyBinding("Esc", "Exit visual mode", width),
			keyBinding("a", "Apply selected changes to destination", width),
		)
	}

	return append(lines,
		keyBinding("?", "Toggle this help", width),
		keyBinding("q or Ctrl-C", "Quit", width),
		"",
	)
}

func interactiveGuideSection(mode string, width int) []string {
	if mode != modeInteractive {
		return nil
	}

	return []string{
		styleSection("Interactive Mode", width),
		wrapText("1. Press 'd' to select a destination revision", width),
		wrapText("2. Navigate to hunks with 'n'/'p'", width),
		wrapText("3. Press Space to select whole hunks", width),
		wrapText("4. Press 'v' for line-level selection (visual mode)", width),
		wrapText("   - Use j/k to extend selection range", width),
		wrapText("   - Press Space to confirm selection", width),
		wrapText("5. Press 'a' to apply selected hunks/lines", width),
		"",
	}
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

	descWidth := width - keyColumnWidth - descriptionGap

	keyText := keyStyle.Render(padRight(key, keyColumnWidth))
	descText := descStyle.Render(truncate(description, descWidth))

	return "  " + keyText + " " + descText
}

// truncate shortens text to width display columns, measured before any styling
// is applied so ANSI escapes are never counted or sliced through.
func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}

	if lipgloss.Width(text) <= width {
		return text
	}

	runes := []rune(text)
	if width <= len(ellipsis) {
		return string(runes[:width])
	}

	return string(runes[:width-len(ellipsis)]) + ellipsis
}

func wrapText(text string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(theme.Text).
		Width(width - wrapTextMargin)

	return "  " + style.Render(text)
}

func padRight(text string, width int) string {
	visible := lipgloss.Width(text)
	if visible >= width {
		return truncate(text, width)
	}

	return text + strings.Repeat(" ", width-visible)
}

func renderModal(content string, termWidth, termHeight int) string {
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Padding(1, modalHorizontalPad)

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

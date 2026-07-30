// Package commitmsg prompts for the commit message of one split tag during a multi-split.
// The parent model owns the prompt, routes keys to it while it is visible, and reads the
// finished message back out when the user confirms.
package commitmsg

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kyleking/jj-diff/internal/theme"
)

// SplitTag names one destination of a multi-split by its display letter ('A', 'B', ...).
type SplitTag rune

// Model is the commit message prompt. Keystrokes reach it through the parent model, which
// must copy it back into its own state because every mutator takes a pointer receiver.
type Model struct {
	tag     SplitTag
	message string
	visible bool
}

// New returns a hidden prompt aimed at tag 'A' with an empty message.
func New() Model {
	return Model{
		tag:     'A',
		message: "",
		visible: false,
	}
}

// SetTag points the prompt at a split tag and clears any message typed for the previous one.
func (m *Model) SetTag(tag SplitTag) {
	m.tag = tag
	m.message = ""
}

// Show reveals the prompt. While it is visible the parent model routes every key here, so
// call SetTag first to avoid collecting text against a stale tag.
func (m *Model) Show() {
	m.visible = true
}

// Hide takes the prompt off screen and keeps the typed message, which stays readable through
// GetMessage until the next SetTag.
func (m *Model) Hide() {
	m.visible = false
}

// IsVisible reports whether keys belong to the prompt rather than the main view.
func (m *Model) IsVisible() bool {
	return m.visible
}

// AppendChar adds one character to the end of the message with no length limit.
func (m *Model) AppendChar(ch rune) {
	m.message += string(ch)
}

// Backspace drops the final byte of the message, so a multi-byte rune is removed one byte at
// a time. An empty message is left alone.
func (m *Model) Backspace() {
	if len(m.message) > 0 {
		m.message = m.message[:len(m.message)-1]
	}
}

// GetMessage returns the text typed so far, empty when the user has entered nothing.
func (m Model) GetMessage() string {
	return m.message
}

// GetTag returns the split tag the current message belongs to.
func (m Model) GetTag() SplitTag {
	return m.tag
}

// View centers the prompt in a terminal of the given cell dimensions, returning an empty
// string while hidden so the caller can fall through to the view underneath.
func (m Model) View(width, height int) string {
	if !m.visible {
		return ""
	}

	modalWidth := width - 20
	if modalWidth < 50 {
		modalWidth = 50
	}
	if modalWidth > 80 {
		modalWidth = 80
	}

	var lines []string
	lines = append(
		lines,
		styleHeader(fmt.Sprintf("Commit Message for Tag [%s]", string(m.tag)), modalWidth),
	)
	lines = append(lines, "")

	inputBox := styleInput(m.message, modalWidth-4)
	lines = append(lines, "  "+inputBox)

	lines = append(lines, "")
	lines = append(lines, styleFooter("Enter: Confirm | Esc: Cancel | Type to edit", modalWidth))

	content := strings.Join(lines, "\n")

	return renderModal(content, width, height)
}

func styleHeader(text string, width int) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary).
		Width(width).
		Align(lipgloss.Center)

	return style.Render(text)
}

func styleFooter(text string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(theme.SoftMutedBg).
		Width(width).
		Align(lipgloss.Center)

	return style.Render(text)
}

func styleInput(text string, width int) string {
	displayText := text
	if len(displayText) == 0 {
		displayText = "(enter message...)"
	}

	if len(displayText) > width {
		displayText = displayText[:width]
	}

	style := lipgloss.NewStyle().
		Background(theme.MutedBg).
		Foreground(theme.Text).
		Width(width).
		Padding(0, 1)

	return style.Render(displayText)
}

func renderModal(content string, termWidth, termHeight int) string {
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Padding(1, 2)

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

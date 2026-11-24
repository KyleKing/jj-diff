// Package commitmsg prompts for the description of a commit a split creates. The parent model owns
// the prompt, routes keys to it while it is visible, and reads the finished message back out when
// the user confirms.
package commitmsg

import (
	"github.com/kyleking/aragonite/tui/keyhint"

	"github.com/kyleking/jj-diff/internal/components/modal"
	"github.com/kyleking/jj-diff/internal/theme"
)

// preferredWidth leaves room for a commit subject line at its conventional length.
const preferredWidth = 60

// Model is the commit message prompt. Keystrokes reach it through the parent model, which
// must copy it back into its own state because every mutator takes a pointer receiver.
type Model struct {
	message string
	visible bool
}

// New returns a hidden prompt with an empty message.
func New() Model {
	return Model{}
}

// Show reveals the prompt with an empty message. While it is visible the parent model routes every
// key here.
func (m *Model) Show() {
	m.visible = true
	m.message = ""
}

// Hide takes the prompt off screen and keeps the typed message, which stays readable through
// GetMessage until the next Show.
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

// Backspace drops the final character of the message. An empty message is left alone.
func (m *Model) Backspace() {
	if r := []rune(m.message); len(r) > 0 {
		m.message = string(r[:len(r)-1])
	}
}

// GetMessage returns the text typed so far, empty when the user has entered nothing.
func (m Model) GetMessage() string {
	return m.message
}

// View centers the prompt in a terminal of the given cell dimensions, returning an empty
// string while hidden so the caller can fall through to the view underneath.
func (m Model) View(width, _ int) string {
	if !m.visible {
		return ""
	}

	inner := modal.Width(width, preferredWidth)

	field := modal.Prompt(m.message)
	if m.message == "" {
		field = modal.Prompt("") + theme.Skin.Muted.Render("describe what goes in the new commit")
	}

	return modal.Box("Split out to a new commit", "", []string{field}, []keyhint.Hint{
		{Key: "enter", What: "confirm"}, {Key: "esc", What: "cancel"},
	}, inner)
}

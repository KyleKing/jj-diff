// Package commitmsg prompts for the commit message of one split tag during a multi-split.
// The parent model owns the prompt, routes keys to it while it is visible, and reads the
// finished message back out when the user confirms.
package commitmsg

import (
	"github.com/kyleking/aragonite/tui/keyhint"

	"github.com/kyleking/jj-diff/internal/components/modal"
	"github.com/kyleking/jj-diff/internal/theme"
)

// preferredWidth leaves room for a commit subject line at its conventional length.
const preferredWidth = 60

// SplitTag names one destination of a multi-split by its display letter ('A', 'B', ...).
type SplitTag rune

// Model is the commit message prompt. Keystrokes reach it through the parent model, which
// must copy it back into its own state because every mutator takes a pointer receiver.
type Model struct {
	message string
	tag     SplitTag
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
	if m.message != "" {
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
func (m Model) View(width, _ int) string {
	if !m.visible {
		return ""
	}

	inner := modal.Width(width, preferredWidth)

	field := modal.Prompt(m.message)
	if m.message == "" {
		field = modal.Prompt("") + theme.Skin.Muted.Render("describe what goes in this commit")
	}

	return modal.Box("Commit message", "tag "+string(m.tag), []string{field}, []keyhint.Hint{
		{Key: "enter", What: "confirm"}, {Key: "esc", What: "cancel"},
	}, inner)
}

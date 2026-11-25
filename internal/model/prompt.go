package model

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/tui/keyhint"
)

// The prompt contract every text entry in the family keeps: enter submits, ctrl+j (or alt+enter)
// starts a new line where the text can hold one, and esc cancels. The footer says so while a prompt
// has the keys, so nothing about typing has to be remembered.
const (
	keyNewline    = "ctrl+j"
	keyAltNewline = "alt+enter"
)

// isNewline reports whether key is the contract's newline.
func isNewline(key string) bool { return key == keyNewline || key == keyAltNewline }

// promptHints is the contract as the footer says it, submit naming what enter does here. A
// single-line prompt leaves the newline out, since it has none to make.
func promptHints(submit string, multiline bool) []keyhint.Hint {
	hints := []keyhint.Hint{{Key: keyEnter, What: submit}}
	if multiline {
		hints = append(hints, keyhint.Hint{Key: keyNewline, What: "newline"})
	}

	hints = append(hints, keyhint.Hint{Key: keyEsc, What: "cancel"})

	for i := range hints {
		hints[i].Priority = len(hints) - i
	}

	hints[len(hints)-1].Pin = true

	return hints
}

// promptFooter is the footer while a prompt has the keys, false when none does.
func (m *Model) promptFooter() ([]keyhint.Hint, bool) {
	switch {
	case m.cols != nil && m.cols.describing:
		return promptHints("set", true), true
	case m.fileList.IsFilterMode():
		return promptHints("open", false), true
	case m.picker.IsVisible() && m.picker.IsFiltering():
		return promptHints("keep", false), true
	}

	return nil, false
}

// oneLine is pasted text as a single-line prompt takes it: its lines joined by spaces.
func oneLine(text string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(text, "\r", "")), " ")
}

// pasted types a paste into whichever prompt has the keys, false when none does.
func (m *Model) pasted(msg tea.PasteMsg) (tea.Cmd, bool) {
	switch {
	case m.cols != nil && m.cols.describing:
		m.cols.draft += strings.ReplaceAll(msg.Content, "\r\n", "\n")
	case m.searchModal.IsVisible():
		m.searchState.Query += oneLine(msg.Content)
		m.searchModal.SetQuery(m.searchState.Query)

		return m.executeSearch(), true
	case m.fileList.IsFilterMode():
		m.fileList.SetFilterQuery(m.fileList.FilterQuery() + oneLine(msg.Content))
	case m.picker.IsVisible() && m.picker.IsFiltering():
		m.picker.SetQuery(m.picker.Query() + oneLine(msg.Content))
		m.refreshPicker()
	default:
		return nil, false
	}

	return nil, true
}

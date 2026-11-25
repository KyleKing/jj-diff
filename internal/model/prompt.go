package model

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/vimtext"

	"github.com/kyleking/jj-diff/internal/theme"
)

// Every text entry is an aragonite vimtext field keeping the family's contract: enter submits in
// every mode, ctrl+j (or alt+enter) starts a new line where the text can hold one, and esc backs out.
// Prose (a column's description) has the vim layer: it opens in insert mode with the cursor at the
// end, so typing needs no vim at all, esc goes to normal mode, and esc again cancels; JJ_DIFF_VIM=off
// drops the layer. A one-line box (search, the filters) is never modal and closes on one esc, as
// vim's own command line does, which keeps it in step with second-look. The footer says the
// contract while a prompt has the keys.
//
// The prompts, by the ID their field carries.
const (
	promptDescribe = "describe"
	promptSearch   = "search"
	promptFilter   = "filter"
	promptPick     = "pick"
)

// describeRows bounds a column's description field: it grows with its text to this many rows, then
// scrolls, so the column's hunks stay in view under it.
const describeRows = 6

// promptBaseOptions is how many options every prompt's field takes: its ID, the vim layer, and the
// skin's faces.
const promptBaseOptions = 3

// openPrompt gives the keys to a field holding value, styled in the theme's skin.
func (m *Model) openPrompt(id, value string, opts ...vimtext.Option) {
	all := make([]vimtext.Option, 0, len(opts)+promptBaseOptions)
	all = append(all, vimtext.WithID(id), vimtext.WithVim(m.cfg.Vim),
		vimtext.WithStyles(vimtext.StylesFor(&theme.Skin)))

	m.prompt = vimtext.New(value, append(all, opts...)...)
}

// openLinePrompt gives the keys to a one-line box: plain editing, with no modal layer whatever
// JJ_DIFF_VIM says, so one esc closes it.
func (m *Model) openLinePrompt(id, value string) {
	m.openPrompt(id, value, vimtext.WithOneLine(), vimtext.WithVim(false))
}

// handlePromptKey gives a key to the open field. The field never takes ctrl+c, so it quits from
// here as it does anywhere, and search keeps its own keys for stepping between matches while typing.
func (m *Model) handlePromptKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()
	if key == keyCtrlC {
		return m.cancel()
	}

	if m.prompt.ID() == promptSearch {
		switch key {
		case "ctrl+n", keyDown:
			return m.nextSearchMatch()
		case "ctrl+p", "up":
			return m.prevSearchMatch()
		}
	}

	return m.afterPrompt(m.prompt.Update(msg))
}

// afterPrompt follows a message the field took: the text so far goes to whatever filters live as it
// is typed, and a field that closed hands its text over at once, so no frame draws a finished field.
// The command is the field's answer, which on close is its SubmitMsg or CancelMsg and nothing else.
func (m *Model) afterPrompt(cmd tea.Cmd) tea.Cmd {
	f := m.prompt
	if f == nil {
		return cmd
	}

	if !f.Done() {
		m.promptChanged(f)

		return cmd
	}

	m.prompt = nil

	for _, msg := range run(cmd) {
		switch msg := msg.(type) {
		case vimtext.SubmitMsg:
			m.promptSubmitted(msg.ID, msg.Text)
		case vimtext.CancelMsg:
			m.promptCanceled(msg.ID)
		}
	}

	return nil
}

// run is what cmd answers, a batch read through, for the closing messages a field sends: pure
// closures over its text, which are safe to run on the Update loop.
func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}

	msg := cmd()

	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}

	var out []tea.Msg
	for _, c := range batch {
		out = append(out, run(c)...)
	}

	return out
}

// promptChanged narrows whatever the open field filters, live on every key, and leaves the rest
// alone until it closes.
func (m *Model) promptChanged(f *vimtext.Field) {
	value := f.Value()

	switch f.ID() {
	case promptSearch:
		if value != m.searchState.Query {
			m.searchState.Query = value
			m.searchModal.SetQuery(value)
			m.executeSearch()
		}
	case promptFilter:
		if value != m.fileList.FilterQuery() {
			m.fileList.SetFilterQuery(value)
		}
	case promptPick:
		if value != m.picker.Query() {
			m.picker.SetQuery(value)
		}

		// The box repeats the footer's keys, which follow the field's mode as well as its text.
		m.refreshPicker()
	}
}

// promptSubmitted is enter (or :w) in a field: each prompt keeps what it was typed for.
func (m *Model) promptSubmitted(id, text string) {
	switch id {
	case promptDescribe:
		if c := m.cols; c != nil && c.describing {
			c.describing = false
			draft, col := strings.TrimRight(text, "\n"), c.focus
			c.edit(func() { c.descs[col] = draft })
		}
	case promptSearch:
		m.searchModal.Hide()
		m.motion.Searched()
	case promptFilter:
		// Enter opens the best match, the top row the filter left, unless the cursor is already on one.
		if idx, ok := m.fileList.FirstShown(); ok && !m.fileList.Shows(m.selectedFile) {
			m.jumpToFile(idx)
		}

		m.fileList.SetFilterMode(false)
		m.focusedPanel = PanelDiffView
	case promptPick:
		m.picker.StopFilter()
		m.refreshPicker()
	}
}

// promptCanceled is esc out of a field's normal mode (or :q): each prompt puts back what it was
// before the field opened.
func (m *Model) promptCanceled(id string) {
	switch id {
	case promptDescribe:
		if m.cols != nil {
			m.cols.describing = false
		}
	case promptSearch:
		m.cancelSearch()
	case promptFilter:
		m.fileList.SetFilterMode(false)
	case promptPick:
		m.picker.SetQuery("")
		m.picker.StopFilter()
		m.refreshPicker()
	}
}

// closePrompt drops the open field without keeping or restoring anything, for a screen closing
// around it.
func (m *Model) closePrompt() { m.prompt = nil }

// promptVerb is what enter does in the open prompt, in the footer's word.
func promptVerb(id string) string {
	switch id {
	case promptDescribe:
		return "set"
	case promptFilter:
		return "open"
	}

	return "keep"
}

// promptFooter is the footer while a prompt has the keys, false when none does: the field's own
// keys for its mode, with enter named for what it does here.
func (m *Model) promptFooter() ([]keyhint.Hint, bool) {
	if m.prompt == nil {
		return nil, false
	}

	hints := m.prompt.Hints()
	verb := promptVerb(m.prompt.ID())

	for i := range hints {
		if hints[i].What == "save" {
			hints[i].What = verb
		}
	}

	return hints, true
}

// pasted types a paste into the open prompt, false when none is open. A one-line field takes a line
// break as a space.
func (m *Model) pasted(msg tea.PasteMsg) (tea.Cmd, bool) {
	if m.prompt == nil {
		return nil, false
	}

	text := strings.ReplaceAll(strings.ReplaceAll(msg.Content, "\r\n", "\n"), "\r", "\n")

	return m.afterPrompt(m.prompt.Update(tea.PasteMsg{Content: text})), true
}

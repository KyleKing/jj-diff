package model

import (
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/tui/vimtext"
)

// editApply takes text back from the user's $EDITOR into the model: a column's description or a
// conflict region.
type editApply func(m *Model, text string)

// editInEditor hands text to $VISUAL or $EDITOR (vi when neither is set) through vimtext.Edit, in a
// temporary file named by pattern, suspending the screen while it runs. What comes back goes to
// apply, which is kept under the request's own ID so a reply can only reach the request that asked.
func (m *Model) editInEditor(pattern, text string, apply editApply) tea.Cmd {
	m.editSeq++
	id := "edit-" + strconv.Itoa(m.editSeq)

	if m.edits == nil {
		m.edits = map[string]editApply{}
	}

	m.edits[id] = apply

	return vimtext.Edit(id, text, pattern)
}

// edited takes an editor's text home: to the open prompt when it asked (ctrl+e in a field), else to
// the request that asked. A failed edit applies nothing, so the text stays as it was.
func (m *Model) edited(msg vimtext.EditedMsg) tea.Cmd {
	if f := m.prompt; f != nil && f.ID() == msg.ID {
		return m.afterPrompt(f.Update(msg))
	}

	apply, ok := m.edits[msg.ID]
	delete(m.edits, msg.ID)

	if msg.Err != nil {
		m.say(firstLine(msg.Err), true)

		return nil
	}

	if ok {
		apply(m, msg.Text)
	}

	return nil
}

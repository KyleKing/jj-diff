package model

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/tui/keys"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
)

// u means one thing across the family: take back the last change. Here the last change is either a
// selection edit or a write, and the history is one line through both: a write starts the selection
// over, so any selection edit is newer than every write, and u takes those back first. Once none is
// left, u takes back the session's last write by restoring the operation recorded before it, which
// is jj undo for that write alone, refused when jj has run anything since.

// writeRecord is one write the session can take back or put again.
type writeRecord struct {
	// before is the operation the write started from, which an undo restores.
	before string
	// after is the operation the write left, which a redo restores.
	after string
	// current is the operation that must still be current for the next undo or redo to touch only
	// this write: after, or whatever the last undo or redo left.
	current string
	summary string
	verb    string
}

// rewoundMsg is an undo or a redo of a write coming home.
type rewoundMsg struct {
	err error
	rec writeRecord
	// op is the operation the restore made.
	op   string
	redo bool
}

// writesThroughJJ reports whether the session writes with jj commands u can take back: -i and the
// conflict viewer. The diff editor hands its result to jj, which owns that write.
func (m *Model) writesThroughJJ() bool {
	return m.mode == ModeInteractive || m.mode == ModeResolve
}

// writeMsg takes home whatever a write, an undo, or a redo answered.
func (m *Model) writeMsg(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case writtenMsg:
		return m.written(&msg)
	case rewoundMsg:
		return m.rewound(&msg)
	case diffEditorAppliedMsg:
		return tea.Quit
	}

	return nil
}

// writing marks a write running in jj until its writtenMsg lands, so an undo cannot race it.
func (m *Model) writing(cmd tea.Cmd) tea.Cmd {
	m.inFlight = true

	return cmd
}

// record keeps a write that landed for u. A write whose operations were not read (the merge tool
// runs outside the client) cannot be taken back here, and nothing before it can either, since
// restoring past it would take it back too.
func (m *Model) record(msg *writtenMsg) bool {
	m.unwritten = nil

	if msg.outcome.Before == "" || msg.outcome.After == "" {
		m.writes = nil

		return false
	}

	m.writes = append(m.writes, writeRecord{
		before: msg.outcome.Before, after: msg.outcome.After, current: msg.outcome.After,
		summary: msg.summary, verb: msg.verb,
	})

	return true
}

// selectionHistory is how many selection edits (or picks, in the conflict viewer) u can take back
// before it reaches a write.
func (m *Model) selectionHistory(redo bool) int {
	if m.mode == ModeResolve {
		if m.res == nil {
			return 0
		}

		if redo {
			return len(m.res.redo)
		}

		return len(m.res.undo)
	}

	if redo {
		return len(m.redo)
	}

	return len(m.undo)
}

// historyGate is whether u (or ctrl+r, redo) has anything to take back here, and why not.
func (m *Model) historyGate(redo bool) keys.Availability {
	if m.selectionHistory(redo) > 0 {
		return keys.Allow()
	}

	writes := len(m.writes)
	if redo {
		writes = len(m.unwritten)
	}

	switch {
	case writes == 0 && redo:
		return keys.Refuse(whyNothingToRedo)
	case writes == 0:
		return keys.Refuse(whyNothingToUndo)
	case m.inFlight:
		return keys.Refuse("jj is still writing; wait for it to land")
	}

	return keys.Allow()
}

// undoWord is the footer's word for u: plain undo for a selection edit, or the write it takes back.
func (m *Model) undoWord() string {
	if m.selectionHistory(false) > 0 || len(m.writes) == 0 {
		return ""
	}

	if verb := m.writes[len(m.writes)-1].verb; verb != "" {
		return "undo " + verb
	}

	return ""
}

// undoAction is u: the newest selection edit, else the newest write.
func (m *Model) undoAction() tea.Cmd {
	if len(m.undo) > 0 {
		m.undoSelection()

		return nil
	}

	return m.rewind(false)
}

// redoAction is ctrl+r: the selection edit u last took back, else the write.
func (m *Model) redoAction() tea.Cmd {
	if len(m.redo) > 0 {
		m.redoSelection()

		return nil
	}

	return m.rewind(true)
}

// rewind takes back the newest write (or, for redo, puts back the newest one taken back) by
// restoring an operation, only while the one that write left is still current.
func (m *Model) rewind(redo bool) tea.Cmd {
	if a := m.historyGate(redo); !a.OK {
		m.say(a.Why, false)

		return nil
	}

	stack := m.writes
	if redo {
		stack = m.unwritten
	}

	rec, client := stack[len(stack)-1], m.client

	to := rec.before
	if redo {
		to = rec.after
	}

	return m.writing(func() tea.Msg {
		op, err := client.Rewind(rec.current, to)

		return rewoundMsg{err: err, rec: rec, op: op, redo: redo}
	})
}

// rewound takes an undo or redo home: the record moves to the other stack and the diff is read
// again, or the footer says why nothing changed.
func (m *Model) rewound(msg *rewoundMsg) tea.Cmd {
	m.inFlight = false

	word := "undo"
	if msg.redo {
		word = "redo"
	}

	if errors.Is(msg.err, jj.ErrMoved) {
		// Every record is stale once something else wrote, since each restore would take that back too.
		m.writes, m.unwritten = nil, nil
		m.say(fmt.Sprintf("can't %s the %s: jj ran another operation since; jj op log shows it, and "+
			"jj op restore %s takes back everything after", word, msg.rec.verb, jj.ShortOp(msg.rec.before)), true)

		return nil
	}

	if msg.err != nil {
		m.say(firstLine(msg.err), true)

		return nil
	}

	rec := msg.rec
	rec.current = msg.op

	if msg.redo {
		m.unwritten = m.unwritten[:len(m.unwritten)-1]
		m.writes = append(m.writes, rec)
		m.say("redid: "+rec.summary+" · u takes it back again", false)
	} else {
		m.writes = m.writes[:len(m.writes)-1]
		m.unwritten = append(m.unwritten, rec)
		m.say("undid: "+rec.summary+" · jj op restore "+jj.ShortOp(rec.before)+" · ctrl+r redoes it", false)
	}

	if m.mode == ModeResolve {
		return m.loadConflicts()
	}

	return m.loadDiff()
}

// changedLines counts the added and deleted lines sel picks, which is what a discard throws away.
func (m *Model) changedLines(sel *SelectionState) int {
	n := 0

	for _, file := range m.changes {
		for h, hunk := range file.Hunks {
			whole := sel.IsHunkSelected(file.Path, h)
			if !whole && !sel.HasPartialSelection(file.Path, h) {
				continue
			}

			for l, line := range hunk.Lines {
				if line.Type != diff.LineContext && (whole || sel.IsLineSelected(file.Path, h, l)) {
					n++
				}
			}
		}
	}

	return n
}

// askDiscard is R: a discard that would throw lines away asks first, since they exist nowhere else
// once it runs. One that changes nothing has nothing to lose and does not ask.
func (m *Model) askDiscard(sel *SelectionState) tea.Cmd {
	if m.changedLines(sel) == 0 {
		m.say("nothing to discard: the selection changes no line", false)

		return nil
	}

	m.discarding = sel

	return nil
}

// discardQuestion is what R asks, in the footer's words.
func (m *Model) discardQuestion() string {
	sel := m.discarding

	return fmt.Sprintf("discard %s from %s?", plural(m.changedLines(sel), "line"), plural(m.hunks(sel), "hunk"))
}

// handleDiscardKey answers the discard question: y discards, and any other key keeps the lines and
// is spent on the answer, so a stray key never acts on the screen behind the question.
func (m *Model) handleDiscardKey(key string) tea.Cmd {
	sel := m.discarding
	m.discarding = nil

	if key != "y" {
		m.say("kept: nothing was discarded", false)

		return nil
	}

	return m.runRestore(sel)
}

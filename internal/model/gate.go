package model

import (
	"fmt"
	"strings"

	"github.com/kyleking/aragonite/tui/keys"
)

// Refusal reasons several screens share, each naming what is missing and the key that supplies it.
const (
	whyNothingSelected = "nothing selected; space selects a hunk, v a range"
	whyNoHunk          = "no hunk under the cursor; ]h finds one"
	whyNothingToUndo   = "nothing to undo"
	whyNothingToRedo   = "nothing to redo"
	whyNothingPicked   = "nothing picked yet: 1–9 takes a side, 0 the base, a all of them"
	whyAllInFirst      = "every change is still in column 1; > or 1–9 moves a hunk"
	whyNoRegion        = "no conflict region here; ]x finds one"
	whyNoConflicts     = "no conflicts here; ctrl+l reads them again"
)

// gate answers whether action can do anything on the screen the next key lands on, and why not.
// The footer dims what it refuses, the palette and the legend say the reason, and pressing a refused
// key says it in the footer instead of doing nothing.
func (m *Model) gate(action string) keys.Availability {
	if action == keys.ActionMessages && m.client == nil {
		return keys.Refuse("the diff editor runs no jj commands; jj ran this one")
	}

	switch m.context() {
	case keys.Conflict:
		return m.conflictGate(action)
	case keys.Split:
		return m.columnsGate(action)
	case keys.List:
		return m.listGate(action)
	case keys.Picker, keys.View, keys.Rounds, keys.Log, keys.Diff:
		return keys.Allow()
	case keys.Select:
		return m.selectGate(action)
	}

	return keys.Allow()
}

// listGate is the gate on the file list, which has nothing to open, filter, or search when the
// revision changes no file.
func (m *Model) listGate(action string) keys.Availability {
	if len(m.changes) > 0 {
		return keys.Allow()
	}

	switch action {
	case actionOpen, keys.ActionFilter, keys.ActionSearch, keys.ActionLineDown, keys.ActionLineUp:
		return keys.Refuse("no files in this revision; ctrl+l reads it again")
	}

	return keys.Allow()
}

// selectGate is the gate on the diff in the interactive and diff-editor modes.
func (m *Model) selectGate(action string) keys.Availability {
	switch action {
	case actionSelect, keys.ActionVisual:
		if !m.hasCurrentHunk() {
			return keys.Refuse(whyNoHunk)
		}
	case actionInvert:
		if m.selectedFile < 0 || m.selectedFile >= len(m.changes) {
			return keys.Refuse("no file under the cursor")
		}
	case actionSquash, actionSplit, actionRestore:
		return m.writeGate()
	case actionConfirm:
		if m.mode == ModeDiffEditor {
			return keys.Allow()
		}

		return m.writeGate()
	case keys.ActionRepeat:
		if m.again == nil {
			return keys.Refuse("nothing to repeat yet; s or R writes first")
		}

		return m.writeGate()
	case keys.ActionUndo:
		if len(m.undo) == 0 {
			return keys.Refuse(whyNothingToUndo)
		}
	case keys.ActionRedo:
		if len(m.redo) == 0 {
			return keys.Refuse(whyNothingToRedo)
		}
	}

	return keys.Allow()
}

// writeGate is whether a write has a revision to write into and something to write.
func (m *Model) writeGate() keys.Availability {
	if m.client == nil || m.revision == "" {
		return keys.Refuse(errNoRevision.Error())
	}

	if m.selection.IsEmpty() && !m.hasCurrentHunk() {
		return keys.Refuse(whyNothingSelected)
	}

	return keys.Allow()
}

// columnsGate is the gate on the split columns.
func (m *Model) columnsGate(action string) keys.Availability {
	c := m.cols
	_, has := m.current()

	switch action {
	case actionSelect, keys.ActionVisual, actionInvert, keys.ActionMoveLeft, keys.ActionMoveRight, keys.ActionSendTo:
		if !has && c.sel.IsEmpty() {
			return keys.Refuse(fmt.Sprintf("column %d is empty; h/l focuses another", c.focus+1))
		}
	case actionConfirm, actionApply:
		if m.client == nil || m.revision == "" {
			return keys.Refuse(errNoRevision.Error())
		}

		if !m.columnsMoved() {
			return keys.Refuse(whyAllInFirst)
		}
	case keys.ActionDescribe, keys.ActionDescribeEditor:
		if c.focus == 0 {
			return keys.Refuse("column 1 keeps the source's description; l focuses a new commit")
		}
	case keys.ActionUndo:
		if len(c.undo) == 0 {
			return keys.Refuse(whyNothingToUndo)
		}
	case keys.ActionRedo:
		if len(c.redo) == 0 {
			return keys.Refuse(whyNothingToRedo)
		}
	}

	return keys.Allow()
}

// columnsMoved reports whether any change has left column 1, which is what a split needs.
func (m *Model) columnsMoved() bool {
	for _, col := range m.cols.owner {
		if col != 0 {
			return true
		}
	}

	return false
}

// conflictGate is the gate on the conflict viewer.
func (m *Model) conflictGate(action string) keys.Availability {
	r := m.res

	switch action {
	case keys.ActionTakeSide, keys.ActionTakeBase, keys.ActionTakeAll, keys.ActionResetRegion,
		keys.ActionEditRegion, keys.ActionMergeTool, actionConfirm, actionApply, keys.ActionUndo, keys.ActionRedo:
		if r == nil || len(r.files) == 0 {
			return keys.Refuse(whyNoConflicts)
		}
	default:
		return keys.Allow()
	}

	switch action {
	case keys.ActionTakeSide, keys.ActionTakeBase, keys.ActionTakeAll, keys.ActionResetRegion, keys.ActionEditRegion:
		region, ok := r.currentRegion()
		if !ok {
			return keys.Refuse(whyNoRegion)
		}

		if action == keys.ActionTakeBase && len(region.Bases) == 0 {
			return keys.Refuse("this region has no base; 1–9 takes a side")
		}
	case actionConfirm, actionApply:
		if !r.anyPicked() {
			return keys.Refuse(whyNothingPicked)
		}
	case keys.ActionUndo:
		if len(r.undo) == 0 {
			return keys.Refuse(whyNothingToUndo)
		}
	case keys.ActionRedo:
		if len(r.redo) == 0 {
			return keys.Refuse(whyNothingToRedo)
		}
	}

	return keys.Allow()
}

// anyPicked reports whether any region in any file has a pick to write.
func (r *resolveState) anyPicked() bool {
	for _, p := range r.picks {
		if len(p) > 0 {
			return true
		}
	}

	return false
}

// refused answers a press of action the gate refuses, saying why in the footer and logging it,
// and reports whether it did.
func (m *Model) refused(action string) bool {
	a := m.gate(action)
	if a.OK {
		return false
	}

	m.say(m.labelOf(action)+"  "+a.Why, false)
	m.usage.Gated(m.contextName(), action)

	return true
}

// labelOf is the key that presses action, as a legend spells it, else the action's name.
func (m *Model) labelOf(action string) string {
	if b, ok := m.keymap.Binding(action); ok && len(b.Keys) > 0 {
		return b.Labels()
	}

	return strings.TrimPrefix(action, ":")
}

// contextName is the screen as the usage log files it.
func (m *Model) contextName() string {
	switch m.context() {
	case keys.View:
		return "view"
	case keys.Select:
		return "select"
	case keys.Split:
		return "split"
	case keys.Conflict:
		return "conflict"
	case keys.List:
		return "list"
	case keys.Picker:
		return "picker"
	case keys.Rounds, keys.Log, keys.Diff:
	}

	return "screen"
}

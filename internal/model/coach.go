package model

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/coach"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/keys"

	"github.com/kyleking/jj-diff/internal/theme"
)

// coachState is what the coach's rules read about the screen.
type coachState struct {
	mode     OperatingMode
	selected int
	hunks    int
	unpicked int
	regions  int
	newCols  int
	ctx      keys.Context
	hasHunk  bool
	moved    bool
	// focusEmpty is the split columns' focus on a column that holds nothing.
	focusEmpty bool
	hasDest    bool
	diffEdits  bool
}

// state is the screen as the coach's rules read it. It is cheap, since the footer asks for it on
// every frame: counts over the selection, not over the diff's lines.
func (m *Model) state() coachState {
	s := coachState{ctx: m.context(), mode: m.mode, hasHunk: m.hasCurrentHunk(), hasDest: m.destination != ""}

	switch s.ctx {
	case keys.Conflict:
		if m.res != nil {
			s.unpicked, s.regions = m.res.unresolved()
		}
	case keys.Split:
		s.moved = m.columnsMoved()
		_, has := m.current()
		s.focusEmpty = !has

		for col := 1; col < m.cols.count(); col++ {
			if len(m.items(col)) > 0 {
				s.newCols++
			}
		}
	case keys.View, keys.Rounds, keys.List, keys.Picker, keys.Log, keys.Diff:
	case keys.Select:
		if !m.selection.IsEmpty() {
			s.selected = m.hunks(m.selection)
		}

		s.diffEdits = len(m.undo) > 0

		for _, f := range m.changes {
			s.hunks += len(f.Hunks)
		}
	}

	return s
}

// Priorities within one tone: the step a screen is for before the one that finishes it.
const (
	rankFirst = 2
	rankThen  = 1
)

// coachRules are the next steps per screen: on the diff, select then write; in the columns, move
// hunks out of column 1 then apply; in the conflict viewer, pick every region then confirm.
func coachRules() []coach.Rule[coachState] {
	return slices.Concat(selectRules(), editorRules(), columnsRules(), resolveRules())
}

// selectRules are the interactive diff's steps: select, then write.
func selectRules() []coach.Rule[coachState] {
	selecting := func(s coachState) bool { return s.ctx == keys.Select && s.mode == ModeInteractive }

	return []coach.Rule[coachState]{
		{
			ID: "jjdiff.select", Action: actionSelect, Tone: coach.Next, Priority: rankFirst,
			When: func(s coachState) bool { return selecting(s) && s.selected == 0 && s.hasHunk },
			Say:  func(coachState) string { return "nothing selected yet" },
			Why:  func(coachState) string { return "v selects a range of lines; s, S, or R then act on it" },
		},
		{
			ID: "jjdiff.squash", Action: actionSquash, Tone: coach.Next, Priority: rankThen,
			Cmd:  "jj squash --from S --into D",
			When: func(s coachState) bool { return selecting(s) && s.selected > 0 && !s.hasDest },
			Say:  func(s coachState) string { return plural(s.selected, "hunk") + " selected" },
			Why:  func(coachState) string { return "S splits them out instead, R discards them" },
		},
		{
			ID: "jjdiff.confirm-dest", Action: actionConfirm, Tone: coach.Next, Priority: rankThen,
			Cmd:  "jj squash --from S --into D",
			When: func(s coachState) bool { return selecting(s) && s.selected > 0 && s.hasDest },
			Say:  func(s coachState) string { return plural(s.selected, "hunk") + " selected for -d" },
			Why:  func(coachState) string { return "s picks another destination, S splits them out" },
		},
	}
}

// editorRules are the diff editor's steps: drop what jj should not keep, then confirm.
func editorRules() []coach.Rule[coachState] {
	return []coach.Rule[coachState]{
		{
			ID: "jjdiff.editor-drop", Action: actionSelect, What: "drop", Tone: coach.Next, Priority: rankFirst,
			When: func(s coachState) bool { return s.ctx == keys.Select && s.mode == ModeDiffEditor && !s.diffEdits },
			Say:  func(coachState) string { return "everything is still kept" },
			Why:  func(coachState) string { return "space drops a hunk from what jj keeps" },
		},
		{
			ID: "jjdiff.editor-confirm", Action: actionConfirm, Tone: coach.Next, Priority: rankThen,
			When: func(s coachState) bool { return s.ctx == keys.Select && s.mode == ModeDiffEditor && s.diffEdits },
			Say:  func(s coachState) string { return plural(s.selected, "hunk") + " kept" },
			Why:  func(coachState) string { return "c hands them back to jj" },
		},
	}
}

// columnsRules are the split columns' steps: move hunks out of column 1, then apply.
func columnsRules() []coach.Rule[coachState] {
	return []coach.Rule[coachState]{
		{
			ID: "jjdiff.columns-move", Action: keys.ActionMoveRight, Tone: coach.Next, Priority: rankFirst,
			When: func(s coachState) bool { return s.ctx == keys.Split && !s.moved && !s.focusEmpty },
			Say:  func(coachState) string { return "every change is still in column 1" },
			Why:  func(coachState) string { return "1–9 sends the hunk to a column, space selects several first" },
		},
		{
			ID: "jjdiff.columns-focus", Action: keys.ActionColumnLeft, Tone: coach.Next, Priority: rankFirst,
			When: func(s coachState) bool { return s.ctx == keys.Split && !s.moved && s.focusEmpty },
			Say:  func(coachState) string { return "every change is still in column 1" },
			Why:  func(coachState) string { return "then > or 1–9 moves a hunk out" },
		},
		{
			ID: "jjdiff.columns-apply", Action: actionApply, Tone: coach.Next, Priority: rankThen,
			Cmd:  "jj split -r S -B S",
			When: func(s coachState) bool { return s.ctx == keys.Split && s.moved },
			Say:  func(s coachState) string { return plural(s.newCols+1, "commit") + " ready" },
			Why:  func(coachState) string { return "d describes a column, -p makes them siblings" },
		},
	}
}

// resolveRules are the conflict viewer's steps: pick every region, then confirm.
func resolveRules() []coach.Rule[coachState] {
	return []coach.Rule[coachState]{
		{
			ID: "jjdiff.resolve-pick", Action: keys.ActionTakeSide, Key: "1–9", Tone: coach.Next, Priority: rankFirst,
			When: func(s coachState) bool { return s.ctx == keys.Conflict && s.unpicked > 0 },
			Say:  func(s coachState) string { return fmt.Sprintf("%d of %d regions unpicked", s.unpicked, s.regions) },
			Why:  func(coachState) string { return "0 takes the base, a every side" },
		},
		{
			ID: "jjdiff.resolve-confirm", Action: actionConfirm, Tone: coach.Next, Priority: rankThen,
			Cmd:  "jj resolve -r R --tool :editor",
			When: func(s coachState) bool { return s.ctx == keys.Conflict && s.regions > 0 && s.unpicked == 0 },
			Say:  func(coachState) string { return "every region picked" },
			Why:  func(coachState) string { return "one jj resolve writes them all" },
		},
	}
}

// coachTips are keys worth knowing that a reader may not reach for, the ones a first session needs
// first.
func coachTips() []coach.Suggestion {
	return []coach.Suggestion{
		{ID: "jjdiff.tip.palette", Action: keys.ActionPalette, Say: "finds any command by name and runs it"},
		{ID: "jjdiff.tip.hunk-next", Action: keys.NextAction(keys.ObjHunk), Say: "jumps to the next hunk"},
		{ID: "jjdiff.tip.visual", Action: keys.ActionVisual, Say: "selects a range of lines inside a hunk"},
		{ID: "jjdiff.tip.invert", Action: actionInvert, Say: "inverts the selection across the file"},
		{ID: "jjdiff.tip.messages", Action: keys.ActionMessages, Say: "lists every jj command this session ran"},
		{ID: "jjdiff.tip.sbs", Action: keys.ActionViewSideBySide, Say: "draws the diff side by side"},
	}
}

// walkRun is how many j presses in a row read as stepping a diff the long way, and viaPaletteRuns
// how many palette runs of one action teach its key.
const (
	walkRun        = 20
	viaPaletteRuns = 3
)

// coachDetectors watch for the long way round: walking a diff line by line where ]h jumps hunks,
// leaving the diff for the file list and back where ]f steps files, and the palette run where a
// key would do.
func coachDetectors(km *keys.Keymap) []coach.Detector {
	return []coach.Detector{
		coach.Repeat(keys.ActionLineDown, walkRun, coach.Suggestion{
			ID: "jjdiff.tip.walk", Action: keys.NextAction(keys.ObjHunk),
			Say: "jumps a hunk at a time, where j walks a line",
		}),
		coach.Instead([]string{keys.ActionSwitchPane, keys.ActionLineDown, keys.ActionSwitchPane}, coach.Suggestion{
			ID: "jjdiff.tip.file-next", Action: keys.NextAction(keys.ObjFile),
			Say: "steps to the next file without leaving the diff",
		}),
		coach.ViaPalette(viaPaletteRuns, func(action string) (string, string, bool) {
			b, ok := km.Binding(action)
			if !ok || len(b.Keys) == 0 {
				return "", "", false
			}

			return b.Labels(), b.Hint().What, true
		}),
	}
}

// observe tells the coach about a key the screen dispatched: any key clears a tip, and one that ran
// an action is a run the ledger counts.
func (m *Model) observe(action string) tea.Cmd {
	now := time.Now()

	if action != "" {
		m.coach.Observe(coach.Event{At: now, Action: action, Motion: moving(action)})
	}

	return m.coach.WakeAt(now)
}

// nextStep is the step the coach points at, false when none holds.
func (m *Model) nextStep() (coach.Suggestion, bool) {
	return m.coach.Next(m.state(), m.gate)
}

// coachFooter is the footer's lead and its hints with the next step's key brightened: a tip until
// the next key, else the next step, with the key the step names. In the band layout the lead is
// empty and the band carries both.
func (m *Model) coachFooter(hints []keyhint.Hint) (string, string, []keyhint.Hint) {
	tip, tipping := m.coach.Showing()
	next, ok := m.nextStep()

	if ok {
		hints = coach.Mark(hints, next)
	}

	switch {
	case m.band:
		return "", "", hints
	case tipping:
		return coach.TipLine(theme.Skin, tip, m.width), "", hints
	case ok:
		return coach.Line(theme.Skin, next, m.width), next.Key, hints
	}

	return "", "", hints
}

// bandLines is the opt-in band above the footer: every next step that holds and a showing tip, nil
// in the inline layout or with nothing to say.
func (m *Model) bandLines() []string {
	if !m.band {
		return nil
	}

	var tip *coach.Suggestion
	if t, ok := m.coach.Showing(); ok {
		tip = &t
	}

	return coach.BandLines(theme.Skin, m.coach.All(m.state(), m.gate), tip, m.width)
}

// screenHeight is the rows a screen draws in: the terminal less the band's rows, which the band
// takes for itself rather than covering, so the cursor row is never under it.
func (m *Model) screenHeight() int {
	return max(m.height-len(m.bandLines()), 1)
}

// withBand puts the band between a screen and its footer. The screen was drawn screenHeight rows
// tall, so the band adds rows instead of covering any, and costs nothing when it is off.
func (m *Model) withBand(frame string) string {
	band := m.bandLines()
	if len(band) == 0 {
		return frame
	}

	lines := strings.Split(frame, "\n")
	last := len(lines) - 1
	out := make([]string, 0, len(lines)+len(band))
	out = append(out, lines[:last]...)

	for _, l := range band[:min(len(band), m.height-1)] {
		out = append(out, ansi.Truncate(l, m.width, ""))
	}

	return strings.Join(append(out, lines[last]), "\n")
}

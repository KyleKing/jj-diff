//nolint:testpackage // white-box: these tests read the quit prompt and the columns' state.
package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// launchedIntoColumns is jj-diff --split: the columns open as the diff loads, with every change in
// column 1, and are the whole session.
func launchedIntoColumns(t *testing.T) *Model {
	t.Helper()

	m := NewTestModel(t, ModeInteractive)
	m.StartInColumns()
	m.loaded(TestChanges())

	if m.cols == nil {
		t.Fatal("--split did not open the columns")
	}

	return m
}

// quits reports whether cmd is, or batches, tea.Quit.
func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}

	switch msg := cmd().(type) {
	case tea.QuitMsg:
		return true
	case tea.BatchMsg:
		for _, c := range msg {
			if quits(c) {
				return true
			}
		}
	}

	return false
}

// TestQuit_SplitLaunchExitsOnQAndEsc is a session jojo opens with jj-diff --split: q or esc at the
// columns' top level leaves the program, rather than dropping to an interactive diff that then needs
// a second q.
func TestQuit_SplitLaunchExitsOnQAndEsc(t *testing.T) {
	t.Parallel()

	for _, key := range []tea.KeyPressMsg{KeyPress('q'), SpecialKey(tea.KeyEsc)} {
		t.Run(key.String(), func(t *testing.T) {
			t.Parallel()

			m := launchedIntoColumns(t)

			_, cmd := m.Update(key)
			if !quits(cmd) {
				t.Errorf("%s at the split's top level did not quit (columns open: %v)", key, m.cols != nil)
			}
		})
	}
}

// TestQuit_SplitLaunchAsksOverAMovedHunk is the same exit with work to lose: a hunk moved out of
// column 1 asks write, discard, or stay first, and d quits without writing.
func TestQuit_SplitLaunchAsksOverAMovedHunk(t *testing.T) {
	t.Parallel()

	m := Press(t, launchedIntoColumns(t), ">")
	if !m.columnsMoved() {
		t.Fatal("> moved nothing out of column 1")
	}

	_, cmd := m.Update(KeyPress('q'))
	if quits(cmd) || !m.quitPrompt {
		t.Fatal("q over a moved hunk quit without asking")
	}

	_, cmd = m.Update(KeyPress('d'))
	if !quits(cmd) {
		t.Error("d at the prompt did not quit")
	}
}

// TestQuit_SplitOpenedFromTheDiffStillBacksOut keeps S's columns a level over the diff: q there
// returns to the diff it was opened from.
func TestQuit_SplitOpenedFromTheDiffStillBacksOut(t *testing.T) {
	t.Parallel()

	m := Press(t, inColumns(t), "<", "q")
	if m.cols != nil {
		t.Error("q in columns opened with S did not return to the diff")
	}
}

// TestQuit_ResolveExitsOnEsc is jj-diff --resolve: esc at its top level leaves, as q does.
func TestQuit_ResolveExitsOnEsc(t *testing.T) {
	t.Parallel()

	_, cmd := inResolve(t).Update(SpecialKey(tea.KeyEsc))
	if !quits(cmd) {
		t.Error("esc in the conflict viewer did not quit")
	}

	m := Press(t, inResolve(t), "1", "esc")
	if !m.quitPrompt {
		t.Error("esc over an unwritten pick quit without asking")
	}
}

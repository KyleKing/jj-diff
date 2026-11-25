//nolint:testpackage // white-box: these tests read the quit prompt and the columns' state.
package model

import (
	"strings"
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

// TestQuit_SplitLaunchExitsOnQ is a session jojo opens with jj-diff --split: q at the columns' top
// level leaves the program, rather than dropping to an interactive diff that then needs a second q.
func TestQuit_SplitLaunchExitsOnQ(t *testing.T) {
	t.Parallel()

	m := launchedIntoColumns(t)

	_, cmd := m.Update(KeyPress('q'))
	if !quits(cmd) {
		t.Errorf("q at the split's top level did not quit (columns open: %v)", m.cols != nil)
	}
}

// TestQuit_EscAtTheRootNeverQuits holds esc to one meaning in every mode: back out one level, and at
// the root say which key quits rather than quit. An esc pressed to dismiss a card that already went
// must not end the session (or, launched from jojo, drop the split or the picks in progress).
func TestQuit_EscAtTheRootNeverQuits(t *testing.T) {
	t.Parallel()

	roots := map[string]func(t *testing.T) *Model{
		"browse": func(t *testing.T) *Model {
			t.Helper()

			m := NewTestModel(t, ModeBrowse)
			m.loaded(TestChanges())

			return m
		},
		"interactive": func(t *testing.T) *Model {
			t.Helper()

			m := NewTestModel(t, ModeInteractive)
			m.loaded(TestChanges())

			return m
		},
		"split launch": launchedIntoColumns,
		"resolve":      inResolve,
	}

	for name, open := range roots {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := open(t)

			// Enough esc to back out of every level there is, and then some.
			for range 4 {
				var cmd tea.Cmd

				_, cmd = m.Update(SpecialKey(tea.KeyEsc))
				if quits(cmd) {
					t.Fatal("esc quit the program")
				}
			}

			if !strings.Contains(m.notice, "nothing to back out of; q") {
				t.Errorf("esc at the root does not say which key quits: %q", m.notice)
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

// TestQuit_ResolveAsksOverUnwrittenPicks is jj-diff --resolve: q over an unwritten pick asks first.
func TestQuit_ResolveAsksOverUnwrittenPicks(t *testing.T) {
	t.Parallel()

	m := Press(t, inResolve(t), "1", "q")
	if !m.quitPrompt {
		t.Error("q over an unwritten pick quit without asking")
	}
}

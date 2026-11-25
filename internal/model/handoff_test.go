//nolint:testpackage // white-box: these tests read the outcome, the focus, and the welcome card's state.
package model

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/coach"

	"github.com/kyleking/jj-diff/internal/jj"
)

// These pin the sessions another program hands the terminal to for one job: jojo's --split and
// --resolve, and jj's diff editor. Each gives the terminal back once the job is done, and says how
// to finish it from the first frame.

var errSplitFailed = errors.New("jj split: a hook refused it")

// returningSplit is jojo's jj-diff --split: launched into the columns for one write.
func returningSplit(t *testing.T) *Model {
	t.Helper()

	m := launchedIntoColumns(t)
	m.ReturnAfterWrite()

	return m
}

// TestSplitLaunch_ExitsAfterTheSplitApplies is the blank wait jojo's S left: the split landed and
// jj-diff stayed on what remained. Now the write quits, and Outcome carries the footer's words for
// stderr.
func TestSplitLaunch_ExitsAfterTheSplitApplies(t *testing.T) {
	t.Parallel()

	m := returningSplit(t)

	cmd := m.written(&writtenMsg{summary: "split @ into 2 commits"})
	if !quits(cmd) {
		t.Fatal("a split launched with --split stayed open after it applied")
	}

	if !strings.HasPrefix(m.Outcome(), "split @ into 2 commits") {
		t.Errorf("Outcome = %q, want the split's summary", m.Outcome())
	}
}

// TestSplitLaunch_StaysOpenWhenTheWriteFails keeps a failed split on screen with its reason, since
// nothing was written and the columns are still the job.
func TestSplitLaunch_StaysOpenWhenTheWriteFails(t *testing.T) {
	t.Parallel()

	m := returningSplit(t)

	if cmd := m.written(&writtenMsg{err: errSplitFailed}); cmd != nil && quits(cmd) {
		t.Fatal("a failed split quit")
	}

	if m.Outcome() != "" {
		t.Errorf("a failed split reported the outcome %q", m.Outcome())
	}
}

// TestSplitFromTheDiff_StaysOpenAfterItApplies keeps S inside -i a step in a longer session: only a
// launch for one write returns.
func TestSplitFromTheDiff_StaysOpenAfterItApplies(t *testing.T) {
	t.Parallel()

	m := inColumns(t)

	if cmd := m.written(&writtenMsg{summary: "split @ into 2 commits"}); cmd == nil {
		t.Fatal("a split in -i did not read the diff again")
	}

	if m.Outcome() == "" {
		t.Error("the write's outcome was not kept")
	}
}

// returningResolve is jojo's jj-diff --resolve on a revision with two conflicted files.
func returningResolve(t *testing.T) *Model {
	t.Helper()

	m := inResolve(t)
	m.ReturnAfterWrite()

	if cmd := m.written(&writtenMsg{summary: "resolved 3 regions in 2 files"}); cmd == nil || !m.leaveIfResolved {
		t.Fatal("a resolve launched with --resolve did not read the conflicts again to see whether it is done")
	}

	return m
}

// TestResolveLaunch_ExitsOnceNoConflictRemains is the blank screen jojo's x left. Every conflict was
// written, nothing was left to show, and the footer offered only the help key.
func TestResolveLaunch_ExitsOnceNoConflictRemains(t *testing.T) {
	t.Parallel()

	m := returningResolve(t)

	_, cmd := m.Update(conflictsLoadedMsg{})
	if !quits(cmd) {
		t.Fatal("a resolve that left no conflict stayed open")
	}

	if !strings.HasPrefix(m.Outcome(), "resolved 3 regions") {
		t.Errorf("Outcome = %q, want the resolve's summary", m.Outcome())
	}
}

// TestResolveLaunch_StaysWhileAConflictRemains keeps a partial resolve on the conflicts it left, and
// says how many and how to leave.
func TestResolveLaunch_StaysWhileAConflictRemains(t *testing.T) {
	t.Parallel()

	m := returningResolve(t)

	_, cmd := m.Update(conflictsLoadedMsg{files: []jj.ConflictedFile{
		{Path: "f.txt", Kind: "2-sided conflict", Text: twoRegions},
	}})
	if cmd != nil && quits(cmd) {
		t.Fatal("a resolve that left a conflict quit")
	}

	if !strings.Contains(m.notice, "1 file still conflicted, q returns") {
		t.Errorf("the footer did not say what is left: %q", m.notice)
	}

	// A later read, such as ctrl+l, is not the write's: it never quits.
	if _, cmd := m.Update(conflictsLoadedMsg{}); cmd != nil && quits(cmd) {
		t.Error("a read with no write before it quit")
	}
}

// TestResolveWithoutLaunch_StaysAfterTheLastConflict keeps jj-diff -i's own resolve open: nobody is
// waiting for the terminal back.
func TestResolveWithoutLaunch_StaysAfterTheLastConflict(t *testing.T) {
	t.Parallel()

	m := inResolve(t)
	m.written(&writtenMsg{summary: "resolved 3 regions in 2 files"})

	if _, cmd := m.Update(conflictsLoadedMsg{}); cmd != nil && quits(cmd) {
		t.Error("a resolve nobody launched for one write quit")
	}
}

// TestDiffEditor_OpensOnTheDiffWithConfirmInTheFooter is jj squash -i or jj split -i with jj-diff as
// the tool: the first frame offers space and c, at every size a terminal opens at, without a Tab
// first.
func TestDiffEditor_OpensOnTheDiffWithConfirmInTheFooter(t *testing.T) {
	t.Parallel()

	for _, size := range frameSizes {
		t.Run(fmt.Sprintf("%dx%d", size.w, size.h), func(t *testing.T) {
			t.Parallel()

			m := NewTestModel(t, ModeDiffEditor)
			m.width, m.height = size.w, size.h
			m.loaded(TestChanges())

			Assert(t, m).FocusedPanelIs(PanelDiffView)

			footer := ansi.Strip(m.renderStatusBar())
			for _, want := range []string{"space", "c confirm"} {
				if !strings.Contains(footer, want) {
					t.Errorf("the first footer lacks %q: %q", want, footer)
				}
			}

			if strings.Contains(footer, "diff-editor") {
				t.Errorf("the footer repeats the screen's name as its source: %q", footer)
			}
		})
	}
}

// TestDiffEditor_FileListOffersConfirm is the same session after Tab to the file list: c still
// confirms there, and the footer says so.
func TestDiffEditor_FileListOffersConfirm(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeDiffEditor)
	m.loaded(TestChanges())
	m = Press(t, m, "tab")

	Assert(t, m).FocusedPanelIs(PanelFileList)

	if footer := ansi.Strip(m.renderStatusBar()); !strings.Contains(footer, "c confirm") {
		t.Errorf("the file list's footer lacks c confirm: %q", footer)
	}

	if _, cmd := m.Update(KeyPress('c')); cmd == nil {
		t.Error("c on the file list did not confirm")
	}
}

// splitFirstRun is jj-diff --split on a ledger's first session.
func splitFirstRun(t *testing.T, width, height int) *Model {
	t.Helper()

	l, err := coach.LedgerAt(filepath.Join(t.TempDir(), "coach.json"))
	if err != nil {
		t.Fatal(err)
	}

	m := NewTestModel(t, ModeInteractive).WithLedger(l)
	m.StartInColumns()
	m.loaded(TestChanges())
	m.width, m.height = width, height

	return m
}

// TestWelcome_SplitLaunchTeachesTheColumns is the card jj-diff --split opened with: it listed s
// squash and S split over columns that take neither. It teaches the columns' keys at every size.
func TestWelcome_SplitLaunchTeachesTheColumns(t *testing.T) {
	t.Parallel()

	for _, size := range frameSizes {
		name := fmt.Sprintf("welcome_split_%dx%d", size.w, size.h)

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := splitFirstRun(t, size.w, size.h)
			assertStarters(t, m, []string{">", "1–9", "enter"}, []string{"s", "S"})

			frame := ansi.Strip(m.render())
			if !strings.Contains(frame, "Welcome to jj-diff") {
				t.Fatalf("the first launch did not open on the card:\n%s", frame)
			}

			frameGolden(t, filepath.Join("testdata", name+".golden"), frame+"\n", size.w)
		})
	}
}

// assertStarters checks the welcome card teaches every key in want and none in gone.
func assertStarters(t *testing.T, m *Model, want, gone []string) {
	t.Helper()

	keys := map[string]bool{}
	for _, s := range m.welcome.Starters {
		keys[s.Key] = true
	}

	for _, k := range want {
		if !keys[k] {
			t.Errorf("the card lacks %q: %+v", k, m.welcome.Starters)
		}
	}

	for _, k := range gone {
		if keys[k] {
			t.Errorf("the card teaches %q, which this screen does not take", k)
		}
	}
}

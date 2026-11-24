//nolint:testpackage // white-box: these tests read the gate, the coach, and the palette's state.
package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/keyusage"
	"github.com/kyleking/aragonite/trail"
	"github.com/kyleking/aragonite/tui/keys"
	"github.com/kyleking/aragonite/tui/palette"

	"github.com/kyleking/jj-diff/internal/jj"
)

// footerScreens are the states whose footers the golden pins: each screen, before and after the
// step its coach rule names.
//
//nolint:gochecknoglobals // a fixture
var footerScreens = []struct {
	build func(t *testing.T) *Model
	name  string
}{
	{name: "browse_diff", build: browseDiff},
	{name: "browse_files", build: browseFiles},
	{name: "interactive_nothing_selected", build: interactiveDiff},
	{name: "interactive_selected", build: interactiveSelected},
	{name: "diff-editor_all_kept", build: editorAllKept},
	{name: "columns_all_in_one", build: columnsAllInOne},
	{name: "columns_ready", build: inColumns},
	{name: "resolve_unpicked", build: inResolve},
	{name: "resolve_all_picked", build: resolveAllPicked},
}

func onDiff(t *testing.T, mode OperatingMode) *Model {
	t.Helper()

	m := NewTestModel(t, mode).WithChanges(TestChanges())
	m.focusedPanel = PanelDiffView

	return m
}

func browseDiff(t *testing.T) *Model      { t.Helper(); return onDiff(t, ModeBrowse) }
func interactiveDiff(t *testing.T) *Model { t.Helper(); return onDiff(t, ModeInteractive) }

func browseFiles(t *testing.T) *Model {
	t.Helper()

	return NewTestModel(t, ModeBrowse).WithChanges(TestChanges())
}

func interactiveSelected(t *testing.T) *Model {
	t.Helper()

	return Press(t, onDiff(t, ModeInteractive), "space")
}

func editorAllKept(t *testing.T) *Model {
	t.Helper()

	m := onDiff(t, ModeDiffEditor)
	m.loaded(TestChanges())
	m.focusedPanel = PanelDiffView

	return m
}

// columnsAllInOne moves the one hunk in column 2 back, leaving the focus on the empty column.
func columnsAllInOne(t *testing.T) *Model {
	t.Helper()

	return Press(t, inColumns(t), "<")
}

// columnsOnFirst is columnsAllInOne with the focus back on column 1.
func columnsOnFirst(t *testing.T) *Model {
	t.Helper()

	return Press(t, inColumns(t), "<", "h")
}

func resolveAllPicked(t *testing.T) *Model {
	t.Helper()

	return Press(t, inResolve(t), "a", "]", "x", "1", "]", "x", "2")
}

func noChangesModel(t *testing.T) *Model {
	t.Helper()

	m := NewTestModel(t, ModeInteractive)
	m.loaded(nil)

	return m
}

func nothingToSplit(t *testing.T) *Model {
	t.Helper()

	m := NewTestModel(t, ModeInteractive)
	m.StartInColumns()
	m.loaded(nil)

	return m
}

func noConflictsModel(t *testing.T) *Model {
	t.Helper()

	m := NewTestModel(t, ModeResolve)
	m.revision = "@"

	return Update(t, m, conflictsLoadedMsg{})
}

func columnsOnSecondThenFirst(t *testing.T) *Model {
	t.Helper()

	return Press(t, inColumns(t), "h")
}

func emptyDiff(t *testing.T) *Model {
	t.Helper()

	m := NewTestModel(t, ModeInteractive).WithChanges(nil)
	m.focusedPanel = PanelDiffView

	return m
}

// TestFooters_Golden pins every screen's footer at 80 and 100 columns: the keys, the ones the gate
// dims, and the next step the coach names in the status slot where it fits whole.
func TestFooters_Golden(t *testing.T) {
	t.Parallel()

	var b strings.Builder

	for _, s := range footerScreens {
		for _, size := range frameSizes {
			m := s.build(t)
			m.width, m.height = size.w, size.h

			fmt.Fprintf(&b, "%s %dx%d\n%s\n", s.name, size.w, size.h, ansi.Strip(m.renderStatusBar()))
		}
	}

	golden(t, filepath.Join("testdata", "footers.golden"), b.String())
}

// TestEmpty_Golden pins the cards drawn where there is nothing to show, which name the revision and
// the keys out.
func TestEmpty_Golden(t *testing.T) {
	t.Parallel()

	cases := []struct {
		build func(t *testing.T) *Model
		name  string
	}{
		{name: "no_changes", build: noChangesModel},
		{name: "nothing_to_split", build: nothingToSplit},
		{name: "no_conflicts", build: noConflictsModel},
	}

	for _, c := range cases {
		for _, size := range frameSizes {
			name := fmt.Sprintf("empty_%s_%dx%d", c.name, size.w, size.h)

			t.Run(name, func(t *testing.T) {
				t.Parallel()

				m := c.build(t)
				m.width, m.height = size.w, size.h

				frameGolden(t, filepath.Join("testdata", name+".golden"), ansi.Strip(m.render())+"\n", size.w)
			})
		}
	}
}

// TestPalette_Golden pins the command palette : opens, at the strip's width and the drawer's.
func TestPalette_Golden(t *testing.T) {
	t.Parallel()

	for _, size := range frameSizes {
		name := fmt.Sprintf("palette_interactive_%dx%d", size.w, size.h)

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := onDiff(t, ModeInteractive)
			m.width, m.height = size.w, size.h
			m = Press(t, m, ":", "a", "m", "e", "n", "d")

			frameGolden(t, filepath.Join("testdata", name+".golden"), ansi.Strip(m.render())+"\n", size.w)
		})
	}
}

// TestGate_RefusedPressSaysWhy is the end of silent no-ops: a key the screen cannot act on says why
// in the footer, and its hint is drawn off with the same reason.
func TestGate_RefusedPressSaysWhy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		build func(t *testing.T) *Model
		name  string
		want  string
		key   []string
	}{
		{name: "undo with no history", build: interactiveDiff, key: []string{"u"}, want: "u  nothing to undo"},
		{name: "repeat before any write", build: interactiveDiff, key: []string{"."}, want: "nothing to repeat yet"},
		{name: "apply with all in column 1", build: columnsAllInOne, key: []string{"enter"}, want: whyAllInFirst},
		{name: "describe column 1", build: columnsOnSecondThenFirst, key: []string{"d"}, want: "column 1 keeps"},
		{name: "confirm with nothing picked", build: inResolve, key: []string{"c"}, want: whyNothingPicked},
		{name: "squash with no hunk", build: emptyDiff, key: []string{"s"}, want: whyNothingSelected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := Press(t, tt.build(t), tt.key...)
			if !strings.Contains(m.notice, tt.want) {
				t.Errorf("notice = %q, want it to say %q", m.notice, tt.want)
			}
		})
	}
}

func TestGate_FooterDimsWhatItRefuses(t *testing.T) {
	t.Parallel()

	m := onDiff(t, ModeInteractive)

	for _, h := range m.footerHints() {
		if h.Key == "u" && (!h.Off || h.Why != whyNothingToUndo) {
			t.Errorf("u hint = %+v, want it off: nothing to undo", h)
		}
	}

	m = Press(t, m, "space")

	for _, h := range m.footerHints() {
		if h.Key == "u" && h.Off {
			t.Errorf("u is still off after a selection change: %+v", h)
		}
	}
}

// TestPalette_RunsByName covers : end to end: a git word finds the jj verb, Enter runs it by its
// action's name, and the footer teaches the key for next time.
func TestPalette_RunsByName(t *testing.T) {
	t.Parallel()

	m := Press(t, onDiff(t, ModeInteractive), ":", "s", "t", "a", "g", "e")

	sel, ok := m.pal.Selected()
	if !ok || sel.Action != actionSelect {
		t.Fatalf("stage found %+v, want select", sel)
	}

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = runCmd(t, m, cmd)

	if m.pal.IsOpen() {
		t.Error("the palette stayed open after a run")
	}

	if m.selection.IsEmpty() {
		t.Error("running select from the palette selected nothing")
	}

	if want := "next time space"; !strings.Contains(m.notice, want) {
		t.Errorf("notice = %q, want it to teach %q", m.notice, want)
	}
}

func TestPalette_FindsGitWords(t *testing.T) {
	t.Parallel()

	for word, want := range map[string]string{
		"amend": actionSquash, "fixup": actionSquash, "unstage": actionRestore, "discard": actionRestore,
	} {
		m := onDiff(t, ModeInteractive)
		m.openPalette(palette.Run)

		for _, r := range word {
			m = Update(t, m, KeyPress(r))
		}

		if sel, ok := m.pal.Selected(); !ok || sel.Action != want {
			t.Errorf("%s found %+v, want %s", word, sel, want)
		}
	}
}

func TestPalette_ExplainsTheJJCommand(t *testing.T) {
	t.Parallel()

	m := onDiff(t, ModeInteractive).WithDestination("rnzx")
	about := m.about()

	for action, want := range map[string]string{
		actionSquash:  "jj squash --from @ --into rnzx",
		actionSplit:   "jj split -r @ -B @",
		actionRestore: "jj restore --changes-in @",
	} {
		if got := about[action].Cmd; got != want {
			t.Errorf("%s runs %q, want %q", action, got, want)
		}
	}

	r := inResolve(t)
	if got := r.about()[actionConfirm].Cmd; got != "jj resolve -r @ --tool :editor" {
		t.Errorf("resolve's confirm runs %q", got)
	}
}

// TestPalette_NextTogglesTheBand covers the band's opt-in from the palette.
func TestPalette_NextTogglesTheBand(t *testing.T) {
	t.Parallel()

	m := onDiff(t, ModeInteractive)
	m.width, m.height = 100, 30

	m = Update(t, m, palette.RunMsg{Action: cmdNext, From: palette.Run})
	if !m.band {
		t.Fatal(":next did not turn the band on")
	}

	frame := ansi.Strip(m.render())
	if !strings.Contains(frame, "nothing selected yet") || !strings.Contains(frame, "next") {
		t.Errorf("the band does not name the next step:\n%s", frame)
	}
}

func TestCoach_NextStepPerScreen(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		build  func(t *testing.T) *Model
		name   string
		action string
	}{
		{name: "nothing selected", build: interactiveDiff, action: actionSelect},
		{name: "selected", build: interactiveSelected, action: actionSquash},
		{name: "all in column 1, focus empty", build: columnsAllInOne, action: keys.ActionColumnLeft},
		{name: "all in column 1", build: columnsOnFirst, action: keys.ActionMoveRight},
		{name: "columns ready", build: inColumns, action: actionApply},
		{name: "regions unpicked", build: inResolve, action: keys.ActionTakeSide},
		{name: "all picked", build: resolveAllPicked, action: actionConfirm},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			next, ok := tt.build(t).nextStep()
			if !ok || next.Action != tt.action {
				t.Errorf("next = %+v, want %s", next, tt.action)
			}
		})
	}
}

// TestCoach_WalkingTeachesTheHunkMotion is the Repeat detector: a long run of j queues ]h.
func TestCoach_WalkingTeachesTheHunkMotion(t *testing.T) {
	t.Parallel()

	m := onDiff(t, ModeBrowse)
	for range walkRun {
		m = Press(t, m, "j")
	}

	// Past the warmup and idle, the queued tip is the one that shows.
	tip, ok := m.coach.Tip(time.Now().Add(time.Hour), false)
	if !ok || tip.Action != keys.NextAction(keys.ObjHunk) {
		t.Errorf("tip = %+v, want ]h", tip)
	}

	m = Press(t, m, "k")
	if _, ok := m.coach.Showing(); ok {
		t.Error("a key press left the tip on screen")
	}
}

// TestMessages_ListsWhatRan covers gm: the trail's writes, Enter for stderr, and a copy as shell.
func TestMessages_ListsWhatRan(t *testing.T) {
	t.Parallel()

	m := onDiff(t, ModeInteractive)
	m.width, m.height = 100, 30
	m.client.Trail().Add(trail.Call{
		Argv: []string{"jj", "squash", "--from", "@", "--into", "rnzx"}, Write: true, Start: time.Now(),
	})
	m.client.Trail().Add(trail.Call{
		Argv: []string{"jj", "restore", "--changes-in", "@"}, Write: true, Exit: 1, Stderr: "Error: nope\n",
		Start: time.Now(),
	})

	m = Press(t, m, "g", "m")
	if m.msgs == nil {
		t.Fatal("gm did not open the command log")
	}

	frame := ansi.Strip(m.render())
	for _, want := range []string{"jj squash --from @ --into rnzx", "jj restore --changes-in @", "Error: nope"} {
		if !strings.Contains(frame, want) {
			t.Errorf("the log does not show %q:\n%s", want, frame)
		}
	}

	_, cmd := m.Update(KeyPress('Y'))
	if cmd == nil || !strings.Contains(m.notice, "jj restore --changes-in @") {
		t.Errorf("Y copied nothing: %q", m.notice)
	}

	m = Press(t, m, "esc")
	if m.msgs != nil {
		t.Error("esc did not close the log")
	}
}

func TestWritten_EchoesTheCommand(t *testing.T) {
	t.Parallel()

	m := onDiff(t, ModeInteractive)
	call := &trail.Call{Argv: jj.Echoed([]string{
		"jj", "squash", "--from", "@", "--into", "rnzx", "--tool", "jj-diff-selection", "--config", "x=1",
	})}

	m.written(writtenMsg{summary: "squashed 1 hunk into rnzx", call: call})

	if want := "squashed 1 hunk into rnzx · jj squash --from @ --into rnzx · jj undo undoes it"; m.notice != want {
		t.Errorf("notice = %q, want %q", m.notice, want)
	}
}

// TestUsage_LogsActionsAndRefusals covers the opt-in log: a key, the action it ran, and a press
// the gate refused.
func TestUsage_LogsActionsAndRefusals(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rec := keyusage.Open("jj-diff", keyusage.WithDir(dir))

	Press(t, onDiff(t, ModeInteractive).WithUsage(rec), "u", "space")

	if err := rec.Close(); err != nil {
		t.Fatal(err)
	}

	var log strings.Builder

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name())) //nolint:gosec // the test's own dir
		if err != nil {
			t.Fatal(err)
		}

		log.Write(data)
	}

	for _, want := range []string{`"undo"`, `"select"`, `"space"`} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("usage log lacks %s:\n%s", want, log.String())
		}
	}
}

// runCmd feeds the messages cmd produces back through Update, as the program would.
func runCmd(t *testing.T, m *Model, cmd tea.Cmd) *Model {
	t.Helper()

	for cmd != nil {
		msg := cmd()
		if msg == nil {
			return m
		}

		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				m = runCmd(t, m, c)
			}

			return m
		}

		var next tea.Model

		next, cmd = m.Update(msg)

		got, ok := next.(*Model)
		if !ok {
			t.Fatalf("Update answered %T", next)
		}

		m = got
	}

	return m
}

func TestPalette_NextFindsTheBand(t *testing.T) {
	t.Parallel()

	m := Press(t, interactiveDiff(t), ":", "n", "e", "x", "t")

	if sel, ok := m.pal.Selected(); !ok || sel.Action != cmdNext {
		t.Errorf("next found %+v, want :next", sel)
	}
}

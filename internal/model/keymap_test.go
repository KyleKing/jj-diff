//nolint:testpackage // white-box: these tests read the model's keymap, help, and selection state.
package model

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/framecheck"
	"github.com/kyleking/aragonite/tui/keys"
)

//nolint:gochecknoglobals // a fixture
var modes = []OperatingMode{ModeBrowse, ModeInteractive, ModeDiffEditor, ModeResolve}

// TestKeymap_AgreesWithShared is what keeps one key meaning one thing across jojo, jj-diff, and
// second-look: no key jj-diff binds may be a key the shared table gives another action on a screen
// the two share.
func TestKeymap_AgreesWithShared(t *testing.T) {
	t.Parallel()

	shared := slices.Concat(keys.Global(), keys.DiffView(), keys.FlagBindings(keys.VerbSquash),
		keys.FlagBindings(keys.VerbSplit), keys.SplitColumns(), keys.ConflictPicks())

	for _, mode := range modes {
		if got := keys.Disagreements(shared, toolBindings(mode)); len(got) != 0 {
			t.Errorf("%s: %q", modeName(mode), got)
		}

		for _, reads := range []bool{false, true} {
			km, err := keys.Merge(sharedBindings(mode, reads), toolBindings(mode), nil)
			if err != nil {
				t.Errorf("%s (reads %t): %v", modeName(mode), reads, err)

				continue
			}

			keys.RequireReserved(t, km)
		}
	}
}

// Every bound action shows up in the help for some screen, because the help is drawn from the
// keymap; a key the legend misses is a key nobody finds.
func TestHelp_ListsEveryBoundAction(t *testing.T) {
	t.Parallel()

	for _, mode := range modes {
		km := newKeymap(mode, true)
		listed := map[string]bool{}

		for _, ctx := range []keys.Context{keys.List, keys.View, keys.Select, keys.Picker, keys.Split, keys.Conflict} {
			for _, h := range km.Legend(ctx) {
				listed[h.Key+" "+h.What] = true
			}
		}

		for _, b := range km.Bindings() {
			if !listed[b.Labels()+" "+b.What] {
				t.Errorf("%s: %s (%s) is bound but in no legend", modeName(mode), b.Action, b.Labels())
			}
		}
	}
}

// The footer's keys come from the keymap, so each one it offers must press what it says.
func TestFooter_OffersOnlyBoundKeys(t *testing.T) {
	t.Parallel()

	for _, mode := range modes {
		m := NewTestModel(t, mode).WithChanges(TestChanges())

		for _, panel := range []FocusedPanel{PanelFileList, PanelDiffView} {
			m.focusedPanel = panel

			for _, h := range m.footerHints() {
				labels := strings.Split(h.Key, "/")
				if h.Key == "/" {
					labels = []string{"/"}
				}

				for _, key := range labels {
					if !m.bound(key) {
						t.Errorf("%s: the footer offers %q, which nothing binds", modeName(mode), key)
					}
				}
			}
		}
	}
}

// The columns' footer comes from the keymap as well.
func TestFooter_ColumnsOfferOnlyBoundKeys(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive).WithChanges(TestChanges())
	m.openColumns(nil)

	for _, visual := range []bool{false, true} {
		m.cols.visual = visual

		for _, h := range m.footerHints() {
			for _, key := range strings.Split(h.Key, "/") {
				if !m.bound(key) && !m.bound(h.Key) {
					t.Errorf("the columns' footer offers %q, which nothing binds", key)
				}
			}
		}
	}
}

func (m *Model) bound(label string) bool {
	for _, b := range m.keymap.Bindings() {
		if b.Labels() == label {
			return true
		}

		for _, k := range b.Keys {
			if strings.ReplaceAll(k, " ", "") == label || k == label {
				return true
			}
		}
	}

	return false
}

func TestWhichKey_ShowsWhileAPrefixWaits(t *testing.T) {
	t.Parallel()

	for _, size := range []struct{ w, h int }{{80, 24}, {120, 40}} {
		m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())
		m.width, m.height = size.w, size.h
		m.focusedPanel = PanelDiffView

		if strings.Contains(ansi.Strip(m.render()), "fold everything") {
			t.Fatal("the panel is up with nothing waiting")
		}

		m = Press(t, m, "z")

		frame := ansi.Strip(m.render())
		if !strings.Contains(frame, "fold everything") || !strings.Contains(frame, "z…") {
			t.Errorf("%dx%d: z waits without its panel:\n%s", size.w, size.h, frame)
		}

		if lines := strings.Split(frame, "\n"); len(lines) != size.h {
			t.Errorf("%dx%d: the panel changed the frame's height to %d", size.w, size.h, len(lines))
		}

		m = Press(t, m, "esc")
		if strings.Contains(ansi.Strip(m.render()), "fold everything") {
			t.Error("esc left the panel up")
		}
	}
}

func TestSelection_UndoAndRedo(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive).WithChanges(TestChanges())
	m.focusedPanel = PanelDiffView

	m = Press(t, m, "space", "]", "h", "space")
	Assert(t, m).HasHunkSelected("file1.txt", 1)

	m = Press(t, m, "u")
	Assert(t, m).HasHunkSelected("file1.txt", 0)
	Assert(t, m).HasHunkNotSelected("file1.txt", 1)

	m = Press(t, m, "u")
	Assert(t, m).HasHunkNotSelected("file1.txt", 0)

	m = Press(t, m, "ctrl+r", "ctrl+r")
	Assert(t, m).HasHunkSelected("file1.txt", 0)
	Assert(t, m).HasHunkSelected("file1.txt", 1)

	// A new edit forks the history, so there is nothing left to redo.
	m = Press(t, m, "u", "~", "ctrl+r")
	if !strings.Contains(m.notice, "nothing to redo") {
		t.Errorf("redo after a fresh edit said %q", m.notice)
	}
}

func TestSelection_InvertFlipsTheFile(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive).WithChanges(TestChanges())
	m.focusedPanel = PanelDiffView

	m = Press(t, m, "space", "~")
	Assert(t, m).HasHunkNotSelected("file1.txt", 0)
	Assert(t, m).HasHunkSelected("file1.txt", 1)
}

// q with a selection pending asks, rather than throwing the selection away.
func TestQuit_AsksOverAPendingSelection(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeInteractive).WithChanges(TestChanges())
	m.focusedPanel = PanelDiffView

	if _, cmd := m.Update(KeyPress('q')); cmd == nil || m.quitPrompt {
		t.Fatal("q with nothing selected should quit at once, but asked or did nothing")
	}

	m = Press(t, m, "space")
	if _, cmd := m.Update(KeyPress('q')); cmd != nil || !m.quitPrompt {
		t.Fatal("q over a selection quit without asking")
	}

	m = Press(t, m, "esc")
	if m.quitPrompt {
		t.Error("esc did not stay")
	}

	m = Press(t, m, "q")
	if _, cmd := m.Update(KeyPress('d')); cmd == nil {
		t.Error("d did not discard and quit")
	}
}

// Leaving the diff editor without writing must exit non-zero, which is what makes jj leave the
// revision as it was rather than take the whole right side.
func TestQuit_DiffEditorWithoutWritingAborts(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeDiffEditor).WithChanges(TestChanges())

	_, cmd := m.Update(KeyPress('q'))
	if cmd == nil || !m.Aborted() {
		t.Fatal("q in an untouched diff editor did not abort")
	}

	m = NewTestModel(t, ModeDiffEditor).WithChanges(TestChanges())
	m.focusedPanel = PanelDiffView
	m = Press(t, m, "space", "q")

	if !m.quitPrompt {
		t.Fatal("q after an edit in the diff editor did not ask")
	}

	if _, cmd := m.Update(KeyPress('w')); cmd == nil || m.Aborted() {
		t.Error("w did not write")
	}

	m = NewTestModel(t, ModeDiffEditor).WithChanges(TestChanges())
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); cmd == nil || !m.Aborted() {
		t.Error("ctrl+c in the diff editor did not abort")
	}
}

func TestFolds_ZFamilyReachesTheDiffView(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse).WithChanges(TestChanges())
	m.focusedPanel = PanelDiffView

	for _, seq := range [][]string{{"z", "a"}, {"z", "R"}, {"z", "M"}, {"z", "i"}, {"z", "z"}, {"z", "t"}, {"z", "b"}} {
		m = Press(t, m, seq...)
		if p := m.matcher.Pending(); p != nil {
			t.Errorf("%q left %q waiting", seq, p)
		}
	}
}

var update = flag.Bool("update", false, "rewrite the golden files") //nolint:gochecknoglobals // the test flag

// TestHelp_Golden pins the legend on each screen: the palette in browse mode, drawn over the screen it
// explains at the strip's width and the drawer's, so a change to a binding or a gate shows up as a
// reviewed diff of what the user reads.
func TestHelp_Golden(t *testing.T) {
	t.Parallel()

	for _, s := range helpScreens() {
		for _, size := range frameSizes {
			name := fmt.Sprintf("help_%s_%dx%d", s.name, size.w, size.h)

			t.Run(name, func(t *testing.T) {
				t.Parallel()

				m := s.build(t)
				m.width, m.height = size.w, size.h

				m = Press(t, m, "?")
				if !m.helpOpen() {
					t.Fatal("? did not open the legend")
				}

				frameGolden(t, filepath.Join("testdata", name+".golden"), ansi.Strip(m.render())+"\n", size.w)
			})
		}
	}
}

// helpScreen is one screen the legend is pinned on.
type helpScreen struct {
	build func(t *testing.T) *Model
	name  string
}

// helpScreens is every mode's file list and diff, the picker and the columns where interactive mode
// has them, and the conflict viewer, which is one screen.
func helpScreens() []helpScreen {
	var out []helpScreen

	for _, mode := range modes {
		prefix := strings.ToLower(modeName(mode)) + "_"

		if mode == ModeResolve {
			out = append(out, helpScreen{name: prefix + "diff", build: inResolve})

			continue
		}

		out = append(out,
			helpScreen{name: prefix + "files", build: func(t *testing.T) *Model {
				t.Helper()

				return NewTestModel(t, mode).WithChanges(TestChanges())
			}},
			helpScreen{name: prefix + "diff", build: func(t *testing.T) *Model {
				t.Helper()

				return onDiff(t, mode)
			}},
		)

		if mode == ModeInteractive {
			out = append(out,
				helpScreen{name: prefix + "picker", build: withPicker},
				helpScreen{name: prefix + "columns", build: inColumns},
			)
		}
	}

	return out
}

func withPicker(t *testing.T) *Model {
	t.Helper()

	m := onDiff(t, ModeInteractive)
	m.picker.Show()

	return m
}

// frameSizes are the terminal sizes the frame goldens pin: the strip's and the drawer's.
//
//nolint:gochecknoglobals // a fixture
var frameSizes = []struct{ w, h int }{{80, 24}, {100, 30}}

// frameGolden holds a whole frame to framecheck (every row exactly width cells, every box corner
// joined, no glyph a terminal may draw wider than counted) before comparing it to its golden, so a
// layout fault fails here with its row and column rather than reading as a plausible golden diff.
func frameGolden(t *testing.T, path, frame string, width int) {
	t.Helper()

	framecheck.Require(t, path, strings.TrimSuffix(frame, "\n"), framecheck.Width(width))
	golden(t, path, frame)
}

func golden(t *testing.T, path, got string) {
	t.Helper()

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}

		return
	}

	want, err := os.ReadFile(path) //nolint:gosec // G304: a fixture path the test builds.
	if err != nil {
		t.Fatalf("reading %s (run with -update to create it): %v", path, err)
	}

	if string(want) != got {
		t.Errorf("%s differs (run with -update and review the diff)\n--- got\n%s", path, got)
	}
}

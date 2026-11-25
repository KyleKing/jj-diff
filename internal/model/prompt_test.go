//nolint:testpackage // white-box: these tests read the columns, the prompt, and the footer.
package model

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/vimtext"

	"github.com/kyleking/jj-diff/internal/jj"
)

// errEditorFailed is an editor that exited with a failure, as vim's :cq does.
var errEditorFailed = errors.New("exit status 1")

// The prompt contract: in the column describe, enter submits, ctrl+j and alt+enter each start a new
// line, and esc backs out.
func TestPrompt_DescribeTakesCtrlJAndAltEnterAsNewline(t *testing.T) {
	t.Parallel()

	m := Press(t, inColumns(t), "d", "a", "ctrl+j", "b")
	m = Update(t, m, tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	m = Press(t, m, "c")

	if !m.cols.describing {
		t.Fatal("a newline key submitted the description")
	}

	m = Press(t, m, "enter")
	if got := m.cols.descs[1]; got != "a\nb\nc" {
		t.Errorf("description = %q, want three lines", got)
	}

	// The header draws the first line, and the frame keeps its height.
	frame := m.View().Content
	if !strings.Contains(frame, " a") || strings.Count(frame, "\n") != m.height-1 {
		t.Errorf("a multi-line description broke the header:\n%s", frame)
	}
}

// openers are the four prompts, each opened on a fresh model.
var openers = []struct {
	open func(t *testing.T, vim bool) *Model
	name string
	// submit is what enter does here, in the footer's word.
	submit string
	multi  bool
}{
	{name: "describe", submit: "set", multi: true, open: func(t *testing.T, vim bool) *Model {
		t.Helper()

		m := inColumns(t)
		m.cfg.Vim = vim

		return Press(t, m, "d")
	}},
	{name: "file filter", submit: "open", open: func(t *testing.T, vim bool) *Model {
		t.Helper()

		return Press(t, loadedModel(t, vim), "f")
	}},
	{name: "search", submit: "keep", open: func(t *testing.T, vim bool) *Model {
		t.Helper()

		return Press(t, loadedModel(t, vim), "/")
	}},
	{name: "picker filter", submit: "keep", open: func(t *testing.T, vim bool) *Model {
		t.Helper()

		return Press(t, inPicker(t, vim), "/")
	}},
}

func loadedModel(t *testing.T, vim bool) *Model {
	t.Helper()

	m := NewTestModel(t, ModeInteractive)
	m.cfg.Vim = vim
	m.loaded(TestChanges())

	return m
}

// inPicker is the squash picker up over the interactive diff, as revisionsLoadedMsg opens it.
func inPicker(t *testing.T, vim bool) *Model {
	t.Helper()

	m := loadedModel(t, vim)
	m.focusedPanel = PanelDiffView
	m.pending = Only("file1.txt", 0)
	m.openPicker(revisionsLoadedMsg{revisions: []jj.RevisionEntry{
		{ChangeID: "aaaa", Short: "aaaa", Description: "feat: the parser"},
		{ChangeID: "bbbb", Short: "bbbb", Description: "fix: the file list"},
		{ChangeID: "cccc", Short: "cccc", Description: "docs: the readme"},
	}})

	return m
}

// While a prompt has the keys, the footer says the contract in the field's words: the mode's badge,
// enter named for what it does here, ctrl+j where a newline means something, and esc, which leaves
// insert for normal mode and cancels from there. With the vim layer off there is no badge and esc
// cancels at once.
func TestPrompt_FooterSaysTheContract(t *testing.T) {
	t.Parallel()

	for _, tc := range openers {
		for _, width := range []int{80, 120} {
			m := Update(t, tc.open(t, true), tea.WindowSizeMsg{Width: width, Height: 24})
			footHas(t, tc.name+" insert", width, lastRow(m), tc.multi,
				"INSERT", "⏎ "+tc.submit, "esc normal")

			m = Press(t, m, "esc")
			footHas(t, tc.name+" normal", width, lastRow(m), false, "NORMAL", "⏎ "+tc.submit, "esc cancel")

			m = Update(t, tc.open(t, false), tea.WindowSizeMsg{Width: width, Height: 24})
			foot := lastRow(m)
			footHas(t, tc.name+" vim off", width, foot, tc.multi, "⏎ "+tc.submit, "esc cancel")

			if strings.Contains(foot, "INSERT") {
				t.Errorf("%s at %d: vim off still draws a badge: %q", tc.name, width, foot)
			}
		}
	}
}

func footHas(t *testing.T, name string, width int, foot string, multi bool, want ...string) {
	t.Helper()

	if multi {
		want = append(want, "ctrl+j new line")
	} else if strings.Contains(foot, "new line") {
		t.Errorf("%s at %d: a one-line prompt offers a new line: %q", name, width, foot)
	}

	for _, w := range want {
		if !strings.Contains(foot, w) {
			t.Errorf("%s at %d: footer lacks %q: %q", name, width, w, foot)
		}
	}
}

// esc backs out one level: insert to normal, normal to closed, with what the prompt was for put
// back. With the vim layer off one esc closes it.
func TestPrompt_EscBacksOutOneLevel(t *testing.T) {
	t.Parallel()

	for _, tc := range openers {
		m := Press(t, tc.open(t, true), "x", "esc")
		if m.prompt == nil || m.prompt.Mode() != vimtext.Normal {
			t.Errorf("%s: the first esc did not leave the field in normal mode", tc.name)

			continue
		}

		if m = Press(t, m, "esc"); m.prompt != nil {
			t.Errorf("%s: esc in normal mode left the field open", tc.name)
		}

		if m = Press(t, tc.open(t, false), "x", "esc"); m.prompt != nil {
			t.Errorf("%s: with vim off one esc left the field open", tc.name)
		}
	}
}

// Inside an open field vim's keys win: u in normal mode undoes the field's text and never reaches
// the keymap, where u is jj undo of the session's last write. The field's ctrl+r redoes the same
// way.
func TestPrompt_UInNormalModeUndoesTextNotTheWrite(t *testing.T) {
	t.Parallel()

	write := writeRecord{before: "aaaa", after: "bbbb", current: "bbbb", verb: "squash", summary: "squashed"}

	// Outside a prompt, u takes back the write, which is what the field must keep it from doing.
	bare := loadedModel(t, true)
	bare.writes = []writeRecord{write}

	if _, cmd := bare.Update(Key("u")); cmd == nil || !bare.inFlight {
		t.Fatal("u outside a prompt did not start an undo of the write; the test's setup is wrong")
	}

	for _, tc := range openers {
		m := tc.open(t, true)
		m.writes = []writeRecord{write}
		before := len(m.undo)

		m = Press(t, m, "a", "b", "c", "esc")

		_, cmd := m.Update(Key("u"))
		if m.inFlight || cmd != nil || len(m.writes) != 1 {
			t.Errorf("%s: u in the field ran jj undo (in flight %v, cmd %v)", tc.name, m.inFlight, cmd != nil)
		}

		if m.prompt == nil || m.prompt.Value() != "" {
			t.Errorf("%s: u did not undo the typing in the field", tc.name)

			continue
		}

		if len(m.undo) != before {
			t.Errorf("%s: u in the field took back a selection edit", tc.name)
		}

		if m = Press(t, m, "ctrl+r"); m.prompt.Value() != "abc" || m.inFlight {
			t.Errorf("%s: ctrl+r in the field = %q, in flight %v", tc.name, m.prompt.Value(), m.inFlight)
		}

		// v, Y, and . are the field's too: a visual yank of the text and a repeat change nothing of
		// the session's.
		m = Press(t, m, "0", "v", "l", "y", "x", ".")
		if m.prompt == nil || m.prompt.Value() != "c" || m.isVisualMode {
			t.Errorf("%s: v y x . in the field left %q (visual %v)", tc.name, valueOf(m), m.isVisualMode)
		}
	}
}

func valueOf(m *Model) string {
	if m.prompt == nil {
		return "(closed)"
	}

	return m.prompt.Value()
}

// ctrl+c is never the field's: it quits from inside any prompt, in either mode.
func TestPrompt_CtrlCQuits(t *testing.T) {
	t.Parallel()

	for _, tc := range openers {
		for _, keys := range [][]string{{"a"}, {"a", "esc"}} {
			m := Press(t, tc.open(t, true), keys...)

			_, cmd := m.Update(Key("ctrl+c"))
			if cmd == nil {
				t.Errorf("%s after %v: ctrl+c did nothing", tc.name, keys)

				continue
			}

			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Errorf("%s after %v: ctrl+c did not quit", tc.name, keys)
			}
		}
	}
}

// Search and the filters narrow on every key, in insert mode and after a normal-mode edit alike,
// since each reads the field's value after every message it takes.
func TestPrompt_FiltersFollowEveryKey(t *testing.T) {
	t.Parallel()

	m := Press(t, loadedModel(t, true), "/", "f", "i", "l", "e", "3")
	if m.searchState.Query != "file3" || m.searchState.MatchCount() == 0 {
		t.Errorf("search query %q with %d matches", m.searchState.Query, m.searchState.MatchCount())
	}

	m = Press(t, m, "esc", "x")
	if m.searchState.Query != "file" {
		t.Errorf("x in normal mode left the search at %q", m.searchState.Query)
	}

	f := Press(t, loadedModel(t, true), "f", "3")
	if f.fileList.FilterQuery() != "3" {
		t.Errorf("filter query = %q", f.fileList.FilterQuery())
	}

	if f = Press(t, f, "backspace"); f.fileList.FilterQuery() != "" {
		t.Errorf("backspace left the filter at %q", f.fileList.FilterQuery())
	}

	p := Press(t, inPicker(t, true), "/", "f", "e", "a")
	if p.picker.Query() != "fea" {
		t.Errorf("picker query = %q", p.picker.Query())
	}
}

// Enter in the file filter opens the top match, not the file the cursor sat on before the filter
// left it out.
func TestPrompt_FilterEnterOpensTheTopMatch(t *testing.T) {
	t.Parallel()

	m := Press(t, loadedModel(t, true), "f", "f", "i", "l", "e", "3", "enter")
	if m.selectedFile != 2 || m.focusedPanel != PanelDiffView {
		t.Errorf("enter opened file %d (focus %d), want file3.txt in the diff", m.selectedFile, m.focusedPanel)
	}
}

// A paste lands in the prompt that has the keys, keeping its newlines only where the prompt takes
// them.
func TestPrompt_PasteTypesIntoThePrompt(t *testing.T) {
	t.Parallel()

	m := Press(t, inColumns(t), "d")
	m = Update(t, m, tea.PasteMsg{Content: "fix: thé bug\r\n\r\nbody ✓"})
	m = Press(t, m, "enter")

	if got := m.cols.descs[1]; got != "fix: thé bug\n\nbody ✓" {
		t.Errorf("pasted description = %q", got)
	}

	f := Press(t, loadedModel(t, true), "f")
	f = Update(t, f, tea.PasteMsg{Content: "file\n2"})

	if got := f.fileList.FilterQuery(); got != "file 2" {
		t.Errorf("a paste into the filter left %q, want one line", got)
	}

	s := Press(t, loadedModel(t, true), "/", "esc")
	s = Update(t, s, tea.PasteMsg{Content: "file"})

	if s.searchState.Query != "file" {
		t.Errorf("a paste into search in normal mode left %q", s.searchState.Query)
	}
}

// ctrl+e in the description hands it to $EDITOR, and what comes back is the field's to keep or
// drop; a failed editor keeps the draft and says why.
func TestPrompt_EditorRoundTripReturnsToTheField(t *testing.T) {
	t.Parallel()

	m := Press(t, inColumns(t), "d", "d", "r", "a", "f", "t")

	if _, cmd := m.Update(Key("ctrl+e")); cmd == nil {
		t.Fatal("ctrl+e in the description started no editor")
	}

	m = Update(t, m, vimtext.EditedMsg{ID: promptDescribe, Err: errEditorFailed, Text: "draft"})
	if m.prompt == nil || m.prompt.Value() != "draft" {
		t.Fatalf("a failed editor lost the draft: %q", valueOf(m))
	}

	if foot := lastRow(m); !strings.Contains(foot, "exit status 1") {
		t.Errorf("the footer does not say the editor failed: %q", foot)
	}

	m = Update(t, m, vimtext.EditedMsg{ID: promptDescribe, Text: "feat: from nvim\n\nbody\n"})
	m = Press(t, m, "enter")

	if got := m.cols.descs[1]; got != "feat: from nvim\n\nbody" {
		t.Errorf("description after the editor = %q", got)
	}
}

// D sends the column's description to $EDITOR outside any field, and the answer reaches only the
// request that asked: one under another ID changes nothing.
func TestPrompt_EditorRepliesAreKeyedByRequest(t *testing.T) {
	t.Parallel()

	m := inColumns(t)

	if cmd := m.describeColumnInEditor(); cmd == nil || len(m.edits) != 1 {
		t.Fatalf("D started no editor request (%d waiting)", len(m.edits))
	}

	var id string
	for k := range m.edits {
		id = k
	}

	m = Update(t, m, vimtext.EditedMsg{ID: "edit-999", Text: "stray"})
	if m.cols.descs[1] != "" || len(m.edits) != 1 {
		t.Errorf("a reply to no request changed the description to %q", m.cols.descs[1])
	}

	m = Update(t, m, vimtext.EditedMsg{ID: id, Text: "  refactor: one\n"})
	if m.cols.descs[1] != "refactor: one" || len(m.edits) != 0 {
		t.Errorf("description = %q with %d requests waiting", m.cols.descs[1], len(m.edits))
	}
}

// Each prompt drawn at 80 columns in insert and in normal mode, held to the glyph and width policy
// before it is compared.
func TestPrompt_Frames(t *testing.T) {
	t.Parallel()

	for _, tc := range openers {
		for _, mode := range []string{"insert", "normal"} {
			name := fmt.Sprintf("prompt_%s_%s_80x24", strings.ReplaceAll(tc.name, " ", "_"), mode)

			t.Run(name, func(t *testing.T) {
				t.Parallel()

				m := Update(t, tc.open(t, true), tea.WindowSizeMsg{Width: 80, Height: 24})
				m = Press(t, m, "f", "i", "l", "e")

				if mode == "normal" {
					m = Press(t, m, "esc")
				}

				frameGolden(t, filepath.Join("testdata", name+".golden"), ansi.Strip(m.render())+"\n", 80)
			})
		}
	}
}

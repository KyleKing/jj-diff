package model

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/trail"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/keys"

	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
)

// target is what a verb acts on: the selection if there is one, else the hunk at the cursor, so a
// verb on one hunk needs no selecting first.
func (m *Model) target() (*SelectionState, error) {
	if !m.selection.IsEmpty() {
		return m.selection.Clone(), nil
	}

	if !m.hasCurrentHunk() {
		return nil, errNothingSelected
	}

	return Only(m.changes[m.selectedFile].Path, m.selectedHunk), nil
}

// hunks counts the hunks sel touches, for the notice that says what a write did.
func (m *Model) hunks(sel *SelectionState) int {
	n := 0

	for _, file := range m.changes {
		for idx := range file.Hunks {
			if sel.IsHunkSelected(file.Path, idx) || sel.HasPartialSelection(file.Path, idx) {
				n++
			}
		}
	}

	return n
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}

	return fmt.Sprintf("%d %ss", n, word)
}

// ready reports whether a write can run, saying why in the footer when not.
func (m *Model) ready() (*SelectionState, bool) {
	if m.client == nil || m.revision == "" {
		m.say(errNoRevision.Error(), true)

		return nil, false
	}

	sel, err := m.target()
	if err != nil {
		m.say(err.Error(), true)

		return nil, false
	}

	return sel, true
}

// squash opens the destination picker for the selection, starting on the -d destination if one
// was given, else on the revision's parent, which is where jj squash goes by default.
func (m *Model) squash() tea.Cmd {
	sel, ok := m.ready()
	if !ok {
		return nil
	}

	m.pending = sel
	client, revision, start := m.client, m.revision, m.destination

	return func() tea.Msg {
		if start == "" {
			start, _ = client.ChangeID(revision + "-") //nolint:errcheck // no parent starts on the first row
		}

		revisions, err := client.GetRevisions(jj.Destinations(revision), 0)

		return revisionsLoadedMsg{revisions: revisions, start: start, err: err}
	}
}

func (m *Model) openPicker(msg revisionsLoadedMsg) {
	if msg.err != nil {
		m.pending = nil
		m.say(firstLine(msg.err), true)

		return
	}

	m.closeAllModals()
	m.picker.SetRevisions(msg.revisions, msg.start)
	m.picker.Show()
	m.refreshPicker()
}

// refreshPicker rebuilds the command line and the keys the picker's box shows.
func (m *Model) refreshPicker() {
	dest := "…"
	if rev := m.picker.GetSelected(); rev != nil {
		dest = rev.Short
	}

	opts := m.moveOptions()
	parts := append([]string{"jj squash --from", m.revision, string(opts.Placement), dest}, m.flags.Args()...)

	m.picker.SetCommand(strings.Join(parts, " "))
	m.picker.SetHints(m.footerHints())
}

// moveOptions turns the picker's flags into what MoveChanges takes.
func (m *Model) moveOptions() jj.MoveOptions {
	opts := jj.MoveOptions{
		Placement:             jj.Into,
		KeepEmptied:           m.flags.On("k"),
		UseDestinationMessage: m.flags.On("u"),
	}

	switch {
	case m.flags.On("o"):
		opts.Placement = jj.Onto
	case m.flags.On("A"):
		opts.Placement = jj.InsertAfter
	case m.flags.On("B"):
		opts.Placement = jj.InsertBefore
	}

	return opts
}

// pickerAction is an action while the destination picker is up.
func (m *Model) pickerAction(action string) tea.Cmd {
	switch action {
	case keys.ActionLineDown:
		m.picker.Move(1)
	case keys.ActionLineUp:
		m.picker.Move(-1)
	case keys.ActionHalfPageDown, keys.ActionPageDown:
		m.picker.Move(helpPageStep)
	case keys.ActionHalfPageUp, keys.ActionPageUp:
		m.picker.Move(-helpPageStep)
	case keys.ActionTop:
		m.picker.Top()
	case keys.ActionBottom:
		m.picker.Bottom()
	case keys.ActionSearch:
		m.picker.StartFilter()
	case keys.ActionBack, keys.ActionQuit:
		m.picker.Hide()
		m.pending = nil
		m.quitAfterWrite = false
	case keys.ActionCancel:
		return m.cancel()
	case actionPick:
		return m.pick()
	default:
		if letter, ok := strings.CutPrefix(action, keys.FlagAction(keys.VerbSquash, "")); ok {
			m.flags.Toggle(letter)
		}
	}

	m.refreshPicker()

	return nil
}

// handlePickerFilterKeyPress types into the picker's filter until Enter keeps it or Esc clears it.
func (m *Model) handlePickerFilterKeyPress(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case keyEnter:
		m.picker.StopFilter()
	case keyEsc:
		m.picker.SetQuery("")
		m.picker.StopFilter()
	case keyBackspace:
		if r := []rune(m.picker.Query()); len(r) > 0 {
			m.picker.SetQuery(string(r[:len(r)-1]))
		}
	default:
		if text := msg.Key().Text; text != "" {
			m.picker.SetQuery(m.picker.Query() + text)
		}
	}

	m.refreshPicker()

	return nil
}

// pick runs the squash into the highlighted row.
func (m *Model) pick() tea.Cmd {
	rev := m.picker.GetSelected()
	if rev == nil || m.pending == nil {
		return nil
	}

	opts := m.moveOptions()
	if rev.Immutable && opts.Placement == jj.Into {
		m.say(errImmutable.Error(), true)

		return nil
	}

	sel := m.pending
	m.pending = nil
	m.picker.Hide()

	return m.runSquash(sel, rev.Short, opts)
}

// runSquash moves sel into dest. It reads a snapshot taken now, not the live model: a command runs
// off the Update loop, so keys handled while jj is working must not change what gets written.
func (m *Model) runSquash(sel *SelectionState, dest string, opts jj.MoveOptions) tea.Cmd {
	m.again = func(m *Model, sel *SelectionState) tea.Cmd { return m.runSquash(sel, dest, opts) }

	client, revision, changes, count := m.client, m.revision, m.changes, m.hunks(sel)

	return func() tea.Msg {
		outcome, err := client.MoveChanges(diff.GeneratePatch(changes, sel), revision, dest, opts)

		return writtenMsg{
			err: err, outcome: outcome, call: lastWrite(client),
			summary: fmt.Sprintf("squashed %s into %s", plural(count, "hunk"), dest),
		}
	}
}

// split opens the split columns with the selection, else the hunk at the cursor, in a new commit's
// column.
func (m *Model) split() {
	sel, ok := m.ready()
	if !ok {
		return
	}

	m.closeAllModals()
	m.openColumns(sel)
}

// restore discards the selection from the revision.
func (m *Model) restore() tea.Cmd {
	sel, ok := m.ready()
	if !ok {
		return nil
	}

	return m.runRestore(sel)
}

func (m *Model) runRestore(sel *SelectionState) tea.Cmd {
	m.again = (*Model).runRestore

	client, revision, changes, count := m.client, m.revision, m.changes, m.hunks(sel)

	return func() tea.Msg {
		outcome, err := client.Restore(diff.GeneratePatch(changes, sel), revision)

		return writtenMsg{
			err: err, outcome: outcome, call: lastWrite(client), summary: "discarded " + plural(count, "hunk"),
		}
	}
}

// confirm writes the selection: back to jj in the diff editor, else into the -d destination, else
// through the picker.
func (m *Model) confirm() tea.Cmd {
	switch m.mode {
	case ModeDiffEditor:
		return m.applyDiffEditorSelection()
	case ModeResolve:
		return m.writeResolution()
	case ModeBrowse, ModeInteractive:
	}

	if m.destination == "" {
		return m.squash()
	}

	sel, ok := m.ready()
	if !ok {
		return nil
	}

	return m.runSquash(sel, m.destination, m.moveOptions())
}

// repeat runs the last write again on what is selected now.
func (m *Model) repeat() tea.Cmd {
	if m.again == nil {
		m.say("nothing to repeat yet", false)

		return nil
	}

	sel, ok := m.ready()
	if !ok {
		return nil
	}

	return m.again(m, sel)
}

// written takes a write home: on success the diff is read again (the selection went with the old
// one) and the footer says what changed and how to take it back; on failure the footer says why,
// and nothing changed, since a failed write restores the operation it started from.
func (m *Model) written(msg writtenMsg) tea.Cmd {
	if msg.err != nil {
		m.quitAfterWrite = false
		m.say(firstLine(msg.err), true)

		return nil
	}

	undo := msg.outcome.Undo
	if undo == "" {
		undo = "jj undo"
	}

	text := msg.summary + " · " + undo + " reverses it"
	if msg.call != nil {
		text = trail.Echo(msg.summary, msg.call, undo+" undoes it")
	}

	// A partial resolve leaves the conflicts it did not touch, which jj reports as new ones.
	conflicts := msg.outcome.Conflicts != "" && m.mode != ModeResolve
	if conflicts {
		text = msg.summary + " · " + msg.outcome.Conflicts
	}

	m.say(text, conflicts)

	if m.quitAfterWrite {
		return tea.Quit
	}

	if m.mode == ModeResolve {
		return m.loadConflicts()
	}

	return m.loadDiff()
}

// lastWrite is the write the client just ran, as the status line names it: without the arguments
// that point jj at this program as its editor. It is nil when nothing has written.
func lastWrite(client *jj.Client) *trail.Call {
	call, ok := client.LastWrite()
	if !ok {
		return nil
	}

	call.Argv = jj.Echoed(call.Argv)

	return &call
}

func firstLine(err error) string {
	line, _, _ := strings.Cut(err.Error(), "\n")

	return line
}

// applyDiffEditorSelection writes the selection into the right tree. It reads a snapshot for the same
// reason runSquash does.
func (m *Model) applyDiffEditorSelection() tea.Cmd {
	source, changes, sel := m.diffSource, m.changes, m.selection.Clone()

	return func() tea.Msg {
		dirSource, ok := diff.DirectoryOf(source)
		if !ok {
			return writtenMsg{err: errNotDirectorySource}
		}

		applier := diff.NewApplier(dirSource.LeftPath, dirSource.RightPath)
		if err := applier.ApplySelections(changes, sel); err != nil {
			return writtenMsg{err: fmt.Errorf("failed to apply selections: %w", err)}
		}

		return diffEditorAppliedMsg{}
	}
}

// quitPromptHints is the question q asks over a pending selection.
func (m *Model) quitPromptHints() []keyhint.Hint {
	write := "write"
	if m.mode == ModeInteractive && m.destination == "" {
		write = "squash…"
	}

	return []keyhint.Hint{
		{Key: "w", What: write, Priority: 3}, //nolint:mnd // kept longest
		{Key: "d", What: "discard", Priority: 2},
		{Key: "esc", What: "stay", Priority: 1, Pin: true},
	}
}

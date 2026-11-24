package model

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/tui/keys"
	"github.com/kyleking/aragonite/tui/palette"

	"github.com/kyleking/jj-diff/internal/components/diffview"
	"github.com/kyleking/jj-diff/internal/search"
)

// handleKeyPress sends a key to whatever is taking text (a prompt, the search bar, a filter), else
// through the keymap to an action. The notice from the last action lasts until this next key.
func (m *Model) handleKeyPress(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()
	m.notice, m.alarm = "", false

	if !m.welcomeKey(key) {
		return nil
	}

	// Any key clears a tip on screen and restarts the coach's idle clock.
	m.coach.Press(time.Now())

	switch {
	case m.pal.IsOpen():
		return m.pal.Update(msg)
	case m.msgs != nil:
		return m.handleMessagesKey(key)
	case m.quitPrompt:
		return m.handleQuitPrompt(key)
	case m.cols != nil && m.cols.describing:
		return m.handleDescribeKeyPress(msg)
	case m.searchModal.IsVisible():
		return m.handleSearchKeyPress(msg)
	case m.fileList.IsFilterMode():
		return m.handleFileListFilterKeyPress(msg)
	case m.picker.IsFiltering():
		return m.handlePickerFilterKeyPress(msg)
	}

	match := m.matcher.Feed(m.context(), key)
	if match.Action == "" {
		return m.observe("")
	}

	return m.press(match.Action, match.Keys)
}

// press runs the action a key sequence completed, unless the gate refuses it here, in which case the
// footer says why rather than nothing happening. The usage log hears the keys, then the action.
func (m *Model) press(action string, pressed []string) tea.Cmd {
	ctx, seq := m.contextName(), strings.Join(pressed, "")
	m.usage.Key(ctx, seq)

	if m.refused(action) {
		return m.observe("")
	}

	cmd := m.dispatch(action, pressed...)
	m.usage.Action(ctx, action, seq)

	return tea.Batch(cmd, m.observe(action))
}

// discoverAction runs the actions every screen shares, false for any other: the legend, the
// command palette, and the command log.
func (m *Model) discoverAction(action string) bool {
	switch action {
	case keys.ActionHelp:
		m.openPalette(palette.Browse)
	case keys.ActionPalette:
		m.openPalette(palette.Run)
	case keys.ActionMessages:
		m.openMessages()
	default:
		return false
	}

	return true
}

// dispatch runs one action on the screen the user is on. The sequence that pressed it is pressed,
// which a digit action reads its number from.
func (m *Model) dispatch(action string, pressed ...string) tea.Cmd {
	if m.discoverAction(action) {
		return nil
	}

	if m.picker.IsVisible() {
		return m.pickerAction(action)
	}

	if m.mode == ModeResolve {
		return m.resolveAction(action, pressed)
	}

	if m.cols != nil && action != keys.ActionCancel {
		if cmd, ok := m.columnsAction(action, pressed); ok {
			return cmd
		}

		return nil
	}

	if step, ok := m.motion.Resolve(action); ok {
		return m.step(step)
	}

	if m.scrollAction(action) || m.viewAction(action) {
		return nil
	}

	return m.verb(action)
}

// verb runs the actions that are neither moving nor drawing: help, quitting, the selection, writes.
func (m *Model) verb(action string) tea.Cmd {
	switch action {
	case keys.ActionQuit:
		return m.quit()
	case keys.ActionCancel:
		return m.cancel()
	case keys.ActionBack:
		m.back()
	case keys.ActionReload:
		return m.loadDiff()
	case keys.ActionSwitchPane:
		m.toggleFocusedPanel()
	case actionOpen:
		m.focusedPanel = PanelDiffView
	case keys.ActionSearch:
		m.enterSearchMode()
	case keys.ActionFilter:
		m.focusedPanel = PanelFileList
		m.fileList.SetFilterMode(true)
	case keys.ActionUndo:
		m.undoSelection()
	case keys.ActionRedo:
		m.redoSelection()
	case keys.ActionRepeat:
		return m.repeat()
	default:
		return m.selectionVerb(action)
	}

	return nil
}

// selectionVerb runs the actions that change or write the selection.
func (m *Model) selectionVerb(action string) tea.Cmd {
	switch action {
	case actionSelect:
		m.toggleCurrentSelection()
	case keys.ActionVisual:
		m.enterVisualMode()
	case actionInvert:
		m.invertSelection()
	case actionSquash:
		return m.squash()
	case actionSplit:
		m.split()
	case actionRestore:
		return m.restore()
	case actionConfirm:
		return m.confirm()
	}

	return nil
}

// scrollAction moves through the focused pane without changing the diff or the selection.
func (m *Model) scrollAction(action string) bool {
	switch action {
	case keys.ActionLineDown:
		m.navigate(1)
	case keys.ActionLineUp:
		m.navigate(-1)
	case keys.ActionHalfPageDown:
		m.scrollDiffView(m.diffView.ScrollHalfPageDown)
	case keys.ActionHalfPageUp:
		m.scrollDiffView(m.diffView.ScrollHalfPageUp)
	case keys.ActionPageDown:
		m.scrollDiffView(m.diffView.ScrollFullPageDown)
	case keys.ActionPageUp:
		m.scrollDiffView(m.diffView.ScrollFullPageUp)
	case keys.ActionScrollDown:
		m.diffView.Scroll(1)
	case keys.ActionScrollUp:
		m.diffView.Scroll(-1)
	case keys.ActionTop:
		m.toEnd(false)
	case keys.ActionBottom:
		m.toEnd(true)
	default:
		return false
	}

	return true
}

// viewAction changes how the diff is drawn, which never touches the selection.
func (m *Model) viewAction(action string) bool {
	switch action {
	case keys.ActionWhitespace:
		m.diffView.ToggleWhitespace()
	case keys.ActionViewRefine:
		m.diffView.ToggleWordDiff()
	case keys.ActionViewSideBySide:
		m.diffView.ToggleSideBySide()
	case keys.ActionViewNumbers:
		m.diffView.ToggleLineNumbers()
	case keys.ActionFoldToggle:
		m.diffView.ToggleFold(m.selectedHunk)
	case keys.ActionFoldOpenAll:
		m.diffView.SetFolds(false)
	case keys.ActionFoldAll:
		m.diffView.SetFolds(true)
	case keys.ActionFoldInvert:
		m.diffView.InvertFolds()
	case keys.ActionFrameMiddle:
		m.frame(diffview.FrameMiddle)
	case keys.ActionFrameTop:
		m.frame(diffview.FrameTop)
	case keys.ActionFrameBottom:
		m.frame(diffview.FrameBottom)
	case keys.ActionOnlyUnread:
		m.toggleOnlyUnread()
	default:
		return false
	}

	return true
}

// frame puts the cursor's row (the line in visual mode, else the hunk's header) at pos.
func (m *Model) frame(pos diffview.FramePosition) {
	line := -1
	if m.isVisualMode {
		line = m.lineCursor
	}

	m.diffView.Frame(m.selectedHunk, line, pos, m.diffHeight())
}

// step runs a ] or [ motion, or its repeat on n and N.
func (m *Model) step(s keys.Step) tea.Cmd {
	delta := 1
	if s.Back {
		delta = -1
	}

	switch s.Object {
	case keys.ObjSearch:
		if !m.hasSearchMatches() {
			return m.step(keys.Step{Object: keys.ObjHunk, Back: s.Back})
		}

		if s.Back {
			return m.prevSearchMatch()
		}

		return m.nextSearchMatch()
	case keys.ObjFile:
		m.selectAdjacentFile(delta)
	case keys.ObjHunk:
		m.focusedPanel = PanelDiffView
		m.selectAdjacentHunk(delta)
	case keys.ObjUnread:
		m.stepUnread(s.Back)
	}

	return nil
}

func modeName(mode OperatingMode) string {
	switch mode {
	case ModeInteractive:
		return "Interactive"
	case ModeDiffEditor:
		return "Diff-Editor"
	case ModeResolve:
		return "Resolve"
	case ModeBrowse:
	}

	return "Browse"
}

// back backs out one level: visual mode, then the file list's focus back to the diff. A pending
// chord and the overlays are handled before an action ever runs.
func (m *Model) back() {
	switch {
	case m.isVisualMode:
		m.isVisualMode = false
		m.visualAnchor = 0
	case m.focusedPanel == PanelFileList && len(m.changes) > 0:
		m.focusedPanel = PanelDiffView
	}
}

// quit closes the program, asking first where leaving would throw away a selection.
func (m *Model) quit() tea.Cmd {
	switch m.mode {
	case ModeInteractive:
		if !m.selection.IsEmpty() {
			m.quitPrompt = true

			return nil
		}
	case ModeDiffEditor:
		if len(m.undo) > 0 {
			m.quitPrompt = true

			return nil
		}

		m.aborted = true
	case ModeResolve:
		if m.res != nil && len(m.res.undo) > 0 {
			m.quitPrompt = true

			return nil
		}
	case ModeBrowse:
	}

	return tea.Quit
}

// cancel is ctrl+c: quit now, writing nothing.
func (m *Model) cancel() tea.Cmd {
	m.aborted = m.mode == ModeDiffEditor

	return tea.Quit
}

// handleQuitPrompt answers write, discard, or stay.
func (m *Model) handleQuitPrompt(key string) tea.Cmd {
	switch key {
	case "w":
		m.quitPrompt = false
		m.quitAfterWrite = true

		return m.confirm()
	case "d":
		m.quitPrompt = false
		m.aborted = m.mode == ModeDiffEditor

		return tea.Quit
	case keyEsc, "q":
		m.quitPrompt = false
	case keyCtrlC:
		return m.cancel()
	}

	return nil
}

// selectionAllowed reports whether the mode and the focused panel let a key change the selection.
func (m *Model) selectionAllowed() bool {
	return m.mode != ModeBrowse && m.focusedPanel == PanelDiffView
}

// hasCurrentHunk reports whether the file and hunk cursors both point at something that exists.
func (m *Model) hasCurrentHunk() bool {
	if m.selectedFile < 0 || m.selectedFile >= len(m.changes) {
		return false
	}

	return m.selectedHunk >= 0 && m.selectedHunk < len(m.changes[m.selectedFile].Hunks)
}

func (m *Model) hasSearchMatches() bool {
	return m.searchState != nil && m.searchState.IsActive && len(m.searchState.Matches) > 0
}

func (m *Model) enterVisualMode() {
	if !m.selectionAllowed() || !m.hasCurrentHunk() {
		return
	}

	m.diffView.Reveal(m.selectedHunk)
	m.isVisualMode = true
	m.visualAnchor = m.lineCursor
}

// edit records the selection before change runs, so undo can return to it, and drops the redo
// history a new edit forks away from.
func (m *Model) edit(change func()) {
	m.undo = append(m.undo, m.selection.Clone())
	m.redo = nil

	change()
}

func (m *Model) undoSelection() {
	if len(m.undo) == 0 {
		m.say("nothing to undo", false)

		return
	}

	m.redo = append(m.redo, m.selection)
	m.selection = m.undo[len(m.undo)-1]
	m.undo = m.undo[:len(m.undo)-1]
}

func (m *Model) redoSelection() {
	if len(m.redo) == 0 {
		m.say("nothing to redo", false)

		return
	}

	m.undo = append(m.undo, m.selection)
	m.selection = m.redo[len(m.redo)-1]
	m.redo = m.redo[:len(m.redo)-1]
}

func (m *Model) toggleCurrentSelection() {
	if !m.selectionAllowed() || !m.hasCurrentHunk() {
		return
	}

	path := m.changes[m.selectedFile].Path

	m.edit(func() {
		if m.isVisualMode {
			m.selection.SelectLineRange(path, m.selectedHunk, m.visualAnchor, m.lineCursor)
			m.isVisualMode = false

			return
		}

		m.selection.ToggleHunk(path, m.selectedHunk)
	})
}

func (m *Model) invertSelection() {
	if !m.selectionAllowed() || m.selectedFile < 0 || m.selectedFile >= len(m.changes) {
		return
	}

	m.edit(func() { m.selection.Invert(m.changes[m.selectedFile]) })
}

func (m *Model) toggleFocusedPanel() {
	if m.focusedPanel == PanelFileList {
		m.focusedPanel = PanelDiffView
	} else {
		m.focusedPanel = PanelFileList
	}
}

// toEnd is gg and G: the first or last file in the list, the top or bottom of the diff in the pane.
func (m *Model) toEnd(bottom bool) {
	if len(m.changes) == 0 {
		return
	}

	if m.focusedPanel == PanelFileList {
		idx, delta := -1, 1
		if bottom {
			idx, delta = len(m.changes), -1
		}

		m.jumpToFile(m.shownFrom(idx, delta))

		return
	}

	hunks := len(m.changes[m.selectedFile].Hunks)
	m.selectedHunk, m.lineCursor = 0, 0

	if bottom {
		m.selectedHunk = max(hunks-1, 0)
		m.diffView.Frame(m.selectedHunk, -1, diffview.FrameBottom, m.diffHeight())
		m.diffView.Scroll(m.diffHeight())

		return
	}

	m.diffView.Frame(0, -1, diffview.FrameTop, m.diffHeight())
}

// jumpToFile moves the file cursor to an absolute index.
func (m *Model) jumpToFile(idx int) {
	if idx < 0 || idx >= len(m.changes) {
		return
	}

	m.selectedFile = idx
	m.selectedHunk = 0
	m.lineCursor = 0
	m.isVisualMode = false
	m.fileList.SetSelected(idx)
	m.diffView.SetFileChange(m.changes[idx])
}

// selectAdjacentFile steps the file cursor to the next file the list shows, stopping at both ends
// rather than wrapping.
func (m *Model) selectAdjacentFile(delta int) {
	m.jumpToFile(m.shownFrom(m.selectedFile, delta))
}

// shownFrom is the first file past from, stepping by delta, that the list shows, or -1 for none.
func (m *Model) shownFrom(from, delta int) int {
	for i := from + delta; i >= 0 && i < len(m.changes); i += delta {
		if m.fileShown(i) {
			return i
		}
	}

	return -1
}

// selectAdjacentHunk steps the hunk cursor within the selected file, wrapping at both ends, and
// scrolls the hunk into view.
func (m *Model) selectAdjacentHunk(delta int) {
	if m.selectedFile < 0 || m.selectedFile >= len(m.changes) {
		return
	}

	hunkCount := len(m.changes[m.selectedFile].Hunks)
	if hunkCount == 0 {
		return
	}

	m.selectedHunk = ((m.selectedHunk+delta)%hunkCount + hunkCount) % hunkCount
	m.lineCursor = 0
	m.isVisualMode = false
	m.diffView.Show(m.selectedHunk, m.diffHeight())
}

func (m *Model) navigate(delta int) {
	switch {
	case m.isVisualMode:
		m.handleVisualNavigation(delta)
	case m.focusedPanel == PanelFileList:
		m.selectAdjacentFile(delta)
	default:
		m.diffView.Scroll(delta)
	}
}

func (m *Model) scrollDiffView(scroll func(viewHeight int)) {
	if m.focusedPanel == PanelDiffView {
		scroll(m.diffHeight())
	}
}

func (m *Model) handleVisualNavigation(delta int) {
	if !m.hasCurrentHunk() {
		return
	}

	hunk := m.changes[m.selectedFile].Hunks[m.selectedHunk]
	if next := m.lineCursor + delta; next >= 0 && next < len(hunk.Lines) {
		m.lineCursor = next
	}
}

func (m *Model) closeAllModals() {
	m.pal.Close()
	m.msgs = nil
	m.picker.Hide()
	m.searchModal.Hide()
	m.fileList.SetFilterMode(false)
	m.quitPrompt = false
}

func (m *Model) enterSearchMode() {
	m.closeAllModals()
	m.searchState.SaveOriginalState(search.NavigationState{
		SelectedFile: m.selectedFile,
		SelectedHunk: m.selectedHunk,
		FocusedPanel: int(m.focusedPanel),
	})
	m.searchModal.Show()
}

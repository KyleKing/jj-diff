// Package model is the Bubble Tea root: it owns the diff, the selection, and every child component,
// and routes each key to the handler for the current mode.
package model

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kyleking/jj-diff/internal/components/commitmsg"
	"github.com/kyleking/jj-diff/internal/components/destpicker"
	"github.com/kyleking/jj-diff/internal/components/diffview"
	"github.com/kyleking/jj-diff/internal/components/filefinder"
	"github.com/kyleking/jj-diff/internal/components/filelist"
	"github.com/kyleking/jj-diff/internal/components/help"
	"github.com/kyleking/jj-diff/internal/components/searchmodal"
	"github.com/kyleking/jj-diff/internal/components/splitassign"
	"github.com/kyleking/jj-diff/internal/components/splitpreview"
	"github.com/kyleking/jj-diff/internal/components/statusbar"
	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/search"
	"github.com/kyleking/jj-diff/internal/theme"
)

// OperatingMode is which of the three entry points the app was started for, which decides what the
// selection means and what applying it does.
type OperatingMode int

// The three ways the app is launched. ModeBrowse is read-only, ModeInteractive moves selected hunks
// into another revision, and ModeDiffEditor writes the selection back into the trees jj passed.
const (
	ModeBrowse OperatingMode = iota
	ModeInteractive
	ModeDiffEditor
)

// FocusedPanel is which of the two panes takes navigation keys.
type FocusedPanel int

// The two panes Tab switches between. PanelFileList is the zero value, so a fresh model starts on the
// file list.
const (
	PanelFileList FocusedPanel = iota
	PanelDiffView
)

// HunkSelection is one hunk's selection. WholeHunk wins over SelectedLines, and selecting the whole
// hunk discards the per-line set, so the two are never both meaningful.
type HunkSelection struct {
	SelectedLines map[int]bool
	WholeHunk     bool
}

// FileSelection holds one file's selected hunks, keyed by the hunk's index in the parsed file. The
// indices go stale when the diff is reloaded.
type FileSelection struct {
	Hunks map[int]*HunkSelection
}

// SelectionState is what the user has picked across every file, keyed by the diff's path. Build it
// with NewSelectionState, because the mutators assume the map exists.
type SelectionState struct {
	Files map[string]*FileSelection
}

// NewSelectionState returns an empty selection.
func NewSelectionState() *SelectionState {
	return &SelectionState{
		Files: make(map[string]*FileSelection),
	}
}

// SplitTag is the single character a hunk carries while a multi-way split is being assembled. Tags
// are handed out from 'A' upward.
type SplitTag rune

// DestinationType separates a split target that already exists from one that will be created.
type DestinationType int

// Destinations a tag can be sent to. DestExistingRevision is the zero value, so a DestinationSpec
// that was never filled in reads as targeting an existing revision.
const (
	DestExistingRevision DestinationType = iota
	DestNewCommit
)

// Terminal geometry the model assumes until Bubble Tea reports the real size.
const (
	defaultTerminalWidth  = 80
	defaultTerminalHeight = 24
)

// Layout arithmetic for the vertical split between the file list and the diff view.
const (
	// Border plus status bar, neither of which scrolls.
	chromeHeight = 2
	// The focused file list takes a quarter of the terminal.
	expandedFileListFraction  = 4
	minExpandedFileListHeight = 5
	collapsedFileListHeight   = 1
)

// revisionListLimit caps how far back the destination pickers look, because the log call is
// synchronous and the pickers are scrollable anyway.
const revisionListLimit = 20

// Key names the handlers branch on in more than one place.
const (
	keyBackspace = "backspace"
	keyCtrlC     = "ctrl+c"
	keyDown      = "down"
	keyEnter     = "enter"
)

// Sentinel errors the apply paths return when the model's own state, rather than jj or the
// filesystem, is what blocks the operation.
var (
	errNoDestinationsAssigned = errors.New("no destinations assigned")
	errNoSelection            = errors.New("no hunks or lines selected")
	errNoSplitPlans           = errors.New("no valid split plans generated")
	errNotDirectorySource     = errors.New("diff-editor mode requires a directory source")
)

// DestinationSpec is where one tag's hunks land. ChangeID is empty for DestNewCommit, where
// Description becomes the message of the commit that gets created.
type DestinationSpec struct {
	ChangeID    string
	Description string
	Type        DestinationType
}

// MultiSplitState is an in-progress multi-way split: one selection and one destination per tag. A tag
// with a selection but no destination is incomplete and blocks the split from being applied.
type MultiSplitState struct {
	Selections   map[SplitTag]*SelectionState
	Destinations map[SplitTag]*DestinationSpec
	CurrentTag   SplitTag
	Active       bool
}

// NewMultiSplitState returns an inactive split with 'A' as the current tag.
func NewMultiSplitState() *MultiSplitState {
	return &MultiSplitState{
		Active:       false,
		Selections:   make(map[SplitTag]*SelectionState),
		Destinations: make(map[SplitTag]*DestinationSpec),
		CurrentTag:   'A',
	}
}

// IsHunkSelected reports whether the whole hunk is selected, which is false for a hunk that only has
// individual lines picked.
func (s *SelectionState) IsHunkSelected(filePath string, hunkIdx int) bool {
	if fileSelection, ok := s.Files[filePath]; ok {
		if hunkSelection, ok := fileSelection.Hunks[hunkIdx]; ok {
			return hunkSelection.WholeHunk
		}
	}

	return false
}

// IsLineSelected reports whether one line is selected, which is true for every line of a hunk
// selected as a whole.
func (s *SelectionState) IsLineSelected(filePath string, hunkIdx, lineIdx int) bool {
	if fileSelection, ok := s.Files[filePath]; ok {
		if hunkSelection, ok := fileSelection.Hunks[hunkIdx]; ok {
			if hunkSelection.WholeHunk {
				return true
			}

			return hunkSelection.SelectedLines[lineIdx]
		}
	}

	return false
}

// ToggleHunk flips whole-hunk selection, creating the file and hunk entries as needed. Selecting a
// hunk discards any lines picked inside it, so a toggle out and back in loses the line selection.
func (s *SelectionState) ToggleHunk(filePath string, hunkIdx int) {
	if _, ok := s.Files[filePath]; !ok {
		s.Files[filePath] = &FileSelection{
			Hunks: make(map[int]*HunkSelection),
		}
	}

	fileSelection := s.Files[filePath]
	if _, ok := fileSelection.Hunks[hunkIdx]; !ok {
		fileSelection.Hunks[hunkIdx] = &HunkSelection{
			SelectedLines: make(map[int]bool),
		}
	}

	hunkSelection := fileSelection.Hunks[hunkIdx]
	hunkSelection.WholeHunk = !hunkSelection.WholeHunk
	if hunkSelection.WholeHunk {
		hunkSelection.SelectedLines = make(map[int]bool)
	}
}

// ToggleLine flips one line's selection. It does nothing while the hunk is selected as a whole,
// because that state has no per-line detail to change.
func (s *SelectionState) ToggleLine(filePath string, hunkIdx, lineIdx int) {
	if _, ok := s.Files[filePath]; !ok {
		s.Files[filePath] = &FileSelection{
			Hunks: make(map[int]*HunkSelection),
		}
	}

	fileSelection := s.Files[filePath]
	if _, ok := fileSelection.Hunks[hunkIdx]; !ok {
		fileSelection.Hunks[hunkIdx] = &HunkSelection{
			SelectedLines: make(map[int]bool),
		}
	}

	hunkSelection := fileSelection.Hunks[hunkIdx]
	if hunkSelection.WholeHunk {
		return
	}

	hunkSelection.SelectedLines[lineIdx] = !hunkSelection.SelectedLines[lineIdx]
}

// SelectLineRange selects an inclusive range of lines, accepting the bounds in either order. It
// clears whole-hunk selection, and it only adds, so lines already selected outside the range stay.
func (s *SelectionState) SelectLineRange(filePath string, hunkIdx, startLine, endLine int) {
	if startLine > endLine {
		startLine, endLine = endLine, startLine
	}

	if _, ok := s.Files[filePath]; !ok {
		s.Files[filePath] = &FileSelection{
			Hunks: make(map[int]*HunkSelection),
		}
	}

	fileSelection := s.Files[filePath]
	if _, ok := fileSelection.Hunks[hunkIdx]; !ok {
		fileSelection.Hunks[hunkIdx] = &HunkSelection{
			SelectedLines: make(map[int]bool),
		}
	}

	hunkSelection := fileSelection.Hunks[hunkIdx]
	hunkSelection.WholeHunk = false

	for i := startLine; i <= endLine; i++ {
		hunkSelection.SelectedLines[i] = true
	}
}

// HasPartialSelection reports whether a hunk has lines picked without being selected as a whole,
// which is what the renderer draws the partial marker for.
func (s *SelectionState) HasPartialSelection(filePath string, hunkIdx int) bool {
	if fileSelection, ok := s.Files[filePath]; ok {
		if hunkSelection, ok := fileSelection.Hunks[hunkIdx]; ok {
			return !hunkSelection.WholeHunk && len(hunkSelection.SelectedLines) > 0
		}
	}

	return false
}

// Model is the whole application state. Bubble Tea passes it by value, so Update returns the updated
// copy and mutating a Model a handler received has no effect unless that copy is returned.
type Model struct {
	statusBar       statusbar.Model
	diffSource      diff.Source
	err             error
	selection       *SelectionState
	searchState     *search.State
	multiSplitState *MultiSplitState
	client          *jj.Client
	destination     string
	source          string
	changes         []diff.FileChange
	commitMsg       commitmsg.Model
	help            help.Model
	cfg             config.Config
	splitPreview    splitpreview.Model
	fileFinder      filefinder.Model
	destPicker      destpicker.Model
	searchModal     searchmodal.Model
	splitAssign     splitassign.Model
	fileList        filelist.Model
	diffView        diffview.Model
	focusedPanel    FocusedPanel
	lineCursor      int
	selectedHunk    int
	selectedFile    int
	visualAnchor    int
	width           int
	height          int
	mode            OperatingMode
	isVisualMode    bool
}

type errMsg struct {
	err error
}

type diffLoadedMsg struct {
	changes []diff.FileChange
}

type revisionsLoadedMsg struct {
	revisions []jj.RevisionEntry
}

type destinationSelectedMsg struct {
	changeID string
}

type splitAppliedMsg struct{}

// NewModel builds a model reading its diff from a jj revision.
func NewModel(
	client *jj.Client,
	source, destination string,
	mode OperatingMode,
	cfg config.Config,
) (Model, error) {
	revSource := diff.NewRevisionSource(client, source)
	return NewModelWithSource(revSource, client, destination, mode, cfg)
}

// NewModelWithSource builds a model over any diff source, which is how diff-editor mode supplies two
// directories instead of a revision. The client is still needed for the write paths and may only be
// nil when nothing will be applied.
func NewModelWithSource(
	source diff.Source,
	client *jj.Client,
	destination string,
	mode OperatingMode,
	cfg config.Config,
) (Model, error) {
	m := Model{
		client:          client,
		diffSource:      source,
		mode:            mode,
		source:          source.GetSourceLabel(),
		destination:     destination,
		cfg:             cfg,
		selectedFile:    0,
		selectedHunk:    0,
		focusedPanel:    PanelFileList,
		width:           defaultTerminalWidth,
		height:          defaultTerminalHeight,
		selection:       NewSelectionState(),
		multiSplitState: NewMultiSplitState(),
	}

	m.fileList = filelist.New()
	m.diffView = diffview.New(cfg)
	m.statusBar = statusbar.New()
	m.destPicker = destpicker.New()
	m.splitAssign = splitassign.New()
	m.splitPreview = splitpreview.New()
	m.commitMsg = commitmsg.New()
	m.help = help.New()
	m.searchModal = searchmodal.New()
	m.searchState = search.NewState()
	m.fileFinder = filefinder.New()

	return m, nil
}

// Init starts the first diff load. Nothing is rendered until it returns and a window size arrives.
func (m Model) Init() tea.Cmd {
	return m.loadDiff()
}

func (m Model) loadDiff() tea.Cmd {
	return func() tea.Msg {
		diffText, err := m.diffSource.GetDiff()
		if err != nil {
			return errMsg{err}
		}

		changes := diff.Parse(diffText)

		return diffLoadedMsg{changes}
	}
}

func (m Model) loadRevisions() tea.Cmd {
	return func() tea.Msg {
		revisions, err := m.client.GetRevisions(revisionListLimit)
		if err != nil {
			return errMsg{err}
		}

		return revisionsLoadedMsg{revisions}
	}
}

// Update handles one message and returns the model to use next. The concrete type is always Model, so
// callers chaining updates can assert it.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		return m, nil

	case diffLoadedMsg:
		m.changes = msg.changes
		m.fileList.SetFiles(m.changes)
		if len(m.changes) > 0 {
			m.diffView.SetFileChange(m.changes[0])
		}

		// jj's diff editor contract is subtractive: the right side starts as the
		// commit's full content and the user removes what should not be kept.
		// Starting empty here would discard every change on apply.
		if m.mode == ModeDiffEditor {
			m.selection = NewSelectionState()
			diff.SelectAll(m.changes, m.selection)
		}

		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case revisionsLoadedMsg:
		m.closeAllModals()
		m.destPicker.SetRevisions(msg.revisions)
		m.destPicker.Show()

		return m, nil

	case destinationSelectedMsg:
		m.destination = msg.changeID
		m.destPicker.Hide()

		return m, m.loadDiff()

	case splitAppliedMsg:
		m.multiSplitState = NewMultiSplitState()
		m.splitPreview.Hide()

		return m, m.loadDiff()

	case diffEditorAppliedMsg:
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	if key == "esc" {
		return m.handleEscape()
	}

	if key == "?" && !m.destPicker.IsVisible() {
		return m.toggleHelp()
	}

	if m.help.IsVisible() {
		if key == "q" {
			m.help.Hide()
		}

		return m, nil
	}

	if model, cmd, handled := m.routeToOverlay(msg); handled {
		return model, cmd
	}

	return m.handleAppKey(key)
}

// handleAppKey handles a key no overlay claimed, so it acts on the two panels or on the mode. Keys
// that fall through the switch are offered to the multi-way split as tag letters.
func (m *Model) handleAppKey(key string) (Model, tea.Cmd) {
	if model, cmd, handled := m.handleNavigationKey(key); handled {
		return model, cmd
	}

	if model, handled := m.handleViewOptionKey(key); handled {
		return model, nil
	}

	if model, cmd, handled := m.handleActionKey(key); handled {
		return model, cmd
	}

	return m.handleTagKey(key)
}

// handleScrollKey scrolls the diff view by a page or half a page. The scroll functions are method
// values bound to the model's own diff view, so the movement lands on the model this call returns.
func (m *Model) handleScrollKey(key string) (Model, bool) {
	switch key {
	case "ctrl+d":
		return m.scrollDiffView(m.diffView.ScrollHalfPageDown), true
	case "ctrl+u":
		return m.scrollDiffView(m.diffView.ScrollHalfPageUp), true
	case "ctrl+f":
		return m.scrollDiffView(m.diffView.ScrollFullPageDown), true
	case "ctrl+b":
		return m.scrollDiffView(m.diffView.ScrollFullPageUp), true
	}

	return *m, false
}

// handleNavigationKey moves a cursor or scrolls a panel without changing the diff or the selection.
func (m *Model) handleNavigationKey(key string) (Model, tea.Cmd, bool) {
	if model, handled := m.handleScrollKey(key); handled {
		return model, nil, true
	}

	var (
		model Model
		cmd   tea.Cmd
	)

	switch key {
	case "tab":
		model = m.toggleFocusedPanel()
	case "[":
		model = m.selectAdjacentFile(-1)
	case "]":
		model = m.selectAdjacentFile(1)
	case "j", keyDown:
		model, cmd = m.navigate(1)
	case "k", "up":
		model, cmd = m.navigate(-1)
	case "g":
		model = m.jumpToFile(0)
	case "G":
		model = m.jumpToFile(len(m.changes) - 1)
	case "n":
		model, cmd = m.nextMatchOrHunk()
	case "N":
		model, cmd = m.prevMatchOrHunk()
	case "p":
		model, cmd = m.selectAdjacentHunk(-1)
	default:
		return *m, nil, false
	}

	return model, cmd, true
}

// handleViewOptionKey toggles how the diff is rendered, which never touches the selection.
func (m *Model) handleViewOptionKey(key string) (Model, bool) {
	switch key {
	case "w":
		m.diffView.ToggleWhitespace()
	case "W":
		m.diffView.ToggleWordDiff()
	case "s":
		m.diffView.ToggleSideBySide()
	case "l":
		m.diffView.ToggleLineNumbers()
	default:
		return *m, false
	}

	return *m, true
}

// handleActionKey opens an overlay, changes the selection, or applies it.
func (m *Model) handleActionKey(key string) (Model, tea.Cmd, bool) {
	var (
		model Model
		cmd   tea.Cmd
	)

	switch key {
	case "q", keyCtrlC:
		return *m, tea.Quit, true
	case "d":
		model, cmd = m.openDestinationPicker()
	case "/":
		m.closeAllModals()
		model, cmd = m.enterSearchMode()
	case "f":
		m.closeAllModals()
		m.focusedPanel = PanelFileList
		m.fileList.SetFilterMode(true)

		model = *m
	case "F":
		m.closeAllModals()
		model = m.openFileFinder()
	case "v":
		model = m.enterVisualMode()
	case "r":
		return *m, m.loadDiff(), true
	case " ":
		model = m.toggleCurrentSelection()
	case "a":
		model, cmd = m.applyCurrentMode()
	case "S":
		model = m.toggleMultiSplit()
	case "D":
		model, cmd = m.openSplitAssign()
	case "P":
		model = m.openSplitPreview()
	default:
		return *m, nil, false
	}

	return model, cmd, true
}

// selectionAllowed reports whether the mode and the focused panel let a key change the selection.
func (m *Model) selectionAllowed() bool {
	return (m.mode == ModeInteractive || m.mode == ModeDiffEditor) &&
		m.focusedPanel == PanelDiffView
}

// hasCurrentHunk reports whether the file and hunk cursors both point at something that exists.
func (m *Model) hasCurrentHunk() bool {
	if m.selectedFile < 0 || m.selectedFile >= len(m.changes) {
		return false
	}

	hunks := m.changes[m.selectedFile].Hunks

	return m.selectedHunk >= 0 && m.selectedHunk < len(hunks)
}

func (m *Model) hasSearchMatches() bool {
	return m.searchState != nil && m.searchState.IsActive && len(m.searchState.Matches) > 0
}

func (m *Model) openDestinationPicker() (Model, tea.Cmd) {
	if m.mode == ModeInteractive && m.diffSource.SupportsRevisions() {
		return *m, m.loadRevisions()
	}

	return *m, nil
}

func (m *Model) enterVisualMode() Model {
	if !m.selectionAllowed() || !m.hasCurrentHunk() {
		return *m
	}

	m.isVisualMode = true
	m.visualAnchor = m.lineCursor

	return *m
}

func (m *Model) toggleCurrentSelection() Model {
	if !m.selectionAllowed() || !m.hasCurrentHunk() {
		return *m
	}

	if m.isVisualMode {
		m.toggleVisualSelection()
		m.isVisualMode = false
	} else {
		m.selection.ToggleHunk(m.changes[m.selectedFile].Path, m.selectedHunk)
	}

	return *m
}

func (m *Model) toggleFocusedPanel() Model {
	if m.focusedPanel == PanelFileList {
		m.focusedPanel = PanelDiffView
	} else {
		m.focusedPanel = PanelFileList
	}

	return *m
}

// jumpToFile moves the file cursor to an absolute index, which is how g and G reach the ends of the
// list. An empty change list leaves the diff view showing whatever it had.
func (m *Model) jumpToFile(idx int) Model {
	m.selectedFile = idx
	m.selectedHunk = 0
	m.lineCursor = 0
	m.fileList.SetSelected(idx)

	if len(m.changes) > 0 {
		m.diffView.SetFileChange(m.changes[idx])
	}

	return *m
}

// selectAdjacentFile steps the file cursor from the diff view, stopping at both ends rather than
// wrapping, and resets the hunk and line cursors with it.
func (m *Model) selectAdjacentFile(delta int) Model {
	if m.focusedPanel != PanelDiffView {
		return *m
	}

	next := m.selectedFile + delta
	if next >= 0 && next < len(m.changes) {
		m.selectedFile = next
		m.selectedHunk = 0
		m.lineCursor = 0
		m.fileList.SetSelected(next)
		m.diffView.SetFileChange(m.changes[next])
	}

	return *m
}

// selectAdjacentHunk steps the hunk cursor within the selected file, wrapping at both ends.
func (m *Model) selectAdjacentHunk(delta int) (Model, tea.Cmd) {
	if m.focusedPanel != PanelDiffView || m.selectedFile < 0 ||
		m.selectedFile >= len(m.changes) {
		return *m, nil
	}

	hunkCount := len(m.changes[m.selectedFile].Hunks)
	if hunkCount == 0 {
		return *m, nil
	}

	m.selectedHunk += delta
	if m.selectedHunk >= hunkCount {
		m.selectedHunk = 0
	}

	if m.selectedHunk < 0 {
		m.selectedHunk = hunkCount - 1
	}

	m.lineCursor = 0

	return *m, nil
}

func (m *Model) navigate(delta int) (Model, tea.Cmd) {
	if m.isVisualMode {
		return m.handleVisualNavigation(delta)
	}

	return m.handleNavigation(delta)
}

func (m *Model) scrollDiffView(scroll func(viewHeight int)) Model {
	if m.focusedPanel == PanelDiffView {
		scroll(m.height - chromeHeight)
	}

	return *m
}

// nextMatchOrHunk sends n to the search results while a search is live, and to the hunk cursor
// otherwise.
func (m *Model) nextMatchOrHunk() (Model, tea.Cmd) {
	if m.hasSearchMatches() {
		return m.nextSearchMatch()
	}

	return m.selectAdjacentHunk(1)
}

// prevMatchOrHunk is the N counterpart to nextMatchOrHunk.
func (m *Model) prevMatchOrHunk() (Model, tea.Cmd) {
	if m.hasSearchMatches() {
		return m.prevSearchMatch()
	}

	return m.selectAdjacentHunk(-1)
}

func (m *Model) applyCurrentMode() (Model, tea.Cmd) {
	if m.mode == ModeInteractive && m.destination != "" {
		return *m, m.applySelection()
	}

	if m.mode == ModeDiffEditor {
		return *m, m.applyDiffEditorSelection()
	}

	return *m, nil
}

func (m *Model) toggleMultiSplit() Model {
	if m.mode != ModeInteractive || m.focusedPanel != PanelDiffView {
		return *m
	}

	m.multiSplitState.Active = !m.multiSplitState.Active
	if m.multiSplitState.Active {
		m.multiSplitState.CurrentTag = 'A'
	}

	return *m
}

func (m *Model) openSplitAssign() (Model, tea.Cmd) {
	if m.mode != ModeInteractive || !m.multiSplitState.Active {
		return *m, nil
	}

	tags := make([]splitassign.SplitTag, 0, len(m.multiSplitState.Selections))
	for tag := range m.multiSplitState.Selections {
		tags = append(tags, splitassign.SplitTag(tag))
	}

	if len(tags) == 0 {
		return *m, nil
	}

	m.splitAssign.SetTags(tags)

	return *m, m.loadRevisionsForSplitAssign()
}

func (m *Model) openSplitPreview() Model {
	if m.mode != ModeInteractive || !m.multiSplitState.Active {
		return *m
	}

	destinations := m.splitAssign.GetDestinations()
	if len(destinations) > 0 {
		m.splitPreview.SetSummaries(m.buildSplitSummaries(destinations))
		m.splitPreview.Show()
	}

	return *m
}

// openFileFinder shows the fuzzy file picker over the current change list.
func (m *Model) openFileFinder() Model {
	items := make([]string, len(m.changes))
	data := make([]any, len(m.changes))
	for i, file := range m.changes {
		items[i] = file.Path
		data[i] = i
	}

	m.fileFinder.Show(items, data)

	return *m
}

// handleTagKey claims a letter key for the multi-way split, which is why the tag letters are not
// listed as cases: they only mean anything while a split is being assembled.
func (m *Model) handleTagKey(key string) (Model, tea.Cmd) {
	if !m.multiSplitState.Active || m.mode != ModeInteractive ||
		m.focusedPanel != PanelDiffView || len(key) != 1 {
		return *m, nil
	}

	tag, ok := splitTagFromKey(key[0])
	if !ok {
		return *m, nil
	}

	return m.toggleTagSelection(tag)
}

// splitTagFromKey maps a letter to its tag, folding lowercase onto the uppercase tag it shares.
func splitTagFromKey(char byte) (SplitTag, bool) {
	switch {
	case char >= 'a' && char <= 'z':
		return SplitTag(char - 'a' + 'A'), true
	case char >= 'A' && char <= 'Z':
		return SplitTag(char), true
	}

	return 0, false
}

func (m Model) applySelection() tea.Cmd {
	return func() tea.Msg {
		hasSelection := false
		for _, file := range m.changes {
			for hunkIdx := range file.Hunks {
				if m.selection.IsHunkSelected(file.Path, hunkIdx) ||
					m.selection.HasPartialSelection(file.Path, hunkIdx) {
					hasSelection = true
					break
				}
			}
			if hasSelection {
				break
			}
		}

		if !hasSelection {
			return errMsg{errNoSelection}
		}

		patch := diff.GeneratePatch(m.changes, m.selection)

		err := m.client.MoveChanges(patch, m.source, m.destination)
		if err != nil {
			return errMsg{fmt.Errorf("failed to move changes: %w", err)}
		}

		// Inside a tea.Cmd the caller expects a tea.Msg, so the reload command has
		// to be run rather than handed back as a message Update cannot match.
		return m.loadDiff()()
	}
}

type diffEditorAppliedMsg struct{}

func (m Model) applyDiffEditorSelection() tea.Cmd {
	return func() tea.Msg {
		dirSource, ok := m.diffSource.(*diff.DirectorySource)
		if !ok {
			return errMsg{errNotDirectorySource}
		}

		applier := diff.NewApplier(dirSource.LeftPath, dirSource.RightPath)
		if err := applier.ApplySelections(m.changes, m.selection); err != nil {
			return errMsg{fmt.Errorf("failed to apply selections: %w", err)}
		}

		return diffEditorAppliedMsg{}
	}
}

// handleEscape closes the topmost overlay, falling back to leaving visual mode and then to doing
// nothing, so esc is always safe to press. The order below is the stacking order on screen.
func (m Model) handleEscape() (Model, tea.Cmd) {
	switch {
	case m.help.IsVisible():
		m.help.Hide()
	case m.destPicker.IsVisible():
		m.destPicker.Hide()
	case m.splitAssign.IsVisible():
		m.splitAssign.Hide()
	case m.splitPreview.IsVisible():
		m.splitPreview.Hide()
	case m.commitMsg.IsVisible():
		m.commitMsg.Hide()
	case m.searchModal.IsVisible():
		if m.searchState != nil {
			origState := m.searchState.RestoreOriginalState()
			m.selectedFile = origState.SelectedFile
			m.selectedHunk = origState.SelectedHunk
			m.focusedPanel = FocusedPanel(origState.FocusedPanel)
		}
		m.searchModal.Hide()
		m.searchState.IsActive = false
	case m.fileFinder.IsVisible():
		m.fileFinder.Hide()
	case m.fileList.IsFilterMode():
		m.fileList.SetFilterMode(false)
	case m.isVisualMode:
		m.isVisualMode = false
		m.visualAnchor = 0
	}

	return m, nil
}

// toggleHelp shows the help overlay labeled for the current mode, or hides it when it is already
// up. Every other overlay is closed first, because help renders over the whole screen.
func (m Model) toggleHelp() (Model, tea.Cmd) {
	if m.help.IsVisible() {
		m.help.Hide()

		return m, nil
	}

	m.closeAllModals()

	modeText := "Browse"

	switch m.mode {
	case ModeInteractive:
		modeText = "Interactive"
	case ModeDiffEditor:
		modeText = "Diff-Editor"
	case ModeBrowse:
	}

	m.help.Show(modeText)

	return m, nil
}

// routeToOverlay hands the key to whichever overlay is visible. The third result reports whether one
// took the key, because an overlay handler may legitimately return a nil command.
func (m Model) routeToOverlay(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	var (
		model Model
		cmd   tea.Cmd
	)

	switch {
	case m.destPicker.IsVisible():
		model, cmd = m.handleDestPickerKeyPress(msg)
	case m.splitAssign.IsVisible():
		model, cmd = m.handleSplitAssignKeyPress(msg)
	case m.splitPreview.IsVisible():
		model, cmd = m.handleSplitPreviewKeyPress(msg)
	case m.commitMsg.IsVisible():
		model, cmd = m.handleCommitMsgKeyPress(msg)
	case m.searchModal.IsVisible():
		model, cmd = m.handleSearchKeyPress(msg)
	case m.fileFinder.IsVisible():
		model, cmd = m.handleFileFinderKeyPress(msg)
	case m.fileList.IsFilterMode():
		model, cmd = m.handleFileListFilterKeyPress(msg)
	default:
		return m, nil, false
	}

	return model, cmd, true
}

func (m Model) handleDestPickerKeyPress(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		m.destPicker.Hide()
		return m, nil

	case "j", keyDown:
		m.destPicker.MoveDown()
		return m, nil

	case "k", "up":
		m.destPicker.MoveUp()
		return m, nil

	case keyEnter:
		if selected := m.destPicker.GetSelected(); selected != nil {
			return m, func() tea.Msg {
				return destinationSelectedMsg{changeID: selected.ChangeID}
			}
		}

		return m, nil
	}

	return m, nil
}

//nolint:unparam // tea.Cmd stays in the signature to match the sibling handlers routeToOverlay dispatches through.
func (m Model) handleSplitAssignKeyPress(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		m.splitAssign.Hide()
		return m, nil

	case "j", keyDown:
		m.splitAssign.MoveDown()
		return m, nil

	case "k", "up":
		m.splitAssign.MoveUp()
		return m, nil

	case "tab":
		m.splitAssign.ToggleFocus()
		return m, nil

	case keyEnter:
		m.splitAssign.AssignRevisionToCurrentTag()
		return m, nil

	case "N":
		m.commitMsg.SetTag(commitmsg.SplitTag(m.multiSplitState.CurrentTag))
		m.splitAssign.Hide()
		m.commitMsg.Show()

		return m, nil
	}

	return m, nil
}

func (m Model) handleSplitPreviewKeyPress(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		m.splitPreview.Hide()
		return m, nil

	case "e":
		m.splitPreview.Hide()
		return m, m.loadRevisionsForSplitAssign()

	case keyEnter:
		return m, m.applySplit()
	}

	return m, nil
}

//nolint:unparam // tea.Cmd stays in the signature to match the sibling handlers routeToOverlay dispatches through.
func (m Model) handleCommitMsgKeyPress(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		m.commitMsg.Hide()
		m.splitAssign.Show()

		return m, nil

	case keyEnter:
		message := m.commitMsg.GetMessage()
		if message != "" {
			tag := m.commitMsg.GetTag()
			m.splitAssign.AssignNewCommitToTag(splitassign.SplitTag(tag), message)
		}
		m.commitMsg.Hide()
		m.splitAssign.Show()

		return m, nil

	case keyBackspace:
		m.commitMsg.Backspace()
		return m, nil

	default:
		if len(msg.String()) == 1 {
			m.commitMsg.AppendChar(rune(msg.String()[0]))
		}

		return m, nil
	}
}

func (m Model) loadRevisionsForSplitAssign() tea.Cmd {
	return func() tea.Msg {
		revisions, err := m.client.GetRevisions(revisionListLimit)
		if err != nil {
			return errMsg{err}
		}
		m.closeAllModals()
		m.splitAssign.SetRevisions(revisions)
		m.splitAssign.Show()

		return nil
	}
}

func (m Model) buildSplitSummaries(
	destinations map[splitassign.SplitTag]*splitassign.DestinationSpec,
) []splitpreview.SplitSummary {
	var summaries []splitpreview.SplitSummary

	for tag, dest := range destinations {
		tagSelection := m.multiSplitState.Selections[SplitTag(tag)]
		if tagSelection == nil {
			continue
		}

		fileCount := 0
		hunkCount := 0
		for _, file := range m.changes {
			fileHasSelection := false
			for hunkIdx := range file.Hunks {
				if tagSelection.IsHunkSelected(file.Path, hunkIdx) ||
					tagSelection.HasPartialSelection(file.Path, hunkIdx) {
					hunkCount++
					fileHasSelection = true
				}
			}
			if fileHasSelection {
				fileCount++
			}
		}

		summary := splitpreview.SplitSummary{
			Tag: splitpreview.SplitTag(tag),
			Destination: splitpreview.DestinationSpec{
				Type:        splitpreview.DestinationType(dest.Type),
				ChangeID:    dest.ChangeID,
				Description: dest.Description,
			},
			FileCount: fileCount,
			HunkCount: hunkCount,
		}
		summaries = append(summaries, summary)
	}

	return summaries
}

func (m Model) applySplit() tea.Cmd {
	return func() tea.Msg {
		destinations := m.splitAssign.GetDestinations()
		if len(destinations) == 0 {
			return errMsg{errNoDestinationsAssigned}
		}

		var plans []jj.SplitPlan
		for tag, dest := range destinations {
			tagSelection := m.multiSplitState.Selections[SplitTag(tag)]
			if tagSelection == nil {
				continue
			}

			patch := diff.GeneratePatchForTag(m.changes, tagSelection)
			if patch == "" {
				continue
			}

			jjDest := jj.SplitDestination{
				Type:        jj.SplitDestinationType(dest.Type),
				ChangeID:    dest.ChangeID,
				Description: dest.Description,
			}

			plans = append(plans, jj.SplitPlan{
				Tag:         rune(tag),
				Patch:       patch,
				Destination: jjDest,
			})
		}

		if len(plans) == 0 {
			return errMsg{errNoSplitPlans}
		}

		if err := m.client.ApplySplit(plans, m.source); err != nil {
			return errMsg{fmt.Errorf("failed to apply split: %w", err)}
		}

		return splitAppliedMsg{}
	}
}

func (m Model) handleNavigation(delta int) (Model, tea.Cmd) {
	if m.focusedPanel == PanelFileList {
		newIdx := m.selectedFile + delta
		if newIdx >= 0 && newIdx < len(m.changes) {
			m.selectedFile = newIdx
			m.selectedHunk = 0
			m.lineCursor = 0
			m.fileList.SetSelected(m.selectedFile)
			m.diffView.SetFileChange(m.changes[m.selectedFile])
		}
	} else {
		m.diffView.Scroll(delta)
	}

	return m, nil
}

func (m Model) handleVisualNavigation(delta int) (Model, tea.Cmd) {
	if m.selectedFile < 0 || m.selectedFile >= len(m.changes) {
		return m, nil
	}
	file := m.changes[m.selectedFile]
	if m.selectedHunk < 0 || m.selectedHunk >= len(file.Hunks) {
		return m, nil
	}

	hunk := file.Hunks[m.selectedHunk]
	newCursor := m.lineCursor + delta

	if newCursor >= 0 && newCursor < len(hunk.Lines) {
		m.lineCursor = newCursor
	}

	return m, nil
}

func (m *Model) closeAllModals() {
	m.help.Hide()
	m.destPicker.Hide()
	m.splitAssign.Hide()
	m.splitPreview.Hide()
	m.commitMsg.Hide()
	m.searchModal.Hide()
	m.fileFinder.Hide()
	m.fileList.SetFilterMode(false)
}

func (m *Model) toggleVisualSelection() {
	if m.selectedFile < 0 || m.selectedFile >= len(m.changes) {
		return
	}
	file := m.changes[m.selectedFile]
	if m.selectedHunk < 0 || m.selectedHunk >= len(file.Hunks) {
		return
	}

	startLine := m.visualAnchor
	endLine := m.lineCursor

	m.selection.SelectLineRange(file.Path, m.selectedHunk, startLine, endLine)
}

func (m Model) toggleTagSelection(tag SplitTag) (Model, tea.Cmd) {
	if m.selectedFile < 0 || m.selectedFile >= len(m.changes) {
		return m, nil
	}
	file := m.changes[m.selectedFile]
	if m.selectedHunk < 0 || m.selectedHunk >= len(file.Hunks) {
		return m, nil
	}

	if _, ok := m.multiSplitState.Selections[tag]; !ok {
		m.multiSplitState.Selections[tag] = NewSelectionState()
	}

	tagSelection := m.multiSplitState.Selections[tag]
	if m.isVisualMode {
		startLine := m.visualAnchor
		endLine := m.lineCursor
		tagSelection.SelectLineRange(file.Path, m.selectedHunk, startLine, endLine)
		m.isVisualMode = false
	} else {
		tagSelection.ToggleHunk(file.Path, m.selectedHunk)
	}

	m.multiSplitState.CurrentTag = tag

	return m, nil
}

func (m Model) getHunkTags(filePath string, hunkIdx int) []SplitTag {
	if !m.multiSplitState.Active {
		return nil
	}

	var tags []SplitTag
	for tag, selection := range m.multiSplitState.Selections {
		if selection.IsHunkSelected(filePath, hunkIdx) ||
			selection.HasPartialSelection(filePath, hunkIdx) {
			tags = append(tags, tag)
		}
	}

	return tags
}

//nolint:unparam // tea.Cmd stays in the signature to match the sibling handlers handleActionKey dispatches through.
func (m Model) enterSearchMode() (Model, tea.Cmd) {
	m.searchState.SaveOriginalState(search.NavigationState{
		SelectedFile:   m.selectedFile,
		SelectedHunk:   m.selectedHunk,
		DiffViewOffset: 0,
		FocusedPanel:   int(m.focusedPanel),
	})
	m.searchModal.Show()

	return m, nil
}

func (m Model) handleSearchKeyPress(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case keyEnter:
		m.searchModal.Hide()
		return m, nil

	case "ctrl+n", keyDown:
		return m.nextSearchMatch()

	case "ctrl+p", "up":
		return m.prevSearchMatch()

	case keyBackspace:
		if m.searchState.Query != "" {
			m.searchState.Query = m.searchState.Query[:len(m.searchState.Query)-1]
			m.searchModal.SetQuery(m.searchState.Query)

			return m.executeSearch()
		}

		return m, nil

	default:
		if len(msg.String()) == 1 {
			m.searchState.Query += msg.String()
			m.searchModal.SetQuery(m.searchState.Query)

			return m.executeSearch()
		}

		return m, nil
	}
}

func (m Model) executeSearch() (Model, tea.Cmd) {
	m.searchState.ExecuteSearch(m.changes)
	m.searchState.IsActive = true
	m.searchModal.UpdateResults(m.searchState.MatchCount(), m.searchState.CurrentIdx)

	if match := m.searchState.GetCurrentMatch(); match != nil {
		m.selectedFile = match.FileIdx
		if match.HunkIdx >= 0 {
			m.selectedHunk = match.HunkIdx
		}
		m.focusedPanel = PanelDiffView
		m.fileList.SetSelected(m.selectedFile)
		if m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
			m.diffView.SetFileChange(m.changes[m.selectedFile])
		}
	}

	return m, nil
}

func (m Model) nextSearchMatch() (Model, tea.Cmd) {
	return m.jumpToMatch(m.searchState.NextMatch())
}

func (m Model) prevSearchMatch() (Model, tea.Cmd) {
	return m.jumpToMatch(m.searchState.PrevMatch())
}

// jumpToMatch moves the cursor and the diff view onto match, leaving the model untouched when match
// is nil so a search that ran off the end is a no-op rather than a jump to file zero.
func (m Model) jumpToMatch(match *search.MatchLocation) (Model, tea.Cmd) {
	if match != nil {
		m.searchModal.UpdateResults(m.searchState.MatchCount(), m.searchState.CurrentIdx)
		m.selectedFile = match.FileIdx
		if match.HunkIdx >= 0 {
			m.selectedHunk = match.HunkIdx
		}
		m.focusedPanel = PanelDiffView
		m.fileList.SetSelected(m.selectedFile)
		if m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
			m.diffView.SetFileChange(m.changes[m.selectedFile])
		}
	}

	return m, nil
}

func (m Model) getFilePathMatches(fileIdx int) []filelist.MatchRange {
	if m.searchState == nil || !m.searchState.IsActive {
		return nil
	}

	var ranges []filelist.MatchRange
	for _, match := range m.searchState.Matches {
		// Only include matches that are in file paths (HunkIdx == -1)
		if match.FileIdx == fileIdx && match.HunkIdx == -1 {
			ranges = append(ranges, filelist.MatchRange{
				Start: match.StartCol,
				End:   match.EndCol,
			})
		}
	}

	return ranges
}

func (m Model) getLineContentMatches(filePath string, hunkIdx, lineIdx int) []diffview.MatchRange {
	if m.searchState == nil || !m.searchState.IsActive {
		return nil
	}

	var ranges []diffview.MatchRange
	for _, match := range m.searchState.Matches {
		// Only include matches that are in line content
		if match.FilePath == filePath && match.HunkIdx == hunkIdx && match.LineIdx == lineIdx {
			ranges = append(ranges, diffview.MatchRange{
				Start: match.StartCol,
				End:   match.EndCol,
			})
		}
	}

	return ranges
}

//nolint:unparam // tea.Cmd stays in the signature to match the sibling handlers routeToOverlay dispatches through.
func (m Model) handleFileListFilterKeyPress(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.fileList.SetFilterMode(false)
		return m, nil

	case keyEnter:
		// Exit filter mode and focus diff view
		m.fileList.SetFilterMode(false)
		m.focusedPanel = PanelDiffView

		return m, nil

	case keyBackspace:
		query := m.fileList.FilterQuery()
		if query != "" {
			query = query[:len(query)-1]
			m.fileList.SetFilterQuery(query)
		}

		return m, nil

	default:
		if len(msg.String()) == 1 {
			query := m.fileList.FilterQuery()
			query += msg.String()
			m.fileList.SetFilterQuery(query)
		}

		return m, nil
	}
}

//nolint:unparam // tea.Cmd stays in the signature to match the sibling handlers routeToOverlay dispatches through.
func (m Model) handleFileFinderKeyPress(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case keyEnter:
		if fileIdx, ok := m.fileFinder.GetSelected().(int); ok {
			m.selectedFile = fileIdx
			m.selectedHunk = 0
			m.lineCursor = 0
			m.fileList.SetSelected(m.selectedFile)
			if m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
				m.diffView.SetFileChange(m.changes[m.selectedFile])
			}
			m.focusedPanel = PanelDiffView
			m.fileFinder.Hide()
		}

		return m, nil

	case "up", "ctrl+p":
		m.fileFinder.SelectPrev()
		return m, nil

	case keyDown, "ctrl+n":
		m.fileFinder.SelectNext()
		return m, nil

	case keyBackspace:
		query := m.fileFinder.Query()
		if query != "" {
			query = query[:len(query)-1]
			m.fileFinder.SetQuery(query)
		}

		return m, nil

	default:
		if len(msg.String()) == 1 {
			query := m.fileFinder.Query()
			query += msg.String()
			m.fileFinder.SetQuery(query)
		}

		return m, nil
	}
}

// View renders the two panels, or the overlay that is up. It returns the empty string until the
// first tea.WindowSizeMsg arrives, because every width is derived from m.width.
func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit", m.err)
	}

	// Bubble Tea renders once before the first tea.WindowSizeMsg arrives. Panel
	// widths are derived from m.width, so laying out at zero produces garbage.
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	if len(m.changes) == 0 {
		return "No changes found.\n\nPress r to refresh or q to quit"
	}

	if overlay := m.overlayView(); overlay != "" {
		return overlay
	}

	fileListExpanded := m.focusedPanel == PanelFileList
	m.fileList.SetExpanded(fileListExpanded)

	fileListHeight := collapsedFileListHeight
	if fileListExpanded {
		fileListHeight = max(m.height/expandedFileListFraction, minExpandedFileListHeight)
	}

	m.pushSelectionState()
	m.pushSearchState()
	m.pushTagState()

	fileListView := m.fileList.View(m.width, fileListHeight, fileListExpanded)
	diffViewHeight := m.height - fileListHeight - chromeHeight
	diffViewView := m.renderDiffView(diffViewHeight)

	border := lipgloss.NewStyle().
		Foreground(theme.Secondary).
		Render(strings.Repeat("\u2500", m.width))

	return fmt.Sprintf("%s\n%s\n%s\n%s", fileListView, border, diffViewView, m.renderStatusBar())
}

// overlayView returns the full-screen overlay that is up, or the empty string when the panels should
// render instead. The order is the stacking order on screen.
func (m Model) overlayView() string {
	switch {
	case m.help.IsVisible():
		return m.help.View(m.width, m.height)
	case m.destPicker.IsVisible():
		return m.destPicker.View(m.width, m.height)
	case m.splitAssign.IsVisible():
		return m.splitAssign.View(m.width, m.height)
	case m.splitPreview.IsVisible():
		return m.splitPreview.View(m.width, m.height)
	case m.commitMsg.IsVisible():
		return m.commitMsg.View(m.width, m.height)
	case m.searchModal.IsVisible():
		return m.searchModal.View(m.width, m.height)
	case m.fileFinder.IsVisible():
		return m.fileFinder.View(m.width, m.height)
	}

	return ""
}

// pushSelectionState hands the diff view the callbacks it needs to draw the current selection. Browse
// mode reports nothing selected, so the view still highlights the hunk cursor without marking it.
func (m Model) pushSelectionState() {
	if m.selectedFile < 0 || m.selectedFile >= len(m.changes) {
		return
	}

	currentFile := m.changes[m.selectedFile]

	if m.mode != ModeInteractive && m.mode != ModeDiffEditor {
		m.diffView.SetSelection(m.selectedHunk, func(_ int) bool {
			return false
		})

		return
	}

	m.diffView.SetSelection(m.selectedHunk, func(hunkIdx int) bool {
		return m.selection.IsHunkSelected(currentFile.Path, hunkIdx)
	})
	m.diffView.SetVisualState(
		m.lineCursor,
		m.isVisualMode,
		m.visualAnchor,
		func(hunkIdx, lineIdx int) bool {
			return m.selection.IsLineSelected(currentFile.Path, hunkIdx, lineIdx)
		},
	)
}

func (m Model) pushSearchState() {
	if m.searchState == nil || !m.searchState.IsActive {
		m.fileList.SetSearchState(false, nil)
		m.diffView.SetSearchState(false, nil)

		return
	}

	m.fileList.SetSearchState(true, m.getFilePathMatches)

	if m.selectedFile < 0 || m.selectedFile >= len(m.changes) {
		return
	}

	currentFile := m.changes[m.selectedFile]
	m.diffView.SetSearchState(true, func(hunkIdx, lineIdx int) []diffview.MatchRange {
		return m.getLineContentMatches(currentFile.Path, hunkIdx, lineIdx)
	})
}

func (m Model) pushTagState() {
	if m.selectedFile < 0 || m.selectedFile >= len(m.changes) {
		return
	}

	currentFile := m.changes[m.selectedFile]
	m.diffView.SetTagState(func(hunkIdx int) []diffview.SplitTag {
		tags := m.getHunkTags(currentFile.Path, hunkIdx)
		diffviewTags := make([]diffview.SplitTag, 0, len(tags))
		for _, tag := range tags {
			diffviewTags = append(diffviewTags, diffview.SplitTag(tag))
		}

		return diffviewTags
	})
}

// renderDiffView dims the whole pane while the file list has focus, so the two panels read as one
// focused and one inactive.
func (m Model) renderDiffView(height int) string {
	focused := m.focusedPanel == PanelDiffView

	view := m.diffView.View(m.width, height, focused)
	if focused {
		return view
	}

	dimStyle := lipgloss.NewStyle().Faint(true)
	lines := strings.Split(view, "\n")

	for i, line := range lines {
		lines[i] = dimStyle.Render(line)
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderStatusBar() string {
	focusedPanelStr := "files"
	if m.focusedPanel == PanelDiffView {
		focusedPanelStr = "diff"
	}

	modeText := "Browse"

	switch m.mode {
	case ModeInteractive:
		modeText = "Interactive"
	case ModeDiffEditor:
		modeText = "Diff-Editor"
	case ModeBrowse:
	}

	return m.statusBar.ViewWithContext(m.width, statusbar.Context{
		Destination:  m.destination,
		FocusedPanel: focusedPanelStr,
		IsVisualMode: m.isVisualMode,
		Mode:         modeText,
		Source:       m.source,
	})
}

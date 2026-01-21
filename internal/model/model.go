package model

import (
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

type OperatingMode int

const (
	ModeBrowse OperatingMode = iota
	ModeInteractive
	ModeDiffEditor
)

type FocusedPanel int

const (
	PanelFileList FocusedPanel = iota
	PanelDiffView
)

type HunkSelection struct {
	WholeHunk    bool
	SelectedLines map[int]bool
}

type FileSelection struct {
	Hunks map[int]*HunkSelection
}

type SelectionState struct {
	Files map[string]*FileSelection
}

func NewSelectionState() *SelectionState {
	return &SelectionState{
		Files: make(map[string]*FileSelection),
	}
}

type SplitTag rune

type DestinationType int

const (
	DestExistingRevision DestinationType = iota
	DestNewCommit
)

type DestinationSpec struct {
	Type        DestinationType
	ChangeID    string
	Description string
}

type MultiSplitState struct {
	Active       bool
	Selections   map[SplitTag]*SelectionState
	Destinations map[SplitTag]*DestinationSpec
	CurrentTag   SplitTag
}

func NewMultiSplitState() *MultiSplitState {
	return &MultiSplitState{
		Active:       false,
		Selections:   make(map[SplitTag]*SelectionState),
		Destinations: make(map[SplitTag]*DestinationSpec),
		CurrentTag:   'A',
	}
}

func (s *SelectionState) IsHunkSelected(filePath string, hunkIdx int) bool {
	if fileSelection, ok := s.Files[filePath]; ok {
		if hunkSelection, ok := fileSelection.Hunks[hunkIdx]; ok {
			return hunkSelection.WholeHunk
		}
	}
	return false
}

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

func (s *SelectionState) HasPartialSelection(filePath string, hunkIdx int) bool {
	if fileSelection, ok := s.Files[filePath]; ok {
		if hunkSelection, ok := fileSelection.Hunks[hunkIdx]; ok {
			return !hunkSelection.WholeHunk && len(hunkSelection.SelectedLines) > 0
		}
	}
	return false
}

type Model struct {
	client      *jj.Client
	diffSource  diff.DiffSource
	mode        OperatingMode
	source      string
	destination string
	cfg         config.Config

	changes      []diff.FileChange
	selectedFile int
	selectedHunk int
	focusedPanel FocusedPanel

	// Visual mode state for line-level selection
	isVisualMode bool
	visualAnchor int
	lineCursor   int

	selection       *SelectionState
	multiSplitState *MultiSplitState
	fileList        filelist.Model
	diffView        diffview.Model
	statusBar       statusbar.Model
	destPicker      destpicker.Model
	splitAssign     splitassign.Model
	splitPreview    splitpreview.Model
	commitMsg       commitmsg.Model
	help            help.Model

	// Search state
	searchModal searchmodal.Model
	searchState *search.SearchState

	// File finder
	fileFinder filefinder.Model

	width  int
	height int

	err error
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

func NewModel(client *jj.Client, source, destination string, mode OperatingMode, cfg config.Config) (Model, error) {
	revSource := diff.NewRevisionSource(client, source)
	return NewModelWithSource(revSource, client, destination, mode, cfg)
}

func NewModelWithSource(source diff.DiffSource, client *jj.Client, destination string, mode OperatingMode, cfg config.Config) (Model, error) {
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
		width:           80,
		height:          24,
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
	m.searchState = search.NewSearchState()
	m.fileFinder = filefinder.New()

	return m, nil
}

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
		revisions, err := m.client.GetRevisions(20)
		if err != nil {
			return errMsg{err}
		}

		return revisionsLoadedMsg{revisions}
	}
}

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

	case diffEditorAppliedMsg:
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if key == "esc" {
		if m.help.IsVisible() {
			m.help.Hide()
			return m, nil
		}
		if m.destPicker.IsVisible() {
			m.destPicker.Hide()
			return m, nil
		}
		if m.splitAssign.IsVisible() {
			m.splitAssign.Hide()
			return m, nil
		}
		if m.splitPreview.IsVisible() {
			m.splitPreview.Hide()
			return m, nil
		}
		if m.commitMsg.IsVisible() {
			m.commitMsg.Hide()
			return m, nil
		}
		if m.searchModal.IsVisible() {
			if m.searchState != nil {
				origState := m.searchState.RestoreOriginalState()
				m.selectedFile = origState.SelectedFile
				m.selectedHunk = origState.SelectedHunk
				m.focusedPanel = FocusedPanel(origState.FocusedPanel)
			}
			m.searchModal.Hide()
			m.searchState.IsActive = false
			return m, nil
		}
		if m.fileFinder.IsVisible() {
			m.fileFinder.Hide()
			return m, nil
		}
		if m.fileList.IsFilterMode() {
			m.fileList.SetFilterMode(false)
			return m, nil
		}
		if m.isVisualMode {
			m.isVisualMode = false
			m.visualAnchor = 0
			return m, nil
		}
		return m, nil
	}

	if key == "?" && !m.destPicker.IsVisible() {
		if m.help.IsVisible() {
			m.help.Hide()
			return m, nil
		}
		m.closeAllModals()
		modeText := "Browse"
		if m.mode == ModeInteractive {
			modeText = "Interactive"
		}
		m.help.Show(modeText)
		return m, nil
	}

	if m.help.IsVisible() {
		switch key {
		case "q":
			m.help.Hide()
		}
		return m, nil
	}

	if m.destPicker.IsVisible() {
		return m.handleDestPickerKeyPress(msg)
	}

	if m.splitAssign.IsVisible() {
		return m.handleSplitAssignKeyPress(msg)
	}

	if m.splitPreview.IsVisible() {
		return m.handleSplitPreviewKeyPress(msg)
	}

	if m.commitMsg.IsVisible() {
		return m.handleCommitMsgKeyPress(msg)
	}

	if m.searchModal.IsVisible() {
		return m.handleSearchKeyPress(msg)
	}

	if m.fileFinder.IsVisible() {
		return m.handleFileFinderKeyPress(msg)
	}

	if m.fileList.IsFilterMode() {
		return m.handleFileListFilterKeyPress(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "d":
		if m.mode == ModeInteractive && m.diffSource.SupportsRevisions() {
			return m, m.loadRevisions()
		}
		return m, nil

	case "/":
		m.closeAllModals()
		return m.enterSearchMode()

	case "f":
		m.closeAllModals()
		m.focusedPanel = PanelFileList
		m.fileList.SetFilterMode(true)
		return m, nil

	case "v":
		if (m.mode == ModeInteractive || m.mode == ModeDiffEditor) && m.focusedPanel == PanelDiffView {
			if m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
				file := m.changes[m.selectedFile]
				if m.selectedHunk >= 0 && m.selectedHunk < len(file.Hunks) {
					m.isVisualMode = true
					m.visualAnchor = m.lineCursor
				}
			}
		}
		return m, nil

	case "tab":
		if m.focusedPanel == PanelFileList {
			m.focusedPanel = PanelDiffView
		} else {
			m.focusedPanel = PanelFileList
		}
		return m, nil

	case "[":
		// Previous file (works from diff view)
		if m.focusedPanel == PanelDiffView && m.selectedFile > 0 {
			m.selectedFile--
			m.selectedHunk = 0
			m.lineCursor = 0
			m.fileList.SetSelected(m.selectedFile)
			if m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
				m.diffView.SetFileChange(m.changes[m.selectedFile])
			}
		}
		return m, nil

	case "]":
		// Next file (works from diff view)
		if m.focusedPanel == PanelDiffView && m.selectedFile < len(m.changes)-1 {
			m.selectedFile++
			m.selectedHunk = 0
			m.lineCursor = 0
			m.fileList.SetSelected(m.selectedFile)
			if m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
				m.diffView.SetFileChange(m.changes[m.selectedFile])
			}
		}
		return m, nil

	case "j", "down":
		if m.isVisualMode {
			return m.handleVisualNavigation(1)
		}
		return m.handleNavigation(1)

	case "k", "up":
		if m.isVisualMode {
			return m.handleVisualNavigation(-1)
		}
		return m.handleNavigation(-1)

	case "ctrl+d":
		if m.focusedPanel == PanelDiffView {
			contentHeight := m.height - 2
			m.diffView.ScrollHalfPageDown(contentHeight)
		}
		return m, nil

	case "ctrl+u":
		if m.focusedPanel == PanelDiffView {
			contentHeight := m.height - 2
			m.diffView.ScrollHalfPageUp(contentHeight)
		}
		return m, nil

	case "ctrl+f":
		if m.focusedPanel == PanelDiffView {
			contentHeight := m.height - 2
			m.diffView.ScrollFullPageDown(contentHeight)
		}
		return m, nil

	case "ctrl+b":
		if m.focusedPanel == PanelDiffView {
			contentHeight := m.height - 2
			m.diffView.ScrollFullPageUp(contentHeight)
		}
		return m, nil

	case "g":
		m.selectedFile = 0
		m.selectedHunk = 0
		m.lineCursor = 0
		m.fileList.SetSelected(m.selectedFile)
		if len(m.changes) > 0 {
			m.diffView.SetFileChange(m.changes[m.selectedFile])
		}
		return m, nil

	case "G":
		m.selectedFile = len(m.changes) - 1
		m.selectedHunk = 0
		m.lineCursor = 0
		m.fileList.SetSelected(m.selectedFile)
		if len(m.changes) > 0 {
			m.diffView.SetFileChange(m.changes[m.selectedFile])
		}
		return m, nil

	case "r":
		return m, m.loadDiff()

	case " ":
		if (m.mode == ModeInteractive || m.mode == ModeDiffEditor) && m.focusedPanel == PanelDiffView {
			if m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
				file := m.changes[m.selectedFile]
				if m.selectedHunk >= 0 && m.selectedHunk < len(file.Hunks) {
					if m.isVisualMode {
						m.toggleVisualSelection()
						m.isVisualMode = false
					} else {
						m.selection.ToggleHunk(file.Path, m.selectedHunk)
					}
				}
			}
		}
		return m, nil

	case "n":
		if m.searchState != nil && m.searchState.IsActive && len(m.searchState.Matches) > 0 {
			return m.nextSearchMatch()
		}
		if m.focusedPanel == PanelDiffView && m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
			file := m.changes[m.selectedFile]
			if len(file.Hunks) > 0 {
				m.selectedHunk++
				if m.selectedHunk >= len(file.Hunks) {
					m.selectedHunk = 0
				}
				m.lineCursor = 0
			}
		}
		return m, nil

	case "N":
		if m.searchState != nil && m.searchState.IsActive && len(m.searchState.Matches) > 0 {
			return m.prevSearchMatch()
		}
		if m.focusedPanel == PanelDiffView && m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
			file := m.changes[m.selectedFile]
			if len(file.Hunks) > 0 {
				m.selectedHunk--
				if m.selectedHunk < 0 {
					m.selectedHunk = len(file.Hunks) - 1
				}
				m.lineCursor = 0
			}
		}
		return m, nil

	case "p":
		if m.focusedPanel == PanelDiffView && m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
			file := m.changes[m.selectedFile]
			if len(file.Hunks) > 0 {
				m.selectedHunk--
				if m.selectedHunk < 0 {
					m.selectedHunk = len(file.Hunks) - 1
				}
				m.lineCursor = 0
			}
		}
		return m, nil

	case "w":
		m.diffView.ToggleWhitespace()
		return m, nil

	case "W":
		m.diffView.ToggleWordDiff()
		return m, nil

	case "s":
		m.diffView.ToggleSideBySide()
		return m, nil

	case "l":
		m.diffView.ToggleLineNumbers()
		return m, nil

	case "a":
		if m.mode == ModeInteractive && m.destination != "" {
			return m, m.applySelection()
		}
		if m.mode == ModeDiffEditor {
			return m, m.applyDiffEditorSelection()
		}
		return m, nil

	case "S":
		if m.mode == ModeInteractive && m.focusedPanel == PanelDiffView {
			m.multiSplitState.Active = !m.multiSplitState.Active
			if m.multiSplitState.Active {
				m.multiSplitState.CurrentTag = 'A'
			}
		}
		return m, nil

	case "D":
		if m.mode == ModeInteractive && m.multiSplitState.Active {
			var tags []splitassign.SplitTag
			for tag := range m.multiSplitState.Selections {
				tags = append(tags, splitassign.SplitTag(tag))
			}
			if len(tags) > 0 {
				m.splitAssign.SetTags(tags)
				return m, m.loadRevisionsForSplitAssign()
			}
		}
		return m, nil

	case "P":
		if m.mode == ModeInteractive && m.multiSplitState.Active {
			destinations := m.splitAssign.GetDestinations()
			if len(destinations) > 0 {
				summaries := m.buildSplitSummaries(destinations)
				m.splitPreview.SetSummaries(summaries)
				m.splitPreview.Show()
			}
		}
		return m, nil

	default:
		if m.multiSplitState.Active && m.mode == ModeInteractive && m.focusedPanel == PanelDiffView {
			if len(key) == 1 {
				r := rune(key[0])
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
					if r >= 'a' && r <= 'z' {
						r = r - 'a' + 'A'
					}
					return m.toggleTagSelection(SplitTag(r))
				}
			}
		}
	}

	return m, nil
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
			return errMsg{fmt.Errorf("no hunks or lines selected")}
		}

		patch := diff.GeneratePatch(m.changes, m.selection)

		err := m.client.MoveChanges(patch, m.source, m.destination)
		if err != nil {
			return errMsg{fmt.Errorf("failed to move changes: %w", err)}
		}

		return m.loadDiff()
	}
}

type diffEditorAppliedMsg struct{}

func (m Model) applyDiffEditorSelection() tea.Cmd {
	return func() tea.Msg {
		dirSource, ok := m.diffSource.(*diff.DirectorySource)
		if !ok {
			return errMsg{fmt.Errorf("diff-editor mode requires directory source")}
		}

		applier := diff.NewApplier(dirSource.LeftPath, dirSource.RightPath)
		if err := applier.ApplySelections(m.changes, m.selection); err != nil {
			return errMsg{fmt.Errorf("failed to apply selections: %w", err)}
		}

		return diffEditorAppliedMsg{}
	}
}

func (m Model) handleDestPickerKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.destPicker.Hide()
		return m, nil

	case "j", "down":
		m.destPicker.MoveDown()
		return m, nil

	case "k", "up":
		m.destPicker.MoveUp()
		return m, nil

	case "enter":
		if selected := m.destPicker.GetSelected(); selected != nil {
			return m, func() tea.Msg {
				return destinationSelectedMsg{changeID: selected.ChangeID}
			}
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleSplitAssignKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.splitAssign.Hide()
		return m, nil

	case "j", "down":
		m.splitAssign.MoveDown()
		return m, nil

	case "k", "up":
		m.splitAssign.MoveUp()
		return m, nil

	case "tab":
		m.splitAssign.ToggleFocus()
		return m, nil

	case "enter":
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

func (m Model) handleSplitPreviewKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.splitPreview.Hide()
		return m, nil

	case "e":
		m.splitPreview.Hide()
		return m, m.loadRevisionsForSplitAssign()

	case "enter":
		return m, m.applySplit()
	}

	return m, nil
}

func (m Model) handleCommitMsgKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.commitMsg.Hide()
		m.splitAssign.Show()
		return m, nil

	case "enter":
		message := m.commitMsg.GetMessage()
		if message != "" {
			tag := m.commitMsg.GetTag()
			m.splitAssign.AssignNewCommitToTag(splitassign.SplitTag(tag), message)
		}
		m.commitMsg.Hide()
		m.splitAssign.Show()
		return m, nil

	case "backspace":
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
		revisions, err := m.client.GetRevisions(20)
		if err != nil {
			return errMsg{err}
		}
		m.closeAllModals()
		m.splitAssign.SetRevisions(revisions)
		m.splitAssign.Show()
		return nil
	}
}

func (m Model) buildSplitSummaries(destinations map[splitassign.SplitTag]*splitassign.DestinationSpec) []splitpreview.SplitSummary {
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
				if tagSelection.IsHunkSelected(file.Path, hunkIdx) || tagSelection.HasPartialSelection(file.Path, hunkIdx) {
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
			return errMsg{fmt.Errorf("no destinations assigned")}
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
			return errMsg{fmt.Errorf("no valid split plans generated")}
		}

		if err := m.client.ApplySplit(plans, m.source); err != nil {
			return errMsg{fmt.Errorf("failed to apply split: %w", err)}
		}

		m.multiSplitState = NewMultiSplitState()
		m.splitPreview.Hide()
		return m.loadDiff()
	}
}

func (m Model) handleNavigation(delta int) (tea.Model, tea.Cmd) {
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

func (m Model) handleVisualNavigation(delta int) (tea.Model, tea.Cmd) {
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

func (m Model) toggleTagSelection(tag SplitTag) (tea.Model, tea.Cmd) {
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
		if selection.IsHunkSelected(filePath, hunkIdx) || selection.HasPartialSelection(filePath, hunkIdx) {
			tags = append(tags, tag)
		}
	}
	return tags
}

func (m Model) enterSearchMode() (tea.Model, tea.Cmd) {
	m.searchState.SaveOriginalState(search.NavigationState{
		SelectedFile:   m.selectedFile,
		SelectedHunk:   m.selectedHunk,
		DiffViewOffset: 0,
		FocusedPanel:   int(m.focusedPanel),
	})
	m.searchModal.Show()
	return m, nil
}

func (m Model) handleSearchKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.searchModal.Hide()
		return m, nil

	case "ctrl+n", "down":
		return m.nextSearchMatch()

	case "ctrl+p", "up":
		return m.prevSearchMatch()

	case "backspace":
		if len(m.searchState.Query) > 0 {
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

func (m Model) executeSearch() (tea.Model, tea.Cmd) {
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

func (m Model) nextSearchMatch() (tea.Model, tea.Cmd) {
	if match := m.searchState.NextMatch(); match != nil {
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

func (m Model) prevSearchMatch() (tea.Model, tea.Cmd) {
	if match := m.searchState.PrevMatch(); match != nil {
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

func (m Model) enterFileFinderMode() (tea.Model, tea.Cmd) {
	// Build list of file paths and indices
	paths := make([]string, len(m.changes))
	indices := make([]interface{}, len(m.changes))
	for i, change := range m.changes {
		paths[i] = change.Path
		indices[i] = i
	}

	m.fileFinder.Show(paths, indices)
	return m, nil
}

func (m Model) handleFileListFilterKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.fileList.SetFilterMode(false)
		return m, nil

	case "enter":
		// Exit filter mode and focus diff view
		m.fileList.SetFilterMode(false)
		m.focusedPanel = PanelDiffView
		return m, nil

	case "backspace":
		query := m.fileList.FilterQuery()
		if len(query) > 0 {
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

func (m Model) handleFileFinderKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if selected := m.fileFinder.GetSelected(); selected != nil {
			fileIdx := selected.(int)
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

	case "down", "ctrl+n":
		m.fileFinder.SelectNext()
		return m, nil

	case "backspace":
		query := m.fileFinder.Query()
		if len(query) > 0 {
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

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit", m.err)
	}

	if len(m.changes) == 0 {
		return "No changes found.\n\nPress r to refresh or q to quit"
	}

	// Calculate panel heights for vertical layout
	fileListExpanded := m.focusedPanel == PanelFileList
	m.fileList.SetExpanded(fileListExpanded)

	var fileListHeight int
	if fileListExpanded {
		fileListHeight = m.height / 4 // 25% when expanded
		if fileListHeight < 5 {
			fileListHeight = 5
		}
	} else {
		fileListHeight = 1 // Single row when collapsed
	}

	diffViewHeight := m.height - fileListHeight - 2 // -2 for border + status bar

	var modeText string
	switch m.mode {
	case ModeBrowse:
		modeText = "Browse"
	case ModeInteractive:
		modeText = "Interactive"
	case ModeDiffEditor:
		modeText = "Diff-Editor"
	}

	// Set selection state for diffview (needed for navigation highlighting in both modes)
	if m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
		currentFile := m.changes[m.selectedFile]

		if m.mode == ModeInteractive || m.mode == ModeDiffEditor {
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
		} else {
			// In Browse mode, just set the selected hunk for navigation
			m.diffView.SetSelection(m.selectedHunk, func(hunkIdx int) bool {
				return false // No selection in browse mode
			})
		}
	}

	// Set search state for components
	if m.searchState != nil && m.searchState.IsActive {
		m.fileList.SetSearchState(true, func(fileIdx int) []filelist.MatchRange {
			return m.getFilePathMatches(fileIdx)
		})

		if m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
			currentFile := m.changes[m.selectedFile]
			m.diffView.SetSearchState(true, func(hunkIdx, lineIdx int) []diffview.MatchRange {
				return m.getLineContentMatches(currentFile.Path, hunkIdx, lineIdx)
			})
		}
	} else {
		m.fileList.SetSearchState(false, nil)
		m.diffView.SetSearchState(false, nil)
	}

	// Set tag state for diffview
	if m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
		currentFile := m.changes[m.selectedFile]
		m.diffView.SetTagState(func(hunkIdx int) []diffview.SplitTag {
			tags := m.getHunkTags(currentFile.Path, hunkIdx)
			var diffviewTags []diffview.SplitTag
			for _, tag := range tags {
				diffviewTags = append(diffviewTags, diffview.SplitTag(tag))
			}
			return diffviewTags
		})
	}

	// Render file list panel
	fileListView := m.fileList.View(m.width, fileListHeight, m.focusedPanel == PanelFileList)

	// Render diff view panel with optional dimming when file list focused
	diffViewFocused := m.focusedPanel == PanelDiffView
	diffViewView := m.diffView.View(m.width, diffViewHeight, diffViewFocused)

	// Apply dimming to diff view when file list is focused
	if !diffViewFocused {
		dimmedLines := strings.Split(diffViewView, "\n")
		dimStyle := lipgloss.NewStyle().Faint(true)
		for i, line := range dimmedLines {
			dimmedLines[i] = dimStyle.Render(line)
		}
		diffViewView = strings.Join(dimmedLines, "\n")
	}

	// Create border between panels
	borderLine := strings.Repeat("─", m.width)
	border := lipgloss.NewStyle().
		Foreground(theme.Secondary).
		Render(borderLine)

	focusedPanelStr := "files"
	if m.focusedPanel == PanelDiffView {
		focusedPanelStr = "diff"
	}
	statusBarView := m.statusBar.ViewWithContext(m.width, statusbar.Context{
		Destination:  m.destination,
		FocusedPanel: focusedPanelStr,
		IsVisualMode: m.isVisualMode,
		Mode:         modeText,
		Source:       m.source,
	})

	// Vertical layout: file list on top, border, diff view, status bar
	baseView := fmt.Sprintf("%s\n%s\n%s\n%s", fileListView, border, diffViewView, statusBarView)

	if m.help.IsVisible() {
		return m.help.View(m.width, m.height)
	}

	if m.destPicker.IsVisible() {
		return m.destPicker.View(m.width, m.height)
	}

	if m.splitAssign.IsVisible() {
		return m.splitAssign.View(m.width, m.height)
	}

	if m.splitPreview.IsVisible() {
		return m.splitPreview.View(m.width, m.height)
	}

	if m.commitMsg.IsVisible() {
		return m.commitMsg.View(m.width, m.height)
	}

	if m.searchModal.IsVisible() {
		return m.searchModal.View(m.width, m.height)
	}

	if m.fileFinder.IsVisible() {
		return m.fileFinder.View(m.width, m.height)
	}

	return baseView
}

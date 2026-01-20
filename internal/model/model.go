package model

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/jj-diff/internal/components/destpicker"
	"github.com/kyleking/jj-diff/internal/components/diffview"
	"github.com/kyleking/jj-diff/internal/components/filelist"
	"github.com/kyleking/jj-diff/internal/components/help"
	"github.com/kyleking/jj-diff/internal/components/searchmodal"
	"github.com/kyleking/jj-diff/internal/components/statusbar"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/search"
)

type OperatingMode int

const (
	ModeBrowse OperatingMode = iota
	ModeInteractive
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
	mode        OperatingMode
	source      string
	destination string

	changes      []diff.FileChange
	selectedFile int
	selectedHunk int
	focusedPanel FocusedPanel

	// Visual mode state for line-level selection
	isVisualMode bool
	visualAnchor int
	lineCursor   int

	selection  *SelectionState
	fileList   filelist.Model
	diffView   diffview.Model
	statusBar  statusbar.Model
	destPicker destpicker.Model
	help       help.Model

	// Search state
	searchModal searchmodal.Model
	searchState *search.SearchState

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

func NewModel(client *jj.Client, source, destination string, mode OperatingMode) (Model, error) {
	m := Model{
		client:       client,
		mode:         mode,
		source:       source,
		destination:  destination,
		selectedFile: 0,
		selectedHunk: 0,
		focusedPanel: PanelFileList,
		width:        80,
		height:       24,
		selection:    NewSelectionState(),
	}

	m.fileList = filelist.New()
	m.diffView = diffview.New()
	m.statusBar = statusbar.New()
	m.destPicker = destpicker.New()
	m.help = help.New()
	m.searchModal = searchmodal.New()
	m.searchState = search.NewSearchState()

	return m, nil
}

func (m Model) Init() tea.Cmd {
	return m.loadDiff()
}

func (m Model) loadDiff() tea.Cmd {
	return func() tea.Msg {
		diffText, err := m.client.Diff(m.source)
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
		m.destPicker.SetRevisions(msg.revisions)
		m.destPicker.Show()
		return m, nil

	case destinationSelectedMsg:
		m.destination = msg.changeID
		m.destPicker.Hide()
		return m, m.loadDiff()
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.help.IsVisible() {
		switch msg.String() {
		case "?", "esc", "q":
			m.help.Hide()
		}
		return m, nil
	}

	if m.destPicker.IsVisible() {
		return m.handleDestPickerKeyPress(msg)
	}

	if m.searchModal.IsVisible() {
		return m.handleSearchKeyPress(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "?":
		modeText := "Browse"
		if m.mode == ModeInteractive {
			modeText = "Interactive"
		}
		m.help.Show(modeText)
		return m, nil

	case "d":
		if m.mode == ModeInteractive {
			return m, m.loadRevisions()
		}
		return m, nil

	case "/":
		return m.enterSearchMode()

	case "v":
		if m.mode == ModeInteractive && m.focusedPanel == PanelDiffView {
			if m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
				file := m.changes[m.selectedFile]
				if m.selectedHunk >= 0 && m.selectedHunk < len(file.Hunks) {
					m.isVisualMode = true
					m.visualAnchor = m.lineCursor
				}
			}
		}
		return m, nil

	case "esc":
		if m.isVisualMode {
			m.isVisualMode = false
			m.visualAnchor = 0
			return m, nil
		}
		return m, nil

	case "tab":
		if m.focusedPanel == PanelFileList {
			m.focusedPanel = PanelDiffView
		} else {
			m.focusedPanel = PanelFileList
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
		if m.mode == ModeInteractive && m.focusedPanel == PanelDiffView {
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
			if m.selectedHunk < len(file.Hunks)-1 {
				m.selectedHunk++
				m.lineCursor = 0
			}
		}
		return m, nil

	case "N":
		if m.searchState != nil && m.searchState.IsActive && len(m.searchState.Matches) > 0 {
			return m.prevSearchMatch()
		}
		return m, nil

	case "p":
		if m.focusedPanel == PanelDiffView {
			if m.selectedHunk > 0 {
				m.selectedHunk--
				m.lineCursor = 0
			}
		}
		return m, nil

	case "a":
		if m.mode == ModeInteractive && m.destination != "" {
			return m, m.applySelection()
		}
		return m, nil
	}

	return m, nil
}

func (m Model) applySelection() tea.Cmd {
	return func() tea.Msg {
		// Check if any hunks or lines are selected
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

func (m Model) handleDestPickerKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
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
	case "esc":
		if m.searchState != nil {
			origState := m.searchState.RestoreOriginalState()
			m.selectedFile = origState.SelectedFile
			m.selectedHunk = origState.SelectedHunk
			m.focusedPanel = FocusedPanel(origState.FocusedPanel)
		}
		m.searchModal.Hide()
		m.searchState.IsActive = false
		return m, nil

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

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit", m.err)
	}

	if len(m.changes) == 0 {
		return "No changes found.\n\nPress r to refresh or q to quit"
	}

	fileListWidth := m.width / 4
	diffViewWidth := m.width - fileListWidth - 2
	contentHeight := m.height - 2

	modeText := "Browse"
	if m.mode == ModeInteractive {
		modeText = "Interactive"
	}

	if m.mode == ModeInteractive && m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
		currentFile := m.changes[m.selectedFile]
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

	fileListView := m.fileList.View(fileListWidth, contentHeight, m.focusedPanel == PanelFileList)
	diffViewView := m.diffView.View(diffViewWidth, contentHeight)
	statusBarView := m.statusBar.View(m.width, modeText, m.source, m.destination, m.isVisualMode)

	baseView := fmt.Sprintf("%s│%s\n%s", fileListView, diffViewView, statusBarView)

	if m.help.IsVisible() {
		return m.help.View(m.width, m.height) + "\n" + baseView
	}

	if m.destPicker.IsVisible() {
		return m.destPicker.View(m.width, m.height) + "\n" + baseView
	}

	if m.searchModal.IsVisible() {
		return m.searchModal.View(m.width, m.height) + "\n" + baseView
	}

	return baseView
}

package model

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/jj-diff/internal/components/destpicker"
	"github.com/kyleking/jj-diff/internal/components/diffview"
	"github.com/kyleking/jj-diff/internal/components/filelist"
	"github.com/kyleking/jj-diff/internal/components/help"
	"github.com/kyleking/jj-diff/internal/components/statusbar"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
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

type Model struct {
	client      *jj.Client
	mode        OperatingMode
	source      string
	destination string

	changes      []diff.FileChange
	selectedFile int
	selectedHunk int
	focusedPanel FocusedPanel

	selection  *SelectionState
	fileList   filelist.Model
	diffView   diffview.Model
	statusBar  statusbar.Model
	destPicker destpicker.Model
	help       help.Model

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

	case "tab":
		if m.focusedPanel == PanelFileList {
			m.focusedPanel = PanelDiffView
		} else {
			m.focusedPanel = PanelFileList
		}
		return m, nil

	case "j", "down":
		return m.handleNavigation(1)

	case "k", "up":
		return m.handleNavigation(-1)

	case "g":
		m.selectedFile = 0
		m.selectedHunk = 0
		m.fileList.SetSelected(m.selectedFile)
		if len(m.changes) > 0 {
			m.diffView.SetFileChange(m.changes[m.selectedFile])
		}
		return m, nil

	case "G":
		m.selectedFile = len(m.changes) - 1
		m.selectedHunk = 0
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
					m.selection.ToggleHunk(file.Path, m.selectedHunk)
				}
			}
		}
		return m, nil

	case "n":
		if m.focusedPanel == PanelDiffView && m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
			file := m.changes[m.selectedFile]
			if m.selectedHunk < len(file.Hunks)-1 {
				m.selectedHunk++
			}
		}
		return m, nil

	case "p":
		if m.focusedPanel == PanelDiffView {
			if m.selectedHunk > 0 {
				m.selectedHunk--
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
		selectedHunks := diff.GetSelectedHunksMap(m.changes, m.selection)

		if len(selectedHunks) == 0 {
			return errMsg{fmt.Errorf("no hunks selected")}
		}

		patch := diff.GeneratePatch(m.changes, selectedHunks)

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
			m.fileList.SetSelected(m.selectedFile)
			m.diffView.SetFileChange(m.changes[m.selectedFile])
		}
	} else {
		m.diffView.Scroll(delta)
	}

	return m, nil
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
	}

	fileListView := m.fileList.View(fileListWidth, contentHeight, m.focusedPanel == PanelFileList)
	diffViewView := m.diffView.View(diffViewWidth, contentHeight)
	statusBarView := m.statusBar.View(m.width, modeText, m.source, m.destination)

	baseView := fmt.Sprintf("%s│%s\n%s", fileListView, diffViewView, statusBarView)

	if m.help.IsVisible() {
		return m.help.View(m.width, m.height) + "\n" + baseView
	}

	if m.destPicker.IsVisible() {
		return m.destPicker.View(m.width, m.height) + "\n" + baseView
	}

	return baseView
}

package model

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/jj-diff/internal/components/diffview"
	"github.com/kyleking/jj-diff/internal/components/filelist"
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

type Model struct {
	client      *jj.Client
	mode        OperatingMode
	source      string
	destination string

	changes      []diff.FileChange
	selectedFile int
	focusedPanel FocusedPanel

	fileList  filelist.Model
	diffView  diffview.Model
	statusBar statusbar.Model

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

func NewModel(client *jj.Client, source, destination string, mode OperatingMode) (Model, error) {
	m := Model{
		client:       client,
		mode:         mode,
		source:       source,
		destination:  destination,
		selectedFile: 0,
		focusedPanel: PanelFileList,
		width:        80,
		height:       24,
	}

	m.fileList = filelist.New()
	m.diffView = diffview.New()
	m.statusBar = statusbar.New()

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
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "?":
		// TODO: Show help overlay
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
		m.fileList.SetSelected(m.selectedFile)
		if len(m.changes) > 0 {
			m.diffView.SetFileChange(m.changes[m.selectedFile])
		}
		return m, nil

	case "G":
		m.selectedFile = len(m.changes) - 1
		m.fileList.SetSelected(m.selectedFile)
		if len(m.changes) > 0 {
			m.diffView.SetFileChange(m.changes[m.selectedFile])
		}
		return m, nil

	case "r":
		return m, m.loadDiff()
	}

	return m, nil
}

func (m Model) handleNavigation(delta int) (tea.Model, tea.Cmd) {
	if m.focusedPanel == PanelFileList {
		newIdx := m.selectedFile + delta
		if newIdx >= 0 && newIdx < len(m.changes) {
			m.selectedFile = newIdx
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

	fileListView := m.fileList.View(fileListWidth, contentHeight, m.focusedPanel == PanelFileList)
	diffViewView := m.diffView.View(diffViewWidth, contentHeight)
	statusBarView := m.statusBar.View(m.width, modeText, m.source, m.destination)

	return fmt.Sprintf("%s│%s\n%s", fileListView, diffViewView, statusBarView)
}

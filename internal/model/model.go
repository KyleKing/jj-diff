// Package model is the Bubble Tea root: it owns the diff, the selection, and every child component,
// and turns each key into an action through the keymap shared with jojo and second-look.
package model

import (
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/keyusage"
	"github.com/kyleking/aragonite/trail"
	"github.com/kyleking/aragonite/tui/coach"
	"github.com/kyleking/aragonite/tui/keys"
	"github.com/kyleking/aragonite/tui/palette"
	"github.com/kyleking/aragonite/tui/pen"

	"github.com/kyleking/jj-diff/internal/components/destpicker"
	"github.com/kyleking/jj-diff/internal/components/diffview"
	"github.com/kyleking/jj-diff/internal/components/filelist"
	"github.com/kyleking/jj-diff/internal/components/searchmodal"
	"github.com/kyleking/jj-diff/internal/components/statusbar"
	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
	"github.com/kyleking/jj-diff/internal/jj"
	"github.com/kyleking/jj-diff/internal/reads"
	"github.com/kyleking/jj-diff/internal/search"
)

// OperatingMode is which of the three entry points the app was started for, which decides what the
// selection means and what writing it does.
type OperatingMode int

// The three ways the app is launched. ModeBrowse is read-only, ModeInteractive moves, splits out, or
// discards selected hunks, and ModeDiffEditor writes the selection back into the trees jj passed.
const (
	ModeBrowse OperatingMode = iota
	ModeInteractive
	ModeDiffEditor
	// ModeResolve is the conflict viewer, which reads the revision's conflicted files rather than
	// its diff and writes picks with jj resolve.
	ModeResolve
)

// FocusedPanel is which of the two panes takes navigation keys.
type FocusedPanel int

// The two panes Tab switches between. PanelFileList is the zero value, so a fresh model starts on the
// file list.
const (
	PanelFileList FocusedPanel = iota
	PanelDiffView
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
	// The focused file list is as tall as its files, capped at a third of the terminal.
	maxFileListFraction     = 3
	minFileListHeight       = 3
	collapsedFileListHeight = 1
	// Every pane keeps paneMargin columns clear on the left, and the chrome between them does too.
	paneMargin = 2
)

// helpPageStep is how far a page key moves a list that sizes itself to the terminal and so has no
// page height of its own: the destination picker and the conflict viewer's scroll.
const helpPageStep = 10

// Key names the text inputs branch on.
const (
	keyBackspace = "backspace"
	keyCtrlC     = "ctrl+c"
	keyDown      = "down"
	keyEnter     = "enter"
	keyEsc       = "esc"
)

// Sentinel errors for state, rather than jj or the filesystem, blocking an operation.
var (
	errNothingSelected    = errors.New("nothing is selected and the cursor is on no hunk")
	errNotDirectorySource = errors.New("diff-editor mode requires a directory source")
	errNoRevision         = errors.New("this diff is not a revision, so it has nothing to write into")
	errImmutable          = errors.New("jj will not squash into an immutable revision; use -o, -A, or -B")
)

// Model is the whole application state, held by pointer: handlers mutate the receiver in place and
// return only a command. A tea.Cmd runs off the Update loop, so a command factory must snapshot the
// state it needs rather than read the model when the command runs.
type Model struct {
	// searchIndex holds the current search's hits by line, shared by every copy of the model so a
	// frame drawn from a copy builds it once.
	searchIndex *matchIndex
	keymap      *keys.Keymap
	matcher     *keys.Matcher
	// pal is the command palette on : and the legend on ?, one list both read.
	pal *palette.Model
	// coach names the next step and, now and then, a key not yet used.
	coach *coach.Coach[coachState]
	// ledger is where the coach remembers tips and runs across sessions, nil for none.
	ledger *coach.Ledger
	// usage is the opt-in usage log, nil when it is off.
	usage *keyusage.Recorder
	// msgs is the gm command log while it is up.
	msgs *messages
	// built is when the session started, which the coach's warmup counts from.
	built time.Time
	// pending is the selection a picker or prompt opened on, which is what it writes when confirmed
	// whatever moved while it was up.
	pending *SelectionState
	// cols is the split columns while they are up, nil otherwise.
	cols *splitColumns
	// res is the conflict viewer's state in ModeResolve.
	res *resolveState
	// marks is the revision's read marks, which jojo writes and jj-diff shows; nil shows none.
	marks *reads.Marks
	// again repeats the last write on the current selection, for the repeat action.
	again func(m *Model, sel *SelectionState) tea.Cmd
	// dim is the faint the diff pane is drawn in while the file list has focus.
	dim         pen.Pen
	statusBar   statusbar.Model
	diffSource  diff.Source
	err         error
	selection   *SelectionState
	searchState *search.State
	client      *jj.Client
	flags       keys.FlagSet
	destination string
	source      string
	// revision is the revset the diff was read from, empty outside revision mode.
	revision     string
	notice       string
	changes      []diff.FileChange
	undo         []*SelectionState
	redo         []*SelectionState
	motion       keys.Motion
	cfg          config.Config
	picker       destpicker.Model
	searchModal  searchmodal.Model
	fileList     filelist.Model
	diffView     diffview.Model
	focusedPanel FocusedPanel
	lineCursor   int
	selectedHunk int
	selectedFile int
	visualAnchor int
	width        int
	height       int
	mode         OperatingMode
	isVisualMode bool
	alarm        bool
	// quitPrompt is up while q waits for write, discard, or stay.
	quitPrompt bool
	// quitAfterWrite quits once the write a quit prompt started lands.
	quitAfterWrite bool
	// aborted is a diff-editor session ended without writing, which must exit non-zero so jj
	// leaves the revision alone.
	aborted bool
	// startColumns opens the split columns once the diff loads, for --split.
	startColumns bool
	// onlyColumns is a session launched into the split columns, where backing out of them quits
	// rather than dropping to the interactive diff nobody asked for.
	onlyColumns bool
	// band draws the coach's next steps as a band above the footer rather than in its status slot.
	band bool
	// loadedOnce is whether a diff has arrived, before which an empty one means nothing yet.
	loadedOnce bool
	// onlyUnread is U: read hunks draw as their header alone and files read whole leave the list.
	onlyUnread bool
}

type errMsg struct {
	err error
}

type diffLoadedMsg struct {
	changes []diff.FileChange
	// hunks names the changes' hunks for the read marks, set only where marks are shown.
	hunks reads.Hunks
	named bool
}

type revisionsLoadedMsg struct {
	err       error
	start     string
	revisions []jj.RevisionEntry
}

// writtenMsg is a write coming home: what it did in words, or why it failed.
type writtenMsg struct {
	err error
	// call is the jj command the write ran, which the notice names, nil where none did.
	call    *trail.Call
	outcome jj.Outcome
	summary string
}

type diffEditorAppliedMsg struct{}

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
// nil when nothing will be written through jj.
func NewModelWithSource(
	source diff.Source,
	client *jj.Client,
	destination string,
	mode OperatingMode,
	cfg config.Config,
) (Model, error) {
	km := newKeymap(mode, false)

	m := Model{
		client:       client,
		diffSource:   source,
		mode:         mode,
		source:       source.GetSourceLabel(),
		destination:  destination,
		cfg:          cfg,
		focusedPanel: PanelFileList,
		width:        defaultTerminalWidth,
		height:       defaultTerminalHeight,
		selection:    NewSelectionState(),
		keymap:       km,
		matcher:      keys.NewMatcher(km),
		flags:        keys.NewFlagSet(keys.VerbSquash),
		built:        time.Now(),
		band:         cfg.CoachLayout == coach.Band,
	}

	if revision, ok := diff.RevisionOf(source); ok {
		m.revision = revision
	}

	m.dim = pen.Of(lipgloss.NewStyle().Faint(true))
	m.searchIndex = &matchIndex{}
	m.fileList = filelist.New()
	m.diffView = diffview.New(cfg)
	m.statusBar = statusbar.New()
	m.picker = destpicker.New()
	m.discover()
	m.searchModal = searchmodal.New()
	m.searchState = search.NewState()

	return m, nil
}

// StartInColumns opens the split columns as soon as the diff loads, with every change in column 1.
func (m *Model) StartInColumns() { m.startColumns, m.onlyColumns = true, true }

// Aborted reports a diff-editor session that ended without writing. The program exits non-zero for
// it, which is what tells jj to leave the revision as it was.
func (m *Model) Aborted() bool { return m.aborted }

// Init starts the first diff load. Nothing is rendered until it returns and a window size arrives.
func (m *Model) Init() tea.Cmd {
	if m.mode == ModeResolve {
		return m.loadConflicts()
	}

	return m.loadDiff()
}

// loadDiff reads the diff, and where read marks are shown the change id they are scoped by beside
// it, so neither jj call waits on the other and the first frame waits on neither's marks.
func (m *Model) loadDiff() tea.Cmd {
	source, named := m.diffSource, m.marks.On()

	load := func() tea.Msg {
		diffText, err := source.GetDiff()
		if err != nil {
			return errMsg{err}
		}

		changes := diff.Parse(diffText)
		if !named {
			return diffLoadedMsg{changes: changes}
		}

		return diffLoadedMsg{changes: changes, hunks: reads.Map(diffText, changes), named: true}
	}

	if !named {
		return load
	}

	return tea.Batch(load, m.loadScope())
}

// Update handles one message in place and returns the receiver, so the concrete type is always
// *Model and callers chaining updates can assert it. A file the diff pane loaded is lexed in the
// background, so every update also hands out the wait for its colors, once per file; the message
// that wait answers needs no handling, since the redraw after it is what draws them.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(diffview.HighlightedMsg); ok {
		return m, nil
	}

	model, cmd := m.handle(msg)
	m.syncReads()

	if lit := m.diffView.HighlightCmd(); lit != nil {
		cmd = tea.Batch(cmd, lit)
	}

	return model, cmd
}

// fullMargins gives a resized terminal back its whole height to scroll in, then repaints. The
// renderer confines a scroll with DECSTBM and restores the margins by number, as the height it last
// drew at. A frame flushed just before the resize reaches the terminal after it, so a window that
// grew keeps the old height as its bottom margin, and every newline the repaint writes at that row
// scrolls the page up instead of moving down: the page sits rows too high, its footer stranded at
// the old bottom, until something redraws it. The margins are reset without numbers, which means the
// whole screen at any size, around a saved cursor since DECSTBM homes it, and the repaint follows
// because the one the resize asked for may already have drawn inside the stale margins.
func fullMargins() tea.Cmd {
	return tea.Sequence(
		tea.Raw(ansi.SaveCursor+ansi.SetTopBottomMargins(0, 0)+ansi.RestoreCursor),
		tea.ClearScreen,
	)
}

func (m *Model) handle(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		cmd := m.handleKeyPress(msg)

		return m, cmd

	case palette.RunMsg, palette.RefusedMsg, coach.IdleMsg, tea.PasteMsg:
		cmd, _ := m.discoverMsg(msg)

		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		return m, fullMargins()

	case diffLoadedMsg:
		m.loaded(msg.changes)

		if msg.named {
			m.marks.SetHunks(msg.hunks)
		}

	case scopeMsg:
		if msg.err == nil {
			m.marks.SetScope(msg.scope)
		}

		if msg.migrate != nil {
			m.say("older read marks were not moved: "+msg.migrate.Error(), false)
		}

	case errMsg:
		m.err = msg.err

	case revisionsLoadedMsg:
		m.openPicker(msg)

	case writtenMsg:
		cmd := m.written(msg)

		return m, cmd

	case diffEditorAppliedMsg:
		return m, tea.Quit

	case editedMsg:
		m.edited(msg)

	case conflictsLoadedMsg:
		m.conflictsLoaded(msg)
	}

	return m, nil
}

// loaded takes a fresh diff. Hunk indices from the last one mean nothing in it, so the selection and
// its history start over, and the cursors are kept only where they still point at something.
func (m *Model) loaded(changes []diff.FileChange) {
	m.changes = changes
	m.loadedOnce = true
	m.fileList.SetFiles(m.changes)
	m.selection = NewSelectionState()
	m.undo, m.redo = nil, nil
	m.isVisualMode = false

	m.selectedFile = min(m.selectedFile, max(len(m.changes)-1, 0))
	m.selectedHunk, m.lineCursor = 0, 0

	if len(m.changes) > 0 {
		m.fileList.SetSelected(m.selectedFile)
		m.diffView.SetFileChange(m.changes[m.selectedFile])
	}

	// jj's diff editor contract is subtractive: the right side starts as the commit's full content
	// and the user removes what should not be kept. Starting empty here would discard every change
	// on confirm.
	if m.mode == ModeDiffEditor {
		diff.SelectAll(m.changes, m.selection)
	}

	// Line indices from the last diff mean nothing in this one, so the columns close with it.
	m.cols = nil

	if m.startColumns && len(m.changes) > 0 {
		m.startColumns = false
		m.openColumns(nil)
	}
}

// say puts a notice in the footer until the next key.
func (m *Model) say(text string, alarm bool) {
	m.notice, m.alarm = text, alarm
}

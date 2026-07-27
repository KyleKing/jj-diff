# Documentation-Driven Testing Implementation Plan: jj-diff

Implementation roadmap for adding fixture-based testing and auto-generated documentation to jj-diff.

## Current Architecture Analysis

**Existing patterns:**
- Bubble Tea model with `Update(msg tea.Msg)` pattern
- Modal states: `ModeBrowse`, `ModeInteractive`, `ModeDiffEditor`
- Key handling in `handleKeyPress(msg tea.KeyMsg)`
- Component-based architecture (filelist, diffview, statusbar, etc.)
- Selection state management (`SelectionState`, `MultiSplitState`)

**Testing gaps:**
- Tests coupled to keyboard simulation (see `internal/app/app_test.go`)
- State assertions after key presses, not semantic actions
- No documentation generated from tests
- Hard to replay or script operations

**Current state:** ~70% compatible with command-based architecture

## Goals

1. **Testability**: Test semantic actions without keyboard simulation
2. **Documentation**: Auto-generate usage docs from test fixtures
3. **Replayability**: Record and replay command sequences
4. **Scriptability**: Command mode for power users (`:move`, `:tag`, `:split`)

## Phase 1: Command Abstraction (Week 1)

### 1.1 Define Command Interface

**File:** `internal/command/command.go`

```go
package command

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/jj-diff/internal/model"
)

// Command represents a semantic action independent of input method
type Command interface {
	Execute(m *model.Model) (tea.Cmd, error)
	String() string           // For logging, replay, documentation
	Undo() (Command, error)   // Return inverse command if reversible
}

// CommandMsg wraps a command for bubble tea message passing
type CommandMsg struct {
	Cmd Command
}
```

### 1.2 Implement Core Commands

**File:** `internal/command/commands.go`

```go
// Navigation commands
type NextFileCmd struct{}
type PrevFileCmd struct{}
type NextHunkCmd struct{}
type PrevHunkCmd struct{}
type SwitchPanelCmd struct{}

// Selection commands
type ToggleHunkSelectionCmd struct {
	FilePath string
	HunkIdx  int
}

type ToggleLineSelectionCmd struct {
	FilePath string
	HunkIdx  int
	LineIdx  int
}

type EnterVisualModeCmd struct{}
type ExitVisualModeCmd struct{}

// Interactive mode commands
type SelectDestinationCmd struct{}

type SetDestinationCmd struct {
	ChangeID string
}

type ApplySelectionsCmd struct {
	Destination string
}

// Multi-split commands
type ToggleMultiSplitCmd struct{}

type TagHunkCmd struct {
	Tag      rune
	FilePath string
	HunkIdx  int
}

type AssignTagsCmd struct {
	Assignments map[rune]string // tag -> changeID
}

type PreviewSplitCmd struct{}
type ApplySplitCmd struct{}

// Modal commands
type ShowHelpCmd struct{}
type HideHelpCmd struct{}
type ShowSearchCmd struct{}
type ShowFileFinderCmd struct{}

// Search commands
type SearchNextCmd struct{}
type SearchPrevCmd struct{}
```

### 1.3 Extract Command Execution from Key Handlers

**File:** `internal/model/model.go`

Refactor `handleKeyPress` to map keys to commands:

```go
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Modal handling (unchanged for now)
	if key == "esc" {
		// ... existing modal close logic
	}

	// Map key to command
	cmd := m.keyToCommand(key)
	if cmd == nil {
		return m, nil
	}

	// Execute command
	teaCmd, err := cmd.Execute(&m)
	if err != nil {
		m.err = err
	}

	// Log command for replay
	m.commandLog = append(m.commandLog, cmd)

	return m, teaCmd
}

func (m Model) keyToCommand(key string) command.Command {
	// Handle modal-specific keys
	if m.help.IsVisible() {
		return m.helpKeyToCommand(key)
	}
	if m.destPicker.IsVisible() {
		return m.destPickerKeyToCommand(key)
	}
	// ... other modals

	// Main mode keys
	switch m.mode {
	case ModeBrowse:
		return m.browseKeyToCommand(key)
	case ModeInteractive:
		return m.interactiveKeyToCommand(key)
	case ModeDiffEditor:
		return m.diffEditorKeyToCommand(key)
	}

	return nil
}

func (m Model) browseKeyToCommand(key string) command.Command {
	switch key {
	case "j", "down":
		if m.focusedPanel == PanelFileList {
			return &command.NextFileCmd{}
		}
		return &command.ScrollDownCmd{}
	case "k", "up":
		if m.focusedPanel == PanelFileList {
			return &command.PrevFileCmd{}
		}
		return &command.ScrollUpCmd{}
	case "n":
		return &command.NextHunkCmd{}
	case "p":
		return &command.PrevHunkCmd{}
	case "tab":
		return &command.SwitchPanelCmd{}
	case "?":
		return &command.ShowHelpCmd{}
	case "/":
		return &command.ShowSearchCmd{}
	case "f":
		return &command.ShowFileFinderCmd{}
	case "q":
		return &command.QuitCmd{}
	default:
		return nil
	}
}

func (m Model) interactiveKeyToCommand(key string) command.Command {
	// Include browse commands
	if cmd := m.browseKeyToCommand(key); cmd != nil {
		return cmd
	}

	switch key {
	case "d":
		return &command.SelectDestinationCmd{}
	case " ", "space":
		return &command.ToggleHunkSelectionCmd{
			FilePath: m.getCurrentFilePath(),
			HunkIdx:  m.getCurrentHunkIdx(),
		}
	case "v":
		return &command.EnterVisualModeCmd{}
	case "a":
		return &command.ApplySelectionsCmd{
			Destination: m.destination,
		}
	case "S":
		return &command.ToggleMultiSplitCmd{}
	default:
		if m.multiSplit.Active && key >= "a" && key <= "z" {
			return &command.TagHunkCmd{
				Tag:      rune(key[0]),
				FilePath: m.getCurrentFilePath(),
				HunkIdx:  m.getCurrentHunkIdx(),
			}
		}
		return nil
	}
}
```

### 1.4 Add Direct Command Execution

Support sending commands directly (for testing):

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case command.CommandMsg:
		teaCmd, err := msg.Cmd.Execute(&m)
		if err != nil {
			m.err = err
		}
		return m, teaCmd

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	// ... rest of message handling
	}

	return m, nil
}
```

**Deliverable:** All key handlers refactored to commands. Tests can send `CommandMsg` directly.

## Phase 2: Serializable State (Week 2)

### 2.1 Define AppState (Serializable)

**File:** `internal/state/appstate.go`

```go
package state

// AppState contains all serializable application state
// Used for fixtures, snapshots, and command replay
type AppState struct {
	// Mode and focus
	Mode         string `json:"mode"`          // "browse", "interactive", "diff-editor"
	FocusedPanel string `json:"focused_panel"` // "filelist", "diffview"

	// Navigation
	SelectedFile int `json:"selected_file"`
	SelectedHunk int `json:"selected_hunk"`

	// Interactive mode
	Destination string                   `json:"destination,omitempty"`
	Selections  map[string][]HunkSelect  `json:"selections,omitempty"`

	// Multi-split mode
	MultiSplitActive bool                       `json:"multi_split_active"`
	SplitTags        map[string][]TaggedHunk    `json:"split_tags,omitempty"`
	SplitAssignments map[string]string          `json:"split_assignments,omitempty"`

	// Visual mode
	VisualMode   bool `json:"visual_mode"`
	VisualAnchor int  `json:"visual_anchor,omitempty"`

	// Search
	SearchActive bool   `json:"search_active"`
	SearchQuery  string `json:"search_query,omitempty"`
}

type HunkSelect struct {
	HunkIdx       int   `json:"hunk_idx"`
	WholeHunk     bool  `json:"whole_hunk"`
	SelectedLines []int `json:"selected_lines,omitempty"`
}

type TaggedHunk struct {
	Tag      string `json:"tag"`
	FilePath string `json:"file_path"`
	HunkIdx  int    `json:"hunk_idx"`
}

func (a *AppState) ToJSON() ([]byte, error)
func (a *AppState) FromJSON(data []byte) error
```

### 2.2 Separate UIState (Ephemeral)

**File:** `internal/state/uistate.go`

```go
package state

import (
	"github.com/kyleking/jj-diff/internal/components/diffview"
	"github.com/kyleking/jj-diff/internal/components/filelist"
	// ... other components
)

// UIState contains ephemeral UI state (not serialized)
type UIState struct {
	// Window dimensions
	Width  int
	Height int

	// Component instances
	FileList     *filelist.Model
	DiffView     *diffview.Model
	StatusBar    *statusbar.Model
	Help         *help.Model
	DestPicker   *destpicker.Model
	SearchModal  *searchmodal.Model
	FileFinder   *filefinder.Model
	SplitAssign  *splitassign.Model
	SplitPreview *splitpreview.Model
	CommitMsg    *commitmsg.Model

	// Theme
	Theme *theme.Theme
}
```

### 2.3 Refactor Model to Use AppState/UIState

**File:** `internal/model/model.go`

```go
type Model struct {
	App   state.AppState
	UI    state.UIState

	// Reference data (not part of state)
	changes     []diff.FileChange
	client      *jj.Client
	config      *config.Config
	commandLog  []command.Command
	err         error
}

// State serialization helpers
func (m Model) ToState() state.AppState {
	return m.App
}

func (m Model) FromState(s state.AppState) Model {
	m.App = s
	return m
}

func (m Model) SaveSnapshot(path string) error {
	data, err := m.App.ToJSON()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (m Model) LoadSnapshot(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return m.App.FromJSON(data)
}
```

**Deliverable:** Model state is fully serializable. Can snapshot/restore app state for testing.

## Phase 3: Fixture-Based Testing (Week 3)

### 3.1 Define Fixture Format

**File:** `internal/fixtures/schema.go`

```go
package fixtures

import (
	"github.com/kyleking/jj-diff/internal/command"
	"github.com/kyleking/jj-diff/internal/state"
)

type Fixture struct {
	Name     string              `yaml:"name"`
	Mode     string              `yaml:"mode"`     // browse, interactive, diff-editor
	Before   state.AppState      `yaml:"before"`
	Commands []CommandSpec       `yaml:"commands"`
	After    state.AppState      `yaml:"after"`
	Expect   ExpectedOutcome     `yaml:"expect"`
	DiffData *DiffDataSpec       `yaml:"diff_data,omitempty"`
}

type CommandSpec struct {
	Type   string                 `yaml:"type"`   // "next_file", "toggle_hunk", etc.
	Params map[string]interface{} `yaml:"params,omitempty"`
}

type ExpectedOutcome struct {
	JJCommands []string `yaml:"jj_commands,omitempty"`
	Error      string   `yaml:"error,omitempty"`
	FileStates []string `yaml:"file_states,omitempty"`
}

type DiffDataSpec struct {
	Files []FileSpec `yaml:"files"`
}

type FileSpec struct {
	Path  string      `yaml:"path"`
	Hunks []HunkSpec  `yaml:"hunks"`
}

type HunkSpec struct {
	OldStart int      `yaml:"old_start"`
	OldLines int      `yaml:"old_lines"`
	NewStart int      `yaml:"new_start"`
	NewLines int      `yaml:"new_lines"`
	Lines    []string `yaml:"lines"`
}

func LoadFixtures(path string) ([]Fixture, error)
func (f *Fixture) ToCommands() ([]command.Command, error)
```

### 3.2 Create Example Fixtures

**File:** `internal/fixtures/browse_mode.yaml`

```yaml
- name: "Navigate to next file"
  mode: "browse"
  before:
    mode: "browse"
    focused_panel: "filelist"
    selected_file: 0
  commands:
    - type: "next_file"
  after:
    selected_file: 1
  diff_data:
    files:
      - path: "src/auth.go"
        hunks: [{old_start: 10, old_lines: 5, new_start: 10, new_lines: 7}]
      - path: "src/handler.go"
        hunks: [{old_start: 20, old_lines: 3, new_start: 20, new_lines: 4}]

- name: "Navigate to next hunk"
  mode: "browse"
  before:
    mode: "browse"
    focused_panel: "diffview"
    selected_file: 0
    selected_hunk: 0
  commands:
    - type: "next_hunk"
  after:
    selected_hunk: 1

- name: "Search for pattern"
  mode: "browse"
  before:
    mode: "browse"
    focused_panel: "diffview"
    selected_file: 0
    selected_hunk: 0
  commands:
    - type: "show_search"
    - type: "search_query"
      params:
        query: "TODO"
    - type: "search_next"
  after:
    search_active: true
    search_query: "TODO"
    selected_hunk: 2  # First hunk containing "TODO"
```

**File:** `internal/fixtures/interactive_mode.yaml`

```yaml
- name: "Select single hunk and move to parent"
  mode: "interactive"
  before:
    mode: "interactive"
    selected_file: 0
    selected_hunk: 1
    destination: ""
  commands:
    - type: "set_destination"
      params:
        change_id: "@-"
    - type: "toggle_hunk_selection"
      params:
        file_path: "src/auth.go"
        hunk_idx: 1
    - type: "apply_selections"
  after:
    selections: {}
  expect:
    jj_commands:
      - "jj move --from @ --to @- <hunk_content>"

- name: "Line-level selection with visual mode"
  mode: "interactive"
  before:
    mode: "interactive"
    selected_file: 0
    selected_hunk: 0
    destination: "@-"
  commands:
    - type: "enter_visual_mode"
    - type: "toggle_line_selection"
      params:
        file_path: "src/auth.go"
        hunk_idx: 0
        line_idx: 2
    - type: "toggle_line_selection"
      params:
        file_path: "src/auth.go"
        hunk_idx: 0
        line_idx: 3
    - type: "exit_visual_mode"
    - type: "apply_selections"
  after:
    visual_mode: false
    selections: {}
  expect:
    jj_commands:
      - "jj move --from @ --to @- <lines_2-3>"

- name: "Multi-split with tags"
  mode: "interactive"
  before:
    mode: "interactive"
    multi_split_active: false
  commands:
    - type: "toggle_multi_split"
    - type: "tag_hunk"
      params:
        tag: "a"
        file_path: "src/ui.go"
        hunk_idx: 0
    - type: "tag_hunk"
      params:
        tag: "b"
        file_path: "src/logic.go"
        hunk_idx: 0
    - type: "assign_tags"
      params:
        assignments:
          a: "feat-ui"
          b: "feat-logic"
    - type: "apply_split"
  after:
    multi_split_active: true
  expect:
    jj_commands:
      - "jj new @ -m 'feat-ui'"
      - "jj move --from @ --to feat-ui <hunk_a>"
      - "jj new @ -m 'feat-logic'"
      - "jj move --from @ --to feat-logic <hunk_b>"
```

### 3.3 Implement Test Harness

**File:** `internal/fixtures/harness_test.go`

```go
package fixtures_test

import (
	"testing"

	"github.com/kyleking/jj-diff/internal/fixtures"
	"github.com/kyleking/jj-diff/internal/model"
	"github.com/kyleking/jj-diff/internal/test/mocks"
)

func TestFixtures(t *testing.T) {
	fixtureFiles := []string{
		"browse_mode.yaml",
		"interactive_mode.yaml",
		"diff_editor_mode.yaml",
	}

	for _, file := range fixtureFiles {
		fixtures, err := fixtures.LoadFixtures(file)
		if err != nil {
			t.Fatalf("Failed to load %s: %v", file, err)
		}

		for _, fx := range fixtures {
			t.Run(fx.Name, func(t *testing.T) {
				runFixture(t, fx)
			})
		}
	}
}

func runFixture(t *testing.T, fx fixtures.Fixture) {
	// Create test model with mock jj client
	mockClient := mocks.NewMockJJClient()
	m := model.New(mockClient, nil)

	// Load fixture state
	m = m.FromState(fx.Before)

	// Load fixture diff data
	if fx.DiffData != nil {
		m.SetChanges(fx.DiffData.ToFileChanges())
	}

	// Execute commands
	commands, err := fx.ToCommands()
	if err != nil {
		t.Fatalf("Failed to convert commands: %v", err)
	}

	for i, cmd := range commands {
		_, err := cmd.Execute(&m)
		if fx.Expect.Error != "" {
			if err == nil {
				t.Fatalf("Command %d expected error, got none", i)
			}
			if err.Error() != fx.Expect.Error {
				t.Errorf("Command %d error mismatch:\nGot:  %s\nWant: %s",
					i, err.Error(), fx.Expect.Error)
			}
			return
		}
		if err != nil {
			t.Fatalf("Command %d failed: %v", i, err)
		}
	}

	// Verify final state
	actual := m.ToState()
	if !statesEqual(actual, fx.After) {
		t.Errorf("State mismatch:\nGot:  %+v\nWant: %+v", actual, fx.After)
	}

	// Verify jj commands
	if len(fx.Expect.JJCommands) > 0 {
		actualCmds := mockClient.GetExecutedCommands()
		if !commandsEqual(actualCmds, fx.Expect.JJCommands) {
			t.Errorf("JJ commands mismatch:\nGot:  %v\nWant: %v",
				actualCmds, fx.Expect.JJCommands)
		}
	}
}

func statesEqual(a, b state.AppState) bool {
	// Deep comparison of states
}

func commandsEqual(a, b []string) bool {
	// Compare command lists
}
```

**Deliverable:** Fixture-based test suite running. Tests verify behavior without keyboard simulation.

## Phase 4: Documentation Generation (Week 4)

### 4.1 Build Doc Generator

**File:** `internal/fixtures/docgen.go`

```go
package fixtures

import (
	"fmt"
	"strings"
)

type DocGenerator struct {
	fixtures []Fixture
}

func NewDocGenerator(fixtures []Fixture) *DocGenerator {
	return &DocGenerator{fixtures: fixtures}
}

func (g *DocGenerator) GenerateMarkdown() string {
	var out strings.Builder

	out.WriteString("# jj-diff Usage Guide\n\n")
	out.WriteString("Auto-generated from test fixtures.\n\n")

	// Group by mode
	byMode := g.groupByMode()

	for _, mode := range []string{"browse", "interactive", "diff-editor"} {
		fixtures := byMode[mode]
		if len(fixtures) == 0 {
			continue
		}

		out.WriteString(fmt.Sprintf("## %s Mode\n\n", strings.Title(mode)))

		for _, fx := range fixtures {
			g.writeFixture(&out, fx)
		}
	}

	return out.String()
}

func (g *DocGenerator) writeFixture(out *strings.Builder, fx Fixture) {
	out.WriteString(fmt.Sprintf("### %s\n\n", fx.Name))

	// Write command sequence
	out.WriteString("**Actions:**\n")
	for i, cmd := range fx.Commands {
		cmdStr := g.commandToString(cmd)
		out.WriteString(fmt.Sprintf("%d. %s\n", i+1, cmdStr))
	}
	out.WriteString("\n")

	// Write state changes
	changes := g.diffStates(fx.Before, fx.After)
	if len(changes) > 0 {
		out.WriteString("**Changes:**\n")
		for _, change := range changes {
			out.WriteString(fmt.Sprintf("- %s: `%v` → `%v`\n",
				change.Field, change.Before, change.After))
		}
		out.WriteString("\n")
	}

	// Write expected outcome
	if len(fx.Expect.JJCommands) > 0 {
		out.WriteString("**Result:**\n```bash\n")
		for _, cmd := range fx.Expect.JJCommands {
			out.WriteString(cmd + "\n")
		}
		out.WriteString("```\n\n")
	}

	// Write keyboard shortcuts (if available)
	keys := g.commandsToKeys(fx.Commands)
	if len(keys) > 0 {
		out.WriteString("**Keyboard:**\n```\n")
		out.WriteString(strings.Join(keys, " → "))
		out.WriteString("\n```\n\n")
	}
}

func (g *DocGenerator) commandToString(cmd CommandSpec) string {
	switch cmd.Type {
	case "next_file":
		return "Navigate to next file"
	case "toggle_hunk_selection":
		return fmt.Sprintf("Select hunk %d in %s",
			cmd.Params["hunk_idx"], cmd.Params["file_path"])
	case "set_destination":
		return fmt.Sprintf("Set destination to %s", cmd.Params["change_id"])
	case "apply_selections":
		return "Apply changes"
	// ... more cases
	default:
		return cmd.Type
	}
}

func (g *DocGenerator) diffStates(before, after state.AppState) []StateChange {
	// Compare before/after states, return list of changes
}

func (g *DocGenerator) commandsToKeys(commands []CommandSpec) []string {
	// Map commands back to keyboard shortcuts
	keyMap := map[string]string{
		"next_file":             "j",
		"prev_file":             "k",
		"next_hunk":             "n",
		"prev_hunk":             "p",
		"toggle_hunk_selection": "Space",
		"apply_selections":      "a",
		// ... more mappings
	}

	var keys []string
	for _, cmd := range commands {
		if key, ok := keyMap[cmd.Type]; ok {
			keys = append(keys, key)
		}
	}
	return keys
}

type StateChange struct {
	Field  string
	Before interface{}
	After  interface{}
}
```

### 4.2 Generate Documentation

**File:** `cmd/gendocs/main.go`

```go
package main

import (
	"fmt"
	"os"

	"github.com/kyleking/jj-diff/internal/fixtures"
)

func main() {
	fixtureFiles := []string{
		"internal/fixtures/browse_mode.yaml",
		"internal/fixtures/interactive_mode.yaml",
		"internal/fixtures/diff_editor_mode.yaml",
	}

	var allFixtures []fixtures.Fixture
	for _, file := range fixtureFiles {
		fx, err := fixtures.LoadFixtures(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load %s: %v\n", file, err)
			os.Exit(1)
		}
		allFixtures = append(allFixtures, fx...)
	}

	gen := fixtures.NewDocGenerator(allFixtures)
	markdown := gen.GenerateMarkdown()

	outputPath := "docs/USAGE.md"
	if err := os.WriteFile(outputPath, []byte(markdown), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write %s: %v\n", outputPath, err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s from %d fixtures\n", outputPath, len(allFixtures))
}
```

### 4.3 Integrate into CI

**File:** `.github/workflows/docs.yml`

```yaml
name: Generate Documentation

on:
  push:
    branches: [main]
    paths:
      - 'internal/fixtures/**'

jobs:
  docs:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Generate docs
        run: go run cmd/gendocs/main.go

      - name: Commit generated docs
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add docs/USAGE.md
          git diff --staged --quiet || git commit -m "docs: regenerate from fixtures"
          git push
```

**Deliverable:** Documentation auto-generated from fixtures. CI keeps docs in sync with tests.

## Phase 5: Command Mode (Week 5 - Optional)

### 5.1 Add Command Parser

**File:** `internal/cmdmode/parser.go`

```go
package cmdmode

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kyleking/jj-diff/internal/command"
)

type Parser struct{}

func New() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(input string) (command.Command, error) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmdName := parts[0]
	args := parts[1:]

	switch cmdName {
	case "move":
		return p.parseMoveCmd(args)
	case "tag":
		return p.parseTagCmd(args)
	case "split":
		return p.parseSplitCmd(args)
	case "select":
		return p.parseSelectCmd(args)
	case "search":
		return p.parseSearchCmd(args)
	case "help":
		return &command.ShowHelpCmd{}, nil
	case "quit", "q":
		return &command.QuitCmd{}, nil
	default:
		return nil, fmt.Errorf("unknown command: %s", cmdName)
	}
}

func (p *Parser) parseMoveCmd(args []string) (command.Command, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("usage: move <destination>")
	}

	return &command.ApplySelectionsCmd{
		Destination: args[0],
	}, nil
}

func (p *Parser) parseTagCmd(args []string) (command.Command, error) {
	if len(args) != 1 || len(args[0]) != 1 {
		return nil, fmt.Errorf("usage: tag <a-z>")
	}

	tag := rune(args[0][0])
	if tag < 'a' || tag > 'z' {
		return nil, fmt.Errorf("tag must be a-z")
	}

	// Tag current hunk
	return &command.TagCurrentHunkCmd{Tag: tag}, nil
}

func (p *Parser) parseSplitCmd(args []string) (command.Command, error) {
	// Parse: split feat:ui test:unit
	// Creates assignments: a -> feat:ui, b -> test:unit

	assignments := make(map[rune]string)
	tag := 'a'

	for _, arg := range args {
		parts := strings.SplitN(arg, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid split format: %s (expected name:desc)", arg)
		}

		assignments[tag] = parts[0]
		tag++
	}

	return &command.AssignTagsCmd{Assignments: assignments}, nil
}
```

### 5.2 Add Command Mode UI

**File:** `internal/components/cmdmode/cmdmode.go`

```go
package cmdmode

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	input   textinput.Model
	visible bool
	err     error
}

func New() Model {
	ti := textinput.New()
	ti.Prompt = ":"
	ti.Width = 80

	return Model{
		input:   ti,
		visible: false,
	}
}

func (m Model) IsVisible() bool {
	return m.visible
}

func (m Model) Show() Model {
	m.visible = true
	m.input.Focus()
	m.input.SetValue("")
	return m
}

func (m Model) Hide() Model {
	m.visible = false
	m.input.Blur()
	return m
}

func (m Model) Value() string {
	return m.input.Value()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) View(width int) string {
	if !m.visible {
		return ""
	}

	style := lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		Background(lipgloss.Color("#1e1e2e"))

	var content string
	if m.err != nil {
		content = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f38ba8")).
			Render(m.err.Error())
	} else {
		content = m.input.View()
	}

	return style.Render(content)
}
```

### 5.3 Integrate Command Mode

**File:** `internal/model/model.go`

```go
type Model struct {
	// ... existing fields

	cmdMode *cmdmode.Model
	cmdParser *cmdmode.Parser
}

func New(client *jj.Client, cfg *config.Config) Model {
	return Model{
		// ... existing initialization

		cmdMode: cmdmode.New(),
		cmdParser: cmdmode.New(),
	}
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Command mode handling
	if m.cmdMode.IsVisible() {
		return m.handleCommandMode(msg)
	}

	// Enter command mode
	if key == ":" {
		m.cmdMode = m.cmdMode.Show()
		return m, nil
	}

	// ... rest of key handling
}

func (m Model) handleCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter":
		cmdStr := m.cmdMode.Value()
		cmd, err := m.cmdParser.Parse(cmdStr)
		if err != nil {
			m.cmdMode.SetError(err)
			return m, nil
		}

		m.cmdMode = m.cmdMode.Hide()

		teaCmd, err := cmd.Execute(&m)
		if err != nil {
			m.err = err
		}
		return m, teaCmd

	case "esc":
		m.cmdMode = m.cmdMode.Hide()
		return m, nil

	default:
		var cmd tea.Cmd
		m.cmdMode, cmd = m.cmdMode.Update(msg)
		return m, cmd
	}
}
```

**Deliverable:** Command mode (`:move`, `:tag`, `:split`) functional. Power users can script operations.

## Phase 6: Polish & Documentation (Week 6)

### 6.1 Add Fixture Coverage

Create comprehensive fixture sets:

- **Browse mode**: Navigation, search, file finder, help (10-15 fixtures)
- **Interactive mode**: Selection, visual mode, destination picking, applying (15-20 fixtures)
- **Multi-split mode**: Tagging, assignment, preview, apply (10-15 fixtures)
- **Diff-editor mode**: Integration with jj commands (5-10 fixtures)
- **Error cases**: Invalid selections, missing destination, empty diffs (10 fixtures)

**Target:** 50-70 total fixtures covering all major workflows.

### 6.2 Documentation Structure

Generate multiple doc files:

```
docs/
├── USAGE.md              # Auto-generated from fixtures
├── KEYBINDINGS.md        # Auto-generated keyboard reference
├── COMMAND_MODE.md       # Command mode reference
├── ARCHITECTURE.md       # Manual architecture doc
└── TESTING.md            # Testing guide
```

**USAGE.md structure:**

```markdown
# jj-diff Usage Guide

Auto-generated from test fixtures.

## Browse Mode

### Navigate to next file
**Keyboard:** `j`
**Actions:**
1. Navigate to next file

**Changes:**
- selected_file: `0` → `1`

### Search for pattern
**Keyboard:** `/` → `TODO` → `n`
**Actions:**
1. Show search modal
2. Enter query: "TODO"
3. Navigate to next match

**Changes:**
- search_active: `false` → `true`
- selected_hunk: `0` → `2`

## Interactive Mode

### Select single hunk and move to parent
**Keyboard:** `d` → `@-` → `Space` → `a`
**Actions:**
1. Set destination to @-
2. Select hunk 1 in src/auth.go
3. Apply changes

**Result:**
```bash
jj move --from @ --to @- <hunk_content>
```

<!-- ... more fixtures -->
```

### 6.3 Update Main README

Add documentation-driven testing section to main README:

```markdown
## Documentation

jj-diff uses documentation-driven testing. All usage examples are executable test fixtures.

See:
- [Usage Guide](docs/USAGE.md) - Generated from test fixtures
- [Keybindings](docs/KEYBINDINGS.md) - Complete keyboard reference
- [Command Mode](docs/COMMAND_MODE.md) - Power user commands

To regenerate documentation:

```bash
go run cmd/gendocs/main.go
```

## Testing

Run fixture-based tests:

```bash
go test ./internal/fixtures/...
```

Add new workflows by creating fixtures in `internal/fixtures/*.yaml`.
```

**Deliverable:** Complete documentation generated from fixtures. README updated.

## Success Metrics

Track the following to measure implementation success:

**Test coverage:**
- Fixture count: Target 50-70 fixtures
- Command coverage: All commands tested
- Mode coverage: Browse, interactive, diff-editor, multi-split

**Documentation accuracy:**
- Zero stale examples (guaranteed by fixture-driven approach)
- All keybindings documented with examples
- Command mode reference complete

**Developer experience:**
- New features include fixtures by default
- Fixtures updated more often than prose docs
- Bug reports reference fixture numbers

**User adoption:**
- Command mode usage metrics (if telemetry added)
- User requests for new commands vs new shortcuts
- Issue reports include command sequences

## Migration Strategy

**Incremental approach:**

1. **Week 1-2**: Add command abstraction alongside existing key handlers (both work)
2. **Week 3**: Add fixtures, run alongside existing imperative tests
3. **Week 4**: Generate docs, compare with existing manual docs
4. **Week 5**: Add command mode, mark as experimental
5. **Week 6**: Remove old imperative tests, mark command mode as stable

**Backward compatibility:**

- All existing keyboard shortcuts continue to work
- Existing tests run until fixture coverage is complete
- Documentation reflects both keyboard and command mode

**Rollback plan:**

- Command abstraction layer is isolated in `internal/command/`
- Can revert by removing command layer and restoring direct key handlers
- State serialization can be used independently of commands

## Future Enhancements

**Command recording:**

```go
// Record user session
m.StartRecording("session.jjdiff")

// Replay session
m.ReplayRecording("session.jjdiff")
```

**Macro support:**

```
:record a          # Start recording to register 'a'
<perform actions>
:stop              # Stop recording
@a                 # Replay macro 'a'
```

**Scriptability:**

```bash
# Run jj-diff with command script
jj-diff --script <(echo "
  select src/auth.go:1
  tag a
  select src/handler.go:2
  tag b
  split feat:auth feat:handler
")
```

**Integration with jj:**

```bash
# Custom jj command using jj-diff
jj move-hunk --from @ --to @- --interactive
# Opens jj-diff in interactive mode
```

## Implementation Notes

**Code organization:**

```
internal/
├── command/          # Command interface and implementations
├── state/            # AppState and UIState definitions
├── fixtures/         # Fixture schema, loading, test harness
├── cmdmode/          # Command mode parser and UI
├── model/            # Refactored model using commands
└── components/       # Existing component code (minimal changes)
```

**Testing strategy:**

- Keep existing tests during migration
- Add fixture tests incrementally
- Remove old tests once fixture coverage exceeds old coverage
- Benchmark fixture execution time (should be fast)

**Performance considerations:**

- Command abstraction adds minimal overhead (<1ms per command)
- State serialization should be fast (use efficient JSON library)
- Fixture loading cached during test runs
- Documentation generation is offline (doesn't affect runtime)

**Dependencies:**

- No new dependencies for core functionality
- YAML library for fixtures (gopkg.in/yaml.v3)
- Existing test framework continues to work

## Questions & Decisions

**Q: Should command mode use `:` or `/` prefix?**
A: Use `:` (vim convention). `/` reserved for search.

**Q: How to handle async operations in fixtures?**
A: Commands return `tea.Cmd` which can be awaited in tests. Use synchronous mode for fixture execution.

**Q: Should fixtures include visual output?**
A: No. Fixtures test behavior, not rendering. Use VHS for visual demos.

**Q: How granular should commands be?**
A: Follow single responsibility. `NextFileCmd` not `NavigateCmd{direction}`. Composition happens at fixture level.

**Q: Should state include reference data (files, diffs)?**
A: No. AppState is user actions/choices. Reference data loaded separately in fixtures via `DiffDataSpec`.

## References

- [Testable TUI Patterns](~/.config/nvim/docs/testable-tui-patterns.md)
- [Documentation-Driven Testing](~/.config/nvim/docs/documentation-driven-testing.md)
- [Elm Architecture](https://guide.elm-lang.org/architecture/)
- [catwalk](https://github.com/knz/catwalk) - Similar command-based TUI testing

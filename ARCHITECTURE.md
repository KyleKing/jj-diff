# Architecture Documentation

## System Overview

`jj-diff` is a Terminal User Interface (TUI) application for viewing and interactively manipulating diffs in Jujutsu (jj) version control system. It provides two operational modes:

- **Browse Mode**: Read-only diff viewing with search capabilities
- **Interactive Mode**: Full manipulation including hunk selection, line-level selection, and real-time change movement

### Technology Stack

- **Language**: Go 1.21+
- **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) (The Elm Architecture)
- **Styling**: [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- **VCS Integration**: Jujutsu CLI (via `os/exec`)
- **Theme**: Catppuccin (Latte/Macchiato) with auto-detection

## Architecture Philosophy

### The Elm Architecture (TEA)

The application follows The Elm Architecture pattern as implemented by Bubble Tea:

```
┌─────────┐
│  Model  │  ← Application state (single source of truth)
└─────────┘
     │
     ├──► Update(msg) → (model, cmd)  ← State transitions
     │
     └──► View() → string             ← Render to terminal
```

**Key Principles:**
1. **Immutable Updates**: State changes return new model values
2. **Message-Driven**: All updates triggered by discrete messages
3. **Command Pattern**: Side effects (I/O, timers) expressed as commands
4. **Pure Rendering**: View is deterministic function of state

### Component Composition

Components follow a consistent pattern:

```go
type Model struct {
    // Component state
}

func New() Model { /* initialize */ }
func (m *Model) SetX(x Type) { /* update state */ }
func (m Model) View(width, height int) string { /* render */ }
```

This pattern enables:
- **Encapsulation**: Components manage their own rendering logic
- **Testability**: Components can be tested in isolation
- **Reusability**: Modal components (help, search, destpicker) share common patterns

## Project Structure

```
jj-diff/
├── cmd/jj-diff/main.go          # Entry point, CLI parsing
├── internal/
│   ├── model/                   # Core application state
│   │   ├── model.go            # Main model and update logic
│   │   └── model_test.go       # Model behavior tests
│   ├── jj/                      # Jujutsu integration
│   │   └── client.go           # jj command execution
│   ├── diff/                    # Diff processing
│   │   ├── parser.go           # Unified diff parser
│   │   ├── patch.go            # Patch generation
│   │   └── *_test.go           # Parser and patch tests
│   ├── search/                  # Search functionality
│   │   └── search.go           # Match tracking and navigation
│   ├── components/              # UI components
│   │   ├── filelist/           # File list panel
│   │   ├── diffview/           # Diff viewing panel
│   │   ├── statusbar/          # Status bar
│   │   ├── help/               # Help modal
│   │   ├── destpicker/         # Destination picker modal
│   │   └── searchmodal/        # Search modal
│   └── theme/                   # Color themes
│       ├── theme.go            # Theme definitions
│       └── styles.go           # Global theme state
└── tests/integration/           # End-to-end tests
    └── client_test.go
```

## Core Components

### 1. Model (internal/model/model.go)

**Purpose**: Central orchestrator managing all application state and coordinating components.

**Key Responsibilities:**
- State management for selection, navigation, modes
- Message routing and update logic
- Component coordination
- Integration with jj client

**State Structure:**
```go
type Model struct {
    // Core state
    client       *jj.Client
    mode         Mode              // Browse or Interactive
    source       string            // Source revision
    destination  string            // Destination revision

    // Navigation state
    selectedFile int
    selectedHunk int
    focusedPanel FocusedPanel

    // Visual mode state (line-level selection)
    isVisualMode bool
    visualAnchor int
    lineCursor   int

    // Selection tracking
    selection    *SelectionState

    // UI components
    fileList     filelist.Model
    diffView     diffview.Model
    statusBar    statusbar.Model
    destPicker   destpicker.Model
    help         help.Model
    searchModal  searchmodal.Model

    // Search state
    searchState  *search.SearchState

    // Diff data
    changes      []diff.FileChange
}
```

**Message Flow:**
```
User Input → KeyMsg → Update() → State Changes → View()
     ↓                                               ↓
Commands (loadDiff, applyChanges)              Render Components
     ↓
Messages (diffLoadedMsg, changesAppliedMsg)
```

### 2. JJ Client (internal/jj/client.go)

**Purpose**: Abstraction layer for all jj command-line interactions.

**Key Operations:**
```go
Diff(revision) → string          // Get unified diff
GetRevisions(limit) → []Entry    // List recent revisions
MoveChanges(patch, src, dest)    // Apply patch to destination
```

**MoveChanges Algorithm:**
1. Save current working copy change ID
2. Create new empty commit on destination
3. Restore working copy to destination state
4. Apply patch using `git apply`
5. Squash changes into destination
6. Restore original working copy
7. On any error: `jj undo` and restore working copy

**Safety Guarantees:**
- Automatic rollback on errors
- Working copy restoration
- No destructive operations without confirmation

### 3. Diff Subsystem (internal/diff/)

#### Parser (parser.go)

**Purpose**: Convert unified diff text to structured data.

**Algorithm:**
1. Split diff into file sections (headers starting with "diff --git")
2. Extract file metadata (change type, paths)
3. Parse hunks (lines starting with "@@")
4. Classify lines by type: Context, Addition, Deletion
5. Track line numbers for old and new versions

**Data Structures:**
```go
type FileChange struct {
    Path       string
    ChangeType ChangeType  // Modified, Added, Deleted
    Hunks      []Hunk
}

type Hunk struct {
    Header     string      // "@@ -1,5 +1,6 @@"
    Lines      []Line
}

type Line struct {
    Type       LineType    // Context, Addition, Deletion
    Content    string
    OldLineNum int
    NewLineNum int
}
```

#### Patch Generator (patch.go)

**Purpose**: Generate unified diff patches from selections.

**Supports:**
- Whole hunk selection
- Partial hunk selection (line-level)
- Multiple files
- New/deleted files

**Interface:**
```go
type SelectionState interface {
    IsHunkSelected(filePath string, hunkIdx int) bool
    HasPartialSelection(filePath string, hunkIdx int) bool
    IsLineSelected(filePath string, hunkIdx, lineIdx int) bool
}
```

**Partial Hunk Algorithm:**
1. Identify selected lines within hunk
2. Expand selection with 3 lines of context before/after
3. Extract selected lines
4. Recalculate hunk header with correct line counts:
   - Context lines: increment both old and new counts
   - Deletions: increment only old count
   - Additions: increment only new count
5. Generate patch with corrected header

**Example:**
```diff
# Original hunk (10 lines)
@@ -1,10 +1,10 @@
 line 1
 line 2
+added line
 line 3

# User selects only "added line"
# After context expansion (3 lines before/after):

@@ -1,4 +1,5 @@
 line 1
 line 2
+added line
 line 3
```

### 4. Search Subsystem (internal/search/)

**Purpose**: Incremental search across file paths and diff content.

**Features:**
- Case-insensitive by default
- Multiple matches per line
- Navigation state preservation
- Match highlighting

**Data Structures:**
```go
type MatchLocation struct {
    FileIdx   int       // -1 for file path matches
    HunkIdx   int
    LineIdx   int
    FilePath  string
    StartCol  int       // Character position in string
    EndCol    int
    MatchText string
}

type SearchState struct {
    Query           string
    Matches         []MatchLocation
    CurrentIdx      int
    OriginalState   NavigationState  // For Esc cancellation
    IsActive        bool
}
```

**Search Algorithm:**
1. Convert query to lowercase (if case-insensitive)
2. Search file paths: single match per file
3. Search line content: multiple matches per line using sliding window
4. Store all matches with precise column positions
5. Initialize CurrentIdx to first match

**Navigation:**
- `n`: NextMatch() → (CurrentIdx + 1) % len(Matches)
- `N`: PrevMatch() → CurrentIdx - 1 (wrap to end)
- Esc: Restore original navigation state
- Enter: Close modal, keep current position

### 5. Visual Mode (Line-Level Selection)

**Purpose**: Vim-style visual selection for choosing individual lines within hunks.

**State Machine:**
```
Normal Mode
    ↓ [v key]
Visual Mode (anchor set)
    ↓ [j/k keys]
Extend/Contract Selection
    ↓ [Space]
Confirm Selection → Normal Mode
    ↓ [Esc]
Cancel → Normal Mode
```

**State:**
```go
isVisualMode bool
visualAnchor int      // Starting line of selection
lineCursor   int      // Current line (end of range)
```

**Selection Storage:**
```go
type HunkSelection struct {
    WholeHunk     bool
    SelectedLines map[int]bool  // lineIdx → selected
}
```

**Visual Indicators:**
- `█` : Line in visual range (between anchor and cursor)
- `•` : Previously selected line
- `>` : Current line cursor

### 6. Components

#### FileList (internal/components/filelist/)

**Purpose**: Display list of changed files with change type indicators.

**Features:**
- File path display with change indicators ([M], [A], [D])
- Selection highlighting (focused vs unfocused)
- Search match highlighting in file paths

**Layout:**
```
┌─────────────────┐
│ Files           │
│ [M] file1.go    │  ← Selected, focused
│ [A] file2.go    │
│ [D] file3.go    │
└─────────────────┘
```

#### DiffView (internal/components/diffview/)

**Purpose**: Render diff content with hunks, line numbers, and indicators.

**Features:**
- Hunk headers with selection indicators
- Line numbers (old/new)
- Color-coded additions/deletions
- Visual mode highlighting
- Line selection indicators
- Search match highlighting

**Layout:**
```
┌─────────────────────────────────┐
│ Modified src/main.go            │
│ > @@ -1,5 +1,6 @@ [X]           │  ← Selected hunk
│      1   line 1                  │
│ █    2 + added line              │  ← Visual range
│ █    3   line 2                  │
│ •    4 - deleted                 │  ← Selected line
└─────────────────────────────────┘
```

**Rendering Pipeline:**
1. Calculate visible window (offset + height)
2. Render file header
3. For each hunk:
   - Render hunk header with indicators
   - For each line:
     - Determine line type (context/addition/deletion)
     - Check selection state (visual, selected, current)
     - Apply match highlighting if searching
     - Apply color styling
     - Apply background highlighting
4. Pad remaining space

#### Modal Components

**Shared Pattern:**
```go
type Model struct {
    visible bool
    // modal-specific state
}

func (m *Model) Show() { m.visible = true }
func (m *Model) Hide() { m.visible = false }
func (m Model) IsVisible() bool { return m.visible }
func (m Model) View(width, height int) string {
    return renderModal(content, width, height)  // Centered overlay
}
```

**Components:**
- **Help Modal**: Keybinding reference and workflow guide
- **Destination Picker**: Scrollable list of revisions
- **Search Modal**: Search input with match counter

**Rendering**: Modals use `lipgloss.Place()` to center content over base view with semi-transparent background effect.

## Data Flow Diagrams

### Application Startup

```
main()
  ↓
Parse CLI flags
  ↓
Initialize jj.Client
  ↓
Check jj installation
  ↓
Initialize Theme
  ↓
Create Model (NewModel)
  ↓
Launch Bubble Tea Program
  ↓
Model.Init() → loadDiff command
  ↓
diffLoadedMsg → populate changes
  ↓
Render initial view
```

### Interactive Mode: Applying Changes

```
User presses 'a' key
  ↓
handleApplyChanges()
  ↓
Generate patch from selections
  ↓
applyChangesCmd → tea.Cmd
  ↓
client.MoveChanges(patch, src, dest)
  ├─ Create new commit on dest
  ├─ Restore to dest state
  ├─ Apply patch (git apply)
  ├─ Squash into dest
  └─ Restore working copy
  ↓ (on error: jj undo + restore)
  ↓
changesAppliedMsg or errMsg
  ↓
Update model state
  ↓
Reload diff
```

### Search Flow

```
User presses '/' key
  ↓
enterSearchMode()
  ├─ Save current navigation state
  └─ Show search modal
  ↓
User types characters
  ↓
handleSearchKeyPress()
  ├─ Update query
  └─ executeSearch()
      ├─ SearchState.ExecuteSearch(files)
      │   ├─ Search file paths
      │   └─ Search line content
      ├─ Jump to first match
      └─ Update modal with match count
  ↓
User presses 'n' or Ctrl-N
  ↓
nextSearchMatch()
  ├─ SearchState.NextMatch()
  ├─ Update navigation (file, hunk)
  └─ Update diffView
  ↓
User presses Enter → close modal, stay at match
User presses Esc → restore original position, close modal
```

### Visual Mode: Line Selection

```
User presses 'v' key
  ↓
Enter visual mode
  ├─ isVisualMode = true
  ├─ visualAnchor = lineCursor
  └─ Re-render with visual indicators
  ↓
User presses 'j' or 'k'
  ↓
handleVisualNavigation(delta)
  ├─ Update lineCursor
  └─ Re-render visual range
  ↓
User presses Space
  ↓
toggleVisualSelection()
  ├─ Calculate range [anchor, cursor]
  ├─ SelectionState.SelectLineRange()
  ├─ Exit visual mode
  └─ Re-render with selection markers (•)
  ↓
User presses Esc → exit visual mode without selecting
```

## Key Design Decisions

### 1. Separation of Concerns

**Model**: Central coordinator but delegates rendering to components
- **Rationale**: Each component knows best how to render itself
- **Benefit**: Components testable in isolation

### 2. Interface-Based Selection

```go
type SelectionState interface {
    IsHunkSelected(filePath string, hunkIdx int) bool
    HasPartialSelection(filePath string, hunkIdx int) bool
    IsLineSelected(filePath string, hunkIdx, lineIdx int) bool
}
```

- **Rationale**: Allows patch generator to work with any selection implementation
- **Benefit**: Testable without full Model, supports future selection strategies

### 3. Message-Driven Architecture

All state changes happen through Update(msg):
- **Rationale**: Single point of control for state transitions
- **Benefit**: Predictable state flow, easier debugging, undo/redo support

### 4. Context Expansion in Partial Hunks

Always include 3 lines before/after selected lines:
- **Rationale**: Git requires context for reliable patch application
- **Benefit**: Patches apply successfully even with surrounding code

### 5. Working Copy Restoration

Always restore working copy after MoveChanges:
- **Rationale**: User expectations - working copy shouldn't change
- **Benefit**: Non-surprising behavior, matches git staging mental model

### 6. Match Highlighting Architecture

Search matches stored as `(StartCol, EndCol)` ranges:
- **Rationale**: Precise highlighting without re-searching during render
- **Benefit**: Fast rendering, supports multiple matches per line

## Testing Strategy

### Test Layers

1. **Unit Tests** (internal/diff/*_test.go)
   - Diff parsing edge cases
   - Patch generation correctness
   - Line number tracking

2. **Model Tests** (internal/model/model_test.go)
   - Selection state behavior
   - Mode transitions
   - Message handling

3. **Integration Tests** (tests/integration/client_test.go)
   - JJ client operations
   - End-to-end workflows
   - Error handling

### Test Coverage

```
internal/diff:        100% (17 tests)
internal/model:       100% (12 tests)
tests/integration:    100% (3 tests)
Total:                32 tests
```

### Mock Strategy

**MockSelectionState** in patch tests:
```go
type mockSelectionState struct {
    selections      map[string]map[int]bool
    lineSelections  map[string]map[int]map[int]bool
    partialHunks    map[string]map[int]bool
}
```

Implements SelectionState interface for testing patch generation without full Model dependency.

## Performance Considerations

### Current Implementation

- **File List**: Renders all files, no virtualization
- **Diff View**: Offset-based scrolling, renders visible window only
- **Search**: Linear scan through all files and lines
- **Match Highlighting**: Pre-computed positions, fast rendering

### Known Limitations

- Large diffs (>1000 lines) may have noticeable lag
- File list not virtualized (memory scales with file count)
- No lazy loading of diff content

### Future Optimizations (Planned)

1. **Virtualization**: Render only visible items in file list
2. **Lazy Loading**: Load diffs on demand, not all at startup
3. **Incremental Rendering**: Update only changed components
4. **Background Loading**: Load revisions asynchronously

## Extension Points

### Adding New Modes

1. Create mode constant in model.go
2. Add mode-specific keybindings in handleKeyPress()
3. Update View() to render mode indicator
4. Implement mode transition logic

### Adding New Components

1. Create package in internal/components/
2. Implement New(), View(), and state methods
3. Add field to Model struct
4. Initialize in NewModel()
5. Call View() in Model.View()

### Adding New Search Features

1. Extend MatchLocation with new fields
2. Update ExecuteSearch() algorithm
3. Add new navigation methods (e.g., JumpToFile)
4. Update searchmodal to display new info

## Dependencies

### Direct Dependencies

- `github.com/charmbracelet/bubbletea`: TUI framework
- `github.com/charmbracelet/lipgloss`: Styling
- Go standard library: `os/exec`, `strings`, `fmt`, etc.

### External Programs

- `jj`: Jujutsu version control (required)
- `git`: Used for `git apply` in patch application

### Version Requirements

- Go 1.21+ (for modern standard library features)
- jj 0.9.0+ (tested version range)

## Color Theme System

### Theme Architecture

Global theme initialized at startup:
```go
theme.Init()  // Detects terminal background, sets global theme
```

Components reference global colors:
```go
style.Foreground(theme.AddedLine)
style.Background(theme.SelectedBg)
```

### Theme Structure

```go
type Theme struct {
    Primary     lipgloss.Color  // Accents (mauve)
    Accent      lipgloss.Color  // Highlights (teal)
    Secondary   lipgloss.Color  // Alt highlights (yellow/peach)
    Text        lipgloss.Color  // Main text
    SelectedBg  lipgloss.Color  // Selection background
    MutedBg     lipgloss.Color  // Subtle background
    SoftMutedBg lipgloss.Color  // Even more subtle
    ModalBg     lipgloss.Color  // Modal backdrop
    AddedLine   lipgloss.Color  // Green
    DeletedLine lipgloss.Color  // Red
}
```

### Theme Detection

1. Check `CATPPUCCIN_THEME` env var
2. If not set, use `lipgloss.HasDarkBackground()`
3. Load Latte (light) or Macchiato (dark)

### Design Philosophy

Follows charm/bubbletea aesthetic:
- Minimal color usage
- Single unified background
- Borders provide visual hierarchy
- Color reserved for actionable elements (badges, accents)
- No rainbow effects or excessive styling

## Future Architecture Considerations

### Potential Refactorings

1. **Component State Separation**: Move more state into components (e.g., scroll position in diffview)
2. **Message Types**: Separate internal messages from external events
3. **Command Factory**: Centralize command creation logic
4. **Navigation Stack**: Support undo/redo for navigation

### Scalability Concerns

1. **Memory**: All diffs loaded in memory (issue for large repos)
2. **CPU**: Search rescans entire diff on every keystroke
3. **Rendering**: Full re-render on every update

### Compatibility

- Designed for jj CLI, but parser could work with any unified diff
- MoveChanges specific to jj workflow (new → restore → apply → squash)
- Could support git with different backend implementation

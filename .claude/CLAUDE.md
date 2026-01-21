# Project: jj-diff

A TUI for viewing and manipulating diffs in Jujutsu (jj). Built with Bubble Tea following The Elm Architecture.

## Version Control

This is a **non-colocated jj repository**. Critical constraints:

1. **READ-ONLY jj operations only** - Do not run any jj commands that modify repository state
2. **No git commands** - There is no `.git` directory; git commands will fail

### Allowed jj Commands

- `jj status`, `jj log`, `jj show`, `jj diff`
- `jj branch list`, `jj file show`, `jj file list`
- `jj op log`, `jj help`

### Forbidden Operations

- Any jj write operation: `new`, `commit`, `describe`, `edit`, `abandon`, `squash`, `split`, `rebase`, `restore`, `resolve`
- Any branch modification: `branch set/create/delete/move`
- Any git operation: `jj git *`, `git *`
- Operation history changes: `op restore`, `op undo`

## Project Structure

```
jj-diff/
├── cmd/jj-diff/          # CLI entry point
├── internal/
│   ├── model/            # Core TEA model (orchestrator)
│   ├── jj/               # jj CLI integration
│   ├── diff/             # Parser and patch generator
│   ├── search/           # Search functionality
│   ├── fuzzy/            # Fuzzy matching
│   ├── components/       # UI components
│   └── theme/            # Catppuccin themes
└── tests/integration/    # End-to-end tests
```

## Key Design Patterns

### The Elm Architecture

All state updates flow through `Update(msg) → (model, cmd)`:

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return m.handleKeyPress(msg)
    case diffLoadedMsg:
        m.changes = msg.changes
        return m, nil
    }
    return m, nil
}
```

### Component Pattern

Components manage their own rendering:

```go
type Component struct {
    // State
}

func New() Component { /* initialize */ }
func (c *Component) SetData(data Type) { /* update */ }
func (c Component) View(width, height int) string { /* render */ }
```

### Selection State

Interface-based design allows testing without full Model:

```go
type SelectionState interface {
    IsHunkSelected(filePath string, hunkIdx int) bool
    HasPartialSelection(filePath string, hunkIdx int) bool
    IsLineSelected(filePath string, hunkIdx, lineIdx int) bool
}
```

## Common Development Tasks

### Adding a New Component

1. Create package in `internal/components/yourcomponent/`
2. Implement `New()`, `View(width, height int) string`, and state methods
3. Add to Model in `internal/model/model.go`
4. Call `View()` in `Model.View()`
5. Handle input in `handleKeyPress()`

### Adding a Keybinding

1. Add case in `internal/model/model.go:handleKeyPress()`
2. Implement handler function
3. Update help overlay in `internal/components/help/help.go`
4. Update README.md keybindings section

### Modifying Diff Parser

Parser in `internal/diff/parser.go` converts unified diff to structured data:

```go
type FileChange struct {
    Path       string
    ChangeType ChangeType  // Modified, Added, Deleted
    Hunks      []Hunk
}
```

### Patch Generation

Generator in `internal/diff/patch.go` creates patches from selections:

- Whole hunks: Copy hunk as-is
- Partial hunks: Extract selected lines + 3 lines context before/after
- Recalculate hunk header with correct line counts

## Testing Approach

60 tests across 3 layers:

**Unit Tests** (45 tests)
- Diff parsing, patch generation, search, fuzzy matching
- `go test ./internal/diff/` etc.

**Model Tests** (12 tests)
- State management, selection, navigation
- `go test ./internal/model/`

**Integration Tests** (3 tests)
- Real jj workflows with temporary repositories
- `go test ./tests/integration/`

Manual testing: `./scripts/interactive-test.sh`

## Performance Characteristics

Based on benchmarks (Apple M2 Pro):

- File list: 7μs (constant regardless of count)
- Diff view: 33-52μs (scales to 20K lines)
- Search: 1.9ms (10K lines), 17ms (100K lines)
- Patch generation: 352μs (200 hunks), 5.5ms (2000 hunks)

**Key optimizations:**
- File list uses height-based cutoff (only renders visible)
- Diff view uses offset-based windowing
- No virtualization needed - current implementation is fast enough

## JJ Integration

MoveChanges workflow in `internal/jj/client.go`:

1. Save current working copy ID
2. Create new empty commit on destination: `jj new destination`
3. Restore to destination state: `jj restore --from destination`
4. Apply patch: `git apply patch.diff`
5. Squash into destination: `jj squash`
6. Restore original working copy: `jj edit original`
7. On error: `jj undo` + restore working copy

## Theme System

Global theme initialized at startup in `internal/theme/`:

- Auto-detection via `lipgloss.HasDarkBackground()`
- Override with `CATPPUCCIN_THEME=latte` or `macchiato`
- Components reference global colors: `theme.Primary`, `theme.AddedLine`, etc.

## Tips for AI Development

1. **Read existing code first**: Components follow consistent patterns
2. **Preserve functional style**: Small functions, composition over inheritance
3. **Test changes**: Run `go test ./...` before committing
4. **Follow TEA**: State updates through messages, pure rendering
5. **Use CONTRIBUTING.md**: Architecture details and development setup

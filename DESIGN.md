# Design

<!-- Project-specific architecture, design decisions, and domain context.
     This file is preserved across template updates (_skip_if_exists). -->

## Architecture

jj-diff is a TUI application built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) following The Elm Architecture pattern.

### Project Structure

```
jj-diff/
├── cmd/jj-diff/          # CLI entry point
├── internal/
│   ├── model/            # Core application state (TEA)
│   ├── jj/               # jj CLI integration
│   ├── diff/             # Diff parsing and patch generation
│   ├── search/           # Search functionality
│   ├── fuzzy/            # Fuzzy matching
│   ├── components/       # UI components (filelist, diffview, modals)
│   └── theme/            # Catppuccin themes
└── tests/integration/    # End-to-end tests
```

### Key Components

**Model** (`internal/model/model.go`)

- Central orchestrator following The Elm Architecture
- Message-driven state updates
- Coordinates UI components and jj client

**JJ Client** (`internal/jj/client.go`)

- Abstraction for jj command execution
- MoveChanges: applies patches using `jj new` + `git apply` + `jj squash`
- Automatic rollback on errors

**Diff Subsystem** (`internal/diff/`)

- Parser: converts unified diff to structured data
- Patch Generator: creates patches from hunk and line selections
- Supports whole hunks and partial hunks with context expansion

**Components** (`internal/components/`)

- FileList: vertical table view with stats
- DiffView: unified or side-by-side rendering with syntax highlighting
- Modals: help, search, destination picker, fuzzy finder

### Design Principles

1. **Functional style**: small, composable functions with single responsibility
2. **The Elm Architecture**: immutable updates, message-driven, pure rendering
3. **Component composition**: each component manages its own rendering
4. **Interface-based selection**: allows patch generation without a full Model dependency
5. **Catppuccin theme**: minimal color usage, borders for hierarchy, color for accents

## Patterns

### Component pattern

```go
type Model struct {
    // Component state
}

func New() Model { /* initialize */ }
func (m *Model) SetX(x Type) { /* update state */ }
func (m Model) View(width, height int) string { /* render */ }
```

### Error handling

- Propagate errors with context: `fmt.Errorf("operation failed: %w", err)`
- Use specific error types where appropriate
- Automatic rollback in jj operations

### Adding a component

1. Create a package in `internal/components/`
2. Implement `New()`, `View()`, and state methods
3. Add a field to the Model struct in `internal/model/model.go`
4. Initialize it in `NewModel()`
5. Call `View()` from `Model.View()`
6. Add keybindings in `handleKeyPress()`

### Adding a mode

1. Add the mode constant in `internal/model/model.go`
2. Add mode-specific keybindings in `handleKeyPress()`
3. Update `View()` to render the mode indicator
4. Implement the mode transition logic

### Extending search

1. Extend `MatchLocation` in `internal/search/search.go`
2. Update the `ExecuteSearch()` algorithm
3. Add navigation methods if needed
4. Update the searchmodal component

## Testing

### Test layers

Unit tests cover diff parsing (`internal/diff/parser_test.go`), patch generation (`internal/diff/patch_test.go`), search (`internal/search/search_test.go`), and fuzzy matching (`internal/fuzzy/fuzzy_test.go`).

Model tests cover UI state management, selection state, navigation, and mode transitions in `internal/model/model_test.go`.

Integration tests in `tests/integration/client_test.go` drive real jj workflows and cover MoveChanges, rollback, and working copy preservation.

### Coverage goals

| Component | Target | Last measured |
|-----------|--------|---------------|
| Diff parser | >90% | 92.5% |
| Search | >90% | 96.6% |
| Model | >40% | 41.2% (acceptable for a TUI) |
| Integration | Critical paths | 100% |

### Manual testing

Two interactive scripts drive a real repository:

```bash
./scripts/test-in-tmpdir.sh      # Create a test repo and launch jj-diff
./scripts/interactive-test.sh    # Menu of scenarios
```

To build a scratch repo by hand:

```bash
cd /tmp
jj init --git test-repo
cd test-repo
echo "line 1" > file.txt
jj commit -m "initial"
echo "line 2" >> file.txt
jj commit -m "changes"
/path/to/jj-diff --interactive
```

Force a theme in tests with `lipgloss.SetHasDarkBackground(true)`.

## Running locally

```bash
./jj-diff                    # Browse mode (read-only)
./jj-diff --interactive      # Move changes between revisions
./jj-diff $left $right       # Diff-editor mode (jj split, diffedit)
```

Prerequisites beyond the template's: jj 0.9.0+ on PATH, and git, which `git apply` needs during patch application.

## Performance

Benchmarks on an Apple M2 Pro:

| Component | Typical | Large | Scaling |
|-----------|---------|-------|---------|
| File list | 7μs | 7μs (5000 files) | Constant, from the height cutoff |
| Diff view | 33μs | 52μs (20K lines) | Offset-based windowing |
| Search | 1.9ms | 17ms (100K lines) | Linear |
| Patch generation | 352μs | 5.5ms (2000 hunks) | Linear |

Search is fast enough without debouncing. Performance stays acceptable up to roughly 5,000 files, 20,000 diff lines, 100,000 lines searched, and 2,000 hunks per patch.

To profile:

```bash
go test -cpuprofile=cpu.prof -bench=. ./internal/components/diffview/
go tool pprof cpu.prof

go test -memprofile=mem.prof -bench=. ./internal/components/diffview/
go tool pprof mem.prof
```

## Resources

- [Bubble Tea documentation](https://github.com/charmbracelet/bubbletea)
- [Lip Gloss styling](https://github.com/charmbracelet/lipgloss)
- [jj documentation](https://martinvonz.github.io/jj/)
- [Catppuccin theme](https://github.com/catppuccin/catppuccin)

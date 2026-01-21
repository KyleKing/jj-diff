# Contributing to jj-diff

## Architecture Overview

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
- Parser: Converts unified diff to structured data
- Patch Generator: Creates patches from hunk/line selections
- Supports whole hunks and partial hunks with context expansion

**Components** (`internal/components/`)
- FileList: Vertical table view with stats
- DiffView: Unified or side-by-side rendering with syntax highlighting
- Modals: Help, search, destination picker, fuzzy finder

### Design Principles

1. **Functional Style**: Small, composable functions with single responsibility
2. **The Elm Architecture**: Immutable updates, message-driven, pure rendering
3. **Component Composition**: Each component manages its own rendering
4. **Interface-Based Selection**: Allows patch generation without full Model dependency
5. **Catppuccin Theme**: Minimal color usage, borders for hierarchy, color for accents

## Development Setup

### Prerequisites

- Go 1.21+
- jj 0.9.0+ installed and in PATH
- git (for `git apply` during patch application)

### Building

```bash
# Install dependencies
make deps

# Build binary
make build

# Run tests
make test

# Clean build artifacts
make clean
```

### Running Locally

```bash
# Browse mode (read-only)
./jj-diff

# Interactive mode (move changes between revisions)
./jj-diff --interactive

# Diff-editor mode (use with jj split, diffedit)
./jj-diff $left $right
```

## Testing

### Test Layers

The project has 60 tests across three layers:

**Unit Tests** (45 tests)
- Diff parsing: `internal/diff/parser_test.go`
- Patch generation: `internal/diff/patch_test.go`
- Search: `internal/search/search_test.go`
- Fuzzy matching: `internal/fuzzy/fuzzy_test.go`

**Model Tests** (12 tests)
- UI state management: `internal/model/model_test.go`
- Selection state, navigation, mode transitions

**Integration Tests** (3 tests)
- jj workflows: `tests/integration/client_test.go`
- Tests MoveChanges, rollback, working copy preservation

### Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/diff/
go test ./internal/model/
go test ./tests/integration/

# With coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Verbose
go test -v ./...
```

### Coverage Goals

| Component | Target | Actual |
|-----------|--------|--------|
| Diff parser | >90% | 92.5% |
| Search | >90% | 96.6% |
| Model | >40% | 41.2% (acceptable for TUI) |
| Integration | Critical paths | 100% |

### Manual Testing

Two interactive test scripts for real repository testing:

```bash
# Quick test - creates test repo and launches jj-diff
./scripts/test-in-tmpdir.sh

# Interactive test suite - multiple scenarios with menu
./scripts/interactive-test.sh
```

## Performance Characteristics

### Rendering Performance

Based on benchmarks on Apple M2 Pro:

| Component | Typical | Large | Performance |
|-----------|---------|-------|-------------|
| File List | 7μs | 7μs (5000 files) | Constant (height cutoff) |
| Diff View | 33μs | 52μs (20K lines) | Excellent scaling |
| Search | 1.9ms | 17ms (100K lines) | Linear, acceptable |
| Patch Gen | 352μs | 5.5ms (2000 hunks) | Fast |

**Key Findings:**
- File list already optimized with height-based cutoff
- Diff view uses offset-based windowing
- Search is fast enough without debouncing
- No performance optimizations needed for v0.1.0

### Performance Limits

Acceptable performance up to:
- 5,000 files
- 20,000 diff lines
- 100,000 lines searched
- 2,000 hunks in patch

## Code Style

### Go Conventions

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable names
- Keep functions small and focused
- Prefer composition over inheritance
- No emojis in code or output

### Component Pattern

```go
type Model struct {
    // Component state
}

func New() Model { /* initialize */ }
func (m *Model) SetX(x Type) { /* update state */ }
func (m Model) View(width, height int) string { /* render */ }
```

### Error Handling

- Propagate errors with context: `fmt.Errorf("operation failed: %w", err)`
- Use specific error types where appropriate
- Automatic rollback in jj operations

## Adding Features

### Adding a New Component

1. Create package in `internal/components/`
2. Implement `New()`, `View()`, and state methods
3. Add field to Model struct in `internal/model/model.go`
4. Initialize in `NewModel()`
5. Call `View()` in `Model.View()`
6. Add keybindings in `handleKeyPress()`

### Adding a New Mode

1. Add mode constant in `internal/model/model.go`
2. Add mode-specific keybindings in `handleKeyPress()`
3. Update `View()` to render mode indicator
4. Implement mode transition logic

### Extending Search

1. Extend `MatchLocation` in `internal/search/search.go`
2. Update `ExecuteSearch()` algorithm
3. Add new navigation methods if needed
4. Update searchmodal component

## Release Process

1. Update version in `cmd/jj-diff/main.go`
2. Run full test suite: `go test ./...`
3. Run manual tests: `./scripts/interactive-test.sh`
4. Build binary: `make build`
5. Tag release: `git tag v0.x.0`
6. Create GitHub release with notes

## Common Tasks

### Debugging TUI Rendering

Use `lipgloss.SetHasDarkBackground(true)` in tests to force theme.

### Testing with Real Repositories

```bash
# Create test repo
cd /tmp
jj init --git test-repo
cd test-repo

# Make changes
echo "line 1" > file.txt
jj commit -m "initial"
echo "line 2" >> file.txt
jj commit -m "changes"

# Run jj-diff
/path/to/jj-diff --interactive
```

### Profiling Performance

```bash
# CPU profile
go test -cpuprofile=cpu.prof -bench=. ./internal/components/diffview/
go tool pprof cpu.prof

# Memory profile
go test -memprofile=mem.prof -bench=. ./internal/components/diffview/
go tool pprof mem.prof
```

## Resources

- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea)
- [Lip Gloss Styling](https://github.com/charmbracelet/lipgloss)
- [jj Documentation](https://martinvonz.github.io/jj/)
- [Catppuccin Theme](https://github.com/catppuccin/catppuccin)

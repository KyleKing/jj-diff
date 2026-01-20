# jj-diff

A TUI for interactive diff viewing and manipulation in Jujutsu (jj).

## Features

### Phase 1 (Current - v0.1.0)

**Browse Mode**
- View diffs for any revision with syntax highlighting
- Navigate through changed files with keyboard shortcuts
- Incremental search through files and diff content (/ key)
- Visual match highlighting in file paths and line content
- Color-coded additions (green) and deletions (red)
- Responsive two-panel layout (file list + diff view)

**Interactive Mode**
- Select destination revision with visual picker
- Navigate and select hunks with keyboard shortcuts
- Line-level selection with visual mode (v key)
- Visual indicators for current and selected hunks/lines
- Patch generation for whole hunks or selected lines
- Real-time change application using jj commands
- Automatic rollback on errors
- Comprehensive help overlay with keybindings
- Catppuccin themes with auto-detection

**Search and Navigation (Phase 2 - v0.2.0)**
- Incremental search through files and diff content (/ key)
- Visual match highlighting in file paths and line content
- Fuzzy file finder with intelligent scoring (f key)
- Real-time filtering as you type
- Match highlighting for easy navigation

**Syntax Highlighting (Phase 2 - v0.2.0)**
- Code syntax highlighting powered by chroma
- Automatic language detection from file extensions
- Supports 100+ languages (Go, Python, JavaScript, Rust, etc.)
- Subtle highlighting that preserves diff colors
- Context lines highlighted for better readability

### Planned Features (Phase 3)

- **Multi-split**: Split commits into multiple focused commits
- **scm-diff-editor Protocol**: Optional drop-in replacement for jj's builtin diff editor (if requested)
- **Performance**: Virtualization for large diffs (>1000 lines)

## Installation

```bash
make deps
make build
```

## Usage

```bash
# Browse working copy changes
./jj-diff

# Browse specific revision
./jj-diff -r @-

# Browse mode (explicit)
./jj-diff --browse

# Interactive mode - select and move changes
./jj-diff --interactive

# Interactive mode with initial destination
./jj-diff -i --destination @-
```

## Keybindings

### Navigation (All Modes)
- `j/k` or `↓/↑` - Move down/up (file list or scroll diff)
- `g` - Jump to first file
- `G` - Jump to last file
- `n` - Next hunk (when in diff view)
- `p` - Previous hunk (when in diff view)
- `Tab` - Switch focus between file list and diff view

### Actions
- `r` - Refresh diff from jj
- `/` - Open search (type to search files and diff content)
- `f` - Open fuzzy file finder (filter-as-you-type)
- `n` - Next search match (or next hunk when not searching)
- `N` - Previous search match
- `?` - Show/hide help overlay
- `q` or `Ctrl-C` - Quit

### Search Mode
- Type characters to filter matches
- `Enter` - Close search and stay at current match
- `Esc` - Cancel search and return to original position
- `Ctrl-N/P` or `↓`/`↑` - Navigate matches while search is open

### File Finder Mode (Fuzzy)
- Type to filter files with fuzzy matching
- `↑`/`↓` - Navigate filtered results
- `Enter` - Select file and jump to it
- `Esc` - Cancel and return to current file

### Interactive Mode Only
- `d` - Select destination revision (opens picker modal)
- `Space` - Toggle current hunk selection
- `v` - Enter visual mode for line-level selection
- `j/k` in visual mode - Extend/contract line selection range
- `Space` in visual mode - Confirm line selection and exit visual mode
- `Esc` - Exit visual mode without applying
- `a` - Apply selected changes to destination
- In destination picker:
  - `j/k` - Navigate revisions
  - `Enter` - Select destination
  - `Esc` - Cancel

### Visual Indicators
- `>` - Current hunk or line (cursor position)
- `[X]` - Selected hunk (will be moved to destination)
- `█` - Visual mode selection range
- `•` - Selected individual lines

## Interactive Mode Workflow

1. **Start Interactive Mode**
   ```bash
   ./jj-diff --interactive
   ```

2. **Select Destination**
   - Press `d` to open the destination picker
   - Navigate with `j/k` to select a revision
   - Press `Enter` to confirm

3. **Select Changes**
   - Navigate to files with `j/k` in the file list
   - Press `Tab` to focus the diff view
   - Navigate hunks with `n/p`
   - Press `Space` to toggle whole hunk selection
   - For line-level selection:
     - Press `v` to enter visual mode at current line
     - Use `j/k` to extend/contract selection range
     - Press `Space` to confirm selection
   - Selected hunks show `[X]` indicator
   - Selected lines show `•` indicator
   - Visual range shows `█` indicator

4. **Apply Changes**
   - Press `a` to apply selected hunks to destination
   - Changes are moved in real-time
   - Press `r` to refresh and see updated diff

5. **Get Help**
   - Press `?` at any time to see all keybindings

## Design Philosophy

- Native jj terminology (no "staging" concept)
- Destination-first workflow (select where, then what)
- Real-time application of changes
- Minimal, focused interface following bubbletea design patterns
- Keyboard-driven interaction inspired by vim/lazygit

## Testing

The project has comprehensive test coverage with 40 tests across three layers:

```bash
# Run all tests
go test ./...

# Run specific test layers
go test ./internal/diff/...         # Unit tests (17 tests)
go test ./internal/model/...        # Model tests (20 tests including line-level selection)
go test ./tests/integration/...     # Integration tests (3 tests)

# Run with coverage
go test -cover ./...

# Verbose output
go test -v ./...
```

### Manual Testing with Real Repositories

Two interactive test scripts are provided for manual testing:

```bash
# Quick test - Creates test repo, shows diff, runs jj-diff
./scripts/test-in-tmpdir.sh

# Interactive test suite - Multiple scenarios with menu
./scripts/interactive-test.sh
```

The interactive test script provides scenarios for:
- Simple changes (single file, few hunks)
- Multiple files (nested directories, new files)
- Large diffs (100+ lines, many hunks)
- Move changes workflow (testing interactive mode)
- Custom scenarios (open shell in test repo)

## Development

```bash
# Run tests
make test

# Build
make build

# Clean
make clean
```

## Known Limitations

- Large diffs (>1000 lines) may have performance impact
- Diff view scrolling is line-based (no page up/down yet)
- Syntax highlighting only on context lines (not additions/deletions)

## License

MIT

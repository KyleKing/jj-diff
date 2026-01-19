# jj-diff

A TUI for interactive diff viewing and manipulation in Jujutsu (jj).

## Features

### Phase 1 (Current - v0.1.0)

**Browse Mode**
- View diffs for any revision with syntax highlighting
- Navigate through changed files with keyboard shortcuts
- Color-coded additions (green) and deletions (red)
- Responsive two-panel layout (file list + diff view)

**Interactive Mode**
- Select destination revision with visual picker
- Navigate and select hunks with keyboard shortcuts
- Visual indicators for current and selected hunks
- Patch generation for selected changes
- Real-time change application (framework in place)
- Comprehensive help overlay with keybindings

### Planned Features (Phase 2)

- **Real jj Integration**: Complete implementation of change application
- **scm-diff-editor Protocol**: Drop-in replacement for jj's builtin diff editor
- **Line-level Selection**: Visual mode for selecting individual lines
- **Multi-split**: Split commits into multiple focused commits
- **Syntax Highlighting**: Code syntax highlighting with chroma
- **Search & Filter**: Search in diffs, fuzzy file finder

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
- `?` - Show/hide help overlay
- `q` or `Ctrl-C` - Quit

### Interactive Mode Only
- `d` - Select destination revision (opens picker modal)
- `Space` - Toggle current hunk selection
- `a` - Apply selected changes to destination
- In destination picker:
  - `j/k` - Navigate revisions
  - `Enter` - Select destination
  - `Esc` - Cancel

### Visual Indicators
- `>` - Current hunk (cursor position)
- `[X]` - Selected hunk (will be moved to destination)

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
   - Press `Space` to toggle hunk selection
   - Selected hunks show `[X]` indicator
   - Current hunk shows `>` indicator

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

## Development

```bash
# Run tests
make test

# Build
make build

# Clean
make clean
```

## License

MIT

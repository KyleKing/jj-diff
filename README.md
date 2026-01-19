# jj-diff

A TUI for interactive diff viewing and manipulation in Jujutsu (jj).

## Features

### Phase 1 (Current)

- **Browse Mode**: View diffs for any revision with syntax highlighting
- **File Navigation**: Navigate through changed files with keyboard shortcuts
- **Diff View**: View unified diffs with color-coded additions and deletions

### Planned Features

- **Interactive Mode**: Move changes between revisions in real-time
- **scm-diff-editor Replacement**: Drop-in replacement for jj's builtin diff editor
- **Multi-split**: Split commits into multiple focused commits
- **Operation History**: Track and undo jj operations

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

# Interactive mode (planned)
./jj-diff -i
```

## Keybindings

- `j/k` or `↓/↑` - Navigate files
- `g/G` - Jump to first/last file
- `Tab` - Switch focus between file list and diff view
- `r` - Refresh from jj
- `?` - Show help (planned)
- `q` or `Ctrl-C` - Quit

## Design Philosophy

- Native jj terminology (no "staging" concept)
- Destination-first workflow
- Real-time application of changes
- Minimal, focused interface following bubbletea design patterns

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

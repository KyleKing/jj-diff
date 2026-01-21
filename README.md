# jj-diff

![.github/assets/demo.gif](.github/assets/demo.gif)

A fast, keyboard-driven TUI for viewing and manipulating diffs in Jujutsu (jj). Move changes between commits, split commits interactively, or use as a drop-in replacement for jj's builtin diff editor.

## Features

- **Browse Mode**: View diffs with syntax highlighting, search, and fuzzy file finding
- **Interactive Mode**: Select and move hunks or individual lines between revisions
- **Diff-Editor Mode**: Use with `jj split`, `jj diffedit`, `jj amend -i`, `jj squash -i`
- **Multi-Split Mode**: Tag changes and split commits into multiple focused commits
- **Fast**: Handles 5000+ files and 20K+ line diffs instantly
- **Beautiful**: Catppuccin themes with syntax highlighting for 100+ languages

## Installation

```bash
# Build from source
make deps
make build

# Or install with go
go install github.com/kyleking/jj-diff/cmd/jj-diff@latest
```

## Quick Start

### Browse Diffs

View and search through changes:

```bash
# View working copy changes
jj-diff

# View specific revision
jj-diff -r @-

# Search with '/', fuzzy find with 'f'
```

### Move Changes Between Commits

Interactively select hunks or lines to move:

```bash
jj-diff --interactive

# Press 'd' to select destination
# Press Space to select hunks
# Press 'v' for line-level selection
# Press 'a' to apply changes
```

### Use as jj's Diff-Editor

Replace jj's builtin editor:

```toml
# ~/.config/jj/config.toml
[ui]
diff-editor = "jj-diff"
diff-instructions = false
```

Then use with jj commands:

```bash
jj split          # Split current commit
jj diffedit       # Edit changes in commit
jj amend -i       # Amend interactively
jj squash -i      # Squash interactively
```

### Split Commits into Multiple Parts

Tag changes and create focused commits:

```bash
jj-diff --interactive

# Press 'S' to enter multi-split mode
# Tag hunks with 'a', 'b', 'c', etc.
# Press 'D' to assign tags to commits
# Press 'P' to preview and apply
```

## Keybindings

Press `?` for help overlay with all keybindings.

### Essential Keys

| Key | Action |
|-----|--------|
| `j/k` or `↓/↑` | Navigate files or scroll diff |
| `Tab` | Switch focus between file list and diff |
| `n/p` | Next/previous hunk |
| `/` | Search files and content |
| `f` | Fuzzy file finder |
| `?` | Show help |
| `q` | Quit |

### Interactive Mode

| Key | Action |
|-----|--------|
| `d` | Select destination revision |
| `Space` | Toggle hunk selection |
| `v` | Enter visual mode (line selection) |
| `a` | Apply selected changes |
| `S` | Toggle multi-split mode |

### Multi-Split Mode

| Key | Action |
|-----|--------|
| `a-z` | Tag hunk with letter |
| `D` | Assign tags to commits |
| `P` | Preview and apply split |

### Visual Indicators

- `>` Current hunk/line
- `[X]` Selected hunk
- `[A]` Tagged hunk (multi-split)
- `█` Visual selection range
- `•` Selected line

## Common Workflows

### Move a Few Lines to Previous Commit

```bash
jj-diff --interactive
# Press 'd', select @-, press Enter
# Navigate to desired hunk with 'n'
# Press 'v' to enter visual mode
# Select lines with 'j/k'
# Press Space to confirm selection
# Press 'a' to apply
```

### Split a Large Commit into Focused Changes

```bash
jj-diff --interactive
# Press 'S' to enter multi-split mode
# Tag related changes: press 'a' on UI changes, 'b' on tests, etc.
# Press 'D' to open assignment modal
# Assign tags to existing commits or create new ones
# Press 'P' to preview and apply split
```

### Review Changes Before Committing

```bash
# Browse mode (read-only)
jj-diff

# Use '/' to search for TODO comments
# Use 'f' to quickly jump between files
# Press Tab to focus diff and scroll with j/k
```

### Edit a Commit Interactively

```bash
# Configure jj-diff as diff-editor first
jj diffedit -r @-

# Select changes to keep with Space
# Line-level editing with 'v'
# Press 'a' to save and exit
```

## Performance

jj-diff is optimized for real-world use:

- Handles 5,000+ files instantly (7μs render time)
- 20,000+ line diffs with no lag (52μs render time)
- Search across 100K lines in 17ms
- Syntax highlighting for 100+ languages

Tested on large repositories with excellent responsiveness.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for:
- Architecture overview
- Development setup
- Testing guide
- Code style guidelines

## Troubleshooting

**TUI rendering issues**: Try setting `TERM=xterm-256color`

**Performance issues**: Disable syntax highlighting or use browse mode for very large diffs

**jj integration fails**: Ensure jj 0.9.0+ is installed and in PATH

## License

MIT

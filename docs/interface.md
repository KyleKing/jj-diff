# Interface

## Keys

Press `?` for the help overlay. It reads the live code and changes with the mode
you are in, so trust it over this page, which lists only enough to start.

| Key | Action |
|-----|--------|
| `j` / `k` | Move through files, or scroll the diff |
| `tab` | Switch focus between the file list and the diff |
| `n` / `p` | Next and previous hunk |
| `/` | Search files and diff content |
| `f` | Filter files by typing |
| `?` | Help overlay |
| `q` | Quit |

The overlay also covers the display toggles (side-by-side, line numbers,
whitespace, word-level diff), page and half-page scrolling, first and last
file, and per-file jumps. Those change often enough that listing them here would
go stale.

Adding a keybinding touches the `handleKeyPress()` case in
`internal/model/model.go`, the handler itself, and the help overlay in
`internal/components/help/help.go`.

## Modes

Browse mode is read-only. It is the default, and `-browse` forces it.

Interactive mode selects and moves changes. `--interactive` starts it.

| Key | Action |
|-----|--------|
| `d` | Choose the destination revision |
| `space` | Toggle hunk selection |
| `v` | Visual mode, for line-level selection |
| `a` | Apply the selected changes |
| `S` | Toggle multi-split mode |

Multi-split mode tags changes and splits them across commits.

| Key | Action |
|-----|--------|
| `a` to `z` | Tag the current hunk with that letter |
| `D` | Assign tags to commits |
| `P` | Preview and apply the split |

Diff-editor mode runs when jj invokes jj-diff for `jj split`, `jj diffedit`,
`jj amend -i`, or `jj squash -i`. See [configuration](./configuration.md).

## Visual indicators

- `>` marks the current hunk or line
- `[X]` marks a selected hunk
- `[A]` marks a hunk tagged in multi-split mode
- `█` marks the visual selection range
- `•` marks a selected line

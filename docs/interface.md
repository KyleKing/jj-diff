# Interface

## Keys

The full table is in the [README](../README.md#keys). Press `?` for the help
overlay, which is drawn from the same keymap as the footer and the dispatch, so it
lists exactly what is bound on the screen you are on.

The keymap is aragonite's shared `tui/keys` vocabulary plus jj-diff's own verbs,
merged in `internal/model/keymap.go`. Adding a key means adding a binding there
and a case in `internal/model/dispatch.go`; the help, the footer, and the
which-key panel pick it up. `TestKeymap_AgreesWithShared` fails if a jj-diff key
takes a key the shared table gives another action.

## Modes

Browse mode is read-only. It is the default, and `-browse` forces it.

Interactive mode (`--interactive`) squashes, splits out, or discards a selection:
`s` opens a destination picker, `S` splits the selection out to a new commit
beneath the revision, `R` discards it, and `c` squashes into the `-d` destination.
Each is one jj operation that `jj undo` reverses.

Diff-editor mode runs when jj invokes jj-diff for `jj split`, `jj diffedit`,
`jj amend -i`, or `jj squash -i`. `c` writes the selection and returns to jj; `q`
without writing exits non-zero so jj leaves the revision alone. See
[configuration](./configuration.md).

## Visual indicators

- `▸` in the left margin marks the current hunk or line
- A hunk is headed by the function or type it sits in and the lines it spans,
  `func greet(name string) string {  L5–15`, rather than its raw `@@` header
- `✓` after that range marks a selected hunk
- `┃` in the margin marks the visual selection range
- `•` in the margin marks a selected line
- `▎` beside the line number marks a changed line, in green or red by its side,
  and turns into `+` or `−` under `NO_COLOR`
- The column on the right edge, drawn when the file is taller than the pane, is
  the overview track: the thumb is what is on screen, `▍` marks a changed row
  above or below it, and `•` a search hit

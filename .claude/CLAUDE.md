@../AGENTS.md

# Project: jj-diff

A TUI for viewing and manipulating diffs in Jujutsu (jj). Built with Bubble Tea following The Elm Architecture.

## Version Control

This is a **colocated jj repository**. Both `.jj` and `.git` exist and both work. jj owns the working copy and snapshots it automatically, and git sees the same commits.

Read freely with either tool. Do not stage, commit, push, or rewrite history without explicit instruction, which is the same rule as every other repo here rather than anything specific to jj.

Two things to know when you do write:

- A `git commit` moves git HEAD, and jj imports it on the next jj command. That is expected and safe. Run `jj status` afterward to confirm the import landed
- The git index normally holds intent-to-add entries pointing at the empty blob. That is jj's snapshot of files git does not track yet, not something a person staged. `git diff --cached` is empty for them

`jj op log` and `jj op restore` recover from a bad write.

## Where things are documented

[DESIGN.md](../DESIGN.md) carries the architecture, package layout, patterns, testing strategy, and performance numbers. [AGENTS.md](../AGENTS.md) carries code organization and error handling conventions. [CONTRIBUTING.md](../CONTRIBUTING.md) carries setup and the task list. This file only covers what those three do not.

## Code shapes worth knowing before editing

State updates flow through `Update(msg) → (model, cmd)`:

```go
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        cmd := m.handleKeyPress(msg)
        return m, cmd
    case diffLoadedMsg:
        m.changes = msg.changes
        return m, nil
    }
    return m, nil
}
```

`Model` is held by pointer, so a handler's mutations are permanent the moment it makes them. A `tea.Cmd` runs off the Update loop, which means a command factory has to copy the state it needs (`snap := *m`) before returning its closure. Reading the model inside the closure both races the loop and lets a key pressed while jj is working change what gets applied. Anything a command needs to write back comes home as a message, never as a mutation inside the closure.

Selection is an interface so patch generation can be tested without a full Model:

```go
type SelectionState interface {
    IsHunkSelected(filePath string, hunkIdx int) bool
    HasPartialSelection(filePath string, hunkIdx int) bool
    IsLineSelected(filePath string, hunkIdx, lineIdx int) bool
}
```

Keys are data. Dispatch, the `?`/`:` palette (aragonite `tui/palette`, browse and run modes), the footer, the which-key panel, and refused-key reasons (`gate(action)`) all come from the one table in `internal/model/keymap.go`, merged with aragonite `tui/keys` (`Global()`, `DiffView()`). Adding a key means a row there, its handler, and the README Keys section. `TestKeymap_AgreesWithShared` fails when jj-diff binds a shared key to a different action, and the palette and footers are pinned by `internal/model/testdata/help_*.golden`, `palette_*`, `footers.golden` and `empty_*`. Every jj call goes through the client's `trail.Trail`, which `gm` lists.

Patch generation in `internal/diff/patch.go` copies whole hunks as-is. For partial hunks it extracts the selected lines plus three lines of context on each side and recalculates the hunk header line counts.

## jj integration

Every write is one jj command with this program passed as the diff editor on the command line (merge-tools entry `jj-diff-selection`, defined per call so a user's own config cannot change what runs), all in `internal/jj/write.go`:

- `MoveChanges`: `jj squash --from S --into D` (or `-o`/`-A`/`-B` per `Placement`)
- `Split`: `jj split -r S -B S`, one call per group; `parallel` makes siblings. A chain's group patch includes every group before it, because jj shows each split the previous split's commit as its left side
- `Restore`: `jj restore --changes-in R`
- `Resolve` (`internal/jj/resolve.go`): `jj resolve -r R --tool :editor` with `ui.editor` set to jj-diff `--resolve-with PLAN`; jj runs it once per file in one operation, and it swaps in the picked text for the file whose content hashes as planned. Conflicts are read and written in `ui.conflict-marker-style=snapshot`, which is what makes the hash match

jj re-runs jj-diff with `--apply-patch`/`--unapply-patch`, and it rewrites `$right` to `$left` plus the selection without drawing. On jj 0.47.1 `$left` is the source's parent and `$right` the source, both holding only the changed files; jj's instructions file is turned off so it is not read as an added file. The editor is otherwise switched off, so a squash that would need one to combine descriptions fails cleanly (`-u`/`-k` get past it) instead of hanging. A failure restores the operation recorded before the command, so one `jj undo` undoes any write and no workspace is ever left behind.

This is the one place the application itself writes to a repository. It targets the user's repo at runtime, not this one.

The invariant that makes it safe: the selection is the only content that changes. What reintroduces the data-loss bug that shipped once already is a step that rewrites working-copy *content* beyond it: `jj restore` against the whole working copy, or anything that resolves a revset relative to `@` and then discards what it finds there. `tests/integration` pins it on real repos: a move leaves an uninvolved `@` at the same commit ID, a sibling move removes the hunk from the source rather than copying it, and an invalid patch makes zero operations.

## Theme system

`internal/theme/` initializes a global theme at startup. It auto-detects via `lipgloss.HasDarkBackground(os.Stdin, os.Stdout)`, honors `CATPPUCCIN_THEME=latte` or `macchiato`, and components read global colors such as `theme.Primary` and `theme.AddedLine`.

Force a theme in tests with `t.Setenv("CATPPUCCIN_THEME", "macchiato")` followed by `theme.Init()`. Lip Gloss v2 has no global background override, so the environment variable is the only seam.

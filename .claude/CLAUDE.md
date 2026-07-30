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

Selection is an interface so patch generation can be tested without a full Model:

```go
type SelectionState interface {
    IsHunkSelected(filePath string, hunkIdx int) bool
    HasPartialSelection(filePath string, hunkIdx int) bool
    IsLineSelected(filePath string, hunkIdx, lineIdx int) bool
}
```

Adding a keybinding touches four places: the `handleKeyPress()` case in `internal/model/model.go`, the handler itself, the help overlay in `internal/components/help/help.go`, and the README keybindings table.

Patch generation in `internal/diff/patch.go` copies whole hunks as-is. For partial hunks it extracts the selected lines plus three lines of context on each side and recalculates the hunk header line counts.

## jj integration

`MoveChanges` in `internal/jj/client.go` runs this sequence:

1. Resolve the destination revset to a change ID and record the current operation ID
2. `jj workspace add` a scratch workspace under a temp directory
3. In the scratch workspace: `jj new <destination>`, `git apply patch.diff`, `jj squash --into <destination>`
4. Always: `jj workspace forget` and delete the directory
5. On error, `jj op restore` back to the operation from step 1

This is the one place the application itself writes to a repository. It targets the user's repo at runtime, not this one.

The invariant that makes it safe: nothing in this path names `@`. The user's working copy is never read, written, or moved, so a failed or abandoned run cannot cost them unselected changes. Anything added here that resolves a revset relative to `@`, or runs `jj restore` against the user's working copy, reintroduces a data-loss bug that shipped once already.

## Theme system

`internal/theme/` initializes a global theme at startup. It auto-detects via `lipgloss.HasDarkBackground()`, honors `CATPPUCCIN_THEME=latte` or `macchiato`, and components read global colors such as `theme.Primary` and `theme.AddedLine`.

Force a theme in tests with `lipgloss.SetHasDarkBackground(true)`.

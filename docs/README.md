# jj-diff docs

A Bubble Tea TUI for Jujutsu diffs. Browse mode reads a revision with syntax
highlighting, search, and fuzzy file finding. Interactive mode selects hunks or
individual lines and moves them to another revision. Multi-split mode tags
changes and splits one commit into several. As jj's configured diff editor it
also backs `jj split`, `jj diffedit`, `jj amend -i`, and `jj squash -i`.

## Pages

- [Interface](./interface.md) for the modes, the panels, the keys, and the visual indicators
- [Workflows](./workflows.md) for worked examples of moving, splitting, and reviewing
- [Command line](./cli.md) for flags and diff-editor setup
- [Configuration](./configuration.md) for the environment variables
- [Troubleshooting](./troubleshooting.md) for rendering and jj integration problems

Setup and tasks live in [CONTRIBUTING.md](../CONTRIBUTING.md). Architecture,
patterns, and the testing guide live in [DESIGN.md](../DESIGN.md), and planned
work in [ROADMAP.md](../ROADMAP.md).

## Requirements

- jj 0.9.0 or newer, on `PATH`
- Go 1.25+, only to build or `go install` from source

## What it gives you

- Browse mode: read a diff with syntax highlighting for over 100 languages,
  search, and fuzzy file finding
- Interactive mode: move hunks or individual lines between revisions
- Multi-split mode: tag changes and split one commit into several focused commits
- Diff-editor mode: stand in for jj's builtin editor
- Display toggles for side-by-side layout, line numbers, whitespace-only changes,
  and word-level highlighting
- Catppuccin themes

## Performance

- 5,000+ files render in about 7μs
- 20,000+ line diffs render in about 52μs
- Search across 100K lines takes about 17ms

## Safety

No step destroys working-copy content. Moving changes resolves the destination,
records the current operation ID, adds a scratch workspace under a temp
directory, applies the patch and squashes there, then forgets the workspace. A
failure runs `jj op restore` back to the recorded operation, so an abandoned run
cannot lose the changes you did not select.

Splitting to a new commit is the one path that touches your working copy, and it
runs `jj new --insert-before`, which moves the parent pointer and leaves every
byte of content alone, the same way `jj split` does.

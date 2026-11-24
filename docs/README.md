# jj-diff docs

A Bubble Tea TUI for Jujutsu diffs. Browse mode reads a revision with syntax
highlighting, search, and fuzzy file finding. Interactive mode selects hunks or
individual lines and squashes them into another revision, splits them out to a
new commit, or discards them. As jj's configured diff editor it
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
- Interactive mode: squash hunks or individual lines into any revision, split
  them out to a new commit, or discard them, each as one undoable jj operation
- Diff-editor mode: stand in for jj's builtin editor
- Display toggles for side-by-side layout, line numbers, whitespace-only changes,
  and word-level highlighting
- Catppuccin themes

## Performance

- 5,000+ files render in about 7μs
- 20,000+ line diffs render in about 52μs
- Search across 100K lines takes about 17ms

## Safety

Every write is one jj command: `jj squash --from <rev> --into <dest> -i`,
`jj split -r <rev> -B <rev> -i`, or `jj restore --changes-in <rev> -i`, with
jj-diff passed as the diff editor on the command line. jj materializes the
revision's parent as `$left` and the revision as `$right`, and the child jj-diff
makes `$right` exactly `$left` plus the selection that was chosen on screen. jj then
does the rewriting, so only the selected lines move, the working copy changes
only when it is the revision being written, and `jj undo` reverses the whole
write. A write that fails after jj integrated it runs `jj op restore` back to the
operation recorded first.

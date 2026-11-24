# Workflows

## Move a few lines to the previous commit

```bash
jj-diff --interactive
```

Press `tab` to reach the diff and `]h` (then `n`) to reach the hunk, `v` for
visual mode, `j`/`k` to pick the lines, `space` to confirm, then `s` and `enter`
on `@-`. `jj undo` takes it back.

## Split a large commit into focused changes

```bash
jj-diff --interactive
```

Select the first group of changes with `space` (or `v` for lines), press `S`,
and describe the new commit. It lands beneath the revision, which keeps what you
did not select. Repeat for the next group.

## Review changes before committing

```bash
jj-diff
```

Browse mode is read-only. Search for TODO comments with `/`, narrow the file list
with `f`, and press `tab` to focus the diff, `]f`/`[f` to step files, and `]h`/`[h`
to step hunks.

## Edit a commit interactively

Configure jj-diff as the diff editor first, per
[the CLI page](./cli.md#as-jjs-diff-editor), then:

```bash
jj diffedit -r @-
```

Choose the changes to keep with `space`, refine to lines with `v`, and press `c`
to write them and exit. `q` leaves without writing.

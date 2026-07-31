# Workflows

## Move a few lines to the previous commit

```bash
jj-diff --interactive
```

Press `d`, select `@-`, and press `enter`. Reach the hunk with `n`, press `v`
for visual mode, pick the lines with `j`/`k`, press `space` to confirm, then `a`
to apply.

## Split a large commit into focused changes

```bash
jj-diff --interactive
```

Press `S` for multi-split mode. Tag related changes by letter: `a` on the UI
changes, `b` on the tests, and so on. Press `D` to open the assignment modal,
assign each tag to an existing commit or a new one, then `P` to preview and
apply.

## Review changes before committing

```bash
jj-diff
```

Browse mode is read-only. Search for TODO comments with `/`, jump between files
with `f`, and press `tab` to focus the diff and scroll it with `j`/`k`.

## Edit a commit interactively

Configure jj-diff as the diff editor first, per
[the CLI page](./cli.md#as-jjs-diff-editor), then:

```bash
jj diffedit -r @-
```

Choose the changes to keep with `space`, refine to lines with `v`, and press `a`
to save and exit.

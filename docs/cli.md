# Command line

```bash
# The working copy, in browse mode
jj-diff

# A specific revision
jj-diff -r @-

# Interactive mode, with a destination already chosen
jj-diff --interactive -d @-
```

## Flags

| Flag | Effect |
|------|--------|
| `-r`, `-revision` | Revision to view or edit, default `@` |
| `-i`, `-interactive` | Force interactive mode |
| `-browse` | Force browse mode, read-only |
| `-d`, `-destination` | Pre-set the destination revision |
| `-s`, `-side-by-side` | Start in the side-by-side layout |
| `-show-whitespace` | Visualize whitespace characters |
| `-tab-width` | Tab display width, default 4, where `0` falls back to the config value |
| `-scm-input` | Path to an scm-record input file, for compatibility mode |
| `-v`, `-version` | Print the version |

`jj-diff [LEFT RIGHT]` takes two positional paths, which is how jj invokes it as
a diff editor.

## As jj's diff editor

Point jj at jj-diff in `~/.config/jj/config.toml`:

```toml
[ui]
diff-editor = "jj-diff"
diff-instructions = false
```

Then the usual jj commands open it:

```bash
jj split          # split the current commit
jj diffedit       # edit the changes in a commit
jj amend -i       # amend interactively
jj squash -i      # squash interactively
```

Select the changes to keep with `space`, refine to individual lines with `v`,
and press `a` to save and exit back to jj.

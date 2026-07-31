# Configuration

jj-diff has no config file. Environment variables set the defaults, and flags
override them for one run.

| Variable | Values | Default | Effect |
|----------|--------|---------|--------|
| `JJ_DIFF_VIEW_MODE` | `unified`, `side-by-side` | `unified` | Diff layout |
| `JJ_DIFF_SHOW_WHITESPACE` | boolean | off | Visualize whitespace characters |
| `JJ_DIFF_SHOW_LINE_NUMBERS` | boolean | on | Show line numbers |
| `JJ_DIFF_TAB_WIDTH` | 1 to 16 | 4 | Tab display width |
| `JJ_DIFF_WORD_DIFF` | boolean | off | Word-level highlighting |
| `CATPPUCCIN_THEME` | `latte`, `macchiato` | auto | Force the theme |

Booleans are true only for `1`, `true`, `yes`, or `on`.

A value jj-diff cannot understand is ignored rather than reported, so the default
survives and startup never fails on a typo. `JJ_DIFF_TAB_WIDTH` outside 1 to 16
is dropped the same way.

Without `CATPPUCCIN_THEME`, the theme follows the detected terminal background.

## jj integration

Setting jj-diff as jj's diff editor lives on
[the CLI page](./cli.md#as-jjs-diff-editor).

# jj-diff itself

The TUI renders wrong. Set `TERM=xterm-256color`.

Very large diffs feel slow. Turn off syntax highlighting, or stay in browse mode.

jj integration fails. jj-diff shells out to `jj`, so make sure jj 0.9.0 or newer
is installed and on `PATH`.

A move failed and you want the old state back. jj-diff records the operation ID
before it writes and runs `jj op restore` on failure, so the repository should
already be where it started. `jj op log` shows the history if you want to check
or roll back further by hand.

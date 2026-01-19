# Project: jj-diff

## Version Control

This is a **non-colocated jj repository**. Critical constraints:

1. **READ-ONLY jj operations only** - Do not run any jj commands that modify repository state
2. **No git commands** - There is no `.git` directory; git commands will fail

### Allowed jj Commands

- `jj status`, `jj log`, `jj show`, `jj diff`
- `jj branch list`, `jj file show`, `jj file list`
- `jj op log`, `jj help`

### Forbidden Operations

- Any jj write operation: `new`, `commit`, `describe`, `edit`, `abandon`, `squash`, `split`, `rebase`, `restore`, `resolve`
- Any branch modification: `branch set/create/delete/move`
- Any git operation: `jj git *`, `git *`
- Operation history changes: `op restore`, `op undo`

Use `/jj` skill for detailed reference on read-only jj workflows.

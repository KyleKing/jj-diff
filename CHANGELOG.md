## v0.1.0 (2026-07-27)

### Feat

- support scm diff editing
- implement commit splitting
- improve panel UX and bugs with modals
- implement diff type toggle
- implement advanced search and syntax highlighting
- finish Phase 1
- add search and visual selection
- make progress on scm-diff-editor parity
- parity with scm-diff-editor

### Fix

- surface the apply key in diff-editor mode
- run the reload command after applying instead of returning it as a message
- start diff-editor mode with every hunk kept
- replay hunks over the left file when reconstructing a selection
- diff directories line by line instead of character by character
- emit real newlines in the jj log template so revisions parse
- measure help text before styling so descriptions are not cut short
- render nothing until the first window size arrives
- stop the panels panicking when the available width is tiny or negative
- **release**: build each target into its own dist path
- keep diff-editor writes inside the target directories
- reject hunk headers whose line numbers do not parse
- guard the unchecked type assertions in the filter and finder paths
- restore the repeated-word fixture dupword deleted
- report rollback failures in the jj client instead of dropping them
- improve color contrast

### Refactor

- drop unreachable render helpers and the file-finder entry point
- satisfy staticcheck quickfix suggestions
- drop duplicate chroma token aliases in the style switch
- begin to merge with template

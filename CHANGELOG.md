## v0.19.0 (2026-11-25)

### Feat

- every prompt is a vim field, so text is edited without leaving for nvim

## v0.18.0 (2026-11-25)

### Feat

- every prompt keeps the family contract: enter submits, ctrl+j newline, esc cancels
- space on the -i file list selects the whole file, and the list names the write keys

### Fix

- a long revision name gives way to the footer's write keys
- the squash picker counts the cursor's place, not a bare total
- esc never quits a root, and polish from the dogfood audit
- u undoes the session's last write, and R asks before discarding lines
- **deps**: aragonite v1.35.1

### Perf

- the file list lexes only the file the cursor rests on, and once

## v0.17.0 (2026-11-25)

### Feat

- detect Nerd-capable terminals for icons (aragonite v1.35.0)

## v0.16.3 (2026-11-25)

### Fix

- **deps**: aragonite v1.34.2 (no detached git maintenance in test homes)

## v0.16.2 (2026-11-25)

### Fix

- **deps**: update to my_go_template v0.24.4

## v0.16.1 (2026-11-25)

### Fix

- **launch**: the first key of a session acts instead of vanishing

## v0.16.0 (2026-11-25)

### Feat

- **reads**: space in browse marks the hunk read in the shared store

## v0.15.3 (2026-11-25)

### Fix

- **deps**: aragonite v1.32.0, so the welcome card only closes on the key that dismisses it

## v0.15.2 (2026-11-25)

### Fix

- return to the launcher once a --split or --resolve write lands

## v0.15.1 (2026-11-24)

### Fix

- **test**: take aragonite v1.31.1, so a hook's GIT_DIR never reaches a fixture

## v0.15.0 (2026-11-24)

### Feat

- **model**: a first-run welcome card of the mode's starter keys

## v0.14.0 (2026-11-24)

### Feat

- **reads**: file marks under the full change id, add U, and draw read as ▰

### Fix

- **test**: run every test in a home of its own and fail a run that reaches the real one

## v0.13.0 (2026-11-24)

### Feat

- **reads**: show jojo's read marks on a revision

## v0.12.3 (2026-11-24)

### Fix

- **theme**: draw selected rows through the skin on aragonite v1.27.1

## v0.12.2 (2026-11-24)

### Fix

- **deps**: take bubbletea v2.1.2 so the screen repaints after a hand-off

## v0.12.1 (2026-11-24)

### Fix

- **render**: lex each file section on its own, not each path

## v0.12.0 (2026-11-24)

### Feat

- **tui**: fit colors to the terminal's depth and hold frames to framecheck

## v0.11.0 (2026-11-24)

### Feat

- **tui**: gate keys with reasons, one palette for ? and :, a coach, gm, and empty cards
- **jj**: log every jj command in a trail and echo writes without the editor plumbing

### Fix

- **tui**: leave jj-diff on q or esc at the top of a --split or --resolve session

## v0.10.1 (2026-11-24)

### Fix

- **tui**: reset the scroll margins on a resize so a grown screen repaints in place
- **resolve**: show a side's whole description and size panes by content
- **split**: collapse an empty column to a strip and give its width away

### Perf

- **render**: draw a Document plain first and color it once the lexer lands

## v0.10.0 (2026-11-24)

### Feat

- **resolve**: pick conflict sides in a viewer and write them as one jj operation
- **split**: split a revision into columns, one per commit

### Fix

- **split**: land a chain whose earlier group took a whole file

## v0.9.0 (2026-11-24)

### BREAKING CHANGE

- keys follow the shared keymap. [ / ] -> [f / ]f; p -> N or
[h; n/N repeat the last motion or search; g/G -> gg/G; s -> \s (side by
side); l -> \n (line numbers); W -> \r (word diff); z -> za (plus zi zR zM
zz zt zb); F -> f (one fuzzy filter); r -> ctrl+l; d (destination) then a
(apply) -> s then enter, or c with -d; S / tag letters / D / P multi-split
-> S (split the selection out); e in the split preview -> removed. q in the
diff editor without writing now aborts the jj command.

### Feat

- **keys**: adopt the shared keymap and make every write one jj operation

## v0.8.1 (2026-11-24)

### Perf

- **startup**: load the diff while the terminal and the label are read
- **ui**: keep the file list, footer, and search hits between frames
- **diffview**: lex off the first frame, draw through pens, and keep drawn rows

## v0.8.0 (2026-11-24)

### Feat

- **ui**: draw every overlay in one frame and move search to the footer
- **render**: draw washes, the sign bar, and hunk labels for other tools

## v0.7.0 (2026-11-23)

### Feat

- **statusbar**: draw the footer as aragonite key hints beside quiet facts
- **filelist**: head the list with a labeled rule and draw paths with filetype icons
- **diffview**: draw changes with aragonite's sign bar, wash, and overview track
- **diff**: align side-by-side rows by content with aragonite's AlignRun

### Fix

- **statusbar**: pin the last hint so esc survives a visual selection

### Perf

- **diff**: list directory trees with WalkDir instead of Walk
- **diff**: skip unchanged pairs in directory comparison before reading them

## v0.6.0 (2026-11-14)

### Feat

- **view**: cache highlighting through aragonite, fold generated files and mechanical hunks, and render a window of rows

## v0.5.0 (2026-11-14)

### Feat

- **view**: tint changed lines, name the revision, and number each side by its own file

## v0.4.0 (2026-09-17)

### Feat

- **render**: render side-by-side diffs from the public package

## v0.3.0 (2026-09-16)

### Feat

- **render**: publish the diff renderer so other tools draw diffs the same way

## v0.2.4 (2026-09-03)

### Fix

- **jj**: insert a split's new commit beneath the source so its patch applies
- **model**: composite modals over the panels instead of replacing the screen
- **help**: scroll the keybinding overlay instead of clipping it
- **statusbar**: show interactive keys on the file list and fit hints by display width

## v0.2.3 (2026-08-31)

### Fix

- **scripts**: generate the tap deploy key inside 1Password

## v0.2.2 (2026-08-31)

### Fix

- **diffview**: only show line cursor in visual mode
- **filefinder**: guard renderMatch against small terminal widths
- **highlight**: use theme colors instead of hardcoded hex

### Refactor

- **diff**: delete unused WhitespaceRenderer

## v0.2.1 (2026-08-27)

### Fix

- open the split-assign modal from a message so it actually shows

### Refactor

- **model**: convert Model to pointer receivers and re-enable gocritic hugeParam

## v0.2.0 (2026-08-27)

### Feat

- bind F to the fuzzy file finder

### Fix

- clear multi-split state and hide the preview after a split applies
- skip jj's own JJ-INSTRUCTIONS scratch file in the diff editor

## v0.1.5 (2026-08-01)

### Fix

- **ci**: let mise own the Go toolchain and skip commitizen on an empty range

### Refactor

- **model**: split the key and view handlers into named steps

## v0.1.4 (2026-07-31)

### Fix

- import AGENTS.md so its instructions actually load

## v0.1.3 (2026-07-30)

### Fix

- **jj**: return the change ID jj actually created from createNewCommit
- **jj**: build the patch commit in a scratch workspace so the working copy survives
- **jj**: pin the destination revset to a change ID before the move starts

## v0.1.2 (2026-07-30)

### Refactor

- **search**: rename SearchState to State
- **diff**: rename DiffSource to Source

## v0.1.1 (2026-07-27)

### Fix

- **ci**: skip the jj integration tests when no jj binary is installed

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

# Findings

Written 2026-07-27 during a maintenance pass. Everything here is something I chose not to
change in code, either because the right answer is a design decision or because the change
is too large to make safely in a sweep. Bug fixes that were obvious went into commits
instead; this file is the remainder.

Everything below was reproduced against a real colocated jj repo with jj-diff wired in as
`ui.diff-editor`, driven through a pty. Nothing here is read off the source alone.

## Interactive mode destroys unselected working-copy changes

This is the one to fix first.

`moveChangesWithPatch` in `internal/jj/client.go:108` runs `jj restore --from <destination>`
as its second step, which resets the working copy to the destination's content. Every change
the user did not select is gone at that moment. The patch is then applied and squashed, so
the selected hunks survive and nothing else does.

To reproduce: a working copy with three modified files, select one hunk in one file, press
`a`. Afterwards `extra.txt` and the unselected `main.go` edit no longer exist on disk, the
working copy is empty, and the extra commit created by step 1 is left dangling in the log.
`jj op restore` gets it back, but the user is never told that.

The sequence also assumes the destination revset stays stable while `@` moves. It does not,
so `-d @-` can resolve to a different commit partway through.

Neither problem is a small fix. The sequence needs to stop touching the user's working copy
at all: resolve the destination to a change ID up front, build the new commit from the patch
alone, and squash that. Until then, interactive mode (`jj-diff -i`) is unsafe on a working
copy that matters, while diff-editor mode (`jj split`, `jj diffedit`, `jj squash -i`) is
fine because it writes only into the directories jj hands it.

## The Bubble Tea migration

The repo is on bubbletea 0.25.0 and lipgloss 0.10.0, both from early 2024. Current is
bubbletea 1.3.10 and lipgloss 1.1.0, and bubbletea v2 is a further break on top of that.
Dependabot PR #4 bundled both bumps with two other updates and was closed unmerged, which
was the right call for a drive-by upgrade and the wrong outcome for the codebase.

This needs a planned migration rather than an opportunistic bump, because the 0.25 to 1.x
break lands directly on the parts of this app that are already fragile:

- `Model.Update` returns `(tea.Model, tea.Cmd)` and the code type-asserts the result back to
  `Model` in `internal/model/testhelpers.go` and everywhere a command chains. v2 makes the
  model generic, which removes the assertions but rewrites every handler signature
- `lipgloss.HasDarkBackground()` is a package-level call in `internal/theme/theme.go:73`. In
  lipgloss 1.x background detection moves onto a renderer, so theme initialization stops
  being a global and has to be threaded through, or read from the background-colour message
  v2 sends. The current global is read at init time by every component
- Colour styles are constructed inline in view code (`lipgloss.NewStyle().Foreground(...)`)
  in at least `diffview.go`, `filelist.go`, `help.go`, and `highlight.go`, with several hex
  literals hard-coded in `internal/highlight/highlight.go` rather than referencing
  `internal/theme`. Every one of those is a touch point for the renderer change
- Key handling is a flat `switch msg.String()` over string literals. v2 replaces
  `tea.KeyMsg` with a key interface and encourages `key.Binding`. Moving to bindings would
  also fix the discoverability problems below, because the help overlay and the status-bar
  hints could be generated from the bindings instead of hand-maintained in parallel

Suggested order, each step shippable on its own:

1. Pull every hex literal and inline style into `internal/theme`, so the renderer change
   later has one place to land
2. Replace the string `switch` with `key.Binding` values, and generate both the help overlay
   and the status-bar hints from them. This deletes the four-places-to-edit rule in
   `.claude/CLAUDE.md`
3. Bump to bubbletea 1.3.10 and lipgloss 1.1.0 together, with the golden-file tests as the
   guard
4. Evaluate v2 separately, after 1 to 3 have settled

Do not do 3 before 1 and 2. The upgrade compiles clean and behaves differently, which is the
worst kind of change to make on top of untested view code.

## UX findings

Captured with VHS at 1400x900, 1000x620, and 660x420, in the default theme and with
`NO_COLOR=1`.

The layout is authored for this domain. A collapsing file list above a diff pane,
hunk-granular selection markers, and a side-by-side toggle are not what a generic
list-of-rows TUI would produce. The problems below are execution, not concept.

### Modals replace the whole screen

`internal/model/model.go:1527` returns the modal's view instead of compositing it over the
base view. Pressing `/` blanks the diff and shows a search box in the middle of an empty
screen, so you cannot see what you are matching while you type. Same for the destination
picker, the commit-message prompt, and the split preview. Overlaying with `lipgloss.Place`
over the rendered base, or dropping the search box into the status-bar row, both fix it.
This is the single biggest usability win available.

### The help overlay clips instead of scrolling

`renderModal` in `internal/components/help/help.go:199` centres a modal that can be taller
than the terminal, with no height budget and no scrolling. In a short terminal the
"Keybindings" title and the whole Navigation section are cut off the top, with nothing on
screen indicating there is more. A first-timer in a laptop-sized split pane sees half the
keys and cannot reach the rest.

### The status bar truncates silently

`truncateOrPad` in `internal/components/statusbar/statusbar.go:85` hard-clips at terminal
width with no ellipsis and no re-prioritisation. At 80 columns the browse-mode bar ends
mid-word at `/:searc`, so `f:find` and `?:help` are both invisible, and `?` is the only
route to the rest of the keys. The hint list should drop items from the right and show that
it did, or shorten to the highest-value few below some width.

### Filtering does not move the selection

`f` filters the file list correctly, but the diff pane keeps showing whatever was selected
before, and Enter closes the filter without selecting the top match. You type `ma`, see
`main.go` alone in the list, press Enter, and get `NOTES.md`. Either the filter should
follow the selection to the top match as you type, or Enter should commit the highlighted
row. That is a decision about how filter and search should differ, which is why it is here
rather than in a commit.

### Two competing file-finder implementations

`internal/components/filefinder/` is fully built, rendered by `Model.View`, and has a key
handler in `internal/model/model.go`, but nothing opens it. `f` opens the file list's inline
filter instead. The dead entry point was deleted in this pass; the component and its handler
were left alone because deleting a working modal is a bigger call than a lint sweep should
make. Pick one: bind the modal to a key and document it, or delete the package and its
handler. The README row for `f` said "Fuzzy file finder" and was corrected to match the code.

### The file list stores search state it never renders

`filelist.SetSearchState` is wired up from `model.go` and `getFilePathMatches` computes real
match ranges, but the render path builds its own columns and never highlights anything.
Diff-view search highlighting does work. The unused render helper that would have done it
was removed in this pass. Wiring it properly means mapping match offsets onto a path column
that truncates, plus ANSI-aware width maths in the `%-*s` layout, which is real work rather
than a reconnect.

### JJ-INSTRUCTIONS is presented as an editable file

jj writes a `JJ-INSTRUCTIONS` file into the right-hand directory and expects the editor to
ignore it. jj-diff lists it first, as an added file with six hunks, so the first thing a
user sees in `jj diffedit` is jj's own scratch file. `walkDirectory` in
`internal/diff/dircompare.go:49` should skip it, but confirm against current jj first
whether the name is stable and whether jj expects it left in place.

### Vertical space is unused

With three changed files in a 900-pixel-tall terminal, roughly two thirds of the screen is
blank. The file list is `m.height / 4` regardless of how many files there are, and the diff
pane is top-anchored with no fill. Sizing the file list to its content up to a cap would
give the diff most of the screen.

### Two cursors at once

In the diff pane both the hunk header and the first line carry a `>` marker, so it is
ambiguous which one `space` acts on. The hunk cursor and the line cursor need different
glyphs, or the line cursor should only appear in visual mode.

### Health score

| # | Heuristic | Score | Key finding |
|---|-----------|-------|-------------|
| 1 | Visibility of system status | 3/4 | Mode, source, and destination are always shown; no feedback at all while `MoveChanges` runs |
| 2 | Match system and real world | 4/4 | jj vocabulary throughout, change IDs shown as jj shows them |
| 3 | User control and freedom | 2/4 | Esc backs out of modals, but there is no undo hint after an apply and no confirmation before one |
| 4 | Consistency and standards | 3/4 | vim keys held consistently across views |
| 5 | Error prevention | 1/4 | `a` in interactive mode destroys unselected changes with no confirmation |
| 6 | Recognition over recall | 2/4 | `?` exists and the footer hints are good, but the footer truncates at 80 columns and the overlay clips |
| 7 | Flexibility and efficiency | 3/4 | Half-page and full-page scroll, visual mode, multi-split tagging; no command mode |
| 8 | Aesthetic and minimalist design | 3/4 | Chrome stays quiet; large blank areas read as unfinished rather than spacious |
| 9 | Error recovery | 2/4 | Errors render in-app as plain text, but a panic still dumps a Go stack over the terminal |
| 10 | Terminal portability | 3/4 | `NO_COLOR` respected and fully legible, `+` and `-` markers carry meaning without colour; no `TERM=dumb` handling |

26/40. Ordinary for a TUI at this stage, and the two lowest scores share one root cause: the
apply path is destructive and says nothing about it.

### Cognitive load

Three failures: working memory (the diff pane does not follow the filter, so you have to
remember what you were looking at), one-thing-at-a-time (file list, diff, and selection state
all compete on the first screen), and progressive disclosure (the help overlay dumps every
binding at once with no grouping by frequency). Moderate load, addressable.

## Deferred lint findings

679 findings remain from `mise exec -- golangci-lint run ./... --max-issues-per-linter=0
--max-same-issues=0`. All are style. The correctness linters are at zero: gosec, staticcheck,
errcheck, govet, and unused were 72 findings between them at the start of the pass and are
now clear.

The strict gate was left strict. No linter was disabled, no exclusion was added, and no
baseline was recorded, so CI will fail on these until they are cleared or a decision is made
to exclude them.

| Linter | Count | What clearing it involves |
|---|---:|---|
| revive | 312 | 261 are `exported` (a doc comment on every exported symbol) and 20 are `package-comments`. Mechanical but large, and whether every internal package needs doc comments is a real decision |
| paralleltest | 113 | Adding `t.Parallel()` to each subtest. Safe for the pure-function packages, needs care in `internal/model` where tests share a lipgloss global, and in `tests/integration` where each test spawns real jj |
| mnd | 91 | Magic numbers, mostly layout constants (column widths, padding, context-line counts) and file modes. Worth naming the layout ones; the file modes read fine as octal literals and are candidates for an exclusion |
| gocritic | 68 | 33 are `hugeParam` on the Bubble Tea value receivers, which is the framework's design and not fixable without switching to pointer receivers. 18 are `appendCombine`. The rest is a handful of if-else chains |
| testpackage | 15 | Moving tests to `_test` packages. Several test unexported functions and would need those exported or the tests split |
| wrapcheck | 12 | Wrapping errors returned straight from `os` and `filepath`. Cheap and worth doing |
| noctx | 11 | Switching `exec.Command` to `exec.CommandContext`. Worth doing properly: it would also give the TUI a way to cancel a slow jj call, which it currently cannot |
| err113 | 9 | Sentinel errors instead of `fmt.Errorf` with no verb. Cheap |
| nestif | 8 | Deeply nested conditionals, all in `internal/model` key handlers. Would resolve itself under the `key.Binding` refactor above |
| gocognit and gocyclo | 12 | `parseFileChange`, `computeHunks`, `Score`, and the round-trip test. Real complexity, worth splitting when each is next touched |
| prealloc, perfsprint, goconst, dupl, exhaustive, intrange, ireturn, lll, wastedassign | 31 | Small and mechanical |

Clearing revive's `exported` rule alone would take the total from 679 to roughly 400. That
and paralleltest are two thirds of the number and neither changes behaviour, so they are the
obvious first batch if the goal is a green gate.

## Smaller notes

- `internal/model/model.go` is 1536 lines and holds the mode enum, the selection state, every
  key handler, and the whole view. Splitting the key handlers per mode into their own files
  would make the diff-editor and interactive paths reviewable
- There is no test covering `reconstructAddedFile` or `reconstructDeletedFile`. The
  round-trip test added in this pass covers the modified-file path, which is where the bug
  was, but the other two are equally load-bearing
- `internal/diff/applier.go` writes files `0600` and creates directories `0750`. That is
  right for a temp directory, but an executable file handed to the editor would lose its bit.
  Worth checking how jj restores modes after the editor returns
- CI never runs the jj integration tests. `tests/integration` needs a real `jj` binary and
  the GitHub runners have none, so the suite failed on every push until it was changed to
  skip when jj is absent. That also unblocked releases, because goreleaser's `before` hook
  runs `go test ./...` and aborted the release job on the same failure. The real fix is to
  install jj in `.github/workflows/ci.yml`, which is template-managed, so it belongs
  upstream in my_go_template rather than as a local edit that conflicts on every update.
  Until then the integration tests are local-only, and they are the tests that caught the
  applier corruption in this pass
- `tests/integration` sits at 56.9% coverage and `mise run test:coverage-min` asserts 70%,
  but `mise run ci` runs only `go test` and `go build`, so the threshold is never enforced

# Findings

Things deliberately not changed in code, either because the right answer is a
design decision or because the change is too large to make safely in a sweep.
Obvious bug fixes went into commits instead; this file is the remainder.

Everything below was reproduced against a real colocated jj repo with jj-diff
wired in as `ui.diff-editor`, driven through a pty, and re-verified against the
source on 2026-07-27. Feature plans live in `ROADMAP.md`; the fixture-testing plan
lives in `doctest-jj-diff.md`; the append-only pass log is `.freshen.md`. Symbols
are named rather than line numbers, because line numbers drift.

## Bugs

### `ApplySplit` cannot create a new commit

Reproduced against v0.1.2 and against current main, and independent of the
working-copy data-loss bug fixed in v0.1.3. A split plan with
`SplitDestNewCommit` creates its destination as a child of the working copy, so
the destination already contains every change the working copy has. Applying the
plan's patch on top of that fails with `patch does not apply`, and the split is
rolled back.

Getting this right is a design decision rather than a repair. Taking selected
hunks out of `@` and into a new commit means creating that commit on `@-` and
rebasing `@` onto it, which is closer to what `jj split` does than to what
`MoveChanges` does. The existing test asserts only the safety property: the failed
plan is reported and leaves the working copy intact.

### `JJ-INSTRUCTIONS` is presented as an editable file

jj writes a `JJ-INSTRUCTIONS` file into the right-hand directory and expects the
editor to ignore it. jj-diff lists it first, as an added file with six hunks, so
the first thing a user sees in `jj diffedit` is jj's own scratch file.
`walkDirectory` in `internal/diff/dircompare.go` should skip it (it has no
reference to the name today), but confirm against current jj first whether the
name is stable and whether jj expects it left in place.

## The Bubble Tea migration

`go.mod` still pins bubbletea 0.25.0 and lipgloss 0.10.0, both from early 2024.
Current is bubbletea 1.3.10 and lipgloss 1.1.0, and bubbletea v2 is a further
break on top of that. Dependabot PR #4 bundled both bumps with two other updates
and was closed unmerged, which was the right call for a drive-by upgrade and the
wrong outcome for the codebase.

This needs a planned migration rather than an opportunistic bump, because the 0.25
to 1.x break lands directly on the parts of this app that are already fragile:

- `Model.Update` returns `(tea.Model, tea.Cmd)` and the code type-asserts the
  result back to `Model` in `internal/model/testhelpers_test.go` and everywhere a
  command chains. v2 makes the model generic, which removes the assertions but
  rewrites every handler signature
- `lipgloss.HasDarkBackground()` is a package-level call in
  `internal/theme/theme.go`. In lipgloss 1.x background detection moves onto a
  renderer, so theme initialization stops being a global and has to be threaded
  through, or read from the background-colour message v2 sends. The current global
  is read at init time by every component
- Colour styles are constructed inline in view code
  (`lipgloss.NewStyle().Foreground(...)`) in at least `diffview.go`,
  `filelist.go`, `help.go`, and `highlight.go`, with several hex literals
  hard-coded in `internal/highlight/highlight.go` rather than referencing
  `internal/theme`. Every one is a touch point for the renderer change
- Key handling is a flat `switch msg.String()` over string literals. v2 replaces
  `tea.KeyMsg` with a key interface and encourages `key.Binding`. Moving to
  bindings would also fix the discoverability problems below, because the help
  overlay and the status-bar hints could be generated from the bindings instead of
  hand-maintained in parallel

Suggested order, each step shippable on its own:

1. Pull every hex literal and inline style into `internal/theme`, so the renderer
   change later has one place to land
2. Replace the string `switch` with `key.Binding` values, and generate both the
   help overlay and the status-bar hints from them. This deletes the
   four-places-to-edit rule in `.claude/CLAUDE.md`
3. Bump to bubbletea 1.3.10 and lipgloss 1.1.0 together, with the golden-file
   tests as the guard
4. Evaluate v2 separately, after 1 to 3 have settled

Do not do 3 before 1 and 2. The upgrade compiles clean and behaves differently,
which is the worst kind of change to make on top of untested view code.

## UX findings

Captured with VHS at 1400x900, 1000x620, and 660x420, in the default theme and
with `NO_COLOR=1`. The layout is authored for this domain: a collapsing file list
above a diff pane, hunk-granular selection markers, and a side-by-side toggle are
not what a generic list-of-rows TUI would produce. The problems below are
execution, not concept. Ordered by payoff.

### Modals replace the whole screen

`Model.View` in `internal/model/model.go` returns the modal's view instead of
compositing it over the base view. Pressing `/` blanks the diff and shows a search
box in the middle of an empty screen, so you cannot see what you are matching
while you type. Same for the destination picker, the commit-message prompt, and
the split preview. Overlaying with `lipgloss.Place` over the rendered base, or
dropping the search box into the status-bar row, both fix it. This is the single
biggest usability win available.

### The help overlay clips instead of scrolling

`renderModal` in `internal/components/help/help.go` centres a modal that can be
taller than the terminal, with no height budget and no scrolling. In a short
terminal the "Keybindings" title and the whole Navigation section are cut off the
top, with nothing on screen indicating there is more. A first-timer in a
laptop-sized split pane sees half the keys and cannot reach the rest.

### The status bar truncates silently

`truncateOrPad` in `internal/components/statusbar/statusbar.go` hard-clips at
terminal width with no ellipsis and no re-prioritisation. At 80 columns the
browse-mode bar ends mid-word at `/:searc`, so `f:find` and `?:help` are both
invisible, and `?` is the only route to the rest of the keys. The hint list should
drop items from the right and show that it did, or shorten to the highest-value
few below some width.

### Filtering does not move the selection

`f` filters the file list correctly, but the diff pane keeps showing whatever was
selected before, and Enter closes the filter without selecting the top match. You
type `ma`, see `main.go` alone in the list, press Enter, and get `NOTES.md`.
Either the filter should follow the selection to the top match as you type, or
Enter should commit the highlighted row. That is a decision about how filter and
search should differ, which is why it is here rather than in a commit.

### Two competing file-finder implementations

`internal/components/filefinder/` is fully built, rendered by `Model.View`, and
has a key handler in `internal/model/model.go`, but nothing opens it. `f` opens
the file list's inline filter instead. The dead entry point is gone; the component
and its handler were left alone because deleting a working modal is a bigger call
than a lint sweep should make. Pick one: bind the modal to a key and document it,
or delete the package and its handler.

### The file list stores search state it never renders

`filelist.SetSearchState` is wired up from `model.go` and `getFilePathMatches`
computes real match ranges, but the render path builds its own columns and never
highlights anything (the doc comment on `SetSearchState` says so outright).
Diff-view search highlighting does work. Wiring this properly means mapping match
offsets onto a path column that truncates, plus ANSI-aware width maths in the
`%-*s` layout, which is real work rather than a reconnect.

### Vertical space is unused

With three changed files in a 900-pixel-tall terminal, roughly two thirds of the
screen is blank. `fileListHeight` is `m.height / 4` regardless of how many files
there are, and the diff pane is top-anchored with no fill. Sizing the file list to
its content up to a cap would give the diff most of the screen.

### Two cursors at once

In the diff pane both the hunk header and the first line carry a `>` marker, so it
is ambiguous which one `space` acts on. The hunk cursor and the line cursor need
different glyphs, or the line cursor should only appear in visual mode.

### Health score

27/40 on the ten usability heuristics. The three 2/4 scores are what to fix: user
control and freedom (no undo hint after an apply, no confirmation before one),
error prevention (`a` in interactive mode applies with no confirmation), and
recognition over recall (the footer truncates at 80 columns and the help overlay
clips). Visibility of system status is 3/4 only because there is no feedback at
all while `MoveChanges` runs, and error recovery is 2/4 because a panic still
dumps a Go stack over the terminal. `NO_COLOR` is respected and fully legible;
`TERM=dumb` is unhandled.

Cognitive load fails on three counts: working memory (the diff pane does not
follow the filter, so you have to remember what you were looking at),
one-thing-at-a-time (file list, diff, and selection state all compete on the first
screen), and progressive disclosure (the help overlay dumps every binding at once
with no grouping by frequency).

## Deferred lint findings

463 findings from `mise exec -- golangci-lint run ./... --max-issues-per-linter=0
--max-same-issues=0`, re-counted 2026-07-27. All are style. The correctness
linters are at zero: gosec, staticcheck, errcheck, and unused were 62 findings
between them before the sweep and are now clear. The `govet` count is
`fieldalignment` only, a layout suggestion rather than a correctness one.

The strict gate was left strict. No linter was disabled, no exclusion was added,
and no baseline was recorded, so CI fails on these until they are cleared or a
decision is made to exclude them.

| Linter | Count | What clearing it involves |
|---|---:|---|
| paralleltest | 113 | Adding `t.Parallel()` to each subtest. Safe for the pure-function packages, needs care in `internal/model` where tests share a lipgloss global, and in `tests/integration` where each test spawns real jj |
| mnd | 91 | Magic numbers, mostly layout constants (column widths, padding, context-line counts) and file modes. Worth naming the layout ones; the file modes read fine as octal literals and are candidates for an exclusion |
| gocritic | 82 | 48 are `hugeParam` on the Bubble Tea value receivers, which is the framework's design and not fixable without switching to pointer receivers. 20 are `appendCombine`, 8 are `emptyStringTest`, and 5 are if-else chains |
| revive | 39 | 19 `unused-receiver`, 7 `use-any`, 7 `unused-parameter`, and 6 singletons. The `exported` and `package-comments` rules are at zero |
| govet | 28 | All `fieldalignment`: reordering struct fields to shrink padding. Mechanical, and it churns every struct literal that lists fields positionally |
| testpackage | 16 | Moving tests to `_test` packages. Several test unexported functions and would need those exported or the tests split |
| gocognit, gocyclo, funlen | 13 | `parseFileChange`, `computeHunks`, `Score`, `help.View`, and the round-trip test. Real complexity, worth splitting when each is next touched |
| wrapcheck | 12 | Wrapping errors returned straight from `os` and `filepath`. Cheap and worth doing |
| noctx | 11 | Switching `exec.Command` to `exec.CommandContext`. Worth doing properly: it would also give the TUI a way to cancel a slow jj call, which it currently cannot |
| unparam | 11 | Parameters and returns that never vary, mostly error returns that are always nil. Worth doing, because each one is a signature that promises more than it delivers |
| err113 | 9 | Sentinel errors instead of `fmt.Errorf` with no verb. Cheap |
| nestif | 8 | Deeply nested conditionals, all in `internal/model` key handlers. Would resolve itself under the `key.Binding` refactor above |
| goconst, prealloc, dupl, perfsprint, exhaustive, intrange, ireturn, lll, wastedassign, nonamedreturns | 30 | Small and mechanical |

paralleltest is the largest block and changes no behaviour, so it is the obvious
next batch if the goal is a green gate.

Expect the total to fall by less than the number you clear. golangci-lint runs
with `uniq-by-line`, so it prints at most one issue per source line and whichever
linter reports first hides the rest. Clearing a rule uncovers whatever else sat on
the same lines: documenting the exported declarations cleared 130 `exported` and 7
`package-comments` findings but moved the total 567 to 459 rather than to 430,
because those lines were also carrying 13 `govet fieldalignment`, 13 `gocritic`, 1
`funlen`, and 2 other revive findings that had been masked.

## Smaller notes

- `internal/model/model.go` is 1583 lines and holds the mode enum, the selection
  state, every key handler, and the whole view. Splitting the key handlers per
  mode into their own files would make the diff-editor and interactive paths
  reviewable
- There is no test covering `reconstructAddedFile` or `reconstructDeletedFile`.
  The round-trip test covers the modified-file path, which is where the applier
  bug was, but the other two are equally load-bearing
- `internal/diff/applier.go` writes files `0600` and creates directories `0750`.
  That is right for a temp directory, but an executable file handed to the editor
  would lose its bit. Worth checking how jj restores modes after the editor
  returns
- CI never runs the jj integration tests. `tests/integration` needs a real `jj`
  binary and the GitHub runners have none, so the suite failed on every push until
  it was changed to skip when jj is absent. That also unblocked releases, because
  goreleaser's `before` hook runs `go test ./...` and aborted the release job on
  the same failure. The real fix is to install jj in `.github/workflows/ci.yml`,
  which is template-managed, so it belongs upstream in my_go_template rather than
  as a local edit that conflicts on every update. Until then the integration tests
  are local-only, and they are the tests that caught the applier corruption
- `tests/integration` sits at 56.9% coverage and `mise run test:coverage-min`
  asserts 70%, but `mise run ci` runs only `go test` and `go build`, so the
  threshold is never enforced

## Pending the next copier update

This repo is pinned at my_go_template v0.7.0; the template is at v0.9.0.

- `hk.pkl` carries a local `exclude` on `check-merge-conflict` for the roadmap,
  which documents jj's conflict-marker format verbatim. `hk.pkl` is
  template-managed, so this hunk lands in a `.rej` on every update and has to be
  re-applied by hand. It is the first case of a child legitimately overriding a
  template hook
- Template v0.8.0 adds a `typos` step that flags deliberate fixture strings in
  `internal/fuzzy/fuzzy_test.go`. The fix is a project-local
  `[default.extend-words]` in a `.typos.toml` here; the template file is the base
  and children extend it
- Template v0.8.0's `remove-if-found.txt` deletes the obsolete `Formula/jj-diff.rb`
  stub automatically. Nothing to delete by hand

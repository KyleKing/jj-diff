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

Re-swept 2026-08-01 with `mise exec -- golangci-lint run ./...
--max-issues-per-linter=0 --max-same-issues=0`. The gate is clean: 0 issues.

### `hugeParam` on the Bubble Tea root: excluded, not refactored

The last 42 findings from the previous sweep were one class: `gocritic hugeParam`
on `internal/model`, because `Model` is 768 bytes and gocritic flags every value
receiver over 80. The fix that clears it for real is converting `Model`'s
remaining value-receiver methods to pointer receivers and passing `&m` from
`cmd/jj-diff/main.go`, but that is a behaviour-risk change, not a style one:

- Every `tea.Cmd` factory (`loadDiff`, `applySelection`, `applySplit`,
  `loadRevisionsForSplitAssign`) returns a closure that reads model state when the
  command runs, not when it is built. Today the closure holds a snapshot. Under a
  pointer receiver it would observe whatever the model became in the meantime, so
  a key pressed between `a` and the command running could change which hunks get
  applied. Each factory would need an explicit `snapshot := *m` to keep today's
  behaviour
- Handlers that mutate and then return early currently discard the mutation,
  because they mutate a copy. Under a pointer receiver those mutations become
  permanent. Every handler would need reading for that pattern before the switch,
  which is an audit rather than a rename

Given that risk against a purely cosmetic finding, the decision was to add
`hugeParam` to `disabled-checks` under `[linters.settings.gocritic]` instead, both
locally in `.golangci.toml` and upstream in `my_go_template`'s
`go_template/.golangci.toml.jinja` (regenerated through `ctt`, which is how
`.ctt/*/.golangci.toml` picked up the change too). The template already disables
`gochecknoglobals` for the same underlying reason (Bubble Tea's package-level
style vars), so this follows an existing precedent rather than setting a new one.

The latent defect this class was masking is still open: `applySplit` ends with
`m.multiSplitState = NewMultiSplitState()` and `m.splitPreview.Hide()` inside a
`tea.Cmd` closure, both of which are discarded because the closure holds a value
copy. After a multi-way split is applied, the split state is never cleared and the
preview never hides. A comment marks the spot. `internal/jj/client.go` is the file
that shipped a data-loss bug, and the move path is what these closures drive, so
fixing this is a reviewed change, not a maintenance one, whenever the pointer
receiver conversion happens.

### What disabling `hugeParam` uncovered

golangci-lint's `uniq-by-line` means the linter that reports first on a line hides
the rest, so 42 lines carrying `hugeParam` were also carrying findings from other
linters that only surfaced once `hugeParam` stopped reporting:

- `ireturn` on `Model.Update`, because it returns `tea.Model`, the interface
  `tea.Program` requires. Resolved by adding
  `github.com/charmbracelet/bubbletea.Model` to `linters.settings.ireturn.allow` in
  both `.golangci.toml` files, which also closed out the `# revisit` comment that
  had been sitting on the `ireturn` entry in the enabled-linters list
- `unparam` on five key handlers (`handleSplitAssignKeyPress`,
  `handleCommitMsgKeyPress`, `enterSearchMode`, `handleFileListFilterKeyPress`,
  `handleFileFinderKeyPress`) whose `tea.Cmd` return is always `nil`. Each is
  dispatched through a switch (`routeToOverlay` or `handleActionKey`) alongside
  sibling handlers that do return a real command, so the signature is uniform by
  design rather than accidentally over-promising. Resolved with a `//nolint:unparam`
  and an explanation on each, following the project's existing pattern for genuine
  linter-vs-design conflicts

### Cleared in the 2026-08-01 sweep

paralleltest 113, mnd 91, gocritic 40 of 82, revive 39, govet 28, testpackage 16,
wrapcheck 12, noctx 11, unparam 11, err113 9, nestif 8, goconst 6, prealloc 5,
perfsprint 4, dupl 4, exhaustive 3, gocognit 10, gocyclo 2, lll 2, ireturn 2,
intrange 2, funlen 1, wastedassign 1, nonamedreturns 1.

Three new `//nolint` directives came out of it, each for a genuine conflict
between two enabled linters rather than to dodge a fix:

- `internal/model/testhelpers.go` `TestChanges`: `mnd`. The literals are hunk
  offsets and line numbers, which are the fixture's data
- `internal/fuzzy/fuzzy.go` `Score`: `unnamedResult` wants the results named and
  `nonamedreturns` rejects that
- `internal/highlight/highlight.go` `detectLexer`: `ireturn`, because returning
  chroma's registry interface is the design

Two follow-ups the sweep surfaced but could not finish inside its own scope:

- `noctx` was cleared with a `jjCommand` helper that passes
  `context.Background()`. Threading a real context means adding `ctx` to `Diff`,
  `GetRevisions`, `MoveChanges`, `ApplySplit`, and `diff.Source.GetDiff()`, which
  would give the TUI a way to cancel a slow jj call. It currently cannot
- `MoveChanges`'s `source` parameter is unused and is now `_`. The signature was
  left alone so `internal/model` still compiles; delete it properly when that call
  site is next touched

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

This repo is on my_go_template v0.9.1 as of 2026-08-01.

- `hk.pkl` carries two local overrides that land in a `.rej` on every update and
  have to be re-applied by hand: an `exclude` on `check-merge-conflict` for
  `ROADMAP.md`, which documents jj's conflict-marker format verbatim, and a guard
  on `commitizen-branch` so `cz check --rev-range origin/HEAD..HEAD` is skipped
  when the range is empty. cz exits non-zero on an empty range, so without the
  guard `hk check --all` cannot pass on a branch level with origin. Both belong
  upstream: the guard is not repo-specific
- `.github/workflows/ci.yml` had `actions/setup-go` removed from the `ci` job.
  Running it alongside `jdx/mise-action` puts two GOROOTs on PATH, and `go` then
  finds the other tree's `compile` and refuses to run
  (`version "go1.26.5" does not match go tool version "go1.25.0"`). The template's
  `go_template/.github/workflows/ci.yml.jinja` still carries the defect, so this
  local fix is reverted by the next update until it is back-ported
- CI still never runs the jj integration tests, because the runners have no `jj`
  binary and the suite skips when it is absent. Installing jj in the workflow is a
  template change, not a local one
- `.typos.toml` now carries a project-local `[default.extend-words]` for the
  truncated query prefixes used as fuzzy-match and search fixtures (`hel`,
  `functio`). The template file is the base and this extends it
- `filelist.renderExpanded` and `filefinder.renderMatch` both panic at very small
  terminal widths in some configurations, for example `maxWidth - 3` when
  `maxWidth` is 2. Pre-existing and untouched, because fixing it changes rendered
  output

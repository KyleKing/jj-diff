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

## The Bubble Tea migration

Done. `go.mod` now pins `charm.land/bubbletea/v2` and `charm.land/lipgloss/v2`,
which is where both modules moved for v2, and the go directive is 1.25.0 because
that is what they require. The jump went straight from 0.25.0 and 0.10.0 to v2
rather than through 1.x, so the 1.x renderer API was never a step this codebase
had to sit on.

What the break actually touched:

- `Model.View` returns `tea.View` instead of `string`. The body moved to an
  unexported `render() string` and `View` wraps it, setting `AltScreen` on the
  view. That replaces `tea.WithAltScreen()`, which no longer exists as a program
  option
- `Model.Update` still returns `(tea.Model, tea.Cmd)`, so the type assertions in
  `internal/model/testhelpers.go` stayed. v2 did not make the model generic
- `tea.KeyMsg` became an interface over `KeyPressMsg` and `KeyReleaseMsg`. Every
  handler takes `tea.KeyPressMsg` and the flat `switch msg.String()` survived
  untouched, so `key.Binding` is still available as a later change
- Space now arrives as `"space"` rather than `" "`, because v2 renders it through
  `Keystroke()`. The hunk-toggle case reads `keySpace`
- Text entry read `len(msg.String()) == 1`, which no longer admits a space and
  never admitted a multi-byte rune. The four entry points (search, file filter,
  file finder, and commit message) read `msg.Key().Text`, which is populated only
  for printable characters
- `lipgloss.Color` is a function returning `color.Color`, so the palette fields and
  the package-level colour vars in `internal/theme` are `color.Color`
- `lipgloss.HasDarkBackground` takes an input and an output file. `theme.Detect`
  passes `os.Stdin` and `os.Stdout` and keeps the global, which is the standalone
  workflow lipgloss documents. It returns dark when neither is a terminal. Reading
  `tea.BackgroundColorMsg` in `Update` instead would let the theme follow a
  terminal that changes background mid-session, and is the reason to revisit this
- `lipgloss.WithWhitespaceForeground` is gone in favour of
  `WithWhitespaceStyle(Style)`, in `searchmodal.go` and `filefinder.go`
- `linters.settings.ireturn.allow` in `.golangci.toml` named the old import path,
  so `Model.Update` started reporting until it was pointed at
  `charm.land/bubbletea/v2.Model`. That file is template-managed, so the same edit
  belongs in my_go_template

Still open, and independent of the migration:

- Styles are still built inline in `diffview.go`, `filelist.go`, and `help.go` rather
  than centralized, though each already references `internal/theme` colors
- Key handling is a flat `switch` over string literals, so the help overlay and the
  status-bar hints are still maintained in parallel with it

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

### The file list stores search state it never renders

`filelist.SetSearchState` is wired up from `model.go` and `getFilePathMatches`
computes real match ranges, but the render path builds its own columns and never
highlights anything (the doc comment on `SetSearchState` says so outright).
Diff-view search highlighting does work. Wiring this properly means mapping match
offsets onto a path column that truncates, plus ANSI-aware width maths in the
`%-*s` layout, which is real work rather than a reconnect.

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

Re-swept 2026-08-27 with `mise exec -- golangci-lint run ./...
--max-issues-per-linter=0 --max-same-issues=0`. The gate is clean: 0 issues, with `hugeParam`
enabled.

### `hugeParam` on the Bubble Tea root: converted to pointer receivers

Cleared on 2026-08-27. `internal/model` now uses pointer receivers throughout: handlers mutate
the receiver and return only a `tea.Cmd`, `Update` returns `m`, and `cmd/jj-diff/main.go` hands
`&initialModel` to `tea.NewProgram`. `hugeParam` is re-enabled locally, overriding the
template's blanket exclusion, so a value receiver cannot creep back in unnoticed.

The two hazards the earlier analysis named, and what was done about each:

- Command factories used to snapshot the model because the receiver was a value. `applySelection`,
  `applyDiffEditorSelection`, and `applySplit` now take an explicit `snap := *m` and the closure
  reads only `snap`, which is byte-for-byte the copy the value receiver used to make.
  `loadDiff` and `loadRevisions` bind the one field each needs instead.
  `TestApplySelectionUsesTheSelectionFromWhenTheKeyWasPressed` discriminates: it fails when the
  snapshot is replaced with the live pointer
- Handlers that mutate and return early. Every discarded-result call site in the pre-conversion
  file was a void method, so no mutation was ever being thrown away by a caller. The two places
  that genuinely relied on the copy were `render` and the `push*` helpers it calls, which install
  per-frame callbacks on the panel components. `render` now draws from an explicit `frame := *m`,
  which is the same copy the value receiver made. No test pins this: every scenario tried produced
  identical output with a shared frame, so the guarantee is structural rather than verified

`loadRevisionsForSplitAssign` was the one place where the discarded copy was a live bug rather
than a hazard: it called `closeAllModals`, `SetRevisions`, and `Show` inside the command, all on
the captured copy, so `D` never opened the split-assign modal. It now reports
`splitRevisionsLoadedMsg` and `Update` opens the modal, matching what `loadRevisions` already did
for the destination picker.

Re-enabling `hugeParam` also surfaced one finding outside `internal/model`:
`diff.NewWhitespaceRenderer` took a 648-byte `lipgloss.Style` by value and had no callers anywhere
in the module. It, and the `WhitespaceRenderer` type it built, were deleted.

### What the 2026-08-01 `hugeParam` exclusion uncovered

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
  `nonamedreturns` rejects that. Removed on 2026-08-27, because my_go_template v0.12.0
  disables `unnamedResult` globally for the same reason
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

- `internal/model/model.go` is 1762 lines and holds the mode enum, the selection
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
- `mise run ci` runs only `go test` and `go build`, so `mise run test:coverage-min`
  and its 70% threshold are never enforced by CI or the hooks

## Pending the next copier update

This repo is on my_go_template v0.13.0 as of 2026-08-31. `.cz.toml`'s version now
survives an update on its own, since v0.13.0 added the post-generation restore. The
AGENTS.md TUI-testing paragraphs this project contributed came back from the template
with no conflict. `docs/troubleshooting.md`'s project-specific entries (TERM, large
diffs, jj on PATH, `jj op restore`) moved to `docs/troubleshooting.local.md`, which
v0.13.0 added as the convention for content the template never renders.

Local drift that still has to be re-applied by hand on every update:

- `.golangci.toml` re-enables `hugeParam`, which the template disables. This stays local
  and is the sanctioned way to express it. Eight of the nine children want the template's
  exclusion, and the template's comment now names this override, so a copier question
  would add a fourth flag and a ctt variant to serve one repo

Still open:

- `.typos.toml` carries a project-local `[default.extend-words]` for the truncated query
  prefixes used as fuzzy-match and search fixtures (`hel`, `functio`). The template file is
  the base and this extends it

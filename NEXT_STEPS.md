# Next steps

What it would take to use jj-diff daily. Everything below was checked against the
working tree on 2026-09-02, not read off the older notes: the TUI was driven under
tmux at 120x40, 80x24, and 60x20, coverage was measured with the new
`test:coverage-min` task, and the jjui comparison comes from its published docs
rather than memory.

Known defects live in [FINDINGS.md](FINDINGS.md) and feature plans live in
[ROADMAP.md](ROADMAP.md). This file is the ordering, plus the scope decision the
rest of it depends on.

## Where it stands

The template update to
[v0.15.1](https://github.com/KyleKing/my_go_template/releases/tag/v0.15.1) is in
and all five gates pass. Two things that update changed are worth knowing, because
both bear on the work below. `test:coverage-min` now merges subprocess coverage, so
the integration tests that drive a built binary finally count, and the real number
moved from 44.7% to 52.4%. Dependabot now ignores `charmbracelet/x/exp/golden` and
`charmbracelet/x/vt`, which are untagged and are what `teatest` pulls in.

Coverage sits at 52.4% against a 70% floor, and nothing enforces it. `mise run ci`
runs `go test` and `go build` only, so neither CI nor the hooks have ever failed on
the threshold. The gap is concentrated rather than spread: `internal/jj` is 586
lines with no test file at all, and it is the one package that writes to a user's
repository. `internal/theme`, `cmd/jj-diff`, and eight of the ten UI components are
also at zero.

## Scope, decided

jj-diff stays the diff tool. The full jj TUI becomes a sibling project, or an extension
of one that exists. Decided 2026-09-02, and the docs now agree: README says the tool does
not browse a repository, ROADMAP keeps only the three features you want while reading a
diff, and `docs/tui-v2-requirements.md` carries a note saying its premise no longer holds.

What settled it is what [jjui](https://github.com/idursun/jjui) actually does. Its
[Details view](https://idursun.github.io/jjui/details/) selects at whole-file level only,
and both `s` (split) and `r` (restore) hand hunk-level work to an external interactive
editor. Hunk and line selection is the thing jj-diff already does and jjui delegates, so
the two compose rather than compete. Rebuilding jjui would mean chasing a tool at v0.10
with an actions-and-bindings system and Lua scripting, and giving up the one place
jj-diff is ahead.

The sibling is also much cheaper than it was in August, which is the other half of the
answer. [aragonite](https://github.com/KyleKing/aragonite) now ships `vcs`, `forge/github`,
`cache`, and `tui/theme`, so a repo browser with a `gh`-backed PR pane starts from those
rather than from nothing. That is where the `o` binding for opening a PR belongs, and
where `docs/tui-v2-requirements.md` should move when the project exists.

## Ordered work

### 1. Wire it in and use it

Nothing else is worth ordering until real use says what hurts. `jj-diff` is not on
PATH and no `ui.diff-editor` is configured, so it has never run as jj invokes it.
`mise run dev` covers browsing from source, and the diff-editor path needs an
installed binary because jj shells out to a fixed command.

### 2. The four defects that made it unpleasant

All four are fixed and on `main` as of 2026-09-02, each reproduced under tmux first and
each with a test that fails without the fix.

The interactive status bar showed browse hints, because `getContextHints` tested the
focused panel before the mode and the file list is where you start. `d:dest` and
`a:apply` were invisible until you pressed Tab, so interactive mode's whole vocabulary
was undiscoverable. Hints are a slice now, fitted by display width rather than byte
length, dropped from the right with an ellipsis, and the help hint always survives.

Modals replaced the screen. `render` composites the box over the panels, so the diff
stays readable while you type a search. `Canvas.Compose` was the trap here: it draws
every layer at the canvas origin and discards X and Y, and `NewCompositor` is the type
that honours them.

The help overlay clipped at 24 rows, losing the title off the top and the close hint off
the bottom. It windows to the terminal height now and the footer reports the position.

`ApplySplit` could not create a new commit, which had never worked in any release. The
cause was direction: `MoveChanges` only ever adds a patch to its destination, so a source
stops reporting hunks only when the destination is one of its ancestors, and the old code
created the commit as a child of `@`. It now runs `jj new --insert-before <source>`, which
is what `jj split` does.

That last one narrowed an invariant rather than working around it. `.claude/CLAUDE.md`
used to say nothing in this path may name `@`. It now says no step may destroy
working-copy content, which is what the rule was protecting: a rebase moves a parent
pointer and leaves every byte alone.

### 3. Adopt aragonite, and push code back up

[aragonite](https://github.com/KyleKing/aragonite) is not imported at all today.
Two packages fit now.

`tui/theme` replaces `internal/theme`. aragonite ships the full Catppuccin palette,
the same `CATPPUCCIN_THEME` override, and a deterministic detect rule that resolves
to Macchiato when it cannot query a background, which is what makes a piped run and
a golden fixture agree. jj-diff's twelve-field `Theme` collapses to a `Palette` plus
`Semantic`, and the diff-specific names (`AddedLine`, `WordDiffAddedBg`,
`SelectedBg`) stay local, which is the layering rule aragonite's DESIGN.md already
states.

`vcs` covers the read side of `internal/jj`. `vcs.JJOperations` has no concept of a
hunk or of `MoveChanges`, so most of `client.go` stays where it is. What moves is
revision listing and repository detection, and the win is that a git-repo fallback
becomes reachable, which is what the forge vocabulary question needs.

Two things should travel the other way once a second consumer appears. The diff
parser and patch generator in `internal/diff` are the only implementation of
hunk-level and line-level patch construction across your tools, and aragonite's
`vcs` has no diff-editing story. Neither should move before something else needs it,
which is aragonite's own rule.

### 4. Coverage, with fewer tests

`teatest/v2` drives the real `tea.Model` and diffs golden frames, so one test can
cover a whole flow. A handful of them covering open, filter, select a hunk, choose a
destination, and apply would reach further than the current 150 unit tests do, and
they would have caught all three rendering defects above. Expect to delete some
narrow unit tests once the flows are covered.

`internal/jj` still has no test file of its own. `tests/integration` covers the two
write paths against real repositories built in `t.TempDir`, which is the aragonite
`vcs` pattern and is the right place for them, so what is missing is the rest:
`Diff`, `Status`, `ShowRevision`, `GetRevisions`, and the parsers behind them. It is
the one package that can lose a user's work, so it earns the coverage first.

Then turn the gate on. Add `test:coverage-min` to `mise run ci`, set the floor to
whatever is real at that point, and raise it as tests land. A 70% threshold that
never runs is worth nothing.

### 5. Layout and the things use will turn up

`internal/model/model.go` is 1762 lines holding the mode enum, the selection state,
every key handler, and the whole view. Splitting handlers per mode into their own
files is what makes the diff-editor and interactive paths reviewable, and it is
worth doing before adding a mode rather than after.

Two smaller items from the tmux run that are not yet written down anywhere. Both
panes reserve fixed proportions, so two changed files leave eight blank rows above a
diff that has room to spare. And `internal/diff/applier.go` writes files `0600`,
which is right for a temp directory, though it is worth checking what jj restores
after the editor returns for a file that had its executable bit set.

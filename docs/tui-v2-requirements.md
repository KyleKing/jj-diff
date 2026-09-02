# Requirements: a full jj-native TUI

**These requirements no longer describe jj-diff.** They were written on 2026-08-19 on the
premise that jj-diff would grow into a full jj TUI in place. That premise was dropped on
2026-09-02: jj-diff stays the diff tool, and the full TUI becomes a sibling project. The
requirements below survive because the sibling will want them, not because this repo is
going to build them. Read them as a spec looking for a home.

What changed the decision is what [jjui](https://github.com/idursun/jjui) turns out to do.
Its Details view selects at whole-file level only, and both split and restore hand
hunk-level work to an external editor, which is exactly jj-diff's job. Rebuilding that
surface would mean chasing a tool at v0.10 with a Lua binding system, and it would replace
the one thing jj-diff is already better at. The sibling also starts far cheaper than it
would have in August, because [aragonite](https://github.com/KyleKing/aragonite) now ships
`vcs`, `forge/github`, `cache`, and `tui/theme`.

The rest of this file is the original text. Scope, as decided going in: this stays a TUI
(not a browser app), GitHub is the only forge target for day one with a provider
abstraction underneath for GitLab/Forgejo later, and the op-log sync workaround piggybacks
on the git remote the user already pushes to. See [tui-comparison.md](tui-comparison.md)
for how lazyjj and jjui were evaluated.

The throughline across every section below: design around what jj actually is (no
staging area, the working copy is always a commit, every mutation is a reversible
operation), not around a git mental model with jj commands substituted in.

## What jj's paradigms mean for the design

- **No staging area.** There's nothing for a TUI to model as an "index" panel or a
  `git add -p`-style staging flow distinct from committing. jj-diff's existing hunk/line
  selection already targets this correctly: selection produces a patch that gets
  squashed into a destination commit directly, not staged first.
- **The working copy is always commit `@`.** Every view that shows "current state" is
  really showing one specific, already-committed revision. This is why jj-diff's own
  `MoveChanges` code is careful never to name `@` in its scratch-workspace sequence
  (`internal/jj/client.go`) — anything new that touches `@` needs the same discipline,
  because an in-progress TUI operation that goes wrong could otherwise cost the user
  their actual working state, not a disposable staging area.
- **The operation log is the undo mechanism, not a side feature.** Every mutating jj
  command is itself a reversible operation (`jj op log`, `jj undo`, `jj op restore`).
  This changes the confirmation-dialog calculus from the tui-design skill's severity
  table: an action that's fully reversible via `jj undo` doesn't need the same friction
  as a genuinely irreversible one. Concretely: routine edits (describe, squash, split,
  rebase, abandon, duplicate) should execute immediately with a status-bar confirmation
  and a visible "press `u` to undo" hint, not a "press y to confirm" gate. Reserve actual
  confirmation prompts for things `jj undo` can't cleanly reverse: `jj git push` (once
  it's left the machine), force-push, and anything that runs `--ignore-immutable`.
- **Conflicts are commit state, not a blocking error.** A rebase that would conflict in
  git still succeeds in jj — the conflict lives inside the commit and descendants keep
  rebasing on top of it. The log view has to show conflicted commits as a distinct,
  browsable state (not a modal error), matching how `jj log` itself marks them.
- **Bookmarks don't auto-advance.** There's no "current branch" the way git has one; `@`
  is independent of any bookmark. A status bar that tries to show "on branch X" the way
  lazygit does is showing the wrong thing for jj — show the bookmark(s) pointing at or
  near `@`, not a single "current branch."
- **Revsets are a live filter, not a one-shot query.** The idiomatic jj workflow is
  editing a revset expression and re-running it repeatedly (`jj log -r '<expr>'`), often
  against a custom alias like `mine() ~ ::trunk()`. The log view's revset box should
  behave like jjui's, autocomplete plus signature help while typing, not a plain text
  field you submit once.

## Log and graph browsing

This is the biggest capability gap between jj-diff today and both lazyjj and jjui, and
the main new surface area.

- Default view driven by `revsets.log` (same default jj itself uses:
  `present(@) | ancestors(immutable_heads().., 2) | trunk()`), with a persistent, editable
  revset box (jjui's `shift+l` pattern) that supports autocomplete over revset functions
  and live re-filtering as you type, not just on submit.
- Immutable commits (`immutable_heads()` and its ancestors) render dimmed/non-selectable
  for direct edits, matching jj's own protection model, with an explicit
  `--ignore-immutable`-equivalent escape hatch that requires the confirmation step called
  out above.
- Conflicted commits get a distinct marker in the graph, not a separate error state.
- Op log as its own scoped view (jjui's `o` pattern), with restore/revert/diff actions
  per operation, since this is jj's actual undo history and deserves first-class
  browsing, not just a `jj undo` keybinding with no visibility into what else is
  restorable.
- Multi-select (space to toggle) for batch actions, mirroring jjui's `context.checked_*`
  pattern, e.g. push all eligible bookmarks on selected revisions in one action.

## Diff viewing (jj-diff's existing strength, kept and extended)

jj-diff's diff rendering (syntax highlighting, side-by-side mode, search, fuzzy file
finder) is already deeper than what lazyjj or jjui do inline, per the tui-comparison
ratings. That doesn't change. What's worth adding once diff viewing sits inside a
broader log-browsing tool:

- Diff-range mode (jjui's `alt+d`): pick a from/to revision pair with a swap key, not
  just parent-vs-revision.
- File-stepping (`[`/`]`) that survives every diff entry point (revision diff, range
  diff, op log diff), which jjui explicitly can't guarantee for its Lua-driven
  `diff.show()` path, since raw text pushed into a panel loses the command context needed
  to know what "next file" means. Keeping diff-viewer state as real structured data,
  never a text dump, avoids inheriting that limitation.
- Evolog view (jjui's `v`): diff a revision against its own previous evolution entry, useful
  for reviewing what a `describe`/`squash`/`rebase` actually changed before it's undone.

## Editing operations, surfaced directly (no modal "rebase mode")

Because every jj edit command is atomic and independently undoable, the natural TUI
shape is direct per-commit actions in the log view, not a staged "enter rebase mode, edit
a plan, run it" flow the way git's interactive rebase works. Bind single mnemonics on the
selected commit, matching the lingua franca jjui and lazyjj both already use:

| Action | Command | Notes |
|---|---|---|
| New child | `jj new` | Multiple parents = merge |
| Edit | `jj edit` | Only on mutable commits |
| Describe | `jj describe -m` | Inline text box, `$EDITOR` fallback |
| Squash | `jj squash [-i] [--into]` | `-i` opens jj-diff's own hunk-selection UI, reused rather than shelling to `jj squash -i`'s own interactive mode |
| Split | `jj split [-i]` | Same hunk-selection UI, opposite direction |
| Absorb | `jj absorb [paths]` | Auto-routes working-copy edits to the right ancestor; worth a dedicated keybinding given how central this is to the "squash workflow" jj users actually use day to day |
| Rebase | `jj rebase -r/-s/-b --onto/-A/-B` | Single-commit, subtree, or branch scope, picked explicitly, not inferred |
| Duplicate | `jj duplicate --onto/-A/-B` | |
| Parallelize | `jj parallelize` | Turns a chain into siblings |
| Abandon | `jj abandon` | Descendants auto-rebase; reversible via undo, so no confirm prompt needed |
| Undo / redo | `jj undo` / `jj op restore` | Always-visible footer hint, not buried in a menu |

### Interactive stack editing: wrap `jj arrange`, don't reinvent it

jj shipped a native interactive stack editor, `jj arrange`, in v0.39.0 (current jj is
v0.44.0 as of this research). It's DAG-shaped rather than a flat todo list like git's
interactive rebase, since jj commits can have multiple children — it shows the target
revisions plus dimmed, non-selectable parent/child context, and can move a commit along
graph edges even across non-adjacent rows. Split and edit are deliberately excluded from
it by design (jj already has dedicated commands for those, listed above).

The right move is to shell out to `jj arrange -r <revset>` for this rather than
reimplementing a todo-list UI, the same way `MoveChanges` already shells out to jj rather
than reimplementing rebase logic. Two things need verifying hands-on before committing to
this, since research couldn't confirm them from docs alone: what `jj arrange`'s actual
per-row action keys are (no firsthand walkthrough found, only the CLI reference and
changelog text), and whether it covers squash/fixup-equivalent behavior or only
reordering (the original feature request explicitly scoped it to reordering). If it turns
out arrange doesn't cover squash-while-reordering, the composed primitives (`rebase
-r --before/--after`, `squash --into`) remain the fallback for anything arrange doesn't
do, and for users on a jj version older than 0.39.

### Bisect: wrap the scripted case, treat manual bisect as a future gap

`jj bisect run --range <revset> -- <cmd>` is real and shipped (v0.34.0), not experimental.
It mirrors `git bisect run`'s exit-code contract (0 good, 125 skip, 127 abort) and exposes
the candidate via `$JJ_BISECT_TARGET`. Wrapping this for day one is straightforward: a
form that collects the range and command, runs it, and streams output.

What doesn't exist yet is a manual, TUI-driven "mark this good/bad and jump to the next
candidate" mode — there's no `jj bisect start/good/bad` the way git has one. This is a
confirmed open gap ([jj-vcs/jj#9080](https://github.com/jj-vcs/jj/issues/9080), open, no
linked PR), for cases where the test needs a human in the loop between steps (their
reported case: a reboot between checks). Building this ourselves means owning the binary
search state client-side (track good/bad revsets, compute the midpoint, `jj edit` into
it) with no CLI shortcut to lean on. Scope this as explicitly future, after the
scripted-bisect wrapper ships, and revisit once #9080 lands upstream since it would make
the client-side state machine unnecessary.

## GitHub integration (day one), provider-agnostic underneath

The concrete gap driving this whole project: neither lazyjj nor jjui has forge
integration beyond generic shell-out escape hatches, and jjui's Lua layer specifically
can't render a custom panel for it (confirmed: its only output surfaces are a flash
message, a modal picker over a fixed string list, and pushing raw text into the existing
diff/preview pane — no custom widget, no table, evidenced by an still-open "arbitrary
revision picker" feature request against a narrower ask than a full panel). Doing this in
Go, in-process, rather than through a scripting layer with a rendering ceiling, is the
actual reason to build rather than extend.

- Single GraphQL query per repo per poll, shaped like:
  ```graphql
  pullRequest(number: $n) {
    number
    reviewDecision
    mergeStateStatus
    baseRefOid
    headRefOid
    commits(last: 1) {
      nodes { commit { statusCheckRollup {
        state
        contexts(first: 100) {
          checkRunCountsByState { state count }
          statusContextCountsByState { state count }
        }
      } } }
    }
  }
  ```
  Prefer the `*CountsByState` aggregates over enumerating every check context, which is
  what makes this cheap regardless of how many checks a PR runs. Ahead/behind isn't on
  the PR object; pair this with `GET /repos/{owner}/{repo}/compare/{base}...{head}` for
  exact counts, or treat `mergeStateStatus == BEHIND` as a boolean proxy if exact counts
  aren't needed for the status line.
- Poll via a `tea.Tick`-driven `tea.Cmd`, entirely decoupled from `View()` — never a
  fetch inside the render path. At roughly 1-5 GraphQL points per poll against a
  5,000-point/hour budget, a 15-60s interval while the relevant view is focused costs
  nowhere near the budget; the actual failure mode other tools hit (gh-dash's own
  reported rate-limit incidents) is per-row fan-out, one call per PR or per check, not
  polling frequency. Stick to one query per repo per poll. Back off or pause entirely
  when the view isn't focused. Read `rateLimit.remaining` opportunistically rather than
  hardcoding a "safe" interval.
- Three distinct UI states, not one collapsed "loading" spinner: fresh, stale-but-cached
  (render immediately from last-known state on startup, never block on a fetch), and
  currently-fetching. Manual refresh key alongside the background poll, matching gh-dash's
  interval-plus-manual-override pattern.
- Provider interface shaped around "aggregate CI state plus optional per-check
  breakdown," since GitHub's check-run granularity, GitLab's job/pipeline granularity,
  and Gitea/Forgejo's flat commit-status list don't line up 1:1 — modeling the common
  case as one aggregate state with an optional deeper per-check list lets the GitHub
  provider fill in real detail while a future Gitea provider can populate just the
  aggregate. `gbarany/tea-dash` is worth a look as a second reference implementation of
  this same dashboard shape already built against Gitea's flatter model.

## Workspace management (an actual gap in both existing tools)

Neither lazyjj nor jjui exposes `jj workspace add`/`forget` as a user-facing feature
(jjui only has a Lua-level `change_workspace()` that redirects which workspace subsequent
commands target, no add/forget UI). This is worth adding as a real panel, since
workspaces are a documented day-to-day jj workflow (running a slow test suite in one
workspace while continuing to edit in another), not just jj-diff's own internal
scratch-workspace mechanism.

Keep these two uses of workspaces architecturally separate even though they share the
same underlying jj commands: the user-facing workspace panel operates on workspaces the
user explicitly creates and names, while `MoveChanges`'s scratch workspace stays
disposable, unnamed-to-the-user, and never surfaced in this new panel. Conflating them
risks a user accidentally forgetting or deleting a workspace jj-diff itself is mid-use
with.

## Conflict resolution (thin in jjui today, room to do better)

jjui marks conflicted files in its details view but delegates resolution entirely to an
external tool via `jj resolve --tool <name>` run through a user-authored Lua binding.
There's no built-in resolution UI. Given jj-diff already has a real diff/hunk-selection
engine, the opportunity is to surface conflict markers inline in that same engine rather
than always shelling out: show the conflict's structural sides (jj stores conflicts
logically, not as pre-rendered text markers, until you check them out) and let the
existing hunk-selection UI drive picking a side or hand-editing, falling back to
`jj resolve --tool` for cases genuinely better handled by an external merge tool.

## Long-term: sharing the operation log across machines via the git remote

This is an explicit workaround, not a permanent design. jj has no export/import command
for the op log today (`jj util gc`, `jj git export/import`, and `--at-op` all operate on
the local store in place; none serialize it) and the jj team's own position, from
[discussion #45](https://github.com/jj-vcs/jj/discussions/45), is that whole-`.jj`-directory
file sync (rsync/Syncthing) is the supported approach, with a "shadow branches on the
git remote" approach explicitly considered and rejected there as unnecessary once rsync
already works. That calculus is different for someone who doesn't want to run rsync
between machines and already pushes to a git remote on every session, hence this
workaround. Mark it inert by default and name its removal condition:

**Removal condition:** delete this feature once jj ships a native op-log sync mechanism.
The closest tracked upstream signal is discussion #45 itself (an unimplemented request for
an auto-sync daemon); watch that thread, and re-evaluate this workaround whenever jj cuts
a release whose changelog mentions op-log sync.

### Design

- **Packaging.** Tar `.jj/repo/{op_store,op_heads,operations}`, encrypt with
  [age](https://github.com/FiloSottile/age) to a machine-local recipient key (no keyring,
  no passphrase prompt if a local identity file is used), then `git hash-object` the
  ciphertext into a blob under a synthetic commit (`git commit-tree`).
- **Transport.** One ref per machine, `refs/jj-opsync/<machine-id>`, pushed with an
  explicit refspec (`git push origin <local-ref>:refs/jj-opsync/<machine-id>`) alongside
  the user's normal `git push`. Per-machine refs sidestep the non-fast-forward race two
  machines writing to one shared ref would hit. Force-overwrite each push (a fresh,
  single-parent commit) rather than chaining sync history, so the remote-side footprint
  stays bounded instead of growing every session — this is a deliberate choice against
  keeping time-travel through past syncs, since the op log itself is already the history
  that matters.
- **Fetch.** Add the mirroring fetch refspec once (`refs/jj-opsync/*:refs/jj-opsync/*`)
  so a plain `git fetch` picks it up automatically on every future pull, no bespoke
  tooling knowledge needed per machine after initial setup. Decrypt with the local age
  identity, unpack, and merge into the local op store — relying on jj's own lock-free
  concurrent-op-head 3-way merge to reconcile it with whatever local operations happened
  since the last sync, the same mechanism that makes rsync safe today.
- **Pruning.** Run `jj op abandon ..<cutoff>` then `jj util gc` before packaging each
  time, so the payload doesn't grow unboundedly. No published number exists for typical
  `.jj/repo/op_store` size, so measure it empirically (`du -sh .jj/repo/op_store`) on a
  real active repo before deciding a retention window, rather than guessing one.
- **Security posture.** Off by default, opt-in via explicit config (not inferred from
  repo visibility, since visibility can change after the fact and this workaround has no
  way to know that at push time). Encrypt unconditionally when enabled, no "skip
  encryption because the repo is private" fast path — op log entries can carry content
  from commits that were abandoned or `jj undo`'d and never became reachable in visible
  history, which is a larger exposure than a normal branch push and GitHub won't purge on
  its own. Single-user scope for now (one age recipient key); a shared/multi-user variant
  (multiple recipient keys on the same encrypted payload) is a plausible later extension
  but not a day-one requirement.
- **Host caveat.** Confirmed safe on GitHub and GitLab (arbitrary custom ref namespaces
  are an established pattern, e.g. `git-bug` and `git-appraise` already do the same
  shape of thing); unconfirmed on Forgejo specifically, verify before relying on it there.
  GitHub's 100 MiB per-blob push limit caps how much can go in one sync commit without
  splitting, independent of the ref namespace.

## Keybinding layering

Following the tui-design skill's four-layer model, and reusing jj-diff's, jjui's, and
lazyjj's existing conventions rather than inventing new ones where they already agree:

- **L0 (universal, always in footer):** arrows, `Enter`, `Esc`, `q`, `Tab` to switch
  focus.
- **L1 (vim motions, always in footer):** `j`/`k`, `/` search, `?` help, `:` command/exec
  box, `g`/`G` top/bottom.
- **L2 (mnemonics, in `?` overlay):** `n` new, `e` edit, `d` describe, `s` squash,
  `S` split, `a` abandon, `r` rebase, `b` bookmarks, `o` op log, `u`/`Ctrl+u` undo/redo,
  `@` jump to working copy, `w` workspaces, `g` (in a GitHub-focused context) forge pane.
- **L3 (power, docs only):** revset alias editing, `jj arrange` launch, bisect form,
  batch/multi-select operators.

Adding any of these still touches the same four places the project's own CLAUDE.md
already documents: `handleKeyPress()`, the handler, the help overlay, and the README
keybindings table.

## Open questions to verify before implementation

- `jj arrange`'s actual per-row action keys and whether it covers squash/fixup, not just
  reordering, confirmed hands-on rather than from docs alone.
- Whether Forgejo imposes any restriction on custom ref namespace pushes (only GitHub and
  GitLab were substantiated).
- Real `.jj/repo/op_store` size on an active repo, to size the pruning/retention window.
- Whether jjui's or lazyjj's user bases would be worth targeting for early feedback once
  a GitHub pane exists, given both are more mature log-browsing tools than jj-diff is
  today.

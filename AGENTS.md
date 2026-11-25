# AI Agent Guidelines for jj-diff

How to work in this Go project. Task and release mechanics live in
[CONTRIBUTING.md](CONTRIBUTING.md), worked code examples live in
[docs/go-best-practices.md](docs/go-best-practices.md), and toolchain failures live
in [docs/troubleshooting.md](docs/troubleshooting.md). Architecture and domain
context, where the project keeps a DESIGN.md, live there. This file covers only
what those do not.

## Verify before you report

CI runs five jobs and `mise run ci` is only the first, so a green `mise run ci` is not
a green build. Reproduce all five locally:

```bash
hk check --all                            # every hook step; also re-runs `mise run ci`
mise exec -- actionlint                   # separate CI job; the validator for workflow YAML
mise exec -- golangci-lint run --allow-serial-runners ./...  # separate CI job, not an hk step
mise exec -- golangci-lint config verify  # `run` accepts v1 schema keys silently
mise run bench                            # compiles and runs the benchmarks
```

Run linters to completion rather than stopping at the first hit: add
`--max-issues-per-linter=0 --max-same-issues=0`. A local run that passes while CI fails
means the two commands differ, so read the workflow step and match its flags.

Known false negatives, each of which has already cost a session:

- **Hooks look uninstalled when they are not.** `hk install --mise` on git 2.55+
  writes `hook.hk-*.command` into `.git/config` and creates no `.git/hooks/pre-commit`.
  Check with `git config --get-regexp '^hook\.'` (expect six entries) or
  `git hook list pre-commit` (expect `hk-pre-commit`). A file-existence check is wrong.
- **Never pipe or chain `git commit`.** That means `|`, `;`, `&&`, and `||`.
  `git commit … | tail` reports the pipe's exit code, and `;` runs the next command
  after a rejected commit, so a failing hook looks like success and the follow-up push
  ships nothing.
  Commit as its own command, then confirm with `git log -1`. Hooks rewrite files here;
  when one fails that way, `git add -A` its edits and commit again.
- **A workflow that passes yamllint can still be broken.** `hk.pkl` excludes
  `.github/workflows/` from yamllint, and yamllint reads YAML syntax only. Runner
  labels, expression syntax, `workflow_call` inputs, and embedded shell are
  `actionlint`'s job, so edit a workflow and run `mise exec -- actionlint`.
- **A release is verified by distinct hashes, not asset count.** Ten assets can be one
  binary published ten times. Confirm with
  `gh release download <tag> -p checksums.txt -O - | awk '{print $1}' | sort -u | wc -l`
  and expect the same number as there are binaries. `gh release download` resolves
  the release over GraphQL, so where only REST is reachable fetch the asset by id:
  `gh api -H 'Accept: application/octet-stream' "repos/{owner}/{repo}/releases/assets/$(gh api repos/{owner}/{repo}/releases/tags/<tag> --jq '.assets[] | select(.name == "checksums.txt") | .id')"`.
- **`hk check --all` aborts without mise on PATH.** The hk steps shell out to mise's
  tools, so a plain shell (no `mise activate`) dies on `ls-lint: command not found`
  and every later step reads as aborted. Run it as
  `mise exec -- hk check --all`.

When a check fails, fix the cause. Do not skip a test, widen a timeout, or disable a
linter to get to green. Three fix-and-push rounds with no new root cause means stop and
report what you found.


## Releases race your push

`bump_version.yml` runs on every push to `main` whose head commit is not itself a
`bump:` commit, and a release-worthy commit (`feat`, `fix`, `perf`) lands a
`bump: version` commit and tag within minutes of your push landing. That means
`main@origin` moves out from under work you have not pushed yet, so `jj git push`
regularly refuses with "references unexpectedly moved". The cycle is
`jj git fetch` → `jj rebase -b <change> -d main@origin` → `jj workspace update-stale`
→ `jj bookmark set main -r @` → `jj git push`, and the simplest prevention is to
fetch and rebase before pushing rather than after the refusal. Do not throttle or
gate the workflow to avoid the cycle: the release cadence is every release-worthy
commit, by design.

Two jj-vs-git traps that recur here: `jj git push` does not run git hooks, so an
`hk` pre-push check never fires for it; and a `blob:none` clone leaves jj's backend
unable to read promisor objects (`Object <id> of type file not found` on checkout),
fixed by `git config --unset remote.origin.partialclonefilter` plus a
`git fetch --refetch`.


## Layout

The entry point lives in `cmd/jj-diff/` and stays thin, delegating to
`internal/`. The compiler blocks imports of `internal/` from outside this module,
so anything under it can change freely.

```
jj-diff/
├── cmd/jj-diff/          # main package, kept thin
├── internal/
│   ├── model/            # core TEA model (orchestrator)
│   ├── jj/               # jj CLI integration
│   ├── diff/             # parser and patch generator
│   ├── search/           # search functionality
│   ├── fuzzy/            # fuzzy matching
│   ├── components/       # UI components
│   ├── config/           # configuration
│   └── theme/            # Catppuccin themes
├── tests/integration/    # end-to-end tests
└── go.mod
```

One package, one purpose. Short lowercase names, no underscores (`httputil`, not
`http_util`), and no grab-bags (`util`, `common`, `misc`). Name a file after the
primary type it holds (`user.go`, `user_test.go`).

## Go conventions

- Define interfaces where they are consumed, not where implemented; keep them to 1-3
  methods and add one only once a consumer needs it
- Take `context.Context` as the first argument for cancellable or I/O-bound work
- MixedCaps naming with uppercase acronyms (`ServeHTTP`, `userID`, `GetHTTPClient`)
- Functional options for constructors with optional configuration (`WithTimeout(d)`)
- Return errors instead of panicking outside truly unrecoverable states, and wrap with
  context: `fmt.Errorf("loading config: %w", err)`
- Inspect with `errors.Is` / `errors.As`; define types for domain-specific errors
- Validate at boundaries and trust internal code (parse, don't validate)
- Doc-comment exported symbols, starting with the symbol name, describing non-obvious
  behavior and invariants rather than restating the types

Avoid: naked returns, functions past ~50 lines, deep nesting (return early), ignored
errors (`_ = doThing()` is almost always wrong), and shared global state (pass
dependencies explicitly).

[docs/go-best-practices.md](docs/go-best-practices.md) has a worked example of each
pattern above, plus the package layout rules and the anti-pattern list in full.

## Testing

Table-driven tests with subtests via `t.Run`, placed next to the code they cover, in a
`_test` package for black-box coverage. `mise run test:coverage-min` enforces the 70%
floor.

Golden fixtures are byte-exact: regenerate with `go test ./... -update` and review the
diff, never hand-edit. `hk.pkl` excludes `**/*.golden` from the whitespace fixers, so a
fixture under any other name (`testdata/golden_*.txt`) will be silently rewritten on
commit; either rename it or add its glob to that exclude.

A test that runs `git`, `jj`, `gh`, or the built binary inherits the runner's home and
writes to the real account: jj 0.47 records every repository it opens under
`$XDG_CONFIG_HOME/jj/repos`, `git config --global` lands in `~/.gitconfig`, and the
tool's own state goes to the user config directory. Give each test binary a home of
its own in `TestMain`, before any test runs: point `HOME`, each `XDG_*_HOME`, `TMPDIR`,
`GIT_CONFIG_GLOBAL` (with `GIT_CONFIG_NOSYSTEM=1`), `JJ_CONFIG`, and `GH_CONFIG_DIR` at
a temp root, and leave `GOCACHE` and `GOPATH` on the real ones so builds stay warm.
Where `github.com/kyleking/aragonite` is already a dependency, its `hermetic` package
does this and fails the run when the real home changed anyway:
`func TestMain(m *testing.M) { hermetic.Main(m, hermetic.App("jj-diff")) }` in
each package, plus one test calling `hermetic.RequireEveryPackage` so a new package
cannot skip it. Point a fixture repository's remote at an address that answers
nothing (`https://127.0.0.1:1/<owner>/<repo>.git`), so a new network call fails at
once rather than passing on a laptop with credentials and hanging in CI.

A test run from a git hook inherits the hook's repository: git sets `GIT_DIR` (and in a
linked work tree `GIT_WORK_TREE`, `GIT_INDEX_FILE`, and the rest of
`git rev-parse --local-env-vars`) before it runs pre-commit or pre-push, and hk runs
`go test` inside that hook. Every fixture `git init`, `git add`, or `git commit` in a
temp directory then writes into the repository being pushed: it stages stale files into
the real index and sets `core.worktree` in `.git/config`. Unset every variable
`git rev-parse --local-env-vars` prints, plus `GIT_NAMESPACE`, in `TestMain` before any
test runs. aragonite `hermetic.Main` (v1.31.1 or later) does this and also fails the run
when the real repository's config, HEAD, refs, or staged entries changed. To check by
hand, `GIT_DIR=$PWD/.git go test ./...` must pass and leave `git status` and
`git config --get core.worktree` as they were.

Write a stub executable from a child process
(`sh -c 'cat > "$1" && chmod 700 "$1"' sh <path>` with the script on stdin), never
`os.WriteFile` with an exec bit. A parallel test's child forked while the file is open
for writing holds the descriptor until it execs, and running the stub meanwhile fails
with "text file busy" (golang/go#22315).

## TUI testing

A subprocess pipe is not a terminal, so the program detects a non-tty and never renders.
Exploratory checks need a real PTY: run it under `tmux new -d -s tui-check -x 80 -y 24
<cmd>`, drive it with `tmux send-keys`, and read what actually rendered with
`tmux capture-pane -p` (`-e` keeps ANSI codes).

Kill the session with `tmux kill-session -t tui-check` once the screen has been read.
A detached session outlives the check that started it, and a TUI whose terminal goes
away can spin rather than exit: 19 orphaned processes from one such burst held 40-49%
CPU each for as long as 10 hours. Name the session, because an anonymous `-d` leaves
nothing to identify a stray one by later.

A tab in captured output is not proof the program emitted one:
`tmux capture-pane` re-emits a tab wherever the cursor was advanced across cells that
were never written. Check which column the text actually lands on before chasing it as
a rendering bug. For scripted tests prefer
`github.com/charmbracelet/x/exp/teatest`, which drives the `tea.Model` directly and
diffs golden frames; on Bubble Tea v2 that import is
`github.com/charmbracelet/x/exp/teatest/v2`, because the unsuffixed module targets v1.
Fall back to `github.com/creack/pty`, or `github.com/Netflix/go-expect` for
expect-style send/wait-for-pattern interaction, on non-Bubble Tea binaries.

Judging whether it looks right means checking wrapping and truncation at the target
width, no overlapping or stale content between re-renders, cursor and alt-screen state
restored on every quit path, and no ANSI escapes bleeding into piped output.

Exercise deliberately, because each of these renders fine in the happy path and breaks
elsewhere: resize mid-session (`SIGWINCH`) and the minimum supported size; every quit
path independently (`q`, `ctrl-c`, `esc`); full keyboard navigation including tab focus
order and wraparound at list boundaries; empty and single-item states; rapid repeated
keypresses against a debounced action; multi-byte and unicode paste; and piped non-tty
stdout degrading instead of hanging.

tmux hides the first two of these, so pin them in a pty test with
`github.com/charmbracelet/x/vt` as the screen; the last two are that harness's own traps:

- **Color depth.** Inside tmux, `colorprofile.Detect` ignores `COLORTERM` and asks
  `tmux info`, which a detached server answers without RGB, so a capture reads as 256
  colors. Detect the profile honoring `COLORTERM=truecolor` (aragonite `theme.Profile`)
  and hand that same profile to `tea.WithColorProfile`, so styles and renderer agree.
- **Resize.** Bubble Tea restores scroll margins (DECSTBM) by number, so a frame
  flushed just before a resize leaves the old height as the bottom margin and a grown
  window scrolls on every repaint. tmux resets margins on resize itself. On each
  `tea.WindowSizeMsg`, also send
  `tea.Raw(ansi.SaveCursor + ansi.SetTopBottomMargins(0, 0) + ansi.RestoreCursor)`
  followed by `tea.ClearScreen`. A resize test replaces its `vt` emulator rather than
  resizing it, because the library's resize keeps the old scroll region.
- **Partial redraws.** A redraw writes only the cells that changed, so `1 of 11`
  becoming `1 of 15` puts `5` in the stream. Assert on the emulator's frame, not on
  the bytes that drew it.
- **Capability queries.** A bare pty answers none, so a program that asks the terminal
  (background color over OSC 11, DA1) waits out a timeout on each before its first
  frame. A pty harness answers them, or every test pays seconds of stall.

Hold Bubble Tea at v2.1.2 or later: v2.1.1 can leave the screen blank for good after
handing the terminal to another program (`tea.ExecProcess`) under load. A golden frame
is byte-exact and still blind to a row wider than the frame, a box edge that joins
nothing, and a glyph terminals measure differently; aragonite `tui/framecheck` checks
those, so run it on a frame before comparing.

Turn every bug found this way into a test named for its trigger
(`TestQuit_RestoresCursorOnCtrlC`), parametrized over terminal size rather than
hardcoding one.

## Dependency updates

The weekly Freshness workflow runs doneram and opens draft PRs on `doneram/*` branches.
Two skills in `.agents/skills` finish what it cannot:

- `doneram-fix-bump` on a `doneram/failing-<tool>` or `doneram/review-<tool>` branch:
  `doneram explain <tool> --validate` gives the change, where it is pinned, the
  breaking notes, and the `mise run ci` output; adapt the code until that passes.
- `copier-update-conflicts` when the updates PR carries a template update: resolve the
  `.rej` files under the rules below.

## Template-managed files

This project is generated from
[my_go_template](https://github.com/KyleKing/my_go_template). Edit the template, not the
render, for anything under `.github/workflows/`, `.golangci.toml`, `hk.pkl`,
`.goreleaser.yml`, or `.config/mise/conf.d/template.toml` — a `copier update` overwrites
local edits there. Project-specific mise tasks belong in a sibling conf.d file
(CONTRIBUTING.md explains the load order that makes the filename matter).

After `copier update --UNSAFE --conflict=rej --defaults`, re-apply real local content
from each `.rej`, discard hunks the new template supersedes, and delete the files (a
hook blocks committing them). Three specifics:

- `.cz.toml` keeps its version: the update's diff carries it, and a post-update
  migration reads it back from `HEAD` if a conflict left the `0.0.0` placeholder.
  Check `grep ^version .cz.toml` anyway, since a `0.0.0` cuts `v0.0.1` on the next release.
- Scaffolding the project then implements (the `cmd/` entry point, the `bindings/` cgo
  shim, its Python wrapper) conflicts on every update, because the template keeps
  rendering its starting version. Re-apply from the `.rej` and move on
- **The same file conflicting on two consecutive updates means an answer is wrong, not
  that the patch needs re-applying.** Read `.copier-answers.yml` first and fix the
  answer; a typo there gets faithfully re-rendered every pass.

Resolve conflicts rather than avoiding them. Adding a file to `_skip_if_exists` looks
like it removes the friction, and instead it makes the file stop receiving every later
template fix, silently and forever. A conflict is loud and usually a few lines to
re-apply, whereas a skipped file drifts without ever saying so. `_skip_if_exists` is
for files the template has nothing further to say about after the first render, which
is why it holds `README.md`, `DESIGN.md`, `go.mod`, and `.config/mise.toml` and nothing
that carries template-maintained content.

Deleting a `_skip_if_exists` file brings it back on the next update, because copier
only skips a file that is already there. To be rid of one, empty it instead.

Resolving a conflict is half the work. The other half is deciding which side was
better and backporting it. A `.rej` is a diff between what the template renders and
what this project actually needed, so it names the gap directly:

- The template's hunk is better, so take it and delete the local version
- The local hunk is better and generalizes, so take it locally AND open the change
  against [my_go_template](https://github.com/KyleKing/my_go_template) so the next
  render starts from it
- The local hunk is better and is specific to this project, so take it locally and
  leave the template alone

The second case is the one to watch for, because skipping it means re-resolving the
same conflict on every future update. If a file clobbers the same way twice, the
template is rendering the wrong thing and the fix belongs upstream.

This file is template-owned and `copier update` keeps it current. Put project-specific
guidance in `AGENTS.local.md` (loaded below when present) or in a nested `AGENTS.md`
scoped to its directory.

@AGENTS.local.md

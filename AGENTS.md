# AI Agent Guidelines for jj-diff

How to work in this Go project. Architecture and domain context live in
[DESIGN.md](DESIGN.md); task and release mechanics live in
[CONTRIBUTING.md](CONTRIBUTING.md). This file covers only what those two do not.

## Verify before you report

CI runs four jobs and `mise run ci` is only the first, so a green `mise run ci` is not
a green build. Reproduce all four locally:

```bash
hk check --all                            # every hook step; also re-runs `mise run ci`
mise exec -- golangci-lint run ./...      # separate CI job, not an hk step
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
- **Never pipe or chain `git commit`.** `git commit … | tail` reports the pipe's exit
  code, so a failing hook looks like success and the follow-up push ships nothing.
  Commit as its own command, then confirm with `git log -1`. Hooks rewrite files here;
  when one fails that way, `git add -A` its edits and commit again.
- **A release is verified by distinct hashes, not asset count.** Ten assets can be one
  binary published ten times. Confirm with
  `gh release download <tag> -p checksums.txt -O - | awk '{print $1}' | sort -u | wc -l`
  and expect the same number as there are binaries.

When a check fails, fix the cause. Do not skip a test, widen a timeout, or disable a
linter to get to green. Three fix-and-push rounds with no new root cause means stop and
report what you found.

## Layout

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
primary type it holds (`user.go`, `user_test.go`). `internal/` blocks outside imports at
the compiler level.

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

## Testing

Table-driven tests with subtests via `t.Run`, placed next to the code they cover, in a
`_test` package for black-box coverage. `mise run test:coverage-min` enforces the 70%
floor.

Golden fixtures are byte-exact: regenerate with `go test ./... -update` and review the
diff, never hand-edit. `hk.pkl` excludes `**/*.golden` from the whitespace fixers, so a
fixture under any other name (`testdata/golden_*.txt`) will be silently rewritten on
commit; either rename it or add its glob to that exclude.

## TUI testing

A subprocess pipe is not a terminal, so the program detects a non-tty and never renders.
Exploratory checks need a real PTY: run it under `tmux new -d -x 80 -y 24 <cmd>`, drive
it with `tmux send-keys`, and read what actually rendered with `tmux capture-pane -p`
(`-e` keeps ANSI codes). This is the fastest way to answer "does it look right" without
writing test code first. Note that `tmux capture-pane` collapses a run of spaces into a
tab, so a tab in captured output is an artifact rather than a rendering bug; check the
column the text lands on before chasing one. For scripted tests prefer
`github.com/charmbracelet/x/exp/teatest/v2`, which drives the `tea.Model` directly and
diffs golden frames. The unsuffixed `teatest` targets Bubble Tea v1 and will not compile
against this project. Fall back to `github.com/creack/pty`, or
`github.com/Netflix/go-expect` for expect-style send/wait-for-pattern interaction, on
non-Bubble Tea binaries.

Judging whether it looks right means checking wrapping and truncation at the target
width, no overlapping or stale content between re-renders, cursor and alt-screen state
restored on every quit path, and no ANSI escapes bleeding into piped output.

Exercise deliberately, because each of these renders fine in the happy path and breaks
elsewhere: resize mid-session (`SIGWINCH`) and the minimum supported size; every quit
path independently (`q`, `ctrl-c`, `esc`); full keyboard navigation including tab focus
order and wraparound at list boundaries; empty and single-item states; rapid repeated
keypresses against a debounced action; multi-byte and unicode paste; and piped non-tty
stdout degrading instead of hanging.

Turn every bug found this way into a test named for its trigger
(`TestQuit_RestoresCursorOnCtrlC`), parametrized over terminal size rather than
hardcoding one.

## Template-managed files

This project is generated from
[my_go_template](https://github.com/KyleKing/my_go_template). Edit the template, not the
render, for anything under `.github/workflows/`, `.golangci.toml`, `hk.pkl`,
`.goreleaser.yml`, or `.config/mise/conf.d/template.toml` — a `copier update` overwrites
local edits there. Project-specific mise tasks belong in a sibling conf.d file
(CONTRIBUTING.md explains the load order that makes the filename matter).

After `copier update --UNSAFE --conflict=rej --defaults`, re-apply real local content
from each `.rej`, discard hunks the new template supersedes, and delete the files (a
hook blocks committing them). Two specifics:

- `.cz.toml` is re-rendered with `version = "0.0.0"`. Restore the real version by hand
  or the next release cuts `v0.0.1`.
- **The same file conflicting on two consecutive updates means an answer is wrong, not
  that the patch needs re-applying.** Read `.copier-answers.yml` first and fix the
  answer; a typo there gets faithfully re-rendered every pass.

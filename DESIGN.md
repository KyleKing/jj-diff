# Design

<!-- Project-specific architecture, design decisions, and domain context.
     This file is preserved across template updates (_skip_if_exists). -->

## Architecture

jj-diff is a TUI application built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) following The Elm Architecture pattern.

### Project Structure

```
jj-diff/
├── cmd/jj-diff/          # CLI entry point
├── internal/
│   ├── model/            # Core application state (TEA)
│   ├── jj/               # jj CLI integration
│   ├── diff/             # Diff parsing and patch generation
│   ├── conflict/         # Snapshot-style conflict markers: parse, pick, write back
│   ├── reads/            # jojo's read marks, shown read-only on a revision
│   ├── search/           # Search functionality
│   ├── fuzzy/            # Fuzzy matching
│   ├── components/       # UI components (filelist, diffview, modals)
│   └── theme/            # Catppuccin themes
├── render/               # Public: unified diff to styled lines
└── tests/integration/    # End-to-end tests
```

### Key Components

**Model** (`internal/model/model.go`)

- Central orchestrator following The Elm Architecture
- Message-driven state updates
- Coordinates UI components and jj client

**JJ Client** (`internal/jj/client.go`)

- Abstraction for jj command execution
- MoveChanges, Split, Restore: one `jj squash -i`, `jj split -i`, or `jj restore -i` with
  jj-diff itself as the diff editor (`--apply-patch` / `--unapply-patch`), which makes `$right`
  the `$left` tree plus the precomputed selection without drawing anything. jj does the move,
  so it works for any destination and is one operation
- A split chain's later steps see only the files that still differ, so a file an earlier group took
  whole is dropped from both of a step's patches when they agree on it
- Resolve: one `jj resolve --tool :editor` with jj-diff as `ui.editor` (`--resolve-with PLAN`),
  which swaps each conflicted file, recognized by the hash of the text it was read as, for the
  picks applied. The `:editor` tool takes any number of sides and keeps leftover markers as the
  conflict that remains, which a three-way merge tool does neither of
- Automatic rollback to the operation recorded before a write that fails after integrating

**Diff Subsystem** (`internal/diff/`)

- Parser: converts unified diff to structured data
- Patch Generator: creates patches from hunk and line selections
- Supports whole hunks and partial hunks: an unselected deletion becomes context and an
  unselected addition is dropped, and every header is recounted

**Components** (`internal/components/`)

- FileList: vertical table view with stats
- DiffView: unified or side-by-side rendering with syntax highlighting, where a generated file or a
  hunk that only reindents folds to one line that `z` expands
- Modals: search, destination picker, fuzzy finder, and the `gm` command log (`internal/model/messages.go`)

**Screens beyond the diff** (`internal/model/columns*.go`, `internal/model/resolve*.go`)

- Split columns: every changed line has one owner column (absent means column 1, the source).
  Applying turns columns 2..N into `Split` groups, a chain's patches cumulative
- Conflict viewer (`ModeResolve`): a pane per side and the base, picks held per file and region
  until one `Resolve` writes them

**Render** (`render/`)

The only package outside `internal/`, so other tools draw a diff the way this one does: the same
parser, the same word-level intra-line highlighting, and the same syntax highlighting, with the
palette passed in rather than read from this program's theme. It renders unified or side by side and
does not select, since hunk and line selection belong to the editor. The two-column pairing lives in
`internal/diff` so both renderers use the same one. `render.Lines` draws a whole diff, while `render.New`
parses once and draws any window of rows on demand, which is what a caller with a small pane over a large
diff wants. Syntax colors come from aragonite's `highlight`, which lexes each file once, in the background for the editor.
`Rows` lexes a file on its first draw and blocks, so a screen draws with `Document.Peek`, which never lexes
and draws a not-yet-lexed line plain, and calls `Document.Start` to lex in the background, redrawing once
its channel closes (2000 rows: 190 ms blocking, against 9 ms plain and 7 ms colored after). [jojo](https://github.com/KyleKing/jj-tui) draws its
diff pane with it.

**Discoverability** (`internal/model/gate.go`, `discover.go`, `coach.go`, `empty.go`, `messages.go`)

Adopted from aragonite's discoverability layer (its `docs/discoverability.md`), the same one
second-look and jojo use:

- One gate, `Model.gate(action) keys.Availability`, answers whether an action can do anything on
  the screen the next key lands on, and why not, from the same state the handlers read
  (`hasCurrentHunk`, the selection, the columns' owners, the picks). The footer draws a refused key
  dim (`keymap.HintsAt`), the palette and legend list it under "off here" with the reason, and
  pressing it says the reason in the footer instead of doing nothing (`Model.press`)
- One `palette.Model` serves `?` (Browse: a key press explains the key, `/` filters, `enter` runs)
  and `:` (Run), over `palette.FromKeymap` with an `About` map that names the jj command each action
  runs, with this session's revisions in it, and the git words that find it. A run dispatches by
  action name, never by replaying a key. It replaced `internal/components/help`. It draws as a drawer
  with an explainer from 100 columns and a strip below, laid over every row but the footer
- The coach (`aragonite/tui/coach`) reads a small `coachState` per frame and names the next step per
  screen in the footer's status slot, after the mode and revisions where it fits whole, alone where
  only that fits, and not at all otherwise; the band layout is opt-in (`JJ_DIFF_COACH_LAYOUT=band`,
  `:next`). Detectors watch for a long `j` run (`]h`), a tab-j-tab round trip (`]f`), and palette
  runs of a bound action. Its ledger lives at `$XDG_STATE_HOME/jj-diff/coach.json`, opened in
  `main` and saved on quit and at each idle tip check; tests build models without one
- `jj.Client` logs every command through an `aragonite/trail.Trail` (reads and writes, stderr
  kept), the write notice is `trail.Echo` over the call with the editor plumbing
  (`--tool jj-diff-selection`, `--config`) left out by `jj.Echoed`, and `gm` lists the trail with
  the exact argv, which `Y` copies as a shell line
- Empty screens (no changes, nothing conflicted, nothing to split, an empty column while it is
  described) draw `empty.Card`/`empty.Block` naming the revision, why, and the keys out
- The opt-in usage log (`keyusage.Open("jj-diff")`) records each command key, the action it ran,
  and refused presses (`Recorder.Gated`)

**Read marks** (`internal/reads`, `internal/model/reads.go`)

jojo marks hunks read and jj-diff shows them, read-only, on a revision in browse or interactive
mode. The two must name one directory and one id per hunk, so nothing is jj-diff's own:

- The directory is `seen.StateDir("jojo", "seen", seen.RepoKey(repo))`, where `repo` is
  aragonite's `seen.JJRepo(cwd)`: the nearest `.jj` above the working directory, followed through
  a secondary workspace's pointer to the main store. jojo keys by the same store from its
  workspace root, and `TestOpen_AgreesWithJojo` writes jojo's derivation out longhand and holds
  `reads.Open` to it from a root, a subdirectory, and a second workspace
- The scope is the revision's change id as jojo writes it everywhere, `change_id.shortest(8)`
  (`Client.ScopeID`, with `--ignore-working-copy` since it runs beside the diff's own jj call); the
  full id would name a file jojo never writes. A hunk's id is `seen.Parse` over the same
  `jj diff --git` text jj-diff parses. `reads.Map` pairs those ids with jj-diff's hunks by path
  and order, and leaves a file whose hunk count the two parsers disagree on unmarked rather than
  marked wrong
- The change id and the diff load as two commands, so the first frame waits on neither; marks
  appear once both have arrived. Every message runs one `stat` of the scope's file, and a changed
  modification time or size rereads it (`Marks.Refresh`), with focus reporting turned on so a
  terminal regaining focus is a message. Nothing in jj-diff writes a mark, which is why `␣` can
  keep meaning select
- The diff editor and the conflict viewer have no change to scope marks by and show none; their
  keymaps bind no `]u`. Where marks show, `]u` / `[u` are also bound on the file list, the screen
  a session opens on

### Design Principles

1. **Functional style**: small, composable functions with single responsibility
2. **The Elm Architecture**: immutable updates, message-driven, pure rendering
3. **Component composition**: each component manages its own rendering
4. **Interface-based selection**: allows patch generation without a full Model dependency
5. **Catppuccin theme, shared skin**: minimal color usage, color for accents. The faces come from
   aragonite's `tui/skin` built over the same flavor, so jj-diff, second-look, and jojo draw a
   changed line (sign bar, full-width wash, deeper word mark), a section heading (labeled rule), a
   file heading (lifted band), the overview track, and the footer's key hints the same way

## Patterns

### Component pattern

```go
type Model struct {
    // Component state
}

func New() Model { /* initialize */ }
func (m *Model) SetX(x Type) { /* update state */ }
func (m Model) View(width, height int) string { /* render */ }
```

### Error handling

- Propagate errors with context: `fmt.Errorf("operation failed: %w", err)`
- Use specific error types where appropriate
- Automatic rollback in jj operations

### Adding a component

1. Create a package in `internal/components/`
2. Implement `New()`, `View()`, and state methods
3. Add a field to the Model struct in `internal/model/model.go`
4. Initialize it in `NewModel()`
5. Call `View()` from `Model.View()`
6. Add keybindings in `handleKeyPress()`

### Adding a mode

1. Add the mode constant in `internal/model/model.go`
2. Add mode-specific keybindings in `handleKeyPress()`
3. Update `View()` to render the mode indicator
4. Implement the mode transition logic

### Extending search

1. Extend `MatchLocation` in `internal/search/search.go`
2. Update the `ExecuteSearch()` algorithm
3. Add navigation methods if needed
4. Update the searchmodal component

## Testing

### Test layers

Unit tests cover diff parsing (`internal/diff/parser_test.go`), patch generation (`internal/diff/patch_test.go`), search (`internal/search/search_test.go`), and fuzzy matching (`internal/fuzzy/fuzzy_test.go`).

Model tests cover UI state management, selection state, navigation, and mode transitions in `internal/model/model_test.go`.

Integration tests in `tests/integration/client_test.go` drive real jj workflows and cover MoveChanges, rollback, and working copy preservation.

### Coverage goals

| Component | Target | Last measured |
|-----------|--------|---------------|
| Diff parser | >90% | 92.5% |
| Search | >90% | 96.6% |
| Model | >40% | 41.2% (acceptable for a TUI) |
| Integration | Critical paths | 100% |

### Manual testing

Two interactive scripts drive a real repository:

```bash
./scripts/test-in-tmpdir.sh      # Create a test repo and launch jj-diff
./scripts/interactive-test.sh    # Menu of scenarios
```

To build a scratch repo by hand:

```bash
cd /tmp
jj init --git test-repo
cd test-repo
echo "line 1" > file.txt
jj commit -m "initial"
echo "line 2" >> file.txt
jj commit -m "changes"
/path/to/jj-diff --interactive
```

Force a theme in tests with `t.Setenv("CATPPUCCIN_THEME", "macchiato")` followed by `theme.Init()`. Lip Gloss v2 has no global background override, so the environment variable is the only seam.

## Running locally

```bash
./jj-diff                    # Browse mode (read-only)
./jj-diff --interactive      # Move changes between revisions
./jj-diff $left $right       # Diff-editor mode (jj split, diffedit)
```

Prerequisites beyond the template's: jj 0.9.0+ on PATH, and git, which `git apply` needs during patch application.

## Performance

Measured in a Linux container on 4 vCPUs of an Intel Xeon at 2.10 GHz with Go 1.26.0
(`go test -bench=. -benchmem`), so absolute numbers run slower than a laptop. "Before" is v0.8.0,
the release before this table was rewritten; both columns come from the same container.

| What | Before | After |
|------|--------|-------|
| Diff pane frame, 40 rows (`DiffViewRender_*`) | 2.7 ms, 7,400 allocs | 16–33 µs, 7–13 allocs |
| Diff pane scrolled one row, 2000-row file (`DiffViewScroll_2000Rows`) | 2.2 ms | 11 µs |
| Diff pane frame where only the line cursor moved (`DiffViewCursorMove_2000Rows`) | — | 10 µs |
| Open a 2000-row file to its first frame (`DiffViewFirstFrame_2000Rows`) | 263 ms | 5.9 ms |
| Whole screen, diff focused (`ModelRender_DiffFocused`) | 1.85 ms | 27 µs |
| Whole screen while searching a common term (`ModelRender_Searching`) | 2.2 ms | 43 µs |
| `j` and the frame it draws (`ModelKeyJ`) | 1.87 ms | 43 µs |
| Expanded file list, 1000 files, cursor moving (`FileListRender_Expanded`) | 514 µs | 40 µs |
| Diff loaded to first frame, 200 files (`ModelStartup`) | 288 ms | 8.3 ms |
| Launch to the diff on screen, 51-file change under tmux | ~440 ms | ~150 ms |
| Search, 10,000 lines (`Search_MediumDiff`) | 5.7 ms | 4.3 ms |

What moved the numbers, so a change can tell whether it undoes one:

- **Syntax colors land after the first frame.** Lexing is chroma's regexp engine and costs about 75 ms
  per side of a 2000-line file. `SetFileChange` starts it in the background (aragonite's
  `highlight.Cache.Start`), rows draw plain until it lands, and `HighlightCmd` redraws once it has.
  The colored frame (`DiffViewColoredFrame_2000Rows`, about 170 ms) is the lexer's cost, off the
  critical path. Scrolling never re-lexes: the spans are cached per file.
- **Rows draw through pens, not `Style.Render`.** A lipgloss render costs about 3.4 µs, and a row is a
  dozen styled runs. aragonite's `tui/pen` learns a style's escape sequence once and writes runs
  byte for byte as `Render` would. The pens are rebuilt only when `theme.Generation` changes.
- **Drawn rows are kept.** The diff pane keeps rows keyed by line, width, and cursor state, the
  hunk headers and the overview track likewise, the file list keeps its last frame and its rows,
  and the footer its last line. A frame where the cursor moved redraws the rows it moved between.
  Rows with search hits are drawn fresh, and a row drawn before its colors landed is not kept.
- **Search hits are indexed once per search,** by line and by file, rather than scanned per row.
- **Startup overlaps its jj calls.** `diff.Prefetch` starts `jj diff` before theme detection, which
  waits on the terminal and on `tmux info`; the label resolves beside it with
  `--ignore-working-copy`, off the working-copy lock; and `CheckInstalled` looks jj up on `PATH`
  rather than running it.

The tmux figure is from `startup.sh`-style polling of `tmux capture-pane` until a line of the first
file's diff appears, on a synthetic repository of 300 files and 31 commits whose working copy
changes 51 files (one with 2,000 changed lines). `jj diff` alone takes 45–75 ms of it there.

To profile:

```bash
go test -cpuprofile=cpu.prof -bench=. ./internal/components/diffview/
go tool pprof cpu.prof

go test -memprofile=mem.prof -bench=. ./internal/components/diffview/
go tool pprof mem.prof
```

## Resources

- [Bubble Tea documentation](https://github.com/charmbracelet/bubbletea)
- [Lip Gloss styling](https://github.com/charmbracelet/lipgloss)
- [jj documentation](https://martinvonz.github.io/jj/)
- [Catppuccin theme](https://github.com/catppuccin/catppuccin)

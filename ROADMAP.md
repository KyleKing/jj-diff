# jj-diff Roadmap

Feature plans that build on jj's unique capabilities. None of the six below is
built as of 2026-07-27: `internal/components/` holds no timeline, oplog, or
smartlog component. Known defects and design decisions live in `FINDINGS.md`; the
fixture-testing plan lives in `doctest-jj-diff.md`.

Section 4 quotes jj's conflict-marker format verbatim, so the `<<<<<<<` and
`>>>>>>>` lines in this file are content, not an unresolved merge. `hk.pkl`
excludes this path from `check-merge-conflict` for that reason; keep the markers
and the exclude in step if the file is ever renamed.

---

## 1. Evolution Timeline

**Keybinding**: `E`

**Problem**: Users cannot see how a change evolved through rebases, amends, and squashes. This is jj's core differentiator from git.

**Implementation**:

- New component: `internal/components/evolutiontimeline/`
- Data source: `jj obslog -r <rev> --no-graph -T 'commit_id ++ "\t" ++ description.first_line() ++ "\n"'`
- Parse obslog output into timeline entries with operation type, timestamp, commit ID
- Render as vertical list with operation annotations (rebase, amend, squash, etc.)
- Support diffing between any two evolutions via selection

**UI Layout**:
```
Evolution of change abc123
─────────────────────────────
  [3] 2h ago   rebase onto main
  [2] 5h ago   amend: fix typo        ← cursor
  [1] 1d ago   initial commit
─────────────────────────────
[Enter] view diff  [d] interdiff  [r] restore  [q] close
```

**Key Commands**:
- `jj obslog -r <rev>` - list evolutions
- `jj diff --from <old> --to <new>` - interdiff between evolutions
- `jj op restore <op-id>` - restore to previous evolution

**Files to Modify**:
- `internal/model/model.go`: Add `ModeEvolutionTimeline`, handle `E` key
- `internal/jj/client.go`: Add `GetObslog(revision)` method
- New: `internal/components/evolutiontimeline/evolutiontimeline.go`

---

## 2. Interdiff Support

**Keybinding**: `I` (when in evolution timeline)

**Problem**: When iterating on a change, users need to see what changed between v1 and v2, not the full diff each time.

**Implementation**:

- Extend evolution timeline with selection of two versions
- Compute interdiff via `jj diff --from <v1> --to <v2>`
- Reuse existing diff parser and view components
- Show interdiff in main diff view with clear header

**Workflow**:
1. Enter evolution timeline (`E`)
2. Select first version (Space to mark)
3. Navigate to second version
4. Press `I` to view interdiff
5. Diff view shows what changed between the two evolutions

**Files to Modify**:
- `internal/components/evolutiontimeline/`: Add dual-selection state
- `internal/diff/source.go`: Add `InterdiffSource` for two-revision comparison
- `internal/model/model.go`: Handle interdiff display mode

---

## 3. Operation Log View

**Keybinding**: `O`

**Problem**: Users need visibility into jj operations for undo/redo and understanding repo state.

**Implementation**:

- New component: `internal/components/oplog/`
- Data source: `jj op log --no-graph -T 'operation_id ++ "\t" ++ description ++ "\t" ++ time ++ "\n"'`
- Render as navigable list with operation descriptions
- Support restore to any operation

**UI Layout**:
```
Operation Log
─────────────────────────────────────────
  abc123  2m ago   commit: fix auth bug
  def456  5m ago   new: working copy      ← cursor
  ghi789  1h ago   squash into xyz
─────────────────────────────────────────
[r] restore  [u] undo last  [q] close
```

**Quick Undo/Redo**:
- `u` anywhere: Execute `jj undo` and refresh view
- `Ctrl+r` anywhere: Execute `jj op restore <prev>` (redo pattern)

**Key Commands**:
- `jj op log` - list operations
- `jj undo` - undo last operation
- `jj op restore <op-id>` - restore to specific operation

**Files to Modify**:
- `internal/model/model.go`: Add `ModeOpLog`, handle `O`/`u`/`Ctrl+r` keys
- `internal/jj/client.go`: Add `GetOpLog()`, `Undo()`, `RestoreOp(opId)` methods
- New: `internal/components/oplog/oplog.go`

---

## 4. Conflict Visualization

**Keybinding**: `C`

**Problem**: jj allows conflicts to exist in commits. Users need to see and navigate conflicts.

**Implementation**:

- Parse jj conflict markers in diff content
- Highlight conflict regions with distinct styling
- New conflict list view showing all conflicted files
- Navigation between conflicts with `]x`/`[x`

**Conflict Marker Format** (jj-specific):
```
<<<<<<< Conflict 1 of N
%%%%%%% Changes from base to side #1
[side 1 changes]
+++++++ Contents of side #2
[side 2 content]
>>>>>>> Conflict 1 of N ends
```

**UI Layout** (conflict list):
```
Conflicts (3 files, 7 total)
────────────────────────────
  src/auth.go         (2 conflicts)
  src/handler.go      (4 conflicts)  ← cursor
  tests/auth_test.go  (1 conflict)
────────────────────────────
[Enter] view  []x next  [[x prev  [q] close
```

**Key Commands**:
- `jj resolve --list` - list conflicted files (if available)
- Parse diff output for conflict markers

**Files to Modify**:
- `internal/diff/parser.go`: Detect and parse conflict markers
- `internal/diff/types.go`: Add `ConflictRegion` struct
- `internal/components/diffview/`: Highlight conflict regions
- `internal/model/model.go`: Add `]x`/`[x` navigation, `C` for conflict list
- New: `internal/components/conflictlist/conflictlist.go`

---

## 5. Background Refresh

**Problem**: View becomes stale when user runs jj commands externally.

**Implementation**:

- Watch `.jj/` directory for changes using fsnotify
- Debounce refresh (500ms) to avoid excessive reloads
- Send `RefreshMsg` to model on detected changes
- Preserve cursor position and selection state across refresh

**Architecture**:
```go
type Watcher struct {
    watcher *fsnotify.Watcher
    updates chan struct{}
}

func (w *Watcher) Start(jjDir string) tea.Cmd {
    // Watch .jj/repo/ for operation changes
    // Return tea.Cmd that sends RefreshMsg
}
```

**Files to Modify**:
- New: `internal/watcher/watcher.go`
- `internal/model/model.go`: Initialize watcher, handle `RefreshMsg`
- `cmd/jj-diff/main.go`: Pass repo path to watcher

**Dependencies**:
- `github.com/fsnotify/fsnotify`

---

## 6. Interactive Smartlog

**Keybinding**: `L`

**Problem**: Users need to see and interact with the change graph, similar to Sapling's smartlog.

**Implementation**:

- New component showing `jj log` output as interactive graph
- Navigate between changes with j/k
- Actions on selected change: view diff, edit, new, squash
- Filter/search changes

**UI Layout**:
```
Smartlog
────────────────────────────────────────
  @  abc123  (empty) working copy
  │
  ○  def456  feat: add auth
  │
  ○  ghi789  fix: handle errors       ← cursor
  │
  ◆  main    initial commit
────────────────────────────────────────
[Enter] view  [e] edit  [n] new  [s] squash  [q] close
```

**Key Commands**:
- `jj log --no-graph -T <template>` - structured output
- `jj log` - graph output for display
- `jj edit <change>` - switch to change
- `jj new <change>` - create child
- `jj squash --from <change>` - squash into parent

**Graph Parsing**:
- Parse `jj log` output preserving graph characters
- Extract change IDs, descriptions, bookmarks
- Map graph lines to selectable entries

**Files to Modify**:
- `internal/model/model.go`: Add `ModeSmartlog`, handle `L` key
- `internal/jj/client.go`: Add `GetLog()` method with template
- New: `internal/components/smartlog/smartlog.go`

---

## Implementation Priority

| Priority | Feature | Effort | Impact |
|----------|---------|--------|--------|
| P0 | Evolution Timeline | Medium | High (jj-unique) |
| P0 | Interdiff | Low | High (builds on timeline) |
| P1 | Operation Log | Low | Medium |
| P1 | Interactive Smartlog | Medium | High |
| P2 | Conflict Visualization | Medium | Medium |
| P3 | Background Refresh | Low | Low (polish) |

---

## Notes

**Read-only by default**: All new views should be read-only unless explicitly in interactive mode. This matches the existing browse/interactive mode split.

**Component pattern**: Follow existing pattern - `New()`, `SetData()`, `View(width, height, focused)`.

**Testing**: Each new component needs unit tests for rendering and state management. Integration tests for jj command wrappers.

**Theme integration**: Use existing theme colors - `theme.Primary` for selections, `theme.Muted` for inactive, `theme.Warning` for conflicts.

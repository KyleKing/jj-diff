# jj-diff Roadmap

Three features that earn a place in a diff tool, meaning you want them while reading a
diff. None is built as of 2026-09-02. Known defects and design decisions live in
`FINDINGS.md`, the ordered work lives in `NEXT_STEPS.md`, and the fixture-testing plan
lives in `doctest-jj-diff.md`.

Repo browsing moved out on 2026-09-02. Evolution timeline, operation log, and interactive
smartlog now live in [jj-tui](https://github.com/KyleKing/jj-tui), the lazygit replacement
that runs jj-diff as its diff editor. Anything that browses a repository rather than reads
a diff belongs there.

| Priority | Feature | Effort | Impact |
|----------|---------|--------|--------|
| P0 | Conflict visualization | Medium | High (nothing else shows jj conflict markers well) |
| P1 | Interdiff | Low | Medium (two revisions, one diff view) |
| P2 | Background refresh | Low | Low (polish) |

Section 1 quotes jj's conflict-marker format verbatim, so the `<<<<<<<` and `>>>>>>>`
lines in this file are content, not an unresolved merge. `hk.pkl` excludes this path from
`check-merge-conflict` for that reason; keep the markers and the exclude in step if the
file is ever renamed.

---

## 1. Conflict visualization

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

## 2. Interdiff

**Keybinding**: `I`

**Problem**: When iterating on a change, the useful diff is what changed between two
revisions, not the full diff of either. Comparing a revision against its own previous
evolution is the common case, and comparing two arbitrary revisions is the general one.

**Implementation**:

- Prompt for a from/to revision pair, defaulting `from` to the revision's previous
  evolution entry (`jj evolog -r <rev>`) and `to` to the revision itself
- Compute via `jj diff --from <v1> --to <v2>` and reuse the existing parser and view
- State which pair is on screen in the header, since the diff alone does not say

**Files to Modify**:

- `internal/diff/source.go`: add `InterdiffSource` for two-revision comparison
- `internal/jj/client.go`: add the evolog read behind it
- `internal/model/model.go`: handle the revision-pair prompt and the display mode

---

## 3. Background refresh

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

## Notes

**Read-only by default**: All new views should be read-only unless explicitly in interactive mode. This matches the existing browse/interactive mode split.

**Component pattern**: Follow existing pattern - `New()`, `SetData()`, `View(width, height, focused)`.

**Testing**: Each new component needs unit tests for rendering and state management. Integration tests for jj command wrappers.

**Theme integration**: Use existing theme colors - `theme.Primary` for selections, `theme.Muted` for inactive, `theme.Warning` for conflicts.

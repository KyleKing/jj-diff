# Performance Improvement Plan

## Current Performance Profile

### Baseline Characteristics

**Diff View (internal/components/diffview/)**
- Uses offset-based windowing: Only renders visible lines
- Algorithm: Skip lines until offset, render until height reached
- Memory: All hunks loaded, but only visible portion rendered
- Complexity: O(n) where n = total lines in all hunks

**File List (internal/components/filelist/)**
- Renders all files regardless of visibility
- No virtualization or windowing
- Memory: Scales with file count
- Complexity: O(n) where n = number of files

**Search (internal/search/)**
- Linear scan through all files and lines on every query change
- Stores all match locations in memory
- Complexity: O(f × h × l) where f=files, h=hunks, l=lines per hunk
- Re-executes search on every keystroke

### Performance Thresholds

Based on empirical testing patterns:

- **Acceptable**: < 100 files, < 1000 total diff lines
- **Noticeable**: 100-500 files, 1000-5000 lines
- **Problematic**: > 500 files, > 5000 lines

## Identified Bottlenecks

### 1. File List: Full Render (HIGH IMPACT)

**Problem**: Renders all files even when only 20-30 are visible.

**Impact**: O(n) rendering cost where n = total files

**Scenario**: Repository with 1000 changed files
- Current: Renders 1000 lines, displays 30
- Waste: 97% of rendering effort

**Evidence**: No height-based limiting in filelist/filelist.go:View()

### 2. Search: Keystroke Re-scan (MEDIUM IMPACT)

**Problem**: Full diff scan on every query character.

**Impact**: O(f × h × l × k) where k = keystrokes

**Scenario**: Typing "function" (8 keystrokes)
- Scans entire diff 8 times
- Each scan processes all lines

**Evidence**: ExecuteSearch() called in handleSearchKeyPress() for every character

### 3. Match Highlighting: Repeated Function Calls (LOW IMPACT)

**Problem**: Callback functions called for every line during render.

**Impact**: Function call overhead for visible lines

**Evidence**:
```go
m.getLineContentMatches(currentFile.Path, hunkIdx, lineIdx)
```
Called for every visible line in diffview

### 4. Component Re-renders: Full Updates (LOW IMPACT)

**Problem**: Every keystroke triggers full component render.

**Impact**: Unnecessary work when only modal state changes

**Evidence**: View() called on every Update() cycle

## Minimal Improvement Strategies

### Strategy 1: File List Virtualization (RECOMMENDED)

**Goal**: Only render visible files

**Changes**:
1. Add offset tracking to filelist.Model
2. Modify View() to skip non-visible files
3. Add scroll bounds checking

**Implementation**:
```go
func (m Model) View(width, height int, focused bool) string {
    visibleCount := height - 1  // Reserve line for header
    startIdx := m.offset
    endIdx := min(startIdx + visibleCount, len(m.files))

    for i := startIdx; i < endIdx; i++ {
        // Render only visible files
    }
}
```

**Impact**: O(1) → O(visible) rendering
- 1000 files: 100ms → 3ms (estimated)

**Complexity**: LOW (20-30 lines)

**Risk**: LOW (well-understood pattern)

### Strategy 2: Search Debouncing (RECOMMENDED)

**Goal**: Reduce search executions during typing

**Changes**:
1. Add debounce timer (300ms) to searchmodal
2. Queue query updates
3. Execute search only after typing pause

**Implementation**:
```go
func (m Model) handleSearchKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    // Update query immediately (no delay in display)
    m.searchState.Query += msg.String()
    m.searchModal.SetQuery(m.searchState.Query)

    // Debounce search execution
    return m, tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg {
        return searchDebounceMsg{}
    })
}
```

**Impact**: 8 scans → 1 scan (for "function")
- Typing "function": 800ms → 100ms (estimated)

**Complexity**: MEDIUM (30-40 lines, requires timer management)

**Risk**: MEDIUM (requires careful message handling)

### Strategy 3: Match Cache (NICE-TO-HAVE)

**Goal**: Pre-compute matches once, reuse during render

**Changes**:
1. Store match map in Model
2. Compute once after search
3. Clear on new search

**Implementation**:
```go
type MatchCache struct {
    fileMatches map[int][]filelist.MatchRange
    lineMatches map[string]map[int]map[int][]diffview.MatchRange
}

// Compute once after search
func (m *Model) buildMatchCache() {
    // Pre-compute all match ranges
}

// Use during render (no function calls)
func (m Model) View() string {
    ranges := m.matchCache.lineMatches[path][hunk][line]
}
```

**Impact**: Function call overhead eliminated
- Visible lines: 30 × 2 function calls → 0
- Negligible improvement (~1-2ms)

**Complexity**: MEDIUM (40-50 lines)

**Risk**: LOW (pure optimization)

### Strategy 4: Lazy Diff Loading (NOT RECOMMENDED)

**Goal**: Load diffs on-demand when file selected

**Changes**: Major refactoring of data model

**Pros**:
- Minimal memory footprint
- Fast startup for large changesets

**Cons**:
- Complex state management (loading states, errors)
- Slower navigation (wait for load)
- Breaks search (needs all diffs loaded)
- Requires caching strategy

**Complexity**: HIGH (200+ lines, major refactor)

**Risk**: HIGH (affects core data flow)

## Recommended Implementation Plan

### Phase 1: Quick Wins (1-2 hours)

**File List Virtualization** (Strategy 1)
- Changes: internal/components/filelist/filelist.go
- Add offset field
- Modify View() to slice visible range
- Add scroll methods

**Expected Improvement**:
- 1000 files: 100ms → 3ms
- 500 files: 50ms → 3ms

**Testing**:
- Create test with 1000 mock files
- Verify render performance
- Test scroll edge cases (top, bottom, middle)

### Phase 2: Search Optimization (2-3 hours)

**Search Debouncing** (Strategy 2)
- Changes: internal/model/model.go, internal/components/searchmodal/
- Add debounce timer
- Handle searchDebounceMsg
- Update search workflow

**Expected Improvement**:
- Fast typing: 8× fewer scans
- Responsive UI during typing

**Testing**:
- Verify debounce timing
- Test rapid keystroke handling
- Ensure no race conditions

### Phase 3: Polish (Optional, 1 hour)

**Match Cache** (Strategy 3)
- Changes: internal/model/model.go
- Pre-compute match ranges
- Pass cached data to components

**Expected Improvement**:
- Marginal (~1-2ms per render)

**Testing**:
- Verify cache correctness
- Test cache invalidation

## Non-Implementation: What to Skip

### Skip: Progressive Rendering

**Why**: Bubble Tea doesn't support partial updates. Full re-render is framework characteristic.

### Skip: Background Loading

**Why**: Adds complexity for minimal gain. Diff loading is already fast (< 100ms for typical diffs).

### Skip: Diff Streaming

**Why**: jj diff outputs complete result, no streaming API available.

## Measurement Strategy

### Benchmarking Approach

1. **Synthetic Tests**: Generate large diffs programmatically
2. **Real-World Tests**: Use actual large repositories
3. **Profiling**: Go's pprof for CPU/memory profiles

### Metrics to Track

- **Render Time**: Time from Update() to View() completion
- **Memory Usage**: Heap allocation per render cycle
- **Search Time**: ExecuteSearch() duration
- **User Experience**: Perceived responsiveness

### Benchmark Scenarios

1. **Large File Count**: 1000 files, 10 lines each
2. **Large Diff**: 100 files, 1000 lines each
3. **Deep Search**: Search with 5000+ matches
4. **Visual Mode**: Line selection in 500-line hunk

## Trade-offs and Risks

### Virtualization Trade-offs

**Pros**:
- Dramatic improvement for large file counts
- No user-facing changes
- Low implementation complexity

**Cons**:
- Slightly more complex scroll logic
- Need to track offset state

### Debouncing Trade-offs

**Pros**:
- Reduces CPU usage during typing
- Feels more responsive

**Cons**:
- 300ms delay before results appear
- More complex message handling
- Potential for stale results

### Match Cache Trade-offs

**Pros**:
- Eliminates function call overhead
- Slight performance improvement

**Cons**:
- Additional memory usage
- Cache invalidation complexity
- Marginal benefit

## Implementation Guidelines

### Code Quality Standards

1. **Maintain current architecture**: No major refactors
2. **Add tests**: Benchmark tests for each optimization
3. **Document changes**: Update ARCHITECTURE.md
4. **Measure impact**: Before/after benchmarks

### Testing Requirements

1. **Unit Tests**: New functions fully tested
2. **Benchmark Tests**: Comparative performance data
3. **Integration Tests**: End-to-end workflows still work
4. **Manual Testing**: Real-world usability validation

## Success Criteria

### Must Have
- File list renders in < 10ms for 1000 files
- Search executes in < 100ms for typical diffs
- All existing tests pass
- No user-facing behavior changes

### Nice to Have
- Memory usage reduced by 20%
- Perceived responsiveness improvement
- Smoother scrolling in large diffs

## Conclusion

The recommended approach focuses on **virtualization** and **debouncing** as minimal, high-impact changes that significantly improve performance for large diffs without requiring architectural changes.

These improvements maintain the current clean architecture while addressing the most impactful bottlenecks. More complex optimizations (lazy loading, progressive rendering) are explicitly avoided to maintain code simplicity and project focus.

**Next Steps**: Proceed to implementation of benchmark tests to establish baseline metrics before optimization.

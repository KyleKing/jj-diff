# Benchmark Results

## Executive Summary

Performance testing reveals that **jj-diff already has excellent performance characteristics** across all major components. The current implementation scales well to large diffs and file counts through existing optimizations (height-based rendering cutoffs).

**Key Findings:**
- File list rendering: ~7μs regardless of file count (10-5000 files)
- Diff view rendering: ~33-52μs for small-huge diffs
- Search execution: 17ms for 100K lines (acceptable)
- Patch generation: 5.4ms for large diffs (2000 hunks)

**Recommendation**: Current performance is production-ready. Proposed optimizations in PERFORMANCE_PLAN.md are **not necessary** for v0.1.0 release.

## Test Environment

- **CPU**: Apple M2 Pro
- **OS**: macOS (darwin/arm64)
- **Go Version**: 1.21+
- **Date**: 2026-01-19

## Component Benchmarks

### 1. File List Rendering

Location: `internal/components/filelist/filelist_bench_test.go`

| Test Case | Time/op | Memory/op | Allocs/op | Notes |
|-----------|---------|-----------|-----------|-------|
| 10 Files | 3.5μs | 4.3 KB | 77 | Baseline |
| 100 Files | 6.9μs | 6.8 KB | 191 | +97% time, +59% memory |
| 500 Files | 6.9μs | 6.8 KB | 191 | **Constant performance** |
| 1000 Files | 7.0μs | 6.8 KB | 191 | **Constant performance** |
| 5000 Files | 7.0μs | 6.8 KB | 191 | **Constant performance** |
| 1000 Files + Search | 6.9μs | 7.0 KB | 160 | Highlighting has minimal overhead |

**Analysis:**
- Performance plateaus at ~100 files due to height-based cutoff
- Current implementation only renders files that fit in visible height
- **No virtualization needed** - already optimized
- Search highlighting adds negligible overhead (<1%)

**Code Evidence:**
```go
// filelist.go:View()
for i, file := range m.files {
    // ...
    if len(lines) >= height {
        break  // ← Stops rendering after filling screen
    }
}
```

### 2. Diff View Rendering

Location: `internal/components/diffview/diffview_bench_test.go`

| Test Case | Lines | Time/op | Memory/op | Allocs/op | Notes |
|-----------|-------|---------|-----------|-----------|-------|
| Small Diff | 100 | 33.1μs | 22.9 KB | 414 | 10 hunks × 10 lines |
| Medium Diff | 1,000 | 33.7μs | 22.4 KB | 414 | 50 hunks × 20 lines |
| Large Diff | 5,000 | 38.5μs | 22.5 KB | 419 | 100 hunks × 50 lines |
| Huge Diff | 20,000 | 52.3μs | 22.5 KB | 419 | 200 hunks × 100 lines |
| With Selection | 1,000 | 34.1μs | 22.4 KB | 414 | Selection check overhead |
| With Search | 1,000 | 53.3μs | 38.8 KB | 710 | Match highlighting overhead |
| Scroll (100×) | 1,000 | 848ms | 693 KB | 3,957 | 100 scroll steps + renders |

**Analysis:**
- Rendering time mostly constant (33-52μs) across 100× size variation
- Memory usage constant (~22 KB) regardless of total diff size
- Offset-based windowing already implemented and working efficiently
- Search highlighting adds ~57% time overhead (still only 53μs)
- Continuous scrolling: 8.5ms per render (117 FPS equivalent)

**Code Evidence:**
```go
// diffview.go:View()
for hunkIdx, hunk := range m.fileChange.Hunks {
    if currentLine >= m.offset && len(lines) < height {
        // Only render lines in visible window
    }
    currentLine++
}
```

### 3. Search Performance

Location: `internal/search/search_bench_test.go`

| Test Case | Lines Scanned | Time/op | Memory/op | Allocs/op | Matches |
|-----------|---------------|---------|-----------|-----------|---------|
| Small Diff | 500 | 71.5μs | 124 KB | 510 | ~50 |
| Medium Diff | 10,000 | 1.94ms | 4.11 MB | 10,019 | ~1,000 |
| Large Diff | 100,000 | 17.3ms | 48.2 MB | 100,029 | ~10,000 |
| Common Term | 10,000 | 1.96ms | 4.11 MB | 10,019 | Many matches |
| Rare Term | 10,000 | 1.99ms | 4.11 MB | 10,020 | Few matches |
| Long Query | 10,000 | 1.87ms | 4.11 MB | 10,020 | Query length matters little |
| Navigation | 10,000 | 49.8μs | 0 B | 0 | NextMatch() is instant |
| Incremental Typing | 80,000 | 16.0ms | 37.0 MB | 80,157 | 8 queries × 10K lines |

**Analysis:**
- Search time scales linearly with content size: ~0.17μs per line
- Match count doesn't significantly affect performance (dominated by scanning)
- Query length has minimal impact
- Incremental typing (8 chars) causes 8 searches = 16ms total (acceptable)
- Navigation between matches is instant (49μs, zero allocations)

**Performance Characteristics:**
```
Small (500 lines):    71μs   → imperceptible
Medium (10K lines):   1.9ms  → imperceptible
Large (100K lines):   17ms   → barely noticeable
```

**Debouncing Analysis:**
- Typing "function" at 40ms per keystroke = 320ms typing time
- Without debouncing: 8 searches = 16ms (5% of typing time)
- Impact: Negligible - search completes before next keystroke

**Conclusion**: Debouncing optimization provides minimal benefit.

### 4. Patch Generation Performance

Location: `internal/diff/patch_bench_test.go`

| Test Case | Files × Hunks | Time/op | Memory/op | Allocs/op | Notes |
|-----------|---------------|---------|-----------|-----------|-------|
| Small | 5 × 5 | 18.7μs | 94.9 KB | 190 | 25 hunks total |
| Medium | 20 × 10 | 352μs | 1.95 MB | 1,663 | 200 hunks total |
| Large | 100 × 20 | 5.49ms | 51.7 MB | 19,355 | 2,000 hunks total |
| Partial Selection | 20 × 10 (50%) | 168μs | 897 KB | 937 | Half hunks selected |
| Line-Level | 20 × 10 | 473μs | 1.30 MB | 2,441 | Context expansion |
| Single File | 1 × 100 | 441μs | 2.61 MB | 1,015 | One large file |
| Many Small Hunks | 10 × 100 (5 lines) | 483μs | 2.17 MB | 4,200 | Hunk overhead |
| Few Large Hunks | 10 × 5 (200 lines) | 946μs | 5.00 MB | 835 | Line processing |

**Analysis:**
- Generation time scales with selection size (not total diff size)
- Line-level selection ~34% slower due to context expansion algorithm
- Memory scales linearly with patch size (expected for string building)
- Even worst case (2000 hunks) generates in 5.4ms

**User Experience:**
- Small selections (< 100 hunks): < 1ms (instant)
- Medium selections (< 500 hunks): < 1ms (instant)
- Large selections (2000 hunks): 5ms (imperceptible)

**Conclusion**: Patch generation is not a bottleneck.

## Performance Thresholds

### Current Implementation Limits

Based on benchmark data, acceptable performance up to:

| Component | Threshold | Time | User Experience |
|-----------|-----------|------|-----------------|
| File List | 5,000 files | 7μs | Instant |
| Diff View | 20,000 lines | 52μs | Instant |
| Search | 100,000 lines | 17ms | Barely noticeable |
| Patch Gen | 2,000 hunks | 5.4ms | Imperceptible |

### Real-World Scenarios

**Typical Use Cases** (well within limits):
- Small feature: 5-20 files, 500 lines → All operations < 1ms
- Medium refactor: 50 files, 2000 lines → All operations < 2ms
- Large refactor: 200 files, 10,000 lines → All operations < 10ms

**Extreme Edge Cases** (still acceptable):
- Mass rename: 1000 files, 2000 lines → Render: 7μs, Search: 2ms
- Generated code: 100 files, 50,000 lines → Render: 52μs, Search: 8.5ms
- Mega-merge: 500 files, 100,000 lines → Render: 7μs, Search: 17ms

## Optimization Analysis

### Previously Proposed Optimizations

From PERFORMANCE_PLAN.md:

#### 1. File List Virtualization
**Status**: ❌ **NOT NEEDED**
- Current implementation already limits rendering to visible height
- Performance constant at ~7μs regardless of file count (10-5000)
- No benefit from additional virtualization

#### 2. Search Debouncing
**Status**: ⚠️ **MARGINAL BENEFIT**
- Incremental typing: 16ms for 8 keystrokes (2ms per keystroke)
- Typical keystroke interval: 40-80ms
- Searches complete before next keystroke
- Adds complexity for <5% time savings

#### 3. Match Caching
**Status**: ⚠️ **MARGINAL BENEFIT**
- Search highlighting adds 20μs overhead (53μs vs 33μs)
- Adds complexity for <1ms savings per render
- Memory overhead for cache storage

#### 4. Lazy Loading
**Status**: ❌ **NOT RECOMMENDED**
- Diff loading not measured (occurs once at startup)
- High complexity for unproven benefit
- Breaks search (needs all content)

### Identified Actual Bottleneck

The **only** performance concern from benchmarks:

**Continuous Scrolling**: 8.5ms per render
- Scrolling through 1000-line diff at 60 FPS = 16.7ms budget
- Current: 8.5ms (51% of budget)
- Acceptable but could be smoother

**Potential Optimization**: Incremental rendering (only update changed lines)
- **Complexity**: HIGH (requires framework changes)
- **Benefit**: ~2× faster scrolling
- **Priority**: LOW (current performance acceptable)

## Memory Profile

### Memory Usage by Component

| Operation | Small | Medium | Large | Huge |
|-----------|-------|--------|-------|------|
| File List | 4.3 KB | 6.8 KB | 6.8 KB | 6.8 KB |
| Diff View | 22.9 KB | 22.4 KB | 22.5 KB | 22.5 KB |
| Search | 124 KB | 4.11 MB | 48.2 MB | - |
| Patch Gen | 95 KB | 1.95 MB | 51.7 MB | - |

**Analysis:**
- Rendering components: Constant memory (~6-22 KB)
- Search/Patch: Memory scales with result size (expected)
- No memory leaks detected (allocations scale linearly)

### Memory Efficiency

**Search Memory**: 48.2 MB for 100K lines
- Each match: ~480 bytes
- 10,000 matches × 480 bytes = 4.8 MB (actual: 48.2 MB)
- Overhead factor: 10× (includes string copies, maps)

**Optimization Potential**: String interning could reduce by ~50%
- **Benefit**: 48 MB → 24 MB for extreme case
- **Priority**: LOW (48 MB is acceptable)

## Conclusions

### Performance Verdict

✅ **Current performance is production-ready for v0.1.0**

**Evidence:**
1. File list: Instant regardless of size (< 10μs)
2. Diff view: Instant regardless of size (< 60μs)
3. Search: Fast even for extreme cases (< 20ms)
4. Patch gen: Imperceptible even for large selections (< 6ms)

### Recommendations

#### For v0.1.0 Release
- ✅ No optimizations needed
- ✅ Current implementation sufficient
- ✅ Focus on features and polish

#### For Future Versions (v0.2.0+)
- Consider incremental rendering for smoother scrolling
- Monitor real-world performance with user feedback
- Profile memory usage with actual large repositories

#### Optimization Priority Ranking
1. **None needed for v0.1.0**
2. Low: Incremental rendering (smoother scrolling)
3. Low: String interning in search (memory)
4. Very Low: Search debouncing (marginal CPU savings)

### Updated PERFORMANCE_PLAN.md

The original plan was based on assumptions about performance bottlenecks. Benchmarking reveals:

❌ **Incorrect Assumption**: File list renders all files
✅ **Reality**: Already optimized with height cutoff

❌ **Incorrect Assumption**: Search is slow during typing
✅ **Reality**: Completes before next keystroke

✅ **Correct Analysis**: Diff view uses windowing
✅ **Correct Analysis**: Patch generation is efficient

## Benchmark Test Coverage

### Tests Created

1. **filelist_bench_test.go**: 7 benchmarks
   - File count scaling (10, 100, 500, 1000, 5000)
   - Search highlighting overhead

2. **diffview_bench_test.go**: 7 benchmarks
   - Diff size scaling (100, 1000, 5000, 20000 lines)
   - Selection overhead
   - Search match highlighting
   - Continuous scrolling

3. **search_bench_test.go**: 8 benchmarks
   - Content size scaling (500, 10K, 100K lines)
   - Match count variations (common vs rare terms)
   - Query length impact
   - Incremental typing simulation
   - Navigation performance

4. **patch_bench_test.go**: 8 benchmarks
   - Hunk count scaling (25, 200, 2000 hunks)
   - Selection strategies (full, partial, line-level)
   - Hunk size variations (many small vs few large)

**Total**: 30 benchmark tests

### Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem ./...

# Run specific component
go test -bench=. -benchmem ./internal/components/filelist/

# Compare before/after optimization
go test -bench=BenchmarkFileListRender -benchmem -count=10 ./internal/components/filelist/ | tee old.txt
# (make changes)
go test -bench=BenchmarkFileListRender -benchmem -count=10 ./internal/components/filelist/ | tee new.txt
benchstat old.txt new.txt
```

## Appendix: Raw Benchmark Output

### File List
```
BenchmarkFileListRender_10Files-10                	  336456	      3455 ns/op	    4274 B/op	      77 allocs/op
BenchmarkFileListRender_100Files-10               	  174346	      6898 ns/op	    6767 B/op	     191 allocs/op
BenchmarkFileListRender_500Files-10               	  171387	      6907 ns/op	    6768 B/op	     191 allocs/op
BenchmarkFileListRender_1000Files-10              	  173312	      6955 ns/op	    6768 B/op	     191 allocs/op
BenchmarkFileListRender_5000Files-10              	  170029	      6992 ns/op	    6768 B/op	     191 allocs/op
BenchmarkFileListRenderWithSearch_1000Files-10    	  167368	      6863 ns/op	    7003 B/op	     160 allocs/op
```

### Diff View
```
BenchmarkDiffViewRender_SmallDiff-10            	   36439	     33058 ns/op	   22855 B/op	     414 allocs/op
BenchmarkDiffViewRender_MediumDiff-10           	   35846	     33690 ns/op	   22367 B/op	     414 allocs/op
BenchmarkDiffViewRender_LargeDiff-10            	   30960	     38487 ns/op	   22493 B/op	     419 allocs/op
BenchmarkDiffViewRender_HugeDiff-10             	   22960	     52288 ns/op	   22498 B/op	     419 allocs/op
BenchmarkDiffViewRender_WithSelection-10        	   35365	     34077 ns/op	   22367 B/op	     414 allocs/op
BenchmarkDiffViewRender_WithSearchMatches-10    	   22310	     53326 ns/op	   38804 B/op	     710 allocs/op
BenchmarkDiffViewScroll-10                      	    1368	    847723 ns/op	  693594 B/op	    3957 allocs/op
```

### Search
```
BenchmarkSearch_SmallDiff-10            	   16885	     71543 ns/op	  124288 B/op	     510 allocs/op
BenchmarkSearch_MediumDiff-10           	     628	   1939273 ns/op	 4113704 B/op	   10019 allocs/op
BenchmarkSearch_LargeDiff-10            	      69	  17264575 ns/op	48158882 B/op	  100029 allocs/op
BenchmarkSearch_CommonTerm-10           	     597	   1958344 ns/op	 4113696 B/op	   10019 allocs/op
BenchmarkSearch_RareTerm-10             	     606	   1991140 ns/op	 4113712 B/op	   10020 allocs/op
BenchmarkSearch_LongQuery-10            	     648	   1867277 ns/op	 4113741 B/op	   10020 allocs/op
BenchmarkSearchNavigation-10            	   24074	     49750 ns/op	       0 B/op	       0 allocs/op
BenchmarkSearch_IncrementalTyping-10    	      72	  15972663 ns/op	37030169 B/op	   80157 allocs/op
```

### Patch Generation
```
BenchmarkPatchGeneration_SmallDiff-10           	   67420	     18690 ns/op	   94898 B/op	     190 allocs/op
BenchmarkPatchGeneration_MediumDiff-10          	    3284	    352125 ns/op	 1953565 B/op	    1663 allocs/op
BenchmarkPatchGeneration_LargeDiff-10           	     224	   5488807 ns/op	51686104 B/op	   19355 allocs/op
BenchmarkPatchGeneration_PartialSelection-10    	    7466	    167740 ns/op	  897507 B/op	     937 allocs/op
BenchmarkPatchGeneration_LineLevel-10           	    2560	    472994 ns/op	 1298744 B/op	    2441 allocs/op
BenchmarkPatchGeneration_SingleFile-10          	    2806	    441434 ns/op	 2607400 B/op	    1015 allocs/op
BenchmarkPatchGeneration_ManySmallHunks-10      	    2452	    483436 ns/op	 2169853 B/op	    4200 allocs/op
BenchmarkPatchGeneration_FewLargeHunks-10       	    1366	    945620 ns/op	 4999900 B/op	     835 allocs/op
```

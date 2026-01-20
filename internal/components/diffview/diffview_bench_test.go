package diffview

import (
	"fmt"
	"testing"

	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
)

func testConfig() config.Config {
	return config.DefaultConfig()
}

// generateHunks creates n hunks with linesPerHunk lines each
func generateHunks(n, linesPerHunk int) []diff.Hunk {
	hunks := make([]diff.Hunk, n)
	for i := 0; i < n; i++ {
		lines := make([]diff.Line, linesPerHunk)
		for j := 0; j < linesPerHunk; j++ {
			lineType := diff.LineContext
			if j%3 == 0 {
				lineType = diff.LineAddition
			} else if j%5 == 0 {
				lineType = diff.LineDeletion
			}

			lines[j] = diff.Line{
				Type:       lineType,
				Content:    fmt.Sprintf("This is line %d with some content that makes it realistic", j),
				OldLineNum: j + 1,
				NewLineNum: j + 1,
			}
		}

		hunks[i] = diff.Hunk{
			Header: fmt.Sprintf("@@ -%d,%d +%d,%d @@", i*linesPerHunk+1, linesPerHunk, i*linesPerHunk+1, linesPerHunk),
			Lines:  lines,
		}
	}
	return hunks
}

// BenchmarkDiffViewRender_SmallDiff benchmarks 10 hunks with 10 lines each (100 total lines)
func BenchmarkDiffViewRender_SmallDiff(b *testing.B) {
	fileChange := diff.FileChange{
		Path:       "src/main.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks:      generateHunks(10, 10),
	}

	m := New(testConfig())
	m.SetFileChange(fileChange)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View(80, 40, false)
	}
}

// BenchmarkDiffViewRender_MediumDiff benchmarks 50 hunks with 20 lines each (1000 total lines)
func BenchmarkDiffViewRender_MediumDiff(b *testing.B) {
	fileChange := diff.FileChange{
		Path:       "src/main.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks:      generateHunks(50, 20),
	}

	m := New(testConfig())
	m.SetFileChange(fileChange)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View(80, 40, false)
	}
}

// BenchmarkDiffViewRender_LargeDiff benchmarks 100 hunks with 50 lines each (5000 total lines)
func BenchmarkDiffViewRender_LargeDiff(b *testing.B) {
	fileChange := diff.FileChange{
		Path:       "src/main.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks:      generateHunks(100, 50),
	}

	m := New(testConfig())
	m.SetFileChange(fileChange)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View(80, 40, false)
	}
}

// BenchmarkDiffViewRender_HugeDiff benchmarks 200 hunks with 100 lines each (20000 total lines - stress test)
func BenchmarkDiffViewRender_HugeDiff(b *testing.B) {
	fileChange := diff.FileChange{
		Path:       "src/main.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks:      generateHunks(200, 100),
	}

	m := New(testConfig())
	m.SetFileChange(fileChange)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View(80, 40, false)
	}
}

// BenchmarkDiffViewRender_WithSelection benchmarks rendering with hunk selection
func BenchmarkDiffViewRender_WithSelection(b *testing.B) {
	fileChange := diff.FileChange{
		Path:       "src/main.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks:      generateHunks(50, 20),
	}

	m := New(testConfig())
	m.SetFileChange(fileChange)
	m.SetSelection(5, func(hunkIdx int) bool {
		return hunkIdx == 5
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View(80, 40, false)
	}
}

// BenchmarkDiffViewRender_WithSearchMatches benchmarks rendering with search match highlighting
func BenchmarkDiffViewRender_WithSearchMatches(b *testing.B) {
	fileChange := diff.FileChange{
		Path:       "src/main.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks:      generateHunks(50, 20),
	}

	m := New(testConfig())
	m.SetFileChange(fileChange)
	m.SetSearchState(true, func(hunkIdx, lineIdx int) []MatchRange {
		// Simulate one match per line
		return []MatchRange{{Start: 10, End: 15}}
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View(80, 40, false)
	}
}

// BenchmarkDiffViewScroll benchmarks scrolling performance
func BenchmarkDiffViewScroll(b *testing.B) {
	fileChange := diff.FileChange{
		Path:       "src/main.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks:      generateHunks(100, 50),
	}

	m := New(testConfig())
	m.SetFileChange(fileChange)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for offset := 0; offset < 100; offset++ {
			m.Scroll(1)
			_ = m.View(80, 40, false)
		}
	}
}

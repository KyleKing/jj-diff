package diffview_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kyleking/jj-diff/internal/components/diffview"
	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
)

func testConfig() config.Config {
	return config.DefaultConfig()
}

// generateHunks creates n hunks with linesPerHunk lines each, numbered the way one file's diff is.
func generateHunks(n, linesPerHunk int) []diff.Hunk {
	hunks := make([]diff.Hunk, n)
	for i := range n {
		first := i*linesPerHunk + 1
		lines := make([]diff.Line, linesPerHunk)
		for j := range linesPerHunk {
			lineType := diff.LineContext
			if j%3 == 0 {
				lineType = diff.LineAddition
			} else if j%5 == 0 {
				lineType = diff.LineDeletion
			}

			lines[j] = diff.Line{
				Type: lineType,
				Content: fmt.Sprintf(
					"This is line %d with some content that makes it realistic",
					j,
				),
				OldLineNum: first + j,
				NewLineNum: first + j,
			}
		}

		hunks[i] = diff.Hunk{
			Header: fmt.Sprintf(
				"@@ -%d,%d +%d,%d @@",
				i*linesPerHunk+1,
				linesPerHunk,
				i*linesPerHunk+1,
				linesPerHunk,
			),
			Lines: lines,
		}
	}

	return hunks
}

// BenchmarkDiffViewRender_SmallDiff benchmarks 10 hunks with 10 lines each (100 total lines).
func BenchmarkDiffViewRender_SmallDiff(b *testing.B) {
	fileChange := diff.FileChange{
		Path:       "src/main.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks:      generateHunks(10, 10),
	}

	m := diffview.New(testConfig())
	m.SetFileChange(fileChange)

	_ = m.View(80, 40, false)

	b.ResetTimer()
	for range b.N {
		_ = m.View(80, 40, false)
	}
}

// BenchmarkDiffViewRender_MediumDiff benchmarks 50 hunks with 20 lines each (1000 total lines).
func BenchmarkDiffViewRender_MediumDiff(b *testing.B) {
	fileChange := diff.FileChange{
		Path:       "src/main.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks:      generateHunks(50, 20),
	}

	m := diffview.New(testConfig())
	m.SetFileChange(fileChange)

	_ = m.View(80, 40, false)

	b.ResetTimer()
	for range b.N {
		_ = m.View(80, 40, false)
	}
}

// BenchmarkDiffViewRender_LargeDiff benchmarks 100 hunks with 50 lines each (5000 total lines).
func BenchmarkDiffViewRender_LargeDiff(b *testing.B) {
	fileChange := diff.FileChange{
		Path:       "src/main.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks:      generateHunks(100, 50),
	}

	m := diffview.New(testConfig())
	m.SetFileChange(fileChange)

	_ = m.View(80, 40, false)

	b.ResetTimer()
	for range b.N {
		_ = m.View(80, 40, false)
	}
}

// BenchmarkDiffViewRender_HugeDiff benchmarks 200 hunks with 100 lines each (20000 total lines - stress test).
func BenchmarkDiffViewRender_HugeDiff(b *testing.B) {
	fileChange := diff.FileChange{
		Path:       "src/main.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks:      generateHunks(200, 100),
	}

	m := diffview.New(testConfig())
	m.SetFileChange(fileChange)

	_ = m.View(80, 40, false)

	b.ResetTimer()
	for range b.N {
		_ = m.View(80, 40, false)
	}
}

// BenchmarkDiffViewRender_WithSelection benchmarks rendering with hunk selection.
func BenchmarkDiffViewRender_WithSelection(b *testing.B) {
	fileChange := diff.FileChange{
		Path:       "src/main.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks:      generateHunks(50, 20),
	}

	m := diffview.New(testConfig())
	m.SetFileChange(fileChange)
	m.SetSelection(5, func(hunkIdx int) bool {
		return hunkIdx == 5
	})

	_ = m.View(80, 40, false)

	b.ResetTimer()
	for range b.N {
		_ = m.View(80, 40, false)
	}
}

// BenchmarkDiffViewRender_WithSearchMatches benchmarks rendering with search match highlighting.
func BenchmarkDiffViewRender_WithSearchMatches(b *testing.B) {
	fileChange := diff.FileChange{
		Path:       "src/main.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks:      generateHunks(50, 20),
	}

	m := diffview.New(testConfig())
	m.SetFileChange(fileChange)
	m.SetSearchState(true, func(_, _ int) []diffview.MatchRange {
		// Simulate one match per line
		return []diffview.MatchRange{{Start: 10, End: 15}}
	})

	_ = m.View(80, 40, false)

	b.ResetTimer()
	for range b.N {
		_ = m.View(80, 40, false)
	}
}

// BenchmarkDiffViewScroll benchmarks scrolling performance.
func BenchmarkDiffViewScroll(b *testing.B) {
	fileChange := diff.FileChange{
		Path:       "src/main.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks:      generateHunks(100, 50),
	}

	m := diffview.New(testConfig())
	m.SetFileChange(fileChange)

	_ = m.View(80, 40, false)

	b.ResetTimer()
	for range b.N {
		for range 100 {
			m.Scroll(1)
			_ = m.View(80, 40, false)
		}
	}
}

// realisticFile parses a Go file diff of about rows rows, the way a file loaded from jj arrives.
func realisticFile(rows int) diff.FileChange {
	var b strings.Builder

	fmt.Fprintf(&b, "diff --git a/svc.go b/svc.go\n--- a/svc.go\n+++ b/svc.go\n@@ -1,%d +1,%d @@\n", rows, rows)

	for i := range rows {
		switch i % 4 {
		case 0:
			fmt.Fprintf(&b, "-\tconst limit%d = %d // old\n", i, i)
		case 1:
			fmt.Fprintf(&b, "+\tconst limit%d = %d // new\n", i-1, i+1)
		default:
			fmt.Fprintf(&b, " \tfunc handle%d(ctx context.Context) error { return nil }\n", i)
		}
	}

	return diff.Parse(b.String())[0]
}

// BenchmarkDiffViewFirstFrame_2000Rows benchmarks opening a 2000-row file: load it and draw the
// first screen, which is where the file is lexed once.
func BenchmarkDiffViewFirstFrame_2000Rows(b *testing.B) {
	file := realisticFile(2000)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		m := diffview.New(testConfig())
		m.SetFileChange(file)
		_ = m.View(160, 44, false)
	}
}

// BenchmarkDiffViewScroll_2000Rows benchmarks drawing screens of an already-open 2000-row file.
func BenchmarkDiffViewScroll_2000Rows(b *testing.B) {
	m := diffview.New(testConfig())
	m.SetFileChange(realisticFile(2000))

	_ = m.View(160, 44, false)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		m.Scroll(1)
		_ = m.View(160, 44, false)
	}
}

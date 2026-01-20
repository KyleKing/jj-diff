package search

import (
	"fmt"
	"testing"

	"github.com/kyleking/jj-diff/internal/diff"
)

// generateSearchTestFiles creates test files with realistic content for search benchmarking
func generateSearchTestFiles(fileCount, hunksPerFile, linesPerHunk int) []diff.FileChange {
	files := make([]diff.FileChange, fileCount)

	for i := 0; i < fileCount; i++ {
		hunks := make([]diff.Hunk, hunksPerFile)

		for j := 0; j < hunksPerFile; j++ {
			lines := make([]diff.Line, linesPerHunk)

			for k := 0; k < linesPerHunk; k++ {
				lineType := diff.LineContext
				if k%3 == 0 {
					lineType = diff.LineAddition
				} else if k%5 == 0 {
					lineType = diff.LineDeletion
				}

				// Create realistic content with searchable terms
				content := fmt.Sprintf("function processData(input, config) { return result; } // line %d", k)

				lines[k] = diff.Line{
					Type:       lineType,
					Content:    content,
					OldLineNum: k + 1,
					NewLineNum: k + 1,
				}
			}

			hunks[j] = diff.Hunk{
				Header: fmt.Sprintf("@@ -%d,%d +%d,%d @@", j*linesPerHunk+1, linesPerHunk, j*linesPerHunk+1, linesPerHunk),
				Lines:  lines,
			}
		}

		files[i] = diff.FileChange{
			Path:       fmt.Sprintf("src/components/module%d.go", i),
			ChangeType: diff.ChangeTypeModified,
			Hunks:      hunks,
		}
	}

	return files
}

// BenchmarkSearch_SmallDiff benchmarks search on 10 files, 5 hunks, 10 lines (500 total lines)
func BenchmarkSearch_SmallDiff(b *testing.B) {
	files := generateSearchTestFiles(10, 5, 10)
	s := NewSearchState()
	s.Query = "function"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ExecuteSearch(files)
	}
}

// BenchmarkSearch_MediumDiff benchmarks search on 50 files, 10 hunks, 20 lines (10,000 total lines)
func BenchmarkSearch_MediumDiff(b *testing.B) {
	files := generateSearchTestFiles(50, 10, 20)
	s := NewSearchState()
	s.Query = "function"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ExecuteSearch(files)
	}
}

// BenchmarkSearch_LargeDiff benchmarks search on 100 files, 20 hunks, 50 lines (100,000 total lines)
func BenchmarkSearch_LargeDiff(b *testing.B) {
	files := generateSearchTestFiles(100, 20, 50)
	s := NewSearchState()
	s.Query = "function"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ExecuteSearch(files)
	}
}

// BenchmarkSearch_CommonTerm benchmarks search for very common term (many matches)
func BenchmarkSearch_CommonTerm(b *testing.B) {
	files := generateSearchTestFiles(50, 10, 20)
	s := NewSearchState()
	s.Query = "line" // Appears in every line

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ExecuteSearch(files)
	}
}

// BenchmarkSearch_RareTerm benchmarks search for rare term (few matches)
func BenchmarkSearch_RareTerm(b *testing.B) {
	files := generateSearchTestFiles(50, 10, 20)
	s := NewSearchState()
	s.Query = "processData"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ExecuteSearch(files)
	}
}

// BenchmarkSearch_LongQuery benchmarks search with longer query string
func BenchmarkSearch_LongQuery(b *testing.B) {
	files := generateSearchTestFiles(50, 10, 20)
	s := NewSearchState()
	s.Query = "function processData(input, config)"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ExecuteSearch(files)
	}
}

// BenchmarkSearchNavigation benchmarks iterating through matches
func BenchmarkSearchNavigation(b *testing.B) {
	files := generateSearchTestFiles(50, 10, 20)
	s := NewSearchState()
	s.Query = "function"
	s.ExecuteSearch(files)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < len(s.Matches); j++ {
			_ = s.NextMatch()
		}
	}
}

// BenchmarkSearch_IncrementalTyping simulates typing a query character by character
func BenchmarkSearch_IncrementalTyping(b *testing.B) {
	files := generateSearchTestFiles(50, 10, 20)
	queries := []string{"f", "fu", "fun", "func", "funct", "functi", "functio", "function"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, query := range queries {
			s := NewSearchState()
			s.Query = query
			s.ExecuteSearch(files)
		}
	}
}

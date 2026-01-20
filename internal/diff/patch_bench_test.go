package diff

import (
	"fmt"
	"testing"
)

// generateBenchmarkFiles creates test files for patch generation benchmarking
func generateBenchmarkFiles(fileCount, hunksPerFile, linesPerHunk int) []FileChange {
	files := make([]FileChange, fileCount)

	for i := 0; i < fileCount; i++ {
		hunks := make([]Hunk, hunksPerFile)

		for j := 0; j < hunksPerFile; j++ {
			lines := make([]Line, linesPerHunk)

			for k := 0; k < linesPerHunk; k++ {
				lineType := LineContext
				if k%3 == 0 {
					lineType = LineAddition
				} else if k%5 == 0 {
					lineType = LineDeletion
				}

				lines[k] = Line{
					Type:       lineType,
					Content:    fmt.Sprintf("This is line %d with realistic content for benchmarking", k),
					OldLineNum: k + 1,
					NewLineNum: k + 1,
				}
			}

			hunks[j] = Hunk{
				Header: fmt.Sprintf("@@ -%d,%d +%d,%d @@", j*linesPerHunk+1, linesPerHunk, j*linesPerHunk+1, linesPerHunk),
				Lines:  lines,
			}
		}

		files[i] = FileChange{
			Path:       fmt.Sprintf("src/file%d.go", i),
			ChangeType: ChangeTypeModified,
			Hunks:      hunks,
		}
	}

	return files
}

// createFullSelection creates a selection with all hunks selected
func createFullSelection(files []FileChange) *mockSelectionState {
	selections := make(map[string]map[int]bool)

	for _, file := range files {
		selections[file.Path] = make(map[int]bool)
		for i := range file.Hunks {
			selections[file.Path][i] = true
		}
	}

	return newMockSelection(selections)
}

// createPartialSelection creates a selection with 50% of hunks selected
func createPartialSelection(files []FileChange) *mockSelectionState {
	selections := make(map[string]map[int]bool)

	for _, file := range files {
		selections[file.Path] = make(map[int]bool)
		for i := range file.Hunks {
			if i%2 == 0 {
				selections[file.Path][i] = true
			}
		}
	}

	return newMockSelection(selections)
}

// createLineSelection creates a selection with line-level selections
func createLineSelection(files []FileChange) *mockSelectionState {
	mock := &mockSelectionState{
		selections:     make(map[string]map[int]bool),
		lineSelections: make(map[string]map[int]map[int]bool),
		partialHunks:   make(map[string]map[int]bool),
	}

	for _, file := range files {
		mock.partialHunks[file.Path] = make(map[int]bool)
		mock.lineSelections[file.Path] = make(map[int]map[int]bool)

		for hunkIdx, hunk := range file.Hunks {
			if hunkIdx%2 == 0 {
				mock.partialHunks[file.Path][hunkIdx] = true
				mock.lineSelections[file.Path][hunkIdx] = make(map[int]bool)

				// Select 50% of lines in this hunk
				for lineIdx := range hunk.Lines {
					if lineIdx%2 == 0 {
						mock.lineSelections[file.Path][hunkIdx][lineIdx] = true
					}
				}
			}
		}
	}

	return mock
}

// BenchmarkPatchGeneration_SmallDiff benchmarks patch generation for 5 files, 5 hunks each
func BenchmarkPatchGeneration_SmallDiff(b *testing.B) {
	files := generateBenchmarkFiles(5, 5, 10)
	selection := createFullSelection(files)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GeneratePatch(files, selection)
	}
}

// BenchmarkPatchGeneration_MediumDiff benchmarks patch generation for 20 files, 10 hunks each
func BenchmarkPatchGeneration_MediumDiff(b *testing.B) {
	files := generateBenchmarkFiles(20, 10, 20)
	selection := createFullSelection(files)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GeneratePatch(files, selection)
	}
}

// BenchmarkPatchGeneration_LargeDiff benchmarks patch generation for 100 files, 20 hunks each
func BenchmarkPatchGeneration_LargeDiff(b *testing.B) {
	files := generateBenchmarkFiles(100, 20, 50)
	selection := createFullSelection(files)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GeneratePatch(files, selection)
	}
}

// BenchmarkPatchGeneration_PartialSelection benchmarks with only 50% of hunks selected
func BenchmarkPatchGeneration_PartialSelection(b *testing.B) {
	files := generateBenchmarkFiles(20, 10, 20)
	selection := createPartialSelection(files)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GeneratePatch(files, selection)
	}
}

// BenchmarkPatchGeneration_LineLevel benchmarks with line-level selections
func BenchmarkPatchGeneration_LineLevel(b *testing.B) {
	files := generateBenchmarkFiles(20, 10, 20)
	selection := createLineSelection(files)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GeneratePatch(files, selection)
	}
}

// BenchmarkPatchGeneration_SingleFile benchmarks generating patch for a single large file
func BenchmarkPatchGeneration_SingleFile(b *testing.B) {
	files := generateBenchmarkFiles(1, 100, 50)
	selection := createFullSelection(files)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GeneratePatch(files, selection)
	}
}

// BenchmarkPatchGeneration_ManySmallHunks benchmarks many small hunks
func BenchmarkPatchGeneration_ManySmallHunks(b *testing.B) {
	files := generateBenchmarkFiles(10, 100, 5)
	selection := createFullSelection(files)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GeneratePatch(files, selection)
	}
}

// BenchmarkPatchGeneration_FewLargeHunks benchmarks few large hunks
func BenchmarkPatchGeneration_FewLargeHunks(b *testing.B) {
	files := generateBenchmarkFiles(10, 5, 200)
	selection := createFullSelection(files)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GeneratePatch(files, selection)
	}
}

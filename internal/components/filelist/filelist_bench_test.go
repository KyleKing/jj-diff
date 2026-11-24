package filelist_test

import (
	"fmt"
	"testing"

	"github.com/kyleking/jj-diff/internal/components/filelist"
	"github.com/kyleking/jj-diff/internal/diff"
)

// generateFiles creates n mock file changes for benchmarking.
func generateFiles(n int) []diff.FileChange {
	files := make([]diff.FileChange, n)
	for i := range n {
		files[i] = diff.FileChange{
			Path:       fmt.Sprintf("src/components/file%d.go", i),
			ChangeType: diff.ChangeTypeModified,
			Hunks:      []diff.Hunk{},
		}
	}

	return files
}

// BenchmarkFileListRender_10Files benchmarks rendering with 10 files.
func BenchmarkFileListRender_10Files(b *testing.B) {
	files := generateFiles(10)
	m := filelist.New()
	m.SetFiles(files)

	b.ResetTimer()
	for i := range b.N {
		// Moving the cursor each frame redraws rather than answering the last frame.
		m.SetSelected(i % 2)
		_ = m.View(40, 30, false)
	}
}

// BenchmarkFileListRender_100Files benchmarks rendering with 100 files.
func BenchmarkFileListRender_100Files(b *testing.B) {
	files := generateFiles(100)
	m := filelist.New()
	m.SetFiles(files)

	b.ResetTimer()
	for i := range b.N {
		// Moving the cursor each frame redraws rather than answering the last frame.
		m.SetSelected(i % 2)
		_ = m.View(40, 30, false)
	}
}

// BenchmarkFileListRender_500Files benchmarks rendering with 500 files.
func BenchmarkFileListRender_500Files(b *testing.B) {
	files := generateFiles(500)
	m := filelist.New()
	m.SetFiles(files)

	b.ResetTimer()
	for i := range b.N {
		// Moving the cursor each frame redraws rather than answering the last frame.
		m.SetSelected(i % 2)
		_ = m.View(40, 30, false)
	}
}

// BenchmarkFileListRender_1000Files benchmarks rendering with 1000 files.
func BenchmarkFileListRender_1000Files(b *testing.B) {
	files := generateFiles(1000)
	m := filelist.New()
	m.SetFiles(files)

	b.ResetTimer()
	for i := range b.N {
		// Moving the cursor each frame redraws rather than answering the last frame.
		m.SetSelected(i % 2)
		_ = m.View(40, 30, false)
	}
}

// BenchmarkFileListRender_5000Files benchmarks rendering with 5000 files (stress test).
func BenchmarkFileListRender_5000Files(b *testing.B) {
	files := generateFiles(5000)
	m := filelist.New()
	m.SetFiles(files)

	b.ResetTimer()
	for i := range b.N {
		// Moving the cursor each frame redraws rather than answering the last frame.
		m.SetSelected(i % 2)
		_ = m.View(40, 30, false)
	}
}

// BenchmarkFileListRenderWithSearch_1000Files benchmarks rendering with search matches.
func BenchmarkFileListRenderWithSearch_1000Files(b *testing.B) {
	files := generateFiles(1000)
	m := filelist.New()
	m.SetFiles(files)

	// Simulate search matches on every 10th file
	m.SetSearchState(true, func(fileIdx int) []filelist.MatchRange {
		if fileIdx%10 == 0 {
			return []filelist.MatchRange{{Start: 0, End: 3}}
		}

		return nil
	})

	b.ResetTimer()
	for i := range b.N {
		// Moving the cursor each frame redraws rather than answering the last frame.
		m.SetSelected(i % 2)
		_ = m.View(40, 30, false)
	}
}

// BenchmarkFileListRender_Unchanged is a frame where nothing the list shows changed, which is most
// frames: the diff pane took the key. It answers the last frame drawn.
func BenchmarkFileListRender_Unchanged(b *testing.B) {
	m := filelist.New()
	m.SetFiles(generateFiles(1000))

	_ = m.View(40, 30, false)

	b.ResetTimer()

	for range b.N {
		_ = m.View(40, 30, false)
	}
}

// BenchmarkFileListRender_Expanded draws the focused list of 1000 files with the cursor moving.
func BenchmarkFileListRender_Expanded(b *testing.B) {
	m := filelist.New()
	m.SetFiles(generateFiles(1000))
	m.SetExpanded(true)

	b.ResetTimer()

	for i := range b.N {
		m.SetSelected(i % 40)
		_ = m.View(120, 30, true)
	}
}

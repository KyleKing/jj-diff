package diff_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleking/jj-diff/internal/diff"
)

// writeBenchTrees writes total files of roughly fileBytes each into left and right, identical except
// for the first changed files, which differ in their last line.
func writeBenchTrees(b *testing.B, total, changed, fileBytes int) (string, string) {
	b.Helper()

	base := b.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")

	line := "the quick brown fox jumps over the lazy dog, again and again\n"
	body := strings.Repeat(line, max(fileBytes/len(line), 1))

	for i := range total {
		name := filepath.Join(fmt.Sprintf("pkg%02d", i%50), fmt.Sprintf("file%04d.go", i))
		rightBody := body
		if i < changed {
			rightBody = body + "changed\n"
		}

		for dir, content := range map[string]string{left: body, right: rightBody} {
			path := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				b.Fatalf("MkdirAll: %v", err)
			}

			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				b.Fatalf("WriteFile: %v", err)
			}
		}
	}

	return left, right
}

// BenchmarkCompareDirectories measures a diff-editor invocation on a large tree where only a handful
// of files changed, which is the common case for `jj split` and `jj diffedit`.
func BenchmarkCompareDirectories(b *testing.B) {
	sizes := []struct {
		total, changed, fileBytes int
	}{
		{total: 2000, changed: 10, fileBytes: 4 << 10},
		{total: 200, changed: 10, fileBytes: 64 << 10},
	}

	for _, s := range sizes {
		b.Run(fmt.Sprintf("files=%d/changed=%d/bytes=%d", s.total, s.changed, s.fileBytes), func(b *testing.B) {
			left, right := writeBenchTrees(b, s.total, s.changed, s.fileBytes)

			b.ResetTimer()

			for b.Loop() {
				if _, err := diff.CompareDirectories(left, right); err != nil {
					b.Fatalf("CompareDirectories: %v", err)
				}
			}
		})
	}
}

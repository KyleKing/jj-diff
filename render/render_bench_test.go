package render_test

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/jj-diff/render"
)

// goDiff builds a single-file Go diff of about n rows, mixing context, edits, and additions.
func goDiff(n int) string {
	var b strings.Builder

	b.WriteString("diff --git a/svc.go b/svc.go\nindex 111..222 100644\n--- a/svc.go\n+++ b/svc.go\n")
	fmt.Fprintf(&b, "@@ -1,%d +1,%d @@\n", n, n)

	for i := range n {
		switch i % 4 {
		case 0:
			fmt.Fprintf(&b, "-\tconst limit%d = %d // old\n", i, i)
		case 1:
			fmt.Fprintf(&b, "+\tconst limit%d = %d // new\n", i-1, i+1)
		default:
			fmt.Fprintf(&b, " \tfunc handle%d(ctx context.Context) error { return nil }\n", i)
		}
	}

	return b.String()
}

func benchOptions() render.Options {
	return render.Options{
		Palette: render.Palette{
			Added:       lipgloss.Color("2"),
			Removed:     lipgloss.Color("1"),
			AddedWord:   lipgloss.Color("22"),
			RemovedWord: lipgloss.Color("52"),
			Syntax: render.SyntaxPalette{
				Comment: lipgloss.Color("8"), Keyword: lipgloss.Color("5"), String: lipgloss.Color("3"),
				Number: lipgloss.Color("6"), Name: lipgloss.Color("4"), Type: lipgloss.Color("13"),
			},
		},
		Width:       120,
		Syntax:      true,
		WordDiff:    true,
		LineNumbers: true,
	}
}

func benchmarkLines(b *testing.B, rows int) {
	b.Helper()

	text, opts := goDiff(rows), benchOptions()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = render.Lines(text, opts)
	}
}

func BenchmarkLines240Rows(b *testing.B)  { benchmarkLines(b, 240) }
func BenchmarkLines2000Rows(b *testing.B) { benchmarkLines(b, 2000) }

// BenchmarkRowsFirstScreen2000Rows is what a caller pays to draw one screen of a 2000-row diff.
func BenchmarkRowsFirstScreen2000Rows(b *testing.B) {
	text, opts := goDiff(2000), benchOptions()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = render.New(text, opts).Rows(0, 44)
	}
}

//nolint:testpackage // white-box: benchmarks set unexported model state and send diffLoadedMsg.
package model

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/jj-diff/internal/diff"
)

// benchDiff is a diff of files files, the first a 2000-row Go file and the rest small edits, the
// shape a real change with one large file has.
func benchDiff(files int) []diff.FileChange {
	var b strings.Builder

	b.WriteString("diff --git a/big.go b/big.go\n--- a/big.go\n+++ b/big.go\n@@ -1,2000 +1,2000 @@\n")

	for i := range 2000 {
		switch i % 4 {
		case 0:
			fmt.Fprintf(&b, "-\tconst limit%d = %d // old\n", i, i)
		case 1:
			fmt.Fprintf(&b, "+\tconst limit%d = %d // new\n", i-1, i+1)
		default:
			fmt.Fprintf(&b, " \tfunc handle%d(ctx context.Context) error { return nil }\n", i)
		}
	}

	for f := range files - 1 {
		fmt.Fprintf(&b, "diff --git a/pkg/f%d.go b/pkg/f%d.go\n--- a/pkg/f%d.go\n+++ b/pkg/f%d.go\n", f, f, f, f)
		fmt.Fprintf(&b, "@@ -1,3 +1,3 @@\n package pkg\n-var x = %d\n+var x = %d\n", f, f+1)
	}

	return diff.Parse(b.String())
}

func benchModel(b *testing.B) *Model {
	b.Helper()

	m := NewTestModel(b, ModeInteractive).WithChanges(benchDiff(200))
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	<-m.diffView.Highlighted()

	return m
}

// BenchmarkModelRender_DiffFocused is one full frame with the diff focused: file list, diff pane,
// status bar.
func BenchmarkModelRender_DiffFocused(b *testing.B) {
	m := benchModel(b)
	m.focusedPanel = PanelDiffView

	_ = m.render()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = m.render()
	}
}

// BenchmarkModelRender_FilesFocused is one full frame with the file list focused, which dims the diff.
func BenchmarkModelRender_FilesFocused(b *testing.B) {
	m := benchModel(b)
	m.focusedPanel = PanelFileList

	_ = m.render()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = m.render()
	}
}

// BenchmarkModelRender_Searching is a frame while a common term is highlighted everywhere.
func BenchmarkModelRender_Searching(b *testing.B) {
	m := benchModel(b)
	m.focusedPanel = PanelDiffView
	m.searchState.Query = "limit"
	m.searchState.IsActive = true
	m.searchState.ExecuteSearch(m.changes)

	_ = m.render()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = m.render()
	}
}

// BenchmarkModelKeyJ is pressing j in the diff pane and drawing the frame it leads to.
func BenchmarkModelKeyJ(b *testing.B) {
	m := benchModel(b)
	m.focusedPanel = PanelDiffView

	_ = m.render()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		m.Update(KeyPress('j'))
		_ = m.render()
	}
}

// A search's hits are indexed once per search, so a new query must replace the old one's hits on
// every line and in the overview track, not keep drawing the first search.
func TestSearchIndexFollowsANewQuery(t *testing.T) {
	t.Parallel()

	m := NewTestModel(t, ModeBrowse).WithChanges(benchDiff(3))
	m.searchState.IsActive = true

	m.searchState.Query = "limit4 "
	m.searchState.ExecuteSearch(m.changes)

	if got := m.getLineContentMatches("big.go", 0, 4); len(got) != 1 || got[0].Start != 7 {
		t.Fatalf("hits on the line holding limit4 = %v, want one at byte 7", got)
	}

	m.searchState.Query = "handle"
	m.searchState.ExecuteSearch(m.changes)

	if got := m.getLineContentMatches("big.go", 0, 4); len(got) != 0 {
		t.Errorf("after a new query the old line still has hits %v", got)
	}

	if got := m.getLineContentMatches("big.go", 0, 2); len(got) != 1 {
		t.Errorf("hits on a handle line = %v, want one", got)
	}

	if got := len(m.matchIndex().hits["big.go"]); got != 1000 {
		t.Errorf("the track has %d hits in big.go, want the 1000 handle lines", got)
	}
}

// BenchmarkModelStartup is the in-process half of opening jj-diff on a 200-file change whose first
// file runs to 2000 rows: parse the diff jj printed, load it, and draw the first frame. The first
// file is lexed in the background, so its colors land on a later frame and are not waited for here.
func BenchmarkModelStartup(b *testing.B) {
	var text strings.Builder

	for _, file := range benchDiff(200) {
		fmt.Fprintf(&text, "diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n", file.Path, file.Path, file.Path, file.Path)

		for _, hunk := range file.Hunks {
			text.WriteString(hunk.Header + "\n")

			for _, line := range hunk.Lines {
				text.WriteString(line.Type.String() + line.Content + "\n")
			}
		}
	}

	raw := text.String()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		m := NewTestModel(b, ModeBrowse)
		m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
		m.Update(diffLoadedMsg{changes: diff.Parse(raw)})
		_ = m.render()

		b.StopTimer()
		<-m.diffView.Highlighted()
		b.StartTimer()
	}
}

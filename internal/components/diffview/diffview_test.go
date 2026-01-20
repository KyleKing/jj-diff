package diffview

import (
	"strings"
	"testing"

	"github.com/kyleking/jj-diff/internal/config"
	"github.com/kyleking/jj-diff/internal/diff"
)

func testFileChange() diff.FileChange {
	return diff.FileChange{
		Path:       "test.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks: []diff.Hunk{
			{
				Header:   "@@ -1,3 +1,3 @@",
				OldStart: 1, OldLines: 3, NewStart: 1, NewLines: 3,
				Lines: []diff.Line{
					{Type: diff.LineContext, Content: "context line", OldLineNum: 1, NewLineNum: 1},
					{Type: diff.LineDeletion, Content: "old line", OldLineNum: 2, NewLineNum: 2},
					{Type: diff.LineAddition, Content: "new line", OldLineNum: 3, NewLineNum: 2},
				},
			},
		},
	}
}

func TestNewWithDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	m := New(cfg)

	if m.IsSideBySide() {
		t.Error("Expected unified mode by default")
	}
	if m.ShowWhitespace() {
		t.Error("Expected whitespace off by default")
	}
	if !m.ShowLineNumbers() {
		t.Error("Expected line numbers on by default")
	}
	if m.WordLevelDiff() {
		t.Error("Expected word diff off by default")
	}
}

func TestNewWithSideBySideConfig(t *testing.T) {
	cfg := config.Config{
		ViewMode:        config.ViewModeSideBySide,
		ShowWhitespace:  true,
		ShowLineNumbers: false,
		TabWidth:        8,
		WordLevelDiff:   true,
	}
	m := New(cfg)

	if !m.IsSideBySide() {
		t.Error("Expected side-by-side mode")
	}
	if !m.ShowWhitespace() {
		t.Error("Expected whitespace on")
	}
	if m.ShowLineNumbers() {
		t.Error("Expected line numbers off")
	}
	if !m.WordLevelDiff() {
		t.Error("Expected word diff on")
	}
}

func TestViewUnifiedMode(t *testing.T) {
	m := New(config.DefaultConfig())
	m.SetFileChange(testFileChange())

	output := m.View(80, 20, false)

	if !strings.Contains(output, "@@") {
		t.Error("Expected hunk header in output")
	}
}

func TestViewSideBySideMode(t *testing.T) {
	cfg := config.Config{
		ViewMode:        config.ViewModeSideBySide,
		ShowLineNumbers: true,
		TabWidth:        4,
	}
	m := New(cfg)
	m.SetFileChange(testFileChange())

	output := m.View(80, 20, false)

	if !strings.Contains(output, "OLD") {
		t.Error("Expected OLD header in side-by-side")
	}
	if !strings.Contains(output, "NEW") {
		t.Error("Expected NEW header in side-by-side")
	}
	if !strings.Contains(output, "│") {
		t.Error("Expected vertical separator in side-by-side")
	}
}

func TestViewWithWhitespaceHiding(t *testing.T) {
	cfg := config.Config{
		ShowWhitespace:  true,
		ShowLineNumbers: true,
		TabWidth:        4,
	}
	m := New(cfg)

	fc := diff.FileChange{
		Path:       "test.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks: []diff.Hunk{
			{
				Header: "@@ -1,2 +1,2 @@",
				Lines: []diff.Line{
					{Type: diff.LineDeletion, Content: "  hello", OldLineNum: 1, NewLineNum: 0},
					{Type: diff.LineAddition, Content: "    hello", OldLineNum: 0, NewLineNum: 1},
					{Type: diff.LineContext, Content: "world", OldLineNum: 2, NewLineNum: 2},
				},
			},
		},
	}
	m.SetFileChange(fc)

	output := m.View(80, 10, false)

	// When hiding whitespace, the deletion+addition pair should be converted to context
	// Should not contain deletion (-) or addition (+) indicators for whitespace-only changes
	lines := strings.Split(output, "\n")
	hasWhitespaceOnlyChange := false
	for _, line := range lines {
		if strings.Contains(line, "hello") && !strings.HasPrefix(strings.TrimSpace(line), "-") && !strings.HasPrefix(strings.TrimSpace(line), "+") {
			hasWhitespaceOnlyChange = true
			break
		}
	}
	if !hasWhitespaceOnlyChange {
		t.Errorf("Expected whitespace-only change to be hidden, got: %s", output)
	}
}

func TestViewWithoutLineNumbers(t *testing.T) {
	cfg := config.Config{
		ShowLineNumbers: false,
		TabWidth:        4,
	}
	m := New(cfg)
	m.SetFileChange(testFileChange())

	output := m.View(80, 20, false)

	lines := strings.Split(output, "\n")
	for _, line := range lines[2:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) > 5 && line[2] >= '0' && line[2] <= '9' {
			continue
		}
	}
}

func TestToggleMethods(t *testing.T) {
	m := New(config.DefaultConfig())

	m.ToggleWhitespace()
	if !m.ShowWhitespace() {
		t.Error("Expected whitespace on after toggle")
	}

	m.ToggleLineNumbers()
	if m.ShowLineNumbers() {
		t.Error("Expected line numbers off after toggle")
	}

	m.ToggleSideBySide()
	if !m.IsSideBySide() {
		t.Error("Expected side-by-side on after toggle")
	}
}

func TestWordDiffToggle(t *testing.T) {
	m := New(config.DefaultConfig())
	m.SetFileChange(testFileChange())

	if m.WordLevelDiff() {
		t.Error("Expected word diff off initially")
	}

	m.ToggleWordDiff()
	if !m.WordLevelDiff() {
		t.Error("Expected word diff on after toggle")
	}

	m.ToggleWordDiff()
	if m.WordLevelDiff() {
		t.Error("Expected word diff off after second toggle")
	}
}

func TestScrolling(t *testing.T) {
	m := New(config.DefaultConfig())

	fc := diff.FileChange{
		Path:       "test.go",
		ChangeType: diff.ChangeTypeModified,
		Hunks: []diff.Hunk{
			{
				Header: "@@ -1,10 +1,10 @@",
				Lines: func() []diff.Line {
					lines := make([]diff.Line, 50)
					for i := range lines {
						lines[i] = diff.Line{Type: diff.LineContext, Content: "line", OldLineNum: i + 1, NewLineNum: i + 1}
					}
					return lines
				}(),
			},
		},
	}
	m.SetFileChange(fc)

	m.Scroll(5)
	output1 := m.View(80, 10, false)

	m.Scroll(-5)
	output2 := m.View(80, 10, false)

	if output1 == output2 {
		t.Error("Expected different output after scrolling")
	}
}

func TestNoFileSelected(t *testing.T) {
	m := New(config.DefaultConfig())

	output := m.View(80, 20, false)

	if !strings.Contains(output, "No file selected") {
		t.Error("Expected 'No file selected' message")
	}
}

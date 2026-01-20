package highlight

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	h := New()
	if h == nil {
		t.Fatal("Expected highlighter to be created")
	}
	if h.style == nil {
		t.Error("Expected style to be initialized")
	}
}

func TestDetectLexer(t *testing.T) {
	h := New()

	testCases := []struct {
		filePath     string
		shouldDetect bool
	}{
		{"main.go", true},
		{"script.py", true},
		{"index.js", true},
		{"app.tsx", true},
		{"README.md", true},
		{"config.json", true},
		{"style.css", true},
		{"unknown.xyz", false},
		{"", false},
	}

	for _, tc := range testCases {
		lexer := h.detectLexer(tc.filePath)
		detected := lexer != nil

		if detected != tc.shouldDetect {
			t.Errorf("File %s: expected detected=%v, got %v", tc.filePath, tc.shouldDetect, detected)
		}
	}
}

func TestHighlightLine_Go(t *testing.T) {
	h := New()

	line := "func main() {"
	result := h.HighlightLine("main.go", line)

	// Result should not be empty
	if result == "" {
		t.Error("Expected non-empty result for Go code")
	}

	// Result should contain styling (ANSI codes or lipgloss styling)
	// At minimum, it should contain the original text
	if !strings.Contains(result, "func") || !strings.Contains(result, "main") {
		t.Errorf("Expected result to contain original text, got: %s", result)
	}
}

func TestHighlightLine_Python(t *testing.T) {
	h := New()

	line := "def hello_world():"
	result := h.HighlightLine("script.py", line)

	if result == "" {
		t.Error("Expected non-empty result for Python code")
	}

	if !strings.Contains(result, "def") || !strings.Contains(result, "hello_world") {
		t.Errorf("Expected result to contain original text, got: %s", result)
	}
}

func TestHighlightLine_EmptyLine(t *testing.T) {
	h := New()

	result := h.HighlightLine("main.go", "")

	if result != "" {
		t.Errorf("Expected empty result for empty line, got: %s", result)
	}
}

func TestHighlightLine_UnknownLanguage(t *testing.T) {
	h := New()

	line := "some random text"
	result := h.HighlightLine("unknown.xyz", line)

	// Should return original line unchanged when language is unknown
	if result != line {
		t.Errorf("Expected unchanged line for unknown language, got: %s", result)
	}
}

func TestIsEnabled(t *testing.T) {
	h := New()

	testCases := []struct {
		filePath string
		enabled  bool
	}{
		{"main.go", true},
		{"script.py", true},
		{"unknown.xyz", false},
		{"", false},
	}

	for _, tc := range testCases {
		enabled := h.IsEnabled(tc.filePath)
		if enabled != tc.enabled {
			t.Errorf("File %s: expected enabled=%v, got %v", tc.filePath, tc.enabled, enabled)
		}
	}
}

func TestHighlightLine_PreservesContent(t *testing.T) {
	h := New()

	testCases := []struct {
		filePath string
		line     string
	}{
		{"main.go", "package main"},
		{"script.py", "import os"},
		{"index.js", "const x = 42;"},
	}

	for _, tc := range testCases {
		result := h.HighlightLine(tc.filePath, tc.line)

		// Check that key words from the original line are preserved
		words := strings.Fields(tc.line)
		for _, word := range words {
			// Remove any non-alphanumeric characters for comparison
			cleanWord := strings.Trim(word, "(){}[];,.")
			if cleanWord != "" && !strings.Contains(result, cleanWord) {
				t.Errorf("File %s, line %s: result missing word %s", tc.filePath, tc.line, cleanWord)
			}
		}
	}
}

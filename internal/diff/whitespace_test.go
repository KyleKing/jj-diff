package diff_test

import (
	"strings"
	"testing"

	"github.com/kyleking/jj-diff/internal/diff"
)

func TestRenderWhitespaceSimple(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		contains []string
		tabWidth int
	}{
		{
			name:     "spaces become dots",
			input:    "hello world",
			tabWidth: 4,
			contains: []string{"hello", string(diff.SpaceChar), "world"},
		},
		{
			name:     "tabs become arrows",
			input:    "hello\tworld",
			tabWidth: 4,
			contains: []string{"hello", string(diff.TabChar), "world"},
		},
		{
			name:     "empty string",
			input:    "",
			tabWidth: 4,
			contains: []string{},
		},
		{
			name:     "no whitespace unchanged",
			input:    "hello",
			tabWidth: 4,
			contains: []string{"hello"},
		},
		{
			name:     "multiple spaces",
			input:    "a  b",
			tabWidth: 4,
			contains: []string{"a", string(diff.SpaceChar), string(diff.SpaceChar), "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := diff.RenderWhitespaceSimple(tt.input, tt.tabWidth)
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain %q, got %q", expected, result)
				}
			}
		})
	}
}

func TestRenderWhitespaceTabPadding(t *testing.T) {
	t.Parallel()

	result := diff.RenderWhitespaceSimple("\t", 4)
	runes := []rune(result)
	if len(runes) != 4 {
		t.Errorf("Expected tab to expand to 4 runes, got %d: %q", len(runes), result)
	}
	if runes[0] != diff.TabChar {
		t.Errorf("Expected first rune to be TabChar, got %q", runes[0])
	}

	result = diff.RenderWhitespaceSimple("\t", 8)
	runes = []rune(result)
	if len(runes) != 8 {
		t.Errorf("Expected tab to expand to 8 runes, got %d: %q", len(runes), result)
	}
}

func TestHasTrailingWhitespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected bool
	}{
		{"hello ", true},
		{"hello\t", true},
		{"hello", false},
		{"", false},
		{" hello", false},
		{"hello  ", true},
	}

	for _, tt := range tests {
		if got := diff.HasTrailingWhitespace(tt.input); got != tt.expected {
			t.Errorf("HasTrailingWhitespace(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestCountTrailingWhitespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected int
	}{
		{"hello   ", 3},
		{"hello\t", 1},
		{"hello", 0},
		{"", 0},
		{"  ", 2},
		{"hello \t ", 3},
	}

	for _, tt := range tests {
		if got := diff.CountTrailingWhitespace(tt.input); got != tt.expected {
			t.Errorf("CountTrailingWhitespace(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

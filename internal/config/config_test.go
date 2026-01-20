package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ViewMode != ViewModeUnified {
		t.Errorf("Expected ViewModeUnified, got %s", cfg.ViewMode)
	}
	if cfg.ShowWhitespace {
		t.Error("Expected ShowWhitespace=false")
	}
	if !cfg.ShowLineNumbers {
		t.Error("Expected ShowLineNumbers=true")
	}
	if cfg.TabWidth != 4 {
		t.Errorf("Expected TabWidth=4, got %d", cfg.TabWidth)
	}
	if cfg.WordLevelDiff {
		t.Error("Expected WordLevelDiff=false")
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		checkFn  func(Config) bool
		expected bool
	}{
		{
			name:     "side-by-side mode",
			envVars:  map[string]string{"JJ_DIFF_VIEW_MODE": "side-by-side"},
			checkFn:  func(c Config) bool { return c.ViewMode == ViewModeSideBySide },
			expected: true,
		},
		{
			name:     "unified mode explicit",
			envVars:  map[string]string{"JJ_DIFF_VIEW_MODE": "unified"},
			checkFn:  func(c Config) bool { return c.ViewMode == ViewModeUnified },
			expected: true,
		},
		{
			name:     "show whitespace true",
			envVars:  map[string]string{"JJ_DIFF_SHOW_WHITESPACE": "true"},
			checkFn:  func(c Config) bool { return c.ShowWhitespace },
			expected: true,
		},
		{
			name:     "show whitespace 1",
			envVars:  map[string]string{"JJ_DIFF_SHOW_WHITESPACE": "1"},
			checkFn:  func(c Config) bool { return c.ShowWhitespace },
			expected: true,
		},
		{
			name:     "show line numbers false",
			envVars:  map[string]string{"JJ_DIFF_SHOW_LINE_NUMBERS": "false"},
			checkFn:  func(c Config) bool { return c.ShowLineNumbers },
			expected: false,
		},
		{
			name:     "tab width 8",
			envVars:  map[string]string{"JJ_DIFF_TAB_WIDTH": "8"},
			checkFn:  func(c Config) bool { return c.TabWidth == 8 },
			expected: true,
		},
		{
			name:     "tab width invalid stays default",
			envVars:  map[string]string{"JJ_DIFF_TAB_WIDTH": "invalid"},
			checkFn:  func(c Config) bool { return c.TabWidth == 4 },
			expected: true,
		},
		{
			name:     "tab width too large stays default",
			envVars:  map[string]string{"JJ_DIFF_TAB_WIDTH": "100"},
			checkFn:  func(c Config) bool { return c.TabWidth == 4 },
			expected: true,
		},
		{
			name:     "word diff on",
			envVars:  map[string]string{"JJ_DIFF_WORD_DIFF": "yes"},
			checkFn:  func(c Config) bool { return c.WordLevelDiff },
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			cfg := LoadConfig()
			if got := tt.checkFn(cfg); got != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestToRenderOptions(t *testing.T) {
	cfg := Config{
		ViewMode:        ViewModeSideBySide,
		ShowWhitespace:  true,
		ShowLineNumbers: false,
		TabWidth:        8,
		WordLevelDiff:   true,
	}

	opts := cfg.ToRenderOptions()

	if !opts.ShowWhitespace {
		t.Error("Expected ShowWhitespace=true")
	}
	if opts.ShowLineNumbers {
		t.Error("Expected ShowLineNumbers=false")
	}
	if opts.TabWidth != 8 {
		t.Errorf("Expected TabWidth=8, got %d", opts.TabWidth)
	}
	if !opts.WordLevelDiff {
		t.Error("Expected WordLevelDiff=true")
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1", true},
		{"true", true},
		{"yes", true},
		{"on", true},
		{"0", false},
		{"false", false},
		{"no", false},
		{"off", false},
		{"anything", false},
	}

	for _, tt := range tests {
		if got := parseBool(tt.input); got != tt.expected {
			t.Errorf("parseBool(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

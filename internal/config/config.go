// Package config resolves the diff render settings for a session, layering the
// JJ_DIFF_* environment variables over a set of built-in defaults.
package config

import (
	"os"
	"strconv"
)

// ViewModeType selects how the two sides of a diff are laid out.
type ViewModeType string

// Diff layouts. The string values are also what JJ_DIFF_VIEW_MODE accepts,
// which is why they are spelled as user-facing words rather than as an int enum.
const (
	ViewModeUnified    ViewModeType = "unified"
	ViewModeSideBySide ViewModeType = "side-by-side"
)

// Config is the fully resolved settings for a session. Every field has a usable
// zero-value replacement from DefaultConfig, so callers never build one by hand.
type Config struct {
	ViewMode        ViewModeType
	ShowWhitespace  bool
	ShowLineNumbers bool
	TabWidth        int
	WordLevelDiff   bool
}

// DefaultConfig returns the settings that apply when no environment variable is
// set: unified layout, line numbers on, whitespace and word-level diff off, tabs
// four columns wide.
func DefaultConfig() Config {
	return Config{
		ViewMode:        ViewModeUnified,
		ShowWhitespace:  false,
		ShowLineNumbers: true,
		TabWidth:        4,
		WordLevelDiff:   false,
	}
}

// LoadConfig reads the JJ_DIFF_* environment variables over DefaultConfig. A
// value that cannot be understood is ignored rather than reported, so the
// default survives and startup never fails on a typo. Booleans are true only for
// "1", "true", "yes", or "on", and JJ_DIFF_TAB_WIDTH is honored for 1 to 16.
func LoadConfig() Config {
	cfg := DefaultConfig()

	if v := os.Getenv("JJ_DIFF_VIEW_MODE"); v != "" {
		switch v {
		case "side-by-side", "sidebyside":
			cfg.ViewMode = ViewModeSideBySide
		case "unified":
			cfg.ViewMode = ViewModeUnified
		}
	}

	if v := os.Getenv("JJ_DIFF_SHOW_WHITESPACE"); v != "" {
		cfg.ShowWhitespace = parseBool(v)
	}

	if v := os.Getenv("JJ_DIFF_SHOW_LINE_NUMBERS"); v != "" {
		cfg.ShowLineNumbers = parseBool(v)
	}

	if v := os.Getenv("JJ_DIFF_TAB_WIDTH"); v != "" {
		if width, err := strconv.Atoi(v); err == nil && width > 0 && width <= 16 {
			cfg.TabWidth = width
		}
	}

	if v := os.Getenv("JJ_DIFF_WORD_DIFF"); v != "" {
		cfg.WordLevelDiff = parseBool(v)
	}

	return cfg
}

func parseBool(s string) bool {
	switch s {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// RenderOptions carries the per-line rendering settings without the layout
// choice, so a renderer can be handed only what affects a single line of output.
type RenderOptions struct {
	ShowWhitespace  bool
	ShowLineNumbers bool
	TabWidth        int
	WordLevelDiff   bool
}

func (c Config) ToRenderOptions() RenderOptions {
	return RenderOptions{
		ShowWhitespace:  c.ShowWhitespace,
		ShowLineNumbers: c.ShowLineNumbers,
		TabWidth:        c.TabWidth,
		WordLevelDiff:   c.WordLevelDiff,
	}
}

package config

import (
	"os"
	"strconv"
)

type ViewModeType string

const (
	ViewModeUnified    ViewModeType = "unified"
	ViewModeSideBySide ViewModeType = "side-by-side"
)

type Config struct {
	ViewMode        ViewModeType
	ShowWhitespace  bool
	ShowLineNumbers bool
	TabWidth        int
	WordLevelDiff   bool
}

func DefaultConfig() Config {
	return Config{
		ViewMode:        ViewModeUnified,
		ShowWhitespace:  false,
		ShowLineNumbers: true,
		TabWidth:        4,
		WordLevelDiff:   false,
	}
}

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

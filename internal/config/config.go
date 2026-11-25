// Package config resolves the diff render settings for a session, layering the
// JJ_DIFF_* environment variables over a set of built-in defaults.
package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/kyleking/aragonite/tui/coach"
	"github.com/kyleking/aragonite/tui/icons"
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
	ViewMode ViewModeType
	// Icons names the icon set file rows are drawn with, as JJ_DIFF_ICONS spells it: "auto" or
	// empty (the default) for Nerd Font icons in a terminal that bundles them and unicode
	// elsewhere, "nerd", "unicode" (or "plain"), or "none" (or "off"). ExplainIcons resolves it.
	Icons string
	// CoachLayout is where the next step shows: the footer's status slot (Inline, the default), or
	// a labeled band above the footer, which JJ_DIFF_COACH_LAYOUT=band asks for.
	CoachLayout     coach.Layout
	TabWidth        int
	ShowWhitespace  bool
	ShowLineNumbers bool
	WordLevelDiff   bool
}

// defaultTabWidth is the column width a tab renders as when JJ_DIFF_TAB_WIDTH is unset.
const defaultTabWidth = 4

// DefaultConfig returns the settings that apply when no environment variable is
// set: unified layout, line numbers on, whitespace and word-level diff off, tabs
// four columns wide.
func DefaultConfig() Config {
	return Config{
		ViewMode:        ViewModeUnified,
		ShowWhitespace:  false,
		ShowLineNumbers: true,
		TabWidth:        defaultTabWidth,
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

	if layout, err := coach.ParseLayout(os.Getenv("JJ_DIFF_COACH_LAYOUT")); err == nil {
		cfg.CoachLayout = layout
	}

	cfg.Icons = os.Getenv(IconsEnv)

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
	TabWidth        int
	ShowWhitespace  bool
	ShowLineNumbers bool
	WordLevelDiff   bool
}

// ToRenderOptions drops the layout choice and keeps the fields that affect a single rendered line.
func (c Config) ToRenderOptions() RenderOptions {
	return RenderOptions{
		ShowWhitespace:  c.ShowWhitespace,
		ShowLineNumbers: c.ShowLineNumbers,
		TabWidth:        c.TabWidth,
		WordLevelDiff:   c.WordLevelDiff,
	}
}

// IconsEnv names the variable that picks jj-diff's icon set, ahead of aragonite's family-wide
// icons.EnvOverride (ARAGONITE_ICONS).
const IconsEnv = "JJ_DIFF_ICONS"

// ExplainIcons resolves the icon set from setting, JJ_DIFF_ICONS's value, and says which input
// decided it. The more specific wins: JJ_DIFF_ICONS, then ARAGONITE_ICONS, then the terminal
// (icons.Explain's Auto). A setting that spells auto, or that icons.Parse cannot read, defers to
// the next input, as icons.Explain does. A Resolution from FromSetting is JJ_DIFF_ICONS's.
func ExplainIcons(setting string) icons.Resolution {
	if m, ok := icons.Parse(setting); ok && m != icons.Auto {
		return icons.Resolution{Mode: m, Source: icons.FromSetting, Detail: strings.TrimSpace(setting)}
	}

	return icons.Explain("")
}

// SayIcons is r in a few words, naming the variable that decided it: "nerd (auto: WezTerm)",
// "none (ARAGONITE_ICONS=off)", or "plain" set through JJ_DIFF_ICONS as
// "unicode (JJ_DIFF_ICONS=plain)", where icons.Resolution.String would say only "(setting)".
func SayIcons(r icons.Resolution) string {
	if r.Source == icons.FromSetting {
		return r.Mode.String() + " (" + IconsEnv + "=" + r.Detail + ")"
	}

	return r.String()
}

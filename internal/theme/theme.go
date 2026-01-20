package theme

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme defines semantic color roles
type Theme struct {
	Primary         lipgloss.Color
	Accent          lipgloss.Color
	Secondary       lipgloss.Color
	Text            lipgloss.Color
	SelectedBg      lipgloss.Color
	MutedBg         lipgloss.Color
	SoftMutedBg     lipgloss.Color
	ModalBg         lipgloss.Color
	AddedLine       lipgloss.Color
	DeletedLine     lipgloss.Color
	WordDiffAddedBg lipgloss.Color
	WordDiffDelBg   lipgloss.Color
}

// Latte returns Catppuccin Latte (light theme)
func Latte() Theme {
	return Theme{
		Primary:         lipgloss.Color("#8839ef"), // mauve
		Accent:          lipgloss.Color("#179299"), // teal
		Secondary:       lipgloss.Color("#df8e1d"), // yellow
		Text:            lipgloss.Color("#4c4f69"),
		SelectedBg:      lipgloss.Color("#7287fd"), // lavender
		MutedBg:         lipgloss.Color("#dce0e8"), // surface0
		SoftMutedBg:     lipgloss.Color("#ccd0da"), // surface1
		ModalBg:         lipgloss.Color("#eff1f5"), // base
		AddedLine:       lipgloss.Color("#40a02b"), // green
		DeletedLine:     lipgloss.Color("#d20f39"), // red
		WordDiffAddedBg: lipgloss.Color("#acf2bd"), // light green bg
		WordDiffDelBg:   lipgloss.Color("#ffc0c0"), // light red bg
	}
}

// Macchiato returns Catppuccin Macchiato (dark theme)
func Macchiato() Theme {
	return Theme{
		Primary:         lipgloss.Color("#c6a0f6"), // mauve
		Accent:          lipgloss.Color("#8bd5ca"), // teal
		Secondary:       lipgloss.Color("#f5a97f"), // peach
		Text:            lipgloss.Color("#cad3f5"),
		SelectedBg:      lipgloss.Color("#b7bdf8"), // lavender
		MutedBg:         lipgloss.Color("#363a4f"), // surface0
		SoftMutedBg:     lipgloss.Color("#494d64"), // surface1
		ModalBg:         lipgloss.Color("#24273a"), // base
		AddedLine:       lipgloss.Color("#a6da95"), // green
		DeletedLine:     lipgloss.Color("#ed8796"), // red
		WordDiffAddedBg: lipgloss.Color("#2d4a3e"), // dark green bg
		WordDiffDelBg:   lipgloss.Color("#4a2d2d"), // dark red bg
	}
}

// Detect returns the appropriate theme based on environment
func Detect() Theme {
	if env := os.Getenv("CATPPUCCIN_THEME"); env != "" {
		switch strings.ToLower(env) {
		case "latte", "light":
			return Latte()
		case "macchiato", "dark":
			return Macchiato()
		}
	}

	if lipgloss.HasDarkBackground() {
		return Macchiato()
	}
	return Latte()
}

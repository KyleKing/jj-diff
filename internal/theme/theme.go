package theme

import (
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
)

// Theme defines semantic color roles.
type Theme struct {
	Primary     color.Color
	Accent      color.Color
	Secondary   color.Color
	Text        color.Color
	AddedLine   color.Color
	DeletedLine color.Color
	Dark        bool
}

// Latte returns Catppuccin Latte (light theme).
func Latte() Theme {
	return Theme{
		Dark:        false,
		Primary:     lipgloss.Color("#8839ef"), // mauve
		Accent:      lipgloss.Color("#179299"), // teal
		Secondary:   lipgloss.Color("#df8e1d"), // yellow
		Text:        lipgloss.Color("#4c4f69"),
		AddedLine:   lipgloss.Color("#40a02b"), // green
		DeletedLine: lipgloss.Color("#d20f39"), // red
	}
}

// Macchiato returns Catppuccin Macchiato (dark theme).
func Macchiato() Theme {
	return Theme{
		Dark:        true,
		Primary:     lipgloss.Color("#c6a0f6"), // mauve
		Accent:      lipgloss.Color("#8bd5ca"), // teal
		Secondary:   lipgloss.Color("#f5a97f"), // peach
		Text:        lipgloss.Color("#cad3f5"),
		AddedLine:   lipgloss.Color("#a6da95"), // green
		DeletedLine: lipgloss.Color("#ed8796"), // red
	}
}

// For is the theme for a terminal whose background is dark or not, unless CATPPUCCIN_THEME names a
// flavor. It queries nothing: the program learns the background through its own input
// (aragonite tui/launch), because a query made before the program runs reads the keys typed
// meanwhile and throws them away with its reply.
func For(dark bool) Theme {
	if t, ok := pinnedTheme(); ok {
		return t
	}

	if dark {
		return Macchiato()
	}

	return Latte()
}

// Pinned reports whether CATPPUCCIN_THEME names a flavor, in which case the terminal's background
// is never needed.
func Pinned() bool {
	_, ok := pinnedTheme()

	return ok
}

func pinnedTheme() (Theme, bool) {
	switch strings.ToLower(os.Getenv("CATPPUCCIN_THEME")) {
	case "latte", "light":
		return Latte(), true
	case "macchiato", "dark":
		return Macchiato(), true
	}

	return Theme{}, false
}

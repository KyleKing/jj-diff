package theme

import "github.com/charmbracelet/colorprofile"

// ApplyFor initializes the theme as a terminal of depth p would show it.
func ApplyFor(t *Theme, p colorprofile.Profile) {
	applyTheme(t, p)
}

//nolint:testpackage // TestMain covers the white-box and black-box tests of this package alike.
package model

import (
	"os"
	"testing"

	"github.com/kyleking/jj-diff/internal/theme"
)

// The theme is process-global, so it is pinned once here: Macchiato, and the plain unicode icon
// set, which draws no file glyph, so a reader's own ARAGONITE_ICONS cannot change a golden. Without
// Init the zero icon set is Nerd's.
func TestMain(m *testing.M) {
	for key, value := range map[string]string{"CATPPUCCIN_THEME": "macchiato", "ARAGONITE_ICONS": "unicode"} {
		if err := os.Setenv(key, value); err != nil {
			panic(err)
		}
	}

	theme.Init()
	os.Exit(m.Run())
}

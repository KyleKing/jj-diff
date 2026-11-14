package theme_test

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/colorprofile"

	"github.com/kyleking/jj-diff/internal/theme"
)

// A faint tint reduced to 256 colors is one grey whatever its hue.
//
//nolint:paralleltest // ApplyFor writes the package-level colors every test in this package reads.
func TestDiffTintsStayDistinctOnReducedColorTerminals(t *testing.T) {
	latte, macchiato := theme.Latte(), theme.Macchiato()

	for _, flavor := range []struct {
		theme *theme.Theme
		name  string
	}{{&latte, "latte"}, {&macchiato, "macchiato"}} {
		for _, profile := range []colorprofile.Profile{colorprofile.TrueColor, colorprofile.ANSI256} {
			t.Run(fmt.Sprintf("%s %s", flavor.name, profile), func(t *testing.T) {
				theme.ApplyFor(flavor.theme, profile)

				seen := map[any]string{}

				for name, c := range map[string]any{
					"added":        profile.Convert(theme.AddedBg),
					"deleted":      profile.Convert(theme.DeletedBg),
					"word added":   profile.Convert(theme.WordDiffAddedBg),
					"word deleted": profile.Convert(theme.WordDiffDelBg),
				} {
					if other, dup := seen[c]; dup {
						t.Errorf("%s and %s both show as %v", name, other, c)
					}

					seen[c] = name
				}
			})
		}
	}
}

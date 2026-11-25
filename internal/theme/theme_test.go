package theme_test

import (
	"fmt"
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/kyleking/jj-diff/internal/theme"
)

func flavors() map[string]theme.Theme {
	return map[string]theme.Theme{"latte": theme.Latte(), "macchiato": theme.Macchiato()}
}

// isGrey reports whether a 256-color background is a step of the grey ramp (232-255) or the
// cube's black or white, never one of the cube's saturated greens, reds, or navies.
func isGrey(c color.Color) bool {
	r, g, b, _ := c.RGBA()

	return r == g && g == b
}

// In truecolor each side carries its own hue, so all four backgrounds differ. In 256 colors
// both sides wash in one grey and mark in a deeper one, since the cube's nearest greens and reds
// (22, 52) are saturated blocks; the side is then said by the sign, so the signs must differ and
// each mark must stand off its wash. In 16 colors nothing is painted behind a line.
//
//nolint:paralleltest // ApplyFor writes the package-level colors every test in this package reads.
func TestDiffBackgroundsFitTheColorDepth(t *testing.T) {
	for name, flavor := range flavors() {
		for _, profile := range []colorprofile.Profile{
			colorprofile.TrueColor, colorprofile.ANSI256, colorprofile.ANSI,
		} {
			t.Run(fmt.Sprintf("%s %s", name, profile), func(t *testing.T) {
				theme.ApplyFor(&flavor, profile)

				backgrounds := map[string]color.Color{
					"added wash": theme.AddedBg, "removed wash": theme.DeletedBg,
					"added mark": theme.WordDiffAddedBg, "removed mark": theme.WordDiffDelBg,
				}

				if profile == colorprofile.ANSI {
					requireNone(t, backgrounds)

					return
				}

				if profile == colorprofile.ANSI256 {
					requireGreys(t, profile, backgrounds)
				}

				requireApart(t, profile, backgrounds)
			})
		}
	}
}

func requireNone(t *testing.T, backgrounds map[string]color.Color) {
	t.Helper()

	for face, c := range backgrounds {
		if _, none := c.(lipgloss.NoColor); !none {
			t.Errorf("%s paints %v in 16 colors", face, c)
		}
	}
}

func requireGreys(t *testing.T, profile colorprofile.Profile, backgrounds map[string]color.Color) {
	t.Helper()

	for face, c := range backgrounds {
		if got := profile.Convert(c); !isGrey(got) {
			t.Errorf("%s is %v in 256 colors, not a grey", face, got)
		}
	}
}

// requireApart holds each mark off its wash, both sides apart in truecolor, and the signs apart
// at every depth.
func requireApart(t *testing.T, profile colorprofile.Profile, backgrounds map[string]color.Color) {
	t.Helper()

	painted := func(c color.Color) string { return fmt.Sprint(profile.Convert(c)) }

	pairs := [][2]string{{"added wash", "added mark"}, {"removed wash", "removed mark"}}
	if profile == colorprofile.TrueColor {
		pairs = append(pairs,
			[2]string{"added wash", "removed wash"}, [2]string{"added mark", "removed mark"})
	}

	for _, pair := range pairs {
		if painted(backgrounds[pair[0]]) == painted(backgrounds[pair[1]]) {
			t.Errorf("%s and %s both show as %s", pair[0], pair[1], painted(backgrounds[pair[0]]))
		}
	}

	if painted(theme.AddedLine) == painted(theme.DeletedLine) || isGrey(profile.Convert(theme.DeletedLine)) {
		t.Errorf("the sides' colors show as %s and %s", painted(theme.AddedLine), painted(theme.DeletedLine))
	}

	addSign := painted(theme.Diff.AddSign.GetForeground())
	removeSign := theme.Diff.RemoveSign.GetForeground()
	if addSign == painted(removeSign) || isGrey(profile.Convert(removeSign)) {
		t.Errorf("the signs show as %s and %s", addSign, painted(removeSign))
	}
}

// Inside tmux, TERM names tmux and colorprofile asks `tmux info`, which a server without the RGB
// feature answers as 256 colors on a terminal that has millions. COLORTERM=truecolor is the
// outer terminal's word and wins, so the wash is the tint and not a grey.
func TestProfileHonorsTruecolorInsideTmux(t *testing.T) {
	t.Setenv("TERM", "tmux-256color")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("TMUX", "/tmp/tmux-0/default,1,0")
	t.Setenv("TTY_FORCE", "1")
	t.Setenv("NO_COLOR", "")

	if got := theme.Profile(); got != colorprofile.TrueColor {
		t.Fatalf("Profile() = %v, want TrueColor", got)
	}

	macchiato := theme.Macchiato()
	theme.ApplyFor(&macchiato, theme.Profile())

	if wash := theme.AddedBg; isGrey(wash) {
		t.Errorf("added wash is the grey %v, want the truecolor tint", wash)
	}
}

// TestFor_HonorsTheOverrideAndQueriesNothing pins the pair main starts with: Pinned says whether
// the background is needed at all, and For takes the answer the program learned through its own
// input rather than asking the terminal. It sets CATPPUCCIN_THEME, which the process shares, so it
// runs alone.
func TestFor_HonorsTheOverrideAndQueriesNothing(t *testing.T) {
	for _, tc := range []struct {
		dark       theme.Theme
		light      theme.Theme
		override   string
		wantPinned bool
	}{
		{theme.Macchiato(), theme.Latte(), "", false},
		{theme.Macchiato(), theme.Latte(), "solarized", false},
		{theme.Latte(), theme.Latte(), "Light", true},
		{theme.Macchiato(), theme.Macchiato(), "macchiato", true},
	} {
		t.Run("override="+tc.override, func(t *testing.T) {
			t.Setenv("CATPPUCCIN_THEME", tc.override)

			if got := theme.Pinned(); got != tc.wantPinned {
				t.Errorf("Pinned() = %t, want %t", got, tc.wantPinned)
			}

			if got := theme.For(true); got != tc.dark {
				t.Errorf("For(dark) = %+v, want %+v", got, tc.dark)
			}

			if got := theme.For(false); got != tc.light {
				t.Errorf("For(light) = %+v, want %+v", got, tc.light)
			}
		})
	}
}

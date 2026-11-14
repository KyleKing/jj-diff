package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	aragonite "github.com/kyleking/aragonite/tui/theme"
)

const (
	lineLift = 0.1
	lineSat  = 0.4
	wordLift = 0.22
	wordSat  = 0.65
)

var (
	lineDepth = aragonite.Depth{Lift: lineLift, Sat: lineSat}
	wordDepth = aragonite.Depth{Lift: wordLift, Sat: wordSat}
)

type tints struct {
	added       color.Color
	deleted     color.Color
	wordAdded   color.Color
	wordDeleted color.Color
}

// diffTints returns the line and word backgrounds for p. A faint tint reduced to 256 colors becomes
// one grey whatever its hue, so that depth gets cube colors, and a 16-color terminal gets none.
func diffTints(base, added, deleted color.Color, dark bool, p colorprofile.Profile) tints {
	switch {
	case p == colorprofile.TrueColor:
		return tints{
			added:       aragonite.Tint(base, added, lineDepth),
			deleted:     aragonite.Tint(base, deleted, lineDepth),
			wordAdded:   aragonite.Tint(base, added, wordDepth),
			wordDeleted: aragonite.Tint(base, deleted, wordDepth),
		}
	case p == colorprofile.ANSI256 && dark:
		return tints{
			added:       lipgloss.ANSIColor(cubeDarkGreen),
			deleted:     lipgloss.ANSIColor(cubeDarkRed),
			wordAdded:   lipgloss.ANSIColor(cubeGreen),
			wordDeleted: lipgloss.ANSIColor(cubeRed),
		}
	case p == colorprofile.ANSI256:
		return tints{
			added:       lipgloss.ANSIColor(cubePaleGreen),
			deleted:     lipgloss.ANSIColor(cubePaleRed),
			wordAdded:   lipgloss.ANSIColor(cubeLightGreen),
			wordDeleted: lipgloss.ANSIColor(cubeLightRed),
		}
	}

	return tints{}
}

const (
	cubeDarkGreen  = 22
	cubeDarkRed    = 52
	cubeGreen      = 28
	cubeRed        = 88
	cubePaleGreen  = 194
	cubePaleRed    = 224
	cubeLightGreen = 157
	cubeLightRed   = 217
)

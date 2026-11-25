package theme

import (
	"github.com/charmbracelet/colorprofile"
	"github.com/kyleking/aragonite/tui/icons"
	"github.com/kyleking/aragonite/tui/skin"
	aragonite "github.com/kyleking/aragonite/tui/theme"

	"github.com/kyleking/jj-diff/internal/config"
)

// The faces shared with second-look and jojo, built from the same Catppuccin flavor as the colors
// above so nothing on screen comes from a palette of its own.
var (
	// Palette is the full Catppuccin flavor the semantic colors above are picked from.
	Palette aragonite.Palette
	// Skin names the faces a screen asks for: titles, muted text, the selection, the file band.
	Skin skin.Skin
	// Diff is what a changed line is drawn with: its wash, the deeper word mark, and the sign bar.
	Diff skin.DiffFaces
	// Icons draws a file's type, in the set config.ExplainIcons picks: JJ_DIFF_ICONS (UseIcons),
	// then ARAGONITE_ICONS, then Nerd Font glyphs in a terminal that bundles them and plain
	// unicode, which draws no file icon, elsewhere.
	Icons icons.Set

	// The configured icon set, kept so a theme rebuilt by Init draws the same one.
	iconSetting string
)

// UseIcons picks the icon set by JJ_DIFF_ICONS's spelling ("auto", "unicode", "nerd", "none"),
// through config.ExplainIcons, and marks every cached frame stale.
func UseIcons(setting string) {
	iconSetting = setting
	Icons = icons.New(config.ExplainIcons(setting).Mode, Palette)

	generation.Add(1)
}

// applySkin builds the shared faces for t and sets the diff tints from them. A 16-color terminal
// gets no tint at all, since the nearest of sixteen colors to a pale wash is a loud one.
func applySkin(t *Theme, p colorprofile.Profile) {
	Palette = aragonite.Latte()
	if t.Dark {
		Palette = aragonite.Macchiato()
	}

	Skin = skin.For(Palette, t.Primary, p)
	Diff = skin.DiffFor(Palette, p)
	Icons = icons.New(config.ExplainIcons(iconSetting).Mode, Palette)

	AddedBg, DeletedBg = Diff.AddWash, Diff.RemoveWash
	WordDiffAddedBg, WordDiffDelBg = Diff.AddMark, Diff.RemoveMark
}

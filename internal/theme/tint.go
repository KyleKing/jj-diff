package theme

import (
	"github.com/charmbracelet/colorprofile"
	"github.com/kyleking/aragonite/tui/icons"
	"github.com/kyleking/aragonite/tui/skin"
	aragonite "github.com/kyleking/aragonite/tui/theme"
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
	// Icons draws a file's type: Nerd Font glyphs unless ARAGONITE_ICONS asks for plain unicode.
	Icons icons.Set
)

// applySkin builds the shared faces for t and sets the diff tints from them. A 16-color terminal
// gets no tint at all, since the nearest of sixteen colors to a pale wash is a loud one.
func applySkin(t *Theme, p colorprofile.Profile) {
	Palette = aragonite.Latte()
	if t.Dark {
		Palette = aragonite.Macchiato()
	}

	Skin = skin.New(Palette, t.Primary)
	Diff = skin.Diff(Palette, p == colorprofile.TrueColor)
	Icons = icons.New(icons.Resolve(""), Palette)

	AddedBg, DeletedBg, WordDiffAddedBg, WordDiffDelBg = nil, nil, nil, nil

	if p >= colorprofile.ANSI256 {
		AddedBg, DeletedBg = Diff.AddWash, Diff.RemoveWash
		WordDiffAddedBg, WordDiffDelBg = Diff.AddMark, Diff.RemoveMark
	}
}

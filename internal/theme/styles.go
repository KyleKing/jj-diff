// Package theme holds the Catppuccin palette as package-level colors that every component reads
// directly. Init must run before any component builds a style, so the colors are set once at
// startup and treated as immutable afterwards.
package theme

import (
	"image/color"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// Exported color variables.
var (
	Primary         color.Color
	Accent          color.Color
	Secondary       color.Color
	Text            color.Color
	SelectedBg      color.Color
	MutedBg         color.Color
	SoftMutedBg     color.Color
	ModalBg         color.Color
	Muted           color.Color
	AddedLine       color.Color
	DeletedLine     color.Color
	AddedBg         color.Color
	DeletedBg       color.Color
	WordDiffAddedBg color.Color
	WordDiffDelBg   color.Color
)

// Exported style variables.
var (
	HeaderStyle            lipgloss.Style
	HunkHeaderStyle        lipgloss.Style
	SectionStyle           lipgloss.Style
	KeyStyle               lipgloss.Style
	FooterStyle            lipgloss.Style
	AdditionStyle          lipgloss.Style
	DeletionStyle          lipgloss.Style
	SelectionStyle         lipgloss.Style
	SelectedFocusedStyle   lipgloss.Style
	SelectedUnfocusedStyle lipgloss.Style
	StatusBarStyle         lipgloss.Style
	BorderStyle            lipgloss.Style
	WordDiffAddedStyle     lipgloss.Style
	WordDiffDeletedStyle   lipgloss.Style
)

// mutedTintShare keeps line numbers and comments readable without competing with code.
const mutedTintShare = 0.6

// tint returns base moved share of the way toward accent.
func tint(base, accent color.Color, share float64) color.Color {
	const channelMax, byteMax = 0xffff, 0xff

	br, bg, bb, _ := base.RGBA()
	ar, ag, ab, _ := accent.RGBA()

	mix := func(from, to uint32) uint8 {
		return uint8((float64(from)*(1-share) + float64(to)*share) / channelMax * byteMax)
	}

	return color.RGBA{R: mix(br, ar), G: mix(bg, ag), B: mix(bb, ab), A: byteMax}
}

// Init detects the appropriate theme and the terminal's color depth, then initializes all colors and
// styles.
func Init() {
	t := Detect()
	applyTheme(&t, colorprofile.Detect(os.Stdout, os.Environ()))
}

// applyTheme sets color variables and recomputes all styles, deriving the diff tints so they stay
// distinct once p reduces them to the colors the terminal can show.
func applyTheme(t *Theme, p colorprofile.Profile) {
	Primary = t.Primary
	Accent = t.Accent
	Secondary = t.Secondary
	Text = t.Text
	SelectedBg = t.SelectedBg
	MutedBg = t.MutedBg
	SoftMutedBg = t.SoftMutedBg
	ModalBg = t.ModalBg
	AddedLine = t.AddedLine
	DeletedLine = t.DeletedLine
	Muted = tint(ModalBg, Text, mutedTintShare)

	applySkin(t, p)

	HeaderStyle = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true)

	HunkHeaderStyle = lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	SectionStyle = lipgloss.NewStyle().
		Foreground(Secondary).
		Bold(true)

	KeyStyle = lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	FooterStyle = lipgloss.NewStyle().
		Background(SoftMutedBg).
		Foreground(Text).
		Padding(0, 1)

	AdditionStyle = lipgloss.NewStyle().
		Foreground(AddedLine)

	DeletionStyle = lipgloss.NewStyle().
		Foreground(DeletedLine)

	// The skin's selection is a tint of the accent, and reverse video under NO_COLOR, so the row
	// under the cursor reads the same here as in second-look and jojo.
	SelectionStyle = Skin.Selection

	SelectedFocusedStyle = SelectionStyle.
		Foreground(Text).
		Bold(true)

	SelectedUnfocusedStyle = Skin.Band.Foreground(Text)

	StatusBarStyle = lipgloss.NewStyle().
		Background(SoftMutedBg).
		Foreground(Text).
		Padding(0, 1)

	BorderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Primary)

	WordDiffAddedStyle = lipgloss.NewStyle().
		Background(WordDiffAddedBg).
		Foreground(Text)

	WordDiffDeletedStyle = lipgloss.NewStyle().
		Background(WordDiffDelBg).
		Foreground(Text)
}

// PaneStyle returns a dynamic border style for panes.
func PaneStyle(width, height int, focused bool) lipgloss.Style {
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		BorderStyle(lipgloss.RoundedBorder())

	if focused {
		return style.BorderForeground(Primary)
	}

	return style.BorderForeground(MutedBg)
}
